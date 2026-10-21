package credicarro

import (
	"fmt"
	"unicode"
)

// Os tipos que carregam CPF, e-mail, celular, renda, conta bancária, credencial, segredo
// ou conteúdo de documento implementam String() e GoString() com esses valores mascarados:
// CPF vira ***.456.789-** e os demais campos sensíveis viram ***. Assim, %v, %+v, %#v e
// fmt.Print nunca vazam dados pessoais em logs.

const masked = "***"

// maskCPF mostra só os dígitos centrais de um CPF de 11 dígitos (***.456.789-**).
func maskCPF(value string) string {
	if value == "" {
		return ""
	}
	var digits []rune
	for _, r := range value {
		if unicode.IsDigit(r) {
			digits = append(digits, r)
		}
	}
	if len(digits) != 11 {
		return masked
	}
	return fmt.Sprintf("***.%s.%s-**", string(digits[3:6]), string(digits[6:9]))
}

// maskValue mascara um valor sensível por inteiro.
func maskValue(value string) string {
	if value == "" {
		return ""
	}
	return masked
}

func (r CreateCustomerRequest) String() string {
	return fmt.Sprintf("CreateCustomerRequest{Cpf: %s, FirstName: %s, LastName: %s, Email: %s, PhoneNumber: %s, DateOfBirth: %s}",
		maskCPF(r.Cpf), maskValue(r.FirstName), maskValue(r.LastName), maskValue(r.Email), maskValue(r.PhoneNumber), maskValue(r.DateOfBirth))
}

// GoString implementa fmt.GoStringer (verbo %#v) sem dados pessoais.
func (r CreateCustomerRequest) GoString() string { return r.String() }

func (r UpsertCustomerRequest) String() string {
	return fmt.Sprintf("UpsertCustomerRequest{Cpf: %s, FirstName: %s, LastName: %s, Email: %s, PhoneNumber: %s, DateOfBirth: %s, Address: %s, ProfessionalData: %s}",
		maskCPF(r.Cpf), maskValue(r.FirstName), maskValue(r.LastName), maskValue(r.Email), maskValue(r.PhoneNumber), maskValue(r.DateOfBirth),
		r.Address.String(), r.ProfessionalData.String())
}

// GoString implementa fmt.GoStringer (verbo %#v) sem dados pessoais.
func (r UpsertCustomerRequest) GoString() string { return r.String() }

func (a CustomerAddress) String() string {
	return fmt.Sprintf("CustomerAddress{Street: %s, Number: %s, Complement: %s, Neighborhood: %s, City: %s, State: %s, PostalCode: %s}",
		maskValue(a.Street), maskValue(a.Number), maskValue(a.Complement), maskValue(a.Neighborhood), a.City, a.State, maskValue(a.PostalCode))
}

// GoString implementa fmt.GoStringer (verbo %#v) sem dados pessoais.
func (a CustomerAddress) GoString() string { return a.String() }

func (p ProfessionalData) String() string {
	return fmt.Sprintf("ProfessionalData{Occupation: %d, MonthlyIncome: %s}", p.Occupation, masked)
}

// GoString implementa fmt.GoStringer (verbo %#v) sem dados pessoais.
func (p ProfessionalData) GoString() string { return p.String() }

func (d DepositAccount) String() string {
	return fmt.Sprintf("DepositAccount{HolderCpf: %s, BankCode: %s, Branch: %s, BranchDigit: %s, AccountNumber: %s, AccountDigit: %s, AccountType: %s}",
		maskCPF(d.HolderCpf), d.BankCode, maskValue(d.Branch), maskValue(d.BranchDigit), maskValue(d.AccountNumber), maskValue(d.AccountDigit), d.AccountType)
}

// GoString implementa fmt.GoStringer (verbo %#v) sem dados bancários.
func (d DepositAccount) GoString() string { return d.String() }

func (r CreateProposalRequest) String() string {
	return fmt.Sprintf("CreateProposalRequest{Cpf: %s, SimulationID: %s}", maskCPF(r.Cpf), r.SimulationID)
}

// GoString implementa fmt.GoStringer (verbo %#v) sem CPF.
func (r CreateProposalRequest) GoString() string { return r.String() }

func (r CreateFinancingSimulationRequest) String() string {
	return fmt.Sprintf("CreateFinancingSimulationRequest{Cpf: %s, LicensePlate: %s, VehicleAmount: %v, DownPayment: %v, State: %s, IsZeroKm: %v, StoreCnpj: %s}",
		maskCPF(r.Cpf), r.LicensePlate, r.VehicleAmount, r.DownPayment, r.State, r.IsZeroKm, r.StoreCnpj)
}

// GoString implementa fmt.GoStringer (verbo %#v) sem CPF.
func (r CreateFinancingSimulationRequest) GoString() string { return r.String() }

func (r CreateRefinancingSimulationRequest) String() string {
	return fmt.Sprintf("CreateRefinancingSimulationRequest{Cpf: %s, LicensePlate: %s, RequestedAmount: %v, StoreCnpj: %s}",
		maskCPF(r.Cpf), r.LicensePlate, r.RequestedAmount, r.StoreCnpj)
}

// GoString implementa fmt.GoStringer (verbo %#v) sem CPF.
func (r CreateRefinancingSimulationRequest) GoString() string { return r.String() }

func (r V1CreateFinancingSimulationRequest) String() string {
	return fmt.Sprintf("V1CreateFinancingSimulationRequest{Cpf: %s, LicensePlate: %s, VehicleAmount: %v, DownPayment: %v, State: %s, IsZeroKm: %v}",
		maskCPF(r.Cpf), r.LicensePlate, r.VehicleAmount, r.DownPayment, r.State, r.IsZeroKm)
}

