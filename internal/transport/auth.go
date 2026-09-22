package transport

import (
	"time"

	"github.com/amarin/genodex/internal/auth"
)

// AuthSession — ответ /api/auth/session, /register, /login, /refresh: кто
// сейчас аутентифицирован.
type AuthSession struct {
	Login string `json:"login"`
}

// AuthSessionFromOwner строит ответ из владельца.
func AuthSessionFromOwner(o *auth.Owner) AuthSession {
	return AuthSession{Login: o.Login}
}

// AuthStatus — ответ /api/auth/status.
type AuthStatus struct {
	Bootstrap bool `json:"bootstrap"`
}

// RegisterRequest — тело POST /api/auth/register (invite — query-параметр,
// не сюда).
type RegisterRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// LoginRequest — тело POST /api/auth/login.
type LoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// PasswordChangeRequest — тело POST /api/auth/password.
type PasswordChangeRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// CreateAPITokenRequest — тело POST /api/auth/tokens.
type CreateAPITokenRequest struct {
	Label string `json:"label"`
}

// APIToken — элемент ответа GET /api/auth/tokens: метаданные без сырого
// значения (его не хранит и сам auth.APIToken).
type APIToken struct {
	ID         string     `json:"id"`
	Label      string     `json:"label"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

// APITokenFromModel конвертирует одну запись.
func APITokenFromModel(t auth.APIToken) APIToken {
	return APIToken{
		ID: string(t.ID), Label: t.Label, CreatedAt: t.CreatedAt,
		LastUsedAt: t.LastUsedAt, RevokedAt: t.RevokedAt,
	}
}

// APITokensFromModels конвертирует список; пустой список — не nil.
func APITokensFromModels(list []auth.APIToken) []APIToken {
	out := make([]APIToken, len(list))
	for i, t := range list {
		out[i] = APITokenFromModel(t)
	}

	return out
}

// NewAPIToken — ответ POST /api/auth/tokens: сырое значение — только здесь.
type NewAPIToken struct {
	ID    string `json:"id"`
	Token string `json:"token"`
	Label string `json:"label"`
}

// Invite — ответ POST /api/auth/invites: сырое значение — только здесь.
type Invite struct {
	Token string `json:"token"`
}
