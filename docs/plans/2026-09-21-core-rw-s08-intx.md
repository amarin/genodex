# S8: `InTx` — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Порт `store.Store` получает `InTx(ctx, func(Store) error) error`: все методы переданного `Store` работают в одной транзакции; ошибка или паника — откат целиком; вложенный `InTx` использует ту же транзакцию.

**Architecture:** Внутри `sqlstore` `Store` получает поле `exec sqlExecutor` (соединение по умолчанию) и признак `scoped`. `run(ctx)` строит `runner` поверх `exec`, а `inTx` внутри `scoped`-`Store` не открывает новую транзакцию, а выполняет функцию в текущей. `InTx` открывает `storage.DB.TxContext` и передаёт функции копию `Store` с `exec = *sql.Tx` и `scoped = true`; на `scoped`-`Store` вложенный `InTx` просто вызывает функцию. Кэш графа внешних ключей (S7) выносится в общий для копий `schemaCache`; внутри `InTx` его загрузка идёт в той же транзакции, поэтому единственное соединение не блокируется (это снимает предпосылку S7). `TxContext` откатывает транзакцию и при панике в функции — иначе оставшаяся открытой транзакция заблокировала бы единственное соединение навсегда.

**Tech Stack:** Go 1.26.4, `database/sql`, `modernc.org/sqlite`, `go.uber.org/mock`.

**Spec:** `docs/data-model/core-read-write.md` §4; `docs/plans/2026-09-20-core-rw-roadmap.md` (этап S8).

## Global Constraints

- Порт: `InTx(ctx context.Context, fn func(Store) error) error`; ошибка `fn` возвращается как есть (после отката), `nil` — фиксация; отменённый `ctx` — ошибка контекста, `fn` не вызывается.
- Внутри `fn` все методы переданного `Store` (включая `Delete*` и `List*`) работают в одной транзакции и видят её незафиксированные записи.
- Вложенный `InTx` на переданном `Store` использует ту же транзакцию, без savepoint (ошибка вложенного вызова, проглоченная снаружи, не откатывается отдельно — ограничение зафиксировано тестом и документацией).
- Паника в `fn` откатывает транзакцию и поднимается дальше; хранилище остаётся пригодным к работе.
- Соединение одно (`SetMaxOpenConns(1)`): внутри `fn` нельзя пользоваться внешним `Store` (он ждёт соединение до отмены `ctx`); переданный `Store` нельзя использовать после возврата из `InTx` (методы вернут `sql.ErrTxDone`) — оба правила в комментарии порта.
- Схема БД, `Validate`, контракты MCP/HTTP не меняются; поведение методов вне `InTx` прежнее.
- Комментарии и тексты ошибок — на русском; код gofmt-clean.
- Проверка рубежа: `gofmt -l .` пусто, `go build ./...`, `go vet ./...`, `go test ./...`.

---

### Task 1: `TxContext` откатывает при панике

**Files:**
- Modify: `internal/storage/db.go`, `internal/storage/db_test.go`

**Interfaces:**
- Consumes: `(*DB).TxContext`, `ctxCause`, `openTestDB` (S6); в `db_test.go` уже подключены `context`, `errors`, `time`.
- Produces: `TxContext` с откатом при панике (панику повторно поднимает).

- [ ] **Step 1: Тест (падает — паника оставляет соединение занятым)**

В конец `internal/storage/db_test.go` дописать:

```go
// TestTxContextPanicRollsBack: паника в fn откатывает транзакцию, повторно
// поднимается и не оставляет единственное соединение занятым.
func TestTxContextPanicRollsBack(t *testing.T) {
	db := openTestDB(t)

	func() {
		defer func() {
			if p := recover(); p != "boom" {
				t.Fatalf("паника = %v, want boom", p)
			}
		}()

		_ = db.TxContext(t.Context(), func(tx *sql.Tx) error {
			if _, err := tx.Exec("INSERT INTO persons(id) VALUES ('p-1')"); err != nil {
				return err
			}

			panic("boom")
		})
	}()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	var n int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM persons").Scan(&n); err != nil {
		t.Fatalf("соединение занято после паники: %v", err)
	}

	if n != 0 {
		t.Fatalf("Count persons=%d после паники, want 0", n)
	}
}
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test ./internal/storage/ -run TxContextPanic -count=1`
Expected: FAIL — «соединение занято после паники» (через 5 секунд).

