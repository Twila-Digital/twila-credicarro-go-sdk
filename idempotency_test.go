package credicarro_test

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	credicarro "github.com/Twila-Digital/twila-credicarro-go-sdk"
)

// failThenSucceed responde failures vezes com status e depois com a resposta de sucesso.
func failThenSucceed(failures int, status int, successStatus int, successBody string) http.HandlerFunc {
	var calls int32
	return func(w http.ResponseWriter, r *http.Request) {
		if int(atomic.AddInt32(&calls, 1)) <= failures {
			writeRaw(w, status, `{"tipo":"Falha.Transitoria","titulo":"Indisponível"}`)
			return
		}
		writeRaw(w, successStatus, successBody)
	}
}

func upsertRequest() credicarro.UpsertCustomerRequest {
	return credicarro.UpsertCustomerRequest{
		Cpf: "12345678909", FirstName: "Maria", LastName: "Souza", Email: "maria@exemplo.com.br",
		PhoneNumber: "+55 (11) 99876-5432", DateOfBirth: "1990-05-20T00:00:00-03:00",
		Address:          credicarro.CustomerAddress{Street: "Av. Paulista", Neighborhood: "Bela Vista", City: "São Paulo", State: "SP", PostalCode: "01311000"},
		ProfessionalData: credicarro.ProfessionalData{Occupation: credicarro.OccupationTypeEmployee, MonthlyIncome: 6500},
	}
}

func TestUnifiedWithoutKeyIsNeverRetriedNorGetsAnAutomaticKey(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusServiceUnavailable, `{"tipo":"Falha.Transitoria"}`)
	})

	_, err := client.Customers.Upsert(context.Background(), upsertRequest())
	var apiErr *credicarro.APIError
	if err == nil || !asAPIError(err, &apiErr) || apiErr.StatusCode != 503 {
		t.Fatalf("esperava APIError 503, veio %T: %v", err, err)
	}

	reqs := api.recorded()
	if len(reqs) != 1 {
		t.Fatalf("esperava exatamente 1 requisição (sem retry), veio %d", len(reqs))
	}
	if key := reqs[0].Header.Get("Idempotency-Key"); key != "" {
		t.Fatalf("o unified não pode receber chave automática, veio %q", key)
	}
}

func TestUnifiedWithExplicitKeySendsItAndIsRetriedWithTheSameKey(t *testing.T) {
	client, api := newTestClient(t, failThenSucceed(2, http.StatusServiceUnavailable, http.StatusOK, `{"id_cliente":"`+testUUID+`"}`))

	id, err := client.Customers.Upsert(context.Background(), upsertRequest(), credicarro.WithIdempotencyKey("minha-chave-123"))
	if err != nil || id != testUUID {
		t.Fatalf("esperava sucesso, veio %q (%v)", id, err)
	}

	reqs := api.recorded()
	if len(reqs) != 3 {
		t.Fatalf("esperava 3 tentativas, veio %d", len(reqs))
	}
	for i, req := range reqs {
		if req.Header.Get("Idempotency-Key") != "minha-chave-123" {
			t.Fatalf("tentativa %d: chave incorreta %q", i, req.Header.Get("Idempotency-Key"))
		}
	}
}

func TestAutomaticKeyIsGeneratedPerLogicalCallAndSharedByAllAttempts(t *testing.T) {
	cases := map[string]func(c *credicarro.Client) error{
		"customer": func(c *credicarro.Client) error {
			_, err := c.Customers.Create(context.Background(), credicarro.CreateCustomerRequest{Cpf: "12345678909"})
			return err
		},
		"address": func(c *credicarro.Client) error {
			return c.Customers.UpsertAddress(context.Background(), "12345678909", credicarro.CustomerAddress{Street: "Rua"})
		},
		"professional": func(c *credicarro.Client) error {
			return c.Customers.UpsertProfessionalData(context.Background(), "12345678909", credicarro.ProfessionalData{Occupation: credicarro.OccupationTypeRetired, MonthlyIncome: 1})
		},
		"financing v2": func(c *credicarro.Client) error {
			_, err := c.Simulations.CreateFinancing(context.Background(), credicarro.CreateFinancingSimulationRequest{Cpf: "12345678909", StoreCnpj: "11222333000181"})
			return err
		},
		"refinancing v2": func(c *credicarro.Client) error {
			_, err := c.Simulations.CreateRefinancing(context.Background(), credicarro.CreateRefinancingSimulationRequest{Cpf: "12345678909", StoreCnpj: "11222333000181"})
			return err
		},
		"financing v1": func(c *credicarro.Client) error {
			_, err := c.V1.Simulations.CreateFinancing(context.Background(), credicarro.V1CreateFinancingSimulationRequest{Cpf: "12345678909"})
			return err
		},
		"refinancing v1": func(c *credicarro.Client) error {
			_, err := c.V1.Simulations.CreateRefinancing(context.Background(), credicarro.V1CreateRefinancingSimulationRequest{Cpf: "12345678909"})
			return err
		},
		"proposal v2": func(c *credicarro.Client) error {
			_, err := c.Proposals.Create(context.Background(), credicarro.CreateProposalRequest{Cpf: "12345678909", SimulationID: testUUID2})
			return err
		},
		"proposal v1": func(c *credicarro.Client) error {
			_, err := c.V1.Proposals.Create(context.Background(), credicarro.CreateProposalRequest{Cpf: "12345678909", SimulationID: testUUID2})
			return err
		},
	}

	for name, call := range cases {
		call := call
		t.Run(name, func(t *testing.T) {
			client, api := newTestClient(t, failThenSucceed(2, http.StatusBadGateway, http.StatusOK, `{}`))

			if err := call(client); err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}

			reqs := api.recorded()
			if len(reqs) != 3 {
				t.Fatalf("esperava 3 tentativas (2 falhas + sucesso), veio %d", len(reqs))
			}
			key := reqs[0].Header.Get("Idempotency-Key")
			if key == "" {
				t.Fatal("esperava Idempotency-Key automática")
			}
			for i, req := range reqs {
				if req.Header.Get("Idempotency-Key") != key {
					t.Fatalf("tentativa %d: a chave deve ser a mesma em todas as tentativas", i)
				}
			}
		})
	}
}

