# Запуск и эндпоинты

## Требования

- Go 1.26+ (система сборки бэкенда и `go:embed`);
- Node.js + npm (только для сборки фронтенда).

## Сборка

```bash
# 1. Собрать фронтенд (нужен только если web/dist отсутствует или устарел)
cd web && npm install && npm run build && cd ..

# 2. Собрать сервер
go build ./...
```

`go build` требует, чтобы `web/dist` существовал — там лежит `index.html`, который встраивается через `//go:embed all:dist`.

## Запуск

Бинарь `genodex` собирается из `cmd/genodex`. По умолчанию работает подкоманда `serve` (совместимость со старым запуском):

```bash
# prod: фронтенд из бинарника (по умолчанию)
go run ./cmd/genodex -p 9000

# dev: фронтенд с диска web/dist, без пересборки Go после правок фронта
go run ./cmd/genodex -p 9000 -web dev
```

### Данные

Каталог данных, где лежит `db/genodex.db`:

```bash
genodex --data /path/to/data serve        # или после подкоманды
genodex serve --data /path/to/data
export GENODEX_DATA=/path/to/data         # env-переменная как дефолт
```

По умолчанию `--data` = `.data` (подкаталог текущего каталога).

### Подкоманды

```bash
genodex serve [--data DIR] [-p 9000] [-web prod|dev] [--trust-proxy]   # сервер (по умолчанию)
genodex backup [--data DIR] [--to DIR]                 # бэкап: бандл «снапшот+манифест»
genodex restore --from DIR --to DIR [--force]          # восстановление из бандла
genodex verify [--data DIR] [--backup DIR]             # проверка целостности БД/бандла
```

- `backup` по умолчанию пишет в `<data>/backups/`.
- Если каталог данных и каталог бэкапа на одном устройстве, `backup` печатает warning в stderr (бэкап не защитит от отказа диска).
- `restore` требует пустой целевой каталог; `--force` разрешает запись поверх существующей БД (удаляет `db/genodex.db`).
- `verify` без `--backup` проверяет базу; с `--backup` сверяет также манифест бандла. При несоответствии — ошибка и ненулевой код возврата.

Флаги:

| Флаг | Значение по умолчанию | Назначение |
|------|----------------------|-----------|
| `-p` | `9000` | Порт HTTP-сервера (для `serve`) |
| `-web` | `prod` | Режим веб-ассетов: `prod` (встроены в бинарник) или `dev` (с диска `web/dist`) |
| `-trust-proxy` | `false` | Доверять `X-Forwarded-Proto` от реверс-прокси для `Secure`-флага cookie (включать только за TLS-терминирующим прокси, которому доверяете) |
| `--data` | `.data` или `GENODEX_DATA` | Каталог данных |
| `--to` | `<data>/backups` | Каталог бэкапа (для `backup`) / целевой каталог (для `restore`) |
| `--from` | — | Каталог бандла (для `restore`, обязателен) |
| `--backup` | — | Каталог бандла для сверки (для `verify`, опционально) |
| `--force` | `false` | Для `restore`: разрешить запись поверх существующей БД |

`-trust-proxy` подразумевает, что перед сервисом стоит доверенный
TLS-терминирующий реверс-прокси — подробности и риски см.
`docs/data-model/auth.md` §4.

Сервер останавливается по `Ctrl+C` (graceful shutdown, до 5 с).

## Эндпоинты

