// Package builtin provides a self-contained Authenticator backed by PostgreSQL.
// It supports JWT sessions (HS256, signed with a shared secret) and API keys
// (Bearer apk_... prefix, resolved via the store).
package builtin

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cosmin-harangus/go-bpmn-server/server/auth"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const apiKeyPrefix = "apk_"
const tokenTTL = 24 * time.Hour

// Authenticator implements auth.Authenticator using JWT + API keys.
type Authenticator struct {
	store     Store
	jwtSecret []byte
}

// New creates a new Authenticator.
func New(store Store, jwtSecret string) *Authenticator {
	return &Authenticator{store: store, jwtSecret: []byte(jwtSecret)}
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

func (a *Authenticator) PublicRoutes(r chi.Router) {
	r.Post("/auth/login", a.LoginHandler())
}

// LoginHandler handles POST /auth/login — email+password → JWT.
func (a *Authenticator) LoginHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid request"})
			return
		}

		user, err := a.store.GetUserByEmail(r.Context(), req.Email)
		if err != nil || user == nil {
			writeJSON(w, 401, map[string]string{"error": "invalid credentials"})
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
			writeJSON(w, 401, map[string]string{"error": "invalid credentials"})
			return
		}

		token, err := a.signToken(user.ID, user.TenantID)
		if err != nil {
			writeJSON(w, 500, map[string]string{"error": "could not issue token"})
			return
		}
		writeJSON(w, 200, map[string]string{"token": token})
	}
}

// identityFromRequest extracts and validates the identity from the Authorization header.
func (a *Authenticator) identityFromRequest(r *http.Request) (auth.Identity, error) {
	hdr := r.Header.Get("Authorization")
	if !strings.HasPrefix(hdr, "Bearer ") {
		return auth.Identity{}, errors.New("missing bearer token")
	}
	raw := strings.TrimPrefix(hdr, "Bearer ")

	if strings.HasPrefix(raw, apiKeyPrefix) {
		return a.identityFromAPIKey(r, raw)
	}
	return a.identityFromJWT(raw)
}

func (a *Authenticator) identityFromJWT(raw string) (auth.Identity, error) {
	tok, err := jwt.Parse(raw, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return a.jwtSecret, nil
	}, jwt.WithExpirationRequired())
	if err != nil || !tok.Valid {
		return auth.Identity{}, errors.New("invalid jwt")
	}
	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return auth.Identity{}, errors.New("invalid claims")
	}
	tenantID, _ := claims["tenant_id"].(string)
	userID, _ := claims["user_id"].(string)
	if tenantID == "" {
		return auth.Identity{}, errors.New("missing tenant_id claim")
	}
	return auth.Identity{UserID: userID, TenantID: tenantID}, nil
}

func (a *Authenticator) identityFromAPIKey(r *http.Request, raw string) (auth.Identity, error) {
	h := sha256.Sum256([]byte(raw))
	keyHash := fmt.Sprintf("%x", h)
	key, err := a.store.GetAPIKeyByHash(r.Context(), keyHash)
	if err != nil || key == nil {
		return auth.Identity{}, errors.New("invalid api key")
	}
	return auth.Identity{TenantID: key.TenantID}, nil
}

func (a *Authenticator) signToken(userID, tenantID string) (string, error) {
	claims := jwt.MapClaims{
		"user_id":   userID,
		"tenant_id": tenantID,
		"exp":       time.Now().Add(tokenTTL).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.jwtSecret)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
