# Дизайн: ядро чтения и записи

Утверждённый дизайн прохода «подготовка ядра к интерфейсам наполнения и отображения
данных». Предыдущий проход — [нормализация S1+S2](normalization-s1s2.md) (домен
`models`, колоночная схема, порт `store`, DTO `transport`). Реализуется этапами по
[дорожной карте](../plans/2026-09-20-core-rw-roadmap.md).

Исходники истины: [decisions.md](decisions.md), `docs/models/*`, спека S1+S2.

## 1. Принятые решения

| # | Вопрос | Решение | Отвергнуто |
|---|---|---|---|
| 1 | Календарь в `FactDate` | Поле `Calendar` (`gregorian`/`julian`/`unknown`; пусто ≡ `unknown`), сравнение с пересчётом через юлианский день; исходная запись хранится как в источнике | Календарь без пересчёта (сдвиг 12–13 дней ломает порядок близких дат); нормализация в григорианский при вводе (теряется «как в метрике») |
| 2 | Формат `ID` | `ПРЕФИКС-ULID` с короткими префиксами в духе GEDCOM, генерация в сценариях | Транслит-слаги (коллизии и переименования ломают ссылки); UUIDv4 и голый ULID (тип не виден из ссылки) |
| 3 | Чтение для отображения | Целевые query-методы порта + пагинация + `Search` | Отдельный CQRS-слой (вторая модель данных); универсальный фильтр (язык фильтров быстро разрастается) |
| 4 | Запись | CRUD-сценарии по сущностям + `InTx` в порту | Только «намеренческие» сценарии (много специфической логики, порт меняется на каждый) |
| 5 | Удаление | Жёсткое; при наличии ссылающихся — ошибка со списком | Мягкое (колонка во всех таблицах, фильтры везде); каскад зависимых (тихая потеря данных без истории правок) |
| 6 | Приватность | Параметр `Access` (`Full`/`Public`) в query-методах порта и read-сценариях; владелец всегда `Full` | Только данные (потом ломается порт); роли и маскирование (лишнее сейчас) |

Решения 1 и 2 переопределяют старые формулировки: №26 в `decisions.md`
(«сравнения в рамках одного календаря») и `identifiers.md` (id-слаги в транслите).
Оба документа обновляются вместе с этой спекой.

## 2. Домен (`internal/models`)

### 2.1. Календарь

`FactDate` получает поле `Calendar FactCalendar`. Пустое значение эквивалентно
`unknown`.

- **Сравнение** (`Compare`):
  - если календари даты и её аргумента совпадают либо хотя бы один из них `unknown` —
    сравниваются порядковые номера компонентов как сейчас («как есть»);
  - если одна дата юлианская, а другая григорианская — границы юлианской даты
    переводятся в григорианские через юлианский день (JDN). Границы `before`/`after`
    (бесконечности) не трогаются; день верхней границы месяца/года — реальный
    последний день месяца в календаре исходной даты.
- **Разбор** (`ParseFactDate`): суффиксы `ст. ст.`/`ст.ст.` (юлианский) и
  `н. ст.`/`н.ст.` (григорианский) в конце строки, регистр не важен; суффикс
  относится ко всей дате, включая `между X и Y`. Без суффикса календарь пуст.
- **Строка** (`String`): юлианская дата заканчивается ` ст. ст.`, григорианская —
  ` н. ст.`, при пустом/`unknown` суффикса нет. Разбор строки и её вывод
  обратимы.
- **Хранение**: `dates.calendar` пишется и читается как есть (пусто → `''`),
  умолчание колонки — `''`.
- Проверка: `1917-10-25 ст. ст.` и `1917-11-07 н. ст.` — один день.

### 2.2. Идентификаторы

Формат: `ПРЕФИКС-ULID`, ULID — 26 символов алфавита Crockford (без `I L O U`),
заглавные. Пример: `I-01J8X4T0K2M9Q7R5V3B6N8C1D4`.

