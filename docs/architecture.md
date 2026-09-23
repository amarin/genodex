# Архитектура и структура проекта

## Обзор

Сервис построен слоями: внешние интерфейсы (MCP и HTTP API) → сценарии (usecase) → порт хранилища → SQLite. Все слои живут в одном процессе, одного бинарника достаточно для запуска. Долговременное хранилище — на диске (SQLite); в памяти данные живут только в цепочке обработки запроса.

```
                    ┌──────────────────────────────┐
                    │      cmd/genodex/main.go      │  флаги, подкоманды
                    │  serve (default) backup       │  backup/restore/verify
                    │  restore  verify  --data        │
                    └───────┬──────────┬────────────┘
                            ▼          ▼
              internal/app (сборка: New/Run/Shutdown)
                            ▼
             ┌──────────────┼──────────────┐
             ▼              ▼              ▼
    internal/mcp    internal/httpapi          web (embed + React SPA)
    (MCP-тулы)          (/api-роуты)          /static/ и / для людей
             └──────────────┼──────────┘
                            ▼
              internal/usecases/<scenario>/  (сценарии; deps.go — узкие интерфейсы)
                            ▼
                    internal/store            (порт: store.Store)
                            ▼
              internal/store/sqlstore         (адаптер: models ⇄ строки колоночной схемы)
                            ▼
                  internal/storage            (SQLite + backup/restore/verify)
                            ▼
                  internal/models (домен: сущности, value-типы, enum, ID; без JSON)
                  internal/transport (DTO + конвертеры models → JSON; только mcp/httpapi)
                  internal/definitions (встроенные данные)
```

## Структура каталогов

| Путь | Назначение |
|------|-----------|
| `cmd/genodex/main.go` | Точка входа: подкоманды `serve` (по умолчанию), `backup`, `restore`, `verify`; `--data` (или env `GENODEX_DATA`). |
| `internal/app/` | Сборка приложения: storage → sqlstore → usecases → mcp/httpapi → mux. `New`, `Run(ctx)` (graceful shutdown), останов хранилища. |
| `internal/models/` | Домен: сущности (человек, административное деление, церковь, приход, архив, событие, источник, семья и др.), value-типы (`FactDate`, `TextRef`, `Anchor`, `SourceLink`), enum-домены, `ID`, `Type`. Единственный внутренний пакет с данными: сценарии, хранилище и `definitions` говорят на нём. Без JSON-тегов и внешних зависимостей; стабилен после проектирования. |
| `internal/transport/` | DTO публичных контрактов (`/api/*`, MCP-тулы): типы с JSON-тегами и конвертеры `models → DTO`. Единственное место, где определена форма JSON на проводе; импортируют только `internal/httpapi` и `internal/mcp`. Изменение контракта правится здесь и не трогает домен и хранилище. |
| `internal/usecases/<scenario>/` | Сценарии: узкий интерфейс в `deps.go`, бизнес-логика, принимают/возвращают `models.<Type>`. MCP/API зависят только от этих интерфейсов. |
| `internal/store/` | Порт хранилища: интерфейс `store.Store` — на каждую сущность `Get`/`Save`/`List`/`Delete` с `ctx` (`List` — с режимом доступа `Access` и окном `Page`; `Get` — `ErrNotFound`, `Delete` — `*InUseError`), плюс `InTx`, `Search`, `ChildrenOfDivision`. `deps.go` — объявление. |
| `internal/store/sqlstore/` | Адаптер: `internal/storage` → `store.Store`. Отображает сущности `internal/models` на наборы строк колоночной схемы `internal/storage`, нормализует поисковые поля (`storage.Normalize`). |
| `internal/storage/` | Долговременное хранилище: SQLite (колоночная схема, `schema_version` 0, таблица `search_index`) + `backup`/`restore`/`verify`. |
| `internal/definitions/` | Встроенные доменные определения. `russia/` — системы административного деления (Российская империя 19 в., СССР). |
| `internal/mcp/` | Слой MCP на `github.com/mark3labs/mcp-go`. `NewServer` создаёт `server.MCPServer`, регистрирует тулы. Транспорт — Streamable HTTP. |
| `internal/httpapi/` | Слой HTTP API: JSON-роуты `/api/*` (health, admin-divisions, surnames, patronymics, estates, titles, given-names, repositories, churches, parishes, archives, auth, docs). Использует те же сценарии, что и MCP. |
| `web/` | Фронтенд: Vite + React 18 + antd. `src/` — исходники, `dist/` — результат сборки, `embed.go` — `//go:embed` и раздача. |
| `docs/` | Документация проекта. |

## Потоки данных

1. Клиент (MCP-клиент, браузер, curl) обращается к одному из маршрутов (`/mcp`, `/api/*`) или к CLI-подкоманде.
2. Хендлер `internal/mcp` или `internal/httpapi` вызывает сценарий (`internal/usecases/...`) через узкий интерфейс (`deps.go`).
3. Сценарий работает с портом `internal/store` (`store.Store`) и типами `internal/models`.
4. `sqlstore` сохраняет/читает сущности через `internal/storage` (SQLite).
5. Результат возвращается хендлеру, тот конвертирует `models` в DTO `internal/transport`, сериализует в JSON и отдаёт клиенту.

## Хранилище и восстановление

- Долговременные данные: `db/genodex.db` (SQLite, WAL). Все записи идут через транзакции БД; отдельного журнала приложений нет.
- Резервное копирование ручное: `genodex backup [--data DIR] [--to DIR]` → бандл «снапшот (VACUUM INTO) + манифест (SHA-256, счётчики сущностей)».
- `genodex restore --from DIR --to DIR [--force]` — восстановление из бандла в пустую директорию (`--force` — в занятую, удаляет существующую БД).
- `genodex verify [--data DIR] [--backup DIR]` — проверка целостности базы и (при `--backup`) манифеста бандла; ненулевой exit при несоответствии.
- Sentinel: если `db/` и каталог бэкапа лежат на одном устройстве, `backup` печатает warning в stderr (не fatal).

## Веб-раздача

- Фронтенд собирается в `web/dist` командой `npm run build`.
- `web/embed.go` встраивает `web/dist` в бинарник через `//go:embed all:dist`.
- Флаг запуска `-web` задаёт режим:
  - `prod` (по умолчанию) — файлы из встроенной в бинарник FS;
  - `dev` — файлы читаются с диска из `web/dist` (удобно при разработке фронта без пересборки Go).
- Vite-конфиг использует `base: '/static/'`, поэтому собранные ассеты отдаются под `/static`, а `/` — SPA-страница `index.html` с fallback на неё для client-side роутов.