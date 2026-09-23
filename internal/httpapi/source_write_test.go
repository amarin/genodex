package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

func TestSourceCreateContract(t *testing.T) {
	svc := &fakeSources{created: models.Source{ID: "S-1", Title: "Ревизская сказка"}}

	rec := postD(t, NewHandler(Deps{Sources: svc, DocsFS: fstest.MapFS{}}), "/api/sources",
		`{"kind":"document","title":"Ревизская сказка","reliability":"primary","repository_id":"R-1",`+
			`"date":{"year":1858,"precision":"year","modifier":"exact"}}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Kind != models.SourceKindDocument || svc.gotCreate.Title != "Ревизская сказка" ||
		svc.gotCreate.Reliability != models.ReliabilityPrimary || svc.gotCreate.RepositoryID != "R-1" ||
		svc.gotCreate.Date == nil || svc.gotCreate.Date.Year != 1858 || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestSourceCreateRepositoryNotFoundIs422 — usecase-слой возвращает
// *models.ValidationError по полю repository_id при несуществующем
// хранилище (см. create_source), writeError мапит это на 422, по образцу
// create_archive.
func TestSourceCreateRepositoryNotFoundIs422(t *testing.T) {
	svc := &fakeSources{err: &models.ValidationError{Entity: models.TypeSource, Field: "repository_id", Reason: "хранилище не найдено"}}

	rec := postD(t, NewHandler(Deps{Sources: svc, DocsFS: fstest.MapFS{}}), "/api/sources",
		`{"kind":"document","title":"Ревизская сказка","reliability":"primary","repository_id":"R-999"}`)

	requireStatus(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"repository_id"`) {
		t.Fatalf("body = %s, ожидалось поле repository_id", rec.Body)
	}
}

func TestSourceCreateAnonymousIs401(t *testing.T) {
	svc := &fakeSources{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Sources: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/sources", strings.NewReader(`{"kind":"document","title":"Ревизская сказка","reliability":"primary"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

// TestSourceUpdateMergesFields — проверяет все поля, включая структурированную
// date и reliability, по образцу TestArchiveUpdateMergesFields/TestParishUpdateMergesFields.
func TestSourceUpdateMergesFields(t *testing.T) {
	svc := &fakeSources{getS: models.Source{ID: "S-1", Title: "Ревизская сказка"}}

	rec := putD(t, NewHandler(Deps{Sources: svc, DocsFS: fstest.MapFS{}}), "/api/sources/S-1",
		`{"kind":"transcription","title":"Ревизская сказка (испр.)","author":"Иванов",`+
			`"date":{"year":1858,"precision":"year","modifier":"exact"},`+
			`"reliability":"contemporary","repository_id":"R-2","private":true}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Kind != models.SourceKindTranscription || svc.updated.Title != "Ревизская сказка (испр.)" ||
		svc.updated.Author != "Иванов" || svc.updated.Date == nil || svc.updated.Date.Year != 1858 ||
		svc.updated.Reliability != models.ReliabilityContemporary || svc.updated.RepositoryID != "R-2" ||
		svc.updated.Private != true || svc.updated.ID != "S-1" {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestSourceDeleteNoContent(t *testing.T) {
	svc := &fakeSources{}

	rec := delD(t, NewHandler(Deps{Sources: svc, DocsFS: fstest.MapFS{}}), "/api/sources/S-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestSourceDeleteInUseIs409(t *testing.T) {
	svc := &fakeSources{deleteErr: &models.InUseError{Type: models.TypeSource, ID: "S-1"}}

	rec := delD(t, NewHandler(Deps{Sources: svc, DocsFS: fstest.MapFS{}}), "/api/sources/S-1")

	requireStatus(t, rec, http.StatusConflict)
}
