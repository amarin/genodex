# S6: `ErrNotFound` и `ctx` в порту — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Все методы порта `store.Store` принимают `ctx context.Context` первым аргументом, а `Get*` для отсутствующей сущности возвращает `models.ErrNotFound` вместо `(nil, nil)`.

**Architecture:** `models.ErrNotFound` — листовая ошибка домена. `storage.DB` получает контекстные методы (`ExecContext`, `QueryContext`, `QueryRowContext`, `TxContext`). В `sqlstore` вводится `runner` — исполнитель запросов с привязанным `ctx` поверх соединения или транзакции; он реализует те же `Exec/Query/QueryRow`, поэтому помощники (`insertDate`, `loadTextRef`, …) меняют только тип параметра `*sql.Tx` → `runner`, а отмена `ctx` доходит до драйвера. Механическая часть (около 300 подписей и вызовов) выполняется двумя проверенными скриптами; тесты и документация — вручную.

**Tech Stack:** Go 1.26.4, `database/sql`, `modernc.org/sqlite`, `go.uber.org/mock` (mockgen).

**Spec:** `docs/data-model/core-read-write.md` §4; `docs/plans/2026-09-20-core-rw-roadmap.md` (этап S6).

## Global Constraints

- Порт: `Get<E>(ctx, id)`, `Save<E>(ctx, e)`, `List<E>(ctx)` для всех 21 сущностей; `ctx` — всегда первый аргумент.
- `Get*` для отсутствующей сущности возвращает `models.ErrNotFound` (проверка `errors.Is`); остальные ошибки — сбой хранилища. `(nil, nil)` из порта исчезает.
- Отменённый `ctx` даёт ошибку контекста (`errors.Is(err, context.Canceled)`), а не `ErrNotFound`; отменённый `Save*` ничего не пишет; ошибка `Save*` по-прежнему называет вид и id сущности.
- `List*` пропускает строку, исчезнувшую между чтением id и `Get` (`ErrNotFound` внутри `listEntities`), любую другую ошибку `Get` возвращает.
- Внутренние помощники `helpers.go` (`loadTextRefPtr`, `loadDate`, `loadAnchor`, `collectMainRefs`) сохраняют `(nil, nil)` — это не порт.
- Соединение одно (`SetMaxOpenConns(1)`): вложенные запросы при открытом курсоре не допускаются (правило не меняется).
- Схема БД, `Validate`, контракты MCP/HTTP не меняются; порядок и содержимое ответов прежние.
- Комментарии и тексты ошибок — на русском; код gofmt-clean.
- Проверка рубежа: `gofmt -l .` пусто, `go build ./...`, `go vet ./...`, `go test ./...`.

---

### Task 1: `models.ErrNotFound` и контекстные методы `storage.DB`

**Files:**
- Create: `internal/models/errors.go`, `internal/models/errors_test.go`
- Modify: `internal/storage/db.go`, `internal/storage/db_test.go`

**Interfaces:**
- Consumes: `DB` (`storage/db.go`), `openTestDB` (`db_test.go`).
- Produces: `models.ErrNotFound`; `(*DB).ExecContext/QueryContext/QueryRowContext(ctx, query, args...)`, `(*DB).TxContext(ctx, fn func(tx *sql.Tx) error) error`. Прежние `Exec/Query/QueryRow/Tx` остаются.

- [ ] **Step 1: Тесты (падают — символов ещё нет)**

Создать `internal/models/errors_test.go`:

```go
package models

import (
	"errors"
	"fmt"
	"testing"
)

func TestErrNotFoundMatchesThroughWrapping(t *testing.T) {
	wrapped := fmt.Errorf("get person %q: %w", "p-1", ErrNotFound)
	if !errors.Is(wrapped, ErrNotFound) {
		t.Fatal("errors.Is не находит ErrNotFound в обёрнутой ошибке")
	}

	// сопоставление по значению ошибки, а не по тексту
	if errors.Is(errors.New(ErrNotFound.Error()), ErrNotFound) {
		t.Fatal("посторонняя ошибка с тем же текстом не должна совпадать с ErrNotFound")
	}
}
```

