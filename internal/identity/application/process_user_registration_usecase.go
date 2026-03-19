package application

import (
	"context"
	"encoding/json"

	"github.com/jvlerner/auth-system/internal/identity/domain"
	"go.uber.org/zap"
)

// ProcessUserRegistrationUseCase orquestra o registro real que foi enviado via fila.
type ProcessUserRegistrationUseCase struct {
	repo   domain.UserRepository
	hasher domain.PasswordHasher
	logger *zap.Logger
}

func NewProcessUserRegistrationUseCase(repo domain.UserRepository, hasher domain.PasswordHasher, logger *zap.Logger) *ProcessUserRegistrationUseCase {
	return &ProcessUserRegistrationUseCase{
		repo:   repo,
		hasher: hasher,
		logger: logger,
	}
}

func (uc *ProcessUserRegistrationUseCase) HandleCommand(ctx context.Context, payload []byte) error {
	var cmd RegisterUserCommand
	if err := json.Unmarshal(payload, &cmd); err != nil {
		uc.logger.Error("Failed to unmarshal register command", zap.Error(err))
		return err
	}

	email, err := domain.NewEmail(cmd.Email)
	if err != nil {
		uc.logger.Warn("Worker skipped invalid email", zap.Error(err), zap.String("email", cmd.Email))
		return nil // Se o dado é inválido no worker, dar "Acknowledge" (return nil) para dropar, senão vira poison message
	}

	existingUser, err := uc.repo.FindByEmail(ctx, email)
	if err != nil {
		return err
	}
	if existingUser != nil {
		// Evento duplicado ou problema de sincronicidade: dropar silenciosamente.
		uc.logger.Warn("Worker skip registration, user already exists", zap.String("email", cmd.Email))
		return nil
	}

	hashedPassword, err := uc.hasher.Hash(ctx, cmd.Password)
	if err != nil {
		uc.logger.Error("Failed to hash password via gRPC", zap.Error(err))
		return err // Retorna erro pra mensagem voltar pra fila e ser tentada novamente (NACK)
	}

	user := domain.NewUser(email, hashedPassword)
	// Como padrão o NewUser já injeta a rule 'user'

	if err := uc.repo.Save(ctx, user); err != nil {
		uc.logger.Error("Failed to save user in database", zap.Error(err))
		return err
	}

	uc.logger.Info("User processed and saved successfully by Worker", zap.String("user_id", user.ID().String()), zap.String("email", user.Email()))
	return nil
}
