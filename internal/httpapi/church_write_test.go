package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

func TestChurchCreateContract(t *testing.T) {
	svc := &fakeChurches{created: models.Church{ID: "CH-1", Name: "Никольская церковь"}}

	rec := postD(t, NewHandler(Deps{Churches: svc, DocsFS: fstest.MapFS{}}), "/api/churches",
		`{"name":"Никольская церковь","parish":{"text":"Никольский приход"}}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Name != "Никольская церковь" || svc.gotCreate.Parish == nil ||
		svc.gotCreate.Parish.Text != "Никольский приход" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

func TestChurchCreateAnonymousIs401(t *testing.T) {
	svc := &fakeChurches{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Churches: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/churches", strings.NewReader(`{"name":"Никольская церковь"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestChurchUpdateMergesFields(t *testing.T) {
	svc := &fakeChurches{getC: models.Church{ID: "CH-1", Name: "Никольская церковь"}}

	rec := putD(t, NewHandler(Deps{Churches: svc, DocsFS: fstest.MapFS{}}), "/api/churches/CH-1",
		`{"name":"Никольская церковь (испр.)","variants":["Николаевская церковь"],"settlements":[{"text":"Давыдово"}]}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Name != "Никольская церковь (испр.)" || svc.updated.ID != "CH-1" ||
		len(svc.updated.Variants) != 1 || svc.updated.Variants[0] != "Николаевская церковь" ||
		len(svc.updated.Settlements) != 1 {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestChurchDeleteNoContent(t *testing.T) {
	svc := &fakeChurches{}

	rec := delD(t, NewHandler(Deps{Churches: svc, DocsFS: fstest.MapFS{}}), "/api/churches/CH-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestChurchDeleteInUseIs409(t *testing.T) {
	svc := &fakeChurches{deleteErr: &models.InUseError{Type: models.TypeChurch, ID: "CH-1"}}

	rec := delD(t, NewHandler(Deps{Churches: svc, DocsFS: fstest.MapFS{}}), "/api/churches/CH-1")

	requireStatus(t, rec, http.StatusConflict)
}
