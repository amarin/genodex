package get_citation

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	citations map[models.ID]*models.Citation
}

func (f *fakeRepo) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return c, nil
}

func TestGetCitationReturnsRecord(t *testing.T) {
	id := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	src := models.ID("S-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{citations: map[models.ID]*models.Citation{id: {ID: id, SourceID: src, Text: "стр. 12"}}}

	got, err := New(repo).GetCitation(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetCitation: %v", err)
	}

	if got.Text != "стр. 12" {
		t.Fatalf("Text = %q", got.Text)
	}
}

func TestGetCitationNotFound(t *testing.T) {
	repo := &fakeRepo{citations: map[models.ID]*models.Citation{}}

	_, err := New(repo).GetCitation(context.Background(), models.AccessFull, "C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetCitationInvalidID(t *testing.T) {
	repo := &fakeRepo{citations: map[models.ID]*models.Citation{}}

	_, err := New(repo).GetCitation(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestGetCitationPrivateHiddenFromPublic(t *testing.T) {
	id := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	src := models.ID("S-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{citations: map[models.ID]*models.Citation{id: {ID: id, SourceID: src, Text: "стр. 12", Private: true}}}

	_, err := New(repo).GetCitation(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for private record with AccessPublic", err)
	}
}

func TestGetCitationPrivateVisibleToFullAccess(t *testing.T) {
	id := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	src := models.ID("S-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{citations: map[models.ID]*models.Citation{id: {ID: id, SourceID: src, Text: "стр. 12", Private: true}}}

	got, err := New(repo).GetCitation(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetCitation: %v", err)
	}

	if got.Text != "стр. 12" {
		t.Fatalf("Text = %q", got.Text)
	}
}
