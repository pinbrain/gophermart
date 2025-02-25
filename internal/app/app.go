package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/pinbrain/gophermart/internal/agent"
	"github.com/pinbrain/gophermart/internal/config"
	"github.com/pinbrain/gophermart/internal/handlers"
	"github.com/pinbrain/gophermart/internal/logger"
	"github.com/pinbrain/gophermart/internal/storage"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

const (
	timeoutServerShutdown = time.Second * 5
	timeoutShutdown       = time.Second * 10
)

func Run() error {
	// корневой контекст приложения
	rootCtx, cancelCtx := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancelCtx()

	g, ctx := errgroup.WithContext(rootCtx)

	// нештатное завершение программы по таймауту
	// происходит, если после завершения контекста
	// приложение не смогло завершиться за отведенный промежуток времени
	context.AfterFunc(ctx, func() {
		ctx, cancelCtx := context.WithTimeout(context.Background(), timeoutShutdown)
		defer cancelCtx()

		<-ctx.Done()
		log.Fatal("failed to gracefully shutdown the service")
	})

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

	accrualAgent := agent.NewAccrualAgent(storage, serverConf.AccrualAddress)
	accrualAgent.StartAgent()

	router := handlers.NewRouter(storage)
	logger.Log.WithFields(logrus.Fields{
		"addr":    serverConf.ServerAddress,
		"log_lvl": serverConf.LogLevel,
	}).Info("Starting server")

	srv := &http.Server{
		Addr:    serverConf.ServerAddress,
		Handler: router,
	}

	// запуск сервера
	g.Go(func() (err error) {
		defer func() {
			errRec := recover()
			if errRec != nil {
				err = fmt.Errorf("a panic occurred: %v", errRec)
			}
		}()
		if err = srv.ListenAndServe(); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return fmt.Errorf("listen and server has failed: %w", err)
		}
		return nil
	})

	// отслеживаем успешное завершение работы сервиса
	g.Go(func() error {
		defer logger.Log.Info("Service has been shutdown")
		<-ctx.Done()
		logger.Log.Info("Gracefully shutting down service...")

		shutdownTimeoutCtx, cancelShutdownTimeoutCtx := context.WithTimeout(context.Background(), timeoutServerShutdown)
		defer cancelShutdownTimeoutCtx()
		if err := srv.Shutdown(shutdownTimeoutCtx); err != nil {
			logger.Log.Errorf("an error occurred during server shutdown: %v", err)
		}
		logger.Log.Info("HTTP server stopped")

		accrualAgent.StopAgent()
		logger.Log.Info("Accrual agent stopped")

		storage.Close()
		logger.Log.Info("Storage closed")

		return nil
	})

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}