| Путь | Назначение |
|------|-----------|
| `/mcp` | MCP-сервер, транспорт Streamable HTTP. Для AI-ассистентов и MCP-клиентов. |
| `/api/health` | Проверка живости: `{"status":"ok"}`. |
| `/api/admin-divisions` | Единицы административного деления (JSON): `GET` — список `[{"id", "name", "type", "parent_id", "sources"}]` (параметры, включая `parent_id`, — ниже); `POST` — создание; `GET/PUT/DELETE /api/admin-divisions/{id}` — чтение/изменение/удаление единицы (единица, ссылающаяся на приватную `Citation` среди источников, — 404, как отсутствующая; у `AdministrativeDivision` нет собственного `private` — это единственная проверка); `GET /api/admin-divisions/search` — поиск по началу названия. |
| `/api/admin-division-types` | Справочник типов единиц административного деления (JSON, `GET`): `[{"type", "label", "rank", "settlement", "children"}]` — ранг (`null` у `other`), признак населённого пункта, допустимые дочерние типы. `POST`/`PUT /api/admin-divisions*` с недопустимой вложенностью — `422` по полю `type` (или `parent_id` при переносе). |
| `/api/surnames` | Словарные записи фамилий (JSON): `GET` — список `[{"id", "canonical", "variants", "items", "notes"}]`; `POST` — создание; `GET/PUT/DELETE /api/surnames/{id}` — чтение/изменение/удаление записи; `GET /api/surnames/search?q=` — поиск по началу канонической формы (включая варианты). |
| `/api/patronymics` | Словарные записи отчеств (JSON): `GET` — список `[{"id", "canonical", "variants", "items", "notes"}]`; `POST` — создание; `GET/PUT/DELETE /api/patronymics/{id}` — чтение/изменение/удаление записи; `GET /api/patronymics/search?q=` — поиск по началу канонической формы (включая варианты). |
| `/api/estates` | Словарные записи сословий (JSON): `GET` — список `[{"id", "canonical", "variants", "items", "notes"}]`; `POST` — создание; `GET/PUT/DELETE /api/estates/{id}` — чтение/изменение/удаление записи; `GET /api/estates/search?q=` — поиск по началу канонической формы (включая варианты). |
| `/api/titles` | Словарные записи титулов (JSON): `GET` — список `[{"id", "canonical", "variants", "items", "notes"}]`; `POST` — создание; `GET/PUT/DELETE /api/titles/{id}` — чтение/изменение/удаление записи; `GET /api/titles/search?q=` — поиск по началу канонической формы (включая варианты). |
| `/api/given-names` | Словарные записи имён (JSON): `GET` — список `[{"id", "canonical", "gender", "variants", "items", "notes"}]`; `POST` — создание; `GET/PUT/DELETE /api/given-names/{id}` — чтение/изменение/удаление записи; `GET /api/given-names/search?q=` — поиск по началу канонической формы (включая варианты). |
| `/api/repositories` | Хранилища-контейнеры источников (JSON): `GET` — список `[{"id", "name", "type", "address", "urls", "notes", "sources", "private"}]`; `POST` — создание; `GET/PUT/DELETE /api/repositories/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404, как отсутствующая; то же для записи, ссылающейся на приватную `Citation` среди источников); `GET /api/repositories/search?q=` — поиск по началу названия. |
| `/api/churches` | Церкви (JSON): `GET` — список `[{"id", "name", "parish", "settlements", "variants", "notes", "sources"}]`; `POST` — создание; `GET/PUT/DELETE /api/churches/{id}` — чтение/изменение/удаление записи (запись, ссылающаяся на приватную `Citation` среди источников, — 404, как отсутствующая; у `Church` нет собственного `private` — это единственная проверка); `GET /api/churches/search?q=` — поиск по началу названия. `parish` — одиночная необязательная ссылка (`{text, ref?, type?}`, v1-форма редактирует только text). |
| `/api/parishes` | Приходы (JSON): `GET` — список `[{"id", "name", "church", "settlements", "since", "until", "notes", "sources"}]`; `POST` — создание; `GET/PUT/DELETE /api/parishes/{id}` — чтение/изменение/удаление записи (запись, ссылающаяся на приватную `Citation` среди источников, — 404, как отсутствующая; у `Parish` нет собственного `private` — это единственная проверка); `GET /api/parishes/search?q=` — поиск по началу названия. `since`/`until` — структурированная дата (`FactDate`, см. `web/src/FactDateEditor.tsx`). |
| `/api/archives` | Архивы (JSON): `GET` — список `[{"id", "name", "system", "repository_id", "notes", "sources", "private"}]`; `POST` — создание; `GET/PUT/DELETE /api/archives/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404; то же для записи, ссылающейся на приватную `Citation` среди источников); `GET /api/archives/search?q=` — поиск по началу названия. `repository_id` — строгая ссылка на `/api/repositories`: несуществующий id при создании/изменении — 422 на поле `repository_id`. |
| `/api/archive-nodes` | Узлы архивного дерева (JSON): `GET` — список `[{"id", "type", "archive_id", "parent_id", "label", "name", "since", "until", "parish", "settlements", "notes", "sources", "private"}]`, **`archive_id` обязателен** (у узла нет смысла вне архива — отсутствующий или пустой параметр — 400), `parent_id` необязателен (отсутствует — корень дерева внутри архива, иначе — только прямые дети этого узла); `POST` — создание; `GET/PUT/DELETE /api/archive-nodes/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404; то же для записи, ссылающейся на приватную `Citation` среди источников); `GET /api/archive-nodes/search?q=` — поиск по началу метки/названия, **без** сужения по `archive_id` (глобальный поиск по всем архивам). `archive_id` — строгая ссылка на `/api/archives`: несуществующий id — 422 на поле `archive_id`. `parent_id` — необязательная строгая self-ref ссылка на другой узел: несуществующий id — 422 на поле `parent_id`; узел, принадлежащий ДРУГОМУ архиву — тоже 422 на поле `parent_id` (инвариант «родитель из того же архива, что и сам узел»); при `PUT`, если новый `parent_id` образует цикл (прямой или через цепочку), — тоже 422 на поле `parent_id`. |
| `/api/archive-documents` | Документы внутри единиц учёта (JSON): `GET` — список `[{"id", "unit_id", "title", "kind", "since", "until", "parish", "settlements", "notes", "sources", "private"}]` (плоский список, без обязательных фильтров); `POST` — создание; `GET/PUT/DELETE /api/archive-documents/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404; то же для записи, ссылающейся на приватную `Citation` среди источников); `GET /api/archive-documents/search?q=` — поиск по началу названия. `unit_id` — обязательная строгая ссылка на `/api/archive-nodes` (единицу учёта): несуществующий id — 422 на поле `unit_id`. |
| `/api/families` | Роды/линии (JSON): `GET` — список `[{"id", "name", "members", "notes", "sources", "private"}]`; `POST` — создание; `GET/PUT/DELETE /api/families/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404, как отсутствующая; то же для записи, ссылающейся на приватную `Citation` среди источников); `GET /api/families/search?q=` — поиск по началу названия. `members` — мягкая ссылка на `Person` (`TypePerson`, полный CRUD — `/api/people`, ниже): `TextRef.Ref`, если задан, только проверяется по формату, существование не проверяется — тот же принцип, что и у любого другого списка `TextRef` в программе. |
| `/api/people` | Персоны (JSON) — ядро графа генеалогии (подпроект 8): `GET` — список `[{"id", "gender", "names", "estates", "titles", "nicknames", "notes", "sources", "private"}]`; `POST` — создание; `GET/PUT/DELETE /api/people/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404, как отсутствующая; то же для записи, ссылающейся на приватную `Citation` среди источников); `GET /api/people/search?q=` — ищет по началу фамилии, имени или отчества из ЛЮБОГО элемента `names` (не только основного) — все имена персоны индексируются под единым полем `name`; `estates`/`titles`/`nicknames`/`notes` поиском не охватываются. `gender` — необязательный `male`/`female`/`unknown`. `names` — список вложенных объектов `{type?, surname, given, patronymic, prefix?, suffix?, since?, until?}`: `type` — `main`/`birth`/`married`/`changed`/`pseudonym` (пусто — не указан); `surname`/`given`/`patronymic` — обычные `TextRef` (мягкая ссылка на `Surname`/`GivenName`/`Patronymic`, существование НЕ проверяется — тот же принцип, что и у любого другого `TextRef` в программе), хотя бы одна часть должна быть заполнена; `since`/`until` — структурированная дата (`FactDate`). `estates`/`titles` — мягкие `TextRef` на `Estate`/`Title`; `nicknames`/`notes` — обычные нетипизированные `TextRef`. Единственная строго проверяемая ссылка — `sources[i].citation_id` (как и везде): несуществующая цитата — 422 на поле `sources[i].citation_id`. |
| `/api/relations` | Рёбра графа родства между двумя персонами (JSON, подпроект 9): `GET` — список `[{"id", "kind", "rel_type", "person_a", "person_b", "since", "until", "sources", "notes", "private"}]`, необязательный `?person_id=` — только рёбра, где эта персона `person_a` ИЛИ `person_b`; `POST` — создание; `GET/PUT/DELETE /api/relations/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404; запись, ссылающаяся на приватную `Person` через `person_a`/`person_b`, или на приватную `Citation` среди источников, скрывается так же, как и список — docs/data-model/entity-write.md §3.1). **Без `/api/relations/search`** — у `Relation` нет собственных поисковых полей (индекс намеренно пуст), `?person_id=` — более полезная замена. `person_a`/`person_b` — ДВЕ строгие ссылки на `Person` (первая пара строгих ссылок на один и тот же тип в программе): несуществующая — 422 на поле `person_a`/`person_b` соответственно; должны отличаться друг от друга (проверка модели). `kind` — закрытый набор `blood`/`marriage`/`adoption`/`associate`; `rel_type` — обязателен и только при `kind=associate`, иначе должен отсутствовать (открытый набор). |
| `/api/residences` | Проживания персоны в месте (JSON, подпроект 9): `GET` — список `[{"id", "person_id", "place_id", "since", "until", "sources", "note", "private"}]`, необязательные `?person_id=`/`?place_id=` (пересекаются, если оба заданы); `POST` — создание; `GET/PUT/DELETE /api/residences/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404; запись, ссылающаяся на приватную `Person` через `person_id`, или на приватную `Citation` среди источников, скрывается так же). **Без `/api/residences/search`** (см. `/api/relations` — тот же принцип). `person_id` — строгая ссылка на `Person`, `place_id` — строгая ссылка именно на `AdministrativeDivision` (не любой `PlaceRef`): несуществующая — 422 на поле `person_id`/`place_id`. `note` — ОДНА строка (не список `TextRef`, в отличие от `notes` у большинства сущностей). |
| `/api/events` | События жизненного факта (JSON, подпроект 9): `GET` — список `[{"id", "type", "date", "place", "participants", "sources", "notes", "private"}]`, необязательный `?person_id=` — только события, где эта персона участвует (любой элемент `participants`); `POST` — создание; `GET/PUT/DELETE /api/events/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404; запись, ссылающаяся на приватную `Person` через `participants[i].person_id`, или на приватную `Citation` среди источников, скрывается так же — тот же принцип и в `search`); `GET /api/events/search?q=` — ищет ТОЛЬКО по началу текста `place` (единственное индексируемое поле события — `type`/`date`/`participants` поиском не охвачены). `type` — открытый набор (`birth`/`death`/`marriage`/`burial`/`confession`/`census` и др.). `place` — необязательная МЯГКАЯ ссылка (`PlaceRef`: текст или ссылка, ограниченная типом `administrative_division`/`church`/`parish`) — существование НИКОГДА не проверяется, только формат/тип. `participants` — список `{person_id, role, note?}`: `person_id` — СТРОГАЯ ссылка на `Person` (как и `sources[i].citation_id`, существование проверяется на элемент массива): несуществующая — 422 на поле `participants[i].person_id` (по индексу элемента); `role` обязательна и непуста. |
| `/api/notes` | Заметки (markdown-текст с иерархией «книга → главы», JSON): `GET` — список `[{"id", "kind", "title", "text", "parent_id", "sources", "private"}]`; `POST` — создание; `GET/PUT/DELETE /api/notes/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404, как отсутствующая; то же для записи, ссылающейся на приватную `Citation` среди источников); `GET /api/notes/search?q=` — поиск по началу заголовка. `parent_id` — строгая self-ref ссылка на другую заметку: несуществующий id — 422 на поле `parent_id`; при `PUT`, если новый `parent_id` образует цикл (прямой или через цепочку), — тоже 422 на поле `parent_id`. |
| `/api/attachments` | Файловые вложения (JSON): `GET` — список `[{"id", "kind", "uri", "filename", "mime", "page", "node_id", "document_id", "note", "private"}]`; `POST` — создание; `GET/PUT/DELETE /api/attachments/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404); `GET /api/attachments/search?q=` — поиск по началу имени файла или URI. `node_id` — обязательная строгая ссылка на архивный узел (`ArchiveNode`, см. `/api/archive-nodes` выше): несуществующий id — 422 на поле `node_id`. `document_id` — необязательная мягкая ссылка на архивный документ (`ArchiveDocument`, см. `/api/archive-documents` выше, `ON DELETE SET NULL` в схеме); при создании/изменении, если задан, существование тоже проверяется — 422 на поле `document_id`. |
| `/api/sources` | Источники доказательств (JSON): `GET` — список `[{"id", "kind", "title", "author", "date", "reliability", "repository_id", "notes", "private"}]`; `POST` — создание; `GET/PUT/DELETE /api/sources/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404); `GET /api/sources/search?q=` — поиск по началу названия или автора. `date` — структурированная дата (`FactDate`). `repository_id` — мягкая ссылка на `/api/repositories`: несуществующий id при создании/изменении — 422 на поле `repository_id`. |
| `/api/citations` | Цитаты из источника (JSON): `GET` — список `[{"id", "source_id", "anchor", "text", "note", "private"}]`; `POST` — создание; `GET/PUT/DELETE /api/citations/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404); `GET /api/citations/search?q=` — поиск по началу текста. `source_id` — обязательная строгая ссылка на `/api/sources`: несуществующий id — 422 на поле `source_id`. `anchor` — необязательная полиморфная привязка «где именно» (плоский объект с дискриминатором `kind`: `archive` — `node_id`/`document_id`/`page`/`rect`, `file` — `attachment_id`/`timecode`, `url` — `url`; пусто или отсутствует — без привязки); если задана и несёт ссылку (`node_id`/`document_id`/`attachment_id`), существование тоже проверяется — 422 на поле `anchor.node_id`/`anchor.document_id`/`anchor.attachment_id`. |
| `/static/` | Собранные ассеты SPA (JS/CSS). |
| `/` | Веб-интерфейс (SPA): `index.html`, для неизвестных путей — fallback на неё. |

`sources` у `/api/admin-divisions`, `/api/repositories`, `/api/churches`, `/api/parishes`, `/api/archives`, `/api/notes` — редактируется с подпроекта 5 (`Citation` теперь имеет CRUD): `POST`/`PUT` принимают `sources: [{"citation_id", "reliability"?, "role"?, "note"?}]`; `target_type`/`target_id` клиент не отправляет — сервер подставляет владельца из контекста. Несуществующий `citation_id` — 422 на поле `sources[i].citation_id` (по индексу элемента). `/api/admin-divisions` — единственный маршрут, где `sources` раньше не было в ответе вовсе (не просто read-only) — теперь есть, как и у остальных пяти. HTTP `PUT` всегда полностью заменяет `sources` телом запроса: отсутствие ключа `sources` в JSON-теле очищает список (в отличие от MCP-тулов — см. ниже). `/api/archive-nodes`/`/api/archive-documents` (подпроект 6), `/api/families` (подпроект 7), `/api/people` (подпроект 8) и `/api/relations`/`/api/residences`/`/api/events` (подпроект 9) — первые полностью новые сущности программы с `sources`, редактируемым с самого начала (не ретрофит).

## Примеры запросов

### MCP

```bash
# Пинг / инициализация сессии
curl -s -X POST http://localhost:9000/mcp \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"curl","version":"1"}}}'
```

Доступные тулы:

| Тул | Описание |
|-----|---------|
| `division_list` | Список единиц административного деления; аргументы `kind`, `type`, `parent_id`, `limit`, `offset` (как параметры HTTP, см. ниже) |
| `division_search` | Поиск единиц по началу названия (включая варианты); аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `division_get` | Единица по `id` (JSON контракта); запись, ссылающаяся на приватную `Citation` среди источников (`sources[i].citation_id`), для не-владельца — ошибка тула (как отсутствующая); у `AdministrativeDivision` нет собственного `private` — это единственная проверка приватности |
| `division_create` | Создание единицы: `name`, `type`, `parent_id` (пусто — корень); id генерирует сервер; тип должен быть допустим внутри родителя (см. `division_type_list`), иначе — ошибка тула с перечнем допустимых типов |
| `division_update` | Изменение записи: `name`/`type` обязательны и заменяются всегда; `parent_id`/`sources` — при отсутствии аргумента в вызове сохраняют текущее значение (для `parent_id` явно переданная пустая строка по-прежнему означает «корень»); результат — обновлённая единица; при смене `type` или `parent_id` проверяется вложенность типов (новый родитель допускает тип, новый тип допускает типы текущих детей) |
| `division_delete` | Удаление единицы по `id`; занятая другая единицей — ошибка тула |
| `division_type_list` | Справочник типов единиц: `type`, `label`, `rank` (`null` у `other`), `settlement`, `children` — типы, допустимые внутри единицы этого типа |
| `surname_list` | Список словарных записей фамилий; аргументы `limit`, `offset` |
| `surname_search` | Поиск записей по началу канонической формы (включая варианты); аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `surname_get` | Запись по `id` (JSON контракта) |
| `surname_create` | Создание записи: `canonical`, `variants`, `items`, `notes` (тексты); id генерирует сервер |
| `surname_update` | Изменение записи: `canonical` обязателен и заменяется всегда; `variants`/`items`/`notes` — при отсутствии аргумента в вызове сохраняют текущее значение, явный пустой список — очищает его; результат — обновлённая запись |
| `surname_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
| `patronymic_list` | Список словарных записей отчеств; аргументы `limit`, `offset` |
| `patronymic_search` | Поиск записей по началу канонической формы (включая варианты); аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `patronymic_get` | Запись по `id` (JSON контракта) |
| `patronymic_create` | Создание записи: `canonical`, `variants`, `items`, `notes` (тексты); id генерирует сервер |
| `patronymic_update` | Изменение записи: `canonical` обязателен и заменяется всегда; `variants`/`items`/`notes` — при отсутствии аргумента в вызове сохраняют текущее значение, явный пустой список — очищает его; результат — обновлённая запись |
| `patronymic_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
| `estate_list` | Список словарных записей сословий; аргументы `limit`, `offset` |
| `estate_search` | Поиск записей по началу канонической формы (включая варианты); аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `estate_get` | Запись по `id` (JSON контракта) |
| `estate_create` | Создание записи: `canonical`, `variants`, `items`, `notes` (тексты); id генерирует сервер |
| `estate_update` | Изменение записи: `canonical` обязателен и заменяется всегда; `variants`/`items`/`notes` — при отсутствии аргумента в вызове сохраняют текущее значение, явный пустой список — очищает его; результат — обновлённая запись |
| `estate_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
| `title_list` | Список словарных записей титулов; аргументы `limit`, `offset` |
| `title_search` | Поиск записей по началу канонической формы (включая варианты); аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `title_get` | Запись по `id` (JSON контракта) |
| `title_create` | Создание записи: `canonical`, `variants`, `items`, `notes` (тексты); id генерирует сервер |
| `title_update` | Изменение записи: `canonical` обязателен и заменяется всегда; `variants`/`items`/`notes` — при отсутствии аргумента в вызове сохраняют текущее значение, явный пустой список — очищает его; результат — обновлённая запись |
| `title_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
| `given_name_list` | Список словарных записей имён; аргументы `limit`, `offset` |
| `given_name_search` | Поиск записей по началу канонической формы (включая варианты); аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `given_name_get` | Запись по `id` (JSON контракта) |
| `given_name_create` | Создание записи: `canonical`, `gender` (обязателен, `male`/`female`/`neutral`), `variants`, `items`, `notes` (тексты); id генерирует сервер |
| `given_name_update` | Изменение записи: `canonical`/`gender` обязательны и заменяются всегда (`gender` — `male`/`female`/`neutral`); `variants`/`items`/`notes` — при отсутствии аргумента в вызове сохраняют текущее значение, явный пустой список — очищает его; результат — обновлённая запись |
| `given_name_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
| `repository_list` | Список хранилищ-контейнеров источников; аргументы `limit`, `offset` |
| `repository_search` | Поиск хранилищ по началу названия; аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `repository_get` | Запись по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула (как отсутствующая); то же для записи, ссылающейся на приватную `Citation` среди источников (`sources[i].citation_id`) |
| `repository_create` | Создание записи: `name`, `type` (обязательны), `address`, `urls`, `notes` (тексты), `private`; id генерирует сервер |
| `repository_update` | Изменение записи: `name`/`type` обязательны и заменяются всегда; `address`/`urls`/`notes`/`private` — при отсутствии аргумента в вызове сохраняют текущее значение, переданное (в т.ч. пустой список или `false`) применяется как есть; результат — обновлённая запись |
| `repository_delete` | Удаление записи по `id`; на неё есть строгие ссылки (`Archive.repository_id`) — ошибка тула |
| `church_list` | Список церквей; аргументы `limit`, `offset` |
| `church_search` | Поиск церквей по началу названия; аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `church_get` | Запись по `id` (JSON контракта); запись, ссылающаяся на приватную `Citation` среди источников (`sources[i].citation_id`), для не-владельца — ошибка тула (как отсутствующая); у `Church` нет собственного `private` — это единственная проверка приватности |
| `church_create` | Создание записи: `name` (обязателен), `parish` (одиночная необязательная ссылка `{text, ref?, type?}`, ref/type сохраняются, если переданы обратно неизменными), `settlements`, `variants`, `notes`; id генерирует сервер |
| `church_update` | Изменение записи: `name` обязателен и заменяется всегда; `parish`/`settlements`/`variants`/`notes` — при отсутствии аргумента в вызове сохраняют текущее значение; `settlements`/`notes` принимают только текст (без ref/type в MCP-контракте) — ссылка на элементе теряется, если поле передано (`variants` — простой `[]string`, у него нет ref/type и терять нечего); `parish` сохраняет ref/type, если передать их обратно неизменными |
| `church_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
| `parish_list` | Список приходов; аргументы `limit`, `offset` |
| `parish_search` | Поиск приходов по началу названия; аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `parish_get` | Запись по `id` (JSON контракта); запись, ссылающаяся на приватную `Citation` среди источников (`sources[i].citation_id`), для не-владельца — ошибка тула (как отсутствующая); у `Parish` нет собственного `private` — это единственная проверка приватности |
| `parish_create` | Создание записи: `name` (обязателен), `church` (одиночная необязательная ссылка `{text, ref?, type?}`, ref/type сохраняются, если переданы обратно неизменными), `settlements`, `since`/`until` (структурированная дата, см. `factDateObjectProperties`), `notes`; id генерирует сервер |
| `parish_update` | Изменение записи: `name` обязателен и заменяется всегда; `church`/`settlements`/`since`/`until`/`notes` — при отсутствии аргумента в вызове сохраняют текущее значение; `settlements`/`notes` принимают только текст (ссылка теряется, если поле передано); `church` сохраняет ref/type, если передать их обратно неизменными |
| `parish_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
| `archive_list` | Список архивов; аргументы `limit`, `offset` |
| `archive_search` | Поиск архивов по началу названия; аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `archive_get` | Запись по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула (как отсутствующая); то же для записи, ссылающейся на приватную `Citation` среди источников (`sources[i].citation_id`) |
| `archive_create` | Создание записи: `name` (обязателен), `system` (только именем, без ссылки), `repository_id` (строгая ссылка — несуществующий id — ошибка тула на поле `repository_id`), `notes`, `private`; id генерирует сервер |
| `archive_update` | Изменение записи: `name` обязателен и заменяется всегда; `system`/`repository_id`/`notes`/`private` — при отсутствии аргумента в вызове сохраняют текущее значение; несуществующий `repository_id` — ошибка тула на поле `repository_id` |
| `archive_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
| `archive_node_list` | Список узлов архивного дерева; аргументы `archive_id` (обязателен), `parent_id` (необязателен — отсутствует: корень дерева внутри архива), `limit`, `offset` |
| `archive_node_search` | Поиск узлов по началу метки/названия, среди всех архивов (без сужения по `archive_id`); аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `archive_node_get` | Запись по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула (как отсутствующая); то же для записи, ссылающейся на приватную `Citation` среди источников (`sources[i].citation_id`) |
| `archive_node_create` | Создание записи: `type`, `archive_id`, `label` (обязательны), `parent_id` (строгая self-ref ссылка — должен принадлежать тому же архиву, иначе ошибка тула на поле `parent_id`), `name`, `since`/`until` (структурированная дата), `parish` (одиночная необязательная ссылка `{text, ref?, type?}`), `settlements`, `notes`, `sources`, `private`; id генерирует сервер |
| `archive_node_update` | Изменение записи: `type`/`archive_id`/`label` обязательны и заменяются всегда (`archive_id` при этом неизменяем — сценарий отвергает попытку сменить архив, см. `entity-write.md` §3.4); `parent_id`/`name`/`since`/`until`/`parish`/`settlements`/`notes`/`private`/`sources` — при отсутствии аргумента в вызове сохраняют текущее значение, явный пустой список/массив — очищает его; несуществующий `archive_id`/`parent_id`, `parent_id` из другого архива или цикл в цепочке родителей — ошибка тула на поле `archive_id`/`parent_id` |
| `archive_node_delete` | Удаление записи по `id`; на неё ссылаются дочерние узлы или документы — ошибка тула |
| `archive_document_list` | Список документов внутри единиц учёта (плоский список, без обязательных фильтров); аргументы `limit`, `offset` |
| `archive_document_search` | Поиск документов по началу названия; аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `archive_document_get` | Запись по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула (как отсутствующая); то же для записи, ссылающейся на приватную `Citation` среди источников (`sources[i].citation_id`) |
| `archive_document_create` | Создание записи: `unit_id` (строгая ссылка на узел архивного дерева, обязателен), `title` (обязателен), `kind`, `since`/`until`, `parish`, `settlements`, `notes`, `sources`, `private`; id генерирует сервер |
| `archive_document_update` | Изменение записи: `unit_id`/`title` обязательны и заменяются всегда; `kind`/`since`/`until`/`parish`/`settlements`/`notes`/`private`/`sources` — при отсутствии аргумента в вызове сохраняют текущее значение, явный пустой список/массив — очищает его; несуществующий `unit_id` — ошибка тула на поле `unit_id` |
| `archive_document_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
| `note_list` | Список заметок; аргументы `limit`, `offset` |
| `note_search` | Поиск заметок по началу заголовка; аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `note_get` | Запись по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула (как отсутствующая); то же для записи, ссылающейся на приватную `Citation` среди источников (`sources[i].citation_id`) |
| `note_create` | Создание записи: `kind` (обязателен), `title`, `text`, `parent_id` (строгая self-ref ссылка на другую заметку — несуществующий id — ошибка тула на поле `parent_id`), `private`; id генерирует сервер |
| `note_update` | Изменение записи: `kind` обязателен и заменяется всегда; `title`/`text`/`parent_id`/`private`/`sources` — при отсутствии аргумента в вызове сохраняют текущее значение; несуществующий `parent_id` или цикл в цепочке родителей — ошибка тула на поле `parent_id` |
| `note_delete` | Удаление записи по `id`; есть дочерние заметки или другие строгие ссылки — ошибка тула |
| `attachment_list` | Список вложений; аргументы `limit`, `offset` |
| `attachment_search` | Поиск вложений по началу имени файла или URI; аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `attachment_get` | Запись по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула (как отсутствующая) |
| `attachment_create` | Создание записи: `kind` (обязателен, закрытый перечень scan/document/audio/photo), `uri`, `filename`, `mime`, `page`, `node_id` (обязателен, строгая ссылка на архивный узел — несуществующий id — ошибка тула на поле `node_id`), `document_id` (необязателен, если задан — тоже проверяется, ошибка тула на поле `document_id`), `note`, `private`; id генерирует сервер |
| `attachment_update` | Изменение записи: `kind`/`node_id` обязательны и заменяются всегда; `uri`/`filename`/`mime`/`page`/`document_id`/`note`/`private` — при отсутствии аргумента в вызове сохраняют текущее значение (для `document_id` явно переданная пустая строка по-прежнему очищает ссылку); несуществующий `node_id` или `document_id` — ошибка тула на соответствующем поле |
| `attachment_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
| `source_list` | Список источников; аргументы `limit`, `offset` |
| `source_search` | Поиск источников по началу названия или автора; аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `source_get` | Запись по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула (как отсутствующая) |
| `source_create` | Создание записи: `kind` (обязателен, закрытый перечень), `title` (обязателен), `author`, `date` (структурированная дата, см. `factDateObjectProperties`), `reliability` (обязателен, закрытый перечень), `repository_id` (необязателен, если задан — проверяется, ошибка тула на поле `repository_id`), `notes`, `private`; id генерирует сервер |
| `source_update` | Изменение записи: `kind`/`title`/`reliability` обязательны и заменяются всегда; `author`/`date`/`repository_id`/`notes`/`private` — при отсутствии аргумента в вызове сохраняют текущее значение; несуществующий `repository_id` — ошибка тула на поле `repository_id` |
| `source_delete` | Удаление записи по `id`; занятая другой сущностью (например, `Citation`) — ошибка тула |
| `citation_list` | Список цитат; аргументы `limit`, `offset` |
| `citation_search` | Поиск цитат по началу текста; аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `citation_get` | Запись по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула (как отсутствующая) |
| `citation_create` | Создание записи: `source_id` (обязателен, строгая ссылка — несуществующий id — ошибка тула на поле `source_id`), `anchor` (необязательный объект — полиморфная привязка, см. `anchorObjectProperties`; если задана и несёт ссылку, существование тоже проверяется — ошибка тула на поле `anchor.node_id`/`anchor.document_id`/`anchor.attachment_id`), `text`, `note`, `private`; id генерирует сервер |
| `citation_update` | Изменение записи: `source_id` обязателен и заменяется всегда; `anchor`/`text`/`note`/`private` — при отсутствии аргумента в вызове сохраняют текущее значение; несуществующий `source_id` или ссылка внутри `anchor` — ошибка тула на соответствующем поле |
| `citation_delete` | Удаление записи по `id`; занятая другой сущностью (`SourceLink` у любой сущности с доказательствами) — ошибка тула |
| `family_list` | Список родов/линий; аргументы `limit`, `offset` |
| `family_search` | Поиск родов по началу названия; аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `family_get` | Запись по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула (как отсутствующая); то же для записи, ссылающейся на приватную `Citation` среди источников (`sources[i].citation_id`) |
| `family_create` | Создание записи: `name` (обязателен), `members`, `notes` (тексты; `members` — мягкая ссылка на персону, без проверки существования), `sources`, `private`; id генерирует сервер |
| `family_update` | Изменение записи: `name` обязателен и заменяется всегда; `members`/`notes`/`private`/`sources` — при отсутствии аргумента в вызове сохраняют текущее значение, явный пустой список/массив — очищает его; `members`/`notes` принимают только текст (без ref/type в MCP-контракте) — ссылка на элементе теряется, если поле передано |
| `family_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
| `person_list` | Список персон; аргументы `limit`, `offset` |
| `person_search` | Поиск персон по началу фамилии, имени или отчества из ЛЮБОГО из имён персоны (не только основного); аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `person_get` | Запись по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула (как отсутствующая); то же для записи, ссылающейся на приватную `Citation` среди источников (`sources[i].citation_id`) |
| `person_create` | Создание записи: все поля необязательны — `gender` (`male`/`female`/`unknown`), `names` (массив объектов `{type?, surname, given, patronymic, prefix?, suffix?, since?, until?}` — первый массив объектов, чьи собственные элементы несут вложенные объекты, см. `personNameObjectProperties`; `surname`/`given`/`patronymic` — мягкие ссылки, существование не проверяется), `estates`/`titles`/`nicknames`/`notes` (тексты; `estates`/`titles` — мягкая ссылка на словарь, без проверки существования), `sources`, `private`; id генерирует сервер |
| `person_update` | Изменение записи: все поля необязательны — `gender`/`estates`/`titles`/`nicknames`/`notes`/`private`/`names`/`sources` при отсутствии аргумента в вызове сохраняют текущее значение, явный пустой список/массив/`false` — очищает/сбрасывает его; `estates`/`titles`/`nicknames`/`notes` принимают только текст (без ref/type в MCP-контракте) — ссылка на элементе теряется, если поле передано |
| `person_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
| `relation_list` | Список рёбер графа родства; аргументы `person_id` (необязателен — только рёбра, где эта персона `person_a` ИЛИ `person_b`), `limit`, `offset`. **Нет `relation_search`** — у `Relation` нет собственных поисковых полей, см. `/api/relations` выше |
| `relation_get` | Ребро по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула; то же для ребра, ссылающегося на приватную `Person` через `person_a`/`person_b`, или на приватную `Citation` среди источников (`sources[i].citation_id`) |
| `relation_create` | Создание ребра: `kind` (обязателен, `blood`/`marriage`/`adoption`/`associate`), `rel_type` (обязателен только при `kind=associate`), `person_a`/`person_b` (обязательны, строгие ссылки на персону, должны отличаться), `since`/`until` (структурированная дата), `sources`, `notes`, `private`; id генерирует сервер |
| `relation_update` | Изменение ребра: `kind`/`person_a`/`person_b` — REQUIRED, заменяются БЕЗУСЛОВНО при каждом вызове (ядро того, что представляет собой ребро, как `citation_update`'s `source_id`); `rel_type`/`sources`/`notes`/`private` — обычный preserve-on-omit (отсутствие аргумента сохраняет текущее значение, явное пустое значение/пустой список очищает); `since`/`until` — одиночные объектные поля: presence-ONLY guard — отсутствие ключа сохраняет текущее значение, явный `null` очищает (пустой объект `{}` не проходит валидацию, так что это единственный способ очистки) |
| `relation_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
| `residence_list` | Список проживаний; аргументы `person_id`/`place_id` (оба необязательны, пересекаются, если заданы вместе), `limit`, `offset`. Нет `residence_search` (см. `relation_list`) |
| `residence_get` | Проживание по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула; то же для записи, ссылающейся на приватную `Person` через `person_id`, или на приватную `Citation` среди источников (`sources[i].citation_id`) |
| `residence_create` | Создание проживания: `person_id`/`place_id` (обязательны, строгие ссылки — `place_id` именно на `AdministrativeDivision`), `since`/`until`, `sources`, `note` (одна строка), `private`; id генерирует сервер |
| `residence_update` | Изменение проживания: `person_id`/`place_id` — REQUIRED, заменяются безусловно (как `relation_update`'s `person_a`/`person_b`); `sources`/`note`/`private` — preserve-on-omit; `since`/`until` — presence-only guard (см. `relation_update`) |
| `residence_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
| `event_list` | Список событий; аргумент `person_id` (необязателен — только события, где эта персона участвует), `limit`, `offset` |
| `event_search` | Поиск событий ТОЛЬКО по началу текста места (`place`) — единственное индексируемое поле события; аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `event_get` | Событие по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула; то же для события, ссылающегося на приватную `Person` через `participants[i].person_id`, или на приватную `Citation` среди источников (`sources[i].citation_id`) (оба случая — и в `event_search`) |
| `event_create` | Создание события: `type` (обязателен, открытый набор), `date` (структурированная дата), `place` (объект `{text, ref?, type?}` — МЯГКАЯ ссылка, существование НИКОГДА не проверяется, только формат/тип-ограничение), `participants` (массив объектов `{person_id, role, note?}` — `person_id` СТРОГАЯ ссылка на персону, единственная в этом подпроекте строгая ссылка внутри массива объектов; несуществующая — ошибка тула на индексированное поле `participants[i].person_id`), `sources`, `notes`, `private`; id генерирует сервер |
| `event_update` | Изменение события: `type` — REQUIRED, заменяется безусловно; `participants`/`sources`/`notes`/`private` — preserve-on-omit (отсутствие аргумента сохраняет текущее значение, явный пустой список очищает); `date`/`place` — одиночные объектные поля: presence-ONLY guard — отсутствие ключа сохраняет текущее значение, явный `null` очищает. `place` — самое рискованное поле этого подпроекта по истории регрессии (см. коммит "fix: null должен снова очищать TextRef/FactDate-поля") — нужен ИМЕННО presence-only guard (`if _, ok := args["place"]; ok {…}`), не `raw != nil` |
| `event_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |

`<entity>_create`/`<entity>_update` у `division`/`repository`/`church`/`parish`/`archive`/`note`/`archive_node`/`archive_document`/`family`/`person`/`relation`/`residence`/`event` принимают аргумент `sources` — массив объектов `{citation_id, reliability?, role?, note?}` (первый MCP-аргумент вида «массив объектов» в программе, введён в подпроекте 5, см. `sourceLinkObjectProperties`; раньше массивы были только строками). Несуществующий `citation_id` — ошибка тула на поле `sources[i].citation_id`. `person_create`/`person_update` (подпроект 8) дополнительно принимают `names` — первый массив объектов, чьи собственные элементы тоже несут вложенные объекты (`surname`/`given`/`patronymic` — `TextRef`, `since`/`until` — `FactDate`, см. `personNameObjectProperties`). `event_create`/`event_update` (подпроект 9) принимают `participants` — массив объектов `{person_id, role, note?}`, без вложенных объектов (проще `sources`/`names`), со СТРОГОЙ (проверяемой на существование) ссылкой `person_id` внутри каждого элемента — не первой такой ссылкой внутри массива объектов в программе (это уже `sources[i].citation_id`, подпроект 5), но первым случаем, когда строгая ссылка — главный субъект элемента массива (участник события И ЕСТЬ персона), а не одно из полей вспомогательной evidence-ссылки, как у `sources[i]`. `event_create`/`event_update` (подпроект 9) — первый тул программы с одиночным объектным аргументом ЗА ПРЕДЕЛАМИ `TextRef`/`FactDate`: `place` у события — `PlaceRef` (`{text, ref?, type?}`, структурно как `TextRef`, но отдельный транспортный тип, т.к. `models.PlaceRef` — отдельный тип модели). `relation_create`/`relation_update`/`residence_create`/`residence_update` (подпроект 9) тоже принимают объектные аргументы (`since`/`until` — `FactDate`), но это не ново: `FactDate`-объекты в программе появились раньше, начиная с подпроекта 3.

**Семантика отсутствующего аргумента в `*_update`, единая для всей программы**: у каждого `<entity>_update` тула ОБЯЗАТЕЛЬНЫЕ аргументы (перечислены в описании каждого тула выше) заменяются всегда, если переданы — MCP-фреймворк отклонит вызов без них. Любой НЕОБЯЗАТЕЛЬНЫЙ аргумент — если он отсутствует в вызове целиком, соответствующее поле записи сохраняет текущее значение как есть; если передан явно (в т.ч. пустой строкой/списком/`false`/`null`), поле заменяется переданным значением, вплоть до очистки. Это относится к КАЖДОМУ необязательному полю на каждом `*_update` туле — `sources` и `names` были первыми полями, получившими это поведение (подпроекты 5 и 8), но оно больше не исключение: обычные скалярные и списочные поля (`notes`, `private`, `variants` и т.п.) ведут себя так же с тех пор, как это было распространено на всю программу (см. `CHANGELOG.md`). Поля с собственной бизнес-логикой для пустого значения (`parent_id` у `division`/`note`/`archive_node` — явно пустая строка значит «корень»/«без родителя»; `document_id` у `attachment` — явно пустая строка значит «без документа») подчиняются тому же правилу «отсутствие = сохранить» — это НЕ исключение из него, отсутствие этих аргументов тоже сохраняет текущее значение. Особенность у них — только в том, что означает ЯВНО переданная пустая строка: для большинства полей это очистка/сброс, а для этих двух — по-прежнему их обычное действие (сделать корнем/убрать ссылку), а не эквивалент отсутствия.

Это отличается от HTTP `PUT`: там отсутствие ключа в теле запроса ВСЕГДА заменяет поле значением по умолчанию (пустой список/пустая строка/`false`) — JSON-тело не может дёшево отличить «поле не упомянуто» от «поле явно пустое» без превращения каждого поля DTO в указатель, а единственный HTTP-клиент в репозитории (веб-приложение) всегда отправляет все поля целиком. См. `CHANGELOG.md` для истории этого решения и его обоснования.

### HTTP API

```bash
curl -s http://localhost:9000/api/health
curl -s 'http://localhost:9000/api/admin-divisions?kind=settlement&limit=20'
```

### Список единиц деления

`GET /api/admin-divisions` и MCP-тул `division_list` принимают одни и те же параметры:

| Параметр | Значение |
|----------|----------|
| `kind` | пусто — без фильтра; `settlement` — только населённые пункты (город, село, деревня, хутор, погост, станица, местечко) |
| `type` | пусто — без фильтра; иначе точный тип: `namestnichestvo`, `provintsiya`, `guberniya`, `uezd`, `stan`, `volost`, `oblast`, `okrug`, `respublika`, `krai`, `rayon`, `selsovet`, `other`, `gorod`, `poselok`, `sloboda`, `selo`, `seltso`, `derevnya`, `hutor`, `pogost`, `stanitsa`, `mestechko` (см. `docs/models/places.md`) |
| `parent_id` | пусто — корень (весь список); иначе — только прямые дети указанной единицы (несуществующий родитель — `404`, неверный формат — `422`) |
| `limit` | размер окна: по умолчанию 50, не больше 500 (больше — сужается до 500) |
| `offset` | сдвиг окна, по умолчанию 0 |

`kind` и `type` пересекаются, `parent_id` сужает источник списка (корень или дети).
Порядок — порядок сохранения; окно считается после фильтра; результат короче
`limit` — конец списка. `parent_id` в ответе — `null` у корневых единиц. Ошибки
HTTP: `400` — `limit`/`offset` не целое число; `422` — неизвестные `kind`/`type`,
неверный формат `parent_id` или отрицательное окно (тело `{"error": "…", "field":
"kind"}`); `404` — несуществующий `parent_id`; `500` — сбой хранилища. Ошибки
MCP-тула приходят в результате вызова с признаком ошибки.

### Поиск единиц деления

`GET /api/admin-divisions/search` и MCP-тул `division_search` ищут единицы по
началу названия (включая варианты названий):

| Параметр | Значение |
|----------|----------|
| `q` | начало названия или варианта; пустое (после обрезки пробелов) — пустой результат без обращения к хранилищу |
| `limit` | размер окна, по умолчанию 50, не больше 500 |
| `offset` | сдвиг окна, по умолчанию 0 |

Результат — полные единицы `[{"id", "name", "type", "parent_id"}]`, окно
считается среди найденных единиц деления (после отбора по типу). Ошибки — как
у списка: `400` — `limit`/`offset` не число; `422` — отрицательное окно.

```bash
curl -s 'http://localhost:9000/api/admin-divisions/search?q=давы'
# → 200 [{"id":"AD-…","name":"Давыдово","type":"selo","parent_id":"AD-…"}]

curl -s 'http://localhost:9000/api/admin-divisions?parent_id=AD-…'
# → 200 [ … прямые дети единицы AD-… … ]
```

### Запись единиц делений

Полный CRUD единицы административного деления (S14+S15). id генерирует сервер
(`AD-…`); `parent_id` — `null`/пусто → корень. PUT полностью заменяет
`name`/`type`/`parent_id`: обработчик читает текущую версию, накладывает поля
запроса и сохраняет; прочие поля модели не затрагиваются.

```bash
# Создать губернию
curl -s -X POST http://localhost:9000/api/admin-divisions \
  -H 'Content-Type: application/json' \
  -d '{"name":"Московская","type":"guberniya"}'
# → 201 {"id":"AD-…","name":"Московская","type":"guberniya","parent_id":null}

# Создать село в губернии (возвращённый id — в кавычках JSON)
curl -s -X POST http://localhost:9000/api/admin-divisions \
  -H 'Content-Type: application/json' \
  -d '{"name":"Давыдово","type":"selo","parent_id":"AD-…"}'
# → 201 {"id":"AD-…","name":"Давыдово","type":"selo","parent_id":"AD-…"}

# Прочитать единицу по id (неверный формат id — 422)
curl -s http://localhost:9000/api/admin-divisions/AD-…
# → 200 {"id":"AD-…","name":"Давыдово","type":"selo","parent_id":"AD-…"}

# Изменить: parent_id:null переводит в корень
curl -s -X PUT http://localhost:9000/api/admin-divisions/AD-… \
  -H 'Content-Type: application/json' \
  -d '{"name":"Давыдова","type":"selo","parent_id":null}'
# → 200 {"id":"AD-…","name":"Давыдова","type":"selo","parent_id":null}

# Удалить (занятая единица — 409 со списком ссылающихся)
curl -s -X DELETE http://localhost:9000/api/admin-divisions/AD-…
# → 204 (успех, без тела)
```

Ошибки HTTP записи: `400` — тело не JSON (строки записаны неверно);
`404` — отсутствующая единица с корректным id; `409` — единица занята, тело
`{"error": "administrative_division \"AD-…\" используется: …", "referrers":
[{"type":"administrative_division","id":"AD-…"}, …]}` (список ссылающихся, до 20);
`422` — неверное значение (`name`, неизвестный `type`, несуществующий/циклический
`parent_id`, неверный формат `id` в пути; тело `{"error": "…", "field": "…"}`);
`500` — сбой хранилища. MCP-тулы `division_get|create|update|delete` принимают те
же значения: ошибки — результат тула с признаком ошибки, успехи `get/create/update`
— JSON контракта, `delete` — текст `деление "<id>" удалено`.

### Веб

Откройте http://localhost:9000/ — единая точка входа: каталог подключённых
сущностей по алфавиту (среди них «Административное деление», «Архивные
единицы», «Архивные документы», «Документация», «Фамилии»), каждая строка
ведёт на свой роут (`/divisions`, `/archive-nodes`, `/archive-documents`,
`/surnames`, …).
На каждой странице — хлебные крошки от корня («Сущности / …»). Страница
«Административное деление» показывает дерево иерархии от корня со строкой
поиска по началу названия; страница «Фамилии» — плоский список с поиском по
началу канонической формы. Вошедшему владельцу на обеих страницах доступны
просмотр, создание, редактирование и удаление записей.