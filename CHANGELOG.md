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
- Заметки/Вложения (`Note`/`Attachment`) — self-ref и подпроектная FK-на-
  сущность-без-CRUD, полный CRUD: HTTP (`/api/notes*`, `/api/attachments*`)
  и MCP (`note_*`/`attachment_*` — по 6 тулов на сущность), веб-страницы
  (список/просмотр/редактирование/создание). Новые паттерны: self-ref
  строгий FK с проверкой существования на create и обходом цепочки
  родителей на цикл на update (`Note.ParentID`); обязательный/
  необязательный строгий FK на сущность без своего CRUD-слоя, существование
  проверяется через generic-хранилище (`Attachment.NodeID`/`DocumentID` →
  `ArchiveNode`/`ArchiveDocument`, появятся в подпроекте 6)
  (`internal/usecases/{list,search,get,create,update,delete}_{note,
  attachment}*` (note usecases named `list_notes`/`search_notes`/`get_note`/
  `create_note`/`update_note`/`delete_note`, attachment usecases named
  `list_attachments`/`search_attachments`/`get_attachment`/
  `create_attachment`/`update_attachment`/`delete_attachment`),
  `internal/httpapi/{note,attachment}*.go`, `internal/mcp/{note,
  attachment}.go`, `web/src/pages/{NotesList,NoteView,NoteForm,
  AttachmentsList,AttachmentView,AttachmentForm}.tsx`).
- Источники/Цитаты (`Source`/`Citation`) — цепочка доказательств, полный
  CRUD: HTTP (`/api/sources*`, `/api/citations*`) и MCP (`source_*`/
  `citation_*` — по 6 тулов на сущность), веб-страницы (список/просмотр/
  редактирование/создание). Первый полиморфный тип в программе
  (`Citation.Anchor` — архивный узел/документ, файл-вложение или внешняя
  ссылка; плоское представление с дискриминатором `kind`, по образцу
  `FactDate`, не вложенный union) и первый MCP-аргумент вида «массив
  объектов» (`sources`, см. ниже — раньше массивы были только строками)
  (`internal/transport/{source,source_write,citation,citation_write,
  anchor}.go`, `internal/usecases/{list,search,get,create,update,delete}_
  {source,citation}*`, `internal/httpapi/{source,citation}*.go`,
  `internal/mcp/{source,citation}.go`, `internal/mcp/object_args.go`
  (`anchorObjectProperties`/`optionalAnchor`,
  `sourceLinkObjectProperties`/`optionalSourceLinks`), `web/src/
  AnchorEditor.tsx`, `web/src/SourceLinkList.tsx`, `web/src/pages/
  {SourcesList,SourceView,SourceForm,CitationsList,CitationView,
  CitationForm}.tsx`).
- `Sources []SourceLink` разблокирован для редактирования у всех 6
  сущностей, где есть: `AdministrativeDivision` (впервые видим на чтении
  тоже — раньше отсутствовал в контракте вовсе), `Repository`, `Church`,
  `Parish`, `Archive`, `Note`. Несуществующий `citation_id` в списке
  `sources` — 422 на поле `sources[i].citation_id` (проверка в той же
  транзакции, что и сохранение — `Repository`/`Church`/`Parish` при этом
  впервые стали транзакционными сценариями, раньше у них не было ни
  одного FK для проверки) (`internal/transport/{admin_division,
  admin_division_write,repository_write,church_write,parish_write,
  archive_write,note_write}.go`, `internal/usecases/{create,update}_
  {division,repository,church,parish,archive,note}/*`, `internal/httpapi/
  {division,repository,church,parish,archive,note}_write.go`,
  `internal/mcp/{division,repository,church,parish,archive,note}.go`,
  `web/src/pages/{Division,Repository,Church,Parish,Archive,Note}
  {Form,View}.tsx`).
- Архивные деревья (`ArchiveNode`/`ArchiveDocument`) — подпроект 6, полный
  стек: usecase-сценарии, HTTP (`/api/archive-nodes*`, `/api/archive-
  documents*`) и MCP (`archive_node_*`/`archive_document_*` — по 6 тулов на
  сущность). `ArchiveNode` — первое дерево, скопированное per-owner
  (обязательный `ArchiveID`), а не единственное глобальное, как у
  `AdministrativeDivision`: список без `ParentID` — корень внутри архива,
  без выделенного метода хранилища «дети узла» — полный проход по окнам
  generic `ListArchiveNodes` с фильтром по `ArchiveID`+`ParentID` (масштаб
  данных это позволяет). Новый вид кросс-полевой проверки — «родитель
  существует И принадлежит тому же архиву, что и сам узел», не только «X
  существует»: несуществующий/чужого-архива `parent_id` — 422 на поле
  `parent_id`, отдельно от 422 на `archive_id` (несуществующий архив);
  `archive_id` самого узла неизменен при обновлении (перенос узла между
  архивами запрещён — иначе дочерние узлы молча пропадают из обоих
  деревьев). `ArchiveDocument` — плоская сущность со строгим `UnitID` на
  `ArchiveNode`. Обе — первые совершенно новые сущности программы,
  получившие `Sources []SourceLink` сразу редактируемым (не ретрофит, как у
  6 сущностей выше). Веб: страницы «Архивные единицы»/«Архивные документы»
  (дерево узлов по архиву, список/просмотр/редактирование/создание
  документов); новые переиспользуемые компоненты `ArchiveNodePicker`/
  `ArchiveDocumentSelect` (archive-select-then-tree модалка — выбор архива,
  затем дерево его узлов; каскадный `Select` документов внутри выбранного
  узла), заменившие собой ретрофитом обычные текстовые поля ввода id в
  `AttachmentForm`/`AttachmentView` и `AnchorEditor`; ссылка «Архивные
  единицы →» на `ArchiveView`, ведущая прямо в дерево своего архива
  (`internal/models/query.go` (`ArchiveNodeQuery`), `internal/usecases/
  {list,search,get,create,update,delete}_archive_{node,document}*`,
  `internal/transport/archive_{node,document}{,_write}.go`,
  `internal/httpapi/archive_{node,document}{,_write}.go`,
  `internal/mcp/archive_{node,document}.go`, `web/src/ArchiveNodePicker.tsx`,
  `web/src/pages/{ArchiveNodesList,ArchiveNodeView,ArchiveNodeForm,
  ArchiveDocumentsList,ArchiveDocumentView,ArchiveDocumentForm}.tsx`,
  `web/src/pages/ArchiveView.tsx`, `web/src/pages/{AttachmentForm,
  AttachmentView}.tsx`, `web/src/AnchorEditor.tsx`).
