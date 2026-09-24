package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

func TestRelationCreateContract(t *testing.T) {
	svc := &fakeRelations{created: models.Relation{ID: "RL-1", Kind: models.RelationKindBlood, PersonA: "I-1", PersonB: "I-2"}}

	rec := postD(t, NewHandler(Deps{Relations: svc, DocsFS: fstest.MapFS{}}), "/api/relations",
		`{"kind":"blood","person_a":"I-1","person_b":"I-2"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.PersonA != "I-1" || svc.gotCreate.PersonB != "I-2" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
	if !strings.Contains(rec.Body.String(), `"person_a":"I-1"`) {
		t.Fatalf("body = %s", rec.Body)
	}
}

func TestRelationCreateAnonymousIs401(t *testing.T) {
	svc := &fakeRelations{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Relations: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/relations", strings.NewReader(`{"kind":"blood","person_a":"I-1","person_b":"I-2"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestRelationUpdateMergesFields(t *testing.T) {
	svc := &fakeRelations{getR: models.Relation{ID: "RL-1", Kind: models.RelationKindBlood, PersonA: "I-1", PersonB: "I-2"}}

	rec := putD(t, NewHandler(Deps{Relations: svc, DocsFS: fstest.MapFS{}}), "/api/relations/RL-1",
		`{"kind":"marriage","person_a":"I-1","person_b":"I-2","private":true}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Kind != models.RelationKindMarriage || svc.updated.Private != true || svc.updated.ID != "RL-1" {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestRelationDeleteNoContent(t *testing.T) {
	svc := &fakeRelations{}

	rec := delD(t, NewHandler(Deps{Relations: svc, DocsFS: fstest.MapFS{}}), "/api/relations/RL-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestRelationDeleteInUseIs409(t *testing.T) {
	svc := &fakeRelations{deleteErr: &models.InUseError{Type: models.TypeRelation, ID: "RL-1"}}

	rec := delD(t, NewHandler(Deps{Relations: svc, DocsFS: fstest.MapFS{}}), "/api/relations/RL-1")

	requireStatus(t, rec, http.StatusConflict)
}
