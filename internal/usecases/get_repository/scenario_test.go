package get_repository

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	repositories map[models.ID]*models.Repository
	citations    map[models.ID]*models.Citation
}

func (f *fakeRepo) GetRepository(_ context.Context, id models.ID) (*models.Repository, error) {
	s, ok := f.repositories[id]
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

func TestGetRepositoryReturnsRecord(t *testing.T) {
	id := models.ID("R-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{repositories: map[models.ID]*models.Repository{id: {ID: id, Name: "ГАВО"}}}

	got, err := New(repo).GetRepository(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}

	if got.Name != "ГАВО" {
		t.Fatalf("Name = %q", got.Name)
	}
}

func TestGetRepositoryNotFound(t *testing.T) {
	repo := &fakeRepo{repositories: map[models.ID]*models.Repository{}}

	_, err := New(repo).GetRepository(context.Background(), models.AccessFull, "R-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetRepositoryInvalidID(t *testing.T) {
	repo := &fakeRepo{repositories: map[models.ID]*models.Repository{}}

	_, err := New(repo).GetRepository(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestGetRepositoryPrivateHiddenFromPublic(t *testing.T) {
	id := models.ID("R-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{repositories: map[models.ID]*models.Repository{id: {ID: id, Name: "ГАВО", Private: true}}}

	_, err := New(repo).GetRepository(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for private record with AccessPublic", err)
	}
}

func TestGetRepositoryPrivateVisibleToFullAccess(t *testing.T) {
	id := models.ID("R-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{repositories: map[models.ID]*models.Repository{id: {ID: id, Name: "ГАВО", Private: true}}}

	got, err := New(repo).GetRepository(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}

	if got.Name != "ГАВО" {
		t.Fatalf("Name = %q", got.Name)
	}
}

// TestGetRepositoryHidesRecordReferencingPrivateCitation: запись сама не
// приватна, но ссылается (Sources[i].CitationID) на приватную цитату — для
// вызывающего без полного доступа она прячется как отсутствующая.
func TestGetRepositoryHidesRecordReferencingPrivateCitation(t *testing.T) {
	id := models.ID("R-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	citationID := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		repositories: map[models.ID]*models.Repository{
			id: {ID: id, Name: "ГАВО", Sources: []models.SourceLink{{CitationID: citationID}}},
		},
		citations: map[models.ID]*models.Citation{citationID: {ID: citationID, Private: true}},
	}

	_, err := New(repo).GetRepository(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for record referencing private citation", err)
	}
}

// TestGetRepositoryShowsRecordReferencingPrivateCitationWithFullAccess: тот
// же случай, но для вызывающего с полным доступом запись видима.
func TestGetRepositoryShowsRecordReferencingPrivateCitationWithFullAccess(t *testing.T) {
	id := models.ID("R-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	citationID := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		repositories: map[models.ID]*models.Repository{
			id: {ID: id, Name: "ГАВО", Sources: []models.SourceLink{{CitationID: citationID}}},
		},
		citations: map[models.ID]*models.Citation{citationID: {ID: citationID, Private: true}},
	}

	got, err := New(repo).GetRepository(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}

	if got.Name != "ГАВО" {
		t.Fatalf("Name = %q", got.Name)
	}
}
