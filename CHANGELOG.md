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
- Граф вокруг Person (`Relation`, `Residence`, `Event`+`EventParticipant`) —
  подпроект 9, backend: usecase-сценарии, HTTP (`/api/relations*`,
  `/api/residences*`, `/api/events*`) и MCP (`relation_*`/`residence_*`
  — по 5 тулов, `event_*` — 6 тулов, включая `event_search`). Три новых
  паттерна программы:
  (1) `Relation.PersonA`/`.PersonB` — первая пара строгих ссылок на ОДИН И
  ТОТ ЖЕ тип (`Person`) у одной сущности; обе проверяются независимо в
  транзакции (`create_relation`/`update_relation`), модельная `Validate()`
  дополнительно отвергает `PersonA == PersonB`;
  (2) осознанный отказ от `search_relations`/`search_residences` — у обеих
  сущностей нет собственных поисковых полей (`replaceSearchIndex(tx, …,
  nil)`, generic-слой), полнотекстовый поиск был бы структурно пустым;
  вместо него — person/place-scoped фильтрация списка (`models.
  RelationQuery`/`ResidenceQuery`/`EventQuery`, `list_relations`/
  `list_residences`/`list_events` — full-scan-and-filter по generic-окнам,
  по образцу `list_archive_nodes`, без новых методов `store.Store`);
  `Event` search остался (индексируется по началу текста `Place`, ТОЛЬКО
  это поле — не `type`/`date`);
  (3) `EventParticipant.PersonID` — первая СТРОГАЯ (проверяемая на
  существование, индексированная ошибка `participants[i].person_id`)
  ссылка внутри array-of-objects MCP/HTTP-аргумента в программе (в отличие
  от мягких/уже-установленных ссылок `PersonName`/`SourceLink`);
  `Event.Place` (`*models.PlaceRef`) — НИКОГДА не проверяется на
  существование (мягкая ссылка, тот же принцип, что и любой `TextRef`) и
  получил presence-ONLY update-guard (не `raw != nil`) как единственный
  способ очистить одиночное объектное поле явным `null`, по недавнему
  program-wide фиксу. Новый транспортный тип `transport.PlaceRef`
  (структурно как `TextRef`, но отдельный тип модели — `models.PlaceRef`)
  и `transport.EventParticipant` (плоский объект, новый файл по образцу
  `transport.PersonName`). Веб-слой — отдельная задача, вне этого прохода.
  (`internal/models/query.go` — `RelationQuery`/`ResidenceQuery`/
  `EventQuery`; `internal/transport/{relation,residence,event,
  event_participant,place_ref}{,_write}.go`; `internal/usecases/
  {list,get,create,update,delete}_relation/`, `internal/usecases/
  {list,get,create,update,delete}_residence/`, `internal/usecases/
  {list,search,get,create,update,delete}_event/`;
  `internal/httpapi/{relation,residence,event}{,_write}.go`;
  `internal/mcp/{relation,residence,event}.go`,
  `internal/mcp/object_args.go` — `placeRefObjectProperties`/
  `optionalPlaceRef`, `eventParticipantObjectProperties`/
  `optionalEventParticipants`).
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

- **Меняет поведение MCP `<entity>_update` тулов (все сущности, ~18 тулов):**
  необязательный аргумент, отсутствующий в вызове, теперь сохраняет текущее
  значение соответствующего поля записи, вместо того чтобы сбрасывать его в
  значение по умолчанию (пустая строка/пустой список/`false`). До этого
  изменения поведение было согласованным только для `sources` (подпроект 5)
  и `names` (подпроект 8) — остальные поля любого `*_update` тула заменялись
  безусловно, даже если аргумент не был передан вовсе, что создавало риск:
  AI-ассистент, вызывающий `person_update` только чтобы поправить одно имя,
  мог незаметно сделать приватную персону публичной и стереть пол/сословия/
  титулы/прозвища, поскольку каждый из этих аргументов необязателен. Найдено
  финальным ревью подпроекта 8 (`Person`), исправлено программно для ВСЕХ
  `*_update` тулов сразу, а не только для `Person`, по решению пользователя.
  Явно переданное значение (в т.ч. пустая строка/пустой список/`false`/явный
  `null`) по-прежнему заменяет поле как раньше — изменилось только поведение
  при полном отсутствии аргумента в вызове. Обязательные аргументы (`name` у
  большинства сущностей, `type`/`kind` и т.п.) не затронуты — их отсутствие
  MCP-фреймворк отклоняет до вызова обработчика. Поля с собственной бизнес-
  логикой для пустого значения (`parent_id` у `division`/`note`/
  `archive_node` — пусто значит «корень»/«без родителя»; `document_id` у
  `attachment` — пусто значит «без документа») сохранили эту логику для
  ЯВНО переданной пустой строки, изменилось только поведение при отсутствии
  ключа целиком. HTTP `PUT` не меняется — там отсутствие ключа в теле
  запроса по-прежнему полностью заменяет поле (см. `docs/usage.md`'s раздел
  про MCP-тулы для точного описания и обоснования асимметрии с HTTP).
  Затронутые файлы: все 18 `internal/mcp/*.go`, кроме `object_args.go`
  (`sources`/`names`'s уже готовый паттерн переиспользован как есть).
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
