package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeArchiveDocuments struct {
	list []models.ArchiveDocument
	err  error
	page models.Page

	getD      models.ArchiveDocument
	gotIDs    []models.ID
	created   models.ArchiveDocument
	gotCreate models.ArchiveDocument
	updated   models.ArchiveDocument
	deleteErr error

	search    []models.ArchiveDocument
	gotSearch models.SearchQuery
}

func (f *fakeArchiveDocuments) ListArchiveDocuments(_ context.Context, _ models.Access, page models.Page) ([]models.ArchiveDocument, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeArchiveDocuments) SearchArchiveDocuments(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.ArchiveDocument, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeArchiveDocuments) GetArchiveDocument(_ context.Context, _ models.Access, id models.ID) (models.ArchiveDocument, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.ArchiveDocument{}, f.err
	}

	return f.getD, nil
}

func (f *fakeArchiveDocuments) CreateArchiveDocument(_ context.Context, d models.ArchiveDocument) (models.ArchiveDocument, error) {
	f.gotCreate = d
	if f.err != nil {
		return models.ArchiveDocument{}, f.err
	}

	return f.created, nil
}

func (f *fakeArchiveDocuments) UpdateArchiveDocument(_ context.Context, d models.ArchiveDocument) error {
	f.updated = d

	return f.err
}

func (f *fakeArchiveDocuments) DeleteArchiveDocument(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestArchiveDocumentListReturnsRecords(t *testing.T) {
	svc := &fakeArchiveDocuments{list: []models.ArchiveDocument{{ID: "DC-1", UnitID: "AN-1", Title: "Метрическая книга"}}}

	rec := get(t, NewHandler(Deps{ArchiveDocs: svc, DocsFS: fstest.MapFS{}}), "/api/archive-documents")
	requireStatus(t, rec, 200)

	want := `[{"id":"DC-1","unit_id":"AN-1","title":"Метрическая книга","kind":"","settlements":[],"notes":[],"sources":[],"private":false}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestArchiveDocumentGetNotFound(t *testing.T) {
	svc := &fakeArchiveDocuments{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{ArchiveDocs: svc, DocsFS: fstest.MapFS{}}), "/api/archive-documents/DC-1")
	requireStatus(t, rec, 404)
}

func TestArchiveDocumentSearchPassesQuery(t *testing.T) {
	svc := &fakeArchiveDocuments{}

	rec := get(t, NewHandler(Deps{ArchiveDocs: svc, DocsFS: fstest.MapFS{}}), "/api/archive-documents/search?q=метрич")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "метрич" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
