package get_archive

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	archives map[models.ID]*models.Archive
}

func (f *fakeRepo) GetArchive(_ context.Context, id models.ID) (*models.Archive, error) {
	s, ok := f.archives[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestGetArchiveReturnsRecord(t *testing.T) {
	id := models.ID("AR-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{archives: map[models.ID]*models.Archive{id: {ID: id, Name: "ГАВО, архив"}}}

	got, err := New(repo).GetArchive(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetArchive: %v", err)
	}

	if got.Name != "ГАВО, архив" {
		t.Fatalf("Name = %q", got.Name)
	}
}

func TestGetArchiveNotFound(t *testing.T) {
	repo := &fakeRepo{archives: map[models.ID]*models.Archive{}}

	_, err := New(repo).GetArchive(context.Background(), models.AccessFull, "AR-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetArchiveInvalidID(t *testing.T) {
	repo := &fakeRepo{archives: map[models.ID]*models.Archive{}}

	_, err := New(repo).GetArchive(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestGetArchivePrivateHiddenFromPublic(t *testing.T) {
	id := models.ID("AR-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{archives: map[models.ID]*models.Archive{id: {ID: id, Name: "ГАВО, архив", Private: true}}}

	_, err := New(repo).GetArchive(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for private record with AccessPublic", err)
	}
}

func TestGetArchivePrivateVisibleToFullAccess(t *testing.T) {
	id := models.ID("AR-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{archives: map[models.ID]*models.Archive{id: {ID: id, Name: "ГАВО, архив", Private: true}}}

	got, err := New(repo).GetArchive(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetArchive: %v", err)
	}

	if got.Name != "ГАВО, архив" {
		t.Fatalf("Name = %q", got.Name)
	}
}
