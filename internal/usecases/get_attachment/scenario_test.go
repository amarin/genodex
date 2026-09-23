package get_attachment

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	attachments map[models.ID]*models.Attachment
}

func (f *fakeRepo) GetAttachment(_ context.Context, id models.ID) (*models.Attachment, error) {
	a, ok := f.attachments[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return a, nil
}

func TestGetAttachmentReturnsRecord(t *testing.T) {
	id := models.ID("O-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{attachments: map[models.ID]*models.Attachment{id: {ID: id, Kind: models.AttachmentKindScan, Filename: "0012.jpg"}}}

	got, err := New(repo).GetAttachment(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetAttachment: %v", err)
	}

	if got.Filename != "0012.jpg" {
		t.Fatalf("Filename = %q", got.Filename)
	}
}

func TestGetAttachmentNotFound(t *testing.T) {
	repo := &fakeRepo{attachments: map[models.ID]*models.Attachment{}}

	_, err := New(repo).GetAttachment(context.Background(), models.AccessFull, "O-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetAttachmentInvalidID(t *testing.T) {
	repo := &fakeRepo{attachments: map[models.ID]*models.Attachment{}}

	_, err := New(repo).GetAttachment(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestGetAttachmentPrivateHiddenFromPublic(t *testing.T) {
	id := models.ID("O-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{attachments: map[models.ID]*models.Attachment{id: {ID: id, Kind: models.AttachmentKindScan, Filename: "0012.jpg", Private: true}}}

	_, err := New(repo).GetAttachment(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for private record with AccessPublic", err)
	}
}

func TestGetAttachmentPrivateVisibleToFullAccess(t *testing.T) {
	id := models.ID("O-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{attachments: map[models.ID]*models.Attachment{id: {ID: id, Kind: models.AttachmentKindScan, Filename: "0012.jpg", Private: true}}}

	got, err := New(repo).GetAttachment(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetAttachment: %v", err)
	}

	if got.Filename != "0012.jpg" {
		t.Fatalf("Filename = %q", got.Filename)
	}
}
