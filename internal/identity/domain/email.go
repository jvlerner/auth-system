package domain

import (
	"errors"
	"regexp"
	"strings"
)

var (
	ErrInvalidEmail = errors.New("invalid email address format")
)

var emailRegex = regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,4}$`)

// Email é um Value Object representando o endereço de e-mail de um usuário
type Email struct {
	value string
}

// NewEmail cria um novo Value Object Email após validar seu formato
func NewEmail(address string) (Email, error) {
	address = strings.TrimSpace(strings.ToLower(address))
	if !emailRegex.MatchString(address) {
		return Email{}, ErrInvalidEmail
	}
	return Email{value: address}, nil
}

// Value retorna o valor primitivo do e-mail
func (e Email) Value() string {
	return e.value
}
