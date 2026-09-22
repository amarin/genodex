package get_division

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

// fakeRepo отдаёт заранее заданную единицу или ошибку и запоминает вызовы.
type fakeRepo struct {
	division *models.AdministrativeDivision
	err      error
	gotCtx   context.Context
	gotIDs   []models.ID
}

func (f *fakeRepo) GetAdministrativeDivision(ctx context.Context, id models.ID) (*models.AdministrativeDivision, error) {
	f.gotCtx = ctx
	f.gotIDs = append(f.gotIDs, id)

	if f.err != nil {
		return nil, f.err
	}

	return f.division, nil
}

func TestGetDivisionReturnsEntity(t *testing.T) {
	want := &models.AdministrativeDivision{ID: adID('V'), Name: "Село", Type: models.AdminDivisionSelo}
	repo := &fakeRepo{division: want}

	got, err := New(repo).GetDivision(context.Background(), adID('V'))
	if err != nil {
		t.Fatalf("GetDivision: %v", err)
	}

	if got.ID != want.ID || got.Name != want.Name || got.Type != want.Type {
		t.Fatalf("got %+v, ожидалась %+v", got, *want)
	}

	if len(repo.gotIDs) != 1 || repo.gotIDs[0] != adID('V') {
		t.Fatalf("репозиторий вызван с %v, ожидался один вызов с %v", repo.gotIDs, adID('V'))
	}
}

func TestGetDivisionNotFound(t *testing.T) {
	repo := &fakeRepo{err: models.ErrNotFound}

	if _, err := New(repo).GetDivision(context.Background(), adID('V')); !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}
}

func TestGetDivisionPropagatesRepoError(t *testing.T) {
	wantErr := errors.New("repo down")
	repo := &fakeRepo{err: wantErr}

	if _, err := New(repo).GetDivision(context.Background(), adID('V')); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

// TestGetDivisionInvalidID: неверный формат id — *ValidationError по полю id,
// репозиторий не вызывается.
func TestGetDivisionInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	for _, id := range []models.ID{"", "ad-1", "I-01ARZ3NDEKTSV4RRFFQ69G5FAV", "AD-короткий"} {
		_, err := New(repo).GetDivision(context.Background(), id)

		var ve *models.ValidationError
		if !errors.As(err, &ve) || ve.Field != "id" || ve.Entity != models.TypeAdministrativeDivision {
			t.Errorf("id %q: err = %v, ожидалась *ValidationError по полю id", id, err)
		}
	}

	if len(repo.gotIDs) != 0 {
		t.Fatalf("репозиторий вызван %d раз при неверном id", len(repo.gotIDs))
	}
}

func TestGetDivisionPassesContext(t *testing.T) {
	repo := &fakeRepo{division: &models.AdministrativeDivision{ID: adID('V'), Name: "Село", Type: models.AdminDivisionSelo}}
	ctx := context.WithValue(context.Background(), ctxKey{}, "marker")

	if _, err := New(repo).GetDivision(ctx, adID('V')); err != nil {
		t.Fatalf("GetDivision: %v", err)
	}

	if repo.gotCtx == nil || repo.gotCtx.Value(ctxKey{}) != "marker" {
		t.Fatalf("репозиторий получил контекст %v, ожидался переданный сценарию", repo.gotCtx)
	}
}
