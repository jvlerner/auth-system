package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// Status representa o estado de uma dependência individual.
type Status struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// HealthResponse é o payload retornado pelas rotas de saúde.
type HealthResponse struct {
	Status       string            `json:"status"`
	Dependencies map[string]Status `json:"dependencies,omitempty"`
}

// ReadinessCheckers é um mapa de funções de verificação por dependência.
type ReadinessCheckers map[string]func(ctx context.Context) error

// LivenessHandler retorna 200 OK enquanto o processo Go estiver vivo.
// Nunca falha — usado pelo K8s para detectar deadlocks/crashes.
func LivenessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(HealthResponse{Status: "ok"})
}

// ReadinessHandler verifica todas as dependências críticas.
// Retorna 503 Service Unavailable se qualquer dep estiver fora do ar.
func ReadinessHandler(checkers ReadinessCheckers) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		deps := make(map[string]Status, len(checkers))
		overall := "ok"

		for name, check := range checkers {
			if err := check(ctx); err != nil {
				deps[name] = Status{Status: "degraded", Error: err.Error()}
				overall = "degraded"
			} else {
				deps[name] = Status{Status: "ok"}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		if overall == "degraded" {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		json.NewEncoder(w).Encode(HealthResponse{Status: overall, Dependencies: deps})
	}
}

// PostgresChecker verifica a conectividade com o pool pgx.
func PostgresChecker(pool *pgxpool.Pool) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return pool.Ping(ctx)
	}
}

// RedisChecker verifica a conectividade com o Redis.
func RedisChecker(client *redis.Client) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return client.Ping(ctx).Err()
	}
}

// RabbitMQChecker verifica se uma conexão AMQP está ativa.
// Recebe uma função para evitar import direto do amqp no pacote health.
// Exemplo: health.RabbitMQChecker(func(ctx) error { if conn.IsClosed() { return amqp.ErrClosed } return nil })
func RabbitMQChecker(check func(ctx context.Context) error) func(ctx context.Context) error {
	return check
}
