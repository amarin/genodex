package get_family

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	families  map[models.ID]*models.Family
	citations map[models.ID]*models.Citation
}

func (f *fakeRepo) GetFamily(_ context.Context, id models.ID) (*models.Family, error) {
	s, ok := f.families[id]
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

func TestGetFamilyReturnsRecord(t *testing.T) {
	id := models.ID("F-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{families: map[models.ID]*models.Family{id: {ID: id, Name: "Ивановы"}}}

	got, err := New(repo).GetFamily(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetFamily: %v", err)
	}

	if got.Name != "Ивановы" {
		t.Fatalf("Name = %q", got.Name)
	}
}

func TestGetFamilyNotFound(t *testing.T) {
	repo := &fakeRepo{families: map[models.ID]*models.Family{}}

	_, err := New(repo).GetFamily(context.Background(), models.AccessFull, "F-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetFamilyInvalidID(t *testing.T) {
	repo := &fakeRepo{families: map[models.ID]*models.Family{}}

	_, err := New(repo).GetFamily(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestGetFamilyPrivateHiddenFromPublic(t *testing.T) {
	id := models.ID("F-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{families: map[models.ID]*models.Family{id: {ID: id, Name: "Ивановы", Private: true}}}

	_, err := New(repo).GetFamily(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for private record with AccessPublic", err)
	}
}

func TestGetFamilyPrivateVisibleToFullAccess(t *testing.T) {
	id := models.ID("F-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{families: map[models.ID]*models.Family{id: {ID: id, Name: "Ивановы", Private: true}}}

	got, err := New(repo).GetFamily(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetFamily: %v", err)
	}

	if got.Name != "Ивановы" {
		t.Fatalf("Name = %q", got.Name)
	}
}

// TestGetFamilyHiddenWhenReferencesPrivateCitation: род сам по себе
// публичен, но ссылается (Sources[i].CitationID) на приватную цитату — для
// вызывающего без полного доступа он отдаётся как models.ErrNotFound, тот же
// принцип «прячем как отсутствующее», что и для собственного Private.
func TestGetFamilyHiddenWhenReferencesPrivateCitation(t *testing.T) {
	id := models.ID("F-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		families: map[models.ID]*models.Family{
			id: {ID: id, Name: "Ивановы", Sources: []models.SourceLink{{CitationID: citation}}},
		},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	_, err := New(repo).GetFamily(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound (ссылка на приватную цитату)", err)
	}
}

// TestGetFamilyVisibleWithFullAccessWhenReferencesPrivateCitation: тот же
// род, но для вызывающего с полным доступом — виден.
func TestGetFamilyVisibleWithFullAccessWhenReferencesPrivateCitation(t *testing.T) {
	id := models.ID("F-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		families: map[models.ID]*models.Family{
			id: {ID: id, Name: "Ивановы", Sources: []models.SourceLink{{CitationID: citation}}},
		},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	got, err := New(repo).GetFamily(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetFamily: %v", err)
	}

	if got.Name != "Ивановы" {
		t.Fatalf("Name = %q", got.Name)
	}
}
