# Ядро чтения и записи — дорожная карта этапов

**Spec:** `docs/data-model/core-read-write.md` (дизайн; §-ссылки ниже — на него).
**Предыдущий проход:** `docs/plans/2026-09-20-normalization-s1s2.md` (выполнен).

Дорожная карта разбивает проход на 16 этапов. Каждый этап:

- укладывается в одну короткую сессию (порядка 3–8 задач);
- даёт наблюдаемый инкремент и оставляет дерево зелёным
  (`gofmt -l .` пусто, `go build ./...`, `go vet ./...`, `go test ./...`);
- получает подробный TDD-план с готовым кодом **непосредственно перед запуском**
  (файл `docs/plans/2026-09-20-core-rw-sNN-<name>.md`): следующие этапы опираются
  на реальные имена и сигнатуры после предыдущих, поэтому полный код всех этапов
  заранее устарел бы. Подробный план этапа 1 уже написан.

Порядок: **A (домен) → B/C (хранилище и порт) → E (мелочи) → D (контракты и срез на
делениях)**. E стоит после C, потому что тесты `List*` для 21 типа зависят от
новой сигнатуры `List(ctx, Access, Page)`.

## Обзор

| # | Часть | Этап | Инкремент | Зависит от |
|---|---|---|---|---|
| S1 | A | Календарь в `FactDate` | Даты хранят и сравнивают календарь, `ст. ст.`/`н. ст.` | — |
| S2 | A | Формат `ID` и генератор | `models.ID.Validate`, `internal/idgen` | — |
| S3 | A | Валидация: основа и люди | `Validate` для value-типов, enum'ов, Person…Family, словарей | S1, S2 |
| S4 | A | Валидация: остальные сущности | `Validate` для делений, церквей, событий, источников, архивов | S3 |
| S5 | B | Индексы | FK-индексы генерируются из схемы, проверены `EXPLAIN` | — |
| S6 | C | `ErrNotFound` и `ctx` в порту | `Get*` → `ErrNotFound`, `ctx` во всех методах | — |
| S7 | C | `Delete*` | Удаление всех 21 сущностей, `*InUseError`, чистка | S5, S6 |
| S8 | C | `InTx` | Атомарная запись нескольких сущностей | S6 |
| S9 | C | `Page` и `Access` | `List*(ctx, Access, Page)` для всех сущностей | S6 |
| S10 | C | Пакетная загрузка | Нет N+1 в `List` для `Person` и `AdministrativeDivision` | S9 |
| S11 | C | `Search` и `ChildrenOfDivision` | Поиск по префиксу через индекс, дочерние деления | S5, S9 |
| S12 | E | Мелочи | Переименование, тесты соответствия и `List*`, DSN, устаревшие абзацы | S1–S11 |
| S13 | D | Контракт делений (чтение) | `division_list`, `/api/admin-divisions`, DTO, фронтенд | S9, S12 |
| S14 | D | Сценарии записи делений | create/update/delete с ID, валидацией, циклами | S2, S4, S7, S8, S13 |
| S15 | D | Контракты записи делений | MCP-тулы и HTTP-роуты, коды ошибок | S14 |
| S16 | D | Поиск и дочерние деления в контрактах | `division_search`, `?parent_id=`, поиск во фронтенде | S11, S15 |

## Часть A — домен (`internal/models`, `internal/idgen`)

### S1. Календарь в `FactDate`
- **Файлы:** `models/fact_date.go`, `models/fact_calendar.go` (без изменений), тест
  `models/fact_date_test.go`; `storage/schema.go` (умолчание колонки `calendar`),
  `store/sqlstore/helpers.go` (`insertDate`/`loadDate`), тест `sqlstore_test.go`.
- **Интерфейсы:** `FactDate.Calendar FactCalendar`; `Compare` с пересчётом
  через юлианский день; суффиксы в `ParseFactDate`/`String`.
- **Приёмка:** `1917-10-25 ст. ст.` и `1917-11-07 н. ст.` сравниваются как один
  день; разбор и вывод суффиксов обратимы; календарь проходит round-trip через
  БД; существующие тесты `FactDate` не менялись.
- **План:** `2026-09-20-core-rw-s01-calendar.md`.

### S2. Формат `ID` и генератор
- **Файлы:** `models/id.go` (+`id_test.go`), `models/id_prefix.go` (таблица префиксов),
  `internal/idgen/idgen.go` (+тест).
- **Интерфейсы:** `Type.IDPrefix() string`, `ParseID(ID) (Type, error)`,
  `ID.Validate(Type) error`, `TypeByIDPrefix(string) (Type, bool)`,
  `AllTypes() []Type`, `BuildID(Type, string) (ID, error)`, `ErrInvalidID`;
  `idgen.Generator` c
  `New(Type) models.ID` (ULID, монотонный в пределах миллисекунды).
