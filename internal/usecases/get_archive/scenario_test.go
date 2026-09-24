package get_archive

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	archives  map[models.ID]*models.Archive
	citations map[models.ID]*models.Citation
}

func (f *fakeRepo) GetArchive(_ context.Context, id models.ID) (*models.Archive, error) {
	s, ok := f.archives[id]
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

// TestGetArchiveHidesRecordReferencingPrivateCitation: архив сам не
// приватен, но ссылается (Sources[i].CitationID) на приватную цитату — для
// вызывающего без полного доступа он прячется как отсутствующий, как если
// бы был приватным сам.
func TestGetArchiveHidesRecordReferencingPrivateCitation(t *testing.T) {
	id := models.ID("AR-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	citationID := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		archives: map[models.ID]*models.Archive{
			id: {ID: id, Name: "ГАВО, архив", Sources: []models.SourceLink{{CitationID: citationID}}},
		},
		citations: map[models.ID]*models.Citation{citationID: {ID: citationID, Private: true}},
	}

	_, err := New(repo).GetArchive(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for record referencing private citation", err)
	}
}

// TestGetArchiveShowsRecordReferencingPrivateCitationWithFullAccess: тот
// же случай, но для вызывающего с полным доступом запись видима.
func TestGetArchiveShowsRecordReferencingPrivateCitationWithFullAccess(t *testing.T) {
	id := models.ID("AR-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	citationID := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		archives: map[models.ID]*models.Archive{
			id: {ID: id, Name: "ГАВО, архив", Sources: []models.SourceLink{{CitationID: citationID}}},
		},
		citations: map[models.ID]*models.Citation{citationID: {ID: citationID, Private: true}},
	}

	got, err := New(repo).GetArchive(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetArchive: %v", err)
	}

	if got.Name != "ГАВО, архив" {
		t.Fatalf("Name = %q", got.Name)
	}
}