В `internal/storage/db_test.go` добавить в import `"context"` и `"errors"` (в алфавитном порядке, рядом с `"database/sql"`, `"fmt"`), а в конец файла дописать:

```go

func TestContextMethodsHonourCancellation(t *testing.T) {
	db := openTestDB(t)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	if _, err := db.ExecContext(ctx, "INSERT INTO persons(id) VALUES ('p-1')"); !errors.Is(err, context.Canceled) {
		t.Errorf("ExecContext = %v, want context.Canceled", err)
	}

	if rows, err := db.QueryContext(ctx, "SELECT id FROM persons"); !errors.Is(err, context.Canceled) {
		if rows != nil {
			_ = rows.Close()
		}

		t.Errorf("QueryContext = %v, want context.Canceled", err)
	}

	if err := db.QueryRowContext(ctx, "SELECT 1").Scan(new(int)); !errors.Is(err, context.Canceled) {
		t.Errorf("QueryRowContext = %v, want context.Canceled", err)
	}

	err := db.TxContext(ctx, func(*sql.Tx) error {
		t.Error("fn не должна вызываться при отменённом контексте")

		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("TxContext = %v, want context.Canceled", err)
	}

	if n, _ := db.Count("persons"); n != 0 {
		t.Fatalf("Count persons=%d после отменённых запросов, want 0", n)
	}
}

func TestTxContextCommitAndRollback(t *testing.T) {
	db := openTestDB(t)

	if err := db.TxContext(t.Context(), func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO persons(id) VALUES ('p-1')")

		return err
	}); err != nil {
		t.Fatal(err)
	}

	err := db.TxContext(t.Context(), func(tx *sql.Tx) error {
		if _, err := tx.Exec("INSERT INTO persons(id) VALUES ('p-2')"); err != nil {
			return err
		}

		return fmt.Errorf("boom")
	})
	if err == nil {
		t.Fatal("expected error")
	}

	if n, _ := db.Count("persons"); n != 1 {
		t.Fatalf("Count persons=%d, want 1 (p-1 закоммичена, p-2 откатана)", n)
	}
}
```

- [ ] **Step 2: Убедиться, что тесты не компилируются**

Run: `go test ./internal/models/ ./internal/storage/`
Expected: FAIL — `undefined: ErrNotFound`, `db.ExecContext undefined`.

- [ ] **Step 3: Реализация**

Создать `internal/models/errors.go`:

```go
package models

import "errors"

// ErrNotFound — запрошенной сущности нет. Get-методы порта возвращают её
// (проверять через errors.Is); обработчики превращают в «не найдено».
var ErrNotFound = errors.New("не найдено")
```

В `internal/storage/db.go` добавить в import `"context"` (перед `"database/sql"`) и вставить перед комментарием `// Tx выполняет fn в транзакции; при ошибке — откат.` методы:

```go
// ExecContext — Exec с контекстом: отмена доходит до драйвера.
func (d *DB) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return d.d.ExecContext(ctx, query, args...)
}

// QueryContext — Query с контекстом.
func (d *DB) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return d.d.QueryContext(ctx, query, args...)
}

// QueryRowContext — QueryRow с контекстом.
func (d *DB) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return d.d.QueryRowContext(ctx, query, args...)
}

// TxContext выполняет fn в транзакции, привязанной к ctx; при ошибке — откат.
func (d *DB) TxContext(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := d.d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
```

- [ ] **Step 4: Тесты проходят**

Run: `gofmt -l internal && go test ./internal/models/ ./internal/storage/`
Expected: пусто от gofmt, оба пакета `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/models/errors.go internal/models/errors_test.go internal/storage/db.go internal/storage/db_test.go
git commit -m "feat(models,storage): ErrNotFound и методы БД с контекстом"
```

---

### Task 2: Порт и адаптер: `ctx` везде, `ErrNotFound` из `Get*`

**Files:**
- Modify: `internal/store/deps.go`, `internal/store/deps_test.go` (генерируется), `internal/store/sqlstore/*.go` (все, включая `sqlstore_test.go`), `internal/usecases/list_settlements/{deps,scenario,scenario_test,deps_test}.go`

