package list_divisions

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
	list      []*models.AdministrativeDivision // корневой список (ListAdministrativeDivisions)
	children  []*models.AdministrativeDivision // список детей (ChildrenOfDivision)
	err       error
	errAt     int // номер вызова (с 1), на котором возвращается err; 0 — на любом
	gotCtx    context.Context
	gotParent *models.ID
	calls     []models.Page
	accesses  []models.Access
}

// window нарезает список по окну, как настоящий репозиторий.
func window(list []*models.AdministrativeDivision, page models.Page) []*models.AdministrativeDivision {
	page = page.Normalized()
	if page.Offset >= len(list) {
		return nil
	}

	return list[page.Offset:min(page.Offset+page.Limit, len(list))]
}

func (f *fakeRepo) ListAdministrativeDivisions(
	ctx context.Context, access models.Access, page models.Page,
) ([]*models.AdministrativeDivision, error) {
	f.gotCtx = ctx
	f.calls = append(f.calls, page)
	f.accesses = append(f.accesses, access)

	if f.err != nil && (f.errAt == 0 || f.errAt == len(f.calls)) {
		return nil, f.err
	}

	return window(f.list, page), nil
}

func (f *fakeRepo) ChildrenOfDivision(
	ctx context.Context, parent models.ID, access models.Access, page models.Page,
) ([]*models.AdministrativeDivision, error) {
	f.gotCtx = ctx
	f.calls = append(f.calls, page)
	f.accesses = append(f.accesses, access)
	f.gotParent = &parent

	if f.err != nil && (f.errAt == 0 || f.errAt == len(f.calls)) {
		return nil, f.err
	}

	return window(f.children, page), nil
}

func division(id string, typ models.AdminDivisionType) *models.AdministrativeDivision {
	return &models.AdministrativeDivision{ID: models.ID(id), Name: id, Type: typ}
}

// validParent — id родителя в валидном формате (префикс AD + 25-значное тело).
const validParent = models.ID("AD-01J8X4T0K2M9Q7R5V3B6N8C1D4")

