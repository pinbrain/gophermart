package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pinbrain/gophermart/internal/logger"
	"github.com/pinbrain/gophermart/internal/storage/migrations"

	"github.com/pressly/goose/v3"
)

type DB struct {
	pool *pgxpool.Pool
}

func newDB(ctx context.Context, dsn string) (*DB, error) {
	if err := runMigrations(dsn); err != nil {
		return nil, err
	}
	pool, err := initPool(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize a db connection: %w", err)
	}
	return &DB{pool: pool}, err
}

func initPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse the DNS: %w", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize a connection pool: %w", err)
	}
	if err = pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping the DB: %w", err)
	}
	return pool, nil
}

func runMigrations(dsn string) error {
	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(logger.Log)
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	sqlDB, err := goose.OpenDBWithDriver("postgres", dsn)
	defer func() {
		if err := sqlDB.Close(); err != nil {
			logger.Log.WithField("err", err).Error("failed to close db connection while migration")
		}

	}()
	if err != nil {
		return err
	}
	if err := goose.Up(sqlDB, "."); err != nil {
		return err
	}
	return nil
}
