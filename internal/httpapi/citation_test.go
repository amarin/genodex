package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeCitations struct {
	list []models.Citation
	err  error
	page models.Page

	getC      models.Citation
	gotIDs    []models.ID
	created   models.Citation
	gotCreate models.Citation
	updated   models.Citation
	deleteErr error

	search    []models.Citation
	gotSearch models.SearchQuery
}

func (f *fakeCitations) ListCitations(_ context.Context, _ models.Access, page models.Page) ([]models.Citation, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeCitations) SearchCitations(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Citation, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeCitations) GetCitation(_ context.Context, _ models.Access, id models.ID) (models.Citation, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Citation{}, f.err
	}

	return f.getC, nil
}

func (f *fakeCitations) CreateCitation(_ context.Context, c models.Citation) (models.Citation, error) {
	f.gotCreate = c
	if f.err != nil {
		return models.Citation{}, f.err
	}

	return f.created, nil
}

func (f *fakeCitations) UpdateCitation(_ context.Context, c models.Citation) error {
	f.updated = c

	return f.err
}

func (f *fakeCitations) DeleteCitation(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

// TestCitationListReturnsRecords — запись без anchor: тело проще для точного
// сравнения; полиморфная привязка проверяется отдельно в create/update-тестах.
func TestCitationListReturnsRecords(t *testing.T) {
	svc := &fakeCitations{list: []models.Citation{{ID: "C-1", SourceID: "S-1", Text: "запись №5"}}}

	rec := get(t, NewHandler(Deps{Citations: svc, DocsFS: fstest.MapFS{}}), "/api/citations")
	requireStatus(t, rec, 200)

	want := `[{"id":"C-1","source_id":"S-1","text":"запись №5","private":false}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestCitationGetNotFound(t *testing.T) {
	svc := &fakeCitations{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Citations: svc, DocsFS: fstest.MapFS{}}), "/api/citations/C-1")
	requireStatus(t, rec, 404)
}

func TestCitationSearchPassesQuery(t *testing.T) {
	svc := &fakeCitations{}

	rec := get(t, NewHandler(Deps{Citations: svc, DocsFS: fstest.MapFS{}}), "/api/citations/search?q=запись")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "запись" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
