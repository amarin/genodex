# S3: Валидация — основа и люди — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ввести `ValidationError`, проверки закрытых и открытых enum'ов, value-типов (`FactDate`, `TextRef`, `SourceLink`, периоды) и `Validate() error` для `Person`, `PersonName`, `Relation`, `Residence`, `Family` и пяти словарей.

**Architecture:** Чистые проверки в `internal/models` без обращения к хранилищу. Внутренние функции возвращают `*ValidationError` (путь поля собирается через `within`), публичный `Validate() error` оборачивает результат в `error` без typed-nil. Закрытые enum'ы получают метод `Valid()`. Существование ссылок и циклы — забота сценариев (S14), здесь не проверяются.

**Tech Stack:** Go 1.26.4, стандартная библиотека.

**Spec:** `docs/data-model/core-read-write.md` §2.3; `docs/models/people.md`; `docs/plans/2026-09-20-core-rw-roadmap.md` (этап S3).

## Global Constraints

- `Validate()` — чистая функция: без I/O, без обращения к хранилищу; возвращает `nil` или `*models.ValidationError` (`Entity`, `Field`, `Reason`); первая найденная ошибка.
- Имена полей в `ValidationError.Field` — как в документах модели (snake_case: `person_a`, `rel_type`, `citation_id`); элементы списков — `names[0]`, вложенные поля — через точку: `names[0].surname.type`.
- Закрытые enum'ы принимают только константы; пустое значение допустимо только у полей, помеченных необязательными (`Person.Gender`, `PersonName.Type`, `FactDate.Calendar`, `SourceLink.Reliability`, `Relation.RelType` вне `associate`).
- Открытые enum'ы (`RelationType`): непустое значение вида `[a-z][a-z0-9_-]*`.
- Идентификаторы — `ID.Validate(Type)` (формат и префикс ожидаемого типа); идентификатор самой сущности обязателен.
- `TextRef`: без ссылки — `Text` не пуст и `Type` пуст; со ссылкой — `Type` обязателен, допустим и совпадает с префиксом `Ref`.
- Правила сущностей (уточнения к §2.3, зафиксированы в спеке): часть имени `PersonName` — `TextRef` с ожидаемым типом ссылки (`surname` → `surname`, `given` → `given_name`, `patronymic` → `patronymic`); `Person.Estates` → `estate`, `Person.Titles` → `title`; `variants` словаря — того же типа, что и словарь; имя (`PersonName`) содержит хотя бы одну непустую часть; `Family.Name` и `canonical` словарей обязательны (непустые после обрезки пробелов); `GivenName.Gender` обязателен.
- `models` не содержит JSON-тегов и не импортирует `encoding/json`. Комментарии и тексты ошибок — на русском. Один файл — один основной тип; проверки живут в файлах `*_validate.go`.
- Существующие тесты и код `store`/`sqlstore` не меняются: `Validate()` там ещё не вызывается.
- Проверка рубежа: `gofmt -l .` пусто, `go build ./...`, `go vet ./...`, `go test ./...`.

---

### Task 1: Ядро — `ValidationError`, `Valid()` у enum'ов, открытые enum'ы

**Files:**
- Create: `internal/models/validation.go`
- Create: `internal/models/enum_valid.go`
- Test: `internal/models/validation_test.go`

**Interfaces:**
- Produces:
  - `type ValidationError struct{ Entity Type; Field, Reason string }` с `Error() string`;
  - внутренние: `fieldErr(field, format, args…) *ValidationError`, `(*ValidationError).within(prefix) *ValidationError`, `indexed(field, i) string`, `finish(entity Type, e *ValidationError) error`, `validOpenEnum(s string) bool`;
  - методы `Valid() bool` у `Type`, `PersonGender`, `NameGender`, `PersonNameType`, `RelationKind`, `SourceKind`, `AttachmentKind`, `Reliability`, `AnchorKind`, `FactPrecision`, `FactModifier`, `FactCalendar`, `AdminDivisionType`;
  - тестовые хелперы (файл `validation_test.go`, пакет `models`): `testID(Type) ID` (валидный id типа, использует `validBody` из `id_test.go`), `wantInvalid(t, name, err, entity, field)`.

- [ ] **Step 1: Написать падающие тесты**

`internal/models/validation_test.go`:

```go
package models

import (
	"errors"
	"testing"
)

// testID возвращает валидный идентификатор для типа сущности.
func testID(t Type) ID {
	id, err := BuildID(t, validBody)
	if err != nil {
		panic(err)
	}

	return id
}

// wantInvalid проверяет, что err — *ValidationError с ожидаемой сущностью и полем.
func wantInvalid(t *testing.T, name string, err error, entity Type, field string) {
	t.Helper()

	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Errorf("%s: err = %v, want *ValidationError", name, err)

		return
	}
	if ve.Entity != entity || ve.Field != field {
		t.Errorf("%s: получено %s / %q (%s), want %s / %q", name, ve.Entity, ve.Field, ve.Reason, entity, field)
	}
}

func TestValidationErrorMessage(t *testing.T) {
	tests := []struct {
		e    ValidationError
		want string
	}{
		{ValidationError{Entity: TypePerson, Field: "names[0].surname", Reason: "пусто"}, "person: names[0].surname: пусто"},
		{ValidationError{Field: "year", Reason: "вне диапазона"}, "year: вне диапазона"},
		{ValidationError{Entity: TypeFamily, Reason: "нет имени"}, "family: нет имени"},
	}
	for _, tt := range tests {
		if got := tt.e.Error(); got != tt.want {
			t.Errorf("Error() = %q, want %q", got, tt.want)
		}
	}
}

func TestWithinBuildsFieldPath(t *testing.T) {
	tests := []struct {
		field, prefix, want string
	}{
		{"", "since", "since"},
		{"year", "since", "since.year"},
		{"[0]", "names", "names[0]"},
		{"surname.type", "names[0]", "names[0].surname.type"},
	}
	for _, tt := range tests {
		e := &ValidationError{Field: tt.field, Reason: "x"}
		if got := e.within(tt.prefix).Field; got != tt.want {
			t.Errorf("within(%q) на %q = %q, want %q", tt.prefix, tt.field, got, tt.want)
		}
	}

	var nilErr *ValidationError
	if nilErr.within("x") != nil {
		t.Error("within на nil должен оставаться nil")
	}
	if got := indexed("names", 2); got != "names[2]" {
		t.Errorf("indexed = %q", got)
	}
}

func TestFinishNoTypedNil(t *testing.T) {
	if err := finish(TypePerson, nil); err != nil {
		t.Errorf("finish(nil) = %v, want nil-интерфейс", err)
	}

	err := finish(TypePerson, fieldErr("id", "плохой %s", "формат"))
	wantInvalid(t, "finish", err, TypePerson, "id")
}

func TestClosedEnumsValid(t *testing.T) {
	enums := []struct {
		name  string
		valid []string
		check func(string) bool
	}{
		{"PersonGender", []string{"unknown", "male", "female"}, func(s string) bool { return PersonGender(s).Valid() }},
		{"NameGender", []string{"male", "female", "neutral"}, func(s string) bool { return NameGender(s).Valid() }},
		{"PersonNameType", []string{"main", "birth", "married", "changed", "pseudonym"}, func(s string) bool { return PersonNameType(s).Valid() }},
		{"RelationKind", []string{"blood", "marriage", "adoption", "associate"}, func(s string) bool { return RelationKind(s).Valid() }},
		{"SourceKind", []string{"archival-scan", "transcription", "document", "audio", "photo", "memory", "external"}, func(s string) bool { return SourceKind(s).Valid() }},
		{"AttachmentKind", []string{"scan", "document", "audio", "photo"}, func(s string) bool { return AttachmentKind(s).Valid() }},
		{"Reliability", []string{"primary", "contemporary", "memory", "indirect", "unknown"}, func(s string) bool { return Reliability(s).Valid() }},
		{"AnchorKind", []string{"archive", "file", "url"}, func(s string) bool { return AnchorKind(s).Valid() }},
		{"FactPrecision", []string{"unknown", "year", "month", "day"}, func(s string) bool { return FactPrecision(s).Valid() }},
		{"FactModifier", []string{"exact", "approx", "before", "after", "between"}, func(s string) bool { return FactModifier(s).Valid() }},
		{"FactCalendar", []string{"gregorian", "julian", "unknown"}, func(s string) bool { return FactCalendar(s).Valid() }},
		{"AdminDivisionType", []string{"governorate", "district", "volost", "other", "gorod", "selo", "derevnya", "hutor", "pogost", "stanitsa", "mestechko"}, func(s string) bool { return AdminDivisionType(s).Valid() }},
	}
	for _, e := range enums {
		for _, v := range e.valid {
			if !e.check(v) {
				t.Errorf("%s(%q) должен быть допустим", e.name, v)
			}
		}
		for _, bad := range []string{"", "bogus", "MALE", " male"} {
			if e.check(bad) {
				t.Errorf("%s(%q) не должен быть допустим", e.name, bad)
			}
		}
	}
}

func TestTypeValid(t *testing.T) {
	for _, typ := range AllTypes() {
		if !typ.Valid() {
			t.Errorf("Type(%q) должен быть допустим", typ)
		}
	}
	for _, bad := range []Type{"", "nonsense", "Person"} {
		if bad.Valid() {
			t.Errorf("Type(%q) не должен быть допустим", bad)
		}
	}
}

func TestValidOpenEnum(t *testing.T) {
	for _, ok := range []string{"neighbor", "god-parent", "a", "witness_2", "x9"} {
		if !validOpenEnum(ok) {
			t.Errorf("validOpenEnum(%q) = false, want true", ok)
		}
	}
	for _, bad := range []string{"", "Neighbor", "2fast", "-x", "_x", "a b", "друг", "a.b", "a\x00"} {
		if validOpenEnum(bad) {
			t.Errorf("validOpenEnum(%q) = true, want false", bad)
		}
	}
}
```

