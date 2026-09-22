package auth

import "time"

// Session — пара access/refresh токенов одного входа владельца.
type Session struct {
	ID               ID
	OwnerID          ID
	AccessTokenHash  string
	AccessExpiresAt  time.Time
	RefreshTokenHash string
	RefreshExpiresAt time.Time
	CreatedAt        time.Time
}

// Validate проверяет формат id, непустые хеши и refresh дольше access.
func (s Session) Validate() error {
	if e := idFieldErr("id", s.ID, KindSession); e != nil {
		return e
	}
	if e := idFieldErr("owner_id", s.OwnerID, KindOwner); e != nil {
		return e
	}
	if s.AccessTokenHash == "" {
		return fieldErr("access_token_hash", "не может быть пустым")
	}
	if s.RefreshTokenHash == "" {
		return fieldErr("refresh_token_hash", "не может быть пустым")
	}
	if !s.RefreshExpiresAt.After(s.AccessExpiresAt) {
		return fieldErr("refresh_expires_at", "должен быть позже access_expires_at")
	}

	return nil
}