- [ ] **Step 3: Реализация**

В `internal/storage/db.go` заменить функцию `TxContext` (вместе с её комментарием; `ctxCause` после неё остаётся) на:

```go
// TxContext выполняет fn в транзакции, привязанной к ctx; при ошибке — откат.
func (d *DB) TxContext(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := d.d.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	// одно соединение: незакрытая транзакция заблокировала бы все следующие
	// запросы, поэтому откат — на любом выходе из fn (ошибка, паника, Goexit);
	// после Commit он возвращает sql.ErrTxDone и ничего не делает
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return ctxCause(ctx, err)
	}
	return ctxCause(ctx, tx.Commit())
}
```

- [ ] **Step 4: Тесты проходят**

Run: `gofmt -l internal && go test ./internal/storage/ -count=1`
Expected: gofmt пусто, `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/storage/db.go internal/storage/db_test.go
git commit -m "fix(storage): TxContext откатывает транзакцию при панике"
```

---

### Task 2: `InTx` в порту и адаптере

**Files:**
- Create: `internal/store/sqlstore/tx.go`
- Modify: `internal/store/deps.go`, `internal/store/deps_test.go` (генерируется), `internal/store/sqlstore/sqlstore.go`, `internal/store/sqlstore/fkgraph.go`

**Interfaces:**
- Consumes: Task 1; `runner`, `sqlExecutor`, `(*Store).run/inTx`, `(*Store).graph`, `schemaGraph` (S6–S7).
- Produces: `store.Store.InTx`; `(*Store).InTx`; поля `Store.exec`, `Store.scoped`, `Store.cache`; тип `schemaCache`.

Код проверен на чистой копии `main` (после Task 1): сборка, `vet` и все существующие тесты зелёные.

- [ ] **Step 1: Создать `tx.go`**

Создать `internal/store/sqlstore/tx.go`:

```go
package sqlstore

import (
	"context"
	"database/sql"

	"github.com/amarin/genodex/internal/store"
)

// InTx выполняет fn в одной транзакции; см. store.Store.InTx. Переданный fn
// Store — копия адаптера, привязанная к транзакции (exec = *sql.Tx): его
// методы, включая вложенный InTx, работают в ней.
func (s *Store) InTx(ctx context.Context, fn func(store.Store) error) error {
	if s.scoped {
		return fn(s)
	}

	return s.db.TxContext(ctx, func(tx *sql.Tx) error {
		return fn(&Store{st: s.st, db: s.db, exec: tx, scoped: true, cache: s.cache})
	})
}
```

- [ ] **Step 2: Правки порта и адаптера**

Сохранить как `$TMPDIR/s8-edit.py` (вне репозитория) и выполнить из корня: `python3 $TMPDIR/s8-edit.py` (при несовпадении скрипт останавливается с сообщением — не подгонять молча, разобраться):

