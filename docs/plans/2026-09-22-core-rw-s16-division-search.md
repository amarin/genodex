# S16. Поиск и дочерние деления в контрактах — план этапа

Дата: 2026-09-22. Формат — как план S15 (`2026-09-22-core-rw-s15-division-contracts.md`).
Спека этапа — [roadmap](2026-09-20-core-rw-roadmap.md), раздел S16. Исполняется в `main`
подрядными маленькими коммитами по ритуалу S1–S15.

## Goal

Довести административное деление до вертикального контракта «чтение + поиск + дочерние
деления»:

1. Список от конкретного родителя (`parent_id`) — и в HTTP (`/api/admin-divisions`), и в
   MCP (`division_list`);
2. Поиск по началу названия (включая варианты названий) — отдельный HTTP-эндпоинт и новый
   MCP-тул;
3. Фасады `internal/app`, `internal/httpapi` и `internal/mcp` расширяются синхронно —
   дерево собирается на каждом коммите (урок S15).

Порт (`internal/store`) **менять не нужно**: `Store.Search` и `Store.ChildrenOfDivision`
реализованы на S16-инфраструктуре (проверено `children_test.go`/`search_test.go` в
`internal/store/sqlstore`).

## Architecture (решения пользователя)

- **Поиск — отдельный эндпоинт** `GET /api/admin-divisions/search?q=&limit=&offset=`.
  В Go 1.22 ServeMux литеральный шаблон побеждает wildcard `{id}` — поведение закрепляем
  явным тестом маршрута (иначе `search` попадёт в `{id}` и ответит 422 «неверный формат id»).
- **Результат поиска — полные единицы `[]AdminDivision`** `{id,name,type,parent_id}`, а не
  хиты порта: сценарий делает `Search` (получает `models.Hit{Type,ID,Label,Field}`) и
  дозагружает каждую единицу через `GetAdministrativeDivision`.
- **Поиск по префиксу**, семантика порта `Search`; **варианты названий участвуют**:
  `SaveAdministrativeDivision` индексирует `Name` + `Variants` в поле `name`. Пустой `q`
  после `TrimSpace` — пустой результат без вызова репозитория (защита от «найди всё»).
- **Окно применяется после отбора единиц деления** — зеркально `list_divisions`: сценарий
  обходит окна `Search` по `MaxPageLimit`, отбрасывает хиты не-делений (`h.Type !=
  TypeAdministrativeDivision`), считает `matched` только среди делений.
- **Список от родителя** делается на уровне репозитория: при `ParentID != nil` окна берутся
  из `ChildrenOfDivision` вместо `ListAdministrativeDivisions`; общий код обхода/фильтра/окна
  один. Несуществующий родитель (`ErrNotFound` порта) — 404. Неверный формат `parent_id` —
  422, репозиторий не вызывается (`Validate` сценария).
- **Типы запросов**: `models.DivisionQuery` получает `ParentID *models.ID`; новый
  `models.DivisionSearchQuery{Text, Page}` с `Validate()` → `*ValidationError`
  (`limit`/`offset` отриц. — 422). Валидация живёт в `models`, т.к. `fieldErr`/`idErr`
  неэкспортируемые.
- **Контракты сценариев**: `DivisionService` в `internal/httpapi/deps.go` и
  `internal/mcp/deps.go` (повторяются, это исторически сложившийся дубль) получают шестой
  метод `SearchDivisions(ctx, models.DivisionSearchQuery) ([]models.AdministrativeDivision, error)`.
- **Хит-гонка**: если единица удалена между `Search` и `GetAdministrativeDivision`
  (`ErrNotFound`) — хит пропускается (не ошибка); прочие ошибки репозитория пробрасываются.

Итог S16: административное деление закрыто «по вертикали»: чтение (S13), запись (S15),
список от родителя и поиск (S16). Остаток программы — `internal/definitions/russia` и
`CHANGELOG.md` (см. `docs/todo.md`).

## Задача 1. `models`: `ParentID` и `DivisionSearchQuery`

`internal/models/query.go` — добавляем поле и новый тип:

```go
// DivisionQuery — запрос списка единиц административного деления: необязательные
// фильтры по виду и типу (пересекаются), прямой родитель и окно. Окно применяется
// после фильтра. ParentID — список прямых детей единицы; nil — корень.
type DivisionQuery struct {
	Kind     DivisionKind      // "" — без фильтра; settlement — только населённые пункты
	Type     AdminDivisionType // "" — без фильтра; иначе точное совпадение типа
	ParentID *ID               // nil — корень; иначе только прямые дети этой единицы
	Page     Page
}

func (q DivisionQuery) Validate() error {
	switch {
	case !q.Kind.Valid():
		return fieldErr("kind", "неизвестный вид %q (допустимо: %q)", q.Kind, DivisionKindSettlement)
	case q.Type != "" && !q.Type.Valid():
		return fieldErr("type", "неизвестный тип единицы деления %q", q.Type)
	case q.Page.Limit < 0:
		return fieldErr("limit", "не может быть отрицательным: %d", q.Page.Limit)
	case q.Page.Offset < 0:
		return fieldErr("offset", "не может быть отрицательным: %d", q.Page.Offset)
	}

	if q.ParentID != nil {
		return idErr("parent_id", *q.ParentID, TypeAdministrativeDivision)
	}

	return nil
}
```

`idErr` мапит `ID.Validate(TypeAdministrativeDivision)` в ошибку поля `parent_id` с
причиной «неверный формат id» / «не тот тип» (см. `internal/models/validation.go`).

Новый тип — туда же (ниже `DivisionQuery`):

```go
// DivisionSearchQuery — запрос поиска единиц административного деления по началу
// названия (включая варианты). Окно применяется после отбора единиц.
type DivisionSearchQuery struct {
	Text string // начало названия или варианта; пустое (после обрезки) — пустой результат
	Page Page
}

// Validate проверяет запрос: отрицательные размер и сдвиг окна — *ValidationError.
// Пустой текст не ошибка.
func (q DivisionSearchQuery) Validate() error {
	switch {
	case q.Page.Limit < 0:
		return fieldErr("limit", "не может быть отрицательным: %d", q.Page.Limit)
	case q.Page.Offset < 0:
		return fieldErr("offset", "не может быть отрицательным: %d", q.Page.Offset)
	}

	return nil
}
```

Шаг 1.1. TDD: `internal/models/query_test.go` — добавить:

```go
func TestDivisionQueryValidatesParentID(t *testing.T) {
	valid := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA9")
	spec := []struct {
		q       Query // или models.DivisionQuery
		wantErr bool
	}{
		{ DivisionQuery{ParentID: &valid}, false},
		{ DivisionQuery{ParentID: idPtr("nope"), }, true},
		{ DivisionQuery{ParentID: idPtr("AD-01ARZ3NDEKTSV4RRFFQ69G5FA9"), Type: "castle"}, true},
	}
	// … вариант: прямая проверка через q.Validate()
}
```

(Тест конкретно: `ParentID` с валидным id — `nil`; с невалидным — `*ValidationError`,
`Field == "parent_id"`.)

```go
func TestDivisionSearchQueryValidate(t *testing.T) {
	for _, tc := range []struct{ q DivisionSearchQuery; wantErr bool }{
		{DivisionSearchQuery{}, false},                 // пустой текст и окно — валидны
		{DivisionSearchQuery{Text: " Давыд "}, false}, // пробелы не ошибка (обрезка в сценарии)
		{DivisionSearchQuery{Page: Page{Limit: -1}}, true},
		{DivisionSearchQuery{Page: Page{Offset: -1}}, true},
		{DivisionSearchQuery{Page: Page{Limit: 501}}, false}, // сужается Normalized
	} {
		err := tc.q.Validate()
		if (err != nil) != tc.wantErr {
			t.Errorf("%+v: err = %v, wantErr %v", tc.q, err, tc.wantErr)
		}
		if tc.wantErr {
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Fatalf("ожидалась *ValidationError, got %v", err)
			}
			if ve.Field != "limit" && ve.Field != "offset" {
				t.Fatalf("field = %q, ожидалось limit или offset", ve.Field)
			}
		}
	}
}
```

Шаг 1.2. Реализация; `go test ./internal/models/` зелёный; рубеж `gofmt -l .`,
`go build ./...` (ничего не ломается — поле новое, никто его ещё не выставляет);
коммит `feat(models): ParentID в DivisionQuery и DivisionSearchQuery`.

## Задача 2. `list_divisions`: список от родителя

### 2.1 Репо-интерфейс

`internal/usecases/list_divisions/deps.go` — добавить метод (порт уже есть
`internal/store/deps.go:76`):

