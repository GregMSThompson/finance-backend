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
