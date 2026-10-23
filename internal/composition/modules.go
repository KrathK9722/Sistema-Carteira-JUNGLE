package composition

import (
	"context"
	"errors"
	"example.com/financial-service/internal/application"
	"example.com/financial-service/internal/configuration"
	"example.com/financial-service/internal/infrastructure/authentication"
	"example.com/financial-service/internal/infrastructure/messaging"
	"example.com/financial-service/internal/infrastructure/postgres"
	transport "example.com/financial-service/internal/transport/http"
	"example.com/financial-service/internal/workers"
	"example.com/financial-service/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"log/slog"
	"net"
	"net/http"
	"time"
)

type databaseLifecycle struct{ pool *pgxpool.Pool }

func (db databaseLifecycle) Start(ctx context.Context) error {
	err := db.pool.Ping(ctx)
	if err != nil {
		return err
	}
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(73428712)`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations(version integer PRIMARY KEY,applied_at timestamptz NOT NULL DEFAULT now())`)
	if err != nil {
		return err
	}
	var exists bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=1)`).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		_, err = tx.Exec(ctx, migrations.Up)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO schema_migrations(version) VALUES(1)`)
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
func (db databaseLifecycle) Stop(ctx context.Context) error { db.pool.Close(); return nil }
func NewPool(lifecycle fx.Lifecycle, config configuration.Config) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(config.DatabaseURL)
	if err != nil {
		return nil, err
	}
	poolConfig.MaxConns = 12
	poolConfig.ConnConfig.ConnectTimeout = 5 * time.Second
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, err
	}
	db := databaseLifecycle{pool: pool}
	lifecycle.Append(fx.Hook{OnStart: db.Start, OnStop: db.Stop})
	return pool, nil
}
func AsStore(repository *postgres.Repository) application.Store { return repository }
func RegisterQueues(lifecycle fx.Lifecycle, queues *messaging.Client) {
	lifecycle.Append(fx.Hook{OnStart: queues.Ensure})
}
func RegisterWorkers(lifecycle fx.Lifecycle, runner *workers.Runner) {
	lifecycle.Append(fx.Hook{OnStart: runner.Start, OnStop: runner.Stop})
}

type serverLifecycle struct {
	server *http.Server
	done   chan struct{}
}

func (server *serverLifecycle) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", server.server.Addr)
	if err != nil {
		return err
	}
	go server.serve(listener)
	return nil
}
func (server *serverLifecycle) serve(listener net.Listener) {
	defer close(server.done)
	err := server.server.Serve(listener)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("http server stopped unexpectedly", "error", err)
	}
}
func (server *serverLifecycle) Stop(ctx context.Context) error {
	err := server.server.Shutdown(ctx)
	if err != nil {
		server.server.Close()
		return err
	}
	select {
	case <-server.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func RegisterHTTP(lifecycle fx.Lifecycle, config configuration.Config, handler *transport.Handler) {
	server := &serverLifecycle{server: &http.Server{Addr: config.HTTPAddress, Handler: handler.Router(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second}, done: make(chan struct{})}
	lifecycle.Append(fx.Hook{OnStart: server.Start, OnStop: server.Stop})
}

var Module = fx.Module("financial-service", fx.Provide(configuration.Load, NewPool, postgres.NewRepository, AsStore, application.NewService, authentication.NewValidator, messaging.NewClient, transport.NewHandler, workers.NewRunner), fx.Invoke(RegisterQueues, RegisterWorkers, RegisterHTTP))
