# S11: `Search` и `ChildrenOfDivision` — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Порт получает `Search(ctx, query, access, page) ([]models.Hit, error)` — префиксный поиск по поисковому индексу через индекс — и `ChildrenOfDivision(ctx, parent, access, page)` — прямые дочерние единицы деления.

**Architecture:** `models.Hit{Type, ID, Label, Field}`. В схему добавляется покрывающий индекс `idx_search_term (term, entity_table, entity_id, field)` (`CREATE INDEX IF NOT EXISTS` — создаётся при каждом открытии БД, миграции данных нет). `Search` нормализует запрос (`storage.Normalize`), ищет диапазоном `term >= prefix AND term < prefix+U+10FFFF` (не `LIKE`: с `ESCAPE` и бинарной коллацией SQLite не использует индекс, а диапазон не требует экранирования `%`/`_`), группирует по сущности (`MIN(field)`), сортирует по `(вид, id)`. Приватность в `Search` решается по таблице сущности: для `AccessPublic` в запрос добавляется `NOT (entity_table = '<t>' AND EXISTS (SELECT 1 FROM <t> WHERE id = entity_id AND private = 1))` по каждой таблице с колонкой `private` (набор берётся из графа схемы S9). Подпись `Hit.Label` читается одним запросом на вид сущности окна (`hitLabelExprs` — выражение по таблице; персоны — из первого имени). `ChildrenOfDivision` — `pagedIDs` с условием `parent_id = ?` (общий с `listIDs` помощник) и `getDivisions` (пакетный путь S10); нет родителя — `ErrNotFound`.

**Tech Stack:** Go 1.26.4, `database/sql`, `modernc.org/sqlite`, `go.uber.org/mock`.

**Spec:** `docs/data-model/core-read-write.md` §4; `docs/plans/2026-09-20-core-rw-roadmap.md` (этап S11).

## Global Constraints

- Порт: `Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)`; `ChildrenOfDivision(ctx context.Context, parent models.ID, access models.Access, page models.Page) ([]*models.AdministrativeDivision, error)`.
- Запрос нормализуется как термины (нижний регистр, ё→е, без диакритики); пробелы по краям отбрасываются; пустой после нормализации запрос — пустой не-nil результат. Совпадение — префикс одного термина (многословный запрос не разбивается на слова). `%`, `_`, `\` — обычные символы.
- Одна сущность — один `Hit`; `Field` — `MIN(field)` совпавших терминов; порядок — `(вид сущности, id)`; окно (`page` нормализуется как в S9) считается после фильтра приватного. Неизвестный `access` — как публичный. Результат — не-nil срез.
- Приватность: `AccessPublic` скрывает приватные сущности по колонке `private` их таблицы; `search_index` колонки `private` не получает.
- `Hit.Label`: имя/название/заголовок сущности; персона — «фамилия имя отчество» первого имени без пустых частей; событие — «тип место»; узел архива — «метка название»; вложение — имя файла (или URI); пустая подпись заменяется на ID. Каждая таблица, пишущая термины в индекс, имеет запись в `hitLabelExprs` (персоны — своя загрузка; `relations` и `residences` терминов не пишут) — фиксируется тестом.
- Запрос `Search` использует `idx_search_term` как покрывающий индекс (тест — `EXPLAIN QUERY PLAN` для точного текста запроса); выборка детей — индекс по `parent_id` без сортировки.
- `ChildrenOfDivision`: только прямые дети, порядок сохранения (rowid), `access` на деления не влияет (флага приватности нет); нет родителя — `models.ErrNotFound`; лист — пустой не-nil срез; результат идентичен поштучному чтению.
- Работает внутри `InTx` (видны незафиксированные записи) и с отменой `ctx` (ошибка контекста).
- Схема данных не меняется, кроме нового индекса; `Validate`, контракты MCP/HTTP не меняются. Комментарии и тексты ошибок — на русском; код gofmt-clean.
- Проверка рубежа: `gofmt -l .` пусто, `go build ./...`, `go vet ./...`, `go test ./...`.

---

### Task 1: `Hit` и индекс `idx_search_term`

**Files:**
- Modify: `internal/models/query.go`, `internal/storage/schema.go`, `internal/storage/indexes_test.go`

**Interfaces:**
- Consumes: `models.Type`, `models.ID`; `openTestDB` (`storage/db_test.go`); в `indexes_test.go` уже подключён `fmt`.
- Produces: `models.Hit`; индекс `idx_search_term` в `schemaDDL`.

- [ ] **Step 1: Тест (падает — индекса нет)**

В конец `internal/storage/indexes_test.go` дописать:

```go
// TestSearchTermIndex: покрывающий индекс поиска по префиксу термина создан
// схемой и содержит колонки выборки Search в нужном порядке.
func TestSearchTermIndex(t *testing.T) {
	db := openTestDB(t)

	rows, err := db.Query(`SELECT name FROM pragma_index_info('idx_search_term') ORDER BY seqno`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	var cols []string

	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			t.Fatal(err)
		}

		cols = append(cols, c)
	}

	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	want := []string{"term", "entity_table", "entity_id", "field"}
	if fmt.Sprint(cols) != fmt.Sprint(want) {
		t.Fatalf("колонки idx_search_term %v, want %v", cols, want)
	}
}
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test ./internal/storage/ -run SearchTermIndex -count=1`
Expected: FAIL — «колонки idx_search_term [] , want [term entity_table entity_id field]».

- [ ] **Step 3: Реализация**

В конец `internal/models/query.go` дописать:

```go
// Hit — результат поиска: сущность, подпись для показа и поле, по которому
// найдено. Для одной сущности возвращается одна запись (первое совпавшее поле
// по алфавиту имён полей).
type Hit struct {
	Type  Type
	ID    ID
	Label string // подпись сущности: имя, название, заголовок; при пустой — ID
	Field string // поле поискового индекса: name, title, place, …
}
```

В `internal/storage/schema.go` в конце `schemaDDL` — после определения таблицы `search_index` и перед закрывающей `}` — добавить:

```go
	// Префиксный поиск — диапазон по term; индекс покрывающий (все колонки
	// выборки Search), поэтому таблица не читается.
	`CREATE INDEX IF NOT EXISTS idx_search_term
		ON search_index (term, entity_table, entity_id, field)`,
