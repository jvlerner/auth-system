package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jvlerner/auth-system/internal/identity"
	"github.com/jvlerner/auth-system/internal/identity/domain"
	"github.com/jvlerner/auth-system/internal/identity/infrastructure"
	"github.com/jvlerner/auth-system/internal/identity/presentation"
	"github.com/jvlerner/auth-system/pkg/auth"
	"github.com/jvlerner/auth-system/pkg/db/pgxdb"
	"github.com/jvlerner/auth-system/pkg/health"
	mw "github.com/jvlerner/auth-system/pkg/middleware"
	"github.com/jvlerner/auth-system/pkg/queue"
	"github.com/jvlerner/auth-system/pkg/telemetry"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Configuração do ambiente
type Config struct {
	Port            string
	DBUrl           string
	GrpcPasswordUrl string
	GrpcTokenUrl    string
	RedisUrl        string
	CertFile        string
	RabbitMqUrl     string
}

func ProvideConfig() Config {
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}
	dbUrl := os.Getenv("DB_URL")
	grpcUrl := os.Getenv("GRPC_PASSWORD_URL")
	if grpcUrl == "" {
		grpcUrl = "localhost:50051"
	}
	grpcTokenUrl := os.Getenv("GRPC_TOKEN_URL")
	if grpcTokenUrl == "" {
		grpcTokenUrl = "localhost:50052"
	}
	redisUrl := os.Getenv("REDIS_URL")
	if redisUrl == "" {
		redisUrl = "redis://localhost:6379"
	}
	cert := os.Getenv("TLS_CERT_PATH")
	rabbitMqUrl := os.Getenv("RABBITMQ_URL")
	if rabbitMqUrl == "" {
		rabbitMqUrl = "amqp://guest:guest@localhost:5672/"
	}

	return Config{
		Port: port, DBUrl: dbUrl, GrpcPasswordUrl: grpcUrl, 
		GrpcTokenUrl: grpcTokenUrl, RedisUrl: redisUrl, CertFile: cert, RabbitMqUrl: rabbitMqUrl,
	}
}

// Injeção do Logger da Uber (ZAP)
func ProvideLogger() *zap.Logger {
	logger, _ := zap.NewProduction()
	return logger
}

func ProvideRedis(lifecycle fx.Lifecycle, cfg Config, logger *zap.Logger) *redis.Client {
	ctx := context.Background()
	opts, err := redis.ParseURL(cfg.RedisUrl)
	if err != nil {
		logger.Fatal("Failed to parse Redis URL", zap.Error(err))
	}
	client := redis.NewClient(opts)
	err = client.Ping(ctx).Err()
	if err != nil {
		logger.Fatal("Failed to connect to Redis", zap.Error(err))
	}

	lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Info("Closing Redis connection in API Write")
			client.Close()
			return nil
		},
	})
	return client
}

func ProvideDB(lifecycle fx.Lifecycle, cfg Config, logger *zap.Logger) *pgxpool.Pool {
	ctx := context.Background()
	pool, err := pgxdb.Setup(ctx, cfg.DBUrl)
	if err != nil {
		logger.Fatal("Failed to connect to Write Database", zap.Error(err))
	}

	lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Info("Closing write database connection pool")
			pool.Close()
			return nil
		},
	})
	return pool
}

