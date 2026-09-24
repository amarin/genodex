package update_event

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

func pID(last byte) models.ID { return models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }
func cID(last byte) models.ID { return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }

type fakeTx struct {
	store.Store
	events    map[models.ID]*models.Event
	people    map[models.ID]*models.Person
	citations map[models.ID]*models.Citation
	saved     []*models.Event
}

func newFakeTx(existing *models.Event, people []models.ID) *fakeTx {
	tx := &fakeTx{events: map[models.ID]*models.Event{}, people: map[models.ID]*models.Person{}, citations: map[models.ID]*models.Citation{}}
	if existing != nil {
		tx.events[existing.ID] = existing
	}
	for _, id := range people {
		tx.people[id] = &models.Person{ID: id}
	}

	return tx
}

func (f *fakeTx) GetEvent(_ context.Context, id models.ID) (*models.Event, error) {
	e, ok := f.events[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *e

	return &cp, nil
}

func (f *fakeTx) GetPerson(_ context.Context, id models.ID) (*models.Person, error) {
	p, ok := f.people[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *p

	return &cp, nil
}

func (f *fakeTx) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *c

	return &cp, nil
}

func (f *fakeTx) SaveEvent(_ context.Context, e *models.Event) error {
	cp := *e
	f.saved = append(f.saved, &cp)

	return nil
}

type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func event(id models.ID) *models.Event {
	return &models.Event{ID: id, Type: models.EventTypeBirth}
}

func TestUpdateEventSaves(t *testing.T) {
	existing := event("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	st := &fakeStore{tx: newFakeTx(existing, nil)}

	updated := *existing
	updated.Private = true

	if err := New(st).UpdateEvent(context.Background(), updated); err != nil {
		t.Fatalf("UpdateEvent: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || !st.tx.saved[0].Private {
		t.Fatalf("calls=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateEventNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx(nil, nil)}

	err := New(st).UpdateEvent(context.Background(), *event("E-01ARZ3NDEKTSV4RRFFQ69G5FA1"))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// TestUpdateEventPlaceSoftRefNotChecked: Place — мягкая ссылка, не
// проверяется на существование при обновлении, как и при создании.
func TestUpdateEventPlaceSoftRefNotChecked(t *testing.T) {
	existing := event("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	st := &fakeStore{tx: newFakeTx(existing, nil)}

	updated := *existing
	updated.Place = &models.PlaceRef{Text: "погост", Ref: "AD-01ARZ3NDEKTSV4RRFFQ69G5FA9", Type: models.TypeAdministrativeDivision}

	if err := New(st).UpdateEvent(context.Background(), updated); err != nil {
		t.Fatalf("UpdateEvent: %v, ожидался успех (Place — мягкая ссылка)", err)
	}

	if len(st.tx.saved) != 1 || st.tx.saved[0].Place == nil || st.tx.saved[0].Place.Ref != "AD-01ARZ3NDEKTSV4RRFFQ69G5FA9" {
		t.Fatalf("saved = %+v", st.tx.saved)
	}
}

// TestUpdateEventParticipantNotFound: несуществующий
// participants[i].person_id — 422 на индексированное поле, ничего не
// сохраняется.
func TestUpdateEventParticipantNotFound(t *testing.T) {
	existing := event("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	st := &fakeStore{tx: newFakeTx(existing, []models.ID{pID('1')})}

	updated := *existing
	updated.Participants = []models.EventParticipant{
		{PersonID: pID('1'), Role: "родитель"},
		{PersonID: pID('9'), Role: "свидетель"},
	}

	err := New(st).UpdateEvent(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "participants[1].person_id" {
		t.Fatalf("err = %v, want ValidationError on participants[1].person_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d записей при несуществующем участнике", len(st.tx.saved))
	}
}

func TestUpdateEventSourceCitationNotFound(t *testing.T) {
	existing := event("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	st := &fakeStore{tx: newFakeTx(existing, nil)}

	updated := *existing
	updated.Sources = []models.SourceLink{{CitationID: cID('0')}}

	err := New(st).UpdateEvent(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, want ValidationError on sources[0].citation_id", err)
	}
}
