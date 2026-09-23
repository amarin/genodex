package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeGivenNames struct {
	list []models.GivenName
	err  error
	page models.Page

	getSN     models.GivenName
	gotIDs    []models.ID
	created   models.GivenName
	gotCreate models.GivenName
	updated   models.GivenName
	deleteErr error

	search    []models.GivenName
	gotSearch models.SearchQuery
}

func (f *fakeGivenNames) ListGivenNames(_ context.Context, _ models.Access, page models.Page) ([]models.GivenName, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeGivenNames) SearchGivenNames(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.GivenName, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeGivenNames) GetGivenName(_ context.Context, id models.ID) (models.GivenName, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.GivenName{}, f.err
	}

	return f.getSN, nil
}

func (f *fakeGivenNames) CreateGivenName(_ context.Context, sn models.GivenName) (models.GivenName, error) {
	f.gotCreate = sn
	if f.err != nil {
		return models.GivenName{}, f.err
	}

	return f.created, nil
}

func (f *fakeGivenNames) UpdateGivenName(_ context.Context, sn models.GivenName) error {
	f.updated = sn

	return f.err
}

func (f *fakeGivenNames) DeleteGivenName(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestGivenNameListReturnsRecords(t *testing.T) {
	svc := &fakeGivenNames{list: []models.GivenName{{ID: "GN-1", Canonical: "Иванов", Variants: []models.TextRef{{Text: "Иванова"}}}}}

	rec := get(t, NewHandler(Deps{GivenNames: svc, DocsFS: fstest.MapFS{}}), "/api/given-names")
	requireStatus(t, rec, 200)

	want := `[{"id":"GN-1","canonical":"Иванов","gender":"","variants":[{"text":"Иванова"}],"items":[],"notes":[]}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestGivenNameGetNotFound(t *testing.T) {
	svc := &fakeGivenNames{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{GivenNames: svc, DocsFS: fstest.MapFS{}}), "/api/given-names/GN-1")
	requireStatus(t, rec, 404)
}

func TestGivenNameSearchPassesQuery(t *testing.T) {
	svc := &fakeGivenNames{}

	rec := get(t, NewHandler(Deps{GivenNames: svc, DocsFS: fstest.MapFS{}}), "/api/given-names/search?q=Ив")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "Ив" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
