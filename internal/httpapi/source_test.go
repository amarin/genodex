package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeSources struct {
	list []models.Source
	err  error
	page models.Page

	getS      models.Source
	gotIDs    []models.ID
	created   models.Source
	gotCreate models.Source
	updated   models.Source
	deleteErr error

	search    []models.Source
	gotSearch models.SearchQuery
}

func (f *fakeSources) ListSources(_ context.Context, _ models.Access, page models.Page) ([]models.Source, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeSources) SearchSources(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Source, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeSources) GetSource(_ context.Context, _ models.Access, id models.ID) (models.Source, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Source{}, f.err
	}

	return f.getS, nil
}

func (f *fakeSources) CreateSource(_ context.Context, s models.Source) (models.Source, error) {
	f.gotCreate = s
	if f.err != nil {
		return models.Source{}, f.err
	}

	return f.created, nil
}

func (f *fakeSources) UpdateSource(_ context.Context, s models.Source) error {
	f.updated = s

	return f.err
}

func (f *fakeSources) DeleteSource(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestSourceListReturnsRecords(t *testing.T) {
	svc := &fakeSources{list: []models.Source{{
		ID:          "S-1",
		Kind:        models.SourceKindDocument,
		Title:       "Ревизская сказка",
		Reliability: models.ReliabilityPrimary,
	}}}

	rec := get(t, NewHandler(Deps{Sources: svc, DocsFS: fstest.MapFS{}}), "/api/sources")
	requireStatus(t, rec, 200)

	want := `[{"id":"S-1","kind":"document","title":"Ревизская сказка","reliability":"primary","notes":[],"private":false}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestSourceGetNotFound(t *testing.T) {
	svc := &fakeSources{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Sources: svc, DocsFS: fstest.MapFS{}}), "/api/sources/S-1")
	requireStatus(t, rec, 404)
}

func TestSourceSearchPassesQuery(t *testing.T) {
	svc := &fakeSources{}

	rec := get(t, NewHandler(Deps{Sources: svc, DocsFS: fstest.MapFS{}}), "/api/sources/search?q=Ревиз")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "Ревиз" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
