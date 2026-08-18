package server

import (
	"net/http"

	"github.com/cosmin-harangus/go-bpmn-engine/tenant"
)

// TenantFromHeader extracts X-Tenant-ID from the request header and injects it
// into the request context via tenant.WithID. Returns 400 if the header is missing.
func TenantFromHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.Header.Get("X-Tenant-ID")
		if tenantID == "" {
			writeError(w, 400, "X-Tenant-ID header is required")
			return
		}
		ctx := tenant.WithID(r.Context(), tenantID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
