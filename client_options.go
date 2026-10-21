package credicarro

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ClientOptions configura o Client.
//
// O ClientSecret nunca aparece em String(), %v, %+v ou %#v.
type ClientOptions struct {
	ClientID     string
	ClientSecret string
	Environment  Environment        // padrão EnvironmentProduction; ignorado se BaseURL for informado
	BaseURL      string             // opcional — sobrescreve Environment; precisa ser https (exceto localhost/127.0.0.1)
	Resilience   *ResilienceOptions // opcional — nil usa DefaultResilienceOptions()
	// HTTPClient é opcional. Quando nil, o SDK usa um cliente próprio (transporte dedicado,
	// TLS 1.2 no mínimo, até 64 conexões ociosas por host). Quando informado, o SDK usa uma
	// cópia rasa dele — o Transport e o Timeout do integrador são mantidos, mas os redirects
	// ficam SEMPRE desligados (CheckRedirect é sobrescrito), para o token Bearer nunca seguir
	// um 3xx para outro destino; um 3xx vira *APIError. Se definir Timeout no cliente, ele
	// vale por requisição e deve ser maior que Resilience.AttemptTimeout (o do SDK).
	HTTPClient *http.Client
}

// String devolve uma representação sem o ClientSecret.
func (o ClientOptions) String() string {
	return fmt.Sprintf("ClientOptions{ClientID: %q, ClientSecret: [REDACTED], Environment: %q, BaseURL: %q}", o.ClientID, o.Environment, o.BaseURL)
}

// MarshalJSON recusa a serialização: as opções do cliente guardam o ClientSecret e nunca
// devem ser persistidas nem enviadas por JSON.
func (o ClientOptions) MarshalJSON() ([]byte, error) {
	return nil, errors.New("credicarro: ClientOptions contém o ClientSecret e não pode ser serializado")
}

// GoString devolve uma representação sem o ClientSecret (verbo %#v).
func (o ClientOptions) GoString() string { return o.String() }

type resolvedClientOptions struct {
	httpClient   *http.Client
	clientID     string
	clientSecret string
	baseURL      string
	resilience   ResilienceOptions
}

func resolveClientOptions(opts ClientOptions) (*resolvedClientOptions, error) {
	if strings.TrimSpace(opts.ClientID) == "" {
		return nil, &ConfigurationError{Message: "ClientID é obrigatório."}
	}
	if strings.TrimSpace(opts.ClientSecret) == "" {
		return nil, &ConfigurationError{Message: "ClientSecret é obrigatório."}
	}

	baseURL := opts.BaseURL
	if baseURL == "" {
		env := opts.Environment
		if env == "" {
			env = EnvironmentProduction
		}
		mapped, ok := environmentBaseURLs[env]
		if !ok {
			return nil, &ConfigurationError{Message: "Environment inválido."}
		}
		baseURL = mapped
	} else {
		parsed, err := url.Parse(baseURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return nil, &ConfigurationError{Message: "BaseURL, quando informada, deve ser uma URL absoluta válida."}
		}
		if parsed.User != nil {
			return nil, &ConfigurationError{Message: "BaseURL não pode conter usuário e senha."}
		}
		if !isAllowedScheme(parsed) {
			return nil, &ConfigurationError{Message: "BaseURL deve usar https (http só é aceito para localhost e 127.0.0.1)."}
		}
	}
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	resilience := DefaultResilienceOptions()
	if opts.Resilience != nil {
		resilience = *opts.Resilience
	}

	if err := validateResilience(resilience); err != nil {
		return nil, err
	}

	return &resolvedClientOptions{
		httpClient:   opts.HTTPClient,
		clientID:     opts.ClientID,
		clientSecret: opts.ClientSecret,
		baseURL:      baseURL,
		resilience:   resilience,
	}, nil
}

func isAllowedScheme(u *url.URL) bool {
	switch strings.ToLower(u.Scheme) {
	case "https":
		return true
	case "http":
		host := strings.ToLower(u.Hostname())
		return host == "localhost" || host == "127.0.0.1"
	default:
		return false
	}
}

func validateResilience(r ResilienceOptions) error {
	if r.MaxRetryAttempts < 1 {
		return &ConfigurationError{Message: "Resilience.MaxRetryAttempts deve ser maior ou igual a 1."}
	}
	if r.TotalTimeout <= 0 {
		return &ConfigurationError{Message: "Resilience.TotalTimeout deve ser maior que zero."}
	}
	if r.AttemptTimeout <= 0 {
		return &ConfigurationError{Message: "Resilience.AttemptTimeout deve ser maior que zero."}
	}
	if r.AttemptTimeout > r.TotalTimeout {
		return &ConfigurationError{Message: "Resilience.AttemptTimeout não pode ser maior que Resilience.TotalTimeout."}
	}
	if r.DocumentUploadAttemptTimeout <= 0 {
		return &ConfigurationError{Message: "Resilience.DocumentUploadAttemptTimeout deve ser maior que zero."}
	}
	if r.RetryBaseDelay <= 0 {
		return &ConfigurationError{Message: "Resilience.RetryBaseDelay deve ser maior que zero."}
	}
	if r.CircuitBreakerSamplingWindow <= 0 {
		return &ConfigurationError{Message: "Resilience.CircuitBreakerSamplingWindow deve ser maior que zero."}
	}
	if r.CircuitBreakerBreakDuration <= 0 {
		return &ConfigurationError{Message: "Resilience.CircuitBreakerBreakDuration deve ser maior que zero."}
	}
	if r.CircuitBreakerFailureRatio <= 0 || r.CircuitBreakerFailureRatio > 1 {
		return &ConfigurationError{Message: "Resilience.CircuitBreakerFailureRatio deve estar entre 0 (exclusivo) e 1 (inclusivo)."}
	}
	if r.CircuitBreakerMinThroughput < 2 {
		return &ConfigurationError{Message: "Resilience.CircuitBreakerMinThroughput deve ser maior ou igual a 2."}
	}
	return nil
}
