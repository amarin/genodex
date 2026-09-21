# S12: Мелочи — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Закрыть накопленные мелочи блока E: переименование констант `PersonGender*`, прагмы соединения в DSN (и удаление `DB.Tx` без `ctx`), тесты (реестр типов, транзакционность всех `Save*`/`Delete*`, порядок списков после `Backup`/`Restore`) и устаревшие абзацы документации.

**Architecture:** Четыре независимых задачи. (1) Механическое переименование констант `Male/Female/Unknown` → `PersonGenderMale/Female/Unknown` (потребители — только тесты и `enum_valid.go`). (2) `storage.OpenDB` строит DSN `file:<экранированный путь>?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)`: драйвер применяет прагмы к каждому новому соединению, а не только к первому (`PRAGMA foreign_keys` действует на соединение — пересозданное пулом соединение без DSN-прагм работало бы без внешних ключей); `DB.Tx` (без `ctx`, заменён `TxContext` в S6–S8) удаляется вместе с двумя тестами. (3) Тесты `sqlstore/registry_test.go`: согласованность реестров типов, транзакционность всех 21 `Save*` и 21 `Delete*` на `fullChain` (откат и фиксация внутри `InTx`), порядок списков после `Backup`/`Restore`. (4) Документация.

**Tech Stack:** Go 1.26.4, `modernc.org/sqlite`, `net/url`.

**Spec:** `docs/data-model/core-read-write.md` §6; `docs/plans/2026-09-20-core-rw-roadmap.md` (этап S12).

## Global Constraints

- Константы: `models.PersonGenderMale`, `PersonGenderFemale`, `PersonGenderUnknown` (значения `"male"`, `"female"`, `"unknown"` не меняются — схема и данные те же); старых имён нет в коде.
- DSN: путь БД экранируется (`url.URL{Path: p}.EscapedPath()`), прагмы — в `_pragma`-параметрах; `SetMaxOpenConns(1)` остаётся. Отдельных `PRAGMA …`-вызовов после открытия нет. Путь со спецсимволами (`пробел # ? %`) открывается ровно по указанному пути.
- `DB.Tx` удалён; `TxContext` и контекстные методы не меняются.
- Каждый из 21 `Save*` и 21 `Delete*` внутри откатываемого `InTx` не оставляет следов; внутри зафиксированного — записывает/удаляет (счётчики всех таблиц, включая value-таблицы, `search_index`, `source_links`).
- Порядок списков (`rowid`) не меняется после `Backup`+`Restore` (даже при дырах в `rowid` от удалений).
- Не меняются: схема БД, значения перечислений, контракты MCP/HTTP, исторические тексты планов (`docs/plans/`, встроенный в них код с прежними именами констант — история, источник истины — код).
- Комментарии и тексты ошибок — на русском; код gofmt-clean.
- Проверка рубежа: `gofmt -l .` пусто, `go build ./...`, `go vet ./...`, `go test ./...`.

---

### Task 1: Переименование констант `PersonGender*`

**Files:**
- Modify: `internal/models/person_gender.go`, `internal/models/enum_valid.go`, `internal/models/person_test.go`, `internal/models/person_validate_test.go`, `internal/store/sqlstore/*_test.go` (`sqlstore_test.go`, `batch_test.go`, `delete_test.go`, `list_test.go`)

**Interfaces:**
- Consumes: —
- Produces: `PersonGenderMale`, `PersonGenderFemale`, `PersonGenderUnknown`.

Скрипт проверен на чистой копии `main`: после него сборка, `vet` и все тесты зелёные.

- [ ] **Step 1: Скрипт замен**

Сохранить как `$TMPDIR/s12-rename.sh` (вне репозитория) и выполнить из корня: `bash $TMPDIR/s12-rename.sh`:

```bash
#!/usr/bin/env bash
# S12, задача 1: PersonGender-константы получают префикс типа. Из корня репозитория.
set -euo pipefail
perl -0pi -e 's/\tMale    PersonGender = "male"\n\tFemale  PersonGender = "female"\n\tUnknown PersonGender = "unknown"/\tPersonGenderMale    PersonGender = "male"\n\tPersonGenderFemale  PersonGender = "female"\n\tPersonGenderUnknown PersonGender = "unknown"/' internal/models/person_gender.go
perl -0pi -e 's/case Male, Female, Unknown:/case PersonGenderMale, PersonGenderFemale, PersonGenderUnknown:/' internal/models/enum_valid.go
perl -0pi -e 's/Gender: Female,/Gender: PersonGenderFemale,/' internal/models/person_validate_test.go internal/models/person_test.go
perl -0pi -e 's/\bmodels\.Male\b/models.PersonGenderMale/g; s/\bmodels\.Female\b/models.PersonGenderFemale/g; s/\bmodels\.Unknown\b/models.PersonGenderUnknown/g' internal/store/sqlstore/*_test.go
```

