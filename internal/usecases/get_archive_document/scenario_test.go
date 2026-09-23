package get_archive_document

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	docs map[models.ID]*models.ArchiveDocument
}

func (f *fakeRepo) GetArchiveDocument(_ context.Context, id models.ID) (*models.ArchiveDocument, error) {
	d, ok := f.docs[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return d, nil
}

func TestGetArchiveDocumentReturnsRecord(t *testing.T) {
	id := models.ID("DC-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{docs: map[models.ID]*models.ArchiveDocument{id: {ID: id, Title: "Метрическая книга"}}}

	got, err := New(repo).GetArchiveDocument(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetArchiveDocument: %v", err)
	}

	if got.Title != "Метрическая книга" {
		t.Fatalf("Title = %q", got.Title)
	}
}

func TestGetArchiveDocumentNotFound(t *testing.T) {
	repo := &fakeRepo{docs: map[models.ID]*models.ArchiveDocument{}}

	_, err := New(repo).GetArchiveDocument(context.Background(), models.AccessFull, "DC-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetArchiveDocumentInvalidID(t *testing.T) {
	repo := &fakeRepo{docs: map[models.ID]*models.ArchiveDocument{}}

	_, err := New(repo).GetArchiveDocument(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestGetArchiveDocumentPrivateHiddenFromPublic(t *testing.T) {
	id := models.ID("DC-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{docs: map[models.ID]*models.ArchiveDocument{id: {ID: id, Title: "Метрическая книга", Private: true}}}

	_, err := New(repo).GetArchiveDocument(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for private record with AccessPublic", err)
	}
}

func TestGetArchiveDocumentPrivateVisibleToFullAccess(t *testing.T) {
	id := models.ID("DC-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{docs: map[models.ID]*models.ArchiveDocument{id: {ID: id, Title: "Метрическая книга", Private: true}}}

	got, err := New(repo).GetArchiveDocument(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetArchiveDocument: %v", err)
	}

	if got.Title != "Метрическая книга" {
		t.Fatalf("Title = %q", got.Title)
	}
}
