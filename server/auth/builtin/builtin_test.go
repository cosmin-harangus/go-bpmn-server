package builtin_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cosmin-harangus/go-bpmn-server/server/auth"
	"github.com/cosmin-harangus/go-bpmn-server/server/auth/builtin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const testSecret = "test-secret-key"

// fakeStore is a stub Store for unit tests.
type fakeStore struct {
	user   *builtin.User
	apiKey *builtin.APIKey
	err    error
}

func (f *fakeStore) GetUserByEmail(_ context.Context, _ string) (*builtin.User, error) {
	return f.user, f.err
}
func (f *fakeStore) GetAPIKeyByHash(_ context.Context, _ string) (*builtin.APIKey, error) {
	return f.apiKey, f.err
}

func newAuth(store builtin.Store) *builtin.Authenticator {
	return builtin.New(store, testSecret)
}

func signedJWT(t *testing.T, tenantID, userID string, secret string, expiry time.Time) string {
	t.Helper()
	claims := jwt.MapClaims{
		"tenant_id": tenantID,
		"user_id":   userID,
		"exp":       expiry.Unix(),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := tok.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// --- Middleware tests ---

func TestMiddleware_RejectsRequestWithNoAuthHeader(t *testing.T) {
	a := newAuth(&fakeStore{})
	h := a.Middleware()(okHandler())

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != 401 {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestMiddleware_RejectsInvalidJWT(t *testing.T) {
	a := newAuth(&fakeStore{})
	h := a.Middleware()(okHandler())

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-valid-jwt")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != 401 {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestMiddleware_RejectsExpiredJWT(t *testing.T) {
	a := newAuth(&fakeStore{})
	h := a.Middleware()(okHandler())

	tok := signedJWT(t, "tenant-1", "user-1", testSecret, time.Now().Add(-time.Hour))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != 401 {
		t.Errorf("expected 401 for expired JWT, got %d", rr.Code)
	}
}

func TestMiddleware_RejectsJWTSignedWithWrongSecret(t *testing.T) {
	a := newAuth(&fakeStore{})
	h := a.Middleware()(okHandler())

	tok := signedJWT(t, "tenant-1", "user-1", "wrong-secret", time.Now().Add(time.Hour))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != 401 {
		t.Errorf("expected 401 for wrong-secret JWT, got %d", rr.Code)
	}
}

func TestMiddleware_AcceptsValidJWT_InjectsIdentity(t *testing.T) {
	a := newAuth(&fakeStore{})

	tok := signedJWT(t, "tenant-1", "user-1", testSecret, time.Now().Add(time.Hour))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rr := httptest.NewRecorder()

	var got auth.Identity
	h := a.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = auth.FromContext(r.Context())
		w.WriteHeader(200)
	}))
	h.ServeHTTP(rr, req)

	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if got.TenantID != "tenant-1" {
		t.Errorf("expected TenantID=tenant-1, got %q", got.TenantID)
	}
	if got.UserID != "user-1" {
		t.Errorf("expected UserID=user-1, got %q", got.UserID)
	}
}

func TestMiddleware_RejectsAPIKeyNotFoundInStore(t *testing.T) {
	a := newAuth(&fakeStore{apiKey: nil, err: errors.New("not found")})
	h := a.Middleware()(okHandler())

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer apk_unknownkey")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != 401 {
		t.Errorf("expected 401 for unknown API key, got %d", rr.Code)
	}
}

func TestMiddleware_AcceptsValidAPIKey_InjectsTenantID(t *testing.T) {
	store := &fakeStore{apiKey: &builtin.APIKey{TenantID: "tenant-2"}}
	a := newAuth(store)

	var got auth.Identity
	h := a.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = auth.FromContext(r.Context())
		w.WriteHeader(200)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer apk_somevalidkey")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if got.TenantID != "tenant-2" {
		t.Errorf("expected TenantID=tenant-2, got %q", got.TenantID)
	}
}

// --- Login handler tests ---

func TestLogin_ValidCredentials_ReturnsJWT(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("secret123"), bcrypt.MinCost)
	store := &fakeStore{user: &builtin.User{
		ID: "user-1", TenantID: "tenant-1", Email: "alice@example.com",
		PasswordHash: string(hash),
	}}
	a := newAuth(store)

	body, _ := json.Marshal(map[string]string{"email": "alice@example.com", "password": "secret123"})
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	a.LoginHandler()(rr, req)

	if rr.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", rr.Code, rr.Body.String())
	}
	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["token"] == "" {
		t.Fatal("expected token in response")
	}

	// Token must be a valid JWT with correct claims.
	tok, err := jwt.Parse(resp["token"], func(t *jwt.Token) (any, error) {
		return []byte(testSecret), nil
	})
	if err != nil || !tok.Valid {
		t.Fatalf("returned token is invalid: %v", err)
	}
	claims := tok.Claims.(jwt.MapClaims)
	if claims["tenant_id"] != "tenant-1" {
		t.Errorf("expected tenant_id=tenant-1, got %v", claims["tenant_id"])
	}
}

func TestLogin_WrongPassword_Returns401(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct"), bcrypt.MinCost)
	store := &fakeStore{user: &builtin.User{
		ID: "user-1", TenantID: "tenant-1", Email: "alice@example.com",
		PasswordHash: string(hash),
	}}
	a := newAuth(store)

	body, _ := json.Marshal(map[string]string{"email": "alice@example.com", "password": "wrong"})
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	a.LoginHandler()(rr, req)

	if rr.Code != 401 {
		t.Errorf("expected 401 for wrong password, got %d", rr.Code)
	}
}

func TestLogin_UnknownEmail_Returns401(t *testing.T) {
	store := &fakeStore{user: nil, err: errors.New("not found")}
	a := newAuth(store)

	body, _ := json.Marshal(map[string]string{"email": "nobody@example.com", "password": "x"})
	req := httptest.NewRequest("POST", "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	a.LoginHandler()(rr, req)

	if rr.Code != 401 {
		t.Errorf("expected 401 for unknown email, got %d", rr.Code)
	}
}

// okHandler is a trivial 200 OK handler used in middleware tests.
func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	})
}