func TestAutomaticKeyDiffersBetweenLogicalCalls(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusCreated, `{"id_proposta":"`+testUUID+`"}`)
	})

	for i := 0; i < 2; i++ {
		if _, err := client.Proposals.Create(context.Background(), credicarro.CreateProposalRequest{Cpf: "12345678909", SimulationID: testUUID2}); err != nil {
			t.Fatalf("erro inesperado: %v", err)
		}
	}
	reqs := api.recorded()
	if reqs[0].Header.Get("Idempotency-Key") == reqs[1].Header.Get("Idempotency-Key") {
		t.Fatal("cada chamada lógica deve gerar a própria chave")
	}
}

func TestExplicitKeyOverridesTheAutomaticOne(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusCreated, `{"id_proposta":"`+testUUID+`"}`)
	})

	if _, err := client.Proposals.Create(context.Background(), credicarro.CreateProposalRequest{Cpf: "12345678909", SimulationID: testUUID2}, credicarro.WithIdempotencyKey("chave-explicita")); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got := api.recorded()[0].Header.Get("Idempotency-Key"); got != "chave-explicita" {
		t.Fatalf("esperava a chave explícita, veio %q", got)
	}
}

func TestDisableAutomaticIdempotencyKeyStopsKeyAndRetryOnPosts(t *testing.T) {
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusServiceUnavailable, `{}`)
	})
	client := api.client(withResilience(func(r *credicarro.ResilienceOptions) { r.DisableAutomaticIdempotencyKey = true }))

	_, _ = client.Proposals.Create(context.Background(), credicarro.CreateProposalRequest{Cpf: "12345678909", SimulationID: testUUID2})

	reqs := api.recorded()
	if len(reqs) != 1 || reqs[0].Header.Get("Idempotency-Key") != "" {
		t.Fatalf("sem chave automática o POST não deve ser repetido nem ter chave: %d requisições, chave %q", len(reqs), reqs[0].Header.Get("Idempotency-Key"))
	}
}

func TestWebhookCreateIsNeverRetriedNorGetsAKey(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusServiceUnavailable, `{}`)
	})

	_, _ = client.Webhooks.Create(context.Background(), credicarro.CreateWebhookRequest{
		Type: credicarro.WebhookTypeProposal, URL: "https://exemplo.com.br/hook", AuthenticationType: credicarro.WebhookAuthenticationTypeNone,
	})

	reqs := api.recorded()
	if len(reqs) != 1 || reqs[0].Header.Get("Idempotency-Key") != "" {
		t.Fatalf("POST v1/webhook não pode ser repetido nem ter chave: %d requisições", len(reqs))
	}
}

