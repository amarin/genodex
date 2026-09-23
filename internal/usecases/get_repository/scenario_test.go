package get_repository

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	repositories map[models.ID]*models.Repository
}

func (f *fakeRepo) GetRepository(_ context.Context, id models.ID) (*models.Repository, error) {
	s, ok := f.repositories[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestGetRepositoryReturnsRecord(t *testing.T) {
	id := models.ID("R-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{repositories: map[models.ID]*models.Repository{id: {ID: id, Name: "ГАВО"}}}

	got, err := New(repo).GetRepository(context.Background(), id)
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}

	if got.Name != "ГАВО" {
		t.Fatalf("Name = %q", got.Name)
	}
}

func TestGetRepositoryNotFound(t *testing.T) {
	repo := &fakeRepo{repositories: map[models.ID]*models.Repository{}}

	_, err := New(repo).GetRepository(context.Background(), "R-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetRepositoryInvalidID(t *testing.T) {
	repo := &fakeRepo{repositories: map[models.ID]*models.Repository{}}

	_, err := New(repo).GetRepository(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}
