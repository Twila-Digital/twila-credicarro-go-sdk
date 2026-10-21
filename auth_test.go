package credicarro_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	credicarro "github.com/Twila-Digital/twila-credicarro-go-sdk"
)

func okProposal(w http.ResponseWriter, r *http.Request) {
	writeRaw(w, http.StatusOK, `{"id_proposta":"`+testUUID+`","numero":1,"id_simulacao":"`+testUUID2+`","tipo":"Financing","status":"Approved","comprador":{"cpf":"12345678909"},"data_criacao":"2026-10-12T14:05:08-03:00"}`)
}

func TestTokenIsRequestedOnceAndReusedAcrossCalls(t *testing.T) {
	client, api := newTestClient(t, okProposal)

	for i := 0; i < 3; i++ {
		if _, err := client.V1.Proposals.Get(context.Background(), testUUID); err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
	}
	if api.tokenCount() != 1 {
		t.Fatalf("esperava 1 requisição de token, veio %d", api.tokenCount())
	}
	for _, req := range api.recorded() {
		if req.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("Authorization incorreto: %q", req.Header.Get("Authorization"))
		}
	}
}

func TestTokenRequestCarriesCredentialsOnly(t *testing.T) {
	var mu sync.Mutex
	var body map[string]string
	var authHeader string
	api := newFakeAPI(t, okProposal)
	api.tokenHandler = func(w http.ResponseWriter, r *http.Request, n int) {
		mu.Lock()
		authHeader = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Unlock()
		writeJSON(w, http.StatusOK, map[string]interface{}{"token_de_acesso": "t", "expira_em_segundos": 3600})
	}

	if _, err := api.client().V1.Proposals.Get(context.Background(), testUUID); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if body["clientId"] != "test-client-id" || body["clientSecret"] != testSecret || len(body) != 2 || authHeader != "" {
		t.Fatalf("requisição de token incorreta: %v (auth=%q)", body, authHeader)
	}
}

func TestExpiredTokenIsRenewedBeforeTheNextCall(t *testing.T) {
	api := newFakeAPI(t, okProposal)
	api.tokenHandler = func(w http.ResponseWriter, r *http.Request, n int) {
		// expira_em já no passado: o SDK precisa renovar a cada chamada
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"token_de_acesso": "t" + string(rune('0'+n)), "expira_em_segundos": 3600, "expira_em": "2020-01-01T00:00:00+00:00",
		})
	}
	client := api.client()

	for i := 0; i < 2; i++ {
		if _, err := client.V1.Proposals.Get(context.Background(), testUUID); err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
	}
	if api.tokenCount() != 2 {
		t.Fatalf("esperava 2 requisições de token, veio %d", api.tokenCount())
	}
	reqs := api.recorded()
	if reqs[0].Header.Get("Authorization") != "Bearer t1" || reqs[1].Header.Get("Authorization") != "Bearer t2" {
		t.Fatalf("tokens usados incorretos: %q / %q", reqs[0].Header.Get("Authorization"), reqs[1].Header.Get("Authorization"))
	}
}

func TestRefreshesTokenAndRetriesOnceOn401KeepingTheIdempotencyKey(t *testing.T) {
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer stale-token" {
			w.WriteHeader(http.StatusUnauthorized) // 401 sem corpo, como a API faz
			return
		}
		writeRaw(w, http.StatusCreated, `{"id_proposta":"`+testUUID+`"}`)
	})
	api.tokenHandler = func(w http.ResponseWriter, r *http.Request, n int) {
		token := "stale-token"
		if n > 1 {
			token = "fresh-token"
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"token_de_acesso": token, "expira_em_segundos": 3600})
	}

	id, err := api.client().Proposals.Create(context.Background(), credicarro.CreateProposalRequest{Cpf: "12345678909", SimulationID: testUUID2})
	if err != nil || id != testUUID {
		t.Fatalf("esperava sucesso após renovar o token, veio %q (%v)", id, err)
	}

	reqs := api.recorded()
	if len(reqs) != 2 {
		t.Fatalf("esperava 2 requisições de recurso, veio %d", len(reqs))
	}
	if reqs[0].Header.Get("Authorization") != "Bearer stale-token" || reqs[1].Header.Get("Authorization") != "Bearer fresh-token" {
		t.Fatalf("tokens incorretos: %q / %q", reqs[0].Header.Get("Authorization"), reqs[1].Header.Get("Authorization"))
	}
	key := reqs[0].Header.Get("Idempotency-Key")
	if key == "" || key != reqs[1].Header.Get("Idempotency-Key") {
		t.Fatalf("a Idempotency-Key deve ser a mesma no replay após 401: %q / %q", key, reqs[1].Header.Get("Idempotency-Key"))
	}
	if api.tokenCount() != 2 {
		t.Fatalf("esperava 2 requisições de token, veio %d", api.tokenCount())
	}
}

func TestPersistent401ReturnsAuthenticationErrorWithoutLooping(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})

	_, err := client.V1.Proposals.Get(context.Background(), testUUID)

	var authErr *credicarro.AuthenticationError
	if !errors.As(err, &authErr) || authErr.StatusCode != 401 {
		t.Fatalf("esperava *AuthenticationError 401, veio %T: %v", err, err)
	}
	if authErr.ErrorCode != "" || authErr.Detail != "" {
		t.Fatalf("401 sem corpo não deveria trazer código/detalhe: %+v", authErr)
	}
	if got := len(api.recorded()); got != 2 {
		t.Fatalf("esperava exatamente 2 tentativas (original + 1 replay), veio %d", got)
	}
}

