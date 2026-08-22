package builtin

import "context"

// User is an authenticated user record.
type User struct {
	ID           string
	TenantID     string
	Email        string
	PasswordHash string
}

// APIKey is a tenant-scoped API key record.
type APIKey struct {
	TenantID string
}

// Store is the persistence interface for built-in auth.
type Store interface {
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetAPIKeyByHash(ctx context.Context, keyHash string) (*APIKey, error)
}
