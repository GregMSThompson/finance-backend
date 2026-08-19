package helpers

import (
	"errors"
	"testing"
)

func TestToMinorUnits(t *testing.T) {
	tests := []struct {
		name  string
		major float64
		want  int64
	}{
		{"whole", 500, 50000},
		{"two decimals", 12.34, 1234},
		{"rounds half up", 12.345, 1235},
		{"rounds down", 12.344, 1234},
		{"negative inflow", -9.99, -999},
		{"zero", 0, 0},
		// Float representation of 0.29 is slightly under; rounding must still land
		// on the intended cent rather than truncating to 28.
		{"float repr edge", 0.29, 29},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToMinorUnits(tt.major, CurrencyUSD)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ToMinorUnits(%v) = %d, want %d", tt.major, got, tt.want)
			}
		})
	}
}

func TestToMajorUnits(t *testing.T) {
	tests := []struct {
		name  string
		minor int64
		want  float64
	}{
		{"whole", 50000, 500},
		{"two decimals", 1234, 12.34},
		{"negative", -999, -9.99},
		{"zero", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ToMajorUnits(tt.minor, CurrencyUSD)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ToMajorUnits(%d) = %v, want %v", tt.minor, got, tt.want)
			}
		})
	}
}

func TestConversionRoundTrip(t *testing.T) {
	for _, major := range []float64{0, 0.01, 12.34, 500, 9999.99, -42.50} {
		minor, err := ToMinorUnits(major, CurrencyUSD)
		if err != nil {
			t.Fatalf("ToMinorUnits(%v): %v", major, err)
		}
		back, err := ToMajorUnits(minor, CurrencyUSD)
		if err != nil {
			t.Fatalf("ToMajorUnits(%d): %v", minor, err)
		}
		if back != major {
			t.Errorf("round trip %v -> %d -> %v", major, minor, back)
		}
	}
}

func TestUnsupportedCurrency(t *testing.T) {
	_, err := ToMinorUnits(1, "GBP")
	var uce *UnsupportedCurrencyError
	if !errors.As(err, &uce) {
		t.Fatalf("ToMinorUnits GBP: got %v, want *UnsupportedCurrencyError", err)
	}
	if uce.Currency != "GBP" {
		t.Errorf("Currency = %q, want %q", uce.Currency, "GBP")
	}

	if _, err := ToMajorUnits(1, ""); !errors.As(err, &uce) {
		t.Errorf("ToMajorUnits empty: got %v, want *UnsupportedCurrencyError", err)
	}
}