```python
"""S8, шаг задачи 2: порт и адаптер. Запуск из корня репозитория."""


def edit(p, pairs):
    s = open(p, encoding="utf-8").read()
    for old, new in pairs:
        if old not in s:
            raise SystemExit(f"{p}: не найден фрагмент: {old[:60]!r}")
        s = s.replace(old, new, 1)
    open(p, "w", encoding="utf-8").write(s)


edit("internal/store/sqlstore/sqlstore.go", [
    ("""type Store struct {
	st *storage.Storage
	db *storage.DB

	graphMu sync.Mutex
	schema  *schemaGraph // граф внешних ключей, строится при первом удалении
}""", """//
// Store внутри InTx — копия с exec = транзакция и scoped = true: все методы
// порта работают в ней, вложенный InTx не открывает новую транзакцию.
type Store struct {
	st *storage.Storage
	db *storage.DB

	exec   sqlExecutor  // соединение или (внутри InTx) транзакция
	scoped bool         // true — Store привязан к транзакции InTx
	cache  *schemaCache // общий для копий Store кэш графа внешних ключей
}

// schemaCache — граф внешних ключей схемы, строится при первом удалении.
type schemaCache struct {
	mu sync.Mutex
	g  *schemaGraph
}"""),
    ("""	return &Store{st: st, db: st.DB()}""", """	db := st.DB()

	return &Store{st: st, db: db, exec: db, cache: &schemaCache{}}"""),
    ("""// run возвращает runner поверх соединения без транзакции.
func (s *Store) run(ctx context.Context) runner { return runner{ctx: ctx, x: s.db} }

// inTx выполняет fn в транзакции с привязанным контекстом; ошибка — откат.
func (s *Store) inTx(ctx context.Context, fn func(tx runner) error) error {
	return s.db.TxContext(ctx, func(tx *sql.Tx) error {""", """// run возвращает runner поверх соединения или, внутри InTx, транзакции.
func (s *Store) run(ctx context.Context) runner { return runner{ctx: ctx, x: s.exec} }

// inTx выполняет fn в транзакции с привязанным контекстом; ошибка — откат.
// Внутри InTx транзакция уже открыта: fn выполняется в ней без новой.
func (s *Store) inTx(ctx context.Context, fn func(tx runner) error) error {
	if s.scoped {
		return fn(s.run(ctx))
	}

	return s.db.TxContext(ctx, func(tx *sql.Tx) error {"""),
])

edit("internal/store/sqlstore/fkgraph.go", [
    ("""	s.graphMu.Lock()
	defer s.graphMu.Unlock()

	if s.schema != nil {
		return s.schema, nil
	}
""", """	s.cache.mu.Lock()
	defer s.cache.mu.Unlock()

	if s.cache.g != nil {
		return s.cache.g, nil
	}
"""),
    ("	s.schema = g\n", "	s.cache.g = g\n"),
    ("""// graph возвращает граф схемы; результат кэшируется, ошибка — нет (отменённый
// контекст не должен «отравить» хранилище).""", """// graph возвращает граф схемы; результат кэшируется, ошибка — нет (отменённый
// контекст не должен «отравить» хранилище). Внутри InTx запрос идёт в той же
// транзакции, поэтому не блокируется на единственном соединении."""),
])

edit("internal/store/deps.go", [
    ("""type Store interface {
""", """type Store interface {
	// InTx выполняет fn в одной транзакции: все методы переданного Store
	// работают в ней; ошибка fn (или паника) откатывает всё, nil — фиксирует.
	// Вложенный InTx на переданном Store использует ту же транзакцию (без
	// savepoint: ошибку вложенного вызова, проглоченную снаружи, откатить
	// нельзя). Внутри fn пользоваться нужно только переданным Store, не
	// внешним: соединение одно, и внешний Store ждёт его до отмены ctx.
	// Переданный Store нельзя сохранять и использовать после возврата из InTx
	// (методы вернут sql.ErrTxDone).
	InTx(ctx context.Context, fn func(Store) error) error

"""),
    ("""//   - Get* возвращает""", """//   - InTx группирует несколько вызовов порта в одну атомарную операцию (см.
//     метод).
//   - Get* возвращает"""),
])
```

Затем перегенерировать мок порта: `(cd internal/store && go generate ./...)`.

- [ ] **Step 3: Рубеж**

Run: `gofmt -l . && go build ./... && go vet ./... && go test ./... -count=1`
Expected: gofmt пусто, сборка и `vet` без ошибок, все пакеты `ok`. Если `go build` жалуется на `web/embed.go` (`pattern all:dist`) — соберите `go build ./internal/... ./cmd/...`.

- [ ] **Step 4: Commit**

```bash
git add -A internal
git commit -m "feat(store): InTx — атомарная запись нескольких сущностей"
```

---

### Task 3: Тесты `InTx`

**Files:**
- Create: `internal/store/sqlstore/tx_test.go`

