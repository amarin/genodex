package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeAttachments struct {
	list []models.Attachment
	err  error
	page models.Page

	getA      models.Attachment
	gotIDs    []models.ID
	created   models.Attachment
	gotCreate models.Attachment
	updated   models.Attachment
	deleteErr error

	search    []models.Attachment
	gotSearch models.SearchQuery
}

func (f *fakeAttachments) ListAttachments(_ context.Context, _ models.Access, page models.Page) ([]models.Attachment, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeAttachments) SearchAttachments(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Attachment, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeAttachments) GetAttachment(_ context.Context, _ models.Access, id models.ID) (models.Attachment, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Attachment{}, f.err
	}

	return f.getA, nil
}

func (f *fakeAttachments) CreateAttachment(_ context.Context, a models.Attachment) (models.Attachment, error) {
	f.gotCreate = a
	if f.err != nil {
		return models.Attachment{}, f.err
	}

	return f.created, nil
}

func (f *fakeAttachments) UpdateAttachment(_ context.Context, a models.Attachment) error {
	f.updated = a

	return f.err
}

func (f *fakeAttachments) DeleteAttachment(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestAttachmentListReturnsRecords(t *testing.T) {
	svc := &fakeAttachments{list: []models.Attachment{{ID: "O-1", Kind: models.AttachmentKindScan, Filename: "0012.jpg", NodeID: "AN-1"}}}

	rec := get(t, NewHandler(Deps{Attachments: svc, DocsFS: fstest.MapFS{}}), "/api/attachments")
	requireStatus(t, rec, 200)

	want := `[{"id":"O-1","kind":"scan","filename":"0012.jpg","node_id":"AN-1","private":false}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestAttachmentGetNotFound(t *testing.T) {
	svc := &fakeAttachments{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Attachments: svc, DocsFS: fstest.MapFS{}}), "/api/attachments/O-1")
	requireStatus(t, rec, 404)
}

func TestAttachmentSearchPassesQuery(t *testing.T) {
	svc := &fakeAttachments{}

	rec := get(t, NewHandler(Deps{Attachments: svc, DocsFS: fstest.MapFS{}}), "/api/attachments/search?q=0012")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "0012" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
