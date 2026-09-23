package update_church

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
	churches map[models.ID]*models.Church
	saved    []*models.Church
}

func newFakeTx(existing ...*models.Church) *fakeTx {
	tx := &fakeTx{churches: map[models.ID]*models.Church{}}
	for _, s := range existing {
		tx.churches[s.ID] = s
	}

	return tx
}

func (f *fakeTx) GetChurch(_ context.Context, id models.ID) (*models.Church, error) {
	s, ok := f.churches[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *s

	return &cp, nil
}

func (f *fakeTx) SaveChurch(_ context.Context, s *models.Church) error {
	cp := *s
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует ChurchStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func church(id models.ID, name string) *models.Church {
	return &models.Church{ID: id, Name: name}
}

func TestUpdateChurchSaves(t *testing.T) {
	existing := church("CH-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Никольская церковь")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Name = "Никольская церковь (испр.)"

	if err := New(st).UpdateChurch(context.Background(), updated); err != nil {
		t.Fatalf("UpdateChurch: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Name != "Никольская церковь (испр.)" {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateChurchNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateChurch(context.Background(), *church("CH-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Никольская церковь"))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateChurchRejectsEmptyName(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateChurch(context.Background(), models.Church{ID: "CH-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, want ValidationError on name", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}
