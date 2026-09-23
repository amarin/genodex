package get_archive_node

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	nodes map[models.ID]*models.ArchiveNode
}

func (f *fakeRepo) GetArchiveNode(_ context.Context, id models.ID) (*models.ArchiveNode, error) {
	n, ok := f.nodes[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return n, nil
}

func TestGetArchiveNodeReturnsRecord(t *testing.T) {
	id := models.ID("AN-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{nodes: map[models.ID]*models.ArchiveNode{id: {ID: id, Label: "Фонд 1"}}}

	got, err := New(repo).GetArchiveNode(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetArchiveNode: %v", err)
	}

	if got.Label != "Фонд 1" {
		t.Fatalf("Label = %q", got.Label)
	}
}

func TestGetArchiveNodeNotFound(t *testing.T) {
	repo := &fakeRepo{nodes: map[models.ID]*models.ArchiveNode{}}

	_, err := New(repo).GetArchiveNode(context.Background(), models.AccessFull, "AN-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetArchiveNodeInvalidID(t *testing.T) {
	repo := &fakeRepo{nodes: map[models.ID]*models.ArchiveNode{}}

	_, err := New(repo).GetArchiveNode(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestGetArchiveNodePrivateHiddenFromPublic(t *testing.T) {
	id := models.ID("AN-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{nodes: map[models.ID]*models.ArchiveNode{id: {ID: id, Label: "Фонд 1", Private: true}}}

	_, err := New(repo).GetArchiveNode(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for private record with AccessPublic", err)
	}
}

func TestGetArchiveNodePrivateVisibleToFullAccess(t *testing.T) {
	id := models.ID("AN-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{nodes: map[models.ID]*models.ArchiveNode{id: {ID: id, Label: "Фонд 1", Private: true}}}

	got, err := New(repo).GetArchiveNode(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetArchiveNode: %v", err)
	}

	if got.Label != "Фонд 1" {
		t.Fatalf("Label = %q", got.Label)
	}
}
