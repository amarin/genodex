package search_attachments

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits        []models.Hit
	attachments map[models.ID]*models.Attachment
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetAttachment(_ context.Context, id models.ID) (*models.Attachment, error) {
	s, ok := f.attachments[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestSearchAttachmentsFiltersByType(t *testing.T) {
	id := models.ID("O-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeAttachment, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не attachment — должен быть пропущен
		},
		attachments: map[models.ID]*models.Attachment{id: {ID: id, Kind: models.AttachmentKindScan, Filename: "0012.jpg"}},
	}

	got, err := New(repo).SearchAttachments(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchAttachments: %v", err)
	}

	if len(got) != 1 || got[0].Filename != "0012.jpg" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchAttachmentsEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchAttachments(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchAttachments: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
