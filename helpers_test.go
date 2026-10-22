package credicarro_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	credicarro "github.com/Twila-Digital/twila-credicarro-go-sdk"
)

const (
	testUUID   = "bea8d060-f93d-48eb-b6c4-45f64483cdd9"
	testUUID2  = "123e4567-e89b-12d3-a456-426614174000"
	testSecret = "segredo-do-cliente-de-teste"
)

// fastResilienceOptions evita esperar o backoff real nos testes — o comportamento de
// retry/circuit breaker em si já tem cobertura dedicada em internal/resilience.
func fastResilienceOptions() credicarro.ResilienceOptions {
	return credicarro.ResilienceOptions{
		TotalTimeout:                 5 * time.Second,
		AttemptTimeout:               2 * time.Second,
		DocumentUploadAttemptTimeout: 5 * time.Second,
		MaxRetryAttempts:             3,
		RetryBaseDelay:               time.Millisecond,
		CircuitBreakerFailureRatio:   0.5,
		CircuitBreakerSamplingWindow: 30 * time.Second,
		CircuitBreakerMinThroughput:  1000,
		CircuitBreakerBreakDuration:  time.Second,
		RetryOn500:                   true,
	}
}

// recordedRequest é uma requisição de recurso recebida pelo servidor de teste.
type recordedRequest struct {
	Method   string
	Path     string
	RawQuery string
	Header   http.Header
	Body     []byte
}

func (r recordedRequest) JSON(t *testing.T) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.Unmarshal(r.Body, &out); err != nil {
		t.Fatalf("corpo da requisição não é JSON: %v (%s)", err, r.Body)
	}
	return out
}

// fakeAPI simula a API: a rota de token (com contador) e um handler para os recursos.
type fakeAPI struct {
	t      *testing.T
	server *httptest.Server

	mu       sync.Mutex
	requests []recordedRequest

	tokenCalls int32
	// tokenHandler substitui a resposta padrão da rota de token (n começa em 1).
	tokenHandler func(w http.ResponseWriter, r *http.Request, n int)
	// resource responde às rotas de recurso.
	resource http.HandlerFunc
}

func newFakeAPI(t *testing.T, resource http.HandlerFunc) *fakeAPI {
	t.Helper()
	api := &fakeAPI{t: t, resource: resource}

	mux := http.NewServeMux()
	mux.HandleFunc("/integration/v1/authentication/accesstoken", func(w http.ResponseWriter, r *http.Request) {
		n := int(atomic.AddInt32(&api.tokenCalls, 1))
		if api.tokenHandler != nil {
			api.tokenHandler(w, r, n)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"token_de_acesso":    "test-token",
			"expira_em_segundos": 3600,
			"tipo_de_token":      "Bearer",
			"escopo":             "credicarro.integration.access/full",
		})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		api.mu.Lock()
		api.requests = append(api.requests, recordedRequest{
			Method:   r.Method,
			Path:     strings.TrimPrefix(r.URL.Path, "/integration/"),
			RawQuery: r.URL.RawQuery,
			Header:   r.Header.Clone(),
			Body:     body,
		})
		api.mu.Unlock()
		if api.resource == nil {
			http.NotFound(w, r)
			return
		}
		api.resource(w, r)
	})

	api.server = httptest.NewServer(mux)
	t.Cleanup(api.server.Close)
	return api
}

func (a *fakeAPI) baseURL() string { return a.server.URL + "/integration/" }

func (a *fakeAPI) tokenCount() int { return int(atomic.LoadInt32(&a.tokenCalls)) }

func (a *fakeAPI) recorded() []recordedRequest {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]recordedRequest(nil), a.requests...)
}

func (a *fakeAPI) client(opts ...func(*credicarro.ClientOptions)) *credicarro.Client {
	a.t.Helper()
	resilience := fastResilienceOptions()
	options := credicarro.ClientOptions{
		ClientID:     "test-client-id",
		ClientSecret: testSecret,
		BaseURL:      a.baseURL(),
		Resilience:   &resilience,
	}
	for _, apply := range opts {
		apply(&options)
	}
	client, err := credicarro.NewClient(options)
	if err != nil {
		a.t.Fatalf("NewClient falhou: %v", err)
	}
	return client
}

func withResilience(mutate func(*credicarro.ResilienceOptions)) func(*credicarro.ClientOptions) {
	return func(o *credicarro.ClientOptions) {
		r := fastResilienceOptions()
		mutate(&r)
		o.Resilience = &r
	}
}

// newTestClient cria um servidor com o handler informado e devolve o cliente apontado para ele.
func newTestClient(t *testing.T, resource http.HandlerFunc) (*credicarro.Client, *fakeAPI) {
	t.Helper()
	api := newFakeAPI(t, resource)
	return api.client(), api
}

func writeJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeRaw(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}
