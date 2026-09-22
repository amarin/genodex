package create_division

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

// stubIDs — генератор с заранее известным результатом; запоминает запрошенный тип.
type stubIDs struct {
	id      models.ID
	gotType models.Type
}

func (s *stubIDs) New(t models.Type) models.ID {
	s.gotType = t

	return s.id
}

// fakeTx реализует нужные сценарию методы store.Store поверх карты делений;
// остальные методы порта паникуют через nil-встраивание — сценарий не должен
// их звать.
type fakeTx struct {
	store.Store
	divisions map[models.ID]*models.AdministrativeDivision
	saved     []*models.AdministrativeDivision
	getErr    error
	saveErr   error
}

func newFakeTx(existing ...*models.AdministrativeDivision) *fakeTx {
	tx := &fakeTx{divisions: map[models.ID]*models.AdministrativeDivision{}}
	for _, d := range existing {
		tx.divisions[d.ID] = d
	}

	return tx
}

func (f *fakeTx) GetAdministrativeDivision(_ context.Context, id models.ID) (*models.AdministrativeDivision, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

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
	tx      *fakeTx
	inTxErr error // ошибка самого InTx (например, отменённый контекст)
	calls   int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	if f.inTxErr != nil {
		return f.inTxErr
	}

	return fn(f.tx)
}

func validInput() models.AdministrativeDivision {
	return models.AdministrativeDivision{Name: "Село", Type: models.AdminDivisionSelo}
}

func TestCreateDivisionGeneratesIDAndSaves(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	ids := &stubIDs{id: adID('V')}

	got, err := New(st, ids).CreateDivision(context.Background(), validInput())
	if err != nil {
		t.Fatalf("CreateDivision: %v", err)
	}

	if got.ID != adID('V') || got.Name != "Село" {
		t.Fatalf("got %+v, ожидалась единица с ID %v", got, adID('V'))
	}

	if ids.gotType != models.TypeAdministrativeDivision {
		t.Errorf("генератор вызван с типом %q, ожидался administrative_division", ids.gotType)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].ID != adID('V') {
		t.Fatalf("InTx=%d, saved=%v; ожидалось одно сохранение внутри одной транзакции", st.calls, st.tx.saved)
	}
}

// TestCreateDivisionRejectsExplicitID: явный входной ID допустим только для
// импорта — сценарий отвергает его до генерации и обращения к хранилищу.
func TestCreateDivisionRejectsExplicitID(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	ids := &stubIDs{id: adID('V')}

	in := validInput()
	in.ID = adID('0')

	_, err := New(st, ids).CreateDivision(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю id", err)
	}

	if st.calls != 0 || ids.gotType != "" {
		t.Fatalf("InTx=%d, генератор вызван с %q; ожидалось: не вызваны", st.calls, ids.gotType)
	}
}

// TestCreateDivisionValidatesBeforeTx: невалидная сущность (пустое название) —
// *ValidationError, транзакция не открывается.
func TestCreateDivisionValidatesBeforeTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	in := validInput()
	in.Name = ""

	_, err := New(st, &stubIDs{id: adID('V')}).CreateDivision(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю name", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestCreateDivisionWithParentSaves(t *testing.T) {
	parent := &models.AdministrativeDivision{ID: adID('0'), Name: "Волость", Type: models.AdminDivisionVolost}
	st := &fakeStore{tx: newFakeTx(parent)}

	in := validInput()
	pid := parent.ID
	in.ParentID = &pid

	got, err := New(st, &stubIDs{id: adID('V')}).CreateDivision(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateDivision: %v", err)
	}

	if got.ParentID == nil || *got.ParentID != parent.ID || len(st.tx.saved) != 1 {
		t.Fatalf("got %+v, saved=%d; ожидалось сохранение с родителем", got, len(st.tx.saved))
	}
}

// TestCreateDivisionParentNotFound: несуществующий родитель — *ValidationError
// по полю parent_id (в S15 — 422), ничего не сохраняется.
func TestCreateDivisionParentNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	in := validInput()
	pid := adID('0')
	in.ParentID = &pid

	_, err := New(st, &stubIDs{id: adID('V')}).CreateDivision(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d единиц при несуществующем родителе", len(st.tx.saved))
	}
}

func TestCreateDivisionPropagatesSaveError(t *testing.T) {
	wantErr := errors.New("save failed")
	st := &fakeStore{tx: newFakeTx()}
	st.tx.saveErr = wantErr

	if _, err := New(st, &stubIDs{id: adID('V')}).CreateDivision(context.Background(), validInput()); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

func TestCreateDivisionPropagatesTxError(t *testing.T) {
	wantErr := errors.New("tx failed")
	st := &fakeStore{tx: newFakeTx(), inTxErr: wantErr}

	if _, err := New(st, &stubIDs{id: adID('V')}).CreateDivision(context.Background(), validInput()); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

// TestCreateDivisionPropagatesParentGetError: сбой чтения родителя (не
// ErrNotFound) — ошибка хранилища как есть, не *ValidationError.
func TestCreateDivisionPropagatesParentGetError(t *testing.T) {
	wantErr := errors.New("get failed")
	st := &fakeStore{tx: newFakeTx()}
	st.tx.getErr = wantErr

	in := validInput()
	pid := adID('0')
	in.ParentID = &pid

	_, err := New(st, &stubIDs{id: adID('V')}).CreateDivision(context.Background(), in)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}

	var ve *models.ValidationError
	if errors.As(err, &ve) {
		t.Fatalf("сбой хранилища превращён в *ValidationError: %v", err)
	}
}