- [ ] **Step 2: Рубеж**

Run: `gofmt -l . && go build ./... && go vet ./... && go test ./... -count=1`
Expected: gofmt пусто, сборка и `vet` без ошибок, все пакеты `ok`.

- [ ] **Step 3: Старых имён нет**

Run: `grep -rnE "\b(Male|Female|Unknown)\b" --include=*.go internal | grep -v fact_date_test`
Expected: пусто (в `fact_date_test.go` — строка «Unknown().Precision», это другое).

- [ ] **Step 4: Commit**

```bash
git add -A internal
git commit -m "refactor(models): константы PersonGender с префиксом типа"
```

---

### Task 2: Прагмы соединения в DSN, удаление `DB.Tx`

**Files:**
- Modify: `internal/storage/db.go`, `internal/storage/storage.go`, `internal/storage/db_test.go`

**Interfaces:**
- Consumes: `OpenDB`, `openTestDB`, `TxContext` (S6–S8).
- Produces: `dsn(path string) string`; удалено `(*DB).Tx`; тесты `TestPragmasApplyToEveryConnection`, `TestOpenDBPathWithSpecialCharacters`.

- [ ] **Step 1: Тесты (падают — прагмы применяются только к первому соединению)**

Дописать в конец `internal/storage/db_test.go` (и добавить в import `"os"` и `"strings"` — в алфавитном порядке, рядом с `"fmt"`, `"path/filepath"`, `"sort"`, `"testing"`):

```go

// TestPragmasApplyToEveryConnection: прагмы заданы в DSN и действуют на любое
// новое соединение пула, а не только на первое (без этого пересозданное
// соединение работало бы без внешних ключей).
func TestPragmasApplyToEveryConnection(t *testing.T) {
	db := openTestDB(t)
	db.d.SetMaxIdleConns(0) // каждый запрос — новое соединение

	for i := 0; i < 3; i++ {
		var fk, busy int
		var journal string

		if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
			t.Fatal(err)
		}

		if err := db.QueryRow("PRAGMA busy_timeout").Scan(&busy); err != nil {
			t.Fatal(err)
		}

		if err := db.QueryRow("PRAGMA journal_mode").Scan(&journal); err != nil {
			t.Fatal(err)
		}

		if fk != 1 || busy != 5000 || journal != "wal" {
			t.Fatalf("соединение %d: foreign_keys=%d busy_timeout=%d journal_mode=%s, want 1/5000/wal", i, fk, busy, journal)
		}
	}
}

// TestOpenDBPathWithSpecialCharacters: путь с пробелом и символами, значимыми
// для URI (?, #, %), открывается ровно по этому пути.
func TestOpenDBPathWithSpecialCharacters(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "a b#c?d%e")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "genealogy.db")

	db, err := OpenDB(path)
	if err != nil {
		t.Fatalf("OpenDB(%q): %v", path, err)
	}

	if _, err := db.Exec("INSERT INTO persons(id) VALUES ('p-1')"); err != nil {
		t.Fatal(err)
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("файл БД не по запрошенному пути: %v", err)
	}

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), "genealogy.db") {
			t.Errorf("лишний файл рядом с БД: %s", e.Name())
		}
	}
}
```

- [ ] **Step 2: Убедиться, что тест прагм падает**

Run: `go test ./internal/storage/ -run Pragmas -count=1`
Expected: FAIL — «соединение 1: foreign_keys=0 …» (первое соединение получило прагмы через `Exec`, пересозданное — нет).

- [ ] **Step 3: Реализация**

Сохранить как `$TMPDIR/s12-storage.py` (вне репозитория) и выполнить из корня: `python3 $TMPDIR/s12-storage.py` (скрипт правит `db.go`, `storage.go` и удаляет `TestTxRollback`/`TestTxCommit` из `db_test.go`; при несовпадении фрагмента останавливается с сообщением — не подгонять молча):

