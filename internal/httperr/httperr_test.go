package httperr_test

import (
	"testing"
	"time"

	"github.com/Twila-Digital/twila-credicarro-go-sdk/internal/httperr"
)

func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		value string
		want  time.Duration
	}{
		{"", 0}, {"abc", 0}, {"0", 0}, {"-3", 0}, {"NaN", 0}, {"nan", 0},
		{"7", 7 * time.Second}, {"1.5", 1500 * time.Millisecond},
		{"Inf", httperr.MaxRetryAfter}, {"+Inf", httperr.MaxRetryAfter}, {"-Inf", 0},
		{"1e300", httperr.MaxRetryAfter}, {"99999999999999999999", httperr.MaxRetryAfter},
		{"Mon, 19 Oct 2026 12:00:30 GMT", 30 * time.Second},
		{"Mon, 19 Oct 2026 11:00:00 GMT", 0},
		{"Mon, 19 Oct 2099 12:00:00 GMT", httperr.MaxRetryAfter},
	}
	for _, c := range cases {
		if got := httperr.ParseRetryAfter(c.value, now); got != c.want {
			t.Errorf("ParseRetryAfter(%q) = %v, esperava %v", c.value, got, c.want)
		}
	}
}
