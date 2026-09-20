# S1+S2: Нормализованная модель сущностей и полная колоночная схема БД — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Заменить плоский персистентный слой (JSON-блоб в таблице `entity`) на нормализованную доменную модель `internal/models` и полную колоночную схему SQLite; вынести форму JSON на проводе в слой DTO `internal/transport`; упразднить `internal/entity`.

**Architecture:** `internal/models` — единственный внутренний пакет с данными: сущности, value-типы, enum-домены, `ID`, `Type`; без JSON-тегов и внешних зависимостей. `internal/transport` — DTO с JSON-тегами и конвертеры `models → DTO`, его импортируют только `httpapi` и `mcp`. `internal/storage` переписывается: схема `schema_version=0`, типизированные таблицы, `search_index` с lower-терминами из Go. `internal/store` расширяется на все типы на `models`; `sqlstore` маппит сущности наборами строк. Сценарий `list_settlements` отдаёт `[]models.AdministrativeDivision`, обработчики конвертируют результат в `transport.Settlement`.

```
models     — домен, без импортов и без JSON-тегов
store, usecases, definitions, storage → models
transport  → models
httpapi, mcp → интерфейсы сценариев + transport
entity     — удаляется в Task 13
```

**Tech Stack:** Go 1.26.4, `modernc.org/sqlite`, `golang.org/x/text/unicode/norm` (уже в go.mod), `go:generate mockgen` (uber), `github.com/mark3labs/mcp-go` v0.58.0 (не трогаем), `encoding/json` (только в `transport` и обработчиках, не в `models` и не в персистенции).

**Spec:** `docs/data-model/normalization-s1s2.md`; модельные доки `docs/models/*`; решения `docs/data-model/decisions.md`.

## Global Constraints

- `Metadata map[string]string` удаляется из всех сущностей, моделей и БД — полей с произвольной структурой нигде нет.
- `Settlement` удаляется из `models`, sqlstore и порта. Населённые пункты — `AdministrativeDivision` с типом-видом нас. пункта.
- Публичные имена контрактов не переименовываются в этом проходе: тул MCP `settlement_list` и `GET /api/settlements` остаются; форма ответа — DTO `transport.Settlement{id, name, type}`.
- `models` не содержит JSON-тегов и не импортирует `encoding/json`; форма JSON на проводе определяется только в `transport`.
- Enum-домены и `ID` — номинальные типы в `models`; `transport` не алиасит их, а ссылается на `models.X` напрямую. Прочие атрибуты-строки остаются базовым `string`.
- Дискриминатор сущности — метод `EntityType() Type` (не `Type()`: у ряда структур есть поле `Type`, метод с тем же именем не компилируется).
- Тип сущности `first_name` переименован в `given_name`; таблица словаря — `given_names`.
- Цепочка доказательств: `SourceLink.CitationID` → `Citation.SourceID` → `Source`; колонка `source_links.citation_id`.
- `schema_version` = `0`; миграции данных нет (потеря разрешена `docs/todo.md`).
- `go:generate mockgen` строка в файлах-интерфейсах сохраняется; сгенерированные `deps_test.go` обновляются через `go generate ./...`.
- Проверка рубежа: `gofmt -l .` пусто, `go build ./...`, `go vet ./...`, `go test ./...`, `grep -rn "internal/entity" .` пусто, `grep -rn 'json:"' internal/models` пусто.
- Один файл — один struct (методы живут в том же файле, что и struct); snake_case имена файлов по struct.
- Всё в `docs/` — на русском; комментарии в коде — на русском (как существующий код).

---

## Порядок и «зелёный» статус

- **Part A (Task 1–8, `models`)** и **Part B (Task 9, `transport`)** аддитивны: `internal/entity` не трогается, старые потребители (`store`, `usecases`, `definitions`, `httpapi`, `mcp`) продолжают собираться. Дерево зелёное после каждого task.
- **Part C (Task 10–12, `storage` → порт → `sqlstore`)** — единственный красный участок: переписанный `storage` ломает старый `sqlstore` до Task 12, а порт на `models` — старый `usecases/list_settlements` до Task 13. Task 10–13 выполняются подряд; зелёная сборка возвращается в Task 13, там же удаляется `internal/entity`.
- **Part D (Task 13)** переключает обработчики на `transport`, сценарий и `definitions` — на `models`, удаляет `models.Settlement` и пакет `entity`.

---

## PART A — Домен: `internal/models`

### Task 1: `ID`, `Type` и value-типы в `internal/models` без JSON-тегов

**Files:**
- Create: `internal/models/id.go`
- Create: `internal/models/type.go`
- Create: `internal/models/value_types_test.go`
- Modify: `internal/models/text_ref.go` (снять теги, `Ref ID`, `Type Type`)
- Modify: `internal/models/anchor.go` (снять теги, id-поля типа `ID`)
- Modify: `internal/models/fact_date.go` (снять JSON-теги с полей `FactDate`)

**Interfaces:**
- Produces:
  - `type ID string`; `type Type string` + константы `TypePerson … TypeAttachment` (дискриминатор сущности, замена `entity.EntityType`).
  - `TextRef{ Text string; Ref ID; Type Type }`, `NamedPeriod{ Text, Since, Until string }`, `PlaceRef{ Text string; Ref ID; Type Type }`.
  - `ArchiveAnchor{ NodeID ID; DocumentID ID; Page int; Rect string }`, `FileAnchor{ AttachmentID ID; Timecode string }`, `URLAnchor{ URL string }` — интерфейс `Anchor` с методом `Kind() AnchorKind` не меняется.
  - `FactDate` — те же поля, `Compare`, `ParseFactDate`, `String()`; без JSON-тегов.
- `internal/entity` не трогается: старые потребители (`store`, `usecases`, `definitions`) собираются как раньше.

- [ ] **Step 1: Написать падающий тест**

`internal/models/value_types_test.go`:

```go
package models

import "testing"

// Ссылки value-типов — строгие ID/Type, а не произвольные строки.
func TestValueTypesUseStrictRefs(t *testing.T) {
	r := TextRef{Text: "Давыдово", Ref: ID("ad-1"), Type: TypeAdministrativeDivision}
	if r.Ref != ID("ad-1") || r.Type != TypeAdministrativeDivision {
		t.Fatalf("TextRef: %+v", r)
	}
	a := &ArchiveAnchor{NodeID: ID("n-1"), DocumentID: ID("d-1"), Page: 3}
	if a.Kind() != AnchorArchive {
		t.Fatalf("Kind() = %q, want %q", a.Kind(), AnchorArchive)
	}
	f := &FileAnchor{AttachmentID: ID("att-1")}
	if f.Kind() != AnchorFile {
		t.Fatalf("Kind() = %q, want %q", f.Kind(), AnchorFile)
	}
}
```

- [ ] **Step 2: Убедиться, что тест не компилируется**

Run: `go test ./internal/models`
Expected: FAIL — `undefined: ID` / `undefined: TypeAdministrativeDivision`.

- [ ] **Step 3: ID и Type**

`internal/models/id.go`:

```go
package models

// ID — строгий идентификатор сущности и целевой строгой ссылки.
type ID string
```

`internal/models/type.go`:

```go
package models

// Type — тип сущности генеалогии (дискриминатор хранилища и поискового индекса).
//
// Каталог типов соответствует решениям docs/data-model/decisions.md:
//
//	#1  брак/погребение — значения Event.type, отдельных сущностей нет
//	#2  единая рекурсивная AdministrativeDivision (без Governorate/District/Volost)
//	#14 архивы: рекурсивный ArchiveNode (без типизированных Fund/Inventory/Case)
//	#15 ArchiveDocument
//	#16 Attachment
//	#17 населённый пункт — вид AdministrativeDivision.type (без типа settlement)
//	#21 цитирование: Citation (промежуточное звено доказательства)
//	#22 Note (заметки первого класса)
//	#25 Repository (хранилище-контейнер источников)
type Type string

const (
	TypePerson                 Type = "person"
	TypeSurname                Type = "surname"
	TypeGivenName              Type = "given_name"
	TypePatronymic             Type = "patronymic"
	TypeEstate                 Type = "estate"
	TypeTitle                  Type = "title"
	TypeChurch                 Type = "church"
	TypeParish                 Type = "parish"
	TypeAdministrativeDivision Type = "administrative_division"
	TypeEvent                  Type = "event"
	TypeSource                 Type = "source"
	TypeCitation               Type = "citation"
	TypeNote                   Type = "note"
	TypeRepository             Type = "repository"
	TypeRelation               Type = "relation"
	TypeResidence              Type = "residence"
	TypeFamily                 Type = "family"
	TypeArchive                Type = "archive"
	TypeArchiveNode            Type = "archive_node"
	TypeArchiveDocument        Type = "archive_document"
	TypeAttachment             Type = "attachment"
)
```

- [ ] **Step 4: TextRef, NamedPeriod, PlaceRef без тегов**

`internal/models/text_ref.go` (заменить целиком):

```go
package models

// TextRef — универсальный «текст-или-ссылка»: элемент может быть либо строкой,
// либо ссылкой на сущность.
type TextRef struct {
	// Text — отображаемая строка (обязательна).
	Text string
	// Ref — id целевой сущности (пусто, если элемент — просто текст).
	Ref ID
	// Type — тип целевой сущности (задаётся вместе с Ref).
	Type Type
}

// NamedPeriod — наименование с периодом действия (открытым с одной или обеих сторон).
type NamedPeriod struct {
	Text  string
	Since string
	Until string
}

// PlaceRef — указание на место: TextRef, чья ссылка (если есть) ведёт на сущность
// места (AdministrativeDivision | Church | Parish). Свободный текст — когда
// сущности места ещё нет.
type PlaceRef struct {
	Text string
	Ref  ID
	Type Type
}
```

- [ ] **Step 5: Anchor без тегов, id-поля типа ID**

В `internal/models/anchor.go` замени три struct (интерфейс `Anchor`, `AnchorKind`, константы и методы `Kind()` не меняются):

```go
// ArchiveAnchor — скан страницы единицы учёта в архиве.
type ArchiveAnchor struct {
	// NodeID — узел цепочки (единица учёта).
	NodeID ID
	// DocumentID — уточнение до документа внутри единицы (опционально).
	DocumentID ID
	// Page — номер скана/страницы.
	Page int
	// Rect — координаты области выделения на изображении (опционально).
	Rect string
}

// FileAnchor — файл с тайминговой привязкой.
type FileAnchor struct {
	// AttachmentID — вложение.
	AttachmentID ID
	// Timecode — тайм-метка для аудио/видео (опционально).
	Timecode string
}

// URLAnchor — внешняя ссылка.
type URLAnchor struct {
	// URL — адрес.
	URL string
}
```

- [ ] **Step 6: FactDate без JSON-тегов**

Run: `perl -pi -e 's/\s*`json:"[^`]*"`//' internal/models/fact_date.go && gofmt -w internal/models/fact_date.go`

Проверь: `grep -n 'json:' internal/models/fact_date.go` — пусто. Логика (`Compare`, `ParseFactDate`, `String()`) не меняется.

- [ ] **Step 7: Прогнать тесты**

Run: `gofmt -l internal/models` → пусто.
Run: `go test ./internal/models`
Expected: PASS (новый тест и существующие `FactDate`-тесты).

Run: `go build ./...`
Expected: PASS. `internal/models.SourceLink` и старые struct пока со старыми тегами — они правятся в Task 2 и Task 3–8; `TestNoJSONTags` (Task 8) закрывает всю группу.

- [ ] **Step 8: Commit**

```bash
git add internal/models
git commit -m "feat(models): ID, Type; value-типы без JSON-тегов, ссылки строгими ID/Type"
```

### Task 2: Enum-домены и `SourceLink` в `internal/models`

