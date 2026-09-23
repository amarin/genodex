package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

// fakeSurnames запоминает запрос и отдаёт заданный ответ.
type fakeSurnames struct {
	list []models.Surname
	err  error

	getSN     models.Surname
	created   models.Surname
	gotCreate models.Surname
	updated   models.Surname
	gotIDs    []models.ID
	deleteErr error

	search []models.Surname
}

func (f *fakeSurnames) ListSurnames(context.Context, models.Access, models.Page) ([]models.Surname, error) {
	return f.list, f.err
}

func (f *fakeSurnames) SearchSurnames(context.Context, models.Access, models.SearchQuery) ([]models.Surname, error) {
	return f.search, f.err
}

func (f *fakeSurnames) GetSurname(_ context.Context, id models.ID) (models.Surname, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Surname{}, f.err
	}

	return f.getSN, nil
}

func (f *fakeSurnames) CreateSurname(_ context.Context, sn models.Surname) (models.Surname, error) {
	f.gotCreate = sn
	if f.err != nil {
		return models.Surname{}, f.err
	}

	return f.created, nil
}

func (f *fakeSurnames) UpdateSurname(_ context.Context, sn models.Surname) error {
	f.updated = sn

	return f.err
}

func (f *fakeSurnames) DeleteSurname(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callSurnameTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestSurnameGetToolContract(t *testing.T) {
	svc := &fakeSurnames{getSN: models.Surname{ID: "SN-1", Canonical: "Иванов"}}

	res := callSurnameTool(t, surnameGetHandler(svc), map[string]any{"id": "SN-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "SN-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

func TestSurnameCreateToolPassesVariants(t *testing.T) {
	svc := &fakeSurnames{created: models.Surname{ID: "SN-new", Canonical: "Иванов"}}

	res := callSurnameTool(t, surnameCreateHandler(svc), map[string]any{
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

func TestSurnameDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeSurnames{deleteErr: &models.InUseError{Type: models.TypeSurname, ID: "SN-1"}}

	res := callSurnameTool(t, surnameDeleteHandler(svc), map[string]any{"id": "SN-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersSurnameTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Surnames: &fakeSurnames{}}).ListTools()

	for _, name := range []string{"surname_list", "surname_search", "surname_get", "surname_create", "surname_update", "surname_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
