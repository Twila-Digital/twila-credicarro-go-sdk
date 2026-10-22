package credicarro

import (
	"strings"
)

// ProposalStatus é o status da proposta (enum_status / status).
type ProposalStatus int

// Valores de ProposalStatus.
const (
	ProposalStatusUnknown        ProposalStatus = -1
	ProposalStatusAnalysing      ProposalStatus = 1 // EM_ANALISE / Analysing
	ProposalStatusApproved       ProposalStatus = 2 // APROVADA / Approved
	ProposalStatusRefused        ProposalStatus = 3 // RECUSADA / Refused
	ProposalStatusCanceled       ProposalStatus = 4 // CANCELADA / Canceled
	ProposalStatusManualAnalysis ProposalStatus = 5 // ANALISE_MANUAL / ManualAnalysis
	ProposalStatusCompleted      ProposalStatus = 6 // FINALIZADA / Completed
	ProposalStatusInNegotiation  ProposalStatus = 7 // EM_NEGOCIACAO / InNegotiation
	ProposalStatusExpired        ProposalStatus = 8 // EXPIRADA / Expired
	ProposalStatusPending        ProposalStatus = 9 // PENDENTE / Pending
)

var proposalStatusNames = map[string]ProposalStatus{
	"EM_ANALISE":     ProposalStatusAnalysing,
	"ANALYSING":      ProposalStatusAnalysing,
	"APROVADA":       ProposalStatusApproved,
	"APPROVED":       ProposalStatusApproved,
	"RECUSADA":       ProposalStatusRefused,
	"REFUSED":        ProposalStatusRefused,
	"CANCELADA":      ProposalStatusCanceled,
	"CANCELED":       ProposalStatusCanceled,
	"ANALISE_MANUAL": ProposalStatusManualAnalysis,
	"MANUALANALYSIS": ProposalStatusManualAnalysis,
	"FINALIZADA":     ProposalStatusCompleted,
	"COMPLETED":      ProposalStatusCompleted,
	"EM_NEGOCIACAO":  ProposalStatusInNegotiation,
	"INNEGOTIATION":  ProposalStatusInNegotiation,
	"EXPIRADA":       ProposalStatusExpired,
	"EXPIRED":        ProposalStatusExpired,
	"PENDENTE":       ProposalStatusPending,
	"PENDING":        ProposalStatusPending,
}

func proposalStatusFromInt(raw int) ProposalStatus {
	if raw >= int(ProposalStatusAnalysing) && raw <= int(ProposalStatusPending) {
		return ProposalStatus(raw)
	}
	return ProposalStatusUnknown
}

// proposalStatusFromName aceita os nomes em português (payload v2) e em inglês (v1).
func proposalStatusFromName(name string) ProposalStatus {
	if status, ok := proposalStatusNames[strings.ToUpper(strings.TrimSpace(name))]; ok {
		return status
	}
	return ProposalStatusUnknown
}

// ProposalType é a modalidade da proposta.
type ProposalType int

// Valores de ProposalType.
const (
	ProposalTypeUnknown     ProposalType = -1
	ProposalTypeFinancing   ProposalType = 1
	ProposalTypeRefinancing ProposalType = 2
)

func proposalTypeFromName(name string) ProposalType {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "financing":
		return ProposalTypeFinancing
	case "refinancing":
		return ProposalTypeRefinancing
	default:
		return ProposalTypeUnknown
	}
}

// SimulationType é o tipo da simulação (campo "tipo" de SimulationResponse).
type SimulationType int

// Valores de SimulationType.
const (
	SimulationTypeUnknown          SimulationType = -1
	SimulationTypeFinancing        SimulationType = 1
	SimulationTypeRefinancing      SimulationType = 2
	SimulationTypeAmountSimulation SimulationType = 3
)

func simulationTypeFromName(name string) SimulationType {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "financing":
		return SimulationTypeFinancing
	case "refinancing":
		return SimulationTypeRefinancing
	case "amountsimulation":
		return SimulationTypeAmountSimulation
	default:
		return SimulationTypeUnknown
	}
}

