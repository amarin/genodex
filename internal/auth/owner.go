package auth

import (
	"strings"
	"time"
)

// Owner — владелец: логин/пароль, полный доступ (models.AccessFull).
type Owner struct {
	ID           ID
	Login        string
	PasswordHash string
	CreatedAt    time.Time
}

// Validate проверяет непустые поля и формат ID.
func (o Owner) Validate() error {
	if e := idFieldErr("id", o.ID, KindOwner); e != nil {
		return e
	}
	if strings.TrimSpace(o.Login) == "" {
		return fieldErr("login", "не может быть пустым")
	}
	if o.PasswordHash == "" {
		return fieldErr("password_hash", "не может быть пустым")
	}

	return nil
}
