package domain

import "context"

// PasswordHasher é o "Port" da camada de Domínio focado em operações CPU-bound.
// Em nossa arquitetura, a implementação concreta dessa interface fará chamadas gRPC
// para um microserviço isolado rodando o Argon2id (grpc-password).
type PasswordHasher interface {
	// Hash converte uma string original em um hash seguro e retorna o Value Object Password
	Hash(ctx context.Context, plainText string) (Password, error)

	// Compare compara a senha em texto puro com o Value Object de Senha criptografada
	// Retorna um booleano `needsUpgrade` sinalizando que o hash antigo deve ser requalificado.
	Compare(ctx context.Context, plainText string, hashed Password) (bool, error)
}
