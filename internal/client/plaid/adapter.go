package plaidclient

import (
	"context"
	"time"

	"github.com/plaid/plaid-go/v24/plaid"

	"github.com/GregMSThompson/finance-backend/internal/dto"
	"github.com/GregMSThompson/finance-backend/internal/models"
)

type Adapter struct {
	client *plaid.APIClient
}

func NewAdapter(clientID, secret string, env dto.PlaidEnvironment) *Adapter {
	cfg := plaid.NewConfiguration()
	cfg.AddDefaultHeader("PLAID-CLIENT-ID", clientID)
	cfg.AddDefaultHeader("PLAID-SECRET", secret)
	cfg.UseEnvironment(toPlaidEnv(env))

	return &Adapter{
		client: plaid.NewAPIClient(cfg),
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

	resp, _, err := a.client.PlaidApi.LinkTokenCreate(ctx).LinkTokenCreateRequest(*req).Execute()
	if err != nil {
		return "", err
	}
	return resp.GetLinkToken(), nil
}

func (a *Adapter) ExchangePublicToken(ctx context.Context, publicToken string) (itemID, accessToken string, err error) {
	req := plaid.NewItemPublicTokenExchangeRequest(publicToken)
	resp, _, err := a.client.PlaidApi.ItemPublicTokenExchange(ctx).ItemPublicTokenExchangeRequest(*req).Execute()
	if err != nil {
		return "", "", err
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
		return page, err
	}

	txs := make([]models.Transaction, 0, len(resp.GetAdded())+len(resp.GetModified()))
	now := time.Now()

	convert := func(plaidTx plaid.Transaction) models.Transaction {
		pfc := plaidTx.GetPersonalFinanceCategory()
		return models.Transaction{
			TransactionID:  plaidTx.GetTransactionId(),
			BankID:         bankID,
			Name:           plaidTx.GetName(),
			Amount:         plaidTx.GetAmount(),
			Currency:       plaidTx.GetIsoCurrencyCode(),
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
		}
	}

	for _, t := range resp.GetAdded() {
		txs = append(txs, convert(t))
	}
	for _, t := range resp.GetModified() {
		txs = append(txs, convert(t))
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
