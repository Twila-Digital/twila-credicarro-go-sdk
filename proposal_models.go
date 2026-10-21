package credicarro

import (
	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/wire"
)

// ActiveStepState é a etapa em que o participante está agora, a situação dela e quem
// precisa agir. Cada enum vem com o valor bruto (XxxRaw) e o nome (XxxName).
type ActiveStepState struct {
	Step            ProposalStep
	StepRaw         int
	StepName        string
	Situation       StepSituation
	SituationRaw    int
	SituationName   string
	ActionOwner     ActionOwner
	ActionOwnerRaw  int
	ActionOwnerName string
}

// CompletedStepState é uma etapa já concluída do participante.
type CompletedStepState struct {
	Step     ProposalStep
	StepRaw  int
	StepName string
	// Date é a data da última passagem pela etapa (ISO-8601, como veio da API).
	Date string
}

// ProposalDocumentState é um documento do participante na proposta.
type ProposalDocumentState struct {
	Type       DocumentType
	TypeRaw    int
	TypeName   string
	Status     DocumentStatus
	StatusRaw  int
	StatusName string
	// Reason é o motivo da recusa (vazio quando não houver).
	Reason string
}

// ProposalParticipantState reúne o estado de um participante (comprador, vendedor ou
// loja) na proposta. É o mesmo tipo em ProposalDetails e no webhook de proposta.
type ProposalParticipantState struct {
	// Document é o CPF (comprador/vendedor) ou o CNPJ (loja).
	Document string
	// Biometry é nil quando o participante não tem biometria (ex.: loja).
	Biometry       *BiometryStatus
	BiometryRaw    int
	BiometryName   string
	ActiveStep     *ActiveStepState
	CompletedSteps []CompletedStepState
	Documents      []ProposalDocumentState
}

func activeStepFromWire(w *wire.ActiveStep) *ActiveStepState {
	if w == nil {
		return nil
	}
	return &ActiveStepState{
		Step:            proposalStepFromInt(intFromPtr(w.Step)),
		StepRaw:         intFromPtr(w.Step),
		StepName:        w.StepName,
		Situation:       stepSituationFromInt(intFromPtr(w.Situation)),
		SituationRaw:    intFromPtr(w.Situation),
		SituationName:   w.SituationName,
		ActionOwner:     actionOwnerFromInt(intFromPtr(w.ActionOwner)),
		ActionOwnerRaw:  intFromPtr(w.ActionOwner),
		ActionOwnerName: w.ActionOwnerName,
	}
}

func completedStepsFromWire(items []wire.CompletedStep) []CompletedStepState {
	steps := make([]CompletedStepState, 0, len(items))
	for _, item := range items {
		steps = append(steps, CompletedStepState{
			Step:     proposalStepFromInt(intFromPtr(item.Step)),
			StepRaw:  intFromPtr(item.Step),
			StepName: item.StepName,
			Date:     item.Date,
		})
	}
	return steps
}

func documentsFromWire(items []wire.Document) []ProposalDocumentState {
	docs := make([]ProposalDocumentState, 0, len(items))
	for _, item := range items {
		docs = append(docs, ProposalDocumentState{
			Type:       documentTypeFromInt(intFromPtr(item.Type)),
			TypeRaw:    intFromPtr(item.Type),
			TypeName:   item.TypeName,
			Status:     documentStatusFromInt(intFromPtr(item.Status)),
			StatusRaw:  intFromPtr(item.Status),
			StatusName: item.StatusName,
			Reason:     item.Reason,
		})
	}
	return docs
}

func participantFromWire(w *wire.Participant) *ProposalParticipantState {
	if w == nil {
		return nil
	}
	state := &ProposalParticipantState{
		Document:       w.Cpf,
		BiometryName:   w.BiometryName,
		ActiveStep:     activeStepFromWire(w.ActiveStep),
		CompletedSteps: completedStepsFromWire(w.CompletedSteps),
		Documents:      documentsFromWire(w.Documents),
	}
	if w.Biometry != nil {
		status := biometryStatusFromInt(*w.Biometry)
		state.Biometry = &status
		state.BiometryRaw = *w.Biometry
	}
	return state
}

func storeFromWire(w *wire.Store) *ProposalParticipantState {
	if w == nil {
		return nil
	}
	return &ProposalParticipantState{
		Document:       w.Cnpj,
		ActiveStep:     activeStepFromWire(w.ActiveStep),
		CompletedSteps: completedStepsFromWire(w.CompletedSteps),
		Documents:      documentsFromWire(w.Documents),
	}
}

// ProposalDetails é o retrato atual da proposta (GET v2/proposal/{id}).
type ProposalDetails struct {
	ID           string
	Number       int64
	SimulationID string
	// Status vem de enum_status; StatusRaw é o código bruto e StatusName o nome (ex.: APROVADA).
	Status     ProposalStatus
	StatusRaw  int
	StatusName string
	// Type vem de "tipo" (Financing/Refinancing); TypeRaw guarda o texto original.
	Type    ProposalType
	TypeRaw string
	// CreatedAt é a data de criação (ISO-8601, como veio da API).
	CreatedAt     string
	RefusalReason string
	BiometryLink  string
	Buyer         *ProposalParticipantState
	Seller        *ProposalParticipantState
	Store         *ProposalParticipantState
}