```python
"""S12, задача 2: прагмы соединения — в DSN; DB.Tx без ctx удалён.
Запуск из корня репозитория."""


def edit(p, pairs):
    s = open(p, encoding="utf-8").read()
    for old, new in pairs:
        if old not in s:
            raise SystemExit(f"{p}: не найден фрагмент: {old[:60]!r}")
        s = s.replace(old, new, 1)
    open(p, "w", encoding="utf-8").write(s)


edit("internal/storage/db.go", [
    ("""// OpenDB открывает БД, включает WAL и foreign keys и создаёт схему целиком.
func OpenDB(path string) (*DB, error) {
	d, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	// одно соединение: PRAGMA-настройки (в т.ч. foreign_keys) действуют
	// на каждое соединение отдельно, единственное соединение гарантирует,
	// что включённые здесь прагмы действуют для всех запросов
	d.SetMaxOpenConns(1)

	for _, p := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := d.Exec(p); err != nil {
			d.Close()
			return nil, fmt.Errorf("open: %w", err)
		}
	}
""", """// dsn строит строку подключения: путь как file:-URI (спецсимволы пути
// экранируются) и прагмы, которые драйвер применяет к КАЖДОМУ новому
// соединению. PRAGMA foreign_keys действует на соединение, а не на базу:
// пересозданное пулом соединение без DSN-прагм потеряло бы внешние ключи.
func dsn(path string) string {
	u := url.URL{Path: path}

	return "file:" + u.EscapedPath() +
		"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
}

// OpenDB открывает БД, включает WAL и foreign keys и создаёт схему целиком.
func OpenDB(path string) (*DB, error) {
	d, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, err
	}
	// одно соединение: вложенный запрос при открытом курсоре встаёт в дедлок
	// (см. sqlstore); прагмы соединения заданы в DSN и не зависят от пула
	d.SetMaxOpenConns(1)
"""),
    ("""import (
	"context"
	"database/sql"
	"errors"
	"fmt"
""", """import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
"""),
])

s = open("internal/storage/db.go", encoding="utf-8").read()
a = s.find("// Tx выполняет fn в транзакции; откат")
b = s.find("// Count считает строки таблицы")
if a < 0 or b < 0:
    raise SystemExit("db.go: не найдены маркеры Tx/Count")
open("internal/storage/db.go", "w", encoding="utf-8").write(s[:a] + s[b:])

edit("internal/storage/storage.go", [
    ("""// Доступ к данным — через Storage.DB() (Exec/Query/QueryRow/Tx), которым
// пользуется internal/store/sqlstore.""",
     """// Доступ к данным — через Storage.DB() (Exec/Query/QueryRow и их *Context-
// варианты, TxContext), которым пользуется internal/store/sqlstore."""),
])

s = open("internal/storage/db_test.go", encoding="utf-8").read()
a = s.find("func TestTxRollback(")
b = s.find("func TestFKRestrict(")
if a < 0 or b < 0:
    raise SystemExit("db_test.go: не найдены TestTxRollback/TestFKRestrict")
s = s[:a] + s[b:]
imp = '\t"fmt"\n\t"path/filepath"\n\t"sort"\n\t"testing"'
if imp not in s:
    raise SystemExit("db_test.go: не найден блок import")
s = s.replace(imp, '\t"fmt"\n\t"os"\n\t"path/filepath"\n\t"sort"\n\t"strings"\n\t"testing"', 1)
open("internal/storage/db_test.go", "w", encoding="utf-8").write(s)
```

- [ ] **Step 4: Рубеж**

Run: `gofmt -l . && go build ./... && go vet ./... && go test ./... -count=1`
Expected: gofmt пусто, сборка и `vet` без ошибок, все пакеты `ok`.

- [ ] **Step 5: Мутация (вручную, откатить сразу: `git checkout -- internal/storage/db.go`)**

В `dsn` убрать `&_pragma=foreign_keys(1)` из строки → `TestPragmasApplyToEveryConnection` должен ПРОВАЛИТЬСЯ («foreign_keys=0»).

- [ ] **Step 6: Commit**

```bash
git add -A internal
git commit -m "fix(storage): прагмы соединения в DSN; удалён DB.Tx без ctx"
```

---

### Task 3: Тесты реестра, транзакционности и порядка после восстановления

