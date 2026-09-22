package auth

import "context"

// Store — узкий порт auth: свои таблицы, не пересекается с internal/store
// (см. auth.md §3 — не входит в generic 21-сущностную систему).
//
// Контракт ErrNotFound: каждый метод Get* обязан возвращать пакетный
// сентинел ErrNotFound (не sql.ErrNoRows и не любой другой драйверный сигнал
// "нет строки"), когда запрошенная строка не найдена. Service полагается на
// errors.Is(err, ErrNotFound) в Login, ResolveAccess, Refresh, RevokeAPIToken
// и т. д., чтобы отличить «не найдено» (не ошибка запроса) от настоящего
// сбоя хранилища — реализация Store на другом сигнале ломает эту логику
// молча.
//
// Тот же контракт распространяется на однострочные целевые операции —
// UpdateOwnerPassword, ReplaceSession, DeleteSession, RevokeAPIToken,
// TouchAPIToken, MarkInviteUsed: если строка с указанным id не найдена (или,
// для MarkInviteUsed, уже использована), метод обязан вернуть ErrNotFound, а
// не тихо завершиться успехом. Массовые операции (DeleteSessionsByOwner) —
// исключение: ноль затронутых строк для них законный, не-ошибочный результат
// (владелец без активных сессий — обычное дело), ErrNotFound не возвращается.
//
// ListAPITokens никогда не возвращает nil-срез: владелец без токенов
// получает ненулевой срез нулевой длины ([]APIToken{}), чтобы сериализация
// в JSON давала "[]", а не "null", независимо от того, откуда идут данные.
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
	DeleteSessionsByOwner(ctx context.Context, ownerID ID) error

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