```

(определение `search_index` заканчивается строками `PRIMARY KEY (entity_table, entity_id, field, term)` / `)`,` — индекс идёт следом, через пустую строку.)

- [ ] **Step 4: Тесты проходят**

Run: `gofmt -l internal && go vet ./internal/... && go test ./internal/models/ ./internal/storage/ -count=1`
Expected: gofmt пусто, `vet` чисто, `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/models/query.go internal/storage/schema.go internal/storage/indexes_test.go
git commit -m "feat(storage,models): Hit и покрывающий индекс idx_search_term"
```

---

### Task 2: `Search` и `ChildrenOfDivision` в порту и адаптере

**Files:**
- Create: `internal/store/sqlstore/search.go`
- Modify: `internal/store/deps.go`, `internal/store/deps_test.go` (генерируется), `internal/store/sqlstore/sqlstore.go`, `internal/store/sqlstore/places.go`, `internal/store/sqlstore/helpers.go`, `internal/store/sqlstore/sqlstore_test.go`

**Interfaces:**
- Consumes: Task 1; `graph`, `schemaGraph.private`, `entityTables`, `typeOfTable`, `queryGrouped`, `scanRows`, `getDivisions`, `listIDs` (S7–S10); `storage.Normalize`.
- Produces: порт `Search`, `ChildrenOfDivision`; `searchSQL`, `hitLabelExprs`, `maxRune`, `fillHitLabels`, `loadHitLabels`, `joinNonEmpty`; `pagedIDs`, `pagedIDsSQL` (`listIDs` поверх них); удалён `searchIDs` (заменён `Search`); существующий `TestStoreSearchIndex` переведён на `Search`.

Код проверен на чистой копии `main` (после Task 1): после него сборка, `vet` и все существующие тесты зелёные.

- [ ] **Step 1: Создать `search.go`**

Создать `internal/store/sqlstore/search.go`:

```go
package sqlstore

import (
	"context"
	"database/sql"
	"sort"
	"strings"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/storage"
)

// hitLabelExprs — SQL-выражение подписи сущности по таблице главной строки.
// Персоны (имя из трёх TextRef) и таблицы без поисковых терминов (relations,
// residences) обрабатываются отдельно. Пустая подпись заменяется на ID.
var hitLabelExprs = map[string]string{
	"families":                 "name",
	"surnames":                 "canonical",
	"given_names":              "canonical",
	"patronymics":              "canonical",
	"estates":                  "canonical",
	"titles":                   "canonical",
	"administrative_divisions": "name",
	"churches":                 "name",
	"parishes":                 "name",
	"events":                   "type || COALESCE(' ' || (SELECT text FROM text_refs WHERE id = events.place_id), '')",
	"sources":                  "title",
	"archives":                 "name",
	"archive_nodes":            "trim(label || ' ' || name)",
	"archive_documents":        "title",
	"attachments":              "CASE WHEN filename != '' THEN filename ELSE uri END",
	"citations":                "text",
	"notes":                    "title",
	"repositories":             "name",
}

// maxRune — верхняя граница диапазона префиксного поиска: любая строка,
// начинающаяся с префикса, меньше prefix+maxRune.
const maxRune = "\U0010FFFF"

// searchSQL строит запрос Search: диапазон по term (использует idx_search_term),
// одна строка на сущность, для публичного доступа — исключение приватных строк
// таблиц privateTables (имена берутся из схемы, значения — параметрами).
func searchSQL(privateTables []string) string {
	var b strings.Builder

	b.WriteString(`SELECT si.entity_table, si.entity_id, MIN(si.field)
		FROM search_index si
		WHERE si.term >= ? AND si.term < ?`)

	for _, t := range privateTables {
		b.WriteString(` AND NOT (si.entity_table = '` + t + `' AND EXISTS (
			SELECT 1 FROM ` + t + ` e WHERE e.id = si.entity_id AND e.private = 1))`)
	}

	b.WriteString(`
		GROUP BY si.entity_table, si.entity_id
		ORDER BY si.entity_table, si.entity_id
		LIMIT ? OFFSET ?`)

	return b.String()
}

// Search ищет сущности по префиксу термина; см. store.Store.Search.
func (s *Store) Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error) {
	prefix := storage.Normalize(strings.TrimSpace(query))
	if prefix == "" {
		return []models.Hit{}, nil
	}

	g, err := s.graph(ctx)
	if err != nil {
		return nil, err
	}

	var privateTables []string

	if access != models.AccessFull {
		for table := range g.private {
			privateTables = append(privateTables, table)
		}

		sort.Strings(privateTables) // стабильный текст запроса
	}

	page = page.Normalized()
	q := s.run(ctx)

	hits, err := scanRows(q, func(r *sql.Rows) (models.Hit, error) {
		var table, id, field string
		err := r.Scan(&table, &id, &field)

		return models.Hit{Type: typeOfTable[table], ID: models.ID(id), Field: field}, err
	}, searchSQL(privateTables), prefix, prefix+maxRune, page.Limit, page.Offset)
	if err != nil {
		return nil, err
	}

	if err := fillHitLabels(q, hits); err != nil {
		return nil, err
	}

	if hits == nil {
		hits = []models.Hit{}
	}

	return hits, nil
}

// fillHitLabels проставляет подписи: по одному запросу на вид сущности окна.
func fillHitLabels(q queryer, hits []models.Hit) error {
	byType := map[models.Type][]string{}

	for _, h := range hits {
		byType[h.Type] = append(byType[h.Type], string(h.ID))
	}

	labels := map[models.Type]map[string]string{}

	for typ, ids := range byType {
		l, err := loadHitLabels(q, entityTables[typ], ids)
		if err != nil {
			return err
		}

		labels[typ] = l
	}

	for i := range hits {
		hits[i].Label = labels[hits[i].Type][string(hits[i].ID)]
		if hits[i].Label == "" {
			hits[i].Label = string(hits[i].ID)
		}
	}

	return nil
}

// loadHitLabels читает подписи сущностей одной таблицы: id → подпись.
func loadHitLabels(q queryer, table string, ids []string) (map[string]string, error) {
	out := make(map[string]string, len(ids))

	if table == "persons" {
		names, err := queryGrouped(q, ids, nil, func(in string) string {
			return `SELECT pn.person_id, s.text, g.text, p.text FROM person_names pn
			        JOIN text_refs s ON s.id = pn.surname_id
			        JOIN text_refs g ON g.id = pn.given_id
			        JOIN text_refs p ON p.id = pn.patronymic_id
			        WHERE pn.person_id IN (` + in + `) ORDER BY pn.id`
		}, func(r *sql.Rows) (string, string, error) {
			var owner, sur, giv, pat string
			err := r.Scan(&owner, &sur, &giv, &pat)

			return owner, joinNonEmpty(sur, giv, pat), err
		})
		if err != nil {
			return nil, err
		}

		for id, labels := range names {
			out[id] = labels[0] // первое имя персоны
		}

		return out, nil
	}

	expr, ok := hitLabelExprs[table]
	if !ok {
		return out, nil // таблица без подписи: останется ID
	}

	rows, err := queryGrouped(q, ids, nil, func(in string) string {
		return `SELECT id, ` + expr + ` FROM ` + table + ` WHERE id IN (` + in + `)`
	}, func(r *sql.Rows) (string, string, error) {
		var id, label string
		err := r.Scan(&id, &label)

		return id, label, err
	})
	if err != nil {
		return nil, err
	}

	for id, labels := range rows {
		out[id] = labels[0]
	}

	return out, nil
}

// joinNonEmpty склеивает непустые части через пробел.
func joinNonEmpty(parts ...string) string {
	var kept []string

	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}

	return strings.Join(kept, " ")
}
```

