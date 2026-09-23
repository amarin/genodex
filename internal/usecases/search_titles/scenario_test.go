package search_titles

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits   []models.Hit
	titles map[models.ID]*models.Title
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetTitle(_ context.Context, id models.ID) (*models.Title, error) {
	s, ok := f.titles[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestSearchTitlesFiltersByType(t *testing.T) {
	id := models.ID("TT-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeTitle, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не title — должен быть пропущен
		},
		titles: map[models.ID]*models.Title{id: {ID: id, Canonical: "Иванов"}},
	}

	got, err := New(repo).SearchTitles(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchTitles: %v", err)
	}

	if len(got) != 1 || got[0].Canonical != "Иванов" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchTitlesEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchTitles(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchTitles: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
