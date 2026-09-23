package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeSurnames struct {
	list []models.Surname
	err  error
	page models.Page

	getSN     models.Surname
	gotIDs    []models.ID
	created   models.Surname
	gotCreate models.Surname
	updated   models.Surname
	deleteErr error

	search    []models.Surname
	gotSearch models.SearchQuery
}

func (f *fakeSurnames) ListSurnames(_ context.Context, _ models.Access, page models.Page) ([]models.Surname, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeSurnames) SearchSurnames(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Surname, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeSurnames) GetSurname(_ context.Context, id models.ID) (models.Surname, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Surname{}, f.err
	}

	return f.getSN, nil
}

func (f *fakeSurnames) CreateSurname(_ context.Context, sn models.Surname) (models.Surname, error) {
	f.gotCreate = sn
	if f.err != nil {
		return models.Surname{}, f.err
	}

	return f.created, nil
}

func (f *fakeSurnames) UpdateSurname(_ context.Context, sn models.Surname) error {
	f.updated = sn

	return f.err
}

func (f *fakeSurnames) DeleteSurname(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestSurnameListReturnsRecords(t *testing.T) {
	svc := &fakeSurnames{list: []models.Surname{{ID: "SN-1", Canonical: "Иванов", Variants: []models.TextRef{{Text: "Иванова"}}}}}

	rec := get(t, NewHandler(Deps{Surnames: svc, DocsFS: fstest.MapFS{}}), "/api/surnames")
	requireStatus(t, rec, 200)

	want := `[{"id":"SN-1","canonical":"Иванов","variants":[{"text":"Иванова"}],"items":[],"notes":[]}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestSurnameGetNotFound(t *testing.T) {
	svc := &fakeSurnames{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Surnames: svc, DocsFS: fstest.MapFS{}}), "/api/surnames/SN-1")
	requireStatus(t, rec, 404)
}

func TestSurnameSearchPassesQuery(t *testing.T) {
	svc := &fakeSurnames{}

	rec := get(t, NewHandler(Deps{Surnames: svc, DocsFS: fstest.MapFS{}}), "/api/surnames/search?q=Ив")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "Ив" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
