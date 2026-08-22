package oidc_test

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cosmin-harangus/go-bpmn-server/server/auth"
	"github.com/cosmin-harangus/go-bpmn-server/server/auth/oidc"
	"github.com/golang-jwt/jwt/v5"
)

// testProvider sets up a minimal JWKS server backed by a generated RSA key pair.
type testProvider struct {
	privateKey *rsa.PrivateKey
	server     *httptest.Server
}

func newTestProvider(t *testing.T) *testProvider {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p := &testProvider{privateKey: priv}
	p.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/jwks.json" {
			http.NotFound(w, r)
			return
		}
		pub := priv.Public().(*rsa.PublicKey)
		n := base64.RawURLEncoding.EncodeToString(pub.N.Bytes())
		e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes())
		json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{
				{"kty": "RSA", "kid": "key-1", "n": n, "e": e},
			},
		})
	}))
	t.Cleanup(p.server.Close)
	return p
}

func (p *testProvider) sign(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = "key-1"
	s, err := tok.SignedString(p.privateKey)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func newOIDCAuth(t *testing.T, provider *testProvider) *oidc.Authenticator {
	t.Helper()
	a, err := oidc.New(oidc.Config{
		IssuerURL:   provider.server.URL,
		TenantClaim: "org_id",
		UserClaim:   "sub",
	}, provider.server.Client())
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestOIDC_RejectsRequestWithNoAuthHeader(t *testing.T) {
	p := newTestProvider(t)
	a := newOIDCAuth(t, p)
	h := a.Middleware()(okHandler())

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != 401 {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestOIDC_AcceptsValidJWT_InjectsIdentity(t *testing.T) {
	p := newTestProvider(t)
	a := newOIDCAuth(t, p)

	tok := p.sign(t, jwt.MapClaims{
		"sub":    "user-42",
		"org_id": "tenant-99",
		"exp":    time.Now().Add(time.Hour).Unix(),
	})

	var got auth.Identity
	h := a.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = auth.FromContext(r.Context())
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if got.UserID != "user-42" {
		t.Errorf("expected UserID=user-42, got %q", got.UserID)
	}
	if got.TenantID != "tenant-99" {
		t.Errorf("expected TenantID=tenant-99, got %q", got.TenantID)
	}
}

func TestOIDC_RejectsExpiredJWT(t *testing.T) {
	p := newTestProvider(t)
	a := newOIDCAuth(t, p)

	tok := p.sign(t, jwt.MapClaims{
		"sub":    "user-1",
		"org_id": "tenant-1",
		"exp":    time.Now().Add(-time.Hour).Unix(),
	})

	h := a.Middleware()(okHandler())
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != 401 {
		t.Errorf("expected 401 for expired JWT, got %d", rr.Code)
	}
}

func TestOIDC_RejectsJWTSignedByUnknownKey(t *testing.T) {
	p := newTestProvider(t)
	a := newOIDCAuth(t, p)

	// Sign with a different key not in the JWKS.
	otherKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":    "user-1",
		"org_id": "tenant-1",
		"exp":    time.Now().Add(time.Hour).Unix(),
	})
	tok.Header["kid"] = "key-1" // claim to be key-1 but signed with wrong key
	s, _ := tok.SignedString(otherKey)

	h := a.Middleware()(okHandler())
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+s)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != 401 {
		t.Errorf("expected 401 for unknown key, got %d", rr.Code)
	}
}

func TestOIDC_RejectsMissingTenantClaim(t *testing.T) {
	p := newTestProvider(t)
	a := newOIDCAuth(t, p)

	tok := p.sign(t, jwt.MapClaims{
		"sub": "user-1",
		// org_id intentionally absent
		"exp": time.Now().Add(time.Hour).Unix(),
	})

	h := a.Middleware()(okHandler())
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != 401 {
		t.Errorf("expected 401 for missing tenant claim, got %d", rr.Code)
	}
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	})
}
