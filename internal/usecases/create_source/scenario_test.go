package create_source

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// srcID возвращает корректный идентификатор источника, отличающийся последним символом.
func srcID(last byte) models.ID {
	return models.ID("S-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// rID возвращает корректный идентификатор хранилища.
func rID(last byte) models.ID {
	return models.ID("R-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

type stubIDs struct {
	id      models.ID
	gotType models.Type
}

func (s *stubIDs) New(t models.Type) models.ID {
	s.gotType = t

	return s.id
}

// fakeTx реализует нужные сценарию методы store.Store поверх карт источников
// и хранилищ; остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	repositories map[models.ID]*models.Repository
	saved        []*models.Source
	getErr       error
	saveErr      error
}

func newFakeTx(existingRepos ...*models.Repository) *fakeTx {
	tx := &fakeTx{repositories: map[models.ID]*models.Repository{}}
	for _, r := range existingRepos {
		tx.repositories[r.ID] = r
	}

	return tx
}

func (f *fakeTx) GetRepository(_ context.Context, id models.ID) (*models.Repository, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

	r, ok := f.repositories[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *r

	return &cp, nil
}

func (f *fakeTx) SaveSource(_ context.Context, s *models.Source) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *s
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует SourceStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx      *fakeTx
	inTxErr error
	calls   int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	if f.inTxErr != nil {
		return f.inTxErr
	}

	return fn(f.tx)
}

func validInput() models.Source {
	return models.Source{Kind: models.SourceKindDocument, Title: "Метрическая книга", Reliability: models.ReliabilityPrimary}
}

func TestCreateSourceGeneratesIDAndSaves(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	ids := &stubIDs{id: srcID('V')}

	got, err := New(st, ids).CreateSource(context.Background(), validInput())
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}

	if got.ID != srcID('V') || got.Title != "Метрическая книга" {
		t.Fatalf("got %+v, ожидался источник с ID %v", got, srcID('V'))
	}

	if ids.gotType != models.TypeSource {
		t.Errorf("генератор вызван с типом %q, ожидался source", ids.gotType)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].ID != srcID('V') {
		t.Fatalf("InTx=%d, saved=%v; ожидалось одно сохранение внутри одной транзакции", st.calls, st.tx.saved)
	}
}

func TestCreateSourceRejectsExplicitID(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	ids := &stubIDs{id: srcID('V')}

	in := validInput()
	in.ID = srcID('0')

	_, err := New(st, ids).CreateSource(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю id", err)
	}

	if st.calls != 0 || ids.gotType != "" {
		t.Fatalf("InTx=%d, генератор вызван с %q; ожидалось: не вызваны", st.calls, ids.gotType)
	}
}

func TestCreateSourceValidatesBeforeTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	in := validInput()
	in.Title = ""

	_, err := New(st, &stubIDs{id: srcID('V')}).CreateSource(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "title" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю title", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestCreateSourceWithRepositorySaves(t *testing.T) {
	repo := &models.Repository{ID: rID('0'), Name: "ГАВО", Type: models.RepositoryTypeArchive}
	st := &fakeStore{tx: newFakeTx(repo)}

	in := validInput()
	in.RepositoryID = repo.ID

	got, err := New(st, &stubIDs{id: srcID('V')}).CreateSource(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}

	if got.RepositoryID != repo.ID || len(st.tx.saved) != 1 {
		t.Fatalf("got %+v, saved=%d; ожидалось сохранение со ссылкой на хранилище", got, len(st.tx.saved))
	}
}

// TestCreateSourceRepositoryNotFound: RepositoryID задан, но такого
// хранилища нет — *models.ValidationError по полю repository_id, ничего не
// сохраняется. Случай «RepositoryID не задан вовсе — хранилище не
// проверяется» уже покрыт TestCreateSourceGeneratesIDAndSaves выше (пустой
// fakeTx, GetRepository не вызывается).
func TestCreateSourceRepositoryNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	in := validInput()
	in.RepositoryID = rID('0')

	_, err := New(st, &stubIDs{id: srcID('V')}).CreateSource(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "repository_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю repository_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d источников при несуществующем хранилище", len(st.tx.saved))
	}
}

func TestCreateSourcePropagatesSaveError(t *testing.T) {
	wantErr := errors.New("save failed")
	st := &fakeStore{tx: newFakeTx()}
	st.tx.saveErr = wantErr

	if _, err := New(st, &stubIDs{id: srcID('V')}).CreateSource(context.Background(), validInput()); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

func TestCreateSourcePropagatesTxError(t *testing.T) {
	wantErr := errors.New("tx failed")
	st := &fakeStore{tx: newFakeTx(), inTxErr: wantErr}

	if _, err := New(st, &stubIDs{id: srcID('V')}).CreateSource(context.Background(), validInput()); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

func TestCreateSourcePropagatesRepositoryGetError(t *testing.T) {
	wantErr := errors.New("get failed")
	st := &fakeStore{tx: newFakeTx()}
	st.tx.getErr = wantErr

	in := validInput()
	in.RepositoryID = rID('0')

	_, err := New(st, &stubIDs{id: srcID('V')}).CreateSource(context.Background(), in)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}

	var ve *models.ValidationError
	if errors.As(err, &ve) {
		t.Fatalf("сбой хранилища превращён в *ValidationError: %v", err)
	}
}
