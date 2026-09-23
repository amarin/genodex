package list_citations

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.Citation
}

func (f *fakeRepo) ListCitations(_ context.Context, _ models.Access, page models.Page) ([]*models.Citation, error) {
	f.page = page

	return f.out, nil
}

func TestListCitationsReturnsRecords(t *testing.T) {
	src := models.ID("S-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{out: []*models.Citation{{ID: "C-01ARZ3NDEKTSV4RRFFQ69G5FA1", SourceID: src, Text: "стр. 12"}}}

	got, err := New(repo).ListCitations(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListCitations: %v", err)
	}

	if len(got) != 1 || got[0].Text != "стр. 12" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListCitationsRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListCitations(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
