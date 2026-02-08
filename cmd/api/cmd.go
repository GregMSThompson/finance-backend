package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/GregMSThompson/finance-backend/internal/bootstrap"
	"github.com/GregMSThompson/finance-backend/internal/config"
	"github.com/GregMSThompson/finance-backend/internal/crypto"
	"github.com/GregMSThompson/finance-backend/internal/handlers"
	"github.com/GregMSThompson/finance-backend/internal/response"
	"github.com/GregMSThompson/finance-backend/internal/router"
	"github.com/GregMSThompson/finance-backend/internal/services"
	"github.com/GregMSThompson/finance-backend/internal/store"
)

func exitOnError(message string, err error, log *slog.Logger) {
	if err != nil {
		log.Error(message, "error", err)
		os.Exit(1)
	}
}

func main() {
	// bootstrap
	cfg := config.New()
	bs, err := bootstrap.Run(cfg)
	exitOnError("bootstrap failed", err, bs.Log)
	defer bs.Close()

	// helpers
	kmsHelper := crypto.NewKMS(bs.KMS, cfg.KMSKeyName)

	// stores
	ustore := store.NewUserStore(bs.Firestore)
	tstore := store.NewTransactionStore(bs.Firestore)
	bstore := store.NewBankStore(bs.Firestore, kmsHelper)
	astore := store.NewAIStore(bs.Firestore)

	// services
	userv := services.NewUserService(ustore)
	bserv := services.NewBankService(bstore, tstore)
	plserv := services.NewPlaidService(bs.PlaidAdapter, bstore, tstore)
	anserv := services.NewAnalyticsService(tstore)
	aiserv := services.NewAIService(bs.VertexAdapter, anserv, astore, cfg.AITTL)

	// response handler
	rh := response.New(bs.Log)

	// dependancies
	deps := new(handlers.Deps)
	deps.Log = bs.Log
	deps.ResponseHandler = rh
	deps.Firebase = bs.Firebase
	deps.UserSvc = userv
	deps.BankSvc = bserv
	deps.PlaidSvc = plserv
	deps.AISvc = aiserv

	// router
	r := router.NewRouter(deps)
	err = http.ListenAndServe(":8080", r)
	exitOnError("server start failed", err, bs.Log)
}
