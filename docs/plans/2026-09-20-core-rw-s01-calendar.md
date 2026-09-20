# S1: Календарь в `FactDate` — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Сделать календарь реальным полем `FactDate`: разбор и вывод суффиксов `ст. ст.`/`н. ст.`, сравнение юлианской и григорианской дат через юлианский день, запись и чтение календаря в БД.

**Architecture:** `FactDate` получает поле `Calendar FactCalendar` (пусто ≡ `unknown`). `Compare` при паре «юлианская — григорианская» переводит границы юлианской даты в григорианские (юлианский день, чистая арифметика, без зависимостей); во всех остальных случаях (`unknown`, одинаковые календари) — сравнение «как есть». `sqlstore` пишет и читает колонку `dates.calendar` как есть.

**Tech Stack:** Go 1.26.4, стандартная библиотека; SQLite через `modernc.org/sqlite` (уже используется).

**Spec:** `docs/data-model/core-read-write.md` §2.1; `docs/plans/2026-09-20-core-rw-roadmap.md` (этап S1).

## Global Constraints

- `models` не содержит JSON-тегов и не импортирует `encoding/json`.
- Пустой `Calendar` ≡ `unknown`; при разборе строки без суффикса `Calendar` остаётся пустым (существующие тесты разбора не меняются).
- Сравнение при `unknown`/пустом календаре хотя бы у одной из дат и при одинаковых календарях — «как есть» (порядковые номера компонентов).
- Проверка: `1917-10-25 ст. ст.` и `1917-11-07 н. ст.` — один и тот же день.
- Комментарии в коде — на русском; gofmt-clean; один файл — один основной тип.
- Проверка рубежа: `gofmt -l .` пусто, `go build ./...`, `go vet ./...`, `go test ./...`.

---

### Task 1: Поле `Calendar`, разбор и вывод суффиксов

**Files:**
- Modify: `internal/models/fact_date.go`
- Test: `internal/models/fact_date_test.go`

**Interfaces:**
- Consumes: `FactCalendar` и константы `FactCalendarGregorian`/`FactCalendarJulian`/`FactCalendarUnknown` (`internal/models/fact_calendar.go`, уже есть).
- Produces: `FactDate.Calendar FactCalendar`; `ParseFactDate` понимает суффиксы; `FactDate.String()` выводит суффикс для юлианского и григорианского календаря.

- [ ] **Step 1: Написать падающие тесты**

Добавь в конец `internal/models/fact_date_test.go`:

