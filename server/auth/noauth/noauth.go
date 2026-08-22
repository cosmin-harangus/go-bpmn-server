// Package noauth provides an Authenticator that trusts the X-Tenant-ID header
// directly. It is intended for development and testing only — never use in
// production as it allows any caller to impersonate any tenant.
package noauth

import (
	"encoding/json"
	"net/http"

	"github.com/cosmin-harangus/go-bpmn-server/server/auth"
	"github.com/go-chi/chi/v5"
)

// NoAuth reads tenant identity from the X-Tenant-ID request header.
// Requests without the header are rejected with 400.
type NoAuth struct{}

func (n *NoAuth) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenantID := r.Header.Get("X-Tenant-ID")
			if tenantID == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "X-Tenant-ID header is required"})
				return
			}
			ctx := auth.WithIdentity(r.Context(), auth.Identity{TenantID: tenantID})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (n *NoAuth) PublicRoutes(_ chi.Router) {}
