package get_source

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	sources map[models.ID]*models.Source
}

func (f *fakeRepo) GetSource(_ context.Context, id models.ID) (*models.Source, error) {
	s, ok := f.sources[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestGetSourceReturnsRecord(t *testing.T) {
	id := models.ID("S-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{sources: map[models.ID]*models.Source{id: {ID: id, Kind: models.SourceKindDocument, Title: "Метрическая книга", Reliability: models.ReliabilityPrimary}}}

	got, err := New(repo).GetSource(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}

	if got.Title != "Метрическая книга" {
		t.Fatalf("Title = %q", got.Title)
	}
}

func TestGetSourceNotFound(t *testing.T) {
	repo := &fakeRepo{sources: map[models.ID]*models.Source{}}

	_, err := New(repo).GetSource(context.Background(), models.AccessFull, "S-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetSourceInvalidID(t *testing.T) {
	repo := &fakeRepo{sources: map[models.ID]*models.Source{}}

	_, err := New(repo).GetSource(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestGetSourcePrivateHiddenFromPublic(t *testing.T) {
	id := models.ID("S-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{sources: map[models.ID]*models.Source{id: {ID: id, Kind: models.SourceKindDocument, Title: "Метрическая книга", Reliability: models.ReliabilityPrimary, Private: true}}}

	_, err := New(repo).GetSource(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for private record with AccessPublic", err)
	}
}

func TestGetSourcePrivateVisibleToFullAccess(t *testing.T) {
	id := models.ID("S-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{sources: map[models.ID]*models.Source{id: {ID: id, Kind: models.SourceKindDocument, Title: "Метрическая книга", Reliability: models.ReliabilityPrimary, Private: true}}}

	got, err := New(repo).GetSource(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}

	if got.Title != "Метрическая книга" {
		t.Fatalf("Title = %q", got.Title)
	}
}
