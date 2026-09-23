package update_family

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
	families  map[models.ID]*models.Family
	citations map[models.ID]*models.Citation
	saved     []*models.Family
}

func newFakeTx(existing ...*models.Family) *fakeTx {
	tx := &fakeTx{families: map[models.ID]*models.Family{}, citations: map[models.ID]*models.Citation{}}
	for _, s := range existing {
		tx.families[s.ID] = s
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

func (f *fakeTx) GetFamily(_ context.Context, id models.ID) (*models.Family, error) {
	s, ok := f.families[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *s

	return &cp, nil
}

func (f *fakeTx) SaveFamily(_ context.Context, s *models.Family) error {
	cp := *s
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует FamilyStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func family(id models.ID, name string) *models.Family {
	return &models.Family{ID: id, Name: name}
}

func TestUpdateFamilySaves(t *testing.T) {
	existing := family("F-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Ивановы")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Name = "Ивановы (испр.)"

	if err := New(st).UpdateFamily(context.Background(), updated); err != nil {
		t.Fatalf("UpdateFamily: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Name != "Ивановы (испр.)" {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateFamilyNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateFamily(context.Background(), *family("F-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Ивановы"))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateFamilyRejectsEmptyName(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateFamily(context.Background(), models.Family{ID: "F-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, want ValidationError on name", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

// TestUpdateFamilySourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestUpdateFamilySourceCitationNotFound(t *testing.T) {
	existing := family("F-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Ивановы")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Sources = []models.SourceLink{{CitationID: cID('0')}}

	err := New(st).UpdateFamily(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d записей при несуществующей цитате", len(st.tx.saved))
	}
}
