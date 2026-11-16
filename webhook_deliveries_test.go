package credicarro_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	credicarro "github.com/Twila-Digital/twila-credicarro-go-sdk"
)

const testAuditID = "7c9e6679-7425-40de-944b-e07fc1f90ae7"

func TestWebhooksListDeliveriesSendsFiltersAndMapsItems(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusOK, `{"itens":[
			{"id_auditoria":"`+testAuditID+`","id_webhook":"`+testUUID+`","tipo":"Proposal","requisicao":"{\"id_proposta\":\"x\"}","resposta":"ok","status_http":200,"data_criacao":"2026-11-05T14:32:10Z"},
			{"id_auditoria":"`+testAuditID+`","id_webhook":null,"tipo":"Algo","requisicao":"{}","resposta":"","status_http":null,"data_criacao":"2026-11-05T14:33:00Z"}
		],"pagina":{"tem_proximo":true,"tem_anterior":false,"numero":2,"tamanho":50,"total":51}}`)
	})
	status := 500

	page, err := client.Webhooks.ListDeliveries(context.Background(), &credicarro.ListWebhookDeliveriesRequest{
		WebhookDeliveryFilter: credicarro.WebhookDeliveryFilter{
			InitialDate: "2026-11-01", FinalDate: "2026-11-05T23:59:59-03:00", ProposalNumber: "12345", StatusCode: &status,
		},
		Page: 2, PageSize: 50,
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	req := api.recorded()[0]
	query, _ := url.ParseQuery(req.RawQuery)
	expected := url.Values{
		"data_inicial": {"2026-11-01"}, "data_final": {"2026-11-05T23:59:59-03:00"}, "numero_proposta": {"12345"},
		"status_http": {"500"}, "pagina_inicial": {"2"}, "limite": {"50"},
	}
	if req.Method != http.MethodGet || req.Path != "v1/webhook/audit" || !reflect.DeepEqual(query, expected) {
		t.Fatalf("requisição incorreta: %s %s %v", req.Method, req.Path, query)
	}

	first, second := page.Items[0], page.Items[1]
	if first.ID != testAuditID || first.WebhookID != testUUID || first.Type != credicarro.WebhookTypeProposal ||
		first.StatusCode == nil || *first.StatusCode != 200 || first.Response != "ok" || first.CreatedAt != "2026-11-05T14:32:10Z" {
		t.Fatalf("primeiro item incorreto: %#v", first)
	}
	if second.WebhookID != "" || second.Type != credicarro.WebhookTypeUnknown || second.TypeRaw != "Algo" || second.StatusCode != nil {
		t.Fatalf("segundo item incorreto: %#v", second)
	}
	if page.Page != 2 || page.TotalCount != 51 || !page.HasNext {
		t.Fatalf("paginação incorreta: %+v", page)
	}
	for _, text := range []string{fmt.Sprint(first), fmt.Sprintf("%+v", first), fmt.Sprintf("%#v", first)} {
		if strings.Contains(text, "id_proposta") {
			t.Errorf("corpo do envio vazou em %q", text)
		}
	}
}

func TestWebhooksListDeliveriesWithoutFiltersSendsDefaultPaging(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusOK, `{"itens":[],"pagina":{"numero":1,"tamanho":10,"total":0}}`)
	})
	if _, err := client.Webhooks.ListDeliveries(context.Background(), nil); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if query := api.recorded()[0].RawQuery; query != "pagina_inicial=1&limite=10" {
		t.Fatalf("query incorreta: %q", query)
	}
}

func TestWebhooksListDeliveriesValidatesPaging(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	_, err := client.Webhooks.ListDeliveries(context.Background(), &credicarro.ListWebhookDeliveriesRequest{PageSize: 101})
	var config *credicarro.ConfigurationError
	if !errors.As(err, &config) || len(api.recorded()) != 0 {
		t.Fatalf("esperava ConfigurationError sem chamar a API, veio %T: %v", err, err)
	}
}

