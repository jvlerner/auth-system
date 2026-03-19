package auth

import (
	"context"
	"net/http"
)

type contextKey string

const (
	userContextKey contextKey = "auth_user_claims"
)

// UserClaims holds the authenticated user's verified identity
type UserClaims struct {
	Subject string   // Usually the User ID
	Email   string   // Email address
	Roles   []string // Access roles (e.g., "admin", "user")
}

// WithUser Injects the UserClaims into the Context
func WithUser(ctx context.Context, claims UserClaims) context.Context {
	return context.WithValue(ctx, userContextKey, claims)
}

// GetUser Extracts the UserClaims from the Context.
// Returns false if the user is not authenticated.
func GetUser(ctx context.Context) (UserClaims, bool) {
	claims, ok := ctx.Value(userContextKey).(UserClaims)
	return claims, ok
}

// MustGetUser extracts the UserClaims or panics (useful behind strict middlewares)
func MustGetUser(ctx context.Context) UserClaims {
	claims, ok := GetUser(ctx)
	if !ok {
		panic("auth: GetUser called in a context without a user. Make sure the handler is wrapped with JWTGuard.")
	}
	return claims
}

// HasRole checks if the authenticated user has a specific role.
// Always returns false if the user is not in the context.
func HasRole(ctx context.Context, requiredRole string) bool {
	claims, ok := GetUser(ctx)
	if !ok {
		return false
	}
	for _, role := range claims.Roles {
		if role == requiredRole {
			return true
		}
	}
	return false
}

// GetUserID Extracts the UserID (Subject) directly from the pure HTTP Request
func GetUserID(r *http.Request) (string, bool) {
	claims, ok := GetUser(r.Context())
	if !ok {
		return "", false
	}
	return claims.Subject, true
}
