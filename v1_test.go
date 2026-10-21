package credicarro_test

import (
	"context"
	"net/http"
	"reflect"
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
