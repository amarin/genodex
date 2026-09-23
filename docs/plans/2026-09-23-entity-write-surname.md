# Веб-CRUD/MCP для всех сущностей — подпроект 1 (Surname): план

Первый проход по декомпозиции `docs/data-model/entity-write.md`: полный
стек записи (usecase-сценарии, транспорт, `httpapi`, MCP-тулы, веб-UI) для
`Surname` — самой простой сущности (плоский словарь, без FK), задающей
паттерн для оставшихся 19. Заодно — навигационная перестройка веб-UI:
каталог сущностей на `/` вместо вкладок, хлебные крошки на каждой странице
(согласовано с пользователем при обсуждении дизайн-документа, коммит
`d2c9c1f`). Формат плана — как у прочих проходов. Исполняется в `main`
подрядными коммитами.

## Goal

1. Полный CRUD для `Surname` — HTTP, MCP, веб — по конвенциям
   `docs/data-model/entity-write.md` §3-4.
2. Единая точка входа веб-UI (`/` — каталог сущностей по алфавиту) вместо
   вкладок; хлебные крошки от корня на каждой странице.
3. `httpapi.NewAPIHandler`/`NewHandler` и `mcp.NewServer` переведены на
   структуру `Deps` — задел на добавление сущностей 2-20 без изменения
   сигнатур.

## Предпосылка: живая проверка

Весь код ниже уже применён к рабочему дереву и проверен: `go build/vet/test
./...` (643 теста, 28 пакетов, `gofmt -l .` пусто), `npm run
typecheck`/`build` чисты. Живой смок-тест бэкенда через `curl` (реальный
`genodex serve`, чистая БД): анонимный `POST /api/surnames` → 401, создание
владельцем → 201, `GET`/`search` находят созданное, `PUT` с пустым
`canonical` → 422 `{"field":"canonical"}`, `PUT` с валидными данными → 200,
`DELETE` → 204, повторный `GET` → 404. Живая проверка веб-UI в браузере
(реальный сервер, `-web dev`): регистрация владельца → каталог на `/` (три
строки по алфавиту) → «Фамилии» → создание записи с вариантом написания
через `TextRefListEditor` → редирект на View с хлебными крошками «Сущности
/ Фамилии / Иванов» → «Редактировать» (форма верно предзаполнена, включая
уже введённый вариант) → сохранение → список показывает обновлённое →
удаление → редирект на список, список снова пуст. Отдельно проверено:
`/divisions` после навигационной перестройки — хлебные крошки корректны,
прямой заход по URL (без клика по каталогу) сразу показывает нужную
страницу (побочно чинит известное ограничение `Tabs`/`defaultActiveKey` из
прошлого прохода — теперь у каждой страницы свой роут, а не общий `Tabs`).

**Уточнение роадмапа**: `docs/data-model/entity-write.md` §5 предполагал
единую навигацию как задачу на будущее — решение вынесено в объём этого
прохода по ходу обсуждения (см. коммит `d2c9c1f`, обновивший §4/§5
документа). Текст документа уже отражает решение — самого документа этот
план не трогает.

## Задача 1. Бэкенд: `Deps`-рефактор + полный стек Surname

**Файлы:**
- Создать: `internal/transport/text_ref.go`, `internal/transport/surname.go`,
  `internal/transport/surname_write.go`,
  `internal/usecases/{list_surnames,search_surnames,get_surname,create_surname,update_surname,delete_surname}/{scenario.go,deps.go}`
  (+ `scenario_test.go` на каждый usecase, см. шаги ниже),
  `internal/httpapi/surname.go`, `internal/httpapi/surname_write.go`
  (+ `surname_test.go`, `surname_write_test.go`), `internal/mcp/surname.go`
  (+ `surname_test.go`)
- Изменить: `internal/models/query.go`, `internal/httpapi/{api.go,deps.go,httpapi.go}`,
  `internal/mcp/{deps.go,server.go}`, `internal/app/app.go`,
  `internal/httpapi/{division_test.go,division_write_test.go,store_test.go,write_store_test.go}`,
  `internal/mcp/{division_test.go,division_write_test.go}` (механическая
  правка вызовов под новую `Deps`)

### Шаг 1.1. `internal/models/query.go` — добавить `SearchQuery`

В конец файла (после `DivisionSearchQuery.Validate`):

```go
// SearchQuery — запрос поиска по началу названия/канонической формы (включая
// варианты), общий для сущностей без собственных доп. фильтров (см.
// DivisionSearchQuery для сущности с фильтрами). Окно применяется после
// отбора хитов own-типа.
type SearchQuery struct {
	Text string // начало текста; пустое (после обрезки) — пустой результат
	Page Page
}

// Validate проверяет запрос: отрицательные размер и сдвиг окна — *ValidationError.
// Пустой текст не ошибка.
func (q SearchQuery) Validate() error {
	switch {
	case q.Page.Limit < 0:
		return fieldErr("limit", "не может быть отрицательным: %d", q.Page.Limit)
	case q.Page.Offset < 0:
		return fieldErr("offset", "не может быть отрицательным: %d", q.Page.Offset)
	}

	return nil
}
```

### Шаг 1.2. `internal/transport/text_ref.go` (создать)

```go
package transport

import "github.com/amarin/genodex/internal/models"

// TextRef — контракт элемента списков вроде Surname.Variants: текст или
// ссылка на другую сущность. Ref/Type пустые — элемент чисто текстовый.
// v1 веб-формы редактируют только Text (docs/data-model/entity-write.md §4) —
// Ref/Type только читаются и переносятся как есть при сохранении остального
// списка.
type TextRef struct {
	Text string `json:"text"`
	Ref  string `json:"ref,omitempty"`
	Type string `json:"type,omitempty"`
}

// TextRefFromModel конвертирует элемент в контракт.
func TextRefFromModel(t models.TextRef) TextRef {
	return TextRef{Text: t.Text, Ref: string(t.Ref), Type: string(t.Type)}
}

// TextRefsFromModel конвертирует список; пустой вход даёт пустой срез, а не nil.
func TextRefsFromModel(ts []models.TextRef) []TextRef {
	out := make([]TextRef, 0, len(ts))
	for _, t := range ts {
		out = append(out, TextRefFromModel(t))
	}

	return out
}

// Model конвертирует контракт обратно в модель.
func (t TextRef) Model() models.TextRef {
	return models.TextRef{Text: t.Text, Ref: models.ID(t.Ref), Type: models.Type(t.Type)}
}

// TextRefsToModel конвертирует список контрактов в модели; пустой вход даёт
// пустой срез, а не nil.
func TextRefsToModel(ts []TextRef) []models.TextRef {
	out := make([]models.TextRef, 0, len(ts))
	for _, t := range ts {
		out = append(out, t.Model())
	}

	return out
}
```

### Шаг 1.3. `internal/transport/surname.go` (создать)

