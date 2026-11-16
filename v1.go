package credicarro

import (
	"context"
	"net/url"

	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/wire"
)

// V1Client agrupa as rotas v1 sem cnpjLoja (a loja é a vinculada ao parceiro).
// Para novas integrações prefira a v2 em Client.Simulations e Client.Proposals.
type V1Client struct {
	Simulations *V1SimulationsClient
	Proposals   *V1ProposalsClient
}

// V1CreateFinancingSimulationRequest é a requisição de simulação de financiamento v1 (sem a loja).
type V1CreateFinancingSimulationRequest struct {
	Cpf           string
	LicensePlate  string
	VehicleAmount float64
	DownPayment   float64
	State         string
	IsZeroKm      bool
}

// CreateFinancingWithoutCpfRequest é a requisição da simulação de financiamento sem CPF (v1).
type CreateFinancingWithoutCpfRequest struct {
	LicensePlate  string
	VehicleAmount float64
	// DownPayment é opcional: nil usa a entrada mínima da taxa da loja e omite valorEntrada do JSON.
	DownPayment *float64
	State       string
}

// CreateRefinancingWithoutCpfRequest é a requisição da simulação de refinanciamento sem CPF (v1).
type CreateRefinancingWithoutCpfRequest struct {
	LicensePlate    string
	RequestedAmount float64
	State           string
}

// V1CreateRefinancingSimulationRequest é a requisição de simulação de refinanciamento v1 (sem a loja).
type V1CreateRefinancingSimulationRequest struct {
	Cpf             string
	LicensePlate    string
	RequestedAmount float64
}

// V1SimulationsClient expõe as simulações v1.
type V1SimulationsClient struct {
	api *apiCaller
}

// GetFinancingRange consulta o valor mínimo de entrada por prazo (GET v1/proposal/simulation/range/financing).
func (c *V1SimulationsClient) GetFinancingRange(ctx context.Context, licensePlate string) ([]FinancingRangeOption, error) {
	if err := requireNonEmpty("licensePlate", licensePlate); err != nil {
		return nil, err
	}
	return getFinancingRange(ctx, c.api, "v1/proposal/simulation/range/financing", rangeQuery(licensePlate, "", false))
}

// GetRefinancingRange consulta a faixa refinanciável por prazo (GET v1/proposal/simulation/range/refinancing).
func (c *V1SimulationsClient) GetRefinancingRange(ctx context.Context, licensePlate string) ([]RefinancingRangeOption, error) {
	if err := requireNonEmpty("licensePlate", licensePlate); err != nil {
		return nil, err
	}
	return getRefinancingRange(ctx, c.api, "v1/proposal/simulation/range/refinancing", rangeQuery(licensePlate, "", false))
}

// CreateFinancing cria a simulação de financiamento v1 (POST v1/proposal/simulation/financing).
// Recebe Idempotency-Key automática.
func (c *V1SimulationsClient) CreateFinancing(ctx context.Context, req V1CreateFinancingSimulationRequest, opts ...RequestOption) ([]SimulationOption, error) {
	ropts, err := applyRequestOptions(opts)
	if err != nil {
		return nil, err
	}
	wireReq := wire.FinancingSimulationRequestV1{
		Cpf:           req.Cpf,
		IsZeroKm:      req.IsZeroKm,
		LicensePlate:  req.LicensePlate,
		VehicleAmount: req.VehicleAmount,
		State:         req.State,
		DownPayment:   req.DownPayment,
	}
	return postSimulation(ctx, c.api, "v1/proposal/simulation/financing", wireReq, ropts)
}

