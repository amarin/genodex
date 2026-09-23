package list_given_names

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.GivenName
}

func (f *fakeRepo) ListGivenNames(_ context.Context, _ models.Access, page models.Page) ([]*models.GivenName, error) {
	f.page = page

	return f.out, nil
}

func TestListGivenNamesReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.GivenName{{ID: "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1", Canonical: "Иванов"}}}

	got, err := New(repo).ListGivenNames(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListGivenNames: %v", err)
	}

	if len(got) != 1 || got[0].Canonical != "Иванов" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListGivenNamesRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListGivenNames(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
