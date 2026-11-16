package credicarro_test

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strconv"
	"testing"

	credicarro "github.com/Twila-Digital/twila-credicarro-go-sdk"
)

func TestV1SimulationsRangesHaveNoStoreCnpj(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/integration/v1/proposal/simulation/range/financing" {
			writeRaw(w, http.StatusOK, `{"opcoes_por_prazo":[{"prazo":12,"valor_minimo_entrada":5000}]}`)
			return
		}
		writeRaw(w, http.StatusOK, `{"opcoes_por_prazo":[{"prazo":6,"valor_minimo_refinanciavel":10,"valor_maximo_refinanciavel":20}]}`)
	})

	financing, err := client.V1.Simulations.GetFinancingRange(context.Background(), "ABC-1234")
	if err != nil || !reflect.DeepEqual(financing, []credicarro.FinancingRangeOption{{Term: 12, MinimumDownPayment: 5000}}) {
		t.Fatalf("financiamento incorreto: %+v (%v)", financing, err)
	}
	refinancing, err := client.V1.Simulations.GetRefinancingRange(context.Background(), "ABC1D23")
	if err != nil || !reflect.DeepEqual(refinancing, []credicarro.RefinancingRangeOption{{Term: 6, MinimumAmount: 10, MaximumAmount: 20}}) {
		t.Fatalf("refinanciamento incorreto: %+v (%v)", refinancing, err)
	}

	reqs := api.recorded()
	if reqs[0].Path != "v1/proposal/simulation/range/financing" || reqs[0].RawQuery != "placa=ABC-1234" ||
		reqs[1].Path != "v1/proposal/simulation/range/refinancing" || reqs[1].RawQuery != "placa=ABC1D23" {
		t.Fatalf("requisições incorretas: %+v", reqs)
	}

	_, err = client.V1.Simulations.GetFinancingRange(context.Background(), "")
	assertConfigurationError(t, err)
}

func TestV1SimulationsCreateBodiesHaveNoStoreCnpj(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusCreated, simulationOptionsJSON)
	})

	options, err := client.V1.Simulations.CreateFinancing(context.Background(), credicarro.V1CreateFinancingSimulationRequest{
		Cpf: "12345678909", LicensePlate: "ABC1D23", VehicleAmount: 45000, DownPayment: 5000, State: "SP", IsZeroKm: false,
	})
	if err != nil || len(options) != 2 {
		t.Fatalf("esperava 2 opções, veio %d (%v)", len(options), err)
	}
	if _, err = client.V1.Simulations.CreateRefinancing(context.Background(), credicarro.V1CreateRefinancingSimulationRequest{
		Cpf: "12345678909", LicensePlate: "ABC1D23", RequestedAmount: 30000,
	}); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	reqs := api.recorded()
	financingBody := map[string]interface{}{
		"cpf": "12345678909", "placa": "ABC1D23", "valorVeiculo": float64(45000), "valorEntrada": float64(5000), "uf": "SP", "isVeiculoZeroKm": false,
	}
	refinancingBody := map[string]interface{}{"cpf": "12345678909", "placa": "ABC1D23", "valorSolicitado": float64(30000)}
	if reqs[0].Path != "v1/proposal/simulation/financing" || !reflect.DeepEqual(reqs[0].JSON(t), financingBody) {
		t.Fatalf("financiamento v1 incorreto: %s %v", reqs[0].Path, reqs[0].JSON(t))
	}
	if reqs[1].Path != "v1/proposal/simulation/refinancing" || !reflect.DeepEqual(reqs[1].JSON(t), refinancingBody) {
		t.Fatalf("refinanciamento v1 incorreto: %s %v", reqs[1].Path, reqs[1].JSON(t))
	}
}

const financingWithoutCpfJSON = `{"cotacoes":[
  {"valor_solicitado":40000,"prazo":24,"valor_parcela":2100.5,"valor_tac":300,"valor_boleto":3.5,"valor_seguro":120,"valor_iof":900.1,"percentual_iof":2.3,"valor_credito_total":40000,"valor_divida_total":50412,"percentual_cet_mensal":2.1,"percentual_cet_anual":28.4,"percentual_juros_mensal":1.9,"percentual_juros_anual":25.2,"valor_retencao_diaria_aproximado":12.75,"valor_entrada_minima":4500.5},
  {"valor_solicitado":40000,"prazo":36,"valor_parcela":1600,"valor_tac":300,"valor_boleto":3.5,"valor_seguro":120,"valor_iof":1000,"percentual_iof":2.4,"valor_credito_total":40000,"valor_divida_total":57600,"percentual_cet_mensal":2.0,"percentual_cet_anual":27,"percentual_juros_mensal":1.8,"percentual_juros_anual":24,"valor_entrada_minima":4500.5}
]}`

