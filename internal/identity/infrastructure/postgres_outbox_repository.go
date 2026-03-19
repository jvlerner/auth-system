package infrastructure

import (
	"context"

	"github.com/jvlerner/auth-system/internal/identity/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresOutboxRepository implementa domain.OutboxRepository
type PostgresOutboxRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresOutboxRepository(pool *pgxpool.Pool) domain.OutboxRepository {
	return &PostgresOutboxRepository{
		pool: pool,
	}
}

// SaveCommand insere um comando na tabela outbox_events.
func (r *PostgresOutboxRepository) SaveCommand(ctx context.Context, aggregateID string, aggregateType string, eventType string, payload []byte) error {
	query := `
		INSERT INTO outbox_events (aggregate_id, aggregate_type, event_type, payload) 
		VALUES ($1, $2, $3, $4)
	`
	_, err := r.pool.Exec(ctx, query, aggregateID, aggregateType, eventType, payload)
	return err
}
