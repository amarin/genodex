package search_archive_documents

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

func hit(id string, typ models.Type) models.Hit {
	return models.Hit{Type: typ, ID: models.ID(id), Label: id}
}

func archiveDocument(id string) *models.ArchiveDocument {
	return &models.ArchiveDocument{ID: models.ID(id), Title: id}
}

func ids(docs []models.ArchiveDocument) []models.ID {
	out := make([]models.ID, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.ID)
	}

	return out
}

func sameIDs[T ~string](got []T, want ...T) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}

	return true
}

type fakeRepo struct {
	hits      []models.Hit
	getErr    map[models.ID]error
	err       error
	errAt     int
	gotQuery  string
	calls     []models.Page
	getCalls  []models.ID
	accesses  []models.Access
	docs      map[models.ID]*models.ArchiveDocument
	citations map[models.ID]*models.Citation
}

func (f *fakeRepo) Search(_ context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error) {
	f.gotQuery = query
	f.calls = append(f.calls, page)
	f.accesses = append(f.accesses, access)

	if f.err != nil && (f.errAt == 0 || f.errAt == len(f.calls)) {
		return nil, f.err
	}

	page = page.Normalized()
	if page.Offset >= len(f.hits) {
		return nil, nil
	}

	end := min(page.Offset+page.Limit, len(f.hits))

	return f.hits[page.Offset:end], nil
}

func (f *fakeRepo) GetArchiveDocument(_ context.Context, id models.ID) (*models.ArchiveDocument, error) {
	f.getCalls = append(f.getCalls, id)

	if f.getErr != nil {
		if e, ok := f.getErr[id]; ok && e != nil {
			return nil, e
		}
	}

	if d, ok := f.docs[id]; ok {
		return d, nil
	}

	return archiveDocument(string(id)), nil
}

func (f *fakeRepo) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return c, nil
}

func TestSearchArchiveDocumentsEmptyTextDoesNotTouchRepo(t *testing.T) {
	repo := &fakeRepo{}

	got, err := New(repo).SearchArchiveDocuments(context.Background(), models.AccessFull, models.SearchQuery{Text: "   "})
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v; ожидался пустой результат", got, err)
	}
	if len(repo.calls) != 0 {
		t.Fatalf("Search вызван %d раз при пустом тексте", len(repo.calls))
	}
}

func TestSearchArchiveDocumentsReturnsFullDocumentsFromHits(t *testing.T) {
	repo := &fakeRepo{
		hits: []models.Hit{
			hit("D-1001", models.TypePerson),
			hit("DC-1002", models.TypeArchiveDocument),
			hit("F-1003", models.TypeFamily),
			hit("DC-1004", models.TypeArchiveDocument),
		},
	}

	got, err := New(repo).SearchArchiveDocuments(context.Background(), models.AccessFull, models.SearchQuery{Text: "метрич"})
	if err != nil || !sameIDs(ids(got), "DC-1002", "DC-1004") {
		t.Fatalf("got %v, %v; ожидались DC-1002, DC-1004", ids(got), err)
	}
	if repo.gotQuery != "метрич" {
		t.Fatalf("Search получил %q, ожидался %q", repo.gotQuery, "метрич")
	}
}

func TestSearchArchiveDocumentsAppliesWindowAmongHits(t *testing.T) {
	repo := &fakeRepo{
		hits: []models.Hit{
			hit("D-1001", models.TypePerson),
			hit("DC-1002", models.TypeArchiveDocument),
			hit("DC-1003", models.TypeArchiveDocument),
			hit("DC-1004", models.TypeArchiveDocument),
		},
	}

	got, err := New(repo).SearchArchiveDocuments(context.Background(), models.AccessFull, models.SearchQuery{
		Text: "м",
		Page: models.Page{Limit: 1, Offset: 1},
	})
	if err != nil || !sameIDs(ids(got), "DC-1003") {
		t.Fatalf("got %v, %v; ожидалась DC-1003 (второй хит)", ids(got), err)
	}
}

func TestSearchArchiveDocumentsSkipsDeletedDocument(t *testing.T) {
	repo := &fakeRepo{
		hits: []models.Hit{
			hit("DC-1002", models.TypeArchiveDocument),
			hit("DC-1003", models.TypeArchiveDocument),
		},
		getErr: map[models.ID]error{"DC-1002": models.ErrNotFound},
	}

	got, err := New(repo).SearchArchiveDocuments(context.Background(), models.AccessFull, models.SearchQuery{Text: "м"})
	if err != nil {
		t.Fatalf("SearchArchiveDocuments: %v", err)
	}
	if !sameIDs(ids(got), "DC-1003") {
		t.Fatalf("got %v; DC-1002 удалён — остаётся DC-1003", ids(got))
	}
}

func TestSearchArchiveDocumentsPropagatesGetError(t *testing.T) {
	boom := errors.New("get boom")
	repo := &fakeRepo{
		hits:   []models.Hit{hit("DC-1002", models.TypeArchiveDocument)},
		getErr: map[models.ID]error{"DC-1002": boom},
	}

	_, err := New(repo).SearchArchiveDocuments(context.Background(), models.AccessFull, models.SearchQuery{Text: "м"})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, ожидался boom", err)
	}
}

func TestSearchArchiveDocumentsPropagatesSearchError(t *testing.T) {
	boom := errors.New("search boom")
	repo := &fakeRepo{err: boom}

	_, err := New(repo).SearchArchiveDocuments(context.Background(), models.AccessFull, models.SearchQuery{Text: "м"})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, ожидался boom", err)
	}
}

