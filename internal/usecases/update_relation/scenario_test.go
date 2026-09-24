package update_relation

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

func pID(last byte) models.ID { return models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }
func cID(last byte) models.ID { return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }

// fakeTx реализует нужные сценарию методы store.Store поверх карт записей;
// остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	relations map[models.ID]*models.Relation
	people    map[models.ID]*models.Person
	citations map[models.ID]*models.Citation
	saved     []*models.Relation
}

func newFakeTx(existing *models.Relation, people ...models.ID) *fakeTx {
	tx := &fakeTx{relations: map[models.ID]*models.Relation{}, people: map[models.ID]*models.Person{}, citations: map[models.ID]*models.Citation{}}
	if existing != nil {
		tx.relations[existing.ID] = existing
	}
	for _, id := range people {
		tx.people[id] = &models.Person{ID: id}
	}

	return tx
}

func (f *fakeTx) GetRelation(_ context.Context, id models.ID) (*models.Relation, error) {
	r, ok := f.relations[id]
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

func (f *fakeTx) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *c

	return &cp, nil
}

func (f *fakeTx) SaveRelation(_ context.Context, r *models.Relation) error {
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

func relation(id models.ID, a, b models.ID) *models.Relation {
	return &models.Relation{ID: id, Kind: models.RelationKindBlood, PersonA: a, PersonB: b}
}

func TestUpdateRelationSaves(t *testing.T) {
	existing := relation("RL-01ARZ3NDEKTSV4RRFFQ69G5FA1", pID('1'), pID('2'))
	st := &fakeStore{tx: newFakeTx(existing, pID('1'), pID('2'))}

	updated := *existing
	updated.Private = true

	if err := New(st).UpdateRelation(context.Background(), updated); err != nil {
		t.Fatalf("UpdateRelation: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || !st.tx.saved[0].Private {
		t.Fatalf("calls=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateRelationNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx(nil, pID('1'), pID('2'))}

	err := New(st).UpdateRelation(context.Background(), *relation("RL-01ARZ3NDEKTSV4RRFFQ69G5FA1", pID('1'), pID('2')))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateRelationPersonANotFound(t *testing.T) {
	existing := relation("RL-01ARZ3NDEKTSV4RRFFQ69G5FA1", pID('1'), pID('2'))
	st := &fakeStore{tx: newFakeTx(existing, pID('2'))}

	err := New(st).UpdateRelation(context.Background(), *existing)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "person_a" {
		t.Fatalf("err = %v, want ValidationError on person_a", err)
	}
}

func TestUpdateRelationPersonBNotFound(t *testing.T) {
	existing := relation("RL-01ARZ3NDEKTSV4RRFFQ69G5FA1", pID('1'), pID('2'))
	st := &fakeStore{tx: newFakeTx(existing, pID('1'))}

	err := New(st).UpdateRelation(context.Background(), *existing)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "person_b" {
		t.Fatalf("err = %v, want ValidationError on person_b", err)
	}
}

func TestUpdateRelationSourceCitationNotFound(t *testing.T) {
	existing := relation("RL-01ARZ3NDEKTSV4RRFFQ69G5FA1", pID('1'), pID('2'))
	st := &fakeStore{tx: newFakeTx(existing, pID('1'), pID('2'))}

	updated := *existing
	updated.Sources = []models.SourceLink{{CitationID: cID('0')}}

	err := New(st).UpdateRelation(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, want ValidationError on sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d записей при несуществующей цитате", len(st.tx.saved))
	}
}

func TestUpdateRelationInvalidRejectedBeforeInTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx(nil)}

	err := New(st).UpdateRelation(context.Background(), models.Relation{ID: "RL-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}
