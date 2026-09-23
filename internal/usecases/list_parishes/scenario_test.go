package list_parishes

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.Parish
}

func (f *fakeRepo) ListParishes(_ context.Context, _ models.Access, page models.Page) ([]*models.Parish, error) {
	f.page = page

	return f.out, nil
}

func TestListParishesReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.Parish{{ID: "PR-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "Никольский приход"}}}

	got, err := New(repo).ListParishes(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListParishes: %v", err)
	}

	if len(got) != 1 || got[0].Name != "Никольский приход" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListParishesRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListParishes(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
