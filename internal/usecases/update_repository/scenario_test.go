package update_repository

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// fakeTx реализует нужные сценарию методы store.Store поверх карты записей;
// остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	repositories map[models.ID]*models.Repository
	citations    map[models.ID]*models.Citation
	saved        []*models.Repository
}

func newFakeTx(existing ...*models.Repository) *fakeTx {
	tx := &fakeTx{repositories: map[models.ID]*models.Repository{}, citations: map[models.ID]*models.Citation{}}
	for _, s := range existing {
		tx.repositories[s.ID] = s
	}

	return tx
}

// cID возвращает корректный идентификатор цитаты.
func cID(last byte) models.ID {
	return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func (f *fakeTx) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *c

	return &cp, nil
}

func (f *fakeTx) GetRepository(_ context.Context, id models.ID) (*models.Repository, error) {
	s, ok := f.repositories[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *s

	return &cp, nil
}

func (f *fakeTx) SaveRepository(_ context.Context, s *models.Repository) error {
	cp := *s
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует RepositoryStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func repository(id models.ID, name string) *models.Repository {
	return &models.Repository{ID: id, Name: name, Type: models.RepositoryTypeArchive}
}

func TestUpdateRepositorySaves(t *testing.T) {
	existing := repository("R-01ARZ3NDEKTSV4RRFFQ69G5FA1", "ГАВО")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Name = "ГАВО (испр.)"

	if err := New(st).UpdateRepository(context.Background(), updated); err != nil {
		t.Fatalf("UpdateRepository: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Name != "ГАВО (испр.)" {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateRepositoryNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateRepository(context.Background(), *repository("R-01ARZ3NDEKTSV4RRFFQ69G5FA1", "ГАВО"))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateRepositoryRejectsEmptyName(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateRepository(context.Background(), models.Repository{ID: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, want ValidationError on name", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

// TestUpdateRepositorySourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestUpdateRepositorySourceCitationNotFound(t *testing.T) {
	existing := repository("R-01ARZ3NDEKTSV4RRFFQ69G5FA1", "ГАВО")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Sources = []models.SourceLink{{CitationID: cID('0')}}

	err := New(st).UpdateRepository(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d записей при несуществующей цитате", len(st.tx.saved))
	}
}

// TestUpdateRepositoryRejectsInvalidType — единственное отличие Repository от
// Surname: тип хранилища обязателен (открытый enum).
func TestUpdateRepositoryRejectsInvalidType(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateRepository(context.Background(), models.Repository{ID: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "ГАВО"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "type" {
		t.Fatalf("err = %v, want ValidationError on type", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}
