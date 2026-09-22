package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/amarin/genodex/internal/models"
)

type fakeStore struct {
	owners   map[ID]Owner
	sessions map[ID]Session
	tokens   map[ID]APIToken
	invites  map[ID]Invite
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		owners: map[ID]Owner{}, sessions: map[ID]Session{},
		tokens: map[ID]APIToken{}, invites: map[ID]Invite{},
	}
}

func (f *fakeStore) CreateOwner(_ context.Context, o Owner) error {
	f.owners[o.ID] = o

	return nil
}

func (f *fakeStore) GetOwnerByLogin(_ context.Context, login string) (*Owner, error) {
	for _, o := range f.owners {
		if o.Login == login {
			cp := o

			return &cp, nil
		}
	}

	return nil, ErrNotFound
}

func (f *fakeStore) GetOwner(_ context.Context, id ID) (*Owner, error) {
	o, ok := f.owners[id]
	if !ok {
		return nil, ErrNotFound
	}

	return &o, nil
}

func (f *fakeStore) UpdateOwnerPassword(_ context.Context, id ID, hash string) error {
	o, ok := f.owners[id]
	if !ok {
		return ErrNotFound
	}

	o.PasswordHash = hash
	f.owners[id] = o

	return nil
}

func (f *fakeStore) CountOwners(_ context.Context) (int, error) {
	return len(f.owners), nil
}

func (f *fakeStore) CreateSession(_ context.Context, s Session) error {
	f.sessions[s.ID] = s

	return nil
}

func (f *fakeStore) GetSessionByAccessHash(_ context.Context, hash string) (*Session, error) {
	for _, s := range f.sessions {
		if s.AccessTokenHash == hash {
			cp := s

			return &cp, nil
		}
	}

	return nil, ErrNotFound
}

func (f *fakeStore) GetSessionByRefreshHash(_ context.Context, hash string) (*Session, error) {
	for _, s := range f.sessions {
		if s.RefreshTokenHash == hash {
			cp := s

			return &cp, nil
		}
	}

	return nil, ErrNotFound
}

func (f *fakeStore) ReplaceSession(_ context.Context, id ID, s Session) error {
	if _, ok := f.sessions[id]; !ok {
		return ErrNotFound
	}

	f.sessions[id] = s

	return nil
}

func (f *fakeStore) DeleteSession(_ context.Context, id ID) error {
	delete(f.sessions, id)

	return nil
}

func (f *fakeStore) CreateAPIToken(_ context.Context, t APIToken) error {
	f.tokens[t.ID] = t

	return nil
}

func (f *fakeStore) GetAPIToken(_ context.Context, id ID) (*APIToken, error) {
	t, ok := f.tokens[id]
	if !ok {
		return nil, ErrNotFound
	}

	return &t, nil
}

func (f *fakeStore) GetAPITokenByHash(_ context.Context, hash string) (*APIToken, error) {
	for _, t := range f.tokens {
		if t.TokenHash == hash {
			cp := t

			return &cp, nil
		}
	}

	return nil, ErrNotFound
}

func (f *fakeStore) ListAPITokens(_ context.Context, ownerID ID) ([]APIToken, error) {
	var out []APIToken

	for _, t := range f.tokens {
		if t.OwnerID == ownerID {
			out = append(out, t)
		}
	}

	return out, nil
}

func (f *fakeStore) RevokeAPIToken(_ context.Context, id ID) error {
	t, ok := f.tokens[id]
	if !ok {
		return ErrNotFound
	}

	now := time.Now()
	t.RevokedAt = &now
	f.tokens[id] = t

	return nil
}

func (f *fakeStore) TouchAPIToken(_ context.Context, id ID) error {
	t, ok := f.tokens[id]
	if !ok {
		return ErrNotFound
	}

	now := time.Now()
	t.LastUsedAt = &now
	f.tokens[id] = t

	return nil
}

func (f *fakeStore) CreateInvite(_ context.Context, i Invite) error {
	f.invites[i.ID] = i

	return nil
}

func (f *fakeStore) GetInviteByHash(_ context.Context, hash string) (*Invite, error) {
	for _, i := range f.invites {
		if i.TokenHash == hash {
			cp := i

			return &cp, nil
		}
	}

	return nil, ErrNotFound
}

func (f *fakeStore) MarkInviteUsed(_ context.Context, id, by ID) error {
	i, ok := f.invites[id]
	if !ok {
		return ErrNotFound
	}

	now := time.Now()
	i.UsedAt = &now
	i.UsedBy = &by
	f.invites[id] = i

	return nil
}

var _ Store = (*fakeStore)(nil)

func newTestService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

