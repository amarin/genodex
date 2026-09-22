package update_division

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// adID возвращает корректный идентификатор деления, отличающийся последним символом.
func adID(last byte) models.ID {
	return models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// fakeTx реализует нужные сценарию методы store.Store поверх карты делений;
// остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	divisions map[models.ID]*models.AdministrativeDivision
	saved     []*models.AdministrativeDivision
	saveErr   error
	gets      int
}

func newFakeTx(existing ...*models.AdministrativeDivision) *fakeTx {
	tx := &fakeTx{divisions: map[models.ID]*models.AdministrativeDivision{}}
	for _, d := range existing {
		tx.divisions[d.ID] = d
	}

	return tx
}

func (f *fakeTx) GetAdministrativeDivision(_ context.Context, id models.ID) (*models.AdministrativeDivision, error) {
	f.gets++

	d, ok := f.divisions[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *d

	return &cp, nil
}

func (f *fakeTx) SaveAdministrativeDivision(_ context.Context, d *models.AdministrativeDivision) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *d
	f.divisions[d.ID] = &cp
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует DivisionStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func division(id models.ID, parent *models.ID) *models.AdministrativeDivision {
	return &models.AdministrativeDivision{ID: id, Name: "Единица " + string(id), Type: models.AdminDivisionSelo, ParentID: parent}
}

func TestUpdateDivisionSaves(t *testing.T) {
	existing := division(adID('V'), nil)
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Name = "Новое название"

	if err := New(st).UpdateDivision(context.Background(), updated); err != nil {
		t.Fatalf("UpdateDivision: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Name != "Новое название" {
		t.Fatalf("InTx=%d, saved=%v; ожидалось одно сохранение с новым названием", st.calls, st.tx.saved)
	}
}

func TestUpdateDivisionNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateDivision(context.Background(), *division(adID('V'), nil))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d единиц при несуществующей единице", len(st.tx.saved))
	}
}

// TestUpdateDivisionValidatesBeforeTx: невалидная сущность — *ValidationError,
// транзакция не открывается.
func TestUpdateDivisionValidatesBeforeTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	bad := *division(adID('V'), nil)
	bad.Name = ""

	err := New(st).UpdateDivision(context.Background(), bad)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю name", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

// TestUpdateDivisionParentNotFound: новый родитель не существует —
// *ValidationError по полю parent_id, ничего не сохраняется.
func TestUpdateDivisionParentNotFound(t *testing.T) {
	existing := division(adID('V'), nil)
	st := &fakeStore{tx: newFakeTx(existing)}

	missing := adID('0')
	updated := *existing
	updated.ParentID = &missing

	err := New(st).UpdateDivision(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d единиц при несуществующем родителе", len(st.tx.saved))
	}
}

// TestUpdateDivisionDirectCycle: B — дочка A; попытка сделать A дочкой B — цикл,
// *ValidationError по полю parent_id.
func TestUpdateDivisionDirectCycle(t *testing.T) {
	idA, idB := adID('A'), adID('B')
	a := division(idA, nil)
	b := division(idB, &idA)
	st := &fakeStore{tx: newFakeTx(a, b)}

	updated := *a
	updated.ParentID = &idB

	err := New(st).UpdateDivision(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id (цикл)", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d единиц при цикле", len(st.tx.saved))
	}
}

// TestUpdateDivisionLongCycle: цепочка C → B → A; попытка сделать A дочкой C —
// цикл длиной три, *ValidationError по полю parent_id.
func TestUpdateDivisionLongCycle(t *testing.T) {
	idA, idB, idC := adID('A'), adID('B'), adID('C')
	a := division(idA, nil)
	b := division(idB, &idA)
	c := division(idC, &idB)
	st := &fakeStore{tx: newFakeTx(a, b, c)}

	updated := *a
	updated.ParentID = &idC

	err := New(st).UpdateDivision(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id (цикл)", err)
	}
}

// TestUpdateDivisionReparentOK: перенос под другого корректного родителя проходит.
func TestUpdateDivisionReparentOK(t *testing.T) {
	idA, idB, idC := adID('A'), adID('B'), adID('C')
	a := division(idA, nil)
	b := division(idB, &idA)
	c := division(idC, nil)
	st := &fakeStore{tx: newFakeTx(a, b, c)}

	updated := *b
	updated.ParentID = &idC

	if err := New(st).UpdateDivision(context.Background(), updated); err != nil {
		t.Fatalf("UpdateDivision: %v", err)
	}

	if len(st.tx.saved) != 1 || *st.tx.saved[0].ParentID != idC {
		t.Fatalf("saved=%v; ожидалось сохранение с родителем %v", st.tx.saved, idC)
	}
}

// TestUpdateDivisionWithoutParentSkipsWalk: без родителя цепочка не обходится —
// одно чтение (существование самой единицы).
func TestUpdateDivisionWithoutParentSkipsWalk(t *testing.T) {
	existing := division(adID('V'), nil)
	st := &fakeStore{tx: newFakeTx(existing)}

	if err := New(st).UpdateDivision(context.Background(), *existing); err != nil {
		t.Fatalf("UpdateDivision: %v", err)
	}

	if st.tx.gets != 1 {
		t.Fatalf("чтений %d, ожидалось одно (без обхода родителей)", st.tx.gets)
	}
}

func TestUpdateDivisionPropagatesSaveError(t *testing.T) {
	wantErr := errors.New("save failed")
	existing := division(adID('V'), nil)
	st := &fakeStore{tx: newFakeTx(existing)}
	st.tx.saveErr = wantErr

	if err := New(st).UpdateDivision(context.Background(), *existing); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}
