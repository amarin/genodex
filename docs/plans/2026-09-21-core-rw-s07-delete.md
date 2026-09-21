# S7: `Delete*` — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Порт `store.Store` получает `Delete<Entity>(ctx, id) error` для всех 21 сущностей: удаляет сущность, её связные строки, записи `search_index` и `source_links` и осиротевшие value-строки; при строгих ссылках на сущность возвращает `*models.InUseError` со списком ссылающихся (до 20), для отсутствующей — `models.ErrNotFound`.

**Architecture:** Модель ошибки (`InUseError`, `EntityRef`, `MaxReferrers`) — в листовом `models`. В `sqlstore` вводится граф внешних ключей схемы (`fkgraph.go`, один запрос по `sqlite_master` + `pragma_foreign_key_list`, кэшируется в `Store`): по нему определяются каскадные связные таблицы, value-колонки (`text_refs`/`dates`/`anchors`) и строгие ссылки на сущность — ничего не перечисляется вручную, новая связная таблица учитывается автоматически. Общая реализация `deleteEntity` (`delete.go`) в одной транзакции: проверка существования → поиск ссылающихся (`InUseError`) → сбор value-id → чистка `source_links`/`search_index` → `DELETE` главной строки (каскад) → удаление value-строк. 21 метод порта — однострочные обёртки.

**Tech Stack:** Go 1.26.4, `database/sql`, `modernc.org/sqlite`, `go.uber.org/mock`.

**Spec:** `docs/data-model/core-read-write.md` §4; `docs/plans/2026-09-20-core-rw-roadmap.md` (этап S7).

## Global Constraints

- Порт: `Delete<E>(ctx context.Context, id models.ID) error` для всех 21 сущностей; `ctx` первый.
- Отсутствующая сущность → `models.ErrNotFound`. Строгие ссылки (FK `ON DELETE RESTRICT` на таблицу сущности) → `*models.InUseError{Type, ID, Referrers}`; `Referrers` — до `models.MaxReferrers` (20) сущностей, без повторов, в стабильном порядке; ничего не удаляется. Ссылка сущности на саму себя не считается. Ссылки `ON DELETE SET NULL` (например, `attachments.document_id`) удаление не блокируют и обнуляются.
- Ссылающийся — сущность-владелец: для связной таблицы (`event_participants.person_id`) это её владелец (`Event`); для `source_links.citation_id` — цель доказательства (`target_type`/`target_id`).
- Удаление чистит: связные строки (каскад), `search_index` (`entity_table` = таблица главной строки), `source_links` по `(target_type, target_id)`, value-строки `text_refs`/`dates`/`anchors`, на которые ссылались главная и связные строки. Счётчики строк всех таблиц после удаления равны значениям до сохранения сущности.
- Отменённый `ctx` — ошибка контекста, сущность остаётся; сбои хранилища оборачиваются `delete <тип> "<id>": %w`, `ErrNotFound` и `*InUseError` возвращаются как есть.
- Соединение одно (`SetMaxOpenConns(1)`): запросы внутри транзакции идут только через `tx runner`; граф схемы загружается до начала транзакции.
- Схема БД, `Validate`, контракты MCP/HTTP не меняются. Внутренние `Tx/Exec/...` `storage.DB` не трогаем.
- Комментарии и тексты ошибок — на русском; код gofmt-clean.
- Проверка рубежа: `gofmt -l .` пусто, `go build ./...`, `go vet ./...`, `go test ./...`.

---

### Task 1: `InUseError`, `EntityRef`, `MaxReferrers`

**Files:**
- Modify: `internal/models/errors.go`, `internal/models/errors_test.go`

**Interfaces:**
- Consumes: `models.Type`, `models.ID`, существующий `ErrNotFound`.
- Produces: `const MaxReferrers = 20`; `type EntityRef struct{ Type Type; ID ID }`; `type InUseError struct{ Type Type; ID ID; Referrers []EntityRef }` с методом `Error() string` (указатель).

- [ ] **Step 1: Тест (падает — символов нет)**

В `internal/models/errors_test.go` добавить `"strings"` в import и дописать в конец файла:

```go
func TestInUseErrorMatchesAsAndNamesReferrers(t *testing.T) {
	err := fmt.Errorf("delete: %w", &InUseError{
		Type: TypePerson, ID: "I-1",
		Referrers: []EntityRef{{Type: TypeRelation, ID: "RL-1"}, {Type: TypeEvent, ID: "E-1"}},
	})

	var inUse *InUseError
	if !errors.As(err, &inUse) {
		t.Fatal("errors.As не находит *InUseError в обёрнутой ошибке")
	}

	for _, want := range []string{`person "I-1"`, "relation RL-1", "event E-1"} {
		if !strings.Contains(inUse.Error(), want) {
			t.Errorf("текст ошибки %q не содержит %q", inUse.Error(), want)
		}
	}

	if MaxReferrers != 20 {
		t.Errorf("MaxReferrers = %d, ожидалось 20", MaxReferrers)
	}
}
```

- [ ] **Step 2: Убедиться, что не компилируется**

Run: `go test ./internal/models/`
Expected: FAIL — `undefined: InUseError`.

- [ ] **Step 3: Реализация**

Заменить содержимое `internal/models/errors.go` на:

```go
package models

import (
	"errors"
	"fmt"
	"strings"
)

// ErrNotFound — запрошенной сущности нет. Get-методы порта возвращают её
// (проверять через errors.Is); обработчики превращают в «не найдено».
var ErrNotFound = errors.New("не найдено")

// MaxReferrers — предел списка ссылающихся сущностей в InUseError.
const MaxReferrers = 20

// EntityRef — ссылка на сущность: тип и идентификатор.
type EntityRef struct {
	Type Type
	ID   ID
}

// InUseError — удаление запрещено: на сущность ссылаются другие. Referrers —
// первые MaxReferrers ссылающихся в стабильном порядке, без повторов.
// Обработчики находят её через errors.As и превращают в конфликт (409).
type InUseError struct {
	Type      Type
	ID        ID
	Referrers []EntityRef
}

func (e *InUseError) Error() string {
	refs := make([]string, len(e.Referrers))
	for i, r := range e.Referrers {
		refs[i] = string(r.Type) + " " + string(r.ID)
	}

	return fmt.Sprintf("%s %q используется: %s", e.Type, e.ID, strings.Join(refs, ", "))
}
```

- [ ] **Step 4: Тесты проходят**

Run: `gofmt -l internal && go test ./internal/models/`
Expected: gofmt пусто, `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/models/errors.go internal/models/errors_test.go
git commit -m "feat(models): InUseError, EntityRef, MaxReferrers"
```

---

### Task 2: Порт, граф схемы, `deleteEntity`

**Files:**
- Create: `internal/store/sqlstore/fkgraph.go`, `internal/store/sqlstore/fkgraph_test.go`, `internal/store/sqlstore/delete.go`
- Modify: `internal/store/deps.go`, `internal/store/deps_test.go` (генерируется), `internal/store/sqlstore/sqlstore.go`

**Interfaces:**
- Consumes: Task 1; `runner`, `(*Store).run/inTx`, `scanRows`, `collectIDs`, `deleteValues`, `notFound` (S6 и ранее); `models.AllTypes()`.
- Produces: порт с 21 методом `Delete*`; `entityTables`/`typeOfTable`, `valueTables`, `schemaGraph` (`cascadeChildren`, `restrictInto`, `owner`, `valueCols`), `(*Store).graph(ctx)`; `deleteEntity(ctx, typ, id)`, `collectOwnedValues`, `referrers`; тестовый хелпер `scanRowsStrings(s, query)`.

Код ниже проверен на чистой копии `main` (после него `go build`, `go vet`, все тесты зелёные).

- [ ] **Step 1: Создать файлы**

Создать `internal/store/sqlstore/fkgraph.go`:

