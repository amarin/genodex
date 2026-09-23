package delete_given_name

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

func (f *fakeRepo) DeleteGivenName(_ context.Context, id models.ID) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)

	return nil
}

func TestDeleteGivenNameCallsRepo(t *testing.T) {
	repo := &fakeRepo{}
	id := models.ID("GN-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	if err := New(repo).DeleteGivenName(context.Background(), id); err != nil {
		t.Fatalf("DeleteGivenName: %v", err)
	}

	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("deleted = %v", repo.deleted)
	}
}

func TestDeleteGivenNameInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	err := New(repo).DeleteGivenName(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}

	if len(repo.deleted) != 0 {
		t.Fatalf("repo вызван при невалидном id")
	}
}

func TestDeleteGivenNamePropagatesInUseError(t *testing.T) {
	wantErr := &models.InUseError{Type: models.TypeGivenName, ID: "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	repo := &fakeRepo{err: wantErr}

	err := New(repo).DeleteGivenName(context.Background(), "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	var iu *models.InUseError
	if !errors.As(err, &iu) {
		t.Fatalf("err = %v, want InUseError", err)
	}
}