- Роды/линии (`Family`) — подпроект 7, полный стек: usecase-сценарии, HTTP
  (`/api/families*`) и MCP (`family_list/search/get/create/update/delete`).
  Самая механическая сущность программы на сегодня — структурно та же
  форма, что `Repository` (подпроект 3) минус `Type`/`Address`, с `URLs`,
  переименованным в `Members`; ни одного нового паттерна не потребовалось.
  `Members` — мягкая ссылка на `Person` (`TypePerson`, без CRUD до
  подпроекта 8): `TextRef.Ref` не проверяется на существование при
  сохранении, как и у любого другого списка `TextRef` в программе.
  `Sources` редактируемый с рождения контракта (как `ArchiveNode`/
  `ArchiveDocument` из подпроекта 6), не ретрофит. Веб: страница «Роды» —
  список/просмотр/редактирование/создание с `TextRefListEditor` для
  `Members`/`Notes` и `SourceLinkListEditor` для `Sources`; строка «Роды» в
  каталоге сущностей (`internal/transport/family{,_write}.go`,
  `internal/usecases/{list,search,get,create,update,delete}_family*`
  (пакеты `list_families`/`search_families`/`get_family`/`create_family`/
  `update_family`/`delete_family`), `internal/httpapi/family{,_write}.go`,
  `internal/mcp/family.go`, `web/src/pages/{FamiliesList,FamilyForm,
  FamilyView}.tsx`, `web/src/App.tsx`, `web/src/pages/EntityCatalog.tsx`).
- Персоны (`Person`) — подпроект 8: ядро
  графа генеалогии, usecase-сценарии, HTTP (`/api/people*`) и MCP
  (`person_list/search/get/create/update/delete`). Единственная строго
  проверяемая ссылка — `Sources[i].CitationID`, как и везде; все прочие
  ссылки (`Names[i].Surname/.Given/.Patronymic` — на `Surname`/`GivenName`/
  `Patronymic`, `Estates`/`Titles` — на словари, `Nicknames`) — мягкие
  `TextRef`, существование не проверяется, несмотря на то что у первых трёх
  словарей есть собственный CRUD с подпроектов 1-2. `PersonName` — первая
  вложенная подформа-«массив объектов» в программе: `transport.PersonName`
  (новый файл, по образцу `transport.Anchor`) переиспользует существующие
  `TextRef`/`FactDate` без нового кода value-уровня; MCP-аргумент `names`
  — второй массив объектов в программе (после `sources`, подпроект 5) и
  первый, чьи элементы сами несут вложенные объекты (`personNameObjectProperties`
  ссылается на `textRefObjectProperties`/`factDateObjectProperties`).
  `person_update` расширяет presence-check «отсутствие аргумента сохраняет
  текущее значение» (прежде — только у `sources`) также на `names`.
  Наименование зеркалирует иррегулярное множественное число хранилища
  (`store.Store.ListPeople`): usecase-пакеты `list_people`/`search_people`
  (иррегулярное множественное), `get_person`/`create_person`/
  `update_person`/`delete_person` (единственное число) — но MCP-тулы и
  HTTP-путь `/api/people` следуют общему для программы правилу (единое
  число сущности как префикс тула, независимо от иррегулярности
  хранилища). Поиск (`person_search`, `GET /api/people/search`) ищет по
  началу фамилии/имени/отчества из ЛЮБОГО элемента `Names`, не только
  основного (`personTerms`, уже существовавший generic-код), что
  доказывается автоматически (`TestSearchPeopleFindsByMarriedNameOnly`,
  `TestPersonWriteContractWithRealStore`)
  (`internal/transport/{person,person_name,person_write}.go`,
  `internal/usecases/{list,search}_people/`, `internal/usecases/
  {get,create,update,delete}_person/`, `internal/httpapi/person{,_write}.go`,
  `internal/mcp/person.go`). Веб: новый переиспользуемый паттерн
  `PersonNameListEditor` (`web/src/PersonNameList.tsx`) — первая в
  программе повторяющаяся форма-редактор со strong-typed baseline-per-mount
  ref-preservation (сохраняет `ref`/`type` поля `surname`/`given`/
  `patronymic`, только если их текст не изменился по сравнению со значением
  на момент монтирования компонента, по строке — независимо друг от
  друга); страницы «Персоны» — список/просмотр/редактирование/создание
  (`web/src/pages/{PeopleList,PersonForm,PersonView}.tsx`), поля `gender`
  (`Select`) и `names` добавлены поверх существующего паттерна
  `estates/titles/nicknames/notes`+`sources`+`private`; строка «Персоны» в
  каталоге сущностей (`web/src/api.ts`, `web/src/App.tsx`,
  `web/src/pages/EntityCatalog.tsx`).
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