**Interfaces:**
- Consumes: Task 1 (`models.ErrNotFound`, `DB.*Context`).
- Produces: порт `store.Store` с `ctx` во всех 63 методах; `sqlstore.runner`, `(*Store).run(ctx) runner`, `(*Store).inTx(ctx, fn func(tx runner) error) error`; `listEntities(ctx, s, table, get)`; `getDictionary` возвращает `(dictValue, error)`; `searchIDs(ctx, table, query)`; репозиторий сценария `ListAdministrativeDivisions(ctx)`.

Скрипты ниже проверены на чистой копии `main` (после Task 1): после них и шагов 3–4 сборка, `vet` и все тесты зелёные. Запускать из корня репозитория, скрипты сохранить во временный каталог вне репозитория (например, `$TMPDIR`).

- [ ] **Step 1: Сохранить и запустить скрипт замен**

Сохранить как `$TMPDIR/s6-transform.sh` и выполнить `bash $TMPDIR/s6-transform.sh`:

```bash
#!/usr/bin/env bash
# S6, шаг 1 задачи 2: механические замены (ctx в подписях, runner вместо *sql.Tx,
# ErrNotFound вместо (nil, nil)). Запуск из корня репозитория.
set -euo pipefail
S=internal/store/sqlstore
NONTEST=$(ls $S/*.go | grep -v _test.go)
HELPERS=$(ls $S/*.go | grep -v '_test.go\|/sqlstore.go')

# Подписи методов порта: адаптер и интерфейс.
perl -0pi -e '
  s/(func \(s \*Store\) Get\w+)\(id models\.ID\)/$1(ctx context.Context, id models.ID)/g;
  s/(func \(s \*Store\) Save\w+)\((\w+) \*/$1(ctx context.Context, $2 */g;
  s/(func \(s \*Store\) List\w+)\(\)/$1(ctx context.Context)/g;
' $NONTEST
perl -0pi -e '
  s/^(\t(?:Get|Save|List)\w+)\(id models\.ID\)/$1(ctx context.Context, id models.ID)/mg;
  s/^(\t(?:Get|Save|List)\w+)\((\w+) \*/$1(ctx context.Context, $2 */mg;
  s/^(\t(?:Get|Save|List)\w+)\(\)/$1(ctx context.Context)/mg;
' internal/store/deps.go

# Транзакции и прямые обращения к соединению — через ctx-привязанный runner.
perl -0pi -e 's/s\.db\.Tx\(func\(tx \*sql\.Tx\) error \{/s.inTx(ctx, func(tx runner) error {/g' $NONTEST
perl -0pi -e 's/\bs\.db\b/s.run(ctx)/g' $NONTEST
perl -0pi -e 's/\(tx \*sql\.Tx\b/(tx runner/g; s/\btx \*sql\.Tx,/tx runner,/g' $HELPERS

# Внутренние методы Store получают ctx.
perl -0pi -e '
  s/func \(s \*Store\) saveDictionary\(spec/func (s *Store) saveDictionary(ctx context.Context, spec/;
  s/func \(s \*Store\) getDictionary\(spec/func (s *Store) getDictionary(ctx context.Context, spec/;
  s/s\.saveDictionary\(/s.saveDictionary(ctx, /g;
  s/s\.getDictionary\(/s.getDictionary(ctx, /g;
' $S/dictionaries.go
perl -0pi -e 's/func \(s \*Store\) searchIDs\(table, query string\)/func (s *Store) searchIDs(ctx context.Context, table, query string)/' $S/helpers.go
perl -0pi -e 's/listEntities\(s, /listEntities(ctx, s, /g' $NONTEST

# Get: нет строки — models.ErrNotFound (кроме внутренних помощников helpers.go).
for f in $NONTEST; do
  [ "$f" = "$S/helpers.go" ] && continue
  perl -0pi -e 's/(if notFound\(err\) \{\n\t\t)return nil, nil/$1return nil, models.ErrNotFound/g' "$f"
done
perl -0pi -e 's/ — \(nil, nil\)\./ — models.ErrNotFound./g' $NONTEST

# Импорт context в файлы адаптера.
for f in $NONTEST; do
  grep -q '"context"' "$f" || perl -0pi -e 's/import \(\n/import (\n\t"context"\n/' "$f"
done

# Тесты адаптера: ctx = t.Context().
T=$S/sqlstore_test.go
perl -0pi -e '
  s/\b(\w+)\.((?:Get|Save|List)[A-Z]\w*)\(\)/$1.$2(t.Context())/g;
  s/\b(\w+)\.((?:Get|Save|List)[A-Z]\w*)\((?!t\.Context)/$1.$2(t.Context(), /g;
  s/\bs\.searchIDs\(/s.searchIDs(t.Context(), /g;
' $T
```

