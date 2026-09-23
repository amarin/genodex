package search_notes

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits  []models.Hit
	notes map[models.ID]*models.Note
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetNote(_ context.Context, id models.ID) (*models.Note, error) {
	s, ok := f.notes[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestSearchNotesFiltersByType(t *testing.T) {
	id := models.ID("N-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeNote, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не note — должен быть пропущен
		},
		notes: map[models.ID]*models.Note{id: {ID: id, Kind: "note", Text: "текст"}},
	}

	got, err := New(repo).SearchNotes(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchNotes: %v", err)
	}

	if len(got) != 1 || got[0].Text != "текст" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchNotesEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchNotes(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchNotes: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
