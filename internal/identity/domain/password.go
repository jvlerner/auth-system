package domain

import "errors"

// Password é um Value Object que armazena, de forma segura, o hash da senha de um usuário.
// O domínio NUNCA deve trafegar senhas em texto puro.
type Password struct {
	hashedValue string
}

// RestorePassword recria o Value Object a partir de um hash já existente no banco.
func RestorePassword(hash string) (Password, error) {
	if hash == "" {
		return Password{}, errors.New("hashed password cannot be empty")
	}
	return Password{hashedValue: hash}, nil
}

// Hash retorna o valor com hash blindado
func (p Password) Hash() string {
	return p.hashedValue
}

// Value retorna a string crua (útil pra transporte via grpc request)
func (p Password) Value() string {
	return p.hashedValue
}
