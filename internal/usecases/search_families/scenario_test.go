package search_families

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits     []models.Hit
	families map[models.ID]*models.Family
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetFamily(_ context.Context, id models.ID) (*models.Family, error) {
	s, ok := f.families[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestSearchFamiliesFiltersByType(t *testing.T) {
	id := models.ID("F-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeFamily, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не family — должен быть пропущен
		},
		families: map[models.ID]*models.Family{id: {ID: id, Name: "Ивановы"}},
	}

	got, err := New(repo).SearchFamilies(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchFamilies: %v", err)
	}

	if len(got) != 1 || got[0].Name != "Ивановы" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchFamiliesEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchFamilies(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchFamilies: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
