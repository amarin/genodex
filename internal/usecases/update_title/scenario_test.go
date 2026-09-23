package update_title

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
	titles map[models.ID]*models.Title
	saved  []*models.Title
}

func newFakeTx(existing ...*models.Title) *fakeTx {
	tx := &fakeTx{titles: map[models.ID]*models.Title{}}
	for _, s := range existing {
		tx.titles[s.ID] = s
	}

	return tx
}

func (f *fakeTx) GetTitle(_ context.Context, id models.ID) (*models.Title, error) {
	s, ok := f.titles[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *s

	return &cp, nil
}

func (f *fakeTx) SaveTitle(_ context.Context, s *models.Title) error {
	cp := *s
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует TitleStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func title(id models.ID, canonical string) *models.Title {
	return &models.Title{ID: id, Canonical: canonical}
}

func TestUpdateTitleSaves(t *testing.T) {
	existing := title("TT-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Иванов")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Canonical = "Иванов (испр.)"

	if err := New(st).UpdateTitle(context.Background(), updated); err != nil {
		t.Fatalf("UpdateTitle: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Canonical != "Иванов (испр.)" {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateTitleNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateTitle(context.Background(), *title("TT-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Иванов"))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateTitleRejectsEmptyCanonical(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateTitle(context.Background(), models.Title{ID: "TT-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "canonical" {
		t.Fatalf("err = %v, want ValidationError on canonical", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}
