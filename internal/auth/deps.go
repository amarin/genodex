package auth

import "context"

// Store — узкий порт auth: свои таблицы, не пересекается с internal/store
// (см. auth.md §3 — не входит в generic 21-сущностную систему).
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package auth
type Store interface {
	CreateOwner(ctx context.Context, o Owner) error
	GetOwnerByLogin(ctx context.Context, login string) (*Owner, error)
	GetOwner(ctx context.Context, id ID) (*Owner, error)
	UpdateOwnerPassword(ctx context.Context, id ID, hash string) error
	CountOwners(ctx context.Context) (int, error)

	CreateSession(ctx context.Context, s Session) error
	GetSessionByAccessHash(ctx context.Context, hash string) (*Session, error)
	GetSessionByRefreshHash(ctx context.Context, hash string) (*Session, error)
	ReplaceSession(ctx context.Context, id ID, s Session) error
	DeleteSession(ctx context.Context, id ID) error

	CreateAPIToken(ctx context.Context, t APIToken) error
	GetAPIToken(ctx context.Context, id ID) (*APIToken, error)
	GetAPITokenByHash(ctx context.Context, hash string) (*APIToken, error)
	ListAPITokens(ctx context.Context, ownerID ID) ([]APIToken, error)
	RevokeAPIToken(ctx context.Context, id ID) error
	TouchAPIToken(ctx context.Context, id ID) error

	CreateInvite(ctx context.Context, i Invite) error
	GetInviteByHash(ctx context.Context, hash string) (*Invite, error)
	MarkInviteUsed(ctx context.Context, id ID, by ID) error
}
