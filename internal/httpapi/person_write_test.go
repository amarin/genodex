package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

func TestPersonCreateContract(t *testing.T) {
	svc := &fakePeople{created: models.Person{ID: "I-1", Gender: models.PersonGenderFemale}}

	rec := postD(t, NewHandler(Deps{People: svc, DocsFS: fstest.MapFS{}}), "/api/people",
		`{"gender":"female"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Gender != models.PersonGenderFemale || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
	if !strings.Contains(rec.Body.String(), `"gender":"female"`) {
		t.Fatalf("body = %s, gender не в ответе", rec.Body)
	}
}

func TestPersonCreateAnonymousIs401(t *testing.T) {
	svc := &fakePeople{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{People: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/people", strings.NewReader(`{}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestPersonUpdateMergesFields(t *testing.T) {
	svc := &fakePeople{getP: models.Person{ID: "I-1", Gender: models.PersonGenderFemale}}

	rec := putD(t, NewHandler(Deps{People: svc, DocsFS: fstest.MapFS{}}), "/api/people/I-1",
		`{"gender":"male","private":true}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Gender != models.PersonGenderMale || svc.updated.Private != true || svc.updated.ID != "I-1" {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestPersonDeleteNoContent(t *testing.T) {
	svc := &fakePeople{}

	rec := delD(t, NewHandler(Deps{People: svc, DocsFS: fstest.MapFS{}}), "/api/people/I-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestPersonDeleteInUseIs409(t *testing.T) {
	svc := &fakePeople{deleteErr: &models.InUseError{Type: models.TypePerson, ID: "I-1"}}

	rec := delD(t, NewHandler(Deps{People: svc, DocsFS: fstest.MapFS{}}), "/api/people/I-1")

	requireStatus(t, rec, http.StatusConflict)
}
