# Веб-CRUD/MCP для всех сущностей — подпроект 5 (цепочка доказательств: Source, Citation): план

> **Для агента-исполнителя:** ОБЯЗАТЕЛЬНЫЙ ПОДНАВЫК: superpowers:subagent-driven-development.

Пятый проход по декомпозиции `docs/data-model/entity-write.md` §2. Вводит `Source` (источник доказательства) и `Citation` (цитата из источника) — первый **полиморфный тип** в программе (`Citation.Anchor` — «где именно»: архивный узел/документ, файл-вложение, внешняя ссылка, или ничего). Одновременно разблокирует редактирование `Sources []SourceLink` — поля, которое было read-only у `AdministrativeDivision`, `Repository`, `Church`, `Parish`, `Archive`, `Note` с момента его первого появления в подпроекте 3 (`entity-write.md` §2.5: «Source → Citation → SourceLink из всех остальных сущностей — вместе, один план»).

## Goal

1. Полный CRUD (HTTP + MCP + веб) для `Source` и `Citation` — по конвенциям `docs/data-model/entity-write.md` §3-4.
2. Разблокировать редактирование `Sources []SourceLink` у всех 6 сущностей, где оно уже есть: транспортные Create/Update DTO, httpapi fetch-then-merge, MCP-аргументы. Впервые для `AdministrativeDivision` — также сделать `Sources` видимым на чтении (было отсутствует в контракте вовсе, не просто read-only).
3. Новые переиспользуемые паттерны: **полиморфный тип** (`Anchor`, плоское представление с дискриминатором `kind`, по образцу `FactDate`); **MCP-аргумент вида «массив объектов»** (`sources` — впервые в программе, до этого массивы были только строками); строгая ссылка внутри списка (`SourceLink.CitationID`) с проверкой существования по индексу (`sources[i].citation_id`).

## Предпосылка: живая проверка

Весь код ниже применён и проверен в отдельном git worktree (`../genodex-verify-subproject5`, ветка `verify/entity-write-subproject5`) — не на `main` напрямую. `gofmt -l .` пусто, `go build/vet/test ./...` — 1093 теста, 100 пакетов (было 989/88 после подпроекта 4 — 104 новых теста: 62 в usecases для Source/Citation, 28 в httpapi+mcp, 12 регресс-тестов на `sources[i].citation_id`, 2 сквозных real-store теста — `TestSourceWriteContractWithRealStore`/`TestCitationWriteContractWithRealStore`, плюс расширение существующего `TestArchiveWriteContractWithRealStore` на `sources`, не отдельная функция), `npm run typecheck`/`build` чисты.

**Находки живой проверки, исправленные до записи плана (не отдельными шагами — код ниже уже содержит исправления):**
- Изначальный ретрофит веб-форм 6 сущностей был БЛОКИРОВАН на первой попытке: `web/src/api.ts`'s шесть `*Input`-интерфейсов (`RepositoryInput`/`ChurchInput`/`ParishInput`/`ArchiveInput`/`NoteInput`/`AdminDivisionInput`) не объявляли поле `sources` — исполнитель корректно остановился, а не работал в обход через `as any`. Добавлено вручную во все 6 плюс в read-интерфейс `AdminDivision`, вторая попытка ретрофита прошла чисто.
- `AdministrativeDivision` — единственная из 6 сущностей, чей read-контракт (`internal/transport/admin_division.go`, TS-интерфейс `AdminDivision`) НИКОГДА не включал `Sources` вовсе (узкий DTO с подпроекта 1 — только name/type/parent_id), в отличие от Repository/Church/Parish/Archive/Note, у которых `Sources` был хотя бы read-only. Добавлено поле в read-контракт (единственное расширение узкого DTO, остальные поля — Items/Variants/Renames/Successors/Since/Until/Notes — по-прежнему вне контракта, это не общий пересмотр минимализма Division, а точечное дополнение только для Sources, раз оно теперь редактируется). Правка вызвала ожидаемый tест-fallout — 11 существующих тестов в 5 файлах проверяли точную JSON-строку без поля `sources`; исправлены на `,"sources":[]`.
- `SourceLink.CitationID` — модельная `Validate()` уже проверяла ФОРМАТ id (корректный ULID с префиксом `C-`), но ни один сценарий не проверял СУЩЕСТВОВАНИЕ цитаты — расхождение с каждой другой строгой ссылкой в программе (`Archive.RepositoryID`, `Citation.SourceID`, `Attachment.NodeID` — все проверяются в той же транзакции, что и сохранение). Добавлена проверка существования по индексу (`sources[i].citation_id`) во ВСЕ 12 create/update-сценариев (`Repository`/`Church`/`Parish` при этом впервые стали транзакционными — раньше делали плоский `Save` без `InTx`, поскольку у них не было ни одного FK для проверки).

Живой смок-тест бэкенда через `curl`: создание `Source` с `date`/`reliability`/`repository_id` → 201; `Citation` без anchor → 201; `Citation` с `anchor.kind=url` → 201, anchor round-trip'ится; `Citation` с несуществующим `source_id` → 422 `{"field":"source_id"}`; `Citation` с `anchor.kind=archive` и несуществующим `node_id` → 422 `{"field":"anchor.node_id"}`; `Archive` с `sources: [{"citation_id": "<несуществующий>"}]` → 422 `{"field":"sources[0].citation_id"}`; приватный `Source`, анонимный `GET` → 404. MCP `initialize` через токен владельца — рукопожатие проходит.

Живая проверка веб-UI в браузере: каталог на `/` — 15 строк по алфавиту (добавились «Источники», «Цитаты»); `/sources` → «+ добавить» → `FactDateEditor` для даты (год 1889, точность «Год», модификатор «Точно») → `Select` «Достоверность» → создание → View показывает «1889» (`formatFactDate`) и «Первичный»; `/citations` → «+ добавить» → `Select` «Источник» с поиском (показывает созданный `Source`) → `AnchorEditor`: переключение `kind` архив→файл→ссылка корректно меняет набор полей и сбрасывает предыдущие (проверено визуально — переключение на «Ссылка» убирает поля узла/документа/страницы) → `kind=url`, `url=https://example.org/...` → создание → View показывает URL текстом, источник — кликабельной ссылкой; `/archives` → «+ добавить» → секция «Доказательства» (`SourceLinkListEditor`) → `Select` «Цитата» с поиском (показывает созданную `Citation`) → создание → View показывает `citation C-...` в «Доказательства»; то же для `/divisions` — «Доказательства» теперь видны и редактируются (раньше не показывались вовсе).

## Задача 1. Бэкенд: Source, Citation (новые сущности)

**Интерфейсы, потребляемые из подпроектов 1-4**: `models.SearchQuery`, `internal/httpapi/surname.go`:`parsePage`, `internal/mcp/*.go`:`optionalInt`/`toolJSONResult`/`textRefsFromStrings`, `transport.{TextRef,FactDate,SourceLink}`, `get_archive`'s access-aware `Get`-паттерн, `create_archive`'s проверка-FK-в-транзакции паттерн, `internal/mcp/object_args.go`:`factDateObjectProperties`/`optionalFactDate` (переиспользуются для `Source.Date`).
**Производит**: `transport.{Source,Citation,Anchor}` + Create/Update-варианты, `transport.SourceLink.Model()`/`SourceLinksToModel` (записываемая сторона — впервые в этом проходе, раньше `SourceLink` был только read-only), `httpapi.{Source,Citation}Service`, `mcp.{Source,Citation}Service`, `mcp.object_args.go`:`anchorObjectProperties`/`optionalAnchor`, `sourceLinkObjectProperties`/`optionalSourceLinks` — вторая пара потребляется Задачей 2 (ретрофит 6 сущностей).

**Файлы:**
- Изменить: `internal/transport/source_link.go` (добавить в конец `.Model()`/`SourceLinksToModel`), `internal/mcp/object_args.go` (добавить `anchorObjectProperties`/`optionalAnchor`/`sourceLinkObjectProperties`/`optionalSourceLinks`), `internal/httpapi/{deps.go,api.go,httpapi.go}`, `internal/mcp/{deps.go,server.go}`, `internal/app/app.go`
- Создать: `internal/transport/{source,source_write,citation,citation_write,anchor}.go`, `internal/usecases/{list_sources,search_sources,get_source,create_source,update_source,delete_source}/{deps.go,scenario.go,scenario_test.go}` (и та же шестёрка для `{list,search,get,create,update,delete}_citations`/`_citation`), `internal/httpapi/{source,source_write,source_test,source_write_test}.go` (и то же для `citation`), `internal/mcp/{source,source_test}.go` (и то же для `citation`)

**Важно для исполнителя**: `list_*`/`search_*`/`delete_*` — механические. `get_source`/`get_citation` — access-aware с первого черновика (оба имеют `Private`). `create_source`/`update_source` — один опциональный strict FK (`RepositoryID`), по образцу `create_archive`/`update_archive` буквально (проверка «если задан»). `create_citation`/`update_citation` — ОБЯЗАТЕЛЬНЫЙ strict FK (`SourceID`, проверяется всегда, не «если задан») ПЛЮС условная проверка ссылок внутри `Anchor`, если он задан: `*ArchiveAnchor` → `NodeID` всегда, `DocumentID` если задан (оба через `tx.GetArchiveNode`/`tx.GetArchiveDocument` — `ArchiveNode`/`ArchiveDocument` ещё без своего CRUD-слоя, подпроект 6, но generic-хранилище уже умеет их читать); `*FileAnchor` → `AttachmentID` всегда (через `tx.GetAttachment`); `*URLAnchor` — ссылок не несёт, проверка не нужна (URL валидируется структурно в `models.Citation.Validate()`). Переносить код ниже как есть.

### Шаг 1.1. `internal/transport/source_link.go` — добавить `.Model()`/`SourceLinksToModel`

Изменить существующий файл (был read-only с подпроекта 3) — итоговое содержимое:
`internal/transport/source_link.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// SourceLink — контракт доказательства «утверждение → цитата»
// (models.SourceLink). Редактируемый с подпроекта 5 (Citation теперь имеет
// CRUD): Create/Update DTO сущностей-владельцев (Repository/Church/Parish/
// Archive/Note/AdministrativeDivision) несут это поле. TargetType/TargetID
// клиент не отправляет и они игнорируются при сохранении — владелец
// восстанавливается сценарием/хранилищем из контекста вызова (см.
// sqlstore.replaceSourceLinks/loadSourceLinks), поэтому Model() их не
// заполняет.
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

// Model конвертирует контракт обратно в модель. TargetType/TargetID не
// заполняются — владелец подставляет их сам (см. тип выше).
func (s SourceLink) Model() models.SourceLink {
	return models.SourceLink{
		CitationID:  models.ID(s.CitationID),
		Reliability: models.Reliability(s.Reliability),
		Role:        s.Role,
		Note:        s.Note,
	}
}

// SourceLinksToModel конвертирует список контрактов в модели; пустой вход
// даёт пустой срез, а не nil.
func SourceLinksToModel(ss []SourceLink) []models.SourceLink {
	out := make([]models.SourceLink, 0, len(ss))
	for _, s := range ss {
		out = append(out, s.Model())
	}

	return out
}
```

### Шаг 1.2. `internal/transport/anchor.go` (создать)
`internal/transport/anchor.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// Anchor — контракт полиморфной привязки «где именно» (models.Anchor):
// первый полиморфный тип в программе. Плоское представление с
// дискриминатором Kind ("archive"/"file"/"url"), по образцу FactDate
// (все поля опциональны вместе, а не набор отдельных объектов-вариантов) —
// проще для JSON REST и MCP-объектного аргумента, чем вложенный union.
// nil — привязки нет (цитата может относиться к источнику целиком).
type Anchor struct {
	Kind         string `json:"kind,omitempty"`
	NodeID       string `json:"node_id,omitempty"`
	DocumentID   string `json:"document_id,omitempty"`
	Page         int    `json:"page,omitempty"`
	Rect         string `json:"rect,omitempty"`
	AttachmentID string `json:"attachment_id,omitempty"`
	Timecode     string `json:"timecode,omitempty"`
	URL          string `json:"url,omitempty"`
}

// AnchorFromModel конвертирует привязку в контракт; nil — привязки нет.
func AnchorFromModel(a models.Anchor) *Anchor {
	switch v := a.(type) {
	case nil:
		return nil
	case *models.ArchiveAnchor:
		if v == nil {
			return nil
		}

		return &Anchor{Kind: "archive", NodeID: string(v.NodeID), DocumentID: string(v.DocumentID), Page: v.Page, Rect: v.Rect}
	case *models.FileAnchor:
		if v == nil {
			return nil
		}

		return &Anchor{Kind: "file", AttachmentID: string(v.AttachmentID), Timecode: v.Timecode}
	case *models.URLAnchor:
		if v == nil {
			return nil
		}

		return &Anchor{Kind: "url", URL: v.URL}
	default:
		return nil
	}
}

// Model конвертирует контракт обратно в модель; nil (или неизвестный/пустой
// Kind) — привязки нет. Само поле Anchor у Citation необязательно —
// невалидный Kind просто даёт "без привязки", а не ошибку конвертации;
// содержательную проверку (например, обязательные поля внутри варианта)
// делает models.Citation.Validate().
func (a *Anchor) Model() models.Anchor {
	if a == nil {
		return nil
	}

	switch a.Kind {
	case "archive":
		return &models.ArchiveAnchor{NodeID: models.ID(a.NodeID), DocumentID: models.ID(a.DocumentID), Page: a.Page, Rect: a.Rect}
	case "file":
		return &models.FileAnchor{AttachmentID: models.ID(a.AttachmentID), Timecode: a.Timecode}
	case "url":
		return &models.URLAnchor{URL: a.URL}
	default:
		return nil
	}
}
```

### Шаг 1.3. Source — транспорт, usecases, httpapi, MCP

#### `internal/transport/source.go` (создать)
`internal/transport/source.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// Source — контракт источника доказательства (GET /api/sources, MCP-тул
// source_list). Date — структурированная дата (см. transport.FactDate).
// RepositoryID — мягкая ссылка на хранилище (просто id, необязательна).
type Source struct {
	ID           models.ID `json:"id"`
	Kind         string    `json:"kind"`
	Title        string    `json:"title"`
	Author       string    `json:"author,omitempty"`
	Date         *FactDate `json:"date,omitempty"`
	Reliability  string    `json:"reliability"`
	RepositoryID string    `json:"repository_id,omitempty"`
	Notes        []TextRef `json:"notes"`
	Private      bool      `json:"private"`
}

// SourceFromModel конвертирует запись в контракт.
func SourceFromModel(s models.Source) Source {
	return Source{
		ID:           s.ID,
		Kind:         string(s.Kind),
		Title:        s.Title,
		Author:       s.Author,
		Date:         FactDateFromModel(s.Date),
		Reliability:  string(s.Reliability),
		RepositoryID: string(s.RepositoryID),
		Notes:        TextRefsFromModel(s.Notes),
		Private:      s.Private,
	}
}

// SourcesFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func SourcesFromModels(ss []models.Source) []Source {
	out := make([]Source, 0, len(ss))
	for _, s := range ss {
		out = append(out, SourceFromModel(s))
	}

	return out
}
```

#### `internal/transport/source_write.go` (создать)
`internal/transport/source_write.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// SourceCreate — тело POST /api/sources и аргументы тула source_create.
// Идентификатор генерирует сценарий. RepositoryID — просто id (пустая
// строка — без хранилища); сценарий проверяет существование при непустом
// значении, по образцу create_archive.
type SourceCreate struct {
	Kind         string    `json:"kind"`
	Title        string    `json:"title"`
	Author       string    `json:"author,omitempty"`
	Date         *FactDate `json:"date,omitempty"`
	Reliability  string    `json:"reliability"`
	RepositoryID string    `json:"repository_id,omitempty"`
	Notes        []TextRef `json:"notes"`
	Private      bool      `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (s SourceCreate) Model() models.Source {
	return models.Source{
		Kind:         models.SourceKind(s.Kind),
		Title:        s.Title,
		Author:       s.Author,
		Date:         s.Date.Model(),
		Reliability:  models.Reliability(s.Reliability),
		RepositoryID: models.ID(s.RepositoryID),
		Notes:        TextRefsToModel(s.Notes),
		Private:      s.Private,
	}
}

// SourceUpdate — тело PUT /api/sources/{id} и аргументы тула source_update:
// полная замена kind/title/author/date/reliability/repository_id/notes/private.
type SourceUpdate struct {
	Kind         string    `json:"kind"`
	Title        string    `json:"title"`
	Author       string    `json:"author,omitempty"`
	Date         *FactDate `json:"date,omitempty"`
	Reliability  string    `json:"reliability"`
	RepositoryID string    `json:"repository_id,omitempty"`
	Notes        []TextRef `json:"notes"`
	Private      bool      `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (s SourceUpdate) Model() models.Source {
	return models.Source{
		Kind:         models.SourceKind(s.Kind),
		Title:        s.Title,
		Author:       s.Author,
		Date:         s.Date.Model(),
		Reliability:  models.Reliability(s.Reliability),
		RepositoryID: models.ID(s.RepositoryID),
		Notes:        TextRefsToModel(s.Notes),
		Private:      s.Private,
	}
}
```

#### `internal/usecases/list_sources/deps.go` (создать)
`internal/usecases/list_sources/deps.go`:
```go
package list_sources

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// SourceRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type SourceRepo interface {
	ListSources(ctx context.Context, access models.Access, page models.Page) ([]*models.Source, error)
}
```

#### `internal/usecases/list_sources/scenario.go` (создать)
`internal/usecases/list_sources/scenario.go`:
```go
package list_sources

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список записей источников».
type Scenario struct {
	sources SourceRepo
}

// New создаёт сценарий.
func New(sources SourceRepo) *Scenario {
	return &Scenario{sources: sources}
}

// ListSources возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListSources(ctx context.Context, access models.Access, page models.Page) ([]models.Source, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeSource, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeSource, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.sources.ListSources(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Source, 0, len(list))
	for _, a := range list {
		out = append(out, *a)
	}

	return out, nil
}
```

#### `internal/usecases/list_sources/scenario_test.go` (создать)
`internal/usecases/list_sources/scenario_test.go`:
```go
package list_sources

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.Source
}

func (f *fakeRepo) ListSources(_ context.Context, _ models.Access, page models.Page) ([]*models.Source, error) {
	f.page = page

	return f.out, nil
}

func TestListSourcesReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.Source{{ID: "S-01ARZ3NDEKTSV4RRFFQ69G5FA1", Kind: models.SourceKindDocument, Title: "Метрическая книга", Reliability: models.ReliabilityPrimary}}}

	got, err := New(repo).ListSources(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListSources: %v", err)
	}

	if len(got) != 1 || got[0].Title != "Метрическая книга" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListSourcesRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListSources(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
```

#### `internal/usecases/search_sources/deps.go` (создать)
`internal/usecases/search_sources/deps.go`:
```go
package search_sources

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// SourceRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type SourceRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetSource(ctx context.Context, id models.ID) (*models.Source, error)
}
```

#### `internal/usecases/search_sources/scenario.go` (создать)
`internal/usecases/search_sources/scenario.go`:
```go
package search_sources

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск записей источников».
type Scenario struct {
	sources SourceRepo
}

// New создаёт сценарий.
func New(sources SourceRepo) *Scenario {
	return &Scenario{sources: sources}
}

// SearchSources находит записи, чьи название или автор начинаются с текста
// запроса (sqlstore.SaveSource индексирует оба поля) — та же механика, что и
// search_divisions.SearchDivisions (см. её комментарий).
func (s *Scenario) SearchSources(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Source, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Source{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Source{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.sources.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeSource {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.sources.GetSource(ctx, h.ID)
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

#### `internal/usecases/search_sources/scenario_test.go` (создать)
`internal/usecases/search_sources/scenario_test.go`:
```go
package search_sources

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits    []models.Hit
	sources map[models.ID]*models.Source
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetSource(_ context.Context, id models.ID) (*models.Source, error) {
	s, ok := f.sources[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestSearchSourcesFiltersByType(t *testing.T) {
	id := models.ID("S-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeSource, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не source — должен быть пропущен
		},
		sources: map[models.ID]*models.Source{id: {ID: id, Kind: models.SourceKindDocument, Title: "Метрическая книга", Author: "Иванов", Reliability: models.ReliabilityPrimary}},
	}

	got, err := New(repo).SearchSources(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchSources: %v", err)
	}

	if len(got) != 1 || got[0].Title != "Метрическая книга" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchSourcesEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchSources(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchSources: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
```

#### `internal/usecases/get_source/deps.go` (создать)
`internal/usecases/get_source/deps.go`:
```go
package get_source

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// SourceRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type SourceRepo interface {
	GetSource(ctx context.Context, id models.ID) (*models.Source, error)
}
```

#### `internal/usecases/get_source/scenario.go` (создать)
`internal/usecases/get_source/scenario.go`:
```go
package get_source

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «запись источника по идентификатору».
type Scenario struct {
	sources SourceRepo
}

// New создаёт сценарий.
func New(sources SourceRepo) *Scenario {
	return &Scenario{sources: sources}
}

// GetSource возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound. Приватная запись
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound — тот же принцип «прячем как отсутствующее», что и в
// List/Search.
func (s *Scenario) GetSource(ctx context.Context, access models.Access, id models.ID) (models.Source, error) {
	if err := validateID(id); err != nil {
		return models.Source{}, err
	}

	src, err := s.sources.GetSource(ctx, id)
	if err != nil {
		return models.Source{}, err
	}

	if src.Private && access != models.AccessFull {
		return models.Source{}, models.ErrNotFound
	}

	return *src, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeSource)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeSource,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

#### `internal/usecases/get_source/scenario_test.go` (создать)
`internal/usecases/get_source/scenario_test.go`:
```go
package get_source

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	sources map[models.ID]*models.Source
}

func (f *fakeRepo) GetSource(_ context.Context, id models.ID) (*models.Source, error) {
	s, ok := f.sources[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
}

func TestGetSourceReturnsRecord(t *testing.T) {
	id := models.ID("S-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{sources: map[models.ID]*models.Source{id: {ID: id, Kind: models.SourceKindDocument, Title: "Метрическая книга", Reliability: models.ReliabilityPrimary}}}

	got, err := New(repo).GetSource(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}

	if got.Title != "Метрическая книга" {
		t.Fatalf("Title = %q", got.Title)
	}
}

func TestGetSourceNotFound(t *testing.T) {
	repo := &fakeRepo{sources: map[models.ID]*models.Source{}}

	_, err := New(repo).GetSource(context.Background(), models.AccessFull, "S-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetSourceInvalidID(t *testing.T) {
	repo := &fakeRepo{sources: map[models.ID]*models.Source{}}

	_, err := New(repo).GetSource(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestGetSourcePrivateHiddenFromPublic(t *testing.T) {
	id := models.ID("S-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{sources: map[models.ID]*models.Source{id: {ID: id, Kind: models.SourceKindDocument, Title: "Метрическая книга", Reliability: models.ReliabilityPrimary, Private: true}}}

	_, err := New(repo).GetSource(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for private record with AccessPublic", err)
	}
}

func TestGetSourcePrivateVisibleToFullAccess(t *testing.T) {
	id := models.ID("S-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{sources: map[models.ID]*models.Source{id: {ID: id, Kind: models.SourceKindDocument, Title: "Метрическая книга", Reliability: models.ReliabilityPrimary, Private: true}}}

	got, err := New(repo).GetSource(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}

	if got.Title != "Метрическая книга" {
		t.Fatalf("Title = %q", got.Title)
	}
}
```

#### `internal/usecases/create_source/deps.go` (создать)
`internal/usecases/create_source/deps.go`:
```go
package create_source

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// SourceStore — зависимость сценария: транзакция порта store.Store. Проверка
// хранилища (если задано) и сохранение идут в одной транзакции на переданном
// fn хранилище.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type SourceStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
```

#### `internal/usecases/create_source/scenario.go` (создать)
`internal/usecases/create_source/scenario.go`:
```go
package create_source

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание источника».
type Scenario struct {
	store SourceStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st SourceStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateSource создаёт источник: генерирует идентификатор, проверяет
// инварианты, в одной транзакции (если хранилище задано) убеждается в его
// существовании и сохраняет. Возвращает созданный источник с заполненным ID.
// По образцу create_archive (одиночный опциональный strict FK RepositoryID).
//
// Ошибки: непустой входной ID, невалидная сущность и несуществующее
// хранилище — *models.ValidationError (поля id, repository_id); прочее —
// ошибки хранилища как есть.
func (s *Scenario) CreateSource(ctx context.Context, src models.Source) (models.Source, error) {
	if src.ID != "" {
		return models.Source{}, &models.ValidationError{
			Entity: models.TypeSource,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", src.ID),
		}
	}

	src.ID = s.ids.New(models.TypeSource)

	if err := src.Validate(); err != nil {
		return models.Source{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		if src.RepositoryID != "" {
			if _, err := tx.GetRepository(ctx, src.RepositoryID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return repositoryErr("хранилище %q не найдено", src.RepositoryID)
				}

				return err
			}
		}

		return tx.SaveSource(ctx, &src)
	})
	if err != nil {
		return models.Source{}, err
	}

	return src, nil
}

// repositoryErr — *models.ValidationError по полю repository_id.
func repositoryErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeSource,
		Field:  "repository_id",
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/create_source/scenario_test.go` (создать)
`internal/usecases/create_source/scenario_test.go`:
```go
package create_source

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// srcID возвращает корректный идентификатор источника, отличающийся последним символом.
func srcID(last byte) models.ID {
	return models.ID("S-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
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

// fakeTx реализует нужные сценарию методы store.Store поверх карт источников
// и хранилищ; остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	repositories map[models.ID]*models.Repository
	saved        []*models.Source
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

func (f *fakeTx) SaveSource(_ context.Context, s *models.Source) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *s
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует SourceStore: InTx выполняет fn на встроенном fakeTx.
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

func validInput() models.Source {
	return models.Source{Kind: models.SourceKindDocument, Title: "Метрическая книга", Reliability: models.ReliabilityPrimary}
}

func TestCreateSourceGeneratesIDAndSaves(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	ids := &stubIDs{id: srcID('V')}

	got, err := New(st, ids).CreateSource(context.Background(), validInput())
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}

	if got.ID != srcID('V') || got.Title != "Метрическая книга" {
		t.Fatalf("got %+v, ожидался источник с ID %v", got, srcID('V'))
	}

	if ids.gotType != models.TypeSource {
		t.Errorf("генератор вызван с типом %q, ожидался source", ids.gotType)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].ID != srcID('V') {
		t.Fatalf("InTx=%d, saved=%v; ожидалось одно сохранение внутри одной транзакции", st.calls, st.tx.saved)
	}
}

func TestCreateSourceRejectsExplicitID(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	ids := &stubIDs{id: srcID('V')}

	in := validInput()
	in.ID = srcID('0')

	_, err := New(st, ids).CreateSource(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю id", err)
	}

	if st.calls != 0 || ids.gotType != "" {
		t.Fatalf("InTx=%d, генератор вызван с %q; ожидалось: не вызваны", st.calls, ids.gotType)
	}
}

func TestCreateSourceValidatesBeforeTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	in := validInput()
	in.Title = ""

	_, err := New(st, &stubIDs{id: srcID('V')}).CreateSource(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "title" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю title", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestCreateSourceWithRepositorySaves(t *testing.T) {
	repo := &models.Repository{ID: rID('0'), Name: "ГАВО", Type: models.RepositoryTypeArchive}
	st := &fakeStore{tx: newFakeTx(repo)}

	in := validInput()
	in.RepositoryID = repo.ID

	got, err := New(st, &stubIDs{id: srcID('V')}).CreateSource(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}

	if got.RepositoryID != repo.ID || len(st.tx.saved) != 1 {
		t.Fatalf("got %+v, saved=%d; ожидалось сохранение со ссылкой на хранилище", got, len(st.tx.saved))
	}
}

// TestCreateSourceRepositoryNotFound: RepositoryID задан, но такого
// хранилища нет — *models.ValidationError по полю repository_id, ничего не
// сохраняется. Случай «RepositoryID не задан вовсе — хранилище не
// проверяется» уже покрыт TestCreateSourceGeneratesIDAndSaves выше (пустой
// fakeTx, GetRepository не вызывается).
func TestCreateSourceRepositoryNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	in := validInput()
	in.RepositoryID = rID('0')

	_, err := New(st, &stubIDs{id: srcID('V')}).CreateSource(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "repository_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю repository_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d источников при несуществующем хранилище", len(st.tx.saved))
	}
}

func TestCreateSourcePropagatesSaveError(t *testing.T) {
	wantErr := errors.New("save failed")
	st := &fakeStore{tx: newFakeTx()}
	st.tx.saveErr = wantErr

	if _, err := New(st, &stubIDs{id: srcID('V')}).CreateSource(context.Background(), validInput()); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

func TestCreateSourcePropagatesTxError(t *testing.T) {
	wantErr := errors.New("tx failed")
	st := &fakeStore{tx: newFakeTx(), inTxErr: wantErr}

	if _, err := New(st, &stubIDs{id: srcID('V')}).CreateSource(context.Background(), validInput()); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

func TestCreateSourcePropagatesRepositoryGetError(t *testing.T) {
	wantErr := errors.New("get failed")
	st := &fakeStore{tx: newFakeTx()}
	st.tx.getErr = wantErr

	in := validInput()
	in.RepositoryID = rID('0')

	_, err := New(st, &stubIDs{id: srcID('V')}).CreateSource(context.Background(), in)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}

	var ve *models.ValidationError
	if errors.As(err, &ve) {
		t.Fatalf("сбой хранилища превращён в *ValidationError: %v", err)
	}
}
```

#### `internal/usecases/update_source/deps.go` (создать)
`internal/usecases/update_source/deps.go`:
```go
package update_source

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// SourceStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования, проверка хранилища (если задано) и сохранение идут
// в одной транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type SourceStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
```

#### `internal/usecases/update_source/scenario.go` (создать)
`internal/usecases/update_source/scenario.go`:
```go
package update_source

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение источника».
type Scenario struct {
	store SourceStore
}

// New создаёт сценарий.
func New(st SourceStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateSource полностью заменяет источник по src.ID: проверяет инварианты, в
// одной транзакции убеждается, что источник существует и (если хранилище
// задано) хранилище существует, и сохраняет.
//
// Ошибки: невалидная сущность и несуществующее хранилище —
// *models.ValidationError (соответствующее поле, repository_id); нет такого
// источника — models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateSource(ctx context.Context, src models.Source) error {
	if err := src.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetSource(ctx, src.ID); err != nil {
			return err
		}

		if src.RepositoryID != "" {
			if _, err := tx.GetRepository(ctx, src.RepositoryID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return repositoryErr("хранилище %q не найдено", src.RepositoryID)
				}

				return err
			}
		}

		return tx.SaveSource(ctx, &src)
	})
}

// repositoryErr — *models.ValidationError по полю repository_id.
func repositoryErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeSource,
		Field:  "repository_id",
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/update_source/scenario_test.go` (создать)
`internal/usecases/update_source/scenario_test.go`:
```go
package update_source

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

func srcID(last byte) models.ID {
	return models.ID("S-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func rID(last byte) models.ID {
	return models.ID("R-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// fakeTx реализует нужные сценарию методы store.Store поверх карт источников и
// хранилищ; остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	sources      map[models.ID]*models.Source
	repositories map[models.ID]*models.Repository
	saved        []*models.Source
	repoGetErr   error
}

func newFakeTx(existing ...*models.Source) *fakeTx {
	tx := &fakeTx{sources: map[models.ID]*models.Source{}, repositories: map[models.ID]*models.Repository{}}
	for _, s := range existing {
		tx.sources[s.ID] = s
	}

	return tx
}

func (f *fakeTx) withRepository(r *models.Repository) *fakeTx {
	f.repositories[r.ID] = r

	return f
}

func (f *fakeTx) GetSource(_ context.Context, id models.ID) (*models.Source, error) {
	s, ok := f.sources[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *s

	return &cp, nil
}

func (f *fakeTx) GetRepository(_ context.Context, id models.ID) (*models.Repository, error) {
	if f.repoGetErr != nil {
		return nil, f.repoGetErr
	}

	r, ok := f.repositories[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *r

	return &cp, nil
}

func (f *fakeTx) SaveSource(_ context.Context, s *models.Source) error {
	cp := *s
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует SourceStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func source(id models.ID, title string) *models.Source {
	return &models.Source{ID: id, Kind: models.SourceKindDocument, Title: title, Reliability: models.ReliabilityPrimary}
}

func TestUpdateSourceSaves(t *testing.T) {
	existing := source(srcID('V'), "Метрическая книга")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Title = "Метрическая книга (испр.)"

	if err := New(st).UpdateSource(context.Background(), updated); err != nil {
		t.Fatalf("UpdateSource: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Title != "Метрическая книга (испр.)" {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateSourceNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateSource(context.Background(), *source(srcID('V'), "Метрическая книга"))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateSourceValidatesBeforeTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateSource(context.Background(), models.Source{ID: srcID('V'), Kind: models.SourceKindDocument, Reliability: models.ReliabilityPrimary})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "title" {
		t.Fatalf("err = %v, want ValidationError on title", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestUpdateSourceWithRepositorySaves(t *testing.T) {
	existing := source(srcID('V'), "Метрическая книга")
	repo := &models.Repository{ID: rID('0'), Name: "ГАВО", Type: models.RepositoryTypeArchive}
	st := &fakeStore{tx: newFakeTx(existing).withRepository(repo)}

	updated := *existing
	updated.RepositoryID = repo.ID

	if err := New(st).UpdateSource(context.Background(), updated); err != nil {
		t.Fatalf("UpdateSource: %v", err)
	}

	if len(st.tx.saved) != 1 || st.tx.saved[0].RepositoryID != repo.ID {
		t.Fatalf("saved = %v", st.tx.saved)
	}
}

func TestUpdateSourceRepositoryNotFound(t *testing.T) {
	existing := source(srcID('V'), "Метрическая книга")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.RepositoryID = rID('0')

	err := New(st).UpdateSource(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "repository_id" {
		t.Fatalf("err = %v, want ValidationError on repository_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d источников при несуществующем хранилище", len(st.tx.saved))
	}
}

// TestUpdateSourcePropagatesRepositoryGetError проверяет, что настоящий сбой
// хранилища (не models.ErrNotFound) при проверке repository_id пробрасывается
// как есть, а не превращается в *models.ValidationError — см.
// create_source/scenario_test.go: TestCreateSourcePropagatesRepositoryGetError
// для того же контракта на создании.
func TestUpdateSourcePropagatesRepositoryGetError(t *testing.T) {
	wantErr := errors.New("get failed")
	existing := source(srcID('V'), "Метрическая книга")
	st := &fakeStore{tx: newFakeTx(existing)}
	st.tx.repoGetErr = wantErr

	updated := *existing
	updated.RepositoryID = rID('0')

	err := New(st).UpdateSource(context.Background(), updated)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}

	var ve *models.ValidationError
	if errors.As(err, &ve) {
		t.Fatalf("сбой хранилища превращён в *ValidationError: %v", err)
	}
}
```

#### `internal/usecases/delete_source/deps.go` (создать)
`internal/usecases/delete_source/deps.go`:
```go
package delete_source

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// SourceRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type SourceRepo interface {
	DeleteSource(ctx context.Context, id models.ID) error
}
```

#### `internal/usecases/delete_source/scenario.go` (создать)
`internal/usecases/delete_source/scenario.go`:
```go
package delete_source

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление записи источника».
type Scenario struct {
	sources SourceRepo
}

// New создаёт сценарий.
func New(sources SourceRepo) *Scenario {
	return &Scenario{sources: sources}
}

// DeleteSource удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteSource(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.sources.DeleteSource(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeSource)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeSource,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

#### `internal/usecases/delete_source/scenario_test.go` (создать)
`internal/usecases/delete_source/scenario_test.go`:
```go
package delete_source

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

func (f *fakeRepo) DeleteSource(_ context.Context, id models.ID) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)

	return nil
}

func TestDeleteSourceCallsRepo(t *testing.T) {
	repo := &fakeRepo{}
	id := models.ID("S-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	if err := New(repo).DeleteSource(context.Background(), id); err != nil {
		t.Fatalf("DeleteSource: %v", err)
	}

	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("deleted = %v", repo.deleted)
	}
}

func TestDeleteSourceInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	err := New(repo).DeleteSource(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}

	if len(repo.deleted) != 0 {
		t.Fatalf("repo вызван при невалидном id")
	}
}

