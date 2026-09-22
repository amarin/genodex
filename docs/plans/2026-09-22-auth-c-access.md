# Этап C. Access в контрактах: план этапа

Спека — [auth.md](../data-model/auth.md) §6 (плюс попутные ссылки на §4/§5);
дорожная карта — [auth-roadmap.md](2026-09-22-auth-roadmap.md), раздел C.
Формат — как этапы A/A2/B/B2. Исполняется в `main` подрядными коммитами.

## Goal

Закрыть исходный долг «Access в контрактах»: `internal/auth` реально
подключается к `/api` и `/mcp` — чтение делений получает `Access` из
контекста запроса (пока без видимого эффекта — `Private` у
`AdministrativeDivision` нет, — но проводка перестаёт быть хардкодом), а
запись (`POST`/`PUT`/`DELETE` в `/api/admin-divisions`) требует вошедшего
владельца (`401` анониму). MCP уже полностью гейтится bearer-токеном с
этапа B2 — здесь он реально монтируется за `RequireAPIToken`.

## Предпосылка: что уже есть, что меняется

- `internal/usecases/list_divisions.Scenario.ListDivisions` и
  `internal/usecases/search_divisions.Scenario.SearchDivisions` сейчас сами
  вызывают репозиторий с захардкоженным `models.AccessFull` — при том что их
  собственные Repo-порты (`AdminDivisionRepo`/`DivisionRepo`) уже принимают
  `access models.Access` параметром. Меняется факт проводки: `access`
  становится параметром сценария вместо константы внутри него.
- `httpapi.DivisionService`/`mcp.DivisionService`: `ListDivisions`/
  `SearchDivisions` получают `access models.Access` вторым параметром (сразу
  после `ctx`, перед основным запросом) — auth.md §6.
- Запись (`POST`/`PUT`/`DELETE /api/admin-divisions...`) сейчас **не
  защищена вообще** — любой аноним может создать/изменить/удалить деление.
  Это дыра, а не решение (см. память программы) — задача 2 закрывает её
  через уже существующий `requireFull` (этап B, `internal/httpapi/auth.go`).
  MCP-письмо ничего похожего не получает — оно и так недостижимо анонимно,
  как только `/mcp` реально обёрнут `mcp.RequireAPIToken` (задача 4).
- **Архитектурная нестыковка, которую закрывает задача 2 (её вторая половина):** `httpapi.
  NewAuthHandler` сам оборачивает **только свой** маршрут (`resolveAccess`+
  `requireCSRFHeader` вокруг `/api/auth/*`), а `httpapi.NewHandler`
  (`/api/admin-divisions`, `/api/docs`, `/api/health`) вообще не обёрнут —
  исходный замысел дизайна (auth.md §4) — оба миддлвари вокруг **всего**
  `/api/`. `resolveAccess`/`requireCSRFHeader` неэкспортируемы — собрать их
  вокруг `NewHandler` из `internal/app` невозможно. Решение: новый
  экспортируемый `httpapi.NewAPIHandler(divisions, auth, docsFS)` —
  регистрирует ВСЕ маршруты (`admin-divisions`, `docs`, `health`, `auth/*`)
  на одном `http.ServeMux` и оборачивает его **один раз**. `NewHandler` и
  `NewAuthHandler` остаются как есть (используются ~40 существующими
  юнит-тестами пакета напрямую, без auth) — общий код регистрации маршрутов
  выносится в неэкспортируемые `registerDivisionRoutes`/`registerAuthRoutes`,
  которые вызывают и старые функции, и новая. Поведение `NewHandler`/
  `NewAuthHandler` не меняется — только не дублируется.
- `internal/store/sqlstore.Store` не отдаёт своё `*storage.DB` наружу — а
  `auth.NewSQLStore` его требует. Чтобы не открывать файл БД дважды (второе
  соединение на тот же SQLite-файл — лишний файловый хендл при
  `SetMaxOpenConns(1)` на каждое), добавляется однострочный аксессор
  `Store.DB() *storage.DB`, использующий уже открытое соединение.
- MCP-письмо (`division_create`/`update`/`delete`) НЕ получает отдельного
  гейта внутри `internal/mcp/division.go` — весь `/mcp` уже недостижим
  анонимно, как только он обёрнут `mcp.RequireAPIToken` (этап B2, задача 4
  этого плана). Точечный гейт на уровне тула был бы дублирующей, ничего не
  добавляющей проверкой.

## Задача 1. Access в сценариях `list_divisions`/`search_divisions`

**Файлы:**
- Изменить: `internal/usecases/list_divisions/scenario.go`,
  `internal/usecases/list_divisions/scenario_test.go`
- Изменить: `internal/usecases/search_divisions/scenario.go`,
  `internal/usecases/search_divisions/scenario_test.go`

`internal/usecases/list_divisions/scenario.go` — полное содержимое файла:

```go
package list_divisions

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список единиц административного деления».
type Scenario struct {
	adminDivisions AdminDivisionRepo
}

// New создаёт сценарий.
func New(adminDivisions AdminDivisionRepo) *Scenario {
	return &Scenario{adminDivisions: adminDivisions}
}

// ListDivisions возвращает единицы деления, прошедшие фильтры запроса, в порядке
// сохранения; окно (размер и сдвиг) применяется после фильтра. С ParentID —
// только прямые дети этой единицы. Некорректный запрос — *models.ValidationError,
// репозиторий не вызывается. Короткий результат (меньше размера окна) означает
// конец списка. access прокидывается в репозиторий как получен — сейчас не
// влияет на результат (у AdministrativeDivision нет Private), задел для
// будущих срезов, где Private есть (auth.md §6).
func (s *Scenario) ListDivisions(ctx context.Context, access models.Access, q models.DivisionQuery) ([]models.AdministrativeDivision, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	if q.ParentID != nil {
		return s.listChildren(ctx, access, q, *q.ParentID)
	}

	page := q.Page.Normalized()
	out := []models.AdministrativeDivision{}
	matched := 0 // сколько единиц прошло фильтр (для сдвига окна)

	// репозиторий отдаёт окна: обходим их до пустого или до заполнения окна запроса
	for offset := 0; ; offset += models.MaxPageLimit {
		divisions, err := s.adminDivisions.ListAdministrativeDivisions(ctx, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(divisions) == 0 {
			return out, nil
		}

		var full bool
		if full, matched, out = applyWindow(q, divisions, page, matched, out); full {
			return out, nil
		}
	}
}

// listChildren — ветка «дети родителя»: тот же обход окон, но по ChildrenOfDivision.
func (s *Scenario) listChildren(ctx context.Context, access models.Access, q models.DivisionQuery, parent models.ID) ([]models.AdministrativeDivision, error) {
	page := q.Page.Normalized()
	out := []models.AdministrativeDivision{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		divisions, err := s.adminDivisions.ChildrenOfDivision(ctx, parent, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(divisions) == 0 {
			return out, nil
		}

		var full bool
		if full, matched, out = applyWindow(q, divisions, page, matched, out); full {
			return out, nil
		}
	}
}

// applyWindow прогоняет одно окно репозитория через фильтр запроса и накапливает
// результат окна запроса. full — окно запроса заполнено (обход можно остановить).
func applyWindow(q models.DivisionQuery, divisions []*models.AdministrativeDivision,
	page models.Page, matched int, out []models.AdministrativeDivision,
) (full bool, nextMatched int, nextOut []models.AdministrativeDivision) {
	for _, d := range divisions {
		if !q.Matches(*d) {
			continue
		}

		if matched >= page.Offset {
			out = append(out, *d)

			if len(out) == page.Limit {
				return true, matched, out
			}
		}

		matched++
	}

	return false, matched, out
}
```

