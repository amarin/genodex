# Веб-CRUD/MCP для всех сущностей — подпроект 3 (одиночный FK: Repository, Church, Parish, Archive): план

> **Для агента-исполнителя:** ОБЯЗАТЕЛЬНЫЙ ПОДНАВЫК: superpowers:subagent-driven-development.

Третий проход по декомпозиции `docs/data-model/entity-write.md` §2 — первый
уровень с настоящими внешними ключами: `Repository`, `Church`, `Parish`,
`Archive`. В отличие от подпроектов 1-2 (плоские словари без связей), здесь
впервые встречаются: одиночная необязательная ссылка `*TextRef` (не список
— `Church.Parish`, `Parish.Church`, `Archive.System`), строгий скалярный FK
без текстового fallback (`Archive.RepositoryID`, проверяется на
существование при create/update — по образцу `create_division`'s
`parentErr`), структурированная дата с точностью `FactDate`
(`Parish.Since`/`Until`) и read-only список доказательств `[]SourceLink`
(есть у всех четырёх — `Sources`, Citation ещё не имеет CRUD, подпроект 5).

## Goal

1. Полный CRUD (HTTP + MCP + веб) для `Repository`, `Church`, `Parish`,
   `Archive` — по конвенциям `docs/data-model/entity-write.md` §3-4.
2. Новые переиспользуемые паттерны, впервые вводимые в этом проходе (будут
   использоваться в подпроектах 4-9): одиночный `*TextRef` в вебе (текстовое
   поле вместо списка), строгий скалярный FK через `Select` с бэкендом
   малого списка (`Archive.RepositoryID` → `fetchRepositories()`), полный
   переиспользуемый `FactDateEditor.tsx`, read-only отображение
   `[]SourceLink` без формы создания/редактирования.
3. `Archive.RepositoryID` — единственный в этом проходе настоящий strict FK
   между новыми сущностями и уже существующей (`Repository`) — сценарии
   `create_archive`/`update_archive` проверяют существование хранилища в
   транзакции, по образцу `create_division`.

## Предпосылка: живая проверка

Весь код ниже применён и проверен в отдельном git worktree
(`../genodex-verify-subproject3`, ветка `verify/entity-write-subproject3`) —
не на `main` напрямую (по итогам обратной связи после подпроекта 2). `gofmt
-l .` пусто, `go build/vet/test ./...` — 894 теста, 76 пакетов (было 767/52
после подпроекта 2), `npm run typecheck`/`build` чисты.

Живой смок-тест бэкенда через `curl` (реальный `genodex serve`, чистая БД,
зарегистрированный владелец): создание `Repository` с `type`/`address`/
`urls`/`private` → 201; `Church` без ссылки на `parish` → 201; `Parish` с
`since`/`until` (структурированная дата, полный JSON) → 201, поля
округляются корректно; `Archive` с валидным `repository_id` → 201; `Archive`
с несуществующим `repository_id` → 422 `{"field":"repository_id"}`; попытка
удалить `Repository`, на который ссылается `Archive` → 409 со списком
ссылающихся (тот же generic FK-graph, что и раньше — просто впервые
проверен для этой пары сущностей); `GET /api/repositories` без auth-куки
анонимно НЕ находит `Repository` с `private:true` (существующий generic
контроль доступа, тоже впервые задействован в этом проходе — не баг,
корректное поведение).

Живая проверка веб-UI в браузере (реальный сервер, собранный `web/dist`):
каталог на `/` — 11 строк по алфавиту (добавились «Архивы», «Приходы»,
«Хранилища», «Церкви»); `/repositories` → «+ добавить» → форма с полем
«Тип» (обычный текстовый инпут, не `Select` — открытый enum) → создание →
View с хлебными крошками; `/parishes` → «+ добавить» → `FactDateEditor` для
«Начало периода»: точность «Год», значение 1880, модификатор «Между»
(показывает доп. блок «До:») → 1917 → создание → View показывает «между
1880 и 1917» (`formatFactDate`) → «Редактировать» — форма верно
предзаполнена (точность/год/модификатор/«До»); `/archives` → «+ добавить» →
`Select` «Хранилище» с поиском, показывает созданный `Repository` → выбор →
создание → View показывает название хранилища ссылкой на его View-страницу
(разрешение `repository_id` → имя через уже загруженный список).

## Задача 1. Бэкенд: Repository, Church, Parish, Archive + новые транспортные типы

**Интерфейсы, потребляемые из подпроектов 1-2**: `transport.TextRef`/
`TextRefsFromModel`/`TextRefsToModel` (`internal/transport/text_ref.go`),
`models.SearchQuery`, общие хелперы `internal/httpapi/surname.go`:`parsePage`
и `internal/mcp/surname.go`:`textRefsFromStrings` — используются как есть, НЕ
переопределяются.

**Производит**: `transport.FactDate`/`FactDateFromModel`/`.Model()`
(`internal/transport/fact_date.go`), `transport.SourceLink`/
`SourceLinkFromModel`/`SourceLinksFromModel` (`internal/transport/source_link.go`),
`transport.TextRefFromModelPtr`/`.ModelPtr()` (добавлено в существующий
`internal/transport/text_ref.go` — для одиночных `*TextRef`-полей),
`httpapi.{Repository,Church,Parish,Archive}Service`, `mcp.{...}Service` —
потребляются Задачами 2 и 3 (веб) через HTTP/MCP.

**Файлы:**
- Изменить: `internal/transport/text_ref.go` (добавить в конец файла
  `TextRefFromModelPtr`/`.ModelPtr()`), `internal/httpapi/{deps.go,api.go,httpapi.go}`,
  `internal/mcp/{deps.go,server.go}`, `internal/app/app.go`
- Создать: `internal/transport/fact_date.go`, `internal/transport/source_link.go`,
  `internal/transport/{repository,repository_write,church,church_write,parish,parish_write,archive,archive_write}.go`,
  `internal/usecases/{list_repositories,search_repositories,get_repository,create_repository,update_repository,delete_repository}/{deps.go,scenario.go,scenario_test.go}`
  (и та же шестёрка директорий для `churches`/`church`, `parishes`/`parish`,
  `archives`/`archive` — для `archive` `create_archive`/`update_archive`
  содержат дополнительную проверку `repository_id`, см. Шаг 1.5),
  `internal/httpapi/{repository,repository_write,repository_test,repository_write_test}.go`
  (и то же для `church`, `parish`, `archive`),
  `internal/mcp/{repository,repository_test}.go` (и то же для `church`,
  `parish`, `archive`), `internal/mcp/object_args.go` (общие хелперы для
  объектных MCP-аргументов `*TextRef`/`FactDate` — новая инфраструктура,
  переиспользуется во всех четырёх сущностях этого прохода и далее)

**Важно для исполнителя**: код ниже даётся полностью, файл за файлом — этот
проход НЕ полностью механический, как подпроекты 1-2 (Patronymic/Estate/
Title были байт-в-байт идентичны Surname по форме) — у каждой из четырёх
сущностей своя форма полей, и `create_archive`/`update_archive` содержат
настоящую бизнес-логику (проверка `repository_id`), не только генерацию ID
и вызов `Validate()`/`Save`. Переносить как есть, без сокращений.

### Шаг 1.1. `internal/transport/text_ref.go` — добавить `TextRefFromModelPtr`/`.ModelPtr()`

В конец существующего файла (после `TextRefsToModel`):
```go
// TextRefFromModelPtr конвертирует необязательную ссылку (например,
// Church.Parish); nil — не задана.
func TextRefFromModelPtr(t *models.TextRef) *TextRef {
	if t == nil {
		return nil
	}

	v := TextRefFromModel(*t)

	return &v
}

// ModelPtr конвертирует контракт обратно в необязательную модель; nil — не задана.
func (t *TextRef) ModelPtr() *models.TextRef {
	if t == nil {
		return nil
	}

	v := t.Model()

	return &v
}
```

### Шаг 1.2. `internal/transport/fact_date.go` (создать)
`internal/transport/fact_date.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// FactDate — контракт структурированной даты с точностью (models.FactDate):
// год/месяц/день, верхняя известная точность, формулировка (точно/около/до/
// после/между), календарь (юлианский/григорианский/неизвестен/не указан),
// верхняя граница периода для modifier=between. nil — дата не указана.
type FactDate struct {
	Year      int    `json:"year"`
	Month     int    `json:"month,omitempty"`
	Day       int    `json:"day,omitempty"`
	Precision string `json:"precision"`
	Modifier  string `json:"modifier"`
	Calendar  string `json:"calendar,omitempty"`
	YearTo    int    `json:"year_to,omitempty"`
	MonthTo   int    `json:"month_to,omitempty"`
	DayTo     int    `json:"day_to,omitempty"`
}

// FactDateFromModel конвертирует дату в контракт; nil — дата не указана.
func FactDateFromModel(d *models.FactDate) *FactDate {
	if d == nil {
		return nil
	}

	return &FactDate{
		Year:      d.Year,
		Month:     d.Month,
		Day:       d.Day,
		Precision: string(d.Precision),
		Modifier:  string(d.Modifier),
		Calendar:  string(d.Calendar),
		YearTo:    d.YearTo,
		MonthTo:   d.MonthTo,
		DayTo:     d.DayTo,
	}
}

// Model конвертирует контракт обратно в модель; nil — дата не указана.
func (d *FactDate) Model() *models.FactDate {
	if d == nil {
		return nil
	}

	return &models.FactDate{
		Year:      d.Year,
		Month:     d.Month,
		Day:       d.Day,
		Precision: models.FactPrecision(d.Precision),
		Modifier:  models.FactModifier(d.Modifier),
		Calendar:  models.FactCalendar(d.Calendar),
		YearTo:    d.YearTo,
		MonthTo:   d.MonthTo,
		DayTo:     d.DayTo,
	}
}
```

### Шаг 1.3. `internal/transport/source_link.go` (создать)
`internal/transport/source_link.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// SourceLink — контракт доказательства «утверждение → цитата» (models.SourceLink),
// read-only в v1: Citation ещё не имеет CRUD (docs/data-model/entity-write.md §2,
// подпроект 5), поэтому Create/Update DTO это поле не несут — только чтение
// уже существующих записей (заведены напрямую в БД/через MCP руками, если
// вообще есть). Появляется впервые в этом проходе (первые сущности с полем
// Sources), переиспользуется везде, где встречается []models.SourceLink.
type SourceLink struct {
	CitationID  string `json:"citation_id"`
	TargetType  string `json:"target_type,omitempty"`
	TargetID    string `json:"target_id,omitempty"`
	Reliability string `json:"reliability,omitempty"`
	Role        string `json:"role,omitempty"`
	Note        string `json:"note,omitempty"`
}

// SourceLinkFromModel конвертирует запись в контракт.
func SourceLinkFromModel(s models.SourceLink) SourceLink {
	return SourceLink{
		CitationID:  string(s.CitationID),
		TargetType:  string(s.TargetType),
		TargetID:    string(s.TargetID),
		Reliability: string(s.Reliability),
		Role:        s.Role,
		Note:        s.Note,
	}
}

// SourceLinksFromModel конвертирует список; пустой вход даёт пустой срез, а не nil.
func SourceLinksFromModel(ss []models.SourceLink) []SourceLink {
	out := make([]SourceLink, 0, len(ss))
	for _, s := range ss {
		out = append(out, SourceLinkFromModel(s))
	}

	return out
}
```

### Шаг 1.4. Repository — транспорт, usecases, httpapi, MCP

#### `internal/transport/repository.go` (создать)
`internal/transport/repository.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// Repository — контракт хранилища-контейнера источников (GET /api/repositories,
// MCP-тул repository_list). Sources — read-only в v1 (см. internal/transport/source_link.go).
type Repository struct {
	ID      models.ID    `json:"id"`
	Name    string       `json:"name"`
	Type    string       `json:"type"`
	Address string       `json:"address,omitempty"`
	URLs    []TextRef    `json:"urls"`
	Notes   []TextRef    `json:"notes"`
	Sources []SourceLink `json:"sources"`
	Private bool         `json:"private"`
}

// RepositoryFromModel конвертирует запись в контракт.
func RepositoryFromModel(r models.Repository) Repository {
	return Repository{
		ID:      r.ID,
		Name:    r.Name,
		Type:    string(r.Type),
		Address: r.Address,
		URLs:    TextRefsFromModel(r.URLs),
		Notes:   TextRefsFromModel(r.Notes),
		Sources: SourceLinksFromModel(r.Sources),
		Private: r.Private,
	}
}

// RepositoriesFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func RepositoriesFromModels(rs []models.Repository) []Repository {
	out := make([]Repository, 0, len(rs))
	for _, r := range rs {
		out = append(out, RepositoryFromModel(r))
	}

	return out
}
```

#### `internal/transport/repository_write.go` (создать)
`internal/transport/repository_write.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// RepositoryCreate — тело POST /api/repositories и аргументы тула
// repository_create. Идентификатор генерирует сценарий. Sources не входит —
// read-only в v1 (см. source_link.go).
type RepositoryCreate struct {
	Name    string    `json:"name"`
	Type    string    `json:"type"`
	Address string    `json:"address,omitempty"`
	URLs    []TextRef `json:"urls"`
	Notes   []TextRef `json:"notes"`
	Private bool      `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (r RepositoryCreate) Model() models.Repository {
	return models.Repository{
		Name:    r.Name,
		Type:    models.RepositoryType(r.Type),
		Address: r.Address,
		URLs:    TextRefsToModel(r.URLs),
		Notes:   TextRefsToModel(r.Notes),
		Private: r.Private,
	}
}

// RepositoryUpdate — тело PUT /api/repositories/{id} и аргументы тула
// repository_update: полная замена name/type/address/urls/notes/private.
// Sources не входит — read-only в v1, fetch-then-merge сохраняет текущее
// значение (internal/httpapi/repository_write.go).
type RepositoryUpdate struct {
	Name    string    `json:"name"`
	Type    string    `json:"type"`
	Address string    `json:"address,omitempty"`
	URLs    []TextRef `json:"urls"`
	Notes   []TextRef `json:"notes"`
	Private bool      `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (r RepositoryUpdate) Model() models.Repository {
	return models.Repository{
		Name:    r.Name,
		Type:    models.RepositoryType(r.Type),
		Address: r.Address,
		URLs:    TextRefsToModel(r.URLs),
		Notes:   TextRefsToModel(r.Notes),
		Private: r.Private,
	}
}
```

#### `internal/usecases/list_repositories/` (создать)
`internal/usecases/list_repositories/deps.go`:
```go
package list_repositories

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// RepositoryRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type RepositoryRepo interface {
	ListRepositories(ctx context.Context, access models.Access, page models.Page) ([]*models.Repository, error)
}
```

`internal/usecases/list_repositories/scenario.go`:
```go
package list_repositories

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список словарных записей фамилий».
type Scenario struct {
	repositories RepositoryRepo
}

// New создаёт сценарий.
func New(repositories RepositoryRepo) *Scenario {
	return &Scenario{repositories: repositories}
}

// ListRepositories возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListRepositories(ctx context.Context, access models.Access, page models.Page) ([]models.Repository, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeRepository, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeRepository, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.repositories.ListRepositories(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Repository, 0, len(list))
	for _, sn := range list {
		out = append(out, *sn)
	}

	return out, nil
}
```

`internal/usecases/list_repositories/scenario_test.go`:
```go
package list_repositories

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.Repository
}

func (f *fakeRepo) ListRepositories(_ context.Context, _ models.Access, page models.Page) ([]*models.Repository, error) {
	f.page = page

	return f.out, nil
}

func TestListRepositoriesReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.Repository{{ID: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "ГАВО"}}}

	got, err := New(repo).ListRepositories(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}

	if len(got) != 1 || got[0].Name != "ГАВО" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListRepositoriesRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListRepositories(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
```

#### `internal/usecases/search_repositories/` (создать)
`internal/usecases/search_repositories/deps.go`:
```go
package search_repositories

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// RepositoryRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type RepositoryRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetRepository(ctx context.Context, id models.ID) (*models.Repository, error)
}
```

`internal/usecases/search_repositories/scenario.go`:
```go
package search_repositories

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск словарных записей фамилий».
type Scenario struct {
	repositories RepositoryRepo
}

// New создаёт сценарий.
func New(repositories RepositoryRepo) *Scenario {
	return &Scenario{repositories: repositories}
}

// SearchRepositories находит записи, чья каноническая форма (или вариант)
// начинается с текста запроса — та же механика, что и
// search_divisions.SearchDivisions (см. её комментарий).
func (s *Scenario) SearchRepositories(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Repository, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Repository{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Repository{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.repositories.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeRepository {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.repositories.GetRepository(ctx, h.ID)
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

`internal/usecases/search_repositories/scenario_test.go`:
```go
package search_repositories

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits         []models.Hit
	repositories map[models.ID]*models.Repository
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetRepository(_ context.Context, id models.ID) (*models.Repository, error) {
	s, ok := f.repositories[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestSearchRepositoriesFiltersByType(t *testing.T) {
	id := models.ID("R-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeRepository, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не repository — должен быть пропущен
		},
		repositories: map[models.ID]*models.Repository{id: {ID: id, Name: "ГАВО"}},
	}

	got, err := New(repo).SearchRepositories(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchRepositories: %v", err)
	}

	if len(got) != 1 || got[0].Name != "ГАВО" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchRepositoriesEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchRepositories(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchRepositories: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
```

#### `internal/usecases/get_repository/` (создать)
`internal/usecases/get_repository/deps.go`:
```go
package get_repository

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// RepositoryRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type RepositoryRepo interface {
	GetRepository(ctx context.Context, id models.ID) (*models.Repository, error)
}
```

`internal/usecases/get_repository/scenario.go`:
```go
package get_repository

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «словарная запись фамилии по идентификатору».
type Scenario struct {
	repositories RepositoryRepo
}

// New создаёт сценарий.
func New(repositories RepositoryRepo) *Scenario {
	return &Scenario{repositories: repositories}
}

// GetRepository возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound.
func (s *Scenario) GetRepository(ctx context.Context, id models.ID) (models.Repository, error) {
	if err := validateID(id); err != nil {
		return models.Repository{}, err
	}

	sn, err := s.repositories.GetRepository(ctx, id)
	if err != nil {
		return models.Repository{}, err
	}

	return *sn, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeRepository)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeRepository,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

`internal/usecases/get_repository/scenario_test.go`:
```go
package get_repository

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	repositories map[models.ID]*models.Repository
}

func (f *fakeRepo) GetRepository(_ context.Context, id models.ID) (*models.Repository, error) {
	s, ok := f.repositories[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestGetRepositoryReturnsRecord(t *testing.T) {
	id := models.ID("R-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{repositories: map[models.ID]*models.Repository{id: {ID: id, Name: "ГАВО"}}}

	got, err := New(repo).GetRepository(context.Background(), id)
	if err != nil {
		t.Fatalf("GetRepository: %v", err)
	}

	if got.Name != "ГАВО" {
		t.Fatalf("Name = %q", got.Name)
	}
}

func TestGetRepositoryNotFound(t *testing.T) {
	repo := &fakeRepo{repositories: map[models.ID]*models.Repository{}}

	_, err := New(repo).GetRepository(context.Background(), "R-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetRepositoryInvalidID(t *testing.T) {
	repo := &fakeRepo{repositories: map[models.ID]*models.Repository{}}

	_, err := New(repo).GetRepository(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}
```

#### `internal/usecases/create_repository/` (создать)
`internal/usecases/create_repository/deps.go`:
```go
package create_repository

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// RepositoryStore — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type RepositoryStore interface {
	SaveRepository(ctx context.Context, s *models.Repository) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
```

`internal/usecases/create_repository/scenario.go`:
```go
package create_repository

import (
	"context"
	"fmt"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «создание словарной записи фамилии».
type Scenario struct {
	store RepositoryStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st RepositoryStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateRepository создаёт запись: генерирует идентификатор, проверяет
// инварианты, сохраняет. Возвращает созданную запись с заполненным ID.
// Флат-сущность без FK — в отличие от create_division, без транзакционной
// проверки родителя.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта) и невалидная сущность — *models.ValidationError; прочее —
// ошибки хранилища как есть.
func (s *Scenario) CreateRepository(ctx context.Context, sn models.Repository) (models.Repository, error) {
	if sn.ID != "" {
		return models.Repository{}, &models.ValidationError{
			Entity: models.TypeRepository,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", sn.ID),
		}
	}

	sn.ID = s.ids.New(models.TypeRepository)

	if err := sn.Validate(); err != nil {
		return models.Repository{}, err
	}

	if err := s.store.SaveRepository(ctx, &sn); err != nil {
		return models.Repository{}, err
	}

	return sn, nil
}
```

`internal/usecases/create_repository/scenario_test.go`:
```go
package create_repository

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type stubIDs struct{ id models.ID }

func (s *stubIDs) New(models.Type) models.ID { return s.id }

type fakeStore struct {
	saved   *models.Repository
	saveErr error
}

func (f *fakeStore) SaveRepository(_ context.Context, s *models.Repository) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *s
	f.saved = &cp

	return nil
}

func TestCreateRepositoryGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{}
	sc := New(st, ids)

	got, err := sc.CreateRepository(context.Background(), models.Repository{Name: "ГАВО", Type: models.RepositoryTypeArchive})
	if err != nil {
		t.Fatalf("CreateRepository: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.saved == nil || st.saved.Name != "ГАВО" {
		t.Fatalf("saved = %+v", st.saved)
	}
}

func TestCreateRepositoryRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{})

	_, err := sc.CreateRepository(context.Background(), models.Repository{ID: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "x"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestCreateRepositoryRejectsEmptyName(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{id: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateRepository(context.Background(), models.Repository{})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, want ValidationError on name", err)
	}
}

// TestCreateRepositoryRejectsInvalidType — единственное отличие Repository от
// Surname: тип хранилища обязателен (открытый enum — формат [a-z][a-z0-9_-]*,
// internal/models/dictionary_validate.go, а не фиксированный список).
func TestCreateRepositoryRejectsInvalidType(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{id: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateRepository(context.Background(), models.Repository{Name: "ГАВО", Type: "Not Valid"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "type" {
		t.Fatalf("err = %v, want ValidationError on type", err)
	}
}

func TestCreateRepositoryPropagatesStoreError(t *testing.T) {
	wantErr := errors.New("db down")
	st := &fakeStore{saveErr: wantErr}
	sc := New(st, &stubIDs{id: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateRepository(context.Background(), models.Repository{Name: "ГАВО", Type: models.RepositoryTypeArchive})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}
```

#### `internal/usecases/update_repository/` (создать)
`internal/usecases/update_repository/deps.go`:
```go
package update_repository

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// RepositoryStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования и сохранение идут в одной транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type RepositoryStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
```

`internal/usecases/update_repository/scenario.go`:
```go
package update_repository

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение словарной записи фамилии».
type Scenario struct {
	store RepositoryStore
}

// New создаёт сценарий.
func New(st RepositoryStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateRepository полностью заменяет запись по sn.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, и сохраняет.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateRepository(ctx context.Context, sn models.Repository) error {
	if err := sn.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetRepository(ctx, sn.ID); err != nil {
			return err
		}

		return tx.SaveRepository(ctx, &sn)
	})
}
```

`internal/usecases/update_repository/scenario_test.go`:
```go
package update_repository

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
	repositories map[models.ID]*models.Repository
	saved        []*models.Repository
}

func newFakeTx(existing ...*models.Repository) *fakeTx {
	tx := &fakeTx{repositories: map[models.ID]*models.Repository{}}
	for _, s := range existing {
		tx.repositories[s.ID] = s
	}

	return tx
}

func (f *fakeTx) GetRepository(_ context.Context, id models.ID) (*models.Repository, error) {
	s, ok := f.repositories[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *s

	return &cp, nil
}

func (f *fakeTx) SaveRepository(_ context.Context, s *models.Repository) error {
	cp := *s
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует RepositoryStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func repository(id models.ID, name string) *models.Repository {
	return &models.Repository{ID: id, Name: name, Type: models.RepositoryTypeArchive}
}

func TestUpdateRepositorySaves(t *testing.T) {
	existing := repository("R-01ARZ3NDEKTSV4RRFFQ69G5FA1", "ГАВО")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Name = "ГАВО (испр.)"

	if err := New(st).UpdateRepository(context.Background(), updated); err != nil {
		t.Fatalf("UpdateRepository: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Name != "ГАВО (испр.)" {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateRepositoryNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateRepository(context.Background(), *repository("R-01ARZ3NDEKTSV4RRFFQ69G5FA1", "ГАВО"))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateRepositoryRejectsEmptyName(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateRepository(context.Background(), models.Repository{ID: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, want ValidationError on name", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

// TestUpdateRepositoryRejectsInvalidType — единственное отличие Repository от
// Surname: тип хранилища обязателен (открытый enum).
func TestUpdateRepositoryRejectsInvalidType(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateRepository(context.Background(), models.Repository{ID: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "ГАВО"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "type" {
		t.Fatalf("err = %v, want ValidationError on type", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}
```

#### `internal/usecases/delete_repository/` (создать)
`internal/usecases/delete_repository/deps.go`:
```go
package delete_repository

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// RepositoryRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type RepositoryRepo interface {
	DeleteRepository(ctx context.Context, id models.ID) error
}
```

`internal/usecases/delete_repository/scenario.go`:
```go
package delete_repository

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление словарной записи фамилии».
type Scenario struct {
	repositories RepositoryRepo
}

// New создаёт сценарий.
func New(repositories RepositoryRepo) *Scenario {
	return &Scenario{repositories: repositories}
}

// DeleteRepository удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteRepository(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.repositories.DeleteRepository(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeRepository)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeRepository,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

`internal/usecases/delete_repository/scenario_test.go`:
```go
package delete_repository

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

func (f *fakeRepo) DeleteRepository(_ context.Context, id models.ID) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)

	return nil
}

func TestDeleteRepositoryCallsRepo(t *testing.T) {
	repo := &fakeRepo{}
	id := models.ID("R-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	if err := New(repo).DeleteRepository(context.Background(), id); err != nil {
		t.Fatalf("DeleteRepository: %v", err)
	}

	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("deleted = %v", repo.deleted)
	}
}

func TestDeleteRepositoryInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	err := New(repo).DeleteRepository(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}

	if len(repo.deleted) != 0 {
		t.Fatalf("repo вызван при невалидном id")
	}
}

func TestDeleteRepositoryPropagatesInUseError(t *testing.T) {
	wantErr := &models.InUseError{Type: models.TypeRepository, ID: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	repo := &fakeRepo{err: wantErr}

	err := New(repo).DeleteRepository(context.Background(), "R-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	var iu *models.InUseError
	if !errors.As(err, &iu) {
		t.Fatalf("err = %v, want InUseError", err)
	}
}
```

#### `internal/httpapi/repository.go` (создать)
`internal/httpapi/repository.go`:
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleRepositoryList — GET /api/repositories?limit=&offset=. Чтение открыто
// анонимному посетителю.
func handleRepositoryList(repositories RepositoryService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := repositories.ListRepositories(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.RepositoriesFromModels(list))
	}
}

// handleRepositorySearch — GET /api/repositories/search?q=&limit=&offset=.
func handleRepositorySearch(repositories RepositoryService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := repositories.SearchRepositories(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.RepositoriesFromModels(list))
	}
}

// handleRepositoryGet — GET /api/repositories/{id}.
func handleRepositoryGet(repositories RepositoryService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rep, err := repositories.GetRepository(r.Context(), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.RepositoryFromModel(rep))
	}
}
```

#### `internal/httpapi/repository_write.go` (создать)
`internal/httpapi/repository_write.go`:
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleRepositoryCreate — POST /api/repositories: создаёт запись, отвечает
// 201 с созданной записью (id генерирует сценарий). Запись — только для
// вошедшего владельца, см. handleDivisionCreate.
func handleRepositoryCreate(repositories RepositoryService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.RepositoryCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := repositories.CreateRepository(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.RepositoryFromModel(created))
	}
}

// handleRepositoryUpdate — PUT /api/repositories/{id}: полная замена
// name/type/address/urls/notes/private. Читает текущую версию, накладывает
// поля запроса (fetch-then-merge, docs/data-model/entity-write.md §3).
// Sources не в DTO — read-only в v1 (internal/transport/source_link.go),
// текущее значение cur.Sources не трогается, остаётся как было.
func handleRepositoryUpdate(repositories RepositoryService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.RepositoryUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := repositories.GetRepository(r.Context(), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Name = m.Name
		cur.Type = m.Type
		cur.Address = m.Address
		cur.URLs = m.URLs
		cur.Notes = m.Notes
		cur.Private = m.Private

		if err := repositories.UpdateRepository(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.RepositoryFromModel(cur))
	}
}

// handleRepositoryDelete — DELETE /api/repositories/{id}: 204 без тела;
// занятая запись — 409 со списком ссылающихся. Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handleRepositoryDelete(repositories RepositoryService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := repositories.DeleteRepository(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
```

#### `internal/httpapi/repository_test.go` (создать)
`internal/httpapi/repository_test.go`:
```go
package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepositories struct {
	list []models.Repository
	err  error
	page models.Page

	getR      models.Repository
	gotIDs    []models.ID
	created   models.Repository
	gotCreate models.Repository
	updated   models.Repository
	deleteErr error

	search    []models.Repository
	gotSearch models.SearchQuery
}

func (f *fakeRepositories) ListRepositories(_ context.Context, _ models.Access, page models.Page) ([]models.Repository, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeRepositories) SearchRepositories(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Repository, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeRepositories) GetRepository(_ context.Context, id models.ID) (models.Repository, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Repository{}, f.err
	}

	return f.getR, nil
}

func (f *fakeRepositories) CreateRepository(_ context.Context, r models.Repository) (models.Repository, error) {
	f.gotCreate = r
	if f.err != nil {
		return models.Repository{}, f.err
	}

	return f.created, nil
}

func (f *fakeRepositories) UpdateRepository(_ context.Context, r models.Repository) error {
	f.updated = r

	return f.err
}

