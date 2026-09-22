# Дизайн: аутентификация и Access в контрактах

Утверждённый дизайн прохода «владелец и публичный доступ». Закрывает открытый
вопрос программы [ядро чтения и записи](core-read-write.md) — обработчики
хардкодят `models.AccessFull` вместо передачи режима доступа из запроса (см.
[implementation/core-read-write.md](../implementation/core-read-write.md) §6) —
и реализует решение #33 (`decisions.md`) на практике: сейчас оба интерфейса
(HTTP, MCP) без исключения считаются владельческими, потому что аутентификации
вообще нет. Реализуется этапами по
[дорожной карте](../plans/2026-09-22-auth-roadmap.md).

Исходники истины: [decisions.md](decisions.md) (#24, #33), `core-read-write.md`
§4 (`Access`/`Page` в порту).

## 1. Принятые решения

| # | Вопрос | Решение | Отвергнуто |
|---|---|---|---|
| 1 | Что становится «публичным» | Тот же сервер и порт; владелец аутентифицируется (логин/пароль), получает `AccessFull`; все прочие запросы — `AccessPublic` | Отдельный порт/процесс без авторизации (нет реального разделения; усложняет деплой) |
| 2 | Механизм владельца | Логин/пароль + сессия (cookie, access+refresh); для MCP/скриптов — отдельный долгоживущий API-токен | Только статический токен (не годится для веб-формы входа, которую владелец явно попросил); Basic Auth (нет разлогина/ротации) |
| 3 | Владельцев — один или несколько | Таблица владельцев, несколько строк сразу; создание — только по одноразовой invite-ссылке от существующего владельца (кроме самого первого — bootstrap) | Жёстко один владелец (не открывает совладение) |
| 4 | Где живут auth-данные | Отдельный пакет `internal/auth`: свой домен, свой узкий порт, свои таблицы. Не входит в `models.Type`/generic `store` (21 сущность, `Delete*`/`Search`/FK-граф) | Четыре новых типа в общей системе (три теста жёстко приравнивают `models.AllTypes()` и generic-реестр: `fkgraph_test.go`, `search_test.go`, `validate_coverage_test.go` — auth туда не должен затягиваться) |
| 5 | Хеш пароля | `bcrypt` (`golang.org/x/crypto/bcrypt`) — специально для паролей (медленный, соль встроена) | Сырой `SHA-256` (быстро перебирается оффлайн); ручной PBKDF2 (лишний код ради нулевой новой зависимости) |
| 6 | Формат сессионных/API-токенов | Случайная строка (32 байта `crypto/rand`, base64url); в БД — только `SHA-256`-хэш. API-токен — с узнаваемым префиксом `gnx_<...>` | Хранение токена в открытом виде (компрометация БД = компрометация всех сессий) |
| 7 | Access/refresh | Access — короткий (15 мин), refresh — длинный (30 дней, скользящий), оба в одной строке `Session`; refresh ротирует оба и инвалидирует старый refresh (повтор украденного — ошибка) | Один долгий токен без разделения (шире окно компрометации при утечке) |
| 8 | Транспорт | Две `HttpOnly` cookie: `genodex_access` (`Path=/`), `genodex_refresh` (`Path=/api/auth/refresh` — уже, чем сайт целиком) | Токен в `localStorage` (доступен JS — риск XSS) |
| 9 | CSRF | Обязательный заголовок `X-Requested-With: genodex` на `POST/PUT/DELETE` (простая защита для SPA на одном origin с cookie-сессией) | Отдельные CSRF-токены (лишняя инфраструктура для одного origin) |
| 10 | Запись у существующих контрактов | `POST/PUT/DELETE /api/admin-divisions*` и их MCP-аналоги требуют `AccessFull` → `401` анонимному | Оставить запись открытой всем (текущее поведение — случайная дыра, не решение) |
| 11 | MCP без токена | Отклонять полностью (`401`, до входа в протокол MCP) — MCP остаётся исключительно владельческим инструментом (решение #33) | Пускать как `AccessPublic` (MCP не задумывался как публичный интерфейс) |

## 2. Домен (`internal/auth`)

Свой тип `ID` (тот же видимый формат `ПРЕФИКС-ULID`, что и `models.ID`, но
генерируется независимым генератором внутри пакета — не через
`internal/idgen`, который жёстко завязан на `models.Type`/`models.BuildID`).
Префиксы: `OW` (owner), `SS` (session), `AT` (api token), `IV` (invite) —
не пересекаются с таблицей `models.idPrefixTable`.

```go
type Owner struct {
    ID           ID
    Login        string
    PasswordHash string    // bcrypt
    CreatedAt    time.Time
}

type Session struct {
    ID                ID
    OwnerID           ID
    AccessTokenHash   string
    AccessExpiresAt   time.Time
    RefreshTokenHash  string
    RefreshExpiresAt  time.Time
    CreatedAt         time.Time
}

type APIToken struct {
    ID         ID
    OwnerID    ID
    Label      string
    TokenHash  string
    CreatedAt  time.Time
    LastUsedAt *time.Time
    RevokedAt  *time.Time
}

type Invite struct {
    ID        ID
    CreatedBy ID
    TokenHash string
    ExpiresAt time.Time
    UsedAt    *time.Time
    UsedBy    *ID
    CreatedAt time.Time
}
```

`Validate() error` на каждом типе — по образцу `models`: непустой `Login`,
непустой `PasswordHash`, `OwnerID`/`CreatedBy` — валидный `ID`; ошибка —
свой `*auth.ValidationError{Field, Reason}` (не переиспользует
`models.ValidationError`, чтобы не тянуть в домен `models` целиком; единственная
точка, где пакет всё же импортирует `models`, — `ResolveAccess`, возвращающий
`models.Access` для `httpapi`/`mcp`).

Сервис (`internal/auth`, пакет-фасад над портом, аналог `usecases/*`, но один
пакет на весь маленький домен — сценариев мало и они плотно связаны):

- `Register(ctx, login, password string, invite *string) (Session, error)` —
  без владельцев в БД `invite` игнорируется (bootstrap); иначе обязателен и
  должен быть валиден/не использован/не истёк.
- `Login(ctx, login, password string) (Session, error)`.
- `Refresh(ctx, refreshToken string) (Session, error)` — ротация.
- `Logout(ctx, accessToken string) error`.
- `ResolveAccess(ctx, accessToken string) (models.Access, *OwnerID, error)` —
  читает `internal/models` только здесь, в точке пересечения с портом
  `httpapi`/`mcp` ожидают.
- `ChangePassword(ctx, ownerID ID, current, new string) error`.
- `CreateInvite(ctx, ownerID ID) (rawToken string, err error)`.
- `CreateAPIToken(ctx, ownerID ID, label string) (rawToken string, id ID, err error)`.
- `ListAPITokens(ctx, ownerID ID) ([]APIToken, error)`.
- `RevokeAPIToken(ctx, ownerID, tokenID ID) error`.
- `ResolveAPIToken(ctx, rawToken string) (OwnerID, error)` — для MCP-middleware.
- `Bootstrap(ctx) (bool, error)` — есть ли хоть один владелец.

Хеширование — `internal/auth/hash.go`: `hashPassword`/`verifyPassword`
(bcrypt), `hashToken` (`sha256`, hex), `newRawToken()` (32 байта
`crypto/rand`, base64url), `newAPIToken()` (тот же генератор + префикс
`gnx_`).

## 3. Хранилище (`internal/auth`, отдельный store-адаптер на том же файле БД)

Четыре таблицы: `owners`, `sessions`, `api_tokens`, `invites` — создаются в
той же миграции/схеме, что и остальные (`internal/storage`), но не входят в
граф внешних ключей generic-порта (`sqlstore/fkgraph.go`) и не пишут в
`search_index`. FK внутри своих четырёх таблиц (`sessions.owner_id` →
`owners.id` и т. д.) — обычные, для целостности, просто не участвуют в
общей машинерии `Delete*`.

Узкий порт (`internal/auth/deps.go`, `//go:generate mockgen`):

```go
type Store interface {
    CreateOwner(ctx context.Context, o Owner) error
    GetOwnerByLogin(ctx context.Context, login string) (*Owner, error)
    GetOwner(ctx context.Context, id ID) (*Owner, error)
    UpdateOwnerPassword(ctx context.Context, id ID, hash string) error
    CountOwners(ctx context.Context) (int, error)

    CreateSession(ctx context.Context, s Session) error
    GetSessionByAccessHash(ctx context.Context, hash string) (*Session, error)
    GetSessionByRefreshHash(ctx context.Context, hash string) (*Session, error)
    ReplaceSession(ctx context.Context, id ID, s Session) error // ротация
    DeleteSession(ctx context.Context, id ID) error

    CreateAPIToken(ctx context.Context, t APIToken) error
    GetAPIToken(ctx context.Context, id ID) (*APIToken, error) // для RevokeAPIToken: проверить владельца
    GetAPITokenByHash(ctx context.Context, hash string) (*APIToken, error)
    ListAPITokens(ctx context.Context, ownerID ID) ([]APIToken, error)
    RevokeAPIToken(ctx context.Context, id ID) error
    TouchAPIToken(ctx context.Context, id ID) error // last_used_at

    CreateInvite(ctx context.Context, i Invite) error
    GetInviteByHash(ctx context.Context, hash string) (*Invite, error)
    MarkInviteUsed(ctx context.Context, id ID, by ID) error
}
```

Просроченные `Session`/`Invite` не чистятся активно в этом проходе (истёкшая
запись просто не проходит проверку срока при чтении) — периодическая уборка,
если понадобится, — отдельная задача.

## 4. HTTP-контракт (`internal/httpapi`)

| Метод | Путь | Access | Действие |
|---|---|---|---|
| `GET` | `/api/auth/status` | любой | `{"bootstrap": bool}` |
| `GET` | `/api/auth/session` | любой | нет сессии — `401`; есть — `{"login": "..."}` (для шапки веб) |
| `POST` | `/api/auth/register` | любой | bootstrap без `invite`; иначе `?invite=` обязателен и валиден |
| `POST` | `/api/auth/login` | любой | `{login, password}` → cookies |
| `POST` | `/api/auth/logout` | Full | инвалидирует сессию, чистит cookies |
| `POST` | `/api/auth/refresh` | по refresh-cookie | ротация access+refresh |
| `POST` | `/api/auth/password` | Full | `{current_password, new_password}` |
| `POST` | `/api/auth/invites` | Full | создаёт одноразовую ссылку (7 дней) |
| `POST` | `/api/auth/tokens` | Full | создаёт `APIToken`, сырое значение — только в ответе |
| `GET` | `/api/auth/tokens` | Full | список метаданных |
| `DELETE` | `/api/auth/tokens/{id}` | Full | отзыв |

Middleware `resolveAccess` оборачивает весь `/api/`, включая `/api/auth/*`
(эти эндпоинты сами решают, что делать с `Access`/сессией — `resolveAccess`
лишь читает cookie и кладёт результат в контекст, не блокирует): читает
`genodex_access`, при валидной активной сессии кладёт в контекст запроса
`models.AccessFull` + `OwnerID`, иначе `models.AccessPublic`. Хелпер
`httpapi.AccessFromContext(ctx) models.Access` и
`httpapi.OwnerFromContext(ctx) (auth.ID, bool)`.

Отдельный middleware `requireCSRFHeader` — на **любой** `POST/PUT/DELETE` под
`/api/` без исключений (включая `/api/auth/login|register`: они тоже вызываются
JS-клиентом того же origin, а не только по cookie, так что заголовок им
доступен так же, как остальным) требует `X-Requested-With: genodex`, иначе
`400` до чтения тела.

Существующие обработчики (`handleDivisionList`, `handleDivisionSearch`)
передают `Access` из контекста в сценарий вместо хардкода. Обработчики
записи (`handleDivisionCreate/Update/Delete`) первым делом проверяют
`Access == models.AccessFull`, иначе `401 {"error": "…"}` — не вызывая
сценарий.

Коды: `401` — нет/просрочена/отозвана сессия для действия, требующего Full;
`403` не используется (в этом проекте различие «не тот» vs «не туда» не
нужно — один владелец = все права). Существующие `400/404/409/422` не
меняются.

## 5. MCP-контракт

Middleware на `/mcp` (пакет — `internal/mcp` или `internal/app`, где
регистрируется маршрут; конкретное место решает план фазы B2 — маршрут `/mcp`
сейчас собирается в `internal/app.New`, так что миддлварь естественно
оборачивает `http.Handler` там, без изменений в `internal/mcp`): читает
`Authorization: Bearer gnx_...`, резолвит через
`auth.Service.ResolveAPIToken`; нет заголовка, неверный или отозванный токен
— `401` до того, как запрос дойдёт до `server.NewStreamableHTTPServer`.
Успешная проверка — `Access=AccessFull`, `OwnerID` в контекст запроса (тот же
путь, что у httpapi, дальше сценарии не отличают источник).

## 6. Проброс `Access` через существующие контракты (закрывает исходный долг)

- `internal/usecases/list_divisions.Scenario.ListDivisions` и
  `internal/usecases/search_divisions.Scenario.SearchDivisions` получают
  `access models.Access` параметром вместо `models.AccessFull` внутри.
- `httpapi.DivisionService`/`mcp.DivisionService`: `ListDivisions`/
  `SearchDivisions` получают `Access` явным параметром (после `ctx`).
- `internal/app.divisionService`: методы `ListDivisions`/`SearchDivisions`
  прокидывают `Access` дальше в сценарий без изменений.
- Поведение сейчас не меняется (у `AdministrativeDivision` нет `Private`) —
  меняется корректность проводки параметра, ради будущих срезов
  (люди/события), где `Private` есть.

## 7. Веб (`web/src`)

- Корень приложения на старте запрашивает `/api/auth/status`:
  `bootstrap=true` → редирект на `/register` (без `invite`); иначе обычная
  загрузка. `GET /api/auth/session` определяет, показывает ли шапка «Войти»
  или логин владельца + «Настройки»/«Выйти».
- `/login` — форма логин/пароль.
- `/register` (`?invite=` опционален) — та же форма создания владельца что
  для bootstrap, что по приглашению; отсутствие обязательного `invite`, когда
  `bootstrap=false`, — ошибка формы до отправки.
- `/settings` — смена пароля; список/создание invite-ссылок (копируемый URL);
  список/создание/отзыв API-токенов (сырое значение — один раз, сразу после
  создания).
- Публичный (неаутентифицированный) посетитель видит текущие вкладки
  («Населённые пункты» и т. д.) в режиме чтения; действий записи там сейчас
  ещё нет (появятся вместе с веб-CRUD для делений отдельным проходом) —
  когда появятся, скрываются для Public на уровне UI, а не только API.

## 8. Тесты

- `internal/auth`: `Validate()` таблично; хеш/проверка пароля; генерация и
  хеширование токена (формат `gnx_`, длина, уникальность на серии — по
  образцу `idgen_test.go`); сервис на фейковом `Store` — bootstrap vs invite,
  ротация refresh, отказ повторному refresh, просроченная/отозванная сессия.
- Store-адаптер: CRUD, срок действия, отзыв — в стиле `sqlstore/*_test.go`.
- `httpapi`: fake `AuthService` для миддлвари и `/api/auth/*` (как
  `fakeDivisions`); e2e на реальном сторе (по образцу
  `TestAdminDivisionsWithRealStore`) — bootstrap → регистрация → логин →
  запись деления → анонимный `401` → invite → второй владелец.
- `mcp`: fake для bearer-проверки; `TestNewServerRejectsRequestsWithoutToken`.
- Веб: ручной smoke в браузере (как в S16): мастер регистрации, логин, invite,
  создание/отзыв токена, `401` в консоли при попытке записи анонимом.

## 9. Вне этого прохода

- Смена логина, удаление владельца, список владельцев с ролями (сейчас все
  владельцы равноправны).
- Периодическая уборка просроченных `Session`/`Invite`.
- TLS-терминация — забота деплоя (`Secure`-флаг cookie следует за `r.TLS`).
- Rate limiting / lockout на `/api/auth/login` (для одного-двух владельцев
  хобби-проекта — не приоритет сейчас; отметить как известный риск).
- Веб-CRUD для делений (кнопки создания/изменения/удаления в
  `SettlementsTab`) — отдельный проход; этот дизайн только даёт API `401`
  для анонимной записи.
