package search_families

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits      []models.Hit
	families  map[models.ID]*models.Family
	citations map[models.ID]*models.Citation
}

// Search эмулирует хранилищное окно generic-индекса: возвращает срез
// f.hits[page.Offset : page.Offset+page.Limit], а не только "первое окно" —
// без этого фейк не может воспроизвести реальную механику сканирования
// SearchFamilies по MaxPageLimit-окнам (см. search_events/scenario_test.go).
func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset >= len(f.hits) {
		return nil, nil
	}

	end := page.Offset + page.Limit
	if end > len(f.hits) {
		end = len(f.hits)
	}

	return f.hits[page.Offset:end], nil
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

func TestSearchFamiliesFiltersByType(t *testing.T) {
	id := models.ID("F-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeFamily, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не family — должен быть пропущен
		},
		families: map[models.ID]*models.Family{id: {ID: id, Name: "Ивановы"}},
	}

	got, err := New(repo).SearchFamilies(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchFamilies: %v", err)
	}

	if len(got) != 1 || got[0].Name != "Ивановы" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchFamiliesEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchFamilies(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchFamilies: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}

// TestSearchFamiliesHidesHitReferencingPrivateCitation: найденный род сам по
// себе не приватен, но ссылается (Sources[i].CitationID) на приватную
// цитату — для вызывающего без полного доступа он исключается из результата
// поиска.
func TestSearchFamiliesHidesHitReferencingPrivateCitation(t *testing.T) {
	id := models.ID("F-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		hits: []models.Hit{{Type: models.TypeFamily, ID: id}},
		families: map[models.ID]*models.Family{
			id: {ID: id, Name: "Ивановы", Sources: []models.SourceLink{{CitationID: citation}}},
		},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	got, err := New(repo).SearchFamilies(context.Background(), models.AccessPublic, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchFamilies: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty (ссылка на приватную цитату)", got)
	}
}

// TestSearchFamiliesShowsHitReferencingPrivateCitationWithFullAccess: тот же
// род, но для вызывающего с полным доступом — находится.
func TestSearchFamiliesShowsHitReferencingPrivateCitationWithFullAccess(t *testing.T) {
	id := models.ID("F-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		hits: []models.Hit{{Type: models.TypeFamily, ID: id}},
		families: map[models.ID]*models.Family{
			id: {ID: id, Name: "Ивановы", Sources: []models.SourceLink{{CitationID: citation}}},
		},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	got, err := New(repo).SearchFamilies(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchFamilies: %v", err)
	}

	if len(got) != 1 || got[0].ID != id {
		t.Fatalf("got = %+v", got)
	}
}

// TestSearchFamiliesPagesWithoutDuplicatesWhenHitBeforeOffsetIsHidden: хит,
// предшествующий запрошенному offset, скрыт (ссылается на приватную
// цитату). Порядок хитов: [hidden, A, B]. Постранично (limit=1) с offset=0 и
// offset=1 должны вернуться разные видимые роды — A, затем B — без дублей и
// без пропусков (по образцу
// search_events/scenario_test.go:TestSearchEventsPagesWithoutDuplicatesWhenHitBeforeOffsetIsHidden).
func TestSearchFamiliesPagesWithoutDuplicatesWhenHitBeforeOffsetIsHidden(t *testing.T) {
	hiddenID := models.ID("F-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	idA := models.ID("F-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	idB := models.ID("F-01ARZ3NDEKTSV4RRFFQ69G5FA3")
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeFamily, ID: hiddenID},
			{Type: models.TypeFamily, ID: idA},
			{Type: models.TypeFamily, ID: idB},
		},
		families: map[models.ID]*models.Family{
			hiddenID: {ID: hiddenID, Name: "Ивановы-0", Sources: []models.SourceLink{{CitationID: citation}}},
			idA:      {ID: idA, Name: "Ивановы-A"},
			idB:      {ID: idB, Name: "Ивановы-B"},
		},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	scenario := New(repo)

	page1, err := scenario.SearchFamilies(context.Background(), models.AccessPublic,
		models.SearchQuery{Text: "Ив", Page: models.Page{Limit: 1, Offset: 0}})
	if err != nil {
		t.Fatalf("SearchFamilies (page1): %v", err)
	}

	if len(page1) != 1 || page1[0].ID != idA {
		t.Fatalf("page1 = %+v, want [A]", page1)
	}

	page2, err := scenario.SearchFamilies(context.Background(), models.AccessPublic,
		models.SearchQuery{Text: "Ив", Page: models.Page{Limit: 1, Offset: 1}})
	if err != nil {
		t.Fatalf("SearchFamilies (page2): %v", err)
	}

	if len(page2) != 1 || page2[0].ID != idB {
		t.Fatalf("page2 = %+v, want [B] (не дубликат page1!)", page2)
	}
}
