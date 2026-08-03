package helpers

import "time"

// DateLayout is the canonical YYYY-MM-DD date format used throughout the app.
// Plaid returns dates this way, and they are stored and queried in Firestore as
// lexicographically-ordered strings.
const DateLayout = "2006-01-02"

// FormatDate formats t as a YYYY-MM-DD string.
func FormatDate(t time.Time) string {
	return t.Format(DateLayout)
}

// ParseDate parses a YYYY-MM-DD string into a time.Time.
func ParseDate(s string) (time.Time, error) {
	return time.Parse(DateLayout, s)
}

// IsValidDate reports whether s is a well-formed YYYY-MM-DD date.
func IsValidDate(s string) bool {
	_, err := ParseDate(s)
	return err == nil
}

// DateOf strips the time-of-day from t, returning midnight UTC on the same
// calendar date.
func DateOf(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// FirstOfMonth returns midnight UTC on the 1st of t's month.
func FirstOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// MondayOf returns midnight UTC on the Monday of t's week (weeks run Mon–Sun).
func MondayOf(t time.Time) time.Time {
	d := DateOf(t)
	// time.Weekday is Sunday=0..Saturday=6; shift so Monday=0.
	offset := (int(d.Weekday()) + 6) % 7
	return d.AddDate(0, 0, -offset)
}
