// Package resilience implementa retry com backoff+jitter e circuit breaker por
// janela deslizante, do zero (sem lib de terceiros) — mesmo desenho usado nos SDKs
// Python e PHP desta família, adaptado pra concorrência real via goroutines.
package resilience

import (
	"context"
	"errors"
	"math/rand"
	"sync"
	"time"
)

// Options espelha os campos relevantes de ResilienceOptions do pacote raiz — definido
// aqui (não importado do pacote raiz) pra evitar import cycle, já que o pacote raiz
// importa este pacote pra construir o Pipeline.
type Options struct {
	TotalTimeout                 time.Duration
	MaxRetryAttempts             int
	RetryBaseDelay               time.Duration
	CircuitBreakerFailureRatio   float64
	CircuitBreakerSamplingWindow time.Duration
	CircuitBreakerMinThroughput  int
	CircuitBreakerBreakDuration  time.Duration
	RetryOn500                   bool
}

// ErrBrokenCircuit é retornado (encapsulado) quando o circuit breaker está aberto.
var ErrBrokenCircuit = errors.New("circuit breaker aberto")

// Response é o mínimo que o Pipeline precisa saber sobre uma resposta HTTP pra
// decidir se é transiente — desacoplado de net/http pra manter este pacote focado.
type Response struct {
	StatusCode int
	RetryAfter time.Duration // zero se o header não existir/for inválido
}

var transientStatusCodes = map[int]bool{408: true, 429: true, 502: true, 503: true, 504: true}

// IsTransientResponse decide se uma resposta HTTP deve ser tratada como falha transiente.
func IsTransientResponse(resp Response, opts Options) bool {
	if transientStatusCodes[resp.StatusCode] {
		return true
	}
	return resp.StatusCode == 500 && opts.RetryOn500
}

type event struct {
	at      time.Time
	failure bool
}

type circuitBreaker struct {
	mu                    sync.Mutex
	opts                  Options
	events                []event
	openedAt              time.Time
	halfOpenTrialInFlight bool
}

func newCircuitBreaker(opts Options) *circuitBreaker {
	return &circuitBreaker{opts: opts}
}

func (b *circuitBreaker) beforeCall() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.openedAt.IsZero() {
		return nil
	}

	if time.Since(b.openedAt) < b.opts.CircuitBreakerBreakDuration {
		return ErrBrokenCircuit
	}
	if b.halfOpenTrialInFlight {
		return ErrBrokenCircuit
	}
	b.halfOpenTrialInFlight = true
	return nil
}

func (b *circuitBreaker) onSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.openedAt.IsZero() {
		b.openedAt = time.Time{}
		b.halfOpenTrialInFlight = false
		b.events = nil
		return
	}
	b.record(false)
}

func (b *circuitBreaker) onFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if !b.openedAt.IsZero() {
		b.openedAt = time.Now()
		b.halfOpenTrialInFlight = false
		return
	}
	b.record(true)
	b.maybeOpen()
}

// onAbort libera a tentativa de teste do estado half-open quando a chamada foi
// interrompida pelo chamador (sem sucesso nem falha do serviço).
func (b *circuitBreaker) onAbort() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.halfOpenTrialInFlight = false
}

func (b *circuitBreaker) record(failure bool) {
	now := time.Now()
	b.events = append(b.events, event{at: now, failure: failure})

	cutoff := now.Add(-b.opts.CircuitBreakerSamplingWindow)
	i := 0
	for i < len(b.events) && b.events[i].at.Before(cutoff) {
		i++
	}
	b.events = b.events[i:]
}

func (b *circuitBreaker) maybeOpen() {
	total := len(b.events)
	if total < b.opts.CircuitBreakerMinThroughput {
		return
	}

	failures := 0
	for _, e := range b.events {
		if e.failure {
			failures++
		}
	}

	if float64(failures)/float64(total) >= b.opts.CircuitBreakerFailureRatio {
		b.openedAt = time.Now()
		b.events = nil
	}
}

// Pipeline aplica retry+circuit breaker+timeout total em torno de uma função de tentativa.
// Uma única instância deve ser compartilhada entre chamadas concorrentes (goroutine-safe).
type Pipeline struct {
	opts    Options
	breaker *circuitBreaker
}

func NewPipeline(opts Options) *Pipeline {
	return &Pipeline{opts: opts, breaker: newCircuitBreaker(opts)}
}

// Attempt executa uma tentativa de chamada. IsNetworkError classifica erros retornados
// como falha de rede (retryable) — qualquer outro erro é propagado sem retry.
type Attempt func(ctx context.Context) (Response, error)