- [ ] **Step 2: Сохранить и запустить скрипт правок**

Сохранить как `$TMPDIR/s6-edit.py` и выполнить `python3 $TMPDIR/s6-edit.py` (при несовпадении фрагмента он останавливается с именем файла — тогда файл изменился, разобраться, не подгонять скрипт молча):

```python
"""S6, шаг 2 задачи 2: правки, которые не сводятся к регулярным заменам.
Запуск из корня репозитория: python3 s6-edit.py"""


def edit(path, pairs):
    s = open(path, encoding="utf-8").read()
    for old, new in pairs:
        if old not in s:
            raise SystemExit(f"{path}: не найден фрагмент: {old[:60]!r}")
        s = s.replace(old, new)
    open(path, "w", encoding="utf-8").write(s)


edit("internal/store/deps.go", [
    ('import "github.com/amarin/genodex/internal/models"\n',
     'import (\n\t"context"\n\n\t"github.com/amarin/genodex/internal/models"\n)\n'),
    ('''//   - Get* возвращает (nil, nil), если сущности с таким id нет; ошибка — только
//     при сбое хранилища.
''', '''//   - Все методы принимают ctx первым аргументом: отмена или истечение срока
//     прерывают запрос и возвращают ошибку контекста (запись при этом не
//     применяется).
//   - Get* возвращает models.ErrNotFound (проверять через errors.Is), если
//     сущности с таким id нет; остальные ошибки — сбой хранилища.
'''),
])

edit("internal/store/sqlstore/sqlstore.go", [
    ('''// queryer — общий интерфейс чтения для *storage.DB и *sql.Tx.
type queryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// notFound сообщает, что строки нет. Контракт Get по всему порту: сущность
// не найдена — (nil, nil), ошибки хранилища возвращаются как есть.
func notFound''', '''// queryer — общий интерфейс чтения для runner.
type queryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}

// sqlExecutor — то, что умеют и соединение (*storage.DB), и транзакция (*sql.Tx).
type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// runner выполняет запросы с привязанным контекстом: помощники вызывают
// Exec/Query/QueryRow, а отмена ctx доходит до драйвера.
type runner struct {
	ctx context.Context
	x   sqlExecutor
}

func (r runner) Exec(query string, args ...any) (sql.Result, error) {
	return r.x.ExecContext(r.ctx, query, args...)
}

func (r runner) Query(query string, args ...any) (*sql.Rows, error) {
	return r.x.QueryContext(r.ctx, query, args...)
}

func (r runner) QueryRow(query string, args ...any) *sql.Row {
	return r.x.QueryRowContext(r.ctx, query, args...)
}

// run возвращает runner поверх соединения без транзакции.
func (s *Store) run(ctx context.Context) runner { return runner{ctx: ctx, x: s.db} }

// inTx выполняет fn в транзакции с привязанным контекстом; ошибка — откат.
func (s *Store) inTx(ctx context.Context, fn func(tx runner) error) error {
	return s.db.TxContext(ctx, func(tx *sql.Tx) error {
		return fn(runner{ctx: ctx, x: tx})
	})
}

// notFound сообщает, что строки нет: Get превращает это в models.ErrNotFound,
// остальные ошибки хранилища возвращаются как есть.
func notFound'''),
    ('func listEntities[T any](s *Store, table string, get func(models.ID) (*T, error)) ([]*T, error) {',
     'func listEntities[T any](ctx context.Context, s *Store, table string, get func(context.Context, models.ID) (*T, error)) ([]*T, error) {'),
    ('''		v, err := get(id)
		if err != nil {
			return nil, err
		}

		if v == nil {
			continue
		}

		out = append(out, v)''', '''		v, err := get(ctx, id)
		if errors.Is(err, models.ErrNotFound) {
			continue // строку удалили между чтением id и Get
		}

		if err != nil {
			return nil, err
		}

		out = append(out, v)'''),
])

edit("internal/store/sqlstore/dictionaries.go", [
    ('\t"database/sql"\n', ''),
    ('''// getDictionary читает скалярные колонки словарной записи и три списка;
// found=false — записи нет.''', '''// getDictionary читает скалярные колонки словарной записи и три списка;
// записи нет — models.ErrNotFound.'''),
    ('dest ...*string) (dictValue, bool, error) {', 'dest ...*string) (dictValue, error) {'),
    ('return v, false, nil\n', 'return v, models.ErrNotFound\n'),
    ('return v, false, err', 'return v, err'),
    ('return v, true, nil', 'return v, nil'),
    ('v, ok, err := s.getDictionary', 'v, err := s.getDictionary'),
    ('if err != nil || !ok {', 'if err != nil {'),
])

edit("internal/usecases/list_settlements/deps.go", [
    ('import "github.com/amarin/genodex/internal/models"',
     'import (\n\t"context"\n\n\t"github.com/amarin/genodex/internal/models"\n)'),
    ('ListAdministrativeDivisions() (', 'ListAdministrativeDivisions(ctx context.Context) ('),
])
edit("internal/usecases/list_settlements/scenario.go", [
    ('s.adminDivisions.ListAdministrativeDivisions()', 's.adminDivisions.ListAdministrativeDivisions(ctx)'),
])
edit("internal/usecases/list_settlements/scenario_test.go", [
    ('''type fakeRepo struct {
	list []*models.AdministrativeDivision
	err  error
}

func (f *fakeRepo) ListAdministrativeDivisions() (''', '''type ctxKey struct{}

type fakeRepo struct {
	list   []*models.AdministrativeDivision
	err    error
	gotCtx context.Context
}

func (f *fakeRepo) ListAdministrativeDivisions(ctx context.Context) ('''),
    ('''	return f.list, f.err
}''', '''	f.gotCtx = ctx

	return f.list, f.err
}'''),
])
```

