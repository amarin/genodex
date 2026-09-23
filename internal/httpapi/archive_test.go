package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeArchives struct {
	list []models.Archive
	err  error
	page models.Page

	getA      models.Archive
	gotIDs    []models.ID
	created   models.Archive
	gotCreate models.Archive
	updated   models.Archive
	deleteErr error

	search    []models.Archive
	gotSearch models.SearchQuery
}

func (f *fakeArchives) ListArchives(_ context.Context, _ models.Access, page models.Page) ([]models.Archive, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeArchives) SearchArchives(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Archive, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeArchives) GetArchive(_ context.Context, id models.ID) (models.Archive, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Archive{}, f.err
	}

	return f.getA, nil
}

func (f *fakeArchives) CreateArchive(_ context.Context, a models.Archive) (models.Archive, error) {
	f.gotCreate = a
	if f.err != nil {
		return models.Archive{}, f.err
	}

	return f.created, nil
}

func (f *fakeArchives) UpdateArchive(_ context.Context, a models.Archive) error {
	f.updated = a

	return f.err
}

func (f *fakeArchives) DeleteArchive(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestArchiveListReturnsRecords(t *testing.T) {
	svc := &fakeArchives{list: []models.Archive{{ID: "AR-1", Name: "ГАВО, архив"}}}

	rec := get(t, NewHandler(Deps{Archives: svc, DocsFS: fstest.MapFS{}}), "/api/archives")
	requireStatus(t, rec, 200)

	want := `[{"id":"AR-1","name":"ГАВО, архив","notes":[],"sources":[],"private":false}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestArchiveGetNotFound(t *testing.T) {
	svc := &fakeArchives{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Archives: svc, DocsFS: fstest.MapFS{}}), "/api/archives/AR-1")
	requireStatus(t, rec, 404)
}

func TestArchiveSearchPassesQuery(t *testing.T) {
	svc := &fakeArchives{}

	rec := get(t, NewHandler(Deps{Archives: svc, DocsFS: fstest.MapFS{}}), "/api/archives/search?q=ГАВ")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "ГАВ" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
