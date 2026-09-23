package list_archives

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.Archive
}

func (f *fakeRepo) ListArchives(_ context.Context, _ models.Access, page models.Page) ([]*models.Archive, error) {
	f.page = page

	return f.out, nil
}

func TestListArchivesReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.Archive{{ID: "AR-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "ГАВО, архив"}}}

	got, err := New(repo).ListArchives(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListArchives: %v", err)
	}

	if len(got) != 1 || got[0].Name != "ГАВО, архив" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListArchivesRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListArchives(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
