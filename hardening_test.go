package credicarro_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	credicarro "github.com/Twila-Digital/twila-credicarro-go-sdk"
)

func TestRetryAfterLargerThanRemainingBudgetReturnsRateLimitImmediately(t *testing.T) {
	for _, value := range []string{"3600", "1e300", "Inf", "99999999999999999999"} {
		value := value
		t.Run(value, func(t *testing.T) {
			client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Retry-After", value)
				writeRaw(w, http.StatusTooManyRequests, `{"tipo":"RateLimit.Exceeded"}`)
			})

			start := time.Now()
			err := getProposal(client)
			var limit *credicarro.RateLimitError
			if !errors.As(err, &limit) {
				t.Fatalf("esperava *RateLimitError, veio %T: %v", err, err)
			}
			if time.Since(start) > 2*time.Second || len(api.recorded()) != 1 {
				t.Fatalf("não deveria esperar nem repetir: %v, %d requisições", time.Since(start), len(api.recorded()))
			}
			if limit.RetryAfter <= 0 || limit.RetryAfter > 366*24*time.Hour {
				t.Fatalf("RetryAfter deveria ser positivo e limitado: %v", limit.RetryAfter)
			}
		})
	}
}

func TestInvalidRetryAfterIsIgnoredAndBackoffApplies(t *testing.T) {
	for _, value := range []string{"-5", "0", "abc", "NaN", ""} {
		value := value
		t.Run(value, func(t *testing.T) {
			var calls int32
			c2, a2 := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				if atomic.AddInt32(&calls, 1) == 1 {
					if value != "" {
						w.Header().Set("Retry-After", value)
					}
					writeRaw(w, http.StatusTooManyRequests, `{}`)
					return
				}
				writeRaw(w, http.StatusOK, `{"id_proposta":"`+testUUID+`"}`)
			})
			if err := getProposal(c2); err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			if len(a2.recorded()) != 2 {
				t.Fatalf("esperava 2 tentativas, veio %d", len(a2.recorded()))
			}
		})
	}
}

func TestRedirectBecomesAPIErrorAndIsNeverFollowed(t *testing.T) {
	var targetHits int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&targetHits, 1)
	}))
	defer target.Close()

	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/roubo", http.StatusFound)
	})

	err := getProposal(client)
	var apiErr *credicarro.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusFound {
		t.Fatalf("esperava *APIError 302, veio %T: %v", err, err)
	}
	if atomic.LoadInt32(&targetHits) != 0 || len(api.recorded()) != 1 {
		t.Fatalf("o redirect não pode ser seguido (destino=%d, origem=%d)", targetHits, len(api.recorded()))
	}
}

func TestTokenRedirectIsNeverFollowed(t *testing.T) {
	var targetHits int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { atomic.AddInt32(&targetHits, 1) }))
	defer target.Close()

	api := newFakeAPI(t, okProposal)
	api.tokenHandler = func(w http.ResponseWriter, r *http.Request, n int) {
		http.Redirect(w, r, target.URL+"/token", http.StatusTemporaryRedirect)
	}
	err := getProposal(api.client())
	var apiErr *credicarro.APIError
	var authErr *credicarro.AuthenticationError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusTemporaryRedirect || errors.As(err, &authErr) {
		t.Fatalf("esperava *APIError 307 (não AuthenticationError), veio %T: %v", err, err)
	}
	if atomic.LoadInt32(&targetHits) != 0 {
		t.Fatal("o redirect do token não pode ser seguido")
	}
}

type countingTransport struct {
	calls int32
	base  http.RoundTripper
}

func (c *countingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	atomic.AddInt32(&c.calls, 1)
	return c.base.RoundTrip(r)
}

