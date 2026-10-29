-- +goose Up

-- Accounts --------------------------------------------------------------

CREATE TABLE users (
    id           UUID PRIMARY KEY,
    email        TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL DEFAULT '',
    locale       TEXT NOT NULL DEFAULT 'system' CHECK (locale IN ('system', 'th', 'en')),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One row per external sign-in (email OTP, Google, Apple, LINE…).
CREATE TABLE auth_identities (
    provider   TEXT NOT NULL,
    subject    TEXT NOT NULL,
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (provider, subject)
);

-- Pending email one-time codes; only the hash is stored.
CREATE TABLE otp_codes (
    email      TEXT PRIMARY KEY,
    code_hash  BYTEA NOT NULL,
    attempts   INT NOT NULL DEFAULT 0,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE refresh_tokens (
    id          UUID PRIMARY KEY,
    user_id     UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash  BYTEA NOT NULL UNIQUE,
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX refresh_tokens_user_idx ON refresh_tokens (user_id);

-- Families --------------------------------------------------------------

-- Amounts are integer satang (1 THB = 100 satang).
CREATE TABLE families (
    id                     UUID PRIMARY KEY,
    name                   TEXT NOT NULL,
    opening_balance_satang BIGINT NOT NULL DEFAULT 0,
    currency               TEXT NOT NULL DEFAULT 'THB',
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- A member may have no account (user_id NULL), e.g. a child without a phone.
-- name is stored as given: sample names are Thai translation keys on the client.
CREATE TABLE family_members (
    id         UUID PRIMARY KEY,
    family_id  UUID NOT NULL REFERENCES families (id) ON DELETE CASCADE,
    user_id    UUID REFERENCES users (id) ON DELETE SET NULL,
    name       TEXT NOT NULL,
    color      BIGINT NOT NULL,
    role       TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('admin', 'member')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX family_members_family_idx ON family_members (family_id);
CREATE UNIQUE INDEX family_members_user_once ON family_members (family_id, user_id)
    WHERE user_id IS NOT NULL AND deleted_at IS NULL;

CREATE TABLE invites (
    code       TEXT PRIMARY KEY,
    family_id  UUID NOT NULL REFERENCES families (id) ON DELETE CASCADE,
    created_by UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- Optional: claim an existing account-less member instead of creating a new one.
    member_id  UUID REFERENCES family_members (id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_by    UUID REFERENCES users (id) ON DELETE SET NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE invites;
DROP TABLE family_members;
DROP TABLE families;
DROP TABLE refresh_tokens;
DROP TABLE otp_codes;
DROP TABLE auth_identities;
DROP TABLE users;
