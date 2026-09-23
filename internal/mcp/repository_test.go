package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepositories struct {
	list []models.Repository
	err  error

	getR      models.Repository
	created   models.Repository
	gotCreate models.Repository
	updated   models.Repository
	gotIDs    []models.ID
	deleteErr error

	search []models.Repository
}

func (f *fakeRepositories) ListRepositories(context.Context, models.Access, models.Page) ([]models.Repository, error) {
	return f.list, f.err
}

func (f *fakeRepositories) SearchRepositories(context.Context, models.Access, models.SearchQuery) ([]models.Repository, error) {
	return f.search, f.err
}

func (f *fakeRepositories) GetRepository(_ context.Context, id models.ID) (models.Repository, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Repository{}, f.err
	}

	return f.getR, nil
}

func (f *fakeRepositories) CreateRepository(_ context.Context, r models.Repository) (models.Repository, error) {
	f.gotCreate = r
	if f.err != nil {
		return models.Repository{}, f.err
	}

	return f.created, nil
}

func (f *fakeRepositories) UpdateRepository(_ context.Context, r models.Repository) error {
	f.updated = r

	return f.err
}

func (f *fakeRepositories) DeleteRepository(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callRepositoryTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestRepositoryGetToolContract(t *testing.T) {
	svc := &fakeRepositories{getR: models.Repository{ID: "R-1", Name: "ГАВО", Type: models.RepositoryTypeArchive}}

	res := callRepositoryTool(t, repositoryGetHandler(svc), map[string]any{"id": "R-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "R-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

func TestRepositoryCreateToolPassesType(t *testing.T) {
	svc := &fakeRepositories{created: models.Repository{ID: "R-new", Name: "ГАВО"}}

	res := callRepositoryTool(t, repositoryCreateHandler(svc), map[string]any{
		"name": "ГАВО",
		"type": "archive",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.Name != "ГАВО" || svc.gotCreate.Type != models.RepositoryTypeArchive {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

func TestRepositoryUpdateToolSetsFields(t *testing.T) {
	svc := &fakeRepositories{getR: models.Repository{ID: "R-1", Name: "ГАВО", Type: models.RepositoryTypeArchive}}

	res := callRepositoryTool(t, repositoryUpdateHandler(svc), map[string]any{
		"id":      "R-1",
		"name":    "ГАВО",
		"type":    "library",
		"private": true,
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Type != models.RepositoryTypeLibrary || svc.updated.Private != true {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestRepositoryDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeRepositories{deleteErr: &models.InUseError{Type: models.TypeRepository, ID: "R-1"}}

	res := callRepositoryTool(t, repositoryDeleteHandler(svc), map[string]any{"id": "R-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersRepositoryTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Repositories: &fakeRepositories{}}).ListTools()

	for _, name := range []string{"repository_list", "repository_search", "repository_get", "repository_create", "repository_update", "repository_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
