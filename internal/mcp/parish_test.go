package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakeParishes struct {
	list []models.Parish
	err  error

	getP      models.Parish
	created   models.Parish
	gotCreate models.Parish
	updated   models.Parish
	gotIDs    []models.ID
	deleteErr error

	search []models.Parish
}

func (f *fakeParishes) ListParishes(context.Context, models.Access, models.Page) ([]models.Parish, error) {
	return f.list, f.err
}

func (f *fakeParishes) SearchParishes(context.Context, models.Access, models.SearchQuery) ([]models.Parish, error) {
	return f.search, f.err
}

func (f *fakeParishes) GetParish(_ context.Context, _ models.Access, id models.ID) (models.Parish, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Parish{}, f.err
	}

	return f.getP, nil
}

func (f *fakeParishes) CreateParish(_ context.Context, p models.Parish) (models.Parish, error) {
	f.gotCreate = p
	if f.err != nil {
		return models.Parish{}, f.err
	}

	return f.created, nil
}

func (f *fakeParishes) UpdateParish(_ context.Context, p models.Parish) error {
	f.updated = p

	return f.err
}

func (f *fakeParishes) DeleteParish(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callParishTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestParishGetToolContract(t *testing.T) {
	svc := &fakeParishes{getP: models.Parish{ID: "PR-1", Name: "Никольский приход"}}

	res := callParishTool(t, parishGetHandler(svc), map[string]any{"id": "PR-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "PR-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

// TestParishCreateToolPassesSince — since передаётся объектным аргументом
// FactDate, единственное такое поле у Parish (и первое в проекте в MCP-туле).
func TestParishCreateToolPassesSince(t *testing.T) {
	svc := &fakeParishes{created: models.Parish{ID: "PR-new", Name: "Никольский приход"}}

	res := callParishTool(t, parishCreateHandler(svc), map[string]any{
		"name": "Никольский приход",
		"since": map[string]any{
			"year": float64(1880), "precision": "year", "modifier": "exact",
		},
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.Name != "Никольский приход" || svc.gotCreate.Since == nil ||
		svc.gotCreate.Since.Year != 1880 || svc.gotCreate.Since.Precision != models.PrecisionYear {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestParishUpdateToolOmittedChurchKeepsCurrent — church отсутствует в
// вызове: текущая ссылка сохраняется.
func TestParishUpdateToolOmittedChurchKeepsCurrent(t *testing.T) {
	svc := &fakeParishes{getP: models.Parish{
		ID: "PR-1", Name: "Никольский приход", Church: &models.TextRef{Text: "Никольская церковь"},
	}}

	res := callParishTool(t, parishUpdateHandler(svc), map[string]any{
		"id": "PR-1", "name": "Никольский приход",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Church == nil || svc.updated.Church.Text != "Никольская церковь" {
		t.Fatalf("updated.Church = %+v, ожидалось сохранение текущей ссылки", svc.updated.Church)
	}
}

// TestParishUpdateToolNullChurchClears — явный null для church очищает
// поле (регрессия финального ревью: raw != nil ошибочно приравнивал явный
// null к отсутствию ключа — очистить *TextRef пустым объектом {} нельзя).
func TestParishUpdateToolNullChurchClears(t *testing.T) {
	svc := &fakeParishes{getP: models.Parish{
		ID: "PR-1", Name: "Никольский приход", Church: &models.TextRef{Text: "Никольская церковь"},
	}}

	res := callParishTool(t, parishUpdateHandler(svc), map[string]any{
		"id": "PR-1", "name": "Никольский приход", "church": nil,
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Church != nil {
		t.Fatalf("updated.Church = %+v, ожидался nil (явный null очищает поле)", svc.updated.Church)
	}
}

// TestParishUpdateToolOmittedSinceKeepsCurrent — since отсутствует в
// вызове: текущее значение сохраняется.
func TestParishUpdateToolOmittedSinceKeepsCurrent(t *testing.T) {
	svc := &fakeParishes{getP: models.Parish{
		ID: "PR-1", Name: "Никольский приход",
		Since: &models.FactDate{Year: 1880, Precision: models.PrecisionYear},
	}}

	res := callParishTool(t, parishUpdateHandler(svc), map[string]any{
		"id": "PR-1", "name": "Никольский приход",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Since == nil || svc.updated.Since.Year != 1880 {
		t.Fatalf("updated.Since = %+v, ожидалось сохранение текущего значения", svc.updated.Since)
	}
}

// TestParishUpdateToolNullSinceClears — явный null для since очищает поле.
func TestParishUpdateToolNullSinceClears(t *testing.T) {
	svc := &fakeParishes{getP: models.Parish{
		ID: "PR-1", Name: "Никольский приход",
		Since: &models.FactDate{Year: 1880, Precision: models.PrecisionYear},
	}}

	res := callParishTool(t, parishUpdateHandler(svc), map[string]any{
		"id": "PR-1", "name": "Никольский приход", "since": nil,
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Since != nil {
		t.Fatalf("updated.Since = %+v, ожидался nil (явный null очищает поле)", svc.updated.Since)
	}
}

// TestParishUpdateToolOmittedUntilKeepsCurrent — until отсутствует в
// вызове: текущее значение сохраняется.
func TestParishUpdateToolOmittedUntilKeepsCurrent(t *testing.T) {
	svc := &fakeParishes{getP: models.Parish{
		ID: "PR-1", Name: "Никольский приход",
		Until: &models.FactDate{Year: 1917, Precision: models.PrecisionYear},
	}}

	res := callParishTool(t, parishUpdateHandler(svc), map[string]any{
		"id": "PR-1", "name": "Никольский приход",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Until == nil || svc.updated.Until.Year != 1917 {
		t.Fatalf("updated.Until = %+v, ожидалось сохранение текущего значения", svc.updated.Until)
	}
}

// TestParishUpdateToolNullUntilClears — явный null для until очищает поле.
func TestParishUpdateToolNullUntilClears(t *testing.T) {
	svc := &fakeParishes{getP: models.Parish{
		ID: "PR-1", Name: "Никольский приход",
		Until: &models.FactDate{Year: 1917, Precision: models.PrecisionYear},
	}}

	res := callParishTool(t, parishUpdateHandler(svc), map[string]any{
		"id": "PR-1", "name": "Никольский приход", "until": nil,
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Until != nil {
		t.Fatalf("updated.Until = %+v, ожидался nil (явный null очищает поле)", svc.updated.Until)
	}
}

func TestParishDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeParishes{deleteErr: &models.InUseError{Type: models.TypeParish, ID: "PR-1"}}

	res := callParishTool(t, parishDeleteHandler(svc), map[string]any{"id": "PR-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersParishTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Parishes: &fakeParishes{}}).ListTools()

	for _, name := range []string{"parish_list", "parish_search", "parish_get", "parish_create", "parish_update", "parish_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
