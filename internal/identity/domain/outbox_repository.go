package domain

import "context"

// OutboxRepository define a porta para salvar Comandos duráveis que serão
// proativamente lidos por um Relay/Worker para garantir Entrega At-Least-Once na Fila.
type OutboxRepository interface {
	SaveCommand(ctx context.Context, aggregateID string, aggregateType string, eventType string, payload []byte) error
}