// CommissionReturnType é o tipo de simulação ao qual o retorno de comissão se aplica (tipo).
type CommissionReturnType int

// Valores de CommissionReturnType.
const (
	CommissionReturnTypeUnknown     CommissionReturnType = -1
	CommissionReturnTypeFinancing   CommissionReturnType = 1
	CommissionReturnTypeRefinancing CommissionReturnType = 2
)

func commissionReturnTypeFromName(name string) CommissionReturnType {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "financing":
		return CommissionReturnTypeFinancing
	case "refinancing":
		return CommissionReturnTypeRefinancing
	default:
		return CommissionReturnTypeUnknown
	}
}

// CommissionReturnTarget é a quem pertence a configuração de comissão (alvo).
type CommissionReturnTarget int

// Valores de CommissionReturnTarget.
const (
	CommissionReturnTargetUnknown      CommissionReturnTarget = -1
	CommissionReturnTargetStore        CommissionReturnTarget = 1
	CommissionReturnTargetOrganization CommissionReturnTarget = 2
)

func commissionReturnTargetFromName(name string) CommissionReturnTarget {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "store":
		return CommissionReturnTargetStore
	case "organization":
		return CommissionReturnTargetOrganization
	default:
		return CommissionReturnTargetUnknown
	}
}

// ProposalStep é a etapa de um participante da proposta (enum_etapa / etapa).
type ProposalStep int

// Valores de ProposalStep.
const (
	ProposalStepUnknown            ProposalStep = -1
	ProposalStepUndefined          ProposalStep = 0  // INDEFINIDO
	ProposalStepProposalCreated    ProposalStep = 1  // PROPOSTA_CRIADA
	ProposalStepProposalApproved   ProposalStep = 2  // PROPOSTA_APROVADA
	ProposalStepSellerNotification ProposalStep = 3  // NOTIFICACAO_VENDEDOR
	ProposalStepBiometryAnalysis   ProposalStep = 4  // ANALISE_BIOMETRIA
	ProposalStepDepositAccount     ProposalStep = 5  // CONTA_RECEBIMENTO
	ProposalStepDocumentSubmission ProposalStep = 6  // ENVIO_DOCUMENTOS
	ProposalStepDocumentCorrection ProposalStep = 7  // CORRECAO_DOCUMENTOS
	ProposalStepVehicleDetails     ProposalStep = 8  // DADOS_VEICULO
	ProposalStepInspection         ProposalStep = 9  // VISTORIA
	ProposalStepSignature          ProposalStep = 10 // ASSINATURA
	ProposalStepPayment            ProposalStep = 11 // PAGAMENTO
	ProposalStepProposalCompleted  ProposalStep = 12 // PROPOSTA_FINALIZADA
)

func proposalStepFromInt(raw int) ProposalStep {
	if raw >= int(ProposalStepUndefined) && raw <= int(ProposalStepProposalCompleted) {
		return ProposalStep(raw)
	}
	return ProposalStepUnknown
}

// StepSituation é a situação da etapa (enum_situacao / situacao).
type StepSituation int

// Valores de StepSituation.
const (
	StepSituationUnknown            StepSituation = -1
	StepSituationAwaitingSubmission StepSituation = 1 // AGUARDANDO_ENVIO
	StepSituationAwaitingCustomer   StepSituation = 2 // AGUARDANDO_CLIENTE
	StepSituationProcessing         StepSituation = 3 // EM_PROCESSAMENTO
	StepSituationPendingCorrection  StepSituation = 4 // PENDENTE_CORRECAO
	StepSituationBlocked            StepSituation = 5 // BLOQUEADA
	StepSituationCompleted          StepSituation = 6 // CONCLUIDA
	StepSituationCanceled           StepSituation = 7 // CANCELADA
)

func stepSituationFromInt(raw int) StepSituation {
	if raw >= int(StepSituationAwaitingSubmission) && raw <= int(StepSituationCanceled) {
		return StepSituation(raw)
	}
	return StepSituationUnknown
}

// ActionOwner é quem precisa agir na etapa (enum_responsavel_acao / responsavel_acao).
type ActionOwner int

