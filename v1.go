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
