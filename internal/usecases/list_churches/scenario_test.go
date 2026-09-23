package list_churches

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.Church
}

func (f *fakeRepo) ListChurches(_ context.Context, _ models.Access, page models.Page) ([]*models.Church, error) {
	f.page = page

	return f.out, nil
}

func TestListChurchesReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.Church{{ID: "CH-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "Никольская церковь"}}}

	got, err := New(repo).ListChurches(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListChurches: %v", err)
	}

	if len(got) != 1 || got[0].Name != "Никольская церковь" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListChurchesRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListChurches(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