**Files:**
- Create: `internal/store/sqlstore/registry_test.go`

**Interfaces:**
- Consumes: `fullChain`, `snapshot`, `deleters`, `getters`, `listers` (`delete_test.go`, `context_test.go`), `withTimeout`, `errBoom` (`tx_test.go`), `scanRowsStrings` (`fkgraph_test.go`), `entityTables`, `typeOfTable`, `newStore`, `mustDo`; `idgen.New`, `models.ParseID`, `storage.Backup`/`Restore`.
- Produces: тесты `TestTypeRegistryIsConsistent`, `TestSaveAndDeleteMethodsAreCoveredByChain`, `TestEverySaveAndDeleteIsTransactional`, `TestListOrderSurvivesBackupAndRestore`.

- [ ] **Step 1: Создать тесты**

Создать `internal/store/sqlstore/registry_test.go` и выполнить на нём `gofmt -w`:

```go
package sqlstore

import (
	"errors"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/amarin/genodex/internal/idgen"
	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/storage"
	"github.com/amarin/genodex/internal/store"
)

// TestTypeRegistryIsConsistent: тип модели ↔ таблица главной строки ↔
// entity_table в search_index ↔ префикс ID согласованы для всех 21 видов.
func TestTypeRegistryIsConsistent(t *testing.T) {
	s := newStore(t)

	for _, st := range fullChain() {
		mustDo(t, "save "+st.kind+" "+string(st.id), st.save(t.Context(), s))
	}

	// таблицы, реально пишущие термины: все сущности, кроме relations и residences
	indexed, err := scanRowsStrings(s, `SELECT DISTINCT entity_table FROM search_index ORDER BY entity_table`)
	mustDo(t, "search_index tables", err)

	var want []string

	for _, table := range entityTables {
		if table != "relations" && table != "residences" {
			want = append(want, table)
		}
	}

	sort.Strings(want)

	if !reflect.DeepEqual(indexed, want) {
		t.Fatalf("entity_table в search_index %v, ожидались таблицы сущностей с терминами %v", indexed, want)
	}

	gen := idgen.New()
	prefixes := map[string]models.Type{}

	for typ, table := range entityTables {
		if typeOfTable[table] != typ {
			t.Errorf("typeOfTable[%s] = %q, ожидался %q", table, typeOfTable[table], typ)
		}

		prefix := typ.IDPrefix()
		if prefix == "" {
			t.Errorf("у типа %q нет префикса ID", typ)

			continue
		}

		if other, dup := prefixes[prefix]; dup {
			t.Errorf("префикс %q у типов %q и %q", prefix, other, typ)
		}

		prefixes[prefix] = typ

		parsed, err := models.ParseID(gen.New(typ))
		if err != nil || parsed != typ {
			t.Errorf("ParseID(New(%q)) = %q, %v; ожидался тот же тип", typ, parsed, err)
		}
	}
}

// TestSaveAndDeleteMethodsAreCoveredByChain: каждому Save*/Delete*-методу
// адаптера соответствует вид из fullChain и таблица getters — новый метод без
// записи в таблицах не пройдёт.
func TestSaveAndDeleteMethodsAreCoveredByChain(t *testing.T) {
	kinds := map[string]bool{}
	for _, st := range fullChain() {
		kinds[st.kind] = true
	}

	var saves, deletes int

	typ := reflect.TypeOf(&Store{})

	for i := 0; i < typ.NumMethod(); i++ {
		name := typ.Method(i).Name

		switch {
		case strings.HasPrefix(name, "Save"):
			saves++

			if !kinds[strings.TrimPrefix(name, "Save")] {
				t.Errorf("метод %s не покрыт fullChain", name)
			}
		case strings.HasPrefix(name, "Delete"):
			deletes++

			if !kinds[strings.TrimPrefix(name, "Delete")] {
				t.Errorf("метод %s не покрыт fullChain", name)
			}
		}
	}

	if saves != len(kinds) || deletes != len(kinds) || len(kinds) != len(getters) {
		t.Fatalf("Save*=%d Delete*=%d видов в fullChain=%d getters=%d — ожидалось поровну", saves, deletes, len(kinds), len(getters))
	}
}

// TestEverySaveAndDeleteIsTransactional: внутри InTx, завершившегося ошибкой,
// все 21 Save* и все 21 Delete* не оставляют следов в таблицах, а при успехе —
// фиксируются. Метод, написанный мимо транзакции Store, повис бы на единственном
// соединении (тест ограничен по времени) или оставил бы следы.
func TestEverySaveAndDeleteIsTransactional(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)

	steps := fullChain()
	before := snapshot(t, s)

	saveAll := func(fail bool) error {
		return s.InTx(ctx, func(tx store.Store) error {
			scoped := tx.(*Store)

			for _, st := range steps {
				if err := st.save(ctx, scoped); err != nil {
					return err
				}
			}

			if fail {
				return errBoom
			}

			return nil
		})
	}

	deleteAll := func(fail bool) error {
		return s.InTx(ctx, func(tx store.Store) error {
			scoped := tx.(*Store)

			for i := len(steps) - 1; i >= 0; i-- {
				if err := deleters[steps[i].kind](scoped, ctx, steps[i].id); err != nil {
					return err
				}
			}

			if fail {
				return errBoom
			}

			return nil
		})
	}

	if err := saveAll(true); !errors.Is(err, errBoom) {
		t.Fatalf("saveAll(fail) = %v, ожидалась errBoom", err)
	}

	if got := snapshot(t, s); !reflect.DeepEqual(got, before) {
		t.Fatalf("откатанные Save* оставили следы:\n got  %v\n want %v", got, before)
	}

	mustDo(t, "saveAll", saveAll(false))

	full := snapshot(t, s)
	if reflect.DeepEqual(full, before) {
		t.Fatal("зафиксированные Save* ничего не записали — проверка пуста")
	}

	if err := deleteAll(true); !errors.Is(err, errBoom) {
		t.Fatalf("deleteAll(fail) = %v, ожидалась errBoom", err)
	}

	if got := snapshot(t, s); !reflect.DeepEqual(got, full) {
		t.Fatalf("откатанные Delete* изменили таблицы:\n got  %v\n want %v", got, full)
	}

	mustDo(t, "deleteAll", deleteAll(false))

	if got := snapshot(t, s); !reflect.DeepEqual(got, before) {
		t.Fatalf("зафиксированные Delete* не вернули таблицы к исходным:\n got  %v\n want %v", got, before)
	}
}

// TestListOrderSurvivesBackupAndRestore: порядок списков (rowid) после
// Backup/Restore (VACUUM INTO) тот же, даже если в rowid были дыры от удалений.
func TestListOrderSurvivesBackupAndRestore(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()

	for _, id := range []models.ID{"p-c", "p-a", "p-b", "p-d"} {
		mustDo(t, "person", s.SavePerson(ctx, &models.Person{ID: id}))
	}

	mustDo(t, "delete", s.DeletePerson(ctx, "p-a")) // дыра в rowid
	mustDo(t, "person", s.SavePerson(ctx, &models.Person{ID: "p-e"}))

	for _, id := range []models.ID{"ad-2", "ad-1", "ad-3"} {
		mustDo(t, "division", s.SaveAdministrativeDivision(ctx, &models.AdministrativeDivision{
			ID: id, Name: string(id), Type: models.AdminDivisionDerevnya}))
	}

	order := func(st *Store) (people, divisions []models.ID) {
		p, err := listers["People"].list(st, ctx, models.AccessFull, models.Page{})
		mustDo(t, "list people", err)

		d, err := listers["AdministrativeDivisions"].list(st, ctx, models.AccessFull, models.Page{})
		mustDo(t, "list divisions", err)

		return p, d
	}

	wantPeople, wantDivisions := order(s)
	if want := []models.ID{"p-c", "p-b", "p-d", "p-e"}; !reflect.DeepEqual(wantPeople, want) {
		t.Fatalf("порядок до бэкапа %v, ожидался %v", wantPeople, want)
	}

	dir := t.TempDir()
	bundle := filepath.Join(dir, "bundle")

	_, err := storage.Backup(s.st, bundle)
	mustDo(t, "backup", err)

	rs, err := storage.Restore(bundle, filepath.Join(dir, "restored"))
	mustDo(t, "restore", err)

	restored := New(rs)
	t.Cleanup(func() { _ = restored.Close() })

	gotPeople, gotDivisions := order(restored)

	if !reflect.DeepEqual(gotPeople, wantPeople) || !reflect.DeepEqual(gotDivisions, wantDivisions) {
		t.Fatalf("порядок после восстановления изменился:\n люди %v → %v\n деления %v → %v",
			wantPeople, gotPeople, wantDivisions, gotDivisions)
	}
}
```

