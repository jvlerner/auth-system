package application

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jvlerner/auth-system/internal/identity/domain"
	"go.uber.org/zap"
)

type ForgotPasswordCommand struct {
	Email string `json:"email"`
}

type ForgotPasswordUseCase struct {
	repo   domain.UserRepository
	outbox domain.OutboxRepository
	logger *zap.Logger
}

func NewForgotPasswordUseCase(repo domain.UserRepository, outbox domain.OutboxRepository, logger *zap.Logger) *ForgotPasswordUseCase {
	return &ForgotPasswordUseCase{
		repo:   repo,
		outbox: outbox,
		logger: logger,
	}
}

type UserForgotPasswordEvent struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func (uc *ForgotPasswordUseCase) Execute(ctx context.Context, cmd ForgotPasswordCommand) error {
	email, err := domain.NewEmail(cmd.Email)
	if err != nil {
		return err
	}

	user, err := uc.repo.FindByEmail(ctx, email)
	if err != nil {
		return err
	}
	// Secure against User Enumeration: if user is not found, we just return nil silently.
	if user == nil {
		uc.logger.Warn("Forgot password requested for non-existent email", zap.String("email", cmd.Email))
		return nil
	}

	event := UserForgotPasswordEvent{
		ID:    user.ID().String(),
		Email: user.Email(),
	}
	payload, _ := json.Marshal(event)

	return uc.outbox.SaveCommand(ctx, uuid.New().String(), "user", "UserForgotPasswordRequested", payload)
}
