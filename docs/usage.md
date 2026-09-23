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
| `/api/admin-divisions` | Единицы административного деления (JSON): `GET` — список `[{"id", "name", "type", "parent_id"}]` (параметры, включая `parent_id`, — ниже); `POST` — создание; `GET/PUT/DELETE /api/admin-divisions/{id}` — чтение/изменение/удаление единицы; `GET /api/admin-divisions/search` — поиск по началу названия. |
| `/api/surnames` | Словарные записи фамилий (JSON): `GET` — список `[{"id", "canonical", "variants", "items", "notes"}]`; `POST` — создание; `GET/PUT/DELETE /api/surnames/{id}` — чтение/изменение/удаление записи; `GET /api/surnames/search?q=` — поиск по началу канонической формы (включая варианты). |
| `/api/patronymics` | Словарные записи отчеств (JSON): `GET` — список `[{"id", "canonical", "variants", "items", "notes"}]`; `POST` — создание; `GET/PUT/DELETE /api/patronymics/{id}` — чтение/изменение/удаление записи; `GET /api/patronymics/search?q=` — поиск по началу канонической формы (включая варианты). |
| `/api/estates` | Словарные записи сословий (JSON): `GET` — список `[{"id", "canonical", "variants", "items", "notes"}]`; `POST` — создание; `GET/PUT/DELETE /api/estates/{id}` — чтение/изменение/удаление записи; `GET /api/estates/search?q=` — поиск по началу канонической формы (включая варианты). |
| `/api/titles` | Словарные записи титулов (JSON): `GET` — список `[{"id", "canonical", "variants", "items", "notes"}]`; `POST` — создание; `GET/PUT/DELETE /api/titles/{id}` — чтение/изменение/удаление записи; `GET /api/titles/search?q=` — поиск по началу канонической формы (включая варианты). |
| `/api/given-names` | Словарные записи имён (JSON): `GET` — список `[{"id", "canonical", "gender", "variants", "items", "notes"}]`; `POST` — создание; `GET/PUT/DELETE /api/given-names/{id}` — чтение/изменение/удаление записи; `GET /api/given-names/search?q=` — поиск по началу канонической формы (включая варианты). |
| `/api/repositories` | Хранилища-контейнеры источников (JSON): `GET` — список `[{"id", "name", "type", "address", "urls", "notes", "sources", "private"}]`; `POST` — создание; `GET/PUT/DELETE /api/repositories/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404, как отсутствующая); `GET /api/repositories/search?q=` — поиск по началу названия. |
| `/api/churches` | Церкви (JSON): `GET` — список `[{"id", "name", "parish", "settlements", "variants", "notes", "sources"}]`; `POST` — создание; `GET/PUT/DELETE /api/churches/{id}` — чтение/изменение/удаление записи; `GET /api/churches/search?q=` — поиск по началу названия. `parish` — одиночная необязательная ссылка (`{text, ref?, type?}`, v1-форма редактирует только text). |
| `/api/parishes` | Приходы (JSON): `GET` — список `[{"id", "name", "church", "settlements", "since", "until", "notes", "sources"}]`; `POST` — создание; `GET/PUT/DELETE /api/parishes/{id}` — чтение/изменение/удаление записи; `GET /api/parishes/search?q=` — поиск по началу названия. `since`/`until` — структурированная дата (`FactDate`, см. `web/src/FactDateEditor.tsx`). |
| `/api/archives` | Архивы (JSON): `GET` — список `[{"id", "name", "system", "repository_id", "notes", "sources", "private"}]`; `POST` — создание; `GET/PUT/DELETE /api/archives/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404); `GET /api/archives/search?q=` — поиск по началу названия. `repository_id` — строгая ссылка на `/api/repositories`: несуществующий id при создании/изменении — 422 на поле `repository_id`. |
| `/api/notes` | Заметки (markdown-текст с иерархией «книга → главы», JSON): `GET` — список `[{"id", "kind", "title", "text", "parent_id", "sources", "private"}]`; `POST` — создание; `GET/PUT/DELETE /api/notes/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404, как отсутствующая); `GET /api/notes/search?q=` — поиск по началу заголовка. `parent_id` — строгая self-ref ссылка на другую заметку: несуществующий id — 422 на поле `parent_id`; при `PUT`, если новый `parent_id` образует цикл (прямой или через цепочку), — тоже 422 на поле `parent_id`. |
| `/api/attachments` | Файловые вложения (JSON): `GET` — список `[{"id", "kind", "uri", "filename", "mime", "page", "node_id", "document_id", "note", "private"}]`; `POST` — создание; `GET/PUT/DELETE /api/attachments/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404); `GET /api/attachments/search?q=` — поиск по началу имени файла или URI. `node_id` — обязательная строгая ссылка на архивный узел (`ArchiveNode`, ещё без своего `/api/`-эндпоинта — подпроект 6): несуществующий id — 422 на поле `node_id`. `document_id` — необязательная мягкая ссылка на архивный документ (`ArchiveDocument`, тоже подпроект 6, `ON DELETE SET NULL` в схеме); при создании/изменении, если задан, существование тоже проверяется — 422 на поле `document_id`. |
| `/api/sources` | Источники доказательств (JSON): `GET` — список `[{"id", "kind", "title", "author", "date", "reliability", "repository_id", "notes", "private"}]`; `POST` — создание; `GET/PUT/DELETE /api/sources/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404); `GET /api/sources/search?q=` — поиск по началу названия или автора. `date` — структурированная дата (`FactDate`). `repository_id` — мягкая ссылка на `/api/repositories`: несуществующий id при создании/изменении — 422 на поле `repository_id`. |
| `/api/citations` | Цитаты из источника (JSON): `GET` — список `[{"id", "source_id", "anchor", "text", "note", "private"}]`; `POST` — создание; `GET/PUT/DELETE /api/citations/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404); `GET /api/citations/search?q=` — поиск по началу текста. `source_id` — обязательная строгая ссылка на `/api/sources`: несуществующий id — 422 на поле `source_id`. `anchor` — необязательная полиморфная привязка «где именно» (плоский объект с дискриминатором `kind`: `archive` — `node_id`/`document_id`/`page`/`rect`, `file` — `attachment_id`/`timecode`, `url` — `url`; пусто или отсутствует — без привязки); если задана и несёт ссылку (`node_id`/`document_id`/`attachment_id`), существование тоже проверяется — 422 на поле `anchor.node_id`/`anchor.document_id`/`anchor.attachment_id`. |
| `/static/` | Собранные ассеты SPA (JS/CSS). |
| `/` | Веб-интерфейс (SPA): `index.html`, для неизвестных путей — fallback на неё. |

