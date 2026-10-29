package auth

import (
	"context"
	"log/slog"
)

// Identity is a verified external account.
type Identity struct {
	Subject string
	Email   string
	Name    string
}

// Provider verifies a credential from a third-party sign-in SDK (e.g. a Google,
// Apple or LINE ID token) and returns the identity it proves. Register
// implementations with Service.RegisterProvider; they are served at
// POST /v1/auth/{provider}.
type Provider interface {
	Name() string
	Verify(ctx context.Context, credential string) (Identity, error)
}

// Mailer delivers one-time sign-in codes.
type Mailer interface {
	SendOTP(ctx context.Context, email, code string) error
}

// LogMailer writes codes to the log. Development only.
type LogMailer struct{}

func (LogMailer) SendOTP(ctx context.Context, email, code string) error {
	slog.InfoContext(ctx, "dev OTP", "email", email, "code", code)
	return nil
}