func TestInjectedHTTPClientKeepsTransportButForcesNoRedirects(t *testing.T) {
	var targetHits int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { atomic.AddInt32(&targetHits, 1) }))
	defer target.Close()

	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/x", http.StatusFound)
	})
	counter := &countingTransport{base: http.DefaultTransport}
	injected := &http.Client{
		Transport:     counter,
		CheckRedirect: func(*http.Request, []*http.Request) error { return nil },
	}
	client := api.client(func(o *credicarro.ClientOptions) { o.HTTPClient = injected })

	err := getProposal(client)
	var apiErr *credicarro.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusFound {
		t.Fatalf("esperava *APIError 302, veio %T: %v", err, err)
	}
	if atomic.LoadInt32(&counter.calls) < 2 {
		t.Fatalf("o Transport do integrador deveria ser usado (token + recurso), veio %d", counter.calls)
	}
	if atomic.LoadInt32(&targetHits) != 0 {
		t.Fatal("o SDK deve forçar CheckRedirect mesmo com HTTPClient injetado")
	}
	if injected.CheckRedirect == nil {
		t.Fatal("o cliente do integrador não pode ser alterado")
	}
}

func TestNetworkFailureBecomesTimeoutErrorWithSanitizedCause(t *testing.T) {
	api := newFakeAPI(t, nil)
	client := api.client(withResilience(func(r *credicarro.ResilienceOptions) { r.MaxRetryAttempts = 2 }))
	if _, err := client.Simulations.GetFinancingRange(context.Background(), "ABC1D23", "11222333000181"); err == nil {
		t.Fatal("esperava 404")
	}
	api.server.Close()

	_, err := client.Simulations.GetFinancingRange(context.Background(), "ABC1D23", "11222333000181")
	var timeoutErr *credicarro.TimeoutError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("esperava *TimeoutError, veio %T: %v", err, err)
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		t.Fatal("a causa não pode conter o *url.Error (requisição/URL)")
	}
	if text := fmt.Sprintf("%v %+v %#v", err, err, timeoutErr.Cause); strings.Contains(text, "ABC1D23") || strings.Contains(text, "11222333000181") || strings.Contains(text, "Bearer") {
		t.Fatalf("a falha vazou dados da requisição: %q", text)
	}
	var netErr net.Error
	if !errors.As(err, &netErr) {
		t.Fatal("a causa sanitizada deveria continuar sendo um net.Error")
	}
}

func TestTokenNetworkFailureBecomesTimeoutErrorNotAuthenticationError(t *testing.T) {
	api := newFakeAPI(t, okProposal)
	client := api.client(withResilience(func(r *credicarro.ResilienceOptions) { r.MaxRetryAttempts = 2 }))
	api.server.Close()

	err := getProposal(client)
	var timeoutErr *credicarro.TimeoutError
	var authErr *credicarro.AuthenticationError
	if !errors.As(err, &timeoutErr) || errors.As(err, &authErr) {
		t.Fatalf("esperava *TimeoutError, veio %T: %v", err, err)
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		t.Fatal("a causa não pode conter o *url.Error")
	}
}

func TestExplicitIdempotencyKeyValidation(t *testing.T) {
	client, api := newTestClient(t, nil)
	invalid := map[string]string{
		"vazia":          "",
		"só espaços":     "   ",
		"201 caracteres": strings.Repeat("a", 201),
		"não ASCII":      "chave-ção",
		"com controle":   "chave\n1",
		"com DEL":        "chave\x7f",
	}
	for name, key := range invalid {
		_, err := client.Proposals.Create(context.Background(), credicarro.CreateProposalRequest{Cpf: "12345678909", SimulationID: testUUID2}, credicarro.WithIdempotencyKey(key))
		if err == nil {
			t.Errorf("%s: a chave deveria ser rejeitada", name)
			continue
		}
		assertConfigurationError(t, err)
	}
	if len(api.recorded()) != 0 {
		t.Fatal("nenhuma requisição deveria ser enviada")
	}

	ok, api2 := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusCreated, `{"id_proposta":"`+testUUID+`"}`)
	})
	for _, key := range []string{"a", strings.Repeat("k", 200), "envio 42 ~!"} {
		if _, err := ok.Proposals.Create(context.Background(), credicarro.CreateProposalRequest{Cpf: "12345678909", SimulationID: testUUID2}, credicarro.WithIdempotencyKey(key)); err != nil {
			t.Errorf("chave %q deveria ser aceita: %v", key, err)
		}
	}
	if len(api2.recorded()) != 3 {
		t.Fatalf("esperava 3 requisições, veio %d", len(api2.recorded()))
	}
}