- [ ] **Step 2: Убедиться, что тесты не компилируются**

Run: `go test ./internal/models`
Expected: FAIL — `undefined: ValidationError`, `undefined: finish`, `undefined: validOpenEnum`, `.Valid undefined`.

- [ ] **Step 3: Ядро валидации**

`internal/models/validation.go`:

```go
package models

import (
	"fmt"
	"strings"
)

// ValidationError — нарушение инварианта сущности или значения. Validate()
// возвращает первую найденную ошибку.
type ValidationError struct {
	// Entity — тип сущности (пусто, если проверяется отдельный value-тип).
	Entity Type
	// Field — путь к полю: names[0].surname.type (имена — как в docs/models).
	Field string
	// Reason — причина по-русски.
	Reason string
}

// Error возвращает «сущность: поле: причина» (пустые части опускаются).
func (e *ValidationError) Error() string {
	var b strings.Builder
	if e.Entity != "" {
		b.WriteString(string(e.Entity))
		b.WriteString(": ")
	}
	if e.Field != "" {
		b.WriteString(e.Field)
		b.WriteString(": ")
	}
	b.WriteString(e.Reason)

	return b.String()
}

// fieldErr создаёт ошибку для поля.
func fieldErr(field, format string, args ...any) *ValidationError {
	return &ValidationError{Field: field, Reason: fmt.Sprintf(format, args...)}
}

// within добавляет к пути поля префикс родительского поля; nil остаётся nil.
func (e *ValidationError) within(prefix string) *ValidationError {
	if e == nil {
		return nil
	}

	switch {
	case e.Field == "":
		e.Field = prefix
	case strings.HasPrefix(e.Field, "["):
		e.Field = prefix + e.Field
	default:
		e.Field = prefix + "." + e.Field
	}

	return e
}

// indexed возвращает путь элемента списка: names[2].
func indexed(field string, i int) string {
	return fmt.Sprintf("%s[%d]", field, i)
}

// finish превращает внутреннюю ошибку в error публичного Validate: nil
// остаётся nil-интерфейсом (без typed-nil), иначе проставляется сущность.
func finish(entity Type, e *ValidationError) error {
	if e == nil {
		return nil
	}
	e.Entity = entity

	return e
}

// validOpenEnum проверяет значение открытого enum'а: непустое, формат
// [a-z][a-z0-9_-]*.
func validOpenEnum(s string) bool {
	if s == "" || s[0] < 'a' || s[0] > 'z' {
		return false
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' && c != '-' {
			return false
		}
	}

	return true
}
```

- [ ] **Step 4: `Valid()` для закрытых enum'ов**