// Injeção do Roteador HTTP (Chi)
func ProvideRouter(
	authHandler *presentation.AuthHandler,
	pool *pgxpool.Pool,
	redisClient *redis.Client,
	cfg Config,
	logger *zap.Logger,
) *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// OpenTelemetry Middleware
	r.Use(func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, "api-write")
	})

	// Rate Limiter genérico para rotas de Comando Públicas
	// Limitando agressivamente tentativas de Brute Force: 5 requests por minuto por IP
	rateLimiter := mw.RedisRateLimiter(redisClient, 5, time.Minute, logger)

	r.Group(func(public chi.Router) {
		public.Use(rateLimiter)

		public.Post("/api/v1/commands/register", authHandler.Register)
		public.Post("/api/v1/commands/login", authHandler.Login)
		public.Post("/api/v1/commands/forgot-password", authHandler.ForgotPassword)
		public.Post("/api/v1/commands/reset-password", authHandler.ResetPassword)
		public.Post("/api/v1/commands/verify-mfa", authHandler.VerifyMFA)
	})

	// Rotas Públicas Normais (sem Rate Limit agressivo ou com outro limitador)
	r.Post("/api/v1/commands/confirm-email", authHandler.ConfirmEmail)
	r.Get("/api/v1/commands/confirm-email", authHandler.ConfirmEmail)
	r.Post("/api/v1/commands/refresh-token", authHandler.RefreshToken)
	r.Post("/api/v1/commands/logout", authHandler.Logout)

	// Rotas Autenticadas (Requerem JWT)
	jwtGuard, err := auth.JWTGuard(auth.Config{PublicKeyPath: cfg.CertFile}, logger)
	if err != nil {
		logger.Fatal("Falha ao inicializar JWT Guard", zap.Error(err))
	}

	r.Group(func(protected chi.Router) {
		protected.Use(jwtGuard)

		protected.Post("/api/v1/commands/setup-mfa", authHandler.SetupMFA)

		// Sub-grupo apenas para Administradores
		protected.Group(func(admin chi.Router) {
			admin.Use(auth.RequireRoles("admin"))
			admin.Post("/api/v1/commands/users/{id}/roles", authHandler.UpdateRoles)
		})
	})

	// Health Probes (Kubernetes)
	r.Get("/health/live", health.LivenessHandler)
	r.Get("/health/ready", health.ReadinessHandler(health.ReadinessCheckers{
		"postgres": health.PostgresChecker(pool),
		"redis":    health.RedisChecker(redisClient),
	}))

	return r
}

// Registrar o Servidor HTTP no Lifecycle do Fx (Graceful Shutdown)
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
			logger.Info("Iniciando servidor API Write", zap.String("porta", cfg.Port))
			go func() {
				if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					logger.Fatal("Erro ao iniciar servidor HTTP", zap.Error(err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("Encerrando servidor API Write graciosamente...")
			return srv.Shutdown(ctx)
		},
	})
}

func ProvideGrpcHasher(cfg Config, logger *zap.Logger) domain.PasswordHasher {
	logger.Info("Connecting to gRPC Password Hasher", zap.String("url", cfg.GrpcPasswordUrl))
	hasher, err := infrastructure.NewGrpcPasswordHasher(cfg.GrpcPasswordUrl, cfg.CertFile)
	if err != nil {
		logger.Fatal("Failed to setup gRPC Password client", zap.Error(err))
	}
	return hasher
}

func ProvideGrpcTokenClient(cfg Config, logger *zap.Logger) domain.TokenGenerator {
	logger.Info("Connecting to gRPC Token Service", zap.String("url", cfg.GrpcTokenUrl))
	hasher, err := infrastructure.NewGrpcTokenClient(cfg.GrpcTokenUrl, cfg.CertFile)
	if err != nil {
		logger.Fatal("Failed to setup gRPC Token client", zap.Error(err))
	}
	return hasher
}

func ProvideRabbitMQPublisher(lifecycle fx.Lifecycle, cfg Config, logger *zap.Logger) queue.Publisher {
	pub, err := queue.NewRabbitMQPublisher(cfg.RabbitMqUrl)
	if err != nil {
		logger.Fatal("Failed to connect RabbitMQ publisher", zap.Error(err))
	}
	lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			pub.Close()
			return nil
		},
	})
	return pub
}

func RegisterOutboxRelay(
	lifecycle fx.Lifecycle,
	pool *pgxpool.Pool,
	publisher queue.Publisher,
	logger *zap.Logger,
) {
	relay := infrastructure.NewOutboxRelay(pool, publisher, logger)

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("Starting API Write Internal Outbox Relay")
			go relay.Start(context.Background())
			return nil
		},
	})
}

func main() {
	// A mágica do Uber Fx: Injeção de Dependências Dinâmica + Lifecycle
	app := fx.New(
		fx.Provide(
			ProvideConfig,
			ProvideLogger,
			telemetry.ProvideTracer,
			ProvideRedis,
			ProvideDB,
			infrastructure.NewPostgresUserRepository,
			infrastructure.NewPostgresOutboxRepository,
			ProvideGrpcHasher,
			ProvideGrpcTokenClient,
			ProvideRabbitMQPublisher,
			ProvideRouter,
		),
		identity.Module, // Inject do contexto do Domínio Identity
		fx.Invoke(func(tp *trace.TracerProvider) {}), // Require Tracer provider to start
		fx.Invoke(RegisterHooks),
		fx.Invoke(RegisterOutboxRelay),
	)

	app.Run()
}
