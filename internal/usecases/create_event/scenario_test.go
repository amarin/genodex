package create_event

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

func pID(last byte) models.ID { return models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }
func cID(last byte) models.ID { return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }

type stubIDs struct{ id models.ID }

func (s *stubIDs) New(models.Type) models.ID { return s.id }

// fakeTx реализует нужные сценарию методы store.Store поверх карт персон и
// цитат; остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	people    map[models.ID]*models.Person
	citations map[models.ID]*models.Citation
	saved     *models.Event
	saveErr   error
}

func newFakeTx(people []models.ID, citations ...*models.Citation) *fakeTx {
	tx := &fakeTx{people: map[models.ID]*models.Person{}, citations: map[models.ID]*models.Citation{}}
	for _, id := range people {
		tx.people[id] = &models.Person{ID: id}
	}
	for _, c := range citations {
		tx.citations[c.ID] = c
	}

	return tx
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
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *e
	f.saved = &cp

	return nil
}

type fakeStore struct{ tx *fakeTx }

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	return fn(f.tx)
}

func TestCreateEventGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "E-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{tx: newFakeTx(nil)}
	sc := New(st, ids)

	got, err := sc.CreateEvent(context.Background(), models.Event{Type: models.EventTypeBirth})
	if err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.tx.saved == nil || st.tx.saved.Type != models.EventTypeBirth {
		t.Fatalf("saved = %+v", st.tx.saved)
	}
}

func TestCreateEventRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx(nil)}, &stubIDs{})

	_, err := sc.CreateEvent(context.Background(), models.Event{ID: "E-01ARZ3NDEKTSV4RRFFQ69G5FA1", Type: models.EventTypeBirth})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

// TestCreateEventPlaceSoftRefNotChecked: Place — мягкая ссылка (PlaceRef),
// НИКОГДА не проверяется на существование при сохранении — даже
// фабрикованный, заведомо несуществующий id проходит, поскольку модельная
// Validate проверяет только формат/тип-ограничение (в отличие от
// participants[i].person_id — строгой ссылки, см.
// TestCreateEventParticipantNotFound).
func TestCreateEventPlaceSoftRefNotChecked(t *testing.T) {
	ids := &stubIDs{id: "E-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{tx: newFakeTx(nil)} // ни одной персоны в хранилище
	sc := New(st, ids)

	in := models.Event{
		Type: models.EventTypeBirth,
		Place: &models.PlaceRef{
			Text: "село Давыдово", Ref: "AD-01ARZ3NDEKTSV4RRFFQ69G5FA9", Type: models.TypeAdministrativeDivision,
		},
	}

	got, err := sc.CreateEvent(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateEvent: %v, ожидался успех (Place — мягкая ссылка)", err)
	}

	if got.Place == nil || got.Place.Ref != "AD-01ARZ3NDEKTSV4RRFFQ69G5FA9" {
		t.Fatalf("Place = %+v", got.Place)
	}
}

// TestCreateEventParticipantNotFound: несуществующий
// participants[i].person_id — 422 на поле participants[i].person_id,
// ничего не сохраняется (в отличие от Place — строгая ссылка).
func TestCreateEventParticipantNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx([]models.ID{pID('1')})} // только первая персона (индекс 0) существует
	sc := New(st, &stubIDs{id: "E-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	in := models.Event{
		Type: models.EventTypeBirth,
		Participants: []models.EventParticipant{
			{PersonID: pID('1'), Role: "родитель"},
			{PersonID: pID('9'), Role: "свидетель"}, // не существует
		},
	}

	_, err := sc.CreateEvent(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "participants[1].person_id" {
		t.Fatalf("err = %v, want ValidationError on participants[1].person_id (индексированная ошибка на позиции несуществующего участника)", err)
	}

	if st.tx.saved != nil {
		t.Fatalf("saved = %+v; ожидалось: ничего не сохранено", st.tx.saved)
	}
}

func TestCreateEventSourceCitationNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx(nil)}
	sc := New(st, &stubIDs{id: "E-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	in := models.Event{Type: models.EventTypeBirth, Sources: []models.SourceLink{{CitationID: cID('0')}}}

	_, err := sc.CreateEvent(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, want ValidationError on sources[0].citation_id", err)
	}
}

func TestCreateEventRejectsInvalidType(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx(nil)}, &stubIDs{id: "E-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateEvent(context.Background(), models.Event{Type: "Not Valid!"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "type" {
		t.Fatalf("err = %v, want ValidationError on type", err)
	}
}
