package create_attachment

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// oID возвращает корректный идентификатор вложения, отличающийся последним символом.
func oID(last byte) models.ID {
	return models.ID("O-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func nodeID(last byte) models.ID {
	return models.ID("AN-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func docID(last byte) models.ID {
	return models.ID("DC-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
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
// документов; остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	nodes     map[models.ID]*models.ArchiveNode
	documents map[models.ID]*models.ArchiveDocument
	saved     []*models.Attachment
	getErr    error
	saveErr   error
}

func newFakeTx(nodeIDs, docIDs []models.ID) *fakeTx {
	tx := &fakeTx{nodes: map[models.ID]*models.ArchiveNode{}, documents: map[models.ID]*models.ArchiveDocument{}}
	for _, id := range nodeIDs {
		tx.nodes[id] = &models.ArchiveNode{ID: id}
	}
	for _, id := range docIDs {
		tx.documents[id] = &models.ArchiveDocument{ID: id}
	}

	return tx
}

func (f *fakeTx) GetArchiveNode(_ context.Context, id models.ID) (*models.ArchiveNode, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

	n, ok := f.nodes[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return n, nil
}

func (f *fakeTx) GetArchiveDocument(_ context.Context, id models.ID) (*models.ArchiveDocument, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

	d, ok := f.documents[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return d, nil
}

func (f *fakeTx) SaveAttachment(_ context.Context, a *models.Attachment) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *a
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует AttachmentStore: InTx выполняет fn на встроенном fakeTx.
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

func validInput(node models.ID) models.Attachment {
	return models.Attachment{Kind: models.AttachmentKindScan, Filename: "0012.jpg", NodeID: node}
}

func TestCreateAttachmentGeneratesIDAndSaves(t *testing.T) {
	node := nodeID('0')
	st := &fakeStore{tx: newFakeTx([]models.ID{node}, nil)}
	ids := &stubIDs{id: oID('V')}

	got, err := New(st, ids).CreateAttachment(context.Background(), validInput(node))
	if err != nil {
		t.Fatalf("CreateAttachment: %v", err)
	}

	if got.ID != oID('V') || got.Filename != "0012.jpg" {
		t.Fatalf("got %+v, ожидалось вложение с ID %v", got, oID('V'))
	}

	if ids.gotType != models.TypeAttachment {
		t.Errorf("генератор вызван с типом %q, ожидался attachment", ids.gotType)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].ID != oID('V') {
		t.Fatalf("InTx=%d, saved=%v; ожидалось одно сохранение внутри одной транзакции", st.calls, st.tx.saved)
	}
}

func TestCreateAttachmentRejectsExplicitID(t *testing.T) {
	node := nodeID('0')
	st := &fakeStore{tx: newFakeTx([]models.ID{node}, nil)}
	ids := &stubIDs{id: oID('V')}

	in := validInput(node)
	in.ID = oID('0')

	_, err := New(st, ids).CreateAttachment(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю id", err)
	}

	if st.calls != 0 || ids.gotType != "" {
		t.Fatalf("InTx=%d, генератор вызван с %q; ожидалось: не вызваны", st.calls, ids.gotType)
	}
}

func TestCreateAttachmentValidatesBeforeTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx(nil, nil)}

	in := models.Attachment{Kind: models.AttachmentKindScan, NodeID: nodeID('0')} // ни uri, ни filename

	_, err := New(st, &stubIDs{id: oID('V')}).CreateAttachment(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "uri" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю uri", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestCreateAttachmentNodeNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx(nil, nil)}

	_, err := New(st, &stubIDs{id: oID('V')}).CreateAttachment(context.Background(), validInput(nodeID('0')))

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "node_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю node_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d вложений при несуществующем узле", len(st.tx.saved))
	}
}

func TestCreateAttachmentWithDocumentSaves(t *testing.T) {
	node, doc := nodeID('0'), docID('0')
	st := &fakeStore{tx: newFakeTx([]models.ID{node}, []models.ID{doc})}

	in := validInput(node)
	in.DocumentID = &doc

	got, err := New(st, &stubIDs{id: oID('V')}).CreateAttachment(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateAttachment: %v", err)
	}

	if got.DocumentID == nil || *got.DocumentID != doc || len(st.tx.saved) != 1 {
		t.Fatalf("got %+v, saved=%d; ожидалось сохранение с документом", got, len(st.tx.saved))
	}
}

func TestCreateAttachmentDocumentNotFound(t *testing.T) {
	node := nodeID('0')
	st := &fakeStore{tx: newFakeTx([]models.ID{node}, nil)}

	in := validInput(node)
	missingDoc := docID('9')
	in.DocumentID = &missingDoc

	_, err := New(st, &stubIDs{id: oID('V')}).CreateAttachment(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "document_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю document_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d вложений при несуществующем документе", len(st.tx.saved))
	}
}

func TestCreateAttachmentPropagatesSaveError(t *testing.T) {
	node := nodeID('0')
	wantErr := errors.New("save failed")
	st := &fakeStore{tx: newFakeTx([]models.ID{node}, nil)}
	st.tx.saveErr = wantErr

	if _, err := New(st, &stubIDs{id: oID('V')}).CreateAttachment(context.Background(), validInput(node)); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

func TestCreateAttachmentPropagatesTxError(t *testing.T) {
	node := nodeID('0')
	wantErr := errors.New("tx failed")
	st := &fakeStore{tx: newFakeTx([]models.ID{node}, nil), inTxErr: wantErr}

	if _, err := New(st, &stubIDs{id: oID('V')}).CreateAttachment(context.Background(), validInput(node)); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

func TestCreateAttachmentPropagatesNodeGetError(t *testing.T) {
	wantErr := errors.New("get failed")
	st := &fakeStore{tx: newFakeTx(nil, nil)}
	st.tx.getErr = wantErr

	_, err := New(st, &stubIDs{id: oID('V')}).CreateAttachment(context.Background(), validInput(nodeID('0')))
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}

	var ve *models.ValidationError
	if errors.As(err, &ve) {
		t.Fatalf("сбой хранилища превращён в *ValidationError: %v", err)
	}
}
