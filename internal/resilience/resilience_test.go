package resilience_test

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/resilience"
)

func fastOptions() resilience.Options {
	return resilience.Options{
		TotalTimeout:                 5 * time.Second,
		MaxRetryAttempts:             3,
		RetryBaseDelay:               time.Millisecond,
		CircuitBreakerFailureRatio:   0.5,
		CircuitBreakerSamplingWindow: 30 * time.Second,
		CircuitBreakerMinThroughput:  10,
		CircuitBreakerBreakDuration:  60 * time.Second,
	}
}

type netError struct{}

func (netError) Error() string   { return "network error" }
func (netError) Timeout() bool   { return true }
func (netError) Temporary() bool { return true }

var _ net.Error = netError{}

func isNetworkError(err error) bool {
	var ne net.Error
	return errors.As(err, &ne)
}

func TestSuccessfulResponseDoesNotRetry(t *testing.T) {
	pipeline := resilience.NewPipeline(fastOptions())
	calls := 0

	resp, err := pipeline.Execute(context.Background(), true, isNetworkError, func(ctx context.Context) (resilience.Response, error) {
		calls++
		return resilience.Response{StatusCode: 200}, nil
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if resp.StatusCode != 200 || calls != 1 {
		t.Fatalf("esperava 1 chamada e status 200, veio %d chamadas e status %d", calls, resp.StatusCode)
	}
}

func TestTransientResponseRetriesWhenRetrySafe(t *testing.T) {
	pipeline := resilience.NewPipeline(fastOptions())
	calls := 0

	resp, err := pipeline.Execute(context.Background(), true, isNetworkError, func(ctx context.Context) (resilience.Response, error) {
		calls++
		if calls < 3 {
			return resilience.Response{StatusCode: 503}, nil
		}
		return resilience.Response{StatusCode: 200}, nil
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if resp.StatusCode != 200 || calls != 3 {
		t.Fatalf("esperava 3 chamadas e status 200, veio %d chamadas e status %d", calls, resp.StatusCode)
	}
}

func TestTransientResponseDoesNotRetryWhenNotRetrySafe(t *testing.T) {
	pipeline := resilience.NewPipeline(fastOptions())
	calls := 0

	resp, err := pipeline.Execute(context.Background(), false, isNetworkError, func(ctx context.Context) (resilience.Response, error) {
		calls++
		return resilience.Response{StatusCode: 503}, nil
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if resp.StatusCode != 503 || calls != 1 {
		t.Fatalf("esperava 1 chamada e status 503, veio %d chamadas e status %d", calls, resp.StatusCode)
	}
}

func TestNetworkErrorRetriesAndEventuallyReturnsError(t *testing.T) {
	pipeline := resilience.NewPipeline(fastOptions())
	calls := 0

	_, err := pipeline.Execute(context.Background(), true, isNetworkError, func(ctx context.Context) (resilience.Response, error) {
		calls++
		return resilience.Response{}, netError{}
	})
	if err == nil {
		t.Fatal("esperava erro")
	}
	if calls != 3 {
		t.Fatalf("esperava 3 chamadas, veio %d", calls)
	}
}

func TestNetworkErrorIsNotRetriedWhenNotRetrySafe(t *testing.T) {
	pipeline := resilience.NewPipeline(fastOptions())
	calls := 0

	_, err := pipeline.Execute(context.Background(), false, isNetworkError, func(ctx context.Context) (resilience.Response, error) {
		calls++
		return resilience.Response{}, netError{}
	})
	if err == nil || calls != 1 {
		t.Fatalf("esperava 1 chamada com erro, veio %d chamadas e erro %v", calls, err)
	}
}

func TestStatus500OnlyRetriesWhenEnabled(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		opts := fastOptions()
		opts.RetryOn500 = enabled
		pipeline := resilience.NewPipeline(opts)
		calls := 0

		_, _ = pipeline.Execute(context.Background(), true, isNetworkError, func(ctx context.Context) (resilience.Response, error) {
			calls++
			return resilience.Response{StatusCode: 500}, nil
		})

		expected := 1
		if enabled {
			expected = 3
		}
		if calls != expected {
			t.Fatalf("RetryOn500=%v: esperava %d chamadas, veio %d", enabled, expected, calls)
		}
	}
}

func TestTransientStatusSet(t *testing.T) {
	opts := fastOptions()
	for _, status := range []int{408, 429, 502, 503, 504} {
		if !resilience.IsTransientResponse(resilience.Response{StatusCode: status}, opts) {
			t.Errorf("status %d deveria ser transitório", status)
		}
	}
	for _, status := range []int{200, 201, 400, 401, 403, 404, 409, 422, 500} {
		if resilience.IsTransientResponse(resilience.Response{StatusCode: status}, opts) {
			t.Errorf("status %d não deveria ser transitório (RetryOn500 desligado)", status)
		}
	}
}

func TestRetryAfterIsRespected(t *testing.T) {
	pipeline := resilience.NewPipeline(fastOptions())
	calls := 0
	var first time.Time
	var gap time.Duration

	_, err := pipeline.Execute(context.Background(), true, isNetworkError, func(ctx context.Context) (resilience.Response, error) {
		calls++
		if calls == 1 {
			first = time.Now()
			return resilience.Response{StatusCode: 429, RetryAfter: 120 * time.Millisecond}, nil
		}
		gap = time.Since(first)
		return resilience.Response{StatusCode: 200}, nil
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if calls != 2 || gap < 100*time.Millisecond {
		t.Fatalf("esperava 2 chamadas com espera >= 100ms (Retry-After), veio %d chamadas e %v", calls, gap)
	}
}

func TestTotalTimeoutReturnsTimeoutExceededError(t *testing.T) {
	opts := resilience.Options{
		TotalTimeout:                 1 * time.Millisecond,
		MaxRetryAttempts:             5,
		RetryBaseDelay:               50 * time.Millisecond,
		CircuitBreakerFailureRatio:   0.5,
		CircuitBreakerSamplingWindow: 30 * time.Second,
		CircuitBreakerMinThroughput:  10,
		CircuitBreakerBreakDuration:  60 * time.Second,
	}
	pipeline := resilience.NewPipeline(opts)

	_, err := pipeline.Execute(context.Background(), true, isNetworkError, func(ctx context.Context) (resilience.Response, error) {
		return resilience.Response{StatusCode: 503}, nil
	})

	var timeoutErr *resilience.TimeoutExceededError
	if !errors.As(err, &timeoutErr) {
		t.Fatalf("esperava *TimeoutExceededError, veio %T: %v", err, err)
	}
}

func TestExecuteWithTotalOverridesConfiguredTimeout(t *testing.T) {
	opts := fastOptions()
	opts.TotalTimeout = time.Millisecond
	pipeline := resilience.NewPipeline(opts)

	resp, err := pipeline.ExecuteWithTotal(context.Background(), 5*time.Second, true, isNetworkError, func(ctx context.Context) (resilience.Response, error) {
		time.Sleep(20 * time.Millisecond)
		return resilience.Response{StatusCode: 200}, nil
	})
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("esperava sucesso com o orçamento sobrescrito, veio %v (%d)", err, resp.StatusCode)
	}
}

func TestCircuitBreakerOpensAfterFailureThresholdAndBlocksNextCall(t *testing.T) {
	opts := resilience.Options{
		TotalTimeout:                 5 * time.Second,
		MaxRetryAttempts:             1,
		RetryBaseDelay:               time.Millisecond,
		CircuitBreakerFailureRatio:   0.5,
		CircuitBreakerSamplingWindow: 30 * time.Second,
		CircuitBreakerMinThroughput:  2,
		CircuitBreakerBreakDuration:  60 * time.Second,
	}
	pipeline := resilience.NewPipeline(opts)

	for i := 0; i < 2; i++ {
		_, _ = pipeline.Execute(context.Background(), false, isNetworkError, func(ctx context.Context) (resilience.Response, error) {
			return resilience.Response{StatusCode: 503}, nil
		})
	}

	_, err := pipeline.Execute(context.Background(), false, isNetworkError, func(ctx context.Context) (resilience.Response, error) {
		return resilience.Response{StatusCode: 200}, nil
	})

	var timeoutErr *resilience.TimeoutExceededError
	if !errors.As(err, &timeoutErr) || !timeoutErr.BrokenCircuit {
		t.Fatalf("esperava *TimeoutExceededError com BrokenCircuit, veio %T: %v", err, err)
	}
}

func TestCircuitBreakerHalfOpenTrialRecoversAfterSuccess(t *testing.T) {
	opts := resilience.Options{
		TotalTimeout:                 5 * time.Second,
		MaxRetryAttempts:             1,
		RetryBaseDelay:               time.Millisecond,
		CircuitBreakerFailureRatio:   0.5,
		CircuitBreakerSamplingWindow: 30 * time.Second,
		CircuitBreakerMinThroughput:  2,
		CircuitBreakerBreakDuration:  30 * time.Millisecond,
	}
	pipeline := resilience.NewPipeline(opts)

	for i := 0; i < 2; i++ {
		_, _ = pipeline.Execute(context.Background(), false, isNetworkError, func(ctx context.Context) (resilience.Response, error) {
			return resilience.Response{StatusCode: 503}, nil
		})
	}
	time.Sleep(50 * time.Millisecond)

	resp, err := pipeline.Execute(context.Background(), false, isNetworkError, func(ctx context.Context) (resilience.Response, error) {
		return resilience.Response{StatusCode: 200}, nil
	})
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("a tentativa de teste do half-open deveria passar, veio %v (%d)", err, resp.StatusCode)
	}
}

func TestCallerCancellationDoesNotCountAsFailureNorRetries(t *testing.T) {
	opts := fastOptions()
	opts.CircuitBreakerMinThroughput = 2
	pipeline := resilience.NewPipeline(opts)

	for i := 0; i < 5; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		calls := 0
		_, err := pipeline.Execute(ctx, true, isNetworkError, func(ctx context.Context) (resilience.Response, error) {
			calls++
			return resilience.Response{}, ctx.Err()
		})
		if !errors.Is(err, context.Canceled) || calls != 1 {
			t.Fatalf("esperava 1 chamada e context.Canceled, veio %d chamadas e %v", calls, err)
		}
	}

	// o circuit breaker não pode ter aberto por causa dos cancelamentos
	resp, err := pipeline.Execute(context.Background(), true, isNetworkError, func(ctx context.Context) (resilience.Response, error) {
		return resilience.Response{StatusCode: 200}, nil
	})
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("o circuit breaker não deveria abrir por cancelamento do chamador: %v", err)
	}
}

func TestCancellationDuringBackoffStopsRetrying(t *testing.T) {
	opts := fastOptions()
	opts.RetryBaseDelay = 5 * time.Second
	pipeline := resilience.NewPipeline(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	calls := 0
	start := time.Now()
	_, err := pipeline.Execute(ctx, true, isNetworkError, func(ctx context.Context) (resilience.Response, error) {
		calls++
		return resilience.Response{StatusCode: 503}, nil
	})
	if !errors.Is(err, context.DeadlineExceeded) || calls != 1 {
		t.Fatalf("esperava 1 chamada e DeadlineExceeded, veio %d chamadas e %v", calls, err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("o backoff deveria ter sido interrompido pelo contexto")
	}
}

func TestRetryAfterBeyondRemainingBudgetReturnsImmediately(t *testing.T) {
	opts := fastOptions()
	opts.TotalTimeout = 2 * time.Second
	pipeline := resilience.NewPipeline(opts)
	calls := 0

	start := time.Now()
	resp, err := pipeline.Execute(context.Background(), true, isNetworkError, func(ctx context.Context) (resilience.Response, error) {
		calls++
		return resilience.Response{StatusCode: 429, RetryAfter: time.Hour}, nil
	})
	if err != nil || resp.StatusCode != 429 || resp.RetryAfter != time.Hour || calls != 1 {
		t.Fatalf("esperava devolver o 429 já: %v %+v calls=%d", err, resp, calls)
	}
	if time.Since(start) > time.Second {
		t.Fatal("não deveria esperar")
	}
}

func TestPanicInHalfOpenTrialReleasesTheCircuitBreaker(t *testing.T) {
	opts := resilience.Options{
		TotalTimeout:                 5 * time.Second,
		MaxRetryAttempts:             1,
		RetryBaseDelay:               time.Millisecond,
		CircuitBreakerFailureRatio:   0.5,
		CircuitBreakerSamplingWindow: 30 * time.Second,
		CircuitBreakerMinThroughput:  2,
		CircuitBreakerBreakDuration:  20 * time.Millisecond,
	}
	pipeline := resilience.NewPipeline(opts)
	for i := 0; i < 2; i++ {
		_, _ = pipeline.Execute(context.Background(), false, isNetworkError, func(ctx context.Context) (resilience.Response, error) {
			return resilience.Response{StatusCode: 503}, nil
		})
	}
	time.Sleep(40 * time.Millisecond)

	func() {
		defer func() { _ = recover() }()
		_, _ = pipeline.Execute(context.Background(), false, isNetworkError, func(ctx context.Context) (resilience.Response, error) {
			panic("boom")
		})
	}()

	resp, err := pipeline.Execute(context.Background(), false, isNetworkError, func(ctx context.Context) (resilience.Response, error) {
		return resilience.Response{StatusCode: 200}, nil
	})
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("a meia-abertura deveria ter sido liberada após o panic: %v", err)
	}
}

func TestNonNetworkErrorInHalfOpenTrialReleasesTheCircuitBreaker(t *testing.T) {
	opts := fastOptions()
	opts.MaxRetryAttempts = 1
	opts.CircuitBreakerMinThroughput = 2
	opts.CircuitBreakerBreakDuration = 20 * time.Millisecond
	pipeline := resilience.NewPipeline(opts)
	for i := 0; i < 2; i++ {
		_, _ = pipeline.Execute(context.Background(), false, isNetworkError, func(ctx context.Context) (resilience.Response, error) {
			return resilience.Response{StatusCode: 503}, nil
		})
	}
	time.Sleep(40 * time.Millisecond)

	_, err := pipeline.Execute(context.Background(), false, isNetworkError, func(ctx context.Context) (resilience.Response, error) {
		return resilience.Response{}, errors.New("erro qualquer")
	})
	if err == nil {
		t.Fatal("esperava o erro")
	}
	if _, err := pipeline.Execute(context.Background(), false, isNetworkError, func(ctx context.Context) (resilience.Response, error) {
		return resilience.Response{StatusCode: 200}, nil
	}); err != nil {
		t.Fatalf("o breaker não deveria ficar preso: %v", err)
	}
}
