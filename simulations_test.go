package credicarro_test

import (
	"context"
	"net/http"
	"reflect"
	"testing"

	credicarro "github.com/Twila-Digital/twila-credicarro-go-sdk"
)

const simulationOptionsJSON = `{"simulacoes":[
  {"id_simulacao":"bea8d060-f93d-48eb-b6c4-45f64483cdd9","valor_solicitado":40000,"prazo":24,"valor_parcela":2100.5,"valor_tac":300,"valor_boleto":3.5,"valor_seguro":120,"valor_iof":900.1,"percentual_iof":2.3,"valor_credito_total":40000,"valor_divida_total":50412,"percentual_cet_mensal":2.1,"percentual_cet_anual":28.4,"percentual_juros_mensal":1.9,"percentual_juros_anual":25.2,"valor_retencao_diaria_aproximado":12.75},
  {"id_simulacao":"123e4567-e89b-12d3-a456-426614174000","valor_solicitado":40000,"prazo":36,"valor_parcela":1600,"valor_tac":300,"valor_boleto":3.5,"valor_seguro":120,"valor_iof":1000,"percentual_iof":2.4,"valor_credito_total":40000,"valor_divida_total":57600,"percentual_cet_mensal":2.0,"percentual_cet_anual":27,"percentual_juros_mensal":1.8,"percentual_juros_anual":24,"valor_retencao_diaria_aproximado":null}
]}`

func TestSimulationsGetFinancingRangeV2(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusOK, `{"opcoes_por_prazo":[{"prazo":12,"valor_minimo_entrada":5000},{"prazo":24,"valor_minimo_entrada":4500.5}]}`)
	})

	options, err := client.Simulations.GetFinancingRange(context.Background(), "ABC-1234", "11.222.333/0001-81")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	expected := []credicarro.FinancingRangeOption{{Term: 12, MinimumDownPayment: 5000}, {Term: 24, MinimumDownPayment: 4500.5}}
	if !reflect.DeepEqual(options, expected) {
		t.Fatalf("opções incorretas: %+v", options)
	}

	req := api.recorded()[0]
	if req.Method != http.MethodGet || req.Path != "v2/proposal/simulation/range/financing" {
		t.Fatalf("rota incorreta: %s %s", req.Method, req.Path)
	}
	if req.RawQuery != "cnpjLoja=11.222.333%2F0001-81&placa=ABC-1234" {
		t.Fatalf("query incorreta: %q", req.RawQuery)
	}
}

func TestSimulationsGetRefinancingRangeV2(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusOK, `{"opcoes_por_prazo":[{"prazo":6,"valor_minimo_refinanciavel":12480,"valor_maximo_refinanciavel":35720}]}`)
	})

	options, err := client.Simulations.GetRefinancingRange(context.Background(), "ABC1D23", "11222333000181")
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	expected := []credicarro.RefinancingRangeOption{{Term: 6, MinimumAmount: 12480, MaximumAmount: 35720}}
	if !reflect.DeepEqual(options, expected) {
		t.Fatalf("opções incorretas: %+v", options)
	}
	if req := api.recorded()[0]; req.Path != "v2/proposal/simulation/range/refinancing" || req.RawQuery != "cnpjLoja=11222333000181&placa=ABC1D23" {
		t.Fatalf("requisição incorreta: %s ?%s", req.Path, req.RawQuery)
	}
}

func TestSimulationsRangeRequiresPlateAndStore(t *testing.T) {
	client, api := newTestClient(t, nil)
	_, err := client.Simulations.GetFinancingRange(context.Background(), "", "11222333000181")
	assertConfigurationError(t, err)
	_, err = client.Simulations.GetRefinancingRange(context.Background(), "ABC1D23", "")
	assertConfigurationError(t, err)
	if len(api.recorded()) != 0 {
		t.Fatal("nenhuma requisição deveria ser enviada")
	}
}

func TestSimulationsRangeUnavailableTermsIsBusinessRule(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusUnprocessableEntity, `{"tipo":"Simulation.Terms.Unavailable","titulo":"Regra de negócio não atendida","status":422}`)
	})
	_, err := client.Simulations.GetFinancingRange(context.Background(), "ABC1D23", "11222333000181")
	var rule *credicarro.BusinessRuleError
	if !asBusinessRule(err, &rule) || rule.ErrorCode != "Simulation.Terms.Unavailable" {
		t.Fatalf("esperava BusinessRuleError, veio %T: %v", err, err)
	}
}