func TestSearchArchiveDocumentsInvalidQueryDoesNotTouchRepo(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).SearchArchiveDocuments(context.Background(), models.AccessFull, models.SearchQuery{Page: models.Page{Limit: -1}})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, ожидалась *ValidationError поля limit", err)
	}
	if len(repo.calls) != 0 {
		t.Fatalf("Search вызван %d раз при некорректном запросе", len(repo.calls))
	}
}

func TestSearchArchiveDocumentsPassesAccessToRepo(t *testing.T) {
	repo := &fakeRepo{hits: []models.Hit{
		hit("DC-1002", models.TypeArchiveDocument),
	}}

	if _, err := New(repo).SearchArchiveDocuments(context.Background(), models.AccessPublic,
		models.SearchQuery{Text: "м"}); err != nil {
		t.Fatalf("SearchArchiveDocuments: %v", err)
	}

	for i, a := range repo.accesses {
		if a != models.AccessPublic {
			t.Errorf("вызов %d: доступ %v, ожидался AccessPublic", i+1, a)
		}
	}
}

// TestSearchArchiveDocumentsHidesRecordReferencingPrivateCitation: хит сам
// не приватен, но ссылается (Sources[i].CitationID) на приватную цитату —
// для вызывающего без полного доступа он исключается из результата.
func TestSearchArchiveDocumentsHidesRecordReferencingPrivateCitation(t *testing.T) {
	id := models.ID("DC-1002")
	citationID := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		hits: []models.Hit{hit("DC-1002", models.TypeArchiveDocument)},
		docs: map[models.ID]*models.ArchiveDocument{
			id: {ID: id, Title: "Метрическая книга", Sources: []models.SourceLink{{CitationID: citationID}}},
		},
		citations: map[models.ID]*models.Citation{citationID: {ID: citationID, Private: true}},
	}

	got, err := New(repo).SearchArchiveDocuments(context.Background(), models.AccessPublic, models.SearchQuery{Text: "м"})
	if err != nil {
		t.Fatalf("SearchArchiveDocuments: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty (record references private citation)", got)
	}
}

// TestSearchArchiveDocumentsShowsRecordReferencingPrivateCitationWithFullAccess:
// тот же случай, но для вызывающего с полным доступом запись видима.
func TestSearchArchiveDocumentsShowsRecordReferencingPrivateCitationWithFullAccess(t *testing.T) {
	id := models.ID("DC-1002")
	citationID := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		hits: []models.Hit{hit("DC-1002", models.TypeArchiveDocument)},
		docs: map[models.ID]*models.ArchiveDocument{
			id: {ID: id, Title: "Метрическая книга", Sources: []models.SourceLink{{CitationID: citationID}}},
		},
		citations: map[models.ID]*models.Citation{citationID: {ID: citationID, Private: true}},
	}

	got, err := New(repo).SearchArchiveDocuments(context.Background(), models.AccessFull, models.SearchQuery{Text: "м"})
	if err != nil {
		t.Fatalf("SearchArchiveDocuments: %v", err)
	}

	if len(got) != 1 || got[0].ID != id {
		t.Fatalf("got = %+v, want record visible with full access", got)
	}
}

// TestSearchArchiveDocumentsPagesWithoutDuplicatesWhenHitBeforeOffsetIsHidden:
// регрессия на баг, ранее исправленный в search_events — хит, скрытый по
// приватной цитате и предшествующий offset, не должен "съедать"
// offset-бюджет наравне с видимыми хитами. Порядок хитов:
// [hidden, A (public), B (public)]. Постранично (limit=1) с offset=0 и
// offset=1 должны вернуться разные видимые записи — A, затем B — без
// дублей и без пропусков.
func TestSearchArchiveDocumentsPagesWithoutDuplicatesWhenHitBeforeOffsetIsHidden(t *testing.T) {
	hiddenID := models.ID("DC-1001")
	idA := models.ID("DC-1002")
	idB := models.ID("DC-1003")
	citationID := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	repo := &fakeRepo{
		hits: []models.Hit{
			hit("DC-1001", models.TypeArchiveDocument),
			hit("DC-1002", models.TypeArchiveDocument),
			hit("DC-1003", models.TypeArchiveDocument),
		},
		docs: map[models.ID]*models.ArchiveDocument{
			hiddenID: {ID: hiddenID, Title: "Скрытый", Sources: []models.SourceLink{{CitationID: citationID}}},
			idA:      {ID: idA, Title: "A"},
			idB:      {ID: idB, Title: "B"},
		},
		citations: map[models.ID]*models.Citation{citationID: {ID: citationID, Private: true}},
	}

	scenario := New(repo)

	page1, err := scenario.SearchArchiveDocuments(context.Background(), models.AccessPublic,
		models.SearchQuery{Text: "м", Page: models.Page{Limit: 1, Offset: 0}})
	if err != nil {
		t.Fatalf("SearchArchiveDocuments (page1): %v", err)
	}

	if len(page1) != 1 || page1[0].ID != idA {
		t.Fatalf("page1 = %+v, want [A]", page1)
	}

	page2, err := scenario.SearchArchiveDocuments(context.Background(), models.AccessPublic,
		models.SearchQuery{Text: "м", Page: models.Page{Limit: 1, Offset: 1}})
	if err != nil {
		t.Fatalf("SearchArchiveDocuments (page2): %v", err)
	}

	if len(page2) != 1 || page2[0].ID != idB {
		t.Fatalf("page2 = %+v, want [B]", page2)
	}
}
