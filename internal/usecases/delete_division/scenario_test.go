package delete_division

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type ctxKey struct{}

// adID возвращает корректный идентификатор деления, отличающийся последним символом.
func adID(last byte) models.ID {
	return models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// fakeRepo возвращает заданную ошибку удаления и запоминает вызовы.
type fakeRepo struct {
	err    error
	gotCtx context.Context
	gotIDs []models.ID
}

func (f *fakeRepo) DeleteAdministrativeDivision(ctx context.Context, id models.ID) error {
	f.gotCtx = ctx
	f.gotIDs = append(f.gotIDs, id)

	return f.err
}

func TestDeleteDivisionDeletes(t *testing.T) {
	repo := &fakeRepo{}

	if err := New(repo).DeleteDivision(context.Background(), adID('V')); err != nil {
		t.Fatalf("DeleteDivision: %v", err)
	}

	if len(repo.gotIDs) != 1 || repo.gotIDs[0] != adID('V') {
		t.Fatalf("репозиторий вызван с %v, ожидался один вызов с %v", repo.gotIDs, adID('V'))
	}
}

func TestDeleteDivisionNotFound(t *testing.T) {
	repo := &fakeRepo{err: models.ErrNotFound}

	if err := New(repo).DeleteDivision(context.Background(), adID('V')); !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}
}

// TestDeleteDivisionInUse: *InUseError пробрасывается как есть — S15 превратит
// его в 409 со списком ссылающихся.
func TestDeleteDivisionInUse(t *testing.T) {
	inUse := &models.InUseError{
		Type: models.TypeAdministrativeDivision,
		ID:   adID('V'),
		Referrers: []models.EntityRef{
			{Type: models.TypeAdministrativeDivision, ID: adID('0')},
		},
	}
	repo := &fakeRepo{err: inUse}

	err := New(repo).DeleteDivision(context.Background(), adID('V'))

	var got *models.InUseError
	if !errors.As(err, &got) || got != inUse {
		t.Fatalf("err = %v, ожидался исходный *InUseError", err)
	}
}

// TestDeleteDivisionInvalidID: неверный формат id — *ValidationError по полю id,
// репозиторий не вызывается.
func TestDeleteDivisionInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	for _, id := range []models.ID{"", "ad-1", "I-01ARZ3NDEKTSV4RRFFQ69G5FAV"} {
		err := New(repo).DeleteDivision(context.Background(), id)

		var ve *models.ValidationError
		if !errors.As(err, &ve) || ve.Field != "id" || ve.Entity != models.TypeAdministrativeDivision {
			t.Errorf("id %q: err = %v, ожидалась *ValidationError по полю id", id, err)
		}
	}

	if len(repo.gotIDs) != 0 {
		t.Fatalf("репозиторий вызван %d раз при неверном id", len(repo.gotIDs))
	}
}

func TestDeleteDivisionPassesContext(t *testing.T) {
	repo := &fakeRepo{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "marker")

	if err := New(repo).DeleteDivision(ctx, adID('V')); err != nil {
		t.Fatalf("DeleteDivision: %v", err)
	}

	if repo.gotCtx == nil || repo.gotCtx.Value(ctxKey{}) != "marker" {
		t.Fatalf("репозиторий получил контекст %v, ожидался переданный сценарию", repo.gotCtx)
	}
}
