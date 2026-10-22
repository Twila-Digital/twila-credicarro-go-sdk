package resilience_test

import (
	"context"
	"testing"
	"time"

	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/resilience"
)

func TestNetworkBackoffIsBoundedByRemainingBudget(t *testing.T) {
	opts := fastOptions()
	opts.RetryBaseDelay = time.Minute
	opts.TotalTimeout = 300 * time.Millisecond
	pipeline := resilience.NewPipeline(opts)

	start := time.Now()
	_, err := pipeline.Execute(context.Background(), true, isNetworkError, func(ctx context.Context) (resilience.Response, error) {
		return resilience.Response{}, netError{}
	})
	if err == nil || time.Since(start) > 2*time.Second {
		t.Fatalf("esperava erro dentro do orçamento, veio %v em %v", err, time.Since(start))
	}
}
