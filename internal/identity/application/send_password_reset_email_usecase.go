package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type SendPasswordResetEmailUseCase struct {
	redisClient *redis.Client
	logger      *zap.Logger
}

func NewSendPasswordResetEmailUseCase(redisClient *redis.Client, logger *zap.Logger) *SendPasswordResetEmailUseCase {
	return &SendPasswordResetEmailUseCase{
		redisClient: redisClient,
		logger:      logger,
	}
}

func (uc *SendPasswordResetEmailUseCase) HandleCommand(ctx context.Context, payload []byte) error {
	var event UserForgotPasswordEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		uc.logger.Error("Failed to unmarshal user forgot password event", zap.Error(err))
		return err
	}

	token := uuid.New().String()
	key := fmt.Sprintf("password_reset:%s", token)

	// Magic link token alive for 15 minutes
	err := uc.redisClient.Set(ctx, key, event.ID, 15*time.Minute).Err()
	if err != nil {
		uc.logger.Error("Failed to save password reset token in Redis", zap.Error(err))
		return err
	}

	// Mocking email sending using Resend/MailHog
	uc.logger.Info("Mock Email Sent: Password Reset Request", 
		zap.String("to", event.Email),
		zap.String("token", token),
		zap.String("link", fmt.Sprintf("http://localhost/api/v1/commands/reset-password?token=%s", token)),
	)

	return nil
}