func ids(list []models.AdministrativeDivision) []models.ID {
	out := []models.ID{}
	for _, d := range list {
		out = append(out, d.ID)
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

func sample() *fakeRepo {
	return &fakeRepo{list: []*models.AdministrativeDivision{
		division("ad-1", models.AdminDivisionSelo),
		division("ad-2", models.AdminDivisionVolost),
		division("ad-3", models.AdminDivisionDerevnya),
		division("ad-4", models.AdminDivisionSelo),
		division("ad-5", models.AdminDivisionGovernorate),
	}}
}

func TestListDivisionsWithoutFiltersReturnsAll(t *testing.T) {
	got, err := New(sample()).ListDivisions(context.Background(), models.DivisionQuery{})
	if err != nil || !sameIDs(ids(got), "ad-1", "ad-2", "ad-3", "ad-4", "ad-5") {
		t.Fatalf("got %v, %v; ожидались все пять единиц", ids(got), err)
	}
}

func TestListDivisionsKindSettlement(t *testing.T) {
	got, err := New(sample()).ListDivisions(context.Background(), models.DivisionQuery{Kind: models.DivisionKindSettlement})
	if err != nil || !sameIDs(ids(got), "ad-1", "ad-3", "ad-4") {
		t.Fatalf("got %v, %v; ожидались ad-1, ad-3, ad-4 (волость и губерния отфильтрованы)", ids(got), err)
	}
}

func TestListDivisionsTypeFilter(t *testing.T) {
	got, err := New(sample()).ListDivisions(context.Background(), models.DivisionQuery{Type: models.AdminDivisionSelo})
	if err != nil || !sameIDs(ids(got), "ad-1", "ad-4") {
		t.Fatalf("got %v, %v; ожидались ad-1 и ad-4", ids(got), err)
	}

	got, err = New(sample()).ListDivisions(context.Background(),
		models.DivisionQuery{Kind: models.DivisionKindSettlement, Type: models.AdminDivisionVolost})
	if err != nil || len(got) != 0 {
		t.Fatalf("вид и тип пересекаются: got %v, %v; ожидался пустой результат", ids(got), err)
	}
}

// TestListDivisionsWindowIsAppliedAfterFilter: сдвиг и размер считаются среди
// прошедших фильтр, а не среди всех единиц.
func TestListDivisionsWindowIsAppliedAfterFilter(t *testing.T) {
	q := models.DivisionQuery{Kind: models.DivisionKindSettlement, Page: models.Page{Limit: 1, Offset: 1}}

	got, err := New(sample()).ListDivisions(context.Background(), q)
	if err != nil || !sameIDs(ids(got), "ad-3") {
		t.Fatalf("got %v, %v; ожидалась одна ad-3 (второй населённый пункт)", ids(got), err)
	}

	q.Page = models.Page{Limit: 5, Offset: 2}

	got, err = New(sample()).ListDivisions(context.Background(), q)
	if err != nil || !sameIDs(ids(got), "ad-4") {
		t.Fatalf("got %v, %v; ожидалась одна ad-4 (короткое окно — конец списка)", ids(got), err)
	}

	q.Page = models.Page{Offset: 10}

	got, err = New(sample()).ListDivisions(context.Background(), q)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("сдвиг за пределами набора: got %#v, %v; ожидался пустой не-nil срез", got, err)
	}
}

// TestListDivisionsWalksAllRepositoryWindows: единицы за первым окном репозитория
// не теряются, окна запрашиваются подряд с полным доступом.
func TestListDivisionsWalksAllRepositoryWindows(t *testing.T) {
	total := 2*models.MaxPageLimit + 7

	repo := &fakeRepo{}

	for i := 0; i < total; i++ {
		typ := models.AdminDivisionDerevnya
		if i%2 == 1 {
			typ = models.AdminDivisionVolost // не населённый пункт
		}

		repo.list = append(repo.list, division("ad-"+strconv.Itoa(i), typ))
	}

	got, err := New(repo).ListDivisions(context.Background(),
		models.DivisionQuery{Kind: models.DivisionKindSettlement, Page: models.Page{Limit: models.MaxPageLimit}})
	if err != nil || len(got) != models.MaxPageLimit {
		t.Fatalf("got %d, %v; ожидалось %d", len(got), err, models.MaxPageLimit)
	}

	// окно запроса заполнено раньше конца списка: репозиторий читается не дальше
	// нужного (500 населённых пунктов лежат в первых двух окнах по 500)
	if len(repo.calls) != 2 {
		t.Fatalf("вызовов репозитория %d (%v), ожидалось 2", len(repo.calls), repo.calls)
	}

	for i, a := range repo.accesses {
		if a != models.AccessFull {
			t.Errorf("вызов %d: доступ %v, ожидался AccessFull", i+1, a)
		}
	}

	all := &fakeRepo{list: repo.list}

	got, err = New(all).ListDivisions(context.Background(), models.DivisionQuery{Kind: models.DivisionKindSettlement,
		Page: models.Page{Limit: models.MaxPageLimit, Offset: models.MaxPageLimit}})
	if err != nil || len(got) != (total+1)/2-models.MaxPageLimit {
		t.Fatalf("второе окно: %d, %v; ожидалось %d", len(got), err, (total+1)/2-models.MaxPageLimit)
	}
}

// TestListDivisionsInvalidQueryDoesNotTouchRepo: некорректный запрос —
// *ValidationError, репозиторий не вызывается.
func TestListDivisionsInvalidQueryDoesNotTouchRepo(t *testing.T) {
	repo := sample()

	for _, q := range []models.DivisionQuery{
		{Kind: "village"}, {Type: "castle"}, {Page: models.Page{Limit: -1}}, {Page: models.Page{Offset: -1}},
	} {
		_, err := New(repo).ListDivisions(context.Background(), q)

		var ve *models.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("запрос %+v: err = %v, ожидалась *ValidationError", q, err)
		}
	}

	if len(repo.calls) != 0 {
		t.Fatalf("репозиторий вызван %d раз при некорректном запросе", len(repo.calls))
	}
}

func TestListDivisionsEmptyRepoGivesEmptyNotNil(t *testing.T) {
	got, err := New(&fakeRepo{}).ListDivisions(context.Background(), models.DivisionQuery{})
	if err != nil {
		t.Fatalf("ListDivisions: %v", err)
	}

	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v, ожидался пустой не-nil срез", got)
	}
}

func TestListDivisionsPropagatesRepoError(t *testing.T) {
	wantErr := errors.New("repo down")

	if _, err := New(&fakeRepo{err: wantErr}).ListDivisions(context.Background(), models.DivisionQuery{}); !errors.Is(err, wantErr) {
		t.Errorf("err=%v, want %v", err, wantErr)
	}
}

