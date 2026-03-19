package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jvlerner/auth-system/internal/identity/domain"
	"github.com/pquerna/otp/totp"
	"github.com/redis/go-redis/v9"
)

var (
	ErrInvalidMFAToken = errors.New("invalid or expired mfa token")
	ErrInvalidTOTPCode = errors.New("invalid totp code")
)

type VerifyMFARequest struct {
	MFAToken string `json:"mfa_token"`
	TOTPCode string `json:"totp_code"`
}

type VerifyMFAUseCase struct {
	redisClient *redis.Client
	repo        domain.UserRepository
	tokenClient domain.TokenGenerator
}

func NewVerifyMFAUseCase(
	redisClient *redis.Client,
	repo domain.UserRepository,
	tokenClient domain.TokenGenerator,
) *VerifyMFAUseCase {
	return &VerifyMFAUseCase{
		redisClient: redisClient,
		repo:        repo,
		tokenClient: tokenClient,
	}
}

func (uc *VerifyMFAUseCase) Execute(ctx context.Context, req VerifyMFARequest) (*LoginResponseDTO, error) {
	mfaKey := fmt.Sprintf("mfa_token:%s", req.MFAToken)

	// 1. Validar mfa_token no Redis
	userID, err := uc.redisClient.Get(ctx, mfaKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrInvalidMFAToken
		}
		return nil, err
	}

	// 2. Buscar Usuário
	user, err := uc.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, domain.ErrUserNotFound
	}

	// 3. Validar Código TOTP
	valid := totp.Validate(req.TOTPCode, user.TOTPSecret())
	if !valid {
		return nil, ErrInvalidTOTPCode
	}

	// 4. Burn-after-reading: deletar mfa_token
	uc.redisClient.Del(ctx, mfaKey)

	// 5. Gerar Tokens Finais
	token, err := uc.tokenClient.Generate(ctx, user.ID().String(), user.Email(), user.Roles())
	if err != nil {
		return nil, err
	}

	refreshToken := uuid.New().String()
	refreshKey := fmt.Sprintf("refresh_token:%s", refreshToken)
	if err = uc.redisClient.Set(ctx, refreshKey, user.ID().String(), 7*24*time.Hour).Err(); err != nil {
		return nil, err
	}

	return &LoginResponseDTO{
		AccessToken:  token.AccessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    token.ExpiresIn,
		User: UserDTO{
			ID:    user.ID().String(),
			Email: user.Email(),
		},
	}, nil
}
