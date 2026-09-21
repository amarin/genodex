# S10: Пакетная загрузка — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `ListPeople` и `ListAdministrativeDivisions` выполняют число SQL-запросов, не зависящее от числа строк окна; результаты совпадают с прежней поштучной загрузкой.

**Architecture:** Новый файл `sqlstore/batch.go`: пакетные загрузчики (`queryGrouped` — запрос по `IN (…)` списку владельцев, нарезанный на куски по `inChunk = 500` значений, со сборкой строк по ключу; загрузчики `text_refs`, `dates`, списков TextRef, строк, переименований и `source_links`). `getPeople` и `getDivisions` читают сущности пачкой: главные строки, связные таблицы и value-строки — по одному запросу на таблицу. Одиночные `GetPerson`/`GetAdministrativeDivision` идут тем же путём (пачка из одного id), поэтому список и `Get` не могут разойтись. Общий `listByIDs` (id окна → `getMany`) обслуживает пакетные виды; прежний `listEntities` остаётся обёрткой поверх него для остальных 19 видов (поштучно, «по мере срезов»). Эталоном служит прежний поштучный код, перенесённый в тестовый файл `legacy_test.go`. Запросы считаются обёрткой над исполнителем `Store.exec`, без правок продакшн-кода.

**Tech Stack:** Go 1.26.4, `database/sql`, `modernc.org/sqlite`.

**Spec:** `docs/data-model/core-read-write.md` §4; `docs/plans/2026-09-20-core-rw-roadmap.md` (этап S10).

## Global Constraints

- Число запросов `ListPeople`/`ListAdministrativeDivisions` для окна в пределах одного куска IN (≤ 500 владельцев/значений) не зависит от числа строк; фиксируется точно: персоны — 10 запросов на окно (id окна + persons, person_names, text_refs, dates, 4 списка, source_links), деления — 9; `GetPerson` — 9.
- IN-списки режутся по `inChunk = 500` значений (предел SQLite на число параметров); значения не теряются и не дублируются на границах кусков.
- Результат идентичен прежней загрузке: тот же порядок списков (`position`), те же `nil` вместо пустых срезов (`reflect.DeepEqual` различает), те же указатели дат (`nil` — даты нет), `TargetType`/`TargetID` у доказательств восстанавливаются из владельца.
- Порядок сущностей в окне — порядок `ids` (rowid), отсутствующие пропускаются; `GetPerson`/`GetAdministrativeDivision` для отсутствующего id — `models.ErrNotFound`.
- Остальные 19 видов читаются как раньше (по одному); публичные подписи порта не меняются.
- Схема БД, `Validate`, контракты MCP/HTTP не меняются. Комментарии и тексты ошибок — на русском; код gofmt-clean.
- Проверка рубежа: `gofmt -l .` пусто, `go build ./...`, `go vet ./...`, `go test ./...`.

---

### Task 1: Пакетные загрузчики

**Files:**
- Create: `internal/store/sqlstore/batch.go`, `internal/store/sqlstore/batch_helpers_test.go`

**Interfaces:**
- Consumes: `queryer`, `scanRows` (`sqlstore.go`); модели `TextRef`, `FactDate`, `NamedPeriod`, `SourceLink`.
- Produces: `inChunk`, `placeholders(n)`, `forChunks(vals, fn)`, `anySlice`, `queryGrouped(q, keys, args, sqlFor, scan)`, `loadTextRefsByID`, `loadDatesByID`, `nonZero`, `loadTextRefListsBatch`, `loadStringListsBatch`, `loadRenamesBatch`, `loadSourceLinksBatch`, `idStrings`.

- [ ] **Step 1: Тест**

Создать `internal/store/sqlstore/batch_helpers_test.go`:

```go
package sqlstore

import (
	"reflect"
	"testing"
)

func TestPlaceholders(t *testing.T) {
	if got := placeholders(3); got != "?, ?, ?" {
		t.Fatalf("placeholders(3) = %q", got)
	}

	if got := placeholders(1); got != "?" {
		t.Fatalf("placeholders(1) = %q", got)
	}
}

func TestForChunksSplitsAtInChunk(t *testing.T) {
	vals := make([]int, 2*inChunk+200)
	for i := range vals {
		vals[i] = i
	}

	var sizes []int

	seen := 0

	if err := forChunks(vals, func(chunk []int) error {
		sizes = append(sizes, len(chunk))

		for _, v := range chunk {
			if v != seen {
				t.Fatalf("значение %d не на своём месте (ожидалось %d)", v, seen)
			}

			seen++
		}

		return nil
	}); err != nil {
		t.Fatalf("forChunks: %v", err)
	}

	if want := []int{inChunk, inChunk, 200}; !reflect.DeepEqual(sizes, want) {
		t.Fatalf("размеры кусков %v, ожидалось %v", sizes, want)
	}

	if seen != len(vals) {
		t.Fatalf("обработано %d значений из %d", seen, len(vals))
	}

	calls := 0
	_ = forChunks([]int(nil), func([]int) error { calls++; return nil })

	if calls != 0 {
		t.Fatalf("пустой список вызвал fn %d раз", calls)
	}
}

func TestNonZero(t *testing.T) {
	got := nonZero([]int64{0, 5, 0, 7})
	if want := []int64{5, 7}; !reflect.DeepEqual(got, want) {
		t.Fatalf("nonZero = %v, ожидалось %v", got, want)
	}
}
```

- [ ] **Step 2: Убедиться, что не компилируется**

Run: `go test ./internal/store/sqlstore/ -run "Placeholders|ForChunks|NonZero"`
Expected: FAIL — `undefined: placeholders`.

- [ ] **Step 3: Реализация**

Создать `internal/store/sqlstore/batch.go`:

```go
package sqlstore

import (
	"database/sql"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Пакетная загрузка: вместо запросов на каждую сущность списка — по одному
// запросу на связную таблицу с IN (...) по всем владельцам окна. Число
// запросов не зависит от числа строк (кроме нарезки IN по inChunk значений).

// inChunk — сколько значений подставляется в один IN (...): списки режутся на
// куски, чтобы не упереться в предел SQLite на число параметров запроса.
const inChunk = 500

// placeholders возвращает «?, ?, …» на n значений.
func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}

// forChunks вызывает fn по кускам не длиннее inChunk; пустой список — без вызовов.
func forChunks[T any](vals []T, fn func([]T) error) error {
	for start := 0; start < len(vals); start += inChunk {
		if err := fn(vals[start:min(start+inChunk, len(vals))]); err != nil {
			return err
		}
	}

	return nil
}

// anySlice превращает список значений в аргументы запроса.
func anySlice[T any](vals []T) []any {
	out := make([]any, len(vals))
	for i, v := range vals {
		out[i] = v
	}

	return out
}

// keyedRow — строка результата с ключом владельца.
type keyedRow[K comparable, V any] struct {
	key K
	val V
}

// queryGrouped выполняет запрос по IN-списку ключей (кусками) и раскладывает
// строки по ключу с сохранением порядка строк. args — параметры, стоящие в
// запросе до IN-списка; sqlFor получает готовые плейсхолдеры IN.
func queryGrouped[K comparable, V any](
	q queryer, keys []K, args []any, sqlFor func(in string) string, scan func(*sql.Rows) (K, V, error),
) (map[K][]V, error) {
	out := map[K][]V{}

	err := forChunks(keys, func(chunk []K) error {
		rows, err := scanRows(q, func(r *sql.Rows) (keyedRow[K, V], error) {
			k, v, err := scan(r)

			return keyedRow[K, V]{key: k, val: v}, err
		}, sqlFor(placeholders(len(chunk))), append(append([]any{}, args...), anySlice(chunk)...)...)
		if err != nil {
			return err
		}

		for _, kr := range rows {
			out[kr.key] = append(out[kr.key], kr.val)
		}

		return nil
	})

	return out, err
}

// loadTextRefsByID читает TextRef по id (нули пропускаются).
func loadTextRefsByID(q queryer, ids []int64) (map[int64]models.TextRef, error) {
	out := make(map[int64]models.TextRef, len(ids))

	grouped, err := queryGrouped(q, nonZero(ids), nil, func(in string) string {
		return `SELECT id, text, ref, ref_type FROM text_refs WHERE id IN (` + in + `)`
	}, func(r *sql.Rows) (int64, models.TextRef, error) {
		var (
			id           int64
			tr           models.TextRef
			ref, refType string
		)

		err := r.Scan(&id, &tr.Text, &ref, &refType)
		tr.Ref, tr.Type = models.ID(ref), models.Type(refType)

		return id, tr, err
	})
	if err != nil {
		return nil, err
	}

	for id, trs := range grouped {
		out[id] = trs[0]
	}

	return out, nil
}

// loadDatesByID читает FactDate по id (нули пропускаются).
func loadDatesByID(q queryer, ids []int64) (map[int64]*models.FactDate, error) {
	grouped, err := queryGrouped(q, nonZero(ids), nil, func(in string) string {
		return `SELECT id, year, month, day, precision, modifier, year_to, month_to, day_to, calendar
		        FROM dates WHERE id IN (` + in + `)`
	}, func(r *sql.Rows) (int64, *models.FactDate, error) {
		var (
			id             int64
			d              models.FactDate
			prec, mod, cal string
		)

		err := r.Scan(&id, &d.Year, &d.Month, &d.Day, &prec, &mod, &d.YearTo, &d.MonthTo, &d.DayTo, &cal)
		d.Precision, d.Modifier = models.FactPrecision(prec), models.FactModifier(mod)
		d.Calendar = models.FactCalendar(cal)

		return id, &d, err
	})
	if err != nil {
		return nil, err
	}

	out := make(map[int64]*models.FactDate, len(grouped))
	for id, ds := range grouped {
		out[id] = ds[0]
	}

	return out, nil
}

// nonZero отбирает ненулевые id (0 — «значения нет»).
func nonZero(ids []int64) []int64 {
	out := make([]int64, 0, len(ids))

	for _, id := range ids {
		if id != 0 {
			out = append(out, id)
		}
	}

	return out
}

// loadTextRefListsBatch читает списки TextRef связной таблицы для всех
// владельцев: владелец → список в порядке position.
func loadTextRefListsBatch(q queryer, table, ownerCol string, owners []string) (map[string][]models.TextRef, error) {
	return queryGrouped(q, owners, nil, func(in string) string {
		return `SELECT t.` + ownerCol + `, tr.text, tr.ref, tr.ref_type FROM ` + table + ` t
		        JOIN text_refs tr ON tr.id = t.text_ref_id
		        WHERE t.` + ownerCol + ` IN (` + in + `) ORDER BY t.position`
	}, func(r *sql.Rows) (string, models.TextRef, error) {
		var (
			owner        string
			tr           models.TextRef
			ref, refType string
		)

		err := r.Scan(&owner, &tr.Text, &ref, &refType)
		tr.Ref, tr.Type = models.ID(ref), models.Type(refType)

		return owner, tr, err
	})
}

// loadStringListsBatch читает списки строк для всех владельцев.
func loadStringListsBatch(q queryer, table, ownerCol string, owners []string) (map[string][]string, error) {
	return queryGrouped(q, owners, nil, func(in string) string {
		return `SELECT ` + ownerCol + `, value FROM ` + table +
			` WHERE ` + ownerCol + ` IN (` + in + `) ORDER BY position`
	}, func(r *sql.Rows) (string, string, error) {
		var owner, v string
		err := r.Scan(&owner, &v)

		return owner, v, err
	})
}

// loadRenamesBatch читает переименования для всех владельцев.
func loadRenamesBatch(q queryer, table, ownerCol string, owners []string) (map[string][]models.NamedPeriod, error) {
	return queryGrouped(q, owners, nil, func(in string) string {
		return `SELECT ` + ownerCol + `, text, since, until FROM ` + table +
			` WHERE ` + ownerCol + ` IN (` + in + `) ORDER BY position`
	}, func(r *sql.Rows) (string, models.NamedPeriod, error) {
		var (
			owner string
			rn    models.NamedPeriod
		)

		err := r.Scan(&owner, &rn.Text, &rn.Since, &rn.Until)

		return owner, rn, err
	})
}

// loadSourceLinksBatch читает доказательства сущностей вида t: владелец →
// доказательства в порядке записи; target_type/target_id восстанавливаются из
// владельца, как в loadSourceLinks.
func loadSourceLinksBatch(q queryer, t models.Type, owners []string) (map[string][]models.SourceLink, error) {
	return queryGrouped(q, owners, []any{string(t)}, func(in string) string {
		return `SELECT target_id, citation_id, reliability, role, note
		        FROM source_links WHERE target_type = ? AND target_id IN (` + in + `) ORDER BY id`
	}, func(r *sql.Rows) (string, models.SourceLink, error) {
		var (
			owner, citationID, reliability string
			sl                             models.SourceLink
		)

		err := r.Scan(&owner, &citationID, &reliability, &sl.Role, &sl.Note)
		sl.CitationID = models.ID(citationID)
		sl.TargetType, sl.TargetID = t, models.ID(owner)
		sl.Reliability = models.Reliability(reliability)

		return owner, sl, err
	})
}

// idStrings переводит идентификаторы сущностей в строки для параметров запроса.
func idStrings(ids []models.ID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = string(id)
	}

	return out
}
```

