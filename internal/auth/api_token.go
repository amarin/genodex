package auth

import "time"

// APIToken — долгоживущий токен для MCP/скриптов, отдельно от Session.
type APIToken struct {
	ID         ID
	OwnerID    ID
	Label      string
	TokenHash  string `json:"-"`
	CreatedAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}

// Validate проверяет формат id и непустой хеш.
func (t APIToken) Validate() error {
	if e := idFieldErr("id", t.ID, KindAPIToken); e != nil {
		return e
	}
	if e := idFieldErr("owner_id", t.OwnerID, KindOwner); e != nil {
		return e
	}
	if t.TokenHash == "" {
		return fieldErr("token_hash", "не может быть пустым")
	}

	return nil
}
