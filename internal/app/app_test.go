package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/PhonIXQ/daily_expenses-backend/internal/app"
	"github.com/PhonIXQ/daily_expenses-backend/internal/platform/config"
)

// Integration test against a real Postgres. Set TEST_DATABASE_URL to a disposable
// database; its public schema is dropped and recreated.

type captureMailer struct {
	mu    sync.Mutex
	codes map[string]string
}

func (m *captureMailer) SendOTP(_ context.Context, email, code string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.codes[email] = code
	return nil
}

type client struct {
	t     *testing.T
	base  string
	token string
}

func (c *client) do(method, path string, body any, wantStatus int, out any) {
	c.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req, _ := http.NewRequest(method, c.base+path, &buf)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != wantStatus {
		var e map[string]any
		_ = json.NewDecoder(res.Body).Decode(&e)
		c.t.Fatalf("%s %s = %d %v; want %d", method, path, res.StatusCode, e, wantStatus)
	}
	if out != nil {
		if err := json.NewDecoder(res.Body).Decode(out); err != nil {
			c.t.Fatal(err)
		}
	}
}

type session struct {
	UserID       string `json:"user_id"`
	IsNewUser    bool   `json:"is_new_user"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func setup(t *testing.T) (*httptest.Server, *captureMailer) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, "DROP SCHEMA public CASCADE; CREATE SCHEMA public"); err != nil {
		t.Fatal(err)
	}
	if err := app.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	mailer := &captureMailer{codes: map[string]string{}}
	cfg := config.Config{
		JWTSecret: []byte("0123456789abcdef0123456789abcdef"), AccessTokenTTL: time.Minute, RefreshTokenTTL: time.Hour,
	}
	srv := httptest.NewServer(app.New(cfg, pool, mailer))
	t.Cleanup(srv.Close)
	return srv, mailer
}

func signIn(t *testing.T, srv *httptest.Server, mailer *captureMailer, email string) (*client, session) {
	c := &client{t: t, base: srv.URL}
	c.do("POST", "/v1/auth/email/request", map[string]string{"email": email}, http.StatusNoContent, nil)
	var s session
	c.do("POST", "/v1/auth/email/verify", map[string]string{"email": email, "code": mailer.codes[strings.ToLower(email)]}, http.StatusOK, &s)
	c.token = s.AccessToken
	return c, s
}

func TestFamilyFlow(t *testing.T) {
	srv, mailer := setup(t)

	// Resends are throttled and wrong codes are rejected.
	anon := &client{t: t, base: srv.URL}
	anon.do("POST", "/v1/auth/email/request", map[string]string{"email": "dad@example.com"}, http.StatusNoContent, nil)
	anon.do("POST", "/v1/auth/email/request", map[string]string{"email": "dad@example.com"}, http.StatusTooManyRequests, nil)
	anon.do("POST", "/v1/auth/email/verify", map[string]string{"email": "dad@example.com", "code": "000000x"}, http.StatusUnauthorized, nil)
	anon.do("GET", "/v1/me", nil, http.StatusUnauthorized, nil)

	mom, momSess := signIn(t, srv, mailer, "Mom@Example.com")
	if !momSess.IsNewUser {
		t.Error("first sign-in should be a new user")
	}
	var me map[string]any
	mom.do("GET", "/v1/me", nil, http.StatusOK, &me)
	if me["email"] != "mom@example.com" {
		t.Errorf("email = %v", me["email"])
	}

	// Create a family with a client-generated ID (offline-created).
	familyID := "0192f000-0000-7000-8000-000000000001"
	var fam struct {
		FamilyID string `json:"family_id"`
		Members  []struct {
			MemberID string `json:"member_id"`
			IsAdmin  bool   `json:"is_admin"`
		} `json:"members"`
	}
	mom.do("POST", "/v1/families", map[string]any{
		"family_id": familyID, "family_name": "ครอบครัวสุขใจ", "opening_balance_satang": 4852000,
		"member": map[string]any{"name": "แม่", "color": 0xFFFF7A1A},
	}, http.StatusCreated, &fam)
	if fam.FamilyID != familyID || len(fam.Members) != 1 || !fam.Members[0].IsAdmin {
		t.Fatalf("created family = %+v", fam)
	}
	momMemberID := fam.Members[0].MemberID

	// An account-less child, later claimed through an invite.
	var child struct {
		MemberID string `json:"member_id"`
	}
	mom.do("POST", "/v1/families/"+familyID+"/members", map[string]any{"name": "ลูก", "color": 0xFF2D8CFF}, http.StatusCreated, &child)
	var inv struct {
		Code string `json:"code"`
	}
	mom.do("POST", "/v1/families/"+familyID+"/invites", map[string]any{"member_id": child.MemberID}, http.StatusCreated, &inv)

	kid, _ := signIn(t, srv, mailer, "kid@example.com")
	kid.do("GET", "/v1/families/"+familyID, nil, http.StatusNotFound, nil) // not a member yet
	kid.do("POST", "/v1/invites/"+inv.Code+"/accept", map[string]any{}, http.StatusOK, nil)
	kid.do("POST", "/v1/invites/"+inv.Code+"/accept", map[string]any{}, http.StatusNotFound, nil) // single use

	var members []struct {
		MemberID string  `json:"member_id"`
		UserID   *string `json:"user_id"`
	}
	kid.do("GET", "/v1/families/"+familyID+"/members", nil, http.StatusOK, &members)
	if len(members) != 2 || members[1].MemberID != child.MemberID || members[1].UserID == nil {
		t.Fatalf("members after claim = %+v", members)
	}

	// Members can rename themselves but not others, and can't change roles or invite.
	kid.do("PATCH", "/v1/families/"+familyID+"/members/"+child.MemberID, map[string]any{"name": "น้องเอ"}, http.StatusOK, nil)
	kid.do("PATCH", "/v1/families/"+familyID+"/members/"+momMemberID, map[string]any{"name": "x"}, http.StatusForbidden, nil)
	kid.do("PATCH", "/v1/families/"+familyID+"/members/"+child.MemberID, map[string]any{"is_admin": true}, http.StatusForbidden, nil)
	kid.do("POST", "/v1/families/"+familyID+"/invites", map[string]any{}, http.StatusForbidden, nil)

	// The last admin can't step down or be removed.
	mom.do("PATCH", "/v1/families/"+familyID+"/members/"+momMemberID, map[string]any{"is_admin": false}, http.StatusConflict, nil)
	mom.do("DELETE", "/v1/families/"+familyID+"/members/"+momMemberID, nil, http.StatusConflict, nil)

	// A member can leave.
	kid.do("DELETE", "/v1/families/"+familyID+"/members/"+child.MemberID, nil, http.StatusNoContent, nil)
	kid.do("GET", "/v1/families/"+familyID, nil, http.StatusNotFound, nil)

	// Refresh rotates; replaying the old token signs the user out everywhere.
	var next session
	anon.do("POST", "/v1/auth/refresh", map[string]string{"refresh_token": momSess.RefreshToken}, http.StatusOK, &next)
	anon.do("POST", "/v1/auth/refresh", map[string]string{"refresh_token": momSess.RefreshToken}, http.StatusUnauthorized, nil)
	anon.do("POST", "/v1/auth/refresh", map[string]string{"refresh_token": next.RefreshToken}, http.StatusUnauthorized, nil)
}
