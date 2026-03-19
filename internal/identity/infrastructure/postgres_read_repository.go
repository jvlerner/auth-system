package infrastructure

import (
	"context"
	"time"

	"github.com/jvlerner/auth-system/internal/identity/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresReadRepository é a implementação de leitura (CQRS).
// Ele vai direto no banco `postgres-read` que é mais performático para queries.
type PostgresReadRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresReadRepository(pool *pgxpool.Pool) *PostgresReadRepository {
	return &PostgresReadRepository{
		pool: pool,
	}
}

// SyncUser é chamado pelo RabbitMQ Consumer.
// Quando o evento "UserRegistered" chega, este método salva a cópia no read_db
func (r *PostgresReadRepository) FindByID(ctx context.Context, id string) (*domain.User, error) {
	query := `
		SELECT id, email, email_verified, mfa_enabled, totp_secret, created_at, updated_at 
		FROM users 
		WHERE id = $1
	`
	var (
		dbID, mail, totpSecret string
		emailVerified, mfaEnabled bool
		createdAt     time.Time
		updatedAt     time.Time
	)

	err := r.pool.QueryRow(ctx, query, id).Scan(&dbID, &mail, &emailVerified, &mfaEnabled, &totpSecret, &createdAt, &updatedAt)
	if err != nil {
		return nil, domain.ErrUserNotFound 
	}

	return domain.RestoreUser(dbID, mail, "fakehash", emailVerified, mfaEnabled, totpSecret, createdAt, updatedAt)
}
