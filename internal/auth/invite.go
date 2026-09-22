package auth

import "time"

// Invite — одноразовая ссылка регистрации совладельца.
type Invite struct {
	ID        ID
	CreatedBy ID
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	UsedBy    *ID
	CreatedAt time.Time
}

// Validate проверяет формат id, непустой хеш и заданный срок действия.
func (i Invite) Validate() error {
	if e := idFieldErr("id", i.ID, KindInvite); e != nil {
		return e
	}
	if e := idFieldErr("created_by", i.CreatedBy, KindOwner); e != nil {
		return e
	}
	if i.TokenHash == "" {
		return fieldErr("token_hash", "не может быть пустым")
	}
	if i.ExpiresAt.IsZero() {
		return fieldErr("expires_at", "не может быть пустым")
	}

	return nil
}
