# Реализация: ядро чтения и записи

Итоговое описание прохода «подготовка ядра к интерфейсам наполнения и отображения
данных» — как он был фактически построен. Консолидирует дизайн
([data-model/core-read-write.md](../data-model/core-read-write.md)) и дорожную карту
с решениями по 16 этапам
([plans/2026-09-20-core-rw-roadmap.md](../plans/2026-09-20-core-rw-roadmap.md));
подробные TDD-планы каждого этапа остаются в `docs/plans/2026-09-2*-core-rw-s*.md`
как рабочий журнал. Предыдущий проход — нормализация хранилища
([normalization-s1s2.md](../data-model/normalization-s1s2.md)).

Весь проход выполнен на `main` без веток: каждый этап — план непосредственно перед
запуском, затем маленькие коммиты по слоям (`models` → `storage`/`sqlstore` →
`usecases` → `httpapi`/`mcp` → `app`/`web` → доки), каждый коммит проходит рубеж
`gofmt -l .` пусто, `go build ./...`, `go vet ./...`, `go test ./...`, smoke.

## 1. Домен (`internal/models`)

- **Календарь.** `FactDate.Calendar FactCalendar` (`gregorian`/`julian`/`unknown`,
  пусто ≡ `unknown`). `Compare` пересчитывает границы через юлианский день, если
  календари дат различаются и ни один не `unknown`; `before`/`after` не трогаются.
  `ParseFactDate`/`String` — суффиксы `ст. ст.`/`н. ст.`, разбор и вывод обратимы.
  `dates.calendar` хранится как есть, умолчание — `''`.
- **Идентификаторы.** Формат `ПРЕФИКС-ULID` (26 символов Crockford base32, без
  `I L O U`); таблица префиксов на 21 тип сущности в `models.Type.IDPrefix`
  (`I`/`F`/`S`/`R`/`N`/`O`/`E`/`C`/`RL`/`RS`/`SN`/`GN`/`PN`/`ES`/`TT`/`CH`/`PR`/`AD`/
  `AR`/`AN`/`DC` — `I F S R N O` совпадают с GEDCOM). `ParseID`/`ID.Validate(Type)` —
  чистые функции в `models`; генератор — `internal/idgen` (`crypto/rand` + время,
  монотонный в пределах миллисекунды, без внешних зависимостей). Порт ID не
  генерирует; явный ID от клиента (импорт) проходит `ID.Validate`.
- **Валидация.** `Validate() error` на каждой сущности и value-типе, ошибка —
  `*models.ValidationError{Entity, Field, Reason}` (одна ошибка на вызов; список
  полей не введён — не понадобился). Закрытые enum'ы (`PersonGender`, `NameGender`,
  `PersonNameType`, `RelationKind`, `SourceKind`, `AttachmentKind`, `Reliability`,
  `AnchorKind`, `FactPrecision`, `FactModifier`, `FactCalendar`, `AdminDivisionType`)
  принимают только константы; открытые (`EventType`, `NoteKind`, `RepositoryType`,
  `RelationType`, `ArchiveNodeType`) — непустое значение формата `[a-z][a-z0-9_-]*`.
  Проверки с обращением к хранилищу (существование ссылок, циклы по `parent_id`) —
  дело сценариев, не `models`.

## 2. Хранилище (`internal/storage`, `internal/store/sqlstore`)

- **Индексы.** `createFKIndexes` (`storage/indexes.go`) генерирует `CREATE INDEX IF
  NOT EXISTS` на каждую FK-колонку из `PRAGMA foreign_key_list` при каждом открытии
  БД — вручную ничего не перечисляется. `idx_search_term (term, entity_table,
  entity_id, field)` — покрывающий, под диапазонный префиксный поиск (`term >= ?
  AND term < ?`; `LIKE` с `ESCAPE` индекс не использует).
- **Чистка при удалении.** `search_index` и `source_links` — полиморфные, без FK на
  владельца; адаптер чистит их сам в `Save*`/`Delete*`, вместе с осиротевшими
  `text_refs`/`dates`/`anchors`.
- **DSN.** `foreign_keys`, `busy_timeout`, `journal_mode` — через `_pragma` в DSN
  (действуют на каждое соединение пула, не только на первое).

## 3. Порт `internal/store`

Общие типы — в `models` (листовой пакет, видят и сценарии, и обработчики):
`ErrNotFound`, `InUseError{Type, ID, Referrers []EntityRef}` (до 20 ссылающихся, без
повторов), `Access` (`AccessFull`/`AccessPublic`), `Page{Limit, Offset}` (0 → 50,
максимум 500), `Hit{Type, ID, Label, Field}`.

- Все методы порта принимают `ctx` первым аргументом; `Get*` возвращает
  `ErrNotFound` вместо `(nil, nil)`.
- `Delete<Entity>(ctx, id) error` для всех 21 сущностей — одна реализация на графе
  внешних ключей (`sqlstore/fkgraph.go`), не ручные списки по типам: удаляет
  сущность, дочерние строки, записи `search_index`/`source_links`, осиротевшие
  value-строки; RESTRICT-ссылки → `*InUseError`; `SET NULL`-ссылки (например
  `Attachment.DocumentID`) не блокируют и обнуляются; «мягкие» ссылки без FK
  (`TextRef.Ref`, якоря цитат) не чистятся и не блокируют — читатель терпит висячую
  ссылку.