```go
package sqlstore

import (
	"context"
	"database/sql"
	"sort"

	"github.com/amarin/genodex/internal/models"
)

// Граф внешних ключей схемы. Удаление сущности и поиск ссылающихся строятся по
// нему, а не по ручному списку таблиц: новая связная таблица или новая строгая
// ссылка учитываются автоматически, как и в storage.createFKIndexes.

// entityTables — таблица главной строки каждой сущности порта.
var entityTables = map[models.Type]string{
	models.TypePerson:                 "persons",
	models.TypeRelation:               "relations",
	models.TypeResidence:              "residences",
	models.TypeFamily:                 "families",
	models.TypeSurname:                "surnames",
	models.TypeGivenName:              "given_names",
	models.TypePatronymic:             "patronymics",
	models.TypeEstate:                 "estates",
	models.TypeTitle:                  "titles",
	models.TypeAdministrativeDivision: "administrative_divisions",
	models.TypeChurch:                 "churches",
	models.TypeParish:                 "parishes",
	models.TypeEvent:                  "events",
	models.TypeSource:                 "sources",
	models.TypeArchive:                "archives",
	models.TypeArchiveNode:            "archive_nodes",
	models.TypeArchiveDocument:        "archive_documents",
	models.TypeAttachment:             "attachments",
	models.TypeCitation:               "citations",
	models.TypeNote:                   "notes",
	models.TypeRepository:             "repositories",
}

// typeOfTable — обратное отображение: таблица главной строки → тип сущности.
var typeOfTable = func() map[string]models.Type {
	m := make(map[string]models.Type, len(entityTables))
	for t, table := range entityTables {
		m[table] = t
	}

	return m
}()

// valueTables — общие таблицы значений: строки принадлежат одной строке-владельцу
// и удаляются вместе с ней (см. очистку сирот в helpers.go).
var valueTables = []string{"text_refs", "dates", "anchors"}

// fkEdge — один внешний ключ: колонка child.col ссылается на parent.
type fkEdge struct {
	child, col, parent, onDelete string
}

// schemaGraph — все внешние ключи схемы в стабильном порядке.
type schemaGraph struct {
	edges []fkEdge
}

// loadSchemaGraph читает внешние ключи всех таблиц одним запросом.
func loadSchemaGraph(ctx context.Context, s *Store) (*schemaGraph, error) {
	edges, err := scanRows(s.run(ctx), func(r *sql.Rows) (fkEdge, error) {
		var e fkEdge
		err := r.Scan(&e.child, &e.col, &e.parent, &e.onDelete)

		return e, err
	}, `SELECT m.name, f."from", f."table", f.on_delete
		FROM sqlite_master AS m, pragma_foreign_key_list(m.name) AS f
		WHERE m.type = 'table' AND m.name NOT LIKE 'sqlite_%'
		ORDER BY m.name, f.id, f.seq`)
	if err != nil {
		return nil, err
	}

	return &schemaGraph{edges: edges}, nil
}

// graph возвращает граф схемы; результат кэшируется, ошибка — нет (отменённый
// контекст не должен «отравить» хранилище).
func (s *Store) graph(ctx context.Context) (*schemaGraph, error) {
	s.graphMu.Lock()
	defer s.graphMu.Unlock()

	if s.schema != nil {
		return s.schema, nil
	}

	g, err := loadSchemaGraph(ctx, s)
	if err != nil {
		return nil, err
	}

	s.schema = g

	return g, nil
}

// cascadeChildren — связные таблицы, каскадно удаляемые вместе с родителем.
func (g *schemaGraph) cascadeChildren(parent string) []fkEdge {
	return g.filter(func(e fkEdge) bool { return e.parent == parent && e.onDelete == "CASCADE" })
}

// restrictInto — строгие ссылки на таблицу (запрещают удаление строки-цели).
func (g *schemaGraph) restrictInto(parent string) []fkEdge {
	return g.filter(func(e fkEdge) bool { return e.parent == parent && e.onDelete == "RESTRICT" })
}

// owner возвращает каскадную ссылку связной таблицы на её владельца.
func (g *schemaGraph) owner(child string) (fkEdge, bool) {
	for _, e := range g.edges {
		if e.child == child && e.onDelete == "CASCADE" {
			return e, true
		}
	}

	return fkEdge{}, false
}

// valueCols группирует колонки таблицы, ссылающиеся на общие value-таблицы:
// value-таблица → колонки. Порядок таблиц — как в valueTables.
func (g *schemaGraph) valueCols(table string) map[string][]string {
	out := map[string][]string{}

	for _, e := range g.edges {
		if e.child != table {
			continue
		}

		for _, v := range valueTables {
			if e.parent == v {
				out[v] = append(out[v], e.col)
			}
		}
	}

	return out
}

func (g *schemaGraph) filter(keep func(fkEdge) bool) []fkEdge {
	var out []fkEdge

	for _, e := range g.edges {
		if keep(e) {
			out = append(out, e)
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].child != out[j].child {
			return out[i].child < out[j].child
		}

		return out[i].col < out[j].col
	})

	return out
}
```

Создать `internal/store/sqlstore/delete.go`:

```go
package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// deleteEntity удаляет сущность вместе с её связными строками, записями
// search_index и source_links и осиротевшими value-строками.
//
//   - Нет такой сущности — models.ErrNotFound.
//   - На сущность ссылаются другие (строгие FK) — *models.InUseError со списком
//     ссылающихся; ничего не удаляется. FK RESTRICT остаётся страховкой.
//   - Ссылки ON DELETE SET NULL (например, attachments.document_id) обнуляются.
//
// Устройство схемы (связные таблицы, value-колонки, ссылающиеся строки) берётся
// из графа внешних ключей, см. fkgraph.go.
func (s *Store) deleteEntity(ctx context.Context, typ models.Type, id models.ID) (err error) {
	table, ok := entityTables[typ]
	if !ok {
		return fmt.Errorf("delete: неизвестный тип %q", typ)
	}

	defer func() {
		// ErrNotFound и *InUseError несут смысл сами, остальное — сбой хранилища.
		if err == nil || err == models.ErrNotFound {
			return
		}

		if _, inUse := err.(*models.InUseError); !inUse {
			err = fmt.Errorf("delete %s %q: %w", typ, string(id), err)
		}
	}()

	g, err := s.graph(ctx)
	if err != nil {
		return err
	}

	return s.inTx(ctx, func(tx runner) error {
		var exists int
		if err := tx.QueryRow(`SELECT 1 FROM `+table+` WHERE id = ?`, string(id)).Scan(&exists); err != nil {
			if notFound(err) {
				return models.ErrNotFound
			}

			return err
		}

		refs, err := referrers(tx, g, typ, id, models.MaxReferrers)
		if err != nil {
			return err
		}

		if len(refs) > 0 {
			return &models.InUseError{Type: typ, ID: id, Referrers: refs}
		}

		values, err := collectOwnedValues(tx, g, table, string(id))
		if err != nil {
			return err
		}

		if _, err := tx.Exec(
			`DELETE FROM source_links WHERE target_type = ? AND target_id = ?`, string(typ), string(id),
		); err != nil {
			return err
		}

		if _, err := tx.Exec(
			`DELETE FROM search_index WHERE entity_table = ? AND entity_id = ?`, table, string(id),
		); err != nil {
			return err
		}

		// каскад сносит связные строки; value-строки на них ссылались, поэтому
		// удаляются только после главной строки.
		if _, err := tx.Exec(`DELETE FROM `+table+` WHERE id = ?`, string(id)); err != nil {
			return err
		}

		for _, v := range valueTables {
			if err := deleteValues(tx, v, values[v]); err != nil {
				return err
			}
		}

		return nil
	})
}

// collectOwnedValues собирает id value-строк (text_refs/dates/anchors), которыми
// владеют главная строка сущности и её связные строки.
func collectOwnedValues(tx runner, g *schemaGraph, table, id string) (map[string][]int64, error) {
	out := map[string][]int64{}

	collect := func(from, ownerCol string) error {
		for v, cols := range g.valueCols(from) {
			ids, err := collectIDs(tx,
				`SELECT `+strings.Join(cols, ", ")+` FROM `+from+` WHERE `+ownerCol+` = ?`, id)
			if err != nil {
				return err
			}

			out[v] = append(out[v], ids...)
		}

		return nil
	}

	if err := collect(table, "id"); err != nil {
		return nil, err
	}

	for _, e := range g.cascadeChildren(table) {
		if err := collect(e.child, e.col); err != nil {
			return nil, err
		}
	}

	return out, nil
}

// referrers возвращает до limit сущностей, строго ссылающихся на (typ, id): без
// повторов, в стабильном порядке. Ссылка из самой сущности на себя не считается.
// Запрос каждой ссылки отдаёт пары (тип, id) ссылающейся сущности.
func referrers(tx runner, g *schemaGraph, typ models.Type, id models.ID, limit int) ([]models.EntityRef, error) {
	var (
		out  []models.EntityRef
		seen = map[models.EntityRef]bool{}
	)

	for _, e := range g.restrictInto(entityTables[typ]) {
		if len(out) >= limit {
			break
		}

		var (
			query string
			args  []any
		)

		switch owner, isChild := g.owner(e.child); {
		case e.child == "source_links":
			// полиморфная связь «утверждение → цитата»: ссылающийся — цель ссылки
			query = `SELECT target_type, target_id FROM source_links WHERE citation_id = ?
			         ORDER BY target_type, target_id LIMIT ?`
			args = []any{string(id), limit}
		case isChild:
			// связная таблица: ссылающийся — её владелец
			query = `SELECT DISTINCT ?, ` + owner.col + ` FROM ` + e.child +
				` WHERE ` + e.col + ` = ? ORDER BY ` + owner.col + ` LIMIT ?`
			args = []any{string(typeOfTable[owner.parent]), string(id), limit}
		default:
			// главная таблица другой сущности (или этой же — тогда без самой себя)
			query = `SELECT ?, id FROM ` + e.child + ` WHERE ` + e.col + ` = ? AND id <> ? ORDER BY id LIMIT ?`
			args = []any{string(typeOfTable[e.child]), string(id), string(id), limit}
		}

		pairs, err := scanRows(tx, func(r *sql.Rows) (models.EntityRef, error) {
			var t, i string
			err := r.Scan(&t, &i)

			return models.EntityRef{Type: models.Type(t), ID: models.ID(i)}, err
		}, query, args...)
		if err != nil {
			return nil, err
		}

		for _, ref := range pairs {
			if !seen[ref] && len(out) < limit {
				seen[ref] = true
				out = append(out, ref)
			}
		}
	}

	return out, nil
}

// --- порт: Delete* ------------------------------------------------------

// DeletePerson удаляет сущность; см. deleteEntity.
func (s *Store) DeletePerson(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypePerson, id)
}

// DeleteRelation удаляет сущность; см. deleteEntity.
func (s *Store) DeleteRelation(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeRelation, id)
}

// DeleteResidence удаляет сущность; см. deleteEntity.
func (s *Store) DeleteResidence(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeResidence, id)
}

// DeleteFamily удаляет сущность; см. deleteEntity.
func (s *Store) DeleteFamily(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeFamily, id)
}

// DeleteSurname удаляет сущность; см. deleteEntity.
func (s *Store) DeleteSurname(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeSurname, id)
}

// DeleteGivenName удаляет сущность; см. deleteEntity.
func (s *Store) DeleteGivenName(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeGivenName, id)
}

// DeletePatronymic удаляет сущность; см. deleteEntity.
func (s *Store) DeletePatronymic(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypePatronymic, id)
}

// DeleteEstate удаляет сущность; см. deleteEntity.
func (s *Store) DeleteEstate(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeEstate, id)
}

// DeleteTitle удаляет сущность; см. deleteEntity.
func (s *Store) DeleteTitle(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeTitle, id)
}

// DeleteAdministrativeDivision удаляет сущность; см. deleteEntity.
func (s *Store) DeleteAdministrativeDivision(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeAdministrativeDivision, id)
}

// DeleteChurch удаляет сущность; см. deleteEntity.
func (s *Store) DeleteChurch(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeChurch, id)
}

// DeleteParish удаляет сущность; см. deleteEntity.
func (s *Store) DeleteParish(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeParish, id)
}

// DeleteEvent удаляет сущность; см. deleteEntity.
func (s *Store) DeleteEvent(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeEvent, id)
}

// DeleteSource удаляет сущность; см. deleteEntity.
func (s *Store) DeleteSource(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeSource, id)
}

// DeleteArchive удаляет сущность; см. deleteEntity.
func (s *Store) DeleteArchive(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeArchive, id)
}

// DeleteArchiveNode удаляет сущность; см. deleteEntity.
func (s *Store) DeleteArchiveNode(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeArchiveNode, id)
}

// DeleteArchiveDocument удаляет сущность; см. deleteEntity.
func (s *Store) DeleteArchiveDocument(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeArchiveDocument, id)
}

// DeleteAttachment удаляет сущность; см. deleteEntity.
func (s *Store) DeleteAttachment(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeAttachment, id)
}

// DeleteCitation удаляет сущность; см. deleteEntity.
func (s *Store) DeleteCitation(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeCitation, id)
}

// DeleteNote удаляет сущность; см. deleteEntity.
func (s *Store) DeleteNote(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeNote, id)
}

// DeleteRepository удаляет сущность; см. deleteEntity.
func (s *Store) DeleteRepository(ctx context.Context, id models.ID) error {
	return s.deleteEntity(ctx, models.TypeRepository, id)
}
```

