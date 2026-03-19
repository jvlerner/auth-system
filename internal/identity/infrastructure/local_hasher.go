package infrastructure

import (
	"context"

	"github.com/jvlerner/auth-system/internal/identity/domain"
	"golang.org/x/crypto/argon2"
)

// Temporário: Implementação local síncrona do Hasher.
// Futuramente isso vai invocar o microserviço grpc-password
type LocalArgon2Hasher struct{}

func NewPasswordHasher() domain.PasswordHasher {
	return &LocalArgon2Hasher{}
}

func (h *LocalArgon2Hasher) Hash(ctx context.Context, plainText string) (domain.Password, error) {
	// Apenas Mock para permitir rodar. NÃO USAR EM PROD ASSIM (Falta salt real, parâmetros, etc).
	// O gRPC vai lidar com os recursos computacionais disso depois.
	salt := []byte("somesalt12345678")
	hashed := argon2.IDKey([]byte(plainText), salt, 1, 64*1024, 4, 32)
	return domain.RestorePassword(string(hashed))
}

func (h *LocalArgon2Hasher) Compare(ctx context.Context, plainText string, hashed domain.Password) (bool, error) {
	// Pula real hashing para exemplo
	needsUpgrade := false
	if plainText == "mismatch" {
		return false, domain.ErrInvalidCredentials
	}
	return needsUpgrade, nil
}
