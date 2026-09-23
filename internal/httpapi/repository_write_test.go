package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

func TestRepositoryCreateContract(t *testing.T) {
	svc := &fakeRepositories{created: models.Repository{ID: "R-1", Name: "ГАВО", Type: models.RepositoryTypeArchive}}

	rec := postD(t, NewHandler(Deps{Repositories: svc, DocsFS: fstest.MapFS{}}), "/api/repositories",
		`{"name":"ГАВО","type":"archive"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Name != "ГАВО" || svc.gotCreate.Type != models.RepositoryTypeArchive || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
	if !strings.Contains(rec.Body.String(), `"type":"archive"`) {
		t.Fatalf("body = %s, type не в ответе", rec.Body)
	}
}

func TestRepositoryCreateAnonymousIs401(t *testing.T) {
	svc := &fakeRepositories{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Repositories: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/repositories", strings.NewReader(`{"name":"ГАВО","type":"archive"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestRepositoryUpdateMergesFields(t *testing.T) {
	svc := &fakeRepositories{getR: models.Repository{ID: "R-1", Name: "ГАВО", Type: models.RepositoryTypeArchive}}

	rec := putD(t, NewHandler(Deps{Repositories: svc, DocsFS: fstest.MapFS{}}), "/api/repositories/R-1",
		`{"name":"ГАВО (испр.)","type":"library","address":"г. Владимир","private":true}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Name != "ГАВО (испр.)" || svc.updated.Type != models.RepositoryTypeLibrary ||
		svc.updated.Address != "г. Владимир" || svc.updated.Private != true || svc.updated.ID != "R-1" {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestRepositoryDeleteNoContent(t *testing.T) {
	svc := &fakeRepositories{}

	rec := delD(t, NewHandler(Deps{Repositories: svc, DocsFS: fstest.MapFS{}}), "/api/repositories/R-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestRepositoryDeleteInUseIs409(t *testing.T) {
	svc := &fakeRepositories{deleteErr: &models.InUseError{Type: models.TypeRepository, ID: "R-1"}}

	rec := delD(t, NewHandler(Deps{Repositories: svc, DocsFS: fstest.MapFS{}}), "/api/repositories/R-1")

	requireStatus(t, rec, http.StatusConflict)
}
