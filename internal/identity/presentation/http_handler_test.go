package presentation_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jvlerner/auth-system/internal/identity/application"
	"github.com/jvlerner/auth-system/internal/identity/domain"
	"github.com/jvlerner/auth-system/internal/identity/presentation"
	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// ─── Stub Use Cases (satisfazem interface indireta via injeção no handler) ───

// Para testar os Handlers sem precisar de banco ou gRPC, injetamos stubs
// usando o padrão de construtores reais com dependências mockadas.

// mockUserRepo para testes de Apresentação
type pressMockUserRepo struct{}

func (m *pressMockUserRepo) Save(ctx context.Context, u *domain.User) error              { return nil }
func (m *pressMockUserRepo) FindByEmail(ctx context.Context, e domain.Email) (*domain.User, error) {
	return nil, nil // simula usuário não existe
}
func (m *pressMockUserRepo) FindByID(ctx context.Context, id string) (*domain.User, error)    { return nil, nil }
func (m *pressMockUserRepo) Update(ctx context.Context, u *domain.User) error                 { return nil }
func (m *pressMockUserRepo) UpdateRoles(ctx context.Context, id string, roles []string) error { return nil }

// mockHasher para testes de Apresentação
type pressMockHasher struct{}

func (h *pressMockHasher) Hash(ctx context.Context, p string) (domain.Password, error) {
	pass, _ := domain.RestorePassword("v1:$argon2id$v=19$m=65536,t=1,p=4$fake$hash")
	return pass, nil
}
func (h *pressMockHasher) Compare(ctx context.Context, p string, hashed domain.Password) (bool, error) {
	return false, nil
}

// mockTokenGen para testes de Apresentação
type pressMockTokenGen struct{}

func (t *pressMockTokenGen) Generate(ctx context.Context, userID, email string, roles []string) (*domain.Token, error) {
	return &domain.Token{AccessToken: "mock-token", ExpiresIn: 3600}, nil
}

// mockOutbox para testes de Apresentação
type pressMockOutbox struct{}

func (o *pressMockOutbox) SaveCommand(ctx context.Context, aggregateID, aggregateType, eventType string, payload []byte) error {
	return nil
}

// ─── Helper: constrói o AuthHandler com stubs ─────────────────────────────

func buildHandler(t *testing.T) *presentation.AuthHandler {
	t.Helper()

	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	logger := zap.NewNop()
	repo := &pressMockUserRepo{}
	hasher := &pressMockHasher{}
	outbox := &pressMockOutbox{}
	tokenGen := &pressMockTokenGen{}

	return presentation.NewAuthHandler(
		application.NewRegisterUserUseCase(repo, outbox),
		application.NewLoginUserUseCase(repo, hasher, tokenGen, outbox, rdb),
		application.NewUpdateUserRolesUseCase(repo, outbox),
		application.NewConfirmEmailUseCase(repo, rdb, logger),
		application.NewForgotPasswordUseCase(repo, outbox, logger),
		application.NewResetPasswordUseCase(repo, hasher, rdb, logger),
		application.NewRefreshTokenUseCase(rdb, repo, tokenGen),
		application.NewLogoutUseCase(rdb),
		application.NewSetupMFAUseCase(repo, logger),
		application.NewVerifyMFAUseCase(rdb, repo, tokenGen),
		logger,
	)
}

// ─── Register Handler Tests ──────────────────────────────────────────────────

func TestRegisterHandler_JsonMalformado_Retorna400(t *testing.T) {
	h := buildHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/commands/register", strings.NewReader("{invalido"))
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("esperado 400, obtido %d", w.Code)
	}
}

func TestRegisterHandler_EmailInvalido_Retorna400(t *testing.T) {
	h := buildHandler(t)

	body := bytes.NewBufferString(`{"email":"nao-email","password":"Senha123!"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/commands/register", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("esperado 400, obtido %d", w.Code)
	}
}

func TestRegisterHandler_Sucesso_Retorna201(t *testing.T) {
	h := buildHandler(t)

	body := bytes.NewBufferString(`{"email":"novo@example.com","password":"Senha123!"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/commands/register", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Register(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("esperado 201, obtido %d — body: %s", w.Code, w.Body.String())
	}
}

// ─── Login Handler Tests ──────────────────────────────────────────────────

func TestLoginHandler_JsonMalformado_Retorna400(t *testing.T) {
	h := buildHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/commands/login", strings.NewReader("{invalido"))
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("esperado 400, obtido %d", w.Code)
	}
}

func TestLoginHandler_CredenciaisInvalidas_Retorna401(t *testing.T) {
	h := buildHandler(t)

	// usuário não existe no mockRepo → ErrInvalidLogin → 401
	body := bytes.NewBufferString(`{"email":"naoexiste@example.com","password":"qualquer"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/commands/login", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Login(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("esperado 401, obtido %d", w.Code)
	}
}

// ─── Logout Handler Tests ─────────────────────────────────────────────────

func TestLogoutHandler_JsonMalformado_Retorna400(t *testing.T) {
	h := buildHandler(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/commands/logout", strings.NewReader("{{{"))
	w := httptest.NewRecorder()

	h.Logout(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("esperado 400, obtido %d", w.Code)
	}
}

func TestLogoutHandler_TokenQualquer_Retorna204(t *testing.T) {
	h := buildHandler(t)

	// Del de chave inexistente no Redis é idempotente
	body := bytes.NewBufferString(`{"refresh_token":"token-qualquer"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/commands/logout", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.Logout(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("esperado 204, obtido %d", w.Code)
	}
}
