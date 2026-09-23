# Веб-CRUD/MCP для всех сущностей — подпроект 2 (словари: GivenName, Patronymic, Estate, Title): план

> **Для агента-исполнителя:** ОБЯЗАТЕЛЬНЫЙ ПОДНАВЫК: superpowers:subagent-driven-development.

Второй проход по декомпозиции `docs/data-model/entity-write.md` §2 —
оставшиеся 4 «плоских» словаря без FK: `Patronymic`, `Estate`, `Title`
(байт-в-байт повторяют форму `Surname` — `{ID, Canonical, Variants, Items,
Notes}`) и `GivenName` (та же форма + обязательное поле `Gender
NameGender`). Паттерн (usecase-конвенции, транспорт, `httpapi`/MCP-роуты,
веб-страницы, `Deps`-реестр) уже задан и провалидирован подпроектом 1
(`docs/plans/2026-09-23-entity-write-surname.md`, коммит `da884ac`) — этот
план его не пересогласовывает, только применяет к оставшимся 4 сущностям
одним проходом, как решено при планировании программы (`docs/data-model/
entity-write.md` §2: «один план на группу однотипных сущностей»).

## Goal

1. Полный CRUD (HTTP + MCP + веб) для `Patronymic`, `Estate`, `Title`,
   `GivenName` — по тем же конвенциям, что `Surname` (`docs/data-model/
   entity-write.md` §3-4), без отклонений в форме DTO/маршрутов/тулов.
2. `GivenName` — единственное отличие от остальных трёх: обязательное поле
   `Gender` (`male`/`female`/`neutral`), провалидированное
   `internal/models/dictionary_validate.go` — отражено в DTO, HTTP JSON,
   MCP-схеме (`mcp.Enum`) и веб-форме (`Select`).
3. `httpapi.Deps`/`mcp.Deps` получают 4 новых поля (по образцу `Surnames`),
   `internal/app/app.go` — 4 новых facade-сервиса; каталог `/` и `App.tsx`
   получают 4 новые строки/роута.

## Предпосылка: живая проверка

Весь код ниже уже применён к рабочему дереву и проверен: `gofmt -l .`
пусто, `go build/vet/test ./...` — 763 теста, 52 пакета (было 643/28 после
подпроекта 1), `npm run typecheck`/`build` чисты. Живой смок-тест бэкенда
через `curl` (реальный `genodex serve`, чистая БД, зарегистрированный
владелец): `POST /api/patronymics`/`estates`/`titles`/`given-names` → 201 с
созданной записью; `GivenName` — `gender` корректно уходит в ответ и
обратно читается; `GET`-список находит созданное; `PUT
/api/given-names/{id}` с `gender: "neutral"` → 200, поле обновилось;
`DELETE /api/titles/{id}` → 204, повторный `GET` списка — пусто. Живая
проверка веб-UI в браузере (реальный сервер, собранный `web/dist`):
регистрация владельца → каталог на `/` (семь строк по алфавиту, включая
«Имена», «Отчества», «Сословия», «Титулы») → «Отчества»/«Сословия»/
«Титулы» — список и хлебные крошки корректны → «Имена» → «+ добавить» —
форма с полем «Пол» (обязательный `Select`: Мужской/Женский/Нейтральный) →
создание записи → View показывает «Пол: Женский» → «Редактировать» — форма
верно предзаполнена (включая пол) → смена пола на «Нейтральный» →
сохранение → View обновился.

При живой проверке найден и исправлен реальный баг генерации: DTO
`transport.GivenNameCreate`/`GivenNameUpdate`/`GivenName` изначально не
содержали поля `Gender` (потеряно при механическом копировании из
`Surname`) — HTTP-контракт молча терял пол при любом create/update через
`/api/given-names`, и MCP `given_name_update` не проставлял `Gender` в
обновляемую запись. Код ниже уже содержит исправленную версию;
регресс-тесты на оба случая добавлены (`TestGivenNameCreateContract`/
`TestGivenNameUpdateMergesFields` в `internal/httpapi/given_name_write_test.go`
проверяют `gender` в теле запроса/ответа; `TestGivenNameUpdateToolSetsGender`
в `internal/mcp/given_name_test.go` — то же через MCP-тул).

---

## Задача 1. Бэкенд: Patronymic, Estate, Title, GivenName + `Deps`-реестр

**Интерфейсы, потребляемые из подпроекта 1**: `transport.TextRef`/`TextRefsFromModel`/`TextRefsToModel` (`internal/transport/text_ref.go`), `models.SearchQuery` (`internal/models/query.go`), общие хелперы `internal/httpapi/surname.go`:`parsePage` и `internal/mcp/surname.go`:`textRefsFromStrings` — **используются как есть, НЕ переопределяются** в новых файлах (иначе `redeclared in this block`, `package httpapi`/`package mcp` — общие для всех сущностей пакеты).

**Производит**: `httpapi.{Patronymic,Estate,Title,GivenName}Service`, `mcp.{Patronymic,Estate,Title,GivenName}Service`, маршруты `/api/{patronymics,estates,titles,given-names}`, MCP-тулы `{patronymic,estate,title,given_name}_{list,search,get,create,update,delete}` — потребляются Задачами 2 и 3 (веб) через HTTP/MCP, не напрямую.

**Файлы:**
- Создать: `internal/transport/{patronymic,patronymic_write,estate,estate_write,title,title_write,given_name,given_name_write}.go`, `internal/usecases/{list_patronymics,search_patronymics,get_patronymic,create_patronymic,update_patronymic,delete_patronymic}/{deps.go,scenario.go,scenario_test.go}` (и та же шестёрка директорий для `estates`/`estate`, `titles`/`title`, `given_names`/`given_name`), `internal/httpapi/{patronymic,patronymic_write,patronymic_test,patronymic_write_test}.go` (и то же для `estate`, `title`, `given_name`), `internal/mcp/{patronymic,patronymic_test}.go` (и то же для `estate`, `title`, `given_name`)
- Изменить: `internal/httpapi/deps.go`, `internal/httpapi/api.go`, `internal/httpapi/httpapi.go`, `internal/mcp/deps.go`, `internal/mcp/server.go`, `internal/app/app.go`

**Важно для исполнителя**: `Patronymic`/`Estate`/`Title` — байт-в-байт одинаковая форма записи (`{ID, Canonical, Variants, Items, Notes}`, ID-префиксы `PN-`/`ES-`/`TT-`, маршруты `/api/patronymics`/`/api/estates`/`/api/titles`, MCP-префиксы тулов `patronymic_`/`estate_`/`title_`). `GivenName` — та же форма плюс обязательное поле `Gender models.NameGender` (`male`/`female`/`neutral`, `internal/models/given_name.go`, валидация — `internal/models/dictionary_validate.go`), ID-префикс `GN-`, маршрут `/api/given-names` (через дефис — составное английское название, в отличие от простых множественных чисел остальных трёх), MCP-префикс `given_name_`. Ниже — полное содержимое каждого файла; переносить как есть, без сокращений.

### Шаг 1.1. Patronymic — транспорт, usecases, httpapi, MCP

#### `internal/transport/patronymic.go` (создать)
`internal/transport/patronymic.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// Patronymic — контракт словарной записи фамилии (GET /api/patronymics, MCP-тул patronymic_list).
type Patronymic struct {
	ID        models.ID `json:"id"`
	Canonical string    `json:"canonical"`
	Variants  []TextRef `json:"variants"`
	Items     []TextRef `json:"items"`
	Notes     []TextRef `json:"notes"`
}

// PatronymicFromModel конвертирует запись в контракт.
func PatronymicFromModel(s models.Patronymic) Patronymic {
	return Patronymic{
		ID:        s.ID,
		Canonical: s.Canonical,
		Variants:  TextRefsFromModel(s.Variants),
		Items:     TextRefsFromModel(s.Items),
		Notes:     TextRefsFromModel(s.Notes),
	}
}

// PatronymicsFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func PatronymicsFromModels(ss []models.Patronymic) []Patronymic {
	out := make([]Patronymic, 0, len(ss))
	for _, s := range ss {
		out = append(out, PatronymicFromModel(s))
	}

	return out
}
```

#### `internal/transport/patronymic_write.go` (создать)
`internal/transport/patronymic_write.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// PatronymicCreate — тело POST /api/patronymics и аргументы тула patronymic_create.
// Идентификатор генерирует сценарий.
type PatronymicCreate struct {
	Canonical string    `json:"canonical"`
	Variants  []TextRef `json:"variants"`
	Items     []TextRef `json:"items"`
	Notes     []TextRef `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (s PatronymicCreate) Model() models.Patronymic {
	return models.Patronymic{
		Canonical: s.Canonical,
		Variants:  TextRefsToModel(s.Variants),
		Items:     TextRefsToModel(s.Items),
		Notes:     TextRefsToModel(s.Notes),
	}
}

// PatronymicUpdate — тело PUT /api/patronymics/{id} и аргументы тула patronymic_update:
// полная замена canonical/variants/items/notes.
type PatronymicUpdate struct {
	Canonical string    `json:"canonical"`
	Variants  []TextRef `json:"variants"`
	Items     []TextRef `json:"items"`
	Notes     []TextRef `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (s PatronymicUpdate) Model() models.Patronymic {
	return models.Patronymic{
		Canonical: s.Canonical,
		Variants:  TextRefsToModel(s.Variants),
		Items:     TextRefsToModel(s.Items),
		Notes:     TextRefsToModel(s.Notes),
	}
}
```

#### `internal/usecases/list_patronymics/` (создать)
`internal/usecases/list_patronymics/deps.go`:
```go
package list_patronymics

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// PatronymicRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type PatronymicRepo interface {
	ListPatronymics(ctx context.Context, access models.Access, page models.Page) ([]*models.Patronymic, error)
}
```

`internal/usecases/list_patronymics/scenario.go`:
```go
package list_patronymics

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список словарных записей фамилий».
type Scenario struct {
	patronymics PatronymicRepo
}

// New создаёт сценарий.
func New(patronymics PatronymicRepo) *Scenario {
	return &Scenario{patronymics: patronymics}
}

// ListPatronymics возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListPatronymics(ctx context.Context, access models.Access, page models.Page) ([]models.Patronymic, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypePatronymic, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypePatronymic, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.patronymics.ListPatronymics(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Patronymic, 0, len(list))
	for _, sn := range list {
		out = append(out, *sn)
	}

	return out, nil
}
```

`internal/usecases/list_patronymics/scenario_test.go`:
```go
package list_patronymics

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.Patronymic
}

func (f *fakeRepo) ListPatronymics(_ context.Context, _ models.Access, page models.Page) ([]*models.Patronymic, error) {
	f.page = page

	return f.out, nil
}

func TestListPatronymicsReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.Patronymic{{ID: "PN-01ARZ3NDEKTSV4RRFFQ69G5FA1", Canonical: "Иванов"}}}

	got, err := New(repo).ListPatronymics(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListPatronymics: %v", err)
	}

	if len(got) != 1 || got[0].Canonical != "Иванов" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListPatronymicsRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListPatronymics(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
```

#### `internal/usecases/search_patronymics/` (создать)
`internal/usecases/search_patronymics/deps.go`:
```go
package search_patronymics

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// PatronymicRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type PatronymicRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetPatronymic(ctx context.Context, id models.ID) (*models.Patronymic, error)
}
```

`internal/usecases/search_patronymics/scenario.go`:
```go
package search_patronymics

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск словарных записей фамилий».
type Scenario struct {
	patronymics PatronymicRepo
}

// New создаёт сценарий.
func New(patronymics PatronymicRepo) *Scenario {
	return &Scenario{patronymics: patronymics}
}

// SearchPatronymics находит записи, чья каноническая форма (или вариант)
// начинается с текста запроса — та же механика, что и
// search_divisions.SearchDivisions (см. её комментарий).
func (s *Scenario) SearchPatronymics(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Patronymic, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Patronymic{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Patronymic{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.patronymics.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypePatronymic {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.patronymics.GetPatronymic(ctx, h.ID)
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

`internal/usecases/search_patronymics/scenario_test.go`:
```go
package search_patronymics

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits        []models.Hit
	patronymics map[models.ID]*models.Patronymic
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetPatronymic(_ context.Context, id models.ID) (*models.Patronymic, error) {
	s, ok := f.patronymics[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestSearchPatronymicsFiltersByType(t *testing.T) {
	id := models.ID("PN-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypePatronymic, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не patronymic — должен быть пропущен
		},
		patronymics: map[models.ID]*models.Patronymic{id: {ID: id, Canonical: "Иванов"}},
	}

	got, err := New(repo).SearchPatronymics(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchPatronymics: %v", err)
	}

	if len(got) != 1 || got[0].Canonical != "Иванов" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchPatronymicsEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchPatronymics(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchPatronymics: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
```

#### `internal/usecases/get_patronymic/` (создать)
`internal/usecases/get_patronymic/deps.go`:
```go
package get_patronymic

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// PatronymicRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type PatronymicRepo interface {
	GetPatronymic(ctx context.Context, id models.ID) (*models.Patronymic, error)
}
```

`internal/usecases/get_patronymic/scenario.go`:
```go
package get_patronymic

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «словарная запись фамилии по идентификатору».
type Scenario struct {
	patronymics PatronymicRepo
}

// New создаёт сценарий.
func New(patronymics PatronymicRepo) *Scenario {
	return &Scenario{patronymics: patronymics}
}

// GetPatronymic возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound.
func (s *Scenario) GetPatronymic(ctx context.Context, id models.ID) (models.Patronymic, error) {
	if err := validateID(id); err != nil {
		return models.Patronymic{}, err
	}

	sn, err := s.patronymics.GetPatronymic(ctx, id)
	if err != nil {
		return models.Patronymic{}, err
	}

	return *sn, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypePatronymic)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypePatronymic,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

`internal/usecases/get_patronymic/scenario_test.go`:
```go
package get_patronymic

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	patronymics map[models.ID]*models.Patronymic
}

func (f *fakeRepo) GetPatronymic(_ context.Context, id models.ID) (*models.Patronymic, error) {
	s, ok := f.patronymics[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestGetPatronymicReturnsRecord(t *testing.T) {
	id := models.ID("PN-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{patronymics: map[models.ID]*models.Patronymic{id: {ID: id, Canonical: "Иванов"}}}

	got, err := New(repo).GetPatronymic(context.Background(), id)
	if err != nil {
		t.Fatalf("GetPatronymic: %v", err)
	}

	if got.Canonical != "Иванов" {
		t.Fatalf("Canonical = %q", got.Canonical)
	}
}

func TestGetPatronymicNotFound(t *testing.T) {
	repo := &fakeRepo{patronymics: map[models.ID]*models.Patronymic{}}

	_, err := New(repo).GetPatronymic(context.Background(), "PN-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetPatronymicInvalidID(t *testing.T) {
	repo := &fakeRepo{patronymics: map[models.ID]*models.Patronymic{}}

	_, err := New(repo).GetPatronymic(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}
```

#### `internal/usecases/create_patronymic/` (создать)
`internal/usecases/create_patronymic/deps.go`:
```go
package create_patronymic

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// PatronymicStore — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type PatronymicStore interface {
	SavePatronymic(ctx context.Context, s *models.Patronymic) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
```

`internal/usecases/create_patronymic/scenario.go`:
```go
package create_patronymic

import (
	"context"
	"fmt"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «создание словарной записи фамилии».
type Scenario struct {
	store PatronymicStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st PatronymicStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreatePatronymic создаёт запись: генерирует идентификатор, проверяет
// инварианты, сохраняет. Возвращает созданную запись с заполненным ID.
// Флат-сущность без FK — в отличие от create_division, без транзакционной
// проверки родителя.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта) и невалидная сущность — *models.ValidationError; прочее —
// ошибки хранилища как есть.
func (s *Scenario) CreatePatronymic(ctx context.Context, sn models.Patronymic) (models.Patronymic, error) {
	if sn.ID != "" {
		return models.Patronymic{}, &models.ValidationError{
			Entity: models.TypePatronymic,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", sn.ID),
		}
	}

	sn.ID = s.ids.New(models.TypePatronymic)

	if err := sn.Validate(); err != nil {
		return models.Patronymic{}, err
	}

	if err := s.store.SavePatronymic(ctx, &sn); err != nil {
		return models.Patronymic{}, err
	}

	return sn, nil
}
```

`internal/usecases/create_patronymic/scenario_test.go`:
```go
package create_patronymic

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type stubIDs struct{ id models.ID }

func (s *stubIDs) New(models.Type) models.ID { return s.id }

type fakeStore struct {
	saved   *models.Patronymic
	saveErr error
}

func (f *fakeStore) SavePatronymic(_ context.Context, s *models.Patronymic) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *s
	f.saved = &cp

	return nil
}

func TestCreatePatronymicGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "PN-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{}
	sc := New(st, ids)

	got, err := sc.CreatePatronymic(context.Background(), models.Patronymic{Canonical: "Иванов"})
	if err != nil {
		t.Fatalf("CreatePatronymic: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.saved == nil || st.saved.Canonical != "Иванов" {
		t.Fatalf("saved = %+v", st.saved)
	}
}

func TestCreatePatronymicRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{})

	_, err := sc.CreatePatronymic(context.Background(), models.Patronymic{ID: "PN-01ARZ3NDEKTSV4RRFFQ69G5FA1", Canonical: "x"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestCreatePatronymicRejectsEmptyCanonical(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{id: "PN-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreatePatronymic(context.Background(), models.Patronymic{})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "canonical" {
		t.Fatalf("err = %v, want ValidationError on canonical", err)
	}
}

func TestCreatePatronymicPropagatesStoreError(t *testing.T) {
	wantErr := errors.New("db down")
	st := &fakeStore{saveErr: wantErr}
	sc := New(st, &stubIDs{id: "PN-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreatePatronymic(context.Background(), models.Patronymic{Canonical: "Иванов"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}
```

#### `internal/usecases/update_patronymic/` (создать)
`internal/usecases/update_patronymic/deps.go`:
```go
package update_patronymic

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// PatronymicStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования и сохранение идут в одной транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type PatronymicStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
```

`internal/usecases/update_patronymic/scenario.go`:
```go
package update_patronymic

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение словарной записи фамилии».
type Scenario struct {
	store PatronymicStore
}

// New создаёт сценарий.
func New(st PatronymicStore) *Scenario {
	return &Scenario{store: st}
}

// UpdatePatronymic полностью заменяет запись по sn.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, и сохраняет.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdatePatronymic(ctx context.Context, sn models.Patronymic) error {
	if err := sn.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetPatronymic(ctx, sn.ID); err != nil {
			return err
		}

		return tx.SavePatronymic(ctx, &sn)
	})
}
```

`internal/usecases/update_patronymic/scenario_test.go`:
```go
package update_patronymic

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
	patronymics map[models.ID]*models.Patronymic
	saved       []*models.Patronymic
}

func newFakeTx(existing ...*models.Patronymic) *fakeTx {
	tx := &fakeTx{patronymics: map[models.ID]*models.Patronymic{}}
	for _, s := range existing {
		tx.patronymics[s.ID] = s
	}

	return tx
}

func (f *fakeTx) GetPatronymic(_ context.Context, id models.ID) (*models.Patronymic, error) {
	s, ok := f.patronymics[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *s

	return &cp, nil
}

func (f *fakeTx) SavePatronymic(_ context.Context, s *models.Patronymic) error {
	cp := *s
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует PatronymicStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func patronymic(id models.ID, canonical string) *models.Patronymic {
	return &models.Patronymic{ID: id, Canonical: canonical}
}

func TestUpdatePatronymicSaves(t *testing.T) {
	existing := patronymic("PN-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Иванов")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Canonical = "Иванов (испр.)"

	if err := New(st).UpdatePatronymic(context.Background(), updated); err != nil {
		t.Fatalf("UpdatePatronymic: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Canonical != "Иванов (испр.)" {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdatePatronymicNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdatePatronymic(context.Background(), *patronymic("PN-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Иванов"))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdatePatronymicRejectsEmptyCanonical(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdatePatronymic(context.Background(), models.Patronymic{ID: "PN-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "canonical" {
		t.Fatalf("err = %v, want ValidationError on canonical", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}
```

#### `internal/usecases/delete_patronymic/` (создать)
`internal/usecases/delete_patronymic/deps.go`:
```go
package delete_patronymic

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// PatronymicRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type PatronymicRepo interface {
	DeletePatronymic(ctx context.Context, id models.ID) error
}
```

`internal/usecases/delete_patronymic/scenario.go`:
```go
package delete_patronymic

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление словарной записи фамилии».
type Scenario struct {
	patronymics PatronymicRepo
}

// New создаёт сценарий.
func New(patronymics PatronymicRepo) *Scenario {
	return &Scenario{patronymics: patronymics}
}

// DeletePatronymic удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeletePatronymic(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.patronymics.DeletePatronymic(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypePatronymic)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypePatronymic,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

`internal/usecases/delete_patronymic/scenario_test.go`:
```go
package delete_patronymic

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

func (f *fakeRepo) DeletePatronymic(_ context.Context, id models.ID) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)

	return nil
}

func TestDeletePatronymicCallsRepo(t *testing.T) {
	repo := &fakeRepo{}
	id := models.ID("PN-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	if err := New(repo).DeletePatronymic(context.Background(), id); err != nil {
		t.Fatalf("DeletePatronymic: %v", err)
	}

	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("deleted = %v", repo.deleted)
	}
}

func TestDeletePatronymicInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	err := New(repo).DeletePatronymic(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}

	if len(repo.deleted) != 0 {
		t.Fatalf("repo вызван при невалидном id")
	}
}

func TestDeletePatronymicPropagatesInUseError(t *testing.T) {
	wantErr := &models.InUseError{Type: models.TypePatronymic, ID: "PN-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	repo := &fakeRepo{err: wantErr}

	err := New(repo).DeletePatronymic(context.Background(), "PN-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	var iu *models.InUseError
	if !errors.As(err, &iu) {
		t.Fatalf("err = %v, want InUseError", err)
	}
}
```

#### `internal/httpapi/patronymic.go` (создать)
`internal/httpapi/patronymic.go`:
```go
package httpapi

import (
	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
	"net/http"
)

// handlePatronymicList — GET /api/patronymics?limit=&offset=. Чтение открыто
// анонимному посетителю (как и деления — auth.md §6, Access здесь не
// проверяется).
func handlePatronymicList(patronymics PatronymicService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := patronymics.ListPatronymics(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.PatronymicsFromModels(list))
	}
}

// handlePatronymicSearch — GET /api/patronymics/search?q=&limit=&offset=.
func handlePatronymicSearch(patronymics PatronymicService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := patronymics.SearchPatronymics(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.PatronymicsFromModels(list))
	}
}

// handlePatronymicGet — GET /api/patronymics/{id}.
func handlePatronymicGet(patronymics PatronymicService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sn, err := patronymics.GetPatronymic(r.Context(), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.PatronymicFromModel(sn))
	}
}
```

#### `internal/httpapi/patronymic_write.go` (создать)
`internal/httpapi/patronymic_write.go`:
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handlePatronymicCreate — POST /api/patronymics: создаёт запись, отвечает 201 с
// созданной записью (id генерирует сценарий). Запись — только для вошедшего
// владельца (auth.md §6, решение 9), см. handleDivisionCreate.
func handlePatronymicCreate(patronymics PatronymicService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.PatronymicCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := patronymics.CreatePatronymic(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.PatronymicFromModel(created))
	}
}

// handlePatronymicUpdate — PUT /api/patronymics/{id}: полная замена
// canonical/variants/items/notes. Читает текущую версию, накладывает поля
// запроса (fetch-then-merge — как handleDivisionUpdate; у Patronymic сейчас DTO
// покрывает всю модель, но конвенция общая для будущих сущностей, чей DTO
// v1 не покрывает всех полей модели, docs/data-model/entity-write.md §3).
func handlePatronymicUpdate(patronymics PatronymicService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.PatronymicUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := patronymics.GetPatronymic(r.Context(), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Canonical = m.Canonical
		cur.Variants = m.Variants
		cur.Items = m.Items
		cur.Notes = m.Notes

		if err := patronymics.UpdatePatronymic(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.PatronymicFromModel(cur))
	}
}

// handlePatronymicDelete — DELETE /api/patronymics/{id}: 204 без тела; занятая
// запись — 409 со списком ссылающихся. Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handlePatronymicDelete(patronymics PatronymicService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := patronymics.DeletePatronymic(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
```

#### `internal/httpapi/patronymic_test.go` (создать)
`internal/httpapi/patronymic_test.go`:
```go
package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakePatronymics struct {
	list []models.Patronymic
	err  error
	page models.Page

	getSN     models.Patronymic
	gotIDs    []models.ID
	created   models.Patronymic
	gotCreate models.Patronymic
	updated   models.Patronymic
	deleteErr error

	search    []models.Patronymic
	gotSearch models.SearchQuery
}

func (f *fakePatronymics) ListPatronymics(_ context.Context, _ models.Access, page models.Page) ([]models.Patronymic, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakePatronymics) SearchPatronymics(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Patronymic, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakePatronymics) GetPatronymic(_ context.Context, id models.ID) (models.Patronymic, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Patronymic{}, f.err
	}

	return f.getSN, nil
}

func (f *fakePatronymics) CreatePatronymic(_ context.Context, sn models.Patronymic) (models.Patronymic, error) {
	f.gotCreate = sn
	if f.err != nil {
		return models.Patronymic{}, f.err
	}

	return f.created, nil
}

func (f *fakePatronymics) UpdatePatronymic(_ context.Context, sn models.Patronymic) error {
	f.updated = sn

	return f.err
}

func (f *fakePatronymics) DeletePatronymic(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestPatronymicListReturnsRecords(t *testing.T) {
	svc := &fakePatronymics{list: []models.Patronymic{{ID: "PN-1", Canonical: "Иванов", Variants: []models.TextRef{{Text: "Иванова"}}}}}

	rec := get(t, NewHandler(Deps{Patronymics: svc, DocsFS: fstest.MapFS{}}), "/api/patronymics")
	requireStatus(t, rec, 200)

	want := `[{"id":"PN-1","canonical":"Иванов","variants":[{"text":"Иванова"}],"items":[],"notes":[]}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestPatronymicGetNotFound(t *testing.T) {
	svc := &fakePatronymics{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Patronymics: svc, DocsFS: fstest.MapFS{}}), "/api/patronymics/PN-1")
	requireStatus(t, rec, 404)
}

