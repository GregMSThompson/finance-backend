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

// ToMajorUnitsFloat converts a minor-unit amount that may be fractional (e.g. a
// per-period average) into major units for the given currency. It is like
// ToMajorUnits but accepts and returns a float rather than requiring an integer
// input, for callers converting already-decoded JSON numbers.
func ToMajorUnitsFloat(minor float64, currency string) (float64, error) {
	exp, err := minorUnitExponent(currency)
	if err != nil {
		return 0, err
	}
	return minor / math.Pow10(exp), nil
}

// FormatCurrency formats an integer minor-unit amount with its currency code
// (e.g. 1250 USD -> "USD 12.50"). The decimal precision is derived from the
// currency's minor-unit exponent, so it is correct for any supported currency
// rather than assuming two places. Returns an error for unsupported currencies,
// since the exponent is required to place the decimal point.
func FormatCurrency(minor int64, currency string) (string, error) {
	exp, err := minorUnitExponent(currency)
	if err != nil {
		return "", err
	}
	major := float64(minor) / math.Pow10(exp)
	return fmt.Sprintf("%s %.*f", currency, exp, major), nil
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
