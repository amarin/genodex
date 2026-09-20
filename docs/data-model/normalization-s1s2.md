# Дизайн: нормализация S1+S2

Утверждённый дизайн прохода «нормализованная модель сущностей + полная колоночная
схема БД». Охватывает разделы A и B плана `docs/todo.md`; контракты (C) и
встроенные данные (D) — следующие проходы.

Исходники истины: `decisions.md`, `docs/models/*`.
Изменение договорённостей против текущего кода:

- `Metadata map[string]string` удаляется из всех сущностей и моделей — полей с
  произвольной структурой не будет нигде (решение пользователя).
- `settlement` больше не отдельная сущность: населённый пункт — уровень
  `AdministrativeDivision.type`. Доменный тип `Settlement` удаляется из `models`
  (пакет `entity` упраздняется целиком). Публичный контракт `settlement_list`
  пока сохраняет имя и отдаётся DTO `transport.Settlement{id, name, type}`;
  сценарий возвращает домен `AdministrativeDivision`. Переименование
  публичных контрактов — проход C и правка только в `transport`.
- Тип сущности `first_name` переименован в `given_name` для соответствия
  `docs/models/people.md` (Surname / GivenName / Patronymic).

## 1. Дома и границы пакетов

Стабильно то, что внутри; пластично то, что наружу.

```
models     — домен, без импортов и без JSON-тегов
store, usecases, definitions, storage → models
transport  → models                    (DTO + конвертеры)
httpapi, mcp → интерфейсы сценариев + transport
entity     — упразднён
```

- `internal/models` — единственный дом модели: сущности, value-типы,
  enum-домены, `ID`, `Type`. Никаких зависимостей; JSON-тегов нет — JSON это
  внешний контракт, и домен о нём не знает. Домен стабилен после завершения
  проектирования. На нём говорят хранилище (`internal/store`/`sqlstore`,
  columnar-схема), сценарии (`internal/usecases`) и встроенные данные
  (`internal/definitions`).
- `internal/transport` — слой DTO для обработчиков: пластичные типы с
  JSON-тегами, зеркалящие публичные контракты (`/api/*` и MCP-тулы), плюс
  конвертеры `models → transport`. Его импортируют только `httpapi` и `mcp`.
  Направление зависимости: `transport → models`; `models` от `transport` не
  зависит. Подробности — раздел 8.
- Пакет `internal/entity` упраздняется полностью: доменная часть уходит в
  `models`, контрактная — в `transport`; импортёров `entity` не остаётся.
  Границы в `docs/architecture.md` и `AGENTS.md` обновляются.
- Value-типы живут в `models`: `FactDate`, `TextRef`, `NamedPeriod`, `PlaceRef`,
  `Anchor` (`ArchiveAnchor`/`FileAnchor`/`URLAnchor`), `AnchorKind`, `SourceLink`.
  У `FactDate` в домене остаются поля, `Compare`, `SameYear`/`SameMonth`,
  `ParseFactDate` и `String()` (каноническая текстовая форма); JSON-теги
  снимаются, форма на проводе — забота `transport`.
- Почему DTO, а не общий тип: контракт — производная от домена, а не источник
  истины. Изменения API (переименования контрактов, формы ответов и запросов)
  правятся только в `transport` и не трогают ни домен, ни хранилище. Например,
  `GET /api/settlements` и `settlement_list` могут отдавать `{id, name, type}`
  под именем `transport.Settlement`, а домен растёт как `AdministrativeDivision`.
  Сейчас путь чтения один, writes нет — конвертеры крошечные.

## 2. Enum-домены (номинальные типы в internal/models)

Для каждого поля с ограниченным набором значений — собственный тип `string` и
константы в `models`. `transport` не алиасит их: поля DTO ссылаются на
`models.SourceKind` и т. п. напрямую (значение на проводе = константа домена).
Если значение на проводе понадобится отделить от домена, в `transport`
вводится собственный тип точечно.