func TestServiceRegisterBootstrapNoInviteNeeded(t *testing.T) {
	svc := newTestService(newFakeStore())

	res, err := svc.Register(context.Background(), "first", "password123", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Fatal("пустые токены после регистрации")
	}
}

func TestServiceRegisterAfterBootstrapRequiresInvite(t *testing.T) {
	store := newFakeStore()
	svc := newTestService(store)
	ctx := context.Background()

	if _, err := svc.Register(ctx, "first", "password123", nil); err != nil {
		t.Fatalf("bootstrap Register: %v", err)
	}

	if _, err := svc.Register(ctx, "second", "password123", nil); !errors.Is(err, ErrInviteRequired) {
		t.Fatalf("err = %v, ожидался ErrInviteRequired", err)
	}
}

func TestServiceRegisterWithValidInvite(t *testing.T) {
	store := newFakeStore()
	svc := newTestService(store)
	ctx := context.Background()

	first, err := svc.Register(ctx, "first", "password123", nil)
	if err != nil {
		t.Fatalf("bootstrap Register: %v", err)
	}

	raw, err := svc.CreateInvite(ctx, first.OwnerID)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	if _, err := svc.Register(ctx, "second", "password123", &raw); err != nil {
		t.Fatalf("Register по invite: %v", err)
	}

	// повторное использование той же ссылки — ошибка
	if _, err := svc.Register(ctx, "third", "password123", &raw); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("повторный invite: err = %v, ожидался ErrInviteInvalid", err)
	}
}

func TestServiceRegisterExpiredInviteFails(t *testing.T) {
	store := newFakeStore()
	svc := newTestService(store)
	ctx := context.Background()

	first, _ := svc.Register(ctx, "first", "password123", nil)
	raw, _ := svc.CreateInvite(ctx, first.OwnerID)

	svc.now = func() time.Time { return time.Now().Add(inviteTTL + time.Hour) }

	if _, err := svc.Register(ctx, "second", "password123", &raw); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("err = %v, ожидался ErrInviteInvalid", err)
	}
}

func TestServiceRegisterDuplicateLoginFails(t *testing.T) {
	svc := newTestService(newFakeStore())
	ctx := context.Background()

	if _, err := svc.Register(ctx, "dup", "password123", nil); err != nil {
		t.Fatalf("Register: %v", err)
	}

	store := svc.store.(*fakeStore)
	raw, _ := svc.CreateInvite(ctx, func() ID {
		for id := range store.owners {
			return id
		}

		return ""
	}())

	if _, err := svc.Register(ctx, "dup", "password123", &raw); !errors.Is(err, ErrLoginTaken) {
		t.Fatalf("err = %v, ожидался ErrLoginTaken", err)
	}
}

func TestServiceLoginSuccessAndFailure(t *testing.T) {
	svc := newTestService(newFakeStore())
	ctx := context.Background()

	if _, err := svc.Register(ctx, "user", "correct-password", nil); err != nil {
		t.Fatalf("Register: %v", err)
	}

	if _, err := svc.Login(ctx, "user", "correct-password"); err != nil {
		t.Fatalf("Login: %v", err)
	}

	if _, err := svc.Login(ctx, "user", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("неверный пароль: err = %v, ожидался ErrInvalidCredentials", err)
	}

	if _, err := svc.Login(ctx, "nobody", "whatever"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("неизвестный логин: err = %v, ожидался ErrInvalidCredentials", err)
	}
}