| Префикс | Тип | Префикс | Тип |
|---|---|---|---|
| `I` | person | `SN` | surname |
| `F` | family | `GN` | given_name |
| `S` | source | `PN` | patronymic |
| `R` | repository | `ES` | estate |
| `N` | note | `TT` | title |
| `O` | attachment | `CH` | church |
| `E` | event | `PR` | parish |
| `C` | citation | `AD` | administrative_division |
| `RL` | relation | `AR` | archive |
| `RS` | residence | `AN` | archive_node |
| | | `DC` | archive_document |

`I F S R N O` совпадают с GEDCOM (INDI, FAM, SOUR, REPO, NOTE, OBJE); остальные
префиксы уникальны и по построению не совпадают ни с одним из них.

- **`models`** (чистые функции): префикс по `Type` и обратно, `ParseID` (тип по
  префиксу с проверкой формата), `ID.Validate(Type)` (формат и соответствие
  префикса типу).
- **Генератор** — интерфейс `IDGenerator` в сценариях; реализация в пакете
  `internal/idgen` (`crypto/rand` + время, монотонность в пределах миллисекунды,
  без внешних зависимостей).
- **`store`** ID не генерирует и не принимает пустой. Явный ID от клиента
  допустим только для импорта и проходит `ID.Validate`.

### 2.3. Инварианты (`Validate`)

Каждая сущность получает `Validate() error` — чистые проверки без обращения к
хранилищу. Ошибка — `*models.ValidationError` (сущность, поле, причина).

- **Идентификаторы:** `ID.Validate(Type)`; строгие ссылки — формат и префикс
  ожидаемого типа (`person_id` → `I`, `place_id` → `AD` и т. д.).
- **Закрытые enum'ы** (принимают только константы): `PersonGender`, `NameGender`,
  `PersonNameType`, `RelationKind`, `SourceKind`, `AttachmentKind`, `Reliability`,
  `AnchorKind`, `FactPrecision`, `FactModifier`, `FactCalendar`, `AdminDivisionType`.
- **Открытые enum'ы** (константы — подсказка): `EventType`, `RelationType`,
  `NoteKind`, `RepositoryType`; допускается непустое значение формата
  `[a-z][a-z0-9_-]*`.
- **Сущностные правила:**
  - `Relation`: при `kind=associate` обязателен `RelType`, при остальных `kind`
    `RelType` пуст; `PersonA != PersonB`.
  - Периоды `since`/`until` (везде, где есть): `since` не позже `until`
    (сравнение `FactDate.Compare`, неопределимое допускается).
  - `FactDate`: `Year` обязателен при `Precision != unknown`; месяц и день в
    допустимых границах; для `between` заполнена верхняя граница.
  - `SourceLink`: `CitationID` обязателен; `TargetType`/`TargetID` при записи
    выставляет адаптер из владельца (пользовательские значения игнорируются).
  - `TextRef`: `Text` не пуст, либо задана ссылка; при `Ref` — `Type` обязателен.
  - Необязательные поля с закрытыми enum'ами (`Person.Gender`, `PersonName.Type`,
    `FactDate.Calendar`, `SourceLink.Reliability`): пустое значение допустимо,
    непустое — только константа. Обязательные (`Relation.Kind`, `GivenName.Gender`,
    `FactDate.Precision`, `FactDate.Modifier`) — только константа.
  - `PersonName`: хотя бы одна из частей (фамилия, имя, отчество) заполнена; ссылки
    частей — на словари ожидаемых типов (`surname`, `given_name`, `patronymic`);
    `Person.Estates` ссылаются на `estate`, `Person.Titles` — на `title`, `Family.Members` — на
    `person` (обычные текстовые имена остаются допустимыми), `variants` словаря —
    на словарь того же типа.
  - `FactDate`: точность ограничивает только нижнюю границу; верхняя граница
    `between` может быть точнее нижней (`между 1880 и 1882-03-05` — допустимо).
  - `FactDate`: точность согласована с компонентами (при `year` месяц и день нулевые,
    при `month` день нулевой, при `unknown` все компоненты нулевые и формулировка
    `exact`); день не больше числа дней месяца в календаре даты (без календаря или при
    `unknown` — по юлианскому правилу, снисходительно); верхняя граница только при `between`
    и не заканчивается раньше начала нижней.
  - `Family.Name` и `canonical` словарей обязательны (непустые после обрезки
    пробелов).
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
- `Validate()` вызывается на непустом указателе; nil-получатель не поддерживается
  (паника).
