package create_citation

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// citID возвращает корректный идентификатор цитаты, отличающийся последним символом.
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

type stubIDs struct {
	id      models.ID
	gotType models.Type
}

func (s *stubIDs) New(t models.Type) models.ID {
	s.gotType = t

	return s.id
}

// fakeTx реализует нужные сценарию методы store.Store поверх карт источников,
// узлов, документов и вложений; остальные методы порта паникуют через
// nil-встраивание.
type fakeTx struct {
	store.Store
	sources     map[models.ID]*models.Source
	nodes       map[models.ID]*models.ArchiveNode
	documents   map[models.ID]*models.ArchiveDocument
	attachments map[models.ID]*models.Attachment
	saved       []*models.Citation
	saveErr     error
}

func newFakeTx(existingSources ...*models.Source) *fakeTx {
	tx := &fakeTx{
		sources:     map[models.ID]*models.Source{},
		nodes:       map[models.ID]*models.ArchiveNode{},
		documents:   map[models.ID]*models.ArchiveDocument{},
		attachments: map[models.ID]*models.Attachment{},
	}
	for _, s := range existingSources {
		tx.sources[s.ID] = s
	}

	return tx
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
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *c
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует CitationStore: InTx выполняет fn на встроенном fakeTx.
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

func validInput(source models.ID) models.Citation {
	return models.Citation{SourceID: source, Text: "стр. 12, запись о рождении"}
}

func TestCreateCitationGeneratesIDAndSaves(t *testing.T) {
	src := srcID('0')
	st := &fakeStore{tx: newFakeTx(&models.Source{ID: src})}
	ids := &stubIDs{id: citID('V')}

	got, err := New(st, ids).CreateCitation(context.Background(), validInput(src))
	if err != nil {
		t.Fatalf("CreateCitation: %v", err)
	}

	if got.ID != citID('V') || got.SourceID != src {
		t.Fatalf("got %+v, ожидалась цитата с ID %v", got, citID('V'))
	}

	if ids.gotType != models.TypeCitation {
		t.Errorf("генератор вызван с типом %q, ожидался citation", ids.gotType)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].ID != citID('V') {
		t.Fatalf("InTx=%d, saved=%v; ожидалось одно сохранение внутри одной транзакции", st.calls, st.tx.saved)
	}
}

func TestCreateCitationRejectsExplicitID(t *testing.T) {
	src := srcID('0')
	st := &fakeStore{tx: newFakeTx(&models.Source{ID: src})}
	ids := &stubIDs{id: citID('V')}

	in := validInput(src)
	in.ID = citID('0')

	_, err := New(st, ids).CreateCitation(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю id", err)
	}

	if st.calls != 0 || ids.gotType != "" {
		t.Fatalf("InTx=%d, генератор вызван с %q; ожидалось: не вызваны", st.calls, ids.gotType)
	}
}

// TestCreateCitationValidatesBeforeTx: якорь структурно невалиден (номер
// страницы меньше 1) — ошибка возвращается ещё до похода в хранилище, хотя
// узел якоря существует и мог бы пройти проверку ссылки.
func TestCreateCitationValidatesBeforeTx(t *testing.T) {
	src := srcID('0')
	st := &fakeStore{tx: newFakeTx(&models.Source{ID: src}).withNode(nodeID('0'))}

	in := validInput(src)
	in.Anchor = &models.ArchiveAnchor{NodeID: nodeID('0'), Page: 0}

	_, err := New(st, &stubIDs{id: citID('V')}).CreateCitation(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "anchor.page" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю anchor.page", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestCreateCitationSourceNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	_, err := New(st, &stubIDs{id: citID('V')}).CreateCitation(context.Background(), validInput(srcID('0')))

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "source_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю source_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d цитат при несуществующем источнике", len(st.tx.saved))
	}
}

func TestCreateCitationSavesWithURLAnchor(t *testing.T) {
	src := srcID('0')
	st := &fakeStore{tx: newFakeTx(&models.Source{ID: src})}

	in := validInput(src)
	in.Anchor = &models.URLAnchor{URL: "https://example.org/scan/12"}

	got, err := New(st, &stubIDs{id: citID('V')}).CreateCitation(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateCitation: %v", err)
	}

	if len(st.tx.saved) != 1 || got.Anchor.Kind() != models.AnchorURL {
		t.Fatalf("got %+v, saved=%d; ожидалось сохранение с url-якорем", got, len(st.tx.saved))
	}
}

