package helpers

import (
	"fmt"
	"math"
)

// CurrencyUSD is the only currency supported today. Money is stored and
// processed throughout the system in minor units (integer); major-unit floats
// exist only at the two edges — the AI tool boundary and the client — and are
// produced via these helpers.
const CurrencyUSD = "USD"

// UnsupportedCurrencyError is returned by the conversion helpers when asked to
// handle a currency whose minor-unit exponent we don't (yet) support. It carries
// the offending code so callers can recover it via errors.As (mirroring the
// typed-error pattern in internal/errs). It lives here rather than in
// internal/errs to keep pkg/helpers free of internal dependencies; the service
// layer may wrap it into a domain/validation error.
type UnsupportedCurrencyError struct {
	Currency string
}

func (e *UnsupportedCurrencyError) Error() string {
	return fmt.Sprintf("unsupported currency: %q", e.Currency)
}

// ToMinorUnits converts a major-unit amount (e.g. dollars) into integer minor
// units (e.g. cents) for the given currency. It rounds to the nearest minor unit
// so the single float->int conversion at the ingestion/AI boundary is exact and
// well-defined; math.Round preserves sign, so Plaid's negative (inflow) amounts
// convert correctly.
func ToMinorUnits(major float64, currency string) (int64, error) {
	exp, err := minorUnitExponent(currency)
	if err != nil {
		return 0, err
	}
	return int64(math.Round(major * math.Pow10(exp))), nil
}

// ToMajorUnits converts integer minor units back into a major-unit float for the
// given currency. It is used only at display/LLM edges — internal math stays in
// minor units to avoid reintroducing float rounding.
func ToMajorUnits(minor int64, currency string) (float64, error) {
	exp, err := minorUnitExponent(currency)
	if err != nil {
		return 0, err
	}
	return float64(minor) / math.Pow10(exp), nil
}

// FormatCurrency formats an amount with its currency code (e.g. "USD 12.50").
// If currency is empty, only the amount is returned.
//
// NOTE: this still takes a major-unit float. It is slated to flip to minor units
// (int64) alongside the goal/snapshot migration, when its callers switch — see
// the currency migration plan.
func FormatCurrency(amount float64, currency string) string {
	if currency != "" {
		return fmt.Sprintf("%s %.2f", currency, amount)
	}
	return fmt.Sprintf("%.2f", amount)
}

// minorUnitExponent returns the number of decimal places in a currency's minor
// unit (USD -> 2, i.e. 100 cents). It is the single place the allowed-currency
// set is defined, so widening support later is a one-line change here rather
// than a hunt for hardcoded *100s.
func minorUnitExponent(currency string) (int, error) {
	switch currency {
	case CurrencyUSD:
		return 2, nil
	default:
		return 0, &UnsupportedCurrencyError{Currency: currency}
	}
}
