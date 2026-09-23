package update_note

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// nID возвращает корректный идентификатор заметки, отличающийся последним символом.
func nID(last byte) models.ID {
	return models.ID("N-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// fakeTx реализует нужные сценарию методы store.Store поверх карты заметок;
// остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	notes   map[models.ID]*models.Note
	saved   []*models.Note
	saveErr error
	gets    int
}

func newFakeTx(existing ...*models.Note) *fakeTx {
	tx := &fakeTx{notes: map[models.ID]*models.Note{}}
	for _, n := range existing {
		tx.notes[n.ID] = n
	}

	return tx
}

func (f *fakeTx) GetNote(_ context.Context, id models.ID) (*models.Note, error) {
	f.gets++

	n, ok := f.notes[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *n

	return &cp, nil
}

func (f *fakeTx) SaveNote(_ context.Context, n *models.Note) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *n
	f.notes[n.ID] = &cp
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует NoteStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func note(id models.ID, parent *models.ID) *models.Note {
	return &models.Note{ID: id, Kind: "note", Text: "текст " + string(id), ParentID: parent}
}

func TestUpdateNoteSaves(t *testing.T) {
	existing := note(nID('V'), nil)
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Text = "новый текст"

	if err := New(st).UpdateNote(context.Background(), updated); err != nil {
		t.Fatalf("UpdateNote: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Text != "новый текст" {
		t.Fatalf("InTx=%d, saved=%v; ожидалось одно сохранение с новым текстом", st.calls, st.tx.saved)
	}
}

func TestUpdateNoteNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateNote(context.Background(), *note(nID('V'), nil))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d заметок при несуществующей заметке", len(st.tx.saved))
	}
}

func TestUpdateNoteValidatesBeforeTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	bad := *note(nID('V'), nil)
	bad.Text = ""
	bad.Title = ""

	err := New(st).UpdateNote(context.Background(), bad)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "text" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю text", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

// TestUpdateNoteParentNotFound: новый родитель не существует —
// *ValidationError по полю parent_id, ничего не сохраняется.
func TestUpdateNoteParentNotFound(t *testing.T) {
	existing := note(nID('V'), nil)
	st := &fakeStore{tx: newFakeTx(existing)}

	missing := nID('0')
	updated := *existing
	updated.ParentID = &missing

	err := New(st).UpdateNote(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d заметок при несуществующем родителе", len(st.tx.saved))
	}
}

// TestUpdateNoteDirectCycle: B — дочка A; попытка сделать A дочкой B — цикл,
// *ValidationError по полю parent_id.
func TestUpdateNoteDirectCycle(t *testing.T) {
	idA, idB := nID('A'), nID('B')
	a := note(idA, nil)
	b := note(idB, &idA)
	st := &fakeStore{tx: newFakeTx(a, b)}

	updated := *a
	updated.ParentID = &idB

	err := New(st).UpdateNote(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id (цикл)", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d заметок при цикле", len(st.tx.saved))
	}
}

// TestUpdateNoteLongCycle: цепочка C → B → A; попытка сделать A дочкой C —
// цикл длиной три, *ValidationError по полю parent_id.
func TestUpdateNoteLongCycle(t *testing.T) {
	idA, idB, idC := nID('A'), nID('B'), nID('C')
	a := note(idA, nil)
	b := note(idB, &idA)
	c := note(idC, &idB)
	st := &fakeStore{tx: newFakeTx(a, b, c)}

	updated := *a
	updated.ParentID = &idC

	err := New(st).UpdateNote(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id (цикл)", err)
	}
}

// TestUpdateNoteReparentOK: перенос под другого корректного родителя проходит.
func TestUpdateNoteReparentOK(t *testing.T) {
	idA, idB, idC := nID('A'), nID('B'), nID('C')
	a := note(idA, nil)
	b := note(idB, &idA)
	c := note(idC, nil)
	st := &fakeStore{tx: newFakeTx(a, b, c)}

	updated := *b
	updated.ParentID = &idC

	if err := New(st).UpdateNote(context.Background(), updated); err != nil {
		t.Fatalf("UpdateNote: %v", err)
	}

	if len(st.tx.saved) != 1 || *st.tx.saved[0].ParentID != idC {
		t.Fatalf("saved=%v; ожидалось сохранение с родителем %v", st.tx.saved, idC)
	}
}

// TestUpdateNoteWithoutParentSkipsWalk: без родителя цепочка не обходится —
// одно чтение (существование самой заметки).
func TestUpdateNoteWithoutParentSkipsWalk(t *testing.T) {
	existing := note(nID('V'), nil)
	st := &fakeStore{tx: newFakeTx(existing)}

	if err := New(st).UpdateNote(context.Background(), *existing); err != nil {
		t.Fatalf("UpdateNote: %v", err)
	}

	if st.tx.gets != 1 {
		t.Fatalf("чтений %d, ожидалось одно (без обхода родителей)", st.tx.gets)
	}
}

func TestUpdateNotePropagatesSaveError(t *testing.T) {
	wantErr := errors.New("save failed")
	existing := note(nID('V'), nil)
	st := &fakeStore{tx: newFakeTx(existing)}
	st.tx.saveErr = wantErr

	if err := New(st).UpdateNote(context.Background(), *existing); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}
