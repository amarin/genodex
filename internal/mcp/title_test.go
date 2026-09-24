package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

// fakeTitles запоминает запрос и отдаёт заданный ответ.
type fakeTitles struct {
	list []models.Title
	err  error

	getSN     models.Title
	created   models.Title
	gotCreate models.Title
	updated   models.Title
	gotIDs    []models.ID
	deleteErr error

	search []models.Title
}

func (f *fakeTitles) ListTitles(context.Context, models.Access, models.Page) ([]models.Title, error) {
	return f.list, f.err
}

func (f *fakeTitles) SearchTitles(context.Context, models.Access, models.SearchQuery) ([]models.Title, error) {
	return f.search, f.err
}

func (f *fakeTitles) GetTitle(_ context.Context, id models.ID) (models.Title, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Title{}, f.err
	}

	return f.getSN, nil
}

func (f *fakeTitles) CreateTitle(_ context.Context, sn models.Title) (models.Title, error) {
	f.gotCreate = sn
	if f.err != nil {
		return models.Title{}, f.err
	}

	return f.created, nil
}

func (f *fakeTitles) UpdateTitle(_ context.Context, sn models.Title) error {
	f.updated = sn

	return f.err
}

func (f *fakeTitles) DeleteTitle(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callTitleTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestTitleGetToolContract(t *testing.T) {
	svc := &fakeTitles{getSN: models.Title{ID: "TT-1", Canonical: "Иванов"}}

	res := callTitleTool(t, titleGetHandler(svc), map[string]any{"id": "TT-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "TT-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

func TestTitleCreateToolPassesVariants(t *testing.T) {
	svc := &fakeTitles{created: models.Title{ID: "TT-new", Canonical: "Иванов"}}

	res := callTitleTool(t, titleCreateHandler(svc), map[string]any{
		"canonical": "Иванов",
		"variants":  []any{"Иванова"},
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.Canonical != "Иванов" || len(svc.gotCreate.Variants) != 1 || svc.gotCreate.Variants[0].Text != "Иванова" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestTitleUpdateToolOmittedNotesKeepsCurrent — notes отсутствует в
// вызове: текущий список сохраняется; явный пустой список — очищает его.
func TestTitleUpdateToolOmittedNotesKeepsCurrent(t *testing.T) {
	svc := &fakeTitles{getSN: models.Title{ID: "TT-1", Canonical: "Иванов", Notes: []models.TextRef{{Text: "заметка"}}}}

	res := callTitleTool(t, titleUpdateHandler(svc), map[string]any{"id": "TT-1", "canonical": "Иванов"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if len(svc.updated.Notes) != 1 || svc.updated.Notes[0].Text != "заметка" {
		t.Fatalf("updated.Notes = %+v, ожидалось сохранение текущих заметок", svc.updated.Notes)
	}
}

// TestTitleUpdateToolEmptyNotesClears — явный пустой список notes очищает поле.
func TestTitleUpdateToolEmptyNotesClears(t *testing.T) {
	svc := &fakeTitles{getSN: models.Title{ID: "TT-1", Canonical: "Иванов", Notes: []models.TextRef{{Text: "заметка"}}}}

	res := callTitleTool(t, titleUpdateHandler(svc), map[string]any{
		"id": "TT-1", "canonical": "Иванов", "notes": []any{},
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if len(svc.updated.Notes) != 0 {
		t.Fatalf("updated.Notes = %+v, ожидался пустой список", svc.updated.Notes)
	}
}

func TestTitleDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeTitles{deleteErr: &models.InUseError{Type: models.TypeTitle, ID: "TT-1"}}

	res := callTitleTool(t, titleDeleteHandler(svc), map[string]any{"id": "TT-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersTitleTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Titles: &fakeTitles{}}).ListTools()

	for _, name := range []string{"title_list", "title_search", "title_get", "title_create", "title_update", "title_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
