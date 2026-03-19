package application

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jvlerner/auth-system/internal/identity/domain"
)

// RegisterUserCommand é o DTO de entrada do Caso de Uso.
type RegisterUserCommand struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RegisterUserUseCase orquestra o registro de um novo usuário.
// Ele implementa a interface de UseCase e publica comandos no RabbitMQ.
type RegisterUserUseCase struct {
	repo   domain.UserRepository
	outbox domain.OutboxRepository
}

// NewRegisterUserUseCase é o construtor gerenciado por injeção de dependência via Uber Fx.
func NewRegisterUserUseCase(repo domain.UserRepository, outbox domain.OutboxRepository) *RegisterUserUseCase {
	return &RegisterUserUseCase{
		repo:   repo,
		outbox: outbox,
	}
}

// Execute executa a regra de negócio central para cadastrar um usuário.
func (uc *RegisterUserUseCase) Execute(ctx context.Context, cmd RegisterUserCommand) error {
	// 1. Criar e validar o e-mail (Sem persistência, puramente em memória)
	email, err := domain.NewEmail(cmd.Email)
	if err != nil {
		return err
	}

	// 2. Verificar se já existe um usuário
	existingUser, err := uc.repo.FindByEmail(ctx, email)
	if err != nil {
		return err // Ideal lidar com não encontrado sem erro fatal
	}
	if existingUser != nil {
		return domain.ErrUserAlreadyExists
	}

	// 3. Serializar o Comando para JSON
	payload, err := json.Marshal(cmd)
	if err != nil {
		return err
	}

	// 4. Salvar na Tabela Outbox local para envio assíncrono garantido pelo Worker Nativo da API.
	// Nota: o "Aggregate ID" aqui é gerado provisoriamente para constar na fila, embora
	// a persistência final de 'users' o Worker que ditará.
	cmdID := uuid.New().String()
	err = uc.outbox.SaveCommand(ctx, cmdID, "command", "user.register", payload)
	if err != nil {
		return err
	}

	return nil
}
