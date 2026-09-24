package search_archive_nodes

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

// hit строит хиты поиска: только нужные поля.
func hit(id string, typ models.Type) models.Hit {
	return models.Hit{Type: typ, ID: models.ID(id), Label: id}
}

// archiveNode строит узел архивного дерева.
func archiveNode(id string) *models.ArchiveNode {
	return &models.ArchiveNode{ID: models.ID(id), Label: id}
}

// ids извлекает идентификаторы узлов.
func ids(nodes []models.ArchiveNode) []models.ID {
	out := make([]models.ID, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.ID)
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

// fakeRepo отдаёт окна хитов и узлы.
type fakeRepo struct {
	hits      []models.Hit
	getErr    map[models.ID]error
	err       error
	errAt     int
	gotQuery  string
	calls     []models.Page
	getCalls  []models.ID
	accesses  []models.Access
	nodes     map[models.ID]*models.ArchiveNode
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

func (f *fakeRepo) GetArchiveNode(_ context.Context, id models.ID) (*models.ArchiveNode, error) {
	f.getCalls = append(f.getCalls, id)

	if f.getErr != nil {
		if e, ok := f.getErr[id]; ok && e != nil {
			return nil, e
		}
	}

	if n, ok := f.nodes[id]; ok {
		return n, nil
	}

	return archiveNode(string(id)), nil
}

func (f *fakeRepo) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return c, nil
}

func TestSearchArchiveNodesEmptyTextDoesNotTouchRepo(t *testing.T) {
	repo := &fakeRepo{}

	got, err := New(repo).SearchArchiveNodes(context.Background(), models.AccessFull, models.SearchQuery{Text: "   "})
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v; ожидался пустой результат", got, err)
	}
	if len(repo.calls) != 0 {
		t.Fatalf("Search вызван %d раз при пустом тексте", len(repo.calls))
	}
}

func TestSearchArchiveNodesReturnsFullNodesFromHits(t *testing.T) {
	repo := &fakeRepo{
		hits: []models.Hit{
			hit("D-1001", models.TypePerson),
			hit("AN-1002", models.TypeArchiveNode),
			hit("F-1003", models.TypeFamily),
			hit("AN-1004", models.TypeArchiveNode),
		},
	}

	got, err := New(repo).SearchArchiveNodes(context.Background(), models.AccessFull, models.SearchQuery{Text: "фонд"})
	if err != nil || !sameIDs(ids(got), "AN-1002", "AN-1004") {
		t.Fatalf("got %v, %v; ожидались AN-1002, AN-1004", ids(got), err)
	}
	if repo.gotQuery != "фонд" {
		t.Fatalf("Search получил %q, ожидался %q", repo.gotQuery, "фонд")
	}
}

func TestSearchArchiveNodesAppliesWindowAmongHits(t *testing.T) {
	repo := &fakeRepo{
		hits: []models.Hit{
			hit("D-1001", models.TypePerson),
			hit("AN-1002", models.TypeArchiveNode),
			hit("AN-1003", models.TypeArchiveNode),
			hit("AN-1004", models.TypeArchiveNode),
		},
	}

	got, err := New(repo).SearchArchiveNodes(context.Background(), models.AccessFull, models.SearchQuery{
		Text: "п",
		Page: models.Page{Limit: 1, Offset: 1},
	})
	if err != nil || !sameIDs(ids(got), "AN-1003") {
		t.Fatalf("got %v, %v; ожидалась AN-1003 (второй хит)", ids(got), err)
	}
}

func TestSearchArchiveNodesSkipsDeletedNode(t *testing.T) {
	repo := &fakeRepo{
		hits: []models.Hit{
			hit("AN-1002", models.TypeArchiveNode),
			hit("AN-1003", models.TypeArchiveNode),
		},
		getErr: map[models.ID]error{"AN-1002": models.ErrNotFound},
	}

	got, err := New(repo).SearchArchiveNodes(context.Background(), models.AccessFull, models.SearchQuery{Text: "п"})
	if err != nil {
		t.Fatalf("SearchArchiveNodes: %v", err)
	}
	if !sameIDs(ids(got), "AN-1003") {
		t.Fatalf("got %v; AN-1002 удалён — остаётся AN-1003", ids(got))
	}
}

func TestSearchArchiveNodesPropagatesGetError(t *testing.T) {
	boom := errors.New("get boom")
	repo := &fakeRepo{
		hits:   []models.Hit{hit("AN-1002", models.TypeArchiveNode)},
		getErr: map[models.ID]error{"AN-1002": boom},
	}

	_, err := New(repo).SearchArchiveNodes(context.Background(), models.AccessFull, models.SearchQuery{Text: "п"})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, ожидался boom", err)
	}
}

func TestSearchArchiveNodesPropagatesSearchError(t *testing.T) {
	boom := errors.New("search boom")
	repo := &fakeRepo{err: boom}

	_, err := New(repo).SearchArchiveNodes(context.Background(), models.AccessFull, models.SearchQuery{Text: "п"})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, ожидался boom", err)
	}
}

