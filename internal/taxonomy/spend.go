package taxonomy

// nonSpendPrimaries are PFC primary categories that don't represent
// discretionary expenditure: income and internal transfers between accounts.
// They're excluded from spend totals so a "how much did I spend" figure isn't
// netted against a paycheck or inflated/deflated by moving money around.
var nonSpendPrimaries = map[string]struct{}{
	"INCOME":       {},
	"TRANSFER_IN":  {},
	"TRANSFER_OUT": {},
}

// IsNonSpendCategory reports whether a PFC primary category is income or a
// transfer, and so should be left out of spend totals unless the caller has
// explicitly asked for that category.
func IsNonSpendCategory(primary string) bool {
	_, ok := nonSpendPrimaries[primary]
	return ok
}
