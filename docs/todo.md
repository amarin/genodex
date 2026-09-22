# TODO: нормализация хранилища

План работ по переходу от плоской переходной схемы к нормализованной модели (блоки A и B выполнены).
Источник истины по модели — [decisions.md](data-model/decisions.md);
модельные доки — [models.md](models.md) и каталог [models/](models/).

Статусы: `[x]` сделано, `[ ]` не начато, `[~]` в работе.

## Сделано (данный проход)

- [x] **Убран append-only журнал** из `internal/storage`:
  - удалены `journal.go`, `journal_test.go`, `util.go`, методы `Storage.LastSeq/SetAppliedSeq/AppliedSeq`,
    поля манифеста (`AppliedSeq`, `JournalFile`, `JournalSHA`), доки и CLI-подкоманды приведены к
    «снапшот + манифест». Бэкап/восстановление остаются штатными функциями обслуживания.
  - причина: журнал с fsync на каждую запись и полным сканированием на старте — ограничитель
    на 100M; normal SQLite (WAL) достаточно.
- [x] **`internal/entity/entity_type.go` согласован с `decisions.md`** (противоречие устранено в пользу решений; историческая запись прохода — пакет `internal/entity` затем упразднён, актуальный набор типов — `models.AllTypes()`):
  - удалены типы-константы и сущности `Governorate`, `District`, `Volost`, `Fund`, `Inventory`,
    `Case`, `Marriage`, `Burial` (файлы удалены);
  - добавлены константы типов `archive_node`, `archive_document`, `attachment` (без структур —
    по образцу `family`);
  - порт `internal/store/deps.go` и sqlstore почищены от методов этих типов;
  - итоговый набор типов (17): `person, surname, first_name, patronymic, estate, title, church,
    parish, administrative_division, settlement, event, source, family, archive, archive_node,
    archive_document, attachment`.
- [x] Доки `usage.md`, `architecture.md`, `development.md`, `AGENTS.md` очищены от журнала/плоской схемы.

## План дальнейшей нормализации

### A. Слой хранилища (`internal/storage`, `internal/store/sqlstore`)

- [x] Перейти от плоской таблицы `entity(type, id, data, search)` к нормализованным таблицам
      по моделям `docs/models/` (детальный дизайн — `data-model/normalization-s1s2.md`).
  - данные на этапе разработки можно потерять; однократная миграция клон-таблицей допустима.
- [x] Разнести поисковый индекс по доменным полям, убрать строку `search` общего вида.
      Решение: отдельная таблица `search_index` с lower-терминами из Go.
- [x] `db.go`: `schema_version` поднят до 0 (миграция не обратносовместима).

### B. Сущности и порт

- [x] Завести полные сущности по `docs/models/*`: `ArchiveNode`, `ArchiveDocument`, `Attachment`,
      `Relation`, `Residence`, `Family`, словари (`Surname`/`GivenName`/`Patronymic`/`Estate`/`Title`).
- [x] События: единый `Event` со значением `type` (`birth`/`marriage`/`death`/…) — без отдельных
      сущностей `Marriage`/`Burial` (решение #1).
- [x] Административное деление: единая рекурсивная `AdministrativeDivision` со значением `type`
      (`governorate`/`district`/`volost`/`gorod`/`selo`/`derevnya`/…) — решения #13, #17.
- [x] Удалить `Metadata` из всех сущностей и моделей; удалить сущность `Settlement` (решения #17, #18).
- [x] Упразднить `internal/entity`: домен — единственный внутренний пакет `internal/models` (без
      JSON-тегов), контракты — DTO в `internal/transport` (только `httpapi`/`mcp`).
- [x] Номинальные типы для enum-доменов и `ID` (решение #20).

### C. Публичные контракты

- [ ] MCP-тулы и `/api` под новые сущности (сейчас публичен только список делений:
      `division_list` / `/api/admin-divisions`, S13).
- [ ] Фронтенд (`web/src`) и его типы — под новые контракты.

### D. Встроенные данные

- [ ] `internal/definitions/russia` переведён на нормализованную модель административного деления.
  Сейчас словарь временный: константы — слова для показа (`"губерния"`), а не
  канонические значения `models.AdminDivisionType` (`governorate`); пометка есть в
  комментариях файлов пакета.

## Следующий проход: ядро чтения и записи

Дизайн — [data-model/core-read-write.md](data-model/core-read-write.md), этапы
(S1–S16: календарь, формат id, валидация, индексы, порт с удалением, транзакциями,
пагинацией и поиском, мелочи, контракты и срез на делениях) —
[plans/2026-09-20-core-rw-roadmap.md](plans/2026-09-20-core-rw-roadmap.md). Выполнены S1–S15 (доменные правила, индексы, порт: `ctx`, `ErrNotFound`, `Delete*`, `InTx`, `Page`/`Access`, пакетная загрузка, `Search`, мелочи, контракт делений на чтение, сценарии и контракты записи делений); остался S16 — поиск и дочерние деления в контрактах.

## Открытые вопросы

Решены (для истории): `settlement` — вид `AdministrativeDivision.type` (решение
#17; публичный контракт переименован в `division_list` на этапе S13);
словари — отдельные таблицы (`surnames` и др.); `Family` и `Relation` введены.

- Форма ошибки валидации: сейчас первая ошибка (`*ValidationError`) — решено на
  S15 (обработчики читают её по «одно полю на запрос»); списка полей
  (`ValidationErrors`) пока не ввели, может понадобиться для форм фронтенда.

- `Access` во внешних контрактах: обработчики публичного API должны передавать
  режим доступа из запроса, а не выбирать его в сценарии (S13–S16).
- Консолидация «план + спецификация → `docs/implementation/`» и запись в
  `CHANGELOG.md` по программе «ядро чтения и записи» — одним проходом после S16.

## Паспорт прохода

Проверки перед рубежом: `gofmt -l .` пусто, `go build ./...`, `go vet ./...`,
`go test ./...`, smoke `/api/health`, `/api/admin-divisions`, `/`.