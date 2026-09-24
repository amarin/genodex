package list_people

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.Person
}

func (f *fakeRepo) ListPeople(_ context.Context, _ models.Access, page models.Page) ([]*models.Person, error) {
	f.page = page

	return f.out, nil
}

func TestListPeopleReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.Person{{ID: "I-01ARZ3NDEKTSV4RRFFQ69G5FA1"}}}

	got, err := New(repo).ListPeople(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListPeople: %v", err)
	}

	if len(got) != 1 || got[0].ID != "I-01ARZ3NDEKTSV4RRFFQ69G5FA1" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListPeopleRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListPeople(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
