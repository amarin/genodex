package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

// fakePatronymics запоминает запрос и отдаёт заданный ответ.
type fakePatronymics struct {
	list []models.Patronymic
	err  error

	getSN     models.Patronymic
	created   models.Patronymic
	gotCreate models.Patronymic
	updated   models.Patronymic
	gotIDs    []models.ID
	deleteErr error

	search []models.Patronymic
}

func (f *fakePatronymics) ListPatronymics(context.Context, models.Access, models.Page) ([]models.Patronymic, error) {
	return f.list, f.err
}

func (f *fakePatronymics) SearchPatronymics(context.Context, models.Access, models.SearchQuery) ([]models.Patronymic, error) {
	return f.search, f.err
}

func (f *fakePatronymics) GetPatronymic(_ context.Context, id models.ID) (models.Patronymic, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Patronymic{}, f.err
	}

	return f.getSN, nil
}

func (f *fakePatronymics) CreatePatronymic(_ context.Context, sn models.Patronymic) (models.Patronymic, error) {
	f.gotCreate = sn
	if f.err != nil {
		return models.Patronymic{}, f.err
	}

	return f.created, nil
}

func (f *fakePatronymics) UpdatePatronymic(_ context.Context, sn models.Patronymic) error {
	f.updated = sn

	return f.err
}

func (f *fakePatronymics) DeletePatronymic(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callPatronymicTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestPatronymicGetToolContract(t *testing.T) {
	svc := &fakePatronymics{getSN: models.Patronymic{ID: "PN-1", Canonical: "Иванов"}}

	res := callPatronymicTool(t, patronymicGetHandler(svc), map[string]any{"id": "PN-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "PN-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

func TestPatronymicCreateToolPassesVariants(t *testing.T) {
	svc := &fakePatronymics{created: models.Patronymic{ID: "PN-new", Canonical: "Иванов"}}

	res := callPatronymicTool(t, patronymicCreateHandler(svc), map[string]any{
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

func TestPatronymicDeleteToolInUseIsError(t *testing.T) {
	svc := &fakePatronymics{deleteErr: &models.InUseError{Type: models.TypePatronymic, ID: "PN-1"}}

	res := callPatronymicTool(t, patronymicDeleteHandler(svc), map[string]any{"id": "PN-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersPatronymicTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Patronymics: &fakePatronymics{}}).ListTools()

	for _, name := range []string{"patronymic_list", "patronymic_search", "patronymic_get", "patronymic_create", "patronymic_update", "patronymic_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