```go
package transport

import "github.com/amarin/genodex/internal/models"

// Surname — контракт словарной записи фамилии (GET /api/surnames, MCP-тул surname_list).
type Surname struct {
	ID        models.ID `json:"id"`
	Canonical string    `json:"canonical"`
	Variants  []TextRef `json:"variants"`
	Items     []TextRef `json:"items"`
	Notes     []TextRef `json:"notes"`
}

// SurnameFromModel конвертирует запись в контракт.
func SurnameFromModel(s models.Surname) Surname {
	return Surname{
		ID:        s.ID,
		Canonical: s.Canonical,
		Variants:  TextRefsFromModel(s.Variants),
		Items:     TextRefsFromModel(s.Items),
		Notes:     TextRefsFromModel(s.Notes),
	}
}

// SurnamesFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func SurnamesFromModels(ss []models.Surname) []Surname {
	out := make([]Surname, 0, len(ss))
	for _, s := range ss {
		out = append(out, SurnameFromModel(s))
	}

	return out
}
```

### Шаг 1.4. `internal/transport/surname_write.go` (создать)

```go
package transport

import "github.com/amarin/genodex/internal/models"

// SurnameCreate — тело POST /api/surnames и аргументы тула surname_create.
// Идентификатор генерирует сценарий.
type SurnameCreate struct {
	Canonical string    `json:"canonical"`
	Variants  []TextRef `json:"variants"`
	Items     []TextRef `json:"items"`
	Notes     []TextRef `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (s SurnameCreate) Model() models.Surname {
	return models.Surname{
		Canonical: s.Canonical,
		Variants:  TextRefsToModel(s.Variants),
		Items:     TextRefsToModel(s.Items),
		Notes:     TextRefsToModel(s.Notes),
	}
}

// SurnameUpdate — тело PUT /api/surnames/{id} и аргументы тула surname_update:
// полная замена canonical/variants/items/notes.
type SurnameUpdate struct {
	Canonical string    `json:"canonical"`
	Variants  []TextRef `json:"variants"`
	Items     []TextRef `json:"items"`
	Notes     []TextRef `json:"notes"`
}

// Model возвращает доменную запись с пустым ID.
func (s SurnameUpdate) Model() models.Surname {
	return models.Surname{
		Canonical: s.Canonical,
		Variants:  TextRefsToModel(s.Variants),
		Items:     TextRefsToModel(s.Items),
		Notes:     TextRefsToModel(s.Notes),
	}
}
```

### Шаг 1.5. Usecase-сценарии — создать 6 пакетов

`internal/usecases/get_surname/deps.go`:
```go
package get_surname

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// SurnameRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type SurnameRepo interface {
	GetSurname(ctx context.Context, id models.ID) (*models.Surname, error)
}
```

`internal/usecases/get_surname/scenario.go`:
```go
package get_surname

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «словарная запись фамилии по идентификатору».
type Scenario struct {
	surnames SurnameRepo
}

// New создаёт сценарий.
func New(surnames SurnameRepo) *Scenario {
	return &Scenario{surnames: surnames}
}

// GetSurname возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound.
func (s *Scenario) GetSurname(ctx context.Context, id models.ID) (models.Surname, error) {
	if err := validateID(id); err != nil {
		return models.Surname{}, err
	}

	sn, err := s.surnames.GetSurname(ctx, id)
	if err != nil {
		return models.Surname{}, err
	}

	return *sn, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeSurname)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeSurname,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

`internal/usecases/get_surname/scenario_test.go`:
```go
package get_surname

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	surnames map[models.ID]*models.Surname
}

func (f *fakeRepo) GetSurname(_ context.Context, id models.ID) (*models.Surname, error) {
	s, ok := f.surnames[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestGetSurnameReturnsRecord(t *testing.T) {
	id := models.ID("SN-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{surnames: map[models.ID]*models.Surname{id: {ID: id, Canonical: "Иванов"}}}

	got, err := New(repo).GetSurname(context.Background(), id)
	if err != nil {
		t.Fatalf("GetSurname: %v", err)
	}

	if got.Canonical != "Иванов" {
		t.Fatalf("Canonical = %q", got.Canonical)
	}
}

func TestGetSurnameNotFound(t *testing.T) {
	repo := &fakeRepo{surnames: map[models.ID]*models.Surname{}}

	_, err := New(repo).GetSurname(context.Background(), "SN-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetSurnameInvalidID(t *testing.T) {
	repo := &fakeRepo{surnames: map[models.ID]*models.Surname{}}

	_, err := New(repo).GetSurname(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}
```

`internal/usecases/list_surnames/deps.go`:
```go
package list_surnames

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// SurnameRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type SurnameRepo interface {
	ListSurnames(ctx context.Context, access models.Access, page models.Page) ([]*models.Surname, error)
}
```

`internal/usecases/list_surnames/scenario.go`:
```go
package list_surnames

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список словарных записей фамилий».
type Scenario struct {
	surnames SurnameRepo
}

// New создаёт сценарий.
func New(surnames SurnameRepo) *Scenario {
	return &Scenario{surnames: surnames}
}

// ListSurnames возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListSurnames(ctx context.Context, access models.Access, page models.Page) ([]models.Surname, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeSurname, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeSurname, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.surnames.ListSurnames(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Surname, 0, len(list))
	for _, sn := range list {
		out = append(out, *sn)
	}

	return out, nil
}
```

`internal/usecases/list_surnames/scenario_test.go`:
```go
package list_surnames

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.Surname
}

func (f *fakeRepo) ListSurnames(_ context.Context, _ models.Access, page models.Page) ([]*models.Surname, error) {
	f.page = page

	return f.out, nil
}

func TestListSurnamesReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.Surname{{ID: "SN-01ARZ3NDEKTSV4RRFFQ69G5FA1", Canonical: "Иванов"}}}

	got, err := New(repo).ListSurnames(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListSurnames: %v", err)
	}

	if len(got) != 1 || got[0].Canonical != "Иванов" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListSurnamesRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListSurnames(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
```

`internal/usecases/search_surnames/deps.go`:
```go
package search_surnames

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// SurnameRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type SurnameRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetSurname(ctx context.Context, id models.ID) (*models.Surname, error)
}
```

