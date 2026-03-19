package application

import (
	"crypto/rsa"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type TokenService struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

func NewTokenService() (*TokenService, error) {
	privBytes, err := os.ReadFile("configs/rsa/private.pem")
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}
	privKey, err := jwt.ParseRSAPrivateKeyFromPEM(privBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	pubBytes, err := os.ReadFile("configs/rsa/public.pem")
	if err != nil {
		return nil, fmt.Errorf("failed to read public key: %w", err)
	}
	pubKey, err := jwt.ParseRSAPublicKeyFromPEM(pubBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	return &TokenService{
		privateKey: privKey,
		publicKey:  pubKey,
	}, nil
}

func (s *TokenService) GenerateToken(userID, email string, roles []string) (string, int64, error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := jwt.MapClaims{
		"sub":   userID,
		"email": email,
		"roles": roles,
		"exp":   expirationTime.Unix(),
		"iat":   time.Now().Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenString, err := token.SignedString(s.privateKey)
	if err != nil {
		return "", 0, err
	}

	return tokenString, int64(expirationTime.Unix()), nil
}

func (s *TokenService) ValidateToken(tokenString string) (bool, string, string, []string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.publicKey, nil
	})

	if err != nil || !token.Valid {
		return false, "", "", nil, nil
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return false, "", "", nil, nil
	}

	userID, _ := claims["sub"].(string)
	email, _ := claims["email"].(string)

	var roles []string
	if rawRoles, ok := claims["roles"].([]interface{}); ok {
		for _, r := range rawRoles {
			if str, ok := r.(string); ok {
				roles = append(roles, str)
			}
		}
	}

	return true, userID, email, roles, nil
}
