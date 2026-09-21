# S13: Контракт делений (чтение) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Единственный публичный контракт переименован и обобщён: `settlement_list` → MCP-тул `division_list`, `GET /api/settlements` → `GET /api/admin-divisions`, сценарий `list_settlements` → `list_divisions`, DTO `Settlement` → `AdminDivision{id, name, type, parent_id}`. «Населённые пункты» — параметр `kind=settlement`; добавлены `type`, `limit`, `offset`. Фронтенд работает на новом URL.

**Architecture:** В `models` (рядом с `Page`/`Access`) — `DivisionKind`, `DivisionQuery{Kind, Type, Page}` с `Validate()` (`*ValidationError` по полям `kind`/`type`/`limit`/`offset`) и `Matches(d)`. Сценарий `list_divisions.ListDivisions(ctx, q)` валидирует запрос, обходит окна репозитория (`ListAdministrativeDivisions`, `AccessFull`), фильтрует в памяти и применяет окно запроса ПОСЛЕ фильтра, останавливаясь, когда окно заполнено; результат короче окна — конец списка. `httpapi` и `mcp` разбирают параметры в `DivisionQuery`; ошибки: 400 — параметр не число, 422 — `*ValidationError` (тело `{error, field}`), 500 — остальное (MCP — ошибка внутри `CallToolResult`). DTO `transport.AdminDivision` (`parent_id` — `null` у корня). Прежних имён нет (тест: `/api/settlements` → 404, тул `settlement_list` не зарегистрирован). Сквозной тест `httpapi/store_test.go` идёт по настоящей БД. Фронтенд: `fetchAdminDivisions({kind: "settlement", limit: 500})`.

**Tech Stack:** Go 1.26.4, `mcp-go` v0.58.0, React/TypeScript (antd, vite).

**Spec:** `docs/data-model/core-read-write.md` §5; `docs/plans/2026-09-20-core-rw-roadmap.md` (этап S13).

## Global Constraints

- Имена контракта: MCP-тул `division_list`; HTTP `GET /api/admin-divisions`; DTO `AdminDivision` с тегами `id`, `name`, `type`, `parent_id`; сценарий `list_divisions`. Прежних (`settlement_list`, `/api/settlements`, `list_settlements`, `ListSettlements`, `SettlementService`, `transport.Settlement`, `fetchSettlements`) в коде и документации нет (кроме явных записей о переименовании в `docs/data-model/{core-read-write,decisions}.md`, `docs/todo.md` и исторических планов `docs/plans/`).
- Параметры (HTTP-запрос и аргументы тула одинаково): `kind` (`""` или `settlement`), `type` (`""` или значение `AdminDivisionType`), `limit`, `offset`. Отсутствующий параметр — нулевое значение. `Page.Normalized`: `limit` ≤ 0 → 50, > 500 → 500 (не ошибка); `limit`/`offset` < 0 — ошибка (`*ValidationError`). `kind` и `type` пересекаются.
- Окно применяется после фильтра; порядок — порядок сохранения; короткий результат — конец списка; пустой результат — `[]` (не `null`).
- Коды HTTP: 200; 400 — `limit`/`offset` не целое число (тело `{"error": "…"}`); 422 — `*models.ValidationError` из сценария (тело `{"error": "<поле>: <причина>", "field": "<поле>"}`); 500 — прочие ошибки (`{"error": "…"}`). MCP: любая ошибка — `CallToolResult` с `IsError` и текстом.
- Сценарий использует `AccessFull` (у делений нет флага приватности) — предпосылка S9: `Access` из обработчиков пробрасывается, когда появятся публичные срезы других сущностей.
- Схема БД, порт `store.Store`, `Validate` сущностей не меняются. Домен (`models`) без JSON-тегов.
- Комментарии и тексты ошибок — на русском; код gofmt-clean; фронтенд проходит `npm run typecheck` и `npm run build`.
- Проверка рубежа: `gofmt -l .` пусто, `go build ./...`, `go vet ./...`, `go test ./...`; для фронтенда — `cd web && npm run typecheck && npm run build`.

---

### Task 1: `DivisionKind` и `DivisionQuery` в `models`

**Files:**
- Modify: `internal/models/query.go`, `internal/models/query_test.go`

**Interfaces:**
- Consumes: `Page`, `AdminDivisionType` (`Valid`, `IsSettlement`), `AdministrativeDivision`, `fieldErr` (`validation.go`).
- Produces: `DivisionKind`, `DivisionKindSettlement`, `(DivisionKind).Valid()`, `DivisionQuery{Kind, Type, Page}`, `(DivisionQuery).Validate() error`, `(DivisionQuery).Matches(AdministrativeDivision) bool`.

- [ ] **Step 1: Тесты (падают — символов нет)**

В `internal/models/query_test.go` заменить строку `import "testing"` на

```go
import (
	"errors"
	"testing"
)
```

и дописать в конец файла:

```go
func TestDivisionKindValid(t *testing.T) {
	for _, k := range []DivisionKind{"", DivisionKindSettlement} {
		if !k.Valid() {
			t.Errorf("DivisionKind(%q).Valid() = false", k)
		}
	}

	if DivisionKind("village").Valid() {
		t.Error("неизвестный вид признан допустимым")
	}
}

func TestDivisionQueryValidate(t *testing.T) {
	cases := []struct {
		name  string
		q     DivisionQuery
		field string // "" — запрос корректен
	}{
		{"пустой запрос", DivisionQuery{}, ""},
		{"вид и тип", DivisionQuery{Kind: DivisionKindSettlement, Type: AdminDivisionSelo}, ""},
		{"окно больше предела — не ошибка", DivisionQuery{Page: Page{Limit: 10 * MaxPageLimit}}, ""},
		{"неизвестный вид", DivisionQuery{Kind: "village"}, "kind"},
		{"неизвестный тип", DivisionQuery{Type: "castle"}, "type"},
		{"отрицательный размер окна", DivisionQuery{Page: Page{Limit: -1}}, "limit"},
		{"отрицательный сдвиг", DivisionQuery{Page: Page{Offset: -5}}, "offset"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.q.Validate()
			if c.field == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, ожидалось nil", err)
				}

				return
			}

			var ve *ValidationError
			if !errors.As(err, &ve) || ve.Field != c.field {
				t.Fatalf("Validate() = %v, ожидалась *ValidationError по полю %q", err, c.field)
			}
		})
	}
}

func TestDivisionQueryMatches(t *testing.T) {
	selo := AdministrativeDivision{Type: AdminDivisionSelo}
	volost := AdministrativeDivision{Type: AdminDivisionVolost}

	cases := []struct {
		name string
		q    DivisionQuery
		d    AdministrativeDivision
		want bool
	}{
		{"без фильтров", DivisionQuery{}, volost, true},
		{"вид: населённый пункт проходит", DivisionQuery{Kind: DivisionKindSettlement}, selo, true},
		{"вид: волость не проходит", DivisionQuery{Kind: DivisionKindSettlement}, volost, false},
		{"тип совпал", DivisionQuery{Type: AdminDivisionVolost}, volost, true},
		{"тип не совпал", DivisionQuery{Type: AdminDivisionVolost}, selo, false},
		{"вид и тип пересекаются", DivisionQuery{Kind: DivisionKindSettlement, Type: AdminDivisionVolost}, volost, false},
	}

	for _, c := range cases {
		if got := c.q.Matches(c.d); got != c.want {
			t.Errorf("%s: Matches = %v, ожидалось %v", c.name, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Убедиться, что не компилируется**

Run: `go test ./internal/models/`
Expected: FAIL — `undefined: DivisionKind`.

- [ ] **Step 3: Реализация**

В конец `internal/models/query.go` дописать:

```go
// DivisionKind — предметный вид единицы деления для фильтра списка.
type DivisionKind string