`internal/usecases/search_divisions/scenario.go` — полное содержимое файла:

```go
package search_divisions

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск единиц административного деления».
type Scenario struct {
	adminDivisions DivisionRepo
}

// New создаёт сценарий.
func New(adminDivisions DivisionRepo) *Scenario {
	return &Scenario{adminDivisions: adminDivisions}
}

// SearchDivisions находит единицы деления, названия (или варианты) которых
// начинаются с текста запроса, и возвращает их целиком (id, название, тип,
// родитель). Результат — в том порядке, в каком их отдаёт Search. Окно
// (размер и сдвиг) применяется после отбора хитов по типу деления: хиты других
// сущностей не расходуют окно. Пустой текст (после обрезки) — пустой результат
// без обращения к репозиторию. Хит, чья единица удалена между поиском и чтением
// (ErrNotFound), пропускается; прочие ошибки пробрасываются. access
// прокидывается в Search как получен (см. list_divisions.ListDivisions).
func (s *Scenario) SearchDivisions(ctx context.Context, access models.Access, q models.DivisionSearchQuery) ([]models.AdministrativeDivision, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.AdministrativeDivision{}, nil
	}

	page := q.Page.Normalized()
	out := []models.AdministrativeDivision{}
	matched := 0 // сколько division-хитов прошло (для сдвига окна)

	// репозиторий отдаёт окна хитов: обходим их до пустого или до заполнения окна.
	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.adminDivisions.Search(ctx, text, access,
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

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.adminDivisions.GetAdministrativeDivision(ctx, h.ID)
			if err != nil {
				if errors.Is(err, models.ErrNotFound) {
					matched++ // хит без живой единицы не расходует окно, но сдвиг вперёд
					continue
				}

				return nil, err
			}

			out = append(out, *got)

			if len(out) == page.Limit {
				return out, nil
			}

			matched++
		}
	}
}
```

- [ ] Шаг 1.1. Заменить оба файла целиком на код выше.

- [ ] Шаг 1.2. `internal/usecases/list_divisions/scenario_test.go` — найти тест
  `TestListDivisionsWalksAllRepositoryWindows` и заменить его целиком на:

```go
// TestListDivisionsWalksAllRepositoryWindows: единицы за первым окном репозитория
// не теряются, окна запрашиваются подряд; access, переданный вызывающим,
// доходит до каждого окна репозитория как есть (не захардкожен на AccessFull —
// подмена на AccessPublic здесь и есть доказательство).
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

	got, err := New(repo).ListDivisions(context.Background(), models.AccessPublic,
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
		if a != models.AccessPublic {
			t.Errorf("вызов %d: доступ %v, ожидался AccessPublic (переданный вызывающим, не захардкоженный AccessFull)", i+1, a)
		}
	}

	all := &fakeRepo{list: repo.list}

	got, err = New(all).ListDivisions(context.Background(), models.AccessPublic, models.DivisionQuery{Kind: models.DivisionKindSettlement,
		Page: models.Page{Limit: models.MaxPageLimit, Offset: models.MaxPageLimit}})
	if err != nil || len(got) != (total+1)/2-models.MaxPageLimit {
		t.Fatalf("второе окно: %d, %v; ожидалось %d", len(got), err, (total+1)/2-models.MaxPageLimit)
	}
}
```

- [ ] Шаг 1.3. В том же файле — во ВСЕХ остальных вызовах `New(...).ListDivisions(
  <ctx-выражение>, <q-выражение>)` (их 16, не считая уже переписанного в шаге
  1.2 теста, который вызывает `ListDivisions` дважды) добавить `models.AccessFull`
  вторым аргументом — между `<ctx-выражение>` и `<q-выражение>`. Например,
  было:
  ```go
  got, err := New(sample()).ListDivisions(context.Background(), models.DivisionQuery{})
  ```
  стало:
  ```go
  got, err := New(sample()).ListDivisions(context.Background(), models.AccessFull, models.DivisionQuery{})
  ```
  Это чисто механическая правка — `go build ./internal/usecases/list_divisions/...`
  не соберётся, пока не поправлен каждый вызов; используй ошибки компиляции
  как чек-лист (все они — `not enough arguments in call to ListDivisions`).

- [ ] Шаг 1.4. `internal/usecases/search_divisions/scenario_test.go` — в
  `fakeRepo` добавить поле и запись в него:
  ```go
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
  ```
  и в методе `Search`, сразу после `f.calls = append(f.calls, page)`:
  ```go
  	f.accesses = append(f.accesses, access)
  ```

- [ ] Шаг 1.5. В том же файле — добавить новый тест (после
  `TestSearchInvalidQueryDoesNotTouchRepo`, последнего в файле):
  ```go
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

  	if len(repo.accesses) != 1 || repo.accesses[0] != models.AccessPublic {
  		t.Fatalf("accesses = %v, ожидался один вызов с AccessPublic", repo.accesses)
  	}
  }
  ```

- [ ] Шаг 1.6. В том же файле — во всех ОСТАЛЬНЫХ (7) вызовах
  `New(...).SearchDivisions(context.Background(), <q-выражение>)` добавить
  `models.AccessFull` вторым аргументом, тем же способом, что в шаге 1.3.
  `go build ./internal/usecases/search_divisions/...` — чек-лист.

- [ ] Шаг 1.7. `go test ./internal/usecases/list_divisions/... ./internal/usecases/search_divisions/...`
  — зелено. `gofmt -w internal/usecases/list_divisions/ internal/usecases/search_divisions/`.

- [ ] Шаг 1.8. Рубеж: `go build ./...` — провалится в `internal/httpapi`,
  `internal/mcp`, `internal/app` (их черёд — задачи 2, 4, 5); это ожидаемо на
  этом шаге, не чинить здесь. Коммит:
  `git add internal/usecases/list_divisions internal/usecases/search_divisions`
  `feat(usecases): access параметром в ListDivisions/SearchDivisions вместо AccessFull внутри`.

## Задача 2. httpapi: интерфейс, Access при чтении, гейт при записи

**Файлы:**
- Изменить: `internal/httpapi/deps.go`, `internal/httpapi/division.go`,
  `internal/httpapi/division_write.go`, `internal/httpapi/division_test.go`,
  `internal/httpapi/division_write_test.go`

- [ ] Шаг 2.1. `internal/httpapi/deps.go` — заменить сигнатуры `ListDivisions`/
  `SearchDivisions` в интерфейсе `DivisionService`:
  ```go
  // DivisionService — контракт сценариев административного деления, отдаваемых
  // в HTTP: список, чтение, создание, изменение, удаление.
  type DivisionService interface {
  	ListDivisions(ctx context.Context, access models.Access, q models.DivisionQuery) ([]models.AdministrativeDivision, error)
  	SearchDivisions(ctx context.Context, access models.Access, q models.DivisionSearchQuery) ([]models.AdministrativeDivision, error)
  	GetDivision(ctx context.Context, id models.ID) (models.AdministrativeDivision, error)
  	CreateDivision(ctx context.Context, d models.AdministrativeDivision) (models.AdministrativeDivision, error)
  	UpdateDivision(ctx context.Context, d models.AdministrativeDivision) error
  	DeleteDivision(ctx context.Context, id models.ID) error
  }
  ```
  (только `DivisionService` — `AuthService` в этом файле не трогать.)

