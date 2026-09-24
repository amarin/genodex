package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakeAttachments struct {
	list []models.Attachment
	err  error

	getA      models.Attachment
	created   models.Attachment
	gotCreate models.Attachment
	updated   models.Attachment
	gotIDs    []models.ID
	deleteErr error

	search []models.Attachment
}

func (f *fakeAttachments) ListAttachments(context.Context, models.Access, models.Page) ([]models.Attachment, error) {
	return f.list, f.err
}

func (f *fakeAttachments) SearchAttachments(context.Context, models.Access, models.SearchQuery) ([]models.Attachment, error) {
	return f.search, f.err
}

func (f *fakeAttachments) GetAttachment(_ context.Context, _ models.Access, id models.ID) (models.Attachment, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Attachment{}, f.err
	}

	return f.getA, nil
}

func (f *fakeAttachments) CreateAttachment(_ context.Context, a models.Attachment) (models.Attachment, error) {
	f.gotCreate = a
	if f.err != nil {
		return models.Attachment{}, f.err
	}

	return f.created, nil
}

func (f *fakeAttachments) UpdateAttachment(_ context.Context, a models.Attachment) error {
	f.updated = a

	return f.err
}

func (f *fakeAttachments) DeleteAttachment(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callAttachmentTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestAttachmentGetToolContract(t *testing.T) {
	svc := &fakeAttachments{getA: models.Attachment{ID: "O-1", Kind: models.AttachmentKindScan, Filename: "0012.jpg"}}

	res := callAttachmentTool(t, attachmentGetHandler(svc), map[string]any{"id": "O-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "O-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

// TestAttachmentCreateToolPassesNodeID — node_id/document_id передаются
// плоскими строками (не объектами).
func TestAttachmentCreateToolPassesNodeID(t *testing.T) {
	svc := &fakeAttachments{created: models.Attachment{ID: "O-new", Kind: models.AttachmentKindScan, Filename: "0012.jpg"}}

	res := callAttachmentTool(t, attachmentCreateHandler(svc), map[string]any{
		"kind":        "scan",
		"filename":    "0012.jpg",
		"node_id":     "AN-1",
		"document_id": "DC-1",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.NodeID != "AN-1" || svc.gotCreate.DocumentID == nil || *svc.gotCreate.DocumentID != "DC-1" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

func TestAttachmentCreateToolNodeNotFoundIsError(t *testing.T) {
	svc := &fakeAttachments{err: &models.ValidationError{Entity: models.TypeAttachment, Field: "node_id", Reason: "не найден"}}

	res := callAttachmentTool(t, attachmentCreateHandler(svc), map[string]any{
		"kind":     "scan",
		"filename": "0012.jpg",
		"node_id":  "AN-999",
	})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

// TestAttachmentUpdateToolOmittedDocumentIDKeepsCurrent — document_id
// отсутствует в вызове: текущий документ сохраняется, в отличие от явно
// пустой строки.
func TestAttachmentUpdateToolOmittedDocumentIDKeepsCurrent(t *testing.T) {
	doc := models.ID("DC-1")
	svc := &fakeAttachments{getA: models.Attachment{
		ID: "O-1", Kind: models.AttachmentKindScan, Filename: "0012.jpg", NodeID: "AN-1", DocumentID: &doc,
	}}

	res := callAttachmentTool(t, attachmentUpdateHandler(svc), map[string]any{
		"id": "O-1", "kind": "scan", "node_id": "AN-1",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.DocumentID == nil || *svc.updated.DocumentID != "DC-1" {
		t.Fatalf("updated.DocumentID = %v, ожидалось сохранение текущего документа", svc.updated.DocumentID)
	}
}

// TestAttachmentUpdateToolEmptyDocumentIDClears — явно пустой document_id
// убирает ссылку на документ, в отличие от отсутствия ключа.
func TestAttachmentUpdateToolEmptyDocumentIDClears(t *testing.T) {
	doc := models.ID("DC-1")
	svc := &fakeAttachments{getA: models.Attachment{
		ID: "O-1", Kind: models.AttachmentKindScan, Filename: "0012.jpg", NodeID: "AN-1", DocumentID: &doc,
	}}

	res := callAttachmentTool(t, attachmentUpdateHandler(svc), map[string]any{
		"id": "O-1", "kind": "scan", "node_id": "AN-1", "document_id": "",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.DocumentID != nil {
		t.Fatalf("updated.DocumentID = %v, ожидался nil (без документа)", svc.updated.DocumentID)
	}
}

func TestAttachmentDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeAttachments{deleteErr: &models.InUseError{Type: models.TypeAttachment, ID: "O-1"}}

	res := callAttachmentTool(t, attachmentDeleteHandler(svc), map[string]any{"id": "O-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersAttachmentTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Attachments: &fakeAttachments{}}).ListTools()

	for _, name := range []string{"attachment_list", "attachment_search", "attachment_get", "attachment_create", "attachment_update", "attachment_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