Создать `internal/store/sqlstore/fkgraph_test.go`:

```go
package sqlstore

import (
	"context"
	"database/sql"
	"sort"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

// serviceTables — таблицы, не являющиеся ни сущностью, ни связной таблицей.
var serviceTables = map[string]bool{
	"meta": true, "dates": true, "text_refs": true, "anchors": true,
	"source_links": true, "search_index": true,
}

// TestEntityTablesMatchSchema: entityTables описывает ровно корневые таблицы
// схемы (не связные и не служебные), по одной на каждый тип модели.
func TestEntityTablesMatchSchema(t *testing.T) {
	s := newStore(t)

	g, err := s.graph(t.Context())
	if err != nil {
		t.Fatalf("graph: %v", err)
	}

	tables, err := scanRowsStrings(s, `SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		t.Fatalf("tables: %v", err)
	}

	var roots []string

	for _, table := range tables {
		if _, isChild := g.owner(table); !isChild && !serviceTables[table] {
			roots = append(roots, table)
		}
	}

	var mapped []string
	for _, table := range entityTables {
		mapped = append(mapped, table)
	}

	sort.Strings(roots)
	sort.Strings(mapped)

	if len(roots) != len(mapped) {
		t.Fatalf("корневые таблицы схемы %v, entityTables %v", roots, mapped)
	}

	for i := range roots {
		if roots[i] != mapped[i] {
			t.Fatalf("корневые таблицы схемы %v, entityTables %v", roots, mapped)
		}
	}

	for _, typ := range models.AllTypes() {
		if _, ok := entityTables[typ]; !ok {
			t.Errorf("тип %q не отображён на таблицу", typ)
		}
	}

	if len(entityTables) != len(models.AllTypes()) {
		t.Errorf("entityTables: %d записей, типов модели: %d", len(entityTables), len(models.AllTypes()))
	}
}

// TestSchemaGraphShapeIsSupported: deleteEntity рассчитан на схему, где связные
// таблицы лежат на один уровень под сущностью, а строгие ссылки идут только из
// сущностей, связных таблиц и source_links. Тест падает, если схема вышла за
// эти рамки, — тогда deleteEntity/referrers надо расширить.
func TestSchemaGraphShapeIsSupported(t *testing.T) {
	s := newStore(t)

	g, err := s.graph(t.Context())
	if err != nil {
		t.Fatalf("graph: %v", err)
	}

	for _, e := range g.edges {
		if e.onDelete == "CASCADE" {
			if _, nested := g.owner(e.parent); nested {
				t.Errorf("%s.%s каскадом зависит от %s, у которой сам есть владелец", e.child, e.col, e.parent)
			}

			if _, ok := typeOfTable[e.parent]; !ok {
				t.Errorf("%s.%s каскадом зависит от %s — это не таблица сущности", e.child, e.col, e.parent)
			}
		}

		if e.onDelete != "RESTRICT" {
			continue
		}

		if _, isEntityTarget := typeOfTable[e.parent]; !isEntityTarget {
			continue // ссылки на value-таблицы не блокируют удаление сущностей
		}

		_, isChild := g.owner(e.child)
		_, isEntity := typeOfTable[e.child]

		if !isChild && !isEntity && e.child != "source_links" {
			t.Errorf("строгая ссылка %s.%s → %s: источник не сущность, не связная таблица и не source_links",
				e.child, e.col, e.parent)
		}
	}
}

// scanRowsStrings читает однострочные строковые результаты запроса.
func scanRowsStrings(s *Store, query string) ([]string, error) {
	return scanRows(s.run(context.Background()), func(r *sql.Rows) (string, error) {
		var v string
		err := r.Scan(&v)

		return v, err
	}, query)
}
```

- [ ] **Step 2: Правки порта и структуры `Store`**

Сохранить как `$TMPDIR/s7-edit.py` (вне репозитория) и выполнить из корня: `python3 $TMPDIR/s7-edit.py` (при несовпадении скрипт останавливается с сообщением — не подгонять молча, разобраться):

```python
"""S7, шаг задачи 2: порт и структура Store. Запуск из корня репозитория."""
import re


def read(p):
    return open(p, encoding="utf-8").read()


def write(p, s):
    open(p, "w", encoding="utf-8").write(s)


# порт: Delete<E> после каждого List<E>, абзац контракта
p = "internal/store/deps.go"
s = read(p)
s, n = re.subn(
    r"(\tList\w+\(ctx context\.Context\) \(\[\]\*models\.(\w+), error\))",
    lambda m: m.group(1) + "\n\tDelete" + m.group(2) + "(ctx context.Context, id models.ID) error",
    s,
)
if n != 21:
    raise SystemExit(f"deps.go: ожидалось 21 метод List*, найдено {n}")
marker = "//   - Save* — upsert"
if marker not in s:
    raise SystemExit("deps.go: не найден абзац Save*")
s = s.replace(marker, """//   - Delete* удаляет сущность вместе с её связными строками, записями
//     поискового индекса и source_links и осиротевшими значениями. Нет такой
//     сущности — models.ErrNotFound; на сущность ссылаются другие (строгие
//     ссылки) — *models.InUseError со списком ссылающихся (до
//     models.MaxReferrers), ничего не удаляется; ссылки «ON DELETE SET NULL»
//     (например, Attachment.DocumentID) обнуляются.
""" + marker, 1)
write(p, s)

