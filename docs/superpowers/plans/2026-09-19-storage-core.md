# Storage Core (SQLite + журнал + бэкап) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Перевести хранение genodex на SQLite (`modernc.org/sqlite`, pure-Go, без cgo) с внешним append-only JSONL-журналом полных образов и командами бэкапа/восстановления/проверки, сохранив рабочие интерфейсы MCP/HTTP.

**Architecture:** Новый модель-агностичный пакет `internal/storage`: SQLite-движок (таблицы `entity(type,id,data,search)` и `meta(applied_seq)`) вместе с `Journal` (append + fsync). Инвариант записи: сначала журнал (fsync), затем SQLite-commit (WAL); при старте БД догоняется из журнала по `seq`. Бэкап — `VACUUM INTO` + копия журнала + `manifest.json` (манифест пишется последним). Существующее in-memory `internal/store` не переписывается: в `main` БД читается на старте и наполняет `store` (read-through), CLI-подкоманды `backup`/`restore`/`verify` работают напрямую со `storage`.

**Tech Stack:** Go 1.26 (go.mod), `modernc.org/sqlite`, `golang.org/x/text` (уже транзитивная, станет прямой). Тесты — stdlib `testing` с `t.TempDir()`.

**Spec:** `docs/superpowers/specs/2026-09-19-storage-design.md`

## Global Constraints

- Single binary, **без cgo**: драйвер только `modernc.org/sqlite` (driver name `"sqlite"`).
- Каталоги запуска: `db/` (БД + журнал) и `backup/` — раздельно; при старте sentinel: если `db/` и `backup/` на одном устройстве — предупреждение в лог.
- SQLite: WAL включён, `PRAGMA foreign_keys=ON`, штатные механизмы консистентности не отключаются.
- Инвариант записи: журнал (append + fsync) **раньше** SQLite-commit; журнал никогда не младше БД.
- Replay идемпотентен: в журнале только полные после-образы (`op:"save"`, payload = весь объект).
- Нормализация кириллицы — одно Go-место: `ToLower` → `ё→е` → NFD → удаление диакритики; применяется и при записи `search`, и при поисковом запросе.
- Бэкап: `VACUUM INTO` + копия журнала + `manifest.json`; манифест пишется **последним**.
- Восстановление: только в новую директорию; поверх живых данных — только с `--force`.
- Схема versioned: `meta.schema_version = 1`.

---

### Task 1: Зависимости и нормализация кириллицы

**Files:**
- Create: `internal/storage/normalize.go`
- Test: `internal/storage/normalize_test.go`
- Modify: `go.mod`, `go.sum` (через `go get`)

**Interfaces:**
- Consumes: ничего.
- Produces: `func Normalize(s string) string` — используется в Task 3 (search-индекс) и Task 4 (поиск). Ниже в плане все шаблоны «нормализуй перед записью и запросом» ссылаются на неё.

- [ ] **Step 1: Добавить зависимость x/text (прямая)**

Run: `go get golang.org/x/text@latest`
Expected: `go.mod` содержит `golang.org/x/text` в прямых require. `modernc.org/sqlite` добавим в Task 3.

- [ ] **Step 2: Написать падающий тест**

`internal/storage/normalize_test.go`:

```go
package storage

import "testing"

func TestNormalizeСемёновVsСеменов(t *testing.T) {
	a := Normalize("Семёнов")
	b := Normalize("Семенов")
	if a != b {
		t.Fatalf("ё должен приравниваться к е: %q != %q", a, b)
	}
}

func TestNormalizeLowercase(t *testing.T) {
	if got := Normalize("СЕМЕНОВ"); got != Normalize("семенов") {
		t.Fatalf("Normalize(uppercase) = %q", got)
	}
}
```

- [ ] **Step 3: Запустить и убедиться, что падает**

Run: `go test ./internal/storage/ -run TestNormalize`
Expected: FAIL — `undefined: Normalize`, пакет `internal/storage` не существует.

- [ ] **Step 4: Реализация**

`internal/storage/normalize.go`:

```go
package storage

import (
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// Normalize приводит строку к канонической форме для поиска по кириллице:
// нижний регистр, ё→е, NFD и удаление комбинирующихся символов (диакритики).
// Применяется и при записи search-индекса, и при поисковом запросе.
func Normalize(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "ё", "е")
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if unicode.IsMark(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}
```

- [ ] **Step 5: Запустить и убедиться, что проходит**

Run: `go test ./internal/storage/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/storage/
git commit -m "feat(storage): нормализация кириллицы для поиска (lowercase, ё→е, NFD)"
```

---

### Task 2: Журнал (append-only JSONL, полные образы)

**Files:**
- Create: `internal/storage/journal.go`
- Test: `internal/storage/journal_test.go`

**Interfaces:**
- Consumes: ничего (нормализация не нужна).
- Produces (нужно Task 4, 5, 8):

```go
type JournalEntry struct {
    Seq        uint64          // монотонный номер записи
    Op         string          // "save" | "delete" | "snapshot"
    EntityType string          // тип сущности (entityType)
    ID         string          // id сущности
    At         string          // RFC3339Nano UTC
    Payload    json.RawMessage // полный после-образ; nil для delete/snapshot
    Search     []byte          // нормализованный поисковый текст (для save);
                               // хранится в журнале, чтобы replay восстановил
                               // и search-индекс без знания модели
}

func OpenJournal(path string) (*Journal, error)
func (j *Journal) Append(op, entityType, id string, payload json.RawMessage, search []byte) (JournalEntry, error)
func (j *Journal) LastSeq() uint64
func (j *Journal) ReplayAll(fn func(JournalEntry) error) error   // в порядке seq
func (j *Journal) Close() error
```

- [ ] **Step 1: Написать падающий тест**

`internal/storage/journal_test.go`:

```go
package storage

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestJournalAppendReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.jsonl")
	j, err := OpenJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	e1, err := j.Append("save", "person", "blohin", json.RawMessage(`{"surname":"Блохин"}`), []byte("blohin"))
	if err != nil {
		t.Fatal(err)
	}
	if e1.Seq != 1 {
		t.Fatalf("first Seq = %d, want 1", e1.Seq)
	}
	if _, err := j.Append("save", "person", "dorozhkin", json.RawMessage(`{"surname":"Дорожкин"}`), []byte("dorozhkin")); err != nil {
		t.Fatal(err)
	}
	if j.LastSeq() != 2 {
		t.Fatalf("LastSeq = %d, want 2", j.LastSeq())
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}

	j2, err := OpenJournal(path) // повторное открытие: seq восстанавливается сканированием
	if err != nil {
		t.Fatal(err)
	}
	defer j2.Close()
	if j2.LastSeq() != 2 {
		t.Fatalf("reopen LastSeq = %d, want 2", j2.LastSeq())
	}
	var got []JournalEntry
	if err := j2.ReplayAll(func(e JournalEntry) error {
		got = append(got, e)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].ID != "dorozhkin" {
		t.Fatalf("replay = %+v", got)
	}
}

func TestJournalAppendIdempotentSeq(t *testing.T) {
	path := filepath.Join(t.TempDir(), "journal.jsonl")
	j, err := OpenJournal(path)
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	for _, id := range []string{"a", "b", "c"} {
		if _, err := j.Append("save", "person", id, json.RawMessage(`{"x":1}`), nil); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := j.Append("delete", "person", "a", nil, nil); err != nil {
		t.Fatal(err)
	}
	if j.LastSeq() != 4 {
		t.Fatalf("LastSeq = %d, want 4", j.LastSeq())
	}
}
```

- [ ] **Step 2: Запустить и убедиться, что падает**

Run: `go test ./internal/storage/ -run TestJournal`
Expected: FAIL — `undefined: OpenJournal`.

- [ ] **Step 3: Реализация**

`internal/storage/journal.go`:

```go
package storage

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

// JournalEntry — одна запись журнала. Payload — ПОЛНЫЙ после-образ сущности
// (не диф), поэтому replay идемпотентен: повторная запись того же образа безопасна.
// Search хранится отдельно, чтобы replay восстановил поисковый индекс даже
// без знания модели (storage модель-агностичен).
type JournalEntry struct {
	Seq        uint64          `json:"seq"`
	Op         string          `json:"op"`
	EntityType string          `json:"entity,omitempty"`
	ID         string          `json:"id,omitempty"`
	At         string          `json:"at"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	Search     []byte          `json:"search,omitempty"`
}

// Journal — append-only JSONL-журнал. Каждая Append делает fsync (Flush+Sync),
// поэтому при крахе журнал не младше подтверждённой записи.
type Journal struct {
	f   *os.File
	w   *bufio.Writer
	seq uint64
}

// OpenJournal открывает журнал в режиме O_APPEND и восстанавливает последний seq.
func OpenJournal(path string) (*Journal, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	j := &Journal{f: f, w: bufio.NewWriter(f)}
	if err := j.scanSeq(); err != nil {
		f.Close()
		return nil, err
	}
	return j, nil
}

func (j *Journal) scanSeq() error {
	dec := json.NewDecoder(j.f)
	for {
		var e JournalEntry
		if err := dec.Decode(&e); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("journal: decoder: %w", err)
		}
		if e.Seq > j.seq {
			j.seq = e.Seq
		}
	}
}

// Append инкрементирует seq и делает durable-запись (Flush + Sync).
func (j *Journal) Append(op, entityType, id string, payload json.RawMessage, search []byte) (JournalEntry, error) {
	j.seq++
	e := JournalEntry{
		Seq:        j.seq,
		Op:         op,
		EntityType: entityType,
		ID:         id,
		At:         time.Now().UTC().Format(time.RFC3339Nano),
		Payload:    payload,
		Search:     search,
	}
	line, err := json.Marshal(e)
	if err != nil {
		return JournalEntry{}, err
	}
	if _, err := j.w.Write(append(line, '\n')); err != nil {
		return JournalEntry{}, err
	}
	if err := j.w.Flush(); err != nil {
		return JournalEntry{}, err
	}
	if err := j.f.Sync(); err != nil {
		return JournalEntry{}, err
	}
	return e, nil
}

// LastSeq возвращает последний записанный seq.
func (j *Journal) LastSeq() uint64 { return j.seq }

// ReplayAll вызывает fn для каждой записи в порядке возрастания seq.
func (j *Journal) ReplayAll(fn func(JournalEntry) error) error {
	// перечитываем с начала файла; для масштаба ~30 MB это быстро
	if _, err := j.f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	dec := json.NewDecoder(j.f)
	for {
		var e JournalEntry
		if err := dec.Decode(&e); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if err := fn(e); err != nil {
			return err
		}
	}
}

// Close сбрасывает буфер и закрывает файл.
func (j *Journal) Close() error {
	if err := j.w.Flush(); err != nil {
		j.f.Close()
		return err
	}
	return j.f.Close()
}
```

- [ ] **Step 4: Запустить и убедиться, что проходит**

Run: `go test ./internal/storage/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/journal.go internal/storage/journal_test.go
git commit -m "feat(storage): append-only JSONL-журнал с полными образами"
```

---

### Task 3: SQLite-движок (схема, upsert/get/list/search, seq)

**Files:**
- Create: `internal/storage/db.go`
- Test: `internal/storage/db_test.go`
- Modify: `go.mod`, `go.sum` (через `go get modernc.org/sqlite`)

**Interfaces:**
- Consumes: `Normalize` (Task 1).
- Produces (нужно Task 4, 5, 7):

```go
const schemaVersion = 1

func OpenDB(path string) (*DB, error)
func (d *DB) Upsert(entityType, id string, data, search []byte) error
func (d *DB) Delete(entityType, id string) error
func (d *DB) Get(entityType, id string) (data []byte, ok bool, err error)
func (d *DB) List(entityType string) ([]entityRow, error) // entityRow{ID string; Data []byte}
func (d *DB) Search(entityType, query string) ([]string, error) // по search, оператор LIKE
func (d *DB) SetAppliedSeq(seq uint64) error
func (d *DB) AppliedSeq() (uint64, error)
func (d *DB) IntegrityCheck() error
func (d *DB) VacuumInto(path string) error
func (d *DB) Count(entityType string) (int, error)
func (d *DB) Close() error
```

Схема: таблица `entity(entity_type TEXT, id TEXT, data TEXT, search TEXT, updated_at TEXT, PRIMARY KEY(entity_type, id))`; служебная `meta(key TEXT PRIMARY KEY, value TEXT)` для `applied_seq` и `schema_version`. WAL включён, `foreign_keys=ON`. Одно соединение (`SetMaxOpenConns(1)`), `busy_timeout`.

- [ ] **Step 1: Добавить зависимость**

Run: `go get modernc.org/sqlite@latest`
Expected: `go.mod` содержит `modernc.org/sqlite` в прямых require.

- [ ] **Step 2: Написать падающий тест**

`internal/storage/db_test.go`:

```go
package storage

import (
	"path/filepath"
	"testing"
)