func TestV1CreateFinancingWithoutCpfWithDownPayment(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusOK, financingWithoutCpfJSON)
	})

	downPayment := 5000.0
	options, err := client.V1.Simulations.CreateFinancingWithoutCpf(context.Background(), credicarro.CreateFinancingWithoutCpfRequest{
		LicensePlate: "ABC1D23", VehicleAmount: 45000, DownPayment: &downPayment, State: "SP",
	})
	if err != nil || len(options) != 2 {
		t.Fatalf("esperava 2 opções, veio %d (%v)", len(options), err)
	}

	retention := 12.75
	first := credicarro.FinancingWithoutCpfOption{
		RequestedAmount: 40000, Term: 24, InstallmentAmount: 2100.5, TacAmount: 300, BoletoAmount: 3.5, InsuranceAmount: 120,
		IofAmount: 900.1, IofRate: 2.3, TotalCreditAmount: 40000, TotalDebtAmount: 50412, MonthlyCet: 2.1, AnnualCet: 28.4,
		MonthlyInterestRate: 1.9, AnnualInterestRate: 25.2, ApproximateDailyRetention: &retention, MinimumDownPayment: 4500.5,
	}
	if !reflect.DeepEqual(options[0], first) {
		t.Fatalf("primeira opção incorreta: %+v", options[0])
	}
	if options[1].Term != 36 || options[1].InstallmentAmount != 1600 || options[1].MinimumDownPayment != 4500.5 || options[1].ApproximateDailyRetention != nil {
		t.Fatalf("segunda opção incorreta: %+v", options[1])
	}

	reqs := api.recorded()
	body := map[string]interface{}{"placa": "ABC1D23", "valorVeiculo": float64(45000), "valorEntrada": float64(5000), "uf": "SP"}
	if len(reqs) != 1 || reqs[0].Method != http.MethodPost || reqs[0].Path != "v1/proposal/simulation/financing/without-cpf" || !reflect.DeepEqual(reqs[0].JSON(t), body) {
		t.Fatalf("requisição incorreta: %+v", reqs)
	}
}

func TestV1CreateFinancingWithoutCpfOmitsDownPaymentAndSendsNoIdempotencyKey(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusOK, financingWithoutCpfJSON)
	})

	options, err := client.V1.Simulations.CreateFinancingWithoutCpf(context.Background(), credicarro.CreateFinancingWithoutCpfRequest{
		LicensePlate: "ABC1D23", VehicleAmount: 45000, State: "SP",
	})
	if err != nil || len(options) != 2 {
		t.Fatalf("esperava 2 opções, veio %d (%v)", len(options), err)
	}

	reqs := api.recorded()
	body := reqs[0].JSON(t)
	if _, present := body["valorEntrada"]; present {
		t.Fatalf("valorEntrada não deveria ir no JSON quando omitido: %v", body)
	}
	if len(body) != 3 {
		t.Fatalf("corpo incorreto: %v", body)
	}
	if got := reqs[0].Header.Get("Idempotency-Key"); got != "" {
		t.Fatalf("não deveria enviar Idempotency-Key, veio %q", got)
	}
}

func TestV1CreateFinancingWithoutCpfSendsZeroDownPayment(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusOK, `{"cotacoes":[]}`)
	})

	zero := 0.0
	options, err := client.V1.Simulations.CreateFinancingWithoutCpf(context.Background(), credicarro.CreateFinancingWithoutCpfRequest{
		LicensePlate: "ABC1D23", VehicleAmount: 45000, DownPayment: &zero, State: "SP",
	})
	if err != nil || options == nil || len(options) != 0 {
		t.Fatalf("esperava lista vazia, veio %+v (%v)", options, err)
	}
	if v, present := api.recorded()[0].JSON(t)["valorEntrada"]; !present || v != float64(0) {
		t.Fatalf("entrada zero deveria ir no JSON: %v", v)
	}
}