# адаптер: поля Store для графа схемы
p = "internal/store/sqlstore/sqlstore.go"
s = read(p)
old = """type Store struct {
	st *storage.Storage
	db *storage.DB
}"""
if old not in s:
    raise SystemExit("sqlstore.go: не найдена структура Store")
s = s.replace(old, """type Store struct {
	st *storage.Storage
	db *storage.DB

	graphMu sync.Mutex
	schema  *schemaGraph // граф внешних ключей, строится при первом удалении
}""")
s = s.replace('\t"fmt"\n', '\t"fmt"\n\t"sync"\n', 1)
write(p, s)
```

Затем перегенерировать мок порта: `(cd internal/store && go generate ./...)`.

- [ ] **Step 3: Рубеж**

Run: `gofmt -l . && go build ./... && go vet ./... && go test ./... -count=1`
Expected: gofmt пусто, сборка и `vet` без ошибок, все пакеты `ok` (включая `TestEntityTablesMatchSchema`, `TestSchemaGraphShapeIsSupported`). Если `go build` жалуется на `web/embed.go` (`pattern all:dist`) — соберите `go build ./internal/... ./cmd/...`.

- [ ] **Step 4: Commit**

```bash
git add -A internal
git commit -m "feat(store): Delete* для всех сущностей по графу внешних ключей"
```

---

### Task 3: Тесты удаления

**Files:**
- Create: `internal/store/sqlstore/delete_test.go`

**Interfaces:**
- Consumes: Task 2; `newStore`, `countRows`, `getters`, `canceled` (`sqlstore_test.go`, `context_test.go`); `scanRowsStrings` (`fkgraph_test.go`).
- Produces: `deleters`, `snapshot`, `fullChain`, тесты `TestDeleteRestoresTableCountsAtEveryStep`, `TestDeleteMethodsAreCovered`, `TestDeleteMissingReturnsErrNotFound`, `TestDeleteCanceledContext`, `TestDeleteInUse`, `TestDeleteInUseCapsReferrers`, `TestDeleteSetNullReferenceDoesNotBlock`, `TestDeleteDoesNotTouchOtherEntities`, `TestDeleteSelfReferenceDoesNotBlock`, `TestDeleteInUseDeduplicatesReferrers`, `TestDeleteInUseDistinctBeforeLimit`.

- [ ] **Step 1: Создать тесты**

Создать `internal/store/sqlstore/delete_test.go`, выполнить `gofmt -w` на нём:

```go
package sqlstore

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type deleteFunc func(*Store, context.Context, models.ID) error

var deleters = map[string]deleteFunc{
	"Person":                 (*Store).DeletePerson,
	"Relation":               (*Store).DeleteRelation,
	"Residence":              (*Store).DeleteResidence,
	"Family":                 (*Store).DeleteFamily,
	"Surname":                (*Store).DeleteSurname,
	"GivenName":              (*Store).DeleteGivenName,
	"Patronymic":             (*Store).DeletePatronymic,
	"Estate":                 (*Store).DeleteEstate,
	"Title":                  (*Store).DeleteTitle,
	"AdministrativeDivision": (*Store).DeleteAdministrativeDivision,
	"Church":                 (*Store).DeleteChurch,
	"Parish":                 (*Store).DeleteParish,
	"Event":                  (*Store).DeleteEvent,
	"Source":                 (*Store).DeleteSource,
	"Archive":                (*Store).DeleteArchive,
	"ArchiveNode":            (*Store).DeleteArchiveNode,
	"ArchiveDocument":        (*Store).DeleteArchiveDocument,
	"Attachment":             (*Store).DeleteAttachment,
	"Citation":               (*Store).DeleteCitation,
	"Note":                   (*Store).DeleteNote,
	"Repository":             (*Store).DeleteRepository,
}

// snapshot — число строк во всех таблицах схемы (включая value-таблицы,
// search_index и source_links).
func snapshot(t *testing.T, s *Store) map[string]int {
	t.Helper()

	tables, err := scanRowsStrings(s,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		t.Fatalf("tables: %v", err)
	}

	out := make(map[string]int, len(tables))
	for _, table := range tables {
		out[table] = countRows(t, s, table)
	}

	return out
}

// link — доказательство для сущности (t, id) через цитату cit-1.
func link(t models.Type, id models.ID) []models.SourceLink {
	return []models.SourceLink{{
		CitationID: "cit-1", TargetType: t, TargetID: id,
		Reliability: models.ReliabilityPrimary, Role: "запись", Note: "прямое",
	}}
}

// chainStep — одна сущность цепочки: вид (ключ таблиц getters/deleters), id и
// сохранение полностью заполненной сущности со всеми списками, датами,
// якорем и доказательствами.
type chainStep struct {
	kind string
	id   models.ID
	save func(context.Context, *Store) error
}

