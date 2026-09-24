package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakeNotes struct {
	list []models.Note
	err  error

	getN      models.Note
	created   models.Note
	gotCreate models.Note
	updated   models.Note
	gotIDs    []models.ID
	deleteErr error

	search []models.Note
}

func (f *fakeNotes) ListNotes(context.Context, models.Access, models.Page) ([]models.Note, error) {
	return f.list, f.err
}

func (f *fakeNotes) SearchNotes(context.Context, models.Access, models.SearchQuery) ([]models.Note, error) {
	return f.search, f.err
}

func (f *fakeNotes) GetNote(_ context.Context, _ models.Access, id models.ID) (models.Note, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Note{}, f.err
	}

	return f.getN, nil
}

func (f *fakeNotes) CreateNote(_ context.Context, n models.Note) (models.Note, error) {
	f.gotCreate = n
	if f.err != nil {
		return models.Note{}, f.err
	}

	return f.created, nil
}

func (f *fakeNotes) UpdateNote(_ context.Context, n models.Note) error {
	f.updated = n

	return f.err
}

func (f *fakeNotes) DeleteNote(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callNoteTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestNoteGetToolContract(t *testing.T) {
	svc := &fakeNotes{getN: models.Note{ID: "N-1", Kind: "note", Text: "текст"}}

	res := callNoteTool(t, noteGetHandler(svc), map[string]any{"id": "N-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "N-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

// TestNoteCreateToolPassesParentID — parent_id передаётся плоской строкой
// (не объектом), в отличие от church/parish.
func TestNoteCreateToolPassesParentID(t *testing.T) {
	svc := &fakeNotes{created: models.Note{ID: "N-new", Kind: "note", Text: "текст"}}

	res := callNoteTool(t, noteCreateHandler(svc), map[string]any{
		"kind":      "note",
		"text":      "текст",
		"parent_id": "N-1",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.Text != "текст" || svc.gotCreate.ParentID == nil || *svc.gotCreate.ParentID != "N-1" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestNoteCreateToolPassesPrivate — регресс: private изначально отсутствовал
// среди аргументов note_create/note_update.
func TestNoteCreateToolPassesPrivate(t *testing.T) {
	svc := &fakeNotes{created: models.Note{ID: "N-new", Kind: "note", Text: "текст", Private: true}}

	res := callNoteTool(t, noteCreateHandler(svc), map[string]any{
		"kind":    "note",
		"text":    "текст",
		"private": true,
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if !svc.gotCreate.Private {
		t.Fatalf("gotCreate.Private = %v, want true", svc.gotCreate.Private)
	}
}

// TestNoteUpdateToolOmittedParentIDKeepsCurrent — parent_id отсутствует в
// вызове: текущий родитель сохраняется, в отличие от явно пустой строки.
func TestNoteUpdateToolOmittedParentIDKeepsCurrent(t *testing.T) {
	parent := models.ID("N-0")
	svc := &fakeNotes{getN: models.Note{ID: "N-1", Kind: "note", Text: "текст", ParentID: &parent}}

	res := callNoteTool(t, noteUpdateHandler(svc), map[string]any{
		"id": "N-1", "kind": "note",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.ParentID == nil || *svc.updated.ParentID != "N-0" {
		t.Fatalf("updated.ParentID = %v, ожидалось сохранение текущего родителя", svc.updated.ParentID)
	}
}

// TestNoteUpdateToolEmptyParentIDClears — явно пустой parent_id убирает
// родителя, в отличие от отсутствия ключа.
func TestNoteUpdateToolEmptyParentIDClears(t *testing.T) {
	parent := models.ID("N-0")
	svc := &fakeNotes{getN: models.Note{ID: "N-1", Kind: "note", Text: "текст", ParentID: &parent}}

	res := callNoteTool(t, noteUpdateHandler(svc), map[string]any{
		"id": "N-1", "kind": "note", "parent_id": "",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.ParentID != nil {
		t.Fatalf("updated.ParentID = %v, ожидался nil (без родителя)", svc.updated.ParentID)
	}
}

func TestNoteDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeNotes{deleteErr: &models.InUseError{Type: models.TypeNote, ID: "N-1"}}

	res := callNoteTool(t, noteDeleteHandler(svc), map[string]any{"id": "N-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersNoteTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Notes: &fakeNotes{}}).ListTools()

	for _, name := range []string{"note_list", "note_search", "note_get", "note_create", "note_update", "note_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
