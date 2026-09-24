package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

func TestEventCreateContract(t *testing.T) {
	svc := &fakeEvents{created: models.Event{ID: "E-1", Type: models.EventTypeBirth}}

	rec := postD(t, NewHandler(Deps{Events: svc, DocsFS: fstest.MapFS{}}), "/api/events",
		`{"type":"birth"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Type != models.EventTypeBirth || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
	if !strings.Contains(rec.Body.String(), `"type":"birth"`) {
		t.Fatalf("body = %s", rec.Body)
	}
}

func TestEventCreateAnonymousIs401(t *testing.T) {
	svc := &fakeEvents{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Events: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/events", strings.NewReader(`{"type":"birth"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestEventUpdateMergesFields(t *testing.T) {
	svc := &fakeEvents{getE: models.Event{ID: "E-1", Type: models.EventTypeBirth}}

	rec := putD(t, NewHandler(Deps{Events: svc, DocsFS: fstest.MapFS{}}), "/api/events/E-1",
		`{"type":"death","private":true}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Type != models.EventTypeDeath || svc.updated.Private != true || svc.updated.ID != "E-1" {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestEventDeleteNoContent(t *testing.T) {
	svc := &fakeEvents{}

	rec := delD(t, NewHandler(Deps{Events: svc, DocsFS: fstest.MapFS{}}), "/api/events/E-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestEventDeleteInUseIs409(t *testing.T) {
	svc := &fakeEvents{deleteErr: &models.InUseError{Type: models.TypeEvent, ID: "E-1"}}

	rec := delD(t, NewHandler(Deps{Events: svc, DocsFS: fstest.MapFS{}}), "/api/events/E-1")

	requireStatus(t, rec, http.StatusConflict)
}