- [ ] **Step 4: Тесты проходят**

Run: `gofmt -l internal && go vet ./internal/store/... && go test ./internal/store/sqlstore/ -count=1`
Expected: gofmt пусто, `vet` чисто, `ok`.

- [ ] **Step 5: Commit**

```bash
git add internal/store/sqlstore/batch.go internal/store/sqlstore/batch_helpers_test.go
git commit -m "feat(store): пакетные загрузчики связных таблиц и значений"
```

---

### Task 2: `Person` и `AdministrativeDivision` читаются пачкой

**Files:**
- Modify: `internal/store/sqlstore/sqlstore.go`, `internal/store/sqlstore/person.go`, `internal/store/sqlstore/places.go`, `internal/store/sqlstore/helpers.go`

**Interfaces:**
- Consumes: Task 1; `listIDs`, `listEntities`, `personNameRow`, `personTextRefLists` (S9 и ранее).
- Produces: `listByIDs(ctx, s, table, access, page, getMany)`; `(*Store).getPeople(ctx, ids)`, `(*Store).getDivisions(ctx, ids)`; `divisionRow`; `GetPerson`/`GetAdministrativeDivision` поверх пачки из одного id; `listEntities` — обёртка над `listByIDs`. Удалено: поштучные `loadPersonName`, `loadRenames` (их копии — в тестовом эталоне Task 3).

Скрипт проверен на чистой копии `main` (после Task 1): после него сборка, `vet` и все существующие тесты (включая круговые `TestStoreRoundTrip*`) зелёные.

- [ ] **Step 1: Правки**

Сохранить как `$TMPDIR/s10-edit.py` (вне репозитория) и выполнить из корня: `python3 $TMPDIR/s10-edit.py` (при несовпадении маркера скрипт останавливается с сообщением — не подгонять молча, разобраться):

