package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeChurches struct {
	list []models.Church
	err  error
	page models.Page

	getC      models.Church
	gotIDs    []models.ID
	created   models.Church
	gotCreate models.Church
	updated   models.Church
	deleteErr error

	search    []models.Church
	gotSearch models.SearchQuery
}

func (f *fakeChurches) ListChurches(_ context.Context, _ models.Access, page models.Page) ([]models.Church, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeChurches) SearchChurches(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Church, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeChurches) GetChurch(_ context.Context, id models.ID) (models.Church, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Church{}, f.err
	}

	return f.getC, nil
}

func (f *fakeChurches) CreateChurch(_ context.Context, c models.Church) (models.Church, error) {
	f.gotCreate = c
	if f.err != nil {
		return models.Church{}, f.err
	}

	return f.created, nil
}

func (f *fakeChurches) UpdateChurch(_ context.Context, c models.Church) error {
	f.updated = c

	return f.err
}

func (f *fakeChurches) DeleteChurch(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestChurchListReturnsRecords(t *testing.T) {
	svc := &fakeChurches{list: []models.Church{{ID: "CH-1", Name: "Никольская церковь"}}}

	rec := get(t, NewHandler(Deps{Churches: svc, DocsFS: fstest.MapFS{}}), "/api/churches")
	requireStatus(t, rec, 200)

	want := `[{"id":"CH-1","name":"Никольская церковь","settlements":[],"variants":[],"notes":[],"sources":[]}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestChurchGetNotFound(t *testing.T) {
	svc := &fakeChurches{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Churches: svc, DocsFS: fstest.MapFS{}}), "/api/churches/CH-1")
	requireStatus(t, rec, 404)
}

func TestChurchSearchPassesQuery(t *testing.T) {
	svc := &fakeChurches{}

	rec := get(t, NewHandler(Deps{Churches: svc, DocsFS: fstest.MapFS{}}), "/api/churches/search?q=Никол")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "Никол" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
