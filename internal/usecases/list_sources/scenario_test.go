package list_sources

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.Source
}

func (f *fakeRepo) ListSources(_ context.Context, _ models.Access, page models.Page) ([]*models.Source, error) {
	f.page = page

	return f.out, nil
}

func TestListSourcesReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.Source{{ID: "S-01ARZ3NDEKTSV4RRFFQ69G5FA1", Kind: models.SourceKindDocument, Title: "Метрическая книга", Reliability: models.ReliabilityPrimary}}}

	got, err := New(repo).ListSources(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListSources: %v", err)
	}

	if len(got) != 1 || got[0].Title != "Метрическая книга" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListSourcesRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListSources(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
