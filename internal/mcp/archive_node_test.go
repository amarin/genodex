package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakeArchiveNodes struct {
	list []models.ArchiveNode
	err  error

	gotList models.ArchiveNodeQuery

	getN      models.ArchiveNode
	created   models.ArchiveNode
	gotCreate models.ArchiveNode
	updated   models.ArchiveNode
	gotIDs    []models.ID
	deleteErr error

	search []models.ArchiveNode
}

func (f *fakeArchiveNodes) ListArchiveNodes(_ context.Context, _ models.Access, q models.ArchiveNodeQuery) ([]models.ArchiveNode, error) {
	f.gotList = q

	return f.list, f.err
}

func (f *fakeArchiveNodes) SearchArchiveNodes(context.Context, models.Access, models.SearchQuery) ([]models.ArchiveNode, error) {
	return f.search, f.err
}

func (f *fakeArchiveNodes) GetArchiveNode(_ context.Context, _ models.Access, id models.ID) (models.ArchiveNode, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.ArchiveNode{}, f.err
	}

	return f.getN, nil
}

func (f *fakeArchiveNodes) CreateArchiveNode(_ context.Context, n models.ArchiveNode) (models.ArchiveNode, error) {
	f.gotCreate = n
	if f.err != nil {
		return models.ArchiveNode{}, f.err
	}

	return f.created, nil
}

func (f *fakeArchiveNodes) UpdateArchiveNode(_ context.Context, n models.ArchiveNode) error {
	f.updated = n

	return f.err
}

func (f *fakeArchiveNodes) DeleteArchiveNode(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callArchiveNodeTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestArchiveNodeListToolRequiresArchiveID(t *testing.T) {
	svc := &fakeArchiveNodes{err: &models.ValidationError{Entity: models.TypeArchiveNode, Field: "archive_id", Reason: "обязателен"}}

	res := callArchiveNodeTool(t, archiveNodeListHandler(svc), map[string]any{})

	if !res.IsError {
		t.Fatalf("isError=%v, want true (archive_id пуст)", res.IsError)
	}
}

func TestArchiveNodeListToolPassesArchiveIDAndParentID(t *testing.T) {
	svc := &fakeArchiveNodes{}

	res := callArchiveNodeTool(t, archiveNodeListHandler(svc), map[string]any{
		"archive_id": "AR-1",
		"parent_id":  "AN-0",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotList.ArchiveID != "AR-1" || svc.gotList.ParentID == nil || *svc.gotList.ParentID != "AN-0" {
		t.Fatalf("gotList = %+v", svc.gotList)
	}
}

func TestArchiveNodeGetToolContract(t *testing.T) {
	svc := &fakeArchiveNodes{getN: models.ArchiveNode{ID: "AN-1", Label: "Фонд 1"}}

	res := callArchiveNodeTool(t, archiveNodeGetHandler(svc), map[string]any{"id": "AN-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "AN-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

func TestArchiveNodeCreateToolPassesFields(t *testing.T) {
	svc := &fakeArchiveNodes{created: models.ArchiveNode{ID: "AN-new", Label: "Фонд 1"}}

	res := callArchiveNodeTool(t, archiveNodeCreateHandler(svc), map[string]any{
		"type":       "fond",
		"archive_id": "AR-1",
		"label":      "Фонд 1",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.ArchiveID != "AR-1" || svc.gotCreate.Label != "Фонд 1" || string(svc.gotCreate.Type) != "fond" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestArchiveNodeCreateToolParentFromOtherArchiveIsError — usecase-слой
// возвращает ошибку на parent_id из другого архива, тул отдаёт её как
// ошибку тула.
func TestArchiveNodeCreateToolParentFromOtherArchiveIsError(t *testing.T) {
	svc := &fakeArchiveNodes{err: &models.ValidationError{Entity: models.TypeArchiveNode, Field: "parent_id", Reason: "другой архив"}}

	res := callArchiveNodeTool(t, archiveNodeCreateHandler(svc), map[string]any{
		"type":       "fond",
		"archive_id": "AR-1",
		"parent_id":  "AN-999",
		"label":      "Фонд-призрак",
	})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

// TestArchiveNodeUpdateToolPreservesSourcesWhenOmitted — sources отсутствует
// в вызове: текущие источники (из GetArchiveNode) сохраняются как есть, не
// затираются (docs/data-model/entity-write.md §3.3, урок подпроекта 5).
func TestArchiveNodeUpdateToolPreservesSourcesWhenOmitted(t *testing.T) {
	existingSources := []models.SourceLink{{CitationID: "C-1"}}
	svc := &fakeArchiveNodes{getN: models.ArchiveNode{ID: "AN-1", ArchiveID: "AR-1", Label: "Фонд 1", Sources: existingSources}}

	res := callArchiveNodeTool(t, archiveNodeUpdateHandler(svc), map[string]any{
		"id":         "AN-1",
		"type":       "fond",
		"archive_id": "AR-1",
		"label":      "Фонд 1 (испр.)",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if len(svc.updated.Sources) != 1 || svc.updated.Sources[0].CitationID != "C-1" {
		t.Fatalf("updated.Sources = %+v, ожидались сохранённые источники", svc.updated.Sources)
	}
}

// TestArchiveNodeUpdateToolClearsSourcesWhenEmptyArray — пустой массив
// sources в вызове очищает источники (в отличие от отсутствия ключа).
func TestArchiveNodeUpdateToolClearsSourcesWhenEmptyArray(t *testing.T) {
	existingSources := []models.SourceLink{{CitationID: "C-1"}}
	svc := &fakeArchiveNodes{getN: models.ArchiveNode{ID: "AN-1", ArchiveID: "AR-1", Label: "Фонд 1", Sources: existingSources}}

	res := callArchiveNodeTool(t, archiveNodeUpdateHandler(svc), map[string]any{
		"id":         "AN-1",
		"type":       "fond",
		"archive_id": "AR-1",
		"label":      "Фонд 1 (испр.)",
		"sources":    []any{},
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if len(svc.updated.Sources) != 0 {
		t.Fatalf("updated.Sources = %+v, ожидался пустой список", svc.updated.Sources)
	}
}

func TestArchiveNodeDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeArchiveNodes{deleteErr: &models.InUseError{Type: models.TypeArchiveNode, ID: "AN-1"}}

	res := callArchiveNodeTool(t, archiveNodeDeleteHandler(svc), map[string]any{"id": "AN-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersArchiveNodeTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, ArchiveNodes: &fakeArchiveNodes{}}).ListTools()

	for _, name := range []string{
		"archive_node_list", "archive_node_search", "archive_node_get",
		"archive_node_create", "archive_node_update", "archive_node_delete",
	} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
