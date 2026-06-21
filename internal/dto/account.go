package dto

// AccountSyncParams are the job parameters for an account.sync job.
type AccountSyncParams struct {
	BankID string `json:"bankId"`
}

// AccountSyncResult is the outcome recorded on the job after a successful sync.
type AccountSyncResult struct {
	BankID         string `json:"bankId"`
	AccountsSynced int    `json:"accountsSynced"`
}
