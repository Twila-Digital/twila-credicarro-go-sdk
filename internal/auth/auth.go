// Package auth cuida da geração e do cache do token de acesso.
package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/httperr"
	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/idempotency"
	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/resilience"
	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/wire"
)

// Kind classifica uma falha ao gerar o token de acesso.
type Kind int

const (
	// KindRejected: a API respondeu com status não-2xx (401 credenciais inválidas, 400...).
	KindRejected Kind = iota + 1
	// KindNetwork: falha de transporte (rede, DNS, timeout de tentativa) ao chamar a API.
	KindNetwork
	// KindEmptyResponse: resposta 2xx sem corpo válido ou sem token_de_acesso.
	KindEmptyResponse
)

// Error é a falha ao gerar o token. Nunca carrega o ClientSecret.
type Error struct {
	Kind     Kind
	Response *httperr.Response // preenchido em KindRejected
	Cause    error             // preenchido em KindNetwork
}

func (e *Error) Error() string {
	switch e.Kind {
	case KindRejected:
		return "a API do Credicarro recusou a geração do token de acesso"
	case KindNetwork:
		return "falha de rede ao gerar o token de acesso"
	default:
		return "a API do Credicarro retornou uma resposta vazia ao gerar o token de acesso"
	}
}

func (e *Error) Unwrap() error { return e.Cause }

// AuthFailure marca o erro como falha de autenticação: o transporte não repete a
// chamada do recurso por causa dele (o próprio cliente de token já tem retry).
func (e *Error) AuthFailure() {}

// TokenAPIClient chama POST v1/authentication/accesstoken com retry e circuit breaker
// próprios (o circuit breaker é separado do das chamadas de recurso).
type TokenAPIClient struct {
	http           *http.Client
	baseURL        string
	attemptTimeout time.Duration
	pipeline       *resilience.Pipeline
}

// NewTokenAPIClient cria o cliente de token. httpClient deve ter os redirects desligados
// (veja internal/httpclient); opts define o retry/circuit breaker.
func NewTokenAPIClient(httpClient *http.Client, baseURL string, attemptTimeout time.Duration, opts resilience.Options) *TokenAPIClient {
	return &TokenAPIClient{
		http:           httpClient,
		baseURL:        baseURL,
		attemptTimeout: attemptTimeout,
		pipeline:       resilience.NewPipeline(opts),
	}
}

// transportError distingue falhas de transporte (repetíveis) dos demais erros dentro do pipeline.
type transportError struct{ cause error }

func (e *transportError) Error() string { return "falha de transporte" }
func (e *transportError) Unwrap() error { return e.cause }

func isTransportError(err error) bool {
	var te *transportError
	return errors.As(err, &te)
}

