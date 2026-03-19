package presentation

import (
	"context"

	"github.com/jvlerner/auth-system/internal/token/application"
	"github.com/jvlerner/auth-system/internal/token/proto"
)

type TokenGrpcServer struct {
	proto.UnimplementedTokenServiceServer
	service *application.TokenService
}

func NewTokenGrpcServer(service *application.TokenService) *TokenGrpcServer {
	return &TokenGrpcServer{
		service: service,
	}
}

func (s *TokenGrpcServer) Generate(ctx context.Context, req *proto.GenerateTokenRequest) (*proto.GenerateTokenResponse, error) {
	token, exp, err := s.service.GenerateToken(req.UserId, req.Email, req.Roles)
	if err != nil {
		return nil, err
	}

	return &proto.GenerateTokenResponse{
		AccessToken: token,
		ExpiresIn:   exp,
	}, nil
}

func (s *TokenGrpcServer) Validate(ctx context.Context, req *proto.ValidateTokenRequest) (*proto.ValidateTokenResponse, error) {
	valid, userID, email, roles, err := s.service.ValidateToken(req.Token)
	if err != nil {
		return nil, err
	}

	return &proto.ValidateTokenResponse{
		Valid:  valid,
		UserId: userID,
		Email:  email,
		Roles:  roles,
	}, nil
}
