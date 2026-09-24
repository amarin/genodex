package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

func TestResidenceCreateContract(t *testing.T) {
	svc := &fakeResidences{created: models.Residence{ID: "RS-1", PersonID: "I-1", PlaceID: "AD-1"}}

	rec := postD(t, NewHandler(Deps{Residences: svc, DocsFS: fstest.MapFS{}}), "/api/residences",
		`{"person_id":"I-1","place_id":"AD-1"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.PersonID != "I-1" || svc.gotCreate.PlaceID != "AD-1" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
	if !strings.Contains(rec.Body.String(), `"place_id":"AD-1"`) {
		t.Fatalf("body = %s", rec.Body)
	}
}

func TestResidenceCreateAnonymousIs401(t *testing.T) {
	svc := &fakeResidences{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Residences: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/residences", strings.NewReader(`{"person_id":"I-1","place_id":"AD-1"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestResidenceUpdateMergesFields(t *testing.T) {
	svc := &fakeResidences{getR: models.Residence{ID: "RS-1", PersonID: "I-1", PlaceID: "AD-1"}}

	rec := putD(t, NewHandler(Deps{Residences: svc, DocsFS: fstest.MapFS{}}), "/api/residences/RS-1",
		`{"person_id":"I-1","place_id":"AD-1","note":"изба","private":true}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Note != "изба" || svc.updated.Private != true || svc.updated.ID != "RS-1" {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestResidenceDeleteNoContent(t *testing.T) {
	svc := &fakeResidences{}

	rec := delD(t, NewHandler(Deps{Residences: svc, DocsFS: fstest.MapFS{}}), "/api/residences/RS-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestResidenceDeleteInUseIs409(t *testing.T) {
	svc := &fakeResidences{deleteErr: &models.InUseError{Type: models.TypeResidence, ID: "RS-1"}}

	rec := delD(t, NewHandler(Deps{Residences: svc, DocsFS: fstest.MapFS{}}), "/api/residences/RS-1")

	requireStatus(t, rec, http.StatusConflict)
}
