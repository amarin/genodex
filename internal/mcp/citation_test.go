package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakeCitations struct {
	list []models.Citation
	err  error

	getC      models.Citation
	created   models.Citation
	gotCreate models.Citation
	updated   models.Citation
	gotIDs    []models.ID
	deleteErr error

	search []models.Citation
}

func (f *fakeCitations) ListCitations(context.Context, models.Access, models.Page) ([]models.Citation, error) {
	return f.list, f.err
}

func (f *fakeCitations) SearchCitations(context.Context, models.Access, models.SearchQuery) ([]models.Citation, error) {
	return f.search, f.err
}

func (f *fakeCitations) GetCitation(_ context.Context, _ models.Access, id models.ID) (models.Citation, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Citation{}, f.err
	}

	return f.getC, nil
}

func (f *fakeCitations) CreateCitation(_ context.Context, c models.Citation) (models.Citation, error) {
	f.gotCreate = c
	if f.err != nil {
		return models.Citation{}, f.err
	}

	return f.created, nil
}

func (f *fakeCitations) UpdateCitation(_ context.Context, c models.Citation) error {
	f.updated = c

	return f.err
}

func (f *fakeCitations) DeleteCitation(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callCitationTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestCitationGetToolContract(t *testing.T) {
	svc := &fakeCitations{getC: models.Citation{ID: "C-1", SourceID: "S-1"}}

	res := callCitationTool(t, citationGetHandler(svc), map[string]any{"id": "C-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "C-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

// TestCitationCreateToolPassesAnchor — anchor передаётся вложенным объектом
// с дискриминатором kind (см. anchorObjectProperties); вариант url — самый
// простой, без ссылки на другую сущность.
func TestCitationCreateToolPassesAnchor(t *testing.T) {
	svc := &fakeCitations{created: models.Citation{ID: "C-new", SourceID: "S-1"}}

	res := callCitationTool(t, citationCreateHandler(svc), map[string]any{
		"source_id": "S-1",
		"anchor":    map[string]any{"kind": "url", "url": "https://example.org"},
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.SourceID != "S-1" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}

	anchor, ok := svc.gotCreate.Anchor.(*models.URLAnchor)
	if !ok || anchor.URL != "https://example.org" {
		t.Fatalf("gotCreate.Anchor = %+v", svc.gotCreate.Anchor)
	}
}

// TestCitationCreateToolSourceNotFoundIsError — usecase-слой возвращает
// ошибку на несуществующий source_id, тул должен отдать её как ошибку тула
// (не паниковать, не проглатывать).
func TestCitationCreateToolSourceNotFoundIsError(t *testing.T) {
	svc := &fakeCitations{err: &models.ValidationError{Entity: models.TypeCitation, Field: "source_id", Reason: "не найдено"}}

	res := callCitationTool(t, citationCreateHandler(svc), map[string]any{
		"source_id": "S-999",
	})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

// TestCitationUpdateToolOmittedNoteKeepsCurrent — note отсутствует в
// вызове: текущее значение сохраняется; явно пустая строка — очищает его.
func TestCitationUpdateToolOmittedNoteKeepsCurrent(t *testing.T) {
	svc := &fakeCitations{getC: models.Citation{ID: "C-1", SourceID: "S-1", Note: "старая заметка"}}

	res := callCitationTool(t, citationUpdateHandler(svc), map[string]any{"id": "C-1", "source_id": "S-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Note != "старая заметка" {
		t.Fatalf("updated.Note = %q, ожидалось сохранение текущего значения", svc.updated.Note)
	}
}

// TestCitationUpdateToolEmptyNoteClears — явно пустая строка note очищает поле.
func TestCitationUpdateToolEmptyNoteClears(t *testing.T) {
	svc := &fakeCitations{getC: models.Citation{ID: "C-1", SourceID: "S-1", Note: "старая заметка"}}

	res := callCitationTool(t, citationUpdateHandler(svc), map[string]any{
		"id": "C-1", "source_id": "S-1", "note": "",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Note != "" {
		t.Fatalf("updated.Note = %q, ожидалась очистка", svc.updated.Note)
	}
}

func TestCitationDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeCitations{deleteErr: &models.InUseError{Type: models.TypeCitation, ID: "C-1"}}

	res := callCitationTool(t, citationDeleteHandler(svc), map[string]any{"id": "C-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersCitationTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Citations: &fakeCitations{}}).ListTools()

	for _, name := range []string{"citation_list", "citation_search", "citation_get", "citation_create", "citation_update", "citation_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
