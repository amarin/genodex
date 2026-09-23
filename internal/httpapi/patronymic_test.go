package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakePatronymics struct {
	list []models.Patronymic
	err  error
	page models.Page

	getSN     models.Patronymic
	gotIDs    []models.ID
	created   models.Patronymic
	gotCreate models.Patronymic
	updated   models.Patronymic
	deleteErr error

	search    []models.Patronymic
	gotSearch models.SearchQuery
}

func (f *fakePatronymics) ListPatronymics(_ context.Context, _ models.Access, page models.Page) ([]models.Patronymic, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakePatronymics) SearchPatronymics(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Patronymic, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakePatronymics) GetPatronymic(_ context.Context, id models.ID) (models.Patronymic, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Patronymic{}, f.err
	}

	return f.getSN, nil
}

func (f *fakePatronymics) CreatePatronymic(_ context.Context, sn models.Patronymic) (models.Patronymic, error) {
	f.gotCreate = sn
	if f.err != nil {
		return models.Patronymic{}, f.err
	}

	return f.created, nil
}

func (f *fakePatronymics) UpdatePatronymic(_ context.Context, sn models.Patronymic) error {
	f.updated = sn

	return f.err
}

func (f *fakePatronymics) DeletePatronymic(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestPatronymicListReturnsRecords(t *testing.T) {
	svc := &fakePatronymics{list: []models.Patronymic{{ID: "PN-1", Canonical: "Иванов", Variants: []models.TextRef{{Text: "Иванова"}}}}}

	rec := get(t, NewHandler(Deps{Patronymics: svc, DocsFS: fstest.MapFS{}}), "/api/patronymics")
	requireStatus(t, rec, 200)

	want := `[{"id":"PN-1","canonical":"Иванов","variants":[{"text":"Иванова"}],"items":[],"notes":[]}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestPatronymicGetNotFound(t *testing.T) {
	svc := &fakePatronymics{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Patronymics: svc, DocsFS: fstest.MapFS{}}), "/api/patronymics/PN-1")
	requireStatus(t, rec, 404)
}

func TestPatronymicSearchPassesQuery(t *testing.T) {
	svc := &fakePatronymics{}

	rec := get(t, NewHandler(Deps{Patronymics: svc, DocsFS: fstest.MapFS{}}), "/api/patronymics/search?q=Ив")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "Ив" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
