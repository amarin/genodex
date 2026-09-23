package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeTitles struct {
	list []models.Title
	err  error
	page models.Page

	getSN     models.Title
	gotIDs    []models.ID
	created   models.Title
	gotCreate models.Title
	updated   models.Title
	deleteErr error

	search    []models.Title
	gotSearch models.SearchQuery
}

func (f *fakeTitles) ListTitles(_ context.Context, _ models.Access, page models.Page) ([]models.Title, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeTitles) SearchTitles(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Title, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeTitles) GetTitle(_ context.Context, id models.ID) (models.Title, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Title{}, f.err
	}

	return f.getSN, nil
}

func (f *fakeTitles) CreateTitle(_ context.Context, sn models.Title) (models.Title, error) {
	f.gotCreate = sn
	if f.err != nil {
		return models.Title{}, f.err
	}

	return f.created, nil
}

func (f *fakeTitles) UpdateTitle(_ context.Context, sn models.Title) error {
	f.updated = sn

	return f.err
}

func (f *fakeTitles) DeleteTitle(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestTitleListReturnsRecords(t *testing.T) {
	svc := &fakeTitles{list: []models.Title{{ID: "TT-1", Canonical: "Иванов", Variants: []models.TextRef{{Text: "Иванова"}}}}}

	rec := get(t, NewHandler(Deps{Titles: svc, DocsFS: fstest.MapFS{}}), "/api/titles")
	requireStatus(t, rec, 200)

	want := `[{"id":"TT-1","canonical":"Иванов","variants":[{"text":"Иванова"}],"items":[],"notes":[]}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestTitleGetNotFound(t *testing.T) {
	svc := &fakeTitles{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Titles: svc, DocsFS: fstest.MapFS{}}), "/api/titles/TT-1")
	requireStatus(t, rec, 404)
}

func TestTitleSearchPassesQuery(t *testing.T) {
	svc := &fakeTitles{}

	rec := get(t, NewHandler(Deps{Titles: svc, DocsFS: fstest.MapFS{}}), "/api/titles/search?q=Ив")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "Ив" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
