package main

import (
	"net/http"

	"github.com/GregMSThompson/finance-backend/internal/bootstrap"
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
	bstore := store.NewBankStore(bs.Firestore, kmsHelper)
	astore := store.NewAIStore(bs.Firestore)
	dstore := store.NewDashboardStore(bs.Firestore)
	alstore := store.NewAlertStore(bs.Firestore)
	aestore := store.NewAlertEventStore(bs.Firestore)
	jstore := store.NewJobStore(bs.Firestore)

	// services
	userv := services.NewUserService(ustore)
	jobsvc := services.NewJobService(jstore, bs.CloudTasks)
	bserv := services.NewBankService(bstore, tstore, jobsvc)
	plserv := services.NewPlaidService(bs.PlaidAdapter, bstore, tstore, jobsvc)
	anserv := services.NewAnalyticsService(tstore)
	aiserv := services.NewAIService(bs.VertexAdapter, anserv, astore, cfg.AITTL)
	dashsvc := services.NewDashboardService(dstore, anserv)
	alertsvc := services.NewAlertService(alstore, aestore)

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
	deps.DashboardSvc = dashsvc
	deps.AlertSvc = alertsvc
	deps.JobSvc = jobsvc

	// router
	r := router.NewAPIRouter(deps)
	err = http.ListenAndServe(":8080", r)
	helpers.ExitOnError("server start failed", err, bs.Log)
}
