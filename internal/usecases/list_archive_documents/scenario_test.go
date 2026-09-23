package list_archive_documents

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.ArchiveDocument
}

func (f *fakeRepo) ListArchiveDocuments(_ context.Context, _ models.Access, page models.Page) ([]*models.ArchiveDocument, error) {
	f.page = page

	return f.out, nil
}

func TestListArchiveDocumentsReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.ArchiveDocument{{ID: "DC-01ARZ3NDEKTSV4RRFFQ69G5FA1", Title: "Метрическая книга 1890"}}}

	got, err := New(repo).ListArchiveDocuments(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListArchiveDocuments: %v", err)
	}

	if len(got) != 1 || got[0].Title != "Метрическая книга 1890" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListArchiveDocumentsRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListArchiveDocuments(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}

func TestListArchiveDocumentsRejectsNegativeOffset(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListArchiveDocuments(context.Background(), models.AccessFull, models.Page{Offset: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "offset" {
		t.Fatalf("err = %v, want ValidationError on offset", err)
	}
}