- [ ] **Step 2: Прогнать**

Run: `gofmt -l . && go vet ./... && go test ./... -count=1`
Expected: gofmt пусто, `vet` чисто, все пакеты `ok`.

- [ ] **Step 3: Мутации (вручную; каждую откатить сразу: `git checkout -- <файл>`)**

| Мутация | Ожидаемый провал |
|---|---|
| в `DeleteFamily` (`delete.go`) заменить `return s.deleteEntity(ctx, models.TypeFamily, id)` на `return s.db.TxContext(ctx, func(*sql.Tx) error { return nil })` | `TestEverySaveAndDeleteIsTransactional` (через 10 с: «context deadline exceeded») |
| в `models/id_prefix.go` дать `TypeNote` тот же префикс, что у `TypeFamily` (`"F"`) | `TestTypeRegistryIsConsistent` («префикс "F" у типов …») |

- [ ] **Step 4: Commit**

```bash
git add internal/store/sqlstore/registry_test.go
git commit -m "test(store): реестр типов, транзакционность Save*/Delete*, порядок после Restore"
```

---

### Task 4: Устаревшие абзацы документации

**Files:**
- Modify: `docs/data-model/normalization-s1s2.md`, `docs/superpowers/specs/2026-09-19-storage-design.md`, `docs/todo.md`, `docs/data-model/core-read-write.md` (§6), `docs/plans/2026-09-20-core-rw-roadmap.md` (S12)

