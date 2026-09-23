package update_parish

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
	parishes map[models.ID]*models.Parish
	saved    []*models.Parish
}

func newFakeTx(existing ...*models.Parish) *fakeTx {
	tx := &fakeTx{parishes: map[models.ID]*models.Parish{}}
	for _, s := range existing {
		tx.parishes[s.ID] = s
	}

	return tx
}

func (f *fakeTx) GetParish(_ context.Context, id models.ID) (*models.Parish, error) {
	s, ok := f.parishes[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *s

	return &cp, nil
}

func (f *fakeTx) SaveParish(_ context.Context, s *models.Parish) error {
	cp := *s
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует ParishStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func parish(id models.ID, name string) *models.Parish {
	return &models.Parish{ID: id, Name: name}
}

func TestUpdateParishSaves(t *testing.T) {
	existing := parish("PR-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Никольский приход")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Name = "Никольский приход (испр.)"

	if err := New(st).UpdateParish(context.Background(), updated); err != nil {
		t.Fatalf("UpdateParish: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Name != "Никольский приход (испр.)" {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateParishNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateParish(context.Background(), *parish("PR-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Никольский приход"))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateParishRejectsEmptyName(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateParish(context.Background(), models.Parish{ID: "PR-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, want ValidationError on name", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}