func TestV1CreateFinancingWithoutCpfMapsErrors(t *testing.T) {
	cases := []struct {
		status int
		code   string
	}{
		{http.StatusNotFound, "Fipe.VehiclePlateNotFound"},
		{http.StatusUnprocessableEntity, "Proposal.DownPaymentInvalid"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.code, func(t *testing.T) {
			client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				writeRaw(w, c.status, `{"tipo":"`+c.code+`","titulo":"Erro","status":`+strconv.Itoa(c.status)+`}`)
			})
			_, err := client.V1.Simulations.CreateFinancingWithoutCpf(context.Background(), credicarro.CreateFinancingWithoutCpfRequest{
				LicensePlate: "ABC1D23", VehicleAmount: 45000, State: "SP",
			})
			if c.status == http.StatusUnprocessableEntity {
				var rule *credicarro.BusinessRuleError
				if !asBusinessRule(err, &rule) || rule.ErrorCode != c.code {
					t.Fatalf("esperava BusinessRuleError, veio %T: %v", err, err)
				}
				return
			}
			var apiErr *credicarro.APIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != c.status || apiErr.ErrorCode != c.code {
				t.Fatalf("esperava APIError %d %s, veio %T: %v", c.status, c.code, err, err)
			}
		})
	}
}

func TestV1CreateFinancingWithoutCpfIsRetriedLikeARead(t *testing.T) {
	client, api := newTestClient(t, failThenSucceed(2, http.StatusBadGateway, http.StatusOK, financingWithoutCpfJSON))

	options, err := client.V1.Simulations.CreateFinancingWithoutCpf(context.Background(), credicarro.CreateFinancingWithoutCpfRequest{
		LicensePlate: "ABC1D23", VehicleAmount: 45000, State: "SP",
	})
	if err != nil || len(options) != 2 {
		t.Fatalf("esperava sucesso após repetir, veio %d (%v)", len(options), err)
	}
	reqs := api.recorded()
	if len(reqs) != 3 {
		t.Fatalf("esperava 3 tentativas, veio %d", len(reqs))
	}
	for i, req := range reqs {
		if req.Header.Get("Idempotency-Key") != "" {
			t.Fatalf("tentativa %d: não deveria enviar Idempotency-Key", i)
		}
	}
}

func TestV1ProposalsCreateAndGet(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			writeRaw(w, http.StatusCreated, `{"id_proposta":"`+testUUID+`"}`)
			return
		}
		writeRaw(w, http.StatusOK, `{"id_proposta":"`+testUUID+`","numero":9,"id_simulacao":"`+testUUID2+`","tipo":"Financing","status":"Completed","comprador":{"cpf":"12345678909"},"data_criacao":"2026-10-12T14:05:08-03:00"}`)
	})

	id, err := client.V1.Proposals.Create(context.Background(), credicarro.CreateProposalRequest{Cpf: "12345678909", SimulationID: testUUID2})
	if err != nil || id != testUUID {
		t.Fatalf("esperava %s, veio %q (%v)", testUUID, id, err)
	}
	summary, err := client.V1.Proposals.Get(context.Background(), testUUID)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if summary.Status != credicarro.ProposalStatusCompleted || summary.StatusName != "Completed" || summary.Type != credicarro.ProposalTypeFinancing ||
		summary.BuyerCpf != "12345678909" || summary.SellerCpf != "" || summary.Number != 9 {
		t.Fatalf("resumo incorreto: %+v", summary)
	}

	reqs := api.recorded()
	if reqs[0].Method != http.MethodPost || reqs[0].Path != "v1/proposal" || reqs[1].Method != http.MethodGet || reqs[1].Path != "v1/proposal/"+testUUID {
		t.Fatalf("rotas incorretas: %+v", reqs)
	}

	_, err = client.V1.Proposals.Get(context.Background(), "x")
	assertConfigurationError(t, err)
}

const refinancingWithoutCpfJSON = `{"cotacoes":[
  {"valor_solicitado":20000,"prazo":24,"valor_parcela":1100.5,"valor_tac":300,"valor_boleto":3.5,"valor_seguro":120,"valor_iof":500.1,"percentual_iof":2.3,"valor_credito_total":20000,"valor_divida_total":26412,"percentual_cet_mensal":2.1,"percentual_cet_anual":28.4,"percentual_juros_mensal":1.9,"percentual_juros_anual":25.2,"valor_retencao_diaria_aproximado":12.75,"valor_minimo_refinanciavel":5000,"valor_maximo_refinanciavel":30000},
  {"valor_solicitado":20000,"prazo":36,"valor_parcela":800,"valor_tac":300,"valor_boleto":3.5,"valor_seguro":120,"valor_iof":600,"percentual_iof":2.4,"valor_credito_total":20000,"valor_divida_total":28800,"percentual_cet_mensal":2.0,"percentual_cet_anual":27,"percentual_juros_mensal":1.8,"percentual_juros_anual":24,"valor_minimo_refinanciavel":6000,"valor_maximo_refinanciavel":25000}
]}`

