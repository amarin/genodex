package search_events

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits   []models.Hit
	events map[models.ID]*models.Event
	people map[models.ID]*models.Person
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

func (f *fakeRepo) GetPerson(_ context.Context, id models.ID) (*models.Person, error) {
	p, ok := f.people[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return p, nil
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

// TestSearchEventsHidesHitReferencingPrivateParticipant: найденное событие
// само по себе не приватно, но один из его участников приватен — для
// вызывающего без полного доступа оно исключается из результата поиска
// (тот же приём, что и для исчезнувшей между поиском и чтением записи:
// матч пропускается, а не превращается в ошибку).
func TestSearchEventsHidesHitReferencingPrivateParticipant(t *testing.T) {
	id := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	person := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		hits: []models.Hit{{Type: models.TypeEvent, ID: id}},
		events: map[models.ID]*models.Event{
			id: {
				ID: id, Private: false, Place: &models.PlaceRef{Text: "Давыдово"},
				Participants: []models.EventParticipant{{PersonID: person}},
			},
		},
		people: map[models.ID]*models.Person{person: {ID: person, Private: true}},
	}

	got, err := New(repo).SearchEvents(context.Background(), models.AccessPublic, models.SearchQuery{Text: "Дав"})
	if err != nil {
		t.Fatalf("SearchEvents: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty (участник приватен)", got)
	}
}

// TestSearchEventsShowsHitReferencingPrivateParticipantWithFullAccess: то
// же событие, но для вызывающего с полным доступом — находится.
func TestSearchEventsShowsHitReferencingPrivateParticipantWithFullAccess(t *testing.T) {
	id := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	person := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		hits: []models.Hit{{Type: models.TypeEvent, ID: id}},
		events: map[models.ID]*models.Event{
			id: {
				ID: id, Private: false, Place: &models.PlaceRef{Text: "Давыдово"},
				Participants: []models.EventParticipant{{PersonID: person}},
			},
		},
		people: map[models.ID]*models.Person{person: {ID: person, Private: true}},
	}

	got, err := New(repo).SearchEvents(context.Background(), models.AccessFull, models.SearchQuery{Text: "Дав"})
	if err != nil {
		t.Fatalf("SearchEvents: %v", err)
	}

	if len(got) != 1 || got[0].ID != id {
		t.Fatalf("got = %+v", got)
	}
}

// TestSearchEventsDoesNotHideHitReferencingPublicParticipant: участник
// публичен — событие не пропадает из результата ошибочно.
func TestSearchEventsDoesNotHideHitReferencingPublicParticipant(t *testing.T) {
	id := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	person := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		hits: []models.Hit{{Type: models.TypeEvent, ID: id}},
		events: map[models.ID]*models.Event{
			id: {
				ID: id, Private: false, Place: &models.PlaceRef{Text: "Давыдово"},
				Participants: []models.EventParticipant{{PersonID: person}},
			},
		},
		people: map[models.ID]*models.Person{person: {ID: person, Private: false}},
	}

	got, err := New(repo).SearchEvents(context.Background(), models.AccessPublic, models.SearchQuery{Text: "Дав"})
	if err != nil {
		t.Fatalf("SearchEvents: %v", err)
	}

	if len(got) != 1 || got[0].ID != id {
		t.Fatalf("got = %+v", got)
	}
}
