package update_archive_node

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// anID возвращает корректный идентификатор узла, отличающийся последним символом.
func anID(last byte) models.ID {
	return models.ID("AN-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// arID возвращает корректный идентификатор архива.
func arID(last byte) models.ID {
	return models.ID("AR-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// cID возвращает корректный идентификатор цитаты.
func cID(last byte) models.ID {
	return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// fakeTx реализует нужные сценарию методы store.Store поверх карт архивов,
// узлов и цитат; остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	archives  map[models.ID]*models.Archive
	nodes     map[models.ID]*models.ArchiveNode
	citations map[models.ID]*models.Citation
	saved     []*models.ArchiveNode
	saveErr   error
}

func newFakeTx() *fakeTx {
	return &fakeTx{
		archives:  map[models.ID]*models.Archive{},
		nodes:     map[models.ID]*models.ArchiveNode{},
		citations: map[models.ID]*models.Citation{},
	}
}

func (f *fakeTx) withArchive(a *models.Archive) *fakeTx {
	f.archives[a.ID] = a

	return f
}

func (f *fakeTx) withNode(n *models.ArchiveNode) *fakeTx {
	f.nodes[n.ID] = n

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

func (f *fakeTx) GetArchiveNode(_ context.Context, id models.ID) (*models.ArchiveNode, error) {
	n, ok := f.nodes[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *n

	return &cp, nil
}

func (f *fakeTx) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *c

	return &cp, nil
}

func (f *fakeTx) SaveArchiveNode(_ context.Context, n *models.ArchiveNode) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *n
	f.nodes[n.ID] = &cp
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует ArchiveNodeStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func node(id models.ID, archive models.ID, parent *models.ID) *models.ArchiveNode {
	return &models.ArchiveNode{ID: id, Type: "fond", ArchiveID: archive, ParentID: parent, Label: "узел " + string(id)}
}

func TestUpdateArchiveNodeSaves(t *testing.T) {
	archive := arID('0')
	existing := node(anID('V'), archive, nil)
	st := &fakeStore{tx: newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"}).withNode(existing)}

	updated := *existing
	updated.Label = "новая метка"

	if err := New(st).UpdateArchiveNode(context.Background(), updated); err != nil {
		t.Fatalf("UpdateArchiveNode: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Label != "новая метка" {
		t.Fatalf("InTx=%d, saved=%v; ожидалось одно сохранение с новой меткой", st.calls, st.tx.saved)
	}
}

func TestUpdateArchiveNodeNotFound(t *testing.T) {
	archive := arID('0')
	st := &fakeStore{tx: newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"})}

	err := New(st).UpdateArchiveNode(context.Background(), *node(anID('V'), archive, nil))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d узлов при несуществующем узле", len(st.tx.saved))
	}
}

func TestUpdateArchiveNodeValidatesBeforeTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	bad := *node(anID('V'), arID('0'), nil)
	bad.Label = ""

	err := New(st).UpdateArchiveNode(context.Background(), bad)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "label" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю label", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestUpdateArchiveNodeArchiveNotFound(t *testing.T) {
	archive := arID('0')
	existing := node(anID('V'), archive, nil)
	st := &fakeStore{tx: newFakeTx().withNode(existing)} // архив не заведён

	err := New(st).UpdateArchiveNode(context.Background(), *existing)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "archive_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю archive_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d узлов при несуществующем архиве", len(st.tx.saved))
	}
}

func TestUpdateArchiveNodeParentNotFound(t *testing.T) {
	archive := arID('0')
	existing := node(anID('V'), archive, nil)
	st := &fakeStore{tx: newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"}).withNode(existing)}

	missing := anID('9')
	updated := *existing
	updated.ParentID = &missing

	err := New(st).UpdateArchiveNode(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d узлов при несуществующем родителе", len(st.tx.saved))
	}
}

// TestUpdateArchiveNodeParentFromOtherArchiveRejected: родитель существует,
// но принадлежит другому архиву — *ValidationError по полю parent_id.
func TestUpdateArchiveNodeParentFromOtherArchiveRejected(t *testing.T) {
	archive := arID('0')
	otherArchive := arID('1')
	existing := node(anID('V'), archive, nil)
	otherParent := node(anID('P'), otherArchive, nil)

	tx := newFakeTx().
		withArchive(&models.Archive{ID: archive, Name: "ГАВО"}).
		withArchive(&models.Archive{ID: otherArchive, Name: "РГАДА"}).
		withNode(existing).withNode(otherParent)
	st := &fakeStore{tx: tx}

	updated := *existing
	pid := otherParent.ID
	updated.ParentID = &pid

	err := New(st).UpdateArchiveNode(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id (родитель из другого архива)", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d узлов при родителе из другого архива", len(st.tx.saved))
	}
}

// TestUpdateArchiveNodeDirectCycle: B — дочка A; попытка сделать A дочкой B — цикл.
func TestUpdateArchiveNodeDirectCycle(t *testing.T) {
	archive := arID('0')
	idA, idB := anID('A'), anID('B')
	a := node(idA, archive, nil)
	b := node(idB, archive, &idA)

	st := &fakeStore{tx: newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"}).withNode(a).withNode(b)}

	updated := *a
	updated.ParentID = &idB

	err := New(st).UpdateArchiveNode(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id (цикл)", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d узлов при цикле", len(st.tx.saved))
	}
}

// TestUpdateArchiveNodeLongCycle: цепочка C → B → A; попытка сделать A дочкой C —
// цикл длиной три.
func TestUpdateArchiveNodeLongCycle(t *testing.T) {
	archive := arID('0')
	idA, idB, idC := anID('A'), anID('B'), anID('C')
	a := node(idA, archive, nil)
	b := node(idB, archive, &idA)
	c := node(idC, archive, &idB)

	st := &fakeStore{tx: newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"}).withNode(a).withNode(b).withNode(c)}

	updated := *a
	updated.ParentID = &idC

	err := New(st).UpdateArchiveNode(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id (цикл)", err)
	}
}

// TestUpdateArchiveNodeReparentOK: перенос под другого корректного родителя
// (того же архива) проходит.
func TestUpdateArchiveNodeReparentOK(t *testing.T) {
	archive := arID('0')
	idA, idB, idC := anID('A'), anID('B'), anID('C')
	a := node(idA, archive, nil)
	b := node(idB, archive, &idA)
	c := node(idC, archive, nil)

	st := &fakeStore{tx: newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"}).withNode(a).withNode(b).withNode(c)}

	updated := *b
	updated.ParentID = &idC

	if err := New(st).UpdateArchiveNode(context.Background(), updated); err != nil {
		t.Fatalf("UpdateArchiveNode: %v", err)
	}

	if len(st.tx.saved) != 1 || *st.tx.saved[0].ParentID != idC {
		t.Fatalf("saved=%v; ожидалось сохранение с родителем %v", st.tx.saved, idC)
	}
}

func TestUpdateArchiveNodePropagatesSaveError(t *testing.T) {
	wantErr := errors.New("save failed")
	archive := arID('0')
	existing := node(anID('V'), archive, nil)
	tx := newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"}).withNode(existing)
	tx.saveErr = wantErr
	st := &fakeStore{tx: tx}

	if err := New(st).UpdateArchiveNode(context.Background(), *existing); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

// TestUpdateArchiveNodeSourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestUpdateArchiveNodeSourceCitationNotFound(t *testing.T) {
	archive := arID('0')
	existing := node(anID('V'), archive, nil)
	st := &fakeStore{tx: newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"}).withNode(existing)}

	updated := *existing
	updated.Sources = []models.SourceLink{{CitationID: cID('0')}}

	err := New(st).UpdateArchiveNode(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d узлов при несуществующей цитате", len(st.tx.saved))
	}
}