- [ ] **Step 2: Правки порта и адаптера**

Сохранить как `$TMPDIR/s11-edit.py` (вне репозитория) и выполнить из корня: `python3 $TMPDIR/s11-edit.py` (при несовпадении фрагмента скрипт останавливается с сообщением — не подгонять молча, разобраться):

```python
"""S11, шаг задачи 2: порт и адаптер. Запуск из корня репозитория."""


def edit(p, pairs):
    s = open(p, encoding="utf-8").read()
    for old, new in pairs:
        if old not in s:
            raise SystemExit(f"{p}: не найден фрагмент: {old[:60]!r}")
        s = s.replace(old, new, 1)
    open(p, "w", encoding="utf-8").write(s)


# --- sqlstore.go: pagedIDs вместо тела listIDs ----------------------------
edit("internal/store/sqlstore/sqlstore.go", [("""func listIDs(q queryer, table string, publicOnly bool, page models.Page) ([]models.ID, error) {
	page = page.Normalized()

	where := ""
	if publicOnly {
		where = ` WHERE private = 0`
	}

	return scanRows(q, func(r *sql.Rows) (models.ID, error) {
		var id string
		err := r.Scan(&id)

		return models.ID(id), err
	}, `SELECT id FROM `+table+where+` ORDER BY rowid LIMIT ? OFFSET ?`, page.Limit, page.Offset)
}""", """func listIDs(q queryer, table string, publicOnly bool, page models.Page) ([]models.ID, error) {
	where := ""
	if publicOnly {
		where = "private = 0"
	}

	return pagedIDs(q, table, where, nil, page)
}

// pagedIDsSQL — текст запроса pagedIDs (отдельно, чтобы тесты объясняли план
// именно этого запроса).
func pagedIDsSQL(table, where string) string {
	if where != "" {
		where = " WHERE " + where
	}

	return `SELECT id FROM ` + table + where + ` ORDER BY rowid LIMIT ? OFFSET ?`
}

// pagedIDs возвращает id строк таблицы в порядке вставки (rowid) в окне page;
// where — необязательное условие (фиксированный текст, значения — в args).
func pagedIDs(q queryer, table, where string, args []any, page models.Page) ([]models.ID, error) {
	page = page.Normalized()

	return scanRows(q, func(r *sql.Rows) (models.ID, error) {
		var id string
		err := r.Scan(&id)

		return models.ID(id), err
	}, pagedIDsSQL(table, where), append(append([]any{}, args...), page.Limit, page.Offset)...)
}""")])

# --- порт -------------------------------------------------------------------
edit("internal/store/deps.go", [("""	// Person
	GetPerson(""", """	// Search ищет сущности по префиксу термина поискового индекса: запрос
	// нормализуется (регистр, ё/е, диакритика), совпадение — по началу одного
	// термина (многословный запрос не разбивается); пустой запрос — пустой
	// результат. Одна сущность — один Hit; порядок — по виду и id (стабилен),
	// access и page действуют как в List*. Приватность решается по таблице
	// сущности, а не по индексу.
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)

	// ChildrenOfDivision — прямые дочерние единицы деления parent в порядке
	// сохранения; нет такого деления — models.ErrNotFound. access и page — как в
	// List* (у делений флага приватности нет).
	ChildrenOfDivision(ctx context.Context, parent models.ID, access models.Access, page models.Page) ([]*models.AdministrativeDivision, error)

	// Person
	GetPerson(""")])

# --- places.go: ChildrenOfDivision ----------------------------------------
edit("internal/store/sqlstore/places.go", [("""// ListAdministrativeDivisions возвращает окно списка единиц деления.""", """// ChildrenOfDivision возвращает прямые дочерние единицы деления parent в порядке
// сохранения; нет такого деления — models.ErrNotFound.
func (s *Store) ChildrenOfDivision(
	ctx context.Context, parent models.ID, access models.Access, page models.Page,
) ([]*models.AdministrativeDivision, error) {
	q := s.run(ctx)

	var exists int
	if err := q.QueryRow(`SELECT 1 FROM administrative_divisions WHERE id = ?`, string(parent)).Scan(&exists); err != nil {
		if notFound(err) {
			return nil, models.ErrNotFound
		}

		return nil, err
	}

	// у делений нет колонки private — access на выборку не влияет
	ids, err := pagedIDs(q, "administrative_divisions", "parent_id = ?", []any{string(parent)}, page)
	if err != nil {
		return nil, err
	}

	return s.getDivisions(ctx, ids)
}

// ListAdministrativeDivisions возвращает окно списка единиц деления.""")])

# --- helpers.go: searchIDs заменён Search ----------------------------------
p = "internal/store/sqlstore/helpers.go"
s = open(p, encoding="utf-8").read()
a = s.find("// searchIDs возвращает")
b = s.find("// wrapSave дополняет")
if a < 0 or b < 0:
    raise SystemExit(f"{p}: не найдены маркеры searchIDs/wrapSave")
s = s[:a] + s[b:]
s = s.replace('\t"context"\n', "", 1)
open(p, "w", encoding="utf-8").write(s)

# --- существующий тест индекса переведён на Search ------------------------
edit("internal/store/sqlstore/sqlstore_test.go", [("""		found, err := s.searchIDs(t.Context(), "persons", query)
		if err != nil {
			t.Fatalf("searchIDs %q: %v", query, err)
		}
		if len(found) != 1 || found[0] != "p-1" {
			t.Fatalf("searchIDs %q вернул %v, ожидалось [p-1]", query, found)
		}""", """		found, err := s.Search(t.Context(), query, models.AccessFull, models.Page{})
		if err != nil {
			t.Fatalf("Search %q: %v", query, err)
		}
		if len(found) != 1 || found[0].ID != "p-1" {
			t.Fatalf("Search %q вернул %v, ожидалось [p-1]", query, found)
		}""")])
```

