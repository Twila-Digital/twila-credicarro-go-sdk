package credicarro

import (
	"context"
	"net/url"
	"time"

	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/wire"
)

// CreateProposalRequest cria uma proposta a partir de uma opção de simulação.
type CreateProposalRequest struct {
	Cpf string
	// SimulationID é o id_simulacao da opção escolhida (UUID). Cada opção gera uma única proposta.
	SimulationID string
}

// VehicleDetails são os dados do veículo da proposta.
type VehicleDetails struct {
	Renavam      string // até 11 dígitos, com dígito verificador válido
	Chassis      string // 17 caracteres, sem as letras I, O e Q
	LicensePlate string // ABC-1234, ABC1234 ou ABC1D23
	Color        string // sem números, até 30 caracteres
}

// DepositAccount é a conta de recebimento de uma proposta de refinanciamento.
type DepositAccount struct {
	HolderCpf     string // CPF do titular; deve ser o cliente da proposta
	BankCode      string // código COMPE, 3 dígitos
	Branch        string // agência, até 4 dígitos, sem dígito
	BranchDigit   string // número ou X
	AccountNumber string // 4 a 13 dígitos, sem dígito
	AccountDigit  string // número ou X
	AccountType   DepositAccountType
}

// ProposalDocumentUpload descreve o envio de um documento de um participante.
type ProposalDocumentUpload struct {
	Participant  ProposalParticipant
	DocumentType DocumentType
	File         DocumentFile
}

// ProposalsClient expõe as operações de propostas (v2, com a listagem v1).
type ProposalsClient struct {
	api                          *apiCaller
	documentUploadAttemptTimeout time.Duration
}

// Create cria uma proposta (POST v2/proposal) e devolve o id_proposta. Recebe
// Idempotency-Key automática (a mesma em todas as tentativas).
func (c *ProposalsClient) Create(ctx context.Context, req CreateProposalRequest, opts ...RequestOption) (string, error) {
	return createProposal(ctx, c.api, "v2/proposal", req, opts)
}

// Get consulta o retrato atual da proposta (GET v2/proposal/{id}). Use
// ProposalDetails.PendingActions para saber o que o integrador precisa enviar.
func (c *ProposalsClient) Get(ctx context.Context, proposalID string) (*ProposalDetails, error) {
	if err := requireUUID("proposalID", proposalID); err != nil {
		return nil, err
	}
	var result wire.ProposalV2
	if err := c.api.get(ctx, "v2/proposal/"+url.PathEscape(proposalID), &result); err != nil {
		return nil, err
	}
	details := proposalDetailsFromWire(result)
	return &details, nil
}

// List lista as propostas, paginadas (GET v1/proposal). req nil usa a página 1 com 10 itens.
func (c *ProposalsClient) List(ctx context.Context, req *ListRequest) (*PagedResult[ProposalSummary], error) {
	query, err := req.query()
	if err != nil {
		return nil, err
	}
	var result wire.PagedProposals
	if err := c.api.get(ctx, "v1/proposal?"+query, &result); err != nil {
		return nil, err
	}

	items := make([]ProposalSummary, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, proposalSummaryFromWire(item))
	}
	return &PagedResult[ProposalSummary]{
		Items:       items,
		Page:        result.Page.Number,
		PageSize:    result.Page.Size,
		TotalCount:  result.Page.Total,
		HasNext:     result.Page.HasNext,
		HasPrevious: result.Page.HasPrevious,
	}, nil
}

// SendVehicleDetails inclui ou atualiza os dados do veículo (POST v2/proposal/{id}/vehicle-detail,
// 204). Aceito só enquanto a etapa DADOS_VEICULO estiver ativa. Reenviar substitui os dados,
// por isso a chamada pode ser repetida em falhas transitórias.
func (c *ProposalsClient) SendVehicleDetails(ctx context.Context, proposalID string, details VehicleDetails) error {
	if err := requireUUID("proposalID", proposalID); err != nil {
		return err
	}
	wireReq := wire.VehicleDetailsRequest{
		Renavam:      details.Renavam,
		Chassis:      details.Chassis,
		LicensePlate: details.LicensePlate,
		Color:        details.Color,
	}
	return c.api.post(ctx, "v2/proposal/"+url.PathEscape(proposalID)+"/vehicle-detail", wireReq, nil, noOptions)
}