func openTestDB(t *testing.T) *DB {
	t.Helper()
	db, err := OpenDB(filepath.Join(t.TempDir(), "genealogy.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestDBUpsertGetListDelete(t *testing.T) {
	db := openTestDB(t)
	if err := db.Upsert("person", "blohin", []byte(`{"surname":"Блохин"}`), []byte(Normalize("Блохин"))); err != nil {
		t.Fatal(err)
	}
	got, ok, err := db.Get("person", "blohin")
	if err != nil || !ok {
		t.Fatalf("Get: ok=%v err=%v", ok, err)
	}
	if string(got) != `{"surname":"Блохин"}` {
		t.Fatalf("data = %s", got)
	}
	if n, _ := db.Count("person"); n != 1 {
		t.Fatalf("Count = %d, want 1", n)
	}
	if _, err := db.List("person"); err != nil {
		t.Fatal(err)
	}
	if err := db.Delete("person", "blohin"); err != nil {
		t.Fatal(err)
	}
	_, ok, _ = db.Get("person", "blohin")
	if ok {
		t.Fatal("entity exists after Delete")
	}
}

func TestDBSearchNormalized(t *testing.T) {
	db := openTestDB(t)
	_ = db.Upsert("person", "blohin", []byte(`{}`), []byte(Normalize("Семёнов Иван")))
	_ = db.Upsert("person", "dorozhkin", []byte(`{}`), []byte(Normalize("Дорожкин Пётр")))
	ids, err := db.Search("person", Normalize("семен"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != "blohin" {
		t.Fatalf("Search = %v, want [blohin]", ids)
	}
}

func TestDBAppliedSeq(t *testing.T) {
	db := openTestDB(t)
	if seq, _ := db.AppliedSeq(); seq != 0 {
		t.Fatalf("initial AppliedSeq = %d", seq)
	}
	if err := db.SetAppliedSeq(42); err != nil {
		t.Fatal(err)
	}
	if seq, _ := db.AppliedSeq(); seq != 42 {
		t.Fatalf("AppliedSeq = %d, want 42", seq)
	}
}

func TestDBIntegrityAndVacuumInto(t *testing.T) {
	db := openTestDB(t)
	_ = db.Upsert("person", "blohin", []byte(`{}`), []byte("x"))
	if err := db.IntegrityCheck(); err != nil {
		t.Fatalf("IntegrityCheck: %v", err)
	}
	out := filepath.Join(t.TempDir(), "snapshot.db")
	if err := db.VacuumInto(out); err != nil {
		t.Fatalf("VacuumInto: %v", err)
	}
}
```

- [ ] **Step 3: Запустить и убедиться, что падает**

Run: `go test ./internal/storage/ -run TestDB`
Expected: FAIL — `undefined: OpenDB`.

- [ ] **Step 4: Реализация**

`internal/storage/db.go`:

```go
package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

const schemaVersion = 1

// entityRow — строка чтения сущности.
type entityRow struct {
	ID   string
	Data []byte
}

// DB оборачивает одно SQLite-соединение. Сущности хранятся модель-агностично:
// JSON в колонке data, нормализованный поисковый индекс — в колонке search.
type DB struct {
	d *sql.DB
}

// OpenDB открывает БД, включает WAL, foreign keys и создаёт схему.
func OpenDB(path string) (*DB, error) {
	d, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	d.SetMaxOpenConns(1)

	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	}
	for _, p := range pragmas {
		if _, err := d.Exec(p); err != nil {
			d.Close()
			return nil, fmt.Errorf("open: %w", err)
		}
	}

	ddl := []string{
		`CREATE TABLE IF NOT EXISTS meta (
			key   TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS entity (
			entity_type TEXT NOT NULL,
			id          TEXT NOT NULL,
			data        TEXT NOT NULL,
			search      TEXT NOT NULL DEFAULT '',
			updated_at  TEXT NOT NULL,
			PRIMARY KEY (entity_type, id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_entity_search
			ON entity (entity_type, search)`,
	}
	for _, q := range ddl {
		if _, err := d.Exec(q); err != nil {
			d.Close()
			return nil, fmt.Errorf("schema: %w", err)
		}
	}
	if _, err := d.Exec(
		`INSERT OR IGNORE INTO meta(key, value) VALUES ('schema_version', ?)`, schemaVersion,
	); err != nil {
		d.Close()
		return nil, err
	}
	return &DB{d: d}, nil
}

// Upsert сохраняет сущность (INSERT OR REPLACE). search — уже нормализованный
// конкатенат поисковых строк сущности.
func (d *DB) Upsert(entityType, id string, data, search []byte) error {
	_, err := d.d.Exec(
		`INSERT OR REPLACE INTO entity(entity_type, id, data, search, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		entityType, id, string(data), string(search), time.Now().UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (d *DB) Delete(entityType, id string) error {
	_, err := d.d.Exec(`DELETE FROM entity WHERE entity_type = ? AND id = ?`, entityType, id)
	return err
}

func (d *DB) Get(entityType, id string) ([]byte, bool, error) {
	var data string
	err := d.d.QueryRow(
		`SELECT data FROM entity WHERE entity_type = ? AND id = ?`, entityType, id,
	).Scan(&data)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return []byte(data), true, nil
}

// List возвращает все сущности типа в недетерминированном порядке.
func (d *DB) List(entityType string) ([]entityRow, error) {
	rows, err := d.d.Query(
		`SELECT id, data FROM entity WHERE entity_type = ?`, entityType,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []entityRow
	for rows.Next() {
		var r entityRow
		var data string
		if err := rows.Scan(&r.ID, &data); err != nil {
			return nil, err
		}
		r.Data = []byte(data)
		out = append(out, r)
	}
	return out, rows.Err()
}

// Search возвращает id сущностей, чей search-индекс содержит query (полное
// или частичное совпадение). query обязан быть уже нормализованным.
func (d *DB) Search(entityType, query string) ([]string, error) {
	rows, err := d.d.Query(
		`SELECT id FROM entity
		 WHERE entity_type = ? AND search LIKE '%' || ? || '%'
		 ORDER BY id`,
		entityType, query,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// SetAppliedSeq фиксирует, до какого seq журнала БД актуальна.
func (d *DB) SetAppliedSeq(seq uint64) error {
	_, err := d.d.Exec(
		`INSERT OR REPLACE INTO meta(key, value) VALUES ('applied_seq', ?)`, seq,
	)
	return err
}

func (d *DB) AppliedSeq() (uint64, error) {
	var v string
	err := d.d.QueryRow(`SELECT value FROM meta WHERE key = 'applied_seq'`).Scan(&v)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	seq, err := ParseUint64(v)
	if err != nil {
		return 0, fmt.Errorf("applied_seq: %w", err)
	}
	return seq, nil
}

func (d *DB) Count(entityType string) (int, error) {
	var n int
	err := d.d.QueryRow(
		`SELECT COUNT(*) FROM entity WHERE entity_type = ?`, entityType,
	).Scan(&n)
	return n, err
}

// IntegrityCheck выполняет PRAGMA integrity_check (максимум 4 строки ошибок).
func (d *DB) IntegrityCheck() error {
	rows, err := d.d.Query("PRAGMA integrity_check")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return err
		}
		if line != "ok" {
			return fmt.Errorf("integrity_check: %s", line)
		}
	}
	return rows.Err()
}

// VacuumInto создаёт консистентный снапшот БД без остановки (SQLite сам
// согласует состояние с WAL на момент выполнения).
func (d *DB) VacuumInto(path string) error {
	_, err := d.d.Exec(fmt.Sprintf("VACUUM INTO '%s'", path))
	return err
}

func (d *DB) Close() error { return d.d.Close() }
```

- [ ] **Step 5: Запустить и убедиться, что проходит**

Run: `go test ./internal/storage/`
Expected: PASS. (Если `ParseUint64` не найден — добавить `internal/storage/util.go` с `func ParseUint64(s string) (uint64, error)` на `strconv.ParseUint`.)

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum internal/storage/db.go internal/storage/db_test.go
git commit -m "feat(storage): SQLite-движок (WAL, upsert/search/seq, VACUUM INTO)"
```

---

### Task 4: Storage — фасад «журнал → БД» с replay при старте

**Files:**
- Create: `internal/storage/storage.go`
- Test: `internal/storage/storage_test.go`
- Creates helper: `internal/storage/util.go` (если не появился в Task 3)

**Interfaces:**
- Consumes: `Normalize` (T1), `Journal`+`JournalEntry` (T2), `DB` (T3).
- Produces (нужно T5..T8):

```go
type Storage struct { ... }

// Open каталога данных: db/genodex.db + db/journal.jsonl; на старте
// восстанавливает БД из журнала (replay записей с seq > applied_seq).
func Open(dataDir string) (*Storage, error)

// Save пишет журнал (fsync), затем upsert в БД и applied_seq.
// payload — JSON сущности; search — уже нормализованный поисковый текст.
func (s *Storage) Save(entityType, id string, data, search []byte) error
func (s *Storage) Delete(entityType, id string) error
func (s *Storage) Get(entityType, id string) ([]byte, bool, error)
func (s *Storage) List(entityType string) ([]entityRow, error)
func (s *Storage) Search(entityType, query string) ([]string, error)
func (s *Storage) Count(entityType string) (int, error)
func (s *Storage) AppliedSeq() (uint64, error)
func (s *Storage) LastSeq() uint64
func (s *Storage) Close() error
```

Ключевой сценарий теста — **потеря БД при живом журнале**: после `Open`/`Save`, удаляем `db/genodex.db`, повторно `Open` — журнал перестраивает БД. Этот тест — доказательство инварианта «журнал не младше БД».

- [ ] **Step 1: Написать падающий тест**

`internal/storage/storage_test.go`:

```go
package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func savePerson(t *testing.T, s *Storage, id, surname string) {
	t.Helper()
	txt := []byte(`{"surname":"` + surname + `"}`)
	if err := s.Save("person", id, txt, []byte(Normalize(surname))); err != nil {
		t.Fatal(err)
	}
}

func TestStorageSaveGet(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	savePerson(t, s, "blohin", "Блохин")
	got, ok, err := s.Get("person", "blohin")
	if err != nil || !ok || string(got) != `{"surname":"Блохин"}` {
		t.Fatalf("Get: ok=%v data=%s err=%v", ok, got, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestStorageReplayOnReopen(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	savePerson(t, s, "blohin", "Блохин")
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	got, ok, _ := s2.Get("person", "blohin")
	if !ok || string(got) != `{"surname":"Блохин"}` {
		t.Fatalf("after reopen: ok=%v data=%s", ok, got)
	}
}

func TestStorageRecoversWhenDBLost(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	savePerson(t, s, "blohin", "Блохин")
	savePerson(t, s, "dorozhkin", "Дорожкин")
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	// симуляция потери БД при живом журнале
	if err := os.Remove(filepath.Join(dir, "db", "genodex.db")); err != nil {
		t.Fatal(err)
	}

	s2, err := Open(dir)
	if err != nil {
		t.Fatalf("не смогли восстановить БД из журнала: %v", err)
	}
	defer s2.Close()
	n, _ := s2.Count("person")
	if n != 2 {
		t.Fatalf("Count after DB loss = %d, want 2 (replay из журнала)", n)
	}
	got, ok, _ := s2.Get("person", "dorozhkin")
	if !ok || string(got) != `{"surname":"Дорожкин"}` {
		t.Fatalf("recovered dorozhkin: ok=%v data=%s", ok, got)
	}
}

func TestStorageDelete(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	savePerson(t, s, "blohin", "Блохин")
	if err := s.Delete("person", "blohin"); err != nil {
		t.Fatal(err)
	}
	_, ok, _ := s.Get("person", "blohin")
	if ok {
		t.Fatal("entity exists after Delete")
	}
}

func TestStorageSearch(t *testing.T) {
	dir := t.TempDir()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	savePerson(t, s, "blohin", "Семёнов")
	savePerson(t, s, "dorozhkin", "Дорожкин")
	ids, err := s.Search("person", Normalize("семен"))
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 1 || ids[0] != "blohin" {
		t.Fatalf("Search = %v, want [blohin]", ids)
	}
}
```

- [ ] **Step 2: Запустить и убедиться, что падает**

Run: `go test ./internal/storage/ -run TestStorage`
Expected: FAIL — `undefined: Open`.

- [ ] **Step 3: Реализация**

`internal/storage/storage.go`:

```go
package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Storage — точка входа: журнал (appendix, durability) + SQLite (запросы,
// индексы). Все записи идут через Save/Delete, которые сначала пишут журнал.
type Storage struct {
	dir     string
	db      *DB
	journal *Journal
}

// Open открывает хранилище в dataDir (структура: db/, journal лежит в db/).
// При старте replay: восстанавливает пропущенные операции (seq > applied_seq)
// из журнала в БД — так поддерживается инвариант «журнал не младше БД».
func Open(dataDir string) (*Storage, error) {
	dbDir := filepath.Join(dataDir, "db")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		return nil, err
	}
	db, err := OpenDB(filepath.Join(dbDir, "genodex.db"))
	if err != nil {
		return nil, err
	}
	j, err := OpenJournal(filepath.Join(dbDir, "journal.jsonl"))
	if err != nil {
		db.Close()
		return nil, err
	}
	s := &Storage{dir: dataDir, db: db, journal: j}
	if err := s.replay(); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// replay применяет к БД записи журнала с seq > applied_seq.
func (s *Storage) replay() error {
	applied, err := s.db.AppliedSeq()
	if err != nil {
		return err
	}
	return s.journal.ReplayAll(func(e JournalEntry) error {
		if e.Seq <= applied {
			return nil
		}
		switch e.Op {
		case "save":
			if err := s.db.Upsert(e.EntityType, e.ID, e.Payload, e.Search); err != nil {
				return err
			}
		case "delete":
			if err := s.db.Delete(e.EntityType, e.ID); err != nil {
				return err
			}
		}
		return s.db.SetAppliedSeq(e.Seq)
	})
}

// Save пишет полный образ в журнал (fsync), затем в БД и обновляет applied_seq.
// search — нормализованный поисковый текст; он идёт и в журнал, чтобы replay
// восстановил search-индекс после потери БД.
func (s *Storage) Save(entityType, id string, data, search []byte) error {
	if _, err := s.journal.Append("save", entityType, id, json.RawMessage(data), search); err != nil {
		return err
	}
	if err := s.db.Upsert(entityType, id, data, search); err != nil {
		return err
	}
	return s.db.SetAppliedSeq(s.journal.LastSeq())
}

func (s *Storage) Delete(entityType, id string) error {
	if _, err := s.journal.Append("delete", entityType, id, nil, nil); err != nil {
		return err
	}
	if err := s.db.Delete(entityType, id); err != nil {
		return err
	}
	return s.db.SetAppliedSeq(s.journal.LastSeq())
}

func (s *Storage) Get(entityType, id string) ([]byte, bool, error) {
	return s.db.Get(entityType, id)
}

func (s *Storage) List(entityType string) ([]entityRow, error) {
	return s.db.List(entityType)
}

func (s *Storage) Search(entityType, query string) ([]string, error) {
	return s.db.Search(entityType, Normalize(query))
}

func (s *Storage) Count(entityType string) (int, error) { return s.db.Count(entityType) }

func (s *Storage) AppliedSeq() (uint64, error) { return s.db.AppliedSeq() }

func (s *Storage) LastSeq() uint64 { return s.journal.LastSeq() }

func (s *Storage) Close() error {
	if err := s.journal.Close(); err != nil {
		s.db.Close()
		return err
	}
	return s.db.Close()
}
```

- [ ] **Step 4: Запустить и убедиться, что проходит**

Run: `go test ./internal/storage/`
Expected: PASS. Если `ParseUint64` отсутствует — создать `internal/storage/util.go`:

```go
package storage

import "strconv"

// ParseUint64 парсит строку в uint64 (для meta-значений).
func ParseUint64(s string) (uint64, error) {
	return strconv.ParseUint(s, 10, 64)
}
```

- [ ] **Step 5: Commit**

```bash
git add internal/storage/storage.go internal/storage/storage_test.go internal/storage/util.go
git commit -m "feat(storage): фасад storage с replay журнала при старте"
```

---

### Task 5: Backup — бандл «снапшот + журнал + манифест»

**Files:**
- Create: `internal/storage/backup.go`
- Test: `internal/storage/backup_test.go`

**Interfaces:**
- Consumes: `Storage` (T4), `DB.VacuumInto`, journal.
- Produces (нужно T6, T8, T9):

```go
type Manifest struct {
    SchemaVersion int                `json:"schema_version"`
    CreatedAt     string             `json:"created_at"`     // RFC3339 UTC
    AppliedSeq    uint64             `json:"applied_seq"`    // seq на момент снапшота
    SnapshotFile  string             `json:"snapshot_file"`  // баснейм
    SnapshotSHA   string             `json:"snapshot_sha256"`
    JournalFile   string             `json:"journal_file"`   // баснейм копии журнала
    JournalSHA    string             `json:"journal_sha256"`
    EntityCounts  map[string]int     `json:"entity_counts"`  // тип -> кол-во сущностей
}

// Backup делает бандл в toDir: VACUUM INTO + копия журнала + manifest.json.
// Manifest пишется последним. Манифест можно переписать из снапшота+журнала.
func Backup(s *Storage, toDir string) (Manifest, error)
func ReadManifest(toDir string) (*Manifest, error)
```

Цель: восстановление БД из бандла без знания данных (снапшот — консистентный
момент; журнал — все изменения после него). Медиа — вне склада, отдельно.

- [ ] **Step 1: Написать падающий тест**

`internal/storage/backup_test.go`:

```go
package storage

import (
	"path/filepath"
	"testing"
)

func TestBackupCreatesBundle(t *testing.T) {
	src := t.TempDir()
	s, err := Open(src)
	if err != nil {
		t.Fatal(err)
	}
	savePerson(t, s, "blohin", "Блохин")
	savePerson(t, s, "dorozhkin", "Дорожкин")
	seq := s.LastSeq()
	dst := filepath.Join(t.TempDir(), "backup")
	m, err := Backup(s, dst)
	if err != nil {
		t.Fatal(err)
	}
	if m.AppliedSeq != seq {
		t.Fatalf("AppliedSeq = %d, want %d", m.AppliedSeq, seq)
	}
	if m.SchemaVersion != schemaVersion {
		t.Fatalf("SchemaVersion = %d, want %d", m.SchemaVersion, schemaVersion)
	}
	if _, err := ReadManifest(dst); err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
}
```

- [ ] **Step 2: Запустить и убедиться, что падает**

Run: `go test ./internal/storage/ -run TestBackup`
Expected: FAIL — `undefined: Backup`.

- [ ] **Step 3: Реализация**

`internal/storage/backup.go`:

```go
package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

const manifestFile = "manifest.json"

// Manifest описывает бандл бэкапа.
type Manifest struct {
	SchemaVersion int               `json:"schema_version"`
	CreatedAt     string            `json:"created_at"`
	AppliedSeq    uint64            `json:"applied_seq"`
	SnapshotFile  string            `json:"snapshot_file"`
	SnapshotSHA   string            `json:"snapshot_sha256"`
	JournalFile   string            `json:"journal_file"`
	JournalSHA    string            `json:"journal_sha256"`
	EntityCounts  map[string]int    `json:"entity_counts"`
}

// Backup создаёт в toDir консистентный бандл: снапшот (VACUUM INTO), копию
// журнала и манифест, который пишется последним. Порядок гарантирует: если
// манифест прочитается — значит и снапшот, и журнал уже на месте и целы.
func Backup(s *Storage, toDir string) (Manifest, error) {
	if err := os.MkdirAll(toDir, 0o755); err != nil {
		return Manifest{}, err
	}
	ts := time.Now().UTC().Format("20060102T150405Z")
	snapFile := "snapshot-" + ts + ".db"
	snapPath := filepath.Join(toDir, snapFile)
	if err := s.db.VacuumInto(snapPath); err != nil {
		return Manifest{}, fmt.Errorf("vacuum: %w", err)
	}
	snapSHA, err := fileSHA(snapPath)
	if err != nil {
		return Manifest{}, err
	}

	jrnFile := "journal-" + ts + ".jsonl"
	jrnPath := filepath.Join(toDir, jrnFile)
	if err := s.copyJournal(jrnPath); err != nil {
		return Manifest{}, err
	}
	jrnSHA, err := fileSHA(jrnPath)
	if err != nil {
		return Manifest{}, err
	}

	counts := map[string]int{}
	for _, et := range []string{
		"person", "settlement", "church", "parish", "governorate", "district",
		"volost", "archive", "fund", "inventory", "case", "event", "marriage", "source",
	} {
		n, err := s.db.Count(et)
		if err != nil {
			return Manifest{}, err
		}
		if n > 0 {
			counts[et] = n
		}
	}

	seq, err := s.db.AppliedSeq()
	if err != nil {
		return Manifest{}, err
	}
	m := Manifest{
		SchemaVersion: schemaVersion,
		CreatedAt:     time.Now().UTC().Format(time.RFC3339),
		AppliedSeq:    seq,
		SnapshotFile:  snapFile,
		SnapshotSHA:   snapSHA,
		JournalFile:   jrnFile,
		JournalSHA:    jrnSHA,
		EntityCounts:  counts,
	}
	if err := writeManifest(toDir, m); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

func (s *Storage) copyJournal(dst string) error {
	in, err := os.Open(filepath.Join(s.dir, "db", "journal.jsonl"))
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func fileSHA(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func writeManifest(dir string, m Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, manifestFile+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, manifestFile))
}

// ReadManifest читает манифест бандла.
func ReadManifest(dir string) (*Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, manifestFile))
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}
```

- [ ] **Step 4: Запустить и убедиться, что проходит**

Run: `go test ./internal/storage/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/backup.go internal/storage/backup_test.go
git commit -m "feat(storage): бэкап-бандл (VACUUM INTO + журнал + manifest)"
```

---

### Task 6: Restore — восстановление из бандла в новую директорию

**Files:**
- Create: `internal/storage/restore.go`
- Test: `internal/storage/restore_test.go`

**Interfaces:**
- Consumes: `Manifest` (T5), `DB` (T3), `Journal`/`JournalEntry` (T2).
- Produces (нужно T8):

```go
// Restore восстанавливает бандл из srcDir в toDir (новая пустая директория).
// Схема: копировать снапшот → открыть БД → integrity_check → открыть копию
// журнала → replay всех записей (remove applied, apply все) → сверить counts.
func Restore(srcDir, toDir string) (*Storage, error)
```

Правила: `toDir` не должен содержать существующую БД; восстановление никогда не
пишет поверх живых данных без `--force` (флаг обрабатывается в CLI, не здесь).

- [ ] **Step 1: Написать падающий тест**

`internal/storage/restore_test.go`:

```go
package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRestoreRoundTrip(t *testing.T) {
	srcData := t.TempDir()
	s, err := Open(srcData)
	if err != nil {
		t.Fatal(err)
	}
	savePerson(t, s, "blohin", "Блохин")
	savePerson(t, s, "dorozhkin", "Дорожкин")
	seq := s.LastSeq()
	bkpDir := filepath.Join(t.TempDir(), "backup")
	if _, err := Backup(s, bkpDir); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	restored := filepath.Join(t.TempDir(), "restored")
	rs, err := Restore(bkpDir, restored)
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	defer rs.Close()
	if rs.LastSeq() != seq {
		t.Fatalf("restored LastSeq = %d, want %d", rs.LastSeq(), seq)
	}
	got, ok, _ := rs.Get("person", "blohin")
	if !ok || string(got) != `{"surname":"Блохин"}` {
		t.Fatalf("restored blohin: ok=%v data=%s", ok, got)
	}
	n, _ := rs.Count("person")
	if n != 2 {
		t.Fatalf("restored Count = %d, want 2", n)
	}
}

func TestRestoreRefusesNonEmptyTarget(t *testing.T) {
	srcData := t.TempDir()
	s, _ := Open(srcData)
	savePerson(t, s, "blohin", "Блохин")
	bkpDir := filepath.Join(t.TempDir(), "backup")
	if _, err := Backup(s, bkpDir); err != nil {
		t.Fatal(err)
	}
	s.Close()

	evil := filepath.Join(t.TempDir(), "occupied")
	if err := os.MkdirAll(filepath.Join(evil, "db"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(evil, "db", "genodex.db"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Restore(bkpDir, evil); err == nil {
		t.Fatal("Restore должен отказаться писать в непустую директорию")
	}
}
```

- [ ] **Step 2: Запустить и убедиться, что падает**

Run: `go test ./internal/storage/ -run TestRestore`
Expected: FAIL — `undefined: Restore`.

- [ ] **Step 3: Реализация**

`internal/storage/restore.go`:

```go
package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Restore создаёт новое хранилище в toDir из бандла srcDir:
// снапшот копируется как genodex.db, журнал — как journal.jsonl, затем БД
// догоняется replay'ем, и результаты сверяются со счётчиками манифеста.
func Restore(srcDir, toDir string) (*Storage, error) {
	m, err := ReadManifest(srcDir)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	dbDir := filepath.Join(toDir, "db")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		return nil, err
	}
	// не пишем поверх живой БД: если genodex.db уже существует — отказ
	dbPath := filepath.Join(dbDir, "genodex.db")
	if _, err := os.Stat(dbPath); err == nil {
		return nil, fmt.Errorf("target already contains %s", dbPath)
	}

	// 1. снапшот → genodex.db
	if err := copyFile(filepath.Join(srcDir, m.SnapshotFile), dbPath); err != nil {
		return nil, fmt.Errorf("copy snapshot: %w", err)
	}
	// 2. журнал → journal.jsonl
	if err := copyFile(filepath.Join(srcDir, m.JournalFile), filepath.Join(dbDir, "journal.jsonl")); err != nil {
		return nil, fmt.Errorf("copy journal: %w", err)
	}

	// 3. открываем как обычное хранилище; replay доводит БД до конца журнала
	s, err := Open(toDir)
	if err != nil {
		return nil, fmt.Errorf("open restored: %w", err)
	}

	// 4. сверка целостности и счётчиков
	if err := s.db.IntegrityCheck(); err != nil {
		s.Close()
		return nil, fmt.Errorf("integrity of restored db: %w", err)
	}
	for et, want := range m.EntityCounts {
		got, err := s.db.Count(et)
		if err != nil {
			s.Close()
			return nil, err
		}
		if got != want {
			s.Close()
			return nil, fmt.Errorf("count %s: got %d want %d", et, got, want)
		}
	}
	return s, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
```

- [ ] **Step 4: Запустить и убедиться, что проходит**

Run: `go test ./internal/storage/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/restore.go internal/storage/restore_test.go
git commit -m "feat(storage): восстановление из бандла в новую директорию"
```

---

### Task 7: Verify — проверка базы, журнала и бандла

**Files:**
- Create: `internal/storage/verify.go`
- Test: `internal/storage/verify_test.go`

**Interfaces:**
- Consumes: `DB` (T3), `Journal` (T2), `Manifest`/`Backup` (T5).
- Produces (нужно T8):

```go
type VerifyResult struct {
	OK              bool
	Integrity       error  // PRAGMA integrity_check
	ReplayedSeq     uint64 // финальный seq после replay
	AppliedSeq      uint64 // applied_seq БД
	EntitiesByType  map[string]int
	JournalEntries  int  // число записей в журнале
	ManifestMatches bool // counts/seq совпали с манифестом (если задан)
}

// Verify проверяет открытое хранилище, не меняя данные: integrity_check,
// перечитывает журнал, сравнивает с applied_seq. Для бандла (backupDir != "")
// дополнительно сверяет sha256 файлов и counts из манифеста.
func Verify(s *Storage, backupDir string) (*VerifyResult, error)
```

- [ ] **Step 1: Написать падающий тест**

`internal/storage/verify_test.go`:

```go
package storage

import (
	"path/filepath"
	"testing"
)

func TestVerifyHappyPath(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)
	savePerson(t, s, "blohin", "Блохин")
	bkp := filepath.Join(t.TempDir(), "backup")
	if _, err := Backup(s, bkp); err != nil {
		t.Fatal(err)
	}
	res, err := Verify(s, bkp)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("Verify: %+v", res)
	}
	if res.ManifestMatches != true {
		t.Fatal("manifest должен совпадать")
	}
}

func TestVerifyDetectsCorruptionInSnapshot(t *testing.T) {
	dir := t.TempDir()
	s, _ := Open(dir)
	savePerson(t, s, "blohin", "Блохин")
	bkp := filepath.Join(t.TempDir(), "backup")
	if _, err := Backup(s, bkp); err != nil {
		t.Fatal(err)
	}
	// портим снапшот в бандле
	m, _ := ReadManifest(bkp)
	corrupt := m.SnapshotFile
	if err := writeGarbage(filepath.Join(bkp, corrupt), 512); err != nil {
		t.Fatal(err)
	}
	res, err := Verify(s, bkp)
	if err == nil && res.ManifestMatches {
		t.Fatal("подпорченный снапшот не должен проходить verify")
	}
}
```

- [ ] **Step 2: Запустить и убедиться, что падает**

Run: `go test ./internal/storage/ -run TestVerify`
Expected: FAIL — `undefined: Verify`.

- [ ] **Step 3: Реализация**

`internal/storage/verify.go`:

```go
package storage

import (
	"fmt"
	"os"
	"path/filepath"
)

// VerifyResult — детали проверки.
type VerifyResult struct {
	OK              bool
	Integrity       error
	ReplayedSeq     uint64
	AppliedSeq      uint64
	EntitiesByType  map[string]int
	JournalEntries  int
	ManifestMatches bool
}

// Verify проверяет целостность журнала и БД (без записи) и, если задан
// каталог бандла, свежесть копии бэкапа.
func Verify(s *Storage, backupDir string) (*VerifyResult, error) {
	res := &VerifyResult{EntitiesByType: map[string]int{}}
	res.Integrity = s.db.IntegrityCheck()
	if res.Integrity != nil {
		return res, nil // результат «бито», но вернём детали
	}

	// перечитываем журнал целиком и считаем записи (не меняя applied_seq)
	count := 0
	var last uint64
	err := s.journal.ReplayAll(func(e JournalEntry) error {
		count++
		last = e.Seq
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("journal: %w", err)
	}
	res.JournalEntries = count
	res.ReplayedSeq = last
	res.AppliedSeq, err = s.db.AppliedSeq()
	if err != nil {
		return nil, err
	}

	if backupDir != "" {
		m, err := ReadManifest(backupDir)
		if err != nil {
			return nil, fmt.Errorf("manifest: %w", err)
		}
		ok := true
		if got := fileSHA(filepath.Join(backupDir, m.SnapshotFile)); got != m.SnapshotSHA {
			ok = false
		}
		if got := fileSHA(filepath.Join(backupDir, m.JournalFile)); got != m.JournalSHA {
			ok = false
		}
		for et, want := range m.EntityCounts {
			n, err := s.db.Count(et)
			if err != nil {
				return nil, err
			}
			res.EntitiesByType[et] = n
			if n != want {
				ok = false
			}
		}
		res.ManifestMatches = ok
	}

	res.OK = res.Integrity == nil && (backupDir == "" || res.ManifestMatches)
	return res, nil
}

func (r *VerifyResult) String() string {
	return fmt.Sprintf(
		"verify ok=%v journal=%d replayed=%d applied=%d manifest_match=%v",
		r.OK, r.JournalEntries, r.ReplayedSeq, r.AppliedSeq, r.ManifestMatches,
	)
}

// helper для теста порчи файла
func writeGarbage(path string, size int) error {
	data := make([]byte, size)
	return os.WriteFile(path, data, 0o644)
}
```

Примечание: `ReplayAll` читает журнал с начала и декодирует построчно
(см. Task 2) — при битой последней строке вернёт ошибку. Такое поведение
нужно: partial-запись при крахе — признак проблемы, которой не должно быть
(журнал всегда fsync'ится до БД).

- [ ] **Step 4: Запустить и убедиться, что проходит**

Run: `go test ./internal/storage/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/verify.go internal/storage/verify_test.go
git commit -m "feat(storage): verify журнала, БД и манифеста бандла"
```

---

### Task 8: CLI — подкоманды, флаг `--data`, read-through в store

**Files:**
- Modify: `cmd/genodex/main.go`
- Create: `cmd/genodex/data.go` (resolveDataDir с sentinel-предупреждением) при необходимости
- Test: `internal/storage/sentinel_test.go` (запуск с `--data` помеченным) — по желанию, можно в T9 интеграционным тестом.

**Правила и решения (подтвердить у пользователя перед выполнением):**
1. Флаг `--data <dir>` (env `GENODEX_DATA`): каталог данных. Ранее данных не было — для сервера каталог просто создаётся; `store` наполняется read-through из SQLite.
2. Подкоманды: `genodex serve` (по умолчанию), `genodex backup --to <dir> [--keep N]`, `genodex restore --from <dir> --to <dir> [--force]`, `genodex verify [--backup <dir>]`. Старый запуск `genodex -p 9000` сохраняем как `serve` (без бэк-совместимых сюрпризов в этих флагах).
3. `--data` используется сервером для read-through: при старте весь SQLite читается в `store` (текущий in-memory MCP/API контракт не меняется). Записи со стороны MCP/API пока НЕ пишут в storage — storage подключается к интерфейсам отдельным PR (вне скоупа этого плана). То есть storage пока однонаправленный: SQLite → store.
4. Авто-бэкап при штатном останове сервера: `backup --auto` в каталог `$GENODEX_DATA/backups` + периодический (не реже раза в сутки, простой таймер). На старте — `verify` бандла, если существует.
5. Sentinel: если `db/` и `backup/` лежат на одном устройстве — warning. Проверка через `stat` блочный device ID.

**Метрика готовности (DoD Task 8):**
- `go build -o genealogy-mcp ./cmd/genodex` и `go vet ./...` без ошибок;
- `genodex --data /tmp/x serve` стартует, `/api/health` отвечает, `store` наполнен из пустой БД;
- `genodex backup --data /tmp/test --to /tmp/bkp` создаёт бандл; `genodex restore --from /tmp/bkp --to /tmp/restored` возвращает те же данные; `genodex verify --data /tmp/restored --backup /tmp/bkp` → ok.

- [ ] **Step 1: Открыть `cmd/genodex/main.go` и понять текущую структуру**

Прочитать `cmd/genodex/main.go`. Зафиксировать: как запускается сервер, как настраивается `store`, где выставляются route. Не менять HTTP/MCP контракты.

- [ ] **Step 2: Определить `--data` и подкоманды**

Добавить флаг `--data` (default `.`), разбор подкоманд. `serve` — текущее поведение сервера; прочие подкоманды — утилиты.

- [ ] **Step 3: read-through: SQLite → store**

При старте сервера открыть storage из `--data`, выполнить `List` по всем типам и наполнить `store`. Это повторный код из будущего репозитория — держать в одном месте `internal/store`? Нет: read-through живёт в `cmd/genodex`, чтобы не менять `store`. Ошибка чтения при старте — fatal (данные не должны молча теряться).

- [ ] **Step 4: `backup`, `restore`, `verify`**

Подкоманды вызывают `Backup`, `Restore`, `Verify`. `restore --force` разрешает запись поверх существующего `db/` (по умолчанию — отказ). Вывод — краткий отчёт (`VerifyResult.String()`).

- [ ] **Step 5: Авто-бэкап на останов**

Graceful shutdown (`SIGTERM`/`SIGINT`) → авто-бэкап. Периодический таймер (например, каждые 24ч) в том же цикле. Ошибки бэкапа логируются, не блокируют сервер.

- [ ] **Step 6: Sentinel**

При `Open(dataDir)`: `db/` и `backup/` на одном устройстве → warning в stderr на старте сервера (не fatal). Реализация: `os.Stat` → `syscall.Stat_t.Dev`.

- [ ] **Step 7: Commit**

```bash
git add cmd/genodex/
git commit -m "feat(cli): флаги --data, backup/restore/verify/auto-бэкап"
```

---

### Task 9: Документация и финальные проверки

**Files:**
- Modify: `docs/architecture.md` (добавить уровень хранения, схему потоков), `docs/usage.md` (команды), `AGENTS.md` (команды разработки: добавить `go test ./...` для storage, `genodex backup/restore/verify`)

**Steps:**
- [ ] **Step 1:** Обновить `docs/architecture.md` — диаграмма: `store` (in-memory, MCP/API) ← read-through ← `storage` (SQLite+journal) ← backup/restore/verify.
- [ ] **Step 2:** `docs/usage.md` — команды.
- [ ] **Step 3:** `AGENTS.md` — команды.
- [ ] **Step 4:** `gofmt -l .` не пуст? Нет — отформатировать.
- [ ] **Step 5:** `go build ./... && go vet ./...` (нужен `web/dist` для embed; при отсутствии выполнить `cd web && npm run build`).
- [ ] **Step 6:** `go test ./...`
- [ ] **Step 7:** Commit документов.

---

## Criteria of Completion (DoD всего плана)

1. `internal/storage` — пакет с `normalize.go`, `journal.go`, `db.go`, `storage.go`, `backup.go`, `restore.go`, `verify.go` (+ тесты), `go test ./internal/storage/` зелёный.
2. Инвариант «журнал не младше БД» доказан тестом `TestStorageRecoversWhenDBLost` (потеря БД при живом журнале восстанавливается).
3. CLI: `genodex backup/restore/verify` работают, read-through наполняет `store`, авто-бэкап на останов.
4. `gofmt -l .` пусто, `go build ./...`, `go vet ./...`, `go test ./...` зелёные.
5. Документация обновлена (architecture, usage, AGENTS).
6. Существующие MCP/HTTP контракты не изменены; `internal/store` не изменён.

## Decision Points (спросить пользователя перед/в ходе выполнения)

1. **Подтвердить разбор** первой подкоманды: `serve` — всегда по умолчанию? Или `genodex serve` required?
2. **Read-through**: загружать в `store` из SQLite при старте (однонаправленно) — ок? Или на этом этапе storage оставить полностью автономным (без read-through) до подключения интерфейсов?
3. **Авто-бэкап**: include в этом плане или отдельно (авто-бэкап требует выбора политики: расписание, ротация `--keep N`)?
4. **Sentinel**: warning ok или сделать fatal при одинаковом устройстве (страхование от ошибочного бэкапа)?

## Worth Noting (риски и открытые вопросы)

- **Модель сущностей не совпадает** между `internal/entity` (старая плоская) и `docs/data-model.md` (целевая). Этот план модель-агностичен (JSON-блобы) и не требует миграции; миграция — отдельный план.
- **`cgo` не используется**: `modernc.org/sqlite` — чистый Go, совместим с `CGO_ENABLED=0` и кросс-сборкой. `VACUUM INTO` поддерживается modernc.
- **`go.mod`** — go 1.26.4; `golang.org/x/text` добавлен как зависимость нормализации (задача T1).
- **AGENTS.md** упоминает `cmd/genealogy-mcp/main.go`, но фактическая точка входа — `cmd/genodex/main.go`; поправить в T9.
- **Naming**: `genealogy-mcp` остаётся именем бинарника; подкоманды добавятся к нему же.
