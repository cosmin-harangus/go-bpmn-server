package auth

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Identity holds the authenticated caller's identity.
type Identity struct {
	UserID   string
	TenantID string
}

type contextKey struct{}

// WithIdentity stores the identity in the context.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

// FromContext retrieves the identity from the context.
// Returns zero value if not set.
func FromContext(ctx context.Context) Identity {
	id, _ := ctx.Value(contextKey{}).(Identity)
	return id
}

// Authenticator validates incoming requests and injects Identity into the context.
type Authenticator interface {
	// Middleware returns an HTTP middleware that authenticates requests.
	// Sets identity in context on success; writes 401 and stops the chain on failure.
	Middleware() func(http.Handler) http.Handler
	// PublicRoutes registers any unauthenticated routes needed by this authenticator
	// (e.g. a login endpoint). Called before the auth middleware is applied.
	PublicRoutes(r chi.Router)
}
