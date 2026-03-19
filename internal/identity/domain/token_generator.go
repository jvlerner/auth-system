package domain

import "context"

type Token struct {
	AccessToken string
	ExpiresIn   int64
}

// TokenGenerator é o Port da camada de Domínio que define a geração de tokens de sessão.
// A implementação real fará a chamada gRPC para o serviço de Tokens.
type TokenGenerator interface {
	Generate(ctx context.Context, userID, email string, roles []string) (*Token, error)
}