// fullChain — по одной полностью заполненной сущности каждого вида. Порядок —
// порядок сохранения: каждая сущность зависит только от предыдущих.
func fullChain() []chainStep {
	since := models.FactDate{Year: 1881, Month: 3, Day: 15, Precision: models.PrecisionDay, Modifier: models.ModifierExact}
	until := models.FactDate{Year: 1917, Precision: models.PrecisionYear, Modifier: models.ModifierExact}
	docID := models.ID("doc-1")
	parentNode := models.ID("node-1")
	rootDiv := models.ID("ad-root")
	book := models.ID("note-book")
	text := func(s string) models.TextRef { return models.TextRef{Text: s} }
	ref := func(s string, id models.ID, t models.Type) models.TextRef {
		return models.TextRef{Text: s, Ref: id, Type: t}
	}

	return []chainStep{
		{"Repository", "rep-1", func(ctx context.Context, s *Store) error {
			return s.SaveRepository(ctx, &models.Repository{
				ID: "rep-1", Name: "ЦГА Москвы", Type: models.RepositoryTypeArchive, Address: "Профсоюзная, 80",
				URLs: []models.TextRef{text("cgamos.ru")}, Notes: []models.TextRef{text("читальный зал")}, Private: true,
			})
		}},
		{"Source", "src-1", func(ctx context.Context, s *Store) error {
			return s.SaveSource(ctx, &models.Source{
				ID: "src-1", Kind: models.SourceKindArchivalScan, Title: "МК Давыдово", Author: "причт",
				Date: &since, Reliability: models.ReliabilityPrimary, RepositoryID: "rep-1",
				Notes: []models.TextRef{text("скан")}, Private: true,
			})
		}},
		{"Citation", "cit-1", func(ctx context.Context, s *Store) error {
			return s.SaveCitation(ctx, &models.Citation{
				ID: "cit-1", SourceID: "src-1", Text: "л. 12 об.", Note: "запись 5",
				Anchor: &models.ArchiveAnchor{NodeID: "node-1", DocumentID: "doc-1", Page: 12, Rect: "1,1,9,9"},
			})
		}},
		{"Repository", "rep-2", func(ctx context.Context, s *Store) error {
			return s.SaveRepository(ctx, &models.Repository{
				ID: "rep-2", Name: "РГАДА", URLs: []models.TextRef{text("rgada.info")},
				Notes: []models.TextRef{text("ф. 1209")}, Sources: link(models.TypeRepository, "rep-2"),
			})
		}},
		{"Archive", "arc-1", func(ctx context.Context, s *Store) error {
			return s.SaveArchive(ctx, &models.Archive{
				ID: "arc-1", Name: "ЦГАМ", System: &models.TextRef{Text: "фонд-опись-дело"}, RepositoryID: "rep-2",
				Notes: []models.TextRef{text("оцифрован")}, Sources: link(models.TypeArchive, "arc-1"),
			})
		}},
		{"ArchiveNode", "node-1", func(ctx context.Context, s *Store) error {
			return s.SaveArchiveNode(ctx, &models.ArchiveNode{
				ID: "node-1", Type: "fund", ArchiveID: "arc-1", Label: "203", Name: "Консистория",
				Since: &since, Until: &until, Parish: &models.TextRef{Text: "Никольский"},
				Settlements: []models.TextRef{text("Давыдово")}, Notes: []models.TextRef{text("частично утрачен")},
				Sources: link(models.TypeArchiveNode, "node-1"),
			})
		}},
		{"ArchiveNode", "node-2", func(ctx context.Context, s *Store) error {
			return s.SaveArchiveNode(ctx, &models.ArchiveNode{
				ID: "node-2", Type: "inventory", ArchiveID: "arc-1", ParentID: &parentNode, Label: "3", Name: "Опись",
			})
		}},
		{"ArchiveDocument", "doc-1", func(ctx context.Context, s *Store) error {
			return s.SaveArchiveDocument(ctx, &models.ArchiveDocument{
				ID: "doc-1", UnitID: "node-2", Title: "МК 1881", Kind: "метрическая книга",
				Since: &since, Until: &until, Parish: &models.TextRef{Text: "Никольский"},
				Settlements: []models.TextRef{text("Давыдово")}, Notes: []models.TextRef{text("том 1")},
				Sources: link(models.TypeArchiveDocument, "doc-1"),
			})
		}},
		{"Attachment", "att-1", func(ctx context.Context, s *Store) error {
			return s.SaveAttachment(ctx, &models.Attachment{
				ID: "att-1", Kind: models.AttachmentKindScan, URI: "file://1.jpg", Filename: "1.jpg",
				MIME: "image/jpeg", Page: 12, NodeID: "node-2", DocumentID: &docID, Note: "разворот",
			})
		}},
		{"Surname", "sur-1", func(ctx context.Context, s *Store) error {
			return s.SaveSurname(ctx, &models.Surname{
				ID: "sur-1", Canonical: "Иванов", Variants: []models.TextRef{text("Иванофф")},
				Items: []models.TextRef{text("Иванов Пётр")}, Notes: []models.TextRef{text("частая")},
			})
		}},
		{"GivenName", "giv-1", func(ctx context.Context, s *Store) error {
			return s.SaveGivenName(ctx, &models.GivenName{
				ID: "giv-1", Canonical: "Пётр", Gender: models.MaleName,
				Variants: []models.TextRef{text("Петр")}, Items: []models.TextRef{text("Пётр Иванов")},
				Notes: []models.TextRef{text("апостол")},
			})
		}},
		{"Patronymic", "pat-1", func(ctx context.Context, s *Store) error {
			return s.SavePatronymic(ctx, &models.Patronymic{
				ID: "pat-1", Canonical: "Сергеевич", Variants: []models.TextRef{text("Сергиевич")},
				Items: []models.TextRef{text("Иванов Пётр Сергеевич")}, Notes: []models.TextRef{text("по отцу")},
			})
		}},
		{"Estate", "est-1", func(ctx context.Context, s *Store) error {
			return s.SaveEstate(ctx, &models.Estate{
				ID: "est-1", Canonical: "крестьянин", Variants: []models.TextRef{text("крестьяне")},
				Items: []models.TextRef{text("Иванов")}, Notes: []models.TextRef{text("сословие")},
			})
		}},
		{"Title", "tit-1", func(ctx context.Context, s *Store) error {
			return s.SaveTitle(ctx, &models.Title{
				ID: "tit-1", Canonical: "унтер-офицер", Variants: []models.TextRef{text("унтер")},
				Items: []models.TextRef{text("Иванов")}, Notes: []models.TextRef{text("звание")},
			})
		}},
		{"AdministrativeDivision", "ad-root", func(ctx context.Context, s *Store) error {
			return s.SaveAdministrativeDivision(ctx, &models.AdministrativeDivision{
				ID: "ad-root", Name: "Московская", Type: models.AdminDivisionGovernorate,
			})
		}},
		{"AdministrativeDivision", "ad-1", func(ctx context.Context, s *Store) error {
			return s.SaveAdministrativeDivision(ctx, &models.AdministrativeDivision{
				ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionDerevnya, ParentID: &rootDiv,
				Items:      []models.TextRef{ref("двор Ивановых", "p-1", models.TypePerson)},
				Variants:   []string{"Давыдовка"},
				Renames:    []models.NamedPeriod{{Text: "Давыдовка", Since: "1800", Until: "1850"}},
				Successors: []models.TextRef{text("Давыдово (совр.)")},
				Since:      &since, Until: &until, Notes: []models.TextRef{text("упомянуто")},
				Sources: link(models.TypeAdministrativeDivision, "ad-1"),
			})
		}},
		{"Church", "chu-1", func(ctx context.Context, s *Store) error {
			return s.SaveChurch(ctx, &models.Church{
				ID: "chu-1", Name: "Никольская", Parish: &models.TextRef{Text: "Никольский приход"},
				Settlements: []models.TextRef{text("Давыдово")}, Variants: []string{"Николая Чудотворца"},
				Notes: []models.TextRef{text("деревянная")}, Sources: link(models.TypeChurch, "chu-1"),
			})
		}},
		{"Parish", "par-1", func(ctx context.Context, s *Store) error {
			return s.SaveParish(ctx, &models.Parish{
				ID: "par-1", Name: "Никольский", Church: &models.TextRef{Text: "Никольская"},
				Settlements: []models.TextRef{text("Давыдово")}, Since: &since, Until: &until,
				Notes: []models.TextRef{text("приход")}, Sources: link(models.TypeParish, "par-1"),
			})
		}},
		{"Person", "p-1", func(ctx context.Context, s *Store) error {
			return s.SavePerson(ctx, &models.Person{
				ID: "p-1", Gender: models.Male,
				Names: []models.PersonName{
					{Type: models.PersonNameMain, Surname: text("Иванов"), Given: text("Пётр"),
						Patronymic: text("Сергеевич"), Prefix: "фон", Suffix: "ст.", Since: &since, Until: &until},
					{Type: models.PersonNameBirth, Surname: text("Петров"), Given: text("Пётр")},
				},
				Estates: []models.TextRef{text("крестьянин")}, Titles: []models.TextRef{text("унтер-офицер")},
				Nicknames: []models.TextRef{text("Петруха")}, Notes: []models.TextRef{text("из ревизии")},
				Sources: link(models.TypePerson, "p-1"), Private: true,
			})
		}},
		{"Person", "p-2", func(ctx context.Context, s *Store) error {
			return s.SavePerson(ctx, &models.Person{ID: "p-2", Gender: models.Female})
		}},
		{"Family", "fam-1", func(ctx context.Context, s *Store) error {
			return s.SaveFamily(ctx, &models.Family{
				ID: "fam-1", Name: "Ивановы", Members: []models.TextRef{ref("Иванов Пётр", "p-1", models.TypePerson)},
				Notes: []models.TextRef{text("линия по отцу")}, Sources: link(models.TypeFamily, "fam-1"),
			})
		}},
		{"Relation", "rel-1", func(ctx context.Context, s *Store) error {
			return s.SaveRelation(ctx, &models.Relation{
				ID: "rel-1", Kind: models.RelationKindMarriage, PersonA: "p-1", PersonB: "p-2",
				Since: &since, Until: &until, Notes: []models.TextRef{text("венчание")},
				Sources: link(models.TypeRelation, "rel-1"), Private: true,
			})
		}},
		{"Residence", "res-1", func(ctx context.Context, s *Store) error {
			return s.SaveResidence(ctx, &models.Residence{
				ID: "res-1", PersonID: "p-1", PlaceID: "ad-1", Since: &since, Until: &until,
				Note: "по ревизии", Sources: link(models.TypeResidence, "res-1"),
			})
		}},
		{"Event", "ev-1", func(ctx context.Context, s *Store) error {
			return s.SaveEvent(ctx, &models.Event{
				ID: "ev-1", Type: models.EventTypeBirth, Date: &since,
				Place: &models.PlaceRef{Text: "Давыдово", Ref: "ad-1", Type: models.TypeAdministrativeDivision},
				Participants: []models.EventParticipant{
					{PersonID: "p-1", Role: "ребёнок", Note: "первый"}, {PersonID: "p-2", Role: "мать"},
				},
				Sources: link(models.TypeEvent, "ev-1"), Notes: []models.TextRef{text("метрика")}, Private: true,
			})
		}},
		{"Note", "note-book", func(ctx context.Context, s *Store) error {
			return s.SaveNote(ctx, &models.Note{
				ID: "note-book", Kind: models.NoteKindBook, Title: "Род Ивановых", Text: "# Род",
				Sources: link(models.TypeNote, "note-book"),
			})
		}},
		{"Note", "note-ch1", func(ctx context.Context, s *Store) error {
			return s.SaveNote(ctx, &models.Note{
				ID: "note-ch1", Kind: models.NoteKindChapter, Title: "Глава 1", Text: "## Давыдово",
				ParentID: &book, Sources: link(models.TypeNote, "note-ch1"), Private: true,
			})
		}},
	}
}