Затем перегенерировать мок и отформатировать:

```bash
(cd internal/store && go generate ./...)
gofmt -w internal/store/sqlstore/*.go
```

- [ ] **Step 3: Рубеж**

Run: `gofmt -l . && go build ./... && go vet ./... && go test ./... -count=1`
Expected: gofmt пусто, сборка и `vet` без ошибок, все пакеты `ok`. Если `go build` жалуется на `web/embed.go` (`pattern all:dist`) — соберите `go build ./internal/... ./cmd/...`.

- [ ] **Step 4: Commit**

```bash
git add -A internal
git commit -m "feat(store): Search по индексу и ChildrenOfDivision"
```

---

### Task 3: Тесты поиска и дочерних делений

**Files:**
- Create: `internal/store/sqlstore/search_test.go`, `internal/store/sqlstore/children_test.go`

**Interfaces:**
- Consumes: Task 2; `fullChain` (`delete_test.go`), `batchDivision`, `legacyGetDivision` (`batch_test.go`, `legacy_test.go`), `newStore`, `mustDo`, `withTimeout`, `canceled`, `seedSource`, `scanRows`, `entityTables`, `typeOfTable`.
- Produces: `searchAll`, `explainDetails`, `scanExplain`, `seedTree`, `childIDs`; тесты `TestSearchPrefixIsCaseAndYoInsensitive`, `TestSearchEveryIndexedKindHasLabelAndField`, `TestSearchHitLabelsCoverIndexedKinds`, `TestSearchAccessHidesPrivateEntities`, `TestSearchOrderAndPagesAreStable`, `TestSearchTreatsWildcardsLiterally`, `TestSearchFollowsSaveAndDelete`, `TestSearchCanceledContext`, `TestSearchQueryUsesTermIndex`, `TestChildrenOfDivision`, `TestChildrenOfDivisionMissingParent`, `TestChildrenOfDivisionPagesAndAccess`, `TestChildrenOfDivisionInTxAndCanceled`, `TestChildrenQueryUsesParentIndex`.

- [ ] **Step 1: Создать тесты**

Создать `internal/store/sqlstore/search_test.go`:

