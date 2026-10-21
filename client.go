// Package credicarro é o SDK oficial em Go para a API de integração do Credicarro
// (financiamento e refinanciamento de veículos): clientes, simulações, propostas,
// documentos e webhooks.
package credicarro

import (
	"context"

	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/auth"
	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/httpclient"
	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/resilience"
	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/transport"
)

// Client é o cliente principal do SDK. Uma instância é segura para uso concorrente
// por múltiplas goroutines — reaproveite-a como singleton na aplicação (não crie uma
// por requisição); ela mantém o cache do token de acesso e o estado do circuit breaker.
type Client struct {
	Customers   *CustomersClient
	Simulations *SimulationsClient
	Proposals   *ProposalsClient
	Webhooks    *WebhooksClient
	// V1 expõe as rotas v1 sem cnpjLoja (simulações e propostas).
	V1 *V1Client
}

// NewClient valida options e constrói um Client pronto para uso.
func NewClient(options ClientOptions) (*Client, error) {
	resolved, err := resolveClientOptions(options)
	if err != nil {
		return nil, err
	}

	resilienceOpts := resilience.Options{
		TotalTimeout:                 resolved.resilience.TotalTimeout,
		MaxRetryAttempts:             resolved.resilience.MaxRetryAttempts,
		RetryBaseDelay:               resolved.resilience.RetryBaseDelay,
		CircuitBreakerFailureRatio:   resolved.resilience.CircuitBreakerFailureRatio,
		CircuitBreakerSamplingWindow: resolved.resilience.CircuitBreakerSamplingWindow,
		CircuitBreakerMinThroughput:  resolved.resilience.CircuitBreakerMinThroughput,
		CircuitBreakerBreakDuration:  resolved.resilience.CircuitBreakerBreakDuration,
		RetryOn500:                   resolved.resilience.RetryOn500,
	}

	httpClient := httpclient.Prepare(resolved.httpClient)

	tokenAPIClient := auth.NewTokenAPIClient(httpClient, resolved.baseURL, resolved.resilience.AttemptTimeout, resilienceOpts)
	tokenProvider := auth.NewTokenProvider(tokenAPIClient, resolved.clientID, resolved.clientSecret)

	executor := transport.NewExecutor(
		httpClient,
		resolved.baseURL,
		tokenProvider,
		resilienceOpts,
		resolved.resilience.AttemptTimeout,
		resolved.resilience.DisableAutomaticIdempotencyKey,
	)
	api := &apiCaller{executor: executor}

	return &Client{
		Customers:   &CustomersClient{api: api},
		Simulations: &SimulationsClient{api: api},
		Proposals:   &ProposalsClient{api: api, documentUploadAttemptTimeout: resolved.resilience.DocumentUploadAttemptTimeout},
		Webhooks:    &WebhooksClient{api: api},
		V1: &V1Client{
			Simulations: &V1SimulationsClient{api: api},
			Proposals:   &V1ProposalsClient{api: api},
		},
	}, nil
}

// apiCaller centraliza execução, checagem de status, decodificação e tradução de erros.
type apiCaller struct {
	executor *transport.Executor
}

func (c *apiCaller) finish(ctx context.Context, raw transport.RawResponse, err error, out interface{}) error {
	if err != nil {
		return translateExecutorError(ctx, err)
	}
	if apiErr := transport.EnsureSuccess(raw); apiErr != nil {
		return translateExecutorError(ctx, apiErr)
	}
	if out != nil {
		return unmarshalJSON(raw.Body, out)
	}
	return nil
}

func (c *apiCaller) get(ctx context.Context, path string, out interface{}) error {
	raw, err := c.executor.Get(ctx, path)
	return c.finish(ctx, raw, err, out)
}

func (c *apiCaller) post(ctx context.Context, path string, body, out interface{}, opts transport.RequestOptions) error {
	raw, err := c.executor.Post(ctx, path, body, opts)
	return c.finish(ctx, raw, err, out)
}

func (c *apiCaller) put(ctx context.Context, path string, body interface{}) error {
	raw, err := c.executor.Put(ctx, path, body)
	return c.finish(ctx, raw, err, nil)
}

func (c *apiCaller) delete(ctx context.Context, path string) error {
	raw, err := c.executor.Delete(ctx, path)
	return c.finish(ctx, raw, err, nil)
}
