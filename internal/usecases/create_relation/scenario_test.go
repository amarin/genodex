package create_relation

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
	saved     *models.Relation
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

func (f *fakeTx) SaveRelation(_ context.Context, r *models.Relation) error {
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

func TestCreateRelationGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "RL-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{tx: newFakeTx([]models.ID{pID('1'), pID('2')})}
	sc := New(st, ids)

	got, err := sc.CreateRelation(context.Background(), models.Relation{
		Kind: models.RelationKindBlood, PersonA: pID('1'), PersonB: pID('2'),
	})
	if err != nil {
		t.Fatalf("CreateRelation: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.tx.saved == nil || st.tx.saved.PersonA != pID('1') || st.tx.saved.PersonB != pID('2') {
		t.Fatalf("saved = %+v", st.tx.saved)
	}
}

func TestCreateRelationRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx(nil)}, &stubIDs{})

	_, err := sc.CreateRelation(context.Background(), models.Relation{
		ID: "RL-01ARZ3NDEKTSV4RRFFQ69G5FA1", Kind: models.RelationKindBlood, PersonA: pID('1'), PersonB: pID('2'),
	})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestCreateRelationRejectsSamePerson(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx(nil)}, &stubIDs{id: "RL-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateRelation(context.Background(), models.Relation{
		Kind: models.RelationKindBlood, PersonA: pID('1'), PersonB: pID('1'),
	})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "person_b" {
		t.Fatalf("err = %v, want ValidationError on person_b (модельная валидация, до InTx)", err)
	}
}

// TestCreateRelationPersonANotFound: несуществующий person_a — 422 на поле
// person_a, ничего не сохраняется.
func TestCreateRelationPersonANotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx([]models.ID{pID('2')})}
	sc := New(st, &stubIDs{id: "RL-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateRelation(context.Background(), models.Relation{
		Kind: models.RelationKindBlood, PersonA: pID('1'), PersonB: pID('2'),
	})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "person_a" {
		t.Fatalf("err = %v, want ValidationError on person_a", err)
	}

	if st.tx.saved != nil {
		t.Fatalf("saved = %+v; ожидалось: ничего не сохранено", st.tx.saved)
	}
}

// TestCreateRelationPersonBNotFound: person_a существует, но person_b — нет
// — 422 на поле person_b (обе стороны проверяются независимо).
func TestCreateRelationPersonBNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx([]models.ID{pID('1')})}
	sc := New(st, &stubIDs{id: "RL-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateRelation(context.Background(), models.Relation{
		Kind: models.RelationKindBlood, PersonA: pID('1'), PersonB: pID('2'),
	})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "person_b" {
		t.Fatalf("err = %v, want ValidationError on person_b", err)
	}
}

func TestCreateRelationSourceCitationNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx([]models.ID{pID('1'), pID('2')})}
	sc := New(st, &stubIDs{id: "RL-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	in := models.Relation{
		Kind: models.RelationKindBlood, PersonA: pID('1'), PersonB: pID('2'),
		Sources: []models.SourceLink{{CitationID: cID('0')}},
	}

	_, err := sc.CreateRelation(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, want ValidationError on sources[0].citation_id", err)
	}
}

func TestCreateRelationAssociateRequiresRelType(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx([]models.ID{pID('1'), pID('2')})}, &stubIDs{id: "RL-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateRelation(context.Background(), models.Relation{
		Kind: models.RelationKindAssociate, PersonA: pID('1'), PersonB: pID('2'),
	})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "rel_type" {
		t.Fatalf("err = %v, want ValidationError on rel_type (модельная валидация)", err)
	}
}
