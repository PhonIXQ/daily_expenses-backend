package family

import (
	"context"
	"crypto/rand"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/PhonIXQ/daily_expenses-backend/internal/platform/httpx"
	"github.com/PhonIXQ/daily_expenses-backend/internal/platform/validate"
	"github.com/PhonIXQ/daily_expenses-backend/internal/store"
)

const (
	inviteTTL = 7 * 24 * time.Hour
	// No 0/O/1/I so codes are easy to read out loud.
	inviteAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	inviteLength   = 8
)

type Service struct {
	store *store.Store
	now   func() time.Time
}

func NewService(st *store.Store) *Service { return &Service{store: st, now: time.Now} }

// ---------- families ----------

type NewMember struct {
	MemberID *uuid.UUID `json:"member_id"`
	Name     string     `json:"name"`
	Color    int64      `json:"color"`
}

type CreateFamilyInput struct {
	FamilyID             *uuid.UUID `json:"family_id"`
	FamilyName           string     `json:"family_name"`
	OpeningBalanceSatang int64      `json:"opening_balance_satang"`
	// The creator's own member record; they become its admin.
	Member NewMember `json:"member"`
}

func (s *Service) Create(ctx context.Context, userID uuid.UUID, in CreateFamilyInput) (FamilyJSON, error) {
	name, err := validate.Name("family_name", in.FamilyName)
	if err != nil {
		return FamilyJSON{}, err
	}
	memberName, err := validate.Name("member.name", in.Member.Name)
	if err != nil {
		return FamilyJSON{}, err
	}
	if err := validate.Color(in.Member.Color); err != nil {
		return FamilyJSON{}, err
	}

	var out FamilyJSON
	err = s.store.InTx(ctx, func(q *store.Queries) error {
		f, err := q.CreateFamily(ctx, store.CreateFamilyParams{
			ID: idOrNew(in.FamilyID), Name: name, OpeningBalanceSatang: in.OpeningBalanceSatang,
		})
		if err != nil {
			return err
		}
		m, err := q.CreateMember(ctx, store.CreateMemberParams{
			ID: idOrNew(in.Member.MemberID), FamilyID: f.ID, UserID: uuid.NullUUID{UUID: userID, Valid: true},
			Name: memberName, Color: in.Member.Color, Role: roleAdmin,
		})
		if err != nil {
			return err
		}
		out = familyJSON(f, []store.FamilyMember{m})
		return nil
	})
	if store.IsUniqueViolation(err) {
		return out, httpx.Conflict("family_id or member_id already exists")
	}
	return out, err
}

func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]FamilyJSON, error) {
	fams, err := s.store.ListFamiliesForUser(ctx, uuid.NullUUID{UUID: userID, Valid: true})
	if err != nil {
		return nil, err
	}
	out := make([]FamilyJSON, 0, len(fams))
	for _, f := range fams {
		out = append(out, familyJSON(f, nil))
	}
	return out, nil
}

func (s *Service) Get(ctx context.Context, userID, familyID uuid.UUID) (FamilyJSON, error) {
	if _, err := RequireMember(ctx, s.store.Queries, familyID, userID); err != nil {
		return FamilyJSON{}, err
	}
	return s.load(ctx, s.store.Queries, familyID)
}

type UpdateFamilyInput struct {
	FamilyName           *string `json:"family_name"`
	OpeningBalanceSatang *int64  `json:"opening_balance_satang"`
}

func (s *Service) Update(ctx context.Context, userID, familyID uuid.UUID, in UpdateFamilyInput) (FamilyJSON, error) {
	if _, err := RequireAdmin(ctx, s.store.Queries, familyID, userID); err != nil {
		return FamilyJSON{}, err
	}
	name, err := validate.OptName("family_name", in.FamilyName)
	if err != nil {
		return FamilyJSON{}, err
	}
	var balance pgtype.Int8
	if in.OpeningBalanceSatang != nil {
		balance = pgtype.Int8{Int64: *in.OpeningBalanceSatang, Valid: true}
	}
	f, err := s.store.UpdateFamily(ctx, store.UpdateFamilyParams{ID: familyID, Name: name, OpeningBalanceSatang: balance})
	if err != nil {
		return FamilyJSON{}, err
	}
	return familyJSON(f, nil), nil
}