- [ ] Шаг 2.2. `internal/httpapi/division.go` — в `handleDivisionList` заменить
  ```go
  		list, err := divisions.ListDivisions(r.Context(), q)
  ```
  на
  ```go
  		list, err := divisions.ListDivisions(r.Context(), AccessFromContext(r.Context()), q)
  ```
  и в `handleDivisionSearch` заменить
  ```go
  		list, err := divisions.SearchDivisions(r.Context(), q)
  ```
  на
  ```go
  		list, err := divisions.SearchDivisions(r.Context(), AccessFromContext(r.Context()), q)
  ```
  (`AccessFromContext` уже объявлена в `internal/httpapi/middleware.go`, того же
  пакета — импортов не добавлять.)

- [ ] Шаг 2.3. `internal/httpapi/division_write.go` — заменить файл целиком:
  ```go
  package httpapi

  import (
  	"encoding/json"
  	"net/http"
  	"strings"

  	"github.com/amarin/genodex/internal/models"
  	"github.com/amarin/genodex/internal/transport"
  )

  // handleDivisionGet — GET /api/admin-divisions/{id}. Неверный формат id — 422
  // (ValidationError сценария), отсутствующая единица — 404. Чтение открыто
  // анонимному посетителю (auth.md §6 — Access здесь не проверяется).
  func handleDivisionGet(divisions DivisionService) http.HandlerFunc {
  	return func(w http.ResponseWriter, r *http.Request) {
  		d, err := divisions.GetDivision(r.Context(), pathDivisionID(r))
  		if err != nil {
  			writeError(w, err)

  			return
  		}

  		writeJSON(w, http.StatusOK, transport.AdminDivisionFromModel(d))
  	}
  }

  // handleDivisionCreate — POST /api/admin-divisions: создаёт единицу, отвечает
  // 201 с созданной единицей (id генерирует сценарий). Запись — только для
  // вошедшего владельца (auth.md §6, решение 9): без активной сессии — 401
  // раньше разбора тела, сценарий не вызывается.
  func handleDivisionCreate(divisions DivisionService) http.HandlerFunc {
  	return func(w http.ResponseWriter, r *http.Request) {
  		if _, ok := requireFull(w, r); !ok {
  			return
  		}

  		var in transport.AdminDivisionCreate
  		if err := decodeJSON(r, &in); err != nil {
  			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

  			return
  		}

  		created, err := divisions.CreateDivision(r.Context(), in.Model())
  		if err != nil {
  			writeError(w, err)

  			return
  		}

  		writeJSON(w, http.StatusCreated, transport.AdminDivisionFromModel(created))
  	}
  }

  // handleDivisionUpdate — PUT /api/admin-divisions/{id}: полная замена полей
  // name/type/parent_id; прочие поля текущей модели сохраняются (обработчик
  // берёт версию через get_division и накладывает поля запроса). Запись —
  // только для вошедшего владельца, см. handleDivisionCreate.
  func handleDivisionUpdate(divisions DivisionService) http.HandlerFunc {
  	return func(w http.ResponseWriter, r *http.Request) {
  		if _, ok := requireFull(w, r); !ok {
  			return
  		}

  		id := pathDivisionID(r)

  		var in transport.AdminDivisionUpdate
  		if err := decodeJSON(r, &in); err != nil {
  			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

  			return
  		}

  		cur, err := divisions.GetDivision(r.Context(), id)
  		if err != nil {
  			writeError(w, err)

  			return
  		}

  		cur.Name = in.Name
  		cur.Type = in.Type
  		cur.ParentID = cloneID(in.ParentID)

  		if err := divisions.UpdateDivision(r.Context(), cur); err != nil {
  			writeError(w, err)

  			return
  		}

  		writeJSON(w, http.StatusOK, transport.AdminDivisionFromModel(cur))
  	}
  }

  // handleDivisionDelete — DELETE /api/admin-divisions/{id}: 204 без тела;
  // занятая единица — 409 со списком ссылающихся. Запись — только для
  // вошедшего владельца, см. handleDivisionCreate.
  func handleDivisionDelete(divisions DivisionService) http.HandlerFunc {
  	return func(w http.ResponseWriter, r *http.Request) {
  		if _, ok := requireFull(w, r); !ok {
  			return
  		}

  		if err := divisions.DeleteDivision(r.Context(), pathDivisionID(r)); err != nil {
  			writeError(w, err)

  			return
  		}

  		w.WriteHeader(http.StatusNoContent)
  	}
  }

  func pathDivisionID(r *http.Request) models.ID {
  	return models.ID(strings.TrimPrefix(r.PathValue("id"), "/"))
  }

  func cloneID(p *models.ID) *models.ID {
  	if p == nil {
  		return nil
  	}

  	v := *p

  	return &v
  }

  func decodeJSON(r *http.Request, v any) error {
  	return json.NewDecoder(r.Body).Decode(v)
  }
  ```

- [ ] Шаг 2.4. `internal/httpapi/division_test.go` — в `fakeDivisions` добавить
  два поля:
  ```go
  	gotListAccess   models.Access
  	gotSearchAccess models.Access
  ```
  (в блок объявления полей структуры, рядом с `got`/`gotSearch`) и заменить
  методы:
  ```go
  func (f *fakeDivisions) ListDivisions(_ context.Context, access models.Access, q models.DivisionQuery) ([]models.AdministrativeDivision, error) {
  	f.got = q
  	f.gotListAccess = access

  	return f.list, f.err
  }

  func (f *fakeDivisions) SearchDivisions(_ context.Context, access models.Access, q models.DivisionSearchQuery) ([]models.AdministrativeDivision, error) {
  	f.gotSearch = q
  	f.gotSearchAccess = access

  	return f.search, f.searchErr
  }
  ```
  Прочие методы `fakeDivisions` не трогать.

- [ ] Шаг 2.5. В том же файле — добавить два новых теста (после
  `TestOldSettlementsRouteIsGone`, последнего в файле):
  ```go
  // TestDivisionListPassesAccessFromContext: Access, положенный resolveAccess в
  // контекст запроса, доходит до сценария как есть (не захардкожен на
  // AccessFull — auth.md §6, приёмка этапа C).
  func TestDivisionListPassesAccessFromContext(t *testing.T) {
  	svc := &fakeDivisions{}

  	req := httptest.NewRequest(http.MethodGet, "/api/admin-divisions", nil)
  	req = req.WithContext(context.WithValue(req.Context(), accessCtxKey, models.AccessPublic))

  	rec := httptest.NewRecorder()
  	NewHandler(svc, fstest.MapFS{}).ServeHTTP(rec, req)

  	if rec.Code != http.StatusOK {
  		t.Fatalf("status = %d", rec.Code)
  	}
  	if svc.gotListAccess != models.AccessPublic {
  		t.Fatalf("gotListAccess = %v, ожидался AccessPublic", svc.gotListAccess)
  	}
  }

  // TestDivisionSearchPassesAccessFromContext: аналогично для поиска.
  func TestDivisionSearchPassesAccessFromContext(t *testing.T) {
  	svc := &fakeDivisions{}

  	req := httptest.NewRequest(http.MethodGet, "/api/admin-divisions/search?q=давы", nil)
  	req = req.WithContext(context.WithValue(req.Context(), accessCtxKey, models.AccessPublic))

  	rec := httptest.NewRecorder()
  	NewHandler(svc, fstest.MapFS{}).ServeHTTP(rec, req)

  	if rec.Code != http.StatusOK {
  		t.Fatalf("status = %d", rec.Code)
  	}
  	if svc.gotSearchAccess != models.AccessPublic {
  		t.Fatalf("gotSearchAccess = %v, ожидался AccessPublic", svc.gotSearchAccess)
  	}
  }
  ```
  (файл — `package httpapi`, `accessCtxKey` — неэкспортируемая константа
  `internal/httpapi/middleware.go` того же пакета, `context`/`models`/`fstest`
  уже импортированы в этом файле.)

