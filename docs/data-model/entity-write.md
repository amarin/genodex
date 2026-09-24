# Запись (create/update/delete), MCP и веб-UI для всех сущностей

Дизайн-документ. Описывает, как из уже готового generic-слоя хранения
(`internal/store` — `Get*/Save*/List*/Delete*/Search` для всех 21 типа,
общий граф внешних ключей, общий upsert, общий поисковый индекс) вырастить
полный стек «запись + MCP-тулы + веб-UI» для каждой из 21 сущности. Первая
сущность (`AdministrativeDivision`) уже прошла этот путь целиком
(`docs/plans/2026-09-23-web-divisions-crud.md`) и служит эталоном.

Motивация: `AGENTS.md` описывает generic-слой хранения как уже готовый —
но выше него (usecase-сценарии записи, транспортные DTO, `httpapi`-роуты,
MCP-тулы, веб-страницы) есть только у одной сущности из 21. Цель — довести
это до «не самого удобного, но полностью работоспособного» интерфейса ввода
и управления данными для всех сущностей, доступного и человеку в браузере,
и агенту через MCP.

## 1. Инвентаризация

Обследование репозитория (2026-09-23) подтвердило:

- **Generic-слой хранения уже полон** для всех 21 типа (`internal/models/id_prefix.go:AllTypes()` —
  источник истины списка типов): `Get*/Save*/List*/Delete*` в
  `internal/store/deps.go`, реализация в `internal/store/sqlstore`. `Save*` —
  upsert, полностью заменяющий сущность и её дочерние value-строки при каждом
  вызове (не частичный patch). `Delete*` — общая реализация на графе внешних
  ключей (`internal/store/sqlstore/fkgraph.go`, `delete.go`): чистит
  search_index/source_links/осиротевшие value-строки, `SET NULL` для мягких
  ссылок, `*models.InUseError` при строгих ссылающихся. `Search` — общий
  префиксный поиск по `search_index` (`idx_search_term`), индекс наполняется
  generic-кодом при каждом `Save*` (`internal/store/sqlstore/helpers.go`).
- **Чего нет ни у одной сущности, кроме AdministrativeDivision**: usecase-
  сценариев записи (`internal/usecases/{create,update,delete}_<entity>`),
  транспортных DTO для записи, `httpapi`-роутов записи, MCP-тулов записи,
  веб-страниц. Это довольно механический каркас по образцу деления — не
  новая генерik-инфраструктура.
- **Транспортного DTO для `TextRef`** (`{Text, Ref ID, Type Type}` —
  элемент списков вроде `Surname.Variants`) нет вообще нигде — появляется
  впервые в этом проходе (у AdministrativeDivision таких полей не было).

## 2. Декомпозиция на подпроекты (согласовано с пользователем)

Порядок — от простого к сложному: сначала дёшево отладить паттерн, потом
применять его почти механически, самые связанные (граф вокруг Person)
проекты — последними, когда уже есть на что ссылаться (View-страницы
Person и др.).

1. **Паттерн** (этот документ + первый план) — на `Surname`: полный стек
   для одной простой словарной сущности, тщательно спроектированный и
   проверенный. Задаёт конвенции для всех следующих.
2. **Остальные словари** — `GivenName`, `Patronymic`, `Estate`, `Title`.
   Структурно почти идентичны `Surname` — один план-документ на всю
   группу, один SDD-диспетч на группу (batch same-shape work), без
   отдельного brainstorming на каждую.
3. **Одиночный-FK уровень** — `Repository`, `Church`, `Parish`, `Archive`.
   Один strict FK (`RepositoryID` и т.п.) или soft-ref. Один план на
   группу.
4. **Self-ref / soft-ref** — `Note` (self-ref `ParentID`), `Attachment`
   (`DocumentID *ID`, SET NULL). Один план на группу.
5. **Цепочка доказательств** — `Source`, `Citation`. Тесно связаны
   (Source → Citation → SourceLink из всех остальных сущностей) — вместе,
   один план.
6. **Архивные деревья** — `ArchiveNode` (переиспользуем tree-паттерн
   `DivisionsList`/`DivisionView` — рекурсивный `ParentID`, тот же
   `antd Tree` + `loadData`), `ArchiveDocument` (picker в это дерево).
   Один план на группу.
7. **Family**.
8. **Person** — ядро графа: `[]PersonName` (вложенная подформа),
   `[]TextRef` (evidence). Отдельный план — сложнее остальных, но не
   входит в группу.
9. **Граф вокруг Person** — `Relation` (2 строгих FK на Person),
   `Residence` (`PersonID`+`PlaceID`), `Event`+`EventParticipant`
   (участники — ссылки на Person). Последними: всем нужен person-picker,
   который имеет смысл строить, когда у Person уже есть List/View, на
   которые он ссылается. Один план на группу (возможно, придётся разбить
   дальше при детальном brainstorming — решим на месте).

Каждая группа (2-9) получает один план-документ и один SDD-проход
(«batch small same-shape work» — один имплементер на всю группу, не по
одному на сущность), без отдельного цикла brainstorming — дизайн уже
решён этим документом. Если при выполнении какой-то группы вскроется, что
сущность внутри неё на самом деле сложнее, чем предполагалось (как уже
было с `DivisionsList`/`parent_id` в прошлом проходе) — это решается
рулингом контроллера по ходу SDD, а не новым циклом brainstorming.

## 3. Бэкенд-конвенции (образец: `division_*`, применяется ко всем)

Для сущности `<entity>` (пример — `surname`/`Surname`):

- **Usecase-сценарии** — `internal/usecases/{list,search,get,create,update,delete}_<entity>(s)/`.
  - `create_<entity>`: генерирует ID (`IDGenerator`, как в `create_division`),
    валидирует через `<Entity>.Validate()`, вызывает `store.Save<Entity>`.
    У сущностей без FK на другую сущность (словари) — без проверки
    родителя/циклов, которая есть у `create_division`. У сущностей с FK
    (уровни 3+) — добавляется проверка существования цели FK, по образцу
    `create_division`'s `parentErr`.
  - `update_<entity>`: читает текущую версию (`Get<Entity>`), накладывает
    новые поля, валидирует, `Save<Entity>` — полная замена, не patch (см. §1).
  - `delete_<entity>`: тонкая обёртка над `store.Delete<Entity>` — вся
    логика (в т.ч. `InUseError`) уже в generic-слое.
  - `list_<entity>(s)`/`get_<entity>`/`search_<entity>(s)` — тонкие обёртки
    над уже существующими `List<Entity>s`/`Get<Entity>`/generic `Search`
    (фильтрация `Search` по типу — как у `search_divisions`).