```go
func TestParseFactDateCalendar(t *testing.T) {
	tests := []struct {
		in   string
		want FactDate
	}{
		{"1917-10-25 ст. ст.", FactDate{Year: 1917, Month: 10, Day: 25, Precision: PrecisionDay, Modifier: ModifierExact, Calendar: FactCalendarJulian}},
		{"1917-10-25 ст.ст.", FactDate{Year: 1917, Month: 10, Day: 25, Precision: PrecisionDay, Modifier: ModifierExact, Calendar: FactCalendarJulian}},
		{"1917-11-07 н. ст.", FactDate{Year: 1917, Month: 11, Day: 7, Precision: PrecisionDay, Modifier: ModifierExact, Calendar: FactCalendarGregorian}},
		{"1917-11-07 н.ст.", FactDate{Year: 1917, Month: 11, Day: 7, Precision: PrecisionDay, Modifier: ModifierExact, Calendar: FactCalendarGregorian}},
		{"1917 Ст. Ст.", FactDate{Year: 1917, Precision: PrecisionYear, Modifier: ModifierExact, Calendar: FactCalendarJulian}},
		{"около 1881 ст. ст.", FactDate{Year: 1881, Precision: PrecisionYear, Modifier: ModifierApprox, Calendar: FactCalendarJulian}},
		{"между 1880 и 1882 ст. ст.", FactDate{Year: 1880, Precision: PrecisionYear, Modifier: ModifierBetween, YearTo: 1882, Calendar: FactCalendarJulian}},
		{"  1881-03  н. ст.  ", FactDate{Year: 1881, Month: 3, Precision: PrecisionMonth, Modifier: ModifierExact, Calendar: FactCalendarGregorian}},
	}
	for _, tt := range tests {
		got, err := ParseFactDate(tt.in)
		if err != nil {
			t.Errorf("ParseFactDate(%q): unexpected error %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseFactDate(%q)=%+v, want %+v", tt.in, got, tt.want)
		}
	}
}

func TestParseFactDateCalendarErrors(t *testing.T) {
	for _, in := range []string{"ст. ст.", "н. ст.", "между 1880 ст. ст."} {
		if _, err := ParseFactDate(in); err == nil {
			t.Errorf("ParseFactDate(%q): want error, got nil", in)
		}
	}
}

func TestFactDateStringCalendar(t *testing.T) {
	tests := []struct {
		in   FactDate
		want string
	}{
		{FactDate{Year: 1917, Month: 10, Day: 25, Precision: PrecisionDay, Modifier: ModifierExact, Calendar: FactCalendarJulian}, "1917-10-25 ст. ст."},
		{FactDate{Year: 1917, Month: 11, Day: 7, Precision: PrecisionDay, Modifier: ModifierExact, Calendar: FactCalendarGregorian}, "1917-11-07 н. ст."},
		{FactDate{Year: 1917, Precision: PrecisionYear, Modifier: ModifierApprox, Calendar: FactCalendarJulian}, "около 1917 ст. ст."},
		{FactDate{Year: 1917, Precision: PrecisionYear, Modifier: ModifierExact, Calendar: FactCalendarUnknown}, "1917"},
		{FactDate{Year: 1917, Precision: PrecisionYear, Modifier: ModifierExact}, "1917"},
		{FactDate{Precision: PrecisionUnknown, Modifier: ModifierExact, Calendar: FactCalendarJulian}, "?"},
	}
	for _, tt := range tests {
		if got := tt.in.String(); got != tt.want {
			t.Errorf("String(%+v)=%q, want %q", tt.in, got, tt.want)
		}
	}
}

// Разбор и вывод обратимы для всех форм с календарём.
func TestFactDateCalendarRoundTrip(t *testing.T) {
	for _, in := range []string{
		"1917-10-25 ст. ст.",
		"1917-11-07 н. ст.",
		"около 1881 ст. ст.",
		"между 1880-03 и 1882-09-01 ст. ст.",
		"до 1918 н. ст.",
	} {
		d, err := ParseFactDate(in)
		if err != nil {
			t.Errorf("parse %q: %v", in, err)
			continue
		}
		if got := d.String(); got != in {
			t.Errorf("String(Parse(%q))=%q", in, got)
		}
	}
}
```

- [ ] **Step 2: Убедиться, что тесты не компилируются**

Run: `go test ./internal/models`
Expected: FAIL — `unknown field Calendar in struct literal of type FactDate`.

- [ ] **Step 3: Добавить поле и разбор суффикса**

В `internal/models/fact_date.go` в struct `FactDate` после поля `Modifier` добавь поле:

```go
	// Modifier — вид формулировки.
	Modifier FactModifier
	// Calendar — календарь, в котором записана дата (пусто ≡ unknown).
	Calendar FactCalendar
```

(строку `Modifier FactModifier` не дублируй — вставь только комментарий и поле `Calendar` после неё).

В начале `ParseFactDate` замени блок

```go
	s = strings.TrimSpace(s)
	if s == "" {
		return FactDate{}, fmt.Errorf("fact date: пустая строка")
	}
```

на

```go
	s = strings.TrimSpace(s)
	if s == "" {
		return FactDate{}, fmt.Errorf("fact date: пустая строка")
	}
	s, cal := splitCalendarSuffix(s)
```

В обоих `return FactDate{...}` внутри `ParseFactDate` добавь поле `Calendar: cal`:

```go
		return FactDate{
			Year: lo.year, Month: lo.month, Day: lo.day, Precision: lo.precision,
			Modifier: mod, Calendar: cal,
			YearTo: hi.year, MonthTo: hi.month, DayTo: hi.day,
		}, nil
```

```go
	return FactDate{
		Year: c.year, Month: c.month, Day: c.day, Precision: c.precision,
		Modifier: mod, Calendar: cal,
	}, nil
```

Добавь функцию (например, сразу после `ParseFactDate`, перед `components`):