// GoString implementa fmt.GoStringer (verbo %#v) sem CPF.
func (r V1CreateFinancingSimulationRequest) GoString() string { return r.String() }

func (r V1CreateRefinancingSimulationRequest) String() string {
	return fmt.Sprintf("V1CreateRefinancingSimulationRequest{Cpf: %s, LicensePlate: %s, RequestedAmount: %v}",
		maskCPF(r.Cpf), r.LicensePlate, r.RequestedAmount)
}

// GoString implementa fmt.GoStringer (verbo %#v) sem CPF.
func (r V1CreateRefinancingSimulationRequest) GoString() string { return r.String() }

func (r ProposalDocumentUpload) String() string {
	return fmt.Sprintf("ProposalDocumentUpload{Participant: %s, DocumentType: %d, File: %s}", r.Participant, r.DocumentType, r.File.String())
}

// GoString implementa fmt.GoStringer (verbo %#v) sem o conteúdo do arquivo.
func (r ProposalDocumentUpload) GoString() string { return r.String() }

// GoString implementa fmt.GoStringer (verbo %#v) sem o conteúdo do arquivo.
func (d DocumentFile) GoString() string { return d.String() }

func (s ProposalParticipantState) String() string {
	return fmt.Sprintf("ProposalParticipantState{Document: %s, BiometryRaw: %d, ActiveStep: %v, CompletedSteps: %d, Documents: %d}",
		maskDocument(s.Document), s.BiometryRaw, s.ActiveStep != nil, len(s.CompletedSteps), len(s.Documents))
}

// GoString implementa fmt.GoStringer (verbo %#v) sem o documento do participante.
func (s ProposalParticipantState) GoString() string { return s.String() }

// participantString formata um participante possivelmente nulo, com o documento mascarado.
func participantString(s *ProposalParticipantState) string {
	if s == nil {
		return "<nil>"
	}
	return s.String()
}

func (d ProposalDocumentState) String() string {
	return fmt.Sprintf("ProposalDocumentState{Type: %d, TypeName: %s, Status: %d, StatusName: %s, Reason: %s}",
		d.TypeRaw, d.TypeName, d.StatusRaw, d.StatusName, maskValue(d.Reason))
}

// GoString implementa fmt.GoStringer (verbo %#v) sem o motivo da recusa.
func (d ProposalDocumentState) GoString() string { return d.String() }

// maskDocument mascara CPF (11 dígitos); qualquer outro documento (CNPJ) é mascarado por inteiro.
func maskDocument(value string) string { return maskCPF(value) }

func (p ProposalSummary) String() string {
	return fmt.Sprintf("ProposalSummary{ID: %s, Number: %d, SimulationID: %s, Type: %s, Status: %s, BuyerCpf: %s, SellerCpf: %s, CreatedAt: %s}",
		p.ID, p.Number, p.SimulationID, p.TypeRaw, p.StatusName, maskCPF(p.BuyerCpf), maskCPF(p.SellerCpf), p.CreatedAt)
}

// GoString implementa fmt.GoStringer (verbo %#v) sem CPF.
func (p ProposalSummary) GoString() string { return p.String() }

func (s Simulation) String() string {
	return fmt.Sprintf("Simulation{SimulationID: %s, Term: %d, RequestedAmount: %v, Type: %s, CustomerCpf: %s, CreatedAt: %s}",
		s.SimulationID, s.Term, s.RequestedAmount, s.TypeRaw, maskCPF(s.CustomerCpf), s.CreatedAt)
}

// GoString implementa fmt.GoStringer (verbo %#v) sem CPF.
func (s Simulation) GoString() string { return s.String() }

func (p ProposalDetails) String() string {
	return fmt.Sprintf("ProposalDetails{ID: %s, Number: %d, Status: %s, Type: %s, RefusalReason: %s, BiometryLink: %s, Buyer: %s, Seller: %s, Store: %s}",
		p.ID, p.Number, p.StatusName, p.TypeRaw, maskValue(p.RefusalReason), maskValue(p.BiometryLink),
		participantString(p.Buyer), participantString(p.Seller), participantString(p.Store))
}

// GoString implementa fmt.GoStringer (verbo %#v) sem dados pessoais.
func (p ProposalDetails) GoString() string { return p.String() }

func (e ProposalWebhookEvent) String() string {
	return fmt.Sprintf("ProposalWebhookEvent{ProposalID: %s, Status: %s, IsDetailed: %v, RefusalReason: %s, Buyer: %s, Seller: %s, Store: %s}",
		e.ProposalID, e.StatusName, e.IsDetailed, maskValue(e.RefusalReason),
		participantString(e.Buyer), participantString(e.Seller), participantString(e.Store))
}

// GoString implementa fmt.GoStringer (verbo %#v) sem dados pessoais.
func (e ProposalWebhookEvent) GoString() string { return e.String() }

// GoString implementa fmt.GoStringer (verbo %#v) sem o detalhe nem o corpo da resposta.
func (e *APIError) GoString() string {
	return fmt.Sprintf("APIError{StatusCode: %d, ErrorCode: %q, Title: %q}", e.StatusCode, e.ErrorCode, e.Title)
}

// GoString implementa fmt.GoStringer (verbo %#v) sem o detalhe nem o corpo da resposta.
func (e *AuthenticationError) GoString() string {
	return fmt.Sprintf("AuthenticationError{StatusCode: %d, ErrorCode: %q, Title: %q}", e.StatusCode, e.ErrorCode, e.Title)
}
