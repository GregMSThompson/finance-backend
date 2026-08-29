package plaidclient

import (
	"context"
	"errors"
	"time"

	"github.com/plaid/plaid-go/v24/plaid"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/errs"
	"github.com/GregMSThompson/finance-backend/internal/models"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
	"github.com/GregMSThompson/finance-backend/pkg/logger"
)

type Adapter struct {
	client           *plaid.APIClient
	webhookURL       string
	oauthRedirectURI string
}

// NewAdapter builds a Plaid SDK wrapper. webhookURL is stamped onto every
// LinkTokenCreate call so new Items are registered to send events back to us;
// oauthRedirectURI enables OAuth-required institutions to redirect into our
// app after authentication. Pass empty strings for binaries that don't create
// link tokens (e.g. the worker).
func NewAdapter(clientID, secret string, env dto.PlaidEnvironment, webhookURL, oauthRedirectURI string) *Adapter {
	cfg := plaid.NewConfiguration()
	cfg.AddDefaultHeader("PLAID-CLIENT-ID", clientID)
	cfg.AddDefaultHeader("PLAID-SECRET", secret)
	cfg.UseEnvironment(toPlaidEnv(env))

	return &Adapter{
		client:           plaid.NewAPIClient(cfg),
		webhookURL:       webhookURL,
		oauthRedirectURI: oauthRedirectURI,
	}
}

func (a *Adapter) CreateLinkToken(ctx context.Context, uid string) (string, error) {
	req := plaid.NewLinkTokenCreateRequest(
		"Finance App",
		"en",
		[]plaid.CountryCode{plaid.CountryCode("US")},
		plaid.LinkTokenCreateRequestUser{ClientUserId: uid},
	)
	req.SetProducts([]plaid.Products{plaid.PRODUCTS_TRANSACTIONS})
	if a.webhookURL != "" {
		req.SetWebhook(a.webhookURL)
	}
	if a.oauthRedirectURI != "" {
		req.SetRedirectUri(a.oauthRedirectURI)
	}

	resp, _, err := a.client.PlaidApi.LinkTokenCreate(ctx).LinkTokenCreateRequest(*req).Execute()
	if err != nil {
		return "", errs.NewExternalServiceError("plaid", "failed to create link token", IsTransientError(err), err)
	}
	return resp.GetLinkToken(), nil
}

// PlaidWebhookKey is a minimal projection of the JWK Plaid returns from
// WebhookVerificationKeyGet, just enough to build an ECDSA P-256 public key
// and reason about expiration. ExpiredAt is a Unix timestamp; nil means active.
type PlaidWebhookKey struct {
	Kid       string
	Alg       string
	Kty       string
	Crv       string
	X         string
	Y         string
	ExpiredAt *int64
}

// WebhookVerificationKeyGet fetches the Plaid-hosted public key for the given
// kid. Used by the Verifier to validate inbound webhook signatures.
func (a *Adapter) WebhookVerificationKeyGet(ctx context.Context, kid string) (PlaidWebhookKey, error) {
	req := plaid.NewWebhookVerificationKeyGetRequest(kid)
	resp, _, err := a.client.PlaidApi.WebhookVerificationKeyGet(ctx).WebhookVerificationKeyGetRequest(*req).Execute()
	if err != nil {
		return PlaidWebhookKey{}, errs.NewExternalServiceError("plaid", "failed to fetch webhook verification key", IsTransientError(err), err)
	}
	key := resp.GetKey()
	out := PlaidWebhookKey{
		Kid: key.GetKid(),
		Alg: key.GetAlg(),
		Kty: key.GetKty(),
		Crv: key.GetCrv(),
		X:   key.GetX(),
		Y:   key.GetY(),
	}
	if exp, ok := key.GetExpiredAtOk(); ok && exp != nil {
		v := int64(*exp)
		out.ExpiredAt = &v
	}
	return out, nil
}

