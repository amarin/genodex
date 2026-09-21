# S5: Индексы — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Создавать индекс на каждую FK-колонку схемы, у которой нет собственного ведущего индекса, — генерацией по `PRAGMA foreign_key_list` при открытии БД, без ручного перечисления.

**Architecture:** Новый файл `internal/storage/indexes.go`: функция `createFKIndexes(d *sql.DB)` после выполнения DDL схемы обходит все таблицы из `sqlite_master`, для каждой читает FK-колонки и колонки, уже ведущие какой-либо индекс (включая автоиндексы первичных ключей), и создаёт недостающие `CREATE INDEX IF NOT EXISTS idx_<таблица>_<колонка>`. Операция идемпотентна, миграций данных не требует. Индекс поиска `search_index(term)` — отдельный этап S11.

**Tech Stack:** Go 1.26.4, `modernc.org/sqlite` (табличные функции `pragma_foreign_key_list`, `pragma_index_list`, `pragma_index_info`).

**Spec:** `docs/data-model/core-read-write.md` §3; `docs/plans/2026-09-20-core-rw-roadmap.md` (этап S5).

## Global Constraints

- Индекс — на каждую FK-колонку каждой таблицы, у которой нет ведущего индекса (владельцы дочерних таблиц, ссылки на `text_refs`/`dates`/`anchors`, строгие ссылки между сущностями, `citation_id` в `source_links`). Список **генерируется** из схемы, а не перечисляется вручную.
- Имя индекса — `idx_<таблица>_<колонка>`; уже существующий явный индекс `idx_source_links_target` сохраняется без изменений.
- Идемпотентность: повторный `OpenDB` на той же БД не создаёт дубликатов и не падает; схема остаётся `schema_version = 0`, миграций данных нет.
- Соединение одно (`SetMaxOpenConns(1)`): результаты каждого запроса читаются целиком и закрываются до следующего запроса и до DDL — вложенные запросы при открытом курсоре зависают.
- Идентификаторы в DDL подставляются в текст запроса только после экранирования (`"` → `""`); значения — параметрами.
- Индекс `search_index(term)` не входит в этап (S11); поведение `sqlstore` не меняется.
- Комментарии и тексты ошибок — на русском; gofmt-clean; один файл — одна ответственность.
- Проверка рубежа: `gofmt -l .` пусто, `go build ./...`, `go vet ./...`, `go test ./...`.

---

### Task 1: Генерация FK-индексов

**Files:**
- Create: `internal/storage/indexes.go`
- Modify: `internal/storage/db.go` (вызов после DDL)
- Test: `internal/storage/indexes_test.go`

**Interfaces:**
- Consumes: `schemaDDL`, `OpenDB`, `openTestDB` (`db_test.go`), `DB.Query`/`Exec`/`Close`.
- Produces: `func createFKIndexes(d *sql.DB) error`; внутренние `queryStrings(d *sql.DB, query string, args ...any) ([]string, error)`, `quoteIdent(s string) string`; тестовые хелперы (файл `indexes_test.go`): `pragmaRows(t, db, query) [][]any`, `fkColumns(t, db, table) []string`, `leadingIndexes(t, db, table) map[string][]string`, `allTables(t, db) []string`.

- [ ] **Step 1: Написать падающие тесты**

`internal/storage/indexes_test.go`:

```go
package storage

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// pragmaRows выполняет запрос и возвращает строки как срезы значений.
func pragmaRows(t *testing.T, db *DB, query string, args ...any) [][]any {
	t.Helper()

	rows, err := db.Query(query, args...)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		t.Fatal(err)
	}

	var out [][]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			t.Fatal(err)
		}
		out = append(out, vals)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}

	return out
}

func allTables(t *testing.T, db *DB) []string {
	t.Helper()

	var names []string
	for _, r := range pragmaRows(t, db,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`) {
		names = append(names, fmt.Sprint(r[0]))
	}

	return names
}

// fkColumns возвращает FK-колонки таблицы классическим PRAGMA (независимо от
// табличных функций, которыми пользуется реализация).
func fkColumns(t *testing.T, db *DB, table string) []string {
	t.Helper()

	var cols []string
	for _, r := range pragmaRows(t, db, fmt.Sprintf("PRAGMA foreign_key_list(%s)", quoteIdent(table))) {
		cols = append(cols, fmt.Sprint(r[3])) // колонки: id, seq, table, from, to, …
	}

	return cols
}

