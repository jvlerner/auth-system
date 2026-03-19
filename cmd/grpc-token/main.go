package main

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/jvlerner/auth-system/internal/token"
	"github.com/jvlerner/auth-system/internal/token/presentation"
	"github.com/jvlerner/auth-system/internal/token/proto"
	"github.com/jvlerner/auth-system/pkg/telemetry"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Config struct {
	Port    string
	CertFile string
	KeyFile  string
}

func ProvideConfig() Config {
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "50052"
	}
	cert := os.Getenv("TLS_CERT_PATH")
	key := os.Getenv("TLS_KEY_PATH")
	return Config{Port: port, CertFile: cert, KeyFile: key}
}

func ProvideLogger() *zap.Logger {
	logger, _ := zap.NewProduction()
	return logger
}

func RegisterHooks(lifecycle fx.Lifecycle, logger *zap.Logger, cfg Config, grpcServer *presentation.TokenGrpcServer) {
	var opts []grpc.ServerOption
	if cfg.CertFile != "" && cfg.KeyFile != "" {
		creds, err := credentials.NewServerTLSFromFile(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			logger.Fatal("Failed to setup TLS", zap.Error(err))
		}
		opts = append(opts, grpc.Creds(creds))
		logger.Info("TLS enabled for gRPC Token server")
	}

	opts = append(opts, grpc.StatsHandler(otelgrpc.NewServerHandler()))

	s := grpc.NewServer(opts...)
	proto.RegisterTokenServiceServer(s, grpcServer)

	lifecycle.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			lis, err := net.Listen("tcp", fmt.Sprintf(":%s", cfg.Port))
			if err != nil {
				return err
			}
			logger.Info("Iniciando gRPC Token Service", zap.String("porta", cfg.Port))
			go s.Serve(lis)
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("Encerrando gRPC Token Service...")
			s.GracefulStop()
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
		),
		token.Module,
		fx.Invoke(func(tp *trace.TracerProvider) {}),
		fx.Invoke(RegisterHooks),
	)

	app.Run()
}
