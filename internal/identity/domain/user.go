package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrUserAlreadyExists  = errors.New("user already exists with this email")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
)

// User representa a raiz de agregação (Aggregate Root) do Bounded Context Identity.
type User struct {
	id            uuid.UUID
	email         Email
	password      Password
	roles         []string
	emailVerified bool
	mfaEnabled    bool
	totpSecret    string
	createdAt     time.Time
	updatedAt     time.Time
}

// NewUser é o Factory Method para criar um usuário válido.
// Garante as invariantes do negócio logo na instanciação.
func NewUser(email Email, password Password) *User {
	now := time.Now().UTC()
	return &User{
		id:            uuid.New(),
		email:         email,
		password:      password,
		roles:         []string{"user"}, // Role default para novos cadastros
		emailVerified: false,            // Email initially unverified
		mfaEnabled:    false,
		totpSecret:    "",
		createdAt:     now,
		updatedAt:     now,
	}
}

// RestoreUser reconstrói uma entidade User a partir do banco de dados (infra).
// Não deve ser usado para criação de novos usuários no sistema.
func RestoreUser(id string, emailStr string, hashedPassword string, emailVerified bool, mfaEnabled bool, totpSecret string, createdAt, updatedAt time.Time) (*User, error) {
	parsedUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, errors.New("invalid user id format")
	}

	email, err := NewEmail(emailStr)
	if err != nil {
		return nil, err
	}

	password, err := RestorePassword(hashedPassword)
	if err != nil {
		return nil, err
	}

	return &User{
		id:            parsedUUID,
		email:         email,
		password:      password,
		roles:         []string{"user"}, // TODO: Receber do DB posteriormente nas queries
		emailVerified: emailVerified,
		mfaEnabled:    mfaEnabled,
		totpSecret:    totpSecret,
		createdAt:     createdAt,
		updatedAt:     updatedAt,
	}, nil
}

// Getters puros (imutáveis para quem consome a entidade)
func (u *User) ID() uuid.UUID {
	return u.id
}

func (u *User) Email() string {
	return u.email.Value()
}

func (u *User) HashedPassword() string {
	return u.password.Hash()
}

func (u *User) Roles() []string {
	// Retorna uma cópia para evitar mutação externa do slice
	rolesCopy := make([]string, len(u.roles))
	copy(rolesCopy, u.roles)
	return rolesCopy
}

func (u *User) CreatedAt() time.Time {
	return u.createdAt
}

func (u *User) UpdatedAt() time.Time {
	return u.updatedAt
}

// UpdatePassword altera a senha do usuário e atualiza a data de modificação.
func (u *User) UpdatePassword(newPassword Password) {
	u.password = newPassword
	u.updatedAt = time.Now().UTC()
}

// AddRole adiciona uma nova role se não existir.
func (u *User) AddRole(role string) {
	for _, r := range u.roles {
		if r == role {
			return
		}
	}
	u.roles = append(u.roles, role)
	u.updatedAt = time.Now().UTC()
}

func (u *User) IsEmailVerified() bool {
	return u.emailVerified
}
 
func (u *User) IsMFAEnabled() bool {
	return u.mfaEnabled
}

func (u *User) TOTPSecret() string {
	return u.totpSecret
}

func (u *User) EnableMFA(secret string) {
	u.mfaEnabled = true
	u.totpSecret = secret
	u.updatedAt = time.Now().UTC()
}

func (u *User) DisableMFA() {
	u.mfaEnabled = false
	u.totpSecret = ""
	u.updatedAt = time.Now().UTC()
}

func (u *User) ConfirmEmail() {
	u.emailVerified = true
	u.updatedAt = time.Now().UTC()
}

// RemoveRole remove uma role existente.
func (u *User) RemoveRole(role string) {
	var newRoles []string
	for _, r := range u.roles {
		if r != role {
			newRoles = append(newRoles, r)
		}
	}
	u.roles = newRoles
	u.updatedAt = time.Now().UTC()
}
