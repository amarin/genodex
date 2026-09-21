# S4: Валидация — остальные сущности — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Добавить `Validate() error` для `AdministrativeDivision`, `Church`, `Parish`, `Event`, `Source`, `Citation`, `Note`, `Repository`, `Archive`, `ArchiveNode`, `ArchiveDocument`, `Attachment` и для value-типов `PlaceRef`, `NamedPeriod`, `Anchor`.

**Architecture:** Продолжение S3: чистые проверки в `internal/models`, внутренние функции возвращают `*ValidationError`, публичный `Validate()` оборачивает через `finish`. Новые общие хелперы: необязательные `ID`/`*ID`/`*TextRef`, обязательный текст, список строк, «родитель не сам себе», проверка якоря. `ArchiveNodeType` — открытый enum; `Archive.System` — только текст.

**Tech Stack:** Go 1.26.4, стандартная библиотека (`net/url`, `mime`).

**Spec:** `docs/data-model/core-read-write.md` §2.3; `docs/models/places.md`, `archives.md`, `evidence.md`, `facts.md`, `notes.md`, `values.md`; `docs/plans/2026-09-20-core-rw-roadmap.md` (этап S4, «Предпосылки из S3»).

## Global Constraints

- Контракт S3 сохраняется: `Validate()` чист (без I/O), возвращает `nil` или `*ValidationError`, первая найденная ошибка, поля snake_case как в `docs/models/*`, списки — `items[0]`, вложенные — через точку.
- Закрытые enum'ы — только константы (`AdminDivisionType`, `SourceKind`, `Reliability`, `AttachmentKind`); открытые — непустое `[a-z][a-z0-9_-]*` (`EventType`, `NoteKind`, `RepositoryType`, `ArchiveNodeType`).
- Идентификаторы — `ID.Validate(Type)`; необязательные — пустое допустимо, непустое проверяется. Идентификатор самой сущности обязателен.
- Ожидаемые типы ссылок: `Settlements`, `Items`, `Successors` (деления) → `administrative_division`; `Church.Parish`, `ArchiveNode.Parish`, `ArchiveDocument.Parish` → `parish`; `Parish.Church` → `church`; `ParentID` у деления, узла, заметки — свой тип и не равен собственному `ID`; `Source.RepositoryID`/`Archive.RepositoryID` → `repository`; `Citation.SourceID` → `source`; `ArchiveNode.ArchiveID` → `archive`; `ArchiveDocument.UnitID`, `Attachment.NodeID` → `archive_node`; `Attachment.DocumentID` → `archive_document`; `EventParticipant.PersonID` → `person`.
- `PlaceRef`: как `TextRef`, а ссылка (если есть) — только на `administrative_division`, `church` или `parish`.
- `NamedPeriod`: `Text` обязателен; `Since`/`Until` — строки в формате `ParseFactDate` (пустое допустимо), результат проходит `FactDate.Validate`, начало не позже конца.
- `Archive.System` — только текст: `Ref` пуст, `Text` непуст (определение системы — built-in данные, не сущность).
- Якорь: `ArchiveAnchor` — `node_id` (`archive_node`, обязателен), `document_id` (`archive_document`, необязателен), `page` ≥ 1, `rect` — свободная строка; `FileAnchor` — `attachment_id` (`attachment`, обязателен), `timecode` свободный; `URLAnchor` — абсолютный `http(s)`-адрес с хостом; пустой якорь допустим; nil-указатель внутри интерфейса и неизвестная реализация — ошибка.
- Сущностные правила (см. Task 3–6); проверки, требующие хранилища (существование, циклы, соответствие якоря `Source.kind`, соответствие уровня узла системе архива), — сценарии (S14), не `models`.
- `models` без JSON-тегов; комментарии и тексты ошибок на русском; один файл — один основной тип, проверки — в `*_validate.go`; существующий код не меняется (`Validate()` пока не вызывается из `store`/сценариев).
- Проверка рубежа: `gofmt -l .` пусто, `go build ./...`, `go vet ./...`, `go test ./...`.

---

### Task 1: Хелперы, `PlaceRef` и `NamedPeriod`

**Files:**
- Create: `internal/models/validate_helpers.go`
- Create: `internal/models/place_ref_validate.go`
- Create: `internal/models/named_period_validate.go`
- Test: `internal/models/validate_helpers_test.go`

**Interfaces:**
- Consumes: S3 (`fieldErr`, `idErr`, `within`, `indexed`, `finish`, `validatePeriod`, `TextRef.validateAs`, `ParseFactDate`).
- Produces:
  - `requireText(field, s string) *ValidationError`;
  - `validateStrings(field string, values []string) *ValidationError`;
  - `validateOptionalID(field string, id ID, want Type) *ValidationError`;
  - `validateOptionalIDPtr(field string, id *ID, want Type) *ValidationError`;
  - `validateNotSelf(self ID, parent *ID) *ValidationError` (поле `parent_id`);
  - `validateOptionalTextRefPtr(field string, r *TextRef, want Type) *ValidationError`;
  - `func (p PlaceRef) Validate() error`, `(p PlaceRef) validate() *ValidationError`;
  - `func (n NamedPeriod) Validate() error`, `(n NamedPeriod) validate() *ValidationError`, `validateRenames(field string, renames []NamedPeriod) *ValidationError`;
  - тестовый хелпер `idPtr(id ID) *ID` (файл `validate_helpers_test.go`).

- [ ] **Step 1: Написать падающие тесты**

`internal/models/validate_helpers_test.go`:

```go
package models

import "testing"

// idPtr возвращает указатель на идентификатор (для необязательных ссылок в тестах).
func idPtr(id ID) *ID { return &id }

func TestRequireText(t *testing.T) {
	if e := requireText("name", "Давыдово"); e != nil {
		t.Errorf("непустой текст: %v", e)
	}
	for _, bad := range []string{"", "   ", "\t\n"} {
		wantInvalid(t, "пустой "+bad, finish("", requireText("name", bad)), "", "name")
	}
}

func TestValidateStrings(t *testing.T) {
	if e := validateStrings("variants", nil); e != nil {
		t.Errorf("пустой список: %v", e)
	}
	if e := validateStrings("variants", []string{"Давыдово", "Давидово"}); e != nil {
		t.Errorf("корректный список: %v", e)
	}
	wantInvalid(t, "пустой элемент",
		finish("", validateStrings("variants", []string{"a", " ", ""})), "", "variants[1]")
}

func TestValidateOptionalIDs(t *testing.T) {
	person := testID(TypePerson)

	if e := validateOptionalID("repository_id", "", TypeRepository); e != nil {
		t.Errorf("пустой необязательный id: %v", e)
	}
	if e := validateOptionalID("repository_id", testID(TypeRepository), TypeRepository); e != nil {
		t.Errorf("корректный id: %v", e)
	}
	wantInvalid(t, "плохой формат", finish("", validateOptionalID("repository_id", "R-1", TypeRepository)), "", "repository_id")
	wantInvalid(t, "не тот тип", finish("", validateOptionalID("repository_id", person, TypeRepository)), "", "repository_id")

	if e := validateOptionalIDPtr("parent_id", nil, TypeNote); e != nil {
		t.Errorf("nil-указатель: %v", e)
	}
	if e := validateOptionalIDPtr("parent_id", idPtr(testID(TypeNote)), TypeNote); e != nil {
		t.Errorf("корректный указатель: %v", e)
	}
	// Указатель на пустой id — «задан, но пуст»: это ошибка.
	wantInvalid(t, "указатель на пустой", finish("", validateOptionalIDPtr("parent_id", idPtr(""), TypeNote)), "", "parent_id")
	wantInvalid(t, "указатель не того типа", finish("", validateOptionalIDPtr("parent_id", idPtr(person), TypeNote)), "", "parent_id")
}

func TestValidateNotSelf(t *testing.T) {
	self := testID(TypeNote)
	if e := validateNotSelf(self, nil); e != nil {
		t.Errorf("без родителя: %v", e)
	}
	if e := validateNotSelf(self, idPtr(personBID())); e != nil {
		t.Errorf("другой родитель: %v", e)
	}
	wantInvalid(t, "сам себе родитель", finish("", validateNotSelf(self, idPtr(self))), "", "parent_id")
}

func TestValidateOptionalTextRefPtr(t *testing.T) {
	if e := validateOptionalTextRefPtr("parish", nil, TypeParish); e != nil {
		t.Errorf("nil: %v", e)
	}
	ok := &TextRef{Text: "Никольский приход", Ref: testID(TypeParish), Type: TypeParish}
	if e := validateOptionalTextRefPtr("parish", ok, TypeParish); e != nil {
		t.Errorf("корректная ссылка: %v", e)
	}
	if e := validateOptionalTextRefPtr("parish", &TextRef{Text: "просто текст"}, TypeParish); e != nil {
		t.Errorf("текст без ссылки: %v", e)
	}
	// Указатель задан, но TextRef пуст — ошибка (в отличие от значения-части имени).
	wantInvalid(t, "пустой TextRef", finish("", validateOptionalTextRefPtr("parish", &TextRef{}, TypeParish)), "", "parish.text")
	wantInvalid(t, "не тот тип",
		finish("", validateOptionalTextRefPtr("parish", &TextRef{Ref: testID(TypeChurch), Type: TypeChurch}, TypeParish)), "", "parish.type")
}

func TestPlaceRefValidate(t *testing.T) {
	for _, typ := range []Type{TypeAdministrativeDivision, TypeChurch, TypeParish} {
		p := PlaceRef{Text: "место", Ref: testID(typ), Type: typ}
		if err := p.Validate(); err != nil {
			t.Errorf("ссылка на %s: %v", typ, err)
		}
	}
	if err := (PlaceRef{Text: "Давыдово (нет сущности)"}).Validate(); err != nil {
		t.Errorf("свободный текст: %v", err)
	}

	invalid := []struct {
		name  string
		p     PlaceRef
		field string
	}{
		{"пустое", PlaceRef{}, "text"},
		{"ссылка на персону", PlaceRef{Ref: testID(TypePerson), Type: TypePerson}, "type"},
		{"тип без ссылки", PlaceRef{Text: "x", Type: TypeParish}, "type"},
		{"плохой формат ссылки", PlaceRef{Ref: "AD-1", Type: TypeAdministrativeDivision}, "ref"},
		{"префикс не того типа", PlaceRef{Ref: testID(TypeChurch), Type: TypeParish}, "ref"},
	}
	for _, tt := range invalid {
		wantInvalid(t, tt.name, tt.p.Validate(), "", tt.field)
	}
}

func TestNamedPeriodValidate(t *testing.T) {
	valid := map[string]NamedPeriod{
		"с периодом":      {Text: "Петроград", Since: "1914", Until: "1924"},
		"открытое начало": {Text: "Ленинград", Until: "1991-09-06"},
		"открытый конец":  {Text: "Санкт-Петербург", Since: "1991-09-06"},
		"без периода":     {Text: "Питер"},
		"с формулировкой": {Text: "Санкт-Петербург", Since: "около 1703", Until: "между 1914 и 1915"},
		"юлианский":       {Text: "Петроград", Since: "1914-08-18 ст. ст."},
	}
	for name, n := range valid {
		if err := n.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	invalid := []struct {
		name  string
		n     NamedPeriod
		field string
	}{
		{"пустое наименование", NamedPeriod{Since: "1914"}, "text"},
		{"наименование из пробелов", NamedPeriod{Text: " "}, "text"},
		{"мусор в начале", NamedPeriod{Text: "x", Since: "когда-то"}, "since"},
		{"мусор в конце", NamedPeriod{Text: "x", Until: "1914-13"}, "until"},
		{"31 апреля", NamedPeriod{Text: "x", Since: "1914-04-31"}, "since.day"},
		{"начало позже конца", NamedPeriod{Text: "x", Since: "1924", Until: "1914"}, "since"},
	}
	for _, tt := range invalid {
		wantInvalid(t, tt.name, tt.n.Validate(), "", tt.field)
	}

	renames := []NamedPeriod{{Text: "Петроград", Since: "1914"}, {Text: "", Since: "1924"}}
	wantInvalid(t, "список переименований", finish("", validateRenames("renames", renames)), "", "renames[1].text")
	if e := validateRenames("renames", renames[:1]); e != nil {
		t.Errorf("корректный список: %v", e)
	}
	if e := validateRenames("renames", nil); e != nil {
		t.Errorf("пустой список: %v", e)
	}
}
```