```go
// calendarSuffixes — суффиксы календаря в конце строки даты (в нижнем регистре).
// Порядок важен только в пределах одного календаря.
var calendarSuffixes = []struct {
	suffix   string
	calendar FactCalendar
}{
	{"ст. ст.", FactCalendarJulian},
	{"ст.ст.", FactCalendarJulian},
	{"н. ст.", FactCalendarGregorian},
	{"н.ст.", FactCalendarGregorian},
}

// splitCalendarSuffix отделяет суффикс календаря: «ст. ст.» — юлианский
// (старый стиль), «н. ст.» — григорианский (новый стиль). Регистр не важен;
// без суффикса календарь пуст. Суффиксы состоят из букв, не меняющих длину в
// байтах при смене регистра, поэтому усечение по длине безопасно.
func splitCalendarSuffix(s string) (string, FactCalendar) {
	lower := strings.ToLower(s)
	for _, sf := range calendarSuffixes {
		if strings.HasSuffix(lower, sf.suffix) {
			return strings.TrimSpace(s[:len(s)-len(sf.suffix)]), sf.calendar
		}
	}

	return s, ""
}
```

- [ ] **Step 4: Вывод суффикса в `String()`**

В `fact_date.go` переименуй существующий метод `String` в `text` (тело не меняй) и добавь новый `String`:

```go
// String возвращает каноническую текстовую форму даты; для юлианского и
// григорианского календаря добавляется суффикс « ст. ст.» / « н. ст.».
func (d FactDate) String() string {
	s := d.text()
	if d.Precision == PrecisionUnknown {
		return s
	}
	switch d.Calendar {
	case FactCalendarJulian:
		return s + " ст. ст."
	case FactCalendarGregorian:
		return s + " н. ст."
	}

	return s
}

// text возвращает форму даты без суффикса календаря.
func (d FactDate) text() string {
```

Строку `// String возвращает каноническую текстовую форму даты.` над старым методом удали (комментарий переезжает к новому `String`); тело старого метода становится телом `text()`.

- [ ] **Step 5: Прогнать тесты**

Run: `gofmt -l internal/models` → пусто.
Run: `go test ./internal/models`
Expected: PASS (новые и существующие тесты `FactDate`).

- [ ] **Step 6: Commit**