- **Транспорт** — `internal/transport/<entity>.go` (read DTO + конвертер)
  и `<entity>_write.go` (Create/Update DTO). Общий новый тип
  `transport.TextRef{Text string; Ref string; Type string}` (пустые
  `Ref`/`Type` — просто текст) — заводится один раз в этом проходе,
  переиспользуется везде, где встречается `[]models.TextRef`.
- **`httpapi`** — `GET /api/<entities>[?...]`, `GET /api/<entities>/search`,
  `GET /api/<entities>/{id}`, `POST /api/<entities>`,
  `PUT /api/<entities>/{id}`, `DELETE /api/<entities>/{id}`. Запись —
  `requireFull`, как у делений. Регистрируется в общем `NewAPIHandler`
  mux (`internal/httpapi/httpapi.go`), не отдельным хендлером.
- **MCP** — тулы `<entity>_list`, `<entity>_search`, `<entity>_get`,
  `<entity>_create`, `<entity>_update`, `<entity>_delete` — та же схема
  имён и авторизации (`mcp.RequireAPIToken`), что у `division_*`.

### 3.1. Подпроект 3 (одиночный FK: `Repository`/`Church`/`Parish`/`Archive`) — новые паттерны

Первая группа с настоящими внешними ключами добавила к конвенциям §3-4
несколько паттернов, которые подпроекты 4-9 переиспользуют как есть.

- **Одиночный необязательный `*TextRef` (не список)** — `Church.Parish`,
  `Parish.Church`. В отличие от списков `TextRef` (§4, v1-ограничение
  ниже), веб-форма (`ChurchForm.tsx`/`ChurchView.tsx`) редактирует только
  текстовую часть; при сохранении существующий `ref` сохраняется, только
  если текст не изменился по сравнению с загруженным значением — иначе
  `ref` сбрасывается (элемент снова становится «чистым текстом»). Сам
  транспортный тип (`transport.TextRef.Model()`) при этом ничего не
  теряет — он честно round-trip'ит `ref`/`type`, если их передать; именно
  поэтому MCP-клиент, отправляющий обратно `ref`/`type`, полученные из
  `church_get`/`parish_get`, ссылку сохраняет (см. пересмотр v1-ограничения
  ниже, §5).
- **Строгий скалярный FK с проверкой существования** (`Archive.RepositoryID`
  → `Repository`) — проверка на уровне usecase, в той же транзакции
  хранилища, что и сохранение (образец: `create_archive`/`update_archive`),
  результат — 422 на поле FK (`repository_id`), не общий `*InUseError`. На
  веб-стороне — searchable `Select`, заполняемый списком целевой сущности
  (лимит 500 записей — известное ограничение v1, не настоящий picker;
  настоящий picker по-прежнему отложен до подпроекта 9, см. §5).
- **`FactDate`/`FactDateEditor.tsx`** — структурированная дата с точностью
  (`precision: unknown/year/month/day`, `modifier`, `calendar`,
  `year_to`/`month_to`/`day_to` при `modifier=between`), впервые
  использована в `Parish.Since`/`Until`. Компонент и хелпер
  `formatFactDate` рассчитаны на переиспользование как есть в
  Family/Person/Event (подпроекты 7-9), без переделки.
- **`Sources []SourceLink`** — есть у любой сущности со ссылками на
  доказательства. До подпроекта 5 было read-only: Create/Update DTO это
  поле не принимали, fetch-then-merge в `update_<entity>` его не трогал —
  значение молча переживало любое обновление как есть (осознанное решение,
  пока у `Citation` не было CRUD). С подпроекта 5 (`Citation` теперь имеет
  CRUD) поле редактируемое у всех шести ретрофитнутых сущностей
  (`AdministrativeDivision`/`Repository`/`Church`/`Parish`/`Archive`/`Note`)
  — подробности контракта и проверки `citation_id` см. §3.3 ниже;
  MCP/HTTP-семантика «отсутствие поля» (MCP сохраняет текущие источники,
  HTTP полностью заменяет) описана в `docs/usage.md`.
- **`Private` действует и на чтении по id, не только в List/Search.**
  Любая будущая сущность с полем `Private bool` обязана прокидывать
  `access models.Access` в свой usecase-уровневый `Get<Entity>` и
  проверять `if rec.Private && access != models.AccessFull { return
  models.ErrNotFound }` — тот же принцип «прячем как отсутствующее», что
  уже применяется в `List`/`Search`. Это было упущено при первом
  появлении `Private` у `Repository`/`Archive` в этом подпроекте и
  исправлено финальным ревью (эталонная реализация —
  `internal/usecases/get_repository/scenario.go` и
  `internal/usecases/get_archive/scenario.go`).

### 3.2. Подпроект 4 (self-ref: `Note`, `Attachment`) — новые паттерны

- **Self-ref строгий FK с обходом цепочки на цикл** (`Note.ParentID` →
  другая `Note`, `ON DELETE RESTRICT`). `create_note` проверяет
  существование `parent_id` в той же транзакции, что и сохранение (образец:
  `create_division`) — на create цикл невозможен, новый id ещё ничьим
  предком быть не может. `update_note` ДОПОЛНИТЕЛЬНО обходит полную цепочку
  родителей (`checkParentChain`, образец: `update_division`), чтобы
  поймать переустановку `parent_id` в цепочку собственных потомков —
  прямую или транзитивную.
