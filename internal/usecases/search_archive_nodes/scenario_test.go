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

func (f *fakeRepo) GetArchiveNode(_ context.Context, id models.ID) (*models.ArchiveNode, error) {
	f.getCalls = append(f.getCalls, id)

	if f.getErr != nil {
		if e, ok := f.getErr[id]; ok && e != nil {
			return nil, e
		}
	}

	return archiveNode(string(id)), nil
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