func (s *Service) load(ctx context.Context, q *store.Queries, familyID uuid.UUID) (FamilyJSON, error) {
	f, err := q.GetFamily(ctx, familyID)
	if err != nil {
		return FamilyJSON{}, err
	}
	members, err := q.ListMembers(ctx, familyID)
	if err != nil {
		return FamilyJSON{}, err
	}
	return familyJSON(f, members), nil
}

// ---------- members ----------

func (s *Service) ListMembers(ctx context.Context, userID, familyID uuid.UUID) ([]MemberJSON, error) {
	if _, err := RequireMember(ctx, s.store.Queries, familyID, userID); err != nil {
		return nil, err
	}
	members, err := s.store.ListMembers(ctx, familyID)
	if err != nil {
		return nil, err
	}
	out := make([]MemberJSON, 0, len(members))
	for _, m := range members {
		out = append(out, memberJSON(m))
	}
	return out, nil
}

// AddMember creates a member without an account (e.g. a child). They can later
// claim it through an invite created with their member_id.
func (s *Service) AddMember(ctx context.Context, userID, familyID uuid.UUID, in NewMember) (MemberJSON, error) {
	if _, err := RequireAdmin(ctx, s.store.Queries, familyID, userID); err != nil {
		return MemberJSON{}, err
	}
	name, err := validate.Name("name", in.Name)
	if err != nil {
		return MemberJSON{}, err
	}
	if err := validate.Color(in.Color); err != nil {
		return MemberJSON{}, err
	}
	m, err := s.store.CreateMember(ctx, store.CreateMemberParams{
		ID: idOrNew(in.MemberID), FamilyID: familyID, Name: name, Color: in.Color, Role: roleMember,
	})
	if store.IsUniqueViolation(err) {
		return MemberJSON{}, httpx.Conflict("member_id already exists")
	}
	return memberJSON(m), err
}

type UpdateMemberInput struct {
	Name    *string `json:"name"`
	Color   *int64  `json:"color"`
	IsAdmin *bool   `json:"is_admin"`
}

// UpdateMember lets admins edit anyone; other members may only rename/recolor themselves.
func (s *Service) UpdateMember(ctx context.Context, userID, familyID, memberID uuid.UUID, in UpdateMemberInput) (MemberJSON, error) {
	name, err := validate.OptName("name", in.Name)
	if err != nil {
		return MemberJSON{}, err
	}
	color, err := validate.OptColor(in.Color)
	if err != nil {
		return MemberJSON{}, err
	}

	var out MemberJSON
	err = s.store.InTx(ctx, func(q *store.Queries) error {
		caller, err := RequireMember(ctx, q, familyID, userID)
		if err != nil {
			return err
		}
		target, err := q.GetMember(ctx, store.GetMemberParams{ID: memberID, FamilyID: familyID})
		if store.IsNotFound(err) {
			return httpx.NotFound("member not found")
		} else if err != nil {
			return err
		}
		isAdmin := caller.Role == roleAdmin
		if !isAdmin && (target.ID != caller.ID || in.IsAdmin != nil) {
			return httpx.Forbidden("only family admins can do this")
		}

		var role pgtype.Text
		if in.IsAdmin != nil {
			if !*in.IsAdmin && target.Role == roleAdmin {
				if err := ensureAnotherAdmin(ctx, q, familyID); err != nil {
					return err
				}
			}
			role = pgtype.Text{String: roleMember, Valid: true}
			if *in.IsAdmin {
				role.String = roleAdmin
			}
		}
		m, err := q.UpdateMember(ctx, store.UpdateMemberParams{
			ID: memberID, FamilyID: familyID, Name: name, Color: color, Role: role,
		})
		out = memberJSON(m)
		return err
	})
	return out, err
}

