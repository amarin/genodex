package get_church

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	churches map[models.ID]*models.Church
}

func (f *fakeRepo) GetChurch(_ context.Context, id models.ID) (*models.Church, error) {
	s, ok := f.churches[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestGetChurchReturnsRecord(t *testing.T) {
	id := models.ID("CH-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{churches: map[models.ID]*models.Church{id: {ID: id, Name: "Никольская церковь"}}}

	got, err := New(repo).GetChurch(context.Background(), id)
	if err != nil {
		t.Fatalf("GetChurch: %v", err)
	}

	if got.Name != "Никольская церковь" {
		t.Fatalf("Name = %q", got.Name)
	}
}

func TestGetChurchNotFound(t *testing.T) {
	repo := &fakeRepo{churches: map[models.ID]*models.Church{}}

	_, err := New(repo).GetChurch(context.Background(), "CH-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetChurchInvalidID(t *testing.T) {
	repo := &fakeRepo{churches: map[models.ID]*models.Church{}}

	_, err := New(repo).GetChurch(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}
