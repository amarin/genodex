package search_divisions

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type ctxKey struct{}

// hit строит хиты поиска: только нужные поля.
func hit(id string, typ models.Type) models.Hit {
	return models.Hit{Type: typ, ID: models.ID(id), Label: id}
}

// division строит единицу деления.
func division(id string) *models.AdministrativeDivision {
	return &models.AdministrativeDivision{ID: models.ID(id), Name: id}
}

// ids извлекает идентификаторы единиц деления.
func ids(divisions []models.AdministrativeDivision) []models.ID {
	out := make([]models.ID, 0, len(divisions))
	for _, d := range divisions {
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

// fakeRepo отдаёт окна хитов и единицы деления.
type fakeRepo struct {
	hits     []models.Hit        // статический список хитов (Search нарезает окнами)
	getErr   map[models.ID]error // ошибка GetAdministrativeDivision по id (nil — ок)
	err      error
	errAt    int
	gotCtx   context.Context
	gotQuery string
	calls    []models.Page
	getCalls []models.ID
	accesses []models.Access
}

func (f *fakeRepo) Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error) {
	f.gotCtx = ctx
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

func (f *fakeRepo) GetAdministrativeDivision(ctx context.Context, id models.ID) (*models.AdministrativeDivision, error) {
	f.getCalls = append(f.getCalls, id)

	if f.getErr != nil {
		if e, ok := f.getErr[id]; ok && e != nil {
			return nil, e
		}
	}

	return division(string(id)), nil
}

// TestEmptyTextDoesNotTouchRepo: пустой (после обрезки) текст — пустой результат,
// репозиторий не вызывается.
func TestEmptyTextDoesNotTouchRepo(t *testing.T) {
	repo := &fakeRepo{}

	got, err := New(repo).SearchDivisions(context.Background(), models.AccessFull, models.DivisionSearchQuery{Text: "   "})
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v; ожидался пустой результат", got, err)
	}
	if len(repo.calls) != 0 {
		t.Fatalf("Search вызван %d раз при пустом тексте", len(repo.calls))
	}
}

// TestSearchReturnsFullDivisionsFromHits: из хитов поиска возвращаются полные
// единицы деления; хиты других типов пропускаются.
func TestSearchReturnsFullDivisionsFromHits(t *testing.T) {
	repo := &fakeRepo{
		hits: []models.Hit{
			hit("D-1001", models.TypePerson),
			hit("AD-1002", models.TypeAdministrativeDivision),
			hit("F-1003", models.TypeFamily),
			hit("AD-1004", models.TypeAdministrativeDivision),
		},
	}

	got, err := New(repo).SearchDivisions(context.Background(), models.AccessFull, models.DivisionSearchQuery{Text: "давыд"})
	if err != nil || !sameIDs(ids(got), "AD-1002", "AD-1004") {
		t.Fatalf("got %v, %v; ожидались AD-1002, AD-1004", ids(got), err)
	}
	if repo.gotQuery != "давыд" {
		t.Fatalf("Search получил %q, ожидался %q", repo.gotQuery, "давыд")
	}
}

// TestSearchAppliesWindowAmongDivisionHits: окно запроса действует только на
// division-хиты (сдвиг и размер после отбора по типу).
func TestSearchAppliesWindowAmongDivisionHits(t *testing.T) {
	repo := &fakeRepo{
		hits: []models.Hit{
			hit("D-1001", models.TypePerson), // не считается
			hit("AD-1002", models.TypeAdministrativeDivision),
			hit("AD-1003", models.TypeAdministrativeDivision),
			hit("AD-1004", models.TypeAdministrativeDivision),
		},
	}

	got, err := New(repo).SearchDivisions(context.Background(), models.AccessFull, models.DivisionSearchQuery{
		Text: "п",
		Page: models.Page{Limit: 1, Offset: 1},
	})
	if err != nil || !sameIDs(ids(got), "AD-1003") {
		t.Fatalf("got %v, %v; ожидалась AD-1003 (второй division-хит)", ids(got), err)
	}
}

// TestSearchSkipsDeletedDivision: хит, у которого единица удалена (ErrNotFound),
// пропускается, остальные возвращаются.
func TestSearchSkipsDeletedDivision(t *testing.T) {
	repo := &fakeRepo{
		hits: []models.Hit{
			hit("AD-1002", models.TypeAdministrativeDivision),
			hit("AD-1003", models.TypeAdministrativeDivision),
		},
		getErr: map[models.ID]error{"AD-1002": models.ErrNotFound},
	}

	got, err := New(repo).SearchDivisions(context.Background(), models.AccessFull, models.DivisionSearchQuery{Text: "п"})
	if err != nil {
		t.Fatalf("SearchDivisions: %v", err)
	}
	if !sameIDs(ids(got), "AD-1003") {
		t.Fatalf("got %v; AD-1002 удалён — остаётся AD-1003", ids(got))
	}
	if !sameIDs(repo.getCalls, "AD-1002", "AD-1003") {
		t.Fatalf("должны были дотянуться до обоих хитов, вызвано: %v", repo.getCalls)
	}
}

// TestSearchPropagatesGetError: ошибка репозитория при чтении единицы (кроме
// ErrNotFound) пробрасывается.
func TestSearchPropagatesGetError(t *testing.T) {
	boom := errors.New("get boom")
	repo := &fakeRepo{
		hits:   []models.Hit{hit("AD-1002", models.TypeAdministrativeDivision)},
		getErr: map[models.ID]error{"AD-1002": boom},
	}

	_, err := New(repo).SearchDivisions(context.Background(), models.AccessFull, models.DivisionSearchQuery{Text: "п"})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, ожидался boom", err)
	}
}

// TestSearchPropagatesSearchError: ошибка репозитория в Search пробрасывается.
func TestSearchPropagatesSearchError(t *testing.T) {
	boom := errors.New("search boom")
	repo := &fakeRepo{err: boom}

	_, err := New(repo).SearchDivisions(context.Background(), models.AccessFull, models.DivisionSearchQuery{Text: "п"})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, ожидался boom", err)
	}
}

// TestSearchInvalidQueryDoesNotTouchRepo: некорректный запрос — *ValidationError,
// репозиторий не вызывается.
func TestSearchInvalidQueryDoesNotTouchRepo(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).SearchDivisions(context.Background(), models.AccessFull, models.DivisionSearchQuery{Page: models.Page{Limit: -1}})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, ожидалась *ValidationError поля limit", err)
	}
	if len(repo.calls) != 0 {
		t.Fatalf("Search вызван %d раз при некорректном запросе", len(repo.calls))
	}
}

// TestSearchDivisionsPassesAccessToRepo: access, переданный вызывающим,
// доходит до Search как есть (не захардкожен на AccessFull).
func TestSearchDivisionsPassesAccessToRepo(t *testing.T) {
	repo := &fakeRepo{hits: []models.Hit{
		hit("AD-1002", models.TypeAdministrativeDivision),
	}}

	if _, err := New(repo).SearchDivisions(context.Background(), models.AccessPublic,
		models.DivisionSearchQuery{Text: "п"}); err != nil {
		t.Fatalf("SearchDivisions: %v", err)
	}

	if len(repo.accesses) == 0 {
		t.Fatalf("accesses пуст, ожидались вызовы с AccessPublic")
	}

	for i, a := range repo.accesses {
		if a != models.AccessPublic {
			t.Errorf("вызов %d: доступ %v, ожидался AccessPublic (переданный вызывающим, не захардкоженный AccessFull)", i+1, a)
		}
	}
}
