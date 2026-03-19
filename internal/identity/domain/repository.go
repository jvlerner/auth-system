package domain

import (
	"context"
)

// UserRepository é o "Port" da camada de Domínio.
// Define como a aplicação interage com o armazenamento do User.
// A implementação real (Adapter) ficará em infrastructure.
type UserRepository interface {
	Save(ctx context.Context, user *User) error
	FindByEmail(ctx context.Context, email Email) (*User, error)
	FindByID(ctx context.Context, id string) (*User, error)
	Update(ctx context.Context, user *User) error
	UpdateRoles(ctx context.Context, id string, roles []string) error
}