`internal/models/enum_valid.go` (проверки допустимости закрытых enum'ов собраны в одном файле: сами типы объявлены в отдельных файлах):

```go
package models

// Valid сообщает, что тип сущности известен (есть префикс идентификатора).
func (t Type) Valid() bool { return t.IDPrefix() != "" }

// Valid сообщает, что значение — одна из констант.
func (g PersonGender) Valid() bool {
	switch g {
	case Male, Female, Unknown:
		return true
	}

	return false
}

// Valid сообщает, что значение — одна из констант.
func (g NameGender) Valid() bool {
	switch g {
	case MaleName, FemaleName, NeutralName:
		return true
	}

	return false
}

// Valid сообщает, что значение — одна из констант.
func (t PersonNameType) Valid() bool {
	switch t {
	case PersonNameMain, PersonNameBirth, PersonNameMarried, PersonNameChanged, PersonNamePseudonym:
		return true
	}

	return false
}

// Valid сообщает, что значение — одна из констант.
func (k RelationKind) Valid() bool {
	switch k {
	case RelationKindBlood, RelationKindMarriage, RelationKindAdoption, RelationKindAssociate:
		return true
	}

	return false
}

// Valid сообщает, что значение — одна из констант.
func (k SourceKind) Valid() bool {
	switch k {
	case SourceKindArchivalScan, SourceKindTranscription, SourceKindDocument,
		SourceKindAudio, SourceKindPhoto, SourceKindMemory, SourceKindExternal:
		return true
	}

	return false
}

// Valid сообщает, что значение — одна из констант.
func (k AttachmentKind) Valid() bool {
	switch k {
	case AttachmentKindScan, AttachmentKindDocument, AttachmentKindAudio, AttachmentKindPhoto:
		return true
	}

	return false
}

// Valid сообщает, что значение — одна из констант.
func (r Reliability) Valid() bool {
	switch r {
	case ReliabilityPrimary, ReliabilityContemporary, ReliabilityMemory,
		ReliabilityIndirect, ReliabilityUnknown:
		return true
	}

	return false
}

// Valid сообщает, что значение — одна из констант.
func (k AnchorKind) Valid() bool {
	switch k {
	case AnchorArchive, AnchorFile, AnchorURL:
		return true
	}

	return false
}

// Valid сообщает, что значение — одна из констант.
func (p FactPrecision) Valid() bool {
	switch p {
	case PrecisionUnknown, PrecisionYear, PrecisionMonth, PrecisionDay:
		return true
	}

	return false
}

// Valid сообщает, что значение — одна из констант.
func (m FactModifier) Valid() bool {
	switch m {
	case ModifierExact, ModifierApprox, ModifierBefore, ModifierAfter, ModifierBetween:
		return true
	}

	return false
}

// Valid сообщает, что значение — одна из констант.
func (c FactCalendar) Valid() bool {
	switch c {
	case FactCalendarGregorian, FactCalendarJulian, FactCalendarUnknown:
		return true
	}

	return false
}

// Valid сообщает, что значение — одна из констант (единицы деления и виды
// населённых пунктов).
func (t AdminDivisionType) Valid() bool {
	switch t {
	case AdminDivisionGovernorate, AdminDivisionDistrict, AdminDivisionVolost, AdminDivisionOther,
		AdminDivisionGorod, AdminDivisionSelo, AdminDivisionDerevnya, AdminDivisionHutor,
		AdminDivisionPogost, AdminDivisionStanitsa, AdminDivisionMestechko:
		return true
	}

	return false
}
```

- [ ] **Step 5: Прогнать тесты**

Run: `gofmt -l internal/models` → пусто.
Run: `go vet ./internal/models && go test ./internal/models`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/models/validation.go internal/models/enum_valid.go internal/models/validation_test.go
git commit -m "feat(models): ValidationError, Valid() закрытых enum'ов, открытые enum'ы

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

### Task 2: `FactDate.Validate` и проверка периодов

**Files:**
- Create: `internal/models/fact_date_validate.go`
- Test: `internal/models/fact_date_validate_test.go`

**Interfaces:**
- Consumes: `ValidationError`, `fieldErr`, `within`, `finish`, `Valid()` (Task 1); `FactDate`, `ord`, `julianMonthDays`, `Compare` (`fact_date.go`).
- Produces: `func (d FactDate) Validate() error`; внутренние `(d FactDate) validate() *ValidationError`, `daysInMonth(year, month int, cal FactCalendar) int`, `validatePeriod(since, until *FactDate) *ValidationError`.

- [ ] **Step 1: Написать падающие тесты**

`internal/models/fact_date_validate_test.go`:

```go
package models

import "testing"

func TestFactDateValidate(t *testing.T) {
	day := FactDate{Year: 1881, Month: 3, Day: 15, Precision: PrecisionDay, Modifier: ModifierExact}
	with := func(f func(*FactDate)) FactDate {
		d := day
		f(&d)

		return d
	}

	valid := map[string]FactDate{
		"день":                     day,
		"месяц":                    {Year: 1881, Month: 3, Precision: PrecisionMonth, Modifier: ModifierExact},
		"год":                      {Year: 1881, Precision: PrecisionYear, Modifier: ModifierApprox},
		"неизвестная":              UnknownDate(),
		"юлианский 29.02.1900":     {Year: 1900, Month: 2, Day: 29, Precision: PrecisionDay, Modifier: ModifierExact, Calendar: FactCalendarJulian},
		"без календаря 29.02.1900": {Year: 1900, Month: 2, Day: 29, Precision: PrecisionDay, Modifier: ModifierExact},
		"between по годам":         {Year: 1880, Precision: PrecisionYear, Modifier: ModifierBetween, YearTo: 1882},
		"between год внутри":       {Year: 1880, Month: 3, Precision: PrecisionMonth, Modifier: ModifierBetween, YearTo: 1880},
		"between по дням":          {Year: 1880, Month: 3, Day: 1, Precision: PrecisionDay, Modifier: ModifierBetween, YearTo: 1880, MonthTo: 3, DayTo: 31},
		"до":                       {Year: 1881, Precision: PrecisionYear, Modifier: ModifierBefore},
	}
	for name, d := range valid {
		if err := d.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	invalid := []struct {
		name  string
		d     FactDate
		field string
	}{
		{"пустая точность", with(func(d *FactDate) { d.Precision = "" }), "precision"},
		{"пустая формулировка", with(func(d *FactDate) { d.Modifier = "" }), "modifier"},
		{"плохой календарь", with(func(d *FactDate) { d.Calendar = "mayan" }), "calendar"},
		{"unknown с годом", FactDate{Year: 1881, Precision: PrecisionUnknown, Modifier: ModifierExact}, "year"},
		{"unknown с YearTo", FactDate{Precision: PrecisionUnknown, Modifier: ModifierExact, YearTo: 1900}, "year_to"},
		{"unknown с approx", FactDate{Precision: PrecisionUnknown, Modifier: ModifierApprox}, "modifier"},
		{"год 0", with(func(d *FactDate) { d.Year = 0 }), "year"},
		{"год 10000", with(func(d *FactDate) { d.Year = 10000 }), "year"},
		{"год с месяцем", FactDate{Year: 1881, Month: 3, Precision: PrecisionYear, Modifier: ModifierExact}, "month"},
		{"год с днём", FactDate{Year: 1881, Day: 3, Precision: PrecisionYear, Modifier: ModifierExact}, "day"},
		{"месяц 13", FactDate{Year: 1881, Month: 13, Precision: PrecisionMonth, Modifier: ModifierExact}, "month"},
		{"месяц 0", FactDate{Year: 1881, Precision: PrecisionMonth, Modifier: ModifierExact}, "month"},
		{"месяц с днём", FactDate{Year: 1881, Month: 3, Day: 5, Precision: PrecisionMonth, Modifier: ModifierExact}, "day"},
		{"день 0", with(func(d *FactDate) { d.Day = 0 }), "day"},
		{"31 апреля", with(func(d *FactDate) { d.Month, d.Day = 4, 31 }), "day"},
		{"29.02.1900 григорианский", with(func(d *FactDate) { d.Month, d.Day, d.Year, d.Calendar = 2, 29, 1900, FactCalendarGregorian }), "day"},
		{"29.02.1901 юлианский", with(func(d *FactDate) { d.Month, d.Day, d.Year, d.Calendar = 2, 29, 1901, FactCalendarJulian }), "day"},
		{"YearTo без between", with(func(d *FactDate) { d.YearTo = 1900 }), "year_to"},
		{"MonthTo без between", with(func(d *FactDate) { d.MonthTo = 5 }), "month_to"},
		{"between без YearTo", FactDate{Year: 1880, Precision: PrecisionYear, Modifier: ModifierBetween}, "year_to"},
		{"between DayTo без MonthTo", FactDate{Year: 1880, Precision: PrecisionYear, Modifier: ModifierBetween, YearTo: 1882, DayTo: 3}, "day_to"},
		{"between MonthTo 13", FactDate{Year: 1880, Precision: PrecisionYear, Modifier: ModifierBetween, YearTo: 1882, MonthTo: 13}, "month_to"},
		{"between DayTo 31 февраля", FactDate{Year: 1880, Precision: PrecisionYear, Modifier: ModifierBetween, YearTo: 1882, MonthTo: 2, DayTo: 31}, "day_to"},
		{"between верхняя раньше нижней", FactDate{Year: 1900, Precision: PrecisionYear, Modifier: ModifierBetween, YearTo: 1899}, "year_to"},
		{"between верхний месяц раньше нижнего", FactDate{Year: 1900, Month: 6, Precision: PrecisionMonth, Modifier: ModifierBetween, YearTo: 1900, MonthTo: 3}, "year_to"},
	}
	for _, tt := range invalid {
		wantInvalid(t, tt.name, tt.d.Validate(), "", tt.field)
	}
}

func TestDaysInMonth(t *testing.T) {
	tests := []struct {
		year, month int
		cal         FactCalendar
		want        int
	}{
		{1900, 2, FactCalendarGregorian, 28},
		{1900, 2, FactCalendarJulian, 29},
		{1900, 2, "", 29}, // календарь не задан — снисходительно, по юлианскому правилу
		{2000, 2, FactCalendarGregorian, 29},
		{1901, 2, FactCalendarJulian, 28},
		{1881, 4, FactCalendarGregorian, 30},
		{1881, 3, FactCalendarJulian, 31},
	}
	for _, tt := range tests {
		if got := daysInMonth(tt.year, tt.month, tt.cal); got != tt.want {
			t.Errorf("daysInMonth(%d, %d, %q) = %d, want %d", tt.year, tt.month, tt.cal, got, tt.want)
		}
	}
}

func TestValidatePeriod(t *testing.T) {
	y := func(year int) *FactDate {
		return &FactDate{Year: year, Precision: PrecisionYear, Modifier: ModifierExact}
	}

	if e := validatePeriod(nil, nil); e != nil {
		t.Errorf("пустой период: %v", e)
	}
	if e := validatePeriod(y(1881), nil); e != nil {
		t.Errorf("открытый конец: %v", e)
	}
	if e := validatePeriod(y(1881), y(1881)); e != nil {
		t.Errorf("совпадающие даты (неопределимо) допустимы: %v", e)
	}
	if e := validatePeriod(y(1881), y(1900)); e != nil {
		t.Errorf("нормальный период: %v", e)
	}

	wantInvalid(t, "начало позже конца", finish("", validatePeriod(y(1900), y(1881))), "", "since")
	wantInvalid(t, "плохое начало", finish("", validatePeriod(&FactDate{}, nil)), "", "since.precision")
	wantInvalid(t, "плохой конец", finish("", validatePeriod(nil, &FactDate{Year: 1881, Precision: PrecisionYear})), "", "until.modifier")

	// Начало 25.10.1917 ст. ст. не позже 07.11.1917 н. ст. (один день).
	julian := &FactDate{Year: 1917, Month: 10, Day: 25, Precision: PrecisionDay, Modifier: ModifierExact, Calendar: FactCalendarJulian}
	greg := &FactDate{Year: 1917, Month: 11, Day: 7, Precision: PrecisionDay, Modifier: ModifierExact, Calendar: FactCalendarGregorian}
	if e := validatePeriod(julian, greg); e != nil {
		t.Errorf("период в разных календарях: %v", e)
	}
	wantInvalid(t, "разные календари, начало позже", finish("", validatePeriod(greg, &FactDate{Year: 1917, Month: 10, Day: 24, Precision: PrecisionDay, Modifier: ModifierExact, Calendar: FactCalendarJulian})), "", "since")
}
```

- [ ] **Step 2: Убедиться, что тесты не компилируются**

Run: `go test ./internal/models`
Expected: FAIL — `d.Validate undefined`, `undefined: daysInMonth`, `undefined: validatePeriod`.

- [ ] **Step 3: Реализация**

`internal/models/fact_date_validate.go`:

```go
package models

// Validate проверяет инварианты даты: допустимые enum'ы, согласованность
// точности и компонентов, реальные границы месяцев и дней (с учётом
// календаря), верхнюю границу для between.
func (d FactDate) Validate() error {
	return finish("", d.validate())
}

// validate возвращает первую найденную ошибку или nil.
func (d FactDate) validate() *ValidationError {
	if !d.Precision.Valid() {
		return fieldErr("precision", "недопустимая точность %q", d.Precision)
	}
	if !d.Modifier.Valid() {
		return fieldErr("modifier", "недопустимая формулировка %q", d.Modifier)
	}
	if d.Calendar != "" && !d.Calendar.Valid() {
		return fieldErr("calendar", "недопустимый календарь %q", d.Calendar)
	}

	if d.Precision == PrecisionUnknown {
		for _, c := range []struct {
			field string
			value int
		}{
			{"year", d.Year}, {"month", d.Month}, {"day", d.Day},
			{"year_to", d.YearTo}, {"month_to", d.MonthTo}, {"day_to", d.DayTo},
		} {
			if c.value != 0 {
				return fieldErr(c.field, "при неизвестной точности компоненты даты должны быть нулевыми")
			}
		}
		if d.Modifier != ModifierExact {
			return fieldErr("modifier", "при неизвестной точности допустим только exact")
		}

		return nil
	}

	if d.Year < 1 || d.Year > 9999 {
		return fieldErr("year", "год вне диапазона 1–9999: %d", d.Year)
	}

	switch d.Precision {
	case PrecisionYear:
		if d.Month != 0 {
			return fieldErr("month", "при точности year месяц должен быть 0")
		}
		if d.Day != 0 {
			return fieldErr("day", "при точности year день должен быть 0")
		}
	case PrecisionMonth:
		if d.Month < 1 || d.Month > 12 {
			return fieldErr("month", "месяц вне диапазона 1–12: %d", d.Month)
		}
		if d.Day != 0 {
			return fieldErr("day", "при точности month день должен быть 0")
		}
	case PrecisionDay:
		if d.Month < 1 || d.Month > 12 {
			return fieldErr("month", "месяц вне диапазона 1–12: %d", d.Month)
		}
		if last := daysInMonth(d.Year, d.Month, d.Calendar); d.Day < 1 || d.Day > last {
			return fieldErr("day", "день вне диапазона 1–%d: %d", last, d.Day)
		}
	}

	return d.validateUpper()
}

// validateUpper проверяет верхнюю границу: она допустима только при
// modifier=between и не может заканчиваться раньше начала нижней.
func (d FactDate) validateUpper() *ValidationError {
	if d.Modifier != ModifierBetween {
		for _, c := range []struct {
			field string
			value int
		}{
			{"year_to", d.YearTo}, {"month_to", d.MonthTo}, {"day_to", d.DayTo},
		} {
			if c.value != 0 {
				return fieldErr(c.field, "верхняя граница допустима только при modifier=between")
			}
		}

		return nil
	}

	if d.YearTo < 1 || d.YearTo > 9999 {
		return fieldErr("year_to", "год верхней границы вне диапазона 1–9999: %d", d.YearTo)
	}
	if d.MonthTo != 0 && (d.MonthTo < 1 || d.MonthTo > 12) {
		return fieldErr("month_to", "месяц верхней границы вне диапазона 1–12: %d", d.MonthTo)
	}
	if d.DayTo != 0 {
		if d.MonthTo == 0 {
			return fieldErr("day_to", "день верхней границы задан без месяца")
		}
		if last := daysInMonth(d.YearTo, d.MonthTo, d.Calendar); d.DayTo < 1 || d.DayTo > last {
			return fieldErr("day_to", "день верхней границы вне диапазона 1–%d: %d", last, d.DayTo)
		}
	}

	// Конец верхней границы (неполные компоненты — до конца периода) не раньше
	// начала нижней (неполные компоненты — с начала периода).
	endMonth, endDay := d.MonthTo, d.DayTo
	if endMonth == 0 {
		endMonth = 12
	}
	if endDay == 0 {
		endDay = daysInMonth(d.YearTo, endMonth, d.Calendar)
	}
	startMonth, startDay := d.Month, d.Day
	if startMonth == 0 {
		startMonth = 1
	}
	if startDay == 0 {
		startDay = 1
	}
	if ord(d.YearTo, endMonth, endDay) < ord(d.Year, startMonth, startDay) {
		return fieldErr("year_to", "верхняя граница раньше нижней")
	}

	return nil
}

// daysInMonth — число дней месяца с учётом календаря. Григорианский — по
// григорианскому правилу високосности; юлианский и не заданный — по
// юлианскому (для не заданного календаря это снисходительное допущение).
func daysInMonth(year, month int, cal FactCalendar) int {
	if month == 2 {
		leap := year%4 == 0
		if cal == FactCalendarGregorian {
			leap = leap && (year%100 != 0 || year%400 == 0)
		}
		if leap {
			return 29
		}

		return 28
	}

	return julianMonthDays(year, month)
}

// validatePeriod проверяет период «с … по …»: обе даты (если заданы) корректны
// и начало не позже конца (по FactDate.Compare; неопределимое допускается).
func validatePeriod(since, until *FactDate) *ValidationError {
	if since != nil {
		if e := since.validate(); e != nil {
			return e.within("since")
		}
	}
	if until != nil {
		if e := until.validate(); e != nil {
			return e.within("until")
		}
	}
	if since != nil && until != nil && since.Compare(*until) == CompareLater {
		return fieldErr("since", "начало периода позже конца")
	}

	return nil
}
```

- [ ] **Step 4: Прогнать тесты**

Run: `gofmt -l internal/models` → пусто.
Run: `go vet ./internal/models && go test ./internal/models`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/models/fact_date_validate.go internal/models/fact_date_validate_test.go
git commit -m "feat(models): FactDate.Validate и проверка периодов

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

### Task 3: `TextRef.Validate` и `SourceLink.Validate`

**Files:**
- Create: `internal/models/text_ref_validate.go`
- Create: `internal/models/source_link_validate.go`
- Test: `internal/models/refs_validate_test.go`

**Interfaces:**
- Consumes: Task 1–2 (`fieldErr`, `within`, `indexed`, `finish`, `Valid()`), `ID.Validate`, `TextRef`, `SourceLink`.
- Produces:
  - `func (r TextRef) Validate() error`; внутренние `(r TextRef) validateAs(want Type) *ValidationError` (`want` пусто — любой известный тип), `(r TextRef) isZero() bool`, `validateOptionalTextRef(r TextRef, want Type) *ValidationError`, `validateTextRefs(field string, refs []TextRef, want Type) *ValidationError`;
  - `func (l SourceLink) Validate() error`; внутренние `(l SourceLink) validate() *ValidationError`, `validateSourceLinks(field string, links []SourceLink) *ValidationError`.

- [ ] **Step 1: Написать падающие тесты**

`internal/models/refs_validate_test.go`:

```go
package models

import "testing"

func TestTextRefValidate(t *testing.T) {
	person := testID(TypePerson)

	valid := map[string]TextRef{
		"только текст":      {Text: "Давыдово"},
		"ссылка с текстом":  {Text: "Иван", Ref: person, Type: TypePerson},
		"ссылка без текста": {Ref: person, Type: TypePerson},
	}
	for name, r := range valid {
		if err := r.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	invalid := []struct {
		name  string
		r     TextRef
		field string
	}{
		{"пустой", TextRef{}, "text"},
		{"только пробелы", TextRef{Text: "  "}, "text"},
		{"тип без ссылки", TextRef{Text: "x", Type: TypePerson}, "type"},
		{"ссылка без типа", TextRef{Text: "x", Ref: person}, "type"},
		{"неизвестный тип", TextRef{Ref: person, Type: "nonsense"}, "type"},
		{"формат ссылки", TextRef{Ref: "p-1", Type: TypePerson}, "ref"},
		{"префикс не того типа", TextRef{Ref: person, Type: TypeFamily}, "ref"},
	}
	for _, tt := range invalid {
		wantInvalid(t, tt.name, tt.r.Validate(), "", tt.field)
	}
}

func TestTextRefValidateAs(t *testing.T) {
	surname := testID(TypeSurname)
	r := TextRef{Text: "Иванов", Ref: surname, Type: TypeSurname}
	if e := r.validateAs(TypeSurname); e != nil {
		t.Errorf("ожидаемый тип: %v", e)
	}
	if e := r.validateAs(""); e != nil {
		t.Errorf("любой тип: %v", e)
	}
	wantInvalid(t, "другой тип", finish("", r.validateAs(TypeGivenName)), "", "type")

	// Ожидаемый тип не требует ссылки: обычный текст допустим.
	if e := (TextRef{Text: "Иван"}).validateAs(TypeGivenName); e != nil {
		t.Errorf("текст без ссылки: %v", e)
	}
}

func TestOptionalAndListTextRefs(t *testing.T) {
	if e := validateOptionalTextRef(TextRef{}, TypeSurname); e != nil {
		t.Errorf("пустая необязательная часть: %v", e)
	}
	wantInvalid(t, "необязательная, но битая",
		finish("", validateOptionalTextRef(TextRef{Text: " "}, "")), "", "text")

	refs := []TextRef{{Text: "a"}, {Text: "b"}, {}}
	wantInvalid(t, "список", finish("", validateTextRefs("notes", refs, "")), "", "notes[2].text")
	if e := validateTextRefs("notes", refs[:2], ""); e != nil {
		t.Errorf("корректный список: %v", e)
	}
	if e := validateTextRefs("notes", nil, ""); e != nil {
		t.Errorf("пустой список: %v", e)
	}
}

func TestTextRefIsZero(t *testing.T) {
	if !(TextRef{}).isZero() {
		t.Error("пустой TextRef должен быть zero")
	}
	for _, r := range []TextRef{{Text: "x"}, {Ref: "x"}, {Type: TypePerson}} {
		if r.isZero() {
			t.Errorf("%+v не должен быть zero", r)
		}
	}
}

func TestSourceLinkValidate(t *testing.T) {
	cit := testID(TypeCitation)

	valid := map[string]SourceLink{
		"минимум":          {CitationID: cit},
		"с достоверностью": {CitationID: cit, Reliability: ReliabilityPrimary, Role: "имя", Note: "л. 12"},
		// TargetType/TargetID выставляет адаптер из владельца — не проверяются.
		"цели игнорируются": {CitationID: cit, TargetType: "nonsense", TargetID: "мусор"},
	}
	for name, l := range valid {
		if err := l.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	invalid := []struct {
		name  string
		l     SourceLink
		field string
	}{
		{"пустая цитата", SourceLink{}, "citation_id"},
		{"плохой формат цитаты", SourceLink{CitationID: "c-1"}, "citation_id"},
		{"цитата не того типа", SourceLink{CitationID: testID(TypeSource)}, "citation_id"},
		{"плохая достоверность", SourceLink{CitationID: cit, Reliability: "maybe"}, "reliability"},
	}
	for _, tt := range invalid {
		wantInvalid(t, tt.name, tt.l.Validate(), "", tt.field)
	}

	links := []SourceLink{{CitationID: cit}, {CitationID: "bad"}}
	wantInvalid(t, "список", finish("", validateSourceLinks("sources", links)), "", "sources[1].citation_id")
	if e := validateSourceLinks("sources", links[:1]); e != nil {
		t.Errorf("корректный список: %v", e)
	}
}
```

- [ ] **Step 2: Убедиться, что тесты не компилируются**

Run: `go test ./internal/models`
Expected: FAIL — `r.Validate undefined`, `undefined: validateTextRefs`, `undefined: validateSourceLinks`.

- [ ] **Step 3: `TextRef`**

`internal/models/text_ref_validate.go`:

```go
package models

import "strings"

// Validate проверяет «текст-или-ссылку»: без ссылки нужен непустой текст и не
// задан тип; со ссылкой тип обязателен, известен и совпадает с префиксом Ref.
func (r TextRef) Validate() error {
	return finish("", r.validateAs(""))
}

// validateAs — Validate с ожидаемым типом ссылки; want пусто — любой известный
// тип. Ожидаемый тип не требует наличия ссылки: обычный текст допустим всегда.
func (r TextRef) validateAs(want Type) *ValidationError {
	if r.Ref == "" {
		if strings.TrimSpace(r.Text) == "" {
			return fieldErr("text", "текст обязателен, если нет ссылки")
		}
		if r.Type != "" {
			return fieldErr("type", "тип задаётся только вместе со ссылкой")
		}

		return nil
	}

	if r.Type == "" {
		return fieldErr("type", "при ссылке тип обязателен")
	}
	if !r.Type.Valid() {
		return fieldErr("type", "неизвестный тип сущности %q", r.Type)
	}
	if want != "" && r.Type != want {
		return fieldErr("type", "ожидается тип %s, получен %s", want, r.Type)
	}
	if err := r.Ref.Validate(r.Type); err != nil {
		return fieldErr("ref", "%v", err)
	}

	return nil
}

// isZero сообщает, что TextRef не заполнен (часть имени отсутствует).
func (r TextRef) isZero() bool {
	return r.Text == "" && r.Ref == "" && r.Type == ""
}

// validateOptionalTextRef проверяет необязательное поле-TextRef: незаполненное
// допустимо, заполненное должно быть корректным.
func validateOptionalTextRef(r TextRef, want Type) *ValidationError {
	if r.isZero() {
		return nil
	}

	return r.validateAs(want)
}

// validateTextRefs проверяет список TextRef; путь ошибки — field[i].….
func validateTextRefs(field string, refs []TextRef, want Type) *ValidationError {
	for i, r := range refs {
		if e := r.validateAs(want); e != nil {
			return e.within(indexed(field, i))
		}
	}

	return nil
}
```

- [ ] **Step 4: `SourceLink`**

`internal/models/source_link_validate.go`:

```go
package models

// Validate проверяет доказательство: цитата обязательна и имеет тип citation;
// достоверность необязательна, но если задана — одна из констант. TargetType и
// TargetID выставляет адаптер из владельца при записи, здесь не проверяются.
func (l SourceLink) Validate() error {
	return finish("", l.validate())
}

func (l SourceLink) validate() *ValidationError {
	if err := l.CitationID.Validate(TypeCitation); err != nil {
		return fieldErr("citation_id", "%v", err)
	}
	if l.Reliability != "" && !l.Reliability.Valid() {
		return fieldErr("reliability", "недопустимая достоверность %q", l.Reliability)
	}

	return nil
}

// validateSourceLinks проверяет список доказательств; путь ошибки — field[i].….
func validateSourceLinks(field string, links []SourceLink) *ValidationError {
	for i, l := range links {
		if e := l.validate(); e != nil {
			return e.within(indexed(field, i))
		}
	}

	return nil
}
```

- [ ] **Step 5: Прогнать тесты**

Run: `gofmt -l internal/models` → пусто.
Run: `go vet ./internal/models && go test ./internal/models`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/models/text_ref_validate.go internal/models/source_link_validate.go internal/models/refs_validate_test.go
git commit -m "feat(models): проверки TextRef и SourceLink

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

### Task 4: `Person` и `PersonName`

**Files:**
- Create: `internal/models/person_validate.go`
- Create: `internal/models/person_name_validate.go`
- Test: `internal/models/person_validate_test.go`

**Interfaces:**
- Consumes: Task 1–3.
- Produces: `func (p *Person) Validate() error`; `func (n PersonName) Validate() error`; внутренние `(p *Person) validate()`, `(n PersonName) validate()`.

Ошибки `Person.Validate` несут `Entity == TypePerson`; `PersonName.Validate` (отдельно) — пустую сущность.

- [ ] **Step 1: Написать падающие тесты**

`internal/models/person_validate_test.go`:

```go
package models

import "testing"

// validPerson — образец корректной персоны; тесты портят по одному полю.
func validPerson() *Person {
	return &Person{
		ID:     testID(TypePerson),
		Gender: Female,
		Names: []PersonName{{
			Type:    PersonNameMain,
			Surname: TextRef{Text: "Дорожкина", Ref: testID(TypeSurname), Type: TypeSurname},
			Given:   TextRef{Text: "Акилина"},
			Prefix:  "фон",
			Since:   &FactDate{Year: 1881, Precision: PrecisionYear, Modifier: ModifierExact},
		}},
		Estates:   []TextRef{{Text: "крестьяне", Ref: testID(TypeEstate), Type: TypeEstate}},
		Titles:    []TextRef{{Text: "вдова"}},
		Nicknames: []TextRef{{Text: "Акилинушка"}},
		Notes:     []TextRef{{Text: "жила в Давыдове"}},
		Sources:   []SourceLink{{CitationID: testID(TypeCitation), Reliability: ReliabilityPrimary}},
		Private:   true,
	}
}

func TestPersonValidateOK(t *testing.T) {
	if err := validPerson().Validate(); err != nil {
		t.Fatalf("образец должен быть валиден: %v", err)
	}

	// Все поля, кроме id, необязательны (people.md).
	if err := (&Person{ID: testID(TypePerson)}).Validate(); err != nil {
		t.Errorf("персона только с id: %v", err)
	}
}

func TestPersonValidateErrors(t *testing.T) {
	past := &FactDate{Year: 1881, Precision: PrecisionYear, Modifier: ModifierExact}
	future := &FactDate{Year: 1900, Precision: PrecisionYear, Modifier: ModifierExact}

	tests := []struct {
		name   string
		mutate func(*Person)
		field  string
	}{
		{"пустой id", func(p *Person) { p.ID = "" }, "id"},
		{"плохой формат id", func(p *Person) { p.ID = "p-1" }, "id"},
		{"id другого типа", func(p *Person) { p.ID = testID(TypeFamily) }, "id"},
		{"плохой пол", func(p *Person) { p.Gender = "x" }, "gender"},
		{"имя без частей", func(p *Person) { p.Names[0] = PersonName{Type: PersonNameMain} }, "names[0]"},
		{"плохой тип имени", func(p *Person) { p.Names[0].Type = "nick" }, "names[0].type"},
		{"фамилия — ссылка на персону", func(p *Person) {
			p.Names[0].Surname = TextRef{Ref: testID(TypePerson), Type: TypePerson}
		}, "names[0].surname.type"},
		{"имя — ссылка на фамилию", func(p *Person) {
			p.Names[0].Given = TextRef{Ref: testID(TypeSurname), Type: TypeSurname}
		}, "names[0].given.type"},
		{"отчество — битый текст", func(p *Person) { p.Names[0].Patronymic = TextRef{Text: "  "} }, "names[0].patronymic.text"},
		{"плохое начало имени", func(p *Person) { p.Names[0].Since = &FactDate{} }, "names[0].since.precision"},
		{"начало имени позже конца", func(p *Person) { p.Names[0].Since, p.Names[0].Until = future, past }, "names[0].since"},
		{"второе имя пустое", func(p *Person) { p.Names = append(p.Names, PersonName{}) }, "names[1]"},
		{"сословие — не estate", func(p *Person) {
			p.Estates[0] = TextRef{Ref: testID(TypeTitle), Type: TypeTitle}
		}, "estates[0].type"},
		{"титул — не title", func(p *Person) {
			p.Titles[0] = TextRef{Ref: testID(TypeEstate), Type: TypeEstate}
		}, "titles[0].type"},
		{"пустое прозвище", func(p *Person) { p.Nicknames = append(p.Nicknames, TextRef{}) }, "nicknames[1].text"},
		{"битая ссылка в заметке", func(p *Person) { p.Notes[0] = TextRef{Ref: "x", Type: TypeNote} }, "notes[0].ref"},
		{"плохая цитата", func(p *Person) { p.Sources[0].CitationID = "c-1" }, "sources[0].citation_id"},
		{"плохая достоверность", func(p *Person) { p.Sources[0].Reliability = "maybe" }, "sources[0].reliability"},
	}
	for _, tt := range tests {
		p := validPerson()
		tt.mutate(p)
		wantInvalid(t, tt.name, p.Validate(), TypePerson, tt.field)
	}
}

func TestPersonNameValidateStandalone(t *testing.T) {
	ok := PersonName{Given: TextRef{Text: "Акилина"}}
	if err := ok.Validate(); err != nil {
		t.Errorf("имя только с именем: %v", err)
	}
	// Отчество без имени и фамилии — тоже имя (части необязательны, но хотя бы одна нужна).
	if err := (PersonName{Patronymic: TextRef{Text: "Ивановна"}}).Validate(); err != nil {
		t.Errorf("имя только с отчеством: %v", err)
	}

	wantInvalid(t, "префикс и суффикс — не имя",
		PersonName{Prefix: "фон", Suffix: "мл."}.Validate(), "", "")
	wantInvalid(t, "плохой тип", PersonName{Type: "x", Given: TextRef{Text: "А"}}.Validate(), "", "type")
}
```

- [ ] **Step 2: Убедиться, что тесты не компилируются**

Run: `go test ./internal/models`
Expected: FAIL — `p.Validate undefined`, `PersonName.Validate undefined`.

- [ ] **Step 3: `PersonName`**

`internal/models/person_name_validate.go`:

```go
package models

// Validate проверяет одно имя персоны отдельно от неё.
func (n PersonName) Validate() error {
	return finish("", n.validate())
}

// validate: вид имени необязателен, но если задан — константа; хотя бы одна из
// частей (фамилия, имя, отчество) заполнена; ссылки частей — на словари
// ожидаемых типов; период корректен.
func (n PersonName) validate() *ValidationError {
	if n.Type != "" && !n.Type.Valid() {
		return fieldErr("type", "недопустимый вид имени %q", n.Type)
	}

	if n.Surname.isZero() && n.Given.isZero() && n.Patronymic.isZero() {
		return fieldErr("", "нужна хотя бы одна часть имени: фамилия, имя или отчество")
	}
	if e := validateOptionalTextRef(n.Surname, TypeSurname); e != nil {
		return e.within("surname")
	}
	if e := validateOptionalTextRef(n.Given, TypeGivenName); e != nil {
		return e.within("given")
	}
	if e := validateOptionalTextRef(n.Patronymic, TypePatronymic); e != nil {
		return e.within("patronymic")
	}

	return validatePeriod(n.Since, n.Until)
}
```

- [ ] **Step 4: `Person`**

`internal/models/person_validate.go`:

```go
package models

// Validate проверяет персону. Все поля, кроме id, необязательны; заданные
// должны быть корректны.
func (p *Person) Validate() error {
	return finish(TypePerson, p.validate())
}

func (p *Person) validate() *ValidationError {
	if err := p.ID.Validate(TypePerson); err != nil {
		return fieldErr("id", "%v", err)
	}
	if p.Gender != "" && !p.Gender.Valid() {
		return fieldErr("gender", "недопустимый пол %q", p.Gender)
	}

	for i := range p.Names {
		if e := p.Names[i].validate(); e != nil {
			return e.within(indexed("names", i))
		}
	}

	if e := validateTextRefs("estates", p.Estates, TypeEstate); e != nil {
		return e
	}
	if e := validateTextRefs("titles", p.Titles, TypeTitle); e != nil {
		return e
	}
	if e := validateTextRefs("nicknames", p.Nicknames, ""); e != nil {
		return e
	}
	if e := validateTextRefs("notes", p.Notes, ""); e != nil {
		return e
	}

	return validateSourceLinks("sources", p.Sources)
}
```

- [ ] **Step 5: Прогнать тесты**

Run: `gofmt -l internal/models` → пусто.
Run: `go vet ./internal/models && go test ./internal/models`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/models/person_validate.go internal/models/person_name_validate.go internal/models/person_validate_test.go
git commit -m "feat(models): Validate для Person и PersonName

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

### Task 5: `Relation`, `Residence`, `Family`

**Files:**
- Create: `internal/models/relation_validate.go`
- Create: `internal/models/residence_validate.go`
- Create: `internal/models/family_validate.go`
- Test: `internal/models/graph_validate_test.go`

**Interfaces:**
- Consumes: Task 1–3; `validOpenEnum`, `validatePeriod`.
- Produces: `func (r *Relation) Validate() error`, `func (r *Residence) Validate() error`, `func (f *Family) Validate() error` (ошибки несут `Entity` = `TypeRelation` / `TypeResidence` / `TypeFamily`).

- [ ] **Step 1: Написать падающие тесты**

`internal/models/graph_validate_test.go`:

```go
package models

import "testing"

func validRelation() *Relation {
	return &Relation{
		ID:      testID(TypeRelation),
		Kind:    RelationKindBlood,
		PersonA: testID(TypePerson),
		PersonB: personBID(),
		Since:   &FactDate{Year: 1881, Precision: PrecisionYear, Modifier: ModifierExact},
		Sources: []SourceLink{{CitationID: testID(TypeCitation)}},
		Notes:   []TextRef{{Text: "по метрике"}},
	}
}

// personBID — другой валидный идентификатор персоны (иное тело ULID).
func personBID() ID {
	id, err := BuildID(TypePerson, "01J8X4T0K2M9Q7R5V3B6N8C1D5")
	if err != nil {
		panic(err)
	}

	return id
}

func TestRelationValidateOK(t *testing.T) {
	if err := validRelation().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}

	assoc := validRelation()
	assoc.Kind, assoc.RelType = RelationKindAssociate, "godparent"
	if err := assoc.Validate(); err != nil {
		t.Errorf("associate с rel_type: %v", err)
	}

	for _, kind := range []RelationKind{RelationKindMarriage, RelationKindAdoption} {
		r := validRelation()
		r.Kind = kind
		if err := r.Validate(); err != nil {
			t.Errorf("kind=%s: %v", kind, err)
		}
	}
}

func TestRelationValidateErrors(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Relation)
		field  string
	}{
		{"плохой id", func(r *Relation) { r.ID = "r-1" }, "id"},
		{"пустой kind", func(r *Relation) { r.Kind = "" }, "kind"},
		{"плохой kind", func(r *Relation) { r.Kind = "friend" }, "kind"},
		{"associate без rel_type", func(r *Relation) { r.Kind = RelationKindAssociate }, "rel_type"},
		{"associate с плохим rel_type", func(r *Relation) { r.Kind, r.RelType = RelationKindAssociate, "Godparent" }, "rel_type"},
		{"rel_type у blood", func(r *Relation) { r.RelType = "neighbor" }, "rel_type"},
		{"person_a не персона", func(r *Relation) { r.PersonA = testID(TypeFamily) }, "person_a"},
		{"пустой person_b", func(r *Relation) { r.PersonB = "" }, "person_b"},
		{"связь с самой собой", func(r *Relation) { r.PersonB = r.PersonA }, "person_b"},
		{"начало позже конца", func(r *Relation) {
			r.Since = &FactDate{Year: 1900, Precision: PrecisionYear, Modifier: ModifierExact}
			r.Until = &FactDate{Year: 1881, Precision: PrecisionYear, Modifier: ModifierExact}
		}, "since"},
		{"плохая цитата", func(r *Relation) { r.Sources[0].CitationID = "x" }, "sources[0].citation_id"},
		{"пустая заметка", func(r *Relation) { r.Notes = append(r.Notes, TextRef{}) }, "notes[1].text"},
	}
	for _, tt := range tests {
		r := validRelation()
		tt.mutate(r)
		wantInvalid(t, tt.name, r.Validate(), TypeRelation, tt.field)
	}
}

func validResidence() *Residence {
	return &Residence{
		ID:       testID(TypeResidence),
		PersonID: testID(TypePerson),
		PlaceID:  testID(TypeAdministrativeDivision),
		Since:    &FactDate{Year: 1881, Precision: PrecisionYear, Modifier: ModifierExact},
		Until:    &FactDate{Year: 1900, Precision: PrecisionYear, Modifier: ModifierExact},
		Sources:  []SourceLink{{CitationID: testID(TypeCitation)}},
		Note:     "дом у церкви",
	}
}

func TestResidenceValidate(t *testing.T) {
	if err := validResidence().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Residence)
		field  string
	}{
		{"плохой id", func(r *Residence) { r.ID = "" }, "id"},
		{"person_id не персона", func(r *Residence) { r.PersonID = testID(TypeEvent) }, "person_id"},
		{"пустой place_id", func(r *Residence) { r.PlaceID = "" }, "place_id"},
		{"place_id не деление", func(r *Residence) { r.PlaceID = testID(TypeChurch) }, "place_id"},
		{"плохое начало", func(r *Residence) { r.Since = &FactDate{Year: 0, Precision: PrecisionYear, Modifier: ModifierExact} }, "since.year"},
		{"начало позже конца", func(r *Residence) { r.Since, r.Until = r.Until, r.Since }, "since"},
		{"плохая цитата", func(r *Residence) { r.Sources[0].CitationID = "" }, "sources[0].citation_id"},
	}
	for _, tt := range tests {
		r := validResidence()
		tt.mutate(r)
		wantInvalid(t, tt.name, r.Validate(), TypeResidence, tt.field)
	}
}

func validFamily() *Family {
	return &Family{
		ID:   testID(TypeFamily),
		Name: "Дорожкины",
		Members: []TextRef{
			{Text: "Иван", Ref: testID(TypePerson), Type: TypePerson},
			{Text: "какая-то Мария"},
		},
		Notes:   []TextRef{{Text: "из Давыдова"}},
		Sources: []SourceLink{{CitationID: testID(TypeCitation)}},
	}
}

func TestFamilyValidate(t *testing.T) {
	if err := validFamily().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Family)
		field  string
	}{
		{"плохой id", func(f *Family) { f.ID = "F" }, "id"},
		{"пустое имя", func(f *Family) { f.Name = "" }, "name"},
		{"имя из пробелов", func(f *Family) { f.Name = "   " }, "name"},
		{"пустой член рода", func(f *Family) { f.Members = append(f.Members, TextRef{}) }, "members[2].text"},
		{"член рода — битая ссылка", func(f *Family) { f.Members[0].Ref = "x" }, "members[0].ref"},
		{"пустая заметка", func(f *Family) { f.Notes[0] = TextRef{} }, "notes[0].text"},
		{"плохая цитата", func(f *Family) { f.Sources[0].CitationID = "c" }, "sources[0].citation_id"},
	}
	for _, tt := range tests {
		f := validFamily()
		tt.mutate(f)
		wantInvalid(t, tt.name, f.Validate(), TypeFamily, tt.field)
	}
}
```

- [ ] **Step 2: Убедиться, что тесты не компилируются**

Run: `go test ./internal/models`
Expected: FAIL — `r.Validate undefined` для `Relation`, `Residence`, `Family`.

- [ ] **Step 3: `Relation`**

`internal/models/relation_validate.go`:

```go
package models

// Validate проверяет ребро графа: вид обязателен и допустим; rel_type — только
// для kind=associate (обязателен и в формате открытого enum'а), у остальных
// видов пуст; стороны — персоны и не совпадают; период корректен.
func (r *Relation) Validate() error {
	return finish(TypeRelation, r.validate())
}

func (r *Relation) validate() *ValidationError {
	if err := r.ID.Validate(TypeRelation); err != nil {
		return fieldErr("id", "%v", err)
	}
	if !r.Kind.Valid() {
		return fieldErr("kind", "недопустимый вид связи %q", r.Kind)
	}

	if r.Kind == RelationKindAssociate {
		if r.RelType == "" {
			return fieldErr("rel_type", "обязателен при kind=associate")
		}
		if !validOpenEnum(string(r.RelType)) {
			return fieldErr("rel_type", "недопустимый формат %q: ожидается [a-z][a-z0-9_-]*", r.RelType)
		}
	} else if r.RelType != "" {
		return fieldErr("rel_type", "допустим только при kind=associate")
	}

	if err := r.PersonA.Validate(TypePerson); err != nil {
		return fieldErr("person_a", "%v", err)
	}
	if err := r.PersonB.Validate(TypePerson); err != nil {
		return fieldErr("person_b", "%v", err)
	}
	if r.PersonA == r.PersonB {
		return fieldErr("person_b", "связь персоны с самой собой")
	}

	if e := validatePeriod(r.Since, r.Until); e != nil {
		return e
	}
	if e := validateSourceLinks("sources", r.Sources); e != nil {
		return e
	}

	return validateTextRefs("notes", r.Notes, "")
}
```

- [ ] **Step 4: `Residence`**

`internal/models/residence_validate.go`:

```go
package models

// Validate проверяет проживание: персона и место (единица административного
// деления) — корректные ссылки, период корректен.
func (r *Residence) Validate() error {
	return finish(TypeResidence, r.validate())
}

func (r *Residence) validate() *ValidationError {
	if err := r.ID.Validate(TypeResidence); err != nil {
		return fieldErr("id", "%v", err)
	}
	if err := r.PersonID.Validate(TypePerson); err != nil {
		return fieldErr("person_id", "%v", err)
	}
	if err := r.PlaceID.Validate(TypeAdministrativeDivision); err != nil {
		return fieldErr("place_id", "%v", err)
	}

	if e := validatePeriod(r.Since, r.Until); e != nil {
		return e
	}

	return validateSourceLinks("sources", r.Sources)
}
```

- [ ] **Step 5: `Family`**

`internal/models/family_validate.go`:

```go
package models

import "strings"

// Validate проверяет род: название обязательно (непустое после обрезки
// пробелов); члены — «текст или ссылка» (ссылка на персону либо строка-имя).
func (f *Family) Validate() error {
	return finish(TypeFamily, f.validate())
}

func (f *Family) validate() *ValidationError {
	if err := f.ID.Validate(TypeFamily); err != nil {
		return fieldErr("id", "%v", err)
	}
	if strings.TrimSpace(f.Name) == "" {
		return fieldErr("name", "название рода обязательно")
	}

	if e := validateTextRefs("members", f.Members, ""); e != nil {
		return e
	}
	if e := validateTextRefs("notes", f.Notes, ""); e != nil {
		return e
	}

	return validateSourceLinks("sources", f.Sources)
}
```

- [ ] **Step 6: Прогнать тесты**

Run: `gofmt -l internal/models` → пусто.
Run: `go vet ./internal/models && go test ./internal/models`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/models/relation_validate.go internal/models/residence_validate.go internal/models/family_validate.go internal/models/graph_validate_test.go
git commit -m "feat(models): Validate для Relation, Residence и Family

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

### Task 6: Словари, документация и рубеж

**Files:**
- Create: `internal/models/dictionary_validate.go`
- Test: `internal/models/dictionary_validate_test.go`
- Modify: `docs/data-model/core-read-write.md` (§2.3)

**Interfaces:**
- Consumes: Task 1–3.
- Produces: `Validate() error` у `*Surname`, `*GivenName`, `*Patronymic`, `*Estate`, `*Title`; внутренняя `validateDictionary(t Type, id ID, canonical string, variants, items, notes []TextRef) *ValidationError`.

- [ ] **Step 1: Написать падающие тесты**

`internal/models/dictionary_validate_test.go`:

```go
package models

import "testing"

var dictionaryTypes = []Type{TypeSurname, TypeGivenName, TypePatronymic, TypeEstate, TypeTitle}

// Общие правила словарной записи проверяются на validateDictionary для каждого
// из пяти словарей.
func TestValidateDictionaryRules(t *testing.T) {
	for _, typ := range dictionaryTypes {
		id := testID(typ)
		variants := []TextRef{{Text: "вариант"}, {Text: "другой", Ref: testID(typ), Type: typ}}
		items := []TextRef{{Text: "Иван"}, {Text: "Пётр", Ref: testID(TypePerson), Type: TypePerson}}
		notes := []TextRef{{Text: "заметка"}}

		if e := validateDictionary(typ, id, "образец", variants, items, notes); e != nil {
			t.Errorf("%s: образец должен быть валиден: %v", typ, e)
		}
		if e := validateDictionary(typ, id, "без вариантов", nil, nil, nil); e != nil {
			t.Errorf("%s: запись без списков: %v", typ, e)
		}

		other := TypeSurname
		if typ == TypeSurname {
			other = TypeGivenName
		}

		tests := []struct {
			name      string
			id        ID
			canonical string
			variants  []TextRef
			items     []TextRef
			notes     []TextRef
			field     string
		}{
			{"пустой id", "", "x", nil, nil, nil, "id"},
			{"id другого типа", testID(TypeEvent), "x", nil, nil, nil, "id"},
			{"пустая каноническая", id, "", nil, nil, nil, "canonical"},
			{"каноническая из пробелов", id, "  ", nil, nil, nil, "canonical"},
			{"пустой вариант", id, "x", []TextRef{{Text: "a"}, {}}, nil, nil, "variants[1].text"},
			{"вариант другого типа", id, "x", []TextRef{{Ref: testID(other), Type: other}}, nil, nil, "variants[0].type"},
			{"пустой носитель", id, "x", nil, []TextRef{{}}, nil, "items[0].text"},
			{"пустая заметка", id, "x", nil, nil, []TextRef{{Text: "a"}, {}}, "notes[1].text"},
		}
		for _, tt := range tests {
			e := validateDictionary(typ, tt.id, tt.canonical, tt.variants, tt.items, tt.notes)
			wantInvalid(t, string(typ)+": "+tt.name, finish(typ, e), typ, tt.field)
		}
	}
}

// Публичные Validate передают в общие правила правильный тип сущности.
func TestDictionaryPublicValidate(t *testing.T) {
	type entry struct {
		typ   Type
		build func(id ID) interface{ Validate() error }
	}
	entries := []entry{
		{TypeSurname, func(id ID) interface{ Validate() error } { return &Surname{ID: id, Canonical: "Дорожкин"} }},
		{TypeGivenName, func(id ID) interface{ Validate() error } {
			return &GivenName{ID: id, Canonical: "Иван", Gender: MaleName}
		}},
		{TypePatronymic, func(id ID) interface{ Validate() error } { return &Patronymic{ID: id, Canonical: "Иванович"} }},
		{TypeEstate, func(id ID) interface{ Validate() error } { return &Estate{ID: id, Canonical: "крестьяне"} }},
		{TypeTitle, func(id ID) interface{ Validate() error } { return &Title{ID: id, Canonical: "вдова"} }},
	}
	for _, e := range entries {
		if err := e.build(testID(e.typ)).Validate(); err != nil {
			t.Errorf("%s: %v", e.typ, err)
		}

		wrong := TypeEvent
		wantInvalid(t, string(e.typ)+": чужой id", e.build(testID(wrong)).Validate(), e.typ, "id")
	}
}

func TestGivenNameValidateGender(t *testing.T) {
	base := func() *GivenName {
		return &GivenName{ID: testID(TypeGivenName), Canonical: "Женя", Gender: NeutralName}
	}
	if err := base().Validate(); err != nil {
		t.Errorf("нейтральное имя: %v", err)
	}
	for _, g := range []NameGender{"", "other"} {
		n := base()
		n.Gender = g
		wantInvalid(t, "пол "+string(g), n.Validate(), TypeGivenName, "gender")
	}
}
```

- [ ] **Step 2: Убедиться, что тесты не компилируются**

Run: `go test ./internal/models`
Expected: FAIL — `undefined: validateDictionary`, `s.Validate undefined` (у словарей нет `Validate`).

- [ ] **Step 3: Реализация**

`internal/models/dictionary_validate.go`:

```go
package models

import "strings"

// Validate проверяет запись словаря фамилий.
func (s *Surname) Validate() error {
	return finish(TypeSurname, validateDictionary(TypeSurname, s.ID, s.Canonical, s.Variants, s.Items, s.Notes))
}

// Validate проверяет запись словаря имён; признак пола обязателен.
func (g *GivenName) Validate() error {
	if e := validateDictionary(TypeGivenName, g.ID, g.Canonical, g.Variants, g.Items, g.Notes); e != nil {
		return finish(TypeGivenName, e)
	}
	if !g.Gender.Valid() {
		return finish(TypeGivenName, fieldErr("gender", "обязателен допустимый пол имени, получено %q", g.Gender))
	}

	return nil
}

// Validate проверяет запись словаря отчеств.
func (p *Patronymic) Validate() error {
	return finish(TypePatronymic, validateDictionary(TypePatronymic, p.ID, p.Canonical, p.Variants, p.Items, p.Notes))
}

// Validate проверяет запись словаря сословий.
func (e *Estate) Validate() error {
	return finish(TypeEstate, validateDictionary(TypeEstate, e.ID, e.Canonical, e.Variants, e.Items, e.Notes))
}

// Validate проверяет запись словаря титулов.
func (t *Title) Validate() error {
	return finish(TypeTitle, validateDictionary(TypeTitle, t.ID, t.Canonical, t.Variants, t.Items, t.Notes))
}

// validateDictionary — общие правила словарной записи: id нужного типа,
// непустая каноническая форма, варианты — того же типа, что и словарь,
// носители и заметки — любые корректные TextRef.
func validateDictionary(t Type, id ID, canonical string, variants, items, notes []TextRef) *ValidationError {
	if err := id.Validate(t); err != nil {
		return fieldErr("id", "%v", err)
	}
	if strings.TrimSpace(canonical) == "" {
		return fieldErr("canonical", "каноническая форма обязательна")
	}
	if e := validateTextRefs("variants", variants, t); e != nil {
		return e
	}
	if e := validateTextRefs("items", items, ""); e != nil {
		return e
	}

	return validateTextRefs("notes", notes, "")
}
```

- [ ] **Step 4: Документация: уточнения правил в спеке**

В `docs/data-model/core-read-write.md`, раздел 2.3 «Инварианты (`Validate`)», в конец списка «Сущностные правила» добавь пункты:

```markdown
  - Необязательные поля с закрытыми enum'ами (`Person.Gender`, `PersonName.Type`,
    `FactDate.Calendar`, `SourceLink.Reliability`): пустое значение допустимо,
    непустое — только константа. Обязательные (`Relation.Kind`, `GivenName.Gender`) —
    только константа.
  - `PersonName`: хотя бы одна из частей (фамилия, имя, отчество) заполнена; ссылки
    частей — на словари ожидаемых типов (`surname`, `given_name`, `patronymic`);
    `Person.Estates` ссылаются на `estate`, `Person.Titles` — на `title`, `variants`
    словаря — на словарь того же типа.
  - `FactDate`: точность согласована с компонентами (при `year` месяц и день нулевые,
    при `month` день нулевой, при `unknown` все компоненты нулевые и формулировка
    `exact`); день не больше числа дней месяца в календаре даты (без календаря —
    по юлианскому правилу, снисходительно); верхняя граница только при `between`
    и не заканчивается раньше начала нижней.
  - `Family.Name` и `canonical` словарей обязательны (непустые после обрезки
    пробелов).
```

- [ ] **Step 5: Рубеж**

```bash
gofmt -l .            # пусто
go build ./...
go vet ./...
go test ./...
```

Expected: всё зелёное.

- [ ] **Step 6: Commit**

```bash
git add internal/models/dictionary_validate.go internal/models/dictionary_validate_test.go docs/data-model/core-read-write.md
git commit -m "feat(models): Validate словарей; уточнения правил в спеке

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

## Самопроверка плана

- **Покрытие спеки §2.3 (этап S3):** идентификаторы через `ID.Validate` (Person, Relation, Residence, Family, словари, `TextRef.Ref`, `SourceLink.CitationID`); закрытые enum'ы через `Valid()` (Task 1); открытый enum `RelationType` (Task 5); `Relation`: `associate` ↔ `RelType`, `PersonA != PersonB`; периоды `since ≤ until` через `Compare`; `FactDate` (Task 2); `SourceLink`, `TextRef` (Task 3). Инварианты остальных сущностей — этап S4.
- **Согласованность имён:** `fieldErr`, `within`, `indexed`, `finish`, `validOpenEnum` (Task 1) → `validatePeriod`, `daysInMonth` (Task 2) → `validateAs`, `validateOptionalTextRef`, `validateTextRefs`, `validateSourceLinks`, `isZero` (Task 3) → `validate()` методы Task 4–6.
- **Не входит в этап:** проверки с обращением к хранилищу (существование ссылок, циклы) — S14; `Validate` для `AdministrativeDivision`, `Church`, `Parish`, `Event`, `Source`, `Citation`, `Note`, `Repository`, `Archive`, `ArchiveNode`, `ArchiveDocument`, `Attachment` — S4; вызов `Validate` из сценариев — S14.