```go
package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

func searchAll(t *testing.T, s *Store, query string, access models.Access, page models.Page) []models.Hit {
	t.Helper()

	hits, err := s.Search(t.Context(), query, access, page)
	mustDo(t, "search "+query, err)

	if hits == nil {
		t.Fatalf("Search %q вернул nil, ожидался пустой срез", query)
	}

	return hits
}

// TestSearchPrefixIsCaseAndYoInsensitive: префикс нормализуется как термины.
func TestSearchPrefixIsCaseAndYoInsensitive(t *testing.T) {
	s := newStore(t)

	mustDo(t, "person", s.SavePerson(t.Context(), &models.Person{
		ID: "p-1",
		Names: []models.PersonName{{
			Surname: models.TextRef{Text: "Иванов"}, Given: models.TextRef{Text: "Пётр"},
			Patronymic: models.TextRef{Text: "Сергеевич"},
		}},
	}))

	for _, q := range []string{"иван", "ИВАН", "Иванов", "петр", "ПЁТР", "сергеев", "  иван  "} {
		hits := searchAll(t, s, q, models.AccessFull, models.Page{})

		want := []models.Hit{{Type: models.TypePerson, ID: "p-1", Label: "Иванов Пётр Сергеевич", Field: "name"}}
		if !reflect.DeepEqual(hits, want) {
			t.Fatalf("Search %q = %+v, ожидалось %+v", q, hits, want)
		}
	}

	for _, q := range []string{"сидоров", "ванов", "иванова", "", "   "} {
		if hits := searchAll(t, s, q, models.AccessFull, models.Page{}); len(hits) != 0 {
			t.Errorf("Search %q = %+v, ожидался пустой результат", q, hits)
		}
	}
}

// TestSearchEveryIndexedKindHasLabelAndField: по одному поиску на каждый вид,
// пишущий термины в индекс (все, кроме relations и residences): тип, подпись и
// поле осмысленны.
func TestSearchEveryIndexedKindHasLabelAndField(t *testing.T) {
	s := newStore(t)

	for _, st := range fullChain() {
		mustDo(t, "save "+st.kind+" "+string(st.id), st.save(t.Context(), s))
	}

	cases := []struct {
		query string
		want  models.Hit
	}{
		{"иванов", models.Hit{Type: models.TypePerson, ID: "p-1", Label: "Иванов Пётр Сергеевич", Field: "name"}},
		{"ивановы", models.Hit{Type: models.TypeFamily, ID: "fam-1", Label: "Ивановы", Field: "name"}},
		{"иванофф", models.Hit{Type: models.TypeSurname, ID: "sur-1", Label: "Иванов", Field: "name"}},
		{"пётр", models.Hit{Type: models.TypeGivenName, ID: "giv-1", Label: "Пётр", Field: "name"}},
		{"сергиевич", models.Hit{Type: models.TypePatronymic, ID: "pat-1", Label: "Сергеевич", Field: "name"}},
		{"крестьяне", models.Hit{Type: models.TypeEstate, ID: "est-1", Label: "крестьянин", Field: "name"}},
		{"унтер-офицер", models.Hit{Type: models.TypeTitle, ID: "tit-1", Label: "унтер-офицер", Field: "name"}},
		{"давыдово", models.Hit{Type: models.TypeAdministrativeDivision, ID: "ad-1", Label: "Давыдово", Field: "name"}},
		{"никольская", models.Hit{Type: models.TypeChurch, ID: "chu-1", Label: "Никольская", Field: "name"}},
		{"никольский", models.Hit{Type: models.TypeParish, ID: "par-1", Label: "Никольский", Field: "name"}},
		{"мк давыдово", models.Hit{Type: models.TypeSource, ID: "src-1", Label: "МК Давыдово", Field: "title"}},
		{"л. 12", models.Hit{Type: models.TypeCitation, ID: "cit-1", Label: "л. 12 об.", Field: "text"}},
		{"рга", models.Hit{Type: models.TypeRepository, ID: "rep-2", Label: "РГАДА", Field: "name"}},
		{"цга москв", models.Hit{Type: models.TypeRepository, ID: "rep-1", Label: "ЦГА Москвы", Field: "name"}},
		{"цгам", models.Hit{Type: models.TypeArchive, ID: "arc-1", Label: "ЦГАМ", Field: "name"}},
		{"203", models.Hit{Type: models.TypeArchiveNode, ID: "node-1", Label: "203 Консистория", Field: "name"}},
		{"мк 1881", models.Hit{Type: models.TypeArchiveDocument, ID: "doc-1", Label: "МК 1881", Field: "title"}},
		{"1.jpg", models.Hit{Type: models.TypeAttachment, ID: "att-1", Label: "1.jpg", Field: "filename"}},
		{"род иван", models.Hit{Type: models.TypeNote, ID: "note-book", Label: "Род Ивановых", Field: "title"}},
	}

	covered := map[models.Type]bool{models.TypeEvent: true} // событие — отдельно ниже

	for _, c := range cases {
		hits := searchAll(t, s, c.query, models.AccessFull, models.Page{Limit: 100})

		found := false

		for _, h := range hits {
			found = found || h == c.want
		}

		if !found {
			t.Errorf("Search %q = %+v, не содержит %+v", c.query, hits, c.want)
		}

		covered[c.want.Type] = true
	}

	// все виды, пишущие термины в индекс, проверены (relations и residences
	// терминов не пишут)
	for _, typ := range models.AllTypes() {
		if typ == models.TypeRelation || typ == models.TypeResidence {
			continue
		}

		if !covered[typ] {
			t.Errorf("вид %s не проверен поиском", typ)
		}
	}

	// событие: подпись — тип и место
	events := searchAll(t, s, "давыд", models.AccessFull, models.Page{Limit: 100})

	wantEvent := models.Hit{Type: models.TypeEvent, ID: "ev-1", Label: "birth Давыдово", Field: "place"}

	var gotEvent bool

	for _, h := range events {
		gotEvent = gotEvent || h == wantEvent
	}

	if !gotEvent {
		t.Errorf("Search \"давыд\" = %+v, не содержит %+v", events, wantEvent)
	}
}

// TestSearchHitLabelsCoverIndexedKinds: у каждой таблицы, пишущей термины в
// индекс, есть подпись (кроме персон — своя загрузка — и таблиц без терминов).
func TestSearchHitLabelsCoverIndexedKinds(t *testing.T) {
	noTerms := map[string]bool{"relations": true, "residences": true}

	for _, table := range entityTables {
		if table == "persons" || noTerms[table] {
			continue
		}

		if _, ok := hitLabelExprs[table]; !ok {
			t.Errorf("у таблицы %s нет выражения подписи в hitLabelExprs", table)
		}
	}

	for table := range hitLabelExprs {
		if _, ok := typeOfTable[table]; !ok {
			t.Errorf("hitLabelExprs содержит %s — не таблицу сущности", table)
		}
	}
}

// TestSearchAccessHidesPrivateEntities: приватность решается по таблице
// сущности; неизвестный режим — как публичный.
func TestSearchAccessHidesPrivateEntities(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()

	mustDo(t, "p-1", s.SavePerson(ctx, &models.Person{ID: "p-1", Private: true,
		Names: []models.PersonName{{Surname: models.TextRef{Text: "Иванов"}}}}))
	mustDo(t, "p-2", s.SavePerson(ctx, &models.Person{ID: "p-2",
		Names: []models.PersonName{{Surname: models.TextRef{Text: "Иванова"}}}}))
	mustDo(t, "division", s.SaveAdministrativeDivision(ctx, &models.AdministrativeDivision{
		ID: "ad-1", Name: "Ивановка", Type: models.AdminDivisionDerevnya}))
	mustDo(t, "note", s.SaveNote(ctx, &models.Note{ID: "n-1", Kind: models.NoteKindBook, Title: "Иванов род", Private: true}))

	ids := func(access models.Access) []models.ID {
		var out []models.ID
		for _, h := range searchAll(t, s, "иванов", access, models.Page{}) {
			out = append(out, h.ID)
		}

		return out
	}

	cases := []struct {
		name   string
		access models.Access
		want   []models.ID
	}{
		{"полный доступ", models.AccessFull, []models.ID{"ad-1", "n-1", "p-1", "p-2"}},
		{"публичный доступ", models.AccessPublic, []models.ID{"ad-1", "p-2"}},
		{"неизвестный режим — как публичный", models.Access(9), []models.ID{"ad-1", "p-2"}},
	}

	for _, c := range cases {
		if got := ids(c.access); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: %v, ожидалось %v", c.name, got, c.want)
		}
	}

	// окно считается после фильтра приватного
	page := searchAll(t, s, "иванов", models.AccessPublic, models.Page{Limit: 1, Offset: 1})
	if len(page) != 1 || page[0].ID != "p-2" {
		t.Errorf("второе публичное окно %+v, ожидалась p-2", page)
	}
}

// TestSearchOrderAndPagesAreStable: одна запись на сущность (два совпавших
// термина не дублируют), порядок — по виду и id, окна не пересекаются.
func TestSearchOrderAndPagesAreStable(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()

	for _, id := range []models.ID{"p-c", "p-a", "p-b"} {
		mustDo(t, "person", s.SavePerson(ctx, &models.Person{ID: id,
			Names: []models.PersonName{{Surname: models.TextRef{Text: "Иванов"}, Given: models.TextRef{Text: "Иван"}}}}))
	}

	mustDo(t, "division", s.SaveAdministrativeDivision(ctx, &models.AdministrativeDivision{
		ID: "ad-1", Name: "Иваново", Type: models.AdminDivisionSelo, Variants: []string{"Ивановское"}}))

	all := searchAll(t, s, "иван", models.AccessFull, models.Page{})

	var gotIDs []models.ID
	for _, h := range all {
		gotIDs = append(gotIDs, h.ID)
	}

	// «Иванов» и «Иван» у персоны — два термина, но запись одна
	if want := []models.ID{"ad-1", "p-a", "p-b", "p-c"}; !reflect.DeepEqual(gotIDs, want) {
		t.Fatalf("порядок и состав %v, ожидалось %v", gotIDs, want)
	}

	var paged []models.Hit

	for off := 0; off < len(all)+1; off++ {
		w := searchAll(t, s, "иван", models.AccessFull, models.Page{Limit: 1, Offset: off})
		paged = append(paged, w...)
	}

	if !reflect.DeepEqual(paged, all) {
		t.Fatalf("окна по одному в сумме %+v, полный список %+v", paged, all)
	}
}

// TestSearchTreatsWildcardsLiterally: %, _ и \ в запросе — обычные символы.
func TestSearchTreatsWildcardsLiterally(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()

	for id, name := range map[models.ID]string{"ad-1": "100%_вода", "ad-2": "1000 вёрст", "ad-3": `a\b`} {
		mustDo(t, "division", s.SaveAdministrativeDivision(ctx, &models.AdministrativeDivision{
			ID: id, Name: name, Type: models.AdminDivisionDerevnya}))
	}

	cases := map[string][]models.ID{
		"100%":  {"ad-1"},
		"100%_": {"ad-1"},
		"100_":  nil, // «_» — обычный символ, а не «любой символ»: после «100» стоит «%»
		"100":   {"ad-1", "ad-2"},
		"%":     nil,
		"_":     nil,
		`a\`:    {"ad-3"},
	}

	for q, want := range cases {
		var got []models.ID
		for _, h := range searchAll(t, s, q, models.AccessFull, models.Page{}) {
			got = append(got, h.ID)
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("Search %q = %v, ожидалось %v", q, got, want)
		}
	}
}

// TestSearchFollowsSaveAndDelete: индекс обновляется при пересохранении и
// очищается при удалении; внутри InTx виден незафиксированный термин.
func TestSearchFollowsSaveAndDelete(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)

	mustDo(t, "save", s.SaveAdministrativeDivision(ctx, &models.AdministrativeDivision{
		ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionDerevnya}))
	mustDo(t, "rename", s.SaveAdministrativeDivision(ctx, &models.AdministrativeDivision{
		ID: "ad-1", Name: "Никифорово", Type: models.AdminDivisionDerevnya}))

	if hits := searchAll(t, s, "давыд", models.AccessFull, models.Page{}); len(hits) != 0 {
		t.Errorf("старое имя ещё находится: %+v", hits)
	}

	if hits := searchAll(t, s, "никиф", models.AccessFull, models.Page{}); len(hits) != 1 {
		t.Errorf("новое имя не находится: %+v", hits)
	}

	mustDo(t, "in tx", s.InTx(ctx, func(tx store.Store) error {
		if err := tx.SaveAdministrativeDivision(ctx, &models.AdministrativeDivision{
			ID: "ad-2", Name: "Хомутово", Type: models.AdminDivisionDerevnya}); err != nil {
			return err
		}

		hits, err := tx.Search(ctx, "хому", models.AccessFull, models.Page{})
		if err != nil || len(hits) != 1 {
			t.Errorf("внутри InTx: %+v, %v; ожидалась одна запись", hits, err)
		}

		return nil
	}))

	mustDo(t, "delete", s.DeleteAdministrativeDivision(ctx, "ad-1"))

	if hits := searchAll(t, s, "никиф", models.AccessFull, models.Page{}); len(hits) != 0 {
		t.Errorf("удалённое деление находится: %+v", hits)
	}
}

// TestSearchCanceledContext: отменённый контекст — ошибка контекста.
func TestSearchCanceledContext(t *testing.T) {
	s := newStore(t)

	if _, err := s.Search(canceled(t), "иван", models.AccessFull, models.Page{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("Search с отменённым ctx = %v, ожидалось context.Canceled", err)
	}
}

// explainDetails возвращает строки плана запроса.
func explainDetails(t *testing.T, s *Store, query string, args ...any) string {
	t.Helper()

	rows, err := scanRows(s.run(t.Context()), scanExplain, `EXPLAIN QUERY PLAN `+query, args...)
	mustDo(t, "explain", err)

	return strings.Join(rows, "\n")
}

// TestSearchQueryUsesTermIndex: префиксный запрос идёт по idx_search_term
// (покрывающему), а не по полному просмотру search_index.
func TestSearchQueryUsesTermIndex(t *testing.T) {
	s := newStore(t)

	g, err := s.graph(t.Context())
	mustDo(t, "graph", err)

	var private []string
	for table := range g.private {
		private = append(private, table)
	}

	sort.Strings(private)

	for name, tables := range map[string][]string{"полный доступ": nil, "публичный доступ": private} {
		plan := explainDetails(t, s, searchSQL(tables), "иван", "иван"+maxRune, 50, 0)

		if !strings.Contains(plan, "idx_search_term") {
			t.Errorf("%s: план не использует idx_search_term:\n%s", name, plan)
		}

		if !strings.Contains(plan, "COVERING INDEX idx_search_term") {
			t.Errorf("%s: индекс должен быть покрывающим:\n%s", name, plan)
		}

		if strings.Contains(plan, "SCAN si") || strings.Contains(plan, "SCAN search_index") {
			t.Errorf("%s: план содержит полный просмотр search_index:\n%s", name, plan)
		}
	}
}

// scanExplain читает столбец detail плана запроса.
func scanExplain(r *sql.Rows) (string, error) {
	var id, parent, unused int
	var detail string
	err := r.Scan(&id, &parent, &unused, &detail)

	return detail, err
}
```

