package search_repositories

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits         []models.Hit
	repositories map[models.ID]*models.Repository
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetRepository(_ context.Context, id models.ID) (*models.Repository, error) {
	s, ok := f.repositories[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestSearchRepositoriesFiltersByType(t *testing.T) {
	id := models.ID("R-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeRepository, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не repository — должен быть пропущен
		},
		repositories: map[models.ID]*models.Repository{id: {ID: id, Name: "ГАВО"}},
	}

	got, err := New(repo).SearchRepositories(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchRepositories: %v", err)
	}

	if len(got) != 1 || got[0].Name != "ГАВО" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchRepositoriesEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchRepositories(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchRepositories: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