// TestDeleteRestoresTableCountsAtEveryStep: сущности сохраняются по одной в
// порядке зависимостей, перед каждой снимается счётчик строк всех таблиц;
// затем они удаляются в обратном порядке, и после каждого удаления счётчики
// равны снимку до её сохранения. Так проверяется всё разом: связные строки,
// search_index, source_links, text_refs, dates, anchors — и по каждой
// сущности отдельно.
func TestDeleteRestoresTableCountsAtEveryStep(t *testing.T) {
	s := newStore(t)
	steps := fullChain()
	snaps := make([]map[string]int, len(steps))

	for i, st := range steps {
		snaps[i] = snapshot(t, s)

		if err := st.save(t.Context(), s); err != nil {
			t.Fatalf("save %s %s: %v", st.kind, st.id, err)
		}

		if reflect.DeepEqual(snaps[i], snapshot(t, s)) {
			t.Fatalf("save %s %s не изменил ни одной таблицы", st.kind, st.id)
		}
	}

	if got := snapshot(t, s); got["search_index"] == 0 || got["source_links"] == 0 ||
		got["text_refs"] == 0 || got["dates"] == 0 || got["anchors"] == 0 {
		t.Fatalf("фикстура не заполнила служебные таблицы: %v", got)
	}

	for i := len(steps) - 1; i >= 0; i-- {
		st := steps[i]

		if err := deleters[st.kind](s, t.Context(), st.id); err != nil {
			t.Fatalf("delete %s %s: %v", st.kind, st.id, err)
		}

		if err := getters[st.kind](s, t.Context(), st.id); !errors.Is(err, models.ErrNotFound) {
			t.Fatalf("Get%s(%s) после удаления = %v, ожидалось ErrNotFound", st.kind, st.id, err)
		}

		if got := snapshot(t, s); !reflect.DeepEqual(got, snaps[i]) {
			t.Fatalf("после delete %s %s счётчики таблиц не вернулись к снимку до save:\n got  %v\n want %v",
				st.kind, st.id, got, snaps[i])
		}
	}
}

// TestDeleteMethodsAreCovered: число Delete*-методов адаптера совпадает с
// таблицей deleters, а та — с таблицей getters.
func TestDeleteMethodsAreCovered(t *testing.T) {
	typ := reflect.TypeOf(&Store{})

	var n int

	for i := 0; i < typ.NumMethod(); i++ {
		name := typ.Method(i).Name
		if strings.HasPrefix(name, "Delete") {
			n++

			if _, ok := deleters[strings.TrimPrefix(name, "Delete")]; !ok {
				t.Errorf("метод %s отсутствует в таблице deleters", name)
			}
		}
	}

	if n != len(deleters) || len(deleters) != len(getters) {
		t.Fatalf("методов Delete* %d, в deleters %d, в getters %d", n, len(deleters), len(getters))
	}
}

// TestDeleteMissingReturnsErrNotFound: удаление отсутствующей сущности —
// models.ErrNotFound для всех 21 видов.
func TestDeleteMissingReturnsErrNotFound(t *testing.T) {
	s := newStore(t)

	for kind, del := range deleters {
		t.Run(kind, func(t *testing.T) {
			if err := del(s, t.Context(), "nope"); !errors.Is(err, models.ErrNotFound) {
				t.Fatalf("Delete%s(nope) = %v, ожидалось models.ErrNotFound", kind, err)
			}
		})
	}
}

// TestDeleteCanceledContext: отменённый контекст — ошибка контекста, сущность
// остаётся на месте.
func TestDeleteCanceledContext(t *testing.T) {
	s := newStore(t)

	if err := s.SavePerson(t.Context(), &models.Person{ID: "p-1"}); err != nil {
		t.Fatalf("save: %v", err)
	}

	err := s.DeletePerson(canceled(t), "p-1")
	if !errors.Is(err, context.Canceled) || errors.Is(err, models.ErrNotFound) {
		t.Fatalf("DeletePerson с отменённым ctx = %v, ожидалось context.Canceled", err)
	}

	if _, err := s.GetPerson(t.Context(), "p-1"); err != nil {
		t.Fatalf("персона пропала после отменённого удаления: %v", err)
	}
}

// inUseCase — удаление (kind, id) при готовых зависимостях запрещено, а после
// снятия ссылок (release) проходит.
type inUseCase struct {
	name      string
	setup     func(t *testing.T, s *Store)
	kind      string
	id        models.ID
	typ       models.Type
	referrers []models.EntityRef
	release   func(t *testing.T, s *Store)
}

func mustDo(t *testing.T, what string, err error) {
	t.Helper()

	if err != nil {
		t.Fatalf("%s: %v", what, err)
	}
}