func TestSimulationsCreateFinancingV2(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusCreated, simulationOptionsJSON)
	})

	options, err := client.Simulations.CreateFinancing(context.Background(), credicarro.CreateFinancingSimulationRequest{
		Cpf: "12345678909", LicensePlate: "ABC1D23", VehicleAmount: 45000, DownPayment: 5000, State: "SP", IsZeroKm: true, StoreCnpj: "11222333000181",
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}

	req := api.recorded()[0]
	if req.Method != http.MethodPost || req.Path != "v2/proposal/simulation/financing" {
		t.Fatalf("rota incorreta: %s %s", req.Method, req.Path)
	}
	expectedBody := map[string]interface{}{
		"cpf": "12345678909", "placa": "ABC1D23", "valorVeiculo": float64(45000), "valorEntrada": float64(5000),
		"uf": "SP", "isVeiculoZeroKm": true, "cnpjLoja": "11222333000181",
	}
	if got := req.JSON(t); !reflect.DeepEqual(got, expectedBody) {
		t.Fatalf("corpo incorreto:\n got: %v\nwant: %v", got, expectedBody)
	}

	if len(options) != 2 {
		t.Fatalf("esperava 2 opções, veio %d", len(options))
	}
	first := options[0]
	if first.SimulationID != testUUID || first.Term != 24 || first.RequestedAmount != 40000 || first.InstallmentAmount != 2100.5 ||
		first.TacAmount != 300 || first.BoletoAmount != 3.5 || first.InsuranceAmount != 120 || first.IofAmount != 900.1 || first.IofRate != 2.3 ||
		first.TotalCreditAmount != 40000 || first.TotalDebtAmount != 50412 || first.MonthlyCet != 2.1 || first.AnnualCet != 28.4 ||
		first.MonthlyInterestRate != 1.9 || first.AnnualInterestRate != 25.2 {
		t.Fatalf("opção incorreta: %+v", first)
	}
	if first.ApproximateDailyRetention == nil || *first.ApproximateDailyRetention != 12.75 {
		t.Fatalf("ApproximateDailyRetention incorreto: %v", first.ApproximateDailyRetention)
	}
	if options[1].ApproximateDailyRetention != nil {
		t.Fatal("ApproximateDailyRetention deveria ser nil quando ausente/nulo")
	}
}

func TestSimulationsCreateRefinancingV2(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusCreated, simulationOptionsJSON)
	})

	options, err := client.Simulations.CreateRefinancing(context.Background(), credicarro.CreateRefinancingSimulationRequest{
		Cpf: "12345678909", LicensePlate: "ABC1D23", RequestedAmount: 30000, StoreCnpj: "11222333000181",
	})
	if err != nil || len(options) != 2 {
		t.Fatalf("esperava 2 opções, veio %d (%v)", len(options), err)
	}

	req := api.recorded()[0]
	expectedBody := map[string]interface{}{"cpf": "12345678909", "placa": "ABC1D23", "valorSolicitado": float64(30000), "cnpjLoja": "11222333000181"}
	if req.Path != "v2/proposal/simulation/refinancing" || !reflect.DeepEqual(req.JSON(t), expectedBody) {
		t.Fatalf("requisição incorreta: %s %v", req.Path, req.JSON(t))
	}
}

func TestSimulationsCreateMapsRuleErrors(t *testing.T) {
	client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusUnprocessableEntity, `{"tipo":"Simulation.Refinancing.AmountOutOfRange","titulo":"Regra de negócio não atendida","status":422}`)
	})
	_, err := client.Simulations.CreateRefinancing(context.Background(), credicarro.CreateRefinancingSimulationRequest{Cpf: "12345678909"})
	var rule *credicarro.BusinessRuleError
	if !asBusinessRule(err, &rule) || rule.ErrorCode != "Simulation.Refinancing.AmountOutOfRange" {
		t.Fatalf("esperava BusinessRuleError, veio %T: %v", err, err)
	}
}

const simulationJSON = `{
 "id_simulacao":"bea8d060-f93d-48eb-b6c4-45f64483cdd9","prazo":36,"valor_parcela":1600.25,"valor_solicitado":40000,"valor_entrada":5000,
 "valor_bem":45000,"valor_tac":300,"taxa_juros_negociada":1.79,"valor_registro":250,"uf":"SP","tipo":"Financing",
 "transacao":{"valor_total_credito":40000,"valor_total_debito":57609},
 "juros":{"percentual_juros_mensal":1.8,"percentual_juros_anual":24},
 "cet":{"percentual_mensal":2.0,"percentual_anual":27},
 "iof":{"valor":1000,"percentual":2.4},
 "veiculo":{"marca":"Fiat","modelo":"Argo","versao":"1.0","ano":2022,"valor":45000,"placa":"ABC1D23","veiculo_zero_km":false,"chassi":"9BD11111111111111","renavam":"12345678901","cor":"Branco"},
 "cliente":{"cpf":"12345678909"},
 "data_criacao":"2026-10-12T14:05:08-03:00"}`

