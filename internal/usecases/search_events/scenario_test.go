package search_events

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits   []models.Hit
	events map[models.ID]*models.Event
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetEvent(_ context.Context, id models.ID) (*models.Event, error) {
	e, ok := f.events[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return e, nil
}

func TestSearchEventsFiltersByType(t *testing.T) {
	id := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("F-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeEvent, ID: id},
			{Type: models.TypeFamily, ID: otherID}, // не event — должен быть пропущен
		},
		events: map[models.ID]*models.Event{
			id: {ID: id, Type: models.EventTypeBirth, Place: &models.PlaceRef{Text: "Давыдово"}},
		},
	}

	got, err := New(repo).SearchEvents(context.Background(), models.AccessFull, models.SearchQuery{Text: "Дав"})
	if err != nil {
		t.Fatalf("SearchEvents: %v", err)
	}

	if len(got) != 1 || got[0].Place == nil || got[0].Place.Text != "Давыдово" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchEventsEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchEvents(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchEvents: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
