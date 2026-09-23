package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

// fakeEstates запоминает запрос и отдаёт заданный ответ.
type fakeEstates struct {
	list []models.Estate
	err  error

	getSN     models.Estate
	created   models.Estate
	gotCreate models.Estate
	updated   models.Estate
	gotIDs    []models.ID
	deleteErr error

	search []models.Estate
}

func (f *fakeEstates) ListEstates(context.Context, models.Access, models.Page) ([]models.Estate, error) {
	return f.list, f.err
}

func (f *fakeEstates) SearchEstates(context.Context, models.Access, models.SearchQuery) ([]models.Estate, error) {
	return f.search, f.err
}

func (f *fakeEstates) GetEstate(_ context.Context, id models.ID) (models.Estate, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Estate{}, f.err
	}

	return f.getSN, nil
}

func (f *fakeEstates) CreateEstate(_ context.Context, sn models.Estate) (models.Estate, error) {
	f.gotCreate = sn
	if f.err != nil {
		return models.Estate{}, f.err
	}

	return f.created, nil
}

func (f *fakeEstates) UpdateEstate(_ context.Context, sn models.Estate) error {
	f.updated = sn

	return f.err
}

func (f *fakeEstates) DeleteEstate(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callEstateTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestEstateGetToolContract(t *testing.T) {
	svc := &fakeEstates{getSN: models.Estate{ID: "ES-1", Canonical: "Иванов"}}

	res := callEstateTool(t, estateGetHandler(svc), map[string]any{"id": "ES-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "ES-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

func TestEstateCreateToolPassesVariants(t *testing.T) {
	svc := &fakeEstates{created: models.Estate{ID: "ES-new", Canonical: "Иванов"}}

	res := callEstateTool(t, estateCreateHandler(svc), map[string]any{
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

func TestEstateDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeEstates{deleteErr: &models.InUseError{Type: models.TypeEstate, ID: "ES-1"}}

	res := callEstateTool(t, estateDeleteHandler(svc), map[string]any{"id": "ES-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersEstateTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Estates: &fakeEstates{}}).ListTools()

	for _, name := range []string{"estate_list", "estate_search", "estate_get", "estate_create", "estate_update", "estate_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
