package get_church

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	churches  map[models.ID]*models.Church
	citations map[models.ID]*models.Citation
}

func (f *fakeRepo) GetChurch(_ context.Context, id models.ID) (*models.Church, error) {
	s, ok := f.churches[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func (f *fakeRepo) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return c, nil
}

func TestGetChurchReturnsRecord(t *testing.T) {
	id := models.ID("CH-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{churches: map[models.ID]*models.Church{id: {ID: id, Name: "Никольская церковь"}}}

	got, err := New(repo).GetChurch(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetChurch: %v", err)
	}

	if got.Name != "Никольская церковь" {
		t.Fatalf("Name = %q", got.Name)
	}
}

func TestGetChurchNotFound(t *testing.T) {
	repo := &fakeRepo{churches: map[models.ID]*models.Church{}}

	_, err := New(repo).GetChurch(context.Background(), models.AccessFull, "CH-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetChurchInvalidID(t *testing.T) {
	repo := &fakeRepo{churches: map[models.ID]*models.Church{}}

	_, err := New(repo).GetChurch(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

// TestGetChurchHiddenWhenReferencesPrivateCitation: публичная запись,
// ссылающаяся на приватную цитату среди источников, скрывается как
// отсутствующая для вызывающего без полного доступа, но видна с
// AccessFull.
func TestGetChurchHiddenWhenReferencesPrivateCitation(t *testing.T) {
	id := models.ID("CH-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	citID := models.ID("CI-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	c := &models.Church{ID: id, Name: "Никольская церковь", Sources: []models.SourceLink{{CitationID: citID}}}

	repo := &fakeRepo{
		churches:  map[models.ID]*models.Church{id: c},
		citations: map[models.ID]*models.Citation{citID: {ID: citID, Private: true}},
	}

	if _, err := New(repo).GetChurch(context.Background(), models.AccessPublic, id); !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound для публичного доступа", err)
	}

	got, err := New(repo).GetChurch(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetChurch с полным доступом: %v", err)
	}

	if got.ID != id {
		t.Fatalf("got.ID = %v, want %v", got.ID, id)
	}
}