```python
"""S10, шаг задачи 2: Person и AdministrativeDivision читаются пачкой.
Запуск из корня репозитория."""


def read(p):
    return open(p, encoding="utf-8").read()


def write(p, s):
    open(p, "w", encoding="utf-8").write(s)


def cut(s, start_marker, end_marker, new, path):
    """Заменить фрагмент от start_marker (включительно) до end_marker (исключительно)."""
    a = s.find(start_marker)
    if a < 0:
        raise SystemExit(f"{path}: не найден маркер начала: {start_marker[:60]!r}")
    b = s.find(end_marker, a + len(start_marker))
    if b < 0:
        raise SystemExit(f"{path}: не найден маркер конца: {end_marker[:60]!r}")
    return s[:a] + new + s[b:]


# --- sqlstore.go: listEntities поверх listByIDs ---------------------------
p = "internal/store/sqlstore/sqlstore.go"
s = read(p)
end = "\treturn out, nil\n}\n"
a = s.find("// listEntities читает окно списка")
if a < 0:
    raise SystemExit(f"{p}: не найден listEntities")
b = s.find(end, a)
if b < 0:
    raise SystemExit(f"{p}: не найден конец listEntities")
b += len(end)
s = s[:a] + '''// listByIDs читает окно списка сущностей таблицы: сначала id окна (курсор
// закрыт), затем getMany по всем id сразу. Любой режим, кроме AccessFull,
// скрывает приватные строки таблиц, у которых есть колонка private. getMany
// возвращает найденные сущности в порядке ids (отсутствующие пропускает).
func listByIDs[T any](
	ctx context.Context, s *Store, table string, access models.Access, page models.Page,
	getMany func(context.Context, []models.ID) ([]*T, error),
) ([]*T, error) {
	g, err := s.graph(ctx)
	if err != nil {
		return nil, err
	}

	ids, err := listIDs(s.run(ctx), table, access != models.AccessFull && g.hasPrivate(table), page)
	if err != nil {
		return nil, err
	}

	return getMany(ctx, ids)
}

// listEntities — listByIDs для сущностей, читаемых по одной (Get на каждый id).
func listEntities[T any](
	ctx context.Context, s *Store, table string, access models.Access, page models.Page,
	get func(context.Context, models.ID) (*T, error),
) ([]*T, error) {
	return listByIDs(ctx, s, table, access, page, func(ctx context.Context, ids []models.ID) ([]*T, error) {
		out := make([]*T, 0, len(ids))

		for _, id := range ids {
			v, err := get(ctx, id)
			if errors.Is(err, models.ErrNotFound) {
				continue // строку удалили между чтением id и Get
			}

			if err != nil {
				return nil, err
			}

			out = append(out, v)
		}

		return out, nil
	})
}
''' + s[b:]
write(p, s)

# --- person.go -------------------------------------------------------------
p = "internal/store/sqlstore/person.go"
s = read(p)
s = cut(s, "// GetPerson читает персону по id", "// ListPeople возвращает", '''// GetPerson читает персону по id; не найдена — models.ErrNotFound.
func (s *Store) GetPerson(ctx context.Context, id models.ID) (*models.Person, error) {
	people, err := s.getPeople(ctx, []models.ID{id})
	if err != nil {
		return nil, err
	}

	if len(people) == 0 {
		return nil, models.ErrNotFound
	}

	return people[0], nil
}

// getPeople читает персон пачкой — по одному запросу на таблицу, а не на
// персону. Результат — найденные персоны в порядке ids; отсутствующие
// пропускаются. Одиночный GetPerson идёт тем же путём, поэтому список и Get
// не могут разойтись.
func (s *Store) getPeople(ctx context.Context, ids []models.ID) ([]*models.Person, error) {
	q := s.run(ctx)
	owners := idStrings(ids)

	main, err := queryGrouped(q, owners, nil, func(in string) string {
		return `SELECT id, gender, private FROM persons WHERE id IN (` + in + `)`
	}, func(r *sql.Rows) (string, *models.Person, error) {
		var (
			p      models.Person
			id     string
			gender string
		)

		err := r.Scan(&id, &gender, &p.Private)
		p.ID, p.Gender = models.ID(id), models.PersonGender(gender)

		return id, &p, err
	})
	if err != nil {
		return nil, err
	}

	names, err := queryGrouped(q, owners, nil, func(in string) string {
		return `SELECT person_id, type, surname_id, given_id, patronymic_id, prefix, suffix, since_id, until_id
		        FROM person_names WHERE person_id IN (` + in + `) ORDER BY id`
	}, func(r *sql.Rows) (string, personNameRow, error) {
		var (
			owner string
			nr    personNameRow
		)

		err := r.Scan(&owner, &nr.typ, &nr.surID, &nr.givID, &nr.patID,
			&nr.prefix, &nr.suffix, &nr.sinceID, &nr.untilID)

		return owner, nr, err
	})
	if err != nil {
		return nil, err
	}

	var textIDs, dateIDs []int64

	for _, rows := range names {
		for _, nr := range rows {
			textIDs = append(textIDs, nr.surID, nr.givID, nr.patID)
			dateIDs = append(dateIDs, int64From(nr.sinceID), int64From(nr.untilID))
		}
	}

	texts, err := loadTextRefsByID(q, textIDs)
	if err != nil {
		return nil, err
	}

	dates, err := loadDatesByID(q, dateIDs)
	if err != nil {
		return nil, err
	}

	sources, err := loadSourceLinksBatch(q, models.TypePerson, owners)
	if err != nil {
		return nil, err
	}

	lists := make([]map[string][]models.TextRef, len(personTextRefLists))

	for i, l := range personTextRefLists {
		if lists[i], err = loadTextRefListsBatch(q, l.table, "person_id", owners); err != nil {
			return nil, err
		}
	}

	out := make([]*models.Person, 0, len(ids))

	for _, id := range ids {
		found := main[string(id)]
		if len(found) == 0 {
			continue
		}

		p := found[0]

		for _, nr := range names[string(id)] {
			p.Names = append(p.Names, models.PersonName{
				Type:       models.PersonNameType(nr.typ),
				Surname:    texts[nr.surID],
				Given:      texts[nr.givID],
				Patronymic: texts[nr.patID],
				Prefix:     nr.prefix,
				Suffix:     nr.suffix,
				Since:      dates[int64From(nr.sinceID)],
				Until:      dates[int64From(nr.untilID)],
			})
		}

		for i, l := range personTextRefLists {
			l.set(p, lists[i][string(id)])
		}

		p.Sources = sources[string(id)]
		out = append(out, p)
	}

	return out, nil
}

''', p)
old = 'return listEntities(ctx, s, "persons", access, page, s.GetPerson)'
if old not in s:
    raise SystemExit(f"{p}: не найден вызов listEntities для persons")
s = s.replace(old, 'return listByIDs(ctx, s, "persons", access, page, s.getPeople)', 1)
write(p, s)

# --- places.go (AdministrativeDivision) ------------------------------------
p = "internal/store/sqlstore/places.go"
s = read(p)
s = cut(s, "// GetAdministrativeDivision читает", "// --- Church", '''// GetAdministrativeDivision читает единицу деления по id; нет — models.ErrNotFound.
func (s *Store) GetAdministrativeDivision(ctx context.Context, id models.ID) (*models.AdministrativeDivision, error) {
	divisions, err := s.getDivisions(ctx, []models.ID{id})
	if err != nil {
		return nil, err
	}

	if len(divisions) == 0 {
		return nil, models.ErrNotFound
	}

	return divisions[0], nil
}

// divisionRow — главная строка единицы деления до разрешения value-ссылок.
type divisionRow struct {
	div              models.AdministrativeDivision
	sinceID, untilID sql.NullInt64
}

// getDivisions читает единицы деления пачкой (см. getPeople): по одному
// запросу на таблицу, результат — найденные в порядке ids.
func (s *Store) getDivisions(ctx context.Context, ids []models.ID) ([]*models.AdministrativeDivision, error) {
	q := s.run(ctx)
	owners := idStrings(ids)

	main, err := queryGrouped(q, owners, nil, func(in string) string {
		return `SELECT id, name, type, parent_id, since_id, until_id
		        FROM administrative_divisions WHERE id IN (` + in + `)`
	}, func(r *sql.Rows) (string, divisionRow, error) {
		var (
			row         divisionRow
			id, divType string
			parentID    sql.NullString
		)

		err := r.Scan(&id, &row.div.Name, &divType, &parentID, &row.sinceID, &row.untilID)
		row.div.ID, row.div.Type = models.ID(id), models.AdminDivisionType(divType)
		row.div.ParentID = idPtrFrom(parentID)

		return id, row, err
	})
	if err != nil {
		return nil, err
	}

	var dateIDs []int64

	for _, rows := range main {
		dateIDs = append(dateIDs, int64From(rows[0].sinceID), int64From(rows[0].untilID))
	}

	dates, err := loadDatesByID(q, dateIDs)
	if err != nil {
		return nil, err
	}

	textLists := []struct {
		table string
		set   func(*models.AdministrativeDivision, []models.TextRef)
	}{
		{"ad_items", func(a *models.AdministrativeDivision, v []models.TextRef) { a.Items = v }},
		{"ad_successors", func(a *models.AdministrativeDivision, v []models.TextRef) { a.Successors = v }},
		{"ad_notes", func(a *models.AdministrativeDivision, v []models.TextRef) { a.Notes = v }},
	}
	lists := make([]map[string][]models.TextRef, len(textLists))

	for i, l := range textLists {
		if lists[i], err = loadTextRefListsBatch(q, l.table, "ad_id", owners); err != nil {
			return nil, err
		}
	}

	variants, err := loadStringListsBatch(q, "ad_variants", "ad_id", owners)
	if err != nil {
		return nil, err
	}

	renames, err := loadRenamesBatch(q, "ad_renames", "ad_id", owners)
	if err != nil {
		return nil, err
	}

	sources, err := loadSourceLinksBatch(q, models.TypeAdministrativeDivision, owners)
	if err != nil {
		return nil, err
	}

	out := make([]*models.AdministrativeDivision, 0, len(ids))

	for _, id := range ids {
		found := main[string(id)]
		if len(found) == 0 {
			continue
		}

		row := found[0]
		a := row.div
		a.Since, a.Until = dates[int64From(row.sinceID)], dates[int64From(row.untilID)]

		for i, l := range textLists {
			l.set(&a, lists[i][string(id)])
		}

		a.Variants = variants[string(id)]
		a.Renames = renames[string(id)]
		a.Sources = sources[string(id)]
		out = append(out, &a)
	}

	return out, nil
}

// ListAdministrativeDivisions возвращает окно списка единиц деления.
func (s *Store) ListAdministrativeDivisions(ctx context.Context, access models.Access, page models.Page) ([]*models.AdministrativeDivision, error) {
	return listByIDs(ctx, s, "administrative_divisions", access, page, s.getDivisions)
}

''', p)
write(p, s)

# --- helpers.go: поштучный loadRenames больше не нужен ----------------------
p = "internal/store/sqlstore/helpers.go"
s = read(p)
s = cut(s, "// loadRenames читает переименования", "// --- поисковый индекс", "", p)
write(p, s)
```

