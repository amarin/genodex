package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepositories struct {
	list []models.Repository
	err  error
	page models.Page

	getR      models.Repository
	gotIDs    []models.ID
	created   models.Repository
	gotCreate models.Repository
	updated   models.Repository
	deleteErr error

	search    []models.Repository
	gotSearch models.SearchQuery
}

func (f *fakeRepositories) ListRepositories(_ context.Context, _ models.Access, page models.Page) ([]models.Repository, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeRepositories) SearchRepositories(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Repository, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeRepositories) GetRepository(_ context.Context, _ models.Access, id models.ID) (models.Repository, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Repository{}, f.err
	}

	return f.getR, nil
}

func (f *fakeRepositories) CreateRepository(_ context.Context, r models.Repository) (models.Repository, error) {
	f.gotCreate = r
	if f.err != nil {
		return models.Repository{}, f.err
	}

	return f.created, nil
}

func (f *fakeRepositories) UpdateRepository(_ context.Context, r models.Repository) error {
	f.updated = r

	return f.err
}

func (f *fakeRepositories) DeleteRepository(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestRepositoryListReturnsRecords(t *testing.T) {
	svc := &fakeRepositories{list: []models.Repository{{ID: "R-1", Name: "ГАВО", Type: models.RepositoryTypeArchive}}}

	rec := get(t, NewHandler(Deps{Repositories: svc, DocsFS: fstest.MapFS{}}), "/api/repositories")
	requireStatus(t, rec, 200)

	want := `[{"id":"R-1","name":"ГАВО","type":"archive","urls":[],"notes":[],"sources":[],"private":false}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestRepositoryGetNotFound(t *testing.T) {
	svc := &fakeRepositories{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Repositories: svc, DocsFS: fstest.MapFS{}}), "/api/repositories/R-1")
	requireStatus(t, rec, 404)
}

func TestRepositorySearchPassesQuery(t *testing.T) {
	svc := &fakeRepositories{}

	rec := get(t, NewHandler(Deps{Repositories: svc, DocsFS: fstest.MapFS{}}), "/api/repositories/search?q=ГАВ")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "ГАВ" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
