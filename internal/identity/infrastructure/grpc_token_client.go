package infrastructure

import (
	"context"

	"github.com/jvlerner/auth-system/internal/identity/domain"
	"github.com/jvlerner/auth-system/internal/token/proto"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// GrpcTokenClient implementa domain.TokenGenerator delegando para o microserviço grpc-token.
type GrpcTokenClient struct {
	client proto.TokenServiceClient
}

func NewGrpcTokenClient(grpcUrl, certFile string) (*GrpcTokenClient, error) {
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

	client := proto.NewTokenServiceClient(conn)

	return &GrpcTokenClient{
		client: client,
	}, nil
}

func (c *GrpcTokenClient) Generate(ctx context.Context, userID, email string, roles []string) (*domain.Token, error) {
	resp, err := c.client.Generate(ctx, &proto.GenerateTokenRequest{
		UserId: userID,
		Email:  email,
		Roles:  roles,
	})
	if err != nil {
		return nil, err
	}

	return &domain.Token{
		AccessToken: resp.AccessToken,
		ExpiresIn:   resp.ExpiresIn,
	}, nil
}
