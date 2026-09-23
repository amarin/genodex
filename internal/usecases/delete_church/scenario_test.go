package delete_church

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	deleted []models.ID
	err     error
}

func (f *fakeRepo) DeleteChurch(_ context.Context, id models.ID) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)

	return nil
}

func TestDeleteChurchCallsRepo(t *testing.T) {
	repo := &fakeRepo{}
	id := models.ID("CH-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	if err := New(repo).DeleteChurch(context.Background(), id); err != nil {
		t.Fatalf("DeleteChurch: %v", err)
	}

	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("deleted = %v", repo.deleted)
	}
}

func TestDeleteChurchInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	err := New(repo).DeleteChurch(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}

	if len(repo.deleted) != 0 {
		t.Fatalf("repo вызван при невалидном id")
	}
}

func TestDeleteChurchPropagatesInUseError(t *testing.T) {
	wantErr := &models.InUseError{Type: models.TypeChurch, ID: "CH-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	repo := &fakeRepo{err: wantErr}

	err := New(repo).DeleteChurch(context.Background(), "CH-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	var iu *models.InUseError
	if !errors.As(err, &iu) {
		t.Fatalf("err = %v, want InUseError", err)
	}
}