- [ ] **Step 3: Перегенерировать моки и поправить тест «не найдено»**

```bash
(cd internal/store && go generate ./...)
(cd internal/usecases/list_settlements && go generate ./...)
```

В `internal/store/sqlstore/sqlstore_test.go` заменить тело `TestStoreGetMissing` (и его комментарий) на:

```go
// TestStoreGetMissing фиксирует контракт «не найдено» — models.ErrNotFound.
func TestStoreGetMissing(t *testing.T) {
	s := newStore(t)

	if _, err := s.GetPerson(t.Context(), "nope"); !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("GetPerson(nope) = %v, ожидалось models.ErrNotFound", err)
	}
}
```

и добавить `"errors"` в import теста. Затем `gofmt -w internal/store/sqlstore/sqlstore_test.go`.

- [ ] **Step 4: Рубеж**

Run: `gofmt -l . && go build ./... && go vet ./... && go test ./...`
Expected: gofmt пусто, сборка и `vet` без ошибок, все пакеты `ok`. Если `go build` жалуется на `web/embed.go` (`pattern all:dist`) — в рабочей копии не собран фронтенд; это не связано с задачей, соберите пакеты выборочно: `go build ./internal/... ./cmd/...`.

- [ ] **Step 5: Проверить, что `nil, nil` из порта исчез**

Run: `grep -n "return nil, nil" internal/store/sqlstore/*.go`
Expected: только `helpers.go` (внутренние `loadTextRefPtr`, `loadDate`, `loadAnchor`, `collectMainRefs`).

- [ ] **Step 6: Commit**

```bash
git add -A internal
git commit -m "feat(store): ctx во всех методах порта, Get* возвращает ErrNotFound"
```

---

### Task 3: Тесты контракта `ctx` и `ErrNotFound`

