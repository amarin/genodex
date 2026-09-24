package search_parishes

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits      []models.Hit
	parishes  map[models.ID]*models.Parish
	citations map[models.ID]*models.Citation
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
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

func TestSearchParishesFiltersByType(t *testing.T) {
	id := models.ID("PR-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeParish, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не parish — должен быть пропущен
		},
		parishes: map[models.ID]*models.Parish{id: {ID: id, Name: "Никольский приход"}},
	}

	got, err := New(repo).SearchParishes(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchParishes: %v", err)
	}

	if len(got) != 1 || got[0].Name != "Никольский приход" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchParishesEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchParishes(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchParishes: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}

// TestSearchParishesHidesReferenceToPrivateCitation: хит, чья запись
// ссылается на приватную цитату среди источников, исключается из
// результата для вызывающего без полного доступа, но видна с AccessFull.
func TestSearchParishesHidesReferenceToPrivateCitation(t *testing.T) {
	hiddenID := models.ID("PR-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	visibleID := models.ID("PR-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	citID := models.ID("CI-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeParish, ID: hiddenID},
			{Type: models.TypeParish, ID: visibleID},
		},
		parishes: map[models.ID]*models.Parish{
			hiddenID:  {ID: hiddenID, Name: "Скрытый приход", Sources: []models.SourceLink{{CitationID: citID}}},
			visibleID: {ID: visibleID, Name: "Видимый приход"},
		},
		citations: map[models.ID]*models.Citation{citID: {ID: citID, Private: true}},
	}

	got, err := New(repo).SearchParishes(context.Background(), models.AccessPublic, models.SearchQuery{Text: "п"})
	if err != nil || len(got) != 1 || got[0].ID != visibleID {
		t.Fatalf("got %+v, %v; ожидалась только видимая запись", got, err)
	}

	got, err = New(repo).SearchParishes(context.Background(), models.AccessFull, models.SearchQuery{Text: "п"})
	if err != nil || len(got) != 2 {
		t.Fatalf("got %+v, %v; с AccessFull ожидались обе записи", got, err)
	}
}

// TestSearchParishesPagesWithoutDuplicatesWhenHitBeforeOffsetIsHidden: хит,
// скрытый приватной цитатой и предшествующий запрошенному offset, не должен
// «съедать» offset-бюджет вслепую.
func TestSearchParishesPagesWithoutDuplicatesWhenHitBeforeOffsetIsHidden(t *testing.T) {
	hiddenID := models.ID("PR-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	idA := models.ID("PR-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	idB := models.ID("PR-01ARZ3NDEKTSV4RRFFQ69G5FA3")
	citID := models.ID("CI-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeParish, ID: hiddenID},
			{Type: models.TypeParish, ID: idA},
			{Type: models.TypeParish, ID: idB},
		},
		parishes: map[models.ID]*models.Parish{
			hiddenID: {ID: hiddenID, Name: "Скрытый приход", Sources: []models.SourceLink{{CitationID: citID}}},
			idA:      {ID: idA, Name: "A"},
			idB:      {ID: idB, Name: "B"},
		},
		citations: map[models.ID]*models.Citation{citID: {ID: citID, Private: true}},
	}

	page1, err := New(repo).SearchParishes(context.Background(), models.AccessPublic,
		models.SearchQuery{Text: "п", Page: models.Page{Limit: 1, Offset: 0}})
	if err != nil || len(page1) != 1 || page1[0].ID != idA {
		t.Fatalf("page1 = %+v, %v; want [%v]", page1, err, idA)
	}

	page2, err := New(repo).SearchParishes(context.Background(), models.AccessPublic,
		models.SearchQuery{Text: "п", Page: models.Page{Limit: 1, Offset: 1}})
	if err != nil || len(page2) != 1 || page2[0].ID != idB {
		t.Fatalf("page2 = %+v, %v; want [%v]", page2, err, idB)
	}
}