// RemoveMember lets admins remove anyone and members leave the family themselves.
func (s *Service) RemoveMember(ctx context.Context, userID, familyID, memberID uuid.UUID) error {
	return s.store.InTx(ctx, func(q *store.Queries) error {
		caller, err := RequireMember(ctx, q, familyID, userID)
		if err != nil {
			return err
		}
		target, err := q.GetMember(ctx, store.GetMemberParams{ID: memberID, FamilyID: familyID})
		if store.IsNotFound(err) {
			return httpx.NotFound("member not found")
		} else if err != nil {
			return err
		}
		if caller.Role != roleAdmin && target.ID != caller.ID {
			return httpx.Forbidden("only family admins can do this")
		}
		if target.Role == roleAdmin {
			if err := ensureAnotherAdmin(ctx, q, familyID); err != nil {
				return err
			}
		}
		return q.SoftDeleteMember(ctx, store.SoftDeleteMemberParams{ID: memberID, FamilyID: familyID})
	})
}

// ---------- invites ----------

type CreateInviteInput struct {
	// Optional: an account-less member the invitee will take over.
	MemberID *uuid.UUID `json:"member_id"`
}

func (s *Service) CreateInvite(ctx context.Context, userID, familyID uuid.UUID, in CreateInviteInput) (InviteJSON, error) {
	if _, err := RequireAdmin(ctx, s.store.Queries, familyID, userID); err != nil {
		return InviteJSON{}, err
	}
	var memberID uuid.NullUUID
	if in.MemberID != nil {
		m, err := s.store.GetMember(ctx, store.GetMemberParams{ID: *in.MemberID, FamilyID: familyID})
		if store.IsNotFound(err) {
			return InviteJSON{}, httpx.NotFound("member not found")
		} else if err != nil {
			return InviteJSON{}, err
		}
		if m.UserID.Valid {
			return InviteJSON{}, httpx.Conflict("member already has an account")
		}
		memberID = uuid.NullUUID{UUID: m.ID, Valid: true}
	}
	inv, err := s.store.CreateInvite(ctx, store.CreateInviteParams{
		Code: inviteCode(), FamilyID: familyID, CreatedBy: userID,
		MemberID: memberID, ExpiresAt: s.now().Add(inviteTTL),
	})
	if err != nil {
		return InviteJSON{}, err
	}
	return inviteJSON(inv), nil
}

type AcceptInviteInput struct {
	// Used only when the invite doesn't point at an existing member.
	Name  string `json:"name"`
	Color int64  `json:"color"`
}

func (s *Service) AcceptInvite(ctx context.Context, userID uuid.UUID, code string, in AcceptInviteInput) (FamilyJSON, error) {
	var out FamilyJSON
	err := s.store.InTx(ctx, func(q *store.Queries) error {
		inv, err := q.GetInviteForUpdate(ctx, code)
		if store.IsNotFound(err) || (err == nil && (inv.UsedAt != nil || s.now().After(inv.ExpiresAt))) {
			return httpx.NotFound("invite not found or expired")
		} else if err != nil {
			return err
		}
		if _, err := RequireMember(ctx, q, inv.FamilyID, userID); err == nil {
			return httpx.Conflict("already a member of this family")
		}

		self := uuid.NullUUID{UUID: userID, Valid: true}
		if inv.MemberID.Valid {
			if _, err := q.LinkMemberUser(ctx, store.LinkMemberUserParams{
				ID: inv.MemberID.UUID, FamilyID: inv.FamilyID, UserID: self,
			}); store.IsNotFound(err) {
				return httpx.Conflict("the invited member was removed or already claimed")
			} else if err != nil {
				return err
			}
		} else {
			name, err := validate.Name("name", in.Name)
			if err != nil {
				return err
			}
			if err := validate.Color(in.Color); err != nil {
				return err
			}
			if _, err := q.CreateMember(ctx, store.CreateMemberParams{
				ID: uuid.Must(uuid.NewV7()), FamilyID: inv.FamilyID, UserID: self,
				Name: name, Color: in.Color, Role: roleMember,
			}); err != nil {
				return err
			}
		}
		if err := q.MarkInviteUsed(ctx, store.MarkInviteUsedParams{Code: code, UsedBy: self}); err != nil {
			return err
		}
		out, err = s.load(ctx, q, inv.FamilyID)
		return err
	})
	return out, err
}

func inviteCode() string {
	b := make([]byte, inviteLength)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = inviteAlphabet[int(b[i])%len(inviteAlphabet)] // 256 % 32 == 0, so unbiased
	}
	return string(b)
}
