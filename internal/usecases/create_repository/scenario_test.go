package create_repository

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// cID возвращает корректный идентификатор цитаты, отличающийся последним символом.
func cID(last byte) models.ID {
	return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

type stubIDs struct{ id models.ID }

func (s *stubIDs) New(models.Type) models.ID { return s.id }

// fakeTx реализует нужные сценарию методы store.Store поверх карты цитат;
// остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	citations map[models.ID]*models.Citation
	saved     *models.Repository
	saveErr   error
}

func newFakeTx(existingCitations ...*models.Citation) *fakeTx {
	tx := &fakeTx{citations: map[models.ID]*models.Citation{}}
	for _, c := range existingCitations {
		tx.citations[c.ID] = c
	}

	return tx
}

func (f *fakeTx) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *c

	return &cp, nil
}

func (f *fakeTx) SaveRepository(_ context.Context, s *models.Repository) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *s
	f.saved = &cp

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

func TestCreateRepositoryGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{tx: newFakeTx()}
	sc := New(st, ids)

	got, err := sc.CreateRepository(context.Background(), models.Repository{Name: "ГАВО", Type: models.RepositoryTypeArchive})
	if err != nil {
		t.Fatalf("CreateRepository: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.tx.saved == nil || st.tx.saved.Name != "ГАВО" {
		t.Fatalf("saved = %+v", st.tx.saved)
	}
}

func TestCreateRepositoryRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx()}, &stubIDs{})

	_, err := sc.CreateRepository(context.Background(), models.Repository{ID: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "x"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestCreateRepositoryRejectsEmptyName(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx()}, &stubIDs{id: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateRepository(context.Background(), models.Repository{})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, want ValidationError on name", err)
	}
}

// TestCreateRepositoryRejectsInvalidType — единственное отличие Repository от
// Surname: тип хранилища обязателен (открытый enum — формат [a-z][a-z0-9_-]*,
// internal/models/dictionary_validate.go, а не фиксированный список).
func TestCreateRepositoryRejectsInvalidType(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx()}, &stubIDs{id: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateRepository(context.Background(), models.Repository{Name: "ГАВО", Type: "Not Valid"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "type" {
		t.Fatalf("err = %v, want ValidationError on type", err)
	}
}

func TestCreateRepositoryPropagatesStoreError(t *testing.T) {
	wantErr := errors.New("db down")
	st := &fakeStore{tx: newFakeTx()}
	st.tx.saveErr = wantErr
	sc := New(st, &stubIDs{id: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateRepository(context.Background(), models.Repository{Name: "ГАВО", Type: models.RepositoryTypeArchive})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

// TestCreateRepositorySourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestCreateRepositorySourceCitationNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	sc := New(st, &stubIDs{id: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	in := models.Repository{Name: "ГАВО", Type: models.RepositoryTypeArchive, Sources: []models.SourceLink{{CitationID: cID('0')}}}

	_, err := sc.CreateRepository(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if st.tx.saved != nil {
		t.Fatalf("saved = %+v; ожидалось: ничего не сохранено", st.tx.saved)
	}
}
