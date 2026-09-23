package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeArchiveNodes struct {
	list []models.ArchiveNode
	err  error

	gotList models.ArchiveNodeQuery

	getN      models.ArchiveNode
	gotIDs    []models.ID
	created   models.ArchiveNode
	gotCreate models.ArchiveNode
	updated   models.ArchiveNode
	deleteErr error

	search    []models.ArchiveNode
	gotSearch models.SearchQuery
}

func (f *fakeArchiveNodes) ListArchiveNodes(_ context.Context, _ models.Access, q models.ArchiveNodeQuery) ([]models.ArchiveNode, error) {
	f.gotList = q

	return f.list, f.err
}

func (f *fakeArchiveNodes) SearchArchiveNodes(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.ArchiveNode, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeArchiveNodes) GetArchiveNode(_ context.Context, _ models.Access, id models.ID) (models.ArchiveNode, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.ArchiveNode{}, f.err
	}

	return f.getN, nil
}

func (f *fakeArchiveNodes) CreateArchiveNode(_ context.Context, n models.ArchiveNode) (models.ArchiveNode, error) {
	f.gotCreate = n
	if f.err != nil {
		return models.ArchiveNode{}, f.err
	}

	return f.created, nil
}

func (f *fakeArchiveNodes) UpdateArchiveNode(_ context.Context, n models.ArchiveNode) error {
	f.updated = n

	return f.err
}

func (f *fakeArchiveNodes) DeleteArchiveNode(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestArchiveNodeListRequiresArchiveID(t *testing.T) {
	svc := &fakeArchiveNodes{}

	rec := get(t, NewHandler(Deps{ArchiveNodes: svc, DocsFS: fstest.MapFS{}}), "/api/archive-nodes")
	requireStatus(t, rec, http.StatusBadRequest)
}

func TestArchiveNodeListPassesArchiveIDAndParentID(t *testing.T) {
	svc := &fakeArchiveNodes{list: []models.ArchiveNode{{ID: "AN-1", ArchiveID: "AR-1", Label: "Фонд 1"}}}

	rec := get(t, NewHandler(Deps{ArchiveNodes: svc, DocsFS: fstest.MapFS{}}), "/api/archive-nodes?archive_id=AR-1&parent_id=AN-0")
	requireStatus(t, rec, http.StatusOK)

	if svc.gotList.ArchiveID != "AR-1" || svc.gotList.ParentID == nil || *svc.gotList.ParentID != "AN-0" {
		t.Fatalf("gotList = %+v", svc.gotList)
	}
}

func TestArchiveNodeGetNotFound(t *testing.T) {
	svc := &fakeArchiveNodes{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{ArchiveNodes: svc, DocsFS: fstest.MapFS{}}), "/api/archive-nodes/AN-1")
	requireStatus(t, rec, http.StatusNotFound)
}

func TestArchiveNodeSearchPassesQuery(t *testing.T) {
	svc := &fakeArchiveNodes{}

	rec := get(t, NewHandler(Deps{ArchiveNodes: svc, DocsFS: fstest.MapFS{}}), "/api/archive-nodes/search?q=фонд")
	requireStatus(t, rec, http.StatusOK)

	if svc.gotSearch.Text != "фонд" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}

func TestArchiveNodeListReturnsRecords(t *testing.T) {
	svc := &fakeArchiveNodes{list: []models.ArchiveNode{{ID: "AN-1", ArchiveID: "AR-1", Type: "fond", Label: "Фонд 1"}}}

	rec := get(t, NewHandler(Deps{ArchiveNodes: svc, DocsFS: fstest.MapFS{}}), "/api/archive-nodes?archive_id=AR-1")
	requireStatus(t, rec, http.StatusOK)

	want := `[{"id":"AN-1","type":"fond","archive_id":"AR-1","label":"Фонд 1","name":"","settlements":[],"notes":[],"sources":[],"private":false}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}
