# Daily Expenses — backend

Go API for the Daily Expenses Flutter app (บันทึกรายจ่ายครอบครัว): accounts, families, members and invites. Transactions, budgets, goals, receipts, sync and notifications come next.

## Run

Requires Go 1.26+ (`go.mod` pins it; with `GOTOOLCHAIN=auto` an older Go downloads it).

```bash
cp .env.example .env
make db      # Postgres in Docker
make run     # applies migrations, listens on :8080
```

With `APP_ENV=dev` the one-time sign-in codes are printed to the log.

## Test

```bash
TEST_DATABASE_URL=postgres://postgres:postgres@localhost:5432/daily_expenses_test?sslmode=disable make test
```

The integration test drops and recreates the `public` schema of that database. Without `TEST_DATABASE_URL` it is skipped.

## Layout

```
cmd/api/              entry point: config, migrations, HTTP server, graceful shutdown
db/migrations/        goose SQL migrations (embedded, applied on startup)
db/queries/           SQL for sqlc → internal/store (run `make generate` after editing)
internal/app/         wires services and routes into one http.Handler
internal/auth/        email OTP, provider interface, JWT access + rotating refresh tokens, middleware
internal/user/        GET/PATCH /v1/me
internal/family/      families, members, invites, and the membership/admin access checks
internal/platform/    config, httpx (JSON + errors + middleware), validate
internal/store/       sqlc-generated queries + Store (pool, InTx)
```

Each handler decodes and validates the request, the service holds the business rules and access checks, and the store only runs SQL.

## Conventions

- **Money** is integer satang (`*_satang`, `BIGINT`). The app converts it to and from baht.
- **IDs** are UUIDs. Create endpoints accept a client-generated `family_id` / `member_id`, so records made offline keep their IDs.
- **JSON keys** are snake_case and match the Flutter models (`member_id`, `is_admin`, `color` as 32-bit ARGB).
- **Names** are stored as sent. Sample names are Thai translation keys on the client and come back unchanged.
- **Soft deletes** (`deleted_at`) on synced records, so a future sync endpoint can propagate deletions.
- **Access:** every family-scoped call goes through `family.RequireMember` / `RequireAdmin`. Non-members get `404`, not `403`.
- **Errors:** `{"error": {"code": "...", "message": "..."}}`.

## API (v1)

All routes except `/healthz` and `/v1/auth/*` need `Authorization: Bearer <access_token>`.

| Method | Path | Notes |
|---|---|---|
| POST | `/v1/auth/email/request` | `{email}`. Sends a 6-digit code (10 min validity, 1 min resend wait). |
| POST | `/v1/auth/email/verify` | `{email, code}` → session. Each code allows 5 attempts. |
| POST | `/v1/auth/providers/{provider}` | `{credential}` → session, for registered `auth.Provider`s (none yet). |
| POST | `/v1/auth/refresh` | `{refresh_token}` → new session. Reusing a rotated token revokes all of the user's sessions. |
| POST | `/v1/auth/logout` | `{refresh_token}` |
| GET / PATCH | `/v1/me` | `{display_name?, locale?: system\|th\|en}` |
| POST | `/v1/families` | `{family_id?, family_name, opening_balance_satang, member: {member_id?, name, color}}`. The caller becomes admin. |
| GET | `/v1/families` | The caller's families. |
| GET / PATCH | `/v1/families/{id}` | GET includes members. PATCH (admin): `{family_name?, opening_balance_satang?}` |
| GET / POST | `/v1/families/{id}/members` | POST (admin) adds an account-less member: `{member_id?, name, color}` |
| PATCH / DELETE | `/v1/families/{id}/members/{memberId}` | Admins can change anyone; a member can rename themselves or leave. The last admin can't be demoted or removed. |
| POST | `/v1/families/{id}/invites` | Admin only. `{member_id?}` lets the invitee take over an account-less member. 8-character code, valid 7 days, single use. |
| POST | `/v1/invites/{code}/accept` | `{name, color}` (ignored when the invite targets a member) → family |

Session: `{user_id, is_new_user, access_token, access_token_expires_at, refresh_token, refresh_token_expires_at}`. Access tokens last 15 min, refresh tokens 60 days.

## Not done yet

- Production mailer. Outside `APP_ENV=dev` the server refuses to start.
- Google / Apple / LINE `auth.Provider` implementations.
- Per-IP rate limiting.
- OpenAPI spec.
- Categories and budgets, transactions, receipts (S3/MinIO), goals and contributions, reports, sync, push notifications.