// leadingIndexes возвращает для каждой колонки таблицы имена индексов, у
// которых она первая (включая автоиндексы первичных и уникальных ключей).
func leadingIndexes(t *testing.T, db *DB, table string) map[string][]string {
	t.Helper()

	out := map[string][]string{}
	for _, il := range pragmaRows(t, db, fmt.Sprintf("PRAGMA index_list(%s)", quoteIdent(table))) {
		name := fmt.Sprint(il[1]) // колонки: seq, name, unique, origin, partial
		info := pragmaRows(t, db, fmt.Sprintf("PRAGMA index_info(%s)", quoteIdent(name)))
		if len(info) == 0 {
			continue
		}
		col := fmt.Sprint(info[0][2]) // колонки: seqno, cid, name; первая строка — ведущая колонка
		out[col] = append(out[col], name)
	}

	return out
}

func TestForeignKeysAreIndexed(t *testing.T) {
	db := openTestDB(t)

	checked := 0
	for _, table := range allTables(t, db) {
		leading := leadingIndexes(t, db, table)
		for _, col := range fkColumns(t, db, table) {
			checked++
			if len(leading[col]) == 0 {
				t.Errorf("%s.%s: у FK-колонки нет ведущего индекса", table, col)
			}
		}
	}

	// Защита от пустой проверки: в схеме больше сотни FK-колонок.
	if checked < 100 {
		t.Errorf("проверено FK-колонок: %d, ожидается не меньше 100", checked)
	}
}

func TestForeignKeyIndexesNotDuplicated(t *testing.T) {
	db := openTestDB(t)

	for _, table := range allTables(t, db) {
		leading := leadingIndexes(t, db, table)
		for _, col := range fkColumns(t, db, table) {
			if n := len(leading[col]); n > 1 {
				t.Errorf("%s.%s: %d ведущих индексов (%v), ожидается один", table, col, n, leading[col])
			}
		}
	}
}

