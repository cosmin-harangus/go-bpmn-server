# Auth Design
**Date:** 2026-08-22
**Status:** Pinned — not yet implemented
**Depends on:** go-to-market design (2026-08-22-go-to-market-design.md)

---

## Goal

Built-in auth by default, pluggable by config. Self-hosters get a working JWT-based auth out of the box. Companies that want Kratos, Keycloak, or Auth0 swap it in via one env var — zero code changes.

---

## Core principle: tenant_id = account_id

- The "tenant" is an organization/account, not an individual user
- Multiple users belong to one account
- `tenant_id` lives in the JWT claim — never trusted from a raw request header
- API requests authenticated via JWT or API key, both of which carry `tenant_id`
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

## Implementations

### Built-in (default)

Package: `server/auth/builtin/`

- `POST /auth/login` — email+password → signed JWT with `user_id` + `tenant_id` claims
- JWT validated on every protected request via middleware
- Users and accounts stored in PostgreSQL (new `accounts` and `users` tables)
- Passwords hashed with bcrypt
- API keys stored in DB (`api_keys` table), associated with `tenant_id`; validated via `Authorization: Bearer apk_...` header
- Phase 1: admin creates users via CLI seed or admin endpoint (no self-registration UI)

### Forward auth (external providers)

Package: `server/auth/forward/`

- Delegates to an external service via the [forward auth pattern](https://doc.traefik.io/traefik/middlewares/http/forwardauth/)
- Makes a subrequest to `AUTH_FORWARD_URL`; 2xx = proceed, 401/403 = reject
- `tenant_id` and `user_id` extracted from response headers (configurable)
- Compatible with: ORY Kratos+Oathkeeper, Keycloak, Authelia, Auth0

---

## ORY Stack for Multi-User Accounts

When using ORY, the recommended stack is:

| Component | Role |
|---|---|
| **Kratos** | Identity management — login, registration, profile, sessions |
| **Hydra** | OAuth 2.0 / OIDC server — issues JWTs with custom claims |
| **Keto** | Permissions / RBAC — "user X is admin of account Y" |
| **Oathkeeper** | Identity-aware proxy — forward auth sidecar |

Kratos does **not** have a native organization/account concept in OSS. The pattern:

1. Store `tenant_id` (account ID) in Kratos identity metadata (`traits` or `metadata_public`)
2. On login, Kratos issues a session token
3. Oathkeeper intercepts requests, validates session, extracts identity metadata, forwards `X-Tenant-ID` + `X-User-ID` headers to your server
4. Alternatively: a thin "token exchange" endpoint in your server converts a Kratos session to a JWT with `tenant_id` claim

For full OAuth 2.0 (API key equivalent, machine-to-machine): add Hydra. It issues tokens with `tenant_id` in the claims via custom token hooks.

---

## Config

```
AUTH_MODE=builtin           # or "forward"
AUTH_JWT_SECRET=...         # required for builtin
AUTH_FORWARD_URL=...        # required for forward
AUTH_TENANT_HEADER=X-Tenant-ID   # header name carrying tenant_id (forward mode)
AUTH_USER_HEADER=X-User-ID       # header name carrying user_id (forward mode)
```

---

## What changes in server.go

- `New()` gains an `Authenticator` parameter
- Auth middleware registered after recovery, before all protected routes
- `TenantFromHeader` middleware replaced by auth middleware (tenant comes from JWT, not raw header)
- No handler changes

---

## Security properties

- No request can set an arbitrary tenant_id — it is always derived from a verified JWT or API key
- API keys are prefixed (`apk_`) to distinguish from JWTs; scoped to a single tenant
- Forward auth mode: trust is delegated to the external provider; your server never sees credentials

---

## Open questions (to resolve before implementation)

1. Should the built-in support user self-registration, or admin-only user creation in Phase 1?
2. Should API keys be per-user or per-account (tenant)?
3. Multi-account membership: can a user belong to more than one tenant? (Affects JWT design — single `tenant_id` claim vs. array)