`sources` у `/api/admin-divisions`, `/api/repositories`, `/api/churches`, `/api/parishes`, `/api/archives`, `/api/notes` — редактируется с подпроекта 5 (`Citation` теперь имеет CRUD): `POST`/`PUT` принимают `sources: [{"citation_id", "reliability"?, "role"?, "note"?}]`; `target_type`/`target_id` клиент не отправляет — сервер подставляет владельца из контекста. Несуществующий `citation_id` — 422 на поле `sources[i].citation_id` (по индексу элемента). `/api/admin-divisions` — единственный маршрут, где `sources` раньше не было в ответе вовсе (не просто read-only) — теперь есть, как и у остальных пяти.

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
| `division_get` | Единица по `id` (JSON контракта) |
| `division_create` | Создание единицы: `name`, `type`, `parent_id` (пусто — корень); id генерирует сервер |
| `division_update` | Изменение `name`/`type`/`parent_id` (пустой `parent_id` — корень), прочие поля сохраняются; результат — обновлённая единица |
| `division_delete` | Удаление единицы по `id`; занятая другая единицей — ошибка тула |
| `surname_list` | Список словарных записей фамилий; аргументы `limit`, `offset` |
| `surname_search` | Поиск записей по началу канонической формы (включая варианты); аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `surname_get` | Запись по `id` (JSON контракта) |
| `surname_create` | Создание записи: `canonical`, `variants`, `items`, `notes` (тексты); id генерирует сервер |
| `surname_update` | Изменение записи: полная замена `canonical`/`variants`/`items`/`notes`; результат — обновлённая запись |
| `surname_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
| `patronymic_list` | Список словарных записей отчеств; аргументы `limit`, `offset` |
| `patronymic_search` | Поиск записей по началу канонической формы (включая варианты); аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `patronymic_get` | Запись по `id` (JSON контракта) |
| `patronymic_create` | Создание записи: `canonical`, `variants`, `items`, `notes` (тексты); id генерирует сервер |
| `patronymic_update` | Изменение записи: полная замена `canonical`/`variants`/`items`/`notes`; результат — обновлённая запись |
| `patronymic_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
| `estate_list` | Список словарных записей сословий; аргументы `limit`, `offset` |
| `estate_search` | Поиск записей по началу канонической формы (включая варианты); аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `estate_get` | Запись по `id` (JSON контракта) |
| `estate_create` | Создание записи: `canonical`, `variants`, `items`, `notes` (тексты); id генерирует сервер |
| `estate_update` | Изменение записи: полная замена `canonical`/`variants`/`items`/`notes`; результат — обновлённая запись |
| `estate_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
| `title_list` | Список словарных записей титулов; аргументы `limit`, `offset` |
| `title_search` | Поиск записей по началу канонической формы (включая варианты); аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `title_get` | Запись по `id` (JSON контракта) |
| `title_create` | Создание записи: `canonical`, `variants`, `items`, `notes` (тексты); id генерирует сервер |
| `title_update` | Изменение записи: полная замена `canonical`/`variants`/`items`/`notes`; результат — обновлённая запись |
| `title_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
| `given_name_list` | Список словарных записей имён; аргументы `limit`, `offset` |
| `given_name_search` | Поиск записей по началу канонической формы (включая варианты); аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `given_name_get` | Запись по `id` (JSON контракта) |
| `given_name_create` | Создание записи: `canonical`, `gender` (обязателен, `male`/`female`/`neutral`), `variants`, `items`, `notes` (тексты); id генерирует сервер |
| `given_name_update` | Изменение записи: полная замена `canonical`/`gender`/`variants`/`items`/`notes` (`gender` обязателен, `male`/`female`/`neutral`); результат — обновлённая запись |
| `given_name_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
| `repository_list` | Список хранилищ-контейнеров источников; аргументы `limit`, `offset` |
| `repository_search` | Поиск хранилищ по началу названия; аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `repository_get` | Запись по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула (как отсутствующая) |
| `repository_create` | Создание записи: `name`, `type` (обязательны), `address`, `urls`, `notes` (тексты), `private`; id генерирует сервер |
| `repository_update` | Изменение записи: полная замена `name`/`type`/`address`/`urls`/`notes`/`private`; результат — обновлённая запись |
| `repository_delete` | Удаление записи по `id`; на неё есть строгие ссылки (`Archive.repository_id`) — ошибка тула |
| `church_list` | Список церквей; аргументы `limit`, `offset` |
| `church_search` | Поиск церквей по началу названия; аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `church_get` | Запись по `id` (JSON контракта) |
| `church_create` | Создание записи: `name` (обязателен), `parish` (одиночная необязательная ссылка `{text, ref?, type?}`, ref/type сохраняются, если переданы обратно неизменными), `settlements`, `variants`, `notes`; id генерирует сервер |
| `church_update` | Изменение записи: полная замена `name`/`parish`/`settlements`/`variants`/`notes`; `settlements`/`notes` принимают только текст (без ref/type в MCP-контракте) — ссылка на элементе теряется при любом обновлении, пока не появится picker; `parish` сохраняет ref/type, если передать их обратно неизменными |
| `church_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
| `parish_list` | Список приходов; аргументы `limit`, `offset` |
| `parish_search` | Поиск приходов по началу названия; аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `parish_get` | Запись по `id` (JSON контракта) |
| `parish_create` | Создание записи: `name` (обязателен), `church` (одиночная необязательная ссылка `{text, ref?, type?}`, ref/type сохраняются, если переданы обратно неизменными), `settlements`, `since`/`until` (структурированная дата, см. `factDateObjectProperties`), `notes`; id генерирует сервер |
| `parish_update` | Изменение записи: полная замена `name`/`church`/`settlements`/`since`/`until`/`notes`; `settlements`/`notes` принимают только текст (ссылка теряется при обновлении через MCP); `church` сохраняет ref/type, если передать их обратно неизменными |
| `parish_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
| `archive_list` | Список архивов; аргументы `limit`, `offset` |
| `archive_search` | Поиск архивов по началу названия; аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `archive_get` | Запись по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула (как отсутствующая) |
| `archive_create` | Создание записи: `name` (обязателен), `system` (только именем, без ссылки), `repository_id` (строгая ссылка — несуществующий id — ошибка тула на поле `repository_id`), `notes`, `private`; id генерирует сервер |
| `archive_update` | Изменение записи: полная замена `name`/`system`/`repository_id`/`notes`/`private`; несуществующий `repository_id` — ошибка тула на поле `repository_id` |
| `archive_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
| `note_list` | Список заметок; аргументы `limit`, `offset` |
| `note_search` | Поиск заметок по началу заголовка; аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `note_get` | Запись по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула (как отсутствующая) |
| `note_create` | Создание записи: `kind` (обязателен), `title`, `text`, `parent_id` (строгая self-ref ссылка на другую заметку — несуществующий id — ошибка тула на поле `parent_id`), `private`; id генерирует сервер |
| `note_update` | Изменение записи: полная замена `kind`/`title`/`text`/`parent_id`/`private`; несуществующий `parent_id` или цикл в цепочке родителей — ошибка тула на поле `parent_id` |
| `note_delete` | Удаление записи по `id`; есть дочерние заметки или другие строгие ссылки — ошибка тула |
| `attachment_list` | Список вложений; аргументы `limit`, `offset` |
| `attachment_search` | Поиск вложений по началу имени файла или URI; аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `attachment_get` | Запись по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула (как отсутствующая) |
| `attachment_create` | Создание записи: `kind` (обязателен, закрытый перечень scan/document/audio/photo), `uri`, `filename`, `mime`, `page`, `node_id` (обязателен, строгая ссылка на архивный узел — несуществующий id — ошибка тула на поле `node_id`), `document_id` (необязателен, если задан — тоже проверяется, ошибка тула на поле `document_id`), `note`, `private`; id генерирует сервер |
| `attachment_update` | Изменение записи: полная замена `kind`/`uri`/`filename`/`mime`/`page`/`node_id`/`document_id`/`note`/`private`; несуществующий `node_id` или `document_id` — ошибка тула на соответствующем поле |
| `attachment_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
| `source_list` | Список источников; аргументы `limit`, `offset` |
| `source_search` | Поиск источников по началу названия или автора; аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `source_get` | Запись по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула (как отсутствующая) |
| `source_create` | Создание записи: `kind` (обязателен, закрытый перечень), `title` (обязателен), `author`, `date` (структурированная дата, см. `factDateObjectProperties`), `reliability` (обязателен, закрытый перечень), `repository_id` (необязателен, если задан — проверяется, ошибка тула на поле `repository_id`), `notes`, `private`; id генерирует сервер |
| `source_update` | Изменение записи: полная замена `kind`/`title`/`author`/`date`/`reliability`/`repository_id`/`notes`/`private`; несуществующий `repository_id` — ошибка тула на поле `repository_id` |
| `source_delete` | Удаление записи по `id`; занятая другой сущностью (например, `Citation`) — ошибка тула |
| `citation_list` | Список цитат; аргументы `limit`, `offset` |
| `citation_search` | Поиск цитат по началу текста; аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `citation_get` | Запись по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула (как отсутствующая) |
| `citation_create` | Создание записи: `source_id` (обязателен, строгая ссылка — несуществующий id — ошибка тула на поле `source_id`), `anchor` (необязательный объект — полиморфная привязка, см. `anchorObjectProperties`; если задана и несёт ссылку, существование тоже проверяется — ошибка тула на поле `anchor.node_id`/`anchor.document_id`/`anchor.attachment_id`), `text`, `note`, `private`; id генерирует сервер |
| `citation_update` | Изменение записи: полная замена `source_id`/`anchor`/`text`/`note`/`private`; несуществующий `source_id` или ссылка внутри `anchor` — ошибка тула на соответствующем поле |
| `citation_delete` | Удаление записи по `id`; занятая другой сущностью (`SourceLink` у любой сущности с доказательствами) — ошибка тула |

`<entity>_create`/`<entity>_update` у `division`/`repository`/`church`/`parish`/`archive`/`note` с подпроекта 5 принимают аргумент `sources` — массив объектов `{citation_id, reliability?, role?, note?}` (первый MCP-аргумент вида «массив объектов» в программе, см. `sourceLinkObjectProperties`; раньше массивы были только строками). Несуществующий `citation_id` — ошибка тула на поле `sources[i].citation_id`.

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
| `type` | пусто — без фильтра; иначе точный тип: `governorate`, `district`, `volost`, `gorod`, `selo`, `derevnya`, `hutor`, `pogost`, `stanitsa`, `mestechko`, `other` |
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
  -d '{"name":"Московская","type":"governorate"}'
# → 201 {"id":"AD-…","name":"Московская","type":"governorate","parent_id":null}

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
сущностей по алфавиту (сейчас «Административное деление», «Документация»,
«Фамилии»), каждая строка ведёт на свой роут (`/divisions`, `/surnames`, …).
На каждой странице — хлебные крошки от корня («Сущности / …»). Страница
«Административное деление» показывает дерево иерархии от корня со строкой
поиска по началу названия; страница «Фамилии» — плоский список с поиском по
началу канонической формы. Вошедшему владельцу на обеих страницах доступны
просмотр, создание, редактирование и удаление записей.