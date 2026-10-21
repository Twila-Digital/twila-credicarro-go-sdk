package credicarro

import (
	"context"
	"net/url"

	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/transport"
	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/wire"
)

// FinancingRangeOption é o valor mínimo de entrada de um prazo de financiamento.
type FinancingRangeOption struct {
	Term               int
	MinimumDownPayment float64
}

// RefinancingRangeOption é a faixa refinanciável de um prazo.
type RefinancingRangeOption struct {
	Term          int
	MinimumAmount float64
	MaximumAmount float64
}

// CreateFinancingSimulationRequest é a requisição de simulação de financiamento (v2, com a loja).
type CreateFinancingSimulationRequest struct {
	Cpf           string
	LicensePlate  string // placa (ABC-1234 ou ABC1D23)
	VehicleAmount float64
	DownPayment   float64 // zero quando não há entrada; menor que VehicleAmount
	State         string  // uf do registro do veículo
	IsZeroKm      bool
	StoreCnpj     string // cnpjLoja, formatado ou só números
}

// CreateRefinancingSimulationRequest é a requisição de simulação de refinanciamento (v2, com a loja).
type CreateRefinancingSimulationRequest struct {
	Cpf             string
	LicensePlate    string
	RequestedAmount float64 // precisa estar na faixa refinanciável da placa
	StoreCnpj       string
}

// SimulationOption é uma opção de simulação (uma por prazo). Use SimulationID em
// CreateProposalRequest.
type SimulationOption struct {
	SimulationID        string
	RequestedAmount     float64
	Term                int
	InstallmentAmount   float64
	TacAmount           float64
	BoletoAmount        float64
	InsuranceAmount     float64
	IofAmount           float64
	IofRate             float64
	TotalCreditAmount   float64
	TotalDebtAmount     float64
	MonthlyCet          float64
	AnnualCet           float64
	MonthlyInterestRate float64
	AnnualInterestRate  float64
	// ApproximateDailyRetention é nil quando não se aplica.
	ApproximateDailyRetention *float64
}

// SimulationTransaction são os totais de crédito e débito da simulação.
type SimulationTransaction struct {
	TotalCredit float64
	TotalDebit  float64
}

// SimulationRates são as taxas mensal e anual (juros e CET).
type SimulationRates struct {
	MonthlyRate float64
	AnnualRate  float64
}

// SimulationIof é o IOF da simulação.
type SimulationIof struct {
	Amount float64
	Rate   float64
}

// SimulationVehicle é o veículo da simulação.
type SimulationVehicle struct {
	Brand        string
	Model        string
	Version      string
	Year         *int
	Value        *float64
	LicensePlate string
	IsZeroKm     *bool
	Chassis      string
	Renavam      string
	Color        string
}

// Simulation é a simulação consultada (schema SimulationResponse).
type Simulation struct {
	SimulationID           string
	Term                   int
	InstallmentAmount      float64
	RequestedAmount        float64
	DownPayment            float64
	AssetAmount            float64
	TacAmount              float64
	NegotiatedInterestRate float64
	RegistrationAmount     float64
	State                  string
	Type                   SimulationType
	TypeRaw                string
	Transaction            SimulationTransaction
	Interest               SimulationRates
	Cet                    SimulationRates
	Iof                    SimulationIof
	Vehicle                SimulationVehicle
	CustomerCpf            string
	CreatedAt              string
}

// SimulationsClient expõe as simulações v2 (com cnpjLoja) e as consultas v1.
type SimulationsClient struct {
	api *apiCaller
}

// GetFinancingRange consulta o valor mínimo de entrada por prazo (GET v2/proposal/simulation/range/financing).
func (c *SimulationsClient) GetFinancingRange(ctx context.Context, licensePlate, storeCnpj string) ([]FinancingRangeOption, error) {
	if err := requireNonEmpty("licensePlate", licensePlate); err != nil {
		return nil, err
	}
	if err := requireNonEmpty("storeCnpj", storeCnpj); err != nil {
		return nil, err
	}
	return getFinancingRange(ctx, c.api, "v2/proposal/simulation/range/financing", rangeQuery(licensePlate, storeCnpj, true))
}

