package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jvlerner/auth-system/internal/identity/domain"
	"github.com/redis/go-redis/v9"
)

var (
	ErrInvalidLogin = errors.New("invalid email or password")
)

type LoginRequestDTO struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type UserDTO struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type LoginResponseDTO struct {
	AccessToken  string  `json:"access_token,omitempty"`
	RefreshToken string  `json:"refresh_token,omitempty"`
	ExpiresIn    int64   `json:"expires_in,omitempty"`
	MFARequired  bool    `json:"mfa_required"`
	MFAToken     string  `json:"mfa_token,omitempty"`
	User         UserDTO `json:"user"`
}

type LoginUserUseCase struct {
	repo           domain.UserRepository
	passwordHasher domain.PasswordHasher
	tokenClient    domain.TokenGenerator
	outboxRepo     domain.OutboxRepository
	redisClient    *redis.Client
}

func NewLoginUserUseCase(
	repo domain.UserRepository,
	passwordHasher domain.PasswordHasher,
	tokenClient domain.TokenGenerator,
	outboxRepo domain.OutboxRepository,
	redisClient *redis.Client,
) *LoginUserUseCase {
	return &LoginUserUseCase{
		repo:           repo,
		passwordHasher: passwordHasher,
		tokenClient:    tokenClient,
		outboxRepo:     outboxRepo,
		redisClient:    redisClient,
	}
}

func (uc *LoginUserUseCase) Execute(ctx context.Context, req LoginRequestDTO) (*LoginResponseDTO, error) {
	email, err := domain.NewEmail(req.Email)
	if err != nil {
		return nil, ErrInvalidLogin
	}

	user, err := uc.repo.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrInvalidLogin
	}

	restoredPassword, err := domain.RestorePassword(user.HashedPassword())
	if err != nil {
		return nil, ErrInvalidLogin
	}

	needsUpgrade, err := uc.passwordHasher.Compare(ctx, req.Password, restoredPassword)
	if err != nil {
		return nil, ErrInvalidLogin
	}

	// Upgrade transparente: o hash está em versão legada — recomputar com V2 e persistir
	// via Outbox. Nunca enviamos plaintext pela fila: apenas o novo hash seguro.
	if needsUpgrade {
		newHash, hashErr := uc.passwordHasher.Hash(ctx, req.Password)
		if hashErr == nil {
			type upgradePayload struct {
				UserID  string `json:"user_id"`
				NewHash string `json:"new_hash"`
			}
			payload, _ := json.Marshal(upgradePayload{
				UserID:  user.ID().String(),
				NewHash: newHash.Value(),
			})
			_ = uc.outboxRepo.SaveCommand(ctx, user.ID().String(), "User", "UserPasswordUpgradeRequested", payload)
		}
	}

	// MFA Check
	if user.IsMFAEnabled() {
		mfaToken := uuid.New().String()
		mfaKey := fmt.Sprintf("mfa_token:%s", mfaToken)
		// Armazena o UserID no Redis por 5 minutos para a segunda fase (Verify)
		if err = uc.redisClient.Set(ctx, mfaKey, user.ID().String(), 5*time.Minute).Err(); err != nil {
			return nil, err
		}

		return &LoginResponseDTO{
			MFARequired: true,
			MFAToken:    mfaToken,
			User: UserDTO{
				ID:    user.ID().String(),
				Email: user.Email(),
			},
		}, nil
	}

	token, err := uc.tokenClient.Generate(ctx, user.ID().String(), user.Email(), user.Roles())
	if err != nil {
		return nil, err
	}

	// Gera Refresh Token Opaco (UUID) e salva no Redis (7 dias de TTL)
	refreshToken := uuid.New().String()
	key := fmt.Sprintf("refresh_token:%s", refreshToken)
	if err = uc.redisClient.Set(ctx, key, user.ID().String(), 7*24*time.Hour).Err(); err != nil {
		return nil, err
	}

	res := &LoginResponseDTO{
		AccessToken:  token.AccessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    token.ExpiresIn,
	}
	res.User.ID = user.ID().String()
	res.User.Email = user.Email()

	return res, nil
}