- **Проверки с обращением к хранилищу** (существование ссылок, циклы по
  `parent_id` у `AdministrativeDivision`, `ArchiveNode`, `Note`) выполняют
  сценарии, а не `models`.

## 3. Хранилище (`internal/storage`, `sqlstore`)

- **Индексы** (в том же `CREATE … IF NOT EXISTS`, миграций данных не требуют):
  - на каждую FK-колонку каждой таблицы, у которой нет своего ведущего индекса
    (владельцы дочерних таблиц, `parent_id`, `person_a/b`, `person_id`,
    `place_id`, `source_id`, `archive_id`, `node_id` и т. д.). Список
    генерируется из `PRAGMA foreign_key_list` при создании схемы, а не
    перечисляется вручную;
  - `search_index(term)` — для поиска по префиксу диапазоном
    `term >= ? AND term < ?` (оператор `LIKE` с `COLLATE BINARY` индекс не
    использует).
- **Чистка при удалении**: `search_index` и `source_links` — полиморфные, FK на
  владельца у них нет; адаптер чистит их сам (в `Save*` и в `Delete*`), а также
  осиротевшие `text_refs`/`dates`/`anchors` (как уже делает `Save*`).
- **DSN**: `foreign_keys` и `busy_timeout` задаются через `_pragma` в DSN,
  чтобы они действовали на каждом соединении.

## 4. Порт `internal/store` и параметры запросов

Ошибки и параметры запросов — в `models` (листовой пакет, их видят и сценарии, и
обработчики):

```go
var ErrNotFound = errors.New("не найдено")

// InUseError — удаление запрещено: на сущность ссылаются другие.
type InUseError struct {
    Type      Type
    ID        ID
    Referrers []EntityRef // до 20 штук
}
type EntityRef struct{ Type Type; ID ID }

type Access int           // AccessFull (0, по умолчанию) | AccessPublic
type Page struct{ Limit, Offset int } // Limit 0 → 50, максимум 500
type Hit struct{ Type Type; ID ID; Label, Field string }
```

Изменения порта:

- `Get*` возвращает `ErrNotFound` (вместо `(nil, nil)`); правило `nil,nil`
  снимается вместе с потребителями.
- `Delete*(id) error` для всех 21 сущностей: удаляет сущность, свои дочерние
  строки, записи `search_index`/`source_links`, осиротевшие значения. Если на
  сущность ссылаются (RESTRICT) — `*InUseError` со списком ссылающихся.
- `InTx(ctx, func(Store) error) error` — все методы порта внутри функции работают
  в одной транзакции; ошибка — откат. Методы порта получают `ctx` (первым
  аргументом) в том же этапе.
- `List*(ctx, Access, Page)` — пагинация везде; `AccessPublic` исключает
  сущности с `private = 1` на уровне SQL (для сущностей без флага — не влияет).
- `Search(ctx, query, Access, Page) ([]Hit, error)` — по `search_index`, префикс
  нормализованного запроса, порядок стабильный.
- Точечные запросы: `ChildrenOfDivision(ctx, parent, Access, Page)`; остальные
  (`EventsOfPerson`, `RelationsOfPerson`, …) вводятся вместе со срезами
  соответствующих сущностей.
- N+1 в `List*` убирается пакетной загрузкой дочерних таблиц (`IN (…)`) сначала
  для `Person` и `AdministrativeDivision`, остальные — по мере срезов.