**Interfaces:**
- Consumes: Task 2; `newStore`, `snapshot`, `link`, `mustDo`, `canceled` (`sqlstore_test.go`, `context_test.go`, `delete_test.go`).
- Produces: хелперы `withTimeout`, `seedSource`, `errBoom`; тесты `TestInTxCommitsCitationAndPersonAtomically`, `TestInTxRollsBackEverythingOnError`, `TestInTxSeesOwnWrites`, `TestInTxDeleteInUseRollsBackNeighbours`, `TestInTxNestedUsesSameTransaction`, `TestInTxNestedErrorPropagatesToOuter`, `TestInTxNestedHasNoSavepoint`, `TestInTxPanicRollsBackAndKeepsStoreUsable`, `TestInTxCanceledContext`, `TestInTxEscapedStoreFailsAfterCommit`.

- [ ] **Step 1: Создать тесты**

Создать `internal/store/sqlstore/tx_test.go`, выполнить `gofmt -w` на нём:

```go
package sqlstore

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// withTimeout ограничивает тест: зависание на единственном соединении должно
// стать ошибкой контекста, а не бесконечным ожиданием.
func withTimeout(t *testing.T) context.Context {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	return ctx
}

// seedSource сохраняет источник — цель для цитаты.
func seedSource(t *testing.T, s *Store) {
	t.Helper()

	mustDo(t, "source", s.SaveSource(t.Context(), &models.Source{
		ID: "src-1", Kind: models.SourceKindMemory, Title: "Рассказ",
	}))
}

var errBoom = errors.New("boom")

// TestInTxCommitsCitationAndPersonAtomically: цитата и ссылающаяся на неё
// персона пишутся одной транзакцией и после успеха видны обе.
func TestInTxCommitsCitationAndPersonAtomically(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)
	seedSource(t, s)

	err := s.InTx(ctx, func(tx store.Store) error {
		if err := tx.SaveCitation(ctx, &models.Citation{ID: "cit-1", SourceID: "src-1"}); err != nil {
			return err
		}

		return tx.SavePerson(ctx, &models.Person{ID: "p-1", Sources: link(models.TypePerson, "p-1")})
	})
	mustDo(t, "InTx", err)

	if _, err := s.GetCitation(ctx, "cit-1"); err != nil {
		t.Fatalf("цитата не сохранена: %v", err)
	}

	got, err := s.GetPerson(ctx, "p-1")
	mustDo(t, "get person", err)

	if len(got.Sources) != 1 {
		t.Fatalf("у персоны %d доказательств, ожидалось 1", len(got.Sources))
	}
}

// TestInTxRollsBackEverythingOnError: ошибка после записи обеих сущностей
// откатывает обе и возвращается как есть.
func TestInTxRollsBackEverythingOnError(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)
	seedSource(t, s)

	before := snapshot(t, s)

	err := s.InTx(ctx, func(tx store.Store) error {
		mustDo(t, "citation", tx.SaveCitation(ctx, &models.Citation{ID: "cit-1", SourceID: "src-1"}))
		mustDo(t, "person", tx.SavePerson(ctx, &models.Person{ID: "p-1", Sources: link(models.TypePerson, "p-1")}))

		return errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("InTx = %v, ожидалась errBoom", err)
	}

	for _, get := range []func() error{
		func() error { _, err := s.GetCitation(ctx, "cit-1"); return err },
		func() error { _, err := s.GetPerson(ctx, "p-1"); return err },
	} {
		if err := get(); !errors.Is(err, models.ErrNotFound) {
			t.Fatalf("после отката Get = %v, ожидалось ErrNotFound", err)
		}
	}

	if got := snapshot(t, s); !reflect.DeepEqual(got, before) {
		t.Fatalf("откат оставил следы:\n got  %v\n want %v", got, before)
	}
}

// TestInTxSeesOwnWrites: внутри транзакции читаются её же незафиксированные
// записи, включая удаление (граф схемы на холодном Store грузится в той же
// транзакции, а не блокируется на соединении).
func TestInTxSeesOwnWrites(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)

	mustDo(t, "seed", s.SavePerson(ctx, &models.Person{ID: "p-old"}))

	err := s.InTx(ctx, func(tx store.Store) error {
		mustDo(t, "save", tx.SavePerson(ctx, &models.Person{ID: "p-1"}))

		if _, err := tx.GetPerson(ctx, "p-1"); err != nil {
			t.Errorf("запись не видна внутри транзакции: %v", err)
		}

		if list, err := tx.ListPeople(ctx); err != nil || len(list) != 2 {
			t.Errorf("ListPeople внутри транзакции: len=%d err=%v, ожидалось 2", len(list), err)
		}

		mustDo(t, "delete", tx.DeletePerson(ctx, "p-old"))

		if _, err := tx.GetPerson(ctx, "p-old"); !errors.Is(err, models.ErrNotFound) {
			t.Errorf("удалённая внутри транзакции персона: %v, ожидалось ErrNotFound", err)
		}

		return nil
	})
	mustDo(t, "InTx", err)

	if _, err := s.GetPerson(ctx, "p-old"); !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("удаление не зафиксировано: %v", err)
	}
}

// TestInTxDeleteInUseRollsBackNeighbours: *InUseError из Delete* внутри
// транзакции доходит до вызывающего, соседние записи откатываются.
func TestInTxDeleteInUseRollsBackNeighbours(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)

	mustDo(t, "p-1", s.SavePerson(ctx, &models.Person{ID: "p-1"}))
	mustDo(t, "p-2", s.SavePerson(ctx, &models.Person{ID: "p-2"}))
	mustDo(t, "relation", s.SaveRelation(ctx, &models.Relation{
		ID: "rel-1", Kind: models.RelationKindMarriage, PersonA: "p-1", PersonB: "p-2",
	}))

	err := s.InTx(ctx, func(tx store.Store) error {
		mustDo(t, "neighbour", tx.SavePerson(ctx, &models.Person{ID: "p-3"}))

		return tx.DeletePerson(ctx, "p-1")
	})

	var inUse *models.InUseError
	if !errors.As(err, &inUse) {
		t.Fatalf("InTx = %v, ожидался *models.InUseError", err)
	}

	if _, err := s.GetPerson(ctx, "p-3"); !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("соседняя запись не откатилась: %v", err)
	}
}

// TestInTxNestedUsesSameTransaction: вложенный InTx работает в той же
// транзакции — видит незафиксированные записи внешнего, а откат внешнего
// откатывает и записи вложенного.
func TestInTxNestedUsesSameTransaction(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)

	err := s.InTx(ctx, func(outer store.Store) error {
		mustDo(t, "outer save", outer.SavePerson(ctx, &models.Person{ID: "p-1"}))

		mustDo(t, "inner", outer.InTx(ctx, func(inner store.Store) error {
			if _, err := inner.GetPerson(ctx, "p-1"); err != nil {
				t.Errorf("вложенный вызов не видит запись внешнего: %v", err)
			}

			return inner.SavePerson(ctx, &models.Person{ID: "p-2"})
		}))

		return errBoom
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("InTx = %v, ожидалась errBoom", err)
	}

	for _, id := range []models.ID{"p-1", "p-2"} {
		if _, err := s.GetPerson(ctx, id); !errors.Is(err, models.ErrNotFound) {
			t.Fatalf("после отката внешнего %s: %v, ожидалось ErrNotFound", id, err)
		}
	}
}

// TestInTxNestedErrorPropagatesToOuter: ошибка вложенного вызова возвращается
// внешней функции.
func TestInTxNestedErrorPropagatesToOuter(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)

	err := s.InTx(ctx, func(outer store.Store) error {
		return outer.InTx(ctx, func(store.Store) error { return errBoom })
	})
	if !errors.Is(err, errBoom) {
		t.Fatalf("InTx = %v, ожидалась errBoom", err)
	}
}

// TestInTxNestedHasNoSavepoint фиксирует документированное ограничение:
// вложенный вызов не откатывается отдельно, поэтому ошибка, проглоченная
// внешней функцией, оставляет записи вложенного в общей транзакции.
func TestInTxNestedHasNoSavepoint(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)

	err := s.InTx(ctx, func(outer store.Store) error {
		_ = outer.InTx(ctx, func(inner store.Store) error {
			mustDo(t, "inner save", inner.SavePerson(ctx, &models.Person{ID: "p-2"}))

			return errBoom
		})

		return nil
	})
	mustDo(t, "InTx", err)

	if _, err := s.GetPerson(ctx, "p-2"); err != nil {
		t.Fatalf("запись вложенного вызова пропала: %v", err)
	}
}

// TestInTxPanicRollsBackAndKeepsStoreUsable: паника в fn откатывает
// транзакцию и не оставляет соединение занятым.
func TestInTxPanicRollsBackAndKeepsStoreUsable(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)

	func() {
		defer func() {
			if p := recover(); p != "boom" {
				t.Fatalf("паника = %v, ожидалась boom", p)
			}
		}()

		_ = s.InTx(ctx, func(tx store.Store) error {
			mustDo(t, "save", tx.SavePerson(ctx, &models.Person{ID: "p-1"}))
			panic("boom")
		})
	}()

	if _, err := s.GetPerson(ctx, "p-1"); !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("после паники Get = %v, ожидалось ErrNotFound (транзакция откатана, соединение свободно)", err)
	}

	mustDo(t, "save after panic", s.SavePerson(ctx, &models.Person{ID: "p-2"}))
}

// TestInTxCanceledContext: отменённый контекст — ошибка контекста, fn не
// вызывается.
func TestInTxCanceledContext(t *testing.T) {
	s := newStore(t)

	err := s.InTx(canceled(t), func(store.Store) error {
		t.Error("fn не должна вызываться при отменённом контексте")

		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("InTx = %v, ожидалось context.Canceled", err)
	}
}

// TestInTxEscapedStoreFailsAfterCommit: Store, сохранённый за пределами
// InTx, после фиксации возвращает ошибку завершённой транзакции, а не виснет.
func TestInTxEscapedStoreFailsAfterCommit(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)

	var leaked store.Store

	mustDo(t, "InTx", s.InTx(ctx, func(tx store.Store) error {
		leaked = tx

		return nil
	}))

	if _, err := leaked.GetPerson(ctx, "p-1"); !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("Get на просочившемся Store = %v, ожидалось sql.ErrTxDone", err)
	}
}
```

