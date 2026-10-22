package credicarro

import "time"

// ResilienceOptions controla retry, circuit breaker e timeouts. Os campos usam time.Duration
// (idiomático em Go), diferente dos SDKs de outras linguagens, que usam milissegundos.
//
// Ao montar o struct manualmente, parta de DefaultResilienceOptions() — o valor zero de
// RetryOn500 é false, enquanto o padrão do SDK é true.
type ResilienceOptions struct {
	// TotalTimeout é o orçamento total de uma chamada, incluindo as retentativas.
	TotalTimeout time.Duration
	// AttemptTimeout é o limite de cada tentativa.
	AttemptTimeout time.Duration
	// DocumentUploadAttemptTimeout é o limite de cada tentativa do envio de documento. O
	// orçamento total do envio é DocumentUploadAttemptTimeout × MaxRetryAttempts (padrão
	// 180 s); as demais chamadas usam TotalTimeout.
	DocumentUploadAttemptTimeout time.Duration
	// MaxRetryAttempts é o número máximo de tentativas (1 = sem retry).
	MaxRetryAttempts int
	// RetryBaseDelay é a base do backoff exponencial (com jitter).
	RetryBaseDelay time.Duration

	CircuitBreakerFailureRatio   float64
	CircuitBreakerSamplingWindow time.Duration
	CircuitBreakerMinThroughput  int
	CircuitBreakerBreakDuration  time.Duration

	// RetryOn500 repete respostas 500 em requisições seguras. A documentação orienta
	// repetir 500 com a mesma Idempotency-Key, por isso o padrão é true.
	RetryOn500 bool
	// DisableAutomaticIdempotencyKey desliga a Idempotency-Key automática. Sem a chave,
	// os POST deixam de ser repetidos. O envio de documento exige a chave e a gera sempre.
	DisableAutomaticIdempotencyKey bool
}

// DefaultResilienceOptions retorna os valores padrão do SDK.
func DefaultResilienceOptions() ResilienceOptions {
	return ResilienceOptions{
		TotalTimeout:                   30 * time.Second,
		AttemptTimeout:                 10 * time.Second,
		DocumentUploadAttemptTimeout:   60 * time.Second,
		MaxRetryAttempts:               3,
		RetryBaseDelay:                 500 * time.Millisecond,
		CircuitBreakerFailureRatio:     0.5,
		CircuitBreakerSamplingWindow:   30 * time.Second,
		CircuitBreakerMinThroughput:    10,
		CircuitBreakerBreakDuration:    15 * time.Second,
		RetryOn500:                     true,
		DisableAutomaticIdempotencyKey: false,
	}
}
