package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

func TestArchiveDocumentCreateContract(t *testing.T) {
	svc := &fakeArchiveDocuments{created: models.ArchiveDocument{ID: "DC-1", UnitID: "AN-1", Title: "Метрическая книга"}}

	rec := postD(t, NewHandler(Deps{ArchiveDocs: svc, DocsFS: fstest.MapFS{}}), "/api/archive-documents",
		`{"unit_id":"AN-1","title":"Метрическая книга"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.UnitID != "AN-1" || svc.gotCreate.Title != "Метрическая книга" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestArchiveDocumentCreateUnitNotFoundIs422 — usecase-слой возвращает
// *models.ValidationError по полю unit_id при несуществующей единице учёта
// (см. create_archive_document), writeError мапит это на 422.
func TestArchiveDocumentCreateUnitNotFoundIs422(t *testing.T) {
	svc := &fakeArchiveDocuments{err: &models.ValidationError{Entity: models.TypeArchiveDocument, Field: "unit_id", Reason: "единица учёта не найдена"}}

	rec := postD(t, NewHandler(Deps{ArchiveDocs: svc, DocsFS: fstest.MapFS{}}), "/api/archive-documents",
		`{"unit_id":"AN-999","title":"Документ-призрак"}`)

	requireStatus(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"unit_id"`) {
		t.Fatalf("body = %s, ожидалось поле unit_id", rec.Body)
	}
}

func TestArchiveDocumentCreateAnonymousIs401(t *testing.T) {
	svc := &fakeArchiveDocuments{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{ArchiveDocs: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/archive-documents", strings.NewReader(`{"unit_id":"AN-1","title":"Документ"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestArchiveDocumentUpdateMergesFields(t *testing.T) {
	svc := &fakeArchiveDocuments{getD: models.ArchiveDocument{ID: "DC-1", UnitID: "AN-1", Title: "Метрическая книга"}}

	rec := putD(t, NewHandler(Deps{ArchiveDocs: svc, DocsFS: fstest.MapFS{}}), "/api/archive-documents/DC-1",
		`{"unit_id":"AN-1","title":"Метрическая книга (испр.)","private":true}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Title != "Метрическая книга (испр.)" || svc.updated.Private != true || svc.updated.ID != "DC-1" {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestArchiveDocumentDeleteNoContent(t *testing.T) {
	svc := &fakeArchiveDocuments{}

	rec := delD(t, NewHandler(Deps{ArchiveDocs: svc, DocsFS: fstest.MapFS{}}), "/api/archive-documents/DC-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestArchiveDocumentDeleteInUseIs409(t *testing.T) {
	svc := &fakeArchiveDocuments{deleteErr: &models.InUseError{Type: models.TypeArchiveDocument, ID: "DC-1"}}

	rec := delD(t, NewHandler(Deps{ArchiveDocs: svc, DocsFS: fstest.MapFS{}}), "/api/archive-documents/DC-1")

	requireStatus(t, rec, http.StatusConflict)
}