**Files:**
- Create: `internal/store/sqlstore/context_test.go`
- Modify: `internal/store/sqlstore/sqlstore_test.go` (удалить `TestStoreGetMissing` — его заменяет табличный тест), `internal/usecases/list_settlements/scenario_test.go`

**Interfaces:**
- Consumes: Task 2 (порт с `ctx`, `listEntities`, `newStore`), `fakeRepo`/`ctxKey` из Task 2.
- Produces: тесты `TestPortMethodsAreCovered`, `TestGetMissingReturnsErrNotFound`, `TestGetCanceledContextIsNotNotFound`, `TestListCanceledContext`, `TestSaveCanceledContextWritesNothing`, `TestListEntitiesSkipsRowDeletedMidList`, `TestScenarioPassesContextToRepo`.

- [ ] **Step 1: Написать тесты**

Создать `internal/store/sqlstore/context_test.go` и выполнить `gofmt -w` на нём:

```go
package sqlstore

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

// getFunc и listFunc — методы порта в виде выражений методов, приведённые к
// одному виду: так один тест обходит все 21 сущность.
type (
	getFunc  func(*Store, context.Context, models.ID) error
	listFunc func(*Store, context.Context) error
)

func probeGet[T any](get func(*Store, context.Context, models.ID) (*T, error)) getFunc {
	return func(s *Store, ctx context.Context, id models.ID) error {
		_, err := get(s, ctx, id)

		return err
	}
}

func probeList[T any](list func(*Store, context.Context) ([]*T, error)) listFunc {
	return func(s *Store, ctx context.Context) error {
		_, err := list(s, ctx)

		return err
	}
}

var getters = map[string]getFunc{
	"Person":                 probeGet((*Store).GetPerson),
	"Relation":               probeGet((*Store).GetRelation),
	"Residence":              probeGet((*Store).GetResidence),
	"Family":                 probeGet((*Store).GetFamily),
	"Surname":                probeGet((*Store).GetSurname),
	"GivenName":              probeGet((*Store).GetGivenName),
	"Patronymic":             probeGet((*Store).GetPatronymic),
	"Estate":                 probeGet((*Store).GetEstate),
	"Title":                  probeGet((*Store).GetTitle),
	"AdministrativeDivision": probeGet((*Store).GetAdministrativeDivision),
	"Church":                 probeGet((*Store).GetChurch),
	"Parish":                 probeGet((*Store).GetParish),
	"Event":                  probeGet((*Store).GetEvent),
	"Source":                 probeGet((*Store).GetSource),
	"Archive":                probeGet((*Store).GetArchive),
	"ArchiveNode":            probeGet((*Store).GetArchiveNode),
	"ArchiveDocument":        probeGet((*Store).GetArchiveDocument),
	"Attachment":             probeGet((*Store).GetAttachment),
	"Citation":               probeGet((*Store).GetCitation),
	"Note":                   probeGet((*Store).GetNote),
	"Repository":             probeGet((*Store).GetRepository),
}

var listers = map[string]listFunc{
	"People":                  probeList((*Store).ListPeople),
	"Relations":               probeList((*Store).ListRelations),
	"Residences":              probeList((*Store).ListResidences),
	"Families":                probeList((*Store).ListFamilies),
	"Surnames":                probeList((*Store).ListSurnames),
	"GivenNames":              probeList((*Store).ListGivenNames),
	"Patronymics":             probeList((*Store).ListPatronymics),
	"Estates":                 probeList((*Store).ListEstates),
	"Titles":                  probeList((*Store).ListTitles),
	"AdministrativeDivisions": probeList((*Store).ListAdministrativeDivisions),
	"Churches":                probeList((*Store).ListChurches),
	"Parishes":                probeList((*Store).ListParishes),
	"Events":                  probeList((*Store).ListEvents),
	"Sources":                 probeList((*Store).ListSources),
	"Archives":                probeList((*Store).ListArchives),
	"ArchiveNodes":            probeList((*Store).ListArchiveNodes),
	"ArchiveDocuments":        probeList((*Store).ListArchiveDocuments),
	"Attachments":             probeList((*Store).ListAttachments),
	"Citations":               probeList((*Store).ListCitations),
	"Notes":                   probeList((*Store).ListNotes),
	"Repositories":            probeList((*Store).ListRepositories),
}

// canceled возвращает уже отменённый контекст.
func canceled(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	return ctx
}

// TestPortMethodsAreCovered не даёт добавить метод порта, не внеся его в
// таблицы обхода: число Get*/List* у адаптера должно совпасть с таблицами.
func TestPortMethodsAreCovered(t *testing.T) {
	var gets, lists int

	typ := reflect.TypeOf(&Store{})
	for i := 0; i < typ.NumMethod(); i++ {
		switch name := typ.Method(i).Name; {
		case strings.HasPrefix(name, "Get"):
			gets++
		case strings.HasPrefix(name, "List"):
			lists++
		}
	}

	if gets != len(getters) || lists != len(listers) {
		t.Fatalf("методов Get*/List* = %d/%d, в таблицах %d/%d", gets, lists, len(getters), len(listers))
	}
}

// TestGetMissingReturnsErrNotFound: для отсутствующего id каждый Get отдаёт
// models.ErrNotFound, а не (nil, nil) и не ошибку хранилища.
func TestGetMissingReturnsErrNotFound(t *testing.T) {
	s := newStore(t)

	for name, get := range getters {
		t.Run(name, func(t *testing.T) {
			if err := get(s, t.Context(), "nope"); !errors.Is(err, models.ErrNotFound) {
				t.Fatalf("Get%s(nope) = %v, ожидалось models.ErrNotFound", name, err)
			}
		})
	}
}

// TestGetCanceledContextIsNotNotFound: отменённый контекст — это ошибка
// контекста, а не «не найдено».
func TestGetCanceledContextIsNotNotFound(t *testing.T) {
	s := newStore(t)

	for name, get := range getters {
		t.Run(name, func(t *testing.T) {
			err := get(s, canceled(t), "nope")
			if !errors.Is(err, context.Canceled) || errors.Is(err, models.ErrNotFound) {
				t.Fatalf("Get%s с отменённым ctx = %v, ожидалось context.Canceled", name, err)
			}
		})
	}
}

// TestListCanceledContext: List всех сущностей прерывается отменой контекста.
func TestListCanceledContext(t *testing.T) {
	s := newStore(t)

	for name, list := range listers {
		t.Run(name, func(t *testing.T) {
			if err := list(s, canceled(t)); !errors.Is(err, context.Canceled) {
				t.Fatalf("List%s с отменённым ctx = %v, ожидалось context.Canceled", name, err)
			}
		})
	}
}

// TestSaveCanceledContextWritesNothing: отменённое сохранение возвращает
// ошибку контекста с видом сущности и ничего не пишет.
func TestSaveCanceledContextWritesNothing(t *testing.T) {
	s := newStore(t)

	err := s.SaveSurname(canceled(t), &models.Surname{ID: "sur-1", Canonical: "Блохин"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SaveSurname с отменённым ctx = %v, ожидалось context.Canceled", err)
	}

	if !strings.Contains(err.Error(), "surname") {
		t.Errorf("ошибка %q не называет вид сущности", err)
	}

	if _, err := s.GetSurname(t.Context(), "sur-1"); !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("после отменённого Save Get = %v, ожидалось models.ErrNotFound", err)
	}
}

// TestListEntitiesSkipsRowDeletedMidList: строка, исчезнувшая между чтением
// id и Get, пропускается; любая другая ошибка Get прерывает список.
func TestListEntitiesSkipsRowDeletedMidList(t *testing.T) {
	s := newStore(t)

	for _, id := range []models.ID{"p-1", "p-2"} {
		if err := s.SavePerson(t.Context(), &models.Person{ID: id}); err != nil {
			t.Fatalf("save %s: %v", id, err)
		}
	}

	gone := func(ctx context.Context, id models.ID) (*models.Person, error) {
		if id == "p-1" {
			return nil, models.ErrNotFound
		}

		return s.GetPerson(ctx, id)
	}

	got, err := listEntities(t.Context(), s, "persons", gone)
	if err != nil || len(got) != 1 || got[0].ID != "p-2" {
		t.Fatalf("listEntities = %+v, %v; ожидалась одна персона p-2", got, err)
	}

	boom := errors.New("boom")
	failing := func(context.Context, models.ID) (*models.Person, error) { return nil, boom }

	if _, err := listEntities(t.Context(), s, "persons", failing); !errors.Is(err, boom) {
		t.Fatalf("listEntities с падающим Get = %v, ожидалась boom", err)
	}
}
```

