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
- Отчества/Сословия/Титулы/Имена (`Patronymic`/`Estate`/`Title`/`GivenName`) —
  ещё 4 словарные сущности по образцу Surname, полный CRUD: HTTP
  (`/api/patronymics*`, `/api/estates*`, `/api/titles*`,
  `/api/given-names*`) и MCP (`patronymic_*`/`estate_*`/`title_*`/
  `given_name_*` — по 6 тулов на сущность), веб-страницы
  (список/просмотр/редактирование/создание) с текстовыми списками
  вариантов написания, носителей и заметок; у `GivenName` дополнительно
  обязательное поле `gender` (`male`/`female`/`neutral`) — единственное
  отличие от остальных трёх
  (`internal/usecases/{list,search,get,create,update,delete}_{patronymic,estate,title,given_name}*`,
  `internal/httpapi/{patronymic,estate,title,given_name}*.go`,
  `internal/mcp/{patronymic,estate,title,given_name}.go`,
  `web/src/pages/{PatronymicsList,PatronymicView,PatronymicForm,EstatesList,
  EstateView,EstateForm,TitlesList,TitleView,TitleForm,GivenNamesList,
  GivenNameView,GivenNameForm}.tsx`).
- Хранилища/Церкви/Приходы/Архивы (`Repository`/`Church`/`Parish`/`Archive`) —
  первая волна сущностей с настоящими внешними ключами, полный CRUD: HTTP
  (`/api/repositories*`, `/api/churches*`, `/api/parishes*`, `/api/archives*`)
  и MCP (`repository_*`/`church_*`/`parish_*`/`archive_*` — по 6 тулов на
  сущность), веб-страницы (список/просмотр/редактирование/создание). Новые
  паттерны: одиночный необязательный `*TextRef` вместо списка
  (`Church.Parish`/`Parish.Church`, round-trip ref/type при неизменной
  передаче); строгий скалярный FK с проверкой существования в той же
  транзакции (`Archive.RepositoryID` → `Repository`, 422 на поле
  `repository_id`); структурированная дата с точностью — `FactDate` и
  `FactDateEditor.tsx` (`Parish.Since`/`Until`), общие для будущих
  Family/Person/Event; `Private` теперь проверяется и на чтении по id, не
  только в списке/поиске (`Repository`/`Archive`); `Sources []SourceLink` —
  read-only, переживает обновления как есть (Citation без CRUD, подпроект 5)
  (`internal/usecases/{list,search,get,create,update,delete}_{repository,
  church,parish,archive}*`, `internal/httpapi/{repository,church,parish,
  archive}*.go`, `internal/mcp/{repository,church,parish,archive}.go`,
  `web/src/pages/{RepositoriesList,RepositoryView,RepositoryForm,ChurchesList,
  ChurchView,ChurchForm,ParishesList,ParishView,ParishForm,ArchivesList,
  ArchiveView,ArchiveForm}.tsx`, `web/src/FactDateEditor.tsx`).
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
