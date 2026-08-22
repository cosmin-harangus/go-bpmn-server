// Package oidc provides an Authenticator that validates standard OIDC JWTs.
// On startup it fetches the provider's JWKS from {IssuerURL}/.well-known/jwks.json
// and caches the public keys locally. No proxy or sidecar required.
// Compatible with Clerk, Auth0, Zitadel, Keycloak, Supabase Auth, and any other
// OIDC-compliant provider.
package oidc

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/cosmin-harangus/go-bpmn-server/server/auth"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
)

// Config holds the OIDC configuration.
type Config struct {
	// IssuerURL is the base URL of the OIDC provider (e.g. https://accounts.example.com).
	// JWKS are fetched from IssuerURL + "/.well-known/jwks.json".
	IssuerURL string
	// TenantClaim is the JWT claim name that holds the tenant/account ID.
	TenantClaim string
	// UserClaim is the JWT claim name that holds the user ID. Defaults to "sub".
	UserClaim string
}

// Authenticator validates OIDC JWTs against the provider's published public keys.
type Authenticator struct {
	cfg  Config
	keys keySet
}

// New creates a new OIDC Authenticator and fetches the provider's JWKS.
func New(cfg Config, httpClient *http.Client) (*Authenticator, error) {
	if cfg.UserClaim == "" {
		cfg.UserClaim = "sub"
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	jwksURL := strings.TrimRight(cfg.IssuerURL, "/") + "/.well-known/jwks.json"
	keys, err := fetchJWKS(httpClient, jwksURL)
	if err != nil {
		return nil, fmt.Errorf("oidc: load jwks: %w", err)
	}
	return &Authenticator{cfg: cfg, keys: keys}, nil
}

func (a *Authenticator) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id, err := a.identityFromRequest(r)
			if err != nil {
				writeJSON(w, 401, map[string]string{"error": "unauthorized"})
				return
			}
			ctx := auth.WithIdentity(r.Context(), id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (a *Authenticator) PublicRoutes(_ chi.Router) {}

func (a *Authenticator) identityFromRequest(r *http.Request) (auth.Identity, error) {
	hdr := r.Header.Get("Authorization")
	if !strings.HasPrefix(hdr, "Bearer ") {
		return auth.Identity{}, errors.New("missing bearer token")
	}
	raw := strings.TrimPrefix(hdr, "Bearer ")

	tok, err := jwt.Parse(raw, a.keyFunc(), jwt.WithExpirationRequired())
	if err != nil || !tok.Valid {
		return auth.Identity{}, fmt.Errorf("invalid jwt: %w", err)
	}
	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return auth.Identity{}, errors.New("invalid claims")
	}
	tenantID, _ := claims[a.cfg.TenantClaim].(string)
	if tenantID == "" {
		return auth.Identity{}, fmt.Errorf("missing %q claim", a.cfg.TenantClaim)
	}
	userID, _ := claims[a.cfg.UserClaim].(string)
	return auth.Identity{UserID: userID, TenantID: tenantID}, nil
}

func (a *Authenticator) keyFunc() jwt.Keyfunc {
	return func(tok *jwt.Token) (any, error) {
		kid, _ := tok.Header["kid"].(string)
		key, ok := a.keys[kid]
		if !ok {
			return nil, fmt.Errorf("unknown kid %q", kid)
		}
		return key, nil
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