Затем `gofmt -w internal/store/sqlstore/places.go`.

- [ ] **Step 2: Рубеж**

Run: `gofmt -l . && go build ./... && go vet ./... && go test ./... -count=1`
Expected: gofmt пусто, сборка и `vet` без ошибок, все пакеты `ok`. Если `go build` жалуется на `web/embed.go` (`pattern all:dist`) — соберите `go build ./internal/... ./cmd/...`.

- [ ] **Step 3: Commit**

```bash
git add -A internal
git commit -m "feat(store): Person и AdministrativeDivision читаются пачкой"
```

---

### Task 3: Эталон и тесты пакетной загрузки

**Files:**
- Create: `internal/store/sqlstore/legacy_test.go`, `internal/store/sqlstore/batch_test.go`

**Interfaces:**
- Consumes: Task 2; `personNameRow`, `loadTextRef`, `loadDate`, `loadTextRefList`, `loadStringList`, `loadSourceLinks`, `scanRows` (продакшн-помощники, оставшиеся для других видов); `newStore`, `mustDo`, `link`, `seedSource`, `withTimeout`, `listers` (тестовые помощники прежних этапов).
- Produces: эталон `legacyGetPerson`, `legacyGetDivision`, `legacyPersonName`, `legacyLoadRenames`; `countingExec`, `counted`; фикстуры `batchPerson`, `batchDivision`, `seedBatch`; тесты `TestBatchListMatchesPerItemLoad`, `TestBatchListAcrossChunkBoundaries`, `TestListQueryCountDoesNotDependOnRowCount`.

- [ ] **Step 1: Создать файлы**

Создать `internal/store/sqlstore/legacy_test.go`:

```go
package sqlstore

import (
	"context"
	"database/sql"

	"github.com/amarin/genodex/internal/models"
)

// Эталон для проверки пакетной загрузки: прежние поштучные загрузчики Person и
// AdministrativeDivision (по запросу на каждое значение и каждую связную
// таблицу), сохранённые как есть. Пакетный путь обязан давать те же значения.

// legacyGetPerson читает персону по id поштучно.
func legacyGetPerson(s *Store, ctx context.Context, id models.ID) (*models.Person, error) {
	var (
		p      models.Person
		rawID  string
		gender string
	)

	err := s.run(ctx).QueryRow(`SELECT id, gender, private FROM persons WHERE id = ?`, string(id)).
		Scan(&rawID, &gender, &p.Private)
	if notFound(err) {
		return nil, models.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	p.ID, p.Gender = models.ID(rawID), models.PersonGender(gender)

	nameRows, err := scanRows(s.run(ctx), func(r *sql.Rows) (personNameRow, error) {
		var nr personNameRow
		err := r.Scan(&nr.typ, &nr.surID, &nr.givID, &nr.patID,
			&nr.prefix, &nr.suffix, &nr.sinceID, &nr.untilID)

		return nr, err
	},
		`SELECT type, surname_id, given_id, patronymic_id, prefix, suffix, since_id, until_id
		 FROM person_names WHERE person_id = ? ORDER BY id`, string(id))
	if err != nil {
		return nil, err
	}

	for _, nr := range nameRows {
		n, err := legacyPersonName(s.run(ctx), nr)
		if err != nil {
			return nil, err
		}

		p.Names = append(p.Names, n)
	}

	for _, l := range personTextRefLists {
		items, err := loadTextRefList(s.run(ctx), l.table, "person_id", string(id))
		if err != nil {
			return nil, err
		}

		l.set(&p, items)
	}

	if p.Sources, err = loadSourceLinks(s.run(ctx), models.TypePerson, id); err != nil {
		return nil, err
	}

	return &p, nil
}

// legacyPersonName разрешает value-ссылки сырой строки person_names.
func legacyPersonName(q queryer, nr personNameRow) (models.PersonName, error) {
	n := models.PersonName{
		Type:   models.PersonNameType(nr.typ),
		Prefix: nr.prefix,
		Suffix: nr.suffix,
	}

	var err error
	if n.Surname, err = loadTextRef(q, nr.surID); err != nil {
		return n, err
	}

	if n.Given, err = loadTextRef(q, nr.givID); err != nil {
		return n, err
	}

	if n.Patronymic, err = loadTextRef(q, nr.patID); err != nil {
		return n, err
	}

	if n.Since, err = loadDate(q, int64From(nr.sinceID)); err != nil {
		return n, err
	}

	if n.Until, err = loadDate(q, int64From(nr.untilID)); err != nil {
		return n, err
	}

	return n, nil
}

// legacyLoadRenames читает переименования поштучно.
func legacyLoadRenames(q queryer, table, ownerCol, ownerID string) ([]models.NamedPeriod, error) {
	return scanRows(q, func(r *sql.Rows) (models.NamedPeriod, error) {
		var rn models.NamedPeriod
		err := r.Scan(&rn.Text, &rn.Since, &rn.Until)

		return rn, err
	}, `SELECT text, since, until FROM `+table+` WHERE `+ownerCol+` = ? ORDER BY position`, ownerID)
}

// legacyGetDivision читает единицу деления по id поштучно.
func legacyGetDivision(s *Store, ctx context.Context, id models.ID) (*models.AdministrativeDivision, error) {
	var (
		a                models.AdministrativeDivision
		rawID, divType   string
		parentID         sql.NullString
		sinceID, untilID sql.NullInt64
	)

	err := s.run(ctx).QueryRow(
		`SELECT id, name, type, parent_id, since_id, until_id
		 FROM administrative_divisions WHERE id = ?`, string(id),
	).Scan(&rawID, &a.Name, &divType, &parentID, &sinceID, &untilID)
	if notFound(err) {
		return nil, models.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	a.ID, a.Type = models.ID(rawID), models.AdminDivisionType(divType)
	a.ParentID = idPtrFrom(parentID)

	if a.Since, err = loadDate(s.run(ctx), int64From(sinceID)); err != nil {
		return nil, err
	}

	if a.Until, err = loadDate(s.run(ctx), int64From(untilID)); err != nil {
		return nil, err
	}

	if a.Items, err = loadTextRefList(s.run(ctx), "ad_items", "ad_id", string(id)); err != nil {
		return nil, err
	}

	if a.Variants, err = loadStringList(s.run(ctx), "ad_variants", "ad_id", string(id)); err != nil {
		return nil, err
	}

	if a.Renames, err = legacyLoadRenames(s.run(ctx), "ad_renames", "ad_id", string(id)); err != nil {
		return nil, err
	}

	if a.Successors, err = loadTextRefList(s.run(ctx), "ad_successors", "ad_id", string(id)); err != nil {
		return nil, err
	}

	if a.Notes, err = loadTextRefList(s.run(ctx), "ad_notes", "ad_id", string(id)); err != nil {
		return nil, err
	}

	if a.Sources, err = loadSourceLinks(s.run(ctx), models.TypeAdministrativeDivision, id); err != nil {
		return nil, err
	}

	return &a, nil
}
```

