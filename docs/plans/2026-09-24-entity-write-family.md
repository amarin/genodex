# Веб-CRUD/MCP для всех сущностей — подпроект 7 (Family): план

> **Для агента-исполнителя:** ОБЯЗАТЕЛЬНЫЙ ПОДНАВЫК: superpowers:subagent-driven-development.

Седьмой проход по декомпозиции `docs/data-model/entity-write.md` §2. Вводит `Family` (род/линия) — плоскую сущность без строгого FK, без self-ref, без полиморфного типа: структурно почти идентична `Repository` (подпроект 3) минус `Type`/`Address`, с `URLs`, переименованным в `Members` (мягкая ссылка на `Person`). Самая простая сущность программы на сегодня — ни одного нового паттерна.

## Goal

Полный CRUD (HTTP + MCP + веб) для `Family` — по конвенциям `docs/data-model/entity-write.md` §3-4. Никаких новых переиспользуемых паттернов или веб-компонентов — только применение уже закреплённых конвенций (access-aware `Get`, `InTx`+проверка `sources[i].citation_id`, fetch-then-merge на `update`, `Sources` редактируемый с рождения, `TextRefListEditor` для мягкого списка `Members`, `SourceLinkListEditor` для `Sources`).

## Архитектура

`Family` — `{ID, Name, Members []TextRef, Notes []TextRef, Sources []SourceLink, Private bool}`. Единственная связь с другой сущностью — `Sources[i].CitationID`, строгий FK, проверяемый в транзакции по образцу `create_repository`/`update_repository`. `Members` — список `TextRef`, мягкая ссылка на `Person` (`TypePerson`, у которого ещё нет CRUD-слоя — появится в подпроекте 8): как и у любого другого `TextRef` в программе, `Ref`, если задан, проверяется только по формату, существование цели не проверяется. `Family` заведена уже после того, как `Citation` получил CRUD (подпроект 5), поэтому `Sources` редактируется с первого дня контракта (не ретрофит) — как у `ArchiveNode`/`ArchiveDocument` в подпроекте 6. На веб-стороне — обычная страница-тройка (List/View+Form) без нового переиспользуемого компонента: `Members`/`Notes` редактируются существующим `TextRefListEditor`, `Sources` — существующим `SourceLinkListEditor`.

## Технологии

Backend: Go, `internal/models` (домен, `Family`/`FamilyValidate` — уже существуют, не меняются) → `internal/usecases/<scenario>` → `internal/transport` (DTO) → `internal/httpapi` (JSON REST) / `internal/mcp` (`github.com/mark3labs/mcp-go`, Streamable HTTP) → `internal/store`/`internal/store/sqlstore` (generic-хранилище, уже готово для всех 21 типа, в этом проходе не меняется). Frontend: Vite + React 18 + TypeScript + antd, файлы-страницы по образцу уже существующих (`RepositoriesList`/`RepositoryForm`/`RepositoryView`), копирование, не общий фреймворк. Тесты: стандартный `go test` (usecases — фейковый стор в памяти; `internal/httpapi/write_store_test.go` — реальный SQLite), `npm run typecheck`/`npm run build` на вебе.

## Спецификация

Источник истины дизайна — `docs/data-model/entity-write.md` (копия из проверочного worktree, см. «Предпосылка» — копия на `main` этого раздела не содержит): §3.6 «Подпроект 7 (`Family`) — без новых паттернов» — подтверждает, что весь стек скопирован с `Repository`/`ArchiveNode` без единого нового решения дизайна; §4 «Веб-UI конвенции» — общие правила List/View/Create/Delete, которым без отступлений следуют новые страницы `Family` (плоский список, без иерархии — то же самое правило, что у `Repository`).

## Глобальные ограничения

Правила программы `entity-write`, обязательные для каждой новой сущности (дословно из `entity-write.md` и правил, закреплённых финальными ревью подпроектов 1-6); подпроект 7 не вводит ни одного нового правила — следует всем существующим:

- **Access-aware `Get` для любой сущности с `Private`.** `get_family` обязан принимать `access models.Access` и прятать приватную запись как отсутствующую для не-владельца: `if rec.Private && access != models.AccessFull { return models.ErrNotFound }` (образец — `internal/usecases/get_repository/scenario.go`).
- **Real-store тесты обязательны.** Каждая пишущая сущность получает сквозной тест на реальном SQLite-сторе в `internal/httpapi/write_store_test.go` (не только фейковый стор в usecase-тестах) — здесь `TestFamilyWriteContractWithRealStore` (Шаг 1.6).
- **Fetch-then-merge на каждом update.** `update_family` читает текущую версию через `GetFamily`, накладывает новые поля, валидирует и сохраняет через `SaveFamily` — полная замена записи, не частичный patch (свойство самого generic-стора, см. `entity-write.md` §1).
- **MCP «отсутствие необязательного списочного аргумента — значит сохранить как есть».** Для `sources` (и любого другого необязательного списка) в `family_update`: аргумент не передан — текущее значение сохраняется; передан пустым списком `[]` — очищает. HTTP `PUT`, наоборот, всегда полностью заменяет тело — отсутствие ключа `sources` в JSON очищает список.
- **Формулировка описания поиска обязана совпадать с реальным списком полей `replaceSearchIndex`.** `family_search`/`GET /api/families/search` называют только `name` — в `internal/store/sqlstore/relations.go` (`SaveFamily`) индексируется только `map[string][]string{"name": {f.Name}}`, `members`/`notes` не индексируются. Расхождение однажды уже поймано финальным ревью подпроекта 4 (`Note`) — с тех пор проверяется явно при каждом новом подпроекте.
- **Мягкая ссылка (`TextRef`) никогда не проверяется на существование при сохранении, независимо от того, есть ли у цели свой CRUD-слой.** `Family.Members` указывает на `Person` (`TypePerson`), у которого ещё нет CRUD (подпроект 8) — это не блокирует `Family`: принцип действует с самого первого появления `TextRef` в программе, не специфика `Family`. Веб-форма редактирует `Members` тем же `TextRefListEditor`, что и `Repository.URLs`/любой другой список `TextRef` — без «пикера по персоне» (см. `entity-write.md` §5 — picker для `TextRef.Ref` вне объёма программы до подпроекта 9).

## Предпосылка: живая проверка

Весь код ниже применён и проверен в отдельном throwaway git worktree (`/Users/asmarin/dev/mine/genodex-verify-subproject7`, ветка `verify/entity-write-subproject7`, форкнута от `main`) — не на `main` напрямую и не в этом плане с нуля: он транскрибирован из уже рабочего дерева, а не написан заново. Полный отчёт того прохода — `/Users/asmarin/dev/mine/genodex-verify-subproject7/REPORT.md` — задокументировал, что было сделано и почему; существенное перенесено сюда, чтобы будущий читатель плана понимал не только ЧТО делает код ниже, но и ПОЧЕМУ он выглядит именно так.

