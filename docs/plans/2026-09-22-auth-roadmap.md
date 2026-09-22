# Аутентификация и Access — дорожная карта этапов

**Spec:** `docs/data-model/auth.md` (дизайн; §-ссылки ниже — на него).
**Предыдущий проход:** «ядро чтения и записи» (S1–S16,
`docs/implementation/core-read-write.md`) — открытый вопрос «`Access` во
внешних контрактах» этого прохода закрывается здесь.

Шесть этапов, ритуал — как в S1–S16: маленькие коммиты по слоям прямо на
`main`, подробный TDD-план пишется непосредственно перед запуском этапа
(следующие этапы опираются на реальные имена и сигнатуры предыдущих), каждый
коммит проходит рубеж `gofmt -l .` пусто, `go build ./...`, `go vet ./...`,
`go test ./...`, smoke.

## Обзор

| # | Этап | Инкремент | Зависит от |
|---|---|---|---|
| A | Домен `internal/auth` | `Owner`/`Session`/`APIToken`/`Invite`, `Validate()`, хеширование пароля и токенов, генератор ID, узкий `Store`-порт (`deps.go`) | — |
| A2 | Хранилище `internal/auth` | Таблицы `owners`/`sessions`/`api_tokens`/`invites`, CRUD, срок действия, ротация refresh | A |
| B | HTTP-контракт auth | `/api/auth/*` (status/register/login/logout/refresh/password/invites/tokens), cookies, middleware `resolveAccess`, заголовок CSRF | A2 |
| B2 | MCP-контракт auth | Bearer-middleware на `/mcp`, `401` без валидного `APIToken` | A2 |
| C | Проброс `Access` в делениях | `Access` параметром в `list_divisions`/`search_divisions`; `401` на записи в httpapi/mcp; `internal/app` собирает `auth`-сервис и подключает оба middleware | B, B2 |
| D | Веб | `/login`, `/register` (bootstrap и invite), `/settings` (пароль/invite/токены), состояние сессии в шапке | C |

## A. Домен `internal/auth`

- **Файлы:** `internal/auth/{owner,session,api_token,invite,hash,id}.go`
  (+тесты), `internal/auth/service.go` (+`deps_test.go` мок), `deps.go`.
- **Решения дизайна:** §2, §9 (`auth.md`). Независимый ID-генератор (не
  `internal/idgen`) — префиксы `OW`/`SS`/`AT`/`IV`. `bcrypt` для паролей,
  `SHA-256` для токенов, `gnx_`-префикс у API-токенов.
- **Приёмка:** `Validate()` по каждому типу (таблично); `hashPassword`/
  `verifyPassword` round-trip; `newRawToken`/`newAPIToken` — формат, длина,
  уникальность на серии (10 000 значений, как `idgen_test.go`); сервис на
  фейковом `Store` — bootstrap (нет владельцев → invite не нужен), invite
  обязателен при непустой таблице, невалидный/использованный/просроченный
  invite — ошибка; `Refresh` ротирует оба токена и инвалидирует старый
  refresh (повторный вызов со старым — ошибка); просроченная/отозванная
  сессия/токен — ошибка `ResolveAccess`/`ResolveAPIToken`.

## A2. Хранилище `internal/auth`

- **Статус:** выполнено (6 задач-коммитов + консолидированный проход по
  итогам финального ревью — см. `2026-09-22-auth-a2-storage.md`).
- **Файлы:** таблицы в `internal/storage/schema.go` (или отдельный файл
  схемы, вне generic-графа), `internal/auth/sqlstore.go` (+тесты), реализация
  порта `Store` из фазы A.
- **Приёмка:** CRUD по всем четырём таблицам; чтение просроченной
  сессии/приглашения — как отсутствующей; `RevokeAPIToken` — токен больше не
  резолвится; `TouchAPIToken` обновляет `last_used_at`; FK
  `sessions.owner_id`/`api_tokens.owner_id`/`invites.created_by` — обычные,
  не участвуют в `sqlstore/fkgraph.go`; `TestTypeRegistryIsConsistent` и
  тесты `Search` (существующие generic-тесты) не меняются и не видят
  auth-таблицы; `fkgraph_test.go` не меняет тестовую *логику*, но его
  data-список `serviceTables` получает четыре имени auth-таблиц — это не
  нарушение изоляции auth от generic-графа, а ровно тот механизм, для
  которого `serviceTables` заведена (см. `2026-09-22-auth-a2-storage.md`,
  «Предпосылка из этапа A», для полного обоснования RESTRICT/SET NULL FK).

