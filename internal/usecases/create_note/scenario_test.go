package create_note

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

type stubIDs struct {
	id      models.ID
	gotType models.Type
}

func (s *stubIDs) New(t models.Type) models.ID {
	s.gotType = t

	return s.id
}

// fakeTx реализует нужные сценарию методы store.Store поверх карты заметок;
// остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	notes   map[models.ID]*models.Note
	saved   []*models.Note
	getErr  error
	saveErr error
}

func newFakeTx(existing ...*models.Note) *fakeTx {
	tx := &fakeTx{notes: map[models.ID]*models.Note{}}
	for _, n := range existing {
		tx.notes[n.ID] = n
	}

	return tx
}

func (f *fakeTx) GetNote(_ context.Context, id models.ID) (*models.Note, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

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
	tx      *fakeTx
	inTxErr error
	calls   int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	if f.inTxErr != nil {
		return f.inTxErr
	}

	return fn(f.tx)
}

func validInput() models.Note {
	return models.Note{Kind: "note", Text: "текст заметки"}
}

func TestCreateNoteGeneratesIDAndSaves(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	ids := &stubIDs{id: nID('V')}

	got, err := New(st, ids).CreateNote(context.Background(), validInput())
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}

	if got.ID != nID('V') || got.Text != "текст заметки" {
		t.Fatalf("got %+v, ожидалась заметка с ID %v", got, nID('V'))
	}

	if ids.gotType != models.TypeNote {
		t.Errorf("генератор вызван с типом %q, ожидался note", ids.gotType)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].ID != nID('V') {
		t.Fatalf("InTx=%d, saved=%v; ожидалось одно сохранение внутри одной транзакции", st.calls, st.tx.saved)
	}
}

func TestCreateNoteRejectsExplicitID(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	ids := &stubIDs{id: nID('V')}

	in := validInput()
	in.ID = nID('0')

	_, err := New(st, ids).CreateNote(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю id", err)
	}

	if st.calls != 0 || ids.gotType != "" {
		t.Fatalf("InTx=%d, генератор вызван с %q; ожидалось: не вызваны", st.calls, ids.gotType)
	}
}

func TestCreateNoteValidatesBeforeTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	in := models.Note{Kind: "note"} // ни Title, ни Text

	_, err := New(st, &stubIDs{id: nID('V')}).CreateNote(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "text" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю text", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestCreateNoteWithParentSaves(t *testing.T) {
	parent := &models.Note{ID: nID('0'), Kind: "book", Title: "Книга"}
	st := &fakeStore{tx: newFakeTx(parent)}

	in := validInput()
	pid := parent.ID
	in.ParentID = &pid

	got, err := New(st, &stubIDs{id: nID('V')}).CreateNote(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}

	if got.ParentID == nil || *got.ParentID != parent.ID || len(st.tx.saved) != 1 {
		t.Fatalf("got %+v, saved=%d; ожидалось сохранение с родителем", got, len(st.tx.saved))
	}
}

func TestCreateNoteParentNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	in := validInput()
	pid := nID('0')
	in.ParentID = &pid

	_, err := New(st, &stubIDs{id: nID('V')}).CreateNote(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d заметок при несуществующем родителе", len(st.tx.saved))
	}
}

func TestCreateNotePropagatesSaveError(t *testing.T) {
	wantErr := errors.New("save failed")
	st := &fakeStore{tx: newFakeTx()}
	st.tx.saveErr = wantErr

	if _, err := New(st, &stubIDs{id: nID('V')}).CreateNote(context.Background(), validInput()); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

func TestCreateNotePropagatesTxError(t *testing.T) {
	wantErr := errors.New("tx failed")
	st := &fakeStore{tx: newFakeTx(), inTxErr: wantErr}

	if _, err := New(st, &stubIDs{id: nID('V')}).CreateNote(context.Background(), validInput()); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

func TestCreateNotePropagatesParentGetError(t *testing.T) {
	wantErr := errors.New("get failed")
	st := &fakeStore{tx: newFakeTx()}
	st.tx.getErr = wantErr

	in := validInput()
	pid := nID('0')
	in.ParentID = &pid

	_, err := New(st, &stubIDs{id: nID('V')}).CreateNote(context.Background(), in)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}

	var ve *models.ValidationError
	if errors.As(err, &ve) {
		t.Fatalf("сбой хранилища превращён в *ValidationError: %v", err)
	}
}
