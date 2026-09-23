package search_given_names

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits       []models.Hit
	givenNames map[models.ID]*models.GivenName
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetGivenName(_ context.Context, id models.ID) (*models.GivenName, error) {
	s, ok := f.givenNames[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestSearchGivenNamesFiltersByType(t *testing.T) {
	id := models.ID("GN-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeGivenName, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не givenName — должен быть пропущен
		},
		givenNames: map[models.ID]*models.GivenName{id: {ID: id, Canonical: "Иванов"}},
	}

	got, err := New(repo).SearchGivenNames(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchGivenNames: %v", err)
	}

	if len(got) != 1 || got[0].Canonical != "Иванов" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchGivenNamesEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchGivenNames(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchGivenNames: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
