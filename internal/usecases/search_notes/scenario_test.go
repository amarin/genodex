package search_notes

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits      []models.Hit
	notes     map[models.ID]*models.Note
	citations map[models.ID]*models.Citation
}

// Search эмулирует хранилищное окно generic-индекса: возвращает срез
// f.hits[page.Offset : page.Offset+page.Limit], а не только "первое окно" —
// без этого фейк не может воспроизвести реальную механику сканирования
// SearchNotes по MaxPageLimit-окнам (см. search_events/scenario_test.go).
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

func (f *fakeRepo) GetNote(_ context.Context, id models.ID) (*models.Note, error) {
	s, ok := f.notes[id]
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

func TestSearchNotesFiltersByType(t *testing.T) {
	id := models.ID("N-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeNote, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не note — должен быть пропущен
		},
		notes: map[models.ID]*models.Note{id: {ID: id, Kind: "note", Text: "текст"}},
	}

	got, err := New(repo).SearchNotes(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchNotes: %v", err)
	}

	if len(got) != 1 || got[0].Text != "текст" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchNotesEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchNotes(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchNotes: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}

// TestSearchNotesHidesHitReferencingPrivateCitation: найденная заметка сама
// по себе не приватна, но ссылается (Sources[i].CitationID) на приватную
// цитату — для вызывающего без полного доступа она исключается из
// результата поиска.
func TestSearchNotesHidesHitReferencingPrivateCitation(t *testing.T) {
	id := models.ID("N-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		hits: []models.Hit{{Type: models.TypeNote, ID: id}},
		notes: map[models.ID]*models.Note{
			id: {ID: id, Kind: "note", Text: "текст", Sources: []models.SourceLink{{CitationID: citation}}},
		},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	got, err := New(repo).SearchNotes(context.Background(), models.AccessPublic, models.SearchQuery{Text: "текст"})
	if err != nil {
		t.Fatalf("SearchNotes: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty (ссылка на приватную цитату)", got)
	}
}

// TestSearchNotesShowsHitReferencingPrivateCitationWithFullAccess: та же
// заметка, но для вызывающего с полным доступом — находится.
func TestSearchNotesShowsHitReferencingPrivateCitationWithFullAccess(t *testing.T) {
	id := models.ID("N-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		hits: []models.Hit{{Type: models.TypeNote, ID: id}},
		notes: map[models.ID]*models.Note{
			id: {ID: id, Kind: "note", Text: "текст", Sources: []models.SourceLink{{CitationID: citation}}},
		},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	got, err := New(repo).SearchNotes(context.Background(), models.AccessFull, models.SearchQuery{Text: "текст"})
	if err != nil {
		t.Fatalf("SearchNotes: %v", err)
	}

	if len(got) != 1 || got[0].ID != id {
		t.Fatalf("got = %+v", got)
	}
}

// TestSearchNotesPagesWithoutDuplicatesWhenHitBeforeOffsetIsHidden: хит,
// предшествующий запрошенному offset, скрыт (ссылается на приватную
// цитату). Порядок хитов: [hidden, A, B]. Постранично (limit=1) с offset=0 и
// offset=1 должны вернуться разные видимые заметки — A, затем B — без
// дублей и без пропусков (по образцу
// search_events/scenario_test.go:TestSearchEventsPagesWithoutDuplicatesWhenHitBeforeOffsetIsHidden).
func TestSearchNotesPagesWithoutDuplicatesWhenHitBeforeOffsetIsHidden(t *testing.T) {
	hiddenID := models.ID("N-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	idA := models.ID("N-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	idB := models.ID("N-01ARZ3NDEKTSV4RRFFQ69G5FA3")
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeNote, ID: hiddenID},
			{Type: models.TypeNote, ID: idA},
			{Type: models.TypeNote, ID: idB},
		},
		notes: map[models.ID]*models.Note{
			hiddenID: {ID: hiddenID, Text: "текст-0", Sources: []models.SourceLink{{CitationID: citation}}},
			idA:      {ID: idA, Text: "текст-A"},
			idB:      {ID: idB, Text: "текст-B"},
		},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	scenario := New(repo)

	page1, err := scenario.SearchNotes(context.Background(), models.AccessPublic,
		models.SearchQuery{Text: "текст", Page: models.Page{Limit: 1, Offset: 0}})
	if err != nil {
		t.Fatalf("SearchNotes (page1): %v", err)
	}

	if len(page1) != 1 || page1[0].ID != idA {
		t.Fatalf("page1 = %+v, want [A]", page1)
	}

	page2, err := scenario.SearchNotes(context.Background(), models.AccessPublic,
		models.SearchQuery{Text: "текст", Page: models.Page{Limit: 1, Offset: 1}})
	if err != nil {
		t.Fatalf("SearchNotes (page2): %v", err)
	}

	if len(page2) != 1 || page2[0].ID != idB {
		t.Fatalf("page2 = %+v, want [B] (не дубликат page1!)", page2)
	}
}
