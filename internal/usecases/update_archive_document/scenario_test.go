package update_archive_document

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

func dcID(last byte) models.ID {
	return models.ID("DC-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func anID(last byte) models.ID {
	return models.ID("AN-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func cID(last byte) models.ID {
	return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// fakeTx реализует нужные сценарию методы store.Store поверх карт документов,
// узлов и цитат; остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	docs      map[models.ID]*models.ArchiveDocument
	nodes     map[models.ID]*models.ArchiveNode
	citations map[models.ID]*models.Citation
	saved     []*models.ArchiveDocument
	saveErr   error
}

func newFakeTx(existing ...*models.ArchiveDocument) *fakeTx {
	tx := &fakeTx{
		docs:      map[models.ID]*models.ArchiveDocument{},
		nodes:     map[models.ID]*models.ArchiveNode{},
		citations: map[models.ID]*models.Citation{},
	}
	for _, d := range existing {
		tx.docs[d.ID] = d
	}

	return tx
}

func (f *fakeTx) withNode(n *models.ArchiveNode) *fakeTx {
	f.nodes[n.ID] = n

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

func (f *fakeTx) GetArchiveDocument(_ context.Context, id models.ID) (*models.ArchiveDocument, error) {
	d, ok := f.docs[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *d

	return &cp, nil
}

func (f *fakeTx) SaveArchiveDocument(_ context.Context, d *models.ArchiveDocument) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *d
	f.docs[d.ID] = &cp
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует ArchiveDocumentStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func doc(id models.ID, unit models.ID) *models.ArchiveDocument {
	return &models.ArchiveDocument{ID: id, UnitID: unit, Title: "документ " + string(id)}
}

func TestUpdateArchiveDocumentSaves(t *testing.T) {
	unit := anID('0')
	existing := doc(dcID('V'), unit)
	st := &fakeStore{tx: newFakeTx(existing).withNode(&models.ArchiveNode{ID: unit, Type: "fond", ArchiveID: "AR-01ARZ3NDEKTSV4RRFFQ69G5FA0", Label: "Фонд"})}

	updated := *existing
	updated.Title = "новое название"

	if err := New(st).UpdateArchiveDocument(context.Background(), updated); err != nil {
		t.Fatalf("UpdateArchiveDocument: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Title != "новое название" {
		t.Fatalf("InTx=%d, saved=%v; ожидалось одно сохранение с новым названием", st.calls, st.tx.saved)
	}
}

func TestUpdateArchiveDocumentNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateArchiveDocument(context.Background(), *doc(dcID('V'), anID('0')))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d документов при несуществующем документе", len(st.tx.saved))
	}
}

func TestUpdateArchiveDocumentValidatesBeforeTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	bad := *doc(dcID('V'), anID('0'))
	bad.Title = ""

	err := New(st).UpdateArchiveDocument(context.Background(), bad)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "title" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю title", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestUpdateArchiveDocumentUnitNotFound(t *testing.T) {
	existing := doc(dcID('V'), anID('0'))
	st := &fakeStore{tx: newFakeTx(existing)} // единица учёта не заведена

	missing := anID('9')
	updated := *existing
	updated.UnitID = missing

	err := New(st).UpdateArchiveDocument(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "unit_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю unit_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d документов при несуществующей единице учёта", len(st.tx.saved))
	}
}

func TestUpdateArchiveDocumentPropagatesSaveError(t *testing.T) {
	wantErr := errors.New("save failed")
	unit := anID('0')
	existing := doc(dcID('V'), unit)
	tx := newFakeTx(existing).withNode(&models.ArchiveNode{ID: unit, Type: "fond", ArchiveID: "AR-01ARZ3NDEKTSV4RRFFQ69G5FA0", Label: "Фонд"})
	tx.saveErr = wantErr
	st := &fakeStore{tx: tx}

	if err := New(st).UpdateArchiveDocument(context.Background(), *existing); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

// TestUpdateArchiveDocumentSourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestUpdateArchiveDocumentSourceCitationNotFound(t *testing.T) {
	unit := anID('0')
	existing := doc(dcID('V'), unit)
	st := &fakeStore{tx: newFakeTx(existing).withNode(&models.ArchiveNode{ID: unit, Type: "fond", ArchiveID: "AR-01ARZ3NDEKTSV4RRFFQ69G5FA0", Label: "Фонд"})}

	updated := *existing
	updated.Sources = []models.SourceLink{{CitationID: cID('0')}}

	err := New(st).UpdateArchiveDocument(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d документов при несуществующей цитате", len(st.tx.saved))
	}
}