`internal/usecases/search_surnames/scenario.go`:
```go
package search_surnames

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск словарных записей фамилий».
type Scenario struct {
	surnames SurnameRepo
}

// New создаёт сценарий.
func New(surnames SurnameRepo) *Scenario {
	return &Scenario{surnames: surnames}
}

// SearchSurnames находит записи, чья каноническая форма (или вариант)
// начинается с текста запроса — та же механика, что и
// search_divisions.SearchDivisions (см. её комментарий).
func (s *Scenario) SearchSurnames(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Surname, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Surname{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Surname{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.surnames.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeSurname {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.surnames.GetSurname(ctx, h.ID)
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

`internal/usecases/search_surnames/scenario_test.go`:
```go
package search_surnames

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits     []models.Hit
	surnames map[models.ID]*models.Surname
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetSurname(_ context.Context, id models.ID) (*models.Surname, error) {
	s, ok := f.surnames[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestSearchSurnamesFiltersByType(t *testing.T) {
	id := models.ID("SN-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeSurname, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не surname — должен быть пропущен
		},
		surnames: map[models.ID]*models.Surname{id: {ID: id, Canonical: "Иванов"}},
	}

	got, err := New(repo).SearchSurnames(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchSurnames: %v", err)
	}

	if len(got) != 1 || got[0].Canonical != "Иванов" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchSurnamesEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchSurnames(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchSurnames: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
```

`internal/usecases/create_surname/deps.go`:
```go
package create_surname

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// SurnameStore — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type SurnameStore interface {
	SaveSurname(ctx context.Context, s *models.Surname) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
```

`internal/usecases/create_surname/scenario.go`:
```go
package create_surname

import (
	"context"
	"fmt"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «создание словарной записи фамилии».
type Scenario struct {
	store SurnameStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st SurnameStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateSurname создаёт запись: генерирует идентификатор, проверяет
// инварианты, сохраняет. Возвращает созданную запись с заполненным ID.
// Флат-сущность без FK — в отличие от create_division, без транзакционной
// проверки родителя.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта) и невалидная сущность — *models.ValidationError; прочее —
// ошибки хранилища как есть.
func (s *Scenario) CreateSurname(ctx context.Context, sn models.Surname) (models.Surname, error) {
	if sn.ID != "" {
		return models.Surname{}, &models.ValidationError{
			Entity: models.TypeSurname,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", sn.ID),
		}
	}

	sn.ID = s.ids.New(models.TypeSurname)

	if err := sn.Validate(); err != nil {
		return models.Surname{}, err
	}

	if err := s.store.SaveSurname(ctx, &sn); err != nil {
		return models.Surname{}, err
	}

	return sn, nil
}
```

`internal/usecases/create_surname/scenario_test.go`:
```go
package create_surname

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type stubIDs struct{ id models.ID }

func (s *stubIDs) New(models.Type) models.ID { return s.id }

type fakeStore struct {
	saved   *models.Surname
	saveErr error
}

func (f *fakeStore) SaveSurname(_ context.Context, s *models.Surname) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *s
	f.saved = &cp

	return nil
}

func TestCreateSurnameGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "SN-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{}
	sc := New(st, ids)

	got, err := sc.CreateSurname(context.Background(), models.Surname{Canonical: "Иванов"})
	if err != nil {
		t.Fatalf("CreateSurname: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.saved == nil || st.saved.Canonical != "Иванов" {
		t.Fatalf("saved = %+v", st.saved)
	}
}

func TestCreateSurnameRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{})

	_, err := sc.CreateSurname(context.Background(), models.Surname{ID: "SN-01ARZ3NDEKTSV4RRFFQ69G5FA1", Canonical: "x"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestCreateSurnameRejectsEmptyCanonical(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{id: "SN-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateSurname(context.Background(), models.Surname{})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "canonical" {
		t.Fatalf("err = %v, want ValidationError on canonical", err)
	}
}

func TestCreateSurnamePropagatesStoreError(t *testing.T) {
	wantErr := errors.New("db down")
	st := &fakeStore{saveErr: wantErr}
	sc := New(st, &stubIDs{id: "SN-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateSurname(context.Background(), models.Surname{Canonical: "Иванов"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}
```

`internal/usecases/update_surname/deps.go`:
```go
package update_surname

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// SurnameStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования и сохранение идут в одной транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type SurnameStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
```

`internal/usecases/update_surname/scenario.go`:
```go
package update_surname

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение словарной записи фамилии».
type Scenario struct {
	store SurnameStore
}

// New создаёт сценарий.
func New(st SurnameStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateSurname полностью заменяет запись по sn.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, и сохраняет.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateSurname(ctx context.Context, sn models.Surname) error {
	if err := sn.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetSurname(ctx, sn.ID); err != nil {
			return err
		}

		return tx.SaveSurname(ctx, &sn)
	})
}
```

`internal/usecases/update_surname/scenario_test.go`:
```go
package update_surname

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
	surnames map[models.ID]*models.Surname
	saved    []*models.Surname
}

func newFakeTx(existing ...*models.Surname) *fakeTx {
	tx := &fakeTx{surnames: map[models.ID]*models.Surname{}}
	for _, s := range existing {
		tx.surnames[s.ID] = s
	}

	return tx
}

func (f *fakeTx) GetSurname(_ context.Context, id models.ID) (*models.Surname, error) {
	s, ok := f.surnames[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *s

	return &cp, nil
}

func (f *fakeTx) SaveSurname(_ context.Context, s *models.Surname) error {
	cp := *s
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует SurnameStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func surname(id models.ID, canonical string) *models.Surname {
	return &models.Surname{ID: id, Canonical: canonical}
}

func TestUpdateSurnameSaves(t *testing.T) {
	existing := surname("SN-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Иванов")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Canonical = "Иванов (испр.)"

	if err := New(st).UpdateSurname(context.Background(), updated); err != nil {
		t.Fatalf("UpdateSurname: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Canonical != "Иванов (испр.)" {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateSurnameNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateSurname(context.Background(), *surname("SN-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Иванов"))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateSurnameRejectsEmptyCanonical(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateSurname(context.Background(), models.Surname{ID: "SN-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "canonical" {
		t.Fatalf("err = %v, want ValidationError on canonical", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}
```

`internal/usecases/delete_surname/deps.go`:
```go
package delete_surname

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// SurnameRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type SurnameRepo interface {
	DeleteSurname(ctx context.Context, id models.ID) error
}
```

`internal/usecases/delete_surname/scenario.go`:
```go
package delete_surname

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление словарной записи фамилии».
type Scenario struct {
	surnames SurnameRepo
}

// New создаёт сценарий.
func New(surnames SurnameRepo) *Scenario {
	return &Scenario{surnames: surnames}
}

// DeleteSurname удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteSurname(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.surnames.DeleteSurname(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeSurname)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeSurname,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

`internal/usecases/delete_surname/scenario_test.go`:
```go
package delete_surname

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

func (f *fakeRepo) DeleteSurname(_ context.Context, id models.ID) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)

	return nil
}

func TestDeleteSurnameCallsRepo(t *testing.T) {
	repo := &fakeRepo{}
	id := models.ID("SN-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	if err := New(repo).DeleteSurname(context.Background(), id); err != nil {
		t.Fatalf("DeleteSurname: %v", err)
	}

	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("deleted = %v", repo.deleted)
	}
}

func TestDeleteSurnameInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	err := New(repo).DeleteSurname(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}

	if len(repo.deleted) != 0 {
		t.Fatalf("repo вызван при невалидном id")
	}
}

func TestDeleteSurnamePropagatesInUseError(t *testing.T) {
	wantErr := &models.InUseError{Type: models.TypeSurname, ID: "SN-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	repo := &fakeRepo{err: wantErr}

	err := New(repo).DeleteSurname(context.Background(), "SN-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	var iu *models.InUseError
	if !errors.As(err, &iu) {
		t.Fatalf("err = %v, want InUseError", err)
	}
}
```

### Шаг 1.6. `internal/httpapi/deps.go` — добавить `SurnameService`

После блока `DivisionService` (перед `AuthService`):
```go
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
```

### Шаг 1.7. `internal/httpapi/surname.go` (создать)

```go
package httpapi

import (
	"net/http"
	"net/url"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleSurnameList — GET /api/surnames?limit=&offset=. Чтение открыто
// анонимному посетителю (как и деления — auth.md §6, Access здесь не
// проверяется).
func handleSurnameList(surnames SurnameService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := surnames.ListSurnames(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.SurnamesFromModels(list))
	}
}

// handleSurnameSearch — GET /api/surnames/search?q=&limit=&offset=.
func handleSurnameSearch(surnames SurnameService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := surnames.SearchSurnames(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.SurnamesFromModels(list))
	}
}

// handleSurnameGet — GET /api/surnames/{id}.
func handleSurnameGet(surnames SurnameService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sn, err := surnames.GetSurname(r.Context(), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.SurnameFromModel(sn))
	}
}

// parsePage разбирает limit/offset из query-параметров.
func parsePage(v url.Values) (models.Page, error) {
	var page models.Page

	var err error

	if page.Limit, err = intParam(v, "limit"); err != nil {
		return page, err
	}

	if page.Offset, err = intParam(v, "offset"); err != nil {
		return page, err
	}

	return page, nil
}
```

### Шаг 1.8. `internal/httpapi/surname_write.go` (создать)

```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleSurnameCreate — POST /api/surnames: создаёт запись, отвечает 201 с
// созданной записью (id генерирует сценарий). Запись — только для вошедшего
// владельца (auth.md §6, решение 9), см. handleDivisionCreate.
func handleSurnameCreate(surnames SurnameService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.SurnameCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := surnames.CreateSurname(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.SurnameFromModel(created))
	}
}

// handleSurnameUpdate — PUT /api/surnames/{id}: полная замена
// canonical/variants/items/notes. Читает текущую версию, накладывает поля
// запроса (fetch-then-merge — как handleDivisionUpdate; у Surname сейчас DTO
// покрывает всю модель, но конвенция общая для будущих сущностей, чей DTO
// v1 не покрывает всех полей модели, docs/data-model/entity-write.md §3).
func handleSurnameUpdate(surnames SurnameService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.SurnameUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := surnames.GetSurname(r.Context(), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Canonical = m.Canonical
		cur.Variants = m.Variants
		cur.Items = m.Items
		cur.Notes = m.Notes

		if err := surnames.UpdateSurname(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.SurnameFromModel(cur))
	}
}

// handleSurnameDelete — DELETE /api/surnames/{id}: 204 без тела; занятая
// запись — 409 со списком ссылающихся. Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handleSurnameDelete(surnames SurnameService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := surnames.DeleteSurname(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
```

### Шаг 1.9. `internal/httpapi/httpapi.go` — `Deps`, `pathID`, роуты Surname

Заменить:
```go
func NewHandler(divisions DivisionService, docsFS fs.FS) http.Handler {
	mux := http.NewServeMux()
	registerDivisionRoutes(mux, divisions, docsFS)

	return mux
}
```
на:
```go
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

	return mux
}
```

После `registerDivisionRoutes` добавить:
```go
// registerSurnameRoutes регистрирует маршруты /api/surnames на переданном mux.
func registerSurnameRoutes(mux *http.ServeMux, surnames SurnameService) {
	mux.HandleFunc("GET /api/surnames", handleSurnameList(surnames))
	mux.HandleFunc("GET /api/surnames/search", handleSurnameSearch(surnames))
	mux.HandleFunc("GET /api/surnames/{id}", handleSurnameGet(surnames))
	mux.HandleFunc("POST /api/surnames", handleSurnameCreate(surnames))
	mux.HandleFunc("PUT /api/surnames/{id}", handleSurnameUpdate(surnames))
	mux.HandleFunc("DELETE /api/surnames/{id}", handleSurnameDelete(surnames))
}
```

После `handleHealth` добавить (и добавить `"strings"` в импорты файла):
```go
// pathID читает {id} из пути запроса — общий хелпер для всех сущностей
// (division.go's pathDivisionID — исторический синоним, оставлен как есть).
func pathID(r *http.Request) models.ID {
	return models.ID(strings.TrimPrefix(r.PathValue("id"), "/"))
}
```

### Шаг 1.10. `internal/httpapi/api.go` — `Deps`, `NewAPIHandler`

Заменить весь блок `NewAPIHandler` на:
```go
// Deps — сервисы, монтируемые в /api (NewAPIHandler) и /api без auth-обёртки
// (NewHandler, юнит-тесты пакета). Явный реестр вместо растущего списка
// позиционных параметров — новая сущность добавляется полем структуры, не
// меняя сигнатуру функций (docs/data-model/entity-write.md §3; решение
// принято при добавлении Surname, первой сущности после AdministrativeDivision).
type Deps struct {
	Divisions  DivisionService
	Surnames   SurnameService
	Auth       AuthService
	DocsFS     fs.FS
	TrustProxy bool
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
	registerSurnameRoutes(mux, deps.Surnames)
	registerAuthRoutes(mux, deps.Auth, deps.TrustProxy)

	return requireCSRFHeader(resolveAccess(deps.Auth)(mux))
}
```

### Шаг 1.11. `internal/httpapi/surname_test.go` (создать)

```go
package httpapi

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeSurnames struct {
	list []models.Surname
	err  error
	page models.Page

	getSN     models.Surname
	gotIDs    []models.ID
	created   models.Surname
	gotCreate models.Surname
	updated   models.Surname
	deleteErr error

	search    []models.Surname
	gotSearch models.SearchQuery
}

func (f *fakeSurnames) ListSurnames(_ context.Context, _ models.Access, page models.Page) ([]models.Surname, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeSurnames) SearchSurnames(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Surname, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeSurnames) GetSurname(_ context.Context, id models.ID) (models.Surname, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Surname{}, f.err
	}

	return f.getSN, nil
}

func (f *fakeSurnames) CreateSurname(_ context.Context, sn models.Surname) (models.Surname, error) {
	f.gotCreate = sn
	if f.err != nil {
		return models.Surname{}, f.err
	}

	return f.created, nil
}

func (f *fakeSurnames) UpdateSurname(_ context.Context, sn models.Surname) error {
	f.updated = sn

	return f.err
}

func (f *fakeSurnames) DeleteSurname(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestSurnameListReturnsRecords(t *testing.T) {
	svc := &fakeSurnames{list: []models.Surname{{ID: "SN-1", Canonical: "Иванов"}}}

	rec := get(t, NewHandler(Deps{Surnames: svc, DocsFS: fstest.MapFS{}}), "/api/surnames")
	requireStatus(t, rec, 200)

	if rec.Body.String() == "" {
		t.Fatalf("empty body")
	}
}

func TestSurnameGetNotFound(t *testing.T) {
	svc := &fakeSurnames{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Surnames: svc, DocsFS: fstest.MapFS{}}), "/api/surnames/SN-1")
	requireStatus(t, rec, 404)
}

func TestSurnameSearchEmptyQ(t *testing.T) {
	svc := &fakeSurnames{}

	rec := get(t, NewHandler(Deps{Surnames: svc, DocsFS: fstest.MapFS{}}), "/api/surnames/search?q=Ив")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "Ив" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
```

### Шаг 1.12. `internal/httpapi/surname_write_test.go` (создать)

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

func TestSurnameCreateContract(t *testing.T) {
	svc := &fakeSurnames{created: models.Surname{ID: "SN-1", Canonical: "Иванов"}}

	rec := postD(t, NewHandler(Deps{Surnames: svc, DocsFS: fstest.MapFS{}}), "/api/surnames", `{"canonical":"Иванов"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Canonical != "Иванов" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

func TestSurnameCreateAnonymousIs401(t *testing.T) {
	svc := &fakeSurnames{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Surnames: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/surnames", strings.NewReader(`{"canonical":"Иванов"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestSurnameUpdateMergesFields(t *testing.T) {
	svc := &fakeSurnames{getSN: models.Surname{ID: "SN-1", Canonical: "Иванов"}}

	rec := putD(t, NewHandler(Deps{Surnames: svc, DocsFS: fstest.MapFS{}}), "/api/surnames/SN-1",
		`{"canonical":"Иванов (испр.)","variants":[{"text":"Иванова"}]}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Canonical != "Иванов (испр.)" || svc.updated.ID != "SN-1" || len(svc.updated.Variants) != 1 {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestSurnameDeleteNoContent(t *testing.T) {
	svc := &fakeSurnames{}

	rec := delD(t, NewHandler(Deps{Surnames: svc, DocsFS: fstest.MapFS{}}), "/api/surnames/SN-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestSurnameDeleteInUseIs409(t *testing.T) {
	svc := &fakeSurnames{deleteErr: &models.InUseError{Type: models.TypeSurname, ID: "SN-1"}}

	rec := delD(t, NewHandler(Deps{Surnames: svc, DocsFS: fstest.MapFS{}}), "/api/surnames/SN-1")

	requireStatus(t, rec, http.StatusConflict)
}
```

### Шаг 1.13. Механическая правка вызовов `NewHandler`/`NewAPIHandler` в существующих тестах `httpapi`

Из директории `internal/httpapi/`:
```bash
sed -i.bak -E 's/NewHandler\((.+), fstest\.MapFS\{\}\)/NewHandler(Deps{Divisions: \1, DocsFS: fstest.MapFS{}})/' division_test.go division_write_test.go
sed -i.bak -E 's/httpapi\.NewHandler\((.+), fstest\.MapFS\{\}\)/httpapi.NewHandler(httpapi.Deps{Divisions: \1, DocsFS: fstest.MapFS{}})/' store_test.go
sed -i.bak -E 's/httpapi\.NewAPIHandler\(newDivisionService\(t, st\), authSvc, fstest\.MapFS\{\}, false\)/httpapi.NewAPIHandler(httpapi.Deps{Divisions: newDivisionService(t, st), Auth: authSvc, DocsFS: fstest.MapFS{}, TrustProxy: false})/' write_store_test.go
rm -f *.bak
```
Проверь результат (`grep -n "NewHandler(\|NewAPIHandler(" division_test.go division_write_test.go store_test.go write_store_test.go`) — каждый вызов должен стать `NewHandler(Deps{Divisions: ..., DocsFS: ...})` / `httpapi.NewAPIHandler(httpapi.Deps{...})`, без синтаксических ошибок (особое внимание — `store_test.go`, там аргумент `newDivisionService(t, st)` сам содержит запятую, жадный `(.+)` в sed должен захватить его целиком, не оборваться на внутренней запятой; после правки обязательно `go build ./...`, чтобы поймать, если где-то не сработало).

### Шаг 1.14. `internal/mcp/deps.go` — добавить `SurnameService`

В конец файла:
```go
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
```

### Шаг 1.15. `internal/mcp/surname.go` (создать)

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

// registerSurnameTools регистрирует тулы для работы со словарными записями
// фамилий. Запись variants/items/notes — только текстом (v1, как и веб-форма,
// docs/data-model/entity-write.md §4): элемент с уже существующей ссылкой
// (Ref/Type) через эти тулы не создать, только прочитать (list/get/search
// отдают полный TextRef, включая Ref/Type).
func registerSurnameTools(s *server.MCPServer, surnames SurnameService) {
	tool := mcp.NewTool(
		"surname_list",
		mcp.WithDescription("Список словарных записей фамилий в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, surnameListHandler(surnames))

	tool = mcp.NewTool(
		"surname_search",
		mcp.WithDescription("Поиск словарных записей фамилий по началу канонической формы (включая варианты); результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало канонической формы или варианта")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, surnameSearchHandler(surnames))

	tool = mcp.NewTool(
		"surname_get",
		mcp.WithDescription("Словарная запись фамилии по id; результат — JSON записи. Неверный формат id или отсутствующая запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например SN-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, surnameGetHandler(surnames))

	tool = mcp.NewTool(
		"surname_create",
		mcp.WithDescription("Создать словарную запись фамилии; id генерируется сервером; результат — JSON созданной записи. variants/items/notes — списки текста (без ссылок на другие сущности, v1)"),
		mcp.WithString("canonical", mcp.Required(), mcp.Description("Каноническая форма")),
		mcp.WithArray("variants", mcp.WithStringItems(), mcp.Description("Варианты написания")),
		mcp.WithArray("items", mcp.WithStringItems(), mcp.Description("Прочие связанные записи (текстом)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
	)
	s.AddTool(tool, surnameCreateHandler(surnames))

	tool = mcp.NewTool(
		"surname_update",
		mcp.WithDescription("Изменить словарную запись фамилии: полная замена canonical/variants/items/notes; результат — JSON обновлённой записи"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("canonical", mcp.Required(), mcp.Description("Новая каноническая форма")),
		mcp.WithArray("variants", mcp.WithStringItems(), mcp.Description("Варианты написания")),
		mcp.WithArray("items", mcp.WithStringItems(), mcp.Description("Прочие связанные записи (текстом)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
	)
	s.AddTool(tool, surnameUpdateHandler(surnames))

	tool = mcp.NewTool(
		"surname_delete",
		mcp.WithDescription("Удалить словарную запись фамилии, если она не занята другими сущностями; занятая — ошибка тула. Необратимо"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, surnameDeleteHandler(surnames))
}

func surnameListHandler(surnames SurnameService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := surnames.ListSurnames(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.SurnamesFromModels(list))
	}
}

func surnameSearchHandler(surnames SurnameService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := surnames.SearchSurnames(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.SurnamesFromModels(list))
	}
}

func surnameGetHandler(surnames SurnameService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sn, err := surnames.GetSurname(ctx, models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.SurnameFromModel(sn))
	}
}

// textRefsFromStrings — v1 текстовые списки MCP-тулов в []models.TextRef без Ref/Type.
func textRefsFromStrings(ss []string) []models.TextRef {
	out := make([]models.TextRef, 0, len(ss))
	for _, s := range ss {
		out = append(out, models.TextRef{Text: s})
	}

	return out
}

func surnameCreateHandler(surnames SurnameService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sn := models.Surname{
			Canonical: req.GetString("canonical", ""),
			Variants:  textRefsFromStrings(req.GetStringSlice("variants", nil)),
			Items:     textRefsFromStrings(req.GetStringSlice("items", nil)),
			Notes:     textRefsFromStrings(req.GetStringSlice("notes", nil)),
		}

		created, err := surnames.CreateSurname(ctx, sn)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.SurnameFromModel(created))
	}
}

func surnameUpdateHandler(surnames SurnameService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := surnames.GetSurname(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		cur.Canonical = req.GetString("canonical", "")
		cur.Variants = textRefsFromStrings(req.GetStringSlice("variants", nil))
		cur.Items = textRefsFromStrings(req.GetStringSlice("items", nil))
		cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))

		if err := surnames.UpdateSurname(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.SurnameFromModel(cur))
	}
}

func surnameDeleteHandler(surnames SurnameService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := surnames.DeleteSurname(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
```

### Шаг 1.16. `internal/mcp/server.go` — `Deps`

Заменить:
```go
// NewServer создаёт MCP-сервер и регистрирует доступные тулы.
func NewServer(divisions DivisionService) *server.MCPServer {
	s := server.NewMCPServer(
		"genodex",
		"0.1.0",
	)

	registerDivisionTools(s, divisions)

	return s
}
```
на:
```go
// Deps — сервисы, отдаваемые в MCP-тулы. Явный реестр вместо растущего
// списка позиционных параметров (docs/data-model/entity-write.md §3) — новая
// сущность добавляется полем структуры, не меняя сигнатуру NewServer.
type Deps struct {
	Divisions DivisionService
	Surnames  SurnameService
}

// NewServer создаёт MCP-сервер и регистрирует доступные тулы.
func NewServer(deps Deps) *server.MCPServer {
	s := server.NewMCPServer(
		"genodex",
		"0.1.0",
	)

	registerDivisionTools(s, deps.Divisions)
	registerSurnameTools(s, deps.Surnames)

	return s
}
```

### Шаг 1.17. `internal/mcp/surname_test.go` (создать)

```go
package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

// fakeSurnames запоминает запрос и отдаёт заданный ответ.
type fakeSurnames struct {
	list []models.Surname
	err  error

	getSN     models.Surname
	created   models.Surname
	gotCreate models.Surname
	updated   models.Surname
	gotIDs    []models.ID
	deleteErr error

	search []models.Surname
}

func (f *fakeSurnames) ListSurnames(context.Context, models.Access, models.Page) ([]models.Surname, error) {
	return f.list, f.err
}

func (f *fakeSurnames) SearchSurnames(context.Context, models.Access, models.SearchQuery) ([]models.Surname, error) {
	return f.search, f.err
}

func (f *fakeSurnames) GetSurname(_ context.Context, id models.ID) (models.Surname, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Surname{}, f.err
	}

	return f.getSN, nil
}

func (f *fakeSurnames) CreateSurname(_ context.Context, sn models.Surname) (models.Surname, error) {
	f.gotCreate = sn
	if f.err != nil {
		return models.Surname{}, f.err
	}

	return f.created, nil
}

func (f *fakeSurnames) UpdateSurname(_ context.Context, sn models.Surname) error {
	f.updated = sn

	return f.err
}

func (f *fakeSurnames) DeleteSurname(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callSurnameTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestSurnameGetToolContract(t *testing.T) {
	svc := &fakeSurnames{getSN: models.Surname{ID: "SN-1", Canonical: "Иванов"}}

	res := callSurnameTool(t, surnameGetHandler(svc), map[string]any{"id": "SN-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "SN-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

func TestSurnameCreateToolPassesVariants(t *testing.T) {
	svc := &fakeSurnames{created: models.Surname{ID: "SN-new", Canonical: "Иванов"}}

	res := callSurnameTool(t, surnameCreateHandler(svc), map[string]any{
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

func TestSurnameDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeSurnames{deleteErr: &models.InUseError{Type: models.TypeSurname, ID: "SN-1"}}

	res := callSurnameTool(t, surnameDeleteHandler(svc), map[string]any{"id": "SN-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersSurnameTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Surnames: &fakeSurnames{}}).ListTools()

	for _, name := range []string{"surname_list", "surname_search", "surname_get", "surname_create", "surname_update", "surname_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
```

### Шаг 1.18. Механическая правка `mcp.NewServer(...)` в существующих тестах `mcp`

Из директории `internal/mcp/`:
```bash
sed -i.bak -E 's/NewServer\(&fakeDivisions\{\}\)/NewServer(Deps{Divisions: \&fakeDivisions{}})/' division_test.go division_write_test.go
rm -f *.bak
```

### Шаг 1.19. `internal/app/app.go` — `surnameService`, сборка, `Deps`

В блок импортов usecases добавить (в алфавитном порядке рядом с division-аналогами):
```go
	create_surname "github.com/amarin/genodex/internal/usecases/create_surname"
	delete_surname "github.com/amarin/genodex/internal/usecases/delete_surname"
	get_surname "github.com/amarin/genodex/internal/usecases/get_surname"
	list_surnames "github.com/amarin/genodex/internal/usecases/list_surnames"
	search_surnames "github.com/amarin/genodex/internal/usecases/search_surnames"
	update_surname "github.com/amarin/genodex/internal/usecases/update_surname"
```

После `divisionService`'s `DeleteDivision` метода и перед блоком `var (...)` добавить:
```go
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
```

Блок `var (...)` — добавить две строки:
```go
var (
	_ httpapi.DivisionService = (*divisionService)(nil)
	_ mcp.DivisionService     = (*divisionService)(nil)
	_ httpapi.SurnameService  = (*surnameService)(nil)
	_ mcp.SurnameService      = (*surnameService)(nil)
	_ httpapi.AuthService     = (*auth.Service)(nil)
	_ mcp.TokenResolver       = (*auth.Service)(nil)
)
```

В `New(cfg Config)`, после сборки `divisions := &divisionService{...}` добавить:
```go
	surnames := &surnameService{
		list:   list_surnames.New(st),
		search: search_surnames.New(st),
		get:    get_surname.New(st),
		create: create_surname.New(st, idgen.New()),
		update: update_surname.New(st),
		del:    delete_surname.New(st),
	}
```

Заменить:
```go
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcp.RequireAPIToken(authService)(server.NewStreamableHTTPServer(mcp.NewServer(divisions))))
	mux.Handle("/api/", httpapi.NewAPIHandler(divisions, authService, genodex.DocsFS(cfg.WebMode), cfg.TrustProxy))
```
на:
```go
	mux := http.NewServeMux()
	mux.Handle("/mcp", mcp.RequireAPIToken(authService)(server.NewStreamableHTTPServer(
		mcp.NewServer(mcp.Deps{Divisions: divisions, Surnames: surnames}),
	)))
	mux.Handle("/api/", httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  divisions,
		Surnames:   surnames,
		Auth:       authService,
		DocsFS:     genodex.DocsFS(cfg.WebMode),
		TrustProxy: cfg.TrustProxy,
	}))
```

### Шаг 1.20. Рубеж

`gofmt -l .` пусто; `go build ./...`, `go vet ./...`, `go test ./...`
(весь репозиторий) зелёные. Живой смок через `curl` (см. «Предпосылка») —
не обязателен повторно, но приветствуется как самопроверка.

### Шаг 1.21. Коммит

```bash
git add internal/models/query.go internal/transport/text_ref.go internal/transport/surname.go internal/transport/surname_write.go internal/usecases/list_surnames internal/usecases/search_surnames internal/usecases/get_surname internal/usecases/create_surname internal/usecases/update_surname internal/usecases/delete_surname internal/httpapi/deps.go internal/httpapi/surname.go internal/httpapi/surname_write.go internal/httpapi/httpapi.go internal/httpapi/api.go internal/httpapi/surname_test.go internal/httpapi/surname_write_test.go internal/httpapi/division_test.go internal/httpapi/division_write_test.go internal/httpapi/store_test.go internal/httpapi/write_store_test.go internal/mcp/deps.go internal/mcp/surname.go internal/mcp/server.go internal/mcp/surname_test.go internal/mcp/division_test.go internal/mcp/division_write_test.go internal/app/app.go
```
`feat(backend): Surname — полный CRUD (usecases/httpapi/mcp) + Deps-реестр`

## Задача 2. Веб-навигация: каталог сущностей + хлебные крошки, снос Tabs

**Интерфейсы, потребляемые из Задачи 1**: нет (эта задача не зависит от
бэкенда, кроме уже существующих Division-эндпойнтов).

**Файлы:**
- Создать: `web/src/pages/EntityCatalog.tsx`
- Изменить: `web/src/App.tsx`, `web/src/docs-panel.tsx`,
  `web/src/pages/DivisionsList.tsx`, `web/src/pages/DivisionView.tsx`

### Шаг 2.1. `web/src/pages/EntityCatalog.tsx` (создать)

```typescript
import { Card, List, Typography } from "antd";
import { Link } from "react-router-dom";

// CATALOG_ENTRIES — единая точка входа приложения: по алфавиту названия.
// Каждая сущность получает свой список при подключении (docs/data-model/
// entity-write.md §4) — здесь просто добавляется новая строка, без вкладок.
const CATALOG_ENTRIES: { label: string; path: string }[] = [
  { label: "Административное деление", path: "/divisions" },
  { label: "Документация", path: "/docs" },
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
(Задача 3 добавит строку «Фамилии» в `CATALOG_ENTRIES`.)

### Шаг 2.2. `web/src/App.tsx` — целиком заменить содержимое файла

```typescript
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import type { ReactNode } from "react";
import { Layout } from "antd";
import { SessionProvider } from "./session";
import { AppHeader } from "./AppHeader";
import DocsPanel from "./docs-panel";
import EntityCatalog from "./pages/EntityCatalog";
import DivisionsList from "./pages/DivisionsList";
import DivisionView from "./pages/DivisionView";
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

Это убирает `SettlementsTab` (дублировал `DivisionsList` — фильтр
`kind=settlement` теперь доступен только через дерево+поиск
«Административное деление», решение согласовано с пользователем при
обсуждении) и весь `Tabs`. `Route path="*"` — редирект на каталог для
незнакомых путей (не было раньше; дёшево и безопасно добавить сейчас).

### Шаг 2.3. `web/src/docs-panel.tsx` — хлебные крошки

Заменить:
```typescript
import { useNavigate, useParams } from "react-router-dom";
```
на:
```typescript
import { Link, useNavigate, useParams } from "react-router-dom";
```

В блоке для открытого документа (`currentFile != null`) заменить:
```typescript
        <Breadcrumb
          items={[
            { title: <a onClick={backToList}>Документация</a> },
            { title: currentFile.title },
          ]}
        />
```
на:
```typescript
        <Breadcrumb
          items={[
            { title: <Link to="/">Сущности</Link> },
            { title: <a onClick={backToList}>Документация</a> },
            { title: currentFile.title },
          ]}
        />
```

В корневом списке документов (после `return (\n    <>`) добавить перед
существующим `{error != null && ...}`:
```typescript
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "Документация" }]}
      />
```

### Шаг 2.4. `web/src/pages/DivisionView.tsx` — добавить корневую крошку

В массиве `items` компонента `Breadcrumb` (внутри `DivisionView`) перед
существующей первой строкой добавить:
```typescript
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/divisions">Административное деление</Link> },
          ...(parent != null
```
(`Link` уже импортирован в этом файле.)

### Шаг 2.5. `web/src/pages/DivisionsList.tsx` — обернуть в хлебные крошки

Заменить импорты:
```typescript
import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Alert, Button, Card, Input, List, Spin, Tree, Typography } from "antd";
```
на:
```typescript
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Input, List, Spin, Tree, Typography } from "antd";
```

Заменить `return (` компонента (начало JSX, `<Card title="Административное деление" ...`) и
закрывающий `</Card>\n  );\n}` в конце файла — обернуть в `<>...</>` с
хлебной крошкой перед `<Card>`:
```typescript
  return (
    <>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "Административное деление" }]}
      />
      <Card
        title="Административное деление"
        extra={
```
(дальше содержимое `Card` без изменений, только отступы сдвигаются на 2
пробела вправо внутри `Card`) и в конце файла:
```typescript
      </Card>
    </>
  );
}
```

### Шаг 2.6. Рубеж

`cd web && npm run typecheck && npm run build` — чисты. Живая проверка в
браузере: `/` показывает каталог (2 строки — «Административное деление»,
«Документация»), `/divisions` и `/docs` открываются напрямую по URL с
верными хлебными крошками, старой вкладки «Населённые пункты» нет нигде.

### Шаг 2.7. Коммит

```bash
git add web/src/pages/EntityCatalog.tsx web/src/App.tsx web/src/docs-panel.tsx web/src/pages/DivisionsList.tsx web/src/pages/DivisionView.tsx
```
`feat(web): каталог сущностей на / + хлебные крошки, снос Tabs/SettlementsTab`

## Задача 3. Веб-страницы Surname

**Интерфейсы, потребляемые из Задачи 1**: `authFetch`, `ApiError` (уже
существуют в `web/src/auth.ts`, не меняются).
**Интерфейсы, потребляемые из Задачи 2**: `EntityCatalog.tsx` (добавить
строку), `App.tsx` (добавить импорты + 2 роута), общий паттерн `PageLayout`.

**Файлы:**
- Создать: `web/src/TextRefList.tsx`, `web/src/pages/SurnameForm.tsx`,
  `web/src/pages/SurnamesList.tsx`, `web/src/pages/SurnameView.tsx`
- Изменить: `web/src/api.ts`, `web/src/App.tsx`, `web/src/pages/EntityCatalog.tsx`

### Шаг 3.1. `web/src/api.ts` — типы и функции Surname

Добавить перед `export interface DocFile {`:
```typescript
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
```

### Шаг 3.2. `web/src/TextRefList.tsx` (создать)

```typescript
import { Button, Input, Space, Typography } from "antd";
import { MinusCircleOutlined, PlusOutlined } from "@ant-design/icons";
import type { TextRef } from "./api";

// TextRefListEditor — общий редактор списков вроде Surname.variants
// (docs/data-model/entity-write.md §4): v1 редактирует только text —
// добавить/удалить/поменять строку. Элемент с уже заполненным ref
// показывается как текст + пометка ссылки, поле неактивно (picker на
// произвольную сущность — отдельная работа, не в этом проходе).
export function TextRefListEditor({
  value,
  onChange,
  addLabel,
}: {
  value: TextRef[];
  onChange: (next: TextRef[]) => void;
  addLabel: string;
}) {
  const setText = (i: number, text: string) => {
    const next = value.slice();
    next[i] = { ...next[i], text };
    onChange(next);
  };

  const remove = (i: number) => onChange(value.filter((_, idx) => idx !== i));

  const add = () => onChange([...value, { text: "" }]);

  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      {value.map((item, i) => {
        const linked = item.ref != null && item.ref !== "";

        return (
          <Space key={i} style={{ width: "100%" }}>
            <Input value={item.text} onChange={(e) => setText(i, e.target.value)} disabled={linked} />
            {linked && (
              <Typography.Text type="secondary">
                → {item.type} {item.ref}
              </Typography.Text>
            )}
            <MinusCircleOutlined onClick={() => remove(i)} />
          </Space>
        );
      })}
      <Button type="dashed" onClick={add} icon={<PlusOutlined />} block>
        {addLabel}
      </Button>
    </Space>
  );
}
```

### Шаг 3.3. `web/src/pages/SurnameForm.tsx` (создать)

```typescript
import { useState } from "react";
import { Alert, Form, Input, Modal } from "antd";
import { createSurname, type Surname, type TextRef } from "../api";
import { ApiError } from "../auth";
import { TextRefListEditor } from "../TextRefList";

interface SurnameFormValues {
  canonical: string;
}

const FORM_FIELDS: (keyof SurnameFormValues)[] = ["canonical"];

// CreateSurnameModal — форма создания словарной записи фамилии. variants/
// items/notes редактируются вне antd Form (TextRefListEditor — не обычный
// текстовый инпут), собираются в одно тело запроса на submit.
export function CreateSurnameModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Surname) => void;
}) {
  const [form] = Form.useForm<SurnameFormValues>();
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

  const onFinish = async (values: SurnameFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await createSurname({ canonical: values.canonical, variants, items, notes });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof SurnameFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить фамилию"
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
        <Form.Item label="Связанные записи">
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

### Шаг 3.4. `web/src/pages/SurnamesList.tsx` (создать)

```typescript
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Input, List, Spin } from "antd";
import { fetchSurnames, searchSurnames, MAX_PAGE_LIMIT, type Surname } from "../api";
import { useSession } from "../session";
import { CreateSurnameModal } from "./SurnameForm";

// SurnamesList — «Фамилии»: плоский список (сущность не иерархична, в
// отличие от AdminDivision — без Tree), пагинация по offset до короткой
// страницы (тот же приём, что DivisionsList.loadRoot — API уже отдаёт
// окнами, docs/data-model/entity-write.md §4). Поиск по названию временно
// подменяет список найденным. Клик по строке — переход на View.
export default function SurnamesList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<Surname[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<Surname[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: Surname[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchSurnames({ limit: MAX_PAGE_LIMIT, offset });
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
    searchSurnames({ q, limit: MAX_PAGE_LIMIT })
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
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "Фамилии" }]}
      />
      <Card
        title="Фамилии"
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
                <Link to={`/surnames/${s.id}`}>{s.canonical}</Link>
              </List.Item>
            )}
          />
        )}
        <CreateSurnameModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(s) => {
            setCreateOpen(false);
            navigate(`/surnames/${s.id}`);
          }}
        />
      </Card>
    </>
  );
}
```

### Шаг 3.5. `web/src/pages/SurnameView.tsx` (создать)

```typescript
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
import { deleteSurname, fetchSurname, updateSurname, type Surname, type TextRef } from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { TextRefListEditor } from "../TextRefList";

interface EditFormValues {
  canonical: string;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["canonical"];

function textRefListText(items: TextRef[]): string {
  return items.map((t) => t.text).join(", ");
}

// SurnameView — просмотр словарной записи фамилии, переключаемый в форму
// редактирования на той же странице (тот же toggle+explicit-save, что
// DivisionView — PUT заменяет запись целиком, docs/data-model/entity-write.md
// §4). Без родителя/детей/дерева — Surname не иерархична, в отличие от
// AdminDivision.
export default function SurnameView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [surname, setSurname] = useState<Surname | null>(null);
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

  const load = (surnameId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setSurname(null);
    fetchSurname(surnameId)
      .then(setSurname)
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
    if (surname == null) {
      return;
    }
    form.setFieldsValue({ canonical: surname.canonical });
    setVariants(surname.variants);
    setItems(surname.items);
    setNotes(surname.notes);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (surname == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await updateSurname(surname.id, {
        canonical: values.canonical,
        variants,
        items,
        notes,
      });
      setSurname(updated);
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
    if (surname == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteSurname(surname.id);
      navigate("/surnames");
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
          <Link to="/surnames">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (surname == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/surnames">Фамилии</Link> },
          { title: surname.canonical },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={surname.canonical} column={1} bordered size="small">
            <Descriptions.Item label="Варианты">{textRefListText(surname.variants) || "—"}</Descriptions.Item>
            <Descriptions.Item label="Связанные записи">{textRefListText(surname.items) || "—"}</Descriptions.Item>
            <Descriptions.Item label="Заметки">{textRefListText(surname.notes) || "—"}</Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title={`Удалить «${surname.canonical}»?`}
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
          <Form.Item label="Связанные записи">
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

### Шаг 3.6. `web/src/App.tsx` — подключить Surname

Добавить импорты (после `DivisionView`):
```typescript
import SurnamesList from "./pages/SurnamesList";
import SurnameView from "./pages/SurnameView";
```

В `<Routes>`, после `<Route path="/divisions/:id" .../>` добавить:
```typescript
          <Route path="/surnames" element={<PageLayout><SurnamesList /></PageLayout>} />
          <Route path="/surnames/:id" element={<PageLayout><SurnameView /></PageLayout>} />
```

### Шаг 3.7. `web/src/pages/EntityCatalog.tsx` — добавить строку

```typescript
const CATALOG_ENTRIES: { label: string; path: string }[] = [
  { label: "Административное деление", path: "/divisions" },
  { label: "Документация", path: "/docs" },
  { label: "Фамилии", path: "/surnames" },
];
```

### Шаг 3.8. Рубеж

`cd web && npm run typecheck && npm run build` — чисты. Живая проверка в
браузере (см. «Предпосылка» — полный сценарий уже пройден: каталог →
Фамилии → создание с вариантом → View с хлебными крошками →
редактирование → сохранение → список → удаление → список пуст).

### Шаг 3.9. Коммит

```bash
git add web/src/api.ts web/src/TextRefList.tsx web/src/pages/SurnameForm.tsx web/src/pages/SurnamesList.tsx web/src/pages/SurnameView.tsx web/src/App.tsx web/src/pages/EntityCatalog.tsx
```
`feat(web): страницы фамилий — список, просмотр/редактирование, форма`

## Рубеж прохода

- Владелец может создавать/просматривать/редактировать/удалять словарные
  записи фамилий через веб (`/surnames`) и через MCP-тулы `surname_*`.
- Единая точка входа веб-UI на `/` — каталог из 3 строк по алфавиту;
  хлебные крошки на каждой странице, от корня.
- `httpapi.NewAPIHandler`/`NewHandler` и `mcp.NewServer` — на `Deps`,
  готовы принять сущность 3 без изменения сигнатур.
- `go build/vet/test ./...` и `npm run typecheck`/`build` зелёные.

## Коммиты

1. `feat(backend): Surname — полный CRUD (usecases/httpapi/mcp) + Deps-реестр`
2. `feat(web): каталог сущностей на / + хлебные крошки, снос Tabs/SettlementsTab`
3. `feat(web): страницы фамилий — список, просмотр/редактирование, форма`
