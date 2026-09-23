package search_churches

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits     []models.Hit
	churches map[models.ID]*models.Church
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetChurch(_ context.Context, id models.ID) (*models.Church, error) {
	s, ok := f.churches[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestSearchChurchesFiltersByType(t *testing.T) {
	id := models.ID("CH-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeChurch, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не church — должен быть пропущен
		},
		churches: map[models.ID]*models.Church{id: {ID: id, Name: "Никольская церковь"}},
	}

	got, err := New(repo).SearchChurches(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchChurches: %v", err)
	}

	if len(got) != 1 || got[0].Name != "Никольская церковь" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchChurchesEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchChurches(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchChurches: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
