package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeParishes struct {
	list []models.Parish
	err  error
	page models.Page

	getP      models.Parish
	gotIDs    []models.ID
	created   models.Parish
	gotCreate models.Parish
	updated   models.Parish
	deleteErr error

	search    []models.Parish
	gotSearch models.SearchQuery
}

func (f *fakeParishes) ListParishes(_ context.Context, _ models.Access, page models.Page) ([]models.Parish, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeParishes) SearchParishes(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Parish, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeParishes) GetParish(_ context.Context, id models.ID) (models.Parish, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Parish{}, f.err
	}

	return f.getP, nil
}

func (f *fakeParishes) CreateParish(_ context.Context, p models.Parish) (models.Parish, error) {
	f.gotCreate = p
	if f.err != nil {
		return models.Parish{}, f.err
	}

	return f.created, nil
}

func (f *fakeParishes) UpdateParish(_ context.Context, p models.Parish) error {
	f.updated = p

	return f.err
}

func (f *fakeParishes) DeleteParish(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestParishListReturnsRecords(t *testing.T) {
	svc := &fakeParishes{list: []models.Parish{{ID: "PR-1", Name: "Никольский приход"}}}

	rec := get(t, NewHandler(Deps{Parishes: svc, DocsFS: fstest.MapFS{}}), "/api/parishes")
	requireStatus(t, rec, 200)

	want := `[{"id":"PR-1","name":"Никольский приход","settlements":[],"notes":[],"sources":[]}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestParishGetNotFound(t *testing.T) {
	svc := &fakeParishes{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Parishes: svc, DocsFS: fstest.MapFS{}}), "/api/parishes/PR-1")
	requireStatus(t, rec, 404)
}

func TestParishSearchPassesQuery(t *testing.T) {
	svc := &fakeParishes{}

	rec := get(t, NewHandler(Deps{Parishes: svc, DocsFS: fstest.MapFS{}}), "/api/parishes/search?q=Никол")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "Никол" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
