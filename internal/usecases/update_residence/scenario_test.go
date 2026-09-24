package update_residence

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

func pID(last byte) models.ID { return models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }
func dID(last byte) models.ID { return models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }
func cID(last byte) models.ID { return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }

type fakeTx struct {
	store.Store
	residences map[models.ID]*models.Residence
	people     map[models.ID]*models.Person
	places     map[models.ID]*models.AdministrativeDivision
	citations  map[models.ID]*models.Citation
	saved      []*models.Residence
}

func newFakeTx(existing *models.Residence, people, places []models.ID) *fakeTx {
	tx := &fakeTx{
		residences: map[models.ID]*models.Residence{}, people: map[models.ID]*models.Person{},
		places: map[models.ID]*models.AdministrativeDivision{}, citations: map[models.ID]*models.Citation{},
	}
	if existing != nil {
		tx.residences[existing.ID] = existing
	}
	for _, id := range people {
		tx.people[id] = &models.Person{ID: id}
	}
	for _, id := range places {
		tx.places[id] = &models.AdministrativeDivision{ID: id}
	}

	return tx
}

func (f *fakeTx) GetResidence(_ context.Context, id models.ID) (*models.Residence, error) {
	r, ok := f.residences[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *r

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

func (f *fakeTx) GetAdministrativeDivision(_ context.Context, id models.ID) (*models.AdministrativeDivision, error) {
	d, ok := f.places[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *d

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

func (f *fakeTx) SaveResidence(_ context.Context, r *models.Residence) error {
	cp := *r
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

func residence(id, person, place models.ID) *models.Residence {
	return &models.Residence{ID: id, PersonID: person, PlaceID: place}
}

func TestUpdateResidenceSaves(t *testing.T) {
	existing := residence("RS-01ARZ3NDEKTSV4RRFFQ69G5FA1", pID('1'), dID('1'))
	st := &fakeStore{tx: newFakeTx(existing, []models.ID{pID('1')}, []models.ID{dID('1')})}

	updated := *existing
	updated.Note = "переезд"

	if err := New(st).UpdateResidence(context.Background(), updated); err != nil {
		t.Fatalf("UpdateResidence: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Note != "переезд" {
		t.Fatalf("calls=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateResidenceNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx(nil, []models.ID{pID('1')}, []models.ID{dID('1')})}

	err := New(st).UpdateResidence(context.Background(), *residence("RS-01ARZ3NDEKTSV4RRFFQ69G5FA1", pID('1'), dID('1')))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateResidencePersonNotFound(t *testing.T) {
	existing := residence("RS-01ARZ3NDEKTSV4RRFFQ69G5FA1", pID('1'), dID('1'))
	st := &fakeStore{tx: newFakeTx(existing, nil, []models.ID{dID('1')})}

	err := New(st).UpdateResidence(context.Background(), *existing)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "person_id" {
		t.Fatalf("err = %v, want ValidationError on person_id", err)
	}
}

func TestUpdateResidencePlaceNotFound(t *testing.T) {
	existing := residence("RS-01ARZ3NDEKTSV4RRFFQ69G5FA1", pID('1'), dID('1'))
	st := &fakeStore{tx: newFakeTx(existing, []models.ID{pID('1')}, nil)}

	err := New(st).UpdateResidence(context.Background(), *existing)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "place_id" {
		t.Fatalf("err = %v, want ValidationError on place_id", err)
	}
}

func TestUpdateResidenceSourceCitationNotFound(t *testing.T) {
	existing := residence("RS-01ARZ3NDEKTSV4RRFFQ69G5FA1", pID('1'), dID('1'))
	st := &fakeStore{tx: newFakeTx(existing, []models.ID{pID('1')}, []models.ID{dID('1')})}

	updated := *existing
	updated.Sources = []models.SourceLink{{CitationID: cID('0')}}

	err := New(st).UpdateResidence(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, want ValidationError on sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d записей при несуществующей цитате", len(st.tx.saved))
	}
}
