package get_parish

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	parishes  map[models.ID]*models.Parish
	citations map[models.ID]*models.Citation
}

func (f *fakeRepo) GetParish(_ context.Context, id models.ID) (*models.Parish, error) {
	s, ok := f.parishes[id]
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

func TestGetParishReturnsRecord(t *testing.T) {
	id := models.ID("PR-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{parishes: map[models.ID]*models.Parish{id: {ID: id, Name: "Никольский приход"}}}

	got, err := New(repo).GetParish(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetParish: %v", err)
	}

	if got.Name != "Никольский приход" {
		t.Fatalf("Name = %q", got.Name)
	}
}

func TestGetParishNotFound(t *testing.T) {
	repo := &fakeRepo{parishes: map[models.ID]*models.Parish{}}

	_, err := New(repo).GetParish(context.Background(), models.AccessFull, "PR-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetParishInvalidID(t *testing.T) {
	repo := &fakeRepo{parishes: map[models.ID]*models.Parish{}}

	_, err := New(repo).GetParish(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

// TestGetParishHiddenWhenReferencesPrivateCitation: публичная запись,
// ссылающаяся на приватную цитату среди источников, скрывается как
// отсутствующая для вызывающего без полного доступа, но видна с
// AccessFull.
func TestGetParishHiddenWhenReferencesPrivateCitation(t *testing.T) {
	id := models.ID("PR-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	citID := models.ID("CI-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	p := &models.Parish{ID: id, Name: "Никольский приход", Sources: []models.SourceLink{{CitationID: citID}}}

	repo := &fakeRepo{
		parishes:  map[models.ID]*models.Parish{id: p},
		citations: map[models.ID]*models.Citation{citID: {ID: citID, Private: true}},
	}

	if _, err := New(repo).GetParish(context.Background(), models.AccessPublic, id); !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound для публичного доступа", err)
	}

	got, err := New(repo).GetParish(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetParish с полным доступом: %v", err)
	}

	if got.ID != id {
		t.Fatalf("got.ID = %v, want %v", got.ID, id)
	}
}
