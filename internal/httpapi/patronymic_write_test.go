package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

func TestPatronymicCreateContract(t *testing.T) {
	svc := &fakePatronymics{created: models.Patronymic{ID: "PN-1", Canonical: "Иванов"}}

	rec := postD(t, NewHandler(Deps{Patronymics: svc, DocsFS: fstest.MapFS{}}), "/api/patronymics", `{"canonical":"Иванов"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Canonical != "Иванов" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

func TestPatronymicCreateAnonymousIs401(t *testing.T) {
	svc := &fakePatronymics{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Patronymics: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/patronymics", strings.NewReader(`{"canonical":"Иванов"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestPatronymicUpdateMergesFields(t *testing.T) {
	svc := &fakePatronymics{getSN: models.Patronymic{ID: "PN-1", Canonical: "Иванов"}}

	rec := putD(t, NewHandler(Deps{Patronymics: svc, DocsFS: fstest.MapFS{}}), "/api/patronymics/PN-1",
		`{"canonical":"Иванов (испр.)","variants":[{"text":"Иванова"}]}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Canonical != "Иванов (испр.)" || svc.updated.ID != "PN-1" || len(svc.updated.Variants) != 1 {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestPatronymicDeleteNoContent(t *testing.T) {
	svc := &fakePatronymics{}

	rec := delD(t, NewHandler(Deps{Patronymics: svc, DocsFS: fstest.MapFS{}}), "/api/patronymics/PN-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestPatronymicDeleteInUseIs409(t *testing.T) {
	svc := &fakePatronymics{deleteErr: &models.InUseError{Type: models.TypePatronymic, ID: "PN-1"}}

	rec := delD(t, NewHandler(Deps{Patronymics: svc, DocsFS: fstest.MapFS{}}), "/api/patronymics/PN-1")

	requireStatus(t, rec, http.StatusConflict)
}