// Execute roda attempt com retry (se retrySafe) e sob o circuit breaker compartilhado,
// limitado pelo TotalTimeout configurado.
func (p *Pipeline) Execute(ctx context.Context, retrySafe bool, isNetworkError func(error) bool, attempt Attempt) (Response, error) {
	return p.ExecuteWithTotal(ctx, p.opts.TotalTimeout, retrySafe, isNetworkError, attempt)
}

// ExecuteWithTotal é como Execute, mas com um timeout total específico da chamada
// (usado no envio de documentos, cuja tentativa pode durar mais que o TotalTimeout padrão).
func (p *Pipeline) ExecuteWithTotal(ctx context.Context, total time.Duration, retrySafe bool, isNetworkError func(error) bool, attempt Attempt) (Response, error) {
	deadline := time.Now().Add(total)
	maxAttempts := p.opts.MaxRetryAttempts
	if maxAttempts < 1 {
		maxAttempts = 1
	}

	var lastErr error

	for attemptNumber := 1; attemptNumber <= maxAttempts; attemptNumber++ {
		if time.Now().After(deadline) {
			return Response{}, &TimeoutExceededError{Cause: lastErr}
		}

		if err := p.breaker.beforeCall(); err != nil {
			return Response{}, &TimeoutExceededError{Cause: err, BrokenCircuit: true}
		}

		resp, err := p.runAttempt(ctx, deadline, attempt)

		if err != nil {
			// Cancelamento/expiração do contexto do chamador, ou um erro que não é de
			// rede, não é falha do serviço: não conta pro circuit breaker nem é repetido.
			if ctx.Err() != nil || !isNetworkError(err) {
				p.breaker.onAbort()
				return Response{}, err
			}

			p.breaker.onFailure()
			lastErr = err

			if retrySafe && attemptNumber < maxAttempts && time.Now().Before(deadline) {
				delay := p.backoff(attemptNumber)
				if remaining := time.Until(deadline); delay > remaining {
					delay = remaining
				}
				sleep(ctx, delay)
				if ctx.Err() != nil {
					return Response{}, ctx.Err()
				}
				continue
			}
			return Response{}, err
		}

		transient := IsTransientResponse(resp, p.opts)
		if transient {
			p.breaker.onFailure()
		} else {
			p.breaker.onSuccess()
		}

		if !transient || !retrySafe {
			return resp, nil
		}
		if attemptNumber >= maxAttempts {
			return resp, nil
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return resp, nil
		}

		// Retry-After maior que o orçamento restante: não espera, devolve já a resposta
		// (o chamador recebe o 429/503 com o RetryAfter).
		if resp.RetryAfter > remaining {
			return resp, nil
		}

		delay := p.delay(attemptNumber, resp)
		if delay > remaining {
			delay = remaining
		}
		sleep(ctx, delay)
		if ctx.Err() != nil {
			return Response{}, ctx.Err()
		}
	}

	// inatingível: o loop sempre retorna ou dá continue até esgotar maxAttempts.
	return Response{}, lastErr
}

// runAttempt executa uma tentativa; se ela terminar sem retornar (panic), libera a
// tentativa de teste do estado half-open para o circuit breaker nunca ficar preso.
func (p *Pipeline) runAttempt(ctx context.Context, deadline time.Time, attempt Attempt) (Response, error) {
	attemptCtx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()

	completed := false
	defer func() {
		if !completed {
			p.breaker.onAbort()
		}
	}()

	resp, err := attempt(attemptCtx)
	completed = true
	return resp, err
}

func (p *Pipeline) delay(attemptNumber int, resp Response) time.Duration {
	if resp.RetryAfter > 0 {
		return resp.RetryAfter
	}
	return p.backoff(attemptNumber)
}

func (p *Pipeline) backoff(attemptNumber int) time.Duration {
	exponent := attemptNumber - 1
	if exponent < 0 {
		exponent = 0
	}
	base := float64(p.opts.RetryBaseDelay)
	exponential := base * float64(int64(1)<<uint(exponent))
	jitter := 0.5 + rand.Float64()
	return time.Duration(exponential * jitter)
}

func sleep(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
}

// TimeoutExceededError é retornado quando o timeout total do pipeline estoura ou o
// circuit breaker está aberto — o pacote raiz mapeia isso pra credicarro.TimeoutError.
type TimeoutExceededError struct {
	Cause         error
	BrokenCircuit bool
}

func (e *TimeoutExceededError) Error() string {
	if e.BrokenCircuit {
		return "o circuit breaker está aberto — chamadas recentes falharam de forma consistente"
	}
	return "a requisição excedeu o tempo limite configurado"
}

func (e *TimeoutExceededError) Unwrap() error { return e.Cause }
