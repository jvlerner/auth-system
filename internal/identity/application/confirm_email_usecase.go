package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/jvlerner/auth-system/internal/identity/domain"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var (
	ErrInvalidVerificationToken = errors.New("invalid or expired verification token")
)

type ConfirmEmailCommand struct {
	Token string `json:"token"`
}

type ConfirmEmailUseCase struct {
	repo        domain.UserRepository
	redisClient *redis.Client
	logger      *zap.Logger
}

func NewConfirmEmailUseCase(repo domain.UserRepository, redisClient *redis.Client, logger *zap.Logger) *ConfirmEmailUseCase {
	return &ConfirmEmailUseCase{
		repo:        repo,
		redisClient: redisClient,
		logger:      logger,
	}
}

func (uc *ConfirmEmailUseCase) Execute(ctx context.Context, cmd ConfirmEmailCommand) error {
	key := fmt.Sprintf("email_confirmation:%s", cmd.Token)
	
	// Recuperar UserID do Redis
	userID, err := uc.redisClient.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrInvalidVerificationToken
		}
		return err
	}

	// Buscar Usuário no BD
	user, err := uc.repo.FindByID(ctx, userID)
	if err != nil {
		return err
	}
	if user == nil {
		return domain.ErrUserNotFound
	}

	// Confirmar e Salvar
	user.ConfirmEmail()
	
	err = uc.repo.Update(ctx, user)
	if err != nil {
		return err
	}

	// Deletar o token
	uc.redisClient.Del(ctx, key)

	uc.logger.Info("Email confirmed successfully", zap.String("user_id", userID))
	return nil
}
