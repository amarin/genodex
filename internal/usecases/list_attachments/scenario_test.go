package list_attachments

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.Attachment
}

func (f *fakeRepo) ListAttachments(_ context.Context, _ models.Access, page models.Page) ([]*models.Attachment, error) {
	f.page = page

	return f.out, nil
}

func TestListAttachmentsReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.Attachment{{ID: "O-01ARZ3NDEKTSV4RRFFQ69G5FA1", Kind: models.AttachmentKindScan, Filename: "0012.jpg"}}}

	got, err := New(repo).ListAttachments(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListAttachments: %v", err)
	}

	if len(got) != 1 || got[0].Filename != "0012.jpg" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListAttachmentsRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListAttachments(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
