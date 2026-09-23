package search_sources

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits    []models.Hit
	sources map[models.ID]*models.Source
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetSource(_ context.Context, id models.ID) (*models.Source, error) {
	s, ok := f.sources[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestSearchSourcesFiltersByType(t *testing.T) {
	id := models.ID("S-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeSource, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не source — должен быть пропущен
		},
		sources: map[models.ID]*models.Source{id: {ID: id, Kind: models.SourceKindDocument, Title: "Метрическая книга", Author: "Иванов", Reliability: models.ReliabilityPrimary}},
	}

	got, err := New(repo).SearchSources(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchSources: %v", err)
	}

	if len(got) != 1 || got[0].Title != "Метрическая книга" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchSourcesEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchSources(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchSources: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
