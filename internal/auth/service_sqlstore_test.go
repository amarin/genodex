package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/storage"
)

func newTestServiceSQL(t *testing.T) *Service {
	t.Helper()

	return New(newSQLStore(t))
}

// TestServiceOnSQLStoreFullLifecycle: bootstrap → invite → второй владелец →
// логин → refresh → API-токен → смена пароля гасит сессии → анонимный доступ.
func TestServiceOnSQLStoreFullLifecycle(t *testing.T) {
	svc := newTestServiceSQL(t)
	ctx := context.Background()

	boot, err := svc.Bootstrap(ctx)
	if err != nil || !boot {
		t.Fatalf("Bootstrap = %v, %v; ожидалось true, nil", boot, err)
	}

	first, err := svc.Register(ctx, "first", "password123", nil)
	if err != nil {
		t.Fatalf("Register (bootstrap): %v", err)
	}

	boot, err = svc.Bootstrap(ctx)
	if err != nil || boot {
		t.Fatalf("Bootstrap после первого владельца = %v, %v; ожидалось false, nil", boot, err)
	}

	raw, err := svc.CreateInvite(ctx, first.OwnerID)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	second, err := svc.Register(ctx, "second", "password123", &raw)
	if err != nil {
		t.Fatalf("Register (invite): %v", err)
	}

	loggedIn, err := svc.Login(ctx, "second", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	access, owner, err := svc.ResolveAccess(ctx, loggedIn.AccessToken)
	if err != nil || owner == nil || *owner != second.OwnerID {
		t.Fatalf("ResolveAccess = %v, %v, %v", access, owner, err)
	}

	rotated, err := svc.Refresh(ctx, loggedIn.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if _, err := svc.Refresh(ctx, loggedIn.RefreshToken); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("повтор старого refresh: err = %v, ожидался ErrSessionExpired", err)
	}

	rawToken, tokenID, err := svc.CreateAPIToken(ctx, second.OwnerID, "MCP")
	if err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}

	if resolved, err := svc.ResolveAPIToken(ctx, rawToken); err != nil || resolved != second.OwnerID {
		t.Fatalf("ResolveAPIToken: %v, %v", resolved, err)
	}

	extraLogin, err := svc.Login(ctx, "second", "password123")
	if err != nil {
		t.Fatalf("второй Login: %v", err)
	}

	if err := svc.ChangePassword(ctx, second.OwnerID, "password123", "new-password456"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	for name, tok := range map[string]string{"rotated": rotated.AccessToken, "extraLogin": extraLogin.AccessToken} {
		if access, owner, err := svc.ResolveAccess(ctx, tok); err != nil || access != models.AccessPublic || owner != nil {
			t.Errorf("%s после ChangePassword: access=%v owner=%v err=%v", name, access, owner, err)
		}
	}

	if err := svc.RevokeAPIToken(ctx, second.OwnerID, tokenID); err != nil {
		t.Fatalf("RevokeAPIToken: %v", err)
	}

	if _, err := svc.ResolveAPIToken(ctx, rawToken); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("отозванный токен: err = %v, ожидался ErrInvalidCredentials", err)
	}
}

// TestServiceOnSQLStorePersistsAcrossInstances: новый *Service на том же
// файле БД видит данные, записанные предыдущим — подтверждает, что данные
// реально на диске, а не только в памяти процесса.
func TestServiceOnSQLStorePersistsAcrossInstances(t *testing.T) {
	dir := t.TempDir()

	open := func(t *testing.T) *Service {
		t.Helper()

		st, err := storage.Open(dir)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		t.Cleanup(func() { _ = st.Close() })

		return New(NewSQLStore(st.DB()))
	}

	svc1 := open(t)

	res, err := svc1.Register(context.Background(), "user", "password123", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	// svc1 stays open (cleanup deferred via t.Cleanup) while a second
	// *storage.Storage is opened on the same on-disk dir below — safe because
	// internal/storage's DSN already sets WAL journal mode, a 5s busy_timeout
	// pragma, and SetMaxOpenConns(1) per *sql.DB, not a real concurrency hazard.
	svc2 := open(t)

	access, owner, err := svc2.ResolveAccess(context.Background(), res.AccessToken)
	if err != nil || access != models.AccessFull || owner == nil || *owner != res.OwnerID {
		t.Fatalf("после переоткрытия: access=%v owner=%v err=%v", access, owner, err)
	}
}