func TestPatronymicSearchPassesQuery(t *testing.T) {
	svc := &fakePatronymics{}

	rec := get(t, NewHandler(Deps{Patronymics: svc, DocsFS: fstest.MapFS{}}), "/api/patronymics/search?q=Ив")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "Ив" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
```

#### `internal/httpapi/patronymic_write_test.go` (создать)
`internal/httpapi/patronymic_write_test.go`:
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

func TestPatronymicCreateContract(t *testing.T) {
	svc := &fakePatronymics{created: models.Patronymic{ID: "PN-1", Canonical: "Иванов"}}

	rec := postD(t, NewHandler(Deps{Patronymics: svc, DocsFS: fstest.MapFS{}}), "/api/patronymics", `{"canonical":"Иванов"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Canonical != "Иванов" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

func TestPatronymicCreateAnonymousIs401(t *testing.T) {
	svc := &fakePatronymics{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Patronymics: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/patronymics", strings.NewReader(`{"canonical":"Иванов"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestPatronymicUpdateMergesFields(t *testing.T) {
	svc := &fakePatronymics{getSN: models.Patronymic{ID: "PN-1", Canonical: "Иванов"}}

	rec := putD(t, NewHandler(Deps{Patronymics: svc, DocsFS: fstest.MapFS{}}), "/api/patronymics/PN-1",
		`{"canonical":"Иванов (испр.)","variants":[{"text":"Иванова"}]}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Canonical != "Иванов (испр.)" || svc.updated.ID != "PN-1" || len(svc.updated.Variants) != 1 {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestPatronymicDeleteNoContent(t *testing.T) {
	svc := &fakePatronymics{}

	rec := delD(t, NewHandler(Deps{Patronymics: svc, DocsFS: fstest.MapFS{}}), "/api/patronymics/PN-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestPatronymicDeleteInUseIs409(t *testing.T) {
	svc := &fakePatronymics{deleteErr: &models.InUseError{Type: models.TypePatronymic, ID: "PN-1"}}

	rec := delD(t, NewHandler(Deps{Patronymics: svc, DocsFS: fstest.MapFS{}}), "/api/patronymics/PN-1")

	requireStatus(t, rec, http.StatusConflict)
}
```

#### `internal/mcp/patronymic.go` (создать)
`internal/mcp/patronymic.go`:
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

// registerPatronymicTools регистрирует тулы для работы со словарными записями
// фамилий. Запись variants/items/notes — только текстом (v1, как и веб-форма,
// docs/data-model/entity-write.md §4): элемент с уже существующей ссылкой
// (Ref/Type) через эти тулы не создать, только прочитать (list/get/search
// отдают полный TextRef, включая Ref/Type). ВАЖНО: patronymic_update заменяет
// списки целиком текстом — существующие ref/type будут потеряны при любом
// обновлении через MCP, пока не появится picker (см. предупреждение в
// описании тула).
func registerPatronymicTools(s *server.MCPServer, patronymics PatronymicService) {
	tool := mcp.NewTool(
		"patronymic_list",
		mcp.WithDescription("Список словарных записей фамилий в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, patronymicListHandler(patronymics))

	tool = mcp.NewTool(
		"patronymic_search",
		mcp.WithDescription("Поиск словарных записей фамилий по началу канонической формы (включая варианты); результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало канонической формы или варианта")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, patronymicSearchHandler(patronymics))

	tool = mcp.NewTool(
		"patronymic_get",
		mcp.WithDescription("Словарная запись фамилии по id; результат — JSON записи. Неверный формат id или отсутствующая запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например PN-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, patronymicGetHandler(patronymics))

	tool = mcp.NewTool(
		"patronymic_create",
		mcp.WithDescription("Создать словарную запись фамилии; id генерируется сервером; результат — JSON созданной записи. variants/items/notes — списки текста (без ссылок на другие сущности, v1)"),
		mcp.WithString("canonical", mcp.Required(), mcp.Description("Каноническая форма")),
		mcp.WithArray("variants", mcp.WithStringItems(), mcp.Description("Варианты написания")),
		mcp.WithArray("items", mcp.WithStringItems(), mcp.Description("Носители/употребления — кто использует эту форму (текстом)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
	)
	s.AddTool(tool, patronymicCreateHandler(patronymics))

	tool = mcp.NewTool(
		"patronymic_update",
		mcp.WithDescription("Изменить словарную запись фамилии: полная замена canonical/variants/items/notes; результат — JSON обновлённой записи. variants/items/notes передаются целиком как текст — если у элемента раньше была ссылка на другую сущность (ref/type из patronymic_get), она будет потеряна: picker для ссылок ещё не реализован ни в вебе, ни в MCP (docs/data-model/entity-write.md §5)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("canonical", mcp.Required(), mcp.Description("Новая каноническая форма")),
		mcp.WithArray("variants", mcp.WithStringItems(), mcp.Description("Варианты написания")),
		mcp.WithArray("items", mcp.WithStringItems(), mcp.Description("Носители/употребления — кто использует эту форму (текстом)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
	)
	s.AddTool(tool, patronymicUpdateHandler(patronymics))

	tool = mcp.NewTool(
		"patronymic_delete",
		mcp.WithDescription("Удалить словарную запись фамилии. Необратимо. Если на неё есть строгие ссылки от других сущностей — ошибка тула (для Patronymic такое сегодня не создаётся, но общий механизм проверки один для всех сущностей)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, patronymicDeleteHandler(patronymics))
}

func patronymicListHandler(patronymics PatronymicService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := patronymics.ListPatronymics(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.PatronymicsFromModels(list))
	}
}

func patronymicSearchHandler(patronymics PatronymicService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := patronymics.SearchPatronymics(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.PatronymicsFromModels(list))
	}
}

func patronymicGetHandler(patronymics PatronymicService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sn, err := patronymics.GetPatronymic(ctx, models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.PatronymicFromModel(sn))
	}
}

func patronymicCreateHandler(patronymics PatronymicService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sn := models.Patronymic{
			Canonical: req.GetString("canonical", ""),
			Variants:  textRefsFromStrings(req.GetStringSlice("variants", nil)),
			Items:     textRefsFromStrings(req.GetStringSlice("items", nil)),
			Notes:     textRefsFromStrings(req.GetStringSlice("notes", nil)),
		}

		created, err := patronymics.CreatePatronymic(ctx, sn)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.PatronymicFromModel(created))
	}
}

func patronymicUpdateHandler(patronymics PatronymicService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := patronymics.GetPatronymic(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		cur.Canonical = req.GetString("canonical", "")
		cur.Variants = textRefsFromStrings(req.GetStringSlice("variants", nil))
		cur.Items = textRefsFromStrings(req.GetStringSlice("items", nil))
		cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))

		if err := patronymics.UpdatePatronymic(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.PatronymicFromModel(cur))
	}
}

func patronymicDeleteHandler(patronymics PatronymicService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := patronymics.DeletePatronymic(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
```

#### `internal/mcp/patronymic_test.go` (создать)
`internal/mcp/patronymic_test.go`:
```go
package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

// fakePatronymics запоминает запрос и отдаёт заданный ответ.
type fakePatronymics struct {
	list []models.Patronymic
	err  error

	getSN     models.Patronymic
	created   models.Patronymic
	gotCreate models.Patronymic
	updated   models.Patronymic
	gotIDs    []models.ID
	deleteErr error

	search []models.Patronymic
}

func (f *fakePatronymics) ListPatronymics(context.Context, models.Access, models.Page) ([]models.Patronymic, error) {
	return f.list, f.err
}

func (f *fakePatronymics) SearchPatronymics(context.Context, models.Access, models.SearchQuery) ([]models.Patronymic, error) {
	return f.search, f.err
}

func (f *fakePatronymics) GetPatronymic(_ context.Context, id models.ID) (models.Patronymic, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Patronymic{}, f.err
	}

	return f.getSN, nil
}

func (f *fakePatronymics) CreatePatronymic(_ context.Context, sn models.Patronymic) (models.Patronymic, error) {
	f.gotCreate = sn
	if f.err != nil {
		return models.Patronymic{}, f.err
	}

	return f.created, nil
}

func (f *fakePatronymics) UpdatePatronymic(_ context.Context, sn models.Patronymic) error {
	f.updated = sn

	return f.err
}

func (f *fakePatronymics) DeletePatronymic(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callPatronymicTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestPatronymicGetToolContract(t *testing.T) {
	svc := &fakePatronymics{getSN: models.Patronymic{ID: "PN-1", Canonical: "Иванов"}}

	res := callPatronymicTool(t, patronymicGetHandler(svc), map[string]any{"id": "PN-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "PN-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

func TestPatronymicCreateToolPassesVariants(t *testing.T) {
	svc := &fakePatronymics{created: models.Patronymic{ID: "PN-new", Canonical: "Иванов"}}

	res := callPatronymicTool(t, patronymicCreateHandler(svc), map[string]any{
		"canonical": "Иванов",
		"variants":  []any{"Иванова"},
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.Canonical != "Иванов" || len(svc.gotCreate.Variants) != 1 || svc.gotCreate.Variants[0].Text != "Иванова" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

func TestPatronymicDeleteToolInUseIsError(t *testing.T) {
	svc := &fakePatronymics{deleteErr: &models.InUseError{Type: models.TypePatronymic, ID: "PN-1"}}

	res := callPatronymicTool(t, patronymicDeleteHandler(svc), map[string]any{"id": "PN-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersPatronymicTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Patronymics: &fakePatronymics{}}).ListTools()

	for _, name := range []string{"patronymic_list", "patronymic_search", "patronymic_get", "patronymic_create", "patronymic_update", "patronymic_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
```

### Шаг 1.2. Estate — транспорт, usecases, httpapi, MCP

#### `internal/transport/estate.go` (создать)
`internal/transport/estate.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// Estate — контракт словарной записи фамилии (GET /api/estates, MCP-тул estate_list).
type Estate struct {
	ID        models.ID `json:"id"`
	Canonical string    `json:"canonical"`
	Variants  []TextRef `json:"variants"`
	Items     []TextRef `json:"items"`
	Notes     []TextRef `json:"notes"`
}

// EstateFromModel конвертирует запись в контракт.
func EstateFromModel(s models.Estate) Estate {
	return Estate{
		ID:        s.ID,
		Canonical: s.Canonical,
		Variants:  TextRefsFromModel(s.Variants),
		Items:     TextRefsFromModel(s.Items),
		Notes:     TextRefsFromModel(s.Notes),
	}
}

// EstatesFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func EstatesFromModels(ss []models.Estate) []Estate {
	out := make([]Estate, 0, len(ss))
	for _, s := range ss {
		out = append(out, EstateFromModel(s))
	}

	return out
}
```

#### `internal/transport/estate_write.go` (создать)
`internal/transport/estate_write.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// EstateCreate — тело POST /api/estates и аргументы тула estate_create.
// Идентификатор генерирует сценарий.
type EstateCreate struct {
	Canonical string    `json:"canonical"`
	Variants  []TextRef `json:"variants"`
	Items     []TextRef `json:"items"`
	Notes     []TextRef `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (s EstateCreate) Model() models.Estate {
	return models.Estate{
		Canonical: s.Canonical,
		Variants:  TextRefsToModel(s.Variants),
		Items:     TextRefsToModel(s.Items),
		Notes:     TextRefsToModel(s.Notes),
	}
}

// EstateUpdate — тело PUT /api/estates/{id} и аргументы тула estate_update:
// полная замена canonical/variants/items/notes.
type EstateUpdate struct {
	Canonical string    `json:"canonical"`
	Variants  []TextRef `json:"variants"`
	Items     []TextRef `json:"items"`
	Notes     []TextRef `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (s EstateUpdate) Model() models.Estate {
	return models.Estate{
		Canonical: s.Canonical,
		Variants:  TextRefsToModel(s.Variants),
		Items:     TextRefsToModel(s.Items),
		Notes:     TextRefsToModel(s.Notes),
	}
}
```

#### `internal/usecases/list_estates/` (создать)
`internal/usecases/list_estates/deps.go`:
```go
package list_estates

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// EstateRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type EstateRepo interface {
	ListEstates(ctx context.Context, access models.Access, page models.Page) ([]*models.Estate, error)
}
```

`internal/usecases/list_estates/scenario.go`:
```go
package list_estates

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список словарных записей фамилий».
type Scenario struct {
	estates EstateRepo
}

// New создаёт сценарий.
func New(estates EstateRepo) *Scenario {
	return &Scenario{estates: estates}
}

// ListEstates возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListEstates(ctx context.Context, access models.Access, page models.Page) ([]models.Estate, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeEstate, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeEstate, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.estates.ListEstates(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Estate, 0, len(list))
	for _, sn := range list {
		out = append(out, *sn)
	}

	return out, nil
}
```

`internal/usecases/list_estates/scenario_test.go`:
```go
package list_estates

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.Estate
}

func (f *fakeRepo) ListEstates(_ context.Context, _ models.Access, page models.Page) ([]*models.Estate, error) {
	f.page = page

	return f.out, nil
}

func TestListEstatesReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.Estate{{ID: "ES-01ARZ3NDEKTSV4RRFFQ69G5FA1", Canonical: "Иванов"}}}

	got, err := New(repo).ListEstates(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListEstates: %v", err)
	}

	if len(got) != 1 || got[0].Canonical != "Иванов" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListEstatesRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListEstates(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
```

#### `internal/usecases/search_estates/` (создать)
`internal/usecases/search_estates/deps.go`:
```go
package search_estates

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// EstateRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type EstateRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetEstate(ctx context.Context, id models.ID) (*models.Estate, error)
}
```

`internal/usecases/search_estates/scenario.go`:
```go
package search_estates

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск словарных записей фамилий».
type Scenario struct {
	estates EstateRepo
}

// New создаёт сценарий.
func New(estates EstateRepo) *Scenario {
	return &Scenario{estates: estates}
}

// SearchEstates находит записи, чья каноническая форма (или вариант)
// начинается с текста запроса — та же механика, что и
// search_divisions.SearchDivisions (см. её комментарий).
func (s *Scenario) SearchEstates(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Estate, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Estate{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Estate{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.estates.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeEstate {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.estates.GetEstate(ctx, h.ID)
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

`internal/usecases/search_estates/scenario_test.go`:
```go
package search_estates

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits    []models.Hit
	estates map[models.ID]*models.Estate
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetEstate(_ context.Context, id models.ID) (*models.Estate, error) {
	s, ok := f.estates[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestSearchEstatesFiltersByType(t *testing.T) {
	id := models.ID("ES-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeEstate, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не estate — должен быть пропущен
		},
		estates: map[models.ID]*models.Estate{id: {ID: id, Canonical: "Иванов"}},
	}

	got, err := New(repo).SearchEstates(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchEstates: %v", err)
	}

	if len(got) != 1 || got[0].Canonical != "Иванов" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchEstatesEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchEstates(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchEstates: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
```

#### `internal/usecases/get_estate/` (создать)
`internal/usecases/get_estate/deps.go`:
```go
package get_estate

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// EstateRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type EstateRepo interface {
	GetEstate(ctx context.Context, id models.ID) (*models.Estate, error)
}
```

`internal/usecases/get_estate/scenario.go`:
```go
package get_estate

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «словарная запись фамилии по идентификатору».
type Scenario struct {
	estates EstateRepo
}

// New создаёт сценарий.
func New(estates EstateRepo) *Scenario {
	return &Scenario{estates: estates}
}

// GetEstate возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound.
func (s *Scenario) GetEstate(ctx context.Context, id models.ID) (models.Estate, error) {
	if err := validateID(id); err != nil {
		return models.Estate{}, err
	}

	sn, err := s.estates.GetEstate(ctx, id)
	if err != nil {
		return models.Estate{}, err
	}

	return *sn, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeEstate)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeEstate,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

`internal/usecases/get_estate/scenario_test.go`:
```go
package get_estate

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	estates map[models.ID]*models.Estate
}

func (f *fakeRepo) GetEstate(_ context.Context, id models.ID) (*models.Estate, error) {
	s, ok := f.estates[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestGetEstateReturnsRecord(t *testing.T) {
	id := models.ID("ES-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{estates: map[models.ID]*models.Estate{id: {ID: id, Canonical: "Иванов"}}}

	got, err := New(repo).GetEstate(context.Background(), id)
	if err != nil {
		t.Fatalf("GetEstate: %v", err)
	}

	if got.Canonical != "Иванов" {
		t.Fatalf("Canonical = %q", got.Canonical)
	}
}

func TestGetEstateNotFound(t *testing.T) {
	repo := &fakeRepo{estates: map[models.ID]*models.Estate{}}

	_, err := New(repo).GetEstate(context.Background(), "ES-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetEstateInvalidID(t *testing.T) {
	repo := &fakeRepo{estates: map[models.ID]*models.Estate{}}

	_, err := New(repo).GetEstate(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}
```

#### `internal/usecases/create_estate/` (создать)
`internal/usecases/create_estate/deps.go`:
```go
package create_estate

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// EstateStore — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type EstateStore interface {
	SaveEstate(ctx context.Context, s *models.Estate) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
```

`internal/usecases/create_estate/scenario.go`:
```go
package create_estate

import (
	"context"
	"fmt"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «создание словарной записи фамилии».
type Scenario struct {
	store EstateStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st EstateStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateEstate создаёт запись: генерирует идентификатор, проверяет
// инварианты, сохраняет. Возвращает созданную запись с заполненным ID.
// Флат-сущность без FK — в отличие от create_division, без транзакционной
// проверки родителя.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта) и невалидная сущность — *models.ValidationError; прочее —
// ошибки хранилища как есть.
func (s *Scenario) CreateEstate(ctx context.Context, sn models.Estate) (models.Estate, error) {
	if sn.ID != "" {
		return models.Estate{}, &models.ValidationError{
			Entity: models.TypeEstate,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", sn.ID),
		}
	}

	sn.ID = s.ids.New(models.TypeEstate)

	if err := sn.Validate(); err != nil {
		return models.Estate{}, err
	}

	if err := s.store.SaveEstate(ctx, &sn); err != nil {
		return models.Estate{}, err
	}

	return sn, nil
}
```

`internal/usecases/create_estate/scenario_test.go`:
```go
package create_estate

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type stubIDs struct{ id models.ID }

func (s *stubIDs) New(models.Type) models.ID { return s.id }

type fakeStore struct {
	saved   *models.Estate
	saveErr error
}

func (f *fakeStore) SaveEstate(_ context.Context, s *models.Estate) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *s
	f.saved = &cp

	return nil
}

func TestCreateEstateGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "ES-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{}
	sc := New(st, ids)

	got, err := sc.CreateEstate(context.Background(), models.Estate{Canonical: "Иванов"})
	if err != nil {
		t.Fatalf("CreateEstate: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.saved == nil || st.saved.Canonical != "Иванов" {
		t.Fatalf("saved = %+v", st.saved)
	}
}

func TestCreateEstateRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{})

	_, err := sc.CreateEstate(context.Background(), models.Estate{ID: "ES-01ARZ3NDEKTSV4RRFFQ69G5FA1", Canonical: "x"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestCreateEstateRejectsEmptyCanonical(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{id: "ES-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateEstate(context.Background(), models.Estate{})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "canonical" {
		t.Fatalf("err = %v, want ValidationError on canonical", err)
	}
}

func TestCreateEstatePropagatesStoreError(t *testing.T) {
	wantErr := errors.New("db down")
	st := &fakeStore{saveErr: wantErr}
	sc := New(st, &stubIDs{id: "ES-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateEstate(context.Background(), models.Estate{Canonical: "Иванов"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}
```

#### `internal/usecases/update_estate/` (создать)
`internal/usecases/update_estate/deps.go`:
```go
package update_estate

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// EstateStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования и сохранение идут в одной транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type EstateStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
```

`internal/usecases/update_estate/scenario.go`:
```go
package update_estate

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение словарной записи фамилии».
type Scenario struct {
	store EstateStore
}

// New создаёт сценарий.
func New(st EstateStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateEstate полностью заменяет запись по sn.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, и сохраняет.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateEstate(ctx context.Context, sn models.Estate) error {
	if err := sn.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetEstate(ctx, sn.ID); err != nil {
			return err
		}

		return tx.SaveEstate(ctx, &sn)
	})
}
```

`internal/usecases/update_estate/scenario_test.go`:
```go
package update_estate

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
	estates map[models.ID]*models.Estate
	saved   []*models.Estate
}

func newFakeTx(existing ...*models.Estate) *fakeTx {
	tx := &fakeTx{estates: map[models.ID]*models.Estate{}}
	for _, s := range existing {
		tx.estates[s.ID] = s
	}

	return tx
}

func (f *fakeTx) GetEstate(_ context.Context, id models.ID) (*models.Estate, error) {
	s, ok := f.estates[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *s

	return &cp, nil
}

func (f *fakeTx) SaveEstate(_ context.Context, s *models.Estate) error {
	cp := *s
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует EstateStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func estate(id models.ID, canonical string) *models.Estate {
	return &models.Estate{ID: id, Canonical: canonical}
}

func TestUpdateEstateSaves(t *testing.T) {
	existing := estate("ES-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Иванов")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Canonical = "Иванов (испр.)"

	if err := New(st).UpdateEstate(context.Background(), updated); err != nil {
		t.Fatalf("UpdateEstate: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Canonical != "Иванов (испр.)" {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateEstateNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateEstate(context.Background(), *estate("ES-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Иванов"))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateEstateRejectsEmptyCanonical(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateEstate(context.Background(), models.Estate{ID: "ES-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "canonical" {
		t.Fatalf("err = %v, want ValidationError on canonical", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}
```

#### `internal/usecases/delete_estate/` (создать)
`internal/usecases/delete_estate/deps.go`:
```go
package delete_estate

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// EstateRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type EstateRepo interface {
	DeleteEstate(ctx context.Context, id models.ID) error
}
```

`internal/usecases/delete_estate/scenario.go`:
```go
package delete_estate

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление словарной записи фамилии».
type Scenario struct {
	estates EstateRepo
}

// New создаёт сценарий.
func New(estates EstateRepo) *Scenario {
	return &Scenario{estates: estates}
}

// DeleteEstate удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteEstate(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.estates.DeleteEstate(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeEstate)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeEstate,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

`internal/usecases/delete_estate/scenario_test.go`:
```go
package delete_estate

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

func (f *fakeRepo) DeleteEstate(_ context.Context, id models.ID) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)

	return nil
}

func TestDeleteEstateCallsRepo(t *testing.T) {
	repo := &fakeRepo{}
	id := models.ID("ES-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	if err := New(repo).DeleteEstate(context.Background(), id); err != nil {
		t.Fatalf("DeleteEstate: %v", err)
	}

	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("deleted = %v", repo.deleted)
	}
}

func TestDeleteEstateInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	err := New(repo).DeleteEstate(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}

	if len(repo.deleted) != 0 {
		t.Fatalf("repo вызван при невалидном id")
	}
}

func TestDeleteEstatePropagatesInUseError(t *testing.T) {
	wantErr := &models.InUseError{Type: models.TypeEstate, ID: "ES-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	repo := &fakeRepo{err: wantErr}

	err := New(repo).DeleteEstate(context.Background(), "ES-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	var iu *models.InUseError
	if !errors.As(err, &iu) {
		t.Fatalf("err = %v, want InUseError", err)
	}
}
```

#### `internal/httpapi/estate.go` (создать)
`internal/httpapi/estate.go`:
```go
package httpapi

import (
	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
	"net/http"
)

// handleEstateList — GET /api/estates?limit=&offset=. Чтение открыто
// анонимному посетителю (как и деления — auth.md §6, Access здесь не
// проверяется).
func handleEstateList(estates EstateService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := estates.ListEstates(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.EstatesFromModels(list))
	}
}

// handleEstateSearch — GET /api/estates/search?q=&limit=&offset=.
func handleEstateSearch(estates EstateService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := estates.SearchEstates(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.EstatesFromModels(list))
	}
}

// handleEstateGet — GET /api/estates/{id}.
func handleEstateGet(estates EstateService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sn, err := estates.GetEstate(r.Context(), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.EstateFromModel(sn))
	}
}
```

#### `internal/httpapi/estate_write.go` (создать)
`internal/httpapi/estate_write.go`:
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleEstateCreate — POST /api/estates: создаёт запись, отвечает 201 с
// созданной записью (id генерирует сценарий). Запись — только для вошедшего
// владельца (auth.md §6, решение 9), см. handleDivisionCreate.
func handleEstateCreate(estates EstateService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.EstateCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := estates.CreateEstate(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.EstateFromModel(created))
	}
}

// handleEstateUpdate — PUT /api/estates/{id}: полная замена
// canonical/variants/items/notes. Читает текущую версию, накладывает поля
// запроса (fetch-then-merge — как handleDivisionUpdate; у Estate сейчас DTO
// покрывает всю модель, но конвенция общая для будущих сущностей, чей DTO
// v1 не покрывает всех полей модели, docs/data-model/entity-write.md §3).
func handleEstateUpdate(estates EstateService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.EstateUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := estates.GetEstate(r.Context(), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Canonical = m.Canonical
		cur.Variants = m.Variants
		cur.Items = m.Items
		cur.Notes = m.Notes

		if err := estates.UpdateEstate(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.EstateFromModel(cur))
	}
}

// handleEstateDelete — DELETE /api/estates/{id}: 204 без тела; занятая
// запись — 409 со списком ссылающихся. Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handleEstateDelete(estates EstateService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := estates.DeleteEstate(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
```

#### `internal/httpapi/estate_test.go` (создать)
`internal/httpapi/estate_test.go`:
```go
package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeEstates struct {
	list []models.Estate
	err  error
	page models.Page

	getSN     models.Estate
	gotIDs    []models.ID
	created   models.Estate
	gotCreate models.Estate
	updated   models.Estate
	deleteErr error

	search    []models.Estate
	gotSearch models.SearchQuery
}

func (f *fakeEstates) ListEstates(_ context.Context, _ models.Access, page models.Page) ([]models.Estate, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeEstates) SearchEstates(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Estate, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeEstates) GetEstate(_ context.Context, id models.ID) (models.Estate, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Estate{}, f.err
	}

	return f.getSN, nil
}

func (f *fakeEstates) CreateEstate(_ context.Context, sn models.Estate) (models.Estate, error) {
	f.gotCreate = sn
	if f.err != nil {
		return models.Estate{}, f.err
	}

	return f.created, nil
}

func (f *fakeEstates) UpdateEstate(_ context.Context, sn models.Estate) error {
	f.updated = sn

	return f.err
}

func (f *fakeEstates) DeleteEstate(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestEstateListReturnsRecords(t *testing.T) {
	svc := &fakeEstates{list: []models.Estate{{ID: "ES-1", Canonical: "Иванов", Variants: []models.TextRef{{Text: "Иванова"}}}}}

	rec := get(t, NewHandler(Deps{Estates: svc, DocsFS: fstest.MapFS{}}), "/api/estates")
	requireStatus(t, rec, 200)

	want := `[{"id":"ES-1","canonical":"Иванов","variants":[{"text":"Иванова"}],"items":[],"notes":[]}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestEstateGetNotFound(t *testing.T) {
	svc := &fakeEstates{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Estates: svc, DocsFS: fstest.MapFS{}}), "/api/estates/ES-1")
	requireStatus(t, rec, 404)
}

func TestEstateSearchPassesQuery(t *testing.T) {
	svc := &fakeEstates{}

	rec := get(t, NewHandler(Deps{Estates: svc, DocsFS: fstest.MapFS{}}), "/api/estates/search?q=Ив")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "Ив" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
```

#### `internal/httpapi/estate_write_test.go` (создать)
`internal/httpapi/estate_write_test.go`:
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

func TestEstateCreateContract(t *testing.T) {
	svc := &fakeEstates{created: models.Estate{ID: "ES-1", Canonical: "Иванов"}}

	rec := postD(t, NewHandler(Deps{Estates: svc, DocsFS: fstest.MapFS{}}), "/api/estates", `{"canonical":"Иванов"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Canonical != "Иванов" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

func TestEstateCreateAnonymousIs401(t *testing.T) {
	svc := &fakeEstates{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Estates: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/estates", strings.NewReader(`{"canonical":"Иванов"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestEstateUpdateMergesFields(t *testing.T) {
	svc := &fakeEstates{getSN: models.Estate{ID: "ES-1", Canonical: "Иванов"}}

	rec := putD(t, NewHandler(Deps{Estates: svc, DocsFS: fstest.MapFS{}}), "/api/estates/ES-1",
		`{"canonical":"Иванов (испр.)","variants":[{"text":"Иванова"}]}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Canonical != "Иванов (испр.)" || svc.updated.ID != "ES-1" || len(svc.updated.Variants) != 1 {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestEstateDeleteNoContent(t *testing.T) {
	svc := &fakeEstates{}

	rec := delD(t, NewHandler(Deps{Estates: svc, DocsFS: fstest.MapFS{}}), "/api/estates/ES-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestEstateDeleteInUseIs409(t *testing.T) {
	svc := &fakeEstates{deleteErr: &models.InUseError{Type: models.TypeEstate, ID: "ES-1"}}

	rec := delD(t, NewHandler(Deps{Estates: svc, DocsFS: fstest.MapFS{}}), "/api/estates/ES-1")

	requireStatus(t, rec, http.StatusConflict)
}
```

#### `internal/mcp/estate.go` (создать)
`internal/mcp/estate.go`:
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

// registerEstateTools регистрирует тулы для работы со словарными записями
// фамилий. Запись variants/items/notes — только текстом (v1, как и веб-форма,
// docs/data-model/entity-write.md §4): элемент с уже существующей ссылкой
// (Ref/Type) через эти тулы не создать, только прочитать (list/get/search
// отдают полный TextRef, включая Ref/Type). ВАЖНО: estate_update заменяет
// списки целиком текстом — существующие ref/type будут потеряны при любом
// обновлении через MCP, пока не появится picker (см. предупреждение в
// описании тула).
func registerEstateTools(s *server.MCPServer, estates EstateService) {
	tool := mcp.NewTool(
		"estate_list",
		mcp.WithDescription("Список словарных записей фамилий в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, estateListHandler(estates))

	tool = mcp.NewTool(
		"estate_search",
		mcp.WithDescription("Поиск словарных записей фамилий по началу канонической формы (включая варианты); результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало канонической формы или варианта")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, estateSearchHandler(estates))

	tool = mcp.NewTool(
		"estate_get",
		mcp.WithDescription("Словарная запись фамилии по id; результат — JSON записи. Неверный формат id или отсутствующая запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например ES-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, estateGetHandler(estates))

	tool = mcp.NewTool(
		"estate_create",
		mcp.WithDescription("Создать словарную запись фамилии; id генерируется сервером; результат — JSON созданной записи. variants/items/notes — списки текста (без ссылок на другие сущности, v1)"),
		mcp.WithString("canonical", mcp.Required(), mcp.Description("Каноническая форма")),
		mcp.WithArray("variants", mcp.WithStringItems(), mcp.Description("Варианты написания")),
		mcp.WithArray("items", mcp.WithStringItems(), mcp.Description("Носители/употребления — кто использует эту форму (текстом)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
	)
	s.AddTool(tool, estateCreateHandler(estates))

	tool = mcp.NewTool(
		"estate_update",
		mcp.WithDescription("Изменить словарную запись фамилии: полная замена canonical/variants/items/notes; результат — JSON обновлённой записи. variants/items/notes передаются целиком как текст — если у элемента раньше была ссылка на другую сущность (ref/type из estate_get), она будет потеряна: picker для ссылок ещё не реализован ни в вебе, ни в MCP (docs/data-model/entity-write.md §5)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("canonical", mcp.Required(), mcp.Description("Новая каноническая форма")),
		mcp.WithArray("variants", mcp.WithStringItems(), mcp.Description("Варианты написания")),
		mcp.WithArray("items", mcp.WithStringItems(), mcp.Description("Носители/употребления — кто использует эту форму (текстом)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
	)
	s.AddTool(tool, estateUpdateHandler(estates))

	tool = mcp.NewTool(
		"estate_delete",
		mcp.WithDescription("Удалить словарную запись фамилии. Необратимо. Если на неё есть строгие ссылки от других сущностей — ошибка тула (для Estate такое сегодня не создаётся, но общий механизм проверки один для всех сущностей)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, estateDeleteHandler(estates))
}

func estateListHandler(estates EstateService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := estates.ListEstates(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.EstatesFromModels(list))
	}
}

func estateSearchHandler(estates EstateService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := estates.SearchEstates(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.EstatesFromModels(list))
	}
}

func estateGetHandler(estates EstateService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sn, err := estates.GetEstate(ctx, models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.EstateFromModel(sn))
	}
}

func estateCreateHandler(estates EstateService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sn := models.Estate{
			Canonical: req.GetString("canonical", ""),
			Variants:  textRefsFromStrings(req.GetStringSlice("variants", nil)),
			Items:     textRefsFromStrings(req.GetStringSlice("items", nil)),
			Notes:     textRefsFromStrings(req.GetStringSlice("notes", nil)),
		}

		created, err := estates.CreateEstate(ctx, sn)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.EstateFromModel(created))
	}
}

func estateUpdateHandler(estates EstateService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := estates.GetEstate(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		cur.Canonical = req.GetString("canonical", "")
		cur.Variants = textRefsFromStrings(req.GetStringSlice("variants", nil))
		cur.Items = textRefsFromStrings(req.GetStringSlice("items", nil))
		cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))

		if err := estates.UpdateEstate(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.EstateFromModel(cur))
	}
}

func estateDeleteHandler(estates EstateService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := estates.DeleteEstate(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
```

#### `internal/mcp/estate_test.go` (создать)
`internal/mcp/estate_test.go`:
```go
package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

// fakeEstates запоминает запрос и отдаёт заданный ответ.
type fakeEstates struct {
	list []models.Estate
	err  error

	getSN     models.Estate
	created   models.Estate
	gotCreate models.Estate
	updated   models.Estate
	gotIDs    []models.ID
	deleteErr error

	search []models.Estate
}

func (f *fakeEstates) ListEstates(context.Context, models.Access, models.Page) ([]models.Estate, error) {
	return f.list, f.err
}

func (f *fakeEstates) SearchEstates(context.Context, models.Access, models.SearchQuery) ([]models.Estate, error) {
	return f.search, f.err
}

func (f *fakeEstates) GetEstate(_ context.Context, id models.ID) (models.Estate, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Estate{}, f.err
	}

	return f.getSN, nil
}

func (f *fakeEstates) CreateEstate(_ context.Context, sn models.Estate) (models.Estate, error) {
	f.gotCreate = sn
	if f.err != nil {
		return models.Estate{}, f.err
	}

	return f.created, nil
}

func (f *fakeEstates) UpdateEstate(_ context.Context, sn models.Estate) error {
	f.updated = sn

	return f.err
}

func (f *fakeEstates) DeleteEstate(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callEstateTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestEstateGetToolContract(t *testing.T) {
	svc := &fakeEstates{getSN: models.Estate{ID: "ES-1", Canonical: "Иванов"}}

	res := callEstateTool(t, estateGetHandler(svc), map[string]any{"id": "ES-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "ES-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

func TestEstateCreateToolPassesVariants(t *testing.T) {
	svc := &fakeEstates{created: models.Estate{ID: "ES-new", Canonical: "Иванов"}}

	res := callEstateTool(t, estateCreateHandler(svc), map[string]any{
		"canonical": "Иванов",
		"variants":  []any{"Иванова"},
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.Canonical != "Иванов" || len(svc.gotCreate.Variants) != 1 || svc.gotCreate.Variants[0].Text != "Иванова" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

func TestEstateDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeEstates{deleteErr: &models.InUseError{Type: models.TypeEstate, ID: "ES-1"}}

	res := callEstateTool(t, estateDeleteHandler(svc), map[string]any{"id": "ES-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersEstateTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Estates: &fakeEstates{}}).ListTools()

	for _, name := range []string{"estate_list", "estate_search", "estate_get", "estate_create", "estate_update", "estate_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
```

### Шаг 1.3. Title — транспорт, usecases, httpapi, MCP

#### `internal/transport/title.go` (создать)
`internal/transport/title.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// Title — контракт словарной записи фамилии (GET /api/titles, MCP-тул title_list).
type Title struct {
	ID        models.ID `json:"id"`
	Canonical string    `json:"canonical"`
	Variants  []TextRef `json:"variants"`
	Items     []TextRef `json:"items"`
	Notes     []TextRef `json:"notes"`
}

// TitleFromModel конвертирует запись в контракт.
func TitleFromModel(s models.Title) Title {
	return Title{
		ID:        s.ID,
		Canonical: s.Canonical,
		Variants:  TextRefsFromModel(s.Variants),
		Items:     TextRefsFromModel(s.Items),
		Notes:     TextRefsFromModel(s.Notes),
	}
}

// TitlesFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func TitlesFromModels(ss []models.Title) []Title {
	out := make([]Title, 0, len(ss))
	for _, s := range ss {
		out = append(out, TitleFromModel(s))
	}

	return out
}
```

#### `internal/transport/title_write.go` (создать)
`internal/transport/title_write.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// TitleCreate — тело POST /api/titles и аргументы тула title_create.
// Идентификатор генерирует сценарий.
type TitleCreate struct {
	Canonical string    `json:"canonical"`
	Variants  []TextRef `json:"variants"`
	Items     []TextRef `json:"items"`
	Notes     []TextRef `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (s TitleCreate) Model() models.Title {
	return models.Title{
		Canonical: s.Canonical,
		Variants:  TextRefsToModel(s.Variants),
		Items:     TextRefsToModel(s.Items),
		Notes:     TextRefsToModel(s.Notes),
	}
}

// TitleUpdate — тело PUT /api/titles/{id} и аргументы тула title_update:
// полная замена canonical/variants/items/notes.
type TitleUpdate struct {
	Canonical string    `json:"canonical"`
	Variants  []TextRef `json:"variants"`
	Items     []TextRef `json:"items"`
	Notes     []TextRef `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (s TitleUpdate) Model() models.Title {
	return models.Title{
		Canonical: s.Canonical,
		Variants:  TextRefsToModel(s.Variants),
		Items:     TextRefsToModel(s.Items),
		Notes:     TextRefsToModel(s.Notes),
	}
}
```

#### `internal/usecases/list_titles/` (создать)
`internal/usecases/list_titles/deps.go`:
```go
package list_titles

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// TitleRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type TitleRepo interface {
	ListTitles(ctx context.Context, access models.Access, page models.Page) ([]*models.Title, error)
}
```

`internal/usecases/list_titles/scenario.go`:
```go
package list_titles

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список словарных записей фамилий».
type Scenario struct {
	titles TitleRepo
}

// New создаёт сценарий.
func New(titles TitleRepo) *Scenario {
	return &Scenario{titles: titles}
}

// ListTitles возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListTitles(ctx context.Context, access models.Access, page models.Page) ([]models.Title, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeTitle, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeTitle, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.titles.ListTitles(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Title, 0, len(list))
	for _, sn := range list {
		out = append(out, *sn)
	}

	return out, nil
}
```

`internal/usecases/list_titles/scenario_test.go`:
```go
package list_titles

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.Title
}

func (f *fakeRepo) ListTitles(_ context.Context, _ models.Access, page models.Page) ([]*models.Title, error) {
	f.page = page

	return f.out, nil
}

func TestListTitlesReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.Title{{ID: "TT-01ARZ3NDEKTSV4RRFFQ69G5FA1", Canonical: "Иванов"}}}

	got, err := New(repo).ListTitles(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListTitles: %v", err)
	}

	if len(got) != 1 || got[0].Canonical != "Иванов" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListTitlesRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListTitles(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
```

#### `internal/usecases/search_titles/` (создать)
`internal/usecases/search_titles/deps.go`:
```go
package search_titles

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// TitleRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type TitleRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetTitle(ctx context.Context, id models.ID) (*models.Title, error)
}
```

`internal/usecases/search_titles/scenario.go`:
```go
package search_titles

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск словарных записей фамилий».
type Scenario struct {
	titles TitleRepo
}

// New создаёт сценарий.
func New(titles TitleRepo) *Scenario {
	return &Scenario{titles: titles}
}

// SearchTitles находит записи, чья каноническая форма (или вариант)
// начинается с текста запроса — та же механика, что и
// search_divisions.SearchDivisions (см. её комментарий).
func (s *Scenario) SearchTitles(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Title, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Title{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Title{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.titles.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeTitle {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.titles.GetTitle(ctx, h.ID)
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

`internal/usecases/search_titles/scenario_test.go`:
```go
package search_titles

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits   []models.Hit
	titles map[models.ID]*models.Title
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetTitle(_ context.Context, id models.ID) (*models.Title, error) {
	s, ok := f.titles[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestSearchTitlesFiltersByType(t *testing.T) {
	id := models.ID("TT-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeTitle, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не title — должен быть пропущен
		},
		titles: map[models.ID]*models.Title{id: {ID: id, Canonical: "Иванов"}},
	}

	got, err := New(repo).SearchTitles(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchTitles: %v", err)
	}

	if len(got) != 1 || got[0].Canonical != "Иванов" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchTitlesEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchTitles(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchTitles: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
```

#### `internal/usecases/get_title/` (создать)
`internal/usecases/get_title/deps.go`:
```go
package get_title

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// TitleRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type TitleRepo interface {
	GetTitle(ctx context.Context, id models.ID) (*models.Title, error)
}
```

`internal/usecases/get_title/scenario.go`:
```go
package get_title

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «словарная запись фамилии по идентификатору».
type Scenario struct {
	titles TitleRepo
}

// New создаёт сценарий.
func New(titles TitleRepo) *Scenario {
	return &Scenario{titles: titles}
}

// GetTitle возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound.
func (s *Scenario) GetTitle(ctx context.Context, id models.ID) (models.Title, error) {
	if err := validateID(id); err != nil {
		return models.Title{}, err
	}

	sn, err := s.titles.GetTitle(ctx, id)
	if err != nil {
		return models.Title{}, err
	}

	return *sn, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeTitle)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeTitle,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

`internal/usecases/get_title/scenario_test.go`:
```go
package get_title

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	titles map[models.ID]*models.Title
}

func (f *fakeRepo) GetTitle(_ context.Context, id models.ID) (*models.Title, error) {
	s, ok := f.titles[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestGetTitleReturnsRecord(t *testing.T) {
	id := models.ID("TT-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{titles: map[models.ID]*models.Title{id: {ID: id, Canonical: "Иванов"}}}

	got, err := New(repo).GetTitle(context.Background(), id)
	if err != nil {
		t.Fatalf("GetTitle: %v", err)
	}

	if got.Canonical != "Иванов" {
		t.Fatalf("Canonical = %q", got.Canonical)
	}
}

func TestGetTitleNotFound(t *testing.T) {
	repo := &fakeRepo{titles: map[models.ID]*models.Title{}}

	_, err := New(repo).GetTitle(context.Background(), "TT-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetTitleInvalidID(t *testing.T) {
	repo := &fakeRepo{titles: map[models.ID]*models.Title{}}

	_, err := New(repo).GetTitle(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}
```

#### `internal/usecases/create_title/` (создать)
`internal/usecases/create_title/deps.go`:
```go
package create_title

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// TitleStore — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type TitleStore interface {
	SaveTitle(ctx context.Context, s *models.Title) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
```

`internal/usecases/create_title/scenario.go`:
```go
package create_title

import (
	"context"
	"fmt"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «создание словарной записи фамилии».
type Scenario struct {
	store TitleStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st TitleStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateTitle создаёт запись: генерирует идентификатор, проверяет
// инварианты, сохраняет. Возвращает созданную запись с заполненным ID.
// Флат-сущность без FK — в отличие от create_division, без транзакционной
// проверки родителя.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта) и невалидная сущность — *models.ValidationError; прочее —
// ошибки хранилища как есть.
func (s *Scenario) CreateTitle(ctx context.Context, sn models.Title) (models.Title, error) {
	if sn.ID != "" {
		return models.Title{}, &models.ValidationError{
			Entity: models.TypeTitle,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", sn.ID),
		}
	}

	sn.ID = s.ids.New(models.TypeTitle)

	if err := sn.Validate(); err != nil {
		return models.Title{}, err
	}

	if err := s.store.SaveTitle(ctx, &sn); err != nil {
		return models.Title{}, err
	}

	return sn, nil
}
```

`internal/usecases/create_title/scenario_test.go`:
```go
package create_title

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type stubIDs struct{ id models.ID }

func (s *stubIDs) New(models.Type) models.ID { return s.id }

type fakeStore struct {
	saved   *models.Title
	saveErr error
}

func (f *fakeStore) SaveTitle(_ context.Context, s *models.Title) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *s
	f.saved = &cp

	return nil
}

func TestCreateTitleGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "TT-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{}
	sc := New(st, ids)

	got, err := sc.CreateTitle(context.Background(), models.Title{Canonical: "Иванов"})
	if err != nil {
		t.Fatalf("CreateTitle: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.saved == nil || st.saved.Canonical != "Иванов" {
		t.Fatalf("saved = %+v", st.saved)
	}
}

func TestCreateTitleRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{})

	_, err := sc.CreateTitle(context.Background(), models.Title{ID: "TT-01ARZ3NDEKTSV4RRFFQ69G5FA1", Canonical: "x"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestCreateTitleRejectsEmptyCanonical(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{id: "TT-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateTitle(context.Background(), models.Title{})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "canonical" {
		t.Fatalf("err = %v, want ValidationError on canonical", err)
	}
}

func TestCreateTitlePropagatesStoreError(t *testing.T) {
	wantErr := errors.New("db down")
	st := &fakeStore{saveErr: wantErr}
	sc := New(st, &stubIDs{id: "TT-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateTitle(context.Background(), models.Title{Canonical: "Иванов"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}
```

#### `internal/usecases/update_title/` (создать)
`internal/usecases/update_title/deps.go`:
```go
package update_title

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// TitleStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования и сохранение идут в одной транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type TitleStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
```

`internal/usecases/update_title/scenario.go`:
```go
package update_title

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение словарной записи фамилии».
type Scenario struct {
	store TitleStore
}

// New создаёт сценарий.
func New(st TitleStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateTitle полностью заменяет запись по sn.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, и сохраняет.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateTitle(ctx context.Context, sn models.Title) error {
	if err := sn.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetTitle(ctx, sn.ID); err != nil {
			return err
		}

		return tx.SaveTitle(ctx, &sn)
	})
}
```

`internal/usecases/update_title/scenario_test.go`:
```go
package update_title

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
	titles map[models.ID]*models.Title
	saved  []*models.Title
}

func newFakeTx(existing ...*models.Title) *fakeTx {
	tx := &fakeTx{titles: map[models.ID]*models.Title{}}
	for _, s := range existing {
		tx.titles[s.ID] = s
	}

	return tx
}

func (f *fakeTx) GetTitle(_ context.Context, id models.ID) (*models.Title, error) {
	s, ok := f.titles[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *s

	return &cp, nil
}

func (f *fakeTx) SaveTitle(_ context.Context, s *models.Title) error {
	cp := *s
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует TitleStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func title(id models.ID, canonical string) *models.Title {
	return &models.Title{ID: id, Canonical: canonical}
}

func TestUpdateTitleSaves(t *testing.T) {
	existing := title("TT-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Иванов")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Canonical = "Иванов (испр.)"

	if err := New(st).UpdateTitle(context.Background(), updated); err != nil {
		t.Fatalf("UpdateTitle: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Canonical != "Иванов (испр.)" {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateTitleNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateTitle(context.Background(), *title("TT-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Иванов"))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateTitleRejectsEmptyCanonical(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateTitle(context.Background(), models.Title{ID: "TT-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "canonical" {
		t.Fatalf("err = %v, want ValidationError on canonical", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}
```

#### `internal/usecases/delete_title/` (создать)
`internal/usecases/delete_title/deps.go`:
```go
package delete_title

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// TitleRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type TitleRepo interface {
	DeleteTitle(ctx context.Context, id models.ID) error
}
```

`internal/usecases/delete_title/scenario.go`:
```go
package delete_title

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление словарной записи фамилии».
type Scenario struct {
	titles TitleRepo
}

// New создаёт сценарий.
func New(titles TitleRepo) *Scenario {
	return &Scenario{titles: titles}
}

// DeleteTitle удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteTitle(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.titles.DeleteTitle(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeTitle)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeTitle,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

`internal/usecases/delete_title/scenario_test.go`:
```go
package delete_title

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

func (f *fakeRepo) DeleteTitle(_ context.Context, id models.ID) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)

	return nil
}

func TestDeleteTitleCallsRepo(t *testing.T) {
	repo := &fakeRepo{}
	id := models.ID("TT-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	if err := New(repo).DeleteTitle(context.Background(), id); err != nil {
		t.Fatalf("DeleteTitle: %v", err)
	}

	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("deleted = %v", repo.deleted)
	}
}

func TestDeleteTitleInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	err := New(repo).DeleteTitle(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}

	if len(repo.deleted) != 0 {
		t.Fatalf("repo вызван при невалидном id")
	}
}

func TestDeleteTitlePropagatesInUseError(t *testing.T) {
	wantErr := &models.InUseError{Type: models.TypeTitle, ID: "TT-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	repo := &fakeRepo{err: wantErr}

	err := New(repo).DeleteTitle(context.Background(), "TT-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	var iu *models.InUseError
	if !errors.As(err, &iu) {
		t.Fatalf("err = %v, want InUseError", err)
	}
}
```

#### `internal/httpapi/title.go` (создать)
`internal/httpapi/title.go`:
```go
package httpapi

import (
	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
	"net/http"
)

// handleTitleList — GET /api/titles?limit=&offset=. Чтение открыто
// анонимному посетителю (как и деления — auth.md §6, Access здесь не
// проверяется).
func handleTitleList(titles TitleService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := titles.ListTitles(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.TitlesFromModels(list))
	}
}

// handleTitleSearch — GET /api/titles/search?q=&limit=&offset=.
func handleTitleSearch(titles TitleService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := titles.SearchTitles(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.TitlesFromModels(list))
	}
}

// handleTitleGet — GET /api/titles/{id}.
func handleTitleGet(titles TitleService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sn, err := titles.GetTitle(r.Context(), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.TitleFromModel(sn))
	}
}
```

#### `internal/httpapi/title_write.go` (создать)
`internal/httpapi/title_write.go`:
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleTitleCreate — POST /api/titles: создаёт запись, отвечает 201 с
// созданной записью (id генерирует сценарий). Запись — только для вошедшего
// владельца (auth.md §6, решение 9), см. handleDivisionCreate.
func handleTitleCreate(titles TitleService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.TitleCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := titles.CreateTitle(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.TitleFromModel(created))
	}
}

// handleTitleUpdate — PUT /api/titles/{id}: полная замена
// canonical/variants/items/notes. Читает текущую версию, накладывает поля
// запроса (fetch-then-merge — как handleDivisionUpdate; у Title сейчас DTO
// покрывает всю модель, но конвенция общая для будущих сущностей, чей DTO
// v1 не покрывает всех полей модели, docs/data-model/entity-write.md §3).
func handleTitleUpdate(titles TitleService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.TitleUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := titles.GetTitle(r.Context(), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Canonical = m.Canonical
		cur.Variants = m.Variants
		cur.Items = m.Items
		cur.Notes = m.Notes

		if err := titles.UpdateTitle(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.TitleFromModel(cur))
	}
}

// handleTitleDelete — DELETE /api/titles/{id}: 204 без тела; занятая
// запись — 409 со списком ссылающихся. Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handleTitleDelete(titles TitleService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := titles.DeleteTitle(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
```

#### `internal/httpapi/title_test.go` (создать)
`internal/httpapi/title_test.go`:
```go
package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeTitles struct {
	list []models.Title
	err  error
	page models.Page

	getSN     models.Title
	gotIDs    []models.ID
	created   models.Title
	gotCreate models.Title
	updated   models.Title
	deleteErr error

	search    []models.Title
	gotSearch models.SearchQuery
}

func (f *fakeTitles) ListTitles(_ context.Context, _ models.Access, page models.Page) ([]models.Title, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeTitles) SearchTitles(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Title, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeTitles) GetTitle(_ context.Context, id models.ID) (models.Title, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Title{}, f.err
	}

	return f.getSN, nil
}

func (f *fakeTitles) CreateTitle(_ context.Context, sn models.Title) (models.Title, error) {
	f.gotCreate = sn
	if f.err != nil {
		return models.Title{}, f.err
	}

	return f.created, nil
}

func (f *fakeTitles) UpdateTitle(_ context.Context, sn models.Title) error {
	f.updated = sn

	return f.err
}

func (f *fakeTitles) DeleteTitle(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestTitleListReturnsRecords(t *testing.T) {
	svc := &fakeTitles{list: []models.Title{{ID: "TT-1", Canonical: "Иванов", Variants: []models.TextRef{{Text: "Иванова"}}}}}

	rec := get(t, NewHandler(Deps{Titles: svc, DocsFS: fstest.MapFS{}}), "/api/titles")
	requireStatus(t, rec, 200)

	want := `[{"id":"TT-1","canonical":"Иванов","variants":[{"text":"Иванова"}],"items":[],"notes":[]}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestTitleGetNotFound(t *testing.T) {
	svc := &fakeTitles{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Titles: svc, DocsFS: fstest.MapFS{}}), "/api/titles/TT-1")
	requireStatus(t, rec, 404)
}

func TestTitleSearchPassesQuery(t *testing.T) {
	svc := &fakeTitles{}

	rec := get(t, NewHandler(Deps{Titles: svc, DocsFS: fstest.MapFS{}}), "/api/titles/search?q=Ив")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "Ив" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
```

#### `internal/httpapi/title_write_test.go` (создать)
`internal/httpapi/title_write_test.go`:
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

func TestTitleCreateContract(t *testing.T) {
	svc := &fakeTitles{created: models.Title{ID: "TT-1", Canonical: "Иванов"}}

	rec := postD(t, NewHandler(Deps{Titles: svc, DocsFS: fstest.MapFS{}}), "/api/titles", `{"canonical":"Иванов"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Canonical != "Иванов" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

func TestTitleCreateAnonymousIs401(t *testing.T) {
	svc := &fakeTitles{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Titles: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/titles", strings.NewReader(`{"canonical":"Иванов"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestTitleUpdateMergesFields(t *testing.T) {
	svc := &fakeTitles{getSN: models.Title{ID: "TT-1", Canonical: "Иванов"}}

	rec := putD(t, NewHandler(Deps{Titles: svc, DocsFS: fstest.MapFS{}}), "/api/titles/TT-1",
		`{"canonical":"Иванов (испр.)","variants":[{"text":"Иванова"}]}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Canonical != "Иванов (испр.)" || svc.updated.ID != "TT-1" || len(svc.updated.Variants) != 1 {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestTitleDeleteNoContent(t *testing.T) {
	svc := &fakeTitles{}

	rec := delD(t, NewHandler(Deps{Titles: svc, DocsFS: fstest.MapFS{}}), "/api/titles/TT-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestTitleDeleteInUseIs409(t *testing.T) {
	svc := &fakeTitles{deleteErr: &models.InUseError{Type: models.TypeTitle, ID: "TT-1"}}

	rec := delD(t, NewHandler(Deps{Titles: svc, DocsFS: fstest.MapFS{}}), "/api/titles/TT-1")

	requireStatus(t, rec, http.StatusConflict)
}
```

#### `internal/mcp/title.go` (создать)
`internal/mcp/title.go`:
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

// registerTitleTools регистрирует тулы для работы со словарными записями
// фамилий. Запись variants/items/notes — только текстом (v1, как и веб-форма,
// docs/data-model/entity-write.md §4): элемент с уже существующей ссылкой
// (Ref/Type) через эти тулы не создать, только прочитать (list/get/search
// отдают полный TextRef, включая Ref/Type). ВАЖНО: title_update заменяет
// списки целиком текстом — существующие ref/type будут потеряны при любом
// обновлении через MCP, пока не появится picker (см. предупреждение в
// описании тула).
func registerTitleTools(s *server.MCPServer, titles TitleService) {
	tool := mcp.NewTool(
		"title_list",
		mcp.WithDescription("Список словарных записей фамилий в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, titleListHandler(titles))

	tool = mcp.NewTool(
		"title_search",
		mcp.WithDescription("Поиск словарных записей фамилий по началу канонической формы (включая варианты); результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало канонической формы или варианта")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, titleSearchHandler(titles))

	tool = mcp.NewTool(
		"title_get",
		mcp.WithDescription("Словарная запись фамилии по id; результат — JSON записи. Неверный формат id или отсутствующая запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например TT-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, titleGetHandler(titles))

	tool = mcp.NewTool(
		"title_create",
		mcp.WithDescription("Создать словарную запись фамилии; id генерируется сервером; результат — JSON созданной записи. variants/items/notes — списки текста (без ссылок на другие сущности, v1)"),
		mcp.WithString("canonical", mcp.Required(), mcp.Description("Каноническая форма")),
		mcp.WithArray("variants", mcp.WithStringItems(), mcp.Description("Варианты написания")),
		mcp.WithArray("items", mcp.WithStringItems(), mcp.Description("Носители/употребления — кто использует эту форму (текстом)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
	)
	s.AddTool(tool, titleCreateHandler(titles))

	tool = mcp.NewTool(
		"title_update",
		mcp.WithDescription("Изменить словарную запись фамилии: полная замена canonical/variants/items/notes; результат — JSON обновлённой записи. variants/items/notes передаются целиком как текст — если у элемента раньше была ссылка на другую сущность (ref/type из title_get), она будет потеряна: picker для ссылок ещё не реализован ни в вебе, ни в MCP (docs/data-model/entity-write.md §5)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("canonical", mcp.Required(), mcp.Description("Новая каноническая форма")),
		mcp.WithArray("variants", mcp.WithStringItems(), mcp.Description("Варианты написания")),
		mcp.WithArray("items", mcp.WithStringItems(), mcp.Description("Носители/употребления — кто использует эту форму (текстом)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
	)
	s.AddTool(tool, titleUpdateHandler(titles))

	tool = mcp.NewTool(
		"title_delete",
		mcp.WithDescription("Удалить словарную запись фамилии. Необратимо. Если на неё есть строгие ссылки от других сущностей — ошибка тула (для Title такое сегодня не создаётся, но общий механизм проверки один для всех сущностей)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, titleDeleteHandler(titles))
}

func titleListHandler(titles TitleService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := titles.ListTitles(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.TitlesFromModels(list))
	}
}

func titleSearchHandler(titles TitleService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := titles.SearchTitles(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.TitlesFromModels(list))
	}
}

func titleGetHandler(titles TitleService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sn, err := titles.GetTitle(ctx, models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.TitleFromModel(sn))
	}
}

func titleCreateHandler(titles TitleService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sn := models.Title{
			Canonical: req.GetString("canonical", ""),
			Variants:  textRefsFromStrings(req.GetStringSlice("variants", nil)),
			Items:     textRefsFromStrings(req.GetStringSlice("items", nil)),
			Notes:     textRefsFromStrings(req.GetStringSlice("notes", nil)),
		}

		created, err := titles.CreateTitle(ctx, sn)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.TitleFromModel(created))
	}
}

func titleUpdateHandler(titles TitleService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := titles.GetTitle(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		cur.Canonical = req.GetString("canonical", "")
		cur.Variants = textRefsFromStrings(req.GetStringSlice("variants", nil))
		cur.Items = textRefsFromStrings(req.GetStringSlice("items", nil))
		cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))

		if err := titles.UpdateTitle(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.TitleFromModel(cur))
	}
}

func titleDeleteHandler(titles TitleService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := titles.DeleteTitle(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
```

#### `internal/mcp/title_test.go` (создать)
`internal/mcp/title_test.go`:
```go
package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

// fakeTitles запоминает запрос и отдаёт заданный ответ.
type fakeTitles struct {
	list []models.Title
	err  error

	getSN     models.Title
	created   models.Title
	gotCreate models.Title
	updated   models.Title
	gotIDs    []models.ID
	deleteErr error

	search []models.Title
}

func (f *fakeTitles) ListTitles(context.Context, models.Access, models.Page) ([]models.Title, error) {
	return f.list, f.err
}

func (f *fakeTitles) SearchTitles(context.Context, models.Access, models.SearchQuery) ([]models.Title, error) {
	return f.search, f.err
}

func (f *fakeTitles) GetTitle(_ context.Context, id models.ID) (models.Title, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Title{}, f.err
	}

	return f.getSN, nil
}

func (f *fakeTitles) CreateTitle(_ context.Context, sn models.Title) (models.Title, error) {
	f.gotCreate = sn
	if f.err != nil {
		return models.Title{}, f.err
	}

	return f.created, nil
}

func (f *fakeTitles) UpdateTitle(_ context.Context, sn models.Title) error {
	f.updated = sn

	return f.err
}

func (f *fakeTitles) DeleteTitle(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callTitleTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestTitleGetToolContract(t *testing.T) {
	svc := &fakeTitles{getSN: models.Title{ID: "TT-1", Canonical: "Иванов"}}

	res := callTitleTool(t, titleGetHandler(svc), map[string]any{"id": "TT-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "TT-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

func TestTitleCreateToolPassesVariants(t *testing.T) {
	svc := &fakeTitles{created: models.Title{ID: "TT-new", Canonical: "Иванов"}}

	res := callTitleTool(t, titleCreateHandler(svc), map[string]any{
		"canonical": "Иванов",
		"variants":  []any{"Иванова"},
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.Canonical != "Иванов" || len(svc.gotCreate.Variants) != 1 || svc.gotCreate.Variants[0].Text != "Иванова" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

func TestTitleDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeTitles{deleteErr: &models.InUseError{Type: models.TypeTitle, ID: "TT-1"}}

	res := callTitleTool(t, titleDeleteHandler(svc), map[string]any{"id": "TT-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersTitleTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Titles: &fakeTitles{}}).ListTools()

	for _, name := range []string{"title_list", "title_search", "title_get", "title_create", "title_update", "title_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
```

### Шаг 1.4. GivenName — транспорт, usecases, httpapi, MCP (с полем `Gender`)

Форма как у Задачи 1.1-1.3, плюс обязательное `Gender`. В `transport.GivenNameCreate`/`GivenNameUpdate`/`GivenName` поле `Gender` **обязательно присутствует** (см. предупреждение в разделе «Предпосылка» — его отсутствие было реальным багом при первой генерации: без него HTTP-контракт молча теряет пол при create/update, а сериализация `GET`-ответа не отдаёт `gender` вовсе). `givenNameUpdateHandler` (MCP) обязан проставлять `cur.Gender` из запроса при обновлении — иначе пол теряется при любом MCP-редактировании (fetch-then-merge, см. `docs/data-model/entity-write.md` §3).

#### `internal/transport/given_name.go` (создать)
`internal/transport/given_name.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// GivenName — контракт словарной записи имени (GET /api/given-names, MCP-тул given_name_list).
type GivenName struct {
	ID        models.ID         `json:"id"`
	Canonical string            `json:"canonical"`
	Gender    models.NameGender `json:"gender"`
	Variants  []TextRef         `json:"variants"`
	Items     []TextRef         `json:"items"`
	Notes     []TextRef         `json:"notes"`
}

// GivenNameFromModel конвертирует запись в контракт.
func GivenNameFromModel(s models.GivenName) GivenName {
	return GivenName{
		ID:        s.ID,
		Canonical: s.Canonical,
		Gender:    s.Gender,
		Variants:  TextRefsFromModel(s.Variants),
		Items:     TextRefsFromModel(s.Items),
		Notes:     TextRefsFromModel(s.Notes),
	}
}

// GivenNamesFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func GivenNamesFromModels(ss []models.GivenName) []GivenName {
	out := make([]GivenName, 0, len(ss))
	for _, s := range ss {
		out = append(out, GivenNameFromModel(s))
	}

	return out
}
```

#### `internal/transport/given_name_write.go` (создать)
`internal/transport/given_name_write.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// GivenNameCreate — тело POST /api/given-names и аргументы тула given_name_create.
// Идентификатор генерирует сценарий.
type GivenNameCreate struct {
	Canonical string            `json:"canonical"`
	Gender    models.NameGender `json:"gender"`
	Variants  []TextRef         `json:"variants"`
	Items     []TextRef         `json:"items"`
	Notes     []TextRef         `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (s GivenNameCreate) Model() models.GivenName {
	return models.GivenName{
		Canonical: s.Canonical,
		Gender:    s.Gender,
		Variants:  TextRefsToModel(s.Variants),
		Items:     TextRefsToModel(s.Items),
		Notes:     TextRefsToModel(s.Notes),
	}
}

// GivenNameUpdate — тело PUT /api/given-names/{id} и аргументы тула given_name_update:
// полная замена canonical/variants/items/notes.
type GivenNameUpdate struct {
	Canonical string            `json:"canonical"`
	Gender    models.NameGender `json:"gender"`
	Variants  []TextRef         `json:"variants"`
	Items     []TextRef         `json:"items"`
	Notes     []TextRef         `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (s GivenNameUpdate) Model() models.GivenName {
	return models.GivenName{
		Canonical: s.Canonical,
		Gender:    s.Gender,
		Variants:  TextRefsToModel(s.Variants),
		Items:     TextRefsToModel(s.Items),
		Notes:     TextRefsToModel(s.Notes),
	}
}
```

#### `internal/usecases/list_given_names/` (создать)
`internal/usecases/list_given_names/deps.go`:
```go
package list_given_names

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// GivenNameRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type GivenNameRepo interface {
	ListGivenNames(ctx context.Context, access models.Access, page models.Page) ([]*models.GivenName, error)
}
```

`internal/usecases/list_given_names/scenario.go`:
```go
package list_given_names

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список словарных записей фамилий».
type Scenario struct {
	givenNames GivenNameRepo
}

// New создаёт сценарий.
func New(givenNames GivenNameRepo) *Scenario {
	return &Scenario{givenNames: givenNames}
}

// ListGivenNames возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListGivenNames(ctx context.Context, access models.Access, page models.Page) ([]models.GivenName, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeGivenName, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeGivenName, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.givenNames.ListGivenNames(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.GivenName, 0, len(list))
	for _, sn := range list {
		out = append(out, *sn)
	}

	return out, nil
}
```

`internal/usecases/list_given_names/scenario_test.go`:
```go
package list_given_names

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.GivenName
}

func (f *fakeRepo) ListGivenNames(_ context.Context, _ models.Access, page models.Page) ([]*models.GivenName, error) {
	f.page = page

	return f.out, nil
}

func TestListGivenNamesReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.GivenName{{ID: "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1", Canonical: "Иванов"}}}

	got, err := New(repo).ListGivenNames(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListGivenNames: %v", err)
	}

	if len(got) != 1 || got[0].Canonical != "Иванов" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListGivenNamesRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListGivenNames(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
```

#### `internal/usecases/search_given_names/` (создать)
`internal/usecases/search_given_names/deps.go`:
```go
package search_given_names

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// GivenNameRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type GivenNameRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetGivenName(ctx context.Context, id models.ID) (*models.GivenName, error)
}
```

`internal/usecases/search_given_names/scenario.go`:
```go
package search_given_names

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск словарных записей фамилий».
type Scenario struct {
	givenNames GivenNameRepo
}

// New создаёт сценарий.
func New(givenNames GivenNameRepo) *Scenario {
	return &Scenario{givenNames: givenNames}
}

// SearchGivenNames находит записи, чья каноническая форма (или вариант)
// начинается с текста запроса — та же механика, что и
// search_divisions.SearchDivisions (см. её комментарий).
func (s *Scenario) SearchGivenNames(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.GivenName, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.GivenName{}, nil
	}

	page := q.Page.Normalized()
	out := []models.GivenName{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.givenNames.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeGivenName {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.givenNames.GetGivenName(ctx, h.ID)
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

`internal/usecases/search_given_names/scenario_test.go`:
```go
package search_given_names

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits       []models.Hit
	givenNames map[models.ID]*models.GivenName
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetGivenName(_ context.Context, id models.ID) (*models.GivenName, error) {
	s, ok := f.givenNames[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestSearchGivenNamesFiltersByType(t *testing.T) {
	id := models.ID("GN-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeGivenName, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не givenName — должен быть пропущен
		},
		givenNames: map[models.ID]*models.GivenName{id: {ID: id, Canonical: "Иванов"}},
	}

	got, err := New(repo).SearchGivenNames(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchGivenNames: %v", err)
	}

	if len(got) != 1 || got[0].Canonical != "Иванов" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchGivenNamesEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchGivenNames(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchGivenNames: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
```

#### `internal/usecases/get_given_name/` (создать)
`internal/usecases/get_given_name/deps.go`:
```go
package get_given_name

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// GivenNameRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type GivenNameRepo interface {
	GetGivenName(ctx context.Context, id models.ID) (*models.GivenName, error)
}
```

`internal/usecases/get_given_name/scenario.go`:
```go
package get_given_name

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «словарная запись фамилии по идентификатору».
type Scenario struct {
	givenNames GivenNameRepo
}

// New создаёт сценарий.
func New(givenNames GivenNameRepo) *Scenario {
	return &Scenario{givenNames: givenNames}
}

// GetGivenName возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound.
func (s *Scenario) GetGivenName(ctx context.Context, id models.ID) (models.GivenName, error) {
	if err := validateID(id); err != nil {
		return models.GivenName{}, err
	}

	sn, err := s.givenNames.GetGivenName(ctx, id)
	if err != nil {
		return models.GivenName{}, err
	}

	return *sn, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeGivenName)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeGivenName,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

`internal/usecases/get_given_name/scenario_test.go`:
```go
package get_given_name

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	givenNames map[models.ID]*models.GivenName
}

func (f *fakeRepo) GetGivenName(_ context.Context, id models.ID) (*models.GivenName, error) {
	s, ok := f.givenNames[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestGetGivenNameReturnsRecord(t *testing.T) {
	id := models.ID("GN-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{givenNames: map[models.ID]*models.GivenName{id: {ID: id, Canonical: "Иванов"}}}

	got, err := New(repo).GetGivenName(context.Background(), id)
	if err != nil {
		t.Fatalf("GetGivenName: %v", err)
	}

	if got.Canonical != "Иванов" {
		t.Fatalf("Canonical = %q", got.Canonical)
	}
}

func TestGetGivenNameNotFound(t *testing.T) {
	repo := &fakeRepo{givenNames: map[models.ID]*models.GivenName{}}

	_, err := New(repo).GetGivenName(context.Background(), "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetGivenNameInvalidID(t *testing.T) {
	repo := &fakeRepo{givenNames: map[models.ID]*models.GivenName{}}

	_, err := New(repo).GetGivenName(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}
```

#### `internal/usecases/create_given_name/` (создать)
`internal/usecases/create_given_name/deps.go`:
```go
package create_given_name

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// GivenNameStore — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type GivenNameStore interface {
	SaveGivenName(ctx context.Context, s *models.GivenName) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
```

`internal/usecases/create_given_name/scenario.go`:
```go
package create_given_name

import (
	"context"
	"fmt"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «создание словарной записи фамилии».
type Scenario struct {
	store GivenNameStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st GivenNameStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateGivenName создаёт запись: генерирует идентификатор, проверяет
// инварианты, сохраняет. Возвращает созданную запись с заполненным ID.
// Флат-сущность без FK — в отличие от create_division, без транзакционной
// проверки родителя.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта) и невалидная сущность — *models.ValidationError; прочее —
// ошибки хранилища как есть.
func (s *Scenario) CreateGivenName(ctx context.Context, sn models.GivenName) (models.GivenName, error) {
	if sn.ID != "" {
		return models.GivenName{}, &models.ValidationError{
			Entity: models.TypeGivenName,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", sn.ID),
		}
	}

	sn.ID = s.ids.New(models.TypeGivenName)

	if err := sn.Validate(); err != nil {
		return models.GivenName{}, err
	}

	if err := s.store.SaveGivenName(ctx, &sn); err != nil {
		return models.GivenName{}, err
	}

	return sn, nil
}
```

`internal/usecases/create_given_name/scenario_test.go`:
```go
package create_given_name

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type stubIDs struct{ id models.ID }

func (s *stubIDs) New(models.Type) models.ID { return s.id }

type fakeStore struct {
	saved   *models.GivenName
	saveErr error
}

func (f *fakeStore) SaveGivenName(_ context.Context, s *models.GivenName) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *s
	f.saved = &cp

	return nil
}

func TestCreateGivenNameGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{}
	sc := New(st, ids)

	got, err := sc.CreateGivenName(context.Background(), models.GivenName{Canonical: "Иванов", Gender: models.NameGenderMale})
	if err != nil {
		t.Fatalf("CreateGivenName: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.saved == nil || st.saved.Canonical != "Иванов" {
		t.Fatalf("saved = %+v", st.saved)
	}
}

func TestCreateGivenNameRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{})

	_, err := sc.CreateGivenName(context.Background(), models.GivenName{ID: "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1", Canonical: "x"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestCreateGivenNameRejectsEmptyCanonical(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{id: "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateGivenName(context.Background(), models.GivenName{Gender: models.NameGenderMale})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "canonical" {
		t.Fatalf("err = %v, want ValidationError on canonical", err)
	}
}

// TestCreateGivenNameRejectsInvalidGender — единственное отличие GivenName от
// остальных словарей (Surname/Patronymic/Estate/Title): gender обязателен и
// должен быть одним из male/female/neutral (internal/models/dictionary_validate.go).
func TestCreateGivenNameRejectsInvalidGender(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{id: "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateGivenName(context.Background(), models.GivenName{Canonical: "Иван", Gender: "unknown"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "gender" {
		t.Fatalf("err = %v, want ValidationError on gender", err)
	}
}

func TestCreateGivenNamePropagatesStoreError(t *testing.T) {
	wantErr := errors.New("db down")
	st := &fakeStore{saveErr: wantErr}
	sc := New(st, &stubIDs{id: "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateGivenName(context.Background(), models.GivenName{Canonical: "Иванов", Gender: models.NameGenderMale})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}
```

#### `internal/usecases/update_given_name/` (создать)
`internal/usecases/update_given_name/deps.go`:
```go
package update_given_name

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// GivenNameStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования и сохранение идут в одной транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type GivenNameStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
```

`internal/usecases/update_given_name/scenario.go`:
```go
package update_given_name

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение словарной записи фамилии».
type Scenario struct {
	store GivenNameStore
}

// New создаёт сценарий.
func New(st GivenNameStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateGivenName полностью заменяет запись по sn.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, и сохраняет.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateGivenName(ctx context.Context, sn models.GivenName) error {
	if err := sn.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetGivenName(ctx, sn.ID); err != nil {
			return err
		}

		return tx.SaveGivenName(ctx, &sn)
	})
}
```

`internal/usecases/update_given_name/scenario_test.go`:
```go
package update_given_name

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
	givenNames map[models.ID]*models.GivenName
	saved      []*models.GivenName
}

func newFakeTx(existing ...*models.GivenName) *fakeTx {
	tx := &fakeTx{givenNames: map[models.ID]*models.GivenName{}}
	for _, s := range existing {
		tx.givenNames[s.ID] = s
	}

	return tx
}

func (f *fakeTx) GetGivenName(_ context.Context, id models.ID) (*models.GivenName, error) {
	s, ok := f.givenNames[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *s

	return &cp, nil
}

func (f *fakeTx) SaveGivenName(_ context.Context, s *models.GivenName) error {
	cp := *s
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует GivenNameStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func givenName(id models.ID, canonical string) *models.GivenName {
	return &models.GivenName{ID: id, Canonical: canonical, Gender: models.NameGenderMale}
}

func TestUpdateGivenNameSaves(t *testing.T) {
	existing := givenName("GN-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Иванов")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Canonical = "Иванов (испр.)"

	if err := New(st).UpdateGivenName(context.Background(), updated); err != nil {
		t.Fatalf("UpdateGivenName: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Canonical != "Иванов (испр.)" {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateGivenNameNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateGivenName(context.Background(), *givenName("GN-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Иванов"))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateGivenNameRejectsEmptyCanonical(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateGivenName(context.Background(), models.GivenName{ID: "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1", Gender: models.NameGenderMale})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "canonical" {
		t.Fatalf("err = %v, want ValidationError on canonical", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

// TestUpdateGivenNameRejectsInvalidGender — единственное отличие GivenName от
// остальных словарей, см. create_given_name's аналогичный тест.
func TestUpdateGivenNameRejectsInvalidGender(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateGivenName(context.Background(), models.GivenName{ID: "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1", Canonical: "Иван"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "gender" {
		t.Fatalf("err = %v, want ValidationError on gender", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}
```

#### `internal/usecases/delete_given_name/` (создать)
`internal/usecases/delete_given_name/deps.go`:
```go
package delete_given_name

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// GivenNameRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type GivenNameRepo interface {
	DeleteGivenName(ctx context.Context, id models.ID) error
}
```

`internal/usecases/delete_given_name/scenario.go`:
```go
package delete_given_name

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление словарной записи фамилии».
type Scenario struct {
	givenNames GivenNameRepo
}

// New создаёт сценарий.
func New(givenNames GivenNameRepo) *Scenario {
	return &Scenario{givenNames: givenNames}
}

// DeleteGivenName удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteGivenName(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.givenNames.DeleteGivenName(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeGivenName)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeGivenName,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

`internal/usecases/delete_given_name/scenario_test.go`:
```go
package delete_given_name

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

func (f *fakeRepo) DeleteGivenName(_ context.Context, id models.ID) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)

	return nil
}

func TestDeleteGivenNameCallsRepo(t *testing.T) {
	repo := &fakeRepo{}
	id := models.ID("GN-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	if err := New(repo).DeleteGivenName(context.Background(), id); err != nil {
		t.Fatalf("DeleteGivenName: %v", err)
	}

	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("deleted = %v", repo.deleted)
	}
}

func TestDeleteGivenNameInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	err := New(repo).DeleteGivenName(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}

	if len(repo.deleted) != 0 {
		t.Fatalf("repo вызван при невалидном id")
	}
}

func TestDeleteGivenNamePropagatesInUseError(t *testing.T) {
	wantErr := &models.InUseError{Type: models.TypeGivenName, ID: "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	repo := &fakeRepo{err: wantErr}

	err := New(repo).DeleteGivenName(context.Background(), "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	var iu *models.InUseError
	if !errors.As(err, &iu) {
		t.Fatalf("err = %v, want InUseError", err)
	}
}
```

#### `internal/httpapi/given_name.go` (создать)
`internal/httpapi/given_name.go`:
```go
package httpapi

import (
	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
	"net/http"
)

// handleGivenNameList — GET /api/given-names?limit=&offset=. Чтение открыто
// анонимному посетителю (как и деления — auth.md §6, Access здесь не
// проверяется).
func handleGivenNameList(givenNames GivenNameService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := givenNames.ListGivenNames(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.GivenNamesFromModels(list))
	}
}

// handleGivenNameSearch — GET /api/given-names/search?q=&limit=&offset=.
func handleGivenNameSearch(givenNames GivenNameService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := givenNames.SearchGivenNames(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.GivenNamesFromModels(list))
	}
}

// handleGivenNameGet — GET /api/given-names/{id}.
func handleGivenNameGet(givenNames GivenNameService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sn, err := givenNames.GetGivenName(r.Context(), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.GivenNameFromModel(sn))
	}
}
```

#### `internal/httpapi/given_name_write.go` (создать)
`internal/httpapi/given_name_write.go`:
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleGivenNameCreate — POST /api/given-names: создаёт запись, отвечает 201 с
// созданной записью (id генерирует сценарий). Запись — только для вошедшего
// владельца (auth.md §6, решение 9), см. handleDivisionCreate.
func handleGivenNameCreate(givenNames GivenNameService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.GivenNameCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := givenNames.CreateGivenName(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.GivenNameFromModel(created))
	}
}

// handleGivenNameUpdate — PUT /api/given-names/{id}: полная замена
// canonical/variants/items/notes. Читает текущую версию, накладывает поля
// запроса (fetch-then-merge — как handleDivisionUpdate; у GivenName сейчас DTO
// покрывает всю модель, но конвенция общая для будущих сущностей, чей DTO
// v1 не покрывает всех полей модели, docs/data-model/entity-write.md §3).
func handleGivenNameUpdate(givenNames GivenNameService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.GivenNameUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := givenNames.GetGivenName(r.Context(), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Canonical = m.Canonical
		cur.Gender = m.Gender
		cur.Variants = m.Variants
		cur.Items = m.Items
		cur.Notes = m.Notes

		if err := givenNames.UpdateGivenName(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.GivenNameFromModel(cur))
	}
}

// handleGivenNameDelete — DELETE /api/given-names/{id}: 204 без тела; занятая
// запись — 409 со списком ссылающихся. Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handleGivenNameDelete(givenNames GivenNameService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := givenNames.DeleteGivenName(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
```

#### `internal/httpapi/given_name_test.go` (создать)
`internal/httpapi/given_name_test.go`:
```go
package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeGivenNames struct {
	list []models.GivenName
	err  error
	page models.Page

	getSN     models.GivenName
	gotIDs    []models.ID
	created   models.GivenName
	gotCreate models.GivenName
	updated   models.GivenName
	deleteErr error

	search    []models.GivenName
	gotSearch models.SearchQuery
}

func (f *fakeGivenNames) ListGivenNames(_ context.Context, _ models.Access, page models.Page) ([]models.GivenName, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeGivenNames) SearchGivenNames(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.GivenName, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeGivenNames) GetGivenName(_ context.Context, id models.ID) (models.GivenName, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.GivenName{}, f.err
	}

	return f.getSN, nil
}

func (f *fakeGivenNames) CreateGivenName(_ context.Context, sn models.GivenName) (models.GivenName, error) {
	f.gotCreate = sn
	if f.err != nil {
		return models.GivenName{}, f.err
	}

	return f.created, nil
}

func (f *fakeGivenNames) UpdateGivenName(_ context.Context, sn models.GivenName) error {
	f.updated = sn

	return f.err
}

func (f *fakeGivenNames) DeleteGivenName(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestGivenNameListReturnsRecords(t *testing.T) {
	svc := &fakeGivenNames{list: []models.GivenName{{ID: "GN-1", Canonical: "Иванов", Variants: []models.TextRef{{Text: "Иванова"}}}}}

	rec := get(t, NewHandler(Deps{GivenNames: svc, DocsFS: fstest.MapFS{}}), "/api/given-names")
	requireStatus(t, rec, 200)

	want := `[{"id":"GN-1","canonical":"Иванов","gender":"","variants":[{"text":"Иванова"}],"items":[],"notes":[]}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestGivenNameGetNotFound(t *testing.T) {
	svc := &fakeGivenNames{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{GivenNames: svc, DocsFS: fstest.MapFS{}}), "/api/given-names/GN-1")
	requireStatus(t, rec, 404)
}

func TestGivenNameSearchPassesQuery(t *testing.T) {
	svc := &fakeGivenNames{}

	rec := get(t, NewHandler(Deps{GivenNames: svc, DocsFS: fstest.MapFS{}}), "/api/given-names/search?q=Ив")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "Ив" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
```

#### `internal/httpapi/given_name_write_test.go` (создать)
`internal/httpapi/given_name_write_test.go`:
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

func TestGivenNameCreateContract(t *testing.T) {
	svc := &fakeGivenNames{created: models.GivenName{ID: "GN-1", Canonical: "Иван", Gender: models.NameGenderMale}}

	rec := postD(t, NewHandler(Deps{GivenNames: svc, DocsFS: fstest.MapFS{}}), "/api/given-names", `{"canonical":"Иван","gender":"male"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Canonical != "Иван" || svc.gotCreate.Gender != models.NameGenderMale || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
	if !strings.Contains(rec.Body.String(), `"gender":"male"`) {
		t.Fatalf("body = %s, gender не в ответе", rec.Body)
	}
}

func TestGivenNameCreateAnonymousIs401(t *testing.T) {
	svc := &fakeGivenNames{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{GivenNames: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/given-names", strings.NewReader(`{"canonical":"Иванов"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestGivenNameUpdateMergesFields(t *testing.T) {
	svc := &fakeGivenNames{getSN: models.GivenName{ID: "GN-1", Canonical: "Иван", Gender: models.NameGenderMale}}

	rec := putD(t, NewHandler(Deps{GivenNames: svc, DocsFS: fstest.MapFS{}}), "/api/given-names/GN-1",
		`{"canonical":"Иван (испр.)","gender":"neutral","variants":[{"text":"Ваня"}]}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Canonical != "Иван (испр.)" || svc.updated.Gender != models.NameGenderNeutral ||
		svc.updated.ID != "GN-1" || len(svc.updated.Variants) != 1 {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestGivenNameDeleteNoContent(t *testing.T) {
	svc := &fakeGivenNames{}

	rec := delD(t, NewHandler(Deps{GivenNames: svc, DocsFS: fstest.MapFS{}}), "/api/given-names/GN-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestGivenNameDeleteInUseIs409(t *testing.T) {
	svc := &fakeGivenNames{deleteErr: &models.InUseError{Type: models.TypeGivenName, ID: "GN-1"}}

	rec := delD(t, NewHandler(Deps{GivenNames: svc, DocsFS: fstest.MapFS{}}), "/api/given-names/GN-1")

	requireStatus(t, rec, http.StatusConflict)
}
```

#### `internal/mcp/given_name.go` (создать)
`internal/mcp/given_name.go`:
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

// registerGivenNameTools регистрирует тулы для работы со словарными записями
// имён. Запись variants/items/notes — только текстом (v1, как и веб-форма,
// docs/data-model/entity-write.md §4): элемент с уже существующей ссылкой
// (Ref/Type) через эти тулы не создать, только прочитать (list/get/search
// отдают полный TextRef, включая Ref/Type). ВАЖНО: given_name_update заменяет
// списки целиком текстом — существующие ref/type будут потеряны при любом
// обновлении через MCP, пока не появится picker (см. предупреждение в
// описании тула). gender обязателен (models.GivenName.Validate()).
func registerGivenNameTools(s *server.MCPServer, givenNames GivenNameService) {
	tool := mcp.NewTool(
		"given_name_list",
		mcp.WithDescription("Список словарных записей имён в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, givenNameListHandler(givenNames))

	tool = mcp.NewTool(
		"given_name_search",
		mcp.WithDescription("Поиск словарных записей имён по началу канонической формы (включая варианты); результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало канонической формы или варианта")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, givenNameSearchHandler(givenNames))

	tool = mcp.NewTool(
		"given_name_get",
		mcp.WithDescription("Словарная запись имени по id; результат — JSON записи. Неверный формат id или отсутствующая запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например GN-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, givenNameGetHandler(givenNames))

	tool = mcp.NewTool(
		"given_name_create",
		mcp.WithDescription("Создать словарную запись имени; id генерируется сервером; результат — JSON созданной записи. variants/items/notes — списки текста (без ссылок на другие сущности, v1)"),
		mcp.WithString("canonical", mcp.Required(), mcp.Description("Каноническая форма")),
		mcp.WithString("gender", mcp.Required(), mcp.Enum("male", "female", "neutral"),
			mcp.Description("Пол имени — обязателен (male/female/neutral; neutral — Женя, Саша: вывод пола по такому имени запрещён)")),
		mcp.WithArray("variants", mcp.WithStringItems(), mcp.Description("Варианты написания")),
		mcp.WithArray("items", mcp.WithStringItems(), mcp.Description("Носители/употребления — кто использует эту форму (текстом)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
	)
	s.AddTool(tool, givenNameCreateHandler(givenNames))

	tool = mcp.NewTool(
		"given_name_update",
		mcp.WithDescription("Изменить словарную запись имени: полная замена canonical/gender/variants/items/notes; результат — JSON обновлённой записи. variants/items/notes передаются целиком как текст — если у элемента раньше была ссылка на другую сущность (ref/type из given_name_get), она будет потеряна: picker для ссылок ещё не реализован ни в вебе, ни в MCP (docs/data-model/entity-write.md §5)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("canonical", mcp.Required(), mcp.Description("Новая каноническая форма")),
		mcp.WithString("gender", mcp.Required(), mcp.Enum("male", "female", "neutral"),
			mcp.Description("Пол имени — обязателен (male/female/neutral)")),
		mcp.WithArray("variants", mcp.WithStringItems(), mcp.Description("Варианты написания")),
		mcp.WithArray("items", mcp.WithStringItems(), mcp.Description("Носители/употребления — кто использует эту форму (текстом)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
	)
	s.AddTool(tool, givenNameUpdateHandler(givenNames))

	tool = mcp.NewTool(
		"given_name_delete",
		mcp.WithDescription("Удалить словарную запись имени. Необратимо. Если на неё есть строгие ссылки от других сущностей — ошибка тула (для GivenName такое сегодня не создаётся, но общий механизм проверки один для всех сущностей)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, givenNameDeleteHandler(givenNames))
}

func givenNameListHandler(givenNames GivenNameService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := givenNames.ListGivenNames(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.GivenNamesFromModels(list))
	}
}

func givenNameSearchHandler(givenNames GivenNameService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := givenNames.SearchGivenNames(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.GivenNamesFromModels(list))
	}
}

func givenNameGetHandler(givenNames GivenNameService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sn, err := givenNames.GetGivenName(ctx, models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.GivenNameFromModel(sn))
	}
}

func givenNameCreateHandler(givenNames GivenNameService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sn := models.GivenName{
			Canonical: req.GetString("canonical", ""),
			Gender:    models.NameGender(req.GetString("gender", "")),
			Variants:  textRefsFromStrings(req.GetStringSlice("variants", nil)),
			Items:     textRefsFromStrings(req.GetStringSlice("items", nil)),
			Notes:     textRefsFromStrings(req.GetStringSlice("notes", nil)),
		}

		created, err := givenNames.CreateGivenName(ctx, sn)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.GivenNameFromModel(created))
	}
}

func givenNameUpdateHandler(givenNames GivenNameService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := givenNames.GetGivenName(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		cur.Canonical = req.GetString("canonical", "")
		cur.Gender = models.NameGender(req.GetString("gender", ""))
		cur.Variants = textRefsFromStrings(req.GetStringSlice("variants", nil))
		cur.Items = textRefsFromStrings(req.GetStringSlice("items", nil))
		cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))

		if err := givenNames.UpdateGivenName(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.GivenNameFromModel(cur))
	}
}

func givenNameDeleteHandler(givenNames GivenNameService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := givenNames.DeleteGivenName(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
```

#### `internal/mcp/given_name_test.go` (создать)
`internal/mcp/given_name_test.go`:
```go
package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

// fakeGivenNames запоминает запрос и отдаёт заданный ответ.
type fakeGivenNames struct {
	list []models.GivenName
	err  error

	getSN     models.GivenName
	created   models.GivenName
	gotCreate models.GivenName
	updated   models.GivenName
	gotIDs    []models.ID
	deleteErr error

	search []models.GivenName
}

func (f *fakeGivenNames) ListGivenNames(context.Context, models.Access, models.Page) ([]models.GivenName, error) {
	return f.list, f.err
}

func (f *fakeGivenNames) SearchGivenNames(context.Context, models.Access, models.SearchQuery) ([]models.GivenName, error) {
	return f.search, f.err
}

func (f *fakeGivenNames) GetGivenName(_ context.Context, id models.ID) (models.GivenName, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.GivenName{}, f.err
	}

	return f.getSN, nil
}

func (f *fakeGivenNames) CreateGivenName(_ context.Context, sn models.GivenName) (models.GivenName, error) {
	f.gotCreate = sn
	if f.err != nil {
		return models.GivenName{}, f.err
	}

	return f.created, nil
}

func (f *fakeGivenNames) UpdateGivenName(_ context.Context, sn models.GivenName) error {
	f.updated = sn

	return f.err
}

func (f *fakeGivenNames) DeleteGivenName(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callGivenNameTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestGivenNameGetToolContract(t *testing.T) {
	svc := &fakeGivenNames{getSN: models.GivenName{ID: "GN-1", Canonical: "Иванов"}}

	res := callGivenNameTool(t, givenNameGetHandler(svc), map[string]any{"id": "GN-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "GN-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

func TestGivenNameCreateToolPassesVariants(t *testing.T) {
	svc := &fakeGivenNames{created: models.GivenName{ID: "GN-new", Canonical: "Иванов", Gender: models.NameGenderMale}}

	res := callGivenNameTool(t, givenNameCreateHandler(svc), map[string]any{
		"canonical": "Иванов",
		"gender":    "male",
		"variants":  []any{"Иванова"},
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.Canonical != "Иванов" || svc.gotCreate.Gender != models.NameGenderMale ||
		len(svc.gotCreate.Variants) != 1 || svc.gotCreate.Variants[0].Text != "Иванова" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

func TestGivenNameUpdateToolSetsGender(t *testing.T) {
	svc := &fakeGivenNames{getSN: models.GivenName{ID: "GN-1", Canonical: "Иванов", Gender: models.NameGenderMale}}

	res := callGivenNameTool(t, givenNameUpdateHandler(svc), map[string]any{
		"id":        "GN-1",
		"canonical": "Иванов",
		"gender":    "neutral",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Gender != models.NameGenderNeutral {
		t.Fatalf("updated = %+v, want gender=neutral", svc.updated)
	}
}

func TestGivenNameDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeGivenNames{deleteErr: &models.InUseError{Type: models.TypeGivenName, ID: "GN-1"}}

	res := callGivenNameTool(t, givenNameDeleteHandler(svc), map[string]any{"id": "GN-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersGivenNameTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, GivenNames: &fakeGivenNames{}}).ListTools()

	for _, name := range []string{"given_name_list", "given_name_search", "given_name_get", "given_name_create", "given_name_update", "given_name_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
```

### Шаг 1.5. `Deps`-реестр: подключить все 4 сервиса

Все 4 сервиса регистрируются одинаково — по образцу `Surnames` из подпроекта 1 (гвард `if deps.X != nil { registerXRoutes/Tools(...) }`). Ниже — итоговое содержимое каждого изменённого файла целиком (переносить как есть, заменяя текущее содержимое файла).

#### `internal/httpapi/deps.go` (изменить — итоговое содержимое)
`internal/httpapi/deps.go`:
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
`internal/httpapi/api.go`:
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
	Divisions   DivisionService
	Surnames    SurnameService
	Patronymics PatronymicService
	Estates     EstateService
	Titles      TitleService
	GivenNames  GivenNameService
	Auth        AuthService
	DocsFS      fs.FS
	TrustProxy  bool
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

	registerAuthRoutes(mux, deps.Auth, deps.TrustProxy)

	return requireCSRFHeader(resolveAccess(deps.Auth)(mux))
}
```

#### `internal/httpapi/httpapi.go` (изменить — итоговое содержимое)
`internal/httpapi/httpapi.go`:
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
`internal/mcp/deps.go`:
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
```

#### `internal/mcp/server.go` (изменить — итоговое содержимое)
`internal/mcp/server.go`:
```go
package mcp

import (
	"github.com/mark3labs/mcp-go/server"
)

// Deps — сервисы, отдаваемые в MCP-тулы. Явный реестр вместо растущего
// списка позиционных параметров (docs/data-model/entity-write.md §3) — новая
// сущность добавляется полем структуры, не меняя сигнатуру NewServer.
type Deps struct {
	Divisions   DivisionService
	Surnames    SurnameService
	Patronymics PatronymicService
	Estates     EstateService
	Titles      TitleService
	GivenNames  GivenNameService
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

	return s
}
```

#### `internal/app/app.go` (изменить — итоговое содержимое)
`internal/app/app.go`:
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
	create_estate "github.com/amarin/genodex/internal/usecases/create_estate"
	create_given_name "github.com/amarin/genodex/internal/usecases/create_given_name"
	create_patronymic "github.com/amarin/genodex/internal/usecases/create_patronymic"
	create_surname "github.com/amarin/genodex/internal/usecases/create_surname"
	create_title "github.com/amarin/genodex/internal/usecases/create_title"
	delete_division "github.com/amarin/genodex/internal/usecases/delete_division"
	delete_estate "github.com/amarin/genodex/internal/usecases/delete_estate"
	delete_given_name "github.com/amarin/genodex/internal/usecases/delete_given_name"
	delete_patronymic "github.com/amarin/genodex/internal/usecases/delete_patronymic"
	delete_surname "github.com/amarin/genodex/internal/usecases/delete_surname"
	delete_title "github.com/amarin/genodex/internal/usecases/delete_title"
	get_division "github.com/amarin/genodex/internal/usecases/get_division"
	get_estate "github.com/amarin/genodex/internal/usecases/get_estate"
	get_given_name "github.com/amarin/genodex/internal/usecases/get_given_name"
	get_patronymic "github.com/amarin/genodex/internal/usecases/get_patronymic"
	get_surname "github.com/amarin/genodex/internal/usecases/get_surname"
	get_title "github.com/amarin/genodex/internal/usecases/get_title"
	list_divisions "github.com/amarin/genodex/internal/usecases/list_divisions"
	list_estates "github.com/amarin/genodex/internal/usecases/list_estates"
	list_given_names "github.com/amarin/genodex/internal/usecases/list_given_names"
	list_patronymics "github.com/amarin/genodex/internal/usecases/list_patronymics"
	list_surnames "github.com/amarin/genodex/internal/usecases/list_surnames"
	list_titles "github.com/amarin/genodex/internal/usecases/list_titles"
	search_divisions "github.com/amarin/genodex/internal/usecases/search_divisions"
	search_estates "github.com/amarin/genodex/internal/usecases/search_estates"
	search_given_names "github.com/amarin/genodex/internal/usecases/search_given_names"
	search_patronymics "github.com/amarin/genodex/internal/usecases/search_patronymics"
	search_surnames "github.com/amarin/genodex/internal/usecases/search_surnames"
	search_titles "github.com/amarin/genodex/internal/usecases/search_titles"
	update_division "github.com/amarin/genodex/internal/usecases/update_division"
	update_estate "github.com/amarin/genodex/internal/usecases/update_estate"
	update_given_name "github.com/amarin/genodex/internal/usecases/update_given_name"
	update_patronymic "github.com/amarin/genodex/internal/usecases/update_patronymic"
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

// given_nameService — фасад всех сценариев словарных записей (имён), отдаваемых
// HTTP и MCP. Тот же приём, что divisionService/surnameService — по одному
// полю на сценарий, тонкие методы-делегаты (docs/data-model/entity-write.md §3).
type given_nameService struct {
	list   *list_given_names.Scenario
	search *search_given_names.Scenario
	get    *get_given_name.Scenario
	create *create_given_name.Scenario
	update *update_given_name.Scenario
	del    *delete_given_name.Scenario
}

func (s *given_nameService) ListGivenNames(ctx context.Context, access models.Access, page models.Page) ([]models.GivenName, error) {
	return s.list.ListGivenNames(ctx, access, page)
}

func (s *given_nameService) SearchGivenNames(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.GivenName, error) {
	return s.search.SearchGivenNames(ctx, access, q)
}

func (s *given_nameService) GetGivenName(ctx context.Context, id models.ID) (models.GivenName, error) {
	return s.get.GetGivenName(ctx, id)
}

func (s *given_nameService) CreateGivenName(ctx context.Context, x models.GivenName) (models.GivenName, error) {
	return s.create.CreateGivenName(ctx, x)
}

func (s *given_nameService) UpdateGivenName(ctx context.Context, x models.GivenName) error {
	return s.update.UpdateGivenName(ctx, x)
}

func (s *given_nameService) DeleteGivenName(ctx context.Context, id models.ID) error {
	return s.del.DeleteGivenName(ctx, id)
}

var (
	_ httpapi.DivisionService   = (*divisionService)(nil)
	_ mcp.DivisionService       = (*divisionService)(nil)
	_ httpapi.SurnameService    = (*surnameService)(nil)
	_ mcp.SurnameService        = (*surnameService)(nil)
	_ httpapi.PatronymicService = (*patronymicService)(nil)
	_ mcp.PatronymicService     = (*patronymicService)(nil)
	_ httpapi.EstateService     = (*estateService)(nil)
	_ mcp.EstateService         = (*estateService)(nil)
	_ httpapi.TitleService      = (*titleService)(nil)
	_ mcp.TitleService          = (*titleService)(nil)
	_ httpapi.GivenNameService  = (*given_nameService)(nil)
	_ mcp.GivenNameService      = (*given_nameService)(nil)
	_ httpapi.AuthService       = (*auth.Service)(nil)
	_ mcp.TokenResolver         = (*auth.Service)(nil)
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

	given_names := &given_nameService{
		list:   list_given_names.New(st),
		search: search_given_names.New(st),
		get:    get_given_name.New(st),
		create: create_given_name.New(st, idgen.New()),
		update: update_given_name.New(st),
		del:    delete_given_name.New(st),
	}

	// auth-хранилище — на том же соединении, что и общий store (см.
	// sqlstore.Store.DB), файл БД один и тот же (internal/storage/schema_auth.go).
	authService := auth.New(auth.NewSQLStore(st.DB()))

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcp.RequireAPIToken(authService)(server.NewStreamableHTTPServer(
		mcp.NewServer(mcp.Deps{
			Divisions: divisions, Surnames: surnames,
			Patronymics: patronymics, Estates: estates, Titles: titles, GivenNames: given_names,
		}),
	)))
	mux.Handle("/api/", httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:   divisions,
		Surnames:    surnames,
		Patronymics: patronymics,
		Estates:     estates,
		Titles:      titles,
		GivenNames:  given_names,
		Auth:        authService,
		DocsFS:      genodex.DocsFS(cfg.WebMode),
		TrustProxy:  cfg.TrustProxy,
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

### Шаг 1.6. Рубеж

`gofmt -l .` пусто; `go build ./...`, `go vet ./...`, `go test ./...` (весь репозиторий) зелёные — ожидается 763 теста, 52 пакета. Живой смок через `curl` (см. «Предпосылка») — не обязателен повторно, но приветствуется как самопроверка: создать по одной записи каждого из 4 видов, для `given-names` — обязательно с `gender`, проверить, что `gender` присутствует в ответе `POST` и в `GET`-списке.

### Шаг 1.7. Коммит

```bash
git add internal/transport/patronymic.go internal/transport/patronymic_write.go internal/transport/estate.go internal/transport/estate_write.go internal/transport/title.go internal/transport/title_write.go internal/transport/given_name.go internal/transport/given_name_write.go internal/usecases/list_patronymics internal/usecases/search_patronymics internal/usecases/get_patronymic internal/usecases/create_patronymic internal/usecases/update_patronymic internal/usecases/delete_patronymic internal/usecases/list_estates internal/usecases/search_estates internal/usecases/get_estate internal/usecases/create_estate internal/usecases/update_estate internal/usecases/delete_estate internal/usecases/list_titles internal/usecases/search_titles internal/usecases/get_title internal/usecases/create_title internal/usecases/update_title internal/usecases/delete_title internal/usecases/list_given_names internal/usecases/search_given_names internal/usecases/get_given_name internal/usecases/create_given_name internal/usecases/update_given_name internal/usecases/delete_given_name internal/httpapi/patronymic.go internal/httpapi/patronymic_write.go internal/httpapi/patronymic_test.go internal/httpapi/patronymic_write_test.go internal/httpapi/estate.go internal/httpapi/estate_write.go internal/httpapi/estate_test.go internal/httpapi/estate_write_test.go internal/httpapi/title.go internal/httpapi/title_write.go internal/httpapi/title_test.go internal/httpapi/title_write_test.go internal/httpapi/given_name.go internal/httpapi/given_name_write.go internal/httpapi/given_name_test.go internal/httpapi/given_name_write_test.go internal/mcp/patronymic.go internal/mcp/patronymic_test.go internal/mcp/estate.go internal/mcp/estate_test.go internal/mcp/title.go internal/mcp/title_test.go internal/mcp/given_name.go internal/mcp/given_name_test.go internal/httpapi/deps.go internal/httpapi/api.go internal/httpapi/httpapi.go internal/mcp/deps.go internal/mcp/server.go internal/app/app.go
git commit -m "feat(backend): Patronymic/Estate/Title/GivenName — полный CRUD (usecases/httpapi/mcp) + Deps-реестр"
```
## Задача 2. Веб-страницы Patronymic/Estate/Title

**Интерфейсы, потребляемые из Задачи 1**: `httpapi`-маршруты `/api/patronymics`/`/api/estates`/`/api/titles` (JSON-форма — см. `transport.{Patronymic,Estate,Title}`/`{...}Create`/`{...}Update` из Задачи 1).
**Интерфейсы, потребляемые из подпроекта 1**: `authFetch`/`ApiError` (`web/src/auth.ts`), `TextRefListEditor` (`web/src/TextRefList.tsx`), `PageLayout`/каталог сущностей (`web/src/pages/EntityCatalog.tsx`, `web/src/App.tsx`), `MAX_PAGE_LIMIT` (`web/src/api.ts`).

**Файлы:**
- Создать: `web/src/pages/{PatronymicsList,PatronymicForm,PatronymicView}.tsx`, `web/src/pages/{EstatesList,EstateForm,EstateView}.tsx`, `web/src/pages/{TitlesList,TitleForm,TitleView}.tsx`
- Изменить: `web/src/api.ts`, `web/src/App.tsx`, `web/src/pages/EntityCatalog.tsx`

### Шаг 2.1. `web/src/api.ts` — типы и функции Patronymic/Estate/Title

Добавить в самый конец файла (после существующего блока `fetchDoc`); порядок внутри файла не важен, важно не разрывать существующие блоки:

```typescript
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
```

### Шаг 2.2. Patronymic — страницы `List`/`Form`/`View`

#### `web/src/pages/PatronymicsList.tsx` (создать)
`web/src/pages/PatronymicsList.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Input, List, Spin } from "antd";
import { fetchPatronymics, searchPatronymics, MAX_PAGE_LIMIT, type Patronymic } from "../api";
import { useSession } from "../session";
import { CreatePatronymicModal } from "./PatronymicForm";

// PatronymicsList — «Отчества»: плоский список (сущность не иерархична, в
// отличие от AdminDivision — без Tree), пагинация по offset до короткой
// страницы (тот же приём, что DivisionsList.loadRoot — API уже отдаёт
// окнами, docs/data-model/entity-write.md §4). Поиск по названию временно
// подменяет список найденным. Клик по строке — переход на View.
export default function PatronymicsList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<Patronymic[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<Patronymic[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: Patronymic[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchPatronymics({ limit: MAX_PAGE_LIMIT, offset });
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
    searchPatronymics({ q, limit: MAX_PAGE_LIMIT })
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
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "Отчества" }]}
      />
      <Card
        title="Отчества"
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
            renderItem={(s) => (
              <List.Item>
                <Link to={`/patronymics/${s.id}`}>{s.canonical}</Link>
              </List.Item>
            )}
          />
        )}
        <CreatePatronymicModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(s) => {
            setCreateOpen(false);
            navigate(`/patronymics/${s.id}`);
          }}
        />
      </Card>
    </>
  );
}
```

#### `web/src/pages/PatronymicForm.tsx` (создать)
`web/src/pages/PatronymicForm.tsx`:
```tsx
import { useState } from "react";
import { Alert, Form, Input, Modal } from "antd";
import { createPatronymic, type Patronymic, type TextRef } from "../api";
import { ApiError } from "../auth";
import { TextRefListEditor } from "../TextRefList";

interface PatronymicFormValues {
  canonical: string;
}

const FORM_FIELDS: (keyof PatronymicFormValues)[] = ["canonical"];

// CreatePatronymicModal — форма создания словарной записи фамилии. variants/
// items/notes редактируются вне antd Form (TextRefListEditor — не обычный
// текстовый инпут), собираются в одно тело запроса на submit.
export function CreatePatronymicModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Patronymic) => void;
}) {
  const [form] = Form.useForm<PatronymicFormValues>();
  const [variants, setVariants] = useState<TextRef[]>([]);
  const [items, setItems] = useState<TextRef[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setVariants([]);
    setItems([]);
    setNotes([]);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: PatronymicFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await createPatronymic({ canonical: values.canonical, variants, items, notes });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof PatronymicFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить отчество"
      open={open}
      onCancel={handleClose}
      onOk={() => form.submit()}
      okText="Создать"
      cancelText="Отмена"
      confirmLoading={submitting}
      destroyOnHidden
    >
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
      <Form form={form} layout="vertical" onFinish={onFinish}>
        <Form.Item
          name="canonical"
          label="Каноническая форма"
          rules={[{ required: true, whitespace: true, message: "Введите каноническую форму" }]}
        >
          <Input autoFocus />
        </Form.Item>
        <Form.Item label="Варианты написания">
          <TextRefListEditor value={variants} onChange={setVariants} addLabel="+ вариант" />
        </Form.Item>
        <Form.Item label="Носители">
          <TextRefListEditor value={items} onChange={setItems} addLabel="+ запись" />
        </Form.Item>
        <Form.Item label="Заметки">
          <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
        </Form.Item>
      </Form>
    </Modal>
  );
}
```

#### `web/src/pages/PatronymicView.tsx` (создать)
`web/src/pages/PatronymicView.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  Alert,
  Breadcrumb,
  Button,
  Card,
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
import { deletePatronymic, fetchPatronymic, updatePatronymic, type Patronymic, type TextRef } from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { TextRefListEditor } from "../TextRefList";

interface EditFormValues {
  canonical: string;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["canonical"];

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

// PatronymicView — просмотр словарной записи фамилии, переключаемый в форму
// редактирования на той же странице (тот же toggle+explicit-save, что
// DivisionView — PUT заменяет запись целиком, docs/data-model/entity-write.md
// §4). Без родителя/детей/дерева — Patronymic не иерархична, в отличие от
// AdminDivision.
export default function PatronymicView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [patronymic, setPatronymic] = useState<Patronymic | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [variants, setVariants] = useState<TextRef[]>([]);
  const [items, setItems] = useState<TextRef[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (patronymicId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setPatronymic(null);
    fetchPatronymic(patronymicId)
      .then(setPatronymic)
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
    if (patronymic == null) {
      return;
    }
    form.setFieldsValue({ canonical: patronymic.canonical });
    setVariants(patronymic.variants);
    setItems(patronymic.items);
    setNotes(patronymic.notes);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (patronymic == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await updatePatronymic(patronymic.id, {
        canonical: values.canonical,
        variants,
        items,
        notes,
      });
      setPatronymic(updated);
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
    if (patronymic == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deletePatronymic(patronymic.id);
      navigate("/patronymics");
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
          <Link to="/patronymics">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (patronymic == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/patronymics">Отчества</Link> },
          { title: patronymic.canonical },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={patronymic.canonical} column={1} bordered size="small">
            <Descriptions.Item label="Варианты"><TextRefListView items={patronymic.variants} /></Descriptions.Item>
            <Descriptions.Item label="Носители"><TextRefListView items={patronymic.items} /></Descriptions.Item>
            <Descriptions.Item label="Заметки"><TextRefListView items={patronymic.notes} /></Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title={`Удалить «${patronymic.canonical}»?`}
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
            name="canonical"
            label="Каноническая форма"
            rules={[{ required: true, whitespace: true, message: "Введите каноническую форму" }]}
          >
            <Input />
          </Form.Item>
          <Form.Item label="Варианты написания">
            <TextRefListEditor value={variants} onChange={setVariants} addLabel="+ вариант" />
          </Form.Item>
          <Form.Item label="Носители">
            <TextRefListEditor value={items} onChange={setItems} addLabel="+ запись" />
          </Form.Item>
          <Form.Item label="Заметки">
            <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
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

### Шаг 2.3. Estate — страницы `List`/`Form`/`View`

#### `web/src/pages/EstatesList.tsx` (создать)
`web/src/pages/EstatesList.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Input, List, Spin } from "antd";
import { fetchEstates, searchEstates, MAX_PAGE_LIMIT, type Estate } from "../api";
import { useSession } from "../session";
import { CreateEstateModal } from "./EstateForm";

// EstatesList — «Сословия»: плоский список (сущность не иерархична, в
// отличие от AdminDivision — без Tree), пагинация по offset до короткой
// страницы (тот же приём, что DivisionsList.loadRoot — API уже отдаёт
// окнами, docs/data-model/entity-write.md §4). Поиск по названию временно
// подменяет список найденным. Клик по строке — переход на View.
export default function EstatesList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<Estate[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<Estate[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: Estate[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchEstates({ limit: MAX_PAGE_LIMIT, offset });
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
    searchEstates({ q, limit: MAX_PAGE_LIMIT })
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
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "Сословия" }]}
      />
      <Card
        title="Сословия"
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
            renderItem={(s) => (
              <List.Item>
                <Link to={`/estates/${s.id}`}>{s.canonical}</Link>
              </List.Item>
            )}
          />
        )}
        <CreateEstateModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(s) => {
            setCreateOpen(false);
            navigate(`/estates/${s.id}`);
          }}
        />
      </Card>
    </>
  );
}
```

#### `web/src/pages/EstateForm.tsx` (создать)
`web/src/pages/EstateForm.tsx`:
```tsx
import { useState } from "react";
import { Alert, Form, Input, Modal } from "antd";
import { createEstate, type Estate, type TextRef } from "../api";
import { ApiError } from "../auth";
import { TextRefListEditor } from "../TextRefList";

interface EstateFormValues {
  canonical: string;
}

const FORM_FIELDS: (keyof EstateFormValues)[] = ["canonical"];

// CreateEstateModal — форма создания словарной записи фамилии. variants/
// items/notes редактируются вне antd Form (TextRefListEditor — не обычный
// текстовый инпут), собираются в одно тело запроса на submit.
export function CreateEstateModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Estate) => void;
}) {
  const [form] = Form.useForm<EstateFormValues>();
  const [variants, setVariants] = useState<TextRef[]>([]);
  const [items, setItems] = useState<TextRef[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setVariants([]);
    setItems([]);
    setNotes([]);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: EstateFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await createEstate({ canonical: values.canonical, variants, items, notes });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof EstateFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить сословие"
      open={open}
      onCancel={handleClose}
      onOk={() => form.submit()}
      okText="Создать"
      cancelText="Отмена"
      confirmLoading={submitting}
      destroyOnHidden
    >
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
      <Form form={form} layout="vertical" onFinish={onFinish}>
        <Form.Item
          name="canonical"
          label="Каноническая форма"
          rules={[{ required: true, whitespace: true, message: "Введите каноническую форму" }]}
        >
          <Input autoFocus />
        </Form.Item>
        <Form.Item label="Варианты написания">
          <TextRefListEditor value={variants} onChange={setVariants} addLabel="+ вариант" />
        </Form.Item>
        <Form.Item label="Носители">
          <TextRefListEditor value={items} onChange={setItems} addLabel="+ запись" />
        </Form.Item>
        <Form.Item label="Заметки">
          <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
        </Form.Item>
      </Form>
    </Modal>
  );
}
```

#### `web/src/pages/EstateView.tsx` (создать)
`web/src/pages/EstateView.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  Alert,
  Breadcrumb,
  Button,
  Card,
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
import { deleteEstate, fetchEstate, updateEstate, type Estate, type TextRef } from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { TextRefListEditor } from "../TextRefList";

interface EditFormValues {
  canonical: string;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["canonical"];

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

// EstateView — просмотр словарной записи фамилии, переключаемый в форму
// редактирования на той же странице (тот же toggle+explicit-save, что
// DivisionView — PUT заменяет запись целиком, docs/data-model/entity-write.md
// §4). Без родителя/детей/дерева — Estate не иерархична, в отличие от
// AdminDivision.
export default function EstateView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [estate, setEstate] = useState<Estate | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [variants, setVariants] = useState<TextRef[]>([]);
  const [items, setItems] = useState<TextRef[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (estateId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setEstate(null);
    fetchEstate(estateId)
      .then(setEstate)
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
    if (estate == null) {
      return;
    }
    form.setFieldsValue({ canonical: estate.canonical });
    setVariants(estate.variants);
    setItems(estate.items);
    setNotes(estate.notes);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (estate == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await updateEstate(estate.id, {
        canonical: values.canonical,
        variants,
        items,
        notes,
      });
      setEstate(updated);
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
    if (estate == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteEstate(estate.id);
      navigate("/estates");
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
          <Link to="/estates">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (estate == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/estates">Сословия</Link> },
          { title: estate.canonical },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={estate.canonical} column={1} bordered size="small">
            <Descriptions.Item label="Варианты"><TextRefListView items={estate.variants} /></Descriptions.Item>
            <Descriptions.Item label="Носители"><TextRefListView items={estate.items} /></Descriptions.Item>
            <Descriptions.Item label="Заметки"><TextRefListView items={estate.notes} /></Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title={`Удалить «${estate.canonical}»?`}
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
            name="canonical"
            label="Каноническая форма"
            rules={[{ required: true, whitespace: true, message: "Введите каноническую форму" }]}
          >
            <Input />
          </Form.Item>
          <Form.Item label="Варианты написания">
            <TextRefListEditor value={variants} onChange={setVariants} addLabel="+ вариант" />
          </Form.Item>
          <Form.Item label="Носители">
            <TextRefListEditor value={items} onChange={setItems} addLabel="+ запись" />
          </Form.Item>
          <Form.Item label="Заметки">
            <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
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

### Шаг 2.4. Title — страницы `List`/`Form`/`View`

#### `web/src/pages/TitlesList.tsx` (создать)
`web/src/pages/TitlesList.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Input, List, Spin } from "antd";
import { fetchTitles, searchTitles, MAX_PAGE_LIMIT, type Title } from "../api";
import { useSession } from "../session";
import { CreateTitleModal } from "./TitleForm";

// TitlesList — «Титулы»: плоский список (сущность не иерархична, в
// отличие от AdminDivision — без Tree), пагинация по offset до короткой
// страницы (тот же приём, что DivisionsList.loadRoot — API уже отдаёт
// окнами, docs/data-model/entity-write.md §4). Поиск по названию временно
// подменяет список найденным. Клик по строке — переход на View.
export default function TitlesList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<Title[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<Title[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: Title[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchTitles({ limit: MAX_PAGE_LIMIT, offset });
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
    searchTitles({ q, limit: MAX_PAGE_LIMIT })
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
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "Титулы" }]}
      />
      <Card
        title="Титулы"
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
            renderItem={(s) => (
              <List.Item>
                <Link to={`/titles/${s.id}`}>{s.canonical}</Link>
              </List.Item>
            )}
          />
        )}
        <CreateTitleModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(s) => {
            setCreateOpen(false);
            navigate(`/titles/${s.id}`);
          }}
        />
      </Card>
    </>
  );
}
```

#### `web/src/pages/TitleForm.tsx` (создать)
`web/src/pages/TitleForm.tsx`:
```tsx
import { useState } from "react";
import { Alert, Form, Input, Modal } from "antd";
import { createTitle, type Title, type TextRef } from "../api";
import { ApiError } from "../auth";
import { TextRefListEditor } from "../TextRefList";

interface TitleFormValues {
  canonical: string;
}

const FORM_FIELDS: (keyof TitleFormValues)[] = ["canonical"];

// CreateTitleModal — форма создания словарной записи фамилии. variants/
// items/notes редактируются вне antd Form (TextRefListEditor — не обычный
// текстовый инпут), собираются в одно тело запроса на submit.
export function CreateTitleModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Title) => void;
}) {
  const [form] = Form.useForm<TitleFormValues>();
  const [variants, setVariants] = useState<TextRef[]>([]);
  const [items, setItems] = useState<TextRef[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setVariants([]);
    setItems([]);
    setNotes([]);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: TitleFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await createTitle({ canonical: values.canonical, variants, items, notes });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof TitleFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить титул"
      open={open}
      onCancel={handleClose}
      onOk={() => form.submit()}
      okText="Создать"
      cancelText="Отмена"
      confirmLoading={submitting}
      destroyOnHidden
    >
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
      <Form form={form} layout="vertical" onFinish={onFinish}>
        <Form.Item
          name="canonical"
          label="Каноническая форма"
          rules={[{ required: true, whitespace: true, message: "Введите каноническую форму" }]}
        >
          <Input autoFocus />
        </Form.Item>
        <Form.Item label="Варианты написания">
          <TextRefListEditor value={variants} onChange={setVariants} addLabel="+ вариант" />
        </Form.Item>
        <Form.Item label="Носители">
          <TextRefListEditor value={items} onChange={setItems} addLabel="+ запись" />
        </Form.Item>
        <Form.Item label="Заметки">
          <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
        </Form.Item>
      </Form>
    </Modal>
  );
}
```

#### `web/src/pages/TitleView.tsx` (создать)
`web/src/pages/TitleView.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  Alert,
  Breadcrumb,
  Button,
  Card,
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
import { deleteTitle, fetchTitle, updateTitle, type Title, type TextRef } from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { TextRefListEditor } from "../TextRefList";

interface EditFormValues {
  canonical: string;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["canonical"];

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

// TitleView — просмотр словарной записи фамилии, переключаемый в форму
// редактирования на той же странице (тот же toggle+explicit-save, что
// DivisionView — PUT заменяет запись целиком, docs/data-model/entity-write.md
// §4). Без родителя/детей/дерева — Title не иерархична, в отличие от
// AdminDivision.
export default function TitleView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [title, setTitle] = useState<Title | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [variants, setVariants] = useState<TextRef[]>([]);
  const [items, setItems] = useState<TextRef[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (titleId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setTitle(null);
    fetchTitle(titleId)
      .then(setTitle)
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
    if (title == null) {
      return;
    }
    form.setFieldsValue({ canonical: title.canonical });
    setVariants(title.variants);
    setItems(title.items);
    setNotes(title.notes);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (title == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await updateTitle(title.id, {
        canonical: values.canonical,
        variants,
        items,
        notes,
      });
      setTitle(updated);
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
    if (title == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteTitle(title.id);
      navigate("/titles");
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
          <Link to="/titles">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (title == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/titles">Титулы</Link> },
          { title: title.canonical },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={title.canonical} column={1} bordered size="small">
            <Descriptions.Item label="Варианты"><TextRefListView items={title.variants} /></Descriptions.Item>
            <Descriptions.Item label="Носители"><TextRefListView items={title.items} /></Descriptions.Item>
            <Descriptions.Item label="Заметки"><TextRefListView items={title.notes} /></Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title={`Удалить «${title.canonical}»?`}
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
            name="canonical"
            label="Каноническая форма"
            rules={[{ required: true, whitespace: true, message: "Введите каноническую форму" }]}
          >
            <Input />
          </Form.Item>
          <Form.Item label="Варианты написания">
            <TextRefListEditor value={variants} onChange={setVariants} addLabel="+ вариант" />
          </Form.Item>
          <Form.Item label="Носители">
            <TextRefListEditor value={items} onChange={setItems} addLabel="+ запись" />
          </Form.Item>
          <Form.Item label="Заметки">
            <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
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

### Шаг 2.5. `web/src/App.tsx` — роуты Patronymic/Estate/Title

Добавить импорты (после импорта `SurnameView`):
```typescript
import PatronymicsList from "./pages/PatronymicsList";
import PatronymicView from "./pages/PatronymicView";
import EstatesList from "./pages/EstatesList";
import EstateView from "./pages/EstateView";
import TitlesList from "./pages/TitlesList";
import TitleView from "./pages/TitleView";
```
Добавить роуты (после роутов `/surnames`/`/surnames/:id`, до `/login`):
```typescript
          <Route path="/patronymics" element={<PageLayout><PatronymicsList /></PageLayout>} />
          <Route path="/patronymics/:id" element={<PageLayout><PatronymicView /></PageLayout>} />
          <Route path="/estates" element={<PageLayout><EstatesList /></PageLayout>} />
          <Route path="/estates/:id" element={<PageLayout><EstateView /></PageLayout>} />
          <Route path="/titles" element={<PageLayout><TitlesList /></PageLayout>} />
          <Route path="/titles/:id" element={<PageLayout><TitleView /></PageLayout>} />
```

### Шаг 2.6. `web/src/pages/EntityCatalog.tsx` — три новые строки каталога

В массиве `CATALOG_ENTRIES` добавить (порядок внутри массива не важен — список сортируется по алфавиту в рантайме через `localeCompare('ru')`):
```typescript
  { label: "Отчества", path: "/patronymics" },
  { label: "Сословия", path: "/estates" },
  { label: "Титулы", path: "/titles" },
```

### Шаг 2.7. Рубеж

`npm run typecheck` и `npm run build` в `web/` чисты. Живая проверка в браузере (см. «Предпосылка»): `/patronymics`, `/estates`, `/titles` — список, создание, просмотр, редактирование, удаление работают, хлебные крошки корректны, каталог на `/` показывает три новые строки в алфавитном порядке.

### Шаг 2.8. Коммит

```bash
git add web/src/api.ts web/src/App.tsx web/src/pages/EntityCatalog.tsx web/src/pages/PatronymicsList.tsx web/src/pages/PatronymicForm.tsx web/src/pages/PatronymicView.tsx web/src/pages/EstatesList.tsx web/src/pages/EstateForm.tsx web/src/pages/EstateView.tsx web/src/pages/TitlesList.tsx web/src/pages/TitleForm.tsx web/src/pages/TitleView.tsx
git commit -m "feat(web): страницы Отчеств/Сословий/Титулов — список, просмотр/редактирование, форма"
```
## Задача 3. Веб-страницы GivenName (с полем «Пол»)

**Интерфейсы, потребляемые из Задачи 1**: `httpapi`-маршрут `/api/given-names` (JSON-форма — `transport.GivenName`/`GivenNameCreate`/`GivenNameUpdate`, включая обязательное поле `gender`).
**Интерфейсы, потребляемые из подпроекта 1**: то же, что Задача 2 — `authFetch`/`ApiError`, `TextRefListEditor`, `PageLayout`, каталог, `MAX_PAGE_LIMIT`.

**Файлы:**
- Создать: `web/src/pages/{GivenNamesList,GivenNameForm,GivenNameView}.tsx`
- Изменить: `web/src/api.ts`, `web/src/App.tsx`, `web/src/pages/EntityCatalog.tsx`

**Важно для исполнителя**: единственное отличие от Задачи 2 — обязательное поле `gender` (`male`/`female`/`neutral`). `GivenNameForm.tsx` и `GivenNameView.tsx` добавляют `Select`-контрол «Пол» (три варианта, с пояснением для `neutral`: «вывод пола запрещён» — имена вроде Женя/Саша, где по форме нельзя определить пол) рядом с «Каноническая форма», обязательный (`rules={{ required: true }}`), и передают `gender` в `createGivenName`/`updateGivenName`. `GivenNameView.tsx` показывает выбранный пол текстом (`Мужской`/`Женский`/`Нейтральный`) в `Descriptions` при просмотре.

### Шаг 3.1. `web/src/api.ts` — типы и функции GivenName

Добавить в самый конец файла (после блока Title из Задачи 2):
```typescript
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
```

### Шаг 3.2. Страницы `List`/`Form`/`View`

#### `web/src/pages/GivenNamesList.tsx` (создать)
`web/src/pages/GivenNamesList.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Input, List, Spin } from "antd";
import { fetchGivenNames, searchGivenNames, MAX_PAGE_LIMIT, type GivenName } from "../api";
import { useSession } from "../session";
import { CreateGivenNameModal } from "./GivenNameForm";

// GivenNamesList — «Имена»: плоский список (сущность не иерархична, в
// отличие от AdminDivision — без Tree), пагинация по offset до короткой
// страницы (тот же приём, что DivisionsList.loadRoot — API уже отдаёт
// окнами, docs/data-model/entity-write.md §4). Поиск по названию временно
// подменяет список найденным. Клик по строке — переход на View.
export default function GivenNamesList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<GivenName[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<GivenName[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: GivenName[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchGivenNames({ limit: MAX_PAGE_LIMIT, offset });
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
    searchGivenNames({ q, limit: MAX_PAGE_LIMIT })
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
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "Имена" }]}
      />
      <Card
        title="Имена"
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
            renderItem={(s) => (
              <List.Item>
                <Link to={`/given-names/${s.id}`}>{s.canonical}</Link>
              </List.Item>
            )}
          />
        )}
        <CreateGivenNameModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(s) => {
            setCreateOpen(false);
            navigate(`/given-names/${s.id}`);
          }}
        />
      </Card>
    </>
  );
}
```

#### `web/src/pages/GivenNameForm.tsx` (создать)
`web/src/pages/GivenNameForm.tsx`:
```tsx
import { useState } from "react";
import { Alert, Form, Input, Modal, Select } from "antd";
import { createGivenName, type GivenName, type NameGender, type TextRef } from "../api";
import { ApiError } from "../auth";
import { TextRefListEditor } from "../TextRefList";

interface GivenNameFormValues {
  canonical: string;
  gender: NameGender;
}

const FORM_FIELDS: (keyof GivenNameFormValues)[] = ["canonical", "gender"];

const GENDER_OPTIONS: { value: NameGender; label: string }[] = [
  { value: "male", label: "Мужской" },
  { value: "female", label: "Женский" },
  { value: "neutral", label: "Нейтральный (вывод пола запрещён)" },
];

// CreateGivenNameModal — форма создания словарной записи имени. gender —
// единственное отличие GivenName от остальных словарей (обязателен,
// docs/data-model/entity-write.md, internal/models/given_name.go). variants/
// items/notes редактируются вне antd Form (TextRefListEditor — не обычный
// текстовый инпут), собираются в одно тело запроса на submit.
export function CreateGivenNameModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: GivenName) => void;
}) {
  const [form] = Form.useForm<GivenNameFormValues>();
  const [variants, setVariants] = useState<TextRef[]>([]);
  const [items, setItems] = useState<TextRef[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setVariants([]);
    setItems([]);
    setNotes([]);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: GivenNameFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await createGivenName({
        canonical: values.canonical,
        gender: values.gender,
        variants,
        items,
        notes,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof GivenNameFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить имя"
      open={open}
      onCancel={handleClose}
      onOk={() => form.submit()}
      okText="Создать"
      cancelText="Отмена"
      confirmLoading={submitting}
      destroyOnHidden
    >
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
      <Form form={form} layout="vertical" onFinish={onFinish}>
        <Form.Item
          name="canonical"
          label="Каноническая форма"
          rules={[{ required: true, whitespace: true, message: "Введите каноническую форму" }]}
        >
          <Input autoFocus />
        </Form.Item>
        <Form.Item
          name="gender"
          label="Пол"
          rules={[{ required: true, message: "Выберите пол" }]}
        >
          <Select options={GENDER_OPTIONS} placeholder="Выберите пол" />
        </Form.Item>
        <Form.Item label="Варианты написания">
          <TextRefListEditor value={variants} onChange={setVariants} addLabel="+ вариант" />
        </Form.Item>
        <Form.Item label="Носители">
          <TextRefListEditor value={items} onChange={setItems} addLabel="+ запись" />
        </Form.Item>
        <Form.Item label="Заметки">
          <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
        </Form.Item>
      </Form>
    </Modal>
  );
}
```

#### `web/src/pages/GivenNameView.tsx` (создать)
`web/src/pages/GivenNameView.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  Alert,
  Breadcrumb,
  Button,
  Card,
  Descriptions,
  Form,
  Input,
  List,
  Modal,
  Popconfirm,
  Select,
  Space,
  Spin,
  Typography,
} from "antd";
import {
  deleteGivenName,
  fetchGivenName,
  updateGivenName,
  type GivenName,
  type NameGender,
  type TextRef,
} from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { TextRefListEditor } from "../TextRefList";

interface EditFormValues {
  canonical: string;
  gender: NameGender;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["canonical", "gender"];

const GENDER_OPTIONS: { value: NameGender; label: string }[] = [
  { value: "male", label: "Мужской" },
  { value: "female", label: "Женский" },
  { value: "neutral", label: "Нейтральный (вывод пола запрещён)" },
];

const GENDER_LABELS: Record<string, string> = {
  male: "Мужской",
  female: "Женский",
  neutral: "Нейтральный",
};

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

// GivenNameView — просмотр словарной записи имени, переключаемый в форму
// редактирования на той же странице (тот же toggle+explicit-save, что
// DivisionView — PUT заменяет запись целиком, docs/data-model/entity-write.md
// §4). Без родителя/детей/дерева — GivenName не иерархична, в отличие от
// AdminDivision.
export default function GivenNameView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [givenName, setGivenName] = useState<GivenName | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [variants, setVariants] = useState<TextRef[]>([]);
  const [items, setItems] = useState<TextRef[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (givenNameId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setGivenName(null);
    fetchGivenName(givenNameId)
      .then(setGivenName)
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
    if (givenName == null) {
      return;
    }
    form.setFieldsValue({ canonical: givenName.canonical, gender: givenName.gender || undefined });
    setVariants(givenName.variants);
    setItems(givenName.items);
    setNotes(givenName.notes);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (givenName == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await updateGivenName(givenName.id, {
        canonical: values.canonical,
        gender: values.gender,
        variants,
        items,
        notes,
      });
      setGivenName(updated);
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
    if (givenName == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteGivenName(givenName.id);
      navigate("/given-names");
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
          <Link to="/given-names">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (givenName == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/given-names">Имена</Link> },
          { title: givenName.canonical },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={givenName.canonical} column={1} bordered size="small">
            <Descriptions.Item label="Пол">{GENDER_LABELS[givenName.gender] ?? "—"}</Descriptions.Item>
            <Descriptions.Item label="Варианты"><TextRefListView items={givenName.variants} /></Descriptions.Item>
            <Descriptions.Item label="Носители"><TextRefListView items={givenName.items} /></Descriptions.Item>
            <Descriptions.Item label="Заметки"><TextRefListView items={givenName.notes} /></Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title={`Удалить «${givenName.canonical}»?`}
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
            name="canonical"
            label="Каноническая форма"
            rules={[{ required: true, whitespace: true, message: "Введите каноническую форму" }]}
          >
            <Input />
          </Form.Item>
          <Form.Item
            name="gender"
            label="Пол"
            rules={[{ required: true, message: "Выберите пол" }]}
          >
            <Select options={GENDER_OPTIONS} placeholder="Выберите пол" />
          </Form.Item>
          <Form.Item label="Варианты написания">
            <TextRefListEditor value={variants} onChange={setVariants} addLabel="+ вариант" />
          </Form.Item>
          <Form.Item label="Носители">
            <TextRefListEditor value={items} onChange={setItems} addLabel="+ запись" />
          </Form.Item>
          <Form.Item label="Заметки">
            <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
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

### Шаг 3.3. `web/src/App.tsx` — роуты GivenName

Добавить импорты (после импортов Title из Задачи 2):
```typescript
import GivenNamesList from "./pages/GivenNamesList";
import GivenNameView from "./pages/GivenNameView";
```
Добавить роуты (после роутов `/titles`/`/titles/:id`, до `/login`):
```typescript
          <Route path="/given-names" element={<PageLayout><GivenNamesList /></PageLayout>} />
          <Route path="/given-names/:id" element={<PageLayout><GivenNameView /></PageLayout>} />
```

### Шаг 3.4. `web/src/pages/EntityCatalog.tsx` — строка «Имена»

В массиве `CATALOG_ENTRIES` добавить:
```typescript
  { label: "Имена", path: "/given-names" },
```

Итоговый вид массива после Задач 2 и 3 (для сверки — порядок внутри массива не влияет на отображение, список сортируется рантаймом):
`web/src/pages/EntityCatalog.tsx`:
```typescript
import { Card, List, Typography } from "antd";
import { Link } from "react-router-dom";

// CATALOG_ENTRIES — единая точка входа приложения: по алфавиту названия.
// Каждая сущность получает свой список при подключении (docs/data-model/
// entity-write.md §4) — здесь просто добавляется новая строка, без вкладок.
const CATALOG_ENTRIES: { label: string; path: string }[] = [
  { label: "Административное деление", path: "/divisions" },
  { label: "Документация", path: "/docs" },
  { label: "Имена", path: "/given-names" },
  { label: "Отчества", path: "/patronymics" },
  { label: "Сословия", path: "/estates" },
  { label: "Титулы", path: "/titles" },
  { label: "Фамилии", path: "/surnames" },
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

### Шаг 3.5. Рубеж

`npm run typecheck` и `npm run build` в `web/` чисты. Живая проверка в браузере (см. «Предпосылка»): каталог на `/` показывает все 7 строк по алфавиту (Административное деление, Документация, Имена, Отчества, Сословия, Титулы, Фамилии); `/given-names` → «+ добавить» → форма с полем «Пол» (обязательный `Select`) → создание → View показывает выбранный пол → «Редактировать» — форма предзаполнена (включая пол) → смена пола → сохранение → View обновился.

### Шаг 3.6. Коммит

```bash
git add web/src/api.ts web/src/App.tsx web/src/pages/EntityCatalog.tsx web/src/pages/GivenNamesList.tsx web/src/pages/GivenNameForm.tsx web/src/pages/GivenNameView.tsx
git commit -m "feat(web): страницы Имён — список, просмотр/редактирование с полем «Пол», форма"
```

## Рубеж прохода

После Задачи 3: `gofmt -l .` пусто, `go build/vet/test ./...` зелёные (763
теста, 52 пакета), `npm run typecheck`/`build` чисты. Каталог `/` — 7
строк по алфавиту. Все 4 словаря (`Patronymic`, `Estate`, `Title`,
`GivenName`) имеют полный CRUD через HTTP, MCP и веб, по тем же
конвенциям, что `Surname`. Следующий подпроект по декомпозиции
`docs/data-model/entity-write.md` §2 — подпроект 3 (`Repository`,
`Church`, `Parish`, `Archive`).

## Коммиты

Три коммита в `main`, по одному на задачу — см. Шаги 1.7, 2.8, 3.6.