// SendDepositAccount inclui ou substitui a conta de recebimento de uma proposta de
// refinanciamento (POST v2/proposal/{id}/deposit-account, 204). Reenviar substitui a conta
// anterior, por isso a chamada pode ser repetida em falhas transitórias.
func (c *ProposalsClient) SendDepositAccount(ctx context.Context, proposalID string, account DepositAccount) error {
	if err := requireUUID("proposalID", proposalID); err != nil {
		return err
	}
	wireReq := wire.DepositAccountRequest{
		HolderCpf:     account.HolderCpf,
		BankCode:      account.BankCode,
		Branch:        account.Branch,
		BranchDigit:   account.BranchDigit,
		AccountNumber: account.AccountNumber,
		AccountDigit:  account.AccountDigit,
		AccountType:   string(account.AccountType),
	}
	return c.api.post(ctx, "v2/proposal/"+url.PathEscape(proposalID)+"/deposit-account", wireReq, nil, noOptions)
}

// UploadDocument envia um documento do comprador, do vendedor ou da loja (POST
// v2/proposal/{id}/document, 204), um arquivo por chamada.
//
// A API exige o header Idempotency-Key. Sem WithIdempotencyKey o SDK gera uma chave por
// chamada e a reusa em todas as tentativas. O recomendado é o integrador persistir uma
// chave por envio e reaproveitá-la ao repetir o mesmo envio; use uma chave nova ao
// reenviar a correção de um documento recusado (INVALIDO). Com a mesma chave e o mesmo
// conteúdo, um envio bem-sucedido nas últimas 24 horas não é processado de novo.
func (c *ProposalsClient) UploadDocument(ctx context.Context, proposalID string, upload ProposalDocumentUpload, opts ...RequestOption) error {
	if err := requireUUID("proposalID", proposalID); err != nil {
		return err
	}
	switch upload.Participant {
	case ProposalParticipantBuyer, ProposalParticipantSeller, ProposalParticipantStore:
	default:
		return &ConfigurationError{Message: "Participant deve ser ProposalParticipantBuyer, ProposalParticipantSeller ou ProposalParticipantStore."}
	}
	if upload.DocumentType <= 0 {
		return &ConfigurationError{Message: "DocumentType é obrigatório."}
	}
	if err := upload.File.validate(); err != nil {
		return err
	}
	ropts, err := applyRequestOptions(opts)
	if err != nil {
		return err
	}
	ropts.RequireKey = true
	ropts.AttemptTimeout = c.documentUploadAttemptTimeout

	wireReq := wire.DocumentUploadRequest{
		Participant:   string(upload.Participant),
		DocumentType:  int(upload.DocumentType),
		FileName:      upload.File.fileName,
		Base64Content: upload.File.base64,
	}
	return c.api.post(ctx, "v2/proposal/"+url.PathEscape(proposalID)+"/document", wireReq, nil, ropts)
}

func createProposal(ctx context.Context, api *apiCaller, path string, req CreateProposalRequest, opts []RequestOption) (string, error) {
	if err := requireUUID("SimulationID", req.SimulationID); err != nil {
		return "", err
	}
	ropts, err := applyRequestOptions(opts)
	if err != nil {
		return "", err
	}
	var result wire.CreateProposalResponse
	wireReq := wire.CreateProposalRequest{Cpf: req.Cpf, SimulationID: req.SimulationID}
	if err := api.post(ctx, path, wireReq, &result, ropts); err != nil {
		return "", err
	}
	return result.ProposalID, nil
}