- [ ] Шаг 2.6. `internal/httpapi/division_write_test.go` — добавить импорты
  `"context"` и `authpkg "github.com/amarin/genodex/internal/auth"` в блок
  импортов, и добавить новую функцию (сразу после существующих `postD`/`putD`/
  `delD`, перед `requireStatus`):
  ```go
  // ownerCtx кладёт в контекст запроса Access=Full и OwnerID — как resolveAccess
  // при валидной cookie-сессии. Существующие тесты записи проверяют контракт
  // хендлера для аутентифицированного владельца; анонимный путь — отдельные
  // тесты TestDivision*AnonymousIs401 ниже.
  func ownerCtx(r *http.Request) *http.Request {
  	ctx := context.WithValue(r.Context(), accessCtxKey, models.AccessFull)
  	ctx = context.WithValue(ctx, ownerCtxKey, authpkg.ID("OW-01ARZ3NDEKTSV4RRFFQ69G5FA9"))

  	return r.WithContext(ctx)
  }
  ```
  и заменить тела `postD`/`putD`/`delD` так, чтобы запрос оборачивался
  `ownerCtx`:
  ```go
  func postD(t *testing.T, h http.Handler, path, body string) *httptest.ResponseRecorder {
  	t.Helper()

  	rec := httptest.NewRecorder()
  	h.ServeHTTP(rec, ownerCtx(httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))))

  	return rec
  }

  func putD(t *testing.T, h http.Handler, path, body string) *httptest.ResponseRecorder {
  	t.Helper()

  	rec := httptest.NewRecorder()
  	h.ServeHTTP(rec, ownerCtx(httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))))

  	return rec
  }

  func delD(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
  	t.Helper()

  	rec := httptest.NewRecorder()
  	h.ServeHTTP(rec, ownerCtx(httptest.NewRequest(http.MethodDelete, path, nil)))

  	return rec
  }
  ```
  Существующие 11 тестов, которые используют `postD`/`putD`/`delD`
  (`TestDivisionCreateContract`, `TestDivisionCreateWithParentPassesModel`,
  `TestDivisionCreateBadJSONIs400`, `TestDivisionCreateValidationErrorIs422`,
  `TestDivisionUpdateMergesFields`, `TestDivisionUpdateNotFoundIs404`,
  `TestDivisionUpdateInvalidIDIs422`, `TestDivisionDeleteNoContent`,
  `TestDivisionDeleteNotFoundIs404`, `TestDivisionDeleteInvalidIDIs422`,
  `TestDivisionDeleteInUseIs409WithReferrers`) не трогать — они автоматически
  становятся «от лица владельца» через изменённые хелперы.

- [ ] Шаг 2.7. В том же файле — добавить три новых теста в конец файла:
  ```go
  // TestDivisionCreateAnonymousIs401: без активной сессии запись отклоняется
  // раньше разбора тела — сценарий не вызывается (auth.md §6, приёмка этапа C).
  func TestDivisionCreateAnonymousIs401(t *testing.T) {
  	svc := &fakeDivisions{}

  	rec := httptest.NewRecorder()
  	req := httptest.NewRequest(http.MethodPost, "/api/admin-divisions", strings.NewReader(`{"name":"x","type":"selo"}`))
  	NewHandler(svc, fstest.MapFS{}).ServeHTTP(rec, req)

  	requireStatus(t, rec, http.StatusUnauthorized)
  	if svc.gotCreate.ID != "" {
  		t.Fatalf("сценарий вызван анонимом: %+v", svc.gotCreate)
  	}
  }

  // TestDivisionUpdateAnonymousIs401: аналогично для PUT.
  func TestDivisionUpdateAnonymousIs401(t *testing.T) {
  	svc := &fakeDivisions{}

  	rec := httptest.NewRecorder()
  	req := httptest.NewRequest(http.MethodPut, "/api/admin-divisions/"+writeID, strings.NewReader(`{}`))
  	NewHandler(svc, fstest.MapFS{}).ServeHTTP(rec, req)

  	requireStatus(t, rec, http.StatusUnauthorized)
  	if len(svc.gotIDs) != 0 {
  		t.Fatalf("сценарий вызван анонимом: %v", svc.gotIDs)
  	}
  }

  // TestDivisionDeleteAnonymousIs401: аналогично для DELETE.
  func TestDivisionDeleteAnonymousIs401(t *testing.T) {
  	svc := &fakeDivisions{}

  	rec := httptest.NewRecorder()
  	req := httptest.NewRequest(http.MethodDelete, "/api/admin-divisions/"+writeID, nil)
  	NewHandler(svc, fstest.MapFS{}).ServeHTTP(rec, req)

  	requireStatus(t, rec, http.StatusUnauthorized)
  	if len(svc.gotIDs) != 0 {
  		t.Fatalf("сценарий вызван анонимом: %v", svc.gotIDs)
  	}
  }
  ```
  (используют уже существующий в этом файле хелпер `requireStatus` и константу
  `writeID`.)

- [ ] Шаг 2.8. `gofmt -w internal/httpapi/`. На этом шаге `go build ./internal/httpapi/...`
  и `go test ./internal/httpapi/...` ещё НЕ зелёные — пакет `httpapi_test`
  (`store_test.go`/`write_store_test.go`) использует старую (без `access`)
  сигнатуру `ListDivisions`/`SearchDivisions` в своей локальной
  `divisionService` и не соберётся, пока их не поправят следующие шаги этой
  же задачи (3.5–3.6 ниже); Go собирает внутренний (`package httpapi`) и
  внешний (`package httpapi_test`) тестовые бинарники одной директории
  совместно, так что до шага 3.6 зелёного `go test` в этом пакете не будет —
  это ожидаемо, не откатывать и не чинить `store_test.go`/`write_store_test.go`
  раньше времени. Проверить на этом шаге можно точечно: сами файлы
  `deps.go`/`division.go`/`division_write.go` синтаксически корректны (`gofmt
  -l` их не показывает), а новые/изменённые тесты в `division_test.go`/
  `division_write_test.go` — визуальным ревью на соответствие описанному
  выше коду (оба файла package `httpapi`, той же директории, что и
  `store_test.go`, поэтому тоже не соберутся в изоляции до шага 3.6 — но их
  логика уже полная и правильная, продолжать можно).

- [ ] Шаг 2.9. Коммит:
  `git add internal/httpapi/deps.go internal/httpapi/division.go internal/httpapi/division_write.go internal/httpapi/division_test.go internal/httpapi/division_write_test.go`
  `feat(httpapi): Access в чтении делений, requireFull на запись`.

### Продолжение задачи 2: единая точка входа `/api` и e2e с реальной сессией

**Файлы:**
- Изменить: `internal/store/sqlstore/sqlstore.go` (один метод)
- Изменить: `internal/httpapi/httpapi.go`, `internal/httpapi/auth.go`
- Создать: `internal/httpapi/api.go`
- Изменить: `internal/httpapi/store_test.go`, `internal/httpapi/write_store_test.go`

