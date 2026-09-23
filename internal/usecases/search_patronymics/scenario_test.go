package search_patronymics

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits        []models.Hit
	patronymics map[models.ID]*models.Patronymic
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetPatronymic(_ context.Context, id models.ID) (*models.Patronymic, error) {
	s, ok := f.patronymics[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestSearchPatronymicsFiltersByType(t *testing.T) {
	id := models.ID("PN-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypePatronymic, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не patronymic — должен быть пропущен
		},
		patronymics: map[models.ID]*models.Patronymic{id: {ID: id, Canonical: "Иванов"}},
	}

	got, err := New(repo).SearchPatronymics(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchPatronymics: %v", err)
	}

	if len(got) != 1 || got[0].Canonical != "Иванов" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchPatronymicsEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchPatronymics(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchPatronymics: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
