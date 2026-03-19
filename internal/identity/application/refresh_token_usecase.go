package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jvlerner/auth-system/internal/identity/domain"
	"github.com/redis/go-redis/v9"
)

var (
	ErrInvalidRefreshToken = errors.New("invalid or expired refresh token")
)

type RefreshTokenCommand struct {
	RefreshToken string `json:"refresh_token"`
}

type RefreshTokenResponseDTO struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type RefreshTokenUseCase struct {
	redisClient *redis.Client
	repo        domain.UserRepository
	tokenClient domain.TokenGenerator
}

func NewRefreshTokenUseCase(
	redisClient *redis.Client,
	repo domain.UserRepository,
	tokenClient domain.TokenGenerator,
) *RefreshTokenUseCase {
	return &RefreshTokenUseCase{
		redisClient: redisClient,
		repo:        repo,
		tokenClient: tokenClient,
	}
}

func (uc *RefreshTokenUseCase) Execute(ctx context.Context, cmd RefreshTokenCommand) (*RefreshTokenResponseDTO, error) {
	oldKey := fmt.Sprintf("refresh_token:%s", cmd.RefreshToken)

	// 1. Validar se token existe e extrair userID
	userID, err := uc.redisClient.Get(ctx, oldKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrInvalidRefreshToken
		}
		return nil, err
	}

	// 2. Buscar o usuário atual (para carregar E-mail e Roles novos)
	user, err := uc.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, domain.ErrUserNotFound
	}

	// 3. Gerar novo Access Token
	token, err := uc.tokenClient.Generate(ctx, user.ID().String(), user.Email(), user.Roles())
	if err != nil {
		return nil, err
	}

	// 4. Estratégia de Rotação (Sliding Window): Gerar novo Refresh Token e apagar antigo
	newRefreshToken := uuid.New().String()
	newKey := fmt.Sprintf("refresh_token:%s", newRefreshToken)

	pipe := uc.redisClient.Pipeline()
	pipe.Set(ctx, newKey, userID, 7*24*time.Hour)
	pipe.Del(ctx, oldKey)
	_, err = pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}

	return &RefreshTokenResponseDTO{
		AccessToken:  token.AccessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    token.ExpiresIn,
	}, nil
}
