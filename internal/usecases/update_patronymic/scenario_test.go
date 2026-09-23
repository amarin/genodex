package update_patronymic

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
	patronymics map[models.ID]*models.Patronymic
	saved       []*models.Patronymic
}

func newFakeTx(existing ...*models.Patronymic) *fakeTx {
	tx := &fakeTx{patronymics: map[models.ID]*models.Patronymic{}}
	for _, s := range existing {
		tx.patronymics[s.ID] = s
	}

	return tx
}

func (f *fakeTx) GetPatronymic(_ context.Context, id models.ID) (*models.Patronymic, error) {
	s, ok := f.patronymics[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *s

	return &cp, nil
}

func (f *fakeTx) SavePatronymic(_ context.Context, s *models.Patronymic) error {
	cp := *s
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует PatronymicStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func patronymic(id models.ID, canonical string) *models.Patronymic {
	return &models.Patronymic{ID: id, Canonical: canonical}
}

func TestUpdatePatronymicSaves(t *testing.T) {
	existing := patronymic("PN-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Иванов")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Canonical = "Иванов (испр.)"

	if err := New(st).UpdatePatronymic(context.Background(), updated); err != nil {
		t.Fatalf("UpdatePatronymic: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Canonical != "Иванов (испр.)" {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdatePatronymicNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdatePatronymic(context.Background(), *patronymic("PN-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Иванов"))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdatePatronymicRejectsEmptyCanonical(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdatePatronymic(context.Background(), models.Patronymic{ID: "PN-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "canonical" {
		t.Fatalf("err = %v, want ValidationError on canonical", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}