// Valores de ActionOwner.
const (
	ActionOwnerUnknown    ActionOwner = -1
	ActionOwnerNone       ActionOwner = 0 // NENHUM
	ActionOwnerCustomer   ActionOwner = 1 // CLIENTE
	ActionOwnerIntegrator ActionOwner = 2 // INTEGRADOR
	ActionOwnerCredicarro ActionOwner = 3 // CREDICARRO
)

func actionOwnerFromInt(raw int) ActionOwner {
	if raw >= int(ActionOwnerNone) && raw <= int(ActionOwnerCredicarro) {
		return ActionOwner(raw)
	}
	return ActionOwnerUnknown
}

// BiometryStatus é a situação da biometria (enum_biometria / biometria).
type BiometryStatus int

// Valores de BiometryStatus.
const (
	BiometryStatusUnknown        BiometryStatus = -1
	BiometryStatusNotStarted     BiometryStatus = 0 // NAO_INICIADA
	BiometryStatusSessionCreated BiometryStatus = 1 // SESSAO_CRIADA
	BiometryStatusInAnalysis     BiometryStatus = 2 // EM_ANALISE
	BiometryStatusApproved       BiometryStatus = 3 // APROVADA
	BiometryStatusRejected       BiometryStatus = 4 // REPROVADA
)

func biometryStatusFromInt(raw int) BiometryStatus {
	if raw >= int(BiometryStatusNotStarted) && raw <= int(BiometryStatusRejected) {
		return BiometryStatus(raw)
	}
	return BiometryStatusUnknown
}

// DocumentStatus é o status de um documento da proposta (enum_status dentro de documentos).
type DocumentStatus int

// Valores de DocumentStatus.
const (
	DocumentStatusUnknown    DocumentStatus = -1
	DocumentStatusReceived   DocumentStatus = 0 // RECEBIDO
	DocumentStatusValid      DocumentStatus = 1 // VALIDO
	DocumentStatusInvalid    DocumentStatus = 2 // INVALIDO
	DocumentStatusInAnalysis DocumentStatus = 3 // EM_ANALISE
	DocumentStatusDeleted    DocumentStatus = 4 // EXCLUIDO
	DocumentStatusPending    DocumentStatus = 5 // PENDENTE (exigido e ainda não enviado)
)

func documentStatusFromInt(raw int) DocumentStatus {
	if raw >= int(DocumentStatusReceived) && raw <= int(DocumentStatusPending) {
		return DocumentStatus(raw)
	}
	return DocumentStatusUnknown
}

// DocumentType é o tipo de documento (código tipoDocumento no envio; enum_tipo na consulta).
type DocumentType int

// Valores de DocumentType.
const (
	DocumentTypeUnknown          DocumentType = -1
	DocumentTypeCnh              DocumentType = 1
	DocumentTypeCpf              DocumentType = 2
	DocumentTypeRg               DocumentType = 3
	DocumentTypeProofOfIncome    DocumentType = 4
	DocumentTypeProofOfAddress   DocumentType = 5
	DocumentTypeVehicleDocument  DocumentType = 6
	DocumentTypeDigitalCnh       DocumentType = 7
	DocumentTypeEcvReport        DocumentType = 8
	DocumentTypeInspectionReport DocumentType = 9
	DocumentTypeCnpjCard         DocumentType = 11
	DocumentTypeAtpv             DocumentType = 57
)

func documentTypeFromInt(raw int) DocumentType {
	switch DocumentType(raw) {
	case DocumentTypeCnh, DocumentTypeCpf, DocumentTypeRg, DocumentTypeProofOfIncome, DocumentTypeProofOfAddress,
		DocumentTypeVehicleDocument, DocumentTypeDigitalCnh, DocumentTypeEcvReport, DocumentTypeInspectionReport,
		DocumentTypeCnpjCard, DocumentTypeAtpv:
		return DocumentType(raw)
	default:
		return DocumentTypeUnknown
	}
}

// ProposalEventReason é o motivo do evento de webhook (motivo_evento).
type ProposalEventReason int

