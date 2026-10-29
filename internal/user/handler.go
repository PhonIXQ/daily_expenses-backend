// Package user serves the signed-in user's own profile.
package user

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PhonIXQ/daily_expenses-backend/internal/auth"
	"github.com/PhonIXQ/daily_expenses-backend/internal/platform/httpx"
	"github.com/PhonIXQ/daily_expenses-backend/internal/platform/validate"
	"github.com/PhonIXQ/daily_expenses-backend/internal/store"
)

type userJSON struct {
	UserID      uuid.UUID `json:"user_id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Locale      string    `json:"locale"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func toJSON(u store.User) userJSON {
	return userJSON{UserID: u.ID, Email: u.Email, DisplayName: u.DisplayName, Locale: u.Locale, UpdatedAt: u.UpdatedAt}
}

type Handler struct{ store *store.Store }

func NewHandler(st *store.Store) *Handler { return &Handler{store: st} }

// Register adds routes to an authenticated mux.
func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("GET /v1/me", httpx.HandlerFunc(h.get))
	mux.Handle("PATCH /v1/me", httpx.HandlerFunc(h.update))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) error {
	u, err := h.store.GetUser(r.Context(), auth.UserID(r.Context()))
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, toJSON(u))
	return nil
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		DisplayName *string `json:"display_name"`
		Locale      *string `json:"locale"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	name, err := validate.OptName("display_name", req.DisplayName)
	if err != nil {
		return err
	}
	var locale pgtype.Text
	if req.Locale != nil {
		switch *req.Locale {
		case "system", "th", "en":
			locale = pgtype.Text{String: *req.Locale, Valid: true}
		default:
			return httpx.BadRequest("locale must be system, th or en")
		}
	}
	u, err := h.store.UpdateUser(r.Context(), store.UpdateUserParams{
		ID: auth.UserID(r.Context()), DisplayName: name, Locale: locale,
	})
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, toJSON(u))
	return nil
}
