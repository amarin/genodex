package get_title

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	titles map[models.ID]*models.Title
}

func (f *fakeRepo) GetTitle(_ context.Context, id models.ID) (*models.Title, error) {
	s, ok := f.titles[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestGetTitleReturnsRecord(t *testing.T) {
	id := models.ID("TT-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{titles: map[models.ID]*models.Title{id: {ID: id, Canonical: "Иванов"}}}

	got, err := New(repo).GetTitle(context.Background(), id)
	if err != nil {
		t.Fatalf("GetTitle: %v", err)
	}

	if got.Canonical != "Иванов" {
		t.Fatalf("Canonical = %q", got.Canonical)
	}
}

func TestGetTitleNotFound(t *testing.T) {
	repo := &fakeRepo{titles: map[models.ID]*models.Title{}}

	_, err := New(repo).GetTitle(context.Background(), "TT-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetTitleInvalidID(t *testing.T) {
	repo := &fakeRepo{titles: map[models.ID]*models.Title{}}

	_, err := New(repo).GetTitle(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}
