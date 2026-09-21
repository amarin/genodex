# S9: `Page` и `Access` — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Все `List*` порта принимают режим доступа и окно: `List<E>(ctx, access models.Access, page models.Page)`; окна не пересекаются и покрывают набор, `AccessPublic` скрывает сущности с `Private = true` на уровне SQL.

**Architecture:** `models/query.go` (листовой пакет) вводит `Access` (`AccessFull` = 0, `AccessPublic`), `Page{Limit, Offset}` с `Normalized()` (0 → 50, не больше 500). В `sqlstore` общий `listEntities` получает `access` и `page`: сначала `SELECT id … [WHERE private = 0] ORDER BY rowid LIMIT ? OFFSET ?`, затем `Get` по каждому id. Таблицы с колонкой `private` определяются по схеме (`pragma_table_info`, вместе с графом внешних ключей S7, поле `schemaGraph.private`), а не списком. Сценарий `list_settlements` обходит окна репозитория до пустого — его поведение «все населённые пункты» не меняется. Порядок списков остаётся порядком сохранения (rowid), поэтому окна стабильны; пакетная загрузка (S10) и поиск (S11) — отдельные этапы.

**Tech Stack:** Go 1.26.4, `database/sql`, `modernc.org/sqlite`, `go.uber.org/mock`.

**Spec:** `docs/data-model/core-read-write.md` §4; `docs/plans/2026-09-20-core-rw-roadmap.md` (этап S9).

## Global Constraints

- Порт: `List<E>(ctx context.Context, access models.Access, page models.Page) ([]*models.<E>, error)` для всех 21 сущностей; порядок аргументов именно такой.
- `Page.Limit`: ≤ 0 → `models.DefaultPageLimit` (50), > `models.MaxPageLimit` (500) → 500; отрицательный `Offset` → 0 (мягкая нормализация; строгая проверка входа — дело обработчиков, S13).
- `AccessFull` (нулевое значение) — без фильтра. Любое другое значение трактуется как публичный доступ (безопасный отказ). На таблицах без колонки `private` (словари, церкви, приходы, деления) режим не влияет.
- Фильтр приватного и окно применяются в SQL; окно считается после фильтра. Порядок — по `rowid` (порядок сохранения); повторное сохранение (upsert) место в списке не меняет.
- Таблицы с колонкой `private` определяются по схеме, а не списком; их набор зафиксирован тестом.
- Сценарий `list_settlements` возвращает по-прежнему все населённые пункты (обходит окна до пустого, `AccessFull`), пустой результат — непустой-nil срез.
- Схема БД, `Validate`, контракты MCP/HTTP не меняются.
- Комментарии и тексты ошибок — на русском; код gofmt-clean.
- Проверка рубежа: `gofmt -l .` пусто, `go build ./...`, `go vet ./...`, `go test ./...`.

---

### Task 1: `Access`, `Page` в `models`

**Files:**
- Create: `internal/models/query.go`, `internal/models/query_test.go`

**Interfaces:**
- Consumes: —
- Produces: `type Access int`, `AccessFull`, `AccessPublic`; `DefaultPageLimit = 50`, `MaxPageLimit = 500`; `type Page struct{ Limit, Offset int }`; `(Page).Normalized() Page`.

- [ ] **Step 1: Тест**

Создать `internal/models/query_test.go`:

```go
package models

import "testing"

func TestPageNormalized(t *testing.T) {
	cases := []struct {
		name string
		in   Page
		want Page
	}{
		{"нулевое окно — размер по умолчанию", Page{}, Page{Limit: DefaultPageLimit}},
		{"отрицательный лимит — по умолчанию", Page{Limit: -3, Offset: 7}, Page{Limit: DefaultPageLimit, Offset: 7}},
		{"обычное окно не меняется", Page{Limit: 10, Offset: 20}, Page{Limit: 10, Offset: 20}},
		{"верхняя граница включена", Page{Limit: MaxPageLimit}, Page{Limit: MaxPageLimit}},
		{"лимит выше предела сужается", Page{Limit: MaxPageLimit + 1}, Page{Limit: MaxPageLimit}},
		{"отрицательный сдвиг — ноль", Page{Limit: 5, Offset: -1}, Page{Limit: 5}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.in.Normalized(); got != c.want {
				t.Fatalf("%+v.Normalized() = %+v, want %+v", c.in, got, c.want)
			}
		})
	}
}

func TestAccessZeroValueIsFull(t *testing.T) {
	var a Access
	if a != AccessFull {
		t.Fatalf("нулевой Access = %d, ожидался AccessFull", a)
	}

	if AccessPublic == AccessFull {
		t.Fatal("AccessPublic совпадает с AccessFull")
	}
}
```

- [ ] **Step 2: Убедиться, что не компилируется**

Run: `go test ./internal/models/`
Expected: FAIL — `undefined: Page`.