func TestDeleteInUse(t *testing.T) {
	persons := func(t *testing.T, s *Store, ids ...models.ID) {
		t.Helper()

		for _, id := range ids {
			mustDo(t, "save person "+string(id), s.SavePerson(t.Context(), &models.Person{ID: id}))
		}
	}
	division := func(t *testing.T, s *Store, id models.ID, parent *models.ID) {
		t.Helper()
		mustDo(t, "save division "+string(id), s.SaveAdministrativeDivision(t.Context(),
			&models.AdministrativeDivision{ID: id, Name: string(id), Type: models.AdminDivisionDerevnya, ParentID: parent}))
	}
	repo := func(t *testing.T, s *Store, id models.ID) {
		t.Helper()
		mustDo(t, "save repository "+string(id), s.SaveRepository(t.Context(), &models.Repository{ID: id, Name: "Р"}))
	}
	archiveChain := func(t *testing.T, s *Store) {
		t.Helper()
		repo(t, s, "rep-1")
		mustDo(t, "archive", s.SaveArchive(t.Context(), &models.Archive{ID: "arc-1", Name: "А", RepositoryID: "rep-1"}))
		mustDo(t, "node", s.SaveArchiveNode(t.Context(), &models.ArchiveNode{ID: "node-1", Type: "fund", ArchiveID: "arc-1"}))
	}
	ref := func(t models.Type, id models.ID) models.EntityRef { return models.EntityRef{Type: t, ID: id} }

	cases := []inUseCase{
		{
			name: "персона со связью", kind: "Person", id: "p-1", typ: models.TypePerson,
			setup: func(t *testing.T, s *Store) {
				persons(t, s, "p-1", "p-2")
				mustDo(t, "relation", s.SaveRelation(t.Context(), &models.Relation{
					ID: "rel-1", Kind: models.RelationKindMarriage, PersonA: "p-1", PersonB: "p-2"}))
			},
			referrers: []models.EntityRef{ref(models.TypeRelation, "rel-1")},
			release:   func(t *testing.T, s *Store) { mustDo(t, "rel", s.DeleteRelation(t.Context(), "rel-1")) },
		},
		{
			name: "персона — участник события (связная таблица → владелец)", kind: "Person", id: "p-1", typ: models.TypePerson,
			setup: func(t *testing.T, s *Store) {
				persons(t, s, "p-1")
				mustDo(t, "event", s.SaveEvent(t.Context(), &models.Event{ID: "ev-1", Type: models.EventTypeBirth,
					Participants: []models.EventParticipant{{PersonID: "p-1"}, {PersonID: "p-1", Role: "восприемник"}}}))
			},
			referrers: []models.EntityRef{ref(models.TypeEvent, "ev-1")}, // событие один раз, хоть участий два
			release:   func(t *testing.T, s *Store) { mustDo(t, "ev", s.DeleteEvent(t.Context(), "ev-1")) },
		},
		{
			name: "персона с проживанием", kind: "Person", id: "p-1", typ: models.TypePerson,
			setup: func(t *testing.T, s *Store) {
				persons(t, s, "p-1")
				division(t, s, "ad-1", nil)
				mustDo(t, "residence", s.SaveResidence(t.Context(), &models.Residence{ID: "res-1", PersonID: "p-1", PlaceID: "ad-1"}))
			},
			referrers: []models.EntityRef{ref(models.TypeResidence, "res-1")},
			release:   func(t *testing.T, s *Store) { mustDo(t, "res", s.DeleteResidence(t.Context(), "res-1")) },
		},
		{
			name: "деление с дочерним и проживанием", kind: "AdministrativeDivision", id: "ad-1", typ: models.TypeAdministrativeDivision,
			setup: func(t *testing.T, s *Store) {
				parent := models.ID("ad-1")
				division(t, s, "ad-1", nil)
				division(t, s, "ad-2", &parent)
				persons(t, s, "p-1")
				mustDo(t, "residence", s.SaveResidence(t.Context(), &models.Residence{ID: "res-1", PersonID: "p-1", PlaceID: "ad-1"}))
			},
			referrers: []models.EntityRef{ref(models.TypeAdministrativeDivision, "ad-2"), ref(models.TypeResidence, "res-1")},
			release: func(t *testing.T, s *Store) {
				mustDo(t, "res", s.DeleteResidence(t.Context(), "res-1"))
				mustDo(t, "ad-2", s.DeleteAdministrativeDivision(t.Context(), "ad-2"))
			},
		},
		{
			name: "хранилище с источником и архивом", kind: "Repository", id: "rep-1", typ: models.TypeRepository,
			setup: func(t *testing.T, s *Store) {
				archiveChain(t, s)
				mustDo(t, "source", s.SaveSource(t.Context(), &models.Source{ID: "src-1", Kind: models.SourceKindMemory, Title: "С", RepositoryID: "rep-1"}))
			},
			referrers: []models.EntityRef{ref(models.TypeArchive, "arc-1"), ref(models.TypeSource, "src-1")},
			release: func(t *testing.T, s *Store) {
				mustDo(t, "src", s.DeleteSource(t.Context(), "src-1"))
				mustDo(t, "node", s.DeleteArchiveNode(t.Context(), "node-1"))
				mustDo(t, "arc", s.DeleteArchive(t.Context(), "arc-1"))
			},
		},
		{
			name: "источник с цитатой", kind: "Source", id: "src-1", typ: models.TypeSource,
			setup: func(t *testing.T, s *Store) {
				mustDo(t, "source", s.SaveSource(t.Context(), &models.Source{ID: "src-1", Kind: models.SourceKindMemory, Title: "С"}))
				mustDo(t, "citation", s.SaveCitation(t.Context(), &models.Citation{ID: "cit-1", SourceID: "src-1"}))
			},
			referrers: []models.EntityRef{ref(models.TypeCitation, "cit-1")},
			release:   func(t *testing.T, s *Store) { mustDo(t, "cit", s.DeleteCitation(t.Context(), "cit-1")) },
		},
		{
			name: "цитата, на которую ссылается доказательство (source_links → цель)", kind: "Citation", id: "cit-1", typ: models.TypeCitation,
			setup: func(t *testing.T, s *Store) {
				mustDo(t, "source", s.SaveSource(t.Context(), &models.Source{ID: "src-1", Kind: models.SourceKindMemory, Title: "С"}))
				mustDo(t, "citation", s.SaveCitation(t.Context(), &models.Citation{ID: "cit-1", SourceID: "src-1"}))
				mustDo(t, "person", s.SavePerson(t.Context(), &models.Person{ID: "p-1", Sources: link(models.TypePerson, "p-1")}))
			},
			referrers: []models.EntityRef{ref(models.TypePerson, "p-1")},
			release:   func(t *testing.T, s *Store) { mustDo(t, "p", s.DeletePerson(t.Context(), "p-1")) },
		},
		{
			name: "заметка с дочерней", kind: "Note", id: "note-book", typ: models.TypeNote,
			setup: func(t *testing.T, s *Store) {
				parent := models.ID("note-book")
				mustDo(t, "book", s.SaveNote(t.Context(), &models.Note{ID: "note-book", Kind: models.NoteKindBook, Title: "К"}))
				mustDo(t, "chapter", s.SaveNote(t.Context(), &models.Note{ID: "note-ch1", Kind: models.NoteKindChapter, Title: "Г", ParentID: &parent}))
			},
			referrers: []models.EntityRef{ref(models.TypeNote, "note-ch1")},
			release:   func(t *testing.T, s *Store) { mustDo(t, "ch", s.DeleteNote(t.Context(), "note-ch1")) },
		},
		{
			name: "архив с узлом", kind: "Archive", id: "arc-1", typ: models.TypeArchive,
			setup:     archiveChain,
			referrers: []models.EntityRef{ref(models.TypeArchiveNode, "node-1")},
			release:   func(t *testing.T, s *Store) { mustDo(t, "node", s.DeleteArchiveNode(t.Context(), "node-1")) },
		},
		{
			name: "узел с дочерним узлом, документом и вложением", kind: "ArchiveNode", id: "node-1", typ: models.TypeArchiveNode,
			setup: func(t *testing.T, s *Store) {
				archiveChain(t, s)
				parent := models.ID("node-1")
				mustDo(t, "child", s.SaveArchiveNode(t.Context(), &models.ArchiveNode{ID: "node-2", Type: "inventory", ArchiveID: "arc-1", ParentID: &parent}))
				mustDo(t, "doc", s.SaveArchiveDocument(t.Context(), &models.ArchiveDocument{ID: "doc-1", UnitID: "node-1", Title: "Д"}))
				mustDo(t, "att", s.SaveAttachment(t.Context(), &models.Attachment{ID: "att-1", Kind: models.AttachmentKindScan, URI: "file://1", NodeID: "node-1"}))
			},
			referrers: []models.EntityRef{
				ref(models.TypeArchiveDocument, "doc-1"), ref(models.TypeArchiveNode, "node-2"), ref(models.TypeAttachment, "att-1"),
			},
			release: func(t *testing.T, s *Store) {
				mustDo(t, "att", s.DeleteAttachment(t.Context(), "att-1"))
				mustDo(t, "doc", s.DeleteArchiveDocument(t.Context(), "doc-1"))
				mustDo(t, "child", s.DeleteArchiveNode(t.Context(), "node-2"))
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newStore(t)
			c.setup(t, s)

			before := snapshot(t, s)

			err := deleters[c.kind](s, t.Context(), c.id)

			var inUse *models.InUseError
			if !errors.As(err, &inUse) {
				t.Fatalf("Delete%s(%s) = %v, ожидался *models.InUseError", c.kind, c.id, err)
			}

			if inUse.Type != c.typ || inUse.ID != c.id || !reflect.DeepEqual(inUse.Referrers, c.referrers) {
				t.Fatalf("InUseError = %+v, ожидались тип %s, id %s, ссылающиеся %v", inUse, c.typ, c.id, c.referrers)
			}

			if !strings.Contains(err.Error(), string(c.id)) {
				t.Errorf("текст ошибки %q не называет id", err)
			}

			if got := snapshot(t, s); !reflect.DeepEqual(got, before) {
				t.Fatalf("отказ изменил таблицы:\n got  %v\n want %v", got, before)
			}

			if err := getters[c.kind](s, t.Context(), c.id); err != nil {
				t.Fatalf("сущность пропала после отказа: %v", err)
			}

			c.release(t, s)

			if err := deleters[c.kind](s, t.Context(), c.id); err != nil {
				t.Fatalf("Delete%s(%s) после снятия ссылок: %v", c.kind, c.id, err)
			}
		})
	}
}

// TestDeleteInUseCapsReferrers: список ссылающихся ограничен MaxReferrers,
// порядок стабильный (по id).
func TestDeleteInUseCapsReferrers(t *testing.T) {
	s := newStore(t)

	mustDo(t, "person", s.SavePerson(t.Context(), &models.Person{ID: "p-1"}))

	total := models.MaxReferrers + 5
	for i := 0; i < total; i++ {
		id := models.ID("ev-" + strconv.Itoa(1000+i))
		mustDo(t, "event", s.SaveEvent(t.Context(), &models.Event{
			ID: id, Type: models.EventTypeBirth, Participants: []models.EventParticipant{{PersonID: "p-1"}},
		}))
	}

	var inUse *models.InUseError
	if err := s.DeletePerson(t.Context(), "p-1"); !errors.As(err, &inUse) {
		t.Fatalf("DeletePerson = %v, ожидался *models.InUseError", err)
	}

	if len(inUse.Referrers) != models.MaxReferrers {
		t.Fatalf("ссылающихся %d, ожидалось %d", len(inUse.Referrers), models.MaxReferrers)
	}

	for i, r := range inUse.Referrers {
		want := models.EntityRef{Type: models.TypeEvent, ID: models.ID("ev-" + strconv.Itoa(1000+i))}
		if r != want {
			t.Fatalf("Referrers[%d] = %+v, ожидалось %+v", i, r, want)
		}
	}
}

// TestDeleteSetNullReferenceDoesNotBlock: вложение ссылается на документ по
// ON DELETE SET NULL — документ удаляется, а вложение остаётся без документа.
func TestDeleteSetNullReferenceDoesNotBlock(t *testing.T) {
	s := newStore(t)

	mustDo(t, "repository", s.SaveRepository(t.Context(), &models.Repository{ID: "rep-1", Name: "Р"}))
	mustDo(t, "archive", s.SaveArchive(t.Context(), &models.Archive{ID: "arc-1", Name: "А", RepositoryID: "rep-1"}))
	mustDo(t, "node", s.SaveArchiveNode(t.Context(), &models.ArchiveNode{ID: "node-1", Type: "fund", ArchiveID: "arc-1"}))
	mustDo(t, "doc", s.SaveArchiveDocument(t.Context(), &models.ArchiveDocument{ID: "doc-1", UnitID: "node-1", Title: "Д"}))

	docID := models.ID("doc-1")
	mustDo(t, "attachment", s.SaveAttachment(t.Context(), &models.Attachment{
		ID: "att-1", Kind: models.AttachmentKindScan, URI: "file://1", NodeID: "node-1", DocumentID: &docID,
	}))

	mustDo(t, "delete document", s.DeleteArchiveDocument(t.Context(), "doc-1"))

	got, err := s.GetAttachment(t.Context(), "att-1")
	mustDo(t, "get attachment", err)

	if got.DocumentID != nil {
		t.Fatalf("DocumentID = %v после удаления документа, ожидалось nil", *got.DocumentID)
	}
}

