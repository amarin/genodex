package list_archive_nodes

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type ctxKey struct{}

// fakeRepo отдаёт окна списка как настоящий репозиторий и запоминает вызовы.
type fakeRepo struct {
	list     []*models.ArchiveNode
	err      error
	errAt    int // номер вызова (с 1), на котором возвращается err; 0 — на любом
	gotCtx   context.Context
	calls    []models.Page
	accesses []models.Access
}

// window нарезает список по окну, как настоящий репозиторий.
func window(list []*models.ArchiveNode, page models.Page) []*models.ArchiveNode {
	page = page.Normalized()
	if page.Offset >= len(list) {
		return nil
	}

	return list[page.Offset:min(page.Offset+page.Limit, len(list))]
}

func (f *fakeRepo) ListArchiveNodes(ctx context.Context, access models.Access, page models.Page) ([]*models.ArchiveNode, error) {
	f.gotCtx = ctx
	f.calls = append(f.calls, page)
	f.accesses = append(f.accesses, access)

	if f.err != nil && (f.errAt == 0 || f.errAt == len(f.calls)) {
		return nil, f.err
	}

	return window(f.list, page), nil
}

const (
	archive1 = models.ID("AR-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	archive2 = models.ID("AR-01ARZ3NDEKTSV4RRFFQ69G5FA2")
)

// anID возвращает корректный идентификатор узла, отличающийся последним символом.
func anID(last byte) models.ID {
	return models.ID("AN-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func node(id models.ID, archive models.ID, parent *models.ID) *models.ArchiveNode {
	return &models.ArchiveNode{ID: id, Type: "fond", ArchiveID: archive, ParentID: parent, Label: string(id)}
}

func ids(list []models.ArchiveNode) []models.ID {
	out := []models.ID{}
	for _, n := range list {
		out = append(out, n.ID)
	}

	return out
}

func sameIDs(got []models.ID, want ...models.ID) bool {
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

var (
	root1 = anID('1')
	node2 = anID('2')
	node3 = anID('3')
	node4 = anID('4')
	node5 = anID('5')
)

func sample() *fakeRepo {
	return &fakeRepo{list: []*models.ArchiveNode{
		node(root1, archive1, nil),
		node(node2, archive1, &root1),
		node(node3, archive2, nil),
		node(node4, archive1, nil),
		node(node5, archive1, &root1),
	}}
}

func TestListArchiveNodesRootOfArchive(t *testing.T) {
	got, err := New(sample()).ListArchiveNodes(context.Background(), models.AccessFull, models.ArchiveNodeQuery{ArchiveID: archive1})
	if err != nil || !sameIDs(ids(got), root1, node4) {
		t.Fatalf("got %v, %v; ожидались корневые узлы root1, node4 архива archive1", ids(got), err)
	}
}

func TestListArchiveNodesOtherArchiveNotMixed(t *testing.T) {
	got, err := New(sample()).ListArchiveNodes(context.Background(), models.AccessFull, models.ArchiveNodeQuery{ArchiveID: archive2})
	if err != nil || !sameIDs(ids(got), node3) {
		t.Fatalf("got %v, %v; ожидался только node3 (другой архив)", ids(got), err)
	}
}

func TestListArchiveNodesChildrenOfParent(t *testing.T) {
	parent := root1

	got, err := New(sample()).ListArchiveNodes(context.Background(), models.AccessFull,
		models.ArchiveNodeQuery{ArchiveID: archive1, ParentID: &parent})
	if err != nil || !sameIDs(ids(got), node2, node5) {
		t.Fatalf("got %v, %v; ожидались дети root1: node2, node5", ids(got), err)
	}
}

// TestListArchiveNodesParentFromOtherArchiveNotConfused: узел с ParentID,
// совпадающим по значению, но принадлежащий другому архиву, не путается с
// запросом на другой архив — фильтр по ArchiveID применяется независимо.
func TestListArchiveNodesParentFromOtherArchiveNotConfused(t *testing.T) {
	parent := root1

	got, err := New(sample()).ListArchiveNodes(context.Background(), models.AccessFull,
		models.ArchiveNodeQuery{ArchiveID: archive2, ParentID: &parent})
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v; ожидался пустой результат (в archive2 нет детей root1)", ids(got), err)
	}
}

// TestListArchiveNodesWindowIsAppliedAfterFilter: сдвиг и размер считаются
// среди прошедших фильтр, а не среди всех узлов.
func TestListArchiveNodesWindowIsAppliedAfterFilter(t *testing.T) {
	q := models.ArchiveNodeQuery{ArchiveID: archive1, Page: models.Page{Limit: 1, Offset: 1}}

	got, err := New(sample()).ListArchiveNodes(context.Background(), models.AccessFull, q)
	if err != nil || !sameIDs(ids(got), node4) {
		t.Fatalf("got %v, %v; ожидался второй корневой узел node4", ids(got), err)
	}
}

// TestListArchiveNodesWalksAllRepositoryWindows: узлы за первым окном
// репозитория не теряются, окна запрашиваются подряд; access, переданный
// вызывающим, доходит до каждого окна репозитория как есть.
func TestListArchiveNodesWalksAllRepositoryWindows(t *testing.T) {
	total := 2*models.MaxPageLimit + 7

	repo := &fakeRepo{}

	for i := 0; i < total; i++ {
		a := archive1
		if i%2 == 1 {
			a = archive2 // не тот архив
		}

		repo.list = append(repo.list, node(models.ID("AN-"+padded(i)), a, nil))
	}

	got, err := New(repo).ListArchiveNodes(context.Background(), models.AccessPublic,
		models.ArchiveNodeQuery{ArchiveID: archive1, Page: models.Page{Limit: models.MaxPageLimit}})
	if err != nil || len(got) != models.MaxPageLimit {
		t.Fatalf("got %d, %v; ожидалось %d", len(got), err, models.MaxPageLimit)
	}

	if len(repo.calls) != 2 {
		t.Fatalf("вызовов репозитория %d (%v), ожидалось 2", len(repo.calls), repo.calls)
	}

	for i, a := range repo.accesses {
		if a != models.AccessPublic {
			t.Errorf("вызов %d: доступ %v, ожидался AccessPublic (переданный вызывающим)", i+1, a)
		}
	}
}

// TestListArchiveNodesInvalidQueryDoesNotTouchRepo: некорректный запрос —
// *ValidationError, репозиторий не вызывается.
func TestListArchiveNodesInvalidQueryDoesNotTouchRepo(t *testing.T) {
	repo := sample()
	bad := models.ID("not-an-id")

	for _, q := range []models.ArchiveNodeQuery{
		{}, // ArchiveID обязателен
		{ArchiveID: archive1, ParentID: &bad},
		{ArchiveID: archive1, Page: models.Page{Limit: -1}},
		{ArchiveID: archive1, Page: models.Page{Offset: -1}},
	} {
		_, err := New(repo).ListArchiveNodes(context.Background(), models.AccessFull, q)

		var ve *models.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("запрос %+v: err = %v, ожидалась *ValidationError", q, err)
		}
	}

	if len(repo.calls) != 0 {
		t.Fatalf("репозиторий вызван %d раз при некорректном запросе", len(repo.calls))
	}
}

func TestListArchiveNodesEmptyRepoGivesEmptyNotNil(t *testing.T) {
	got, err := New(&fakeRepo{}).ListArchiveNodes(context.Background(), models.AccessFull, models.ArchiveNodeQuery{ArchiveID: archive1})
	if err != nil {
		t.Fatalf("ListArchiveNodes: %v", err)
	}

	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v, ожидался пустой не-nil срез", got)
	}
}

