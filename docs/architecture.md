# Архитектура и структура проекта

## Обзор

Сервис построен слоями: внешние интерфейсы (MCP и HTTP API) → сценарии (usecase) → порт хранилища → SQLite с журналом. Все слои живут в одном процессе, одного бинарника достаточно для запуска. Долговременное хранилище — на диске (SQLite + append-only журнал); в памяти данные живут только в цепочке обработки запроса.

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
              internal/store/sqlstore         (адаптер: JSON entity ⇄ blob)
                            ▼
                  internal/storage            (SQLite + журнал + backup/restore/verify)
                            ▼
                  internal/entity (внешние JSON-контракты)
                  internal/models (внутренние типы сценариев)
                  internal/definitions (встроенные данные)
```

## Структура каталогов

| Путь | Назначение |
|------|-----------|
| `cmd/genodex/main.go` | Точка входа: подкоманды `serve` (по умолчанию), `backup`, `restore`, `verify`; `--data` (или env `GENODEX_DATA`). |
| `internal/app/` | Сборка приложения: storage → sqlstore → usecases → mcp/httpapi → mux. `New`, `Run(ctx)` (graceful shutdown), останов хранилища. |
| `internal/entity/` | Доменные модели — внешние JSON-контракты сервиса (человек, нас. пункт, церковь, приход, губерния, уезд, волость, архив, фонд, опись, дело, событие, брак, источник). JSON-теги задают форму JSON в `/api` и MCP-тулах. |
| `internal/models/` | Внутренние типы сценариев (не контракты API). |
| `internal/usecases/<scenario>/` | Сценарии: узкий интерфейс в `deps.go`, бизнес-логика, принимают/возвращают `models.<Type>`. MCP/API зависят только от этих интерфейсов. |
| `internal/store/` | Порт хранилища: интерфейс `store.Store` (типизированные `Get`/`Save`/`List` на каждую сущность). `deps.go` — объявление. |
| `internal/store/sqlstore/` | Адаптер: `internal/storage` → `store.Store`. Сериализует `entity` в JSON-блоб, нормализует поисковые поля (`storage.Normalize`). |
| `internal/storage/` | Долговременное хранилище: SQLite (снапшот) + append-only журнал (JSONL) + `backup`/`restore`/`verify`. Восстанавливается из журнала при потере БД. |
| `internal/definitions/` | Встроенные доменные определения. `russia/` — системы административного деления (Российская империя 19 в., СССР). |
| `internal/mcp/` | Слой MCP на `github.com/mark3labs/mcp-go`. `NewServer` создаёт `server.MCPServer`, регистрирует тулы. Транспорт — Streamable HTTP. |
| `internal/httpapi/` | Слой HTTP API: JSON-роуты `/api/*` (health, settlements, docs). Использует те же сценарии, что и MCP. |
| `web/` | Фронтенд: Vite + React 18 + antd. `src/` — исходники, `dist/` — результат сборки, `embed.go` — `//go:embed` и раздача. |
| `docs/` | Документация проекта. |

## Потоки данных

1. Клиент (MCP-клиент, браузер, curl) обращается к одному из маршрутов (`/mcp`, `/api/*`) или к CLI-подкоманде.
2. Хендлер `internal/mcp` или `internal/httpapi` вызывает сценарий (`internal/usecases/...`) через узкий интерфейс (`deps.go`).
3. Сценарий работает с портом `internal/store` (`store.Store`) и типами `internal/models`/`internal/entity`.
4. `sqlstore` сериализует сущности в JSON-блоб и сохраняет/читает через `internal/storage` (SQLite + журнал).
5. Результат возвращается сценарию, сериализуется в JSON и уходит клиенту.

## Хранилище и восстановление

- Долговременные данные: `db/genodex.db` (SQLite-снапшот) + `db/journal.jsonl` (append-only журнал полных образов сущностей).
- При старте `internal/storage` реплеит журнал, если снапшот отстал («журнал не младше БД» — инвариант, тест `TestStorageRecoversWhenDBLost`).
- Резервное копирование ручное: `genodex backup [--data DIR] [--to DIR]` → бандл «снапшот + журнал + манифест (SHA)».
- `genodex restore --from DIR --to DIR [--force]` — восстановление из бандла в пустую директорию (`--force` — в занятую, удаляет существующую БД и журнал).
- `genodex verify [--data DIR] [--backup DIR]` — проверка целостности базы, журнала и (при `--backup`) манифеста бандла; ненулевой exit при несоответствии.
- Sentinel: если `db/` и каталог бэкапа лежат на одном устройстве, `backup` печатает warning в stderr (не fatal).

## Веб-раздача

- Фронтенд собирается в `web/dist` командой `npm run build`.
- `web/embed.go` встраивает `web/dist` в бинарник через `//go:embed all:dist`.
- Флаг запуска `-web` задаёт режим:
  - `prod` (по умолчанию) — файлы из встроенной в бинарник FS;
  - `dev` — файлы читаются с диска из `web/dist` (удобно при разработке фронта без пересборки Go).
- Vite-конфиг использует `base: '/static/'`, поэтому собранные ассеты отдаются под `/static`, а `/` — SPA-страница `index.html` с fallback на неё для client-side роутов.