func TestSimulationsGetMapsEverything(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusOK, simulationJSON)
	})

	sim, err := client.Simulations.Get(context.Background(), testUUID)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if req := api.recorded()[0]; req.Method != http.MethodGet || req.Path != "v1/proposal/simulation/"+testUUID {
		t.Fatalf("rota incorreta: %s %s", req.Method, req.Path)
	}

	year, value, zeroKm := 2022, 45000.0, false
	expected := &credicarro.Simulation{
		SimulationID: testUUID, Term: 36, InstallmentAmount: 1600.25, RequestedAmount: 40000, DownPayment: 5000, AssetAmount: 45000,
		TacAmount: 300, NegotiatedInterestRate: 1.79, RegistrationAmount: 250, State: "SP",
		Type: credicarro.SimulationTypeFinancing, TypeRaw: "Financing",
		Transaction: credicarro.SimulationTransaction{TotalCredit: 40000, TotalDebit: 57609},
		Interest:    credicarro.SimulationRates{MonthlyRate: 1.8, AnnualRate: 24},
		Cet:         credicarro.SimulationRates{MonthlyRate: 2.0, AnnualRate: 27},
		Iof:         credicarro.SimulationIof{Amount: 1000, Rate: 2.4},
		Vehicle: credicarro.SimulationVehicle{
			Brand: "Fiat", Model: "Argo", Version: "1.0", Year: &year, Value: &value, LicensePlate: "ABC1D23", IsZeroKm: &zeroKm,
			Chassis: "9BD11111111111111", Renavam: "12345678901", Color: "Branco",
		},
		CustomerCpf: "12345678909", CreatedAt: "2026-10-12T14:05:08-03:00",
	}
	if !reflect.DeepEqual(sim, expected) {
		t.Fatalf("simulação incorreta:\n got: %+v\nwant: %+v", sim, expected)
	}
}

func TestSimulationTypeMapping(t *testing.T) {
	cases := map[string]credicarro.SimulationType{
		"Financing": credicarro.SimulationTypeFinancing, "Refinancing": credicarro.SimulationTypeRefinancing,
		"AmountSimulation": credicarro.SimulationTypeAmountSimulation, "OutroTipo": credicarro.SimulationTypeUnknown, "": credicarro.SimulationTypeUnknown,
	}
	for raw, expected := range cases {
		raw := raw
		client, _ := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
			writeRaw(w, http.StatusOK, `{"id_simulacao":"`+testUUID+`","tipo":"`+raw+`"}`)
		})
		sim, err := client.Simulations.Get(context.Background(), testUUID)
		if err != nil || sim.Type != expected || sim.TypeRaw != raw {
			t.Errorf("tipo %q: esperava %d, veio %+v (%v)", raw, expected, sim, err)
		}
	}
}

func TestSimulationsGetRejectsInvalidID(t *testing.T) {
	client, api := newTestClient(t, nil)
	for _, id := range []string{"", "abc", "../v1/customer", testUUID + "/x"} {
		if _, err := client.Simulations.Get(context.Background(), id); err == nil {
			t.Errorf("id %q deveria ser rejeitado", id)
		}
	}
	if len(api.recorded()) != 0 {
		t.Fatal("nenhuma requisição deveria ser enviada")
	}
}

func TestSimulationsListUsesPagingAndMapsPage(t *testing.T) {
	client, api := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		writeRaw(w, http.StatusOK, `{"itens":[`+simulationJSON+`],"pagina":{"tem_proximo":true,"tem_anterior":false,"numero":1,"tamanho":10,"total":42}}`)
	})

	page, err := client.Simulations.List(context.Background(), nil)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if req := api.recorded()[0]; req.Path != "v1/proposal/simulation" || req.RawQuery != "pagina_inicial=1&limite=10" {
		t.Fatalf("requisição incorreta: %s ?%s", req.Path, req.RawQuery)
	}
	if len(page.Items) != 1 || page.Items[0].SimulationID != testUUID || page.Page != 1 || page.PageSize != 10 || page.TotalCount != 42 || !page.HasNext || page.HasPrevious {
		t.Fatalf("página incorreta: %+v", page)
	}

	if _, err := client.Simulations.List(context.Background(), &credicarro.ListRequest{Page: 3, PageSize: 100}); err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got := api.recorded()[1].RawQuery; got != "pagina_inicial=3&limite=100" {
		t.Fatalf("query incorreta: %q", got)
	}
}
