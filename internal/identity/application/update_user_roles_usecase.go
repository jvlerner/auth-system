package application

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jvlerner/auth-system/internal/identity/domain"
)

// UpdateUserRolesCommand é o DTO para adicionar/remover roles.
type UpdateUserRolesCommand struct {
	UserID string   `json:"user_id"`
	Roles  []string `json:"roles"`
}

type UpdateUserRolesUseCase struct {
	repo   domain.UserRepository
	outbox domain.OutboxRepository
}

func NewUpdateUserRolesUseCase(repo domain.UserRepository, outbox domain.OutboxRepository) *UpdateUserRolesUseCase {
	return &UpdateUserRolesUseCase{
		repo:   repo,
		outbox: outbox,
	}
}

func (uc *UpdateUserRolesUseCase) Execute(ctx context.Context, cmd UpdateUserRolesCommand) error {
	// Fail-fast: Verificar se o usuário existe antes de enfileirar comando inútil
	user, err := uc.repo.FindByID(ctx, cmd.UserID)
	if err != nil {
		return err
	}
	if user == nil {
		return domain.ErrUserNotFound
	}

	payload, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	// Gera o comando no Outbox
	cmdID := uuid.New().String()
	err = uc.outbox.SaveCommand(ctx, cmdID, "command", "user.update_roles", payload)
	if err != nil {
		return err
	}

	return nil
}
