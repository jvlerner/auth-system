package auth

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// Config holds the JWT configuration
type Config struct {
	PublicKeyPath string
}

// CustomClaims represents our JWT schema payload
type CustomClaims struct {
	Email string   `json:"email"`
	Roles []string `json:"roles"`
	jwt.RegisteredClaims
}

// JWTGuard is a middleware factory that uses an RSA Public Key to validate Bearer tokens
func JWTGuard(cfg Config, logger *zap.Logger) (func(http.Handler) http.Handler, error) {
	// Parse Public Key once during server initialization
	pubKeyBytes, err := os.ReadFile(cfg.PublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key %s: %w", cfg.PublicKeyPath, err)
	}

	pubKey, err := jwt.ParseRSAPublicKeyFromPEM(pubKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSA public key: %w", err)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenString := extractTokenFromHeader(r)
			if tokenString == "" {
				http.Error(w, "Unauthorized: missing bearer token", http.StatusUnauthorized)
				return
			}

			claims, err := parseAndValidateToken(tokenString, pubKey)
			if err != nil {
				logger.Warn("Unauthorized access attempt", zap.Error(err), zap.String("remote_ip", r.RemoteAddr))
				http.Error(w, fmt.Sprintf("Unauthorized: %s", err.Error()), http.StatusUnauthorized)
				return
			}

			// Token is valid. Bind claims to the request context.
			userClaims := UserClaims{
				Subject: claims.Subject,
				Email:   claims.Email,
				Roles:   claims.Roles,
			}
			ctx := WithUser(r.Context(), userClaims)

			// Continue down the chain
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}, nil
}

func extractTokenFromHeader(r *http.Request) string {
	bearerToken := r.Header.Get("Authorization")
	parts := strings.Split(bearerToken, " ")
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		return parts[1]
	}
	return ""
}

func parseAndValidateToken(tokenString string, pubKey *rsa.PublicKey) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		// ALGO EXPLICIT CHECK (Anti-Spoofing):
		// Forcefully reject any token that doesn't use the RS256 algorithm.
		// This prevents "none" alg attacks or HS256 symmetric key substitutions.
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return pubKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, errors.New("token expired")
		}
		return nil, errors.New("invalid token signature or format")
	}

	claims, ok := token.Claims.(*CustomClaims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}

// RequireRoles is a declarative middleware that enforces RBAC.
// It MUST be used AFTER JWTGuard in the Chi router chain.
// It allows access if the user has AT LEAST ONE of the specified allowedRoles.
func RequireRoles(allowedRoles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := GetUser(r.Context())
			if !ok {
				// Failsafe: Se JWTGuard não rodou, bloqueamos.
				http.Error(w, "Unauthorized: User context missing", http.StatusUnauthorized)
				return
			}

			hasAccess := false
			for _, userRole := range claims.Roles {
				for _, allowed := range allowedRoles {
					if userRole == allowed {
						hasAccess = true
						break
					}
				}
				if hasAccess {
					break
				}
			}

			if !hasAccess {
				http.Error(w, "Forbidden: Insufficient permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
