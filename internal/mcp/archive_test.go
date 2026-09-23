package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakeArchives struct {
	list []models.Archive
	err  error

	getA      models.Archive
	created   models.Archive
	gotCreate models.Archive
	updated   models.Archive
	gotIDs    []models.ID
	deleteErr error

	search []models.Archive
}

func (f *fakeArchives) ListArchives(context.Context, models.Access, models.Page) ([]models.Archive, error) {
	return f.list, f.err
}

func (f *fakeArchives) SearchArchives(context.Context, models.Access, models.SearchQuery) ([]models.Archive, error) {
	return f.search, f.err
}

func (f *fakeArchives) GetArchive(_ context.Context, id models.ID) (models.Archive, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Archive{}, f.err
	}

	return f.getA, nil
}

func (f *fakeArchives) CreateArchive(_ context.Context, a models.Archive) (models.Archive, error) {
	f.gotCreate = a
	if f.err != nil {
		return models.Archive{}, f.err
	}

	return f.created, nil
}

func (f *fakeArchives) UpdateArchive(_ context.Context, a models.Archive) error {
	f.updated = a

	return f.err
}

func (f *fakeArchives) DeleteArchive(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callArchiveTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestArchiveGetToolContract(t *testing.T) {
	svc := &fakeArchives{getA: models.Archive{ID: "AR-1", Name: "ГАВО, архив"}}

	res := callArchiveTool(t, archiveGetHandler(svc), map[string]any{"id": "AR-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "AR-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

// TestArchiveCreateToolPassesRepositoryID — repository_id передаётся плоской
// строкой (не объектом), в отличие от parish/church/system.
func TestArchiveCreateToolPassesRepositoryID(t *testing.T) {
	svc := &fakeArchives{created: models.Archive{ID: "AR-new", Name: "ГАВО, архив"}}

	res := callArchiveTool(t, archiveCreateHandler(svc), map[string]any{
		"name":          "ГАВО, архив",
		"repository_id": "R-1",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.Name != "ГАВО, архив" || svc.gotCreate.RepositoryID != "R-1" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestArchiveCreateToolRepositoryNotFoundIsError — usecase-слой возвращает
// ошибку на несуществующее хранилище, тул должен отдать её как ошибку тула
// (не паниковать, не проглатывать).
func TestArchiveCreateToolRepositoryNotFoundIsError(t *testing.T) {
	svc := &fakeArchives{err: &models.ValidationError{Entity: models.TypeArchive, Field: "repository_id", Reason: "не найдено"}}

	res := callArchiveTool(t, archiveCreateHandler(svc), map[string]any{
		"name":          "ГАВО, архив",
		"repository_id": "R-999",
	})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestArchiveDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeArchives{deleteErr: &models.InUseError{Type: models.TypeArchive, ID: "AR-1"}}

	res := callArchiveTool(t, archiveDeleteHandler(svc), map[string]any{"id": "AR-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersArchiveTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Archives: &fakeArchives{}}).ListTools()

	for _, name := range []string{"archive_list", "archive_search", "archive_get", "archive_create", "archive_update", "archive_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