Создать `internal/store/sqlstore/children_test.go`:

```go
package sqlstore

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// seedTree сохраняет дерево делений: корень ad-root с детьми ad-1..ad-3 и внуком
// ad-11 у ad-1 (полностью заполненные единицы — сверка с эталоном).
func seedTree(t *testing.T, s *Store) {
	t.Helper()

	seedSource(t, s)
	mustDo(t, "citation", s.SaveCitation(t.Context(), &models.Citation{ID: "cit-1", SourceID: "src-1"}))

	root := models.ID("ad-root")
	ad1 := models.ID("ad-0001")

	mustDo(t, "root", s.SaveAdministrativeDivision(t.Context(), &models.AdministrativeDivision{
		ID: root, Name: "Московская", Type: models.AdminDivisionGovernorate,
	}))

	for _, i := range []int{1, 2, 3} {
		mustDo(t, "child", s.SaveAdministrativeDivision(t.Context(), batchDivision(i, true, &root)))
	}

	mustDo(t, "grandchild", s.SaveAdministrativeDivision(t.Context(), batchDivision(11, true, &ad1)))
}

func childIDs(t *testing.T, s *Store, parent models.ID, access models.Access, page models.Page) []models.ID {
	t.Helper()

	got, err := s.ChildrenOfDivision(t.Context(), parent, access, page)
	mustDo(t, "children of "+string(parent), err)

	if got == nil {
		t.Fatalf("ChildrenOfDivision(%s) вернул nil, ожидался пустой срез", parent)
	}

	ids := []models.ID{}
	for _, d := range got {
		ids = append(ids, d.ID)
	}

	return ids
}

// TestChildrenOfDivision: только прямые дети, в порядке сохранения, полностью
// заполненные (как поштучное чтение), листья — пустой не-nil срез.
func TestChildrenOfDivision(t *testing.T) {
	s := newStore(t)
	seedTree(t, s)

	cases := []struct {
		parent models.ID
		want   []models.ID
	}{
		{"ad-root", []models.ID{"ad-0001", "ad-0002", "ad-0003"}},
		{"ad-0001", []models.ID{"ad-0011"}},
		{"ad-0011", []models.ID{}},
		{"ad-0002", []models.ID{}},
	}

	for _, c := range cases {
		if got := childIDs(t, s, c.parent, models.AccessFull, models.Page{}); !reflect.DeepEqual(got, c.want) {
			t.Errorf("дети %s: %v, ожидалось %v", c.parent, got, c.want)
		}
	}

	children, err := s.ChildrenOfDivision(t.Context(), "ad-root", models.AccessFull, models.Page{})
	mustDo(t, "children", err)

	for _, d := range children {
		want, err := legacyGetDivision(s, t.Context(), d.ID)
		mustDo(t, "legacy "+string(d.ID), err)

		if !reflect.DeepEqual(d, want) {
			t.Fatalf("ребёнок %s расходится с поштучной загрузкой:\n got  %+v\n want %+v", d.ID, d, want)
		}
	}
}

// TestChildrenOfDivisionMissingParent: нет такого деления — ErrNotFound (а не
// пустой список: обработчик отличает «нет деления» от «нет детей»).
func TestChildrenOfDivisionMissingParent(t *testing.T) {
	s := newStore(t)

	if _, err := s.ChildrenOfDivision(t.Context(), "nope", models.AccessFull, models.Page{}); !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("ChildrenOfDivision(nope) = %v, ожидалось models.ErrNotFound", err)
	}
}

// TestChildrenOfDivisionPagesAndAccess: окна не пересекаются и покрывают
// набор; режим доступа на деления не влияет.
func TestChildrenOfDivisionPagesAndAccess(t *testing.T) {
	s := newStore(t)
	seedTree(t, s)

	all := childIDs(t, s, "ad-root", models.AccessFull, models.Page{})

	var paged []models.ID

	for off := 0; off < len(all)+1; off += 2 {
		paged = append(paged, childIDs(t, s, "ad-root", models.AccessFull, models.Page{Limit: 2, Offset: off})...)
	}

	if !reflect.DeepEqual(paged, all) {
		t.Fatalf("окна по 2 в сумме %v, полный список %v", paged, all)
	}

	if got := childIDs(t, s, "ad-root", models.AccessPublic, models.Page{}); !reflect.DeepEqual(got, all) {
		t.Fatalf("публичный доступ вернул %v, ожидалось %v (у делений нет флага приватности)", got, all)
	}
}

// TestChildrenOfDivisionInTxAndCanceled: внутри InTx видны незафиксированные
// дети; отменённый контекст — ошибка контекста.
func TestChildrenOfDivisionInTxAndCanceled(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)

	mustDo(t, "root", s.SaveAdministrativeDivision(ctx, &models.AdministrativeDivision{
		ID: "ad-root", Name: "Московская", Type: models.AdminDivisionGovernorate}))

	mustDo(t, "in tx", s.InTx(ctx, func(tx store.Store) error {
		root := models.ID("ad-root")
		if err := tx.SaveAdministrativeDivision(ctx, &models.AdministrativeDivision{
			ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionDerevnya, ParentID: &root}); err != nil {
			return err
		}

		got, err := tx.ChildrenOfDivision(ctx, "ad-root", models.AccessFull, models.Page{})
		if err != nil || len(got) != 1 {
			t.Errorf("внутри InTx: %v, %v; ожидался один ребёнок", got, err)
		}

		return nil
	}))

	if _, err := s.ChildrenOfDivision(canceled(t), "ad-root", models.AccessFull, models.Page{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("ChildrenOfDivision с отменённым ctx = %v, ожидалось context.Canceled", err)
	}
}

// TestChildrenQueryUsesParentIndex: выборка id детей идёт по индексу parent_id
// (созданному генерацией FK-индексов) без сортировки.
func TestChildrenQueryUsesParentIndex(t *testing.T) {
	s := newStore(t)

	plan := explainDetails(t, s, pagedIDsSQL("administrative_divisions", "parent_id = ?"), "ad-root", 50, 0)

	if !strings.Contains(plan, "idx_administrative_divisions_parent_id") {
		t.Errorf("план не использует индекс по parent_id:\n%s", plan)
	}

	if strings.Contains(plan, "TEMP B-TREE") {
		t.Errorf("план содержит сортировку:\n%s", plan)
	}
}
```

