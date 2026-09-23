package get_given_name

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	givenNames map[models.ID]*models.GivenName
}

func (f *fakeRepo) GetGivenName(_ context.Context, id models.ID) (*models.GivenName, error) {
	s, ok := f.givenNames[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestGetGivenNameReturnsRecord(t *testing.T) {
	id := models.ID("GN-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{givenNames: map[models.ID]*models.GivenName{id: {ID: id, Canonical: "Иванов"}}}

	got, err := New(repo).GetGivenName(context.Background(), id)
	if err != nil {
		t.Fatalf("GetGivenName: %v", err)
	}

	if got.Canonical != "Иванов" {
		t.Fatalf("Canonical = %q", got.Canonical)
	}
}

func TestGetGivenNameNotFound(t *testing.T) {
	repo := &fakeRepo{givenNames: map[models.ID]*models.GivenName{}}

	_, err := New(repo).GetGivenName(context.Background(), "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetGivenNameInvalidID(t *testing.T) {
	repo := &fakeRepo{givenNames: map[models.ID]*models.GivenName{}}

	_, err := New(repo).GetGivenName(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}
