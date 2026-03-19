package application

import (
	"context"

	"github.com/jvlerner/auth-system/internal/identity/domain"
	"go.uber.org/zap"
)

type ReadRepository interface {
	FindByID(ctx context.Context, id string) (*domain.User, error)
	SyncUser(ctx context.Context, user *domain.User) error
}

type GetUserProfileQuery struct {
	ID string `json:"id"`
	// Outros filtros poderiam entrar aqui
}

// UserProfileDTO restringe os dados que voltam pro Frontend (Ex: nunca retornar hash de senha)
type UserProfileDTO struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

type ProfileCache interface {
	GetProfile(ctx context.Context, id string) (*UserProfileDTO, error)
	SetProfile(ctx context.Context, profile *UserProfileDTO) error
}

type GetUserProfileUseCase struct {
	// Importante: No CQRS, as queries podem ler direto da Infra/View.
	// Não precisamos necessariamente carregar um Aggregate Root pesadíssimo do Domínio.
	readRepo ReadRepository
	cache    ProfileCache
	logger   *zap.Logger
}

func NewGetUserProfileUseCase(readRepo ReadRepository, cache ProfileCache, logger *zap.Logger) *GetUserProfileUseCase {
	return &GetUserProfileUseCase{
		readRepo: readRepo,
		cache:    cache,
		logger:   logger,
	}
}

func (uc *GetUserProfileUseCase) Execute(ctx context.Context, query GetUserProfileQuery) (*UserProfileDTO, error) {
	// 0. Tenta buscar do Cache primeiro
	cached, err := uc.cache.GetProfile(ctx, query.ID)
	if err == nil && cached != nil {
		uc.logger.Debug("Profile served from cache", zap.String("id", query.ID))
		return cached, nil
	}

	// 1. Busca direto do banco formatado para leitura
	user, err := uc.readRepo.FindByID(ctx, query.ID)
	if err != nil {
		if err == domain.ErrUserNotFound {
			return nil, err // Repassamos pro handler tratar como 404
		}
		uc.logger.Error("Error querying user profile", zap.Error(err), zap.String("id", query.ID))
		return nil, err
	}

	// 2. Transforma em DTO seguro
	dto := &UserProfileDTO{
		ID:        user.ID().String(),
		Email:     user.Email(),
		CreatedAt: user.CreatedAt().Format("2006-01-02T15:04:05Z07:00"),
	}

	// 3. Atualiza o Cache (fire and forget)
	go func() {
		// Nova prop pro Context por que o pai vai morrer com a requisição
		uc.cache.SetProfile(context.Background(), dto)
	}()

	return dto, nil
}
