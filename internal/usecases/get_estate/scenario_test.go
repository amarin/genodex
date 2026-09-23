package get_estate

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	estates map[models.ID]*models.Estate
}

func (f *fakeRepo) GetEstate(_ context.Context, id models.ID) (*models.Estate, error) {
	s, ok := f.estates[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestGetEstateReturnsRecord(t *testing.T) {
	id := models.ID("ES-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{estates: map[models.ID]*models.Estate{id: {ID: id, Canonical: "Иванов"}}}

	got, err := New(repo).GetEstate(context.Background(), id)
	if err != nil {
		t.Fatalf("GetEstate: %v", err)
	}

	if got.Canonical != "Иванов" {
		t.Fatalf("Canonical = %q", got.Canonical)
	}
}

func TestGetEstateNotFound(t *testing.T) {
	repo := &fakeRepo{estates: map[models.ID]*models.Estate{}}

	_, err := New(repo).GetEstate(context.Background(), "ES-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetEstateInvalidID(t *testing.T) {
	repo := &fakeRepo{estates: map[models.ID]*models.Estate{}}

	_, err := New(repo).GetEstate(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}
