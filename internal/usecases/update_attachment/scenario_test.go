package update_attachment

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

func oID(last byte) models.ID {
	return models.ID("O-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func nodeID(last byte) models.ID {
	return models.ID("AN-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func docID(last byte) models.ID {
	return models.ID("DC-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// fakeTx реализует нужные сценарию методы store.Store поверх карт вложений,
// узлов и документов; остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	attachments map[models.ID]*models.Attachment
	nodes       map[models.ID]*models.ArchiveNode
	documents   map[models.ID]*models.ArchiveDocument
	saved       []*models.Attachment
}

func newFakeTx(existing []*models.Attachment, nodeIDs, docIDs []models.ID) *fakeTx {
	tx := &fakeTx{
		attachments: map[models.ID]*models.Attachment{},
		nodes:       map[models.ID]*models.ArchiveNode{},
		documents:   map[models.ID]*models.ArchiveDocument{},
	}
	for _, a := range existing {
		tx.attachments[a.ID] = a
	}
	for _, id := range nodeIDs {
		tx.nodes[id] = &models.ArchiveNode{ID: id}
	}
	for _, id := range docIDs {
		tx.documents[id] = &models.ArchiveDocument{ID: id}
	}

	return tx
}

func (f *fakeTx) GetAttachment(_ context.Context, id models.ID) (*models.Attachment, error) {
	a, ok := f.attachments[id]
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

	return n, nil
}

func (f *fakeTx) GetArchiveDocument(_ context.Context, id models.ID) (*models.ArchiveDocument, error) {
	d, ok := f.documents[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return d, nil
}

func (f *fakeTx) SaveAttachment(_ context.Context, a *models.Attachment) error {
	cp := *a
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует AttachmentStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func attachment(id, node models.ID) *models.Attachment {
	return &models.Attachment{ID: id, Kind: models.AttachmentKindScan, Filename: "0012.jpg", NodeID: node}
}

func TestUpdateAttachmentSaves(t *testing.T) {
	node := nodeID('0')
	existing := attachment(oID('V'), node)
	st := &fakeStore{tx: newFakeTx([]*models.Attachment{existing}, []models.ID{node}, nil)}

	updated := *existing
	updated.Filename = "0013.jpg"

	if err := New(st).UpdateAttachment(context.Background(), updated); err != nil {
		t.Fatalf("UpdateAttachment: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Filename != "0013.jpg" {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateAttachmentNotFound(t *testing.T) {
	node := nodeID('0')
	st := &fakeStore{tx: newFakeTx(nil, []models.ID{node}, nil)}

	err := New(st).UpdateAttachment(context.Background(), *attachment(oID('V'), node))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateAttachmentRejectsEmptyURIAndFilename(t *testing.T) {
	st := &fakeStore{tx: newFakeTx(nil, nil, nil)}

	err := New(st).UpdateAttachment(context.Background(), models.Attachment{ID: oID('V'), Kind: models.AttachmentKindScan, NodeID: nodeID('0')})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "uri" {
		t.Fatalf("err = %v, want ValidationError on uri", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestUpdateAttachmentNodeNotFound(t *testing.T) {
	node := nodeID('0')
	existing := attachment(oID('V'), node)
	st := &fakeStore{tx: newFakeTx([]*models.Attachment{existing}, nil, nil)}

	err := New(st).UpdateAttachment(context.Background(), *existing)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "node_id" {
		t.Fatalf("err = %v, want ValidationError on node_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d вложений при несуществующем узле", len(st.tx.saved))
	}
}

func TestUpdateAttachmentWithDocumentSaves(t *testing.T) {
	node, doc := nodeID('0'), docID('0')
	existing := attachment(oID('V'), node)
	st := &fakeStore{tx: newFakeTx([]*models.Attachment{existing}, []models.ID{node}, []models.ID{doc})}

	updated := *existing
	updated.DocumentID = &doc

	if err := New(st).UpdateAttachment(context.Background(), updated); err != nil {
		t.Fatalf("UpdateAttachment: %v", err)
	}

	if len(st.tx.saved) != 1 || st.tx.saved[0].DocumentID == nil || *st.tx.saved[0].DocumentID != doc {
		t.Fatalf("saved = %v", st.tx.saved)
	}
}

func TestUpdateAttachmentDocumentNotFound(t *testing.T) {
	node := nodeID('0')
	existing := attachment(oID('V'), node)
	st := &fakeStore{tx: newFakeTx([]*models.Attachment{existing}, []models.ID{node}, nil)}

	missingDoc := docID('9')
	updated := *existing
	updated.DocumentID = &missingDoc

	err := New(st).UpdateAttachment(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "document_id" {
		t.Fatalf("err = %v, want ValidationError on document_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d вложений при несуществующем документе", len(st.tx.saved))
	}
}
