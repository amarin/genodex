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

// TestArchiveNodeUpdateToolOmittedSinceKeepsCurrent — since отсутствует в
// вызове: текущее значение (из GetArchiveNode) сохраняется.
func TestArchiveNodeUpdateToolOmittedSinceKeepsCurrent(t *testing.T) {
	svc := &fakeArchiveNodes{getN: models.ArchiveNode{
		ID: "AN-1", ArchiveID: "AR-1", Label: "Фонд 1",
		Since: &models.FactDate{Year: 1880, Precision: models.PrecisionYear},
	}}

	res := callArchiveNodeTool(t, archiveNodeUpdateHandler(svc), map[string]any{
		"id": "AN-1", "type": "fond", "archive_id": "AR-1", "label": "Фонд 1",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Since == nil || svc.updated.Since.Year != 1880 {
		t.Fatalf("updated.Since = %+v, ожидалось сохранение текущего значения", svc.updated.Since)
	}
}

// TestArchiveNodeUpdateToolNullSinceClears — явный null для since очищает
// поле (регрессия финального ревью: raw != nil ошибочно приравнивал явный
// null к отсутствию ключа для *FactDate-полей — очистить FactDate{} пустым
// объектом нельзя, не пройдёт валидацию).
func TestArchiveNodeUpdateToolNullSinceClears(t *testing.T) {
	svc := &fakeArchiveNodes{getN: models.ArchiveNode{
		ID: "AN-1", ArchiveID: "AR-1", Label: "Фонд 1",
		Since: &models.FactDate{Year: 1880, Precision: models.PrecisionYear},
	}}

	res := callArchiveNodeTool(t, archiveNodeUpdateHandler(svc), map[string]any{
		"id": "AN-1", "type": "fond", "archive_id": "AR-1", "label": "Фонд 1", "since": nil,
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Since != nil {
		t.Fatalf("updated.Since = %+v, ожидался nil (явный null очищает поле)", svc.updated.Since)
	}
}

// TestArchiveNodeUpdateToolOmittedUntilKeepsCurrent — until отсутствует в
// вызове: текущее значение сохраняется.
func TestArchiveNodeUpdateToolOmittedUntilKeepsCurrent(t *testing.T) {
	svc := &fakeArchiveNodes{getN: models.ArchiveNode{
		ID: "AN-1", ArchiveID: "AR-1", Label: "Фонд 1",
		Until: &models.FactDate{Year: 1917, Precision: models.PrecisionYear},
	}}

	res := callArchiveNodeTool(t, archiveNodeUpdateHandler(svc), map[string]any{
		"id": "AN-1", "type": "fond", "archive_id": "AR-1", "label": "Фонд 1",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Until == nil || svc.updated.Until.Year != 1917 {
		t.Fatalf("updated.Until = %+v, ожидалось сохранение текущего значения", svc.updated.Until)
	}
}

// TestArchiveNodeUpdateToolNullUntilClears — явный null для until очищает поле.
func TestArchiveNodeUpdateToolNullUntilClears(t *testing.T) {
	svc := &fakeArchiveNodes{getN: models.ArchiveNode{
		ID: "AN-1", ArchiveID: "AR-1", Label: "Фонд 1",
		Until: &models.FactDate{Year: 1917, Precision: models.PrecisionYear},
	}}

	res := callArchiveNodeTool(t, archiveNodeUpdateHandler(svc), map[string]any{
		"id": "AN-1", "type": "fond", "archive_id": "AR-1", "label": "Фонд 1", "until": nil,
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Until != nil {
		t.Fatalf("updated.Until = %+v, ожидался nil (явный null очищает поле)", svc.updated.Until)
	}
}

// TestArchiveNodeUpdateToolOmittedParishKeepsCurrent — parish отсутствует в
// вызове: текущая ссылка сохраняется.
func TestArchiveNodeUpdateToolOmittedParishKeepsCurrent(t *testing.T) {
	svc := &fakeArchiveNodes{getN: models.ArchiveNode{
		ID: "AN-1", ArchiveID: "AR-1", Label: "Фонд 1",
		Parish: &models.TextRef{Text: "Никольский приход"},
	}}

	res := callArchiveNodeTool(t, archiveNodeUpdateHandler(svc), map[string]any{
		"id": "AN-1", "type": "fond", "archive_id": "AR-1", "label": "Фонд 1",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Parish == nil || svc.updated.Parish.Text != "Никольский приход" {
		t.Fatalf("updated.Parish = %+v, ожидалось сохранение текущей ссылки", svc.updated.Parish)
	}
}

// TestArchiveNodeUpdateToolNullParishClears — явный null для parish очищает поле.
func TestArchiveNodeUpdateToolNullParishClears(t *testing.T) {
	svc := &fakeArchiveNodes{getN: models.ArchiveNode{
		ID: "AN-1", ArchiveID: "AR-1", Label: "Фонд 1",
		Parish: &models.TextRef{Text: "Никольский приход"},
	}}

	res := callArchiveNodeTool(t, archiveNodeUpdateHandler(svc), map[string]any{
		"id": "AN-1", "type": "fond", "archive_id": "AR-1", "label": "Фонд 1", "parish": nil,
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Parish != nil {
		t.Fatalf("updated.Parish = %+v, ожидался nil (явный null очищает поле)", svc.updated.Parish)
	}
}

// TestArchiveNodeUpdateToolOmittedParentIDKeepsCurrent — parent_id
// отсутствует в вызове: текущий родитель сохраняется — отличие от явно
// пустой строки, которая делает узел корнем (см.
// TestArchiveNodeCreateToolParentFromOtherArchiveIsError для ошибки чужого
// архива и общего механизма).
func TestArchiveNodeUpdateToolOmittedParentIDKeepsCurrent(t *testing.T) {
	parent := models.ID("AN-0")
	svc := &fakeArchiveNodes{getN: models.ArchiveNode{
		ID: "AN-1", ArchiveID: "AR-1", Label: "Фонд 1", ParentID: &parent,
	}}

	res := callArchiveNodeTool(t, archiveNodeUpdateHandler(svc), map[string]any{
		"id": "AN-1", "type": "fond", "archive_id": "AR-1", "label": "Фонд 1",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.ParentID == nil || *svc.updated.ParentID != "AN-0" {
		t.Fatalf("updated.ParentID = %v, ожидалось сохранение текущего родителя", svc.updated.ParentID)
	}
}

// TestArchiveNodeUpdateToolEmptyParentIDMakesRoot — явно пустой parent_id
// делает узел корнем, в отличие от отсутствия ключа.
func TestArchiveNodeUpdateToolEmptyParentIDMakesRoot(t *testing.T) {
	parent := models.ID("AN-0")
	svc := &fakeArchiveNodes{getN: models.ArchiveNode{
		ID: "AN-1", ArchiveID: "AR-1", Label: "Фонд 1", ParentID: &parent,
	}}

	res := callArchiveNodeTool(t, archiveNodeUpdateHandler(svc), map[string]any{
		"id": "AN-1", "type": "fond", "archive_id": "AR-1", "label": "Фонд 1", "parent_id": "",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.ParentID != nil {
		t.Fatalf("updated.ParentID = %v, ожидался корень (nil)", svc.updated.ParentID)
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
