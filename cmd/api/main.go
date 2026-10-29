package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PhonIXQ/daily_expenses-backend/internal/app"
	"github.com/PhonIXQ/daily_expenses-backend/internal/auth"
	"github.com/PhonIXQ/daily_expenses-backend/internal/platform/config"
)

func main() {
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := app.Migrate(ctx, pool); err != nil {
		return err
	}

	var mailer auth.Mailer
	if cfg.Dev {
		mailer = auth.LogMailer{}
	} else {
		return errors.New("no production mailer configured; set APP_ENV=dev to log OTP codes")
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           app.New(cfg, pool, mailer),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.Info("listening", "addr", cfg.Addr)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
