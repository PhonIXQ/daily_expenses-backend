package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/PhonIXQ/daily_expenses-backend/internal/platform/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.Handle("POST /v1/auth/email/request", httpx.HandlerFunc(h.requestOTP))
	mux.Handle("POST /v1/auth/email/verify", httpx.HandlerFunc(h.verifyOTP))
	mux.Handle("POST /v1/auth/refresh", httpx.HandlerFunc(h.refresh))
	mux.Handle("POST /v1/auth/logout", httpx.HandlerFunc(h.logout))
	mux.Handle("POST /v1/auth/providers/{provider}", httpx.HandlerFunc(h.provider))
}

func (h *Handler) requestOTP(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Email string `json:"email"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	if err := h.svc.RequestOTP(r.Context(), req.Email); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) verifyOTP(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	sess, err := h.svc.VerifyOTP(r.Context(), req.Email, req.Code)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, sess)
	return nil
}

func (h *Handler) provider(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		Credential string `json:"credential"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	sess, err := h.svc.SignInWithProvider(r.Context(), r.PathValue("provider"), req.Credential)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, sess)
	return nil
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	sess, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		return err
	}
	httpx.WriteJSON(w, http.StatusOK, sess)
	return nil
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) error {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	if err := h.svc.Logout(r.Context(), req.RefreshToken); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// ---------- middleware ----------

type ctxKey struct{}

// Require rejects requests without a valid "Authorization: Bearer <access token>".
func Require(tokens *Tokens, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		userID, err := tokens.Verify(raw)
		if !ok || err != nil {
			httpx.HandlerFunc(func(http.ResponseWriter, *http.Request) error {
				return httpx.Unauthorized("missing or invalid access token")
			}).ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, userID)))
	})
}

// UserID returns the authenticated user. Only valid behind Require.
func UserID(ctx context.Context) uuid.UUID {
	id, _ := ctx.Value(ctxKey{}).(uuid.UUID)
	return id
}
