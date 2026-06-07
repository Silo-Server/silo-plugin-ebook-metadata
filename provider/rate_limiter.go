package provider

import (
	"context"
	"math"

	"golang.org/x/time/rate"
)

// newLimiter creates a token-bucket rate limiter allowing rpm requests per minute.
// A burst of 1 is used so requests are spaced evenly rather than batched.
func newLimiter(rpm float64) *rate.Limiter {
	if rpm <= 0 || math.IsNaN(rpm) || math.IsInf(rpm, 0) {
		return rate.NewLimiter(0, 0)
	}
	return rate.NewLimiter(rate.Limit(rpm/60.0), 1)
}

// waitForLimiter blocks until the limiter permits a request or ctx is cancelled.
func waitForLimiter(ctx context.Context, l *rate.Limiter) error {
	return l.Wait(ctx)
}