// GetAccounts fetches the accounts associated with the given access token.
func (a *Adapter) GetAccounts(ctx context.Context, accessToken string) ([]models.Account, error) {
	req := plaid.NewAccountsGetRequest(accessToken)
	resp, _, err := a.client.PlaidApi.AccountsGet(ctx).AccountsGetRequest(*req).Execute()
	if err != nil {
		return nil, errs.NewExternalServiceError("plaid", "failed to get accounts", IsTransientError(err), err)
	}

	now := time.Now()
	log := logger.FromContext(ctx)
	accounts := make([]models.Account, 0, len(resp.GetAccounts()))
	for _, acct := range resp.GetAccounts() {
		bal := acct.GetBalances()
		available, _ := bal.GetAvailableOk()
		current, _ := bal.GetCurrentOk()
		limit, _ := bal.GetLimitOk()
		currency := bal.GetIsoCurrencyCode()

		// Normalise balances to integer minor units at this ingestion boundary so
		// no float amount is stored. All three share the account's currency; if
		// it's unsupported we leave them unset and carry on rather than failing
		// the whole accounts fetch.
		availableMinor, err := balanceToMinor(available, currency)
		if err != nil {
			log.Warn("account balances left unset for unsupported currency",
				"accountId", acct.GetAccountId(), "currency", currency)
		}
		currentMinor, _ := balanceToMinor(current, currency)
		limitMinor, _ := balanceToMinor(limit, currency)

		accounts = append(accounts, models.Account{
			AccountID:             acct.GetAccountId(),
			Name:                  acct.GetName(),
			OfficialName:          acct.GetOfficialName(),
			Type:                  string(acct.GetType()),
			Subtype:               string(acct.GetSubtype()),
			Mask:                  acct.GetMask(),
			BalanceAvailableMinor: availableMinor,
			BalanceCurrentMinor:   currentMinor,
			BalanceLimitMinor:     limitMinor,
			Currency:              currency,
			CreatedAt:             now,
			UpdatedAt:             now,
		})
	}
	return accounts, nil
}

// balanceToMinor converts an optional major-unit balance into integer minor
// units, preserving nil (an absent balance stays absent). It returns an error
// for unsupported currencies so the caller can log and skip.
func balanceToMinor(major *float64, currency string) (*int64, error) {
	if major == nil {
		return nil, nil
	}
	minor, err := helpers.ToMinorUnits(*major, currency)
	if err != nil {
		return nil, err
	}
	return &minor, nil
}

func (a *Adapter) ExchangePublicToken(ctx context.Context, publicToken string) (itemID, accessToken string, err error) {
	req := plaid.NewItemPublicTokenExchangeRequest(publicToken)
	resp, _, err := a.client.PlaidApi.ItemPublicTokenExchange(ctx).ItemPublicTokenExchangeRequest(*req).Execute()
	if err != nil {
		return "", "", errs.NewExternalServiceError("plaid", "failed to exchange public token", IsTransientError(err), err)
	}
	return resp.GetItemId(), resp.GetAccessToken(), nil
}

