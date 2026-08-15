// Package clock provides an operation-scoped source of current time.
package clock

import (
	"context"
	"time"
)

type contextKey struct{}

// WithClock returns a context whose callers use now as their current time.
// Passing nil preserves the parent clock.
func WithClock(ctx context.Context, now func() time.Time) context.Context {
	if now == nil {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, now)
}

// Now returns the current time from the context's clock, or the system clock
// when no clock has been attached.
func Now(ctx context.Context) time.Time {
	now, ok := ctx.Value(contextKey{}).(func() time.Time)
	if ok && now != nil {
		return now()
	}
	return time.Now()
}