// DivisionKindSettlement — только населённые пункты (AdminDivisionType.IsSettlement).
const DivisionKindSettlement DivisionKind = "settlement"

// Valid сообщает, допустимо ли значение фильтра (пустое — «без фильтра»).
func (k DivisionKind) Valid() bool { return k == "" || k == DivisionKindSettlement }

// DivisionQuery — запрос списка единиц административного деления: необязательные
// фильтры по виду и типу (пересекаются) и окно. Окно применяется после фильтра.
type DivisionQuery struct {
	Kind DivisionKind      // "" — без фильтра; settlement — только населённые пункты
	Type AdminDivisionType // "" — без фильтра; иначе точное совпадение типа
	Page Page
}

// Validate проверяет запрос: неизвестные вид и тип, отрицательные размер и
// сдвиг окна — *ValidationError (поля названы как параметры контракта).
// Размер окна больше MaxPageLimit не ошибка: Page.Normalized сужает его.
func (q DivisionQuery) Validate() error {
	switch {
	case !q.Kind.Valid():
		return fieldErr("kind", "неизвестный вид %q (допустимо: %q)", q.Kind, DivisionKindSettlement)
	case q.Type != "" && !q.Type.Valid():
		return fieldErr("type", "неизвестный тип единицы деления %q", q.Type)
	case q.Page.Limit < 0:
		return fieldErr("limit", "не может быть отрицательным: %d", q.Page.Limit)
	case q.Page.Offset < 0:
		return fieldErr("offset", "не может быть отрицательным: %d", q.Page.Offset)
	}

	return nil
}