// CreateFinancingWithoutCpf simula um financiamento só com placa, valor do veículo e UF, sem CPF
// e sem cadastro de cliente (POST v1/proposal/simulation/financing/without-cpf) e devolve uma
// opção por prazo. Usa a loja de integração do parceiro. Nada é gravado: as opções não têm
// SimulationID e não servem para criar proposta. Não usa Idempotency-Key e é seguro repetir.
func (c *V1SimulationsClient) CreateFinancingWithoutCpf(ctx context.Context, req CreateFinancingWithoutCpfRequest, opts ...RequestOption) ([]FinancingWithoutCpfOption, error) {
	ropts, err := applyRequestOptions(opts)
	if err != nil {
		return nil, err
	}
	wireReq := wire.FinancingWithoutCpfRequest{
		LicensePlate:  req.LicensePlate,
		VehicleAmount: req.VehicleAmount,
		DownPayment:   req.DownPayment,
		State:         req.State,
	}
	var result wire.FinancingWithoutCpfResponse
	if err := c.api.post(ctx, "v1/proposal/simulation/financing/without-cpf", wireReq, &result, ropts); err != nil {
		return nil, err
	}
	options := make([]FinancingWithoutCpfOption, 0, len(result.Options))
	for _, o := range result.Options {
		options = append(options, FinancingWithoutCpfOption{
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
			MinimumDownPayment:        o.MinimumDownPayment,
		})
	}
	return options, nil
}

// CreateRefinancingWithoutCpf simula um refinanciamento só com placa, valor solicitado e UF, sem CPF
// e sem cadastro de cliente (POST v1/proposal/simulation/refinancing/without-cpf) e devolve uma
// opção por prazo em cuja faixa o valor cabe. Usa a loja de integração do parceiro. Nada é gravado:
// as opções não têm SimulationID e não servem para criar proposta. Não usa Idempotency-Key e é
// seguro repetir.
func (c *V1SimulationsClient) CreateRefinancingWithoutCpf(ctx context.Context, req CreateRefinancingWithoutCpfRequest, opts ...RequestOption) ([]RefinancingWithoutCpfOption, error) {
	ropts, err := applyRequestOptions(opts)
	if err != nil {
		return nil, err
	}
	wireReq := wire.RefinancingWithoutCpfRequest{
		LicensePlate:    req.LicensePlate,
		RequestedAmount: req.RequestedAmount,
		State:           req.State,
	}
	var result wire.RefinancingWithoutCpfResponse
	if err := c.api.post(ctx, "v1/proposal/simulation/refinancing/without-cpf", wireReq, &result, ropts); err != nil {
		return nil, err
	}
	options := make([]RefinancingWithoutCpfOption, 0, len(result.Options))
	for _, o := range result.Options {
		options = append(options, RefinancingWithoutCpfOption{
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
			MinimumAmount:             o.MinimumAmount,
			MaximumAmount:             o.MaximumAmount,
		})
	}
	return options, nil
}

// CreateRefinancing cria a simulação de refinanciamento v1 (POST v1/proposal/simulation/refinancing).
// Recebe Idempotency-Key automática.
func (c *V1SimulationsClient) CreateRefinancing(ctx context.Context, req V1CreateRefinancingSimulationRequest, opts ...RequestOption) ([]SimulationOption, error) {
	ropts, err := applyRequestOptions(opts)
	if err != nil {
		return nil, err
	}
	wireReq := wire.RefinancingSimulationRequestV1{
		Cpf:             req.Cpf,
		LicensePlate:    req.LicensePlate,
		RequestedAmount: req.RequestedAmount,
	}
	return postSimulation(ctx, c.api, "v1/proposal/simulation/refinancing", wireReq, ropts)
}

// V1ProposalsClient expõe as propostas v1.
type V1ProposalsClient struct {
	api *apiCaller
}

// Create cria uma proposta a partir de uma simulação (POST v1/proposal) e devolve o id_proposta.
// Recebe Idempotency-Key automática.
func (c *V1ProposalsClient) Create(ctx context.Context, req CreateProposalRequest, opts ...RequestOption) (string, error) {
	return createProposal(ctx, c.api, "v1/proposal", req, opts)
}

// Get consulta uma proposta v1 (GET v1/proposal/{id}).
func (c *V1ProposalsClient) Get(ctx context.Context, proposalID string) (*ProposalSummary, error) {
	if err := requireUUID("proposalID", proposalID); err != nil {
		return nil, err
	}
	var result wire.ProposalV1
	if err := c.api.get(ctx, "v1/proposal/"+url.PathEscape(proposalID), &result); err != nil {
		return nil, err
	}
	summary := proposalSummaryFromWire(result)
	return &summary, nil
}