- [ ] **Step 3: Реализация**

Создать `internal/models/query.go`:

```go
package models

// Access — режим доступа читателя к данным. Нулевое значение — полный доступ
// (владелец); любое другое значение трактуется как публичный доступ, то есть
// фильтр приватного включается, а не отключается (безопасный отказ).
type Access int

const (
	// AccessFull — читатель видит всё, включая сущности с Private = true.
	AccessFull Access = iota
	// AccessPublic — сущности с Private = true скрыты. На сущности без флага
	// приватности (словари, церкви, приходы, деления) режим не влияет.
	AccessPublic
)

// Пределы страницы списка.
const (
	DefaultPageLimit = 50
	MaxPageLimit     = 500
)

// Page — окно списка: не больше Limit записей, начиная с Offset-й. Порядок
// записей стабилен и совпадает с порядком сохранения сущностей.
type Page struct {
	Limit, Offset int
}

// Normalized приводит окно к допустимому виду: Limit меньше либо равный нулю —
// DefaultPageLimit, больше MaxPageLimit — MaxPageLimit; отрицательный Offset — 0.
// Строгая проверка входных значений — дело обработчиков.
func (p Page) Normalized() Page {
	switch {
	case p.Limit <= 0:
		p.Limit = DefaultPageLimit
	case p.Limit > MaxPageLimit:
		p.Limit = MaxPageLimit
	}

	if p.Offset < 0 {
		p.Offset = 0
	}

	return p
}
```

- [ ] **Step 4: Тесты проходят**

Run: `gofmt -l internal && go test ./internal/models/`
Expected: gofmt пусто, `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/models/query.go internal/models/query_test.go
git commit -m "feat(models): Access и Page для запросов списков"
```

---

### Task 2: Порт, адаптер и сценарий

**Files:**
- Modify: `internal/store/deps.go`, `internal/store/deps_test.go` (генерируется), `internal/store/sqlstore/*.go` (включая тесты), `internal/usecases/list_settlements/{deps,deps_test,scenario,scenario_test}.go`

**Interfaces:**
- Consumes: Task 1; `listEntities`, `listIDs`, `schemaGraph`, `(*Store).graph` (S6–S8).
- Produces: порт с 21 методом `List*(ctx, access, page)`; `listIDs(q, table, publicOnly, page)`; `listEntities(ctx, s, table, access, page, get)`; `schemaGraph.private`, `(*schemaGraph).hasPrivate(table)`; тестовый тип `listerSpec{table, list}` и `probeList(table, list)` (`context_test.go`); репозиторий сценария `ListAdministrativeDivisions(ctx, access, page)`.

Скрипты ниже проверены на чистой копии `main` (после Task 1): после них сборка, `vet` и все существующие тесты зелёные. Сохранить в каталог вне репозитория (`$TMPDIR`), запускать из корня репозитория.

- [ ] **Step 1: Замены подписей**

Сохранить как `$TMPDIR/s9-transform.sh` и выполнить `bash $TMPDIR/s9-transform.sh`:

```bash
#!/usr/bin/env bash
# S9, шаг 1 задачи 2: подписи List* во всех слоях. Запуск из корня репозитория.
set -euo pipefail
S=internal/store/sqlstore
NONTEST=$(ls $S/*.go | grep -v _test.go)

# порт и адаптер
perl -0pi -e 's/^(\tList\w+)\(ctx context\.Context\)/$1(ctx context.Context, access models.Access, page models.Page)/mg' internal/store/deps.go
perl -0pi -e '
  s/(func \(s \*Store\) List\w+)\(ctx context\.Context\)/$1(ctx context.Context, access models.Access, page models.Page)/g;
  s/listEntities\(ctx, s, ("[a-z_]+"), /listEntities(ctx, s, $1, access, page, /g;
' $NONTEST

# существующие тесты: вызовы List* без окна и с полным доступом
perl -0pi -e 's/\b(\w+)\.(List[A-Z]\w*)\(t\.Context\(\)\)/$1.$2(t.Context(), models.AccessFull, models.Page{})/g' $(ls $S/*_test.go)

# вызовы listEntities и List* с ctx-переменной в тестах адаптера
perl -0pi -e 's/listEntities\(t\.Context\(\), s, ("[a-z_]+"), /listEntities(t.Context(), s, $1, models.AccessFull, models.Page{}, /g' $(ls $S/*_test.go)
perl -0pi -e 's/\b(\w+)\.(List[A-Z]\w*)\(ctx\)/$1.$2(ctx, models.AccessFull, models.Page{})/g' $(ls $S/*_test.go)
```

- [ ] **Step 2: Правки, не сводящиеся к заменам**

Сохранить как `$TMPDIR/s9-edit.py` и выполнить `python3 $TMPDIR/s9-edit.py` (при несовпадении фрагмента скрипт останавливается с сообщением — не подгонять молча, разобраться):