- [ ] **Step 2: Убедиться, что тесты не компилируются**

Run: `go test ./internal/models`
Expected: FAIL — `undefined: requireText`, `undefined: validateStrings`, `PlaceRef.Validate undefined`, `NamedPeriod.Validate undefined`.

- [ ] **Step 3: Общие хелперы**

`internal/models/validate_helpers.go`:

```go
package models

import "strings"

// requireText проверяет обязательное текстовое поле: непустое после обрезки
// пробелов.
func requireText(field, s string) *ValidationError {
	if strings.TrimSpace(s) == "" {
		return fieldErr(field, "значение обязательно (непустое после обрезки пробелов)")
	}

	return nil
}

// validateStrings проверяет список строк: пустых элементов быть не должно.
func validateStrings(field string, values []string) *ValidationError {
	for i, s := range values {
		if strings.TrimSpace(s) == "" {
			return fieldErr(indexed(field, i), "значение не может быть пустым")
		}
	}

	return nil
}

// validateOptionalID проверяет необязательный идентификатор: пустой допустим,
// непустой должен иметь формат и префикс ожидаемого типа.
func validateOptionalID(field string, id ID, want Type) *ValidationError {
	if id == "" {
		return nil
	}

	return idErr(field, id, want)
}

// validateOptionalIDPtr проверяет необязательную ссылку-указатель: nil допустим,
// указатель на пустой id — нет («задан, но пуст»).
func validateOptionalIDPtr(field string, id *ID, want Type) *ValidationError {
	if id == nil {
		return nil
	}

	return idErr(field, *id, want)
}

// validateNotSelf проверяет, что родитель не совпадает с самой сущностью
// (циклы длиннее — забота сценариев: для них нужно хранилище).
func validateNotSelf(self ID, parent *ID) *ValidationError {
	if parent != nil && *parent == self {
		return fieldErr("parent_id", "родитель совпадает с самой сущностью")
	}

	return nil
}

// validateOptionalTextRefPtr проверяет необязательный *TextRef: nil допустим;
// заданный указатель проверяется целиком (пустой TextRef — ошибка).
func validateOptionalTextRefPtr(field string, r *TextRef, want Type) *ValidationError {
	if r == nil {
		return nil
	}

	return r.validateAs(want).within(field)
}
```

- [ ] **Step 4: `PlaceRef`**

`internal/models/place_ref_validate.go`:

```go
package models

// placeTypes — типы сущностей, на которые может указывать PlaceRef.
var placeTypes = [...]Type{TypeAdministrativeDivision, TypeChurch, TypeParish}

// Validate проверяет указание на место: как TextRef, а ссылка (если есть) —
// только на административное деление, церковь или приход.
func (p PlaceRef) Validate() error {
	return finish("", p.validate())
}

func (p PlaceRef) validate() *ValidationError {
	r := TextRef(p)
	if e := r.validateAs(""); e != nil {
		return e
	}
	if r.Ref == "" {
		return nil
	}

	for _, t := range placeTypes {
		if r.Type == t {
			return nil
		}
	}

	return fieldErr("type", "место может ссылаться только на administrative_division, church или parish, получено %s", r.Type)
}
```

- [ ] **Step 5: `NamedPeriod`**

`internal/models/named_period_validate.go`:

```go
package models

import "strings"

// Validate проверяет именование с периодом: наименование обязательно; начало и
// конец — строки в формате ParseFactDate (пустое допустимо, период открытый),
// проходят FactDate.Validate, начало не позже конца.
func (n NamedPeriod) Validate() error {
	return finish("", n.validate())
}

func (n NamedPeriod) validate() *ValidationError {
	if e := requireText("text", n.Text); e != nil {
		return e
	}

	since, e := parseNamedDate("since", n.Since)
	if e != nil {
		return e
	}
	until, e := parseNamedDate("until", n.Until)
	if e != nil {
		return e
	}

	return validatePeriod(since, until)
}

// parseNamedDate разбирает необязательную дату периода; пустая строка — nil.
func parseNamedDate(field, s string) (*FactDate, *ValidationError) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}

	d, err := ParseFactDate(s)
	if err != nil {
		return nil, fieldErr(field, "недопустимая дата %q: %v", s, err)
	}

	return &d, nil
}

// validateRenames проверяет список исторических наименований.
func validateRenames(field string, renames []NamedPeriod) *ValidationError {
	for i, n := range renames {
		if e := n.validate(); e != nil {
			return e.within(indexed(field, i))
		}
	}

	return nil
}
```

- [ ] **Step 6: Прогнать тесты**

Run: `gofmt -l internal/models` → пусто.
Run: `go vet ./internal/models && go test ./internal/models`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/models/validate_helpers.go internal/models/place_ref_validate.go internal/models/named_period_validate.go internal/models/validate_helpers_test.go
git commit -m "feat(models): хелперы валидации, PlaceRef и NamedPeriod

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

### Task 2: Якорь и `Citation`

**Files:**
- Create: `internal/models/anchor_validate.go`
- Create: `internal/models/citation_validate.go`
- Test: `internal/models/citation_validate_test.go`

**Interfaces:**
- Consumes: Task 1 (`idErr`, `validateOptionalID`), `Anchor`, `ArchiveAnchor`, `FileAnchor`, `URLAnchor`.
- Produces: `validateAnchor(a Anchor) *ValidationError` (путь пуст для самого якоря, `node_id` и т. д. — для полей); `func (c *Citation) Validate() error`.

- [ ] **Step 1: Написать падающие тесты**

`internal/models/citation_validate_test.go`:

```go
package models

import "testing"

func TestValidateAnchor(t *testing.T) {
	valid := map[string]Anchor{
		"nil-интерфейс": nil,
		"архивный":      &ArchiveAnchor{NodeID: testID(TypeArchiveNode), Page: 12, Rect: "10,10,20,20"},
		"архивный с документом": &ArchiveAnchor{
			NodeID: testID(TypeArchiveNode), DocumentID: testID(TypeArchiveDocument), Page: 1,
		},
		"файл":          &FileAnchor{AttachmentID: testID(TypeAttachment)},
		"файл с меткой": &FileAnchor{AttachmentID: testID(TypeAttachment), Timecode: "00:12:30"},
		"url":           &URLAnchor{URL: "https://example.org/page?a=1#x"},
		"http":          &URLAnchor{URL: "http://example.org"},
	}
	for name, a := range valid {
		if e := validateAnchor(a); e != nil {
			t.Errorf("%s: %v", name, e)
		}
	}

	var nilArchive *ArchiveAnchor
	var nilFile *FileAnchor
	var nilURL *URLAnchor

	invalid := []struct {
		name  string
		a     Anchor
		field string
	}{
		{"nil *ArchiveAnchor", nilArchive, ""},
		{"nil *FileAnchor", nilFile, ""},
		{"nil *URLAnchor", nilURL, ""},
		{"архивный без узла", &ArchiveAnchor{Page: 1}, "node_id"},
		{"архивный узел не того типа", &ArchiveAnchor{NodeID: testID(TypeArchive), Page: 1}, "node_id"},
		{"архивный документ не того типа", &ArchiveAnchor{NodeID: testID(TypeArchiveNode), DocumentID: testID(TypeArchiveNode), Page: 1}, "document_id"},
		{"архивный page 0", &ArchiveAnchor{NodeID: testID(TypeArchiveNode)}, "page"},
		{"архивный page < 0", &ArchiveAnchor{NodeID: testID(TypeArchiveNode), Page: -3}, "page"},
		{"файл без вложения", &FileAnchor{}, "attachment_id"},
		{"файл вложение не того типа", &FileAnchor{AttachmentID: testID(TypeNote)}, "attachment_id"},
		{"url пустой", &URLAnchor{}, "url"},
		{"url без схемы", &URLAnchor{URL: "example.org/page"}, "url"},
		{"url ftp", &URLAnchor{URL: "ftp://example.org/file"}, "url"},
		{"url без хоста", &URLAnchor{URL: "https:///path"}, "url"},
		{"url мусор", &URLAnchor{URL: "http://exa mple.org"}, "url"},
	}
	for _, tt := range invalid {
		wantInvalid(t, tt.name, finish("", validateAnchor(tt.a)), "", tt.field)
	}
}

// Неизвестная реализация интерфейса Anchor отвергается.
type foreignAnchor struct{}

func (foreignAnchor) Kind() AnchorKind { return "foreign" }

func TestValidateAnchorUnknownImplementation(t *testing.T) {
	wantInvalid(t, "чужая реализация", finish("", validateAnchor(foreignAnchor{})), "", "")
}

func validCitation() *Citation {
	return &Citation{
		ID:       testID(TypeCitation),
		SourceID: testID(TypeSource),
		Anchor:   &ArchiveAnchor{NodeID: testID(TypeArchiveNode), Page: 12, Rect: "10,10,20,20"},
		Text:     "Акилина Иванова, 53 лет",
		Note:     "запись 14",
		Private:  true,
	}
}

func TestCitationValidate(t *testing.T) {
	if err := validCitation().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}

	// Якорь и текст необязательны: цитата может ссылаться на источник целиком.
	if err := (&Citation{ID: testID(TypeCitation), SourceID: testID(TypeSource)}).Validate(); err != nil {
		t.Errorf("цитата без якоря и текста: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Citation)
		field  string
	}{
		{"плохой id", func(c *Citation) { c.ID = "c-1" }, "id"},
		{"id другого типа", func(c *Citation) { c.ID = testID(TypeSource) }, "id"},
		{"пустой source_id", func(c *Citation) { c.SourceID = "" }, "source_id"},
		{"source_id не источник", func(c *Citation) { c.SourceID = testID(TypeCitation) }, "source_id"},
		{"плохой якорь", func(c *Citation) { c.Anchor = &ArchiveAnchor{Page: 1} }, "anchor.node_id"},
		{"nil внутри якоря", func(c *Citation) { c.Anchor = (*URLAnchor)(nil) }, "anchor"},
		{"плохой url", func(c *Citation) { c.Anchor = &URLAnchor{URL: "nope"} }, "anchor.url"},
	}
	for _, tt := range tests {
		c := validCitation()
		tt.mutate(c)
		wantInvalid(t, tt.name, c.Validate(), TypeCitation, tt.field)
	}
}
```

- [ ] **Step 2: Убедиться, что тесты не компилируются**

Run: `go test ./internal/models`
Expected: FAIL — `undefined: validateAnchor`, `c.Validate undefined`.

- [ ] **Step 3: Якорь**

`internal/models/anchor_validate.go`:

```go
package models

import "net/url"

// validateAnchor проверяет привязку «где именно». Пустой якорь допустим
// (цитата может быть только текстовой); nil-указатель внутри интерфейса и
// неизвестная реализация — ошибка. Путь ошибки самого якоря пуст, полей — по
// имени (node_id, page, url, …).
func validateAnchor(a Anchor) *ValidationError {
	switch v := a.(type) {
	case nil:
		return nil
	case *ArchiveAnchor:
		if v == nil {
			return fieldErr("", "пустой якорь (nil-указатель)")
		}
		if e := idErr("node_id", v.NodeID, TypeArchiveNode); e != nil {
			return e
		}
		if e := validateOptionalID("document_id", v.DocumentID, TypeArchiveDocument); e != nil {
			return e
		}
		if v.Page < 1 {
			return fieldErr("page", "номер страницы должен быть не меньше 1: %d", v.Page)
		}

		return nil
	case *FileAnchor:
		if v == nil {
			return fieldErr("", "пустой якорь (nil-указатель)")
		}

		return idErr("attachment_id", v.AttachmentID, TypeAttachment)
	case *URLAnchor:
		if v == nil {
			return fieldErr("", "пустой якорь (nil-указатель)")
		}
		u, err := url.Parse(v.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fieldErr("url", "нужен абсолютный http(s)-адрес с хостом: %q", v.URL)
		}

		return nil
	default:
		return fieldErr("", "неизвестная реализация якоря %T", a)
	}
}
```

- [ ] **Step 4: `Citation`**

`internal/models/citation_validate.go`:

```go
package models

// Validate проверяет цитату: источник обязателен и имеет тип source; якорь и
// текст необязательны (цитата может относиться к источнику целиком).
// Соответствие вида якоря Source.kind проверяют сценарии (нужно хранилище).
func (c *Citation) Validate() error {
	return finish(TypeCitation, c.validate())
}

func (c *Citation) validate() *ValidationError {
	if e := idErr("id", c.ID, TypeCitation); e != nil {
		return e
	}
	if e := idErr("source_id", c.SourceID, TypeSource); e != nil {
		return e
	}

	return validateAnchor(c.Anchor).within("anchor")
}
```

- [ ] **Step 5: Прогнать тесты**

Run: `gofmt -l internal/models` → пусто.
Run: `go vet ./internal/models && go test ./internal/models`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/models/anchor_validate.go internal/models/citation_validate.go internal/models/citation_validate_test.go
git commit -m "feat(models): проверки якоря и Citation

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

### Task 3: `AdministrativeDivision`, `Church`, `Parish`

**Files:**
- Create: `internal/models/administrative_division_validate.go`
- Create: `internal/models/church_validate.go`
- Create: `internal/models/parish_validate.go`
- Test: `internal/models/places_validate_test.go`

**Interfaces:**
- Consumes: Task 1 helpers, S3 (`validateTextRefs`, `validatePeriod`, `validateSourceLinks`).
- Produces: `func (a *AdministrativeDivision) Validate() error`, `func (c *Church) Validate() error`, `func (p *Parish) Validate() error`.

- [ ] **Step 1: Написать падающие тесты**

`internal/models/places_validate_test.go`:

```go
package models

import "testing"

func validDivision() *AdministrativeDivision {
	return &AdministrativeDivision{
		ID:         testID(TypeAdministrativeDivision),
		Name:       "Санкт-Петербург",
		Type:       AdminDivisionGorod,
		ParentID:   idPtr(divisionBID()),
		Items:      []TextRef{{Text: "Васильевский остров", Ref: divisionBID(), Type: TypeAdministrativeDivision}, {Text: "Охта"}},
		Variants:   []string{"Санктпетербург", "Питербурх"},
		Renames:    []NamedPeriod{{Text: "Петроград", Since: "1914", Until: "1924"}},
		Successors: []TextRef{{Text: "Ленинград"}},
		Since:      &FactDate{Year: 1703, Precision: PrecisionYear, Modifier: ModifierExact},
		Notes:      []TextRef{{Text: "столица"}},
		Sources:    []SourceLink{{CitationID: testID(TypeCitation)}},
	}
}

// divisionBID — другой валидный идентификатор деления (иное тело ULID).
func divisionBID() ID {
	id, err := BuildID(TypeAdministrativeDivision, "01J8X4T0K2M9Q7R5V3B6N8C1D5")
	if err != nil {
		panic(err)
	}

	return id
}

func TestAdministrativeDivisionValidate(t *testing.T) {
	if err := validDivision().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}

	// Корень иерархии: без родителя и без необязательных списков.
	root := &AdministrativeDivision{ID: testID(TypeAdministrativeDivision), Name: "Российская империя", Type: AdminDivisionOther}
	if err := root.Validate(); err != nil {
		t.Errorf("минимальное деление: %v", err)
	}

	past := &FactDate{Year: 1703, Precision: PrecisionYear, Modifier: ModifierExact}
	future := &FactDate{Year: 1917, Precision: PrecisionYear, Modifier: ModifierExact}

	tests := []struct {
		name   string
		mutate func(*AdministrativeDivision)
		field  string
	}{
		{"плохой id", func(a *AdministrativeDivision) { a.ID = "ad-1" }, "id"},
		{"id другого типа", func(a *AdministrativeDivision) { a.ID = testID(TypeChurch) }, "id"},
		{"пустое название", func(a *AdministrativeDivision) { a.Name = "" }, "name"},
		{"название из пробелов", func(a *AdministrativeDivision) { a.Name = "  " }, "name"},
		{"пустой тип", func(a *AdministrativeDivision) { a.Type = "" }, "type"},
		{"неизвестный тип", func(a *AdministrativeDivision) { a.Type = "kray" }, "type"},
		{"родитель не деление", func(a *AdministrativeDivision) { a.ParentID = idPtr(testID(TypeParish)) }, "parent_id"},
		{"указатель на пустой родитель", func(a *AdministrativeDivision) { a.ParentID = idPtr("") }, "parent_id"},
		{"родитель — сам себе", func(a *AdministrativeDivision) { a.ParentID = idPtr(a.ID) }, "parent_id"},
		{"составляющая — не деление", func(a *AdministrativeDivision) {
			a.Items[0] = TextRef{Ref: testID(TypeParish), Type: TypeParish}
		}, "items[0].type"},
		{"пустая составляющая", func(a *AdministrativeDivision) { a.Items = append(a.Items, TextRef{}) }, "items[2].text"},
		{"пустой вариант", func(a *AdministrativeDivision) { a.Variants = append(a.Variants, " ") }, "variants[2]"},
		{"переименование без имени", func(a *AdministrativeDivision) { a.Renames[0].Text = "" }, "renames[0].text"},
		{"переименование с плохой датой", func(a *AdministrativeDivision) { a.Renames[0].Since = "давно" }, "renames[0].since"},
		{"преемник — не деление", func(a *AdministrativeDivision) {
			a.Successors[0] = TextRef{Ref: testID(TypeChurch), Type: TypeChurch}
		}, "successors[0].type"},
		{"плохое начало", func(a *AdministrativeDivision) { a.Since = &FactDate{} }, "since.precision"},
		{"начало позже конца", func(a *AdministrativeDivision) { a.Since, a.Until = future, past }, "since"},
		{"пустая заметка", func(a *AdministrativeDivision) { a.Notes[0] = TextRef{} }, "notes[0].text"},
		{"плохая цитата", func(a *AdministrativeDivision) { a.Sources[0].CitationID = "x" }, "sources[0].citation_id"},
	}
	for _, tt := range tests {
		a := validDivision()
		tt.mutate(a)
		wantInvalid(t, tt.name, a.Validate(), TypeAdministrativeDivision, tt.field)
	}
}

func validChurch() *Church {
	return &Church{
		ID:          testID(TypeChurch),
		Name:        "Никольская церковь",
		Parish:      &TextRef{Text: "Никольский приход", Ref: testID(TypeParish), Type: TypeParish},
		Settlements: []TextRef{{Text: "Давыдово", Ref: testID(TypeAdministrativeDivision), Type: TypeAdministrativeDivision}, {Text: "Никифорово"}},
		Variants:    []string{"Никольская ц."},
		Notes:       []TextRef{{Text: "деревянная"}},
		Sources:     []SourceLink{{CitationID: testID(TypeCitation)}},
	}
}

func TestChurchValidate(t *testing.T) {
	if err := validChurch().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}
	if err := (&Church{ID: testID(TypeChurch), Name: "ц."}).Validate(); err != nil {
		t.Errorf("минимальная церковь: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Church)
		field  string
	}{
		{"плохой id", func(c *Church) { c.ID = "" }, "id"},
		{"пустое название", func(c *Church) { c.Name = "" }, "name"},
		{"приход — не приход", func(c *Church) { c.Parish = &TextRef{Ref: testID(TypeChurch), Type: TypeChurch} }, "parish.type"},
		{"пустой приход", func(c *Church) { c.Parish = &TextRef{} }, "parish.text"},
		{"населённый пункт — не деление", func(c *Church) {
			c.Settlements[0] = TextRef{Ref: testID(TypeParish), Type: TypeParish}
		}, "settlements[0].type"},
		{"пустой вариант", func(c *Church) { c.Variants = []string{""} }, "variants[0]"},
		{"пустая заметка", func(c *Church) { c.Notes = append(c.Notes, TextRef{}) }, "notes[1].text"},
		{"плохая цитата", func(c *Church) { c.Sources[0].CitationID = "" }, "sources[0].citation_id"},
	}
	for _, tt := range tests {
		c := validChurch()
		tt.mutate(c)
		wantInvalid(t, tt.name, c.Validate(), TypeChurch, tt.field)
	}
}

func validParish() *Parish {
	return &Parish{
		ID:          testID(TypeParish),
		Name:        "Никольский приход",
		Church:      &TextRef{Text: "Никольская церковь", Ref: testID(TypeChurch), Type: TypeChurch},
		Settlements: []TextRef{{Text: "Давыдово", Ref: testID(TypeAdministrativeDivision), Type: TypeAdministrativeDivision}},
		Since:       &FactDate{Year: 1800, Precision: PrecisionYear, Modifier: ModifierExact},
		Until:       &FactDate{Year: 1917, Precision: PrecisionYear, Modifier: ModifierExact},
		Notes:       []TextRef{{Text: "упразднён"}},
		Sources:     []SourceLink{{CitationID: testID(TypeCitation)}},
	}
}

func TestParishValidate(t *testing.T) {
	if err := validParish().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Parish)
		field  string
	}{
		{"плохой id", func(p *Parish) { p.ID = testID(TypeChurch) }, "id"},
		{"пустое название", func(p *Parish) { p.Name = " " }, "name"},
		{"церковь — не церковь", func(p *Parish) { p.Church = &TextRef{Ref: testID(TypeParish), Type: TypeParish} }, "church.type"},
		{"населённый пункт — не деление", func(p *Parish) {
			p.Settlements[0] = TextRef{Ref: testID(TypeChurch), Type: TypeChurch}
		}, "settlements[0].type"},
		{"начало позже конца", func(p *Parish) { p.Since, p.Until = p.Until, p.Since }, "since"},
		{"плохой конец", func(p *Parish) { p.Until = &FactDate{Year: 1917} }, "until.precision"},
		{"пустая заметка", func(p *Parish) { p.Notes[0] = TextRef{} }, "notes[0].text"},
		{"плохая цитата", func(p *Parish) { p.Sources[0].Reliability = "maybe" }, "sources[0].reliability"},
	}
	for _, tt := range tests {
		p := validParish()
		tt.mutate(p)
		wantInvalid(t, tt.name, p.Validate(), TypeParish, tt.field)
	}
}
```

- [ ] **Step 2: Убедиться, что тесты не компилируются**

Run: `go test ./internal/models`
Expected: FAIL — `a.Validate undefined` для трёх типов.

- [ ] **Step 3: `AdministrativeDivision`**

`internal/models/administrative_division_validate.go`:

```go
package models

// Validate проверяет единицу административного деления: название и вид
// (закрытый enum) обязательны; родитель — деление, не совпадающее с самой
// единицей (циклы длиннее проверяют сценарии); составляющие и преемники
// ссылаются на деления; варианты — непустые строки; переименования и период
// корректны.
func (a *AdministrativeDivision) Validate() error {
	return finish(TypeAdministrativeDivision, a.validate())
}

func (a *AdministrativeDivision) validate() *ValidationError {
	if e := idErr("id", a.ID, TypeAdministrativeDivision); e != nil {
		return e
	}
	if e := requireText("name", a.Name); e != nil {
		return e
	}
	if !a.Type.Valid() {
		return fieldErr("type", "недопустимый тип единицы деления %q", a.Type)
	}

	if e := validateOptionalIDPtr("parent_id", a.ParentID, TypeAdministrativeDivision); e != nil {
		return e
	}
	if e := validateNotSelf(a.ID, a.ParentID); e != nil {
		return e
	}

	if e := validateTextRefs("items", a.Items, TypeAdministrativeDivision); e != nil {
		return e
	}
	if e := validateStrings("variants", a.Variants); e != nil {
		return e
	}
	if e := validateRenames("renames", a.Renames); e != nil {
		return e
	}
	if e := validateTextRefs("successors", a.Successors, TypeAdministrativeDivision); e != nil {
		return e
	}

	if e := validatePeriod(a.Since, a.Until); e != nil {
		return e
	}
	if e := validateTextRefs("notes", a.Notes, ""); e != nil {
		return e
	}

	return validateSourceLinks("sources", a.Sources)
}
```

- [ ] **Step 4: `Church`**

`internal/models/church_validate.go`:

```go
package models

// Validate проверяет церковь: название обязательно; приход — ссылка на parish
// (или текст); населённые пункты — ссылки на деления; варианты — непустые строки.
func (c *Church) Validate() error {
	return finish(TypeChurch, c.validate())
}

func (c *Church) validate() *ValidationError {
	if e := idErr("id", c.ID, TypeChurch); e != nil {
		return e
	}
	if e := requireText("name", c.Name); e != nil {
		return e
	}

	if e := validateOptionalTextRefPtr("parish", c.Parish, TypeParish); e != nil {
		return e
	}
	if e := validateTextRefs("settlements", c.Settlements, TypeAdministrativeDivision); e != nil {
		return e
	}
	if e := validateStrings("variants", c.Variants); e != nil {
		return e
	}
	if e := validateTextRefs("notes", c.Notes, ""); e != nil {
		return e
	}

	return validateSourceLinks("sources", c.Sources)
}
```

- [ ] **Step 5: `Parish`**

`internal/models/parish_validate.go`:

```go
package models

// Validate проверяет приход: название обязательно; церковь — ссылка на church
// (или текст); населённые пункты — ссылки на деления; период корректен.
func (p *Parish) Validate() error {
	return finish(TypeParish, p.validate())
}

func (p *Parish) validate() *ValidationError {
	if e := idErr("id", p.ID, TypeParish); e != nil {
		return e
	}
	if e := requireText("name", p.Name); e != nil {
		return e
	}

	if e := validateOptionalTextRefPtr("church", p.Church, TypeChurch); e != nil {
		return e
	}
	if e := validateTextRefs("settlements", p.Settlements, TypeAdministrativeDivision); e != nil {
		return e
	}
	if e := validatePeriod(p.Since, p.Until); e != nil {
		return e
	}
	if e := validateTextRefs("notes", p.Notes, ""); e != nil {
		return e
	}

	return validateSourceLinks("sources", p.Sources)
}
```

- [ ] **Step 6: Прогнать тесты**

Run: `gofmt -l internal/models` → пусто.
Run: `go vet ./internal/models && go test ./internal/models`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/models/administrative_division_validate.go internal/models/church_validate.go internal/models/parish_validate.go internal/models/places_validate_test.go
git commit -m "feat(models): Validate для AdministrativeDivision, Church и Parish

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

### Task 4: `Event` и `Source`

**Files:**
- Create: `internal/models/event_validate.go`
- Create: `internal/models/source_validate.go`
- Test: `internal/models/evidence_validate_test.go`

**Interfaces:**
- Consumes: Task 1–2 helpers; `EventParticipant`.
- Produces: `func (e *Event) Validate() error`, `func (s *Source) Validate() error`; внутренний `(p EventParticipant) validate() *ValidationError`.

- [ ] **Step 1: Написать падающие тесты**

`internal/models/evidence_validate_test.go`:

```go
package models

import "testing"

func validEvent() *Event {
	return &Event{
		ID:    testID(TypeEvent),
		Type:  EventTypeBirth,
		Date:  &FactDate{Year: 1881, Month: 3, Day: 15, Precision: PrecisionDay, Modifier: ModifierExact},
		Place: &PlaceRef{Text: "Давыдово", Ref: testID(TypeAdministrativeDivision), Type: TypeAdministrativeDivision},
		Participants: []EventParticipant{
			{PersonID: testID(TypePerson), Role: "ребёнок", Note: "первый"},
			{PersonID: personBID(), Role: "восприемник"},
		},
		Sources: []SourceLink{{CitationID: testID(TypeCitation)}},
		Notes:   []TextRef{{Text: "метрическая запись"}},
		Private: true,
	}
}

func TestEventValidate(t *testing.T) {
	if err := validEvent().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}

	// Дата, место и участники необязательны.
	if err := (&Event{ID: testID(TypeEvent), Type: "census"}).Validate(); err != nil {
		t.Errorf("минимальное событие: %v", err)
	}
	// Открытый enum: своё значение допустимо.
	if err := (&Event{ID: testID(TypeEvent), Type: "first-communion"}).Validate(); err != nil {
		t.Errorf("своё значение типа: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Event)
		field  string
	}{
		{"плохой id", func(e *Event) { e.ID = "ev-1" }, "id"},
		{"id другого типа", func(e *Event) { e.ID = testID(TypeCitation) }, "id"},
		{"пустой тип", func(e *Event) { e.Type = "" }, "type"},
		{"тип не в формате", func(e *Event) { e.Type = "Birth" }, "type"},
		{"тип с пробелом", func(e *Event) { e.Type = "first communion" }, "type"},
		{"плохая дата", func(e *Event) { e.Date = &FactDate{} }, "date.precision"},
		{"31 апреля", func(e *Event) {
			e.Date = &FactDate{Year: 1881, Month: 4, Day: 31, Precision: PrecisionDay, Modifier: ModifierExact}
		}, "date.day"},
		{"место — персона", func(e *Event) { e.Place = &PlaceRef{Ref: testID(TypePerson), Type: TypePerson} }, "place.type"},
		{"пустое место", func(e *Event) { e.Place = &PlaceRef{} }, "place.text"},
		{"участник без персоны", func(e *Event) { e.Participants[0].PersonID = "" }, "participants[0].person_id"},
		{"участник — не персона", func(e *Event) { e.Participants[1].PersonID = testID(TypeFamily) }, "participants[1].person_id"},
		{"участник без роли", func(e *Event) { e.Participants[1].Role = "" }, "participants[1].role"},
		{"роль из пробелов", func(e *Event) { e.Participants[0].Role = "  " }, "participants[0].role"},
		{"плохая цитата", func(e *Event) { e.Sources[0].CitationID = "x" }, "sources[0].citation_id"},
		{"пустая заметка", func(e *Event) { e.Notes = append(e.Notes, TextRef{}) }, "notes[1].text"},
	}
	for _, tt := range tests {
		e := validEvent()
		tt.mutate(e)
		wantInvalid(t, tt.name, e.Validate(), TypeEvent, tt.field)
	}
}

func validSource() *Source {
	return &Source{
		ID:           testID(TypeSource),
		Kind:         SourceKindArchivalScan,
		Title:        "МК с. Давыдово за 1881",
		Author:       "причт Никольской церкви",
		Date:         &FactDate{Year: 1881, Precision: PrecisionYear, Modifier: ModifierExact},
		Reliability:  ReliabilityPrimary,
		RepositoryID: testID(TypeRepository),
		Notes:        []TextRef{{Text: "хорошая сохранность"}},
		Private:      true,
	}
}

func TestSourceValidate(t *testing.T) {
	if err := validSource().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}
	// Автор, дата, хранилище и заметки необязательны.
	min := &Source{ID: testID(TypeSource), Kind: SourceKindMemory, Title: "воспоминания бабушки", Reliability: ReliabilityUnknown}
	if err := min.Validate(); err != nil {
		t.Errorf("минимальный источник: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Source)
		field  string
	}{
		{"плохой id", func(s *Source) { s.ID = "" }, "id"},
		{"пустой вид", func(s *Source) { s.Kind = "" }, "kind"},
		{"неизвестный вид", func(s *Source) { s.Kind = "rumor" }, "kind"},
		{"пустое название", func(s *Source) { s.Title = "" }, "title"},
		{"название из пробелов", func(s *Source) { s.Title = " " }, "title"},
		{"плохая дата", func(s *Source) { s.Date = &FactDate{Year: 0, Precision: PrecisionYear, Modifier: ModifierExact} }, "date.year"},
		{"пустая достоверность", func(s *Source) { s.Reliability = "" }, "reliability"},
		{"неизвестная достоверность", func(s *Source) { s.Reliability = "maybe" }, "reliability"},
		{"хранилище — не хранилище", func(s *Source) { s.RepositoryID = testID(TypeArchive) }, "repository_id"},
		{"хранилище — плохой формат", func(s *Source) { s.RepositoryID = "R-1" }, "repository_id"},
		{"пустая заметка", func(s *Source) { s.Notes[0] = TextRef{} }, "notes[0].text"},
	}
	for _, tt := range tests {
		s := validSource()
		tt.mutate(s)
		wantInvalid(t, tt.name, s.Validate(), TypeSource, tt.field)
	}
}
```

- [ ] **Step 2: Убедиться, что тесты не компилируются**

Run: `go test ./internal/models`
Expected: FAIL — `e.Validate undefined` (`Event`), `s.Validate undefined` (`Source`).

- [ ] **Step 3: `Event`**

`internal/models/event_validate.go`:

```go
package models

// Validate проверяет событие: вид обязателен (открытый enum); дата, место
// (PlaceRef) и участники необязательны; у участника — персона и роль.
func (e *Event) Validate() error {
	return finish(TypeEvent, e.validate())
}

func (e *Event) validate() *ValidationError {
	if err := idErr("id", e.ID, TypeEvent); err != nil {
		return err
	}
	if !validOpenEnum(string(e.Type)) {
		return fieldErr("type", "недопустимый вид события %q: ожидается [a-z][a-z0-9_-]*", e.Type)
	}

	if e.Date != nil {
		if err := e.Date.validate(); err != nil {
			return err.within("date")
		}
	}
	if e.Place != nil {
		if err := e.Place.validate(); err != nil {
			return err.within("place")
		}
	}

	for i, p := range e.Participants {
		if err := p.validate(); err != nil {
			return err.within(indexed("participants", i))
		}
	}

	if err := validateSourceLinks("sources", e.Sources); err != nil {
		return err
	}

	return validateTextRefs("notes", e.Notes, "")
}

// validate проверяет участника: персона обязательна, роль — непустая.
func (p EventParticipant) validate() *ValidationError {
	if e := idErr("person_id", p.PersonID, TypePerson); e != nil {
		return e
	}

	return requireText("role", p.Role)
}
```

- [ ] **Step 4: `Source`**

`internal/models/source_validate.go`:

```go
package models

// Validate проверяет источник: вид, название и общая достоверность обязательны
// (вид и достоверность — закрытые enum'ы); автор и заметки необязательны; дата
// корректна; хранилище (если задано) — repository.
func (s *Source) Validate() error {
	return finish(TypeSource, s.validate())
}

func (s *Source) validate() *ValidationError {
	if e := idErr("id", s.ID, TypeSource); e != nil {
		return e
	}
	if !s.Kind.Valid() {
		return fieldErr("kind", "недопустимый вид источника %q", s.Kind)
	}
	if e := requireText("title", s.Title); e != nil {
		return e
	}

	if s.Date != nil {
		if e := s.Date.validate(); e != nil {
			return e.within("date")
		}
	}
	if !s.Reliability.Valid() {
		return fieldErr("reliability", "недопустимая достоверность %q", s.Reliability)
	}
	if e := validateOptionalID("repository_id", s.RepositoryID, TypeRepository); e != nil {
		return e
	}

	return validateTextRefs("notes", s.Notes, "")
}
```

- [ ] **Step 5: Прогнать тесты**

Run: `gofmt -l internal/models` → пусто.
Run: `go vet ./internal/models && go test ./internal/models`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/models/event_validate.go internal/models/source_validate.go internal/models/evidence_validate_test.go
git commit -m "feat(models): Validate для Event и Source

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

### Task 5: `Note`, `Repository`, `Archive`

**Files:**
- Create: `internal/models/note_validate.go`
- Create: `internal/models/repository_validate.go`
- Create: `internal/models/archive_validate.go`
- Test: `internal/models/storage_entities_validate_test.go`

**Interfaces:**
- Consumes: Task 1 helpers.
- Produces: `func (n *Note) Validate() error`, `func (r *Repository) Validate() error`, `func (a *Archive) Validate() error`.

- [ ] **Step 1: Написать падающие тесты**

`internal/models/storage_entities_validate_test.go`:

```go
package models

import "testing"

func validNote() *Note {
	return &Note{
		ID:       testID(TypeNote),
		Kind:     NoteKindChapter,
		Title:    "Глава 1",
		Text:     "# Начало\n\nтекст",
		ParentID: idPtr(noteBID()),
		Sources:  []SourceLink{{CitationID: testID(TypeCitation)}},
		Private:  true,
	}
}

// noteBID — другой валидный идентификатор заметки (иное тело ULID).
func noteBID() ID {
	id, err := BuildID(TypeNote, "01J8X4T0K2M9Q7R5V3B6N8C1D5")
	if err != nil {
		panic(err)
	}

	return id
}

func TestNoteValidate(t *testing.T) {
	if err := validNote().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}

	// Книга-контейнер: только заголовок, текст — в главах.
	book := &Note{ID: testID(TypeNote), Kind: NoteKindBook, Title: "Родословная книга"}
	if err := book.Validate(); err != nil {
		t.Errorf("книга только с заголовком: %v", err)
	}
	// Заметка без заголовка, только текст.
	if err := (&Note{ID: testID(TypeNote), Kind: NoteKindNote, Text: "выписка"}).Validate(); err != nil {
		t.Errorf("заметка только с текстом: %v", err)
	}
	// Открытый enum.
	if err := (&Note{ID: testID(TypeNote), Kind: "letter", Text: "x"}).Validate(); err != nil {
		t.Errorf("свой вид заметки: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Note)
		field  string
	}{
		{"плохой id", func(n *Note) { n.ID = "n-1" }, "id"},
		{"пустой вид", func(n *Note) { n.Kind = "" }, "kind"},
		{"вид не в формате", func(n *Note) { n.Kind = "Chapter" }, "kind"},
		{"ни заголовка, ни текста", func(n *Note) { n.Title, n.Text = "", "" }, "text"},
		{"заголовок и текст из пробелов", func(n *Note) { n.Title, n.Text = " ", "\n" }, "text"},
		{"родитель не заметка", func(n *Note) { n.ParentID = idPtr(testID(TypeSource)) }, "parent_id"},
		{"родитель — сам себе", func(n *Note) { n.ParentID = idPtr(n.ID) }, "parent_id"},
		{"плохая цитата", func(n *Note) { n.Sources[0].CitationID = "" }, "sources[0].citation_id"},
	}
	for _, tt := range tests {
		n := validNote()
		tt.mutate(n)
		wantInvalid(t, tt.name, n.Validate(), TypeNote, tt.field)
	}
}

func validRepository() *Repository {
	return &Repository{
		ID:      testID(TypeRepository),
		Name:    "ГАКО",
		Type:    RepositoryTypeArchive,
		Address: "г. Калуга, ул. Ленина, 1",
		URLs:    []TextRef{{Text: "https://gako.example.org"}},
		Notes:   []TextRef{{Text: "читальный зал по записи"}},
		Sources: []SourceLink{{CitationID: testID(TypeCitation)}},
		Private: false,
	}
}

func TestRepositoryValidate(t *testing.T) {
	if err := validRepository().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}
	if err := (&Repository{ID: testID(TypeRepository), Name: "дом", Type: "family-home"}).Validate(); err != nil {
		t.Errorf("свой тип хранилища: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Repository)
		field  string
	}{
		{"плохой id", func(r *Repository) { r.ID = "R" }, "id"},
		{"пустое название", func(r *Repository) { r.Name = "" }, "name"},
		{"пустой тип", func(r *Repository) { r.Type = "" }, "type"},
		{"тип не в формате", func(r *Repository) { r.Type = "Archive" }, "type"},
		{"пустая ссылка", func(r *Repository) { r.URLs = append(r.URLs, TextRef{}) }, "urls[1].text"},
		{"пустая заметка", func(r *Repository) { r.Notes[0] = TextRef{} }, "notes[0].text"},
		{"плохая цитата", func(r *Repository) { r.Sources[0].CitationID = "x" }, "sources[0].citation_id"},
	}
	for _, tt := range tests {
		r := validRepository()
		tt.mutate(r)
		wantInvalid(t, tt.name, r.Validate(), TypeRepository, tt.field)
	}
}

func validArchive() *Archive {
	return &Archive{
		ID:           testID(TypeArchive),
		Name:         "ГАКО",
		System:       &TextRef{Text: "Фонды/Описи/Дела"},
		RepositoryID: testID(TypeRepository),
		Notes:        []TextRef{{Text: "фонд 33"}},
		Sources:      []SourceLink{{CitationID: testID(TypeCitation)}},
		Private:      true,
	}
}

func TestArchiveValidate(t *testing.T) {
	if err := validArchive().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}
	// Система и хранилище необязательны.
	if err := (&Archive{ID: testID(TypeArchive), Name: "домашний архив"}).Validate(); err != nil {
		t.Errorf("минимальный архив: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Archive)
		field  string
	}{
		{"плохой id", func(a *Archive) { a.ID = "" }, "id"},
		{"пустое название", func(a *Archive) { a.Name = "" }, "name"},
		{"система — пустой TextRef", func(a *Archive) { a.System = &TextRef{} }, "system.text"},
		{"система — только пробелы", func(a *Archive) { a.System = &TextRef{Text: " "} }, "system.text"},
		{"система со ссылкой", func(a *Archive) {
			a.System = &TextRef{Text: "Фонды/Описи/Дела", Ref: testID(TypeArchive), Type: TypeArchive}
		}, "system.ref"},
		{"хранилище — не хранилище", func(a *Archive) { a.RepositoryID = testID(TypeSource) }, "repository_id"},
		{"пустая заметка", func(a *Archive) { a.Notes[0] = TextRef{} }, "notes[0].text"},
		{"плохая цитата", func(a *Archive) { a.Sources[0].CitationID = "c" }, "sources[0].citation_id"},
	}
	for _, tt := range tests {
		a := validArchive()
		tt.mutate(a)
		wantInvalid(t, tt.name, a.Validate(), TypeArchive, tt.field)
	}
}
```

- [ ] **Step 2: Убедиться, что тесты не компилируются**

Run: `go test ./internal/models`
Expected: FAIL — `n.Validate undefined` (`Note`), `Repository`, `Archive`.

- [ ] **Step 3: `Note`**

`internal/models/note_validate.go`:

```go
package models

import "strings"

// Validate проверяет заметку: вид обязателен (открытый enum); нужен заголовок
// или текст (книга-контейнер может иметь только заголовок, заметка — только
// текст); родитель — note, не совпадающая с самой заметкой (циклы длиннее
// проверяют сценарии).
func (n *Note) Validate() error {
	return finish(TypeNote, n.validate())
}

func (n *Note) validate() *ValidationError {
	if e := idErr("id", n.ID, TypeNote); e != nil {
		return e
	}
	if !validOpenEnum(string(n.Kind)) {
		return fieldErr("kind", "недопустимый вид заметки %q: ожидается [a-z][a-z0-9_-]*", n.Kind)
	}
	if strings.TrimSpace(n.Title) == "" && strings.TrimSpace(n.Text) == "" {
		return fieldErr("text", "нужен заголовок или текст")
	}

	if e := validateOptionalIDPtr("parent_id", n.ParentID, TypeNote); e != nil {
		return e
	}
	if e := validateNotSelf(n.ID, n.ParentID); e != nil {
		return e
	}

	return validateSourceLinks("sources", n.Sources)
}
```

- [ ] **Step 4: `Repository`**

`internal/models/repository_validate.go`:

```go
package models

// Validate проверяет хранилище-контейнер источников: название и тип
// обязательны (тип — открытый enum); адрес свободный; ссылки и заметки —
// корректные TextRef.
func (r *Repository) Validate() error {
	return finish(TypeRepository, r.validate())
}

func (r *Repository) validate() *ValidationError {
	if e := idErr("id", r.ID, TypeRepository); e != nil {
		return e
	}
	if e := requireText("name", r.Name); e != nil {
		return e
	}
	if !validOpenEnum(string(r.Type)) {
		return fieldErr("type", "недопустимый тип хранилища %q: ожидается [a-z][a-z0-9_-]*", r.Type)
	}

	if e := validateTextRefs("urls", r.URLs, ""); e != nil {
		return e
	}
	if e := validateTextRefs("notes", r.Notes, ""); e != nil {
		return e
	}

	return validateSourceLinks("sources", r.Sources)
}
```

- [ ] **Step 5: `Archive`**

`internal/models/archive_validate.go`:

```go
package models

// Validate проверяет архив: название обязательно; система иерархии (если
// задана) — только текст с именем системы, без ссылки (определение системы —
// built-in данные, а не сущность); хранилище (если задано) — repository.
func (a *Archive) Validate() error {
	return finish(TypeArchive, a.validate())
}

func (a *Archive) validate() *ValidationError {
	if e := idErr("id", a.ID, TypeArchive); e != nil {
		return e
	}
	if e := requireText("name", a.Name); e != nil {
		return e
	}

	if a.System != nil {
		if a.System.Ref != "" || a.System.Type != "" {
			return fieldErr("system.ref", "система иерархии задаётся именем, ссылка на сущность не допускается")
		}
		if e := requireText("system.text", a.System.Text); e != nil {
			return e
		}
	}

	if e := validateOptionalID("repository_id", a.RepositoryID, TypeRepository); e != nil {
		return e
	}
	if e := validateTextRefs("notes", a.Notes, ""); e != nil {
		return e
	}

	return validateSourceLinks("sources", a.Sources)
}
```

- [ ] **Step 6: Прогнать тесты**

Run: `gofmt -l internal/models` → пусто.
Run: `go vet ./internal/models && go test ./internal/models`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/models/note_validate.go internal/models/repository_validate.go internal/models/archive_validate.go internal/models/storage_entities_validate_test.go
git commit -m "feat(models): Validate для Note, Repository и Archive

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

### Task 6: `ArchiveNode`, `ArchiveDocument`, `Attachment`

**Files:**
- Create: `internal/models/archive_node_validate.go`
- Create: `internal/models/archive_document_validate.go`
- Create: `internal/models/attachment_validate.go`
- Test: `internal/models/archive_chain_validate_test.go`

**Interfaces:**
- Consumes: Task 1 helpers.
- Produces: `func (n *ArchiveNode) Validate() error`, `func (d *ArchiveDocument) Validate() error`, `func (a *Attachment) Validate() error`.

- [ ] **Step 1: Написать падающие тесты**

`internal/models/archive_chain_validate_test.go`:

```go
package models

import "testing"

// archiveNodeBID — другой валидный идентификатор узла архива (иное тело ULID).
func archiveNodeBID() ID {
	id, err := BuildID(TypeArchiveNode, "01J8X4T0K2M9Q7R5V3B6N8C1D5")
	if err != nil {
		panic(err)
	}

	return id
}

func validArchiveNode() *ArchiveNode {
	return &ArchiveNode{
		ID:          testID(TypeArchiveNode),
		Type:        "case",
		ArchiveID:   testID(TypeArchive),
		ParentID:    idPtr(archiveNodeBID()),
		Label:       "дело 264",
		Name:        "Метрические книги с. Давыдово",
		Since:       &FactDate{Year: 1880, Precision: PrecisionYear, Modifier: ModifierExact},
		Until:       &FactDate{Year: 1890, Precision: PrecisionYear, Modifier: ModifierExact},
		Parish:      &TextRef{Text: "Никольский приход", Ref: testID(TypeParish), Type: TypeParish},
		Settlements: []TextRef{{Text: "Давыдово", Ref: testID(TypeAdministrativeDivision), Type: TypeAdministrativeDivision}},
		Notes:       []TextRef{{Text: "подшито"}},
		Sources:     []SourceLink{{CitationID: testID(TypeCitation)}},
		Private:     true,
	}
}

func TestArchiveNodeValidate(t *testing.T) {
	if err := validArchiveNode().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}

	// Промежуточный уровень: только тип, архив и шифр.
	fond := &ArchiveNode{ID: testID(TypeArchiveNode), Type: "fond", ArchiveID: testID(TypeArchive), Label: "ф. 33"}
	if err := fond.Validate(); err != nil {
		t.Errorf("минимальный узел: %v", err)
	}

	past := &FactDate{Year: 1880, Precision: PrecisionYear, Modifier: ModifierExact}
	future := &FactDate{Year: 1890, Precision: PrecisionYear, Modifier: ModifierExact}

	tests := []struct {
		name   string
		mutate func(*ArchiveNode)
		field  string
	}{
		{"плохой id", func(n *ArchiveNode) { n.ID = "an-1" }, "id"},
		{"пустой уровень", func(n *ArchiveNode) { n.Type = "" }, "type"},
		{"уровень не в формате", func(n *ArchiveNode) { n.Type = "Дело" }, "type"},
		{"пустой archive_id", func(n *ArchiveNode) { n.ArchiveID = "" }, "archive_id"},
		{"archive_id — не архив", func(n *ArchiveNode) { n.ArchiveID = testID(TypeRepository) }, "archive_id"},
		{"родитель не узел", func(n *ArchiveNode) { n.ParentID = idPtr(testID(TypeArchive)) }, "parent_id"},
		{"родитель — сам себе", func(n *ArchiveNode) { n.ParentID = idPtr(n.ID) }, "parent_id"},
		{"пустой шифр", func(n *ArchiveNode) { n.Label = "" }, "label"},
		{"шифр из пробелов", func(n *ArchiveNode) { n.Label = "  " }, "label"},
		{"начало позже конца", func(n *ArchiveNode) { n.Since, n.Until = future, past }, "since"},
		{"плохое начало", func(n *ArchiveNode) { n.Since = &FactDate{} }, "since.precision"},
		{"приход — не приход", func(n *ArchiveNode) { n.Parish = &TextRef{Ref: testID(TypeChurch), Type: TypeChurch} }, "parish.type"},
		{"населённый пункт — не деление", func(n *ArchiveNode) {
			n.Settlements[0] = TextRef{Ref: testID(TypeParish), Type: TypeParish}
		}, "settlements[0].type"},
		{"пустая заметка", func(n *ArchiveNode) { n.Notes[0] = TextRef{} }, "notes[0].text"},
		{"плохая цитата", func(n *ArchiveNode) { n.Sources[0].CitationID = "" }, "sources[0].citation_id"},
	}
	for _, tt := range tests {
		n := validArchiveNode()
		tt.mutate(n)
		wantInvalid(t, tt.name, n.Validate(), TypeArchiveNode, tt.field)
	}
}

func validArchiveDocument() *ArchiveDocument {
	return &ArchiveDocument{
		ID:          testID(TypeArchiveDocument),
		UnitID:      testID(TypeArchiveNode),
		Title:       "Метрическая книга Николаевской ц. с. Давыдово за 1880–1882",
		Kind:        "метрическая книга",
		Since:       &FactDate{Year: 1880, Precision: PrecisionYear, Modifier: ModifierExact},
		Until:       &FactDate{Year: 1882, Precision: PrecisionYear, Modifier: ModifierExact},
		Parish:      &TextRef{Text: "Никольский приход", Ref: testID(TypeParish), Type: TypeParish},
		Settlements: []TextRef{{Text: "Давыдово", Ref: testID(TypeAdministrativeDivision), Type: TypeAdministrativeDivision}},
		Notes:       []TextRef{{Text: "книга за три года"}},
		Sources:     []SourceLink{{CitationID: testID(TypeCitation)}},
		Private:     true,
	}
}

func TestArchiveDocumentValidate(t *testing.T) {
	if err := validArchiveDocument().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}
	// Вид — свободный текст и необязателен.
	min := &ArchiveDocument{ID: testID(TypeArchiveDocument), UnitID: testID(TypeArchiveNode), Title: "карточка 12"}
	if err := min.Validate(); err != nil {
		t.Errorf("минимальный документ: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*ArchiveDocument)
		field  string
	}{
		{"плохой id", func(d *ArchiveDocument) { d.ID = "" }, "id"},
		{"пустой unit_id", func(d *ArchiveDocument) { d.UnitID = "" }, "unit_id"},
		{"unit_id — не узел", func(d *ArchiveDocument) { d.UnitID = testID(TypeArchive) }, "unit_id"},
		{"пустое название", func(d *ArchiveDocument) { d.Title = "" }, "title"},
		{"название из пробелов", func(d *ArchiveDocument) { d.Title = "\t" }, "title"},
		{"начало позже конца", func(d *ArchiveDocument) { d.Since, d.Until = d.Until, d.Since }, "since"},
		{"приход — не приход", func(d *ArchiveDocument) { d.Parish = &TextRef{Ref: testID(TypeChurch), Type: TypeChurch} }, "parish.type"},
		{"населённый пункт — не деление", func(d *ArchiveDocument) {
			d.Settlements[0] = TextRef{Ref: testID(TypeChurch), Type: TypeChurch}
		}, "settlements[0].type"},
		{"пустая заметка", func(d *ArchiveDocument) { d.Notes = append(d.Notes, TextRef{}) }, "notes[1].text"},
		{"плохая цитата", func(d *ArchiveDocument) { d.Sources[0].CitationID = "x" }, "sources[0].citation_id"},
	}
	for _, tt := range tests {
		d := validArchiveDocument()
		tt.mutate(d)
		wantInvalid(t, tt.name, d.Validate(), TypeArchiveDocument, tt.field)
	}
}

func validAttachment() *Attachment {
	return &Attachment{
		ID:         testID(TypeAttachment),
		Kind:       AttachmentKindScan,
		URI:        "file:///archive/gako/f33/o6/d264/0012.jpg",
		Filename:   "0012.jpg",
		MIME:       "image/jpeg",
		Page:       12,
		NodeID:     testID(TypeArchiveNode),
		DocumentID: idPtr(testID(TypeArchiveDocument)),
		Note:       "скан разворота",
		Private:    true,
	}
}

func TestAttachmentValidate(t *testing.T) {
	if err := validAttachment().Validate(); err != nil {
		t.Fatalf("образец: %v", err)
	}
	// Достаточно uri или filename; страница, документ и MIME необязательны.
	byURI := &Attachment{ID: testID(TypeAttachment), Kind: AttachmentKindAudio, URI: "https://example.org/a.mp3", NodeID: testID(TypeArchiveNode)}
	if err := byURI.Validate(); err != nil {
		t.Errorf("только uri: %v", err)
	}
	byName := &Attachment{ID: testID(TypeAttachment), Kind: AttachmentKindPhoto, Filename: "photo.png", NodeID: testID(TypeArchiveNode)}
	if err := byName.Validate(); err != nil {
		t.Errorf("только filename: %v", err)
	}
	withParams := validAttachment()
	withParams.MIME = "text/plain; charset=utf-8"
	if err := withParams.Validate(); err != nil {
		t.Errorf("MIME с параметрами: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Attachment)
		field  string
	}{
		{"плохой id", func(a *Attachment) { a.ID = "att" }, "id"},
		{"пустой вид", func(a *Attachment) { a.Kind = "" }, "kind"},
		{"неизвестный вид", func(a *Attachment) { a.Kind = "video" }, "kind"},
		{"ни uri, ни filename", func(a *Attachment) { a.URI, a.Filename = "", "" }, "uri"},
		{"uri и filename из пробелов", func(a *Attachment) { a.URI, a.Filename = " ", " " }, "uri"},
		{"MIME без слэша", func(a *Attachment) { a.MIME = "image" }, "mime"},
		{"MIME с пробелом", func(a *Attachment) { a.MIME = "image /jpeg" }, "mime"},
		{"отрицательная страница", func(a *Attachment) { a.Page = -1 }, "page"},
		{"пустой node_id", func(a *Attachment) { a.NodeID = "" }, "node_id"},
		{"node_id — не узел", func(a *Attachment) { a.NodeID = testID(TypeArchive) }, "node_id"},
		{"document_id — не документ", func(a *Attachment) { a.DocumentID = idPtr(testID(TypeArchiveNode)) }, "document_id"},
		{"указатель на пустой document_id", func(a *Attachment) { a.DocumentID = idPtr("") }, "document_id"},
	}
	for _, tt := range tests {
		a := validAttachment()
		tt.mutate(a)
		wantInvalid(t, tt.name, a.Validate(), TypeAttachment, tt.field)
	}
}
```

- [ ] **Step 2: Убедиться, что тесты не компилируются**

Run: `go test ./internal/models`
Expected: FAIL — `n.Validate undefined` (`ArchiveNode`), `ArchiveDocument`, `Attachment`.

- [ ] **Step 3: `ArchiveNode`**

`internal/models/archive_node_validate.go`:

```go
package models

// Validate проверяет узел цепочки хранения: уровень обязателен (открытый enum
// — системы иерархии архивов расширяются данными, а не кодом); архив — archive;
// родитель — archive_node, не совпадающий с самим узлом (циклы длиннее и
// соответствие уровня системе архива проверяют сценарии); шифр обязателен;
// период, приход и населённые пункты корректны.
func (n *ArchiveNode) Validate() error {
	return finish(TypeArchiveNode, n.validate())
}

func (n *ArchiveNode) validate() *ValidationError {
	if e := idErr("id", n.ID, TypeArchiveNode); e != nil {
		return e
	}
	if !validOpenEnum(string(n.Type)) {
		return fieldErr("type", "недопустимый уровень узла %q: ожидается [a-z][a-z0-9_-]*", n.Type)
	}
	if e := idErr("archive_id", n.ArchiveID, TypeArchive); e != nil {
		return e
	}

	if e := validateOptionalIDPtr("parent_id", n.ParentID, TypeArchiveNode); e != nil {
		return e
	}
	if e := validateNotSelf(n.ID, n.ParentID); e != nil {
		return e
	}
	if e := requireText("label", n.Label); e != nil {
		return e
	}

	if e := validatePeriod(n.Since, n.Until); e != nil {
		return e
	}
	if e := validateOptionalTextRefPtr("parish", n.Parish, TypeParish); e != nil {
		return e
	}
	if e := validateTextRefs("settlements", n.Settlements, TypeAdministrativeDivision); e != nil {
		return e
	}
	if e := validateTextRefs("notes", n.Notes, ""); e != nil {
		return e
	}

	return validateSourceLinks("sources", n.Sources)
}
```

- [ ] **Step 4: `ArchiveDocument`**

`internal/models/archive_document_validate.go`:

```go
package models

// Validate проверяет документ внутри единицы учёта: единица — archive_node,
// название обязательно; вид — свободный необязательный текст; период, приход и
// населённые пункты корректны.
func (d *ArchiveDocument) Validate() error {
	return finish(TypeArchiveDocument, d.validate())
}

func (d *ArchiveDocument) validate() *ValidationError {
	if e := idErr("id", d.ID, TypeArchiveDocument); e != nil {
		return e
	}
	if e := idErr("unit_id", d.UnitID, TypeArchiveNode); e != nil {
		return e
	}
	if e := requireText("title", d.Title); e != nil {
		return e
	}

	if e := validatePeriod(d.Since, d.Until); e != nil {
		return e
	}
	if e := validateOptionalTextRefPtr("parish", d.Parish, TypeParish); e != nil {
		return e
	}
	if e := validateTextRefs("settlements", d.Settlements, TypeAdministrativeDivision); e != nil {
		return e
	}
	if e := validateTextRefs("notes", d.Notes, ""); e != nil {
		return e
	}

	return validateSourceLinks("sources", d.Sources)
}
```

- [ ] **Step 5: `Attachment`**

`internal/models/attachment_validate.go`:

```go
package models

import (
	"mime"
	"strings"
)

// Validate проверяет файловое вложение: вид — закрытый enum; нужен uri или
// имя файла; MIME (если задан) — тип/подтип; страница не отрицательна (0 —
// не указана); узел — archive_node, документ (если задан) — archive_document.
func (a *Attachment) Validate() error {
	return finish(TypeAttachment, a.validate())
}

func (a *Attachment) validate() *ValidationError {
	if e := idErr("id", a.ID, TypeAttachment); e != nil {
		return e
	}
	if !a.Kind.Valid() {
		return fieldErr("kind", "недопустимый вид вложения %q", a.Kind)
	}
	if strings.TrimSpace(a.URI) == "" && strings.TrimSpace(a.Filename) == "" {
		return fieldErr("uri", "нужен uri или имя файла")
	}

	if a.MIME != "" {
		mediaType, _, err := mime.ParseMediaType(a.MIME)
		if err != nil || !strings.Contains(mediaType, "/") {
			return fieldErr("mime", "недопустимый MIME-тип %q: ожидается тип/подтип", a.MIME)
		}
	}
	if a.Page < 0 {
		return fieldErr("page", "номер страницы не может быть отрицательным: %d", a.Page)
	}

	if e := idErr("node_id", a.NodeID, TypeArchiveNode); e != nil {
		return e
	}

	return validateOptionalIDPtr("document_id", a.DocumentID, TypeArchiveDocument)
}
```

- [ ] **Step 6: Прогнать тесты**

Run: `gofmt -l internal/models` → пусто.
Run: `go vet ./internal/models && go test ./internal/models`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/models/archive_node_validate.go internal/models/archive_document_validate.go internal/models/attachment_validate.go internal/models/archive_chain_validate_test.go
git commit -m "feat(models): Validate для ArchiveNode, ArchiveDocument и Attachment

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

### Task 7: Документация и рубеж

**Files:**
- Modify: `docs/data-model/core-read-write.md` (§2.3)
- Modify: `docs/plans/2026-09-20-core-rw-roadmap.md` (S4)

**Interfaces:**
- Consumes: реализованное поведение Task 1–6.
- Produces: документы без утверждений, противоречащих коду.

- [ ] **Step 1: Правила S4 в спеке**

В `docs/data-model/core-read-write.md`, §2.3 «Инварианты (`Validate`)», в конец списка «Сущностные правила» (после пункта про `Family.Name` и `canonical`) добавь:

```markdown
  - Открытые enum'ы: `EventType`, `NoteKind`, `RepositoryType`, `RelationType` и
    `ArchiveNodeType` (уровни систем иерархии архивов задаются данными, а не кодом;
    соответствие уровня системе архива проверяют сценарии). Закрытые:
    `AdminDivisionType`, `SourceKind`, `Reliability`, `AttachmentKind`.
  - Ожидаемые типы ссылок: `Settlements`, `Items`, `Successors` — на
    `administrative_division`; `Church.Parish`, `ArchiveNode.Parish`,
    `ArchiveDocument.Parish` — на `parish`; `Parish.Church` — на `church`;
    `Source.RepositoryID`, `Archive.RepositoryID` — на `repository`;
    `ArchiveNode.ArchiveID` — на `archive`; `ArchiveDocument.UnitID`,
    `Attachment.NodeID` — на `archive_node`; `Attachment.DocumentID` — на
    `archive_document`; `Citation.SourceID` — на `source`;
    `EventParticipant.PersonID` — на `person`.
  - `ParentID` у `AdministrativeDivision`, `ArchiveNode`, `Note` — сущность того же
    типа и не она сама; более длинные циклы проверяют сценарии.
  - `PlaceRef` — как `TextRef`, а ссылка только на `administrative_division`,
    `church` или `parish`. `NamedPeriod`: наименование обязательно, `since`/`until` —
    строки формата `ParseFactDate` (пустое допустимо), начало не позже конца.
  - `Archive.System` — только текст (имя системы иерархии, без ссылки на сущность).
  - `Citation`: `SourceID` обязателен, якорь и текст необязательны. Якорь: `ArchiveAnchor` —
    `node_id` (`archive_node`), `document_id` (необязателен), `page` не меньше 1;
    `FileAnchor` — `attachment_id`; `URLAnchor` — абсолютный `http(s)`-адрес с хостом;
    `rect` и `timecode` — свободные строки.
  - `Event`: вид обязателен, у участника обязательны персона и роль. `Source`: вид,
    название и достоверность обязательны. `Note`: нужен заголовок или текст.
    `Attachment`: нужен `uri` или имя файла, MIME — «тип/подтип», страница не
    отрицательна (0 — не указана). Названия и шифры (`Name`, `Title`, `Label`)
    обязательны там, где это указано в `docs/models/*` (не пусты после обрезки
    пробелов).
```

- [ ] **Step 2: Дорожная карта**

В `docs/plans/2026-09-20-core-rw-roadmap.md`, раздел S4, в конец добавь строку:

```markdown
- **Решения этапа:** `ArchiveNodeType` — открытый enum; `Archive.System` — только
  текст; `PlaceRef` проверяется как `TextRef` с типами
  `administrative_division`/`church`/`parish`; `NamedPeriod` — строки формата
  `ParseFactDate`; план — `2026-09-21-core-rw-s04-validation.md`.
```

Пункты списка «Предпосылки из S3» (a)–(e) оставь: они разрешены этим этапом, а решения перечислены строкой выше.

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
git add docs/data-model/core-read-write.md docs/plans/2026-09-20-core-rw-roadmap.md
git commit -m "docs: правила валидации S4 в спеке и решения этапа в дорожной карте

Co-Authored-By: Claude Sonnet 5 <noreply@anthropic.com>"
```

---

## Самопроверка плана

- **Покрытие S4 (roadmap):** `Validate` для `AdministrativeDivision`, `Church`, `Parish` (Task 3), `Event`+участники, `Source` (Task 4), `Citation` + якорь (Task 2), `Note`, `Repository`, `Archive` (Task 5), `ArchiveNode`, `ArchiveDocument`, `Attachment` (Task 6); предпосылки из S3 — `validateOptionalID`/`Ptr`, `PlaceRef` с тремя типами, `NamedPeriod`, классификация `ArchiveNodeType`, `*TextRef` — Task 1 и решения в Task 7.
- **Согласованность имён:** хелперы Task 1 (`requireText`, `validateStrings`, `validateOptionalID`, `validateOptionalIDPtr`, `validateNotSelf`, `validateOptionalTextRefPtr`, `validateRenames`, `PlaceRef.validate`, `NamedPeriod.validate`) используются в Task 2–6; `validateAnchor` (Task 2) — в `Citation.validate`; тестовые хелперы `idPtr` (Task 1) и уже существующие `testID`, `wantInvalid`, `personBID` — в Task 1–6.
- **Не входит в этап:** вызов `Validate()` из сценариев и проверки с хранилищем (существование ссылок, циклы, соответствие якоря `Source.kind`, уровня узла — системе архива) — S14; `internal/definitions` и системы иерархии архивов.
