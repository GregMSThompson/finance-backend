package clock

import (
	"context"
	"testing"
	"time"
)

func TestNowUsesContextClock(t *testing.T) {
	fixed := time.Date(2026, time.August, 14, 12, 0, 0, 0, time.UTC)
	ctx := WithClock(context.Background(), func() time.Time { return fixed })

	if got := Now(ctx); !got.Equal(fixed) {
		t.Fatalf("Now() = %s, want %s", got, fixed)
	}
}

func TestNowFallsBackToSystemClock(t *testing.T) {
	before := time.Now()
	got := Now(context.Background())
	after := time.Now()

	if got.Before(before) || got.After(after) {
		t.Fatalf("Now() = %s, want a time between %s and %s", got, before, after)
	}
}

func TestWithClockNilPreservesParent(t *testing.T) {
	parent := context.Background()
	if got := WithClock(parent, nil); got != parent {
		t.Fatal("WithClock(nil) did not preserve the parent context")
	}
}