func TestCreateCitationSavesWithArchiveAnchorValidNode(t *testing.T) {
	src, node := srcID('0'), nodeID('0')
	st := &fakeStore{tx: newFakeTx(&models.Source{ID: src}).withNode(node)}

	in := validInput(src)
	in.Anchor = &models.ArchiveAnchor{NodeID: node, Page: 1}

	got, err := New(st, &stubIDs{id: citID('V')}).CreateCitation(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateCitation: %v", err)
	}

	if len(st.tx.saved) != 1 || got.ID != citID('V') {
		t.Fatalf("got %+v, saved=%d; ожидалось сохранение с архивным якорем", got, len(st.tx.saved))
	}
}

func TestCreateCitationArchiveAnchorNodeNotFound(t *testing.T) {
	src := srcID('0')
	st := &fakeStore{tx: newFakeTx(&models.Source{ID: src})}

	in := validInput(src)
	in.Anchor = &models.ArchiveAnchor{NodeID: nodeID('0'), Page: 1}

	_, err := New(st, &stubIDs{id: citID('V')}).CreateCitation(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "anchor.node_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю anchor.node_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d цитат при несуществующем узле", len(st.tx.saved))
	}
}

func TestCreateCitationArchiveAnchorWithDocumentValid(t *testing.T) {
	src, node, doc := srcID('0'), nodeID('0'), docID('0')
	st := &fakeStore{tx: newFakeTx(&models.Source{ID: src}).withNode(node).withDocument(doc)}

	in := validInput(src)
	in.Anchor = &models.ArchiveAnchor{NodeID: node, DocumentID: doc, Page: 1}

	got, err := New(st, &stubIDs{id: citID('V')}).CreateCitation(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateCitation: %v", err)
	}

	if len(st.tx.saved) != 1 || got.ID != citID('V') {
		t.Fatalf("got %+v, saved=%d; ожидалось сохранение с архивным якорем и документом", got, len(st.tx.saved))
	}
}

func TestCreateCitationArchiveAnchorDocumentNotFound(t *testing.T) {
	src, node := srcID('0'), nodeID('0')
	st := &fakeStore{tx: newFakeTx(&models.Source{ID: src}).withNode(node)}

	in := validInput(src)
	in.Anchor = &models.ArchiveAnchor{NodeID: node, DocumentID: docID('9'), Page: 1}

	_, err := New(st, &stubIDs{id: citID('V')}).CreateCitation(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "anchor.document_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю anchor.document_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d цитат при несуществующем документе", len(st.tx.saved))
	}
}

func TestCreateCitationSavesWithFileAnchorValidAttachment(t *testing.T) {
	src, att := srcID('0'), attID('0')
	st := &fakeStore{tx: newFakeTx(&models.Source{ID: src}).withAttachment(att)}

	in := validInput(src)
	in.Anchor = &models.FileAnchor{AttachmentID: att}

	got, err := New(st, &stubIDs{id: citID('V')}).CreateCitation(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateCitation: %v", err)
	}

	if len(st.tx.saved) != 1 || got.ID != citID('V') {
		t.Fatalf("got %+v, saved=%d; ожидалось сохранение с файловым якорем", got, len(st.tx.saved))
	}
}

func TestCreateCitationFileAnchorAttachmentNotFound(t *testing.T) {
	src := srcID('0')
	st := &fakeStore{tx: newFakeTx(&models.Source{ID: src})}

	in := validInput(src)
	in.Anchor = &models.FileAnchor{AttachmentID: attID('0')}

	_, err := New(st, &stubIDs{id: citID('V')}).CreateCitation(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "anchor.attachment_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю anchor.attachment_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d цитат при несуществующем вложении", len(st.tx.saved))
	}
}

func TestCreateCitationPropagatesSaveError(t *testing.T) {
	src := srcID('0')
	wantErr := errors.New("save failed")
	st := &fakeStore{tx: newFakeTx(&models.Source{ID: src})}
	st.tx.saveErr = wantErr

	if _, err := New(st, &stubIDs{id: citID('V')}).CreateCitation(context.Background(), validInput(src)); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

func TestCreateCitationPropagatesTxError(t *testing.T) {
	src := srcID('0')
	wantErr := errors.New("tx failed")
	st := &fakeStore{tx: newFakeTx(&models.Source{ID: src}), inTxErr: wantErr}

	if _, err := New(st, &stubIDs{id: citID('V')}).CreateCitation(context.Background(), validInput(src)); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}
