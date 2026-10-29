-- name: CreateFamily :one
INSERT INTO families (id, name, opening_balance_satang) VALUES ($1, $2, $3) RETURNING *;

-- name: GetFamily :one
SELECT * FROM families WHERE id = $1;

-- name: ListFamiliesForUser :many
SELECT f.*
FROM families f
JOIN family_members m ON m.family_id = f.id
WHERE m.user_id = $1 AND m.deleted_at IS NULL
ORDER BY f.created_at;

-- name: UpdateFamily :one
UPDATE families
SET name                   = COALESCE(sqlc.narg('name'), name),
    opening_balance_satang = COALESCE(sqlc.narg('opening_balance_satang'), opening_balance_satang),
    updated_at             = now()
WHERE id = $1
RETURNING *;

-- name: CreateInvite :one
INSERT INTO invites (code, family_id, created_by, member_id, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetInviteForUpdate :one
SELECT * FROM invites WHERE code = $1 FOR UPDATE;

-- name: MarkInviteUsed :exec
UPDATE invites SET used_by = $2, used_at = now() WHERE code = $1;