| Тип | Значения | Где используется |
|---|---|---|
| `models.Type` (переименование `EntityType`) | person, surname, given_name, patronymic, estate, title, church, parish, administrative_division, event, source, citation, note, repository, relation, residence, family, archive, archive_node, archive_document, attachment | дискриминатор сущности (было `EntityType`) |
| `PersonGender` | unknown, male, female | `Person.gender` |
| `NameGender` | male, female, neutral | `GivenName.gender`, вывод пола из имени |
| `PersonNameType` | main, birth, married, changed, pseudonym | `PersonName.type` |
| `RelationKind` | blood, marriage, adoption, associate | `Relation.kind` (associate — деятельная связь, не участвует в цепочках родства) |
| `RelationType` | neighbor, colleague, godparent, witness, friend (+ открытый) | `Relation.rel_type` (для `kind=associate`) |
| `EventType` | birth, death, burial, marriage, confession, census (+ открытый набор) | `Event.type` |
| `SourceKind` | archival-scan, transcription, document, audio, photo, memory, external | `Source.kind` |
| `AttachmentKind` | scan, document, audio, photo | `Attachment.kind` |
| `NoteKind` | note, article, book, chapter (+ открытый) | `Note.kind` |
| `RepositoryType` | archive, library, museum, private, other (+ открытый) | `Repository.type` |
| `AdminDivisionType` (переименование `AdministrativeDivisionType`) | governorate, district, volost, other + виды нас. пунктов: gorod, selo, derevnya, hutor, pogost, stanitsa, mestechko | `AdministrativeDivision.type` |
| `Reliability` | primary, contemporary, memory, indirect, unknown | `Source.reliability`, `SourceLink.reliability` |
| `FactPrecision`, `FactModifier`, `FactCalendar` | (`FactDate`): precision/modifier как сейчас; `FactCalendar` = gregorian, julian, unknown | даты |
| `AnchorKind` | archive, file, url | сериализация `Anchor` |

Константы `AdminDivisionType` для России (губерния/уезд/волость) живут в
`internal/definitions/russia` как значения типа `AdminDivisionType`.

Перенос типов из упраздняемого `entity` в `models`: `entity/entity_type.go`
(→ `Type` в `type.go`), `entity/name_gender.go` (→ `NameGender`),
`entity/administrative_division_type.go` (→ `AdminDivisionType`),
`entity/person_names.go` (→ `PersonNameType`). Один файл — один тип/struct,
snake_case по имени.

Имена констант — литерал значения (как в `EntityType`: `TypePerson`,
`PersonGenderMale`) либо префикс типа (`NameGenderMale`).

## 3. ID как отдельный тип

Вводится `type ID string` в `internal/models`. Все id-поля сущностей и строгие
ссылки (`parent_id`, `person_a`, `archive_id`, `node_id`, `person_id`, `place_id`,
`source_id`, `unit_id`, `document_id`, `target_id`) — типа `ID` (не `string`).
`TextRef.ref` — тоже `ID`. Это даёт проверку типов для жёстких ссылок;
несовместимые строки не смешиваются с id без явного приведения.

Прочие атрибуты-строки (`name`, `title`, `label`, `canonical`, `role`, `note`,
текст `TextRef`) остаются базовым `string`. Полная номинальная типизация всех
атрибутов — отдельный будущий проход.

## 4. Сущности internal/models (по docs/models/*)

Ниже поля описаны с именами по `docs/models/*` (snake_case); в Go — обычные
CamelCase-поля без JSON-тегов.

Базовые поля: `id ID`, списки `sources []SourceLink` и `notes []TextRef` — у
сущностей, где они есть в модели. `Metadata` нигде нет. Флаг `private bool`
(решение 24) — у primary-сущностей.

### Person
`id`, `gender PersonGender`, `names []PersonName`, `estates []TextRef`,
`titles []TextRef`, `nicknames []TextRef`, `notes []TextRef`, `sources []SourceLink`,
`private bool`.

`PersonName` (вложенный): `type PersonNameType`, `surname TextRef`,
`given TextRef`, `patronymic TextRef`, `prefix string?`, `suffix string?`,
`since FactDate?`, `until FactDate?`.

### Relation
`id`, `kind RelationKind`, `rel_type RelationType?` (для `kind=associate`),
`person_a ID`, `person_b ID`, `since FactDate?`, `until FactDate?`,
`sources []SourceLink`, `notes []TextRef`, `private bool`.

### Family
`id`, `name string`, `members []TextRef`, `notes []TextRef`, `sources []SourceLink`,
`private bool`.

### Словари: Surname, GivenName, Patronymic, Estate, Title
`id`, `canonical string`, `variants []TextRef`, `items []TextRef`,
`notes []TextRef`; у `GivenName` — `gender NameGender`.

