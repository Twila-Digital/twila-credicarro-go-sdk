package auth_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/auth"
	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/httpclient"
	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/resilience"
	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/wire"
)

type fakeGenerator struct {
	calls   int32
	release chan struct{} // quando não nil, Generate espera
	respond func(n int32) (wire.GenerateAccessTokenResponse, error)
}

func (g *fakeGenerator) Generate(ctx context.Context, clientID, clientSecret string) (wire.GenerateAccessTokenResponse, error) {
	n := atomic.AddInt32(&g.calls, 1)
	if g.release != nil {
		<-g.release
	}
	if g.respond != nil {
		return g.respond(n)
	}
	return wire.GenerateAccessTokenResponse{Token: "token-1", ExpiresInS: 3600}, nil
}

func (g *fakeGenerator) callCount() int { return int(atomic.LoadInt32(&g.calls)) }

type clock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

func newClock() *clock { return &clock{t: time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)} }

func TestTokenIsCachedUntilExpiry(t *testing.T) {
	gen := &fakeGenerator{}
	clk := newClock()
	provider := auth.NewTokenProviderWithClock(gen, "id", "secret", clk.now)

	for i := 0; i < 3; i++ {
		token, err := provider.GetToken(context.Background())
		if err != nil || token != "token-1" {
			t.Fatalf("esperava token-1, veio %q (%v)", token, err)
		}
	}
	if gen.callCount() != 1 {
		t.Fatalf("esperava 1 geração de token, veio %d", gen.callCount())
	}

	clk.advance(3539 * time.Second)
	if _, err := provider.GetToken(context.Background()); err != nil || gen.callCount() != 1 {
		t.Fatalf("o token ainda deveria valer (calls=%d, err=%v)", gen.callCount(), err)
	}
	clk.advance(2 * time.Second)
	if _, err := provider.GetToken(context.Background()); err != nil || gen.callCount() != 2 {
		t.Fatalf("o token deveria ter sido renovado (calls=%d, err=%v)", gen.callCount(), err)
	}
}

func TestExpiresAtFromResponseTakesPrecedence(t *testing.T) {
	clk := newClock()
	gen := &fakeGenerator{respond: func(n int32) (wire.GenerateAccessTokenResponse, error) {
		return wire.GenerateAccessTokenResponse{
			Token:      "t",
			ExpiresInS: 3600,
			ExpiresAt:  clk.now().Add(30 * time.Second).UTC().Format(time.RFC3339),
		}, nil
	}}
	provider := auth.NewTokenProviderWithClock(gen, "id", "secret", clk.now)

	_, _ = provider.GetToken(context.Background())
	clk.advance(29 * time.Second)
	_, _ = provider.GetToken(context.Background())
	if gen.callCount() != 1 {
		t.Fatalf("esperava 1 geração antes de expira_em, veio %d", gen.callCount())
	}
	clk.advance(2 * time.Second)
	_, _ = provider.GetToken(context.Background())
	if gen.callCount() != 2 {
		t.Fatalf("esperava renovação depois de expira_em, veio %d gerações", gen.callCount())
	}
}

func TestConcurrentCallsShareASingleRefresh(t *testing.T) {
	gen := &fakeGenerator{release: make(chan struct{})}
	provider := auth.NewTokenProvider(gen, "id", "secret")

	const callers = 25
	var wg sync.WaitGroup
	results := make(chan string, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			token, err := provider.GetToken(context.Background())
			if err != nil {
				results <- "erro: " + err.Error()
				return
			}
			results <- token
		}()
	}

	time.Sleep(100 * time.Millisecond)
	close(gen.release)
	wg.Wait()
	close(results)

	for token := range results {
		if token != "token-1" {
			t.Fatalf("token inesperado: %q", token)
		}
	}
	if gen.callCount() != 1 {
		t.Fatalf("esperava 1 geração (single-flight), veio %d", gen.callCount())
	}
}

