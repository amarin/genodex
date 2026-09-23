package create_archive_node

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

type stubIDs struct {
	id      models.ID
	gotType models.Type
}

func (s *stubIDs) New(t models.Type) models.ID {
	s.gotType = t

	return s.id
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

func (f *fakeTx) withCitation(c *models.Citation) *fakeTx {
	f.citations[c.ID] = c

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
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует ArchiveNodeStore: InTx выполняет fn на встроенном fakeTx.
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

func validInput(archive models.ID) models.ArchiveNode {
	return models.ArchiveNode{Type: "fond", ArchiveID: archive, Label: "Фонд 1"}
}

func TestCreateArchiveNodeGeneratesIDAndSaves(t *testing.T) {
	archive := arID('0')
	st := &fakeStore{tx: newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"})}
	ids := &stubIDs{id: anID('V')}

	got, err := New(st, ids).CreateArchiveNode(context.Background(), validInput(archive))
	if err != nil {
		t.Fatalf("CreateArchiveNode: %v", err)
	}

	if got.ID != anID('V') || got.Label != "Фонд 1" {
		t.Fatalf("got %+v, ожидался узел с ID %v", got, anID('V'))
	}

	if ids.gotType != models.TypeArchiveNode {
		t.Errorf("генератор вызван с типом %q, ожидался archive_node", ids.gotType)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].ID != anID('V') {
		t.Fatalf("InTx=%d, saved=%v; ожидалось одно сохранение внутри одной транзакции", st.calls, st.tx.saved)
	}
}

func TestCreateArchiveNodeRejectsExplicitID(t *testing.T) {
	archive := arID('0')
	st := &fakeStore{tx: newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"})}
	ids := &stubIDs{id: anID('V')}

	in := validInput(archive)
	in.ID = anID('0')

	_, err := New(st, ids).CreateArchiveNode(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю id", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx=%d; ожидалось: не вызван", st.calls)
	}
}

func TestCreateArchiveNodeValidatesBeforeTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	in := validInput(arID('0'))
	in.Label = ""

	_, err := New(st, &stubIDs{id: anID('V')}).CreateArchiveNode(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "label" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю label", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestCreateArchiveNodeArchiveNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	_, err := New(st, &stubIDs{id: anID('V')}).CreateArchiveNode(context.Background(), validInput(arID('0')))

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "archive_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю archive_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d узлов при несуществующем архиве", len(st.tx.saved))
	}
}

func TestCreateArchiveNodeWithParentSaves(t *testing.T) {
	archive := arID('0')
	parent := &models.ArchiveNode{ID: anID('P'), Type: "fond", ArchiveID: archive, Label: "Родитель"}
	tx := newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"}).withNode(parent)
	st := &fakeStore{tx: tx}

	in := validInput(archive)
	pid := parent.ID
	in.ParentID = &pid

	got, err := New(st, &stubIDs{id: anID('V')}).CreateArchiveNode(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateArchiveNode: %v", err)
	}

	if got.ParentID == nil || *got.ParentID != parent.ID || len(st.tx.saved) != 1 {
		t.Fatalf("got %+v, saved=%d; ожидалось сохранение со ссылкой на родителя", got, len(st.tx.saved))
	}
}

func TestCreateArchiveNodeParentNotFound(t *testing.T) {
	archive := arID('0')
	st := &fakeStore{tx: newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"})}

	in := validInput(archive)
	missing := anID('9')
	in.ParentID = &missing

	_, err := New(st, &stubIDs{id: anID('V')}).CreateArchiveNode(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d узлов при несуществующем родителе", len(st.tx.saved))
	}
}

// TestCreateArchiveNodeParentFromOtherArchiveRejected: родитель существует,
// но принадлежит другому архиву — *ValidationError по полю parent_id
// (не archive_id: сам архив существует, инвариант — «родитель из того же
// архива»).
func TestCreateArchiveNodeParentFromOtherArchiveRejected(t *testing.T) {
	archive := arID('0')
	otherArchive := arID('1')
	parent := &models.ArchiveNode{ID: anID('P'), Type: "fond", ArchiveID: otherArchive, Label: "Чужой родитель"}
	tx := newFakeTx().
		withArchive(&models.Archive{ID: archive, Name: "ГАВО"}).
		withArchive(&models.Archive{ID: otherArchive, Name: "РГАДА"}).
		withNode(parent)
	st := &fakeStore{tx: tx}

	in := validInput(archive)
	pid := parent.ID
	in.ParentID = &pid

	_, err := New(st, &stubIDs{id: anID('V')}).CreateArchiveNode(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id (родитель из другого архива)", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d узлов при родителе из другого архива", len(st.tx.saved))
	}
}

func TestCreateArchiveNodePropagatesSaveError(t *testing.T) {
	wantErr := errors.New("save failed")
	archive := arID('0')
	tx := newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"})
	tx.saveErr = wantErr
	st := &fakeStore{tx: tx}

	if _, err := New(st, &stubIDs{id: anID('V')}).CreateArchiveNode(context.Background(), validInput(archive)); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

func TestCreateArchiveNodePropagatesTxError(t *testing.T) {
	wantErr := errors.New("tx failed")
	st := &fakeStore{tx: newFakeTx(), inTxErr: wantErr}

	if _, err := New(st, &stubIDs{id: anID('V')}).CreateArchiveNode(context.Background(), validInput(arID('0'))); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

// TestCreateArchiveNodeSourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestCreateArchiveNodeSourceCitationNotFound(t *testing.T) {
	archive := arID('0')
	st := &fakeStore{tx: newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"})}

	in := validInput(archive)
	in.Sources = []models.SourceLink{{CitationID: cID('0')}}

	_, err := New(st, &stubIDs{id: anID('V')}).CreateArchiveNode(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d узлов при несуществующей цитате", len(st.tx.saved))
	}
}