func proposalDetailsFromWire(w wire.ProposalV2) ProposalDetails {
	details := ProposalDetails{
		ID:            w.ProposalID,
		Number:        w.Number,
		SimulationID:  w.SimulationID,
		StatusName:    w.StatusName,
		Type:          proposalTypeFromName(w.Type),
		TypeRaw:       w.Type,
		CreatedAt:     w.CreatedAt,
		RefusalReason: w.RefusalReason,
		BiometryLink:  w.BiometryLink,
		Buyer:         participantFromWire(w.Buyer),
		Seller:        participantFromWire(w.Seller),
		Store:         storeFromWire(w.Store),
	}
	if w.Status != nil {
		details.Status = proposalStatusFromInt(*w.Status)
		details.StatusRaw = *w.Status
	} else {
		details.Status = proposalStatusFromName(w.StatusName)
	}
	return details
}

// PendingActionKind é o que o integrador precisa fazer numa etapa.
type PendingActionKind string

const (
	// PendingActionSendDocuments: enviar os documentos pendentes (etapa ENVIO_DOCUMENTOS).
	PendingActionSendDocuments PendingActionKind = "SendDocuments"
	// PendingActionCorrectDocuments: reenviar os documentos recusados (etapa CORRECAO_DOCUMENTOS).
	PendingActionCorrectDocuments PendingActionKind = "CorrectDocuments"
	// PendingActionSendVehicleDetails: enviar os dados do veículo (etapa DADOS_VEICULO).
	PendingActionSendVehicleDetails PendingActionKind = "SendVehicleDetails"
	// PendingActionSendDepositAccount: enviar a conta de recebimento (etapa CONTA_RECEBIMENTO).
	PendingActionSendDepositAccount PendingActionKind = "SendDepositAccount"
)

// PendingAction é uma ação que a proposta espera do integrador.
type PendingAction struct {
	Participant ProposalParticipant
	Step        ProposalStep
	Kind        PendingActionKind
	// DocumentTypes lista os tipos a enviar (SendDocuments) ou a reenviar (CorrectDocuments).
	DocumentTypes []DocumentType
}

// PendingActions lista o que a proposta espera do integrador: só as etapas ativas com
// ActionOwner == ActionOwnerIntegrator. Etapa 6 => SendDocuments (documentos Pending),
// 7 => CorrectDocuments (documentos Invalid), 8 => SendVehicleDetails, 5 => SendDepositAccount.
func (p *ProposalDetails) PendingActions() []PendingAction {
	if p == nil {
		return nil
	}

	actions := []PendingAction{}
	participants := []struct {
		who   ProposalParticipant
		state *ProposalParticipantState
	}{
		{ProposalParticipantBuyer, p.Buyer},
		{ProposalParticipantSeller, p.Seller},
		{ProposalParticipantStore, p.Store},
	}

	for _, entry := range participants {
		state := entry.state
		if state == nil || state.ActiveStep == nil || state.ActiveStep.ActionOwner != ActionOwnerIntegrator {
			continue
		}

		step := state.ActiveStep.Step
		action := PendingAction{Participant: entry.who, Step: step}
		switch step {
		case ProposalStepDocumentSubmission:
			action.Kind = PendingActionSendDocuments
			action.DocumentTypes = documentTypesWithStatus(state.Documents, DocumentStatusPending)
		case ProposalStepDocumentCorrection:
			action.Kind = PendingActionCorrectDocuments
			action.DocumentTypes = documentTypesWithStatus(state.Documents, DocumentStatusInvalid)
		case ProposalStepVehicleDetails:
			action.Kind = PendingActionSendVehicleDetails
		case ProposalStepDepositAccount:
			action.Kind = PendingActionSendDepositAccount
		default:
			continue
		}
		actions = append(actions, action)
	}
	return actions
}

func documentTypesWithStatus(docs []ProposalDocumentState, status DocumentStatus) []DocumentType {
	types := []DocumentType{}
	for _, doc := range docs {
		if doc.Status == status {
			types = append(types, doc.Type)
		}
	}
	return types
}

// ProposalSummary é o resumo da proposta das rotas v1 (listagem e consulta v1).
type ProposalSummary struct {
	ID           string
	Number       int64
	SimulationID string
	Type         ProposalType
	TypeRaw      string
	// Status vem do nome em inglês (ex.: Approved); StatusName guarda o texto original.
	Status     ProposalStatus
	StatusName string
	BuyerCpf   string
	// SellerCpf é vazio quando a proposta não tem vendedor.
	SellerCpf string
	CreatedAt string
}

func proposalSummaryFromWire(w wire.ProposalV1) ProposalSummary {
	summary := ProposalSummary{
		ID:           w.ProposalID,
		Number:       w.Number,
		SimulationID: w.SimulationID,
		Type:         proposalTypeFromName(w.Type),
		TypeRaw:      w.Type,
		Status:       proposalStatusFromName(w.Status),
		StatusName:   w.Status,
		CreatedAt:    w.CreatedAt,
	}
	if w.Buyer != nil {
		summary.BuyerCpf = w.Buyer.Cpf
	}
	if w.Seller != nil {
		summary.SellerCpf = w.Seller.Cpf
	}
	return summary
}