func TestWaiterRespectsItsOwnContextWhileRefreshContinues(t *testing.T) {
	gen := &fakeGenerator{release: make(chan struct{})}
	provider := auth.NewTokenProvider(gen, "id", "secret")

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := provider.GetToken(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("esperava DeadlineExceeded, veio %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("o chamador deveria ter saído assim que o contexto expirou")
	}

	done := make(chan string, 1)
	go func() {
		token, _ := provider.GetToken(context.Background())
		done <- token
	}()
	time.Sleep(30 * time.Millisecond)
	close(gen.release)

	select {
	case token := <-done:
		if token != "token-1" {
			t.Fatalf("token inesperado: %q", token)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("o segundo chamador não recebeu o token")
	}
	if gen.callCount() != 1 {
		t.Fatalf("esperava 1 geração, veio %d", gen.callCount())
	}
}

func TestCanceledContextFailsFast(t *testing.T) {
	gen := &fakeGenerator{}
	provider := auth.NewTokenProvider(gen, "id", "secret")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := provider.GetToken(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("esperava context.Canceled, veio %v", err)
	}
	if gen.callCount() != 0 {
		t.Fatalf("não deveria gerar token com o contexto cancelado, veio %d", gen.callCount())
	}
}

func TestInvalidateOnlyDropsTheRejectedToken(t *testing.T) {
	names := map[int32]string{1: "old", 2: "new", 3: "newer"}
	gen := &fakeGenerator{respond: func(n int32) (wire.GenerateAccessTokenResponse, error) {
		return wire.GenerateAccessTokenResponse{Token: names[n], ExpiresInS: 3600}, nil
	}}
	provider := auth.NewTokenProvider(gen, "id", "secret")

	first, _ := provider.GetToken(context.Background())
	if first != "old" {
		t.Fatalf("token inesperado: %q", first)
	}
	provider.Invalidate("old")
	second, _ := provider.GetToken(context.Background())
	if second != "new" || gen.callCount() != 2 {
		t.Fatalf("esperava renovação para new, veio %q (calls=%d)", second, gen.callCount())
	}

	provider.Invalidate("old")
	again, _ := provider.GetToken(context.Background())
	if again != "new" || gen.callCount() != 2 {
		t.Fatalf("o token novo deveria ser preservado, veio %q (calls=%d)", again, gen.callCount())
	}
}

func TestFailedRefreshIsNotCachedAndIsShared(t *testing.T) {
	boom := errors.New("falha")
	gen := &fakeGenerator{respond: func(n int32) (wire.GenerateAccessTokenResponse, error) {
		if n == 1 {
			return wire.GenerateAccessTokenResponse{}, boom
		}
		return wire.GenerateAccessTokenResponse{Token: "ok", ExpiresInS: 3600}, nil
	}}
	provider := auth.NewTokenProvider(gen, "id", "secret")

	if _, err := provider.GetToken(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("esperava a falha da geração, veio %v", err)
	}
	token, err := provider.GetToken(context.Background())
	if err != nil || token != "ok" {
		t.Fatalf("esperava recuperar na chamada seguinte, veio %q (%v)", token, err)
	}
}

func fastResilience() resilience.Options {
	return resilience.Options{
		TotalTimeout:                 5 * time.Second,
		MaxRetryAttempts:             3,
		RetryBaseDelay:               time.Millisecond,
		CircuitBreakerFailureRatio:   0.5,
		CircuitBreakerSamplingWindow: 30 * time.Second,
		CircuitBreakerMinThroughput:  1000,
		CircuitBreakerBreakDuration:  time.Second,
		RetryOn500:                   true,
	}
}

func TestGenerateSendsCredentialsAndParsesResponse(t *testing.T) {
	var gotBody map[string]string
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/integration/v1/authentication/accesstoken" || r.Method != http.MethodPost {
			t.Errorf("rota inesperada: %s %s", r.Method, r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"token_de_acesso":"abc","expira_em_segundos":3600,"expira_em":"2025-08-12T18:06:28.088+00:00","tipo_de_token":"Bearer","escopo":"credicarro.integration.access/full"}`))
	}))
	defer server.Close()

	client := auth.NewTokenAPIClient(httpclient.New(), server.URL+"/integration/", 2*time.Second, fastResilience())
	resp, err := client.Generate(context.Background(), "meu-id", "meu-segredo")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if resp.Token != "abc" || resp.TokenType != "Bearer" || resp.Scope == "" || resp.ExpiresInS != 3600 || resp.ExpiresAt == "" {
		t.Fatalf("resposta incorreta: %+v", resp)
	}
	if gotBody["clientId"] != "meu-id" || gotBody["clientSecret"] != "meu-segredo" || gotAuth != "" {
		t.Fatalf("requisição incorreta: body=%v auth=%q", gotBody, gotAuth)
	}
}

func TestGenerateRejectedKeepsStatusAndBodyButNeverTheSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"tipo":"Integration.Authentication.InvalidCredentials","status":401}`))
	}))
	defer server.Close()

	client := auth.NewTokenAPIClient(httpclient.New(), server.URL+"/", 2*time.Second, fastResilience())
	_, err := client.Generate(context.Background(), "id", "segredo-super-secreto")

	var authErr *auth.Error
	if !errors.As(err, &authErr) || authErr.Kind != auth.KindRejected || authErr.Response.StatusCode != 401 {
		t.Fatalf("esperava auth.Error rejeitado com 401, veio %T: %v", err, err)
	}
	if strings.Contains(err.Error(), "segredo-super-secreto") {
		t.Fatal("o erro não pode conter o ClientSecret")
	}
}