func TestWebhooksGetDeliveryChartSendsPeriodAndMapsPoints(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusOK, `[{"rotulo":"04","sucessos":42,"erros":0},{"rotulo":"05","sucessos":38,"erros":3}]`)
	})

	points, err := client.Webhooks.GetDeliveryChart(context.Background(), credicarro.WebhookDeliveryChartRequest{
		WebhookDeliveryFilter: credicarro.WebhookDeliveryFilter{InitialDate: "2026-11-04", FinalDate: "2026-11-05", ProposalID: testUUID},
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	query, _ := url.ParseQuery(api.recorded()[0].RawQuery)
	expected := url.Values{"data_inicial": {"2026-11-04"}, "data_final": {"2026-11-05"}, "id_proposta": {testUUID}}
	if api.recorded()[0].Path != "v1/webhook/audit/chart" || !reflect.DeepEqual(query, expected) {
		t.Fatalf("requisição incorreta: %v", query)
	}
	want := []credicarro.WebhookDeliveryChartPoint{{Label: "04", SuccessCount: 42}, {Label: "05", SuccessCount: 38, ErrorCount: 3}}
	if !reflect.DeepEqual(points, want) {
		t.Fatalf("pontos incorretos: %+v", points)
	}
}

func TestWebhooksGetDeliveryChartRequiresThePeriod(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	_, err := client.Webhooks.GetDeliveryChart(context.Background(), credicarro.WebhookDeliveryChartRequest{
		WebhookDeliveryFilter: credicarro.WebhookDeliveryFilter{InitialDate: "2026-11-04"},
	})
	var config *credicarro.ConfigurationError
	if !errors.As(err, &config) || len(api.recorded()) != 0 {
		t.Fatalf("esperava ConfigurationError sem chamar a API, veio %T: %v", err, err)
	}
}

func TestWebhooksGetDeliveryChartMapsDateRangeTooLarge(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusBadRequest, `{"tipo":"Organization.Webhook.Audit.DateRangeTooLarge","titulo":"Requisição inválida","status":400}`)
	})
	_, err := client.Webhooks.GetDeliveryChart(context.Background(), credicarro.WebhookDeliveryChartRequest{
		WebhookDeliveryFilter: credicarro.WebhookDeliveryFilter{InitialDate: "2026-01-01", FinalDate: "2026-03-01"},
	})
	var validation *credicarro.ValidationError
	if !errors.As(err, &validation) || validation.ErrorCode != "Organization.Webhook.Audit.DateRangeTooLarge" {
		t.Fatalf("esperava ValidationError, veio %T: %v", err, err)
	}
}

func TestWebhooksResendPostsWithoutBodyOrIdempotencyKey(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusOK, `{"status_http":null,"resposta":""}`)
	})

	result, err := client.Webhooks.Resend(context.Background(), testAuditID)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	req := api.recorded()[0]
	if req.Method != http.MethodPost || req.Path != "v1/webhook/audit/"+testAuditID+"/resend" || len(req.Body) != 0 ||
		req.Header.Get("Idempotency-Key") != "" {
		t.Fatalf("requisição incorreta: %s %s corpo=%q chave=%q", req.Method, req.Path, req.Body, req.Header.Get("Idempotency-Key"))
	}
	if result.StatusCode != nil || result.Response != "" {
		t.Fatalf("resultado incorreto: %+v", result)
	}
}

func TestWebhooksResendIsNotRetriedOn503(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusServiceUnavailable, `{"tipo":"Organization.Webhook.Resend.DependencyUnavailable","titulo":"Indisponível","status":503}`)
	})
	_, err := client.Webhooks.Resend(context.Background(), testAuditID)
	var apiErr *credicarro.APIError
	if !errors.As(err, &apiErr) || apiErr.ErrorCode != "Organization.Webhook.Resend.DependencyUnavailable" {
		t.Fatalf("esperava APIError, veio %T: %v", err, err)
	}
	if calls := len(api.recorded()); calls != 1 {
		t.Fatalf("o reenvio não pode ser repetido: %d chamadas", calls)
	}
}

func TestWebhooksResendRequiresUUID(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})
	_, err := client.Webhooks.Resend(context.Background(), "nao-e-uuid")
	var config *credicarro.ConfigurationError
	if !errors.As(err, &config) || len(api.recorded()) != 0 {
		t.Fatalf("esperava ConfigurationError sem chamar a API, veio %T: %v", err, err)
	}
}

