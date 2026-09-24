package list_notes

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	list      []*models.Note
	page      models.Page
	citations map[models.ID]*models.Citation
}

func window(list []*models.Note, page models.Page) []*models.Note {
	page = page.Normalized()
	if page.Offset >= len(list) {
		return nil
	}

	return list[page.Offset:min(page.Offset+page.Limit, len(list))]
}

func (f *fakeRepo) ListNotes(_ context.Context, _ models.Access, page models.Page) ([]*models.Note, error) {
	f.page = page

	return window(f.list, page), nil
}

func (f *fakeRepo) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return c, nil
}

func TestListNotesReturnsRecords(t *testing.T) {
	repo := &fakeRepo{list: []*models.Note{{ID: "N-01ARZ3NDEKTSV4RRFFQ69G5FA1", Kind: "note", Text: "текст"}}}

	got, err := New(repo).ListNotes(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}

	if len(got) != 1 || got[0].Text != "текст" {
		t.Fatalf("got = %+v", got)
	}
}

func TestListNotesRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListNotes(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}

// TestListNotesHidesNoteReferencingPrivateCitation: заметка сама по себе не
// приватна, но ссылается (Sources[i].CitationID) на приватную цитату — для
// вызывающего без полного доступа она исключается из списка.
func TestListNotesHidesNoteReferencingPrivateCitation(t *testing.T) {
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		list: []*models.Note{
			{ID: "N-01ARZ3NDEKTSV4RRFFQ69G5FA1", Sources: []models.SourceLink{{CitationID: citation}}},
			{ID: "N-01ARZ3NDEKTSV4RRFFQ69G5FA2"},
		},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	got, err := New(repo).ListNotes(context.Background(), models.AccessPublic, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}

	if len(got) != 1 || got[0].ID != "N-01ARZ3NDEKTSV4RRFFQ69G5FA2" {
		t.Fatalf("got = %+v, want only the note without a private citation", got)
	}
}

// TestListNotesShowsNoteReferencingPrivateCitationWithFullAccess: тот же
// набор данных, но для вызывающего с полным доступом — обе заметки видимы.
func TestListNotesShowsNoteReferencingPrivateCitationWithFullAccess(t *testing.T) {
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		list: []*models.Note{
			{ID: "N-01ARZ3NDEKTSV4RRFFQ69G5FA1", Sources: []models.SourceLink{{CitationID: citation}}},
			{ID: "N-01ARZ3NDEKTSV4RRFFQ69G5FA2"},
		},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	got, err := New(repo).ListNotes(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("got = %+v, want both notes with full access", got)
	}
}

// TestListNotesPagesWithoutDuplicatesWhenHitHiddenByCitation: из двух
// заметок первая скрыта приватной цитатой — offset=0/limit=1 дважды подряд
// не должен ни дублировать, ни терять вторую (видимую) заметку (по образцу
// list_relations/scenario_test.go:TestListRelationsPagesWithoutDuplicatesWhenHitHiddenByCitation).
func TestListNotesPagesWithoutDuplicatesWhenHitHiddenByCitation(t *testing.T) {
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		list: []*models.Note{
			{ID: "N-01ARZ3NDEKTSV4RRFFQ69G5FA1", Sources: []models.SourceLink{{CitationID: citation}}},
			{ID: "N-01ARZ3NDEKTSV4RRFFQ69G5FA2"},
		},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	page1, err := New(repo).ListNotes(context.Background(), models.AccessPublic, models.Page{Limit: 1, Offset: 0})
	if err != nil {
		t.Fatalf("ListNotes (page1): %v", err)
	}

	if len(page1) != 1 || page1[0].ID != "N-01ARZ3NDEKTSV4RRFFQ69G5FA2" {
		t.Fatalf("page1 = %+v, want [N2]", page1)
	}

	page2, err := New(repo).ListNotes(context.Background(), models.AccessPublic, models.Page{Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("ListNotes (page2): %v", err)
	}

	if len(page2) != 0 {
		t.Fatalf("page2 = %+v, want empty (единственная видимая заметка уже была на page1)", page2)
	}
}
