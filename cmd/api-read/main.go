package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/jvlerner/auth-system/internal/identity/application"
	"github.com/jvlerner/auth-system/internal/identity/infrastructure"
	"github.com/jvlerner/auth-system/internal/identity/presentation"
	"github.com/jvlerner/auth-system/pkg/db/pgxdb"
	"github.com/jvlerner/auth-system/pkg/db/redisdb"
	"github.com/jvlerner/auth-system/pkg/telemetry"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type Config struct {
	Port     string
	DBUrl    string
	RedisUrl string
}

func ProvideConfig() Config {
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8081"
	}
	dbUrl := os.Getenv("DB_READ_URL")
	redisUrl := os.Getenv("REDIS_URL")
	if redisUrl == "" {
		redisUrl = "redis://localhost:6379"
	}
	return Config{Port: port, DBUrl: dbUrl, RedisUrl: redisUrl}
}

func ProvideLogger() *zap.Logger {
	logger, _ := zap.NewProduction()
	return logger
}

func ProvideDB(lifecycle fx.Lifecycle, cfg Config, logger *zap.Logger) *pgxpool.Pool {
	ctx := context.Background()
	pool, err := pgxdb.Setup(ctx, cfg.DBUrl)
	if err != nil {
		logger.Fatal("Failed to connect to Read DB", zap.Error(err))
	}

	lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Info("Closing database connection pool")
			pool.Close()
			return nil
		},
	})
	return pool
}

func ProvideRedis(lifecycle fx.Lifecycle, cfg Config, logger *zap.Logger) *redis.Client {
	ctx := context.Background()
	client, err := redisdb.Setup(ctx, cfg.RedisUrl)
	if err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Info("Closing Redis connection")
			client.Close()
			return nil
		},
	})
	return client
}

func ProvideRouter(logger *zap.Logger, queryHandler *presentation.QueryHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// OpenTelemetry Middleware
	r.Use(func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, "api-read")
	})

	r.Get("/api/v1/queries/users/{id}", queryHandler.GetProfile)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("API Read: Ok!"))
	})

	return r
}

func RegisterHooks(
	lifecycle fx.Lifecycle,
	logger *zap.Logger,
	cfg Config,
	r *chi.Mux,
) {
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.Port),
		Handler: r,
	}

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("Iniciando servidor API Read", zap.String("porta", cfg.Port))
			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					logger.Fatal("Erro ao iniciar servidor HTTP", zap.Error(err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("Encerrando servidor API Read graciosamente...")
			return srv.Shutdown(ctx)
		},
	})
}

func main() {
	app := fx.New(
		fx.Provide(
			ProvideConfig,
			ProvideLogger,
			telemetry.ProvideTracer,
			ProvideDB,
			ProvideRedis,
			infrastructure.NewPostgresReadRepository,
			infrastructure.NewRedisProfileCache,
			application.NewGetUserProfileUseCase,
			presentation.NewQueryHandler,
			ProvideRouter,
		),
		fx.Invoke(func(tp *trace.TracerProvider) {}), // Require Tracer provider to start
		fx.Invoke(RegisterHooks),
	)

	app.Run()
}