- `InTx(ctx, func(Store) error) error` — атомарная запись нескольких сущностей;
  `Store` внутри функции — копия адаптера с `exec = *sql.Tx`; вложенный `InTx`
  использует ту же транзакцию (без savepoint); переданный `Store` непригоден после
  выхода из функции.
- `List*(ctx, Access, Page)` — пагинация везде; `AccessPublic` фильтрует строки с
  `private = 1` на уровне SQL (таблицы с флагом — из графа схемы); порядок —
  порядок сохранения; окно — после фильтра; короткое окно не гарантирует конец
  списка (конец — пустое окно). Пакетная загрузка (`IN (…)`, куски по 500) убирает
  N+1 для `Person` и `AdministrativeDivision`; остальные 19 видов читаются
  поштучно — расширяется по мере вертикальных срезов.
- `Search(ctx, query, Access, Page) ([]Hit, error)` — префиксный поиск по
  `idx_search_term`, регистронезависимый и нечувствительный к `ё/е`
  (`storage.Normalize`); одна сущность — один `Hit` (первое по алфавиту совпавшее
  поле).
- `ChildrenOfDivision(ctx, parent, Access, Page)` — прямые дети деления в порядке
  сохранения; нет родителя — `ErrNotFound`. Точечные методы для других сущностей
  (`EventsOfPerson`, …) вводятся вместе с их срезами.

Агрегаты остаются единственной моделью и для чтения, и для записи; `Save*` —
upsert с заменой дочерних строк (без изменений в этом проходе).

## 4. Вертикальный срез: административное деление

Образец полного контракта, который следующие срезы (люди, события, источники,
архивы) будут повторять.

**Сценарии** (`internal/usecases/`, по пакету на действие):
`list_divisions` (список, фильтр `kind`/`type`/`parent_id`), `search_divisions`
(поиск по началу названия, включая варианты), `get_division`, `create_division`
(генерирует `ID`, проверяет существование родителя, пишет в `InTx`),
`update_division` (полная замена, проверка цикла по `parent_id` обходом цепочки
родителей вверх), `delete_division` (`Delete*`, `*InUseError` пробрасывается).
`create`/`update` отвергают несуществующий или зацикленный `parent_id` —
`*ValidationError` поля `parent_id`; `get`/`delete` при неверном формате
id-аргумента — `*ValidationError` поля `id`.

**Контракты** — единый интерфейс `DivisionService` (шесть методов, дублируется в
`internal/httpapi/deps.go` и `internal/mcp/deps.go` по правилу «зависимости —
интерфейсом в своём пакете»), собирается фасадом `divisionService` в
`internal/app/app.go`:

| HTTP | MCP | Действие |
|---|---|---|
| `GET /api/admin-divisions` | `division_list` | список; параметры `kind`, `type`, `parent_id`, `limit`, `offset` |
| `GET /api/admin-divisions/search` | `division_search` | поиск по началу названия/варианта; параметры `q`, `limit`, `offset` |
| `GET /api/admin-divisions/{id}` | `division_get` | единица по id |
| `POST /api/admin-divisions` | `division_create` | создание, id генерирует сервер |
| `PUT /api/admin-divisions/{id}` | `division_update` | полная замена `name`/`type`/`parent_id` |
| `DELETE /api/admin-divisions/{id}` | `division_delete` | удаление, занятая — ошибка |

Коды ответов: `POST` → `201`, `GET`/`PUT` → `200`, `DELETE` → `204`; `400` —
синтаксически неверный параметр (`limit`/`offset` не число); `422` —
`*models.ValidationError` (тело `{error, field}`, включая неверный формат `id`/
`parent_id` в пути или запросе — не `404`); `404` — `ErrNotFound` (`id` корректного
формата, сущности нет, либо `parent_id` не существует); `409` — `*InUseError`
(тело `{error, referrers:[{type,id}…]}`, до 20). В MCP та же классификация ошибок
возвращается как `CallToolResult` с `IsError` и текстом `не удалось …: <err>`;
успех `get/create/update` — JSON контракта `AdminDivision`, `delete` — текст
`деление "<id>" удалено`.

Маршрут `/search` — литеральный, регистрируется до `{id}` (в Go 1.22 `ServeMux`
литерал побеждает wildcard; закреплено тестом). Поиск — префиксный по полю `name`,
куда индексируются `Name` и `Variants`; пустой `q` (после `TrimSpace`) — пустой
результат без обращения к хранилищу. `parent_id` в списке переключает источник
окон на `Store.ChildrenOfDivision` вместо `Store.ListAdministrativeDivisions`
(общий код обхода/фильтра/окна).

**DTO** (`internal/transport`): `AdminDivision{id, name, type, parent_id}`,
`AdminDivisionCreate`/`Update` — импортируются только обработчиками (`httpapi`,
`mcp`), `models` про них не знает.

