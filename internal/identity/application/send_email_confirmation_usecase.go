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

type SendEmailConfirmationUseCase struct {
	redisClient *redis.Client
	logger      *zap.Logger
}

func NewSendEmailConfirmationUseCase(redisClient *redis.Client, logger *zap.Logger) *SendEmailConfirmationUseCase {
	return &SendEmailConfirmationUseCase{
		redisClient: redisClient,
		logger:      logger,
	}
}

// UserRegisteredEvent define a estrutura do evento que vem do outbox/rabbitmq.
type UserRegisteredEvent struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

func (uc *SendEmailConfirmationUseCase) HandleCommand(ctx context.Context, payload []byte) error {
	var event UserRegisteredEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		uc.logger.Error("Failed to unmarshal user registered event", zap.Error(err))
		return err
	}

	token := uuid.New().String()
	key := fmt.Sprintf("email_confirmation:%s", token)

	err := uc.redisClient.Set(ctx, key, event.ID, 24*time.Hour).Err()
	if err != nil {
		uc.logger.Error("Failed to save confirmation token in Redis", zap.Error(err))
		return err
	}

	// Mocking email sending using Resend/MailHog
	uc.logger.Info("Mock Email Sent: Please confirm your email address", 
		zap.String("to", event.Email),
		zap.String("token", token),
		zap.String("link", fmt.Sprintf("http://localhost/api/v1/commands/confirm-email?token=%s", token)),
	)

	return nil
}
