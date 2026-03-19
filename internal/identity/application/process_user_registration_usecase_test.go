package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/jvlerner/auth-system/internal/identity/application"
	"github.com/jvlerner/auth-system/internal/identity/domain"
	"go.uber.org/zap"
)

// ─── Mocks estão no arquivo login_user_usecase_test.go ───────────────────────
// mockUserRepo, mockHasher, mockOutbox estão disponíveis em todos os _test.go
// do mesmo package application_test.

// ─── Mock Logger ────────────────────────────────────────────────────────────

func noopLogger() *zap.Logger {
	return zap.NewNop()
}

// ─── Testes: ProcessUserRegistrationUseCase ─────────────────────────────────

func TestProcessRegistration_PayloadInvalido_RetornaErro(t *testing.T) {
	uc := application.NewProcessUserRegistrationUseCase(
		newMockUserRepo(),
		&mockHasher{},
		noopLogger(),
	)

	err := uc.HandleCommand(context.Background(), []byte("invalido{{{"))
	if err == nil {
		t.Error("esperado erro de parsing, obtido nil")
	}
}

func TestProcessRegistration_EmailInvalido_RetornaNilEDescarta(t *testing.T) {
	uc := application.NewProcessUserRegistrationUseCase(
		newMockUserRepo(),
		&mockHasher{},
		noopLogger(),
	)

	payload, _ := json.Marshal(map[string]string{"email": "nao-email", "password": "qualquer"})
	err := uc.HandleCommand(context.Background(), payload)

	// Email inválido deve ser descartado silenciosamente (nil) para não virar poison message
	if err != nil {
		t.Errorf("esperado nil para email inválido, obtido: %v", err)
	}
}

func TestProcessRegistration_UsuarioJaExiste_DescartaSilenciosamente(t *testing.T) {
	user := buildUser(t)
	repo := newMockUserRepo(user)

	uc := application.NewProcessUserRegistrationUseCase(repo, &mockHasher{}, noopLogger())

	payload, _ := json.Marshal(map[string]string{
		"email":    "user@example.com",
		"password": "qualquer",
	})
	err := uc.HandleCommand(context.Background(), payload)

	if err != nil {
		t.Errorf("esperado nil (idempotência), obtido: %v", err)
	}
}

func TestProcessRegistration_FalhaNoHash_RetornaErro(t *testing.T) {
	uc := application.NewProcessUserRegistrationUseCase(
		newMockUserRepo(),
		&mockHasher{shouldFail: true},
		noopLogger(),
	)

	payload, _ := json.Marshal(map[string]string{
		"email":    "novo@example.com",
		"password": "qualquer",
	})
	err := uc.HandleCommand(context.Background(), payload)

	if err == nil {
		t.Error("esperado erro do hasher, obtido nil")
	}
}

func TestProcessRegistration_Sucesso_UsuarioSalvo(t *testing.T) {
	repo := &mockUserRepoSaveTracking{}

	uc := application.NewProcessUserRegistrationUseCase(repo, &mockHasher{}, noopLogger())

	payload, _ := json.Marshal(map[string]string{
		"email":    "novo@example.com",
		"password": "SenhaForte123!",
	})
	err := uc.HandleCommand(context.Background(), payload)

	if err != nil {
		t.Fatalf("esperado sucesso, obtido: %v", err)
	}
	if !repo.saved {
		t.Error("esperado que o usuário fosse salvo no repositório")
	}
}

// ─── Mock com tracking de Save ───────────────────────────────────────────────

type mockUserRepoSaveTracking struct {
	saved bool
}

func (m *mockUserRepoSaveTracking) Save(ctx context.Context, u *domain.User) error {
	m.saved = true
	return nil
}
func (m *mockUserRepoSaveTracking) FindByEmail(ctx context.Context, email domain.Email) (*domain.User, error) {
	return nil, nil // usuário não existe
}
func (m *mockUserRepoSaveTracking) FindByID(ctx context.Context, id string) (*domain.User, error)    { return nil, nil }
func (m *mockUserRepoSaveTracking) Update(ctx context.Context, u *domain.User) error                 { return nil }
func (m *mockUserRepoSaveTracking) UpdateRoles(ctx context.Context, id string, roles []string) error { return nil }

// ─── Testes: UpdateUserRolesUseCase ─────────────────────────────────────────

func TestUpdateRoles_UsuarioNaoEncontrado_RetornaErro(t *testing.T) {
	uc := application.NewUpdateUserRolesUseCase(newMockUserRepo(), &mockOutbox{})

	err := uc.Execute(context.Background(), application.UpdateUserRolesCommand{
		UserID: "id-que-nao-existe",
		Roles:  []string{"admin"},
	})

	if !errors.Is(err, domain.ErrUserNotFound) {
		t.Errorf("esperado ErrUserNotFound, obtido: %v", err)
	}
}

func TestUpdateRoles_Sucesso_EmiteComandoNoOutbox(t *testing.T) {
	repo := &mockUserRepoFindByID{user: buildUser(t)}
	outbox := &mockOutbox{}

	uc := application.NewUpdateUserRolesUseCase(repo, outbox)

	err := uc.Execute(context.Background(), application.UpdateUserRolesCommand{
		UserID: repo.user.ID().String(),
		Roles:  []string{"admin", "user"},
	})

	if err != nil {
		t.Fatalf("esperado sucesso, obtido: %v", err)
	}
	if len(outbox.saved) == 0 {
		t.Error("esperado evento no outbox, outbox está vazio")
	}
}

// ─── Mock repo com FindByID pré-populado ────────────────────────────────────

type mockUserRepoFindByID struct {
	user *domain.User
}

func (m *mockUserRepoFindByID) FindByID(ctx context.Context, id string) (*domain.User, error) {
	if m.user != nil && m.user.ID().String() == id {
		return m.user, nil
	}
	return nil, nil
}
func (m *mockUserRepoFindByID) FindByEmail(ctx context.Context, email domain.Email) (*domain.User, error) { return nil, nil }
func (m *mockUserRepoFindByID) Save(ctx context.Context, u *domain.User) error                           { return nil }
func (m *mockUserRepoFindByID) Update(ctx context.Context, u *domain.User) error                         { return nil }
func (m *mockUserRepoFindByID) UpdateRoles(ctx context.Context, id string, roles []string) error         { return nil }