### AdministrativeDivision
`id`, `name string`, `type AdminDivisionType`, `parent_id ID?`,
`items []TextRef`, `variants []string`, `renames []NamedPeriod`,
`successors []TextRef`, `since FactDate?`, `until FactDate?`,
`notes []TextRef`, `sources []SourceLink`.

### Church, Parish
`id`, `name string`; Church: `parish TextRef?`, `settlements []TextRef`,
`variants []string`, `notes`, `sources`. Parish: `church TextRef?`,
`settlements []TextRef`, `since?`, `until?`, `notes`, `sources`.

### Event
`id`, `type EventType`, `date FactDate?`, `place PlaceRef?`,
`participants []EventParticipant`, `sources []SourceLink`, `notes []TextRef`,
`private bool`.

`EventParticipant`: `person_id ID`, `role string`, `note string?`.

### Residence
`id`, `person_id ID`, `place_id ID`, `since FactDate?`, `until FactDate?`,
`sources []SourceLink`, `note string?`, `private bool`.

### Source
`id`, `kind SourceKind`, `title string`, `author string?`, `date FactDate?`,
`reliability Reliability`, `repository_id ID?`, `notes []TextRef`, `private bool`.

### Citation
`id`, `source_id ID`, `anchor Anchor?`, `text string?`, `note string?`,
`private bool`.

### Note
`id`, `kind NoteKind`, `title string?`, `text string` (markdown),
`parent_id ID?`, `sources []SourceLink`, `private bool`.

### Repository
`id`, `name string`, `type RepositoryType`, `address string?`, `urls []TextRef`,
`sources []SourceLink`, `private bool`.

### Archive
`id`, `name string`, `system TextRef?`, `repository_id ID?`, `notes []TextRef`,
`sources []SourceLink`, `private bool`.

### ArchiveNode
`id`, `type ArchiveNodeType` (строковый, из системы иерархии архива),
`archive_id ID`, `parent_id ID?`, `label string`, `name string?`,
`since FactDate?`, `until FactDate?`, `parish TextRef?`, `settlements []TextRef`,
`notes []TextRef`, `sources []SourceLink`, `private bool`.

### ArchiveDocument
`id`, `unit_id ID`, `title string`, `kind string?`, `since FactDate?`,
`until FactDate?`, `parish TextRef?`, `settlements []TextRef`,
`notes []TextRef`, `sources []SourceLink`, `private bool`.

### Attachment
`id`, `kind AttachmentKind`, `uri string?`, `filename string?`, `mime string?`,
`page int?`, `node_id ID`, `document_id ID?`, `note string?`, `private bool`.

### Type — дискриминатор сущности
`models.Type` (переименованный `EntityType`) константы покрывают весь набор, включая
новые типы: relation, residence, family, citation, note, repository, archive_node,
archive_document, attachment, словари (surname, given_name, patronymic, estate, title).

## 5. Schema БД (internal/storage)

- `schema_version` → `0` (миграция не обратносовместима; старые данные можно
  потерять — разрешено `docs/todo.md`).
- Таблица `entity` и индекс `idx_entity_search` удаляются.
- Полная колоночная схема: сущность = таблица, коллекция = связная таблица,
  value-тип = колонки/таблицы. JSON-блобов нигде нет.

### Общие таблицы value-типов

`dates(id INTEGER PK, year, month, day, precision, modifier, calendar,
year_to, month_to, day_to)` — все `FactDate` ссылаются `date_id`.

`anchors(id INTEGER PK, kind, node_id, document_id, page, rect, attachment_id,
timecode, url)` — все `Anchor`; колонки зависят от `kind`, остальные NULL.

`text_refs(id INTEGER PK, text, ref, ref_type)` — одиночные `TextRef?`-поля,
которые имеют семантику ссылки (например `AdministrativeDivision.parent` — нет,
это строгий FK). Применяется только там, где `TextRef` как связь: `Church.parish`,
`Parish.church`, `ArchiveNode.parish`, `Archive.system`. Иначе — не `text_refs`:
напр. `Citation.anchor` — anchor отдельно, в `anchors`. См. карту полей ниже.

Многозначные `TextRef`-списки — связные таблицы вида
`<entity>_<field>(entity_id, position, text, ref, ref_type)`, например
`person_estates`, `administrative_division_items`, `family_members`, словарные
`*_variants`, `*_items`, `source_notes` и т.д.

### Таблицы сущностей

