// Package transport executa requisições HTTP autenticadas com idempotency key
// automática e pipeline de resiliência (retry + circuit breaker + timeout total).
package transport

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/httperr"
	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/idempotency"
	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/resilience"
)

// TokenProvider é satisfeito estruturalmente por auth.TokenProvider — sem import direto
// entre os dois pacotes.
type TokenProvider interface {
	GetToken(ctx context.Context) (string, error)
	Invalidate(rejected string)
}

// RawResponse é uma resposta HTTP já lida por completo.
type RawResponse struct {
	StatusCode int
	Body       []byte
	Header     http.Header
}

// RequestOptions ajusta uma requisição.
type RequestOptions struct {
	// IdempotencyKey explícita; substitui a automática e é enviada em qualquer POST.
	IdempotencyKey string
	// RequireKey gera uma chave quando IdempotencyKey está vazia, mesmo com a chave
	// automática desabilitada (rota em que o header é obrigatório: envio de documento).
	RequireKey bool
	// AttemptTimeout sobrescreve o timeout por tentativa (zero usa o padrão do cliente).
	AttemptTimeout time.Duration
}

// Executor envia as requisições autenticadas.
type Executor struct {
	http                   *http.Client
	maxAttempts            int
	baseURL                string
	tokenProvider          TokenProvider
	resiliencePipeline     *resilience.Pipeline
	totalTimeout           time.Duration
	attemptTimeout         time.Duration
	disableAutoIdempotency bool
}

// NewExecutor cria o executor. baseURL deve terminar com "/".
func NewExecutor(
	httpClient *http.Client,
	baseURL string,
	tokenProvider TokenProvider,
	resilienceOpts resilience.Options,
	attemptTimeout time.Duration,
	disableAutoIdempotency bool,
) *Executor {
	return &Executor{
		http:                   httpClient,
		maxAttempts:            resilienceOpts.MaxRetryAttempts,
		baseURL:                baseURL,
		tokenProvider:          tokenProvider,
		resiliencePipeline:     resilience.NewPipeline(resilienceOpts),
		totalTimeout:           resilienceOpts.TotalTimeout,
		attemptTimeout:         attemptTimeout,
		disableAutoIdempotency: disableAutoIdempotency,
	}
}

// Get envia um GET.
func (e *Executor) Get(ctx context.Context, path string) (RawResponse, error) {
	return e.send(ctx, http.MethodGet, path, nil, RequestOptions{})
}

// Post envia um POST com corpo JSON.
func (e *Executor) Post(ctx context.Context, path string, body interface{}, opts RequestOptions) (RawResponse, error) {
	return e.send(ctx, http.MethodPost, path, body, opts)
}

// Put envia um PUT com corpo JSON.
func (e *Executor) Put(ctx context.Context, path string, body interface{}) (RawResponse, error) {
	return e.send(ctx, http.MethodPut, path, body, RequestOptions{})
}

// Delete envia um DELETE.
func (e *Executor) Delete(ctx context.Context, path string) (RawResponse, error) {
	return e.send(ctx, http.MethodDelete, path, nil, RequestOptions{})
}

// EnsureSuccess retorna um *httperr.Response (como error) se o status não for 2xx.
func EnsureSuccess(resp RawResponse) error {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &httperr.Response{StatusCode: resp.StatusCode, Body: resp.Body, Header: resp.Header}
	}
	return nil
}

func (e *Executor) send(ctx context.Context, method, path string, body interface{}, opts RequestOptions) (RawResponse, error) {
	// A chave é decidida UMA vez por chamada lógica — fora do pipeline de retry — para
	// que todas as tentativas (e o replay após 401) usem exatamente a mesma.
	idempotencyKey := opts.IdempotencyKey
	if idempotencyKey == "" && method == http.MethodPost {
		auto := !e.disableAutoIdempotency && idempotency.RequiresKey(method, path)
		if auto || opts.RequireKey {
			generated, err := newUUIDv4()
			if err != nil {
				return RawResponse{}, err
			}
			idempotencyKey = generated
		}
	}
	retrySafe := idempotency.IsRetrySafe(method, path, idempotencyKey != "")

	attemptTimeout := opts.AttemptTimeout
	if attemptTimeout <= 0 {
		attemptTimeout = e.attemptTimeout
	}
	// Orçamento total: TotalTimeout; no envio de documento (tentativa própria, mais longa),
	// tentativa × número máximo de tentativas.
	total := e.totalTimeout
	if opts.AttemptTimeout > 0 {
		total = opts.AttemptTimeout * time.Duration(e.maxAttempts)
	}

	var bodyBytes []byte
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return RawResponse{}, err
		}
		bodyBytes = encoded
	}

	var lastRaw RawResponse

	_, err := e.resiliencePipeline.ExecuteWithTotal(ctx, total, retrySafe, isNetworkError, func(attemptCtx context.Context) (resilience.Response, error) {
		raw, sendErr := e.sendWithAuth(attemptCtx, method, path, bodyBytes, idempotencyKey, attemptTimeout)
		if sendErr != nil {
			return resilience.Response{}, sendErr
		}
		lastRaw = raw
		return resilience.Response{StatusCode: raw.StatusCode, RetryAfter: httperr.ParseRetryAfter(raw.Header.Get("Retry-After"), time.Now())}, nil
	})
	if err != nil {
		return RawResponse{}, err
	}
	return lastRaw, nil
}