**Веб** (`web/src`) — исторически, на момент этого прохода: вкладка «Населённые
пункты» — список от корня, строка поиска (`Input.Search` → `/search`), переход
к дочерним единицам (кнопка «дети» → `?parent_id=`) и обратно («к корню»).
Структура с вкладками впоследствии снесена (навигационная перестройка
подпроекта Surname — единая точка входа `/`, роут `/divisions`, хлебные
крошки; см. `docs/data-model/entity-write.md` §4, `docs/usage.md`). `web/dist`
не хранится в git (`web/.gitignore`) — собирается перед prod-запуском
(`npm run build`, `go:embed` в `internal/web`/`web/embed.go`).

Соглашение об именах для следующих срезов: MCP
`<entity>_list|search|get|create|update|delete`, HTTP `GET/POST
/api/<множественное>`, `GET/PUT/DELETE /api/<множественное>/{id}`, `GET
/api/<множественное>/search`.

Переименование предыдущего единственного контракта (S13): `settlement_list` →
`division_list`, `GET /api/settlements` → `GET /api/admin-divisions`, DTO
`Settlement` → `AdminDivision`; фильтр «только населённые пункты» стал параметром
(`kind=settlement`), а не именем контракта.

## 5. Встроенные данные (`internal/definitions/russia`)

Две исторические системы административного деления (Российская империя 1708–1917,
СССР) объявлены как `models.AdministrativeDivisionSystem{Name, Relations
[]AdminDivisionType}` с каноническими значениями `AdminDivisionType`
(`governorate`/`district`/`volost`) — раньше константы держали слова для показа
(`"губерния"`), что не проходило бы `AdminDivisionType.Valid()`. Русские названия —
только в комментариях к константам. Тест (`russia_test.go`) проверяет `Valid()` на
каждое значение `Relations`. Мёртвый тип `models.AdministrativeDivisionTypeRelation`
(без единого потребителя, включая сам пакет `russia`) удалён. Пакет по-прежнему не
подключён ни к одному сценарию или CLI-подкоманде — объявляет только структуру двух
систем деления; интеграция (сид реальных данных, импорт) не входила в проход.

## 6. Итог

Административное деление закрыто по вертикали: чтение (список, фильтры, поиск,
дочерние единицы) + полный CRUD + HTTP/MCP контракты + веб-интерфейс. Инфраструктура
порта (ctx, `ErrNotFound`, `Delete*`, `InTx`, `Page`/`Access`, пакетная загрузка,
`Search`) — общая для всех 21 типа сущности, не только делений, и готова под
следующие вертикальные срезы (люди, события, источники, архивы) без изменений.

Не входило в проход и остаётся открытым:
- `Access` во внешних контрактах — обработчики публичного API должны передавать
  режим доступа из запроса, а не жёстко использовать `AccessFull` в сценарии (сейчас
  безвредно: у делений нет флага приватности).
- Вертикальные срезы для людей, событий, источников, архивов — по образцу делений.
- Экспорт/публикация, оптимистичные блокировки, история правок.

## Журнал этапов

| Этап | Что сделано |
|---|---|
| S1 | Календарь в `FactDate`: поле `Calendar`, сравнение через юлианский день, суффиксы `ст. ст.`/`н. ст.` |
| S2 | Формат `ID` (`ПРЕФИКС-ULID`), таблица префиксов, `internal/idgen` |
| S3 | `Validate()`: основа (`ValidationError`, enum'ы) + `Person`/`Family`/`Relation`/`Residence`/словари |
| S4 | `Validate()`: деления, церкви, приходы, события, источники, архивы |
| S5 | FK-индексы из схемы (`createFKIndexes`), поисковый индекс |
| S6 | `ErrNotFound` вместо `(nil, nil)`, `ctx` первым аргументом во всём порту |
| S7 | `Delete*` на графе внешних ключей для всех 21 сущности, `*InUseError` |
| S8 | `InTx` — атомарная запись нескольких сущностей |
| S9 | `List*(ctx, Access, Page)`, пагинация и публичный/полный доступ |
| S10 | Пакетная загрузка (`IN`, до 500) — устранён N+1 для `Person`/`AdministrativeDivision` |
| S11 | `Search` (префиксный, по индексу) и `ChildrenOfDivision` |
| S12 | Переименования констант, DSN-прагмы, тесты соответствия реестра типов, устаревшие абзацы |
| S13 | Контракт чтения делений: `division_list`, `GET /api/admin-divisions`, DTO, веб |
| S14 | Сценарии записи: `create`/`update`/`delete_division`, генерация ID, проверка циклов |
| S15 | Контракты записи: HTTP `POST/PUT/DELETE/GET {id}`, MCP `division_*`, коды 404/409/422 |
| S16 | `division_search`, `parent_id` в списке, `GET /api/admin-divisions/search`, поиск в веб |

Подробности и TDD-код каждого этапа — `docs/plans/2026-09-2*-core-rw-s*.md`
(ссылки — в [roadmap](../plans/2026-09-20-core-rw-roadmap.md)).
