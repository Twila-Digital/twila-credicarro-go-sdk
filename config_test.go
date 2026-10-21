package credicarro_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	credicarro "github.com/Twila-Digital/twila-credicarro-go-sdk"
)

func TestNewClientRequiresClientID(t *testing.T) {
	_, err := credicarro.NewClient(credicarro.ClientOptions{ClientSecret: "secret"})
	assertConfigurationError(t, err)
}

func TestNewClientRequiresClientSecret(t *testing.T) {
	_, err := credicarro.NewClient(credicarro.ClientOptions{ClientID: "id"})
	assertConfigurationError(t, err)
}

func TestNewClientRejectsInvalidBaseURL(t *testing.T) {
	cases := map[string]string{
		"nao e url":          "not-a-url",
		"http remoto":        "http://api.credicarro.com.br/integration/",
		"http com host fake": "http://localhost.evil.com/",
		"esquema ftp":        "ftp://localhost/",
		"com usuario":        "https://user:pass@api.credicarro.com.br/integration/",
	}
	for name, baseURL := range cases {
		baseURL := baseURL
		t.Run(name, func(t *testing.T) {
			_, err := credicarro.NewClient(credicarro.ClientOptions{ClientID: "id", ClientSecret: "secret", BaseURL: baseURL})
			assertConfigurationError(t, err)
		})
	}
}

func TestNewClientAcceptsHTTPOnlyForLocalhost(t *testing.T) {
	for _, baseURL := range []string{"http://localhost:8080/integration", "http://127.0.0.1:5000/", "https://qualquer.exemplo.com.br/integration/"} {
		if _, err := credicarro.NewClient(credicarro.ClientOptions{ClientID: "id", ClientSecret: "secret", BaseURL: baseURL}); err != nil {
			t.Errorf("BaseURL %q deveria ser aceita: %v", baseURL, err)
		}
	}
}

func TestEnvironmentsResolveToDocumentedBaseURLs(t *testing.T) {
	cases := []struct {
		environment credicarro.Environment
		expected    string
	}{
		{"", "https://api.credicarro.com.br/integration/"}, // padrão: produção
		{credicarro.EnvironmentProduction, "https://api.credicarro.com.br/integration/"},
		{credicarro.EnvironmentStaging, "https://api.staging.credicarro.com.br/integration/"},
	}
	for _, c := range cases {
		got, err := credicarro.ResolvedBaseURL(credicarro.ClientOptions{ClientID: "id", ClientSecret: "secret", Environment: c.environment})
		if err != nil || got != c.expected {
			t.Errorf("Environment %q: esperava %q, veio %q (%v)", c.environment, c.expected, got, err)
		}
	}

	got, _ := credicarro.ResolvedBaseURL(credicarro.ClientOptions{ClientID: "id", ClientSecret: "secret", BaseURL: "https://exemplo.com.br/integration"})
	if got != "https://exemplo.com.br/integration/" {
		t.Errorf("BaseURL sem barra final deveria ganhar uma, veio %q", got)
	}

	_, err := credicarro.ResolvedBaseURL(credicarro.ClientOptions{ClientID: "id", ClientSecret: "secret", Environment: "dev"})
	assertConfigurationError(t, err)
}

func TestNewClientSucceedsWithDefaults(t *testing.T) {
	client, err := credicarro.NewClient(credicarro.ClientOptions{ClientID: "id", ClientSecret: "secret"})
	if err != nil {
		t.Fatalf("esperava sucesso, veio erro: %v", err)
	}
	if client.Customers == nil || client.Simulations == nil || client.Proposals == nil || client.Webhooks == nil ||
		client.V1 == nil || client.V1.Simulations == nil || client.V1.Proposals == nil {
		t.Fatal("todos os clients de recurso deveriam estar inicializados")
	}
}

