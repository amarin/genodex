package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

func TestParishCreateContract(t *testing.T) {
	svc := &fakeParishes{created: models.Parish{ID: "PR-1", Name: "Никольский приход"}}

	rec := postD(t, NewHandler(Deps{Parishes: svc, DocsFS: fstest.MapFS{}}), "/api/parishes",
		`{"name":"Никольский приход","since":{"year":1880,"precision":"year","modifier":"exact"}}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Name != "Никольский приход" || svc.gotCreate.Since == nil ||
		svc.gotCreate.Since.Year != 1880 || svc.gotCreate.Since.Precision != models.PrecisionYear ||
		svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

func TestParishCreateAnonymousIs401(t *testing.T) {
	svc := &fakeParishes{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Parishes: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/parishes", strings.NewReader(`{"name":"Никольский приход"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

// TestParishUpdateMergesFields — включая since/until (FactDate), единственное
// новое поле, которого не было у Surname/Patronymic/Estate/Title/GivenName.
func TestParishUpdateMergesFields(t *testing.T) {
	svc := &fakeParishes{getP: models.Parish{ID: "PR-1", Name: "Никольский приход"}}

	rec := putD(t, NewHandler(Deps{Parishes: svc, DocsFS: fstest.MapFS{}}), "/api/parishes/PR-1",
		`{"name":"Никольский приход (испр.)",`+
			`"since":{"year":1880,"precision":"year","modifier":"exact"},`+
			`"until":{"year":1917,"precision":"year","modifier":"exact"}}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Name != "Никольский приход (испр.)" || svc.updated.ID != "PR-1" ||
		svc.updated.Since == nil || svc.updated.Since.Year != 1880 ||
		svc.updated.Until == nil || svc.updated.Until.Year != 1917 {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestParishDeleteNoContent(t *testing.T) {
	svc := &fakeParishes{}

	rec := delD(t, NewHandler(Deps{Parishes: svc, DocsFS: fstest.MapFS{}}), "/api/parishes/PR-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestParishDeleteInUseIs409(t *testing.T) {
	svc := &fakeParishes{deleteErr: &models.InUseError{Type: models.TypeParish, ID: "PR-1"}}

	rec := delD(t, NewHandler(Deps{Parishes: svc, DocsFS: fstest.MapFS{}}), "/api/parishes/PR-1")

	requireStatus(t, rec, http.StatusConflict)
}
