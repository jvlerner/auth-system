package application

import "context"

// HashService é a interface que define o contrato para todas as implementações
// de algoritmos de hashing. Novas versões (V2, V3...) implementarão esta interface.
type HashService interface {
	// Hash gera um hash seguro a partir de uma senha em texto puro.
	Hash(ctx context.Context, plainText string) (string, error)

	// Compare verifica se a senha em texto puro corresponde ao hash armazenado.
	// O hash armazenado deve ter sido gerado por este mesmo serviço.
	Compare(ctx context.Context, plainText, encodedHash string) (bool, error)

	// Version retorna o identificador da versão deste serviço (ex: "v1", "v2").
	Version() string
}
