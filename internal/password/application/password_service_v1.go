package application

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// PasswordServiceV1 implementa HashService com os parâmetros Argon2id atuais (OWASP recomendados).
// A string de hash gerada sempre terá o prefixo "v1:" para permitir roteamento no gRPC.
type PasswordServiceV1 struct {
	time    uint32
	memory  uint32
	threads uint8
	keyLen  uint32
}

func NewPasswordServiceV1() *PasswordServiceV1 {
	return &PasswordServiceV1{
		time:    1,
		memory:  64 * 1024, // 64 MB
		threads: 4,
		keyLen:  32,
	}
}

func (s *PasswordServiceV1) Version() string {
	return "v1"
}

func (s *PasswordServiceV1) Hash(ctx context.Context, password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	hash := argon2.IDKey([]byte(password), salt, s.time, s.memory, s.threads, s.keyLen)

	b64Salt := base64.RawStdEncoding.EncodeToString(salt)
	b64Hash := base64.RawStdEncoding.EncodeToString(hash)

	// Formato: v1:$argon2id$v=19$m=65536,t=1,p=4$<salt>$<hash>
	encoded := fmt.Sprintf("v1:$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, s.memory, s.time, s.threads, b64Salt, b64Hash)

	return encoded, nil
}

func (s *PasswordServiceV1) Compare(ctx context.Context, plainText, encodedHash string) (bool, error) {
	// Remove o prefixo de versão antes de parsear
	phc := strings.TrimPrefix(encodedHash, "v1:")

	parts := strings.Split(phc, "$")
	if len(parts) != 6 {
		return false, errors.New("formato de hash inválido para v1")
	}
	if parts[1] != "argon2id" {
		return false, errors.New("algoritmo incompatível")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, err
	}

	var memory uint32
	var t uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &t, &threads); err != nil {
		return false, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}

	targetHash, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}

	comparisonHash := argon2.IDKey([]byte(plainText), salt, t, memory, threads, uint32(len(targetHash)))

	if subtle.ConstantTimeCompare(targetHash, comparisonHash) == 1 {
		return true, nil
	}
	return false, nil
}