func TestSubstitutionPostsAreRetriedWithoutKey(t *testing.T) {
	cases := map[string]func(c *credicarro.Client) error{
		"vehicle-detail": func(c *credicarro.Client) error {
			return c.Proposals.SendVehicleDetails(context.Background(), testUUID, credicarro.VehicleDetails{Renavam: "1"})
		},
		"deposit-account": func(c *credicarro.Client) error {
			return c.Proposals.SendDepositAccount(context.Background(), testUUID, credicarro.DepositAccount{AccountType: credicarro.DepositAccountTypeChecking})
		},
	}
	for name, call := range cases {
		call := call
		t.Run(name, func(t *testing.T) {
			client, api := newTestClient(t, failThenSucceed(1, http.StatusServiceUnavailable, http.StatusNoContent, ``))
			if err := call(client); err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}
			reqs := api.recorded()
			if len(reqs) != 2 {
				t.Fatalf("esperava 2 tentativas, veio %d", len(reqs))
			}
			if reqs[0].Header.Get("Idempotency-Key") != "" {
				t.Fatal("a rota não usa Idempotency-Key")
			}
		})
	}
}

func TestWebhookUpdateAndDeleteAreRetried(t *testing.T) {
	client, api := newTestClient(t, failThenSucceed(1, http.StatusServiceUnavailable, http.StatusNoContent, ``))
	if err := client.Webhooks.Update(context.Background(), testUUID, credicarro.UpdateWebhookRequest{Type: credicarro.WebhookTypeProposal, URL: "https://exemplo.com.br/h"}); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(api.recorded()) != 2 {
		t.Fatalf("PUT deveria ter sido repetido: %d requisições", len(api.recorded()))
	}

	client, api = newTestClient(t, failThenSucceed(1, http.StatusServiceUnavailable, http.StatusNoContent, ``))
	if err := client.Webhooks.Delete(context.Background(), testUUID); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if len(api.recorded()) != 2 {
		t.Fatalf("DELETE deveria ter sido repetido: %d requisições", len(api.recorded()))
	}
}

func TestDocumentUploadReusesTheSameKeyAcrossAttempts(t *testing.T) {
	client, api := newTestClient(t, failThenSucceed(2, http.StatusBadGateway, http.StatusNoContent, ``))

	file, err := credicarro.NewDocumentFileFromBytes([]byte("%PDF-1.7 conteudo"), "cnh-comprador.pdf")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	err = client.Proposals.UploadDocument(context.Background(), testUUID, credicarro.ProposalDocumentUpload{
		Participant: credicarro.ProposalParticipantBuyer, DocumentType: credicarro.DocumentTypeCnh, File: file,
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	reqs := api.recorded()
	if len(reqs) != 3 {
		t.Fatalf("esperava 3 tentativas, veio %d", len(reqs))
	}
	key := reqs[0].Header.Get("Idempotency-Key")
	if key == "" {
		t.Fatal("o envio de documento exige Idempotency-Key")
	}
	for i, req := range reqs {
		if req.Header.Get("Idempotency-Key") != key {
			t.Fatalf("tentativa %d: a chave deve ser a mesma", i)
		}
	}
}

func TestDocumentUploadUsesExplicitKeyAndStillGeneratesOneWhenAutomaticKeysAreDisabled(t *testing.T) {
	file, _ := credicarro.NewDocumentFileFromBytes([]byte("conteudo"), "doc.png")
	upload := credicarro.ProposalDocumentUpload{Participant: credicarro.ProposalParticipantStore, DocumentType: credicarro.DocumentTypeEcvReport, File: file}

	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) })
	client := api.client(withResilience(func(r *credicarro.ResilienceOptions) { r.DisableAutomaticIdempotencyKey = true }))

	if err := client.Proposals.UploadDocument(context.Background(), testUUID, upload, credicarro.WithIdempotencyKey("envio-42")); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if err := client.Proposals.UploadDocument(context.Background(), testUUID, upload); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	reqs := api.recorded()
	if reqs[0].Header.Get("Idempotency-Key") != "envio-42" {
		t.Fatalf("esperava a chave explícita, veio %q", reqs[0].Header.Get("Idempotency-Key"))
	}
	if reqs[1].Header.Get("Idempotency-Key") == "" {
		t.Fatal("o header é obrigatório no envio de documento: o SDK deve gerar a chave mesmo com as chaves automáticas desligadas")
	}
}

func TestIdempotencyKeyLongerThan200CharactersIsRejected(t *testing.T) {
	client, api := newTestClient(t, nil)
	long := make([]byte, 201)
	for i := range long {
		long[i] = 'a'
	}
	_, err := client.Customers.Upsert(context.Background(), upsertRequest(), credicarro.WithIdempotencyKey(string(long)))
	assertConfigurationError(t, err)
	if len(api.recorded()) != 0 {
		t.Fatal("nenhuma requisição deveria ser enviada")
	}
}