Правки — инструментом Edit с точным старым текстом (сначала прочитать файл). При несовпадении старого текста остановиться и сообщить.

- [ ] **Step 1: `docs/data-model/normalization-s1s2.md` — пять правок**

(а) Заменить

```
Поведение при удалении цели строгой ссылки — `RESTRICT` (нельзя удалить персону,
на которую есть `Relation`), при удалении владельца `source_links`/`text_refs` —
каскад.
```

на

```
Поведение при удалении цели строгой ссылки — `RESTRICT` (нельзя удалить персону,
на которую есть `Relation`). При удалении владельца каскад сносит только его
связные строки; `source_links`, `search_index` и осиротевшие строки
`text_refs`/`dates`/`anchors` внешнего ключа на владельца не имеют, поэтому их
чистит `sqlstore` явно (`Delete*`, S7); `source_links.citation_id` — `RESTRICT`.
```

(б) Заменить

```
  sqlstore чистит их явно (сейчас — при `Save`; любой будущий `Delete*` обязан
  делать то же).
```

на

```
  sqlstore чистит их явно (при `Save` и при `Delete*`, S7).
```

(в) Заменить

```
- Запрос: `WHERE term LIKE ?` с параметром = уже lowered строка + `%`
  (без участия sqlite-коллаций — регистронезависимость обеспечена нормализацией
  обеих сторон в Go).
```

на

```
- Запрос (S11): диапазон `term >= p AND term < p+U+10FFFF` по покрывающему индексу
  `idx_search_term`, `p` — префикс, нормализованный так же, как термины (без
  участия sqlite-коллаций — регистронезависимость обеспечена нормализацией обеих
  сторон в Go); см. `core-read-write.md` §4.
```

(г) Заменить

```
`internal/models`. Методы `GetSettlement/...` удаляются. `go:generate mockgen`
сохраняется.
```

на

```
`internal/models`. Методы `GetSettlement/...` удаляются. `go:generate mockgen`
сохраняется. Позже порт расширен методами `DeleteX`, `Search`,
`ChildrenOfDivision`, `InTx` и параметрами `ctx`, `Access`, `Page` — актуальный
контракт в `core-read-write.md` §4.
```

(д) Заменить `INSERT; удаление — каскад по FK).` на `INSERT; удаление — `Delete*` по графу внешних ключей, см. `core-read-write.md` §4).`

- [ ] **Step 2: `docs/superpowers/specs/2026-09-19-storage-design.md` — две правки**

(а) Заменить

```
- Первичный ключ — строковый `id` из модели (транслит + дисамбигуатор, см.
  `docs/data-model/identifiers.md`).
```

на

```
- Первичный ключ — строковый `id` из модели (формат `ПРЕФИКС-ULID`, см.
  `docs/data-model/identifiers.md`; правило «транслит + дисамбигуатор» отменено).
```

