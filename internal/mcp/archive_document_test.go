package mcp

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeArchiveDocuments struct {
	list []models.ArchiveDocument
	err  error

	getD      models.ArchiveDocument
	created   models.ArchiveDocument
	gotCreate models.ArchiveDocument
	updated   models.ArchiveDocument
	gotIDs    []models.ID
	deleteErr error

	search []models.ArchiveDocument
}

func (f *fakeArchiveDocuments) ListArchiveDocuments(context.Context, models.Access, models.Page) ([]models.ArchiveDocument, error) {
	return f.list, f.err
}

func (f *fakeArchiveDocuments) SearchArchiveDocuments(context.Context, models.Access, models.SearchQuery) ([]models.ArchiveDocument, error) {
	return f.search, f.err
}

func (f *fakeArchiveDocuments) GetArchiveDocument(_ context.Context, _ models.Access, id models.ID) (models.ArchiveDocument, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.ArchiveDocument{}, f.err
	}

	return f.getD, nil
}

func (f *fakeArchiveDocuments) CreateArchiveDocument(_ context.Context, d models.ArchiveDocument) (models.ArchiveDocument, error) {
	f.gotCreate = d
	if f.err != nil {
		return models.ArchiveDocument{}, f.err
	}

	return f.created, nil
}

func (f *fakeArchiveDocuments) UpdateArchiveDocument(_ context.Context, d models.ArchiveDocument) error {
	f.updated = d

	return f.err
}

func (f *fakeArchiveDocuments) DeleteArchiveDocument(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestArchiveDocumentGetToolContract(t *testing.T) {
	svc := &fakeArchiveDocuments{getD: models.ArchiveDocument{ID: "DC-1", Title: "Метрическая книга"}}

	res := callArchiveNodeTool(t, archiveDocumentGetHandler(svc), map[string]any{"id": "DC-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "DC-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

func TestArchiveDocumentCreateToolPassesFields(t *testing.T) {
	svc := &fakeArchiveDocuments{created: models.ArchiveDocument{ID: "DC-new", Title: "Метрическая книга"}}

	res := callArchiveNodeTool(t, archiveDocumentCreateHandler(svc), map[string]any{
		"unit_id": "AN-1",
		"title":   "Метрическая книга",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.UnitID != "AN-1" || svc.gotCreate.Title != "Метрическая книга" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestArchiveDocumentCreateToolUnitNotFoundIsError — usecase-слой возвращает
// ошибку на несуществующий unit_id, тул отдаёт её как ошибку тула.
func TestArchiveDocumentCreateToolUnitNotFoundIsError(t *testing.T) {
	svc := &fakeArchiveDocuments{err: &models.ValidationError{Entity: models.TypeArchiveDocument, Field: "unit_id", Reason: "не найдена"}}

	res := callArchiveNodeTool(t, archiveDocumentCreateHandler(svc), map[string]any{
		"unit_id": "AN-999",
		"title":   "Документ-призрак",
	})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

// TestArchiveDocumentUpdateToolPreservesSourcesWhenOmitted — sources
// отсутствует в вызове: текущие источники сохраняются как есть.
func TestArchiveDocumentUpdateToolPreservesSourcesWhenOmitted(t *testing.T) {
	existingSources := []models.SourceLink{{CitationID: "C-1"}}
	svc := &fakeArchiveDocuments{getD: models.ArchiveDocument{ID: "DC-1", UnitID: "AN-1", Title: "Метрическая книга", Sources: existingSources}}

	res := callArchiveNodeTool(t, archiveDocumentUpdateHandler(svc), map[string]any{
		"id":      "DC-1",
		"unit_id": "AN-1",
		"title":   "Метрическая книга (испр.)",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if len(svc.updated.Sources) != 1 || svc.updated.Sources[0].CitationID != "C-1" {
		t.Fatalf("updated.Sources = %+v, ожидались сохранённые источники", svc.updated.Sources)
	}
}

// TestArchiveDocumentUpdateToolClearsSourcesWhenEmptyArray — пустой массив
// sources в вызове очищает источники.
func TestArchiveDocumentUpdateToolClearsSourcesWhenEmptyArray(t *testing.T) {
	existingSources := []models.SourceLink{{CitationID: "C-1"}}
	svc := &fakeArchiveDocuments{getD: models.ArchiveDocument{ID: "DC-1", UnitID: "AN-1", Title: "Метрическая книга", Sources: existingSources}}

	res := callArchiveNodeTool(t, archiveDocumentUpdateHandler(svc), map[string]any{
		"id":      "DC-1",
		"unit_id": "AN-1",
		"title":   "Метрическая книга (испр.)",
		"sources": []any{},
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if len(svc.updated.Sources) != 0 {
		t.Fatalf("updated.Sources = %+v, ожидался пустой список", svc.updated.Sources)
	}
}

func TestArchiveDocumentDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeArchiveDocuments{deleteErr: &models.InUseError{Type: models.TypeArchiveDocument, ID: "DC-1"}}

	res := callArchiveNodeTool(t, archiveDocumentDeleteHandler(svc), map[string]any{"id": "DC-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersArchiveDocumentTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, ArchiveDocs: &fakeArchiveDocuments{}}).ListTools()

	for _, name := range []string{
		"archive_document_list", "archive_document_search", "archive_document_get",
		"archive_document_create", "archive_document_update", "archive_document_delete",
	} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
