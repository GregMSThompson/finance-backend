package dto

type AnalyticsSpendTotalArgs struct {
	Pending    *bool
	PFCPrimary *string
	AccountID  *string
	Merchant   *string
	DateFrom   *string
	DateTo     *string
}

type AnalyticsSpendTotalResult struct {
	TotalMinor int64  `json:"totalMinor"`
	Currency   string `json:"currency"`
	From       string `json:"from,omitempty"`
	To         string `json:"to,omitempty"`
}

// AnalyticsIncomeTotalArgs mirrors the spend-total args minus a category filter:
// income is always the INCOME category, so PFCPrimary would be meaningless.
type AnalyticsIncomeTotalArgs struct {
	Pending   *bool
	AccountID *string
	Merchant  *string
	DateFrom  *string
	DateTo    *string
}

// AnalyticsIncomeTotalResult reports income as a positive magnitude even though
// inflows are stored with Plaid's negative sign.
type AnalyticsIncomeTotalResult struct {
	TotalMinor int64  `json:"totalMinor"`
	Currency   string `json:"currency"`
	From       string `json:"from,omitempty"`
	To         string `json:"to,omitempty"`
}

// AnalyticsContributionsTotalArgs mirrors the income-total args: contributions
// are always the TRANSFER_IN category, so no category filter is exposed. The
// AccountID scope is the point of the figure — it measures money moved into one
// account.
type AnalyticsContributionsTotalArgs struct {
	Pending   *bool
	AccountID *string
	Merchant  *string
	DateFrom  *string
	DateTo    *string
}

// AnalyticsContributionsTotalResult reports contributions (transfers in) as a
// positive magnitude even though inflows are stored with Plaid's negative sign.
type AnalyticsContributionsTotalResult struct {
	TotalMinor int64  `json:"totalMinor"`
	Currency   string `json:"currency"`
	From       string `json:"from,omitempty"`
	To         string `json:"to,omitempty"`
}

// AnalyticsAverageMonthlySpendResult reports average monthly spend over the
// available history within the requested lookback. MonthsOfData is how many months
// of actual history the average is based on (bounded by the lookback and the
// earliest transaction), so callers can reject a figure derived from too little
// history. AverageMinor is 0 when there is no history (MonthsOfData 0).
type AnalyticsAverageMonthlySpendResult struct {
	AverageMinor int64   `json:"averageMinor"`
	MonthsOfData float64 `json:"monthsOfData"`
	Currency     string  `json:"currency"`
	From         string  `json:"from,omitempty"`
	To           string  `json:"to,omitempty"`
}

type AnalyticsSpendBreakdownArgs struct {
	Pending    *bool
	PFCPrimary *string
	AccountID  *string
	DateFrom   *string
	DateTo     *string
	GroupBy    string
}

type AnalyticsBreakdownItem struct {
	Key        string `json:"key"`
	TotalMinor int64  `json:"totalMinor"`
	Count      int    `json:"count"`
}

type AnalyticsSpendBreakdownResult struct {
	GroupBy  string                   `json:"groupBy"`
	Items    []AnalyticsBreakdownItem `json:"items"`
	Currency string                   `json:"currency"`
	From     string                   `json:"from,omitempty"`
	To       string                   `json:"to,omitempty"`
}

type AnalyticsPeriodComparisonArgs struct {
	Pending      *bool
	PFCPrimary   *string
	AccountID    *string
	Merchant     *string
	CurrentFrom  string
	CurrentTo    string
	PreviousFrom string
	PreviousTo   string
	GroupBy      string
}

type PeriodSummary struct {
	TotalMinor int64                    `json:"totalMinor"`
	Count      int                      `json:"count"`
	Currency   string                   `json:"currency"`
	From       string                   `json:"from"`
	To         string                   `json:"to"`
	Items      []AnalyticsBreakdownItem `json:"items,omitempty"`
}

type BreakdownItemChange struct {
	Key                 string   `json:"key"`
	AbsoluteChangeMinor int64    `json:"absoluteChangeMinor"`
	PercentageChange    *float64 `json:"percentageChange,omitempty"`
	CountChange         int      `json:"countChange"`
}

