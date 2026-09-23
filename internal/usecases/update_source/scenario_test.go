package update_source

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

func srcID(last byte) models.ID {
	return models.ID("S-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func rID(last byte) models.ID {
	return models.ID("R-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// fakeTx реализует нужные сценарию методы store.Store поверх карт источников и
// хранилищ; остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	sources      map[models.ID]*models.Source
	repositories map[models.ID]*models.Repository
	saved        []*models.Source
	repoGetErr   error
}

func newFakeTx(existing ...*models.Source) *fakeTx {
	tx := &fakeTx{sources: map[models.ID]*models.Source{}, repositories: map[models.ID]*models.Repository{}}
	for _, s := range existing {
		tx.sources[s.ID] = s
	}

	return tx
}

func (f *fakeTx) withRepository(r *models.Repository) *fakeTx {
	f.repositories[r.ID] = r

	return f
}

func (f *fakeTx) GetSource(_ context.Context, id models.ID) (*models.Source, error) {
	s, ok := f.sources[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *s

	return &cp, nil
}

func (f *fakeTx) GetRepository(_ context.Context, id models.ID) (*models.Repository, error) {
	if f.repoGetErr != nil {
		return nil, f.repoGetErr
	}

	r, ok := f.repositories[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *r

	return &cp, nil
}

func (f *fakeTx) SaveSource(_ context.Context, s *models.Source) error {
	cp := *s
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует SourceStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func source(id models.ID, title string) *models.Source {
	return &models.Source{ID: id, Kind: models.SourceKindDocument, Title: title, Reliability: models.ReliabilityPrimary}
}

func TestUpdateSourceSaves(t *testing.T) {
	existing := source(srcID('V'), "Метрическая книга")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Title = "Метрическая книга (испр.)"

	if err := New(st).UpdateSource(context.Background(), updated); err != nil {
		t.Fatalf("UpdateSource: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Title != "Метрическая книга (испр.)" {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateSourceNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateSource(context.Background(), *source(srcID('V'), "Метрическая книга"))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateSourceValidatesBeforeTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateSource(context.Background(), models.Source{ID: srcID('V'), Kind: models.SourceKindDocument, Reliability: models.ReliabilityPrimary})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "title" {
		t.Fatalf("err = %v, want ValidationError on title", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestUpdateSourceWithRepositorySaves(t *testing.T) {
	existing := source(srcID('V'), "Метрическая книга")
	repo := &models.Repository{ID: rID('0'), Name: "ГАВО", Type: models.RepositoryTypeArchive}
	st := &fakeStore{tx: newFakeTx(existing).withRepository(repo)}

	updated := *existing
	updated.RepositoryID = repo.ID

	if err := New(st).UpdateSource(context.Background(), updated); err != nil {
		t.Fatalf("UpdateSource: %v", err)
	}

	if len(st.tx.saved) != 1 || st.tx.saved[0].RepositoryID != repo.ID {
		t.Fatalf("saved = %v", st.tx.saved)
	}
}

func TestUpdateSourceRepositoryNotFound(t *testing.T) {
	existing := source(srcID('V'), "Метрическая книга")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.RepositoryID = rID('0')

	err := New(st).UpdateSource(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "repository_id" {
		t.Fatalf("err = %v, want ValidationError on repository_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d источников при несуществующем хранилище", len(st.tx.saved))
	}
}

// TestUpdateSourcePropagatesRepositoryGetError проверяет, что настоящий сбой
// хранилища (не models.ErrNotFound) при проверке repository_id пробрасывается
// как есть, а не превращается в *models.ValidationError — см.
// create_source/scenario_test.go: TestCreateSourcePropagatesRepositoryGetError
// для того же контракта на создании.
func TestUpdateSourcePropagatesRepositoryGetError(t *testing.T) {
	wantErr := errors.New("get failed")
	existing := source(srcID('V'), "Метрическая книга")
	st := &fakeStore{tx: newFakeTx(existing)}
	st.tx.repoGetErr = wantErr

	updated := *existing
	updated.RepositoryID = rID('0')

	err := New(st).UpdateSource(context.Background(), updated)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}

	var ve *models.ValidationError
	if errors.As(err, &ve) {
		t.Fatalf("сбой хранилища превращён в *ValidationError: %v", err)
	}
}