func TestTokenEndpoint401MapsToAuthenticationErrorWithStableCode(t *testing.T) {
	api := newFakeAPI(t, okProposal)
	api.tokenHandler = func(w http.ResponseWriter, r *http.Request, n int) {
		writeRaw(w, http.StatusUnauthorized, `{"tipo":"Integration.Authentication.InvalidCredentials","titulo":"Não autorizado","status":401,"detalhe":"Credenciais inválidas para o CPF 12345678909."}`)
	}

	_, err := api.client().V1.Proposals.Get(context.Background(), testUUID)

	var authErr *credicarro.AuthenticationError
	if !errors.As(err, &authErr) {
		t.Fatalf("esperava *AuthenticationError, veio %T: %v", err, err)
	}
	if authErr.ErrorCode != "Integration.Authentication.InvalidCredentials" || authErr.StatusCode != 401 {
		t.Fatalf("campos incorretos: %+v", authErr)
	}
	if strings.Contains(err.Error(), "12345678909") || strings.Contains(err.Error(), testSecret) {
		t.Fatalf("a mensagem não pode conter detalhe nem segredo: %q", err.Error())
	}
	if !strings.Contains(authErr.Detail, "12345678909") {
		t.Fatal("o detalhe deveria ficar disponível na propriedade Detail")
	}
	if len(api.recorded()) != 0 {
		t.Fatal("o recurso não deveria ser chamado sem token")
	}
}

func TestTokenEndpoint400MapsToAuthenticationError(t *testing.T) {
	api := newFakeAPI(t, okProposal)
	api.tokenHandler = func(w http.ResponseWriter, r *http.Request, n int) {
		writeRaw(w, http.StatusBadRequest, `{"tipo":"Validation.Field.Required","titulo":"Requisição inválida","status":400,"campo":"clientSecret"}`)
	}

	_, err := api.client().V1.Proposals.Get(context.Background(), testUUID)

	var authErr *credicarro.AuthenticationError
	if !errors.As(err, &authErr) || authErr.StatusCode != 400 || authErr.ErrorCode != "Validation.Field.Required" || authErr.Field != "clientSecret" {
		t.Fatalf("esperava *AuthenticationError 400, veio %T: %+v", err, err)
	}
	if api.tokenCount() != 1 {
		t.Fatalf("400 não deveria ser repetido, veio %d requisições de token", api.tokenCount())
	}
}

func TestTokenEndpointEmptyBodyMapsToAuthenticationError(t *testing.T) {
	api := newFakeAPI(t, okProposal)
	api.tokenHandler = func(w http.ResponseWriter, r *http.Request, n int) {
		writeRaw(w, http.StatusOK, `{}`)
	}

	_, err := api.client().V1.Proposals.Get(context.Background(), testUUID)
	var authErr *credicarro.AuthenticationError
	if !errors.As(err, &authErr) {
		t.Fatalf("esperava *AuthenticationError, veio %T: %v", err, err)
	}
}

func TestTokenEndpointTransientFailureIsRetried(t *testing.T) {
	api := newFakeAPI(t, okProposal)
	api.tokenHandler = func(w http.ResponseWriter, r *http.Request, n int) {
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"token_de_acesso": "t", "expira_em_segundos": 3600})
	}

	if _, err := api.client().V1.Proposals.Get(context.Background(), testUUID); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if api.tokenCount() != 3 {
		t.Fatalf("esperava 3 requisições de token, veio %d", api.tokenCount())
	}
}

func TestConcurrentCallsShareASingleTokenRequest(t *testing.T) {
	api := newFakeAPI(t, okProposal)
	api.tokenHandler = func(w http.ResponseWriter, r *http.Request, n int) {
		time.Sleep(150 * time.Millisecond)
		writeJSON(w, http.StatusOK, map[string]interface{}{"token_de_acesso": "t", "expira_em_segundos": 3600})
	}
	client := api.client()

	const callers = 20
	var wg sync.WaitGroup
	errs := make(chan error, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := client.V1.Proposals.Get(context.Background(), testUUID)
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
	}
	if api.tokenCount() != 1 {
		t.Fatalf("esperava 1 requisição de token (single-flight), veio %d", api.tokenCount())
	}
}

func TestCallerContextIsRespectedWhileWaitingForTheToken(t *testing.T) {
	release := make(chan struct{})
	api := newFakeAPI(t, okProposal)
	api.tokenHandler = func(w http.ResponseWriter, r *http.Request, n int) {
		<-release
		writeJSON(w, http.StatusOK, map[string]interface{}{"token_de_acesso": "t", "expira_em_segundos": 3600})
	}
	client := api.client()
	defer close(release)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := client.V1.Proposals.Get(ctx, testUUID)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("esperava context.DeadlineExceeded, veio %T: %v", err, err)
	}
	var timeoutErr *credicarro.TimeoutError
	if errors.As(err, &timeoutErr) {
		t.Fatal("o timeout do contexto do chamador não deve virar TimeoutError do SDK")
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("a chamada deveria ter retornado assim que o contexto expirou")
	}
}

func TestCanceledContextReturnsContextError(t *testing.T) {
	client, _ := newTestClient(t, okProposal)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.V1.Proposals.Get(ctx, testUUID)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("esperava context.Canceled, veio %T: %v", err, err)
	}
}
