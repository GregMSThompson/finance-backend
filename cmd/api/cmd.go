package main

import (
	"net/http"

	"github.com/GregMSThompson/finance-backend/internal/bootstrap"
	plaidclient "github.com/GregMSThompson/finance-backend/internal/client/plaid"
	"github.com/GregMSThompson/finance-backend/internal/config"
	"github.com/GregMSThompson/finance-backend/internal/crypto"
	"github.com/GregMSThompson/finance-backend/internal/handlers"
	"github.com/GregMSThompson/finance-backend/internal/response"
	"github.com/GregMSThompson/finance-backend/internal/router"
	"github.com/GregMSThompson/finance-backend/internal/services"
	"github.com/GregMSThompson/finance-backend/internal/store"
	"github.com/GregMSThompson/finance-backend/pkg/helpers"
)

func main() {
	// bootstrap
	cfg := config.NewAPI()
	bs, err := bootstrap.RunAPI(cfg)
	helpers.ExitOnError("bootstrap failed", err, bs.Log)
	defer bs.Close()

	// helpers
	kmsHelper := crypto.NewKMS(bs.KMS, cfg.KMSKeyName)

	// stores
	ustore := store.NewUserStore(bs.Firestore)
	tstore := store.NewTransactionStore(bs.Firestore)
	acstore := store.NewAccountStore(bs.Firestore)
	bstore := store.NewBankStore(bs.Firestore, kmsHelper)
	astore := store.NewAIStore(bs.Firestore)
	dstore := store.NewDashboardStore(bs.Firestore)
	alstore := store.NewAlertStore(bs.Firestore)
	aestore := store.NewAlertEventStore(bs.Firestore)
	jstore := store.NewJobStore(bs.Firestore)

	// services
	userv := services.NewUserService(ustore)
	jobsvc := services.NewJobService(jstore, bs.CloudTasks)
	bserv := services.NewBankService(bstore, tstore, acstore, jobsvc)
	accountsvc := services.NewAccountsService(bs.PlaidAdapter, bstore, acstore, jobsvc)
	plserv := services.NewPlaidService(bs.PlaidAdapter, bstore, tstore, jobsvc, bserv, accountsvc)
	anserv := services.NewAnalyticsService(tstore)
	txserv := services.NewTransactionsService(tstore)
	aiserv := services.NewAIService(bs.VertexAdapter, anserv, txserv, astore, cfg.AITTL)
	dashsvc := services.NewDashboardService(dstore, anserv, txserv)
	alertsvc := services.NewAlertService(alstore, aestore)

	// response handler
	rh := response.New(bs.Log)

	// plaid webhook verifier — caches JWS public keys per kid
	plaidVerifier := plaidclient.NewVerifier(bs.PlaidAdapter)

	// dependancies
	deps := new(handlers.Deps)
	deps.Log = bs.Log
	deps.ResponseHandler = rh
	deps.Firebase = bs.Firebase
	deps.PlaidVerifier = plaidVerifier
	deps.AppleAppID = cfg.AppleAppID
	deps.UserSvc = userv
	deps.BankSvc = bserv
	deps.PlaidSvc = plserv
	deps.PlaidWebhookSvc = plserv
	deps.AISvc = aiserv
	deps.DashboardSvc = dashsvc
	deps.AlertSvc = alertsvc
	deps.JobSvc = jobsvc
	deps.TransactionsSvc = txserv
	deps.AccountsSvc = accountsvc

	// router
	r := router.NewAPIRouter(deps)
	err = http.ListenAndServe(":8080", r)
	helpers.ExitOnError("server start failed", err, bs.Log)
}