- [ ] **Step 2: Прогнать**

Run: `gofmt -l . && go vet ./... && go test ./internal/store/... -count=1`
Expected: gofmt пусто, `vet` чисто, тесты `ok`.

- [ ] **Step 3: Мутационные проверки (вручную, каждую откатить сразу: `git checkout -- <файл>`)**

Провал тестов на зависании занимает по 10 секунд на тест (таймаут `withTimeout`) — запускайте с `-timeout 120s -run "InTx|TxContextPanic"`.

| Мутация | Ожидаемый провал |
|---|---|
| в `inTx` (`sqlstore.go`) удалить ветку `if s.scoped { return fn(s.run(ctx)) }` | `TestInTxCommitsCitationAndPersonAtomically`, `TestInTxRollsBackEverythingOnError`, `TestInTxSeesOwnWrites` (таймаут 10 с) |
| в `InTx` (`tx.go`) удалить ветку `if s.scoped { return fn(s) }` | `TestInTxNestedUsesSameTransaction`, `TestInTxNestedErrorPropagatesToOuter`, `TestInTxNestedHasNoSavepoint` |
| в `TxContext` (`db.go`) удалить `defer func() { _ = tx.Rollback() }()` | `TestTxContextPanicRollsBack`, `TestInTxPanicRollsBackAndKeepsStoreUsable` |
| в `run` (`sqlstore.go`) заменить `x: s.exec` на `x: s.db` | `TestInTxCommitsCitationAndPersonAtomically` и др. (таймаут 10 с) |

