package list_families

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.Family
}

func (f *fakeRepo) ListFamilies(_ context.Context, _ models.Access, page models.Page) ([]*models.Family, error) {
	f.page = page

	return f.out, nil
}

func TestListFamiliesReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.Family{{ID: "F-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "Ивановы"}}}

	got, err := New(repo).ListFamilies(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListFamilies: %v", err)
	}

	if len(got) != 1 || got[0].Name != "Ивановы" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListFamiliesRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListFamilies(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
