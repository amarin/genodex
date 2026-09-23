# Веб-CRUD/MCP для всех сущностей — подпроект 4 (self-ref: Note, Attachment): план

> **Для агента-исполнителя:** ОБЯЗАТЕЛЬНЫЙ ПОДНАВЫК: superpowers:subagent-driven-development.

Четвёртый проход по декомпозиции `docs/data-model/entity-write.md` §2. Впервые встречаются: self-ref строгий FK (`Note.ParentID *models.ID`, `ON DELETE RESTRICT` — ссылка на запись той же сущности, требует проверки существования при создании И обхода цепочки родителей на цикл при обновлении, по образцу `create_division`/`update_division`) и обязательный строгий FK на сущность без своего CRUD-слоя (`Attachment.NodeID` → `ArchiveNode`, появится только в подпроекте 6) рядом с необязательной мягкой ссылкой (`Attachment.DocumentID *models.ID`, `ON DELETE SET NULL` — на `ArchiveDocument`, тоже без CRUD, тоже подпроект 6). Обе ссылки existence-проверяются через generic-хранилище (`store.Store.Get{ArchiveNode,ArchiveDocument}`), которое умеет читать любую сущность по id независимо от готовности её orchestration-слоя — подтверждено прямым чтением `internal/store/deps.go`.

## Goal

1. Полный CRUD (HTTP + MCP + веб) для `Note` и `Attachment` — по конвенциям `docs/data-model/entity-write.md` §3-4.
2. Новые переиспользуемые паттерны (будут использоваться в подпроектах 5-9): self-ref строгий FK с проверкой существования на create и обходом цепочки на цикл на update (`Note.ParentID`); обязательный/необязательный строгий FK на сущность без своего CRUD-слоя, существование проверяется через generic-хранилище, веб — обычное текстовое поле id без `Select`/picker'а до появления orchestration-слоя цели (`Attachment.NodeID`/`DocumentID`).
3. `Note`/`Attachment` оба содержат `Private bool` — `Get` обеих сущностей обязан быть access-aware с первого черновика (приватная запись скрыта без полного доступа), по правилу, закреплённому финальным ревью подпроекта 3 (см. `docs/data-model/entity-write.md` §3.1).

## Предпосылка: живая проверка

Весь код ниже применён и проверен в отдельном git worktree (`../genodex-verify-subproject4`, ветка `verify/entity-write-subproject4`) — не на `main` напрямую (по итогам обратной связи после подпроекта 2). `gofmt -l .` пусто, `go build/vet/test ./...` — 987 тестов, 88 пакетов (было 894/76 после подпроекта 3), `npm run typecheck`/`build` чисты.

