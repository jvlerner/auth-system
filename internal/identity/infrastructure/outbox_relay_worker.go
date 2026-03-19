package infrastructure

import (
	"context"
	"time"

	"github.com/jvlerner/auth-system/pkg/queue"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// OutboxRelay varre a tabela de eventos não publicados e envia para o RabbitMQ
type OutboxRelay struct {
	pool      *pgxpool.Pool
	publisher queue.Publisher
	logger    *zap.Logger
}

func NewOutboxRelay(pool *pgxpool.Pool, publisher queue.Publisher, logger *zap.Logger) *OutboxRelay {
	return &OutboxRelay{
		pool:      pool,
		publisher: publisher,
		logger:    logger,
	}
}

// Start roda em background como um Worker.
func (r *OutboxRelay) Start(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Info("Outbox relay shutting down gracefully...")
			return
		case <-ticker.C:
			r.processOutbox(ctx)
		}
	}
}

func (r *OutboxRelay) processOutbox(ctx context.Context) {
	// Lógica simplificada: Buscar 50 eventos não publicados, enviar e marcar como publicado.
	// Em um ambiente de extremo volume, usaríamos FOR UPDATE SKIP LOCKED
	query := `
		SELECT id, event_type, payload 
		FROM outbox_events 
		WHERE published_at IS NULL 
		ORDER BY created_at ASC 
		LIMIT 50
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		r.logger.Error("Failed to query outbox", zap.Error(err))
		return
	}
	defer rows.Close()

	var eventsToMark []int

	for rows.Next() {
		var (
			id        int
			eventType string
			payload   []byte
		)
		if err := rows.Scan(&id, &eventType, &payload); err != nil {
			r.logger.Error("Failed to scan outbox row", zap.Error(err))
			continue
		}

		// Publicar no RabbitMQ (A exchange e routing key seguem padrões baseados no tipo do evento)
		err := r.publisher.Publish(ctx, "auth.events", "user."+eventType, payload)
		if err != nil {
			r.logger.Error("Failed to publish event to RabbitMQ", zap.Error(err), zap.Int("outbox_id", id))
			continue // Pula para o próximo e deixa esse para tentar no próximo tick
		}

		eventsToMark = append(eventsToMark, id)
	}
	rows.Close() // Fecha o resultset antes de fazer o update massivo

	// Marca todos os publicados
	if len(eventsToMark) > 0 {
		updateQuery := `UPDATE outbox_events SET published_at = NOW() WHERE id = ANY($1)`
		_, err := r.pool.Exec(ctx, updateQuery, eventsToMark)
		if err != nil {
			r.logger.Error("Failed to mark outbox events as published", zap.Error(err))
		} else {
			r.logger.Info("Outbox events published successfully", zap.Int("count", len(eventsToMark)))
		}
	}
}
