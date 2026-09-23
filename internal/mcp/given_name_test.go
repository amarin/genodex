package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

// fakeGivenNames запоминает запрос и отдаёт заданный ответ.
type fakeGivenNames struct {
	list []models.GivenName
	err  error

	getSN     models.GivenName
	created   models.GivenName
	gotCreate models.GivenName
	updated   models.GivenName
	gotIDs    []models.ID
	deleteErr error

	search []models.GivenName
}

func (f *fakeGivenNames) ListGivenNames(context.Context, models.Access, models.Page) ([]models.GivenName, error) {
	return f.list, f.err
}

func (f *fakeGivenNames) SearchGivenNames(context.Context, models.Access, models.SearchQuery) ([]models.GivenName, error) {
	return f.search, f.err
}

func (f *fakeGivenNames) GetGivenName(_ context.Context, id models.ID) (models.GivenName, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.GivenName{}, f.err
	}

	return f.getSN, nil
}

func (f *fakeGivenNames) CreateGivenName(_ context.Context, sn models.GivenName) (models.GivenName, error) {
	f.gotCreate = sn
	if f.err != nil {
		return models.GivenName{}, f.err
	}

	return f.created, nil
}

func (f *fakeGivenNames) UpdateGivenName(_ context.Context, sn models.GivenName) error {
	f.updated = sn

	return f.err
}

func (f *fakeGivenNames) DeleteGivenName(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callGivenNameTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestGivenNameGetToolContract(t *testing.T) {
	svc := &fakeGivenNames{getSN: models.GivenName{ID: "GN-1", Canonical: "Иванов"}}

	res := callGivenNameTool(t, givenNameGetHandler(svc), map[string]any{"id": "GN-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "GN-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

func TestGivenNameCreateToolPassesVariants(t *testing.T) {
	svc := &fakeGivenNames{created: models.GivenName{ID: "GN-new", Canonical: "Иванов", Gender: models.NameGenderMale}}

	res := callGivenNameTool(t, givenNameCreateHandler(svc), map[string]any{
		"canonical": "Иванов",
		"gender":    "male",
		"variants":  []any{"Иванова"},
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.Canonical != "Иванов" || svc.gotCreate.Gender != models.NameGenderMale ||
		len(svc.gotCreate.Variants) != 1 || svc.gotCreate.Variants[0].Text != "Иванова" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

func TestGivenNameUpdateToolSetsGender(t *testing.T) {
	svc := &fakeGivenNames{getSN: models.GivenName{ID: "GN-1", Canonical: "Иванов", Gender: models.NameGenderMale}}

	res := callGivenNameTool(t, givenNameUpdateHandler(svc), map[string]any{
		"id":        "GN-1",
		"canonical": "Иванов",
		"gender":    "neutral",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Gender != models.NameGenderNeutral {
		t.Fatalf("updated = %+v, want gender=neutral", svc.updated)
	}
}

func TestGivenNameDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeGivenNames{deleteErr: &models.InUseError{Type: models.TypeGivenName, ID: "GN-1"}}

	res := callGivenNameTool(t, givenNameDeleteHandler(svc), map[string]any{"id": "GN-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersGivenNameTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, GivenNames: &fakeGivenNames{}}).ListTools()

	for _, name := range []string{"given_name_list", "given_name_search", "given_name_get", "given_name_create", "given_name_update", "given_name_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