func TestSimulationsGetCommissionReturnsMapsRanges(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusOK, `{"retornos_comissao":[
			{"tipo":"Financing","alvo":"Store","retorno_minimo":0,"retorno_maximo":4,"permite_taxa_padrao":true},
			{"tipo":"Outro","alvo":"Novo","retorno_minimo":1,"retorno_maximo":3,"permite_taxa_padrao":false}
		]}`)
	})

	returns, err := client.Simulations.GetCommissionReturns(context.Background(), "11.222.333/0001-81")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	req := api.recorded()[0]
	query, _ := url.ParseQuery(req.RawQuery)
	if req.Path != "v2/proposal/simulation/commission-returns" || query.Get("cnpjLoja") != "11.222.333/0001-81" {
		t.Fatalf("requisição incorreta: %s %v", req.Path, query)
	}
	want := []credicarro.CommissionReturn{
		{Type: credicarro.SimulationTypeFinancing, TypeRaw: "Financing", Target: credicarro.CommissionReturnTargetStore, TargetRaw: "Store", MaximumReturn: 4, AllowsDefaultRate: true},
		{Type: credicarro.SimulationTypeUnknown, TypeRaw: "Outro", Target: credicarro.CommissionReturnTargetUnknown, TargetRaw: "Novo", MinimumReturn: 1, MaximumReturn: 3},
	}
	if !reflect.DeepEqual(returns, want) {
		t.Fatalf("faixas incorretas: %+v", returns)
	}
}

func TestSimulationsSendCommissionReturnOnlyWhenProvided(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusCreated, simulationOptionsJSON)
	})
	zero, two := 0, 2

	ctx := context.Background()
	if _, err := client.Simulations.CreateFinancing(ctx, credicarro.CreateFinancingSimulationRequest{
		Cpf: "12345678909", LicensePlate: "ABC1D23", VehicleAmount: 45000, DownPayment: 5000, State: "SP", StoreCnpj: "11222333000181", CommissionReturn: &two,
	}); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if _, err := client.Simulations.CreateRefinancing(ctx, credicarro.CreateRefinancingSimulationRequest{
		Cpf: "12345678909", LicensePlate: "ABC1D23", RequestedAmount: 25000, StoreCnpj: "11222333000181", CommissionReturn: &zero,
	}); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if _, err := client.Simulations.CreateRefinancing(ctx, credicarro.CreateRefinancingSimulationRequest{
		Cpf: "12345678909", LicensePlate: "ABC1D23", RequestedAmount: 25000, StoreCnpj: "11222333000181",
	}); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	requests := api.recorded()
	if got := requests[0].JSON(t)["retornoComissao"]; got != float64(2) {
		t.Fatalf("financiamento deveria enviar retornoComissao 2, veio %v", got)
	}
	if got := requests[1].JSON(t)["retornoComissao"]; got != float64(0) {
		t.Fatalf("refinanciamento deveria enviar retornoComissao 0, veio %v", got)
	}
	if _, present := requests[2].JSON(t)["retornoComissao"]; present {
		t.Fatal("retornoComissao nil não deve ser enviado")
	}
}

func TestSimulationsMapCommissionReturnOutOfRangeAsBusinessRule(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusUnprocessableEntity, `{"tipo":"Store.CommissionReturn.OutOfRange","titulo":"Regra de negócio não atendida","status":422}`)
	})
	nine := 9
	_, err := client.Simulations.CreateFinancing(context.Background(), credicarro.CreateFinancingSimulationRequest{
		Cpf: "12345678909", LicensePlate: "ABC1D23", VehicleAmount: 45000, DownPayment: 5000, State: "SP", StoreCnpj: "11222333000181", CommissionReturn: &nine,
	})
	var rule *credicarro.BusinessRuleError
	if !asBusinessRule(err, &rule) || rule.ErrorCode != "Store.CommissionReturn.OutOfRange" {
		t.Fatalf("esperava BusinessRuleError, veio %T: %v", err, err)
	}
}