// sendWithAuth envia a requisição e, em caso de 401, renova o token e repete UMA vez.
func (e *Executor) sendWithAuth(ctx context.Context, method, path string, body []byte, idempotencyKey string, attemptTimeout time.Duration) (RawResponse, error) {
	token, err := e.tokenProvider.GetToken(ctx)
	if err != nil {
		return RawResponse{}, err
	}

	resp, err := e.sendOnce(ctx, method, path, body, idempotencyKey, attemptTimeout, token)
	if err != nil {
		return RawResponse{}, err
	}
	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}

	e.tokenProvider.Invalidate(token)
	newToken, err := e.tokenProvider.GetToken(ctx)
	if err != nil {
		return RawResponse{}, err
	}

	return e.sendOnce(ctx, method, path, body, idempotencyKey, attemptTimeout, newToken)
}

func (e *Executor) sendOnce(ctx context.Context, method, path string, body []byte, idempotencyKey string, attemptTimeout time.Duration, token string) (RawResponse, error) {
	// baseURL sempre termina com "/" (garantido na resolução de ClientOptions) e path nunca
	// começa com "/" — concatenação direta evita a armadilha de resolução de URI relativa
	// que descartaria o segmento "integration" da base URL.
	fullURL := e.baseURL + path

	attemptCtx := ctx
	if attemptTimeout > 0 {
		var cancel context.CancelFunc
		attemptCtx, cancel = context.WithTimeout(ctx, attemptTimeout)
		defer cancel()
	}

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(attemptCtx, method, fullURL, reader)
	if err != nil {
		return RawResponse{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if idempotencyKey != "" {
		req.Header.Set("Idempotency-Key", idempotencyKey)
	}

	raw, err := e.http.Do(req)
	if err != nil {
		return RawResponse{}, &NetworkError{Cause: sanitize(err)}
	}
	defer func() { _ = raw.Body.Close() }()

	respBody, err := io.ReadAll(raw.Body)
	if err != nil {
		return RawResponse{}, &NetworkError{Cause: sanitize(err)}
	}

	return RawResponse{StatusCode: raw.StatusCode, Body: respBody, Header: raw.Header}, nil
}

// NetworkError é uma falha de transporte (conexão, DNS, reset, timeout de tentativa).
// Cause nunca carrega a requisição, a URL (com placa, CNPJ...), cabeçalhos nem corpo.
type NetworkError struct {
	Cause error
}

func (e *NetworkError) Error() string { return "falha de rede ou timeout" }
func (e *NetworkError) Unwrap() error { return e.Cause }

// sanitize remove o *url.Error (que embute a URL completa) deixando só a causa.
func sanitize(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) && ue.Err != nil {
		return ue.Err
	}
	return err
}

func isNetworkError(err error) bool {
	var httpErr *httperr.Response
	if errors.As(err, &httpErr) {
		return false
	}
	var authFailure interface{ AuthFailure() }
	if errors.As(err, &authFailure) {
		return false
	}
	var exhausted *resilience.TimeoutExceededError
	if errors.As(err, &exhausted) {
		return false
	}
	var transportErr *NetworkError
	if errors.As(err, &transportErr) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var urlErr *url.Error
	return errors.As(err, &urlErr)
}

func newUUIDv4() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	buf[6] = (buf[6] & 0x0f) | 0x40
	buf[8] = (buf[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", buf[0:4], buf[4:6], buf[6:8], buf[8:10], buf[10:16]), nil
}
