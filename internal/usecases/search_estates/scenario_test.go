package search_estates

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits    []models.Hit
	estates map[models.ID]*models.Estate
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetEstate(_ context.Context, id models.ID) (*models.Estate, error) {
	s, ok := f.estates[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestSearchEstatesFiltersByType(t *testing.T) {
	id := models.ID("ES-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeEstate, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не estate — должен быть пропущен
		},
		estates: map[models.ID]*models.Estate{id: {ID: id, Canonical: "Иванов"}},
	}

	got, err := New(repo).SearchEstates(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchEstates: %v", err)
	}

	if len(got) != 1 || got[0].Canonical != "Иванов" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchEstatesEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchEstates(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchEstates: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
