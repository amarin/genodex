package create_person

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// cID возвращает корректный идентификатор цитаты, отличающийся последним символом.
func cID(last byte) models.ID {
	return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

type stubIDs struct{ id models.ID }

func (s *stubIDs) New(models.Type) models.ID { return s.id }

// fakeTx реализует нужные сценарию методы store.Store поверх карты цитат;
// остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	citations map[models.ID]*models.Citation
	saved     *models.Person
	saveErr   error
}

func newFakeTx(existingCitations ...*models.Citation) *fakeTx {
	tx := &fakeTx{citations: map[models.ID]*models.Citation{}}
	for _, c := range existingCitations {
		tx.citations[c.ID] = c
	}

	return tx
}

func (f *fakeTx) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *c

	return &cp, nil
}

func (f *fakeTx) SavePerson(_ context.Context, p *models.Person) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *p
	f.saved = &cp

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

func TestCreatePersonGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "I-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{tx: newFakeTx()}
	sc := New(st, ids)

	in := models.Person{Gender: models.PersonGenderFemale, Names: []models.PersonName{
		{Given: models.TextRef{Text: "Акилина"}},
	}}

	got, err := sc.CreatePerson(context.Background(), in)
	if err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.tx.saved == nil || st.tx.saved.Gender != models.PersonGenderFemale {
		t.Fatalf("saved = %+v", st.tx.saved)
	}
}

func TestCreatePersonRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx()}, &stubIDs{})

	_, err := sc.CreatePerson(context.Background(), models.Person{ID: "I-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestCreatePersonRejectsBadGender(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx()}, &stubIDs{id: "I-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreatePerson(context.Background(), models.Person{Gender: "x"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "gender" {
		t.Fatalf("err = %v, want ValidationError on gender", err)
	}
}

func TestCreatePersonAllFieldsOptional(t *testing.T) {
	ids := &stubIDs{id: "I-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{tx: newFakeTx()}
	sc := New(st, ids)

	got, err := sc.CreatePerson(context.Background(), models.Person{})
	if err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q", got.ID)
	}
}

func TestCreatePersonPropagatesStoreError(t *testing.T) {
	wantErr := errors.New("db down")
	st := &fakeStore{tx: newFakeTx()}
	st.tx.saveErr = wantErr
	sc := New(st, &stubIDs{id: "I-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreatePerson(context.Background(), models.Person{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

// TestCreatePersonSourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestCreatePersonSourceCitationNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	sc := New(st, &stubIDs{id: "I-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	in := models.Person{Sources: []models.SourceLink{{CitationID: cID('0')}}}

	_, err := sc.CreatePerson(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if st.tx.saved != nil {
		t.Fatalf("saved = %+v; ожидалось: ничего не сохранено", st.tx.saved)
	}
}

// TestCreatePersonNamesSoftRefNotChecked: Names[i].Surname/.Given/.Patronymic
// — мягкие ссылки на Surname/GivenName/Patronymic (все три имеют CRUD с
// подпроектов 1-2), TextRef.Ref не проверяется на существование при
// сохранении — только формат (тот же принцип, что у Family.Members на
// Person, см. TestCreateFamilyMembersSoftRefNotChecked). Ссылки указывают на
// заведомо несуществующие записи словарей — сохранение всё равно проходит.
func TestCreatePersonNamesSoftRefNotChecked(t *testing.T) {
	ids := &stubIDs{id: "I-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{tx: newFakeTx()}
	sc := New(st, ids)

	in := models.Person{
		Names: []models.PersonName{{
			Type:       models.PersonNameMain,
			Surname:    models.TextRef{Text: "Дорожкина", Ref: "SN-01ARZ3NDEKTSV4RRFFQ69G5FA1", Type: models.TypeSurname},
			Given:      models.TextRef{Text: "Акилина", Ref: "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1", Type: models.TypeGivenName},
			Patronymic: models.TextRef{Text: "Ивановна", Ref: "PN-01ARZ3NDEKTSV4RRFFQ69G5FA1", Type: models.TypePatronymic},
		}},
	}

	got, err := sc.CreatePerson(context.Background(), in)
	if err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}

	if len(got.Names) != 1 ||
		got.Names[0].Surname.Ref != "SN-01ARZ3NDEKTSV4RRFFQ69G5FA1" ||
		got.Names[0].Given.Ref != "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1" ||
		got.Names[0].Patronymic.Ref != "PN-01ARZ3NDEKTSV4RRFFQ69G5FA1" {
		t.Fatalf("Names = %+v", got.Names)
	}

	if st.tx.saved == nil {
		t.Fatalf("saved = nil, want сохранённая запись")
	}
}
