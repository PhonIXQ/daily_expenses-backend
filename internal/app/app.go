// Package app wires services and routes into one http.Handler.
package app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/PhonIXQ/daily_expenses-backend/db/migrations"
	"github.com/PhonIXQ/daily_expenses-backend/internal/auth"
	"github.com/PhonIXQ/daily_expenses-backend/internal/family"
	"github.com/PhonIXQ/daily_expenses-backend/internal/platform/config"
	"github.com/PhonIXQ/daily_expenses-backend/internal/platform/httpx"
	"github.com/PhonIXQ/daily_expenses-backend/internal/store"
	"github.com/PhonIXQ/daily_expenses-backend/internal/user"
)

// Migrate applies pending migrations.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return fmt.Errorf("goose: %w", err)
	}
	_, err = provider.Up(ctx)
	return err
}

// New builds the API. mailer may be nil outside dev, in which case OTP sign-in fails
// until a real mailer is configured.
func New(cfg config.Config, pool *pgxpool.Pool, mailer auth.Mailer) http.Handler {
	st := store.Open(pool)
	tokens := auth.NewTokens(cfg.JWTSecret, cfg.AccessTokenTTL)
	authSvc := auth.NewService(st, tokens, mailer, cfg.JWTSecret, cfg.RefreshTokenTTL)

	// Routes that require a signed-in user.
	private := http.NewServeMux()
	user.NewHandler(st).Register(private)
	family.NewHandler(family.NewService(st)).Register(private)

	mux := http.NewServeMux()
	mux.Handle("GET /healthz", httpx.HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
		if err := st.Ping(r.Context()); err != nil {
			return err
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		return nil
	}))
	auth.NewHandler(authSvc).Register(mux)
	mux.Handle("/v1/", auth.Require(tokens, private))

	return httpx.Recover(httpx.Logging(mux))
}