- [ ] **Step 4: Commit**

```bash
git add internal/store/sqlstore/tx_test.go
git commit -m "test(store): InTx — атомарность, вложенность, паника, отмена"
```

---

### Task 4: Документация

**Files:**
- Modify: `docs/data-model/core-read-write.md` (§4), `docs/plans/2026-09-20-core-rw-roadmap.md` (S8)

- [ ] **Step 1: Спецификация §4**

В `docs/data-model/core-read-write.md` заменить пункт

```
- `InTx(ctx, func(Store) error) error` — все методы порта внутри функции работают
  в одной транзакции; ошибка — откат. Методы порта получили `ctx` (первым
  аргументом) на этапе S6.
```

на

```
- `InTx(ctx, func(Store) error) error` — все методы порта внутри функции работают
  в одной транзакции и видят её незафиксированные записи; ошибка или паника —
  откат. Методы порта получили `ctx` (первым аргументом) на этапе S6. Вложенный
  `InTx` использует ту же транзакцию (без savepoint: проглоченную снаружи ошибку
  вложенного вызова откатить нельзя). Соединение одно: внутри функции
  пользоваться нужно только переданным `Store` (внешний ждёт соединение до
  отмены `ctx`), а после возврата из `InTx` переданный `Store` непригоден
  (`sql.ErrTxDone`). Реализовано в S8: `Store` внутри `InTx` — копия адаптера с
  `exec = *sql.Tx`.
```