// Matches сообщает, проходит ли единица деления фильтры запроса.
func (q DivisionQuery) Matches(d AdministrativeDivision) bool {
	if q.Kind == DivisionKindSettlement && !d.Type.IsSettlement() {
		return false
	}

	return q.Type == "" || d.Type == q.Type
}
```

- [ ] **Step 4: Тесты проходят**

Run: `gofmt -l internal && go vet ./internal/models/ && go test ./internal/models/ -count=1`
Expected: gofmt пусто, `vet` чисто, `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/models/query.go internal/models/query_test.go
git commit -m "feat(models): DivisionQuery — фильтры и окно списка делений"
```

---

### Task 2: Контракт `division_list` / `/api/admin-divisions`

**Files:**
- Rename: `internal/usecases/list_settlements/` → `internal/usecases/list_divisions/` (`deps.go`, `deps_test.go` — перегенерируется, `scenario.go`, `scenario_test.go` — заменяются)
- Delete: `internal/transport/settlement.go`, `internal/transport/settlement_test.go`, `internal/httpapi/settlement.go`, `internal/httpapi/settlement_test.go`, `internal/mcp/settlement.go`, `internal/mcp/settlement_test.go`
- Create: `internal/transport/admin_division.go` (+`_test.go`), `internal/httpapi/division.go` (+`division_test.go`, `store_test.go`), `internal/mcp/division.go` (+`division_test.go`)
- Modify: `internal/httpapi/deps.go`, `internal/httpapi/httpapi.go`, `internal/mcp/deps.go`, `internal/mcp/server.go`, `internal/app/app.go`

**Interfaces:**
- Consumes: Task 1; `store.Store.ListAdministrativeDivisions(ctx, access, page)` (S9); `sqlstore.Open`/`SaveAdministrativeDivision` (для `store_test.go`).
- Produces: `list_divisions.Scenario.ListDivisions(ctx, q models.DivisionQuery)`; `transport.AdminDivision`, `AdminDivisionFromModel`, `AdminDivisionsFromModels`; `httpapi.DivisionService`, `handleDivisionList`, `parseDivisionQuery`, `intParam`, `writeError`; `mcp.DivisionService`, `registerDivisionTools`, `divisionListHandler`, `divisionQueryFromRequest`, `optionalInt`.

Код проверен на чистой копии `main` (после Task 1): сборка, `vet` и все тесты зелёные.

- [ ] **Step 1: Переименование и удаления**

Сохранить как `$TMPDIR/s13-app.sh` (вне репозитория) и выполнить из корня: `bash $TMPDIR/s13-app.sh`:

```bash
#!/usr/bin/env bash
# S13, задача 2: сценарий переименован (git mv), прежние файлы контракта удалены,
# app.go переключён на новый сценарий. Из корня репозитория.
set -euo pipefail
git mv internal/usecases/list_settlements internal/usecases/list_divisions
sed -i.bak 's/package list_settlements/package list_divisions/' internal/usecases/list_divisions/*.go
rm -f internal/usecases/list_divisions/*.bak
git rm -q internal/transport/settlement.go internal/transport/settlement_test.go \
  internal/httpapi/settlement.go internal/httpapi/settlement_test.go \
  internal/mcp/settlement.go internal/mcp/settlement_test.go
python3 - <<'PY'
p = "internal/app/app.go"
s = open(p, encoding="utf-8").read()
pairs = [
    ('"github.com/amarin/genodex/internal/usecases/list_settlements"', '"github.com/amarin/genodex/internal/usecases/list_divisions"'),
    ("settlements := list_settlements.New(st)", "divisions := list_divisions.New(st)"),
    ("mcp.NewServer(settlements)", "mcp.NewServer(divisions)"),
    ("httpapi.NewHandler(settlements, ", "httpapi.NewHandler(divisions, "),
]
for old, new in pairs:
    if old not in s:
        raise SystemExit(f"app.go: не найден фрагмент {old!r}")
    s = s.replace(old, new, 1)
open(p, "w", encoding="utf-8").write(s)
PY
```

Затем `mkdir -p internal/transport` (каталог исчез вместе с удалёнными файлами).

- [ ] **Step 2: Сценарий**

Заменить содержимое `internal/usecases/list_divisions/scenario.go` на:

```go
package list_divisions

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список единиц административного деления».
type Scenario struct {
	adminDivisions AdminDivisionRepo
}

// New создаёт сценарий.
func New(adminDivisions AdminDivisionRepo) *Scenario {
	return &Scenario{adminDivisions: adminDivisions}
}

// ListDivisions возвращает единицы деления, прошедшие фильтры запроса, в порядке
// сохранения; окно (размер и сдвиг) применяется после фильтра. Некорректный запрос
// — *models.ValidationError, репозиторий не вызывается. Короткий результат
// (меньше размера окна) означает конец списка.
func (s *Scenario) ListDivisions(ctx context.Context, q models.DivisionQuery) ([]models.AdministrativeDivision, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	page := q.Page.Normalized()
	out := []models.AdministrativeDivision{}
	matched := 0 // сколько единиц прошло фильтр (для сдвига окна)

	// репозиторий отдаёт окна: обходим их до пустого или до заполнения окна запроса
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
			if !q.Matches(*d) {
				continue
			}

			if matched >= page.Offset {
				out = append(out, *d)

				if len(out) == page.Limit {
					return out, nil
				}
			}

			matched++
		}
	}
}
```

Заменить содержимое `internal/usecases/list_divisions/scenario_test.go` на:

```go
package list_divisions

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
	list     []*models.AdministrativeDivision
	err      error
	errAt    int // номер вызова (с 1), на котором возвращается err; 0 — на любом
	gotCtx   context.Context
	calls    []models.Page
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

func division(id string, typ models.AdminDivisionType) *models.AdministrativeDivision {
	return &models.AdministrativeDivision{ID: models.ID(id), Name: id, Type: typ}
}

func ids(list []models.AdministrativeDivision) []models.ID {
	out := []models.ID{}
	for _, d := range list {
		out = append(out, d.ID)
	}

	return out
}

func sameIDs(got []models.ID, want ...models.ID) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}

	return true
}

func sample() *fakeRepo {
	return &fakeRepo{list: []*models.AdministrativeDivision{
		division("ad-1", models.AdminDivisionSelo),
		division("ad-2", models.AdminDivisionVolost),
		division("ad-3", models.AdminDivisionDerevnya),
		division("ad-4", models.AdminDivisionSelo),
		division("ad-5", models.AdminDivisionGovernorate),
	}}
}

func TestListDivisionsWithoutFiltersReturnsAll(t *testing.T) {
	got, err := New(sample()).ListDivisions(context.Background(), models.DivisionQuery{})
	if err != nil || !sameIDs(ids(got), "ad-1", "ad-2", "ad-3", "ad-4", "ad-5") {
		t.Fatalf("got %v, %v; ожидались все пять единиц", ids(got), err)
	}
}

func TestListDivisionsKindSettlement(t *testing.T) {
	got, err := New(sample()).ListDivisions(context.Background(), models.DivisionQuery{Kind: models.DivisionKindSettlement})
	if err != nil || !sameIDs(ids(got), "ad-1", "ad-3", "ad-4") {
		t.Fatalf("got %v, %v; ожидались ad-1, ad-3, ad-4 (волость и губерния отфильтрованы)", ids(got), err)
	}
}

func TestListDivisionsTypeFilter(t *testing.T) {
	got, err := New(sample()).ListDivisions(context.Background(), models.DivisionQuery{Type: models.AdminDivisionSelo})
	if err != nil || !sameIDs(ids(got), "ad-1", "ad-4") {
		t.Fatalf("got %v, %v; ожидались ad-1 и ad-4", ids(got), err)
	}

	got, err = New(sample()).ListDivisions(context.Background(),
		models.DivisionQuery{Kind: models.DivisionKindSettlement, Type: models.AdminDivisionVolost})
	if err != nil || len(got) != 0 {
		t.Fatalf("вид и тип пересекаются: got %v, %v; ожидался пустой результат", ids(got), err)
	}
}

// TestListDivisionsWindowIsAppliedAfterFilter: сдвиг и размер считаются среди
// прошедших фильтр, а не среди всех единиц.
func TestListDivisionsWindowIsAppliedAfterFilter(t *testing.T) {
	q := models.DivisionQuery{Kind: models.DivisionKindSettlement, Page: models.Page{Limit: 1, Offset: 1}}

	got, err := New(sample()).ListDivisions(context.Background(), q)
	if err != nil || !sameIDs(ids(got), "ad-3") {
		t.Fatalf("got %v, %v; ожидалась одна ad-3 (второй населённый пункт)", ids(got), err)
	}

	q.Page = models.Page{Limit: 5, Offset: 2}

	got, err = New(sample()).ListDivisions(context.Background(), q)
	if err != nil || !sameIDs(ids(got), "ad-4") {
		t.Fatalf("got %v, %v; ожидалась одна ad-4 (короткое окно — конец списка)", ids(got), err)
	}

	q.Page = models.Page{Offset: 10}

	got, err = New(sample()).ListDivisions(context.Background(), q)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("сдвиг за пределами набора: got %#v, %v; ожидался пустой не-nil срез", got, err)
	}
}

// TestListDivisionsWalksAllRepositoryWindows: единицы за первым окном репозитория
// не теряются, окна запрашиваются подряд с полным доступом.
func TestListDivisionsWalksAllRepositoryWindows(t *testing.T) {
	total := 2*models.MaxPageLimit + 7

	repo := &fakeRepo{}

	for i := 0; i < total; i++ {
		typ := models.AdminDivisionDerevnya
		if i%2 == 1 {
			typ = models.AdminDivisionVolost // не населённый пункт
		}

		repo.list = append(repo.list, division("ad-"+strconv.Itoa(i), typ))
	}

	got, err := New(repo).ListDivisions(context.Background(),
		models.DivisionQuery{Kind: models.DivisionKindSettlement, Page: models.Page{Limit: models.MaxPageLimit}})
	if err != nil || len(got) != models.MaxPageLimit {
		t.Fatalf("got %d, %v; ожидалось %d", len(got), err, models.MaxPageLimit)
	}

	// окно запроса заполнено раньше конца списка: репозиторий читается не дальше
	// нужного (500 населённых пунктов лежат в первых двух окнах по 500)
	if len(repo.calls) != 2 {
		t.Fatalf("вызовов репозитория %d (%v), ожидалось 2", len(repo.calls), repo.calls)
	}

	for i, a := range repo.accesses {
		if a != models.AccessFull {
			t.Errorf("вызов %d: доступ %v, ожидался AccessFull", i+1, a)
		}
	}

	all := &fakeRepo{list: repo.list}

	got, err = New(all).ListDivisions(context.Background(), models.DivisionQuery{Kind: models.DivisionKindSettlement,
		Page: models.Page{Limit: models.MaxPageLimit, Offset: models.MaxPageLimit}})
	if err != nil || len(got) != (total+1)/2-models.MaxPageLimit {
		t.Fatalf("второе окно: %d, %v; ожидалось %d", len(got), err, (total+1)/2-models.MaxPageLimit)
	}
}

// TestListDivisionsInvalidQueryDoesNotTouchRepo: некорректный запрос —
// *ValidationError, репозиторий не вызывается.
func TestListDivisionsInvalidQueryDoesNotTouchRepo(t *testing.T) {
	repo := sample()

	for _, q := range []models.DivisionQuery{
		{Kind: "village"}, {Type: "castle"}, {Page: models.Page{Limit: -1}}, {Page: models.Page{Offset: -1}},
	} {
		_, err := New(repo).ListDivisions(context.Background(), q)

		var ve *models.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("запрос %+v: err = %v, ожидалась *ValidationError", q, err)
		}
	}

	if len(repo.calls) != 0 {
		t.Fatalf("репозиторий вызван %d раз при некорректном запросе", len(repo.calls))
	}
}

func TestListDivisionsEmptyRepoGivesEmptyNotNil(t *testing.T) {
	got, err := New(&fakeRepo{}).ListDivisions(context.Background(), models.DivisionQuery{})
	if err != nil {
		t.Fatalf("ListDivisions: %v", err)
	}

	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v, ожидался пустой не-nil срез", got)
	}
}

func TestListDivisionsPropagatesRepoError(t *testing.T) {
	wantErr := errors.New("repo down")

	if _, err := New(&fakeRepo{err: wantErr}).ListDivisions(context.Background(), models.DivisionQuery{}); !errors.Is(err, wantErr) {
		t.Errorf("err=%v, want %v", err, wantErr)
	}
}

// TestListDivisionsPropagatesErrorFromLaterWindow: сбой на втором окне
// репозитория не даёт частичного результата (после фильтра первое окно
// репозитория даёт лишь половину окна запроса, поэтому читается и второе).
func TestListDivisionsPropagatesErrorFromLaterWindow(t *testing.T) {
	wantErr := errors.New("repo down on window 2")

	repo := &fakeRepo{err: wantErr, errAt: 2}

	for i := 0; i < models.MaxPageLimit+1; i++ {
		typ := models.AdminDivisionDerevnya
		if i%2 == 1 {
			typ = models.AdminDivisionVolost
		}

		repo.list = append(repo.list, division("ad-"+strconv.Itoa(i), typ))
	}

	got, err := New(repo).ListDivisions(context.Background(),
		models.DivisionQuery{Kind: models.DivisionKindSettlement, Page: models.Page{Limit: models.MaxPageLimit}})
	if !errors.Is(err, wantErr) || got != nil {
		t.Fatalf("got %v, %v; ожидалась ошибка %v без результата", ids(got), err, wantErr)
	}
}

func TestListDivisionsPassesContextToRepo(t *testing.T) {
	repo := &fakeRepo{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "marker")

	if _, err := New(repo).ListDivisions(ctx, models.DivisionQuery{}); err != nil {
		t.Fatalf("ListDivisions: %v", err)
	}

	if repo.gotCtx == nil || repo.gotCtx.Value(ctxKey{}) != "marker" {
		t.Fatalf("репозиторий получил контекст %v, ожидался переданный сценарию", repo.gotCtx)
	}
}
```

Перегенерировать мок: `(cd internal/usecases/list_divisions && go generate ./...)`.

- [ ] **Step 3: `transport`**

Создать `internal/transport/admin_division.go` (комментарий пакета `transport` переехал сюда из удалённого `settlement.go`):

```go
// Package transport — DTO публичных контрактов (/api и MCP-тулы) и конвертеры
// из домена. Единственное место, где определена форма JSON на проводе; домен
// (internal/models) о JSON не знает.
package transport

import "github.com/amarin/genodex/internal/models"

// AdminDivision — контракт единицы административного деления
// (GET /api/admin-divisions, MCP-тул division_list). parent_id — null у корня.
type AdminDivision struct {
	ID       models.ID                `json:"id"`
	Name     string                   `json:"name"`
	Type     models.AdminDivisionType `json:"type"`
	ParentID *models.ID               `json:"parent_id"`
}

// AdminDivisionFromModel конвертирует единицу деления в контракт.
func AdminDivisionFromModel(d models.AdministrativeDivision) AdminDivision {
	var parent *models.ID

	if d.ParentID != nil {
		p := *d.ParentID // копия: контракт не делит указатель с моделью
		parent = &p
	}

	return AdminDivision{ID: d.ID, Name: d.Name, Type: d.Type, ParentID: parent}
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

Создать `internal/transport/admin_division_test.go`:

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

	want := `[{"id":"ad-root","name":"Московская","type":"governorate","parent_id":null},` +
		`{"id":"ad-1","name":"Давыдово","type":"selo","parent_id":"ad-root"}]`
	if string(list) != want {
		t.Fatalf("list = %s, want %s", list, want)
	}
}
```

- [ ] **Step 4: `httpapi`**

Заменить содержимое `internal/httpapi/deps.go`:

```go
package httpapi

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// DivisionService — контракт сценария «список единиц административного деления».
type DivisionService interface {
	ListDivisions(ctx context.Context, q models.DivisionQuery) ([]models.AdministrativeDivision, error)
}
```

Заменить содержимое `internal/httpapi/httpapi.go`:

```go
package httpapi

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"

	"github.com/amarin/genodex/internal/models"
)

// NewHandler возвращает http.Handler с маршрутами /api.
// docsFS — файловая система папки docs для раздела «Документация».
func NewHandler(divisions DivisionService, docsFS fs.FS) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handleHealth)
	mux.HandleFunc("GET /api/admin-divisions", handleDivisionList(divisions))
	mux.HandleFunc("GET /api/docs", handleDocList(docsFS))
	mux.HandleFunc("GET /api/docs/{path}", handleDocContent(docsFS))
	return mux
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError отвечает на ошибку сценария: *models.ValidationError — 422 с полем,
// остальное — 500.
func writeError(w http.ResponseWriter, err error) {
	var ve *models.ValidationError
	if errors.As(err, &ve) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": ve.Error(), "field": ve.Field})

		return
	}

	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}
```

Создать `internal/httpapi/division.go`:

```go
package httpapi

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleDivisionList — GET /api/admin-divisions?kind=&type=&limit=&offset=.
// Синтаксически неверный параметр (limit не число) — 400; неверное значение
// (неизвестные kind/type, отрицательное окно) — 422 (*models.ValidationError
// из сценария).
func handleDivisionList(divisions DivisionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := parseDivisionQuery(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := divisions.ListDivisions(r.Context(), q)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.AdminDivisionsFromModels(list))
	}
}

// parseDivisionQuery разбирает параметры запроса; отсутствующий — нулевое значение.
func parseDivisionQuery(v url.Values) (models.DivisionQuery, error) {
	q := models.DivisionQuery{
		Kind: models.DivisionKind(v.Get("kind")),
		Type: models.AdminDivisionType(v.Get("type")),
	}

	var err error

	if q.Page.Limit, err = intParam(v, "limit"); err != nil {
		return q, err
	}

	if q.Page.Offset, err = intParam(v, "offset"); err != nil {
		return q, err
	}

	return q, nil
}

// intParam читает необязательный целочисленный параметр.
func intParam(v url.Values, name string) (int, error) {
	raw := v.Get(name)
	if raw == "" {
		return 0, nil
	}

	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("параметр %s: ожидалось целое число, получено %q", name, raw)
	}

	return n, nil
}
```

Создать `internal/httpapi/division_test.go`:

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
}

func (f *fakeDivisions) ListDivisions(_ context.Context, q models.DivisionQuery) ([]models.AdministrativeDivision, error) {
	f.got = q

	return f.list, f.err
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

	rec := get(t, NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	want := `[{"id":"ad-root","name":"Московская","type":"governorate","parent_id":null},` +
		`{"id":"ad-1","name":"Давыдово","type":"selo","parent_id":"ad-root"}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}

	if !reflect.DeepEqual(svc.got, models.DivisionQuery{}) {
		t.Fatalf("запрос без параметров дошёл до сценария как %+v", svc.got)
	}
}

func TestDivisionListEmptyIsJSONArray(t *testing.T) {
	rec := get(t, NewHandler(&fakeDivisions{}, fstest.MapFS{}), "/api/admin-divisions")

	if got := strings.TrimSpace(rec.Body.String()); rec.Code != http.StatusOK || got != `[]` {
		t.Fatalf("status = %d, body = %s; ожидалось 200 и []", rec.Code, got)
	}
}

// TestDivisionListPassesParameters: параметры kind/type/limit/offset доходят до сценария.
func TestDivisionListPassesParameters(t *testing.T) {
	svc := &fakeDivisions{}

	rec := get(t, NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions?kind=settlement&type=selo&limit=20&offset=40")
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
		rec := get(t, NewHandler(svc, fstest.MapFS{}), target)

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

	rec := get(t, NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions?kind=village")

	want := `{"error":"kind: неизвестный вид","field":"kind"}`
	if got := strings.TrimSpace(rec.Body.String()); rec.Code != http.StatusUnprocessableEntity || got != want {
		t.Fatalf("status = %d, body = %s; ожидалось 422 и %s", rec.Code, got, want)
	}
}

func TestDivisionListServiceErrorIs500(t *testing.T) {
	svc := &fakeDivisions{err: errors.New("хранилище недоступно")}

	rec := get(t, NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions")

	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "хранилище недоступно") {
		t.Fatalf("status = %d, body = %s; ожидался 500 с текстом ошибки", rec.Code, rec.Body)
	}
}

// TestOldSettlementsRouteIsGone: прежнего имени контракта нет.
func TestOldSettlementsRouteIsGone(t *testing.T) {
	rec := get(t, NewHandler(&fakeDivisions{}, fstest.MapFS{}), "/api/settlements")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /api/settlements = %d, ожидался 404", rec.Code)
	}
}
```

Создать `internal/httpapi/store_test.go`:

```go
package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/httpapi"
	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store/sqlstore"
	"github.com/amarin/genodex/internal/usecases/list_divisions"
)

// TestAdminDivisionsWithRealStore: сквозной путь «хранилище → сценарий → HTTP»
// на настоящей БД (так же собран internal/app): фильтры, окно после фильтра,
// parent_id, коды ошибок, прежнего пути нет.
func TestAdminDivisionsWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	root := models.ID("ad-root")

	for _, d := range []models.AdministrativeDivision{
		{ID: "ad-root", Name: "Московская", Type: models.AdminDivisionGovernorate},
		{ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo, ParentID: &root},
		{ID: "ad-2", Name: "Никифоровская", Type: models.AdminDivisionVolost, ParentID: &root},
		{ID: "ad-3", Name: "Никифорово", Type: models.AdminDivisionDerevnya, ParentID: &root},
	} {
		if err := st.SaveAdministrativeDivision(t.Context(), &d); err != nil {
			t.Fatalf("save %s: %v", d.ID, err)
		}
	}

	h := httpapi.NewHandler(list_divisions.New(st), fstest.MapFS{})

	cases := []struct {
		target string
		code   int
		body   string // пусто — не сверять
	}{
		{"/api/admin-divisions?kind=settlement", http.StatusOK,
			`[{"id":"ad-1","name":"Давыдово","type":"selo","parent_id":"ad-root"},` +
				`{"id":"ad-3","name":"Никифорово","type":"derevnya","parent_id":"ad-root"}]`},
		{"/api/admin-divisions?kind=settlement&limit=1&offset=1", http.StatusOK,
			`[{"id":"ad-3","name":"Никифорово","type":"derevnya","parent_id":"ad-root"}]`},
		{"/api/admin-divisions?type=governorate", http.StatusOK,
			`[{"id":"ad-root","name":"Московская","type":"governorate","parent_id":null}]`},
		{"/api/admin-divisions?type=castle", http.StatusUnprocessableEntity,
			`{"error":"type: неизвестный тип единицы деления \"castle\"","field":"type"}`},
		{"/api/admin-divisions?limit=x", http.StatusBadRequest,
			`{"error":"параметр limit: ожидалось целое число, получено \"x\""}`},
		{"/api/settlements", http.StatusNotFound, ""},
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

- [ ] **Step 5: `mcp`**

Заменить содержимое `internal/mcp/deps.go`:

```go
package mcp

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// DivisionService — контракт сценария «список единиц административного деления».
type DivisionService interface {
	ListDivisions(ctx context.Context, q models.DivisionQuery) ([]models.AdministrativeDivision, error)
}
```

Заменить содержимое `internal/mcp/server.go`:

```go
package mcp

import (
	"github.com/mark3labs/mcp-go/server"
)

// NewServer создаёт MCP-сервер и регистрирует доступные тулы.
func NewServer(divisions DivisionService) *server.MCPServer {
	s := server.NewMCPServer(
		"genodex",
		"0.1.0",
	)

	registerDivisionTools(s, divisions)

	return s
}
```

Создать `internal/mcp/division.go`:

```go
package mcp

import (
	"context"
	"encoding/json"
	"fmt"

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
	)

	s.AddTool(tool, divisionListHandler(divisions))
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

		list, err := divisions.ListDivisions(ctx, q)
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
		Kind: models.DivisionKind(req.GetString("kind", "")),
		Type: models.AdminDivisionType(req.GetString("type", "")),
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

// optionalInt читает необязательный целочисленный аргумент.
func optionalInt(req mcp.CallToolRequest, name string) (int, error) {
	if _, ok := req.GetArguments()[name]; !ok {
		return 0, nil
	}

	n, err := req.RequireInt(name)
	if err != nil {
		return 0, fmt.Errorf("аргумент %s: ожидалось целое число", name)
	}

	return n, nil
}
```

Создать `internal/mcp/division_test.go`:

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
}

func (f *fakeDivisions) ListDivisions(_ context.Context, q models.DivisionQuery) ([]models.AdministrativeDivision, error) {
	f.got = q
	f.call++

	return f.list, f.err
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

	want := `[{"id":"ad-root","name":"Московская","type":"governorate","parent_id":null},` +
		`{"id":"ad-1","name":"Давыдово","type":"selo","parent_id":"ad-root"}]`
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

// TestNewServerRegistersDivisionListOnly: тул зарегистрирован под новым именем,
// прежнего settlement_list нет.
func TestNewServerRegistersDivisionListOnly(t *testing.T) {
	tools := NewServer(&fakeDivisions{}).ListTools()

	if _, ok := tools["division_list"]; !ok {
		t.Errorf("тул division_list не зарегистрирован: %v", tools)
	}

	if _, ok := tools["settlement_list"]; ok {
		t.Error("прежний тул settlement_list всё ещё зарегистрирован")
	}
}
```

- [ ] **Step 6: Рубеж**

Run: `gofmt -l . && go build ./... && go vet ./... && go test ./... -count=1`
Expected: gofmt пусто, сборка и `vet` без ошибок, все пакеты `ok`.

- [ ] **Step 7: Старых имён нет**

Run: `grep -rnE "settlement_list|/api/settlements|list_settlements|ListSettlements|SettlementService|transport\.Settlement|SettlementsFromModels|SettlementFromModel" --include=*.go internal cmd`
Expected: только тесты, которые проверяют отсутствие прежних имён: `internal/mcp/division_test.go` (`settlement_list` не зарегистрирован), `internal/httpapi/division_test.go` и `internal/httpapi/store_test.go` (`/api/settlements` → 404). Слово `settlement` в значениях `kind=settlement`, `DivisionKindSettlement`, `IsSettlement`, поле `Settlements` церкви/прихода/узла архива — не прежние имена контракта.

- [ ] **Step 8: Мутации (вручную; каждую откатить сразу: `git checkout -- <файл>`)**

Запускать `go test ./internal/... -count=1`; указанный тест должен ПРОВАЛИТЬСЯ.

| Мутация | Ожидаемый провал |
|---|---|
| в `ListDivisions` (`scenario.go`) заменить `if matched >= page.Offset {` на `if true {` (окно не сдвигается) | `TestListDivisionsWindowIsAppliedAfterFilter` |
| в `ListDivisions` убрать блок `if len(out) == page.Limit { return out, nil }` | `TestListDivisionsWalksAllRepositoryWindows` |
| в `(DivisionQuery).Validate` (`models/query.go`) удалить ветку `case !q.Kind.Valid():` (и её `return`) | `TestDivisionQueryValidate`, `TestListDivisionsInvalidQueryDoesNotTouchRepo` |
| в `httpapi.go` вернуть путь маршрута `"GET /api/admin-divisions"` → `"GET /api/settlements"` | `TestDivisionListContract`, `TestOldSettlementsRouteIsGone`, `TestAdminDivisionsWithRealStore` |
| в `writeError` (`httpapi.go`) убрать ветку `*models.ValidationError` (всё — 500) | `TestDivisionListValidationErrorIs422` |
| в `AdminDivisionFromModel` (`transport/admin_division.go`) вместо копии `p := *d.ParentID; parent = &p` взять `parent = d.ParentID` | `TestAdminDivisionFromModel` |
| в `optionalInt` (`mcp/division.go`) всегда возвращать `req.GetInt(name, 0), nil` | `TestDivisionListToolBadNumberIsToolError` |

- [ ] **Step 9: Commit**

```bash
git add -A internal
git commit -m "feat(api): division_list и /api/admin-divisions вместо settlement_list и /api/settlements"
```

---

### Task 3: Фронтенд на `/api/admin-divisions`

**Files:**
- Modify: `web/src/api.ts`, `web/src/App.tsx`

**Interfaces:**
- Consumes: Task 2 (контракт `/api/admin-divisions`).
- Produces: `AdminDivision`, `AdminDivisionQuery`, `MAX_PAGE_LIMIT`, `fetchAdminDivisions(query)`; вкладка «Населённые пункты» запрашивает `kind=settlement&limit=500` и показывает пояснение, если пришло ровно 500.

- [ ] **Step 1: `api.ts`**

Заменить в `web/src/api.ts` блок от `export interface Settlement {` до конца функции `fetchSettlements` (первые 12 строк файла; далее идут `DocFile`, `DocListResponse`, `fetchDocList`, `fetchDoc` — их не трогать) на:

```ts
export interface AdminDivision {
  id: string;
  name: string;
  type: string;
  parent_id: string | null;
}

export interface AdminDivisionQuery {
  kind?: "settlement";
  type?: string;
  limit?: number;
  offset?: number;
}

// Максимальный размер окна списка (совпадает с MaxPageLimit на сервере).
export const MAX_PAGE_LIMIT = 500;

export async function fetchAdminDivisions(
  query: AdminDivisionQuery = {},
): Promise<AdminDivision[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/admin-divisions${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}
```

- [ ] **Step 2: `App.tsx`**

Сохранить как `$TMPDIR/s13-web.py` (вне репозитория) и выполнить из корня: `python3 $TMPDIR/s13-web.py` (при несовпадении фрагмента скрипт останавливается с сообщением — не подгонять молча):

```python
"""S13, задача 3: фронтенд на новом URL. Запуск из корня репозитория."""


def edit(p, pairs):
    s = open(p, encoding="utf-8").read()
    for old, new in pairs:
        if old not in s:
            raise SystemExit(f"{p}: не найден фрагмент: {old[:60]!r}")
        s = s.replace(old, new, 1)
    open(p, "w", encoding="utf-8").write(s)


edit("web/src/App.tsx", [
    ('import { fetchSettlements, type Settlement } from "./api";',
     'import { fetchAdminDivisions, MAX_PAGE_LIMIT, type AdminDivision } from "./api";'),
    ("  const [settlements, setSettlements] = useState<Settlement[]>([]);",
     "  const [settlements, setSettlements] = useState<AdminDivision[]>([]);"),
    ("""    fetchSettlements()
      .then(setSettlements)""", """    fetchAdminDivisions({ kind: "settlement", limit: MAX_PAGE_LIMIT })
      .then(setSettlements)"""),
    ("""      {error != null && <Alert type="error" showIcon message={error} />}
      {!loading && error == null && (""", """      {error != null && <Alert type="error" showIcon message={error} />}
      {settlements.length >= MAX_PAGE_LIMIT && (
        <Alert
          type="info"
          showIcon
          message={`Показаны первые ${MAX_PAGE_LIMIT} населённых пунктов`}
        />
      )}
      {!loading && error == null && ("""),
])
```

- [ ] **Step 3: Проверка**

Run: `cd web && npm run typecheck && npm run build`
Expected: `tsc --noEmit` без ошибок, `vite build` — `✓ built in …` (предупреждение о размере чанка допустимо). `web/dist` в git не попадает (в `.gitignore`); не добавлять его в коммит.

- [ ] **Step 4: Старых имён нет**

Run: `grep -rn "fetchSettlements\|Settlement\b\|/api/settlements" web/src`
Expected: пусто.

- [ ] **Step 5: Commit**

```bash
git add web/src/api.ts web/src/App.tsx
git commit -m "feat(web): вкладка населённых пунктов на /api/admin-divisions"
```

---

### Task 4: Документация

**Files:**
- Modify: `README.md`, `AGENTS.md`, `docs/usage.md`, `docs/development.md`, `docs/architecture.md`, `docs/todo.md`, `docs/data-model/normalization-s1s2.md`, `docs/data-model/core-read-write.md` (§5), `docs/plans/2026-09-20-core-rw-roadmap.md` (S13, S15, S16)

Правки — инструментом Edit с точным старым текстом (сначала прочитать файл). При несовпадении старого текста остановиться и сообщить.

- [ ] **Step 1: `README.md`**

Заменить `| `/api/settlements` | List of settlements (JSON) |` на

```
| `/api/admin-divisions` | Administrative divisions (JSON array `[{"id","name","type","parent_id"}]`); query: `kind=settlement`, `type=<type>`, `limit` (default 50, max 500), `offset` |
```

и `curl -s http://localhost:9000/api/settlements` на `curl -s 'http://localhost:9000/api/admin-divisions?kind=settlement'`.

- [ ] **Step 2: `AGENTS.md`, `docs/development.md`, `docs/architecture.md`**

- `AGENTS.md`: в строке про smoke test заменить `/api/settlements` на `/api/admin-divisions`.
- `docs/development.md`: заменить `curl -s http://localhost:9000/api/settlements # [...]` на `curl -s 'http://localhost:9000/api/admin-divisions?kind=settlement' # [...]`.
- `docs/architecture.md`: заменить `(health, settlements, docs)` на `(health, admin-divisions, docs)`.

- [ ] **Step 3: `docs/usage.md`**

(а) Заменить строку таблицы эндпоинтов

```
| `/api/settlements` | Список населённых пунктов (JSON): массив `[{"id", "name", "type"}]`. |
```

на

```
| `/api/admin-divisions` | Единицы административного деления (JSON): массив `[{"id", "name", "type", "parent_id"}]`; параметры — ниже. |
```

(б) Заменить строку таблицы тулов

```
| `settlement_list` | Получить список всех населённых пунктов |
```

на

```
| `division_list` | Список единиц административного деления; аргументы `kind`, `type`, `limit`, `offset` (как параметры HTTP, см. ниже) |
```

(в) Заменить `curl -s http://localhost:9000/api/settlements` на

```
curl -s 'http://localhost:9000/api/admin-divisions?kind=settlement&limit=20'
```

(г) После блока `### HTTP API` (после его закрывающего ```` ``` ````) и перед `### Веб` добавить раздел:

```
### Список единиц деления

`GET /api/admin-divisions` и MCP-тул `division_list` принимают одни и те же параметры:

| Параметр | Значение |
|----------|----------|
| `kind` | пусто — без фильтра; `settlement` — только населённые пункты (город, село, деревня, хутор, погост, станица, местечко) |
| `type` | пусто — без фильтра; иначе точный тип: `governorate`, `district`, `volost`, `gorod`, `selo`, `derevnya`, `hutor`, `pogost`, `stanitsa`, `mestechko`, `other` |
| `limit` | размер окна: по умолчанию 50, не больше 500 (больше — сужается до 500) |
| `offset` | сдвиг окна, по умолчанию 0 |

`kind` и `type` пересекаются. Порядок — порядок сохранения; окно считается после
фильтра; результат короче `limit` — конец списка. `parent_id` — `null` у корневых
единиц. Ошибки HTTP: `400` — `limit`/`offset` не целое число; `422` — неизвестные
`kind`/`type` или отрицательное окно (тело `{"error": "…", "field": "kind"}`);
`500` — сбой хранилища. Ошибки MCP-тула приходят в результате вызова с признаком ошибки.
```

- [ ] **Step 4: `docs/todo.md`**

(а) Заменить

```
- [ ] MCP-тулы и `/api` под новые сущности (сейчас публичен только `settlement_list` /
      `/api/settlements`).
```

на

```
- [ ] MCP-тулы и `/api` под новые сущности (сейчас публичен только список делений:
      `division_list` / `/api/admin-divisions`, S13).
```

(б) Заменить `#17; публичный контракт `settlement_list` переименовывается на этапе S13);` на `#17; публичный контракт переименован в `division_list` на этапе S13);`

(в) Заменить в «Паспорте прохода» `smoke `/api/health`, `/api/settlements`, `/`.` на `smoke `/api/health`, `/api/admin-divisions`, `/`.`

(г) В разделе «Следующий проход» заменить `Выполнены S1–S12 (` на `Выполнены S1–S13 (` и `мелочи); осталось S13–S16 (контракты и срез на делениях).` на `мелочи, контракт делений на чтение); осталось S14–S16 (запись делений, контракты записи, поиск в контрактах).`

- [ ] **Step 5: `docs/data-model/normalization-s1s2.md`**

Шесть правок (состояние S1+S2 приведено к текущему коду; записи «до S13» оставлены как история):

(а) Заменить

```
  (пакет `entity` упраздняется целиком). Публичный контракт `settlement_list`
  пока сохраняет имя и отдаётся DTO `transport.Settlement{id, name, type}`;
  сценарий возвращает домен `AdministrativeDivision`. Переименование
  публичных контрактов — проход C и правка только в `transport`.
```

на

```
  (пакет `entity` упраздняется целиком). Публичный контракт (с S13 — MCP-тул
  `division_list` и `GET /api/admin-divisions`; «населённые пункты» — фильтр
  `kind=settlement`) отдаётся DTO `transport.AdminDivision{id, name, type, parent_id}`;
  сценарий возвращает домен `AdministrativeDivision`. Переименования контрактов
  правятся только в `transport` (так и сделано в S13).
```

(б) Заменить

```
  `GET /api/settlements` и `settlement_list` могут отдавать `{id, name, type}`
  под именем `transport.Settlement`, а домен растёт как `AdministrativeDivision`.
```

на

```
  контракт делений отдаёт `{id, name, type, parent_id}` под именем
  `transport.AdminDivision` (до S13 — `{id, name, type}` как `transport.Settlement`),
  а домен растёт как `AdministrativeDivision`.
```

(в) Заменить заголовок и абзац раздела 7:

```
## 7. usecase settlement_list (в составе S1+S2)

Единственный существующий сценарий: возвращает населённые пункты как
`[]models.AdministrativeDivision` (типы из вида нас. пунктов). Сценарий не знает
про JSON и про `transport`. Публичные имена `settlement_list` (MCP) и
`/api/settlements` (HTTP) пока сохраняются — см. раздел 8; переименование
контрактов — проход C.
```

на

```
## 7. usecase «список делений» (в составе S1+S2)

Единственный сценарий этого прохода (с S13 — `list_divisions`, ранее
`list_settlements`): возвращает единицы деления как
`[]models.AdministrativeDivision` (с фильтрами вида и типа и окном). Сценарий не
знает про JSON и про `transport`. Публичные имена — `division_list` (MCP) и
`/api/admin-divisions` (HTTP), см. раздел 8.
```

(г) В разделе 8 заменить

```
  `transport.Settlement{ID, Name, Type}` с тегами `id`, `name`, `type`
  (`Type` — `models.AdminDivisionType`, значение на проводе = домен).
- Конвертеры односторонние (домен → DTO): `SettlementFromModel(models.AdministrativeDivision)`
  и `SettlementsFromModels([]models.AdministrativeDivision)` (пустой список — `[]`,
```

на

```
  `transport.AdminDivision{ID, Name, Type, ParentID}` с тегами `id`, `name`, `type`,
  `parent_id` (`Type` — `models.AdminDivisionType`, значение на проводе = домен;
  `parent_id` — `null` у корня; до S13 — `Settlement{id, name, type}`).
- Конвертеры односторонние (домен → DTO): `AdminDivisionFromModel(models.AdministrativeDivision)`
  и `AdminDivisionsFromModels([]models.AdministrativeDivision)` (пустой список — `[]`,
```

а также

```
- Обработчики: `httpapi.handleSettlementList` и MCP-тул `settlement_list`
  вызывают сценарий, конвертируют результат через `transport` и сериализуют.
  Интерфейсы `SettlementService` в `httpapi/deps.go` и `mcp/deps.go` возвращают
  `[]models.AdministrativeDivision`.
- Форма ответа меняется с `[{id,name,metadata?}]` на `[{id,name,type}]`;
  `metadata` исчезает вместе с `Metadata` в домене.
```

на

```
- Обработчики: `httpapi.handleDivisionList` и MCP-тул `division_list`
  вызывают сценарий, конвертируют результат через `transport` и сериализуют.
  Интерфейсы `DivisionService` в `httpapi/deps.go` и `mcp/deps.go` возвращают
  `[]models.AdministrativeDivision`.
- Форма ответа: `[{id,name,type}]` (S1+S2; `metadata` исчезла вместе с `Metadata`
  в домене), с S13 — `[{id,name,type,parent_id}]`.
```

(д) В разделе 9 заменить `конвертер `Settlement` — поля,` на `конвертер `AdminDivision` — поля,` и `обработчики отдают форму `transport.Settlement`.` на `обработчики отдают форму `transport.AdminDivision`.`

(е) В разделе 10 заменить `smoke `/api/health`, `/api/settlements`, `/`.` на `smoke `/api/health`, `/api/admin-divisions`, `/`.`

- [ ] **Step 6: `docs/data-model/core-read-write.md` (§5)**

Заменить

```
- Переименование единственного текущего контракта: `settlement_list` →
  `division_list`, `GET /api/settlements` → `GET /api/admin-divisions`, сценарий
  `list_settlements` → `list_divisions`, DTO `Settlement` → `AdminDivision`
  (`id`, `name`, `type`, `parent_id`). Фильтр «только населённые пункты» становится
  параметром запроса (`kind=settlement`), а не именем контракта.
```

на

```
- Переименование единственного текущего контракта (выполнено в S13): `settlement_list` →
  `division_list`, `GET /api/settlements` → `GET /api/admin-divisions`, сценарий
  `list_settlements` → `list_divisions`, DTO `Settlement` → `AdminDivision`
  (`id`, `name`, `type`, `parent_id`). Фильтр «только населённые пункты» становится
  параметром запроса (`kind=settlement`), а не именем контракта. Параметры списка:
  `kind`, `type`, `limit`, `offset` (`models.DivisionQuery`); окно применяется после
  фильтра, сценарий фильтрует в памяти по окнам репозитория. Коды: `400` — параметр
  не число, `422` — `*ValidationError` (`{error, field}`), `500` — сбой; в MCP —
  ошибка внутри результата тула. Подробности — `docs/usage.md`.
```

- [ ] **Step 7: `docs/plans/2026-09-20-core-rw-roadmap.md`**

(а) Заменить блок S13 (от `### S13. Контракт делений (чтение)` до строки перед `### S14.`):

```
### S13. Контракт делений (чтение)
- **Файлы:** `usecases/list_divisions` (переименование пакета сценария),
  `transport/admin_division.go` (DTO `AdminDivision` с `parent_id`),
  `httpapi`/`mcp` (`division_list`, `GET /api/admin-divisions`, параметры
  `kind`, `type`, `limit`, `offset`), `web/src/api.ts`, `web/src/App.tsx`,
  `docs/usage.md`.
- **Приёмка:** строки JSON зафиксированы тестами; старых имён нет в коде и
  документации; вкладка фронтенда работает на новом URL.
- **Предпосылка из S9:** сценарий `list_settlements` (→ `list_divisions`)
  жёстко использует `AccessFull` (у делений нет флага приватности, поэтому
  безвредно); когда появятся публичные обработчики других сущностей, `Access`
  нужно пробрасывать из обработчика, а не выбирать в сценарии.
```

на

```
### S13. Контракт делений (чтение)
- **Файлы:** `models/query.go` (`DivisionQuery`, `DivisionKind`),
  `usecases/list_divisions` (переименование пакета сценария),
  `transport/admin_division.go` (DTO `AdminDivision` с `parent_id`),
  `httpapi`/`mcp` (`division_list`, `GET /api/admin-divisions`, параметры
  `kind`, `type`, `limit`, `offset`), `web/src/api.ts`, `web/src/App.tsx`,
  `docs/usage.md`; план — `2026-09-21-core-rw-s13-division-read.md`.
- **Приёмка:** строки JSON зафиксированы тестами; старых имён нет в коде и
  документации; вкладка фронтенда работает на новом URL.
- **Предпосылка из S9:** сценарий `list_divisions` жёстко использует `AccessFull`
  (у делений нет флага приватности, поэтому безвредно); когда появятся публичные
  обработчики других сущностей, `Access` нужно пробрасывать из обработчика, а не
  выбирать в сценарии.
```

(б) В блоке S15 после строки `  отсутствующей сущности — голый `ErrNotFound` без вида и id (вопрос S12).` добавить:

```
- **Предпосылка из S13:** `httpapi.writeError` уже отвечает `422` на
  `*models.ValidationError` (тело `{error, field}`) и `500` на остальное; S15
  добавляет `404` (`ErrNotFound`) и `409` (`*InUseError` со списком). Соглашение
  о кодах: `400` — синтаксически неверный параметр, `422` — неверное значение.
```

(в) В блоке S16 после строки `  выбираются по индексу; фронтенд показывает результаты поиска.` добавить:

```
- **Предпосылка из S13:** `models.DivisionQuery` получает поле `ParentID`;
  `list_divisions` фильтрует в памяти по окнам репозитория — для `parent_id`
  использовать `Store.ChildrenOfDivision` (индекс), а не фильтр в памяти.
```

- [ ] **Step 8: Проверка и commit**

Run: `git grep -nE "settlement_list|/api/settlements|list_settlements|ListSettlements|SettlementService|transport\.Settlement" -- . ':!docs/plans' ':!docs/superpowers/plans'`
Expected: только явные записи о переименовании: `docs/data-model/core-read-write.md` (§5), `docs/data-model/decisions.md` (#34), `docs/data-model/normalization-s1s2.md` (пометки «до S13»/«ранее»), `docs/todo.md` (история решений).

```bash
git add -A README.md AGENTS.md docs
git commit -m "docs: S13 — контракт делений, параметры, коды ответов"
```
