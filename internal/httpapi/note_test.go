package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeNotes struct {
	list []models.Note
	err  error
	page models.Page

	getN      models.Note
	gotIDs    []models.ID
	created   models.Note
	gotCreate models.Note
	updated   models.Note
	deleteErr error

	search    []models.Note
	gotSearch models.SearchQuery

	gotAccess models.Access
}

func (f *fakeNotes) ListNotes(_ context.Context, _ models.Access, page models.Page) ([]models.Note, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeNotes) SearchNotes(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Note, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeNotes) GetNote(_ context.Context, access models.Access, id models.ID) (models.Note, error) {
	f.gotIDs = append(f.gotIDs, id)
	f.gotAccess = access
	if f.err != nil {
		return models.Note{}, f.err
	}

	return f.getN, nil
}

func (f *fakeNotes) CreateNote(_ context.Context, n models.Note) (models.Note, error) {
	f.gotCreate = n
	if f.err != nil {
		return models.Note{}, f.err
	}

	return f.created, nil
}

func (f *fakeNotes) UpdateNote(_ context.Context, n models.Note) error {
	f.updated = n

	return f.err
}

func (f *fakeNotes) DeleteNote(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestNoteListReturnsRecords(t *testing.T) {
	svc := &fakeNotes{list: []models.Note{{ID: "N-1", Kind: "note", Text: "текст"}}}

	rec := get(t, NewHandler(Deps{Notes: svc, DocsFS: fstest.MapFS{}}), "/api/notes")
	requireStatus(t, rec, 200)

	want := `[{"id":"N-1","kind":"note","text":"текст","sources":[],"private":false}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestNoteGetNotFound(t *testing.T) {
	svc := &fakeNotes{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Notes: svc, DocsFS: fstest.MapFS{}}), "/api/notes/N-1")
	requireStatus(t, rec, 404)
}

func TestNoteSearchPassesQuery(t *testing.T) {
	svc := &fakeNotes{}

	rec := get(t, NewHandler(Deps{Notes: svc, DocsFS: fstest.MapFS{}}), "/api/notes/search?q=текст")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "текст" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