```go
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type AdminDivisionRepo interface {
	ListAdministrativeDivisions(ctx context.Context, access models.Access, page models.Page) ([]*models.AdministrativeDivision, error)
	// ChildrenOfDivision — прямые дочерние единицы деления parent в порядке
	// сохранения; нет такой единицы — models.ErrNotFound.
	ChildrenOfDivision(ctx context.Context, parent models.ID, access models.Access, page models.Page) ([]*models.AdministrativeDivision, error)
}
```

Перегенерация мока (mockgen установлен в `$(go env GOPATH)/bin`):
`PATH="$PATH:$(go env GOPATH)/bin" mockgen -source deps.go -destination deps_test.go -package list_divisions`

### 2.2 Сценарий

`scenario.go` — вынести выбор «корень/дети» в маленький хелпер и использовать его в цикле:

```go
// window отдаёт очередное окно исходных единиц: для корня — список всех
// ListAdministrativeDivisions, для родителя — прямых детей ChildrenOfDivision.
func (s *Scenario) window(ctx context.Context, parent *models.ID, offset int) ([]*models.AdministrativeDivision, error) {
	page := models.Page{Limit: models.MaxPageLimit, Offset: offset}

	if parent == nil {
		return s.adminDivisions.ListAdministrativeDivisions(ctx, models.AccessFull, page)
	}

	return s.adminDivisions.ChildrenOfDivision(ctx, *parent, models.AccessFull, page)
}
```

и в `ListDivisions`:

```go
for offset := 0; ; offset += models.MaxPageLimit {
	divisions, err := s.window(ctx, q.ParentID, offset)
	// … прежний код обхода, q.Matches, окна запроса
}
```

### 2.3 Фейк и тесты

`scenario_test.go`: `fakeRepo` получает метод и поля. Общую нарезку окон вынести в функцию:

```go
func window(list []*models.AdministrativeDivision, page models.Page) []*models.AdministrativeDivision {
	page = page.Normalized()
	if page.Offset >= len(list) {
		return nil
	}

	return list[page.Offset:min(page.Offset+page.Limit, len(list))]
}
```

Метод `ChildrenOfDivision` на `fakeRepo`:

```go
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
```

поля `fakeRepo` дополнить: `children []*models.AdministrativeDivision`, `gotParent *models.ID`;
`ListAdministrativeDivisions` переписать через `window(f.list, page)` (поведение то же).

Добавить тесты:

```go
// TestListDivisionsFromParentReturnsChildren: при ParentID — только прямые дети,
// корневой список не читается.
func TestListDivisionsFromParentReturnsChildren(t *testing.T) {
	repo := &fakeRepo{
		list: []*models.AdministrativeDivision{division("ad-root", models.AdminDivisionGovernorate)},
		children: []*models.AdministrativeDivision{
			division("ad-1", models.AdminDivisionSelo),
			division("ad-2", models.AdminDivisionVolost),
		},
	}
	parent := models.ID("ad-root")

	got, err := New(repo).ListDivisions(context.Background(), models.DivisionQuery{ParentID: &parent})
	if err != nil || !sameIDs(ids(got), "ad-1", "ad-2") {
		t.Fatalf("got %v, %v; ожидались ad-1, ad-2", ids(got), err)
	}
	if repo.gotParent == nil || *repo.gotParent != parent {
		t.Fatalf("gotParent = %v, ожидался %s", repo.gotParent, parent)
	}
	// корневой список не вызван: ChildrenOfDivision не трогает f.list
}

// TestListDivisionsFromParentAppliesFiltersAndWindow: вид/тип/окно — среди детей.
func TestListDivisionsFromParentAppliesFiltersAndWindow(t *testing.T) {
	// children: selo, volost, derevnya, selo — фильтр kind=settlement+limit=1+offset=1 → ad-4
}

// TestListDivisionsFromMissingParentIsNotFound: ChildrenOfDivision с ErrNotFound порта
// пробрасывается (404 наверху).
func TestListDivisionsFromMissingParentIsNotFound(t *testing.T) {
	repo := &fakeRepo{err: models.ErrNotFound}
	parent := models.ID("nope")

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
```

Шаг 2.4. Рубеж `gofmt`/`go build ./...`/`go vet ./...`/`go test ./internal/usecases/...`.
Коммит `feat(usecases): список от родителя через ChildrenOfDivision (list_divisions)`.

## Задача 3. Новый сценарий `usecases/search_divisions`

Пакет `internal/usecases/search_divisions/`.

### 3.1 TDD: `deps_test.go` (генерируемый) + `scenario_test.go`

`deps.go` — срез порта:

```go
package search_divisions

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// DivisionRepo — зависимость сценария: срез порта store.Store: глобальный поиск и
// чтение единицы по id.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type DivisionRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetAdministrativeDivision(ctx context.Context, id models.ID) (*models.AdministrativeDivision, error)
}
```

`scenario_test.go` — рукописный фейк по образцу `list_divisions`:

```go
// fakeRepo отдаёт окна поиска и загружает единицы, запоминая вызовы.
type fakeRepo struct {
	hits     []models.Hit
	units    map[models.ID]*models.AdministrativeDivision
	searchErr error
	getErr   error
	errAt    int // номер вызова Search (с 1), на котором возвращается searchErr; 0 — на любом
	gotCtx   context.Context
	searchCalls []models.Page
	accesses []models.Access
	getCalls []models.ID
}

func (f *fakeRepo) Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error) {
	f.gotCtx = ctx
	f.searchCalls = append(f.searchCalls, page)
	f.accesses = append(f.accesses, access)

	if f.searchErr != nil && (f.errAt == 0 || f.errAt == len(f.searchCalls)) {
		return nil, f.searchErr
	}

	return hitWindow(f.hits, page), nil
}
```

(хелпер `hitWindow` — аналог `window` для `[]models.Hit`; `GetAdministrativeDivision`
читает `f.units[id]`, при отсутствии возвращает `models.ErrNotFound` либо `f.getErr`.)

Тесты:

- `TestSearchDivisionsReturnsFullUnitsByHits` — два хита делений; результат — полные
  единицы (поля `name/type/parent_id`), не `Label` хита; `getCalls` получил id найденного.
- `TestSearchDivisionsSkipsNonDivisionHits` — в хитах есть `person` и `event`; окно
  считается среди делений (sift): хиты делений идут вторым/третьим окнами, а `page`
  запроса уже отработано по делениям.
- `TestSearchDivisionsWalksAllSearchWindows` — `2*MaxPageLimit+7` хитов, деления разбросаны;
  всё собранно, окна подряд с `AccessFull` и `MaxPageLimit`.
- `TestSearchDivisionsEmptyTextReturnsEmptyNoRepoCall` — `Text: "  "` → пустой не-nil срез,
  `len(f.searchCalls) == 0`.
- `TestSearchDivisionsSkipsVanishedHit` — хит деления, у которого `units` нет → пропущен;
  соседний хит вернулся; `getErr` (не-ErrNotFound) — ошибка целиком.
- `TestSearchDivisionsPropagatesSearchError` / `TestSearchDivisionsPropagatesGetError`.
- `TestSearchDivisionsInvalidQueryDoesNotTouchRepo` — `Limit:-1`, `Offset:-1` →
  `*ValidationError`, репозиторий не вызван.
- `TestSearchDivisionsPassesContextToRepo`.

### 3.2 Реализация `scenario.go`

```go
package search_divisions

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск единиц административного деления по началу названия».
type Scenario struct {
	divisions DivisionRepo
}

// New создаёт сценарий.
func New(divisions DivisionRepo) *Scenario {
	return &Scenario{divisions: divisions}
}

// SearchDivisions возвращает единицы деления, чьё название (или вариант названия)
// начинается с текста запроса (после обрезки), в порядке сохранения; окно
// применяется после отбора единиц деления. Некорректный запрос — *models.ValidationError,
// репозиторий не вызывается. Пустой текст — пустой результат. Хит, чья единица удалена
// между поиском и чтением (models.ErrNotFound), пропускается. Короткий результат —
// конец списка.
func (s *Scenario) SearchDivisions(ctx context.Context, q models.DivisionSearchQuery) ([]models.AdministrativeDivision, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	query := strings.TrimSpace(q.Text)
	if query == "" {
		return []models.AdministrativeDivision{}, nil
	}

	page := q.Page.Normalized()
	out := []models.AdministrativeDivision{}
	matched := 0 // сколько единиц деления прошло (для сдвига окна запроса)

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.divisions.Search(ctx, query, models.AccessFull,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeAdministrativeDivision {
				continue
			}

			d, err := s.divisions.GetAdministrativeDivision(ctx, h.ID)
			if err != nil {
				if !errors.Is(err, models.ErrNotFound) {
					return nil, err
				}
				continue // единица удалена между поиском и чтением
			}

			if matched >= page.Offset {
				out = append(out, *d)

				if len(out) == page.Limit {
					return out, nil
				}
			}

			matched++
		}
	}
}
```

