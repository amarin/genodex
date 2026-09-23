package list_repositories

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.Repository
}

func (f *fakeRepo) ListRepositories(_ context.Context, _ models.Access, page models.Page) ([]*models.Repository, error) {
	f.page = page

	return f.out, nil
}

func TestListRepositoriesReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.Repository{{ID: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "ГАВО"}}}

	got, err := New(repo).ListRepositories(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}

	if len(got) != 1 || got[0].Name != "ГАВО" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListRepositoriesRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListRepositories(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