```python
"""S9, шаг 2 задачи 2: правки, не сводящиеся к заменам. Запуск из корня репозитория."""


def edit(p, pairs):
    s = open(p, encoding="utf-8").read()
    for old, new in pairs:
        if old not in s:
            raise SystemExit(f"{p}: не найден фрагмент: {old[:60]!r}")
        s = s.replace(old, new, 1)
    open(p, "w", encoding="utf-8").write(s)


edit("internal/store/sqlstore/sqlstore.go", [
    ("""// listIDs возвращает идентификаторы всех строк таблицы в порядке вставки.
func listIDs(q queryer, table string) ([]models.ID, error) {
	return scanRows(q, func(r *sql.Rows) (models.ID, error) {
		var id string
		err := r.Scan(&id)

		return models.ID(id), err
	}, `SELECT id FROM `+table+` ORDER BY rowid`)
}

// listEntities читает список сущностей таблицы: сначала все id (курсор
// закрыт), затем Get по каждому. Масштаб личной генеалогии это позволяет.
func listEntities[T any](ctx context.Context, s *Store, table string, get func(context.Context, models.ID) (*T, error)) ([]*T, error) {
	ids, err := listIDs(s.run(ctx), table)
	if err != nil {
		return nil, err
	}
""", """// listIDs возвращает идентификаторы строк таблицы в порядке вставки (rowid)
// в окне page; publicOnly исключает строки с private = 1 на уровне SQL.
func listIDs(q queryer, table string, publicOnly bool, page models.Page) ([]models.ID, error) {
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
}

// listEntities читает окно списка сущностей таблицы: сначала id страницы
// (курсор закрыт), затем Get по каждому. Любой режим, кроме AccessFull,
// скрывает приватные строки таблиц, у которых есть колонка private.
func listEntities[T any](
	ctx context.Context, s *Store, table string, access models.Access, page models.Page,
	get func(context.Context, models.ID) (*T, error),
) ([]*T, error) {
	g, err := s.graph(ctx)
	if err != nil {
		return nil, err
	}

	ids, err := listIDs(s.run(ctx), table, access != models.AccessFull && g.hasPrivate(table), page)
	if err != nil {
		return nil, err
	}
"""),
])

edit("internal/store/sqlstore/fkgraph.go", [
    ("""// schemaGraph — все внешние ключи схемы в стабильном порядке.
type schemaGraph struct {
	edges []fkEdge
}""", """// schemaGraph — все внешние ключи схемы в стабильном порядке и таблицы с
// колонкой private (по ним List* фильтрует приватное для AccessPublic).
type schemaGraph struct {
	edges   []fkEdge
	private map[string]bool
}"""),
    ("""	if err != nil {
		return nil, err
	}

	return &schemaGraph{edges: edges}, nil
}""", """	if err != nil {
		return nil, err
	}

	names, err := scanRows(s.run(ctx), func(r *sql.Rows) (string, error) {
		var n string
		err := r.Scan(&n)

		return n, err
	}, `SELECT m.name FROM sqlite_master AS m, pragma_table_info(m.name) AS p
		WHERE m.type = 'table' AND m.name NOT LIKE 'sqlite_%' AND p.name = 'private'
		ORDER BY m.name`)
	if err != nil {
		return nil, err
	}

	private := make(map[string]bool, len(names))
	for _, n := range names {
		private[n] = true
	}

	return &schemaGraph{edges: edges, private: private}, nil
}

// hasPrivate — есть ли у таблицы колонка private.
func (g *schemaGraph) hasPrivate(table string) bool { return g.private[table] }"""),
    ("// loadSchemaGraph читает внешние ключи всех таблиц одним запросом.",
     "// loadSchemaGraph читает внешние ключи всех таблиц и список таблиц с колонкой private."),
])

edit("internal/store/deps.go", [
    ("//   - Save* — upsert", """//   - List*(ctx, access, page) отдаёт окно списка в порядке сохранения (стабильном);
//     page нормализуется (Limit 0 → models.DefaultPageLimit, не больше
//     models.MaxPageLimit). Любой access, кроме models.AccessFull, скрывает
//     сущности с Private = true (для сущностей без флага режим не влияет).
//   - Save* — upsert"""),
])

edit("internal/usecases/list_settlements/deps.go", [
    ("ListAdministrativeDivisions(ctx context.Context) (",
     "ListAdministrativeDivisions(ctx context.Context, access models.Access, page models.Page) ("),
])

edit("internal/usecases/list_settlements/scenario.go", [
    ("""	divisions, err := s.adminDivisions.ListAdministrativeDivisions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]models.AdministrativeDivision, 0, len(divisions))
	for _, d := range divisions {
		if d.Type.IsSettlement() {
			out = append(out, *d)
		}
	}
	return out, nil""", """	out := []models.AdministrativeDivision{}

	// репозиторий отдаёт окна: обходим их все до пустого
	for offset := 0; ; offset += models.MaxPageLimit {
		divisions, err := s.adminDivisions.ListAdministrativeDivisions(ctx, models.AccessFull,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(divisions) == 0 {
			return out, nil
		}

		for _, d := range divisions {
			if d.Type.IsSettlement() {
				out = append(out, *d)
			}
		}
	}"""),
])
```

