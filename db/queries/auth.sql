-- name: UpsertOTP :exec
INSERT INTO otp_codes (email, code_hash, attempts, expires_at)
VALUES ($1, $2, 0, $3)
ON CONFLICT (email) DO UPDATE
SET code_hash = EXCLUDED.code_hash, attempts = 0, expires_at = EXCLUDED.expires_at;

-- name: GetOTP :one
SELECT * FROM otp_codes WHERE email = $1;

-- Counts a verification attempt atomically and returns the updated row.
-- name: BumpOTPAttempts :one
UPDATE otp_codes SET attempts = attempts + 1 WHERE email = $1 RETURNING *;

-- name: DeleteOTP :exec
DELETE FROM otp_codes WHERE email = $1;

-- name: GetIdentity :one
SELECT * FROM auth_identities WHERE provider = $1 AND subject = $2;

-- name: CreateIdentity :exec
INSERT INTO auth_identities (provider, subject, user_id) VALUES ($1, $2, $3);

-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at) VALUES ($1, $2, $3, $4);

-- name: GetRefreshTokenForUpdate :one
SELECT * FROM refresh_tokens WHERE token_hash = $1 FOR UPDATE;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL;

-- name: RevokeRefreshTokenByHash :exec
UPDATE refresh_tokens SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL;

-- name: RevokeUserRefreshTokens :exec
UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL;
