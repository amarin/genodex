package update_given_name

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
	givenNames map[models.ID]*models.GivenName
	saved      []*models.GivenName
}

func newFakeTx(existing ...*models.GivenName) *fakeTx {
	tx := &fakeTx{givenNames: map[models.ID]*models.GivenName{}}
	for _, s := range existing {
		tx.givenNames[s.ID] = s
	}

	return tx
}

func (f *fakeTx) GetGivenName(_ context.Context, id models.ID) (*models.GivenName, error) {
	s, ok := f.givenNames[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *s

	return &cp, nil
}

func (f *fakeTx) SaveGivenName(_ context.Context, s *models.GivenName) error {
	cp := *s
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует GivenNameStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func givenName(id models.ID, canonical string) *models.GivenName {
	return &models.GivenName{ID: id, Canonical: canonical, Gender: models.NameGenderMale}
}

func TestUpdateGivenNameSaves(t *testing.T) {
	existing := givenName("GN-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Иванов")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Canonical = "Иванов (испр.)"

	if err := New(st).UpdateGivenName(context.Background(), updated); err != nil {
		t.Fatalf("UpdateGivenName: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Canonical != "Иванов (испр.)" {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateGivenNameNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateGivenName(context.Background(), *givenName("GN-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Иванов"))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateGivenNameRejectsEmptyCanonical(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateGivenName(context.Background(), models.GivenName{ID: "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1", Gender: models.NameGenderMale})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "canonical" {
		t.Fatalf("err = %v, want ValidationError on canonical", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

// TestUpdateGivenNameRejectsInvalidGender — единственное отличие GivenName от
// остальных словарей, см. create_given_name's аналогичный тест.
func TestUpdateGivenNameRejectsInvalidGender(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateGivenName(context.Background(), models.GivenName{ID: "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1", Canonical: "Иван"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "gender" {
		t.Fatalf("err = %v, want ValidationError on gender", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}