- [ ] Шаг 3.1. `internal/store/sqlstore/sqlstore.go` — добавить метод сразу
  после `Close`:
  ```go
  // DB возвращает соединение хранилища — нужно, чтобы построить
  // internal/auth.SQLStore на том же соединении, не открывая файл БД дважды
  // (auth.md §6, этап C).
  func (s *Store) DB() *storage.DB {
  	return s.db
  }
  ```

- [ ] Шаг 3.2. `internal/httpapi/httpapi.go` — заменить `NewHandler` и
  добавить `registerDivisionRoutes`:
  ```go
  // NewHandler возвращает http.Handler с маршрутами /api (без auth-
  // оборачивания) — используется юнит-тестами этого пакета напрямую.
  // Реальное приложение монтирует NewAPIHandler (api.go).
  func NewHandler(divisions DivisionService, docsFS fs.FS) http.Handler {
  	mux := http.NewServeMux()
  	registerDivisionRoutes(mux, divisions, docsFS)

  	return mux
  }

  // registerDivisionRoutes регистрирует маршруты /api/admin-divisions,
  // /api/docs, /api/health на переданном mux — общий код NewHandler и
  // NewAPIHandler.
  func registerDivisionRoutes(mux *http.ServeMux, divisions DivisionService, docsFS fs.FS) {
  	mux.HandleFunc("GET /api/health", handleHealth)
  	mux.HandleFunc("GET /api/admin-divisions", handleDivisionList(divisions))
  	mux.HandleFunc("GET /api/admin-divisions/search", handleDivisionSearch(divisions))
  	mux.HandleFunc("GET /api/admin-divisions/{id}", handleDivisionGet(divisions))
  	mux.HandleFunc("POST /api/admin-divisions", handleDivisionCreate(divisions))
  	mux.HandleFunc("PUT /api/admin-divisions/{id}", handleDivisionUpdate(divisions))
  	mux.HandleFunc("DELETE /api/admin-divisions/{id}", handleDivisionDelete(divisions))
  	mux.HandleFunc("GET /api/docs", handleDocList(docsFS))
  	mux.HandleFunc("GET /api/docs/{path}", handleDocContent(docsFS))
  }
  ```
  Остальную часть файла (`handleHealth`, `writeJSON`, `writeError`) не менять.

- [ ] Шаг 3.3. `internal/httpapi/auth.go` — заменить `NewAuthHandler` и
  добавить `registerAuthRoutes`:
  ```go
  // NewAuthHandler строит обработчики /api/auth/*, обёрнутые resolveAccess и
  // requireCSRFHeader — используется юнит-тестами этого пакета напрямую.
  // Реальное приложение монтирует весь /api/ через NewAPIHandler (api.go),
  // который вызывает registerAuthRoutes без повторного оборачивания.
  func NewAuthHandler(auth AuthService) http.Handler {
  	mux := http.NewServeMux()
  	registerAuthRoutes(mux, auth)

  	return requireCSRFHeader(resolveAccess(auth)(mux))
  }

  // registerAuthRoutes регистрирует маршруты /api/auth/* на переданном mux.
  func registerAuthRoutes(mux *http.ServeMux, auth AuthService) {
  	mux.HandleFunc("GET /api/auth/status", handleAuthStatus(auth))
  	mux.HandleFunc("GET /api/auth/session", handleAuthSession(auth))
  	mux.HandleFunc("POST /api/auth/register", handleAuthRegister(auth))
  	mux.HandleFunc("POST /api/auth/login", handleAuthLogin(auth))
  	mux.HandleFunc("POST /api/auth/logout", handleAuthLogout(auth))
  	mux.HandleFunc("POST /api/auth/refresh", handleAuthRefresh(auth))
  	mux.HandleFunc("POST /api/auth/password", handleAuthPassword(auth))
  	mux.HandleFunc("POST /api/auth/invites", handleAuthCreateInvite(auth))
  	mux.HandleFunc("POST /api/auth/tokens", handleAuthCreateToken(auth))
  	mux.HandleFunc("GET /api/auth/tokens", handleAuthListTokens(auth))
  	mux.HandleFunc("DELETE /api/auth/tokens/{id}", handleAuthRevokeToken(auth))
  }
  ```
  Остальную часть файла (`setSessionCookies`, `clearSessionCookies`,
  `requireFull`, `writeAuthError`, все `handleAuth*`) не менять.

- [ ] Шаг 3.4. Создать `internal/httpapi/api.go`:
  ```go
  package httpapi

  import (
  	"io/fs"
  	"net/http"
  )

  // NewAPIHandler — единая точка входа /api: маршруты делений, документации и
  // auth на одном mux, обёрнутые ОДИН раз resolveAccess + requireCSRFHeader
  // (auth.md §4 — исходный замысел дизайна: оба миддлвари вокруг всего /api/,
  // не только /api/auth/*). Это то, что реально монтирует internal/app
  // (этап C). NewHandler и NewAuthHandler остаются отдельно для существующих
  // юнит-тестов пакета, не зависящих от auth.
  func NewAPIHandler(divisions DivisionService, auth AuthService, docsFS fs.FS) http.Handler {
  	mux := http.NewServeMux()
  	registerDivisionRoutes(mux, divisions, docsFS)
  	registerAuthRoutes(mux, auth)

  	return requireCSRFHeader(resolveAccess(auth)(mux))
  }
  ```

- [ ] Шаг 3.5. `internal/httpapi/store_test.go` — заменить методы локальной
  `divisionService`:
  ```go
  func (s *divisionService) ListDivisions(ctx context.Context, access models.Access, q models.DivisionQuery) ([]models.AdministrativeDivision, error) {
  	return s.list.ListDivisions(ctx, access, q)
  }

  func (s *divisionService) SearchDivisions(ctx context.Context, access models.Access, q models.DivisionSearchQuery) ([]models.AdministrativeDivision, error) {
  	return s.search.SearchDivisions(ctx, access, q)
  }
  ```
  Остальные методы `divisionService`, `newDivisionService` и
  `TestAdminDivisionsWithRealStore` (только чтение — GET, поведение не
  меняется) не трогать.

