package create_archive

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// arID возвращает корректный идентификатор архива, отличающийся последним символом.
func arID(last byte) models.ID {
	return models.ID("AR-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// rID возвращает корректный идентификатор хранилища.
func rID(last byte) models.ID {
	return models.ID("R-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// cID возвращает корректный идентификатор цитаты.
func cID(last byte) models.ID {
	return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

type stubIDs struct {
	id      models.ID
	gotType models.Type
}

func (s *stubIDs) New(t models.Type) models.ID {
	s.gotType = t

	return s.id
}

// fakeTx реализует нужные сценарию методы store.Store поверх карт архивов и
// хранилищ; остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	repositories map[models.ID]*models.Repository
	citations    map[models.ID]*models.Citation
	saved        []*models.Archive
	getErr       error
	saveErr      error
}

func newFakeTx(existingRepos ...*models.Repository) *fakeTx {
	tx := &fakeTx{repositories: map[models.ID]*models.Repository{}, citations: map[models.ID]*models.Citation{}}
	for _, r := range existingRepos {
		tx.repositories[r.ID] = r
	}

	return tx
}

func (f *fakeTx) withCitation(c *models.Citation) *fakeTx {
	f.citations[c.ID] = c

	return f
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

func (f *fakeTx) SaveArchive(_ context.Context, a *models.Archive) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *a
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует ArchiveStore: InTx выполняет fn на встроенном fakeTx.
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

func validInput() models.Archive {
	return models.Archive{Name: "ГАВО, архив"}
}

func TestCreateArchiveGeneratesIDAndSaves(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	ids := &stubIDs{id: arID('V')}

	got, err := New(st, ids).CreateArchive(context.Background(), validInput())
	if err != nil {
		t.Fatalf("CreateArchive: %v", err)
	}

	if got.ID != arID('V') || got.Name != "ГАВО, архив" {
		t.Fatalf("got %+v, ожидался архив с ID %v", got, arID('V'))
	}

	if ids.gotType != models.TypeArchive {
		t.Errorf("генератор вызван с типом %q, ожидался archive", ids.gotType)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].ID != arID('V') {
		t.Fatalf("InTx=%d, saved=%v; ожидалось одно сохранение внутри одной транзакции", st.calls, st.tx.saved)
	}
}

func TestCreateArchiveRejectsExplicitID(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	ids := &stubIDs{id: arID('V')}

	in := validInput()
	in.ID = arID('0')

	_, err := New(st, ids).CreateArchive(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю id", err)
	}

	if st.calls != 0 || ids.gotType != "" {
		t.Fatalf("InTx=%d, генератор вызван с %q; ожидалось: не вызваны", st.calls, ids.gotType)
	}
}

func TestCreateArchiveValidatesBeforeTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	in := validInput()
	in.Name = ""

	_, err := New(st, &stubIDs{id: arID('V')}).CreateArchive(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю name", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestCreateArchiveWithRepositorySaves(t *testing.T) {
	repo := &models.Repository{ID: rID('0'), Name: "ГАВО", Type: models.RepositoryTypeArchive}
	st := &fakeStore{tx: newFakeTx(repo)}

	in := validInput()
	in.RepositoryID = repo.ID

	got, err := New(st, &stubIDs{id: arID('V')}).CreateArchive(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateArchive: %v", err)
	}

	if got.RepositoryID != repo.ID || len(st.tx.saved) != 1 {
		t.Fatalf("got %+v, saved=%d; ожидалось сохранение со ссылкой на хранилище", got, len(st.tx.saved))
	}
}

// TestCreateArchiveRepositoryNotFound: RepositoryID задан, но такого
// хранилища нет — *models.ValidationError по полю repository_id, ничего не
// сохраняется. Случай «RepositoryID не задан вовсе — хранилище не
// проверяется» уже покрыт TestCreateArchiveGeneratesIDAndSaves выше (пустой
// fakeTx, GetRepository не вызывается).
func TestCreateArchiveRepositoryNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	in := validInput()
	in.RepositoryID = rID('0')

	_, err := New(st, &stubIDs{id: arID('V')}).CreateArchive(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "repository_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю repository_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d архивов при несуществующем хранилище", len(st.tx.saved))
	}
}

func TestCreateArchivePropagatesSaveError(t *testing.T) {
	wantErr := errors.New("save failed")
	st := &fakeStore{tx: newFakeTx()}
	st.tx.saveErr = wantErr

	if _, err := New(st, &stubIDs{id: arID('V')}).CreateArchive(context.Background(), validInput()); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

func TestCreateArchivePropagatesTxError(t *testing.T) {
	wantErr := errors.New("tx failed")
	st := &fakeStore{tx: newFakeTx(), inTxErr: wantErr}

	if _, err := New(st, &stubIDs{id: arID('V')}).CreateArchive(context.Background(), validInput()); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

func TestCreateArchivePropagatesRepositoryGetError(t *testing.T) {
	wantErr := errors.New("get failed")
	st := &fakeStore{tx: newFakeTx()}
	st.tx.getErr = wantErr

	in := validInput()
	in.RepositoryID = rID('0')

	_, err := New(st, &stubIDs{id: arID('V')}).CreateArchive(context.Background(), in)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}

	var ve *models.ValidationError
	if errors.As(err, &ve) {
		t.Fatalf("сбой хранилища превращён в *ValidationError: %v", err)
	}
}

// TestCreateArchiveSourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestCreateArchiveSourceCitationNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	in := validInput()
	in.Sources = []models.SourceLink{{CitationID: cID('0')}}

	_, err := New(st, &stubIDs{id: arID('V')}).CreateArchive(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d архивов при несуществующей цитате", len(st.tx.saved))
	}
}
