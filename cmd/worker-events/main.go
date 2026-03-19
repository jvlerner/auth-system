package main

import (
	"context"
	"os"

	"github.com/jvlerner/auth-system/internal/identity/application"
	"github.com/jvlerner/auth-system/internal/identity/domain"
	"github.com/jvlerner/auth-system/internal/identity/infrastructure"
	"github.com/jvlerner/auth-system/pkg/db/pgxdb"
	"github.com/jvlerner/auth-system/pkg/queue"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type Config struct {
	DBWriteUrl      string
	RabbitMQUrl     string
	GrpcPasswordUrl string
	RedisUrl        string
	CertFile        string
}

func ProvideConfig() Config {
	grpcUrl := os.Getenv("GRPC_PASSWORD_URL")
	if grpcUrl == "" {
		grpcUrl = "localhost:50051"
	}
	redisUrl := os.Getenv("REDIS_URL")
	if redisUrl == "" {
		redisUrl = "redis://localhost:6379"
	}
	return Config{
		DBWriteUrl:      os.Getenv("DB_WRITE_URL"),
		RabbitMQUrl:     os.Getenv("RABBITMQ_URL"),
		GrpcPasswordUrl: grpcUrl,
		RedisUrl:        redisUrl,
		CertFile:        os.Getenv("TLS_CERT_PATH"),
	}
}

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
			logger.Info("Closing Redis connection in Worker")
			client.Close()
			return nil
		},
	})
	return client
}

// Conexão do DB Master (de onde sai o Outbox)
func ProvideWriteDB(lifecycle fx.Lifecycle, cfg Config, logger *zap.Logger) *pgxpool.Pool {
	ctx := context.Background()
	pool, err := pgxdb.Setup(ctx, cfg.DBWriteUrl)
	if err != nil {
		logger.Fatal("Failed to connect to Write DB", zap.Error(err))
	}
	lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			pool.Close()
			return nil
		},
	})
	return pool
}

// Provedores RabbitMQ (Publisher do Outbox e Consumer dos Eventos)
func ProvideRabbitMQPublisher(lifecycle fx.Lifecycle, cfg Config, logger *zap.Logger) queue.Publisher {
	pub, err := queue.NewRabbitMQPublisher(cfg.RabbitMQUrl)
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

func ProvideRabbitMQConsumer(lifecycle fx.Lifecycle, cfg Config, logger *zap.Logger) queue.Consumer {
	consumer, err := queue.NewRabbitMQConsumer(
		cfg.RabbitMQUrl,
		"auth_registration_commands_queue",
		"auth.commands",
		"user.*", // Ouve todos os comandos de user (register, update_roles, UserRegistered)
		logger,
	)
	if err != nil {
		logger.Fatal("Failed to connect RabbitMQ consumer", zap.Error(err))
	}
	lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			consumer.Close()
			return nil
		},
	})
	return consumer
}

func ProvideGrpcHasher(cfg Config, logger *zap.Logger) domain.PasswordHasher {
	logger.Info("Connecting to gRPC Password Hasher in Worker", zap.String("url", cfg.GrpcPasswordUrl))
	hasher, err := infrastructure.NewGrpcPasswordHasher(cfg.GrpcPasswordUrl, cfg.CertFile)
	if err != nil {
		logger.Fatal("Failed to setup gRPC Password client", zap.Error(err))
	}
	return hasher
}

func RegisterWorkers(
	lifecycle fx.Lifecycle,
	outboxRelay *infrastructure.OutboxRelay,
	consumer queue.Consumer,
	processRegistrationUseCase *application.ProcessUserRegistrationUseCase,
	processUpdateRolesUseCase *application.ProcessUpdateUserRolesUseCase,
	sendEmailConfirmationUseCase *application.SendEmailConfirmationUseCase,
	sendPasswordResetEmailUseCase *application.SendPasswordResetEmailUseCase,
	processPasswordUpgradeUseCase *application.ProcessPasswordUpgradeUseCase,
	logger *zap.Logger,
) {
	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			// Inicia o processo que varre a tabela Outbox -> RabbitMQ
			go outboxRelay.Start(ctx)

			// Inicia o consumo do RabbitMQ
			err := consumer.Consume(ctx, func(routingKey string, payload []byte) error {
				switch routingKey {
				case "user.register":
					return processRegistrationUseCase.HandleCommand(context.Background(), payload)
				case "user.update_roles":
					return processUpdateRolesUseCase.HandleCommand(context.Background(), payload)
				case "user.UserRegistered":
					return sendEmailConfirmationUseCase.HandleCommand(context.Background(), payload)
				case "user.UserForgotPasswordRequested":
					return sendPasswordResetEmailUseCase.HandleCommand(context.Background(), payload)
				case "user.UserPasswordUpgradeRequested":
					return processPasswordUpgradeUseCase.HandleCommand(context.Background(), payload)
				default:
					logger.Warn("Worker received unknown routing key. Skipping.", zap.String("routing_key", routingKey))
					return nil // Ignora comandos desconhecidos
				}
			})
			return err
		},
	})
}

func main() {
	app := fx.New(
		fx.Provide(
			ProvideConfig,
			ProvideLogger,
			ProvideRedis,

			// Infra
			ProvideWriteDB,
			infrastructure.NewPostgresUserRepository,
			ProvideGrpcHasher,
			ProvideRabbitMQPublisher,
			ProvideRabbitMQConsumer,

			infrastructure.NewOutboxRelay,

			// UseCase
			application.NewProcessUserRegistrationUseCase,
			application.NewProcessUpdateUserRolesUseCase,
			application.NewSendEmailConfirmationUseCase,
			application.NewSendPasswordResetEmailUseCase,
			application.NewProcessPasswordUpgradeUseCase,
		),
		fx.Invoke(RegisterWorkers),
	)
	app.Run()
}