// TestListDivisionsPropagatesErrorFromLaterWindow: сбой на втором окне
// репозитория не даёт частичного результата (после фильтра первое окно
// репозитория даёт лишь половину окна запроса, поэтому читается и второе).
func TestListDivisionsPropagatesErrorFromLaterWindow(t *testing.T) {
	wantErr := errors.New("repo down on window 2")

	repo := &fakeRepo{err: wantErr, errAt: 2}

	for i := 0; i < models.MaxPageLimit+1; i++ {
		typ := models.AdminDivisionDerevnya
		if i%2 == 1 {
			typ = models.AdminDivisionVolost
		}

		repo.list = append(repo.list, division("ad-"+strconv.Itoa(i), typ))
	}

	got, err := New(repo).ListDivisions(context.Background(),
		models.DivisionQuery{Kind: models.DivisionKindSettlement, Page: models.Page{Limit: models.MaxPageLimit}})
	if !errors.Is(err, wantErr) || got != nil {
		t.Fatalf("got %v, %v; ожидалась ошибка %v без результата", ids(got), err, wantErr)
	}
}

func TestListDivisionsPassesContextToRepo(t *testing.T) {
	repo := &fakeRepo{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "marker")

	if _, err := New(repo).ListDivisions(ctx, models.DivisionQuery{}); err != nil {
		t.Fatalf("ListDivisions: %v", err)
	}

	if repo.gotCtx == nil || repo.gotCtx.Value(ctxKey{}) != "marker" {
		t.Fatalf("репозиторий получил контекст %v, ожидался переданный сценарию", repo.gotCtx)
	}
}

// TestListDivisionsFromParentReturnsChildren: при ParentID — только прямые дети,
// корневой список не читается (ChildrenOfDivision не трогает f.list).
func TestListDivisionsFromParentReturnsChildren(t *testing.T) {
	repo := &fakeRepo{
		list: []*models.AdministrativeDivision{division("ad-root", models.AdminDivisionGovernorate)},
		children: []*models.AdministrativeDivision{
			division("ad-1", models.AdminDivisionSelo),
			division("ad-2", models.AdminDivisionVolost),
		},
	}
	parent := validParent

	got, err := New(repo).ListDivisions(context.Background(), models.DivisionQuery{ParentID: &parent})
	if err != nil || !sameIDs(ids(got), "ad-1", "ad-2") {
		t.Fatalf("got %v, %v; ожидались ad-1, ad-2", ids(got), err)
	}
	if repo.gotParent == nil || *repo.gotParent != parent {
		t.Fatalf("gotParent = %v, ожидался %s", repo.gotParent, parent)
	}
}

// TestListDivisionsFromParentAppliesFiltersAndWindow: вид/тип и окно применяются
// среди детей, а не среди всего репозитория.
func TestListDivisionsFromParentAppliesFiltersAndWindow(t *testing.T) {
	repo := &fakeRepo{
		children: []*models.AdministrativeDivision{
			division("ad-1", models.AdminDivisionSelo),
			division("ad-2", models.AdminDivisionVolost),
			division("ad-3", models.AdminDivisionDerevnya),
			division("ad-4", models.AdminDivisionSelo),
		},
	}
	parent := validParent

	got, err := New(repo).ListDivisions(context.Background(), models.DivisionQuery{
		Kind:     models.DivisionKindSettlement,
		ParentID: &parent,
		Page:     models.Page{Limit: 1, Offset: 1},
	})
	if err != nil || !sameIDs(ids(got), "ad-3") {
		t.Fatalf("got %v, %v; ожидалась ad-3 (второе селение среди детей)", ids(got), err)
	}
}

// TestListDivisionsFromMissingParentIsNotFound: ChildrenOfDivision с ErrNotFound
// порта пробрасывается (404 наверху).
func TestListDivisionsFromMissingParentIsNotFound(t *testing.T) {
	repo := &fakeRepo{err: models.ErrNotFound}
	parent := validParent

	_, err := New(repo).ListDivisions(context.Background(), models.DivisionQuery{ParentID: &parent})
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}
}

// TestListDivisionsInvalidParentDoesNotTouchRepo: неверный формат parent_id —
// *ValidationError, репозиторий не вызывается.
func TestListDivisionsInvalidParentDoesNotTouchRepo(t *testing.T) {
	repo := sample()
	bad := models.ID("not-an-id")

	_, err := New(repo).ListDivisions(context.Background(), models.DivisionQuery{ParentID: &bad})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError поля parent_id", err)
	}
	if len(repo.calls) != 0 {
		t.Fatalf("репозиторий вызван %d раз при некорректном parent_id", len(repo.calls))
	}
}
