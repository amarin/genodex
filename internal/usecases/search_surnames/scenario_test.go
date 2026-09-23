package search_surnames

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits     []models.Hit
	surnames map[models.ID]*models.Surname
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetSurname(_ context.Context, id models.ID) (*models.Surname, error) {
	s, ok := f.surnames[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestSearchSurnamesFiltersByType(t *testing.T) {
	id := models.ID("SN-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeSurname, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не surname — должен быть пропущен
		},
		surnames: map[models.ID]*models.Surname{id: {ID: id, Canonical: "Иванов"}},
	}

	got, err := New(repo).SearchSurnames(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchSurnames: %v", err)
	}

	if len(got) != 1 || got[0].Canonical != "Иванов" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchSurnamesEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchSurnames(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchSurnames: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
