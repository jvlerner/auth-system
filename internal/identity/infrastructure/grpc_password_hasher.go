package infrastructure

import (
	"context"

	"github.com/jvlerner/auth-system/internal/identity/domain"
	"github.com/jvlerner/auth-system/internal/password/proto"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// GrpcPasswordHasher implementa a interface de domínio delegando pra rede.
type GrpcPasswordHasher struct {
	client proto.PasswordServiceClient
}

func NewGrpcPasswordHasher(grpcUrl, certFile string) (*GrpcPasswordHasher, error) {
	opts := []grpc.DialOption{}
	
	if certFile != "" {
		creds, err := credentials.NewClientTLSFromFile(certFile, "")
		if err != nil {
			return nil, err
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	opts = append(opts, grpc.WithStatsHandler(otelgrpc.NewClientHandler()))

	conn, err := grpc.NewClient(grpcUrl, opts...)
	if err != nil {
		return nil, err
	}

	client := proto.NewPasswordServiceClient(conn)

	return &GrpcPasswordHasher{
		client: client,
	}, nil
}

func (h *GrpcPasswordHasher) Hash(ctx context.Context, plainText string) (domain.Password, error) {
	resp, err := h.client.Hash(ctx, &proto.HashRequest{PlainText: plainText})
	if err != nil {
		return domain.Password{}, err
	}

	return domain.RestorePassword(resp.Hash)
}

func (h *GrpcPasswordHasher) Compare(ctx context.Context, plainText string, hashed domain.Password) (bool, error) {
	// Chamaremos .String() ou a exportação padrão do seu VO:
	resp, err := h.client.Compare(ctx, &proto.CompareRequest{PlainText: plainText, Hash: hashed.Value()})
	if err != nil {
		return false, err
	}
	if !resp.Match {
		return false, domain.ErrInvalidCredentials
	}

	return resp.NeedsUpgrade, nil
}
