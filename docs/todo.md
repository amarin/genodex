# TODO: нормализация хранилища

План работ по переходу от плоской переходной схемы к нормализованной модели.
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
- [x] **`internal/entity/entity_type.go` согласован с `decisions.md`** (противоречие устранено в пользу решений):
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

- [~] Перейти от плоской таблицы `entity(type, id, data, search)` к нормализованным таблицам
      по моделям `docs/models/` (детальный дизайн — `data-model/normalization-s1s2.md`).
  - данные на этапе разработки можно потерять; однократная миграция клон-таблицей допустима.
- [~] Разнести поисковый индекс по доменным полям, убрать строку `search` общего вида.
      Решение: отдельная таблица `search_index` с lower-терминами из Go.
- [~] `db.go`: `schema_version` поднят до 0 (миграция не обратносовместима).

### B. Сущности и порт

- [~] Завести полные сущности по `docs/models/*`: `ArchiveNode`, `ArchiveDocument`, `Attachment`,
      `Relation`, `Residence`, `Family`, словари (`Surname`/`GivenName`/`Patronymic`/`Estate`/`Title`).
- [~] События: единый `Event` со значением `type` (`birth`/`marriage`/`death`/…) — без отдельных
      сущностей `Marriage`/`Burial` (решение #1).
- [~] Административное деление: единая рекурсивная `AdministrativeDivision` со значением `type`
      (`governorate`/`district`/`volost`/`gorod`/`selo`/`derevnya`/…) — решения #13, #17.
- [~] Удалить `Metadata` из всех сущностей и моделей; удалить сущность `Settlement` (решения #17, #18).
- [~] Упразднить `internal/entity`: домен — единственный внутренний пакет `internal/models` (без
      JSON-тегов), контракты — DTO в `internal/transport` (только `httpapi`/`mcp`).
- [~] Номинальные типы для enum-доменов и `ID` (решение #20).

### C. Публичные контракты

- [ ] MCP-тулы и `/api` под новые сущности (сейчас публичен только `settlement_list` /
      `/api/settlements`).
- [ ] Фронтенд (`web/src`) и его типы — под новые контракты.

### D. Встроенные данные

- [ ] `internal/definitions/russia` переведён на нормализованную модель административного деления.

## Открытые вопросы

- `settlement` оставлена отдельной сущностью (публичный контракт `settlement_list`).
  Решить: закрепить отдельной сущностью или сделать уровнем `AdministrativeDivision.type`.
- Словари (`surname`, `given_name`, `patronymic`, `estate`, `title`) — текстовые ссылки из
  `PersonName.TextRef` против отдельных строк в таблицах словарей (решение в `decisions.md` #?).
- `family` и связи `person ↔ person` (родители/супруги, `Relation.kind`) — порядок введения.

## Паспорт прохода

Проверки перед рубежом: `gofmt -l .` пусто, `go build ./...`, `go vet ./...`,
`go test ./...`, smoke `/api/health`, `/api/settlements`, `/`.