- **Строгий FK на сущность без своего CRUD-слоя (на момент этого
  подпроекта).** `Attachment.NodeID` (обязателен) и `Attachment.DocumentID`
  (необязателен, `ON DELETE SET NULL`) ссылаются на
  `ArchiveNode`/`ArchiveDocument`, у которых на момент подпроекта 4 ещё не
  было usecase/httpapi/mcp-слоя — но generic-хранилище (`store.Store`,
  `internal/store/deps.go`) уже умеет читать любую сущность по id
  независимо от готовности её orchestration-слоя. `create_attachment`/
  `update_attachment` вызывают `tx.GetArchiveNode`/`tx.GetArchiveDocument`
  напрямую, в той же транзакции, что и сохранение — 422 на поле
  `node_id`/`document_id`. Веб-форма в v1 была обычными текстовыми полями
  ввода id (без `Select`/picker'а); подпроект 6 добавил CRUD-слой для
  `ArchiveNode`/`ArchiveDocument` и ретрофитнул форму на
  `ArchiveNodePicker`/`ArchiveDocumentSelect` — подробности в §3.4/§3.5.
- **Осторожно с описанием search.** Полнотекстовый поиск индексирует
  только те поля, что явно переданы в `replaceSearchIndex` при сохранении
  (см. `internal/store/sqlstore`), а не все текстовые поля модели — для
  `Note` это только `title` (не `text`), для `Attachment` — `filename` и
  `uri` (не `note`). Формулировки в MCP-описаниях/doc-комментариях/
  веб-плейсхолдерах должны буквально совпадать со списком полей в
  `replaceSearchIndex`, иначе описание вводит в заблуждение (найдено
  финальным ревью подпроекта 4 — было "по началу заголовка или текста" при
  индексации только `title`).

### 3.3. Подпроект 5 (цепочка доказательств: `Source`, `Citation`) — новые паттерны

- **Полиморфный тип.** `Citation.Anchor` — первый полиморфный тип в
  программе: интерфейс `models.Anchor` с тремя реализациями
  (`ArchiveAnchor`/`FileAnchor`/`URLAnchor`) или `nil`. Контракт
  (`transport.Anchor`) — плоское представление с дискриминатором `kind` и
  полями всех вариантов вместе (по образцу `FactDate`), а не вложенный
  union — проще для JSON REST и MCP-объектного аргумента; конвертация
  туда/обратно через `switch v := a.(type)`. Ссылки внутри якоря
  (`ArchiveAnchor.NodeID`/`DocumentID`, `FileAnchor.AttachmentID`)
  проверяются в той же транзакции, что источник (`SourceID`) и
  сохранение — `URLAnchor` ссылок не несёт, проверяется только
  структурно (`models.Citation.Validate()`).
- **MCP-аргумент вида «массив объектов».** До этого прохода
  MCP-аргументы-массивы были только строками (`mcp.WithStringItems()`).
  `sources` — первый массив объектов: `mcp.WithArray("sources",
  mcp.Items(map[string]any{"type": "object", "properties":
  sourceLinkObjectProperties()}), ...)` — `mcp.Items` принимает
  произвольную JSON-schema, не только примитивы. Чтение — `json.Marshal`
  сырого `[]any` из аргументов тула, затем `json.Unmarshal` в
  `[]transport.SourceLink` (см. `optionalSourceLinks`,
  `internal/mcp/object_args.go`) — тот же приём, что `optionalTextRef`/
  `optionalFactDate` для одиночных объектов, просто для среза.
- **Строгая ссылка внутри списка, с индексом в пути ошибки.**
  `SourceLink.CitationID` — обязательная ссылка на `Citation` у каждого
  элемента `Sources []SourceLink`. Проверяется в той же транзакции, что и
  сохранение владельца, ошибка — `*models.ValidationError` на поле
  `sources[%d].citation_id` (индекс элемента, не общее поле `sources`) —
  тот же путь, что модельная `Validate()` уже строит для структурных
  ошибок (`validateSourceLinks`/`indexed`), только уровнем выше
  (существование, а не форма). `SourceLink.TargetType`/`TargetID` клиент
  никогда не отправляет — сервер подставляет владельца из контекста при
  сохранении (`sqlstore.replaceSourceLinks`/`loadSourceLinks`), контракт
  (`transport.SourceLink.Model()`) их не заполняет.
- **Ретрофит read-only → editable может вскрыть отсутствие транзакции.**
  `Repository`/`Church`/`Parish` до этого прохода не имели ни одного FK —
  их `create_*`-сценарии делали плоский `store.SaveX(...)` без `InTx`
  вовсе. Как только у сущности появляется первая проверка существования
  (здесь — `sources[i].citation_id`), сценарий необходимо перевести на
  транзакционный `InTx`-паттерн (образец: `create_archive`) — само поле
  `Store`-зависимости (`deps.go`) меняет форму (`SaveX(ctx, *models.X)
  error` → `InTx(ctx, fn func(store.Store) error) error`). Проверить это
  явно для любой будущей сущности, получающей свой первый FK не при
  первом появлении, а позже, ретрофитом.
- **Read-контракт может отставать от read-only поля, даже когда пишущий
  контракт уже минимален осознанно.** `AdministrativeDivision` — самый
  узкий DTO в программе (только `name`/`type`/`parent_id`, с подпроекта
  1) — но `Sources` в нём отсутствовал не по тому же осознанному решению
  минимализма, что `Items`/`Variants`/`Notes` и др., а просто потому что
  поле `Sources` появилось в модели уже ПОСЛЕ того, как DTO был
  зафиксирован (подпроект 3), и контракт `AdministrativeDivision`
  никогда не пересматривался вместе с добавлением `Sources` другим
  сущностям. Обнаружено живой проверкой ретрофита (см. подпроект 5's
  предпосылку), не заранее. При добавлении нового поля, которое должно
  появиться у уже существующих сущностей «везде, где есть», явно
  сверять КАЖДУЮ такую сущность на предмет «а её read-контракт вообще
  видит это поле» — узкий DTO по решению и узкий DTO по недосмотру
  выглядят одинаково снаружи.

### 3.4. Подпроект 6 (архивные деревья: `ArchiveNode`, `ArchiveDocument`) — новые паттерны

- **Дерево, скопированное по обязательному внешнему владельцу, а не одно
  глобальное.** `AdministrativeDivision` — единственное дерево на весь
  сервис (`ParentID *ID`, без другого обязательного контекста).
  `ArchiveNode` — то же самое рекурсивное `ParentID`, но каждый узел ещё
  обязан принадлежать одному `Archive` (`ArchiveID ID`, обязателен) —
  дерево на самом деле много: по одному на архив. Отсюда новый тип запроса
  `models.ArchiveNodeQuery{ArchiveID, ParentID, Page}` (не переиспользуешь
  `DivisionQuery` — у него нет и не должно быть обязательного внешнего
  владельца) — `ArchiveID` обязателен всегда (узел бессмысленен вне
  архива), `ParentID` — как у делений: `nil` — корень (но корень внутри
  архива, не всего дерева), иначе — прямые дети. У `AdministrativeDivision`
  есть выделенный `store.ChildrenOfDivision` (унаследован от версии
  проекта до этой программы записи); заводить его аналог для `ArchiveNode`
  не стали — `list_archive_nodes.ListArchiveNodes` делает один проход по
  generic-окнам `ListArchiveNodes(ctx, access, page)`, фильтруя каждый
  узел по `ArchiveID == q.ArchiveID && ParentID == q.ParentID` (nil-safe
  сравнение). При объёме данных этой сущности (архивные деревья одного
  пользователя, не миллионы строк) полное сканирование окнами по
  `MaxPageLimit` — приемлемая цена за то, чтобы не трогать порт
  `store.Store` ради единственного потребителя. Правило на будущее: новый
  выделенный метод хранилища заводится только тогда, когда
  full-scan-and-filter в usecase перестаёт тянуть по объёму данных, не
  заранее.
- **Кросс-полевая проверка «чужой FK» — не только «существует ли X», а
  «согласуется ли FK у X с FK у меня».** Все проверки существования до
  этого подпроекта были одной формы: цель по id есть — ок, нет — 422.
  Здесь добавился новый вид: `ArchiveNode.ParentID`, если задан, должен
  указывать на узел, чей СОБСТВЕННЫЙ `ArchiveID` совпадает с `ArchiveID`
  создаваемого/изменяемого узла. Родитель может существовать и быть
  совершенно корректным узлом — и всё равно быть отвергнут, если он из
  другого архива. Ошибка — тоже `*models.ValidationError` на поле
  `parent_id` (не на `archive_id`: сам архив существует, инвариант — про
  родителя), но с другим текстом, чем «родитель не найден» (см.
  `create_archive_node`/`update_archive_node`). Проверяется только
  непосредственный родитель, не вся цепочка предков — по индукции
  (каждое звено проверено при своём сохранении) цепочка целиком
  оказывается согласованной, cascade-проверка не нужна — тот же принцип
  «без каскадной валидации», что и везде в этой программе. Индукция
  держится только потому, что `ArchiveID` узла неизменяем после создания
  (`update_archive_node` отвергает попытку его сменить, поле `archive_id`,
  ещё до проверки родителя) — будь `ArchiveID` изменяемым, родителя можно
  было бы перенести в другой архив, ни разу не тронув `ParentID` ни у
  одного ребёнка, и вся цепочка потомков молча перестала бы совпадать по
  архиву со своим (никуда не переехавшим) предком, не задев ни одну из
  проверок «непосредственного родителя». Ровно эта дыра и была найдена
  финальным ревью подпроекта 6, закрыта неизменяемостью `ArchiveID`.
  `update_archive_node` при этом всё равно обходит цепочку родителей
  целиком (`checkParentChain`, по образцу `update_note`) — но за циклом, не
  за принадлежностью архиву.
- **`Sources` редактируемый с рождения, не ретрофит.** Все шесть сущностей
  из §3.3 получили `Sources []SourceLink` в контракте задним числом — до
  подпроекта 5 у них было read-only-переживание поля. `ArchiveNode` и
  `ArchiveDocument` — первые две совершенно новые сущности программы
  (появившиеся уже после того, как `Citation` получил CRUD) — их
  Create/Update DTO несут `sources` с первого дня, никакого read-only-этапа
  не было.

### 3.5. Подпроект 6 (веб): `ArchiveNodePicker`/`ArchiveDocumentSelect`

- **Archive-select-then-tree модалка.** Поскольку `ArchiveNode` — не одно
  глобальное дерево, а по одному на архив (§3.4), выбор узла не может быть
  просто деревом — сперва нужен архив. `ArchiveNodePicker` (`web/src/
  ArchiveNodePicker.tsx`) — кнопка, открывающая `Modal`: если `archiveId` не
  передан пропом, сначала `Select` архива (через `useArchiveOptions`), и
  только после выбора рендерится `Tree`, scoped к этому `archiveId`; если
  вызывающая сторона уже знает `archiveId` (например, переродительствование
  внутри `ArchiveNodeView`), шаг выбора архива пропускается. Дерево
  использует тот же приём, что `DivisionsList.tsx`: корень — постраничная
  загрузка до конца, дети — лениво по `loadData`/`onLoadData`.
- **`ArchiveDocumentSelect` — каскадный `Select`.** Документ бессмысленен
  без узла (`ArchiveDocument.unit_id` обязателен), поэтому выбор документа
  становится доступен только после того, как выбран узел — `Select`
  подгружает документы, отфильтрованные по выбранному `unit_id`.
- **Ретрофит потребителей.** `AttachmentForm`/`AttachmentView` (`node_id`/
  `document_id`) и `AnchorEditor` (архивный вариант `Citation.Anchor`)
  переведены с обычных текстовых полей ввода id на эту пару компонентов.

### 3.6. Подпроект 7 (`Family`) — без новых паттернов

- **Самая механическая сущность программы на сегодня.** `Family` —
  `{ID, Name, Members []TextRef, Notes []TextRef, Sources []SourceLink,
  Private bool}` — структурно та же форма, что `Repository` (подпроект 3)
  минус `Type`/`Address`, с `URLs`, переименованным в `Members`. Весь стек
  (usecase-сценарии, транспорт, `httpapi`, MCP, веб) скопирован с
  `Repository` file-for-file, без единого нового решения дизайна: нет
  своего FK (только `Sources[i].CitationID`, проверяемый в транзакции по
  образцу `create_repository`/`update_repository`), нет self-ref, нет
  полиморфного типа.
- **`Members` — мягкая ссылка на `Person`, `TypePerson`.** На момент этого
  подпроекта `Person` ещё не имел CRUD (получил его в подпроекте 8, §3.7) —
  как и `ArchiveNode`/`ArchiveDocument` до подпроекта 6 (§3.2), это не
  мешает `Family` ссылаться на него: `TextRef`
  для мягких ссылок никогда не проверяется на существование при
  сохранении, независимо от того, есть ли у цели свой CRUD-слой — этот
  принцип действует с самого первого появления `TextRef` в программе, не
  специфика `Family`. Веб-форма редактирует `Members` тем же
  `TextRefListEditor`, что и `Repository.URLs`/любой другой список
  `TextRef` — без «пикера по персоне» (см. §5 — picker для `TextRef.Ref`
  остаётся вне объёма программы до подпроекта 9).
- **`Sources` редактируемый с рождения, не ретрофит** — как и у
  `ArchiveNode`/`ArchiveDocument` (подпроект 6, §3.4): `Family` заведена
  уже после того, как `Citation` получил CRUD (подпроект 5), поэтому её
  Create/Update DTO несут `sources` с первого дня.
- **Поисковый индекс — только `name`** (`replaceSearchIndex(tx, "families",
  f.ID, map[string][]string{"name": {f.Name}})`,
  `internal/store/sqlstore/relations.go`) — `members`/`notes` не
  индексируются. Формулировки MCP-описаний/doc-комментариев/веб-плейсхолдеров
  («поиск по началу названия») сверены с этим списком буквально — тот же
  урок, что и в §3.2 (найдено финальным ревью подпроекта 4, с тех пор
  проверяется явно при каждом новом подпроекте).

### 3.7. Подпроект 8 (`Person`) — вложенная подформа `PersonName`

- **Ядро графа генеалогии, но без нового FK-паттерна.** `Person` —
  `{ID, Gender, Names []PersonName, Estates []TextRef, Titles []TextRef,
  Nicknames []TextRef, Notes []TextRef, Sources []SourceLink, Private bool}`
  — все поля, кроме `ID`, опциональны, включая `Gender` (пустая строка —
  «не указан»). Единственная строго проверяемая ссылка — по-прежнему
  `Sources[i].CitationID` (`InTx`+`tx.GetCitation`, образец —
  `create_family`/`update_family`); `Person` доступен и access-aware (образец
  — `get_family`). Новый содержательный элемент этого подпроекта —
  вложенная подформа `[]PersonName`, а не новый вид FK.
- **`PersonName` — первый массив объектов, чьи собственные элементы несут
  вложенные объекты**, а не просто список `TextRef` (сам массив объектов
  не новость — `Citation.sources`, подпроект 5, — но его элементы,
  `SourceLink`, плоские: id цитаты + несколько скаляров, без вложенных
  `TextRef`/`FactDate` внутри). Каждый элемент —
  `{Type, Surname TextRef, Given TextRef, Patronymic TextRef, Prefix, Suffix,
  Since *FactDate, Until *FactDate}`: вид имени (`main`/`birth`/`married`/
  `changed`/`pseudonym`, пусто — не указан), три МЯГКИЕ ссылки на словари
  (`Surname`/`GivenName`/`Patronymic` — `TextRef.Ref`, если задан,
  проверяется только по формату, СУЩЕСТВОВАНИЕ НЕ ПРОВЕРЯЕТСЯ, тот же
  принцип, что и у любого другого `TextRef` в программе, несмотря на то что
  все три словаря имеют собственный CRUD с подпроектов 1-2), служебные
  части имени (`Prefix`/`Suffix`) и период действия (`FactDate`). Модельная
  валидация (`person_name_validate.go`) требует хотя бы одну из трёх частей
  непустой — иначе `""`-путь ошибки (пустое поле `Field`, тест на уровне
  `PersonName` отдельно от `Person`, `TestPersonNameValidateStandalone`).
- **Транспорт: `transport.PersonName`, новый файл, по образцу `transport.Anchor`
  (первый прецедент «собственный файл под вложенный не-полиморфный DTO»,
  подпроект 5).** `{type, surname, given, patronymic, prefix, suffix, since,
  until}`, все поля JSON-тегами snake_case; `surname`/`given`/`patronymic` —
  обычный `transport.TextRef` (те же `TextRefFromModel`/`(*TextRef).Model()`,
  без нового кода), `since`/`until` — обычный `transport.FactDate` (те же
  `FactDateFromModel`/`(*FactDate).Model()`). Ничего нового в самих
  value-DTO не понадобилось — только оболочка `PersonNameFromModel`/
  `(PersonName).Model()` + `PersonNamesFromModel`/`PersonNamesToModel` для
  среза, зеркалирующая `TextRefsFromModel`/`TextRefsToModel`.
- **MCP: `names` — второй массив объектов в программе, первый с вложенными
  объектами внутри своих же элементов.** Тот же технический приём, что и
  `sources` (подпроект 5, `mcp.WithArray("names", mcp.Items(map[string]any{
  "type": "object", "properties": personNameObjectProperties()}), ...)`,
  чтение — marshal/unmarshal сырого `[]any` в `[]transport.PersonName` через
  `optionalPersonNames`, `internal/mcp/object_args.go`) — но
  `personNameObjectProperties()` сама ссылается на `textRefObjectProperties()`
  и `factDateObjectProperties()` для полей `surname`/`given`/`patronymic`/
  `since`/`until`: первый случай в программе, когда элемент MCP-массива сам
  несёт вложенные MCP-объекты, а не только плоские скаляры (как у
  `sourceLinkObjectProperties`).
- **`person_update`: presence-check «отсутствие аргумента сохраняет текущее
  значение» расширен с `sources` (единственного прежде исключения,
  подпроект 5) на `names`.** `if raw, ok := args["names"]; ok && raw != nil
  { ... }` — тот же паттерн, что и у `sources` в `familyUpdateHandler`
  (§3.3): отсутствие ключа `names` в вызове `person_update` сохраняет
  текущие имена персоны как есть, пустой массив `[]` — явно очищает список.
  `estates`/`titles`/`nicknames`/`notes` остаются на общем для всей
  программы v1-ограничении (§5) — заменяются целиком текстом при каждом
  `person_update`, без presence-check.
- **Наименование зеркалирует иррегулярное множественное число хранилища.**
  `internal/store/deps.go` называет метод списка `ListPeople` (не
  `ListPersons`) — единственная иррегулярная пара в generic-хранилище.
  Usecase-пакеты названы в её честь: `list_people`/`search_people`
  (иррегулярное множественное, зеркалирует `ListPeople`), `get_person`/
  `create_person`/`update_person`/`delete_person` (единственное число,
  зеркалирует одноимённые методы хранилища). Экспортируемые методы
  сценариев следуют той же логике: `Scenario.ListPeople`/`Scenario.
  SearchPeople`, но `Scenario.GetPerson`/`CreatePerson`/`UpdatePerson`/
  `DeletePerson`. `httpapi.PersonService`/`mcp.PersonService` — те же имена
  методов. МАРШРУТЫ и ИМЕНА MCP-ТУЛОВ, тем не менее, следуют ЕДИНОМУ для
  всей программы правилу — единственное число сущности как префикс,
  независимо от иррегулярности хранилища: `/api/people` (путь во
  множественном числе, как и `/api/families`, но `person_list`/
  `person_search`/`person_get`/`person_create`/`person_update`/
  `person_delete` — единым префиксом `person_`, точно как `family_*`
  использует `family_`, а не `families_`.
- **Поиск — особый случай, не «одно поле `name`» как у большинства
  сущностей.** `personTerms(p *models.Person) []string`
  (`internal/store/sqlstore/person.go`, уже существовал до этого
  подпроекта как часть generic-слоя) собирает `Surname.Text`/`Given.Text`/
  `Patronymic.Text` из КАЖДОГО элемента `p.Names`, не только из
  «основного» (`main`), и индексирует всё под одним полем `"name"`.
  Формулировки MCP-описаний/doc-комментариев/веб-плейсхолдеров должны
  отражать это буквально («ищет по началу фамилии/имени/отчества из ЛЮБОГО
  из имён персоны, не только основного») — тот же урок §3.2/§3.6, но
  здесь качественно иной (не «одно поле вместо всех текстовых», а «поле
  агрегирует несколько повторяющихся дочерних записей»); `Estates`/
  `Titles`/`Nicknames`/`Notes` поиском не охвачены, как и `Members`/`Notes`
  у `Family`. Автоматическое доказательство — `TestSearchPeopleFindsByMarriedNameOnly`
  (`internal/usecases/search_people/scenario_test.go`) на уровне сценария и
  `TestPersonWriteContractWithRealStore` (`internal/httpapi/write_store_test.go`)
  на реальном SQLite: обе ищут по части фамилии, которая встречается ТОЛЬКО
  во второй (не основной) записи `Names`.
- **Веб: `PersonNameListEditor` — первый в программе повторяющийся
  форма-редактор с несколькими текстовыми полями и датой на строку.**
  `web/src/PersonNameList.tsx` редактирует `[]PersonName` построчно (тип,
  три текстовых поля `surname`/`given`/`patronymic` без пикера — как
  `Church.Parish`, `prefix`/`suffix`, `FactDateEditor` x2 для
  `since`/`until`). Ref-preservation для `surname`/`given`/`patronymic`
  построена на **baseline захваченной один раз при монтировании
  компонента** (`rowMeta: {id, base: PersonName}[]`, `useState`-
  инициализатор, НЕ `useEffect`-пересинхронизация); baseline
  подставляется по ИНДЕКСУ строки в массиве (`rowMeta[i]`, не по `id`) —
  стабильный `id` служит только React `key` для рендеринга списка, а
  соответствие `value[i]`/`rowMeta[i]` держится за счёт того, что `add`/
  `remove` всегда меняют оба массива параллельно (один элемент за раз, с
  той же позиции) — рассинхронизировались бы, переупорядочь что-то `names`
  извне, пока редактор смонтирован; текст поля, не изменившийся относительно этой
  базовой линии, сохраняет исходный `{text, ref, type}` целиком; изменённый
  — испускает `{text}` без `ref`/`type`. Это работает, только пока
  компонент действительно размонтируется/монтируется заново на каждую
  сессию редактирования (подтверждено для обоих вызывающих мест —
  `PersonForm`'s модалка с `destroyOnHidden`, `PersonView`'s условная
  ветка `editing`); при переиспользовании там, где компонент остаётся
  смонтированным между переключением записей, потребовался бы другой
  дизайн (например, явный сигнал сброса baseline) — ограничение,
  унаследованное от способа, которым ref-preservation впервые появилась у
  `Church.Parish`/`Parish.Church` (§3.1), но здесь качественно сложнее из-за
  повторяющихся строк с независимым состоянием каждая. Страницы
  `PeopleList`/`PersonForm`/`PersonView` — иначе обычная страница-тройка
  по конвенциям §4, без отступлений (плоский список, toggle-редактирование
  + явное сохранение, модалка создания).

### 3.8. Подпроект 9 (граф вокруг Person: `Relation`, `Residence`, `Event`) — новые паттерны

Последний подпроект программы: три сущности, которые вместе образуют «граф
вокруг Person» — `Relation` (ребро родства/связи между двумя персонами),
`Residence` (проживание персоны в месте) и `Event`+`EventParticipant`
(событие с участниками). Backend и веб построены и проверены двумя
последовательными живыми проходами, зафиксированными сквозным тестом
`TestEventWriteContractWithRealStore` (`internal/httpapi/
write_store_test.go`) и аналогичными контрактными тестами `Relation`/
`Residence`; этот раздел описывает оба. Веб добавляет первый в программе
переиспользуемый picker персоны
(`PersonPicker`+`usePersonOptions`, `web/src/PersonPicker.tsx`) и
`EventParticipantListEditor` (`web/src/EventParticipantList.tsx`) — оба
описаны в последнем пункте этого раздела.

- **Первая пара строгих ссылок на ОДИН И ТОТ ЖЕ тип.** `Relation.PersonA`/
  `.PersonB` — обе строгие ссылки на `Person`, а не на разные сущности (как
  `Archive.RepositoryID`, §3.1) или self-ref (как `Note.ParentID`, §3.2).
  `create_relation`/`update_relation` проверяют существование каждой стороны
  своим вызовом `GetPerson`, но последовательно, с ранним `return` на первой
  же неудаче — тот же приём, что и у любого usecase с несколькими проверками
  существования в программе, а не особая «обе стороны независимо»
  семантика: если не найдены обе персоны, ошибка вернётся только по
  `person_a`, до проверки `person_b` дело не дойдёт. Различность
  `PersonA`/`PersonB` — забота модельной `Validate()` (`person_b`:
  "связь персоны с самой собой"), не usecase-уровня: то же разделение
  ответственности, что и везде в программе (форма/структура — `Validate()`,
  существование — usecase в транзакции).
- **Осознанный отказ от `search_relations`/`search_residences` — не
  недосмотр, а решение, продиктованное устройством generic-слоя.**
  `internal/store/sqlstore/relations.go`'s `SaveRelation`/`SaveResidence`
  вызывают `replaceSearchIndex(tx, "relations"/"residences", r.ID, nil)` —
  передают `nil` вместо карты полей, то есть НИ ОДНО поле этих двух
  сущностей не попадает в поисковый индекс, буквально никогда. Программа
  до этого подпроекта неукоснительно придерживалась «каждая сущность
  получает 6 операций» (`list`/`search`/`get`/`create`/`update`/`delete`)
  — здесь это правило сознательно нарушено: `search_relations`/
  `search_residences` были бы тулами/маршрутами, которые ВСЕГДА возвращают
  пустой результат, независимо от запроса — не «менее удобная», а
  структурно нерабочая функциональность, к тому же вводящая клиента в
  заблуждение (наличие тула подразумевает, что он что-то находит).
  Вместо этого — person/place-scoped фильтрация списка: новые типы запроса
  `models.RelationQuery{PersonID *ID, Page}` (ребро проходит, если
  `PersonID` совпадает с `PersonA` ИЛИ `PersonB`) и
  `models.ResidenceQuery{PersonID, PlaceID *ID, Page}` (оба фильтра
  пересекаются, если заданы вместе). Это НЕ обходной путь вокруг
  отсутствующего поиска, а более полезный инструмент для собственно
  предметной задачи графа родства — «все связи этой персоны» или «все
  проживания в этом месте» осмысленнее, чем текстовый поиск по сущности,
  у которой из текстовых полей всего пара необязательных заметок.
  `list_relations`/`list_residences` — full-scan-and-filter по
  generic-окнам `ListRelations`/`ListResidences`, тот же приём, что и
  `list_archive_nodes` (подпроект 6, §3.4): фильтрация в usecase-цикле, без
  нового метода `store.Store` — оправдано тем же доводом об объёме данных.
  `Event`, в отличие от них, ЕСТЬ поисковый индекс — `SaveEvent` вызывает
  `replaceSearchIndex(tx, "events", e.ID, map[string][]string{"place":
  {place}})` — индексируется ТОЛЬКО текст `Place` (не `type`, не `date`, не
  участники); `search_events`/`GET /api/events/search` построен обычным
  образом (по образцу `search_families`), а `models.EventQuery{PersonID
  *ID, Page}` — дополнительный, не заменяющий поиск фильтр по участнику
  (`list_events`, тот же full-scan-and-filter). MCP-описания/doc-комментарии
  везде говорят буквально «по началу текста места (place)», не «по типу
  события» — тот же урок об overclaim-е описания поиска, что и в §3.2/§3.6/
  §3.7 (сам этот дефект уже случался в подпроекте 4 и был найден финальным
  ревью).
- **`EventParticipant.PersonID` — не первая строгая ссылка внутри
  array-of-objects аргумента в программе, но первая, где она — главный
  субъект элемента.** `sources[i].citation_id` (подпроект 5) уже была
  строгой ссылкой ВНУТРИ каждого элемента массива `Sources`, с той же
  формой проверки и индексированной ошибкой — так что `Participants[
  i].PersonID` не вводит новый технический паттерн проверки. Разница у́же:
  участник события — это ссылка на персону (главный субъект элемента), а не
  одно из нескольких полей вспомогательной evidence-ссылки, как
  `citation_id` у `SourceLink`. `PersonName.Surname/.Given/.Patronymic`
  (подпроект 8), для контраста, — мягкие `TextRef`, существование не
  проверяется вовсе. `create_event`/`update_event` идут циклом `for i, p :=
  range e.Participants { tx.GetPerson(...) }`, результат — `*models.
  ValidationError{Field: fmt.Sprintf("participants[%d].person_id", i)}`
  (тот же приём индексации, что и `sourceLinkErr`, только на другом поле
  массива). Цикл по `Participants` и цикл по `Sources` — независимы (оба
  нужны, разные массивы, разные проверяемые сущности).
- **`Event.Place` — мягкая ссылка, но нового объектного вида (`PlaceRef`,
  не `TextRef`/`FactDate`), и первое одиночное объектное поле, получившее
  правильный update-guard с рождения, а не ретрофитом.** `models.PlaceRef`
  — структурно идентичен `models.TextRef` (`{Text, Ref ID, Type Type}`), но
  это ОТДЕЛЬНЫЙ тип домена (валидация ограничивает `Ref`'s тип только
  `AdministrativeDivision`/`Church`/`Parish` — см. `place_ref_validate.go`),
  поэтому и транспортный слой заводит `transport.PlaceRef` — свой тип,
  свои `PlaceRefFromModel`/`(*PlaceRef).Model()`, а не переиспользование
  `transport.TextRef`, хотя JSON-форма та же `{text, ref?, type?}`. Как и
  любой `TextRef`-подобный soft ref в программе, `Place` НИКОГДА не
  проверяется на существование при сохранении — ни в `create_event`, ни в
  `update_event`; только `Event.Validate()` проверяет формат/ограничение
  типа (вызывается до `InTx`, как всегда). На MCP-уровне `Place`
  (`*models.PlaceRef`) — второе (после `Church.Parish`/`Parish.Church`,
  §3.1, и `since`/`until` многих сущностей) одиночное объектное поле в
  программе, но ПЕРВОЕ, заведённое уже ПОСЛЕ того, как программа
  исправила регрессию "`raw != nil` вместо presence-only guard для
  single-object `TextRef`/`FactDate`-полей" (коммит с описанием "fix:
  ревью — null должен снова очищать TextRef/FactDate-поля в *_update"),
  так что `eventUpdateHandler` с самого начала использует правильный
  guard — `if _, ok := args["place"]; ok { ... }` (без `raw != nil`),
  позволяя явному `{"place": null}` очистить поле, при том что отсутствие
  ключа `place` в вызове сохраняет текущее значение. Та же presence-only
  форма — у `since`/`until` (`*models.FactDate`) `Relation`/`Residence`.
  `Relation.Kind`/`Event.Type` — REQUIRED на `_update` и заменяются
  безусловно (как `Citation.SourceID`, "ядро того, чем является сущность"),
  как и `Relation.PersonA`/`.PersonB`/`Residence.PersonID`/`.PlaceID` —
  сознательно БЕЗ preserve-on-omit: подразумевать «меняю только вид связи,
  а стороны остаются прежними» через отсутствие аргумента было бы
  двусмысленно для полей, которые и есть идентичность записи.
  `Relation.RelType`, чья обязательность условна (только при
  `kind=associate`, проверяется `Validate()`, не MCP-схемой, которая не
  умеет условных требований), — обычный необязательный скаляр с
  preserve-on-omit guard, как любое другое опциональное поле.
- **Новый транспортный файл `transport.EventParticipant`** (по образцу
  `transport.PersonName`, подпроект 8, — «собственный файл под вложенный
  не-полиморфный DTO») — флат-объект `{person_id, role, note?}`, ПРОЩЕ
  `PersonName`/`SourceLink`: никаких вложенных объектов внутри (в отличие
  от `PersonName.surname/.given/.patronymic`), просто три скаляра;
  сложность здесь не в форме контракта, а в том, что `person_id` требует
  проверки существования (см. выше). MCP-схема —
  `eventParticipantObjectProperties()`/`optionalEventParticipants()`
  (`internal/mcp/object_args.go`), по образцу `sourceLinkObjectProperties`/
  `optionalSourceLinks`, только без вложенных `mcp.WithObject` внутри
  элемента.
- **Веб: `PersonPicker` — первый в программе переиспользуемый picker
  персоны.** Стал возможен только теперь, когда `Person` (подпроект 8)
  наконец получил `List`/`Search`: `usePersonOptions()`
  (`web/src/PersonPicker.tsx`) грузит до `MAX_PAGE_LIMIT` персон и
  экспортируется отдельно от компонента `PersonPicker`, т.к. View-страницы
  (`RelationView`/`ResidenceView`/`EventView`) резолвят голый `person_id` в
  подпись/ссылку тем же хуком, не только предлагают выбор (тот же приём,
  что `ArchiveView.tsx`/`fetchRepositories`). Используется в трёх местах:
  `Relation.PersonA`/`.PersonB` (два picker'а на одной форме),
  `Residence.PersonID`, каждая строка `Event.Participants` (внутри
  `EventParticipantListEditor`). `EventParticipantListEditor`
  (`web/src/EventParticipantList.tsx`) структурно повторяет
  `SourceLinkListEditor`, но проще `PersonNameListEditor` (подпроект 8):
  `person_id` — строгая ссылка на существующую персону, проверяемая на
  сервере, а не мягкий `TextRef` с ref-preservation — значит никакой
  baseline-per-mount-логики `PersonNameListEditor` здесь не нужно.
  `ResidenceForm.tsx` заводит локальный `useAdminDivisionOptions()` —
  первый picker, точечно нацеленный на `AdministrativeDivision` как
  строгую FK-цель, не общий `TextRef`-словарь; не вынесен в общий файл —
  единственная точка использования в этом подпроекте. `EventView.tsx` —
  единственная страница подпроекта с ref-preservation-логикой: `place`
  обрабатывается ИМЕННО как `ChurchView.tsx.onSave` (§3.1) — исходный
  `{text, ref, type}` сохраняется, если текст в форме не изменился
  относительно загруженного значения, иначе отправляется `{text}` без
  `ref`/`type`. `RelationsList`/`ResidencesList` рендерятся БЕЗ
  `Input.Search` — единственные списковые страницы в программе без него,
  отражая отсутствие `search_relations`/`search_residences` на бэкенде.

## 4. Веб-UI конвенции

**Навигация — единая точка входа, без вкладок.** Уточнение по ходу
обсуждения этого документа: `Tabs` в `AppContent` убираются совсем.
Вместо них:

- **Каталог** (`/`, без редиректа на `/docs`) — список подключённых
  сущностей по алфавиту (пополняется по мере выполнения подпроектов 1-9 —
  сейчас 3 строки: Административное деление, Документация, Фамилии),
  каждая строка — ссылка на `/<entities>` (List этой сущности) либо на
  `/docs` для строки «Документация». Компонент без собственного
  состояния — статический массив `{label, path}`, сортировка
  `localeCompare('ru')`.
- **Breadcrumbs** на каждой странице (List/View/Docs), дублируют путь от
  корня: `Сущности / <Entity Plural>` на List, `Сущности / <Entity Plural>
  / <имя>` на View — тот же `antd Breadcrumb`, что уже есть в
  `DivisionView`/`DocsPanel`, только с добавленным корневым звеном
  `Сущности` (ссылка на `/`).
- **`SettlementsTab` удаляется** (не мигрирует на отдельный роут) —
  фильтрованный взгляд `kind=settlement` дублирует дерево+поиск
  `DivisionsList`, отдельная страница ради него не оправдана. Его роль
  полностью покрывает уже существующий `/divisions`.
- Эта навигационная перестройка (каталог + breadcrumbs везде + снос
  `Tabs`/`SettlementsTab`) — часть плана подпроекта 1 (Surname), а не
  будущая работа: она нужна сразу, чтобы Surname корректно встраивался
  в UI, а не добавлял двадцать первую вкладку.

- **List** (`/<entities>`) — для НЕ-иерархических сущностей (все, кроме
  `ArchiveNode`, которая переиспользует tree-паттерн `DivisionsList`) —
  плоский список с пагинацией (та же `offset`-пагинация до короткой
  страницы, что уже есть в `DivisionsList.loadRoot` после финального
  ревью прошлого прохода — не только первое окно в `MAX_PAGE_LIMIT`),
  строка поиска (`Input.Search`, `onSearch`), «+ добавить» (только
  `session != null`), клик по строке — переход на View.
- **View** (`/<entities>/:id`) — поля сущности + списки `TextRef`
  (показываются как текст; если `Ref`/`Type` заполнены — мелкая
  вторичная пометка «→ Type ID», НЕ кликабельно: у большинства типов
  ещё нет View-страницы, вести некуда). Режим редактирования — тот же
  toggle на той же странице + явная кнопка «Сохранить» (не автосейв по
  полю) — потому что `Save*` заменяет сущность целиком при каждом
  вызове (§1), это свойство самого стора, не специфика деления.
  Кнопки записи (редактировать/удалить/добавить) — только
  `session != null`.
- **TextRef-списки в форме (v1)** — редактируется только текстовая
  часть (добавить/удалить/поменять строку). Элементы с уже заполненным
  `Ref` показываются как есть (текст + пометка о ссылке), но не
  редактируются через эту форму. Picker для выбора ссылки на другую
  сущность — сознательно отложен: отдельная работа, нужна только когда
  реально понадобится (Person/Event и т.д., подпроект 9).
- **Create** — модалка по образцу `CreateDivisionModal`, свой файл на
  сущность (`<Entity>Form.tsx`) — без преждевременной параметризации
  одним общим компонентом на все 20 сущностей: файлы копируются по
  образцу, а не наследуются от общего родителя (тот же выбор, что и в
  прошлом проходе — «раскладка рассчитана на копирование, не общий
  фреймворк сейчас»).
- **Delete** — `Popconfirm` + модалка конфликта 409
  (`{error, referrers: [{type, id}]}`) — как в делениях.

## 5. Не в объёме этого документа

- Picker для `TextRef.Ref` (выбор ссылки на произвольную сущность) —
  всё ещё отложен, но не целиком: `PersonPicker` (§3.8) теперь существует —
  для СТРОГИХ ссылок на `Person` (`Relation.PersonA`/`.PersonB`,
  `Residence.PersonID`, `EventParticipant.PersonID`), где выбор можно
  проверить на сервере. Пикер для МЯГКИХ `TextRef.Ref`-ссылок на `Person`
  (`Family.Members`, части `PersonName` — `Surname`/`Given`/`Patronymic`) —
  по-прежнему отдельная, отложенная идея: эти поля формат-only, существование
  никогда не проверяется, так что задача пикера там другая (подсказка/выбор
  из словаря, а не выбор строго существующей записи).
- Вложенные подформы Person (`[]PersonName`, подпроект 8) и Event
  (`[]EventParticipant`, подпроект 9) реализованы целиком, backend и веб —
  `PersonNameListEditor` (§3.7) и `EventParticipantListEditor` (§3.8)
  соответственно. Программа `entity-write` завершена подпроектом 9: все 21
  сущность домена имеют полный CRUD через HTTP, MCP и веб. Общие конвенции
  §3-4 всё равно применяются как основа.
- **Известное ограничение v1 MCP-тулов записи** (обнаружено финальным ревью
  подпроекта 1): `<entity>_update` заменяет СПИСКИ `TextRef` (например,
  `Surname.Variants`, `Church.Settlements`/`Notes`) целиком текстом — эти
  поля в MCP-контракте принимают только плоские строки (`WithStringItems`),
  без `ref`/`type` вовсе, так что ссылка на элементе (если задана иначе, не
  через MCP) теряется при любом MCP-обновлении. Практический блэк-радиус
  сейчас нулевой (ни один путь — ни веб, ни MCP — пока не создаёт ссылочных
  `TextRef` ни для одной сущности). Пересмотреть вместе с picker'ом (см.
  выше) — тогда же решить, принимать ли в MCP-тулах записи объекты
  `{text, ref?, type?}` вместо плоского текста для оставшихся
  списочных полей (смена MCP-контракта, нужно согласие пользователя,
  `AGENTS.md`).
  **Уточнение подпроекта 3:** это ограничение НЕ распространяется на
  одиночный необязательный `*TextRef` (§3.1, `Church.Parish`/
  `Parish.Church`) — там MCP-аргумент уже объектный (`{text, ref?, type?}`,
  `mcp.WithObject` + `textRefObjectProperties`), и `ref`/`type`
  round-trip'ятся как есть: клиент, получивший их через `church_get`/
  `parish_get` и отправивший обратно неизменными в `church_update`/
  `parish_update`, ссылку не теряет. Ошибка в описании этих MCP-тулов
  (`ref`/`type` были помечены как «только для чтения») исправлена
  финальным ревью подпроекта 3.