func TestServiceRefreshRotatesAndRejectsReuse(t *testing.T) {
	svc := newTestService(newFakeStore())
	ctx := context.Background()

	first, err := svc.Register(ctx, "user", "password123", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	rotated, err := svc.Refresh(ctx, first.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if rotated.AccessToken == first.AccessToken || rotated.RefreshToken == first.RefreshToken {
		t.Fatal("Refresh не сменил токены")
	}

	if _, err := svc.Refresh(ctx, first.RefreshToken); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("повтор старого refresh: err = %v, ожидался ErrSessionExpired", err)
	}

	if _, err := svc.Refresh(ctx, rotated.RefreshToken); err != nil {
		t.Fatalf("новый refresh должен работать: %v", err)
	}
}

func TestServiceRefreshExpiredFails(t *testing.T) {
	svc := newTestService(newFakeStore())
	ctx := context.Background()

	res, err := svc.Register(ctx, "user", "password123", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	svc.now = func() time.Time { return time.Now().Add(refreshTokenTTL + time.Hour) }

	if _, err := svc.Refresh(ctx, res.RefreshToken); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("err = %v, ожидался ErrSessionExpired", err)
	}
}

func TestServiceLogoutThenResolveAccessIsPublic(t *testing.T) {
	svc := newTestService(newFakeStore())
	ctx := context.Background()

	res, err := svc.Register(ctx, "user", "password123", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	access, owner, err := svc.ResolveAccess(ctx, res.AccessToken)
	if err != nil || access != models.AccessFull || owner == nil {
		t.Fatalf("до logout: access=%v owner=%v err=%v", access, owner, err)
	}

	if err := svc.Logout(ctx, res.AccessToken); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	access, owner, err = svc.ResolveAccess(ctx, res.AccessToken)
	if err != nil || access != models.AccessPublic || owner != nil {
		t.Fatalf("после logout: access=%v owner=%v err=%v", access, owner, err)
	}
}

func TestServiceResolveAccessEmptyAndExpired(t *testing.T) {
	svc := newTestService(newFakeStore())
	ctx := context.Background()

	if access, owner, err := svc.ResolveAccess(ctx, ""); err != nil || access != models.AccessPublic || owner != nil {
		t.Fatalf("пустой токен: access=%v owner=%v err=%v", access, owner, err)
	}

	res, err := svc.Register(ctx, "user", "password123", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	svc.now = func() time.Time { return time.Now().Add(accessTokenTTL + time.Hour) }

	if access, owner, err := svc.ResolveAccess(ctx, res.AccessToken); err != nil || access != models.AccessPublic || owner != nil {
		t.Fatalf("просроченный access: access=%v owner=%v err=%v", access, owner, err)
	}
}

func TestServiceChangePassword(t *testing.T) {
	svc := newTestService(newFakeStore())
	ctx := context.Background()

	res, err := svc.Register(ctx, "user", "old-password", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if err := svc.ChangePassword(ctx, res.OwnerID, "wrong", "new-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("неверный текущий пароль: err = %v", err)
	}

	if err := svc.ChangePassword(ctx, res.OwnerID, "old-password", "new-password"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	if _, err := svc.Login(ctx, "user", "new-password"); err != nil {
		t.Fatalf("логин новым паролем: %v", err)
	}
	if _, err := svc.Login(ctx, "user", "old-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal("старый пароль всё ещё работает")
	}
}

func TestServiceAPITokenLifecycle(t *testing.T) {
	svc := newTestService(newFakeStore())
	ctx := context.Background()

	owner, err := svc.Register(ctx, "user", "password123", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	raw, id, err := svc.CreateAPIToken(ctx, owner.OwnerID, "MCP")
	if err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}

	resolved, err := svc.ResolveAPIToken(ctx, raw)
	if err != nil || resolved != owner.OwnerID {
		t.Fatalf("ResolveAPIToken: owner=%v err=%v", resolved, err)
	}

	list, err := svc.ListAPITokens(ctx, owner.OwnerID)
	if err != nil || len(list) != 1 || list[0].ID != id {
		t.Fatalf("ListAPITokens: %+v, err=%v", list, err)
	}

	if err := svc.RevokeAPIToken(ctx, owner.OwnerID, id); err != nil {
		t.Fatalf("RevokeAPIToken: %v", err)
	}

	if _, err := svc.ResolveAPIToken(ctx, raw); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("отозванный токен: err = %v, ожидался ErrInvalidCredentials", err)
	}
}

func TestServiceRevokeAPITokenWrongOwnerFails(t *testing.T) {
	svc := newTestService(newFakeStore())
	ctx := context.Background()

	owner1, _ := svc.Register(ctx, "user1", "password123", nil)
	raw, _ := svc.CreateInvite(ctx, owner1.OwnerID)
	owner2, err := svc.Register(ctx, "user2", "password123", &raw)
	if err != nil {
		t.Fatalf("Register user2: %v", err)
	}

	_, id, err := svc.CreateAPIToken(ctx, owner1.OwnerID, "MCP")
	if err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}

	if err := svc.RevokeAPIToken(ctx, owner2.OwnerID, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("чужой токен: err = %v, ожидался ErrNotFound", err)
	}
}

func TestServiceBootstrap(t *testing.T) {
	store := newFakeStore()
	svc := newTestService(store)
	ctx := context.Background()

	boot, err := svc.Bootstrap(ctx)
	if err != nil || !boot {
		t.Fatalf("до первого владельца: boot=%v err=%v", boot, err)
	}

	if _, err := svc.Register(ctx, "user", "password123", nil); err != nil {
		t.Fatalf("Register: %v", err)
	}

	boot, err = svc.Bootstrap(ctx)
	if err != nil || boot {
		t.Fatalf("после первого владельца: boot=%v err=%v", boot, err)
	}
}