Шаг 3.3. `gofmt`/`go test ./internal/usecases/...` зелёный; рубеж `go build ./...` (пакет
новый, никого не ломает). Коммит `feat(usecases): поиск делений (search_divisions)`.

## Задача 4. `httpapi`: `/search` и `parent_id`

### 4.1 Интерфейс

`internal/httpapi/deps.go` — шестой метод:

```go
type DivisionService interface {
	ListDivisions(ctx context.Context, q models.DivisionQuery) ([]models.AdministrativeDivision, error)
	SearchDivisions(ctx context.Context, q models.DivisionSearchQuery) ([]models.AdministrativeDivision, error)
	// … прежние четыре метода без изменений
}
```

### 4.2 Обработчики и маршрут

`internal/httpapi/division.go`:

```go
// handleDivisionSearch — GET /api/admin-divisions/search?q=&limit=&offset=.
// Синтаксически неверный параметр — 400; неверное значение (отрицательное окно) — 422.
func handleDivisionSearch(divisions DivisionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := parseDivisionSearchQuery(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := divisions.SearchDivisions(r.Context(), q)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.AdminDivisionsFromModels(list))
	}
}

// parseDivisionSearchQuery разбирает параметры поиска.
func parseDivisionSearchQuery(v url.Values) (models.DivisionSearchQuery, error) {
	q := models.DivisionSearchQuery{Text: v.Get("q")}

	var err error

	if q.Page.Limit, err = intParam(v, "limit"); err != nil {
		return q, err
	}

	if q.Page.Offset, err = intParam(v, "offset"); err != nil {
		return q, err
	}

	return q, nil
}
```

В `parseDivisionQuery` добавить `parent_id` (неверный формат — 422, не 400: пусть
`Validate` сценария, как для `{id}` в GET/как в S15):

```go
if raw := v.Get("parent_id"); raw != "" {
	pid := models.ID(raw)
	q.ParentID = &pid
}
```

`internal/httpapi/httpapi.go` — маршрут (литерал перед wildcard):

```go
mux.HandleFunc("GET /api/admin-divisions", handleDivisionList(divisions))
mux.HandleFunc("GET /api/admin-divisions/search", handleDivisionSearch(divisions))
mux.HandleFunc("GET /api/admin-divisions/{id}", handleDivisionGet(divisions))
```

### 4.3 Юнит-тесты `division_test.go`

`fakeDivisions` дополнить полями `search []models.AdministrativeDivision`,
`searchErr error`, `gotSearchQuery models.DivisionSearchQuery` и методом:

```go
func (f *fakeDivisions) SearchDivisions(ctx context.Context, q models.DivisionSearchQuery) ([]models.AdministrativeDivision, error) {
	f.gotSearchQuery = q

	return f.search, f.searchErr
}
```

Тесты (в стиле существующих, хелпер `get()`):

- `TestDivisionSearch` — `GET /api/admin-divisions/search?q=давы`: 200, JSON-массив
  `[{"id":…, "name":…, "type":…, "parent_id":…}]`; `gotSearchQuery.Text == "давы"`.
- `TestDivisionSearchInvalidLimitIs400` — `limit=x` → 400 (синтаксис, `intParam`).
- `TestDivisionSearchNegativeLimitIs422` — `limit=-1` → 422 (валидация сценария).
- `TestDivisionSearchNotFoundParentIs404` — `parent_id` отсутствующего родителя в списке →
  404 (через `fakeDivisions{err: models.ErrNotFound}`).
- **`TestDivisionSearchRouteDoesNotHitID`** — «родителя нет, но `/search` — не id»:
  `GET /api/admin-divisions/search?q=давы` при пустом фейке отвечает 200 пустым массивом,
  **не** 422 «неверный формат id»: маршрут литеральный.
- Существующий `TestDivisionList…`: проверить `parent_id` — вызвать
  `GET /api/admin-divisions?parent_id=AD-…` и убедиться, что `gotQuery.ParentID == &id` и
  результат = список детей фейка.

### 4.4 e2e на реальном store (`store_test.go`)

Фасад `divisionService` дополнить: поле `search *search_divisions.Scenario`, метод
`SearchDivisions`, `newDivisionService` → `search: search_divisions.New(st)`
(импорт `search_divisions`).

В `TestAdminDivisionsWithRealStore` (или отдельный `TestAdminDivisionsSearchWithRealStore`,
но дешевле добавить в существующий стол `cases`) — с данными из S15 (root + ad-1..ad-3):

