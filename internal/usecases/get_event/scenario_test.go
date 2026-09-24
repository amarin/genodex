package get_event

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	events    map[models.ID]*models.Event
	people    map[models.ID]*models.Person
	citations map[models.ID]*models.Citation
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

func (f *fakeRepo) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return c, nil
}

func TestGetEventReturnsRecord(t *testing.T) {
	id := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{events: map[models.ID]*models.Event{id: {ID: id, Type: models.EventTypeBirth}}}

	got, err := New(repo).GetEvent(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}

	if got.Type != models.EventTypeBirth {
		t.Fatalf("Type = %q", got.Type)
	}
}

func TestGetEventNotFound(t *testing.T) {
	repo := &fakeRepo{events: map[models.ID]*models.Event{}}

	_, err := New(repo).GetEvent(context.Background(), models.AccessFull, "E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetEventInvalidID(t *testing.T) {
	repo := &fakeRepo{events: map[models.ID]*models.Event{}}

	_, err := New(repo).GetEvent(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestGetEventPrivateHiddenFromPublic(t *testing.T) {
	id := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{events: map[models.ID]*models.Event{id: {ID: id, Private: true}}}

	_, err := New(repo).GetEvent(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for private record with AccessPublic", err)
	}
}

func TestGetEventPrivateVisibleToFullAccess(t *testing.T) {
	id := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{events: map[models.ID]*models.Event{id: {ID: id, Private: true}}}

	got, err := New(repo).GetEvent(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}

	if !got.Private {
		t.Fatalf("Private = %v", got.Private)
	}
}

// TestGetEventHiddenWhenParticipantPrivate: событие само по себе не
// приватно, но один из участников приватен — прячется как отсутствующее
// для вызывающего без полного доступа.
func TestGetEventHiddenWhenParticipantPrivate(t *testing.T) {
	id := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	p1 := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	p2 := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		events: map[models.ID]*models.Event{id: {
			ID: id, Private: false,
			Participants: []models.EventParticipant{{PersonID: p1}, {PersonID: p2}},
		}},
		people: map[models.ID]*models.Person{
			p1: {ID: p1, Private: true},
			p2: {ID: p2, Private: false},
		},
	}

	_, err := New(repo).GetEvent(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for public event with a private participant", err)
	}
}

// TestGetEventVisibleToFullAccessWhenParticipantPrivate: то же событие, но
// для вызывающего с полным доступом — видно.
func TestGetEventVisibleToFullAccessWhenParticipantPrivate(t *testing.T) {
	id := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	p1 := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	p2 := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		events: map[models.ID]*models.Event{id: {
			ID: id, Private: false,
			Participants: []models.EventParticipant{{PersonID: p1}, {PersonID: p2}},
		}},
		people: map[models.ID]*models.Person{
			p1: {ID: p1, Private: true},
			p2: {ID: p2, Private: false},
		},
	}

	got, err := New(repo).GetEvent(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}

	if got.ID != id {
		t.Fatalf("got = %+v", got)
	}
}

// TestGetEventHiddenWhenOtherParticipantPrivate: первый участник публичен,
// второй приватен — событие всё равно прячется, каждый участник
// проверяется независимо (не только первый).
func TestGetEventHiddenWhenOtherParticipantPrivate(t *testing.T) {
	id := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	p1 := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	p2 := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		events: map[models.ID]*models.Event{id: {
			ID: id, Private: false,
			Participants: []models.EventParticipant{{PersonID: p1}, {PersonID: p2}},
		}},
		people: map[models.ID]*models.Person{
			p1: {ID: p1, Private: false},
			p2: {ID: p2, Private: true},
		},
	}

	_, err := New(repo).GetEvent(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound when the second participant is private", err)
	}
}

// TestGetEventNotHiddenWhenParticipantsPublic: оба участника публичны —
// событие не прячется ошибочно (защита от false positive).
func TestGetEventNotHiddenWhenParticipantsPublic(t *testing.T) {
	id := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	p1 := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	p2 := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		events: map[models.ID]*models.Event{id: {
			ID: id, Private: false,
			Participants: []models.EventParticipant{{PersonID: p1}, {PersonID: p2}},
		}},
		people: map[models.ID]*models.Person{
			p1: {ID: p1, Private: false},
			p2: {ID: p2, Private: false},
		},
	}

	got, err := New(repo).GetEvent(context.Background(), models.AccessPublic, id)
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}

	if got.ID != id {
		t.Fatalf("got = %+v", got)
	}
}

// TestGetEventHiddenWhenReferencedCitationPrivate: событие само по себе не
// приватно и участники публичны, но одна из Sources ссылается на приватную
// цитату — прячется как отсутствующее для вызывающего без полного доступа.
// Независимая проверка, параллельная TestGetEventHiddenWhenParticipantPrivate.
func TestGetEventHiddenWhenReferencedCitationPrivate(t *testing.T) {
	id := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	p1 := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		events: map[models.ID]*models.Event{id: {
			ID: id, Private: false,
			Participants: []models.EventParticipant{{PersonID: p1}},
			Sources:      []models.SourceLink{{CitationID: citation}},
		}},
		people:    map[models.ID]*models.Person{p1: {ID: p1, Private: false}},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	_, err := New(repo).GetEvent(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for public event referencing a private citation", err)
	}
}

// TestGetEventVisibleToFullAccessWhenReferencedCitationPrivate: то же
// событие, но для вызывающего с полным доступом — видно.
func TestGetEventVisibleToFullAccessWhenReferencedCitationPrivate(t *testing.T) {
	id := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	p1 := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		events: map[models.ID]*models.Event{id: {
			ID: id, Private: false,
			Participants: []models.EventParticipant{{PersonID: p1}},
			Sources:      []models.SourceLink{{CitationID: citation}},
		}},
		people:    map[models.ID]*models.Person{p1: {ID: p1, Private: false}},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	got, err := New(repo).GetEvent(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}

	if got.ID != id {
		t.Fatalf("got = %+v", got)
	}
}

// TestGetEventNotHiddenWhenReferencedCitationPublic: ссылка на цитату есть,
// но цитата публична — событие не прячется ошибочно (защита от false
// positive).
func TestGetEventNotHiddenWhenReferencedCitationPublic(t *testing.T) {
	id := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	p1 := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		events: map[models.ID]*models.Event{id: {
			ID: id, Private: false,
			Participants: []models.EventParticipant{{PersonID: p1}},
			Sources:      []models.SourceLink{{CitationID: citation}},
		}},
		people:    map[models.ID]*models.Person{p1: {ID: p1, Private: false}},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: false}},
	}

	got, err := New(repo).GetEvent(context.Background(), models.AccessPublic, id)
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}

	if got.ID != id {
		t.Fatalf("got = %+v", got)
	}
}
