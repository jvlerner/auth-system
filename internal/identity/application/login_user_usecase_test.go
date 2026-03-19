package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jvlerner/auth-system/internal/identity/application"
	"github.com/jvlerner/auth-system/internal/identity/domain"
	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// ─── Mocks em memória (sem frameworks) ─────────────────────────────────────

type mockUserRepo struct {
	users map[string]*domain.User // chave = email
}

func newMockUserRepo(users ...*domain.User) *mockUserRepo {
	m := &mockUserRepo{users: make(map[string]*domain.User)}
	for _, u := range users {
		m.users[u.Email()] = u
	}
	return m
}

func (m *mockUserRepo) FindByEmail(ctx context.Context, email domain.Email) (*domain.User, error) {
	u, ok := m.users[email.Value()]
	if !ok {
		return nil, nil
	}
	return u, nil
}
func (m *mockUserRepo) FindByID(ctx context.Context, id string) (*domain.User, error)       { return nil, nil }
func (m *mockUserRepo) Save(ctx context.Context, u *domain.User) error                      { return nil }
func (m *mockUserRepo) Update(ctx context.Context, u *domain.User) error                    { return nil }
func (m *mockUserRepo) UpdateRoles(ctx context.Context, id string, roles []string) error    { return nil }

// ─── Mock PasswordHasher ────────────────────────────────────────────────────

type mockHasher struct {
	shouldFail    bool
	needsUpgrade  bool
	hashedResult  domain.Password
}

func (h *mockHasher) Hash(ctx context.Context, plainText string) (domain.Password, error) {
	if h.shouldFail {
		return domain.Password{}, errors.New("hash error")
	}
	p, _ := domain.RestorePassword("v1:$argon2id$v=19$m=65536,t=1,p=4$fake$hash")
	return p, nil
}

func (h *mockHasher) Compare(ctx context.Context, plainText string, hashed domain.Password) (bool, error) {
	if h.shouldFail {
		return false, domain.ErrInvalidCredentials
	}
	return h.needsUpgrade, nil
}

// ─── Mock TokenGenerator ────────────────────────────────────────────────────

type mockTokenGen struct{}

func (t *mockTokenGen) Generate(ctx context.Context, userID, email string, roles []string) (*domain.Token, error) {
	return &domain.Token{AccessToken: "mock-jwt", ExpiresIn: 3600}, nil
}

// ─── Mock OutboxRepository ──────────────────────────────────────────────────

type mockOutbox struct {
	saved []string
}

func (o *mockOutbox) SaveCommand(ctx context.Context, aggregateID, aggregateType, eventType string, payload []byte) error {
	o.saved = append(o.saved, eventType)
	return nil
}

// ─── Helper ─────────────────────────────────────────────────────────────────

// buildUseCase monta o LoginUserUseCase com dependências mockadas e um mini-redis local.
func buildUseCase(t *testing.T, repo domain.UserRepository, hasher domain.PasswordHasher, outbox domain.OutboxRepository) (*application.LoginUserUseCase, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	return application.NewLoginUserUseCase(repo, hasher, &mockTokenGen{}, outbox, rdb), mr
}

// buildUser cria um User de domínio com senha já hasheada como mock.
func buildUser(t *testing.T) *domain.User {
	t.Helper()
	email, _ := domain.NewEmail("user@example.com")
	hash, _ := domain.RestorePassword("v1:$argon2id$v=19$m=65536,t=1,p=4$fake$hash")
	return domain.NewUser(email, hash)
}

// ─── Testes ─────────────────────────────────────────────────────────────────

func TestLogin_EmailInvalido_RetornaErro(t *testing.T) {
	uc, _ := buildUseCase(t, newMockUserRepo(), &mockHasher{}, &mockOutbox{})

	_, err := uc.Execute(context.Background(), application.LoginRequestDTO{
		Email:    "nao-é-um-email",
		Password: "qualquer",
	})

	if !errors.Is(err, application.ErrInvalidLogin) {
		t.Errorf("esperado ErrInvalidLogin, obtido: %v", err)
	}
}

func TestLogin_UsuarioNaoEncontrado_RetornaErro(t *testing.T) {
	uc, _ := buildUseCase(t, newMockUserRepo(), &mockHasher{}, &mockOutbox{})

	_, err := uc.Execute(context.Background(), application.LoginRequestDTO{
		Email:    "naoexiste@exemplo.com",
		Password: "qualquer",
	})

	if !errors.Is(err, application.ErrInvalidLogin) {
		t.Errorf("esperado ErrInvalidLogin, obtido: %v", err)
	}
}

func TestLogin_SenhaInvalida_RetornaErro(t *testing.T) {
	user := buildUser(t)
	uc, _ := buildUseCase(t, newMockUserRepo(user), &mockHasher{shouldFail: true}, &mockOutbox{})

	_, err := uc.Execute(context.Background(), application.LoginRequestDTO{
		Email:    "user@example.com",
		Password: "senha-errada",
	})

	if !errors.Is(err, application.ErrInvalidLogin) {
		t.Errorf("esperado ErrInvalidLogin, obtido: %v", err)
	}
}

func TestLogin_Sucesso_RetornaTokens(t *testing.T) {
	user := buildUser(t)
	uc, _ := buildUseCase(t, newMockUserRepo(user), &mockHasher{}, &mockOutbox{})

	res, err := uc.Execute(context.Background(), application.LoginRequestDTO{
		Email:    "user@example.com",
		Password: "correta",
	})

	if err != nil {
		t.Fatalf("esperado sucesso, obtido erro: %v", err)
	}
	if res.AccessToken == "" {
		t.Error("access_token não pode ser vazio")
	}
	if res.RefreshToken == "" {
		t.Error("refresh_token não pode ser vazio")
	}
}

func TestLogin_HashLegado_EmiteEventoDeUpgrade(t *testing.T) {
	user := buildUser(t)
	outbox := &mockOutbox{}
	uc, _ := buildUseCase(t, newMockUserRepo(user), &mockHasher{needsUpgrade: true}, outbox)

	_, err := uc.Execute(context.Background(), application.LoginRequestDTO{
		Email:    "user@example.com",
		Password: "correta",
	})

	if err != nil {
		t.Fatalf("esperado sucesso, obtido erro: %v", err)
	}
	if len(outbox.saved) == 0 || outbox.saved[0] != "UserPasswordUpgradeRequested" {
		t.Errorf("esperado evento UserPasswordUpgradeRequested na outbox, obtido: %v", outbox.saved)
	}
}
