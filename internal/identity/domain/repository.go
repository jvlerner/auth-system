package domain

import (
	"context"
)

// UserRepository define operações de escrita e validação (Command Side).
type UserRepository interface {
	Save(ctx context.Context, user *User) error
	Update(ctx context.Context, user *User) error
	UpdateRoles(ctx context.Context, id string, roles []string) error
	FindByID(ctx context.Context, id string) (*User, error)
	FindByEmail(ctx context.Context, email Email) (*User, error)
}

// UserReadRepository define operações de consulta para visualização (Query Side).
type UserReadRepository interface {
	FindByID(ctx context.Context, id string) (*User, error)
	FindByEmail(ctx context.Context, email Email) (*User, error)
}

