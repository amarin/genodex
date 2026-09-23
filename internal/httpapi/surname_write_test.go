package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

func TestSurnameCreateContract(t *testing.T) {
	svc := &fakeSurnames{created: models.Surname{ID: "SN-1", Canonical: "Иванов"}}

	rec := postD(t, NewHandler(Deps{Surnames: svc, DocsFS: fstest.MapFS{}}), "/api/surnames", `{"canonical":"Иванов"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Canonical != "Иванов" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

func TestSurnameCreateAnonymousIs401(t *testing.T) {
	svc := &fakeSurnames{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Surnames: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/surnames", strings.NewReader(`{"canonical":"Иванов"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestSurnameUpdateMergesFields(t *testing.T) {
	svc := &fakeSurnames{getSN: models.Surname{ID: "SN-1", Canonical: "Иванов"}}

	rec := putD(t, NewHandler(Deps{Surnames: svc, DocsFS: fstest.MapFS{}}), "/api/surnames/SN-1",
		`{"canonical":"Иванов (испр.)","variants":[{"text":"Иванова"}]}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Canonical != "Иванов (испр.)" || svc.updated.ID != "SN-1" || len(svc.updated.Variants) != 1 {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestSurnameDeleteNoContent(t *testing.T) {
	svc := &fakeSurnames{}

	rec := delD(t, NewHandler(Deps{Surnames: svc, DocsFS: fstest.MapFS{}}), "/api/surnames/SN-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestSurnameDeleteInUseIs409(t *testing.T) {
	svc := &fakeSurnames{deleteErr: &models.InUseError{Type: models.TypeSurname, ID: "SN-1"}}

	rec := delD(t, NewHandler(Deps{Surnames: svc, DocsFS: fstest.MapFS{}}), "/api/surnames/SN-1")

	requireStatus(t, rec, http.StatusConflict)
}
