package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakeFamilies struct {
	list []models.Family
	err  error

	getF      models.Family
	created   models.Family
	gotCreate models.Family
	updated   models.Family
	gotIDs    []models.ID
	deleteErr error

	search []models.Family
}

func (f *fakeFamilies) ListFamilies(context.Context, models.Access, models.Page) ([]models.Family, error) {
	return f.list, f.err
}

func (f *fakeFamilies) SearchFamilies(context.Context, models.Access, models.SearchQuery) ([]models.Family, error) {
	return f.search, f.err
}

func (f *fakeFamilies) GetFamily(_ context.Context, _ models.Access, id models.ID) (models.Family, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Family{}, f.err
	}

	return f.getF, nil
}

func (f *fakeFamilies) CreateFamily(_ context.Context, fam models.Family) (models.Family, error) {
	f.gotCreate = fam
	if f.err != nil {
		return models.Family{}, f.err
	}

	return f.created, nil
}

func (f *fakeFamilies) UpdateFamily(_ context.Context, fam models.Family) error {
	f.updated = fam

	return f.err
}

func (f *fakeFamilies) DeleteFamily(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callFamilyTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestFamilyGetToolContract(t *testing.T) {
	svc := &fakeFamilies{getF: models.Family{ID: "F-1", Name: "Ивановы"}}

	res := callFamilyTool(t, familyGetHandler(svc), map[string]any{"id": "F-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "F-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

func TestFamilyCreateToolPassesName(t *testing.T) {
	svc := &fakeFamilies{created: models.Family{ID: "F-new", Name: "Ивановы"}}

	res := callFamilyTool(t, familyCreateHandler(svc), map[string]any{
		"name": "Ивановы",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.Name != "Ивановы" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

func TestFamilyUpdateToolSetsFields(t *testing.T) {
	svc := &fakeFamilies{getF: models.Family{ID: "F-1", Name: "Ивановы"}}

	res := callFamilyTool(t, familyUpdateHandler(svc), map[string]any{
		"id":      "F-1",
		"name":    "Ивановы",
		"private": true,
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Private != true {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestFamilyDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeFamilies{deleteErr: &models.InUseError{Type: models.TypeFamily, ID: "F-1"}}

	res := callFamilyTool(t, familyDeleteHandler(svc), map[string]any{"id": "F-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersFamilyTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Families: &fakeFamilies{}}).ListTools()

	for _, name := range []string{"family_list", "family_search", "family_get", "family_create", "family_update", "family_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
