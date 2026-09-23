package delete_archive_node

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

func (f *fakeRepo) DeleteArchiveNode(_ context.Context, id models.ID) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)

	return nil
}

func TestDeleteArchiveNodeCallsRepo(t *testing.T) {
	repo := &fakeRepo{}
	id := models.ID("AN-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	if err := New(repo).DeleteArchiveNode(context.Background(), id); err != nil {
		t.Fatalf("DeleteArchiveNode: %v", err)
	}

	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("deleted = %v", repo.deleted)
	}
}

func TestDeleteArchiveNodeInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	err := New(repo).DeleteArchiveNode(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}

	if len(repo.deleted) != 0 {
		t.Fatalf("repo вызван при невалидном id")
	}
}

func TestDeleteArchiveNodePropagatesInUseError(t *testing.T) {
	wantErr := &models.InUseError{Type: models.TypeArchiveNode, ID: "AN-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	repo := &fakeRepo{err: wantErr}

	err := New(repo).DeleteArchiveNode(context.Background(), "AN-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	var iu *models.InUseError
	if !errors.As(err, &iu) {
		t.Fatalf("err = %v, want InUseError", err)
	}
}