| Таблица | Колонки-скаляры | Связные таблицы |
|---|---|---|
| `persons` | id, gender, private | person_names, person_estates, person_titles, person_nicknames, person_notes, source_links(target=person) |
| `person_names` | id, person_id, type, prefix, suffix, since(date_id), until(date_id) | … см. карту |
| `relations` | id, kind, rel_type, person_a, person_b, since, until, private | relation_sources, relation_notes |
| `families` | id, name, private | family_members, family_notes, source_links |
| `surnames`, `given_names`, `patronymics`, `estates`, `titles` | id, canonical(+given_names.gender) | *_variants, *_items, *_notes |
| `administrative_divisions` | id, name, type, parent_id(fk), item ids | ad_items, ad_variants, ad_renames(named_periods), ad_successors, ad_notes, source_links |
| `churches` | id, name, parish text_ref_id | church_settlements(text_refs), church_variants, church_notes, source_links |
| `parishes` | id, name, church text_ref_id, since, until | parish_settlements, parish_notes, source_links |
| `events` | id, type, date(date_id), place text_ref_id, private | event_participants, event_sources, event_notes |
| `event_participants` | event_id, position, person_id, role, note | |
| `residences` | id, person_id, place_id, since, until, note, private | residence_sources |
| `sources` | id, kind, title, author, date(date_id), reliability, repository_id(fk), private | source_notes |
| `citations` | id, source_id(fk), anchor_id, text, note, private | (без списков) |
| `notes` | id, kind, title, text, parent_id(fk), private | note_sources (source_links(target=note)) |
| `repositories` | id, name, type, address, private | repository_urls, repository_notes, source_links(target=repository) |
| `archives` | id, name, system text_ref_id, repository_id(fk), private | archive_notes, source_links |
| `archive_nodes` | id, type, archive_id, parent_id, label, name, since, until, parish text_ref_id, private | node_settlements, node_notes, source_links |
| `archive_documents` | id, unit_id, title, kind, since, until, parish text_ref_id, private | doc_settlements, doc_notes, source_links |
| `attachments` | id, kind, uri, filename, mime, page, node_id, document_id, note, private | (без списков) |
| `source_links` | citation_id, target_type(Type), target_id, reliability, role, note | — глобальная связка «утверждение → цитата» |

`source_links` — единая полиморфная таблица: `target_type` + `target_id`
указывают на утверждение (person/event/relation/…). Это единственная
полиморфная связь в схеме, но она columnar (без JSON).

### Карта TextRef-полей

- Одиночные `TextRef?`-ссылки (`Church.parish`, `Parish.church`, `ArchiveNode.parish`,
  `ArchiveDocument.parish`) — колонка `parish_id` → `text_refs.id`.
- Многозначные `TextRef`-списки — связные таблицы `*_(items|settlements|members|
  estates|titles|nicknames|notes|urls|successors|variants)`.
- `PersonName.surname/given/patronymic` — связные `person_names` с колонками
  `surname_id`/`given_id`/`patronymic_id` → `text_refs.id`.
- `Event.place` — `text_refs` для места. Текстовые колонки (`Citation.text`,
  `Note.text`, `Address`, `Source.title`) — скаляры, не `text_refs`.

### FK и каскады

Дочерние строки удаляются каскадно (`ON DELETE CASCADE`) при удалении сущности.
Строгие ссылки (`parent_id`, `person_a/b`, `archive_id`, `node_id`, `unit_id`,
`person_id`, `place_id`, `citation_id` в `source_links`, `Citation.source_id`,
`Note.parent_id`, `Source.repository_id`, `Archive.repository_id`) — внешние ключи.
Связывающие `text_refs`, `dates`, `anchors` — внешние ключи от владельцев.
Поведение при удалении цели строгой ссылки — `RESTRICT` (нельзя удалить персону,
на которую есть `Relation`), при удалении владельца `source_links`/`text_refs` —
каскад.

### Поиск: таблица search_index

`search_index(entity_table TEXT, entity_id TEXT, field TEXT, term TEXT COLLATE BINARY,
PRIMARY KEY(entity_table, entity_id, field, term))`.

- Термины — normalized lower в Go (`strings.ToLower` из stdlib, корректно для
  кириллицы).
- Канонический регистр остаётся в колонках сущностей; индекс — производная копия.
- При `Save` сущности её поисковые поля (name/label/canonical + variants
  + тексты TextRef) переписываются в индекс; при `Delete` — удаляются каскадом
  по владельцу.
- Запрос: `WHERE term LIKE ?` с параметром = уже lowered строка + `%`
  (без участия sqlite-коллаций — регистронезависимость обеспечена нормализацией
  обеих сторон в Go).
