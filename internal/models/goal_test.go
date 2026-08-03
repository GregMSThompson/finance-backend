package models

import (
	"testing"
	"time"
)

func date(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestResolveWindow_RecurringMonthly(t *testing.T) {
	g := &Goal{TimeWindow: GoalWindowMonthly, Recurrence: GoalRecurrenceRecurring}
	// now mid-month; a recurring goal tracks the live month.
	start, end, err := g.ResolveWindow(date("2026-08-15"))
	if err != nil {
		t.Fatalf("ResolveWindow error: %v", err)
	}
	if !start.Equal(date("2026-08-01")) || !end.Equal(date("2026-08-31")) {
		t.Fatalf("got [%s, %s], want [2026-08-01, 2026-08-31]", start.Format("2006-01-02"), end.Format("2006-01-02"))
	}
}

func TestResolveWindow_RecurringMonthlyLeapFebruary(t *testing.T) {
	g := &Goal{TimeWindow: GoalWindowMonthly, Recurrence: GoalRecurrenceRecurring}
	start, end, err := g.ResolveWindow(date("2024-02-10"))
	if err != nil {
		t.Fatalf("ResolveWindow error: %v", err)
	}
	if !start.Equal(date("2024-02-01")) || !end.Equal(date("2024-02-29")) {
		t.Fatalf("got [%s, %s], want [2024-02-01, 2024-02-29]", start.Format("2006-01-02"), end.Format("2006-01-02"))
	}
}

func TestResolveWindow_RecurringWeekly(t *testing.T) {
	g := &Goal{TimeWindow: GoalWindowWeekly, Recurrence: GoalRecurrenceRecurring}
	now := date("2026-08-12") // a Wednesday
	start, end, err := g.ResolveWindow(now)
	if err != nil {
		t.Fatalf("ResolveWindow error: %v", err)
	}
	if start.Weekday() != time.Monday {
		t.Fatalf("start %s is %s, want Monday", start.Format("2006-01-02"), start.Weekday())
	}
	if end.Weekday() != time.Sunday {
		t.Fatalf("end %s is %s, want Sunday", end.Format("2006-01-02"), end.Weekday())
	}
	if end.Sub(start) != 6*24*time.Hour {
		t.Fatalf("week span is %v, want 6 days", end.Sub(start))
	}
	if start.After(now) || now.After(end) {
		t.Fatalf("now %s not within [%s, %s]", now.Format("2006-01-02"), start.Format("2006-01-02"), end.Format("2006-01-02"))
	}
	// Concrete: the week of Wed 2026-08-12 runs Mon 2026-08-10 .. Sun 2026-08-16.
	if !start.Equal(date("2026-08-10")) || !end.Equal(date("2026-08-16")) {
		t.Fatalf("got [%s, %s], want [2026-08-10, 2026-08-16]", start.Format("2006-01-02"), end.Format("2006-01-02"))
	}
}

func TestResolveWindow_OneOffMonthlyPinnedToCreation(t *testing.T) {
	g := &Goal{
		TimeWindow: GoalWindowMonthly,
		Recurrence: GoalRecurrenceOneOff,
		CreatedAt:  date("2026-05-10"),
	}
	// now is three months later; a one-off stays pinned to its creation month.
	start, end, err := g.ResolveWindow(date("2026-08-15"))
	if err != nil {
		t.Fatalf("ResolveWindow error: %v", err)
	}
	if !start.Equal(date("2026-05-01")) || !end.Equal(date("2026-05-31")) {
		t.Fatalf("got [%s, %s], want [2026-05-01, 2026-05-31]", start.Format("2006-01-02"), end.Format("2006-01-02"))
	}
}

func TestResolveWindow_Fixed(t *testing.T) {
	g := &Goal{
		TimeWindow: GoalWindowFixed,
		Recurrence: GoalRecurrenceOneOff,
		CreatedAt:  date("2026-06-10"),
		EndDate:    "2026-12-31",
	}
	start, end, err := g.ResolveWindow(date("2026-08-15"))
	if err != nil {
		t.Fatalf("ResolveWindow error: %v", err)
	}
	if !start.Equal(date("2026-06-10")) || !end.Equal(date("2026-12-31")) {
		t.Fatalf("got [%s, %s], want [2026-06-10, 2026-12-31]", start.Format("2006-01-02"), end.Format("2006-01-02"))
	}
}

func TestResolveWindow_FixedInvalidEndDate(t *testing.T) {
	g := &Goal{TimeWindow: GoalWindowFixed, EndDate: "not-a-date"}
	if _, _, err := g.ResolveWindow(date("2026-08-15")); err == nil {
		t.Fatal("expected error for invalid endDate")
	}
}

func TestResolveWindow_UnknownWindow(t *testing.T) {
	g := &Goal{TimeWindow: "quarterly"}
	if _, _, err := g.ResolveWindow(date("2026-08-15")); err == nil {
		t.Fatal("expected error for unknown timeWindow")
	}
}
