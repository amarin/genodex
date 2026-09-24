package create_residence

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

type stubIDs struct{ id models.ID }

func (s *stubIDs) New(models.Type) models.ID { return s.id }

// fakeTx реализует нужные сценарию методы store.Store поверх карт персон,
// мест и цитат; остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	people    map[models.ID]*models.Person
	places    map[models.ID]*models.AdministrativeDivision
	citations map[models.ID]*models.Citation
	saved     *models.Residence
	saveErr   error
}

func newFakeTx(people, places []models.ID, citations ...*models.Citation) *fakeTx {
	tx := &fakeTx{
		people: map[models.ID]*models.Person{}, places: map[models.ID]*models.AdministrativeDivision{},
		citations: map[models.ID]*models.Citation{},
	}
	for _, id := range people {
		tx.people[id] = &models.Person{ID: id}
	}
	for _, id := range places {
		tx.places[id] = &models.AdministrativeDivision{ID: id}
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
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *r
	f.saved = &cp

	return nil
}

type fakeStore struct{ tx *fakeTx }

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	return fn(f.tx)
}

func TestCreateResidenceGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "RS-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{tx: newFakeTx([]models.ID{pID('1')}, []models.ID{dID('1')})}
	sc := New(st, ids)

	got, err := sc.CreateResidence(context.Background(), models.Residence{PersonID: pID('1'), PlaceID: dID('1')})
	if err != nil {
		t.Fatalf("CreateResidence: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.tx.saved == nil || st.tx.saved.PersonID != pID('1') || st.tx.saved.PlaceID != dID('1') {
		t.Fatalf("saved = %+v", st.tx.saved)
	}
}

func TestCreateResidenceRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx(nil, nil)}, &stubIDs{})

	_, err := sc.CreateResidence(context.Background(), models.Residence{
		ID: "RS-01ARZ3NDEKTSV4RRFFQ69G5FA1", PersonID: pID('1'), PlaceID: dID('1'),
	})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

// TestCreateResidencePersonNotFound: несуществующий person_id — 422 на поле
// person_id, ничего не сохраняется.
func TestCreateResidencePersonNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx(nil, []models.ID{dID('1')})}
	sc := New(st, &stubIDs{id: "RS-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateResidence(context.Background(), models.Residence{PersonID: pID('1'), PlaceID: dID('1')})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "person_id" {
		t.Fatalf("err = %v, want ValidationError on person_id", err)
	}

	if st.tx.saved != nil {
		t.Fatalf("saved = %+v; ожидалось: ничего не сохранено", st.tx.saved)
	}
}

// TestCreateResidencePlaceNotFound: person_id существует, но place_id — нет
// — 422 на поле place_id.
func TestCreateResidencePlaceNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx([]models.ID{pID('1')}, nil)}
	sc := New(st, &stubIDs{id: "RS-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateResidence(context.Background(), models.Residence{PersonID: pID('1'), PlaceID: dID('1')})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "place_id" {
		t.Fatalf("err = %v, want ValidationError on place_id", err)
	}
}

func TestCreateResidenceSourceCitationNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx([]models.ID{pID('1')}, []models.ID{dID('1')})}
	sc := New(st, &stubIDs{id: "RS-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	in := models.Residence{PersonID: pID('1'), PlaceID: dID('1'), Sources: []models.SourceLink{{CitationID: cID('0')}}}

	_, err := sc.CreateResidence(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, want ValidationError on sources[0].citation_id", err)
	}
}