```go
{"/api/admin-divisions/search?q=давы", http.StatusOK,
	`[{"id":"ad-1","name":"Давыдово","type":"selo","parent_id":"ad-root"}]`},
{"/api/admin-divisions/search?q=", http.StatusOK, `[]`},        // пустой запрос — пустой результат
{"/api/admin-divisions/search?q=ниц", http.StatusOK,
	`[{"id":"ad-2","name":"Никифоровская","type":"volost","parent_id":"ad-root"},{"id":"ad-3","name":"Никифорово","type":"derevnya","parent_id":"ad-root"}]`},
{"/api/admin-divisions/search?q=ник", http.StatusOK, `[]`},     // суффикс не ищется (префикс) — оставить как есть, ИЛИ поправить под реальные данные
{"/api/admin-divisions?parent_id=ad-root", http.StatusOK,
	`[{"id":"ad-1",…},{"id":"ad-2",…},{"id":"ad-3",…}]`},         // прямые дети, без parent — 404
{"/api/admin-divisions?parent_id=nope", http.StatusNotFound, ""},
{"/api/admin-divisions?parent_id=not-an-id", http.StatusUnprocessableEntity, ""},
```

Прим.: поиск — префиксный, поэтому `q=ник` (начальные буквы «Ник») вернёт две единицы, а
`q=ик` — пусто. Подставить актуальные ожидания на данных теста (вариант названия проверить:
добавить `ad-2` вариантом, например `Variants:[]string{"Никифоровск"}` — тогда `q=никиф`
находит обе). Точные строки сверяются с фактическим выводом `handleDivisionList`/`writeJSON`.

Шаг 4.5. Рубеж: всё зелёное; smoke: `go run ./cmd/genodex -p 9017`, затем
`curl '/api/admin-divisions/search?q=ниц'` и `curl '/api/admin-divisions?parent_id=…'`.
Коммит `feat(httpapi): GET /api/admin-divisions/search и parent_id в списке`.

## Задача 5. `mcp`: тул `division_search` и `parent_id`

### 5.1 Интерфейс

`internal/mcp/deps.go` — шестой метод (идентично httpapi):

```go
type DivisionService interface {
	ListDivisions(ctx context.Context, q models.DivisionQuery) ([]models.AdministrativeDivision, error)
	SearchDivisions(ctx context.Context, q models.DivisionSearchQuery) ([]models.AdministrativeDivision, error)
	// … прежние четыре
}
```

### 5.2 `mcp/division.go`

- В `division_list` добавить аргумент:

```go
mcp.WithString("parent_id", mcp.Description("id родительской единицы; пусто — корень (весь список)")),
```

и `divisionQueryFromRequest`:

```go
q.ParentID = optionalParentID(req)
```

- Новый тул:

```go
tool = mcp.NewTool(
	"division_search",
	mcp.WithDescription("Поиск единиц административного деления по началу названия (включая варианты названий); результат — JSON-массив единиц. Пустой q — пустой результат"),
	mcp.WithString("q", mcp.Required(), mcp.Description("Начало названия или варианта")),
	mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
	mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
)
s.AddTool(tool, divisionSearchHandler(divisions))
```

- Обработчик (стиль `divisionListHandler`):

```go
func divisionSearchHandler(divisions DivisionService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.DivisionSearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := divisions.SearchDivisions(ctx, q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		data, err := json.Marshal(transport.AdminDivisionsFromModels(list))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сериализовать список: %v", err)), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}
```

### 5.3 Тесты `mcp/division_test.go`

`fakeDivisions` дополнить так же, как httpapi-фейк (поля `search`, `searchErr`,
`gotSearchQuery`, метод `SearchDivisions`). Тесты по образцу S15 (`callTool`, `toolText`,
`wantToolError`):

- `TestDivisionSearchTool` — «давы» → JSON-массив двух единиц; `gotSearchQuery.Text`.
- `TestDivisionSearchToolEmptyText` — пустой `q` → `[]`.
- `TestDivisionSearchToolInvalidLimitIsError` — `limit` не число → `wantToolError`.
- `TestDivisionListToolWithParent` — `parent_id` родителя → `gotQuery.ParentID` выставлен,
  результат детей фейка.
- `TestNewServerRegistersDivisionSearchTool` — дополнить список имён `"division_search"`
  (отдельный тест, чтобы существующий регресс не трогать или расширить существующий
  `TestNewServerRegistersDivisionWriteTools`).

