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

Каталог данных, где лежит `db/genodex.db` и `db/journal.jsonl`:

```bash
genodex --data /path/to/data serve        # или после подкоманды
genodex serve --data /path/to/data
export GENODEX_DATA=/path/to/data         # env-переменная как дефолт
```

По умолчанию `--data` = `.` (текущий каталог).

### Подкоманды

```bash
genodex serve [--data DIR] [-p 9000] [-web prod|dev]   # сервер (по умолчанию)
genodex backup [--data DIR] [--to DIR]                 # бэкап: бандл «снапшот+журнал+манифест»
genodex restore --from DIR --to DIR [--force]          # восстановление из бандла
genodex verify [--data DIR] [--backup DIR]             # проверка целостности БД/журнала/бандла
```

- `backup` по умолчанию пишет в `<data>/backups/`.
- Если каталог данных и каталог бэкапа на одном устройстве, `backup` печатает warning в stderr (бэкап не защитит от отказа диска).
- `restore` требует пустой целевой каталог; `--force` разрешает запись поверх существующей БД (удаляет `db/genodex.db` и `db/journal.jsonl`).
- `verify` без `--backup` проверяет базу и журнал; с `--backup` сверяет также манифест бандла. При несоответствии — ошибка и ненулевой код возврата.

Флаги:

| Флаг | Значение по умолчанию | Назначение |
|------|----------------------|-----------|
| `-p` | `9000` | Порт HTTP-сервера (для `serve`) |
| `-web` | `prod` | Режим веб-ассетов: `prod` (встроены в бинарник) или `dev` (с диска `web/dist`) |
| `--data` | `.` или `GENODEX_DATA` | Каталог данных |
| `--to` | `<data>/backups` | Каталог бэкапа (для `backup`) / целевой каталог (для `restore`) |
| `--from` | — | Каталог бандла (для `restore`, обязателен) |
| `--backup` | — | Каталог бандла для сверки (для `verify`, опционально) |
| `--force` | `false` | Для `restore`: разрешить запись поверх существующей БД |

Сервер останавливается по `Ctrl+C` (graceful shutdown, до 5 с).

## Эндпоинты

| Путь | Назначение |
|------|-----------|
| `/mcp` | MCP-сервер, транспорт Streamable HTTP. Для AI-ассистентов и MCP-клиентов. |
| `/api/health` | Проверка живости: `{"status":"ok"}`. |
| `/api/settlements` | Список населённых пунктов (JSON). |
| `/static/` | Собранные ассеты SPA (JS/CSS). |
| `/` | Веб-интерфейс (SPA): `index.html`, для неизвестных путей — fallback на неё. |

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
| `settlement_list` | Получить список всех населённых пунктов |

### HTTP API

```bash
curl -s http://localhost:9000/api/health
curl -s http://localhost:9000/api/settlements
```

### Веб

Откройте http://localhost:9000/ — SPA-интерфейс со списком населённых пунктов.