**Проверка** (`REPORT.md`): `gofmt -l .` пусто, `go build/vet/test ./...` зелёные — **1252 теста, 118 пакетов** (было 1216 на `main` до этого подпроекта — 36 новых: list_families 2, search_families 2, get_family 5, create_family 6, update_family 4, delete_family 3, `internal/httpapi/family_test.go` 3, `internal/httpapi/family_write_test.go` 5, `internal/httpapi/write_store_test.go` (`TestFamilyWriteContractWithRealStore`) 1, `internal/mcp/family_test.go` 5), `npm run typecheck`/`npm run build` чисты. Живой смок-тест через `curl` (реальный `genodex serve`, чистая БД): создание записи с `private:false` и отдельно с `private:true` (round-trip через `POST`, `GET` анонимно → 404, `GET` от владельца → 200); `PUT` в обе стороны `private:false→true→false` (оба направления проверены явно); list/search корректно скрывают приватную запись от анонимного вызова; строгий FK `sources[i].citation_id` проверен в той же транзакции, что и сохранение (валидный `citation_id` round-trip'ится, несуществующий — 422 на поле `sources[0].citation_id`); `DELETE` → 204, повторный `GET` → 404; MCP — `tools/list` отдаёт все шесть `family_*` тулов, `family_create` через `tools/call` возвращает созданную запись с `private:true`. Веб-UI (браузер, залогинен как `owner`): `/families` → «+ добавить» → `CreateFamilyModal` (поля `Название`, `Members`/`Notes` через `TextRefListEditor`, `Sources` через `SourceLinkListEditor` с живым списком цитат, чекбокс «Приватная запись») → создание → `/families/{id}` → `FamilyView` корректно показывает все поля → «Редактировать» → форма предзаполнена включая чекбокс приватности → снятие чекбокса → «Сохранить» → `PUT` прошёл, view обновился на «Приватная: нет» — update round-trip подтверждён через реальный UI, не только curl.

Поисковый индекс отдельно сверен с `internal/store/sqlstore/relations.go`/`search.go`: только `name` (`replaceSearchIndex(tx, "families", f.ID, map[string][]string{"name": {f.Name}})`) — все формулировки («поиск по началу названия») в MCP-описаниях/doc-комментариях/веб-плейсхолдерах сверены буквально, без переоценки объёма индекса.

**Доки Задачи 1 уже описывают и Задачу 2 (веб), не только бэкенд** — явно проверено по инструкции этого подпроекта (лесон финального ревью подпроекта 6): бэкенд и веб в проверочном worktree строились одним проходом, поэтому единственная точка риска — не забыть web-часть в доках Задачи 1. Проверка построчная: `docs/usage.md` (HTTP-таблица, MCP-таблица, абзац про `sources`) и `docs/architecture.md` (строка `internal/httpapi/`) — контрактные доки, по конвенции этого репозитория не описывают веб-страницы вообще (не только для `Family` — сверено с тем, что аналогичные доки подпроекта 6 тоже не упоминают веб-компоненты в этих двух файлах), поэтому отсутствие web-упоминания в них — не пробел. `CHANGELOG.md`, единственный файл из четырёх, где по конвенции описывается весь подпроект целиком (бэкенд+веб в одном пункте) — новый пункт Шага 1.7 явно перечисляет и бэкенд (usecase/HTTP/MCP), и веб-часть («Веб: страница «Роды» — список/просмотр/редактирование/создание с `TextRefListEditor`... и `SourceLinkListEditor`...», с перечислением файлов `web/src/pages/{FamiliesList,FamilyForm,FamilyView}.tsx`, `web/src/App.tsx`, `web/src/pages/EntityCatalog.tsx`) — подтверждено byte-exact в Шаге 1.7 ниже, пробела нет.

**Design judgment calls из `REPORT.md`, перенесённые сюда дословно по смыслу:**

- Ни один новый бэкенд- или веб-паттерн не потребовался — `Family` подтверждена как «чисто механический» подпроект, которым его и описывала задача. Все конвенции (access-aware `GetFamily`, `InTx` + проверка существования цитаты в create/update, fetch-then-merge в `PUT`, `Sources` редактируемый с рождения, `TextRefListEditor` для мягкого `Members`) скопированы как есть с `Repository`/`ArchiveNode`, без модификации.
- `FamilyService`/`Deps.Families` размещены в конце каждой структуры `Deps` (после `Citations`), по хронологическому порядку подпроектов, а не по алфавиту — согласуется с тем, как добавлялось поле каждого предыдущего подпроекта.
- Добавлен отдельный тест `TestCreateFamilyMembersSoftRefNotChecked` и явная проверка `members` в `TestFamilyWriteContractWithRealStore`, чтобы утверждение «мягкая ссылка, без проверки существования» было проверяемо в CI, а не только задокументировано в прозе — у `Repository` эквивалента нет, поскольку ни одно из его полей `TextRef` не указывает на типизированную сущность так, как `Family.Members`.

## Задача 1. Бэкенд: Family — полный стек + доки + real-store тесты

**Интерфейсы, потребляемые из подпроектов 1-6**: `models.Page`/`models.Access`, `internal/httpapi/surname.go`:`parsePage`, `internal/mcp/*.go`:`optionalInt`/`toolJSONResult`/`textRefsFromStrings`, `transport.{TextRef,SourceLink}`, `mcp/object_args.go`:`textRefObjectProperties`/`optionalTextRef`/`sourceLinkObjectProperties`/`optionalSourceLinks` (подпроект 5), `get_repository`'s access-aware `Get`-паттерн, `create_repository`'s проверка-FK-в-транзакции паттерн, `store.Store.{GetFamily,GetCitation,InTx}` (generic-хранилище, `internal/store/deps.go`, уже умеет читать/писать `Family` по id до появления её usecase-слоя — модель `internal/models/family.go`/`family_validate.go` уже существует и не меняется этим подпроектом).
**Производит**: `transport.Family` + Create/Update-варианты, `httpapi.FamilyService`, `mcp.FamilyService`, HTTP-роуты `/api/families*`, MCP-тулы `family_*` (6 штук) — потребляются Задачей 2 (веб-страницы).

**Файлы:**
- Изменить: `internal/httpapi/{deps.go,api.go,httpapi.go}`, `internal/mcp/{deps.go,server.go}`, `internal/app/app.go`, `internal/httpapi/{write_store_test.go,store_test.go}`, `docs/usage.md`, `docs/architecture.md`, `CHANGELOG.md`, `docs/data-model/entity-write.md`
- Создать: `internal/transport/{family,family_write}.go`, `internal/usecases/{list_families,search_families,get_family,create_family,update_family,delete_family}/{deps.go,scenario.go,scenario_test.go}`, `internal/httpapi/{family,family_write,family_test,family_write_test}.go`, `internal/mcp/{family,family_test}.go`

**Важно для исполнителя**: `list_families`/`search_families`/`delete_family` — механические, по образцу `list_repositories`/`search_repositories`/`delete_repository`. `get_family` — access-aware с первого черновика (`Family` несёт `Private`). `create_family`/`update_family` — единственная проверка — `sources[i].citation_id` (строгий FK), в той же транзакции, что и сохранение (`InTx` с рождения — `Family` заведена уже после того, как `Citation` получил CRUD, ретрофита `Save → InTx` не требуется). `Members`/`Notes` — списки `TextRef`, копируются как есть без проверки существования цели (мягкая ссылка). Переносить код ниже как есть, файл за файлом — итоговое содержимое, не диффы (кроме секции документации, Шаг 1.7, — там даны точные фрагменты-вставки в существующие большие файлы).

### 1.1. Транспорт

#### `internal/transport/family.go` (создать)
```go
package transport

import "github.com/amarin/genodex/internal/models"

// Family — контракт рода/линии (GET /api/families, MCP-тул family_list).
// Sources редактируется с рождения контракта (сущность заведена уже после
// подпроекта 5, см. internal/transport/source_link.go).
type Family struct {
	ID      models.ID    `json:"id"`
	Name    string       `json:"name"`
	Members []TextRef    `json:"members"`
	Notes   []TextRef    `json:"notes"`
	Sources []SourceLink `json:"sources"`
	Private bool         `json:"private"`
}

// FamilyFromModel конвертирует запись в контракт.
func FamilyFromModel(f models.Family) Family {
	return Family{
		ID:      f.ID,
		Name:    f.Name,
		Members: TextRefsFromModel(f.Members),
		Notes:   TextRefsFromModel(f.Notes),
		Sources: SourceLinksFromModel(f.Sources),
		Private: f.Private,
	}
}

// FamiliesFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func FamiliesFromModels(fs []models.Family) []Family {
	out := make([]Family, 0, len(fs))
	for _, f := range fs {
		out = append(out, FamilyFromModel(f))
	}

	return out
}
```
#### `internal/transport/family_write.go` (создать)
```go
package transport

import "github.com/amarin/genodex/internal/models"

// FamilyCreate — тело POST /api/families и аргументы тула family_create.
// Идентификатор генерирует сценарий.
type FamilyCreate struct {
	Name    string       `json:"name"`
	Members []TextRef    `json:"members"`
	Notes   []TextRef    `json:"notes"`
	Sources []SourceLink `json:"sources"`
	Private bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (f FamilyCreate) Model() models.Family {
	return models.Family{
		Name:    f.Name,
		Members: TextRefsToModel(f.Members),
		Notes:   TextRefsToModel(f.Notes),
		Sources: SourceLinksToModel(f.Sources),
		Private: f.Private,
	}
}

// FamilyUpdate — тело PUT /api/families/{id} и аргументы тула
// family_update: полная замена name/members/notes/sources/private.
type FamilyUpdate struct {
	Name    string       `json:"name"`
	Members []TextRef    `json:"members"`
	Notes   []TextRef    `json:"notes"`
	Sources []SourceLink `json:"sources"`
	Private bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (f FamilyUpdate) Model() models.Family {
	return models.Family{
		Name:    f.Name,
		Members: TextRefsToModel(f.Members),
		Notes:   TextRefsToModel(f.Notes),
		Sources: SourceLinksToModel(f.Sources),
		Private: f.Private,
	}
}
```

### 1.2. Usecase-сценарии

#### `internal/usecases/list_families/`

#### `internal/usecases/list_families/deps.go` (создать)
```go
package list_families

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// FamilyRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type FamilyRepo interface {
	ListFamilies(ctx context.Context, access models.Access, page models.Page) ([]*models.Family, error)
}
```
#### `internal/usecases/list_families/scenario.go` (создать)
```go
package list_families

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список записей родов».
type Scenario struct {
	families FamilyRepo
}

// New создаёт сценарий.
func New(families FamilyRepo) *Scenario {
	return &Scenario{families: families}
}

// ListFamilies возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListFamilies(ctx context.Context, access models.Access, page models.Page) ([]models.Family, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeFamily, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeFamily, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.families.ListFamilies(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Family, 0, len(list))
	for _, f := range list {
		out = append(out, *f)
	}

	return out, nil
}
```
#### `internal/usecases/list_families/scenario_test.go` (создать)
```go
package list_families

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.Family
}

func (f *fakeRepo) ListFamilies(_ context.Context, _ models.Access, page models.Page) ([]*models.Family, error) {
	f.page = page

	return f.out, nil
}

func TestListFamiliesReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.Family{{ID: "F-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "Ивановы"}}}

	got, err := New(repo).ListFamilies(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListFamilies: %v", err)
	}

	if len(got) != 1 || got[0].Name != "Ивановы" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListFamiliesRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListFamilies(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
```

#### `internal/usecases/search_families/`

#### `internal/usecases/search_families/deps.go` (создать)
```go
package search_families

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// FamilyRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type FamilyRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetFamily(ctx context.Context, id models.ID) (*models.Family, error)
}
```
#### `internal/usecases/search_families/scenario.go` (создать)
```go
package search_families

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск записей родов».
type Scenario struct {
	families FamilyRepo
}

// New создаёт сценарий.
func New(families FamilyRepo) *Scenario {
	return &Scenario{families: families}
}

// SearchFamilies находит записи, чьё название начинается с текста
// запроса — та же механика, что и search_divisions.SearchDivisions (см. её
// комментарий).
func (s *Scenario) SearchFamilies(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Family, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Family{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Family{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.families.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeFamily {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.families.GetFamily(ctx, h.ID)
			if err != nil {
				if errors.Is(err, models.ErrNotFound) {
					matched++
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
#### `internal/usecases/search_families/scenario_test.go` (создать)
```go
package search_families

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits     []models.Hit
	families map[models.ID]*models.Family
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetFamily(_ context.Context, id models.ID) (*models.Family, error) {
	s, ok := f.families[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestSearchFamiliesFiltersByType(t *testing.T) {
	id := models.ID("F-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeFamily, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не family — должен быть пропущен
		},
		families: map[models.ID]*models.Family{id: {ID: id, Name: "Ивановы"}},
	}

	got, err := New(repo).SearchFamilies(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchFamilies: %v", err)
	}

	if len(got) != 1 || got[0].Name != "Ивановы" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchFamiliesEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchFamilies(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchFamilies: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
```

#### `internal/usecases/get_family/`

#### `internal/usecases/get_family/deps.go` (создать)
```go
package get_family

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// FamilyRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type FamilyRepo interface {
	GetFamily(ctx context.Context, id models.ID) (*models.Family, error)
}
```
#### `internal/usecases/get_family/scenario.go` (создать)
```go
package get_family

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «запись рода по идентификатору».
type Scenario struct {
	families FamilyRepo
}

// New создаёт сценарий.
func New(families FamilyRepo) *Scenario {
	return &Scenario{families: families}
}

// GetFamily возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound. Приватная запись
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound — тот же принцип «прячем как отсутствующее», что и в
// List/Search.
func (s *Scenario) GetFamily(ctx context.Context, access models.Access, id models.ID) (models.Family, error) {
	if err := validateID(id); err != nil {
		return models.Family{}, err
	}

	f, err := s.families.GetFamily(ctx, id)
	if err != nil {
		return models.Family{}, err
	}

	if f.Private && access != models.AccessFull {
		return models.Family{}, models.ErrNotFound
	}

	return *f, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeFamily)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeFamily,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```
#### `internal/usecases/get_family/scenario_test.go` (создать)
```go
package get_family

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	families map[models.ID]*models.Family
}

func (f *fakeRepo) GetFamily(_ context.Context, id models.ID) (*models.Family, error) {
	s, ok := f.families[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestGetFamilyReturnsRecord(t *testing.T) {
	id := models.ID("F-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{families: map[models.ID]*models.Family{id: {ID: id, Name: "Ивановы"}}}

	got, err := New(repo).GetFamily(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetFamily: %v", err)
	}

	if got.Name != "Ивановы" {
		t.Fatalf("Name = %q", got.Name)
	}
}

func TestGetFamilyNotFound(t *testing.T) {
	repo := &fakeRepo{families: map[models.ID]*models.Family{}}

	_, err := New(repo).GetFamily(context.Background(), models.AccessFull, "F-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetFamilyInvalidID(t *testing.T) {
	repo := &fakeRepo{families: map[models.ID]*models.Family{}}

	_, err := New(repo).GetFamily(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestGetFamilyPrivateHiddenFromPublic(t *testing.T) {
	id := models.ID("F-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{families: map[models.ID]*models.Family{id: {ID: id, Name: "Ивановы", Private: true}}}

	_, err := New(repo).GetFamily(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for private record with AccessPublic", err)
	}
}

func TestGetFamilyPrivateVisibleToFullAccess(t *testing.T) {
	id := models.ID("F-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{families: map[models.ID]*models.Family{id: {ID: id, Name: "Ивановы", Private: true}}}

	got, err := New(repo).GetFamily(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetFamily: %v", err)
	}

	if got.Name != "Ивановы" {
		t.Fatalf("Name = %q", got.Name)
	}
}
```

#### `internal/usecases/create_family/`

#### `internal/usecases/create_family/deps.go` (создать)
```go
package create_family

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// FamilyStore — зависимость сценария: транзакция порта store.Store.
// Проверка ссылок (Sources) и сохранение идут в одной транзакции на
// переданном fn хранилище.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type FamilyStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
```
#### `internal/usecases/create_family/scenario.go` (создать)
```go
package create_family

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание записи рода».
type Scenario struct {
	store FamilyStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st FamilyStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateFamily создаёт запись: генерирует идентификатор, проверяет
// инварианты, в одной транзакции убеждается в существовании цитат из Sources
// и сохраняет. Возвращает созданную запись с заполненным ID.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта), невалидная сущность и несуществующая цитата —
// *models.ValidationError; прочее — ошибки хранилища как есть.
func (s *Scenario) CreateFamily(ctx context.Context, f models.Family) (models.Family, error) {
	if f.ID != "" {
		return models.Family{}, &models.ValidationError{
			Entity: models.TypeFamily,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", f.ID),
		}
	}

	f.ID = s.ids.New(models.TypeFamily)

	if err := f.Validate(); err != nil {
		return models.Family{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		for i, link := range f.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveFamily(ctx, &f)
	})
	if err != nil {
		return models.Family{}, err
	}

	return f, nil
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeFamily,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
```
#### `internal/usecases/create_family/scenario_test.go` (создать)
```go
package create_family

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// cID возвращает корректный идентификатор цитаты, отличающийся последним символом.
func cID(last byte) models.ID {
	return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

type stubIDs struct{ id models.ID }

func (s *stubIDs) New(models.Type) models.ID { return s.id }

// fakeTx реализует нужные сценарию методы store.Store поверх карты цитат;
// остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	citations map[models.ID]*models.Citation
	saved     *models.Family
	saveErr   error
}

func newFakeTx(existingCitations ...*models.Citation) *fakeTx {
	tx := &fakeTx{citations: map[models.ID]*models.Citation{}}
	for _, c := range existingCitations {
		tx.citations[c.ID] = c
	}

	return tx
}

func (f *fakeTx) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *c

	return &cp, nil
}

func (f *fakeTx) SaveFamily(_ context.Context, s *models.Family) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *s
	f.saved = &cp

	return nil
}

// fakeStore реализует FamilyStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func TestCreateFamilyGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "F-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{tx: newFakeTx()}
	sc := New(st, ids)

	got, err := sc.CreateFamily(context.Background(), models.Family{Name: "Ивановы"})
	if err != nil {
		t.Fatalf("CreateFamily: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.tx.saved == nil || st.tx.saved.Name != "Ивановы" {
		t.Fatalf("saved = %+v", st.tx.saved)
	}
}

func TestCreateFamilyRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx()}, &stubIDs{})

	_, err := sc.CreateFamily(context.Background(), models.Family{ID: "F-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "x"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestCreateFamilyRejectsEmptyName(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx()}, &stubIDs{id: "F-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateFamily(context.Background(), models.Family{})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, want ValidationError on name", err)
	}
}

func TestCreateFamilyPropagatesStoreError(t *testing.T) {
	wantErr := errors.New("db down")
	st := &fakeStore{tx: newFakeTx()}
	st.tx.saveErr = wantErr
	sc := New(st, &stubIDs{id: "F-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateFamily(context.Background(), models.Family{Name: "Ивановы"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

// TestCreateFamilySourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestCreateFamilySourceCitationNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	sc := New(st, &stubIDs{id: "F-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	in := models.Family{Name: "Ивановы", Sources: []models.SourceLink{{CitationID: cID('0')}}}

	_, err := sc.CreateFamily(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if st.tx.saved != nil {
		t.Fatalf("saved = %+v; ожидалось: ничего не сохранено", st.tx.saved)
	}
}

// TestCreateFamilyMembersSoftRefNotChecked: Members — мягкая ссылка на
// TypePerson (у Person ещё нет CRUD, подпроект 8), TextRef.Ref не
// проверяется на существование при сохранении — только формат.
func TestCreateFamilyMembersSoftRefNotChecked(t *testing.T) {
	ids := &stubIDs{id: "F-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{tx: newFakeTx()}
	sc := New(st, ids)

	in := models.Family{
		Name: "Ивановы",
		Members: []models.TextRef{
			{Text: "Иван Иванов", Ref: "I-01ARZ3NDEKTSV4RRFFQ69G5FA9", Type: models.TypePerson},
		},
	}

	got, err := sc.CreateFamily(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateFamily: %v", err)
	}

	if len(got.Members) != 1 || got.Members[0].Ref != "I-01ARZ3NDEKTSV4RRFFQ69G5FA9" {
		t.Fatalf("Members = %+v", got.Members)
	}
}
```

#### `internal/usecases/update_family/`

#### `internal/usecases/update_family/deps.go` (создать)
```go
package update_family

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// FamilyStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования и сохранение идут в одной транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type FamilyStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
```
#### `internal/usecases/update_family/scenario.go` (создать)
```go
package update_family

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение записи рода».
type Scenario struct {
	store FamilyStore
}

// New создаёт сценарий.
func New(st FamilyStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateFamily полностью заменяет запись по f.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, и сохраняет.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateFamily(ctx context.Context, f models.Family) error {
	if err := f.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetFamily(ctx, f.ID); err != nil {
			return err
		}

		for i, link := range f.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveFamily(ctx, &f)
	})
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeFamily,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
```
#### `internal/usecases/update_family/scenario_test.go` (создать)
```go
package update_family

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// fakeTx реализует нужные сценарию методы store.Store поверх карты записей;
// остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	families  map[models.ID]*models.Family
	citations map[models.ID]*models.Citation
	saved     []*models.Family
}

func newFakeTx(existing ...*models.Family) *fakeTx {
	tx := &fakeTx{families: map[models.ID]*models.Family{}, citations: map[models.ID]*models.Citation{}}
	for _, s := range existing {
		tx.families[s.ID] = s
	}

	return tx
}

// cID возвращает корректный идентификатор цитаты.
func cID(last byte) models.ID {
	return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func (f *fakeTx) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *c

	return &cp, nil
}

func (f *fakeTx) GetFamily(_ context.Context, id models.ID) (*models.Family, error) {
	s, ok := f.families[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *s

	return &cp, nil
}

func (f *fakeTx) SaveFamily(_ context.Context, s *models.Family) error {
	cp := *s
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует FamilyStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func family(id models.ID, name string) *models.Family {
	return &models.Family{ID: id, Name: name}
}

func TestUpdateFamilySaves(t *testing.T) {
	existing := family("F-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Ивановы")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Name = "Ивановы (испр.)"

	if err := New(st).UpdateFamily(context.Background(), updated); err != nil {
		t.Fatalf("UpdateFamily: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Name != "Ивановы (испр.)" {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateFamilyNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateFamily(context.Background(), *family("F-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Ивановы"))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateFamilyRejectsEmptyName(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateFamily(context.Background(), models.Family{ID: "F-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, want ValidationError on name", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

// TestUpdateFamilySourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestUpdateFamilySourceCitationNotFound(t *testing.T) {
	existing := family("F-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Ивановы")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Sources = []models.SourceLink{{CitationID: cID('0')}}

	err := New(st).UpdateFamily(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d записей при несуществующей цитате", len(st.tx.saved))
	}
}
```

#### `internal/usecases/delete_family/`

#### `internal/usecases/delete_family/deps.go` (создать)
```go
package delete_family

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// FamilyRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type FamilyRepo interface {
	DeleteFamily(ctx context.Context, id models.ID) error
}
```
#### `internal/usecases/delete_family/scenario.go` (создать)
```go
package delete_family

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление записи рода».
type Scenario struct {
	families FamilyRepo
}

// New создаёт сценарий.
func New(families FamilyRepo) *Scenario {
	return &Scenario{families: families}
}

// DeleteFamily удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteFamily(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.families.DeleteFamily(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeFamily)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeFamily,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```
#### `internal/usecases/delete_family/scenario_test.go` (создать)
```go
package delete_family

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	deleted []models.ID
	err     error
}

func (f *fakeRepo) DeleteFamily(_ context.Context, id models.ID) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)

	return nil
}

func TestDeleteFamilyCallsRepo(t *testing.T) {
	repo := &fakeRepo{}
	id := models.ID("F-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	if err := New(repo).DeleteFamily(context.Background(), id); err != nil {
		t.Fatalf("DeleteFamily: %v", err)
	}

	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("deleted = %v", repo.deleted)
	}
}

func TestDeleteFamilyInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	err := New(repo).DeleteFamily(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}

	if len(repo.deleted) != 0 {
		t.Fatalf("repo вызван при невалидном id")
	}
}

func TestDeleteFamilyPropagatesInUseError(t *testing.T) {
	wantErr := &models.InUseError{Type: models.TypeFamily, ID: "F-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	repo := &fakeRepo{err: wantErr}

	err := New(repo).DeleteFamily(context.Background(), "F-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	var iu *models.InUseError
	if !errors.As(err, &iu) {
		t.Fatalf("err = %v, want InUseError", err)
	}
}
```

### 1.3. HTTP API

#### `internal/httpapi/family.go` (создать)
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleFamilyList — GET /api/families?limit=&offset=. Чтение открыто
// анонимному посетителю.
func handleFamilyList(families FamilyService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := families.ListFamilies(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.FamiliesFromModels(list))
	}
}

// handleFamilySearch — GET /api/families/search?q=&limit=&offset=.
func handleFamilySearch(families FamilyService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := families.SearchFamilies(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.FamiliesFromModels(list))
	}
}

// handleFamilyGet — GET /api/families/{id}.
func handleFamilyGet(families FamilyService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f, err := families.GetFamily(r.Context(), AccessFromContext(r.Context()), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.FamilyFromModel(f))
	}
}
```
#### `internal/httpapi/family_write.go` (создать)
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleFamilyCreate — POST /api/families: создаёт запись, отвечает
// 201 с созданной записью (id генерирует сценарий). Запись — только для
// вошедшего владельца, см. handleDivisionCreate.
func handleFamilyCreate(families FamilyService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.FamilyCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := families.CreateFamily(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.FamilyFromModel(created))
	}
}

// handleFamilyUpdate — PUT /api/families/{id}: полная замена
// name/members/notes/sources/private. Читает текущую версию, накладывает
// поля запроса (fetch-then-merge, docs/data-model/entity-write.md §3).
func handleFamilyUpdate(families FamilyService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.FamilyUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := families.GetFamily(r.Context(), AccessFromContext(r.Context()), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Name = m.Name
		cur.Members = m.Members
		cur.Notes = m.Notes
		cur.Sources = m.Sources
		cur.Private = m.Private

		if err := families.UpdateFamily(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.FamilyFromModel(cur))
	}
}

// handleFamilyDelete — DELETE /api/families/{id}: 204 без тела; занятая
// запись — 409 со списком ссылающихся. Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handleFamilyDelete(families FamilyService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := families.DeleteFamily(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
```
#### `internal/httpapi/family_test.go` (создать)
```go
package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeFamilies struct {
	list []models.Family
	err  error
	page models.Page

	getF      models.Family
	gotIDs    []models.ID
	created   models.Family
	gotCreate models.Family
	updated   models.Family
	deleteErr error

	search    []models.Family
	gotSearch models.SearchQuery
}

func (f *fakeFamilies) ListFamilies(_ context.Context, _ models.Access, page models.Page) ([]models.Family, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeFamilies) SearchFamilies(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Family, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeFamilies) GetFamily(_ context.Context, _ models.Access, id models.ID) (models.Family, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Family{}, f.err
	}

	return f.getF, nil
}

func (f *fakeFamilies) CreateFamily(_ context.Context, fam models.Family) (models.Family, error) {
	f.gotCreate = fam
	if f.err != nil {
		return models.Family{}, f.err
	}

	return f.created, nil
}

func (f *fakeFamilies) UpdateFamily(_ context.Context, fam models.Family) error {
	f.updated = fam

	return f.err
}

func (f *fakeFamilies) DeleteFamily(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestFamilyListReturnsRecords(t *testing.T) {
	svc := &fakeFamilies{list: []models.Family{{ID: "F-1", Name: "Ивановы"}}}

	rec := get(t, NewHandler(Deps{Families: svc, DocsFS: fstest.MapFS{}}), "/api/families")
	requireStatus(t, rec, 200)

	want := `[{"id":"F-1","name":"Ивановы","members":[],"notes":[],"sources":[],"private":false}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestFamilyGetNotFound(t *testing.T) {
	svc := &fakeFamilies{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Families: svc, DocsFS: fstest.MapFS{}}), "/api/families/F-1")
	requireStatus(t, rec, 404)
}

func TestFamilySearchPassesQuery(t *testing.T) {
	svc := &fakeFamilies{}

	rec := get(t, NewHandler(Deps{Families: svc, DocsFS: fstest.MapFS{}}), "/api/families/search?q=Ив")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "Ив" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
```
#### `internal/httpapi/family_write_test.go` (создать)
```go
package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

func TestFamilyCreateContract(t *testing.T) {
	svc := &fakeFamilies{created: models.Family{ID: "F-1", Name: "Ивановы"}}

	rec := postD(t, NewHandler(Deps{Families: svc, DocsFS: fstest.MapFS{}}), "/api/families",
		`{"name":"Ивановы"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Name != "Ивановы" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
	if !strings.Contains(rec.Body.String(), `"name":"Ивановы"`) {
		t.Fatalf("body = %s, name не в ответе", rec.Body)
	}
}

func TestFamilyCreateAnonymousIs401(t *testing.T) {
	svc := &fakeFamilies{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Families: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/families", strings.NewReader(`{"name":"Ивановы"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestFamilyUpdateMergesFields(t *testing.T) {
	svc := &fakeFamilies{getF: models.Family{ID: "F-1", Name: "Ивановы"}}

	rec := putD(t, NewHandler(Deps{Families: svc, DocsFS: fstest.MapFS{}}), "/api/families/F-1",
		`{"name":"Ивановы (испр.)","private":true}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Name != "Ивановы (испр.)" || svc.updated.Private != true || svc.updated.ID != "F-1" {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestFamilyDeleteNoContent(t *testing.T) {
	svc := &fakeFamilies{}

	rec := delD(t, NewHandler(Deps{Families: svc, DocsFS: fstest.MapFS{}}), "/api/families/F-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestFamilyDeleteInUseIs409(t *testing.T) {
	svc := &fakeFamilies{deleteErr: &models.InUseError{Type: models.TypeFamily, ID: "F-1"}}

	rec := delD(t, NewHandler(Deps{Families: svc, DocsFS: fstest.MapFS{}}), "/api/families/F-1")

	requireStatus(t, rec, http.StatusConflict)
}
```

### 1.4. MCP

#### `internal/mcp/family.go` (создать)
```go
package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// registerFamilyTools регистрирует тулы для работы с родами/линиями.
// members/notes — только текстом (v1, docs/data-model/entity-write.md §4).
// ВАЖНО: family_update заменяет списки целиком текстом — существующие
// ref/type будут потеряны при любом обновлении через MCP, пока не появится
// picker.
func registerFamilyTools(s *server.MCPServer, families FamilyService) {
	tool := mcp.NewTool(
		"family_list",
		mcp.WithDescription("Список родов/линий в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, familyListHandler(families))

	tool = mcp.NewTool(
		"family_search",
		mcp.WithDescription("Поиск родов по началу названия; результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало названия")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, familySearchHandler(families))

	tool = mcp.NewTool(
		"family_get",
		mcp.WithDescription("Род по id; результат — JSON записи. Неверный формат id или отсутствующая запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например F-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, familyGetHandler(families))

	tool = mcp.NewTool(
		"family_create",
		mcp.WithDescription("Создать род; id генерируется сервером; результат — JSON созданной записи"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Название")),
		mcp.WithArray("members", mcp.WithStringItems(), mcp.Description("Члены рода (текстом; мягкая ссылка на персону, без проверки существования)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, familyCreateHandler(families))

	tool = mcp.NewTool(
		"family_update",
		mcp.WithDescription("Изменить род: полная замена name/members/notes/private; результат — JSON обновлённой записи; sources — при отсутствии в вызове текущие источники сохраняются, пустой массив — очищает их"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Новое название")),
		mcp.WithArray("members", mcp.WithStringItems(), mcp.Description("Члены рода")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, familyUpdateHandler(families))

	tool = mcp.NewTool(
		"family_delete",
		mcp.WithDescription("Удалить род. Необратимо. Если на него есть строгие ссылки — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, familyDeleteHandler(families))
}

func familyListHandler(families FamilyService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := families.ListFamilies(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.FamiliesFromModels(list))
	}
}

func familySearchHandler(families FamilyService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := families.SearchFamilies(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.FamiliesFromModels(list))
	}
}

func familyGetHandler(families FamilyService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		f, err := families.GetFamily(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.FamilyFromModel(f))
	}
}

func familyCreateHandler(families FamilyService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sources, err := optionalSourceLinks(req.GetArguments(), "sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		f := models.Family{
			Name:    req.GetString("name", ""),
			Members: textRefsFromStrings(req.GetStringSlice("members", nil)),
			Notes:   textRefsFromStrings(req.GetStringSlice("notes", nil)),
			Sources: sources,
			Private: req.GetBool("private", false),
		}

		created, err := families.CreateFamily(ctx, f)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.FamilyFromModel(created))
	}
}

func familyUpdateHandler(families FamilyService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := families.GetFamily(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		cur.Name = req.GetString("name", "")
		cur.Members = textRefsFromStrings(req.GetStringSlice("members", nil))
		cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))
		cur.Private = req.GetBool("private", false)

		if raw, ok := req.GetArguments()["sources"]; ok && raw != nil {
			sources, err := optionalSourceLinks(req.GetArguments(), "sources")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Sources = sources
		}

		if err := families.UpdateFamily(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.FamilyFromModel(cur))
	}
}

func familyDeleteHandler(families FamilyService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := families.DeleteFamily(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
```
#### `internal/mcp/family_test.go` (создать)
```go
package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakeFamilies struct {
	list []models.Family
	err  error

	getF      models.Family
	created   models.Family
	gotCreate models.Family
	updated   models.Family
	gotIDs    []models.ID
	deleteErr error

	search []models.Family
}

func (f *fakeFamilies) ListFamilies(context.Context, models.Access, models.Page) ([]models.Family, error) {
	return f.list, f.err
}

func (f *fakeFamilies) SearchFamilies(context.Context, models.Access, models.SearchQuery) ([]models.Family, error) {
	return f.search, f.err
}

func (f *fakeFamilies) GetFamily(_ context.Context, _ models.Access, id models.ID) (models.Family, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Family{}, f.err
	}

	return f.getF, nil
}

func (f *fakeFamilies) CreateFamily(_ context.Context, fam models.Family) (models.Family, error) {
	f.gotCreate = fam
	if f.err != nil {
		return models.Family{}, f.err
	}

	return f.created, nil
}

func (f *fakeFamilies) UpdateFamily(_ context.Context, fam models.Family) error {
	f.updated = fam

	return f.err
}

func (f *fakeFamilies) DeleteFamily(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callFamilyTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestFamilyGetToolContract(t *testing.T) {
	svc := &fakeFamilies{getF: models.Family{ID: "F-1", Name: "Ивановы"}}

	res := callFamilyTool(t, familyGetHandler(svc), map[string]any{"id": "F-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "F-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

func TestFamilyCreateToolPassesName(t *testing.T) {
	svc := &fakeFamilies{created: models.Family{ID: "F-new", Name: "Ивановы"}}

	res := callFamilyTool(t, familyCreateHandler(svc), map[string]any{
		"name": "Ивановы",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.Name != "Ивановы" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

func TestFamilyUpdateToolSetsFields(t *testing.T) {
	svc := &fakeFamilies{getF: models.Family{ID: "F-1", Name: "Ивановы"}}

	res := callFamilyTool(t, familyUpdateHandler(svc), map[string]any{
		"id":      "F-1",
		"name":    "Ивановы",
		"private": true,
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Private != true {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestFamilyDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeFamilies{deleteErr: &models.InUseError{Type: models.TypeFamily, ID: "F-1"}}

	res := callFamilyTool(t, familyDeleteHandler(svc), map[string]any{"id": "F-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersFamilyTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Families: &fakeFamilies{}}).ListTools()

	for _, name := range []string{"family_list", "family_search", "family_get", "family_create", "family_update", "family_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
```

### 1.5. `Deps`-реестр: подключить Family

#### `internal/httpapi/deps.go` (изменить — итоговое содержимое)
```go
package httpapi

import (
	"context"

	authpkg "github.com/amarin/genodex/internal/auth"
	"github.com/amarin/genodex/internal/models"
)

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

// SurnameService — контракт сценариев словарных записей фамилий, отдаваемых
// в HTTP: список, поиск, чтение, создание, изменение, удаление.
type SurnameService interface {
	ListSurnames(ctx context.Context, access models.Access, page models.Page) ([]models.Surname, error)
	SearchSurnames(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Surname, error)
	GetSurname(ctx context.Context, id models.ID) (models.Surname, error)
	CreateSurname(ctx context.Context, sn models.Surname) (models.Surname, error)
	UpdateSurname(ctx context.Context, sn models.Surname) error
	DeleteSurname(ctx context.Context, id models.ID) error
}

// PatronymicService — контракт сценариев словарных записей (отчеств), отдаваемых
// в HTTP: список, поиск, чтение, создание, изменение, удаление.
type PatronymicService interface {
	ListPatronymics(ctx context.Context, access models.Access, page models.Page) ([]models.Patronymic, error)
	SearchPatronymics(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Patronymic, error)
	GetPatronymic(ctx context.Context, id models.ID) (models.Patronymic, error)
	CreatePatronymic(ctx context.Context, x models.Patronymic) (models.Patronymic, error)
	UpdatePatronymic(ctx context.Context, x models.Patronymic) error
	DeletePatronymic(ctx context.Context, id models.ID) error
}

// EstateService — контракт сценариев словарных записей (сословий), отдаваемых
// в HTTP: список, поиск, чтение, создание, изменение, удаление.
type EstateService interface {
	ListEstates(ctx context.Context, access models.Access, page models.Page) ([]models.Estate, error)
	SearchEstates(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Estate, error)
	GetEstate(ctx context.Context, id models.ID) (models.Estate, error)
	CreateEstate(ctx context.Context, x models.Estate) (models.Estate, error)
	UpdateEstate(ctx context.Context, x models.Estate) error
	DeleteEstate(ctx context.Context, id models.ID) error
}

// TitleService — контракт сценариев словарных записей (званий/титулов), отдаваемых
// в HTTP: список, поиск, чтение, создание, изменение, удаление.
type TitleService interface {
	ListTitles(ctx context.Context, access models.Access, page models.Page) ([]models.Title, error)
	SearchTitles(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Title, error)
	GetTitle(ctx context.Context, id models.ID) (models.Title, error)
	CreateTitle(ctx context.Context, x models.Title) (models.Title, error)
	UpdateTitle(ctx context.Context, x models.Title) error
	DeleteTitle(ctx context.Context, id models.ID) error
}

// GivenNameService — контракт сценариев словарных записей имён, отдаваемых
// в HTTP: список, поиск, чтение, создание, изменение, удаление.
type GivenNameService interface {
	ListGivenNames(ctx context.Context, access models.Access, page models.Page) ([]models.GivenName, error)
	SearchGivenNames(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.GivenName, error)
	GetGivenName(ctx context.Context, id models.ID) (models.GivenName, error)
	CreateGivenName(ctx context.Context, x models.GivenName) (models.GivenName, error)
	UpdateGivenName(ctx context.Context, x models.GivenName) error
	DeleteGivenName(ctx context.Context, id models.ID) error
}

// RepositoryService — контракт сценариев хранилищ-контейнеров источников,
// отдаваемых в HTTP: список, поиск, чтение, создание, изменение, удаление.
type RepositoryService interface {
	ListRepositories(ctx context.Context, access models.Access, page models.Page) ([]models.Repository, error)
	SearchRepositories(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Repository, error)
	GetRepository(ctx context.Context, access models.Access, id models.ID) (models.Repository, error)
	CreateRepository(ctx context.Context, r models.Repository) (models.Repository, error)
	UpdateRepository(ctx context.Context, r models.Repository) error
	DeleteRepository(ctx context.Context, id models.ID) error
}

// ChurchService — контракт сценариев церквей, отдаваемых в HTTP: список,
// поиск, чтение, создание, изменение, удаление.
type ChurchService interface {
	ListChurches(ctx context.Context, access models.Access, page models.Page) ([]models.Church, error)
	SearchChurches(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Church, error)
	GetChurch(ctx context.Context, id models.ID) (models.Church, error)
	CreateChurch(ctx context.Context, c models.Church) (models.Church, error)
	UpdateChurch(ctx context.Context, c models.Church) error
	DeleteChurch(ctx context.Context, id models.ID) error
}

// ParishService — контракт сценариев приходов, отдаваемых в HTTP: список,
// поиск, чтение, создание, изменение, удаление.
type ParishService interface {
	ListParishes(ctx context.Context, access models.Access, page models.Page) ([]models.Parish, error)
	SearchParishes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Parish, error)
	GetParish(ctx context.Context, id models.ID) (models.Parish, error)
	CreateParish(ctx context.Context, p models.Parish) (models.Parish, error)
	UpdateParish(ctx context.Context, p models.Parish) error
	DeleteParish(ctx context.Context, id models.ID) error
}

// ArchiveService — контракт сценариев архивов, отдаваемых в HTTP: список,
// поиск, чтение, создание, изменение, удаление.
type ArchiveService interface {
	ListArchives(ctx context.Context, access models.Access, page models.Page) ([]models.Archive, error)
	SearchArchives(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Archive, error)
	GetArchive(ctx context.Context, access models.Access, id models.ID) (models.Archive, error)
	CreateArchive(ctx context.Context, a models.Archive) (models.Archive, error)
	UpdateArchive(ctx context.Context, a models.Archive) error
	DeleteArchive(ctx context.Context, id models.ID) error
}

// ArchiveNodeService — контракт сценариев узлов архивного дерева, отдаваемых
// в HTTP: список (обязательный archive_id — см. models.ArchiveNodeQuery),
// поиск, чтение, создание, изменение, удаление.
type ArchiveNodeService interface {
	ListArchiveNodes(ctx context.Context, access models.Access, q models.ArchiveNodeQuery) ([]models.ArchiveNode, error)
	SearchArchiveNodes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.ArchiveNode, error)
	GetArchiveNode(ctx context.Context, access models.Access, id models.ID) (models.ArchiveNode, error)
	CreateArchiveNode(ctx context.Context, n models.ArchiveNode) (models.ArchiveNode, error)
	UpdateArchiveNode(ctx context.Context, n models.ArchiveNode) error
	DeleteArchiveNode(ctx context.Context, id models.ID) error
}

// ArchiveDocumentService — контракт сценариев документов внутри единиц
// учёта, отдаваемых в HTTP: список, поиск, чтение, создание, изменение,
// удаление.
type ArchiveDocumentService interface {
	ListArchiveDocuments(ctx context.Context, access models.Access, page models.Page) ([]models.ArchiveDocument, error)
	SearchArchiveDocuments(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.ArchiveDocument, error)
	GetArchiveDocument(ctx context.Context, access models.Access, id models.ID) (models.ArchiveDocument, error)
	CreateArchiveDocument(ctx context.Context, d models.ArchiveDocument) (models.ArchiveDocument, error)
	UpdateArchiveDocument(ctx context.Context, d models.ArchiveDocument) error
	DeleteArchiveDocument(ctx context.Context, id models.ID) error
}

// NoteService — контракт сценариев заметок, отдаваемых в HTTP: список,
// поиск, чтение, создание, изменение, удаление.
type NoteService interface {
	ListNotes(ctx context.Context, access models.Access, page models.Page) ([]models.Note, error)
	SearchNotes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Note, error)
	GetNote(ctx context.Context, access models.Access, id models.ID) (models.Note, error)
	CreateNote(ctx context.Context, n models.Note) (models.Note, error)
	UpdateNote(ctx context.Context, n models.Note) error
	DeleteNote(ctx context.Context, id models.ID) error
}

// AttachmentService — контракт сценариев файловых вложений, отдаваемых в
// HTTP: список, поиск, чтение, создание, изменение, удаление.
type AttachmentService interface {
	ListAttachments(ctx context.Context, access models.Access, page models.Page) ([]models.Attachment, error)
	SearchAttachments(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Attachment, error)
	GetAttachment(ctx context.Context, access models.Access, id models.ID) (models.Attachment, error)
	CreateAttachment(ctx context.Context, a models.Attachment) (models.Attachment, error)
	UpdateAttachment(ctx context.Context, a models.Attachment) error
	DeleteAttachment(ctx context.Context, id models.ID) error
}

// SourceService — контракт сценариев источников доказательств, отдаваемых в
// HTTP: список, поиск, чтение, создание, изменение, удаление.
type SourceService interface {
	ListSources(ctx context.Context, access models.Access, page models.Page) ([]models.Source, error)
	SearchSources(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Source, error)
	GetSource(ctx context.Context, access models.Access, id models.ID) (models.Source, error)
	CreateSource(ctx context.Context, s models.Source) (models.Source, error)
	UpdateSource(ctx context.Context, s models.Source) error
	DeleteSource(ctx context.Context, id models.ID) error
}

// CitationService — контракт сценариев цитат, отдаваемых в HTTP: список,
// поиск, чтение, создание, изменение, удаление.
type CitationService interface {
	ListCitations(ctx context.Context, access models.Access, page models.Page) ([]models.Citation, error)
	SearchCitations(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Citation, error)
	GetCitation(ctx context.Context, access models.Access, id models.ID) (models.Citation, error)
	CreateCitation(ctx context.Context, c models.Citation) (models.Citation, error)
	UpdateCitation(ctx context.Context, c models.Citation) error
	DeleteCitation(ctx context.Context, id models.ID) error
}

// FamilyService — контракт сценариев родов/линий, отдаваемых в HTTP: список,
// поиск, чтение, создание, изменение, удаление.
type FamilyService interface {
	ListFamilies(ctx context.Context, access models.Access, page models.Page) ([]models.Family, error)
	SearchFamilies(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Family, error)
	GetFamily(ctx context.Context, access models.Access, id models.ID) (models.Family, error)
	CreateFamily(ctx context.Context, f models.Family) (models.Family, error)
	UpdateFamily(ctx context.Context, f models.Family) error
	DeleteFamily(ctx context.Context, id models.ID) error
}

// AuthService — контракт auth.Service, отдаваемый в HTTP-обработчики.
type AuthService interface {
	Bootstrap(ctx context.Context) (bool, error)
	Register(ctx context.Context, login, password string, invite *string) (authpkg.AuthResult, error)
	Login(ctx context.Context, login, password string) (authpkg.AuthResult, error)
	Refresh(ctx context.Context, rawRefresh string) (authpkg.AuthResult, error)
	Logout(ctx context.Context, rawAccess string) error
	ResolveAccess(ctx context.Context, rawAccess string) (models.Access, *authpkg.ID, error)
	ChangePassword(ctx context.Context, ownerID authpkg.ID, current, newPassword string) error
	CreateInvite(ctx context.Context, ownerID authpkg.ID) (string, error)
	CreateAPIToken(ctx context.Context, ownerID authpkg.ID, label string) (string, authpkg.ID, error)
	ListAPITokens(ctx context.Context, ownerID authpkg.ID) ([]authpkg.APIToken, error)
	RevokeAPIToken(ctx context.Context, ownerID, tokenID authpkg.ID) error
	GetOwner(ctx context.Context, id authpkg.ID) (*authpkg.Owner, error)
}
```
#### `internal/httpapi/api.go` (изменить — итоговое содержимое)
```go
package httpapi

import (
	"io/fs"
	"net/http"
)

// Deps — сервисы, монтируемые в /api (NewAPIHandler) и /api без auth-обёртки
// (NewHandler, юнит-тесты пакета). Явный реестр вместо растущего списка
// позиционных параметров — новая сущность добавляется полем структуры, не
// меняя сигнатуру функций (docs/data-model/entity-write.md §3; решение
// принято при добавлении Surname, первой сущности после AdministrativeDivision).
type Deps struct {
	Divisions    DivisionService
	Surnames     SurnameService
	Patronymics  PatronymicService
	Estates      EstateService
	Titles       TitleService
	GivenNames   GivenNameService
	Repositories RepositoryService
	Churches     ChurchService
	Parishes     ParishService
	Archives     ArchiveService
	ArchiveNodes ArchiveNodeService
	ArchiveDocs  ArchiveDocumentService
	Notes        NoteService
	Attachments  AttachmentService
	Sources      SourceService
	Citations    CitationService
	Families     FamilyService
	Auth         AuthService
	DocsFS       fs.FS
	TrustProxy   bool
}

// NewAPIHandler — единая точка входа /api: маршруты делений, фамилий,
// документации и auth на одном mux, обёрнутые ОДИН раз resolveAccess +
// requireCSRFHeader (auth.md §4 — исходный замысел дизайна: оба миддлвари
// вокруг всего /api/, не только /api/auth/*). Это то, что реально монтирует
// internal/app (этап C). Deps.TrustProxy — см. isSecureRequest (auth.go),
// включается флагом -trust-proxy. NewHandler и NewAuthHandler остаются
// отдельно для существующих юнит-тестов пакета, не зависящих от auth.
func NewAPIHandler(deps Deps) http.Handler {
	mux := http.NewServeMux()
	registerDivisionRoutes(mux, deps.Divisions, deps.DocsFS)

	if deps.Surnames != nil {
		registerSurnameRoutes(mux, deps.Surnames)
	}

	if deps.Patronymics != nil {
		registerPatronymicRoutes(mux, deps.Patronymics)
	}

	if deps.Estates != nil {
		registerEstateRoutes(mux, deps.Estates)
	}

	if deps.Titles != nil {
		registerTitleRoutes(mux, deps.Titles)
	}

	if deps.GivenNames != nil {
		registerGivenNameRoutes(mux, deps.GivenNames)
	}

	if deps.Repositories != nil {
		registerRepositoryRoutes(mux, deps.Repositories)
	}

	if deps.Churches != nil {
		registerChurchRoutes(mux, deps.Churches)
	}

	if deps.Parishes != nil {
		registerParishRoutes(mux, deps.Parishes)
	}

	if deps.Archives != nil {
		registerArchiveRoutes(mux, deps.Archives)
	}

	if deps.ArchiveNodes != nil {
		registerArchiveNodeRoutes(mux, deps.ArchiveNodes)
	}

	if deps.ArchiveDocs != nil {
		registerArchiveDocumentRoutes(mux, deps.ArchiveDocs)
	}

	if deps.Notes != nil {
		registerNoteRoutes(mux, deps.Notes)
	}

	if deps.Attachments != nil {
		registerAttachmentRoutes(mux, deps.Attachments)
	}

	if deps.Sources != nil {
		registerSourceRoutes(mux, deps.Sources)
	}

	if deps.Citations != nil {
		registerCitationRoutes(mux, deps.Citations)
	}

	if deps.Families != nil {
		registerFamilyRoutes(mux, deps.Families)
	}

	registerAuthRoutes(mux, deps.Auth, deps.TrustProxy)

	return requireCSRFHeader(resolveAccess(deps.Auth)(mux))
}
```
#### `internal/httpapi/httpapi.go` (изменить — итоговое содержимое)
```go
package httpapi

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strings"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// NewHandler возвращает http.Handler с маршрутами /api (без auth-
// оборачивания) — используется юнит-тестами этого пакета напрямую.
// Реальное приложение монтирует NewAPIHandler (api.go). Deps.Auth/TrustProxy
// не используются (без auth-обёртки), Deps.Surnames может быть nil, если
// тесту нужны только маршруты делений.
func NewHandler(deps Deps) http.Handler {
	mux := http.NewServeMux()
	registerDivisionRoutes(mux, deps.Divisions, deps.DocsFS)

	if deps.Surnames != nil {
		registerSurnameRoutes(mux, deps.Surnames)
	}

	if deps.Patronymics != nil {
		registerPatronymicRoutes(mux, deps.Patronymics)
	}

	if deps.Estates != nil {
		registerEstateRoutes(mux, deps.Estates)
	}

	if deps.Titles != nil {
		registerTitleRoutes(mux, deps.Titles)
	}

	if deps.GivenNames != nil {
		registerGivenNameRoutes(mux, deps.GivenNames)
	}

	if deps.Repositories != nil {
		registerRepositoryRoutes(mux, deps.Repositories)
	}

	if deps.Churches != nil {
		registerChurchRoutes(mux, deps.Churches)
	}

	if deps.Parishes != nil {
		registerParishRoutes(mux, deps.Parishes)
	}

	if deps.Archives != nil {
		registerArchiveRoutes(mux, deps.Archives)
	}

	if deps.ArchiveNodes != nil {
		registerArchiveNodeRoutes(mux, deps.ArchiveNodes)
	}

	if deps.ArchiveDocs != nil {
		registerArchiveDocumentRoutes(mux, deps.ArchiveDocs)
	}

	if deps.Notes != nil {
		registerNoteRoutes(mux, deps.Notes)
	}

	if deps.Attachments != nil {
		registerAttachmentRoutes(mux, deps.Attachments)
	}

	if deps.Sources != nil {
		registerSourceRoutes(mux, deps.Sources)
	}

	if deps.Citations != nil {
		registerCitationRoutes(mux, deps.Citations)
	}

	if deps.Families != nil {
		registerFamilyRoutes(mux, deps.Families)
	}

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

// registerSurnameRoutes регистрирует маршруты /api/surnames на переданном mux.
func registerSurnameRoutes(mux *http.ServeMux, surnames SurnameService) {
	mux.HandleFunc("GET /api/surnames", handleSurnameList(surnames))
	mux.HandleFunc("GET /api/surnames/search", handleSurnameSearch(surnames))
	mux.HandleFunc("GET /api/surnames/{id}", handleSurnameGet(surnames))
	mux.HandleFunc("POST /api/surnames", handleSurnameCreate(surnames))
	mux.HandleFunc("PUT /api/surnames/{id}", handleSurnameUpdate(surnames))
	mux.HandleFunc("DELETE /api/surnames/{id}", handleSurnameDelete(surnames))
}

// registerPatronymicRoutes регистрирует маршруты /api/patronymics на переданном mux.
func registerPatronymicRoutes(mux *http.ServeMux, patronymics PatronymicService) {
	mux.HandleFunc("GET /api/patronymics", handlePatronymicList(patronymics))
	mux.HandleFunc("GET /api/patronymics/search", handlePatronymicSearch(patronymics))
	mux.HandleFunc("GET /api/patronymics/{id}", handlePatronymicGet(patronymics))
	mux.HandleFunc("POST /api/patronymics", handlePatronymicCreate(patronymics))
	mux.HandleFunc("PUT /api/patronymics/{id}", handlePatronymicUpdate(patronymics))
	mux.HandleFunc("DELETE /api/patronymics/{id}", handlePatronymicDelete(patronymics))
}

// registerEstateRoutes регистрирует маршруты /api/estates на переданном mux.
func registerEstateRoutes(mux *http.ServeMux, estates EstateService) {
	mux.HandleFunc("GET /api/estates", handleEstateList(estates))
	mux.HandleFunc("GET /api/estates/search", handleEstateSearch(estates))
	mux.HandleFunc("GET /api/estates/{id}", handleEstateGet(estates))
	mux.HandleFunc("POST /api/estates", handleEstateCreate(estates))
	mux.HandleFunc("PUT /api/estates/{id}", handleEstateUpdate(estates))
	mux.HandleFunc("DELETE /api/estates/{id}", handleEstateDelete(estates))
}

// registerTitleRoutes регистрирует маршруты /api/titles на переданном mux.
func registerTitleRoutes(mux *http.ServeMux, titles TitleService) {
	mux.HandleFunc("GET /api/titles", handleTitleList(titles))
	mux.HandleFunc("GET /api/titles/search", handleTitleSearch(titles))
	mux.HandleFunc("GET /api/titles/{id}", handleTitleGet(titles))
	mux.HandleFunc("POST /api/titles", handleTitleCreate(titles))
	mux.HandleFunc("PUT /api/titles/{id}", handleTitleUpdate(titles))
	mux.HandleFunc("DELETE /api/titles/{id}", handleTitleDelete(titles))
}

// registerGivenNameRoutes регистрирует маршруты /api/given-names на переданном mux.
func registerGivenNameRoutes(mux *http.ServeMux, givenNames GivenNameService) {
	mux.HandleFunc("GET /api/given-names", handleGivenNameList(givenNames))
	mux.HandleFunc("GET /api/given-names/search", handleGivenNameSearch(givenNames))
	mux.HandleFunc("GET /api/given-names/{id}", handleGivenNameGet(givenNames))
	mux.HandleFunc("POST /api/given-names", handleGivenNameCreate(givenNames))
	mux.HandleFunc("PUT /api/given-names/{id}", handleGivenNameUpdate(givenNames))
	mux.HandleFunc("DELETE /api/given-names/{id}", handleGivenNameDelete(givenNames))
}

// registerRepositoryRoutes регистрирует маршруты /api/repositories на переданном mux.
func registerRepositoryRoutes(mux *http.ServeMux, repositories RepositoryService) {
	mux.HandleFunc("GET /api/repositories", handleRepositoryList(repositories))
	mux.HandleFunc("GET /api/repositories/search", handleRepositorySearch(repositories))
	mux.HandleFunc("GET /api/repositories/{id}", handleRepositoryGet(repositories))
	mux.HandleFunc("POST /api/repositories", handleRepositoryCreate(repositories))
	mux.HandleFunc("PUT /api/repositories/{id}", handleRepositoryUpdate(repositories))
	mux.HandleFunc("DELETE /api/repositories/{id}", handleRepositoryDelete(repositories))
}

// registerChurchRoutes регистрирует маршруты /api/churches на переданном mux.
func registerChurchRoutes(mux *http.ServeMux, churches ChurchService) {
	mux.HandleFunc("GET /api/churches", handleChurchList(churches))
	mux.HandleFunc("GET /api/churches/search", handleChurchSearch(churches))
	mux.HandleFunc("GET /api/churches/{id}", handleChurchGet(churches))
	mux.HandleFunc("POST /api/churches", handleChurchCreate(churches))
	mux.HandleFunc("PUT /api/churches/{id}", handleChurchUpdate(churches))
	mux.HandleFunc("DELETE /api/churches/{id}", handleChurchDelete(churches))
}

// registerParishRoutes регистрирует маршруты /api/parishes на переданном mux.
func registerParishRoutes(mux *http.ServeMux, parishes ParishService) {
	mux.HandleFunc("GET /api/parishes", handleParishList(parishes))
	mux.HandleFunc("GET /api/parishes/search", handleParishSearch(parishes))
	mux.HandleFunc("GET /api/parishes/{id}", handleParishGet(parishes))
	mux.HandleFunc("POST /api/parishes", handleParishCreate(parishes))
	mux.HandleFunc("PUT /api/parishes/{id}", handleParishUpdate(parishes))
	mux.HandleFunc("DELETE /api/parishes/{id}", handleParishDelete(parishes))
}

// registerArchiveRoutes регистрирует маршруты /api/archives на переданном mux.
func registerArchiveRoutes(mux *http.ServeMux, archives ArchiveService) {
	mux.HandleFunc("GET /api/archives", handleArchiveList(archives))
	mux.HandleFunc("GET /api/archives/search", handleArchiveSearch(archives))
	mux.HandleFunc("GET /api/archives/{id}", handleArchiveGet(archives))
	mux.HandleFunc("POST /api/archives", handleArchiveCreate(archives))
	mux.HandleFunc("PUT /api/archives/{id}", handleArchiveUpdate(archives))
	mux.HandleFunc("DELETE /api/archives/{id}", handleArchiveDelete(archives))
}

// registerArchiveNodeRoutes регистрирует маршруты /api/archive-nodes на
// переданном mux. archive_id обязателен для списка (см. parseArchiveNodeQuery).
func registerArchiveNodeRoutes(mux *http.ServeMux, archiveNodes ArchiveNodeService) {
	mux.HandleFunc("GET /api/archive-nodes", handleArchiveNodeList(archiveNodes))
	mux.HandleFunc("GET /api/archive-nodes/search", handleArchiveNodeSearch(archiveNodes))
	mux.HandleFunc("GET /api/archive-nodes/{id}", handleArchiveNodeGet(archiveNodes))
	mux.HandleFunc("POST /api/archive-nodes", handleArchiveNodeCreate(archiveNodes))
	mux.HandleFunc("PUT /api/archive-nodes/{id}", handleArchiveNodeUpdate(archiveNodes))
	mux.HandleFunc("DELETE /api/archive-nodes/{id}", handleArchiveNodeDelete(archiveNodes))
}

// registerArchiveDocumentRoutes регистрирует маршруты /api/archive-documents
// на переданном mux.
func registerArchiveDocumentRoutes(mux *http.ServeMux, archiveDocuments ArchiveDocumentService) {
	mux.HandleFunc("GET /api/archive-documents", handleArchiveDocumentList(archiveDocuments))
	mux.HandleFunc("GET /api/archive-documents/search", handleArchiveDocumentSearch(archiveDocuments))
	mux.HandleFunc("GET /api/archive-documents/{id}", handleArchiveDocumentGet(archiveDocuments))
	mux.HandleFunc("POST /api/archive-documents", handleArchiveDocumentCreate(archiveDocuments))
	mux.HandleFunc("PUT /api/archive-documents/{id}", handleArchiveDocumentUpdate(archiveDocuments))
	mux.HandleFunc("DELETE /api/archive-documents/{id}", handleArchiveDocumentDelete(archiveDocuments))
}

// registerNoteRoutes регистрирует маршруты /api/notes на переданном mux.
func registerNoteRoutes(mux *http.ServeMux, notes NoteService) {
	mux.HandleFunc("GET /api/notes", handleNoteList(notes))
	mux.HandleFunc("GET /api/notes/search", handleNoteSearch(notes))
	mux.HandleFunc("GET /api/notes/{id}", handleNoteGet(notes))
	mux.HandleFunc("POST /api/notes", handleNoteCreate(notes))
	mux.HandleFunc("PUT /api/notes/{id}", handleNoteUpdate(notes))
	mux.HandleFunc("DELETE /api/notes/{id}", handleNoteDelete(notes))
}

// registerAttachmentRoutes регистрирует маршруты /api/attachments на переданном mux.
func registerAttachmentRoutes(mux *http.ServeMux, attachments AttachmentService) {
	mux.HandleFunc("GET /api/attachments", handleAttachmentList(attachments))
	mux.HandleFunc("GET /api/attachments/search", handleAttachmentSearch(attachments))
	mux.HandleFunc("GET /api/attachments/{id}", handleAttachmentGet(attachments))
	mux.HandleFunc("POST /api/attachments", handleAttachmentCreate(attachments))
	mux.HandleFunc("PUT /api/attachments/{id}", handleAttachmentUpdate(attachments))
	mux.HandleFunc("DELETE /api/attachments/{id}", handleAttachmentDelete(attachments))
}

// registerSourceRoutes регистрирует маршруты /api/sources на переданном mux.
func registerSourceRoutes(mux *http.ServeMux, sources SourceService) {
	mux.HandleFunc("GET /api/sources", handleSourceList(sources))
	mux.HandleFunc("GET /api/sources/search", handleSourceSearch(sources))
	mux.HandleFunc("GET /api/sources/{id}", handleSourceGet(sources))
	mux.HandleFunc("POST /api/sources", handleSourceCreate(sources))
	mux.HandleFunc("PUT /api/sources/{id}", handleSourceUpdate(sources))
	mux.HandleFunc("DELETE /api/sources/{id}", handleSourceDelete(sources))
}

// registerCitationRoutes регистрирует маршруты /api/citations на переданном mux.
func registerCitationRoutes(mux *http.ServeMux, citations CitationService) {
	mux.HandleFunc("GET /api/citations", handleCitationList(citations))
	mux.HandleFunc("GET /api/citations/search", handleCitationSearch(citations))
	mux.HandleFunc("GET /api/citations/{id}", handleCitationGet(citations))
	mux.HandleFunc("POST /api/citations", handleCitationCreate(citations))
	mux.HandleFunc("PUT /api/citations/{id}", handleCitationUpdate(citations))
	mux.HandleFunc("DELETE /api/citations/{id}", handleCitationDelete(citations))
}

// registerFamilyRoutes регистрирует маршруты /api/families на переданном mux.
func registerFamilyRoutes(mux *http.ServeMux, families FamilyService) {
	mux.HandleFunc("GET /api/families", handleFamilyList(families))
	mux.HandleFunc("GET /api/families/search", handleFamilySearch(families))
	mux.HandleFunc("GET /api/families/{id}", handleFamilyGet(families))
	mux.HandleFunc("POST /api/families", handleFamilyCreate(families))
	mux.HandleFunc("PUT /api/families/{id}", handleFamilyUpdate(families))
	mux.HandleFunc("DELETE /api/families/{id}", handleFamilyDelete(families))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// pathID читает {id} из пути запроса — общий хелпер для всех сущностей
// (division.go's pathDivisionID — исторический синоним, оставлен как есть).
func pathID(r *http.Request) models.ID {
	return models.ID(strings.TrimPrefix(r.PathValue("id"), "/"))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError отвечает на ошибку сценария: *models.ValidationError — 422 с полем,
// models.ErrNotFound — 404, *models.InUseError — 409 с телом InUseErrorBody,
// остальное — 500.
func writeError(w http.ResponseWriter, err error) {
	var ve *models.ValidationError
	if errors.As(err, &ve) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": ve.Error(), "field": ve.Field})

		return
	}

	if errors.Is(err, models.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})

		return
	}

	var iu *models.InUseError
	if errors.As(err, &iu) {
		writeJSON(w, http.StatusConflict, transport.InUseErrorBodyFromModel(iu))

		return
	}

	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}
```
#### `internal/mcp/deps.go` (изменить — итоговое содержимое)
```go
package mcp

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

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

// SurnameService — контракт сценариев словарных записей фамилий, отдаваемых
// в MCP-тулы: список, поиск, чтение, создание, изменение, удаление.
type SurnameService interface {
	ListSurnames(ctx context.Context, access models.Access, page models.Page) ([]models.Surname, error)
	SearchSurnames(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Surname, error)
	GetSurname(ctx context.Context, id models.ID) (models.Surname, error)
	CreateSurname(ctx context.Context, sn models.Surname) (models.Surname, error)
	UpdateSurname(ctx context.Context, sn models.Surname) error
	DeleteSurname(ctx context.Context, id models.ID) error
}

// PatronymicService — контракт сценариев словарных записей (отчеств), отдаваемых
// в MCP-тулы: список, поиск, чтение, создание, изменение, удаление.
type PatronymicService interface {
	ListPatronymics(ctx context.Context, access models.Access, page models.Page) ([]models.Patronymic, error)
	SearchPatronymics(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Patronymic, error)
	GetPatronymic(ctx context.Context, id models.ID) (models.Patronymic, error)
	CreatePatronymic(ctx context.Context, x models.Patronymic) (models.Patronymic, error)
	UpdatePatronymic(ctx context.Context, x models.Patronymic) error
	DeletePatronymic(ctx context.Context, id models.ID) error
}

// EstateService — контракт сценариев словарных записей (сословий), отдаваемых
// в MCP-тулы: список, поиск, чтение, создание, изменение, удаление.
type EstateService interface {
	ListEstates(ctx context.Context, access models.Access, page models.Page) ([]models.Estate, error)
	SearchEstates(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Estate, error)
	GetEstate(ctx context.Context, id models.ID) (models.Estate, error)
	CreateEstate(ctx context.Context, x models.Estate) (models.Estate, error)
	UpdateEstate(ctx context.Context, x models.Estate) error
	DeleteEstate(ctx context.Context, id models.ID) error
}

// TitleService — контракт сценариев словарных записей (званий/титулов), отдаваемых
// в MCP-тулы: список, поиск, чтение, создание, изменение, удаление.
type TitleService interface {
	ListTitles(ctx context.Context, access models.Access, page models.Page) ([]models.Title, error)
	SearchTitles(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Title, error)
	GetTitle(ctx context.Context, id models.ID) (models.Title, error)
	CreateTitle(ctx context.Context, x models.Title) (models.Title, error)
	UpdateTitle(ctx context.Context, x models.Title) error
	DeleteTitle(ctx context.Context, id models.ID) error
}

// GivenNameService — контракт сценариев словарных записей имён, отдаваемых
// в MCP-тулы: список, поиск, чтение, создание, изменение, удаление.
type GivenNameService interface {
	ListGivenNames(ctx context.Context, access models.Access, page models.Page) ([]models.GivenName, error)
	SearchGivenNames(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.GivenName, error)
	GetGivenName(ctx context.Context, id models.ID) (models.GivenName, error)
	CreateGivenName(ctx context.Context, x models.GivenName) (models.GivenName, error)
	UpdateGivenName(ctx context.Context, x models.GivenName) error
	DeleteGivenName(ctx context.Context, id models.ID) error
}

// RepositoryService — контракт сценариев хранилищ-контейнеров источников,
// отдаваемых в MCP-тулы: список, поиск, чтение, создание, изменение, удаление.
type RepositoryService interface {
	ListRepositories(ctx context.Context, access models.Access, page models.Page) ([]models.Repository, error)
	SearchRepositories(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Repository, error)
	GetRepository(ctx context.Context, access models.Access, id models.ID) (models.Repository, error)
	CreateRepository(ctx context.Context, r models.Repository) (models.Repository, error)
	UpdateRepository(ctx context.Context, r models.Repository) error
	DeleteRepository(ctx context.Context, id models.ID) error
}

// ChurchService — контракт сценариев церквей, отдаваемых в MCP-тулы: список,
// поиск, чтение, создание, изменение, удаление.
type ChurchService interface {
	ListChurches(ctx context.Context, access models.Access, page models.Page) ([]models.Church, error)
	SearchChurches(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Church, error)
	GetChurch(ctx context.Context, id models.ID) (models.Church, error)
	CreateChurch(ctx context.Context, c models.Church) (models.Church, error)
	UpdateChurch(ctx context.Context, c models.Church) error
	DeleteChurch(ctx context.Context, id models.ID) error
}

// ParishService — контракт сценариев приходов, отдаваемых в MCP-тулы: список,
// поиск, чтение, создание, изменение, удаление.
type ParishService interface {
	ListParishes(ctx context.Context, access models.Access, page models.Page) ([]models.Parish, error)
	SearchParishes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Parish, error)
	GetParish(ctx context.Context, id models.ID) (models.Parish, error)
	CreateParish(ctx context.Context, p models.Parish) (models.Parish, error)
	UpdateParish(ctx context.Context, p models.Parish) error
	DeleteParish(ctx context.Context, id models.ID) error
}

// ArchiveService — контракт сценариев архивов, отдаваемых в MCP-тулы: список,
// поиск, чтение, создание, изменение, удаление.
type ArchiveService interface {
	ListArchives(ctx context.Context, access models.Access, page models.Page) ([]models.Archive, error)
	SearchArchives(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Archive, error)
	GetArchive(ctx context.Context, access models.Access, id models.ID) (models.Archive, error)
	CreateArchive(ctx context.Context, a models.Archive) (models.Archive, error)
	UpdateArchive(ctx context.Context, a models.Archive) error
	DeleteArchive(ctx context.Context, id models.ID) error
}

// ArchiveNodeService — контракт сценариев узлов архивного дерева, отдаваемых
// в MCP-тулы: список (обязательный archive_id), поиск, чтение, создание,
// изменение, удаление.
type ArchiveNodeService interface {
	ListArchiveNodes(ctx context.Context, access models.Access, q models.ArchiveNodeQuery) ([]models.ArchiveNode, error)
	SearchArchiveNodes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.ArchiveNode, error)
	GetArchiveNode(ctx context.Context, access models.Access, id models.ID) (models.ArchiveNode, error)
	CreateArchiveNode(ctx context.Context, n models.ArchiveNode) (models.ArchiveNode, error)
	UpdateArchiveNode(ctx context.Context, n models.ArchiveNode) error
	DeleteArchiveNode(ctx context.Context, id models.ID) error
}

// ArchiveDocumentService — контракт сценариев документов внутри единиц
// учёта, отдаваемых в MCP-тулы: список, поиск, чтение, создание, изменение,
// удаление.
type ArchiveDocumentService interface {
	ListArchiveDocuments(ctx context.Context, access models.Access, page models.Page) ([]models.ArchiveDocument, error)
	SearchArchiveDocuments(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.ArchiveDocument, error)
	GetArchiveDocument(ctx context.Context, access models.Access, id models.ID) (models.ArchiveDocument, error)
	CreateArchiveDocument(ctx context.Context, d models.ArchiveDocument) (models.ArchiveDocument, error)
	UpdateArchiveDocument(ctx context.Context, d models.ArchiveDocument) error
	DeleteArchiveDocument(ctx context.Context, id models.ID) error
}

// NoteService — контракт сценариев заметок, отдаваемых в MCP-тулы: список,
// поиск, чтение, создание, изменение, удаление.
type NoteService interface {
	ListNotes(ctx context.Context, access models.Access, page models.Page) ([]models.Note, error)
	SearchNotes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Note, error)
	GetNote(ctx context.Context, access models.Access, id models.ID) (models.Note, error)
	CreateNote(ctx context.Context, n models.Note) (models.Note, error)
	UpdateNote(ctx context.Context, n models.Note) error
	DeleteNote(ctx context.Context, id models.ID) error
}

// AttachmentService — контракт сценариев файловых вложений, отдаваемых в
// MCP-тулы: список, поиск, чтение, создание, изменение, удаление.
type AttachmentService interface {
	ListAttachments(ctx context.Context, access models.Access, page models.Page) ([]models.Attachment, error)
	SearchAttachments(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Attachment, error)
	GetAttachment(ctx context.Context, access models.Access, id models.ID) (models.Attachment, error)
	CreateAttachment(ctx context.Context, a models.Attachment) (models.Attachment, error)
	UpdateAttachment(ctx context.Context, a models.Attachment) error
	DeleteAttachment(ctx context.Context, id models.ID) error
}

// SourceService — контракт сценариев источников доказательств, отдаваемых в
// MCP-тулы: список, поиск, чтение, создание, изменение, удаление.
type SourceService interface {
	ListSources(ctx context.Context, access models.Access, page models.Page) ([]models.Source, error)
	SearchSources(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Source, error)
	GetSource(ctx context.Context, access models.Access, id models.ID) (models.Source, error)
	CreateSource(ctx context.Context, s models.Source) (models.Source, error)
	UpdateSource(ctx context.Context, s models.Source) error
	DeleteSource(ctx context.Context, id models.ID) error
}

// CitationService — контракт сценариев цитат, отдаваемых в MCP-тулы: список,
// поиск, чтение, создание, изменение, удаление.
type CitationService interface {
	ListCitations(ctx context.Context, access models.Access, page models.Page) ([]models.Citation, error)
	SearchCitations(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Citation, error)
	GetCitation(ctx context.Context, access models.Access, id models.ID) (models.Citation, error)
	CreateCitation(ctx context.Context, c models.Citation) (models.Citation, error)
	UpdateCitation(ctx context.Context, c models.Citation) error
	DeleteCitation(ctx context.Context, id models.ID) error
}

// FamilyService — контракт сценариев родов/линий, отдаваемых в MCP-тулы:
// список, поиск, чтение, создание, изменение, удаление.
type FamilyService interface {
	ListFamilies(ctx context.Context, access models.Access, page models.Page) ([]models.Family, error)
	SearchFamilies(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Family, error)
	GetFamily(ctx context.Context, access models.Access, id models.ID) (models.Family, error)
	CreateFamily(ctx context.Context, f models.Family) (models.Family, error)
	UpdateFamily(ctx context.Context, f models.Family) error
	DeleteFamily(ctx context.Context, id models.ID) error
}
```
#### `internal/mcp/server.go` (изменить — итоговое содержимое)
```go
package mcp

import (
	"github.com/mark3labs/mcp-go/server"
)

// Deps — сервисы, отдаваемые в MCP-тулы. Явный реестр вместо растущего
// списка позиционных параметров (docs/data-model/entity-write.md §3) — новая
// сущность добавляется полем структуры, не меняя сигнатуру NewServer.
type Deps struct {
	Divisions    DivisionService
	Surnames     SurnameService
	Patronymics  PatronymicService
	Estates      EstateService
	Titles       TitleService
	GivenNames   GivenNameService
	Repositories RepositoryService
	Churches     ChurchService
	Parishes     ParishService
	Archives     ArchiveService
	ArchiveNodes ArchiveNodeService
	ArchiveDocs  ArchiveDocumentService
	Notes        NoteService
	Attachments  AttachmentService
	Sources      SourceService
	Citations    CitationService
	Families     FamilyService
}

// NewServer создаёт MCP-сервер и регистрирует доступные тулы.
func NewServer(deps Deps) *server.MCPServer {
	s := server.NewMCPServer(
		"genodex",
		"0.1.0",
	)

	registerDivisionTools(s, deps.Divisions)

	if deps.Surnames != nil {
		registerSurnameTools(s, deps.Surnames)
	}

	if deps.Patronymics != nil {
		registerPatronymicTools(s, deps.Patronymics)
	}

	if deps.Estates != nil {
		registerEstateTools(s, deps.Estates)
	}

	if deps.Titles != nil {
		registerTitleTools(s, deps.Titles)
	}

	if deps.GivenNames != nil {
		registerGivenNameTools(s, deps.GivenNames)
	}

	if deps.Repositories != nil {
		registerRepositoryTools(s, deps.Repositories)
	}

	if deps.Churches != nil {
		registerChurchTools(s, deps.Churches)
	}

	if deps.Parishes != nil {
		registerParishTools(s, deps.Parishes)
	}

	if deps.Archives != nil {
		registerArchiveTools(s, deps.Archives)
	}

	if deps.ArchiveNodes != nil {
		registerArchiveNodeTools(s, deps.ArchiveNodes)
	}

	if deps.ArchiveDocs != nil {
		registerArchiveDocumentTools(s, deps.ArchiveDocs)
	}

	if deps.Notes != nil {
		registerNoteTools(s, deps.Notes)
	}

	if deps.Attachments != nil {
		registerAttachmentTools(s, deps.Attachments)
	}

	if deps.Sources != nil {
		registerSourceTools(s, deps.Sources)
	}

	if deps.Citations != nil {
		registerCitationTools(s, deps.Citations)
	}

	if deps.Families != nil {
		registerFamilyTools(s, deps.Families)
	}

	return s
}
```
#### `internal/app/app.go` (изменить — итоговое содержимое)
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
	create_archive "github.com/amarin/genodex/internal/usecases/create_archive"
	create_archive_document "github.com/amarin/genodex/internal/usecases/create_archive_document"
	create_archive_node "github.com/amarin/genodex/internal/usecases/create_archive_node"
	create_attachment "github.com/amarin/genodex/internal/usecases/create_attachment"
	create_church "github.com/amarin/genodex/internal/usecases/create_church"
	create_citation "github.com/amarin/genodex/internal/usecases/create_citation"
	create_division "github.com/amarin/genodex/internal/usecases/create_division"
	create_estate "github.com/amarin/genodex/internal/usecases/create_estate"
	create_family "github.com/amarin/genodex/internal/usecases/create_family"
	create_given_name "github.com/amarin/genodex/internal/usecases/create_given_name"
	create_note "github.com/amarin/genodex/internal/usecases/create_note"
	create_parish "github.com/amarin/genodex/internal/usecases/create_parish"
	create_patronymic "github.com/amarin/genodex/internal/usecases/create_patronymic"
	create_repository "github.com/amarin/genodex/internal/usecases/create_repository"
	create_source "github.com/amarin/genodex/internal/usecases/create_source"
	create_surname "github.com/amarin/genodex/internal/usecases/create_surname"
	create_title "github.com/amarin/genodex/internal/usecases/create_title"
	delete_archive "github.com/amarin/genodex/internal/usecases/delete_archive"
	delete_archive_document "github.com/amarin/genodex/internal/usecases/delete_archive_document"
	delete_archive_node "github.com/amarin/genodex/internal/usecases/delete_archive_node"
	delete_attachment "github.com/amarin/genodex/internal/usecases/delete_attachment"
	delete_church "github.com/amarin/genodex/internal/usecases/delete_church"
	delete_citation "github.com/amarin/genodex/internal/usecases/delete_citation"
	delete_division "github.com/amarin/genodex/internal/usecases/delete_division"
	delete_estate "github.com/amarin/genodex/internal/usecases/delete_estate"
	delete_family "github.com/amarin/genodex/internal/usecases/delete_family"
	delete_given_name "github.com/amarin/genodex/internal/usecases/delete_given_name"
	delete_note "github.com/amarin/genodex/internal/usecases/delete_note"
	delete_parish "github.com/amarin/genodex/internal/usecases/delete_parish"
	delete_patronymic "github.com/amarin/genodex/internal/usecases/delete_patronymic"
	delete_repository "github.com/amarin/genodex/internal/usecases/delete_repository"
	delete_source "github.com/amarin/genodex/internal/usecases/delete_source"
	delete_surname "github.com/amarin/genodex/internal/usecases/delete_surname"
	delete_title "github.com/amarin/genodex/internal/usecases/delete_title"
	get_archive "github.com/amarin/genodex/internal/usecases/get_archive"
	get_archive_document "github.com/amarin/genodex/internal/usecases/get_archive_document"
	get_archive_node "github.com/amarin/genodex/internal/usecases/get_archive_node"
	get_attachment "github.com/amarin/genodex/internal/usecases/get_attachment"
	get_church "github.com/amarin/genodex/internal/usecases/get_church"
	get_citation "github.com/amarin/genodex/internal/usecases/get_citation"
	get_division "github.com/amarin/genodex/internal/usecases/get_division"
	get_estate "github.com/amarin/genodex/internal/usecases/get_estate"
	get_family "github.com/amarin/genodex/internal/usecases/get_family"
	get_given_name "github.com/amarin/genodex/internal/usecases/get_given_name"
	get_note "github.com/amarin/genodex/internal/usecases/get_note"
	get_parish "github.com/amarin/genodex/internal/usecases/get_parish"
	get_patronymic "github.com/amarin/genodex/internal/usecases/get_patronymic"
	get_repository "github.com/amarin/genodex/internal/usecases/get_repository"
	get_source "github.com/amarin/genodex/internal/usecases/get_source"
	get_surname "github.com/amarin/genodex/internal/usecases/get_surname"
	get_title "github.com/amarin/genodex/internal/usecases/get_title"
	list_archive_documents "github.com/amarin/genodex/internal/usecases/list_archive_documents"
	list_archive_nodes "github.com/amarin/genodex/internal/usecases/list_archive_nodes"
	list_archives "github.com/amarin/genodex/internal/usecases/list_archives"
	list_attachments "github.com/amarin/genodex/internal/usecases/list_attachments"
	list_churches "github.com/amarin/genodex/internal/usecases/list_churches"
	list_citations "github.com/amarin/genodex/internal/usecases/list_citations"
	list_divisions "github.com/amarin/genodex/internal/usecases/list_divisions"
	list_estates "github.com/amarin/genodex/internal/usecases/list_estates"
	list_families "github.com/amarin/genodex/internal/usecases/list_families"
	list_given_names "github.com/amarin/genodex/internal/usecases/list_given_names"
	list_notes "github.com/amarin/genodex/internal/usecases/list_notes"
	list_parishes "github.com/amarin/genodex/internal/usecases/list_parishes"
	list_patronymics "github.com/amarin/genodex/internal/usecases/list_patronymics"
	list_repositories "github.com/amarin/genodex/internal/usecases/list_repositories"
	list_sources "github.com/amarin/genodex/internal/usecases/list_sources"
	list_surnames "github.com/amarin/genodex/internal/usecases/list_surnames"
	list_titles "github.com/amarin/genodex/internal/usecases/list_titles"
	search_archive_documents "github.com/amarin/genodex/internal/usecases/search_archive_documents"
	search_archive_nodes "github.com/amarin/genodex/internal/usecases/search_archive_nodes"
	search_archives "github.com/amarin/genodex/internal/usecases/search_archives"
	search_attachments "github.com/amarin/genodex/internal/usecases/search_attachments"
	search_churches "github.com/amarin/genodex/internal/usecases/search_churches"
	search_citations "github.com/amarin/genodex/internal/usecases/search_citations"
	search_divisions "github.com/amarin/genodex/internal/usecases/search_divisions"
	search_estates "github.com/amarin/genodex/internal/usecases/search_estates"
	search_families "github.com/amarin/genodex/internal/usecases/search_families"
	search_given_names "github.com/amarin/genodex/internal/usecases/search_given_names"
	search_notes "github.com/amarin/genodex/internal/usecases/search_notes"
	search_parishes "github.com/amarin/genodex/internal/usecases/search_parishes"
	search_patronymics "github.com/amarin/genodex/internal/usecases/search_patronymics"
	search_repositories "github.com/amarin/genodex/internal/usecases/search_repositories"
	search_sources "github.com/amarin/genodex/internal/usecases/search_sources"
	search_surnames "github.com/amarin/genodex/internal/usecases/search_surnames"
	search_titles "github.com/amarin/genodex/internal/usecases/search_titles"
	update_archive "github.com/amarin/genodex/internal/usecases/update_archive"
	update_archive_document "github.com/amarin/genodex/internal/usecases/update_archive_document"
	update_archive_node "github.com/amarin/genodex/internal/usecases/update_archive_node"
	update_attachment "github.com/amarin/genodex/internal/usecases/update_attachment"
	update_church "github.com/amarin/genodex/internal/usecases/update_church"
	update_citation "github.com/amarin/genodex/internal/usecases/update_citation"
	update_division "github.com/amarin/genodex/internal/usecases/update_division"
	update_estate "github.com/amarin/genodex/internal/usecases/update_estate"
	update_family "github.com/amarin/genodex/internal/usecases/update_family"
	update_given_name "github.com/amarin/genodex/internal/usecases/update_given_name"
	update_note "github.com/amarin/genodex/internal/usecases/update_note"
	update_parish "github.com/amarin/genodex/internal/usecases/update_parish"
	update_patronymic "github.com/amarin/genodex/internal/usecases/update_patronymic"
	update_repository "github.com/amarin/genodex/internal/usecases/update_repository"
	update_source "github.com/amarin/genodex/internal/usecases/update_source"
	update_surname "github.com/amarin/genodex/internal/usecases/update_surname"
	update_title "github.com/amarin/genodex/internal/usecases/update_title"
	"github.com/amarin/genodex/web"
)

// Config — параметры запуска приложения.
type Config struct {
	DataDir    string
	Port       int
	WebMode    string
	TrustProxy bool
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

// surnameService — фасад всех сценариев словарных записей фамилий, отдаваемых
// HTTP и MCP. Тот же приём, что divisionService — по одному полю на
// сценарий, тонкие методы-делегаты (docs/data-model/entity-write.md §3).
type surnameService struct {
	list   *list_surnames.Scenario
	search *search_surnames.Scenario
	get    *get_surname.Scenario
	create *create_surname.Scenario
	update *update_surname.Scenario
	del    *delete_surname.Scenario
}

func (s *surnameService) ListSurnames(ctx context.Context, access models.Access, page models.Page) ([]models.Surname, error) {
	return s.list.ListSurnames(ctx, access, page)
}

func (s *surnameService) SearchSurnames(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Surname, error) {
	return s.search.SearchSurnames(ctx, access, q)
}

func (s *surnameService) GetSurname(ctx context.Context, id models.ID) (models.Surname, error) {
	return s.get.GetSurname(ctx, id)
}

func (s *surnameService) CreateSurname(ctx context.Context, sn models.Surname) (models.Surname, error) {
	return s.create.CreateSurname(ctx, sn)
}

func (s *surnameService) UpdateSurname(ctx context.Context, sn models.Surname) error {
	return s.update.UpdateSurname(ctx, sn)
}

func (s *surnameService) DeleteSurname(ctx context.Context, id models.ID) error {
	return s.del.DeleteSurname(ctx, id)
}

// patronymicService — фасад всех сценариев словарных записей (отчеств), отдаваемых
// HTTP и MCP. Тот же приём, что divisionService/surnameService — по одному
// полю на сценарий, тонкие методы-делегаты (docs/data-model/entity-write.md §3).
type patronymicService struct {
	list   *list_patronymics.Scenario
	search *search_patronymics.Scenario
	get    *get_patronymic.Scenario
	create *create_patronymic.Scenario
	update *update_patronymic.Scenario
	del    *delete_patronymic.Scenario
}

func (s *patronymicService) ListPatronymics(ctx context.Context, access models.Access, page models.Page) ([]models.Patronymic, error) {
	return s.list.ListPatronymics(ctx, access, page)
}

func (s *patronymicService) SearchPatronymics(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Patronymic, error) {
	return s.search.SearchPatronymics(ctx, access, q)
}

func (s *patronymicService) GetPatronymic(ctx context.Context, id models.ID) (models.Patronymic, error) {
	return s.get.GetPatronymic(ctx, id)
}

func (s *patronymicService) CreatePatronymic(ctx context.Context, x models.Patronymic) (models.Patronymic, error) {
	return s.create.CreatePatronymic(ctx, x)
}

func (s *patronymicService) UpdatePatronymic(ctx context.Context, x models.Patronymic) error {
	return s.update.UpdatePatronymic(ctx, x)
}

func (s *patronymicService) DeletePatronymic(ctx context.Context, id models.ID) error {
	return s.del.DeletePatronymic(ctx, id)
}

// estateService — фасад всех сценариев словарных записей (сословий), отдаваемых
// HTTP и MCP. Тот же приём, что divisionService/surnameService — по одному
// полю на сценарий, тонкие методы-делегаты (docs/data-model/entity-write.md §3).
type estateService struct {
	list   *list_estates.Scenario
	search *search_estates.Scenario
	get    *get_estate.Scenario
	create *create_estate.Scenario
	update *update_estate.Scenario
	del    *delete_estate.Scenario
}

func (s *estateService) ListEstates(ctx context.Context, access models.Access, page models.Page) ([]models.Estate, error) {
	return s.list.ListEstates(ctx, access, page)
}

func (s *estateService) SearchEstates(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Estate, error) {
	return s.search.SearchEstates(ctx, access, q)
}

func (s *estateService) GetEstate(ctx context.Context, id models.ID) (models.Estate, error) {
	return s.get.GetEstate(ctx, id)
}

func (s *estateService) CreateEstate(ctx context.Context, x models.Estate) (models.Estate, error) {
	return s.create.CreateEstate(ctx, x)
}

func (s *estateService) UpdateEstate(ctx context.Context, x models.Estate) error {
	return s.update.UpdateEstate(ctx, x)
}

func (s *estateService) DeleteEstate(ctx context.Context, id models.ID) error {
	return s.del.DeleteEstate(ctx, id)
}

// titleService — фасад всех сценариев словарных записей (званий/титулов), отдаваемых
// HTTP и MCP. Тот же приём, что divisionService/surnameService — по одному
// полю на сценарий, тонкие методы-делегаты (docs/data-model/entity-write.md §3).
type titleService struct {
	list   *list_titles.Scenario
	search *search_titles.Scenario
	get    *get_title.Scenario
	create *create_title.Scenario
	update *update_title.Scenario
	del    *delete_title.Scenario
}

func (s *titleService) ListTitles(ctx context.Context, access models.Access, page models.Page) ([]models.Title, error) {
	return s.list.ListTitles(ctx, access, page)
}

func (s *titleService) SearchTitles(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Title, error) {
	return s.search.SearchTitles(ctx, access, q)
}

func (s *titleService) GetTitle(ctx context.Context, id models.ID) (models.Title, error) {
	return s.get.GetTitle(ctx, id)
}

func (s *titleService) CreateTitle(ctx context.Context, x models.Title) (models.Title, error) {
	return s.create.CreateTitle(ctx, x)
}

func (s *titleService) UpdateTitle(ctx context.Context, x models.Title) error {
	return s.update.UpdateTitle(ctx, x)
}

func (s *titleService) DeleteTitle(ctx context.Context, id models.ID) error {
	return s.del.DeleteTitle(ctx, id)
}

// givenNameService — фасад всех сценариев словарных записей (имён), отдаваемых
// HTTP и MCP. Тот же приём, что divisionService/surnameService — по одному
// полю на сценарий, тонкие методы-делегаты (docs/data-model/entity-write.md §3).
type givenNameService struct {
	list   *list_given_names.Scenario
	search *search_given_names.Scenario
	get    *get_given_name.Scenario
	create *create_given_name.Scenario
	update *update_given_name.Scenario
	del    *delete_given_name.Scenario
}

func (s *givenNameService) ListGivenNames(ctx context.Context, access models.Access, page models.Page) ([]models.GivenName, error) {
	return s.list.ListGivenNames(ctx, access, page)
}

func (s *givenNameService) SearchGivenNames(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.GivenName, error) {
	return s.search.SearchGivenNames(ctx, access, q)
}

func (s *givenNameService) GetGivenName(ctx context.Context, id models.ID) (models.GivenName, error) {
	return s.get.GetGivenName(ctx, id)
}

func (s *givenNameService) CreateGivenName(ctx context.Context, x models.GivenName) (models.GivenName, error) {
	return s.create.CreateGivenName(ctx, x)
}

func (s *givenNameService) UpdateGivenName(ctx context.Context, x models.GivenName) error {
	return s.update.UpdateGivenName(ctx, x)
}

func (s *givenNameService) DeleteGivenName(ctx context.Context, id models.ID) error {
	return s.del.DeleteGivenName(ctx, id)
}

// repositoryService — фасад всех сценариев хранилищ-контейнеров источников,
// отдаваемых HTTP и MCP. Тот же приём, что divisionService/surnameService —
// по одному полю на сценарий, тонкие методы-делегаты (docs/data-model/entity-write.md §3).
type repositoryService struct {
	list   *list_repositories.Scenario
	search *search_repositories.Scenario
	get    *get_repository.Scenario
	create *create_repository.Scenario
	update *update_repository.Scenario
	del    *delete_repository.Scenario
}

func (s *repositoryService) ListRepositories(ctx context.Context, access models.Access, page models.Page) ([]models.Repository, error) {
	return s.list.ListRepositories(ctx, access, page)
}

func (s *repositoryService) SearchRepositories(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Repository, error) {
	return s.search.SearchRepositories(ctx, access, q)
}

func (s *repositoryService) GetRepository(ctx context.Context, access models.Access, id models.ID) (models.Repository, error) {
	return s.get.GetRepository(ctx, access, id)
}

func (s *repositoryService) CreateRepository(ctx context.Context, r models.Repository) (models.Repository, error) {
	return s.create.CreateRepository(ctx, r)
}

func (s *repositoryService) UpdateRepository(ctx context.Context, r models.Repository) error {
	return s.update.UpdateRepository(ctx, r)
}

func (s *repositoryService) DeleteRepository(ctx context.Context, id models.ID) error {
	return s.del.DeleteRepository(ctx, id)
}

// churchService — фасад всех сценариев церквей, отдаваемых HTTP и MCP.
type churchService struct {
	list   *list_churches.Scenario
	search *search_churches.Scenario
	get    *get_church.Scenario
	create *create_church.Scenario
	update *update_church.Scenario
	del    *delete_church.Scenario
}

func (s *churchService) ListChurches(ctx context.Context, access models.Access, page models.Page) ([]models.Church, error) {
	return s.list.ListChurches(ctx, access, page)
}

func (s *churchService) SearchChurches(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Church, error) {
	return s.search.SearchChurches(ctx, access, q)
}

func (s *churchService) GetChurch(ctx context.Context, id models.ID) (models.Church, error) {
	return s.get.GetChurch(ctx, id)
}

func (s *churchService) CreateChurch(ctx context.Context, c models.Church) (models.Church, error) {
	return s.create.CreateChurch(ctx, c)
}

func (s *churchService) UpdateChurch(ctx context.Context, c models.Church) error {
	return s.update.UpdateChurch(ctx, c)
}

func (s *churchService) DeleteChurch(ctx context.Context, id models.ID) error {
	return s.del.DeleteChurch(ctx, id)
}

// parishService — фасад всех сценариев приходов, отдаваемых HTTP и MCP.
type parishService struct {
	list   *list_parishes.Scenario
	search *search_parishes.Scenario
	get    *get_parish.Scenario
	create *create_parish.Scenario
	update *update_parish.Scenario
	del    *delete_parish.Scenario
}

func (s *parishService) ListParishes(ctx context.Context, access models.Access, page models.Page) ([]models.Parish, error) {
	return s.list.ListParishes(ctx, access, page)
}

func (s *parishService) SearchParishes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Parish, error) {
	return s.search.SearchParishes(ctx, access, q)
}

func (s *parishService) GetParish(ctx context.Context, id models.ID) (models.Parish, error) {
	return s.get.GetParish(ctx, id)
}

func (s *parishService) CreateParish(ctx context.Context, p models.Parish) (models.Parish, error) {
	return s.create.CreateParish(ctx, p)
}

func (s *parishService) UpdateParish(ctx context.Context, p models.Parish) error {
	return s.update.UpdateParish(ctx, p)
}

func (s *parishService) DeleteParish(ctx context.Context, id models.ID) error {
	return s.del.DeleteParish(ctx, id)
}

// archiveService — фасад всех сценариев архивов, отдаваемых HTTP и MCP.
type archiveService struct {
	list   *list_archives.Scenario
	search *search_archives.Scenario
	get    *get_archive.Scenario
	create *create_archive.Scenario
	update *update_archive.Scenario
	del    *delete_archive.Scenario
}

func (s *archiveService) ListArchives(ctx context.Context, access models.Access, page models.Page) ([]models.Archive, error) {
	return s.list.ListArchives(ctx, access, page)
}

func (s *archiveService) SearchArchives(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Archive, error) {
	return s.search.SearchArchives(ctx, access, q)
}

func (s *archiveService) GetArchive(ctx context.Context, access models.Access, id models.ID) (models.Archive, error) {
	return s.get.GetArchive(ctx, access, id)
}

func (s *archiveService) CreateArchive(ctx context.Context, a models.Archive) (models.Archive, error) {
	return s.create.CreateArchive(ctx, a)
}

func (s *archiveService) UpdateArchive(ctx context.Context, a models.Archive) error {
	return s.update.UpdateArchive(ctx, a)
}

func (s *archiveService) DeleteArchive(ctx context.Context, id models.ID) error {
	return s.del.DeleteArchive(ctx, id)
}

// archiveNodeService — фасад всех сценариев узлов архивного дерева,
// отдаваемых HTTP и MCP.
type archiveNodeService struct {
	list   *list_archive_nodes.Scenario
	search *search_archive_nodes.Scenario
	get    *get_archive_node.Scenario
	create *create_archive_node.Scenario
	update *update_archive_node.Scenario
	del    *delete_archive_node.Scenario
}

func (s *archiveNodeService) ListArchiveNodes(ctx context.Context, access models.Access, q models.ArchiveNodeQuery) ([]models.ArchiveNode, error) {
	return s.list.ListArchiveNodes(ctx, access, q)
}

func (s *archiveNodeService) SearchArchiveNodes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.ArchiveNode, error) {
	return s.search.SearchArchiveNodes(ctx, access, q)
}

func (s *archiveNodeService) GetArchiveNode(ctx context.Context, access models.Access, id models.ID) (models.ArchiveNode, error) {
	return s.get.GetArchiveNode(ctx, access, id)
}

func (s *archiveNodeService) CreateArchiveNode(ctx context.Context, n models.ArchiveNode) (models.ArchiveNode, error) {
	return s.create.CreateArchiveNode(ctx, n)
}

func (s *archiveNodeService) UpdateArchiveNode(ctx context.Context, n models.ArchiveNode) error {
	return s.update.UpdateArchiveNode(ctx, n)
}

func (s *archiveNodeService) DeleteArchiveNode(ctx context.Context, id models.ID) error {
	return s.del.DeleteArchiveNode(ctx, id)
}

// archiveDocumentService — фасад всех сценариев документов внутри единиц
// учёта, отдаваемых HTTP и MCP.
type archiveDocumentService struct {
	list   *list_archive_documents.Scenario
	search *search_archive_documents.Scenario
	get    *get_archive_document.Scenario
	create *create_archive_document.Scenario
	update *update_archive_document.Scenario
	del    *delete_archive_document.Scenario
}

func (s *archiveDocumentService) ListArchiveDocuments(ctx context.Context, access models.Access, page models.Page) ([]models.ArchiveDocument, error) {
	return s.list.ListArchiveDocuments(ctx, access, page)
}

func (s *archiveDocumentService) SearchArchiveDocuments(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.ArchiveDocument, error) {
	return s.search.SearchArchiveDocuments(ctx, access, q)
}

func (s *archiveDocumentService) GetArchiveDocument(ctx context.Context, access models.Access, id models.ID) (models.ArchiveDocument, error) {
	return s.get.GetArchiveDocument(ctx, access, id)
}

func (s *archiveDocumentService) CreateArchiveDocument(ctx context.Context, d models.ArchiveDocument) (models.ArchiveDocument, error) {
	return s.create.CreateArchiveDocument(ctx, d)
}

func (s *archiveDocumentService) UpdateArchiveDocument(ctx context.Context, d models.ArchiveDocument) error {
	return s.update.UpdateArchiveDocument(ctx, d)
}

func (s *archiveDocumentService) DeleteArchiveDocument(ctx context.Context, id models.ID) error {
	return s.del.DeleteArchiveDocument(ctx, id)
}

// noteService — фасад всех сценариев заметок, отдаваемых HTTP и MCP.
type noteService struct {
	list   *list_notes.Scenario
	search *search_notes.Scenario
	get    *get_note.Scenario
	create *create_note.Scenario
	update *update_note.Scenario
	del    *delete_note.Scenario
}

func (s *noteService) ListNotes(ctx context.Context, access models.Access, page models.Page) ([]models.Note, error) {
	return s.list.ListNotes(ctx, access, page)
}

func (s *noteService) SearchNotes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Note, error) {
	return s.search.SearchNotes(ctx, access, q)
}

func (s *noteService) GetNote(ctx context.Context, access models.Access, id models.ID) (models.Note, error) {
	return s.get.GetNote(ctx, access, id)
}

func (s *noteService) CreateNote(ctx context.Context, n models.Note) (models.Note, error) {
	return s.create.CreateNote(ctx, n)
}

func (s *noteService) UpdateNote(ctx context.Context, n models.Note) error {
	return s.update.UpdateNote(ctx, n)
}

func (s *noteService) DeleteNote(ctx context.Context, id models.ID) error {
	return s.del.DeleteNote(ctx, id)
}

// attachmentService — фасад всех сценариев файловых вложений, отдаваемых
// HTTP и MCP.
type attachmentService struct {
	list   *list_attachments.Scenario
	search *search_attachments.Scenario
	get    *get_attachment.Scenario
	create *create_attachment.Scenario
	update *update_attachment.Scenario
	del    *delete_attachment.Scenario
}

func (s *attachmentService) ListAttachments(ctx context.Context, access models.Access, page models.Page) ([]models.Attachment, error) {
	return s.list.ListAttachments(ctx, access, page)
}

func (s *attachmentService) SearchAttachments(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Attachment, error) {
	return s.search.SearchAttachments(ctx, access, q)
}

func (s *attachmentService) GetAttachment(ctx context.Context, access models.Access, id models.ID) (models.Attachment, error) {
	return s.get.GetAttachment(ctx, access, id)
}

func (s *attachmentService) CreateAttachment(ctx context.Context, a models.Attachment) (models.Attachment, error) {
	return s.create.CreateAttachment(ctx, a)
}

func (s *attachmentService) UpdateAttachment(ctx context.Context, a models.Attachment) error {
	return s.update.UpdateAttachment(ctx, a)
}

func (s *attachmentService) DeleteAttachment(ctx context.Context, id models.ID) error {
	return s.del.DeleteAttachment(ctx, id)
}

// sourceService — фасад всех сценариев источников доказательств, отдаваемых
// HTTP и MCP.
type sourceService struct {
	list   *list_sources.Scenario
	search *search_sources.Scenario
	get    *get_source.Scenario
	create *create_source.Scenario
	update *update_source.Scenario
	del    *delete_source.Scenario
}

func (s *sourceService) ListSources(ctx context.Context, access models.Access, page models.Page) ([]models.Source, error) {
	return s.list.ListSources(ctx, access, page)
}

func (s *sourceService) SearchSources(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Source, error) {
	return s.search.SearchSources(ctx, access, q)
}

func (s *sourceService) GetSource(ctx context.Context, access models.Access, id models.ID) (models.Source, error) {
	return s.get.GetSource(ctx, access, id)
}

func (s *sourceService) CreateSource(ctx context.Context, src models.Source) (models.Source, error) {
	return s.create.CreateSource(ctx, src)
}

func (s *sourceService) UpdateSource(ctx context.Context, src models.Source) error {
	return s.update.UpdateSource(ctx, src)
}

func (s *sourceService) DeleteSource(ctx context.Context, id models.ID) error {
	return s.del.DeleteSource(ctx, id)
}

// citationService — фасад всех сценариев цитат, отдаваемых HTTP и MCP.
type citationService struct {
	list   *list_citations.Scenario
	search *search_citations.Scenario
	get    *get_citation.Scenario
	create *create_citation.Scenario
	update *update_citation.Scenario
	del    *delete_citation.Scenario
}

func (s *citationService) ListCitations(ctx context.Context, access models.Access, page models.Page) ([]models.Citation, error) {
	return s.list.ListCitations(ctx, access, page)
}

func (s *citationService) SearchCitations(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Citation, error) {
	return s.search.SearchCitations(ctx, access, q)
}

func (s *citationService) GetCitation(ctx context.Context, access models.Access, id models.ID) (models.Citation, error) {
	return s.get.GetCitation(ctx, access, id)
}

func (s *citationService) CreateCitation(ctx context.Context, c models.Citation) (models.Citation, error) {
	return s.create.CreateCitation(ctx, c)
}

func (s *citationService) UpdateCitation(ctx context.Context, c models.Citation) error {
	return s.update.UpdateCitation(ctx, c)
}

func (s *citationService) DeleteCitation(ctx context.Context, id models.ID) error {
	return s.del.DeleteCitation(ctx, id)
}

// familyService — фасад всех сценариев родов/линий, отдаваемых HTTP и MCP.
type familyService struct {
	list   *list_families.Scenario
	search *search_families.Scenario
	get    *get_family.Scenario
	create *create_family.Scenario
	update *update_family.Scenario
	del    *delete_family.Scenario
}

func (s *familyService) ListFamilies(ctx context.Context, access models.Access, page models.Page) ([]models.Family, error) {
	return s.list.ListFamilies(ctx, access, page)
}

func (s *familyService) SearchFamilies(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Family, error) {
	return s.search.SearchFamilies(ctx, access, q)
}

func (s *familyService) GetFamily(ctx context.Context, access models.Access, id models.ID) (models.Family, error) {
	return s.get.GetFamily(ctx, access, id)
}

func (s *familyService) CreateFamily(ctx context.Context, f models.Family) (models.Family, error) {
	return s.create.CreateFamily(ctx, f)
}

func (s *familyService) UpdateFamily(ctx context.Context, f models.Family) error {
	return s.update.UpdateFamily(ctx, f)
}

func (s *familyService) DeleteFamily(ctx context.Context, id models.ID) error {
	return s.del.DeleteFamily(ctx, id)
}

var (
	_ httpapi.DivisionService        = (*divisionService)(nil)
	_ mcp.DivisionService            = (*divisionService)(nil)
	_ httpapi.SurnameService         = (*surnameService)(nil)
	_ mcp.SurnameService             = (*surnameService)(nil)
	_ httpapi.PatronymicService      = (*patronymicService)(nil)
	_ mcp.PatronymicService          = (*patronymicService)(nil)
	_ httpapi.EstateService          = (*estateService)(nil)
	_ mcp.EstateService              = (*estateService)(nil)
	_ httpapi.TitleService           = (*titleService)(nil)
	_ mcp.TitleService               = (*titleService)(nil)
	_ httpapi.GivenNameService       = (*givenNameService)(nil)
	_ mcp.GivenNameService           = (*givenNameService)(nil)
	_ httpapi.RepositoryService      = (*repositoryService)(nil)
	_ mcp.RepositoryService          = (*repositoryService)(nil)
	_ httpapi.ChurchService          = (*churchService)(nil)
	_ mcp.ChurchService              = (*churchService)(nil)
	_ httpapi.ParishService          = (*parishService)(nil)
	_ mcp.ParishService              = (*parishService)(nil)
	_ httpapi.ArchiveService         = (*archiveService)(nil)
	_ mcp.ArchiveService             = (*archiveService)(nil)
	_ httpapi.ArchiveNodeService     = (*archiveNodeService)(nil)
	_ mcp.ArchiveNodeService         = (*archiveNodeService)(nil)
	_ httpapi.ArchiveDocumentService = (*archiveDocumentService)(nil)
	_ mcp.ArchiveDocumentService     = (*archiveDocumentService)(nil)
	_ httpapi.NoteService            = (*noteService)(nil)
	_ mcp.NoteService                = (*noteService)(nil)
	_ httpapi.AttachmentService      = (*attachmentService)(nil)
	_ mcp.AttachmentService          = (*attachmentService)(nil)
	_ httpapi.SourceService          = (*sourceService)(nil)
	_ mcp.SourceService              = (*sourceService)(nil)
	_ httpapi.CitationService        = (*citationService)(nil)
	_ mcp.CitationService            = (*citationService)(nil)
	_ httpapi.FamilyService          = (*familyService)(nil)
	_ mcp.FamilyService              = (*familyService)(nil)
	_ httpapi.AuthService            = (*auth.Service)(nil)
	_ mcp.TokenResolver              = (*auth.Service)(nil)
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

	surnames := &surnameService{
		list:   list_surnames.New(st),
		search: search_surnames.New(st),
		get:    get_surname.New(st),
		create: create_surname.New(st, idgen.New()),
		update: update_surname.New(st),
		del:    delete_surname.New(st),
	}

	patronymics := &patronymicService{
		list:   list_patronymics.New(st),
		search: search_patronymics.New(st),
		get:    get_patronymic.New(st),
		create: create_patronymic.New(st, idgen.New()),
		update: update_patronymic.New(st),
		del:    delete_patronymic.New(st),
	}

	estates := &estateService{
		list:   list_estates.New(st),
		search: search_estates.New(st),
		get:    get_estate.New(st),
		create: create_estate.New(st, idgen.New()),
		update: update_estate.New(st),
		del:    delete_estate.New(st),
	}

	titles := &titleService{
		list:   list_titles.New(st),
		search: search_titles.New(st),
		get:    get_title.New(st),
		create: create_title.New(st, idgen.New()),
		update: update_title.New(st),
		del:    delete_title.New(st),
	}

	givenNames := &givenNameService{
		list:   list_given_names.New(st),
		search: search_given_names.New(st),
		get:    get_given_name.New(st),
		create: create_given_name.New(st, idgen.New()),
		update: update_given_name.New(st),
		del:    delete_given_name.New(st),
	}

	repositories := &repositoryService{
		list:   list_repositories.New(st),
		search: search_repositories.New(st),
		get:    get_repository.New(st),
		create: create_repository.New(st, idgen.New()),
		update: update_repository.New(st),
		del:    delete_repository.New(st),
	}

	churches := &churchService{
		list:   list_churches.New(st),
		search: search_churches.New(st),
		get:    get_church.New(st),
		create: create_church.New(st, idgen.New()),
		update: update_church.New(st),
		del:    delete_church.New(st),
	}

	parishes := &parishService{
		list:   list_parishes.New(st),
		search: search_parishes.New(st),
		get:    get_parish.New(st),
		create: create_parish.New(st, idgen.New()),
		update: update_parish.New(st),
		del:    delete_parish.New(st),
	}

	archives := &archiveService{
		list:   list_archives.New(st),
		search: search_archives.New(st),
		get:    get_archive.New(st),
		create: create_archive.New(st, idgen.New()),
		update: update_archive.New(st),
		del:    delete_archive.New(st),
	}

	archiveNodes := &archiveNodeService{
		list:   list_archive_nodes.New(st),
		search: search_archive_nodes.New(st),
		get:    get_archive_node.New(st),
		create: create_archive_node.New(st, idgen.New()),
		update: update_archive_node.New(st),
		del:    delete_archive_node.New(st),
	}

	archiveDocs := &archiveDocumentService{
		list:   list_archive_documents.New(st),
		search: search_archive_documents.New(st),
		get:    get_archive_document.New(st),
		create: create_archive_document.New(st, idgen.New()),
		update: update_archive_document.New(st),
		del:    delete_archive_document.New(st),
	}

	notes := &noteService{
		list:   list_notes.New(st),
		search: search_notes.New(st),
		get:    get_note.New(st),
		create: create_note.New(st, idgen.New()),
		update: update_note.New(st),
		del:    delete_note.New(st),
	}

	attachments := &attachmentService{
		list:   list_attachments.New(st),
		search: search_attachments.New(st),
		get:    get_attachment.New(st),
		create: create_attachment.New(st, idgen.New()),
		update: update_attachment.New(st),
		del:    delete_attachment.New(st),
	}

	sources := &sourceService{
		list:   list_sources.New(st),
		search: search_sources.New(st),
		get:    get_source.New(st),
		create: create_source.New(st, idgen.New()),
		update: update_source.New(st),
		del:    delete_source.New(st),
	}

	citations := &citationService{
		list:   list_citations.New(st),
		search: search_citations.New(st),
		get:    get_citation.New(st),
		create: create_citation.New(st, idgen.New()),
		update: update_citation.New(st),
		del:    delete_citation.New(st),
	}

	families := &familyService{
		list:   list_families.New(st),
		search: search_families.New(st),
		get:    get_family.New(st),
		create: create_family.New(st, idgen.New()),
		update: update_family.New(st),
		del:    delete_family.New(st),
	}

	// auth-хранилище — на том же соединении, что и общий store (см.
	// sqlstore.Store.DB), файл БД один и тот же (internal/storage/schema_auth.go).
	authService := auth.New(auth.NewSQLStore(st.DB()))

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcp.RequireAPIToken(authService)(server.NewStreamableHTTPServer(
		mcp.NewServer(mcp.Deps{
			Divisions: divisions, Surnames: surnames,
			Patronymics: patronymics, Estates: estates, Titles: titles, GivenNames: givenNames,
			Repositories: repositories, Churches: churches, Parishes: parishes, Archives: archives,
			ArchiveNodes: archiveNodes,
			ArchiveDocs:  archiveDocs,
			Notes:        notes,
			Attachments:  attachments,
			Sources:      sources,
			Citations:    citations,
			Families:     families,
		}),
	)))
	mux.Handle("/api/", httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:    divisions,
		Surnames:     surnames,
		Patronymics:  patronymics,
		Estates:      estates,
		Titles:       titles,
		GivenNames:   givenNames,
		Repositories: repositories,
		Churches:     churches,
		Parishes:     parishes,
		Archives:     archives,
		ArchiveNodes: archiveNodes,
		ArchiveDocs:  archiveDocs,
		Notes:        notes,
		Attachments:  attachments,
		Sources:      sources,
		Citations:    citations,
		Families:     families,
		Auth:         authService,
		DocsFS:       genodex.DocsFS(cfg.WebMode),
		TrustProxy:   cfg.TrustProxy,
	}))
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
	log.Printf("  Trust proxy (X-Forwarded-Proto): %v", a.cfg.TrustProxy)

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

### 1.6. Real-store интеграционные тесты

#### `internal/httpapi/store_test.go` (изменить — итоговое содержимое)
```go
package httpapi_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/httpapi"
	"github.com/amarin/genodex/internal/idgen"
	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store/sqlstore"
	create_archive "github.com/amarin/genodex/internal/usecases/create_archive"
	create_archive_document "github.com/amarin/genodex/internal/usecases/create_archive_document"
	create_archive_node "github.com/amarin/genodex/internal/usecases/create_archive_node"
	create_attachment "github.com/amarin/genodex/internal/usecases/create_attachment"
	create_church "github.com/amarin/genodex/internal/usecases/create_church"
	create_citation "github.com/amarin/genodex/internal/usecases/create_citation"
	create_division "github.com/amarin/genodex/internal/usecases/create_division"
	create_estate "github.com/amarin/genodex/internal/usecases/create_estate"
	create_family "github.com/amarin/genodex/internal/usecases/create_family"
	create_given_name "github.com/amarin/genodex/internal/usecases/create_given_name"
	create_note "github.com/amarin/genodex/internal/usecases/create_note"
	create_parish "github.com/amarin/genodex/internal/usecases/create_parish"
	create_patronymic "github.com/amarin/genodex/internal/usecases/create_patronymic"
	create_repository "github.com/amarin/genodex/internal/usecases/create_repository"
	create_source "github.com/amarin/genodex/internal/usecases/create_source"
	create_surname "github.com/amarin/genodex/internal/usecases/create_surname"
	create_title "github.com/amarin/genodex/internal/usecases/create_title"
	delete_archive "github.com/amarin/genodex/internal/usecases/delete_archive"
	delete_archive_document "github.com/amarin/genodex/internal/usecases/delete_archive_document"
	delete_archive_node "github.com/amarin/genodex/internal/usecases/delete_archive_node"
	delete_attachment "github.com/amarin/genodex/internal/usecases/delete_attachment"
	delete_church "github.com/amarin/genodex/internal/usecases/delete_church"
	delete_citation "github.com/amarin/genodex/internal/usecases/delete_citation"
	delete_division "github.com/amarin/genodex/internal/usecases/delete_division"
	delete_estate "github.com/amarin/genodex/internal/usecases/delete_estate"
	delete_family "github.com/amarin/genodex/internal/usecases/delete_family"
	delete_given_name "github.com/amarin/genodex/internal/usecases/delete_given_name"
	delete_note "github.com/amarin/genodex/internal/usecases/delete_note"
	delete_parish "github.com/amarin/genodex/internal/usecases/delete_parish"
	delete_patronymic "github.com/amarin/genodex/internal/usecases/delete_patronymic"
	delete_repository "github.com/amarin/genodex/internal/usecases/delete_repository"
	delete_source "github.com/amarin/genodex/internal/usecases/delete_source"
	delete_surname "github.com/amarin/genodex/internal/usecases/delete_surname"
	delete_title "github.com/amarin/genodex/internal/usecases/delete_title"
	get_archive "github.com/amarin/genodex/internal/usecases/get_archive"
	get_archive_document "github.com/amarin/genodex/internal/usecases/get_archive_document"
	get_archive_node "github.com/amarin/genodex/internal/usecases/get_archive_node"
	get_attachment "github.com/amarin/genodex/internal/usecases/get_attachment"
	get_church "github.com/amarin/genodex/internal/usecases/get_church"
	get_citation "github.com/amarin/genodex/internal/usecases/get_citation"
	get_division "github.com/amarin/genodex/internal/usecases/get_division"
	get_estate "github.com/amarin/genodex/internal/usecases/get_estate"
	get_family "github.com/amarin/genodex/internal/usecases/get_family"
	get_given_name "github.com/amarin/genodex/internal/usecases/get_given_name"
	get_note "github.com/amarin/genodex/internal/usecases/get_note"
	get_parish "github.com/amarin/genodex/internal/usecases/get_parish"
	get_patronymic "github.com/amarin/genodex/internal/usecases/get_patronymic"
	get_repository "github.com/amarin/genodex/internal/usecases/get_repository"
	get_source "github.com/amarin/genodex/internal/usecases/get_source"
	get_surname "github.com/amarin/genodex/internal/usecases/get_surname"
	get_title "github.com/amarin/genodex/internal/usecases/get_title"
	list_archive_documents "github.com/amarin/genodex/internal/usecases/list_archive_documents"
	list_archive_nodes "github.com/amarin/genodex/internal/usecases/list_archive_nodes"
	list_archives "github.com/amarin/genodex/internal/usecases/list_archives"
	list_attachments "github.com/amarin/genodex/internal/usecases/list_attachments"
	list_churches "github.com/amarin/genodex/internal/usecases/list_churches"
	list_citations "github.com/amarin/genodex/internal/usecases/list_citations"
	list_divisions "github.com/amarin/genodex/internal/usecases/list_divisions"
	list_estates "github.com/amarin/genodex/internal/usecases/list_estates"
	list_families "github.com/amarin/genodex/internal/usecases/list_families"
	list_given_names "github.com/amarin/genodex/internal/usecases/list_given_names"
	list_notes "github.com/amarin/genodex/internal/usecases/list_notes"
	list_parishes "github.com/amarin/genodex/internal/usecases/list_parishes"
	list_patronymics "github.com/amarin/genodex/internal/usecases/list_patronymics"
	list_repositories "github.com/amarin/genodex/internal/usecases/list_repositories"
	list_sources "github.com/amarin/genodex/internal/usecases/list_sources"
	list_surnames "github.com/amarin/genodex/internal/usecases/list_surnames"
	list_titles "github.com/amarin/genodex/internal/usecases/list_titles"
	search_archive_documents "github.com/amarin/genodex/internal/usecases/search_archive_documents"
	search_archive_nodes "github.com/amarin/genodex/internal/usecases/search_archive_nodes"
	search_archives "github.com/amarin/genodex/internal/usecases/search_archives"
	search_attachments "github.com/amarin/genodex/internal/usecases/search_attachments"
	search_churches "github.com/amarin/genodex/internal/usecases/search_churches"
	search_citations "github.com/amarin/genodex/internal/usecases/search_citations"
	search_divisions "github.com/amarin/genodex/internal/usecases/search_divisions"
	search_estates "github.com/amarin/genodex/internal/usecases/search_estates"
	search_families "github.com/amarin/genodex/internal/usecases/search_families"
	search_given_names "github.com/amarin/genodex/internal/usecases/search_given_names"
	search_notes "github.com/amarin/genodex/internal/usecases/search_notes"
	search_parishes "github.com/amarin/genodex/internal/usecases/search_parishes"
	search_patronymics "github.com/amarin/genodex/internal/usecases/search_patronymics"
	search_repositories "github.com/amarin/genodex/internal/usecases/search_repositories"
	search_sources "github.com/amarin/genodex/internal/usecases/search_sources"
	search_surnames "github.com/amarin/genodex/internal/usecases/search_surnames"
	search_titles "github.com/amarin/genodex/internal/usecases/search_titles"
	update_archive "github.com/amarin/genodex/internal/usecases/update_archive"
	update_archive_document "github.com/amarin/genodex/internal/usecases/update_archive_document"
	update_archive_node "github.com/amarin/genodex/internal/usecases/update_archive_node"
	update_attachment "github.com/amarin/genodex/internal/usecases/update_attachment"
	update_church "github.com/amarin/genodex/internal/usecases/update_church"
	update_citation "github.com/amarin/genodex/internal/usecases/update_citation"
	update_division "github.com/amarin/genodex/internal/usecases/update_division"
	update_estate "github.com/amarin/genodex/internal/usecases/update_estate"
	update_family "github.com/amarin/genodex/internal/usecases/update_family"
	update_given_name "github.com/amarin/genodex/internal/usecases/update_given_name"
	update_note "github.com/amarin/genodex/internal/usecases/update_note"
	update_parish "github.com/amarin/genodex/internal/usecases/update_parish"
	update_patronymic "github.com/amarin/genodex/internal/usecases/update_patronymic"
	update_repository "github.com/amarin/genodex/internal/usecases/update_repository"
	update_source "github.com/amarin/genodex/internal/usecases/update_source"
	update_surname "github.com/amarin/genodex/internal/usecases/update_surname"
	update_title "github.com/amarin/genodex/internal/usecases/update_title"
)

// divisionService — сборка httpapi.DivisionService на настоящих сценариях
// (так же собран internal/app).
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

// newDivisionService собирает фасад на настоящем хранилище.
func newDivisionService(t *testing.T, st *sqlstore.Store) *divisionService {
	t.Helper()

	return &divisionService{
		list:   list_divisions.New(st),
		search: search_divisions.New(st),
		get:    get_division.New(st),
		create: create_division.New(st, idgen.New()),
		update: update_division.New(st),
		del:    delete_division.New(st),
	}
}

// surnameService — сборка httpapi.SurnameService на настоящих сценариях
// (так же собран internal/app's surnameService).
type surnameService struct {
	list   *list_surnames.Scenario
	search *search_surnames.Scenario
	get    *get_surname.Scenario
	create *create_surname.Scenario
	update *update_surname.Scenario
	del    *delete_surname.Scenario
}

func (s *surnameService) ListSurnames(ctx context.Context, access models.Access, page models.Page) ([]models.Surname, error) {
	return s.list.ListSurnames(ctx, access, page)
}

func (s *surnameService) SearchSurnames(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Surname, error) {
	return s.search.SearchSurnames(ctx, access, q)
}

func (s *surnameService) GetSurname(ctx context.Context, id models.ID) (models.Surname, error) {
	return s.get.GetSurname(ctx, id)
}

func (s *surnameService) CreateSurname(ctx context.Context, sn models.Surname) (models.Surname, error) {
	return s.create.CreateSurname(ctx, sn)
}

func (s *surnameService) UpdateSurname(ctx context.Context, sn models.Surname) error {
	return s.update.UpdateSurname(ctx, sn)
}

func (s *surnameService) DeleteSurname(ctx context.Context, id models.ID) error {
	return s.del.DeleteSurname(ctx, id)
}

// newSurnameService собирает фасад на настоящем хранилище.
func newSurnameService(t *testing.T, st *sqlstore.Store) *surnameService {
	t.Helper()

	return &surnameService{
		list:   list_surnames.New(st),
		search: search_surnames.New(st),
		get:    get_surname.New(st),
		create: create_surname.New(st, idgen.New()),
		update: update_surname.New(st),
		del:    delete_surname.New(st),
	}
}

// patronymicService — сборка httpapi.PatronymicService на настоящих сценариях
// (так же собран internal/app's patronymicService).
type patronymicService struct {
	list   *list_patronymics.Scenario
	search *search_patronymics.Scenario
	get    *get_patronymic.Scenario
	create *create_patronymic.Scenario
	update *update_patronymic.Scenario
	del    *delete_patronymic.Scenario
}

func (s *patronymicService) ListPatronymics(ctx context.Context, access models.Access, page models.Page) ([]models.Patronymic, error) {
	return s.list.ListPatronymics(ctx, access, page)
}

func (s *patronymicService) SearchPatronymics(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Patronymic, error) {
	return s.search.SearchPatronymics(ctx, access, q)
}

func (s *patronymicService) GetPatronymic(ctx context.Context, id models.ID) (models.Patronymic, error) {
	return s.get.GetPatronymic(ctx, id)
}

func (s *patronymicService) CreatePatronymic(ctx context.Context, x models.Patronymic) (models.Patronymic, error) {
	return s.create.CreatePatronymic(ctx, x)
}

func (s *patronymicService) UpdatePatronymic(ctx context.Context, x models.Patronymic) error {
	return s.update.UpdatePatronymic(ctx, x)
}

func (s *patronymicService) DeletePatronymic(ctx context.Context, id models.ID) error {
	return s.del.DeletePatronymic(ctx, id)
}

// newPatronymicService собирает фасад на настоящем хранилище.
func newPatronymicService(t *testing.T, st *sqlstore.Store) *patronymicService {
	t.Helper()

	return &patronymicService{
		list:   list_patronymics.New(st),
		search: search_patronymics.New(st),
		get:    get_patronymic.New(st),
		create: create_patronymic.New(st, idgen.New()),
		update: update_patronymic.New(st),
		del:    delete_patronymic.New(st),
	}
}

// estateService — сборка httpapi.EstateService на настоящих сценариях
// (так же собран internal/app's estateService).
type estateService struct {
	list   *list_estates.Scenario
	search *search_estates.Scenario
	get    *get_estate.Scenario
	create *create_estate.Scenario
	update *update_estate.Scenario
	del    *delete_estate.Scenario
}

func (s *estateService) ListEstates(ctx context.Context, access models.Access, page models.Page) ([]models.Estate, error) {
	return s.list.ListEstates(ctx, access, page)
}

func (s *estateService) SearchEstates(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Estate, error) {
	return s.search.SearchEstates(ctx, access, q)
}

func (s *estateService) GetEstate(ctx context.Context, id models.ID) (models.Estate, error) {
	return s.get.GetEstate(ctx, id)
}

func (s *estateService) CreateEstate(ctx context.Context, x models.Estate) (models.Estate, error) {
	return s.create.CreateEstate(ctx, x)
}

func (s *estateService) UpdateEstate(ctx context.Context, x models.Estate) error {
	return s.update.UpdateEstate(ctx, x)
}

func (s *estateService) DeleteEstate(ctx context.Context, id models.ID) error {
	return s.del.DeleteEstate(ctx, id)
}

// newEstateService собирает фасад на настоящем хранилище.
func newEstateService(t *testing.T, st *sqlstore.Store) *estateService {
	t.Helper()

	return &estateService{
		list:   list_estates.New(st),
		search: search_estates.New(st),
		get:    get_estate.New(st),
		create: create_estate.New(st, idgen.New()),
		update: update_estate.New(st),
		del:    delete_estate.New(st),
	}
}

// titleService — сборка httpapi.TitleService на настоящих сценариях
// (так же собран internal/app's titleService).
type titleService struct {
	list   *list_titles.Scenario
	search *search_titles.Scenario
	get    *get_title.Scenario
	create *create_title.Scenario
	update *update_title.Scenario
	del    *delete_title.Scenario
}

func (s *titleService) ListTitles(ctx context.Context, access models.Access, page models.Page) ([]models.Title, error) {
	return s.list.ListTitles(ctx, access, page)
}

func (s *titleService) SearchTitles(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Title, error) {
	return s.search.SearchTitles(ctx, access, q)
}

func (s *titleService) GetTitle(ctx context.Context, id models.ID) (models.Title, error) {
	return s.get.GetTitle(ctx, id)
}

func (s *titleService) CreateTitle(ctx context.Context, x models.Title) (models.Title, error) {
	return s.create.CreateTitle(ctx, x)
}

func (s *titleService) UpdateTitle(ctx context.Context, x models.Title) error {
	return s.update.UpdateTitle(ctx, x)
}

func (s *titleService) DeleteTitle(ctx context.Context, id models.ID) error {
	return s.del.DeleteTitle(ctx, id)
}

// newTitleService собирает фасад на настоящем хранилище.
func newTitleService(t *testing.T, st *sqlstore.Store) *titleService {
	t.Helper()

	return &titleService{
		list:   list_titles.New(st),
		search: search_titles.New(st),
		get:    get_title.New(st),
		create: create_title.New(st, idgen.New()),
		update: update_title.New(st),
		del:    delete_title.New(st),
	}
}

// givenNameService — сборка httpapi.GivenNameService на настоящих сценариях
// (так же собран internal/app's givenNameService).
type givenNameService struct {
	list   *list_given_names.Scenario
	search *search_given_names.Scenario
	get    *get_given_name.Scenario
	create *create_given_name.Scenario
	update *update_given_name.Scenario
	del    *delete_given_name.Scenario
}

func (s *givenNameService) ListGivenNames(ctx context.Context, access models.Access, page models.Page) ([]models.GivenName, error) {
	return s.list.ListGivenNames(ctx, access, page)
}

func (s *givenNameService) SearchGivenNames(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.GivenName, error) {
	return s.search.SearchGivenNames(ctx, access, q)
}

func (s *givenNameService) GetGivenName(ctx context.Context, id models.ID) (models.GivenName, error) {
	return s.get.GetGivenName(ctx, id)
}

func (s *givenNameService) CreateGivenName(ctx context.Context, x models.GivenName) (models.GivenName, error) {
	return s.create.CreateGivenName(ctx, x)
}

func (s *givenNameService) UpdateGivenName(ctx context.Context, x models.GivenName) error {
	return s.update.UpdateGivenName(ctx, x)
}

func (s *givenNameService) DeleteGivenName(ctx context.Context, id models.ID) error {
	return s.del.DeleteGivenName(ctx, id)
}

// newGivenNameService собирает фасад на настоящем хранилище.
func newGivenNameService(t *testing.T, st *sqlstore.Store) *givenNameService {
	t.Helper()

	return &givenNameService{
		list:   list_given_names.New(st),
		search: search_given_names.New(st),
		get:    get_given_name.New(st),
		create: create_given_name.New(st, idgen.New()),
		update: update_given_name.New(st),
		del:    delete_given_name.New(st),
	}
}

// familyService — фасад httpapi.FamilyService на настоящих сценариях (так же
// собран internal/app's familyService).
type familyService struct {
	list   *list_families.Scenario
	search *search_families.Scenario
	get    *get_family.Scenario
	create *create_family.Scenario
	update *update_family.Scenario
	del    *delete_family.Scenario
}

func (s *familyService) ListFamilies(ctx context.Context, access models.Access, page models.Page) ([]models.Family, error) {
	return s.list.ListFamilies(ctx, access, page)
}

func (s *familyService) SearchFamilies(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Family, error) {
	return s.search.SearchFamilies(ctx, access, q)
}

func (s *familyService) GetFamily(ctx context.Context, access models.Access, id models.ID) (models.Family, error) {
	return s.get.GetFamily(ctx, access, id)
}

func (s *familyService) CreateFamily(ctx context.Context, f models.Family) (models.Family, error) {
	return s.create.CreateFamily(ctx, f)
}

func (s *familyService) UpdateFamily(ctx context.Context, f models.Family) error {
	return s.update.UpdateFamily(ctx, f)
}

func (s *familyService) DeleteFamily(ctx context.Context, id models.ID) error {
	return s.del.DeleteFamily(ctx, id)
}

// newFamilyService собирает фасад на настоящем хранилище.
func newFamilyService(t *testing.T, st *sqlstore.Store) *familyService {
	t.Helper()

	return &familyService{
		list:   list_families.New(st),
		search: search_families.New(st),
		get:    get_family.New(st),
		create: create_family.New(st, idgen.New()),
		update: update_family.New(st),
		del:    delete_family.New(st),
	}
}

// repositoryService — фасад httpapi.RepositoryService на настоящих сценариях
// (так же собран internal/app's repositoryService).
type repositoryService struct {
	list   *list_repositories.Scenario
	search *search_repositories.Scenario
	get    *get_repository.Scenario
	create *create_repository.Scenario
	update *update_repository.Scenario
	del    *delete_repository.Scenario
}

func (s *repositoryService) ListRepositories(ctx context.Context, access models.Access, page models.Page) ([]models.Repository, error) {
	return s.list.ListRepositories(ctx, access, page)
}

func (s *repositoryService) SearchRepositories(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Repository, error) {
	return s.search.SearchRepositories(ctx, access, q)
}

func (s *repositoryService) GetRepository(ctx context.Context, access models.Access, id models.ID) (models.Repository, error) {
	return s.get.GetRepository(ctx, access, id)
}

func (s *repositoryService) CreateRepository(ctx context.Context, r models.Repository) (models.Repository, error) {
	return s.create.CreateRepository(ctx, r)
}

func (s *repositoryService) UpdateRepository(ctx context.Context, r models.Repository) error {
	return s.update.UpdateRepository(ctx, r)
}

func (s *repositoryService) DeleteRepository(ctx context.Context, id models.ID) error {
	return s.del.DeleteRepository(ctx, id)
}

// newRepositoryService собирает фасад на настоящем хранилище.
func newRepositoryService(t *testing.T, st *sqlstore.Store) *repositoryService {
	t.Helper()

	return &repositoryService{
		list:   list_repositories.New(st),
		search: search_repositories.New(st),
		get:    get_repository.New(st),
		create: create_repository.New(st, idgen.New()),
		update: update_repository.New(st),
		del:    delete_repository.New(st),
	}
}

// churchService — фасад httpapi.ChurchService на настоящих сценариях (так же
// собран internal/app's churchService).
type churchService struct {
	list   *list_churches.Scenario
	search *search_churches.Scenario
	get    *get_church.Scenario
	create *create_church.Scenario
	update *update_church.Scenario
	del    *delete_church.Scenario
}

func (s *churchService) ListChurches(ctx context.Context, access models.Access, page models.Page) ([]models.Church, error) {
	return s.list.ListChurches(ctx, access, page)
}

func (s *churchService) SearchChurches(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Church, error) {
	return s.search.SearchChurches(ctx, access, q)
}

func (s *churchService) GetChurch(ctx context.Context, id models.ID) (models.Church, error) {
	return s.get.GetChurch(ctx, id)
}

func (s *churchService) CreateChurch(ctx context.Context, c models.Church) (models.Church, error) {
	return s.create.CreateChurch(ctx, c)
}

func (s *churchService) UpdateChurch(ctx context.Context, c models.Church) error {
	return s.update.UpdateChurch(ctx, c)
}

func (s *churchService) DeleteChurch(ctx context.Context, id models.ID) error {
	return s.del.DeleteChurch(ctx, id)
}

// newChurchService собирает фасад на настоящем хранилище.
func newChurchService(t *testing.T, st *sqlstore.Store) *churchService {
	t.Helper()

	return &churchService{
		list:   list_churches.New(st),
		search: search_churches.New(st),
		get:    get_church.New(st),
		create: create_church.New(st, idgen.New()),
		update: update_church.New(st),
		del:    delete_church.New(st),
	}
}

// parishService — фасад httpapi.ParishService на настоящих сценариях (так же
// собран internal/app's parishService).
type parishService struct {
	list   *list_parishes.Scenario
	search *search_parishes.Scenario
	get    *get_parish.Scenario
	create *create_parish.Scenario
	update *update_parish.Scenario
	del    *delete_parish.Scenario
}

func (s *parishService) ListParishes(ctx context.Context, access models.Access, page models.Page) ([]models.Parish, error) {
	return s.list.ListParishes(ctx, access, page)
}

func (s *parishService) SearchParishes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Parish, error) {
	return s.search.SearchParishes(ctx, access, q)
}

func (s *parishService) GetParish(ctx context.Context, id models.ID) (models.Parish, error) {
	return s.get.GetParish(ctx, id)
}

func (s *parishService) CreateParish(ctx context.Context, p models.Parish) (models.Parish, error) {
	return s.create.CreateParish(ctx, p)
}

func (s *parishService) UpdateParish(ctx context.Context, p models.Parish) error {
	return s.update.UpdateParish(ctx, p)
}

func (s *parishService) DeleteParish(ctx context.Context, id models.ID) error {
	return s.del.DeleteParish(ctx, id)
}

// newParishService собирает фасад на настоящем хранилище.
func newParishService(t *testing.T, st *sqlstore.Store) *parishService {
	t.Helper()

	return &parishService{
		list:   list_parishes.New(st),
		search: search_parishes.New(st),
		get:    get_parish.New(st),
		create: create_parish.New(st, idgen.New()),
		update: update_parish.New(st),
		del:    delete_parish.New(st),
	}
}

// archiveService — фасад httpapi.ArchiveService на настоящих сценариях (так
// же собран internal/app's archiveService).
type archiveService struct {
	list   *list_archives.Scenario
	search *search_archives.Scenario
	get    *get_archive.Scenario
	create *create_archive.Scenario
	update *update_archive.Scenario
	del    *delete_archive.Scenario
}

func (s *archiveService) ListArchives(ctx context.Context, access models.Access, page models.Page) ([]models.Archive, error) {
	return s.list.ListArchives(ctx, access, page)
}

func (s *archiveService) SearchArchives(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Archive, error) {
	return s.search.SearchArchives(ctx, access, q)
}

func (s *archiveService) GetArchive(ctx context.Context, access models.Access, id models.ID) (models.Archive, error) {
	return s.get.GetArchive(ctx, access, id)
}

func (s *archiveService) CreateArchive(ctx context.Context, a models.Archive) (models.Archive, error) {
	return s.create.CreateArchive(ctx, a)
}

func (s *archiveService) UpdateArchive(ctx context.Context, a models.Archive) error {
	return s.update.UpdateArchive(ctx, a)
}

func (s *archiveService) DeleteArchive(ctx context.Context, id models.ID) error {
	return s.del.DeleteArchive(ctx, id)
}

// newArchiveService собирает фасад на настоящем хранилище.
func newArchiveService(t *testing.T, st *sqlstore.Store) *archiveService {
	t.Helper()

	return &archiveService{
		list:   list_archives.New(st),
		search: search_archives.New(st),
		get:    get_archive.New(st),
		create: create_archive.New(st, idgen.New()),
		update: update_archive.New(st),
		del:    delete_archive.New(st),
	}
}

// archiveNodeService — фасад httpapi.ArchiveNodeService на настоящих
// сценариях (так же собран internal/app's archiveNodeService).
type archiveNodeService struct {
	list   *list_archive_nodes.Scenario
	search *search_archive_nodes.Scenario
	get    *get_archive_node.Scenario
	create *create_archive_node.Scenario
	update *update_archive_node.Scenario
	del    *delete_archive_node.Scenario
}

func (s *archiveNodeService) ListArchiveNodes(ctx context.Context, access models.Access, q models.ArchiveNodeQuery) ([]models.ArchiveNode, error) {
	return s.list.ListArchiveNodes(ctx, access, q)
}

func (s *archiveNodeService) SearchArchiveNodes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.ArchiveNode, error) {
	return s.search.SearchArchiveNodes(ctx, access, q)
}

func (s *archiveNodeService) GetArchiveNode(ctx context.Context, access models.Access, id models.ID) (models.ArchiveNode, error) {
	return s.get.GetArchiveNode(ctx, access, id)
}

func (s *archiveNodeService) CreateArchiveNode(ctx context.Context, n models.ArchiveNode) (models.ArchiveNode, error) {
	return s.create.CreateArchiveNode(ctx, n)
}

func (s *archiveNodeService) UpdateArchiveNode(ctx context.Context, n models.ArchiveNode) error {
	return s.update.UpdateArchiveNode(ctx, n)
}

func (s *archiveNodeService) DeleteArchiveNode(ctx context.Context, id models.ID) error {
	return s.del.DeleteArchiveNode(ctx, id)
}

// newArchiveNodeService собирает фасад на настоящем хранилище.
func newArchiveNodeService(t *testing.T, st *sqlstore.Store) *archiveNodeService {
	t.Helper()

	return &archiveNodeService{
		list:   list_archive_nodes.New(st),
		search: search_archive_nodes.New(st),
		get:    get_archive_node.New(st),
		create: create_archive_node.New(st, idgen.New()),
		update: update_archive_node.New(st),
		del:    delete_archive_node.New(st),
	}
}

// archiveDocumentService — фасад httpapi.ArchiveDocumentService на настоящих
// сценариях (так же собран internal/app's archiveDocumentService).
type archiveDocumentService struct {
	list   *list_archive_documents.Scenario
	search *search_archive_documents.Scenario
	get    *get_archive_document.Scenario
	create *create_archive_document.Scenario
	update *update_archive_document.Scenario
	del    *delete_archive_document.Scenario
}

func (s *archiveDocumentService) ListArchiveDocuments(ctx context.Context, access models.Access, page models.Page) ([]models.ArchiveDocument, error) {
	return s.list.ListArchiveDocuments(ctx, access, page)
}

func (s *archiveDocumentService) SearchArchiveDocuments(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.ArchiveDocument, error) {
	return s.search.SearchArchiveDocuments(ctx, access, q)
}

func (s *archiveDocumentService) GetArchiveDocument(ctx context.Context, access models.Access, id models.ID) (models.ArchiveDocument, error) {
	return s.get.GetArchiveDocument(ctx, access, id)
}

func (s *archiveDocumentService) CreateArchiveDocument(ctx context.Context, d models.ArchiveDocument) (models.ArchiveDocument, error) {
	return s.create.CreateArchiveDocument(ctx, d)
}

func (s *archiveDocumentService) UpdateArchiveDocument(ctx context.Context, d models.ArchiveDocument) error {
	return s.update.UpdateArchiveDocument(ctx, d)
}

func (s *archiveDocumentService) DeleteArchiveDocument(ctx context.Context, id models.ID) error {
	return s.del.DeleteArchiveDocument(ctx, id)
}

// newArchiveDocumentService собирает фасад на настоящем хранилище.
func newArchiveDocumentService(t *testing.T, st *sqlstore.Store) *archiveDocumentService {
	t.Helper()

	return &archiveDocumentService{
		list:   list_archive_documents.New(st),
		search: search_archive_documents.New(st),
		get:    get_archive_document.New(st),
		create: create_archive_document.New(st, idgen.New()),
		update: update_archive_document.New(st),
		del:    delete_archive_document.New(st),
	}
}

// sourceService — фасад httpapi.SourceService на настоящих сценариях (так же
// собран internal/app's sourceService).
type sourceService struct {
	list   *list_sources.Scenario
	search *search_sources.Scenario
	get    *get_source.Scenario
	create *create_source.Scenario
	update *update_source.Scenario
	del    *delete_source.Scenario
}

func (s *sourceService) ListSources(ctx context.Context, access models.Access, page models.Page) ([]models.Source, error) {
	return s.list.ListSources(ctx, access, page)
}

func (s *sourceService) SearchSources(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Source, error) {
	return s.search.SearchSources(ctx, access, q)
}

func (s *sourceService) GetSource(ctx context.Context, access models.Access, id models.ID) (models.Source, error) {
	return s.get.GetSource(ctx, access, id)
}

func (s *sourceService) CreateSource(ctx context.Context, src models.Source) (models.Source, error) {
	return s.create.CreateSource(ctx, src)
}

func (s *sourceService) UpdateSource(ctx context.Context, src models.Source) error {
	return s.update.UpdateSource(ctx, src)
}

func (s *sourceService) DeleteSource(ctx context.Context, id models.ID) error {
	return s.del.DeleteSource(ctx, id)
}

// newSourceService собирает фасад на настоящем хранилище.
func newSourceService(t *testing.T, st *sqlstore.Store) *sourceService {
	t.Helper()

	return &sourceService{
		list:   list_sources.New(st),
		search: search_sources.New(st),
		get:    get_source.New(st),
		create: create_source.New(st, idgen.New()),
		update: update_source.New(st),
		del:    delete_source.New(st),
	}
}

// citationService — фасад httpapi.CitationService на настоящих сценариях
// (так же собран internal/app's citationService).
type citationService struct {
	list   *list_citations.Scenario
	search *search_citations.Scenario
	get    *get_citation.Scenario
	create *create_citation.Scenario
	update *update_citation.Scenario
	del    *delete_citation.Scenario
}

func (s *citationService) ListCitations(ctx context.Context, access models.Access, page models.Page) ([]models.Citation, error) {
	return s.list.ListCitations(ctx, access, page)
}

func (s *citationService) SearchCitations(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Citation, error) {
	return s.search.SearchCitations(ctx, access, q)
}

func (s *citationService) GetCitation(ctx context.Context, access models.Access, id models.ID) (models.Citation, error) {
	return s.get.GetCitation(ctx, access, id)
}

func (s *citationService) CreateCitation(ctx context.Context, c models.Citation) (models.Citation, error) {
	return s.create.CreateCitation(ctx, c)
}

func (s *citationService) UpdateCitation(ctx context.Context, c models.Citation) error {
	return s.update.UpdateCitation(ctx, c)
}

func (s *citationService) DeleteCitation(ctx context.Context, id models.ID) error {
	return s.del.DeleteCitation(ctx, id)
}

// newCitationService собирает фасад на настоящем хранилище.
func newCitationService(t *testing.T, st *sqlstore.Store) *citationService {
	t.Helper()

	return &citationService{
		list:   list_citations.New(st),
		search: search_citations.New(st),
		get:    get_citation.New(st),
		create: create_citation.New(st, idgen.New()),
		update: update_citation.New(st),
		del:    delete_citation.New(st),
	}
}

// noteService — фасад httpapi.NoteService на настоящих сценариях (так же
// собран internal/app's noteService).
type noteService struct {
	list   *list_notes.Scenario
	search *search_notes.Scenario
	get    *get_note.Scenario
	create *create_note.Scenario
	update *update_note.Scenario
	del    *delete_note.Scenario
}

func (s *noteService) ListNotes(ctx context.Context, access models.Access, page models.Page) ([]models.Note, error) {
	return s.list.ListNotes(ctx, access, page)
}

func (s *noteService) SearchNotes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Note, error) {
	return s.search.SearchNotes(ctx, access, q)
}

func (s *noteService) GetNote(ctx context.Context, access models.Access, id models.ID) (models.Note, error) {
	return s.get.GetNote(ctx, access, id)
}

func (s *noteService) CreateNote(ctx context.Context, n models.Note) (models.Note, error) {
	return s.create.CreateNote(ctx, n)
}

func (s *noteService) UpdateNote(ctx context.Context, n models.Note) error {
	return s.update.UpdateNote(ctx, n)
}

func (s *noteService) DeleteNote(ctx context.Context, id models.ID) error {
	return s.del.DeleteNote(ctx, id)
}

// newNoteService собирает фасад на настоящем хранилище.
func newNoteService(t *testing.T, st *sqlstore.Store) *noteService {
	t.Helper()

	return &noteService{
		list:   list_notes.New(st),
		search: search_notes.New(st),
		get:    get_note.New(st),
		create: create_note.New(st, idgen.New()),
		update: update_note.New(st),
		del:    delete_note.New(st),
	}
}

// attachmentService — фасад httpapi.AttachmentService на настоящих сценариях
// (так же собран internal/app's attachmentService).
type attachmentService struct {
	list   *list_attachments.Scenario
	search *search_attachments.Scenario
	get    *get_attachment.Scenario
	create *create_attachment.Scenario
	update *update_attachment.Scenario
	del    *delete_attachment.Scenario
}

func (s *attachmentService) ListAttachments(ctx context.Context, access models.Access, page models.Page) ([]models.Attachment, error) {
	return s.list.ListAttachments(ctx, access, page)
}

func (s *attachmentService) SearchAttachments(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Attachment, error) {
	return s.search.SearchAttachments(ctx, access, q)
}

func (s *attachmentService) GetAttachment(ctx context.Context, access models.Access, id models.ID) (models.Attachment, error) {
	return s.get.GetAttachment(ctx, access, id)
}

func (s *attachmentService) CreateAttachment(ctx context.Context, a models.Attachment) (models.Attachment, error) {
	return s.create.CreateAttachment(ctx, a)
}

func (s *attachmentService) UpdateAttachment(ctx context.Context, a models.Attachment) error {
	return s.update.UpdateAttachment(ctx, a)
}

func (s *attachmentService) DeleteAttachment(ctx context.Context, id models.ID) error {
	return s.del.DeleteAttachment(ctx, id)
}

// newAttachmentService собирает фасад на настоящем хранилище.
func newAttachmentService(t *testing.T, st *sqlstore.Store) *attachmentService {
	t.Helper()

	return &attachmentService{
		list:   list_attachments.New(st),
		search: search_attachments.New(st),
		get:    get_attachment.New(st),
		create: create_attachment.New(st, idgen.New()),
		update: update_attachment.New(st),
		del:    delete_attachment.New(st),
	}
}

// TestAdminDivisionsWithRealStore: сквозной путь «хранилище → сценарий → HTTP»
// на настоящей БД (так же собран internal/app): фильтры, окно после фильтра,
// parent_id, коды ошибок, прежнего пути нет.
func TestAdminDivisionsWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	root := models.ID("AD-11HFE865V215DE1CTEWH0AVNH9")
	ad1 := models.ID("AD-3N65R6PPG2X7R5JQC42EJ5E6RH")
	ad2 := models.ID("AD-7QAQPDH4AFAXRHEM3E2MSH4DMD")
	ad3 := models.ID("AD-7SX9G8FGVSQE8Z5379AV4RRXG0")
	missingParent := "AD-01ARZ3NDEKTSV4RRFFQ69G5FA9" // валидный формат, не сохранён

	for _, d := range []models.AdministrativeDivision{
		{ID: root, Name: "Московская", Type: models.AdminDivisionGovernorate},
		{ID: ad1, Name: "Давыдово", Type: models.AdminDivisionSelo, ParentID: &root},
		{ID: ad2, Name: "Никифоровская", Type: models.AdminDivisionVolost, ParentID: &root,
			Variants: []string{"Никольское"}},
		{ID: ad3, Name: "Никифорово", Type: models.AdminDivisionDerevnya, ParentID: &root},
	} {
		if err := st.SaveAdministrativeDivision(t.Context(), &d); err != nil {
			t.Fatalf("save %s: %v", d.ID, err)
		}
	}

	h := httpapi.NewHandler(httpapi.Deps{Divisions: newDivisionService(t, st), DocsFS: fstest.MapFS{}})

	cases := []struct {
		target string
		code   int
		body   string // пусто — не сверять
	}{
		{"/api/admin-divisions?kind=settlement", http.StatusOK,
			fmt.Sprintf(`[{"id":%q,"name":"Давыдово","type":"selo","parent_id":%q,"sources":[]},`+
				`{"id":%q,"name":"Никифорово","type":"derevnya","parent_id":%q,"sources":[]}]`, ad1, root, ad3, root)},
		{"/api/admin-divisions?kind=settlement&limit=1&offset=1", http.StatusOK,
			fmt.Sprintf(`[{"id":%q,"name":"Никифорово","type":"derevnya","parent_id":%q,"sources":[]}]`, ad3, root)},
		{"/api/admin-divisions?type=governorate", http.StatusOK,
			fmt.Sprintf(`[{"id":%q,"name":"Московская","type":"governorate","parent_id":null,"sources":[]}]`, root)},
		{"/api/admin-divisions?type=castle", http.StatusUnprocessableEntity,
			`{"error":"type: неизвестный тип единицы деления \"castle\"","field":"type"}`},
		{"/api/admin-divisions?limit=x", http.StatusBadRequest,
			`{"error":"параметр limit: ожидалось целое число, получено \"x\""}`},
		{"/api/settlements", http.StatusNotFound, ""},
		{"/api/admin-divisions/search?q=давы", http.StatusOK,
			fmt.Sprintf(`[{"id":%q,"name":"Давыдово","type":"selo","parent_id":%q,"sources":[]}]`, ad1, root)},
		{"/api/admin-divisions/search?q=", http.StatusOK, `[]`},
		{"/api/admin-divisions/search?q=ник", http.StatusOK,
			fmt.Sprintf(`[{"id":%q,"name":"Никифоровская","type":"volost","parent_id":%q,"sources":[]},`+
				`{"id":%q,"name":"Никифорово","type":"derevnya","parent_id":%q,"sources":[]}]`, ad2, root, ad3, root)},
		{"/api/admin-divisions/search?q=никол", http.StatusOK,
			fmt.Sprintf(`[{"id":%q,"name":"Никифоровская","type":"volost","parent_id":%q,"sources":[]}]`, ad2, root)},
		{"/api/admin-divisions/search?q=ик", http.StatusOK, `[]`},
		{"/api/admin-divisions?parent_id=" + string(root), http.StatusOK,
			fmt.Sprintf(`[{"id":%q,"name":"Давыдово","type":"selo","parent_id":%q,"sources":[]},`+
				`{"id":%q,"name":"Никифоровская","type":"volost","parent_id":%q,"sources":[]},`+
				`{"id":%q,"name":"Никифорово","type":"derevnya","parent_id":%q,"sources":[]}]`,
				ad1, root, ad2, root, ad3, root)},
		{"/api/admin-divisions?parent_id=" + missingParent, http.StatusNotFound, ""},
		{"/api/admin-divisions?parent_id=not-an-id", http.StatusUnprocessableEntity, ""},
	}

	for _, c := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.target, nil))

		body := strings.TrimSpace(rec.Body.String())
		if rec.Code != c.code || (c.body != "" && body != c.body) {
			t.Errorf("GET %s = %d %s\n want %d %s", c.target, rec.Code, body, c.code, c.body)
		}
	}
}
```
#### `internal/httpapi/write_store_test.go` (изменить — итоговое содержимое)
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
	"github.com/amarin/genodex/internal/idgen"
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
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Sources:    newSourceService(t, st),
		Citations:  newCitationService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

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
	requireStatusS(t, delReq(t, h, owner, "/api/admin-divisions/"+string(child.ID)), http.StatusNoContent)
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

	// Ретрофит: строгий FK sources[i].citation_id — сквозная проверка через
	// реальный HTTP-хендлер → сценарий → SQLite (по образцу
	// TestArchiveWriteContractWithRealStore; Division — единственная сущность,
	// у которой Sources пришлось добавлять в read/write-контракт с нуля, а не
	// только в usecase, см. docs/data-model/entity-write.md §3.3).
	src := createSource(t, h, owner,
		`{"kind":"document","title":"Ревизская сказка","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	cit := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)

	withCitation := createDivision(t, h, owner,
		fmt.Sprintf(`{"name":"Гавриловское","type":"volost","sources":[{"citation_id":%q}]}`, cit.ID),
		http.StatusCreated)
	if len(withCitation.Sources) != 1 || withCitation.Sources[0].CitationID != string(cit.ID) {
		t.Fatalf("withCitation.Sources = %+v, want [{citation_id: %q}]", withCitation.Sources, cit.ID)
	}

	// Строгий FK: корректный по формату, но несуществующий citation_id — 422 на sources[0].citation_id.
	rec = postReq(t, h, owner,
		`{"name":"Призрачная волость","type":"volost","sources":[{"citation_id":"C-01ARZ3NDEKTSV4RRFFQ69G5FA9"}]}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"sources[0].citation_id"`) {
		t.Fatalf("body = %s, want field=sources[0].citation_id", rec.Body)
	}

	// Logout инвалидирует сессию: та же cookie больше не проходит requireFull.
	logoutRec := postAuthReq(t, h, "/api/auth/logout", "", owner)
	requireStatusS(t, logoutRec, http.StatusNoContent)

	staleRec := postReq(t, h, owner, `{"name":"После логаута","type":"selo"}`)
	requireStatusS(t, staleRec, http.StatusUnauthorized)

	// Анонимная попытка создать — 401, до разбора тела.
	anonRec := postReq(t, h, nil, `{"name":"Аноним","type":"selo"}`)
	requireStatusS(t, anonRec, http.StatusUnauthorized)
}

// TestSurnameWriteContractWithRealStore: сквозной путь «хранилище → сценарии
// → HTTP» (так же собран internal/app) для записи словарных записей фамилий
// — через NewAPIHandler с реальной сессией владельца: bootstrap-регистрация
// → создание → чтение → изменение → удаление → повторное чтение — 404
// (по образцу TestDivisionWriteContractWithRealStore, но без 409-сценария:
// у Surname нет строгих внешних ключей, docs/data-model/entity-write.md).
func TestSurnameWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Surnames:   newSurnameService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание записи.
	created := createSurname(t, h, owner,
		`{"canonical":"Иванов","variants":[{"text":"Иванова"}],"items":[],"notes":[]}`, http.StatusCreated)
	if created.Canonical != "Иванов" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "SN-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id — открыто анонимному посетителю.
	rec := getReq(t, h, "/api/surnames/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"canonical":"Иванов"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена canonical/variants/items/notes.
	rec = putSurnameReq(t, h, owner, "/api/surnames/"+string(created.ID),
		`{"canonical":"Иванова","variants":[],"items":[],"notes":[]}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeSurnameS(t, rec)
	if updated.Canonical != "Иванова" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление — у Surname нет строгих FK, конфликта не бывает.
	requireStatusS(t, delReq(t, h, owner, "/api/surnames/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/surnames/"+string(created.ID)), http.StatusNotFound)
}

func createSurname(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Surname {
	t.Helper()

	rec := postSurnameReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeSurnameS(t, rec)
}

func decodeSurnameS(t *testing.T, rec *httptest.ResponseRecorder) transport.Surname {
	t.Helper()

	var s transport.Surname
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return s
}

func postSurnameReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/surnames", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putSurnameReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestPatronymicWriteContractWithRealStore: сквозной путь «хранилище →
// сценарии → HTTP» (так же собран internal/app) для записи словарных
// записей отчеств — через NewAPIHandler с реальной сессией владельца:
// bootstrap-регистрация → создание → чтение → изменение → удаление →
// повторное чтение — 404 (по образцу TestSurnameWriteContractWithRealStore,
// без 409-сценария: у Patronymic нет строгих внешних ключей,
// docs/data-model/entity-write.md).
func TestPatronymicWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:   newDivisionService(t, st),
		Patronymics: newPatronymicService(t, st),
		Auth:        authSvc,
		DocsFS:      fstest.MapFS{},
		TrustProxy:  false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание записи.
	created := createPatronymic(t, h, owner,
		`{"canonical":"Иванович","variants":[{"text":"Иванычъ"}],"items":[],"notes":[]}`, http.StatusCreated)
	if created.Canonical != "Иванович" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "PN-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id — открыто анонимному посетителю.
	rec := getReq(t, h, "/api/patronymics/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"canonical":"Иванович"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена canonical/variants/items/notes.
	rec = putPatronymicReq(t, h, owner, "/api/patronymics/"+string(created.ID),
		`{"canonical":"Ивановна","variants":[],"items":[],"notes":[]}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodePatronymicS(t, rec)
	if updated.Canonical != "Ивановна" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление — у Patronymic нет строгих FK, конфликта не бывает.
	requireStatusS(t, delReq(t, h, owner, "/api/patronymics/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/patronymics/"+string(created.ID)), http.StatusNotFound)
}

func createPatronymic(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Patronymic {
	t.Helper()

	rec := postPatronymicReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodePatronymicS(t, rec)
}

func decodePatronymicS(t *testing.T, rec *httptest.ResponseRecorder) transport.Patronymic {
	t.Helper()

	var s transport.Patronymic
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return s
}

func postPatronymicReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/patronymics", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putPatronymicReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestEstateWriteContractWithRealStore: сквозной путь «хранилище → сценарии
// → HTTP» (так же собран internal/app) для записи словарных записей
// сословий — через NewAPIHandler с реальной сессией владельца:
// bootstrap-регистрация → создание → чтение → изменение → удаление →
// повторное чтение — 404 (по образцу TestSurnameWriteContractWithRealStore,
// без 409-сценария: у Estate нет строгих внешних ключей,
// docs/data-model/entity-write.md).
func TestEstateWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Estates:    newEstateService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание записи.
	created := createEstate(t, h, owner,
		`{"canonical":"крестьяне","variants":[{"text":"крестьянство"}],"items":[],"notes":[]}`, http.StatusCreated)
	if created.Canonical != "крестьяне" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "ES-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id — открыто анонимному посетителю.
	rec := getReq(t, h, "/api/estates/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"canonical":"крестьяне"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена canonical/variants/items/notes.
	rec = putEstateReq(t, h, owner, "/api/estates/"+string(created.ID),
		`{"canonical":"мещане","variants":[],"items":[],"notes":[]}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeEstateS(t, rec)
	if updated.Canonical != "мещане" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление — у Estate нет строгих FK, конфликта не бывает.
	requireStatusS(t, delReq(t, h, owner, "/api/estates/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/estates/"+string(created.ID)), http.StatusNotFound)
}

func createEstate(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Estate {
	t.Helper()

	rec := postEstateReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeEstateS(t, rec)
}

func decodeEstateS(t *testing.T, rec *httptest.ResponseRecorder) transport.Estate {
	t.Helper()

	var s transport.Estate
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return s
}

func postEstateReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/estates", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putEstateReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestTitleWriteContractWithRealStore: сквозной путь «хранилище → сценарии
// → HTTP» (так же собран internal/app) для записи словарных записей
// титулов — через NewAPIHandler с реальной сессией владельца:
// bootstrap-регистрация → создание → чтение → изменение → удаление →
// повторное чтение — 404 (по образцу TestSurnameWriteContractWithRealStore,
// без 409-сценария: у Title нет строгих внешних ключей,
// docs/data-model/entity-write.md).
func TestTitleWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Titles:     newTitleService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание записи.
	created := createTitle(t, h, owner,
		`{"canonical":"вдова","variants":[{"text":"вдовица"}],"items":[],"notes":[]}`, http.StatusCreated)
	if created.Canonical != "вдова" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "TT-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id — открыто анонимному посетителю.
	rec := getReq(t, h, "/api/titles/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"canonical":"вдова"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена canonical/variants/items/notes.
	rec = putTitleReq(t, h, owner, "/api/titles/"+string(created.ID),
		`{"canonical":"вдовец","variants":[],"items":[],"notes":[]}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeTitleS(t, rec)
	if updated.Canonical != "вдовец" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление — у Title нет строгих FK, конфликта не бывает.
	requireStatusS(t, delReq(t, h, owner, "/api/titles/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/titles/"+string(created.ID)), http.StatusNotFound)
}

func createTitle(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Title {
	t.Helper()

	rec := postTitleReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeTitleS(t, rec)
}

func decodeTitleS(t *testing.T, rec *httptest.ResponseRecorder) transport.Title {
	t.Helper()

	var s transport.Title
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return s
}

func postTitleReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/titles", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putTitleReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestGivenNameWriteContractWithRealStore: сквозной путь «хранилище →
// сценарии → HTTP» (так же собран internal/app) для записи словарных
// записей имён — через NewAPIHandler с реальной сессией владельца:
// bootstrap-регистрация → создание → чтение → изменение → удаление →
// повторное чтение — 404 (по образцу TestSurnameWriteContractWithRealStore,
// без 409-сценария: у GivenName нет строгих внешних ключей,
// docs/data-model/entity-write.md). Дополнительно проверяет поле gender —
// единственное отличие GivenName от остальных трёх словарных сущностей.
func TestGivenNameWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		GivenNames: newGivenNameService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание записи с полом male.
	created := createGivenName(t, h, owner,
		`{"canonical":"Иван","gender":"male","variants":[{"text":"Иоанн"}],"items":[],"notes":[]}`, http.StatusCreated)
	if created.Canonical != "Иван" {
		t.Fatalf("created = %+v", created)
	}
	if created.Gender != models.NameGenderMale {
		t.Fatalf("created.Gender = %q, want male", created.Gender)
	}
	if !strings.HasPrefix(string(created.ID), "GN-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id — открыто анонимному посетителю, gender виден в ответе.
	rec := getReq(t, h, "/api/given-names/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"canonical":"Иван"`) || !strings.Contains(rec.Body.String(), `"gender":"male"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена canonical/gender/variants/items/notes, gender → neutral.
	rec = putGivenNameReq(t, h, owner, "/api/given-names/"+string(created.ID),
		`{"canonical":"Саша","gender":"neutral","variants":[],"items":[],"notes":[]}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeGivenNameS(t, rec)
	if updated.Canonical != "Саша" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}
	if updated.Gender != models.NameGenderNeutral {
		t.Fatalf("updated.Gender = %q, want neutral", updated.Gender)
	}

	// Удаление — у GivenName нет строгих FK, конфликта не бывает.
	requireStatusS(t, delReq(t, h, owner, "/api/given-names/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/given-names/"+string(created.ID)), http.StatusNotFound)
}

func createGivenName(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.GivenName {
	t.Helper()

	rec := postGivenNameReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeGivenNameS(t, rec)
}

func decodeGivenNameS(t *testing.T, rec *httptest.ResponseRecorder) transport.GivenName {
	t.Helper()

	var s transport.GivenName
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return s
}

func postGivenNameReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/given-names", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putGivenNameReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestDivisionCreateWithoutCSRFHeaderIs400: валидная сессия владельца, но без
// X-Requested-With — 400 (requireCSRFHeader), сценарий не вызывается. Пин на
// то, что NewAPIHandler реально оборачивает division-записи, не только auth.
func TestDivisionCreateWithoutCSRFHeaderIs400(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{Divisions: newDivisionService(t, st), Auth: authSvc, DocsFS: fstest.MapFS{}, TrustProxy: false})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)

	req := httptest.NewRequest(http.MethodPost, "/api/admin-divisions", strings.NewReader(`{"name":"x","type":"selo"}`))
	req.AddCookie(accessCookie)
	// нарочно без X-Requested-With

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	requireStatusS(t, rec, http.StatusBadRequest)
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

// TestRepositoryWriteContractWithRealStore: сквозной путь «хранилище →
// сценарии → HTTP» (по образцу TestGivenNameWriteContractWithRealStore) для
// хранилищ-контейнеров источников: bootstrap-регистрация → создание →
// чтение → изменение → удаление → повторное чтение — 404. Дополнительно
// закрывает Fix 1 (CRITICAL): приватная запись, созданная владельцем, должна
// быть недоступна анонимному GET /api/repositories/{id} — 404, а не 200.
func TestRepositoryWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:    newDivisionService(t, st),
		Repositories: newRepositoryService(t, st),
		Auth:         authSvc,
		DocsFS:       fstest.MapFS{},
		TrustProxy:   false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание.
	created := createRepository(t, h, owner,
		`{"name":"ГАВО","type":"archive","address":"Вологда","urls":[],"notes":[],"private":false}`, http.StatusCreated)
	if created.Name != "ГАВО" || created.Type != "archive" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "R-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id — открыто анонимному посетителю.
	rec := getReq(t, h, "/api/repositories/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"name":"ГАВО"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена name/type/address/urls/notes/private.
	rec = putRepositoryReq(t, h, owner, "/api/repositories/"+string(created.ID),
		`{"name":"ГАВО (испр.)","type":"library","address":"Вологда","urls":[],"notes":[],"private":false}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeRepositoryS(t, rec)
	if updated.Name != "ГАВО (испр.)" || updated.Type != "library" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/repositories/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/repositories/"+string(created.ID)), http.StatusNotFound)

	// Fix 1: приватная запись, анонимный GET — 404, не 200.
	private := createRepository(t, h, owner,
		`{"name":"Частное собрание","type":"private","address":"","urls":[],"notes":[],"private":true}`, http.StatusCreated)
	if !private.Private {
		t.Fatalf("private = %+v, want Private=true", private)
	}

	requireStatusS(t, getReq(t, h, "/api/repositories/"+string(private.ID)), http.StatusNotFound)
}

func createRepository(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Repository {
	t.Helper()

	rec := postRepositoryReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeRepositoryS(t, rec)
}

func decodeRepositoryS(t *testing.T, rec *httptest.ResponseRecorder) transport.Repository {
	t.Helper()

	var r transport.Repository
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return r
}

func postRepositoryReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/repositories", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putRepositoryReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestChurchWriteContractWithRealStore: сквозной путь «хранилище → сценарии →
// HTTP» для церквей (по образцу TestGivenNameWriteContractWithRealStore, без
// приватности — у Church нет поля Private): bootstrap-регистрация →
// создание → чтение → изменение → удаление → повторное чтение — 404.
func TestChurchWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Churches:   newChurchService(t, st),
		Sources:    newSourceService(t, st),
		Citations:  newCitationService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание.
	created := createChurch(t, h, owner,
		`{"name":"Троицкая церковь","settlements":[],"variants":[],"notes":[]}`, http.StatusCreated)
	if created.Name != "Троицкая церковь" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "CH-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id.
	rec := getReq(t, h, "/api/churches/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"name":"Троицкая церковь"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена name/parish/settlements/variants/notes.
	rec = putChurchReq(t, h, owner, "/api/churches/"+string(created.ID),
		`{"name":"Троицкая церковь (испр.)","settlements":[],"variants":[],"notes":[]}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeChurchS(t, rec)
	if updated.Name != "Троицкая церковь (испр.)" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/churches/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/churches/"+string(created.ID)), http.StatusNotFound)

	// Ретрофит: строгий FK sources[i].citation_id — сквозная проверка через
	// реальный HTTP-хендлер → сценарий → SQLite (по образцу
	// TestArchiveWriteContractWithRealStore). Также закрывает create_church's
	// перевод на транзакционный InTx (docs/data-model/entity-write.md §3.3),
	// который иначе не проверяется ни одним real-store тестом.
	src := createSource(t, h, owner,
		`{"kind":"document","title":"Метрическая книга","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	cit := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)

	withCitation := createChurch(t, h, owner,
		fmt.Sprintf(`{"name":"Покровская церковь","settlements":[],"variants":[],"notes":[],"sources":[{"citation_id":%q}]}`, cit.ID),
		http.StatusCreated)
	if len(withCitation.Sources) != 1 || withCitation.Sources[0].CitationID != string(cit.ID) {
		t.Fatalf("withCitation.Sources = %+v, want [{citation_id: %q}]", withCitation.Sources, cit.ID)
	}

	// Строгий FK: корректный по формату, но несуществующий citation_id — 422 на sources[0].citation_id.
	rec = postChurchReq(t, h, owner,
		`{"name":"Церковь-призрак","settlements":[],"variants":[],"notes":[],"sources":[{"citation_id":"C-01ARZ3NDEKTSV4RRFFQ69G5FA9"}]}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"sources[0].citation_id"`) {
		t.Fatalf("body = %s, want field=sources[0].citation_id", rec.Body)
	}
}

func createChurch(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Church {
	t.Helper()

	rec := postChurchReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeChurchS(t, rec)
}

func decodeChurchS(t *testing.T, rec *httptest.ResponseRecorder) transport.Church {
	t.Helper()

	var c transport.Church
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return c
}

func postChurchReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/churches", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putChurchReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestParishWriteContractWithRealStore: сквозной путь «хранилище → сценарии →
// HTTP» для приходов (по образцу TestGivenNameWriteContractWithRealStore, без
// приватности — у Parish нет поля Private): bootstrap-регистрация →
// создание → чтение → изменение → удаление → повторное чтение — 404.
func TestParishWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Parishes:   newParishService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание.
	created := createParish(t, h, owner,
		`{"name":"Троицкий приход","settlements":[],"notes":[]}`, http.StatusCreated)
	if created.Name != "Троицкий приход" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "PR-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id.
	rec := getReq(t, h, "/api/parishes/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"name":"Троицкий приход"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена name/church/settlements/since/until/notes.
	rec = putParishReq(t, h, owner, "/api/parishes/"+string(created.ID),
		`{"name":"Троицкий приход (испр.)","settlements":[],"notes":[]}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeParishS(t, rec)
	if updated.Name != "Троицкий приход (испр.)" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/parishes/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/parishes/"+string(created.ID)), http.StatusNotFound)
}

func createParish(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Parish {
	t.Helper()

	rec := postParishReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeParishS(t, rec)
}

func decodeParishS(t *testing.T, rec *httptest.ResponseRecorder) transport.Parish {
	t.Helper()

	var p transport.Parish
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return p
}

func postParishReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/parishes", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putParishReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestArchiveWriteContractWithRealStore: сквозной путь «хранилище → сценарии
// → HTTP» для архивов (по образцу TestGivenNameWriteContractWithRealStore):
// bootstrap-регистрация → создание → чтение → изменение → удаление →
// повторное чтение — 404. Дополнительно: строгий FK на Repository (валидный
// repository_id сохраняется; несуществующий, но корректный по формату —
// 422 на поле repository_id) и Fix 1 (CRITICAL): приватная запись, анонимный
// GET — 404, не 200.
func TestArchiveWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:    newDivisionService(t, st),
		Repositories: newRepositoryService(t, st),
		Archives:     newArchiveService(t, st),
		Sources:      newSourceService(t, st),
		Citations:    newCitationService(t, st),
		Auth:         authSvc,
		DocsFS:       fstest.MapFS{},
		TrustProxy:   false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание без хранилища.
	created := createArchive(t, h, owner, `{"name":"ГАВО, архив","notes":[],"private":false}`, http.StatusCreated)
	if created.Name != "ГАВО, архив" || created.RepositoryID != "" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "AR-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id.
	rec := getReq(t, h, "/api/archives/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"name":"ГАВО, архив"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена name/system/repository_id/notes/private.
	rec = putArchiveReq(t, h, owner, "/api/archives/"+string(created.ID),
		`{"name":"ГАВО, архив (испр.)","notes":[],"private":false}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeArchiveS(t, rec)
	if updated.Name != "ГАВО, архив (испр.)" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/archives/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/archives/"+string(created.ID)), http.StatusNotFound)

	// Строгий FK: валидный repository_id создаётся и сохраняется как есть.
	repo := createRepository(t, h, owner,
		`{"name":"ГАВО","type":"archive","address":"","urls":[],"notes":[],"private":false}`, http.StatusCreated)

	withRepo := createArchive(t, h, owner,
		fmt.Sprintf(`{"name":"Фонд 1","repository_id":%q,"notes":[],"private":false}`, repo.ID), http.StatusCreated)
	if withRepo.RepositoryID != string(repo.ID) {
		t.Fatalf("withRepo.RepositoryID = %q, want %q", withRepo.RepositoryID, repo.ID)
	}

	// Строгий FK: корректный по формату, но несуществующий repository_id — 422.
	rec = postArchiveReq(t, h, owner,
		`{"name":"Фонд-призрак","repository_id":"R-01ARZ3NDEKTSV4RRFFQ69G5FA9","notes":[],"private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"repository_id"`) {
		t.Fatalf("body = %s, want field=repository_id", rec.Body)
	}

	// Fix 1: приватная запись, анонимный GET — 404, не 200.
	private := createArchive(t, h, owner, `{"name":"Приватный архив","notes":[],"private":true}`, http.StatusCreated)
	if !private.Private {
		t.Fatalf("private = %+v, want Private=true", private)
	}

	requireStatusS(t, getReq(t, h, "/api/archives/"+string(private.ID)), http.StatusNotFound)

	// Ретрофит: строгий FK sources[i].citation_id — сквозная проверка через
	// реальный HTTP-хендлер → сценарий → SQLite, а не только на fake-store.
	src := createSource(t, h, owner,
		`{"kind":"document","title":"Метрическая книга","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	cit := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)

	withCitation := createArchive(t, h, owner,
		fmt.Sprintf(`{"name":"Фонд с цитатой","sources":[{"citation_id":%q}],"notes":[],"private":false}`, cit.ID),
		http.StatusCreated)
	if len(withCitation.Sources) != 1 || withCitation.Sources[0].CitationID != string(cit.ID) {
		t.Fatalf("withCitation.Sources = %+v, want [{citation_id: %q}]", withCitation.Sources, cit.ID)
	}

	// Строгий FK: корректный по формату, но несуществующий citation_id — 422 на sources[0].citation_id.
	rec = postArchiveReq(t, h, owner,
		`{"name":"Фонд-призрак 2","sources":[{"citation_id":"C-01ARZ3NDEKTSV4RRFFQ69G5FA9"}],"notes":[],"private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"sources[0].citation_id"`) {
		t.Fatalf("body = %s, want field=sources[0].citation_id", rec.Body)
	}
}

func createArchive(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Archive {
	t.Helper()

	rec := postArchiveReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeArchiveS(t, rec)
}

func decodeArchiveS(t *testing.T, rec *httptest.ResponseRecorder) transport.Archive {
	t.Helper()

	var a transport.Archive
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return a
}

func postArchiveReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/archives", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putArchiveReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestArchiveNodeWriteContractWithRealStore: сквозной путь «хранилище →
// сценарии → HTTP» (по образцу TestArchiveWriteContractWithRealStore) для
// узлов архивного дерева: bootstrap-регистрация → создание архива → создание
// корневого узла → создание дочернего узла → список с фильтром по
// archive_id (оба узла видны, корректно вложены) → изменение → удаление →
// повторное чтение — 404. Дополнительно закрывает новый для этого
// подпункта инвариант «родитель из того же архива»: несуществующий
// parent_id — 422 на поле parent_id; parent_id, указывающий на реальный
// узел, но из ДРУГОГО архива — тоже 422 на поле parent_id (не archive_id).
// Также проверяет неизменность archive_id при обновлении: попытка перенести
// существующий узел в другой архив — 422 на поле archive_id.
func TestArchiveNodeWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:    newDivisionService(t, st),
		Archives:     newArchiveService(t, st),
		ArchiveNodes: newArchiveNodeService(t, st),
		Sources:      newSourceService(t, st),
		Citations:    newCitationService(t, st),
		Auth:         authSvc,
		DocsFS:       fstest.MapFS{},
		TrustProxy:   false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	archive := createArchive(t, h, owner, `{"name":"ГАВО, архив","notes":[],"private":false}`, http.StatusCreated)
	otherArchive := createArchive(t, h, owner, `{"name":"РГАДА","notes":[],"private":false}`, http.StatusCreated)

	// Корневой узел (без parent_id).
	root := createArchiveNode(t, h, owner,
		fmt.Sprintf(`{"type":"fond","archive_id":%q,"label":"Фонд 1"}`, archive.ID), http.StatusCreated)
	if root.ArchiveID != archive.ID || root.ParentID != nil {
		t.Fatalf("root = %+v", root)
	}
	if !strings.HasPrefix(string(root.ID), "AN-") {
		t.Fatalf("id = %q", root.ID)
	}

	// Дочерний узел под корневым, в том же архиве.
	child := createArchiveNode(t, h, owner,
		fmt.Sprintf(`{"type":"opis","archive_id":%q,"parent_id":%q,"label":"Опись 1"}`, archive.ID, root.ID), http.StatusCreated)
	if child.ParentID == nil || *child.ParentID != root.ID {
		t.Fatalf("child = %+v", child)
	}

	// Список с фильтром по archive_id, без parent_id — только корень
	// (models.ArchiveNodeQuery: ParentID nil — корень внутри архива).
	rec := getReq(t, h, "/api/archive-nodes?archive_id="+string(archive.ID))
	requireStatusS(t, rec, http.StatusOK)
	rootList := decodeArchiveNodeListS(t, rec)
	if len(rootList) != 1 || rootList[0].ID != root.ID {
		t.Fatalf("rootList = %+v, want exactly [root]", rootList)
	}

	// Список с parent_id=root — только прямые дети (child).
	rec = getReq(t, h, "/api/archive-nodes?archive_id="+string(archive.ID)+"&parent_id="+string(root.ID))
	requireStatusS(t, rec, http.StatusOK)
	childList := decodeArchiveNodeListS(t, rec)
	if len(childList) != 1 || childList[0].ID != child.ID {
		t.Fatalf("childList = %+v, want exactly [child]", childList)
	}

	// Список без archive_id — 400.
	requireStatusS(t, getReq(t, h, "/api/archive-nodes"), http.StatusBadRequest)

	// Изменение: полная замена label.
	rec = putArchiveNodeReq(t, h, owner, "/api/archive-nodes/"+string(root.ID),
		fmt.Sprintf(`{"type":"fond","archive_id":%q,"label":"Фонд 1 (испр.)"}`, archive.ID))
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeArchiveNodeS(t, rec)
	if updated.Label != "Фонд 1 (испр.)" || updated.ID != root.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Несуществующий parent_id — 422 на поле parent_id.
	rec = postArchiveNodeReq(t, h, owner,
		fmt.Sprintf(`{"type":"delo","archive_id":%q,"parent_id":"AN-01ARZ3NDEKTSV4RRFFQ69G5FA9","label":"Дело-призрак"}`, archive.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"parent_id"`) {
		t.Fatalf("body = %s, want field=parent_id", rec.Body)
	}

	// parent_id из ДРУГОГО архива — тоже 422 на поле parent_id (не archive_id):
	// сам родитель существует, инвариант — «родитель из того же архива».
	rec = postArchiveNodeReq(t, h, owner,
		fmt.Sprintf(`{"type":"delo","archive_id":%q,"parent_id":%q,"label":"Дело из чужого архива"}`, otherArchive.ID, root.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"parent_id"`) {
		t.Fatalf("body = %s, want field=parent_id (родитель из другого архива)", rec.Body)
	}

	// Попытка сменить archive_id узла при обновлении — 422 на поле
	// archive_id: принадлежность архиву неизменна после создания, иначе
	// дочерний узел (child) молча пропал бы из обоих деревьев.
	rec = putArchiveNodeReq(t, h, owner, "/api/archive-nodes/"+string(root.ID),
		fmt.Sprintf(`{"type":"fond","archive_id":%q,"label":"Фонд 1 (испр.)"}`, otherArchive.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"archive_id"`) {
		t.Fatalf("body = %s, want field=archive_id (смена archive_id при обновлении)", rec.Body)
	}

	// Ретрофит: строгий FK sources[i].citation_id.
	src := createSource(t, h, owner,
		`{"kind":"document","title":"Опись фонда","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	cit := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)

	withCitation := createArchiveNode(t, h, owner,
		fmt.Sprintf(`{"type":"fond","archive_id":%q,"label":"Фонд с цитатой","sources":[{"citation_id":%q}]}`, archive.ID, cit.ID),
		http.StatusCreated)
	if len(withCitation.Sources) != 1 || withCitation.Sources[0].CitationID != string(cit.ID) {
		t.Fatalf("withCitation.Sources = %+v, want [{citation_id: %q}]", withCitation.Sources, cit.ID)
	}

	rec = postArchiveNodeReq(t, h, owner,
		fmt.Sprintf(`{"type":"fond","archive_id":%q,"label":"Фонд-призрак","sources":[{"citation_id":"C-01ARZ3NDEKTSV4RRFFQ69G5FA9"}]}`, archive.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"sources[0].citation_id"`) {
		t.Fatalf("body = %s, want field=sources[0].citation_id", rec.Body)
	}

	// Удаление корня, занятого дочерним узлом — 409.
	requireStatusS(t, delReq(t, h, owner, "/api/archive-nodes/"+string(root.ID)), http.StatusConflict)

	// Удаление дочернего узла, затем корня — оба 204.
	requireStatusS(t, delReq(t, h, owner, "/api/archive-nodes/"+string(child.ID)), http.StatusNoContent)
	requireStatusS(t, delReq(t, h, owner, "/api/archive-nodes/"+string(root.ID)), http.StatusNoContent)

	requireStatusS(t, getReq(t, h, "/api/archive-nodes/"+string(root.ID)), http.StatusNotFound)
}

func createArchiveNode(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.ArchiveNode {
	t.Helper()

	rec := postArchiveNodeReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeArchiveNodeS(t, rec)
}

func decodeArchiveNodeS(t *testing.T, rec *httptest.ResponseRecorder) transport.ArchiveNode {
	t.Helper()

	var n transport.ArchiveNode
	if err := json.Unmarshal(rec.Body.Bytes(), &n); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return n
}

func decodeArchiveNodeListS(t *testing.T, rec *httptest.ResponseRecorder) []transport.ArchiveNode {
	t.Helper()

	var list []transport.ArchiveNode
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return list
}

func postArchiveNodeReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/archive-nodes", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putArchiveNodeReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestArchiveDocumentWriteContractWithRealStore: сквозной путь «хранилище →
// сценарии → HTTP» для документов внутри единиц учёта: bootstrap-регистрация
// → создание архива → создание узла → создание документа под этим узлом →
// чтение → изменение → удаление → повторное чтение — 404. Дополнительно
// закрывает строгий FK unit_id (несуществующий — 422 на поле unit_id) и
// sources[i].citation_id (несуществующий — 422).
func TestArchiveDocumentWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:    newDivisionService(t, st),
		Archives:     newArchiveService(t, st),
		ArchiveNodes: newArchiveNodeService(t, st),
		ArchiveDocs:  newArchiveDocumentService(t, st),
		Sources:      newSourceService(t, st),
		Citations:    newCitationService(t, st),
		Auth:         authSvc,
		DocsFS:       fstest.MapFS{},
		TrustProxy:   false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	archive := createArchive(t, h, owner, `{"name":"ГАВО, архив","notes":[],"private":false}`, http.StatusCreated)
	unit := createArchiveNode(t, h, owner,
		fmt.Sprintf(`{"type":"delo","archive_id":%q,"label":"Дело 1"}`, archive.ID), http.StatusCreated)

	created := createArchiveDocument(t, h, owner,
		fmt.Sprintf(`{"unit_id":%q,"title":"Метрическая книга 1890"}`, unit.ID), http.StatusCreated)
	if created.UnitID != unit.ID || created.Title != "Метрическая книга 1890" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "DC-") {
		t.Fatalf("id = %q", created.ID)
	}

	rec := getReq(t, h, "/api/archive-documents/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"title":"Метрическая книга 1890"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	rec = putArchiveDocumentReq(t, h, owner, "/api/archive-documents/"+string(created.ID),
		fmt.Sprintf(`{"unit_id":%q,"title":"Метрическая книга 1890 (испр.)"}`, unit.ID))
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeArchiveDocumentS(t, rec)
	if updated.Title != "Метрическая книга 1890 (испр.)" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	requireStatusS(t, delReq(t, h, owner, "/api/archive-documents/"+string(created.ID)), http.StatusNoContent)
	requireStatusS(t, getReq(t, h, "/api/archive-documents/"+string(created.ID)), http.StatusNotFound)

	// Строгий FK: несуществующий unit_id — 422 на поле unit_id.
	rec = postArchiveDocumentReq(t, h, owner, `{"unit_id":"AN-01ARZ3NDEKTSV4RRFFQ69G5FA9","title":"Документ-призрак"}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"unit_id"`) {
		t.Fatalf("body = %s, want field=unit_id", rec.Body)
	}

	// Ретрофит: строгий FK sources[i].citation_id.
	src := createSource(t, h, owner,
		`{"kind":"document","title":"Метрическая книга","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	cit := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)

	withCitation := createArchiveDocument(t, h, owner,
		fmt.Sprintf(`{"unit_id":%q,"title":"Документ с цитатой","sources":[{"citation_id":%q}]}`, unit.ID, cit.ID),
		http.StatusCreated)
	if len(withCitation.Sources) != 1 || withCitation.Sources[0].CitationID != string(cit.ID) {
		t.Fatalf("withCitation.Sources = %+v, want [{citation_id: %q}]", withCitation.Sources, cit.ID)
	}

	rec = postArchiveDocumentReq(t, h, owner,
		fmt.Sprintf(`{"unit_id":%q,"title":"Документ-призрак 2","sources":[{"citation_id":"C-01ARZ3NDEKTSV4RRFFQ69G5FA9"}]}`, unit.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"sources[0].citation_id"`) {
		t.Fatalf("body = %s, want field=sources[0].citation_id", rec.Body)
	}
}

func createArchiveDocument(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.ArchiveDocument {
	t.Helper()

	rec := postArchiveDocumentReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeArchiveDocumentS(t, rec)
}

func decodeArchiveDocumentS(t *testing.T, rec *httptest.ResponseRecorder) transport.ArchiveDocument {
	t.Helper()

	var d transport.ArchiveDocument
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return d
}

func postArchiveDocumentReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/archive-documents", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putArchiveDocumentReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestNoteWriteContractWithRealStore: сквозной путь «хранилище → сценарии →
// HTTP» (по образцу TestArchiveWriteContractWithRealStore) для заметок:
// bootstrap-регистрация → создание → чтение → изменение → удаление →
// повторное чтение — 404. Дополнительно закрывает self-ref FK
// (Note.ParentID): несуществующий родитель — 422 на поле parent_id;
// переустановка parent_id в цепочку собственных потомков (цикл) — тоже 422
// на поле parent_id; и Fix 1 (приватная запись, анонимный GET — 404).
func TestNoteWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Notes:      newNoteService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание без родителя.
	created := createNote(t, h, owner,
		`{"kind":"note","title":"Заголовок","text":"Текст записи","private":false}`, http.StatusCreated)
	if created.Title != "Заголовок" || created.Text != "Текст записи" || created.ParentID != "" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "N-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id.
	rec := getReq(t, h, "/api/notes/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"text":"Текст записи"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена kind/title/text/parent_id/private.
	rec = putNoteReq(t, h, owner, "/api/notes/"+string(created.ID),
		`{"kind":"article","title":"Заголовок (испр.)","text":"Текст записи","private":false}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeNoteS(t, rec)
	if updated.Kind != "article" || updated.Title != "Заголовок (испр.)" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/notes/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/notes/"+string(created.ID)), http.StatusNotFound)

	// Self-ref FK: валидный parent_id создаётся и сохраняется как есть.
	parent := createNote(t, h, owner, `{"kind":"book","title":"Книга","private":false}`, http.StatusCreated)

	child := createNote(t, h, owner,
		fmt.Sprintf(`{"kind":"chapter","title":"Глава 1","parent_id":%q,"private":false}`, parent.ID), http.StatusCreated)
	if child.ParentID != string(parent.ID) {
		t.Fatalf("child.ParentID = %q, want %q", child.ParentID, parent.ID)
	}

	// Self-ref FK: корректный по формату, но несуществующий parent_id — 422.
	rec = postNoteReq(t, h, owner,
		`{"kind":"note","title":"Сирота","parent_id":"N-01ARZ3NDEKTSV4RRFFQ69G5FA9","private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"parent_id"`) {
		t.Fatalf("body = %s, want field=parent_id", rec.Body)
	}

	// Цикл: A без родителя, B — потомок A, затем A переставляется в потомки B — 422.
	noteA := createNote(t, h, owner, `{"kind":"note","title":"A","private":false}`, http.StatusCreated)
	noteB := createNote(t, h, owner,
		fmt.Sprintf(`{"kind":"note","title":"B","parent_id":%q,"private":false}`, noteA.ID), http.StatusCreated)

	rec = putNoteReq(t, h, owner, "/api/notes/"+string(noteA.ID),
		fmt.Sprintf(`{"kind":"note","title":"A","parent_id":%q,"private":false}`, noteB.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"parent_id"`) {
		t.Fatalf("body = %s, want field=parent_id", rec.Body)
	}

	// Fix 1: приватная запись, анонимный GET — 404, не 200.
	private := createNote(t, h, owner, `{"kind":"note","title":"Приватная","private":true}`, http.StatusCreated)
	if !private.Private {
		t.Fatalf("private = %+v, want Private=true", private)
	}

	requireStatusS(t, getReq(t, h, "/api/notes/"+string(private.ID)), http.StatusNotFound)
}

func createNote(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Note {
	t.Helper()

	rec := postNoteReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeNoteS(t, rec)
}

func decodeNoteS(t *testing.T, rec *httptest.ResponseRecorder) transport.Note {
	t.Helper()

	var n transport.Note
	if err := json.Unmarshal(rec.Body.Bytes(), &n); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return n
}

func postNoteReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/notes", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putNoteReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestAttachmentWriteContractWithRealStore: сквозной путь «хранилище →
// сценарии → HTTP» (по образцу TestArchiveWriteContractWithRealStore) для
// файловых вложений: bootstrap-регистрация → обязательный строгий FK
// node_id (пустой — 422 на поле node_id; корректный по формату, но
// несуществующий — тоже 422 на поле node_id) → Fix 1 (приватная запись,
// анонимный GET — 404). ArchiveNode ещё не имеет своего CRUD-слоя
// (подпроект 6) — сеется напрямую через generic-хранилище вместе с
// Archive, на который он ссылается.
func TestAttachmentWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	ids := idgen.New()
	archiveID := ids.New(models.TypeArchive)
	if err := st.SaveArchive(t.Context(), &models.Archive{ID: archiveID, Name: "ГАВО, архив"}); err != nil {
		t.Fatalf("seed archive: %v", err)
	}

	nodeID := ids.New(models.TypeArchiveNode)
	if err := st.SaveArchiveNode(t.Context(), &models.ArchiveNode{
		ID: nodeID, Type: "fond", ArchiveID: archiveID, Label: "Фонд 1",
	}); err != nil {
		t.Fatalf("seed archive node: %v", err)
	}

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:   newDivisionService(t, st),
		Attachments: newAttachmentService(t, st),
		Auth:        authSvc,
		DocsFS:      fstest.MapFS{},
		TrustProxy:  false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Строгий FK, всегда обязателен: пустой node_id — 422 на поле node_id.
	rec := postAttachmentReq(t, h, owner, `{"kind":"scan","filename":"скан.jpg","private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"node_id"`) {
		t.Fatalf("body = %s, want field=node_id", rec.Body)
	}

	// Строгий FK: корректный по формату, но несуществующий node_id — 422.
	rec = postAttachmentReq(t, h, owner,
		`{"kind":"scan","filename":"скан.jpg","node_id":"AN-01ARZ3NDEKTSV4RRFFQ69G5FA9","private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"node_id"`) {
		t.Fatalf("body = %s, want field=node_id", rec.Body)
	}

	// Fix 1: приватная запись (с настоящим node_id), анонимный GET — 404, не 200.
	private := createAttachment(t, h, owner,
		fmt.Sprintf(`{"kind":"scan","filename":"скан.jpg","node_id":%q,"private":true}`, nodeID), http.StatusCreated)
	if !private.Private {
		t.Fatalf("private = %+v, want Private=true", private)
	}
	if private.NodeID != string(nodeID) {
		t.Fatalf("private.NodeID = %q, want %q", private.NodeID, nodeID)
	}

	requireStatusS(t, getReq(t, h, "/api/attachments/"+string(private.ID)), http.StatusNotFound)
}

func createAttachment(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Attachment {
	t.Helper()

	rec := postAttachmentReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeAttachmentS(t, rec)
}

func decodeAttachmentS(t *testing.T, rec *httptest.ResponseRecorder) transport.Attachment {
	t.Helper()

	var a transport.Attachment
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return a
}

func postAttachmentReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/attachments", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

// TestSourceWriteContractWithRealStore: сквозной путь «хранилище → сценарии
// → HTTP» (по образцу TestArchiveWriteContractWithRealStore) для источников
// доказательств: bootstrap-регистрация → создание без хранилища → чтение →
// изменение → удаление → повторное чтение — 404. Дополнительно: строгий FK
// на Repository (валидный repository_id сохраняется; несуществующий, но
// корректный по формату — 422 на поле repository_id) и Fix 1 (CRITICAL):
// приватная запись, анонимный GET — 404, не 200.
func TestSourceWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:    newDivisionService(t, st),
		Repositories: newRepositoryService(t, st),
		Sources:      newSourceService(t, st),
		Auth:         authSvc,
		DocsFS:       fstest.MapFS{},
		TrustProxy:   false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание без хранилища.
	created := createSource(t, h, owner,
		`{"kind":"document","title":"Метрическая книга","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	if created.Title != "Метрическая книга" || created.RepositoryID != "" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "S-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id.
	rec := getReq(t, h, "/api/sources/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"title":"Метрическая книга"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена kind/title/author/date/reliability/repository_id/notes/private.
	rec = putSourceReq(t, h, owner, "/api/sources/"+string(created.ID),
		`{"kind":"transcription","title":"Метрическая книга (испр.)","reliability":"contemporary","notes":[],"private":false}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeSourceS(t, rec)
	if updated.Title != "Метрическая книга (испр.)" || updated.Kind != "transcription" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/sources/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/sources/"+string(created.ID)), http.StatusNotFound)

	// Строгий FK: валидный repository_id создаётся и сохраняется как есть.
	repo := createRepository(t, h, owner,
		`{"name":"ГАВО","type":"archive","address":"","urls":[],"notes":[],"private":false}`, http.StatusCreated)

	withRepo := createSource(t, h, owner,
		fmt.Sprintf(`{"kind":"document","title":"Дело 1","reliability":"primary","repository_id":%q,"notes":[],"private":false}`, repo.ID),
		http.StatusCreated)
	if withRepo.RepositoryID != string(repo.ID) {
		t.Fatalf("withRepo.RepositoryID = %q, want %q", withRepo.RepositoryID, repo.ID)
	}

	// Строгий FK: корректный по формату, но несуществующий repository_id — 422.
	rec = postSourceReq(t, h, owner,
		`{"kind":"document","title":"Дело-призрак","reliability":"primary","repository_id":"R-01ARZ3NDEKTSV4RRFFQ69G5FA9","notes":[],"private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"repository_id"`) {
		t.Fatalf("body = %s, want field=repository_id", rec.Body)
	}

	// Fix 1: приватная запись, анонимный GET — 404, не 200.
	private := createSource(t, h, owner,
		`{"kind":"memory","title":"Частные воспоминания","reliability":"memory","notes":[],"private":true}`,
		http.StatusCreated)
	if !private.Private {
		t.Fatalf("private = %+v, want Private=true", private)
	}

	requireStatusS(t, getReq(t, h, "/api/sources/"+string(private.ID)), http.StatusNotFound)
}

func createSource(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Source {
	t.Helper()

	rec := postSourceReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeSourceS(t, rec)
}

func decodeSourceS(t *testing.T, rec *httptest.ResponseRecorder) transport.Source {
	t.Helper()

	var s transport.Source
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return s
}

func postSourceReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/sources", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putSourceReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestCitationWriteContractWithRealStore: сквозной путь «хранилище →
// сценарии → HTTP» (по образцу TestArchiveWriteContractWithRealStore) для
// цитат: bootstrap-регистрация → создание источника → создание цитаты без
// якоря → чтение → изменение → удаление → повторное чтение — 404.
// Дополнительно: строгий FK на Source (SourceID всегда обязателен, в
// отличие от Archive.RepositoryID — несуществующий, но корректный по формату
// — 422 на поле source_id), круговорот якоря (ArchiveAnchor — ссылка на
// несуществующий ArchiveNode — 422 на поле anchor.node_id; URLAnchor —
// круговорот без ссылок) и Fix 1 (CRITICAL): приватная запись, анонимный
// GET — 404, не 200.
func TestCitationWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Sources:    newSourceService(t, st),
		Citations:  newCitationService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	src := createSource(t, h, owner,
		`{"kind":"document","title":"Метрическая книга","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)

	// Создание без якоря.
	created := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)
	if created.SourceID != string(src.ID) || created.Anchor != nil {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "C-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id.
	rec := getReq(t, h, "/api/citations/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), fmt.Sprintf(`"source_id":%q`, src.ID)) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена source_id/anchor/text/note/private, всё ещё без якоря.
	rec = putCitationReq(t, h, owner, "/api/citations/"+string(created.ID),
		fmt.Sprintf(`{"source_id":%q,"text":"л. 12 об.","private":false}`, src.ID))
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeCitationS(t, rec)
	if updated.Text != "л. 12 об." || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Строгий FK: корректный по формату, но несуществующий source_id — 422.
	rec = postCitationReq(t, h, owner,
		`{"source_id":"S-01ARZ3NDEKTSV4RRFFQ69G5FA9","private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"source_id"`) {
		t.Fatalf("body = %s, want field=source_id", rec.Body)
	}

	// Круговорот якоря: URLAnchor без ссылок на другие сущности.
	withAnchor := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"anchor":{"kind":"url","url":"https://example.org/page"},"private":false}`, src.ID),
		http.StatusCreated)
	if withAnchor.Anchor == nil || withAnchor.Anchor.Kind != "url" || withAnchor.Anchor.URL != "https://example.org/page" {
		t.Fatalf("withAnchor.Anchor = %+v", withAnchor.Anchor)
	}

	// Якорь со ссылкой на несуществующий ArchiveNode — 422 на поле anchor.node_id.
	rec = postCitationReq(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"anchor":{"kind":"archive","node_id":"AN-01ARZ3NDEKTSV4RRFFQ69G5FA9","page":1},"private":false}`, src.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"anchor.node_id"`) {
		t.Fatalf("body = %s, want field=anchor.node_id", rec.Body)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/citations/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/citations/"+string(created.ID)), http.StatusNotFound)

	// Fix 1: приватная запись, анонимный GET — 404, не 200.
	private := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":true}`, src.ID), http.StatusCreated)
	if !private.Private {
		t.Fatalf("private = %+v, want Private=true", private)
	}

	requireStatusS(t, getReq(t, h, "/api/citations/"+string(private.ID)), http.StatusNotFound)
}

func createCitation(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Citation {
	t.Helper()

	rec := postCitationReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeCitationS(t, rec)
}

func decodeCitationS(t *testing.T, rec *httptest.ResponseRecorder) transport.Citation {
	t.Helper()

	var c transport.Citation
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return c
}

func postCitationReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/citations", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putCitationReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestFamilyWriteContractWithRealStore: сквозной путь «хранилище → сценарии
// → HTTP» (по образцу TestRepositoryWriteContractWithRealStore, структурно
// ближайшего шаблона подпроекта 7 — Family отличается от Repository
// отсутствием Type/Address и переименованием URLs → Members) для родов:
// bootstrap-регистрация → создание → чтение → изменение → удаление →
// повторное чтение — 404. Дополнительно закрывает: приватность (Fix 1),
// проверяемая явно через ОБА пути (create И update — этот дефект уже
// повторялся дважды в программе, подпроекты 2 и 4); строгий FK
// sources[i].citation_id; и то, что members — мягкая ссылка на Person (без
// CRUD, подпроект 8) — ref/type round-trip'ятся как обычный TextRef, без
// проверки существования.
func TestFamilyWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Families:   newFamilyService(t, st),
		Sources:    newSourceService(t, st),
		Citations:  newCitationService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание.
	created := createFamily(t, h, owner,
		`{"name":"Ивановы","members":[],"notes":[],"private":false}`, http.StatusCreated)
	if created.Name != "Ивановы" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "F-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id — открыто анонимному посетителю.
	rec := getReq(t, h, "/api/families/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"name":"Ивановы"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена name/members/notes/sources/private.
	rec = putFamilyReq(t, h, owner, "/api/families/"+string(created.ID),
		`{"name":"Ивановы (испр.)","members":[],"notes":[],"private":false}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeFamilyS(t, rec)
	if updated.Name != "Ивановы (испр.)" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/families/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/families/"+string(created.ID)), http.StatusNotFound)

	// Fix 1 (CRITICAL, повторялась дважды — подпроекты 2 и 4): private
	// должен пережить и create, и update, а не только прямое сохранение.
	// Сначала create с private:true.
	private := createFamily(t, h, owner,
		`{"name":"Приватный род","members":[],"notes":[],"private":true}`, http.StatusCreated)
	if !private.Private {
		t.Fatalf("private (после create) = %+v, want Private=true", private)
	}
	requireStatusS(t, getReq(t, h, "/api/families/"+string(private.ID)), http.StatusNotFound)

	// Теперь update с private:true у записи, созданной как публичная.
	rec = putFamilyReq(t, h, owner, "/api/families/"+string(private.ID),
		`{"name":"Приватный род","members":[],"notes":[],"private":true}`)
	requireStatusS(t, rec, http.StatusOK)
	afterUpdate := decodeFamilyS(t, rec)
	if !afterUpdate.Private {
		t.Fatalf("private (после update) = %+v, want Private=true", afterUpdate)
	}
	requireStatusS(t, getReq(t, h, "/api/families/"+string(private.ID)), http.StatusNotFound)

	// members — мягкая ссылка на Person (без CRUD, подпроект 8):
	// ref/type сохраняются как обычный TextRef, существование не проверяется.
	withMember := createFamily(t, h, owner,
		`{"name":"Петровы","members":[{"text":"Пётр Петров","ref":"I-01ARZ3NDEKTSV4RRFFQ69G5FA9","type":"person"}],"notes":[],"private":false}`,
		http.StatusCreated)
	if len(withMember.Members) != 1 || withMember.Members[0].Ref != "I-01ARZ3NDEKTSV4RRFFQ69G5FA9" {
		t.Fatalf("withMember.Members = %+v", withMember.Members)
	}

	// Ретрофит-паттерн (по образцу TestArchiveWriteContractWithRealStore):
	// строгий FK sources[i].citation_id — сквозная проверка через реальный
	// HTTP-хендлер → сценарий → SQLite.
	src := createSource(t, h, owner,
		`{"kind":"document","title":"Ревизская сказка","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	cit := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)

	withCitation := createFamily(t, h, owner,
		fmt.Sprintf(`{"name":"Сидоровы","members":[],"notes":[],"sources":[{"citation_id":%q}],"private":false}`, cit.ID),
		http.StatusCreated)
	if len(withCitation.Sources) != 1 || withCitation.Sources[0].CitationID != string(cit.ID) {
		t.Fatalf("withCitation.Sources = %+v, want [{citation_id: %q}]", withCitation.Sources, cit.ID)
	}

	// Строгий FK: корректный по формату, но несуществующий citation_id — 422 на sources[0].citation_id.
	rec = postFamilyReq(t, h, owner,
		`{"name":"Призрачный род","members":[],"notes":[],"sources":[{"citation_id":"C-01ARZ3NDEKTSV4RRFFQ69G5FA9"}],"private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"sources[0].citation_id"`) {
		t.Fatalf("body = %s, want field=sources[0].citation_id", rec.Body)
	}
}

func createFamily(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Family {
	t.Helper()

	rec := postFamilyReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeFamilyS(t, rec)
}

func decodeFamilyS(t *testing.T, rec *httptest.ResponseRecorder) transport.Family {
	t.Helper()

	var f transport.Family
	if err := json.Unmarshal(rec.Body.Bytes(), &f); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return f
}

func postFamilyReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/families", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putFamilyReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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
```

### 1.7. Документация: `docs/usage.md`, `docs/architecture.md`, `CHANGELOG.md`, `entity-write.md` §3.6

По правилу, закреплённому после подпроекта 4 (доки/CHANGELOG пишутся в плане, а не оставляются на финальное ревью) — ниже точные вставки/замены в уже существующие большие файлы; порядок существующих строк не меняется, только добавляются/заменяются указанные фрагменты. Все четыре файла входят в Задачу 1 целиком и самодостаточны на докам — Задача 2 доки не трогает (см. «Предпосылка»: контрактные доки `usage.md`/`architecture.md` по конвенции репозитория не описывают веб-страницы вовсе, `CHANGELOG.md` описывает и бэкенд, и веб в одном пункте, см. фрагмент ниже).

#### `docs/usage.md` — новая строка HTTP-таблицы (после `/api/archives`, до `/api/notes`)
Вставить после строки `/api/archives` и перед строкой `/api/notes`:
```markdown
| `/api/families` | Роды/линии (JSON): `GET` — список `[{"id", "name", "members", "notes", "sources", "private"}]`; `POST` — создание; `GET/PUT/DELETE /api/families/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404, как отсутствующая); `GET /api/families/search?q=` — поиск по началу названия. `members` — мягкая ссылка на `Person` (`TypePerson`, у которого ещё нет CRUD, подпроект 8): `TextRef.Ref`, если задан, только проверяется по формату, существование не проверяется — тот же принцип, что и у любого другого списка `TextRef` в программе. |
```

#### `docs/usage.md` — дополнение к абзацу про `sources` (после HTTP-таблицы)
Заменить предложение про `ArchiveNode`/`ArchiveDocument` в конце существующего абзаца про `sources`:
```markdown
`/api/archive-nodes`/`/api/archive-documents` (подпроект 6) — первые полностью новые сущности программы с `sources`, редактируемым с самого начала (не ретрофит).
```
на:
```markdown
`/api/archive-nodes`/`/api/archive-documents` (подпроект 6) и `/api/families` (подпроект 7) — первые полностью новые сущности программы с `sources`, редактируемым с самого начала (не ретрофит).
```

#### `docs/usage.md` — 6 новых строк таблицы MCP-тулов (после `citation_delete`, до абзаца про `sources`-аргумент)
```markdown
| `family_list` | Список родов/линий; аргументы `limit`, `offset` |
| `family_search` | Поиск родов по началу названия; аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `family_get` | Запись по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула (как отсутствующая) |
| `family_create` | Создание записи: `name` (обязателен), `members`, `notes` (тексты; `members` — мягкая ссылка на персону, без проверки существования), `sources`, `private`; id генерирует сервер |
| `family_update` | Изменение записи: полная замена `name`/`members`/`notes`/`private`; `members`/`notes` принимают только текст (без ref/type в MCP-контракте) — ссылка на элементе теряется при любом обновлении, пока не появится picker; `sources` — при отсутствии в вызове текущие источники сохраняются, пустой массив — очищает их |
| `family_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
```

#### `docs/architecture.md` — строка `internal/httpapi/`
Заменить:
```markdown
| `internal/httpapi/` | Слой HTTP API: JSON-роуты `/api/*` (health, admin-divisions, surnames, patronymics, estates, titles, given-names, repositories, churches, parishes, archives, archive-nodes, archive-documents, notes, attachments, sources, citations, auth, docs). Использует те же сценарии, что и MCP. |
```
на:
```markdown
| `internal/httpapi/` | Слой HTTP API: JSON-роуты `/api/*` (health, admin-divisions, surnames, patronymics, estates, titles, given-names, repositories, churches, parishes, archives, archive-nodes, archive-documents, notes, attachments, sources, citations, families, auth, docs). Использует те же сценарии, что и MCP. |
```

#### `CHANGELOG.md` — новый пункт (после пункта про ретрофит Attachment/AnchorEditor, перед «Веб: единая точка входа»)
Вставить перед пунктом «Веб: единая точка входа»:
```markdown
- Роды/линии (`Family`) — подпроект 7, полный стек: usecase-сценарии, HTTP
  (`/api/families*`) и MCP (`family_list/search/get/create/update/delete`).
  Самая механическая сущность программы на сегодня — структурно та же
  форма, что `Repository` (подпроект 3) минус `Type`/`Address`, с `URLs`,
  переименованным в `Members`; ни одного нового паттерна не потребовалось.
  `Members` — мягкая ссылка на `Person` (`TypePerson`, без CRUD до
  подпроекта 8): `TextRef.Ref` не проверяется на существование при
  сохранении, как и у любого другого списка `TextRef` в программе.
  `Sources` редактируемый с рождения контракта (как `ArchiveNode`/
  `ArchiveDocument` из подпроекта 6), не ретрофит. Веб: страница «Роды» —
  список/просмотр/редактирование/создание с `TextRefListEditor` для
  `Members`/`Notes` и `SourceLinkListEditor` для `Sources`; строка «Роды» в
  каталоге сущностей (`internal/transport/family{,_write}.go`,
  `internal/usecases/{list,search,get,create,update,delete}_family*`
  (пакеты `list_families`/`search_families`/`get_family`/`create_family`/
  `update_family`/`delete_family`), `internal/httpapi/family{,_write}.go`,
  `internal/mcp/family.go`, `web/src/pages/{FamiliesList,FamilyForm,
  FamilyView}.tsx`, `web/src/App.tsx`, `web/src/pages/EntityCatalog.tsx`).
```

#### `docs/data-model/entity-write.md` — новая секция §3.6 (после §3.5, до `## 4.`)
`docs/data-model/entity-write.md (фрагмент — новая секция §3.6)`:
```markdown
### 3.6. Подпроект 7 (`Family`) — без новых паттернов

- **Самая механическая сущность программы на сегодня.** `Family` —
  `{ID, Name, Members []TextRef, Notes []TextRef, Sources []SourceLink,
  Private bool}` — структурно та же форма, что `Repository` (подпроект 3)
  минус `Type`/`Address`, с `URLs`, переименованным в `Members`. Весь стек
  (usecase-сценарии, транспорт, `httpapi`, MCP, веб) скопирован с
  `Repository` file-for-file, без единого нового решения дизайна: нет
  своего FK (только `Sources[i].CitationID`, проверяемый в транзакции по
  образцу `create_repository`/`update_repository`), нет self-ref, нет
  полиморфного типа.
- **`Members` — мягкая ссылка на `Person`, `TypePerson`.** `Person` не
  получит CRUD до подпроекта 8 — как и `ArchiveNode`/`ArchiveDocument` до
  подпроекта 6 (§3.2), это не мешает `Family` ссылаться на него: `TextRef`
  для мягких ссылок никогда не проверяется на существование при
  сохранении, независимо от того, есть ли у цели свой CRUD-слой — этот
  принцип действует с самого первого появления `TextRef` в программе, не
  специфика `Family`. Веб-форма редактирует `Members` тем же
  `TextRefListEditor`, что и `Repository.URLs`/любой другой список
  `TextRef` — без «пикера по персоне» (см. §5 — picker для `TextRef.Ref`
  остаётся вне объёма программы до подпроекта 9).
- **`Sources` редактируемый с рождения, не ретрофит** — как и у
  `ArchiveNode`/`ArchiveDocument` (подпроект 6, §3.4): `Family` заведена
  уже после того, как `Citation` получил CRUD (подпроект 5), поэтому её
  Create/Update DTO несут `sources` с первого дня.
- **Поисковый индекс — только `name`** (`replaceSearchIndex(tx, "families",
  f.ID, map[string][]string{"name": {f.Name}})`,
  `internal/store/sqlstore/relations.go`) — `members`/`notes` не
  индексируются. Формулировки MCP-описаний/doc-комментариев/веб-плейсхолдеров
  («поиск по началу названия») сверены с этим списком буквально — тот же
  урок, что и в §3.2 (найдено финальным ревью подпроекта 4, с тех пор
  проверяется явно при каждом новом подпроекте).
```

### 1.8. Рубеж

`gofmt -l .` пусто; `go build ./...`, `go vet ./...`, `go test ./...` (весь репозиторий) зелёные — ожидается 1252 теста, 118 пакетов (было 1216 после подпроекта 6). Живой смок через `curl` (см. «Предпосылка») — не обязателен повторно: создать `Family` с `private:false`, отдельно с `private:true`, проверить `GET` анонимно (404 для приватной) и от владельца (200); `PUT` в обе стороны `private:false→true→false`; создать `Source`→`Citation`, затем `Family`, ссылающийся на неё (round-trip), затем `Family` с несуществующим `citation_id` (422 на `sources[0].citation_id`); `DELETE` → 204, повторный `GET` → 404.

### 1.9. Коммит

```bash
git add \
  internal/transport/family.go internal/transport/family_write.go \
  internal/usecases/list_families internal/usecases/search_families internal/usecases/get_family internal/usecases/create_family internal/usecases/update_family internal/usecases/delete_family \
  internal/httpapi/family.go internal/httpapi/family_write.go internal/httpapi/family_test.go internal/httpapi/family_write_test.go \
  internal/mcp/family.go internal/mcp/family_test.go \
  internal/httpapi/deps.go internal/httpapi/api.go internal/httpapi/httpapi.go internal/mcp/deps.go internal/mcp/server.go internal/app/app.go \
  internal/httpapi/write_store_test.go internal/httpapi/store_test.go \
  docs/usage.md docs/architecture.md CHANGELOG.md docs/data-model/entity-write.md
git commit -m "feat(backend): Family — полный CRUD (usecases/httpapi/mcp) + доки + real-store тесты"
```

## Задача 2. Веб: страницы Family + роуты/каталог

**Интерфейсы, потребляемые из Задачи 1**: HTTP-контракт `/api/families*` (Шаги 1.3, 1.5).
**Интерфейсы, потребляемые с веба подпроектов 1-6**: `TextRefListEditor`, `SourceLinkListEditor` (переиспользуемые редакторы списков `TextRef`/`SourceLink`, используются уже `RepositoryForm`/`RepositoryView` и другими страницами), общий каркас страницы-тройки (List/View+Form) по образцу `RepositoriesList.tsx`/`RepositoryForm.tsx`/`RepositoryView.tsx`.
**Производит**: страницы `/families`, `/families/:id`, строку «Роды» в каталоге сущностей.

**Файлы:**
- Изменить: `web/src/api.ts`, `web/src/App.tsx`, `web/src/pages/EntityCatalog.tsx`
- Создать: `web/src/pages/{FamiliesList,FamilyForm,FamilyView}.tsx`

**Важно для исполнителя**: копия страниц `Repository` без нового компонента — `FamiliesList.tsx` (плоский список, без дерева), `FamilyForm.tsx` (`CreateFamilyModal`, поля `name`/`members`/`notes`/`sources`/`private`), `FamilyView.tsx` (просмотр + инлайн-редактирование, тот же паттерн `editing`-state, что у `RepositoryView`). Переносить код ниже как есть, файл за файлом — итоговое содержимое.

### 2.1. Страницы Family

#### `web/src/pages/FamiliesList.tsx` (создать)
```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Input, List, Spin } from "antd";
import { fetchFamilies, searchFamilies, MAX_PAGE_LIMIT, type Family } from "../api";
import { useSession } from "../session";
import { CreateFamilyModal } from "./FamilyForm";

// FamiliesList — «Роды»: плоский список (сущность не иерархична, в отличие
// от AdminDivision — без Tree), пагинация по offset до короткой страницы
// (тот же приём, что DivisionsList.loadRoot — API уже отдаёт окнами,
// docs/data-model/entity-write.md §4). Поиск по названию временно подменяет
// список найденным. Клик по строке — переход на View.
export default function FamiliesList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<Family[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<Family[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: Family[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchFamilies({ limit: MAX_PAGE_LIMIT, offset });
        all.push(...page);
        if (page.length < MAX_PAGE_LIMIT) {
          break;
        }
        offset += MAX_PAGE_LIMIT;
      }
      setItems(all);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Не удалось загрузить список");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const onSearch = (value: string) => {
    const q = value.trim();
    if (!q) {
      setSearchResults(null);
      return;
    }
    setSearching(true);
    setError(null);
    searchFamilies({ q, limit: MAX_PAGE_LIMIT })
      .then(setSearchResults)
      .catch((e: Error) => setError(e.message))
      .finally(() => setSearching(false));
  };

  const onSearchChange = (value: string) => {
    if (value.trim() === "") {
      setSearchResults(null);
    }
  };

  const shown = searchResults ?? items;

  return (
    <>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "Роды" }]}
      />
      <Card
        title="Роды"
        extra={
          session != null ? (
            <Button type="primary" onClick={() => setCreateOpen(true)}>
              + добавить
            </Button>
          ) : undefined
        }
      >
        <Input.Search
          placeholder="Поиск по названию…"
          allowClear
          enterButton
          loading={searching}
          onSearch={onSearch}
          onChange={(e) => onSearchChange(e.target.value)}
          style={{ marginBottom: 16 }}
        />
        {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
        {loading && searchResults == null ? (
          <Spin />
        ) : (
          <List
            dataSource={shown}
            locale={{ emptyText: "Список пуст" }}
            renderItem={(f) => (
              <List.Item>
                <Link to={`/families/${f.id}`}>{f.name}</Link>
              </List.Item>
            )}
          />
        )}
        <CreateFamilyModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(f) => {
            setCreateOpen(false);
            navigate(`/families/${f.id}`);
          }}
        />
      </Card>
    </>
  );
}
```
#### `web/src/pages/FamilyForm.tsx` (создать)
```tsx
import { useState } from "react";
import { Alert, Checkbox, Form, Input, Modal } from "antd";
import { createFamily, type Family, type SourceLink, type TextRef } from "../api";
import { ApiError } from "../auth";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";

interface FamilyFormValues {
  name: string;
  private?: boolean;
}

const FORM_FIELDS: (keyof FamilyFormValues)[] = ["name"];

// CreateFamilyModal — форма создания рода/линии. Структурно почти идентична
// CreateRepositoryModal (подпроект 3): та же сущность минус Type/Address,
// Members — тот же TextRefListEditor, что URLs у Repository (мягкая
// ссылка на Person, без picker'а — у Person нет CRUD, подпроект 8,
// docs/data-model/entity-write.md). Sources редактируется с рождения
// контракта (SourceLinkListEditor), а не ретрофитом.
export function CreateFamilyModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Family) => void;
}) {
  const [form] = Form.useForm<FamilyFormValues>();
  const [members, setMembers] = useState<TextRef[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setMembers([]);
    setNotes([]);
    setSources([]);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: FamilyFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await createFamily({
        name: values.name,
        members,
        notes,
        sources,
        private: values.private ?? false,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof FamilyFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить род"
      open={open}
      onCancel={handleClose}
      onOk={() => form.submit()}
      okText="Создать"
      cancelText="Отмена"
      confirmLoading={submitting}
      destroyOnHidden
    >
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
      <Form form={form} layout="vertical" onFinish={onFinish} initialValues={{ private: false }}>
        <Form.Item
          name="name"
          label="Название"
          rules={[{ required: true, whitespace: true, message: "Введите название" }]}
        >
          <Input autoFocus />
        </Form.Item>
        <Form.Item label="Члены рода">
          <TextRefListEditor value={members} onChange={setMembers} addLabel="+ член рода" />
        </Form.Item>
        <Form.Item label="Заметки">
          <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
        </Form.Item>
        <Form.Item label="Доказательства">
          <SourceLinkListEditor value={sources} onChange={setSources} addLabel="+ доказательство" />
        </Form.Item>
        <Form.Item name="private" valuePropName="checked">
          <Checkbox>Приватная запись</Checkbox>
        </Form.Item>
      </Form>
    </Modal>
  );
}
```
#### `web/src/pages/FamilyView.tsx` (создать)
```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  Alert,
  Breadcrumb,
  Button,
  Card,
  Checkbox,
  Descriptions,
  Form,
  Input,
  List,
  Modal,
  Popconfirm,
  Space,
  Spin,
  Typography,
} from "antd";
import {
  deleteFamily,
  fetchFamily,
  updateFamily,
  type Family,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";

interface EditFormValues {
  name: string;
  private?: boolean;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["name"];

function TextRefListView({ items }: { items: TextRef[] }) {
  if (items.length === 0) {
    return <Typography.Text type="secondary">—</Typography.Text>;
  }
  return (
    <Space direction="vertical" size={0}>
      {items.map((t, i) => (
        <Typography.Text key={i}>
          {t.text}
          {t.ref != null && t.ref !== "" && (
            <Typography.Text type="secondary"> → {t.type} {t.ref}</Typography.Text>
          )}
        </Typography.Text>
      ))}
    </Space>
  );
}

// SourceLinkListView — read-only отображение списка доказательств (Sources)
// в режиме просмотра; редактируется отдельным SourceLinkListEditor в форме
// ниже (Citation имеет CRUD с подпроекта 5, docs/data-model/entity-write.md §3.3).
function SourceLinkListView({ items }: { items: SourceLink[] }) {
  if (items.length === 0) {
    return <Typography.Text type="secondary">—</Typography.Text>;
  }
  return (
    <List
      size="small"
      dataSource={items}
      renderItem={(s) => (
        <List.Item>
          <Link to={`/citations/${s.citation_id}`}>citation {s.citation_id}</Link>
          {s.role ? ` — ${s.role}` : ""}
        </List.Item>
      )}
    />
  );
}

// FamilyView — просмотр рода, переключаемый в форму редактирования на той
// же странице (toggle+explicit-save — PUT заменяет запись целиком,
// docs/data-model/entity-write.md §4). Без родителя/детей/дерева — плоская
// сущность, структурно почти идентична RepositoryView минус Type/Address.
export default function FamilyView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [family, setFamily] = useState<Family | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [members, setMembers] = useState<TextRef[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (familyId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setFamily(null);
    fetchFamily(familyId)
      .then(setFamily)
      .catch((e) => {
        if (e instanceof ApiError && e.status === 404) {
          setNotFound(true);
        } else {
          setError(e instanceof Error ? e.message : "Не удалось загрузить запись");
        }
      })
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    setEditing(false);
    setSaveError(null);
    if (id != null) {
      load(id);
    }
  }, [id]);

  const startEdit = () => {
    if (family == null) {
      return;
    }
    form.setFieldsValue({
      name: family.name,
      private: family.private,
    });
    setMembers(family.members);
    setNotes(family.notes);
    setSources(family.sources);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (family == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await updateFamily(family.id, {
        name: values.name,
        members,
        notes,
        sources,
        private: values.private ?? false,
      });
      setFamily(updated);
      setEditing(false);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (EDIT_FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof EditFormValues, errors: [e.message] }]);
      } else {
        setSaveError(e instanceof ApiError ? e.message : "Не удалось сохранить изменения");
      }
    } finally {
      setSaving(false);
    }
  };

  const onDelete = async () => {
    if (family == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteFamily(family.id);
      navigate("/families");
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        setConflict(e.referrers ?? []);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось удалить запись");
      }
    } finally {
      setDeleting(false);
    }
  };

  if (loading) {
    return <Spin />;
  }

  if (notFound) {
    return (
      <Alert
        type="warning"
        showIcon
        message="Запись не найдена"
        description="Возможно, её удалили. Вернитесь к списку."
        action={
          <Link to="/families">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (family == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/families">Роды</Link> },
          { title: family.name },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={family.name} column={1} bordered size="small">
            <Descriptions.Item label="Члены рода"><TextRefListView items={family.members} /></Descriptions.Item>
            <Descriptions.Item label="Заметки"><TextRefListView items={family.notes} /></Descriptions.Item>
            <Descriptions.Item label="Доказательства"><SourceLinkListView items={family.sources} /></Descriptions.Item>
            <Descriptions.Item label="Приватная">{family.private ? "да" : "нет"}</Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title={`Удалить «${family.name}»?`}
                description="Действие необратимо."
                okText="Удалить"
                cancelText="Отмена"
                onConfirm={onDelete}
              >
                <Button danger loading={deleting}>
                  Удалить
                </Button>
              </Popconfirm>
            </Space>
          )}
        </>
      ) : (
        <Form form={form} layout="vertical" onFinish={onSave} style={{ maxWidth: 480 }}>
          {saveError != null && (
            <Alert type="error" showIcon message={saveError} style={{ marginBottom: 16 }} />
          )}
          <Form.Item
            name="name"
            label="Название"
            rules={[{ required: true, whitespace: true, message: "Введите название" }]}
          >
            <Input />
          </Form.Item>
          <Form.Item label="Члены рода">
            <TextRefListEditor value={members} onChange={setMembers} addLabel="+ член рода" />
          </Form.Item>
          <Form.Item label="Заметки">
            <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
          </Form.Item>
          <Form.Item label="Доказательства">
            <SourceLinkListEditor value={sources} onChange={setSources} addLabel="+ доказательство" />
          </Form.Item>
          <Form.Item name="private" valuePropName="checked">
            <Checkbox>Приватная запись</Checkbox>
          </Form.Item>
          <Space>
            <Button type="primary" htmlType="submit" loading={saving}>
              Сохранить
            </Button>
            <Button onClick={cancelEdit}>Отмена</Button>
          </Space>
        </Form>
      )}

      <Modal
        title="Запись используется"
        open={conflict != null}
        onCancel={() => setConflict(null)}
        footer={<Button onClick={() => setConflict(null)}>Закрыть</Button>}
      >
        <Typography.Paragraph>
          Нельзя удалить — на запись ссылаются другие сущности:
        </Typography.Paragraph>
        <List
          size="small"
          dataSource={conflict ?? []}
          renderItem={(r) => (
            <List.Item>
              {r.type} {r.id}
            </List.Item>
          )}
        />
      </Modal>
    </Card>
  );
}
```

### 2.2. `api.ts`, `App.tsx`, `EntityCatalog.tsx` — типы/функции, роуты, каталог

#### `web/src/api.ts` (изменить — итоговое содержимое)
```typescript
import { authFetch } from "./auth";

export interface AdminDivision {
  id: string;
  name: string;
  type: string;
  parent_id: string | null;
  sources: SourceLink[];
}

// AdminDivisionType — models.AdminDivisionType (internal/models/administrative_division_type.go).
export type AdminDivisionType =
  | "governorate"
  | "district"
  | "volost"
  | "other"
  | "gorod"
  | "selo"
  | "derevnya"
  | "hutor"
  | "pogost"
  | "stanitsa"
  | "mestechko";

export const ADMIN_DIVISION_TYPE_LABELS: Record<AdminDivisionType, string> = {
  governorate: "Губерния",
  district: "Уезд",
  volost: "Волость",
  other: "Иное",
  gorod: "Город",
  selo: "Село",
  derevnya: "Деревня",
  hutor: "Хутор",
  pogost: "Погост",
  stanitsa: "Станица",
  mestechko: "Местечко",
};

export function adminDivisionTypeLabel(t: string): string {
  return ADMIN_DIVISION_TYPE_LABELS[t as AdminDivisionType] ?? t;
}

// AdminDivisionInput — тело POST/PUT /api/admin-divisions (transport.AdminDivisionCreate
// и transport.AdminDivisionUpdate имеют одинаковую форму: полная замена name/type/parent_id;
// с подпроекта 5 сюда же входит sources — тоже полная замена).
export interface AdminDivisionInput {
  name: string;
  type: AdminDivisionType;
  parent_id: string | null;
  sources: SourceLink[];
}

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

// Максимальный размер окна списка (совпадает с MaxPageLimit на сервере).
export const MAX_PAGE_LIMIT = 500;

export async function fetchAdminDivisions(
  query: AdminDivisionQuery = {},
): Promise<AdminDivision[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/admin-divisions${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
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

// fetchDivision — GET /api/admin-divisions/{id}, открыто анонимному
// посетителю (auth.md §6). Через authFetch — ради ApiError (нужен код 404
// на странице View: единицу могли удалить в другой вкладке).
export async function fetchDivision(id: string): Promise<AdminDivision> {
  return authFetch<AdminDivision>(`/api/admin-divisions/${encodeURIComponent(id)}`);
}

// createDivision/updateDivision/deleteDivision — запись, только для
// вошедшего владельца (requireFull на сервере, internal/httpapi/division_write.go).
export async function createDivision(input: AdminDivisionInput): Promise<AdminDivision> {
  return authFetch<AdminDivision>("/api/admin-divisions", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateDivision(id: string, input: AdminDivisionInput): Promise<AdminDivision> {
  return authFetch<AdminDivision>(`/api/admin-divisions/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteDivision(id: string): Promise<void> {
  return authFetch<void>(`/api/admin-divisions/${encodeURIComponent(id)}`, { method: "DELETE" });
}

// TextRef — контракт элемента списков вроде Surname.variants: текст или
// ссылка на другую сущность (transport.TextRef). v1-формы редактируют
// только text; ref/type только читаются (docs/data-model/entity-write.md §4).
export interface TextRef {
  text: string;
  ref?: string;
  type?: string;
}

export interface Surname {
  id: string;
  canonical: string;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

// SurnameInput — тело POST/PUT /api/surnames (transport.SurnameCreate и
// transport.SurnameUpdate имеют одинаковую форму: полная замена всех полей).
export interface SurnameInput {
  canonical: string;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

export interface SurnameQuery {
  limit?: number;
  offset?: number;
}

export interface SurnameSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchSurnames(query: SurnameQuery = {}): Promise<Surname[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/surnames${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchSurnames(query: SurnameSearchQuery): Promise<Surname[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/surnames/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

// fetchSurname — GET /api/surnames/{id}, открыто анонимному посетителю. Через
// authFetch — ради ApiError (нужен код 404 на странице View).
export async function fetchSurname(id: string): Promise<Surname> {
  return authFetch<Surname>(`/api/surnames/${encodeURIComponent(id)}`);
}

// createSurname/updateSurname/deleteSurname — запись, только для вошедшего
// владельца (requireFull на сервере, internal/httpapi/surname_write.go).
export async function createSurname(input: SurnameInput): Promise<Surname> {
  return authFetch<Surname>("/api/surnames", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateSurname(id: string, input: SurnameInput): Promise<Surname> {
  return authFetch<Surname>(`/api/surnames/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteSurname(id: string): Promise<void> {
  return authFetch<void>(`/api/surnames/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export interface DocFile {
  path: string;
  title: string;
}

export interface DocListResponse {
  files: DocFile[];
}

export async function fetchDocList(): Promise<DocFile[]> {
  const resp = await fetch("/api/docs");
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  const data: DocListResponse = await resp.json();
  return data.files;
}

export async function fetchDoc(path: string): Promise<string> {
  const resp = await fetch(`/api/docs/${encodeURIComponent(path)}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.text();
}

export interface Patronymic {
  id: string;
  canonical: string;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

// PatronymicInput — тело POST/PUT /api/patronymics (transport.PatronymicCreate и
// transport.PatronymicUpdate имеют одинаковую форму: полная замена всех полей).
export interface PatronymicInput {
  canonical: string;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

export interface PatronymicQuery {
  limit?: number;
  offset?: number;
}

export interface PatronymicSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchPatronymics(query: PatronymicQuery = {}): Promise<Patronymic[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/patronymics${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchPatronymics(query: PatronymicSearchQuery): Promise<Patronymic[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/patronymics/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

// fetchPatronymic — GET /api/patronymics/{id}, открыто анонимному посетителю. Через
// authFetch — ради ApiError (нужен код 404 на странице View).
export async function fetchPatronymic(id: string): Promise<Patronymic> {
  return authFetch<Patronymic>(`/api/patronymics/${encodeURIComponent(id)}`);
}

// createPatronymic/updatePatronymic/deletePatronymic — запись, только для вошедшего
// владельца (requireFull на сервере, internal/httpapi/patronymic_write.go).
export async function createPatronymic(input: PatronymicInput): Promise<Patronymic> {
  return authFetch<Patronymic>("/api/patronymics", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updatePatronymic(id: string, input: PatronymicInput): Promise<Patronymic> {
  return authFetch<Patronymic>(`/api/patronymics/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deletePatronymic(id: string): Promise<void> {
  return authFetch<void>(`/api/patronymics/${encodeURIComponent(id)}`, { method: "DELETE" });
}


export interface Estate {
  id: string;
  canonical: string;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

// EstateInput — тело POST/PUT /api/estates (transport.EstateCreate и
// transport.EstateUpdate имеют одинаковую форму: полная замена всех полей).
export interface EstateInput {
  canonical: string;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

export interface EstateQuery {
  limit?: number;
  offset?: number;
}

export interface EstateSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchEstates(query: EstateQuery = {}): Promise<Estate[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/estates${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchEstates(query: EstateSearchQuery): Promise<Estate[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/estates/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

// fetchEstate — GET /api/estates/{id}, открыто анонимному посетителю. Через
// authFetch — ради ApiError (нужен код 404 на странице View).
export async function fetchEstate(id: string): Promise<Estate> {
  return authFetch<Estate>(`/api/estates/${encodeURIComponent(id)}`);
}

// createEstate/updateEstate/deleteEstate — запись, только для вошедшего
// владельца (requireFull на сервере, internal/httpapi/estate_write.go).
export async function createEstate(input: EstateInput): Promise<Estate> {
  return authFetch<Estate>("/api/estates", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateEstate(id: string, input: EstateInput): Promise<Estate> {
  return authFetch<Estate>(`/api/estates/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteEstate(id: string): Promise<void> {
  return authFetch<void>(`/api/estates/${encodeURIComponent(id)}`, { method: "DELETE" });
}


export interface Title {
  id: string;
  canonical: string;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

// TitleInput — тело POST/PUT /api/titles (transport.TitleCreate и
// transport.TitleUpdate имеют одинаковую форму: полная замена всех полей).
export interface TitleInput {
  canonical: string;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

export interface TitleQuery {
  limit?: number;
  offset?: number;
}

export interface TitleSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchTitles(query: TitleQuery = {}): Promise<Title[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/titles${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchTitles(query: TitleSearchQuery): Promise<Title[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/titles/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

// fetchTitle — GET /api/titles/{id}, открыто анонимному посетителю. Через
// authFetch — ради ApiError (нужен код 404 на странице View).
export async function fetchTitle(id: string): Promise<Title> {
  return authFetch<Title>(`/api/titles/${encodeURIComponent(id)}`);
}

// createTitle/updateTitle/deleteTitle — запись, только для вошедшего
// владельца (requireFull на сервере, internal/httpapi/title_write.go).
export async function createTitle(input: TitleInput): Promise<Title> {
  return authFetch<Title>("/api/titles", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateTitle(id: string, input: TitleInput): Promise<Title> {
  return authFetch<Title>(`/api/titles/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteTitle(id: string): Promise<void> {
  return authFetch<void>(`/api/titles/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export type NameGender = "male" | "female" | "neutral";

export interface GivenName {
  id: string;
  canonical: string;
  gender: NameGender | "";
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

// GivenNameInput — тело POST/PUT /api/given-names (transport.GivenNameCreate и
// transport.GivenNameUpdate имеют одинаковую форму: полная замена всех полей).
export interface GivenNameInput {
  canonical: string;
  gender: NameGender;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

export interface GivenNameQuery {
  limit?: number;
  offset?: number;
}

export interface GivenNameSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchGivenNames(query: GivenNameQuery = {}): Promise<GivenName[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/given-names${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchGivenNames(query: GivenNameSearchQuery): Promise<GivenName[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/given-names/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

// fetchGivenName — GET /api/given-names/{id}, открыто анонимному посетителю. Через
// authFetch — ради ApiError (нужен код 404 на странице View).
export async function fetchGivenName(id: string): Promise<GivenName> {
  return authFetch<GivenName>(`/api/given-names/${encodeURIComponent(id)}`);
}

// createGivenName/updateGivenName/deleteGivenName — запись, только для вошедшего
// владельца (requireFull на сервере, internal/httpapi/given_name_write.go).
export async function createGivenName(input: GivenNameInput): Promise<GivenName> {
  return authFetch<GivenName>("/api/given-names", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateGivenName(id: string, input: GivenNameInput): Promise<GivenName> {
  return authFetch<GivenName>(`/api/given-names/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteGivenName(id: string): Promise<void> {
  return authFetch<void>(`/api/given-names/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export interface SourceLink {
  citation_id: string;
  target_type?: string;
  target_id?: string;
  reliability?: string;
  role?: string;
  note?: string;
}

// FactDate — контракт структурированной даты (transport.FactDate): год/
// месяц/день, точность, формулировка, календарь, верхняя граница периода для
// modifier=between. Первое появление в проекте (Parish.Since/Until).
export type FactPrecision = "unknown" | "year" | "month" | "day";
export type FactModifier = "exact" | "approx" | "before" | "after" | "between";
export type FactCalendar = "" | "gregorian" | "julian" | "unknown";

export interface FactDate {
  year: number;
  month?: number;
  day?: number;
  precision: FactPrecision;
  modifier: FactModifier;
  calendar?: FactCalendar;
  year_to?: number;
  month_to?: number;
  day_to?: number;
}

export interface Repository {
  id: string;
  name: string;
  type: string;
  address?: string;
  urls: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

// RepositoryInput — тело POST/PUT /api/repositories (transport.RepositoryCreate
// и transport.RepositoryUpdate имеют одинаковую форму: полная замена всех
// полей, включая sources — редактируется с подпроекта 5).
export interface RepositoryInput {
  name: string;
  type: string;
  address?: string;
  urls: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

export interface RepositoryQuery {
  limit?: number;
  offset?: number;
}

export interface RepositorySearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchRepositories(query: RepositoryQuery = {}): Promise<Repository[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/repositories${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchRepositories(query: RepositorySearchQuery): Promise<Repository[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/repositories/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchRepository(id: string): Promise<Repository> {
  return authFetch<Repository>(`/api/repositories/${encodeURIComponent(id)}`);
}

export async function createRepository(input: RepositoryInput): Promise<Repository> {
  return authFetch<Repository>("/api/repositories", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateRepository(id: string, input: RepositoryInput): Promise<Repository> {
  return authFetch<Repository>(`/api/repositories/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteRepository(id: string): Promise<void> {
  return authFetch<void>(`/api/repositories/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export interface Family {
  id: string;
  name: string;
  members: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

// FamilyInput — тело POST/PUT /api/families (transport.FamilyCreate и
// transport.FamilyUpdate имеют одинаковую форму: полная замена всех полей,
// включая sources — редактируется с рождения контракта, docs/data-model/
// entity-write.md §3.3/§3.6).
export interface FamilyInput {
  name: string;
  members: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

export interface FamilyQuery {
  limit?: number;
  offset?: number;
}

export interface FamilySearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchFamilies(query: FamilyQuery = {}): Promise<Family[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/families${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchFamilies(query: FamilySearchQuery): Promise<Family[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/families/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchFamily(id: string): Promise<Family> {
  return authFetch<Family>(`/api/families/${encodeURIComponent(id)}`);
}

export async function createFamily(input: FamilyInput): Promise<Family> {
  return authFetch<Family>("/api/families", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateFamily(id: string, input: FamilyInput): Promise<Family> {
  return authFetch<Family>(`/api/families/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteFamily(id: string): Promise<void> {
  return authFetch<void>(`/api/families/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export interface Church {
  id: string;
  name: string;
  parish?: TextRef | null;
  settlements: TextRef[];
  variants: string[];
  notes: TextRef[];
  sources: SourceLink[];
}

export interface ChurchInput {
  name: string;
  parish?: TextRef | null;
  settlements: TextRef[];
  variants: string[];
  notes: TextRef[];
  sources: SourceLink[];
}

export interface ChurchQuery {
  limit?: number;
  offset?: number;
}

export interface ChurchSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchChurches(query: ChurchQuery = {}): Promise<Church[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/churches${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchChurches(query: ChurchSearchQuery): Promise<Church[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/churches/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchChurch(id: string): Promise<Church> {
  return authFetch<Church>(`/api/churches/${encodeURIComponent(id)}`);
}

export async function createChurch(input: ChurchInput): Promise<Church> {
  return authFetch<Church>("/api/churches", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateChurch(id: string, input: ChurchInput): Promise<Church> {
  return authFetch<Church>(`/api/churches/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteChurch(id: string): Promise<void> {
  return authFetch<void>(`/api/churches/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export interface Parish {
  id: string;
  name: string;
  church?: TextRef | null;
  settlements: TextRef[];
  since?: FactDate | null;
  until?: FactDate | null;
  notes: TextRef[];
  sources: SourceLink[];
}

export interface ParishInput {
  name: string;
  church?: TextRef | null;
  settlements: TextRef[];
  since?: FactDate | null;
  until?: FactDate | null;
  notes: TextRef[];
  sources: SourceLink[];
}

export interface ParishQuery {
  limit?: number;
  offset?: number;
}

export interface ParishSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchParishes(query: ParishQuery = {}): Promise<Parish[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/parishes${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchParishes(query: ParishSearchQuery): Promise<Parish[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/parishes/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchParish(id: string): Promise<Parish> {
  return authFetch<Parish>(`/api/parishes/${encodeURIComponent(id)}`);
}

export async function createParish(input: ParishInput): Promise<Parish> {
  return authFetch<Parish>("/api/parishes", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateParish(id: string, input: ParishInput): Promise<Parish> {
  return authFetch<Parish>(`/api/parishes/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteParish(id: string): Promise<void> {
  return authFetch<void>(`/api/parishes/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export interface Archive {
  id: string;
  name: string;
  system?: TextRef | null;
  repository_id?: string;
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

// ArchiveInput — repository_id — просто id (не TextRef, в отличие от
// system/parish/church у других сущностей): пустая строка — без хранилища.
export interface ArchiveInput {
  name: string;
  system?: TextRef | null;
  repository_id?: string;
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

export interface ArchiveQuery {
  limit?: number;
  offset?: number;
}

export interface ArchiveSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchArchives(query: ArchiveQuery = {}): Promise<Archive[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/archives${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchArchives(query: ArchiveSearchQuery): Promise<Archive[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/archives/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchArchive(id: string): Promise<Archive> {
  return authFetch<Archive>(`/api/archives/${encodeURIComponent(id)}`);
}

export async function createArchive(input: ArchiveInput): Promise<Archive> {
  return authFetch<Archive>("/api/archives", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateArchive(id: string, input: ArchiveInput): Promise<Archive> {
  return authFetch<Archive>(`/api/archives/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteArchive(id: string): Promise<void> {
  return authFetch<void>(`/api/archives/${encodeURIComponent(id)}`, { method: "DELETE" });
}

// Note — заметка (markdown-текст с иерархией «книга → главы»). parent_id —
// просто id родительской заметки (не TextRef — строгая self-ref ссылка, как
// у Archive.repository_id); пустая строка — без родителя. sources
// редактируется с подпроекта 5 (Citation теперь имеет CRUD).
export interface Note {
  id: string;
  kind: string;
  title?: string;
  text?: string;
  parent_id?: string;
  sources: SourceLink[];
  private: boolean;
}

export interface NoteInput {
  kind: string;
  title?: string;
  text?: string;
  parent_id?: string;
  sources: SourceLink[];
  private: boolean;
}

export interface NoteQuery {
  limit?: number;
  offset?: number;
}

export interface NoteSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchNotes(query: NoteQuery = {}): Promise<Note[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/notes${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchNotes(query: NoteSearchQuery): Promise<Note[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/notes/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchNote(id: string): Promise<Note> {
  return authFetch<Note>(`/api/notes/${encodeURIComponent(id)}`);
}

export async function createNote(input: NoteInput): Promise<Note> {
  return authFetch<Note>("/api/notes", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateNote(id: string, input: NoteInput): Promise<Note> {
  return authFetch<Note>(`/api/notes/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteNote(id: string): Promise<void> {
  return authFetch<void>(`/api/notes/${encodeURIComponent(id)}`, { method: "DELETE" });
}

// Attachment — файловое вложение. node_id — обязательная строгая ссылка на
// архивный узел (просто id — строка, не объект). document_id —
// необязательная мягкая ссылка (ON DELETE SET NULL). Оба поля в веб-форме
// заполняются через ArchiveNodePicker/ArchiveDocumentSelect (см.
// ArchiveNodePicker.tsx).
export interface Attachment {
  id: string;
  kind: string;
  uri?: string;
  filename?: string;
  mime?: string;
  page?: number;
  node_id: string;
  document_id?: string;
  note?: string;
  private: boolean;
}

export interface AttachmentInput {
  kind: string;
  uri?: string;
  filename?: string;
  mime?: string;
  page?: number;
  node_id: string;
  document_id?: string;
  note?: string;
  private: boolean;
}

export interface AttachmentQuery {
  limit?: number;
  offset?: number;
}

export interface AttachmentSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchAttachments(query: AttachmentQuery = {}): Promise<Attachment[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/attachments${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchAttachments(query: AttachmentSearchQuery): Promise<Attachment[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/attachments/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchAttachment(id: string): Promise<Attachment> {
  return authFetch<Attachment>(`/api/attachments/${encodeURIComponent(id)}`);
}

export async function createAttachment(input: AttachmentInput): Promise<Attachment> {
  return authFetch<Attachment>("/api/attachments", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateAttachment(id: string, input: AttachmentInput): Promise<Attachment> {
  return authFetch<Attachment>(`/api/attachments/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteAttachment(id: string): Promise<void> {
  return authFetch<void>(`/api/attachments/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export type Reliability = "primary" | "contemporary" | "memory" | "indirect" | "unknown";

// Anchor — контракт полиморфной привязки «где именно» (transport.Anchor):
// плоское представление с дискриминатором kind, по образцу FactDate. undefined
// — привязки нет (цитата может относиться к источнику целиком).
export type AnchorKind = "archive" | "file" | "url";

export interface Anchor {
  kind: AnchorKind;
  node_id?: string;
  document_id?: string;
  page?: number;
  rect?: string;
  attachment_id?: string;
  timecode?: string;
  url?: string;
}

// Source — источник доказательства. date — структурированная дата (см.
// FactDate). repository_id — просто id (мягкая ссылка, необязательна).
export interface Source {
  id: string;
  kind: string;
  title: string;
  author?: string;
  date?: FactDate | null;
  reliability: Reliability;
  repository_id?: string;
  notes: TextRef[];
  private: boolean;
}

export interface SourceInput {
  kind: string;
  title: string;
  author?: string;
  date?: FactDate | null;
  reliability: Reliability;
  repository_id?: string;
  notes: TextRef[];
  private: boolean;
}

export interface SourceQuery {
  limit?: number;
  offset?: number;
}

export interface SourceSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchSources(query: SourceQuery = {}): Promise<Source[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/sources${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchSources(query: SourceSearchQuery): Promise<Source[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/sources/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchSource(id: string): Promise<Source> {
  return authFetch<Source>(`/api/sources/${encodeURIComponent(id)}`);
}

export async function createSource(input: SourceInput): Promise<Source> {
  return authFetch<Source>("/api/sources", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateSource(id: string, input: SourceInput): Promise<Source> {
  return authFetch<Source>(`/api/sources/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteSource(id: string): Promise<void> {
  return authFetch<void>(`/api/sources/${encodeURIComponent(id)}`, { method: "DELETE" });
}

// Citation — цитата из источника. anchor — необязательная полиморфная
// привязка «где именно» (см. Anchor).
export interface Citation {
  id: string;
  source_id: string;
  anchor?: Anchor | null;
  text?: string;
  note?: string;
  private: boolean;
}

export interface CitationInput {
  source_id: string;
  anchor?: Anchor | null;
  text?: string;
  note?: string;
  private: boolean;
}

export interface CitationQuery {
  limit?: number;
  offset?: number;
}

export interface CitationSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchCitations(query: CitationQuery = {}): Promise<Citation[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/citations${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchCitations(query: CitationSearchQuery): Promise<Citation[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/citations/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchCitation(id: string): Promise<Citation> {
  return authFetch<Citation>(`/api/citations/${encodeURIComponent(id)}`);
}

export async function createCitation(input: CitationInput): Promise<Citation> {
  return authFetch<Citation>("/api/citations", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateCitation(id: string, input: CitationInput): Promise<Citation> {
  return authFetch<Citation>(`/api/citations/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteCitation(id: string): Promise<void> {
  return authFetch<void>(`/api/citations/${encodeURIComponent(id)}`, { method: "DELETE" });
}

// ArchiveNode — узел архивного дерева (transport.ArchiveNode). Дерево
// скопировано по обязательному ArchiveID — не одно глобальное дерево, как
// AdminDivision, а по одному на архив (docs/data-model/entity-write.md §3.4).
// ParentID, если задан, — родитель В ПРЕДЕЛАХ ТОГО ЖЕ архива (сервер это
// проверяет — 422 на parent_id, если родитель из другого архива).
export interface ArchiveNode {
  id: string;
  type: string;
  archive_id: string;
  parent_id?: string | null;
  label: string;
  name: string;
  since?: FactDate | null;
  until?: FactDate | null;
  parish?: TextRef | null;
  settlements: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

export interface ArchiveNodeInput {
  type: string;
  archive_id: string;
  parent_id?: string | null;
  label: string;
  name: string;
  since?: FactDate | null;
  until?: FactDate | null;
  parish?: TextRef | null;
  settlements: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

// ArchiveNodeQuery — archive_id ОБЯЗАТЕЛЕН (сервер отдаёт 400 без него, узел
// бессмысленен вне архива). parent_id — как у AdminDivisionQuery: не задан —
// корень (внутри архива), иначе — прямые дети.
export interface ArchiveNodeQuery {
  archive_id: string;
  parent_id?: string;
  limit?: number;
  offset?: number;
}

export interface ArchiveNodeSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchArchiveNodes(query: ArchiveNodeQuery): Promise<ArchiveNode[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/archive-nodes${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

// searchArchiveNodes — GET /api/archive-nodes/search?q=. Глобальный поиск, НЕ
// ограничен archive_id (в отличие от fetchArchiveNodes) — сужение по архиву,
// если нужно, делает вызывающая сторона на клиенте (см. ArchiveNodesList.tsx).
export async function searchArchiveNodes(query: ArchiveNodeSearchQuery): Promise<ArchiveNode[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/archive-nodes/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchArchiveNode(id: string): Promise<ArchiveNode> {
  return authFetch<ArchiveNode>(`/api/archive-nodes/${encodeURIComponent(id)}`);
}

export async function createArchiveNode(input: ArchiveNodeInput): Promise<ArchiveNode> {
  return authFetch<ArchiveNode>("/api/archive-nodes", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateArchiveNode(id: string, input: ArchiveNodeInput): Promise<ArchiveNode> {
  return authFetch<ArchiveNode>(`/api/archive-nodes/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteArchiveNode(id: string): Promise<void> {
  return authFetch<void>(`/api/archive-nodes/${encodeURIComponent(id)}`, { method: "DELETE" });
}

// ArchiveDocument — документ внутри единицы учёта (transport.ArchiveDocument).
// В отличие от ArchiveNode — плоская сущность, без собственной иерархии;
// unit_id — обязательная строгая ссылка на ArchiveNode.
export interface ArchiveDocument {
  id: string;
  unit_id: string;
  title: string;
  kind: string;
  since?: FactDate | null;
  until?: FactDate | null;
  parish?: TextRef | null;
  settlements: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

export interface ArchiveDocumentInput {
  unit_id: string;
  title: string;
  kind: string;
  since?: FactDate | null;
  until?: FactDate | null;
  parish?: TextRef | null;
  settlements: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

export interface ArchiveDocumentQuery {
  limit?: number;
  offset?: number;
}

export interface ArchiveDocumentSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchArchiveDocuments(
  query: ArchiveDocumentQuery = {},
): Promise<ArchiveDocument[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/archive-documents${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchArchiveDocuments(
  query: ArchiveDocumentSearchQuery,
): Promise<ArchiveDocument[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/archive-documents/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchArchiveDocument(id: string): Promise<ArchiveDocument> {
  return authFetch<ArchiveDocument>(`/api/archive-documents/${encodeURIComponent(id)}`);
}

export async function createArchiveDocument(input: ArchiveDocumentInput): Promise<ArchiveDocument> {
  return authFetch<ArchiveDocument>("/api/archive-documents", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateArchiveDocument(
  id: string,
  input: ArchiveDocumentInput,
): Promise<ArchiveDocument> {
  return authFetch<ArchiveDocument>(`/api/archive-documents/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteArchiveDocument(id: string): Promise<void> {
  return authFetch<void>(`/api/archive-documents/${encodeURIComponent(id)}`, { method: "DELETE" });
}
```
#### `web/src/App.tsx` (изменить — итоговое содержимое)
```tsx
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import type { ReactNode } from "react";
import { Layout } from "antd";
import { SessionProvider } from "./session";
import { AppHeader } from "./AppHeader";
import DocsPanel from "./docs-panel";
import EntityCatalog from "./pages/EntityCatalog";
import DivisionsList from "./pages/DivisionsList";
import DivisionView from "./pages/DivisionView";
import SurnamesList from "./pages/SurnamesList";
import SurnameView from "./pages/SurnameView";
import PatronymicsList from "./pages/PatronymicsList";
import PatronymicView from "./pages/PatronymicView";
import EstatesList from "./pages/EstatesList";
import EstateView from "./pages/EstateView";
import TitlesList from "./pages/TitlesList";
import TitleView from "./pages/TitleView";
import GivenNamesList from "./pages/GivenNamesList";
import GivenNameView from "./pages/GivenNameView";
import RepositoriesList from "./pages/RepositoriesList";
import RepositoryView from "./pages/RepositoryView";
import ChurchesList from "./pages/ChurchesList";
import ChurchView from "./pages/ChurchView";
import ParishesList from "./pages/ParishesList";
import ParishView from "./pages/ParishView";
import ArchivesList from "./pages/ArchivesList";
import ArchiveView from "./pages/ArchiveView";
import ArchiveNodesList from "./pages/ArchiveNodesList";
import ArchiveNodeView from "./pages/ArchiveNodeView";
import ArchiveDocumentsList from "./pages/ArchiveDocumentsList";
import ArchiveDocumentView from "./pages/ArchiveDocumentView";
import NotesList from "./pages/NotesList";
import NoteView from "./pages/NoteView";
import AttachmentsList from "./pages/AttachmentsList";
import AttachmentView from "./pages/AttachmentView";
import SourcesList from "./pages/SourcesList";
import SourceView from "./pages/SourceView";
import CitationsList from "./pages/CitationsList";
import CitationView from "./pages/CitationView";
import FamiliesList from "./pages/FamiliesList";
import FamilyView from "./pages/FamilyView";
import LoginPage from "./pages/Login";
import RegisterPage from "./pages/Register";
import SettingsPage from "./pages/Settings";

const { Content } = Layout;

// PageLayout — общая рамка (шапка + отступы) для каждой страницы каталога.
// Раньше все страницы делили один AppContent с Tabs; теперь у каждой —
// собственный роут, а переключение между ними — через каталог сущностей
// (/) + хлебные крошки на каждой странице, не вкладки.
function PageLayout({ children }: { children: ReactNode }) {
  return (
    <Layout style={{ minHeight: "100vh" }}>
      <AppHeader />
      <Content style={{ padding: 24 }}>{children}</Content>
    </Layout>
  );
}

export default function App() {
  return (
    <BrowserRouter>
      <SessionProvider>
        <Routes>
          <Route path="/" element={<PageLayout><EntityCatalog /></PageLayout>} />
          <Route path="/docs" element={<PageLayout><DocsPanel /></PageLayout>} />
          <Route path="/docs/:docPath*" element={<PageLayout><DocsPanel /></PageLayout>} />
          <Route path="/divisions" element={<PageLayout><DivisionsList /></PageLayout>} />
          <Route path="/divisions/:id" element={<PageLayout><DivisionView /></PageLayout>} />
          <Route path="/surnames" element={<PageLayout><SurnamesList /></PageLayout>} />
          <Route path="/surnames/:id" element={<PageLayout><SurnameView /></PageLayout>} />
          <Route path="/patronymics" element={<PageLayout><PatronymicsList /></PageLayout>} />
          <Route path="/patronymics/:id" element={<PageLayout><PatronymicView /></PageLayout>} />
          <Route path="/estates" element={<PageLayout><EstatesList /></PageLayout>} />
          <Route path="/estates/:id" element={<PageLayout><EstateView /></PageLayout>} />
          <Route path="/titles" element={<PageLayout><TitlesList /></PageLayout>} />
          <Route path="/titles/:id" element={<PageLayout><TitleView /></PageLayout>} />
          <Route path="/given-names" element={<PageLayout><GivenNamesList /></PageLayout>} />
          <Route path="/given-names/:id" element={<PageLayout><GivenNameView /></PageLayout>} />
          <Route path="/repositories" element={<PageLayout><RepositoriesList /></PageLayout>} />
          <Route path="/repositories/:id" element={<PageLayout><RepositoryView /></PageLayout>} />
          <Route path="/churches" element={<PageLayout><ChurchesList /></PageLayout>} />
          <Route path="/churches/:id" element={<PageLayout><ChurchView /></PageLayout>} />
          <Route path="/parishes" element={<PageLayout><ParishesList /></PageLayout>} />
          <Route path="/parishes/:id" element={<PageLayout><ParishView /></PageLayout>} />
          <Route path="/archives" element={<PageLayout><ArchivesList /></PageLayout>} />
          <Route path="/archives/:id" element={<PageLayout><ArchiveView /></PageLayout>} />
          <Route path="/archive-nodes" element={<PageLayout><ArchiveNodesList /></PageLayout>} />
          <Route path="/archive-nodes/:id" element={<PageLayout><ArchiveNodeView /></PageLayout>} />
          <Route path="/archive-documents" element={<PageLayout><ArchiveDocumentsList /></PageLayout>} />
          <Route path="/archive-documents/:id" element={<PageLayout><ArchiveDocumentView /></PageLayout>} />
          <Route path="/notes" element={<PageLayout><NotesList /></PageLayout>} />
          <Route path="/notes/:id" element={<PageLayout><NoteView /></PageLayout>} />
          <Route path="/attachments" element={<PageLayout><AttachmentsList /></PageLayout>} />
          <Route path="/attachments/:id" element={<PageLayout><AttachmentView /></PageLayout>} />
          <Route path="/sources" element={<PageLayout><SourcesList /></PageLayout>} />
          <Route path="/sources/:id" element={<PageLayout><SourceView /></PageLayout>} />
          <Route path="/citations" element={<PageLayout><CitationsList /></PageLayout>} />
          <Route path="/citations/:id" element={<PageLayout><CitationView /></PageLayout>} />
          <Route path="/families" element={<PageLayout><FamiliesList /></PageLayout>} />
          <Route path="/families/:id" element={<PageLayout><FamilyView /></PageLayout>} />
          <Route path="/login" element={<LoginPage />} />
          <Route path="/register" element={<RegisterPage />} />
          <Route path="/settings" element={<SettingsPage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </SessionProvider>
    </BrowserRouter>
  );
}
```
#### `web/src/pages/EntityCatalog.tsx` (изменить — итоговое содержимое)
```tsx
import { Card, List, Typography } from "antd";
import { Link } from "react-router-dom";

// CATALOG_ENTRIES — единая точка входа приложения: по алфавиту названия.
// Каждая сущность получает свой список при подключении (docs/data-model/
// entity-write.md §4) — здесь просто добавляется новая строка, без вкладок.
const CATALOG_ENTRIES: { label: string; path: string }[] = [
  { label: "Административное деление", path: "/divisions" },
  { label: "Архивные документы", path: "/archive-documents" },
  { label: "Архивные единицы", path: "/archive-nodes" },
  { label: "Архивы", path: "/archives" },
  { label: "Вложения", path: "/attachments" },
  { label: "Документация", path: "/docs" },
  { label: "Заметки", path: "/notes" },
  { label: "Имена", path: "/given-names" },
  { label: "Отчества", path: "/patronymics" },
  { label: "Источники", path: "/sources" },
  { label: "Приходы", path: "/parishes" },
  { label: "Роды", path: "/families" },
  { label: "Сословия", path: "/estates" },
  { label: "Титулы", path: "/titles" },
  { label: "Фамилии", path: "/surnames" },
  { label: "Хранилища", path: "/repositories" },
  { label: "Церкви", path: "/churches" },
  { label: "Цитаты", path: "/citations" },
];

const SORTED_ENTRIES = [...CATALOG_ENTRIES].sort((a, b) => a.label.localeCompare(b.label, "ru"));

export default function EntityCatalog() {
  return (
    <Card title="Сущности">
      <List
        dataSource={SORTED_ENTRIES}
        renderItem={(entry) => (
          <List.Item>
            <Link to={entry.path}>
              <Typography.Text>{entry.label}</Typography.Text>
            </Link>
          </List.Item>
        )}
      />
    </Card>
  );
}
```

### 2.3. Рубеж

`cd web && npm run typecheck` (чисто), `npm run build` (чисто). Живая проверка (см. «Предпосылка») — не обязательна повторно: `/families` список рендерится, «+ добавить» открывает `CreateFamilyModal`; заполнение `Название`, строки `Members`/`Notes` через `TextRefListEditor`, строки `Sources` через `SourceLinkListEditor` (живой выбор цитаты из выпадающего списка), чекбокс «Приватная запись», отправка → переход на `/families/{id}`, `FamilyView` корректно показывает все четыре поля; «Редактировать» → форма предзаполнена всеми текущими значениями (включая отмеченный чекбокс приватности) → снятие чекбокса → «Сохранить» → `PUT` прошёл, view обновился на «Приватная: нет».

### 2.4. Коммит

```bash
git add web/src/pages/FamiliesList.tsx web/src/pages/FamilyForm.tsx web/src/pages/FamilyView.tsx web/src/api.ts web/src/App.tsx web/src/pages/EntityCatalog.tsx
git commit -m "feat(web): страницы Family — список/просмотр/редактирование/создание, каталог"
```

## Рубеж прохода

После Задачи 2: `gofmt -l .` пусто, `go build/vet/test ./...` зелёные (1252 теста, 118 пакетов), `npm run typecheck`/`build` чисты. Каталог `/` — на одну строку больше, «Роды» в алфавитном порядке. `Family` имеет полный CRUD через HTTP, MCP и веб, по конвенциям `docs/data-model/entity-write.md` §3-4, без единого нового переиспользуемого паттерна программы — подтверждено и в `REPORT.md`, и в новой секции §3.6. После обеих задач — обзор всей ветки целиком (`git diff main...HEAD` по объёму подпроекта), при необходимости волна точечных фиксов по результатам ревью, затем слияние в `main` — как и в предыдущих шести подпроектах программы `entity-write`. Следующий подпроект — 8 (`Person`).

## Коммиты

Два коммита в `main`, по одному на задачу — см. Шаги 1.9, 2.4.
