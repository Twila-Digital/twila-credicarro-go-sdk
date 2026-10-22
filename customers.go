package credicarro

import (
	"context"

	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/wire"
)

// CustomerAddress é o endereço do cliente (rua, bairro, cidade, estado e CEP obrigatórios).
type CustomerAddress struct {
	Street       string // rua
	Number       string // numero (opcional)
	Complement   string // complemento (opcional)
	Neighborhood string // bairro
	City         string // cidade
	State        string // estado (UF)
	PostalCode   string // cep (8 dígitos, com ou sem hífen)
}

// ProfessionalData são a ocupação e a renda mensal do cliente.
type ProfessionalData struct {
	Occupation    OccupationType // tipoOcupacao
	MonthlyIncome float64        // renda, em reais
}

// CreateCustomerRequest traz só os dados pessoais (POST v1/customer).
type CreateCustomerRequest struct {
	Cpf         string // com ou sem pontuação
	FirstName   string // nome
	LastName    string // sobrenome
	Email       string
	PhoneNumber string // celular, no formato +55 (DD) 99999-9999
	DateOfBirth string // dataDeNascimento, ISO-8601 com offset (ex.: 1990-05-20T00:00:00-03:00)
}

// UpsertCustomerRequest traz o cadastro completo (POST v1/customer/unified).
type UpsertCustomerRequest struct {
	Cpf              string
	FirstName        string
	LastName         string
	Email            string
	PhoneNumber      string
	DateOfBirth      string
	Address          CustomerAddress
	ProfessionalData ProfessionalData
}

// CustomersClient expõe as operações de clientes.
type CustomersClient struct {
	api *apiCaller
}

// Upsert cria ou atualiza o cliente com dados completos (POST v1/customer/unified) e
// devolve o id_cliente.
//
// Sem WithIdempotencyKey a rota é um upsert: cria o cliente ou atualiza o cadastro de um
// CPF existente. Com a chave ela só cria (CPF já cadastrado => 409 Cpf.CpfAlreadyAdded).
// Por isso o SDK NUNCA gera a chave automaticamente aqui, e sem chave a chamada não é
// repetida em caso de falha transitória.
func (c *CustomersClient) Upsert(ctx context.Context, req UpsertCustomerRequest, opts ...RequestOption) (string, error) {
	ropts, err := applyRequestOptions(opts)
	if err != nil {
		return "", err
	}

	wireReq := wire.UnifiedCustomerRequest{
		Phone:            req.PhoneNumber,
		Cpf:              req.Cpf,
		DateOfBirth:      req.DateOfBirth,
		Email:            req.Email,
		FirstName:        req.FirstName,
		LastName:         req.LastName,
		Address:          addressToWire(req.Address),
		ProfessionalData: professionalToWire(req.ProfessionalData),
	}

	var result wire.CustomerIDResponse
	if err := c.api.post(ctx, "v1/customer/unified", wireReq, &result, ropts); err != nil {
		return "", err
	}
	return result.CustomerID, nil
}

// Create cria um cliente só com os dados pessoais (POST v1/customer) e devolve o
// id_cliente. CPF, e-mail ou celular já cadastrados => 409. Recebe Idempotency-Key
// automática (a mesma em todas as tentativas).
func (c *CustomersClient) Create(ctx context.Context, req CreateCustomerRequest, opts ...RequestOption) (string, error) {
	ropts, err := applyRequestOptions(opts)
	if err != nil {
		return "", err
	}

	wireReq := wire.CreateCustomerRequest{
		Phone:       req.PhoneNumber,
		Cpf:         req.Cpf,
		DateOfBirth: req.DateOfBirth,
		Email:       req.Email,
		FirstName:   req.FirstName,
		LastName:    req.LastName,
	}

	var result wire.CustomerIDResponse
	if err := c.api.post(ctx, "v1/customer", wireReq, &result, ropts); err != nil {
		return "", err
	}
	return result.CustomerID, nil
}

// UpsertAddress cria ou atualiza o endereço de um cliente existente (POST v1/customer/address,
// resposta 202 sem corpo). Cliente inexistente => 404 Customer.Account.NotFound.
func (c *CustomersClient) UpsertAddress(ctx context.Context, cpf string, address CustomerAddress, opts ...RequestOption) error {
	if err := requireNonEmpty("cpf", cpf); err != nil {
		return err
	}
	ropts, err := applyRequestOptions(opts)
	if err != nil {
		return err
	}

	wireReq := wire.CustomerAddressRequest{Cpf: cpf, Address: addressToWire(address)}
	return c.api.post(ctx, "v1/customer/address", wireReq, nil, ropts)
}

// UpsertProfessionalData cria ou atualiza a ocupação e a renda de um cliente existente
// (POST v1/customer/professional, resposta 202 sem corpo).
func (c *CustomersClient) UpsertProfessionalData(ctx context.Context, cpf string, data ProfessionalData, opts ...RequestOption) error {
	if err := requireNonEmpty("cpf", cpf); err != nil {
		return err
	}
	ropts, err := applyRequestOptions(opts)
	if err != nil {
		return err
	}

	wireReq := wire.CustomerProfessionalRequest{Cpf: cpf, Occupation: int(data.Occupation), Income: data.MonthlyIncome}
	return c.api.post(ctx, "v1/customer/professional", wireReq, nil, ropts)
}

func addressToWire(a CustomerAddress) wire.Address {
	return wire.Address{
		Street:       a.Street,
		City:         a.City,
		State:        a.State,
		Neighborhood: a.Neighborhood,
		PostalCode:   a.PostalCode,
		Number:       a.Number,
		Complement:   a.Complement,
	}
}

func professionalToWire(p ProfessionalData) wire.ProfessionalData {
	return wire.ProfessionalData{Occupation: int(p.Occupation), Income: p.MonthlyIncome}
}
