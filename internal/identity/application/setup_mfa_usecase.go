package application

import (
	"context"

	"github.com/jvlerner/auth-system/internal/identity/domain"
	"github.com/pquerna/otp/totp"
	"go.uber.org/zap"
)

type SetupMFAResponse struct {
	Secret    string `json:"secret"`
	QRCodeURL string `json:"qr_code_url"`
}

type SetupMFAUseCase struct {
	repo   domain.UserRepository
	logger *zap.Logger
}

func NewSetupMFAUseCase(repo domain.UserRepository, logger *zap.Logger) *SetupMFAUseCase {
	return &SetupMFAUseCase{
		repo:   repo,
		logger: logger,
	}
}

func (uc *SetupMFAUseCase) Execute(ctx context.Context, userID string) (*SetupMFAResponse, error) {
	user, err := uc.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, domain.ErrUserNotFound
	}

	// 1. Gerar Secret TOTP
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "AuthSystem",
		AccountName: user.Email(),
	})
	if err != nil {
		uc.logger.Error("Failed to generate TOTP key", zap.Error(err), zap.String("user_id", userID))
		return nil, err
	}

	// 2. Salvar secret no usuário (mas mfa_enabled ainda é false até confirmar)
	user.EnableMFA(key.Secret()) // Por enquanto EnableMFA já seta mfa_enabled=true no domínio
	// TODO: Talvez separar 'PrepareMFA' de 'EnableMFA' para evitar lockout precoce.
	// Por simplicidade do MVP, vamos assumir que o usuário só habilita se for usar.
	
	if err := uc.repo.Update(ctx, user); err != nil {
		return nil, err
	}

	return &SetupMFAResponse{
		Secret:    key.Secret(),
		QRCodeURL: key.URL(),
	}, nil
}