func TestListArchiveNodesPropagatesRepoError(t *testing.T) {
	wantErr := errors.New("repo down")

	if _, err := New(&fakeRepo{err: wantErr}).ListArchiveNodes(context.Background(), models.AccessFull, models.ArchiveNodeQuery{ArchiveID: archive1}); !errors.Is(err, wantErr) {
		t.Errorf("err=%v, want %v", err, wantErr)
	}
}

// TestListArchiveNodesPropagatesErrorFromLaterWindow: сбой на втором окне
// репозитория не даёт частичного результата (первое окно репозитория,
// наполовину отфильтрованное по архиву, даёт лишь половину окна запроса,
// поэтому читается и второе).
func TestListArchiveNodesPropagatesErrorFromLaterWindow(t *testing.T) {
	wantErr := errors.New("repo down on window 2")

	repo := &fakeRepo{err: wantErr, errAt: 2}

	for i := 0; i < models.MaxPageLimit+1; i++ {
		a := archive1
		if i%2 == 1 {
			a = archive2
		}

		repo.list = append(repo.list, node(models.ID("AN-"+padded(i)), a, nil))
	}

	got, err := New(repo).ListArchiveNodes(context.Background(), models.AccessFull,
		models.ArchiveNodeQuery{ArchiveID: archive1, Page: models.Page{Limit: models.MaxPageLimit}})
	if !errors.Is(err, wantErr) || got != nil {
		t.Fatalf("got %v, %v; ожидалась ошибка %v без результата", ids(got), err, wantErr)
	}
}

func TestListArchiveNodesPassesContextToRepo(t *testing.T) {
	repo := &fakeRepo{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "marker")

	if _, err := New(repo).ListArchiveNodes(ctx, models.AccessFull, models.ArchiveNodeQuery{ArchiveID: archive1}); err != nil {
		t.Fatalf("ListArchiveNodes: %v", err)
	}

	if repo.gotCtx == nil || repo.gotCtx.Value(ctxKey{}) != "marker" {
		t.Fatalf("репозиторий получил контекст %v, ожидался переданный сценарию", repo.gotCtx)
	}
}

// padded возвращает номер, дополненный нулями слева до 20 знаков — узлы этих
// тестов не проверяются на формат id (только archive_id/parent_id запроса),
// но детерминированный текст полезен для отладки при падении теста.
func padded(i int) string {
	s := strconv.Itoa(i)
	for len(s) < 20 {
		s = "0" + s
	}

	return s
}
