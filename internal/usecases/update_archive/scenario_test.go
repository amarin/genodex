package update_archive

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

func arID(last byte) models.ID {
	return models.ID("AR-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func rID(last byte) models.ID {
	return models.ID("R-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// fakeTx реализует нужные сценарию методы store.Store поверх карт архивов и
// хранилищ; остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	archives     map[models.ID]*models.Archive
	repositories map[models.ID]*models.Repository
	saved        []*models.Archive
}

func newFakeTx(existing ...*models.Archive) *fakeTx {
	tx := &fakeTx{archives: map[models.ID]*models.Archive{}, repositories: map[models.ID]*models.Repository{}}
	for _, a := range existing {
		tx.archives[a.ID] = a
	}

	return tx
}

func (f *fakeTx) withRepository(r *models.Repository) *fakeTx {
	f.repositories[r.ID] = r

	return f
}

func (f *fakeTx) GetArchive(_ context.Context, id models.ID) (*models.Archive, error) {
	a, ok := f.archives[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *a

	return &cp, nil
}

func (f *fakeTx) GetRepository(_ context.Context, id models.ID) (*models.Repository, error) {
	r, ok := f.repositories[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *r

	return &cp, nil
}

func (f *fakeTx) SaveArchive(_ context.Context, a *models.Archive) error {
	cp := *a
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует ArchiveStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func archive(id models.ID, name string) *models.Archive {
	return &models.Archive{ID: id, Name: name}
}

func TestUpdateArchiveSaves(t *testing.T) {
	existing := archive(arID('V'), "ГАВО, архив")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Name = "ГАВО, архив (испр.)"

	if err := New(st).UpdateArchive(context.Background(), updated); err != nil {
		t.Fatalf("UpdateArchive: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Name != "ГАВО, архив (испр.)" {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateArchiveNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateArchive(context.Background(), *archive(arID('V'), "ГАВО, архив"))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateArchiveRejectsEmptyName(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateArchive(context.Background(), models.Archive{ID: arID('V')})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, want ValidationError on name", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestUpdateArchiveWithRepositorySaves(t *testing.T) {
	existing := archive(arID('V'), "ГАВО, архив")
	repo := &models.Repository{ID: rID('0'), Name: "ГАВО", Type: models.RepositoryTypeArchive}
	st := &fakeStore{tx: newFakeTx(existing).withRepository(repo)}

	updated := *existing
	updated.RepositoryID = repo.ID

	if err := New(st).UpdateArchive(context.Background(), updated); err != nil {
		t.Fatalf("UpdateArchive: %v", err)
	}

	if len(st.tx.saved) != 1 || st.tx.saved[0].RepositoryID != repo.ID {
		t.Fatalf("saved = %v", st.tx.saved)
	}
}

func TestUpdateArchiveRepositoryNotFound(t *testing.T) {
	existing := archive(arID('V'), "ГАВО, архив")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.RepositoryID = rID('0')

	err := New(st).UpdateArchive(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "repository_id" {
		t.Fatalf("err = %v, want ValidationError on repository_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d архивов при несуществующем хранилище", len(st.tx.saved))
	}
}
