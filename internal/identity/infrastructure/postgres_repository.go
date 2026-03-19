package infrastructure

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jvlerner/auth-system/internal/identity/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresUserRepository implementa domain.UserRepository
type PostgresUserRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresUserRepository(pool *pgxpool.Pool) domain.UserRepository {
	return &PostgresUserRepository{
		pool: pool,
	}
}

// UserRegisteredEvent define a estrutura do evento que vai para fila.
type UserRegisteredEvent struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

// Save insere o usuário e o evento outbox em uma MESMA transação atômica.
func (r *PostgresUserRepository) Save(ctx context.Context, user *domain.User) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// 1. Salvar na tabela de write (users)
	query := `
		INSERT INTO users (id, email, password_hash, email_verified, mfa_enabled, totp_secret, created_at, updated_at) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	_, err = tx.Exec(ctx, query,
		user.ID().String(),
		user.Email(),
		user.HashedPassword(),
		user.IsEmailVerified(),
		user.IsMFAEnabled(),
		user.TOTPSecret(),
		user.CreatedAt(),
		user.UpdatedAt(),
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.ErrUserAlreadyExists
		}
		return err
	}

	// 2. Preparar payload do evento para RabbitMQ
	eventPayload := UserRegisteredEvent{
		ID:        user.ID().String(),
		Email:     user.Email(),
		CreatedAt: user.CreatedAt(),
	}
	payloadBytes, _ := json.Marshal(eventPayload)

	// 3. Salvar no Outbox transacionalmente (O Worker vai ler isso depois)
	outboxQuery := `
		INSERT INTO outbox_events (aggregate_id, aggregate_type, event_type, payload) 
		VALUES ($1, $2, $3, $4)
	`
	_, err = tx.Exec(ctx, outboxQuery,
		user.ID().String(),
		"user",
		"UserRegistered",
		payloadBytes,
	)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *PostgresUserRepository) FindByEmail(ctx context.Context, email domain.Email) (*domain.User, error) {
	query := `
		SELECT id, email, password_hash, roles, email_verified, mfa_enabled, totp_secret, created_at, updated_at 
		FROM users 
		WHERE email = $1
	`
	var (
		id, mail, hash, totpSecret string
		roles                      []string
		emailVerified, mfaEnabled  bool
		createdAt                  pgtype.Timestamptz
		updatedAt                  pgtype.Timestamptz
	)

	err := r.pool.QueryRow(ctx, query, email.Value()).Scan(&id, &mail, &hash, &roles, &emailVerified, &mfaEnabled, &totpSecret, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // Nil = não achou (não é um erro fatal para o domínio)
		}
		return nil, err
	}

	user, err := domain.RestoreUser(id, mail, hash, emailVerified, mfaEnabled, totpSecret, createdAt.Time, updatedAt.Time)
	if err != nil {
		return nil, err
	}
	// Adicionando as roles vindas do DB
	for _, role := range roles {
		user.AddRole(role)
	}
	return user, nil
}

func (r *PostgresUserRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	query := `
		SELECT id, email, password_hash, roles, email_verified, mfa_enabled, totp_secret, created_at, updated_at 
		FROM users 
		WHERE id = $1
	`
	var (
		dbID, mail, hash, totpSecret string
		roles                        []string
		emailVerified, mfaEnabled    bool
		createdAt                    pgtype.Timestamptz
		updatedAt                    pgtype.Timestamptz
	)

	err := r.pool.QueryRow(ctx, query, id).Scan(&dbID, &mail, &hash, &roles, &emailVerified, &mfaEnabled, &totpSecret, &createdAt, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}

	user, err := domain.RestoreUser(dbID, mail, hash, emailVerified, mfaEnabled, totpSecret, createdAt.Time, updatedAt.Time)
	if err != nil {
		return nil, err
	}
	for _, role := range roles {
		user.AddRole(role)
	}
	return user, nil
}

func (r *PostgresUserRepository) Update(ctx context.Context, user *domain.User) error {
	query := `
		UPDATE users 
		SET password_hash = $1, mfa_enabled = $2, totp_secret = $3, updated_at = $4 
		WHERE id = $5
	`
	_, err := r.pool.Exec(ctx, query,
		user.HashedPassword(),
		user.IsMFAEnabled(),
		user.TOTPSecret(),
		user.UpdatedAt(),
		user.ID().String(),
	)
	return err
}

func (r *PostgresUserRepository) UpdateRoles(ctx context.Context, id string, roles []string) error {
	query := `
		UPDATE users 
		SET roles = $1, updated_at = NOW() 
		WHERE id = $2
	`
	_, err := r.pool.Exec(ctx, query, roles, id)
	return err
}
