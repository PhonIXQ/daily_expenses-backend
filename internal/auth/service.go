// Package auth handles sign-in (email OTP and pluggable providers), access tokens
// and rotating refresh tokens.
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/PhonIXQ/daily_expenses-backend/internal/platform/httpx"
	"github.com/PhonIXQ/daily_expenses-backend/internal/store"
)

const (
	otpTTL         = 10 * time.Minute
	otpResendAfter = time.Minute
	otpMaxAttempts = 5
	providerEmail  = "email"
)

// Session is returned by every successful sign-in or refresh.
type Session struct {
	UserID           uuid.UUID `json:"user_id"`
	IsNewUser        bool      `json:"is_new_user"`
	AccessToken      string    `json:"access_token"`
	AccessExpiresAt  time.Time `json:"access_token_expires_at"`
	RefreshToken     string    `json:"refresh_token"`
	RefreshExpiresAt time.Time `json:"refresh_token_expires_at"`
}

type Service struct {
	store      *store.Store
	tokens     *Tokens
	mailer     Mailer
	otpKey     []byte
	refreshTTL time.Duration
	providers  map[string]Provider
	now        func() time.Time
}

func NewService(st *store.Store, tokens *Tokens, mailer Mailer, otpKey []byte, refreshTTL time.Duration) *Service {
	return &Service{
		store:      st,
		tokens:     tokens,
		mailer:     mailer,
		otpKey:     otpKey,
		refreshTTL: refreshTTL,
		providers:  map[string]Provider{},
		now:        time.Now,
	}
}

func (s *Service) RegisterProvider(p Provider) { s.providers[p.Name()] = p }

// ---------- email OTP ----------

func (s *Service) RequestOTP(ctx context.Context, rawEmail string) error {
	email, err := normalizeEmail(rawEmail)
	if err != nil {
		return err
	}
	now := s.now()
	if existing, err := s.store.GetOTP(ctx, email); err == nil {
		if now.Before(existing.ExpiresAt.Add(-otpTTL + otpResendAfter)) {
			return httpx.TooManyRequests("wait a minute before requesting another code")
		}
	} else if !store.IsNotFound(err) {
		return err
	}

	code, err := randomDigits(6)
	if err != nil {
		return err
	}
	if err := s.store.UpsertOTP(ctx, store.UpsertOTPParams{
		Email: email, CodeHash: s.hashOTP(email, code), ExpiresAt: now.Add(otpTTL),
	}); err != nil {
		return err
	}
	return s.mailer.SendOTP(ctx, email, code)
}

func (s *Service) VerifyOTP(ctx context.Context, rawEmail, code string) (Session, error) {
	email, err := normalizeEmail(rawEmail)
	if err != nil {
		return Session{}, err
	}
	invalid := httpx.Unauthorized("invalid or expired code")

	// Counted outside the sign-in transaction so failed guesses are never rolled back.
	otp, err := s.store.BumpOTPAttempts(ctx, email)
	if store.IsNotFound(err) {
		return Session{}, invalid
	} else if err != nil {
		return Session{}, err
	}
	if otp.Attempts > otpMaxAttempts || s.now().After(otp.ExpiresAt) ||
		!hmac.Equal(otp.CodeHash, s.hashOTP(email, strings.TrimSpace(code))) {
		return Session{}, invalid
	}

	var sess Session
	err = s.store.InTx(ctx, func(q *store.Queries) error {
		if err := q.DeleteOTP(ctx, email); err != nil {
			return err
		}
		userID, isNew, err := s.findOrCreateUser(ctx, q, providerEmail, Identity{Subject: email, Email: email}, true)
		if err != nil {
			return err
		}
		sess, err = s.issue(ctx, q, userID)
		sess.IsNewUser = isNew
		return err
	})
	return sess, err
}

// ---------- third-party providers ----------

func (s *Service) SignInWithProvider(ctx context.Context, name, credential string) (Session, error) {
	p, ok := s.providers[name]
	if !ok {
		return Session{}, httpx.NotFound("unknown sign-in provider")
	}
	id, err := p.Verify(ctx, credential)
	if err != nil {
		return Session{}, httpx.Unauthorized("credential rejected")
	}
	if id.Subject == "" {
		return Session{}, httpx.Unauthorized("credential has no subject")
	}
	if id.Email, err = normalizeEmail(id.Email); err != nil {
		return Session{}, httpx.Unauthorized("credential has no usable email")
	}

	var sess Session
	err = s.store.InTx(ctx, func(q *store.Queries) error {
		userID, isNew, err := s.findOrCreateUser(ctx, q, name, id, false)
		if err != nil {
			return err
		}
		sess, err = s.issue(ctx, q, userID)
		sess.IsNewUser = isNew
		return err
	})
	return sess, err
}