```bash
git add internal/models/fact_date.go internal/models/fact_date_test.go
git commit -m "feat(models): FactDate.Calendar; разбор и вывод суффиксов ст. ст./н. ст.

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

### Task 2: Сравнение с пересчётом через юлианский день

**Files:**
- Modify: `internal/models/fact_date.go`
- Test: `internal/models/fact_date_test.go`

**Interfaces:**
- Consumes: `FactDate.Calendar`, `bound`, `ordinal`, `ordMin`, `ordMax`, `ord` (Task 1 и существующий код).
- Produces: `FactDate.Compare` с пересчётом пары «юлианская — григорианская»; внутренние функции `julianToJDN`, `jdnToGregorian`, `julianMonthDays`, `julianOrdToGregorian`, метод `bound.fromJulian`.

- [ ] **Step 1: Написать падающие тесты**

Добавь в конец `internal/models/fact_date_test.go` (хелпер `mustParse(t, s)` в этом файле уже есть — не дублируй):

```go
func TestFactDateCompareCalendars(t *testing.T) {
	tests := []struct {
		a, b string
		want FactCompare
	}{
		// 25 октября 1917 ст. ст. — это 7 ноября 1917 н. ст.: один и тот же день.
		{"1917-10-25 ст. ст.", "1917-11-07 н. ст.", CompareIndeterminate},
		{"1917-11-07 н. ст.", "1917-10-25 ст. ст.", CompareIndeterminate},
		// Без пересчёта 25 октября было бы «раньше» 6 ноября — пересчёт это исправляет.
		{"1917-10-25 ст. ст.", "1917-11-06 н. ст.", CompareLater},
		{"1917-11-06 н. ст.", "1917-10-25 ст. ст.", CompareEarlier},
		{"1917-10-25 ст. ст.", "1917-11-08 н. ст.", CompareEarlier},
		// Юлианский месяц покрывает 14 января – 13 февраля 1918 н. ст.
		{"1918-01 ст. ст.", "1918-02-01 н. ст.", CompareIndeterminate},
		{"1918-01 ст. ст.", "1918-02-14 н. ст.", CompareEarlier},
		{"1918-01 ст. ст.", "1918-01-13 н. ст.", CompareLater},
		// 1900 — високосный год в юлианском календаре: 29 февраля ст. ст. = 13 марта н. ст.
		{"1900-02-29 ст. ст.", "1900-03-13 н. ст.", CompareIndeterminate},
		{"1900-02-29 ст. ст.", "1900-03-12 н. ст.", CompareLater},
		// Границы «до»/«после» остаются бесконечными и после пересчёта.
		{"до 1917-10-25 ст. ст.", "1917-11-08 н. ст.", CompareEarlier},
		{"после 1917-10-25 ст. ст.", "1917-11-06 н. ст.", CompareLater},
		{"после 1917-10-25 ст. ст.", "1917-11-08 н. ст.", CompareIndeterminate},
		// Календарь неизвестен или совпадает — сравнение «как есть».
		{"1917-10-25 ст. ст.", "1917-11-06", CompareEarlier},
		{"1917-10-25", "1917-11-06 н. ст.", CompareEarlier},
		{"1917-10-25 ст. ст.", "1917-11-06 ст. ст.", CompareEarlier},
		{"1917-10-25 н. ст.", "1917-11-06 н. ст.", CompareEarlier},
	}
	for _, tt := range tests {
		a, b := mustParse(t, tt.a), mustParse(t, tt.b)
		if got := a.Compare(b); got != tt.want {
			t.Errorf("Compare(%q, %q)=%v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestJulianToGregorian(t *testing.T) {
	tests := []struct {
		y, m, d          int
		wantY, wantM, wD int
	}{
		{1917, 10, 25, 1917, 11, 7},
		{1918, 1, 31, 1918, 2, 13},
		{1900, 2, 29, 1900, 3, 13},
		{1582, 10, 4, 1582, 10, 14},
		{1700, 2, 28, 1700, 3, 10},
		{1800, 1, 1, 1800, 1, 12}, // до 1 марта 1800 разница ещё 11 дней
	}
	for _, tt := range tests {
		gy, gm, gd := jdnToGregorian(julianToJDN(tt.y, tt.m, tt.d))
		if gy != tt.wantY || gm != tt.wantM || gd != tt.wD {
			t.Errorf("Julian %d-%02d-%02d → Gregorian %d-%02d-%02d, want %d-%02d-%02d",
				tt.y, tt.m, tt.d, gy, gm, gd, tt.wantY, tt.wantM, tt.wD)
		}
	}
}
```

- [ ] **Step 2: Убедиться, что тесты не компилируются**

Run: `go test ./internal/models`
Expected: FAIL — `undefined: jdnToGregorian`, `undefined: julianToJDN`.

- [ ] **Step 3: Арифметика юлианского дня**

Добавь в `internal/models/fact_date.go` (после `Compare`):

```go
// julianToJDN возвращает юлианский день (JDN) для даты юлианского календаря.
func julianToJDN(y, m, d int) int {
	a := (14 - m) / 12
	yy := y + 4800 - a
	mm := m + 12*a - 3

	return d + (153*mm+2)/5 + 365*yy + yy/4 - 32083
}

// jdnToGregorian возвращает дату григорианского календаря по юлианскому дню.
func jdnToGregorian(jdn int) (year, month, day int) {
	a := jdn + 32044
	b := (4*a + 3) / 146097
	c := a - 146097*b/4
	d := (4*c + 3) / 1461
	e := c - 1461*d/4
	m := (5*e + 2) / 153

	day = e - (153*m+2)/5 + 1
	month = m + 3 - 12*(m/10)
	year = 100*b + d - 4800 + m/10

	return year, month, day
}

// julianMonthDays — число дней месяца в юлианском календаре (високосный год —
// каждый четвёртый).
func julianMonthDays(y, m int) int {
	switch m {
	case 2:
		if y%4 == 0 {
			return 29
		}

		return 28
	case 4, 6, 9, 11:
		return 30
	default:
		return 31
	}
}

// julianOrdToGregorian переводит порядковый номер юлианской даты в
// григорианский. Бесконечные границы не меняются; «31-е» верхней границы
// месяца сначала ограничивается реальным последним днём юлианского месяца.
func julianOrdToGregorian(o ordinal) ordinal {
	if o == ordMin || o == ordMax {
		return o
	}

	y, m, d := int(o)/10000, int(o)/100%100, int(o)%100
	if last := julianMonthDays(y, m); d > last {
		d = last
	}

	return ord(jdnToGregorian(julianToJDN(y, m, d)))
}

// fromJulian переводит обе границы интервала из юлианского календаря в григорианский.
func (b bound) fromJulian() bound {
	return bound{julianOrdToGregorian(b.lo), julianOrdToGregorian(b.hi)}
}
```

Функция `ord(year, month, day int)` принимает три аргумента, а `jdnToGregorian` возвращает три значения — вызов `ord(jdnToGregorian(...))` допустим в Go.

- [ ] **Step 4: Подключить пересчёт в `Compare`**

В `Compare` перед `switch` (после проверки `!okA || !okB`) добавь:

```go
	switch {
	case d.Calendar == FactCalendarJulian && other.Calendar == FactCalendarGregorian:
		a = a.fromJulian()
	case d.Calendar == FactCalendarGregorian && other.Calendar == FactCalendarJulian:
		b = b.fromJulian()
	}
```

Обнови комментарий метода:

```go
// Compare сравнивает даты по интервалам: раньше / позже / перекрытие
// (неопределимо). Если одна дата юлианская, а другая григорианская, границы
// юлианской переводятся в григорианские; в остальных случаях (одинаковые
// календари, unknown или пустой календарь) сравнение идёт «как есть».
```

- [ ] **Step 5: Прогнать тесты**

Run: `gofmt -l internal/models` → пусто.
Run: `go test ./internal/models`
Expected: PASS, включая `TestFactDateCompareCalendars` и `TestJulianToGregorian`; существующие `TestFactDateCompare` и `TestFactDateUnknownCompare` не изменились и проходят.

- [ ] **Step 6: Commit**

```bash
git add internal/models/fact_date.go internal/models/fact_date_test.go
git commit -m "feat(models): FactDate.Compare переводит юлианские границы в григорианские

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

### Task 3: Запись и чтение календаря в БД

**Files:**
- Modify: `internal/storage/schema.go` (умолчание колонки `calendar`)
- Modify: `internal/store/sqlstore/helpers.go` (`insertDate`, `loadDate`)
- Test: `internal/store/sqlstore/sqlstore_test.go`

**Interfaces:**
- Consumes: `FactDate.Calendar` (Task 1), `models.Event`, `Store.SaveEvent`/`GetEvent`.
- Produces: календарь даты сохраняется в `dates.calendar` и возвращается при чтении без изменений (пустой остаётся пустым).

- [ ] **Step 1: Написать падающий тест**

Добавь в `internal/store/sqlstore/sqlstore_test.go` (рядом с другими round-trip тестами):

```go
// TestStoreDateCalendarRoundTrip: календарь даты пишется и читается как есть,
// включая пустое значение (пусто ≡ unknown, но идентичность сохраняется).
func TestStoreDateCalendarRoundTrip(t *testing.T) {
	s := newStore(t)

	tests := []struct {
		id       models.ID
		calendar models.FactCalendar
	}{
		{"ev-cal-julian", models.FactCalendarJulian},
		{"ev-cal-gregorian", models.FactCalendarGregorian},
		{"ev-cal-unknown", models.FactCalendarUnknown},
		{"ev-cal-empty", ""},
	}
	for _, tt := range tests {
		date := models.FactDate{
			Year: 1917, Month: 10, Day: 25,
			Precision: models.PrecisionDay, Modifier: models.ModifierExact,
			Calendar: tt.calendar,
		}
		if err := s.SaveEvent(&models.Event{ID: tt.id, Type: models.EventTypeBirth, Date: &date}); err != nil {
			t.Fatalf("save event %s: %v", tt.id, err)
		}

		got, err := s.GetEvent(tt.id)
		if err != nil {
			t.Fatalf("get event %s: %v", tt.id, err)
		}
		if got.Date == nil {
			t.Fatalf("event %s: дата потеряна", tt.id)
		}
		if got.Date.Calendar != tt.calendar {
			t.Errorf("event %s: calendar = %q, want %q", tt.id, got.Date.Calendar, tt.calendar)
		}
	}
}
```

- [ ] **Step 2: Убедиться, что тест падает**

Run: `go test ./internal/store/sqlstore -run TestStoreDateCalendarRoundTrip`
Expected: FAIL — календарь читается пустым, а не записанным значением (например, `event ev-cal-julian: calendar = "", want "julian"`).

- [ ] **Step 3: Писать и читать календарь**

В `internal/store/sqlstore/helpers.go` замени `insertDate` и убери устаревшую фразу в комментарии:

```go
// insertDate пишет FactDate в dates и возвращает id; nil — 0 (колонка NULL).
func insertDate(tx *sql.Tx, d *models.FactDate) (int64, error) {
	if d == nil {
		return 0, nil
	}

	res, err := tx.Exec(
		`INSERT INTO dates(year, month, day, precision, modifier, year_to, month_to, day_to, calendar)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.Year, d.Month, d.Day, string(d.Precision), string(d.Modifier),
		d.YearTo, d.MonthTo, d.DayTo, string(d.Calendar))
	if err != nil {
		return 0, err
	}

	return res.LastInsertId()
}
```

и `loadDate`:

```go
// loadDate читает FactDate по id; 0 — nil (поле не заполнено).
func loadDate(q queryer, id int64) (*models.FactDate, error) {
	if id == 0 {
		return nil, nil
	}

	var (
		d              models.FactDate
		prec, mod, cal string
	)

	if err := q.QueryRow(
		`SELECT year, month, day, precision, modifier, year_to, month_to, day_to, calendar
		 FROM dates WHERE id = ?`, id,
	).Scan(&d.Year, &d.Month, &d.Day, &prec, &mod, &d.YearTo, &d.MonthTo, &d.DayTo, &cal); err != nil {
		return nil, err
	}

	d.Precision, d.Modifier = models.FactPrecision(prec), models.FactModifier(mod)
	d.Calendar = models.FactCalendar(cal)

	return &d, nil
}
```

В `internal/storage/schema.go` замени умолчание колонки:

```go
		calendar  TEXT    NOT NULL DEFAULT ''
```

(вместо `DEFAULT 'gregorian'`; схема ещё `schema_version = 0`, данные разработки допускается потерять).

- [ ] **Step 4: Прогнать тесты**

Run: `gofmt -l internal/store internal/storage` → пусто.
Run: `go test ./internal/store/... ./internal/storage/...`
Expected: PASS — новый тест и существующие round-trip тесты (`FactDate` без календаря возвращается пустым, `reflect.DeepEqual` сохраняется).

- [ ] **Step 5: Commit**

```bash
git add internal/storage/schema.go internal/store/sqlstore/helpers.go internal/store/sqlstore/sqlstore_test.go
git commit -m "feat(sqlstore): календарь даты пишется и читается через dates.calendar

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

### Task 4: Документация и рубеж

**Files:**
- Modify: `docs/models/values.md`

**Interfaces:**
- Consumes: реализованное поведение Task 1–3.
- Produces: документ модели без утверждений, противоречащих коду.

- [ ] **Step 1: Обновить описание календаря**

В `docs/models/values.md` (раздел `FactDate`) замени фразу

```
и календарь ортогональны (независимые признаки). Сравнения выполняются в рамках
одного календаря.
```

на

```
и календарь ортогональны (независимые признаки). Дата хранится так, как записана в
источнике. При сравнении юлианской и григорианской дат границы юлианской
переводятся в григорианские (через юлианский день); при `unknown` или совпадающих
календарях сравнение идёт «как есть». Разбор понимает суффиксы `ст. ст.` (юлианский)
и `н. ст.` (григорианский): `1917-10-25 ст. ст.` = `1917-11-07 н. ст.`.
```

и в описании поля `calendar` допиши: «пустое значение эквивалентно `unknown`».

- [ ] **Step 2: Рубеж**

```bash
gofmt -l .            # пусто
go build ./...
go vet ./...
go test ./...
```

Expected: всё зелёное.

- [ ] **Step 3: Commit**

```bash
git add docs/models/values.md
git commit -m "docs(models): календарь FactDate — хранение и сравнение

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

## Самопроверка плана

- **Покрытие спеки §2.1:** поле и пустое ≡ unknown (Task 1); разбор и вывод суффиксов, обратимость (Task 1); сравнение с пересчётом, «как есть» при `unknown`/одинаковых календарях, бесконечные границы, реальный последний день юлианского месяца (Task 2); хранение `dates.calendar` как есть, умолчание `''` (Task 3); документация (Task 4).
- **Согласованность типов и имён:** `FactDate.Calendar`, `splitCalendarSuffix`, `julianToJDN`, `jdnToGregorian`, `julianMonthDays`, `julianOrdToGregorian`, `bound.fromJulian` — определены до использования.
- **Не входит в этап:** валидация (S3), формат `ID` (S2), проверка допустимости календаря в `Validate` (S3).
