package get_parish

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	parishes map[models.ID]*models.Parish
}

func (f *fakeRepo) GetParish(_ context.Context, id models.ID) (*models.Parish, error) {
	s, ok := f.parishes[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestGetParishReturnsRecord(t *testing.T) {
	id := models.ID("PR-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{parishes: map[models.ID]*models.Parish{id: {ID: id, Name: "Никольский приход"}}}

	got, err := New(repo).GetParish(context.Background(), id)
	if err != nil {
		t.Fatalf("GetParish: %v", err)
	}

	if got.Name != "Никольский приход" {
		t.Fatalf("Name = %q", got.Name)
	}
}

func TestGetParishNotFound(t *testing.T) {
	repo := &fakeRepo{parishes: map[models.ID]*models.Parish{}}

	_, err := New(repo).GetParish(context.Background(), "PR-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetParishInvalidID(t *testing.T) {
	repo := &fakeRepo{parishes: map[models.ID]*models.Parish{}}

	_, err := New(repo).GetParish(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}