func TestDefaultResilienceOptions(t *testing.T) {
	d := credicarro.DefaultResilienceOptions()
	if d.TotalTimeout != 30*time.Second || d.AttemptTimeout != 10*time.Second || d.DocumentUploadAttemptTimeout != 60*time.Second ||
		d.MaxRetryAttempts != 3 || d.RetryBaseDelay != 500*time.Millisecond ||
		d.CircuitBreakerFailureRatio != 0.5 || d.CircuitBreakerSamplingWindow != 30*time.Second ||
		d.CircuitBreakerMinThroughput != 10 || d.CircuitBreakerBreakDuration != 15*time.Second {
		t.Fatalf("padrões incorretos: %+v", d)
	}
	if !d.RetryOn500 {
		t.Error("RetryOn500 deveria ser true por padrão")
	}
	if d.DisableAutomaticIdempotencyKey {
		t.Error("a chave automática deveria vir habilitada por padrão")
	}
}

func TestNewClientRejectsInvalidResilienceOptions(t *testing.T) {
	cases := map[string]credicarro.ResilienceOptions{
		"zero retries":       {MaxRetryAttempts: 0, TotalTimeout: time.Second, AttemptTimeout: time.Second, CircuitBreakerFailureRatio: 0.5, CircuitBreakerMinThroughput: 2},
		"zero total timeout": {MaxRetryAttempts: 1, TotalTimeout: 0, AttemptTimeout: time.Second, CircuitBreakerFailureRatio: 0.5, CircuitBreakerMinThroughput: 2},
		"zero attempt":       {MaxRetryAttempts: 1, TotalTimeout: time.Second, AttemptTimeout: 0, CircuitBreakerFailureRatio: 0.5, CircuitBreakerMinThroughput: 2},
		"attempt > total":    {MaxRetryAttempts: 1, TotalTimeout: time.Second, AttemptTimeout: 2 * time.Second, CircuitBreakerFailureRatio: 0.5, CircuitBreakerMinThroughput: 2},
		"ratio zero":         {MaxRetryAttempts: 1, TotalTimeout: time.Second, AttemptTimeout: time.Second, CircuitBreakerFailureRatio: 0, CircuitBreakerMinThroughput: 2},
		"ratio above 1":      {MaxRetryAttempts: 1, TotalTimeout: time.Second, AttemptTimeout: time.Second, CircuitBreakerFailureRatio: 1.5, CircuitBreakerMinThroughput: 2},
		"min throughput < 2": {MaxRetryAttempts: 1, TotalTimeout: time.Second, AttemptTimeout: time.Second, CircuitBreakerFailureRatio: 0.5, CircuitBreakerMinThroughput: 1},
	}

	for name, resilience := range cases {
		resilience := resilience
		t.Run(name, func(t *testing.T) {
			_, err := credicarro.NewClient(credicarro.ClientOptions{
				ClientID:     "id",
				ClientSecret: "secret",
				Resilience:   &resilience,
			})
			assertConfigurationError(t, err)
		})
	}
}

func TestClientSecretNeverAppearsInFormattedOptions(t *testing.T) {
	opts := credicarro.ClientOptions{ClientID: "meu-id", ClientSecret: testSecret, Environment: credicarro.EnvironmentStaging}

	rendered := []string{
		opts.String(),
		fmt.Sprint(opts),
		fmt.Sprintf("%v", opts),
		fmt.Sprintf("%+v", opts),
		fmt.Sprintf("%#v", opts),
		fmt.Sprintf("%v", &opts),
	}
	for _, text := range rendered {
		if strings.Contains(text, testSecret) {
			t.Errorf("o ClientSecret vazou em %q", text)
		}
		if !strings.Contains(text, "meu-id") {
			t.Errorf("o ClientID deveria aparecer em %q", text)
		}
	}
}

func TestConfigurationErrorsNeverContainTheSecret(t *testing.T) {
	_, err := credicarro.NewClient(credicarro.ClientOptions{ClientID: "id", ClientSecret: testSecret, BaseURL: "http://remoto.exemplo.com/"})
	if err == nil || strings.Contains(err.Error(), testSecret) {
		t.Fatalf("erro inesperado: %v", err)
	}
}

func assertConfigurationError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("esperava erro, veio nil")
	}
	var configErr *credicarro.ConfigurationError
	if !errors.As(err, &configErr) {
		t.Fatalf("esperava *ConfigurationError, veio %T: %v", err, err)
	}
}
