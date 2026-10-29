-- name: CreateMember :one
INSERT INTO family_members (id, family_id, user_id, name, color, role)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetMember :one
SELECT * FROM family_members
WHERE id = $1 AND family_id = $2 AND deleted_at IS NULL;

-- name: GetMembership :one
SELECT * FROM family_members
WHERE family_id = $1 AND user_id = $2 AND deleted_at IS NULL;

-- name: ListMembers :many
SELECT * FROM family_members
WHERE family_id = $1 AND deleted_at IS NULL
ORDER BY created_at;

-- name: UpdateMember :one
UPDATE family_members
SET name       = COALESCE(sqlc.narg('name'), name),
    color      = COALESCE(sqlc.narg('color'), color),
    role       = COALESCE(sqlc.narg('role'), role),
    updated_at = now()
WHERE id = $1 AND family_id = $2 AND deleted_at IS NULL
RETURNING *;

-- name: LinkMemberUser :one
UPDATE family_members
SET user_id = $3, updated_at = now()
WHERE id = $1 AND family_id = $2 AND user_id IS NULL AND deleted_at IS NULL
RETURNING *;

-- name: SoftDeleteMember :exec
UPDATE family_members
SET deleted_at = now(), updated_at = now()
WHERE id = $1 AND family_id = $2 AND deleted_at IS NULL;

-- name: CountAdmins :one
SELECT count(*) FROM family_members
WHERE family_id = $1 AND role = 'admin' AND deleted_at IS NULL;