func TestForeignKeyIndexNaming(t *testing.T) {
	db := openTestDB(t)

	for _, want := range []struct{ table, name string }{
		{"person_names", "idx_person_names_person_id"},
		{"person_names", "idx_person_names_surname_id"},
		{"source_links", "idx_source_links_citation_id"},
		// явный индекс, созданный DDL схемы, сохраняется
		{"source_links", "idx_source_links_target"},
	} {
		var n int
		if err := db.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type = 'index' AND tbl_name = ? AND name = ?`,
			want.table, want.name,
		).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Errorf("индекс %s на %s: найдено %d, ожидается 1", want.name, want.table, n)
		}
	}
}

func indexNames(t *testing.T, db *DB) []string {
	t.Helper()

	var names []string
	for _, r := range pragmaRows(t, db, `SELECT name FROM sqlite_master WHERE type = 'index' ORDER BY name`) {
		names = append(names, fmt.Sprint(r[0]))
	}
	sort.Strings(names)

	return names
}

func TestForeignKeyIndexesIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "genealogy.db")

	first, err := OpenDB(path)
	if err != nil {
		t.Fatal(err)
	}
	before := indexNames(t, first)
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second, err := OpenDB(path)
	if err != nil {
		t.Fatalf("повторное открытие: %v", err)
	}
	defer second.Close()

	after := indexNames(t, second)
	if strings.Join(before, ",") != strings.Join(after, ",") {
		t.Errorf("набор индексов изменился при повторном открытии:\nдо    %v\nпосле %v", before, after)
	}
	if len(before) < 100 {
		t.Errorf("индексов %d, ожидается не меньше 100 (FK-индексы + явные)", len(before))
	}
}

// Колонки, у которых уже есть ведущий индекс (первичный ключ, явный индекс,
// UNIQUE), новыми индексами не дублируются; вторая колонка составного индекса
// ведущей не считается. В боевой схеме такого случая нет, поэтому ветка
// проверяется на игрушечной БД.
func TestCreateFKIndexesSkipsCoveredColumns(t *testing.T) {
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	d.SetMaxOpenConns(1)

	for _, q := range []string{
		`CREATE TABLE parent (id TEXT PRIMARY KEY)`,
		// FK-колонка — первичный ключ: автоиндекс уже есть
		`CREATE TABLE child_pk (owner TEXT PRIMARY KEY REFERENCES parent(id))`,
		// FK-колонка без индекса
		`CREATE TABLE child_bare (owner TEXT REFERENCES parent(id), n INTEGER)`,
		// явный индекс, ведущий FK-колонкой
		`CREATE TABLE child_explicit (owner TEXT REFERENCES parent(id))`,
		`CREATE INDEX my_owner_idx ON child_explicit (owner)`,
		// UNIQUE, ведущий FK-колонкой
		`CREATE TABLE child_unique (x TEXT, y TEXT REFERENCES parent(id), UNIQUE (y, x))`,
		// FK-колонка вторая в UNIQUE: ведущего индекса нет
		`CREATE TABLE child_second (a TEXT, b TEXT REFERENCES parent(id), UNIQUE (a, b))`,
	} {
		if _, err := d.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}

	for i := 0; i < 2; i++ { // второй проход — идемпотентность
		if err := createFKIndexes(d); err != nil {
			t.Fatalf("проход %d: %v", i+1, err)
		}
	}

	want := map[string]string{
		"child_pk":       "sqlite_autoindex_child_pk_1",
		"child_bare":     "idx_child_bare_owner",
		"child_explicit": "my_owner_idx",
		"child_unique":   "sqlite_autoindex_child_unique_1",
		"child_second":   "idx_child_second_b,sqlite_autoindex_child_second_1",
	}
	for table, wantNames := range want {
		got, err := queryStrings(d,
			`SELECT name FROM sqlite_master WHERE type = 'index' AND tbl_name = ? ORDER BY name`, table)
		if err != nil {
			t.Fatal(err)
		}
		if joined := strings.Join(got, ","); joined != wantNames {
			t.Errorf("%s: индексы %q, ожидается %q", table, joined, wantNames)
		}
	}
}

// Выборки по владельцу и по ссылке на value-таблицу идут по индексу.
func TestLookupsUseIndexes(t *testing.T) {
	db := openTestDB(t)

	tests := []struct {
		query string
		index string
	}{
		{"SELECT id FROM person_names WHERE person_id = ?", "idx_person_names_person_id"},
		{"SELECT id FROM person_names WHERE surname_id = ?", "idx_person_names_surname_id"},
		{"SELECT id FROM source_links WHERE citation_id = ?", "idx_source_links_citation_id"},
		{"DELETE FROM person_names WHERE person_id = ?", "idx_person_names_person_id"},
	}
	for _, tt := range tests {
		var details []string
		for _, r := range pragmaRows(t, db, "EXPLAIN QUERY PLAN "+tt.query, "x") {
			details = append(details, fmt.Sprint(r[3])) // колонки: id, parent, notused, detail
		}
		plan := strings.Join(details, "; ")
		if !strings.Contains(plan, tt.index) {
			t.Errorf("%s\nплан: %s\nожидается индекс %s", tt.query, plan, tt.index)
		}
	}
}
```

- [ ] **Step 2: Убедиться, что тесты не компилируются**

Run: `go test ./internal/storage`
Expected: FAIL — `undefined: quoteIdent`.

- [ ] **Step 3: Реализация**

`internal/storage/indexes.go`:

```go
package storage

import (
	"database/sql"
	"fmt"
	"strings"
)

// createFKIndexes создаёт индекс на каждую FK-колонку каждой таблицы схемы,
// у которой ещё нет индекса с этой колонкой первой (автоиндексы первичных и
// уникальных ключей и явные индексы DDL учитываются). Индекс на FK-колонке
// нужен и выборке дочерних строк по владельцу, и проверке внешних ключей при
// удалении родителя (иначе SQLite сканирует дочернюю таблицу целиком).
//
// Список генерируется по PRAGMA foreign_key_list, а не перечисляется вручную:
// новая таблица со ссылками получает индексы автоматически. Операция
// идемпотентна (CREATE INDEX IF NOT EXISTS) и не требует миграции данных.
func createFKIndexes(d *sql.DB) error {
	tables, err := queryStrings(d,
		`SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%' ORDER BY name`)
	if err != nil {
		return fmt.Errorf("список таблиц: %w", err)
	}

	for _, table := range tables {
		fkCols, err := queryStrings(d,
			`SELECT "from" FROM pragma_foreign_key_list(?) ORDER BY id, seq`, table)
		if err != nil {
			return fmt.Errorf("внешние ключи %s: %w", table, err)
		}
		if len(fkCols) == 0 {
			continue
		}

		leading, err := queryStrings(d,
			`SELECT ii.name FROM pragma_index_list(?) AS il, pragma_index_info(il.name) AS ii WHERE ii.seqno = 0`,
			table)
		if err != nil {
			return fmt.Errorf("индексы %s: %w", table, err)
		}
		covered := make(map[string]bool, len(leading))
		for _, col := range leading {
			covered[col] = true
		}

		for _, col := range fkCols {
			if covered[col] {
				continue
			}
			ddl := fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s (%s)`,
				quoteIdent("idx_"+table+"_"+col), quoteIdent(table), quoteIdent(col))
			if _, err := d.Exec(ddl); err != nil {
				return fmt.Errorf("индекс %s.%s: %w", table, col, err)
			}
			covered[col] = true
		}
	}

	return nil
}

// queryStrings выполняет запрос и возвращает первый столбец всех строк.
// Курсор закрывается до возврата: соединение одно, вложенные запросы и DDL при
// открытом курсоре зависают.
func queryStrings(d *sql.DB, query string, args ...any) ([]string, error) {
	rows, err := d.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}

	return out, rows.Err()
}

// quoteIdent экранирует идентификатор для подстановки в текст SQL.
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
```

В `internal/storage/db.go`, в `OpenDB`, сразу после цикла `for _, q := range ddl { … }` (перед `INSERT OR IGNORE INTO meta`) добавь:

```go
	if err := createFKIndexes(d); err != nil {
		d.Close()
		return nil, fmt.Errorf("indexes: %w", err)
	}
```

- [ ] **Step 4: Прогнать тесты**

Run: `gofmt -l internal/storage` → пусто.
Run: `go vet ./internal/storage && go test -count=1 ./internal/storage`
Expected: PASS, включая существующие тесты пакета (`TestSchemaCreated` не меняется: индексы — не таблицы).

Run: `go build ./... && go test ./internal/store/...`
Expected: PASS (`sqlstore` работает поверх схемы с индексами без изменений).

- [ ] **Step 5: Commit**

```bash
git add internal/storage/indexes.go internal/storage/indexes_test.go internal/storage/db.go
git commit -m "feat(storage): FK-индексы генерируются из схемы при открытии БД

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

### Task 2: Документация и рубеж

**Files:**
- Modify: `internal/storage/schema.go` (комментарий в шапке)
- Modify: `docs/plans/2026-09-20-core-rw-roadmap.md` (S5)

**Interfaces:**
- Consumes: реализация Task 1.
- Produces: документация без утверждений, противоречащих коду.

- [ ] **Step 1: Комментарий в шапке схемы**

В `internal/storage/schema.go` в шапочный комментарий (после абзаца про `source_links` — «её чистит владелец утверждения по индексу idx_source_links_target.») добавь абзац:

```go
//
// Индексы на FK-колонках не перечисляются в DDL: createFKIndexes (indexes.go)
// создаёт их при открытии БД по PRAGMA foreign_key_list для каждой колонки, у
// которой нет собственного ведущего индекса (idx_<таблица>_<колонка>).
```

- [ ] **Step 2: Дорожная карта**

В `docs/plans/2026-09-20-core-rw-roadmap.md`, раздел S5, замени строку «Файлы»:

```
- **Файлы:** `storage/schema.go` (функция `createIndexes` по
  `PRAGMA foreign_key_list`), тест `storage/db_test.go`.
```

на

```
- **Файлы:** `storage/indexes.go` (`createFKIndexes` по `pragma_foreign_key_list`,
  вызывается из `OpenDB`), тест `storage/indexes_test.go`; план —
  `2026-09-21-core-rw-s05-indexes.md`.
```

(если текст отличается от приведённого, сохрани смысл: файл реализации — `indexes.go`, тест — `indexes_test.go`).

- [ ] **Step 3: Рубеж**

```bash
gofmt -l .            # пусто
go build ./...
go vet ./...
go test ./...
```

Expected: всё зелёное.

- [ ] **Step 4: Commit**

```bash
git add internal/storage/schema.go docs/plans/2026-09-20-core-rw-roadmap.md
git commit -m "docs(storage): описание генерации FK-индексов; файлы этапа S5 в дорожной карте

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

## Самопроверка плана

- **Покрытие спеки §3 (индексы) и roadmap S5:** индекс на каждую FK-колонку без ведущего индекса, генерация из `PRAGMA foreign_key_list` (Task 1); проверка обходом `sqlite_master` независимыми классическими PRAGMA (`TestForeignKeysAreIndexed`, `TestForeignKeyIndexesNotDuplicated`); `EXPLAIN QUERY PLAN` для выборки по владельцу и по ссылке на value-таблицу (`TestLookupsUseIndexes`); идемпотентность (`TestForeignKeyIndexesIdempotent`). Индекс `search_index(term)` — S11.
- **Согласованность имён:** `createFKIndexes`, `queryStrings`, `quoteIdent` (Task 1) используются в `OpenDB` и тестах; хелперы тестов определены в `indexes_test.go`.
- **Не входит в этап:** `idx_search_term` и `Search` (S11), `_pragma`-DSN (S12), чистка при `Delete*` (S7).
