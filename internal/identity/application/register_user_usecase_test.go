package application

import (
	"context"
	"testing"

	"github.com/jvlerner/auth-system/internal/identity/domain"
)

// --- Mocks ---

type mockUserRepository struct {
	saveFn        func(ctx context.Context, user *domain.User) error
	findByEmailFn func(ctx context.Context, email domain.Email) (*domain.User, error)
	findByIDFn    func(ctx context.Context, id string) (*domain.User, error)
	updateFn      func(ctx context.Context, user *domain.User) error
	updateRolesFn func(ctx context.Context, id string, roles []string) error
}

func (m *mockUserRepository) Save(ctx context.Context, user *domain.User) error {
	if m.saveFn != nil {
		return m.saveFn(ctx, user)
	}
	return nil
}
func (m *mockUserRepository) FindByEmail(ctx context.Context, email domain.Email) (*domain.User, error) {
	if m.findByEmailFn != nil {
		return m.findByEmailFn(ctx, email)
	}
	return nil, nil
}
func (m *mockUserRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	if m.findByIDFn != nil {
		return m.findByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *mockUserRepository) Update(ctx context.Context, user *domain.User) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, user)
	}
	return nil
}

func (m *mockUserRepository) UpdateRoles(ctx context.Context, id string, roles []string) error {
	if m.updateRolesFn != nil {
		return m.updateRolesFn(ctx, id, roles)
	}
	return nil
}

type mockOutboxRepository struct {
	saveCmdFn func(ctx context.Context, aggregateID string, aggregateType string, eventType string, payload []byte) error
}

func (m *mockOutboxRepository) SaveCommand(ctx context.Context, aggregateID string, aggregateType string, eventType string, payload []byte) error {
	if m.saveCmdFn != nil {
		return m.saveCmdFn(ctx, aggregateID, aggregateType, eventType, payload)
	}
	return nil
}

// --- Tests ---

func TestRegisterUserUseCase_Success(t *testing.T) {
	mockRepo := &mockUserRepository{
		findByEmailFn: func(ctx context.Context, email domain.Email) (*domain.User, error) {
			return nil, nil // User não existe
		},
	}
	mockOutbox := &mockOutboxRepository{
		saveCmdFn: func(ctx context.Context, aggregateID string, aggregateType string, eventType string, payload []byte) error {
			return nil // Sucesso simulado
		},
	}

	uc := NewRegisterUserUseCase(mockRepo, mockOutbox)
	
	cmd := RegisterUserCommand{
		Email:    "test@example.com",
		Password: "password123",
	}
	
	err := uc.Execute(context.TODO(), cmd)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestRegisterUserUseCase_UserAlreadyExists(t *testing.T) {
	mockRepo := &mockUserRepository{
		findByEmailFn: func(ctx context.Context, email domain.Email) (*domain.User, error) {
			password, _ := domain.RestorePassword("existing")
			return domain.NewUser(email, password), nil
		},
	}
	mockOutbox := &mockOutboxRepository{}

	uc := NewRegisterUserUseCase(mockRepo, mockOutbox)
	
	cmd := RegisterUserCommand{
		Email:    "test@example.com",
		Password: "password123",
	}
	
	err := uc.Execute(context.TODO(), cmd)
	if err != domain.ErrUserAlreadyExists {
		t.Errorf("expected ErrUserAlreadyExists, got %v", err)
	}
}

func TestRegisterUserUseCase_InvalidEmail(t *testing.T) {
	mockRepo := &mockUserRepository{}
	mockOutbox := &mockOutboxRepository{}

	uc := NewRegisterUserUseCase(mockRepo, mockOutbox)
	
	cmd := RegisterUserCommand{
		Email:    "invalid-email",
		Password: "password123",
	}
	
	err := uc.Execute(context.TODO(), cmd)
	if err == nil {
		t.Error("expected error for invalid email, got nil")
	}
}
