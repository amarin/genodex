package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

func TestArchiveNodeCreateContract(t *testing.T) {
	svc := &fakeArchiveNodes{created: models.ArchiveNode{ID: "AN-1", ArchiveID: "AR-1", Label: "Фонд 1"}}

	rec := postD(t, NewHandler(Deps{ArchiveNodes: svc, DocsFS: fstest.MapFS{}}), "/api/archive-nodes",
		`{"type":"fond","archive_id":"AR-1","label":"Фонд 1"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.ArchiveID != "AR-1" || svc.gotCreate.Label != "Фонд 1" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestArchiveNodeCreateParentFromOtherArchiveIs422 — usecase-слой возвращает
// *models.ValidationError по полю parent_id при родителе из другого архива
// (см. create_archive_node), writeError мапит это на 422.
func TestArchiveNodeCreateParentFromOtherArchiveIs422(t *testing.T) {
	svc := &fakeArchiveNodes{err: &models.ValidationError{Entity: models.TypeArchiveNode, Field: "parent_id", Reason: "родитель принадлежит другому архиву"}}

	rec := postD(t, NewHandler(Deps{ArchiveNodes: svc, DocsFS: fstest.MapFS{}}), "/api/archive-nodes",
		`{"type":"fond","archive_id":"AR-1","parent_id":"AN-999","label":"Фонд-призрак"}`)

	requireStatus(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"parent_id"`) {
		t.Fatalf("body = %s, ожидалось поле parent_id", rec.Body)
	}
}

func TestArchiveNodeCreateAnonymousIs401(t *testing.T) {
	svc := &fakeArchiveNodes{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{ArchiveNodes: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/archive-nodes", strings.NewReader(`{"type":"fond","archive_id":"AR-1","label":"Фонд"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestArchiveNodeUpdateMergesFields(t *testing.T) {
	svc := &fakeArchiveNodes{getN: models.ArchiveNode{ID: "AN-1", ArchiveID: "AR-1", Label: "Фонд 1"}}

	rec := putD(t, NewHandler(Deps{ArchiveNodes: svc, DocsFS: fstest.MapFS{}}), "/api/archive-nodes/AN-1",
		`{"type":"fond","archive_id":"AR-1","label":"Фонд 1 (испр.)","private":true}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Label != "Фонд 1 (испр.)" || svc.updated.Private != true || svc.updated.ID != "AN-1" {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestArchiveNodeDeleteNoContent(t *testing.T) {
	svc := &fakeArchiveNodes{}

	rec := delD(t, NewHandler(Deps{ArchiveNodes: svc, DocsFS: fstest.MapFS{}}), "/api/archive-nodes/AN-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestArchiveNodeDeleteInUseIs409(t *testing.T) {
	svc := &fakeArchiveNodes{deleteErr: &models.InUseError{Type: models.TypeArchiveNode, ID: "AN-1"}}

	rec := delD(t, NewHandler(Deps{ArchiveNodes: svc, DocsFS: fstest.MapFS{}}), "/api/archive-nodes/AN-1")

	requireStatus(t, rec, http.StatusConflict)
}
