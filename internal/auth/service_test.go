package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
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
	if _, ok := f.sessions[id]; !ok {
		return ErrNotFound
	}

	delete(f.sessions, id)

	return nil
}

func (f *fakeStore) DeleteSessionsByOwner(_ context.Context, ownerID ID) error {
	for id, s := range f.sessions {
		if s.OwnerID == ownerID {
			delete(f.sessions, id)
		}
	}

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
	out := []APIToken{}

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

// TestServiceRegisterBootstrapRaceOnlyOneOwnerCreated: N конкурентных
// bootstrap-регистраций (разные логины, без invite) на пустом сторе —
// ровно одна проходит, остальные получают ErrInviteRequired (мьютекс
// сериализует Register — второй и далее видят count>0). Гоняется под
// -race: fakeStore использует обычные map без своей синхронизации —
// если бы мьютекса не было или он был бы дырявым, конкурентный доступ к
// map поймал бы race detector, а не только тест упал бы по количеству.
func TestServiceRegisterBootstrapRaceOnlyOneOwnerCreated(t *testing.T) {
	svc := newTestService(newFakeStore())

	const n = 10

	var wg sync.WaitGroup

	results := make([]error, n)

	for i := 0; i < n; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			_, err := svc.Register(context.Background(), fmt.Sprintf("owner%d", i), "password123", nil)
			results[i] = err
		}(i)
	}

	wg.Wait()

	var succeeded, inviteRequired int

	for _, err := range results {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, ErrInviteRequired):
			inviteRequired++
		default:
			t.Errorf("неожиданная ошибка: %v", err)
		}
	}

	if succeeded != 1 {
		t.Fatalf("succeeded = %d, ожидалась ровно 1 (гонка должна сериализоваться)", succeeded)
	}

	if inviteRequired != n-1 {
		t.Fatalf("inviteRequired = %d, ожидалось %d", inviteRequired, n-1)
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

func TestServiceGetOwner(t *testing.T) {
	svc := newTestService(newFakeStore())
	ctx := context.Background()

	res, err := svc.Register(ctx, "user", "password123", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	owner, err := svc.GetOwner(ctx, res.OwnerID)
	if err != nil || owner.Login != "user" {
		t.Fatalf("GetOwner: %+v, %v", owner, err)
	}

	if _, err := svc.GetOwner(ctx, "OW-nonexistent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}
}

// --- Валидация пароля (находка 1) ---

func TestServiceRegisterEmptyPasswordFails(t *testing.T) {
	svc := newTestService(newFakeStore())
	ctx := context.Background()

	_, err := svc.Register(ctx, "user", "", nil)

	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("err = %v, ожидался *ValidationError", err)
	}
	if verr.Field != "password" {
		t.Fatalf("Field = %q, ожидалось %q", verr.Field, "password")
	}
}

func TestServiceRegisterPasswordTooLongFails(t *testing.T) {
	svc := newTestService(newFakeStore())
	ctx := context.Background()

	_, err := svc.Register(ctx, "user", strings.Repeat("a", 73), nil)

	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("err = %v, ожидался *ValidationError", err)
	}
	if verr.Field != "password" {
		t.Fatalf("Field = %q, ожидалось %q", verr.Field, "password")
	}
}

func TestServiceChangePasswordEmptyNewPasswordFails(t *testing.T) {
	svc := newTestService(newFakeStore())
	ctx := context.Background()

	res, err := svc.Register(ctx, "user", "password123", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	err = svc.ChangePassword(ctx, res.OwnerID, "password123", "")

	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("err = %v, ожидался *ValidationError", err)
	}
	if verr.Field != "new_password" {
		t.Fatalf("Field = %q, ожидалось %q", verr.Field, "new_password")
	}
}

func TestServiceChangePasswordNewPasswordTooLongFails(t *testing.T) {
	svc := newTestService(newFakeStore())
	ctx := context.Background()

	res, err := svc.Register(ctx, "user", "password123", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	err = svc.ChangePassword(ctx, res.OwnerID, "password123", strings.Repeat("a", 73))

	var verr *ValidationError
	if !errors.As(err, &verr) {
		t.Fatalf("err = %v, ожидался *ValidationError", err)
	}
	if verr.Field != "new_password" {
		t.Fatalf("Field = %q, ожидалось %q", verr.Field, "new_password")
	}
}

// --- ChangePassword инвалидирует прочие сессии (находка 4) ---

func TestServiceChangePasswordInvalidatesOtherSessions(t *testing.T) {
	svc := newTestService(newFakeStore())
	ctx := context.Background()

	session1, err := svc.Register(ctx, "user", "old-password", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	session2, err := svc.Login(ctx, "user", "old-password")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	for name, res := range map[string]AuthResult{"session1": session1, "session2": session2} {
		access, owner, err := svc.ResolveAccess(ctx, res.AccessToken)
		if err != nil || access != models.AccessFull || owner == nil {
			t.Fatalf("%s до ChangePassword: access=%v owner=%v err=%v", name, access, owner, err)
		}
	}

	if err := svc.ChangePassword(ctx, session1.OwnerID, "old-password", "new-password"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	for name, res := range map[string]AuthResult{"session1": session1, "session2": session2} {
		access, owner, err := svc.ResolveAccess(ctx, res.AccessToken)
		if err != nil || access != models.AccessPublic || owner != nil {
			t.Fatalf("%s после ChangePassword: access=%v owner=%v err=%v (ожидался AccessPublic)", name, access, owner, err)
		}
	}
}

// --- errStore: инъекция сбоев Store поверх fakeStore (находка 5) ---

// errStore embeds *fakeStore and lets a test force any named Store method to
// fail, to exercise the Service's non-ErrNotFound error-propagation paths
// that fakeStore alone can never reach.
type errStore struct {
	*fakeStore
	fail map[string]error
}

func newErrStore() *errStore {
	return &errStore{fakeStore: newFakeStore(), fail: map[string]error{}}
}

func (e *errStore) CreateOwner(ctx context.Context, o Owner) error {
	if err := e.fail["CreateOwner"]; err != nil {
		return err
	}

	return e.fakeStore.CreateOwner(ctx, o)
}

func (e *errStore) GetOwnerByLogin(ctx context.Context, login string) (*Owner, error) {
	if err := e.fail["GetOwnerByLogin"]; err != nil {
		return nil, err
	}

	return e.fakeStore.GetOwnerByLogin(ctx, login)
}

func (e *errStore) GetOwner(ctx context.Context, id ID) (*Owner, error) {
	if err := e.fail["GetOwner"]; err != nil {
		return nil, err
	}

	return e.fakeStore.GetOwner(ctx, id)
}

func (e *errStore) UpdateOwnerPassword(ctx context.Context, id ID, hash string) error {
	if err := e.fail["UpdateOwnerPassword"]; err != nil {
		return err
	}

	return e.fakeStore.UpdateOwnerPassword(ctx, id, hash)
}

func (e *errStore) CountOwners(ctx context.Context) (int, error) {
	if err := e.fail["CountOwners"]; err != nil {
		return 0, err
	}

	return e.fakeStore.CountOwners(ctx)
}

func (e *errStore) CreateSession(ctx context.Context, s Session) error {
	if err := e.fail["CreateSession"]; err != nil {
		return err
	}

	return e.fakeStore.CreateSession(ctx, s)
}

func (e *errStore) GetSessionByAccessHash(ctx context.Context, hash string) (*Session, error) {
	if err := e.fail["GetSessionByAccessHash"]; err != nil {
		return nil, err
	}

	return e.fakeStore.GetSessionByAccessHash(ctx, hash)
}

func (e *errStore) GetSessionByRefreshHash(ctx context.Context, hash string) (*Session, error) {
	if err := e.fail["GetSessionByRefreshHash"]; err != nil {
		return nil, err
	}

	return e.fakeStore.GetSessionByRefreshHash(ctx, hash)
}

func (e *errStore) ReplaceSession(ctx context.Context, id ID, s Session) error {
	if err := e.fail["ReplaceSession"]; err != nil {
		return err
	}

	return e.fakeStore.ReplaceSession(ctx, id, s)
}

func (e *errStore) DeleteSession(ctx context.Context, id ID) error {
	if err := e.fail["DeleteSession"]; err != nil {
		return err
	}

	return e.fakeStore.DeleteSession(ctx, id)
}

func (e *errStore) DeleteSessionsByOwner(ctx context.Context, ownerID ID) error {
	if err := e.fail["DeleteSessionsByOwner"]; err != nil {
		return err
	}

	return e.fakeStore.DeleteSessionsByOwner(ctx, ownerID)
}

func (e *errStore) CreateAPIToken(ctx context.Context, t APIToken) error {
	if err := e.fail["CreateAPIToken"]; err != nil {
		return err
	}

	return e.fakeStore.CreateAPIToken(ctx, t)
}

func (e *errStore) GetAPIToken(ctx context.Context, id ID) (*APIToken, error) {
	if err := e.fail["GetAPIToken"]; err != nil {
		return nil, err
	}

	return e.fakeStore.GetAPIToken(ctx, id)
}

func (e *errStore) GetAPITokenByHash(ctx context.Context, hash string) (*APIToken, error) {
	if err := e.fail["GetAPITokenByHash"]; err != nil {
		return nil, err
	}

	return e.fakeStore.GetAPITokenByHash(ctx, hash)
}

func (e *errStore) ListAPITokens(ctx context.Context, ownerID ID) ([]APIToken, error) {
	if err := e.fail["ListAPITokens"]; err != nil {
		return nil, err
	}

	return e.fakeStore.ListAPITokens(ctx, ownerID)
}

func (e *errStore) RevokeAPIToken(ctx context.Context, id ID) error {
	if err := e.fail["RevokeAPIToken"]; err != nil {
		return err
	}

	return e.fakeStore.RevokeAPIToken(ctx, id)
}

func (e *errStore) TouchAPIToken(ctx context.Context, id ID) error {
	if err := e.fail["TouchAPIToken"]; err != nil {
		return err
	}

	return e.fakeStore.TouchAPIToken(ctx, id)
}

func (e *errStore) CreateInvite(ctx context.Context, i Invite) error {
	if err := e.fail["CreateInvite"]; err != nil {
		return err
	}

	return e.fakeStore.CreateInvite(ctx, i)
}

func (e *errStore) GetInviteByHash(ctx context.Context, hash string) (*Invite, error) {
	if err := e.fail["GetInviteByHash"]; err != nil {
		return nil, err
	}

	return e.fakeStore.GetInviteByHash(ctx, hash)
}

func (e *errStore) MarkInviteUsed(ctx context.Context, id, by ID) error {
	if err := e.fail["MarkInviteUsed"]; err != nil {
		return err
	}

	return e.fakeStore.MarkInviteUsed(ctx, id, by)
}

var _ Store = (*errStore)(nil)

func TestServiceResolveAccessPropagatesStoreError(t *testing.T) {
	store := newErrStore()
	injected := errors.New("db down")
	store.fail["GetSessionByAccessHash"] = injected

	svc := newTestService(store)

	_, _, err := svc.ResolveAccess(context.Background(), "some-token")
	if err == nil {
		t.Fatal("err = nil, ожидалась распространённая ошибка store")
	}
	if !errors.Is(err, injected) {
		t.Fatalf("err = %v, ожидался %v", err, injected)
	}
}

func TestServiceLoginPropagatesStoreError(t *testing.T) {
	store := newErrStore()
	injected := errors.New("db down")
	store.fail["GetOwnerByLogin"] = injected

	svc := newTestService(store)

	_, err := svc.Login(context.Background(), "user", "password123")
	if !errors.Is(err, injected) {
		t.Fatalf("err = %v, ожидался %v", err, injected)
	}
	if errors.Is(err, ErrInvalidCredentials) {
		t.Fatal("сбой store не должен маскироваться под ErrInvalidCredentials")
	}
}

func TestServiceRegisterPropagatesCountOwnersError(t *testing.T) {
	store := newErrStore()
	injected := errors.New("db down")
	store.fail["CountOwners"] = injected

	svc := newTestService(store)

	_, err := svc.Register(context.Background(), "user", "password123", nil)
	if !errors.Is(err, injected) {
		t.Fatalf("err = %v, ожидался %v", err, injected)
	}
}

func TestServiceRevokeAPITokenPropagatesGetError(t *testing.T) {
	store := newErrStore()
	svc := newTestService(store)
	ctx := context.Background()

	owner, err := svc.Register(ctx, "user", "password123", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	_, id, err := svc.CreateAPIToken(ctx, owner.OwnerID, "MCP")
	if err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}

	injected := errors.New("db down")
	store.fail["GetAPIToken"] = injected

	if err := svc.RevokeAPIToken(ctx, owner.OwnerID, id); !errors.Is(err, injected) {
		t.Fatalf("err = %v, ожидался %v", err, injected)
	} else if errors.Is(err, ErrNotFound) {
		t.Fatal("сбой store не должен маскироваться под ErrNotFound")
	}
}

func TestServiceResolveAPITokenIgnoresTouchError(t *testing.T) {
	store := newErrStore()
	svc := newTestService(store)
	ctx := context.Background()

	owner, err := svc.Register(ctx, "user", "password123", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	raw, _, err := svc.CreateAPIToken(ctx, owner.OwnerID, "MCP")
	if err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}

	store.fail["TouchAPIToken"] = errors.New("touch failed")

	resolved, err := svc.ResolveAPIToken(ctx, raw)
	if err != nil {
		t.Fatalf("ResolveAPIToken: err = %v, ожидался nil (сбой touch не должен отклонять валидный токен)", err)
	}
	if resolved != owner.OwnerID {
		t.Fatalf("OwnerID = %v, ожидался %v", resolved, owner.OwnerID)
	}
}