Шаг 5.4. `go test ./internal/mcp/` зелёный; рубеж; коммит
`feat(mcp): тул division_search и parent_id в division_list`.

## Задача 6. `internal/app` и `web`

### 6.1 `internal/app/app.go`

Фасад (стиль S13/S15) — шестое поле и метод, синхронно с интерфейсом:

```go
import search_divisions "github.com/amarin/genodex/internal/usecases/search_divisions"

type divisionService struct {
	list   *list_divisions.Scenario
	search *search_divisions.Scenario
	get    *get_division.Scenario
	create *create_division.Scenario
	update *update_division.Scenario
	del    *delete_division.Scenario
}

func (s *divisionService) SearchDivisions(ctx context.Context, q models.DivisionSearchQuery) ([]models.AdministrativeDivision, error) {
	return s.search.SearchDivisions(ctx, q)
}
```

в `New`:

```go
divisions := &divisionService{
	list:   list_divisions.New(st),
	search: search_divisions.New(st),
	get:    get_division.New(st),
	create: create_division.New(st, idgen.New()),
	update: update_division.New(st),
	del:    delete_division.New(st),
}
```

### 6.2 `web/src/api.ts`

```ts
export interface AdminDivisionQuery {
  kind?: "settlement";
  type?: string;
  parent_id?: string;
  limit?: number;
  offset?: number;
}

export interface DivisionSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function searchAdminDivisions(
  query: DivisionSearchQuery,
): Promise<AdminDivision[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/admin-divisions/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}
```

### 6.3 `web/src/App.tsx` — поиск и переход к дочерним в `SettlementsTab`

Замена содержимого `SettlementsTab` (по образцу текущего, но с `Input.Search` и кнопкой
возврата; точные имена/раскладку не меняем сверх необходимости):

```tsx
function SettlementsTab() {
  const [items, setItems] = useState<AdminDivision[]>([]);
  const [parent, setParent] = useState<AdminDivision | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searching, setSearching] = useState(false);

  const load = (q: AdminDivisionQuery) => {
    setLoading(true);
    setError(null);
    fetchAdminDivisions(q)
      .then(setItems)
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    load({ kind: "settlement", limit: MAX_PAGE_LIMIT });
  }, []);

  const onSearch = (value: string) => {
    const q = value.trim();
    if (!q) {
      return;
    }
    setSearching(true);
    searchAdminDivisions({ q, limit: MAX_PAGE_LIMIT })
      .then(setItems)
      .catch((e: Error) => setError(e.message))
      .finally(() => setSearching(false));
  };

  const openChildren = (node: AdminDivision) => {
    setParent(node);
    load({ parent_id: node.id, limit: MAX_PAGE_LIMIT });
  };

  const toRoot = () => {
    setParent(null);
    load({ kind: "settlement", limit: MAX_PAGE_LIMIT });
  };

  return (
    <Card
      title={
        <>
          Населённые пункты
          {parent != null && (
            <Button type="link" onClick={toRoot} style={{ marginLeft: 12 }}>
              ← к корню
            </Button>
          )}
        </>
      }
    >
      <Input.Search
        placeholder="Поиск по названию…"
        allowClear
        enterButton
        loading={searching}
        onSearch={onSearch}
      />
      {loading && <Spin />}
      {error != null && <Alert type="error" showIcon message={error} />}
      {!loading && error == null && (
        <List
          dataSource={items}
          locale={{ emptyText: "Найдено пусто" }}
          renderItem={(s) => (
            <List.Item>
              <Typography.Text strong>{s.name}</Typography.Text>
              <Typography.Text type="secondary">{s.id}</Typography.Text>
              <Button type="link" onClick={() => openChildren(s)}>
                дети
              </Button>
            </List.Item>
          )}
        />
      )}
    </Card>
  );
}
```

импорт дополнить `Button, Input` из antd и `searchAdminDivisions`.

Шаг 6.4. `cd web && npm run typecheck && npm run build`; коммит `web/dist` (обновление
`//go:embed`-ассета включено в коммит); `go build ./...` после него.

Шаг 6.5. Рубеж; smoke: `/api/health`, `/api/admin-divisions`, `/api/admin-divisions/search?q=`,
`/` (web отдаёт новую сборку).
Коммит `feat(web): поиск и переход к дочерним в «Населённых пунктах»`.

## Задача 7. Документы и рубеж

- `docs/plans/2026-09-20-core-rw-roadmap.md` — раздел S16: «Решения этапа» (см. ниже) +
  ссылка на этот план.
