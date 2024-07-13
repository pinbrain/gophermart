package app

import (
	"context"
	"net/http"

	"github.com/pinbrain/gophermart/internal/config"
	"github.com/pinbrain/gophermart/internal/handlers"
	"github.com/pinbrain/gophermart/internal/logger"
	"github.com/pinbrain/gophermart/internal/storage"
	"github.com/sirupsen/logrus"
)

func Run() error {
	ctx := context.Background()
	serverConf, err := config.InitConfig()
	if err != nil {
		return err
	}

	if err = logger.Initialize(serverConf.LogLevel); err != nil {
		return err
	}

	storage, err := storage.NewStorage(ctx, storage.StorageCfg{DSN: serverConf.DSN})
	if err != nil {
		return err
	}
	defer storage.Close()

	router := handlers.NewRouter(storage)
	logger.Log.WithFields(logrus.Fields{
		"addr":    serverConf.ServerAddress,
		"log_lvl": serverConf.LogLevel,
	}).Info("Starting server")

	return http.ListenAndServe(serverConf.ServerAddress, router)
}
