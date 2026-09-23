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
	churches  map[models.ID]*models.Church
	citations map[models.ID]*models.Citation
	saved     []*models.Church
}

func newFakeTx(existing ...*models.Church) *fakeTx {
	tx := &fakeTx{churches: map[models.ID]*models.Church{}, citations: map[models.ID]*models.Citation{}}
	for _, s := range existing {
		tx.churches[s.ID] = s
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

// TestUpdateChurchSourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestUpdateChurchSourceCitationNotFound(t *testing.T) {
	existing := church("CH-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Никольская церковь")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Sources = []models.SourceLink{{CitationID: cID('0')}}

	err := New(st).UpdateChurch(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d записей при несуществующей цитате", len(st.tx.saved))
	}
}