- Поддерживает «точное совпадение» и «совпадение по вариантам» (поле
  различает name/label/canonical/variant) — ранжирование — проход C.

## 6. Порт internal/store

Типизированный порт расширяется на все сущности S1+S2: `GetX/SaveX/ListX` для
каждого типа (`Person`, `Relation`, `Residence`, `Family`, `Surname`,
`GivenName`, `Patronymic`, `Estate`, `Title`, `AdministrativeDivision`, `Church`,
`Parish`, `Event`, `Source`, `Citation`, `Note`, `Repository`, `Archive`,
`ArchiveNode`, `ArchiveDocument`, `Attachment`). Порт оперирует типами
`internal/models`. Методы `GetSettlement/...` удаляются. `go:generate mockgen`
сохраняется.

`sqlstore` маппит сущность ↔ набор строк (вложенные списки — отдельными
INSERT; удаление — каскад по FK).

## 7. usecase settlement_list (в составе S1+S2)

Единственный существующий сценарий: возвращает населённые пункты как
`[]models.AdministrativeDivision` (типы из вида нас. пунктов). Сценарий не знает
про JSON и про `transport`. Публичные имена `settlement_list` (MCP) и
`/api/settlements` (HTTP) пока сохраняются — см. раздел 8; переименование
контрактов — проход C.

## 8. transport: DTO и конвертеры

`internal/transport` — единственное место, где определена форма JSON на проводе.
Его импортируют `httpapi` и `mcp`; `usecases`, `store`, `storage`,
`definitions` о нём не знают.

- Один DTO на публичный контракт, файл на тип (snake_case). В этом проходе —
  `transport.Settlement{ID, Name, Type}` с тегами `id`, `name`, `type`
  (`Type` — `models.AdminDivisionType`, значение на проводе = домен).
- Конвертеры односторонние (домен → DTO): `SettlementFromModel(models.AdministrativeDivision)`
  и `SettlementsFromModels([]models.AdministrativeDivision)` (пустой список — `[]`,
  не `null`). `ToModel` не пишется, пока нет путей записи (появится вместе с
  первой write-операцией).
- Enum-домены и `ID` — типы `models` напрямую, без алиасов (раздел 2).
- Формы value-типов на проводе определяет `transport`, когда они нужны
  контрактам: `FactDate` — строка через `FactDate.String()` (обратно —
  `ParseFactDate`); `Anchor` — плоский DTO с дискриминатором `kind`
  (archive/file/url). В этом проходе контрактов, которые их отдают, нет —
  типы добавляются вместе с первым контрактом, который их использует.
- Обработчики: `httpapi.handleSettlementList` и MCP-тул `settlement_list`
  вызывают сценарий, конвертируют результат через `transport` и сериализуют.
  Интерфейсы `SettlementService` в `httpapi/deps.go` и `mcp/deps.go` возвращают
  `[]models.AdministrativeDivision`.
- Форма ответа меняется с `[{id,name,metadata?}]` на `[{id,name,type}]`;
  `metadata` исчезает вместе с `Metadata` в домене.

## 9. Тесты

- `internal/storage`: создание схемы всех таблиц; FK-каскады (удаление сущности
  удаляет связные строки, `RESTRICT` на строгие ссылки); `search_index`
  заполняется/удаляется; `schema_version=0`.
- `internal/store/sqlstore`: round-trip Save/Get/List для всех типов
  (включая вложенные списки: names, participants, items и т.д.); delete-каскад.
- `internal/transport`: конвертер `Settlement` — поля, `type`, пустой список
  сериализуется как `[]`; JSON-контракт зафиксирован тестом (golden-строка).
- `httpapi`/`mcp`: обработчики отдают форму `transport.Settlement`.
- Бэкап/restore/verify остаются (работают через `VACUUM INTO` — изменений не
  требуют, кроме списка типов в `entity_counts`, который читает из схемы).
- Сохраняются существующие тесты `FactDate`.
- `models` не содержит JSON-тегов и не импортирует `encoding/json` —
  проверяется `grep` на рубеже.

## 10. Рубеж прохода

`gofmt -l .` пусто; `go build ./...`; `go vet ./...`; `go test ./...`;
`grep -rn "internal/entity" .` пусто; `grep -rn 'json:"' internal/models` пусто;
smoke `/api/health`, `/api/settlements`, `/`.
