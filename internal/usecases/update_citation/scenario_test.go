package update_citation

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

func citID(last byte) models.ID {
	return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func srcID(last byte) models.ID {
	return models.ID("S-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func nodeID(last byte) models.ID {
	return models.ID("AN-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func docID(last byte) models.ID {
	return models.ID("DC-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func attID(last byte) models.ID {
	return models.ID("O-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// fakeTx реализует нужные сценарию методы store.Store поверх карт цитат,
// источников, узлов, документов и вложений; остальные методы порта паникуют
// через nil-встраивание.
type fakeTx struct {
	store.Store
	citations   map[models.ID]*models.Citation
	sources     map[models.ID]*models.Source
	nodes       map[models.ID]*models.ArchiveNode
	documents   map[models.ID]*models.ArchiveDocument
	attachments map[models.ID]*models.Attachment
	saved       []*models.Citation
}

func newFakeTx(existingCitations ...*models.Citation) *fakeTx {
	tx := &fakeTx{
		citations:   map[models.ID]*models.Citation{},
		sources:     map[models.ID]*models.Source{},
		nodes:       map[models.ID]*models.ArchiveNode{},
		documents:   map[models.ID]*models.ArchiveDocument{},
		attachments: map[models.ID]*models.Attachment{},
	}
	for _, c := range existingCitations {
		tx.citations[c.ID] = c
	}

	return tx
}

func (f *fakeTx) withSource(id models.ID) *fakeTx {
	f.sources[id] = &models.Source{ID: id}

	return f
}

func (f *fakeTx) withNode(id models.ID) *fakeTx {
	f.nodes[id] = &models.ArchiveNode{ID: id}

	return f
}

func (f *fakeTx) withDocument(id models.ID) *fakeTx {
	f.documents[id] = &models.ArchiveDocument{ID: id}

	return f
}

func (f *fakeTx) withAttachment(id models.ID) *fakeTx {
	f.attachments[id] = &models.Attachment{ID: id}

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

func (f *fakeTx) GetSource(_ context.Context, id models.ID) (*models.Source, error) {
	s, ok := f.sources[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
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

func (f *fakeTx) GetAttachment(_ context.Context, id models.ID) (*models.Attachment, error) {
	a, ok := f.attachments[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return a, nil
}

func (f *fakeTx) SaveCitation(_ context.Context, c *models.Citation) error {
	cp := *c
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует CitationStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func citation(id, source models.ID) *models.Citation {
	return &models.Citation{ID: id, SourceID: source, Text: "стр. 12, запись о рождении"}
}

func TestUpdateCitationSaves(t *testing.T) {
	src := srcID('0')
	existing := citation(citID('V'), src)
	st := &fakeStore{tx: newFakeTx(existing).withSource(src)}

	updated := *existing
	updated.Text = "стр. 12, запись о рождении (испр.)"

	if err := New(st).UpdateCitation(context.Background(), updated); err != nil {
		t.Fatalf("UpdateCitation: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Text != "стр. 12, запись о рождении (испр.)" {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateCitationNotFound(t *testing.T) {
	src := srcID('0')
	st := &fakeStore{tx: newFakeTx().withSource(src)}

	err := New(st).UpdateCitation(context.Background(), *citation(citID('V'), src))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// TestUpdateCitationValidatesBeforeTx: якорь структурно невалиден (номер
// страницы меньше 1) — ошибка возвращается ещё до похода в хранилище.
func TestUpdateCitationValidatesBeforeTx(t *testing.T) {
	src := srcID('0')
	existing := citation(citID('V'), src)
	st := &fakeStore{tx: newFakeTx(existing).withSource(src).withNode(nodeID('0'))}

	updated := *existing
	updated.Anchor = &models.ArchiveAnchor{NodeID: nodeID('0'), Page: 0}

	err := New(st).UpdateCitation(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "anchor.page" {
		t.Fatalf("err = %v, want ValidationError on anchor.page", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestUpdateCitationSourceNotFound(t *testing.T) {
	src := srcID('0')
	existing := citation(citID('V'), src)
	st := &fakeStore{tx: newFakeTx(existing)}

	err := New(st).UpdateCitation(context.Background(), *existing)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "source_id" {
		t.Fatalf("err = %v, want ValidationError on source_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d цитат при несуществующем источнике", len(st.tx.saved))
	}
}

func TestUpdateCitationWithURLAnchorSaves(t *testing.T) {
	src := srcID('0')
	existing := citation(citID('V'), src)
	st := &fakeStore{tx: newFakeTx(existing).withSource(src)}

	updated := *existing
	updated.Anchor = &models.URLAnchor{URL: "https://example.org/scan/12"}

	if err := New(st).UpdateCitation(context.Background(), updated); err != nil {
		t.Fatalf("UpdateCitation: %v", err)
	}

	if len(st.tx.saved) != 1 || st.tx.saved[0].Anchor.Kind() != models.AnchorURL {
		t.Fatalf("saved = %v", st.tx.saved)
	}
}

func TestUpdateCitationWithArchiveAnchorSaves(t *testing.T) {
	src, node := srcID('0'), nodeID('0')
	existing := citation(citID('V'), src)
	st := &fakeStore{tx: newFakeTx(existing).withSource(src).withNode(node)}

	updated := *existing
	updated.Anchor = &models.ArchiveAnchor{NodeID: node, Page: 1}

	if err := New(st).UpdateCitation(context.Background(), updated); err != nil {
		t.Fatalf("UpdateCitation: %v", err)
	}

	if len(st.tx.saved) != 1 {
		t.Fatalf("saved = %v", st.tx.saved)
	}
}

func TestUpdateCitationArchiveAnchorNodeNotFound(t *testing.T) {
	src := srcID('0')
	existing := citation(citID('V'), src)
	st := &fakeStore{tx: newFakeTx(existing).withSource(src)}

	updated := *existing
	updated.Anchor = &models.ArchiveAnchor{NodeID: nodeID('0'), Page: 1}

	err := New(st).UpdateCitation(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "anchor.node_id" {
		t.Fatalf("err = %v, want ValidationError on anchor.node_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d цитат при несуществующем узле", len(st.tx.saved))
	}
}

func TestUpdateCitationArchiveAnchorWithDocumentSaves(t *testing.T) {
	src, node, doc := srcID('0'), nodeID('0'), docID('0')
	existing := citation(citID('V'), src)
	st := &fakeStore{tx: newFakeTx(existing).withSource(src).withNode(node).withDocument(doc)}

	updated := *existing
	updated.Anchor = &models.ArchiveAnchor{NodeID: node, DocumentID: doc, Page: 1}

	if err := New(st).UpdateCitation(context.Background(), updated); err != nil {
		t.Fatalf("UpdateCitation: %v", err)
	}

	if len(st.tx.saved) != 1 {
		t.Fatalf("saved = %v", st.tx.saved)
	}
}

func TestUpdateCitationArchiveAnchorDocumentNotFound(t *testing.T) {
	src, node := srcID('0'), nodeID('0')
	existing := citation(citID('V'), src)
	st := &fakeStore{tx: newFakeTx(existing).withSource(src).withNode(node)}

	updated := *existing
	updated.Anchor = &models.ArchiveAnchor{NodeID: node, DocumentID: docID('9'), Page: 1}

	err := New(st).UpdateCitation(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "anchor.document_id" {
		t.Fatalf("err = %v, want ValidationError on anchor.document_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d цитат при несуществующем документе", len(st.tx.saved))
	}
}

func TestUpdateCitationWithFileAnchorSaves(t *testing.T) {
	src, att := srcID('0'), attID('0')
	existing := citation(citID('V'), src)
	st := &fakeStore{tx: newFakeTx(existing).withSource(src).withAttachment(att)}

	updated := *existing
	updated.Anchor = &models.FileAnchor{AttachmentID: att}

	if err := New(st).UpdateCitation(context.Background(), updated); err != nil {
		t.Fatalf("UpdateCitation: %v", err)
	}

	if len(st.tx.saved) != 1 {
		t.Fatalf("saved = %v", st.tx.saved)
	}
}

func TestUpdateCitationFileAnchorAttachmentNotFound(t *testing.T) {
	src := srcID('0')
	existing := citation(citID('V'), src)
	st := &fakeStore{tx: newFakeTx(existing).withSource(src)}

	updated := *existing
	updated.Anchor = &models.FileAnchor{AttachmentID: attID('0')}

	err := New(st).UpdateCitation(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "anchor.attachment_id" {
		t.Fatalf("err = %v, want ValidationError on anchor.attachment_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d цитат при несуществующем вложении", len(st.tx.saved))
	}
}