func TestSearchArchiveNodesInvalidQueryDoesNotTouchRepo(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).SearchArchiveNodes(context.Background(), models.AccessFull, models.SearchQuery{Page: models.Page{Limit: -1}})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, ожидалась *ValidationError поля limit", err)
	}
	if len(repo.calls) != 0 {
		t.Fatalf("Search вызван %d раз при некорректном запросе", len(repo.calls))
	}
}

func TestSearchArchiveNodesPassesAccessToRepo(t *testing.T) {
	repo := &fakeRepo{hits: []models.Hit{
		hit("AN-1002", models.TypeArchiveNode),
	}}

	if _, err := New(repo).SearchArchiveNodes(context.Background(), models.AccessPublic,
		models.SearchQuery{Text: "п"}); err != nil {
		t.Fatalf("SearchArchiveNodes: %v", err)
	}

	for i, a := range repo.accesses {
		if a != models.AccessPublic {
			t.Errorf("вызов %d: доступ %v, ожидался AccessPublic", i+1, a)
		}
	}
}

// TestSearchArchiveNodesHidesRecordReferencingPrivateCitation: хит сам не
// приватен, но ссылается (Sources[i].CitationID) на приватную цитату — для
// вызывающего без полного доступа он исключается из результата.
func TestSearchArchiveNodesHidesRecordReferencingPrivateCitation(t *testing.T) {
	id := models.ID("AN-1002")
	citationID := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		hits: []models.Hit{hit("AN-1002", models.TypeArchiveNode)},
		nodes: map[models.ID]*models.ArchiveNode{
			id: {ID: id, Label: "Фонд 1", Sources: []models.SourceLink{{CitationID: citationID}}},
		},
		citations: map[models.ID]*models.Citation{citationID: {ID: citationID, Private: true}},
	}

	got, err := New(repo).SearchArchiveNodes(context.Background(), models.AccessPublic, models.SearchQuery{Text: "ф"})
	if err != nil {
		t.Fatalf("SearchArchiveNodes: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty (record references private citation)", got)
	}
}

// TestSearchArchiveNodesShowsRecordReferencingPrivateCitationWithFullAccess:
// тот же случай, но для вызывающего с полным доступом узел видим.
func TestSearchArchiveNodesShowsRecordReferencingPrivateCitationWithFullAccess(t *testing.T) {
	id := models.ID("AN-1002")
	citationID := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		hits: []models.Hit{hit("AN-1002", models.TypeArchiveNode)},
		nodes: map[models.ID]*models.ArchiveNode{
			id: {ID: id, Label: "Фонд 1", Sources: []models.SourceLink{{CitationID: citationID}}},
		},
		citations: map[models.ID]*models.Citation{citationID: {ID: citationID, Private: true}},
	}

	got, err := New(repo).SearchArchiveNodes(context.Background(), models.AccessFull, models.SearchQuery{Text: "ф"})
	if err != nil {
		t.Fatalf("SearchArchiveNodes: %v", err)
	}

	if !sameIDs(ids(got), id) {
		t.Fatalf("got = %+v, want record visible with full access", ids(got))
	}
}

// TestSearchArchiveNodesPagesWithoutDuplicatesWhenHitBeforeOffsetIsHidden:
// регрессия на баг, ранее исправленный в search_events — хит, скрытый по
// приватной цитате и предшествующий offset, не должен "съедать"
// offset-бюджет наравне с видимыми хитами. Порядок хитов:
// [hidden, A (public), B (public)]. Постранично (limit=1) с offset=0 и
// offset=1 должны вернуться разные видимые записи — A, затем B — без
// дублей и без пропусков.
func TestSearchArchiveNodesPagesWithoutDuplicatesWhenHitBeforeOffsetIsHidden(t *testing.T) {
	hiddenID := models.ID("AN-1001")
	idA := models.ID("AN-1002")
	idB := models.ID("AN-1003")
	citationID := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	repo := &fakeRepo{
		hits: []models.Hit{
			hit("AN-1001", models.TypeArchiveNode),
			hit("AN-1002", models.TypeArchiveNode),
			hit("AN-1003", models.TypeArchiveNode),
		},
		nodes: map[models.ID]*models.ArchiveNode{
			hiddenID: {ID: hiddenID, Label: "Скрытый", Sources: []models.SourceLink{{CitationID: citationID}}},
			idA:      {ID: idA, Label: "A"},
			idB:      {ID: idB, Label: "B"},
		},
		citations: map[models.ID]*models.Citation{citationID: {ID: citationID, Private: true}},
	}

	scenario := New(repo)

	page1, err := scenario.SearchArchiveNodes(context.Background(), models.AccessPublic,
		models.SearchQuery{Text: "ф", Page: models.Page{Limit: 1, Offset: 0}})
	if err != nil {
		t.Fatalf("SearchArchiveNodes (page1): %v", err)
	}

	if !sameIDs(ids(page1), idA) {
		t.Fatalf("page1 = %v, want [A]", ids(page1))
	}

	page2, err := scenario.SearchArchiveNodes(context.Background(), models.AccessPublic,
		models.SearchQuery{Text: "ф", Page: models.Page{Limit: 1, Offset: 1}})
	if err != nil {
		t.Fatalf("SearchArchiveNodes (page2): %v", err)
	}

	if !sameIDs(ids(page2), idB) {
		t.Fatalf("page2 = %v, want [B]", ids(page2))
	}
}
