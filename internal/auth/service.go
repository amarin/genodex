package auth

import (
	"context"
	"errors"
	"time"

	"github.com/amarin/genodex/internal/models"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 30 * 24 * time.Hour
	inviteTTL       = 7 * 24 * time.Hour
)

// AuthResult — результат успешной аутентификации: сырые токены (вызывающая
// сторона — httpapi — ставит их в cookie) и срок действия.
type AuthResult struct {
	OwnerID          ID
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
}

// Service — фасад над Store: вся бизнес-логика auth в одном месте (домен
// маленький и плотно связанный — как сценарии usecases/*, но один пакет).
type Service struct {
	store Store
	now   func() time.Time
}

// New создаёт сервис на системных часах.
func New(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

// Bootstrap сообщает, есть ли хоть один владелец (false — обычная работа,
// true — веб должен предложить мастер регистрации без invite).
func (s *Service) Bootstrap(ctx context.Context) (bool, error) {
	count, err := s.store.CountOwners(ctx)
	if err != nil {
		return false, err
	}

	return count == 0, nil
}

// Register создаёт владельца: до первого владельца invite не нужен
// (bootstrap), после — обязателен, валиден, не использован и не истёк.
func (s *Service) Register(ctx context.Context, login, password string, invite *string) (AuthResult, error) {
	count, err := s.store.CountOwners(ctx)
	if err != nil {
		return AuthResult{}, err
	}

	var usedInvite *Invite

	if count > 0 {
		usedInvite, err = s.checkInvite(ctx, invite)
		if err != nil {
			return AuthResult{}, err
		}
	}

	if _, err := s.store.GetOwnerByLogin(ctx, login); err == nil {
		return AuthResult{}, ErrLoginTaken
	} else if !errors.Is(err, ErrNotFound) {
		return AuthResult{}, err
	}

	hash, err := hashPassword(password)
	if err != nil {
		return AuthResult{}, err
	}

	id, err := newID(KindOwner)
	if err != nil {
		return AuthResult{}, err
	}

	owner := Owner{ID: id, Login: login, PasswordHash: hash, CreatedAt: s.now()}
	if err := owner.Validate(); err != nil {
		return AuthResult{}, err
	}

	if err := s.store.CreateOwner(ctx, owner); err != nil {
		return AuthResult{}, err
	}

	if usedInvite != nil {
		if err := s.store.MarkInviteUsed(ctx, usedInvite.ID, owner.ID); err != nil {
			return AuthResult{}, err
		}
	}

	return s.newSession(ctx, owner.ID)
}

// checkInvite проверяет сырое значение invite-ссылки.
func (s *Service) checkInvite(ctx context.Context, raw *string) (*Invite, error) {
	if raw == nil || *raw == "" {
		return nil, ErrInviteRequired
	}

	inv, err := s.store.GetInviteByHash(ctx, hashToken(*raw))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrInviteInvalid
		}

		return nil, err
	}

	if inv.UsedAt != nil || !inv.ExpiresAt.After(s.now()) {
		return nil, ErrInviteInvalid
	}

	return inv, nil
}

// Login проверяет логин/пароль и заводит новую сессию. Неизвестный логин и
// неверный пароль дают один и тот же ErrInvalidCredentials (защита от
// перебора логинов).
func (s *Service) Login(ctx context.Context, login, password string) (AuthResult, error) {
	owner, err := s.store.GetOwnerByLogin(ctx, login)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return AuthResult{}, ErrInvalidCredentials
		}

		return AuthResult{}, err
	}

	if !verifyPassword(owner.PasswordHash, password) {
		return AuthResult{}, ErrInvalidCredentials
	}

	return s.newSession(ctx, owner.ID)
}

// Refresh ротирует access и refresh по действующему refresh-токену; старый
// refresh перестаёт резолвиться сам собой (ReplaceSession перезаписывает
// хеши той же строки) — повторное использование украденного токена даёт
// ErrSessionExpired.
func (s *Service) Refresh(ctx context.Context, rawRefresh string) (AuthResult, error) {
	session, err := s.store.GetSessionByRefreshHash(ctx, hashToken(rawRefresh))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return AuthResult{}, ErrSessionExpired
		}

		return AuthResult{}, err
	}

	if !session.RefreshExpiresAt.After(s.now()) {
		return AuthResult{}, ErrSessionExpired
	}

	return s.rotateSession(ctx, *session)
}

// Logout удаляет сессию по access-токену; нет такой сессии — не ошибка
// (уже разлогинен).
func (s *Service) Logout(ctx context.Context, rawAccess string) error {
	session, err := s.store.GetSessionByAccessHash(ctx, hashToken(rawAccess))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}

		return err
	}

	return s.store.DeleteSession(ctx, session.ID)
}

// ResolveAccess — режим доступа по access-cookie: пусто, не найдена или
// просрочена — models.AccessPublic без ошибки (это не ошибка запроса, а
// нормальный анонимный путь); валидная — models.AccessFull и OwnerID.
func (s *Service) ResolveAccess(ctx context.Context, rawAccess string) (models.Access, *ID, error) {
	if rawAccess == "" {
		return models.AccessPublic, nil, nil
	}

	session, err := s.store.GetSessionByAccessHash(ctx, hashToken(rawAccess))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return models.AccessPublic, nil, nil
		}

		return models.AccessPublic, nil, err
	}

	if !session.AccessExpiresAt.After(s.now()) {
		return models.AccessPublic, nil, nil
	}

	owner := session.OwnerID

	return models.AccessFull, &owner, nil
}