func TestDeleteSourcePropagatesInUseError(t *testing.T) {
	wantErr := &models.InUseError{Type: models.TypeSource, ID: "S-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	repo := &fakeRepo{err: wantErr}

	err := New(repo).DeleteSource(context.Background(), "S-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	var iu *models.InUseError
	if !errors.As(err, &iu) {
		t.Fatalf("err = %v, want InUseError", err)
	}
}
```

#### `internal/httpapi/source.go` (создать)
`internal/httpapi/source.go`:
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleSourceList — GET /api/sources?limit=&offset=. Чтение открыто
// анонимному посетителю.
func handleSourceList(sources SourceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := sources.ListSources(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.SourcesFromModels(list))
	}
}

// handleSourceSearch — GET /api/sources/search?q=&limit=&offset=.
func handleSourceSearch(sources SourceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := sources.SearchSources(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.SourcesFromModels(list))
	}
}

// handleSourceGet — GET /api/sources/{id}.
func handleSourceGet(sources SourceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		src, err := sources.GetSource(r.Context(), AccessFromContext(r.Context()), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.SourceFromModel(src))
	}
}
```

#### `internal/httpapi/source_write.go` (создать)
`internal/httpapi/source_write.go`:
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleSourceCreate — POST /api/sources: создаёт запись, отвечает 201 с
// созданной записью (id генерирует сценарий). Несуществующее repository_id —
// 422, по образцу handleArchiveCreate. Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handleSourceCreate(sources SourceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.SourceCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := sources.CreateSource(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.SourceFromModel(created))
	}
}

// handleSourceUpdate — PUT /api/sources/{id}: полная замена
// kind/title/author/date/reliability/repository_id/notes/private. Читает
// текущую версию, накладывает поля запроса (fetch-then-merge).
func handleSourceUpdate(sources SourceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.SourceUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := sources.GetSource(r.Context(), AccessFromContext(r.Context()), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Kind = m.Kind
		cur.Title = m.Title
		cur.Author = m.Author
		cur.Date = m.Date
		cur.Reliability = m.Reliability
		cur.RepositoryID = m.RepositoryID
		cur.Notes = m.Notes
		cur.Private = m.Private

		if err := sources.UpdateSource(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.SourceFromModel(cur))
	}
}

// handleSourceDelete — DELETE /api/sources/{id}: 204 без тела; занятая
// запись — 409 со списком ссылающихся. Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handleSourceDelete(sources SourceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := sources.DeleteSource(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
```

#### `internal/httpapi/source_test.go` (создать)
`internal/httpapi/source_test.go`:
```go
package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeSources struct {
	list []models.Source
	err  error
	page models.Page

	getS      models.Source
	gotIDs    []models.ID
	created   models.Source
	gotCreate models.Source
	updated   models.Source
	deleteErr error

	search    []models.Source
	gotSearch models.SearchQuery
}

func (f *fakeSources) ListSources(_ context.Context, _ models.Access, page models.Page) ([]models.Source, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeSources) SearchSources(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Source, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeSources) GetSource(_ context.Context, _ models.Access, id models.ID) (models.Source, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Source{}, f.err
	}

	return f.getS, nil
}

func (f *fakeSources) CreateSource(_ context.Context, s models.Source) (models.Source, error) {
	f.gotCreate = s
	if f.err != nil {
		return models.Source{}, f.err
	}

	return f.created, nil
}

func (f *fakeSources) UpdateSource(_ context.Context, s models.Source) error {
	f.updated = s

	return f.err
}

