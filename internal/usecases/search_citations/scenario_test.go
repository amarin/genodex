package search_citations

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits      []models.Hit
	citations map[models.ID]*models.Citation
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return c, nil
}

func TestSearchCitationsFiltersByType(t *testing.T) {
	id := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	src := models.ID("S-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeCitation, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не citation — должен быть пропущен
		},
		citations: map[models.ID]*models.Citation{id: {ID: id, SourceID: src, Text: "стр. 12, запись о рождении"}},
	}

	got, err := New(repo).SearchCitations(context.Background(), models.AccessFull, models.SearchQuery{Text: "стр"})
	if err != nil {
		t.Fatalf("SearchCitations: %v", err)
	}

	if len(got) != 1 || got[0].Text != "стр. 12, запись о рождении" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchCitationsEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchCitations(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchCitations: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