type PeriodChange struct {
	AbsoluteChangeMinor int64    `json:"absoluteChangeMinor"`
	PercentageChange    *float64 `json:"percentageChange,omitempty"`
	CountChange         int      `json:"countChange"`
	// Currency is coalesced from the current/previous periods so the change stands
	// alone even when the current period is empty and has no currency of its own.
	Currency string                `json:"currency"`
	Items    []BreakdownItemChange `json:"items,omitempty"`
}

type AnalyticsPeriodComparisonResult struct {
	GroupBy  string        `json:"groupBy,omitempty"`
	Current  PeriodSummary `json:"current"`
	Previous PeriodSummary `json:"previous"`
	Change   PeriodChange  `json:"change"`
}

type AnalyticsRecurringArgs struct {
	AccountID *string
	DateFrom  string
	DateTo    string
}

type RecurringItem struct {
	Merchant               string `json:"merchant"`
	Frequency              string `json:"frequency"`
	TypicalAmountMinor     int64  `json:"typicalAmountMinor"`
	AmountIsVariable       bool   `json:"amountIsVariable"`
	Currency               string `json:"currency"`
	OccurrenceCount        int    `json:"occurrenceCount"`
	LastDate               string `json:"lastDate"`
	LastAmountMinor        int64  `json:"lastAmountMinor"`
	MonthlyEquivalentMinor int64  `json:"monthlyEquivalentMinor"`
}

type RecurringTransactionsResult struct {
	Items                       []RecurringItem `json:"items"`
	TotalMonthlyEquivalentMinor int64           `json:"totalMonthlyEquivalentMinor"`
	Currency                    string          `json:"currency"`
	From                        string          `json:"from"`
	To                          string          `json:"to"`
}

type AnalyticsMovingAverageArgs struct {
	Granularity string
	Scope       string
	PFCPrimary  *string
	Merchant    *string
	AccountID   *string
	DateFrom    string
	DateTo      string
}

type MovingAverageDataPoint struct {
	Period           string `json:"period"`
	TotalMinor       int64  `json:"totalMinor"`
	TransactionCount int    `json:"transactionCount"`
}

type MovingAverageItem struct {
	Key string `json:"key"`
	// AveragePerUnitMinor is a per-period average in minor units. It stays a
	// float because it's a computed average (total / units), not a raw amount;
	// the Minor suffix signals the unit so the client converts for display.
	AveragePerUnitMinor float64                  `json:"averagePerUnitMinor"`
	TransactionCount    int                      `json:"transactionCount"`
	Series              []MovingAverageDataPoint `json:"series"`
}

type AnalyticsTopNArgs struct {
	Dimension  string
	Direction  string // "top" or "bottom"
	Limit      int
	MinCount   int
	PFCPrimary *string
	AccountID  *string
	DateFrom   string
	DateTo     string
}

type TopNItem struct {
	Key        string  `json:"key"`
	TotalMinor int64   `json:"totalMinor"`
	Count      int     `json:"count"`
	Percentage float64 `json:"percentage"`
}

type AnalyticsTopNResult struct {
	Dimension       string     `json:"dimension"`
	Direction       string     `json:"direction"`
	TotalSpendMinor int64      `json:"totalSpendMinor"`
	Currency        string     `json:"currency"`
	From            string     `json:"from"`
	To              string     `json:"to"`
	Items           []TopNItem `json:"items"`
}

type AnalyticsIncomeVsExpensesArgs struct {
	AccountID *string
	DateFrom  string
	DateTo    string
}

type IncomeVsExpensesResult struct {
	IncomeMinor   int64  `json:"incomeMinor"`
	ExpensesMinor int64  `json:"expensesMinor"`
	NetMinor      int64  `json:"netMinor"`
	Currency      string `json:"currency"`
	From          string `json:"from"`
	To            string `json:"to"`
}

type AnalyticsMovingAverageResult struct {
	Granularity string `json:"granularity"`
	Scope       string `json:"scope"`
	// AveragePerUnitMinor is a per-period average in minor units — see the note
	// on MovingAverageItem.AveragePerUnitMinor.
	AveragePerUnitMinor float64                  `json:"averagePerUnitMinor"`
	TransactionCount    int                      `json:"transactionCount"`
	DaysAnalyzed        int                      `json:"daysAnalyzed"`
	Currency            string                   `json:"currency"`
	From                string                   `json:"from"`
	To                  string                   `json:"to"`
	Series              []MovingAverageDataPoint `json:"series,omitempty"`
	Items               []MovingAverageItem      `json:"items,omitempty"`
}
