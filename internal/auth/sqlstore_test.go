package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/amarin/genodex/internal/storage"
)

// newSQLStore открывает адаптер на временном каталоге.
func newSQLStore(t *testing.T) *SQLStore {
	t.Helper()

	st, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	return NewSQLStore(st.DB())
}

func testOwner(id ID) Owner {
	return Owner{ID: id, Login: "vladelec", PasswordHash: "hash", CreatedAt: time.Now().UTC().Truncate(time.Second)}
}

func TestSQLStoreCreateAndGetOwner(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()
	id, _ := newID(KindOwner)
	want := testOwner(id)

	if err := s.CreateOwner(ctx, want); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	byID, err := s.GetOwner(ctx, id)
	if err != nil {
		t.Fatalf("GetOwner: %v", err)
	}
	if *byID != want {
		t.Fatalf("GetOwner = %+v, want %+v", *byID, want)
	}

	byLogin, err := s.GetOwnerByLogin(ctx, "vladelec")
	if err != nil {
		t.Fatalf("GetOwnerByLogin: %v", err)
	}
	if *byLogin != want {
		t.Fatalf("GetOwnerByLogin = %+v, want %+v", *byLogin, want)
	}
}

func TestSQLStoreGetOwnerNotFound(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()

	if _, err := s.GetOwner(ctx, "OW-nonexistent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}

	if _, err := s.GetOwnerByLogin(ctx, "nikto"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}
}

func TestSQLStoreUpdateOwnerPassword(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()
	id, _ := newID(KindOwner)

	if err := s.CreateOwner(ctx, testOwner(id)); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	if err := s.UpdateOwnerPassword(ctx, id, "new-hash"); err != nil {
		t.Fatalf("UpdateOwnerPassword: %v", err)
	}

	got, err := s.GetOwner(ctx, id)
	if err != nil || got.PasswordHash != "new-hash" {
		t.Fatalf("got = %+v, err = %v", got, err)
	}
}

func TestSQLStoreUpdateOwnerPasswordNotFound(t *testing.T) {
	s := newSQLStore(t)

	if err := s.UpdateOwnerPassword(context.Background(), "OW-nonexistent", "h"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}
}

func TestSQLStoreCountOwners(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()

	n, err := s.CountOwners(ctx)
	if err != nil || n != 0 {
		t.Fatalf("n = %d, err = %v; ожидалось 0", n, err)
	}

	id, _ := newID(KindOwner)
	if err := s.CreateOwner(ctx, testOwner(id)); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	n, err = s.CountOwners(ctx)
	if err != nil || n != 1 {
		t.Fatalf("n = %d, err = %v; ожидалось 1", n, err)
	}
}

func TestSQLStoreCreateOwnerDuplicateLoginFails(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()
	id1, _ := newID(KindOwner)
	id2, _ := newID(KindOwner)

	if err := s.CreateOwner(ctx, testOwner(id1)); err != nil {
		t.Fatalf("первый CreateOwner: %v", err)
	}

	if err := s.CreateOwner(ctx, testOwner(id2)); err == nil {
		t.Fatal("ожидалась ошибка (login UNIQUE)")
	}
}
