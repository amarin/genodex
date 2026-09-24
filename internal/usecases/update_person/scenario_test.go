package update_person

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// fakeTx реализует нужные сценарию методы store.Store поверх карты записей;
// остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	people    map[models.ID]*models.Person
	citations map[models.ID]*models.Citation
	saved     []*models.Person
}

func newFakeTx(existing ...*models.Person) *fakeTx {
	tx := &fakeTx{people: map[models.ID]*models.Person{}, citations: map[models.ID]*models.Citation{}}
	for _, p := range existing {
		tx.people[p.ID] = p
	}

	return tx
}

// cID возвращает корректный идентификатор цитаты.
func cID(last byte) models.ID {
	return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func (f *fakeTx) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *c

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

func (f *fakeTx) SavePerson(_ context.Context, p *models.Person) error {
	cp := *p
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует PersonStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func person(id models.ID) *models.Person {
	return &models.Person{ID: id}
}

func TestUpdatePersonSaves(t *testing.T) {
	existing := person("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Gender = models.PersonGenderMale

	if err := New(st).UpdatePerson(context.Background(), updated); err != nil {
		t.Fatalf("UpdatePerson: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Gender != models.PersonGenderMale {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdatePersonNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdatePerson(context.Background(), *person("I-01ARZ3NDEKTSV4RRFFQ69G5FA1"))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdatePersonRejectsBadGender(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdatePerson(context.Background(), models.Person{ID: "I-01ARZ3NDEKTSV4RRFFQ69G5FA1", Gender: "x"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "gender" {
		t.Fatalf("err = %v, want ValidationError on gender", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

// TestUpdatePersonSourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestUpdatePersonSourceCitationNotFound(t *testing.T) {
	existing := person("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Sources = []models.SourceLink{{CitationID: cID('0')}}

	err := New(st).UpdatePerson(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d записей при несуществующей цитате", len(st.tx.saved))
	}
}

// TestUpdatePersonNamesSoftRefNotChecked: как и при создании (см.
// create_person.TestCreatePersonNamesSoftRefNotChecked), Names[i].Surname/
// .Given/.Patronymic не проверяются на существование при обновлении.
func TestUpdatePersonNamesSoftRefNotChecked(t *testing.T) {
	existing := person("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Names = []models.PersonName{{
		Type:       models.PersonNameMarried,
		Surname:    models.TextRef{Text: "Петрова", Ref: "SN-01ARZ3NDEKTSV4RRFFQ69G5FA1", Type: models.TypeSurname},
		Given:      models.TextRef{Text: "Мария", Ref: "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1", Type: models.TypeGivenName},
		Patronymic: models.TextRef{Text: "Ивановна", Ref: "PN-01ARZ3NDEKTSV4RRFFQ69G5FA1", Type: models.TypePatronymic},
	}}

	if err := New(st).UpdatePerson(context.Background(), updated); err != nil {
		t.Fatalf("UpdatePerson: %v", err)
	}

	if len(st.tx.saved) != 1 || st.tx.saved[0].Names[0].Surname.Ref != "SN-01ARZ3NDEKTSV4RRFFQ69G5FA1" {
		t.Fatalf("saved = %+v", st.tx.saved)
	}
}