Создать `internal/store/sqlstore/batch_test.go`:

```go
package sqlstore

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// countingExec считает запросы, проходящие через исполнитель Store.
type countingExec struct {
	x sqlExecutor
	n atomic.Int64
}

func (c *countingExec) ExecContext(ctx context.Context, q string, args ...any) (sql.Result, error) {
	c.n.Add(1)

	return c.x.ExecContext(ctx, q, args...)
}

func (c *countingExec) QueryContext(ctx context.Context, q string, args ...any) (*sql.Rows, error) {
	c.n.Add(1)

	return c.x.QueryContext(ctx, q, args...)
}

func (c *countingExec) QueryRowContext(ctx context.Context, q string, args ...any) *sql.Row {
	c.n.Add(1)

	return c.x.QueryRowContext(ctx, q, args...)
}

// counted возвращает копию Store, считающую запросы, и счётчик. Кэш графа схемы
// прогревается заранее, чтобы его загрузка не попала в подсчёт.
func counted(t *testing.T, s *Store) (*Store, *countingExec) {
	t.Helper()

	if _, err := s.graph(t.Context()); err != nil {
		t.Fatalf("graph: %v", err)
	}

	c := &countingExec{x: s.exec}

	return &Store{st: s.st, db: s.db, exec: c, cache: s.cache}, c
}

// batchDate — дата для фикстур: разные i дают разные значения.
func batchDate(i int) *models.FactDate {
	return &models.FactDate{
		Year: 1800 + i%100, Month: 1 + i%12, Day: 1 + i%28,
		Precision: models.PrecisionDay, Modifier: models.ModifierExact,
	}
}

// batchPerson строит персону: full — все поля заполнены; иначе по i получается
// минимальная, с одним именем без дат или без имён (нули и пустые списки —
// отдельные случаи пакетной сборки).
func batchPerson(i int, full bool) *models.Person {
	id := models.ID(fmt.Sprintf("p-%04d", i))
	p := &models.Person{ID: id, Gender: models.Male, Private: i%4 == 0}

	switch {
	case full || i%3 == 0:
		p.Names = []models.PersonName{
			{Type: models.PersonNameMain, Surname: models.TextRef{Text: "Иванов", Ref: "sur-1", Type: models.TypeSurname},
				Given: models.TextRef{Text: "Пётр"}, Patronymic: models.TextRef{Text: "Сергеевич"},
				Prefix: "фон", Suffix: "ст.", Since: batchDate(i), Until: batchDate(i + 1)},
			{Type: models.PersonNameBirth, Surname: models.TextRef{Text: "Петров"}, Given: models.TextRef{Text: "Пётр"}},
		}
		p.Estates = []models.TextRef{{Text: "крестьянин"}, {Text: "мещанин"}}
		p.Titles = []models.TextRef{{Text: "унтер-офицер"}}
		p.Nicknames = []models.TextRef{{Text: "Петруха"}}
		p.Notes = []models.TextRef{{Text: "из ревизии"}, {Text: "запись " + string(id)}}
		p.Sources = link(models.TypePerson, id)
	case i%3 == 2:
		p.Names = []models.PersonName{{Surname: models.TextRef{Text: "Сидоров"}, Given: models.TextRef{Text: "Иван"}}}
		p.Notes = []models.TextRef{{Text: "только заметка"}}
	}

	return p
}

// batchDivision строит единицу деления (см. batchPerson).
func batchDivision(i int, full bool, parent *models.ID) *models.AdministrativeDivision {
	a := &models.AdministrativeDivision{
		ID: models.ID(fmt.Sprintf("ad-%04d", i)), Name: fmt.Sprintf("Деревня %d", i),
		Type: models.AdminDivisionDerevnya, ParentID: parent,
	}

	if full || i%3 == 0 {
		a.Items = []models.TextRef{{Text: "двор Ивановых"}, {Text: "двор Петровых"}}
		a.Variants = []string{"Давыдовка", "Давыдово-Никольское"}
		a.Renames = []models.NamedPeriod{{Text: "Давыдовка", Since: "1800", Until: "1850"}, {Text: "Давыдово", Since: "1850"}}
		a.Successors = []models.TextRef{{Text: "Давыдово (совр.)"}}
		a.Since, a.Until = batchDate(i), batchDate(i+5)
		a.Notes = []models.TextRef{{Text: "упомянуто"}}
		a.Sources = link(models.TypeAdministrativeDivision, a.ID)
	} else if i%3 == 2 {
		a.Notes = []models.TextRef{{Text: "только заметка"}}
	}

	return a
}

// seedBatch сохраняет n персон и n единиц деления (одной транзакцией).
func seedBatch(t *testing.T, s *Store, n int, full bool) {
	t.Helper()

	ctx := t.Context()
	seedSource(t, s)
	mustDo(t, "citation", s.SaveCitation(ctx, &models.Citation{ID: "cit-1", SourceID: "src-1"}))

	mustDo(t, "seed", s.InTx(ctx, func(tx store.Store) error {
		root := models.ID("ad-root")
		if err := tx.SaveAdministrativeDivision(ctx, &models.AdministrativeDivision{
			ID: root, Name: "Московская", Type: models.AdminDivisionGovernorate,
		}); err != nil {
			return err
		}

		for i := 0; i < n; i++ {
			if err := tx.SavePerson(ctx, batchPerson(i, full)); err != nil {
				return err
			}

			if err := tx.SaveAdministrativeDivision(ctx, batchDivision(i, full, &root)); err != nil {
				return err
			}
		}

		return nil
	}))
}

// TestBatchListMatchesPerItemLoad: пакетный список совпадает с прежней
// поштучной загрузкой каждой сущности (включая nil против пустых срезов) при
// обоих режимах доступа.
func TestBatchListMatchesPerItemLoad(t *testing.T) {
	s := newStore(t)
	ctx := t.Context()

	seedBatch(t, s, 40, false)

	for _, access := range []models.Access{models.AccessFull, models.AccessPublic} {
		people, err := s.ListPeople(ctx, access, models.Page{})
		mustDo(t, "list people", err)

		if len(people) == 0 {
			t.Fatal("ListPeople вернул пустой список")
		}

		for _, p := range people {
			want, err := legacyGetPerson(s, ctx, p.ID)
			mustDo(t, "legacy person "+string(p.ID), err)

			if !reflect.DeepEqual(p, want) {
				t.Fatalf("персона %s\n пакетно  %+v\n поштучно %+v", p.ID, p, want)
			}
		}

		divisions, err := s.ListAdministrativeDivisions(ctx, access, models.Page{Limit: 100})
		mustDo(t, "list divisions", err)

		for _, d := range divisions {
			want, err := legacyGetDivision(s, ctx, d.ID)
			mustDo(t, "legacy division "+string(d.ID), err)

			if !reflect.DeepEqual(d, want) {
				t.Fatalf("деление %s\n пакетно  %+v\n поштучно %+v", d.ID, d, want)
			}
		}
	}

	// приватные персоны (i%4 == 0) скрыты в публичном режиме, деления — нет
	public, err := s.ListPeople(ctx, models.AccessPublic, models.Page{Limit: 100})
	mustDo(t, "public people", err)

	if len(public) != 30 {
		t.Fatalf("публичных персон %d, ожидалось 30", len(public))
	}
}

// TestBatchListAcrossChunkBoundaries: окно из 500 персон разбивает IN-списки
// значений на несколько кусков (1500 text_refs), и ни одно значение не теряется.
func TestBatchListAcrossChunkBoundaries(t *testing.T) {
	s := newStore(t)
	ctx := withTimeout(t)

	seedBatch(t, s, inChunk+20, true)

	first, err := s.ListPeople(ctx, models.AccessFull, models.Page{Limit: models.MaxPageLimit})
	mustDo(t, "first window", err)

	if len(first) != models.MaxPageLimit {
		t.Fatalf("первое окно %d персон, ожидалось %d", len(first), models.MaxPageLimit)
	}

	for _, p := range first {
		want, err := legacyGetPerson(s, ctx, p.ID)
		mustDo(t, "legacy "+string(p.ID), err)

		if !reflect.DeepEqual(p, want) {
			t.Fatalf("персона %s расходится с поштучной загрузкой:\n пакетно  %+v\n поштучно %+v", p.ID, p, want)
		}
	}

	tail, err := s.ListPeople(ctx, models.AccessFull, models.Page{Limit: models.MaxPageLimit, Offset: models.MaxPageLimit})
	mustDo(t, "tail", err)

	if len(tail) != 20 {
		t.Fatalf("хвост %d персон, ожидалось 20", len(tail))
	}

	divisions, err := s.ListAdministrativeDivisions(ctx, models.AccessFull, models.Page{Limit: models.MaxPageLimit})
	mustDo(t, "divisions", err)

	for _, d := range divisions {
		want, err := legacyGetDivision(s, ctx, d.ID)
		mustDo(t, "legacy "+string(d.ID), err)

		if !reflect.DeepEqual(d, want) {
			t.Fatalf("деление %s расходится с поштучной загрузкой:\n пакетно  %+v\n поштучно %+v", d.ID, d, want)
		}
	}
}

// TestListQueryCountDoesNotDependOnRowCount: число SQL-запросов ListPeople и
// ListAdministrativeDivisions одно и то же для 3 и 30 полностью заполненных
// строк (в пределах одного куска IN) и равно известной константе.
func TestListQueryCountDoesNotDependOnRowCount(t *testing.T) {
	const (
		// id окна + persons, person_names, text_refs, dates, 4 списка, source_links
		peopleQueries = 10
		// id окна + деления, dates, 3 списка, variants, renames, source_links
		divisionQueries = 9
		// то же без запроса id окна
		getPersonQueries = peopleQueries - 1
	)

	measure := func(n int) (people, divisions int64) {
		s := newStore(t)
		seedBatch(t, s, n, true)

		c, counter := counted(t, s)

		got, err := c.ListPeople(t.Context(), models.AccessFull, models.Page{Limit: models.MaxPageLimit})
		mustDo(t, "list people", err)

		if len(got) != n {
			t.Fatalf("ListPeople вернул %d персон, ожидалось %d", len(got), n)
		}

		people = counter.n.Swap(0)

		gotDivs, err := c.ListAdministrativeDivisions(t.Context(), models.AccessFull, models.Page{Limit: models.MaxPageLimit})
		mustDo(t, "list divisions", err)

		if len(gotDivs) != n+1 { // и корень
			t.Fatalf("ListAdministrativeDivisions вернул %d, ожидалось %d", len(gotDivs), n+1)
		}

		return people, counter.n.Swap(0)
	}

	smallPeople, smallDivs := measure(3)
	bigPeople, bigDivs := measure(30)

	if smallPeople != bigPeople || smallDivs != bigDivs {
		t.Fatalf("число запросов зависит от числа строк: персоны %d→%d, деления %d→%d",
			smallPeople, bigPeople, smallDivs, bigDivs)
	}

	if bigPeople != peopleQueries || bigDivs != divisionQueries {
		t.Fatalf("число запросов изменилось: персоны %d (было %d), деления %d (было %d); "+
			"если набор запросов изменён осознанно — поправьте константы", bigPeople, peopleQueries, bigDivs, divisionQueries)
	}

	// одиночное чтение идёт тем же пакетным путём
	s := newStore(t)
	seedBatch(t, s, 3, true)

	c, counter := counted(t, s)
	if _, err := c.GetPerson(t.Context(), "p-0001"); err != nil {
		t.Fatalf("GetPerson: %v", err)
	}

	if got := counter.n.Load(); got != getPersonQueries {
		t.Fatalf("GetPerson выполнил %d запросов, ожидалось %d", got, getPersonQueries)
	}
}
```

