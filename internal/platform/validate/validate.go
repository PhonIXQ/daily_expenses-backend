// Package validate holds small input checks shared by handlers.
package validate

import (
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PhonIXQ/daily_expenses-backend/internal/platform/httpx"
)

const maxNameRunes = 100

// Name trims s and requires 1–100 characters.
func Name(field, s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" || utf8.RuneCountInString(s) > maxNameRunes {
		return "", httpx.BadRequest(field + " must be 1–100 characters")
	}
	return s, nil
}

// Color requires a 32-bit ARGB value, as used by Flutter's Color.
func Color(c int64) error {
	if c < 0 || c > 0xFFFFFFFF {
		return httpx.BadRequest("color must be a 32-bit ARGB value")
	}
	return nil
}

// OptName validates an optional name for a PATCH; nil means "unchanged".
func OptName(field string, s *string) (pgtype.Text, error) {
	if s == nil {
		return pgtype.Text{}, nil
	}
	v, err := Name(field, *s)
	return pgtype.Text{String: v, Valid: true}, err
}

func OptColor(c *int64) (pgtype.Int8, error) {
	if c == nil {
		return pgtype.Int8{}, nil
	}
	return pgtype.Int8{Int64: *c, Valid: true}, Color(*c)
}
