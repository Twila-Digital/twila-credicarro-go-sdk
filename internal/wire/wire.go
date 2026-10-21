// Package wire contém os DTOs que espelham exatamente o formato JSON da API do
// Credicarro (nomes de campo em português). Nunca são expostos publicamente — o
// pacote raiz sempre mapeia pra/de tipos públicos em inglês.
//
// Os módulos de contract-tests comparam as tags JSON destes tipos com o openapi.yaml
// público; por isso cada tipo de requisição/resposta carrega só os campos do schema.
package wire

// Os DTOs espelham o JSON da API campo a campo (o openapi.yaml documenta cada um) e não são API pública.
//revive:disable:exported

// ProblemDetails é o corpo de erro padrão da API (campos em português).
type ProblemDetails struct {
	Type   string         `json:"tipo,omitempty"`
	Title  string         `json:"titulo,omitempty"`
	Status int            `json:"status,omitempty"`
	Detail string         `json:"detalhe,omitempty"`
	Field  string         `json:"campo,omitempty"`
	Errors []ProblemError `json:"erros,omitempty"`
}

// ProblemError é um item de ProblemDetails.Errors.
type ProblemError struct {
	Code    string `json:"codigo,omitempty"`
	Message string `json:"mensagem,omitempty"`
	Field   string `json:"campo,omitempty"`
}

