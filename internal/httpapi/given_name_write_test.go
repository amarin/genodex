package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

func TestGivenNameCreateContract(t *testing.T) {
	svc := &fakeGivenNames{created: models.GivenName{ID: "GN-1", Canonical: "Иван", Gender: models.NameGenderMale}}

	rec := postD(t, NewHandler(Deps{GivenNames: svc, DocsFS: fstest.MapFS{}}), "/api/given-names", `{"canonical":"Иван","gender":"male"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Canonical != "Иван" || svc.gotCreate.Gender != models.NameGenderMale || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
	if !strings.Contains(rec.Body.String(), `"gender":"male"`) {
		t.Fatalf("body = %s, gender не в ответе", rec.Body)
	}
}

func TestGivenNameCreateAnonymousIs401(t *testing.T) {
	svc := &fakeGivenNames{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{GivenNames: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/given-names", strings.NewReader(`{"canonical":"Иванов"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestGivenNameUpdateMergesFields(t *testing.T) {
	svc := &fakeGivenNames{getSN: models.GivenName{ID: "GN-1", Canonical: "Иван", Gender: models.NameGenderMale}}

	rec := putD(t, NewHandler(Deps{GivenNames: svc, DocsFS: fstest.MapFS{}}), "/api/given-names/GN-1",
		`{"canonical":"Иван (испр.)","gender":"neutral","variants":[{"text":"Ваня"}]}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Canonical != "Иван (испр.)" || svc.updated.Gender != models.NameGenderNeutral ||
		svc.updated.ID != "GN-1" || len(svc.updated.Variants) != 1 {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestGivenNameDeleteNoContent(t *testing.T) {
	svc := &fakeGivenNames{}

	rec := delD(t, NewHandler(Deps{GivenNames: svc, DocsFS: fstest.MapFS{}}), "/api/given-names/GN-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestGivenNameDeleteInUseIs409(t *testing.T) {
	svc := &fakeGivenNames{deleteErr: &models.InUseError{Type: models.TypeGivenName, ID: "GN-1"}}

	rec := delD(t, NewHandler(Deps{GivenNames: svc, DocsFS: fstest.MapFS{}}), "/api/given-names/GN-1")

	requireStatus(t, rec, http.StatusConflict)
}