**Files:**
- Create: `internal/models/person_name_type.go`
- Create: `internal/models/relation_kind.go`
- Create: `internal/models/relation_type.go` (новый домен, решение #23)
- Create: `internal/models/note_kind.go` (новый домен, решение #22)
- Create: `internal/models/repository_type.go` (новый домен, решение #25)
- Create: `internal/models/fact_calendar.go` (новый домен, решение #26)
- Create: `internal/models/source_kind.go`
- Create: `internal/models/attachment_kind.go`
- Create: `internal/models/reliability.go`
- Create: `internal/models/administrative_division_type.go`
- Create: `internal/models/administrative_division_system.go`
- Create: `internal/models/administrative_relation.go`
- Create: `internal/models/name_gender.go`
- Modify: `internal/models/source_link.go` (заменяется целиком: JSON-теги и `SourceID` уходят)

**Interfaces:**
- Consumes: `ID`, `Type` из Task 1.
- Produces:
  - `type PersonNameType string` + константы.
  - `type RelationKind string` + константы (включая `associate`, решение #23).
  - `type RelationType string` + константы (вид ребра для `kind=associate`, решение #23).
  - `type NoteKind string` + константы (решение #22).
  - `type RepositoryType string` + константы (решение #25).
  - `type FactCalendar string` + константы (решение #26).
  - `type SourceKind string` + константы.
  - `type AttachmentKind string` + константы.
  - `type Reliability string` + константы.
  - `type AdminDivisionType string` + константы и метод `IsSettlement() bool` (для сценария list_settlements).
  - `type NameGender string` + константы `MaleName`/`FemaleName`/`NeutralName`.
  - `SourceLink{ CitationID ID; TargetType Type; TargetID ID; Reliability Reliability; Role string; Note string }` (решение #21: `SourceLink` ссылается на `Citation`).
  - Доработанные `AdministrativeDivisionSystem` (поле `Relations []AdminDivisionType`) и `AdministrativeDivisionTypeRelation` (поля `Parent`/`Child AdminDivisionType`).

- [ ] **Step 1: PersonNameType**

`internal/models/person_name_type.go`:

```go
package models

// PersonNameType — вид имени персоны (много имён у одной персоны).
type PersonNameType string

const (
	PersonNameMain      PersonNameType = "main"
	PersonNameBirth     PersonNameType = "birth"
	PersonNameMarried   PersonNameType = "married"
	PersonNameChanged   PersonNameType = "changed"
	PersonNamePseudonym PersonNameType = "pseudonym"
)
```


- [ ] **Step 2: RelationKind**

`internal/models/relation_kind.go`:

```go
package models

// RelationKind — вид ребра графа родства (решение #23).
type RelationKind string

const (
	RelationKindBlood     RelationKind = "blood"
	RelationKindMarriage  RelationKind = "marriage"
	RelationKindAdoption  RelationKind = "adoption"
	RelationKindAssociate RelationKind = "associate"
)
```

- [ ] **Step 3: RelationType (решение #23)**

`internal/models/relation_type.go`:

```go
package models

// RelationType — вид связи для kind=associate (нет родства: сосед, коллега,
// кум, свидетель, друг и т.п.). Значения свободные и расширяемые; не влияют на
// построение дерева родства.
type RelationType string

const (
	RelationTypeNeighbor  RelationType = "neighbor"
	RelationTypeColleague RelationType = "colleague"
	RelationTypeGodparent RelationType = "godparent"
	RelationTypeWitness   RelationType = "witness"
	RelationTypeFriend    RelationType = "friend"
)
```

Значения-константы отражают текущие потребности; при необходимости расширяются без миграции (свободный enum).

- [ ] **Step 4: NoteKind (решение #22)**

`internal/models/note_kind.go`:

```go
package models

// NoteKind — тип заметки как самостоятельной сущности (решение #22).
// Значения свободные и расширяемые.
type NoteKind string

const (
	NoteKindNote    NoteKind = "note"
	NoteKindArticle NoteKind = "article"
	NoteKindBook    NoteKind = "book"
	NoteKindChapter NoteKind = "chapter"
)
```

- [ ] **Step 5: RepositoryType (решение #25)**

`internal/models/repository_type.go`:

```go
package models

// RepositoryType — тип хранилища-контейнера источников (решение #25).
// Значения свободные и расширяемые.
type RepositoryType string

const (
	RepositoryTypeArchive RepositoryType = "archive"
	RepositoryTypeLibrary RepositoryType = "library"
	RepositoryTypeMuseum  RepositoryType = "museum"
	RepositoryTypePrivate RepositoryType = "private"
	RepositoryTypeOther   RepositoryType = "other"
)
```

- [ ] **Step 6: FactCalendar (решение #26)**

`internal/models/fact_calendar.go`:

```go
package models

// FactCalendar — календарь, в котором записана дата.
type FactCalendar string

const (
	FactCalendarGregorian FactCalendar = "gregorian"
	FactCalendarJulian    FactCalendar = "julian"
	FactCalendarUnknown   FactCalendar = "unknown"
)
```

- [ ] **Step 7: SourceKind**

`internal/models/source_kind.go`:

```go
package models

// SourceKind — тип доказательства.
type SourceKind string

const (
	SourceKindArchivalScan  SourceKind = "archival-scan"
	SourceKindTranscription SourceKind = "transcription"
	SourceKindDocument      SourceKind = "document"
	SourceKindAudio         SourceKind = "audio"
	SourceKindPhoto         SourceKind = "photo"
	SourceKindMemory        SourceKind = "memory"
	SourceKindExternal      SourceKind = "external"
)
```

- [ ] **Step 8: AttachmentKind**

`internal/models/attachment_kind.go`:

```go
package models

// AttachmentKind — тип файлового вложения.
type AttachmentKind string

const (
	AttachmentKindScan     AttachmentKind = "scan"
	AttachmentKindDocument AttachmentKind = "document"
	AttachmentKindAudio    AttachmentKind = "audio"
	AttachmentKindPhoto    AttachmentKind = "photo"
)
```

- [ ] **Step 9: Reliability**

`internal/models/reliability.go`:

```go
package models

// Reliability — градация достоверности доказательства или утверждения.
type Reliability string

const (
	ReliabilityPrimary      Reliability = "primary"
	ReliabilityContemporary Reliability = "contemporary"
	ReliabilityMemory       Reliability = "memory"
	ReliabilityIndirect     Reliability = "indirect"
	ReliabilityUnknown      Reliability = "unknown"
)
```

- [ ] **Step 10: AdminDivisionType**

Создай `internal/models/administrative_division_type.go`:

```go
package models

// AdminDivisionType — тип единицы административного деления: уровень деления
// или вид населённого пункта (решение #17). Один рекурсивный узел,
// глубина и состав системы не фиксированы.
type AdminDivisionType string

// Единицы деления.
const (
	AdminDivisionGovernorate AdminDivisionType = "governorate"
	AdminDivisionDistrict    AdminDivisionType = "district"
	AdminDivisionVolost      AdminDivisionType = "volost"
	AdminDivisionOther       AdminDivisionType = "other"
)

// Виды населённых пунктов (решение #17).
const (
	AdminDivisionGorod     AdminDivisionType = "gorod"
	AdminDivisionSelo      AdminDivisionType = "selo"
	AdminDivisionDerevnya  AdminDivisionType = "derevnya"
	AdminDivisionHutor     AdminDivisionType = "hutor"
	AdminDivisionPogost    AdminDivisionType = "pogost"
	AdminDivisionStanitsa  AdminDivisionType = "stanitsa"
	AdminDivisionMestechko AdminDivisionType = "mestechko"
)

// isDivisionUnit — единицы деления (не населённые пункты).
func isDivisionUnit(t AdminDivisionType) bool {
	switch t {
	case AdminDivisionGovernorate, AdminDivisionDistrict, AdminDivisionVolost, AdminDivisionOther:
		return true
	default:
		return false
	}
}

// IsSettlement сообщает, является ли тип видом населённого пункта.
func (t AdminDivisionType) IsSettlement() bool {
	return !isDivisionUnit(t)
}
```

- [ ] **Step 11: Система и связь деления на новом типе**

`internal/models/administrative_division_system.go`:

```go
package models

type AdministrativeDivisionSystem struct {
	Name      string
	Relations []AdminDivisionType
}
```

`internal/models/administrative_relation.go`:

```go
package models

type AdministrativeDivisionTypeRelation struct {
	Parent AdminDivisionType
	Child  AdminDivisionType
}
```

- [ ] **Step 12: NameGender**

`internal/models/name_gender.go`:

```go
package models

// NameGender — пол, выводимый из имени (словарь GivenName).
type NameGender string

const (
	MaleName    NameGender = "male"
	FemaleName  NameGender = "female"
	NeutralName NameGender = "neutral"
)
```

- [ ] **Step 13: SourceLink**

`internal/models/source_link.go` (заменяет старый файл целиком: без JSON-тегов, `CitationID` вместо `SourceID`):

```go
package models

// SourceLink — доказательство: связь «утверждение → цитата» (решение #21).
// Применяется к любому утверждению любой сущности. Цепочка:
// SourceLink.CitationID → Citation.SourceID → Source.
type SourceLink struct {
	// CitationID — id цитаты (строгая ссылка).
	CitationID ID
	// TargetType — тип сущности-утверждения.
	TargetType Type
	// TargetID — id сущности-утверждения.
	TargetID ID
	// Reliability — частная достоверность именно этого утверждения по этой цитате.
	Reliability Reliability
	// Role — роль утверждения.
	Role string
	// Note — примечание.
	Note string
}
```

- [ ] **Step 14: Сборка и тесты**

Run: `gofmt -l internal/models` → пусто.
Run: `go build ./... && go test ./internal/models`
Expected: PASS. `internal/entity` не тронут, старые потребители (`store`, `usecases`, `definitions`) собираются как раньше.

- [ ] **Step 15: Commit**

```bash
git add internal/models
git commit -m "feat(models): enum-домены, NameGender, SourceLink на CitationID; система административного деления"
```

### Task 3: Person, PersonName, PersonGender

**Files:**
- Modify: `internal/models/person.go` (переписать: старая плоская форма с Surname/FirstName/Metadata заменяется новой)
- Create: `internal/models/person_name.go`
- Create: `internal/models/person_gender.go`
- Create: `internal/models/person_test.go`
- Create: `internal/models/settlement_test.go`

**Interfaces:**
- Consumes: `ID`, `Type`, `PersonNameType`, `PersonGender`, `TextRef`, `FactDate`, `SourceLink` (все из Task 1–2).
- Produces:
  - `models.Person{ ID ID; Gender PersonGender; Names []PersonName; Estates []TextRef; Titles []TextRef; Nicknames []TextRef; Notes []TextRef; Sources []SourceLink; Private bool }` с методом `EntityType() Type` (решения #24, #28).
  - `models.PersonName{ Type PersonNameType; Surname TextRef; Given TextRef; Patronymic TextRef; Prefix string; Suffix string; Since *FactDate; Until *FactDate }` (`prefix` — фон/де, `suffix` — ст./мл.; решение #28).
  - `PersonGender` (`Male`/`Female`/`Unknown`) — переезжает из `internal/models/person.go` в отдельный файл `person_gender.go`.

- [ ] **Step 1: Сначала красный тест — Person несёт новую форму**

`internal/models/person_test.go`:

```go
package models

import "testing"

func TestPersonFullForm(t *testing.T) {
	p := Person{
		ID:     ID("p-1"),
		Gender: Female,
		Names: []PersonName{
			{
				Type:    PersonNameMain,
				Surname: TextRef{Text: "Дорожкина"},
				Given:   TextRef{Text: "Акилина"},
				Prefix:  "фон",
				// без Suffix — опционально
			},
			{
				Type:    PersonNameBirth,
				Surname: TextRef{Text: "Иванова"},
			},
		},
		Estates:   []TextRef{{Text: "крестьяне"}},
		Titles:    []TextRef{{Text: "вдова"}},
		Nicknames: []TextRef{{Text: "Акилинушка"}},
		Notes:     []TextRef{{Text: "жила в Давыдове"}},
		Sources:   []SourceLink{{CitationID: ID("c-1"), TargetType: TypePerson, TargetID: ID("p-1")}},
		Private:   true,
	}
	if p.EntityType() != TypePerson {
		t.Fatalf("EntityType() = %q, want %q", p.EntityType(), TypePerson)
	}
	if len(p.Names) != 2 {
		t.Fatalf("Names %d, want 2", len(p.Names))
	}
	if len(p.Nicknames) != 1 || !p.Private {
		t.Fatalf("nicknames/private не прочитаны")
	}
	// пол как именованный домен, а не свободная строка:
	if p.Gender == "что-то" {
		t.Fatal("gender должен быть типизирован; сравнение с произвольной строкой не компилируется")
	}
}
```

`internal/models/settlement_test.go`:

```go
package models

import "testing"

// Тест фиксирует решение #17: населённый пункт — вид AdministrativeDivision.type.
func TestSettlementIsAdminDivisionKind(t *testing.T) {
	selo := AdminDivisionType("selo")
	if !selo.IsSettlement() {
		t.Fatal("selo должен быть населённым пунктом")
	}
	volost := AdminDivisionType("volost")
	if volost.IsSettlement() {
		t.Fatal("volost не населённый пункт")
	}
}
```

- [ ] **Step 2: Запустить тест — убедиться, что он не компилируется**

Run: `go test ./internal/models`
Expected: FAIL — `Person` ещё старая форма (компиляция падает).

- [ ] **Step 3: Переписать Person и PersonName**

`internal/models/person.go`:

```go
package models

// Person — персона. Все поля, кроме id, опциональны. Private — приватность
// персональных данных (решение #24).
type Person struct {
	ID        ID
	Gender    PersonGender
	Names     []PersonName
	Estates   []TextRef
	Titles    []TextRef
	Nicknames []TextRef
	Notes     []TextRef
	Sources   []SourceLink
	Private   bool
}

// Type возвращает тип сущности.
func (p *Person) EntityType() Type { return TypePerson }
```

`internal/models/person_name.go`:

```go
package models

// PersonName — одно имя персоны (фамилия/имя/отчество могут быть нескольких
// родов: основное, по браку, при рождении…). Каждое из полей — TextRef
// с мягкой ссылкой на словарную запись (Surname/GivenName/Patronymic).
// Prefix/Suffix — служебные части имени (фон/де, ст./мл.) (решение #28).
type PersonName struct {
	Type       PersonNameType
	Surname    TextRef
	Given      TextRef
	Patronymic TextRef
	Prefix     string
	Suffix     string
	Since      *FactDate
	Until      *FactDate
}
```

- [ ] **Step 4: PersonGender в отдельный файл**

`internal/models/person_gender.go`:

```go
package models

// PersonGender — пол персоны.
type PersonGender string

const (
	Male    PersonGender = "male"
	Female  PersonGender = "female"
	Unknown PersonGender = "unknown"
)
```

Убедись, что в новом `internal/models/person.go` объявления `PersonGender` и констант нет (оно только что переехало сюда). Старый `Metadata`, `Surname`, `FirstName`, `Patronymic` (строки) из `models.Person` исчезают вместе с перезаписью файла; потребителей `models.Person` в репозитории нет.

- [ ] **Step 5: Прогнать тесты**

Run: `gofmt -l internal/models` → пусто.
Run: `go build ./... && go test ./internal/models`
Expected: PASS. Дерево остаётся зелёным: `internal/entity` не тронут, `models.Settlement` пока существует (удаляется в Task 13).

- [ ] **Step 6: Commit**

```bash
git add internal/models
git commit -m "feat(models): новая форма Person/PersonName, PersonGender отдельным файлом"
```

### Task 4: Relation, Residence, Family

**Files:**
- Create: `internal/models/relation.go`
- Create: `internal/models/residence.go`
- Create: `internal/models/family.go`

**Interfaces:**
- Consumes: `ID`, `Type`, `RelationKind`, `RelationType`, `TextRef`, `FactDate`, `SourceLink`, `PersonGender` (для комментариев), `AdminDivisionType` (для Residence.place_id комментария).
- Produces:
  - `models.Relation{ ID ID; Kind RelationKind; RelType RelationType; PersonA ID; PersonB ID; Since *FactDate; Until *FactDate; Sources []SourceLink; Notes []TextRef; Private bool }`, метод `EntityType() Type = TypeRelation` (решения #23, #24).
  - `models.Residence{ ID ID; PersonID ID; PlaceID ID; Since *FactDate; Until *FactDate; Sources []SourceLink; Note string; Private bool }`, `EntityType() Type = TypeResidence` (решение #24).
  - `models.Family{ ID ID; Name string; Members []TextRef; Notes []TextRef; Sources []SourceLink; Private bool }`, `EntityType() Type = TypeFamily` (решение #24).

- [ ] **Step 1: Написать структуры**

`internal/models/relation.go`:

```go
package models

// Relation — ребро графа родства между двумя персонами. RelType задаётся
// только для kind=associate (решение #23). Private — приватность (решение #24).
type Relation struct {
	ID      ID
	Kind    RelationKind
	RelType RelationType
	PersonA ID
	PersonB ID
	Since   *FactDate
	Until   *FactDate
	Sources []SourceLink
	Notes   []TextRef
	Private bool
}

// Type возвращает тип сущности.
func (r *Relation) EntityType() Type { return TypeRelation }
```

`internal/models/residence.go`:

```go
package models

// Residence — проживание персоны в месте (AdministrativeDivision) с периодом.
type Residence struct {
	ID       ID
	PersonID ID
	PlaceID  ID
	Since    *FactDate
	Until    *FactDate
	Sources  []SourceLink
	Note     string
	Private  bool
}

// Type возвращает тип сущности.
func (r *Residence) EntityType() Type { return TypeResidence }
```

`internal/models/family.go`:

```go
package models

// Family — род/линия: группирующая сущность.
type Family struct {
	ID      ID
	Name    string
	Members []TextRef
	Notes   []TextRef
	Sources []SourceLink
	Private bool
}

// Type возвращает тип сущности.
func (f *Family) EntityType() Type { return TypeFamily }
```

- [ ] **Step 2: Проверка**

Run: `gofmt -l internal/models internal/models` → пусто.
Run: `go build ./internal/models`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/models/relation.go internal/models/residence.go internal/models/family.go
git commit -m "feat(models): Relation, Residence, Family"
```

### Task 5: Словари Surname, GivenName, Patronymic, Estate, Title

**Files:**
- Create: `internal/models/surname.go`
- Create: `internal/models/given_name.go`
- Create: `internal/models/patronymic.go`
- Create: `internal/models/estate.go`
- Create: `internal/models/title.go`
- Create: `internal/models/dictionary_test.go`

**Interfaces:**
- Consumes: `ID`, `Type`, `TextRef`, `NameGender`.
- Produces: пять сущностей словарей по одному шаблону:
  - `Surname{ ID ID; Canonical string; Variants []TextRef; Items []TextRef; Notes []TextRef }`, `EntityType() Type = TypeSurname`;
  - `GivenName{ ...; Gender NameGender }`, `EntityType() Type = TypeGivenName`;
  - `Patronymic{ ... }`, `EntityType() Type = TypePatronymic`;
  - `Estate{ ... }`, `EntityType() Type = TypeEstate`;
  - `Title{ ... }`, `EntityType() Type = TypeTitle`.

- [ ] **Step 1: Первый словарь полностью, остальные по шаблону**

`internal/models/surname.go`:

```go
package models

// Surname — словарная запись фамилии.
type Surname struct {
	ID        ID
	Canonical string
	Variants  []TextRef
	Items     []TextRef
	Notes     []TextRef
}

// Type возвращает тип сущности.
func (s *Surname) EntityType() Type { return TypeSurname }
```

`internal/models/given_name.go`:

```go
package models

// GivenName — словарная запись имени; обязателен признак пола.
type GivenName struct {
	ID        ID
	Canonical string
	Gender    NameGender
	Variants  []TextRef
	Items     []TextRef
	Notes     []TextRef
}

// Type возвращает тип сущности.
func (g *GivenName) EntityType() Type { return TypeGivenName }
```

`internal/models/estate.go`:

```go
package models

// Estate — словарная запись сословия/общинного статуса.
type Estate struct {
	ID        ID
	Canonical string
	Variants  []TextRef
	Items     []TextRef
	Notes     []TextRef
}

// Type возвращает тип сущности.
func (e *Estate) EntityType() Type { return TypeEstate }
```

`internal/models/title.go` — аналогично `Estate`, метод `EntityType() Type { return TypeTitle }`.

`internal/models/patronymic.go` — словарь отчеств (словарная сущность отчеств):

```go
package models

// Patronymic — словарная запись отчества.
type Patronymic struct {
	ID        ID
	Canonical string
	Variants  []TextRef
	Items     []TextRef
	Notes     []TextRef
}

// Type возвращает тип сущности.
func (p *Patronymic) EntityType() Type { return TypePatronymic }
```

- [ ] **Step 2: Тест-фиксация формы словарей**

`internal/models/dictionary_test.go`:

```go
package models

import "testing"

func TestDictionaryFullForm(t *testing.T) {
	s := Surname{
		ID:        ID("sur-1"),
		Canonical: "Дорожкин",
		Variants:  []TextRef{{Text: "Дарожкин"}, {Text: "Дорожкина"}},
		Items:     []TextRef{{Text: "Иван Дорожкин", Ref: "p-1", Type: TypePerson}},
	}
	if s.EntityType() != TypeSurname {
		t.Fatalf("EntityType() = %q", s.EntityType())
	}
	if len(s.Variants) != 2 || s.Items[0].Ref != "p-1" {
		t.Fatalf("form lost: %+v", s)
	}
}

func TestGivenNameGenderRequired(t *testing.T) {
	// gender — обязательный номиnalьный домен: без него нельзя строить
	// вывод пола (docs/models/people.md).
	var _ NameGender = NeutralName
	var g GivenName
	if g.Gender != "" {
		t.Fatalf("по умолчанию пусто, но наличие поля обязательно для заполнения")
	}
}
```

- [ ] **Step 3: Проверка и commit**

Run: `gofmt -l internal/models` → пусто; `go build ./internal/models` → PASS.

```bash
git add internal/models/surname.go internal/models/given_name.go internal/models/patronymic.go internal/models/estate.go internal/models/title.go internal/models/dictionary_test.go
git commit -m "feat(models): словари Surname, GivenName, Patronymic, Estate, Title"
```

### Task 6: AdministrativeDivision, Church, Parish

**Files:**
- Modify: `internal/models/administrative_division.go` (переписать)
- Modify: `internal/models/church.go` (переписать)
- Modify: `internal/models/parish.go` (переписать)
- Create: `internal/models/places_test.go`

**Interfaces:**
- Consumes: `ID`, `Type`, `AdminDivisionType`, `TextRef`, `NamedPeriod`, `FactDate`, `SourceLink`.
- Produces:
  - `AdministrativeDivision{ ID ID; Name string; Type AdminDivisionType; ParentID *ID; Items []TextRef; Variants []string; Renames []NamedPeriod; Successors []TextRef; Since *FactDate; Until *FactDate; Notes []TextRef; Sources []SourceLink }`, `EntityType() Type = TypeAdministrativeDivision`.
  - `Church{ ID ID; Name string; Parish *TextRef; Settlements []TextRef; Variants []string; Notes []TextRef; Sources []SourceLink }`, `EntityType() Type = TypeChurch`.
  - `Parish{ ID ID; Name string; Church *TextRef; Settlements []TextRef; Since *FactDate; Until *FactDate; Notes []TextRef; Sources []SourceLink }`, `EntityType() Type = TypeParish`.

- [ ] **Step 1: Переписать структуры**

`internal/models/administrative_division.go`:

```go
package models

// AdministrativeDivision — единая рекурсивная единица административного
// деления; тип совмещает уровень деления и вид населённого пункта (#17).
type AdministrativeDivision struct {
	ID         ID
	Name       string
	Type       AdminDivisionType
	ParentID   *ID
	Items      []TextRef
	Variants   []string
	Renames    []NamedPeriod
	Successors []TextRef
	Since      *FactDate
	Until      *FactDate
	Notes      []TextRef
	Sources    []SourceLink
}

// Type возвращает тип сущности.
func (a *AdministrativeDivision) EntityType() Type { return TypeAdministrativeDivision }
```

`internal/models/church.go`:

```go
package models

// Church — церковь.
type Church struct {
	ID          ID
	Name        string
	Parish      *TextRef
	Settlements []TextRef
	Variants    []string
	Notes       []TextRef
	Sources     []SourceLink
}

// Type возвращает тип сущности.
func (c *Church) EntityType() Type { return TypeChurch }
```

`internal/models/parish.go`:

```go
package models

// Parish — приход.
type Parish struct {
	ID          ID
	Name        string
	Church      *TextRef
	Settlements []TextRef
	Since       *FactDate
	Until       *FactDate
	Notes       []TextRef
	Sources     []SourceLink
}

// Type возвращает тип сущности.
func (p *Parish) EntityType() Type { return TypeParish }
```

- [ ] **Step 2: Тест формы мест**

`internal/models/places_test.go`:

```go
package models

import "testing"

func TestAdministrativeDivisionFullForm(t *testing.T) {
	parent := ID("ad-1")
	a := AdministrativeDivision{
		ID:       ID("ad-2"),
		Name:     "Давыдово",
		Type:     AdminDivisionSelo,
		ParentID: &parent,
		Variants: []string{"Давидово"},
		Renames:  []NamedPeriod{{Text: "Давыдово", Since: "1881", Until: "1917"}},
		Items:    []TextRef{{Text: "дворы"}},
	}
	if !a.Type.IsSettlement() {
		t.Fatal("selo должна считаться населённым пунктом")
	}
	if a.ParentID == nil || *a.ParentID != parent {
		t.Fatalf("strict link lost: %+v", a.ParentID)
	}
}
```

(В этом тесте `derevnya`/`selo` проверяются через `Type.IsSettlement()` (поле).)

- [ ] **Step 3: Проверка и commit**

Run: `gofmt -l internal/models` → пусто; `go build ./internal/models` → PASS.

```bash
git add internal/models/administrative_division.go internal/models/church.go internal/models/parish.go internal/models/places_test.go
git commit -m "feat(models): нормализованные AdministrativeDivision, Church, Parish"
```

### Task 7: Event + EventParticipant, Source, Citation

**Files:**
- Modify: `internal/models/event.go` (переписать)
- Modify: `internal/models/source.go` (переписать — без anchor/text, решение #21)
- Create: `internal/models/citation.go` (новая сущность, решение #21)
- Create: `internal/models/event_participant.go`
- Create: `internal/models/facts_test.go`

**Interfaces:**
- Consumes: `ID`, `Type`, `EventType`, `FactDate`, `PlaceRef`, `SourceKind`, `Anchor`, `Reliability`, `TextRef`, `SourceLink`.
- Produces:
  - `EventParticipant{ PersonID ID; Role string; Note string }`.
  - `Event{ ID ID; Type EventType; Date *FactDate; Place *PlaceRef; Participants []EventParticipant; Sources []SourceLink; Notes []TextRef; Private bool }`, `EntityType() Type = TypeEvent`.
  - `Source{ ID ID; Kind SourceKind; Title string; Author string; Date *FactDate; Reliability Reliability; RepositoryID ID; Notes []TextRef; Private bool }`, `EntityType() Type = TypeSource` (**без полей Anchor/Text** — неструктурированный текст и привязка к архиву живут в `Citation`, решение #21). `RepositoryID` — мягкая (free) ссылка на `Repository` (Task 9).
  - `Citation{ ID ID; SourceID ID; Anchor Anchor; Text string; Note string; Private bool }`, `EntityType() Type = TypeCitation` — цитата из источника: якорь в источнике + неструктурированный текст (решение #21). `SourceID` — строгая ссылка на `Source`.

- [ ] **Step 1: Переписать Event и создать EventParticipant**

`internal/models/event_participant.go`:

```go
package models

// EventParticipant — участник события с ролью в нём.
type EventParticipant struct {
	PersonID ID
	Role     string
	Note     string
}
```

`internal/models/event.go`:

```go
package models

// Event — событие жизненного факта. Private — приватность (решение #24).
type Event struct {
	ID           ID
	Type         EventType
	Date         *FactDate
	Place        *PlaceRef
	Participants []EventParticipant
	Sources      []SourceLink
	Notes        []TextRef
	Private      bool
}

// Type возвращает тип сущности.
func (e *Event) EntityType() Type { return TypeEvent }
```

**Важно:** `Event.Type` — тип `models.EventType` (`event_type.go`), а не свободная строка. Создай `internal/models/event_type.go`:

```go
package models

// EventType — тип события (открытый набор: значения расширяются без миграции).
type EventType string

const (
	EventTypeBirth      EventType = "birth"
	EventTypeDeath      EventType = "death"
	EventTypeMarriage   EventType = "marriage"
	EventTypeBurial     EventType = "burial"
	EventTypeConfession EventType = "confession"
	EventTypeCensus     EventType = "census"
)
```

- [ ] **Step 2: Переписать Source и создать Citation**

`internal/models/source.go`:

```go
package models

// Source — единая абстракция доказательства. Ссылка на хранилище
// (Repository) и приватность — решение #25/#24. Текст/ядро — в Citation,
// а не здесь (решение #21).
type Source struct {
	ID           ID
	Kind         SourceKind
	Title        string
	Author       string
	Date         *FactDate
	Reliability  Reliability
	RepositoryID ID
	Notes        []TextRef
	Private      bool
}

// Type возвращает тип сущности.
func (s *Source) EntityType() Type { return TypeSource }
```

`internal/models/citation.go`:

```go
package models

// Citation — цитата из источника: доказательственная привязка
// (якорь/движимое страницы/тайм-код/url) и выписка текстом.
// Цепочка доказательства: SourceLink.citation_id → Citation.source_id → Source
// (решение #21).
type Citation struct {
	ID       ID
	SourceID ID
	Anchor   Anchor
	Text     string
	Note     string
	Private  bool
}

// Type возвращает тип сущности.
func (c *Citation) EntityType() Type { return TypeCitation }
```

- [ ] **Step 3: Тест**

`internal/models/facts_test.go`:

```go
package models

import (
	"reflect"
	"testing"
)

func TestEventFullForm(t *testing.T) {
	d, _ := ParseFactDate("1881-03-15")
	e := Event{
		ID:    ID("ev-1"),
		Type:  EventTypeBirth,
		Date:  &d,
		Place: &PlaceRef{Text: "Давыдово"},
		Participants: []EventParticipant{
			{PersonID: ID("p-1"), Role: "subject"},
			{PersonID: ID("p-2"), Role: "mother"},
		},
	}
	if e.EntityType() != TypeEvent {
		t.Fatalf("EntityType() = %q", e.EntityType())
	}
	if e.Date.String() != "1881-03-15" {
		t.Fatalf("date lost: %s", e.Date)
	}
}

func TestSourceFullForm(t *testing.T) {
	s := Source{
		ID:           ID("s-1"),
		Kind:         SourceKindArchivalScan,
		Title:        "МК с. Давыдово за 1881",
		Reliability:  ReliabilityPrimary,
		RepositoryID: ID("r-1"),
		Private:      true,
	}
	if s.EntityType() != TypeSource {
		t.Fatalf("EntityType() = %q", s.EntityType())
	}
	if _, has := reflect.TypeOf(s).FieldByName("Anchor"); has {
		t.Fatalf("Source не несёт Anchor (решение #21)")
	}
}

func TestCitationFullForm(t *testing.T) {
	c := Citation{
		ID:       ID("c-1"),
		SourceID: ID("s-1"),
		Anchor:   &ArchiveAnchor{NodeID: ID("n-1"), Page: 12, Rect: "10,10,20,20"},
		Text:     "Акилина Иванова, 53 лет",
	}
	if c.EntityType() != TypeCitation {
		t.Fatalf("EntityType() = %q", c.EntityType())
	}
	if _, ok := c.Anchor.(*ArchiveAnchor); !ok {
		t.Fatalf("anchor kind lost: %T", c.Anchor)
	}
}
```

- [ ] **Step 4: Проверка и commit**

Run: `gofmt -l internal/models` → пусто; `go build ./internal/models` → PASS.

```bash
git add internal/models/event.go internal/models/source.go internal/models/citation.go internal/models/event_participant.go internal/models/event_type.go internal/models/facts_test.go
git commit -m "feat(models): нормализованные Event (с участниками), Source без anchor/text, новая Citation"
```

### Task 8: Archive, ArchiveNode, ArchiveDocument, Attachment

**Files:**
- Modify: `internal/models/archive.go` (переписать)
- Create: `internal/models/archive_node.go`
- Create: `internal/models/archive_document.go`
- Create: `internal/models/attachment.go`
- Create: `internal/models/repository.go` (новая сущность, решение #25)
- Create: `internal/models/archives_test.go`
- Create: `internal/models/no_json_test.go`

**Interfaces:**
- Consumes: `ID`, `Type`, `ArchiveNodeType`, `FactDate`, `TextRef`, `SourceLink`, `AttachmentKind`.
- Produces:
  - `type ArchiveNodeType string` (в `archive_node.go`).
  - `Archive{ ID ID; Name string; System *TextRef; RepositoryID ID; Notes []TextRef; Sources []SourceLink; Private bool }`, `EntityType() Type = TypeArchive` (`RepositoryID` — решение #25, репозиторий-хранилище; `Private` — решение #24).
  - `ArchiveNode{ ID ID; Type ArchiveNodeType; ArchiveID ID; ParentID *ID; Label string; Name string; Since *FactDate; Until *FactDate; Parish *TextRef; Settlements []TextRef; Notes []TextRef; Sources []SourceLink; Private bool }`, `EntityType() Type = TypeArchiveNode`.
  - `ArchiveDocument{ ID ID; UnitID ID; Title string; Kind string; Since *FactDate; Until *FactDate; Parish *TextRef; Settlements []TextRef; Notes []TextRef; Sources []SourceLink; Private bool }`, `EntityType() Type = TypeArchiveDocument`.
  - `Attachment{ ID ID; Kind AttachmentKind; URI string; Filename string; MIME string; Page int; NodeID ID; DocumentID *ID; Note string; Private bool }`, `EntityType() Type = TypeAttachment`.
  - `Repository{ ID ID; Name string; Type RepositoryType; Address string; URLs []TextRef; Notes []TextRef; Sources []SourceLink; Private bool }`, `EntityType() Type = TypeRepository` (новая сущность, решение #25).

- [ ] **Step 1: Переписать Archive**

`internal/models/archive.go`:

```go
package models

// Archive — архив (хранилище с системой иерархии). RepositoryID — ссылка на
// репозиторий (решение #25); Private — приватность (решение #24).
type Archive struct {
	ID           ID
	Name         string
	System       *TextRef
	RepositoryID ID
	Notes        []TextRef
	Sources      []SourceLink
	Private      bool
}

// Type возвращает тип сущности.
func (a *Archive) EntityType() Type { return TypeArchive }
```

- [ ] **Step 2: ArchiveNode**

`internal/models/archive_node.go`:

```go
package models

// ArchiveNodeType — уровень системы иерархии архива (фонд/опись/дело/шкаф/…).
type ArchiveNodeType string

// ArchiveNode — рекурсивный узел цепочки хранения в архиве.
type ArchiveNode struct {
	ID          ID
	Type        ArchiveNodeType
	ArchiveID   ID
	ParentID    *ID
	Label       string
	Name        string
	Since       *FactDate
	Until       *FactDate
	Parish      *TextRef
	Settlements []TextRef
	Notes       []TextRef
	Sources     []SourceLink
	Private     bool
}

// Type возвращает тип сущности.
func (n *ArchiveNode) EntityType() Type { return TypeArchiveNode }
```

- [ ] **Step 3: ArchiveDocument**

`internal/models/archive_document.go`:

```go
package models

// ArchiveDocument — документ внутри единицы учёта (метрическая книга за годы…).
type ArchiveDocument struct {
	ID          ID
	UnitID      ID
	Title       string
	Kind        string
	Since       *FactDate
	Until       *FactDate
	Parish      *TextRef
	Settlements []TextRef
	Notes       []TextRef
	Sources     []SourceLink
	Private     bool
}

// Type возвращает тип сущности.
func (d *ArchiveDocument) EntityType() Type { return TypeArchiveDocument }
```

- [ ] **Step 4: Attachment**

`internal/models/attachment.go`:

```go
package models

// Attachment — файловое вложение (скан страницы, документ, аудио, фото).
type Attachment struct {
	ID         ID
	Kind       AttachmentKind
	URI        string
	Filename   string
	MIME       string
	Page       int
	NodeID     ID
	DocumentID *ID
	Note       string
	Private    bool
}

// Type возвращает тип сущности.
func (a *Attachment) EntityType() Type { return TypeAttachment }
```

- [ ] **Step 5: Repository (новая сущность, решение #25)**

`internal/models/repository.go`:

```go
package models

// Repository — хранилище-контейнер источников (архив, библиотека, музей,
// частное собрание и т.п.). Source.repository_id / Archive.repository_id —
// мягкие ссылки на него.
type Repository struct {
	ID      ID
	Name    string
	Type    RepositoryType
	Address string
	URLs    []TextRef
	Notes   []TextRef
	Sources []SourceLink
	Private bool
}

// Type возвращает тип сущности.
func (r *Repository) EntityType() Type { return TypeRepository }
```

- [ ] **Step 6: Тест**

`internal/models/archives_test.go`:

```go
package models

import "testing"

func TestArchiveNodeFullForm(t *testing.T) {
	n := ArchiveNode{
		ID:        ID("n-1"),
		Type:      ArchiveNodeType("case"),
		ArchiveID: ID("a-1"),
		Label:     "дело №264",
		Parish:    &TextRef{Text: "Николаевская ц."},
	}
	if n.EntityType() != TypeArchiveNode {
		t.Fatalf("EntityType() = %q", n.EntityType())
	}
	if n.ArchiveID != ID("a-1") {
		t.Fatalf("strict link lost")
	}
}

func TestRepositoryFullForm(t *testing.T) {
	r := Repository{
		ID:      ID("r-1"),
		Name:    "ГАВО",
		Type:    RepositoryTypeArchive,
		Address: "г. Владимир",
		URLs:    []TextRef{{Text: "https://example.gov.ru/funds"}},
		Private: true,
	}
	if r.EntityType() != TypeRepository {
		t.Fatalf("EntityType() = %q", r.EntityType())
	}
	if len(r.URLs) != 1 || r.Private != true {
		t.Fatalf("urls/private не прочитаны")
	}
}
```

- [ ] **Step 6: Регрессионный тест «домен без JSON»**

`internal/models/no_json_test.go`:

```go
package models

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TestNoJSONTags фиксирует границу пакетов: домен о JSON не знает,
// форма на проводе определяется в internal/transport.
func TestNoJSONTags(t *testing.T) {
	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") {
			continue
		}
		f, err := parser.ParseFile(fset, e.Name(), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if field, ok := n.(*ast.Field); ok && field.Tag != nil && strings.Contains(field.Tag.Value, "json:") {
				t.Errorf("%s: JSON-тег в домене: %s", fset.Position(field.Pos()), field.Tag.Value)
			}
			return true
		})
	}
}
```

- [ ] **Step 7: Полная сборка и тесты models**

Run: `gofmt -l internal/models` → пусто.
Run: `go build ./... && go test ./internal/models`
Expected: PASS, включая `TestNoJSONTags` (если он падает, где-то остался JSON-тег — сними его, исключений нет).

Дерево зелёное: `internal/entity` пока существует и используется старыми потребителями (удаляется в Task 13).

- [ ] **Step 8: Commit**

```bash
git add internal/models
git commit -m "feat(models): полный набор сущностей по docs/models без JSON-тегов; TestNoJSONTags"
```

---

## PART B — `internal/transport`

### Task 9: DTO публичных контрактов и конвертеры

**Files:**
- Create: `internal/transport/settlement.go`
- Create: `internal/transport/settlement_test.go`

**Interfaces:**
- Consumes: `models.AdministrativeDivision` (`ID`, `Name`, `Type AdminDivisionType`), `models.ID`, `models.AdminDivisionType` (Task 2, Task 6).
- Produces:
  - `transport.Settlement{ ID models.ID; Name string; Type models.AdminDivisionType }` с JSON-тегами `id`, `name`, `type`.
  - `transport.SettlementFromModel(d models.AdministrativeDivision) transport.Settlement`.
  - `transport.SettlementsFromModels(ds []models.AdministrativeDivision) []transport.Settlement` — пустой/nil-вход даёт пустой срез (в JSON — `[]`, не `null`).
- Пакет импортируют только `internal/httpapi` и `internal/mcp` (Task 13). `ToModel`-конвертеров нет: путей записи пока не существует.

- [ ] **Step 1: Написать падающие тесты**

`internal/transport/settlement_test.go`:

```go
package transport

import (
	"encoding/json"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

func TestSettlementFromModel(t *testing.T) {
	got := SettlementFromModel(models.AdministrativeDivision{
		ID:       "ad-1",
		Name:     "Давыдово",
		Type:     models.AdminDivisionSelo,
		Variants: []string{"Давыдова"},
	})
	want := Settlement{ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo}
	if got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// Контракт на проводе зафиксирован строкой: пустой список — `[]`, поля — id/name/type.
func TestSettlementsJSONContract(t *testing.T) {
	empty, err := json.Marshal(SettlementsFromModels(nil))
	if err != nil {
		t.Fatal(err)
	}
	if string(empty) != `[]` {
		t.Fatalf("empty = %s, want []", empty)
	}

	list, err := json.Marshal(SettlementsFromModels([]models.AdministrativeDivision{
		{ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo},
		{ID: "ad-2", Name: "Никифорово", Type: models.AdminDivisionDerevnya},
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := `[{"id":"ad-1","name":"Давыдово","type":"selo"},{"id":"ad-2","name":"Никифорово","type":"derevnya"}]`
	if string(list) != want {
		t.Fatalf("list = %s, want %s", list, want)
	}
}
```

- [ ] **Step 2: Убедиться, что тесты не компилируются**

Run: `go test ./internal/transport`
Expected: FAIL — `undefined: SettlementFromModel`, `undefined: Settlement`.

- [ ] **Step 3: Реализация**

`internal/transport/settlement.go`:

```go
// Package transport — DTO публичных контрактов (/api и MCP-тулы) и конвертеры
// из домена. Единственное место, где определена форма JSON на проводе; домен
// (internal/models) о JSON не знает.
package transport

import "github.com/amarin/genodex/internal/models"

// Settlement — контракт списка населённых пунктов (GET /api/settlements,
// MCP-тул settlement_list).
type Settlement struct {
	ID   models.ID                `json:"id"`
	Name string                   `json:"name"`
	Type models.AdminDivisionType `json:"type"`
}

// SettlementFromModel конвертирует единицу деления в контракт.
func SettlementFromModel(d models.AdministrativeDivision) Settlement {
	return Settlement{ID: d.ID, Name: d.Name, Type: d.Type}
}

// SettlementsFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func SettlementsFromModels(ds []models.AdministrativeDivision) []Settlement {
	out := make([]Settlement, 0, len(ds))
	for _, d := range ds {
		out = append(out, SettlementFromModel(d))
	}
	return out
}
```

- [ ] **Step 4: Прогнать тесты**

Run: `gofmt -l internal/transport` → пусто.
Run: `go test ./internal/transport && go build ./...`
Expected: PASS; сборка зелёная (`transport` пока никем не импортируется).

- [ ] **Step 5: Commit**

```bash
git add internal/transport
git commit -m "feat(transport): DTO Settlement и конвертеры models → DTO"
```

---

## PART C — Хранилище и порт (на `models`)

### Task 10: Схема БД в `internal/storage` (schema_version=0, колоночные таблицы, search_index)

**Files:**
- Modify: `internal/storage/db.go` (переписать полностью)
- Modify: `internal/storage/db_test.go` (переписать — новый API)
- Modify: `internal/storage/storage.go` (новые методы доступа)
- Modify: `internal/storage/normalize.go` (остаётся; нормализация поисковых терминов)
- Modify: `internal/storage/backup.go` (список типов для counts)
- Modify: `internal/storage/verify.go` (Count по именам таблиц)
- Modify: `internal/storage/restore.go` (Count по именам таблиц)

**Interfaces:**
- Consumes: `models.Type` (для валидации имени таблицы), `Normalize` из `internal/storage`.
- Produces — новый публичный API `storage.DB`:
  - `OpenDB(path) (*DB, error)` — создаёт схему целиком;
  - `Exec(query string, args ...any) (sql.Result, error)`;
  - `Query(query string, args ...any) (*sql.Rows, error)`;
  - `QueryRow(query string, args ...any) *sql.Row`;
  - `Tx(fn func(tx *sql.Tx) error) error` — транзакция с откатом при ошибке;
  - `Count(table string) (int, error)` — считает строки таблицы (валидация имени по реестру);
  - `IntegrityCheck() error`, `VacuumInto(path) error`, `Close() error`.
  - `Storage.DB() *DB`, `Storage.Search...` удаляются (поиск уходит в sqlstore через `Query`).

- [ ] **Step 1: DDL-реестр таблиц**

В `internal/storage/db.go` определи константу версии и полный DDL. Таблицы (см. spec §5):

- `meta(key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
- `dates(id INTEGER PRIMARY KEY AUTOINCREMENT, year INTEGER NOT NULL DEFAULT 0, month INTEGER NOT NULL DEFAULT 0, day INTEGER NOT NULL DEFAULT 0, precision TEXT NOT NULL, modifier TEXT NOT NULL, year_to INTEGER NOT NULL DEFAULT 0, month_to INTEGER NOT NULL DEFAULT 0, day_to INTEGER NOT NULL DEFAULT 0, calendar TEXT NOT NULL DEFAULT 'gregorian')`,
- `text_refs(id INTEGER PRIMARY KEY AUTOINCREMENT, text TEXT NOT NULL, ref TEXT NOT NULL DEFAULT '', ref_type TEXT NOT NULL DEFAULT '')`,
- `anchors(id INTEGER PRIMARY KEY AUTOINCREMENT, kind TEXT NOT NULL, node_id TEXT NOT NULL DEFAULT '', document_id TEXT NOT NULL DEFAULT '', page INTEGER NOT NULL DEFAULT 0, rect TEXT NOT NULL DEFAULT '', attachment_id TEXT NOT NULL DEFAULT '', timecode TEXT NOT NULL DEFAULT '', url TEXT NOT NULL DEFAULT '')`,
- `source_links(id INTEGER PRIMARY KEY AUTOINCREMENT, citation_id TEXT NOT NULL REFERENCES citations(id) ON DELETE RESTRICT, target_type TEXT NOT NULL, target_id TEXT NOT NULL, reliability TEXT NOT NULL DEFAULT '', role TEXT NOT NULL DEFAULT '', note TEXT NOT NULL DEFAULT '')`,
- `search_index(entity_table TEXT NOT NULL, entity_id TEXT NOT NULL, field TEXT NOT NULL, term TEXT NOT NULL COLLATE BINARY, PRIMARY KEY (entity_table, entity_id, field, term))`,
- `persons(id TEXT PRIMARY KEY, gender TEXT NOT NULL DEFAULT '', private INTEGER NOT NULL DEFAULT 0)`,
- `person_names(id INTEGER PRIMARY KEY AUTOINCREMENT, person_id TEXT NOT NULL REFERENCES persons(id) ON DELETE CASCADE, type TEXT NOT NULL DEFAULT '', surname_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT, given_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT, patronymic_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT, prefix TEXT NOT NULL DEFAULT '', suffix TEXT NOT NULL DEFAULT '', since_id INTEGER REFERENCES dates(id) ON DELETE SET NULL, until_id INTEGER REFERENCES dates(id) ON DELETE SET NULL)`,
- `person_estates(id INTEGER PRIMARY KEY AUTOINCREMENT, person_id TEXT NOT NULL REFERENCES persons(id) ON DELETE CASCADE, position INTEGER NOT NULL, text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT)`,
- `person_titles(аналогично person_estates)`,
- `person_nicknames(аналогично person_estates)`,
- `person_notes(аналогично person_estates)`,
- `relations(id TEXT PRIMARY KEY, kind TEXT NOT NULL, rel_type TEXT NOT NULL DEFAULT '', person_a TEXT NOT NULL REFERENCES persons(id) ON DELETE RESTRICT, person_b TEXT NOT NULL REFERENCES persons(id) ON DELETE RESTRICT, since_id INTEGER REFERENCES dates(id) ON DELETE SET NULL, until_id INTEGER REFERENCES dates(id) ON DELETE SET NULL, private INTEGER NOT NULL DEFAULT 0)`,
- `relation_notes(id INTEGER PRIMARY KEY AUTOINCREMENT, relation_id TEXT NOT NULL REFERENCES relations(id) ON DELETE CASCADE, position INTEGER NOT NULL, text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT)`,
- `families(id TEXT PRIMARY KEY, name TEXT NOT NULL DEFAULT '', private INTEGER NOT NULL DEFAULT 0)`,
- `family_members(id INTEGER PRIMARY KEY AUTOINCREMENT, family_id TEXT NOT NULL REFERENCES families(id) ON DELETE CASCADE, position INTEGER NOT NULL, text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT)`,
- `family_notes(аналогично family_members)`,
- `surnames(id TEXT PRIMARY KEY, canonical TEXT NOT NULL)`,
- `surname_variants(id INTEGER PRIMARY KEY AUTOINCREMENT, surname_id TEXT NOT NULL REFERENCES surnames(id) ON DELETE CASCADE, position INTEGER NOT NULL, text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT)`,
- `surname_items(аналогично surname_variants)`, `surname_notes(аналогично)`,
- `given_names(id TEXT PRIMARY KEY, canonical TEXT NOT NULL, gender TEXT NOT NULL)`,
- `given_name_variants/_items/_notes(аналогично surname_*)`,
- `patronymics(id TEXT PRIMARY KEY, canonical TEXT NOT NULL)` + `patronymic_variants/_items/_notes`,
- `estates/id_title` + `estate_*`, `title_*` по тому же шаблону,
- `administrative_divisions(id TEXT PRIMARY KEY, name TEXT NOT NULL, type TEXT NOT NULL, parent_id TEXT REFERENCES administrative_divisions(id) ON DELETE RESTRICT, since_id INTEGER REFERENCES dates(id) ON DELETE SET NULL, until_id INTEGER REFERENCES dates(id) ON DELETE SET NULL)`,
- `ad_items(id INTEGER PRIMARY KEY AUTOINCREMENT, ad_id TEXT NOT NULL REFERENCES administrative_divisions(id) ON DELETE CASCADE, position INTEGER NOT NULL, text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT)`,
- `ad_variants(id INTEGER PRIMARY KEY AUTOINCREMENT, ad_id TEXT NOT NULL REFERENCES administrative_divisions(id) ON DELETE CASCADE, position INTEGER NOT NULL, value TEXT NOT NULL)`,
- `ad_renames(id INTEGER PRIMARY KEY AUTOINCREMENT, ad_id TEXT NOT NULL REFERENCES administrative_divisions(id) ON DELETE CASCADE, position INTEGER NOT NULL, text TEXT NOT NULL, since TEXT NOT NULL DEFAULT '', until TEXT NOT NULL DEFAULT '')`,
- `ad_successors(аналогично ad_items)`, `ad_notes(аналогично)`,
- `churches(id TEXT PRIMARY KEY, name TEXT NOT NULL, parish_id INTEGER REFERENCES text_refs(id) ON DELETE SET NULL)`,
- `church_settlements(id INTEGER PRIMARY KEY AUTOINCREMENT, church_id TEXT NOT NULL REFERENCES churches(id) ON DELETE CASCADE, position INTEGER NOT NULL, text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT)`,
- `church_variants(как ad_variants, owner church_id→churches)`, `church_notes(как ad_items)`,
- `parishes(id TEXT PRIMARY KEY, name TEXT NOT NULL, church_id INTEGER REFERENCES text_refs(id) ON DELETE SET NULL, since_id INTEGER REFERENCES dates(id) ON DELETE SET NULL, until_id INTEGER REFERENCES dates(id) ON DELETE SET NULL)`,
- `parish_settlements(как church_settlements, owner parishes)`, `parish_notes`,
- `events(id TEXT PRIMARY KEY, type TEXT NOT NULL, date_id INTEGER REFERENCES dates(id) ON DELETE SET NULL, place_id INTEGER REFERENCES text_refs(id) ON DELETE SET NULL, private INTEGER NOT NULL DEFAULT 0)`,
- `event_participants(id INTEGER PRIMARY KEY AUTOINCREMENT, event_id TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE, position INTEGER NOT NULL, person_id TEXT NOT NULL REFERENCES persons(id) ON DELETE RESTRICT, role TEXT NOT NULL DEFAULT '', note TEXT NOT NULL DEFAULT '')`,
- `event_notes(как ad_items, owner events)`,
- `residences(id TEXT PRIMARY KEY, person_id TEXT NOT NULL REFERENCES persons(id) ON DELETE RESTRICT, place_id TEXT NOT NULL REFERENCES administrative_divisions(id) ON DELETE RESTRICT, since_id INTEGER REFERENCES dates(id) ON DELETE SET NULL, until_id INTEGER REFERENCES dates(id) ON DELETE SET NULL, note TEXT NOT NULL DEFAULT '', private INTEGER NOT NULL DEFAULT 0)`,
- `sources(id TEXT PRIMARY KEY, kind TEXT NOT NULL, title TEXT NOT NULL, author TEXT NOT NULL DEFAULT '', date_id INTEGER REFERENCES dates(id) ON DELETE SET NULL, reliability TEXT NOT NULL DEFAULT '', repository_id TEXT REFERENCES repositories(id) ON DELETE RESTRICT, private INTEGER NOT NULL DEFAULT 0)`,
- `source_notes(как ad_items, owner sources)`,
- `citations(id TEXT PRIMARY KEY, source_id TEXT NOT NULL REFERENCES sources(id) ON DELETE RESTRICT, anchor_id INTEGER REFERENCES anchors(id) ON DELETE SET NULL, text TEXT NOT NULL DEFAULT '', note TEXT NOT NULL DEFAULT '', private INTEGER NOT NULL DEFAULT 0)`,
- `notes(id TEXT PRIMARY KEY, kind TEXT NOT NULL DEFAULT '', title TEXT NOT NULL DEFAULT '', text TEXT NOT NULL DEFAULT '', parent_id TEXT REFERENCES notes(id) ON DELETE RESTRICT, private INTEGER NOT NULL DEFAULT 0)`,
- `repositories(id TEXT PRIMARY KEY, name TEXT NOT NULL, type TEXT NOT NULL DEFAULT '', address TEXT NOT NULL DEFAULT '', private INTEGER NOT NULL DEFAULT 0)`,
- `repository_urls(id INTEGER PRIMARY KEY AUTOINCREMENT, repository_id TEXT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE, position INTEGER NOT NULL, text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT)`,
- `repository_notes(как repository_urls)`,
- `archives(id TEXT PRIMARY KEY, name TEXT NOT NULL, system_id INTEGER REFERENCES text_refs(id) ON DELETE SET NULL, repository_id TEXT REFERENCES repositories(id) ON DELETE RESTRICT, private INTEGER NOT NULL DEFAULT 0)`,
- `archive_notes(как ad_items, owner archives)`,
- `archive_nodes(id TEXT PRIMARY KEY, type TEXT NOT NULL DEFAULT '', archive_id TEXT NOT NULL REFERENCES archives(id) ON DELETE RESTRICT, parent_id TEXT REFERENCES archive_nodes(id) ON DELETE RESTRICT, label TEXT NOT NULL DEFAULT '', name TEXT NOT NULL DEFAULT '', since_id INTEGER REFERENCES dates(id) ON DELETE SET NULL, until_id INTEGER REFERENCES dates(id) ON DELETE SET NULL, parish_id INTEGER REFERENCES text_refs(id) ON DELETE SET NULL, private INTEGER NOT NULL DEFAULT 0)`,
- `node_settlements(как church_settlements, owner archive_nodes)`, `node_notes`,
- `archive_documents(id TEXT PRIMARY KEY, unit_id TEXT NOT NULL REFERENCES archive_nodes(id) ON DELETE RESTRICT, title TEXT NOT NULL, kind TEXT NOT NULL DEFAULT '', since_id INTEGER REFERENCES dates(id) ON DELETE SET NULL, until_id INTEGER REFERENCES dates(id) ON DELETE SET NULL, parish_id INTEGER REFERENCES text_refs(id) ON DELETE SET NULL, private INTEGER NOT NULL DEFAULT 0)`,
- `doc_settlements(как church_settlements, owner archive_documents)`, `doc_notes`,
- `attachments(id TEXT PRIMARY KEY, kind TEXT NOT NULL, uri TEXT NOT NULL DEFAULT '', filename TEXT NOT NULL DEFAULT '', mime TEXT NOT NULL DEFAULT '', page INTEGER NOT NULL DEFAULT 0, node_id TEXT NOT NULL REFERENCES archive_nodes(id) ON DELETE RESTRICT, document_id TEXT REFERENCES archive_documents(id) ON DELETE SET NULL, note TEXT NOT NULL DEFAULT '', private INTEGER NOT NULL DEFAULT 0)`.

Индексы для FK-колонок, по которым идут delete restrictions, не обязательны (маленькие данные), но для `source_links(target_type, target_id)` создай индекс — по нему идёт очистка при удалении сущности: `CREATE INDEX idx_source_links_target ON source_links(target_type, target_id)`.

Реестр таблиц для `Count`:

```go
var tables = []string{
	"persons", "relations", "residences", "families",
	"surnames", "given_names", "patronymics", "estates", "titles",
	"churches", "parishes", "administrative_divisions",
	"events", "sources", "citations", "notes", "repositories",
	"archives", "archive_nodes", "archive_documents",
	"attachments",
}

var tableNames = map[string]bool{}
func init() {
	for _, t := range tables {
		tableNames[t] = true
	}
}
```

Далее весь DDL выполняется одним заходом в `OpenDB`, `PRAGMA foreign_keys=ON` сохраняется, `schema_version` записывается как `0`:

```go
const schemaVersion = 0
```

- [ ] **Step 2: Новый API DB**

Перепиши `internal/storage/db.go`:

```go
package storage

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const schemaVersion = 0

// DB оборачивает одно SQLite-соединение. Схема — колоночная (сущность =
// таблица, коллекция = связная таблица), см. docs/data-model/normalization-s1s2.md.
type DB struct {
	d *sql.DB
}

func OpenDB(path string) (*DB, error) {
	d, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
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
	ddl := append([]string{`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`}, schemaDDL...)
	for _, q := range ddl {
		if _, err := d.Exec(q); err != nil {
			d.Close()
			return nil, fmt.Errorf("schema: %w", err)
		}
	}
	if _, err := d.Exec(`INSERT OR IGNORE INTO meta(key, value) VALUES ('schema_version', ?)`, schemaVersion); err != nil {
		d.Close()
		return nil, err
	}
	return &DB{d: d}, nil
}

func (d *DB) Exec(query string, args ...any) (sql.Result, error) { return d.d.Exec(query, args...) }

func (d *DB) Query(query string, args ...any) (*sql.Rows, error) { return d.d.Query(query, args...) }

func (d *DB) QueryRow(query string, args ...any) *sql.Row { return d.d.QueryRow(query, args...) }

// Tx выполняет fn в транзакции; при ошибке — откат.
func (d *DB) Tx(fn func(tx *sql.Tx) error) error {
	tx, err := d.d.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// Count считает строки таблицы из реестра (валидация имени защищает от
// SQL-инъекции; резерв — внешний список типов для манифеста).
func (d *DB) Count(table string) (int, error) {
	if !tableNames[table] {
		return 0, fmt.Errorf("unknown table %q", table)
	}
	var n int
	err := d.d.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n)
	return n, err
}

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

func (d *DB) VacuumInto(path string) error {
	esc := strings.ReplaceAll(path, "'", "''")
	_, err := d.d.Exec(fmt.Sprintf("VACUUM INTO '%s'", esc))
	return err
}

func (d *DB) Close() error { return d.d.Close() }
```

`schemaDDL` — слайс строк DDL из Step 1 (все `CREATE TABLE IF NOT EXISTS`/`CREATE INDEX IF NOT EXISTS`). Расположен в этом же файле или в отдельном `schema.go` — по предпочтению разработчика (DDL объёмный, допускается вынести в `internal/storage/schema.go`).

- [ ] **Step 3: Storage — доступ к DB, удаление старого API**

`internal/storage/storage.go`:

```go
package storage

import (
	"os"
	"path/filepath"
)

// Storage — точка входа: SQLite-хранилище с WAL. Доступ к данным — через
// Store.DB() (Exec/Query/QueryRow/Tx), которую использует sqlstore.
type Storage struct {
	dir string
	db  *DB
}

func Open(dataDir string) (*Storage, error) {
	dbDir := filepath.Join(dataDir, "db")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		return nil, err
	}
	db, err := OpenDB(filepath.Join(dbDir, "genodex.db"))
	if err != nil {
		return nil, err
	}
	return &Storage{dir: dataDir, db: db}, nil
}

// DB возвращает доступ к соединению.
func (s *Storage) DB() *DB { return s.db }

func (s *Storage) Close() error { return s.db.Close() }

// Dir возвращает каталог данных (нужно backup/restore).
func (s *Storage) Dir() string { return s.dir }
```

Удаляются методы `Save`, `Delete`, `Get`, `List`, `Search`, `Count` из `Storage` и все `entityRow`, `Upsert`, `Delete`, `Get`, `List`, `Search` старого `DB`.

- [ ] **Step 4: Обновить backup/verify/restore на новые имена таблиц**

В `internal/storage/backup.go` замени список типов в `Backup`:

```go
for _, t := range tables {
	n, err := s.db.Count(t)
	if err != nil {
		return Manifest{}, err
	}
	if n > 0 {
		counts[t] = n
	}
}
```

В `internal/storage/verify.go` — `s.db.Count(et)` уже работает (тип `et` = имя таблицы в реестре); ничего менять не нужно, кроме комментария.

В `internal/storage/restore.go` — `s.db.Count(et)` работает; без изменений.

- [ ] **Step 5: Переписать тесты БД**

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

func TestSchemaCreated(t *testing.T) {
	db := openTestDB(t)
	for _, table := range tables {
		var n int
		if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&n); err != nil {
			t.Fatalf("table %s: %v", table, err)
		}
	}
	var v string
	if err := db.QueryRow("SELECT value FROM meta WHERE key='schema_version'").Scan(&v); err != nil {
		t.Fatalf("schema_version: %v", err)
	}
	if v != "0" {
		t.Fatalf("schema_version = %q, want 0", v)
	}
}

func TestTxRollback(t *testing.T) {
	db := openTestDB(t)
	err := db.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec("INSERT INTO persons(id) VALUES ('p-1')"); err != nil {
			return err
		}
		return fmt.Errorf("boom")
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if n, _ := db.Count("persons"); n != 0 {
		t.Fatalf("Count persons=%d после отката, want 0", n)
	}
}

func TestFKRestrict(t *testing.T) {
	db := openTestDB(t)
	// строгая ссылка не даёт удалить персону, на которую ссылается Relation
	if _, err := db.Exec("INSERT INTO persons(id) VALUES ('p-1'), ('p-2')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO relations(id, kind, person_a, person_b) VALUES ('r-1','blood','p-1','p-2')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DELETE FROM persons WHERE id='p-1'"); err == nil {
		t.Fatal("FK RESTRICT не сработал")
	}
}

func TestFKCascade(t *testing.T) {
	db := openTestDB(t)
	if _, err := db.Exec("INSERT INTO persons(id) VALUES ('p-1')"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO person_names(person_id, surname_id, given_id, patronymic_id) VALUES ('p-1', 1, 1, 1)"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DELETE FROM persons WHERE id='p-1'"); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM person_names").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("person_names count=%d после каскада, want 0", n)
	}
}

func TestDBIntegrityAndVacuumInto(t *testing.T) {
	db := openTestDB(t)
	if err := db.IntegrityCheck(); err != nil {
		t.Fatalf("IntegrityCheck: %v", err)
	}
	out := filepath.Join(t.TempDir(), "snapshot.db")
	if err := db.VacuumInto(out); err != nil {
		t.Fatalf("VacuumInto: %v", err)
	}
}
```

Добавь импорты `database/sql` и `fmt`; удали старые тесты `TestDBUpsertGetListDelete` и `TestDBSearchNormalized`.

Тесты `internal/storage/backup_test.go`, `verify_test.go`, `restore_test.go`, `normalize_test.go` продолжают работать (они используют `Open`, `Backup`, `Verify`, `Restore`, `Normalize`) — обнови в них только те места, где упоминается `entity_counts` по именам `person`/`settlement`: теперь имена — `persons`/`administrative_divisions` (см. Task 12).

- [ ] **Step 6: Проверка и commit**

Run: `gofmt -l internal/storage` → пусто; `go test ./internal/storage` → PASS (возможны правки backup_test/verify_test/restore_test — выполни их здесь).

⚠️ **Дерево красное с этого шага и до Task 13:** `internal/store/sqlstore` ещё использует старый API (`saveJSON`/`getJSON`). Задачи 10–13 идут подряд, коммит Task 10 допустимо сделать до возврата зелёной сборки.

```bash
git add internal/storage
git commit -m "feat(storage): колоночная схема schema_version=0, search_index, новый API Exec/Query/Tx
старый entity-блоб удалён; backup/verify/restore считают по таблицам"
```

### Task 11: Порт `internal/store` — расширение на все сущности

**Files:**
- Modify: `internal/store/deps.go` (переписать)
- Test: `internal/store/deps_test.go` (перегенерировать mockgen; удалить старый)

**Interfaces:**
- Consumes: `models.*` из PART A.
- Produces — интерфейс `Store`:
  - `GetPerson(id models.ID) (*models.Person, error)`, `SavePerson(p *models.Person) error`, `ListPeople() ([]*models.Person, error)`;
  - группа `Get/Save/List` для: `Relation`, `Residence`, `Family`, `Surname`, `GivenName`, `Patronymic`, `Estate`, `Title`, `AdministrativeDivision`, `Church`, `Parish`, `Event`, `Source`, `Archive`, `ArchiveNode`, `ArchiveDocument`, `Attachment` (именование: `GetX/SaveX/ListXs`; для словарей — `ListSurnames` и т.д.);
  - **удалены** методы `GetSettlement/SaveSettlement/ListSettlements`.

- [ ] **Step 1: Переписать интерфейс**

`internal/store/deps.go`:

```go
package store

import "github.com/amarin/genodex/internal/models"

// Store — порт хранилища сущностей генеалогии.
// Реализация: internal/store/sqlstore (адаптер поверх internal/storage).
// MCP/API никогда не работают с портом напрямую — только через usecases.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type Store interface {
	// Person
	GetPerson(id models.ID) (*models.Person, error)
	SavePerson(p *models.Person) error
	ListPeople() ([]*models.Person, error)

	// Relation
	GetRelation(id models.ID) (*models.Relation, error)
	SaveRelation(r *models.Relation) error
	ListRelations() ([]*models.Relation, error)

	// Residence
	GetResidence(id models.ID) (*models.Residence, error)
	SaveResidence(r *models.Residence) error
	ListResidences() ([]*models.Residence, error)

	// Family
	GetFamily(id models.ID) (*models.Family, error)
	SaveFamily(f *models.Family) error
	ListFamilies() ([]*models.Family, error)

	// Дictionaries
	GetSurname(id models.ID) (*models.Surname, error)
	SaveSurname(s *models.Surname) error
	ListSurnames() ([]*models.Surname, error)

	GetGivenName(id models.ID) (*models.GivenName, error)
	SaveGivenName(g *models.GivenName) error
	ListGivenNames() ([]*models.GivenName, error)

	GetPatronymic(id models.ID) (*models.Patronymic, error)
	SavePatronymic(p *models.Patronymic) error
	ListPatronymics() ([]*models.Patronymic, error)

	GetEstate(id models.ID) (*models.Estate, error)
	SaveEstate(e *models.Estate) error
	ListEstates() ([]*models.Estate, error)

	GetTitle(id models.ID) (*models.Title, error)
	SaveTitle(t *models.Title) error
	ListTitles() ([]*models.Title, error)

	// AdministrativeDivision
	GetAdministrativeDivision(id models.ID) (*models.AdministrativeDivision, error)
	SaveAdministrativeDivision(a *models.AdministrativeDivision) error
	ListAdministrativeDivisions() ([]*models.AdministrativeDivision, error)

	// Church
	GetChurch(id models.ID) (*models.Church, error)
	SaveChurch(c *models.Church) error
	ListChurches() ([]*models.Church, error)

	// Parish
	GetParish(id models.ID) (*models.Parish, error)
	SaveParish(p *models.Parish) error
	ListParishes() ([]*models.Parish, error)

	// Event
	GetEvent(id models.ID) (*models.Event, error)
	SaveEvent(e *models.Event) error
	ListEvents() ([]*models.Event, error)

	// Source
	GetSource(id models.ID) (*models.Source, error)
	SaveSource(s *models.Source) error
	ListSources() ([]*models.Source, error)

	// Archive
	GetArchive(id models.ID) (*models.Archive, error)
	SaveArchive(a *models.Archive) error
	ListArchives() ([]*models.Archive, error)

	// ArchiveNode
	GetArchiveNode(id models.ID) (*models.ArchiveNode, error)
	SaveArchiveNode(n *models.ArchiveNode) error
	ListArchiveNodes() ([]*models.ArchiveNode, error)

	// ArchiveDocument
	GetArchiveDocument(id models.ID) (*models.ArchiveDocument, error)
	SaveArchiveDocument(d *models.ArchiveDocument) error
	ListArchiveDocuments() ([]*models.ArchiveDocument, error)

	// Attachment
	GetAttachment(id models.ID) (*models.Attachment, error)
	SaveAttachment(a *models.Attachment) error
	ListAttachments() ([]*models.Attachment, error)
}
```

- [ ] **Step 2: Перегенерировать мок**

Run: `go generate ./internal/store/...`
Expected: обновлён `internal/store/deps_test.go`.

- [ ] **Step 3: Проверка и commit**

Run: `gofmt -l internal/store` → пусто; `go build ./internal/store` → только интерфейс, PASS.

```bash
git add internal/store/deps.go internal/store/deps_test.go
git commit -m "feat(store): порт расширен на все сущности; settlement-методы удалены"
```

### Task 12: `sqlstore` — маппинг сущностей на наборы строк

**Files:**
- Modify: `internal/store/sqlstore/sqlstore.go` (переписать: общие низовые функции)
- Modify: `internal/store/sqlstore/records.go` (переписать)
- Modify: `internal/store/sqlstore/person.go`, `place.go`, `settlement.go`, `archive.go` (переписать/удалить)
- Create: `internal/store/sqlstore/helpers.go` (dates / text_refs / anchors / source_links / search_index / generic Save/get)
- Modify: `internal/store/sqlstore/sqlstore_test.go` (переписать под новый API)

**Interfaces:**
- Consumes: `storage.DB` (Task 10), `models.*`, порт `store.Store`.
- Produces: реализация всего интерфейса `store.Store`.

- [ ] **Step 1: Удалить старый маппер**

Удали `internal/store/sqlstore/{settlement,place,archive,records,person}.go` и старые `saveJSON/getJSON/listJSON` из `sqlstore.go`.

- [ ] **Step 2: Низовые функции в helpers.go**

```go
package sqlstore

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/storage"
)

// Store — реализация порта store.Store поверх internal/storage: каждая
// сущность — набор строк в колоночной схеме (см. docs/data-model/normalization-s1s2.md).
type Store struct {
	st *storage.Storage
}

var _ store.Store = (*Store)(nil)

func Open(dataDir string) (*Store, error) {
	st, err := storage.Open(dataDir)
	if err != nil {
		return nil, fmt.Errorf("open storage: %w", err)
	}
	return &Store{st: st}, nil
}

func (s *Store) Close() error { return s.st.Close() }
```

Далее в том же файле (или отдельными файлами, по предпочтению — файлы ниже):

```go
const textRefDupesAllowed = true

// insertTextRef пишет TextRef в text_refs и возвращает id.
func insertTextRef(tx *sql.Tx, tr *models.TextRef) (int64, error) {
	if tr == nil {
		return 0, nil
	}
	if tr.Text == "" && tr.Ref == "" {
		return 0, nil
	}
	res, err := tx.Exec(`INSERT INTO text_refs(text, ref, ref_type) VALUES (?, ?, ?)`,
		tr.Text, string(tr.Ref), string(tr.Type))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
```

- [ ] **Step 3: Общие функции маппинга (helpers.go продолжение)**

```go
func insertDate(tx *sql.Tx, d *models.FactDate) (int64, error) {
	if d == nil {
		return 0, nil
	}
	res, err := tx.Exec(`INSERT INTO dates(year, month, day, precision, modifier, year_to, month_to, day_to)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		d.Year, d.Month, d.Day, string(d.Precision), string(d.Modifier), d.YearTo, d.MonthTo, d.DayTo)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func insertAnchor(tx *sql.Tx, a models.Anchor) (int64, error) {
	if a == nil {
		return 0, nil
	}
	var kind, nodeID, docID, rect, attID, timecode, url string
	var page int
	switch v := a.(type) {
	case *models.ArchiveAnchor:
		kind, nodeID, docID, page, rect = string(models.AnchorArchive), string(v.NodeID), string(v.DocumentID), v.Page, v.Rect
	case *models.FileAnchor:
		kind, attID, timecode = string(models.AnchorFile), string(v.AttachmentID), v.Timecode
	case *models.URLAnchor:
		kind, url = string(models.AnchorURL), v.URL
	default:
		return 0, fmt.Errorf("anchor: неизвестная реализация %T", a)
	}
	res, err := tx.Exec(`INSERT INTO anchors(kind, node_id, document_id, page, rect, attachment_id, timecode, url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, kind, nodeID, docID, page, rect, attID, timecode, url)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func insertSourceLink(tx *sql.Tx, l models.SourceLink) error {
	_, err := tx.Exec(`INSERT INTO source_links(citation_id, target_type, target_id, reliability, role, note)
		VALUES (?, ?, ?, ?, ?, ?)`,
		string(l.CitationID), string(l.TargetType), string(l.TargetID),
		string(l.Reliability), l.Role, l.Note)
	return err
}

// replaceSourceLinks переписывает связку источников сущности.
func replaceSourceLinks(tx *sql.Tx, t models.Type, id models.ID, links []models.SourceLink) error {
	if _, err := tx.Exec(`DELETE FROM source_links WHERE target_type = ? AND target_id = ?`, string(t), string(id)); err != nil {
		return err
	}
	for _, l := range links {
		if err := insertSourceLink(tx, l); err != nil {
			return err
		}
	}
	return nil
}

// replaceTextRefList переписывает список TextRef в связной таблице.
func replaceTextRefList(tx *sql.Tx, table, ownerCol, ownerID string, items []models.TextRef) error {
	if _, err := tx.Exec(`DELETE FROM `+table+` WHERE `+ownerCol+` = ?`, ownerID); err != nil {
		return err
	}
	for i, it := range items {
		trID, err := insertTextRef(tx, &it)
		if err != nil {
			return err
		}
		if trID == 0 {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO `+table+`(`+ownerCol+`, position, text_ref_id) VALUES (?, ?, ?)`, ownerID, i, trID); err != nil {
			return err
		}
	}
	return nil
}

func loadTextRefList(q *sql.Tx, table, ownerCol, ownerID string) ([]models.TextRef, error) {
	rows, err := q.Query(
		`SELECT tr.text, tr.ref, tr.ref_type FROM `+table+` t
		 JOIN text_refs tr ON tr.id = t.text_ref_id
		 WHERE t.`+ownerCol+` = ? ORDER BY t.position`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.TextRef
	for rows.Next() {
		var tr models.TextRef
		var ref, refType string
		if err := rows.Scan(&tr.Text, &ref, &refType); err != nil {
			return nil, err
		}
		tr.Ref = models.ID(ref)
		tr.Type = models.Type(refType)
		out = append(out, tr)
	}
	return out, rows.Err()
}
```

Замечание: `loadTextRefList` принимает `*sql.Tx` — для чтения вне транзакции достаточно конвертации; в SQLite `*sql.DB` и `*sql.Tx` оба имеют `Query`. Чтобы не дублировать, определи интерфейс:

```go
type queryer interface {
	Query(query string, args ...any) (*sql.Rows, error)
	QueryRow(query string, args ...any) *sql.Row
}
```

и используй `q queryer` вместо `*sql.Tx`/`*sql.DB`.

- [ ] **Step 4: search_index helper**

```go
// replaceSearchIndex переписывает поисковые термины сущности (lower в Go).
// fields — отображение поле → список терминов.
func replaceSearchIndex(tx *sql.Tx, table string, id models.ID, fields map[string][]string) error {
	if _, err := tx.Exec(`DELETE FROM search_index WHERE entity_table = ? AND entity_id = ?`, table, string(id)); err != nil {
		return err
	}
	for field, terms := range fields {
		for _, term := range terms {
			if term == "" {
				continue
			}
			if _, err := tx.Exec(`INSERT INTO search_index(entity_table, entity_id, field, term) VALUES (?, ?, ?, ?)`,
				table, string(id), field, storage.Normalize(term)); err != nil {
				return err
			}
		}
	}
	return nil
}

// searchIDs возвращает id сущностей таблицы, чей термин начинается с query.
// query нормализуется здесь же (обе стороны в lower — регистронезависимость
// без участия sqlite-коллации).
func (s *Store) searchIDs(table, query string) ([]models.ID, error) {
	norm := storage.Normalize(query)
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(norm)
	rows, err := s.db.Query(
		`SELECT entity_id FROM search_index WHERE entity_table = ? AND term LIKE ? ESCAPE '\'`,
		table, escaped+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.ID
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, models.ID(id))
	}
	return out, rows.Err()
}
```

Примечание: `searchIDs` в S1+S2 из порта не вызывается (порт Task 11 содержит только `Get/Save/List`), это помощник для будущего поискового сценария с `ListAdministrativeDivisions`/`ListPeople` — оставляем или удаляем по вкусу; `replaceSearchIndex` обязателен (вызывается в каждом `Save*`).

**Get-side помощники** (дополнение к Step 4/5 в `helpers.go`) — чтение дат, якорей, связей источников и строковых списков:

```go
// loadDate возвращает FactDate по id; id = 0 — пустая дата (поле не заполнено).
func loadDate(q queryer, id int64) (models.FactDate, error) {
	var d models.FactDate
	if id == 0 {
		return d, nil
	}
	var prec, mod string
	err := q.QueryRow(`SELECT year, month, day, precision, modifier, year_to, month_to, day_to
		FROM dates WHERE id = ?`, id).
		Scan(&d.Year, &d.Month, &d.Day, &prec, &mod, &d.YearTo, &d.MonthTo, &d.DayTo)
	if err != nil {
		return models.FactDate{}, err
	}
	d.Precision = models.FactPrecision(prec)
	d.Modifier = models.FactModifier(mod)
	return d, nil
}

// loadAnchor возвращает Anchor по id (дискриминатор в колонке kind).
func loadAnchor(q queryer, id int64) (models.Anchor, error) {
	if id == 0 {
		return nil, nil
	}
	var kind, nodeID, docID, rect, attID, timecode, url string
	var page int
	if err := q.QueryRow(`SELECT kind, node_id, document_id, page, rect, attachment_id, timecode, url
		FROM anchors WHERE id = ?`, id).
		Scan(&kind, &nodeID, &docID, &page, &rect, &attID, &timecode, &url); err != nil {
		return nil, err
	}
	switch models.AnchorKind(kind) {
	case models.AnchorArchive:
		return &models.ArchiveAnchor{NodeID: models.ID(nodeID), DocumentID: models.ID(docID), Page: page, Rect: rect}, nil
	case models.AnchorFile:
		return &models.FileAnchor{AttachmentID: models.ID(attID), Timecode: timecode}, nil
	case models.AnchorURL:
		return &models.URLAnchor{URL: url}, nil
	}
	return nil, fmt.Errorf("unknown anchor kind %q", kind)
}

// loadSourceLinks читает доказательства сущности (полиморфная target_type).
func loadSourceLinks(q queryer, targetType, targetID string) ([]models.SourceLink, error) {
	rows, err := q.Query(`SELECT citation_id, reliability, role, note
		FROM source_links WHERE target_type = ? AND target_id = ? ORDER BY id`,
		targetType, targetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.SourceLink
	for rows.Next() {
		var sl models.SourceLink
		var citationID, reliability, role, note string
		if err := rows.Scan(&citationID, &reliability, &role, &note); err != nil {
			return nil, err
		}
		sl.CitationID = models.ID(citationID)
		sl.Reliability = models.Reliability(reliability)
		sl.Role, sl.Note = role, note
		out = append(out, sl)
	}
	return out, rows.Err()
}

// replaceStringList / loadStringList — для списков-значений (ad_variants, church_variants).
func replaceStringList(tx *sql.Tx, table, ownerCol, ownerID string, values []string) error {
	if _, err := tx.Exec(`DELETE FROM `+table+` WHERE `+ownerCol+` = ?`, ownerID); err != nil {
		return err
	}
	for i, v := range values {
		if v == "" {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO `+table+`(`+ownerCol+`, position, value) VALUES (?, ?, ?)`,
			ownerID, i, v); err != nil {
			return err
		}
	}
	return nil
}

func loadStringList(q queryer, table, ownerCol, ownerID string) ([]string, error) {
	rows, err := q.Query(`SELECT value FROM `+table+` WHERE `+ownerCol+` = ? ORDER BY position`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// replaceRenames / loadRenames — для ad_renames (text + строковые since/until).
func replaceRenames(tx *sql.Tx, table, ownerCol, ownerID string, rns []models.AdministrativeRename) error {
	if _, err := tx.Exec(`DELETE FROM `+table+` WHERE `+ownerCol+` = ?`, ownerID); err != nil {
		return err
	}
	for i, rn := range rns {
		if _, err := tx.Exec(`INSERT INTO `+table+`(`+ownerCol+`, position, text, since, until) VALUES (?, ?, ?, ?, ?)`,
			ownerID, i, rn.Text, rn.Since, rn.Until); err != nil {
			return err
		}
	}
	return nil
}

func loadRenames(q queryer, table, ownerCol, ownerID string) ([]models.AdministrativeRename, error) {
	rows, err := q.Query(`SELECT text, since, until FROM `+table+` WHERE `+ownerCol+` = ? ORDER BY position`, ownerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.AdministrativeRename
	for rows.Next() {
		var rn models.AdministrativeRename
		if err := rows.Scan(&rn.Text, &rn.Since, &rn.Until); err != nil {
			return nil, err
		}
		out = append(out, rn)
	}
	return out, rows.Err()
}
```

Также переведи `loadTextRefList` (Step 4) с `*sql.Tx` на `queryer` — он вызывается и в Get (вне транзакции).

- [ ] **Step 6 (развязка): маппинг сущностей — Save/Get/List**

Файлы реализации: `person.go` (Person), `dictionaries.go` (новый: Surname, GivenName, Patronymic, Estate, Title), `relations.go` (новый: Relation, Residence, Family), `places.go` (переименование `settlement.go`: AdministrativeDivision, Church, Parish), `records.go` (Event, Source), `archive.go` (archive-группа). Шаблон на каждую сущность одинаков: **Save** = транзакция (upsert главной строки → перезапись дочерних списков → `replaceSourceLinks` → `replaceSearchIndex`), **Get** = главная строка + дочерние списки + `loadSourceLinks`, **List** = `SELECT id … ORDER BY rowid` + Get по каждому (масштаб личной генеалогии позволяет).

Полностью расписанный шаблон — Person (`person.go`):

```go
func (s *Store) SavePerson(p *models.Person) error {
	return s.db.Tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`INSERT INTO persons(id, gender) VALUES (?, ?)
			ON CONFLICT(id) DO UPDATE SET gender = excluded.gender`,
			string(p.ID), string(p.Gender)); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM person_names WHERE person_id = ?`, string(p.ID)); err != nil {
			return err
		}
		// person_names.surname_id/given_id/patronymic_id NOT NULL REFERENCES text_refs —
		// вставляем TextRef всегда (даже пустую строку), либо сними NOT NULL с этих колонок.
		for _, n := range p.Names {
			surID, err := insertTextRef(tx, &n.Surname)
			if err != nil {
				return err
			}
			givID, err := insertTextRef(tx, &n.Given)
			if err != nil {
				return err
			}
			patID, err := insertTextRef(tx, &n.Patronymic)
			if err != nil {
				return err
			}
			sinceID, err := insertDate(tx, n.Since)
			if err != nil {
				return err
			}
			untilID, err := insertDate(tx, n.Until)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(`INSERT INTO person_names(person_id, type, surname_id, given_id, patronymic_id, since_id, until_id)
				VALUES (?, ?, ?, ?, ?, ?, ?)`,
				string(p.ID), string(n.Type), surID, givID, patID, sinceID, untilID); err != nil {
				return err
			}
		}
		for _, c := range []struct {
			table string
			items []models.TextRef
		}{
			{"person_estates", p.Estates},
			{"person_titles", p.Titles},
			{"person_notes", p.Notes},
		} {
			if err := replaceTextRefList(tx, c.table, "person_id", string(p.ID), c.items); err != nil {
				return err
			}
		}
		if err := replaceSourceLinks(tx, "person", string(p.ID), p.Sources); err != nil {
			return err
		}
		var terms []string
		for _, n := range p.Names {
			if n.Surname.Text != "" {
				terms = append(terms, n.Surname.Text)
			}
			if n.Given.Text != "" {
				terms = append(terms, n.Given.Text)
			}
			if n.Patronymic.Text != "" {
				terms = append(terms, n.Patronymic.Text)
			}
		}
		return replaceSearchIndex(tx, "persons", p.ID, map[string][]string{"name": terms})
	})
}

func (s *Store) GetPerson(id models.ID) (*models.Person, error) {
	var p models.Person
	var gender string
	if err := s.db.QueryRow(`SELECT id, gender FROM persons WHERE id = ?`, string(id)).
		Scan(&p.ID, &gender); err != nil {
		return nil, err
	}
	p.Gender = models.PersonGender(gender)
	rows, err := s.db.Query(`SELECT type, surname_id, given_id, patronymic_id, since_id, until_id
		FROM person_names WHERE person_id = ? ORDER BY rowid`, string(id))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var n models.PersonName
		var typ string
		var surID, givID, patID, sinceID, untilID int64
		if err := rows.Scan(&typ, &surID, &givID, &patID, &sinceID, &untilID); err != nil {
			return nil, err
		}
		n.Type = models.PersonNameType(typ)
		if n.Surname, err = loadTextRef(s.db, surID); err != nil {
			return nil, err
		}
		if n.Given, err = loadTextRef(s.db, givID); err != nil {
			return nil, err
		}
		if n.Patronymic, err = loadTextRef(s.db, patID); err != nil {
			return nil, err
		}
		if n.Since, err = loadDate(s.db, sinceID); err != nil {
			return nil, err
		}
		if n.Until, err = loadDate(s.db, untilID); err != nil {
			return nil, err
		}
		p.Names = append(p.Names, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if p.Estates, err = loadTextRefList(s.db, "person_estates", "person_id", string(id)); err != nil {
		return nil, err
	}
	if p.Titles, err = loadTextRefList(s.db, "person_titles", "person_id", string(id)); err != nil {
		return nil, err
	}
	if p.Notes, err = loadTextRefList(s.db, "person_notes", "person_id", string(id)); err != nil {
		return nil, err
	}
	if p.Sources, err = loadSourceLinks(s.db, "person", string(id)); err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Store) ListPeople() ([]*models.Person, error) {
	rows, err := s.db.Query(`SELECT id FROM persons ORDER BY rowid`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []models.ID
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, models.ID(id))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]*models.Person, 0, len(ids))
	for _, id := range ids {
		p, err := s.GetPerson(id)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}
```

`loadTextRef(q, id int64)` — одиночный TextRef по id (по образцу `loadTextRefList`, но без owner-колонки) — добавь рядом с прочими get-помощниками.

Остальные сущности — по этому же шаблону. Сводная карта «сущность → файл → таблицы/коллекции → sources → search_index»:

| Сущность | Файл | Главная строка (+ даты/TextRef колонки) | Дочерние коллекции (колонка в связной = `*_id`, helpers в скобках) | Sources | search_index (`field: terms`) |
|---|--|--|--|--|--|
| Person | person.go | persons (+gender) | person_names (вручную, TextRefs+даты), person_estates/titles/notes (replaceTextRefList) | да | `name`: все Surname/Given/Patronymic |
| Relation | relations.go | relations (kind, person_a, person_b, since, until) | relation_notes (replaceTextRefList) | да | — |
| Residence | relations.go | residences (person_id, place_id, since, until, note) | — | да | — |
| Family | relations.go | families (name) | family_members, family_notes (replaceTextRefList) | да | `name` |
| Surname | dictionaries.go | surnames (canonical) | surname_variants/_items/_notes (replaceTextRefList) | да | `name`: canonical + variants |
| GivenName | dictionaries.go | given_names (canonical, gender) | given_name_variants/_items/_notes (replaceTextRefList) | да | `name`: canonical + variants |
| Patronymic | dictionaries.go | patronymics (canonical) | patronymic_variants/_items/_notes | да | `name` |
| Estate | dictionaries.go | estates (title) | estate_variants/_items/_notes | да | `name`: title + variants |
| Title | dictionaries.go | titles (title) | title_variants/_items/_notes | да | `name` |
| AdministrativeDivision | places.go | administrative_divisions (name, type, parent_id, since, until) | ad_items/notes (replaceTextRefList), ad_variants (replaceStringList), ad_renames (replaceRenames), ad_successors (replaceTextRefList) | да | `name`: name + variants |
| Church | places.go | churches (name, parish_id TextRef) | church_settlements (replaceTextRefList), church_variants (replaceStringList), church_notes | да | `name`: name + variants |
| Parish | places.go | parishes (name, church_id TextRef, since, until) | parish_settlements (replaceTextRefList), parish_notes | да | `name` |
| Event | records.go | events (type, date_id, place_id TextRef) | event_participants (прямые колонки person_id/role/note), event_notes (replaceTextRefList) | да | `place`: текст места |
| Source | records.go | sources (kind, title, text, author, date_id, reliability, anchor_id) | source_notes (replaceTextRefList) | — | `title`+`author`+`text` |
| Archive | archive.go | archives (name, system_id TextRef) | archive_notes | да | `name` |
| ArchiveNode | archive.go | archive_nodes (type, archive_id, parent_id, label, name, since, until, parish_id TextRef) | node_settlements, node_notes (replaceTextRefList) | да | `name`: label + name |
| ArchiveDocument | archive.go | archive_documents (unit_id, title, kind, since, until, parish_id TextRef) | doc_settlements, doc_notes (replaceTextRefList) | да | `title` |
| Attachment | archive.go | attachments (kind, uri, filename, mime, page, node_id, document_id, note) | — | — | `filename`+`uri` |

Общее по всем: главная строка — upsert по `ON CONFLICT(id) DO UPDATE SET …` (колонки ровно те, что у сущности); дочерние — delete+insert (перезапись); `replaceSourceLinks(tx, "архивная_table_строка_target_type", string(id), src)`; `replaceSearchIndex` по полям из колонки search_index. Для save/get `date_id`/`since_id`/`until_id`/`anchor_id` — `insertDate`/`loadDate`/`insertAnchor`/`loadAnchor`; одиночные TextRef-колонки — `insertTextRef`/`loadTextRef`.

`event_participants` не является сущностью — это коллекция: в `SaveEvent` перезаписывается (delete+insert), в `GetEvent` читается и собирается в `Event.Participants []EventParticipant` (поля `PersonID models.ID, Role, Note string`).

Проверка после группы: `go build ./internal/store/...` — интерфейс `store.Store` реализован.

- [ ] **Step 5: Переписать `sqlstore_test.go` под новый API**

Обнови `internal/store/sqlstore/sqlstore_test.go` (классический тест с временной БД: `storage.Open(t.TempDir())` → `Open(storage)` → сценарий). Покрыть:

```go
func TestStoreRoundTripPerson(t *testing.T)      // Save → Get → сравнение всех полей (names/estates/titles/notes/sources)
func TestStoreRoundTripDivision(t *testing.T)    // AD с variants/renames/items/notes/sources → Get
func TestStoreListPeople(t *testing.T)           // List после нескольких Save; порядок по rowid
func TestStoreSearchIndex(t *testing.T)          // после SavePerson записать в search_index
                                                  // lower-термины (проверка raw-query по search_index),
                                                  // регистр-независимость lookup
func TestStoreFKRESTRICT(t *testing.T)           // Person с Relation; удалить Relation нельзя→ Relation удаляют через cascade;
                                                  // попытка SQL DELETE relations с существующей relation_notes — RESTRICT
func TestStoreCascadeDelete(t *testing.T)        // DELETE persons каскадно сносит person_names/notes,
                                                  // но relations (RESTRICT) блокирует
func TestStoreCount(t *testing.T)                // storage.DB.Count("persons") == числу сохранённых
```

Цель — не «повторить sqlstore», а зафиксировать контракты: сохранение/чтение целостно, каскады и RESTRICT работают, search_index заполняется в lower.

---

## PART D — Переключение потребителей и удаление `entity`

### Task 13: usecase, definitions, обработчики на `transport`; удаление `internal/entity`

**Files:**
- Modify: `internal/usecases/list_settlements/deps.go`
- Modify: `internal/usecases/list_settlements/scenario.go`
- Modify: `internal/usecases/list_settlements/scenario_test.go`
- Regenerate: `internal/usecases/list_settlements/deps_test.go`
- Modify: `internal/definitions/russia/soviet_russia.go`
- Modify: `internal/definitions/russia/tsardom_russia_1708_1917.go`
- Modify: `internal/httpapi/deps.go`
- Modify: `internal/httpapi/settlement.go`
- Create: `internal/httpapi/settlement_test.go`
- Modify: `internal/mcp/deps.go`
- Modify: `internal/mcp/settlement.go`
- Create: `internal/mcp/settlement_test.go`
- Modify: `internal/app/app.go` (при необходимости — передача `store` в `list_settlements.New`)
- Delete: `internal/models/settlement.go`
- Delete: `internal/entity/` (весь каталог)

**Interfaces:**
- Consumes: `store.Store` (Task 11), `sqlstore.Store` (Task 12), `transport.SettlementsFromModels` (Task 9), `models.AdministrativeDivision.Type.IsSettlement()` (Task 2).
- Produces: `list_settlements.Scenario.ListSettlements(ctx) ([]models.AdministrativeDivision, error)`; интерфейсы `SettlementService` в `httpapi`/`mcp` возвращают `[]models.AdministrativeDivision`.

- [ ] **Step 1: Сценарий `list_settlements` на `models`**

`internal/usecases/list_settlements/deps.go`:

```go
package list_settlements

import "github.com/amarin/genodex/internal/models"

// AdminDivisionRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type AdminDivisionRepo interface {
	ListAdministrativeDivisions() ([]*models.AdministrativeDivision, error)
}
```

`internal/usecases/list_settlements/scenario.go` — переименуй поле `settlements` → `adminDivisions`, конструктор `New` принимает `AdminDivisionRepo`; метод:

```go
// ListSettlements возвращает населённые пункты (единицы деления вида нас. пункта).
func (s *Scenario) ListSettlements(ctx context.Context) ([]models.AdministrativeDivision, error) {
	divisions, err := s.adminDivisions.ListAdministrativeDivisions()
	if err != nil {
		return nil, err
	}
	out := make([]models.AdministrativeDivision, 0, len(divisions))
	for _, d := range divisions {
		if d.Type.IsSettlement() {
			out = append(out, *d)
		}
	}
	return out, nil
}
```

Удали функцию `toModel` (конвертации `entity → models` больше нет). Перегенерируй мок: удали старое `deps_test.go`, затем `go generate ./internal/usecases/list_settlements/...`.

`internal/usecases/list_settlements/scenario_test.go`:

```go
package list_settlements

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	list []*models.AdministrativeDivision
	err  error
}

func (f *fakeRepo) ListAdministrativeDivisions() ([]*models.AdministrativeDivision, error) {
	return f.list, f.err
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

func TestScenarioPropagatesRepoError(t *testing.T) {
	wantErr := errors.New("repo down")
	sc := New(&fakeRepo{err: wantErr})

	if _, err := sc.ListSettlements(context.Background()); !errors.Is(err, wantErr) {
		t.Errorf("err=%v, want %v", err, wantErr)
	}
}
```

- [ ] **Step 2: `definitions/russia` на `models`**

В `soviet_russia.go` и `tsardom_russia_1708_1917.go` замени импорт `internal/entity` → `internal/models`, тип `entity.AdministrativeDivisionType` → `models.AdminDivisionType`, `entity.AdministrativeDivisionSystem` → `models.AdministrativeDivisionSystem`:

Run: `perl -pi -e 's#internal/entity#internal/models#; s#entity\.AdministrativeDivisionType#models.AdminDivisionType#g; s#entity\.#models.#g' internal/definitions/russia/*.go && gofmt -w internal/definitions/russia`

- [ ] **Step 3: Обработчики отдают `transport.Settlement`**

`internal/httpapi/deps.go` и `internal/mcp/deps.go` — интерфейс `SettlementService`:

```go
type SettlementService interface {
	ListSettlements(ctx context.Context) ([]models.AdministrativeDivision, error)
}
```

`internal/httpapi/settlement.go`:

```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

func handleSettlementList(settlements SettlementService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := settlements.ListSettlements(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, transport.SettlementsFromModels(list))
	}
}
```

`internal/mcp/settlement.go` — в обработчике замени `json.Marshal(list)` на `json.Marshal(transport.SettlementsFromModels(list))` и добавь импорт `github.com/amarin/genodex/internal/transport`.

- [ ] **Step 4: Тесты обработчиков**

`internal/httpapi/settlement_test.go`:

```go
package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeSettlements struct {
	list []models.AdministrativeDivision
	err  error
}

func (f fakeSettlements) ListSettlements(context.Context) ([]models.AdministrativeDivision, error) {
	return f.list, f.err
}

func TestSettlementListContract(t *testing.T) {
	h := NewHandler(fakeSettlements{list: []models.AdministrativeDivision{
		{ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo},
	}}, fstest.MapFS{})

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/settlements", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	want := `[{"id":"ad-1","name":"Давыдово","type":"selo"}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}
```

`internal/mcp/settlement_test.go`:

```go
package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/amarin/genodex/internal/models"
)

type fakeSettlements struct {
	list []models.AdministrativeDivision
}

func (f fakeSettlements) ListSettlements(context.Context) ([]models.AdministrativeDivision, error) {
	return f.list, nil
}

func TestSettlementListToolContract(t *testing.T) {
	handler := settlementListHandler(fakeSettlements{list: []models.AdministrativeDivision{
		{ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo},
	}})

	res, err := handler(context.Background(), mcp.CallToolRequest{})
	if err != nil {
		t.Fatal(err)
	}
	text, ok := mcp.AsTextContent(res.Content[0])
	if !ok {
		t.Fatalf("content[0] = %T, want TextContent", res.Content[0])
	}
	want := `[{"id":"ad-1","name":"Давыдово","type":"selo"}]`
	if text.Text != want {
		t.Fatalf("text = %s, want %s", text.Text, want)
	}
}
```

- [ ] **Step 5: Удалить `models.Settlement` и пакет `entity`**

Run: `git rm internal/models/settlement.go && git rm -r internal/entity`
Run: `grep -rn "internal/entity" --include=*.go .` → пусто.

Если `grep` находит импортёров — перенеси их на `models` тем же приёмом (`entity.X` → `models.X`).

- [ ] **Step 6: Рубеж — зелёное дерево**

```bash
gofmt -l .            # пусто
go build ./...
go vet ./...
go test ./...
grep -rn 'json:"' internal/models   # пусто
```

Smoke: `go run ./cmd/genodex -p 8080`, затем:
- `GET /api/health` → 200;
- `GET /api/settlements` → `[{"id":…,"name":…,"type":…}]`;
- MCP-тул `settlement_list` → тот же список;
- `sqlite3 <data>/db/genodex.db 'SELECT schema_version FROM meta'` → `0`.

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "feat: обработчики отдают transport.Settlement; сценарии и definitions на models; internal/entity удалён"
```

## После выполнения

- Перенеси этот план в `docs/superpowers/archive/` (или удали); в `docs/todo.md` сними отметки `[~]` с блоков A/B и отметь упразднение `internal/entity` выполненным.
- В `docs/architecture.md` убери пометку о переходе (`internal/entity` и JSON-блоб в `sqlstore` больше нет); в `docs/usage.md` обнови форму ответа `/api/settlements`.
- S3: переименования публичных контрактов (`settlement_list` → `division_list`, `/api/settlements` → `/api/admin-divisions`) — правка `transport` и обработчиков; usecase `list_divisions`; адаптация фронтенда (`web/src`) под форму `{id, name, type}`.
- S4: прогон `internal/definitions` через новую модель.
