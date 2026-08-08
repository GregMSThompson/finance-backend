package helpers

import "fmt"

// FormatPercent formats a percentage value to one decimal place with a trailing
// sign (e.g. 80.0 -> "80.0%"). It is for human-readable text (notifications,
// insights); numeric values returned to clients stay unrounded.
func FormatPercent(value float64) string {
	return fmt.Sprintf("%.1f%%", value)
}