Затем перегенерировать моки:

```bash
(cd internal/store && go generate ./...)
(cd internal/usecases/list_settlements && go generate ./...)
```

- [ ] **Step 3: Тесты сценария**

Заменить содержимое `internal/usecases/list_settlements/scenario_test.go` на:

```go
package list_settlements

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type ctxKey struct{}

// fakeRepo отдаёт окна списка как настоящий репозиторий и запоминает вызовы.
type fakeRepo struct {
	list    []*models.AdministrativeDivision
	err     error
	errAt   int // номер вызова (с 1), на котором возвращается err; 0 — на любом
	gotCtx  context.Context
	calls   []models.Page
	accesses []models.Access
}

func (f *fakeRepo) ListAdministrativeDivisions(
	ctx context.Context, access models.Access, page models.Page,
) ([]*models.AdministrativeDivision, error) {
	f.gotCtx = ctx
	f.calls = append(f.calls, page)
	f.accesses = append(f.accesses, access)

	if f.err != nil && (f.errAt == 0 || f.errAt == len(f.calls)) {
		return nil, f.err
	}

	page = page.Normalized()
	if page.Offset >= len(f.list) {
		return nil, nil
	}

	end := min(page.Offset+page.Limit, len(f.list))

	return f.list[page.Offset:end], nil
}

func TestScenarioListSettlementsFiltersDivisions(t *testing.T) {
	sc := New(&fakeRepo{
		list: []*models.AdministrativeDivision{
			{ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo},
			{ID: "ad-2", Name: "Никифоровская", Type: models.AdminDivisionVolost},
			{ID: "ad-3", Name: "Никифорово", Type: models.AdminDivisionDerevnya},
		},
	})

	got, err := sc.ListSettlements(context.Background())
	if err != nil {
		t.Fatalf("ListSettlements: %v", err)
	}
	if len(got) != 2 || got[0].ID != "ad-1" || got[1].ID != "ad-3" {
		t.Fatalf("got %+v, want ad-1 и ad-3 (волость отфильтрована)", got)
	}
}

// TestScenarioWalksAllPages: населённые пункты за пределами первого окна не
// теряются, окна запрашиваются подряд с полным доступом.
func TestScenarioWalksAllPages(t *testing.T) {
	total := 2*models.MaxPageLimit + 7

	repo := &fakeRepo{}
	for i := 0; i < total; i++ {
		typ := models.AdminDivisionDerevnya
		if i%2 == 1 {
			typ = models.AdminDivisionVolost // не населённый пункт
		}

		repo.list = append(repo.list, &models.AdministrativeDivision{
			ID: models.ID("ad-" + strconv.Itoa(i)), Type: typ,
		})
	}

	got, err := New(repo).ListSettlements(context.Background())
	if err != nil {
		t.Fatalf("ListSettlements: %v", err)
	}

	want := (total + 1) / 2 // чётные индексы
	if len(got) != want {
		t.Fatalf("населённых пунктов %d, ожидалось %d", len(got), want)
	}

	// три окна с данными и пустое, завершающее обход
	wantCalls := []models.Page{
		{Limit: models.MaxPageLimit, Offset: 0},
		{Limit: models.MaxPageLimit, Offset: models.MaxPageLimit},
		{Limit: models.MaxPageLimit, Offset: 2 * models.MaxPageLimit},
		{Limit: models.MaxPageLimit, Offset: 3 * models.MaxPageLimit},
	}
	if len(repo.calls) != len(wantCalls) {
		t.Fatalf("вызовов репозитория %d (%v), ожидалось %d", len(repo.calls), repo.calls, len(wantCalls))
	}

	for i, c := range wantCalls {
		if repo.calls[i] != c {
			t.Errorf("вызов %d: окно %+v, ожидалось %+v", i+1, repo.calls[i], c)
		}

		if repo.accesses[i] != models.AccessFull {
			t.Errorf("вызов %d: доступ %v, ожидался AccessFull", i+1, repo.accesses[i])
		}
	}
}

func TestScenarioEmptyRepoGivesEmptyNotNil(t *testing.T) {
	got, err := New(&fakeRepo{}).ListSettlements(context.Background())
	if err != nil {
		t.Fatalf("ListSettlements: %v", err)
	}

	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v, ожидался пустой не-nil срез", got)
	}
}

func TestScenarioPropagatesRepoError(t *testing.T) {
	wantErr := errors.New("repo down")
	sc := New(&fakeRepo{err: wantErr})

	if _, err := sc.ListSettlements(context.Background()); !errors.Is(err, wantErr) {
		t.Errorf("err=%v, want %v", err, wantErr)
	}
}

// TestScenarioPropagatesErrorFromLaterPage: сбой на втором окне не даёт
// частичного результата.
func TestScenarioPropagatesErrorFromLaterPage(t *testing.T) {
	wantErr := errors.New("repo down on page 2")

	repo := &fakeRepo{err: wantErr, errAt: 2}
	for i := 0; i < models.MaxPageLimit+1; i++ {
		repo.list = append(repo.list, &models.AdministrativeDivision{
			ID: models.ID("ad-" + strconv.Itoa(i)), Type: models.AdminDivisionDerevnya,
		})
	}

	got, err := New(repo).ListSettlements(context.Background())
	if !errors.Is(err, wantErr) || got != nil {
		t.Fatalf("got %v, %v; ожидалась ошибка %v без результата", got, err, wantErr)
	}
}

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

- [ ] **Step 4: Общие тесты порта под новые подписи**

Сохранить как `$TMPDIR/s9-tests-edit.py` и выполнить `python3 $TMPDIR/s9-tests-edit.py`:

```python
"""S9, шаг 3 задачи 2: context_test.go под новые подписи List*. Из корня репозитория."""
import re