- `docs/usage.md`:
  - строка таблицы `/api/admin-divisions`: добавить `GET /api/admin-divisions/search` —
    поиск по началу названия (параметры `q`, `limit`, `offset`), `parent_id` — список
    прямых детей;
  - MCP-тулы: `division_search`; в `division_list` — аргумент `parent_id`;
  - новый раздел «Поиск и дочерние деления»: curl-примеры, коды 400/404/422,
    упоминание вариантов названий.
- `docs/data-model/core-read-write.md` §5 (после bullet про ошибки S14, строки 268–273) —
  bullet «Контракты поиска и родителя делений (S16)».
- `docs/todo.md`:
  - «Следующий проход»: «Выполнены S1–S15 … остался S16» → «Выполнены S1–S16; остаток
    программы: перевод `internal/definitions/russia`, `CHANGELOG.md`, вопрос доступа»;
  - раздел C «Публичные контракты»: отметить деления закрытыми (S13–S16); оставить
    людей/события как цели на новый проход;
  - «Открытые вопросы»: снять пункты, закрытые на S16 (контракт поиска/родителя — решён;
    `Access` во внешних контрактах — остаётся).
- Шаг 7.1. Правки; рубеж `gofmt -l .`, `go build ./...`, `go vet ./...`, `go test ./...`,
  smoke по полному набору (`/api/health`, `/api/admin-divisions`, `/api/admin-divisions/search`,
  `/` + write-сценарий S15). Коммит `docs: S16 — поиск и дочерние деления в контрактах`.

### Решения этапа (для roadmap и history)

1. Поиск — отдельный эндпоинт `GET /api/admin-divisions/search` (литеральный маршрут
   побеждает `{id}` в Go 1.22 ServeMux; закреплено тестом `/search` ≠ «неверный формат id»).
   Параметр `q`; результат — полные `[]AdminDivision`, не хиты порта.
2. Поиск префиксный, по полю `name`, куда индексируются `Name` и `Variants`
   (варианты названий участвуют). Пустой `q` после `TrimSpace` — пустой результат без
   обращения к репозиторию (защита от «найди всё»).
3. Дочерние деления — на уровне репозитория (`ChildrenOfDivision` порта);
   `list_divisions` при `ParentID != nil` берёт окна оттуда, разделяя код обхода/
   фильтра/окна с корневым списком. Несуществующий родитель — 404
   (`ErrNotFound` порта); неверный формат `parent_id` — 422, репозиторий не вызывается.
4. Окно поиска считается среди единиц деления (после отбора по типу), зеркально
   списку; поэтому окна `Search` могут читаться дальше нужной позиции — это принятая
   цена схемы «хиты глобального поиска».
5. Хит, чья единица удалена между поиском и чтением (`ErrNotFound`), пропускается,
   не ошибка; прочие ошибки репозитория — ошибка сценария.
6. Валидация окон обоих запросов — в `models` (`*ValidationError`, поля `limit`/`offset`);
   `DivisionSearchQuery` и `DivisionQuery.ParentID` валидируются до вызова репозитория.
7. `DivisionService` (httpapi и mcp) — единый интерфейс шести методов; фасад
   `app.divisionService` собирает шесть сценариев; все фейки/фасады и `store_test.go`
   расширены в том же коммите, что и интерфейс (урок S15).
8. Web: поиск + переход к дочерним в «Населённых пунктах»; `web/dist` пересобирается и
   коммитится; сервер отдаёт SPA `/` из встроенных ассетов.
9. Итог S16: деления закрыты по вертикали «чтение + список от родителя + поиск + запись»
   (S13–S16) — образец для людей/событий в будущем проходе.
10. (если во время e2e окажется иная семантика поиска/родителя — зафиксировать фактическое
    поведение и поправить ожидания тестов/документацию, не меняя контракт сценария).

## Коммиты

1. `docs(plans): план этапа S16 — поиск и дочерние деления в контрактах`
2. `feat(models): ParentID в DivisionQuery и DivisionSearchQuery`
3. `feat(usecases): список от родителя через ChildrenOfDivision (list_divisions)`
4. `feat(usecases): поиск делений (search_divisions)`
5. `feat(httpapi): GET /api/admin-divisions/search и parent_id в списке`
6. `feat(mcp): тул division_search и parent_id в division_list`
7. `feat(web): поиск и переход к дочерним в «Населённых пунктах»`
8. `docs: S16 — поиск и дочерние деления в контрактах`