В `sqlstore_test.go` удалить `TestStoreGetMissing` целиком (вместе с комментарием) и убрать из import добавленный в Task 2 `"errors"`, если он больше нигде не нужен.

В конец `internal/usecases/list_settlements/scenario_test.go` дописать:

```go

func TestScenarioPassesContextToRepo(t *testing.T) {
	repo := &fakeRepo{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "marker")

	if _, err := New(repo).ListSettlements(ctx); err != nil {
		t.Fatalf("ListSettlements: %v", err)
	}

	if repo.gotCtx == nil || repo.gotCtx.Value(ctxKey{}) != "marker" {
		t.Fatalf("репозиторий получил контекст %v, ожидался переданный сценарию", repo.gotCtx)
	}
}
```

- [ ] **Step 2: Прогнать**

Run: `gofmt -l . && go vet ./... && go test ./internal/store/... ./internal/usecases/...`
Expected: gofmt пусто, `vet` чисто, тесты `ok`.

- [ ] **Step 3: Мутационные проверки (вручную, каждую откатить сразу после прогона)**

Каждая мутация должна провалить указанный тест; после проверки вернуть файл (`git checkout -- <файл>`).

| Мутация | Ожидаемый провал |
|---|---|
| в `GetPerson` (`person.go`) `models.ErrNotFound` → `nil` | `TestGetMissingReturnsErrNotFound/Person` |
| в `listEntities` убрать ветку `errors.Is(err, models.ErrNotFound)` | `TestListEntitiesSkipsRowDeletedMidList` |
| в `runner.QueryRow` `r.ctx` → `context.Background()` | `TestGetCanceledContextIsNotNotFound` |
| в `runner.Query` `r.ctx` → `context.Background()` | `TestListCanceledContext` |
| в сценарии `ListAdministrativeDivisions(ctx)` → `(context.Background())` | `TestScenarioPassesContextToRepo` |