p = "internal/store/sqlstore/context_test.go"
s = open(p, encoding="utf-8").read()


def sub(old, new):
    global s
    if old not in s:
        raise SystemExit(f"{p}: не найден фрагмент: {old[:60]!r}")
    s = s.replace(old, new, 1)


sub("""// getFunc и listFunc — методы порта в виде выражений методов, приведённые к
// одному виду: так один тест обходит все 21 сущность.
type (
	getFunc  func(*Store, context.Context, models.ID) error
	listFunc func(*Store, context.Context) error
)
""", """// getFunc и listerSpec — методы порта в виде выражений методов, приведённые к
// одному виду: так один тест обходит все 21 сущность.
type (
	getFunc func(*Store, context.Context, models.ID) error

	// listerSpec — List*-метод с таблицей главной строки; список отдаёт id.
	listerSpec struct {
		table string
		list  func(*Store, context.Context, models.Access, models.Page) ([]models.ID, error)
	}
)
""")
sub("""func probeList[T any](list func(*Store, context.Context) ([]*T, error)) listFunc {
	return func(s *Store, ctx context.Context) error {
		_, err := list(s, ctx)

		return err
	}
}""", """func probeList[T any](
	table string, list func(*Store, context.Context, models.Access, models.Page) ([]*T, error),
) listerSpec {
	return listerSpec{table: table, list: func(s *Store, ctx context.Context, a models.Access, p models.Page) ([]models.ID, error) {
		items, err := list(s, ctx, a, p)

		ids := make([]models.ID, len(items))
		for i, it := range items {
			ids[i] = models.ID(reflect.ValueOf(it).Elem().FieldByName("ID").String())
		}

		return ids, err
	}}
}""")
sub("var listers = map[string]listFunc{", "var listers = map[string]listerSpec{")

tables = {
    "People": "persons", "Relations": "relations", "Residences": "residences", "Families": "families",
    "Surnames": "surnames", "GivenNames": "given_names", "Patronymics": "patronymics", "Estates": "estates",
    "Titles": "titles", "AdministrativeDivisions": "administrative_divisions", "Churches": "churches",
    "Parishes": "parishes", "Events": "events", "Sources": "sources", "Archives": "archives",
    "ArchiveNodes": "archive_nodes", "ArchiveDocuments": "archive_documents", "Attachments": "attachments",
    "Citations": "citations", "Notes": "notes", "Repositories": "repositories",
}
for name, table in tables.items():
    sub(f"probeList((*Store).List{name}),", f'probeList("{table}", (*Store).List{name}),')

sub("""	for name, list := range listers {
		t.Run(name, func(t *testing.T) {
			if err := list(s, canceled(t)); !errors.Is(err, context.Canceled) {""", """	for name, spec := range listers {
		t.Run(name, func(t *testing.T) {
			if _, err := spec.list(s, canceled(t), models.AccessFull, models.Page{}); !errors.Is(err, context.Canceled) {""")