- [ ] **Step 2: Дорожная карта**

В `docs/plans/2026-09-20-core-rw-roadmap.md` в блоке `### S8.` заменить строки файлов

```
- **Файлы:** `store/deps.go`, `sqlstore/sqlstore.go`, `sqlstore/tx.go`,
  рефакторинг методов на общий исполнитель, тесты.
```

на

```
- **Файлы:** `storage/db.go` (`TxContext` откатывает при панике),
  `store/deps.go`, `sqlstore/sqlstore.go` (`exec`, `scoped`, кэш графа схемы),
  `sqlstore/tx.go`, тесты (`sqlstore/tx_test.go`); общий исполнитель (`runner`)
  появился в S6; план — `2026-09-21-core-rw-s08-intx.md`.
```

а абзац

```
- **Предпосылка из S7:** `deleteEntity` лениво строит граф внешних ключей
  (`(*Store).graph(ctx)`) отдельным запросом до своей транзакции. Внутри
  `InTx` (одно соединение, открытая внешняя транзакция) этот запрос повиснет
  до отмены `ctx` — граф нужно загружать при `Open`/`New` (или передавать
  `runner`), иначе `Delete*` внутри `InTx` зависнет.
```

на

```
- **Предпосылка из S7 (закрыта):** граф внешних ключей внутри `InTx` строится в
  той же транзакции (`Store.exec = *sql.Tx`), поэтому `Delete*` внутри `InTx` не
  зависает; кэш графа общий для копий `Store` (`schemaCache`).
```

- [ ] **Step 3: Commit**

```bash
git add docs/data-model/core-read-write.md docs/plans/2026-09-20-core-rw-roadmap.md
git commit -m "docs: S8 — InTx"
```

---

## Правки по итогам ревью (внесены после выполнения задач)

Код в репозитории — источник истины; плановые блоки выше описывают первую версию. Отличия:

- `TxContext` (`storage/db.go`): вместо `recover` — `defer func() { _ = tx.Rollback() }()` (откат на любом выходе, включая `runtime.Goexit`; после `Commit` — `ErrTxDone`, безвредно); план Task 1 и таблица мутаций уже приведены к этому виду.
- `schemaCache` (`sqlstore.go`/`fkgraph.go`): вместо `sync.Mutex` — `atomic.Pointer[schemaGraph]` с `CompareAndSwap`; замок на время загрузки графа давал взаимоблокировку с транзакцией, держащей единственное соединение (Delete вне транзакции: замок → ждёт соединение; Delete внутри `InTx`: соединение → ждёт замок).
- `tx.go`: вложенный `InTx` на `scoped`-`Store` проверяет `ctx.Err()` до вызова `fn`; комментарий порта дополнен (ошибка `fn` возвращается как есть, отменённый `ctx` — ошибка контекста без вызова `fn`).
- `tx_test.go`: добавлены `TestInTxColdGraphDoesNotDeadlockWithOutsideDelete` и `TestInTxNestedCanceledContext`.