// Valores de ProposalEventReason.
const (
	ProposalEventReasonUnknown          ProposalEventReason = -1
	ProposalEventReasonStatusChanged    ProposalEventReason = 1 // STATUS_ALTERADO
	ProposalEventReasonStepsChanged     ProposalEventReason = 2 // ETAPAS_ALTERADAS
	ProposalEventReasonDocumentsChanged ProposalEventReason = 3 // DOCUMENTOS_ALTERADOS
)

func proposalEventReasonFromName(name string) ProposalEventReason {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "STATUS_ALTERADO":
		return ProposalEventReasonStatusChanged
	case "ETAPAS_ALTERADAS":
		return ProposalEventReasonStepsChanged
	case "DOCUMENTOS_ALTERADOS":
		return ProposalEventReasonDocumentsChanged
	default:
		return ProposalEventReasonUnknown
	}
}

// OccupationType é o tipo de ocupação do cliente (tipoOcupacao). Valores de requisição.
type OccupationType int

// Valores de OccupationType.
const (
	OccupationTypeRetired       OccupationType = 1
	OccupationTypePensioner     OccupationType = 2
	OccupationTypeEmployee      OccupationType = 3
	OccupationTypeSelfEmployed  OccupationType = 4
	OccupationTypeFreelancer    OccupationType = 6
	OccupationTypeBusinessOwner OccupationType = 7
)

// DepositAccountType é o tipo da conta de recebimento (tipoConta). Valores de requisição.
type DepositAccountType string

// Valores de DepositAccountType.
const (
	DepositAccountTypeChecking DepositAccountType = "CORRENTE"
	DepositAccountTypeSavings  DepositAccountType = "POUPANCA"
	DepositAccountTypePayment  DepositAccountType = "PAGAMENTO"
)

// ProposalParticipant é o participante dono do documento enviado (participante).
type ProposalParticipant string

// Valores de ProposalParticipant.
const (
	ProposalParticipantBuyer  ProposalParticipant = "COMPRADOR"
	ProposalParticipantSeller ProposalParticipant = "VENDEDOR"
	ProposalParticipantStore  ProposalParticipant = "LOJA"
)

// WebhookType é o evento notificado pelo webhook (tipo).
type WebhookType int

// Valores de WebhookType.
const (
	WebhookTypeUnknown    WebhookType = -1
	WebhookTypeCustomer   WebhookType = 1
	WebhookTypeSimulation WebhookType = 2
	WebhookTypeProposal   WebhookType = 3
)

func webhookTypeFromText(text string) WebhookType {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "customer", "1":
		return WebhookTypeCustomer
	case "simulation", "2":
		return WebhookTypeSimulation
	case "proposal", "3":
		return WebhookTypeProposal
	default:
		return WebhookTypeUnknown
	}
}

// WebhookAuthenticationType é a autenticação que a Credicarro envia ao chamar a URL (tipo_autenticacao).
type WebhookAuthenticationType int

// Valores de WebhookAuthenticationType.
const (
	WebhookAuthenticationTypeUnknown WebhookAuthenticationType = -1
	WebhookAuthenticationTypeNone    WebhookAuthenticationType = 1
	WebhookAuthenticationTypeBasic   WebhookAuthenticationType = 2
	WebhookAuthenticationTypeJwt     WebhookAuthenticationType = 3
)

func webhookAuthenticationTypeFromText(text string) WebhookAuthenticationType {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "none", "1":
		return WebhookAuthenticationTypeNone
	case "basic", "2":
		return WebhookAuthenticationTypeBasic
	case "jwt", "3":
		return WebhookAuthenticationTypeJwt
	default:
		return WebhookAuthenticationTypeUnknown
	}
}

// WebhookStatus é o status do webhook (status).
type WebhookStatus int

// Valores de WebhookStatus.
const (
	WebhookStatusUnknown  WebhookStatus = -1
	WebhookStatusActive   WebhookStatus = 1
	WebhookStatusInactive WebhookStatus = 2
)

func webhookStatusFromText(text string) WebhookStatus {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "active", "1":
		return WebhookStatusActive
	case "inactive", "2":
		return WebhookStatusInactive
	default:
		return WebhookStatusUnknown
	}
}

// intFromPtr devolve 0 para nil.
func intFromPtr(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}
