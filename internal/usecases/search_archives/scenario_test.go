package search_archives

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits     []models.Hit
	archives map[models.ID]*models.Archive
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetArchive(_ context.Context, id models.ID) (*models.Archive, error) {
	s, ok := f.archives[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestSearchArchivesFiltersByType(t *testing.T) {
	id := models.ID("AR-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeArchive, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не archive — должен быть пропущен
		},
		archives: map[models.ID]*models.Archive{id: {ID: id, Name: "ГАВО, архив"}},
	}

	got, err := New(repo).SearchArchives(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchArchives: %v", err)
	}

	if len(got) != 1 || got[0].Name != "ГАВО, архив" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchArchivesEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchArchives(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchArchives: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