// findOrCreateUser resolves an identity to a user. linkByEmail allows attaching the
// identity to an existing account with the same email, which is only safe when the
// identity itself proves ownership of that email (email OTP).
func (s *Service) findOrCreateUser(ctx context.Context, q *store.Queries, provider string, id Identity, linkByEmail bool) (uuid.UUID, bool, error) {
	ident, err := q.GetIdentity(ctx, store.GetIdentityParams{Provider: provider, Subject: id.Subject})
	if err == nil {
		return ident.UserID, false, nil
	} else if !store.IsNotFound(err) {
		return uuid.Nil, false, err
	}

	isNew := false
	user, err := q.GetUserByEmail(ctx, id.Email)
	switch {
	case err == nil && !linkByEmail:
		return uuid.Nil, false, httpx.Conflict("an account with this email exists; sign in with email first")
	case store.IsNotFound(err):
		user, err = q.CreateUser(ctx, store.CreateUserParams{ID: uuid.Must(uuid.NewV7()), Email: id.Email, DisplayName: id.Name})
		if err != nil {
			return uuid.Nil, false, err
		}
		isNew = true
	case err != nil:
		return uuid.Nil, false, err
	}
	err = q.CreateIdentity(ctx, store.CreateIdentityParams{Provider: provider, Subject: id.Subject, UserID: user.ID})
	return user.ID, isNew, err
}

// ---------- refresh tokens ----------

// Refresh rotates a refresh token. Presenting an already-rotated token revokes every
// session of that user, since it means the token was copied.
func (s *Service) Refresh(ctx context.Context, token string) (Session, error) {
	var sess Session
	reused := false
	err := s.store.InTx(ctx, func(q *store.Queries) error {
		rt, err := q.GetRefreshTokenForUpdate(ctx, hashToken(token))
		if store.IsNotFound(err) {
			return httpx.Unauthorized("invalid refresh token")
		} else if err != nil {
			return err
		}
		if rt.RevokedAt != nil {
			reused = true
			return q.RevokeUserRefreshTokens(ctx, rt.UserID) // commit, then reject
		}
		if s.now().After(rt.ExpiresAt) {
			return httpx.Unauthorized("refresh token expired")
		}
		if err := q.RevokeRefreshToken(ctx, rt.ID); err != nil {
			return err
		}
		sess, err = s.issue(ctx, q, rt.UserID)
		return err
	})
	if err == nil && reused {
		return Session{}, httpx.Unauthorized("refresh token reused; signed out everywhere")
	}
	return sess, err
}

func (s *Service) Logout(ctx context.Context, token string) error {
	return s.store.RevokeRefreshTokenByHash(ctx, hashToken(token))
}

func (s *Service) issue(ctx context.Context, q *store.Queries, userID uuid.UUID) (Session, error) {
	access, accessExp, err := s.tokens.Issue(userID)
	if err != nil {
		return Session{}, err
	}
	refresh, hash := newRefreshToken()
	refreshExp := s.now().Add(s.refreshTTL)
	if err := q.CreateRefreshToken(ctx, store.CreateRefreshTokenParams{
		ID: uuid.Must(uuid.NewV7()), UserID: userID, TokenHash: hash, ExpiresAt: refreshExp,
	}); err != nil {
		return Session{}, err
	}
	return Session{
		UserID: userID, AccessToken: access, AccessExpiresAt: accessExp,
		RefreshToken: refresh, RefreshExpiresAt: refreshExp,
	}, nil
}

// ---------- helpers ----------

func (s *Service) hashOTP(email, code string) []byte {
	m := hmac.New(sha256.New, s.otpKey)
	m.Write([]byte(email + "\x00" + code))
	return m.Sum(nil)
}

func normalizeEmail(raw string) (string, error) {
	addr, err := mail.ParseAddress(strings.TrimSpace(raw))
	if err != nil || addr.Name != "" {
		return "", httpx.BadRequest("invalid email")
	}
	return strings.ToLower(addr.Address), nil
}

func randomDigits(n int) (string, error) {
	max := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(n)), nil)
	v, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%0*d", n, v), nil
}