// GetRefinancingRange consulta a faixa refinanciável por prazo (GET v2/proposal/simulation/range/refinancing).
func (c *SimulationsClient) GetRefinancingRange(ctx context.Context, licensePlate, storeCnpj string) ([]RefinancingRangeOption, error) {
	if err := requireNonEmpty("licensePlate", licensePlate); err != nil {
		return nil, err
	}
	if err := requireNonEmpty("storeCnpj", storeCnpj); err != nil {
		return nil, err
	}
	return getRefinancingRange(ctx, c.api, "v2/proposal/simulation/range/refinancing", rangeQuery(licensePlate, storeCnpj, true))
}

// CreateFinancing cria a simulação de financiamento (POST v2/proposal/simulation/financing)
// e devolve uma opção por prazo. Recebe Idempotency-Key automática.
func (c *SimulationsClient) CreateFinancing(ctx context.Context, req CreateFinancingSimulationRequest, opts ...RequestOption) ([]SimulationOption, error) {
	ropts, err := applyRequestOptions(opts)
	if err != nil {
		return nil, err
	}
	wireReq := wire.FinancingSimulationRequest{
		Cpf:           req.Cpf,
		LicensePlate:  req.LicensePlate,
		VehicleAmount: req.VehicleAmount,
		DownPayment:   req.DownPayment,
		State:         req.State,
		IsZeroKm:      req.IsZeroKm,
		StoreCnpj:     req.StoreCnpj,
	}
	return postSimulation(ctx, c.api, "v2/proposal/simulation/financing", wireReq, ropts)
}

// CreateRefinancing cria a simulação de refinanciamento (POST v2/proposal/simulation/refinancing).
// Recebe Idempotency-Key automática.
func (c *SimulationsClient) CreateRefinancing(ctx context.Context, req CreateRefinancingSimulationRequest, opts ...RequestOption) ([]SimulationOption, error) {
	ropts, err := applyRequestOptions(opts)
	if err != nil {
		return nil, err
	}
	wireReq := wire.RefinancingSimulationRequest{
		Cpf:             req.Cpf,
		LicensePlate:    req.LicensePlate,
		RequestedAmount: req.RequestedAmount,
		StoreCnpj:       req.StoreCnpj,
	}
	return postSimulation(ctx, c.api, "v2/proposal/simulation/refinancing", wireReq, ropts)
}

// Get consulta uma simulação pelo id (GET v1/proposal/simulation/{id}).
func (c *SimulationsClient) Get(ctx context.Context, simulationID string) (*Simulation, error) {
	if err := requireUUID("simulationID", simulationID); err != nil {
		return nil, err
	}
	var result wire.Simulation
	if err := c.api.get(ctx, "v1/proposal/simulation/"+url.PathEscape(simulationID), &result); err != nil {
		return nil, err
	}
	simulation := simulationFromWire(result)
	return &simulation, nil
}

// List lista as simulações, paginadas (GET v1/proposal/simulation). req nil usa a página 1 com 10 itens.
func (c *SimulationsClient) List(ctx context.Context, req *ListRequest) (*PagedResult[Simulation], error) {
	query, err := req.query()
	if err != nil {
		return nil, err
	}
	var result wire.PagedSimulations
	if err := c.api.get(ctx, "v1/proposal/simulation?"+query, &result); err != nil {
		return nil, err
	}

	items := make([]Simulation, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, simulationFromWire(item))
	}
	return &PagedResult[Simulation]{
		Items:       items,
		Page:        result.Page.Number,
		PageSize:    result.Page.Size,
		TotalCount:  result.Page.Total,
		HasNext:     result.Page.HasNext,
		HasPrevious: result.Page.HasPrevious,
	}, nil
}

