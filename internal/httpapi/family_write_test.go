package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

func TestFamilyCreateContract(t *testing.T) {
	svc := &fakeFamilies{created: models.Family{ID: "F-1", Name: "Ивановы"}}

	rec := postD(t, NewHandler(Deps{Families: svc, DocsFS: fstest.MapFS{}}), "/api/families",
		`{"name":"Ивановы"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Name != "Ивановы" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
	if !strings.Contains(rec.Body.String(), `"name":"Ивановы"`) {
		t.Fatalf("body = %s, name не в ответе", rec.Body)
	}
}

func TestFamilyCreateAnonymousIs401(t *testing.T) {
	svc := &fakeFamilies{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Families: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/families", strings.NewReader(`{"name":"Ивановы"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestFamilyUpdateMergesFields(t *testing.T) {
	svc := &fakeFamilies{getF: models.Family{ID: "F-1", Name: "Ивановы"}}

	rec := putD(t, NewHandler(Deps{Families: svc, DocsFS: fstest.MapFS{}}), "/api/families/F-1",
		`{"name":"Ивановы (испр.)","private":true}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Name != "Ивановы (испр.)" || svc.updated.Private != true || svc.updated.ID != "F-1" {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestFamilyDeleteNoContent(t *testing.T) {
	svc := &fakeFamilies{}

	rec := delD(t, NewHandler(Deps{Families: svc, DocsFS: fstest.MapFS{}}), "/api/families/F-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestFamilyDeleteInUseIs409(t *testing.T) {
	svc := &fakeFamilies{deleteErr: &models.InUseError{Type: models.TypeFamily, ID: "F-1"}}

	rec := delD(t, NewHandler(Deps{Families: svc, DocsFS: fstest.MapFS{}}), "/api/families/F-1")

	requireStatus(t, rec, http.StatusConflict)
}