(б) После строки `> После ревью этот документ становится отправной точкой плана реализации (writing-plans).` добавить (через пустую строку с `>`):

```
>
> **Историческая заметка (S12):** дизайн первого прохода (плоская схема,
> `*_norm`-колонки, внешний журнал) заменён колоночной схемой и «снапшот +
> манифест»; актуальные документы — `docs/data-model/normalization-s1s2.md` и
> `docs/data-model/core-read-write.md`.
```

- [ ] **Step 3: `docs/todo.md` — четыре правки**

(а) Заменить раздел целиком (от `## Открытые вопросы` до строки перед `## Паспорт прохода`):

```
## Открытые вопросы

- `settlement` оставлена отдельной сущностью (публичный контракт `settlement_list`).
  Решить: закрепить отдельной сущностью или сделать уровнем `AdministrativeDivision.type`.
- Словари (`surname`, `given_name`, `patronymic`, `estate`, `title`) — текстовые ссылки из
  `PersonName.TextRef` против отдельных строк в таблицах словарей (решение в `decisions.md` #?).
- `family` и связи `person ↔ person` (родители/супруги, `Relation.kind`) — порядок введения.
```

на

```
## Открытые вопросы

Решены (для истории): `settlement` — вид `AdministrativeDivision.type` (решение
#17; публичный контракт `settlement_list` переименовывается на этапе S13);
словари — отдельные таблицы (`surnames` и др.); `Family` и `Relation` введены.

- Форма ошибки валидации: сейчас первая ошибка (`*ValidationError`), список
  (`ValidationErrors`) — решается на этапе S15.
- `Access` во внешних контрактах: обработчики публичного API должны передавать
  режим доступа из запроса, а не выбирать его в сценарии (S13–S16).
- Консолидация «план + спецификация → `docs/implementation/`» и запись в
  `CHANGELOG.md` по программе «ядро чтения и записи» — одним проходом после S16.

```

(б) В блоке `### D. Встроенные данные` после строки `- [ ] `internal/definitions/russia` переведён на нормализованную модель административного деления.` добавить строки:

```
  Сейчас словарь временный: константы — слова для показа (`"губерния"`), а не
  канонические значения `models.AdminDivisionType` (`governorate`); пометка есть в
  комментариях файлов пакета.
```

(в) В строке `- [x] **`internal/entity/entity_type.go` согласован с `decisions.md`** (противоречие устранено в пользу решений):` заменить `(противоречие устранено в пользу решений):` на `(противоречие устранено в пользу решений; историческая запись прохода — пакет `internal/entity` затем упразднён, актуальный набор типов — `models.AllTypes()`):`

(г) В разделе `## Следующий проход: ядро чтения и записи` после ссылки `[plans/2026-09-20-core-rw-roadmap.md](plans/2026-09-20-core-rw-roadmap.md).` добавить: ` Выполнены S1–S12 (доменные правила, индексы, порт: `ctx`, `ErrNotFound`, `Delete*`, `InTx`, `Page`/`Access`, пакетная загрузка, `Search`, мелочи); осталось S13–S16 (контракты и срез на делениях).`

- [ ] **Step 4: `docs/data-model/core-read-write.md` §6**

Заменить раздел целиком (от `## 6. Мелочи (этап E)` до строки перед `## 7. Порядок работ`):

```
## 6. Мелочи (этап E)

- `PersonGender` константы: `Male/Female/Unknown` → `PersonGenderMale/Female/Unknown`
  (как в спеке S1+S2 §2).
- Тест, фиксирующий соответствие `models.Type` ↔ реестр таблиц ↔
  `search_index.entity_table` ↔ префикс `ID`.
- `List*`-тесты для всех 21 типа.
- Устаревшие абзацы: каскад `source_links`/`text_refs` в спеке S1+S2 §5,
  раздел «Открытые вопросы» в `docs/todo.md`, пометка о несоответствии
  словаря `internal/definitions/russia` (блок D `docs/todo.md`).
```

на

