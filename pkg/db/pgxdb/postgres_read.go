package pgxdb

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ReadSetup conecta ao PostgreSQL de leitura.
// Reutiliza lógicas simples podendo ser expandido se o read-replica tiver tunning diferente
func ReadSetup(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	return Setup(ctx, databaseURL)
}
