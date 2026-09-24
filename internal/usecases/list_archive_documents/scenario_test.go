package list_archive_documents

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

// fakeRepo отдаёт окна списка как настоящий репозиторий.
type fakeRepo struct {
	list      []*models.ArchiveDocument
	err       error
	calls     []models.Page
	citations map[models.ID]*models.Citation
}

// window нарезает список по окну, как настоящий репозиторий.
func window(list []*models.ArchiveDocument, page models.Page) []*models.ArchiveDocument {
	page = page.Normalized()
	if page.Offset >= len(list) {
		return nil
	}

	return list[page.Offset:min(page.Offset+page.Limit, len(list))]
}

func (f *fakeRepo) ListArchiveDocuments(_ context.Context, _ models.Access, page models.Page) ([]*models.ArchiveDocument, error) {
	f.calls = append(f.calls, page)

	if f.err != nil {
		return nil, f.err
	}

	return window(f.list, page), nil
}

func (f *fakeRepo) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return c, nil
}

func TestListArchiveDocumentsReturnsRecords(t *testing.T) {
	repo := &fakeRepo{list: []*models.ArchiveDocument{{ID: "DC-01ARZ3NDEKTSV4RRFFQ69G5FA1", Title: "Метрическая книга 1890"}}}

	got, err := New(repo).ListArchiveDocuments(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListArchiveDocuments: %v", err)
	}

	if len(got) != 1 || got[0].Title != "Метрическая книга 1890" {
		t.Fatalf("got = %+v", got)
	}
}

func TestListArchiveDocumentsRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListArchiveDocuments(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}

	if len(repo.calls) != 0 {
		t.Fatalf("репозиторий вызван %d раз при некорректном page", len(repo.calls))
	}
}

func TestListArchiveDocumentsRejectsNegativeOffset(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListArchiveDocuments(context.Background(), models.AccessFull, models.Page{Offset: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "offset" {
		t.Fatalf("err = %v, want ValidationError on offset", err)
	}
}

func TestListArchiveDocumentsEmptyRepoGivesEmptyNotNil(t *testing.T) {
	got, err := New(&fakeRepo{}).ListArchiveDocuments(context.Background(), models.AccessFull, models.Page{})
	if err != nil {
		t.Fatalf("ListArchiveDocuments: %v", err)
	}

	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v, ожидался пустой не-nil срез", got)
	}
}

func TestListArchiveDocumentsPropagatesRepoError(t *testing.T) {
	wantErr := errors.New("repo down")

	if _, err := New(&fakeRepo{err: wantErr}).ListArchiveDocuments(context.Background(), models.AccessFull, models.Page{}); !errors.Is(err, wantErr) {
		t.Errorf("err=%v, want %v", err, wantErr)
	}
}

// TestListArchiveDocumentsHidesRecordReferencingPrivateCitation: запись
// сама не приватна, но ссылается (Sources[i].CitationID) на приватную
// цитату — для вызывающего без полного доступа она исключается из списка.
func TestListArchiveDocumentsHidesRecordReferencingPrivateCitation(t *testing.T) {
	visibleID := models.ID("DC-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	hiddenID := models.ID("DC-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	citationID := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	repo := &fakeRepo{
		list: []*models.ArchiveDocument{
			{ID: hiddenID, Title: "Скрытый", Sources: []models.SourceLink{{CitationID: citationID}}},
			{ID: visibleID, Title: "Видимый"},
		},
		citations: map[models.ID]*models.Citation{citationID: {ID: citationID, Private: true}},
	}

	got, err := New(repo).ListArchiveDocuments(context.Background(), models.AccessPublic, models.Page{})
	if err != nil {
		t.Fatalf("ListArchiveDocuments: %v", err)
	}

	if len(got) != 1 || got[0].ID != visibleID {
		t.Fatalf("got = %+v, want only visible record", got)
	}
}

// TestListArchiveDocumentsShowsRecordReferencingPrivateCitationWithFullAccess:
// тот же случай, но для вызывающего с полным доступом запись видима.
func TestListArchiveDocumentsShowsRecordReferencingPrivateCitationWithFullAccess(t *testing.T) {
	hiddenID := models.ID("DC-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	citationID := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	repo := &fakeRepo{
		list: []*models.ArchiveDocument{
			{ID: hiddenID, Title: "Скрытый", Sources: []models.SourceLink{{CitationID: citationID}}},
		},
		citations: map[models.ID]*models.Citation{citationID: {ID: citationID, Private: true}},
	}

	got, err := New(repo).ListArchiveDocuments(context.Background(), models.AccessFull, models.Page{})
	if err != nil {
		t.Fatalf("ListArchiveDocuments: %v", err)
	}

	if len(got) != 1 || got[0].ID != hiddenID {
		t.Fatalf("got = %+v, want record visible with full access", got)
	}
}

// TestListArchiveDocumentsPagesWithoutDuplicatesWhenHiddenBeforeOffset:
// регрессия на баг, ранее исправленный в search_events — запись, скрытая по
// приватной цитате и предшествующая offset, не должна «съедать»
// offset-бюджет наравне с видимыми, иначе соседние страницы начинают
// дублировать/терять записи.
func TestListArchiveDocumentsPagesWithoutDuplicatesWhenHiddenBeforeOffset(t *testing.T) {
	hiddenID := models.ID("DC-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	idA := models.ID("DC-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	idB := models.ID("DC-01ARZ3NDEKTSV4RRFFQ69G5FA3")
	citationID := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	repo := &fakeRepo{
		list: []*models.ArchiveDocument{
			{ID: hiddenID, Title: "Скрытый", Sources: []models.SourceLink{{CitationID: citationID}}},
			{ID: idA, Title: "A"},
			{ID: idB, Title: "B"},
		},
		citations: map[models.ID]*models.Citation{citationID: {ID: citationID, Private: true}},
	}

	page1, err := New(repo).ListArchiveDocuments(context.Background(), models.AccessPublic, models.Page{Limit: 1, Offset: 0})
	if err != nil || len(page1) != 1 || page1[0].ID != idA {
		t.Fatalf("page1 = %+v, %v; want [A]", page1, err)
	}

	page2, err := New(repo).ListArchiveDocuments(context.Background(), models.AccessPublic, models.Page{Limit: 1, Offset: 1})
	if err != nil || len(page2) != 1 || page2[0].ID != idB {
		t.Fatalf("page2 = %+v, %v; want [B]", page2, err)
	}
}
