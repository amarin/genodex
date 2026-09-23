package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

func TestTitleCreateContract(t *testing.T) {
	svc := &fakeTitles{created: models.Title{ID: "TT-1", Canonical: "Иванов"}}

	rec := postD(t, NewHandler(Deps{Titles: svc, DocsFS: fstest.MapFS{}}), "/api/titles", `{"canonical":"Иванов"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Canonical != "Иванов" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

func TestTitleCreateAnonymousIs401(t *testing.T) {
	svc := &fakeTitles{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Titles: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/titles", strings.NewReader(`{"canonical":"Иванов"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestTitleUpdateMergesFields(t *testing.T) {
	svc := &fakeTitles{getSN: models.Title{ID: "TT-1", Canonical: "Иванов"}}

	rec := putD(t, NewHandler(Deps{Titles: svc, DocsFS: fstest.MapFS{}}), "/api/titles/TT-1",
		`{"canonical":"Иванов (испр.)","variants":[{"text":"Иванова"}]}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Canonical != "Иванов (испр.)" || svc.updated.ID != "TT-1" || len(svc.updated.Variants) != 1 {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestTitleDeleteNoContent(t *testing.T) {
	svc := &fakeTitles{}

	rec := delD(t, NewHandler(Deps{Titles: svc, DocsFS: fstest.MapFS{}}), "/api/titles/TT-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestTitleDeleteInUseIs409(t *testing.T) {
	svc := &fakeTitles{deleteErr: &models.InUseError{Type: models.TypeTitle, ID: "TT-1"}}

	rec := delD(t, NewHandler(Deps{Titles: svc, DocsFS: fstest.MapFS{}}), "/api/titles/TT-1")

	requireStatus(t, rec, http.StatusConflict)
}