Выполнить `gofmt -w` на обоих.

- [ ] **Step 2: Прогнать**

Run: `gofmt -l . && go vet ./... && go test ./... -count=1`
Expected: gofmt пусто, `vet` чисто, все пакеты `ok`.

- [ ] **Step 3: Мутационные проверки (вручную; каждую откатить сразу: `git checkout -- <файл>`)**

Запускать `go test ./internal/store/sqlstore/ ./internal/storage/ -count=1 -timeout 100s`; указанный тест должен ПРОВАЛИТЬСЯ.

| Мутация | Ожидаемый провал |
|---|---|
| в `Search` (`search.go`) `if access != models.AccessFull {` → `if false {` | `TestSearchAccessHidesPrivateEntities` |
| в `Search` убрать верхнюю границу: ` AND si.term < ?` из `searchSQL` и аргумент `prefix+maxRune` | `TestSearchPrefixIsCaseAndYoInsensitive`, `TestSearchTreatsWildcardsLiterally` |
| в `loadHitLabels` `out[id] = labels[0] // первое имя персоны` → `labels[len(labels)-1]` | `TestSearchEveryIndexedKindHasLabelAndField` |
| в `schema.go` удалить оператор `CREATE INDEX … idx_search_term` | `TestSearchQueryUsesTermIndex`, `TestSearchTermIndex` |
| в `ChildrenOfDivision` (`places.go`) удалить проверку существования родителя | `TestChildrenOfDivisionMissingParent` |
| в `ChildrenOfDivision` условие `"parent_id = ?", []any{string(parent)}` → `"", nil` | `TestChildrenOfDivision` |
| в `hitLabelExprs` подпись события → `"type"` (без места) | `TestSearchEveryIndexedKindHasLabelAndField` |

