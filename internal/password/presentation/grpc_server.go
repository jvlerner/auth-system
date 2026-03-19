package presentation

import (
	"context"
	"fmt"
	"strings"

	"github.com/jvlerner/auth-system/internal/password/application"
	"github.com/jvlerner/auth-system/internal/password/proto"
)

// GrpcServer implementa a interface proto gerada e roteia as chamadas
// para a implementação correta de HashService com base na versão do hash.
type GrpcServer struct {
	proto.UnimplementedPasswordServiceServer
	services map[string]application.HashService
	// currentVersion define a versão a ser utilizada para novos hashes
	currentVersion string
}

func NewGrpcServer(services ...application.HashService) *GrpcServer {
	svcMap := make(map[string]application.HashService, len(services))
	var latest string
	for _, svc := range services {
		svcMap[svc.Version()] = svc
		latest = svc.Version() // último registrado é o mais novo
	}
	return &GrpcServer{
		services:       svcMap,
		currentVersion: latest,
	}
}

func (s *GrpcServer) Hash(ctx context.Context, req *proto.HashRequest) (*proto.HashResponse, error) {
	if req.PlainText == "" {
		return nil, fmt.Errorf("plain_text não pode ser vazio")
	}

	svc := s.services[s.currentVersion]
	hashed, err := svc.Hash(ctx, req.PlainText)
	if err != nil {
		return nil, err
	}

	return &proto.HashResponse{Hash: hashed}, nil
}

func (s *GrpcServer) Compare(ctx context.Context, req *proto.CompareRequest) (*proto.CompareResponse, error) {
	// Detecta a versão a partir do prefixo do hash armazenado (ex: "v1:$argon2id...")
	hashVersion := detectVersion(req.Hash)

	svc, ok := s.services[hashVersion]
	if !ok {
		return nil, fmt.Errorf("serviço para hash versão '%s' não encontrado", hashVersion)
	}

	match, err := svc.Compare(ctx, req.PlainText, req.Hash)
	if err != nil {
		return nil, err
	}

	// Sinaliza upgrade se a versão do hash for diferente da versão atual do servidor
	needsUpgrade := hashVersion != s.currentVersion

	return &proto.CompareResponse{
		Match:        match,
		NeedsUpgrade: needsUpgrade,
	}, nil
}

// detectVersion extrai o prefixo de versão do hash armazenado.
// Hashes sem prefixo assumem "v1" como legado.
func detectVersion(encodedHash string) string {
	if idx := strings.Index(encodedHash, ":"); idx > 0 {
		return encodedHash[:idx]
	}
	return "v1" // legado: sem prefixo = v1
}