- [ ] Шаг 3.6. `internal/httpapi/write_store_test.go` — заменить файл целиком:
  ```go
  package httpapi_test

  import (
  	"encoding/json"
  	"fmt"
  	"net/http"
  	"net/http/httptest"
  	"strings"
  	"testing"
  	"testing/fstest"

  	"github.com/amarin/genodex/internal/auth"
  	"github.com/amarin/genodex/internal/httpapi"
  	"github.com/amarin/genodex/internal/models"
  	"github.com/amarin/genodex/internal/store/sqlstore"
  	"github.com/amarin/genodex/internal/transport"
  )

  // TestDivisionWriteContractWithRealStore: сквозной путь «хранилище → сценарии
  // → HTTP» (так же собран internal/app) для записи делений — через
  // NewAPIHandler с реальной сессией владельца: регистрация (bootstrap) →
  // создание/чтение/изменение/удаление → 409 со списком ссылающихся →
  // анонимная попытка записи — 401 (auth.md §6, приёмка этапа C).
  func TestDivisionWriteContractWithRealStore(t *testing.T) {
  	st, err := sqlstore.Open(t.TempDir())
  	if err != nil {
  		t.Fatalf("open store: %v", err)
  	}
  	t.Cleanup(func() { _ = st.Close() })

  	authSvc := auth.New(auth.NewSQLStore(st.DB()))
  	h := httpapi.NewAPIHandler(newDivisionService(t, st), authSvc, fstest.MapFS{})

  	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
  	requireStatusS(t, regRec, http.StatusCreated)
  	accessCookie, _ := sessionCookies(t, regRec)
  	owner := []*http.Cookie{accessCookie}

  	// Создание корня.
  	root := createDivision(t, h, owner, `{"name":"Московская","type":"governorate"}`, http.StatusCreated)
  	if root.Name != "Московская" || root.Type != models.AdminDivisionGovernorate || root.ParentID != nil {
  		t.Fatalf("root = %+v", root)
  	}
  	if !strings.HasPrefix(string(root.ID), "AD-") {
  		t.Fatalf("id = %q", root.ID)
  	}

  	// Создание дочерней.
  	child := createDivision(t, h, owner,
  		fmt.Sprintf(`{"name":"Давыдово","type":"selo","parent_id":%q}`, root.ID), http.StatusCreated)
  	if child.ParentID == nil || *child.ParentID != root.ID {
  		t.Fatalf("child = %+v", child)
  	}

  	// Чтение по id — открыто анонимному посетителю.
  	rec := getReq(t, h, "/api/admin-divisions/"+string(child.ID))
  	requireStatusS(t, rec, http.StatusOK)
  	if !strings.Contains(rec.Body.String(), `"name":"Давыдово"`) {
  		t.Fatalf("body = %s", rec.Body)
  	}

  	// Изменение: полная замена полей name/type/parent_id.
  	rec = putReq(t, h, owner, "/api/admin-divisions/"+string(child.ID),
  		fmt.Sprintf(`{"name":"Давыдова","type":"selo","parent_id":%q}`, root.ID))
  	requireStatusS(t, rec, http.StatusOK)
  	updated := decodeDivisionS(t, rec)
  	if updated.Name != "Давыдова" || updated.ID != child.ID {
  		t.Fatalf("after update = %+v", updated)
  	}

  	// Родитель занят дочерью → 409 со списком ссылающихся.
  	rec = delReq(t, h, owner, "/api/admin-divisions/"+string(root.ID))
  	requireStatusS(t, rec, http.StatusConflict)
  	if !strings.Contains(rec.Body.String(), `"referrers"`) || !strings.Contains(rec.Body.String(), string(child.ID)) {
  		t.Fatalf("body = %s", rec.Body)
  	}

  	// Удаление дочерней, затем корня.
  	delReq(t, h, owner, "/api/admin-divisions/"+string(child.ID))
  	requireStatusS(t, delReq(t, h, owner, "/api/admin-divisions/"+string(root.ID)), http.StatusNoContent)

  	// Несуществующая — 404.
  	requireStatusS(t, getReq(t, h, "/api/admin-divisions/"+string(root.ID)), http.StatusNotFound)

  	// Неверный формат id в пути — 422 (не 404).
  	requireStatusS(t, getReq(t, h, "/api/admin-divisions/obvious-bad"), http.StatusUnprocessableEntity)

  	// Несуществующий родитель (id валидного формата) — 422 с полем parent_id.
  	rec = postReq(t, h, owner, `{"name":"Давыдово","type":"selo","parent_id":"AD-01ARZ3NDEKTSV4RRFFQ69G5FA9"}`)
  	requireStatusS(t, rec, http.StatusUnprocessableEntity)
  	if !strings.Contains(rec.Body.String(), `"field":"parent_id"`) {
  		t.Fatalf("body = %s", rec.Body)
  	}

  	// Анонимная попытка создать — 401, до разбора тела.
  	anonRec := postReq(t, h, nil, `{"name":"Аноним","type":"selo"}`)
  	requireStatusS(t, anonRec, http.StatusUnauthorized)
  }

  func createDivision(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.AdminDivision {
  	t.Helper()

  	rec := postReq(t, h, cookies, body)
  	if rec.Code != want {
  		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
  	}

  	return decodeDivisionS(t, rec)
  }

  func decodeDivisionS(t *testing.T, rec *httptest.ResponseRecorder) transport.AdminDivision {
  	t.Helper()

  	var d transport.AdminDivision
  	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
  		t.Fatalf("decode: %v; body = %s", err, rec.Body)
  	}

  	return d
  }

  func postReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
  	t.Helper()

  	req := httptest.NewRequest(http.MethodPost, "/api/admin-divisions", strings.NewReader(body))
  	req.Header.Set("X-Requested-With", "genodex")
  	for _, c := range cookies {
  		req.AddCookie(c)
  	}

  	rec := httptest.NewRecorder()
  	h.ServeHTTP(rec, req)

  	return rec
  }

  func getReq(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
  	t.Helper()

  	rec := httptest.NewRecorder()
  	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

  	return rec
  }

  func putReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
  	t.Helper()

  	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
  	req.Header.Set("X-Requested-With", "genodex")
  	for _, c := range cookies {
  		req.AddCookie(c)
  	}

  	rec := httptest.NewRecorder()
  	h.ServeHTTP(rec, req)

  	return rec
  }

  func delReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path string) *httptest.ResponseRecorder {
  	t.Helper()

  	req := httptest.NewRequest(http.MethodDelete, path, nil)
  	req.Header.Set("X-Requested-With", "genodex")
  	for _, c := range cookies {
  		req.AddCookie(c)
  	}

  	rec := httptest.NewRecorder()
  	h.ServeHTTP(rec, req)

  	return rec
  }

  func requireStatusS(t *testing.T, rec *httptest.ResponseRecorder, want int) {
  	t.Helper()

  	if rec.Code != want {
  		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
  	}
  }
  ```
  (`postAuthReq`/`sessionCookies` — переиспользуются из `auth_service_test.go`,
  того же пакета `httpapi_test`; не объявлять заново.)

- [ ] Шаг 3.7. `gofmt -w internal/httpapi/ internal/store/sqlstore/`.
  `go build ./...` — провалится в `internal/mcp`/`internal/app` (их черёд —
  задачи 4, 5); `go build ./internal/httpapi/... ./internal/store/sqlstore/...`
  — зелено. `go test ./internal/httpapi/... ./internal/store/sqlstore/...` —
  зелено.

- [ ] Шаг 3.8. Коммит:
  `git add internal/store/sqlstore/sqlstore.go internal/httpapi/httpapi.go internal/httpapi/auth.go internal/httpapi/api.go internal/httpapi/store_test.go internal/httpapi/write_store_test.go`
  `feat(httpapi): NewAPIHandler — единая точка входа /api, оборачивает resolveAccess+requireCSRFHeader один раз`.

## Задача 3. mcp: Access из контекста

**Файлы:**
- Изменить: `internal/mcp/deps.go`, `internal/mcp/division.go`,
  `internal/mcp/division_test.go`

- [ ] Шаг 3.1. `internal/mcp/deps.go` — заменить сигнатуры в интерфейсе:
  ```go
  // DivisionService — контракт сценариев административного деления, отдаваемых в
  // MCP-тулы: список, чтение, создание, изменение, удаление.
  type DivisionService interface {
  	ListDivisions(ctx context.Context, access models.Access, q models.DivisionQuery) ([]models.AdministrativeDivision, error)
  	SearchDivisions(ctx context.Context, access models.Access, q models.DivisionSearchQuery) ([]models.AdministrativeDivision, error)
  	GetDivision(ctx context.Context, id models.ID) (models.AdministrativeDivision, error)
  	CreateDivision(ctx context.Context, d models.AdministrativeDivision) (models.AdministrativeDivision, error)
  	UpdateDivision(ctx context.Context, d models.AdministrativeDivision) error
  	DeleteDivision(ctx context.Context, id models.ID) error
  }
  ```

