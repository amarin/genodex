package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

func TestArchiveCreateContract(t *testing.T) {
	svc := &fakeArchives{created: models.Archive{ID: "AR-1", Name: "ГАВО, архив"}}

	rec := postD(t, NewHandler(Deps{Archives: svc, DocsFS: fstest.MapFS{}}), "/api/archives",
		`{"name":"ГАВО, архив","repository_id":"R-1","system":{"text":"фонд-опись-дело"}}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Name != "ГАВО, архив" || svc.gotCreate.RepositoryID != "R-1" ||
		svc.gotCreate.System == nil || svc.gotCreate.System.Text != "фонд-опись-дело" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestArchiveCreateRepositoryNotFoundIs422 — usecase-слой возвращает
// *models.ValidationError по полю repository_id при несуществующем
// хранилище (см. create_archive), writeError мапит это на 422, как и
// parent_id у делений.
func TestArchiveCreateRepositoryNotFoundIs422(t *testing.T) {
	svc := &fakeArchives{err: &models.ValidationError{Entity: models.TypeArchive, Field: "repository_id", Reason: "хранилище не найдено"}}

	rec := postD(t, NewHandler(Deps{Archives: svc, DocsFS: fstest.MapFS{}}), "/api/archives",
		`{"name":"ГАВО, архив","repository_id":"R-999"}`)

	requireStatus(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"repository_id"`) {
		t.Fatalf("body = %s, ожидалось поле repository_id", rec.Body)
	}
}

func TestArchiveCreateAnonymousIs401(t *testing.T) {
	svc := &fakeArchives{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Archives: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/archives", strings.NewReader(`{"name":"ГАВО, архив"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestArchiveUpdateMergesFields(t *testing.T) {
	svc := &fakeArchives{getA: models.Archive{ID: "AR-1", Name: "ГАВО, архив"}}

	rec := putD(t, NewHandler(Deps{Archives: svc, DocsFS: fstest.MapFS{}}), "/api/archives/AR-1",
		`{"name":"ГАВО, архив (испр.)","repository_id":"R-2","private":true}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Name != "ГАВО, архив (испр.)" || svc.updated.RepositoryID != "R-2" ||
		svc.updated.Private != true || svc.updated.ID != "AR-1" {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestArchiveDeleteNoContent(t *testing.T) {
	svc := &fakeArchives{}

	rec := delD(t, NewHandler(Deps{Archives: svc, DocsFS: fstest.MapFS{}}), "/api/archives/AR-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestArchiveDeleteInUseIs409(t *testing.T) {
	svc := &fakeArchives{deleteErr: &models.InUseError{Type: models.TypeArchive, ID: "AR-1"}}

	rec := delD(t, NewHandler(Deps{Archives: svc, DocsFS: fstest.MapFS{}}), "/api/archives/AR-1")

	requireStatus(t, rec, http.StatusConflict)
}