open(p, "w", encoding="utf-8").write(s)
```

Затем `gofmt -w internal/store/sqlstore/*_test.go internal/usecases/list_settlements/scenario_test.go`.

- [ ] **Step 5: Рубеж**

Run: `gofmt -l . && go build ./... && go vet ./... && go test ./... -count=1`
Expected: gofmt пусто, сборка и `vet` без ошибок, все пакеты `ok`. Если `go build` жалуется на `web/embed.go` (`pattern all:dist`) — соберите `go build ./internal/... ./cmd/...`.

- [ ] **Step 6: Commit**

```bash
git add -A internal
git commit -m "feat(store): List* с режимом доступа и окном"
```

---

### Task 3: Тесты окон и приватности

**Files:**
- Create: `internal/store/sqlstore/list_test.go`
- Modify: `internal/store/sqlstore/fkgraph_test.go`

**Interfaces:**
- Consumes: Task 2; `listers`/`listerSpec` (`context_test.go`), `fullChain`, `snapshot` (`delete_test.go`), `withTimeout` (`tx_test.go`), `countRows`, `mustDo`, `entityTables`, `typeOfTable`.
- Produces: `tableHasPrivate`; тесты `TestListPagesPartitionEveryKind`, `TestListPublicHidesPrivateOnly`, `TestListDefaultAndMaxPageLimit`, `TestListOrderSurvivesResave`, `TestPrivateColumnTablesAreTheEntitiesWithFlag`.

- [ ] **Step 1: Создать тесты**

Создать `internal/store/sqlstore/list_test.go`:

```go
package sqlstore

import (
	"reflect"
	"strconv"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// tableHasPrivate — есть ли у таблицы колонка private (независимо от кода адаптера).
func tableHasPrivate(t *testing.T, s *Store, table string) bool {
	t.Helper()

	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = 'private'`, table).Scan(&n); err != nil {
		t.Fatalf("table_info %s: %v", table, err)
	}

	return n > 0
}

// TestListPagesPartitionEveryKind: для каждого из 21 видов окна любого размера
// не пересекаются и в сумме дают весь список, окно за пределами набора пусто,
// AccessPublic скрывает ровно приватные строки (а на таблицах без флага не
// влияет). Набор — fullChain: по сущности каждого вида, часть приватная.
func TestListPagesPartitionEveryKind(t *testing.T) {
	s := newStore(t)

	for _, st := range fullChain() {
		mustDo(t, "save "+st.kind+" "+string(st.id), st.save(t.Context(), s))
	}

	hidden := 0

	for name, spec := range listers {
		t.Run(name, func(t *testing.T) {
			ctx := t.Context()

			all, err := spec.list(s, ctx, models.AccessFull, models.Page{})
			mustDo(t, "list all", err)

			total := countRows(t, s, spec.table)
			if total == 0 || len(all) != total {
				t.Fatalf("List%s = %d строк, в таблице %d (набор должен покрывать вид)", name, len(all), total)
			}

			for _, size := range []int{1, 2, 3} {
				var got []models.ID

				// обход ограничен: сломанный сдвиг не должен зациклить тест
				for off := 0; off < total+size; off += size {
					page, err := spec.list(s, ctx, models.AccessFull, models.Page{Limit: size, Offset: off})
					mustDo(t, "list page", err)

					if len(page) > size {
						t.Fatalf("окно размера %d вернуло %d строк", size, len(page))
					}

					if len(page) == 0 {
						break
					}

					got = append(got, page...)
				}

				if !reflect.DeepEqual(got, all) {
					t.Fatalf("окна по %d в сумме %v, полный список %v", size, got, all)
				}
			}

			beyond, err := spec.list(s, ctx, models.AccessFull, models.Page{Limit: 5, Offset: total})
			mustDo(t, "list beyond", err)

			if len(beyond) != 0 {
				t.Fatalf("окно за пределами набора вернуло %v", beyond)
			}

			wantPublic := total
			if tableHasPrivate(t, s, spec.table) {
				var priv int
				if err := s.db.QueryRow(`SELECT COUNT(*) FROM ` + spec.table + ` WHERE private = 1`).Scan(&priv); err != nil {
					t.Fatalf("count private: %v", err)
				}

				wantPublic -= priv
				hidden += priv
			}

			public, err := spec.list(s, ctx, models.AccessPublic, models.Page{})
			mustDo(t, "list public", err)

			if len(public) != wantPublic {
				t.Fatalf("AccessPublic вернул %d строк, ожидалось %d", len(public), wantPublic)
			}
		})
	}

	if hidden == 0 {
		t.Fatal("в наборе нет приватных строк — проверка фильтра пуста")
	}
}

// TestListPublicHidesPrivateOnly: точный состав при разных режимах, включая
// неизвестный (он не открывает приватное).
func TestListPublicHidesPrivateOnly(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()

	mustDo(t, "p-1", s.SavePerson(ctx, &models.Person{ID: "p-1", Private: true}))
	mustDo(t, "p-2", s.SavePerson(ctx, &models.Person{ID: "p-2"}))
	mustDo(t, "p-3", s.SavePerson(ctx, &models.Person{ID: "p-3", Private: true}))
	mustDo(t, "p-4", s.SavePerson(ctx, &models.Person{ID: "p-4"}))

	ids := func(access models.Access, page models.Page) []models.ID {
		got, err := listers["People"].list(s, ctx, access, page)
		mustDo(t, "list", err)

		return got
	}

	cases := []struct {
		name   string
		access models.Access
		page   models.Page
		want   []models.ID
	}{
		{"полный доступ", models.AccessFull, models.Page{}, []models.ID{"p-1", "p-2", "p-3", "p-4"}},
		{"публичный доступ", models.AccessPublic, models.Page{}, []models.ID{"p-2", "p-4"}},
		{"неизвестный режим — как публичный", models.Access(7), models.Page{}, []models.ID{"p-2", "p-4"}},
		{"окно считается после фильтра", models.AccessPublic, models.Page{Limit: 1, Offset: 1}, []models.ID{"p-4"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ids(c.access, c.page); !reflect.DeepEqual(got, c.want) {
				t.Fatalf("получено %v, ожидалось %v", got, c.want)
			}
		})
	}

	// у делений нет флага приватности: публичный режим отдаёт всё
	mustDo(t, "division", s.SaveAdministrativeDivision(ctx, &models.AdministrativeDivision{
		ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionDerevnya,
	}))

	divisions, err := listers["AdministrativeDivisions"].list(s, ctx, models.AccessPublic, models.Page{})
	mustDo(t, "list divisions", err)

	if len(divisions) != 1 {
		t.Fatalf("деления при AccessPublic: %v, ожидалась одна запись", divisions)
	}
}

// TestListDefaultAndMaxPageLimit: размер окна по умолчанию и предел.
func TestListDefaultAndMaxPageLimit(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)

	const total = models.MaxPageLimit + 10

	mustDo(t, "seed", s.InTx(ctx, func(tx store.Store) error {
		for i := 0; i < total; i++ {
			if err := tx.SavePerson(ctx, &models.Person{ID: models.ID("p-" + strconv.Itoa(1000+i))}); err != nil {
				return err
			}
		}

		return nil
	}))

	cases := []struct {
		name string
		page models.Page
		want int
	}{
		{"нулевое окно — 50", models.Page{}, models.DefaultPageLimit},
		{"отрицательный лимит — 50", models.Page{Limit: -5}, models.DefaultPageLimit},
		{"лимит выше предела — 500", models.Page{Limit: 1 << 20}, models.MaxPageLimit},
		{"хвост после предела", models.Page{Limit: models.MaxPageLimit, Offset: models.MaxPageLimit}, 10},
		{"отрицательный сдвиг как ноль", models.Page{Limit: 3, Offset: -4}, 3},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := listers["People"].list(s, ctx, models.AccessFull, c.page)
			mustDo(t, "list", err)

			if len(got) != c.want {
				t.Fatalf("окно %+v вернуло %d строк, ожидалось %d", c.page, len(got), c.want)
			}
		})
	}
}

// TestListOrderSurvivesResave: повторное сохранение (upsert) не меняет место
// сущности в списке — окна остаются стабильными.
func TestListOrderSurvivesResave(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()

	for _, id := range []models.ID{"p-c", "p-a", "p-b"} {
		mustDo(t, "save "+string(id), s.SavePerson(ctx, &models.Person{ID: id}))
	}

	mustDo(t, "resave", s.SavePerson(ctx, &models.Person{ID: "p-c", Gender: models.Female}))

	got, err := listers["People"].list(s, ctx, models.AccessFull, models.Page{Limit: 2, Offset: 0})
	mustDo(t, "list", err)

	if want := []models.ID{"p-c", "p-a"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("первое окно после пересохранения %v, ожидалось %v", got, want)
	}
}
```

В `internal/store/sqlstore/fkgraph_test.go` добавить `"reflect"` в import (между `"database/sql"` и `"sort"`) и дописать в конец файла:

```go
// TestPrivateColumnTablesAreTheEntitiesWithFlag: фильтр AccessPublic опирается
// на граф схемы; набор таблиц с колонкой private — ровно сущности с флагом
// приватности в моделях.
func TestPrivateColumnTablesAreTheEntitiesWithFlag(t *testing.T) {
	s := newStore(t)

	g, err := s.graph(t.Context())
	if err != nil {
		t.Fatalf("graph: %v", err)
	}

	want := []string{
		"archive_documents", "archive_nodes", "archives", "attachments", "citations", "events",
		"families", "notes", "persons", "relations", "repositories", "residences", "sources",
	}

	var got []string

	for _, table := range entityTables {
		if g.hasPrivate(table) {
			got = append(got, table)
		}
	}

	sort.Strings(got)

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("таблицы с колонкой private %v, ожидалось %v", got, want)
	}

	for table := range g.private {
		if _, isEntity := typeOfTable[table]; !isEntity {
			t.Errorf("колонка private у таблицы %s, не являющейся сущностью", table)
		}
	}
}
```

- [ ] **Step 2: Прогнать**

Run: `gofmt -l . && go vet ./... && go test ./... -count=1`
Expected: gofmt пусто, `vet` чисто, все пакеты `ok`.

- [ ] **Step 3: Мутационные проверки (вручную; каждую откатить сразу: `git checkout -- <файл>`)**

Запускать `go test ./internal/store/sqlstore/ ./internal/usecases/... ./internal/models/ -count=1 -timeout 100s`; указанный тест должен ПРОВАЛИТЬСЯ.

| Мутация | Ожидаемый провал |
|---|---|
| в `listIDs` (`sqlstore.go`) `where = ` WHERE private = 0`` → `where = ""` | `TestListPagesPartitionEveryKind`, `TestListPublicHidesPrivateOnly` |
| в `listEntities` `access != models.AccessFull && …` → `access == models.AccessPublic && …` | `TestListPublicHidesPrivateOnly/неизвестный режим` |
| в `listIDs` убрать ` OFFSET ?` и аргумент `page.Offset` | `TestListPagesPartitionEveryKind` |
| в `listIDs` удалить строку `page = page.Normalized()` | `TestListPagesPartitionEveryKind`, `TestListEntitiesSkipsRowDeletedMidList` |
| в `listIDs` `ORDER BY rowid` → `ORDER BY id` | `TestListOrderSurvivesResave`, `TestStoreListPeople` |
| в `loadSchemaGraph` (`fkgraph.go`) заменить `private[n] = true` на `_ = n` | `TestPrivateColumnTablesAreTheEntitiesWithFlag`, `TestListPagesPartitionEveryKind` |
| в сценарии `len(divisions) == 0` → `len(divisions) >= 0` | `TestScenarioWalksAllPages`, `TestScenarioPropagatesErrorFromLaterPage` |
| в `Page.Normalized` (`models/query.go`) удалить ветку `case p.Limit > MaxPageLimit:` (две строки) | `TestPageNormalized`, `TestListDefaultAndMaxPageLimit` |

- [ ] **Step 4: Commit**

```bash
git add internal/store/sqlstore/list_test.go internal/store/sqlstore/fkgraph_test.go
git commit -m "test(store): окна и приватность списков для 21 вида"
```

---

### Task 4: Документация

**Files:**
- Modify: `docs/data-model/core-read-write.md` (§4), `docs/plans/2026-09-20-core-rw-roadmap.md` (S9)

- [ ] **Step 1: Спецификация §4**

В `docs/data-model/core-read-write.md` заменить пункт

```
- `List*(ctx, Access, Page)` — пагинация везде; `AccessPublic` исключает
  сущности с `private = 1` на уровне SQL (для сущностей без флага — не влияет).
```

на

```
- `List*(ctx, Access, Page)` — пагинация везде; `AccessPublic` исключает
  сущности с `private = 1` на уровне SQL (для сущностей без флага — не влияет).
  Порядок — порядок сохранения (`rowid`), окно считается после фильтра;
  `Page` нормализуется (`Limit` ≤ 0 → 50, > 500 → 500, отрицательный `Offset` →
  0), любой режим, кроме `AccessFull`, — публичный (безопасный отказ). Таблицы
  с колонкой `private` определяются по схеме. Реализовано в S9.
```

- [ ] **Step 2: Дорожная карта**

В `docs/plans/2026-09-20-core-rw-roadmap.md` в блоке `### S9.` заменить строки

```
- **Файлы:** `models/query.go` (`Page`, `Access`), `store/deps.go`,
  `sqlstore/*` (общий `listEntities`), тесты, `usecases`.
```

на

```
- **Файлы:** `models/query.go` (`Page`, `Access`), `store/deps.go`,
  `sqlstore/*` (`listEntities`, `listIDs`, признак `private` в графе схемы),
  тесты (`sqlstore/list_test.go`), `usecases/list_settlements` (обход окон);
  план — `2026-09-21-core-rw-s09-page-access.md`.
```

- [ ] **Step 3: Commit**

```bash
git add docs/data-model/core-read-write.md docs/plans/2026-09-20-core-rw-roadmap.md
git commit -m "docs: S9 — Page и Access"
```

---

## Правки по итогам ревью (внесены после выполнения задач)

Код в репозитории — источник истины; плановые блоки выше описывают первую версию. Отличия:

- `store/deps.go`: в комментарии `List*` правило `Limit` — «≤ 0 → по умолчанию».
- `list_test.go`: `TestListPagesPartitionEveryKind` считает «смешанные» виды (и приватные, и публичные строки) и требует их не меньше трёх; `TestListDefaultAndMaxPageLimit` проверяет ещё и первую строку окна; тест обхода окон ограничен по числу итераций.