func (a *Adapter) SyncTransactions(ctx context.Context, bankID string, accessToken string, cursor *string) (dto.PlaidSyncPage, error) {
	req := plaid.NewTransactionsSyncRequest(accessToken)
	if cursor != nil {
		req.SetCursor(*cursor)
	}
	req.SetCount(500)
	opts := plaid.NewTransactionsSyncRequestOptions()
	opts.SetIncludePersonalFinanceCategory(true)
	req.SetOptions(*opts)

	var page dto.PlaidSyncPage

	resp, _, err := a.client.PlaidApi.TransactionsSync(ctx).TransactionsSyncRequest(*req).Execute()
	if err != nil {
		return page, errs.NewExternalServiceError("plaid", "failed to sync transactions", IsTransientError(err), err)
	}

	txs := make([]models.Transaction, 0, len(resp.GetAdded())+len(resp.GetModified()))
	now := time.Now()
	log := logger.FromContext(ctx)

	// convert maps a Plaid transaction into our model, normalising Plaid's
	// major-unit float amount into integer minor units at this ingestion boundary
	// so no float amount is ever stored. Returns ok=false for transactions we
	// can't convert (unsupported currency) so the caller can skip them.
	convert := func(plaidTx plaid.Transaction) (models.Transaction, bool) {
		currency := plaidTx.GetIsoCurrencyCode()
		amountMinor, err := helpers.ToMinorUnits(plaidTx.GetAmount(), currency)
		if err != nil {
			// Carry on and log: one unsupported-currency transaction must not wedge
			// the whole sync. USD-only for now.
			log.Warn("skipping transaction with unsupported currency",
				"transaction_id", plaidTx.GetTransactionId(),
				"currency", currency)
			return models.Transaction{}, false
		}
		pfc := plaidTx.GetPersonalFinanceCategory()
		return models.Transaction{
			TransactionID:  plaidTx.GetTransactionId(),
			BankID:         bankID,
			AccountID:      plaidTx.GetAccountId(),
			Name:           plaidTx.GetName(),
			AmountMinor:    amountMinor,
			Currency:       currency,
			Pending:        plaidTx.GetPending(),
			Date:           plaidTx.GetDate(),
			AuthorizedDate: plaidTx.GetAuthorizedDate(),
			Categories:     plaidTx.GetCategory(),
			PFCPrimary:     pfc.GetPrimary(),
			PFCDetailed:    pfc.GetDetailed(),
			PFCConfidence:  pfc.GetConfidenceLevel(),
			PFCIconURL:     plaidTx.GetPersonalFinanceCategoryIconUrl(),
			CreatedAt:      now,
			UpdatedAt:      now,
		}, true
	}

	for _, t := range resp.GetAdded() {
		if tx, ok := convert(t); ok {
			txs = append(txs, tx)
		}
	}
	for _, t := range resp.GetModified() {
		if tx, ok := convert(t); ok {
			txs = append(txs, tx)
		}
	}

	page.Transactions = txs
	page.Cursor = resp.GetNextCursor()
	page.HasMore = resp.GetHasMore()

	return page, nil
}

func toPlaidEnv(env dto.PlaidEnvironment) plaid.Environment {
	switch env {
	case dto.PlaidSandbox:
		return plaid.Sandbox
	case dto.PalidDevelopment:
		return plaid.Development
	default: // dto.PlaidProduction:
		return plaid.Production
	}
}

// IsTransientError checks if a Plaid error is transient (retryable).
// Transient errors include rate limits, maintenance, and temporary service issues.
// Non-transient errors include authentication failures, invalid credentials, etc.
func IsTransientError(err error) bool {
	if err == nil {
		return false
	}

	// Check for Plaid API errors
	var apiErr plaid.GenericOpenAPIError
	if errors.As(err, &apiErr) {
		// Extract the PlaidError from the response body
		if plaidErr, ok := apiErr.Model().(plaid.PlaidError); ok {
			code := plaidErr.GetErrorCode()

			// Transient error codes that may succeed on retry
			switch code {
			case "RATE_LIMIT_EXCEEDED",
				"PLANNED_MAINTENANCE",
				"INTERNAL_SERVER_ERROR",
				"PRODUCT_NOT_READY":
				return true

			// Non-transient errors that won't succeed on retry
			case "INVALID_API_KEYS",
				"INVALID_SECRET",
				"INVALID_ACCESS_TOKEN",
				"INVALID_PUBLIC_TOKEN",
				"ITEM_LOGIN_REQUIRED",
				"ITEM_LOCKED",
				"ITEM_NOT_FOUND",
				"INSUFFICIENT_CREDENTIALS",
				"INVALID_CREDENTIALS",
				"INVALID_MFA",
				"INVALID_REQUEST",
				"INVALID_RESULT":
				return false
			}
		}
	}

	// For unknown errors, assume non-transient to avoid infinite retries
	return false
}