## B. HTTP-контракт auth

- **Статус:** выполнено (5 задач-коммитов + консолидированный проход по
  итогам финального ревью — см. `2026-09-22-auth-b-http.md`).
- **Файлы:** `internal/httpapi/auth.go` (+тесты), `internal/httpapi/deps.go`
  (интерфейс `AuthService`), `internal/httpapi/middleware.go`
  (`resolveAccess`, `AccessFromContext`, `OwnerFromContext`),
  `internal/httpapi/httpapi.go` (маршруты, подключение middleware).
- **Решения дизайна:** §4. Cookies `genodex_access`/`genodex_refresh`
  (`HttpOnly`, разный `Path`/`SameSite`); заголовок `X-Requested-With:
  genodex` обязателен на любом `POST/PUT/DELETE` под `/api/`, без исключений
  (включая `/api/auth/login|register`).
- **Приёмка:** `TestAuthStatusBootstrap`, `TestAuthRegisterBootstrap`,
  `TestAuthRegisterRequiresInviteAfterBootstrap`,
  `TestAuthLoginSetsCookies`, `TestAuthRefreshRotates`,
  `TestAuthRefreshReuseIsRejected`, `TestAuthLogoutClearsSession`,
  `TestAuthPasswordChangeRequiresFull`, `TestAuthTokensCRUDRequiresFull`,
  `TestMissingCSRFHeaderIs4xx`.

## B2. MCP-контракт auth

- **Статус:** выполнено (2 коммита: middleware `RequireAPIToken` и e2e-тест на реальном `auth.Service` — см. `2026-09-22-auth-b2-mcp.md`).
- **Файлы:** `internal/mcp` или `internal/app` — где регистрируется `/mcp`
  (уточняется в плане этапа: миддлварь может жить в `internal/app`, раз она
  оборачивает `http.Handler` до `server.NewStreamableHTTPServer`, без
  изменений в `internal/mcp` вообще).
- **Приёмка:** запрос без `Authorization` — `401`, не доходит до MCP-сервера;
  валидный `gnx_...` — доходит, `OwnerFromContext` резолвится; отозванный —
  `401`; `TouchAPIToken` вызывается при успехе (обновление `last_used_at`).

## C. Проброс `Access` в делениях

- **Файлы:** `internal/usecases/list_divisions/scenario.go`,
  `internal/usecases/search_divisions/scenario.go` (+тесты — фейки получают
  ожидаемый `Access`), `internal/httpapi/deps.go`/`division.go`,
  `internal/mcp/deps.go`/`division.go`, `internal/app/app.go` (сборка
  `auth.Service`, подключение обоих middleware).
- **Решения дизайна:** §6. Поведение делений не меняется (нет `Private`) —
  меняется факт проводки параметра и коды ошибок записи.
- **Приёмка:** `TestDivisionListPassesAccessFromContext`,
  `TestDivisionCreateAnonymousIs401`,
  `TestDivisionCreateOwnerSucceeds` (httpapi и mcp); e2e на реальном сторе —
  bootstrap → создать владельца → залогиниться → создать деление → выйти →
  анонимная попытка создать → `401`.

## D. Веб

- **Файлы:** `web/src/auth.ts` (API-клиент: status/register/login/logout/
  refresh/password/invites/tokens), `web/src/pages/{Login,Register,
  Settings}.tsx`, `web/src/App.tsx` (роуты, состояние сессии в шапке,
  bootstrap-редирект).
- **Приёмка:** ручной smoke в браузере (как S16) — мастер регистрации на
  чистой БД, логин, создание invite-ссылки, регистрация второго владельца по
  ней, создание/отзыв API-токена (значение показывается один раз), попытка
  записи деления анонимом даёт видимую ошибку в UI.

## Что вне этого прохода

- Смена логина, удаление/список владельцев с ролями.
- Периодическая уборка просроченных `Session`/`Invite`.
- Rate limiting / lockout на `/api/auth/login`.
- Веб-CRUD для делений (кнопки создания/изменения/удаления во вкладке) —
  этот проход только защищает API.
- Публичные (Access-aware) срезы для людей/событий — следующий большой
  проход после этого.
