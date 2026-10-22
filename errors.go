package credicarro

import (
	"fmt"
	"time"
)

// FieldError é um item da lista "erros" do ProblemDetails (JSON malformado ou valor de
// tipo errado; Code costuma ser "Validation.InvalidFormat").
type FieldError struct {
	Code    string
	Message string
	Field   string
}

// ConfigurationError indica configuração ou argumento inválido detectado no cliente,
// antes de qualquer chamada à API (ex.: ClientID ausente, ListRequest fora do intervalo,
// DocumentFile inválido).
type ConfigurationError struct {
	Message string
}

func (e *ConfigurationError) Error() string { return e.Message }

// AuthenticationError indica falha ao gerar o token de acesso (credenciais inválidas,
// integração inativa) ou um 401 que persiste depois de renovar o token.
//
// Message traz só o status e o código "tipo"; o detalhe da API fica em Detail.
type AuthenticationError struct {
	Message    string
	StatusCode int    // 0 quando a falha não veio de uma resposta HTTP
	ErrorCode  string // "tipo" do ProblemDetails, quando houver
	Title      string
	Detail     string // pode conter dados pessoais — não registre em log
	Field      string
	Errors     []FieldError
	Body       string // corpo bruto da resposta (pode conter dados pessoais)
	Cause      error
}

func (e *AuthenticationError) Error() string { return e.Message }
func (e *AuthenticationError) Unwrap() error { return e.Cause }

// TimeoutError indica timeout de tentativa, timeout total do pipeline de resiliência
// ou circuit breaker aberto.
type TimeoutError struct {
	Message string
	Cause   error
}

func (e *TimeoutError) Error() string { return e.Message }
func (e *TimeoutError) Unwrap() error { return e.Cause }

// WebhookSignatureError indica assinatura de webhook inválida, malformada, fora da
// janela de tolerância, ou corpo que não é um JSON de evento válido.
type WebhookSignatureError struct {
	Message string
}

func (e *WebhookSignatureError) Error() string { return e.Message }

// APIError é o erro de qualquer resposta de erro da API (404, 409, 5xx, etc.).
//
// A mensagem (Error) traz só o status, o código "tipo" e o "titulo". Detail e Body
// podem conter dados pessoais e ficam apenas nos campos — não os registre em log.
type APIError struct {
	StatusCode int
	// ErrorCode é o campo "tipo" (código estável, ex.: Store.Integration.NotFound).
	// Use-o no tratamento de erros.
	ErrorCode string
	Title     string
	Detail    string
	Field     string
	Errors    []FieldError
	// Body é o corpo bruto da resposta.
	Body string
}

func (e *APIError) Error() string {
	return formatStatusMessage("API do Credicarro retornou", e.StatusCode, e.ErrorCode, e.Title)
}

// ValidationError é um APIError de status 400.
type ValidationError struct {
	*APIError
}

// Unwrap permite errors.As(err, &apiErr) com *APIError.
func (e *ValidationError) Unwrap() error { return e.APIError }

// BusinessRuleError é um APIError de status 422 (regra de negócio não atendida);
// trate pelo ErrorCode.
type BusinessRuleError struct {
	*APIError
}

// Unwrap permite errors.As(err, &apiErr) com *APIError.
func (e *BusinessRuleError) Unwrap() error { return e.APIError }

// RateLimitError é um APIError de status 429.
type RateLimitError struct {
	*APIError
	// RetryAfter é o valor do cabeçalho Retry-After (zero quando ausente).
	RetryAfter time.Duration
}

// Unwrap permite errors.As(err, &apiErr) com *APIError.
func (e *RateLimitError) Unwrap() error { return e.APIError }

func formatStatusMessage(prefix string, status int, code, title string) string {
	msg := fmt.Sprintf("%s %d", prefix, status)
	switch {
	case code != "" && title != "":
		msg += fmt.Sprintf(": %s (%s)", code, title)
	case code != "":
		msg += ": " + code
	case title != "":
		msg += ": " + title
	}
	return msg + "."
}