func (f *fakeRepositories) DeleteRepository(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestRepositoryListReturnsRecords(t *testing.T) {
	svc := &fakeRepositories{list: []models.Repository{{ID: "R-1", Name: "ГАВО", Type: models.RepositoryTypeArchive}}}

	rec := get(t, NewHandler(Deps{Repositories: svc, DocsFS: fstest.MapFS{}}), "/api/repositories")
	requireStatus(t, rec, 200)

	want := `[{"id":"R-1","name":"ГАВО","type":"archive","urls":[],"notes":[],"sources":[],"private":false}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestRepositoryGetNotFound(t *testing.T) {
	svc := &fakeRepositories{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Repositories: svc, DocsFS: fstest.MapFS{}}), "/api/repositories/R-1")
	requireStatus(t, rec, 404)
}

func TestRepositorySearchPassesQuery(t *testing.T) {
	svc := &fakeRepositories{}

	rec := get(t, NewHandler(Deps{Repositories: svc, DocsFS: fstest.MapFS{}}), "/api/repositories/search?q=ГАВ")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "ГАВ" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
```

#### `internal/httpapi/repository_write_test.go` (создать)
`internal/httpapi/repository_write_test.go`:
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

func TestRepositoryCreateContract(t *testing.T) {
	svc := &fakeRepositories{created: models.Repository{ID: "R-1", Name: "ГАВО", Type: models.RepositoryTypeArchive}}

	rec := postD(t, NewHandler(Deps{Repositories: svc, DocsFS: fstest.MapFS{}}), "/api/repositories",
		`{"name":"ГАВО","type":"archive"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Name != "ГАВО" || svc.gotCreate.Type != models.RepositoryTypeArchive || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
	if !strings.Contains(rec.Body.String(), `"type":"archive"`) {
		t.Fatalf("body = %s, type не в ответе", rec.Body)
	}
}

func TestRepositoryCreateAnonymousIs401(t *testing.T) {
	svc := &fakeRepositories{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Repositories: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/repositories", strings.NewReader(`{"name":"ГАВО","type":"archive"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestRepositoryUpdateMergesFields(t *testing.T) {
	svc := &fakeRepositories{getR: models.Repository{ID: "R-1", Name: "ГАВО", Type: models.RepositoryTypeArchive}}

	rec := putD(t, NewHandler(Deps{Repositories: svc, DocsFS: fstest.MapFS{}}), "/api/repositories/R-1",
		`{"name":"ГАВО (испр.)","type":"library","address":"г. Владимир","private":true}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Name != "ГАВО (испр.)" || svc.updated.Type != models.RepositoryTypeLibrary ||
		svc.updated.Address != "г. Владимир" || svc.updated.Private != true || svc.updated.ID != "R-1" {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestRepositoryDeleteNoContent(t *testing.T) {
	svc := &fakeRepositories{}

	rec := delD(t, NewHandler(Deps{Repositories: svc, DocsFS: fstest.MapFS{}}), "/api/repositories/R-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestRepositoryDeleteInUseIs409(t *testing.T) {
	svc := &fakeRepositories{deleteErr: &models.InUseError{Type: models.TypeRepository, ID: "R-1"}}

	rec := delD(t, NewHandler(Deps{Repositories: svc, DocsFS: fstest.MapFS{}}), "/api/repositories/R-1")

	requireStatus(t, rec, http.StatusConflict)
}
```

#### `internal/mcp/repository.go` (создать)
`internal/mcp/repository.go`:
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

// registerRepositoryTools регистрирует тулы для работы с хранилищами-
// контейнерами источников. urls/notes — только текстом (v1,
// docs/data-model/entity-write.md §4). ВАЖНО: repository_update заменяет
// списки целиком текстом — существующие ref/type будут потеряны при любом
// обновлении через MCP, пока не появится picker.
func registerRepositoryTools(s *server.MCPServer, repositories RepositoryService) {
	tool := mcp.NewTool(
		"repository_list",
		mcp.WithDescription("Список хранилищ-контейнеров источников в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, repositoryListHandler(repositories))

	tool = mcp.NewTool(
		"repository_search",
		mcp.WithDescription("Поиск хранилищ по началу названия; результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало названия")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, repositorySearchHandler(repositories))

	tool = mcp.NewTool(
		"repository_get",
		mcp.WithDescription("Хранилище по id; результат — JSON записи. Неверный формат id или отсутствующая запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например R-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, repositoryGetHandler(repositories))

	tool = mcp.NewTool(
		"repository_create",
		mcp.WithDescription("Создать хранилище; id генерируется сервером; результат — JSON созданной записи"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Название")),
		mcp.WithString("type", mcp.Required(), mcp.Description("Тип хранилища: открытый список, формат [a-z][a-z0-9_-]* (archive/library/museum/private/other — типовые значения, допустимы и другие)")),
		mcp.WithString("address", mcp.Description("Адрес")),
		mcp.WithArray("urls", mcp.WithStringItems(), mcp.Description("Ссылки (URL/DOI и т.п., текстом)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, repositoryCreateHandler(repositories))

	tool = mcp.NewTool(
		"repository_update",
		mcp.WithDescription("Изменить хранилище: полная замена name/type/address/urls/notes/private; результат — JSON обновлённой записи"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Новое название")),
		mcp.WithString("type", mcp.Required(), mcp.Description("Новый тип")),
		mcp.WithString("address", mcp.Description("Адрес")),
		mcp.WithArray("urls", mcp.WithStringItems(), mcp.Description("Ссылки")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, repositoryUpdateHandler(repositories))

	tool = mcp.NewTool(
		"repository_delete",
		mcp.WithDescription("Удалить хранилище. Необратимо. Если на него есть строгие ссылки (Archive.repository_id) — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, repositoryDeleteHandler(repositories))
}

func repositoryListHandler(repositories RepositoryService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := repositories.ListRepositories(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.RepositoriesFromModels(list))
	}
}

func repositorySearchHandler(repositories RepositoryService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := repositories.SearchRepositories(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.RepositoriesFromModels(list))
	}
}

func repositoryGetHandler(repositories RepositoryService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		r, err := repositories.GetRepository(ctx, models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.RepositoryFromModel(r))
	}
}

func repositoryCreateHandler(repositories RepositoryService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		r := models.Repository{
			Name:    req.GetString("name", ""),
			Type:    models.RepositoryType(req.GetString("type", "")),
			Address: req.GetString("address", ""),
			URLs:    textRefsFromStrings(req.GetStringSlice("urls", nil)),
			Notes:   textRefsFromStrings(req.GetStringSlice("notes", nil)),
			Private: req.GetBool("private", false),
		}

		created, err := repositories.CreateRepository(ctx, r)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.RepositoryFromModel(created))
	}
}

func repositoryUpdateHandler(repositories RepositoryService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := repositories.GetRepository(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		cur.Name = req.GetString("name", "")
		cur.Type = models.RepositoryType(req.GetString("type", ""))
		cur.Address = req.GetString("address", "")
		cur.URLs = textRefsFromStrings(req.GetStringSlice("urls", nil))
		cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))
		cur.Private = req.GetBool("private", false)

		if err := repositories.UpdateRepository(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.RepositoryFromModel(cur))
	}
}

func repositoryDeleteHandler(repositories RepositoryService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := repositories.DeleteRepository(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
```

#### `internal/mcp/repository_test.go` (создать)
`internal/mcp/repository_test.go`:
```go
package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepositories struct {
	list []models.Repository
	err  error

	getR      models.Repository
	created   models.Repository
	gotCreate models.Repository
	updated   models.Repository
	gotIDs    []models.ID
	deleteErr error

	search []models.Repository
}

func (f *fakeRepositories) ListRepositories(context.Context, models.Access, models.Page) ([]models.Repository, error) {
	return f.list, f.err
}

func (f *fakeRepositories) SearchRepositories(context.Context, models.Access, models.SearchQuery) ([]models.Repository, error) {
	return f.search, f.err
}

func (f *fakeRepositories) GetRepository(_ context.Context, id models.ID) (models.Repository, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Repository{}, f.err
	}

	return f.getR, nil
}

func (f *fakeRepositories) CreateRepository(_ context.Context, r models.Repository) (models.Repository, error) {
	f.gotCreate = r
	if f.err != nil {
		return models.Repository{}, f.err
	}

	return f.created, nil
}

func (f *fakeRepositories) UpdateRepository(_ context.Context, r models.Repository) error {
	f.updated = r

	return f.err
}

func (f *fakeRepositories) DeleteRepository(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callRepositoryTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestRepositoryGetToolContract(t *testing.T) {
	svc := &fakeRepositories{getR: models.Repository{ID: "R-1", Name: "ГАВО", Type: models.RepositoryTypeArchive}}

	res := callRepositoryTool(t, repositoryGetHandler(svc), map[string]any{"id": "R-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "R-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

func TestRepositoryCreateToolPassesType(t *testing.T) {
	svc := &fakeRepositories{created: models.Repository{ID: "R-new", Name: "ГАВО"}}

	res := callRepositoryTool(t, repositoryCreateHandler(svc), map[string]any{
		"name": "ГАВО",
		"type": "archive",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.Name != "ГАВО" || svc.gotCreate.Type != models.RepositoryTypeArchive {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

func TestRepositoryUpdateToolSetsFields(t *testing.T) {
	svc := &fakeRepositories{getR: models.Repository{ID: "R-1", Name: "ГАВО", Type: models.RepositoryTypeArchive}}

	res := callRepositoryTool(t, repositoryUpdateHandler(svc), map[string]any{
		"id":      "R-1",
		"name":    "ГАВО",
		"type":    "library",
		"private": true,
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Type != models.RepositoryTypeLibrary || svc.updated.Private != true {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestRepositoryDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeRepositories{deleteErr: &models.InUseError{Type: models.TypeRepository, ID: "R-1"}}

	res := callRepositoryTool(t, repositoryDeleteHandler(svc), map[string]any{"id": "R-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersRepositoryTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Repositories: &fakeRepositories{}}).ListTools()

	for _, name := range []string{"repository_list", "repository_search", "repository_get", "repository_create", "repository_update", "repository_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
```

### Шаг 1.5. Church — транспорт, usecases, httpapi, MCP

#### `internal/transport/church.go` (создать)
`internal/transport/church.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// Church — контракт церкви (GET /api/churches, MCP-тул church_list). Parish —
// необязательная ссылка (текст или ссылка на приход); Settlements — ссылки на
// административные деления; Variants — простые строки (не TextRef).
type Church struct {
	ID          models.ID    `json:"id"`
	Name        string       `json:"name"`
	Parish      *TextRef     `json:"parish,omitempty"`
	Settlements []TextRef    `json:"settlements"`
	Variants    []string     `json:"variants"`
	Notes       []TextRef    `json:"notes"`
	Sources     []SourceLink `json:"sources"`
}

// ChurchFromModel конвертирует запись в контракт.
func ChurchFromModel(c models.Church) Church {
	return Church{
		ID:          c.ID,
		Name:        c.Name,
		Parish:      TextRefFromModelPtr(c.Parish),
		Settlements: TextRefsFromModel(c.Settlements),
		Variants:    stringsOrEmpty(c.Variants),
		Notes:       TextRefsFromModel(c.Notes),
		Sources:     SourceLinksFromModel(c.Sources),
	}
}

// ChurchesFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func ChurchesFromModels(cs []models.Church) []Church {
	out := make([]Church, 0, len(cs))
	for _, c := range cs {
		out = append(out, ChurchFromModel(c))
	}

	return out
}

// stringsOrEmpty возвращает пустой срез вместо nil (единый вид JSON-ответа с
// остальными списковыми полями).
func stringsOrEmpty(ss []string) []string {
	if ss == nil {
		return []string{}
	}

	return ss
}
```

#### `internal/transport/church_write.go` (создать)
`internal/transport/church_write.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// ChurchCreate — тело POST /api/churches и аргументы тула church_create.
// Идентификатор генерирует сценарий. Parish редактируется только текстом (v1,
// docs/data-model/entity-write.md §4) — элемент с уже заполненным ref через
// этот DTO не создать.
type ChurchCreate struct {
	Name        string    `json:"name"`
	Parish      *TextRef  `json:"parish,omitempty"`
	Settlements []TextRef `json:"settlements"`
	Variants    []string  `json:"variants"`
	Notes       []TextRef `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (c ChurchCreate) Model() models.Church {
	return models.Church{
		Name:        c.Name,
		Parish:      c.Parish.ModelPtr(),
		Settlements: TextRefsToModel(c.Settlements),
		Variants:    c.Variants,
		Notes:       TextRefsToModel(c.Notes),
	}
}

// ChurchUpdate — тело PUT /api/churches/{id} и аргументы тула church_update:
// полная замена name/parish/settlements/variants/notes.
type ChurchUpdate struct {
	Name        string    `json:"name"`
	Parish      *TextRef  `json:"parish,omitempty"`
	Settlements []TextRef `json:"settlements"`
	Variants    []string  `json:"variants"`
	Notes       []TextRef `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (c ChurchUpdate) Model() models.Church {
	return models.Church{
		Name:        c.Name,
		Parish:      c.Parish.ModelPtr(),
		Settlements: TextRefsToModel(c.Settlements),
		Variants:    c.Variants,
		Notes:       TextRefsToModel(c.Notes),
	}
}
```

#### `internal/usecases/list_churches/` (создать)
`internal/usecases/list_churches/deps.go`:
```go
package list_churches

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ChurchRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ChurchRepo interface {
	ListChurches(ctx context.Context, access models.Access, page models.Page) ([]*models.Church, error)
}
```

`internal/usecases/list_churches/scenario.go`:
```go
package list_churches

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список словарных записей фамилий».
type Scenario struct {
	churches ChurchRepo
}

// New создаёт сценарий.
func New(churches ChurchRepo) *Scenario {
	return &Scenario{churches: churches}
}

// ListChurches возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListChurches(ctx context.Context, access models.Access, page models.Page) ([]models.Church, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeChurch, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeChurch, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.churches.ListChurches(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Church, 0, len(list))
	for _, sn := range list {
		out = append(out, *sn)
	}

	return out, nil
}
```

`internal/usecases/list_churches/scenario_test.go`:
```go
package list_churches

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.Church
}

func (f *fakeRepo) ListChurches(_ context.Context, _ models.Access, page models.Page) ([]*models.Church, error) {
	f.page = page

	return f.out, nil
}

func TestListChurchesReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.Church{{ID: "CH-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "Никольская церковь"}}}

	got, err := New(repo).ListChurches(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListChurches: %v", err)
	}

	if len(got) != 1 || got[0].Name != "Никольская церковь" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListChurchesRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListChurches(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
```

#### `internal/usecases/search_churches/` (создать)
`internal/usecases/search_churches/deps.go`:
```go
package search_churches

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ChurchRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ChurchRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetChurch(ctx context.Context, id models.ID) (*models.Church, error)
}
```

`internal/usecases/search_churches/scenario.go`:
```go
package search_churches

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск словарных записей фамилий».
type Scenario struct {
	churches ChurchRepo
}

// New создаёт сценарий.
func New(churches ChurchRepo) *Scenario {
	return &Scenario{churches: churches}
}

// SearchChurches находит записи, чья каноническая форма (или вариант)
// начинается с текста запроса — та же механика, что и
// search_divisions.SearchDivisions (см. её комментарий).
func (s *Scenario) SearchChurches(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Church, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Church{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Church{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.churches.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeChurch {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.churches.GetChurch(ctx, h.ID)
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

`internal/usecases/search_churches/scenario_test.go`:
```go
package search_churches

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits     []models.Hit
	churches map[models.ID]*models.Church
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetChurch(_ context.Context, id models.ID) (*models.Church, error) {
	s, ok := f.churches[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestSearchChurchesFiltersByType(t *testing.T) {
	id := models.ID("CH-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeChurch, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не church — должен быть пропущен
		},
		churches: map[models.ID]*models.Church{id: {ID: id, Name: "Никольская церковь"}},
	}

	got, err := New(repo).SearchChurches(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchChurches: %v", err)
	}

	if len(got) != 1 || got[0].Name != "Никольская церковь" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchChurchesEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchChurches(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchChurches: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
```

#### `internal/usecases/get_church/` (создать)
`internal/usecases/get_church/deps.go`:
```go
package get_church

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ChurchRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ChurchRepo interface {
	GetChurch(ctx context.Context, id models.ID) (*models.Church, error)
}
```

`internal/usecases/get_church/scenario.go`:
```go
package get_church

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «словарная запись фамилии по идентификатору».
type Scenario struct {
	churches ChurchRepo
}

// New создаёт сценарий.
func New(churches ChurchRepo) *Scenario {
	return &Scenario{churches: churches}
}

// GetChurch возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound.
func (s *Scenario) GetChurch(ctx context.Context, id models.ID) (models.Church, error) {
	if err := validateID(id); err != nil {
		return models.Church{}, err
	}

	sn, err := s.churches.GetChurch(ctx, id)
	if err != nil {
		return models.Church{}, err
	}

	return *sn, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeChurch)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeChurch,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

`internal/usecases/get_church/scenario_test.go`:
```go
package get_church

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	churches map[models.ID]*models.Church
}

func (f *fakeRepo) GetChurch(_ context.Context, id models.ID) (*models.Church, error) {
	s, ok := f.churches[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestGetChurchReturnsRecord(t *testing.T) {
	id := models.ID("CH-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{churches: map[models.ID]*models.Church{id: {ID: id, Name: "Никольская церковь"}}}

	got, err := New(repo).GetChurch(context.Background(), id)
	if err != nil {
		t.Fatalf("GetChurch: %v", err)
	}

	if got.Name != "Никольская церковь" {
		t.Fatalf("Name = %q", got.Name)
	}
}

func TestGetChurchNotFound(t *testing.T) {
	repo := &fakeRepo{churches: map[models.ID]*models.Church{}}

	_, err := New(repo).GetChurch(context.Background(), "CH-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetChurchInvalidID(t *testing.T) {
	repo := &fakeRepo{churches: map[models.ID]*models.Church{}}

	_, err := New(repo).GetChurch(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}
```

#### `internal/usecases/create_church/` (создать)
`internal/usecases/create_church/deps.go`:
```go
package create_church

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ChurchStore — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ChurchStore interface {
	SaveChurch(ctx context.Context, s *models.Church) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
```

`internal/usecases/create_church/scenario.go`:
```go
package create_church

import (
	"context"
	"fmt"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «создание словарной записи фамилии».
type Scenario struct {
	store ChurchStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st ChurchStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateChurch создаёт запись: генерирует идентификатор, проверяет
// инварианты, сохраняет. Возвращает созданную запись с заполненным ID.
// Флат-сущность без FK — в отличие от create_division, без транзакционной
// проверки родителя.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта) и невалидная сущность — *models.ValidationError; прочее —
// ошибки хранилища как есть.
func (s *Scenario) CreateChurch(ctx context.Context, sn models.Church) (models.Church, error) {
	if sn.ID != "" {
		return models.Church{}, &models.ValidationError{
			Entity: models.TypeChurch,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", sn.ID),
		}
	}

	sn.ID = s.ids.New(models.TypeChurch)

	if err := sn.Validate(); err != nil {
		return models.Church{}, err
	}

	if err := s.store.SaveChurch(ctx, &sn); err != nil {
		return models.Church{}, err
	}

	return sn, nil
}
```

`internal/usecases/create_church/scenario_test.go`:
```go
package create_church

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type stubIDs struct{ id models.ID }

func (s *stubIDs) New(models.Type) models.ID { return s.id }

type fakeStore struct {
	saved   *models.Church
	saveErr error
}

func (f *fakeStore) SaveChurch(_ context.Context, s *models.Church) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *s
	f.saved = &cp

	return nil
}

func TestCreateChurchGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "CH-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{}
	sc := New(st, ids)

	got, err := sc.CreateChurch(context.Background(), models.Church{Name: "Никольская церковь"})
	if err != nil {
		t.Fatalf("CreateChurch: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.saved == nil || st.saved.Name != "Никольская церковь" {
		t.Fatalf("saved = %+v", st.saved)
	}
}

func TestCreateChurchRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{})

	_, err := sc.CreateChurch(context.Background(), models.Church{ID: "CH-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "x"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestCreateChurchRejectsEmptyName(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{id: "CH-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateChurch(context.Background(), models.Church{})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, want ValidationError on name", err)
	}
}

func TestCreateChurchPropagatesStoreError(t *testing.T) {
	wantErr := errors.New("db down")
	st := &fakeStore{saveErr: wantErr}
	sc := New(st, &stubIDs{id: "CH-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateChurch(context.Background(), models.Church{Name: "Никольская церковь"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}
```

#### `internal/usecases/update_church/` (создать)
`internal/usecases/update_church/deps.go`:
```go
package update_church

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// ChurchStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования и сохранение идут в одной транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ChurchStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
```

`internal/usecases/update_church/scenario.go`:
```go
package update_church

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение словарной записи фамилии».
type Scenario struct {
	store ChurchStore
}

// New создаёт сценарий.
func New(st ChurchStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateChurch полностью заменяет запись по sn.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, и сохраняет.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateChurch(ctx context.Context, sn models.Church) error {
	if err := sn.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetChurch(ctx, sn.ID); err != nil {
			return err
		}

		return tx.SaveChurch(ctx, &sn)
	})
}
```

`internal/usecases/update_church/scenario_test.go`:
```go
package update_church

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
	churches map[models.ID]*models.Church
	saved    []*models.Church
}

func newFakeTx(existing ...*models.Church) *fakeTx {
	tx := &fakeTx{churches: map[models.ID]*models.Church{}}
	for _, s := range existing {
		tx.churches[s.ID] = s
	}

	return tx
}

func (f *fakeTx) GetChurch(_ context.Context, id models.ID) (*models.Church, error) {
	s, ok := f.churches[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *s

	return &cp, nil
}

func (f *fakeTx) SaveChurch(_ context.Context, s *models.Church) error {
	cp := *s
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует ChurchStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func church(id models.ID, name string) *models.Church {
	return &models.Church{ID: id, Name: name}
}

func TestUpdateChurchSaves(t *testing.T) {
	existing := church("CH-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Никольская церковь")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Name = "Никольская церковь (испр.)"

	if err := New(st).UpdateChurch(context.Background(), updated); err != nil {
		t.Fatalf("UpdateChurch: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Name != "Никольская церковь (испр.)" {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateChurchNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateChurch(context.Background(), *church("CH-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Никольская церковь"))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateChurchRejectsEmptyName(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateChurch(context.Background(), models.Church{ID: "CH-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, want ValidationError on name", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}
```

#### `internal/usecases/delete_church/` (создать)
`internal/usecases/delete_church/deps.go`:
```go
package delete_church

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ChurchRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ChurchRepo interface {
	DeleteChurch(ctx context.Context, id models.ID) error
}
```

`internal/usecases/delete_church/scenario.go`:
```go
package delete_church

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление словарной записи фамилии».
type Scenario struct {
	churches ChurchRepo
}

// New создаёт сценарий.
func New(churches ChurchRepo) *Scenario {
	return &Scenario{churches: churches}
}

// DeleteChurch удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteChurch(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.churches.DeleteChurch(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeChurch)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeChurch,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

`internal/usecases/delete_church/scenario_test.go`:
```go
package delete_church

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

func (f *fakeRepo) DeleteChurch(_ context.Context, id models.ID) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)

	return nil
}

func TestDeleteChurchCallsRepo(t *testing.T) {
	repo := &fakeRepo{}
	id := models.ID("CH-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	if err := New(repo).DeleteChurch(context.Background(), id); err != nil {
		t.Fatalf("DeleteChurch: %v", err)
	}

	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("deleted = %v", repo.deleted)
	}
}

func TestDeleteChurchInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	err := New(repo).DeleteChurch(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}

	if len(repo.deleted) != 0 {
		t.Fatalf("repo вызван при невалидном id")
	}
}

func TestDeleteChurchPropagatesInUseError(t *testing.T) {
	wantErr := &models.InUseError{Type: models.TypeChurch, ID: "CH-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	repo := &fakeRepo{err: wantErr}

	err := New(repo).DeleteChurch(context.Background(), "CH-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	var iu *models.InUseError
	if !errors.As(err, &iu) {
		t.Fatalf("err = %v, want InUseError", err)
	}
}
```

#### `internal/httpapi/church.go` (создать)
`internal/httpapi/church.go`:
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleChurchList — GET /api/churches?limit=&offset=. Чтение открыто
// анонимному посетителю.
func handleChurchList(churches ChurchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := churches.ListChurches(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ChurchesFromModels(list))
	}
}

// handleChurchSearch — GET /api/churches/search?q=&limit=&offset=.
func handleChurchSearch(churches ChurchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := churches.SearchChurches(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ChurchesFromModels(list))
	}
}

// handleChurchGet — GET /api/churches/{id}.
func handleChurchGet(churches ChurchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := churches.GetChurch(r.Context(), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ChurchFromModel(c))
	}
}
```

#### `internal/httpapi/church_write.go` (создать)
`internal/httpapi/church_write.go`:
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleChurchCreate — POST /api/churches: создаёт запись, отвечает 201 с
// созданной записью (id генерирует сценарий). Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handleChurchCreate(churches ChurchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.ChurchCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := churches.CreateChurch(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.ChurchFromModel(created))
	}
}

// handleChurchUpdate — PUT /api/churches/{id}: полная замена
// name/parish/settlements/variants/notes. Читает текущую версию, накладывает
// поля запроса (fetch-then-merge). Sources не в DTO — read-only в v1.
func handleChurchUpdate(churches ChurchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.ChurchUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := churches.GetChurch(r.Context(), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Name = m.Name
		cur.Parish = m.Parish
		cur.Settlements = m.Settlements
		cur.Variants = m.Variants
		cur.Notes = m.Notes

		if err := churches.UpdateChurch(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ChurchFromModel(cur))
	}
}

// handleChurchDelete — DELETE /api/churches/{id}: 204 без тела; занятая
// запись — 409 со списком ссылающихся. Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handleChurchDelete(churches ChurchService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := churches.DeleteChurch(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
```

#### `internal/httpapi/church_test.go` (создать)
`internal/httpapi/church_test.go`:
```go
package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeChurches struct {
	list []models.Church
	err  error
	page models.Page

	getC      models.Church
	gotIDs    []models.ID
	created   models.Church
	gotCreate models.Church
	updated   models.Church
	deleteErr error

	search    []models.Church
	gotSearch models.SearchQuery
}

func (f *fakeChurches) ListChurches(_ context.Context, _ models.Access, page models.Page) ([]models.Church, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeChurches) SearchChurches(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Church, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeChurches) GetChurch(_ context.Context, id models.ID) (models.Church, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Church{}, f.err
	}

	return f.getC, nil
}

func (f *fakeChurches) CreateChurch(_ context.Context, c models.Church) (models.Church, error) {
	f.gotCreate = c
	if f.err != nil {
		return models.Church{}, f.err
	}

	return f.created, nil
}

func (f *fakeChurches) UpdateChurch(_ context.Context, c models.Church) error {
	f.updated = c

	return f.err
}

func (f *fakeChurches) DeleteChurch(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestChurchListReturnsRecords(t *testing.T) {
	svc := &fakeChurches{list: []models.Church{{ID: "CH-1", Name: "Никольская церковь"}}}

	rec := get(t, NewHandler(Deps{Churches: svc, DocsFS: fstest.MapFS{}}), "/api/churches")
	requireStatus(t, rec, 200)

	want := `[{"id":"CH-1","name":"Никольская церковь","settlements":[],"variants":[],"notes":[],"sources":[]}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestChurchGetNotFound(t *testing.T) {
	svc := &fakeChurches{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Churches: svc, DocsFS: fstest.MapFS{}}), "/api/churches/CH-1")
	requireStatus(t, rec, 404)
}

func TestChurchSearchPassesQuery(t *testing.T) {
	svc := &fakeChurches{}

	rec := get(t, NewHandler(Deps{Churches: svc, DocsFS: fstest.MapFS{}}), "/api/churches/search?q=Никол")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "Никол" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
```

#### `internal/httpapi/church_write_test.go` (создать)
`internal/httpapi/church_write_test.go`:
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

func TestChurchCreateContract(t *testing.T) {
	svc := &fakeChurches{created: models.Church{ID: "CH-1", Name: "Никольская церковь"}}

	rec := postD(t, NewHandler(Deps{Churches: svc, DocsFS: fstest.MapFS{}}), "/api/churches",
		`{"name":"Никольская церковь","parish":{"text":"Никольский приход"}}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Name != "Никольская церковь" || svc.gotCreate.Parish == nil ||
		svc.gotCreate.Parish.Text != "Никольский приход" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

func TestChurchCreateAnonymousIs401(t *testing.T) {
	svc := &fakeChurches{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Churches: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/churches", strings.NewReader(`{"name":"Никольская церковь"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestChurchUpdateMergesFields(t *testing.T) {
	svc := &fakeChurches{getC: models.Church{ID: "CH-1", Name: "Никольская церковь"}}

	rec := putD(t, NewHandler(Deps{Churches: svc, DocsFS: fstest.MapFS{}}), "/api/churches/CH-1",
		`{"name":"Никольская церковь (испр.)","variants":["Николаевская церковь"],"settlements":[{"text":"Давыдово"}]}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Name != "Никольская церковь (испр.)" || svc.updated.ID != "CH-1" ||
		len(svc.updated.Variants) != 1 || svc.updated.Variants[0] != "Николаевская церковь" ||
		len(svc.updated.Settlements) != 1 {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestChurchDeleteNoContent(t *testing.T) {
	svc := &fakeChurches{}

	rec := delD(t, NewHandler(Deps{Churches: svc, DocsFS: fstest.MapFS{}}), "/api/churches/CH-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestChurchDeleteInUseIs409(t *testing.T) {
	svc := &fakeChurches{deleteErr: &models.InUseError{Type: models.TypeChurch, ID: "CH-1"}}

	rec := delD(t, NewHandler(Deps{Churches: svc, DocsFS: fstest.MapFS{}}), "/api/churches/CH-1")

	requireStatus(t, rec, http.StatusConflict)
}
```

#### `internal/mcp/church.go` (создать)
`internal/mcp/church.go`:
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

// registerChurchTools регистрирует тулы для работы с церквями. parish —
// одиночная необязательная ссылка (текст или ссылка на приход, объект
// {text, ref?, type?} — v1 создаёт/редактирует только text, ref/type только
// читаются через church_get/list/search). settlements/notes — списки текста
// (v1). variants — простые строки. ВАЖНО: church_update заменяет
// parish/settlements/notes целиком текстом — существующие ref/type будут
// потеряны при любом обновлении через MCP, пока не появится picker.
func registerChurchTools(s *server.MCPServer, churches ChurchService) {
	tool := mcp.NewTool(
		"church_list",
		mcp.WithDescription("Список церквей в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, churchListHandler(churches))

	tool = mcp.NewTool(
		"church_search",
		mcp.WithDescription("Поиск церквей по началу названия; результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало названия")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, churchSearchHandler(churches))

	tool = mcp.NewTool(
		"church_get",
		mcp.WithDescription("Церковь по id; результат — JSON записи. Неверный формат id или отсутствующая запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например CH-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, churchGetHandler(churches))

	tool = mcp.NewTool(
		"church_create",
		mcp.WithDescription("Создать церковь; id генерируется сервером; результат — JSON созданной записи"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Название")),
		mcp.WithObject("parish", mcp.Description("Приход (текстом; ref/type только для чтения)"), mcp.Properties(textRefObjectProperties())),
		mcp.WithArray("settlements", mcp.WithStringItems(), mcp.Description("Населённые пункты (текстом)")),
		mcp.WithArray("variants", mcp.WithStringItems(), mcp.Description("Варианты названия")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
	)
	s.AddTool(tool, churchCreateHandler(churches))

	tool = mcp.NewTool(
		"church_update",
		mcp.WithDescription("Изменить церковь: полная замена name/parish/settlements/variants/notes; результат — JSON обновлённой записи. parish/settlements/notes передаются целиком как текст — если у элемента раньше была ссылка на другую сущность (ref/type из church_get), она будет потеряна: picker для ссылок ещё не реализован (docs/data-model/entity-write.md §5)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Новое название")),
		mcp.WithObject("parish", mcp.Description("Приход (текстом)"), mcp.Properties(textRefObjectProperties())),
		mcp.WithArray("settlements", mcp.WithStringItems(), mcp.Description("Населённые пункты")),
		mcp.WithArray("variants", mcp.WithStringItems(), mcp.Description("Варианты названия")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
	)
	s.AddTool(tool, churchUpdateHandler(churches))

	tool = mcp.NewTool(
		"church_delete",
		mcp.WithDescription("Удалить церковь. Необратимо. Если на неё есть строгие ссылки от других сущностей — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, churchDeleteHandler(churches))
}

func churchListHandler(churches ChurchService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := churches.ListChurches(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.ChurchesFromModels(list))
	}
}

func churchSearchHandler(churches ChurchService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := churches.SearchChurches(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.ChurchesFromModels(list))
	}
}

func churchGetHandler(churches ChurchService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		c, err := churches.GetChurch(ctx, models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.ChurchFromModel(c))
	}
}

func churchCreateHandler(churches ChurchService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parish, err := optionalTextRef(req.GetArguments(), "parish")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		c := models.Church{
			Name:        req.GetString("name", ""),
			Parish:      parish,
			Settlements: textRefsFromStrings(req.GetStringSlice("settlements", nil)),
			Variants:    req.GetStringSlice("variants", nil),
			Notes:       textRefsFromStrings(req.GetStringSlice("notes", nil)),
		}

		created, err := churches.CreateChurch(ctx, c)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.ChurchFromModel(created))
	}
}

func churchUpdateHandler(churches ChurchService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := churches.GetChurch(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		parish, err := optionalTextRef(req.GetArguments(), "parish")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cur.Name = req.GetString("name", "")
		cur.Parish = parish
		cur.Settlements = textRefsFromStrings(req.GetStringSlice("settlements", nil))
		cur.Variants = req.GetStringSlice("variants", nil)
		cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))

		if err := churches.UpdateChurch(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.ChurchFromModel(cur))
	}
}

func churchDeleteHandler(churches ChurchService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := churches.DeleteChurch(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
```

#### `internal/mcp/church_test.go` (создать)
`internal/mcp/church_test.go`:
```go
package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakeChurches struct {
	list []models.Church
	err  error

	getC      models.Church
	created   models.Church
	gotCreate models.Church
	updated   models.Church
	gotIDs    []models.ID
	deleteErr error

	search []models.Church
}

func (f *fakeChurches) ListChurches(context.Context, models.Access, models.Page) ([]models.Church, error) {
	return f.list, f.err
}

func (f *fakeChurches) SearchChurches(context.Context, models.Access, models.SearchQuery) ([]models.Church, error) {
	return f.search, f.err
}

func (f *fakeChurches) GetChurch(_ context.Context, id models.ID) (models.Church, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Church{}, f.err
	}

	return f.getC, nil
}

func (f *fakeChurches) CreateChurch(_ context.Context, c models.Church) (models.Church, error) {
	f.gotCreate = c
	if f.err != nil {
		return models.Church{}, f.err
	}

	return f.created, nil
}

func (f *fakeChurches) UpdateChurch(_ context.Context, c models.Church) error {
	f.updated = c

	return f.err
}

func (f *fakeChurches) DeleteChurch(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callChurchTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestChurchGetToolContract(t *testing.T) {
	svc := &fakeChurches{getC: models.Church{ID: "CH-1", Name: "Никольская церковь"}}

	res := callChurchTool(t, churchGetHandler(svc), map[string]any{"id": "CH-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "CH-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

// TestChurchCreateToolPassesParish — parish передаётся объектным аргументом
// {text, ...}, единственное поле такой формы у Church.
func TestChurchCreateToolPassesParish(t *testing.T) {
	svc := &fakeChurches{created: models.Church{ID: "CH-new", Name: "Никольская церковь"}}

	res := callChurchTool(t, churchCreateHandler(svc), map[string]any{
		"name":   "Никольская церковь",
		"parish": map[string]any{"text": "Никольский приход"},
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.Name != "Никольская церковь" || svc.gotCreate.Parish == nil ||
		svc.gotCreate.Parish.Text != "Никольский приход" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

func TestChurchDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeChurches{deleteErr: &models.InUseError{Type: models.TypeChurch, ID: "CH-1"}}

	res := callChurchTool(t, churchDeleteHandler(svc), map[string]any{"id": "CH-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersChurchTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Churches: &fakeChurches{}}).ListTools()

	for _, name := range []string{"church_list", "church_search", "church_get", "church_create", "church_update", "church_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
```

### Шаг 1.6. Parish — транспорт, usecases, httpapi, MCP

#### `internal/transport/parish.go` (создать)
`internal/transport/parish.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// Parish — контракт прихода (GET /api/parishes, MCP-тул parish_list). Church —
// необязательная ссылка (текст или ссылка на церковь); Since/Until — период
// действия прихода (структурированная дата, см. fact_date.go).
type Parish struct {
	ID          models.ID    `json:"id"`
	Name        string       `json:"name"`
	Church      *TextRef     `json:"church,omitempty"`
	Settlements []TextRef    `json:"settlements"`
	Since       *FactDate    `json:"since,omitempty"`
	Until       *FactDate    `json:"until,omitempty"`
	Notes       []TextRef    `json:"notes"`
	Sources     []SourceLink `json:"sources"`
}

// ParishFromModel конвертирует запись в контракт.
func ParishFromModel(p models.Parish) Parish {
	return Parish{
		ID:          p.ID,
		Name:        p.Name,
		Church:      TextRefFromModelPtr(p.Church),
		Settlements: TextRefsFromModel(p.Settlements),
		Since:       FactDateFromModel(p.Since),
		Until:       FactDateFromModel(p.Until),
		Notes:       TextRefsFromModel(p.Notes),
		Sources:     SourceLinksFromModel(p.Sources),
	}
}

// ParishesFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func ParishesFromModels(ps []models.Parish) []Parish {
	out := make([]Parish, 0, len(ps))
	for _, p := range ps {
		out = append(out, ParishFromModel(p))
	}

	return out
}
```

#### `internal/transport/parish_write.go` (создать)
`internal/transport/parish_write.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// ParishCreate — тело POST /api/parishes и аргументы тула parish_create.
// Идентификатор генерирует сценарий. Church редактируется только текстом (v1).
type ParishCreate struct {
	Name        string    `json:"name"`
	Church      *TextRef  `json:"church,omitempty"`
	Settlements []TextRef `json:"settlements"`
	Since       *FactDate `json:"since,omitempty"`
	Until       *FactDate `json:"until,omitempty"`
	Notes       []TextRef `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (p ParishCreate) Model() models.Parish {
	return models.Parish{
		Name:        p.Name,
		Church:      p.Church.ModelPtr(),
		Settlements: TextRefsToModel(p.Settlements),
		Since:       p.Since.Model(),
		Until:       p.Until.Model(),
		Notes:       TextRefsToModel(p.Notes),
	}
}

// ParishUpdate — тело PUT /api/parishes/{id} и аргументы тула parish_update:
// полная замена name/church/settlements/since/until/notes.
type ParishUpdate struct {
	Name        string    `json:"name"`
	Church      *TextRef  `json:"church,omitempty"`
	Settlements []TextRef `json:"settlements"`
	Since       *FactDate `json:"since,omitempty"`
	Until       *FactDate `json:"until,omitempty"`
	Notes       []TextRef `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (p ParishUpdate) Model() models.Parish {
	return models.Parish{
		Name:        p.Name,
		Church:      p.Church.ModelPtr(),
		Settlements: TextRefsToModel(p.Settlements),
		Since:       p.Since.Model(),
		Until:       p.Until.Model(),
		Notes:       TextRefsToModel(p.Notes),
	}
}
```

#### `internal/usecases/list_parishes/` (создать)
`internal/usecases/list_parishes/deps.go`:
```go
package list_parishes

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ParishRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ParishRepo interface {
	ListParishes(ctx context.Context, access models.Access, page models.Page) ([]*models.Parish, error)
}
```

`internal/usecases/list_parishes/scenario.go`:
```go
package list_parishes

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список словарных записей фамилий».
type Scenario struct {
	parishes ParishRepo
}

// New создаёт сценарий.
func New(parishes ParishRepo) *Scenario {
	return &Scenario{parishes: parishes}
}

// ListParishes возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListParishes(ctx context.Context, access models.Access, page models.Page) ([]models.Parish, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeParish, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeParish, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.parishes.ListParishes(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Parish, 0, len(list))
	for _, sn := range list {
		out = append(out, *sn)
	}

	return out, nil
}
```

`internal/usecases/list_parishes/scenario_test.go`:
```go
package list_parishes

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.Parish
}

func (f *fakeRepo) ListParishes(_ context.Context, _ models.Access, page models.Page) ([]*models.Parish, error) {
	f.page = page

	return f.out, nil
}

func TestListParishesReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.Parish{{ID: "PR-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "Никольский приход"}}}

	got, err := New(repo).ListParishes(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListParishes: %v", err)
	}

	if len(got) != 1 || got[0].Name != "Никольский приход" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListParishesRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListParishes(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
```

#### `internal/usecases/search_parishes/` (создать)
`internal/usecases/search_parishes/deps.go`:
```go
package search_parishes

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ParishRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ParishRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetParish(ctx context.Context, id models.ID) (*models.Parish, error)
}
```

`internal/usecases/search_parishes/scenario.go`:
```go
package search_parishes

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск словарных записей фамилий».
type Scenario struct {
	parishes ParishRepo
}

// New создаёт сценарий.
func New(parishes ParishRepo) *Scenario {
	return &Scenario{parishes: parishes}
}

// SearchParishes находит записи, чья каноническая форма (или вариант)
// начинается с текста запроса — та же механика, что и
// search_divisions.SearchDivisions (см. её комментарий).
func (s *Scenario) SearchParishes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Parish, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Parish{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Parish{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.parishes.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeParish {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.parishes.GetParish(ctx, h.ID)
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

`internal/usecases/search_parishes/scenario_test.go`:
```go
package search_parishes

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits     []models.Hit
	parishes map[models.ID]*models.Parish
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetParish(_ context.Context, id models.ID) (*models.Parish, error) {
	s, ok := f.parishes[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestSearchParishesFiltersByType(t *testing.T) {
	id := models.ID("PR-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeParish, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не parish — должен быть пропущен
		},
		parishes: map[models.ID]*models.Parish{id: {ID: id, Name: "Никольский приход"}},
	}

	got, err := New(repo).SearchParishes(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchParishes: %v", err)
	}

	if len(got) != 1 || got[0].Name != "Никольский приход" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchParishesEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchParishes(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchParishes: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
```

#### `internal/usecases/get_parish/` (создать)
`internal/usecases/get_parish/deps.go`:
```go
package get_parish

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ParishRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ParishRepo interface {
	GetParish(ctx context.Context, id models.ID) (*models.Parish, error)
}
```

`internal/usecases/get_parish/scenario.go`:
```go
package get_parish

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «словарная запись фамилии по идентификатору».
type Scenario struct {
	parishes ParishRepo
}

// New создаёт сценарий.
func New(parishes ParishRepo) *Scenario {
	return &Scenario{parishes: parishes}
}

// GetParish возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound.
func (s *Scenario) GetParish(ctx context.Context, id models.ID) (models.Parish, error) {
	if err := validateID(id); err != nil {
		return models.Parish{}, err
	}

	sn, err := s.parishes.GetParish(ctx, id)
	if err != nil {
		return models.Parish{}, err
	}

	return *sn, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeParish)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeParish,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

`internal/usecases/get_parish/scenario_test.go`:
```go
package get_parish

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	parishes map[models.ID]*models.Parish
}

func (f *fakeRepo) GetParish(_ context.Context, id models.ID) (*models.Parish, error) {
	s, ok := f.parishes[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestGetParishReturnsRecord(t *testing.T) {
	id := models.ID("PR-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{parishes: map[models.ID]*models.Parish{id: {ID: id, Name: "Никольский приход"}}}

	got, err := New(repo).GetParish(context.Background(), id)
	if err != nil {
		t.Fatalf("GetParish: %v", err)
	}

	if got.Name != "Никольский приход" {
		t.Fatalf("Name = %q", got.Name)
	}
}

func TestGetParishNotFound(t *testing.T) {
	repo := &fakeRepo{parishes: map[models.ID]*models.Parish{}}

	_, err := New(repo).GetParish(context.Background(), "PR-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetParishInvalidID(t *testing.T) {
	repo := &fakeRepo{parishes: map[models.ID]*models.Parish{}}

	_, err := New(repo).GetParish(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}
```

#### `internal/usecases/create_parish/` (создать)
`internal/usecases/create_parish/deps.go`:
```go
package create_parish

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ParishStore — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ParishStore interface {
	SaveParish(ctx context.Context, s *models.Parish) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
```

`internal/usecases/create_parish/scenario.go`:
```go
package create_parish

import (
	"context"
	"fmt"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «создание словарной записи фамилии».
type Scenario struct {
	store ParishStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st ParishStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateParish создаёт запись: генерирует идентификатор, проверяет
// инварианты, сохраняет. Возвращает созданную запись с заполненным ID.
// Флат-сущность без FK — в отличие от create_division, без транзакционной
// проверки родителя.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта) и невалидная сущность — *models.ValidationError; прочее —
// ошибки хранилища как есть.
func (s *Scenario) CreateParish(ctx context.Context, sn models.Parish) (models.Parish, error) {
	if sn.ID != "" {
		return models.Parish{}, &models.ValidationError{
			Entity: models.TypeParish,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", sn.ID),
		}
	}

	sn.ID = s.ids.New(models.TypeParish)

	if err := sn.Validate(); err != nil {
		return models.Parish{}, err
	}

	if err := s.store.SaveParish(ctx, &sn); err != nil {
		return models.Parish{}, err
	}

	return sn, nil
}
```

`internal/usecases/create_parish/scenario_test.go`:
```go
package create_parish

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type stubIDs struct{ id models.ID }

func (s *stubIDs) New(models.Type) models.ID { return s.id }

type fakeStore struct {
	saved   *models.Parish
	saveErr error
}

func (f *fakeStore) SaveParish(_ context.Context, s *models.Parish) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *s
	f.saved = &cp

	return nil
}

func TestCreateParishGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "PR-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{}
	sc := New(st, ids)

	got, err := sc.CreateParish(context.Background(), models.Parish{Name: "Никольский приход"})
	if err != nil {
		t.Fatalf("CreateParish: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.saved == nil || st.saved.Name != "Никольский приход" {
		t.Fatalf("saved = %+v", st.saved)
	}
}

func TestCreateParishRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{})

	_, err := sc.CreateParish(context.Background(), models.Parish{ID: "PR-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "x"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestCreateParishRejectsEmptyName(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{id: "PR-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateParish(context.Background(), models.Parish{})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, want ValidationError on name", err)
	}
}

func TestCreateParishPropagatesStoreError(t *testing.T) {
	wantErr := errors.New("db down")
	st := &fakeStore{saveErr: wantErr}
	sc := New(st, &stubIDs{id: "PR-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateParish(context.Background(), models.Parish{Name: "Никольский приход"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}
```

#### `internal/usecases/update_parish/` (создать)
`internal/usecases/update_parish/deps.go`:
```go
package update_parish

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// ParishStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования и сохранение идут в одной транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ParishStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
```

`internal/usecases/update_parish/scenario.go`:
```go
package update_parish

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение словарной записи фамилии».
type Scenario struct {
	store ParishStore
}

// New создаёт сценарий.
func New(st ParishStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateParish полностью заменяет запись по sn.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, и сохраняет.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateParish(ctx context.Context, sn models.Parish) error {
	if err := sn.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetParish(ctx, sn.ID); err != nil {
			return err
		}

		return tx.SaveParish(ctx, &sn)
	})
}
```

`internal/usecases/update_parish/scenario_test.go`:
```go
package update_parish

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
	parishes map[models.ID]*models.Parish
	saved    []*models.Parish
}

func newFakeTx(existing ...*models.Parish) *fakeTx {
	tx := &fakeTx{parishes: map[models.ID]*models.Parish{}}
	for _, s := range existing {
		tx.parishes[s.ID] = s
	}

	return tx
}

func (f *fakeTx) GetParish(_ context.Context, id models.ID) (*models.Parish, error) {
	s, ok := f.parishes[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *s

	return &cp, nil
}

func (f *fakeTx) SaveParish(_ context.Context, s *models.Parish) error {
	cp := *s
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует ParishStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func parish(id models.ID, name string) *models.Parish {
	return &models.Parish{ID: id, Name: name}
}

func TestUpdateParishSaves(t *testing.T) {
	existing := parish("PR-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Никольский приход")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Name = "Никольский приход (испр.)"

	if err := New(st).UpdateParish(context.Background(), updated); err != nil {
		t.Fatalf("UpdateParish: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Name != "Никольский приход (испр.)" {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateParishNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateParish(context.Background(), *parish("PR-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Никольский приход"))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateParishRejectsEmptyName(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateParish(context.Background(), models.Parish{ID: "PR-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, want ValidationError on name", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}
```

#### `internal/usecases/delete_parish/` (создать)
`internal/usecases/delete_parish/deps.go`:
```go
package delete_parish

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ParishRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ParishRepo interface {
	DeleteParish(ctx context.Context, id models.ID) error
}
```

`internal/usecases/delete_parish/scenario.go`:
```go
package delete_parish

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление словарной записи фамилии».
type Scenario struct {
	parishes ParishRepo
}

// New создаёт сценарий.
func New(parishes ParishRepo) *Scenario {
	return &Scenario{parishes: parishes}
}

// DeleteParish удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteParish(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.parishes.DeleteParish(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeParish)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeParish,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

`internal/usecases/delete_parish/scenario_test.go`:
```go
package delete_parish

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

func (f *fakeRepo) DeleteParish(_ context.Context, id models.ID) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)

	return nil
}

func TestDeleteParishCallsRepo(t *testing.T) {
	repo := &fakeRepo{}
	id := models.ID("PR-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	if err := New(repo).DeleteParish(context.Background(), id); err != nil {
		t.Fatalf("DeleteParish: %v", err)
	}

	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("deleted = %v", repo.deleted)
	}
}

func TestDeleteParishInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	err := New(repo).DeleteParish(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}

	if len(repo.deleted) != 0 {
		t.Fatalf("repo вызван при невалидном id")
	}
}

func TestDeleteParishPropagatesInUseError(t *testing.T) {
	wantErr := &models.InUseError{Type: models.TypeParish, ID: "PR-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	repo := &fakeRepo{err: wantErr}

	err := New(repo).DeleteParish(context.Background(), "PR-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	var iu *models.InUseError
	if !errors.As(err, &iu) {
		t.Fatalf("err = %v, want InUseError", err)
	}
}
```

#### `internal/httpapi/parish.go` (создать)
`internal/httpapi/parish.go`:
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleParishList — GET /api/parishes?limit=&offset=. Чтение открыто
// анонимному посетителю.
func handleParishList(parishes ParishService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := parishes.ListParishes(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ParishesFromModels(list))
	}
}

// handleParishSearch — GET /api/parishes/search?q=&limit=&offset=.
func handleParishSearch(parishes ParishService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := parishes.SearchParishes(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ParishesFromModels(list))
	}
}

// handleParishGet — GET /api/parishes/{id}.
func handleParishGet(parishes ParishService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := parishes.GetParish(r.Context(), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ParishFromModel(p))
	}
}
```

#### `internal/httpapi/parish_write.go` (создать)
`internal/httpapi/parish_write.go`:
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleParishCreate — POST /api/parishes: создаёт запись, отвечает 201 с
// созданной записью (id генерирует сценарий). Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handleParishCreate(parishes ParishService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.ParishCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := parishes.CreateParish(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.ParishFromModel(created))
	}
}

// handleParishUpdate — PUT /api/parishes/{id}: полная замена
// name/church/settlements/since/until/notes. Читает текущую версию,
// накладывает поля запроса (fetch-then-merge). Sources не в DTO — read-only
// в v1.
func handleParishUpdate(parishes ParishService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.ParishUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := parishes.GetParish(r.Context(), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Name = m.Name
		cur.Church = m.Church
		cur.Settlements = m.Settlements
		cur.Since = m.Since
		cur.Until = m.Until
		cur.Notes = m.Notes

		if err := parishes.UpdateParish(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ParishFromModel(cur))
	}
}

// handleParishDelete — DELETE /api/parishes/{id}: 204 без тела; занятая
// запись — 409 со списком ссылающихся. Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handleParishDelete(parishes ParishService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := parishes.DeleteParish(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
```

#### `internal/httpapi/parish_test.go` (создать)
`internal/httpapi/parish_test.go`:
```go
package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeParishes struct {
	list []models.Parish
	err  error
	page models.Page

	getP      models.Parish
	gotIDs    []models.ID
	created   models.Parish
	gotCreate models.Parish
	updated   models.Parish
	deleteErr error

	search    []models.Parish
	gotSearch models.SearchQuery
}

func (f *fakeParishes) ListParishes(_ context.Context, _ models.Access, page models.Page) ([]models.Parish, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeParishes) SearchParishes(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Parish, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeParishes) GetParish(_ context.Context, id models.ID) (models.Parish, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Parish{}, f.err
	}

	return f.getP, nil
}

func (f *fakeParishes) CreateParish(_ context.Context, p models.Parish) (models.Parish, error) {
	f.gotCreate = p
	if f.err != nil {
		return models.Parish{}, f.err
	}

	return f.created, nil
}

func (f *fakeParishes) UpdateParish(_ context.Context, p models.Parish) error {
	f.updated = p

	return f.err
}

func (f *fakeParishes) DeleteParish(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestParishListReturnsRecords(t *testing.T) {
	svc := &fakeParishes{list: []models.Parish{{ID: "PR-1", Name: "Никольский приход"}}}

	rec := get(t, NewHandler(Deps{Parishes: svc, DocsFS: fstest.MapFS{}}), "/api/parishes")
	requireStatus(t, rec, 200)

	want := `[{"id":"PR-1","name":"Никольский приход","settlements":[],"notes":[],"sources":[]}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestParishGetNotFound(t *testing.T) {
	svc := &fakeParishes{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Parishes: svc, DocsFS: fstest.MapFS{}}), "/api/parishes/PR-1")
	requireStatus(t, rec, 404)
}

func TestParishSearchPassesQuery(t *testing.T) {
	svc := &fakeParishes{}

	rec := get(t, NewHandler(Deps{Parishes: svc, DocsFS: fstest.MapFS{}}), "/api/parishes/search?q=Никол")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "Никол" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
```

#### `internal/httpapi/parish_write_test.go` (создать)
`internal/httpapi/parish_write_test.go`:
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

func TestParishCreateContract(t *testing.T) {
	svc := &fakeParishes{created: models.Parish{ID: "PR-1", Name: "Никольский приход"}}

	rec := postD(t, NewHandler(Deps{Parishes: svc, DocsFS: fstest.MapFS{}}), "/api/parishes",
		`{"name":"Никольский приход","since":{"year":1880,"precision":"year","modifier":"exact"}}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Name != "Никольский приход" || svc.gotCreate.Since == nil ||
		svc.gotCreate.Since.Year != 1880 || svc.gotCreate.Since.Precision != models.PrecisionYear ||
		svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

func TestParishCreateAnonymousIs401(t *testing.T) {
	svc := &fakeParishes{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Parishes: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/parishes", strings.NewReader(`{"name":"Никольский приход"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

// TestParishUpdateMergesFields — включая since/until (FactDate), единственное
// новое поле, которого не было у Surname/Patronymic/Estate/Title/GivenName.
func TestParishUpdateMergesFields(t *testing.T) {
	svc := &fakeParishes{getP: models.Parish{ID: "PR-1", Name: "Никольский приход"}}

	rec := putD(t, NewHandler(Deps{Parishes: svc, DocsFS: fstest.MapFS{}}), "/api/parishes/PR-1",
		`{"name":"Никольский приход (испр.)",`+
			`"since":{"year":1880,"precision":"year","modifier":"exact"},`+
			`"until":{"year":1917,"precision":"year","modifier":"exact"}}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Name != "Никольский приход (испр.)" || svc.updated.ID != "PR-1" ||
		svc.updated.Since == nil || svc.updated.Since.Year != 1880 ||
		svc.updated.Until == nil || svc.updated.Until.Year != 1917 {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestParishDeleteNoContent(t *testing.T) {
	svc := &fakeParishes{}

	rec := delD(t, NewHandler(Deps{Parishes: svc, DocsFS: fstest.MapFS{}}), "/api/parishes/PR-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestParishDeleteInUseIs409(t *testing.T) {
	svc := &fakeParishes{deleteErr: &models.InUseError{Type: models.TypeParish, ID: "PR-1"}}

	rec := delD(t, NewHandler(Deps{Parishes: svc, DocsFS: fstest.MapFS{}}), "/api/parishes/PR-1")

	requireStatus(t, rec, http.StatusConflict)
}
```

#### `internal/mcp/parish.go` (создать)
`internal/mcp/parish.go`:
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

// registerParishTools регистрирует тулы для работы с приходами. church —
// одиночная необязательная ссылка (текст или ссылка на церковь, v1 — только
// текст). since/until — структурированная дата (объект, см.
// factDateObjectProperties). ВАЖНО: parish_update заменяет
// church/settlements/notes целиком текстом — существующие ref/type будут
// потеряны при любом обновлении через MCP, пока не появится picker.
func registerParishTools(s *server.MCPServer, parishes ParishService) {
	tool := mcp.NewTool(
		"parish_list",
		mcp.WithDescription("Список приходов в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, parishListHandler(parishes))

	tool = mcp.NewTool(
		"parish_search",
		mcp.WithDescription("Поиск приходов по началу названия; результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало названия")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, parishSearchHandler(parishes))

	tool = mcp.NewTool(
		"parish_get",
		mcp.WithDescription("Приход по id; результат — JSON записи. Неверный формат id или отсутствующая запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например PR-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, parishGetHandler(parishes))

	tool = mcp.NewTool(
		"parish_create",
		mcp.WithDescription("Создать приход; id генерируется сервером; результат — JSON созданной записи"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Название")),
		mcp.WithObject("church", mcp.Description("Церковь (текстом; ref/type только для чтения)"), mcp.Properties(textRefObjectProperties())),
		mcp.WithArray("settlements", mcp.WithStringItems(), mcp.Description("Населённые пункты (текстом)")),
		mcp.WithObject("since", mcp.Description("Начало периода действия прихода"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("until", mcp.Description("Конец периода действия прихода"), mcp.Properties(factDateObjectProperties())),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
	)
	s.AddTool(tool, parishCreateHandler(parishes))

	tool = mcp.NewTool(
		"parish_update",
		mcp.WithDescription("Изменить приход: полная замена name/church/settlements/since/until/notes; результат — JSON обновлённой записи. church/settlements/notes передаются целиком как текст — если у элемента раньше была ссылка на другую сущность (ref/type из parish_get), она будет потеряна: picker для ссылок ещё не реализован (docs/data-model/entity-write.md §5)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Новое название")),
		mcp.WithObject("church", mcp.Description("Церковь (текстом)"), mcp.Properties(textRefObjectProperties())),
		mcp.WithArray("settlements", mcp.WithStringItems(), mcp.Description("Населённые пункты")),
		mcp.WithObject("since", mcp.Description("Начало периода"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("until", mcp.Description("Конец периода"), mcp.Properties(factDateObjectProperties())),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
	)
	s.AddTool(tool, parishUpdateHandler(parishes))

	tool = mcp.NewTool(
		"parish_delete",
		mcp.WithDescription("Удалить приход. Необратимо. Если на него есть строгие ссылки от других сущностей — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, parishDeleteHandler(parishes))
}

func parishListHandler(parishes ParishService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := parishes.ListParishes(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.ParishesFromModels(list))
	}
}

func parishSearchHandler(parishes ParishService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := parishes.SearchParishes(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.ParishesFromModels(list))
	}
}

func parishGetHandler(parishes ParishService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		p, err := parishes.GetParish(ctx, models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.ParishFromModel(p))
	}
}

func parishCreateHandler(parishes ParishService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		church, err := optionalTextRef(args, "church")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		since, err := optionalFactDate(args, "since")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		until, err := optionalFactDate(args, "until")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		p := models.Parish{
			Name:        req.GetString("name", ""),
			Church:      church,
			Settlements: textRefsFromStrings(req.GetStringSlice("settlements", nil)),
			Since:       since,
			Until:       until,
			Notes:       textRefsFromStrings(req.GetStringSlice("notes", nil)),
		}

		created, err := parishes.CreateParish(ctx, p)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.ParishFromModel(created))
	}
}

func parishUpdateHandler(parishes ParishService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := parishes.GetParish(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		args := req.GetArguments()

		church, err := optionalTextRef(args, "church")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		since, err := optionalFactDate(args, "since")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		until, err := optionalFactDate(args, "until")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cur.Name = req.GetString("name", "")
		cur.Church = church
		cur.Settlements = textRefsFromStrings(req.GetStringSlice("settlements", nil))
		cur.Since = since
		cur.Until = until
		cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))

		if err := parishes.UpdateParish(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.ParishFromModel(cur))
	}
}

func parishDeleteHandler(parishes ParishService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := parishes.DeleteParish(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
```

#### `internal/mcp/parish_test.go` (создать)
`internal/mcp/parish_test.go`:
```go
package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakeParishes struct {
	list []models.Parish
	err  error

	getP      models.Parish
	created   models.Parish
	gotCreate models.Parish
	updated   models.Parish
	gotIDs    []models.ID
	deleteErr error

	search []models.Parish
}

func (f *fakeParishes) ListParishes(context.Context, models.Access, models.Page) ([]models.Parish, error) {
	return f.list, f.err
}

func (f *fakeParishes) SearchParishes(context.Context, models.Access, models.SearchQuery) ([]models.Parish, error) {
	return f.search, f.err
}

func (f *fakeParishes) GetParish(_ context.Context, id models.ID) (models.Parish, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Parish{}, f.err
	}

	return f.getP, nil
}

func (f *fakeParishes) CreateParish(_ context.Context, p models.Parish) (models.Parish, error) {
	f.gotCreate = p
	if f.err != nil {
		return models.Parish{}, f.err
	}

	return f.created, nil
}

func (f *fakeParishes) UpdateParish(_ context.Context, p models.Parish) error {
	f.updated = p

	return f.err
}

func (f *fakeParishes) DeleteParish(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callParishTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestParishGetToolContract(t *testing.T) {
	svc := &fakeParishes{getP: models.Parish{ID: "PR-1", Name: "Никольский приход"}}

	res := callParishTool(t, parishGetHandler(svc), map[string]any{"id": "PR-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "PR-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

// TestParishCreateToolPassesSince — since передаётся объектным аргументом
// FactDate, единственное такое поле у Parish (и первое в проекте в MCP-туле).
func TestParishCreateToolPassesSince(t *testing.T) {
	svc := &fakeParishes{created: models.Parish{ID: "PR-new", Name: "Никольский приход"}}

	res := callParishTool(t, parishCreateHandler(svc), map[string]any{
		"name": "Никольский приход",
		"since": map[string]any{
			"year": float64(1880), "precision": "year", "modifier": "exact",
		},
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.Name != "Никольский приход" || svc.gotCreate.Since == nil ||
		svc.gotCreate.Since.Year != 1880 || svc.gotCreate.Since.Precision != models.PrecisionYear {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

func TestParishDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeParishes{deleteErr: &models.InUseError{Type: models.TypeParish, ID: "PR-1"}}

	res := callParishTool(t, parishDeleteHandler(svc), map[string]any{"id": "PR-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersParishTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Parishes: &fakeParishes{}}).ListTools()

	for _, name := range []string{"parish_list", "parish_search", "parish_get", "parish_create", "parish_update", "parish_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
```

### Шаг 1.7. Archive — транспорт, usecases, httpapi, MCP

#### `internal/transport/archive.go` (создать)
`internal/transport/archive.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// Archive — контракт архива (GET /api/archives, MCP-тул archive_list). System —
// система иерархии архива, задаётся только именем (text), ссылка на сущность
// не допускается (models.Archive.Validate). RepositoryID — необязательная
// строгая ссылка на хранилище (просто id, не TextRef — в отличие от
// Church.Parish/Parish.Church): пустая строка — не задана.
type Archive struct {
	ID           models.ID    `json:"id"`
	Name         string       `json:"name"`
	System       *TextRef     `json:"system,omitempty"`
	RepositoryID string       `json:"repository_id,omitempty"`
	Notes        []TextRef    `json:"notes"`
	Sources      []SourceLink `json:"sources"`
	Private      bool         `json:"private"`
}

// ArchiveFromModel конвертирует запись в контракт.
func ArchiveFromModel(a models.Archive) Archive {
	return Archive{
		ID:           a.ID,
		Name:         a.Name,
		System:       TextRefFromModelPtr(a.System),
		RepositoryID: string(a.RepositoryID),
		Notes:        TextRefsFromModel(a.Notes),
		Sources:      SourceLinksFromModel(a.Sources),
		Private:      a.Private,
	}
}

// ArchivesFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func ArchivesFromModels(as []models.Archive) []Archive {
	out := make([]Archive, 0, len(as))
	for _, a := range as {
		out = append(out, ArchiveFromModel(a))
	}

	return out
}
```

#### `internal/transport/archive_write.go` (создать)
`internal/transport/archive_write.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// ArchiveCreate — тело POST /api/archives и аргументы тула archive_create.
// Идентификатор генерирует сценарий. RepositoryID — просто id (пустая строка —
// без хранилища); сценарий проверяет существование при непустом значении.
type ArchiveCreate struct {
	Name         string    `json:"name"`
	System       *TextRef  `json:"system,omitempty"`
	RepositoryID string    `json:"repository_id,omitempty"`
	Notes        []TextRef `json:"notes"`
	Private      bool      `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (a ArchiveCreate) Model() models.Archive {
	return models.Archive{
		Name:         a.Name,
		System:       a.System.ModelPtr(),
		RepositoryID: models.ID(a.RepositoryID),
		Notes:        TextRefsToModel(a.Notes),
		Private:      a.Private,
	}
}

// ArchiveUpdate — тело PUT /api/archives/{id} и аргументы тула archive_update:
// полная замена name/system/repository_id/notes/private.
type ArchiveUpdate struct {
	Name         string    `json:"name"`
	System       *TextRef  `json:"system,omitempty"`
	RepositoryID string    `json:"repository_id,omitempty"`
	Notes        []TextRef `json:"notes"`
	Private      bool      `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (a ArchiveUpdate) Model() models.Archive {
	return models.Archive{
		Name:         a.Name,
		System:       a.System.ModelPtr(),
		RepositoryID: models.ID(a.RepositoryID),
		Notes:        TextRefsToModel(a.Notes),
		Private:      a.Private,
	}
}
```

#### `internal/usecases/list_archives/` (создать)
`internal/usecases/list_archives/deps.go`:
```go
package list_archives

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ArchiveRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveRepo interface {
	ListArchives(ctx context.Context, access models.Access, page models.Page) ([]*models.Archive, error)
}
```

`internal/usecases/list_archives/scenario.go`:
```go
package list_archives

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список словарных записей фамилий».
type Scenario struct {
	archives ArchiveRepo
}

// New создаёт сценарий.
func New(archives ArchiveRepo) *Scenario {
	return &Scenario{archives: archives}
}

// ListArchives возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListArchives(ctx context.Context, access models.Access, page models.Page) ([]models.Archive, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeArchive, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeArchive, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.archives.ListArchives(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Archive, 0, len(list))
	for _, sn := range list {
		out = append(out, *sn)
	}

	return out, nil
}
```

`internal/usecases/list_archives/scenario_test.go`:
```go
package list_archives

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.Archive
}

func (f *fakeRepo) ListArchives(_ context.Context, _ models.Access, page models.Page) ([]*models.Archive, error) {
	f.page = page

	return f.out, nil
}

func TestListArchivesReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.Archive{{ID: "AR-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "ГАВО, архив"}}}

	got, err := New(repo).ListArchives(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListArchives: %v", err)
	}

	if len(got) != 1 || got[0].Name != "ГАВО, архив" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListArchivesRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListArchives(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
```

#### `internal/usecases/search_archives/` (создать)
`internal/usecases/search_archives/deps.go`:
```go
package search_archives

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ArchiveRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetArchive(ctx context.Context, id models.ID) (*models.Archive, error)
}
```

`internal/usecases/search_archives/scenario.go`:
```go
package search_archives

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск словарных записей фамилий».
type Scenario struct {
	archives ArchiveRepo
}

// New создаёт сценарий.
func New(archives ArchiveRepo) *Scenario {
	return &Scenario{archives: archives}
}

// SearchArchives находит записи, чья каноническая форма (или вариант)
// начинается с текста запроса — та же механика, что и
// search_divisions.SearchDivisions (см. её комментарий).
func (s *Scenario) SearchArchives(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Archive, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Archive{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Archive{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.archives.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeArchive {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.archives.GetArchive(ctx, h.ID)
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

`internal/usecases/search_archives/scenario_test.go`:
```go
package search_archives

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits     []models.Hit
	archives map[models.ID]*models.Archive
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetArchive(_ context.Context, id models.ID) (*models.Archive, error) {
	s, ok := f.archives[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestSearchArchivesFiltersByType(t *testing.T) {
	id := models.ID("AR-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeArchive, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не archive — должен быть пропущен
		},
		archives: map[models.ID]*models.Archive{id: {ID: id, Name: "ГАВО, архив"}},
	}

	got, err := New(repo).SearchArchives(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchArchives: %v", err)
	}

	if len(got) != 1 || got[0].Name != "ГАВО, архив" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchArchivesEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchArchives(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchArchives: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
```

#### `internal/usecases/get_archive/` (создать)
`internal/usecases/get_archive/deps.go`:
```go
package get_archive

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ArchiveRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveRepo interface {
	GetArchive(ctx context.Context, id models.ID) (*models.Archive, error)
}
```

`internal/usecases/get_archive/scenario.go`:
```go
package get_archive

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «словарная запись фамилии по идентификатору».
type Scenario struct {
	archives ArchiveRepo
}

// New создаёт сценарий.
func New(archives ArchiveRepo) *Scenario {
	return &Scenario{archives: archives}
}

// GetArchive возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound.
func (s *Scenario) GetArchive(ctx context.Context, id models.ID) (models.Archive, error) {
	if err := validateID(id); err != nil {
		return models.Archive{}, err
	}

	sn, err := s.archives.GetArchive(ctx, id)
	if err != nil {
		return models.Archive{}, err
	}

	return *sn, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeArchive)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeArchive,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

`internal/usecases/get_archive/scenario_test.go`:
```go
package get_archive

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	archives map[models.ID]*models.Archive
}

func (f *fakeRepo) GetArchive(_ context.Context, id models.ID) (*models.Archive, error) {
	s, ok := f.archives[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestGetArchiveReturnsRecord(t *testing.T) {
	id := models.ID("AR-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{archives: map[models.ID]*models.Archive{id: {ID: id, Name: "ГАВО, архив"}}}

	got, err := New(repo).GetArchive(context.Background(), id)
	if err != nil {
		t.Fatalf("GetArchive: %v", err)
	}

	if got.Name != "ГАВО, архив" {
		t.Fatalf("Name = %q", got.Name)
	}
}

func TestGetArchiveNotFound(t *testing.T) {
	repo := &fakeRepo{archives: map[models.ID]*models.Archive{}}

	_, err := New(repo).GetArchive(context.Background(), "AR-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetArchiveInvalidID(t *testing.T) {
	repo := &fakeRepo{archives: map[models.ID]*models.Archive{}}

	_, err := New(repo).GetArchive(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}
```

#### `internal/usecases/create_archive/` (создать)
`internal/usecases/create_archive/deps.go`:
```go
package create_archive

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// ArchiveStore — зависимость сценария: транзакция порта store.Store. Проверка
// хранилища (если задано) и сохранение идут в одной транзакции на переданном
// fn хранилище.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
```

`internal/usecases/create_archive/scenario.go`:
```go
package create_archive

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание архива».
type Scenario struct {
	store ArchiveStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st ArchiveStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateArchive создаёт архив: генерирует идентификатор, проверяет инварианты,
// в одной транзакции (если хранилище задано) убеждается в его существовании и
// сохраняет. Возвращает созданный архив с заполненным ID. По образцу
// create_division's проверки родителя, но для одиночного strict FK
// (RepositoryID), а не self-referencing иерархии.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта), невалидная сущность и несуществующее хранилище —
// *models.ValidationError (поля id, repository_id); прочее — ошибки хранилища
// как есть.
func (s *Scenario) CreateArchive(ctx context.Context, a models.Archive) (models.Archive, error) {
	if a.ID != "" {
		return models.Archive{}, &models.ValidationError{
			Entity: models.TypeArchive,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", a.ID),
		}
	}

	a.ID = s.ids.New(models.TypeArchive)

	if err := a.Validate(); err != nil {
		return models.Archive{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		if a.RepositoryID != "" {
			if _, err := tx.GetRepository(ctx, a.RepositoryID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return repositoryErr("хранилище %q не найдено", a.RepositoryID)
				}

				return err
			}
		}

		return tx.SaveArchive(ctx, &a)
	})
	if err != nil {
		return models.Archive{}, err
	}

	return a, nil
}

// repositoryErr — *models.ValidationError по полю repository_id.
func repositoryErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeArchive,
		Field:  "repository_id",
		Reason: fmt.Sprintf(format, args...),
	}
}
```

`internal/usecases/create_archive/scenario_test.go`:
```go
package create_archive

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// arID возвращает корректный идентификатор архива, отличающийся последним символом.
func arID(last byte) models.ID {
	return models.ID("AR-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// rID возвращает корректный идентификатор хранилища.
func rID(last byte) models.ID {
	return models.ID("R-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

type stubIDs struct {
	id      models.ID
	gotType models.Type
}

func (s *stubIDs) New(t models.Type) models.ID {
	s.gotType = t

	return s.id
}

// fakeTx реализует нужные сценарию методы store.Store поверх карт архивов и
// хранилищ; остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	repositories map[models.ID]*models.Repository
	saved        []*models.Archive
	getErr       error
	saveErr      error
}

func newFakeTx(existingRepos ...*models.Repository) *fakeTx {
	tx := &fakeTx{repositories: map[models.ID]*models.Repository{}}
	for _, r := range existingRepos {
		tx.repositories[r.ID] = r
	}

	return tx
}

func (f *fakeTx) GetRepository(_ context.Context, id models.ID) (*models.Repository, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

	r, ok := f.repositories[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *r

	return &cp, nil
}

func (f *fakeTx) SaveArchive(_ context.Context, a *models.Archive) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *a
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует ArchiveStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx      *fakeTx
	inTxErr error
	calls   int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	if f.inTxErr != nil {
		return f.inTxErr
	}

	return fn(f.tx)
}

func validInput() models.Archive {
	return models.Archive{Name: "ГАВО, архив"}
}

func TestCreateArchiveGeneratesIDAndSaves(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	ids := &stubIDs{id: arID('V')}

	got, err := New(st, ids).CreateArchive(context.Background(), validInput())
	if err != nil {
		t.Fatalf("CreateArchive: %v", err)
	}

	if got.ID != arID('V') || got.Name != "ГАВО, архив" {
		t.Fatalf("got %+v, ожидался архив с ID %v", got, arID('V'))
	}

	if ids.gotType != models.TypeArchive {
		t.Errorf("генератор вызван с типом %q, ожидался archive", ids.gotType)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].ID != arID('V') {
		t.Fatalf("InTx=%d, saved=%v; ожидалось одно сохранение внутри одной транзакции", st.calls, st.tx.saved)
	}
}

func TestCreateArchiveRejectsExplicitID(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	ids := &stubIDs{id: arID('V')}

	in := validInput()
	in.ID = arID('0')

	_, err := New(st, ids).CreateArchive(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю id", err)
	}

	if st.calls != 0 || ids.gotType != "" {
		t.Fatalf("InTx=%d, генератор вызван с %q; ожидалось: не вызваны", st.calls, ids.gotType)
	}
}

func TestCreateArchiveValidatesBeforeTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	in := validInput()
	in.Name = ""

	_, err := New(st, &stubIDs{id: arID('V')}).CreateArchive(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю name", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestCreateArchiveWithRepositorySaves(t *testing.T) {
	repo := &models.Repository{ID: rID('0'), Name: "ГАВО", Type: models.RepositoryTypeArchive}
	st := &fakeStore{tx: newFakeTx(repo)}

	in := validInput()
	in.RepositoryID = repo.ID

	got, err := New(st, &stubIDs{id: arID('V')}).CreateArchive(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateArchive: %v", err)
	}

	if got.RepositoryID != repo.ID || len(st.tx.saved) != 1 {
		t.Fatalf("got %+v, saved=%d; ожидалось сохранение со ссылкой на хранилище", got, len(st.tx.saved))
	}
}

// TestCreateArchiveWithoutRepositorySaves: RepositoryID не задан — хранилище
// не проверяется вовсе (GetRepository не вызывается, что подтверждает
// TestCreateArchiveGeneratesIDAndSaves выше с пустым fakeTx).
func TestCreateArchiveRepositoryNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	in := validInput()
	in.RepositoryID = rID('0')

	_, err := New(st, &stubIDs{id: arID('V')}).CreateArchive(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "repository_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю repository_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d архивов при несуществующем хранилище", len(st.tx.saved))
	}
}

func TestCreateArchivePropagatesSaveError(t *testing.T) {
	wantErr := errors.New("save failed")
	st := &fakeStore{tx: newFakeTx()}
	st.tx.saveErr = wantErr

	if _, err := New(st, &stubIDs{id: arID('V')}).CreateArchive(context.Background(), validInput()); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

func TestCreateArchivePropagatesTxError(t *testing.T) {
	wantErr := errors.New("tx failed")
	st := &fakeStore{tx: newFakeTx(), inTxErr: wantErr}

	if _, err := New(st, &stubIDs{id: arID('V')}).CreateArchive(context.Background(), validInput()); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

func TestCreateArchivePropagatesRepositoryGetError(t *testing.T) {
	wantErr := errors.New("get failed")
	st := &fakeStore{tx: newFakeTx()}
	st.tx.getErr = wantErr

	in := validInput()
	in.RepositoryID = rID('0')

	_, err := New(st, &stubIDs{id: arID('V')}).CreateArchive(context.Background(), in)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}

	var ve *models.ValidationError
	if errors.As(err, &ve) {
		t.Fatalf("сбой хранилища превращён в *ValidationError: %v", err)
	}
}
```

#### `internal/usecases/update_archive/` (создать)
`internal/usecases/update_archive/deps.go`:
```go
package update_archive

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// ArchiveStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования, проверка хранилища (если задано) и сохранение идут
// в одной транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
```

`internal/usecases/update_archive/scenario.go`:
```go
package update_archive

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение архива».
type Scenario struct {
	store ArchiveStore
}

// New создаёт сценарий.
func New(st ArchiveStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateArchive полностью заменяет архив по a.ID: проверяет инварианты, в
// одной транзакции убеждается, что архив существует и (если хранилище
// задано) хранилище существует, и сохраняет.
//
// Ошибки: невалидная сущность и несуществующее хранилище —
// *models.ValidationError (соответствующее поле, repository_id); нет такого
// архива — models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateArchive(ctx context.Context, a models.Archive) error {
	if err := a.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetArchive(ctx, a.ID); err != nil {
			return err
		}

		if a.RepositoryID != "" {
			if _, err := tx.GetRepository(ctx, a.RepositoryID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return repositoryErr("хранилище %q не найдено", a.RepositoryID)
				}

				return err
			}
		}

		return tx.SaveArchive(ctx, &a)
	})
}

// repositoryErr — *models.ValidationError по полю repository_id.
func repositoryErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeArchive,
		Field:  "repository_id",
		Reason: fmt.Sprintf(format, args...),
	}
}
```

`internal/usecases/update_archive/scenario_test.go`:
```go
package update_archive

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

func arID(last byte) models.ID {
	return models.ID("AR-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func rID(last byte) models.ID {
	return models.ID("R-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// fakeTx реализует нужные сценарию методы store.Store поверх карт архивов и
// хранилищ; остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	archives     map[models.ID]*models.Archive
	repositories map[models.ID]*models.Repository
	saved        []*models.Archive
}

func newFakeTx(existing ...*models.Archive) *fakeTx {
	tx := &fakeTx{archives: map[models.ID]*models.Archive{}, repositories: map[models.ID]*models.Repository{}}
	for _, a := range existing {
		tx.archives[a.ID] = a
	}

	return tx
}

func (f *fakeTx) withRepository(r *models.Repository) *fakeTx {
	f.repositories[r.ID] = r

	return f
}

func (f *fakeTx) GetArchive(_ context.Context, id models.ID) (*models.Archive, error) {
	a, ok := f.archives[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *a

	return &cp, nil
}

func (f *fakeTx) GetRepository(_ context.Context, id models.ID) (*models.Repository, error) {
	r, ok := f.repositories[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *r

	return &cp, nil
}

func (f *fakeTx) SaveArchive(_ context.Context, a *models.Archive) error {
	cp := *a
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует ArchiveStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func archive(id models.ID, name string) *models.Archive {
	return &models.Archive{ID: id, Name: name}
}

func TestUpdateArchiveSaves(t *testing.T) {
	existing := archive(arID('V'), "ГАВО, архив")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Name = "ГАВО, архив (испр.)"

	if err := New(st).UpdateArchive(context.Background(), updated); err != nil {
		t.Fatalf("UpdateArchive: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Name != "ГАВО, архив (испр.)" {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateArchiveNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateArchive(context.Background(), *archive(arID('V'), "ГАВО, архив"))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateArchiveRejectsEmptyName(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateArchive(context.Background(), models.Archive{ID: arID('V')})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, want ValidationError on name", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestUpdateArchiveWithRepositorySaves(t *testing.T) {
	existing := archive(arID('V'), "ГАВО, архив")
	repo := &models.Repository{ID: rID('0'), Name: "ГАВО", Type: models.RepositoryTypeArchive}
	st := &fakeStore{tx: newFakeTx(existing).withRepository(repo)}

	updated := *existing
	updated.RepositoryID = repo.ID

	if err := New(st).UpdateArchive(context.Background(), updated); err != nil {
		t.Fatalf("UpdateArchive: %v", err)
	}

	if len(st.tx.saved) != 1 || st.tx.saved[0].RepositoryID != repo.ID {
		t.Fatalf("saved = %v", st.tx.saved)
	}
}

func TestUpdateArchiveRepositoryNotFound(t *testing.T) {
	existing := archive(arID('V'), "ГАВО, архив")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.RepositoryID = rID('0')

	err := New(st).UpdateArchive(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "repository_id" {
		t.Fatalf("err = %v, want ValidationError on repository_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d архивов при несуществующем хранилище", len(st.tx.saved))
	}
}
```

#### `internal/usecases/delete_archive/` (создать)
`internal/usecases/delete_archive/deps.go`:
```go
package delete_archive

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ArchiveRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveRepo interface {
	DeleteArchive(ctx context.Context, id models.ID) error
}
```

`internal/usecases/delete_archive/scenario.go`:
```go
package delete_archive

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление словарной записи фамилии».
type Scenario struct {
	archives ArchiveRepo
}

// New создаёт сценарий.
func New(archives ArchiveRepo) *Scenario {
	return &Scenario{archives: archives}
}

// DeleteArchive удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteArchive(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.archives.DeleteArchive(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeArchive)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeArchive,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

`internal/usecases/delete_archive/scenario_test.go`:
```go
package delete_archive

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

func (f *fakeRepo) DeleteArchive(_ context.Context, id models.ID) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)

	return nil
}

func TestDeleteArchiveCallsRepo(t *testing.T) {
	repo := &fakeRepo{}
	id := models.ID("AR-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	if err := New(repo).DeleteArchive(context.Background(), id); err != nil {
		t.Fatalf("DeleteArchive: %v", err)
	}

	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("deleted = %v", repo.deleted)
	}
}

func TestDeleteArchiveInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	err := New(repo).DeleteArchive(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}

	if len(repo.deleted) != 0 {
		t.Fatalf("repo вызван при невалидном id")
	}
}

func TestDeleteArchivePropagatesInUseError(t *testing.T) {
	wantErr := &models.InUseError{Type: models.TypeArchive, ID: "AR-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	repo := &fakeRepo{err: wantErr}

	err := New(repo).DeleteArchive(context.Background(), "AR-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	var iu *models.InUseError
	if !errors.As(err, &iu) {
		t.Fatalf("err = %v, want InUseError", err)
	}
}
```

#### `internal/httpapi/archive.go` (создать)
`internal/httpapi/archive.go`:
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleArchiveList — GET /api/archives?limit=&offset=. Чтение открыто
// анонимному посетителю.
func handleArchiveList(archives ArchiveService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := archives.ListArchives(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ArchivesFromModels(list))
	}
}

// handleArchiveSearch — GET /api/archives/search?q=&limit=&offset=.
func handleArchiveSearch(archives ArchiveService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := archives.SearchArchives(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ArchivesFromModels(list))
	}
}

// handleArchiveGet — GET /api/archives/{id}.
func handleArchiveGet(archives ArchiveService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, err := archives.GetArchive(r.Context(), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ArchiveFromModel(a))
	}
}
```

#### `internal/httpapi/archive_write.go` (создать)
`internal/httpapi/archive_write.go`:
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleArchiveCreate — POST /api/archives: создаёт запись, отвечает 201 с
// созданной записью (id генерирует сценарий). Несуществующее repository_id —
// 422 (см. writeError, та же механика, что и parent_id у делений). Запись —
// только для вошедшего владельца, см. handleDivisionCreate.
func handleArchiveCreate(archives ArchiveService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.ArchiveCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := archives.CreateArchive(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.ArchiveFromModel(created))
	}
}

// handleArchiveUpdate — PUT /api/archives/{id}: полная замена
// name/system/repository_id/notes/private. Читает текущую версию, накладывает
// поля запроса (fetch-then-merge). Sources не в DTO — read-only в v1.
func handleArchiveUpdate(archives ArchiveService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.ArchiveUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := archives.GetArchive(r.Context(), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Name = m.Name
		cur.System = m.System
		cur.RepositoryID = m.RepositoryID
		cur.Notes = m.Notes
		cur.Private = m.Private

		if err := archives.UpdateArchive(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ArchiveFromModel(cur))
	}
}

// handleArchiveDelete — DELETE /api/archives/{id}: 204 без тела; занятая
// запись — 409 со списком ссылающихся. Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handleArchiveDelete(archives ArchiveService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := archives.DeleteArchive(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
```

#### `internal/httpapi/archive_test.go` (создать)
`internal/httpapi/archive_test.go`:
```go
package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeArchives struct {
	list []models.Archive
	err  error
	page models.Page

	getA      models.Archive
	gotIDs    []models.ID
	created   models.Archive
	gotCreate models.Archive
	updated   models.Archive
	deleteErr error

	search    []models.Archive
	gotSearch models.SearchQuery
}

func (f *fakeArchives) ListArchives(_ context.Context, _ models.Access, page models.Page) ([]models.Archive, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeArchives) SearchArchives(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Archive, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeArchives) GetArchive(_ context.Context, id models.ID) (models.Archive, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Archive{}, f.err
	}

	return f.getA, nil
}

func (f *fakeArchives) CreateArchive(_ context.Context, a models.Archive) (models.Archive, error) {
	f.gotCreate = a
	if f.err != nil {
		return models.Archive{}, f.err
	}

	return f.created, nil
}

func (f *fakeArchives) UpdateArchive(_ context.Context, a models.Archive) error {
	f.updated = a

	return f.err
}

func (f *fakeArchives) DeleteArchive(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestArchiveListReturnsRecords(t *testing.T) {
	svc := &fakeArchives{list: []models.Archive{{ID: "AR-1", Name: "ГАВО, архив"}}}

	rec := get(t, NewHandler(Deps{Archives: svc, DocsFS: fstest.MapFS{}}), "/api/archives")
	requireStatus(t, rec, 200)

	want := `[{"id":"AR-1","name":"ГАВО, архив","notes":[],"sources":[],"private":false}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestArchiveGetNotFound(t *testing.T) {
	svc := &fakeArchives{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Archives: svc, DocsFS: fstest.MapFS{}}), "/api/archives/AR-1")
	requireStatus(t, rec, 404)
}

func TestArchiveSearchPassesQuery(t *testing.T) {
	svc := &fakeArchives{}

	rec := get(t, NewHandler(Deps{Archives: svc, DocsFS: fstest.MapFS{}}), "/api/archives/search?q=ГАВ")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "ГАВ" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
```

#### `internal/httpapi/archive_write_test.go` (создать)
`internal/httpapi/archive_write_test.go`:
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

func TestArchiveCreateContract(t *testing.T) {
	svc := &fakeArchives{created: models.Archive{ID: "AR-1", Name: "ГАВО, архив"}}

	rec := postD(t, NewHandler(Deps{Archives: svc, DocsFS: fstest.MapFS{}}), "/api/archives",
		`{"name":"ГАВО, архив","repository_id":"R-1","system":{"text":"фонд-опись-дело"}}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Name != "ГАВО, архив" || svc.gotCreate.RepositoryID != "R-1" ||
		svc.gotCreate.System == nil || svc.gotCreate.System.Text != "фонд-опись-дело" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestArchiveCreateRepositoryNotFoundIs422 — usecase-слой возвращает
// *models.ValidationError по полю repository_id при несуществующем
// хранилище (см. create_archive), writeError мапит это на 422, как и
// parent_id у делений.
func TestArchiveCreateRepositoryNotFoundIs422(t *testing.T) {
	svc := &fakeArchives{err: &models.ValidationError{Entity: models.TypeArchive, Field: "repository_id", Reason: "хранилище не найдено"}}

	rec := postD(t, NewHandler(Deps{Archives: svc, DocsFS: fstest.MapFS{}}), "/api/archives",
		`{"name":"ГАВО, архив","repository_id":"R-999"}`)

	requireStatus(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"repository_id"`) {
		t.Fatalf("body = %s, ожидалось поле repository_id", rec.Body)
	}
}

func TestArchiveCreateAnonymousIs401(t *testing.T) {
	svc := &fakeArchives{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Archives: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/archives", strings.NewReader(`{"name":"ГАВО, архив"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestArchiveUpdateMergesFields(t *testing.T) {
	svc := &fakeArchives{getA: models.Archive{ID: "AR-1", Name: "ГАВО, архив"}}

	rec := putD(t, NewHandler(Deps{Archives: svc, DocsFS: fstest.MapFS{}}), "/api/archives/AR-1",
		`{"name":"ГАВО, архив (испр.)","repository_id":"R-2","private":true}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Name != "ГАВО, архив (испр.)" || svc.updated.RepositoryID != "R-2" ||
		svc.updated.Private != true || svc.updated.ID != "AR-1" {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestArchiveDeleteNoContent(t *testing.T) {
	svc := &fakeArchives{}

	rec := delD(t, NewHandler(Deps{Archives: svc, DocsFS: fstest.MapFS{}}), "/api/archives/AR-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestArchiveDeleteInUseIs409(t *testing.T) {
	svc := &fakeArchives{deleteErr: &models.InUseError{Type: models.TypeArchive, ID: "AR-1"}}

	rec := delD(t, NewHandler(Deps{Archives: svc, DocsFS: fstest.MapFS{}}), "/api/archives/AR-1")

	requireStatus(t, rec, http.StatusConflict)
}
```

#### `internal/mcp/archive.go` (создать)
`internal/mcp/archive.go`:
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

// registerArchiveTools регистрирует тулы для работы с архивами. system —
// система иерархии, только текстом (ссылка на сущность не допускается,
// models.Archive.Validate). repository_id — просто id (не объект TextRef, в
// отличие от system/parish/church у других сущностей): пустая строка — без
// хранилища; сценарий проверяет существование при непустом значении
// (archive_create/archive_update вернут ошибку тула на несуществующий id).
func registerArchiveTools(s *server.MCPServer, archives ArchiveService) {
	tool := mcp.NewTool(
		"archive_list",
		mcp.WithDescription("Список архивов в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, archiveListHandler(archives))

	tool = mcp.NewTool(
		"archive_search",
		mcp.WithDescription("Поиск архивов по началу названия; результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало названия")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, archiveSearchHandler(archives))

	tool = mcp.NewTool(
		"archive_get",
		mcp.WithDescription("Архив по id; результат — JSON записи. Неверный формат id или отсутствующая запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например AR-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, archiveGetHandler(archives))

	tool = mcp.NewTool(
		"archive_create",
		mcp.WithDescription("Создать архив; id генерируется сервером; результат — JSON созданной записи. Несуществующий repository_id — ошибка тула"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Название")),
		mcp.WithObject("system", mcp.Description("Система иерархии архива — только именем, без ссылки"), mcp.Properties(map[string]any{
			"text": map[string]any{"type": "string", "description": "Имя системы иерархии"},
		})),
		mcp.WithString("repository_id", mcp.Description("id хранилища (необязательно; пусто — без хранилища)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, archiveCreateHandler(archives))

	tool = mcp.NewTool(
		"archive_update",
		mcp.WithDescription("Изменить архив: полная замена name/system/repository_id/notes/private; результат — JSON обновлённой записи. Несуществующий repository_id — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Новое название")),
		mcp.WithObject("system", mcp.Description("Система иерархии — только именем"), mcp.Properties(map[string]any{
			"text": map[string]any{"type": "string", "description": "Имя системы иерархии"},
		})),
		mcp.WithString("repository_id", mcp.Description("id хранилища (необязательно)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, archiveUpdateHandler(archives))

	tool = mcp.NewTool(
		"archive_delete",
		mcp.WithDescription("Удалить архив. Необратимо. Если на него есть строгие ссылки от других сущностей — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, archiveDeleteHandler(archives))
}

func archiveListHandler(archives ArchiveService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := archives.ListArchives(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.ArchivesFromModels(list))
	}
}

func archiveSearchHandler(archives ArchiveService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := archives.SearchArchives(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.ArchivesFromModels(list))
	}
}

func archiveGetHandler(archives ArchiveService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		a, err := archives.GetArchive(ctx, models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.ArchiveFromModel(a))
	}
}

func archiveCreateHandler(archives ArchiveService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		system, err := optionalTextRef(req.GetArguments(), "system")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		a := models.Archive{
			Name:         req.GetString("name", ""),
			System:       system,
			RepositoryID: models.ID(req.GetString("repository_id", "")),
			Notes:        textRefsFromStrings(req.GetStringSlice("notes", nil)),
			Private:      req.GetBool("private", false),
		}

		created, err := archives.CreateArchive(ctx, a)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.ArchiveFromModel(created))
	}
}

func archiveUpdateHandler(archives ArchiveService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := archives.GetArchive(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		system, err := optionalTextRef(req.GetArguments(), "system")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cur.Name = req.GetString("name", "")
		cur.System = system
		cur.RepositoryID = models.ID(req.GetString("repository_id", ""))
		cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))
		cur.Private = req.GetBool("private", false)

		if err := archives.UpdateArchive(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.ArchiveFromModel(cur))
	}
}

func archiveDeleteHandler(archives ArchiveService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := archives.DeleteArchive(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
```

#### `internal/mcp/archive_test.go` (создать)
`internal/mcp/archive_test.go`:
```go
package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakeArchives struct {
	list []models.Archive
	err  error

	getA      models.Archive
	created   models.Archive
	gotCreate models.Archive
	updated   models.Archive
	gotIDs    []models.ID
	deleteErr error

	search []models.Archive
}

func (f *fakeArchives) ListArchives(context.Context, models.Access, models.Page) ([]models.Archive, error) {
	return f.list, f.err
}

func (f *fakeArchives) SearchArchives(context.Context, models.Access, models.SearchQuery) ([]models.Archive, error) {
	return f.search, f.err
}

func (f *fakeArchives) GetArchive(_ context.Context, id models.ID) (models.Archive, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Archive{}, f.err
	}

	return f.getA, nil
}

func (f *fakeArchives) CreateArchive(_ context.Context, a models.Archive) (models.Archive, error) {
	f.gotCreate = a
	if f.err != nil {
		return models.Archive{}, f.err
	}

	return f.created, nil
}

func (f *fakeArchives) UpdateArchive(_ context.Context, a models.Archive) error {
	f.updated = a

	return f.err
}

func (f *fakeArchives) DeleteArchive(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callArchiveTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestArchiveGetToolContract(t *testing.T) {
	svc := &fakeArchives{getA: models.Archive{ID: "AR-1", Name: "ГАВО, архив"}}

	res := callArchiveTool(t, archiveGetHandler(svc), map[string]any{"id": "AR-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "AR-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

// TestArchiveCreateToolPassesRepositoryID — repository_id передаётся плоской
// строкой (не объектом), в отличие от parish/church/system.
func TestArchiveCreateToolPassesRepositoryID(t *testing.T) {
	svc := &fakeArchives{created: models.Archive{ID: "AR-new", Name: "ГАВО, архив"}}

	res := callArchiveTool(t, archiveCreateHandler(svc), map[string]any{
		"name":          "ГАВО, архив",
		"repository_id": "R-1",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.Name != "ГАВО, архив" || svc.gotCreate.RepositoryID != "R-1" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestArchiveCreateToolRepositoryNotFoundIsError — usecase-слой возвращает
// ошибку на несуществующее хранилище, тул должен отдать её как ошибку тула
// (не паниковать, не проглатывать).
func TestArchiveCreateToolRepositoryNotFoundIsError(t *testing.T) {
	svc := &fakeArchives{err: &models.ValidationError{Entity: models.TypeArchive, Field: "repository_id", Reason: "не найдено"}}

	res := callArchiveTool(t, archiveCreateHandler(svc), map[string]any{
		"name":          "ГАВО, архив",
		"repository_id": "R-999",
	})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestArchiveDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeArchives{deleteErr: &models.InUseError{Type: models.TypeArchive, ID: "AR-1"}}

	res := callArchiveTool(t, archiveDeleteHandler(svc), map[string]any{"id": "AR-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersArchiveTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Archives: &fakeArchives{}}).ListTools()

	for _, name := range []string{"archive_list", "archive_search", "archive_get", "archive_create", "archive_update", "archive_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
```

### Шаг 1.8. `internal/mcp/object_args.go` (создать)

Общие хелперы для объектных MCP-аргументов (`*TextRef` и `FactDate` как `mcp.WithObject`) — используются во всех четырёх сущностях этого прохода (и далее, где встретятся такие же поля).
`internal/mcp/object_args.go`:
```go
package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// textRefObjectProperties — JSON-schema свойств объектного аргумента вида
// TextRef ({text, ref?, type?}) — используется в mcp.WithObject для полей
// вроде Church.Parish/Parish.Church/Archive.System. Появляется впервые в этом
// проходе (первые сущности с одиночным *TextRef, не списком).
func textRefObjectProperties() map[string]any {
	return map[string]any{
		"text": map[string]any{"type": "string", "description": "Текст (обязателен, если нет ссылки)"},
		"ref":  map[string]any{"type": "string", "description": "id сущности-ссылки (только чтение — задаётся не через этот тул)"},
		"type": map[string]any{"type": "string", "description": "тип сущности-ссылки (только чтение)"},
	}
}

// factDateObjectProperties — JSON-schema свойств объектного аргумента вида
// FactDate (структурированная дата с точностью). Появляется впервые в этом
// проходе.
func factDateObjectProperties() map[string]any {
	return map[string]any{
		"year":      map[string]any{"type": "integer", "description": "Год (1-9999), обязателен, если precision не unknown"},
		"month":     map[string]any{"type": "integer", "description": "Месяц (1-12), нужен при precision=month/day"},
		"day":       map[string]any{"type": "integer", "description": "День, нужен при precision=day"},
		"precision": map[string]any{"type": "string", "enum": []string{"unknown", "year", "month", "day"}, "description": "Верхняя известная точность"},
		"modifier":  map[string]any{"type": "string", "enum": []string{"exact", "approx", "before", "after", "between"}, "description": "Формулировка: точно/около/до/после/между"},
		"calendar":  map[string]any{"type": "string", "enum": []string{"", "gregorian", "julian", "unknown"}, "description": "Календарь; пусто — не указан"},
		"year_to":   map[string]any{"type": "integer", "description": "Год верхней границы, только при modifier=between"},
		"month_to":  map[string]any{"type": "integer", "description": "Месяц верхней границы"},
		"day_to":    map[string]any{"type": "integer", "description": "День верхней границы"},
	}
}

// optionalTextRef читает необязательный объектный аргумент {text, ref?, type?}
// (см. transport.TextRef) из сырых аргументов тула и конвертирует его в
// модель; отсутствующий или null аргумент — nil, без ошибки.
func optionalTextRef(args map[string]any, name string) (*models.TextRef, error) {
	raw, ok := args[name]
	if !ok || raw == nil {
		return nil, nil
	}

	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	var t transport.TextRef
	if err := json.Unmarshal(b, &t); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	m := t.Model()

	return &m, nil
}

// optionalFactDate читает необязательный объектный аргумент (структурированная
// дата, см. transport.FactDate) из сырых аргументов тула и конвертирует его в
// модель; отсутствующий или null аргумент — nil, без ошибки.
func optionalFactDate(args map[string]any, name string) (*models.FactDate, error) {
	raw, ok := args[name]
	if !ok || raw == nil {
		return nil, nil
	}

	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	var d transport.FactDate
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	return d.Model(), nil
}
```

### Шаг 1.9. `Deps`-реестр: подключить все 4 сервиса

По образцу `Repositories`/`Churches`/`Parishes`/`Archives` — гвард `if deps.X != nil { register...(...) }`. Ниже — итоговое содержимое каждого изменённого файла целиком.

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

// RepositoryService — контракт сценариев хранилищ-контейнеров источников,
// отдаваемых в HTTP: список, поиск, чтение, создание, изменение, удаление.
type RepositoryService interface {
	ListRepositories(ctx context.Context, access models.Access, page models.Page) ([]models.Repository, error)
	SearchRepositories(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Repository, error)
	GetRepository(ctx context.Context, id models.ID) (models.Repository, error)
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
	GetArchive(ctx context.Context, id models.ID) (models.Archive, error)
	CreateArchive(ctx context.Context, a models.Archive) (models.Archive, error)
	UpdateArchive(ctx context.Context, a models.Archive) error
	DeleteArchive(ctx context.Context, id models.ID) error
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

// RepositoryService — контракт сценариев хранилищ-контейнеров источников,
// отдаваемых в MCP-тулы: список, поиск, чтение, создание, изменение, удаление.
type RepositoryService interface {
	ListRepositories(ctx context.Context, access models.Access, page models.Page) ([]models.Repository, error)
	SearchRepositories(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Repository, error)
	GetRepository(ctx context.Context, id models.ID) (models.Repository, error)
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
	GetArchive(ctx context.Context, id models.ID) (models.Archive, error)
	CreateArchive(ctx context.Context, a models.Archive) (models.Archive, error)
	UpdateArchive(ctx context.Context, a models.Archive) error
	DeleteArchive(ctx context.Context, id models.ID) error
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
	create_archive "github.com/amarin/genodex/internal/usecases/create_archive"
	create_church "github.com/amarin/genodex/internal/usecases/create_church"
	create_division "github.com/amarin/genodex/internal/usecases/create_division"
	create_estate "github.com/amarin/genodex/internal/usecases/create_estate"
	create_given_name "github.com/amarin/genodex/internal/usecases/create_given_name"
	create_parish "github.com/amarin/genodex/internal/usecases/create_parish"
	create_patronymic "github.com/amarin/genodex/internal/usecases/create_patronymic"
	create_repository "github.com/amarin/genodex/internal/usecases/create_repository"
	create_surname "github.com/amarin/genodex/internal/usecases/create_surname"
	create_title "github.com/amarin/genodex/internal/usecases/create_title"
	delete_archive "github.com/amarin/genodex/internal/usecases/delete_archive"
	delete_church "github.com/amarin/genodex/internal/usecases/delete_church"
	delete_division "github.com/amarin/genodex/internal/usecases/delete_division"
	delete_estate "github.com/amarin/genodex/internal/usecases/delete_estate"
	delete_given_name "github.com/amarin/genodex/internal/usecases/delete_given_name"
	delete_parish "github.com/amarin/genodex/internal/usecases/delete_parish"
	delete_patronymic "github.com/amarin/genodex/internal/usecases/delete_patronymic"
	delete_repository "github.com/amarin/genodex/internal/usecases/delete_repository"
	delete_surname "github.com/amarin/genodex/internal/usecases/delete_surname"
	delete_title "github.com/amarin/genodex/internal/usecases/delete_title"
	get_archive "github.com/amarin/genodex/internal/usecases/get_archive"
	get_church "github.com/amarin/genodex/internal/usecases/get_church"
	get_division "github.com/amarin/genodex/internal/usecases/get_division"
	get_estate "github.com/amarin/genodex/internal/usecases/get_estate"
	get_given_name "github.com/amarin/genodex/internal/usecases/get_given_name"
	get_parish "github.com/amarin/genodex/internal/usecases/get_parish"
	get_patronymic "github.com/amarin/genodex/internal/usecases/get_patronymic"
	get_repository "github.com/amarin/genodex/internal/usecases/get_repository"
	get_surname "github.com/amarin/genodex/internal/usecases/get_surname"
	get_title "github.com/amarin/genodex/internal/usecases/get_title"
	list_archives "github.com/amarin/genodex/internal/usecases/list_archives"
	list_churches "github.com/amarin/genodex/internal/usecases/list_churches"
	list_divisions "github.com/amarin/genodex/internal/usecases/list_divisions"
	list_estates "github.com/amarin/genodex/internal/usecases/list_estates"
	list_given_names "github.com/amarin/genodex/internal/usecases/list_given_names"
	list_parishes "github.com/amarin/genodex/internal/usecases/list_parishes"
	list_patronymics "github.com/amarin/genodex/internal/usecases/list_patronymics"
	list_repositories "github.com/amarin/genodex/internal/usecases/list_repositories"
	list_surnames "github.com/amarin/genodex/internal/usecases/list_surnames"
	list_titles "github.com/amarin/genodex/internal/usecases/list_titles"
	search_archives "github.com/amarin/genodex/internal/usecases/search_archives"
	search_churches "github.com/amarin/genodex/internal/usecases/search_churches"
	search_divisions "github.com/amarin/genodex/internal/usecases/search_divisions"
	search_estates "github.com/amarin/genodex/internal/usecases/search_estates"
	search_given_names "github.com/amarin/genodex/internal/usecases/search_given_names"
	search_parishes "github.com/amarin/genodex/internal/usecases/search_parishes"
	search_patronymics "github.com/amarin/genodex/internal/usecases/search_patronymics"
	search_repositories "github.com/amarin/genodex/internal/usecases/search_repositories"
	search_surnames "github.com/amarin/genodex/internal/usecases/search_surnames"
	search_titles "github.com/amarin/genodex/internal/usecases/search_titles"
	update_archive "github.com/amarin/genodex/internal/usecases/update_archive"
	update_church "github.com/amarin/genodex/internal/usecases/update_church"
	update_division "github.com/amarin/genodex/internal/usecases/update_division"
	update_estate "github.com/amarin/genodex/internal/usecases/update_estate"
	update_given_name "github.com/amarin/genodex/internal/usecases/update_given_name"
	update_parish "github.com/amarin/genodex/internal/usecases/update_parish"
	update_patronymic "github.com/amarin/genodex/internal/usecases/update_patronymic"
	update_repository "github.com/amarin/genodex/internal/usecases/update_repository"
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

func (s *repositoryService) GetRepository(ctx context.Context, id models.ID) (models.Repository, error) {
	return s.get.GetRepository(ctx, id)
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

func (s *archiveService) GetArchive(ctx context.Context, id models.ID) (models.Archive, error) {
	return s.get.GetArchive(ctx, id)
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
	_ httpapi.GivenNameService  = (*givenNameService)(nil)
	_ mcp.GivenNameService      = (*givenNameService)(nil)
	_ httpapi.RepositoryService = (*repositoryService)(nil)
	_ mcp.RepositoryService     = (*repositoryService)(nil)
	_ httpapi.ChurchService     = (*churchService)(nil)
	_ mcp.ChurchService         = (*churchService)(nil)
	_ httpapi.ParishService     = (*parishService)(nil)
	_ mcp.ParishService         = (*parishService)(nil)
	_ httpapi.ArchiveService    = (*archiveService)(nil)
	_ mcp.ArchiveService        = (*archiveService)(nil)
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

	// auth-хранилище — на том же соединении, что и общий store (см.
	// sqlstore.Store.DB), файл БД один и тот же (internal/storage/schema_auth.go).
	authService := auth.New(auth.NewSQLStore(st.DB()))

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcp.RequireAPIToken(authService)(server.NewStreamableHTTPServer(
		mcp.NewServer(mcp.Deps{
			Divisions: divisions, Surnames: surnames,
			Patronymics: patronymics, Estates: estates, Titles: titles, GivenNames: givenNames,
			Repositories: repositories, Churches: churches, Parishes: parishes, Archives: archives,
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

### Шаг 1.10. Рубеж

`gofmt -l .` пусто; `go build ./...`, `go vet ./...`, `go test ./...` (весь репозиторий) зелёные — ожидается 894 теста, 76 пакетов. Живой смок через `curl` (см. «Предпосылка») — не обязателен повторно, но приветствуется: создать Repository/Church/Parish/Archive, проверить 422 на несуществующий repository_id и 409 на удаление используемого Repository.

### Шаг 1.11. Коммит

```bash
git add \
  internal/transport/text_ref.go internal/transport/fact_date.go internal/transport/source_link.go \
  internal/transport/repository.go internal/transport/repository_write.go \
  internal/transport/church.go internal/transport/church_write.go \
  internal/transport/parish.go internal/transport/parish_write.go \
  internal/transport/archive.go internal/transport/archive_write.go \
  internal/usecases/list_repositories internal/usecases/search_repositories internal/usecases/get_repository internal/usecases/create_repository internal/usecases/update_repository internal/usecases/delete_repository \
  internal/usecases/list_churches internal/usecases/search_churches internal/usecases/get_church internal/usecases/create_church internal/usecases/update_church internal/usecases/delete_church \
  internal/usecases/list_parishes internal/usecases/search_parishes internal/usecases/get_parish internal/usecases/create_parish internal/usecases/update_parish internal/usecases/delete_parish \
  internal/usecases/list_archives internal/usecases/search_archives internal/usecases/get_archive internal/usecases/create_archive internal/usecases/update_archive internal/usecases/delete_archive \
  internal/httpapi/repository.go internal/httpapi/repository_write.go internal/httpapi/repository_test.go internal/httpapi/repository_write_test.go \
  internal/httpapi/church.go internal/httpapi/church_write.go internal/httpapi/church_test.go internal/httpapi/church_write_test.go \
  internal/httpapi/parish.go internal/httpapi/parish_write.go internal/httpapi/parish_test.go internal/httpapi/parish_write_test.go \
  internal/httpapi/archive.go internal/httpapi/archive_write.go internal/httpapi/archive_test.go internal/httpapi/archive_write_test.go \
  internal/mcp/repository.go internal/mcp/repository_test.go internal/mcp/church.go internal/mcp/church_test.go internal/mcp/parish.go internal/mcp/parish_test.go internal/mcp/archive.go internal/mcp/archive_test.go internal/mcp/object_args.go \
  internal/httpapi/deps.go internal/httpapi/api.go internal/httpapi/httpapi.go internal/mcp/deps.go internal/mcp/server.go internal/app/app.go
git commit -m "feat(backend): Repository/Church/Parish/Archive — полный CRUD (usecases/httpapi/mcp) + Deps-реестр"
```
## Задача 2. Веб: `FactDateEditor` + страницы Repository/Church

**Интерфейсы, потребляемые из Задачи 1**: `httpapi`-маршруты `/api/repositories`/`/api/churches` (JSON-форма — `transport.{Repository,Church}`/`{...}Create`/`{...}Update`).
**Интерфейсы, потребляемые из подпроектов 1-2**: `authFetch`/`ApiError` (`web/src/auth.ts`), `TextRefListEditor` (`web/src/TextRefList.tsx`), `PageLayout`/каталог сущностей, `MAX_PAGE_LIMIT` (`web/src/api.ts`).

**Файлы:**
- Создать: `web/src/FactDateEditor.tsx` (общий компонент — Repository/Church его не используют, но заводится в этой задаче как инфраструктура для Задачи 3, чтобы не откладывать), `web/src/pages/{RepositoriesList,RepositoryForm,RepositoryView}.tsx`, `web/src/pages/{ChurchesList,ChurchForm,ChurchView}.tsx`
- Изменить: `web/src/api.ts`, `web/src/App.tsx`, `web/src/pages/EntityCatalog.tsx`

**Важно для исполнителя**: `Church.Parish` — одиночная необязательная ссылка (`*TextRef`, не список) — в форме это обычный текстовый `Input`, не `TextRefListEditor`. `Church.Variants` — простые строки (`string[]`, не `TextRef[]`) — редактируется через `TextRefListEditor`, но на сохранении берётся только `.text` каждого элемента (см. код `ChurchForm.tsx`/`ChurchView.tsx` — `variants.map((v) => v.text)`). `Sources` (доказательства) — read-only список, без формы создания: Citation ещё не имеет CRUD (подпроект 5).

### Шаг 2.1. `web/src/FactDateEditor.tsx` (создать)
`web/src/FactDateEditor.tsx`:
```tsx
import { Button, InputNumber, Select, Space, Typography } from "antd";
import { MinusCircleOutlined, PlusOutlined } from "@ant-design/icons";
import type { FactCalendar, FactDate, FactModifier, FactPrecision } from "./api";

const PRECISION_OPTIONS: { value: FactPrecision; label: string }[] = [
  { value: "unknown", label: "Неизвестна" },
  { value: "year", label: "Год" },
  { value: "month", label: "Месяц" },
  { value: "day", label: "День" },
];

const MODIFIER_OPTIONS: { value: FactModifier; label: string }[] = [
  { value: "exact", label: "Точно" },
  { value: "approx", label: "Около" },
  { value: "before", label: "До" },
  { value: "after", label: "После" },
  { value: "between", label: "Между" },
];

const CALENDAR_OPTIONS: { value: FactCalendar; label: string }[] = [
  { value: "", label: "Не указан" },
  { value: "gregorian", label: "Григорианский" },
  { value: "julian", label: "Юлианский" },
  { value: "unknown", label: "Неизвестен" },
];

const EMPTY_DATE: FactDate = { year: 0, precision: "year", modifier: "exact" };

const CALENDAR_SUFFIX: Record<string, string> = { julian: " ст. ст.", gregorian: " н. ст." };

// formatFactDate — читаемое текстовое представление даты для режима
// просмотра (аналог models.FactDate.String() на бэкенде, упрощённый —
// только для отображения, не для парсинга обратно).
export function formatFactDate(d: FactDate | null | undefined): string {
  if (d == null || d.precision === "unknown") {
    return "—";
  }

  const pad = (n: number) => String(n).padStart(2, "0");
  let base = String(d.year);
  if (d.precision === "month" || d.precision === "day") {
    base += `-${pad(d.month ?? 0)}`;
  }
  if (d.precision === "day") {
    base += `-${pad(d.day ?? 0)}`;
  }

  let text: string;
  switch (d.modifier) {
    case "approx":
      text = `около ${base}`;
      break;
    case "before":
      text = `до ${base}`;
      break;
    case "after":
      text = `после ${base}`;
      break;
    case "between": {
      let hi = String(d.year_to ?? 0);
      if (d.month_to) {
        hi += `-${pad(d.month_to)}`;
      }
      if (d.day_to) {
        hi += `-${pad(d.day_to)}`;
      }
      text = `между ${base} и ${hi}`;
      break;
    }
    default:
      text = base;
  }

  return text + (CALENDAR_SUFFIX[d.calendar ?? ""] ?? "");
}

// FactDateEditor — редактор структурированной даты (models.FactDate): год/
// месяц/день по точности, формулировка (точно/около/до/после/между),
// календарь, верхняя граница периода для modifier=between. Первое появление
// в проекте (Parish.Since/Until) — переиспользуется в Family/Person/Event
// (docs/data-model/entity-write.md §2, подпроекты 7-9). Верхняя граница
// («До:») использует ту же точность, что и нижняя — упрощение v1: модель
// допускает разную точность, но для одной формы это не нужно.
export function FactDateEditor({
  value,
  onChange,
  addLabel,
}: {
  value: FactDate | null;
  onChange: (next: FactDate | null) => void;
  addLabel: string;
}) {
  if (value == null) {
    return (
      <Button type="dashed" onClick={() => onChange(EMPTY_DATE)} icon={<PlusOutlined />} block>
        {addLabel}
      </Button>
    );
  }

  const set = (patch: Partial<FactDate>) => onChange({ ...value, ...patch });

  const setPrecision = (precision: FactPrecision) => {
    const patch: Partial<FactDate> = { precision };
    if (precision === "unknown") {
      patch.year = 0;
      patch.month = 0;
      patch.day = 0;
    } else if (precision === "year") {
      patch.month = 0;
      patch.day = 0;
    } else if (precision === "month") {
      patch.day = 0;
    }
    set(patch);
  };

  const setModifier = (modifier: FactModifier) => {
    if (modifier === "between") {
      set({ modifier });
    } else {
      set({ modifier, year_to: 0, month_to: 0, day_to: 0 });
    }
  };

  const needMonth = value.precision === "month" || value.precision === "day";
  const needDay = value.precision === "day";
  const isBetween = value.modifier === "between";

  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      <Space wrap>
        <Select
          style={{ width: 140 }}
          value={value.precision}
          options={PRECISION_OPTIONS}
          onChange={setPrecision}
        />
        {value.precision !== "unknown" && (
          <>
            <InputNumber
              placeholder="Год"
              min={1}
              max={9999}
              value={value.year || undefined}
              onChange={(v) => set({ year: v ?? 0 })}
              style={{ width: 90 }}
            />
            {needMonth && (
              <InputNumber
                placeholder="Месяц"
                min={1}
                max={12}
                value={value.month || undefined}
                onChange={(v) => set({ month: v ?? 0 })}
                style={{ width: 80 }}
              />
            )}
            {needDay && (
              <InputNumber
                placeholder="День"
                min={1}
                max={31}
                value={value.day || undefined}
                onChange={(v) => set({ day: v ?? 0 })}
                style={{ width: 80 }}
              />
            )}
          </>
        )}
        <Select
          style={{ width: 120 }}
          value={value.modifier}
          options={MODIFIER_OPTIONS}
          disabled={value.precision === "unknown"}
          onChange={setModifier}
        />
        <Select
          style={{ width: 150 }}
          value={value.calendar ?? ""}
          options={CALENDAR_OPTIONS}
          onChange={(calendar: FactCalendar) => set({ calendar })}
        />
        <MinusCircleOutlined onClick={() => onChange(null)} />
      </Space>
      {isBetween && (
        <Space wrap>
          <Typography.Text type="secondary">До:</Typography.Text>
          <InputNumber
            placeholder="Год"
            min={1}
            max={9999}
            value={value.year_to || undefined}
            onChange={(v) => set({ year_to: v ?? 0 })}
            style={{ width: 90 }}
          />
          {needMonth && (
            <InputNumber
              placeholder="Месяц"
              min={1}
              max={12}
              value={value.month_to || undefined}
              onChange={(v) => set({ month_to: v ?? 0 })}
              style={{ width: 80 }}
            />
          )}
          {needDay && (
            <InputNumber
              placeholder="День"
              min={1}
              max={31}
              value={value.day_to || undefined}
              onChange={(v) => set({ day_to: v ?? 0 })}
              style={{ width: 80 }}
            />
          )}
        </Space>
      )}
    </Space>
  );
}
```

### Шаг 2.2. `web/src/api.ts` — типы и функции SourceLink/FactDate/Repository/Church

Добавить в самый конец файла (после существующего блока `deleteGivenName`):
```typescript
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
// полей, кроме sources — read-only в v1).
export interface RepositoryInput {
  name: string;
  type: string;
  address?: string;
  urls: TextRef[];
  notes: TextRef[];
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
```

### Шаг 2.3. Repository — страницы `List`/`Form`/`View`

#### `web/src/pages/RepositoriesList.tsx` (создать)
`web/src/pages/RepositoriesList.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Input, List, Spin } from "antd";
import { fetchRepositories, searchRepositories, MAX_PAGE_LIMIT, type Repository } from "../api";
import { useSession } from "../session";
import { CreateRepositoryModal } from "./RepositoryForm";

// RepositoriesList — «Хранилища»: плоский список (сущность не иерархична, в
// отличие от AdminDivision — без Tree), пагинация по offset до короткой
// страницы (тот же приём, что DivisionsList.loadRoot — API уже отдаёт
// окнами, docs/data-model/entity-write.md §4). Поиск по названию временно
// подменяет список найденным. Клик по строке — переход на View.
export default function RepositoriesList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<Repository[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<Repository[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: Repository[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchRepositories({ limit: MAX_PAGE_LIMIT, offset });
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
    searchRepositories({ q, limit: MAX_PAGE_LIMIT })
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
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "Хранилища" }]}
      />
      <Card
        title="Хранилища"
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
                <Link to={`/repositories/${s.id}`}>{s.name}</Link>
              </List.Item>
            )}
          />
        )}
        <CreateRepositoryModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(s) => {
            setCreateOpen(false);
            navigate(`/repositories/${s.id}`);
          }}
        />
      </Card>
    </>
  );
}
```

#### `web/src/pages/RepositoryForm.tsx` (создать)
`web/src/pages/RepositoryForm.tsx`:
```tsx
import { useState } from "react";
import { Alert, Checkbox, Form, Input, Modal } from "antd";
import { createRepository, type Repository, type TextRef } from "../api";
import { ApiError } from "../auth";
import { TextRefListEditor } from "../TextRefList";

interface RepositoryFormValues {
  name: string;
  type: string;
  address?: string;
  private?: boolean;
}

const FORM_FIELDS: (keyof RepositoryFormValues)[] = ["name", "type", "address"];

// CreateRepositoryModal — форма создания хранилища-контейнера источников.
// type — открытый список (любая строка формата [a-z][a-z0-9_-]*, а не
// фиксированный enum вроде AdminDivisionType/Gender) — обычный текстовый
// инпут, не Select (docs/data-model/entity-write.md, models/repository_type.go
// перечисляет типовые значения archive/library/museum/private/other как
// подсказку, но допустимы и другие). urls/notes редактируются вне antd Form
// (TextRefListEditor). sources не редактируется — read-only в v1 (Citation
// ещё не имеет CRUD).
export function CreateRepositoryModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Repository) => void;
}) {
  const [form] = Form.useForm<RepositoryFormValues>();
  const [urls, setUrls] = useState<TextRef[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setUrls([]);
    setNotes([]);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: RepositoryFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await createRepository({
        name: values.name,
        type: values.type,
        address: values.address ?? "",
        urls,
        notes,
        private: values.private ?? false,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof RepositoryFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить хранилище"
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
        <Form.Item
          name="type"
          label="Тип"
          rules={[{ required: true, whitespace: true, message: "Введите тип (archive/library/museum/private/other или другой)" }]}
        >
          <Input placeholder="archive" />
        </Form.Item>
        <Form.Item name="address" label="Адрес">
          <Input />
        </Form.Item>
        <Form.Item label="Ссылки">
          <TextRefListEditor value={urls} onChange={setUrls} addLabel="+ ссылка" />
        </Form.Item>
        <Form.Item label="Заметки">
          <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
        </Form.Item>
        <Form.Item name="private" valuePropName="checked">
          <Checkbox>Приватная запись</Checkbox>
        </Form.Item>
      </Form>
    </Modal>
  );
}
```

#### `web/src/pages/RepositoryView.tsx` (создать)
`web/src/pages/RepositoryView.tsx`:
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
  deleteRepository,
  fetchRepository,
  updateRepository,
  type Repository,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { TextRefListEditor } from "../TextRefList";

interface EditFormValues {
  name: string;
  type: string;
  address?: string;
  private?: boolean;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["name", "type", "address"];

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

// SourceLinkListView — read-only список доказательств (Sources): Citation
// ещё не имеет CRUD (docs/data-model/entity-write.md §2, подпроект 5), поэтому
// здесь только чтение, без формы создания/редактирования.
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
          citation {s.citation_id}
          {s.role ? ` — ${s.role}` : ""}
        </List.Item>
      )}
    />
  );
}

// RepositoryView — просмотр хранилища, переключаемый в форму редактирования
// на той же странице (toggle+explicit-save — PUT заменяет запись целиком,
// docs/data-model/entity-write.md §4). Без родителя/детей/дерева.
export default function RepositoryView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [repository, setRepository] = useState<Repository | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [urls, setUrls] = useState<TextRef[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (repositoryId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setRepository(null);
    fetchRepository(repositoryId)
      .then(setRepository)
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
    if (repository == null) {
      return;
    }
    form.setFieldsValue({
      name: repository.name,
      type: repository.type,
      address: repository.address,
      private: repository.private,
    });
    setUrls(repository.urls);
    setNotes(repository.notes);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (repository == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await updateRepository(repository.id, {
        name: values.name,
        type: values.type,
        address: values.address ?? "",
        urls,
        notes,
        private: values.private ?? false,
      });
      setRepository(updated);
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
    if (repository == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteRepository(repository.id);
      navigate("/repositories");
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
          <Link to="/repositories">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (repository == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/repositories">Хранилища</Link> },
          { title: repository.name },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={repository.name} column={1} bordered size="small">
            <Descriptions.Item label="Тип">{repository.type}</Descriptions.Item>
            <Descriptions.Item label="Адрес">{repository.address || "—"}</Descriptions.Item>
            <Descriptions.Item label="Ссылки"><TextRefListView items={repository.urls} /></Descriptions.Item>
            <Descriptions.Item label="Заметки"><TextRefListView items={repository.notes} /></Descriptions.Item>
            <Descriptions.Item label="Доказательства"><SourceLinkListView items={repository.sources} /></Descriptions.Item>
            <Descriptions.Item label="Приватная">{repository.private ? "да" : "нет"}</Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title={`Удалить «${repository.name}»?`}
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
          <Form.Item
            name="type"
            label="Тип"
            rules={[{ required: true, whitespace: true, message: "Введите тип" }]}
          >
            <Input placeholder="archive" />
          </Form.Item>
          <Form.Item name="address" label="Адрес">
            <Input />
          </Form.Item>
          <Form.Item label="Ссылки">
            <TextRefListEditor value={urls} onChange={setUrls} addLabel="+ ссылка" />
          </Form.Item>
          <Form.Item label="Заметки">
            <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
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

### Шаг 2.4. Church — страницы `List`/`Form`/`View`

#### `web/src/pages/ChurchesList.tsx` (создать)
`web/src/pages/ChurchesList.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Input, List, Spin } from "antd";
import { fetchChurches, searchChurches, MAX_PAGE_LIMIT, type Church } from "../api";
import { useSession } from "../session";
import { CreateChurchModal } from "./ChurchForm";

// ChurchesList — «Церкви»: плоский список (сущность не иерархична, в
// отличие от AdminDivision — без Tree), пагинация по offset до короткой
// страницы (тот же приём, что DivisionsList.loadRoot — API уже отдаёт
// окнами, docs/data-model/entity-write.md §4). Поиск по названию временно
// подменяет список найденным. Клик по строке — переход на View.
export default function ChurchesList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<Church[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<Church[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: Church[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchChurches({ limit: MAX_PAGE_LIMIT, offset });
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
    searchChurches({ q, limit: MAX_PAGE_LIMIT })
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
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "Церкви" }]}
      />
      <Card
        title="Церкви"
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
                <Link to={`/churches/${s.id}`}>{s.name}</Link>
              </List.Item>
            )}
          />
        )}
        <CreateChurchModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(s) => {
            setCreateOpen(false);
            navigate(`/churches/${s.id}`);
          }}
        />
      </Card>
    </>
  );
}
```

#### `web/src/pages/ChurchForm.tsx` (создать)
`web/src/pages/ChurchForm.tsx`:
```tsx
import { useState } from "react";
import { Alert, Form, Input, Modal } from "antd";
import { createChurch, type Church, type TextRef } from "../api";
import { ApiError } from "../auth";
import { TextRefListEditor } from "../TextRefList";

interface ChurchFormValues {
  name: string;
  parishText?: string;
}

const FORM_FIELDS: (keyof ChurchFormValues)[] = ["name"];

// CreateChurchModal — форма создания церкви. parish — одиночная необязательная
// ссылка (text-only в v1, как элементы TextRef-списков — обычный текстовый
// инпут вместо TextRefListEditor, т.к. поле одно, не список). settlements —
// TextRef-список (населённые пункты, ссылки на AdministrativeDivision — v1
// текстом). variants — простые строки (не TextRef), свой список без
// возможности нести ссылку.
export function CreateChurchModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Church) => void;
}) {
  const [form] = Form.useForm<ChurchFormValues>();
  const [settlements, setSettlements] = useState<TextRef[]>([]);
  const [variants, setVariants] = useState<TextRef[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setSettlements([]);
    setVariants([]);
    setNotes([]);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: ChurchFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const parishText = (values.parishText ?? "").trim();
      const created = await createChurch({
        name: values.name,
        parish: parishText ? { text: parishText } : null,
        settlements,
        variants: variants.map((v) => v.text).filter((t) => t.trim() !== ""),
        notes,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof ChurchFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить церковь"
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
          name="name"
          label="Название"
          rules={[{ required: true, whitespace: true, message: "Введите название" }]}
        >
          <Input autoFocus />
        </Form.Item>
        <Form.Item name="parishText" label="Приход (текстом)">
          <Input placeholder="Никольский приход" />
        </Form.Item>
        <Form.Item label="Населённые пункты">
          <TextRefListEditor value={settlements} onChange={setSettlements} addLabel="+ населённый пункт" />
        </Form.Item>
        <Form.Item label="Варианты названия">
          <TextRefListEditor value={variants} onChange={setVariants} addLabel="+ вариант" />
        </Form.Item>
        <Form.Item label="Заметки">
          <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
        </Form.Item>
      </Form>
    </Modal>
  );
}
```

#### `web/src/pages/ChurchView.tsx` (создать)
`web/src/pages/ChurchView.tsx`:
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
import {
  deleteChurch,
  fetchChurch,
  updateChurch,
  type Church,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { TextRefListEditor } from "../TextRefList";

interface EditFormValues {
  name: string;
  parishText?: string;
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
          citation {s.citation_id}
          {s.role ? ` — ${s.role}` : ""}
        </List.Item>
      )}
    />
  );
}

// ChurchView — просмотр церкви, переключаемый в форму редактирования на той
// же странице (toggle+explicit-save). parish показывается текстом; если у
// него уже есть ref — вторичная пометка «→ Type ID» (не кликабельно, picker
// не реализован, docs/data-model/entity-write.md §4-5).
export default function ChurchView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [church, setChurch] = useState<Church | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [settlements, setSettlements] = useState<TextRef[]>([]);
  const [variants, setVariants] = useState<TextRef[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (churchId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setChurch(null);
    fetchChurch(churchId)
      .then(setChurch)
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
    if (church == null) {
      return;
    }
    form.setFieldsValue({ name: church.name, parishText: church.parish?.text ?? "" });
    setSettlements(church.settlements);
    setVariants(church.variants.map((v) => ({ text: v })));
    setNotes(church.notes);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (church == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const parishText = (values.parishText ?? "").trim();
      const parishHasRef = church.parish?.ref != null && church.parish.ref !== "";
      const updated = await updateChurch(church.id, {
        name: values.name,
        // Сохраняем существующую ссылку, если поле не тронуто (тот же текст) и
        // уже несло ref; иначе — чистый текст без ref (picker не реализован).
        parish: parishHasRef && parishText === church.parish?.text
          ? church.parish
          : parishText
            ? { text: parishText }
            : null,
        settlements,
        variants: variants.map((v) => v.text).filter((t) => t.trim() !== ""),
        notes,
      });
      setChurch(updated);
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
    if (church == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteChurch(church.id);
      navigate("/churches");
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
          <Link to="/churches">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (church == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/churches">Церкви</Link> },
          { title: church.name },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={church.name} column={1} bordered size="small">
            <Descriptions.Item label="Приход">
              {church.parish == null ? (
                "—"
              ) : (
                <>
                  {church.parish.text}
                  {church.parish.ref && (
                    <Typography.Text type="secondary"> → {church.parish.type} {church.parish.ref}</Typography.Text>
                  )}
                </>
              )}
            </Descriptions.Item>
            <Descriptions.Item label="Населённые пункты"><TextRefListView items={church.settlements} /></Descriptions.Item>
            <Descriptions.Item label="Варианты названия">
              {church.variants.length === 0 ? "—" : church.variants.join(", ")}
            </Descriptions.Item>
            <Descriptions.Item label="Заметки"><TextRefListView items={church.notes} /></Descriptions.Item>
            <Descriptions.Item label="Доказательства"><SourceLinkListView items={church.sources} /></Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title={`Удалить «${church.name}»?`}
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
          <Form.Item name="parishText" label="Приход (текстом)">
            <Input placeholder="Никольский приход" />
          </Form.Item>
          <Form.Item label="Населённые пункты">
            <TextRefListEditor value={settlements} onChange={setSettlements} addLabel="+ населённый пункт" />
          </Form.Item>
          <Form.Item label="Варианты названия">
            <TextRefListEditor value={variants} onChange={setVariants} addLabel="+ вариант" />
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

### Шаг 2.5. `web/src/App.tsx` — роуты Repository/Church

Добавить импорты (после импортов GivenName из подпроекта 2):
```typescript
import RepositoriesList from "./pages/RepositoriesList";
import RepositoryView from "./pages/RepositoryView";
import ChurchesList from "./pages/ChurchesList";
import ChurchView from "./pages/ChurchView";
```
Добавить роуты (после роутов `/given-names`/`/given-names/:id`, до `/login`):
```typescript
          <Route path="/repositories" element={<PageLayout><RepositoriesList /></PageLayout>} />
          <Route path="/repositories/:id" element={<PageLayout><RepositoryView /></PageLayout>} />
          <Route path="/churches" element={<PageLayout><ChurchesList /></PageLayout>} />
          <Route path="/churches/:id" element={<PageLayout><ChurchView /></PageLayout>} />
```

### Шаг 2.6. `web/src/pages/EntityCatalog.tsx` — две новые строки каталога

В массиве `CATALOG_ENTRIES` добавить (порядок внутри массива не важен — сортировка `localeCompare('ru')` в рантайме):
```typescript
  { label: "Хранилища", path: "/repositories" },
  { label: "Церкви", path: "/churches" },
```

### Шаг 2.7. Рубеж

`npm run typecheck` и `npm run build` в `web/` чисты. Живая проверка в браузере (см. «Предпосылка»): `/repositories`, `/churches` — список, создание (включая поле «Тип» у Repository и «Приход (текстом)» у Church), просмотр, редактирование, удаление работают, хлебные крошки корректны, каталог на `/` показывает две новые строки.

### Шаг 2.8. Коммит

```bash
git add web/src/FactDateEditor.tsx web/src/api.ts web/src/App.tsx web/src/pages/EntityCatalog.tsx \
  web/src/pages/RepositoriesList.tsx web/src/pages/RepositoryForm.tsx web/src/pages/RepositoryView.tsx \
  web/src/pages/ChurchesList.tsx web/src/pages/ChurchForm.tsx web/src/pages/ChurchView.tsx
git commit -m "feat(web): страницы Хранилищ/Церквей + FactDateEditor — список, просмотр/редактирование, форма"
```
## Задача 3. Веб: страницы Parish/Archive (FactDateEditor, Select хранилищ)

**Интерфейсы, потребляемые из Задачи 1**: `httpapi`-маршруты `/api/parishes`/`/api/archives`.
**Интерфейсы, потребляемые из Задачи 2**: `FactDateEditor` (`web/src/FactDateEditor.tsx`, включая `formatFactDate`), `fetchRepositories` (`web/src/api.ts`, добавлена в Задаче 2) — используется в `ArchiveForm.tsx`/`ArchiveView.tsx` для заполнения `Select` хранилищ.
**Интерфейсы, потребляемые из подпроектов 1-2**: `authFetch`/`ApiError`, `TextRefListEditor`, `PageLayout`/каталог, `MAX_PAGE_LIMIT`.

**Файлы:**
- Создать: `web/src/pages/{ParishesList,ParishForm,ParishView}.tsx`, `web/src/pages/{ArchivesList,ArchiveForm,ArchiveView}.tsx`
- Изменить: `web/src/api.ts`, `web/src/App.tsx`, `web/src/pages/EntityCatalog.tsx`

**Важно для исполнителя**: `Parish.Since`/`Until` — `FactDateEditor` (значение `FactDate | null`, `null` — кнопка «+ добавить дату», заполненное значение — полный редактор с точностью/годом-месяцем-днём/модификатором/календарём, доп. блок «До:» только при `modifier=between`). `Archive.RepositoryID` — просто строка id (не `TextRef`), заполняется через `Select` (не свободный текст — `fetchRepositories()` при монтировании формы/страницы, `showSearch` с фильтром по названию). `Archive.System` — текстовое поле, как `Church.Parish`/`Parish.Church`, но модель запрещает на нём ref/type (только имя системы) — веб-форма и так не даёт задать ref/type ни для одного текстового поля в v1, так что дополнительных ограничений в коде не требуется.

### Шаг 3.1. `web/src/api.ts` — типы и функции Parish/Archive

Добавить в самый конец файла (после блока Church из Задачи 2):
```typescript
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
```

### Шаг 3.2. Parish — страницы `List`/`Form`/`View`

#### `web/src/pages/ParishesList.tsx` (создать)
`web/src/pages/ParishesList.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Input, List, Spin } from "antd";
import { fetchParishes, searchParishes, MAX_PAGE_LIMIT, type Parish } from "../api";
import { useSession } from "../session";
import { CreateParishModal } from "./ParishForm";

// ParishesList — «Приходы»: плоский список (сущность не иерархична, в
// отличие от AdminDivision — без Tree), пагинация по offset до короткой
// страницы (тот же приём, что DivisionsList.loadRoot — API уже отдаёт
// окнами, docs/data-model/entity-write.md §4). Поиск по названию временно
// подменяет список найденным. Клик по строке — переход на View.
export default function ParishesList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<Parish[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<Parish[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: Parish[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchParishes({ limit: MAX_PAGE_LIMIT, offset });
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
    searchParishes({ q, limit: MAX_PAGE_LIMIT })
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
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "Приходы" }]}
      />
      <Card
        title="Приходы"
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
                <Link to={`/parishes/${s.id}`}>{s.name}</Link>
              </List.Item>
            )}
          />
        )}
        <CreateParishModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(s) => {
            setCreateOpen(false);
            navigate(`/parishes/${s.id}`);
          }}
        />
      </Card>
    </>
  );
}
```

#### `web/src/pages/ParishForm.tsx` (создать)
`web/src/pages/ParishForm.tsx`:
```tsx
import { useState } from "react";
import { Alert, Form, Input, Modal } from "antd";
import { createParish, type FactDate, type Parish, type TextRef } from "../api";
import { ApiError } from "../auth";
import { TextRefListEditor } from "../TextRefList";
import { FactDateEditor } from "../FactDateEditor";

interface ParishFormValues {
  name: string;
  churchText?: string;
}

const FORM_FIELDS: (keyof ParishFormValues)[] = ["name"];

// CreateParishModal — форма создания прихода. church — одиночная
// необязательная ссылка (text-only в v1). since/until — структурированная
// дата (FactDateEditor, первое появление в проекте).
export function CreateParishModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Parish) => void;
}) {
  const [form] = Form.useForm<ParishFormValues>();
  const [settlements, setSettlements] = useState<TextRef[]>([]);
  const [since, setSince] = useState<FactDate | null>(null);
  const [until, setUntil] = useState<FactDate | null>(null);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setSettlements([]);
    setSince(null);
    setUntil(null);
    setNotes([]);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: ParishFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const churchText = (values.churchText ?? "").trim();
      const created = await createParish({
        name: values.name,
        church: churchText ? { text: churchText } : null,
        settlements,
        since,
        until,
        notes,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof ParishFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить приход"
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
          name="name"
          label="Название"
          rules={[{ required: true, whitespace: true, message: "Введите название" }]}
        >
          <Input autoFocus />
        </Form.Item>
        <Form.Item name="churchText" label="Церковь (текстом)">
          <Input placeholder="Никольская церковь" />
        </Form.Item>
        <Form.Item label="Населённые пункты">
          <TextRefListEditor value={settlements} onChange={setSettlements} addLabel="+ населённый пункт" />
        </Form.Item>
        <Form.Item label="Начало периода">
          <FactDateEditor value={since} onChange={setSince} addLabel="+ дата начала" />
        </Form.Item>
        <Form.Item label="Конец периода">
          <FactDateEditor value={until} onChange={setUntil} addLabel="+ дата конца" />
        </Form.Item>
        <Form.Item label="Заметки">
          <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
        </Form.Item>
      </Form>
    </Modal>
  );
}
```

#### `web/src/pages/ParishView.tsx` (создать)
`web/src/pages/ParishView.tsx`:
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
import {
  deleteParish,
  fetchParish,
  updateParish,
  type FactDate,
  type Parish,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { TextRefListEditor } from "../TextRefList";
import { FactDateEditor, formatFactDate } from "../FactDateEditor";

interface EditFormValues {
  name: string;
  churchText?: string;
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
          citation {s.citation_id}
          {s.role ? ` — ${s.role}` : ""}
        </List.Item>
      )}
    />
  );
}

// ParishView — просмотр прихода, переключаемый в форму редактирования на той
// же странице (toggle+explicit-save). since/until показываются
// отформатированным текстом (formatFactDate), в редактировании —
// FactDateEditor.
export default function ParishView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [parish, setParish] = useState<Parish | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [settlements, setSettlements] = useState<TextRef[]>([]);
  const [since, setSince] = useState<FactDate | null>(null);
  const [until, setUntil] = useState<FactDate | null>(null);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (parishId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setParish(null);
    fetchParish(parishId)
      .then(setParish)
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
    if (parish == null) {
      return;
    }
    form.setFieldsValue({ name: parish.name, churchText: parish.church?.text ?? "" });
    setSettlements(parish.settlements);
    setSince(parish.since ?? null);
    setUntil(parish.until ?? null);
    setNotes(parish.notes);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (parish == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const churchText = (values.churchText ?? "").trim();
      const churchHasRef = parish.church?.ref != null && parish.church.ref !== "";
      const updated = await updateParish(parish.id, {
        name: values.name,
        church: churchHasRef && churchText === parish.church?.text
          ? parish.church
          : churchText
            ? { text: churchText }
            : null,
        settlements,
        since,
        until,
        notes,
      });
      setParish(updated);
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
    if (parish == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteParish(parish.id);
      navigate("/parishes");
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
          <Link to="/parishes">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (parish == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/parishes">Приходы</Link> },
          { title: parish.name },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={parish.name} column={1} bordered size="small">
            <Descriptions.Item label="Церковь">
              {parish.church == null ? (
                "—"
              ) : (
                <>
                  {parish.church.text}
                  {parish.church.ref && (
                    <Typography.Text type="secondary"> → {parish.church.type} {parish.church.ref}</Typography.Text>
                  )}
                </>
              )}
            </Descriptions.Item>
            <Descriptions.Item label="Населённые пункты"><TextRefListView items={parish.settlements} /></Descriptions.Item>
            <Descriptions.Item label="Начало периода">{formatFactDate(parish.since)}</Descriptions.Item>
            <Descriptions.Item label="Конец периода">{formatFactDate(parish.until)}</Descriptions.Item>
            <Descriptions.Item label="Заметки"><TextRefListView items={parish.notes} /></Descriptions.Item>
            <Descriptions.Item label="Доказательства"><SourceLinkListView items={parish.sources} /></Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title={`Удалить «${parish.name}»?`}
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
        <Form form={form} layout="vertical" onFinish={onSave} style={{ maxWidth: 560 }}>
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
          <Form.Item name="churchText" label="Церковь (текстом)">
            <Input placeholder="Никольская церковь" />
          </Form.Item>
          <Form.Item label="Населённые пункты">
            <TextRefListEditor value={settlements} onChange={setSettlements} addLabel="+ населённый пункт" />
          </Form.Item>
          <Form.Item label="Начало периода">
            <FactDateEditor value={since} onChange={setSince} addLabel="+ дата начала" />
          </Form.Item>
          <Form.Item label="Конец периода">
            <FactDateEditor value={until} onChange={setUntil} addLabel="+ дата конца" />
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

### Шаг 3.3. Archive — страницы `List`/`Form`/`View`

#### `web/src/pages/ArchivesList.tsx` (создать)
`web/src/pages/ArchivesList.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Input, List, Spin } from "antd";
import { fetchArchives, searchArchives, MAX_PAGE_LIMIT, type Archive } from "../api";
import { useSession } from "../session";
import { CreateArchiveModal } from "./ArchiveForm";

// ArchivesList — «Архивы»: плоский список (сущность не иерархична, в
// отличие от AdminDivision — без Tree), пагинация по offset до короткой
// страницы (тот же приём, что DivisionsList.loadRoot — API уже отдаёт
// окнами, docs/data-model/entity-write.md §4). Поиск по названию временно
// подменяет список найденным. Клик по строке — переход на View.
export default function ArchivesList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<Archive[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<Archive[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: Archive[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchArchives({ limit: MAX_PAGE_LIMIT, offset });
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
    searchArchives({ q, limit: MAX_PAGE_LIMIT })
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
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "Архивы" }]}
      />
      <Card
        title="Архивы"
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
                <Link to={`/archives/${s.id}`}>{s.name}</Link>
              </List.Item>
            )}
          />
        )}
        <CreateArchiveModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(s) => {
            setCreateOpen(false);
            navigate(`/archives/${s.id}`);
          }}
        />
      </Card>
    </>
  );
}
```

#### `web/src/pages/ArchiveForm.tsx` (создать)
`web/src/pages/ArchiveForm.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Alert, Checkbox, Form, Input, Modal, Select } from "antd";
import { createArchive, fetchRepositories, type Archive, type Repository, type TextRef } from "../api";
import { ApiError } from "../auth";
import { TextRefListEditor } from "../TextRefList";

interface ArchiveFormValues {
  name: string;
  systemText?: string;
  repository_id?: string;
  private?: boolean;
}

const FORM_FIELDS: (keyof ArchiveFormValues)[] = ["name", "repository_id"];

// useRepositoryOptions — заполняет Select репозиториев для repository_id.
// Не общий picker (тот отложен до подпроекта 9) — точечный select именно для
// этой связи, раз у Repository уже есть свой (небольшой, целиком постраничный)
// список (docs/data-model/entity-write.md, обсуждение подпроекта 3).
function useRepositoryOptions() {
  const [repositories, setRepositories] = useState<Repository[]>([]);

  useEffect(() => {
    fetchRepositories({ limit: 500 })
      .then(setRepositories)
      .catch(() => setRepositories([]));
  }, []);

  return repositories.map((r) => ({ value: r.id, label: r.name }));
}

// CreateArchiveModal — форма создания архива. system — только текстом (ссылка
// на сущность не допускается, models.Archive.Validate). repository_id —
// Select со списком хранилищ, не TextRef (просто id, строгая ссылка).
export function CreateArchiveModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Archive) => void;
}) {
  const [form] = Form.useForm<ArchiveFormValues>();
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const repositoryOptions = useRepositoryOptions();

  const reset = () => {
    form.resetFields();
    setNotes([]);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: ArchiveFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const systemText = (values.systemText ?? "").trim();
      const created = await createArchive({
        name: values.name,
        system: systemText ? { text: systemText } : null,
        repository_id: values.repository_id ?? "",
        notes,
        private: values.private ?? false,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof ArchiveFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить архив"
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
        <Form.Item name="systemText" label="Система иерархии (текстом)">
          <Input placeholder="фонд-опись-дело" />
        </Form.Item>
        <Form.Item name="repository_id" label="Хранилище">
          <Select
            allowClear
            placeholder="Не выбрано"
            options={repositoryOptions}
            showSearch
            filterOption={(input, option) =>
              (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
            }
          />
        </Form.Item>
        <Form.Item label="Заметки">
          <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
        </Form.Item>
        <Form.Item name="private" valuePropName="checked">
          <Checkbox>Приватная запись</Checkbox>
        </Form.Item>
      </Form>
    </Modal>
  );
}
```

#### `web/src/pages/ArchiveView.tsx` (создать)
`web/src/pages/ArchiveView.tsx`:
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
  Select,
  Space,
  Spin,
  Typography,
} from "antd";
import {
  deleteArchive,
  fetchArchive,
  fetchRepositories,
  updateArchive,
  type Archive,
  type Repository,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { TextRefListEditor } from "../TextRefList";

interface EditFormValues {
  name: string;
  systemText?: string;
  repository_id?: string;
  private?: boolean;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["name", "repository_id"];

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
          citation {s.citation_id}
          {s.role ? ` — ${s.role}` : ""}
        </List.Item>
      )}
    />
  );
}

// ArchiveView — просмотр архива, переключаемый в форму редактирования на той
// же странице (toggle+explicit-save). repository_id — Select со списком
// хранилищ (fetchRepositories), не TextRef.
export default function ArchiveView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [archive, setArchive] = useState<Archive | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [repositories, setRepositories] = useState<Repository[]>([]);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (archiveId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setArchive(null);
    fetchArchive(archiveId)
      .then(setArchive)
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

  useEffect(() => {
    fetchRepositories({ limit: 500 })
      .then(setRepositories)
      .catch(() => setRepositories([]));
  }, []);

  const repositoryOptions = repositories.map((r) => ({ value: r.id, label: r.name }));
  const repositoryName = (repoID?: string) =>
    repositories.find((r) => r.id === repoID)?.name ?? repoID;

  const startEdit = () => {
    if (archive == null) {
      return;
    }
    form.setFieldsValue({
      name: archive.name,
      systemText: archive.system?.text ?? "",
      repository_id: archive.repository_id ?? undefined,
      private: archive.private,
    });
    setNotes(archive.notes);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (archive == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const systemText = (values.systemText ?? "").trim();
      const updated = await updateArchive(archive.id, {
        name: values.name,
        system: systemText ? { text: systemText } : null,
        repository_id: values.repository_id ?? "",
        notes,
        private: values.private ?? false,
      });
      setArchive(updated);
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
    if (archive == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteArchive(archive.id);
      navigate("/archives");
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
          <Link to="/archives">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (archive == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/archives">Архивы</Link> },
          { title: archive.name },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={archive.name} column={1} bordered size="small">
            <Descriptions.Item label="Система иерархии">{archive.system?.text || "—"}</Descriptions.Item>
            <Descriptions.Item label="Хранилище">
              {archive.repository_id ? (
                <Link to={`/repositories/${archive.repository_id}`}>{repositoryName(archive.repository_id)}</Link>
              ) : (
                "—"
              )}
            </Descriptions.Item>
            <Descriptions.Item label="Заметки"><TextRefListView items={archive.notes} /></Descriptions.Item>
            <Descriptions.Item label="Доказательства"><SourceLinkListView items={archive.sources} /></Descriptions.Item>
            <Descriptions.Item label="Приватная">{archive.private ? "да" : "нет"}</Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title={`Удалить «${archive.name}»?`}
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
          <Form.Item name="systemText" label="Система иерархии (текстом)">
            <Input placeholder="фонд-опись-дело" />
          </Form.Item>
          <Form.Item name="repository_id" label="Хранилище">
            <Select
              allowClear
              placeholder="Не выбрано"
              options={repositoryOptions}
              showSearch
              filterOption={(input, option) =>
                (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
              }
            />
          </Form.Item>
          <Form.Item label="Заметки">
            <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
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

### Шаг 3.4. `web/src/App.tsx` — роуты Parish/Archive

Добавить импорты (после импортов Church из Задачи 2):
```typescript
import ParishesList from "./pages/ParishesList";
import ParishView from "./pages/ParishView";
import ArchivesList from "./pages/ArchivesList";
import ArchiveView from "./pages/ArchiveView";
```
Добавить роуты (после роутов `/churches`/`/churches/:id`, до `/login`):
```typescript
          <Route path="/parishes" element={<PageLayout><ParishesList /></PageLayout>} />
          <Route path="/parishes/:id" element={<PageLayout><ParishView /></PageLayout>} />
          <Route path="/archives" element={<PageLayout><ArchivesList /></PageLayout>} />
          <Route path="/archives/:id" element={<PageLayout><ArchiveView /></PageLayout>} />
```

### Шаг 3.5. `web/src/pages/EntityCatalog.tsx` — две новые строки каталога

В массиве `CATALOG_ENTRIES` добавить:
```typescript
  { label: "Приходы", path: "/parishes" },
  { label: "Архивы", path: "/archives" },
```

Итоговый вид массива после Задач 2 и 3 (для сверки — 11 строк, порядок внутри массива не влияет на отображение):
`web/src/pages/EntityCatalog.tsx`:
```typescript
import { Card, List, Typography } from "antd";
import { Link } from "react-router-dom";

// CATALOG_ENTRIES — единая точка входа приложения: по алфавиту названия.
// Каждая сущность получает свой список при подключении (docs/data-model/
// entity-write.md §4) — здесь просто добавляется новая строка, без вкладок.
const CATALOG_ENTRIES: { label: string; path: string }[] = [
  { label: "Административное деление", path: "/divisions" },
  { label: "Архивы", path: "/archives" },
  { label: "Документация", path: "/docs" },
  { label: "Имена", path: "/given-names" },
  { label: "Отчества", path: "/patronymics" },
  { label: "Приходы", path: "/parishes" },
  { label: "Сословия", path: "/estates" },
  { label: "Титулы", path: "/titles" },
  { label: "Фамилии", path: "/surnames" },
  { label: "Хранилища", path: "/repositories" },
  { label: "Церкви", path: "/churches" },
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

### Шаг 3.6. Рубеж

`npm run typecheck` и `npm run build` в `web/` чисты. Живая проверка в браузере (см. «Предпосылка»): каталог на `/` показывает все 11 строк по алфавиту; `/parishes` → «+ добавить» → `FactDateEditor` для «Начало периода» — точность «Год», год 1880, модификатор «Между» (появляется блок «До:»), год 1917 → создание → View показывает «между 1880 и 1917» → «Редактировать» — форма верно предзаполнена; `/archives` → «+ добавить» → `Select` «Хранилище» показывает ранее созданные записи, поиск по названию работает → выбор → создание → View показывает название хранилища ссылкой на `/repositories/{id}`.

### Шаг 3.7. Коммит

```bash
git add web/src/api.ts web/src/App.tsx web/src/pages/EntityCatalog.tsx \
  web/src/pages/ParishesList.tsx web/src/pages/ParishForm.tsx web/src/pages/ParishView.tsx \
  web/src/pages/ArchivesList.tsx web/src/pages/ArchiveForm.tsx web/src/pages/ArchiveView.tsx
git commit -m "feat(web): страницы Приходов/Архивов — FactDateEditor, Select хранилищ"
```

## Рубеж прохода

После Задачи 3: `gofmt -l .` пусто, `go build/vet/test ./...` зелёные (894
теста, 76 пакетов), `npm run typecheck`/`build` чисты. Каталог `/` — 11
строк по алфавиту. Все 4 сущности (`Repository`, `Church`, `Parish`,
`Archive`) имеют полный CRUD через HTTP, MCP и веб, по конвенциям
`docs/data-model/entity-write.md` §3-4, плюс новые паттерны (одиночный
`*TextRef`, строгий скалярный FK через `Select`, `FactDateEditor`,
read-only `SourceLink`) — задел для подпроектов 4-9, где эти же поля
встретятся снова. Следующий подпроект — 4 (`Note` с self-ref `ParentID`,
`Attachment` с `DocumentID *ID`, `SET NULL`).

## Коммиты

Три коммита в `main`, по одному на задачу — см. Шаги 1.11, 2.8, 3.7.