func TestGenerateRetriesTransientStatus(t *testing.T) {
	var calls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"token_de_acesso":"abc","expira_em_segundos":3600}`))
	}))
	defer server.Close()

	client := auth.NewTokenAPIClient(httpclient.New(), server.URL+"/", 2*time.Second, fastResilience())
	resp, err := client.Generate(context.Background(), "id", "secret")
	if err != nil || resp.Token != "abc" || atomic.LoadInt32(&calls) != 3 {
		t.Fatalf("esperava sucesso na 3a tentativa, veio %v (calls=%d)", err, atomic.LoadInt32(&calls))
	}
}

func TestGenerateEmptyOrInvalidBody(t *testing.T) {
	for _, body := range []string{``, `not json`, `{}`, `{"token_de_acesso":""}`} {
		body := body
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(body))
		}))
		client := auth.NewTokenAPIClient(httpclient.New(), server.URL+"/", 2*time.Second, fastResilience())
		_, err := client.Generate(context.Background(), "id", "secret")
		server.Close()

		var authErr *auth.Error
		if !errors.As(err, &authErr) || authErr.Kind != auth.KindEmptyResponse {
			t.Fatalf("corpo %q: esperava KindEmptyResponse, veio %T: %v", body, err, err)
		}
	}
}

func TestGenerateNetworkFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := server.URL
	server.Close()

	client := auth.NewTokenAPIClient(httpclient.New(), url+"/", time.Second, fastResilience())
	_, err := client.Generate(context.Background(), "id", "secret")

	var authErr *auth.Error
	if !errors.As(err, &authErr) || authErr.Kind != auth.KindNetwork {
		t.Fatalf("esperava KindNetwork, veio %T: %v", err, err)
	}
}

func TestShortLivedTokenIsStillCachedForHalfItsLifetime(t *testing.T) {
	cases := []struct {
		seconds int64
		valid   time.Duration
	}{{30, 15 * time.Second}, {60, 30 * time.Second}, {100, 50 * time.Second}, {120, 60 * time.Second}, {3600, 3540 * time.Second}}
	for _, c := range cases {
		c := c
		clk := newClock()
		gen := &fakeGenerator{respond: func(n int32) (wire.GenerateAccessTokenResponse, error) {
			return wire.GenerateAccessTokenResponse{Token: "t", ExpiresInS: c.seconds}, nil
		}}
		provider := auth.NewTokenProviderWithClock(gen, "id", "secret", clk.now)

		_, _ = provider.GetToken(context.Background())
		clk.advance(c.valid - time.Second)
		_, _ = provider.GetToken(context.Background())
		if gen.callCount() != 1 {
			t.Errorf("%d s: o token deveria valer até %v", c.seconds, c.valid)
		}
		clk.advance(2 * time.Second)
		_, _ = provider.GetToken(context.Background())
		if gen.callCount() != 2 {
			t.Errorf("%d s: o token deveria expirar após %v", c.seconds, c.valid)
		}
	}
}