// StdProblemDetails é o formato Problem Details padrão (RFC 7807), que a API usa em
// falhas inesperadas (500 e alguns 502). Fora do contrato do openapi.yaml.
type StdProblemDetails struct {
	Type   string `json:"type,omitempty"`
	Title  string `json:"title,omitempty"`
	Status int    `json:"status,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// Page é o bloco "pagina" das listagens.
type Page struct {
	HasNext     bool  `json:"tem_proximo"`
	HasPrevious bool  `json:"tem_anterior"`
	Number      int   `json:"numero"`
	Size        int   `json:"tamanho"`
	Total       int64 `json:"total"`
}

// ---------------------------------------------------------------- autenticação

type GenerateAccessTokenRequest struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

type GenerateAccessTokenResponse struct {
	Token      string `json:"token_de_acesso"`
	TokenType  string `json:"tipo_de_token"`
	ExpiresInS int64  `json:"expira_em_segundos"`
	ExpiresAt  string `json:"expira_em"`
	Scope      string `json:"escopo"`
}

// -------------------------------------------------------------------- clientes

type Address struct {
	Street       string `json:"rua"`
	City         string `json:"cidade"`
	State        string `json:"estado"`
	Neighborhood string `json:"bairro"`
	PostalCode   string `json:"cep"`
	Number       string `json:"numero,omitempty"`
	Complement   string `json:"complemento,omitempty"`
}

type ProfessionalData struct {
	Occupation int     `json:"tipoOcupacao"`
	Income     float64 `json:"renda"`
}

type CreateCustomerRequest struct {
	Phone       string `json:"celular"`
	Cpf         string `json:"cpf"`
	DateOfBirth string `json:"dataDeNascimento"`
	Email       string `json:"email"`
	FirstName   string `json:"nome"`
	LastName    string `json:"sobrenome"`
}

type UnifiedCustomerRequest struct {
	Phone            string           `json:"celular"`
	Cpf              string           `json:"cpf"`
	DateOfBirth      string           `json:"dataDeNascimento"`
	Email            string           `json:"email"`
	FirstName        string           `json:"nome"`
	LastName         string           `json:"sobrenome"`
	Address          Address          `json:"endereco"`
	ProfessionalData ProfessionalData `json:"dadosProfissionais"`
}

type CustomerAddressRequest struct {
	Cpf     string  `json:"cpf"`
	Address Address `json:"endereco"`
}

type CustomerProfessionalRequest struct {
	Cpf        string  `json:"cpf"`
	Occupation int     `json:"tipoOcupacao"`
	Income     float64 `json:"renda"`
}

type CustomerIDResponse struct {
	CustomerID string `json:"id_cliente"`
}

// ------------------------------------------------------------------ simulações

type FinancingRangeOption struct {
	Term               int     `json:"prazo"`
	MinimumDownPayment float64 `json:"valor_minimo_entrada"`
}

type FinancingRangeResponse struct {
	Options []FinancingRangeOption `json:"opcoes_por_prazo"`
}

type RefinancingRangeOption struct {
	Term          int     `json:"prazo"`
	MinimumAmount float64 `json:"valor_minimo_refinanciavel"`
	MaximumAmount float64 `json:"valor_maximo_refinanciavel"`
}

type RefinancingRangeResponse struct {
	Options []RefinancingRangeOption `json:"opcoes_por_prazo"`
}

// FinancingSimulationRequest é a requisição v2 (com cnpjLoja).
type FinancingSimulationRequest struct {
	Cpf           string  `json:"cpf"`
	LicensePlate  string  `json:"placa"`
	VehicleAmount float64 `json:"valorVeiculo"`
	DownPayment   float64 `json:"valorEntrada"`
	State         string  `json:"uf"`
	IsZeroKm      bool    `json:"isVeiculoZeroKm"`
	StoreCnpj     string  `json:"cnpjLoja"`
}

// FinancingSimulationRequestV1 é a requisição v1 (sem cnpjLoja).
type FinancingSimulationRequestV1 struct {
	Cpf           string  `json:"cpf"`
	IsZeroKm      bool    `json:"isVeiculoZeroKm"`
	LicensePlate  string  `json:"placa"`
	VehicleAmount float64 `json:"valorVeiculo"`
	State         string  `json:"uf"`
	DownPayment   float64 `json:"valorEntrada"`
}

// RefinancingSimulationRequest é a requisição v2 (com cnpjLoja).
type RefinancingSimulationRequest struct {
	Cpf             string  `json:"cpf"`
	LicensePlate    string  `json:"placa"`
	RequestedAmount float64 `json:"valorSolicitado"`
	StoreCnpj       string  `json:"cnpjLoja"`
}

// RefinancingSimulationRequestV1 é a requisição v1 (sem cnpjLoja).
type RefinancingSimulationRequestV1 struct {
	Cpf             string  `json:"cpf"`
	LicensePlate    string  `json:"placa"`
	RequestedAmount float64 `json:"valorSolicitado"`
}

type SimulationOption struct {
	SimulationID              string   `json:"id_simulacao"`
	RequestedAmount           float64  `json:"valor_solicitado"`
	Term                      int      `json:"prazo"`
	InstallmentAmount         float64  `json:"valor_parcela"`
	TacAmount                 float64  `json:"valor_tac"`
	BoletoAmount              float64  `json:"valor_boleto"`
	InsuranceAmount           float64  `json:"valor_seguro"`
	IofAmount                 float64  `json:"valor_iof"`
	IofRate                   float64  `json:"percentual_iof"`
	TotalCreditAmount         float64  `json:"valor_credito_total"`
	TotalDebtAmount           float64  `json:"valor_divida_total"`
	MonthlyCet                float64  `json:"percentual_cet_mensal"`
	AnnualCet                 float64  `json:"percentual_cet_anual"`
	MonthlyInterestRate       float64  `json:"percentual_juros_mensal"`
	AnnualInterestRate        float64  `json:"percentual_juros_anual"`
	ApproximateDailyRetention *float64 `json:"valor_retencao_diaria_aproximado,omitempty"`
}

type SimulationListResponse struct {
	Simulations []SimulationOption `json:"simulacoes"`
}

type SimulationTransaction struct {
	TotalCredit float64 `json:"valor_total_credito"`
	TotalDebit  float64 `json:"valor_total_debito"`
}

type SimulationInterest struct {
	MonthlyRate float64 `json:"percentual_juros_mensal"`
	AnnualRate  float64 `json:"percentual_juros_anual"`
}

type SimulationCet struct {
	MonthlyRate float64 `json:"percentual_mensal"`
	AnnualRate  float64 `json:"percentual_anual"`
}

type SimulationIof struct {
	Amount float64 `json:"valor"`
	Rate   float64 `json:"percentual"`
}

type SimulationVehicle struct {
	Brand        string   `json:"marca"`
	Model        string   `json:"modelo"`
	Version      string   `json:"versao"`
	Year         *int     `json:"ano"`
	Value        *float64 `json:"valor"`
	LicensePlate string   `json:"placa"`
	IsZeroKm     *bool    `json:"veiculo_zero_km"`
	Chassis      string   `json:"chassi"`
	Renavam      string   `json:"renavam"`
	Color        string   `json:"cor"`
}

type SimulationCustomer struct {
	Cpf string `json:"cpf"`
}

type Simulation struct {
	SimulationID           string                `json:"id_simulacao"`
	Term                   int                   `json:"prazo"`
	InstallmentAmount      float64               `json:"valor_parcela"`
	RequestedAmount        float64               `json:"valor_solicitado"`
	DownPayment            float64               `json:"valor_entrada"`
	AssetAmount            float64               `json:"valor_bem"`
	TacAmount              float64               `json:"valor_tac"`
	NegotiatedInterestRate float64               `json:"taxa_juros_negociada"`
	RegistrationAmount     float64               `json:"valor_registro"`
	State                  string                `json:"uf"`
	Type                   string                `json:"tipo"`
	Transaction            SimulationTransaction `json:"transacao"`
	Interest               SimulationInterest    `json:"juros"`
	Cet                    SimulationCet         `json:"cet"`
	Iof                    SimulationIof         `json:"iof"`
	Vehicle                SimulationVehicle     `json:"veiculo"`
	Customer               SimulationCustomer    `json:"cliente"`
	CreatedAt              string                `json:"data_criacao"`
}

type PagedSimulations struct {
	Items []Simulation `json:"itens"`
	Page  Page         `json:"pagina"`
}

// ------------------------------------------------------------------- propostas

type CreateProposalRequest struct {
	Cpf          string `json:"cpf"`
	SimulationID string `json:"simulacaoId"`
}

type CreateProposalResponse struct {
	ProposalID string `json:"id_proposta"`
}

type VehicleDetailsRequest struct {
	Renavam      string `json:"renavam"`
	Chassis      string `json:"chassi"`
	LicensePlate string `json:"placa"`
	Color        string `json:"cor"`
}

type DepositAccountRequest struct {
	HolderCpf     string `json:"cpfTitular"`
	BankCode      string `json:"codigoBanco"`
	Branch        string `json:"agencia"`
	BranchDigit   string `json:"digitoAgencia"`
	AccountNumber string `json:"conta"`
	AccountDigit  string `json:"digitoConta"`
	AccountType   string `json:"tipoConta"`
}

type DocumentUploadRequest struct {
	Participant   string `json:"participante"`
	DocumentType  int    `json:"tipoDocumento"`
	FileName      string `json:"nomeArquivo"`
	Base64Content string `json:"conteudoBase64"`
}

type ActiveStep struct {
	Step            *int   `json:"enum_etapa"`
	StepName        string `json:"etapa"`
	Situation       *int   `json:"enum_situacao"`
	SituationName   string `json:"situacao"`
	ActionOwner     *int   `json:"enum_responsavel_acao"`
	ActionOwnerName string `json:"responsavel_acao"`
}

type CompletedStep struct {
	Step     *int   `json:"enum_etapa"`
	StepName string `json:"etapa"`
	Date     string `json:"data"`
}

type Document struct {
	Type       *int   `json:"enum_tipo"`
	TypeName   string `json:"tipo"`
	Status     *int   `json:"enum_status"`
	StatusName string `json:"status"`
	Reason     string `json:"motivo,omitempty"`
}

// Participant é o comprador/vendedor da proposta v2 (identificado pelo cpf).
type Participant struct {
	Cpf            string          `json:"cpf"`
	Biometry       *int            `json:"enum_biometria"`
	BiometryName   string          `json:"biometria"`
	ActiveStep     *ActiveStep     `json:"status_ativo"`
	CompletedSteps []CompletedStep `json:"etapas_concluidas"`
	Documents      []Document      `json:"documentos"`
}

// Store é a loja de integração da proposta v2 (identificada pelo cnpj).
type Store struct {
	Cnpj           string          `json:"cnpj"`
	ActiveStep     *ActiveStep     `json:"status_ativo"`
	CompletedSteps []CompletedStep `json:"etapas_concluidas"`
	Documents      []Document      `json:"documentos"`
}

type ProposalV2 struct {
	ProposalID    string       `json:"id_proposta"`
	Number        int64        `json:"numero"`
	SimulationID  string       `json:"id_simulacao"`
	Status        *int         `json:"enum_status"`
	StatusName    string       `json:"status"`
	Type          string       `json:"tipo"`
	CreatedAt     string       `json:"data_criacao"`
	RefusalReason string       `json:"motivo_recusa,omitempty"`
	BiometryLink  string       `json:"link_biometria,omitempty"`
	Buyer         *Participant `json:"comprador"`
	Seller        *Participant `json:"vendedor"`
	Store         *Store       `json:"loja"`
}

type ProposalParty struct {
	Cpf string `json:"cpf"`
}

// ProposalV1 é o schema ProposalResponse (listagem e consulta v1).
type ProposalV1 struct {
	ProposalID   string         `json:"id_proposta"`
	Number       int64          `json:"numero"`
	SimulationID string         `json:"id_simulacao"`
	Type         string         `json:"tipo"`
	Status       string         `json:"status"`
	Buyer        *ProposalParty `json:"comprador"`
	Seller       *ProposalParty `json:"vendedor"`
	CreatedAt    string         `json:"data_criacao"`
}

type PagedProposals struct {
	Items []ProposalV1 `json:"itens"`
	Page  Page         `json:"pagina"`
}

// ------------------------------------------------------------------- webhooks

type CreateWebhookRequest struct {
	Type               int    `json:"tipo"`
	URL                string `json:"url"`
	AuthenticationType int    `json:"tipo_autenticacao"`
	Credential         string `json:"credencial,omitempty"`
}

type CreateWebhookResponse struct {
	WebhookID     string `json:"id_webhook"`
	SigningSecret string `json:"segredo_assinatura"`
}

type UpdateWebhookRequest struct {
	Type   int    `json:"tipo"`
	URL    string `json:"url"`
	Status *int   `json:"status,omitempty"`
}

type Webhook struct {
	WebhookID          string `json:"id_webhook"`
	Type               string `json:"tipo"`
	URL                string `json:"url"`
	AuthenticationType string `json:"tipo_autenticacao"`
	Status             string `json:"status"`
}

type PagedWebhooks struct {
	Items []Webhook `json:"itens"`
	Page  Page      `json:"pagina"`
}

// ProposalWebhookEvent é o payload de notificação de proposta. Aceita o formato v1
// (id_proposta, enum_status, status em inglês) e o v2 (retrato completo, nomes em
// português). Fora do contrato do openapi.yaml.
type ProposalWebhookEvent struct {
	ProposalID    string       `json:"id_proposta"`
	Status        *int         `json:"enum_status"`
	StatusName    string       `json:"status"`
	EventReason   string       `json:"motivo_evento"`
	RefusalReason string       `json:"motivo_recusa"`
	EventDate     string       `json:"data_evento"`
	Buyer         *Participant `json:"comprador"`
	Seller        *Participant `json:"vendedor"`
	Store         *Store       `json:"loja"`
}