- [ ] Шаг 3.2. `internal/mcp/division.go` — в `divisionListHandler` заменить
  ```go
  		list, err := divisions.ListDivisions(ctx, q)
  ```
  на
  ```go
  		list, err := divisions.ListDivisions(ctx, AccessFromContext(ctx), q)
  ```
  и в `divisionSearchHandler` заменить
  ```go
  		list, err := divisions.SearchDivisions(ctx, q)
  ```
  на
  ```go
  		list, err := divisions.SearchDivisions(ctx, AccessFromContext(ctx), q)
  ```
  (`AccessFromContext` уже объявлена в `internal/mcp/middleware.go`, того же
  пакета; MCP-письмо `divisionCreateHandler`/`divisionUpdateHandler`/
  `divisionDeleteHandler` НЕ трогать — гейт уже на уровне всего `/mcp`, этап
  B2+задача 4 этого плана.)

- [ ] Шаг 3.3. `internal/mcp/division_test.go` — в `fakeDivisions` добавить
  поля:
  ```go
  	gotListAccess   models.Access
  	gotSearchAccess models.Access
  ```
  и заменить методы:
  ```go
  func (f *fakeDivisions) ListDivisions(_ context.Context, access models.Access, q models.DivisionQuery) ([]models.AdministrativeDivision, error) {
  	f.got = q
  	f.gotListAccess = access
  	f.call++

  	return f.list, f.err
  }

  func (f *fakeDivisions) SearchDivisions(_ context.Context, access models.Access, q models.DivisionSearchQuery) ([]models.AdministrativeDivision, error) {
  	f.gotSearch = q
  	f.gotSearchAccess = access
  	f.call++

  	return f.search, f.searchErr
  }
  ```

- [ ] Шаг 3.4. В том же файле — добавить два новых теста (после
  `TestNewServerRegistersDivisionListOnly`, последнего в файле):
  ```go
  // TestDivisionListToolPassesAccessFromContext: Access, положенный
  // RequireAPIToken в контекст запроса, доходит до сценария как есть (не
  // захардкожен на AccessFull).
  func TestDivisionListToolPassesAccessFromContext(t *testing.T) {
  	svc := &fakeDivisions{}

  	ctx := context.WithValue(context.Background(), accessCtxKey, models.AccessPublic)
  	req := mcp.CallToolRequest{}

  	if _, err := divisionListHandler(svc)(ctx, req); err != nil {
  		t.Fatal(err)
  	}

  	if svc.gotListAccess != models.AccessPublic {
  		t.Fatalf("gotListAccess = %v, ожидался AccessPublic", svc.gotListAccess)
  	}
  }

  // TestDivisionSearchToolPassesAccessFromContext: аналогично для поиска.
  func TestDivisionSearchToolPassesAccessFromContext(t *testing.T) {
  	svc := &fakeDivisions{}

  	ctx := context.WithValue(context.Background(), accessCtxKey, models.AccessPublic)
  	req := mcp.CallToolRequest{}
  	req.Params.Arguments = map[string]any{"q": "давы"}

  	if _, err := divisionSearchHandler(svc)(ctx, req); err != nil {
  		t.Fatal(err)
  	}

  	if svc.gotSearchAccess != models.AccessPublic {
  		t.Fatalf("gotSearchAccess = %v, ожидался AccessPublic", svc.gotSearchAccess)
  	}
  }
  ```
  (файл — `package mcp`, `accessCtxKey` — неэкспортируемая константа
  `internal/mcp/middleware.go` того же пакета; `context`/`mcp`/`models` уже
  импортированы в этом файле — `mcp` здесь означает пакет
  `github.com/mark3labs/mcp-go/mcp`, а не текущий пакет `internal/mcp`, как и
  в остальном файле.)

- [ ] Шаг 3.5. `gofmt -w internal/mcp/`. `go build ./internal/mcp/...` —
  зелено (в `division_write_test.go`/`middleware*_test.go` изменений нет,
  сигнатуры `CreateDivision`/`UpdateDivision`/`DeleteDivision` не менялись).
  `go test ./internal/mcp/...` — зелено.

- [ ] Шаг 3.6. Коммит:
  `git add internal/mcp/deps.go internal/mcp/division.go internal/mcp/division_test.go`
  `feat(mcp): Access из контекста в ListDivisions/SearchDivisions`.

## Задача 4. internal/app: подключение auth, доки

**Файлы:**
- Изменить: `internal/app/app.go`
- Создать: `internal/app/app_test.go`
- Изменить: `docs/data-model/auth.md`, `docs/plans/2026-09-22-auth-roadmap.md`

