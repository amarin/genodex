package get_patronymic

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	patronymics map[models.ID]*models.Patronymic
}

func (f *fakeRepo) GetPatronymic(_ context.Context, id models.ID) (*models.Patronymic, error) {
	s, ok := f.patronymics[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestGetPatronymicReturnsRecord(t *testing.T) {
	id := models.ID("PN-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{patronymics: map[models.ID]*models.Patronymic{id: {ID: id, Canonical: "Иванов"}}}

	got, err := New(repo).GetPatronymic(context.Background(), id)
	if err != nil {
		t.Fatalf("GetPatronymic: %v", err)
	}

	if got.Canonical != "Иванов" {
		t.Fatalf("Canonical = %q", got.Canonical)
	}
}

func TestGetPatronymicNotFound(t *testing.T) {
	repo := &fakeRepo{patronymics: map[models.ID]*models.Patronymic{}}

	_, err := New(repo).GetPatronymic(context.Background(), "PN-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetPatronymicInvalidID(t *testing.T) {
	repo := &fakeRepo{patronymics: map[models.ID]*models.Patronymic{}}

	_, err := New(repo).GetPatronymic(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}