- [ ] **Step 4: Commit**

```bash
git add internal/store/sqlstore/context_test.go internal/store/sqlstore/sqlstore_test.go internal/usecases/list_settlements/scenario_test.go
git commit -m "test(store): контракт ctx и ErrNotFound для всех 21 сущностей"
```

---

### Task 4: Документация

**Files:**
- Modify: `docs/data-model/core-read-write.md` (§4), `docs/plans/2026-09-20-core-rw-roadmap.md` (S6)

- [ ] **Step 1: Спецификация §4**

В `docs/data-model/core-read-write.md` заменить пункт

```
- `Get*` возвращает `ErrNotFound` (вместо `(nil, nil)`); правило `nil,nil`
  снимается вместе с потребителями.
```

на

```
- `Get*` возвращает `ErrNotFound` (вместо `(nil, nil)`); правило `nil,nil`
  снято на этапе S6. `ctx` в адаптере привязывается к каждому запросу и
  транзакции (`sqlstore.runner`), отмена доходит до драйвера; отменённая запись
  ничего не пишет.
```

- [ ] **Step 2: Дорожная карта**

В `docs/plans/2026-09-20-core-rw-roadmap.md` в блоке `### S6.` заменить список файлов на
`` `models/errors.go` (`ErrNotFound`), `storage/db.go` (`*Context`-методы), `store/deps.go`, `sqlstore/*` (`runner`, `inTx`), сгенерированные моки, тесты (`sqlstore/context_test.go`), `usecases/list_settlements`; план — `2026-09-21-core-rw-s06-notfound-ctx.md`. ``

- [ ] **Step 3: Commit**

```bash
git add docs/data-model/core-read-write.md docs/plans/2026-09-20-core-rw-roadmap.md
git commit -m "docs: S6 — ErrNotFound и ctx в порту"
```
