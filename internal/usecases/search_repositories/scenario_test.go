package search_repositories

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits         []models.Hit
	repositories map[models.ID]*models.Repository
	citations    map[models.ID]*models.Citation
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
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

func TestSearchRepositoriesFiltersByType(t *testing.T) {
	id := models.ID("R-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeRepository, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не repository — должен быть пропущен
		},
		repositories: map[models.ID]*models.Repository{id: {ID: id, Name: "ГАВО"}},
	}

	got, err := New(repo).SearchRepositories(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchRepositories: %v", err)
	}

	if len(got) != 1 || got[0].Name != "ГАВО" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchRepositoriesEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchRepositories(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchRepositories: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}

// TestSearchRepositoriesHidesRecordReferencingPrivateCitation: хит сам не
// приватен, но ссылается (Sources[i].CitationID) на приватную цитату — для
// вызывающего без полного доступа он исключается из результата.
func TestSearchRepositoriesHidesRecordReferencingPrivateCitation(t *testing.T) {
	id := models.ID("R-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	citationID := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		hits: []models.Hit{{Type: models.TypeRepository, ID: id}},
		repositories: map[models.ID]*models.Repository{
			id: {ID: id, Name: "ГАВО", Sources: []models.SourceLink{{CitationID: citationID}}},
		},
		citations: map[models.ID]*models.Citation{citationID: {ID: citationID, Private: true}},
	}

	got, err := New(repo).SearchRepositories(context.Background(), models.AccessPublic, models.SearchQuery{Text: "ГАВ"})
	if err != nil {
		t.Fatalf("SearchRepositories: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty (record references private citation)", got)
	}
}

// TestSearchRepositoriesShowsRecordReferencingPrivateCitationWithFullAccess:
// тот же случай, но для вызывающего с полным доступом запись видима.
func TestSearchRepositoriesShowsRecordReferencingPrivateCitationWithFullAccess(t *testing.T) {
	id := models.ID("R-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	citationID := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		hits: []models.Hit{{Type: models.TypeRepository, ID: id}},
		repositories: map[models.ID]*models.Repository{
			id: {ID: id, Name: "ГАВО", Sources: []models.SourceLink{{CitationID: citationID}}},
		},
		citations: map[models.ID]*models.Citation{citationID: {ID: citationID, Private: true}},
	}

	got, err := New(repo).SearchRepositories(context.Background(), models.AccessFull, models.SearchQuery{Text: "ГАВ"})
	if err != nil {
		t.Fatalf("SearchRepositories: %v", err)
	}

	if len(got) != 1 || got[0].ID != id {
		t.Fatalf("got = %+v, want record visible with full access", got)
	}
}

// TestSearchRepositoriesPagesWithoutDuplicatesWhenHitBeforeOffsetIsHidden:
// регрессия на баг, ранее исправленный в search_events — хит, скрытый по
// приватной цитате и предшествующий offset, не должен "съедать"
// offset-бюджет наравне с видимыми хитами. Порядок хитов:
// [hidden, A (public), B (public)]. Постранично (limit=1) с offset=0 и
// offset=1 должны вернуться разные видимые записи — A, затем B — без
// дублей и без пропусков.
func TestSearchRepositoriesPagesWithoutDuplicatesWhenHitBeforeOffsetIsHidden(t *testing.T) {
	hiddenID := models.ID("R-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	idA := models.ID("R-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	idB := models.ID("R-01ARZ3NDEKTSV4RRFFQ69G5FA3")
	citationID := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeRepository, ID: hiddenID},
			{Type: models.TypeRepository, ID: idA},
			{Type: models.TypeRepository, ID: idB},
		},
		repositories: map[models.ID]*models.Repository{
			hiddenID: {ID: hiddenID, Name: "Скрытый", Sources: []models.SourceLink{{CitationID: citationID}}},
			idA:      {ID: idA, Name: "A"},
			idB:      {ID: idB, Name: "B"},
		},
		citations: map[models.ID]*models.Citation{citationID: {ID: citationID, Private: true}},
	}

	scenario := New(repo)

	page1, err := scenario.SearchRepositories(context.Background(), models.AccessPublic,
		models.SearchQuery{Text: "х", Page: models.Page{Limit: 1, Offset: 0}})
	if err != nil {
		t.Fatalf("SearchRepositories (page1): %v", err)
	}

	if len(page1) != 1 || page1[0].ID != idA {
		t.Fatalf("page1 = %+v, want [A]", page1)
	}

	page2, err := scenario.SearchRepositories(context.Background(), models.AccessPublic,
		models.SearchQuery{Text: "х", Page: models.Page{Limit: 1, Offset: 1}})
	if err != nil {
		t.Fatalf("SearchRepositories (page2): %v", err)
	}

	if len(page2) != 1 || page2[0].ID != idB {
		t.Fatalf("page2 = %+v, want [B]", page2)
	}
}

func TestSearchRepositoriesSkipsDeletedRecord(t *testing.T) {
	id := models.ID("R-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		hits:         []models.Hit{{Type: models.TypeRepository, ID: id}},
		repositories: map[models.ID]*models.Repository{},
	}

	got, err := New(repo).SearchRepositories(context.Background(), models.AccessFull, models.SearchQuery{Text: "х"})
	if err != nil {
		t.Fatalf("SearchRepositories: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty (record deleted)", got)
	}
}
