# Разработка

## Команды

### Бэкенд (Go)

| Команда | Назначение |
|---------|-----------|
| `go build ./...` | Скомпилировать модуль. Требует существующий `web/dist` из-за `//go:embed`. |
| `go vet ./...` | Статический анализ. |
| `gofmt -w <файлы>` | Форматирование. Код должен оставаться gofmt-чистым. |
| `go run ./cmd/genodex -p <порт> [-web prod\|dev]` | Локальный запуск сервера (`serve` — подкоманда по умолчанию). |

### Фронтенд (web/)

| Команда | Назначение |
|---------|-----------|
| `npm install` | Установить зависимости (в `web/`). |
| `npm run build` | Собрать фронтенд в `web/dist` (с `base: '/static/'`). |
| `npm run typecheck` | Проверка типов TypeScript (без сборки). |
| `npm run dev` | Dev-сервер Vite (для изолированной работы с фронтом; т. к. сервис раздаёт фронт сам, обычно не требуется). |

## Ключевые правила

1. **Порядок сборки после правки фронта.** Изменения в `web/src/` не попадут в бинарник, пока не выполнен `npm run build` (обновляет `web/dist`), а затем `go build ./...`.
2. **`web/dist` — генерат.** Редактировать вручную нельзя, только `npm run build`.
3. **Режимы раздачи.** При изменении `web/embed.go` или способа раздачи проверяйте оба режима: `-web prod` (из бинарника) и `-web dev` (с диска).
4. **Контракты наружу.** `internal/mcp` (тулы) и `internal/httpapi` (маршруты `/api`) — публичные интерфейсы сервиса. Добавление/изменение тула или эндпоинта затрагивает и клиентов, и SPA (например, типы в `web/src/api.ts`).

## Что нельзя редактировать вручную

- `web/dist/**` — результат сборки фронта;
- `web/node_modules/**` — зависимости npm;
- корневой бинарник `genealogy-mcp` — старый артефакт; актуальная сборка — `genodex` из `cmd/genodex`.

## Проверка перед сдачей

```bash
gofmt -l .          # пусто
go build ./...
go vet ./...
go test ./...       # storage (журнал/БД/backup/restore/verify), sqlstore, usecases
```

Затем smoke-проверка основных маршрутов:

```bash
go run ./cmd/genodex -p 9000 &
curl -s http://localhost:9000/api/health      # {"status":"ok"}
curl -s http://localhost:9000/api/settlements # [...]
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:9000/  # 200
```

Резервное копирование данных (на диске):

```bash
go build -o genodex ./cmd/genodex
./genodex backup --data /path/to/data --to /path/to/bkp
./genodex verify --data /path/to/data --backup /path/to/bkp
./genodex restore --from /path/to/bkp --to /path/to/restored
```