func (f *fakeSources) DeleteSource(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestSourceListReturnsRecords(t *testing.T) {
	svc := &fakeSources{list: []models.Source{{
		ID:          "S-1",
		Kind:        models.SourceKindDocument,
		Title:       "Ревизская сказка",
		Reliability: models.ReliabilityPrimary,
	}}}

	rec := get(t, NewHandler(Deps{Sources: svc, DocsFS: fstest.MapFS{}}), "/api/sources")
	requireStatus(t, rec, 200)

	want := `[{"id":"S-1","kind":"document","title":"Ревизская сказка","reliability":"primary","notes":[],"private":false}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestSourceGetNotFound(t *testing.T) {
	svc := &fakeSources{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Sources: svc, DocsFS: fstest.MapFS{}}), "/api/sources/S-1")
	requireStatus(t, rec, 404)
}

func TestSourceSearchPassesQuery(t *testing.T) {
	svc := &fakeSources{}

	rec := get(t, NewHandler(Deps{Sources: svc, DocsFS: fstest.MapFS{}}), "/api/sources/search?q=Ревиз")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "Ревиз" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
```

#### `internal/httpapi/source_write_test.go` (создать)
`internal/httpapi/source_write_test.go`:
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

func TestSourceCreateContract(t *testing.T) {
	svc := &fakeSources{created: models.Source{ID: "S-1", Title: "Ревизская сказка"}}

	rec := postD(t, NewHandler(Deps{Sources: svc, DocsFS: fstest.MapFS{}}), "/api/sources",
		`{"kind":"document","title":"Ревизская сказка","reliability":"primary","repository_id":"R-1",`+
			`"date":{"year":1858,"precision":"year","modifier":"exact"}}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Kind != models.SourceKindDocument || svc.gotCreate.Title != "Ревизская сказка" ||
		svc.gotCreate.Reliability != models.ReliabilityPrimary || svc.gotCreate.RepositoryID != "R-1" ||
		svc.gotCreate.Date == nil || svc.gotCreate.Date.Year != 1858 || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestSourceCreateRepositoryNotFoundIs422 — usecase-слой возвращает
// *models.ValidationError по полю repository_id при несуществующем
// хранилище (см. create_source), writeError мапит это на 422, по образцу
// create_archive.
func TestSourceCreateRepositoryNotFoundIs422(t *testing.T) {
	svc := &fakeSources{err: &models.ValidationError{Entity: models.TypeSource, Field: "repository_id", Reason: "хранилище не найдено"}}

	rec := postD(t, NewHandler(Deps{Sources: svc, DocsFS: fstest.MapFS{}}), "/api/sources",
		`{"kind":"document","title":"Ревизская сказка","reliability":"primary","repository_id":"R-999"}`)

	requireStatus(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"repository_id"`) {
		t.Fatalf("body = %s, ожидалось поле repository_id", rec.Body)
	}
}

func TestSourceCreateAnonymousIs401(t *testing.T) {
	svc := &fakeSources{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Sources: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/sources", strings.NewReader(`{"kind":"document","title":"Ревизская сказка","reliability":"primary"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

// TestSourceUpdateMergesFields — проверяет все поля, включая структурированную
// date и reliability, по образцу TestArchiveUpdateMergesFields/TestParishUpdateMergesFields.
func TestSourceUpdateMergesFields(t *testing.T) {
	svc := &fakeSources{getS: models.Source{ID: "S-1", Title: "Ревизская сказка"}}

	rec := putD(t, NewHandler(Deps{Sources: svc, DocsFS: fstest.MapFS{}}), "/api/sources/S-1",
		`{"kind":"transcription","title":"Ревизская сказка (испр.)","author":"Иванов",`+
			`"date":{"year":1858,"precision":"year","modifier":"exact"},`+
			`"reliability":"contemporary","repository_id":"R-2","private":true}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Kind != models.SourceKindTranscription || svc.updated.Title != "Ревизская сказка (испр.)" ||
		svc.updated.Author != "Иванов" || svc.updated.Date == nil || svc.updated.Date.Year != 1858 ||
		svc.updated.Reliability != models.ReliabilityContemporary || svc.updated.RepositoryID != "R-2" ||
		svc.updated.Private != true || svc.updated.ID != "S-1" {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestSourceDeleteNoContent(t *testing.T) {
	svc := &fakeSources{}

	rec := delD(t, NewHandler(Deps{Sources: svc, DocsFS: fstest.MapFS{}}), "/api/sources/S-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestSourceDeleteInUseIs409(t *testing.T) {
	svc := &fakeSources{deleteErr: &models.InUseError{Type: models.TypeSource, ID: "S-1"}}

	rec := delD(t, NewHandler(Deps{Sources: svc, DocsFS: fstest.MapFS{}}), "/api/sources/S-1")

	requireStatus(t, rec, http.StatusConflict)
}
```

#### `internal/mcp/source.go` (создать)
`internal/mcp/source.go`:
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

// registerSourceTools регистрирует тулы для работы с источниками
// доказательств. date — структурированная дата (объект, см.
// factDateObjectProperties), по образцу Parish.Since/Until. repository_id —
// просто id (не объект TextRef): пустая строка — без хранилища; сценарий
// проверяет существование при непустом значении.
func registerSourceTools(s *server.MCPServer, sources SourceService) {
	tool := mcp.NewTool(
		"source_list",
		mcp.WithDescription("Список источников в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, sourceListHandler(sources))

	tool = mcp.NewTool(
		"source_search",
		mcp.WithDescription("Поиск источников по началу названия или автора; результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало названия или автора")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, sourceSearchHandler(sources))

	tool = mcp.NewTool(
		"source_get",
		mcp.WithDescription("Источник по id; результат — JSON записи. Неверный формат id, отсутствующая или приватная (без полного доступа) запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например S-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, sourceGetHandler(sources))

	tool = mcp.NewTool(
		"source_create",
		mcp.WithDescription("Создать источник; id генерируется сервером; результат — JSON созданной записи. Несуществующий repository_id — ошибка тула"),
		mcp.WithString("kind", mcp.Required(), mcp.Enum("archival-scan", "transcription", "document", "audio", "photo", "memory", "external"), mcp.Description("Вид источника")),
		mcp.WithString("title", mcp.Required(), mcp.Description("Название")),
		mcp.WithString("author", mcp.Description("Автор")),
		mcp.WithObject("date", mcp.Description("Дата источника"), mcp.Properties(factDateObjectProperties())),
		mcp.WithString("reliability", mcp.Required(), mcp.Enum("primary", "contemporary", "memory", "indirect", "unknown"), mcp.Description("Общая достоверность")),
		mcp.WithString("repository_id", mcp.Description("id хранилища (необязательно; пусто — без хранилища)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, sourceCreateHandler(sources))

	tool = mcp.NewTool(
		"source_update",
		mcp.WithDescription("Изменить источник: полная замена kind/title/author/date/reliability/repository_id/notes/private; результат — JSON обновлённой записи. Несуществующий repository_id — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("kind", mcp.Required(), mcp.Enum("archival-scan", "transcription", "document", "audio", "photo", "memory", "external"), mcp.Description("Вид источника")),
		mcp.WithString("title", mcp.Required(), mcp.Description("Название")),
		mcp.WithString("author", mcp.Description("Автор")),
		mcp.WithObject("date", mcp.Description("Дата источника"), mcp.Properties(factDateObjectProperties())),
		mcp.WithString("reliability", mcp.Required(), mcp.Enum("primary", "contemporary", "memory", "indirect", "unknown"), mcp.Description("Общая достоверность")),
		mcp.WithString("repository_id", mcp.Description("id хранилища (необязательно)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, sourceUpdateHandler(sources))

	tool = mcp.NewTool(
		"source_delete",
		mcp.WithDescription("Удалить источник. Необратимо. Если на него есть строгие ссылки от других сущностей (например, Citation) — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, sourceDeleteHandler(sources))
}

func sourceListHandler(sources SourceService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := sources.ListSources(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.SourcesFromModels(list))
	}
}

func sourceSearchHandler(sources SourceService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := sources.SearchSources(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.SourcesFromModels(list))
	}
}

func sourceGetHandler(sources SourceService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		src, err := sources.GetSource(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.SourceFromModel(src))
	}
}

func sourceCreateHandler(sources SourceService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		date, err := optionalFactDate(req.GetArguments(), "date")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		src := models.Source{
			Kind:         models.SourceKind(req.GetString("kind", "")),
			Title:        req.GetString("title", ""),
			Author:       req.GetString("author", ""),
			Date:         date,
			Reliability:  models.Reliability(req.GetString("reliability", "")),
			RepositoryID: models.ID(req.GetString("repository_id", "")),
			Notes:        textRefsFromStrings(req.GetStringSlice("notes", nil)),
			Private:      req.GetBool("private", false),
		}

		created, err := sources.CreateSource(ctx, src)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.SourceFromModel(created))
	}
}

func sourceUpdateHandler(sources SourceService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := sources.GetSource(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		date, err := optionalFactDate(req.GetArguments(), "date")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cur.Kind = models.SourceKind(req.GetString("kind", ""))
		cur.Title = req.GetString("title", "")
		cur.Author = req.GetString("author", "")
		cur.Date = date
		cur.Reliability = models.Reliability(req.GetString("reliability", ""))
		cur.RepositoryID = models.ID(req.GetString("repository_id", ""))
		cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))
		cur.Private = req.GetBool("private", false)

		if err := sources.UpdateSource(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.SourceFromModel(cur))
	}
}

func sourceDeleteHandler(sources SourceService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := sources.DeleteSource(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
```

#### `internal/mcp/source_test.go` (создать)
`internal/mcp/source_test.go`:
```go
package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakeSources struct {
	list []models.Source
	err  error

	getS      models.Source
	created   models.Source
	gotCreate models.Source
	updated   models.Source
	gotIDs    []models.ID
	deleteErr error

	search []models.Source
}

func (f *fakeSources) ListSources(context.Context, models.Access, models.Page) ([]models.Source, error) {
	return f.list, f.err
}

func (f *fakeSources) SearchSources(context.Context, models.Access, models.SearchQuery) ([]models.Source, error) {
	return f.search, f.err
}

func (f *fakeSources) GetSource(_ context.Context, _ models.Access, id models.ID) (models.Source, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Source{}, f.err
	}

	return f.getS, nil
}

func (f *fakeSources) CreateSource(_ context.Context, s models.Source) (models.Source, error) {
	f.gotCreate = s
	if f.err != nil {
		return models.Source{}, f.err
	}

	return f.created, nil
}

func (f *fakeSources) UpdateSource(_ context.Context, s models.Source) error {
	f.updated = s

	return f.err
}

func (f *fakeSources) DeleteSource(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callSourceTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestSourceGetToolContract(t *testing.T) {
	svc := &fakeSources{getS: models.Source{ID: "S-1", Title: "Ревизская сказка"}}

	res := callSourceTool(t, sourceGetHandler(svc), map[string]any{"id": "S-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "S-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

// TestSourceCreateToolPassesRepositoryID — repository_id передаётся плоской
// строкой (не объектом), по образцу TestArchiveCreateToolPassesRepositoryID.
func TestSourceCreateToolPassesRepositoryID(t *testing.T) {
	svc := &fakeSources{created: models.Source{ID: "S-new", Title: "Ревизская сказка"}}

	res := callSourceTool(t, sourceCreateHandler(svc), map[string]any{
		"kind":          "document",
		"title":         "Ревизская сказка",
		"reliability":   "primary",
		"repository_id": "R-1",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.Title != "Ревизская сказка" || svc.gotCreate.RepositoryID != "R-1" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestSourceCreateToolRepositoryNotFoundIsError — usecase-слой возвращает
// ошибку на несуществующее хранилище, тул должен отдать её как ошибку тула
// (не паниковать, не проглатывать).
func TestSourceCreateToolRepositoryNotFoundIsError(t *testing.T) {
	svc := &fakeSources{err: &models.ValidationError{Entity: models.TypeSource, Field: "repository_id", Reason: "не найдено"}}

	res := callSourceTool(t, sourceCreateHandler(svc), map[string]any{
		"kind":          "document",
		"title":         "Ревизская сказка",
		"reliability":   "primary",
		"repository_id": "R-999",
	})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestSourceDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeSources{deleteErr: &models.InUseError{Type: models.TypeSource, ID: "S-1"}}

	res := callSourceTool(t, sourceDeleteHandler(svc), map[string]any{"id": "S-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersSourceTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Sources: &fakeSources{}}).ListTools()

	for _, name := range []string{"source_list", "source_search", "source_get", "source_create", "source_update", "source_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
```

### Шаг 1.4. Citation — транспорт, usecases, httpapi, MCP

#### `internal/transport/citation.go` (создать)
`internal/transport/citation.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// Citation — контракт цитаты из источника (GET /api/citations, MCP-тул
// citation_list). Anchor — полиморфная привязка «где именно» (см.
// transport.Anchor), опциональна.
type Citation struct {
	ID       models.ID `json:"id"`
	SourceID string    `json:"source_id"`
	Anchor   *Anchor   `json:"anchor,omitempty"`
	Text     string    `json:"text,omitempty"`
	Note     string    `json:"note,omitempty"`
	Private  bool      `json:"private"`
}

// CitationFromModel конвертирует запись в контракт.
func CitationFromModel(c models.Citation) Citation {
	return Citation{
		ID:       c.ID,
		SourceID: string(c.SourceID),
		Anchor:   AnchorFromModel(c.Anchor),
		Text:     c.Text,
		Note:     c.Note,
		Private:  c.Private,
	}
}

// CitationsFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func CitationsFromModels(cs []models.Citation) []Citation {
	out := make([]Citation, 0, len(cs))
	for _, c := range cs {
		out = append(out, CitationFromModel(c))
	}

	return out
}
```

#### `internal/transport/citation_write.go` (создать)
`internal/transport/citation_write.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// CitationCreate — тело POST /api/citations и аргументы тула
// citation_create. Идентификатор генерирует сценарий. SourceID — строгая
// ссылка на источник (сценарий проверяет существование). Anchor —
// необязательная полиморфная привязка (см. transport.Anchor).
type CitationCreate struct {
	SourceID string  `json:"source_id"`
	Anchor   *Anchor `json:"anchor,omitempty"`
	Text     string  `json:"text,omitempty"`
	Note     string  `json:"note,omitempty"`
	Private  bool    `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (c CitationCreate) Model() models.Citation {
	return models.Citation{
		SourceID: models.ID(c.SourceID),
		Anchor:   c.Anchor.Model(),
		Text:     c.Text,
		Note:     c.Note,
		Private:  c.Private,
	}
}

// CitationUpdate — тело PUT /api/citations/{id} и аргументы тула
// citation_update: полная замена source_id/anchor/text/note/private.
type CitationUpdate struct {
	SourceID string  `json:"source_id"`
	Anchor   *Anchor `json:"anchor,omitempty"`
	Text     string  `json:"text,omitempty"`
	Note     string  `json:"note,omitempty"`
	Private  bool    `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (c CitationUpdate) Model() models.Citation {
	return models.Citation{
		SourceID: models.ID(c.SourceID),
		Anchor:   c.Anchor.Model(),
		Text:     c.Text,
		Note:     c.Note,
		Private:  c.Private,
	}
}
```

#### `internal/usecases/list_citations/deps.go` (создать)
`internal/usecases/list_citations/deps.go`:
```go
package list_citations

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// CitationRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -citation $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type CitationRepo interface {
	ListCitations(ctx context.Context, access models.Access, page models.Page) ([]*models.Citation, error)
}
```

#### `internal/usecases/list_citations/scenario.go` (создать)
`internal/usecases/list_citations/scenario.go`:
```go
package list_citations

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список цитат».
type Scenario struct {
	citations CitationRepo
}

// New создаёт сценарий.
func New(citations CitationRepo) *Scenario {
	return &Scenario{citations: citations}
}

// ListCitations возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListCitations(ctx context.Context, access models.Access, page models.Page) ([]models.Citation, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeCitation, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeCitation, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.citations.ListCitations(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Citation, 0, len(list))
	for _, a := range list {
		out = append(out, *a)
	}

	return out, nil
}
```

#### `internal/usecases/list_citations/scenario_test.go` (создать)
`internal/usecases/list_citations/scenario_test.go`:
```go
package list_citations

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.Citation
}

func (f *fakeRepo) ListCitations(_ context.Context, _ models.Access, page models.Page) ([]*models.Citation, error) {
	f.page = page

	return f.out, nil
}

func TestListCitationsReturnsRecords(t *testing.T) {
	src := models.ID("S-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{out: []*models.Citation{{ID: "C-01ARZ3NDEKTSV4RRFFQ69G5FA1", SourceID: src, Text: "стр. 12"}}}

	got, err := New(repo).ListCitations(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListCitations: %v", err)
	}

	if len(got) != 1 || got[0].Text != "стр. 12" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListCitationsRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListCitations(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
```

#### `internal/usecases/search_citations/deps.go` (создать)
`internal/usecases/search_citations/deps.go`:
```go
package search_citations

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// CitationRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -citation $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type CitationRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetCitation(ctx context.Context, id models.ID) (*models.Citation, error)
}
```

#### `internal/usecases/search_citations/scenario.go` (создать)
`internal/usecases/search_citations/scenario.go`:
```go
package search_citations

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск цитат».
type Scenario struct {
	citations CitationRepo
}

// New создаёт сценарий.
func New(citations CitationRepo) *Scenario {
	return &Scenario{citations: citations}
}

// SearchCitations находит цитаты, чей текст начинается с текста запроса
// (sqlstore.SaveCitation индексирует только text) — та же механика, что и
// search_divisions.SearchDivisions (см. её комментарий).
func (s *Scenario) SearchCitations(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Citation, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Citation{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Citation{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.citations.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeCitation {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.citations.GetCitation(ctx, h.ID)
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

#### `internal/usecases/search_citations/scenario_test.go` (создать)
`internal/usecases/search_citations/scenario_test.go`:
```go
package search_citations

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits      []models.Hit
	citations map[models.ID]*models.Citation
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return c, nil
}

func TestSearchCitationsFiltersByType(t *testing.T) {
	id := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	src := models.ID("S-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeCitation, ID: id},
			{Type: models.TypeAdministrativeDivision, ID: otherID}, // не citation — должен быть пропущен
		},
		citations: map[models.ID]*models.Citation{id: {ID: id, SourceID: src, Text: "стр. 12, запись о рождении"}},
	}

	got, err := New(repo).SearchCitations(context.Background(), models.AccessFull, models.SearchQuery{Text: "стр"})
	if err != nil {
		t.Fatalf("SearchCitations: %v", err)
	}

	if len(got) != 1 || got[0].Text != "стр. 12, запись о рождении" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchCitationsEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchCitations(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchCitations: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
```

#### `internal/usecases/get_citation/deps.go` (создать)
`internal/usecases/get_citation/deps.go`:
```go
package get_citation

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// CitationRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type CitationRepo interface {
	GetCitation(ctx context.Context, id models.ID) (*models.Citation, error)
}
```

#### `internal/usecases/get_citation/scenario.go` (создать)
`internal/usecases/get_citation/scenario.go`:
```go
package get_citation

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «цитата по идентификатору».
type Scenario struct {
	citations CitationRepo
}

// New создаёт сценарий.
func New(citations CitationRepo) *Scenario {
	return &Scenario{citations: citations}
}

// GetCitation возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound. Приватная запись
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound — тот же принцип «прячем как отсутствующее», что и в
// List/Search.
func (s *Scenario) GetCitation(ctx context.Context, access models.Access, id models.ID) (models.Citation, error) {
	if err := validateID(id); err != nil {
		return models.Citation{}, err
	}

	c, err := s.citations.GetCitation(ctx, id)
	if err != nil {
		return models.Citation{}, err
	}

	if c.Private && access != models.AccessFull {
		return models.Citation{}, models.ErrNotFound
	}

	return *c, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeCitation)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeCitation,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

#### `internal/usecases/get_citation/scenario_test.go` (создать)
`internal/usecases/get_citation/scenario_test.go`:
```go
package get_citation

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	citations map[models.ID]*models.Citation
}

func (f *fakeRepo) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return c, nil
}

func TestGetCitationReturnsRecord(t *testing.T) {
	id := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	src := models.ID("S-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{citations: map[models.ID]*models.Citation{id: {ID: id, SourceID: src, Text: "стр. 12"}}}

	got, err := New(repo).GetCitation(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetCitation: %v", err)
	}

	if got.Text != "стр. 12" {
		t.Fatalf("Text = %q", got.Text)
	}
}

func TestGetCitationNotFound(t *testing.T) {
	repo := &fakeRepo{citations: map[models.ID]*models.Citation{}}

	_, err := New(repo).GetCitation(context.Background(), models.AccessFull, "C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetCitationInvalidID(t *testing.T) {
	repo := &fakeRepo{citations: map[models.ID]*models.Citation{}}

	_, err := New(repo).GetCitation(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestGetCitationPrivateHiddenFromPublic(t *testing.T) {
	id := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	src := models.ID("S-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{citations: map[models.ID]*models.Citation{id: {ID: id, SourceID: src, Text: "стр. 12", Private: true}}}

	_, err := New(repo).GetCitation(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for private record with AccessPublic", err)
	}
}

func TestGetCitationPrivateVisibleToFullAccess(t *testing.T) {
	id := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	src := models.ID("S-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{citations: map[models.ID]*models.Citation{id: {ID: id, SourceID: src, Text: "стр. 12", Private: true}}}

	got, err := New(repo).GetCitation(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetCitation: %v", err)
	}

	if got.Text != "стр. 12" {
		t.Fatalf("Text = %q", got.Text)
	}
}
```

#### `internal/usecases/create_citation/deps.go` (создать)
`internal/usecases/create_citation/deps.go`:
```go
package create_citation

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// CitationStore — зависимость сценария: транзакция порта store.Store.
// Проверка источника, проверка ссылок внутри якоря (если задан) и
// сохранение идут в одной транзакции на переданном fn хранилище.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type CitationStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
```

#### `internal/usecases/create_citation/scenario.go` (создать)
`internal/usecases/create_citation/scenario.go`:
```go
package create_citation

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание цитаты».
type Scenario struct {
	store CitationStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st CitationStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateCitation создаёт цитату: генерирует идентификатор, проверяет
// инварианты, в одной транзакции убеждается в существовании источника
// (SourceID — обязательная строгая ссылка, в отличие от Archive.RepositoryID)
// и, если якорь задан и ссылается на другую сущность (ArchiveAnchor.NodeID/
// DocumentID, FileAnchor.AttachmentID), убеждается в её существовании тоже —
// по тому же принципу, что create_attachment проверяет NodeID/DocumentID
// через generic-хранилище, даже если у цели ещё нет своего CRUD-слоя
// (ArchiveNode/ArchiveDocument — подпроект 6). URLAnchor ссылок не несёт.
//
// Ошибки: непустой входной ID, невалидная сущность и несуществующая
// ссылка — *models.ValidationError (поля id, source_id, anchor.node_id,
// anchor.document_id, anchor.attachment_id); прочее — ошибки хранилища как
// есть.
func (s *Scenario) CreateCitation(ctx context.Context, c models.Citation) (models.Citation, error) {
	if c.ID != "" {
		return models.Citation{}, &models.ValidationError{
			Entity: models.TypeCitation,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", c.ID),
		}
	}

	c.ID = s.ids.New(models.TypeCitation)

	if err := c.Validate(); err != nil {
		return models.Citation{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetSource(ctx, c.SourceID); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return fieldErr("source_id", "источник %q не найден", c.SourceID)
			}

			return err
		}

		if err := checkAnchorRefs(ctx, tx, c.Anchor); err != nil {
			return err
		}

		return tx.SaveCitation(ctx, &c)
	})
	if err != nil {
		return models.Citation{}, err
	}

	return c, nil
}

// checkAnchorRefs проверяет существование ссылок внутри якоря (если он
// задан и несёт ссылку); ArchiveAnchor.DocumentID — только если задан.
func checkAnchorRefs(ctx context.Context, tx store.Store, a models.Anchor) error {
	switch v := a.(type) {
	case nil:
		return nil
	case *models.ArchiveAnchor:
		if v == nil {
			return nil
		}

		if _, err := tx.GetArchiveNode(ctx, v.NodeID); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return fieldErr("anchor.node_id", "архивный узел %q не найден", v.NodeID)
			}

			return err
		}

		if v.DocumentID != "" {
			if _, err := tx.GetArchiveDocument(ctx, v.DocumentID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return fieldErr("anchor.document_id", "архивный документ %q не найден", v.DocumentID)
				}

				return err
			}
		}

		return nil
	case *models.FileAnchor:
		if v == nil {
			return nil
		}

		if _, err := tx.GetAttachment(ctx, v.AttachmentID); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return fieldErr("anchor.attachment_id", "вложение %q не найдено", v.AttachmentID)
			}

			return err
		}

		return nil
	default:
		return nil
	}
}

// fieldErr — *models.ValidationError по указанному полю.
func fieldErr(field, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeCitation,
		Field:  field,
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/create_citation/scenario_test.go` (создать)
`internal/usecases/create_citation/scenario_test.go`:
```go
package create_citation

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// citID возвращает корректный идентификатор цитаты, отличающийся последним символом.
func citID(last byte) models.ID {
	return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func srcID(last byte) models.ID {
	return models.ID("S-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func nodeID(last byte) models.ID {
	return models.ID("AN-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func docID(last byte) models.ID {
	return models.ID("DC-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func attID(last byte) models.ID {
	return models.ID("O-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

type stubIDs struct {
	id      models.ID
	gotType models.Type
}

func (s *stubIDs) New(t models.Type) models.ID {
	s.gotType = t

	return s.id
}

// fakeTx реализует нужные сценарию методы store.Store поверх карт источников,
// узлов, документов и вложений; остальные методы порта паникуют через
// nil-встраивание.
type fakeTx struct {
	store.Store
	sources     map[models.ID]*models.Source
	nodes       map[models.ID]*models.ArchiveNode
	documents   map[models.ID]*models.ArchiveDocument
	attachments map[models.ID]*models.Attachment
	saved       []*models.Citation
	saveErr     error
}

func newFakeTx(existingSources ...*models.Source) *fakeTx {
	tx := &fakeTx{
		sources:     map[models.ID]*models.Source{},
		nodes:       map[models.ID]*models.ArchiveNode{},
		documents:   map[models.ID]*models.ArchiveDocument{},
		attachments: map[models.ID]*models.Attachment{},
	}
	for _, s := range existingSources {
		tx.sources[s.ID] = s
	}

	return tx
}

func (f *fakeTx) withNode(id models.ID) *fakeTx {
	f.nodes[id] = &models.ArchiveNode{ID: id}

	return f
}

func (f *fakeTx) withDocument(id models.ID) *fakeTx {
	f.documents[id] = &models.ArchiveDocument{ID: id}

	return f
}

func (f *fakeTx) withAttachment(id models.ID) *fakeTx {
	f.attachments[id] = &models.Attachment{ID: id}

	return f
}

func (f *fakeTx) GetSource(_ context.Context, id models.ID) (*models.Source, error) {
	s, ok := f.sources[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
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

func (f *fakeTx) GetAttachment(_ context.Context, id models.ID) (*models.Attachment, error) {
	a, ok := f.attachments[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return a, nil
}

func (f *fakeTx) SaveCitation(_ context.Context, c *models.Citation) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *c
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует CitationStore: InTx выполняет fn на встроенном fakeTx.
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

func validInput(source models.ID) models.Citation {
	return models.Citation{SourceID: source, Text: "стр. 12, запись о рождении"}
}

func TestCreateCitationGeneratesIDAndSaves(t *testing.T) {
	src := srcID('0')
	st := &fakeStore{tx: newFakeTx(&models.Source{ID: src})}
	ids := &stubIDs{id: citID('V')}

	got, err := New(st, ids).CreateCitation(context.Background(), validInput(src))
	if err != nil {
		t.Fatalf("CreateCitation: %v", err)
	}

	if got.ID != citID('V') || got.SourceID != src {
		t.Fatalf("got %+v, ожидалась цитата с ID %v", got, citID('V'))
	}

	if ids.gotType != models.TypeCitation {
		t.Errorf("генератор вызван с типом %q, ожидался citation", ids.gotType)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].ID != citID('V') {
		t.Fatalf("InTx=%d, saved=%v; ожидалось одно сохранение внутри одной транзакции", st.calls, st.tx.saved)
	}
}

func TestCreateCitationRejectsExplicitID(t *testing.T) {
	src := srcID('0')
	st := &fakeStore{tx: newFakeTx(&models.Source{ID: src})}
	ids := &stubIDs{id: citID('V')}

	in := validInput(src)
	in.ID = citID('0')

	_, err := New(st, ids).CreateCitation(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю id", err)
	}

	if st.calls != 0 || ids.gotType != "" {
		t.Fatalf("InTx=%d, генератор вызван с %q; ожидалось: не вызваны", st.calls, ids.gotType)
	}
}

// TestCreateCitationValidatesBeforeTx: якорь структурно невалиден (номер
// страницы меньше 1) — ошибка возвращается ещё до похода в хранилище, хотя
// узел якоря существует и мог бы пройти проверку ссылки.
func TestCreateCitationValidatesBeforeTx(t *testing.T) {
	src := srcID('0')
	st := &fakeStore{tx: newFakeTx(&models.Source{ID: src}).withNode(nodeID('0'))}

	in := validInput(src)
	in.Anchor = &models.ArchiveAnchor{NodeID: nodeID('0'), Page: 0}

	_, err := New(st, &stubIDs{id: citID('V')}).CreateCitation(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "anchor.page" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю anchor.page", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestCreateCitationSourceNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	_, err := New(st, &stubIDs{id: citID('V')}).CreateCitation(context.Background(), validInput(srcID('0')))

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "source_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю source_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d цитат при несуществующем источнике", len(st.tx.saved))
	}
}

func TestCreateCitationSavesWithURLAnchor(t *testing.T) {
	src := srcID('0')
	st := &fakeStore{tx: newFakeTx(&models.Source{ID: src})}

	in := validInput(src)
	in.Anchor = &models.URLAnchor{URL: "https://example.org/scan/12"}

	got, err := New(st, &stubIDs{id: citID('V')}).CreateCitation(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateCitation: %v", err)
	}

	if len(st.tx.saved) != 1 || got.Anchor.Kind() != models.AnchorURL {
		t.Fatalf("got %+v, saved=%d; ожидалось сохранение с url-якорем", got, len(st.tx.saved))
	}
}

func TestCreateCitationSavesWithArchiveAnchorValidNode(t *testing.T) {
	src, node := srcID('0'), nodeID('0')
	st := &fakeStore{tx: newFakeTx(&models.Source{ID: src}).withNode(node)}

	in := validInput(src)
	in.Anchor = &models.ArchiveAnchor{NodeID: node, Page: 1}

	got, err := New(st, &stubIDs{id: citID('V')}).CreateCitation(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateCitation: %v", err)
	}

	if len(st.tx.saved) != 1 || got.ID != citID('V') {
		t.Fatalf("got %+v, saved=%d; ожидалось сохранение с архивным якорем", got, len(st.tx.saved))
	}
}

func TestCreateCitationArchiveAnchorNodeNotFound(t *testing.T) {
	src := srcID('0')
	st := &fakeStore{tx: newFakeTx(&models.Source{ID: src})}

	in := validInput(src)
	in.Anchor = &models.ArchiveAnchor{NodeID: nodeID('0'), Page: 1}

	_, err := New(st, &stubIDs{id: citID('V')}).CreateCitation(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "anchor.node_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю anchor.node_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d цитат при несуществующем узле", len(st.tx.saved))
	}
}

func TestCreateCitationArchiveAnchorWithDocumentValid(t *testing.T) {
	src, node, doc := srcID('0'), nodeID('0'), docID('0')
	st := &fakeStore{tx: newFakeTx(&models.Source{ID: src}).withNode(node).withDocument(doc)}

	in := validInput(src)
	in.Anchor = &models.ArchiveAnchor{NodeID: node, DocumentID: doc, Page: 1}

	got, err := New(st, &stubIDs{id: citID('V')}).CreateCitation(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateCitation: %v", err)
	}

	if len(st.tx.saved) != 1 || got.ID != citID('V') {
		t.Fatalf("got %+v, saved=%d; ожидалось сохранение с архивным якорем и документом", got, len(st.tx.saved))
	}
}

func TestCreateCitationArchiveAnchorDocumentNotFound(t *testing.T) {
	src, node := srcID('0'), nodeID('0')
	st := &fakeStore{tx: newFakeTx(&models.Source{ID: src}).withNode(node)}

	in := validInput(src)
	in.Anchor = &models.ArchiveAnchor{NodeID: node, DocumentID: docID('9'), Page: 1}

	_, err := New(st, &stubIDs{id: citID('V')}).CreateCitation(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "anchor.document_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю anchor.document_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d цитат при несуществующем документе", len(st.tx.saved))
	}
}

func TestCreateCitationSavesWithFileAnchorValidAttachment(t *testing.T) {
	src, att := srcID('0'), attID('0')
	st := &fakeStore{tx: newFakeTx(&models.Source{ID: src}).withAttachment(att)}

	in := validInput(src)
	in.Anchor = &models.FileAnchor{AttachmentID: att}

	got, err := New(st, &stubIDs{id: citID('V')}).CreateCitation(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateCitation: %v", err)
	}

	if len(st.tx.saved) != 1 || got.ID != citID('V') {
		t.Fatalf("got %+v, saved=%d; ожидалось сохранение с файловым якорем", got, len(st.tx.saved))
	}
}

func TestCreateCitationFileAnchorAttachmentNotFound(t *testing.T) {
	src := srcID('0')
	st := &fakeStore{tx: newFakeTx(&models.Source{ID: src})}

	in := validInput(src)
	in.Anchor = &models.FileAnchor{AttachmentID: attID('0')}

	_, err := New(st, &stubIDs{id: citID('V')}).CreateCitation(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "anchor.attachment_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю anchor.attachment_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d цитат при несуществующем вложении", len(st.tx.saved))
	}
}

func TestCreateCitationPropagatesSaveError(t *testing.T) {
	src := srcID('0')
	wantErr := errors.New("save failed")
	st := &fakeStore{tx: newFakeTx(&models.Source{ID: src})}
	st.tx.saveErr = wantErr

	if _, err := New(st, &stubIDs{id: citID('V')}).CreateCitation(context.Background(), validInput(src)); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

func TestCreateCitationPropagatesTxError(t *testing.T) {
	src := srcID('0')
	wantErr := errors.New("tx failed")
	st := &fakeStore{tx: newFakeTx(&models.Source{ID: src}), inTxErr: wantErr}

	if _, err := New(st, &stubIDs{id: citID('V')}).CreateCitation(context.Background(), validInput(src)); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}
```

#### `internal/usecases/update_citation/deps.go` (создать)
`internal/usecases/update_citation/deps.go`:
```go
package update_citation

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// CitationStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования, проверка источника/ссылок якоря и сохранение идут
// в одной транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type CitationStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
```

#### `internal/usecases/update_citation/scenario.go` (создать)
`internal/usecases/update_citation/scenario.go`:
```go
package update_citation

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение цитаты».
type Scenario struct {
	store CitationStore
}

// New создаёт сценарий.
func New(st CitationStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateCitation полностью заменяет цитату по c.ID: проверяет инварианты, в
// одной транзакции убеждается, что цитата существует, источник существует,
// и (если якорь задан и несёт ссылку) ссылка внутри якоря существует, и
// сохраняет. По образцу create_citation.
//
// Ошибки: невалидная сущность и несуществующая ссылка —
// *models.ValidationError (source_id, anchor.node_id, anchor.document_id,
// anchor.attachment_id); нет такой цитаты — models.ErrNotFound; прочее —
// ошибки хранилища как есть.
func (s *Scenario) UpdateCitation(ctx context.Context, c models.Citation) error {
	if err := c.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetCitation(ctx, c.ID); err != nil {
			return err
		}

		if _, err := tx.GetSource(ctx, c.SourceID); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return fieldErr("source_id", "источник %q не найден", c.SourceID)
			}

			return err
		}

		if err := checkAnchorRefs(ctx, tx, c.Anchor); err != nil {
			return err
		}

		return tx.SaveCitation(ctx, &c)
	})
}

// checkAnchorRefs проверяет существование ссылок внутри якоря (если он
// задан и несёт ссылку); ArchiveAnchor.DocumentID — только если задан. По
// образцу create_citation.checkAnchorRefs.
func checkAnchorRefs(ctx context.Context, tx store.Store, a models.Anchor) error {
	switch v := a.(type) {
	case nil:
		return nil
	case *models.ArchiveAnchor:
		if v == nil {
			return nil
		}

		if _, err := tx.GetArchiveNode(ctx, v.NodeID); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return fieldErr("anchor.node_id", "архивный узел %q не найден", v.NodeID)
			}

			return err
		}

		if v.DocumentID != "" {
			if _, err := tx.GetArchiveDocument(ctx, v.DocumentID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return fieldErr("anchor.document_id", "архивный документ %q не найден", v.DocumentID)
				}

				return err
			}
		}

		return nil
	case *models.FileAnchor:
		if v == nil {
			return nil
		}

		if _, err := tx.GetAttachment(ctx, v.AttachmentID); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return fieldErr("anchor.attachment_id", "вложение %q не найдено", v.AttachmentID)
			}

			return err
		}

		return nil
	default:
		return nil
	}
}

// fieldErr — *models.ValidationError по указанному полю.
func fieldErr(field, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeCitation,
		Field:  field,
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/update_citation/scenario_test.go` (создать)
`internal/usecases/update_citation/scenario_test.go`:
```go
package update_citation

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

func citID(last byte) models.ID {
	return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func srcID(last byte) models.ID {
	return models.ID("S-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func nodeID(last byte) models.ID {
	return models.ID("AN-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func docID(last byte) models.ID {
	return models.ID("DC-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func attID(last byte) models.ID {
	return models.ID("O-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// fakeTx реализует нужные сценарию методы store.Store поверх карт цитат,
// источников, узлов, документов и вложений; остальные методы порта паникуют
// через nil-встраивание.
type fakeTx struct {
	store.Store
	citations   map[models.ID]*models.Citation
	sources     map[models.ID]*models.Source
	nodes       map[models.ID]*models.ArchiveNode
	documents   map[models.ID]*models.ArchiveDocument
	attachments map[models.ID]*models.Attachment
	saved       []*models.Citation
}

func newFakeTx(existingCitations ...*models.Citation) *fakeTx {
	tx := &fakeTx{
		citations:   map[models.ID]*models.Citation{},
		sources:     map[models.ID]*models.Source{},
		nodes:       map[models.ID]*models.ArchiveNode{},
		documents:   map[models.ID]*models.ArchiveDocument{},
		attachments: map[models.ID]*models.Attachment{},
	}
	for _, c := range existingCitations {
		tx.citations[c.ID] = c
	}

	return tx
}

func (f *fakeTx) withSource(id models.ID) *fakeTx {
	f.sources[id] = &models.Source{ID: id}

	return f
}

func (f *fakeTx) withNode(id models.ID) *fakeTx {
	f.nodes[id] = &models.ArchiveNode{ID: id}

	return f
}

func (f *fakeTx) withDocument(id models.ID) *fakeTx {
	f.documents[id] = &models.ArchiveDocument{ID: id}

	return f
}

func (f *fakeTx) withAttachment(id models.ID) *fakeTx {
	f.attachments[id] = &models.Attachment{ID: id}

	return f
}

func (f *fakeTx) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *c

	return &cp, nil
}

func (f *fakeTx) GetSource(_ context.Context, id models.ID) (*models.Source, error) {
	s, ok := f.sources[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return s, nil
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

func (f *fakeTx) GetAttachment(_ context.Context, id models.ID) (*models.Attachment, error) {
	a, ok := f.attachments[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return a, nil
}

func (f *fakeTx) SaveCitation(_ context.Context, c *models.Citation) error {
	cp := *c
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует CitationStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func citation(id, source models.ID) *models.Citation {
	return &models.Citation{ID: id, SourceID: source, Text: "стр. 12, запись о рождении"}
}

func TestUpdateCitationSaves(t *testing.T) {
	src := srcID('0')
	existing := citation(citID('V'), src)
	st := &fakeStore{tx: newFakeTx(existing).withSource(src)}

	updated := *existing
	updated.Text = "стр. 12, запись о рождении (испр.)"

	if err := New(st).UpdateCitation(context.Background(), updated); err != nil {
		t.Fatalf("UpdateCitation: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Text != "стр. 12, запись о рождении (испр.)" {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateCitationNotFound(t *testing.T) {
	src := srcID('0')
	st := &fakeStore{tx: newFakeTx().withSource(src)}

	err := New(st).UpdateCitation(context.Background(), *citation(citID('V'), src))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// TestUpdateCitationValidatesBeforeTx: якорь структурно невалиден (номер
// страницы меньше 1) — ошибка возвращается ещё до похода в хранилище.
func TestUpdateCitationValidatesBeforeTx(t *testing.T) {
	src := srcID('0')
	existing := citation(citID('V'), src)
	st := &fakeStore{tx: newFakeTx(existing).withSource(src).withNode(nodeID('0'))}

	updated := *existing
	updated.Anchor = &models.ArchiveAnchor{NodeID: nodeID('0'), Page: 0}

	err := New(st).UpdateCitation(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "anchor.page" {
		t.Fatalf("err = %v, want ValidationError on anchor.page", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestUpdateCitationSourceNotFound(t *testing.T) {
	src := srcID('0')
	existing := citation(citID('V'), src)
	st := &fakeStore{tx: newFakeTx(existing)}

	err := New(st).UpdateCitation(context.Background(), *existing)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "source_id" {
		t.Fatalf("err = %v, want ValidationError on source_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d цитат при несуществующем источнике", len(st.tx.saved))
	}
}

func TestUpdateCitationWithURLAnchorSaves(t *testing.T) {
	src := srcID('0')
	existing := citation(citID('V'), src)
	st := &fakeStore{tx: newFakeTx(existing).withSource(src)}

	updated := *existing
	updated.Anchor = &models.URLAnchor{URL: "https://example.org/scan/12"}

	if err := New(st).UpdateCitation(context.Background(), updated); err != nil {
		t.Fatalf("UpdateCitation: %v", err)
	}

	if len(st.tx.saved) != 1 || st.tx.saved[0].Anchor.Kind() != models.AnchorURL {
		t.Fatalf("saved = %v", st.tx.saved)
	}
}

func TestUpdateCitationWithArchiveAnchorSaves(t *testing.T) {
	src, node := srcID('0'), nodeID('0')
	existing := citation(citID('V'), src)
	st := &fakeStore{tx: newFakeTx(existing).withSource(src).withNode(node)}

	updated := *existing
	updated.Anchor = &models.ArchiveAnchor{NodeID: node, Page: 1}

	if err := New(st).UpdateCitation(context.Background(), updated); err != nil {
		t.Fatalf("UpdateCitation: %v", err)
	}

	if len(st.tx.saved) != 1 {
		t.Fatalf("saved = %v", st.tx.saved)
	}
}

func TestUpdateCitationArchiveAnchorNodeNotFound(t *testing.T) {
	src := srcID('0')
	existing := citation(citID('V'), src)
	st := &fakeStore{tx: newFakeTx(existing).withSource(src)}

	updated := *existing
	updated.Anchor = &models.ArchiveAnchor{NodeID: nodeID('0'), Page: 1}

	err := New(st).UpdateCitation(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "anchor.node_id" {
		t.Fatalf("err = %v, want ValidationError on anchor.node_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d цитат при несуществующем узле", len(st.tx.saved))
	}
}

func TestUpdateCitationArchiveAnchorWithDocumentSaves(t *testing.T) {
	src, node, doc := srcID('0'), nodeID('0'), docID('0')
	existing := citation(citID('V'), src)
	st := &fakeStore{tx: newFakeTx(existing).withSource(src).withNode(node).withDocument(doc)}

	updated := *existing
	updated.Anchor = &models.ArchiveAnchor{NodeID: node, DocumentID: doc, Page: 1}

	if err := New(st).UpdateCitation(context.Background(), updated); err != nil {
		t.Fatalf("UpdateCitation: %v", err)
	}

	if len(st.tx.saved) != 1 {
		t.Fatalf("saved = %v", st.tx.saved)
	}
}

func TestUpdateCitationArchiveAnchorDocumentNotFound(t *testing.T) {
	src, node := srcID('0'), nodeID('0')
	existing := citation(citID('V'), src)
	st := &fakeStore{tx: newFakeTx(existing).withSource(src).withNode(node)}

	updated := *existing
	updated.Anchor = &models.ArchiveAnchor{NodeID: node, DocumentID: docID('9'), Page: 1}

	err := New(st).UpdateCitation(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "anchor.document_id" {
		t.Fatalf("err = %v, want ValidationError on anchor.document_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d цитат при несуществующем документе", len(st.tx.saved))
	}
}

func TestUpdateCitationWithFileAnchorSaves(t *testing.T) {
	src, att := srcID('0'), attID('0')
	existing := citation(citID('V'), src)
	st := &fakeStore{tx: newFakeTx(existing).withSource(src).withAttachment(att)}

	updated := *existing
	updated.Anchor = &models.FileAnchor{AttachmentID: att}

	if err := New(st).UpdateCitation(context.Background(), updated); err != nil {
		t.Fatalf("UpdateCitation: %v", err)
	}

	if len(st.tx.saved) != 1 {
		t.Fatalf("saved = %v", st.tx.saved)
	}
}

func TestUpdateCitationFileAnchorAttachmentNotFound(t *testing.T) {
	src := srcID('0')
	existing := citation(citID('V'), src)
	st := &fakeStore{tx: newFakeTx(existing).withSource(src)}

	updated := *existing
	updated.Anchor = &models.FileAnchor{AttachmentID: attID('0')}

	err := New(st).UpdateCitation(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "anchor.attachment_id" {
		t.Fatalf("err = %v, want ValidationError on anchor.attachment_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d цитат при несуществующем вложении", len(st.tx.saved))
	}
}
```

#### `internal/usecases/delete_citation/deps.go` (создать)
`internal/usecases/delete_citation/deps.go`:
```go
package delete_citation

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// CitationRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type CitationRepo interface {
	DeleteCitation(ctx context.Context, id models.ID) error
}
```

#### `internal/usecases/delete_citation/scenario.go` (создать)
`internal/usecases/delete_citation/scenario.go`:
```go
package delete_citation

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление цитаты».
type Scenario struct {
	citations CitationRepo
}

// New создаёт сценарий.
func New(citations CitationRepo) *Scenario {
	return &Scenario{citations: citations}
}

// DeleteCitation удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие (SourceLink.CitationID
// у любой сущности с доказательствами) — *models.InUseError со списком
// ссылающихся.
func (s *Scenario) DeleteCitation(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.citations.DeleteCitation(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeCitation)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeCitation,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

#### `internal/usecases/delete_citation/scenario_test.go` (создать)
`internal/usecases/delete_citation/scenario_test.go`:
```go
package delete_citation

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

func (f *fakeRepo) DeleteCitation(_ context.Context, id models.ID) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)

	return nil
}

func TestDeleteCitationCallsRepo(t *testing.T) {
	repo := &fakeRepo{}
	id := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	if err := New(repo).DeleteCitation(context.Background(), id); err != nil {
		t.Fatalf("DeleteCitation: %v", err)
	}

	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("deleted = %v", repo.deleted)
	}
}

func TestDeleteCitationInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	err := New(repo).DeleteCitation(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}

	if len(repo.deleted) != 0 {
		t.Fatalf("repo вызван при невалидном id")
	}
}

func TestDeleteCitationPropagatesInUseError(t *testing.T) {
	wantErr := &models.InUseError{Type: models.TypeCitation, ID: "C-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	repo := &fakeRepo{err: wantErr}

	err := New(repo).DeleteCitation(context.Background(), "C-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	var iu *models.InUseError
	if !errors.As(err, &iu) {
		t.Fatalf("err = %v, want InUseError", err)
	}
}
```

#### `internal/httpapi/citation.go` (создать)
`internal/httpapi/citation.go`:
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleCitationList — GET /api/citations?limit=&offset=. Чтение открыто
// анонимному посетителю.
func handleCitationList(citations CitationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := citations.ListCitations(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.CitationsFromModels(list))
	}
}

// handleCitationSearch — GET /api/citations/search?q=&limit=&offset=.
func handleCitationSearch(citations CitationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := citations.SearchCitations(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.CitationsFromModels(list))
	}
}

// handleCitationGet — GET /api/citations/{id}.
func handleCitationGet(citations CitationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := citations.GetCitation(r.Context(), AccessFromContext(r.Context()), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.CitationFromModel(c))
	}
}
```

#### `internal/httpapi/citation_write.go` (создать)
`internal/httpapi/citation_write.go`:
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleCitationCreate — POST /api/citations: создаёт запись, отвечает 201 с
// созданной записью (id генерирует сценарий). Несуществующий source_id или
// ссылка внутри anchor — 422. Запись — только для вошедшего владельца, см.
// handleDivisionCreate.
func handleCitationCreate(citations CitationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.CitationCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := citations.CreateCitation(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.CitationFromModel(created))
	}
}

// handleCitationUpdate — PUT /api/citations/{id}: полная замена
// source_id/anchor/text/note/private. Читает текущую версию, накладывает
// поля запроса (fetch-then-merge).
func handleCitationUpdate(citations CitationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.CitationUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := citations.GetCitation(r.Context(), AccessFromContext(r.Context()), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.SourceID = m.SourceID
		cur.Anchor = m.Anchor
		cur.Text = m.Text
		cur.Note = m.Note
		cur.Private = m.Private

		if err := citations.UpdateCitation(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.CitationFromModel(cur))
	}
}

// handleCitationDelete — DELETE /api/citations/{id}: 204 без тела; занятая
// запись (есть SourceLink на неё) — 409 со списком ссылающихся. Запись —
// только для вошедшего владельца, см. handleDivisionCreate.
func handleCitationDelete(citations CitationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := citations.DeleteCitation(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
```

#### `internal/httpapi/citation_test.go` (создать)
`internal/httpapi/citation_test.go`:
```go
package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeCitations struct {
	list []models.Citation
	err  error
	page models.Page

	getC      models.Citation
	gotIDs    []models.ID
	created   models.Citation
	gotCreate models.Citation
	updated   models.Citation
	deleteErr error

	search    []models.Citation
	gotSearch models.SearchQuery
}

func (f *fakeCitations) ListCitations(_ context.Context, _ models.Access, page models.Page) ([]models.Citation, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeCitations) SearchCitations(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Citation, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeCitations) GetCitation(_ context.Context, _ models.Access, id models.ID) (models.Citation, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Citation{}, f.err
	}

	return f.getC, nil
}

func (f *fakeCitations) CreateCitation(_ context.Context, c models.Citation) (models.Citation, error) {
	f.gotCreate = c
	if f.err != nil {
		return models.Citation{}, f.err
	}

	return f.created, nil
}

func (f *fakeCitations) UpdateCitation(_ context.Context, c models.Citation) error {
	f.updated = c

	return f.err
}

func (f *fakeCitations) DeleteCitation(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

// TestCitationListReturnsRecords — запись без anchor: тело проще для точного
// сравнения; полиморфная привязка проверяется отдельно в create/update-тестах.
func TestCitationListReturnsRecords(t *testing.T) {
	svc := &fakeCitations{list: []models.Citation{{ID: "C-1", SourceID: "S-1", Text: "запись №5"}}}

	rec := get(t, NewHandler(Deps{Citations: svc, DocsFS: fstest.MapFS{}}), "/api/citations")
	requireStatus(t, rec, 200)

	want := `[{"id":"C-1","source_id":"S-1","text":"запись №5","private":false}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestCitationGetNotFound(t *testing.T) {
	svc := &fakeCitations{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Citations: svc, DocsFS: fstest.MapFS{}}), "/api/citations/C-1")
	requireStatus(t, rec, 404)
}

func TestCitationSearchPassesQuery(t *testing.T) {
	svc := &fakeCitations{}

	rec := get(t, NewHandler(Deps{Citations: svc, DocsFS: fstest.MapFS{}}), "/api/citations/search?q=запись")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "запись" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
```

#### `internal/httpapi/citation_write_test.go` (создать)
`internal/httpapi/citation_write_test.go`:
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

// TestCitationCreateContract — anchor кодируется как вложенный объект с
// дискриминатором kind; используем вариант url — самый простой, без ссылки
// на другую сущность.
func TestCitationCreateContract(t *testing.T) {
	svc := &fakeCitations{created: models.Citation{ID: "C-1", SourceID: "S-1"}}

	rec := postD(t, NewHandler(Deps{Citations: svc, DocsFS: fstest.MapFS{}}), "/api/citations",
		`{"source_id":"S-1","anchor":{"kind":"url","url":"https://example.org"},"text":"запись №5"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.SourceID != "S-1" || svc.gotCreate.Text != "запись №5" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}

	anchor, ok := svc.gotCreate.Anchor.(*models.URLAnchor)
	if !ok || anchor.URL != "https://example.org" {
		t.Fatalf("gotCreate.Anchor = %+v", svc.gotCreate.Anchor)
	}
}

// TestCitationCreateSourceNotFoundIs422 — usecase-слой возвращает
// *models.ValidationError по полю source_id при несуществующем источнике
// (см. create_citation), writeError мапит это на 422, по образцу
// create_archive/create_source.
func TestCitationCreateSourceNotFoundIs422(t *testing.T) {
	svc := &fakeCitations{err: &models.ValidationError{Entity: models.TypeCitation, Field: "source_id", Reason: "источник не найден"}}

	rec := postD(t, NewHandler(Deps{Citations: svc, DocsFS: fstest.MapFS{}}), "/api/citations",
		`{"source_id":"S-999","text":"запись №5"}`)

	requireStatus(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"source_id"`) {
		t.Fatalf("body = %s, ожидалось поле source_id", rec.Body)
	}
}

func TestCitationCreateAnonymousIs401(t *testing.T) {
	svc := &fakeCitations{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Citations: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/citations", strings.NewReader(`{"source_id":"S-1","text":"запись №5"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

// TestCitationUpdateMergesFields — проверяет все поля, включая anchor.
func TestCitationUpdateMergesFields(t *testing.T) {
	svc := &fakeCitations{getC: models.Citation{ID: "C-1", SourceID: "S-1"}}

	rec := putD(t, NewHandler(Deps{Citations: svc, DocsFS: fstest.MapFS{}}), "/api/citations/C-1",
		`{"source_id":"S-2","anchor":{"kind":"url","url":"https://example.org"},`+
			`"text":"запись №5 (испр.)","note":"проверить","private":true}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.SourceID != "S-2" || svc.updated.Text != "запись №5 (испр.)" ||
		svc.updated.Note != "проверить" || svc.updated.Private != true || svc.updated.ID != "C-1" {
		t.Fatalf("updated = %+v", svc.updated)
	}

	anchor, ok := svc.updated.Anchor.(*models.URLAnchor)
	if !ok || anchor.URL != "https://example.org" {
		t.Fatalf("updated.Anchor = %+v", svc.updated.Anchor)
	}
}

func TestCitationDeleteNoContent(t *testing.T) {
	svc := &fakeCitations{}

	rec := delD(t, NewHandler(Deps{Citations: svc, DocsFS: fstest.MapFS{}}), "/api/citations/C-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestCitationDeleteInUseIs409(t *testing.T) {
	svc := &fakeCitations{deleteErr: &models.InUseError{Type: models.TypeCitation, ID: "C-1"}}

	rec := delD(t, NewHandler(Deps{Citations: svc, DocsFS: fstest.MapFS{}}), "/api/citations/C-1")

	requireStatus(t, rec, http.StatusConflict)
}
```

#### `internal/mcp/citation.go` (создать)
`internal/mcp/citation.go`:
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

// registerCitationTools регистрирует тулы для работы с цитатами. source_id —
// обязательная строгая ссылка на источник (просто id). anchor — необязательная
// полиморфная привязка «где именно» (объект, см. anchorObjectProperties) —
// пустой объект или отсутствие аргумента означает «без привязки».
func registerCitationTools(s *server.MCPServer, citations CitationService) {
	tool := mcp.NewTool(
		"citation_list",
		mcp.WithDescription("Список цитат в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, citationListHandler(citations))

	tool = mcp.NewTool(
		"citation_search",
		mcp.WithDescription("Поиск цитат по началу текста; результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало текста цитаты")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, citationSearchHandler(citations))

	tool = mcp.NewTool(
		"citation_get",
		mcp.WithDescription("Цитата по id; результат — JSON записи. Неверный формат id, отсутствующая или приватная (без полного доступа) запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например C-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, citationGetHandler(citations))

	tool = mcp.NewTool(
		"citation_create",
		mcp.WithDescription("Создать цитату; id генерируется сервером; результат — JSON созданной записи. Несуществующий source_id или ссылка внутри anchor — ошибка тула"),
		mcp.WithString("source_id", mcp.Required(), mcp.Description("id источника")),
		mcp.WithObject("anchor", mcp.Description("Привязка «где именно» (необязательно)"), mcp.Properties(anchorObjectProperties())),
		mcp.WithString("text", mcp.Description("Текст выписки")),
		mcp.WithString("note", mcp.Description("Заметка")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, citationCreateHandler(citations))

	tool = mcp.NewTool(
		"citation_update",
		mcp.WithDescription("Изменить цитату: полная замена source_id/anchor/text/note/private; результат — JSON обновлённой записи. Несуществующий source_id или ссылка внутри anchor — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("source_id", mcp.Required(), mcp.Description("id источника")),
		mcp.WithObject("anchor", mcp.Description("Привязка «где именно» (необязательно)"), mcp.Properties(anchorObjectProperties())),
		mcp.WithString("text", mcp.Description("Текст выписки")),
		mcp.WithString("note", mcp.Description("Заметка")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, citationUpdateHandler(citations))

	tool = mcp.NewTool(
		"citation_delete",
		mcp.WithDescription("Удалить цитату. Необратимо. Если на неё есть строгие ссылки (SourceLink у любой сущности с доказательствами) — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, citationDeleteHandler(citations))
}

func citationListHandler(citations CitationService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := citations.ListCitations(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.CitationsFromModels(list))
	}
}

func citationSearchHandler(citations CitationService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := citations.SearchCitations(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.CitationsFromModels(list))
	}
}

func citationGetHandler(citations CitationService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		c, err := citations.GetCitation(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.CitationFromModel(c))
	}
}

func citationCreateHandler(citations CitationService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		anchor, err := optionalAnchor(req.GetArguments(), "anchor")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		c := models.Citation{
			SourceID: models.ID(req.GetString("source_id", "")),
			Anchor:   anchor,
			Text:     req.GetString("text", ""),
			Note:     req.GetString("note", ""),
			Private:  req.GetBool("private", false),
		}

		created, err := citations.CreateCitation(ctx, c)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.CitationFromModel(created))
	}
}

func citationUpdateHandler(citations CitationService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := citations.GetCitation(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		anchor, err := optionalAnchor(req.GetArguments(), "anchor")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cur.SourceID = models.ID(req.GetString("source_id", ""))
		cur.Anchor = anchor
		cur.Text = req.GetString("text", "")
		cur.Note = req.GetString("note", "")
		cur.Private = req.GetBool("private", false)

		if err := citations.UpdateCitation(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.CitationFromModel(cur))
	}
}

func citationDeleteHandler(citations CitationService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := citations.DeleteCitation(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
```

#### `internal/mcp/citation_test.go` (создать)
`internal/mcp/citation_test.go`:
```go
package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakeCitations struct {
	list []models.Citation
	err  error

	getC      models.Citation
	created   models.Citation
	gotCreate models.Citation
	updated   models.Citation
	gotIDs    []models.ID
	deleteErr error

	search []models.Citation
}

func (f *fakeCitations) ListCitations(context.Context, models.Access, models.Page) ([]models.Citation, error) {
	return f.list, f.err
}

func (f *fakeCitations) SearchCitations(context.Context, models.Access, models.SearchQuery) ([]models.Citation, error) {
	return f.search, f.err
}

func (f *fakeCitations) GetCitation(_ context.Context, _ models.Access, id models.ID) (models.Citation, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Citation{}, f.err
	}

	return f.getC, nil
}

func (f *fakeCitations) CreateCitation(_ context.Context, c models.Citation) (models.Citation, error) {
	f.gotCreate = c
	if f.err != nil {
		return models.Citation{}, f.err
	}

	return f.created, nil
}

func (f *fakeCitations) UpdateCitation(_ context.Context, c models.Citation) error {
	f.updated = c

	return f.err
}

func (f *fakeCitations) DeleteCitation(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callCitationTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestCitationGetToolContract(t *testing.T) {
	svc := &fakeCitations{getC: models.Citation{ID: "C-1", SourceID: "S-1"}}

	res := callCitationTool(t, citationGetHandler(svc), map[string]any{"id": "C-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "C-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

// TestCitationCreateToolPassesAnchor — anchor передаётся вложенным объектом
// с дискриминатором kind (см. anchorObjectProperties); вариант url — самый
// простой, без ссылки на другую сущность.
func TestCitationCreateToolPassesAnchor(t *testing.T) {
	svc := &fakeCitations{created: models.Citation{ID: "C-new", SourceID: "S-1"}}

	res := callCitationTool(t, citationCreateHandler(svc), map[string]any{
		"source_id": "S-1",
		"anchor":    map[string]any{"kind": "url", "url": "https://example.org"},
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.SourceID != "S-1" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}

	anchor, ok := svc.gotCreate.Anchor.(*models.URLAnchor)
	if !ok || anchor.URL != "https://example.org" {
		t.Fatalf("gotCreate.Anchor = %+v", svc.gotCreate.Anchor)
	}
}

// TestCitationCreateToolSourceNotFoundIsError — usecase-слой возвращает
// ошибку на несуществующий source_id, тул должен отдать её как ошибку тула
// (не паниковать, не проглатывать).
func TestCitationCreateToolSourceNotFoundIsError(t *testing.T) {
	svc := &fakeCitations{err: &models.ValidationError{Entity: models.TypeCitation, Field: "source_id", Reason: "не найдено"}}

	res := callCitationTool(t, citationCreateHandler(svc), map[string]any{
		"source_id": "S-999",
	})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestCitationDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeCitations{deleteErr: &models.InUseError{Type: models.TypeCitation, ID: "C-1"}}

	res := callCitationTool(t, citationDeleteHandler(svc), map[string]any{"id": "C-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersCitationTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Citations: &fakeCitations{}}).ListTools()

	for _, name := range []string{"citation_list", "citation_search", "citation_get", "citation_create", "citation_update", "citation_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
```

### Шаг 1.5. `internal/mcp/object_args.go` — добавить Anchor/SourceLink хелперы

Изменить существующий файл — итоговое содержимое:
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
// вроде Church.Parish/Parish.Church. Появляется впервые в этом проходе
// (первые сущности с одиночным *TextRef, не списком). ref/type round-trip'ятся
// как есть (см. transport.TextRef.Model): клиент, уже получивший ref/type
// через church_get/parish_get, сохранит ссылку, отправив их обратно
// неизменными в church_update/parish_update; если их не передать вовсе или
// изменить только text — ссылка будет потеряна или расходиться с ним.
func textRefObjectProperties() map[string]any {
	return map[string]any{
		"text": map[string]any{"type": "string", "description": "Текст (обязателен, если нет ссылки)"},
		"ref":  map[string]any{"type": "string", "description": "id сущности-ссылки; сохраняется, если передать его обратно неизменным (например, из предыдущего *_get)"},
		"type": map[string]any{"type": "string", "description": "тип сущности-ссылки; сохраняется вместе с ref при неизменной передаче"},
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

// anchorObjectProperties — JSON-schema свойств объектного аргумента вида
// Anchor (полиморфная привязка «где именно» у Citation, см.
// transport.Anchor): плоский объект с дискриминатором kind и полями всех
// трёх вариантов вместе (по образцу FactDate), а не вложенный union — проще
// для MCP-клиента, чем oneOf. Первый полиморфный тип в программе.
func anchorObjectProperties() map[string]any {
	return map[string]any{
		"kind":          map[string]any{"type": "string", "enum": []string{"archive", "file", "url"}, "description": "Вид привязки; пустой объект или отсутствие аргумента — без привязки"},
		"node_id":       map[string]any{"type": "string", "description": "id архивного узла (kind=archive, обязателен для этого вида)"},
		"document_id":   map[string]any{"type": "string", "description": "id архивного документа (kind=archive, необязательно)"},
		"page":          map[string]any{"type": "integer", "description": "Номер страницы/скана (kind=archive, обязателен, не меньше 1)"},
		"rect":          map[string]any{"type": "string", "description": "Координаты области выделения на изображении (kind=archive, необязательно)"},
		"attachment_id": map[string]any{"type": "string", "description": "id вложения (kind=file, обязателен для этого вида)"},
		"timecode":      map[string]any{"type": "string", "description": "Тайм-метка для аудио/видео (kind=file, необязательно)"},
		"url":           map[string]any{"type": "string", "description": "Абсолютный http(s)-адрес (kind=url, обязателен для этого вида)"},
	}
}

// optionalAnchor читает необязательный объектный аргумент вида Anchor (см.
// anchorObjectProperties) из сырых аргументов тула и конвертирует его в
// модель; отсутствующий, null или пустой (kind не задан/не распознан)
// аргумент — nil, без ошибки (см. (*transport.Anchor).Model()).
func optionalAnchor(args map[string]any, name string) (models.Anchor, error) {
	raw, ok := args[name]
	if !ok || raw == nil {
		return nil, nil
	}

	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	var a transport.Anchor
	if err := json.Unmarshal(b, &a); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	return a.Model(), nil
}

// sourceLinkObjectProperties — JSON-schema свойств одного элемента массива
// sources (доказательство, см. transport.SourceLink). target_type/target_id
// сюда не входят — клиент их не отправляет, владелец подставляется сервером
// из контекста вызова (см. transport.SourceLink.Model()).
func sourceLinkObjectProperties() map[string]any {
	return map[string]any{
		"citation_id": map[string]any{"type": "string", "description": "id цитаты (обязателен)"},
		"reliability": map[string]any{"type": "string", "enum": []string{"primary", "contemporary", "memory", "indirect", "unknown"}, "description": "Достоверность именно этого утверждения по этой цитате"},
		"role":        map[string]any{"type": "string", "description": "Роль утверждения"},
		"note":        map[string]any{"type": "string", "description": "Заметка"},
	}
}

// optionalSourceLinks читает массив объектов вида SourceLink (см.
// sourceLinkObjectProperties) из сырых аргументов тула и конвертирует его в
// модели; отсутствующий или null аргумент — пустой срез, без ошибки (та же
// механика, что и textRefsFromStrings для списков TextRef).
func optionalSourceLinks(args map[string]any, name string) ([]models.SourceLink, error) {
	raw, ok := args[name]
	if !ok || raw == nil {
		return nil, nil
	}

	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	var links []transport.SourceLink
	if err := json.Unmarshal(b, &links); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	return transport.SourceLinksToModel(links), nil
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

### Шаг 1.6. `Deps`-реестр: подключить Source и Citation

По образцу `Notes`/`Attachments` — гвард `if deps.X != nil { register...(...) }`. Ниже — итоговое содержимое каждого изменённого файла целиком.

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
	Sources      SourceService
	Citations    CitationService
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

	if deps.Sources != nil {
		registerSourceRoutes(mux, deps.Sources)
	}

	if deps.Citations != nil {
		registerCitationRoutes(mux, deps.Citations)
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

	if deps.Sources != nil {
		registerSourceRoutes(mux, deps.Sources)
	}

	if deps.Citations != nil {
		registerCitationRoutes(mux, deps.Citations)
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
	Sources      SourceService
	Citations    CitationService
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

	if deps.Sources != nil {
		registerSourceTools(s, deps.Sources)
	}

	if deps.Citations != nil {
		registerCitationTools(s, deps.Citations)
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
	create_citation "github.com/amarin/genodex/internal/usecases/create_citation"
	create_division "github.com/amarin/genodex/internal/usecases/create_division"
	create_estate "github.com/amarin/genodex/internal/usecases/create_estate"
	create_given_name "github.com/amarin/genodex/internal/usecases/create_given_name"
	create_note "github.com/amarin/genodex/internal/usecases/create_note"
	create_parish "github.com/amarin/genodex/internal/usecases/create_parish"
	create_patronymic "github.com/amarin/genodex/internal/usecases/create_patronymic"
	create_repository "github.com/amarin/genodex/internal/usecases/create_repository"
	create_source "github.com/amarin/genodex/internal/usecases/create_source"
	create_surname "github.com/amarin/genodex/internal/usecases/create_surname"
	create_title "github.com/amarin/genodex/internal/usecases/create_title"
	delete_archive "github.com/amarin/genodex/internal/usecases/delete_archive"
	delete_attachment "github.com/amarin/genodex/internal/usecases/delete_attachment"
	delete_church "github.com/amarin/genodex/internal/usecases/delete_church"
	delete_citation "github.com/amarin/genodex/internal/usecases/delete_citation"
	delete_division "github.com/amarin/genodex/internal/usecases/delete_division"
	delete_estate "github.com/amarin/genodex/internal/usecases/delete_estate"
	delete_given_name "github.com/amarin/genodex/internal/usecases/delete_given_name"
	delete_note "github.com/amarin/genodex/internal/usecases/delete_note"
	delete_parish "github.com/amarin/genodex/internal/usecases/delete_parish"
	delete_patronymic "github.com/amarin/genodex/internal/usecases/delete_patronymic"
	delete_repository "github.com/amarin/genodex/internal/usecases/delete_repository"
	delete_source "github.com/amarin/genodex/internal/usecases/delete_source"
	delete_surname "github.com/amarin/genodex/internal/usecases/delete_surname"
	delete_title "github.com/amarin/genodex/internal/usecases/delete_title"
	get_archive "github.com/amarin/genodex/internal/usecases/get_archive"
	get_attachment "github.com/amarin/genodex/internal/usecases/get_attachment"
	get_church "github.com/amarin/genodex/internal/usecases/get_church"
	get_citation "github.com/amarin/genodex/internal/usecases/get_citation"
	get_division "github.com/amarin/genodex/internal/usecases/get_division"
	get_estate "github.com/amarin/genodex/internal/usecases/get_estate"
	get_given_name "github.com/amarin/genodex/internal/usecases/get_given_name"
	get_note "github.com/amarin/genodex/internal/usecases/get_note"
	get_parish "github.com/amarin/genodex/internal/usecases/get_parish"
	get_patronymic "github.com/amarin/genodex/internal/usecases/get_patronymic"
	get_repository "github.com/amarin/genodex/internal/usecases/get_repository"
	get_source "github.com/amarin/genodex/internal/usecases/get_source"
	get_surname "github.com/amarin/genodex/internal/usecases/get_surname"
	get_title "github.com/amarin/genodex/internal/usecases/get_title"
	list_archives "github.com/amarin/genodex/internal/usecases/list_archives"
	list_attachments "github.com/amarin/genodex/internal/usecases/list_attachments"
	list_churches "github.com/amarin/genodex/internal/usecases/list_churches"
	list_citations "github.com/amarin/genodex/internal/usecases/list_citations"
	list_divisions "github.com/amarin/genodex/internal/usecases/list_divisions"
	list_estates "github.com/amarin/genodex/internal/usecases/list_estates"
	list_given_names "github.com/amarin/genodex/internal/usecases/list_given_names"
	list_notes "github.com/amarin/genodex/internal/usecases/list_notes"
	list_parishes "github.com/amarin/genodex/internal/usecases/list_parishes"
	list_patronymics "github.com/amarin/genodex/internal/usecases/list_patronymics"
	list_repositories "github.com/amarin/genodex/internal/usecases/list_repositories"
	list_sources "github.com/amarin/genodex/internal/usecases/list_sources"
	list_surnames "github.com/amarin/genodex/internal/usecases/list_surnames"
	list_titles "github.com/amarin/genodex/internal/usecases/list_titles"
	search_archives "github.com/amarin/genodex/internal/usecases/search_archives"
	search_attachments "github.com/amarin/genodex/internal/usecases/search_attachments"
	search_churches "github.com/amarin/genodex/internal/usecases/search_churches"
	search_citations "github.com/amarin/genodex/internal/usecases/search_citations"
	search_divisions "github.com/amarin/genodex/internal/usecases/search_divisions"
	search_estates "github.com/amarin/genodex/internal/usecases/search_estates"
	search_given_names "github.com/amarin/genodex/internal/usecases/search_given_names"
	search_notes "github.com/amarin/genodex/internal/usecases/search_notes"
	search_parishes "github.com/amarin/genodex/internal/usecases/search_parishes"
	search_patronymics "github.com/amarin/genodex/internal/usecases/search_patronymics"
	search_repositories "github.com/amarin/genodex/internal/usecases/search_repositories"
	search_sources "github.com/amarin/genodex/internal/usecases/search_sources"
	search_surnames "github.com/amarin/genodex/internal/usecases/search_surnames"
	search_titles "github.com/amarin/genodex/internal/usecases/search_titles"
	update_archive "github.com/amarin/genodex/internal/usecases/update_archive"
	update_attachment "github.com/amarin/genodex/internal/usecases/update_attachment"
	update_church "github.com/amarin/genodex/internal/usecases/update_church"
	update_citation "github.com/amarin/genodex/internal/usecases/update_citation"
	update_division "github.com/amarin/genodex/internal/usecases/update_division"
	update_estate "github.com/amarin/genodex/internal/usecases/update_estate"
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
	_ httpapi.SourceService     = (*sourceService)(nil)
	_ mcp.SourceService         = (*sourceService)(nil)
	_ httpapi.CitationService   = (*citationService)(nil)
	_ mcp.CitationService       = (*citationService)(nil)
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
			Sources:     sources,
			Citations:   citations,
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
		Sources:      sources,
		Citations:    citations,
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

### Шаг 1.7. Рубеж

`gofmt -l .` пусто; `go build ./...`, `go vet ./...`, `go test ./...` (весь репозиторий) зелёные. Живой смок через `curl` (см. «Предпосылка») — не обязателен повторно: создать `Source`, создать `Citation` без anchor и с `anchor.kind=url`, проверить 422 на несуществующий `source_id` и на `anchor.node_id`.

### Шаг 1.8. Коммит

```bash
git add \
  internal/transport/source_link.go internal/transport/anchor.go \
  internal/transport/source.go internal/transport/source_write.go \
  internal/transport/citation.go internal/transport/citation_write.go \
  internal/usecases/list_sources internal/usecases/search_sources internal/usecases/get_source internal/usecases/create_source internal/usecases/update_source internal/usecases/delete_source \
  internal/usecases/list_citations internal/usecases/search_citations internal/usecases/get_citation internal/usecases/create_citation internal/usecases/update_citation internal/usecases/delete_citation \
  internal/httpapi/source.go internal/httpapi/source_write.go internal/httpapi/source_test.go internal/httpapi/source_write_test.go \
  internal/httpapi/citation.go internal/httpapi/citation_write.go internal/httpapi/citation_test.go internal/httpapi/citation_write_test.go \
  internal/mcp/source.go internal/mcp/source_test.go internal/mcp/citation.go internal/mcp/citation_test.go internal/mcp/object_args.go \
  internal/httpapi/deps.go internal/httpapi/api.go internal/httpapi/httpapi.go internal/mcp/deps.go internal/mcp/server.go internal/app/app.go
git commit -m "feat(backend): Source/Citation — полный CRUD (usecases/httpapi/mcp) + Deps-реестр"
```

## Задача 2. Бэкенд: редактирование Sources у 6 сущностей + проверка ссылки на цитату

**Интерфейсы, потребляемые из Задачи 1**: `transport.SourceLink.Model()`/`SourceLinksToModel`, `mcp.object_args.go`:`sourceLinkObjectProperties`/`optionalSourceLinks`, `store.Store.GetCitation` (генерическое хранилище, `internal/store/deps.go:190`).
**Производит**: редактируемое поле `sources` в Create/Update DTO и MCP-аргументах `AdministrativeDivision`/`Repository`/`Church`/`Parish`/`Archive`/`Note`; видимое (было отсутствующим) поле `sources` в read-контракте `AdministrativeDivision`; проверка существования `sources[i].citation_id` во всех 12 create/update-сценариях этих 6 сущностей — потребляется Задачей 4 (веб-ретрофит).

**Файлы:**
- Изменить (транспорт, DTO): `internal/transport/{admin_division,admin_division_write,repository_write,church_write,parish_write,archive_write,note_write}.go`
- Изменить (httpapi, fetch-then-merge): `internal/httpapi/{division_write,repository_write,church_write,parish_write,archive_write,note_write}.go`
- Изменить (MCP, новый аргумент `sources`): `internal/mcp/{division,repository,church,parish,archive,note}.go`
- Изменить (usecases, проверка `sources[i].citation_id` в транзакции — `create_repository`/`create_church`/`create_parish` дополнительно переведены с плоского `Save` на транзакционный `InTx`, у них раньше не было ни одного FK для проверки): `internal/usecases/{create,update}_{division,repository,church,parish,archive,note}/scenario.go` (12 файлов) + `deps.go` для `create_repository`/`create_church`/`create_parish` (3 файла) + `scenario_test.go` для всех 12 пакетов (новый регресс-тест на несуществующую цитату в каждом)
- Изменить (test fallout — существующие тесты проверяли точную JSON-строку без поля `sources`, добавлено `"sources":[]`): `internal/httpapi/{division_test,division_write_test,store_test}.go`, `internal/transport/admin_division_test.go`, `internal/mcp/{division_test,division_write_test}.go`

**Важно для исполнителя**: код ниже даётся полностью, файл за файлом — итоговое содержимое после правки, не диффы. Для `create_repository`/`create_church`/`create_parish` итоговый файл выглядит структурно иначе, чем до правки (раньше — плоский вызов `s.store.SaveX(...)`, теперь — `s.store.InTx(ctx, func(tx store.Store) error { ... проверка sources ...; return tx.SaveX(ctx, &x) })`, по образцу `create_archive`) — переносить как итоговый файл целиком, не пытаться найти минимальный дифф. Порядок проверок внутри транзакции (там, где уже есть другая проверка — `RepositoryID` у Archive, `ParentID` у Division/Note, обход цикла у Note/Division): сначала существующая проверка, затем цикл по `sources[i].citation_id`, затем `Save`.

### Шаг 2.1. `AdministrativeDivision` — транспорт, usecases, httpapi, MCP

#### `internal/transport/admin_division.go` (изменить — итоговое содержимое)
`internal/transport/admin_division.go`:
```go
// Package transport — DTO публичных контрактов (/api и MCP-тулы) и конвертеры
// из домена. Единственное место, где определена форма JSON на проводе; домен
// (internal/models) о JSON не знает.
package transport

import "github.com/amarin/genodex/internal/models"

// AdminDivision — контракт единицы административного деления
// (GET /api/admin-divisions, MCP-тул division_list). parent_id — null у
// корня. Sources — единственное поле полной модели, кроме name/type/
// parent_id, отдаваемое в контракте (подпроект 5: Sources редактируется у
// всех сущностей, где есть, остальные поля — Items/Variants/Renames/
// Successors/Since/Until/Notes — по-прежнему вне контракта, узкий DTO с
// подпроекта 1).
type AdminDivision struct {
	ID       models.ID                `json:"id"`
	Name     string                   `json:"name"`
	Type     models.AdminDivisionType `json:"type"`
	ParentID *models.ID               `json:"parent_id"`
	Sources  []SourceLink             `json:"sources"`
}

// AdminDivisionFromModel конвертирует единицу деления в контракт.
func AdminDivisionFromModel(d models.AdministrativeDivision) AdminDivision {
	var parent *models.ID

	if d.ParentID != nil {
		p := *d.ParentID // копия: контракт не делит указатель с моделью
		parent = &p
	}

	return AdminDivision{ID: d.ID, Name: d.Name, Type: d.Type, ParentID: parent, Sources: SourceLinksFromModel(d.Sources)}
}

// AdminDivisionsFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func AdminDivisionsFromModels(ds []models.AdministrativeDivision) []AdminDivision {
	out := make([]AdminDivision, 0, len(ds))
	for _, d := range ds {
		out = append(out, AdminDivisionFromModel(d))
	}

	return out
}
```

#### `internal/transport/admin_division_write.go` (изменить — итоговое содержимое)
`internal/transport/admin_division_write.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// AdminDivisionCreate — тело POST /api/admin-divisions и аргументы тула
// division_create. Идентификатор генерирует сценарий; parent_id = null у корня.
type AdminDivisionCreate struct {
	Name     string                   `json:"name"`
	Type     models.AdminDivisionType `json:"type"`
	ParentID *models.ID               `json:"parent_id"`
	Sources  []SourceLink             `json:"sources"`
}

// Model возвращает доменную единицу с пустым ID.
func (d AdminDivisionCreate) Model() models.AdministrativeDivision {
	return models.AdministrativeDivision{
		Name:     d.Name,
		Type:     d.Type,
		ParentID: cloneParentID(d.ParentID),
		Sources:  SourceLinksToModel(d.Sources),
	}
}

// AdminDivisionUpdate — тело PUT /api/admin-divisions/{id} и аргументы тула
// division_update: полная замена полей name/type/parent_id.
type AdminDivisionUpdate struct {
	Name     string                   `json:"name"`
	Type     models.AdminDivisionType `json:"type"`
	ParentID *models.ID               `json:"parent_id"`
	Sources  []SourceLink             `json:"sources"`
}

func (d AdminDivisionUpdate) Model() models.AdministrativeDivision {
	return models.AdministrativeDivision{
		Name:     d.Name,
		Type:     d.Type,
		ParentID: cloneParentID(d.ParentID),
		Sources:  SourceLinksToModel(d.Sources),
	}
}

func cloneParentID(p *models.ID) *models.ID {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
```

#### `internal/usecases/create_division/scenario.go` (изменить — итоговое содержимое)
`internal/usecases/create_division/scenario.go`:
```go
package create_division

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание единицы административного деления».
type Scenario struct {
	store DivisionStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st DivisionStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateDivision создаёт единицу деления: генерирует идентификатор, проверяет
// инварианты, в одной транзакции убеждается в существовании родителя и
// сохраняет. Возвращает созданную единицу с заполненным ID.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта), невалидная сущность и несуществующий родитель —
// *models.ValidationError (поля id, соответствующее и parent_id); прочее —
// ошибки хранилища как есть.
func (s *Scenario) CreateDivision(ctx context.Context, d models.AdministrativeDivision) (models.AdministrativeDivision, error) {
	if d.ID != "" {
		return models.AdministrativeDivision{}, &models.ValidationError{
			Entity: models.TypeAdministrativeDivision,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", d.ID),
		}
	}

	d.ID = s.ids.New(models.TypeAdministrativeDivision)

	if err := d.Validate(); err != nil {
		return models.AdministrativeDivision{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		if d.ParentID != nil {
			if _, err := tx.GetAdministrativeDivision(ctx, *d.ParentID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return parentErr("родитель %q не найден", *d.ParentID)
				}

				return err
			}
		}

		for i, link := range d.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveAdministrativeDivision(ctx, &d)
	})
	if err != nil {
		return models.AdministrativeDivision{}, err
	}

	return d, nil
}

// parentErr — *models.ValidationError по полю parent_id.
func parentErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeAdministrativeDivision,
		Field:  "parent_id",
		Reason: fmt.Sprintf(format, args...),
	}
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeAdministrativeDivision,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/create_division/scenario_test.go` (изменить — итоговое содержимое)
`internal/usecases/create_division/scenario_test.go`:
```go
package create_division

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// adID возвращает корректный идентификатор деления, отличающийся последним символом.
func adID(last byte) models.ID {
	return models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// cID возвращает корректный идентификатор цитаты.
func cID(last byte) models.ID {
	return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// stubIDs — генератор с заранее известным результатом; запоминает запрошенный тип.
type stubIDs struct {
	id      models.ID
	gotType models.Type
}

func (s *stubIDs) New(t models.Type) models.ID {
	s.gotType = t

	return s.id
}

// fakeTx реализует нужные сценарию методы store.Store поверх карты делений;
// остальные методы порта паникуют через nil-встраивание — сценарий не должен
// их звать.
type fakeTx struct {
	store.Store
	divisions map[models.ID]*models.AdministrativeDivision
	citations map[models.ID]*models.Citation
	saved     []*models.AdministrativeDivision
	getErr    error
	saveErr   error
}

func newFakeTx(existing ...*models.AdministrativeDivision) *fakeTx {
	tx := &fakeTx{divisions: map[models.ID]*models.AdministrativeDivision{}, citations: map[models.ID]*models.Citation{}}
	for _, d := range existing {
		tx.divisions[d.ID] = d
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

func (f *fakeTx) GetAdministrativeDivision(_ context.Context, id models.ID) (*models.AdministrativeDivision, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

	d, ok := f.divisions[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *d

	return &cp, nil
}

func (f *fakeTx) SaveAdministrativeDivision(_ context.Context, d *models.AdministrativeDivision) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *d
	f.divisions[d.ID] = &cp
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует DivisionStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx      *fakeTx
	inTxErr error // ошибка самого InTx (например, отменённый контекст)
	calls   int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	if f.inTxErr != nil {
		return f.inTxErr
	}

	return fn(f.tx)
}

func validInput() models.AdministrativeDivision {
	return models.AdministrativeDivision{Name: "Село", Type: models.AdminDivisionSelo}
}

func TestCreateDivisionGeneratesIDAndSaves(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	ids := &stubIDs{id: adID('V')}

	got, err := New(st, ids).CreateDivision(context.Background(), validInput())
	if err != nil {
		t.Fatalf("CreateDivision: %v", err)
	}

	if got.ID != adID('V') || got.Name != "Село" {
		t.Fatalf("got %+v, ожидалась единица с ID %v", got, adID('V'))
	}

	if ids.gotType != models.TypeAdministrativeDivision {
		t.Errorf("генератор вызван с типом %q, ожидался administrative_division", ids.gotType)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].ID != adID('V') {
		t.Fatalf("InTx=%d, saved=%v; ожидалось одно сохранение внутри одной транзакции", st.calls, st.tx.saved)
	}
}

// TestCreateDivisionRejectsExplicitID: явный входной ID допустим только для
// импорта — сценарий отвергает его до генерации и обращения к хранилищу.
func TestCreateDivisionRejectsExplicitID(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	ids := &stubIDs{id: adID('V')}

	in := validInput()
	in.ID = adID('0')

	_, err := New(st, ids).CreateDivision(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю id", err)
	}

	if st.calls != 0 || ids.gotType != "" {
		t.Fatalf("InTx=%d, генератор вызван с %q; ожидалось: не вызваны", st.calls, ids.gotType)
	}
}

// TestCreateDivisionValidatesBeforeTx: невалидная сущность (пустое название) —
// *ValidationError, транзакция не открывается.
func TestCreateDivisionValidatesBeforeTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	in := validInput()
	in.Name = ""

	_, err := New(st, &stubIDs{id: adID('V')}).CreateDivision(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю name", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestCreateDivisionWithParentSaves(t *testing.T) {
	parent := &models.AdministrativeDivision{ID: adID('0'), Name: "Волость", Type: models.AdminDivisionVolost}
	st := &fakeStore{tx: newFakeTx(parent)}

	in := validInput()
	pid := parent.ID
	in.ParentID = &pid

	got, err := New(st, &stubIDs{id: adID('V')}).CreateDivision(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateDivision: %v", err)
	}

	if got.ParentID == nil || *got.ParentID != parent.ID || len(st.tx.saved) != 1 {
		t.Fatalf("got %+v, saved=%d; ожидалось сохранение с родителем", got, len(st.tx.saved))
	}
}

// TestCreateDivisionParentNotFound: несуществующий родитель — *ValidationError
// по полю parent_id (в S15 — 422), ничего не сохраняется.
func TestCreateDivisionParentNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	in := validInput()
	pid := adID('0')
	in.ParentID = &pid

	_, err := New(st, &stubIDs{id: adID('V')}).CreateDivision(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d единиц при несуществующем родителе", len(st.tx.saved))
	}
}

func TestCreateDivisionPropagatesSaveError(t *testing.T) {
	wantErr := errors.New("save failed")
	st := &fakeStore{tx: newFakeTx()}
	st.tx.saveErr = wantErr

	if _, err := New(st, &stubIDs{id: adID('V')}).CreateDivision(context.Background(), validInput()); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

func TestCreateDivisionPropagatesTxError(t *testing.T) {
	wantErr := errors.New("tx failed")
	st := &fakeStore{tx: newFakeTx(), inTxErr: wantErr}

	if _, err := New(st, &stubIDs{id: adID('V')}).CreateDivision(context.Background(), validInput()); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

// TestCreateDivisionPropagatesParentGetError: сбой чтения родителя (не
// ErrNotFound) — ошибка хранилища как есть, не *ValidationError.
func TestCreateDivisionPropagatesParentGetError(t *testing.T) {
	wantErr := errors.New("get failed")
	st := &fakeStore{tx: newFakeTx()}
	st.tx.getErr = wantErr

	in := validInput()
	pid := adID('0')
	in.ParentID = &pid

	_, err := New(st, &stubIDs{id: adID('V')}).CreateDivision(context.Background(), in)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}

	var ve *models.ValidationError
	if errors.As(err, &ve) {
		t.Fatalf("сбой хранилища превращён в *ValidationError: %v", err)
	}
}

// TestCreateDivisionSourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestCreateDivisionSourceCitationNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	in := validInput()
	in.Sources = []models.SourceLink{{CitationID: cID('0')}}

	_, err := New(st, &stubIDs{id: adID('V')}).CreateDivision(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d единиц при несуществующей цитате", len(st.tx.saved))
	}
}
```

#### `internal/usecases/update_division/scenario.go` (изменить — итоговое содержимое)
`internal/usecases/update_division/scenario.go`:
```go
package update_division

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение единицы административного деления».
type Scenario struct {
	store DivisionStore
}

// New создаёт сценарий.
func New(st DivisionStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateDivision полностью заменяет единицу деления по d.ID: проверяет
// инварианты, в одной транзакции убеждается, что единица существует, а цепочка
// родителей не проходит через неё саму (цикл), и сохраняет.
//
// Ошибки: невалидная сущность, несуществующий родитель и цикл по parent_id —
// *models.ValidationError (соответствующее поле и parent_id); нет такой
// единицы — models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateDivision(ctx context.Context, d models.AdministrativeDivision) error {
	if err := d.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetAdministrativeDivision(ctx, d.ID); err != nil {
			return err
		}

		if err := checkParentChain(ctx, tx, &d); err != nil {
			return err
		}

		for i, link := range d.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveAdministrativeDivision(ctx, &d)
	})
}

// checkParentChain обходит цепочку родителей d вверх: каждый предок должен
// существовать, и цепочка не должна проходить через саму единицу или
// замыкаться (защита от испорченных данных). Ошибка цепочки —
// *models.ValidationError по полю parent_id.
func checkParentChain(ctx context.Context, tx store.Store, d *models.AdministrativeDivision) error {
	seen := map[models.ID]bool{d.ID: true}

	for cur := d.ParentID; cur != nil; {
		if seen[*cur] {
			return parentErr("цепочка родителей %q проходит через саму единицу или замыкается на %q", d.ID, *cur)
		}
		seen[*cur] = true

		p, err := tx.GetAdministrativeDivision(ctx, *cur)
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
		Entity: models.TypeAdministrativeDivision,
		Field:  "parent_id",
		Reason: fmt.Sprintf(format, args...),
	}
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeAdministrativeDivision,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/update_division/scenario_test.go` (изменить — итоговое содержимое)
`internal/usecases/update_division/scenario_test.go`:
```go
package update_division

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// adID возвращает корректный идентификатор деления, отличающийся последним символом.
func adID(last byte) models.ID {
	return models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// cID возвращает корректный идентификатор цитаты.
func cID(last byte) models.ID {
	return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// fakeTx реализует нужные сценарию методы store.Store поверх карты делений;
// остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	divisions map[models.ID]*models.AdministrativeDivision
	citations map[models.ID]*models.Citation
	saved     []*models.AdministrativeDivision
	saveErr   error
	gets      int
}

func newFakeTx(existing ...*models.AdministrativeDivision) *fakeTx {
	tx := &fakeTx{divisions: map[models.ID]*models.AdministrativeDivision{}, citations: map[models.ID]*models.Citation{}}
	for _, d := range existing {
		tx.divisions[d.ID] = d
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

func (f *fakeTx) GetAdministrativeDivision(_ context.Context, id models.ID) (*models.AdministrativeDivision, error) {
	f.gets++

	d, ok := f.divisions[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *d

	return &cp, nil
}

func (f *fakeTx) SaveAdministrativeDivision(_ context.Context, d *models.AdministrativeDivision) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *d
	f.divisions[d.ID] = &cp
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует DivisionStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func division(id models.ID, parent *models.ID) *models.AdministrativeDivision {
	return &models.AdministrativeDivision{ID: id, Name: "Единица " + string(id), Type: models.AdminDivisionSelo, ParentID: parent}
}

func TestUpdateDivisionSaves(t *testing.T) {
	existing := division(adID('V'), nil)
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Name = "Новое название"

	if err := New(st).UpdateDivision(context.Background(), updated); err != nil {
		t.Fatalf("UpdateDivision: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Name != "Новое название" {
		t.Fatalf("InTx=%d, saved=%v; ожидалось одно сохранение с новым названием", st.calls, st.tx.saved)
	}
}

func TestUpdateDivisionNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateDivision(context.Background(), *division(adID('V'), nil))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d единиц при несуществующей единице", len(st.tx.saved))
	}
}

// TestUpdateDivisionValidatesBeforeTx: невалидная сущность — *ValidationError,
// транзакция не открывается.
func TestUpdateDivisionValidatesBeforeTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	bad := *division(adID('V'), nil)
	bad.Name = ""

	err := New(st).UpdateDivision(context.Background(), bad)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю name", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

// TestUpdateDivisionParentNotFound: новый родитель не существует —
// *ValidationError по полю parent_id, ничего не сохраняется.
func TestUpdateDivisionParentNotFound(t *testing.T) {
	existing := division(adID('V'), nil)
	st := &fakeStore{tx: newFakeTx(existing)}

	missing := adID('0')
	updated := *existing
	updated.ParentID = &missing

	err := New(st).UpdateDivision(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d единиц при несуществующем родителе", len(st.tx.saved))
	}
}

// TestUpdateDivisionDirectCycle: B — дочка A; попытка сделать A дочкой B — цикл,
// *ValidationError по полю parent_id.
func TestUpdateDivisionDirectCycle(t *testing.T) {
	idA, idB := adID('A'), adID('B')
	a := division(idA, nil)
	b := division(idB, &idA)
	st := &fakeStore{tx: newFakeTx(a, b)}

	updated := *a
	updated.ParentID = &idB

	err := New(st).UpdateDivision(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id (цикл)", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d единиц при цикле", len(st.tx.saved))
	}
}

// TestUpdateDivisionLongCycle: цепочка C → B → A; попытка сделать A дочкой C —
// цикл длиной три, *ValidationError по полю parent_id.
func TestUpdateDivisionLongCycle(t *testing.T) {
	idA, idB, idC := adID('A'), adID('B'), adID('C')
	a := division(idA, nil)
	b := division(idB, &idA)
	c := division(idC, &idB)
	st := &fakeStore{tx: newFakeTx(a, b, c)}

	updated := *a
	updated.ParentID = &idC

	err := New(st).UpdateDivision(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id (цикл)", err)
	}
}

// TestUpdateDivisionReparentOK: перенос под другого корректного родителя проходит.
func TestUpdateDivisionReparentOK(t *testing.T) {
	idA, idB, idC := adID('A'), adID('B'), adID('C')
	a := division(idA, nil)
	b := division(idB, &idA)
	c := division(idC, nil)
	st := &fakeStore{tx: newFakeTx(a, b, c)}

	updated := *b
	updated.ParentID = &idC

	if err := New(st).UpdateDivision(context.Background(), updated); err != nil {
		t.Fatalf("UpdateDivision: %v", err)
	}

	if len(st.tx.saved) != 1 || *st.tx.saved[0].ParentID != idC {
		t.Fatalf("saved=%v; ожидалось сохранение с родителем %v", st.tx.saved, idC)
	}
}

// TestUpdateDivisionWithoutParentSkipsWalk: без родителя цепочка не обходится —
// одно чтение (существование самой единицы).
func TestUpdateDivisionWithoutParentSkipsWalk(t *testing.T) {
	existing := division(adID('V'), nil)
	st := &fakeStore{tx: newFakeTx(existing)}

	if err := New(st).UpdateDivision(context.Background(), *existing); err != nil {
		t.Fatalf("UpdateDivision: %v", err)
	}

	if st.tx.gets != 1 {
		t.Fatalf("чтений %d, ожидалось одно (без обхода родителей)", st.tx.gets)
	}
}

func TestUpdateDivisionPropagatesSaveError(t *testing.T) {
	wantErr := errors.New("save failed")
	existing := division(adID('V'), nil)
	st := &fakeStore{tx: newFakeTx(existing)}
	st.tx.saveErr = wantErr

	if err := New(st).UpdateDivision(context.Background(), *existing); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

// TestUpdateDivisionSourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestUpdateDivisionSourceCitationNotFound(t *testing.T) {
	existing := division(adID('V'), nil)
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Sources = []models.SourceLink{{CitationID: cID('0')}}

	err := New(st).UpdateDivision(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d единиц при несуществующей цитате", len(st.tx.saved))
	}
}
```

#### `internal/httpapi/division_write.go` (изменить — итоговое содержимое)
`internal/httpapi/division_write.go`:
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
		cur.Sources = transport.SourceLinksToModel(in.Sources)

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

#### `internal/mcp/division.go` (изменить — итоговое содержимое)
`internal/mcp/division.go`:
```go
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// registerDivisionTools регистрирует тулы для работы с единицами административного деления.
func registerDivisionTools(s *server.MCPServer, divisions DivisionService) {
	tool := mcp.NewTool(
		"division_list",
		mcp.WithDescription("Список единиц административного деления (губернии, уезды, волости, населённые "+
			"пункты) в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithString("kind", mcp.Description("Вид: settlement — только населённые пункты; пусто — без фильтра")),
		mcp.WithString("type", mcp.Description("Точный тип единицы: governorate, district, volost, gorod, selo, "+
			"derevnya, hutor, pogost, stanitsa, mestechko, other; пусто — без фильтра")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
		mcp.WithString("parent_id", mcp.Description("id родительской единицы; пусто — корень (весь список)")),
	)

	s.AddTool(tool, divisionListHandler(divisions))

	tool = mcp.NewTool(
		"division_search",
		mcp.WithDescription("Поиск единиц административного деления по началу названия (включая варианты "+
			"названий); результат — JSON-массив единиц. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало названия или варианта")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, divisionSearchHandler(divisions))

	tool = mcp.NewTool(
		"division_get",
		mcp.WithDescription("Единица административного деления по id; результат — JSON единицы. Неверный формат id или отсутствующая единица — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id единицы, например AD-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, divisionGetHandler(divisions))

	tool = mcp.NewTool(
		"division_create",
		mcp.WithDescription("Создать единицу административного деления; id генерируется сервером; результат — JSON созданной единицы. Неверные name/type или несуществующий parent_id — ошибка тула"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Название единицы")),
		mcp.WithString("type", mcp.Required(), mcp.Description("governorate, district, volost, gorod, selo, derevnya, hutor, pogost, stanitsa, mestechko, other")),
		mcp.WithString("parent_id", mcp.Description("id родительской единицы; пусто — корень")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
	)
	s.AddTool(tool, divisionCreateHandler(divisions))

	tool = mcp.NewTool(
		"division_update",
		mcp.WithDescription("Изменить единицу административного деления: обновляются name, type и parent_id (пустой parent_id — корень); прочие поля текущей версии сохраняются; результат — JSON обновлённой единицы"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id единицы")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Новое название")),
		mcp.WithString("type", mcp.Required(), mcp.Description("governorate, district, volost, gorod, selo, derevnya, hutor, pogost, stanitsa, mestechko, other")),
		mcp.WithString("parent_id", mcp.Description("id нового родителя; пусто — корень")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
	)
	s.AddTool(tool, divisionUpdateHandler(divisions))

	tool = mcp.NewTool(
		"division_delete",
		mcp.WithDescription("Удалить единицу административного деления, если она не занята другими единицами; занятая — ошибка тула. Необратимо"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id единицы")),
	)
	s.AddTool(tool, divisionDeleteHandler(divisions))
}

// divisionListHandler возвращает обработчик тула division_list: разбирает
// аргументы, обращается к сценарию и возвращает JSON-массив контракта
// transport.AdminDivision. Ошибки тула сообщаются внутри CallToolResult через
// NewToolResultError.
func divisionListHandler(divisions DivisionService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q, err := divisionQueryFromRequest(req)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := divisions.ListDivisions(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		data, err := json.Marshal(transport.AdminDivisionsFromModels(list))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сериализовать список: %v", err)), nil
		}

		return mcp.NewToolResultText(string(data)), nil
	}
}

// divisionQueryFromRequest собирает запрос из аргументов тула; отсутствующий
// аргумент — нулевое значение, неверный тип числа — ошибка.
func divisionQueryFromRequest(req mcp.CallToolRequest) (models.DivisionQuery, error) {
	q := models.DivisionQuery{
		Kind:     models.DivisionKind(req.GetString("kind", "")),
		Type:     models.AdminDivisionType(req.GetString("type", "")),
		ParentID: optionalParentID(req),
	}

	var err error

	if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
		return q, err
	}

	if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
		return q, err
	}

	return q, nil
}

// divisionSearchHandler — тул division_search: ищет единицы по началу названия
// (включая варианты); результат — JSON-массив transport.AdminDivision.
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

		list, err := divisions.SearchDivisions(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.AdminDivisionsFromModels(list))
	}
}

// optionalInt читает необязательный целочисленный аргумент.
func optionalInt(req mcp.CallToolRequest, name string) (int, error) {
	raw, ok := req.GetArguments()[name]
	if !ok || raw == nil { // нет аргумента или явный null — значение по умолчанию
		return 0, nil
	}

	if f, isFloat := raw.(float64); isFloat && f != math.Trunc(f) {
		return 0, fmt.Errorf("аргумент %s: ожидалось целое число", name)
	}

	n, err := req.RequireInt(name)
	if err != nil {
		return 0, fmt.Errorf("аргумент %s: ожидалось целое число", name)
	}

	return n, nil
}

// divisionGetHandler — тул division_get: читает единицу по id. Валидация
// формата id и проверка существования — в сценарии; его ошибки — ошибки тула.
func divisionGetHandler(divisions DivisionService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		d, err := divisions.GetDivision(ctx, models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить единицу: %v", err)), nil
		}

		return toolJSONResult(transport.AdminDivisionFromModel(d))
	}
}

// divisionCreateHandler — тул division_create: собирает модель из аргументов
// (пустой parent_id — корень) и отдаёт созданную единицу.
func divisionCreateHandler(divisions DivisionService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sources, err := optionalSourceLinks(req.GetArguments(), "sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		row := transport.AdminDivisionCreate{
			Name:     req.GetString("name", ""),
			Type:     models.AdminDivisionType(req.GetString("type", "")),
			ParentID: optionalParentID(req),
		}

		m := row.Model()
		m.Sources = sources

		created, err := divisions.CreateDivision(ctx, m)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать единицу: %v", err)), nil
		}

		return toolJSONResult(transport.AdminDivisionFromModel(created))
	}
}

// divisionUpdateHandler — тул division_update: меняет name/type/parent_id,
// остальные поля берутся из актуальной версии сценария.
func divisionUpdateHandler(divisions DivisionService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := divisions.GetDivision(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		sources, err := optionalSourceLinks(req.GetArguments(), "sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		row := transport.AdminDivisionUpdate{
			Name:     req.GetString("name", ""),
			Type:     models.AdminDivisionType(req.GetString("type", "")),
			ParentID: optionalParentID(req),
		}
		cur.Name = row.Name
		cur.Type = row.Type
		cur.ParentID = row.ParentID
		cur.Sources = sources

		if err := divisions.UpdateDivision(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.AdminDivisionFromModel(cur))
	}
}

// divisionDeleteHandler — тул division_delete: удаляет единицу; занятая —
// ошибка тула с текстом ошибки сценария.
func divisionDeleteHandler(divisions DivisionService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := divisions.DeleteDivision(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить единицу: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("деление %q удалено", id)), nil
	}
}

// toolJSONResult сериализует значение в JSON-текст результата тула.
func toolJSONResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("не удалось сериализовать: %v", err)), nil
	}

	return mcp.NewToolResultText(string(data)), nil
}

// optionalParentID читает необязательный parent_id; пустая строка (в т.ч. явный
// null) — корень (nil-указатель).
func optionalParentID(req mcp.CallToolRequest) *models.ID {
	raw := req.GetString("parent_id", "")
	if raw == "" {
		return nil
	}

	id := models.ID(raw)

	return &id
}
```

### Шаг 2.2. `Repository` — транспорт, usecases, httpapi, MCP

#### `internal/transport/repository_write.go` (изменить — итоговое содержимое)
`internal/transport/repository_write.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// RepositoryCreate — тело POST /api/repositories и аргументы тула
// repository_create. Идентификатор генерирует сценарий.
type RepositoryCreate struct {
	Name    string       `json:"name"`
	Type    string       `json:"type"`
	Address string       `json:"address,omitempty"`
	URLs    []TextRef    `json:"urls"`
	Notes   []TextRef    `json:"notes"`
	Sources []SourceLink `json:"sources"`
	Private bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (r RepositoryCreate) Model() models.Repository {
	return models.Repository{
		Name:    r.Name,
		Type:    models.RepositoryType(r.Type),
		Address: r.Address,
		URLs:    TextRefsToModel(r.URLs),
		Notes:   TextRefsToModel(r.Notes),
		Sources: SourceLinksToModel(r.Sources),
		Private: r.Private,
	}
}

// RepositoryUpdate — тело PUT /api/repositories/{id} и аргументы тула
// repository_update: полная замена name/type/address/urls/notes/sources/private.
type RepositoryUpdate struct {
	Name    string       `json:"name"`
	Type    string       `json:"type"`
	Address string       `json:"address,omitempty"`
	URLs    []TextRef    `json:"urls"`
	Notes   []TextRef    `json:"notes"`
	Sources []SourceLink `json:"sources"`
	Private bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (r RepositoryUpdate) Model() models.Repository {
	return models.Repository{
		Name:    r.Name,
		Type:    models.RepositoryType(r.Type),
		Address: r.Address,
		URLs:    TextRefsToModel(r.URLs),
		Notes:   TextRefsToModel(r.Notes),
		Sources: SourceLinksToModel(r.Sources),
		Private: r.Private,
	}
}
```

#### `internal/usecases/create_repository/deps.go` (изменить — итоговое содержимое)
`internal/usecases/create_repository/deps.go`:
```go
package create_repository

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// RepositoryStore — зависимость сценария: транзакция порта store.Store.
// Проверка ссылок (Sources) и сохранение идут в одной транзакции на
// переданном fn хранилище.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type RepositoryStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
```

#### `internal/usecases/create_repository/scenario.go` (изменить — итоговое содержимое)
`internal/usecases/create_repository/scenario.go`:
```go
package create_repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание записи хранилища».
type Scenario struct {
	store RepositoryStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st RepositoryStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateRepository создаёт запись: генерирует идентификатор, проверяет
// инварианты, в одной транзакции убеждается в существовании цитат из Sources
// и сохраняет. Возвращает созданную запись с заполненным ID.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта), невалидная сущность и несуществующая цитата —
// *models.ValidationError; прочее — ошибки хранилища как есть.
func (s *Scenario) CreateRepository(ctx context.Context, r models.Repository) (models.Repository, error) {
	if r.ID != "" {
		return models.Repository{}, &models.ValidationError{
			Entity: models.TypeRepository,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", r.ID),
		}
	}

	r.ID = s.ids.New(models.TypeRepository)

	if err := r.Validate(); err != nil {
		return models.Repository{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		for i, link := range r.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveRepository(ctx, &r)
	})
	if err != nil {
		return models.Repository{}, err
	}

	return r, nil
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeRepository,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/create_repository/scenario_test.go` (изменить — итоговое содержимое)
`internal/usecases/create_repository/scenario_test.go`:
```go
package create_repository

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
	saved     *models.Repository
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

func (f *fakeTx) SaveRepository(_ context.Context, s *models.Repository) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *s
	f.saved = &cp

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

func TestCreateRepositoryGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{tx: newFakeTx()}
	sc := New(st, ids)

	got, err := sc.CreateRepository(context.Background(), models.Repository{Name: "ГАВО", Type: models.RepositoryTypeArchive})
	if err != nil {
		t.Fatalf("CreateRepository: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.tx.saved == nil || st.tx.saved.Name != "ГАВО" {
		t.Fatalf("saved = %+v", st.tx.saved)
	}
}

func TestCreateRepositoryRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx()}, &stubIDs{})

	_, err := sc.CreateRepository(context.Background(), models.Repository{ID: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "x"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestCreateRepositoryRejectsEmptyName(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx()}, &stubIDs{id: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

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
	sc := New(&fakeStore{tx: newFakeTx()}, &stubIDs{id: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateRepository(context.Background(), models.Repository{Name: "ГАВО", Type: "Not Valid"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "type" {
		t.Fatalf("err = %v, want ValidationError on type", err)
	}
}

func TestCreateRepositoryPropagatesStoreError(t *testing.T) {
	wantErr := errors.New("db down")
	st := &fakeStore{tx: newFakeTx()}
	st.tx.saveErr = wantErr
	sc := New(st, &stubIDs{id: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateRepository(context.Background(), models.Repository{Name: "ГАВО", Type: models.RepositoryTypeArchive})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

// TestCreateRepositorySourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestCreateRepositorySourceCitationNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	sc := New(st, &stubIDs{id: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	in := models.Repository{Name: "ГАВО", Type: models.RepositoryTypeArchive, Sources: []models.SourceLink{{CitationID: cID('0')}}}

	_, err := sc.CreateRepository(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if st.tx.saved != nil {
		t.Fatalf("saved = %+v; ожидалось: ничего не сохранено", st.tx.saved)
	}
}
```

#### `internal/usecases/update_repository/scenario.go` (изменить — итоговое содержимое)
`internal/usecases/update_repository/scenario.go`:
```go
package update_repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение записи хранилища».
type Scenario struct {
	store RepositoryStore
}

// New создаёт сценарий.
func New(st RepositoryStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateRepository полностью заменяет запись по r.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, и сохраняет.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateRepository(ctx context.Context, r models.Repository) error {
	if err := r.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetRepository(ctx, r.ID); err != nil {
			return err
		}

		for i, link := range r.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveRepository(ctx, &r)
	})
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeRepository,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/update_repository/scenario_test.go` (изменить — итоговое содержимое)
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
	citations    map[models.ID]*models.Citation
	saved        []*models.Repository
}

func newFakeTx(existing ...*models.Repository) *fakeTx {
	tx := &fakeTx{repositories: map[models.ID]*models.Repository{}, citations: map[models.ID]*models.Citation{}}
	for _, s := range existing {
		tx.repositories[s.ID] = s
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

// TestUpdateRepositorySourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestUpdateRepositorySourceCitationNotFound(t *testing.T) {
	existing := repository("R-01ARZ3NDEKTSV4RRFFQ69G5FA1", "ГАВО")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Sources = []models.SourceLink{{CitationID: cID('0')}}

	err := New(st).UpdateRepository(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d записей при несуществующей цитате", len(st.tx.saved))
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

#### `internal/httpapi/repository_write.go` (изменить — итоговое содержимое)
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
// name/type/address/urls/notes/sources/private. Читает текущую версию,
// накладывает поля запроса (fetch-then-merge, docs/data-model/entity-write.md §3).
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

		cur, err := repositories.GetRepository(r.Context(), AccessFromContext(r.Context()), id)
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
		cur.Sources = m.Sources
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

#### `internal/mcp/repository.go` (изменить — итоговое содержимое)
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
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
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
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
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
		r, err := repositories.GetRepository(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.RepositoryFromModel(r))
	}
}

func repositoryCreateHandler(repositories RepositoryService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sources, err := optionalSourceLinks(req.GetArguments(), "sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		r := models.Repository{
			Name:    req.GetString("name", ""),
			Type:    models.RepositoryType(req.GetString("type", "")),
			Address: req.GetString("address", ""),
			URLs:    textRefsFromStrings(req.GetStringSlice("urls", nil)),
			Notes:   textRefsFromStrings(req.GetStringSlice("notes", nil)),
			Sources: sources,
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

		cur, err := repositories.GetRepository(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		sources, err := optionalSourceLinks(req.GetArguments(), "sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cur.Name = req.GetString("name", "")
		cur.Type = models.RepositoryType(req.GetString("type", ""))
		cur.Address = req.GetString("address", "")
		cur.URLs = textRefsFromStrings(req.GetStringSlice("urls", nil))
		cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))
		cur.Sources = sources
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

### Шаг 2.3. `Church` — транспорт, usecases, httpapi, MCP

#### `internal/transport/church_write.go` (изменить — итоговое содержимое)
`internal/transport/church_write.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// ChurchCreate — тело POST /api/churches и аргументы тула church_create.
// Идентификатор генерирует сценарий. Parish — {text, ref?, type?}: сама DTO
// round-trip'ит ref/type как есть (TextRef.Model), веб-форма v1 (web/src/
// ChurchForm.tsx) редактирует только text и не выставляет ref/type сама
// (docs/data-model/entity-write.md §4) — ограничение интерфейса, а не
// контракта.
type ChurchCreate struct {
	Name        string       `json:"name"`
	Parish      *TextRef     `json:"parish,omitempty"`
	Settlements []TextRef    `json:"settlements"`
	Variants    []string     `json:"variants"`
	Notes       []TextRef    `json:"notes"`
	Sources     []SourceLink `json:"sources"`
}

// Model возвращает доменную запись с пустым ID.
func (c ChurchCreate) Model() models.Church {
	return models.Church{
		Name:        c.Name,
		Parish:      c.Parish.ModelPtr(),
		Settlements: TextRefsToModel(c.Settlements),
		Variants:    c.Variants,
		Notes:       TextRefsToModel(c.Notes),
		Sources:     SourceLinksToModel(c.Sources),
	}
}

// ChurchUpdate — тело PUT /api/churches/{id} и аргументы тула church_update:
// полная замена name/parish/settlements/variants/notes/sources.
type ChurchUpdate struct {
	Name        string       `json:"name"`
	Parish      *TextRef     `json:"parish,omitempty"`
	Settlements []TextRef    `json:"settlements"`
	Variants    []string     `json:"variants"`
	Notes       []TextRef    `json:"notes"`
	Sources     []SourceLink `json:"sources"`
}

// Model возвращает доменную запись с пустым ID.
func (c ChurchUpdate) Model() models.Church {
	return models.Church{
		Name:        c.Name,
		Parish:      c.Parish.ModelPtr(),
		Settlements: TextRefsToModel(c.Settlements),
		Variants:    c.Variants,
		Notes:       TextRefsToModel(c.Notes),
		Sources:     SourceLinksToModel(c.Sources),
	}
}
```

#### `internal/usecases/create_church/deps.go` (изменить — итоговое содержимое)
`internal/usecases/create_church/deps.go`:
```go
package create_church

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// ChurchStore — зависимость сценария: транзакция порта store.Store. Проверка
// ссылок (Sources) и сохранение идут в одной транзакции на переданном fn
// хранилище.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ChurchStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
```

#### `internal/usecases/create_church/scenario.go` (изменить — итоговое содержимое)
`internal/usecases/create_church/scenario.go`:
```go
package create_church

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание записи церкви».
type Scenario struct {
	store ChurchStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st ChurchStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateChurch создаёт запись: генерирует идентификатор, проверяет
// инварианты, в одной транзакции убеждается в существовании цитат из Sources
// и сохраняет. Возвращает созданную запись с заполненным ID.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта), невалидная сущность и несуществующая цитата —
// *models.ValidationError; прочее — ошибки хранилища как есть.
func (s *Scenario) CreateChurch(ctx context.Context, c models.Church) (models.Church, error) {
	if c.ID != "" {
		return models.Church{}, &models.ValidationError{
			Entity: models.TypeChurch,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", c.ID),
		}
	}

	c.ID = s.ids.New(models.TypeChurch)

	if err := c.Validate(); err != nil {
		return models.Church{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		for i, link := range c.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveChurch(ctx, &c)
	})
	if err != nil {
		return models.Church{}, err
	}

	return c, nil
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeChurch,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/create_church/scenario_test.go` (изменить — итоговое содержимое)
`internal/usecases/create_church/scenario_test.go`:
```go
package create_church

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
	saved     *models.Church
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

func (f *fakeTx) SaveChurch(_ context.Context, s *models.Church) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *s
	f.saved = &cp

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

func TestCreateChurchGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "CH-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{tx: newFakeTx()}
	sc := New(st, ids)

	got, err := sc.CreateChurch(context.Background(), models.Church{Name: "Никольская церковь"})
	if err != nil {
		t.Fatalf("CreateChurch: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.tx.saved == nil || st.tx.saved.Name != "Никольская церковь" {
		t.Fatalf("saved = %+v", st.tx.saved)
	}
}

func TestCreateChurchRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx()}, &stubIDs{})

	_, err := sc.CreateChurch(context.Background(), models.Church{ID: "CH-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "x"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestCreateChurchRejectsEmptyName(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx()}, &stubIDs{id: "CH-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateChurch(context.Background(), models.Church{})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, want ValidationError on name", err)
	}
}

func TestCreateChurchPropagatesStoreError(t *testing.T) {
	wantErr := errors.New("db down")
	st := &fakeStore{tx: newFakeTx()}
	st.tx.saveErr = wantErr
	sc := New(st, &stubIDs{id: "CH-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateChurch(context.Background(), models.Church{Name: "Никольская церковь"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

// TestCreateChurchSourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestCreateChurchSourceCitationNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	sc := New(st, &stubIDs{id: "CH-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	in := models.Church{Name: "Никольская церковь", Sources: []models.SourceLink{{CitationID: cID('0')}}}

	_, err := sc.CreateChurch(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if st.tx.saved != nil {
		t.Fatalf("saved = %+v; ожидалось: ничего не сохранено", st.tx.saved)
	}
}
```

#### `internal/usecases/update_church/scenario.go` (изменить — итоговое содержимое)
`internal/usecases/update_church/scenario.go`:
```go
package update_church

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение записи церкви».
type Scenario struct {
	store ChurchStore
}

// New создаёт сценарий.
func New(st ChurchStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateChurch полностью заменяет запись по c.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, и сохраняет.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateChurch(ctx context.Context, c models.Church) error {
	if err := c.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetChurch(ctx, c.ID); err != nil {
			return err
		}

		for i, link := range c.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveChurch(ctx, &c)
	})
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeChurch,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/update_church/scenario_test.go` (изменить — итоговое содержимое)
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
	churches  map[models.ID]*models.Church
	citations map[models.ID]*models.Citation
	saved     []*models.Church
}

func newFakeTx(existing ...*models.Church) *fakeTx {
	tx := &fakeTx{churches: map[models.ID]*models.Church{}, citations: map[models.ID]*models.Citation{}}
	for _, s := range existing {
		tx.churches[s.ID] = s
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

// TestUpdateChurchSourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestUpdateChurchSourceCitationNotFound(t *testing.T) {
	existing := church("CH-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Никольская церковь")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Sources = []models.SourceLink{{CitationID: cID('0')}}

	err := New(st).UpdateChurch(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d записей при несуществующей цитате", len(st.tx.saved))
	}
}
```

#### `internal/httpapi/church_write.go` (изменить — итоговое содержимое)
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
// name/parish/settlements/variants/notes/sources. Читает текущую версию,
// накладывает поля запроса (fetch-then-merge).
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
		cur.Sources = m.Sources

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

#### `internal/mcp/church.go` (изменить — итоговое содержимое)
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
// {text, ref?, type?}); ref/type round-trip'ятся как есть (transport.TextRef.
// Model) — клиент, отправляющий обратно ref/type, полученные через
// church_get/list/search, не потеряет ссылку. settlements/notes — списки
// текста (v1, только строки, без ref/type). variants — простые строки.
// ВАЖНО: church_update заменяет parish/settlements/notes целиком — у
// settlements/notes нет ref/type в MCP-контракте вовсе, так что элемент с
// такой ссылкой (заданной иначе, не через MCP) будет потерян при любом
// обновлении через MCP, пока не появится picker; parish эту ссылку сохраняет,
// если её передать обратно неизменной.
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
		mcp.WithObject("parish", mcp.Description("Приход (текст или ссылка {text, ref?, type?}; ref/type сохраняются, если переданы)"), mcp.Properties(textRefObjectProperties())),
		mcp.WithArray("settlements", mcp.WithStringItems(), mcp.Description("Населённые пункты (текстом)")),
		mcp.WithArray("variants", mcp.WithStringItems(), mcp.Description("Варианты названия")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
	)
	s.AddTool(tool, churchCreateHandler(churches))

	tool = mcp.NewTool(
		"church_update",
		mcp.WithDescription("Изменить церковь: полная замена name/parish/settlements/variants/notes; результат — JSON обновлённой записи. parish — {text, ref?, type?}: передайте обратно ref/type, полученные из church_get, чтобы сохранить ссылку; settlements/notes принимают только текст (без ref/type в MCP-контракте) — ссылка на элементе (если задана иначе) будет потеряна при любом обновлении через MCP, пока не появится picker (docs/data-model/entity-write.md §5)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Новое название")),
		mcp.WithObject("parish", mcp.Description("Приход (текст или ссылка {text, ref?, type?}; ref/type сохраняются, если переданы)"), mcp.Properties(textRefObjectProperties())),
		mcp.WithArray("settlements", mcp.WithStringItems(), mcp.Description("Населённые пункты")),
		mcp.WithArray("variants", mcp.WithStringItems(), mcp.Description("Варианты названия")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
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

		sources, err := optionalSourceLinks(req.GetArguments(), "sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		c := models.Church{
			Name:        req.GetString("name", ""),
			Parish:      parish,
			Settlements: textRefsFromStrings(req.GetStringSlice("settlements", nil)),
			Variants:    req.GetStringSlice("variants", nil),
			Notes:       textRefsFromStrings(req.GetStringSlice("notes", nil)),
			Sources:     sources,
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

		sources, err := optionalSourceLinks(req.GetArguments(), "sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cur.Name = req.GetString("name", "")
		cur.Parish = parish
		cur.Settlements = textRefsFromStrings(req.GetStringSlice("settlements", nil))
		cur.Variants = req.GetStringSlice("variants", nil)
		cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))
		cur.Sources = sources

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

### Шаг 2.4. `Parish` — транспорт, usecases, httpapi, MCP

#### `internal/transport/parish_write.go` (изменить — итоговое содержимое)
`internal/transport/parish_write.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// ParishCreate — тело POST /api/parishes и аргументы тула parish_create.
// Идентификатор генерирует сценарий. Church редактируется только текстом (v1).
type ParishCreate struct {
	Name        string       `json:"name"`
	Church      *TextRef     `json:"church,omitempty"`
	Settlements []TextRef    `json:"settlements"`
	Since       *FactDate    `json:"since,omitempty"`
	Until       *FactDate    `json:"until,omitempty"`
	Notes       []TextRef    `json:"notes"`
	Sources     []SourceLink `json:"sources"`
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
		Sources:     SourceLinksToModel(p.Sources),
	}
}

// ParishUpdate — тело PUT /api/parishes/{id} и аргументы тула parish_update:
// полная замена name/church/settlements/since/until/notes/sources.
type ParishUpdate struct {
	Name        string       `json:"name"`
	Church      *TextRef     `json:"church,omitempty"`
	Settlements []TextRef    `json:"settlements"`
	Since       *FactDate    `json:"since,omitempty"`
	Until       *FactDate    `json:"until,omitempty"`
	Notes       []TextRef    `json:"notes"`
	Sources     []SourceLink `json:"sources"`
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
		Sources:     SourceLinksToModel(p.Sources),
	}
}
```

#### `internal/usecases/create_parish/deps.go` (изменить — итоговое содержимое)
`internal/usecases/create_parish/deps.go`:
```go
package create_parish

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// ParishStore — зависимость сценария: транзакция порта store.Store. Проверка
// ссылок (Sources) и сохранение идут в одной транзакции на переданном fn
// хранилище.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ParishStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
```

#### `internal/usecases/create_parish/scenario.go` (изменить — итоговое содержимое)
`internal/usecases/create_parish/scenario.go`:
```go
package create_parish

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание записи прихода».
type Scenario struct {
	store ParishStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st ParishStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateParish создаёт запись: генерирует идентификатор, проверяет
// инварианты, в одной транзакции убеждается в существовании цитат из Sources
// и сохраняет. Возвращает созданную запись с заполненным ID.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта), невалидная сущность и несуществующая цитата —
// *models.ValidationError; прочее — ошибки хранилища как есть.
func (s *Scenario) CreateParish(ctx context.Context, p models.Parish) (models.Parish, error) {
	if p.ID != "" {
		return models.Parish{}, &models.ValidationError{
			Entity: models.TypeParish,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", p.ID),
		}
	}

	p.ID = s.ids.New(models.TypeParish)

	if err := p.Validate(); err != nil {
		return models.Parish{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		for i, link := range p.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveParish(ctx, &p)
	})
	if err != nil {
		return models.Parish{}, err
	}

	return p, nil
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeParish,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/create_parish/scenario_test.go` (изменить — итоговое содержимое)
`internal/usecases/create_parish/scenario_test.go`:
```go
package create_parish

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
	saved     *models.Parish
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

func (f *fakeTx) SaveParish(_ context.Context, s *models.Parish) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *s
	f.saved = &cp

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

func TestCreateParishGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "PR-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{tx: newFakeTx()}
	sc := New(st, ids)

	got, err := sc.CreateParish(context.Background(), models.Parish{Name: "Никольский приход"})
	if err != nil {
		t.Fatalf("CreateParish: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.tx.saved == nil || st.tx.saved.Name != "Никольский приход" {
		t.Fatalf("saved = %+v", st.tx.saved)
	}
}

func TestCreateParishRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx()}, &stubIDs{})

	_, err := sc.CreateParish(context.Background(), models.Parish{ID: "PR-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "x"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestCreateParishRejectsEmptyName(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx()}, &stubIDs{id: "PR-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateParish(context.Background(), models.Parish{})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, want ValidationError on name", err)
	}
}

func TestCreateParishPropagatesStoreError(t *testing.T) {
	wantErr := errors.New("db down")
	st := &fakeStore{tx: newFakeTx()}
	st.tx.saveErr = wantErr
	sc := New(st, &stubIDs{id: "PR-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateParish(context.Background(), models.Parish{Name: "Никольский приход"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

// TestCreateParishSourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestCreateParishSourceCitationNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	sc := New(st, &stubIDs{id: "PR-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	in := models.Parish{Name: "Никольский приход", Sources: []models.SourceLink{{CitationID: cID('0')}}}

	_, err := sc.CreateParish(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if st.tx.saved != nil {
		t.Fatalf("saved = %+v; ожидалось: ничего не сохранено", st.tx.saved)
	}
}
```

#### `internal/usecases/update_parish/scenario.go` (изменить — итоговое содержимое)
`internal/usecases/update_parish/scenario.go`:
```go
package update_parish

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение записи прихода».
type Scenario struct {
	store ParishStore
}

// New создаёт сценарий.
func New(st ParishStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateParish полностью заменяет запись по p.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, и сохраняет.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateParish(ctx context.Context, p models.Parish) error {
	if err := p.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetParish(ctx, p.ID); err != nil {
			return err
		}

		for i, link := range p.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveParish(ctx, &p)
	})
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeParish,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/update_parish/scenario_test.go` (изменить — итоговое содержимое)
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
	parishes  map[models.ID]*models.Parish
	citations map[models.ID]*models.Citation
	saved     []*models.Parish
}

func newFakeTx(existing ...*models.Parish) *fakeTx {
	tx := &fakeTx{parishes: map[models.ID]*models.Parish{}, citations: map[models.ID]*models.Citation{}}
	for _, s := range existing {
		tx.parishes[s.ID] = s
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

// TestUpdateParishSourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestUpdateParishSourceCitationNotFound(t *testing.T) {
	existing := parish("PR-01ARZ3NDEKTSV4RRFFQ69G5FA1", "Никольский приход")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Sources = []models.SourceLink{{CitationID: cID('0')}}

	err := New(st).UpdateParish(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d записей при несуществующей цитате", len(st.tx.saved))
	}
}
```

#### `internal/httpapi/parish_write.go` (изменить — итоговое содержимое)
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
// name/church/settlements/since/until/notes/sources. Читает текущую версию,
// накладывает поля запроса (fetch-then-merge).
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
		cur.Sources = m.Sources

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

#### `internal/mcp/parish.go` (изменить — итоговое содержимое)
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
// одиночная необязательная ссылка (текст или ссылка на церковь, объект
// {text, ref?, type?}); ref/type round-trip'ятся как есть (transport.TextRef.
// Model) — клиент, отправляющий обратно ref/type, полученные через
// parish_get/list/search, не потеряет ссылку. since/until — структурированная
// дата (объект, см. factDateObjectProperties). ВАЖНО: parish_update заменяет
// church/settlements/notes целиком — у settlements/notes нет ref/type в
// MCP-контракте вовсе (только текст), так что такая ссылка (заданная иначе)
// будет потеряна при любом обновлении через MCP, пока не появится picker;
// church эту ссылку сохраняет, если её передать обратно неизменной.
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
		mcp.WithObject("church", mcp.Description("Церковь (текст или ссылка {text, ref?, type?}; ref/type сохраняются, если переданы)"), mcp.Properties(textRefObjectProperties())),
		mcp.WithArray("settlements", mcp.WithStringItems(), mcp.Description("Населённые пункты (текстом)")),
		mcp.WithObject("since", mcp.Description("Начало периода действия прихода"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("until", mcp.Description("Конец периода действия прихода"), mcp.Properties(factDateObjectProperties())),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
	)
	s.AddTool(tool, parishCreateHandler(parishes))

	tool = mcp.NewTool(
		"parish_update",
		mcp.WithDescription("Изменить приход: полная замена name/church/settlements/since/until/notes; результат — JSON обновлённой записи. church — {text, ref?, type?}: передайте обратно ref/type, полученные из parish_get, чтобы сохранить ссылку; settlements/notes принимают только текст (без ref/type в MCP-контракте) — ссылка на элементе (если задана иначе) будет потеряна при любом обновлении через MCP, пока не появится picker (docs/data-model/entity-write.md §5)"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Новое название")),
		mcp.WithObject("church", mcp.Description("Церковь (текст или ссылка {text, ref?, type?}; ref/type сохраняются, если переданы)"), mcp.Properties(textRefObjectProperties())),
		mcp.WithArray("settlements", mcp.WithStringItems(), mcp.Description("Населённые пункты")),
		mcp.WithObject("since", mcp.Description("Начало периода"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("until", mcp.Description("Конец периода"), mcp.Properties(factDateObjectProperties())),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
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

		sources, err := optionalSourceLinks(args, "sources")
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
			Sources:     sources,
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

		sources, err := optionalSourceLinks(args, "sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cur.Name = req.GetString("name", "")
		cur.Church = church
		cur.Settlements = textRefsFromStrings(req.GetStringSlice("settlements", nil))
		cur.Since = since
		cur.Until = until
		cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))
		cur.Sources = sources

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

### Шаг 2.5. `Archive` — транспорт, usecases, httpapi, MCP

#### `internal/transport/archive_write.go` (изменить — итоговое содержимое)
`internal/transport/archive_write.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// ArchiveCreate — тело POST /api/archives и аргументы тула archive_create.
// Идентификатор генерирует сценарий. RepositoryID — просто id (пустая строка —
// без хранилища); сценарий проверяет существование при непустом значении.
type ArchiveCreate struct {
	Name         string       `json:"name"`
	System       *TextRef     `json:"system,omitempty"`
	RepositoryID string       `json:"repository_id,omitempty"`
	Notes        []TextRef    `json:"notes"`
	Sources      []SourceLink `json:"sources"`
	Private      bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (a ArchiveCreate) Model() models.Archive {
	return models.Archive{
		Name:         a.Name,
		System:       a.System.ModelPtr(),
		RepositoryID: models.ID(a.RepositoryID),
		Notes:        TextRefsToModel(a.Notes),
		Sources:      SourceLinksToModel(a.Sources),
		Private:      a.Private,
	}
}

// ArchiveUpdate — тело PUT /api/archives/{id} и аргументы тула archive_update:
// полная замена name/system/repository_id/notes/sources/private.
type ArchiveUpdate struct {
	Name         string       `json:"name"`
	System       *TextRef     `json:"system,omitempty"`
	RepositoryID string       `json:"repository_id,omitempty"`
	Notes        []TextRef    `json:"notes"`
	Sources      []SourceLink `json:"sources"`
	Private      bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (a ArchiveUpdate) Model() models.Archive {
	return models.Archive{
		Name:         a.Name,
		System:       a.System.ModelPtr(),
		RepositoryID: models.ID(a.RepositoryID),
		Notes:        TextRefsToModel(a.Notes),
		Sources:      SourceLinksToModel(a.Sources),
		Private:      a.Private,
	}
}
```

#### `internal/usecases/create_archive/scenario.go` (изменить — итоговое содержимое)
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

		for i, link := range a.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
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

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeArchive,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/create_archive/scenario_test.go` (изменить — итоговое содержимое)
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

// cID возвращает корректный идентификатор цитаты.
func cID(last byte) models.ID {
	return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
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
	citations    map[models.ID]*models.Citation
	saved        []*models.Archive
	getErr       error
	saveErr      error
}

func newFakeTx(existingRepos ...*models.Repository) *fakeTx {
	tx := &fakeTx{repositories: map[models.ID]*models.Repository{}, citations: map[models.ID]*models.Citation{}}
	for _, r := range existingRepos {
		tx.repositories[r.ID] = r
	}

	return tx
}

func (f *fakeTx) withCitation(c *models.Citation) *fakeTx {
	f.citations[c.ID] = c

	return f
}

func (f *fakeTx) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *c

	return &cp, nil
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

// TestCreateArchiveRepositoryNotFound: RepositoryID задан, но такого
// хранилища нет — *models.ValidationError по полю repository_id, ничего не
// сохраняется. Случай «RepositoryID не задан вовсе — хранилище не
// проверяется» уже покрыт TestCreateArchiveGeneratesIDAndSaves выше (пустой
// fakeTx, GetRepository не вызывается).
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

// TestCreateArchiveSourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestCreateArchiveSourceCitationNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	in := validInput()
	in.Sources = []models.SourceLink{{CitationID: cID('0')}}

	_, err := New(st, &stubIDs{id: arID('V')}).CreateArchive(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d архивов при несуществующей цитате", len(st.tx.saved))
	}
}
```

#### `internal/usecases/update_archive/scenario.go` (изменить — итоговое содержимое)
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

		for i, link := range a.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
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

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeArchive,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/update_archive/scenario_test.go` (изменить — итоговое содержимое)
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

// cID возвращает корректный идентификатор цитаты.
func cID(last byte) models.ID {
	return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// fakeTx реализует нужные сценарию методы store.Store поверх карт архивов и
// хранилищ; остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	archives     map[models.ID]*models.Archive
	repositories map[models.ID]*models.Repository
	citations    map[models.ID]*models.Citation
	saved        []*models.Archive
	repoGetErr   error
}

func newFakeTx(existing ...*models.Archive) *fakeTx {
	tx := &fakeTx{archives: map[models.ID]*models.Archive{}, repositories: map[models.ID]*models.Repository{}, citations: map[models.ID]*models.Citation{}}
	for _, a := range existing {
		tx.archives[a.ID] = a
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
	if f.repoGetErr != nil {
		return nil, f.repoGetErr
	}

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

// TestUpdateArchivePropagatesRepositoryGetError проверяет, что настоящий сбой
// хранилища (не models.ErrNotFound) при проверке repository_id пробрасывается
// как есть, а не превращается в *models.ValidationError — см.
// create_archive/scenario_test.go: TestCreateArchivePropagatesRepositoryGetError
// для того же контракта на создании.
func TestUpdateArchivePropagatesRepositoryGetError(t *testing.T) {
	wantErr := errors.New("get failed")
	existing := archive(arID('V'), "ГАВО, архив")
	st := &fakeStore{tx: newFakeTx(existing)}
	st.tx.repoGetErr = wantErr

	updated := *existing
	updated.RepositoryID = rID('0')

	err := New(st).UpdateArchive(context.Background(), updated)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}

	var ve *models.ValidationError
	if errors.As(err, &ve) {
		t.Fatalf("сбой хранилища превращён в *ValidationError: %v", err)
	}
}

// TestUpdateArchiveSourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestUpdateArchiveSourceCitationNotFound(t *testing.T) {
	existing := archive(arID('V'), "ГАВО, архив")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Sources = []models.SourceLink{{CitationID: cID('0')}}

	err := New(st).UpdateArchive(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d архивов при несуществующей цитате", len(st.tx.saved))
	}
}
```

#### `internal/httpapi/archive_write.go` (изменить — итоговое содержимое)
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
// name/system/repository_id/notes/sources/private. Читает текущую версию,
// накладывает поля запроса (fetch-then-merge).
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

		cur, err := archives.GetArchive(r.Context(), AccessFromContext(r.Context()), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Name = m.Name
		cur.System = m.System
		cur.RepositoryID = m.RepositoryID
		cur.Notes = m.Notes
		cur.Sources = m.Sources
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

#### `internal/mcp/archive.go` (изменить — итоговое содержимое)
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
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
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
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
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
		a, err := archives.GetArchive(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
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

		sources, err := optionalSourceLinks(req.GetArguments(), "sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		a := models.Archive{
			Name:         req.GetString("name", ""),
			System:       system,
			RepositoryID: models.ID(req.GetString("repository_id", "")),
			Notes:        textRefsFromStrings(req.GetStringSlice("notes", nil)),
			Sources:      sources,
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

		cur, err := archives.GetArchive(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		system, err := optionalTextRef(req.GetArguments(), "system")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sources, err := optionalSourceLinks(req.GetArguments(), "sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cur.Name = req.GetString("name", "")
		cur.System = system
		cur.RepositoryID = models.ID(req.GetString("repository_id", ""))
		cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))
		cur.Sources = sources
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

### Шаг 2.6. `Note` — транспорт, usecases, httpapi, MCP

#### `internal/transport/note_write.go` (изменить — итоговое содержимое)
`internal/transport/note_write.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// NoteCreate — тело POST /api/notes и аргументы тула note_create.
// Идентификатор генерирует сценарий. ParentID — просто id (пустая строка —
// без родителя); сценарий проверяет существование и отсутствие циклов, по
// образцу create_division.
type NoteCreate struct {
	Kind     string       `json:"kind"`
	Title    string       `json:"title,omitempty"`
	Text     string       `json:"text,omitempty"`
	ParentID string       `json:"parent_id,omitempty"`
	Sources  []SourceLink `json:"sources"`
	Private  bool         `json:"private"`
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
		Sources:  SourceLinksToModel(n.Sources),
		Private:  n.Private,
	}
}

// NoteUpdate — тело PUT /api/notes/{id} и аргументы тула note_update:
// полная замена kind/title/text/parent_id/sources/private.
type NoteUpdate struct {
	Kind     string       `json:"kind"`
	Title    string       `json:"title,omitempty"`
	Text     string       `json:"text,omitempty"`
	ParentID string       `json:"parent_id,omitempty"`
	Sources  []SourceLink `json:"sources"`
	Private  bool         `json:"private"`
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
		Sources:  SourceLinksToModel(n.Sources),
		Private:  n.Private,
	}
}
```

#### `internal/usecases/create_note/scenario.go` (изменить — итоговое содержимое)
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

		for i, link := range n.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
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

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeNote,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/create_note/scenario_test.go` (изменить — итоговое содержимое)
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

// cID возвращает корректный идентификатор цитаты.
func cID(last byte) models.ID {
	return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
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
	notes     map[models.ID]*models.Note
	citations map[models.ID]*models.Citation
	saved     []*models.Note
	getErr    error
	saveErr   error
}

func newFakeTx(existing ...*models.Note) *fakeTx {
	tx := &fakeTx{notes: map[models.ID]*models.Note{}, citations: map[models.ID]*models.Citation{}}
	for _, n := range existing {
		tx.notes[n.ID] = n
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

// TestCreateNoteSourceCitationNotFound: Sources ссылается на несуществующую
// цитату — *models.ValidationError по полю sources[0].citation_id, ничего не
// сохраняется.
func TestCreateNoteSourceCitationNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	in := validInput()
	in.Sources = []models.SourceLink{{CitationID: cID('0')}}

	_, err := New(st, &stubIDs{id: nID('V')}).CreateNote(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d заметок при несуществующей цитате", len(st.tx.saved))
	}
}
```

#### `internal/usecases/update_note/scenario.go` (изменить — итоговое содержимое)
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

		for i, link := range n.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
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

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeNote,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/update_note/scenario_test.go` (изменить — итоговое содержимое)
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

// cID возвращает корректный идентификатор цитаты.
func cID(last byte) models.ID {
	return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// fakeTx реализует нужные сценарию методы store.Store поверх карты заметок;
// остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	notes     map[models.ID]*models.Note
	citations map[models.ID]*models.Citation
	saved     []*models.Note
	saveErr   error
	gets      int
}

func newFakeTx(existing ...*models.Note) *fakeTx {
	tx := &fakeTx{notes: map[models.ID]*models.Note{}, citations: map[models.ID]*models.Citation{}}
	for _, n := range existing {
		tx.notes[n.ID] = n
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

// TestUpdateNoteSourceCitationNotFound: Sources ссылается на несуществующую
// цитату — *models.ValidationError по полю sources[0].citation_id, ничего не
// сохраняется. Проверяется, что проверка идёт после checkParentChain (родитель
// не задан, поэтому обход цепочки тривиален).
func TestUpdateNoteSourceCitationNotFound(t *testing.T) {
	existing := note(nID('V'), nil)
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Sources = []models.SourceLink{{CitationID: cID('0')}}

	err := New(st).UpdateNote(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d заметок при несуществующей цитате", len(st.tx.saved))
	}
}
```

#### `internal/httpapi/note_write.go` (изменить — итоговое содержимое)
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
// kind/title/text/parent_id/sources. Читает текущую версию, накладывает поля
// запроса (fetch-then-merge). Владелец всегда видит запись при fetch
// (requireFull даёт полный доступ).
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
		cur.Sources = m.Sources
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

#### `internal/mcp/note.go` (изменить — итоговое содержимое)
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
		mcp.WithDescription("Поиск заметок по началу заголовка; результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало заголовка")),
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
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
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
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
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
		sources, err := optionalSourceLinks(req.GetArguments(), "sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		n := models.Note{
			Kind:    models.NoteKind(req.GetString("kind", "")),
			Title:   req.GetString("title", ""),
			Text:    req.GetString("text", ""),
			Sources: sources,
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

		sources, err := optionalSourceLinks(req.GetArguments(), "sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cur.Kind = models.NoteKind(req.GetString("kind", ""))
		cur.Title = req.GetString("title", "")
		cur.Text = req.GetString("text", "")
		cur.Sources = sources
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

### Шаг 2.7. Test fallout (`"sources":[]`) и фасады для real-store тестов (`internal/httpapi/store_test.go`)

`AdministrativeDivision` получил новое поле `sources` в read-контракте (Шаг 2.1) — 6 существующих тестов в 5 файлах сравнивали точную JSON-строку ответа без этого поля. Добавить `,"sources":[]` перед закрывающей `}` каждого объекта в их ожидаемых строках (после `"parent_id":...`). `internal/httpapi/store_test.go` дополнительно получает фасады `sourceService`/`citationService` (+ конструкторы `newSourceService`/`newCitationService`, по образцу уже существующих `archiveService`/`newArchiveService`) — нужны Шагу 2.8 (real-store тесты) для сборки `httpapi.Deps` на настоящем хранилище. Ниже — итоговое содержимое.

#### `internal/httpapi/division_test.go` (изменить — итоговое содержимое)
`internal/httpapi/division_test.go`:
```go
package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

// fakeDivisions запоминает запрос и отдаёт заданный ответ.
type fakeDivisions struct {
	list []models.AdministrativeDivision
	err  error
	got  models.DivisionQuery
	call int

	getDiv    models.AdministrativeDivision
	created   models.AdministrativeDivision
	gotCreate models.AdministrativeDivision
	updated   models.AdministrativeDivision
	gotIDs    []models.ID
	deleteErr error

	search    []models.AdministrativeDivision
	searchErr error
	gotSearch models.DivisionSearchQuery

	gotListAccess   models.Access
	gotSearchAccess models.Access
}

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

func (f *fakeDivisions) GetDivision(_ context.Context, id models.ID) (models.AdministrativeDivision, error) {
	f.gotIDs = append(f.gotIDs, id)

	if f.err != nil {
		return models.AdministrativeDivision{}, f.err
	}

	return f.getDiv, nil
}

func (f *fakeDivisions) CreateDivision(_ context.Context, d models.AdministrativeDivision) (models.AdministrativeDivision, error) {
	f.gotCreate = d

	if f.err != nil {
		return models.AdministrativeDivision{}, f.err
	}

	return f.created, nil
}

func (f *fakeDivisions) UpdateDivision(_ context.Context, d models.AdministrativeDivision) error {
	f.updated = d

	return f.err
}

func (f *fakeDivisions) DeleteDivision(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func get(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))

	return rec
}

func TestDivisionListContract(t *testing.T) {
	root := models.ID("ad-root")
	svc := &fakeDivisions{list: []models.AdministrativeDivision{
		{ID: "ad-root", Name: "Московская", Type: models.AdminDivisionGovernorate},
		{ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo, ParentID: &root},
	}}

	rec := get(t, NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}), "/api/admin-divisions")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	want := `[{"id":"ad-root","name":"Московская","type":"governorate","parent_id":null,"sources":[]},` +
		`{"id":"ad-1","name":"Давыдово","type":"selo","parent_id":"ad-root","sources":[]}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}

	if !reflect.DeepEqual(svc.got, models.DivisionQuery{}) {
		t.Fatalf("запрос без параметров дошёл до сценария как %+v", svc.got)
	}
}

func TestDivisionListEmptyIsJSONArray(t *testing.T) {
	rec := get(t, NewHandler(Deps{Divisions: &fakeDivisions{}, DocsFS: fstest.MapFS{}}), "/api/admin-divisions")

	if got := strings.TrimSpace(rec.Body.String()); rec.Code != http.StatusOK || got != `[]` {
		t.Fatalf("status = %d, body = %s; ожидалось 200 и []", rec.Code, got)
	}
}

// TestDivisionListPassesParameters: параметры kind/type/limit/offset доходят до сценария.
func TestDivisionListPassesParameters(t *testing.T) {
	svc := &fakeDivisions{}

	rec := get(t, NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}), "/api/admin-divisions?kind=settlement&type=selo&limit=20&offset=40")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}

	want := models.DivisionQuery{
		Kind: models.DivisionKindSettlement, Type: models.AdminDivisionSelo,
		Page: models.Page{Limit: 20, Offset: 40},
	}
	if svc.got != want {
		t.Fatalf("запрос %+v, ожидался %+v", svc.got, want)
	}
}

// TestDivisionListBadNumberIs400: параметр limit/offset — не число.
func TestDivisionListBadNumberIs400(t *testing.T) {
	for _, target := range []string{"/api/admin-divisions?limit=abc", "/api/admin-divisions?offset=1.5"} {
		svc := &fakeDivisions{}
		rec := get(t, NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}), target)

		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"error"`) {
			t.Errorf("%s: status = %d, body = %s; ожидался 400 с error", target, rec.Code, rec.Body)
		}

		if svc.got != (models.DivisionQuery{}) {
			t.Errorf("%s: сценарий вызван при неверном параметре: %+v", target, svc.got)
		}
	}
}

// TestDivisionListValidationErrorIs422: неверное значение — 422 с полем.
func TestDivisionListValidationErrorIs422(t *testing.T) {
	svc := &fakeDivisions{err: &models.ValidationError{Field: "kind", Reason: "неизвестный вид"}}

	rec := get(t, NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}), "/api/admin-divisions?kind=village")

	want := `{"error":"kind: неизвестный вид","field":"kind"}`
	if got := strings.TrimSpace(rec.Body.String()); rec.Code != http.StatusUnprocessableEntity || got != want {
		t.Fatalf("status = %d, body = %s; ожидалось 422 и %s", rec.Code, got, want)
	}
}

func TestDivisionListServiceErrorIs500(t *testing.T) {
	svc := &fakeDivisions{err: errors.New("хранилище недоступно")}

	rec := get(t, NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}), "/api/admin-divisions")

	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "хранилище недоступно") {
		t.Fatalf("status = %d, body = %s; ожидался 500 с текстом ошибки", rec.Code, rec.Body)
	}
}

// TestDivisionListPassesParentID: parent_id доходит до сценария.
func TestDivisionListPassesParentID(t *testing.T) {
	svc := &fakeDivisions{}

	rec := get(t, NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}), "/api/admin-divisions?parent_id=ad-root")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}

	want := models.ID("ad-root")
	if svc.got.ParentID == nil || *svc.got.ParentID != want {
		t.Fatalf("got.ParentID = %v, ожидался %s", svc.got.ParentID, want)
	}
}

func TestDivisionSearch(t *testing.T) {
	svc := &fakeDivisions{search: []models.AdministrativeDivision{
		{ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo},
	}}

	rec := get(t, NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}), "/api/admin-divisions/search?q=давы")

	want := `[{"id":"ad-1","name":"Давыдово","type":"selo","parent_id":null,"sources":[]}]`
	if got := strings.TrimSpace(rec.Body.String()); rec.Code != http.StatusOK || got != want {
		t.Fatalf("status = %d, body = %s; ожидалось 200 и %s", rec.Code, got, want)
	}

	if svc.gotSearch.Text != "давы" {
		t.Fatalf("gotSearch.Text = %q, ожидалось %q", svc.gotSearch.Text, "давы")
	}
}

// TestDivisionSearchRouteDoesNotHitID: литеральный маршрут /search побеждает {id};
// пустой фейк отвечает 200 пустым массивом, не 422 «неверный формат id».
func TestDivisionSearchRouteDoesNotHitID(t *testing.T) {
	rec := get(t, NewHandler(Deps{Divisions: &fakeDivisions{}, DocsFS: fstest.MapFS{}}), "/api/admin-divisions/search?q=давы")

	if got := strings.TrimSpace(rec.Body.String()); rec.Code != http.StatusOK || got != `[]` {
		t.Fatalf("status = %d, body = %s; ожидалось 200 []", rec.Code, got)
	}
}

// TestDivisionSearchBadNumberIs400: limit/offset — не число.
func TestDivisionSearchBadNumberIs400(t *testing.T) {
	svc := &fakeDivisions{}

	rec := get(t, NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}), "/api/admin-divisions/search?q=давы&limit=abc")

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"error"`) {
		t.Fatalf("status = %d, body = %s; ожидался 400 с error", rec.Code, rec.Body)
	}
}

// TestDivisionSearchNegativeLimitIs422: отрицательное окно — ошибка валидации сценария.
func TestDivisionSearchNegativeLimitIs422(t *testing.T) {
	svc := &fakeDivisions{searchErr: &models.ValidationError{Field: "limit", Reason: "не может быть отрицательным"}}

	rec := get(t, NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}), "/api/admin-divisions/search?q=давы&limit=-1")

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s; ожидался 422", rec.Code, rec.Body)
	}
}

// TestDivisionSearchServiceErrorIs500: прочая ошибка сценария — 500.
func TestDivisionSearchServiceErrorIs500(t *testing.T) {
	svc := &fakeDivisions{searchErr: errors.New("хранилище недоступно")}

	rec := get(t, NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}), "/api/admin-divisions/search?q=давы")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s; ожидался 500", rec.Code, rec.Body)
	}
}

// TestOldSettlementsRouteIsGone: прежнего имени контракта нет.
func TestOldSettlementsRouteIsGone(t *testing.T) {
	rec := get(t, NewHandler(Deps{Divisions: &fakeDivisions{}, DocsFS: fstest.MapFS{}}), "/api/settlements")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /api/settlements = %d, ожидался 404", rec.Code)
	}
}

// TestDivisionListPassesAccessFromContext: Access, положенный resolveAccess в
// контекст запроса, доходит до сценария как есть (не захардкожен на
// AccessFull — auth.md §6, приёмка этапа C).
func TestDivisionListPassesAccessFromContext(t *testing.T) {
	svc := &fakeDivisions{}

	req := httptest.NewRequest(http.MethodGet, "/api/admin-divisions", nil)
	req = req.WithContext(context.WithValue(req.Context(), accessCtxKey, models.AccessPublic))

	rec := httptest.NewRecorder()
	NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec, req)

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
	NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if svc.gotSearchAccess != models.AccessPublic {
		t.Fatalf("gotSearchAccess = %v, ожидался AccessPublic", svc.gotSearchAccess)
	}
}
```

#### `internal/httpapi/division_write_test.go` (изменить — итоговое содержимое)
`internal/httpapi/division_write_test.go`:
```go
package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	authpkg "github.com/amarin/genodex/internal/auth"
	"github.com/amarin/genodex/internal/models"
)

const writeID = "AD-01ARZ3NDEKTSV4RRFFQ69G5FAV"
const writeRefID = "AD-01ARZ3NDEKTSV4RRFFQ69G5FA0"

// ownerCtx кладёт в контекст запроса Access=Full и OwnerID — как resolveAccess
// при валидной cookie-сессии. Существующие тесты записи проверяют контракт
// хендлера для аутентифицированного владельца; анонимный путь — отдельные
// тесты TestDivision*AnonymousIs401 ниже.
func ownerCtx(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), accessCtxKey, models.AccessFull)
	ctx = context.WithValue(ctx, ownerCtxKey, authpkg.ID("OW-01ARZ3NDEKTSV4RRFFQ69G5FA9"))

	return r.WithContext(ctx)
}

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

func requireStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()

	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}
}

func TestDivisionGetContract(t *testing.T) {
	svc := &fakeDivisions{getDiv: models.AdministrativeDivision{ID: writeID, Name: "Давыдово", Type: models.AdminDivisionSelo}}

	rec := get(t, NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}), "/api/admin-divisions/"+writeID)

	want := `{"id":"` + writeID + `","name":"Давыдово","type":"selo","parent_id":null,"sources":[]}`
	if got := strings.TrimSpace(rec.Body.String()); rec.Code != http.StatusOK || got != want {
		t.Fatalf("status = %d, body = %s; ожидались 200 и %s", rec.Code, got, want)
	}

	if got := svc.gotIDs[0]; got != writeID {
		t.Fatalf("id = %s", got)
	}
}

func TestDivisionGetNotFoundIs404(t *testing.T) {
	svc := &fakeDivisions{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}), "/api/admin-divisions/"+writeID)

	requireStatus(t, rec, http.StatusNotFound)
	if got, want := strings.TrimSpace(rec.Body.String()), `{"error":"не найдено"}`; got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

// TestDivisionGetInvalidIDIs422: неверный формат id в пути — 422 (не 404).
func TestDivisionGetInvalidIDIs422(t *testing.T) {
	svc := &fakeDivisions{err: &models.ValidationError{Field: "id", Reason: "неверный формат"}}

	rec := get(t, NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}), "/api/admin-divisions/ad-1")

	requireStatus(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"id"`) {
		t.Fatalf("body = %s", rec.Body)
	}
}

func TestDivisionCreateContract(t *testing.T) {
	svc := &fakeDivisions{created: models.AdministrativeDivision{ID: writeID, Name: "Давыдово", Type: models.AdminDivisionSelo}}

	rec := postD(t, NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}), "/api/admin-divisions", `{"name":"Давыдово","type":"selo"}`)

	want := `{"id":"` + writeID + `","name":"Давыдово","type":"selo","parent_id":null,"sources":[]}`
	if got := strings.TrimSpace(rec.Body.String()); rec.Code != http.StatusCreated || got != want {
		t.Fatalf("status = %d, body = %s; ожидались 201 и %s", rec.Code, got, want)
	}

	if got := svc.gotCreate; got.Name != "Давыдово" || got.Type != models.AdminDivisionSelo || got.ParentID != nil || got.ID != "" {
		t.Fatalf("создана модель %+v, ожидались name/type без id и parent_id", got)
	}
}

func TestDivisionCreateWithParentPassesModel(t *testing.T) {
	svc := &fakeDivisions{created: models.AdministrativeDivision{ID: writeID, Name: "Давыдово", Type: models.AdminDivisionSelo}}

	body := `{"name":"Давыдово","type":"selo","parent_id":"` + writeRefID + `"}`
	rec := postD(t, NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}), "/api/admin-divisions", body)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.ParentID == nil || *svc.gotCreate.ParentID != writeRefID {
		t.Fatalf("parent_id = %v", svc.gotCreate.ParentID)
	}
}

func TestDivisionCreateBadJSONIs400(t *testing.T) {
	rec := postD(t, NewHandler(Deps{Divisions: &fakeDivisions{}, DocsFS: fstest.MapFS{}}), "/api/admin-divisions", `{`)

	requireStatus(t, rec, http.StatusBadRequest)
	if !strings.Contains(rec.Body.String(), "не удалось разобрать тело") {
		t.Fatalf("body = %s", rec.Body)
	}
}

func TestDivisionCreateValidationErrorIs422(t *testing.T) {
	svc := &fakeDivisions{err: &models.ValidationError{Field: "name", Reason: "пустое значение"}}

	rec := postD(t, NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}), "/api/admin-divisions", `{"name":"","type":"selo"}`)

	requireStatus(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"name"`) {
		t.Fatalf("body = %s", rec.Body)
	}
}

// TestDivisionUpdateMergesFields: PUT заменяет name/type/parent_id, прочие поля
// текущей модели сохраняются.
func TestDivisionUpdateMergesFields(t *testing.T) {
	parent := models.ID(writeRefID)
	svc := &fakeDivisions{getDiv: models.AdministrativeDivision{
		ID: writeID, Name: "Село", Type: models.AdminDivisionSelo,
		ParentID: &parent, Variants: []string{"Давыдова"},
	}}

	rec := putD(t, NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}), "/api/admin-divisions/"+writeID, `{"name":"Давыдово","type":"selo","parent_id":null}`)

	want := `{"id":"` + writeID + `","name":"Давыдово","type":"selo","parent_id":null,"sources":[]}`
	if got := strings.TrimSpace(rec.Body.String()); rec.Code != http.StatusOK || got != want {
		t.Fatalf("status = %d, body = %s; ожидались 200 и %s", rec.Code, got, want)
	}

	if svc.updated.Name != "Давыдово" || svc.updated.ID != writeID {
		t.Fatalf("updated = %+v", svc.updated)
	}
	if svc.updated.ParentID != nil {
		t.Fatalf("parent_id должен стать nil, got %v", svc.updated.ParentID)
	}
	if len(svc.updated.Variants) != 1 || svc.updated.Variants[0] != "Давыдова" {
		t.Fatalf("прочие поля должны сохраняться: %+v", svc.updated)
	}
}

func TestDivisionUpdateNotFoundIs404(t *testing.T) {
	svc := &fakeDivisions{err: models.ErrNotFound}

	rec := putD(t, NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}), "/api/admin-divisions/"+writeID, `{"name":"Давыдово","type":"selo"}`)

	requireStatus(t, rec, http.StatusNotFound)
}

func TestDivisionUpdateInvalidIDIs422(t *testing.T) {
	svc := &fakeDivisions{err: &models.ValidationError{Field: "id", Reason: "неверный формат"}}

	rec := putD(t, NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}), "/api/admin-divisions/ad-1", `{"name":"Давыдово","type":"selo"}`)

	requireStatus(t, rec, http.StatusUnprocessableEntity)
}

func TestDivisionDeleteNoContent(t *testing.T) {
	svc := &fakeDivisions{}

	rec := delD(t, NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}), "/api/admin-divisions/"+writeID)

	requireStatus(t, rec, http.StatusNoContent)
	if rec.Body.Len() != 0 {
		t.Fatalf("204 обязан быть без тела: %q", rec.Body)
	}
	if got := svc.gotIDs[0]; got != writeID {
		t.Fatalf("id = %s", got)
	}
}

func TestDivisionDeleteNotFoundIs404(t *testing.T) {
	svc := &fakeDivisions{deleteErr: models.ErrNotFound}

	rec := delD(t, NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}), "/api/admin-divisions/"+writeID)

	requireStatus(t, rec, http.StatusNotFound)
}

func TestDivisionDeleteInvalidIDIs422(t *testing.T) {
	svc := &fakeDivisions{deleteErr: &models.ValidationError{Field: "id", Reason: "неверный формат"}}

	rec := delD(t, NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}), "/api/admin-divisions/ad-1")

	requireStatus(t, rec, http.StatusUnprocessableEntity)
}

// TestDivisionDeleteInUseIs409WithReferrers: тело 409 содержит список ссылающихся.
func TestDivisionDeleteInUseIs409WithReferrers(t *testing.T) {
	svc := &fakeDivisions{deleteErr: &models.InUseError{
		Type: models.TypeAdministrativeDivision, ID: writeID,
		Referrers: []models.EntityRef{{Type: models.TypeAdministrativeDivision, ID: writeRefID}},
	}}

	rec := delD(t, NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}), "/api/admin-divisions/"+writeID)

	requireStatus(t, rec, http.StatusConflict)
	want := `{"error":"administrative_division \"` + writeID + `\" используется: administrative_division ` + writeRefID +
		`","referrers":[{"type":"administrative_division","id":"` + writeRefID + `"}]}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

// TestDivisionCreateAnonymousIs401: без активной сессии запись отклоняется
// раньше разбора тела — сценарий не вызывается (auth.md §6, приёмка этапа C).
func TestDivisionCreateAnonymousIs401(t *testing.T) {
	svc := &fakeDivisions{}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin-divisions", strings.NewReader(`{"name":"x","type":"selo"}`))
	NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec, req)

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
	NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec, req)

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
	NewHandler(Deps{Divisions: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec, req)

	requireStatus(t, rec, http.StatusUnauthorized)
	if len(svc.gotIDs) != 0 {
		t.Fatalf("сценарий вызван анонимом: %v", svc.gotIDs)
	}
}
```

#### `internal/httpapi/store_test.go` (изменить — итоговое содержимое)
`internal/httpapi/store_test.go`:
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
	create_attachment "github.com/amarin/genodex/internal/usecases/create_attachment"
	create_church "github.com/amarin/genodex/internal/usecases/create_church"
	create_citation "github.com/amarin/genodex/internal/usecases/create_citation"
	create_division "github.com/amarin/genodex/internal/usecases/create_division"
	create_estate "github.com/amarin/genodex/internal/usecases/create_estate"
	create_given_name "github.com/amarin/genodex/internal/usecases/create_given_name"
	create_note "github.com/amarin/genodex/internal/usecases/create_note"
	create_parish "github.com/amarin/genodex/internal/usecases/create_parish"
	create_patronymic "github.com/amarin/genodex/internal/usecases/create_patronymic"
	create_repository "github.com/amarin/genodex/internal/usecases/create_repository"
	create_source "github.com/amarin/genodex/internal/usecases/create_source"
	create_surname "github.com/amarin/genodex/internal/usecases/create_surname"
	create_title "github.com/amarin/genodex/internal/usecases/create_title"
	delete_archive "github.com/amarin/genodex/internal/usecases/delete_archive"
	delete_attachment "github.com/amarin/genodex/internal/usecases/delete_attachment"
	delete_church "github.com/amarin/genodex/internal/usecases/delete_church"
	delete_citation "github.com/amarin/genodex/internal/usecases/delete_citation"
	delete_division "github.com/amarin/genodex/internal/usecases/delete_division"
	delete_estate "github.com/amarin/genodex/internal/usecases/delete_estate"
	delete_given_name "github.com/amarin/genodex/internal/usecases/delete_given_name"
	delete_note "github.com/amarin/genodex/internal/usecases/delete_note"
	delete_parish "github.com/amarin/genodex/internal/usecases/delete_parish"
	delete_patronymic "github.com/amarin/genodex/internal/usecases/delete_patronymic"
	delete_repository "github.com/amarin/genodex/internal/usecases/delete_repository"
	delete_source "github.com/amarin/genodex/internal/usecases/delete_source"
	delete_surname "github.com/amarin/genodex/internal/usecases/delete_surname"
	delete_title "github.com/amarin/genodex/internal/usecases/delete_title"
	get_archive "github.com/amarin/genodex/internal/usecases/get_archive"
	get_attachment "github.com/amarin/genodex/internal/usecases/get_attachment"
	get_church "github.com/amarin/genodex/internal/usecases/get_church"
	get_citation "github.com/amarin/genodex/internal/usecases/get_citation"
	get_division "github.com/amarin/genodex/internal/usecases/get_division"
	get_estate "github.com/amarin/genodex/internal/usecases/get_estate"
	get_given_name "github.com/amarin/genodex/internal/usecases/get_given_name"
	get_note "github.com/amarin/genodex/internal/usecases/get_note"
	get_parish "github.com/amarin/genodex/internal/usecases/get_parish"
	get_patronymic "github.com/amarin/genodex/internal/usecases/get_patronymic"
	get_repository "github.com/amarin/genodex/internal/usecases/get_repository"
	get_source "github.com/amarin/genodex/internal/usecases/get_source"
	get_surname "github.com/amarin/genodex/internal/usecases/get_surname"
	get_title "github.com/amarin/genodex/internal/usecases/get_title"
	list_archives "github.com/amarin/genodex/internal/usecases/list_archives"
	list_attachments "github.com/amarin/genodex/internal/usecases/list_attachments"
	list_churches "github.com/amarin/genodex/internal/usecases/list_churches"
	list_citations "github.com/amarin/genodex/internal/usecases/list_citations"
	list_divisions "github.com/amarin/genodex/internal/usecases/list_divisions"
	list_estates "github.com/amarin/genodex/internal/usecases/list_estates"
	list_given_names "github.com/amarin/genodex/internal/usecases/list_given_names"
	list_notes "github.com/amarin/genodex/internal/usecases/list_notes"
	list_parishes "github.com/amarin/genodex/internal/usecases/list_parishes"
	list_patronymics "github.com/amarin/genodex/internal/usecases/list_patronymics"
	list_repositories "github.com/amarin/genodex/internal/usecases/list_repositories"
	list_sources "github.com/amarin/genodex/internal/usecases/list_sources"
	list_surnames "github.com/amarin/genodex/internal/usecases/list_surnames"
	list_titles "github.com/amarin/genodex/internal/usecases/list_titles"
	search_archives "github.com/amarin/genodex/internal/usecases/search_archives"
	search_attachments "github.com/amarin/genodex/internal/usecases/search_attachments"
	search_churches "github.com/amarin/genodex/internal/usecases/search_churches"
	search_citations "github.com/amarin/genodex/internal/usecases/search_citations"
	search_divisions "github.com/amarin/genodex/internal/usecases/search_divisions"
	search_estates "github.com/amarin/genodex/internal/usecases/search_estates"
	search_given_names "github.com/amarin/genodex/internal/usecases/search_given_names"
	search_notes "github.com/amarin/genodex/internal/usecases/search_notes"
	search_parishes "github.com/amarin/genodex/internal/usecases/search_parishes"
	search_patronymics "github.com/amarin/genodex/internal/usecases/search_patronymics"
	search_repositories "github.com/amarin/genodex/internal/usecases/search_repositories"
	search_sources "github.com/amarin/genodex/internal/usecases/search_sources"
	search_surnames "github.com/amarin/genodex/internal/usecases/search_surnames"
	search_titles "github.com/amarin/genodex/internal/usecases/search_titles"
	update_archive "github.com/amarin/genodex/internal/usecases/update_archive"
	update_attachment "github.com/amarin/genodex/internal/usecases/update_attachment"
	update_church "github.com/amarin/genodex/internal/usecases/update_church"
	update_citation "github.com/amarin/genodex/internal/usecases/update_citation"
	update_division "github.com/amarin/genodex/internal/usecases/update_division"
	update_estate "github.com/amarin/genodex/internal/usecases/update_estate"
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

#### `internal/transport/admin_division_test.go` (изменить — итоговое содержимое)
`internal/transport/admin_division_test.go`:
```go
package transport

import (
	"encoding/json"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

func TestAdminDivisionFromModel(t *testing.T) {
	parent := models.ID("ad-root")
	src := models.AdministrativeDivision{
		ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo,
		ParentID: &parent, Variants: []string{"Давыдова"},
	}

	got := AdminDivisionFromModel(src)

	if got.ID != "ad-1" || got.Name != "Давыдово" || got.Type != models.AdminDivisionSelo ||
		got.ParentID == nil || *got.ParentID != "ad-root" {
		t.Fatalf("got %+v", got)
	}

	if got.ParentID == src.ParentID {
		t.Fatal("контракт делит указатель ParentID с моделью")
	}
}

// Контракт на проводе зафиксирован строкой: пустой список — `[]`, поля —
// id/name/type/parent_id, у корня parent_id — null.
func TestAdminDivisionsJSONContract(t *testing.T) {
	empty, err := json.Marshal(AdminDivisionsFromModels(nil))
	if err != nil {
		t.Fatal(err)
	}

	if string(empty) != `[]` {
		t.Fatalf("empty = %s, want []", empty)
	}

	root := models.ID("ad-root")

	list, err := json.Marshal(AdminDivisionsFromModels([]models.AdministrativeDivision{
		{ID: "ad-root", Name: "Московская", Type: models.AdminDivisionGovernorate},
		{ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo, ParentID: &root},
	}))
	if err != nil {
		t.Fatal(err)
	}

	want := `[{"id":"ad-root","name":"Московская","type":"governorate","parent_id":null,"sources":[]},` +
		`{"id":"ad-1","name":"Давыдово","type":"selo","parent_id":"ad-root","sources":[]}]`
	if string(list) != want {
		t.Fatalf("list = %s, want %s", list, want)
	}
}
```

#### `internal/mcp/division_test.go` (изменить — итоговое содержимое)
`internal/mcp/division_test.go`:
```go
package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/amarin/genodex/internal/models"
)

// fakeDivisions запоминает запрос и отдаёт заданный ответ.
type fakeDivisions struct {
	list []models.AdministrativeDivision
	err  error
	got  models.DivisionQuery
	call int

	getDiv    models.AdministrativeDivision
	created   models.AdministrativeDivision
	gotCreate models.AdministrativeDivision
	updated   models.AdministrativeDivision
	gotIDs    []models.ID
	deleteErr error

	search    []models.AdministrativeDivision
	searchErr error
	gotSearch models.DivisionSearchQuery

	gotListAccess   models.Access
	gotSearchAccess models.Access
}

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

func (f *fakeDivisions) GetDivision(_ context.Context, id models.ID) (models.AdministrativeDivision, error) {
	f.gotIDs = append(f.gotIDs, id)

	if f.err != nil {
		return models.AdministrativeDivision{}, f.err
	}

	return f.getDiv, nil
}

func (f *fakeDivisions) CreateDivision(_ context.Context, d models.AdministrativeDivision) (models.AdministrativeDivision, error) {
	f.gotCreate = d

	if f.err != nil {
		return models.AdministrativeDivision{}, f.err
	}

	return f.created, nil
}

func (f *fakeDivisions) UpdateDivision(_ context.Context, d models.AdministrativeDivision) error {
	f.updated = d

	return f.err
}

func (f *fakeDivisions) DeleteDivision(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callDivisionList(t *testing.T, svc *fakeDivisions, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := divisionListHandler(svc)(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func resultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()

	text, ok := mcp.AsTextContent(res.Content[0])
	if !ok {
		t.Fatalf("content[0] = %T, want TextContent", res.Content[0])
	}

	return text.Text
}

func TestDivisionListToolContract(t *testing.T) {
	root := models.ID("ad-root")
	svc := &fakeDivisions{list: []models.AdministrativeDivision{
		{ID: "ad-root", Name: "Московская", Type: models.AdminDivisionGovernorate},
		{ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo, ParentID: &root},
	}}

	res := callDivisionList(t, svc, nil)

	want := `[{"id":"ad-root","name":"Московская","type":"governorate","parent_id":null,"sources":[]},` +
		`{"id":"ad-1","name":"Давыдово","type":"selo","parent_id":"ad-root","sources":[]}]`
	if res.IsError || resultText(t, res) != want {
		t.Fatalf("isError=%v text=%s, want %s", res.IsError, resultText(t, res), want)
	}

	if svc.got != (models.DivisionQuery{}) {
		t.Fatalf("вызов без аргументов дошёл до сценария как %+v", svc.got)
	}
}

// TestDivisionListToolPassesArguments: аргументы kind/type/limit/offset
// (число приходит из JSON как float64) доходят до сценария.
func TestDivisionListToolPassesArguments(t *testing.T) {
	svc := &fakeDivisions{}

	res := callDivisionList(t, svc, map[string]any{
		"kind": "settlement", "type": "selo", "limit": float64(20), "offset": float64(40),
	})
	if res.IsError {
		t.Fatalf("неожиданная ошибка тула: %s", resultText(t, res))
	}

	want := models.DivisionQuery{
		Kind: models.DivisionKindSettlement, Type: models.AdminDivisionSelo,
		Page: models.Page{Limit: 20, Offset: 40},
	}
	if svc.got != want {
		t.Fatalf("запрос %+v, ожидался %+v", svc.got, want)
	}
}

// TestDivisionListToolBadNumberIsToolError: нечисловой limit — ошибка тула, сценарий не вызван.
func TestDivisionListToolBadNumberIsToolError(t *testing.T) {
	svc := &fakeDivisions{}

	res := callDivisionList(t, svc, map[string]any{"limit": "abc"})

	if !res.IsError || !strings.Contains(resultText(t, res), "limit") || svc.call != 0 {
		t.Fatalf("isError=%v text=%s calls=%d; ожидалась ошибка про limit без вызова сценария",
			res.IsError, resultText(t, res), svc.call)
	}
}

// TestDivisionListToolNullAndFractionalNumbers: явный null — как отсутствие
// аргумента; дробное число — ошибка тула, а не молчаливое усечение.
func TestDivisionListToolNullAndFractionalNumbers(t *testing.T) {
	svc := &fakeDivisions{}

	res := callDivisionList(t, svc, map[string]any{"limit": nil, "offset": nil})
	if res.IsError || svc.got != (models.DivisionQuery{}) {
		t.Fatalf("null-аргументы: isError=%v запрос=%+v; ожидалось значение по умолчанию", res.IsError, svc.got)
	}

	svc = &fakeDivisions{}

	res = callDivisionList(t, svc, map[string]any{"limit": 1.5})
	if !res.IsError || svc.call != 0 {
		t.Fatalf("дробный limit: isError=%v calls=%d; ожидалась ошибка тула без вызова сценария", res.IsError, svc.call)
	}
}

func TestDivisionListToolErrorsAreToolErrors(t *testing.T) {
	for name, err := range map[string]error{
		"ошибка проверки": &models.ValidationError{Field: "kind", Reason: "неизвестный вид"},
		"сбой хранилища":  errors.New("хранилище недоступно"),
	} {
		res := callDivisionList(t, &fakeDivisions{err: err}, nil)

		if !res.IsError || !strings.Contains(resultText(t, res), err.Error()) {
			t.Errorf("%s: isError=%v text=%s; ожидалась ошибка тула с текстом %q", name, res.IsError, resultText(t, res), err)
		}
	}
}

func callDivisionSearch(t *testing.T, svc *fakeDivisions, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := divisionSearchHandler(svc)(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestDivisionSearchTool(t *testing.T) {
	svc := &fakeDivisions{search: []models.AdministrativeDivision{
		{ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo},
	}}

	res := callDivisionSearch(t, svc, map[string]any{"q": "давы"})

	want := `[{"id":"ad-1","name":"Давыдово","type":"selo","parent_id":null,"sources":[]}]`
	if res.IsError || resultText(t, res) != want {
		t.Fatalf("isError=%v text=%s, want %s", res.IsError, resultText(t, res), want)
	}

	if svc.gotSearch.Text != "давы" {
		t.Fatalf("gotSearch.Text = %q, ожидалось %q", svc.gotSearch.Text, "давы")
	}
}

func TestDivisionSearchToolEmptyText(t *testing.T) {
	svc := &fakeDivisions{}

	res := callDivisionSearch(t, svc, map[string]any{"q": ""})

	if res.IsError || resultText(t, res) != `[]` {
		t.Fatalf("isError=%v text=%s, want []", res.IsError, resultText(t, res))
	}
}

func TestDivisionSearchToolInvalidLimitIsError(t *testing.T) {
	svc := &fakeDivisions{}

	res := callDivisionSearch(t, svc, map[string]any{"q": "давы", "limit": "abc"})

	if !res.IsError || svc.call != 0 {
		t.Fatalf("isError=%v calls=%d; ожидалась ошибка тула без вызова сценария", res.IsError, svc.call)
	}
}

// TestDivisionListToolWithParent: parent_id доходит до сценария.
func TestDivisionListToolWithParent(t *testing.T) {
	svc := &fakeDivisions{}

	res := callDivisionList(t, svc, map[string]any{"parent_id": "ad-root"})
	if res.IsError {
		t.Fatalf("неожиданная ошибка тула: %s", resultText(t, res))
	}

	want := models.ID("ad-root")
	if svc.got.ParentID == nil || *svc.got.ParentID != want {
		t.Fatalf("got.ParentID = %v, ожидался %s", svc.got.ParentID, want)
	}
}

// TestNewServerRegistersDivisionSearchTool: тул division_search зарегистрирован.
func TestNewServerRegistersDivisionSearchTool(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}}).ListTools()

	if _, ok := tools["division_search"]; !ok {
		t.Errorf("тул division_search не зарегистрирован: %v", tools)
	}
}

// TestNewServerRegistersDivisionListOnly: тул зарегистрирован под новым именем,
// прежнего settlement_list нет.
func TestNewServerRegistersDivisionListOnly(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}}).ListTools()

	if _, ok := tools["division_list"]; !ok {
		t.Errorf("тул division_list не зарегистрирован: %v", tools)
	}

	if _, ok := tools["settlement_list"]; ok {
		t.Error("прежний тул settlement_list всё ещё зарегистрирован")
	}
}

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

#### `internal/mcp/division_write_test.go` (изменить — итоговое содержимое)
`internal/mcp/division_write_test.go`:
```go
package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

func callDivisionWrite(t *testing.T, handler server.ToolHandlerFunc, svc *fakeDivisions, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

// TestDivisionGetToolContract: чтение по id возвращает JSON контракта.
func TestDivisionGetToolContract(t *testing.T) {
	svc := &fakeDivisions{getDiv: models.AdministrativeDivision{
		ID: "ad-root", Name: "Московская", Type: models.AdminDivisionGovernorate,
	}}

	res := callDivisionWrite(t, divisionGetHandler(svc), svc, map[string]any{"id": "ad-root"})

	want := `{"id":"ad-root","name":"Московская","type":"governorate","parent_id":null,"sources":[]}`
	if res.IsError || resultText(t, res) != want {
		t.Fatalf("isError=%v text=%s, want %s", res.IsError, resultText(t, res), want)
	}
	if len(svc.gotIDs) != 1 || svc.gotIDs[0] != "ad-root" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

// TestDivisionGetToolErrorsAreToolErrors: отсутствующая единица и неверный формат
// id — ошибка тула, а не результата.
func TestDivisionGetToolErrorsAreToolErrors(t *testing.T) {
	for name, err := range map[string]error{
		"не найдено":      models.ErrNotFound,
		"неверный формат": &models.ValidationError{Field: "id"},
		"сбой хранилища":  errors.New("хранилище недоступно"),
	} {
		res := callDivisionWrite(t, divisionGetHandler(&fakeDivisions{err: err}), &fakeDivisions{err: err}, map[string]any{"id": "ad-root"})

		if !res.IsError || resultText(t, res) == "" {
			t.Errorf("%s: isError=%v text=%q; ожидалась ошибка тула", name, res.IsError, resultText(t, res))
		}
	}
}

// TestDivisionCreateToolPassesModel: аргументы name/type/parent_id доходят до
// сценария моделью; ответ сценария возвращается как JSON.
func TestDivisionCreateToolPassesModel(t *testing.T) {
	svc := &fakeDivisions{created: models.AdministrativeDivision{
		ID: "ad-new", Name: "Давыдово", Type: models.AdminDivisionSelo,
	}}

	res := callDivisionWrite(t, divisionCreateHandler(svc), svc,
		map[string]any{"name": "Давыдово", "type": "selo", "parent_id": "ad-root"})

	want := `{"id":"ad-new","name":"Давыдово","type":"selo","parent_id":null,"sources":[]}`
	if res.IsError || resultText(t, res) != want {
		t.Fatalf("isError=%v text=%s, want %s", res.IsError, resultText(t, res), want)
	}

	got := svc.gotCreate
	if got.Name != "Давыдово" || got.Type != models.AdminDivisionSelo {
		t.Fatalf("gotCreate = %+v", got)
	}
	if got.ParentID == nil || *got.ParentID != "ad-root" {
		t.Fatalf("gotCreate.ParentID = %v, ожидался ad-root", got.ParentID)
	}
}

// TestDivisionCreateToolEmptyParentMeansRoot: пустой parent_id — корень.
func TestDivisionCreateToolEmptyParentMeansRoot(t *testing.T) {
	svc := &fakeDivisions{created: models.AdministrativeDivision{}}

	callDivisionWrite(t, divisionCreateHandler(svc), svc, map[string]any{"name": "Московская", "type": "governorate", "parent_id": ""})

	if svc.gotCreate.ParentID != nil {
		t.Fatalf("gotCreate.ParentID = %v, ожидался корень", svc.gotCreate.ParentID)
	}
}

// TestDivisionCreateToolErrorIsToolError: ошибка проверки в сценарии — ошибка тула.
func TestDivisionCreateToolErrorIsToolError(t *testing.T) {
	ve := &models.ValidationError{Field: "name", Reason: "пустое имя"}
	res := callDivisionWrite(t, divisionCreateHandler(&fakeDivisions{err: ve}), &fakeDivisions{err: ve},
		map[string]any{"name": "", "type": "selo"})

	if !res.IsError || !strings.Contains(resultText(t, res), ve.Error()) {
		t.Fatalf("isError=%v text=%q", res.IsError, resultText(t, res))
	}
}

// TestDivisionUpdateToolMergesFields: обновление заменяет name/type/parent_id;
// прочие поля текущей версии (например parent_id-корень) не затрагиваются.
func TestDivisionUpdateToolMergesFields(t *testing.T) {
	svc := &fakeDivisions{getDiv: models.AdministrativeDivision{
		ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo,
		ParentID: func() *models.ID { v := models.ID("ad-root"); return &v }(),
	}}

	res := callDivisionWrite(t, divisionUpdateHandler(svc), svc,
		map[string]any{"id": "ad-1", "name": "Давыдова", "type": "selo", "parent_id": ""})

	if res.IsError {
		t.Fatalf("неожиданная ошибка тула: %s", resultText(t, res))
	}

	got := svc.updated
	if got.ID != "ad-1" || got.Name != "Давыдова" || got.Type != models.AdminDivisionSelo {
		t.Fatalf("updated = %+v", got)
	}
	if got.ParentID != nil {
		t.Fatalf("updated.ParentID = %v, пустой parent_id = корень", got.ParentID)
	}
}

// TestDivisionUpdateToolNotFoundIsError: отсутствующая единица — ошибка тула.
func TestDivisionUpdateToolNotFoundIsError(t *testing.T) {
	res := callDivisionWrite(t, divisionUpdateHandler(&fakeDivisions{err: models.ErrNotFound}),
		&fakeDivisions{err: models.ErrNotFound}, map[string]any{"id": "ad-1", "name": "Давыдова"})

	if !res.IsError {
		t.Fatalf("text = %q; ожидалась ошибка тула", resultText(t, res))
	}
}

// TestDivisionDeleteToolContract: удаление возвращает текст подтверждения.
func TestDivisionDeleteToolContract(t *testing.T) {
	svc := &fakeDivisions{}

	res := callDivisionWrite(t, divisionDeleteHandler(svc), svc, map[string]any{"id": "ad-1"})

	if res.IsError {
		t.Fatalf("неожиданная ошибка тула: %s", resultText(t, res))
	}
	if text := resultText(t, res); !strings.Contains(text, `"ad-1"`) || !strings.Contains(text, "удалено") {
		t.Fatalf("text = %q, ожидалось подтверждение удаления ad-1", text)
	}
	if len(svc.gotIDs) != 1 || svc.gotIDs[0] != "ad-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

// TestDivisionDeleteToolInUseIsError: занятая единица — ошибка тула с текстом ошибки.
func TestDivisionDeleteToolInUseIsError(t *testing.T) {
	iu := &models.InUseError{Type: "administrative_division", ID: "ad-1"}
	res := callDivisionWrite(t, divisionDeleteHandler(&fakeDivisions{deleteErr: iu}),
		&fakeDivisions{deleteErr: iu}, map[string]any{"id": "ad-1"})

	if !res.IsError || !strings.Contains(resultText(t, res), iu.Error()) {
		t.Fatalf("isError=%v text=%q", res.IsError, resultText(t, res))
	}
}

// TestNewServerRegistersDivisionWriteTools: тулы записи зарегистрированы.
func TestNewServerRegistersDivisionWriteTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}}).ListTools()

	for _, name := range []string{"division_get", "division_create", "division_update", "division_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
```

### Шаг 2.8. Real-store интеграционные тесты: Source, Citation, ретрофит Archive

По правилу, закреплённому финальным ревью подпроекта 4 (`entity-write.md`'s working notes) — каждая сущность с проверкой существования (FK) должна получить сквозной тест на настоящем хранилище (`sqlstore.Open(t.TempDir())`, не fake), а не только на fake-сторе unit-тестах усечесаев. `TestSourceWriteContractWithRealStore` и `TestCitationWriteContractWithRealStore` — новые тесты, по образцу `TestRepositoryWriteContractWithRealStore`/`TestArchiveWriteContractWithRealStore`. `TestArchiveWriteContractWithRealStore` — СУЩЕСТВУЮЩИЙ тест, ДОПОЛНЕН шагами на `sources` (создать `Citation`, создать `Archive` с ссылкой на неё → 201, с несуществующей → 422 `sources[0].citation_id`) — само расширение могло появиться только здесь, а не в Задаче 1: оно требует ОБЕИХ половин прохода одновременно — `Citation` (Задача 1) и `Archive`, принимающий `sources` (эта задача). Итоговое содержимое всего файла ниже уже включает все три теста.

#### `internal/httpapi/write_store_test.go` (изменить — итоговое содержимое)
`internal/httpapi/write_store_test.go`:
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
	h := httpapi.NewAPIHandler(httpapi.Deps{Divisions: newDivisionService(t, st), Auth: authSvc, DocsFS: fstest.MapFS{}, TrustProxy: false})

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
```

### Шаг 2.9. Документация: `docs/usage.md`, `docs/architecture.md`, `CHANGELOG.md`, `entity-write.md` §3.3

По правилу, закреплённому после подпроекта 4 (там этот шаг был пропущен в плане уже в четвёртый раз подряд и снова пойман только финальным ревью) — доки/CHANGELOG пишутся ЗДЕСЬ, в плане, а не оставляются на финальное ревью. Ниже — точные вставки; порядок существующих строк не меняется, только добавляются новые.

#### `docs/usage.md` — две новые строки HTTP-таблицы (после `/api/attachments`, до `/static/`)
```markdown
| `/api/sources` | Источники доказательств (JSON): `GET` — список `[{"id", "kind", "title", "author", "date", "reliability", "repository_id", "notes", "private"}]`; `POST` — создание; `GET/PUT/DELETE /api/sources/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404); `GET /api/sources/search?q=` — поиск по началу названия или автора. `date` — структурированная дата (`FactDate`). `repository_id` — мягкая ссылка на `/api/repositories`: несуществующий id при создании/изменении — 422 на поле `repository_id`. |
| `/api/citations` | Цитаты из источника (JSON): `GET` — список `[{"id", "source_id", "anchor", "text", "note", "private"}]`; `POST` — создание; `GET/PUT/DELETE /api/citations/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404); `GET /api/citations/search?q=` — поиск по началу текста. `source_id` — обязательная строгая ссылка на `/api/sources`: несуществующий id — 422 на поле `source_id`. `anchor` — необязательная полиморфная привязка «где именно» (плоский объект с дискриминатором `kind`: `archive` — `node_id`/`document_id`/`page`/`rect`, `file` — `attachment_id`/`timecode`, `url` — `url`; пусто или отсутствует — без привязки); если задана и несёт ссылку (`node_id`/`document_id`/`attachment_id`), существование тоже проверяется — 422 на поле `anchor.node_id`/`anchor.document_id`/`anchor.attachment_id`. |
```

#### `docs/usage.md` — новая строка про `sources` (заменяет прежнюю «read-only» строку после HTTP-таблицы)
Заменить:
```markdown
`sources` у `/api/repositories`, `/api/churches`, `/api/parishes`, `/api/archives`, `/api/notes` — read-only: create/update DTO его не принимают, изменить нельзя (Citation ещё без CRUD, подпроект 5).
```
на:
```markdown
`sources` у `/api/admin-divisions`, `/api/repositories`, `/api/churches`, `/api/parishes`, `/api/archives`, `/api/notes` — редактируется с подпроекта 5 (`Citation` теперь имеет CRUD): `POST`/`PUT` принимают `sources: [{"citation_id", "reliability"?, "role"?, "note"?}]`; `target_type`/`target_id` клиент не отправляет — сервер подставляет владельца из контекста. Несуществующий `citation_id` — 422 на поле `sources[i].citation_id` (по индексу элемента). `/api/admin-divisions` — единственный маршрут, где `sources` раньше не было в ответе вовсе (не просто read-only) — теперь есть, как и у остальных пяти.
```

#### `docs/usage.md` — 12 новых строк таблицы MCP-тулов (после `attachment_delete`, до `### HTTP API`)
```markdown
| `source_list` | Список источников; аргументы `limit`, `offset` |
| `source_search` | Поиск источников по началу названия или автора; аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `source_get` | Запись по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула (как отсутствующая) |
| `source_create` | Создание записи: `kind` (обязателен, закрытый перечень), `title` (обязателен), `author`, `date` (структурированная дата, см. `factDateObjectProperties`), `reliability` (обязателен, закрытый перечень), `repository_id` (необязателен, если задан — проверяется, ошибка тула на поле `repository_id`), `notes`, `private`; id генерирует сервер |
| `source_update` | Изменение записи: полная замена `kind`/`title`/`author`/`date`/`reliability`/`repository_id`/`notes`/`private`; несуществующий `repository_id` — ошибка тула на поле `repository_id` |
| `source_delete` | Удаление записи по `id`; занятая другой сущностью (например, `Citation`) — ошибка тула |
| `citation_list` | Список цитат; аргументы `limit`, `offset` |
| `citation_search` | Поиск цитат по началу текста; аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `citation_get` | Запись по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула (как отсутствующая) |
| `citation_create` | Создание записи: `source_id` (обязателен, строгая ссылка — несуществующий id — ошибка тула на поле `source_id`), `anchor` (необязательный объект — полиморфная привязка, см. `anchorObjectProperties`; если задана и несёт ссылку, существование тоже проверяется — ошибка тула на поле `anchor.node_id`/`anchor.document_id`/`anchor.attachment_id`), `text`, `note`, `private`; id генерирует сервер |
| `citation_update` | Изменение записи: полная замена `source_id`/`anchor`/`text`/`note`/`private`; несуществующий `source_id` или ссылка внутри `anchor` — ошибка тула на соответствующем поле |
| `citation_delete` | Удаление записи по `id`; занятая другой сущностью (`SourceLink` у любой сущности с доказательствами) — ошибка тула |
```
И следом отдельным абзацем (после таблицы тулов, перед `### HTTP API`):
```markdown
`<entity>_create`/`<entity>_update` у `division`/`repository`/`church`/`parish`/`archive`/`note` с подпроекта 5 принимают аргумент `sources` — массив объектов `{citation_id, reliability?, role?, note?}` (первый MCP-аргумент вида «массив объектов» в программе, см. `sourceLinkObjectProperties`; раньше массивы были только строками). Несуществующий `citation_id` — ошибка тула на поле `sources[i].citation_id`.
```

#### `docs/architecture.md` — строка `internal/httpapi/`
Заменить:
```markdown
| `internal/httpapi/` | Слой HTTP API: JSON-роуты `/api/*` (health, admin-divisions, surnames, patronymics, estates, titles, given-names, repositories, churches, parishes, archives, notes, attachments, auth, docs). Использует те же сценарии, что и MCP. |
```
на:
```markdown
| `internal/httpapi/` | Слой HTTP API: JSON-роуты `/api/*` (health, admin-divisions, surnames, patronymics, estates, titles, given-names, repositories, churches, parishes, archives, notes, attachments, sources, citations, auth, docs). Использует те же сценарии, что и MCP. |
```

#### `CHANGELOG.md` — два новых пункта (после пункта про Заметки/Вложения)
Вставить перед пунктом «Веб: единая точка входа» — итоговый фрагмент для сверки:
`CHANGELOG.md (фрагмент — новые пункты)`:
```markdown
- Источники/Цитаты (`Source`/`Citation`) — цепочка доказательств, полный
  CRUD: HTTP (`/api/sources*`, `/api/citations*`) и MCP (`source_*`/
  `citation_*` — по 6 тулов на сущность), веб-страницы (список/просмотр/
  редактирование/создание). Первый полиморфный тип в программе
  (`Citation.Anchor` — архивный узел/документ, файл-вложение или внешняя
  ссылка; плоское представление с дискриминатором `kind`, по образцу
  `FactDate`, не вложенный union) и первый MCP-аргумент вида «массив
  объектов» (`sources`, см. ниже — раньше массивы были только строками)
  (`internal/transport/{source,source_write,citation,citation_write,
  anchor}.go`, `internal/usecases/{list,search,get,create,update,delete}_
  {source,citation}*`, `internal/httpapi/{source,citation}*.go`,
  `internal/mcp/{source,citation}.go`, `internal/mcp/object_args.go`
  (`anchorObjectProperties`/`optionalAnchor`,
  `sourceLinkObjectProperties`/`optionalSourceLinks`), `web/src/
  AnchorEditor.tsx`, `web/src/SourceLinkList.tsx`, `web/src/pages/
  {SourcesList,SourceView,SourceForm,CitationsList,CitationView,
  CitationForm}.tsx`).
- `Sources []SourceLink` разблокирован для редактирования у всех 6
  сущностей, где есть: `AdministrativeDivision` (впервые видим на чтении
  тоже — раньше отсутствовал в контракте вовсе), `Repository`, `Church`,
  `Parish`, `Archive`, `Note`. Несуществующий `citation_id` в списке
  `sources` — 422 на поле `sources[i].citation_id` (проверка в той же
  транзакции, что и сохранение — `Repository`/`Church`/`Parish` при этом
  впервые стали транзакционными сценариями, раньше у них не было ни
  одного FK для проверки) (`internal/transport/{admin_division,
  admin_division_write,repository_write,church_write,parish_write,
  archive_write,note_write}.go`, `internal/usecases/{create,update}_
  {division,repository,church,parish,archive,note}/*`, `internal/httpapi/
  {division,repository,church,parish,archive,note}_write.go`,
  `internal/mcp/{division,repository,church,parish,archive,note}.go`,
  `web/src/pages/{Division,Repository,Church,Parish,Archive,Note}
  {Form,View}.tsx`).
```

#### `docs/data-model/entity-write.md` — новая секция §3.3 (после §3.2, до `## 4.`)
`docs/data-model/entity-write.md (фрагмент — новая секция §3.3)`:
```markdown
### 3.3. Подпроект 5 (цепочка доказательств: `Source`, `Citation`) — новые паттерны

- **Полиморфный тип.** `Citation.Anchor` — первый полиморфный тип в
  программе: интерфейс `models.Anchor` с тремя реализациями
  (`ArchiveAnchor`/`FileAnchor`/`URLAnchor`) или `nil`. Контракт
  (`transport.Anchor`) — плоское представление с дискриминатором `kind` и
  полями всех вариантов вместе (по образцу `FactDate`), а не вложенный
  union — проще для JSON REST и MCP-объектного аргумента; конвертация
  туда/обратно через `switch v := a.(type)`. Ссылки внутри якоря
  (`ArchiveAnchor.NodeID`/`DocumentID`, `FileAnchor.AttachmentID`)
  проверяются в той же транзакции, что источник (`SourceID`) и
  сохранение — `URLAnchor` ссылок не несёт, проверяется только
  структурно (`models.Citation.Validate()`).
- **MCP-аргумент вида «массив объектов».** До этого прохода
  MCP-аргументы-массивы были только строками (`mcp.WithStringItems()`).
  `sources` — первый массив объектов: `mcp.WithArray("sources",
  mcp.Items(map[string]any{"type": "object", "properties":
  sourceLinkObjectProperties()}), ...)` — `mcp.Items` принимает
  произвольную JSON-schema, не только примитивы. Чтение — `json.Marshal`
  сырого `[]any` из аргументов тула, затем `json.Unmarshal` в
  `[]transport.SourceLink` (см. `optionalSourceLinks`,
  `internal/mcp/object_args.go`) — тот же приём, что `optionalTextRef`/
  `optionalFactDate` для одиночных объектов, просто для среза.
- **Строгая ссылка внутри списка, с индексом в пути ошибки.**
  `SourceLink.CitationID` — обязательная ссылка на `Citation` у каждого
  элемента `Sources []SourceLink`. Проверяется в той же транзакции, что и
  сохранение владельца, ошибка — `*models.ValidationError` на поле
  `sources[%d].citation_id` (индекс элемента, не общее поле `sources`) —
  тот же путь, что модельная `Validate()` уже строит для структурных
  ошибок (`validateSourceLinks`/`indexed`), только уровнем выше
  (существование, а не форма). `SourceLink.TargetType`/`TargetID` клиент
  никогда не отправляет — сервер подставляет владельца из контекста при
  сохранении (`sqlstore.replaceSourceLinks`/`loadSourceLinks`), контракт
  (`transport.SourceLink.Model()`) их не заполняет.
- **Ретрофит read-only → editable может вскрыть отсутствие транзакции.**
  `Repository`/`Church`/`Parish` до этого прохода не имели ни одного FK —
  их `create_*`-сценарии делали плоский `store.SaveX(...)` без `InTx`
  вовсе. Как только у сущности появляется первая проверка существования
  (здесь — `sources[i].citation_id`), сценарий необходимо перевести на
  транзакционный `InTx`-паттерн (образец: `create_archive`) — само поле
  `Store`-зависимости (`deps.go`) меняет форму (`SaveX(ctx, *models.X)
  error` → `InTx(ctx, fn func(store.Store) error) error`). Проверить это
  явно для любой будущей сущности, получающей свой первый FK не при
  первом появлении, а позже, ретрофитом.
- **Read-контракт может отставать от read-only поля, даже когда пишущий
  контракт уже минимален осознанно.** `AdministrativeDivision` — самый
  узкий DTO в программе (только `name`/`type`/`parent_id`, с подпроекта
  1) — но `Sources` в нём отсутствовал не по тому же осознанному решению
  минимализма, что `Items`/`Variants`/`Notes` и др., а просто потому что
  поле `Sources` появилось в модели уже ПОСЛЕ того, как DTO был
  зафиксирован (подпроект 3), и контракт `AdministrativeDivision`
  никогда не пересматривался вместе с добавлением `Sources` другим
  сущностям. Обнаружено живой проверкой ретрофита (см. подпроект 5's
  предпосылку), не заранее. При добавлении нового поля, которое должно
  появиться у уже существующих сущностей «везде, где есть», явно
  сверять КАЖДУЮ такую сущность на предмет «а её read-контракт вообще
  видит это поле» — узкий DTO по решению и узкий DTO по недосмотру
  выглядят одинаково снаружи.
```

### Шаг 2.10. Рубеж

`gofmt -l .` пусто; `go build/vet/test ./...` зелёные — ожидается 1093 теста, 100 пакетов. Живой смок через `curl` (см. «Предпосылка») — не обязателен повторно: создать `Archive` с `sources: [{"citation_id": "<существующий>"}]` → 201, с несуществующим → 422 `{"field":"sources[0].citation_id"}`; создать `AdministrativeDivision` с `sources` → `GET` показывает поле (раньше не показывал вовсе).

### Шаг 2.11. Коммит

```bash
git add \
  internal/transport/admin_division.go internal/transport/admin_division_write.go \
  internal/transport/repository_write.go internal/transport/church_write.go internal/transport/parish_write.go \
  internal/transport/archive_write.go internal/transport/note_write.go \
  internal/usecases/create_division/scenario.go internal/usecases/create_division/scenario_test.go \
  internal/usecases/update_division/scenario.go internal/usecases/update_division/scenario_test.go \
  internal/usecases/create_repository internal/usecases/update_repository/scenario.go internal/usecases/update_repository/scenario_test.go \
  internal/usecases/create_church internal/usecases/update_church/scenario.go internal/usecases/update_church/scenario_test.go \
  internal/usecases/create_parish internal/usecases/update_parish/scenario.go internal/usecases/update_parish/scenario_test.go \
  internal/usecases/create_archive/scenario.go internal/usecases/create_archive/scenario_test.go \
  internal/usecases/update_archive/scenario.go internal/usecases/update_archive/scenario_test.go \
  internal/usecases/create_note/scenario.go internal/usecases/create_note/scenario_test.go \
  internal/usecases/update_note/scenario.go internal/usecases/update_note/scenario_test.go \
  internal/httpapi/division_write.go internal/httpapi/repository_write.go internal/httpapi/church_write.go \
  internal/httpapi/parish_write.go internal/httpapi/archive_write.go internal/httpapi/note_write.go \
  internal/mcp/division.go internal/mcp/repository.go internal/mcp/church.go internal/mcp/parish.go internal/mcp/archive.go internal/mcp/note.go \
  internal/httpapi/division_test.go internal/httpapi/division_write_test.go internal/httpapi/store_test.go \
  internal/httpapi/write_store_test.go \
  internal/transport/admin_division_test.go internal/mcp/division_test.go internal/mcp/division_write_test.go \
  docs/usage.md docs/architecture.md docs/data-model/entity-write.md CHANGELOG.md
git commit -m "feat(backend): редактируемый Sources у 6 сущностей + проверка sources[i].citation_id + доки + real-store тесты"
```

## Задача 3. Веб: страницы Source, Citation

**Интерфейсы, потребляемые из Задачи 1**: `httpapi`-маршруты `/api/sources`/`/api/citations` (JSON-форма — `transport.{Source,Citation}`/`{...}Create`/`{...}Update`, включая `date`/`anchor`).
**Интерфейсы, потребляемые из подпроектов 1-4**: `authFetch`/`ApiError` (`web/src/auth.ts`), `PageLayout`/каталог сущностей, `MAX_PAGE_LIMIT` (`web/src/api.ts`), `FactDateEditor`/`formatFactDate` (`web/src/FactDateEditor.tsx`, подпроект 3), `fetchRepositories` (подпроект 3), `fetchAttachments` (подпроект 4).

**Файлы:**
- Создать: `web/src/AnchorEditor.tsx`, `web/src/pages/{SourceForm,SourcesList,SourceView,CitationForm,CitationsList,CitationView}.tsx`
- Изменить: `web/src/api.ts`, `web/src/App.tsx`, `web/src/pages/EntityCatalog.tsx`

**Важно для исполнителя**: `AnchorEditor` — первый полиморфный редактор в программе. `kind`-`Select` переключает набор полей ЦЕЛИКОМ (смена kind заменяет весь объект на `{kind}` без переноса полей прежнего варианта — не пытаться сохранить `page`/`url` и т.п. при переключении). `anchor.attachment_id` (вариант `file`) — searchable `Select` (`fetchAttachments`, по образцу `useRepositoryOptions`), в отличие от `anchor.node_id`/`anchor.document_id` (вариант `archive`) — обычные текстовые поля (`ArchiveNode`/`ArchiveDocument` ещё без CRUD, подпроект 6). `SourceLinkList.tsx` (общий редактор Sources) СОЗДАЁТСЯ В ЭТОЙ ЖЕ Задаче (нужен `fetchCitations`, который появляется здесь), хотя ИСПОЛЬЗУЕТСЯ он только в Задаче 2 (бэкенд, уже готов) и Задаче 4 (веб-ретрофит 6 сущностей, следующая) — без него Задача 4 не сможет начаться.

### Шаг 3.1. `web/src/api.ts` — типы и функции Source/Citation/Anchor

Добавить в конец файла (после блока Attachment из подпроекта 4):
`web/src/api.ts (фрагмент — добавить в конец файла)`:
```typescript
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
```

### Шаг 3.2. `web/src/AnchorEditor.tsx` (создать)
`web/src/AnchorEditor.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Button, Input, InputNumber, Select, Space } from "antd";
import { MinusCircleOutlined, PlusOutlined } from "@ant-design/icons";
import { fetchAttachments, type Anchor, type AnchorKind, type Attachment } from "./api";

const KIND_OPTIONS: { value: AnchorKind; label: string }[] = [
  { value: "archive", label: "Архив (узел/документ)" },
  { value: "file", label: "Файл (вложение)" },
  { value: "url", label: "Ссылка" },
];

const EMPTY_ANCHOR: Anchor = { kind: "archive", page: 1 };

// useAttachmentOptions — заполняет Select вложений для anchor.attachment_id.
// В отличие от anchor.node_id/document_id (ArchiveNode/ArchiveDocument ещё
// без CRUD, подпроект 6 — обычные текстовые поля), Attachment уже есть с
// подпроекта 4, поэтому здесь — полноценный searchable Select, по образцу
// useRepositoryOptions/ArchiveForm.tsx (обсуждение подпроекта 5).
function useAttachmentOptions() {
  const [attachments, setAttachments] = useState<Attachment[]>([]);

  useEffect(() => {
    fetchAttachments({ limit: 500 })
      .then(setAttachments)
      .catch(() => setAttachments([]));
  }, []);

  return attachments.map((a) => ({ value: a.id, label: a.filename || a.uri || a.id }));
}

// AnchorEditor — редактор полиморфной привязки «где именно» у Citation
// (models.Anchor): дискриминатор kind (archive/file/url) переключает набор
// полей. Первый полиморфный тип в программе — редактор целиком заменяется
// при смене kind (EMPTY_ANCHOR для нового варианта), а не сохраняет поля
// прежнего варианта.
export function AnchorEditor({
  value,
  onChange,
  addLabel,
}: {
  value: Anchor | null | undefined;
  onChange: (next: Anchor | null) => void;
  addLabel: string;
}) {
  const attachmentOptions = useAttachmentOptions();

  if (value == null) {
    return (
      <Button type="dashed" onClick={() => onChange(EMPTY_ANCHOR)} icon={<PlusOutlined />} block>
        {addLabel}
      </Button>
    );
  }

  const set = (patch: Partial<Anchor>) => onChange({ ...value, ...patch });

  const setKind = (kind: AnchorKind) => {
    if (kind === "archive") {
      onChange({ kind, page: 1 });
    } else if (kind === "file") {
      onChange({ kind });
    } else {
      onChange({ kind });
    }
  };

  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      <Space wrap>
        <Select style={{ width: 220 }} value={value.kind} options={KIND_OPTIONS} onChange={setKind} />
        <MinusCircleOutlined onClick={() => onChange(null)} />
      </Space>
      {value.kind === "archive" && (
        <Space wrap style={{ width: "100%" }}>
          <Input
            placeholder="id архивного узла (AN-…)"
            value={value.node_id}
            onChange={(e) => set({ node_id: e.target.value })}
            style={{ width: 260 }}
          />
          <Input
            placeholder="id архивного документа (DC-…, необязательно)"
            value={value.document_id}
            onChange={(e) => set({ document_id: e.target.value })}
            style={{ width: 300 }}
          />
          <InputNumber
            placeholder="Страница"
            min={1}
            value={value.page}
            onChange={(v) => set({ page: v ?? undefined })}
            style={{ width: 110 }}
          />
          <Input
            placeholder="Область (необязательно)"
            value={value.rect}
            onChange={(e) => set({ rect: e.target.value })}
            style={{ width: 200 }}
          />
        </Space>
      )}
      {value.kind === "file" && (
        <Space wrap style={{ width: "100%" }}>
          <Select
            style={{ width: 300 }}
            placeholder="Вложение"
            options={attachmentOptions}
            value={value.attachment_id || undefined}
            onChange={(v) => set({ attachment_id: v })}
            showSearch
            filterOption={(input, option) =>
              (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
            }
          />
          <Input
            placeholder="Тайм-код (необязательно)"
            value={value.timecode}
            onChange={(e) => set({ timecode: e.target.value })}
            style={{ width: 160 }}
          />
        </Space>
      )}
      {value.kind === "url" && (
        <Input
          placeholder="https://…"
          value={value.url}
          onChange={(e) => set({ url: e.target.value })}
        />
      )}
    </Space>
  );
}
```

### Шаг 3.3. `web/src/SourceLinkList.tsx` (создать)

Общий редактор списка доказательств — используется в Задаче 4 (ретрофит 6 сущностей), но создаётся здесь, поскольку зависит от `fetchCitations`/`Citation`/`Reliability`, добавленных в Шаге 3.1.
`web/src/SourceLinkList.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Button, Input, Select, Space } from "antd";
import { MinusCircleOutlined, PlusOutlined } from "@ant-design/icons";
import { fetchCitations, type Citation, type Reliability, type SourceLink } from "./api";

const RELIABILITY_OPTIONS: { value: Reliability; label: string }[] = [
  { value: "primary", label: "Первичный" },
  { value: "contemporary", label: "Современник" },
  { value: "memory", label: "Со слов/по памяти" },
  { value: "indirect", label: "Косвенный" },
  { value: "unknown", label: "Неизвестна" },
];

function citationLabel(c: Citation): string {
  return c.text || c.id;
}

// useCitationOptions — заполняет Select цитат для SourceLink.citation_id, по
// образцу useRepositoryOptions/ArchiveForm.tsx.
function useCitationOptions() {
  const [citations, setCitations] = useState<Citation[]>([]);

  useEffect(() => {
    fetchCitations({ limit: 500 })
      .then(setCitations)
      .catch(() => setCitations([]));
  }, []);

  return citations.map((c) => ({ value: c.id, label: citationLabel(c) }));
}

// SourceLinkListEditor — общий редактор списка доказательств (Sources —
// подпроект 5: было read-only у Repository/Church/Parish/Archive/Note/
// AdministrativeDivision, теперь редактируется везде, где есть). Каждая
// строка — ссылка на цитату (Select с поиском) + достоверность именно этого
// утверждения по этой цитате + роль + заметка. target_type/target_id не
// редактируются и не отправляются — владелец подставляется сервером из
// контекста (см. api.ts's SourceLink и бэкенд-комментарий в
// transport.SourceLink.Model()).
export function SourceLinkListEditor({
  value,
  onChange,
  addLabel,
}: {
  value: SourceLink[];
  onChange: (next: SourceLink[]) => void;
  addLabel: string;
}) {
  const citationOptions = useCitationOptions();

  const setField = (i: number, patch: Partial<SourceLink>) => {
    const next = value.slice();
    next[i] = { ...next[i], ...patch };
    onChange(next);
  };

  const remove = (i: number) => onChange(value.filter((_, idx) => idx !== i));

  const add = () => onChange([...value, { citation_id: "" }]);

  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      {value.map((item, i) => (
        <Space key={i} wrap style={{ width: "100%" }}>
          <Select
            style={{ width: 260 }}
            placeholder="Цитата"
            options={citationOptions}
            value={item.citation_id || undefined}
            onChange={(v) => setField(i, { citation_id: v })}
            showSearch
            filterOption={(input, option) =>
              (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
            }
          />
          <Select
            style={{ width: 180 }}
            placeholder="Достоверность"
            allowClear
            options={RELIABILITY_OPTIONS}
            value={(item.reliability as Reliability) || undefined}
            onChange={(v) => setField(i, { reliability: v })}
          />
          <Input
            placeholder="Роль"
            value={item.role}
            onChange={(e) => setField(i, { role: e.target.value })}
            style={{ width: 140 }}
          />
          <Input
            placeholder="Заметка"
            value={item.note}
            onChange={(e) => setField(i, { note: e.target.value })}
            style={{ width: 200 }}
          />
          <MinusCircleOutlined onClick={() => remove(i)} />
        </Space>
      ))}
      <Button type="dashed" onClick={add} icon={<PlusOutlined />} block>
        {addLabel}
      </Button>
    </Space>
  );
}
```

### Шаг 3.4. Страницы Source

#### `web/src/pages/SourceForm.tsx` (создать)
`web/src/pages/SourceForm.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Alert, Checkbox, Form, Input, Modal, Select } from "antd";
import { createSource, fetchRepositories, type FactDate, type Reliability, type Repository, type Source, type TextRef } from "../api";
import { ApiError } from "../auth";
import { FactDateEditor } from "../FactDateEditor";
import { TextRefListEditor } from "../TextRefList";

const KIND_OPTIONS = [
  { value: "archival-scan", label: "скан из архива" },
  { value: "transcription", label: "расшифровка" },
  { value: "document", label: "документ" },
  { value: "audio", label: "аудио" },
  { value: "photo", label: "фото" },
  { value: "memory", label: "со слов/по памяти" },
  { value: "external", label: "внешний источник" },
];

const RELIABILITY_OPTIONS: { value: Reliability; label: string }[] = [
  { value: "primary", label: "Первичный" },
  { value: "contemporary", label: "Современник" },
  { value: "memory", label: "Со слов/по памяти" },
  { value: "indirect", label: "Косвенный" },
  { value: "unknown", label: "Неизвестна" },
];

interface SourceFormValues {
  kind: string;
  title: string;
  author?: string;
  reliability: Reliability;
  repository_id?: string;
  private?: boolean;
}

const FORM_FIELDS: (keyof SourceFormValues)[] = ["kind", "title", "author", "reliability", "repository_id"];

// useRepositoryOptions — заполняет Select хранилищ для repository_id, по
// образцу ArchiveForm.tsx (подпроект 3).
function useRepositoryOptions() {
  const [repositories, setRepositories] = useState<Repository[]>([]);

  useEffect(() => {
    fetchRepositories({ limit: 500 })
      .then(setRepositories)
      .catch(() => setRepositories([]));
  }, []);

  return repositories.map((r) => ({ value: r.id, label: r.name }));
}

// CreateSourceModal — форма создания источника. date — FactDateEditor (см.
// подпроект 3), repository_id — Select со списком хранилищ (мягкая ссылка).
export function CreateSourceModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Source) => void;
}) {
  const [form] = Form.useForm<SourceFormValues>();
  const [date, setDate] = useState<FactDate | null>(null);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const repositoryOptions = useRepositoryOptions();

  const reset = () => {
    form.resetFields();
    setDate(null);
    setNotes([]);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: SourceFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await createSource({
        kind: values.kind,
        title: values.title,
        author: values.author,
        date,
        reliability: values.reliability,
        repository_id: values.repository_id ?? "",
        notes,
        private: values.private ?? false,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof SourceFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить источник"
      open={open}
      onCancel={handleClose}
      onOk={() => form.submit()}
      okText="Создать"
      cancelText="Отмена"
      confirmLoading={submitting}
      destroyOnHidden
    >
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
      <Form form={form} layout="vertical" onFinish={onFinish} initialValues={{ kind: "document", reliability: "unknown", private: false }}>
        <Form.Item name="kind" label="Вид" rules={[{ required: true, message: "Выберите вид" }]}>
          <Select options={KIND_OPTIONS} />
        </Form.Item>
        <Form.Item
          name="title"
          label="Название"
          rules={[{ required: true, whitespace: true, message: "Введите название" }]}
        >
          <Input autoFocus />
        </Form.Item>
        <Form.Item name="author" label="Автор">
          <Input />
        </Form.Item>
        <Form.Item label="Дата">
          <FactDateEditor value={date} onChange={setDate} addLabel="+ дата" />
        </Form.Item>
        <Form.Item name="reliability" label="Достоверность" rules={[{ required: true, message: "Выберите достоверность" }]}>
          <Select options={RELIABILITY_OPTIONS} />
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

#### `web/src/pages/SourcesList.tsx` (создать)
`web/src/pages/SourcesList.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Input, List, Spin } from "antd";
import { fetchSources, searchSources, MAX_PAGE_LIMIT, type Source } from "../api";
import { useSession } from "../session";
import { CreateSourceModal } from "./SourceForm";

// SourcesList — «Источники»: плоский список, та же пагинация-до-короткой-
// страницы и поиск-подменяет-список, что у ArchivesList/NotesList.
export default function SourcesList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<Source[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<Source[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: Source[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchSources({ limit: MAX_PAGE_LIMIT, offset });
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
    searchSources({ q, limit: MAX_PAGE_LIMIT })
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
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "Источники" }]}
      />
      <Card
        title="Источники"
        extra={
          session != null ? (
            <Button type="primary" onClick={() => setCreateOpen(true)}>
              + добавить
            </Button>
          ) : undefined
        }
      >
        <Input.Search
          placeholder="Поиск по названию или автору…"
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
                <Link to={`/sources/${s.id}`}>{s.title}</Link>
              </List.Item>
            )}
          />
        )}
        <CreateSourceModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(s) => {
            setCreateOpen(false);
            navigate(`/sources/${s.id}`);
          }}
        />
      </Card>
    </>
  );
}
```

#### `web/src/pages/SourceView.tsx` (создать)
`web/src/pages/SourceView.tsx`:
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
  Select,
  Space,
  Spin,
  Typography,
} from "antd";
import {
  deleteSource,
  fetchRepositories,
  fetchSource,
  updateSource,
  type FactDate,
  type Reliability,
  type Repository,
  type Source,
  type TextRef,
} from "../api";
import { ApiError } from "../auth";
import { useSession } from "../session";
import { FactDateEditor, formatFactDate } from "../FactDateEditor";
import { TextRefListEditor } from "../TextRefList";

const KIND_OPTIONS = [
  { value: "archival-scan", label: "скан из архива" },
  { value: "transcription", label: "расшифровка" },
  { value: "document", label: "документ" },
  { value: "audio", label: "аудио" },
  { value: "photo", label: "фото" },
  { value: "memory", label: "со слов/по памяти" },
  { value: "external", label: "внешний источник" },
];

const RELIABILITY_OPTIONS: { value: Reliability; label: string }[] = [
  { value: "primary", label: "Первичный" },
  { value: "contemporary", label: "Современник" },
  { value: "memory", label: "Со слов/по памяти" },
  { value: "indirect", label: "Косвенный" },
  { value: "unknown", label: "Неизвестна" },
];

const RELIABILITY_LABELS: Record<string, string> = Object.fromEntries(
  RELIABILITY_OPTIONS.map((o) => [o.value, o.label]),
);

interface EditFormValues {
  kind: string;
  title: string;
  author?: string;
  reliability: Reliability;
  repository_id?: string;
  private?: boolean;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["kind", "title", "author", "reliability", "repository_id"];

// SourceView — просмотр источника, переключаемый в форму редактирования на
// той же странице (toggle+explicit-save), по образцу ArchiveView.tsx.
export default function SourceView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [source, setSource] = useState<Source | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [repositories, setRepositories] = useState<Repository[]>([]);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [date, setDate] = useState<FactDate | null>(null);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);

  const load = (sourceId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setSource(null);
    fetchSource(sourceId)
      .then(setSource)
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
  const repositoryName = (repoID?: string) => repositories.find((r) => r.id === repoID)?.name ?? repoID;

  const startEdit = () => {
    if (source == null) {
      return;
    }
    form.setFieldsValue({
      kind: source.kind,
      title: source.title,
      author: source.author ?? "",
      reliability: source.reliability,
      repository_id: source.repository_id ?? undefined,
      private: source.private,
    });
    setDate(source.date ?? null);
    setNotes(source.notes);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (source == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await updateSource(source.id, {
        kind: values.kind,
        title: values.title,
        author: values.author,
        date,
        reliability: values.reliability,
        repository_id: values.repository_id ?? "",
        notes,
        private: values.private ?? false,
      });
      setSource(updated);
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
    if (source == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteSource(source.id);
      navigate("/sources");
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Не удалось удалить запись");
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
          <Link to="/sources">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (source == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/sources">Источники</Link> },
          { title: source.title },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={source.title} column={1} bordered size="small">
            <Descriptions.Item label="Вид">{source.kind}</Descriptions.Item>
            <Descriptions.Item label="Автор">{source.author || "—"}</Descriptions.Item>
            <Descriptions.Item label="Дата">{formatFactDate(source.date)}</Descriptions.Item>
            <Descriptions.Item label="Достоверность">{RELIABILITY_LABELS[source.reliability] ?? source.reliability}</Descriptions.Item>
            <Descriptions.Item label="Хранилище">
              {source.repository_id ? (
                <Link to={`/repositories/${source.repository_id}`}>{repositoryName(source.repository_id)}</Link>
              ) : (
                "—"
              )}
            </Descriptions.Item>
            <Descriptions.Item label="Заметки">
              {source.notes.length === 0 ? (
                <Typography.Text type="secondary">—</Typography.Text>
              ) : (
                <Space direction="vertical" size={0}>
                  {source.notes.map((n, i) => (
                    <Typography.Text key={i}>{n.text}</Typography.Text>
                  ))}
                </Space>
              )}
            </Descriptions.Item>
            <Descriptions.Item label="Приватная">{source.private ? "да" : "нет"}</Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Button danger loading={deleting} onClick={onDelete}>
                Удалить
              </Button>
            </Space>
          )}
        </>
      ) : (
        <Form form={form} layout="vertical" onFinish={onSave} style={{ maxWidth: 520 }}>
          {saveError != null && (
            <Alert type="error" showIcon message={saveError} style={{ marginBottom: 16 }} />
          )}
          <Form.Item name="kind" label="Вид" rules={[{ required: true, message: "Выберите вид" }]}>
            <Select options={KIND_OPTIONS} />
          </Form.Item>
          <Form.Item
            name="title"
            label="Название"
            rules={[{ required: true, whitespace: true, message: "Введите название" }]}
          >
            <Input />
          </Form.Item>
          <Form.Item name="author" label="Автор">
            <Input />
          </Form.Item>
          <Form.Item label="Дата">
            <FactDateEditor value={date} onChange={setDate} addLabel="+ дата" />
          </Form.Item>
          <Form.Item name="reliability" label="Достоверность" rules={[{ required: true, message: "Выберите достоверность" }]}>
            <Select options={RELIABILITY_OPTIONS} />
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
    </Card>
  );
}
```

### Шаг 3.5. Страницы Citation

#### `web/src/pages/CitationForm.tsx` (создать)
`web/src/pages/CitationForm.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Alert, Checkbox, Form, Input, Modal, Select } from "antd";
import { createCitation, fetchSources, type Anchor, type Citation, type Source } from "../api";
import { ApiError } from "../auth";
import { AnchorEditor } from "../AnchorEditor";

interface CitationFormValues {
  source_id: string;
  text?: string;
  note?: string;
  private?: boolean;
}

const FORM_FIELDS: (keyof CitationFormValues)[] = ["source_id", "text", "note"];

// useSourceOptions — заполняет Select источников для source_id, по образцу
// useRepositoryOptions/ArchiveForm.tsx.
function useSourceOptions() {
  const [sources, setSources] = useState<Source[]>([]);

  useEffect(() => {
    fetchSources({ limit: 500 })
      .then(setSources)
      .catch(() => setSources([]));
  }, []);

  return sources.map((s) => ({ value: s.id, label: s.title }));
}

// CreateCitationModal — форма создания цитаты. anchor — AnchorEditor
// (полиморфная привязка, необязательна).
export function CreateCitationModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Citation) => void;
}) {
  const [form] = Form.useForm<CitationFormValues>();
  const [anchor, setAnchor] = useState<Anchor | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const sourceOptions = useSourceOptions();

  const reset = () => {
    form.resetFields();
    setAnchor(null);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: CitationFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await createCitation({
        source_id: values.source_id,
        anchor,
        text: values.text,
        note: values.note,
        private: values.private ?? false,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof CitationFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить цитату"
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
          name="source_id"
          label="Источник"
          rules={[{ required: true, message: "Выберите источник" }]}
        >
          <Select
            options={sourceOptions}
            showSearch
            filterOption={(input, option) =>
              (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
            }
          />
        </Form.Item>
        <Form.Item name="text" label="Текст выписки">
          <Input.TextArea rows={4} />
        </Form.Item>
        <Form.Item label="Привязка">
          <AnchorEditor value={anchor} onChange={setAnchor} addLabel="+ привязка" />
        </Form.Item>
        <Form.Item name="note" label="Заметка">
          <Input />
        </Form.Item>
        <Form.Item name="private" valuePropName="checked">
          <Checkbox>Приватная запись</Checkbox>
        </Form.Item>
      </Form>
    </Modal>
  );
}
```

#### `web/src/pages/CitationsList.tsx` (создать)
`web/src/pages/CitationsList.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Input, List, Spin } from "antd";
import { fetchCitations, searchCitations, MAX_PAGE_LIMIT, type Citation } from "../api";
import { useSession } from "../session";
import { CreateCitationModal } from "./CitationForm";

function citationLabel(c: Citation): string {
  return c.text || c.id;
}

// CitationsList — «Цитаты»: плоский список, та же пагинация-до-короткой-
// страницы и поиск-подменяет-список, что у ArchivesList/NotesList.
export default function CitationsList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<Citation[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<Citation[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: Citation[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchCitations({ limit: MAX_PAGE_LIMIT, offset });
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
    searchCitations({ q, limit: MAX_PAGE_LIMIT })
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
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "Цитаты" }]}
      />
      <Card
        title="Цитаты"
        extra={
          session != null ? (
            <Button type="primary" onClick={() => setCreateOpen(true)}>
              + добавить
            </Button>
          ) : undefined
        }
      >
        <Input.Search
          placeholder="Поиск по началу текста…"
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
            renderItem={(c) => (
              <List.Item>
                <Link to={`/citations/${c.id}`}>{citationLabel(c)}</Link>
              </List.Item>
            )}
          />
        )}
        <CreateCitationModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(c) => {
            setCreateOpen(false);
            navigate(`/citations/${c.id}`);
          }}
        />
      </Card>
    </>
  );
}
```

#### `web/src/pages/CitationView.tsx` (создать)
`web/src/pages/CitationView.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Checkbox, Descriptions, Form, Input, Select, Space, Spin, Typography } from "antd";
import { deleteCitation, fetchCitation, fetchSources, updateCitation, type Anchor, type Citation, type Source } from "../api";
import { ApiError } from "../auth";
import { useSession } from "../session";
import { AnchorEditor } from "../AnchorEditor";

interface EditFormValues {
  source_id: string;
  text?: string;
  note?: string;
  private?: boolean;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["source_id", "text", "note"];

function citationLabel(c: Citation): string {
  return c.text || c.id;
}

function anchorSummary(a: Anchor | null | undefined): string {
  if (a == null) {
    return "—";
  }
  if (a.kind === "archive") {
    return `архив: узел ${a.node_id ?? "—"}${a.document_id ? `, документ ${a.document_id}` : ""}, стр. ${a.page ?? "—"}`;
  }
  if (a.kind === "file") {
    return `файл: вложение ${a.attachment_id ?? "—"}${a.timecode ? `, ${a.timecode}` : ""}`;
  }
  if (a.kind === "url") {
    return a.url ?? "—";
  }
  return "—";
}

// CitationView — просмотр цитаты, переключаемый в форму редактирования на
// той же странице (toggle+explicit-save), по образцу ArchiveView.tsx.
export default function CitationView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [citation, setCitation] = useState<Citation | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sources, setSources] = useState<Source[]>([]);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [anchor, setAnchor] = useState<Anchor | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);

  const load = (citationId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setCitation(null);
    fetchCitation(citationId)
      .then(setCitation)
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
    fetchSources({ limit: 500 })
      .then(setSources)
      .catch(() => setSources([]));
  }, []);

  const sourceOptions = sources.map((s) => ({ value: s.id, label: s.title }));
  const sourceTitle = (sourceID?: string) => sources.find((s) => s.id === sourceID)?.title ?? sourceID;

  const startEdit = () => {
    if (citation == null) {
      return;
    }
    form.setFieldsValue({
      source_id: citation.source_id,
      text: citation.text ?? "",
      note: citation.note ?? "",
      private: citation.private,
    });
    setAnchor(citation.anchor ?? null);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (citation == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await updateCitation(citation.id, {
        source_id: values.source_id,
        anchor,
        text: values.text,
        note: values.note,
        private: values.private ?? false,
      });
      setCitation(updated);
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
    if (citation == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteCitation(citation.id);
      navigate("/citations");
    } catch (e) {
      setError(e instanceof ApiError ? e.message : "Не удалось удалить запись");
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
          <Link to="/citations">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (citation == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/citations">Цитаты</Link> },
          { title: citationLabel(citation) },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={citationLabel(citation)} column={1} bordered size="small">
            <Descriptions.Item label="Источник">
              <Link to={`/sources/${citation.source_id}`}>{sourceTitle(citation.source_id)}</Link>
            </Descriptions.Item>
            <Descriptions.Item label="Текст">
              <Typography.Paragraph style={{ whiteSpace: "pre-wrap", marginBottom: 0 }}>
                {citation.text || "—"}
              </Typography.Paragraph>
            </Descriptions.Item>
            <Descriptions.Item label="Привязка">{anchorSummary(citation.anchor)}</Descriptions.Item>
            <Descriptions.Item label="Заметка">{citation.note || "—"}</Descriptions.Item>
            <Descriptions.Item label="Приватная">{citation.private ? "да" : "нет"}</Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Button danger loading={deleting} onClick={onDelete}>
                Удалить
              </Button>
            </Space>
          )}
        </>
      ) : (
        <Form form={form} layout="vertical" onFinish={onSave} style={{ maxWidth: 520 }}>
          {saveError != null && (
            <Alert type="error" showIcon message={saveError} style={{ marginBottom: 16 }} />
          )}
          <Form.Item name="source_id" label="Источник" rules={[{ required: true, message: "Выберите источник" }]}>
            <Select
              options={sourceOptions}
              showSearch
              filterOption={(input, option) =>
                (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
              }
            />
          </Form.Item>
          <Form.Item name="text" label="Текст выписки">
            <Input.TextArea rows={4} />
          </Form.Item>
          <Form.Item label="Привязка">
            <AnchorEditor value={anchor} onChange={setAnchor} addLabel="+ привязка" />
          </Form.Item>
          <Form.Item name="note" label="Заметка">
            <Input />
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
    </Card>
  );
}
```

### Шаг 3.6. `web/src/App.tsx` — роуты, `EntityCatalog.tsx` — строки «Источники»/«Цитаты»

Добавить импорты в `web/src/App.tsx` (после импортов Attachment из подпроекта 4):
```typescript
import SourcesList from "./pages/SourcesList";
import SourceView from "./pages/SourceView";
import CitationsList from "./pages/CitationsList";
import CitationView from "./pages/CitationView";
```
Добавить роуты (после роутов `/attachments`/`/attachments/:id`, до `/login`):
```typescript
          <Route path="/sources" element={<PageLayout><SourcesList /></PageLayout>} />
          <Route path="/sources/:id" element={<PageLayout><SourceView /></PageLayout>} />
          <Route path="/citations" element={<PageLayout><CitationsList /></PageLayout>} />
          <Route path="/citations/:id" element={<PageLayout><CitationView /></PageLayout>} />
```

В массиве `CATALOG_ENTRIES` (`web/src/pages/EntityCatalog.tsx`) добавить:
```typescript
  { label: "Источники", path: "/sources" },
```
и (порядок в массиве по алфавиту, но не влияет на отображение — сортируется рантаймом):
```typescript
  { label: "Цитаты", path: "/citations" },
```

### Шаг 3.7. Рубеж

`npm run typecheck` и `npm run build` в `web/` чисты. Живая проверка в браузере (см. «Предпосылка»): `/sources` → «+ добавить» → `FactDateEditor` для даты → `Select` «Достоверность» → создание → View показывает дату и достоверность; `/citations` → «+ добавить» → `Select` «Источник» с поиском → `AnchorEditor`: переключение kind архив→файл→ссылка меняет набор полей → `kind=url` → создание → View показывает URL и источник кликабельной ссылкой.

### Шаг 3.8. Коммит

```bash
git add web/src/api.ts web/src/App.tsx web/src/pages/EntityCatalog.tsx \
  web/src/AnchorEditor.tsx web/src/SourceLinkList.tsx \
  web/src/pages/SourceForm.tsx web/src/pages/SourcesList.tsx web/src/pages/SourceView.tsx \
  web/src/pages/CitationForm.tsx web/src/pages/CitationsList.tsx web/src/pages/CitationView.tsx
git commit -m "feat(web): страницы Источников/Цитат — AnchorEditor, SourceLinkListEditor"
```

## Задача 4. Веб: редактирование Sources у 6 сущностей

**Интерфейсы, потребляемые из Задачи 2**: `httpapi`-маршруты `/api/{admin-divisions,repositories,churches,parishes,archives,notes}` теперь принимают `sources` в Create/Update-теле.
**Интерфейсы, потребляемые из Задачи 3**: `web/src/SourceLinkList.tsx`:`SourceLinkListEditor`, `web/src/api.ts`:`SourceLink`/`fetchCitations`.

**Файлы:**
- Изменить: `web/src/api.ts` (добавить поле `sources: SourceLink[]` в 6 `*Input`-интерфейсов: `AdminDivisionInput`, `RepositoryInput`, `ChurchInput`, `ParishInput`, `ArchiveInput`, `NoteInput`, и в read-интерфейс `AdminDivision` — бэкенд (`internal/transport/admin_division.go`) уже готов к этому с Шага 2.1, здесь меняется только TypeScript-сторона), `web/src/pages/{Division,Repository,Church,Parish,Archive,Note}{Form,View}.tsx`

**Важно для исполнителя**: `sources` в каждом `*Input`-интерфейсе ОБЯЗАТЕЛЬНОЕ поле (не `sources?:`) — TypeScript не даст собрать проект, если хотя бы один вызов `create<Entity>`/`update<Entity>` забудет его передать; это осознанно (тот же паттерн, что и `notes`/`private` — компилятор ловит забытые места сам). В каждой из 6 `*View.tsx`-страниц СУЩЕСТВУЕТ локальная read-only функция вида `SourceLinkListView` (использовалась для показа `sources` до этого прохода) — она НЕ трогается, используется как раньше в неактивном (не-`editing`) состоянии. Добавляется только редактируемая версия (`SourceLinkListEditor`) — активна в форме создания и в режиме `editing`. Единственное исключение — `DivisionView.tsx`: он вообще не показывал `sources` до этого прохода (см. находку в «Предпосылка»), здесь ему добавляется и read-only `Descriptions.Item`/`SourceLinkListView`, и редактируемая версия — по образцу того, что уже есть в `ArchiveView.tsx`.

### Шаг 4.1. `web/src/api.ts` — поле `sources` в 6 `*Input`-интерфейсах

В каждом из перечисленных интерфейсов добавить строку `sources: SourceLink[];` (после `notes`, если оно есть в интерфейсе, иначе — после последнего строкового/ссылочного поля, перед `private`, если оно есть). Итоговый вид каждого интерфейса — для сверки:

#### `AdminDivisionInput`
`web/src/api.ts (фрагмент — AdminDivisionInput)`:
```typescript
export interface AdminDivisionInput {
  name: string;
  type: AdminDivisionType;
  parent_id: string | null;
  sources: SourceLink[];
}
```

#### `RepositoryInput`
`web/src/api.ts (фрагмент — RepositoryInput)`:
```typescript
export interface RepositoryInput {
  name: string;
  type: string;
  address?: string;
  urls: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}
```

#### `ChurchInput`
`web/src/api.ts (фрагмент — ChurchInput)`:
```typescript
export interface ChurchInput {
  name: string;
  parish?: TextRef | null;
  settlements: TextRef[];
  variants: string[];
  notes: TextRef[];
  sources: SourceLink[];
}
```

#### `ParishInput`
`web/src/api.ts (фрагмент — ParishInput)`:
```typescript
export interface ParishInput {
  name: string;
  church?: TextRef | null;
  settlements: TextRef[];
  since?: FactDate | null;
  until?: FactDate | null;
  notes: TextRef[];
  sources: SourceLink[];
}
```

#### `ArchiveInput`
`web/src/api.ts (фрагмент — ArchiveInput)`:
```typescript
export interface ArchiveInput {
  name: string;
  system?: TextRef | null;
  repository_id?: string;
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}
```

#### `NoteInput`
`web/src/api.ts (фрагмент — NoteInput)`:
```typescript
export interface NoteInput {
  kind: string;
  title?: string;
  text?: string;
  parent_id?: string;
  sources: SourceLink[];
  private: boolean;
}
```

Также добавить поле `sources: SourceLink[];` в read-интерфейс `AdminDivision` (в самом начале файла) — итоговый вид:
`web/src/api.ts (фрагмент — AdminDivision)`:
```typescript
export interface AdminDivision {
  id: string;
  name: string;
  type: string;
  parent_id: string | null;
  sources: SourceLink[];
}
```

### Шаг 4.2. `AdministrativeDivision` — Form/View

#### `web/src/pages/DivisionForm.tsx` (изменить — итоговое содержимое)
`web/src/pages/DivisionForm.tsx`:
```tsx
import { useState } from "react";
import { Alert, Form, Input, Modal, Select } from "antd";
import {
  ADMIN_DIVISION_TYPE_LABELS,
  createDivision,
  type AdminDivision,
  type AdminDivisionType,
  type SourceLink,
} from "../api";
import { ApiError } from "../auth";
import { SourceLinkListEditor } from "../SourceLinkList";

export const TYPE_OPTIONS = (Object.keys(ADMIN_DIVISION_TYPE_LABELS) as AdminDivisionType[]).map(
  (value) => ({ value, label: ADMIN_DIVISION_TYPE_LABELS[value] }),
);

interface DivisionFormValues {
  name: string;
  type: AdminDivisionType;
}

const FORM_FIELDS: (keyof DivisionFormValues)[] = ["name", "type"];

// CreateDivisionModal — форма создания единицы, используется и на
// DivisionsList (parentId=null — корень) и на DivisionView (parentId —
// текущая единица, «добавить дочернюю»). Единица создаётся с фиксированным
// parent_id из пропа: сам parent_id полем формы не является.
export function CreateDivisionModal({
  open,
  parentId,
  onClose,
  onCreated,
}: {
  open: boolean;
  parentId: string | null;
  onClose: () => void;
  onCreated: (created: AdminDivision) => void;
}) {
  const [form] = Form.useForm<DivisionFormValues>();
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const handleClose = () => {
    form.resetFields();
    setSources([]);
    setError(null);
    onClose();
  };

  const onFinish = async (values: DivisionFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await createDivision({
        name: values.name,
        type: values.type,
        parent_id: parentId,
        sources,
      });
      form.resetFields();
      setSources([]);
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof DivisionFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать единицу");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title={parentId == null ? "Добавить в корень" : "Добавить дочернюю единицу"}
      open={open}
      onCancel={handleClose}
      onOk={() => form.submit()}
      okText="Создать"
      cancelText="Отмена"
      confirmLoading={submitting}
      destroyOnHidden
    >
      {error != null && (
        <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />
      )}
      <Form form={form} layout="vertical" onFinish={onFinish}>
        <Form.Item
          name="name"
          label="Название"
          rules={[{ required: true, whitespace: true, message: "Введите название" }]}
        >
          <Input autoFocus />
        </Form.Item>
        <Form.Item name="type" label="Тип" rules={[{ required: true, message: "Выберите тип" }]}>
          <Select options={TYPE_OPTIONS} placeholder="Выберите тип" />
        </Form.Item>
        <Form.Item label="Доказательства">
          <SourceLinkListEditor value={sources} onChange={setSources} addLabel="+ доказательство" />
        </Form.Item>
      </Form>
    </Modal>
  );
}
```

#### `web/src/pages/DivisionView.tsx` (изменить — итоговое содержимое)
`web/src/pages/DivisionView.tsx`:
```tsx
import { useEffect, useRef, useState } from "react";
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
  adminDivisionTypeLabel,
  deleteDivision,
  fetchAdminDivisions,
  fetchDivision,
  searchAdminDivisions,
  updateDivision,
  MAX_PAGE_LIMIT,
  type AdminDivision,
  type AdminDivisionType,
  type SourceLink,
} from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { SourceLinkListEditor } from "../SourceLinkList";
import { CreateDivisionModal, TYPE_OPTIONS } from "./DivisionForm";

function divisionLabel(d: AdminDivision): string {
  return `${d.name} (${adminDivisionTypeLabel(d.type)})`;
}

// SourceLinkListView — read-only список доказательств (Sources). До
// подпроекта 5 у Division он вообще не показывался (Sources были невидимы
// даже через GET); теперь бэкенд отдаёт их, и здесь — та же схема
// отображения, что и в ArchiveView/RepositoryView/….
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

interface EditFormValues {
  name: string;
  type: AdminDivisionType;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["name", "type"];

// DivisionView — просмотр единицы, переключаемый в форму редактирования
// (та же страница, без смены URL — решение: PUT заменяет
// name/type/parent_id разом, автосейв по полям не подходит). Дочерние
// единицы — список со ссылками на их View; «добавить дочернюю» открывает
// CreateDivisionModal с parentId = текущая единица. Удаление — Popconfirm,
// конфликт (409, единица занята) — модалка со списком ссылающихся.
export default function DivisionView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [division, setDivision] = useState<AdminDivision | null>(null);
  const [parent, setParent] = useState<AdminDivision | null>(null);
  const [children, setChildren] = useState<AdminDivision[]>([]);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  // editParentId — родитель, выбранный в форме редактирования (null — корень).
  const [editParentId, setEditParentId] = useState<string | null>(null);
  const [editParentLabel, setEditParentLabel] = useState<string | null>(null);
  const [parentPickerOpen, setParentPickerOpen] = useState(false);
  const [parentQuery, setParentQuery] = useState("");
  const [parentResults, setParentResults] = useState<AdminDivision[]>([]);
  const [parentSearching, setParentSearching] = useState(false);

  const [addChildOpen, setAddChildOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  // parentSearchSeq — защита от устаревших (out-of-order) ответов
  // onParentSearch: поздний ответ на уже неактуальный запрос не должен
  // перезаписать список результатов.
  const parentSearchSeq = useRef(0);

  const load = (divisionId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setDivision(null);
    setParent(null);
    setChildren([]);
    fetchDivision(divisionId)
      .then((d) => {
        setDivision(d);
        const parentPromise =
          d.parent_id != null
            ? fetchDivision(d.parent_id)
                .then(setParent)
                .catch(() => setParent(null))
            : Promise.resolve(setParent(null));
        const childrenPromise = fetchAdminDivisions({
          parent_id: divisionId,
          limit: MAX_PAGE_LIMIT,
        }).then(setChildren);
        return Promise.all([parentPromise, childrenPromise]);
      })
      .catch((e) => {
        if (e instanceof ApiError && e.status === 404) {
          setNotFound(true);
        } else {
          setError(e instanceof Error ? e.message : "Не удалось загрузить единицу");
        }
      })
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    setEditing(false);
    setParentPickerOpen(false);
    setSaveError(null);
    setParentQuery("");
    setParentResults([]);
    if (id != null) {
      load(id);
    }
  }, [id]);

  const startEdit = () => {
    if (division == null) {
      return;
    }
    form.setFieldsValue({ name: division.name, type: division.type as AdminDivisionType });
    setEditParentId(division.parent_id);
    setEditParentLabel(parent != null ? divisionLabel(parent) : null);
    setParentPickerOpen(false);
    setParentQuery("");
    setParentResults([]);
    setSources(division.sources);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setParentPickerOpen(false);
    setParentQuery("");
    setParentResults([]);
    setSaveError(null);
  };

  const onParentSearch = (value: string) => {
    setParentQuery(value);
    const q = value.trim();
    if (!q) {
      setParentResults([]);
      return;
    }
    const seq = ++parentSearchSeq.current;
    setParentSearching(true);
    searchAdminDivisions({ q, limit: 20 })
      .then((results) => {
        if (seq === parentSearchSeq.current) {
          setParentResults(results.filter((r) => r.id !== division?.id));
        }
      })
      .catch(() => {
        if (seq === parentSearchSeq.current) {
          setParentResults([]);
        }
      })
      .finally(() => {
        if (seq === parentSearchSeq.current) {
          setParentSearching(false);
        }
      });
  };

  const pickParent = (d: AdminDivision | null) => {
    setEditParentId(d?.id ?? null);
    setEditParentLabel(d != null ? divisionLabel(d) : null);
    setParentPickerOpen(false);
    setParentQuery("");
    setParentResults([]);
  };

  const onSave = async (values: EditFormValues) => {
    if (division == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await updateDivision(division.id, {
        name: values.name,
        type: values.type,
        parent_id: editParentId,
        sources,
      });
      setDivision(updated);
      if (updated.parent_id != null) {
        fetchDivision(updated.parent_id)
          .then(setParent)
          .catch(() => setParent(null));
      } else {
        setParent(null);
      }
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
    if (division == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteDivision(division.id);
      navigate(division.parent_id != null ? `/divisions/${division.parent_id}` : "/divisions");
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        setConflict(e.referrers ?? []);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось удалить единицу");
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
        message="Единица не найдена"
        description="Возможно, её удалили. Вернитесь к списку."
        action={
          <Link to="/divisions">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (division == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/divisions">Административное деление</Link> },
          ...(parent != null
            ? [{ title: <Link to={`/divisions/${parent.id}`}>{parent.name}</Link> }]
            : []),
          { title: division.name },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={division.name} column={1} bordered size="small">
            <Descriptions.Item label="Тип">{adminDivisionTypeLabel(division.type)}</Descriptions.Item>
            <Descriptions.Item label="Родитель">
              {parent != null ? (
                <Link to={`/divisions/${parent.id}`}>{parent.name}</Link>
              ) : (
                <Typography.Text type="secondary">корень</Typography.Text>
              )}
            </Descriptions.Item>
            <Descriptions.Item label="Доказательства"><SourceLinkListView items={division.sources} /></Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Button onClick={() => setAddChildOpen(true)}>+ добавить дочернюю</Button>
              <Popconfirm
                title={`Удалить «${division.name}»?`}
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
          <Form.Item name="type" label="Тип" rules={[{ required: true, message: "Выберите тип" }]}>
            <Select options={TYPE_OPTIONS} />
          </Form.Item>
          <Form.Item label="Родитель">
            <Space direction="vertical" style={{ width: "100%" }}>
              <Space>
                <Typography.Text>
                  {editParentLabel ?? <Typography.Text type="secondary">корень</Typography.Text>}
                </Typography.Text>
                <Button size="small" onClick={() => setParentPickerOpen((v) => !v)}>
                  Изменить
                </Button>
                {editParentId != null && (
                  <Button size="small" onClick={() => pickParent(null)}>
                    Сделать корневой
                  </Button>
                )}
              </Space>
              {parentPickerOpen && (
                <Card size="small">
                  <Input.Search
                    placeholder="Поиск родителя по названию…"
                    value={parentQuery}
                    onChange={(e) => onParentSearch(e.target.value)}
                    loading={parentSearching}
                    allowClear
                  />
                  <List
                    size="small"
                    dataSource={parentResults}
                    locale={{ emptyText: "Ничего не найдено" }}
                    renderItem={(d) => (
                      <List.Item style={{ cursor: "pointer" }} onClick={() => pickParent(d)}>
                        {divisionLabel(d)}
                      </List.Item>
                    )}
                  />
                </Card>
              )}
            </Space>
          </Form.Item>
          <Form.Item label="Доказательства">
            <SourceLinkListEditor value={sources} onChange={setSources} addLabel="+ доказательство" />
          </Form.Item>
          <Space>
            <Button type="primary" htmlType="submit" loading={saving}>
              Сохранить
            </Button>
            <Button onClick={cancelEdit}>Отмена</Button>
          </Space>
        </Form>
      )}

      <Typography.Title level={5} style={{ marginTop: 24 }}>
        Дочерние единицы
      </Typography.Title>
      <List
        dataSource={children}
        locale={{ emptyText: "Дочерних единиц нет" }}
        renderItem={(c) => (
          <List.Item>
            <Link to={`/divisions/${c.id}`}>{divisionLabel(c)}</Link>
          </List.Item>
        )}
      />

      <CreateDivisionModal
        open={addChildOpen}
        parentId={division.id}
        onClose={() => setAddChildOpen(false)}
        onCreated={() => {
          setAddChildOpen(false);
          fetchAdminDivisions({ parent_id: division.id, limit: MAX_PAGE_LIMIT })
            .then(setChildren)
            .catch(() => {});
        }}
      />

      <Modal
        title="Единица используется"
        open={conflict != null}
        onCancel={() => setConflict(null)}
        footer={<Button onClick={() => setConflict(null)}>Закрыть</Button>}
      >
        <Typography.Paragraph>
          Нельзя удалить — на единицу ссылаются другие сущности:
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

### Шаг 4.3. `Repository` — Form/View

#### `web/src/pages/RepositoryForm.tsx` (изменить — итоговое содержимое)
`web/src/pages/RepositoryForm.tsx`:
```tsx
import { useState } from "react";
import { Alert, Checkbox, Form, Input, Modal } from "antd";
import { createRepository, type Repository, type SourceLink, type TextRef } from "../api";
import { ApiError } from "../auth";
import { SourceLinkListEditor } from "../SourceLinkList";
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
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setUrls([]);
    setNotes([]);
    setSources([]);
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
        sources,
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

#### `web/src/pages/RepositoryView.tsx` (изменить — итоговое содержимое)
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
import { SourceLinkListEditor } from "../SourceLinkList";
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
  const [sources, setSources] = useState<SourceLink[]>([]);
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
    setSources(repository.sources);
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
        sources,
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

### Шаг 4.4. `Church` — Form/View

#### `web/src/pages/ChurchForm.tsx` (изменить — итоговое содержимое)
`web/src/pages/ChurchForm.tsx`:
```tsx
import { useState } from "react";
import { Alert, Form, Input, Modal } from "antd";
import { createChurch, type Church, type SourceLink, type TextRef } from "../api";
import { ApiError } from "../auth";
import { SourceLinkListEditor } from "../SourceLinkList";
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
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setSettlements([]);
    setVariants([]);
    setNotes([]);
    setSources([]);
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
        sources,
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
        <Form.Item label="Доказательства">
          <SourceLinkListEditor value={sources} onChange={setSources} addLabel="+ доказательство" />
        </Form.Item>
      </Form>
    </Modal>
  );
}
```

#### `web/src/pages/ChurchView.tsx` (изменить — итоговое содержимое)
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
import { SourceLinkListEditor } from "../SourceLinkList";
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
  const [sources, setSources] = useState<SourceLink[]>([]);
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
    setSources(church.sources);
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
        sources,
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
          <Form.Item label="Доказательства">
            <SourceLinkListEditor value={sources} onChange={setSources} addLabel="+ доказательство" />
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

### Шаг 4.5. `Parish` — Form/View

#### `web/src/pages/ParishForm.tsx` (изменить — итоговое содержимое)
`web/src/pages/ParishForm.tsx`:
```tsx
import { useState } from "react";
import { Alert, Form, Input, Modal } from "antd";
import { createParish, type FactDate, type Parish, type SourceLink, type TextRef } from "../api";
import { ApiError } from "../auth";
import { SourceLinkListEditor } from "../SourceLinkList";
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
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setSettlements([]);
    setSince(null);
    setUntil(null);
    setNotes([]);
    setSources([]);
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
        sources,
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
        <Form.Item label="Доказательства">
          <SourceLinkListEditor value={sources} onChange={setSources} addLabel="+ доказательство" />
        </Form.Item>
      </Form>
    </Modal>
  );
}
```

#### `web/src/pages/ParishView.tsx` (изменить — итоговое содержимое)
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
import { SourceLinkListEditor } from "../SourceLinkList";
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
  const [sources, setSources] = useState<SourceLink[]>([]);
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
    setSources(parish.sources);
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
        sources,
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
          <Form.Item label="Доказательства">
            <SourceLinkListEditor value={sources} onChange={setSources} addLabel="+ доказательство" />
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

### Шаг 4.6. `Archive` — Form/View

#### `web/src/pages/ArchiveForm.tsx` (изменить — итоговое содержимое)
`web/src/pages/ArchiveForm.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Alert, Checkbox, Form, Input, Modal, Select } from "antd";
import {
  createArchive,
  fetchRepositories,
  type Archive,
  type Repository,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError } from "../auth";
import { SourceLinkListEditor } from "../SourceLinkList";
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
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const repositoryOptions = useRepositoryOptions();

  const reset = () => {
    form.resetFields();
    setNotes([]);
    setSources([]);
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
        sources,
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

#### `web/src/pages/ArchiveView.tsx` (изменить — итоговое содержимое)
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
import { SourceLinkListEditor } from "../SourceLinkList";
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
  const [sources, setSources] = useState<SourceLink[]>([]);
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
    setSources(archive.sources);
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
        sources,
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

### Шаг 4.7. `Note` — Form/View

#### `web/src/pages/NoteForm.tsx` (изменить — итоговое содержимое)
`web/src/pages/NoteForm.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Alert, Checkbox, Form, Input, Modal, Select } from "antd";
import { createNote, fetchNotes, type Note, type SourceLink } from "../api";
import { ApiError } from "../auth";
import { SourceLinkListEditor } from "../SourceLinkList";

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
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const noteOptions = useNoteOptions();

  const reset = () => {
    form.resetFields();
    setSources([]);
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
        sources,
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

#### `web/src/pages/NoteView.tsx` (изменить — итоговое содержимое)
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
import { SourceLinkListEditor } from "../SourceLinkList";

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
  const [sources, setSources] = useState<SourceLink[]>([]);
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
    setSources(note.sources);
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
        sources,
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

### Шаг 4.8. Рубеж

`npm run typecheck` и `npm run build` в `web/` чисты. Живая проверка в браузере (см. «Предпосылка»): `/archives` → «+ добавить» → секция «Доказательства» → `Select` «Цитата» с поиском → создание → View показывает `citation ...` в «Доказательства»; то же для `/divisions` (раньше секция не показывалась вовсе) — «+ добавить в корень» → «Доказательства» → создание → View показывает «Доказательства» как новую строку в `Descriptions`.

### Шаг 4.9. Коммит

```bash
git add web/src/api.ts \
  web/src/pages/DivisionForm.tsx web/src/pages/DivisionView.tsx \
  web/src/pages/RepositoryForm.tsx web/src/pages/RepositoryView.tsx \
  web/src/pages/ChurchForm.tsx web/src/pages/ChurchView.tsx \
  web/src/pages/ParishForm.tsx web/src/pages/ParishView.tsx \
  web/src/pages/ArchiveForm.tsx web/src/pages/ArchiveView.tsx \
  web/src/pages/NoteForm.tsx web/src/pages/NoteView.tsx
git commit -m "feat(web): редактирование Sources у 6 сущностей (Административное деление/Хранилища/Церкви/Приходы/Архивы/Заметки)"
```

## Рубеж прохода

После Задачи 4: `gofmt -l .` пусто, `go build/vet/test ./...` зелёные (1093
теста, 100 пакетов), `npm run typecheck`/`build` чисты. Каталог `/` — 15
строк по алфавиту. `Source` и `Citation` имеют полный CRUD через HTTP, MCP и
веб, по конвенциям `docs/data-model/entity-write.md` §3-4, с первым
полиморфным типом в программе (`Anchor`) и первым MCP-аргументом
«массив объектов» (`sources`). `Sources []SourceLink` редактируется у всех
6 сущностей, где есть (`AdministrativeDivision`, `Repository`, `Church`,
`Parish`, `Archive`, `Note`), с проверкой существования каждой цитаты в той
же транзакции, что и сохранение. Следующий подпроект — 6 (`ArchiveNode`
— переиспользование tree-паттерна `DivisionsList`/`DivisionView`,
`ArchiveDocument` — picker в это дерево); после него `Attachment.NodeID`/
`DocumentID` и `Citation.Anchor`'s `ArchiveAnchor`-вариант впервые получат
настоящий picker вместо текстовых полей.

## Коммиты

Четыре коммита в `main`, по одному на задачу — см. Шаги 1.8, 2.11, 3.8, 4.9.
