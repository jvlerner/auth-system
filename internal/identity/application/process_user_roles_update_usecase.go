package application

import (
	"context"
	"encoding/json"

	"github.com/jvlerner/auth-system/internal/identity/domain"
	"go.uber.org/zap"
)

// ProcessUpdateUserRolesUseCase atualiza o banco real consumindo a mensageria.
type ProcessUpdateUserRolesUseCase struct {
	repo   domain.UserRepository
	logger *zap.Logger
}

func NewProcessUpdateUserRolesUseCase(repo domain.UserRepository, logger *zap.Logger) *ProcessUpdateUserRolesUseCase {
	return &ProcessUpdateUserRolesUseCase{
		repo:   repo,
		logger: logger,
	}
}

func (uc *ProcessUpdateUserRolesUseCase) HandleCommand(ctx context.Context, payload []byte) error {
	var cmd UpdateUserRolesCommand
	if err := json.Unmarshal(payload, &cmd); err != nil {
		uc.logger.Error("Failed to unmarshal update roles command", zap.Error(err))
		return err
	}

	// Restaurar Entidade atualizada
	user, err := uc.repo.FindByID(ctx, cmd.UserID)
	if err != nil {
		uc.logger.Warn("Worker failed to find user for roles update", zap.Error(err))
		return err
	}

	// Limpar e assentar as novas roles
	// Para caso simples, sobrescreve o slice (substituição inteira do state)
	// Em um sistema real, poderíamos ter Add/Remove explícitos
	user.RemoveRole("user") // Reset (exemplo) simplificado pelo repo update custom:
	
	err = uc.repo.UpdateRoles(ctx, user.ID().String(), cmd.Roles)
	if err != nil {
		uc.logger.Error("Failed to update user roles in database", zap.Error(err))
		return err
	}

	uc.logger.Info("User roles updated successfully by Worker", zap.String("user_id", user.ID().String()), zap.Any("new_roles", cmd.Roles))
	return nil
}