Выполнить `gofmt -w` на обоих.

- [ ] **Step 2: Прогнать**

Run: `gofmt -l . && go vet ./... && go test ./... -count=1`
Expected: gofmt пусто, `vet` чисто, все пакеты `ok`.

- [ ] **Step 3: Мутационные проверки (вручную; каждую откатить сразу: `git checkout -- <файл>`)**

Запускать `go test ./internal/store/sqlstore/ -count=1 -timeout 100s`; указанный тест должен ПРОВАЛИТЬСЯ.

| Мутация | Ожидаемый провал |
|---|---|
| в `forChunks` (`batch.go`) `min(start+inChunk, len(vals))` → `min(start+inChunk-1, len(vals))` | `TestForChunksSplitsAtInChunk`, `TestBatchListAcrossChunkBoundaries` |
| в `loadTextRefListsBatch` `ORDER BY t.position` → `ORDER BY t.position DESC` | `TestBatchListMatchesPerItemLoad`, `TestBatchListAcrossChunkBoundaries` |
| в `getPeople` (`person.go`) `Since: dates[int64From(nr.sinceID)]` → `dates[int64From(nr.untilID)]` | `TestBatchListMatchesPerItemLoad`, `TestStoreRoundTripPerson` |
| в `getPeople` `Surname: texts[nr.surID]` → `texts[nr.givID]` | `TestBatchListMatchesPerItemLoad`, `TestStoreRoundTripPerson` |
| в `getPeople` заменить `texts, err := loadTextRefsByID(q, textIDs)` на цикл `loadTextRef(q, id)` по каждому id (N+1) | `TestListQueryCountDoesNotDependOnRowCount` |

