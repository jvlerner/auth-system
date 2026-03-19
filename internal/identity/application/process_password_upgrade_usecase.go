package application

import (
	"context"
	"encoding/json"

	"github.com/jvlerner/auth-system/internal/identity/domain"
)

// ProcessPasswordUpgradeCommand é o payload do evento de upgrade transparente de senha.
type ProcessPasswordUpgradeCommand struct {
	UserID  string `json:"user_id"`
	NewHash string `json:"new_hash"`
}

// ProcessPasswordUpgradeUseCase aplica os upgrades de hash transparentes
// a usuários que fizeram login com uma senha em versão legada.
type ProcessPasswordUpgradeUseCase struct {
	repo domain.UserRepository
}

func NewProcessPasswordUpgradeUseCase(repo domain.UserRepository) *ProcessPasswordUpgradeUseCase {
	return &ProcessPasswordUpgradeUseCase{repo: repo}
}

// HandleCommand é chamado pelo worker-events ao consumir UserPasswordUpgradeRequested.
func (uc *ProcessPasswordUpgradeUseCase) HandleCommand(ctx context.Context, data []byte) error {
	var cmd ProcessPasswordUpgradeCommand
	if err := json.Unmarshal(data, &cmd); err != nil {
		return err
	}

	user, err := uc.repo.FindByID(ctx, cmd.UserID)
	if err != nil {
		return err
	}
	if user == nil {
		return domain.ErrUserNotFound
	}

	newPassword, err := domain.RestorePassword(cmd.NewHash)
	if err != nil {
		return err
	}

	user.UpdatePassword(newPassword)

	return uc.repo.Update(ctx, user)
}
