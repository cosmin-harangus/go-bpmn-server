package noauth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cosmin-harangus/go-bpmn-server/server/auth"
	"github.com/cosmin-harangus/go-bpmn-server/server/auth/noauth"
)

func applyMiddleware(a *noauth.NoAuth, next http.Handler) http.Handler {
	return a.Middleware()(next)
}

func TestNoAuth_RejectsMissingTenantHeader(t *testing.T) {
	h := applyMiddleware(&noauth.NoAuth{}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != 400 {
		t.Errorf("expected 400 without X-Tenant-ID, got %d", rr.Code)
	}
}

func TestNoAuth_InjectsTenantIDFromHeader(t *testing.T) {
	var got auth.Identity
	h := applyMiddleware(&noauth.NoAuth{}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = auth.FromContext(r.Context())
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Tenant-ID", "tenant-abc")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if got.TenantID != "tenant-abc" {
		t.Errorf("expected TenantID=tenant-abc, got %q", got.TenantID)
	}
}