- [ ] Шаг 4.1. `internal/app/app.go` — заменить файл целиком:
  ```go
  package app

  import (
  	"context"
  	"fmt"
  	"log"
  	"net/http"
  	"time"

  	"github.com/mark3labs/mcp-go/server"

  	"github.com/amarin/genodex"
  	"github.com/amarin/genodex/internal/auth"
  	"github.com/amarin/genodex/internal/httpapi"
  	"github.com/amarin/genodex/internal/idgen"
  	"github.com/amarin/genodex/internal/mcp"
  	"github.com/amarin/genodex/internal/models"
  	"github.com/amarin/genodex/internal/store/sqlstore"
  	create_division "github.com/amarin/genodex/internal/usecases/create_division"
  	delete_division "github.com/amarin/genodex/internal/usecases/delete_division"
  	get_division "github.com/amarin/genodex/internal/usecases/get_division"
  	list_divisions "github.com/amarin/genodex/internal/usecases/list_divisions"
  	search_divisions "github.com/amarin/genodex/internal/usecases/search_divisions"
  	update_division "github.com/amarin/genodex/internal/usecases/update_division"
  	"github.com/amarin/genodex/web"
  )

  // Config — параметры запуска приложения.
  type Config struct {
  	DataDir string
  	Port    int
  	WebMode string
  }

  // App — корневой объект приложения: собирает хранилище, usecases и интерфейсы.
  type App struct {
  	cfg   Config
  	http  *http.Server
  	store *sqlstore.Store
  }

  // divisionService — фасад всех сценариев делений, отдаваемых HTTP и MCP.
  type divisionService struct {
  	list   *list_divisions.Scenario
  	search *search_divisions.Scenario
  	get    *get_division.Scenario
  	create *create_division.Scenario
  	update *update_division.Scenario
  	del    *delete_division.Scenario
  }

  func (s *divisionService) ListDivisions(ctx context.Context, access models.Access, q models.DivisionQuery) ([]models.AdministrativeDivision, error) {
  	return s.list.ListDivisions(ctx, access, q)
  }

  func (s *divisionService) SearchDivisions(ctx context.Context, access models.Access, q models.DivisionSearchQuery) ([]models.AdministrativeDivision, error) {
  	return s.search.SearchDivisions(ctx, access, q)
  }

  func (s *divisionService) GetDivision(ctx context.Context, id models.ID) (models.AdministrativeDivision, error) {
  	return s.get.GetDivision(ctx, id)
  }

  func (s *divisionService) CreateDivision(ctx context.Context, d models.AdministrativeDivision) (models.AdministrativeDivision, error) {
  	return s.create.CreateDivision(ctx, d)
  }

  func (s *divisionService) UpdateDivision(ctx context.Context, d models.AdministrativeDivision) error {
  	return s.update.UpdateDivision(ctx, d)
  }

  func (s *divisionService) DeleteDivision(ctx context.Context, id models.ID) error {
  	return s.del.DeleteDivision(ctx, id)
  }

  var (
  	_ httpapi.DivisionService = (*divisionService)(nil)
  	_ mcp.DivisionService     = (*divisionService)(nil)
  	_ httpapi.AuthService     = (*auth.Service)(nil)
  	_ mcp.TokenResolver       = (*auth.Service)(nil)
  )

  // New собирает приложение: хранилище → сценарии/auth → MCP/HTTP интерфейсы.
  func New(cfg Config) (*App, error) {
  	st, err := sqlstore.Open(cfg.DataDir)
  	if err != nil {
  		return nil, fmt.Errorf("open store: %w", err)
  	}

  	divisions := &divisionService{
  		list:   list_divisions.New(st),
  		search: search_divisions.New(st),
  		get:    get_division.New(st),
  		create: create_division.New(st, idgen.New()),
  		update: update_division.New(st),
  		del:    delete_division.New(st),
  	}

  	// auth-хранилище — на том же соединении, что и общий store (см.
  	// sqlstore.Store.DB), файл БД один и тот же (internal/storage/schema_auth.go).
  	authService := auth.New(auth.NewSQLStore(st.DB()))

  	mux := http.NewServeMux()
  	mux.Handle("/mcp", mcp.RequireAPIToken(authService)(server.NewStreamableHTTPServer(mcp.NewServer(divisions))))
  	mux.Handle("/api/", httpapi.NewAPIHandler(divisions, authService, genodex.DocsFS(cfg.WebMode)))
  	mux.Handle("/static/", http.StripPrefix("/static/", web.StaticHandler(cfg.WebMode)))
  	mux.Handle("/", web.SPAHandler(cfg.WebMode))

  	return &App{
  		cfg: cfg,
  		http: &http.Server{
  			Addr:    fmt.Sprintf("0.0.0.0:%d", cfg.Port),
  			Handler: mux,
  		},
  		store: st,
  	}, nil
  }

  // Run запускает HTTP-сервер. Останавливается по отмене ctx (graceful shutdown
  // HTTP и закрытие хранилища) либо по ошибке ListenAndServe.
  func (a *App) Run(ctx context.Context) error {
  	log.Printf("Genealogy MCP server started on %s", a.http.Addr)
  	log.Printf("  MCP:      http://localhost:%d/mcp", a.cfg.Port)
  	log.Printf("  API:      http://localhost:%d/api", a.cfg.Port)
  	log.Printf("  Web:      http://localhost:%d/ (web mode: %s)", a.cfg.Port, a.cfg.WebMode)

  	go func() {
  		<-ctx.Done()
  		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
  		defer cancel()
  		if err := a.http.Shutdown(shCtx); err != nil {
  			log.Printf("Graceful shutdown error: %v", err)
  		}
  		if err := a.store.Close(); err != nil {
  			log.Printf("Store close error: %v", err)
  		}
  	}()

  	if err := a.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
  		return err
  	}
  	return nil
  }
  ```

- [ ] Шаг 4.2. Создать `internal/app/app_test.go`:
  ```go
  package app

  import (
  	"net/http"
  	"net/http/httptest"
  	"testing"

  	"github.com/amarin/genodex/web"
  )

  // TestAppRejectsUnauthenticatedWrites: сквозная проверка сборки New —
  // реальная сборка действительно подключает auth к /api и /mcp (этап C).
  // Полное поведение самого auth — в internal/httpapi и internal/mcp; здесь —
  // только доказательство того, что New их реально соединяет.
  func TestAppRejectsUnauthenticatedWrites(t *testing.T) {
  	a, err := New(Config{DataDir: t.TempDir(), Port: 0, WebMode: web.ModeProd})
  	if err != nil {
  		t.Fatalf("New: %v", err)
  	}
  	t.Cleanup(func() { _ = a.store.Close() })

  	rec := httptest.NewRecorder()
  	req := httptest.NewRequest(http.MethodPost, "/api/admin-divisions", nil)
  	req.Header.Set("X-Requested-With", "genodex")
  	a.http.Handler.ServeHTTP(rec, req)
  	if rec.Code != http.StatusUnauthorized {
  		t.Errorf("POST /api/admin-divisions без сессии = %d, ожидался 401", rec.Code)
  	}

  	rec = httptest.NewRecorder()
  	a.http.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", nil))
  	if rec.Code != http.StatusUnauthorized {
  		t.Errorf("POST /mcp без токена = %d, ожидался 401", rec.Code)
  	}
  }
  ```

- [ ] Шаг 4.3. `docs/data-model/auth.md` §6 — дополнить пунктом о фактическом
  подключении (после существующего списка):
  - `internal/app.New` строит `auth.Service` на том же соединении, что и
    общий `sqlstore.Store` (`Store.DB()`, этап C) — не открывает файл БД
    дважды.
  - `/mcp` монтируется через `mcp.RequireAPIToken(authService)(...)`; весь
    `/api/` — через `httpapi.NewAPIHandler(divisions, authService, docsFS)`
    (единая точка входа, `resolveAccess`+`requireCSRFHeader` вокруг всего
    `/api/`, не только `/api/auth/*` — закрывает гэп, отмеченный после этапа
    B).
  §4 — убрать формулировку «подключается в общий mux на этапе C» и
  «NewAuthHandler... подключается в общий mux на этапе C» (обе встречаются в
  доккомментах `auth.go`/§4 текста) — заменить на «подключён в
  `internal/app.New` через `httpapi.NewAPIHandler`, см. §6».

- [ ] Шаг 4.4. `docs/plans/2026-09-22-auth-roadmap.md` — раздел `## C.` —
  добавить строку `**Статус:** выполнено (...)` тем же стилем, что у
  A2/B/B2, со ссылкой на `2026-09-22-auth-c-access.md` и кратким
  перечислением (Access в List/Search, requireFull на запись,
  `NewAPIHandler`, реальное подключение в `internal/app`).

- [ ] Шаг 4.5. `gofmt -l .` — пусто. `go build ./...` — зелено. `go vet ./...`
  — зелено. `go test ./...` — зелено (весь репозиторий, включая
  `internal/app`).

- [ ] Шаг 4.6. Коммит:
  `git add internal/app/app.go internal/app/app_test.go docs/data-model/auth.md docs/plans/2026-09-22-auth-roadmap.md`
  `feat(app): подключить auth.Service — /api и /mcp реально защищены`.

## Рубеж этапа

- `internal/usecases/list_divisions`/`search_divisions`: `access` — параметр
  сценария, не хардкод.
- `internal/httpapi`/`internal/mcp`: `DivisionService.ListDivisions/
  SearchDivisions` берут `access` из контекста своего транспорта.
- `POST`/`PUT`/`DELETE /api/admin-divisions...` — `401` анонимному
  посетителю; MCP-письмо недостижимо анонимно (весь `/mcp` за bearer-токеном).
- `internal/app.New` реально собирает `auth.Service` и подключает оба
  миддлвари; `go test ./...` зелёный на всём репозитории.

## Коммиты

1. `feat(usecases): access параметром в ListDivisions/SearchDivisions вместо AccessFull внутри`
2. `feat(httpapi): Access в чтении делений, requireFull на запись`
3. `feat(httpapi): NewAPIHandler — единая точка входа /api, оборачивает resolveAccess+requireCSRFHeader один раз`
4. `feat(mcp): Access из контекста в ListDivisions/SearchDivisions`
5. `feat(app): подключить auth.Service — /api и /mcp реально защищены`