Живой смок-тест бэкенда через `curl` (реальный `genodex serve`, чистая БД, зарегистрированный владелец): создание `Note` без родителя → 201; дочерняя `Note` с `parent_id` существующей заметки → 201; `Note` с несуществующим `parent_id` → 422 `{"field":"parent_id"}`; попытка сделать первую заметку потомком своего же потомка (переставить `parent_id` так, чтобы образовался цикл) → 422 `{"field":"parent_id"}` с текстом про цепочку родителей (`update_note`'s `checkParentChain`); приватная `Note` (`private:true`) → анонимный `GET /api/notes/{id}` → 404, `GET /api/notes` (список) её не показывает, авторизованный владелец видит запись при обоих запросах. `Attachment` без `node_id` → 422 (поле обязательно); `Attachment` с `node_id` неверного формата → 422 (ошибка валидации ULID); `Attachment` с `node_id` корректного формата, но не существующим узлом → 422 `{"field":"node_id"}` — сценарий проверяет существование через `store.GetArchiveNode` ещё до сохранения. MCP: `initialize` через API-токен владельца — рукопожатие проходит, `note_*`/`attachment_*` тулы зарегистрированы.

**Важная находка живой проверки (исправлено до коммита, не после ревью):** изначально `NoteCreate`/`NoteUpdate` (`internal/transport/note_write.go`) не содержали поле `private` ни в самом DTO, ни в httpapi merge (`handleNoteUpdate`), ни в MCP-аргументах (`note_create`/`note_update`) — приватность нельзя было выставить при создании или изменении заметки, `private` в теле запроса молча игнорировался (запись всегда создавалась с `private:false`). Найдено живой проверкой через браузер/`curl` (создали заметку с `"private":true`, получили в ответе `"private":false`), не автотестами — юнит-тесты подставляли `models.Note` напрямую, минуя DTO, и не заметили дыру в контракте. Добавлено поле `private` во все три слоя (транспорт/httpapi merge/MCP-аргументы) с регресс-тестами в каждом (`TestNoteCreatePassesPrivate`, обновлённый `TestNoteUpdateMergesFields`, `TestNoteCreateToolPassesPrivate`) — код ниже уже содержит исправление, отдельного шага для него в плане нет.

Живая проверка веб-UI в браузере (реальный сервер, собранный `web/dist`): каталог на `/` — 13 строк по алфавиту (добавились «Вложения», «Заметки»); `/notes` → «+ добавить» → форма → заголовок + текст → создание → View с хлебными крошками; вторая заметка с `parent_id` первой (`Select` с поиском, список из `fetchNotes`) + чекбокс «Приватная запись» → создание → View показывает родителя-ссылку и «Приватная: да»; поиск по заголовку в списке корректно фильтрует; `/attachments` → «+ добавить» → форма (Вид — `Select` с закрытым перечнем скан/документ/аудио/фото, `node_id`/`document_id` — обычные текстовые поля) → несуществующий `node_id` → создание → ошибка поля всплывает как `Form.Item` inline-текст под полем «Архивный узел (id)» (`attachment: node_id: узел "…" не найден»), без общего alert — обработка `ApiError.field` через `form.setFields` работает.

## Задача 1. Бэкенд: Note, Attachment

**Интерфейсы, потребляемые из подпроектов 1-3**: `models.SearchQuery`, общие хелперы `internal/httpapi/surname.go`:`parsePage`, `internal/mcp/*.go`:`optionalInt`/`toolJSONResult`, `get_repository`'s access-aware `Get` shape (образец для `get_note`/`get_attachment`), `create_division`'s `parentErr`-style существующая-проверка-FK-в-транзакции (образец для `create_note`/`create_attachment`), `update_division`'s `checkParentChain` (образец для `update_note`).
**Производит**: `httpapi.{Note,Attachment}Service`, `mcp.{Note,Attachment}Service` — потребляются Задачами 2 и 3 (веб) через HTTP/MCP.

**Файлы:**
- Изменить: `internal/httpapi/{deps.go,api.go,httpapi.go}`, `internal/mcp/{deps.go,server.go}`, `internal/app/app.go`
- Создать: `internal/transport/{note,note_write,attachment,attachment_write}.go`, `internal/usecases/{list_notes,search_notes,get_note,create_note,update_note,delete_note}/{deps.go,scenario.go,scenario_test.go}` (и та же шестёрка для `list_attachments`/`search_attachments`/`get_attachment`/`create_attachment`/`update_attachment`/`delete_attachment`), `internal/httpapi/{note,note_write,note_test,note_write_test}.go` (и то же для `attachment`), `internal/mcp/{note,note_test}.go` (и то же для `attachment`)

**Важно для исполнителя**: `list_*`/`search_*`/`delete_*` — механические (только вызывают generic `Search`/`Get`/`Delete`, без полевой логики). `get_note`/`get_attachment` — access-aware с первого черновика (см. Goal п.3). `create_note`/`update_note` — self-ref FK: `create_note` проверяет существование `parent_id` в той же транзакции, что и сохранение (по образцу `create_division`); `update_note` ДОПОЛНИТЕЛЬНО обходит цепочку родителей (`checkParentChain`) — на update запись уже существует и теоретически может быть переставлена в цепочку собственных потомков, на create это невозможно (новый id ещё ничьим предком быть не может). `create_attachment`/`update_attachment` — проверяют существование `node_id` (всегда, поле обязательно) и `document_id` (если задан), оба — в одной транзакции с сохранением; `ArchiveNode`/`ArchiveDocument` ещё не имеют своего usecase/httpapi/mcp-слоя (подпроект 6), но generic-хранилище (`store.Store`, `internal/store/deps.go`) уже умеет читать их по id — `tx.GetArchiveNode(ctx, id)`/`tx.GetArchiveDocument(ctx, id)` работают сегодня. Переносить код ниже как есть, без сокращений.

### Шаг 1.1. Note — транспорт, usecases, httpapi, MCP

#### `internal/transport/note.go` (создать)
`internal/transport/note.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// Note — контракт заметки как самостоятельной сущности (markdown-текст с
// иерархией «книга → главы», GET /api/notes, MCP-тул note_list). ParentID —
// просто id родительской заметки (не TextRef — строгая self-ref ссылка, как
// Archive.RepositoryID; пустая строка — без родителя). Sources — read-only
// в v1 (см. internal/transport/source_link.go).
type Note struct {
	ID       models.ID    `json:"id"`
	Kind     string       `json:"kind"`
	Title    string       `json:"title,omitempty"`
	Text     string       `json:"text,omitempty"`
	ParentID string       `json:"parent_id,omitempty"`
	Sources  []SourceLink `json:"sources"`
	Private  bool         `json:"private"`
}

// NoteFromModel конвертирует запись в контракт.
func NoteFromModel(n models.Note) Note {
	var parentID string
	if n.ParentID != nil {
		parentID = string(*n.ParentID)
	}

	return Note{
		ID:       n.ID,
		Kind:     string(n.Kind),
		Title:    n.Title,
		Text:     n.Text,
		ParentID: parentID,
		Sources:  SourceLinksFromModel(n.Sources),
		Private:  n.Private,
	}
}

// NotesFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func NotesFromModels(ns []models.Note) []Note {
	out := make([]Note, 0, len(ns))
	for _, n := range ns {
		out = append(out, NoteFromModel(n))
	}

	return out
}
```

#### `internal/transport/note_write.go` (создать)
`internal/transport/note_write.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// NoteCreate — тело POST /api/notes и аргументы тула note_create.
// Идентификатор генерирует сценарий. ParentID — просто id (пустая строка —
// без родителя); сценарий проверяет существование и отсутствие циклов, по
// образцу create_division.
type NoteCreate struct {
	Kind     string `json:"kind"`
	Title    string `json:"title,omitempty"`
	Text     string `json:"text,omitempty"`
	ParentID string `json:"parent_id,omitempty"`
	Private  bool   `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (n NoteCreate) Model() models.Note {
	var parentID *models.ID
	if n.ParentID != "" {
		id := models.ID(n.ParentID)
		parentID = &id
	}

	return models.Note{
		Kind:     models.NoteKind(n.Kind),
		Title:    n.Title,
		Text:     n.Text,
		ParentID: parentID,
		Private:  n.Private,
	}
}

// NoteUpdate — тело PUT /api/notes/{id} и аргументы тула note_update:
// полная замена kind/title/text/parent_id/private.
type NoteUpdate struct {
	Kind     string `json:"kind"`
	Title    string `json:"title,omitempty"`
	Text     string `json:"text,omitempty"`
	ParentID string `json:"parent_id,omitempty"`
	Private  bool   `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (n NoteUpdate) Model() models.Note {
	var parentID *models.ID
	if n.ParentID != "" {
		id := models.ID(n.ParentID)
		parentID = &id
	}

	return models.Note{
		Kind:     models.NoteKind(n.Kind),
		Title:    n.Title,
		Text:     n.Text,
		ParentID: parentID,
		Private:  n.Private,
	}
}
```

#### `internal/usecases/list_notes/deps.go` (создать)
`internal/usecases/list_notes/deps.go`:
```go
package list_notes

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// NoteRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type NoteRepo interface {
	ListNotes(ctx context.Context, access models.Access, page models.Page) ([]*models.Note, error)
}
```

#### `internal/usecases/list_notes/scenario.go` (создать)
`internal/usecases/list_notes/scenario.go`:
```go
package list_notes

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список заметок».
type Scenario struct {
	notes NoteRepo
}

// New создаёт сценарий.
func New(notes NoteRepo) *Scenario {
	return &Scenario{notes: notes}
}

// ListNotes возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListNotes(ctx context.Context, access models.Access, page models.Page) ([]models.Note, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeNote, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeNote, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.notes.ListNotes(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Note, 0, len(list))
	for _, sn := range list {
		out = append(out, *sn)
	}

	return out, nil
}
```

#### `internal/usecases/list_notes/scenario_test.go` (создать)
`internal/usecases/list_notes/scenario_test.go`:
```go
package list_notes

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.Note
}

func (f *fakeRepo) ListNotes(_ context.Context, _ models.Access, page models.Page) ([]*models.Note, error) {
	f.page = page

	return f.out, nil
}

func TestListNotesReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.Note{{ID: "N-01ARZ3NDEKTSV4RRFFQ69G5FA1", Kind: "note", Text: "текст"}}}

	got, err := New(repo).ListNotes(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}

	if len(got) != 1 || got[0].Text != "текст" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListNotesRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListNotes(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
```

#### `internal/usecases/search_notes/deps.go` (создать)
`internal/usecases/search_notes/deps.go`:
```go
package search_notes

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// NoteRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type NoteRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetNote(ctx context.Context, id models.ID) (*models.Note, error)
}
```

#### `internal/usecases/search_notes/scenario.go` (создать)
`internal/usecases/search_notes/scenario.go`:
```go
package search_notes

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск заметок».
type Scenario struct {
	notes NoteRepo
}

// New создаёт сценарий.
func New(notes NoteRepo) *Scenario {
	return &Scenario{notes: notes}
}

// SearchNotes находит заметки, чьи заголовок или текст начинаются с текста
// запроса — та же механика, что и
// search_divisions.SearchDivisions (см. её комментарий).
func (s *Scenario) SearchNotes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Note, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Note{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Note{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.notes.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeNote {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.notes.GetNote(ctx, h.ID)
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

#### `internal/usecases/search_notes/scenario_test.go` (создать)
`internal/usecases/search_notes/scenario_test.go`:
```go
package search_notes

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits  []models.Hit
	notes map[models.ID]*models.Note
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetNote(_ context.Context, id models.ID) (*models.Note, error) {
	s, ok := f.notes[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestSearchNotesFiltersByType(t *testing.T) {
	id := models.ID("N-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeNote, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не note — должен быть пропущен
		},
		notes: map[models.ID]*models.Note{id: {ID: id, Kind: "note", Text: "текст"}},
	}

	got, err := New(repo).SearchNotes(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchNotes: %v", err)
	}

	if len(got) != 1 || got[0].Text != "текст" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchNotesEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchNotes(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchNotes: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
```

#### `internal/usecases/get_note/deps.go` (создать)
`internal/usecases/get_note/deps.go`:
```go
package get_note

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// NoteRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type NoteRepo interface {
	GetNote(ctx context.Context, id models.ID) (*models.Note, error)
}
```

#### `internal/usecases/get_note/scenario.go` (создать)
`internal/usecases/get_note/scenario.go`:
```go
package get_note

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «заметка по идентификатору».
type Scenario struct {
	notes NoteRepo
}

// New создаёт сценарий.
func New(notes NoteRepo) *Scenario {
	return &Scenario{notes: notes}
}

// GetNote возвращает заметку по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой заметки — models.ErrNotFound. Приватная заметка
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound — тот же принцип «прячем как отсутствующее», что и в
// List/Search и как исправлено для get_repository/get_archive в подпроекте 3
// (docs/data-model/entity-write.md §3.1) — Note имеет Private с самого
// начала, этот параметр не добавляется задним числом.
func (s *Scenario) GetNote(ctx context.Context, access models.Access, id models.ID) (models.Note, error) {
	if err := validateID(id); err != nil {
		return models.Note{}, err
	}

	n, err := s.notes.GetNote(ctx, id)
	if err != nil {
		return models.Note{}, err
	}

	if n.Private && access != models.AccessFull {
		return models.Note{}, models.ErrNotFound
	}

	return *n, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeNote)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeNote,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

#### `internal/usecases/get_note/scenario_test.go` (создать)
`internal/usecases/get_note/scenario_test.go`:
```go
package get_note

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	notes map[models.ID]*models.Note
}

func (f *fakeRepo) GetNote(_ context.Context, id models.ID) (*models.Note, error) {
	n, ok := f.notes[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return n, nil
}

func TestGetNoteReturnsRecord(t *testing.T) {
	id := models.ID("N-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{notes: map[models.ID]*models.Note{id: {ID: id, Kind: "note", Text: "текст"}}}

	got, err := New(repo).GetNote(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}

	if got.Text != "текст" {
		t.Fatalf("Text = %q", got.Text)
	}
}

func TestGetNoteNotFound(t *testing.T) {
	repo := &fakeRepo{notes: map[models.ID]*models.Note{}}

	_, err := New(repo).GetNote(context.Background(), models.AccessFull, "N-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetNoteInvalidID(t *testing.T) {
	repo := &fakeRepo{notes: map[models.ID]*models.Note{}}

	_, err := New(repo).GetNote(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestGetNotePrivateHiddenFromPublic(t *testing.T) {
	id := models.ID("N-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{notes: map[models.ID]*models.Note{id: {ID: id, Kind: "note", Text: "текст", Private: true}}}

	_, err := New(repo).GetNote(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for private record with AccessPublic", err)
	}
}

func TestGetNotePrivateVisibleToFullAccess(t *testing.T) {
	id := models.ID("N-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{notes: map[models.ID]*models.Note{id: {ID: id, Kind: "note", Text: "текст", Private: true}}}

	got, err := New(repo).GetNote(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetNote: %v", err)
	}

	if got.Text != "текст" {
		t.Fatalf("Text = %q", got.Text)
	}
}
```

#### `internal/usecases/create_note/deps.go` (создать)
`internal/usecases/create_note/deps.go`:
```go
package create_note

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// NoteStore — зависимость сценария: транзакция порта store.Store. Проверка
// родителя и сохранение идут в одной транзакции на переданном fn хранилище.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type NoteStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
```

#### `internal/usecases/create_note/scenario.go` (создать)
`internal/usecases/create_note/scenario.go`:
```go
package create_note

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание заметки».
type Scenario struct {
	store NoteStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st NoteStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateNote создаёт заметку: генерирует идентификатор, проверяет
// инварианты, в одной транзакции (если родитель задан) убеждается в его
// существовании и сохраняет. Возвращает созданную заметку с заполненным ID.
// Цикл на создании невозможен (новый ID ещё нигде не встречается) — в
// отличие от UpdateNote, цепочка родителей не обходится, по образцу
// create_division.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта), невалидная сущность и несуществующий родитель —
// *models.ValidationError (поля id, parent_id); прочее — ошибки хранилища
// как есть.
func (s *Scenario) CreateNote(ctx context.Context, n models.Note) (models.Note, error) {
	if n.ID != "" {
		return models.Note{}, &models.ValidationError{
			Entity: models.TypeNote,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", n.ID),
		}
	}

	n.ID = s.ids.New(models.TypeNote)

	if err := n.Validate(); err != nil {
		return models.Note{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		if n.ParentID != nil {
			if _, err := tx.GetNote(ctx, *n.ParentID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return parentErr("родитель %q не найден", *n.ParentID)
				}

				return err
			}
		}

		return tx.SaveNote(ctx, &n)
	})
	if err != nil {
		return models.Note{}, err
	}

	return n, nil
}

// parentErr — *models.ValidationError по полю parent_id.
func parentErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeNote,
		Field:  "parent_id",
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/create_note/scenario_test.go` (создать)
`internal/usecases/create_note/scenario_test.go`:
```go
package create_note

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// nID возвращает корректный идентификатор заметки, отличающийся последним символом.
func nID(last byte) models.ID {
	return models.ID("N-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

type stubIDs struct {
	id      models.ID
	gotType models.Type
}

func (s *stubIDs) New(t models.Type) models.ID {
	s.gotType = t

	return s.id
}

// fakeTx реализует нужные сценарию методы store.Store поверх карты заметок;
// остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	notes   map[models.ID]*models.Note
	saved   []*models.Note
	getErr  error
	saveErr error
}

func newFakeTx(existing ...*models.Note) *fakeTx {
	tx := &fakeTx{notes: map[models.ID]*models.Note{}}
	for _, n := range existing {
		tx.notes[n.ID] = n
	}

	return tx
}

func (f *fakeTx) GetNote(_ context.Context, id models.ID) (*models.Note, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

	n, ok := f.notes[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *n

	return &cp, nil
}

func (f *fakeTx) SaveNote(_ context.Context, n *models.Note) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *n
	f.notes[n.ID] = &cp
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует NoteStore: InTx выполняет fn на встроенном fakeTx.
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

func validInput() models.Note {
	return models.Note{Kind: "note", Text: "текст заметки"}
}

func TestCreateNoteGeneratesIDAndSaves(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	ids := &stubIDs{id: nID('V')}

	got, err := New(st, ids).CreateNote(context.Background(), validInput())
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}

	if got.ID != nID('V') || got.Text != "текст заметки" {
		t.Fatalf("got %+v, ожидалась заметка с ID %v", got, nID('V'))
	}

	if ids.gotType != models.TypeNote {
		t.Errorf("генератор вызван с типом %q, ожидался note", ids.gotType)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].ID != nID('V') {
		t.Fatalf("InTx=%d, saved=%v; ожидалось одно сохранение внутри одной транзакции", st.calls, st.tx.saved)
	}
}

func TestCreateNoteRejectsExplicitID(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	ids := &stubIDs{id: nID('V')}

	in := validInput()
	in.ID = nID('0')

	_, err := New(st, ids).CreateNote(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю id", err)
	}

	if st.calls != 0 || ids.gotType != "" {
		t.Fatalf("InTx=%d, генератор вызван с %q; ожидалось: не вызваны", st.calls, ids.gotType)
	}
}

func TestCreateNoteValidatesBeforeTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	in := models.Note{Kind: "note"} // ни Title, ни Text

	_, err := New(st, &stubIDs{id: nID('V')}).CreateNote(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "text" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю text", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestCreateNoteWithParentSaves(t *testing.T) {
	parent := &models.Note{ID: nID('0'), Kind: "book", Title: "Книга"}
	st := &fakeStore{tx: newFakeTx(parent)}

	in := validInput()
	pid := parent.ID
	in.ParentID = &pid

	got, err := New(st, &stubIDs{id: nID('V')}).CreateNote(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}

	if got.ParentID == nil || *got.ParentID != parent.ID || len(st.tx.saved) != 1 {
		t.Fatalf("got %+v, saved=%d; ожидалось сохранение с родителем", got, len(st.tx.saved))
	}
}

func TestCreateNoteParentNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	in := validInput()
	pid := nID('0')
	in.ParentID = &pid

	_, err := New(st, &stubIDs{id: nID('V')}).CreateNote(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d заметок при несуществующем родителе", len(st.tx.saved))
	}
}

func TestCreateNotePropagatesSaveError(t *testing.T) {
	wantErr := errors.New("save failed")
	st := &fakeStore{tx: newFakeTx()}
	st.tx.saveErr = wantErr

	if _, err := New(st, &stubIDs{id: nID('V')}).CreateNote(context.Background(), validInput()); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

func TestCreateNotePropagatesTxError(t *testing.T) {
	wantErr := errors.New("tx failed")
	st := &fakeStore{tx: newFakeTx(), inTxErr: wantErr}

	if _, err := New(st, &stubIDs{id: nID('V')}).CreateNote(context.Background(), validInput()); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

func TestCreateNotePropagatesParentGetError(t *testing.T) {
	wantErr := errors.New("get failed")
	st := &fakeStore{tx: newFakeTx()}
	st.tx.getErr = wantErr

	in := validInput()
	pid := nID('0')
	in.ParentID = &pid

	_, err := New(st, &stubIDs{id: nID('V')}).CreateNote(context.Background(), in)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}

	var ve *models.ValidationError
	if errors.As(err, &ve) {
		t.Fatalf("сбой хранилища превращён в *ValidationError: %v", err)
	}
}
```

#### `internal/usecases/update_note/deps.go` (создать)
`internal/usecases/update_note/deps.go`:
```go
package update_note

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// NoteStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования, цепочки родителей и сохранение идут в одной
// транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type NoteStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
```

#### `internal/usecases/update_note/scenario.go` (создать)
`internal/usecases/update_note/scenario.go`:
```go
package update_note

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение заметки».
type Scenario struct {
	store NoteStore
}

// New создаёт сценарий.
func New(st NoteStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateNote полностью заменяет заметку по n.ID: проверяет инварианты, в
// одной транзакции убеждается, что заметка существует, а цепочка родителей
// не проходит через неё саму (цикл), и сохраняет.
//
// Ошибки: невалидная сущность, несуществующий родитель и цикл по parent_id —
// *models.ValidationError (соответствующее поле и parent_id); нет такой
// заметки — models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateNote(ctx context.Context, n models.Note) error {
	if err := n.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetNote(ctx, n.ID); err != nil {
			return err
		}

		if err := checkParentChain(ctx, tx, &n); err != nil {
			return err
		}

		return tx.SaveNote(ctx, &n)
	})
}

// checkParentChain обходит цепочку родителей n вверх: каждый предок должен
// существовать, и цепочка не должна проходить через саму заметку или
// замыкаться (защита от испорченных данных). Ошибка цепочки —
// *models.ValidationError по полю parent_id. По образцу
// update_division.checkParentChain.
func checkParentChain(ctx context.Context, tx store.Store, n *models.Note) error {
	seen := map[models.ID]bool{n.ID: true}

	for cur := n.ParentID; cur != nil; {
		if seen[*cur] {
			return parentErr("цепочка родителей %q проходит через саму заметку или замыкается на %q", n.ID, *cur)
		}
		seen[*cur] = true

		p, err := tx.GetNote(ctx, *cur)
		if err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return parentErr("родитель %q не найден", *cur)
			}

			return err
		}

		cur = p.ParentID
	}

	return nil
}

// parentErr — *models.ValidationError по полю parent_id.
func parentErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeNote,
		Field:  "parent_id",
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/update_note/scenario_test.go` (создать)
`internal/usecases/update_note/scenario_test.go`:
```go
package update_note

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// nID возвращает корректный идентификатор заметки, отличающийся последним символом.
func nID(last byte) models.ID {
	return models.ID("N-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// fakeTx реализует нужные сценарию методы store.Store поверх карты заметок;
// остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	notes   map[models.ID]*models.Note
	saved   []*models.Note
	saveErr error
	gets    int
}

func newFakeTx(existing ...*models.Note) *fakeTx {
	tx := &fakeTx{notes: map[models.ID]*models.Note{}}
	for _, n := range existing {
		tx.notes[n.ID] = n
	}

	return tx
}

func (f *fakeTx) GetNote(_ context.Context, id models.ID) (*models.Note, error) {
	f.gets++

	n, ok := f.notes[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *n

	return &cp, nil
}

func (f *fakeTx) SaveNote(_ context.Context, n *models.Note) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *n
	f.notes[n.ID] = &cp
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует NoteStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func note(id models.ID, parent *models.ID) *models.Note {
	return &models.Note{ID: id, Kind: "note", Text: "текст " + string(id), ParentID: parent}
}

func TestUpdateNoteSaves(t *testing.T) {
	existing := note(nID('V'), nil)
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Text = "новый текст"

	if err := New(st).UpdateNote(context.Background(), updated); err != nil {
		t.Fatalf("UpdateNote: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Text != "новый текст" {
		t.Fatalf("InTx=%d, saved=%v; ожидалось одно сохранение с новым текстом", st.calls, st.tx.saved)
	}
}

func TestUpdateNoteNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateNote(context.Background(), *note(nID('V'), nil))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d заметок при несуществующей заметке", len(st.tx.saved))
	}
}

func TestUpdateNoteValidatesBeforeTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	bad := *note(nID('V'), nil)
	bad.Text = ""
	bad.Title = ""

	err := New(st).UpdateNote(context.Background(), bad)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "text" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю text", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

// TestUpdateNoteParentNotFound: новый родитель не существует —
// *ValidationError по полю parent_id, ничего не сохраняется.
func TestUpdateNoteParentNotFound(t *testing.T) {
	existing := note(nID('V'), nil)
	st := &fakeStore{tx: newFakeTx(existing)}

	missing := nID('0')
	updated := *existing
	updated.ParentID = &missing

	err := New(st).UpdateNote(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d заметок при несуществующем родителе", len(st.tx.saved))
	}
}

// TestUpdateNoteDirectCycle: B — дочка A; попытка сделать A дочкой B — цикл,
// *ValidationError по полю parent_id.
func TestUpdateNoteDirectCycle(t *testing.T) {
	idA, idB := nID('A'), nID('B')
	a := note(idA, nil)
	b := note(idB, &idA)
	st := &fakeStore{tx: newFakeTx(a, b)}

	updated := *a
	updated.ParentID = &idB

	err := New(st).UpdateNote(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id (цикл)", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d заметок при цикле", len(st.tx.saved))
	}
}

// TestUpdateNoteLongCycle: цепочка C → B → A; попытка сделать A дочкой C —
// цикл длиной три, *ValidationError по полю parent_id.
func TestUpdateNoteLongCycle(t *testing.T) {
	idA, idB, idC := nID('A'), nID('B'), nID('C')
	a := note(idA, nil)
	b := note(idB, &idA)
	c := note(idC, &idB)
	st := &fakeStore{tx: newFakeTx(a, b, c)}

	updated := *a
	updated.ParentID = &idC

	err := New(st).UpdateNote(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id (цикл)", err)
	}
}

// TestUpdateNoteReparentOK: перенос под другого корректного родителя проходит.
func TestUpdateNoteReparentOK(t *testing.T) {
	idA, idB, idC := nID('A'), nID('B'), nID('C')
	a := note(idA, nil)
	b := note(idB, &idA)
	c := note(idC, nil)
	st := &fakeStore{tx: newFakeTx(a, b, c)}

	updated := *b
	updated.ParentID = &idC

	if err := New(st).UpdateNote(context.Background(), updated); err != nil {
		t.Fatalf("UpdateNote: %v", err)
	}

	if len(st.tx.saved) != 1 || *st.tx.saved[0].ParentID != idC {
		t.Fatalf("saved=%v; ожидалось сохранение с родителем %v", st.tx.saved, idC)
	}
}

// TestUpdateNoteWithoutParentSkipsWalk: без родителя цепочка не обходится —
// одно чтение (существование самой заметки).
func TestUpdateNoteWithoutParentSkipsWalk(t *testing.T) {
	existing := note(nID('V'), nil)
	st := &fakeStore{tx: newFakeTx(existing)}

	if err := New(st).UpdateNote(context.Background(), *existing); err != nil {
		t.Fatalf("UpdateNote: %v", err)
	}

	if st.tx.gets != 1 {
		t.Fatalf("чтений %d, ожидалось одно (без обхода родителей)", st.tx.gets)
	}
}

func TestUpdateNotePropagatesSaveError(t *testing.T) {
	wantErr := errors.New("save failed")
	existing := note(nID('V'), nil)
	st := &fakeStore{tx: newFakeTx(existing)}
	st.tx.saveErr = wantErr

	if err := New(st).UpdateNote(context.Background(), *existing); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}
```

#### `internal/usecases/delete_note/deps.go` (создать)
`internal/usecases/delete_note/deps.go`:
```go
package delete_note

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// NoteRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type NoteRepo interface {
	DeleteNote(ctx context.Context, id models.ID) error
}
```

#### `internal/usecases/delete_note/scenario.go` (создать)
`internal/usecases/delete_note/scenario.go`:
```go
package delete_note

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление заметки».
type Scenario struct {
	notes NoteRepo
}

// New создаёт сценарий.
func New(notes NoteRepo) *Scenario {
	return &Scenario{notes: notes}
}

// DeleteNote удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteNote(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.notes.DeleteNote(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeNote)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeNote,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

#### `internal/usecases/delete_note/scenario_test.go` (создать)
`internal/usecases/delete_note/scenario_test.go`:
```go
package delete_note

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

func (f *fakeRepo) DeleteNote(_ context.Context, id models.ID) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)

	return nil
}

func TestDeleteNoteCallsRepo(t *testing.T) {
	repo := &fakeRepo{}
	id := models.ID("N-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	if err := New(repo).DeleteNote(context.Background(), id); err != nil {
		t.Fatalf("DeleteNote: %v", err)
	}

	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("deleted = %v", repo.deleted)
	}
}

func TestDeleteNoteInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	err := New(repo).DeleteNote(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}

	if len(repo.deleted) != 0 {
		t.Fatalf("repo вызван при невалидном id")
	}
}

func TestDeleteNotePropagatesInUseError(t *testing.T) {
	wantErr := &models.InUseError{Type: models.TypeNote, ID: "N-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	repo := &fakeRepo{err: wantErr}

	err := New(repo).DeleteNote(context.Background(), "N-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	var iu *models.InUseError
	if !errors.As(err, &iu) {
		t.Fatalf("err = %v, want InUseError", err)
	}
}
```

#### `internal/httpapi/note.go` (создать)
`internal/httpapi/note.go`:
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleNoteList — GET /api/notes?limit=&offset=. Чтение открыто анонимному
// посетителю (приватные заметки скрыты — см. handleNoteGet).
func handleNoteList(notes NoteService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := notes.ListNotes(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.NotesFromModels(list))
	}
}

// handleNoteSearch — GET /api/notes/search?q=&limit=&offset=.
func handleNoteSearch(notes NoteService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := notes.SearchNotes(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.NotesFromModels(list))
	}
}

// handleNoteGet — GET /api/notes/{id}. Приватная заметка для анонимного или
// не-владельца — 404 (см. get_note.Scenario.GetNote,
// docs/data-model/entity-write.md §3.1).
func handleNoteGet(notes NoteService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n, err := notes.GetNote(r.Context(), AccessFromContext(r.Context()), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.NoteFromModel(n))
	}
}
```

#### `internal/httpapi/note_write.go` (создать)
`internal/httpapi/note_write.go`:
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleNoteCreate — POST /api/notes: создаёт запись, отвечает 201 с
// созданной записью (id генерирует сценарий). Несуществующий parent_id —
// 422 (та же механика, что и у делений/архивов). Запись — только для
// вошедшего владельца, см. handleDivisionCreate.
func handleNoteCreate(notes NoteService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.NoteCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := notes.CreateNote(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.NoteFromModel(created))
	}
}

// handleNoteUpdate — PUT /api/notes/{id}: полная замена
// kind/title/text/parent_id. Читает текущую версию, накладывает поля
// запроса (fetch-then-merge). Sources не в DTO — read-only в v1. Владелец
// всегда видит запись при fetch (requireFull даёт полный доступ).
func handleNoteUpdate(notes NoteService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.NoteUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := notes.GetNote(r.Context(), AccessFromContext(r.Context()), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Kind = m.Kind
		cur.Title = m.Title
		cur.Text = m.Text
		cur.ParentID = m.ParentID
		cur.Private = m.Private

		if err := notes.UpdateNote(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.NoteFromModel(cur))
	}
}

// handleNoteDelete — DELETE /api/notes/{id}: 204 без тела; занятая запись
// (есть дочерние заметки или другие ссылки) — 409 со списком ссылающихся.
// Запись — только для вошедшего владельца, см. handleDivisionCreate.
func handleNoteDelete(notes NoteService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := notes.DeleteNote(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
```

#### `internal/httpapi/note_test.go` (создать)
`internal/httpapi/note_test.go`:
```go
package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeNotes struct {
	list []models.Note
	err  error
	page models.Page

	getN      models.Note
	gotIDs    []models.ID
	created   models.Note
	gotCreate models.Note
	updated   models.Note
	deleteErr error

	search    []models.Note
	gotSearch models.SearchQuery

	gotAccess models.Access
}

func (f *fakeNotes) ListNotes(_ context.Context, _ models.Access, page models.Page) ([]models.Note, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeNotes) SearchNotes(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Note, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeNotes) GetNote(_ context.Context, access models.Access, id models.ID) (models.Note, error) {
	f.gotIDs = append(f.gotIDs, id)
	f.gotAccess = access
	if f.err != nil {
		return models.Note{}, f.err
	}

	return f.getN, nil
}

func (f *fakeNotes) CreateNote(_ context.Context, n models.Note) (models.Note, error) {
	f.gotCreate = n
	if f.err != nil {
		return models.Note{}, f.err
	}

	return f.created, nil
}

func (f *fakeNotes) UpdateNote(_ context.Context, n models.Note) error {
	f.updated = n

	return f.err
}

func (f *fakeNotes) DeleteNote(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestNoteListReturnsRecords(t *testing.T) {
	svc := &fakeNotes{list: []models.Note{{ID: "N-1", Kind: "note", Text: "текст"}}}

	rec := get(t, NewHandler(Deps{Notes: svc, DocsFS: fstest.MapFS{}}), "/api/notes")
	requireStatus(t, rec, 200)

	want := `[{"id":"N-1","kind":"note","text":"текст","sources":[],"private":false}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestNoteGetNotFound(t *testing.T) {
	svc := &fakeNotes{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Notes: svc, DocsFS: fstest.MapFS{}}), "/api/notes/N-1")
	requireStatus(t, rec, 404)
}

func TestNoteSearchPassesQuery(t *testing.T) {
	svc := &fakeNotes{}

	rec := get(t, NewHandler(Deps{Notes: svc, DocsFS: fstest.MapFS{}}), "/api/notes/search?q=текст")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "текст" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
```

#### `internal/httpapi/note_write_test.go` (создать)
`internal/httpapi/note_write_test.go`:
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

func TestNoteCreateContract(t *testing.T) {
	svc := &fakeNotes{created: models.Note{ID: "N-1", Kind: "note", Text: "текст"}}

	rec := postD(t, NewHandler(Deps{Notes: svc, DocsFS: fstest.MapFS{}}), "/api/notes",
		`{"kind":"note","text":"текст"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Kind != "note" || svc.gotCreate.Text != "текст" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestNoteCreatePassesPrivate — регресс: private изначально отсутствовал в
// NoteCreate DTO, из-за чего его нельзя было выставить при создании.
func TestNoteCreatePassesPrivate(t *testing.T) {
	svc := &fakeNotes{created: models.Note{ID: "N-1", Kind: "note", Text: "текст", Private: true}}

	rec := postD(t, NewHandler(Deps{Notes: svc, DocsFS: fstest.MapFS{}}), "/api/notes",
		`{"kind":"note","text":"текст","private":true}`)

	requireStatus(t, rec, http.StatusCreated)
	if !svc.gotCreate.Private {
		t.Fatalf("gotCreate.Private = %v, want true", svc.gotCreate.Private)
	}
}

func TestNoteCreateAnonymousIs401(t *testing.T) {
	svc := &fakeNotes{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Notes: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/notes", strings.NewReader(`{"kind":"note","text":"текст"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

// TestNoteUpdateMergesFields — включая parent_id, единственное поле у Note
// со строгой self-ref ссылкой.
func TestNoteUpdateMergesFields(t *testing.T) {
	svc := &fakeNotes{getN: models.Note{ID: "N-1", Kind: "note", Text: "текст"}}

	rec := putD(t, NewHandler(Deps{Notes: svc, DocsFS: fstest.MapFS{}}), "/api/notes/N-1",
		`{"kind":"note","text":"новый текст","parent_id":"N-2","private":true}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Text != "новый текст" || svc.updated.ID != "N-1" ||
		svc.updated.ParentID == nil || *svc.updated.ParentID != "N-2" ||
		!svc.updated.Private {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestNoteDeleteNoContent(t *testing.T) {
	svc := &fakeNotes{}

	rec := delD(t, NewHandler(Deps{Notes: svc, DocsFS: fstest.MapFS{}}), "/api/notes/N-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestNoteDeleteInUseIs409(t *testing.T) {
	svc := &fakeNotes{deleteErr: &models.InUseError{Type: models.TypeNote, ID: "N-1"}}

	rec := delD(t, NewHandler(Deps{Notes: svc, DocsFS: fstest.MapFS{}}), "/api/notes/N-1")

	requireStatus(t, rec, http.StatusConflict)
}
```

#### `internal/mcp/note.go` (создать)
`internal/mcp/note.go`:
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

// registerNoteTools регистрирует тулы для работы с заметками. parent_id —
// просто id родительской заметки (не объект, в отличие от church/parish's
// одиночного TextRef — self-ref строгая ссылка, как archive's
// repository_id): пустая строка — без родителя. note_create/note_update
// проверяют существование родителя и отсутствие циклов (по образцу
// division). Приватная заметка недоступна вызывающему без полного доступа —
// note_get отдаёт ошибку тула, как и List/Search её не покажут.
func registerNoteTools(s *server.MCPServer, notes NoteService) {
	tool := mcp.NewTool(
		"note_list",
		mcp.WithDescription("Список заметок в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, noteListHandler(notes))

	tool = mcp.NewTool(
		"note_search",
		mcp.WithDescription("Поиск заметок по началу заголовка или текста; результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало заголовка или текста")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, noteSearchHandler(notes))

	tool = mcp.NewTool(
		"note_get",
		mcp.WithDescription("Заметка по id; результат — JSON записи. Неверный формат id, отсутствующая или приватная (без полного доступа) запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например N-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, noteGetHandler(notes))

	tool = mcp.NewTool(
		"note_create",
		mcp.WithDescription("Создать заметку; id генерируется сервером; результат — JSON созданной записи. Нужен заголовок или текст. Несуществующий parent_id — ошибка тула"),
		mcp.WithString("kind", mcp.Required(), mcp.Description("Вид заметки: открытый список (note/article/book/chapter — типовые значения, допустимы и другие), формат [a-z][a-z0-9_-]*")),
		mcp.WithString("title", mcp.Description("Заголовок")),
		mcp.WithString("text", mcp.Description("Текст (markdown)")),
		mcp.WithString("parent_id", mcp.Description("id родительской заметки (необязательно; пусто — без родителя)")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, noteCreateHandler(notes))

	tool = mcp.NewTool(
		"note_update",
		mcp.WithDescription("Изменить заметку: полная замена kind/title/text/parent_id/private; результат — JSON обновлённой записи. Несуществующий parent_id или цикл в цепочке родителей — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("kind", mcp.Required(), mcp.Description("Вид заметки")),
		mcp.WithString("title", mcp.Description("Заголовок")),
		mcp.WithString("text", mcp.Description("Текст (markdown)")),
		mcp.WithString("parent_id", mcp.Description("id родительской заметки (необязательно)")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, noteUpdateHandler(notes))

	tool = mcp.NewTool(
		"note_delete",
		mcp.WithDescription("Удалить заметку. Необратимо. Если есть дочерние заметки или другие строгие ссылки — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, noteDeleteHandler(notes))
}

func noteListHandler(notes NoteService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := notes.ListNotes(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.NotesFromModels(list))
	}
}

func noteSearchHandler(notes NoteService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := notes.SearchNotes(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.NotesFromModels(list))
	}
}

func noteGetHandler(notes NoteService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		n, err := notes.GetNote(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.NoteFromModel(n))
	}
}

func noteCreateHandler(notes NoteService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		n := models.Note{
			Kind:    models.NoteKind(req.GetString("kind", "")),
			Title:   req.GetString("title", ""),
			Text:    req.GetString("text", ""),
			Private: req.GetBool("private", false),
		}

		if pid := req.GetString("parent_id", ""); pid != "" {
			id := models.ID(pid)
			n.ParentID = &id
		}

		created, err := notes.CreateNote(ctx, n)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.NoteFromModel(created))
	}
}

func noteUpdateHandler(notes NoteService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := notes.GetNote(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		cur.Kind = models.NoteKind(req.GetString("kind", ""))
		cur.Title = req.GetString("title", "")
		cur.Text = req.GetString("text", "")
		cur.Private = req.GetBool("private", false)

		if pid := req.GetString("parent_id", ""); pid != "" {
			pidID := models.ID(pid)
			cur.ParentID = &pidID
		} else {
			cur.ParentID = nil
		}

		if err := notes.UpdateNote(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.NoteFromModel(cur))
	}
}

func noteDeleteHandler(notes NoteService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := notes.DeleteNote(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
```

#### `internal/mcp/note_test.go` (создать)
`internal/mcp/note_test.go`:
```go
package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakeNotes struct {
	list []models.Note
	err  error

	getN      models.Note
	created   models.Note
	gotCreate models.Note
	updated   models.Note
	gotIDs    []models.ID
	deleteErr error

	search []models.Note
}

func (f *fakeNotes) ListNotes(context.Context, models.Access, models.Page) ([]models.Note, error) {
	return f.list, f.err
}

func (f *fakeNotes) SearchNotes(context.Context, models.Access, models.SearchQuery) ([]models.Note, error) {
	return f.search, f.err
}

func (f *fakeNotes) GetNote(_ context.Context, _ models.Access, id models.ID) (models.Note, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Note{}, f.err
	}

	return f.getN, nil
}

func (f *fakeNotes) CreateNote(_ context.Context, n models.Note) (models.Note, error) {
	f.gotCreate = n
	if f.err != nil {
		return models.Note{}, f.err
	}

	return f.created, nil
}

func (f *fakeNotes) UpdateNote(_ context.Context, n models.Note) error {
	f.updated = n

	return f.err
}

func (f *fakeNotes) DeleteNote(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callNoteTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestNoteGetToolContract(t *testing.T) {
	svc := &fakeNotes{getN: models.Note{ID: "N-1", Kind: "note", Text: "текст"}}

	res := callNoteTool(t, noteGetHandler(svc), map[string]any{"id": "N-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "N-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

// TestNoteCreateToolPassesParentID — parent_id передаётся плоской строкой
// (не объектом), в отличие от church/parish.
func TestNoteCreateToolPassesParentID(t *testing.T) {
	svc := &fakeNotes{created: models.Note{ID: "N-new", Kind: "note", Text: "текст"}}

	res := callNoteTool(t, noteCreateHandler(svc), map[string]any{
		"kind":      "note",
		"text":      "текст",
		"parent_id": "N-1",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.Text != "текст" || svc.gotCreate.ParentID == nil || *svc.gotCreate.ParentID != "N-1" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestNoteCreateToolPassesPrivate — регресс: private изначально отсутствовал
// среди аргументов note_create/note_update.
func TestNoteCreateToolPassesPrivate(t *testing.T) {
	svc := &fakeNotes{created: models.Note{ID: "N-new", Kind: "note", Text: "текст", Private: true}}

	res := callNoteTool(t, noteCreateHandler(svc), map[string]any{
		"kind":    "note",
		"text":    "текст",
		"private": true,
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if !svc.gotCreate.Private {
		t.Fatalf("gotCreate.Private = %v, want true", svc.gotCreate.Private)
	}
}

func TestNoteDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeNotes{deleteErr: &models.InUseError{Type: models.TypeNote, ID: "N-1"}}

	res := callNoteTool(t, noteDeleteHandler(svc), map[string]any{"id": "N-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersNoteTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Notes: &fakeNotes{}}).ListTools()

	for _, name := range []string{"note_list", "note_search", "note_get", "note_create", "note_update", "note_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
```

### Шаг 1.2. Attachment — транспорт, usecases, httpapi, MCP

#### `internal/transport/attachment.go` (создать)
`internal/transport/attachment.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// Attachment — контракт файлового вложения (GET /api/attachments, MCP-тул
// attachment_list). NodeID — обязательная строгая ссылка на архивный узел
// (просто id — ArchiveNode ещё не имеет CRUD, подпроект 6; существование
// проверяется в сценарии, generic-хранилище уже умеет читать любую сущность
// по id независимо от готовности её orchestration-слоя). DocumentID —
// необязательная мягкая ссылка на архивный документ (ON DELETE SET NULL в
// схеме — при удалении документа поле обнуляется автоматически, но
// существование при создании/изменении сценарий всё равно проверяет, как и
// для NodeID). Sources у Attachment нет (в отличие от Repository/Church/
// Parish/Archive/Note).
type Attachment struct {
	ID         models.ID `json:"id"`
	Kind       string    `json:"kind"`
	URI        string    `json:"uri,omitempty"`
	Filename   string    `json:"filename,omitempty"`
	MIME       string    `json:"mime,omitempty"`
	Page       int       `json:"page,omitempty"`
	NodeID     string    `json:"node_id"`
	DocumentID string    `json:"document_id,omitempty"`
	Note       string    `json:"note,omitempty"`
	Private    bool      `json:"private"`
}

// AttachmentFromModel конвертирует запись в контракт.
func AttachmentFromModel(a models.Attachment) Attachment {
	var documentID string
	if a.DocumentID != nil {
		documentID = string(*a.DocumentID)
	}

	return Attachment{
		ID:         a.ID,
		Kind:       string(a.Kind),
		URI:        a.URI,
		Filename:   a.Filename,
		MIME:       a.MIME,
		Page:       a.Page,
		NodeID:     string(a.NodeID),
		DocumentID: documentID,
		Note:       a.Note,
		Private:    a.Private,
	}
}

// AttachmentsFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func AttachmentsFromModels(as []models.Attachment) []Attachment {
	out := make([]Attachment, 0, len(as))
	for _, a := range as {
		out = append(out, AttachmentFromModel(a))
	}

	return out
}
```

#### `internal/transport/attachment_write.go` (создать)
`internal/transport/attachment_write.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// AttachmentCreate — тело POST /api/attachments и аргументы тула
// attachment_create. Идентификатор генерирует сценарий. NodeID обязателен;
// DocumentID — просто id, пустая строка — не задан. Сценарий проверяет
// существование обоих (если заданы).
type AttachmentCreate struct {
	Kind       string `json:"kind"`
	URI        string `json:"uri,omitempty"`
	Filename   string `json:"filename,omitempty"`
	MIME       string `json:"mime,omitempty"`
	Page       int    `json:"page,omitempty"`
	NodeID     string `json:"node_id"`
	DocumentID string `json:"document_id,omitempty"`
	Note       string `json:"note,omitempty"`
	Private    bool   `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (a AttachmentCreate) Model() models.Attachment {
	var documentID *models.ID
	if a.DocumentID != "" {
		id := models.ID(a.DocumentID)
		documentID = &id
	}

	return models.Attachment{
		Kind:       models.AttachmentKind(a.Kind),
		URI:        a.URI,
		Filename:   a.Filename,
		MIME:       a.MIME,
		Page:       a.Page,
		NodeID:     models.ID(a.NodeID),
		DocumentID: documentID,
		Note:       a.Note,
		Private:    a.Private,
	}
}

// AttachmentUpdate — тело PUT /api/attachments/{id} и аргументы тула
// attachment_update: полная замена kind/uri/filename/mime/page/node_id/
// document_id/note/private.
type AttachmentUpdate struct {
	Kind       string `json:"kind"`
	URI        string `json:"uri,omitempty"`
	Filename   string `json:"filename,omitempty"`
	MIME       string `json:"mime,omitempty"`
	Page       int    `json:"page,omitempty"`
	NodeID     string `json:"node_id"`
	DocumentID string `json:"document_id,omitempty"`
	Note       string `json:"note,omitempty"`
	Private    bool   `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (a AttachmentUpdate) Model() models.Attachment {
	var documentID *models.ID
	if a.DocumentID != "" {
		id := models.ID(a.DocumentID)
		documentID = &id
	}

	return models.Attachment{
		Kind:       models.AttachmentKind(a.Kind),
		URI:        a.URI,
		Filename:   a.Filename,
		MIME:       a.MIME,
		Page:       a.Page,
		NodeID:     models.ID(a.NodeID),
		DocumentID: documentID,
		Note:       a.Note,
		Private:    a.Private,
	}
}
```

#### `internal/usecases/list_attachments/deps.go` (создать)
`internal/usecases/list_attachments/deps.go`:
```go
package list_attachments

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// AttachmentRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type AttachmentRepo interface {
	ListAttachments(ctx context.Context, access models.Access, page models.Page) ([]*models.Attachment, error)
}
```

#### `internal/usecases/list_attachments/scenario.go` (создать)
`internal/usecases/list_attachments/scenario.go`:
```go
package list_attachments

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список файловых вложений».
type Scenario struct {
	attachments AttachmentRepo
}

// New создаёт сценарий.
func New(attachments AttachmentRepo) *Scenario {
	return &Scenario{attachments: attachments}
}

// ListAttachments возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListAttachments(ctx context.Context, access models.Access, page models.Page) ([]models.Attachment, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeAttachment, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeAttachment, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.attachments.ListAttachments(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Attachment, 0, len(list))
	for _, sn := range list {
		out = append(out, *sn)
	}

	return out, nil
}
```

#### `internal/usecases/list_attachments/scenario_test.go` (создать)
`internal/usecases/list_attachments/scenario_test.go`:
```go
package list_attachments

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.Attachment
}

func (f *fakeRepo) ListAttachments(_ context.Context, _ models.Access, page models.Page) ([]*models.Attachment, error) {
	f.page = page

	return f.out, nil
}

func TestListAttachmentsReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.Attachment{{ID: "O-01ARZ3NDEKTSV4RRFFQ69G5FA1", Kind: models.AttachmentKindScan, Filename: "0012.jpg"}}}

	got, err := New(repo).ListAttachments(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListAttachments: %v", err)
	}

	if len(got) != 1 || got[0].Filename != "0012.jpg" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListAttachmentsRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListAttachments(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
```

#### `internal/usecases/search_attachments/deps.go` (создать)
`internal/usecases/search_attachments/deps.go`:
```go
package search_attachments

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// AttachmentRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type AttachmentRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetAttachment(ctx context.Context, id models.ID) (*models.Attachment, error)
}
```

#### `internal/usecases/search_attachments/scenario.go` (создать)
`internal/usecases/search_attachments/scenario.go`:
```go
package search_attachments

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск файловых вложений».
type Scenario struct {
	attachments AttachmentRepo
}

// New создаёт сценарий.
func New(attachments AttachmentRepo) *Scenario {
	return &Scenario{attachments: attachments}
}

// SearchAttachments находит вложения, чьи имя файла или заметка начинаются
// с текста запроса — та же механика, что и
// search_divisions.SearchDivisions (см. её комментарий).
func (s *Scenario) SearchAttachments(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Attachment, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Attachment{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Attachment{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.attachments.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeAttachment {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.attachments.GetAttachment(ctx, h.ID)
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

#### `internal/usecases/search_attachments/scenario_test.go` (создать)
`internal/usecases/search_attachments/scenario_test.go`:
```go
package search_attachments

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits        []models.Hit
	attachments map[models.ID]*models.Attachment
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetAttachment(_ context.Context, id models.ID) (*models.Attachment, error) {
	s, ok := f.attachments[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestSearchAttachmentsFiltersByType(t *testing.T) {
	id := models.ID("O-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeAttachment, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не attachment — должен быть пропущен
		},
		attachments: map[models.ID]*models.Attachment{id: {ID: id, Kind: models.AttachmentKindScan, Filename: "0012.jpg"}},
	}

	got, err := New(repo).SearchAttachments(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchAttachments: %v", err)
	}

	if len(got) != 1 || got[0].Filename != "0012.jpg" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchAttachmentsEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchAttachments(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchAttachments: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
```

#### `internal/usecases/get_attachment/deps.go` (создать)
`internal/usecases/get_attachment/deps.go`:
```go
package get_attachment

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// AttachmentRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type AttachmentRepo interface {
	GetAttachment(ctx context.Context, id models.ID) (*models.Attachment, error)
}
```

#### `internal/usecases/get_attachment/scenario.go` (создать)
`internal/usecases/get_attachment/scenario.go`:
```go
package get_attachment

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «файловое вложение по идентификатору».
type Scenario struct {
	attachments AttachmentRepo
}

// New создаёт сценарий.
func New(attachments AttachmentRepo) *Scenario {
	return &Scenario{attachments: attachments}
}

// GetAttachment возвращает вложение по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такого вложения — models.ErrNotFound. Приватное вложение
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound (docs/data-model/entity-write.md §3.1).
func (s *Scenario) GetAttachment(ctx context.Context, access models.Access, id models.ID) (models.Attachment, error) {
	if err := validateID(id); err != nil {
		return models.Attachment{}, err
	}

	a, err := s.attachments.GetAttachment(ctx, id)
	if err != nil {
		return models.Attachment{}, err
	}

	if a.Private && access != models.AccessFull {
		return models.Attachment{}, models.ErrNotFound
	}

	return *a, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeAttachment)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeAttachment,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

#### `internal/usecases/get_attachment/scenario_test.go` (создать)
`internal/usecases/get_attachment/scenario_test.go`:
```go
package get_attachment

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	attachments map[models.ID]*models.Attachment
}

func (f *fakeRepo) GetAttachment(_ context.Context, id models.ID) (*models.Attachment, error) {
	a, ok := f.attachments[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return a, nil
}

func TestGetAttachmentReturnsRecord(t *testing.T) {
	id := models.ID("O-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{attachments: map[models.ID]*models.Attachment{id: {ID: id, Kind: models.AttachmentKindScan, Filename: "0012.jpg"}}}

	got, err := New(repo).GetAttachment(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetAttachment: %v", err)
	}

	if got.Filename != "0012.jpg" {
		t.Fatalf("Filename = %q", got.Filename)
	}
}

func TestGetAttachmentNotFound(t *testing.T) {
	repo := &fakeRepo{attachments: map[models.ID]*models.Attachment{}}

	_, err := New(repo).GetAttachment(context.Background(), models.AccessFull, "O-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetAttachmentInvalidID(t *testing.T) {
	repo := &fakeRepo{attachments: map[models.ID]*models.Attachment{}}

	_, err := New(repo).GetAttachment(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestGetAttachmentPrivateHiddenFromPublic(t *testing.T) {
	id := models.ID("O-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{attachments: map[models.ID]*models.Attachment{id: {ID: id, Kind: models.AttachmentKindScan, Filename: "0012.jpg", Private: true}}}

	_, err := New(repo).GetAttachment(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for private record with AccessPublic", err)
	}
}

func TestGetAttachmentPrivateVisibleToFullAccess(t *testing.T) {
	id := models.ID("O-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{attachments: map[models.ID]*models.Attachment{id: {ID: id, Kind: models.AttachmentKindScan, Filename: "0012.jpg", Private: true}}}

	got, err := New(repo).GetAttachment(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetAttachment: %v", err)
	}

	if got.Filename != "0012.jpg" {
		t.Fatalf("Filename = %q", got.Filename)
	}
}
```

#### `internal/usecases/create_attachment/deps.go` (создать)
`internal/usecases/create_attachment/deps.go`:
```go
package create_attachment

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// AttachmentStore — зависимость сценария: транзакция порта store.Store.
// Проверка узла, документа (если задан) и сохранение идут в одной
// транзакции на переданном fn хранилище.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type AttachmentStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
```

#### `internal/usecases/create_attachment/scenario.go` (создать)
`internal/usecases/create_attachment/scenario.go`:
```go
package create_attachment

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание файлового вложения».
type Scenario struct {
	store AttachmentStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st AttachmentStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateAttachment создаёт вложение: генерирует идентификатор, проверяет
// инварианты, в одной транзакции убеждается в существовании узла (всегда —
// NodeID обязателен) и документа (если задан) и сохраняет. Возвращает
// созданное вложение с заполненным ID. NodeID/DocumentID ссылаются на
// ArchiveNode/ArchiveDocument, у которых ещё нет своего CRUD-слоя
// (подпроект 6) — generic-хранилище уже умеет проверять их существование по
// id независимо от готовности orchestration-слоя цели.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта), невалидная сущность, несуществующий узел или документ —
// *models.ValidationError (поля id, node_id, document_id); прочее — ошибки
// хранилища как есть.
func (s *Scenario) CreateAttachment(ctx context.Context, a models.Attachment) (models.Attachment, error) {
	if a.ID != "" {
		return models.Attachment{}, &models.ValidationError{
			Entity: models.TypeAttachment,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", a.ID),
		}
	}

	a.ID = s.ids.New(models.TypeAttachment)

	if err := a.Validate(); err != nil {
		return models.Attachment{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetArchiveNode(ctx, a.NodeID); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return nodeErr("узел %q не найден", a.NodeID)
			}

			return err
		}

		if a.DocumentID != nil {
			if _, err := tx.GetArchiveDocument(ctx, *a.DocumentID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return documentErr("документ %q не найден", *a.DocumentID)
				}

				return err
			}
		}

		return tx.SaveAttachment(ctx, &a)
	})
	if err != nil {
		return models.Attachment{}, err
	}

	return a, nil
}

// nodeErr — *models.ValidationError по полю node_id.
func nodeErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeAttachment,
		Field:  "node_id",
		Reason: fmt.Sprintf(format, args...),
	}
}

// documentErr — *models.ValidationError по полю document_id.
func documentErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeAttachment,
		Field:  "document_id",
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/create_attachment/scenario_test.go` (создать)
`internal/usecases/create_attachment/scenario_test.go`:
```go
package create_attachment

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// oID возвращает корректный идентификатор вложения, отличающийся последним символом.
func oID(last byte) models.ID {
	return models.ID("O-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func nodeID(last byte) models.ID {
	return models.ID("AN-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func docID(last byte) models.ID {
	return models.ID("DC-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

type stubIDs struct {
	id      models.ID
	gotType models.Type
}

func (s *stubIDs) New(t models.Type) models.ID {
	s.gotType = t

	return s.id
}

// fakeTx реализует нужные сценарию методы store.Store поверх карт узлов и
// документов; остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	nodes     map[models.ID]*models.ArchiveNode
	documents map[models.ID]*models.ArchiveDocument
	saved     []*models.Attachment
	getErr    error
	saveErr   error
}

func newFakeTx(nodeIDs, docIDs []models.ID) *fakeTx {
	tx := &fakeTx{nodes: map[models.ID]*models.ArchiveNode{}, documents: map[models.ID]*models.ArchiveDocument{}}
	for _, id := range nodeIDs {
		tx.nodes[id] = &models.ArchiveNode{ID: id}
	}
	for _, id := range docIDs {
		tx.documents[id] = &models.ArchiveDocument{ID: id}
	}

	return tx
}

func (f *fakeTx) GetArchiveNode(_ context.Context, id models.ID) (*models.ArchiveNode, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

	n, ok := f.nodes[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return n, nil
}

func (f *fakeTx) GetArchiveDocument(_ context.Context, id models.ID) (*models.ArchiveDocument, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

	d, ok := f.documents[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return d, nil
}

func (f *fakeTx) SaveAttachment(_ context.Context, a *models.Attachment) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *a
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует AttachmentStore: InTx выполняет fn на встроенном fakeTx.
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

func validInput(node models.ID) models.Attachment {
	return models.Attachment{Kind: models.AttachmentKindScan, Filename: "0012.jpg", NodeID: node}
}

func TestCreateAttachmentGeneratesIDAndSaves(t *testing.T) {
	node := nodeID('0')
	st := &fakeStore{tx: newFakeTx([]models.ID{node}, nil)}
	ids := &stubIDs{id: oID('V')}

	got, err := New(st, ids).CreateAttachment(context.Background(), validInput(node))
	if err != nil {
		t.Fatalf("CreateAttachment: %v", err)
	}

	if got.ID != oID('V') || got.Filename != "0012.jpg" {
		t.Fatalf("got %+v, ожидалось вложение с ID %v", got, oID('V'))
	}

	if ids.gotType != models.TypeAttachment {
		t.Errorf("генератор вызван с типом %q, ожидался attachment", ids.gotType)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].ID != oID('V') {
		t.Fatalf("InTx=%d, saved=%v; ожидалось одно сохранение внутри одной транзакции", st.calls, st.tx.saved)
	}
}

func TestCreateAttachmentRejectsExplicitID(t *testing.T) {
	node := nodeID('0')
	st := &fakeStore{tx: newFakeTx([]models.ID{node}, nil)}
	ids := &stubIDs{id: oID('V')}

	in := validInput(node)
	in.ID = oID('0')

	_, err := New(st, ids).CreateAttachment(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю id", err)
	}

	if st.calls != 0 || ids.gotType != "" {
		t.Fatalf("InTx=%d, генератор вызван с %q; ожидалось: не вызваны", st.calls, ids.gotType)
	}
}

func TestCreateAttachmentValidatesBeforeTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx(nil, nil)}

	in := models.Attachment{Kind: models.AttachmentKindScan, NodeID: nodeID('0')} // ни uri, ни filename

	_, err := New(st, &stubIDs{id: oID('V')}).CreateAttachment(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "uri" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю uri", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestCreateAttachmentNodeNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx(nil, nil)}

	_, err := New(st, &stubIDs{id: oID('V')}).CreateAttachment(context.Background(), validInput(nodeID('0')))

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "node_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю node_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d вложений при несуществующем узле", len(st.tx.saved))
	}
}

func TestCreateAttachmentWithDocumentSaves(t *testing.T) {
	node, doc := nodeID('0'), docID('0')
	st := &fakeStore{tx: newFakeTx([]models.ID{node}, []models.ID{doc})}

	in := validInput(node)
	in.DocumentID = &doc

	got, err := New(st, &stubIDs{id: oID('V')}).CreateAttachment(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateAttachment: %v", err)
	}

	if got.DocumentID == nil || *got.DocumentID != doc || len(st.tx.saved) != 1 {
		t.Fatalf("got %+v, saved=%d; ожидалось сохранение с документом", got, len(st.tx.saved))
	}
}

func TestCreateAttachmentDocumentNotFound(t *testing.T) {
	node := nodeID('0')
	st := &fakeStore{tx: newFakeTx([]models.ID{node}, nil)}

	in := validInput(node)
	missingDoc := docID('9')
	in.DocumentID = &missingDoc

	_, err := New(st, &stubIDs{id: oID('V')}).CreateAttachment(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "document_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю document_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d вложений при несуществующем документе", len(st.tx.saved))
	}
}

func TestCreateAttachmentPropagatesSaveError(t *testing.T) {
	node := nodeID('0')
	wantErr := errors.New("save failed")
	st := &fakeStore{tx: newFakeTx([]models.ID{node}, nil)}
	st.tx.saveErr = wantErr

	if _, err := New(st, &stubIDs{id: oID('V')}).CreateAttachment(context.Background(), validInput(node)); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

func TestCreateAttachmentPropagatesTxError(t *testing.T) {
	node := nodeID('0')
	wantErr := errors.New("tx failed")
	st := &fakeStore{tx: newFakeTx([]models.ID{node}, nil), inTxErr: wantErr}

	if _, err := New(st, &stubIDs{id: oID('V')}).CreateAttachment(context.Background(), validInput(node)); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

func TestCreateAttachmentPropagatesNodeGetError(t *testing.T) {
	wantErr := errors.New("get failed")
	st := &fakeStore{tx: newFakeTx(nil, nil)}
	st.tx.getErr = wantErr

	_, err := New(st, &stubIDs{id: oID('V')}).CreateAttachment(context.Background(), validInput(nodeID('0')))
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}

	var ve *models.ValidationError
	if errors.As(err, &ve) {
		t.Fatalf("сбой хранилища превращён в *ValidationError: %v", err)
	}
}
```

#### `internal/usecases/update_attachment/deps.go` (создать)
`internal/usecases/update_attachment/deps.go`:
```go
package update_attachment

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// AttachmentStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования, узла, документа (если задан) и сохранение идут в
// одной транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type AttachmentStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
```

#### `internal/usecases/update_attachment/scenario.go` (создать)
`internal/usecases/update_attachment/scenario.go`:
```go
package update_attachment

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение файлового вложения».
type Scenario struct {
	store AttachmentStore
}

// New создаёт сценарий.
func New(st AttachmentStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateAttachment полностью заменяет вложение по a.ID: проверяет
// инварианты, в одной транзакции убеждается, что вложение существует, узел
// существует (документ — если задан), и сохраняет.
//
// Ошибки: невалидная сущность, несуществующий узел или документ —
// *models.ValidationError (соответствующее поле, node_id, document_id); нет
// такого вложения — models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateAttachment(ctx context.Context, a models.Attachment) error {
	if err := a.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetAttachment(ctx, a.ID); err != nil {
			return err
		}

		if _, err := tx.GetArchiveNode(ctx, a.NodeID); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return nodeErr("узел %q не найден", a.NodeID)
			}

			return err
		}

		if a.DocumentID != nil {
			if _, err := tx.GetArchiveDocument(ctx, *a.DocumentID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return documentErr("документ %q не найден", *a.DocumentID)
				}

				return err
			}
		}

		return tx.SaveAttachment(ctx, &a)
	})
}

// nodeErr — *models.ValidationError по полю node_id.
func nodeErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeAttachment,
		Field:  "node_id",
		Reason: fmt.Sprintf(format, args...),
	}
}

// documentErr — *models.ValidationError по полю document_id.
func documentErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeAttachment,
		Field:  "document_id",
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/update_attachment/scenario_test.go` (создать)
`internal/usecases/update_attachment/scenario_test.go`:
```go
package update_attachment

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

func oID(last byte) models.ID {
	return models.ID("O-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func nodeID(last byte) models.ID {
	return models.ID("AN-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func docID(last byte) models.ID {
	return models.ID("DC-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// fakeTx реализует нужные сценарию методы store.Store поверх карт вложений,
// узлов и документов; остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	attachments map[models.ID]*models.Attachment
	nodes       map[models.ID]*models.ArchiveNode
	documents   map[models.ID]*models.ArchiveDocument
	saved       []*models.Attachment
}

func newFakeTx(existing []*models.Attachment, nodeIDs, docIDs []models.ID) *fakeTx {
	tx := &fakeTx{
		attachments: map[models.ID]*models.Attachment{},
		nodes:       map[models.ID]*models.ArchiveNode{},
		documents:   map[models.ID]*models.ArchiveDocument{},
	}
	for _, a := range existing {
		tx.attachments[a.ID] = a
	}
	for _, id := range nodeIDs {
		tx.nodes[id] = &models.ArchiveNode{ID: id}
	}
	for _, id := range docIDs {
		tx.documents[id] = &models.ArchiveDocument{ID: id}
	}

	return tx
}

func (f *fakeTx) GetAttachment(_ context.Context, id models.ID) (*models.Attachment, error) {
	a, ok := f.attachments[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *a

	return &cp, nil
}

func (f *fakeTx) GetArchiveNode(_ context.Context, id models.ID) (*models.ArchiveNode, error) {
	n, ok := f.nodes[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return n, nil
}

func (f *fakeTx) GetArchiveDocument(_ context.Context, id models.ID) (*models.ArchiveDocument, error) {
	d, ok := f.documents[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return d, nil
}

func (f *fakeTx) SaveAttachment(_ context.Context, a *models.Attachment) error {
	cp := *a
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует AttachmentStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func attachment(id, node models.ID) *models.Attachment {
	return &models.Attachment{ID: id, Kind: models.AttachmentKindScan, Filename: "0012.jpg", NodeID: node}
}

func TestUpdateAttachmentSaves(t *testing.T) {
	node := nodeID('0')
	existing := attachment(oID('V'), node)
	st := &fakeStore{tx: newFakeTx([]*models.Attachment{existing}, []models.ID{node}, nil)}

	updated := *existing
	updated.Filename = "0013.jpg"

	if err := New(st).UpdateAttachment(context.Background(), updated); err != nil {
		t.Fatalf("UpdateAttachment: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Filename != "0013.jpg" {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateAttachmentNotFound(t *testing.T) {
	node := nodeID('0')
	st := &fakeStore{tx: newFakeTx(nil, []models.ID{node}, nil)}

	err := New(st).UpdateAttachment(context.Background(), *attachment(oID('V'), node))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateAttachmentRejectsEmptyURIAndFilename(t *testing.T) {
	st := &fakeStore{tx: newFakeTx(nil, nil, nil)}

	err := New(st).UpdateAttachment(context.Background(), models.Attachment{ID: oID('V'), Kind: models.AttachmentKindScan, NodeID: nodeID('0')})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "uri" {
		t.Fatalf("err = %v, want ValidationError on uri", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestUpdateAttachmentNodeNotFound(t *testing.T) {
	node := nodeID('0')
	existing := attachment(oID('V'), node)
	st := &fakeStore{tx: newFakeTx([]*models.Attachment{existing}, nil, nil)}

	err := New(st).UpdateAttachment(context.Background(), *existing)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "node_id" {
		t.Fatalf("err = %v, want ValidationError on node_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d вложений при несуществующем узле", len(st.tx.saved))
	}
}

func TestUpdateAttachmentWithDocumentSaves(t *testing.T) {
	node, doc := nodeID('0'), docID('0')
	existing := attachment(oID('V'), node)
	st := &fakeStore{tx: newFakeTx([]*models.Attachment{existing}, []models.ID{node}, []models.ID{doc})}

	updated := *existing
	updated.DocumentID = &doc

	if err := New(st).UpdateAttachment(context.Background(), updated); err != nil {
		t.Fatalf("UpdateAttachment: %v", err)
	}

	if len(st.tx.saved) != 1 || st.tx.saved[0].DocumentID == nil || *st.tx.saved[0].DocumentID != doc {
		t.Fatalf("saved = %v", st.tx.saved)
	}
}

func TestUpdateAttachmentDocumentNotFound(t *testing.T) {
	node := nodeID('0')
	existing := attachment(oID('V'), node)
	st := &fakeStore{tx: newFakeTx([]*models.Attachment{existing}, []models.ID{node}, nil)}

	missingDoc := docID('9')
	updated := *existing
	updated.DocumentID = &missingDoc

	err := New(st).UpdateAttachment(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "document_id" {
		t.Fatalf("err = %v, want ValidationError on document_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d вложений при несуществующем документе", len(st.tx.saved))
	}
}
```

#### `internal/usecases/delete_attachment/deps.go` (создать)
`internal/usecases/delete_attachment/deps.go`:
```go
package delete_attachment

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// AttachmentRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type AttachmentRepo interface {
	DeleteAttachment(ctx context.Context, id models.ID) error
}
```

#### `internal/usecases/delete_attachment/scenario.go` (создать)
`internal/usecases/delete_attachment/scenario.go`:
```go
package delete_attachment

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление файлового вложения».
type Scenario struct {
	attachments AttachmentRepo
}

// New создаёт сценарий.
func New(attachments AttachmentRepo) *Scenario {
	return &Scenario{attachments: attachments}
}

// DeleteAttachment удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteAttachment(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.attachments.DeleteAttachment(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeAttachment)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeAttachment,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

#### `internal/usecases/delete_attachment/scenario_test.go` (создать)
`internal/usecases/delete_attachment/scenario_test.go`:
```go
package delete_attachment

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

func (f *fakeRepo) DeleteAttachment(_ context.Context, id models.ID) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)

	return nil
}

func TestDeleteAttachmentCallsRepo(t *testing.T) {
	repo := &fakeRepo{}
	id := models.ID("O-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	if err := New(repo).DeleteAttachment(context.Background(), id); err != nil {
		t.Fatalf("DeleteAttachment: %v", err)
	}

	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("deleted = %v", repo.deleted)
	}
}

func TestDeleteAttachmentInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	err := New(repo).DeleteAttachment(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}

	if len(repo.deleted) != 0 {
		t.Fatalf("repo вызван при невалидном id")
	}
}

func TestDeleteAttachmentPropagatesInUseError(t *testing.T) {
	wantErr := &models.InUseError{Type: models.TypeAttachment, ID: "O-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	repo := &fakeRepo{err: wantErr}

	err := New(repo).DeleteAttachment(context.Background(), "O-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	var iu *models.InUseError
	if !errors.As(err, &iu) {
		t.Fatalf("err = %v, want InUseError", err)
	}
}
```

#### `internal/httpapi/attachment.go` (создать)
`internal/httpapi/attachment.go`:
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleAttachmentList — GET /api/attachments?limit=&offset=. Чтение открыто
// анонимному посетителю (приватные вложения скрыты — см. handleAttachmentGet).
func handleAttachmentList(attachments AttachmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := attachments.ListAttachments(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.AttachmentsFromModels(list))
	}
}

// handleAttachmentSearch — GET /api/attachments/search?q=&limit=&offset=.
func handleAttachmentSearch(attachments AttachmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := attachments.SearchAttachments(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.AttachmentsFromModels(list))
	}
}

// handleAttachmentGet — GET /api/attachments/{id}. Приватное вложение для
// анонимного или не-владельца — 404.
func handleAttachmentGet(attachments AttachmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a, err := attachments.GetAttachment(r.Context(), AccessFromContext(r.Context()), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.AttachmentFromModel(a))
	}
}
```

#### `internal/httpapi/attachment_write.go` (создать)
`internal/httpapi/attachment_write.go`:
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleAttachmentCreate — POST /api/attachments: создаёт запись, отвечает
// 201 с созданной записью (id генерирует сценарий). Несуществующий node_id
// или document_id — 422. Запись — только для вошедшего владельца, см.
// handleDivisionCreate.
func handleAttachmentCreate(attachments AttachmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.AttachmentCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := attachments.CreateAttachment(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.AttachmentFromModel(created))
	}
}

// handleAttachmentUpdate — PUT /api/attachments/{id}: полная замена
// kind/uri/filename/mime/page/node_id/document_id/note/private. Читает
// текущую версию, накладывает поля запроса (fetch-then-merge).
func handleAttachmentUpdate(attachments AttachmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.AttachmentUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := attachments.GetAttachment(r.Context(), AccessFromContext(r.Context()), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Kind = m.Kind
		cur.URI = m.URI
		cur.Filename = m.Filename
		cur.MIME = m.MIME
		cur.Page = m.Page
		cur.NodeID = m.NodeID
		cur.DocumentID = m.DocumentID
		cur.Note = m.Note
		cur.Private = m.Private

		if err := attachments.UpdateAttachment(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.AttachmentFromModel(cur))
	}
}

// handleAttachmentDelete — DELETE /api/attachments/{id}: 204 без тела;
// занятая запись — 409 со списком ссылающихся. Запись — только для
// вошедшего владельца, см. handleDivisionCreate.
func handleAttachmentDelete(attachments AttachmentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := attachments.DeleteAttachment(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
```

#### `internal/httpapi/attachment_test.go` (создать)
`internal/httpapi/attachment_test.go`:
```go
package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeAttachments struct {
	list []models.Attachment
	err  error
	page models.Page

	getA      models.Attachment
	gotIDs    []models.ID
	created   models.Attachment
	gotCreate models.Attachment
	updated   models.Attachment
	deleteErr error

	search    []models.Attachment
	gotSearch models.SearchQuery
}

func (f *fakeAttachments) ListAttachments(_ context.Context, _ models.Access, page models.Page) ([]models.Attachment, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeAttachments) SearchAttachments(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Attachment, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeAttachments) GetAttachment(_ context.Context, _ models.Access, id models.ID) (models.Attachment, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Attachment{}, f.err
	}

	return f.getA, nil
}

func (f *fakeAttachments) CreateAttachment(_ context.Context, a models.Attachment) (models.Attachment, error) {
	f.gotCreate = a
	if f.err != nil {
		return models.Attachment{}, f.err
	}

	return f.created, nil
}

func (f *fakeAttachments) UpdateAttachment(_ context.Context, a models.Attachment) error {
	f.updated = a

	return f.err
}

func (f *fakeAttachments) DeleteAttachment(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestAttachmentListReturnsRecords(t *testing.T) {
	svc := &fakeAttachments{list: []models.Attachment{{ID: "O-1", Kind: models.AttachmentKindScan, Filename: "0012.jpg", NodeID: "AN-1"}}}

	rec := get(t, NewHandler(Deps{Attachments: svc, DocsFS: fstest.MapFS{}}), "/api/attachments")
	requireStatus(t, rec, 200)

	want := `[{"id":"O-1","kind":"scan","filename":"0012.jpg","node_id":"AN-1","private":false}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestAttachmentGetNotFound(t *testing.T) {
	svc := &fakeAttachments{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Attachments: svc, DocsFS: fstest.MapFS{}}), "/api/attachments/O-1")
	requireStatus(t, rec, 404)
}

func TestAttachmentSearchPassesQuery(t *testing.T) {
	svc := &fakeAttachments{}

	rec := get(t, NewHandler(Deps{Attachments: svc, DocsFS: fstest.MapFS{}}), "/api/attachments/search?q=0012")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "0012" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
```

#### `internal/httpapi/attachment_write_test.go` (создать)
`internal/httpapi/attachment_write_test.go`:
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

func TestAttachmentCreateContract(t *testing.T) {
	svc := &fakeAttachments{created: models.Attachment{ID: "O-1", Kind: models.AttachmentKindScan, Filename: "0012.jpg", NodeID: "AN-1"}}

	rec := postD(t, NewHandler(Deps{Attachments: svc, DocsFS: fstest.MapFS{}}), "/api/attachments",
		`{"kind":"scan","filename":"0012.jpg","node_id":"AN-1"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Kind != models.AttachmentKindScan || svc.gotCreate.NodeID != "AN-1" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestAttachmentCreateNodeNotFoundIs422 — usecase-слой возвращает
// *models.ValidationError по полю node_id при несуществующем узле,
// writeError мапит это на 422.
func TestAttachmentCreateNodeNotFoundIs422(t *testing.T) {
	svc := &fakeAttachments{err: &models.ValidationError{Entity: models.TypeAttachment, Field: "node_id", Reason: "узел не найден"}}

	rec := postD(t, NewHandler(Deps{Attachments: svc, DocsFS: fstest.MapFS{}}), "/api/attachments",
		`{"kind":"scan","filename":"0012.jpg","node_id":"AN-999"}`)

	requireStatus(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"node_id"`) {
		t.Fatalf("body = %s, ожидалось поле node_id", rec.Body)
	}
}

func TestAttachmentCreateAnonymousIs401(t *testing.T) {
	svc := &fakeAttachments{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Attachments: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/attachments", strings.NewReader(`{"kind":"scan","filename":"0012.jpg","node_id":"AN-1"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestAttachmentUpdateMergesFields(t *testing.T) {
	svc := &fakeAttachments{getA: models.Attachment{ID: "O-1", Kind: models.AttachmentKindScan, Filename: "0012.jpg", NodeID: "AN-1"}}

	rec := putD(t, NewHandler(Deps{Attachments: svc, DocsFS: fstest.MapFS{}}), "/api/attachments/O-1",
		`{"kind":"photo","filename":"0013.jpg","node_id":"AN-1","document_id":"DC-1","private":true}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Kind != models.AttachmentKindPhoto || svc.updated.Filename != "0013.jpg" ||
		svc.updated.DocumentID == nil || *svc.updated.DocumentID != "DC-1" ||
		svc.updated.Private != true || svc.updated.ID != "O-1" {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestAttachmentDeleteNoContent(t *testing.T) {
	svc := &fakeAttachments{}

	rec := delD(t, NewHandler(Deps{Attachments: svc, DocsFS: fstest.MapFS{}}), "/api/attachments/O-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestAttachmentDeleteInUseIs409(t *testing.T) {
	svc := &fakeAttachments{deleteErr: &models.InUseError{Type: models.TypeAttachment, ID: "O-1"}}

	rec := delD(t, NewHandler(Deps{Attachments: svc, DocsFS: fstest.MapFS{}}), "/api/attachments/O-1")

	requireStatus(t, rec, http.StatusConflict)
}
```

#### `internal/mcp/attachment.go` (создать)
`internal/mcp/attachment.go`:
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

// registerAttachmentTools регистрирует тулы для работы с файловыми
// вложениями. node_id/document_id — просто id (не объекты): node_id
// обязателен, document_id — необязателен (пусто — не задан).
// attachment_create/attachment_update проверяют существование обоих
// (document_id — если задан); ArchiveNode/ArchiveDocument ещё не имеют
// своего CRUD-слоя (подпроект 6), но generic-хранилище уже умеет проверять
// существование любого id.
func registerAttachmentTools(s *server.MCPServer, attachments AttachmentService) {
	tool := mcp.NewTool(
		"attachment_list",
		mcp.WithDescription("Список файловых вложений в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, attachmentListHandler(attachments))

	tool = mcp.NewTool(
		"attachment_search",
		mcp.WithDescription("Поиск вложений по началу имени файла или заметки; результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало имени файла или заметки")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, attachmentSearchHandler(attachments))

	tool = mcp.NewTool(
		"attachment_get",
		mcp.WithDescription("Вложение по id; результат — JSON записи. Неверный формат id, отсутствующая или приватная (без полного доступа) запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например O-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, attachmentGetHandler(attachments))

	tool = mcp.NewTool(
		"attachment_create",
		mcp.WithDescription("Создать вложение; id генерируется сервером; результат — JSON созданной записи. Нужен uri или filename. Несуществующий node_id или document_id — ошибка тула"),
		mcp.WithString("kind", mcp.Required(), mcp.Enum("scan", "document", "audio", "photo"), mcp.Description("Вид вложения")),
		mcp.WithString("uri", mcp.Description("URI (файл, ссылка)")),
		mcp.WithString("filename", mcp.Description("Имя файла")),
		mcp.WithString("mime", mcp.Description("MIME-тип (тип/подтип)")),
		mcp.WithNumber("page", mcp.Description("Номер страницы (0 — не указана)")),
		mcp.WithString("node_id", mcp.Required(), mcp.Description("id архивного узла (ArchiveNode)")),
		mcp.WithString("document_id", mcp.Description("id архивного документа (ArchiveDocument), необязательно")),
		mcp.WithString("note", mcp.Description("Заметка")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, attachmentCreateHandler(attachments))

	tool = mcp.NewTool(
		"attachment_update",
		mcp.WithDescription("Изменить вложение: полная замена kind/uri/filename/mime/page/node_id/document_id/note/private; результат — JSON обновлённой записи. Несуществующий node_id или document_id — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("kind", mcp.Required(), mcp.Enum("scan", "document", "audio", "photo"), mcp.Description("Вид вложения")),
		mcp.WithString("uri", mcp.Description("URI")),
		mcp.WithString("filename", mcp.Description("Имя файла")),
		mcp.WithString("mime", mcp.Description("MIME-тип")),
		mcp.WithNumber("page", mcp.Description("Номер страницы")),
		mcp.WithString("node_id", mcp.Required(), mcp.Description("id архивного узла")),
		mcp.WithString("document_id", mcp.Description("id архивного документа, необязательно")),
		mcp.WithString("note", mcp.Description("Заметка")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, attachmentUpdateHandler(attachments))

	tool = mcp.NewTool(
		"attachment_delete",
		mcp.WithDescription("Удалить вложение. Необратимо. Если на него есть строгие ссылки от других сущностей — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, attachmentDeleteHandler(attachments))
}

func attachmentListHandler(attachments AttachmentService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := attachments.ListAttachments(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.AttachmentsFromModels(list))
	}
}

func attachmentSearchHandler(attachments AttachmentService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := attachments.SearchAttachments(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.AttachmentsFromModels(list))
	}
}

func attachmentGetHandler(attachments AttachmentService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		a, err := attachments.GetAttachment(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.AttachmentFromModel(a))
	}
}

func attachmentCreateHandler(attachments AttachmentService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page, err := optionalInt(req, "page")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		a := models.Attachment{
			Kind:     models.AttachmentKind(req.GetString("kind", "")),
			URI:      req.GetString("uri", ""),
			Filename: req.GetString("filename", ""),
			MIME:     req.GetString("mime", ""),
			Page:     page,
			NodeID:   models.ID(req.GetString("node_id", "")),
			Note:     req.GetString("note", ""),
			Private:  req.GetBool("private", false),
		}

		if did := req.GetString("document_id", ""); did != "" {
			id := models.ID(did)
			a.DocumentID = &id
		}

		created, err := attachments.CreateAttachment(ctx, a)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.AttachmentFromModel(created))
	}
}

func attachmentUpdateHandler(attachments AttachmentService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := attachments.GetAttachment(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		page, err := optionalInt(req, "page")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cur.Kind = models.AttachmentKind(req.GetString("kind", ""))
		cur.URI = req.GetString("uri", "")
		cur.Filename = req.GetString("filename", "")
		cur.MIME = req.GetString("mime", "")
		cur.Page = page
		cur.NodeID = models.ID(req.GetString("node_id", ""))
		cur.Note = req.GetString("note", "")
		cur.Private = req.GetBool("private", false)

		if did := req.GetString("document_id", ""); did != "" {
			didID := models.ID(did)
			cur.DocumentID = &didID
		} else {
			cur.DocumentID = nil
		}

		if err := attachments.UpdateAttachment(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.AttachmentFromModel(cur))
	}
}

func attachmentDeleteHandler(attachments AttachmentService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := attachments.DeleteAttachment(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
```

#### `internal/mcp/attachment_test.go` (создать)
`internal/mcp/attachment_test.go`:
```go
package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakeAttachments struct {
	list []models.Attachment
	err  error

	getA      models.Attachment
	created   models.Attachment
	gotCreate models.Attachment
	updated   models.Attachment
	gotIDs    []models.ID
	deleteErr error

	search []models.Attachment
}

func (f *fakeAttachments) ListAttachments(context.Context, models.Access, models.Page) ([]models.Attachment, error) {
	return f.list, f.err
}

func (f *fakeAttachments) SearchAttachments(context.Context, models.Access, models.SearchQuery) ([]models.Attachment, error) {
	return f.search, f.err
}

func (f *fakeAttachments) GetAttachment(_ context.Context, _ models.Access, id models.ID) (models.Attachment, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Attachment{}, f.err
	}

	return f.getA, nil
}

func (f *fakeAttachments) CreateAttachment(_ context.Context, a models.Attachment) (models.Attachment, error) {
	f.gotCreate = a
	if f.err != nil {
		return models.Attachment{}, f.err
	}

	return f.created, nil
}

func (f *fakeAttachments) UpdateAttachment(_ context.Context, a models.Attachment) error {
	f.updated = a

	return f.err
}

func (f *fakeAttachments) DeleteAttachment(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callAttachmentTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestAttachmentGetToolContract(t *testing.T) {
	svc := &fakeAttachments{getA: models.Attachment{ID: "O-1", Kind: models.AttachmentKindScan, Filename: "0012.jpg"}}

	res := callAttachmentTool(t, attachmentGetHandler(svc), map[string]any{"id": "O-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "O-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

// TestAttachmentCreateToolPassesNodeID — node_id/document_id передаются
// плоскими строками (не объектами).
func TestAttachmentCreateToolPassesNodeID(t *testing.T) {
	svc := &fakeAttachments{created: models.Attachment{ID: "O-new", Kind: models.AttachmentKindScan, Filename: "0012.jpg"}}

	res := callAttachmentTool(t, attachmentCreateHandler(svc), map[string]any{
		"kind":        "scan",
		"filename":    "0012.jpg",
		"node_id":     "AN-1",
		"document_id": "DC-1",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.NodeID != "AN-1" || svc.gotCreate.DocumentID == nil || *svc.gotCreate.DocumentID != "DC-1" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

func TestAttachmentCreateToolNodeNotFoundIsError(t *testing.T) {
	svc := &fakeAttachments{err: &models.ValidationError{Entity: models.TypeAttachment, Field: "node_id", Reason: "не найден"}}

	res := callAttachmentTool(t, attachmentCreateHandler(svc), map[string]any{
		"kind":     "scan",
		"filename": "0012.jpg",
		"node_id":  "AN-999",
	})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestAttachmentDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeAttachments{deleteErr: &models.InUseError{Type: models.TypeAttachment, ID: "O-1"}}

	res := callAttachmentTool(t, attachmentDeleteHandler(svc), map[string]any{"id": "O-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersAttachmentTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Attachments: &fakeAttachments{}}).ListTools()

	for _, name := range []string{"attachment_list", "attachment_search", "attachment_get", "attachment_create", "attachment_update", "attachment_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
```

### Шаг 1.3. `Deps`-реестр: подключить оба сервиса

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
	Notes        NoteService
	Attachments  AttachmentService
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

	if deps.Notes != nil {
		registerNoteRoutes(mux, deps.Notes)
	}

	if deps.Attachments != nil {
		registerAttachmentRoutes(mux, deps.Attachments)
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

	if deps.Notes != nil {
		registerNoteRoutes(mux, deps.Notes)
	}

	if deps.Attachments != nil {
		registerAttachmentRoutes(mux, deps.Attachments)
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
	Notes        NoteService
	Attachments  AttachmentService
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

	if deps.Notes != nil {
		registerNoteTools(s, deps.Notes)
	}

	if deps.Attachments != nil {
		registerAttachmentTools(s, deps.Attachments)
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
	create_attachment "github.com/amarin/genodex/internal/usecases/create_attachment"
	create_church "github.com/amarin/genodex/internal/usecases/create_church"
	create_division "github.com/amarin/genodex/internal/usecases/create_division"
	create_estate "github.com/amarin/genodex/internal/usecases/create_estate"
	create_given_name "github.com/amarin/genodex/internal/usecases/create_given_name"
	create_note "github.com/amarin/genodex/internal/usecases/create_note"
	create_parish "github.com/amarin/genodex/internal/usecases/create_parish"
	create_patronymic "github.com/amarin/genodex/internal/usecases/create_patronymic"
	create_repository "github.com/amarin/genodex/internal/usecases/create_repository"
	create_surname "github.com/amarin/genodex/internal/usecases/create_surname"
	create_title "github.com/amarin/genodex/internal/usecases/create_title"
	delete_archive "github.com/amarin/genodex/internal/usecases/delete_archive"
	delete_attachment "github.com/amarin/genodex/internal/usecases/delete_attachment"
	delete_church "github.com/amarin/genodex/internal/usecases/delete_church"
	delete_division "github.com/amarin/genodex/internal/usecases/delete_division"
	delete_estate "github.com/amarin/genodex/internal/usecases/delete_estate"
	delete_given_name "github.com/amarin/genodex/internal/usecases/delete_given_name"
	delete_note "github.com/amarin/genodex/internal/usecases/delete_note"
	delete_parish "github.com/amarin/genodex/internal/usecases/delete_parish"
	delete_patronymic "github.com/amarin/genodex/internal/usecases/delete_patronymic"
	delete_repository "github.com/amarin/genodex/internal/usecases/delete_repository"
	delete_surname "github.com/amarin/genodex/internal/usecases/delete_surname"
	delete_title "github.com/amarin/genodex/internal/usecases/delete_title"
	get_archive "github.com/amarin/genodex/internal/usecases/get_archive"
	get_attachment "github.com/amarin/genodex/internal/usecases/get_attachment"
	get_church "github.com/amarin/genodex/internal/usecases/get_church"
	get_division "github.com/amarin/genodex/internal/usecases/get_division"
	get_estate "github.com/amarin/genodex/internal/usecases/get_estate"
	get_given_name "github.com/amarin/genodex/internal/usecases/get_given_name"
	get_note "github.com/amarin/genodex/internal/usecases/get_note"
	get_parish "github.com/amarin/genodex/internal/usecases/get_parish"
	get_patronymic "github.com/amarin/genodex/internal/usecases/get_patronymic"
	get_repository "github.com/amarin/genodex/internal/usecases/get_repository"
	get_surname "github.com/amarin/genodex/internal/usecases/get_surname"
	get_title "github.com/amarin/genodex/internal/usecases/get_title"
	list_archives "github.com/amarin/genodex/internal/usecases/list_archives"
	list_attachments "github.com/amarin/genodex/internal/usecases/list_attachments"
	list_churches "github.com/amarin/genodex/internal/usecases/list_churches"
	list_divisions "github.com/amarin/genodex/internal/usecases/list_divisions"
	list_estates "github.com/amarin/genodex/internal/usecases/list_estates"
	list_given_names "github.com/amarin/genodex/internal/usecases/list_given_names"
	list_notes "github.com/amarin/genodex/internal/usecases/list_notes"
	list_parishes "github.com/amarin/genodex/internal/usecases/list_parishes"
	list_patronymics "github.com/amarin/genodex/internal/usecases/list_patronymics"
	list_repositories "github.com/amarin/genodex/internal/usecases/list_repositories"
	list_surnames "github.com/amarin/genodex/internal/usecases/list_surnames"
	list_titles "github.com/amarin/genodex/internal/usecases/list_titles"
	search_archives "github.com/amarin/genodex/internal/usecases/search_archives"
	search_attachments "github.com/amarin/genodex/internal/usecases/search_attachments"
	search_churches "github.com/amarin/genodex/internal/usecases/search_churches"
	search_divisions "github.com/amarin/genodex/internal/usecases/search_divisions"
	search_estates "github.com/amarin/genodex/internal/usecases/search_estates"
	search_given_names "github.com/amarin/genodex/internal/usecases/search_given_names"
	search_notes "github.com/amarin/genodex/internal/usecases/search_notes"
	search_parishes "github.com/amarin/genodex/internal/usecases/search_parishes"
	search_patronymics "github.com/amarin/genodex/internal/usecases/search_patronymics"
	search_repositories "github.com/amarin/genodex/internal/usecases/search_repositories"
	search_surnames "github.com/amarin/genodex/internal/usecases/search_surnames"
	search_titles "github.com/amarin/genodex/internal/usecases/search_titles"
	update_archive "github.com/amarin/genodex/internal/usecases/update_archive"
	update_attachment "github.com/amarin/genodex/internal/usecases/update_attachment"
	update_church "github.com/amarin/genodex/internal/usecases/update_church"
	update_division "github.com/amarin/genodex/internal/usecases/update_division"
	update_estate "github.com/amarin/genodex/internal/usecases/update_estate"
	update_given_name "github.com/amarin/genodex/internal/usecases/update_given_name"
	update_note "github.com/amarin/genodex/internal/usecases/update_note"
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
	_ httpapi.NoteService       = (*noteService)(nil)
	_ mcp.NoteService           = (*noteService)(nil)
	_ httpapi.AttachmentService = (*attachmentService)(nil)
	_ mcp.AttachmentService     = (*attachmentService)(nil)
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

	// auth-хранилище — на том же соединении, что и общий store (см.
	// sqlstore.Store.DB), файл БД один и тот же (internal/storage/schema_auth.go).
	authService := auth.New(auth.NewSQLStore(st.DB()))

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcp.RequireAPIToken(authService)(server.NewStreamableHTTPServer(
		mcp.NewServer(mcp.Deps{
			Divisions: divisions, Surnames: surnames,
			Patronymics: patronymics, Estates: estates, Titles: titles, GivenNames: givenNames,
			Repositories: repositories, Churches: churches, Parishes: parishes, Archives: archives,
			Notes:       notes,
			Attachments: attachments,
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
		Notes:        notes,
		Attachments:  attachments,
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

### Шаг 1.4. Рубеж

`gofmt -l .` пусто; `go build ./...`, `go vet ./...`, `go test ./...` (весь репозиторий) зелёные — ожидается 987 тестов, 88 пакетов. Живой смок через `curl` (см. «Предпосылка») — не обязателен повторно, но приветствуется: создать `Note` с `parent_id`, проверить 422 на несуществующий `parent_id` и на цикл при `update`, проверить 404 анонимно на приватную заметку; создать `Attachment`, проверить 422 на несуществующий `node_id`.

### Шаг 1.5. Коммит

```bash
git add \
  internal/transport/note.go internal/transport/note_write.go \
  internal/transport/attachment.go internal/transport/attachment_write.go \
  internal/usecases/list_notes internal/usecases/search_notes internal/usecases/get_note internal/usecases/create_note internal/usecases/update_note internal/usecases/delete_note \
  internal/usecases/list_attachments internal/usecases/search_attachments internal/usecases/get_attachment internal/usecases/create_attachment internal/usecases/update_attachment internal/usecases/delete_attachment \
  internal/httpapi/note.go internal/httpapi/note_write.go internal/httpapi/note_test.go internal/httpapi/note_write_test.go \
  internal/httpapi/attachment.go internal/httpapi/attachment_write.go internal/httpapi/attachment_test.go internal/httpapi/attachment_write_test.go \
  internal/mcp/note.go internal/mcp/note_test.go internal/mcp/attachment.go internal/mcp/attachment_test.go \
  internal/httpapi/deps.go internal/httpapi/api.go internal/httpapi/httpapi.go internal/mcp/deps.go internal/mcp/server.go internal/app/app.go
git commit -m "feat(backend): Note/Attachment — полный CRUD (usecases/httpapi/mcp) + Deps-реестр"
```

## Задача 2. Веб: страницы Note

**Интерфейсы, потребляемые из Задачи 1**: `httpapi`-маршрут `/api/notes` (JSON-форма — `transport.Note`/`NoteCreate`/`NoteUpdate`, включая `parent_id`/`private`).
**Интерфейсы, потребляемые из подпроектов 1-3**: `authFetch`/`ApiError` (`web/src/auth.ts`), `PageLayout`/каталог сущностей, `MAX_PAGE_LIMIT` (`web/src/api.ts`).

**Файлы:**
- Создать: `web/src/pages/{NotesList,NoteForm,NoteView}.tsx`
- Изменить: `web/src/api.ts`, `web/src/App.tsx`, `web/src/pages/EntityCatalog.tsx`

**Важно для исполнителя**: `parent_id` — `Select` с поиском, список заполняется `fetchNotes({ limit: 500 })` (по образцу `useRepositoryOptions` у `ArchiveForm.tsx`), в `NoteView.tsx` редактируемая заметка исключается из списка вариантов (`n.id !== id`) — обход собственного цикла сценарий всё равно проверит, но так короче путь до ошибки. `Sources` — read-only, показывается в `Descriptions` при просмотре, без редактора (Citation ещё без CRUD, подпроект 5).

### Шаг 2.1. `web/src/api.ts` — типы и функции Note

Добавить в конец файла (после блока Archive из подпроекта 3):
`web/src/api.ts (фрагмент)`:
```typescript
// Note — заметка (markdown-текст с иерархией «книга → главы»). parent_id —
// просто id родительской заметки (не TextRef — строгая self-ref ссылка, как
// у Archive.repository_id); пустая строка — без родителя. sources — read-only
// в v1 (Citation ещё без CRUD, подпроект 5).
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
```

### Шаг 2.2. Страницы `List`/`Form`/`View`

#### `web/src/pages/NoteForm.tsx` (создать)
`web/src/pages/NoteForm.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Alert, Checkbox, Form, Input, Modal, Select } from "antd";
import { createNote, fetchNotes, type Note } from "../api";
import { ApiError } from "../auth";

interface NoteFormValues {
  kind: string;
  title?: string;
  text?: string;
  parent_id?: string;
  private?: boolean;
}

const FORM_FIELDS: (keyof NoteFormValues)[] = ["kind", "title", "text", "parent_id"];

// useNoteOptions — заполняет Select заметок для parent_id: плоский список +
// Select (не Tree — Note не показывается иерархией в v1, обсуждение
// подпроекта 4), по образцу useRepositoryOptions у Archive.
function useNoteOptions(excludeID?: string) {
  const [notes, setNotes] = useState<Note[]>([]);

  useEffect(() => {
    fetchNotes({ limit: 500 })
      .then(setNotes)
      .catch(() => setNotes([]));
  }, []);

  return notes
    .filter((n) => n.id !== excludeID)
    .map((n) => ({ value: n.id, label: n.title || n.text?.slice(0, 60) || n.id }));
}

// CreateNoteModal — форма создания заметки. parent_id — Select со списком
// заметок (существование и отсутствие циклов проверяет сценарий, при
// создании цикл невозможен — новая запись ещё ничьим предком быть не может).
export function CreateNoteModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Note) => void;
}) {
  const [form] = Form.useForm<NoteFormValues>();
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const noteOptions = useNoteOptions();

  const reset = () => {
    form.resetFields();
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: NoteFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await createNote({
        kind: values.kind,
        title: values.title,
        text: values.text,
        parent_id: values.parent_id ?? "",
        private: values.private ?? false,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof NoteFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить заметку"
      open={open}
      onCancel={handleClose}
      onOk={() => form.submit()}
      okText="Создать"
      cancelText="Отмена"
      confirmLoading={submitting}
      destroyOnHidden
    >
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
      <Form form={form} layout="vertical" onFinish={onFinish} initialValues={{ kind: "note", private: false }}>
        <Form.Item
          name="kind"
          label="Вид"
          rules={[{ required: true, whitespace: true, message: "Введите вид (например, note)" }]}
        >
          <Input placeholder="note / article / book / chapter" autoFocus />
        </Form.Item>
        <Form.Item name="title" label="Заголовок">
          <Input />
        </Form.Item>
        <Form.Item name="text" label="Текст (markdown)">
          <Input.TextArea rows={6} />
        </Form.Item>
        <Form.Item name="parent_id" label="Родительская заметка">
          <Select
            allowClear
            placeholder="Не выбрана"
            options={noteOptions}
            showSearch
            filterOption={(input, option) =>
              (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
            }
          />
        </Form.Item>
        <Form.Item name="private" valuePropName="checked">
          <Checkbox>Приватная запись</Checkbox>
        </Form.Item>
      </Form>
    </Modal>
  );
}
```

#### `web/src/pages/NotesList.tsx` (создать)
`web/src/pages/NotesList.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Input, List, Spin } from "antd";
import { fetchNotes, searchNotes, MAX_PAGE_LIMIT, type Note } from "../api";
import { useSession } from "../session";
import { CreateNoteModal } from "./NoteForm";

function noteLabel(n: Note): string {
  return n.title || n.text?.slice(0, 80) || n.id;
}

// NotesList — «Заметки»: плоский список (иерархия parent_id не показывается
// деревом в v1, обсуждение подпроекта 4) — та же пагинация-до-короткой-
// страницы и поиск-подменяет-список, что у ArchivesList.
export default function NotesList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<Note[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<Note[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: Note[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchNotes({ limit: MAX_PAGE_LIMIT, offset });
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
    searchNotes({ q, limit: MAX_PAGE_LIMIT })
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
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "Заметки" }]}
      />
      <Card
        title="Заметки"
        extra={
          session != null ? (
            <Button type="primary" onClick={() => setCreateOpen(true)}>
              + добавить
            </Button>
          ) : undefined
        }
      >
        <Input.Search
          placeholder="Поиск по заголовку или тексту…"
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
            renderItem={(n) => (
              <List.Item>
                <Link to={`/notes/${n.id}`}>{noteLabel(n)}</Link>
              </List.Item>
            )}
          />
        )}
        <CreateNoteModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(n) => {
            setCreateOpen(false);
            navigate(`/notes/${n.id}`);
          }}
        />
      </Card>
    </>
  );
}
```

#### `web/src/pages/NoteView.tsx` (создать)
`web/src/pages/NoteView.tsx`:
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
import { deleteNote, fetchNote, fetchNotes, updateNote, type Note, type SourceLink } from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";

interface EditFormValues {
  kind: string;
  title?: string;
  text?: string;
  parent_id?: string;
  private?: boolean;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["kind", "title", "text", "parent_id"];

function noteLabel(n: Note): string {
  return n.title || n.text?.slice(0, 80) || n.id;
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

// NoteView — просмотр заметки, переключаемый в форму редактирования на той
// же странице (toggle+explicit-save). parent_id — Select со списком заметок
// (fetchNotes), сама заметка исключена из списка (нельзя выбрать себя же
// родителем — сценарий всё равно проверит цикл, но так короче путь до
// ошибки).
export default function NoteView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [note, setNote] = useState<Note | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notes, setNotes] = useState<Note[]>([]);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (noteId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setNote(null);
    fetchNote(noteId)
      .then(setNote)
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
    fetchNotes({ limit: 500 })
      .then(setNotes)
      .catch(() => setNotes([]));
  }, []);

  const noteOptions = notes
    .filter((n) => n.id !== id)
    .map((n) => ({ value: n.id, label: noteLabel(n) }));
  const parentLabel = (parentID?: string) => {
    const parent = notes.find((n) => n.id === parentID);
    return parent != null ? noteLabel(parent) : parentID;
  };

  const startEdit = () => {
    if (note == null) {
      return;
    }
    form.setFieldsValue({
      kind: note.kind,
      title: note.title ?? "",
      text: note.text ?? "",
      parent_id: note.parent_id ?? undefined,
      private: note.private,
    });
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (note == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await updateNote(note.id, {
        kind: values.kind,
        title: values.title,
        text: values.text,
        parent_id: values.parent_id ?? "",
        private: values.private ?? false,
      });
      setNote(updated);
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
    if (note == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteNote(note.id);
      navigate("/notes");
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
          <Link to="/notes">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (note == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/notes">Заметки</Link> },
          { title: noteLabel(note) },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={noteLabel(note)} column={1} bordered size="small">
            <Descriptions.Item label="Вид">{note.kind}</Descriptions.Item>
            <Descriptions.Item label="Заголовок">{note.title || "—"}</Descriptions.Item>
            <Descriptions.Item label="Текст">
              <Typography.Paragraph style={{ whiteSpace: "pre-wrap", marginBottom: 0 }}>
                {note.text || "—"}
              </Typography.Paragraph>
            </Descriptions.Item>
            <Descriptions.Item label="Родительская заметка">
              {note.parent_id ? (
                <Link to={`/notes/${note.parent_id}`}>{parentLabel(note.parent_id)}</Link>
              ) : (
                "—"
              )}
            </Descriptions.Item>
            <Descriptions.Item label="Доказательства"><SourceLinkListView items={note.sources} /></Descriptions.Item>
            <Descriptions.Item label="Приватная">{note.private ? "да" : "нет"}</Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title={`Удалить «${noteLabel(note)}»?`}
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
            name="kind"
            label="Вид"
            rules={[{ required: true, whitespace: true, message: "Введите вид" }]}
          >
            <Input placeholder="note / article / book / chapter" />
          </Form.Item>
          <Form.Item name="title" label="Заголовок">
            <Input />
          </Form.Item>
          <Form.Item name="text" label="Текст (markdown)">
            <Input.TextArea rows={6} />
          </Form.Item>
          <Form.Item name="parent_id" label="Родительская заметка">
            <Select
              allowClear
              placeholder="Не выбрана"
              options={noteOptions}
              showSearch
              filterOption={(input, option) =>
                (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
              }
            />
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

### Шаг 2.3. `web/src/App.tsx` — роуты Note, `EntityCatalog.tsx` — строка «Заметки»

Добавить импорты в `web/src/App.tsx` (после импортов Archive из подпроекта 3):
```typescript
import NotesList from "./pages/NotesList";
import NoteView from "./pages/NoteView";
```
Добавить роуты (после роутов `/archives`/`/archives/:id`, до `/login`):
```typescript
          <Route path="/notes" element={<PageLayout><NotesList /></PageLayout>} />
          <Route path="/notes/:id" element={<PageLayout><NoteView /></PageLayout>} />
```

В массиве `CATALOG_ENTRIES` (`web/src/pages/EntityCatalog.tsx`) добавить:
```typescript
  { label: "Заметки", path: "/notes" },
```

### Шаг 2.4. Рубеж

`npm run typecheck` и `npm run build` в `web/` чисты. Живая проверка в браузере (см. «Предпосылка»): `/notes` → «+ добавить» → заголовок + текст → создание → View с хлебными крошками; вторая заметка с `parent_id` первой (`Select` с поиском) + «Приватная запись» → создание → View показывает родителя-ссылку и «Приватная: да»; поиск по заголовку в списке корректно фильтрует до совпадающей записи.

### Шаг 2.5. Коммит

```bash
git add web/src/api.ts web/src/App.tsx web/src/pages/EntityCatalog.tsx \
  web/src/pages/NotesList.tsx web/src/pages/NoteForm.tsx web/src/pages/NoteView.tsx
git commit -m "feat(web): страницы Заметок — список, просмотр/редактирование, форма"
```

## Задача 3. Веб: страницы Attachment

**Интерфейсы, потребляемые из Задачи 1**: `httpapi`-маршрут `/api/attachments` (JSON-форма — `transport.Attachment`/`AttachmentCreate`/`AttachmentUpdate`, включая `node_id`/`document_id`/`private`).
**Интерфейсы, потребляемые из подпроектов 1-3**: то же, что Задача 2 — `authFetch`/`ApiError`, `PageLayout`/каталог, `MAX_PAGE_LIMIT`.

**Файлы:**
- Создать: `web/src/pages/{AttachmentsList,AttachmentForm,AttachmentView}.tsx`
- Изменить: `web/src/api.ts`, `web/src/App.tsx`, `web/src/pages/EntityCatalog.tsx`

**Важно для исполнителя**: `node_id`/`document_id` — обычные текстовые поля ввода id (`Input`), БЕЗ `Select`/picker'а — `ArchiveNode`/`ArchiveDocument` ещё не имеют своего списка, чтобы искать по нему (подпроод 6 вернёт сюда полноценный `Select`, как у `Archive.repository_id`). `kind` — закрытый перечень (скан/документ/аудио/фото) через `Select`, в отличие от `Note.Kind` (открытый — обычный `Input`).

### Шаг 3.1. `web/src/api.ts` — типы и функции Attachment

Добавить в конец файла (после блока Note из Задачи 2):
`web/src/api.ts (фрагмент)`:
```typescript
// Attachment — файловое вложение. node_id — обязательная строгая ссылка на
// архивный узел (просто id — ArchiveNode ещё без CRUD, подпроект 6).
// document_id — необязательная мягкая ссылка (ON DELETE SET NULL). Оба поля
// в v1 — обычные текстовые поля ввода id (без picker'а, тот появится вместе
// с ArchiveNode/ArchiveDocument).
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
```

### Шаг 3.2. Страницы `List`/`Form`/`View`

#### `web/src/pages/AttachmentForm.tsx` (создать)
`web/src/pages/AttachmentForm.tsx`:
```tsx
import { useState } from "react";
import { Alert, Checkbox, Form, Input, InputNumber, Modal, Select } from "antd";
import { createAttachment, type Attachment } from "../api";
import { ApiError } from "../auth";

interface AttachmentFormValues {
  kind: string;
  uri?: string;
  filename?: string;
  mime?: string;
  page?: number;
  node_id: string;
  document_id?: string;
  note?: string;
  private?: boolean;
}

const FORM_FIELDS: (keyof AttachmentFormValues)[] = [
  "kind",
  "uri",
  "filename",
  "mime",
  "page",
  "node_id",
  "document_id",
  "note",
];

const KIND_OPTIONS = [
  { value: "scan", label: "скан" },
  { value: "document", label: "документ" },
  { value: "audio", label: "аудио" },
  { value: "photo", label: "фото" },
];

// CreateAttachmentModal — форма создания вложения. node_id/document_id — в
// v1 обычные текстовые поля ввода id (без picker'а): ArchiveNode/
// ArchiveDocument ещё не имеют своего списка, чтобы искать по нему (подпроект
// 6 добавит их CRUD и вернёт сюда полноценный Select). node_id обязателен,
// document_id — нет.
export function CreateAttachmentModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Attachment) => void;
}) {
  const [form] = Form.useForm<AttachmentFormValues>();
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: AttachmentFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await createAttachment({
        kind: values.kind,
        uri: values.uri,
        filename: values.filename,
        mime: values.mime,
        page: values.page,
        node_id: values.node_id,
        document_id: values.document_id,
        note: values.note,
        private: values.private ?? false,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof AttachmentFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить вложение"
      open={open}
      onCancel={handleClose}
      onOk={() => form.submit()}
      okText="Создать"
      cancelText="Отмена"
      confirmLoading={submitting}
      destroyOnHidden
    >
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
      <Form form={form} layout="vertical" onFinish={onFinish} initialValues={{ kind: "scan", private: false }}>
        <Form.Item name="kind" label="Вид" rules={[{ required: true, message: "Выберите вид" }]}>
          <Select options={KIND_OPTIONS} />
        </Form.Item>
        <Form.Item name="filename" label="Имя файла">
          <Input autoFocus />
        </Form.Item>
        <Form.Item name="uri" label="URI">
          <Input placeholder="файл, ссылка" />
        </Form.Item>
        <Form.Item name="mime" label="MIME-тип">
          <Input placeholder="image/jpeg" />
        </Form.Item>
        <Form.Item name="page" label="Страница">
          <InputNumber min={0} style={{ width: "100%" }} />
        </Form.Item>
        <Form.Item
          name="node_id"
          label="Архивный узел (id)"
          rules={[{ required: true, whitespace: true, message: "Введите id архивного узла" }]}
        >
          <Input placeholder="AN-…" />
        </Form.Item>
        <Form.Item name="document_id" label="Архивный документ (id)">
          <Input placeholder="DC-… (необязательно)" />
        </Form.Item>
        <Form.Item name="note" label="Заметка">
          <Input.TextArea rows={3} />
        </Form.Item>
        <Form.Item name="private" valuePropName="checked">
          <Checkbox>Приватная запись</Checkbox>
        </Form.Item>
      </Form>
    </Modal>
  );
}
```

#### `web/src/pages/AttachmentsList.tsx` (создать)
`web/src/pages/AttachmentsList.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Input, List, Spin } from "antd";
import { fetchAttachments, searchAttachments, MAX_PAGE_LIMIT, type Attachment } from "../api";
import { useSession } from "../session";
import { CreateAttachmentModal } from "./AttachmentForm";

function attachmentLabel(a: Attachment): string {
  return a.filename || a.uri || a.id;
}

// AttachmentsList — «Вложения»: плоский список, та же пагинация-до-короткой-
// страницы и поиск-подменяет-список, что у ArchivesList/NotesList.
export default function AttachmentsList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<Attachment[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<Attachment[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: Attachment[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchAttachments({ limit: MAX_PAGE_LIMIT, offset });
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
    searchAttachments({ q, limit: MAX_PAGE_LIMIT })
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
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "Вложения" }]}
      />
      <Card
        title="Вложения"
        extra={
          session != null ? (
            <Button type="primary" onClick={() => setCreateOpen(true)}>
              + добавить
            </Button>
          ) : undefined
        }
      >
        <Input.Search
          placeholder="Поиск по имени файла или заметке…"
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
            renderItem={(a) => (
              <List.Item>
                <Link to={`/attachments/${a.id}`}>{attachmentLabel(a)}</Link>
              </List.Item>
            )}
          />
        )}
        <CreateAttachmentModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(a) => {
            setCreateOpen(false);
            navigate(`/attachments/${a.id}`);
          }}
        />
      </Card>
    </>
  );
}
```

#### `web/src/pages/AttachmentView.tsx` (создать)
`web/src/pages/AttachmentView.tsx`:
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
  InputNumber,
  List,
  Modal,
  Popconfirm,
  Select,
  Space,
  Spin,
  Typography,
} from "antd";
import { deleteAttachment, fetchAttachment, updateAttachment, type Attachment } from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";

interface EditFormValues {
  kind: string;
  uri?: string;
  filename?: string;
  mime?: string;
  page?: number;
  node_id: string;
  document_id?: string;
  note?: string;
  private?: boolean;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = [
  "kind",
  "uri",
  "filename",
  "mime",
  "page",
  "node_id",
  "document_id",
  "note",
];

const KIND_OPTIONS = [
  { value: "scan", label: "скан" },
  { value: "document", label: "документ" },
  { value: "audio", label: "аудио" },
  { value: "photo", label: "фото" },
];

function attachmentLabel(a: Attachment): string {
  return a.filename || a.uri || a.id;
}

// AttachmentView — просмотр вложения, переключаемый в форму редактирования
// на той же странице. node_id/document_id — обычные текстовые поля ввода id
// (см. AttachmentForm) — ArchiveNode/ArchiveDocument ещё без CRUD и своего
// списка для Select (подпроект 6).
export default function AttachmentView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [attachment, setAttachment] = useState<Attachment | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (attachmentId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setAttachment(null);
    fetchAttachment(attachmentId)
      .then(setAttachment)
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
    if (attachment == null) {
      return;
    }
    form.setFieldsValue({
      kind: attachment.kind,
      uri: attachment.uri ?? "",
      filename: attachment.filename ?? "",
      mime: attachment.mime ?? "",
      page: attachment.page,
      node_id: attachment.node_id,
      document_id: attachment.document_id ?? "",
      note: attachment.note ?? "",
      private: attachment.private,
    });
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (attachment == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await updateAttachment(attachment.id, {
        kind: values.kind,
        uri: values.uri,
        filename: values.filename,
        mime: values.mime,
        page: values.page,
        node_id: values.node_id,
        document_id: values.document_id,
        note: values.note,
        private: values.private ?? false,
      });
      setAttachment(updated);
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
    if (attachment == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteAttachment(attachment.id);
      navigate("/attachments");
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
          <Link to="/attachments">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (attachment == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/attachments">Вложения</Link> },
          { title: attachmentLabel(attachment) },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={attachmentLabel(attachment)} column={1} bordered size="small">
            <Descriptions.Item label="Вид">{attachment.kind}</Descriptions.Item>
            <Descriptions.Item label="Имя файла">{attachment.filename || "—"}</Descriptions.Item>
            <Descriptions.Item label="URI">{attachment.uri || "—"}</Descriptions.Item>
            <Descriptions.Item label="MIME-тип">{attachment.mime || "—"}</Descriptions.Item>
            <Descriptions.Item label="Страница">{attachment.page || "—"}</Descriptions.Item>
            <Descriptions.Item label="Архивный узел">{attachment.node_id}</Descriptions.Item>
            <Descriptions.Item label="Архивный документ">{attachment.document_id || "—"}</Descriptions.Item>
            <Descriptions.Item label="Заметка">{attachment.note || "—"}</Descriptions.Item>
            <Descriptions.Item label="Приватная">{attachment.private ? "да" : "нет"}</Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title={`Удалить «${attachmentLabel(attachment)}»?`}
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
          <Form.Item name="kind" label="Вид" rules={[{ required: true, message: "Выберите вид" }]}>
            <Select options={KIND_OPTIONS} />
          </Form.Item>
          <Form.Item name="filename" label="Имя файла">
            <Input />
          </Form.Item>
          <Form.Item name="uri" label="URI">
            <Input placeholder="файл, ссылка" />
          </Form.Item>
          <Form.Item name="mime" label="MIME-тип">
            <Input placeholder="image/jpeg" />
          </Form.Item>
          <Form.Item name="page" label="Страница">
            <InputNumber min={0} style={{ width: "100%" }} />
          </Form.Item>
          <Form.Item
            name="node_id"
            label="Архивный узел (id)"
            rules={[{ required: true, whitespace: true, message: "Введите id архивного узла" }]}
          >
            <Input placeholder="AN-…" />
          </Form.Item>
          <Form.Item name="document_id" label="Архивный документ (id)">
            <Input placeholder="DC-… (необязательно)" />
          </Form.Item>
          <Form.Item name="note" label="Заметка">
            <Input.TextArea rows={3} />
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

### Шаг 3.3. `web/src/App.tsx` — роуты Attachment, `EntityCatalog.tsx` — строка «Вложения»

Добавить импорты в `web/src/App.tsx` (после импортов Note из Задачи 2):
```typescript
import AttachmentsList from "./pages/AttachmentsList";
import AttachmentView from "./pages/AttachmentView";
```
Добавить роуты (после роутов `/notes`/`/notes/:id`, до `/login`):
```typescript
          <Route path="/attachments" element={<PageLayout><AttachmentsList /></PageLayout>} />
          <Route path="/attachments/:id" element={<PageLayout><AttachmentView /></PageLayout>} />
```

В массиве `CATALOG_ENTRIES` добавить:
```typescript
  { label: "Вложения", path: "/attachments" },
```

Итоговый вид массива после Задач 2 и 3 (для сверки — порядок внутри массива не влияет на отображение, список сортируется рантаймом):
`web/src/pages/EntityCatalog.tsx`:
```tsx
import { Card, List, Typography } from "antd";
import { Link } from "react-router-dom";

// CATALOG_ENTRIES — единая точка входа приложения: по алфавиту названия.
// Каждая сущность получает свой список при подключении (docs/data-model/
// entity-write.md §4) — здесь просто добавляется новая строка, без вкладок.
const CATALOG_ENTRIES: { label: string; path: string }[] = [
  { label: "Административное деление", path: "/divisions" },
  { label: "Архивы", path: "/archives" },
  { label: "Вложения", path: "/attachments" },
  { label: "Документация", path: "/docs" },
  { label: "Заметки", path: "/notes" },
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

### Шаг 3.4. Рубеж

`npm run typecheck` и `npm run build` в `web/` чисты. Живая проверка в браузере (см. «Предпосылка»): каталог на `/` показывает все 13 строк по алфавиту; `/attachments` → «+ добавить» → форма (Вид — `Select` с закрытым перечнем, `node_id`/`document_id` — текстовые поля) → несуществующий `node_id` → создание → ошибка поля всплывает inline под полем «Архивный узел (id)», без общего alert.

### Шаг 3.5. Коммит

```bash
git add web/src/api.ts web/src/App.tsx web/src/pages/EntityCatalog.tsx \
  web/src/pages/AttachmentsList.tsx web/src/pages/AttachmentForm.tsx web/src/pages/AttachmentView.tsx
git commit -m "feat(web): страницы Вложений — список, просмотр/редактирование, форма"
```

## Рубеж прохода

После Задачи 3: `gofmt -l .` пусто, `go build/vet/test ./...` зелёные (987
теста, 88 пакетов), `npm run typecheck`/`build` чисты. Каталог `/` — 13
строк по алфавиту. `Note` (self-ref `ParentID` со строгой FK, проверка
существования на create + обход цепочки на цикл на update) и `Attachment`
(обязательный `NodeID` + необязательный `DocumentID` — SET NULL, оба
existence-проверяются через generic-хранилище на сущность без своего
CRUD-слоя) имеют полный CRUD через HTTP, MCP и веб, по конвенциям
`docs/data-model/entity-write.md` §3-4 — задел для подпроектов 5-9.
Следующий подпроект — 5 (`Source`/`Citation`, вернёт `Sources`
`Note`/`Church`/`Parish`/`Archive`/`Repository` из read-only в полноценный
редактируемый список).

## Коммиты

Три коммита в `main`, по одному на задачу — см. Шаги 1.5, 2.5, 3.5.
