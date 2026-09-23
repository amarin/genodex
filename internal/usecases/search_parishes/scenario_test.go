package search_parishes

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits     []models.Hit
	parishes map[models.ID]*models.Parish
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetParish(_ context.Context, id models.ID) (*models.Parish, error) {
	s, ok := f.parishes[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestSearchParishesFiltersByType(t *testing.T) {
	id := models.ID("PR-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeParish, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не parish — должен быть пропущен
		},
		parishes: map[models.ID]*models.Parish{id: {ID: id, Name: "Никольский приход"}},
	}

	got, err := New(repo).SearchParishes(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchParishes: %v", err)
	}

	if len(got) != 1 || got[0].Name != "Никольский приход" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchParishesEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchParishes(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchParishes: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