- [ ] **Step 4: Commit**

```bash
git add internal/store/sqlstore/search_test.go internal/store/sqlstore/children_test.go
git commit -m "test(store): поиск по индексу и дочерние деления"
```

---

### Task 4: Документация

**Files:**
- Modify: `docs/data-model/core-read-write.md` (§4), `docs/plans/2026-09-20-core-rw-roadmap.md` (S11)

- [ ] **Step 1: Спецификация §4**

В `docs/data-model/core-read-write.md` заменить

```
- `Search(ctx, query, Access, Page) ([]Hit, error)` — по `search_index`, префикс
  нормализованного запроса, порядок стабильный.
- Точечные запросы: `ChildrenOfDivision(ctx, parent, Access, Page)`; остальные
  (`EventsOfPerson`, `RelationsOfPerson`, …) вводятся вместе со срезами
  соответствующих сущностей.
```

на

```
- `Search(ctx, query, Access, Page) ([]Hit, error)` — по `search_index`, префикс
  нормализованного запроса, порядок стабильный (вид, id). Реализовано в S11:
  диапазон `term >= p AND term < p+U+10FFFF` по покрывающему индексу
  `idx_search_term` (не `LIKE`: с `ESCAPE` индекс не используется); совпадение —
  начало одного термина, многословный запрос на слова не разбивается; одна
  сущность — один `Hit` (`Field` — первое по алфавиту совпавшее поле, `Label` —
  подпись сущности, при пустой — `ID`). Приватность решается по таблице сущности
  (в `search_index` колонки `private` нет).
- Точечные запросы: `ChildrenOfDivision(ctx, parent, Access, Page)` — прямые
  дети деления в порядке сохранения, нет родителя — `ErrNotFound` (S11);
  остальные (`EventsOfPerson`, `RelationsOfPerson`, …) вводятся вместе со срезами
  соответствующих сущностей.
```

- [ ] **Step 2: Дорожная карта**

В `docs/plans/2026-09-20-core-rw-roadmap.md` в блоке `### S11.` заменить строки

```
- **Файлы:** `models/query.go` (`Hit`), `store/deps.go`, `sqlstore/search.go`,
  `storage/schema.go` (`idx_search_term`), тесты.
```

на

```
- **Файлы:** `models/query.go` (`Hit`), `store/deps.go`, `sqlstore/search.go`,
  `sqlstore/places.go` (`ChildrenOfDivision`), `storage/schema.go`
  (`idx_search_term`), тесты (`search_test.go`, `children_test.go`,
  `storage/indexes_test.go`); план — `2026-09-21-core-rw-s11-search.md`.
```

а абзац

```
- **Предпосылка из S9:** в `search_index` нет колонки `private`, поэтому
  `Access` в `Search` нельзя решить через `schemaGraph.hasPrivate` — нужен
  join на таблицу сущности по `entity_table` (или колонка в индексе).
```

на

```
- **Предпосылка из S9 (закрыта):** приватность в `Search` решается по таблице
  сущности — для `AccessPublic` в запрос добавляются условия
  `NOT (entity_table = '<t>' AND EXISTS (… private = 1))` по таблицам с колонкой
  `private` (набор берётся из графа схемы).
```

- [ ] **Step 3: Commit**

```bash
git add docs/data-model/core-read-write.md docs/plans/2026-09-20-core-rw-roadmap.md
git commit -m "docs: S11 — Search и ChildrenOfDivision"
```

---

## Правки по итогам ревью (внесены после выполнения задач)

Код в репозитории — источник истины; плановые блоки выше описывают первую версию. Отличия:

- Порядок `Search` — по имени таблицы сущности и id (а не по `models.Type`): порядок стабилен и дешевле; в комментариях порта, `searchSQL` и спецификации так и названо («таблица сущности, id»), в ограничениях плана выше — «(вид сущности, id)» надо читать так же.
- `searchSQL`: имена приватных таблиц заключены в двойные кавычки; комментарий схемы у `search_index` говорит про диапазон по `term`, а не про `LIKE`.
- `places.go`: условие выборки детей вынесено в константу `childrenWhere` (её же использует тест плана).
- `children_test.go`: дети сохраняются в порядке 3, 1, 2 (не совпадает с порядком id), поэтому `ORDER BY id` вместо `ORDER BY rowid` тест ловит; из `TestSearchQueryUsesTermIndex` убраны пустая на старых SQLite отрицательная проверка и дублирующая проверка имени индекса.