- **Приёмка:** таблица 21 тип ↔ префикс без коллизий; `Validate` отвергает
  чужой префикс, неверную длину и алфавит; `idgen` — 10 000 значений уникальны и
  упорядочены по времени; формат совпадает с `identifiers.md`.

### S3. Валидация: основа и люди
- **Файлы:** `models/validation.go` (`ValidationError`, `fieldErr`, `within`,
  `indexed`, `finish`, `validOpenEnum`), `models/enum_valid.go` (`Valid()`
  закрытых enum'ов), `models/fact_date_validate.go`, `text_ref_validate.go`,
  `source_link_validate.go`, `person_validate.go`, `person_name_validate.go`,
  `relation_validate.go`, `residence_validate.go`, `family_validate.go`,
  `dictionary_validate.go`; тесты таблично.
- **Интерфейсы:** `(*ValidationError).Error()`, `Validate() error` на
  перечисленных сущностях, `Valid() bool` у закрытых enum'ов,
  `validOpenEnum(string) bool` для открытых.
- **Приёмка:** правила §2.3 покрыты (в том числе `Relation`: `associate` ↔
  `RelType`, `PersonA != PersonB`; `since <= until`).

### S4. Валидация: остальные сущности
- **Файлы:** `Validate` для `AdministrativeDivision`, `Church`, `Parish`,
  `Event` (+участники), `Source`, `Citation`, `Note`, `Repository`, `Archive`,
  `ArchiveNode`, `ArchiveDocument`, `Attachment`; тесты.
- **Приёмка:** для каждой сущности: валидный экземпляр проходит, каждый
  нарушенный инвариант даёт `*ValidationError` с полем.
- **Решения этапа:** `ArchiveNodeType` — открытый enum; `Archive.System` — только
  текст; `PlaceRef` проверяется как `TextRef` с типами
  `administrative_division`/`church`/`parish`; `NamedPeriod` — строки формата
  `ParseFactDate`; план — `2026-09-21-core-rw-s04-validation.md`.
- **Предпосылки из S3:**
  - helper `validateOptionalID` для `*ID` и необязательных `ID` (`ParentID`,
    `DocumentID`, `RepositoryID`);
  - `PlaceRef` — отдельная структура с тремя допустимыми типами цели
    (`administrative_division | church | parish`), а `validateAs` принимает один
    тип: нужно обобщить (`...Type`) или свести `PlaceRef` к `TextRef`;
  - `NamedPeriod.Since`/`Until` — строки, `validatePeriod` к ним неприменим;
  - `ArchiveNodeType` не имеет констант и `Valid()` — нужно решение
    «закрытый/открытый»;
  - `*TextRef` (`Church.Parish`, `ArchiveNode.Parish`) — нужен вариант helper'а.

## Часть B/C — хранилище и порт

### S5. Индексы
- **Файлы:** `storage/schema.go` (функция `createIndexes` по
  `PRAGMA foreign_key_list`), тест `storage/db_test.go`.
- **Приёмка:** каждая FK-колонка имеет индекс (тест обходит `sqlite_master`);
  `EXPLAIN QUERY PLAN` для выборки дочерних строк по владельцу использует индекс;
  повторное открытие БД идемпотентно.

### S6. `ErrNotFound` и `ctx` в порту
- **Файлы:** `models/errors.go` (`ErrNotFound`), `store/deps.go`, `sqlstore/*`,
  сгенерированный мок, тесты, `usecases/list_settlements`.
- **Интерфейсы:** все методы порта принимают `ctx context.Context` первым
  аргументом; `Get*` возвращает `ErrNotFound`.
- **Приёмка:** тесты `Get` для отсутствующего id проверяют `errors.Is(err, ErrNotFound)`;
  сборка и тесты всего репозитория зелёные.

### S7. `Delete*`
- **Файлы:** `models/errors.go` (`InUseError`, `EntityRef`), `store/deps.go`,
  `sqlstore/delete.go` (общая реализация), тесты.
- **Интерфейсы:** `Delete<Entity>(ctx, id) error` для 21 сущности.
- **Приёмка:** удаляет сущность, дочерние строки, записи `search_index` и
  `source_links`, осиротевшие значения; ссылающиеся сущности → `*InUseError` со
  списком (до 20); удаление отсутствующей → `ErrNotFound`; счётчики таблиц
  (`text_refs`, `dates`, `anchors`) не растут.

### S8. `InTx`
- **Файлы:** `store/deps.go`, `sqlstore/sqlstore.go`, `sqlstore/tx.go`,
  рефакторинг методов на общий исполнитель, тесты.
- **Интерфейсы:** `InTx(ctx, func(Store) error) error`.
- **Приёмка:** `Citation` + `Person` записываются атомарно; ошибка внутри
  откатывает обе; вложенный `InTx` использует ту же транзакцию.

### S9. `Page` и `Access`
- **Файлы:** `models/query.go` (`Page`, `Access`), `store/deps.go`,
  `sqlstore/*` (общий `listEntities`), тесты, `usecases`.
- **Интерфейсы:** `List*(ctx, Access, Page)`; `Page.Limit` 0 → 50, максимум 500.
- **Приёмка:** страницы не пересекаются и покрывают набор; `AccessPublic`
  скрывает `private` (на сущностях с флагом), `AccessFull` — нет.

### S10. Пакетная загрузка
- **Файлы:** `sqlstore/helpers.go`, `person.go`, `places.go`, тесты.
- **Приёмка:** число SQL-запросов `ListPeople`/`ListAdministrativeDivisions` не
  зависит от числа строк (тест считает запросы через обёртку над `*sql.DB`);
  результаты совпадают с поштучной загрузкой.

### S11. `Search` и `ChildrenOfDivision`
- **Файлы:** `models/query.go` (`Hit`), `store/deps.go`, `sqlstore/search.go`,
  `storage/schema.go` (`idx_search_term`), тесты.
- **Интерфейсы:** `Search(ctx, query, Access, Page) ([]Hit, error)`;
  `ChildrenOfDivision(ctx, parent, Access, Page)`.
- **Приёмка:** префиксный поиск нечувствителен к регистру и `ё/е`; использует
  индекс (`EXPLAIN`); `Hit.Label` и `Field` осмысленны; порядок стабилен.

## Часть E — мелочи

### S12. Мелочи
- `PersonGender*` переименование (только тесты).
- Тест соответствия `models.Type` ↔ реестр таблиц ↔ `search_index.entity_table` ↔
  префикс `ID`.
- `List*`-тесты для всех 21 типа.
- DSN `_pragma` для `foreign_keys` и `busy_timeout`.
- Устаревшие абзацы: спека S1+S2 §5 (каскад), `docs/todo.md` (открытые вопросы,
  новый блок), пометка о словаре `definitions/russia`,
  `docs/superpowers/specs/2026-09-19-storage-design.md` (ссылка на отменённое
  правило id «транслит + дисамбигуатор»).
- **Приёмка:** дерево зелёное; в `docs/` нет утверждений, противоречащих коду.

## Часть D — контракты и срез на делениях

### S13. Контракт делений (чтение)
- **Файлы:** `usecases/list_divisions` (переименование пакета сценария),
  `transport/admin_division.go` (DTO `AdminDivision` с `parent_id`),
  `httpapi`/`mcp` (`division_list`, `GET /api/admin-divisions`, параметры
  `kind`, `type`, `limit`, `offset`), `web/src/api.ts`, `web/src/App.tsx`,
  `docs/usage.md`.
- **Приёмка:** строки JSON зафиксированы тестами; старых имён нет в коде и
  документации; вкладка фронтенда работает на новом URL.

### S14. Сценарии записи делений
- **Файлы:** `usecases/create_division`, `update_division`, `delete_division`
  (+`get_division`), интерфейс `IDGenerator`, тесты на фейковом порту.
- **Приёмка:** `create` генерирует `AD-…`, вызывает `Validate`, проверяет
  существование родителя и отсутствие циклов, сохраняет в `InTx`; `update` не
  создаёт цикл; `delete` пробрасывает `*InUseError`.

### S15. Контракты записи делений
- **Файлы:** `transport` (запросы `AdminDivisionCreate`/`Update`, разбор
  ошибок), `httpapi` (`POST`, `PUT`, `DELETE`, `GET /{id}`), `mcp`
  (`division_get|create|update|delete`), тесты.
- **Приёмка:** коды `404`/`409`/`422`; тело `409` содержит список ссылающихся;
  MCP-тулы возвращают структурированную ошибку.

### S16. Поиск и дочерние деления в контрактах
- **Файлы:** `usecases/search_divisions`, `list_divisions` (параметр `parent_id`),
  `httpapi`/`mcp` (`division_search`, `?parent_id=`), `web` (строка поиска и
  переход к дочерним), тесты.
- **Приёмка:** поиск по префиксу и по варианту названия; дочерние единицы
  выбираются по индексу; фронтенд показывает результаты поиска.

## Что вне этого прохода
- Срезы для людей, событий, источников и архивов (по образцу делений).
- Экспорт и публикация (`AccessPublic` сейчас только в порту).
- Оптимистичные блокировки и история правок.
- Перенос `internal/definitions/russia` на нормализованную модель (блок D в
  `docs/todo.md`).