// ChangePassword проверяет текущий пароль и сохраняет новый хеш.
func (s *Service) ChangePassword(ctx context.Context, ownerID ID, current, newPassword string) error {
	owner, err := s.store.GetOwner(ctx, ownerID)
	if err != nil {
		return err
	}

	if !verifyPassword(owner.PasswordHash, current) {
		return ErrInvalidCredentials
	}

	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}

	return s.store.UpdateOwnerPassword(ctx, ownerID, hash)
}

// CreateInvite создаёт одноразовую ссылку (срок — inviteTTL) и возвращает
// сырое значение (только здесь — дальше хранится лишь хеш).
func (s *Service) CreateInvite(ctx context.Context, ownerID ID) (string, error) {
	id, err := newID(KindInvite)
	if err != nil {
		return "", err
	}

	raw, err := newRawToken()
	if err != nil {
		return "", err
	}

	invite := Invite{
		ID: id, CreatedBy: ownerID, TokenHash: hashToken(raw),
		ExpiresAt: s.now().Add(inviteTTL), CreatedAt: s.now(),
	}
	if err := invite.Validate(); err != nil {
		return "", err
	}

	if err := s.store.CreateInvite(ctx, invite); err != nil {
		return "", err
	}

	return raw, nil
}

// CreateAPIToken создаёт долгоживущий токен для MCP/скриптов; сырое
// значение — только здесь.
func (s *Service) CreateAPIToken(ctx context.Context, ownerID ID, label string) (string, ID, error) {
	id, err := newID(KindAPIToken)
	if err != nil {
		return "", "", err
	}

	raw, err := newAPIToken()
	if err != nil {
		return "", "", err
	}

	token := APIToken{ID: id, OwnerID: ownerID, Label: label, TokenHash: hashToken(raw), CreatedAt: s.now()}
	if err := token.Validate(); err != nil {
		return "", "", err
	}

	if err := s.store.CreateAPIToken(ctx, token); err != nil {
		return "", "", err
	}

	return raw, id, nil
}

// ListAPITokens отдаёт метаданные токенов владельца (без сырых значений —
// они не хранятся).
func (s *Service) ListAPITokens(ctx context.Context, ownerID ID) ([]APIToken, error) {
	return s.store.ListAPITokens(ctx, ownerID)
}

// RevokeAPIToken отзывает токен, если он принадлежит ownerID; чужой токен —
// ErrNotFound (не подтверждаем существование чужого токена).
func (s *Service) RevokeAPIToken(ctx context.Context, ownerID, tokenID ID) error {
	token, err := s.store.GetAPIToken(ctx, tokenID)
	if err != nil {
		return err
	}

	if token.OwnerID != ownerID {
		return ErrNotFound
	}

	return s.store.RevokeAPIToken(ctx, tokenID)
}

// ResolveAPIToken — для MCP-middleware: сырой bearer-токен → OwnerID.
// Отозванный или неизвестный — ErrInvalidCredentials.
func (s *Service) ResolveAPIToken(ctx context.Context, raw string) (ID, error) {
	token, err := s.store.GetAPITokenByHash(ctx, hashToken(raw))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", ErrInvalidCredentials
		}

		return "", err
	}

	if token.RevokedAt != nil {
		return "", ErrInvalidCredentials
	}

	if err := s.store.TouchAPIToken(ctx, token.ID); err != nil {
		return "", err
	}

	return token.OwnerID, nil
}

// newSession заводит новую сессию для ownerID.
func (s *Service) newSession(ctx context.Context, ownerID ID) (AuthResult, error) {
	id, err := newID(KindSession)
	if err != nil {
		return AuthResult{}, err
	}

	return s.saveSession(ctx, Session{ID: id, OwnerID: ownerID, CreatedAt: s.now()}, s.store.CreateSession)
}

// rotateSession перевыпускает токены существующей сессии (Refresh).
func (s *Service) rotateSession(ctx context.Context, existing Session) (AuthResult, error) {
	replace := func(ctx context.Context, sess Session) error {
		return s.store.ReplaceSession(ctx, existing.ID, sess)
	}

	return s.saveSession(ctx, existing, replace)
}

// saveSession генерирует новую пару токенов для session (ID/OwnerID/CreatedAt
// уже заполнены вызывающим) и сохраняет через save.
func (s *Service) saveSession(
	ctx context.Context, session Session, save func(context.Context, Session) error,
) (AuthResult, error) {
	rawAccess, err := newRawToken()
	if err != nil {
		return AuthResult{}, err
	}

	rawRefresh, err := newRawToken()
	if err != nil {
		return AuthResult{}, err
	}

	now := s.now()
	session.AccessTokenHash = hashToken(rawAccess)
	session.AccessExpiresAt = now.Add(accessTokenTTL)
	session.RefreshTokenHash = hashToken(rawRefresh)
	session.RefreshExpiresAt = now.Add(refreshTokenTTL)

	if err := session.Validate(); err != nil {
		return AuthResult{}, err
	}

	if err := save(ctx, session); err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		OwnerID:     session.OwnerID,
		AccessToken: rawAccess, AccessExpiresAt: session.AccessExpiresAt,
		RefreshToken: rawRefresh, RefreshExpiresAt: session.RefreshExpiresAt,
	}, nil
}
