# Auth Design
**Date:** 2026-08-22
**Status:** Pinned — not yet implemented
**Depends on:** go-to-market design (2026-08-22-go-to-market-design.md)

---

## Goal

Built-in auth by default, pluggable by config. Self-hosters get a working JWT-based auth out of the box. Companies that want Clerk, Auth0, Zitadel, Keycloak, or any other OIDC-compliant provider swap it in via two env vars — zero code changes, no proxy, no sidecar.

---

## Core principle: tenant_id = account_id

- The "tenant" is an organization/account, not an individual user
- Multiple users belong to one account
- `tenant_id` lives in the JWT claim — never trusted from a raw request header
- API requests authenticated via JWT (`Authorization: Bearer <jwt>`) or API key (`Authorization: Bearer apk_...`)
- The existing `X-Tenant-ID` header is removed or demoted to internal/dev use only

---

## Authenticator Interface

```go
// server/auth.go
type Authenticator interface {
    // Middleware validates the request, extracts identity + tenant_id,
    // and injects both into the context. Returns 401 if invalid.
    Middleware() func(http.Handler) http.Handler
}
```

`server.New()` accepts an `Authenticator`. The caller (`cmd/server`) wires the implementation from config.

---

## Two Modes

### Mode 1 — Built-in (default)

~300 lines of Go in the server itself. No new services or dependencies beyond the existing PostgreSQL.

**New DB tables** (same database, new migration):
- `accounts` — one row per tenant (`id`, `name`, `created_at`)
- `users` — one row per user (`id`, `account_id`, `email`, `password_hash`, `role`, `created_at`)
- `api_keys` — (`id`, `key_hash`, `account_id`, `label`, `created_at`)

**Endpoints added to the server:**
- `POST /auth/login` — email+password → signs a JWT with `{ user_id, tenant_id }` claims using `AUTH_JWT_SECRET`

**Middleware behavior:**
1. Check `Authorization` header
2. If `Bearer apk_...` → look up key hash in DB, resolve `tenant_id`
3. If `Bearer <jwt>` → validate signature with `AUTH_JWT_SECRET`, read `tenant_id` from claims
4. Inject `tenant_id` and `user_id` into context

**Phase 1:** admin creates accounts and users via a CLI command or seed migration. No self-registration UI.

---

### Mode 2 — OIDC (external providers)

Validate standard OIDC JWTs locally. On startup, fetch the provider's public keys from `{OIDC_ISSUER_URL}/.well-known/jwks.json` and cache them. No proxy, no sidecar, no subrequests per API call.

**Middleware behavior:**
1. Validate JWT signature against cached JWKS (refresh keys on 401 from provider)
2. Read `tenant_id` from the claim named by `AUTH_TENANT_CLAIM`
3. Read `user_id` from the `sub` claim (standard) or configurable `AUTH_USER_CLAIM`
4. Inject into context — identical to built-in mode

**Compatible with any OIDC provider:**

| Provider | Multi-user accounts | Notes |
|---|---|---|
| **Clerk** | "Organizations" | Best choice for SaaS phase — managed, free to 10K MAU, includes invite UI |
| **Zitadel** | "Organizations" | Best OSS self-hosted recommendation — single binary, OIDC-native |
| **Auth0** | "Organizations" | Managed; organizations in paid tier |
| **Keycloak** | "Realms" / "Groups" | OSS, heavy JVM dependency |
| **Supabase Auth** | Custom JWT claims | Managed Postgres-native; manual org claim |
| **Google Workspace** | `hd` claim = domain | Simple for internal/single-org deployments |

**API keys in OIDC mode:** handled by your built-in DB layer regardless of auth mode. API key validation is always local — OIDC is only for human user sessions.

---

## Config

```
AUTH_MODE=builtin            # or "oidc"

# Required for builtin
AUTH_JWT_SECRET=...

# Required for oidc
OIDC_ISSUER_URL=https://your-provider.com
AUTH_TENANT_CLAIM=org_id     # claim name that holds the tenant/account ID
AUTH_USER_CLAIM=sub          # claim name for user ID (default: sub)
```

---

## What changes in server.go

- `New()` gains an `Authenticator` parameter
- Auth middleware registered after recovery, before all protected routes
- `TenantFromHeader` middleware replaced by auth middleware (`tenant_id` comes from JWT, not raw header)
- No handler changes

---

## Security properties

- No request can set an arbitrary `tenant_id` — always derived from a verified JWT or API key
- API keys are prefixed (`apk_`) and stored as hashes; scoped to a single tenant
- OIDC mode: your server never sees user credentials; JWT signature is verified against provider's published public keys
- Built-in mode: passwords are bcrypt-hashed; JWT secret never leaves the server

---

## Provider recommendations by deployment scenario

| Scenario | Recommendation |
|---|---|
| SaaS (you host it) | Clerk — managed orgs, invite flows, no infra |
| Self-hosted, want OSS auth | Zitadel — single binary, OIDC-native, organizations built-in |
| Self-hosted, already running Keycloak | Keycloak via OIDC mode |
| Self-hosted, no auth infra | Built-in (default) |
| Internal single-org deployment | Built-in or Google Workspace OIDC |

---

## Open questions (to resolve before implementation)

1. Should the built-in support user self-registration, or admin-only user creation in Phase 1?
2. Should API keys be per-user or per-account (tenant)?
3. Can a user belong to more than one tenant? (Affects JWT design — single `tenant_id` claim vs. array; OIDC providers handle this differently)