func getFinancingRange(ctx context.Context, api *apiCaller, path string, q url.Values) ([]FinancingRangeOption, error) {
	var result wire.FinancingRangeResponse
	if err := api.get(ctx, path+"?"+q.Encode(), &result); err != nil {
		return nil, err
	}
	options := make([]FinancingRangeOption, 0, len(result.Options))
	for _, o := range result.Options {
		options = append(options, FinancingRangeOption{Term: o.Term, MinimumDownPayment: o.MinimumDownPayment})
	}
	return options, nil
}

func getRefinancingRange(ctx context.Context, api *apiCaller, path string, q url.Values) ([]RefinancingRangeOption, error) {
	var result wire.RefinancingRangeResponse
	if err := api.get(ctx, path+"?"+q.Encode(), &result); err != nil {
		return nil, err
	}
	options := make([]RefinancingRangeOption, 0, len(result.Options))
	for _, o := range result.Options {
		options = append(options, RefinancingRangeOption{Term: o.Term, MinimumAmount: o.MinimumAmount, MaximumAmount: o.MaximumAmount})
	}
	return options, nil
}

func postSimulation(ctx context.Context, api *apiCaller, path string, body interface{}, ropts transport.RequestOptions) ([]SimulationOption, error) {
	var result wire.SimulationListResponse
	if err := api.post(ctx, path, body, &result, ropts); err != nil {
		return nil, err
	}
	options := make([]SimulationOption, 0, len(result.Simulations))
	for _, o := range result.Simulations {
		options = append(options, SimulationOption{
			SimulationID:              o.SimulationID,
			RequestedAmount:           o.RequestedAmount,
			Term:                      o.Term,
			InstallmentAmount:         o.InstallmentAmount,
			TacAmount:                 o.TacAmount,
			BoletoAmount:              o.BoletoAmount,
			InsuranceAmount:           o.InsuranceAmount,
			IofAmount:                 o.IofAmount,
			IofRate:                   o.IofRate,
			TotalCreditAmount:         o.TotalCreditAmount,
			TotalDebtAmount:           o.TotalDebtAmount,
			MonthlyCet:                o.MonthlyCet,
			AnnualCet:                 o.AnnualCet,
			MonthlyInterestRate:       o.MonthlyInterestRate,
			AnnualInterestRate:        o.AnnualInterestRate,
			ApproximateDailyRetention: o.ApproximateDailyRetention,
		})
	}
	return options, nil
}

func simulationFromWire(w wire.Simulation) Simulation {
	return Simulation{
		SimulationID:           w.SimulationID,
		Term:                   w.Term,
		InstallmentAmount:      w.InstallmentAmount,
		RequestedAmount:        w.RequestedAmount,
		DownPayment:            w.DownPayment,
		AssetAmount:            w.AssetAmount,
		TacAmount:              w.TacAmount,
		NegotiatedInterestRate: w.NegotiatedInterestRate,
		RegistrationAmount:     w.RegistrationAmount,
		State:                  w.State,
		Type:                   simulationTypeFromName(w.Type),
		TypeRaw:                w.Type,
		Transaction:            SimulationTransaction{TotalCredit: w.Transaction.TotalCredit, TotalDebit: w.Transaction.TotalDebit},
		Interest:               SimulationRates{MonthlyRate: w.Interest.MonthlyRate, AnnualRate: w.Interest.AnnualRate},
		Cet:                    SimulationRates{MonthlyRate: w.Cet.MonthlyRate, AnnualRate: w.Cet.AnnualRate},
		Iof:                    SimulationIof{Amount: w.Iof.Amount, Rate: w.Iof.Rate},
		Vehicle: SimulationVehicle{
			Brand:        w.Vehicle.Brand,
			Model:        w.Vehicle.Model,
			Version:      w.Vehicle.Version,
			Year:         w.Vehicle.Year,
			Value:        w.Vehicle.Value,
			LicensePlate: w.Vehicle.LicensePlate,
			IsZeroKm:     w.Vehicle.IsZeroKm,
			Chassis:      w.Vehicle.Chassis,
			Renavam:      w.Vehicle.Renavam,
			Color:        w.Vehicle.Color,
		},
		CustomerCpf: w.Customer.Cpf,
		CreatedAt:   w.CreatedAt,
	}
}