func TestGetIsRetriedOnTransientStatusesAndNeverOnOther4xx(t *testing.T) {
	for _, status := range []int{408, 429, 500, 502, 503, 504} {
		status := status
		t.Run(http.StatusText(status), func(t *testing.T) {
			client, api := newTestClient(t, failThenSucceed(1, status, http.StatusOK, `{"id_proposta":"`+testUUID+`"}`))
			if _, err := client.V1.Proposals.Get(context.Background(), testUUID); err != nil {
				t.Fatalf("status %d: erro inesperado: %v", status, err)
			}
			if len(api.recorded()) != 2 {
				t.Fatalf("status %d: esperava 2 tentativas, veio %d", status, len(api.recorded()))
			}
		})
	}

	for _, status := range []int{400, 401, 403, 404, 409, 422} {
		status := status
		t.Run(http.StatusText(status), func(t *testing.T) {
			client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				writeRaw(w, status, `{"tipo":"X.Y.Z"}`)
			})
			_, _ = client.V1.Proposals.Get(context.Background(), testUUID)
			expected := 1
			if status == 401 {
				expected = 2
			}
			if len(api.recorded()) != expected {
				t.Fatalf("status %d: esperava %d requisições, veio %d", status, expected, len(api.recorded()))
			}
		})
	}
}

func TestStatus500IsNotRetriedWhenRetryOn500IsDisabled(t *testing.T) {
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) { writeRaw(w, http.StatusInternalServerError, `{}`) })
	client := api.client(withResilience(func(r *credicarro.ResilienceOptions) { r.RetryOn500 = false }))

	_, _ = client.V1.Proposals.Get(context.Background(), testUUID)
	if len(api.recorded()) != 1 {
		t.Fatalf("esperava 1 requisição, veio %d", len(api.recorded()))
	}
}

func TestRetryAfterOf429IsRespected(t *testing.T) {
	var mu sync.Mutex
	var arrivals []time.Time
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		arrivals = append(arrivals, time.Now())
		n := len(arrivals)
		mu.Unlock()
		if n == 1 {
			w.Header().Set("Retry-After", "1")
			writeRaw(w, http.StatusTooManyRequests, `{}`)
			return
		}
		writeRaw(w, http.StatusOK, `{"id_proposta":"`+testUUID+`"}`)
	})

	if _, err := client.V1.Proposals.Get(context.Background(), testUUID); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(arrivals) != 2 {
		t.Fatalf("esperava 2 tentativas, veio %d", len(arrivals))
	}
	if gap := arrivals[1].Sub(arrivals[0]); gap < 900*time.Millisecond {
		t.Fatalf("o Retry-After de 1 s deveria ser respeitado, esperou %v", gap)
	}
}

func TestAttemptTimeoutIsRetriedForSafeRequests(t *testing.T) {
	var calls int32
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			time.Sleep(400 * time.Millisecond)
		}
		writeRaw(w, http.StatusOK, `{"id_proposta":"`+testUUID+`"}`)
	})
	client := api.client(withResilience(func(r *credicarro.ResilienceOptions) {
		r.AttemptTimeout = 150 * time.Millisecond
		r.TotalTimeout = 3 * time.Second
	}))

	if _, err := client.V1.Proposals.Get(context.Background(), testUUID); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("esperava 2 tentativas, veio %d", atomic.LoadInt32(&calls))
	}
}

func TestExhaustedAttemptTimeoutBecomesTimeoutError(t *testing.T) {
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		writeRaw(w, http.StatusOK, `{}`)
	})
	client := api.client(withResilience(func(r *credicarro.ResilienceOptions) {
		r.AttemptTimeout = 60 * time.Millisecond
		r.TotalTimeout = time.Second
		r.MaxRetryAttempts = 2
	}))

	_, err := client.V1.Proposals.Get(context.Background(), testUUID)
	var timeoutErr *credicarro.TimeoutError
	if !asTimeoutError(err, &timeoutErr) {
		t.Fatalf("esperava *TimeoutError, veio %T: %v", err, err)
	}
}

func TestCircuitBreakerOpenBecomesTimeoutError(t *testing.T) {
	api := newFakeAPI(t, func(w http.ResponseWriter, r *http.Request) { writeRaw(w, http.StatusServiceUnavailable, `{}`) })
	client := api.client(withResilience(func(r *credicarro.ResilienceOptions) {
		r.MaxRetryAttempts = 1
		r.CircuitBreakerMinThroughput = 2
		r.CircuitBreakerBreakDuration = time.Minute
	}))

	for i := 0; i < 2; i++ {
		_, _ = client.V1.Proposals.Get(context.Background(), testUUID)
	}
	before := len(api.recorded())
	_, err := client.V1.Proposals.Get(context.Background(), testUUID)

	var timeoutErr *credicarro.TimeoutError
	if !asTimeoutError(err, &timeoutErr) {
		t.Fatalf("esperava *TimeoutError com o circuit breaker aberto, veio %T: %v", err, err)
	}
	if len(api.recorded()) != before {
		t.Fatal("com o circuit breaker aberto nenhuma requisição deveria sair")
	}
}
