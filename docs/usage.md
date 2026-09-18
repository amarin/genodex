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

```bash
# prod: фронтенд из бинарника (по умолчанию)
go run ./cmd/genealogy-mcp -p 9000

# dev: фронтенд с диска web/dist, без пересборки Go после правок фронта
go run ./cmd/genealogy-mcp -p 9000 -web dev
```

Флаги:

| Флаг | Значение по умолчанию | Назначение |
|------|----------------------|-----------|
| `-p` | `9000` | Порт HTTP-сервера |
| `-web` | `prod` | Режим веб-ассетов: `prod` (встроены в бинарник) или `dev` (с диска `web/dist`) |

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