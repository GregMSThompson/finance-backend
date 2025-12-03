package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/GregMSThompson/finance-backend/internal/bootstrap"
	"github.com/GregMSThompson/finance-backend/internal/config"
	"github.com/GregMSThompson/finance-backend/internal/handlers"
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

	// stores
	ustore := store.NewUserStore(bs.Firestore)

	// services
	userv := services.NewUserService(bs.Log, ustore)

	// dependancies
	deps := new(handlers.Deps)
	deps.Log = bs.Log
	deps.UserSvc = userv

	// router
	r := router.NewRouter(deps)
	err = http.ListenAndServe(":8080", r)
	exitOnError("server start failed", err, bs.Log)
}