func TestV1CreateRefinancingWithoutCpfMapsOptionsAndBody(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusOK, refinancingWithoutCpfJSON)
	})

	options, err := client.V1.Simulations.CreateRefinancingWithoutCpf(context.Background(), credicarro.CreateRefinancingWithoutCpfRequest{
		LicensePlate: "ABC1D23", RequestedAmount: 20000, State: "SP",
	})
	if err != nil || len(options) != 2 {
		t.Fatalf("esperava 2 opções, veio %d (%v)", len(options), err)
	}

	retention := 12.75
	first := credicarro.RefinancingWithoutCpfOption{
		RequestedAmount: 20000, Term: 24, InstallmentAmount: 1100.5, TacAmount: 300, BoletoAmount: 3.5, InsuranceAmount: 120,
		IofAmount: 500.1, IofRate: 2.3, TotalCreditAmount: 20000, TotalDebtAmount: 26412, MonthlyCet: 2.1, AnnualCet: 28.4,
		MonthlyInterestRate: 1.9, AnnualInterestRate: 25.2, ApproximateDailyRetention: &retention, MinimumAmount: 5000, MaximumAmount: 30000,
	}
	if !reflect.DeepEqual(options[0], first) {
		t.Fatalf("primeira opção incorreta: %+v", options[0])
	}
	if options[1].Term != 36 || options[1].MinimumAmount != 6000 || options[1].MaximumAmount != 25000 || options[1].ApproximateDailyRetention != nil {
		t.Fatalf("segunda opção incorreta: %+v", options[1])
	}

	reqs := api.recorded()
	body := map[string]interface{}{"placa": "ABC1D23", "valorSolicitado": float64(20000), "uf": "SP"}
	if len(reqs) != 1 || reqs[0].Method != http.MethodPost || reqs[0].Path != "v1/proposal/simulation/refinancing/without-cpf" || !reflect.DeepEqual(reqs[0].JSON(t), body) {
		t.Fatalf("requisição incorreta: %+v", reqs)
	}
	if got := reqs[0].Header.Get("Idempotency-Key"); got != "" {
		t.Fatalf("não deveria enviar Idempotency-Key, veio %q", got)
	}
}

func TestV1CreateRefinancingWithoutCpfMapsErrors(t *testing.T) {
	cases := []struct {
		status int
		code   string
	}{
		{http.StatusNotFound, "Fipe.VehiclePlateNotFound"},
		{http.StatusUnprocessableEntity, "Proposal.RequestAmountInvalid"},
	}
	for _, c := range cases {
		c := c
		t.Run(c.code, func(t *testing.T) {
			client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				writeRaw(w, c.status, `{"tipo":"`+c.code+`","titulo":"Erro","status":`+strconv.Itoa(c.status)+`}`)
			})
			_, err := client.V1.Simulations.CreateRefinancingWithoutCpf(context.Background(), credicarro.CreateRefinancingWithoutCpfRequest{
				LicensePlate: "ABC1D23", RequestedAmount: 20000, State: "SP",
			})
			if c.status == http.StatusUnprocessableEntity {
				var rule *credicarro.BusinessRuleError
				if !asBusinessRule(err, &rule) || rule.ErrorCode != c.code {
					t.Fatalf("esperava BusinessRuleError, veio %T: %v", err, err)
				}
				return
			}
			var apiErr *credicarro.APIError
			if !errors.As(err, &apiErr) || apiErr.StatusCode != c.status || apiErr.ErrorCode != c.code {
				t.Fatalf("esperava APIError %d %s, veio %T: %v", c.status, c.code, err, err)
			}
		})
	}
}

func TestV1CreateRefinancingWithoutCpfIsRetriedLikeARead(t *testing.T) {
	client, api := newTestClient(t, failThenSucceed(2, http.StatusBadGateway, http.StatusOK, refinancingWithoutCpfJSON))

	options, err := client.V1.Simulations.CreateRefinancingWithoutCpf(context.Background(), credicarro.CreateRefinancingWithoutCpfRequest{
		LicensePlate: "ABC1D23", RequestedAmount: 20000, State: "SP",
	})
	if err != nil || len(options) != 2 {
		t.Fatalf("esperava sucesso após repetir, veio %d (%v)", len(options), err)
	}
	reqs := api.recorded()
	if len(reqs) != 3 {
		t.Fatalf("esperava 3 tentativas, veio %d", len(reqs))
	}
	for i, req := range reqs {
		if req.Header.Get("Idempotency-Key") != "" {
			t.Fatalf("tentativa %d: não deveria enviar Idempotency-Key", i)
		}
	}
}
