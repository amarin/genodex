package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeFamilies struct {
	list []models.Family
	err  error
	page models.Page

	getF      models.Family
	gotIDs    []models.ID
	created   models.Family
	gotCreate models.Family
	updated   models.Family
	deleteErr error

	search    []models.Family
	gotSearch models.SearchQuery
}

func (f *fakeFamilies) ListFamilies(_ context.Context, _ models.Access, page models.Page) ([]models.Family, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeFamilies) SearchFamilies(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Family, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeFamilies) GetFamily(_ context.Context, _ models.Access, id models.ID) (models.Family, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Family{}, f.err
	}

	return f.getF, nil
}

func (f *fakeFamilies) CreateFamily(_ context.Context, fam models.Family) (models.Family, error) {
	f.gotCreate = fam
	if f.err != nil {
		return models.Family{}, f.err
	}

	return f.created, nil
}

func (f *fakeFamilies) UpdateFamily(_ context.Context, fam models.Family) error {
	f.updated = fam

	return f.err
}

func (f *fakeFamilies) DeleteFamily(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestFamilyListReturnsRecords(t *testing.T) {
	svc := &fakeFamilies{list: []models.Family{{ID: "F-1", Name: "Ивановы"}}}

	rec := get(t, NewHandler(Deps{Families: svc, DocsFS: fstest.MapFS{}}), "/api/families")
	requireStatus(t, rec, 200)

	want := `[{"id":"F-1","name":"Ивановы","members":[],"notes":[],"sources":[],"private":false}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestFamilyGetNotFound(t *testing.T) {
	svc := &fakeFamilies{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Families: svc, DocsFS: fstest.MapFS{}}), "/api/families/F-1")
	requireStatus(t, rec, 404)
}

func TestFamilySearchPassesQuery(t *testing.T) {
	svc := &fakeFamilies{}

	rec := get(t, NewHandler(Deps{Families: svc, DocsFS: fstest.MapFS{}}), "/api/families/search?q=Ив")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "Ив" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
