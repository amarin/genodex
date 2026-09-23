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
	hits     []models.Hit
	getErr   map[models.ID]error
	err      error
	errAt    int
	gotQuery string
	calls    []models.Page
	getCalls []models.ID
	accesses []models.Access
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

	return archiveDocument(string(id)), nil
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
