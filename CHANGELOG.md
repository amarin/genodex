# Changelog

Заметные изменения проекта фиксируются в этом файле (формат — по мотивам
[Keep a Changelog](https://keepachangelog.com/)).

## [Unreleased]

### Added

- Аутентификация владельцев: логин/пароль со скользящей cookie-сессией
  (access 15 минут / refresh 30 дней), приглашения для новых владельцев,
  долгоживущие API-токены для MCP-клиентов (`gnx_...`, показываются один
  раз). Веб-страницы `/login`, `/register`, `/settings`
  (`internal/auth`, `internal/httpapi/auth.go`, `internal/mcp/middleware.go`,
  `web/src/auth.ts`, `web/src/pages/{Login,Register,Settings}.tsx`).
- Флаг `-trust-proxy` (по умолчанию выключен): за TLS-терминирующим
  реверс-прокси (nginx, Caddy, Cloudflare) позволяет `Secure`-флагу
  cookie учитывать `X-Forwarded-Proto: https` от прокси — без флага
  `Secure` следует только за `r.TLS` (`internal/httpapi/auth.go`,
  `cmd/genodex/main.go`).
- Административное деление — вертикальный срез, полный контракт HTTP и MCP:
  список с фильтрами (`kind`, `type`), список прямых детей (`parent_id`),
  префиксный поиск по названию и вариантам названия (`division_search`,
  `GET /api/admin-divisions/search`), чтение по id, создание/изменение/удаление
  с генерацией id сервером и проверкой циклов по `parent_id`.
  (`internal/usecases/{list,search,get,create,update,delete}_division*`,
  `internal/httpapi`, `internal/mcp`).
- Веб: страница «Административное деление» — дерево от корня по всей
  иерархии (не только населённые пункты), строка поиска, переход к
  дочерним единицам, просмотр со ссылками на родителя и детей,
  создание/редактирование/удаление для вошедшего владельца (конфликт
  удаления — список ссылающихся сущностей)
  (`web/src/pages/{DivisionForm,DivisionsList,DivisionView}.tsx`,
  `web/src/api.ts`, `web/src/App.tsx`).
- Фамилии (`Surname`) — первая сущность после AdministrativeDivision,
  полный CRUD: usecase-сценарии записи, HTTP (`/api/surnames*`) и MCP
  (`surname_list/search/get/create/update/delete`), веб-страницы
  (список/просмотр/редактирование/создание) с текстовыми списками
  вариантов написания, носителей и заметок
  (`internal/usecases/{list,search,get,create,update,delete}_surname*`,
  `internal/httpapi/surname.go`, `internal/mcp/surname.go`,
  `web/src/pages/{SurnamesList,SurnameView,SurnameForm}.tsx`).
- Веб: единая точка входа `/` — каталог подключённых сущностей по
  алфавиту (Административное деление, Документация, Фамилии), вместо
  прежних вкладок; хлебные крошки от корня на каждой странице
  (`web/src/pages/EntityCatalog.tsx`, `web/src/App.tsx`).
- Юлианский/григорианский календарь в `FactDate` (поле `Calendar`, сравнение
  дат разных календарей через юлианский день, суффиксы `ст. ст.`/`н. ст.` при
  разборе и выводе).
- Формат идентификаторов `ПРЕФИКС-ULID` для всех 21 типов сущностей и генератор
  (`internal/idgen`).
- `Validate() error` на каждой доменной сущности и value-типе
  (`*models.ValidationError`).
- Индексы по внешним ключам, генерируемые из схемы (`PRAGMA
  foreign_key_list`), и поисковый индекс для префиксного поиска
  (`idx_search_term`).
- Порт хранилища: `ErrNotFound` для отсутствующих сущностей, `ctx` во всех
  методах, `Delete*` для всех 21 типов (общая реализация на графе внешних
  ключей, `*InUseError` со списком ссылающихся), `InTx` для атомарной записи
  нескольких сущностей, `Page`/`Access` (публичный/полный доступ) во всех
  списках, пакетная загрузка (без N+1) для людей и административных делений,
  `Search` и `ChildrenOfDivision`.

### Changed

- **Ломает совместимость:** запись в `/api` (`POST`/`PUT`/`DELETE
  /api/admin-divisions...`) и весь `/mcp` теперь требуют аутентификации
  владельца — анонимные запросы получают `401`. При первом запуске сервер
  переходит в режим bootstrap: откройте `/register` в браузере, чтобы
  создать первого владельца; для MCP-клиентов создайте API-токен на
  `/settings` и передайте его как `Authorization: Bearer gnx_...`.
- Переименован единственный публичный контракт, существовавший до этого
  прохода: `settlement_list` / `GET /api/settlements` → `division_list` /
  `GET /api/admin-divisions`; DTO `Settlement` → `AdminDivision`. Фильтр
  «только населённые пункты» стал параметром запроса (`kind=settlement`), а не
  отдельным именем контракта.
- `internal/definitions/russia`: константы двух исторических систем деления
  (Российская империя, СССР) используют канонические значения
  `models.AdminDivisionType` (`governorate`/`district`/`volost`) вместо
  русских слов для показа, не проходивших `AdminDivisionType.Valid()`.

### Removed

- Мёртвый тип `models.AdministrativeDivisionTypeRelation` (не имел ни одного
  потребителя в кодовой базе).
