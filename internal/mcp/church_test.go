package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakeChurches struct {
	list []models.Church
	err  error

	getC      models.Church
	created   models.Church
	gotCreate models.Church
	updated   models.Church
	gotIDs    []models.ID
	deleteErr error

	search []models.Church
}

func (f *fakeChurches) ListChurches(context.Context, models.Access, models.Page) ([]models.Church, error) {
	return f.list, f.err
}

func (f *fakeChurches) SearchChurches(context.Context, models.Access, models.SearchQuery) ([]models.Church, error) {
	return f.search, f.err
}

func (f *fakeChurches) GetChurch(_ context.Context, id models.ID) (models.Church, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Church{}, f.err
	}

	return f.getC, nil
}

func (f *fakeChurches) CreateChurch(_ context.Context, c models.Church) (models.Church, error) {
	f.gotCreate = c
	if f.err != nil {
		return models.Church{}, f.err
	}

	return f.created, nil
}

func (f *fakeChurches) UpdateChurch(_ context.Context, c models.Church) error {
	f.updated = c

	return f.err
}

func (f *fakeChurches) DeleteChurch(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callChurchTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestChurchGetToolContract(t *testing.T) {
	svc := &fakeChurches{getC: models.Church{ID: "CH-1", Name: "Никольская церковь"}}

	res := callChurchTool(t, churchGetHandler(svc), map[string]any{"id": "CH-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "CH-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

// TestChurchCreateToolPassesParish — parish передаётся объектным аргументом
// {text, ...}, единственное поле такой формы у Church.
func TestChurchCreateToolPassesParish(t *testing.T) {
	svc := &fakeChurches{created: models.Church{ID: "CH-new", Name: "Никольская церковь"}}

	res := callChurchTool(t, churchCreateHandler(svc), map[string]any{
		"name":   "Никольская церковь",
		"parish": map[string]any{"text": "Никольский приход"},
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.Name != "Никольская церковь" || svc.gotCreate.Parish == nil ||
		svc.gotCreate.Parish.Text != "Никольский приход" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestChurchUpdateToolOmittedParishKeepsCurrent — parish отсутствует в
// вызове: текущая ссылка (из GetChurch) сохраняется.
func TestChurchUpdateToolOmittedParishKeepsCurrent(t *testing.T) {
	svc := &fakeChurches{getC: models.Church{
		ID: "CH-1", Name: "Никольская церковь", Parish: &models.TextRef{Text: "Никольский приход"},
	}}

	res := callChurchTool(t, churchUpdateHandler(svc), map[string]any{
		"id": "CH-1", "name": "Никольская церковь",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Parish == nil || svc.updated.Parish.Text != "Никольский приход" {
		t.Fatalf("updated.Parish = %+v, ожидалось сохранение текущей ссылки", svc.updated.Parish)
	}
}

// TestChurchUpdateToolNullParishClears — явный null для parish очищает
// поле (регрессия финального ревью: raw != nil ошибочно приравнивал явный
// null к отсутствию ключа — очистить *TextRef пустым объектом {} нельзя).
func TestChurchUpdateToolNullParishClears(t *testing.T) {
	svc := &fakeChurches{getC: models.Church{
		ID: "CH-1", Name: "Никольская церковь", Parish: &models.TextRef{Text: "Никольский приход"},
	}}

	res := callChurchTool(t, churchUpdateHandler(svc), map[string]any{
		"id": "CH-1", "name": "Никольская церковь", "parish": nil,
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Parish != nil {
		t.Fatalf("updated.Parish = %+v, ожидался nil (явный null очищает поле)", svc.updated.Parish)
	}
}

func TestChurchDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeChurches{deleteErr: &models.InUseError{Type: models.TypeChurch, ID: "CH-1"}}

	res := callChurchTool(t, churchDeleteHandler(svc), map[string]any{"id": "CH-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersChurchTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Churches: &fakeChurches{}}).ListTools()

	for _, name := range []string{"church_list", "church_search", "church_get", "church_create", "church_update", "church_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
