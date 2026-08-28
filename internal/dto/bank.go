package dto

// BankDeleteParams is the params payload for a JobTypeBankDelete job.
// Stored as the Job.Params JSON blob.
type BankDeleteParams struct {
	BankID string `json:"bankId"`
}

// BankDeleteResult summarises the outcome of a bank delete job.
type BankDeleteResult struct {
	BankID string `json:"bankId"`
}
