package list_notes

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.Note
}

func (f *fakeRepo) ListNotes(_ context.Context, _ models.Access, page models.Page) ([]*models.Note, error) {
	f.page = page

	return f.out, nil
}

func TestListNotesReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.Note{{ID: "N-01ARZ3NDEKTSV4RRFFQ69G5FA1", Kind: "note", Text: "текст"}}}

	got, err := New(repo).ListNotes(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}

	if len(got) != 1 || got[0].Text != "текст" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListNotesRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListNotes(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