func TestDocumentUploadBudgetIsAttemptTimeoutTimesMaxAttempts(t *testing.T) {
	var calls int32
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		if atomic.AddInt32(&calls, 1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	client := api.client(withResilience(func(r *credicarro.ResilienceOptions) {
		r.TotalTimeout = 300 * time.Millisecond
		r.AttemptTimeout = 200 * time.Millisecond
		r.DocumentUploadAttemptTimeout = 400 * time.Millisecond
	}))

	file, _ := credicarro.NewDocumentFileFromBytes([]byte("x"), "a.pdf")
	err := client.Proposals.UploadDocument(context.Background(), testUUID, credicarro.ProposalDocumentUpload{
		Participant: credicarro.ProposalParticipantBuyer, DocumentType: credicarro.DocumentTypeCnh, File: file,
	})
	if err != nil {
		t.Fatalf("o envio deveria caber no orçamento tentativa × tentativas: %v", err)
	}
}

func TestOtherCallsStillUseTotalTimeout(t *testing.T) {
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	client := api.client(withResilience(func(r *credicarro.ResilienceOptions) {
		r.TotalTimeout = 300 * time.Millisecond
		r.AttemptTimeout = 250 * time.Millisecond
		r.DocumentUploadAttemptTimeout = 5 * time.Second
	}))
	start := time.Now()
	err := getProposal(client)
	if err == nil || time.Since(start) > 2*time.Second {
		t.Fatalf("o orçamento total deveria limitar a chamada: %v em %v", err, time.Since(start))
	}
}

func TestNewClientRejectsNewResilienceFieldsAtZero(t *testing.T) {
	mutations := map[string]func(*credicarro.ResilienceOptions){
		"upload timeout": func(r *credicarro.ResilienceOptions) { r.DocumentUploadAttemptTimeout = 0 },
		"base delay":     func(r *credicarro.ResilienceOptions) { r.RetryBaseDelay = 0 },
		"sampling":       func(r *credicarro.ResilienceOptions) { r.CircuitBreakerSamplingWindow = 0 },
		"break duration": func(r *credicarro.ResilienceOptions) { r.CircuitBreakerBreakDuration = 0 },
	}
	for name, mutate := range mutations {
		resilience := credicarro.DefaultResilienceOptions()
		mutate(&resilience)
		_, err := credicarro.NewClient(credicarro.ClientOptions{ClientID: "id", ClientSecret: "s", Resilience: &resilience})
		if err == nil {
			t.Errorf("%s: esperava erro de configuração", name)
			continue
		}
		assertConfigurationError(t, err)
	}
}

func TestSensitiveTypesAreMaskedInEveryFormat(t *testing.T) {
	const cpf = "12345678909"
	secrets := []string{cpf, "maria@exemplo.com.br", "99876-5432", "6500", "Maria", "Souza", "Av. Paulista", "01311000", "123456", "1990-05-20", "base64conteudo"}

	file, _ := credicarro.NewDocumentFileFromBytes([]byte("base64conteudo"), "cnh.pdf")
	buyer := &credicarro.ProposalParticipantState{Document: cpf}
	values := []interface{}{
		upsertRequest(),
		&credicarro.CreateCustomerRequest{Cpf: cpf, FirstName: "Maria", LastName: "Souza", Email: "maria@exemplo.com.br", PhoneNumber: "+55 (11) 99876-5432", DateOfBirth: "1990-05-20T00:00:00-03:00"},
		credicarro.CustomerAddress{Street: "Av. Paulista", PostalCode: "01311000", City: "São Paulo"},
		credicarro.ProfessionalData{Occupation: credicarro.OccupationTypeEmployee, MonthlyIncome: 6500},
		credicarro.DepositAccount{HolderCpf: cpf, BankCode: "341", Branch: "1234", AccountNumber: "123456", AccountDigit: "7", AccountType: credicarro.DepositAccountTypeChecking},
		credicarro.CreateProposalRequest{Cpf: cpf, SimulationID: testUUID},
		credicarro.CreateFinancingSimulationRequest{Cpf: cpf},
		credicarro.CreateRefinancingSimulationRequest{Cpf: cpf},
		credicarro.V1CreateFinancingSimulationRequest{Cpf: cpf},
		credicarro.V1CreateRefinancingSimulationRequest{Cpf: cpf},
		credicarro.ProposalDocumentUpload{Participant: credicarro.ProposalParticipantBuyer, DocumentType: credicarro.DocumentTypeCnh, File: file},
		file,
		credicarro.ProposalSummary{BuyerCpf: cpf, SellerCpf: cpf},
		credicarro.Simulation{CustomerCpf: cpf},
		buyer,
		&credicarro.ProposalDetails{Buyer: buyer, Seller: buyer, Store: buyer},
		&credicarro.ProposalWebhookEvent{Buyer: buyer, Seller: buyer},
	}

	for _, value := range values {
		for _, text := range []string{fmt.Sprint(value), fmt.Sprintf("%v", value), fmt.Sprintf("%+v", value), fmt.Sprintf("%#v", value), fmt.Sprintf("%s", value)} { //nolint:gosimple
			for _, secret := range secrets {
				if strings.Contains(text, secret) {
					t.Errorf("%T vazou %q em %q", value, secret, text)
				}
			}
		}
	}

	if got := fmt.Sprint(credicarro.CreateProposalRequest{Cpf: cpf, SimulationID: testUUID}); !strings.Contains(got, "***.456.789-**") {
		t.Errorf("o CPF deveria aparecer no formato ***.456.789-**: %q", got)
	}
}

func TestErrorFieldsWithRawBodyStayOutOfFormattedOutput(t *testing.T) {
	body := `{"tipo":"X.Y","titulo":"T","detalhe":"CPF 12345678909 inválido"}`
	err := getProposal(errorClient(t, http.StatusNotFound, body, nil))
	var apiErr *credicarro.APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("veio %T", err)
	}
	for _, text := range []string{fmt.Sprint(err), fmt.Sprintf("%+v", err), fmt.Sprintf("%#v", apiErr), fmt.Sprintf("%#v", err)} {
		if strings.Contains(text, "12345678909") {
			t.Errorf("detalhe vazou em %q", text)
		}
	}

	authErr := &credicarro.AuthenticationError{Message: "m", Detail: "CPF 12345678909", Body: "12345678909"}
	if text := fmt.Sprintf("%#v %v", authErr, authErr); strings.Contains(text, "12345678909") {
		t.Errorf("detalhe vazou em %q", text)
	}
}

func TestWebhookEventPendingActionsFromDetailedPayload(t *testing.T) {
	event, err := credicarro.ParseWebhookEventUnverified([]byte(v2Body()))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got := len(event.PendingActions()); got != 2 {
		t.Fatalf("esperava 2 ações, veio %d", got)
	}
	v1, _ := credicarro.ParseWebhookEventUnverified([]byte(docBody))
	if got := len(v1.PendingActions()); got != 0 {
		t.Fatalf("payload v1 não tem participantes, veio %d ações", got)
	}
}

func TestNegativeWebhookToleranceIsRejected(t *testing.T) {
	header := signedHeader(docSecret, time.Now().Unix(), docBody)
	if credicarro.VerifyWebhookSignature([]byte(docBody), header, docSecret, -time.Second) {
		t.Fatal("tolerância negativa deveria ser rejeitada")
	}
}
