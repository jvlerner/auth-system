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
	ErrInvalidResetToken = errors.New("invalid or expired password reset token")
)

type ResetPasswordCommand struct {
	Token       string `json:"token"`
	NewPassword string `json:"new_password"`
}

type ResetPasswordUseCase struct {
	repo        domain.UserRepository
	hasher      domain.PasswordHasher
	redisClient *redis.Client
	logger      *zap.Logger
}

func NewResetPasswordUseCase(
	repo domain.UserRepository, 
	hasher domain.PasswordHasher, 
	redisClient *redis.Client, 
	logger *zap.Logger,
) *ResetPasswordUseCase {
	return &ResetPasswordUseCase{
		repo:        repo,
		hasher:      hasher,
		redisClient: redisClient,
		logger:      logger,
	}
}

func (uc *ResetPasswordUseCase) Execute(ctx context.Context, cmd ResetPasswordCommand) error {
	key := fmt.Sprintf("password_reset:%s", cmd.Token)
	
	// Recuperar UserID do Redis
	userID, err := uc.redisClient.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return ErrInvalidResetToken
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

	// Call password hasher gRPC to hash the new password asynchronously
	newPassword, err := uc.hasher.Hash(ctx, cmd.NewPassword)
	if err != nil {
		uc.logger.Error("Failed to hash new password", zap.Error(err))
		return err
	}

	// Atualizar senha no Domínio e BD
	user.UpdatePassword(newPassword)
	
	err = uc.repo.Update(ctx, user)
	if err != nil {
		return err
	}

	// Deletar o token após sucesso para evitar reuso (Burn-after-reading)
	uc.redisClient.Del(ctx, key)

	uc.logger.Info("Password reset successfully", zap.String("user_id", userID))
	return nil
}