- [ ] **Step 4: Commit**

```bash
git add internal/store/sqlstore/legacy_test.go internal/store/sqlstore/batch_test.go
git commit -m "test(store): пакетная загрузка совпадает с поштучной, число запросов постоянно"
```

---

### Task 4: Документация

**Files:**
- Modify: `docs/data-model/core-read-write.md` (§4), `docs/plans/2026-09-20-core-rw-roadmap.md` (S10)

- [ ] **Step 1: Спецификация §4**

В `docs/data-model/core-read-write.md` заменить пункт

```
- N+1 в `List*` убирается пакетной загрузкой дочерних таблиц (`IN (…)`) сначала
  для `Person` и `AdministrativeDivision`, остальные — по мере срезов.
```

на

```
- N+1 в `List*` убирается пакетной загрузкой дочерних таблиц (`IN (…)`) сначала
  для `Person` и `AdministrativeDivision`, остальные — по мере срезов.
  Реализовано в S10: `getPeople`/`getDivisions` читают окно по одному запросу на
  таблицу (`IN` режется по 500 значений), одиночный `Get*` идёт тем же путём;
  остальные 19 видов пока читаются поштучно (`listEntities`).
```

- [ ] **Step 2: Дорожная карта**

В `docs/plans/2026-09-20-core-rw-roadmap.md` в блоке `### S10.` заменить строку

```
- **Файлы:** `sqlstore/helpers.go`, `person.go`, `places.go`, тесты.
```

на

```
- **Файлы:** `sqlstore/batch.go` (пакетные загрузчики), `person.go`, `places.go`,
  `sqlstore.go` (`listByIDs`), тесты (`batch_test.go`, `legacy_test.go` —
  эталон поштучной загрузки); план — `2026-09-21-core-rw-s10-batch.md`.
```

- [ ] **Step 3: Commit**

```bash
git add docs/data-model/core-read-write.md docs/plans/2026-09-20-core-rw-roadmap.md
git commit -m "docs: S10 — пакетная загрузка"
```

---

## Правки по итогам ревью (внесены после выполнения задач)

Код в репозитории — источник истины; плановые блоки выше описывают первую версию. Отличия:

- `batch.go`: в комментариях `queryGrouped` (ключи должны быть различны) и `loadTextRefsByID` (id, которого нет в таблице, даёт нулевое значение; внешние ключи делают его недостижимым) добавлены предусловия.
- `batch_test.go`: `batchDate` задаёт диапазоны (`between`, `YearTo/MonthTo/DayTo`) и юлианский календарь; у одного из имён только `Since`; `TestBatchListAcrossChunkBoundaries` работает с `t.Context()` (тысячи эталонных запросов не укладывались в 10 с под `-race`), сверяет с эталоном хвост окна персон и оба окна делений; комментарий о числе кусков исправлен (3000 id text_refs — 6 кусков, 1000 id dates — 2 куска; списки по владельцам — ровно один кусок в 500).
