package helpers

import "fmt"

// FormatCurrency formats an amount with its currency code (e.g. "USD 12.50").
// If currency is empty, only the amount is returned.
func FormatCurrency(amount float64, currency string) string {
	if currency != "" {
		return fmt.Sprintf("%s %.2f", currency, amount)
	}
	return fmt.Sprintf("%.2f", amount)
}