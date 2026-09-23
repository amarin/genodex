package create_archive_document

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// dcID возвращает корректный идентификатор документа.
func dcID(last byte) models.ID {
	return models.ID("DC-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// anID возвращает корректный идентификатор узла архивного дерева.
func anID(last byte) models.ID {
	return models.ID("AN-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
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

// fakeTx реализует нужные сценарию методы store.Store поверх карт узлов и
// цитат; остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	nodes     map[models.ID]*models.ArchiveNode
	citations map[models.ID]*models.Citation
	saved     []*models.ArchiveDocument
	saveErr   error
}

func newFakeTx(existingNodes ...*models.ArchiveNode) *fakeTx {
	tx := &fakeTx{nodes: map[models.ID]*models.ArchiveNode{}, citations: map[models.ID]*models.Citation{}}
	for _, n := range existingNodes {
		tx.nodes[n.ID] = n
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

func (f *fakeTx) GetArchiveNode(_ context.Context, id models.ID) (*models.ArchiveNode, error) {
	n, ok := f.nodes[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *n

	return &cp, nil
}

func (f *fakeTx) SaveArchiveDocument(_ context.Context, d *models.ArchiveDocument) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *d
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует ArchiveDocumentStore: InTx выполняет fn на встроенном fakeTx.
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

func validInput(unit models.ID) models.ArchiveDocument {
	return models.ArchiveDocument{UnitID: unit, Title: "Метрическая книга 1890"}
}

func TestCreateArchiveDocumentGeneratesIDAndSaves(t *testing.T) {
	unit := anID('0')
	st := &fakeStore{tx: newFakeTx(&models.ArchiveNode{ID: unit, Type: "fond", ArchiveID: "AR-01ARZ3NDEKTSV4RRFFQ69G5FA0", Label: "Фонд"})}
	ids := &stubIDs{id: dcID('V')}

	got, err := New(st, ids).CreateArchiveDocument(context.Background(), validInput(unit))
	if err != nil {
		t.Fatalf("CreateArchiveDocument: %v", err)
	}

	if got.ID != dcID('V') || got.Title != "Метрическая книга 1890" {
		t.Fatalf("got %+v, ожидался документ с ID %v", got, dcID('V'))
	}

	if ids.gotType != models.TypeArchiveDocument {
		t.Errorf("генератор вызван с типом %q, ожидался archive_document", ids.gotType)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].ID != dcID('V') {
		t.Fatalf("InTx=%d, saved=%v; ожидалось одно сохранение внутри одной транзакции", st.calls, st.tx.saved)
	}
}

func TestCreateArchiveDocumentRejectsExplicitID(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	ids := &stubIDs{id: dcID('V')}

	in := validInput(anID('0'))
	in.ID = dcID('0')

	_, err := New(st, ids).CreateArchiveDocument(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю id", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx=%d; ожидалось: не вызван", st.calls)
	}
}

func TestCreateArchiveDocumentValidatesBeforeTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	in := validInput(anID('0'))
	in.Title = ""

	_, err := New(st, &stubIDs{id: dcID('V')}).CreateArchiveDocument(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "title" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю title", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestCreateArchiveDocumentUnitNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	_, err := New(st, &stubIDs{id: dcID('V')}).CreateArchiveDocument(context.Background(), validInput(anID('0')))

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "unit_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю unit_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d документов при несуществующей единице учёта", len(st.tx.saved))
	}
}

func TestCreateArchiveDocumentPropagatesSaveError(t *testing.T) {
	wantErr := errors.New("save failed")
	unit := anID('0')
	st := &fakeStore{tx: newFakeTx(&models.ArchiveNode{ID: unit, Type: "fond", ArchiveID: "AR-01ARZ3NDEKTSV4RRFFQ69G5FA0", Label: "Фонд"})}
	st.tx.saveErr = wantErr

	if _, err := New(st, &stubIDs{id: dcID('V')}).CreateArchiveDocument(context.Background(), validInput(unit)); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

func TestCreateArchiveDocumentPropagatesTxError(t *testing.T) {
	wantErr := errors.New("tx failed")
	st := &fakeStore{tx: newFakeTx(), inTxErr: wantErr}

	if _, err := New(st, &stubIDs{id: dcID('V')}).CreateArchiveDocument(context.Background(), validInput(anID('0'))); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

// TestCreateArchiveDocumentSourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestCreateArchiveDocumentSourceCitationNotFound(t *testing.T) {
	unit := anID('0')
	st := &fakeStore{tx: newFakeTx(&models.ArchiveNode{ID: unit, Type: "fond", ArchiveID: "AR-01ARZ3NDEKTSV4RRFFQ69G5FA0", Label: "Фонд"})}

	in := validInput(unit)
	in.Sources = []models.SourceLink{{CitationID: cID('0')}}

	_, err := New(st, &stubIDs{id: dcID('V')}).CreateArchiveDocument(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d документов при несуществующей цитате", len(st.tx.saved))
	}
}
