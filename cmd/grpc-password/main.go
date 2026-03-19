package main

import (
	"context"
	"net"
	"os"

	"github.com/jvlerner/auth-system/internal/password/application"
	"github.com/jvlerner/auth-system/internal/password/presentation"
	"github.com/jvlerner/auth-system/internal/password/proto"
	"github.com/jvlerner/auth-system/pkg/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Config struct {
	Port     string
	CertFile string
	KeyFile  string
}

func ProvideConfig() Config {
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "50051" // Porta Padrão gRPC
	}
	cert := os.Getenv("TLS_CERT_PATH")
	key := os.Getenv("TLS_KEY_PATH")
	return Config{Port: port, CertFile: cert, KeyFile: key}
}

func ProvideLogger() *zap.Logger {
	logger, _ := zap.NewProduction()
	return logger
}

// Inicializador limpo rodando tcp Listener para o grpc rodar o Server
func RunGrpcServer(lifecycle fx.Lifecycle, logger *zap.Logger, cfg Config, grpcHandler *presentation.GrpcServer) {
	var opts []grpc.ServerOption
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		creds, err := credentials.NewServerTLSFromFile(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			logger.Fatal("Failed to setup TLS", zap.Error(err))
		}
		opts = append(opts, grpc.Creds(creds))
		logger.Info("TLS enabled for gRPC Password server")
	}

	opts = append(opts, grpc.StatsHandler(otelgrpc.NewServerHandler()))

	server := grpc.NewServer(opts...)
	
	proto.RegisterPasswordServiceServer(server, grpcHandler)

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			lis, err := net.Listen("tcp", ":"+cfg.Port)
			if err != nil {
				return err
			}

			logger.Info("Starting gRPC Password Service", zap.String("port", cfg.Port))
			go func() {
				if err := server.Serve(lis); err != nil {
					logger.Error("gRPC server failed", zap.Error(err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("Gracefully stopping gRPC Password Service")
			server.GracefulStop()
			return nil
		},
	})
}

func main() {
	app := fx.New(
		fx.Provide(
			ProvideConfig,
			ProvideLogger,
			telemetry.ProvideTracer,
			// Registrar as versões de HashService disponíveis
			func() application.HashService { return application.NewPasswordServiceV1() },
			// Quando V2 existir, basta adicionar aqui:
			// func() application.HashService { return application.NewPasswordServiceV2() },
			func(svc application.HashService) *presentation.GrpcServer {
				return presentation.NewGrpcServer(svc)
			},
		),
		fx.Invoke(func(tp *trace.TracerProvider) {}),
		fx.Invoke(RunGrpcServer),
	)
	app.Run()
}
