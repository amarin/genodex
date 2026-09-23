package update_surname

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
	surnames map[models.ID]*models.Surname
	saved    []*models.Surname
}

func newFakeTx(existing ...*models.Surname) *fakeTx {
	tx := &fakeTx{surnames: map[models.ID]*models.Surname{}}
	for _, s := range existing {
		tx.surnames[s.ID] = s
	}

	return tx
}

func (f *fakeTx) GetSurname(_ context.Context, id models.ID) (*models.Surname, error) {
	s, ok := f.surnames[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *s

	return &cp, nil
}

func (f *fakeTx) SaveSurname(_ context.Context, s *models.Surname) error {
	cp := *s
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует SurnameStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func surname(id models.ID, canonical string) *models.Surname {
	return &models.Surname{ID: id, Canonical: canonical}
}

func TestUpdateSurnameSaves(t *testing.T) {
	existing := surname("SN-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Иванов")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Canonical = "Иванов (испр.)"

	if err := New(st).UpdateSurname(context.Background(), updated); err != nil {
		t.Fatalf("UpdateSurname: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Canonical != "Иванов (испр.)" {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateSurnameNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateSurname(context.Background(), *surname("SN-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Иванов"))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateSurnameRejectsEmptyCanonical(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateSurname(context.Background(), models.Surname{ID: "SN-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "canonical" {
		t.Fatalf("err = %v, want ValidationError on canonical", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}
