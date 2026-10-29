// Package family manages families, their members and invites.
package family

import (
	"time"

	"github.com/google/uuid"

	"github.com/PhonIXQ/daily_expenses-backend/internal/store"
)

const (
	roleAdmin  = "admin"
	roleMember = "member"
)

// JSON keys follow the Flutter models (member_id, is_admin, color…).

type FamilyJSON struct {
	FamilyID             uuid.UUID    `json:"family_id"`
	FamilyName           string       `json:"family_name"`
	OpeningBalanceSatang int64        `json:"opening_balance_satang"`
	Currency             string       `json:"currency"`
	UpdatedAt            time.Time    `json:"updated_at"`
	Members              []MemberJSON `json:"members,omitempty"`
}

type MemberJSON struct {
	MemberID  uuid.UUID  `json:"member_id"`
	UserID    *uuid.UUID `json:"user_id"`
	Name      string     `json:"name"`
	Color     int64      `json:"color"`
	IsAdmin   bool       `json:"is_admin"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type InviteJSON struct {
	Code      string     `json:"code"`
	FamilyID  uuid.UUID  `json:"family_id"`
	MemberID  *uuid.UUID `json:"member_id"`
	ExpiresAt time.Time  `json:"expires_at"`
}

func familyJSON(f store.Family, members []store.FamilyMember) FamilyJSON {
	out := FamilyJSON{
		FamilyID: f.ID, FamilyName: f.Name, OpeningBalanceSatang: f.OpeningBalanceSatang,
		Currency: f.Currency, UpdatedAt: f.UpdatedAt,
	}
	for _, m := range members {
		out.Members = append(out.Members, memberJSON(m))
	}
	return out
}

func memberJSON(m store.FamilyMember) MemberJSON {
	return MemberJSON{
		MemberID: m.ID, UserID: nullUUID(m.UserID), Name: m.Name, Color: m.Color,
		IsAdmin: m.Role == roleAdmin, UpdatedAt: m.UpdatedAt,
	}
}

func inviteJSON(i store.Invite) InviteJSON {
	return InviteJSON{Code: i.Code, FamilyID: i.FamilyID, MemberID: nullUUID(i.MemberID), ExpiresAt: i.ExpiresAt}
}

func nullUUID(n uuid.NullUUID) *uuid.UUID {
	if !n.Valid {
		return nil
	}
	return &n.UUID
}

// idOrNew returns the client-generated ID if given (offline-created records), else a new UUIDv7.
func idOrNew(id *uuid.UUID) uuid.UUID {
	if id != nil && *id != uuid.Nil {
		return *id
	}
	return uuid.Must(uuid.NewV7())
}
