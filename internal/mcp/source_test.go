package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakeSources struct {
	list []models.Source
	err  error

	getS      models.Source
	created   models.Source
	gotCreate models.Source
	updated   models.Source
	gotIDs    []models.ID
	deleteErr error

	search []models.Source
}

func (f *fakeSources) ListSources(context.Context, models.Access, models.Page) ([]models.Source, error) {
	return f.list, f.err
}

func (f *fakeSources) SearchSources(context.Context, models.Access, models.SearchQuery) ([]models.Source, error) {
	return f.search, f.err
}

func (f *fakeSources) GetSource(_ context.Context, _ models.Access, id models.ID) (models.Source, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Source{}, f.err
	}

	return f.getS, nil
}

func (f *fakeSources) CreateSource(_ context.Context, s models.Source) (models.Source, error) {
	f.gotCreate = s
	if f.err != nil {
		return models.Source{}, f.err
	}

	return f.created, nil
}

func (f *fakeSources) UpdateSource(_ context.Context, s models.Source) error {
	f.updated = s

	return f.err
}

func (f *fakeSources) DeleteSource(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callSourceTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestSourceGetToolContract(t *testing.T) {
	svc := &fakeSources{getS: models.Source{ID: "S-1", Title: "Ревизская сказка"}}

	res := callSourceTool(t, sourceGetHandler(svc), map[string]any{"id": "S-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "S-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

// TestSourceCreateToolPassesRepositoryID — repository_id передаётся плоской
// строкой (не объектом), по образцу TestArchiveCreateToolPassesRepositoryID.
func TestSourceCreateToolPassesRepositoryID(t *testing.T) {
	svc := &fakeSources{created: models.Source{ID: "S-new", Title: "Ревизская сказка"}}

	res := callSourceTool(t, sourceCreateHandler(svc), map[string]any{
		"kind":          "document",
		"title":         "Ревизская сказка",
		"reliability":   "primary",
		"repository_id": "R-1",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.Title != "Ревизская сказка" || svc.gotCreate.RepositoryID != "R-1" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestSourceCreateToolRepositoryNotFoundIsError — usecase-слой возвращает
// ошибку на несуществующее хранилище, тул должен отдать её как ошибку тула
// (не паниковать, не проглатывать).
func TestSourceCreateToolRepositoryNotFoundIsError(t *testing.T) {
	svc := &fakeSources{err: &models.ValidationError{Entity: models.TypeSource, Field: "repository_id", Reason: "не найдено"}}

	res := callSourceTool(t, sourceCreateHandler(svc), map[string]any{
		"kind":          "document",
		"title":         "Ревизская сказка",
		"reliability":   "primary",
		"repository_id": "R-999",
	})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

// TestSourceUpdateToolOmittedDateKeepsCurrent — date отсутствует в вызове:
// текущее значение сохраняется.
func TestSourceUpdateToolOmittedDateKeepsCurrent(t *testing.T) {
	svc := &fakeSources{getS: models.Source{
		ID: "S-1", Kind: models.SourceKindDocument, Title: "Ревизская сказка", Reliability: models.ReliabilityPrimary,
		Date: &models.FactDate{Year: 1858, Precision: models.PrecisionYear},
	}}

	res := callSourceTool(t, sourceUpdateHandler(svc), map[string]any{
		"id": "S-1", "kind": "document", "title": "Ревизская сказка", "reliability": "primary",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Date == nil || svc.updated.Date.Year != 1858 {
		t.Fatalf("updated.Date = %+v, ожидалось сохранение текущего значения", svc.updated.Date)
	}
}

// TestSourceUpdateToolNullDateClears — явный null для date очищает поле
// (регрессия финального ревью: raw != nil ошибочно приравнивал явный null
// к отсутствию ключа для *FactDate-полей).
func TestSourceUpdateToolNullDateClears(t *testing.T) {
	svc := &fakeSources{getS: models.Source{
		ID: "S-1", Kind: models.SourceKindDocument, Title: "Ревизская сказка", Reliability: models.ReliabilityPrimary,
		Date: &models.FactDate{Year: 1858, Precision: models.PrecisionYear},
	}}

	res := callSourceTool(t, sourceUpdateHandler(svc), map[string]any{
		"id": "S-1", "kind": "document", "title": "Ревизская сказка", "reliability": "primary", "date": nil,
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Date != nil {
		t.Fatalf("updated.Date = %+v, ожидался nil (явный null очищает поле)", svc.updated.Date)
	}
}

func TestSourceDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeSources{deleteErr: &models.InUseError{Type: models.TypeSource, ID: "S-1"}}

	res := callSourceTool(t, sourceDeleteHandler(svc), map[string]any{"id": "S-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersSourceTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Sources: &fakeSources{}}).ListTools()

	for _, name := range []string{"source_list", "source_search", "source_get", "source_create", "source_update", "source_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
