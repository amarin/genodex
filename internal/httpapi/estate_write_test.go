package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

func TestEstateCreateContract(t *testing.T) {
	svc := &fakeEstates{created: models.Estate{ID: "ES-1", Canonical: "Иванов"}}

	rec := postD(t, NewHandler(Deps{Estates: svc, DocsFS: fstest.MapFS{}}), "/api/estates", `{"canonical":"Иванов"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Canonical != "Иванов" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

func TestEstateCreateAnonymousIs401(t *testing.T) {
	svc := &fakeEstates{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Estates: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/estates", strings.NewReader(`{"canonical":"Иванов"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestEstateUpdateMergesFields(t *testing.T) {
	svc := &fakeEstates{getSN: models.Estate{ID: "ES-1", Canonical: "Иванов"}}

	rec := putD(t, NewHandler(Deps{Estates: svc, DocsFS: fstest.MapFS{}}), "/api/estates/ES-1",
		`{"canonical":"Иванов (испр.)","variants":[{"text":"Иванова"}]}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Canonical != "Иванов (испр.)" || svc.updated.ID != "ES-1" || len(svc.updated.Variants) != 1 {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestEstateDeleteNoContent(t *testing.T) {
	svc := &fakeEstates{}

	rec := delD(t, NewHandler(Deps{Estates: svc, DocsFS: fstest.MapFS{}}), "/api/estates/ES-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestEstateDeleteInUseIs409(t *testing.T) {
	svc := &fakeEstates{deleteErr: &models.InUseError{Type: models.TypeEstate, ID: "ES-1"}}

	rec := delD(t, NewHandler(Deps{Estates: svc, DocsFS: fstest.MapFS{}}), "/api/estates/ES-1")

	requireStatus(t, rec, http.StatusConflict)
}