```
## 6. Мелочи (этап E)

Выполнено в S12:

- `PersonGender` константы: `Male/Female/Unknown` → `PersonGenderMale/Female/Unknown`.
- Тест соответствия `models.Type` ↔ реестр таблиц ↔ `search_index.entity_table` ↔
  префикс `ID` (`sqlstore/registry_test.go`).
- `List*`-тесты для всех 21 типа — закрыты в S9 (`TestListPagesPartitionEveryKind`).
- Тест транзакционности порта (все 21 `Save*` и 21 `Delete*` внутри откатываемого
  `InTx`) и тест порядка списков после `Backup`/`Restore`.
- Прагмы соединения (`foreign_keys`, `busy_timeout`, `journal_mode`) — в DSN, а
  значит действуют на каждое соединение пула; `DB.Tx` без `ctx` удалён.
- Устаревшие абзацы: каскад `source_links`/`text_refs`, механизм поиска и порт в
  `normalization-s1s2.md`, «Открытые вопросы» в `docs/todo.md`, пометка о словаре
  `internal/definitions/russia`, ссылка на отменённое правило id в
  `docs/superpowers/specs/2026-09-19-storage-design.md`.

Отклонено: ошибка «не найдено» с видом и id сущности — обработчик знает, что
запрашивал, а `errors.Is(err, ErrNotFound)` достаточно.

```

- [ ] **Step 5: `docs/plans/2026-09-20-core-rw-roadmap.md` (S12)**

Заменить блок (от `### S12. Мелочи` до строки перед `## Часть D`):

```
### S12. Мелочи
- `PersonGender*` переименование (только тесты).
- Тест соответствия `models.Type` ↔ реестр таблиц ↔ `search_index.entity_table` ↔
  префикс `ID`.
- `List*`-тесты для всех 21 типа.
- Тест порядка списков после `Backup`/`Restore` (`VACUUM INTO`): SQLite может
  перенумеровать `rowid` у таблиц без `INTEGER PRIMARY KEY`, порядок должен
  сохраниться (проверено вручную, тестом не закреплено).
- Тест транзакционности порта: каждый `Save*`/`Delete*` внутри откатываемого
  `InTx` не оставляет следов (сейчас `TestPortMethodsAreCovered` считает только
  `Get*`/`List*`, и новый метод, написанный мимо `s.run`/`s.inTx`, обошёл бы
  транзакцию молча); `DB.Tx` без ctx удалить, когда не останется тестов на нём.
- DSN `_pragma` для `foreign_keys` и `busy_timeout`.
- Устаревшие абзацы: спека S1+S2 §5 (каскад), `docs/todo.md` (открытые вопросы,
  новый блок), пометка о словаре `definitions/russia`,
  `docs/superpowers/specs/2026-09-19-storage-design.md` (ссылка на отменённое
  правило id «транслит + дисамбигуатор»).
- **Приёмка:** дерево зелёное; в `docs/` нет утверждений, противоречащих коду.
```

на

```
### S12. Мелочи
- **Файлы:** `models/person_gender.go` (константы `PersonGender*`),
  `storage/db.go` (DSN-прагмы, удалён `Tx`), тесты (`sqlstore/registry_test.go`,
  `storage/db_test.go`), доки; план — `2026-09-21-core-rw-s12-small.md`.
- **Сделано:** переименование констант; прагмы соединения в DSN; реестр типов;
  транзакционность всех `Save*`/`Delete*`; порядок списков после
  `Backup`/`Restore`; устаревшие абзацы (`normalization-s1s2.md`, `todo.md`,
  `storage-design.md`). `List*`-тесты для всех 21 типа закрыты в S9.
- **Приёмка:** дерево зелёное; в `docs/` (кроме исторических планов) нет
  утверждений, противоречащих коду.

```

- [ ] **Step 6: Проверка и commit**

Run: `grep -n "LIKE ?" docs/data-model/normalization-s1s2.md; grep -n "транслит + дисамбигуатор" docs/superpowers/specs/2026-09-19-storage-design.md`
Expected: первая команда пусто; вторая — только строка с пометкой «отменено».

```bash
git add docs
git commit -m "docs: S12 — устаревшие абзацы, статусы этапа"
```

---

## Правки по итогам ревью (внесены после выполнения задач)

Код в репозитории — источник истины; плановые блоки выше описывают первую версию. Отличия:

- Task 2, Step 2: до правки тест прагм падает уже на соединении 0, а не 1 (`SetMaxIdleConns(0)` закрывает исходное соединение до первого запроса); суть дефекта та же.
- `registry_test.go`: охват методов дополнен проверкой `getters`/`deleters` по видам `fullChain`, в порядке списков после `Restore` явно проверен порядок делений.
