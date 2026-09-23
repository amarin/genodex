package list_estates

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.Estate
}

func (f *fakeRepo) ListEstates(_ context.Context, _ models.Access, page models.Page) ([]*models.Estate, error) {
	f.page = page

	return f.out, nil
}

func TestListEstatesReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.Estate{{ID: "ES-01ARZ3NDEKTSV4RRFFQ69G5FA1", Canonical: "Иванов"}}}

	got, err := New(repo).ListEstates(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListEstates: %v", err)
	}

	if len(got) != 1 || got[0].Canonical != "Иванов" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListEstatesRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListEstates(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
