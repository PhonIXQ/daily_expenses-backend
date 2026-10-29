package family

import (
	"context"

	"github.com/google/uuid"

	"github.com/PhonIXQ/daily_expenses-backend/internal/platform/httpx"
	"github.com/PhonIXQ/daily_expenses-backend/internal/store"
)

// RequireMember returns the caller's membership in a family. Non-members get 404 so
// family IDs can't be probed. Every family-scoped endpoint (including later
// transactions, goals, reports) must go through this or RequireAdmin.
func RequireMember(ctx context.Context, q *store.Queries, familyID, userID uuid.UUID) (store.FamilyMember, error) {
	m, err := q.GetMembership(ctx, store.GetMembershipParams{
		FamilyID: familyID, UserID: uuid.NullUUID{UUID: userID, Valid: true},
	})
	if store.IsNotFound(err) {
		return m, httpx.NotFound("family not found")
	}
	return m, err
}

func RequireAdmin(ctx context.Context, q *store.Queries, familyID, userID uuid.UUID) (store.FamilyMember, error) {
	m, err := RequireMember(ctx, q, familyID, userID)
	if err == nil && m.Role != roleAdmin {
		return m, httpx.Forbidden("only family admins can do this")
	}
	return m, err
}

// ensureAnotherAdmin rejects removing or demoting the family's last admin.
func ensureAnotherAdmin(ctx context.Context, q *store.Queries, familyID uuid.UUID) error {
	n, err := q.CountAdmins(ctx, familyID)
	if err == nil && n <= 1 {
		return httpx.Conflict("a family needs at least one admin")
	}
	return err
}