// Generate obtém um novo token. Falhas voltam como *Error, *resilience.TimeoutExceededError
// (timeout total / circuit breaker aberto) ou o erro do contexto do chamador.
func (c *TokenAPIClient) Generate(ctx context.Context, clientID, clientSecret string) (wire.GenerateAccessTokenResponse, error) {
	reqBody, err := json.Marshal(wire.GenerateAccessTokenRequest{ClientID: clientID, ClientSecret: clientSecret})
	if err != nil {
		return wire.GenerateAccessTokenResponse{}, err
	}

	var (
		status int
		header http.Header
		body   []byte
	)

	_, err = c.pipeline.Execute(ctx, true, isTransportError, func(attemptCtx context.Context) (resilience.Response, error) {
		if c.attemptTimeout > 0 {
			var cancel context.CancelFunc
			attemptCtx, cancel = context.WithTimeout(attemptCtx, c.attemptTimeout)
			defer cancel()
		}

		req, reqErr := http.NewRequestWithContext(attemptCtx, http.MethodPost, c.baseURL+idempotency.TokenPath, bytes.NewReader(reqBody))
		if reqErr != nil {
			return resilience.Response{}, reqErr
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")

		raw, doErr := c.http.Do(req)
		if doErr != nil {
			return resilience.Response{}, &transportError{cause: doErr}
		}
		defer func() { _ = raw.Body.Close() }()

		data, readErr := io.ReadAll(raw.Body)
		if readErr != nil {
			return resilience.Response{}, &transportError{cause: readErr}
		}

		status, header, body = raw.StatusCode, raw.Header, data
		return resilience.Response{StatusCode: raw.StatusCode, RetryAfter: httperr.ParseRetryAfter(raw.Header.Get("Retry-After"), time.Now())}, nil
	})
	if err != nil {
		var te *transportError
		if errors.As(err, &te) {
			return wire.GenerateAccessTokenResponse{}, &Error{Kind: KindNetwork, Cause: sanitize(te.cause)}
		}
		return wire.GenerateAccessTokenResponse{}, err
	}

	if status < 200 || status >= 300 {
		return wire.GenerateAccessTokenResponse{}, &Error{Kind: KindRejected, Response: &httperr.Response{StatusCode: status, Body: body, Header: header}}
	}

	var result wire.GenerateAccessTokenResponse
	if err := json.Unmarshal(body, &result); err != nil || result.Token == "" {
		return wire.GenerateAccessTokenResponse{}, &Error{Kind: KindEmptyResponse}
	}
	return result, nil
}

// sanitize devolve a causa de transporte sem a URL completa embutida no *url.Error.
func sanitize(err error) error {
	var ue interface{ Unwrap() error }
	if errors.As(err, &ue) {
		if inner := ue.Unwrap(); inner != nil {
			return inner
		}
	}
	return err
}

// ExpiryMargin é a margem aplicada quando a API não informa expira_em.
const ExpiryMargin = 60 * time.Second

// Generator é satisfeito por *TokenAPIClient (e por fakes nos testes).
type Generator interface {
	Generate(ctx context.Context, clientID, clientSecret string) (wire.GenerateAccessTokenResponse, error)
}

type call struct {
	done  chan struct{}
	token string
	err   error
}

// TokenProvider mantém o token em cache e renova sob demanda. A renovação é
// single-flight: chamadas concorrentes compartilham uma única requisição de token, e
// cada uma respeita o próprio contexto enquanto espera.
type TokenProvider struct {
	generator    Generator
	clientID     string
	clientSecret string
	now          func() time.Time

	mu        sync.Mutex
	value     string
	expiresAt time.Time
	inflight  *call
}

// NewTokenProvider cria o provedor com o relógio do sistema.
func NewTokenProvider(generator Generator, clientID, clientSecret string) *TokenProvider {
	return NewTokenProviderWithClock(generator, clientID, clientSecret, time.Now)
}

// NewTokenProviderWithClock permite injetar o relógio (testes).
func NewTokenProviderWithClock(generator Generator, clientID, clientSecret string, now func() time.Time) *TokenProvider {
	return &TokenProvider{generator: generator, clientID: clientID, clientSecret: clientSecret, now: now}
}

// GetToken devolve o token em cache ainda válido ou renova (uma requisição por vez).
func (p *TokenProvider) GetToken(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	p.mu.Lock()
	if p.value != "" && p.now().Before(p.expiresAt) {
		token := p.value
		p.mu.Unlock()
		return token, nil
	}

	c := p.inflight
	if c == nil {
		c = &call{done: make(chan struct{})}
		p.inflight = c
		// A renovação roda desacoplada do contexto de quem a disparou: se esse chamador
		// cancelar, os demais que esperam o mesmo token não são afetados. Ela é limitada
		// pelo timeout total do pipeline do cliente de token.
		go p.refresh(c)
	}
	p.mu.Unlock()

	select {
	case <-c.done:
		return c.token, c.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (p *TokenProvider) refresh(c *call) {
	resp, err := p.generator.Generate(context.Background(), p.clientID, p.clientSecret)

	p.mu.Lock()
	if err == nil {
		p.value = resp.Token
		p.expiresAt = p.computeExpiry(resp)
		c.token = resp.Token
	}
	c.err = err
	p.inflight = nil
	p.mu.Unlock()

	close(c.done)
}

func (p *TokenProvider) computeExpiry(resp wire.GenerateAccessTokenResponse) time.Time {
	now := p.now()
	if resp.ExpiresAt != "" {
		if t, err := time.Parse(time.RFC3339, resp.ExpiresAt); err == nil {
			return t
		}
	}
	if resp.ExpiresInS > 0 {
		// validade = max(s/2, s-60): tokens curtos não são recusados de imediato.
		lifetime := time.Duration(resp.ExpiresInS) * time.Second
		margin := lifetime - ExpiryMargin
		if half := lifetime / 2; half > margin {
			margin = half
		}
		return now.Add(margin)
	}
	return now
}

// Invalidate descarta o token em cache somente se rejected ainda for o token atual —
// evita derrubar um token novo que outra goroutine acabou de obter.
func (p *TokenProvider) Invalidate(rejected string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.value == rejected {
		p.value = ""
		p.expiresAt = time.Time{}
	}
}