// TestDeleteDoesNotTouchOtherEntities: удаление одной сущности не задевает
// соседние строки той же таблицы.
func TestDeleteDoesNotTouchOtherEntities(t *testing.T) {
	s := newStore(t)

	for _, id := range []models.ID{"p-1", "p-2"} {
		mustDo(t, "person", s.SavePerson(t.Context(), &models.Person{
			ID: id, Names: []models.PersonName{{Surname: models.TextRef{Text: "Иванов"}, Given: models.TextRef{Text: "Пётр"}}},
			Notes: []models.TextRef{{Text: "заметка"}},
		}))
	}

	before := snapshot(t, s)

	mustDo(t, "delete", s.DeletePerson(t.Context(), "p-1"))

	got := snapshot(t, s)
	if got["persons"] != 1 || got["person_names"] != before["person_names"]/2 ||
		got["text_refs"] != before["text_refs"]/2 || got["search_index"] != before["search_index"]/2 {
		t.Fatalf("удаление p-1 задело p-2: до %v, после %v", before, got)
	}

	if _, err := s.GetPerson(t.Context(), "p-2"); err != nil {
		t.Fatalf("p-2 пропала: %v", err)
	}
}

// TestDeleteSelfReferenceDoesNotBlock: ссылка сущности на саму себя (заметка —
// собственный родитель) не считается «используется другими».
func TestDeleteSelfReferenceDoesNotBlock(t *testing.T) {
	s := newStore(t)

	mustDo(t, "note", s.SaveNote(t.Context(), &models.Note{ID: "note-1", Kind: models.NoteKindBook, Title: "К"}))

	self := models.ID("note-1")
	mustDo(t, "self parent", s.SaveNote(t.Context(), &models.Note{ID: "note-1", Kind: models.NoteKindBook, Title: "К", ParentID: &self}))
	mustDo(t, "delete", s.DeleteNote(t.Context(), "note-1"))
}

// TestDeleteInUseDeduplicatesReferrers: сущность, ссылающаяся двумя колонками
// (связь person_a = person_b), попадает в список один раз.
func TestDeleteInUseDeduplicatesReferrers(t *testing.T) {
	s := newStore(t)

	mustDo(t, "person", s.SavePerson(t.Context(), &models.Person{ID: "p-1"}))
	mustDo(t, "relation", s.SaveRelation(t.Context(), &models.Relation{
		ID: "rel-1", Kind: models.RelationKindAssociate, PersonA: "p-1", PersonB: "p-1",
	}))

	var inUse *models.InUseError
	if err := s.DeletePerson(t.Context(), "p-1"); !errors.As(err, &inUse) {
		t.Fatalf("DeletePerson = %v, ожидался *models.InUseError", err)
	}

	want := []models.EntityRef{{Type: models.TypeRelation, ID: "rel-1"}}
	if !reflect.DeepEqual(inUse.Referrers, want) {
		t.Fatalf("Referrers = %v, ожидалось %v", inUse.Referrers, want)
	}
}

// TestDeleteInUseDistinctBeforeLimit: повторные участия одной персоны в одном
// событии не съедают лимит списка — все события попадают в него по разу.
func TestDeleteInUseDistinctBeforeLimit(t *testing.T) {
	s := newStore(t)

	mustDo(t, "person", s.SavePerson(t.Context(), &models.Person{ID: "p-1"}))

	const events = 12 // по два участия в каждом: 24 строки при лимите 20

	for i := 0; i < events; i++ {
		mustDo(t, "event", s.SaveEvent(t.Context(), &models.Event{
			ID: models.ID("ev-" + strconv.Itoa(1000+i)), Type: models.EventTypeBirth,
			Participants: []models.EventParticipant{{PersonID: "p-1"}, {PersonID: "p-1", Role: "восприемник"}},
		}))
	}

	var inUse *models.InUseError
	if err := s.DeletePerson(t.Context(), "p-1"); !errors.As(err, &inUse) {
		t.Fatalf("DeletePerson = %v, ожидался *models.InUseError", err)
	}

	if len(inUse.Referrers) != events {
		t.Fatalf("ссылающихся %d, ожидалось %d (по одному на событие)", len(inUse.Referrers), events)
	}
}
```

- [ ] **Step 2: Прогнать**

Run: `gofmt -l . && go vet ./... && go test ./internal/store/... -count=1`
Expected: gofmt пусто, `vet` чисто, тесты `ok`.

- [ ] **Step 3: Мутационные проверки (вручную, каждую откатить сразу: `git checkout -- internal/store/sqlstore/delete.go`)**

Каждая мутация вносится в `delete.go`; указанный тест должен ПРОВАЛИТЬСЯ.

| Мутация | Ожидаемый провал |
|---|---|
| в `deleteEntity` удалить блок `DELETE FROM source_links …` | `TestDeleteRestoresTableCountsAtEveryStep` |
| удалить блок `DELETE FROM search_index …` | `TestDeleteRestoresTableCountsAtEveryStep` |
| цикл `for _, v := range valueTables` → `[]string{"text_refs", "dates"}` | `TestDeleteRestoresTableCountsAtEveryStep` |
| в `collectOwnedValues` удалить цикл по `cascadeChildren` | `TestDeleteRestoresTableCountsAtEveryStep` |
| `if !seen[ref] && len(out) < limit` → `if len(out) < limit` | `TestDeleteInUseDeduplicatesReferrers` |
| `SELECT DISTINCT ?, ` → `SELECT ?, ` | `TestDeleteInUseDistinctBeforeLimit` |
| убрать ` AND id <> ?` из запроса и лишний `string(id)` из `args` | `TestDeleteSelfReferenceDoesNotBlock` |
| `if len(refs) > 0 {` → `if false && len(refs) > 0 {` | `TestDeleteInUse` |
| удалить ветку `if notFound(err) { return models.ErrNotFound }` в проверке существования | `TestDeleteMissingReturnsErrNotFound` |

- [ ] **Step 4: Commit**

```bash
git add internal/store/sqlstore/delete_test.go
git commit -m "test(store): удаление 21 сущности, InUseError, чистка таблиц"
```

---

### Task 4: Документация

**Files:**
- Modify: `docs/data-model/core-read-write.md` (§4), `docs/plans/2026-09-20-core-rw-roadmap.md` (S7)

- [ ] **Step 1: Спецификация §4**

В `docs/data-model/core-read-write.md` заменить пункт

```
- `Delete*(id) error` для всех 21 сущностей: удаляет сущность, свои дочерние
  строки, записи `search_index`/`source_links`, осиротевшие значения. Если на
  сущность ссылаются (RESTRICT) — `*InUseError` со списком ссылающихся.
```

на

```
- `Delete*(ctx, id) error` для всех 21 сущностей: удаляет сущность, свои дочерние
  строки, записи `search_index`/`source_links`, осиротевшие значения. Если на
  сущность ссылаются (RESTRICT) — `*InUseError` со списком ссылающихся (до 20,
  без повторов; для связной таблицы ссылающийся — её владелец, для
  `source_links` — цель доказательства). Ссылки `ON DELETE SET NULL`
  (`Attachment.DocumentID`) удаление не блокируют и обнуляются; ссылка
  сущности на саму себя не считается. Устройство схемы (связные таблицы,
  value-колонки, ссылающиеся строки) берётся из графа внешних ключей
  (`sqlstore/fkgraph.go`), а не из ручных списков (S7).
```

- [ ] **Step 2: Дорожная карта**

В `docs/plans/2026-09-20-core-rw-roadmap.md` в блоке `### S7.` заменить строку файлов

```
- **Файлы:** `models/errors.go` (`InUseError`, `EntityRef`), `store/deps.go`,
  `sqlstore/delete.go` (общая реализация), тесты.
```

на

```
- **Файлы:** `models/errors.go` (`InUseError`, `EntityRef`, `MaxReferrers`),
  `store/deps.go`, `sqlstore/fkgraph.go` (граф внешних ключей),
  `sqlstore/delete.go` (общая реализация), тесты; план —
  `2026-09-21-core-rw-s07-delete.md`.
```

- [ ] **Step 3: Commit**

```bash
git add docs/data-model/core-read-write.md docs/plans/2026-09-20-core-rw-roadmap.md
git commit -m "docs: S7 — Delete* и граф внешних ключей"
```

---

## Правки по итогам ревью (внесены после выполнения задач)

Код в репозитории — источник истины; плановые блоки выше описывают первую версию. Отличия:

- `delete.go`, запрос ссылающихся по `source_links`: `SELECT DISTINCT target_type, target_id … WHERE <e.col> = ?` (без `DISTINCT` цель со многими ссылками вытесняла остальные из `Referrers` — `LIMIT` действовал до удаления повторов); колонка берётся из ребра графа, а не жёстко `citation_id`.
- `delete.go`, отложенная обёртка ошибки: `errors.Is(err, models.ErrNotFound)` и `errors.As(err, &inUse)` вместо `==` и приведения типа.
- `fkgraph.go`: уточнён комментарий `valueCols`.
- `delete_test.go`: добавлен `TestDeleteInUseCitationDistinctTargets`; фикстура `fullChain` — с кириллицей; `TestDeleteCanceledContext` проверяет, что ошибка сбоя называет вид и id.
