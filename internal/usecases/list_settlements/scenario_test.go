package list_settlements

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
	list     []*models.AdministrativeDivision
	err      error
	errAt    int // номер вызова (с 1), на котором возвращается err; 0 — на любом
	gotCtx   context.Context
	calls    []models.Page
	accesses []models.Access
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

	page = page.Normalized()
	if page.Offset >= len(f.list) {
		return nil, nil
	}

	end := min(page.Offset+page.Limit, len(f.list))

	return f.list[page.Offset:end], nil
}

func TestScenarioListSettlementsFiltersDivisions(t *testing.T) {
	sc := New(&fakeRepo{
		list: []*models.AdministrativeDivision{
			{ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo},
			{ID: "ad-2", Name: "Никифоровская", Type: models.AdminDivisionVolost},
			{ID: "ad-3", Name: "Никифорово", Type: models.AdminDivisionDerevnya},
		},
	})

	got, err := sc.ListSettlements(context.Background())
	if err != nil {
		t.Fatalf("ListSettlements: %v", err)
	}
	if len(got) != 2 || got[0].ID != "ad-1" || got[1].ID != "ad-3" {
		t.Fatalf("got %+v, want ad-1 и ad-3 (волость отфильтрована)", got)
	}
}

// TestScenarioWalksAllPages: населённые пункты за пределами первого окна не
// теряются, окна запрашиваются подряд с полным доступом.
func TestScenarioWalksAllPages(t *testing.T) {
	total := 2*models.MaxPageLimit + 7

	repo := &fakeRepo{}
	for i := 0; i < total; i++ {
		typ := models.AdminDivisionDerevnya
		if i%2 == 1 {
			typ = models.AdminDivisionVolost // не населённый пункт
		}

		repo.list = append(repo.list, &models.AdministrativeDivision{
			ID: models.ID("ad-" + strconv.Itoa(i)), Type: typ,
		})
	}

	got, err := New(repo).ListSettlements(context.Background())
	if err != nil {
		t.Fatalf("ListSettlements: %v", err)
	}

	want := (total + 1) / 2 // чётные индексы
	if len(got) != want {
		t.Fatalf("населённых пунктов %d, ожидалось %d", len(got), want)
	}

	// три окна с данными и пустое, завершающее обход
	wantCalls := []models.Page{
		{Limit: models.MaxPageLimit, Offset: 0},
		{Limit: models.MaxPageLimit, Offset: models.MaxPageLimit},
		{Limit: models.MaxPageLimit, Offset: 2 * models.MaxPageLimit},
		{Limit: models.MaxPageLimit, Offset: 3 * models.MaxPageLimit},
	}
	if len(repo.calls) != len(wantCalls) {
		t.Fatalf("вызовов репозитория %d (%v), ожидалось %d", len(repo.calls), repo.calls, len(wantCalls))
	}

	for i, c := range wantCalls {
		if repo.calls[i] != c {
			t.Errorf("вызов %d: окно %+v, ожидалось %+v", i+1, repo.calls[i], c)
		}

		if repo.accesses[i] != models.AccessFull {
			t.Errorf("вызов %d: доступ %v, ожидался AccessFull", i+1, repo.accesses[i])
		}
	}
}

func TestScenarioEmptyRepoGivesEmptyNotNil(t *testing.T) {
	got, err := New(&fakeRepo{}).ListSettlements(context.Background())
	if err != nil {
		t.Fatalf("ListSettlements: %v", err)
	}

	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v, ожидался пустой не-nil срез", got)
	}
}

func TestScenarioPropagatesRepoError(t *testing.T) {
	wantErr := errors.New("repo down")
	sc := New(&fakeRepo{err: wantErr})

	if _, err := sc.ListSettlements(context.Background()); !errors.Is(err, wantErr) {
		t.Errorf("err=%v, want %v", err, wantErr)
	}
}

// TestScenarioPropagatesErrorFromLaterPage: сбой на втором окне не даёт
// частичного результата.
func TestScenarioPropagatesErrorFromLaterPage(t *testing.T) {
	wantErr := errors.New("repo down on page 2")

	repo := &fakeRepo{err: wantErr, errAt: 2}
	for i := 0; i < models.MaxPageLimit+1; i++ {
		repo.list = append(repo.list, &models.AdministrativeDivision{
			ID: models.ID("ad-" + strconv.Itoa(i)), Type: models.AdminDivisionDerevnya,
		})
	}

	got, err := New(repo).ListSettlements(context.Background())
	if !errors.Is(err, wantErr) || got != nil {
		t.Fatalf("got %v, %v; ожидалась ошибка %v без результата", got, err, wantErr)
	}
}

func TestScenarioPassesContextToRepo(t *testing.T) {
	repo := &fakeRepo{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "marker")

	if _, err := New(repo).ListSettlements(ctx); err != nil {
		t.Fatalf("ListSettlements: %v", err)
	}

	if repo.gotCtx == nil || repo.gotCtx.Value(ctxKey{}) != "marker" {
		t.Fatalf("репозиторий получил контекст %v, ожидался переданный сценарию", repo.gotCtx)
	}
}