Что не меняется: агрегаты остаются единственной моделью и для чтения, и для
записи; `Save*` — upsert с заменой дочерних строк.

## 5. Сценарии и контракты

- Сценарий на действие над сущностью: `create/get/update/delete/list/search`, по
  пакету на сценарий, как сейчас (`internal/usecases/<scenario>/`, узкий `deps.go`).
  Составные сценарии — только по реальной необходимости.
- Сценарий `create` генерирует `ID` (интерфейс `IDGenerator`), вызывает
  `Validate`, выполняет проверки с обращением к хранилищу и сохраняет в `InTx`;
  `update` — читает, применяет изменения, валидирует, сохраняет; `delete` —
  `Delete*`, ошибка `*InUseError` пробрасывается.
- Соглашение об именах контрактов: MCP-тулы `<entity>_list|get|create|update|delete|search`,
  HTTP `GET/POST /api/<множественное>`, `GET/PUT/DELETE /api/<множественное>/{id}`.
  DTO и запросы — в `transport`; ошибки сценариев в статусы: `ErrNotFound` → 404,
  `*ValidationError` → 422, `*InUseError` → 409 (тело — список ссылающихся).
- Переименование единственного текущего контракта: `settlement_list` →
  `division_list`, `GET /api/settlements` → `GET /api/admin-divisions`, сценарий
  `list_settlements` → `list_divisions`, DTO `Settlement` → `AdminDivision`
  (`id`, `name`, `type`, `parent_id`). Фильтр «только населённые пункты» становится
  параметром запроса (`kind=settlement`), а не именем контракта.
- Вертикальный срез на делениях (`AdministrativeDivision`): полный CRUD, список
  (с фильтром по `parent_id`, `type`, `kind`), поиск, дочерние единицы. Срез
  служит образцом для остальных групп сущностей (люди, события, источники,
  архивы) — они идут отдельными проходами.
- Фронтенд: тип `AdminDivision` в `web/src/api.ts` и обращение к новому URL.

## 6. Мелочи (этап E)

- `PersonGender` константы: `Male/Female/Unknown` → `PersonGenderMale/Female/Unknown`
  (как в спеке S1+S2 §2).
- Тест, фиксирующий соответствие `models.Type` ↔ реестр таблиц ↔
  `search_index.entity_table` ↔ префикс `ID`.
- `List*`-тесты для всех 21 типа.
- Устаревшие абзацы: каскад `source_links`/`text_refs` в спеке S1+S2 §5,
  раздел «Открытые вопросы» в `docs/todo.md`, пометка о несоответствии
  словаря `internal/definitions/russia` (блок D `docs/todo.md`).

## 7. Порядок работ

Этапы — в [дорожной карте](../plans/2026-09-20-core-rw-roadmap.md): A (домен) →
B/C (хранилище и порт) → E (мелочи) → D (контракты и срез на делениях). Каждый
этап укладывается в одну короткую сессию и оставляет дерево зелёным
(`gofmt -l .`, `go build ./...`, `go vet ./...`, `go test ./...`).

## 8. Тесты

- `models`: календарь (разбор, вывод, сравнение через границы календарей
  1582/1918), формат `ID`, `Validate` по сущностям (таблично).
- `idgen`: формат, уникальность на серии, монотонность.
- `storage`/`sqlstore`: индексы существуют и используются
  (`EXPLAIN QUERY PLAN` для поиска по префиксу и выборки дочерних), `Delete*`
  (успех, `*InUseError`, чистка индекса и связок, отсутствие сирот),
  `InTx` (откат, атомарность двух сущностей), пагинация и `Access`,
  `Search`, `ErrNotFound`.
- `usecases`: create/update/delete на делениях с фейковым портом (генерация ID,
  валидация, циклы, `InUseError`).
- `transport`/`httpapi`/`mcp`: точные строки JSON, коды ответов ошибок.
