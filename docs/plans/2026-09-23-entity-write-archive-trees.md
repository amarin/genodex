# Веб-CRUD/MCP для всех сущностей — подпроект 6 (архивные деревья: ArchiveNode, ArchiveDocument): план

> **Для агента-исполнителя:** ОБЯЗАТЕЛЬНЫЙ ПОДНАВЫК: superpowers:subagent-driven-development.

Шестой проход по декомпозиции `docs/data-model/entity-write.md` §2. Вводит `ArchiveNode` (узел архивного дерева — фонд/опись/дело и т.п., рекурсивный `ParentID`, но, в отличие от `AdministrativeDivision`, обязательно привязан к одному `Archive` через `ArchiveID`, так что деревьев на самом деле много — по одному на архив) и `ArchiveDocument` (документ внутри единицы учёта — плоская сущность со строгой ссылкой `UnitID` на `ArchiveNode`). Обе сущности впервые появляются в модели уже после того, как `Citation` получил CRUD (подпроект 5) — их `Sources []SourceLink` редактируется с рождения, никакого read-only-этапа не было. На веб-стороне впервые появляется переиспользуемый `ArchiveNodePicker` — им же сразу ретрофитятся два места, до сих пор обходившиеся текстовыми полями ввода id: `Attachment.NodeID`/`DocumentID` и `Citation.Anchor`'s `archive`-вариант (`AnchorEditor`).

## Goal

1. Полный CRUD (HTTP + MCP + веб) для `ArchiveNode` и `ArchiveDocument` — по конвенциям `docs/data-model/entity-write.md` §3-4.
2. Новый переиспользуемый веб-компонент `ArchiveNodePicker` (+ `ArchiveDocumentSelect`, `useArchiveOptions`) — дерево с ленивой подгрузкой (`antd Tree` + `loadData`, тот же приём, что `DivisionsList.tsx`), используемый собственными формами `ArchiveNode`/`ArchiveDocument` и ретрофитом `Attachment`/`AnchorEditor`.
3. Ретрофит `AttachmentForm.tsx`/`AttachmentView.tsx`/`AnchorEditor.tsx`: `node_id`/`document_id` — раньше обычные текстовые `Input`, теперь `ArchiveNodePicker`/`ArchiveDocumentSelect`.
4. Новые переиспользуемые паттерны программы: дерево, скопированное по обязательному внешнему владельцу (не одно глобальное, как у `AdministrativeDivision`); кросс-полевая проверка «чужой FK» («родитель существует И принадлежит тому же архиву», а не только «существует»).

## Архитектура

`ArchiveNode` — дерево как `AdministrativeDivision` (рекурсивный `ParentID`, тот же `antd Tree`/`loadData`-паттерн на вебе), но со скопом на конкретный `Archive` через обязательный `ArchiveID` — по факту деревьев столько же, сколько архивов, отсюда отдельный тип запроса `models.ArchiveNodeQuery{ArchiveID, ParentID, Page}`, не переиспользующий `DivisionQuery`. `ArchiveDocument` — плоская сущность (без собственной иерархии), обязательная строгая ссылка `UnitID → ArchiveNode`. Обе сущности несут `Sources []SourceLink` редактируемым с первого дня (не ретрофит, как у шести сущностей подпроекта 5) — `Citation` уже имеет CRUD к моменту их появления. На веб-стороне вводится один переиспользуемый компонент `ArchiveNodePicker` (модалка: шаг выбора архива, если `archiveId` не передан пропом, затем дерево с ленивой подгрузкой) и спутник `ArchiveDocumentSelect` (плоский `Select`, отфильтрованный на клиенте по `unit_id`) — используются как собственными формами новых сущностей, так и ретрофитом двух существующих мест (`AttachmentForm`/`AttachmentView`, `AnchorEditor`'s `archive`-вариант привязки), которые раньше принимали `node_id`/`document_id` обычным текстом, потому что у `ArchiveNode`/`ArchiveDocument` ещё не было своего CRUD-слоя, по который можно было бы построить picker.

## Технологии

Backend: Go, `internal/models` (домен) → `internal/usecases/<scenario>` → `internal/transport` (DTO) → `internal/httpapi` (JSON REST) / `internal/mcp` (`github.com/mark3labs/mcp-go`, Streamable HTTP) → `internal/store`/`internal/store/sqlstore` (generic-хранилище, уже готово для всех 21 типа, в этом проходе не меняется). Frontend: Vite + React 18 + TypeScript + antd (`Tree`, `Select`, `Modal`, `Form`), файлы-страницы по образцу уже существующих (копирование, не общий фреймворк). Тесты: стандартный `go test` (usecases — фейковый стор в памяти; `internal/httpapi/write_store_test.go` — реальный SQLite), `npm run typecheck`/`npm run build` на вебе.

## Спецификация

Источник истины дизайна — `docs/data-model/entity-write.md` (копия из проверочного worktree, см. «Предпосылка» — копия на `main` этого раздела не содержит): §3.4 «Подпроект 6 (архивные деревья: `ArchiveNode`, `ArchiveDocument`) — новые паттерны» — специфичные для этого подпроекта решения (скоп дерева по архиву, кросс-полевая проверка родителя, `Sources` редактируемый с рождения); §4 «Веб-UI конвенции» — общие правила List/View/Create/Delete, которым следуют новые страницы `ArchiveNode`/`ArchiveDocument`, за вычетом единственного отступления: `ArchiveNode` — исключение из правила «List = плоский список», это единственная (кроме `AdministrativeDivision`) иерархическая сущность и её список переиспользует tree-паттерн `DivisionsList`, а не плоскую пагинацию.

## Глобальные ограничения

Правила программы `entity-write`, обязательные для каждой новой/ретрофитуемой сущности (дословно из `entity-write.md` и правил, закреплённых финальными ревью подпроектов 1-5); подпроект 6 следует всем, два последних — новые, вводятся именно им:

- **Access-aware `Get` для любой сущности с `Private`.** `get_archive_node`/`get_archive_document` обязаны принимать `access models.Access` и прятать приватную запись как отсутствующую для не-владельца: `if rec.Private && access != models.AccessFull { return models.ErrNotFound }` (образец — `internal/usecases/get_archive/scenario.go`, правило закреплено после того, как было упущено при первом появлении `Private` в подпроекте 3).
- **Real-store тесты обязательны.** Каждая пишущая сущность получает сквозной тест на реальном SQLite-сторе в `internal/httpapi/write_store_test.go` (не только фейковый стор в usecase-тестах) — здесь `TestArchiveNodeWriteContractWithRealStore`/`TestArchiveDocumentWriteContractWithRealStore` (Шаг 1.5).
- **Fetch-then-merge на каждом update.** `update_<entity>` читает текущую версию через `Get<Entity>`, накладывает новые поля, валидирует и сохраняет через `Save<Entity>` — полная замена записи, не частичный patch (свойство самого generic-стора, см. `entity-write.md` §1).
- **MCP «отсутствие необязательного списочного аргумента — значит сохранить как есть».** Для `sources` (и любого другого необязательного списка) в `<entity>_update`: аргумент не передан — текущее значение сохраняется; передан пустым списком `[]` — очищает. HTTP `PUT`, наоборот, всегда полностью заменяет тело — отсутствие ключа `sources` в JSON очищает список. Разное поведение MCP/HTTP для одного и того же поля — осознанное решение подпроекта 5, задокументированное в `docs/usage.md`.
- **Формулировка описания поиска обязана совпадать с реальным списком полей `replaceSearchIndex`.** MCP-описания/doc-комментарии/веб-плейсхолдеры для `<entity>_search`/`GET .../search` должны буквально называть те поля, что действительно передаются в `replaceSearchIndex` при сохранении (`internal/store/sqlstore`), а не любые текстовые поля модели — расхождение однажды уже поймано финальным ревью подпроекта 4 (`Note`: было «по началу заголовка или текста» при индексации только `title`).
- **Same-archive-parent — новый инвариант этого подпроекта.** `ArchiveNode.ParentID`, если задан, обязан указывать на узел, чей собственный `ArchiveID` совпадает с `ArchiveID` создаваемого/изменяемого узла — родитель может существовать и быть валидным узлом и всё равно быть отвергнут, если он из другого архива. Ошибка — `*models.ValidationError` на поле `parent_id` (не `archive_id`), с текстом «родитель %q принадлежит другому архиву», отдельным от текста «родитель %q не найден». Проверяется только непосредственный родитель, не вся цепочка — по индукции цепочка целиком согласована, если согласовано каждое звено при своём сохранении.
- **Дерево, скопированное по обязательному внешнему владельцу — новый паттерн этого подпроекта.** `ArchiveNodeQuery{ArchiveID, ParentID, Page}` — `ArchiveID` обязателен всегда (узел бессмысленен вне архива), `ParentID` — `nil` значит «корень внутри этого архива», не «корень всего дерева». Выделенного метода хранилища «дети узла» не заводится — `list_archive_nodes` делает полный проход по generic-окнам с фильтром `ArchiveID == q.ArchiveID && ParentID == q.ParentID` в usecase-слое; новый выделенный метод стора оправдан только когда full-scan-and-filter перестанет тянуть по объёму данных, не заранее.

## Предпосылка: живая проверка

Весь код ниже применён и проверен в отдельном throwaway git worktree (`/Users/asmarin/dev/mine/genodex-verify-subproject6`, ветка `verify/entity-write-subproject6`, форкнута от `main` на `35ce70d`) — не на `main` напрямую и не в этом плане с нуля: он транскрибирован из уже рабочего дерева, а не написан заново. Два отчёта из того прохода — `/Users/asmarin/dev/mine/genodex-verify-subproject6/BACKEND_REPORT.md` и `WEB_REPORT.md` — задокументировали, что было сделано и почему; существенные решения из обоих перенесены сюда, чтобы будущий читатель плана понимал не только ЧТО делает код ниже, но и ПОЧЕМУ он выглядит именно так.

**Бэкенд** (`BACKEND_REPORT.md`): `gofmt -l .` пусто, `go build/vet/test ./...` зелёные — **1215 тестов, 112 пакетов** (было 1097 после подпроекта 5 — 118 новых: 80 в 12 usecase-пакетах, 20 в httpapi, 16 в mcp, 2 сквозных real-store теста). Живой смок-тест через `curl` (реальный `genodex serve`, чистая БД): создание двух `Archive`, корневого `ArchiveNode` без `parent_id`, дочернего `ArchiveNode` с `parent_id` — `GET /api/archive-nodes?archive_id=…` без `parent_id` возвращает только корень, с `parent_id` — только прямых детей; `GET /api/archive-nodes` без `archive_id` — `400 {"error":"параметр archive_id обязателен"}`; создание `ArchiveDocument` с несуществующим `unit_id` — `422 {"field":"unit_id"}`; **ключевая проверка** — `POST /api/archive-nodes` с валидным `parent_id`, принадлежащим ДРУГОМУ архиву, чем указанный `archive_id`, — `422 {"error":"archive_node: parent_id: родитель \"AN-…\" принадлежит другому архиву","field":"parent_id"}`, подтверждает инвариант «тот же архив» сквозь реальный SQLite-стор, не только фейковый стор в unit-тестах.

**Веб** (`WEB_REPORT.md`): `npm run typecheck`/`npm run build` чисты. Живая проверка в браузере (реальный сервер, `-web dev`): каталог на `/` показывает «Архивные документы»/«Архивные единицы» в алфавитном порядке; создание архива → `/archive-nodes` → выбор архива в `Select` → создание корневого узла через модалку → View с корректной ссылкой на архив и «корень» как родитель → «+ добавить дочерний узел» с `parent_id`, предзаполненным на View, → дочерний узел появляется в «Дочерние узлы» родителя; `/archive-nodes?archive_id=<id>` (точная ссылка, которую строит новая кнопка на `ArchiveView.tsx`) — архив выбирается из URL-параметра автоматически, дерево лениво подгружает детей через `loadData`; создание `ArchiveDocument` через picker (шаг выбора архива → дерево → «Текущий узел: …») — View показывает рабочую ссылку на узел; ретрофит `AttachmentForm`: «Архивный узел» теперь кнопка «Выбрать узел» (не текстовый `Input`), «Архивный документ» — `Select`, задизейбленный до выбора узла, корректно фильтруется по `unit_id` после выбора; ретрофит `AnchorEditor` (`kind=archive`) — та же замена, привязка round-trip'ится через `Citation`; попытка удалить узел с дочерним узлом — 409 с модалкой конфликта, перечисляющей `archive_node` как referrer (тот же паттерн, что `ArchiveView`/`DivisionView`); редактирование узла — поле «Родитель» рендерится как `ArchiveNodePicker`. Console чист от неожиданных ошибок (три ожидаемые — сознательно спровоцированные проверки).

**Существенные находки живой проверки (design judgment calls), перенесённые сюда дословно по смыслу, чтобы код ниже не выглядел произвольным:**

- `ArchiveNodeQuery.Validate()` проверяет формат `parent_id` через `idErr` напрямую (как уже делает `DivisionQuery.Validate()`), а не через отдельный хелпер `validateOptionalIDPtr` — план по ходу обсуждения упоминал такой хелпер, но реальная соседняя конвенция в `query.go` использует `idErr` инлайн; код ниже отражает реальный код, а не пересказ плана.
- `update_archive_node` перечитывает `ArchiveNode` непосредственного родителя дважды: один раз для проверки «тот же архив», второй раз — уже внутри обхода `checkParentChain`. Функционально корректно, но не оптимизировано — сделано так намеренно, чтобы буквально повторить форму `update_note.checkParentChain`, а не ручную оптимизацию лишнего вызова `GetArchiveNode`, по инструкции «адаптировать только тип сущности и поле ошибки».
- `parseArchiveNodeQuery` в `internal/httpapi/archive_node.go` возвращает `400` (не `422 ValidationError` из сценария), когда `archive_id` отсутствует в запросе — согласуется с тем, как уже обрабатываются синтаксические проблемы запроса (`intParam` и т.п.) в каждом другом `parse*Query`-хелпере: синтаксические проблемы самого запроса — 400, семантические/бизнес-ошибки, которые ловит сценарий, — 422.
- `ArchiveNodeForm`'s `parent_id` — настоящее редактируемое поле picker'а, а не фиксированный проп, как `parent_id` у `CreateDivisionModal`. Дизайн-документ описывал `parent_id` как «предзаполнен, если форма открыта как „добавить дочерний“, иначе — через `ArchiveNodePicker`» — прочитано как «всегда поле picker'а, просто предзаполненное значением пропа», сознательное отличие от tree-паттерна `AdministrativeDivision`, потому что у `ArchiveNode` теперь есть picker, которого не было у `AdministrativeDivision`.
- `EDIT_FORM_FIELDS`/`FORM_FIELDS` нигде не включают `parent_id`/`node_id`/`unit_id`/`document_id` — эти поля управляются отдельным `useState`, а не полями antd `Form` (picker — не нативный контрол формы), поэтому 422 на них попадает в общий баннер `saveError`/`error`, а не привязывается к конкретному полю формы. Это повторяет уже существующий выбор `DivisionView.tsx` для `parent_id` (тоже вне `EDIT_FORM_FIELDS`), только распространённый на новые picker-поля.
- `ArchiveDocumentSelect`'s фильтр по `unit_id` — без сужения на сервере: по спецификации задачи компонент запрашивает все `ArchiveDocument` (лимит 500, та же конвенция, что у любого другого плоского `Select` в этой кодовой базе) и фильтрует на клиенте по `unit_id === nodeId` — у `GET /api/archive-documents` нет параметра `unit_id`. В масштабе v1 это повторяет уже существующую конвенцию `Select` цитат в `SourceLinkListEditor`.
- Метки `nodeLabel`/`unitLabel` — только локальное состояние для отображения, нигде не сохраняются в контракте API. Там, где компонент уже имел загруженный объект `ArchiveNode` (родитель у `ArchiveNodeView`, единица учёта у `ArchiveDocumentView`) — метка picker'а заполняется из него; там, где раньше объект не подгружался (`AttachmentView`, `AnchorEditor`) — новый запрос ради красивой метки не добавлялся, fallback picker'а (`label ?? value`, то есть сырой id) закрывает этот случай — минимальный по объёму ретрофит, по инструкции задачи «не трогать остальное».
- `AttachmentView`'s read-only `Descriptions` по-прежнему показывает `node_id`/`document_id` сырыми id, не ссылками — инструкция по ретрофиту касалась именно текстовых `Input`-полей формы редактирования, «не трогать несвязанные поля» — read-only режим просмотра оставлен как есть, апгрейд до ссылки вне объёма.

## Задача 1. Бэкенд: ArchiveNode, ArchiveDocument — полный стек + доки + real-store тесты

**Интерфейсы, потребляемые из подпроектов 1-5**: `models.Page`/`models.Access`, `internal/httpapi/surname.go`:`parsePage`, `internal/mcp/*.go`:`optionalInt`/`toolJSONResult`/`textRefsFromStrings`, `transport.{TextRef,FactDate,SourceLink}`, `mcp/object_args.go`:`factDateObjectProperties`/`optionalFactDate`/`textRefObjectProperties`/`optionalTextRef`/`sourceLinkObjectProperties`/`optionalSourceLinks` (подпроект 5), `get_archive`'s access-aware `Get`-паттерн, `create_archive`'s проверка-FK-в-транзакции паттерн, `update_note.checkParentChain`'s обход цепочки родителей на цикл, `store.Store.{GetArchive,GetArchiveNode,GetCitation,InTx}` (generic-хранилище, `internal/store/deps.go`, уже умеет читать `ArchiveNode`/`ArchiveDocument` по id до появления их usecase-слоя — использовалось `create_attachment`/`update_attachment` подпроекта 4 и `create_citation`/`update_citation` подпроекта 5, см. `entity-write.md` §3.2/§3.3).
**Производит**: `transport.{ArchiveNode,ArchiveDocument}` + Create/Update-варианты, `models.ArchiveNodeQuery`, `httpapi.{ArchiveNode,ArchiveDocument}Service`, `mcp.{ArchiveNode,ArchiveDocument}Service`, HTTP-роуты `/api/archive-nodes*`/`/api/archive-documents*`, MCP-тулы `archive_node_*`/`archive_document_*` (по 6 на сущность) — потребляются Задачей 2 (веб-страницы) и Задачей 3 (веб-ретрофит, косвенно — через уже существующий `/api/attachments`, который подпроект 6 не меняет, но чья документация в `docs/usage.md` ссылается на новые маршруты).

**Файлы:**
- Изменить: `internal/models/query.go` (добавить `ArchiveNodeQuery`), `internal/httpapi/{deps.go,api.go,httpapi.go}`, `internal/mcp/{deps.go,server.go}`, `internal/app/app.go`, `internal/httpapi/{write_store_test.go,store_test.go}` (real-store тесты и фасады), `docs/usage.md`, `docs/architecture.md`, `CHANGELOG.md`, `docs/data-model/entity-write.md`
- Создать: `internal/transport/{archive_node,archive_node_write,archive_document,archive_document_write}.go`, `internal/usecases/{list,search,get,create,update,delete}_archive_node(s)/{deps.go,scenario.go,scenario_test.go}` (и та же шестёрка для `{list,search,get,create,update,delete}_archive_document(s)`), `internal/httpapi/{archive_node,archive_node_write,archive_node_test,archive_node_write_test}.go` (и то же для `archive_document`), `internal/mcp/{archive_node,archive_node_test}.go` (и то же для `archive_document`)

**Важно для исполнителя**: `list_archive_nodes`/`search_archive_nodes`/`delete_archive_node` — механические, по образцу `list_archive`/`search_archive`/`delete_archive`, только `list_archive_nodes` дополнительно фильтрует каждую запись по `ArchiveID == q.ArchiveID && ParentID == q.ParentID` (nil-safe сравнение) вместо выделенного метода стора — full-scan-and-filter по generic-окнам, см. «Глобальные ограничения». `get_archive_node`/`get_archive_document` — access-aware с первого черновика (обе сущности несут `Private`). `create_archive_node`/`update_archive_node` — ОБЯЗАТЕЛЬНЫЙ strict FK (`ArchiveID`, проверяется всегда) ПЛЮС необязательный self-ref (`ParentID`, если задан — проверяется существование И принадлежность тому же архиву — новый инвариант, см. «Глобальные ограничения») ПЛЮС для `update` — обход цепочки родителей на цикл (`checkParentChain`, по образцу `update_note`, но за циклом, не за принадлежностью архиву — та проверяется раньше, для непосредственного родителя). `create_archive_document`/`update_archive_document` — ОБЯЗАТЕЛЬНЫЙ strict FK (`UnitID → ArchiveNode`, всегда). Обе пары сценариев дополнительно проверяют `sources[i].citation_id` в той же транзакции (по образцу подпроекта 5) — обе сущности транзакционные (`InTx`) с рождения, ретрофита `Save → InTx`, в отличие от `Repository`/`Church`/`Parish` в подпроекте 5, здесь не требуется. Переносить код ниже как есть, файл за файлом — итоговое содержимое, не диффы (кроме секции документации, Шаг 1.6, — там даны точные фрагменты-вставки в существующие большие файлы).

### 1.1. `internal/models/query.go` — добавить `ArchiveNodeQuery`

#### `internal/models/query.go` (изменить — итоговое содержимое)
`internal/models/query.go`:
```go
package models

// Access — режим доступа читателя к данным. Нулевое значение — полный доступ
// (владелец); любое другое значение трактуется как публичный доступ, то есть
// фильтр приватного включается, а не отключается (безопасный отказ).
type Access int

const (
	// AccessFull — читатель видит всё, включая сущности с Private = true.
	AccessFull Access = iota
	// AccessPublic — сущности с Private = true скрыты. На сущности без флага
	// приватности (словари, церкви, приходы, деления) режим не влияет.
	AccessPublic
)

// Пределы страницы списка.
const (
	DefaultPageLimit = 50
	MaxPageLimit     = 500
)

// Page — окно списка: не больше Limit записей, начиная с Offset-й. Порядок
// записей стабилен и совпадает с порядком сохранения сущностей.
type Page struct {
	Limit, Offset int
}

// Normalized приводит окно к допустимому виду: Limit меньше либо равный нулю —
// DefaultPageLimit, больше MaxPageLimit — MaxPageLimit; отрицательный Offset — 0.
// Строгая проверка входных значений — дело обработчиков.
func (p Page) Normalized() Page {
	switch {
	case p.Limit <= 0:
		p.Limit = DefaultPageLimit
	case p.Limit > MaxPageLimit:
		p.Limit = MaxPageLimit
	}

	if p.Offset < 0 {
		p.Offset = 0
	}

	return p
}

// Hit — результат поиска: сущность, подпись для показа и поле, по которому
// найдено. Для одной сущности возвращается одна запись (первое совпавшее поле
// по алфавиту имён полей).
type Hit struct {
	Type  Type
	ID    ID
	Label string // подпись сущности: имя, название, заголовок; при пустой — ID
	Field string // поле поискового индекса: name, title, place, …
}

// DivisionKind — предметный вид единицы деления для фильтра списка.
type DivisionKind string

// DivisionKindSettlement — только населённые пункты (AdminDivisionType.IsSettlement).
const DivisionKindSettlement DivisionKind = "settlement"

// Valid сообщает, допустимо ли значение фильтра (пустое — «без фильтра»).
func (k DivisionKind) Valid() bool { return k == "" || k == DivisionKindSettlement }

// DivisionQuery — запрос списка единиц административного деления: необязательные
// фильтры по виду и типу (пересекаются), прямой родитель и окно. Окно применяется
// после фильтра. ParentID — список прямых детей единицы; nil — корень списка.
type DivisionQuery struct {
	Kind     DivisionKind      // "" — без фильтра; settlement — только населённые пункты
	Type     AdminDivisionType // "" — без фильтра; иначе точное совпадение типа
	ParentID *ID               // nil — корень; иначе только прямые дети этой единицы
	Page     Page
}

// Validate проверяет запрос: неизвестные вид и тип, отрицательные размер и
// сдвиг окна, неверный parent_id — *ValidationError (поля названы как параметры
// контракта). Размер окна больше MaxPageLimit не ошибка: Page.Normalized сужает его.
func (q DivisionQuery) Validate() error {
	switch {
	case !q.Kind.Valid():
		return fieldErr("kind", "неизвестный вид %q (допустимо: %q)", q.Kind, DivisionKindSettlement)
	case q.Type != "" && !q.Type.Valid():
		return fieldErr("type", "неизвестный тип единицы деления %q", q.Type)
	case q.Page.Limit < 0:
		return fieldErr("limit", "не может быть отрицательным: %d", q.Page.Limit)
	case q.Page.Offset < 0:
		return fieldErr("offset", "не может быть отрицательным: %d", q.Page.Offset)
	}

	if q.ParentID != nil {
		if err := idErr("parent_id", *q.ParentID, TypeAdministrativeDivision); err != nil {
			return err
		}
	}

	return nil
}

// Matches сообщает, проходит ли единица деления фильтры запроса.
func (q DivisionQuery) Matches(d AdministrativeDivision) bool {
	if q.Kind == DivisionKindSettlement && !d.Type.IsSettlement() {
		return false
	}

	return q.Type == "" || d.Type == q.Type
}

// DivisionSearchQuery — запрос поиска единиц административного деления по началу
// названия (включая варианты). Окно применяется после отбора единиц.
type DivisionSearchQuery struct {
	Text string // начало названия или варианта; пустое (после обрезки) — пустой результат
	Page Page
}

// Validate проверяет запрос: отрицательные размер и сдвиг окна — *ValidationError.
// Пустой текст не ошибка.
func (q DivisionSearchQuery) Validate() error {
	switch {
	case q.Page.Limit < 0:
		return fieldErr("limit", "не может быть отрицательным: %d", q.Page.Limit)
	case q.Page.Offset < 0:
		return fieldErr("offset", "не может быть отрицательным: %d", q.Page.Offset)
	}

	return nil
}

// SearchQuery — запрос поиска по началу названия/канонической формы (включая
// варианты), общий для сущностей без собственных доп. фильтров (см.
// DivisionSearchQuery для сущности с фильтрами). Окно применяется после
// отбора хитов own-типа.
type SearchQuery struct {
	Text string // начало текста; пустое (после обрезки) — пустой результат
	Page Page
}

// Validate проверяет запрос: отрицательные размер и сдвиг окна — *ValidationError.
// Пустой текст не ошибка.
func (q SearchQuery) Validate() error {
	switch {
	case q.Page.Limit < 0:
		return fieldErr("limit", "не может быть отрицательным: %d", q.Page.Limit)
	case q.Page.Offset < 0:
		return fieldErr("offset", "не может быть отрицательным: %d", q.Page.Offset)
	}

	return nil
}

// ArchiveNodeQuery — запрос списка узлов архивного дерева: обязательный
// фильтр по архиву (у узла нет смысла вне архива), необязательный —
// прямой родитель. ParentID — nil — корень (внутри ArchiveID); иначе
// только прямые дети этого узла.
type ArchiveNodeQuery struct {
	ArchiveID ID
	ParentID  *ID
	Page      Page
}

// Validate проверяет запрос: archive_id обязателен и должен быть валидным
// id архива; parent_id (если задан) — валидным id узла; отрицательные
// размер и сдвиг окна — *ValidationError. Размер окна больше MaxPageLimit
// не ошибка: Page.Normalized сужает его.
func (q ArchiveNodeQuery) Validate() error {
	if err := idErr("archive_id", q.ArchiveID, TypeArchive); err != nil {
		return err
	}

	switch {
	case q.Page.Limit < 0:
		return fieldErr("limit", "не может быть отрицательным: %d", q.Page.Limit)
	case q.Page.Offset < 0:
		return fieldErr("offset", "не может быть отрицательным: %d", q.Page.Offset)
	}

	if q.ParentID != nil {
		if err := idErr("parent_id", *q.ParentID, TypeArchiveNode); err != nil {
			return err
		}
	}

	return nil
}
```

### 1.2. ArchiveNode — транспорт, usecases, httpapi, MCP

#### `internal/transport/archive_node.go` (создать)
`internal/transport/archive_node.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// ArchiveNode — контракт узла архивного дерева (GET /api/archive-nodes,
// MCP-тул archive_node_list). ArchiveID — обязательная строгая ссылка на
// архив, ParentID — необязательная ссылка на родительский узел того же
// архива (см. create_archive_node/update_archive_node — сценарии проверяют
// оба инварианта). Parish — одиночная необязательная ссылка (текст или
// ссылка на приход), по образцу Parish.Church/Church.Parish.
type ArchiveNode struct {
	ID          models.ID    `json:"id"`
	Type        string       `json:"type"`
	ArchiveID   models.ID    `json:"archive_id"`
	ParentID    *models.ID   `json:"parent_id,omitempty"`
	Label       string       `json:"label"`
	Name        string       `json:"name"`
	Since       *FactDate    `json:"since,omitempty"`
	Until       *FactDate    `json:"until,omitempty"`
	Parish      *TextRef     `json:"parish,omitempty"`
	Settlements []TextRef    `json:"settlements"`
	Notes       []TextRef    `json:"notes"`
	Sources     []SourceLink `json:"sources"`
	Private     bool         `json:"private"`
}

// ArchiveNodeFromModel конвертирует запись в контракт.
func ArchiveNodeFromModel(n models.ArchiveNode) ArchiveNode {
	return ArchiveNode{
		ID:          n.ID,
		Type:        string(n.Type),
		ArchiveID:   n.ArchiveID,
		ParentID:    n.ParentID,
		Label:       n.Label,
		Name:        n.Name,
		Since:       FactDateFromModel(n.Since),
		Until:       FactDateFromModel(n.Until),
		Parish:      TextRefFromModelPtr(n.Parish),
		Settlements: TextRefsFromModel(n.Settlements),
		Notes:       TextRefsFromModel(n.Notes),
		Sources:     SourceLinksFromModel(n.Sources),
		Private:     n.Private,
	}
}

// ArchiveNodesFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func ArchiveNodesFromModels(ns []models.ArchiveNode) []ArchiveNode {
	out := make([]ArchiveNode, 0, len(ns))
	for _, n := range ns {
		out = append(out, ArchiveNodeFromModel(n))
	}

	return out
}
```

#### `internal/transport/archive_node_write.go` (создать)
`internal/transport/archive_node_write.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// ArchiveNodeCreate — тело POST /api/archive-nodes и аргументы тула
// archive_node_create. Идентификатор генерирует сценарий. Sources —
// редактируемое с самого начала (это новая сущность, не ретрофит, см.
// docs/data-model/entity-write.md §3.4).
type ArchiveNodeCreate struct {
	Type        string       `json:"type"`
	ArchiveID   models.ID    `json:"archive_id"`
	ParentID    *models.ID   `json:"parent_id,omitempty"`
	Label       string       `json:"label"`
	Name        string       `json:"name"`
	Since       *FactDate    `json:"since,omitempty"`
	Until       *FactDate    `json:"until,omitempty"`
	Parish      *TextRef     `json:"parish,omitempty"`
	Settlements []TextRef    `json:"settlements"`
	Notes       []TextRef    `json:"notes"`
	Sources     []SourceLink `json:"sources"`
	Private     bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (n ArchiveNodeCreate) Model() models.ArchiveNode {
	return models.ArchiveNode{
		Type:        models.ArchiveNodeType(n.Type),
		ArchiveID:   n.ArchiveID,
		ParentID:    n.ParentID,
		Label:       n.Label,
		Name:        n.Name,
		Since:       n.Since.Model(),
		Until:       n.Until.Model(),
		Parish:      n.Parish.ModelPtr(),
		Settlements: TextRefsToModel(n.Settlements),
		Notes:       TextRefsToModel(n.Notes),
		Sources:     SourceLinksToModel(n.Sources),
		Private:     n.Private,
	}
}

// ArchiveNodeUpdate — тело PUT /api/archive-nodes/{id} и аргументы тула
// archive_node_update: полная замена всех полей ниже id.
type ArchiveNodeUpdate struct {
	Type        string       `json:"type"`
	ArchiveID   models.ID    `json:"archive_id"`
	ParentID    *models.ID   `json:"parent_id,omitempty"`
	Label       string       `json:"label"`
	Name        string       `json:"name"`
	Since       *FactDate    `json:"since,omitempty"`
	Until       *FactDate    `json:"until,omitempty"`
	Parish      *TextRef     `json:"parish,omitempty"`
	Settlements []TextRef    `json:"settlements"`
	Notes       []TextRef    `json:"notes"`
	Sources     []SourceLink `json:"sources"`
	Private     bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (n ArchiveNodeUpdate) Model() models.ArchiveNode {
	return models.ArchiveNode{
		Type:        models.ArchiveNodeType(n.Type),
		ArchiveID:   n.ArchiveID,
		ParentID:    n.ParentID,
		Label:       n.Label,
		Name:        n.Name,
		Since:       n.Since.Model(),
		Until:       n.Until.Model(),
		Parish:      n.Parish.ModelPtr(),
		Settlements: TextRefsToModel(n.Settlements),
		Notes:       TextRefsToModel(n.Notes),
		Sources:     SourceLinksToModel(n.Sources),
		Private:     n.Private,
	}
}
```

#### `internal/usecases/list_archive_nodes/deps.go` (создать)
`internal/usecases/list_archive_nodes/deps.go`:
```go
package list_archive_nodes

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ArchiveNodeRepo — зависимость сценария: срез порта store.Store. В отличие
// от AdministrativeDivision, у ArchiveNode нет отдельного метода «дети
// узла» — фильтрация по архиву и родителю идёт полным сканированием
// ListArchiveNodes (см. scenario.go).
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveNodeRepo interface {
	ListArchiveNodes(ctx context.Context, access models.Access, page models.Page) ([]*models.ArchiveNode, error)
}
```

#### `internal/usecases/list_archive_nodes/scenario.go` (создать)
`internal/usecases/list_archive_nodes/scenario.go`:
```go
package list_archive_nodes

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список узлов архивного дерева».
type Scenario struct {
	archiveNodes ArchiveNodeRepo
}

// New создаёт сценарий.
func New(archiveNodes ArchiveNodeRepo) *Scenario {
	return &Scenario{archiveNodes: archiveNodes}
}

// ListArchiveNodes возвращает узлы дерева заданного архива (q.ArchiveID
// обязателен), прошедшие фильтры запроса, в порядке сохранения; окно
// применяется после фильтра. С ParentID — только прямые дети этого узла;
// без ParentID — корень дерева внутри архива. Некорректный запрос —
// *models.ValidationError, репозиторий не вызывается. Короткий результат
// (меньше размера окна) означает конец списка.
//
// В отличие от list_divisions (у AdministrativeDivision есть отдельный
// ChildrenOfDivision), у ArchiveNode нет своего метода «дети узла» —
// фильтрация идёт одним проходом по generic-окнам ListArchiveNodes: узел
// проходит, если его ArchiveID совпадает с q.ArchiveID и ParentID совпадает
// с q.ParentID (сравнение nil-safe, см. sameParent). При объёме данных
// этой сущности (архивные деревья, не тысячи записей) полное сканирование
// приемлемо; выделенный метод хранилища не оправдан.
func (s *Scenario) ListArchiveNodes(ctx context.Context, access models.Access, q models.ArchiveNodeQuery) ([]models.ArchiveNode, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	page := q.Page.Normalized()
	out := []models.ArchiveNode{}
	matched := 0 // сколько узлов прошло фильтр (для сдвига окна)

	// репозиторий отдаёт окна: обходим их до пустого или до заполнения окна запроса
	for offset := 0; ; offset += models.MaxPageLimit {
		nodes, err := s.archiveNodes.ListArchiveNodes(ctx, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(nodes) == 0 {
			return out, nil
		}

		var full bool
		if full, matched, out = applyWindow(q, nodes, page, matched, out); full {
			return out, nil
		}
	}
}

// applyWindow прогоняет одно окно репозитория через фильтр запроса и
// накапливает результат окна запроса. full — окно запроса заполнено (обход
// можно остановить).
func applyWindow(q models.ArchiveNodeQuery, nodes []*models.ArchiveNode,
	page models.Page, matched int, out []models.ArchiveNode,
) (full bool, nextMatched int, nextOut []models.ArchiveNode) {
	for _, n := range nodes {
		if n.ArchiveID != q.ArchiveID || !sameParent(n.ParentID, q.ParentID) {
			continue
		}

		if matched >= page.Offset {
			out = append(out, *n)

			if len(out) == page.Limit {
				return true, matched, out
			}
		}

		matched++
	}

	return false, matched, out
}

// sameParent сравнивает два необязательных id родителя nil-safe: оба nil —
// равны; один nil — не равны; иначе сравниваются значения.
func sameParent(a, b *models.ID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	return *a == *b
}
```

#### `internal/usecases/list_archive_nodes/scenario_test.go` (создать)
`internal/usecases/list_archive_nodes/scenario_test.go`:
```go
package list_archive_nodes

import (
	"context"
	"errors"
	"strconv"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type ctxKey struct{}

// fakeRepo отдаёт окна списка как настоящий репозиторий и запоминает вызовы.
type fakeRepo struct {
	list     []*models.ArchiveNode
	err      error
	errAt    int // номер вызова (с 1), на котором возвращается err; 0 — на любом
	gotCtx   context.Context
	calls    []models.Page
	accesses []models.Access
}

// window нарезает список по окну, как настоящий репозиторий.
func window(list []*models.ArchiveNode, page models.Page) []*models.ArchiveNode {
	page = page.Normalized()
	if page.Offset >= len(list) {
		return nil
	}

	return list[page.Offset:min(page.Offset+page.Limit, len(list))]
}

func (f *fakeRepo) ListArchiveNodes(ctx context.Context, access models.Access, page models.Page) ([]*models.ArchiveNode, error) {
	f.gotCtx = ctx
	f.calls = append(f.calls, page)
	f.accesses = append(f.accesses, access)

	if f.err != nil && (f.errAt == 0 || f.errAt == len(f.calls)) {
		return nil, f.err
	}

	return window(f.list, page), nil
}

const (
	archive1 = models.ID("AR-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	archive2 = models.ID("AR-01ARZ3NDEKTSV4RRFFQ69G5FA2")
)

// anID возвращает корректный идентификатор узла, отличающийся последним символом.
func anID(last byte) models.ID {
	return models.ID("AN-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func node(id models.ID, archive models.ID, parent *models.ID) *models.ArchiveNode {
	return &models.ArchiveNode{ID: id, Type: "fond", ArchiveID: archive, ParentID: parent, Label: string(id)}
}

func ids(list []models.ArchiveNode) []models.ID {
	out := []models.ID{}
	for _, n := range list {
		out = append(out, n.ID)
	}

	return out
}

func sameIDs(got []models.ID, want ...models.ID) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}

	return true
}

var (
	root1 = anID('1')
	node2 = anID('2')
	node3 = anID('3')
	node4 = anID('4')
	node5 = anID('5')
)

func sample() *fakeRepo {
	return &fakeRepo{list: []*models.ArchiveNode{
		node(root1, archive1, nil),
		node(node2, archive1, &root1),
		node(node3, archive2, nil),
		node(node4, archive1, nil),
		node(node5, archive1, &root1),
	}}
}

func TestListArchiveNodesRootOfArchive(t *testing.T) {
	got, err := New(sample()).ListArchiveNodes(context.Background(), models.AccessFull, models.ArchiveNodeQuery{ArchiveID: archive1})
	if err != nil || !sameIDs(ids(got), root1, node4) {
		t.Fatalf("got %v, %v; ожидались корневые узлы root1, node4 архива archive1", ids(got), err)
	}
}

func TestListArchiveNodesOtherArchiveNotMixed(t *testing.T) {
	got, err := New(sample()).ListArchiveNodes(context.Background(), models.AccessFull, models.ArchiveNodeQuery{ArchiveID: archive2})
	if err != nil || !sameIDs(ids(got), node3) {
		t.Fatalf("got %v, %v; ожидался только node3 (другой архив)", ids(got), err)
	}
}

func TestListArchiveNodesChildrenOfParent(t *testing.T) {
	parent := root1

	got, err := New(sample()).ListArchiveNodes(context.Background(), models.AccessFull,
		models.ArchiveNodeQuery{ArchiveID: archive1, ParentID: &parent})
	if err != nil || !sameIDs(ids(got), node2, node5) {
		t.Fatalf("got %v, %v; ожидались дети root1: node2, node5", ids(got), err)
	}
}

// TestListArchiveNodesParentFromOtherArchiveNotConfused: узел с ParentID,
// совпадающим по значению, но принадлежащий другому архиву, не путается с
// запросом на другой архив — фильтр по ArchiveID применяется независимо.
func TestListArchiveNodesParentFromOtherArchiveNotConfused(t *testing.T) {
	parent := root1

	got, err := New(sample()).ListArchiveNodes(context.Background(), models.AccessFull,
		models.ArchiveNodeQuery{ArchiveID: archive2, ParentID: &parent})
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v; ожидался пустой результат (в archive2 нет детей root1)", ids(got), err)
	}
}

// TestListArchiveNodesWindowIsAppliedAfterFilter: сдвиг и размер считаются
// среди прошедших фильтр, а не среди всех узлов.
func TestListArchiveNodesWindowIsAppliedAfterFilter(t *testing.T) {
	q := models.ArchiveNodeQuery{ArchiveID: archive1, Page: models.Page{Limit: 1, Offset: 1}}

	got, err := New(sample()).ListArchiveNodes(context.Background(), models.AccessFull, q)
	if err != nil || !sameIDs(ids(got), node4) {
		t.Fatalf("got %v, %v; ожидался второй корневой узел node4", ids(got), err)
	}
}

// TestListArchiveNodesWalksAllRepositoryWindows: узлы за первым окном
// репозитория не теряются, окна запрашиваются подряд; access, переданный
// вызывающим, доходит до каждого окна репозитория как есть.
func TestListArchiveNodesWalksAllRepositoryWindows(t *testing.T) {
	total := 2*models.MaxPageLimit + 7

	repo := &fakeRepo{}

	for i := 0; i < total; i++ {
		a := archive1
		if i%2 == 1 {
			a = archive2 // не тот архив
		}

		repo.list = append(repo.list, node(models.ID("AN-"+padded(i)), a, nil))
	}

	got, err := New(repo).ListArchiveNodes(context.Background(), models.AccessPublic,
		models.ArchiveNodeQuery{ArchiveID: archive1, Page: models.Page{Limit: models.MaxPageLimit}})
	if err != nil || len(got) != models.MaxPageLimit {
		t.Fatalf("got %d, %v; ожидалось %d", len(got), err, models.MaxPageLimit)
	}

	if len(repo.calls) != 2 {
		t.Fatalf("вызовов репозитория %d (%v), ожидалось 2", len(repo.calls), repo.calls)
	}

	for i, a := range repo.accesses {
		if a != models.AccessPublic {
			t.Errorf("вызов %d: доступ %v, ожидался AccessPublic (переданный вызывающим)", i+1, a)
		}
	}
}

// TestListArchiveNodesInvalidQueryDoesNotTouchRepo: некорректный запрос —
// *ValidationError, репозиторий не вызывается.
func TestListArchiveNodesInvalidQueryDoesNotTouchRepo(t *testing.T) {
	repo := sample()
	bad := models.ID("not-an-id")

	for _, q := range []models.ArchiveNodeQuery{
		{}, // ArchiveID обязателен
		{ArchiveID: archive1, ParentID: &bad},
		{ArchiveID: archive1, Page: models.Page{Limit: -1}},
		{ArchiveID: archive1, Page: models.Page{Offset: -1}},
	} {
		_, err := New(repo).ListArchiveNodes(context.Background(), models.AccessFull, q)

		var ve *models.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("запрос %+v: err = %v, ожидалась *ValidationError", q, err)
		}
	}

	if len(repo.calls) != 0 {
		t.Fatalf("репозиторий вызван %d раз при некорректном запросе", len(repo.calls))
	}
}

func TestListArchiveNodesEmptyRepoGivesEmptyNotNil(t *testing.T) {
	got, err := New(&fakeRepo{}).ListArchiveNodes(context.Background(), models.AccessFull, models.ArchiveNodeQuery{ArchiveID: archive1})
	if err != nil {
		t.Fatalf("ListArchiveNodes: %v", err)
	}

	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v, ожидался пустой не-nil срез", got)
	}
}

func TestListArchiveNodesPropagatesRepoError(t *testing.T) {
	wantErr := errors.New("repo down")

	if _, err := New(&fakeRepo{err: wantErr}).ListArchiveNodes(context.Background(), models.AccessFull, models.ArchiveNodeQuery{ArchiveID: archive1}); !errors.Is(err, wantErr) {
		t.Errorf("err=%v, want %v", err, wantErr)
	}
}

// TestListArchiveNodesPropagatesErrorFromLaterWindow: сбой на втором окне
// репозитория не даёт частичного результата (первое окно репозитория,
// наполовину отфильтрованное по архиву, даёт лишь половину окна запроса,
// поэтому читается и второе).
func TestListArchiveNodesPropagatesErrorFromLaterWindow(t *testing.T) {
	wantErr := errors.New("repo down on window 2")

	repo := &fakeRepo{err: wantErr, errAt: 2}

	for i := 0; i < models.MaxPageLimit+1; i++ {
		a := archive1
		if i%2 == 1 {
			a = archive2
		}

		repo.list = append(repo.list, node(models.ID("AN-"+padded(i)), a, nil))
	}

	got, err := New(repo).ListArchiveNodes(context.Background(), models.AccessFull,
		models.ArchiveNodeQuery{ArchiveID: archive1, Page: models.Page{Limit: models.MaxPageLimit}})
	if !errors.Is(err, wantErr) || got != nil {
		t.Fatalf("got %v, %v; ожидалась ошибка %v без результата", ids(got), err, wantErr)
	}
}

func TestListArchiveNodesPassesContextToRepo(t *testing.T) {
	repo := &fakeRepo{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "marker")

	if _, err := New(repo).ListArchiveNodes(ctx, models.AccessFull, models.ArchiveNodeQuery{ArchiveID: archive1}); err != nil {
		t.Fatalf("ListArchiveNodes: %v", err)
	}

	if repo.gotCtx == nil || repo.gotCtx.Value(ctxKey{}) != "marker" {
		t.Fatalf("репозиторий получил контекст %v, ожидался переданный сценарию", repo.gotCtx)
	}
}

// padded возвращает номер, дополненный нулями слева до 20 знаков — узлы этих
// тестов не проверяются на формат id (только archive_id/parent_id запроса),
// но детерминированный текст полезен для отладки при падении теста.
func padded(i int) string {
	s := strconv.Itoa(i)
	for len(s) < 20 {
		s = "0" + s
	}

	return s
}
```

#### `internal/usecases/search_archive_nodes/deps.go` (создать)
`internal/usecases/search_archive_nodes/deps.go`:
```go
package search_archive_nodes

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ArchiveNodeRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveNodeRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetArchiveNode(ctx context.Context, id models.ID) (*models.ArchiveNode, error)
}
```

#### `internal/usecases/search_archive_nodes/scenario.go` (создать)
`internal/usecases/search_archive_nodes/scenario.go`:
```go
package search_archive_nodes

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск узлов архивного дерева».
type Scenario struct {
	archiveNodes ArchiveNodeRepo
}

// New создаёт сценарий.
func New(archiveNodes ArchiveNodeRepo) *Scenario {
	return &Scenario{archiveNodes: archiveNodes}
}

// SearchArchiveNodes находит узлы, чьи label/name (индексируются вместе под
// полем "name", см. internal/store/sqlstore/archive.go) начинаются с текста
// запроса, и возвращает их целиком. Результат — в том порядке, в каком их
// отдаёт Search, без фильтра по архиву (поиск глобальный по всем архивам —
// сужение по архиву, если понадобится, веб-сторона делает сама). Окно
// (размер и сдвиг) применяется после отбора хитов own-типа: хиты других
// сущностей не расходуют окно. Пустой текст (после обрезки) — пустой
// результат без обращения к репозиторию. Хит, чей узел удалён между поиском
// и чтением (ErrNotFound), пропускается; прочие ошибки пробрасываются.
func (s *Scenario) SearchArchiveNodes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.ArchiveNode, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.ArchiveNode{}, nil
	}

	page := q.Page.Normalized()
	out := []models.ArchiveNode{}
	matched := 0 // сколько хитов узлов прошло (для сдвига окна)

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.archiveNodes.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeArchiveNode {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.archiveNodes.GetArchiveNode(ctx, h.ID)
			if err != nil {
				if errors.Is(err, models.ErrNotFound) {
					matched++

					continue
				}

				return nil, err
			}

			out = append(out, *got)

			if len(out) == page.Limit {
				return out, nil
			}

			matched++
		}
	}
}
```

#### `internal/usecases/search_archive_nodes/scenario_test.go` (создать)
`internal/usecases/search_archive_nodes/scenario_test.go`:
```go
package search_archive_nodes

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

// hit строит хиты поиска: только нужные поля.
func hit(id string, typ models.Type) models.Hit {
	return models.Hit{Type: typ, ID: models.ID(id), Label: id}
}

// archiveNode строит узел архивного дерева.
func archiveNode(id string) *models.ArchiveNode {
	return &models.ArchiveNode{ID: models.ID(id), Label: id}
}

// ids извлекает идентификаторы узлов.
func ids(nodes []models.ArchiveNode) []models.ID {
	out := make([]models.ID, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.ID)
	}

	return out
}

func sameIDs[T ~string](got []T, want ...T) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}

	return true
}

// fakeRepo отдаёт окна хитов и узлы.
type fakeRepo struct {
	hits     []models.Hit
	getErr   map[models.ID]error
	err      error
	errAt    int
	gotQuery string
	calls    []models.Page
	getCalls []models.ID
	accesses []models.Access
}

func (f *fakeRepo) Search(_ context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error) {
	f.gotQuery = query
	f.calls = append(f.calls, page)
	f.accesses = append(f.accesses, access)

	if f.err != nil && (f.errAt == 0 || f.errAt == len(f.calls)) {
		return nil, f.err
	}

	page = page.Normalized()
	if page.Offset >= len(f.hits) {
		return nil, nil
	}

	end := min(page.Offset+page.Limit, len(f.hits))

	return f.hits[page.Offset:end], nil
}

func (f *fakeRepo) GetArchiveNode(_ context.Context, id models.ID) (*models.ArchiveNode, error) {
	f.getCalls = append(f.getCalls, id)

	if f.getErr != nil {
		if e, ok := f.getErr[id]; ok && e != nil {
			return nil, e
		}
	}

	return archiveNode(string(id)), nil
}

func TestSearchArchiveNodesEmptyTextDoesNotTouchRepo(t *testing.T) {
	repo := &fakeRepo{}

	got, err := New(repo).SearchArchiveNodes(context.Background(), models.AccessFull, models.SearchQuery{Text: "   "})
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v; ожидался пустой результат", got, err)
	}
	if len(repo.calls) != 0 {
		t.Fatalf("Search вызван %d раз при пустом тексте", len(repo.calls))
	}
}

func TestSearchArchiveNodesReturnsFullNodesFromHits(t *testing.T) {
	repo := &fakeRepo{
		hits: []models.Hit{
			hit("D-1001", models.TypePerson),
			hit("AN-1002", models.TypeArchiveNode),
			hit("F-1003", models.TypeFamily),
			hit("AN-1004", models.TypeArchiveNode),
		},
	}

	got, err := New(repo).SearchArchiveNodes(context.Background(), models.AccessFull, models.SearchQuery{Text: "фонд"})
	if err != nil || !sameIDs(ids(got), "AN-1002", "AN-1004") {
		t.Fatalf("got %v, %v; ожидались AN-1002, AN-1004", ids(got), err)
	}
	if repo.gotQuery != "фонд" {
		t.Fatalf("Search получил %q, ожидался %q", repo.gotQuery, "фонд")
	}
}

func TestSearchArchiveNodesAppliesWindowAmongHits(t *testing.T) {
	repo := &fakeRepo{
		hits: []models.Hit{
			hit("D-1001", models.TypePerson),
			hit("AN-1002", models.TypeArchiveNode),
			hit("AN-1003", models.TypeArchiveNode),
			hit("AN-1004", models.TypeArchiveNode),
		},
	}

	got, err := New(repo).SearchArchiveNodes(context.Background(), models.AccessFull, models.SearchQuery{
		Text: "п",
		Page: models.Page{Limit: 1, Offset: 1},
	})
	if err != nil || !sameIDs(ids(got), "AN-1003") {
		t.Fatalf("got %v, %v; ожидалась AN-1003 (второй хит)", ids(got), err)
	}
}

func TestSearchArchiveNodesSkipsDeletedNode(t *testing.T) {
	repo := &fakeRepo{
		hits: []models.Hit{
			hit("AN-1002", models.TypeArchiveNode),
			hit("AN-1003", models.TypeArchiveNode),
		},
		getErr: map[models.ID]error{"AN-1002": models.ErrNotFound},
	}

	got, err := New(repo).SearchArchiveNodes(context.Background(), models.AccessFull, models.SearchQuery{Text: "п"})
	if err != nil {
		t.Fatalf("SearchArchiveNodes: %v", err)
	}
	if !sameIDs(ids(got), "AN-1003") {
		t.Fatalf("got %v; AN-1002 удалён — остаётся AN-1003", ids(got))
	}
}

func TestSearchArchiveNodesPropagatesGetError(t *testing.T) {
	boom := errors.New("get boom")
	repo := &fakeRepo{
		hits:   []models.Hit{hit("AN-1002", models.TypeArchiveNode)},
		getErr: map[models.ID]error{"AN-1002": boom},
	}

	_, err := New(repo).SearchArchiveNodes(context.Background(), models.AccessFull, models.SearchQuery{Text: "п"})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, ожидался boom", err)
	}
}

func TestSearchArchiveNodesPropagatesSearchError(t *testing.T) {
	boom := errors.New("search boom")
	repo := &fakeRepo{err: boom}

	_, err := New(repo).SearchArchiveNodes(context.Background(), models.AccessFull, models.SearchQuery{Text: "п"})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, ожидался boom", err)
	}
}

func TestSearchArchiveNodesInvalidQueryDoesNotTouchRepo(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).SearchArchiveNodes(context.Background(), models.AccessFull, models.SearchQuery{Page: models.Page{Limit: -1}})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, ожидалась *ValidationError поля limit", err)
	}
	if len(repo.calls) != 0 {
		t.Fatalf("Search вызван %d раз при некорректном запросе", len(repo.calls))
	}
}

func TestSearchArchiveNodesPassesAccessToRepo(t *testing.T) {
	repo := &fakeRepo{hits: []models.Hit{
		hit("AN-1002", models.TypeArchiveNode),
	}}

	if _, err := New(repo).SearchArchiveNodes(context.Background(), models.AccessPublic,
		models.SearchQuery{Text: "п"}); err != nil {
		t.Fatalf("SearchArchiveNodes: %v", err)
	}

	for i, a := range repo.accesses {
		if a != models.AccessPublic {
			t.Errorf("вызов %d: доступ %v, ожидался AccessPublic", i+1, a)
		}
	}
}
```

#### `internal/usecases/get_archive_node/deps.go` (создать)
`internal/usecases/get_archive_node/deps.go`:
```go
package get_archive_node

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ArchiveNodeRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveNodeRepo interface {
	GetArchiveNode(ctx context.Context, id models.ID) (*models.ArchiveNode, error)
}
```

#### `internal/usecases/get_archive_node/scenario.go` (создать)
`internal/usecases/get_archive_node/scenario.go`:
```go
package get_archive_node

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «узел архивного дерева по идентификатору».
type Scenario struct {
	archiveNodes ArchiveNodeRepo
}

// New создаёт сценарий.
func New(archiveNodes ArchiveNodeRepo) *Scenario {
	return &Scenario{archiveNodes: archiveNodes}
}

// GetArchiveNode возвращает узел по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такого узла — models.ErrNotFound. Приватный узел
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound — тот же принцип «прячем как отсутствующее», что и в
// List/Search (docs/data-model/entity-write.md §3.1).
func (s *Scenario) GetArchiveNode(ctx context.Context, access models.Access, id models.ID) (models.ArchiveNode, error) {
	if err := validateID(id); err != nil {
		return models.ArchiveNode{}, err
	}

	n, err := s.archiveNodes.GetArchiveNode(ctx, id)
	if err != nil {
		return models.ArchiveNode{}, err
	}

	if n.Private && access != models.AccessFull {
		return models.ArchiveNode{}, models.ErrNotFound
	}

	return *n, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeArchiveNode)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeArchiveNode,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

#### `internal/usecases/get_archive_node/scenario_test.go` (создать)
`internal/usecases/get_archive_node/scenario_test.go`:
```go
package get_archive_node

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	nodes map[models.ID]*models.ArchiveNode
}

func (f *fakeRepo) GetArchiveNode(_ context.Context, id models.ID) (*models.ArchiveNode, error) {
	n, ok := f.nodes[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return n, nil
}

func TestGetArchiveNodeReturnsRecord(t *testing.T) {
	id := models.ID("AN-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{nodes: map[models.ID]*models.ArchiveNode{id: {ID: id, Label: "Фонд 1"}}}

	got, err := New(repo).GetArchiveNode(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetArchiveNode: %v", err)
	}

	if got.Label != "Фонд 1" {
		t.Fatalf("Label = %q", got.Label)
	}
}

func TestGetArchiveNodeNotFound(t *testing.T) {
	repo := &fakeRepo{nodes: map[models.ID]*models.ArchiveNode{}}

	_, err := New(repo).GetArchiveNode(context.Background(), models.AccessFull, "AN-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetArchiveNodeInvalidID(t *testing.T) {
	repo := &fakeRepo{nodes: map[models.ID]*models.ArchiveNode{}}

	_, err := New(repo).GetArchiveNode(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestGetArchiveNodePrivateHiddenFromPublic(t *testing.T) {
	id := models.ID("AN-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{nodes: map[models.ID]*models.ArchiveNode{id: {ID: id, Label: "Фонд 1", Private: true}}}

	_, err := New(repo).GetArchiveNode(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for private record with AccessPublic", err)
	}
}

func TestGetArchiveNodePrivateVisibleToFullAccess(t *testing.T) {
	id := models.ID("AN-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{nodes: map[models.ID]*models.ArchiveNode{id: {ID: id, Label: "Фонд 1", Private: true}}}

	got, err := New(repo).GetArchiveNode(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetArchiveNode: %v", err)
	}

	if got.Label != "Фонд 1" {
		t.Fatalf("Label = %q", got.Label)
	}
}
```

#### `internal/usecases/create_archive_node/deps.go` (создать)
`internal/usecases/create_archive_node/deps.go`:
```go
package create_archive_node

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// ArchiveNodeStore — зависимость сценария: транзакция порта store.Store.
// Проверка архива, родителя (если задан) и сохранение идут в одной
// транзакции на переданном fn хранилище.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveNodeStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
```

#### `internal/usecases/create_archive_node/scenario.go` (создать)
`internal/usecases/create_archive_node/scenario.go`:
```go
package create_archive_node

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание узла архивного дерева».
type Scenario struct {
	store ArchiveNodeStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st ArchiveNodeStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateArchiveNode создаёт узел: генерирует идентификатор, проверяет
// инварианты, в одной транзакции убеждается в существовании архива и (если
// задан) родителя, проверяет, что родитель принадлежит тому же архиву, и
// сохраняет. Возвращает созданный узел с заполненным ID.
//
// Ошибки: непустой входной ID, невалидная сущность, несуществующий архив
// или родитель, родитель из другого архива —
// *models.ValidationError (поля id, archive_id, parent_id); прочее —
// ошибки хранилища как есть.
func (s *Scenario) CreateArchiveNode(ctx context.Context, n models.ArchiveNode) (models.ArchiveNode, error) {
	if n.ID != "" {
		return models.ArchiveNode{}, &models.ValidationError{
			Entity: models.TypeArchiveNode,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", n.ID),
		}
	}

	n.ID = s.ids.New(models.TypeArchiveNode)

	if err := n.Validate(); err != nil {
		return models.ArchiveNode{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetArchive(ctx, n.ArchiveID); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return archiveErr("архив %q не найден", n.ArchiveID)
			}

			return err
		}

		if n.ParentID != nil {
			parent, err := tx.GetArchiveNode(ctx, *n.ParentID)
			if err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return parentErr("родитель %q не найден", *n.ParentID)
				}

				return err
			}

			if parent.ArchiveID != n.ArchiveID {
				return parentErr("родитель %q принадлежит другому архиву", *n.ParentID)
			}
		}

		for i, link := range n.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveArchiveNode(ctx, &n)
	})
	if err != nil {
		return models.ArchiveNode{}, err
	}

	return n, nil
}

// archiveErr — *models.ValidationError по полю archive_id.
func archiveErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeArchiveNode,
		Field:  "archive_id",
		Reason: fmt.Sprintf(format, args...),
	}
}

// parentErr — *models.ValidationError по полю parent_id.
func parentErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeArchiveNode,
		Field:  "parent_id",
		Reason: fmt.Sprintf(format, args...),
	}
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeArchiveNode,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/create_archive_node/scenario_test.go` (создать)
`internal/usecases/create_archive_node/scenario_test.go`:
```go
package create_archive_node

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// anID возвращает корректный идентификатор узла, отличающийся последним символом.
func anID(last byte) models.ID {
	return models.ID("AN-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// arID возвращает корректный идентификатор архива.
func arID(last byte) models.ID {
	return models.ID("AR-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// cID возвращает корректный идентификатор цитаты.
func cID(last byte) models.ID {
	return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

type stubIDs struct {
	id      models.ID
	gotType models.Type
}

func (s *stubIDs) New(t models.Type) models.ID {
	s.gotType = t

	return s.id
}

// fakeTx реализует нужные сценарию методы store.Store поверх карт архивов,
// узлов и цитат; остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	archives  map[models.ID]*models.Archive
	nodes     map[models.ID]*models.ArchiveNode
	citations map[models.ID]*models.Citation
	saved     []*models.ArchiveNode
	saveErr   error
}

func newFakeTx() *fakeTx {
	return &fakeTx{
		archives:  map[models.ID]*models.Archive{},
		nodes:     map[models.ID]*models.ArchiveNode{},
		citations: map[models.ID]*models.Citation{},
	}
}

func (f *fakeTx) withArchive(a *models.Archive) *fakeTx {
	f.archives[a.ID] = a

	return f
}

func (f *fakeTx) withNode(n *models.ArchiveNode) *fakeTx {
	f.nodes[n.ID] = n

	return f
}

func (f *fakeTx) withCitation(c *models.Citation) *fakeTx {
	f.citations[c.ID] = c

	return f
}

func (f *fakeTx) GetArchive(_ context.Context, id models.ID) (*models.Archive, error) {
	a, ok := f.archives[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *a

	return &cp, nil
}

func (f *fakeTx) GetArchiveNode(_ context.Context, id models.ID) (*models.ArchiveNode, error) {
	n, ok := f.nodes[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *n

	return &cp, nil
}

func (f *fakeTx) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *c

	return &cp, nil
}

func (f *fakeTx) SaveArchiveNode(_ context.Context, n *models.ArchiveNode) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *n
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует ArchiveNodeStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx      *fakeTx
	inTxErr error
	calls   int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	if f.inTxErr != nil {
		return f.inTxErr
	}

	return fn(f.tx)
}

func validInput(archive models.ID) models.ArchiveNode {
	return models.ArchiveNode{Type: "fond", ArchiveID: archive, Label: "Фонд 1"}
}

func TestCreateArchiveNodeGeneratesIDAndSaves(t *testing.T) {
	archive := arID('0')
	st := &fakeStore{tx: newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"})}
	ids := &stubIDs{id: anID('V')}

	got, err := New(st, ids).CreateArchiveNode(context.Background(), validInput(archive))
	if err != nil {
		t.Fatalf("CreateArchiveNode: %v", err)
	}

	if got.ID != anID('V') || got.Label != "Фонд 1" {
		t.Fatalf("got %+v, ожидался узел с ID %v", got, anID('V'))
	}

	if ids.gotType != models.TypeArchiveNode {
		t.Errorf("генератор вызван с типом %q, ожидался archive_node", ids.gotType)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].ID != anID('V') {
		t.Fatalf("InTx=%d, saved=%v; ожидалось одно сохранение внутри одной транзакции", st.calls, st.tx.saved)
	}
}

func TestCreateArchiveNodeRejectsExplicitID(t *testing.T) {
	archive := arID('0')
	st := &fakeStore{tx: newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"})}
	ids := &stubIDs{id: anID('V')}

	in := validInput(archive)
	in.ID = anID('0')

	_, err := New(st, ids).CreateArchiveNode(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю id", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx=%d; ожидалось: не вызван", st.calls)
	}
}

func TestCreateArchiveNodeValidatesBeforeTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	in := validInput(arID('0'))
	in.Label = ""

	_, err := New(st, &stubIDs{id: anID('V')}).CreateArchiveNode(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "label" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю label", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestCreateArchiveNodeArchiveNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	_, err := New(st, &stubIDs{id: anID('V')}).CreateArchiveNode(context.Background(), validInput(arID('0')))

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "archive_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю archive_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d узлов при несуществующем архиве", len(st.tx.saved))
	}
}

func TestCreateArchiveNodeWithParentSaves(t *testing.T) {
	archive := arID('0')
	parent := &models.ArchiveNode{ID: anID('P'), Type: "fond", ArchiveID: archive, Label: "Родитель"}
	tx := newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"}).withNode(parent)
	st := &fakeStore{tx: tx}

	in := validInput(archive)
	pid := parent.ID
	in.ParentID = &pid

	got, err := New(st, &stubIDs{id: anID('V')}).CreateArchiveNode(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateArchiveNode: %v", err)
	}

	if got.ParentID == nil || *got.ParentID != parent.ID || len(st.tx.saved) != 1 {
		t.Fatalf("got %+v, saved=%d; ожидалось сохранение со ссылкой на родителя", got, len(st.tx.saved))
	}
}

func TestCreateArchiveNodeParentNotFound(t *testing.T) {
	archive := arID('0')
	st := &fakeStore{tx: newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"})}

	in := validInput(archive)
	missing := anID('9')
	in.ParentID = &missing

	_, err := New(st, &stubIDs{id: anID('V')}).CreateArchiveNode(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d узлов при несуществующем родителе", len(st.tx.saved))
	}
}

// TestCreateArchiveNodeParentFromOtherArchiveRejected: родитель существует,
// но принадлежит другому архиву — *ValidationError по полю parent_id
// (не archive_id: сам архив существует, инвариант — «родитель из того же
// архива»).
func TestCreateArchiveNodeParentFromOtherArchiveRejected(t *testing.T) {
	archive := arID('0')
	otherArchive := arID('1')
	parent := &models.ArchiveNode{ID: anID('P'), Type: "fond", ArchiveID: otherArchive, Label: "Чужой родитель"}
	tx := newFakeTx().
		withArchive(&models.Archive{ID: archive, Name: "ГАВО"}).
		withArchive(&models.Archive{ID: otherArchive, Name: "РГАДА"}).
		withNode(parent)
	st := &fakeStore{tx: tx}

	in := validInput(archive)
	pid := parent.ID
	in.ParentID = &pid

	_, err := New(st, &stubIDs{id: anID('V')}).CreateArchiveNode(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id (родитель из другого архива)", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d узлов при родителе из другого архива", len(st.tx.saved))
	}
}

func TestCreateArchiveNodePropagatesSaveError(t *testing.T) {
	wantErr := errors.New("save failed")
	archive := arID('0')
	tx := newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"})
	tx.saveErr = wantErr
	st := &fakeStore{tx: tx}

	if _, err := New(st, &stubIDs{id: anID('V')}).CreateArchiveNode(context.Background(), validInput(archive)); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

func TestCreateArchiveNodePropagatesTxError(t *testing.T) {
	wantErr := errors.New("tx failed")
	st := &fakeStore{tx: newFakeTx(), inTxErr: wantErr}

	if _, err := New(st, &stubIDs{id: anID('V')}).CreateArchiveNode(context.Background(), validInput(arID('0'))); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

// TestCreateArchiveNodeSourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestCreateArchiveNodeSourceCitationNotFound(t *testing.T) {
	archive := arID('0')
	st := &fakeStore{tx: newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"})}

	in := validInput(archive)
	in.Sources = []models.SourceLink{{CitationID: cID('0')}}

	_, err := New(st, &stubIDs{id: anID('V')}).CreateArchiveNode(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d узлов при несуществующей цитате", len(st.tx.saved))
	}
}
```

#### `internal/usecases/update_archive_node/deps.go` (создать)
`internal/usecases/update_archive_node/deps.go`:
```go
package update_archive_node

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// ArchiveNodeStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования, архива, родителя (в т.ч. цепочки на цикл) и
// сохранение идут в одной транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveNodeStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
```

#### `internal/usecases/update_archive_node/scenario.go` (создать)
`internal/usecases/update_archive_node/scenario.go`:
```go
package update_archive_node

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение узла архивного дерева».
type Scenario struct {
	store ArchiveNodeStore
}

// New создаёт сценарий.
func New(st ArchiveNodeStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateArchiveNode полностью заменяет узел по n.ID: проверяет инварианты, в
// одной транзакции убеждается, что узел существует, архив существует,
// родитель (если задан) существует и принадлежит тому же архиву, а цепочка
// родителей не проходит через сам узел (цикл), и сохраняет.
//
// Ошибки: невалидная сущность — *models.ValidationError соответствующего
// поля; несуществующий архив/родитель, родитель из другого архива, цикл по
// parent_id — *models.ValidationError (archive_id/parent_id); нет такого
// узла — models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateArchiveNode(ctx context.Context, n models.ArchiveNode) error {
	if err := n.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetArchiveNode(ctx, n.ID); err != nil {
			return err
		}

		if _, err := tx.GetArchive(ctx, n.ArchiveID); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return archiveErr("архив %q не найден", n.ArchiveID)
			}

			return err
		}

		if n.ParentID != nil {
			parent, err := tx.GetArchiveNode(ctx, *n.ParentID)
			if err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return parentErr("родитель %q не найден", *n.ParentID)
				}

				return err
			}

			if parent.ArchiveID != n.ArchiveID {
				return parentErr("родитель %q принадлежит другому архиву", *n.ParentID)
			}
		}

		if err := checkParentChain(ctx, tx, &n); err != nil {
			return err
		}

		for i, link := range n.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveArchiveNode(ctx, &n)
	})
}

// checkParentChain обходит цепочку родителей n вверх: каждый предок должен
// существовать, и цепочка не должна проходить через сам узел или
// замыкаться (защита от испорченных данных). Ошибка цепочки —
// *models.ValidationError по полю parent_id. По образцу
// update_note.checkParentChain — существование непосредственного родителя и
// совпадение архива уже проверены выше, здесь только обход выше по цепочке.
func checkParentChain(ctx context.Context, tx store.Store, n *models.ArchiveNode) error {
	seen := map[models.ID]bool{n.ID: true}

	for cur := n.ParentID; cur != nil; {
		if seen[*cur] {
			return parentErr("цепочка родителей %q проходит через сам узел или замыкается на %q", n.ID, *cur)
		}
		seen[*cur] = true

		p, err := tx.GetArchiveNode(ctx, *cur)
		if err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return parentErr("родитель %q не найден", *cur)
			}

			return err
		}

		cur = p.ParentID
	}

	return nil
}

// archiveErr — *models.ValidationError по полю archive_id.
func archiveErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeArchiveNode,
		Field:  "archive_id",
		Reason: fmt.Sprintf(format, args...),
	}
}

// parentErr — *models.ValidationError по полю parent_id.
func parentErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeArchiveNode,
		Field:  "parent_id",
		Reason: fmt.Sprintf(format, args...),
	}
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeArchiveNode,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/update_archive_node/scenario_test.go` (создать)
`internal/usecases/update_archive_node/scenario_test.go`:
```go
package update_archive_node

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// anID возвращает корректный идентификатор узла, отличающийся последним символом.
func anID(last byte) models.ID {
	return models.ID("AN-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// arID возвращает корректный идентификатор архива.
func arID(last byte) models.ID {
	return models.ID("AR-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// cID возвращает корректный идентификатор цитаты.
func cID(last byte) models.ID {
	return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// fakeTx реализует нужные сценарию методы store.Store поверх карт архивов,
// узлов и цитат; остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	archives  map[models.ID]*models.Archive
	nodes     map[models.ID]*models.ArchiveNode
	citations map[models.ID]*models.Citation
	saved     []*models.ArchiveNode
	saveErr   error
}

func newFakeTx() *fakeTx {
	return &fakeTx{
		archives:  map[models.ID]*models.Archive{},
		nodes:     map[models.ID]*models.ArchiveNode{},
		citations: map[models.ID]*models.Citation{},
	}
}

func (f *fakeTx) withArchive(a *models.Archive) *fakeTx {
	f.archives[a.ID] = a

	return f
}

func (f *fakeTx) withNode(n *models.ArchiveNode) *fakeTx {
	f.nodes[n.ID] = n

	return f
}

func (f *fakeTx) GetArchive(_ context.Context, id models.ID) (*models.Archive, error) {
	a, ok := f.archives[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *a

	return &cp, nil
}

func (f *fakeTx) GetArchiveNode(_ context.Context, id models.ID) (*models.ArchiveNode, error) {
	n, ok := f.nodes[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *n

	return &cp, nil
}

func (f *fakeTx) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *c

	return &cp, nil
}

func (f *fakeTx) SaveArchiveNode(_ context.Context, n *models.ArchiveNode) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *n
	f.nodes[n.ID] = &cp
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует ArchiveNodeStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func node(id models.ID, archive models.ID, parent *models.ID) *models.ArchiveNode {
	return &models.ArchiveNode{ID: id, Type: "fond", ArchiveID: archive, ParentID: parent, Label: "узел " + string(id)}
}

func TestUpdateArchiveNodeSaves(t *testing.T) {
	archive := arID('0')
	existing := node(anID('V'), archive, nil)
	st := &fakeStore{tx: newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"}).withNode(existing)}

	updated := *existing
	updated.Label = "новая метка"

	if err := New(st).UpdateArchiveNode(context.Background(), updated); err != nil {
		t.Fatalf("UpdateArchiveNode: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Label != "новая метка" {
		t.Fatalf("InTx=%d, saved=%v; ожидалось одно сохранение с новой меткой", st.calls, st.tx.saved)
	}
}

func TestUpdateArchiveNodeNotFound(t *testing.T) {
	archive := arID('0')
	st := &fakeStore{tx: newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"})}

	err := New(st).UpdateArchiveNode(context.Background(), *node(anID('V'), archive, nil))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d узлов при несуществующем узле", len(st.tx.saved))
	}
}

func TestUpdateArchiveNodeValidatesBeforeTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	bad := *node(anID('V'), arID('0'), nil)
	bad.Label = ""

	err := New(st).UpdateArchiveNode(context.Background(), bad)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "label" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю label", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestUpdateArchiveNodeArchiveNotFound(t *testing.T) {
	archive := arID('0')
	existing := node(anID('V'), archive, nil)
	st := &fakeStore{tx: newFakeTx().withNode(existing)} // архив не заведён

	err := New(st).UpdateArchiveNode(context.Background(), *existing)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "archive_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю archive_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d узлов при несуществующем архиве", len(st.tx.saved))
	}
}

func TestUpdateArchiveNodeParentNotFound(t *testing.T) {
	archive := arID('0')
	existing := node(anID('V'), archive, nil)
	st := &fakeStore{tx: newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"}).withNode(existing)}

	missing := anID('9')
	updated := *existing
	updated.ParentID = &missing

	err := New(st).UpdateArchiveNode(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d узлов при несуществующем родителе", len(st.tx.saved))
	}
}

// TestUpdateArchiveNodeParentFromOtherArchiveRejected: родитель существует,
// но принадлежит другому архиву — *ValidationError по полю parent_id.
func TestUpdateArchiveNodeParentFromOtherArchiveRejected(t *testing.T) {
	archive := arID('0')
	otherArchive := arID('1')
	existing := node(anID('V'), archive, nil)
	otherParent := node(anID('P'), otherArchive, nil)

	tx := newFakeTx().
		withArchive(&models.Archive{ID: archive, Name: "ГАВО"}).
		withArchive(&models.Archive{ID: otherArchive, Name: "РГАДА"}).
		withNode(existing).withNode(otherParent)
	st := &fakeStore{tx: tx}

	updated := *existing
	pid := otherParent.ID
	updated.ParentID = &pid

	err := New(st).UpdateArchiveNode(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id (родитель из другого архива)", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d узлов при родителе из другого архива", len(st.tx.saved))
	}
}

// TestUpdateArchiveNodeDirectCycle: B — дочка A; попытка сделать A дочкой B — цикл.
func TestUpdateArchiveNodeDirectCycle(t *testing.T) {
	archive := arID('0')
	idA, idB := anID('A'), anID('B')
	a := node(idA, archive, nil)
	b := node(idB, archive, &idA)

	st := &fakeStore{tx: newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"}).withNode(a).withNode(b)}

	updated := *a
	updated.ParentID = &idB

	err := New(st).UpdateArchiveNode(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id (цикл)", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d узлов при цикле", len(st.tx.saved))
	}
}

// TestUpdateArchiveNodeLongCycle: цепочка C → B → A; попытка сделать A дочкой C —
// цикл длиной три.
func TestUpdateArchiveNodeLongCycle(t *testing.T) {
	archive := arID('0')
	idA, idB, idC := anID('A'), anID('B'), anID('C')
	a := node(idA, archive, nil)
	b := node(idB, archive, &idA)
	c := node(idC, archive, &idB)

	st := &fakeStore{tx: newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"}).withNode(a).withNode(b).withNode(c)}

	updated := *a
	updated.ParentID = &idC

	err := New(st).UpdateArchiveNode(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id (цикл)", err)
	}
}

// TestUpdateArchiveNodeReparentOK: перенос под другого корректного родителя
// (того же архива) проходит.
func TestUpdateArchiveNodeReparentOK(t *testing.T) {
	archive := arID('0')
	idA, idB, idC := anID('A'), anID('B'), anID('C')
	a := node(idA, archive, nil)
	b := node(idB, archive, &idA)
	c := node(idC, archive, nil)

	st := &fakeStore{tx: newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"}).withNode(a).withNode(b).withNode(c)}

	updated := *b
	updated.ParentID = &idC

	if err := New(st).UpdateArchiveNode(context.Background(), updated); err != nil {
		t.Fatalf("UpdateArchiveNode: %v", err)
	}

	if len(st.tx.saved) != 1 || *st.tx.saved[0].ParentID != idC {
		t.Fatalf("saved=%v; ожидалось сохранение с родителем %v", st.tx.saved, idC)
	}
}

func TestUpdateArchiveNodePropagatesSaveError(t *testing.T) {
	wantErr := errors.New("save failed")
	archive := arID('0')
	existing := node(anID('V'), archive, nil)
	tx := newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"}).withNode(existing)
	tx.saveErr = wantErr
	st := &fakeStore{tx: tx}

	if err := New(st).UpdateArchiveNode(context.Background(), *existing); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

// TestUpdateArchiveNodeSourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestUpdateArchiveNodeSourceCitationNotFound(t *testing.T) {
	archive := arID('0')
	existing := node(anID('V'), archive, nil)
	st := &fakeStore{tx: newFakeTx().withArchive(&models.Archive{ID: archive, Name: "ГАВО"}).withNode(existing)}

	updated := *existing
	updated.Sources = []models.SourceLink{{CitationID: cID('0')}}

	err := New(st).UpdateArchiveNode(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d узлов при несуществующей цитате", len(st.tx.saved))
	}
}
```

#### `internal/usecases/delete_archive_node/deps.go` (создать)
`internal/usecases/delete_archive_node/deps.go`:
```go
package delete_archive_node

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ArchiveNodeRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveNodeRepo interface {
	DeleteArchiveNode(ctx context.Context, id models.ID) error
}
```

#### `internal/usecases/delete_archive_node/scenario.go` (создать)
`internal/usecases/delete_archive_node/scenario.go`:
```go
package delete_archive_node

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление узла архивного дерева».
type Scenario struct {
	archiveNodes ArchiveNodeRepo
}

// New создаёт сценарий.
func New(archiveNodes ArchiveNodeRepo) *Scenario {
	return &Scenario{archiveNodes: archiveNodes}
}

// DeleteArchiveNode удаляет узел. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такого
// узла — models.ErrNotFound; на узел ссылаются другие (дочерние узлы,
// документы) — *models.InUseError со списком ссылающихся (генерическая
// реализация ON DELETE RESTRICT уже в internal/store/sqlstore).
func (s *Scenario) DeleteArchiveNode(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.archiveNodes.DeleteArchiveNode(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeArchiveNode)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeArchiveNode,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

#### `internal/usecases/delete_archive_node/scenario_test.go` (создать)
`internal/usecases/delete_archive_node/scenario_test.go`:
```go
package delete_archive_node

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	deleted []models.ID
	err     error
}

func (f *fakeRepo) DeleteArchiveNode(_ context.Context, id models.ID) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)

	return nil
}

func TestDeleteArchiveNodeCallsRepo(t *testing.T) {
	repo := &fakeRepo{}
	id := models.ID("AN-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	if err := New(repo).DeleteArchiveNode(context.Background(), id); err != nil {
		t.Fatalf("DeleteArchiveNode: %v", err)
	}

	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("deleted = %v", repo.deleted)
	}
}

func TestDeleteArchiveNodeInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	err := New(repo).DeleteArchiveNode(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}

	if len(repo.deleted) != 0 {
		t.Fatalf("repo вызван при невалидном id")
	}
}

func TestDeleteArchiveNodePropagatesInUseError(t *testing.T) {
	wantErr := &models.InUseError{Type: models.TypeArchiveNode, ID: "AN-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	repo := &fakeRepo{err: wantErr}

	err := New(repo).DeleteArchiveNode(context.Background(), "AN-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	var iu *models.InUseError
	if !errors.As(err, &iu) {
		t.Fatalf("err = %v, want InUseError", err)
	}
}
```

#### `internal/httpapi/archive_node.go` (создать)
`internal/httpapi/archive_node.go`:
```go
package httpapi

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// errMissingArchiveID — archive_id обязателен для списка узлов дерева
// (у узла нет смысла вне архива, docs/data-model/entity-write.md §3.4).
var errMissingArchiveID = errors.New("параметр archive_id обязателен")

// handleArchiveNodeList — GET /api/archive-nodes?archive_id=&parent_id=&limit=&offset=.
// archive_id обязателен (у узла нет смысла вне архива) — отсутствующий или
// синтаксически неверный параметр — 400 до вызова сценария; неверное
// значение (несуществующий формат archive_id/parent_id, отрицательное
// окно) — 422 (*models.ValidationError из сценария).
func handleArchiveNodeList(archiveNodes ArchiveNodeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := parseArchiveNodeQuery(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := archiveNodes.ListArchiveNodes(r.Context(), AccessFromContext(r.Context()), q)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ArchiveNodesFromModels(list))
	}
}

// parseArchiveNodeQuery разбирает параметры запроса; archive_id обязателен —
// отсутствующий это 400 (не 422 из сценария, чтобы не тратить обращение к
// сценарию на заведомо неполный запрос).
func parseArchiveNodeQuery(v url.Values) (models.ArchiveNodeQuery, error) {
	raw := v.Get("archive_id")
	if raw == "" {
		return models.ArchiveNodeQuery{}, errMissingArchiveID
	}

	q := models.ArchiveNodeQuery{ArchiveID: models.ID(raw)}

	if pid := v.Get("parent_id"); pid != "" {
		id := models.ID(pid)
		q.ParentID = &id
	}

	var err error

	if q.Page.Limit, err = intParam(v, "limit"); err != nil {
		return q, err
	}

	if q.Page.Offset, err = intParam(v, "offset"); err != nil {
		return q, err
	}

	return q, nil
}

// handleArchiveNodeSearch — GET /api/archive-nodes/search?q=&limit=&offset=.
// Глобальный поиск по всем архивам (не сужен по archive_id — сужение, если
// понадобится, веб-сторона делает сама, docs/data-model/entity-write.md §3.4).
func handleArchiveNodeSearch(archiveNodes ArchiveNodeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := archiveNodes.SearchArchiveNodes(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ArchiveNodesFromModels(list))
	}
}

// handleArchiveNodeGet — GET /api/archive-nodes/{id}.
func handleArchiveNodeGet(archiveNodes ArchiveNodeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n, err := archiveNodes.GetArchiveNode(r.Context(), AccessFromContext(r.Context()), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ArchiveNodeFromModel(n))
	}
}
```

#### `internal/httpapi/archive_node_write.go` (создать)
`internal/httpapi/archive_node_write.go`:
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleArchiveNodeCreate — POST /api/archive-nodes: создаёт узел, отвечает
// 201 с созданным узлом (id генерирует сценарий). Несуществующий archive_id
// или parent_id, либо parent_id из другого архива — 422 (см. writeError).
// Запись — только для вошедшего владельца, см. handleArchiveCreate.
func handleArchiveNodeCreate(archiveNodes ArchiveNodeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.ArchiveNodeCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := archiveNodes.CreateArchiveNode(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.ArchiveNodeFromModel(created))
	}
}

// handleArchiveNodeUpdate — PUT /api/archive-nodes/{id}: полная замена всех
// полей узла (fetch-then-merge: читает текущую версию, накладывает поля
// запроса, сохраняет — тот же приём, что и у Archive, даже при DTO,
// покрывающем сейчас все поля модели, docs/data-model/entity-write.md §3).
func handleArchiveNodeUpdate(archiveNodes ArchiveNodeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.ArchiveNodeUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := archiveNodes.GetArchiveNode(r.Context(), AccessFromContext(r.Context()), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Type = m.Type
		cur.ArchiveID = m.ArchiveID
		cur.ParentID = m.ParentID
		cur.Label = m.Label
		cur.Name = m.Name
		cur.Since = m.Since
		cur.Until = m.Until
		cur.Parish = m.Parish
		cur.Settlements = m.Settlements
		cur.Notes = m.Notes
		cur.Sources = m.Sources
		cur.Private = m.Private

		if err := archiveNodes.UpdateArchiveNode(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ArchiveNodeFromModel(cur))
	}
}

// handleArchiveNodeDelete — DELETE /api/archive-nodes/{id}: 204 без тела;
// занятый узел (дочерние узлы или документы) — 409 со списком ссылающихся.
// Запись — только для вошедшего владельца, см. handleArchiveCreate.
func handleArchiveNodeDelete(archiveNodes ArchiveNodeService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := archiveNodes.DeleteArchiveNode(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
```

#### `internal/httpapi/archive_node_test.go` (создать)
`internal/httpapi/archive_node_test.go`:
```go
package httpapi

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeArchiveNodes struct {
	list []models.ArchiveNode
	err  error

	gotList models.ArchiveNodeQuery

	getN      models.ArchiveNode
	gotIDs    []models.ID
	created   models.ArchiveNode
	gotCreate models.ArchiveNode
	updated   models.ArchiveNode
	deleteErr error

	search    []models.ArchiveNode
	gotSearch models.SearchQuery
}

func (f *fakeArchiveNodes) ListArchiveNodes(_ context.Context, _ models.Access, q models.ArchiveNodeQuery) ([]models.ArchiveNode, error) {
	f.gotList = q

	return f.list, f.err
}

func (f *fakeArchiveNodes) SearchArchiveNodes(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.ArchiveNode, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeArchiveNodes) GetArchiveNode(_ context.Context, _ models.Access, id models.ID) (models.ArchiveNode, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.ArchiveNode{}, f.err
	}

	return f.getN, nil
}

func (f *fakeArchiveNodes) CreateArchiveNode(_ context.Context, n models.ArchiveNode) (models.ArchiveNode, error) {
	f.gotCreate = n
	if f.err != nil {
		return models.ArchiveNode{}, f.err
	}

	return f.created, nil
}

func (f *fakeArchiveNodes) UpdateArchiveNode(_ context.Context, n models.ArchiveNode) error {
	f.updated = n

	return f.err
}

func (f *fakeArchiveNodes) DeleteArchiveNode(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestArchiveNodeListRequiresArchiveID(t *testing.T) {
	svc := &fakeArchiveNodes{}

	rec := get(t, NewHandler(Deps{ArchiveNodes: svc, DocsFS: fstest.MapFS{}}), "/api/archive-nodes")
	requireStatus(t, rec, http.StatusBadRequest)
}

func TestArchiveNodeListPassesArchiveIDAndParentID(t *testing.T) {
	svc := &fakeArchiveNodes{list: []models.ArchiveNode{{ID: "AN-1", ArchiveID: "AR-1", Label: "Фонд 1"}}}

	rec := get(t, NewHandler(Deps{ArchiveNodes: svc, DocsFS: fstest.MapFS{}}), "/api/archive-nodes?archive_id=AR-1&parent_id=AN-0")
	requireStatus(t, rec, http.StatusOK)

	if svc.gotList.ArchiveID != "AR-1" || svc.gotList.ParentID == nil || *svc.gotList.ParentID != "AN-0" {
		t.Fatalf("gotList = %+v", svc.gotList)
	}
}

func TestArchiveNodeGetNotFound(t *testing.T) {
	svc := &fakeArchiveNodes{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{ArchiveNodes: svc, DocsFS: fstest.MapFS{}}), "/api/archive-nodes/AN-1")
	requireStatus(t, rec, http.StatusNotFound)
}

func TestArchiveNodeSearchPassesQuery(t *testing.T) {
	svc := &fakeArchiveNodes{}

	rec := get(t, NewHandler(Deps{ArchiveNodes: svc, DocsFS: fstest.MapFS{}}), "/api/archive-nodes/search?q=фонд")
	requireStatus(t, rec, http.StatusOK)

	if svc.gotSearch.Text != "фонд" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}

func TestArchiveNodeListReturnsRecords(t *testing.T) {
	svc := &fakeArchiveNodes{list: []models.ArchiveNode{{ID: "AN-1", ArchiveID: "AR-1", Type: "fond", Label: "Фонд 1"}}}

	rec := get(t, NewHandler(Deps{ArchiveNodes: svc, DocsFS: fstest.MapFS{}}), "/api/archive-nodes?archive_id=AR-1")
	requireStatus(t, rec, http.StatusOK)

	want := `[{"id":"AN-1","type":"fond","archive_id":"AR-1","label":"Фонд 1","name":"","settlements":[],"notes":[],"sources":[],"private":false}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}
```

#### `internal/httpapi/archive_node_write_test.go` (создать)
`internal/httpapi/archive_node_write_test.go`:
```go
package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

func TestArchiveNodeCreateContract(t *testing.T) {
	svc := &fakeArchiveNodes{created: models.ArchiveNode{ID: "AN-1", ArchiveID: "AR-1", Label: "Фонд 1"}}

	rec := postD(t, NewHandler(Deps{ArchiveNodes: svc, DocsFS: fstest.MapFS{}}), "/api/archive-nodes",
		`{"type":"fond","archive_id":"AR-1","label":"Фонд 1"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.ArchiveID != "AR-1" || svc.gotCreate.Label != "Фонд 1" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestArchiveNodeCreateParentFromOtherArchiveIs422 — usecase-слой возвращает
// *models.ValidationError по полю parent_id при родителе из другого архива
// (см. create_archive_node), writeError мапит это на 422.
func TestArchiveNodeCreateParentFromOtherArchiveIs422(t *testing.T) {
	svc := &fakeArchiveNodes{err: &models.ValidationError{Entity: models.TypeArchiveNode, Field: "parent_id", Reason: "родитель принадлежит другому архиву"}}

	rec := postD(t, NewHandler(Deps{ArchiveNodes: svc, DocsFS: fstest.MapFS{}}), "/api/archive-nodes",
		`{"type":"fond","archive_id":"AR-1","parent_id":"AN-999","label":"Фонд-призрак"}`)

	requireStatus(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"parent_id"`) {
		t.Fatalf("body = %s, ожидалось поле parent_id", rec.Body)
	}
}

func TestArchiveNodeCreateAnonymousIs401(t *testing.T) {
	svc := &fakeArchiveNodes{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{ArchiveNodes: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/archive-nodes", strings.NewReader(`{"type":"fond","archive_id":"AR-1","label":"Фонд"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestArchiveNodeUpdateMergesFields(t *testing.T) {
	svc := &fakeArchiveNodes{getN: models.ArchiveNode{ID: "AN-1", ArchiveID: "AR-1", Label: "Фонд 1"}}

	rec := putD(t, NewHandler(Deps{ArchiveNodes: svc, DocsFS: fstest.MapFS{}}), "/api/archive-nodes/AN-1",
		`{"type":"fond","archive_id":"AR-1","label":"Фонд 1 (испр.)","private":true}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Label != "Фонд 1 (испр.)" || svc.updated.Private != true || svc.updated.ID != "AN-1" {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestArchiveNodeDeleteNoContent(t *testing.T) {
	svc := &fakeArchiveNodes{}

	rec := delD(t, NewHandler(Deps{ArchiveNodes: svc, DocsFS: fstest.MapFS{}}), "/api/archive-nodes/AN-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestArchiveNodeDeleteInUseIs409(t *testing.T) {
	svc := &fakeArchiveNodes{deleteErr: &models.InUseError{Type: models.TypeArchiveNode, ID: "AN-1"}}

	rec := delD(t, NewHandler(Deps{ArchiveNodes: svc, DocsFS: fstest.MapFS{}}), "/api/archive-nodes/AN-1")

	requireStatus(t, rec, http.StatusConflict)
}
```

#### `internal/mcp/archive_node.go` (создать)
`internal/mcp/archive_node.go`:
```go
package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// registerArchiveNodeTools регистрирует тулы для работы с узлами архивного
// дерева. Дерево скопировано по архиву (в отличие от AdministrativeDivision
// — единого глобального дерева): archive_node_list требует archive_id;
// parent_id — необязательный прямой родитель (пусто — корень внутри
// архива). archive_node_create/update дополнительно проверяют, что
// parent_id (если задан) принадлежит тому же архиву, что и сам узел —
// ошибка тула на поле parent_id, если нет.
func registerArchiveNodeTools(s *server.MCPServer, archiveNodes ArchiveNodeService) {
	tool := mcp.NewTool(
		"archive_node_list",
		mcp.WithDescription("Список узлов архивного дерева заданного архива в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithString("archive_id", mcp.Required(), mcp.Description("id архива (обязателен — у узла нет смысла вне архива)")),
		mcp.WithString("parent_id", mcp.Description("id родительского узла; пусто — корень дерева внутри архива")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, archiveNodeListHandler(archiveNodes))

	tool = mcp.NewTool(
		"archive_node_search",
		mcp.WithDescription("Поиск узлов архивного дерева по началу метки/названия, среди всех архивов (без сужения по archive_id); результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало метки или названия")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, archiveNodeSearchHandler(archiveNodes))

	tool = mcp.NewTool(
		"archive_node_get",
		mcp.WithDescription("Узел архивного дерева по id; результат — JSON записи. Неверный формат id или отсутствующая запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например AN-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, archiveNodeGetHandler(archiveNodes))

	tool = mcp.NewTool(
		"archive_node_create",
		mcp.WithDescription("Создать узел архивного дерева; id генерируется сервером; результат — JSON созданной записи. Несуществующий archive_id или parent_id, либо parent_id из другого архива — ошибка тула"),
		mcp.WithString("type", mcp.Required(), mcp.Description("Уровень узла в системе иерархии архива (fond, opis, delo, …) — открытый список")),
		mcp.WithString("archive_id", mcp.Required(), mcp.Description("id архива")),
		mcp.WithString("parent_id", mcp.Description("id родительского узла (должен принадлежать тому же архиву); пусто — корень")),
		mcp.WithString("label", mcp.Required(), mcp.Description("Шифр/метка узла")),
		mcp.WithString("name", mcp.Description("Название")),
		mcp.WithObject("since", mcp.Description("Начало периода"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("until", mcp.Description("Конец периода"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("parish", mcp.Description("Приход (текст или ссылка {text, ref?, type?})"), mcp.Properties(textRefObjectProperties())),
		mcp.WithArray("settlements", mcp.WithStringItems(), mcp.Description("Населённые пункты")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, archiveNodeCreateHandler(archiveNodes))

	tool = mcp.NewTool(
		"archive_node_update",
		mcp.WithDescription("Изменить узел архивного дерева: полная замена type/archive_id/parent_id/label/name/since/until/parish/settlements/notes/private; результат — JSON обновлённой записи. Несуществующий archive_id/parent_id, parent_id из другого архива или цикл по parent_id — ошибка тула; sources — при отсутствии в вызове текущие источники сохраняются, пустой массив — очищает их"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("type", mcp.Required(), mcp.Description("Уровень узла")),
		mcp.WithString("archive_id", mcp.Required(), mcp.Description("id архива")),
		mcp.WithString("parent_id", mcp.Description("id родительского узла; пусто — корень")),
		mcp.WithString("label", mcp.Required(), mcp.Description("Шифр/метка узла")),
		mcp.WithString("name", mcp.Description("Название")),
		mcp.WithObject("since", mcp.Description("Начало периода"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("until", mcp.Description("Конец периода"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("parish", mcp.Description("Приход (текст или ссылка {text, ref?, type?})"), mcp.Properties(textRefObjectProperties())),
		mcp.WithArray("settlements", mcp.WithStringItems(), mcp.Description("Населённые пункты")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, archiveNodeUpdateHandler(archiveNodes))

	tool = mcp.NewTool(
		"archive_node_delete",
		mcp.WithDescription("Удалить узел архивного дерева. Необратимо. Если на него ссылаются дочерние узлы или документы — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, archiveNodeDeleteHandler(archiveNodes))
}

func archiveNodeListHandler(archiveNodes ArchiveNodeService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.ArchiveNodeQuery{ArchiveID: models.ID(req.GetString("archive_id", ""))}

		if raw := req.GetString("parent_id", ""); raw != "" {
			id := models.ID(raw)
			q.ParentID = &id
		}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := archiveNodes.ListArchiveNodes(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.ArchiveNodesFromModels(list))
	}
}

func archiveNodeSearchHandler(archiveNodes ArchiveNodeService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := archiveNodes.SearchArchiveNodes(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.ArchiveNodesFromModels(list))
	}
}

func archiveNodeGetHandler(archiveNodes ArchiveNodeService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		n, err := archiveNodes.GetArchiveNode(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.ArchiveNodeFromModel(n))
	}
}

func archiveNodeCreateHandler(archiveNodes ArchiveNodeService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		parish, err := optionalTextRef(args, "parish")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		since, err := optionalFactDate(args, "since")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		until, err := optionalFactDate(args, "until")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sources, err := optionalSourceLinks(args, "sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		n := models.ArchiveNode{
			Type:        models.ArchiveNodeType(req.GetString("type", "")),
			ArchiveID:   models.ID(req.GetString("archive_id", "")),
			ParentID:    optionalArchiveNodeParentID(req),
			Label:       req.GetString("label", ""),
			Name:        req.GetString("name", ""),
			Since:       since,
			Until:       until,
			Parish:      parish,
			Settlements: textRefsFromStrings(req.GetStringSlice("settlements", nil)),
			Notes:       textRefsFromStrings(req.GetStringSlice("notes", nil)),
			Sources:     sources,
			Private:     req.GetBool("private", false),
		}

		created, err := archiveNodes.CreateArchiveNode(ctx, n)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.ArchiveNodeFromModel(created))
	}
}

func archiveNodeUpdateHandler(archiveNodes ArchiveNodeService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := archiveNodes.GetArchiveNode(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		args := req.GetArguments()

		parish, err := optionalTextRef(args, "parish")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		since, err := optionalFactDate(args, "since")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		until, err := optionalFactDate(args, "until")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cur.Type = models.ArchiveNodeType(req.GetString("type", ""))
		cur.ArchiveID = models.ID(req.GetString("archive_id", ""))
		cur.ParentID = optionalArchiveNodeParentID(req)
		cur.Label = req.GetString("label", "")
		cur.Name = req.GetString("name", "")
		cur.Since = since
		cur.Until = until
		cur.Parish = parish
		cur.Settlements = textRefsFromStrings(req.GetStringSlice("settlements", nil))
		cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))
		cur.Private = req.GetBool("private", false)

		if raw, ok := args["sources"]; ok && raw != nil {
			sources, err := optionalSourceLinks(args, "sources")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Sources = sources
		}

		if err := archiveNodes.UpdateArchiveNode(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.ArchiveNodeFromModel(cur))
	}
}

func archiveNodeDeleteHandler(archiveNodes ArchiveNodeService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := archiveNodes.DeleteArchiveNode(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}

// optionalArchiveNodeParentID читает необязательный parent_id; пустая строка
// (в т.ч. явный null) — корень (nil-указатель). По образцу
// optionalParentID у division.go.
func optionalArchiveNodeParentID(req mcp.CallToolRequest) *models.ID {
	raw := req.GetString("parent_id", "")
	if raw == "" {
		return nil
	}

	id := models.ID(raw)

	return &id
}
```

#### `internal/mcp/archive_node_test.go` (создать)
`internal/mcp/archive_node_test.go`:
```go
package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakeArchiveNodes struct {
	list []models.ArchiveNode
	err  error

	gotList models.ArchiveNodeQuery

	getN      models.ArchiveNode
	created   models.ArchiveNode
	gotCreate models.ArchiveNode
	updated   models.ArchiveNode
	gotIDs    []models.ID
	deleteErr error

	search []models.ArchiveNode
}

func (f *fakeArchiveNodes) ListArchiveNodes(_ context.Context, _ models.Access, q models.ArchiveNodeQuery) ([]models.ArchiveNode, error) {
	f.gotList = q

	return f.list, f.err
}

func (f *fakeArchiveNodes) SearchArchiveNodes(context.Context, models.Access, models.SearchQuery) ([]models.ArchiveNode, error) {
	return f.search, f.err
}

func (f *fakeArchiveNodes) GetArchiveNode(_ context.Context, _ models.Access, id models.ID) (models.ArchiveNode, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.ArchiveNode{}, f.err
	}

	return f.getN, nil
}

func (f *fakeArchiveNodes) CreateArchiveNode(_ context.Context, n models.ArchiveNode) (models.ArchiveNode, error) {
	f.gotCreate = n
	if f.err != nil {
		return models.ArchiveNode{}, f.err
	}

	return f.created, nil
}

func (f *fakeArchiveNodes) UpdateArchiveNode(_ context.Context, n models.ArchiveNode) error {
	f.updated = n

	return f.err
}

func (f *fakeArchiveNodes) DeleteArchiveNode(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callArchiveNodeTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestArchiveNodeListToolRequiresArchiveID(t *testing.T) {
	svc := &fakeArchiveNodes{err: &models.ValidationError{Entity: models.TypeArchiveNode, Field: "archive_id", Reason: "обязателен"}}

	res := callArchiveNodeTool(t, archiveNodeListHandler(svc), map[string]any{})

	if !res.IsError {
		t.Fatalf("isError=%v, want true (archive_id пуст)", res.IsError)
	}
}

func TestArchiveNodeListToolPassesArchiveIDAndParentID(t *testing.T) {
	svc := &fakeArchiveNodes{}

	res := callArchiveNodeTool(t, archiveNodeListHandler(svc), map[string]any{
		"archive_id": "AR-1",
		"parent_id":  "AN-0",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotList.ArchiveID != "AR-1" || svc.gotList.ParentID == nil || *svc.gotList.ParentID != "AN-0" {
		t.Fatalf("gotList = %+v", svc.gotList)
	}
}

func TestArchiveNodeGetToolContract(t *testing.T) {
	svc := &fakeArchiveNodes{getN: models.ArchiveNode{ID: "AN-1", Label: "Фонд 1"}}

	res := callArchiveNodeTool(t, archiveNodeGetHandler(svc), map[string]any{"id": "AN-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "AN-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

func TestArchiveNodeCreateToolPassesFields(t *testing.T) {
	svc := &fakeArchiveNodes{created: models.ArchiveNode{ID: "AN-new", Label: "Фонд 1"}}

	res := callArchiveNodeTool(t, archiveNodeCreateHandler(svc), map[string]any{
		"type":       "fond",
		"archive_id": "AR-1",
		"label":      "Фонд 1",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.ArchiveID != "AR-1" || svc.gotCreate.Label != "Фонд 1" || string(svc.gotCreate.Type) != "fond" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestArchiveNodeCreateToolParentFromOtherArchiveIsError — usecase-слой
// возвращает ошибку на parent_id из другого архива, тул отдаёт её как
// ошибку тула.
func TestArchiveNodeCreateToolParentFromOtherArchiveIsError(t *testing.T) {
	svc := &fakeArchiveNodes{err: &models.ValidationError{Entity: models.TypeArchiveNode, Field: "parent_id", Reason: "другой архив"}}

	res := callArchiveNodeTool(t, archiveNodeCreateHandler(svc), map[string]any{
		"type":       "fond",
		"archive_id": "AR-1",
		"parent_id":  "AN-999",
		"label":      "Фонд-призрак",
	})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

// TestArchiveNodeUpdateToolPreservesSourcesWhenOmitted — sources отсутствует
// в вызове: текущие источники (из GetArchiveNode) сохраняются как есть, не
// затираются (docs/data-model/entity-write.md §3.3, урок подпроекта 5).
func TestArchiveNodeUpdateToolPreservesSourcesWhenOmitted(t *testing.T) {
	existingSources := []models.SourceLink{{CitationID: "C-1"}}
	svc := &fakeArchiveNodes{getN: models.ArchiveNode{ID: "AN-1", ArchiveID: "AR-1", Label: "Фонд 1", Sources: existingSources}}

	res := callArchiveNodeTool(t, archiveNodeUpdateHandler(svc), map[string]any{
		"id":         "AN-1",
		"type":       "fond",
		"archive_id": "AR-1",
		"label":      "Фонд 1 (испр.)",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if len(svc.updated.Sources) != 1 || svc.updated.Sources[0].CitationID != "C-1" {
		t.Fatalf("updated.Sources = %+v, ожидались сохранённые источники", svc.updated.Sources)
	}
}

// TestArchiveNodeUpdateToolClearsSourcesWhenEmptyArray — пустой массив
// sources в вызове очищает источники (в отличие от отсутствия ключа).
func TestArchiveNodeUpdateToolClearsSourcesWhenEmptyArray(t *testing.T) {
	existingSources := []models.SourceLink{{CitationID: "C-1"}}
	svc := &fakeArchiveNodes{getN: models.ArchiveNode{ID: "AN-1", ArchiveID: "AR-1", Label: "Фонд 1", Sources: existingSources}}

	res := callArchiveNodeTool(t, archiveNodeUpdateHandler(svc), map[string]any{
		"id":         "AN-1",
		"type":       "fond",
		"archive_id": "AR-1",
		"label":      "Фонд 1 (испр.)",
		"sources":    []any{},
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if len(svc.updated.Sources) != 0 {
		t.Fatalf("updated.Sources = %+v, ожидался пустой список", svc.updated.Sources)
	}
}

func TestArchiveNodeDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeArchiveNodes{deleteErr: &models.InUseError{Type: models.TypeArchiveNode, ID: "AN-1"}}

	res := callArchiveNodeTool(t, archiveNodeDeleteHandler(svc), map[string]any{"id": "AN-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersArchiveNodeTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, ArchiveNodes: &fakeArchiveNodes{}}).ListTools()

	for _, name := range []string{
		"archive_node_list", "archive_node_search", "archive_node_get",
		"archive_node_create", "archive_node_update", "archive_node_delete",
	} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
```

### 1.3. ArchiveDocument — транспорт, usecases, httpapi, MCP

#### `internal/transport/archive_document.go` (создать)
`internal/transport/archive_document.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// ArchiveDocument — контракт документа внутри единицы учёта (GET
// /api/archive-documents, MCP-тул archive_document_list). UnitID —
// обязательная строгая ссылка на ArchiveNode (единицу учёта, к которой
// относится документ).
type ArchiveDocument struct {
	ID          models.ID    `json:"id"`
	UnitID      models.ID    `json:"unit_id"`
	Title       string       `json:"title"`
	Kind        string       `json:"kind"`
	Since       *FactDate    `json:"since,omitempty"`
	Until       *FactDate    `json:"until,omitempty"`
	Parish      *TextRef     `json:"parish,omitempty"`
	Settlements []TextRef    `json:"settlements"`
	Notes       []TextRef    `json:"notes"`
	Sources     []SourceLink `json:"sources"`
	Private     bool         `json:"private"`
}

// ArchiveDocumentFromModel конвертирует запись в контракт.
func ArchiveDocumentFromModel(d models.ArchiveDocument) ArchiveDocument {
	return ArchiveDocument{
		ID:          d.ID,
		UnitID:      d.UnitID,
		Title:       d.Title,
		Kind:        d.Kind,
		Since:       FactDateFromModel(d.Since),
		Until:       FactDateFromModel(d.Until),
		Parish:      TextRefFromModelPtr(d.Parish),
		Settlements: TextRefsFromModel(d.Settlements),
		Notes:       TextRefsFromModel(d.Notes),
		Sources:     SourceLinksFromModel(d.Sources),
		Private:     d.Private,
	}
}

// ArchiveDocumentsFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func ArchiveDocumentsFromModels(ds []models.ArchiveDocument) []ArchiveDocument {
	out := make([]ArchiveDocument, 0, len(ds))
	for _, d := range ds {
		out = append(out, ArchiveDocumentFromModel(d))
	}

	return out
}
```

#### `internal/transport/archive_document_write.go` (создать)
`internal/transport/archive_document_write.go`:
```go
package transport

import "github.com/amarin/genodex/internal/models"

// ArchiveDocumentCreate — тело POST /api/archive-documents и аргументы тула
// archive_document_create. Идентификатор генерирует сценарий.
type ArchiveDocumentCreate struct {
	UnitID      models.ID    `json:"unit_id"`
	Title       string       `json:"title"`
	Kind        string       `json:"kind"`
	Since       *FactDate    `json:"since,omitempty"`
	Until       *FactDate    `json:"until,omitempty"`
	Parish      *TextRef     `json:"parish,omitempty"`
	Settlements []TextRef    `json:"settlements"`
	Notes       []TextRef    `json:"notes"`
	Sources     []SourceLink `json:"sources"`
	Private     bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (d ArchiveDocumentCreate) Model() models.ArchiveDocument {
	return models.ArchiveDocument{
		UnitID:      d.UnitID,
		Title:       d.Title,
		Kind:        d.Kind,
		Since:       d.Since.Model(),
		Until:       d.Until.Model(),
		Parish:      d.Parish.ModelPtr(),
		Settlements: TextRefsToModel(d.Settlements),
		Notes:       TextRefsToModel(d.Notes),
		Sources:     SourceLinksToModel(d.Sources),
		Private:     d.Private,
	}
}

// ArchiveDocumentUpdate — тело PUT /api/archive-documents/{id} и аргументы
// тула archive_document_update: полная замена всех полей ниже id.
type ArchiveDocumentUpdate struct {
	UnitID      models.ID    `json:"unit_id"`
	Title       string       `json:"title"`
	Kind        string       `json:"kind"`
	Since       *FactDate    `json:"since,omitempty"`
	Until       *FactDate    `json:"until,omitempty"`
	Parish      *TextRef     `json:"parish,omitempty"`
	Settlements []TextRef    `json:"settlements"`
	Notes       []TextRef    `json:"notes"`
	Sources     []SourceLink `json:"sources"`
	Private     bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (d ArchiveDocumentUpdate) Model() models.ArchiveDocument {
	return models.ArchiveDocument{
		UnitID:      d.UnitID,
		Title:       d.Title,
		Kind:        d.Kind,
		Since:       d.Since.Model(),
		Until:       d.Until.Model(),
		Parish:      d.Parish.ModelPtr(),
		Settlements: TextRefsToModel(d.Settlements),
		Notes:       TextRefsToModel(d.Notes),
		Sources:     SourceLinksToModel(d.Sources),
		Private:     d.Private,
	}
}
```

#### `internal/usecases/list_archive_documents/deps.go` (создать)
`internal/usecases/list_archive_documents/deps.go`:
```go
package list_archive_documents

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ArchiveDocumentRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveDocumentRepo interface {
	ListArchiveDocuments(ctx context.Context, access models.Access, page models.Page) ([]*models.ArchiveDocument, error)
}
```

#### `internal/usecases/list_archive_documents/scenario.go` (создать)
`internal/usecases/list_archive_documents/scenario.go`:
```go
package list_archive_documents

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список документов внутри единиц учёта».
type Scenario struct {
	archiveDocuments ArchiveDocumentRepo
}

// New создаёт сценарий.
func New(archiveDocuments ArchiveDocumentRepo) *Scenario {
	return &Scenario{archiveDocuments: archiveDocuments}
}

// ListArchiveDocuments возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка. Плоская
// сущность (в отличие от ArchiveNode) — без дополнительного фильтра.
func (s *Scenario) ListArchiveDocuments(ctx context.Context, access models.Access, page models.Page) ([]models.ArchiveDocument, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeArchiveDocument, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeArchiveDocument, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.archiveDocuments.ListArchiveDocuments(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.ArchiveDocument, 0, len(list))
	for _, d := range list {
		out = append(out, *d)
	}

	return out, nil
}
```

#### `internal/usecases/list_archive_documents/scenario_test.go` (создать)
`internal/usecases/list_archive_documents/scenario_test.go`:
```go
package list_archive_documents

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.ArchiveDocument
}

func (f *fakeRepo) ListArchiveDocuments(_ context.Context, _ models.Access, page models.Page) ([]*models.ArchiveDocument, error) {
	f.page = page

	return f.out, nil
}

func TestListArchiveDocumentsReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.ArchiveDocument{{ID: "DC-01ARZ3NDEKTSV4RRFFQ69G5FA1", Title: "Метрическая книга 1890"}}}

	got, err := New(repo).ListArchiveDocuments(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListArchiveDocuments: %v", err)
	}

	if len(got) != 1 || got[0].Title != "Метрическая книга 1890" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListArchiveDocumentsRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListArchiveDocuments(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}

func TestListArchiveDocumentsRejectsNegativeOffset(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListArchiveDocuments(context.Background(), models.AccessFull, models.Page{Offset: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "offset" {
		t.Fatalf("err = %v, want ValidationError on offset", err)
	}
}
```

#### `internal/usecases/search_archive_documents/deps.go` (создать)
`internal/usecases/search_archive_documents/deps.go`:
```go
package search_archive_documents

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ArchiveDocumentRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveDocumentRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetArchiveDocument(ctx context.Context, id models.ID) (*models.ArchiveDocument, error)
}
```

#### `internal/usecases/search_archive_documents/scenario.go` (создать)
`internal/usecases/search_archive_documents/scenario.go`:
```go
package search_archive_documents

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск документов внутри единиц учёта».
type Scenario struct {
	archiveDocuments ArchiveDocumentRepo
}

// New создаёт сценарий.
func New(archiveDocuments ArchiveDocumentRepo) *Scenario {
	return &Scenario{archiveDocuments: archiveDocuments}
}

// SearchArchiveDocuments находит документы, чей title (индексируется под
// полем "title", см. internal/store/sqlstore/archive.go) начинается с
// текста запроса, и возвращает их целиком. Результат — в том порядке, в
// каком их отдаёт Search. Окно (размер и сдвиг) применяется после отбора
// хитов own-типа. Пустой текст (после обрезки) — пустой результат без
// обращения к репозиторию. Хит, чей документ удалён между поиском и
// чтением (ErrNotFound), пропускается; прочие ошибки пробрасываются.
func (s *Scenario) SearchArchiveDocuments(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.ArchiveDocument, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.ArchiveDocument{}, nil
	}

	page := q.Page.Normalized()
	out := []models.ArchiveDocument{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.archiveDocuments.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeArchiveDocument {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.archiveDocuments.GetArchiveDocument(ctx, h.ID)
			if err != nil {
				if errors.Is(err, models.ErrNotFound) {
					matched++

					continue
				}

				return nil, err
			}

			out = append(out, *got)

			if len(out) == page.Limit {
				return out, nil
			}

			matched++
		}
	}
}
```

#### `internal/usecases/search_archive_documents/scenario_test.go` (создать)
`internal/usecases/search_archive_documents/scenario_test.go`:
```go
package search_archive_documents

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

func hit(id string, typ models.Type) models.Hit {
	return models.Hit{Type: typ, ID: models.ID(id), Label: id}
}

func archiveDocument(id string) *models.ArchiveDocument {
	return &models.ArchiveDocument{ID: models.ID(id), Title: id}
}

func ids(docs []models.ArchiveDocument) []models.ID {
	out := make([]models.ID, 0, len(docs))
	for _, d := range docs {
		out = append(out, d.ID)
	}

	return out
}

func sameIDs[T ~string](got []T, want ...T) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}

	return true
}

type fakeRepo struct {
	hits     []models.Hit
	getErr   map[models.ID]error
	err      error
	errAt    int
	gotQuery string
	calls    []models.Page
	getCalls []models.ID
	accesses []models.Access
}

func (f *fakeRepo) Search(_ context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error) {
	f.gotQuery = query
	f.calls = append(f.calls, page)
	f.accesses = append(f.accesses, access)

	if f.err != nil && (f.errAt == 0 || f.errAt == len(f.calls)) {
		return nil, f.err
	}

	page = page.Normalized()
	if page.Offset >= len(f.hits) {
		return nil, nil
	}

	end := min(page.Offset+page.Limit, len(f.hits))

	return f.hits[page.Offset:end], nil
}

func (f *fakeRepo) GetArchiveDocument(_ context.Context, id models.ID) (*models.ArchiveDocument, error) {
	f.getCalls = append(f.getCalls, id)

	if f.getErr != nil {
		if e, ok := f.getErr[id]; ok && e != nil {
			return nil, e
		}
	}

	return archiveDocument(string(id)), nil
}

func TestSearchArchiveDocumentsEmptyTextDoesNotTouchRepo(t *testing.T) {
	repo := &fakeRepo{}

	got, err := New(repo).SearchArchiveDocuments(context.Background(), models.AccessFull, models.SearchQuery{Text: "   "})
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v; ожидался пустой результат", got, err)
	}
	if len(repo.calls) != 0 {
		t.Fatalf("Search вызван %d раз при пустом тексте", len(repo.calls))
	}
}

func TestSearchArchiveDocumentsReturnsFullDocumentsFromHits(t *testing.T) {
	repo := &fakeRepo{
		hits: []models.Hit{
			hit("D-1001", models.TypePerson),
			hit("DC-1002", models.TypeArchiveDocument),
			hit("F-1003", models.TypeFamily),
			hit("DC-1004", models.TypeArchiveDocument),
		},
	}

	got, err := New(repo).SearchArchiveDocuments(context.Background(), models.AccessFull, models.SearchQuery{Text: "метрич"})
	if err != nil || !sameIDs(ids(got), "DC-1002", "DC-1004") {
		t.Fatalf("got %v, %v; ожидались DC-1002, DC-1004", ids(got), err)
	}
	if repo.gotQuery != "метрич" {
		t.Fatalf("Search получил %q, ожидался %q", repo.gotQuery, "метрич")
	}
}

func TestSearchArchiveDocumentsAppliesWindowAmongHits(t *testing.T) {
	repo := &fakeRepo{
		hits: []models.Hit{
			hit("D-1001", models.TypePerson),
			hit("DC-1002", models.TypeArchiveDocument),
			hit("DC-1003", models.TypeArchiveDocument),
			hit("DC-1004", models.TypeArchiveDocument),
		},
	}

	got, err := New(repo).SearchArchiveDocuments(context.Background(), models.AccessFull, models.SearchQuery{
		Text: "м",
		Page: models.Page{Limit: 1, Offset: 1},
	})
	if err != nil || !sameIDs(ids(got), "DC-1003") {
		t.Fatalf("got %v, %v; ожидалась DC-1003 (второй хит)", ids(got), err)
	}
}

func TestSearchArchiveDocumentsSkipsDeletedDocument(t *testing.T) {
	repo := &fakeRepo{
		hits: []models.Hit{
			hit("DC-1002", models.TypeArchiveDocument),
			hit("DC-1003", models.TypeArchiveDocument),
		},
		getErr: map[models.ID]error{"DC-1002": models.ErrNotFound},
	}

	got, err := New(repo).SearchArchiveDocuments(context.Background(), models.AccessFull, models.SearchQuery{Text: "м"})
	if err != nil {
		t.Fatalf("SearchArchiveDocuments: %v", err)
	}
	if !sameIDs(ids(got), "DC-1003") {
		t.Fatalf("got %v; DC-1002 удалён — остаётся DC-1003", ids(got))
	}
}

func TestSearchArchiveDocumentsPropagatesGetError(t *testing.T) {
	boom := errors.New("get boom")
	repo := &fakeRepo{
		hits:   []models.Hit{hit("DC-1002", models.TypeArchiveDocument)},
		getErr: map[models.ID]error{"DC-1002": boom},
	}

	_, err := New(repo).SearchArchiveDocuments(context.Background(), models.AccessFull, models.SearchQuery{Text: "м"})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, ожидался boom", err)
	}
}

func TestSearchArchiveDocumentsPropagatesSearchError(t *testing.T) {
	boom := errors.New("search boom")
	repo := &fakeRepo{err: boom}

	_, err := New(repo).SearchArchiveDocuments(context.Background(), models.AccessFull, models.SearchQuery{Text: "м"})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, ожидался boom", err)
	}
}

func TestSearchArchiveDocumentsInvalidQueryDoesNotTouchRepo(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).SearchArchiveDocuments(context.Background(), models.AccessFull, models.SearchQuery{Page: models.Page{Limit: -1}})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, ожидалась *ValidationError поля limit", err)
	}
	if len(repo.calls) != 0 {
		t.Fatalf("Search вызван %d раз при некорректном запросе", len(repo.calls))
	}
}

func TestSearchArchiveDocumentsPassesAccessToRepo(t *testing.T) {
	repo := &fakeRepo{hits: []models.Hit{
		hit("DC-1002", models.TypeArchiveDocument),
	}}

	if _, err := New(repo).SearchArchiveDocuments(context.Background(), models.AccessPublic,
		models.SearchQuery{Text: "м"}); err != nil {
		t.Fatalf("SearchArchiveDocuments: %v", err)
	}

	for i, a := range repo.accesses {
		if a != models.AccessPublic {
			t.Errorf("вызов %d: доступ %v, ожидался AccessPublic", i+1, a)
		}
	}
}
```

#### `internal/usecases/get_archive_document/deps.go` (создать)
`internal/usecases/get_archive_document/deps.go`:
```go
package get_archive_document

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ArchiveDocumentRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveDocumentRepo interface {
	GetArchiveDocument(ctx context.Context, id models.ID) (*models.ArchiveDocument, error)
}
```

#### `internal/usecases/get_archive_document/scenario.go` (создать)
`internal/usecases/get_archive_document/scenario.go`:
```go
package get_archive_document

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «документ внутри единицы учёта по идентификатору».
type Scenario struct {
	archiveDocuments ArchiveDocumentRepo
}

// New создаёт сценарий.
func New(archiveDocuments ArchiveDocumentRepo) *Scenario {
	return &Scenario{archiveDocuments: archiveDocuments}
}

// GetArchiveDocument возвращает документ по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такого документа — models.ErrNotFound. Приватный документ
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound (docs/data-model/entity-write.md §3.1).
func (s *Scenario) GetArchiveDocument(ctx context.Context, access models.Access, id models.ID) (models.ArchiveDocument, error) {
	if err := validateID(id); err != nil {
		return models.ArchiveDocument{}, err
	}

	d, err := s.archiveDocuments.GetArchiveDocument(ctx, id)
	if err != nil {
		return models.ArchiveDocument{}, err
	}

	if d.Private && access != models.AccessFull {
		return models.ArchiveDocument{}, models.ErrNotFound
	}

	return *d, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeArchiveDocument)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeArchiveDocument,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

#### `internal/usecases/get_archive_document/scenario_test.go` (создать)
`internal/usecases/get_archive_document/scenario_test.go`:
```go
package get_archive_document

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	docs map[models.ID]*models.ArchiveDocument
}

func (f *fakeRepo) GetArchiveDocument(_ context.Context, id models.ID) (*models.ArchiveDocument, error) {
	d, ok := f.docs[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return d, nil
}

func TestGetArchiveDocumentReturnsRecord(t *testing.T) {
	id := models.ID("DC-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{docs: map[models.ID]*models.ArchiveDocument{id: {ID: id, Title: "Метрическая книга"}}}

	got, err := New(repo).GetArchiveDocument(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetArchiveDocument: %v", err)
	}

	if got.Title != "Метрическая книга" {
		t.Fatalf("Title = %q", got.Title)
	}
}

func TestGetArchiveDocumentNotFound(t *testing.T) {
	repo := &fakeRepo{docs: map[models.ID]*models.ArchiveDocument{}}

	_, err := New(repo).GetArchiveDocument(context.Background(), models.AccessFull, "DC-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetArchiveDocumentInvalidID(t *testing.T) {
	repo := &fakeRepo{docs: map[models.ID]*models.ArchiveDocument{}}

	_, err := New(repo).GetArchiveDocument(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestGetArchiveDocumentPrivateHiddenFromPublic(t *testing.T) {
	id := models.ID("DC-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{docs: map[models.ID]*models.ArchiveDocument{id: {ID: id, Title: "Метрическая книга", Private: true}}}

	_, err := New(repo).GetArchiveDocument(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for private record with AccessPublic", err)
	}
}

func TestGetArchiveDocumentPrivateVisibleToFullAccess(t *testing.T) {
	id := models.ID("DC-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{docs: map[models.ID]*models.ArchiveDocument{id: {ID: id, Title: "Метрическая книга", Private: true}}}

	got, err := New(repo).GetArchiveDocument(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetArchiveDocument: %v", err)
	}

	if got.Title != "Метрическая книга" {
		t.Fatalf("Title = %q", got.Title)
	}
}
```

#### `internal/usecases/create_archive_document/deps.go` (создать)
`internal/usecases/create_archive_document/deps.go`:
```go
package create_archive_document

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// ArchiveDocumentStore — зависимость сценария: транзакция порта store.Store.
// Проверка единицы учёта (unit_id) и сохранение идут в одной транзакции на
// переданном fn хранилище.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveDocumentStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
```

#### `internal/usecases/create_archive_document/scenario.go` (создать)
`internal/usecases/create_archive_document/scenario.go`:
```go
package create_archive_document

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание документа внутри единицы учёта».
type Scenario struct {
	store ArchiveDocumentStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st ArchiveDocumentStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateArchiveDocument создаёт документ: генерирует идентификатор,
// проверяет инварианты, в одной транзакции убеждается в существовании
// единицы учёта (UnitID — обязательная ссылка на ArchiveNode) и сохраняет.
// Возвращает созданный документ с заполненным ID.
//
// Ошибки: непустой входной ID, невалидная сущность, несуществующая единица
// учёта — *models.ValidationError (поля id, unit_id); прочее — ошибки
// хранилища как есть.
func (s *Scenario) CreateArchiveDocument(ctx context.Context, d models.ArchiveDocument) (models.ArchiveDocument, error) {
	if d.ID != "" {
		return models.ArchiveDocument{}, &models.ValidationError{
			Entity: models.TypeArchiveDocument,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", d.ID),
		}
	}

	d.ID = s.ids.New(models.TypeArchiveDocument)

	if err := d.Validate(); err != nil {
		return models.ArchiveDocument{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetArchiveNode(ctx, d.UnitID); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return unitErr("единица учёта %q не найдена", d.UnitID)
			}

			return err
		}

		for i, link := range d.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveArchiveDocument(ctx, &d)
	})
	if err != nil {
		return models.ArchiveDocument{}, err
	}

	return d, nil
}

// unitErr — *models.ValidationError по полю unit_id.
func unitErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeArchiveDocument,
		Field:  "unit_id",
		Reason: fmt.Sprintf(format, args...),
	}
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeArchiveDocument,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/create_archive_document/scenario_test.go` (создать)
`internal/usecases/create_archive_document/scenario_test.go`:
```go
package create_archive_document

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// dcID возвращает корректный идентификатор документа.
func dcID(last byte) models.ID {
	return models.ID("DC-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// anID возвращает корректный идентификатор узла архивного дерева.
func anID(last byte) models.ID {
	return models.ID("AN-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// cID возвращает корректный идентификатор цитаты.
func cID(last byte) models.ID {
	return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

type stubIDs struct {
	id      models.ID
	gotType models.Type
}

func (s *stubIDs) New(t models.Type) models.ID {
	s.gotType = t

	return s.id
}

// fakeTx реализует нужные сценарию методы store.Store поверх карт узлов и
// цитат; остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	nodes     map[models.ID]*models.ArchiveNode
	citations map[models.ID]*models.Citation
	saved     []*models.ArchiveDocument
	saveErr   error
}

func newFakeTx(existingNodes ...*models.ArchiveNode) *fakeTx {
	tx := &fakeTx{nodes: map[models.ID]*models.ArchiveNode{}, citations: map[models.ID]*models.Citation{}}
	for _, n := range existingNodes {
		tx.nodes[n.ID] = n
	}

	return tx
}

func (f *fakeTx) withCitation(c *models.Citation) *fakeTx {
	f.citations[c.ID] = c

	return f
}

func (f *fakeTx) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *c

	return &cp, nil
}

func (f *fakeTx) GetArchiveNode(_ context.Context, id models.ID) (*models.ArchiveNode, error) {
	n, ok := f.nodes[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *n

	return &cp, nil
}

func (f *fakeTx) SaveArchiveDocument(_ context.Context, d *models.ArchiveDocument) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *d
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует ArchiveDocumentStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx      *fakeTx
	inTxErr error
	calls   int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	if f.inTxErr != nil {
		return f.inTxErr
	}

	return fn(f.tx)
}

func validInput(unit models.ID) models.ArchiveDocument {
	return models.ArchiveDocument{UnitID: unit, Title: "Метрическая книга 1890"}
}

func TestCreateArchiveDocumentGeneratesIDAndSaves(t *testing.T) {
	unit := anID('0')
	st := &fakeStore{tx: newFakeTx(&models.ArchiveNode{ID: unit, Type: "fond", ArchiveID: "AR-01ARZ3NDEKTSV4RRFFQ69G5FA0", Label: "Фонд"})}
	ids := &stubIDs{id: dcID('V')}

	got, err := New(st, ids).CreateArchiveDocument(context.Background(), validInput(unit))
	if err != nil {
		t.Fatalf("CreateArchiveDocument: %v", err)
	}

	if got.ID != dcID('V') || got.Title != "Метрическая книга 1890" {
		t.Fatalf("got %+v, ожидался документ с ID %v", got, dcID('V'))
	}

	if ids.gotType != models.TypeArchiveDocument {
		t.Errorf("генератор вызван с типом %q, ожидался archive_document", ids.gotType)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].ID != dcID('V') {
		t.Fatalf("InTx=%d, saved=%v; ожидалось одно сохранение внутри одной транзакции", st.calls, st.tx.saved)
	}
}

func TestCreateArchiveDocumentRejectsExplicitID(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	ids := &stubIDs{id: dcID('V')}

	in := validInput(anID('0'))
	in.ID = dcID('0')

	_, err := New(st, ids).CreateArchiveDocument(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю id", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx=%d; ожидалось: не вызван", st.calls)
	}
}

func TestCreateArchiveDocumentValidatesBeforeTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	in := validInput(anID('0'))
	in.Title = ""

	_, err := New(st, &stubIDs{id: dcID('V')}).CreateArchiveDocument(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "title" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю title", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestCreateArchiveDocumentUnitNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	_, err := New(st, &stubIDs{id: dcID('V')}).CreateArchiveDocument(context.Background(), validInput(anID('0')))

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "unit_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю unit_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d документов при несуществующей единице учёта", len(st.tx.saved))
	}
}

func TestCreateArchiveDocumentPropagatesSaveError(t *testing.T) {
	wantErr := errors.New("save failed")
	unit := anID('0')
	st := &fakeStore{tx: newFakeTx(&models.ArchiveNode{ID: unit, Type: "fond", ArchiveID: "AR-01ARZ3NDEKTSV4RRFFQ69G5FA0", Label: "Фонд"})}
	st.tx.saveErr = wantErr

	if _, err := New(st, &stubIDs{id: dcID('V')}).CreateArchiveDocument(context.Background(), validInput(unit)); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

func TestCreateArchiveDocumentPropagatesTxError(t *testing.T) {
	wantErr := errors.New("tx failed")
	st := &fakeStore{tx: newFakeTx(), inTxErr: wantErr}

	if _, err := New(st, &stubIDs{id: dcID('V')}).CreateArchiveDocument(context.Background(), validInput(anID('0'))); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

// TestCreateArchiveDocumentSourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestCreateArchiveDocumentSourceCitationNotFound(t *testing.T) {
	unit := anID('0')
	st := &fakeStore{tx: newFakeTx(&models.ArchiveNode{ID: unit, Type: "fond", ArchiveID: "AR-01ARZ3NDEKTSV4RRFFQ69G5FA0", Label: "Фонд"})}

	in := validInput(unit)
	in.Sources = []models.SourceLink{{CitationID: cID('0')}}

	_, err := New(st, &stubIDs{id: dcID('V')}).CreateArchiveDocument(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d документов при несуществующей цитате", len(st.tx.saved))
	}
}
```

#### `internal/usecases/update_archive_document/deps.go` (создать)
`internal/usecases/update_archive_document/deps.go`:
```go
package update_archive_document

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// ArchiveDocumentStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования, единицы учёта (unit_id) и сохранение идут в одной
// транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveDocumentStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
```

#### `internal/usecases/update_archive_document/scenario.go` (создать)
`internal/usecases/update_archive_document/scenario.go`:
```go
package update_archive_document

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение документа внутри единицы учёта».
type Scenario struct {
	store ArchiveDocumentStore
}

// New создаёт сценарий.
func New(st ArchiveDocumentStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateArchiveDocument полностью заменяет документ по d.ID: проверяет
// инварианты, в одной транзакции убеждается, что документ существует и
// (если задан) единица учёта существует, и сохраняет. ArchiveDocument не
// самореферентен — обхода цепочки на цикл, в отличие от
// update_archive_node, не требуется.
//
// Ошибки: невалидная сущность и несуществующая единица учёта —
// *models.ValidationError (поля соответствующего поля, unit_id); нет такого
// документа — models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateArchiveDocument(ctx context.Context, d models.ArchiveDocument) error {
	if err := d.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetArchiveDocument(ctx, d.ID); err != nil {
			return err
		}

		if _, err := tx.GetArchiveNode(ctx, d.UnitID); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return unitErr("единица учёта %q не найдена", d.UnitID)
			}

			return err
		}

		for i, link := range d.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveArchiveDocument(ctx, &d)
	})
}

// unitErr — *models.ValidationError по полю unit_id.
func unitErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeArchiveDocument,
		Field:  "unit_id",
		Reason: fmt.Sprintf(format, args...),
	}
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeArchiveDocument,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/update_archive_document/scenario_test.go` (создать)
`internal/usecases/update_archive_document/scenario_test.go`:
```go
package update_archive_document

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

func dcID(last byte) models.ID {
	return models.ID("DC-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func anID(last byte) models.ID {
	return models.ID("AN-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func cID(last byte) models.ID {
	return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// fakeTx реализует нужные сценарию методы store.Store поверх карт документов,
// узлов и цитат; остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	docs      map[models.ID]*models.ArchiveDocument
	nodes     map[models.ID]*models.ArchiveNode
	citations map[models.ID]*models.Citation
	saved     []*models.ArchiveDocument
	saveErr   error
}

func newFakeTx(existing ...*models.ArchiveDocument) *fakeTx {
	tx := &fakeTx{
		docs:      map[models.ID]*models.ArchiveDocument{},
		nodes:     map[models.ID]*models.ArchiveNode{},
		citations: map[models.ID]*models.Citation{},
	}
	for _, d := range existing {
		tx.docs[d.ID] = d
	}

	return tx
}

func (f *fakeTx) withNode(n *models.ArchiveNode) *fakeTx {
	f.nodes[n.ID] = n

	return f
}

func (f *fakeTx) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *c

	return &cp, nil
}

func (f *fakeTx) GetArchiveNode(_ context.Context, id models.ID) (*models.ArchiveNode, error) {
	n, ok := f.nodes[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *n

	return &cp, nil
}

func (f *fakeTx) GetArchiveDocument(_ context.Context, id models.ID) (*models.ArchiveDocument, error) {
	d, ok := f.docs[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *d

	return &cp, nil
}

func (f *fakeTx) SaveArchiveDocument(_ context.Context, d *models.ArchiveDocument) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *d
	f.docs[d.ID] = &cp
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует ArchiveDocumentStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func doc(id models.ID, unit models.ID) *models.ArchiveDocument {
	return &models.ArchiveDocument{ID: id, UnitID: unit, Title: "документ " + string(id)}
}

func TestUpdateArchiveDocumentSaves(t *testing.T) {
	unit := anID('0')
	existing := doc(dcID('V'), unit)
	st := &fakeStore{tx: newFakeTx(existing).withNode(&models.ArchiveNode{ID: unit, Type: "fond", ArchiveID: "AR-01ARZ3NDEKTSV4RRFFQ69G5FA0", Label: "Фонд"})}

	updated := *existing
	updated.Title = "новое название"

	if err := New(st).UpdateArchiveDocument(context.Background(), updated); err != nil {
		t.Fatalf("UpdateArchiveDocument: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Title != "новое название" {
		t.Fatalf("InTx=%d, saved=%v; ожидалось одно сохранение с новым названием", st.calls, st.tx.saved)
	}
}

func TestUpdateArchiveDocumentNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateArchiveDocument(context.Background(), *doc(dcID('V'), anID('0')))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d документов при несуществующем документе", len(st.tx.saved))
	}
}

func TestUpdateArchiveDocumentValidatesBeforeTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	bad := *doc(dcID('V'), anID('0'))
	bad.Title = ""

	err := New(st).UpdateArchiveDocument(context.Background(), bad)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "title" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю title", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestUpdateArchiveDocumentUnitNotFound(t *testing.T) {
	existing := doc(dcID('V'), anID('0'))
	st := &fakeStore{tx: newFakeTx(existing)} // единица учёта не заведена

	missing := anID('9')
	updated := *existing
	updated.UnitID = missing

	err := New(st).UpdateArchiveDocument(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "unit_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю unit_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d документов при несуществующей единице учёта", len(st.tx.saved))
	}
}

func TestUpdateArchiveDocumentPropagatesSaveError(t *testing.T) {
	wantErr := errors.New("save failed")
	unit := anID('0')
	existing := doc(dcID('V'), unit)
	tx := newFakeTx(existing).withNode(&models.ArchiveNode{ID: unit, Type: "fond", ArchiveID: "AR-01ARZ3NDEKTSV4RRFFQ69G5FA0", Label: "Фонд"})
	tx.saveErr = wantErr
	st := &fakeStore{tx: tx}

	if err := New(st).UpdateArchiveDocument(context.Background(), *existing); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

// TestUpdateArchiveDocumentSourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestUpdateArchiveDocumentSourceCitationNotFound(t *testing.T) {
	unit := anID('0')
	existing := doc(dcID('V'), unit)
	st := &fakeStore{tx: newFakeTx(existing).withNode(&models.ArchiveNode{ID: unit, Type: "fond", ArchiveID: "AR-01ARZ3NDEKTSV4RRFFQ69G5FA0", Label: "Фонд"})}

	updated := *existing
	updated.Sources = []models.SourceLink{{CitationID: cID('0')}}

	err := New(st).UpdateArchiveDocument(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d документов при несуществующей цитате", len(st.tx.saved))
	}
}
```

#### `internal/usecases/delete_archive_document/deps.go` (создать)
`internal/usecases/delete_archive_document/deps.go`:
```go
package delete_archive_document

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ArchiveDocumentRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveDocumentRepo interface {
	DeleteArchiveDocument(ctx context.Context, id models.ID) error
}
```

#### `internal/usecases/delete_archive_document/scenario.go` (создать)
`internal/usecases/delete_archive_document/scenario.go`:
```go
package delete_archive_document

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление документа внутри единицы учёта».
type Scenario struct {
	archiveDocuments ArchiveDocumentRepo
}

// New создаёт сценарий.
func New(archiveDocuments ArchiveDocumentRepo) *Scenario {
	return &Scenario{archiveDocuments: archiveDocuments}
}

// DeleteArchiveDocument удаляет документ. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такого
// документа — models.ErrNotFound; на документ ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteArchiveDocument(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.archiveDocuments.DeleteArchiveDocument(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeArchiveDocument)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeArchiveDocument,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

#### `internal/usecases/delete_archive_document/scenario_test.go` (создать)
`internal/usecases/delete_archive_document/scenario_test.go`:
```go
package delete_archive_document

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	deleted []models.ID
	err     error
}

func (f *fakeRepo) DeleteArchiveDocument(_ context.Context, id models.ID) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)

	return nil
}

func TestDeleteArchiveDocumentCallsRepo(t *testing.T) {
	repo := &fakeRepo{}
	id := models.ID("DC-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	if err := New(repo).DeleteArchiveDocument(context.Background(), id); err != nil {
		t.Fatalf("DeleteArchiveDocument: %v", err)
	}

	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("deleted = %v", repo.deleted)
	}
}

func TestDeleteArchiveDocumentInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	err := New(repo).DeleteArchiveDocument(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}

	if len(repo.deleted) != 0 {
		t.Fatalf("repo вызван при невалидном id")
	}
}

func TestDeleteArchiveDocumentPropagatesInUseError(t *testing.T) {
	wantErr := &models.InUseError{Type: models.TypeArchiveDocument, ID: "DC-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	repo := &fakeRepo{err: wantErr}

	err := New(repo).DeleteArchiveDocument(context.Background(), "DC-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	var iu *models.InUseError
	if !errors.As(err, &iu) {
		t.Fatalf("err = %v, want InUseError", err)
	}
}
```

#### `internal/httpapi/archive_document.go` (создать)
`internal/httpapi/archive_document.go`:
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleArchiveDocumentList — GET /api/archive-documents?limit=&offset=.
// Плоская сущность (в отличие от archive-nodes) — без обязательных фильтров.
func handleArchiveDocumentList(archiveDocuments ArchiveDocumentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := archiveDocuments.ListArchiveDocuments(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ArchiveDocumentsFromModels(list))
	}
}

// handleArchiveDocumentSearch — GET /api/archive-documents/search?q=&limit=&offset=.
func handleArchiveDocumentSearch(archiveDocuments ArchiveDocumentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := archiveDocuments.SearchArchiveDocuments(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ArchiveDocumentsFromModels(list))
	}
}

// handleArchiveDocumentGet — GET /api/archive-documents/{id}.
func handleArchiveDocumentGet(archiveDocuments ArchiveDocumentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		d, err := archiveDocuments.GetArchiveDocument(r.Context(), AccessFromContext(r.Context()), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ArchiveDocumentFromModel(d))
	}
}
```

#### `internal/httpapi/archive_document_write.go` (создать)
`internal/httpapi/archive_document_write.go`:
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleArchiveDocumentCreate — POST /api/archive-documents: создаёт
// документ, отвечает 201 с созданным документом (id генерирует сценарий).
// Несуществующий unit_id — 422 (см. writeError). Запись — только для
// вошедшего владельца, см. handleArchiveCreate.
func handleArchiveDocumentCreate(archiveDocuments ArchiveDocumentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.ArchiveDocumentCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := archiveDocuments.CreateArchiveDocument(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.ArchiveDocumentFromModel(created))
	}
}

// handleArchiveDocumentUpdate — PUT /api/archive-documents/{id}: полная
// замена всех полей документа (fetch-then-merge, как у ArchiveNode/Archive).
func handleArchiveDocumentUpdate(archiveDocuments ArchiveDocumentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.ArchiveDocumentUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := archiveDocuments.GetArchiveDocument(r.Context(), AccessFromContext(r.Context()), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.UnitID = m.UnitID
		cur.Title = m.Title
		cur.Kind = m.Kind
		cur.Since = m.Since
		cur.Until = m.Until
		cur.Parish = m.Parish
		cur.Settlements = m.Settlements
		cur.Notes = m.Notes
		cur.Sources = m.Sources
		cur.Private = m.Private

		if err := archiveDocuments.UpdateArchiveDocument(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ArchiveDocumentFromModel(cur))
	}
}

// handleArchiveDocumentDelete — DELETE /api/archive-documents/{id}: 204 без
// тела; занятый документ — 409 со списком ссылающихся.
func handleArchiveDocumentDelete(archiveDocuments ArchiveDocumentService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := archiveDocuments.DeleteArchiveDocument(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
```

#### `internal/httpapi/archive_document_test.go` (создать)
`internal/httpapi/archive_document_test.go`:
```go
package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeArchiveDocuments struct {
	list []models.ArchiveDocument
	err  error
	page models.Page

	getD      models.ArchiveDocument
	gotIDs    []models.ID
	created   models.ArchiveDocument
	gotCreate models.ArchiveDocument
	updated   models.ArchiveDocument
	deleteErr error

	search    []models.ArchiveDocument
	gotSearch models.SearchQuery
}

func (f *fakeArchiveDocuments) ListArchiveDocuments(_ context.Context, _ models.Access, page models.Page) ([]models.ArchiveDocument, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakeArchiveDocuments) SearchArchiveDocuments(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.ArchiveDocument, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeArchiveDocuments) GetArchiveDocument(_ context.Context, _ models.Access, id models.ID) (models.ArchiveDocument, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.ArchiveDocument{}, f.err
	}

	return f.getD, nil
}

func (f *fakeArchiveDocuments) CreateArchiveDocument(_ context.Context, d models.ArchiveDocument) (models.ArchiveDocument, error) {
	f.gotCreate = d
	if f.err != nil {
		return models.ArchiveDocument{}, f.err
	}

	return f.created, nil
}

func (f *fakeArchiveDocuments) UpdateArchiveDocument(_ context.Context, d models.ArchiveDocument) error {
	f.updated = d

	return f.err
}

func (f *fakeArchiveDocuments) DeleteArchiveDocument(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestArchiveDocumentListReturnsRecords(t *testing.T) {
	svc := &fakeArchiveDocuments{list: []models.ArchiveDocument{{ID: "DC-1", UnitID: "AN-1", Title: "Метрическая книга"}}}

	rec := get(t, NewHandler(Deps{ArchiveDocs: svc, DocsFS: fstest.MapFS{}}), "/api/archive-documents")
	requireStatus(t, rec, 200)

	want := `[{"id":"DC-1","unit_id":"AN-1","title":"Метрическая книга","kind":"","settlements":[],"notes":[],"sources":[],"private":false}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestArchiveDocumentGetNotFound(t *testing.T) {
	svc := &fakeArchiveDocuments{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{ArchiveDocs: svc, DocsFS: fstest.MapFS{}}), "/api/archive-documents/DC-1")
	requireStatus(t, rec, 404)
}

func TestArchiveDocumentSearchPassesQuery(t *testing.T) {
	svc := &fakeArchiveDocuments{}

	rec := get(t, NewHandler(Deps{ArchiveDocs: svc, DocsFS: fstest.MapFS{}}), "/api/archive-documents/search?q=метрич")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "метрич" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
```

#### `internal/httpapi/archive_document_write_test.go` (создать)
`internal/httpapi/archive_document_write_test.go`:
```go
package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

func TestArchiveDocumentCreateContract(t *testing.T) {
	svc := &fakeArchiveDocuments{created: models.ArchiveDocument{ID: "DC-1", UnitID: "AN-1", Title: "Метрическая книга"}}

	rec := postD(t, NewHandler(Deps{ArchiveDocs: svc, DocsFS: fstest.MapFS{}}), "/api/archive-documents",
		`{"unit_id":"AN-1","title":"Метрическая книга"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.UnitID != "AN-1" || svc.gotCreate.Title != "Метрическая книга" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestArchiveDocumentCreateUnitNotFoundIs422 — usecase-слой возвращает
// *models.ValidationError по полю unit_id при несуществующей единице учёта
// (см. create_archive_document), writeError мапит это на 422.
func TestArchiveDocumentCreateUnitNotFoundIs422(t *testing.T) {
	svc := &fakeArchiveDocuments{err: &models.ValidationError{Entity: models.TypeArchiveDocument, Field: "unit_id", Reason: "единица учёта не найдена"}}

	rec := postD(t, NewHandler(Deps{ArchiveDocs: svc, DocsFS: fstest.MapFS{}}), "/api/archive-documents",
		`{"unit_id":"AN-999","title":"Документ-призрак"}`)

	requireStatus(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"unit_id"`) {
		t.Fatalf("body = %s, ожидалось поле unit_id", rec.Body)
	}
}

func TestArchiveDocumentCreateAnonymousIs401(t *testing.T) {
	svc := &fakeArchiveDocuments{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{ArchiveDocs: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/archive-documents", strings.NewReader(`{"unit_id":"AN-1","title":"Документ"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestArchiveDocumentUpdateMergesFields(t *testing.T) {
	svc := &fakeArchiveDocuments{getD: models.ArchiveDocument{ID: "DC-1", UnitID: "AN-1", Title: "Метрическая книга"}}

	rec := putD(t, NewHandler(Deps{ArchiveDocs: svc, DocsFS: fstest.MapFS{}}), "/api/archive-documents/DC-1",
		`{"unit_id":"AN-1","title":"Метрическая книга (испр.)","private":true}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Title != "Метрическая книга (испр.)" || svc.updated.Private != true || svc.updated.ID != "DC-1" {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestArchiveDocumentDeleteNoContent(t *testing.T) {
	svc := &fakeArchiveDocuments{}

	rec := delD(t, NewHandler(Deps{ArchiveDocs: svc, DocsFS: fstest.MapFS{}}), "/api/archive-documents/DC-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestArchiveDocumentDeleteInUseIs409(t *testing.T) {
	svc := &fakeArchiveDocuments{deleteErr: &models.InUseError{Type: models.TypeArchiveDocument, ID: "DC-1"}}

	rec := delD(t, NewHandler(Deps{ArchiveDocs: svc, DocsFS: fstest.MapFS{}}), "/api/archive-documents/DC-1")

	requireStatus(t, rec, http.StatusConflict)
}
```

#### `internal/mcp/archive_document.go` (создать)
`internal/mcp/archive_document.go`:
```go
package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// registerArchiveDocumentTools регистрирует тулы для работы с документами
// внутри единиц учёта. unit_id — обязательная строгая ссылка на
// ArchiveNode; в отличие от archive_node_list, archive_document_list —
// плоский список без обязательного фильтра (сущность не иерархична).
func registerArchiveDocumentTools(s *server.MCPServer, archiveDocuments ArchiveDocumentService) {
	tool := mcp.NewTool(
		"archive_document_list",
		mcp.WithDescription("Список документов внутри единиц учёта в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, archiveDocumentListHandler(archiveDocuments))

	tool = mcp.NewTool(
		"archive_document_search",
		mcp.WithDescription("Поиск документов внутри единиц учёта по началу названия; результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало названия")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, archiveDocumentSearchHandler(archiveDocuments))

	tool = mcp.NewTool(
		"archive_document_get",
		mcp.WithDescription("Документ внутри единицы учёта по id; результат — JSON записи. Неверный формат id или отсутствующая запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например DC-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, archiveDocumentGetHandler(archiveDocuments))

	tool = mcp.NewTool(
		"archive_document_create",
		mcp.WithDescription("Создать документ внутри единицы учёта; id генерируется сервером; результат — JSON созданной записи. Несуществующий unit_id — ошибка тула"),
		mcp.WithString("unit_id", mcp.Required(), mcp.Description("id единицы учёта (узла архивного дерева)")),
		mcp.WithString("title", mcp.Required(), mcp.Description("Название документа")),
		mcp.WithString("kind", mcp.Description("Вид документа (свободный текст)")),
		mcp.WithObject("since", mcp.Description("Начало периода"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("until", mcp.Description("Конец периода"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("parish", mcp.Description("Приход (текст или ссылка {text, ref?, type?})"), mcp.Properties(textRefObjectProperties())),
		mcp.WithArray("settlements", mcp.WithStringItems(), mcp.Description("Населённые пункты")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, archiveDocumentCreateHandler(archiveDocuments))

	tool = mcp.NewTool(
		"archive_document_update",
		mcp.WithDescription("Изменить документ внутри единицы учёта: полная замена unit_id/title/kind/since/until/parish/settlements/notes/private; результат — JSON обновлённой записи. Несуществующий unit_id — ошибка тула; sources — при отсутствии в вызове текущие источники сохраняются, пустой массив — очищает их"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("unit_id", mcp.Required(), mcp.Description("id единицы учёта")),
		mcp.WithString("title", mcp.Required(), mcp.Description("Название документа")),
		mcp.WithString("kind", mcp.Description("Вид документа")),
		mcp.WithObject("since", mcp.Description("Начало периода"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("until", mcp.Description("Конец периода"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("parish", mcp.Description("Приход (текст или ссылка {text, ref?, type?})"), mcp.Properties(textRefObjectProperties())),
		mcp.WithArray("settlements", mcp.WithStringItems(), mcp.Description("Населённые пункты")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, archiveDocumentUpdateHandler(archiveDocuments))

	tool = mcp.NewTool(
		"archive_document_delete",
		mcp.WithDescription("Удалить документ внутри единицы учёта. Необратимо. Если на него есть строгие ссылки от других сущностей — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, archiveDocumentDeleteHandler(archiveDocuments))
}

func archiveDocumentListHandler(archiveDocuments ArchiveDocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := archiveDocuments.ListArchiveDocuments(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.ArchiveDocumentsFromModels(list))
	}
}

func archiveDocumentSearchHandler(archiveDocuments ArchiveDocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := archiveDocuments.SearchArchiveDocuments(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.ArchiveDocumentsFromModels(list))
	}
}

func archiveDocumentGetHandler(archiveDocuments ArchiveDocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		d, err := archiveDocuments.GetArchiveDocument(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.ArchiveDocumentFromModel(d))
	}
}

func archiveDocumentCreateHandler(archiveDocuments ArchiveDocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		parish, err := optionalTextRef(args, "parish")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		since, err := optionalFactDate(args, "since")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		until, err := optionalFactDate(args, "until")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sources, err := optionalSourceLinks(args, "sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		d := models.ArchiveDocument{
			UnitID:      models.ID(req.GetString("unit_id", "")),
			Title:       req.GetString("title", ""),
			Kind:        req.GetString("kind", ""),
			Since:       since,
			Until:       until,
			Parish:      parish,
			Settlements: textRefsFromStrings(req.GetStringSlice("settlements", nil)),
			Notes:       textRefsFromStrings(req.GetStringSlice("notes", nil)),
			Sources:     sources,
			Private:     req.GetBool("private", false),
		}

		created, err := archiveDocuments.CreateArchiveDocument(ctx, d)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.ArchiveDocumentFromModel(created))
	}
}

func archiveDocumentUpdateHandler(archiveDocuments ArchiveDocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := archiveDocuments.GetArchiveDocument(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		args := req.GetArguments()

		parish, err := optionalTextRef(args, "parish")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		since, err := optionalFactDate(args, "since")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		until, err := optionalFactDate(args, "until")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cur.UnitID = models.ID(req.GetString("unit_id", ""))
		cur.Title = req.GetString("title", "")
		cur.Kind = req.GetString("kind", "")
		cur.Since = since
		cur.Until = until
		cur.Parish = parish
		cur.Settlements = textRefsFromStrings(req.GetStringSlice("settlements", nil))
		cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))
		cur.Private = req.GetBool("private", false)

		if raw, ok := args["sources"]; ok && raw != nil {
			sources, err := optionalSourceLinks(args, "sources")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Sources = sources
		}

		if err := archiveDocuments.UpdateArchiveDocument(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.ArchiveDocumentFromModel(cur))
	}
}

func archiveDocumentDeleteHandler(archiveDocuments ArchiveDocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := archiveDocuments.DeleteArchiveDocument(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
```

#### `internal/mcp/archive_document_test.go` (создать)
`internal/mcp/archive_document_test.go`:
```go
package mcp

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeArchiveDocuments struct {
	list []models.ArchiveDocument
	err  error

	getD      models.ArchiveDocument
	created   models.ArchiveDocument
	gotCreate models.ArchiveDocument
	updated   models.ArchiveDocument
	gotIDs    []models.ID
	deleteErr error

	search []models.ArchiveDocument
}

func (f *fakeArchiveDocuments) ListArchiveDocuments(context.Context, models.Access, models.Page) ([]models.ArchiveDocument, error) {
	return f.list, f.err
}

func (f *fakeArchiveDocuments) SearchArchiveDocuments(context.Context, models.Access, models.SearchQuery) ([]models.ArchiveDocument, error) {
	return f.search, f.err
}

func (f *fakeArchiveDocuments) GetArchiveDocument(_ context.Context, _ models.Access, id models.ID) (models.ArchiveDocument, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.ArchiveDocument{}, f.err
	}

	return f.getD, nil
}

func (f *fakeArchiveDocuments) CreateArchiveDocument(_ context.Context, d models.ArchiveDocument) (models.ArchiveDocument, error) {
	f.gotCreate = d
	if f.err != nil {
		return models.ArchiveDocument{}, f.err
	}

	return f.created, nil
}

func (f *fakeArchiveDocuments) UpdateArchiveDocument(_ context.Context, d models.ArchiveDocument) error {
	f.updated = d

	return f.err
}

func (f *fakeArchiveDocuments) DeleteArchiveDocument(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestArchiveDocumentGetToolContract(t *testing.T) {
	svc := &fakeArchiveDocuments{getD: models.ArchiveDocument{ID: "DC-1", Title: "Метрическая книга"}}

	res := callArchiveNodeTool(t, archiveDocumentGetHandler(svc), map[string]any{"id": "DC-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "DC-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

func TestArchiveDocumentCreateToolPassesFields(t *testing.T) {
	svc := &fakeArchiveDocuments{created: models.ArchiveDocument{ID: "DC-new", Title: "Метрическая книга"}}

	res := callArchiveNodeTool(t, archiveDocumentCreateHandler(svc), map[string]any{
		"unit_id": "AN-1",
		"title":   "Метрическая книга",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.UnitID != "AN-1" || svc.gotCreate.Title != "Метрическая книга" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestArchiveDocumentCreateToolUnitNotFoundIsError — usecase-слой возвращает
// ошибку на несуществующий unit_id, тул отдаёт её как ошибку тула.
func TestArchiveDocumentCreateToolUnitNotFoundIsError(t *testing.T) {
	svc := &fakeArchiveDocuments{err: &models.ValidationError{Entity: models.TypeArchiveDocument, Field: "unit_id", Reason: "не найдена"}}

	res := callArchiveNodeTool(t, archiveDocumentCreateHandler(svc), map[string]any{
		"unit_id": "AN-999",
		"title":   "Документ-призрак",
	})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

// TestArchiveDocumentUpdateToolPreservesSourcesWhenOmitted — sources
// отсутствует в вызове: текущие источники сохраняются как есть.
func TestArchiveDocumentUpdateToolPreservesSourcesWhenOmitted(t *testing.T) {
	existingSources := []models.SourceLink{{CitationID: "C-1"}}
	svc := &fakeArchiveDocuments{getD: models.ArchiveDocument{ID: "DC-1", UnitID: "AN-1", Title: "Метрическая книга", Sources: existingSources}}

	res := callArchiveNodeTool(t, archiveDocumentUpdateHandler(svc), map[string]any{
		"id":      "DC-1",
		"unit_id": "AN-1",
		"title":   "Метрическая книга (испр.)",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if len(svc.updated.Sources) != 1 || svc.updated.Sources[0].CitationID != "C-1" {
		t.Fatalf("updated.Sources = %+v, ожидались сохранённые источники", svc.updated.Sources)
	}
}

// TestArchiveDocumentUpdateToolClearsSourcesWhenEmptyArray — пустой массив
// sources в вызове очищает источники.
func TestArchiveDocumentUpdateToolClearsSourcesWhenEmptyArray(t *testing.T) {
	existingSources := []models.SourceLink{{CitationID: "C-1"}}
	svc := &fakeArchiveDocuments{getD: models.ArchiveDocument{ID: "DC-1", UnitID: "AN-1", Title: "Метрическая книга", Sources: existingSources}}

	res := callArchiveNodeTool(t, archiveDocumentUpdateHandler(svc), map[string]any{
		"id":      "DC-1",
		"unit_id": "AN-1",
		"title":   "Метрическая книга (испр.)",
		"sources": []any{},
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if len(svc.updated.Sources) != 0 {
		t.Fatalf("updated.Sources = %+v, ожидался пустой список", svc.updated.Sources)
	}
}

func TestArchiveDocumentDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeArchiveDocuments{deleteErr: &models.InUseError{Type: models.TypeArchiveDocument, ID: "DC-1"}}

	res := callArchiveNodeTool(t, archiveDocumentDeleteHandler(svc), map[string]any{"id": "DC-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersArchiveDocumentTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, ArchiveDocs: &fakeArchiveDocuments{}}).ListTools()

	for _, name := range []string{
		"archive_document_list", "archive_document_search", "archive_document_get",
		"archive_document_create", "archive_document_update", "archive_document_delete",
	} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
```

### 1.4. `Deps`-реестр: подключить ArchiveNode и ArchiveDocument

#### `internal/httpapi/deps.go` (изменить — итоговое содержимое)
`internal/httpapi/deps.go`:
```go
package httpapi

import (
	"context"

	authpkg "github.com/amarin/genodex/internal/auth"
	"github.com/amarin/genodex/internal/models"
)

// DivisionService — контракт сценариев административного деления, отдаваемых
// в HTTP: список, чтение, создание, изменение, удаление.
type DivisionService interface {
	ListDivisions(ctx context.Context, access models.Access, q models.DivisionQuery) ([]models.AdministrativeDivision, error)
	SearchDivisions(ctx context.Context, access models.Access, q models.DivisionSearchQuery) ([]models.AdministrativeDivision, error)
	GetDivision(ctx context.Context, id models.ID) (models.AdministrativeDivision, error)
	CreateDivision(ctx context.Context, d models.AdministrativeDivision) (models.AdministrativeDivision, error)
	UpdateDivision(ctx context.Context, d models.AdministrativeDivision) error
	DeleteDivision(ctx context.Context, id models.ID) error
}

// SurnameService — контракт сценариев словарных записей фамилий, отдаваемых
// в HTTP: список, поиск, чтение, создание, изменение, удаление.
type SurnameService interface {
	ListSurnames(ctx context.Context, access models.Access, page models.Page) ([]models.Surname, error)
	SearchSurnames(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Surname, error)
	GetSurname(ctx context.Context, id models.ID) (models.Surname, error)
	CreateSurname(ctx context.Context, sn models.Surname) (models.Surname, error)
	UpdateSurname(ctx context.Context, sn models.Surname) error
	DeleteSurname(ctx context.Context, id models.ID) error
}

// PatronymicService — контракт сценариев словарных записей (отчеств), отдаваемых
// в HTTP: список, поиск, чтение, создание, изменение, удаление.
type PatronymicService interface {
	ListPatronymics(ctx context.Context, access models.Access, page models.Page) ([]models.Patronymic, error)
	SearchPatronymics(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Patronymic, error)
	GetPatronymic(ctx context.Context, id models.ID) (models.Patronymic, error)
	CreatePatronymic(ctx context.Context, x models.Patronymic) (models.Patronymic, error)
	UpdatePatronymic(ctx context.Context, x models.Patronymic) error
	DeletePatronymic(ctx context.Context, id models.ID) error
}

// EstateService — контракт сценариев словарных записей (сословий), отдаваемых
// в HTTP: список, поиск, чтение, создание, изменение, удаление.
type EstateService interface {
	ListEstates(ctx context.Context, access models.Access, page models.Page) ([]models.Estate, error)
	SearchEstates(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Estate, error)
	GetEstate(ctx context.Context, id models.ID) (models.Estate, error)
	CreateEstate(ctx context.Context, x models.Estate) (models.Estate, error)
	UpdateEstate(ctx context.Context, x models.Estate) error
	DeleteEstate(ctx context.Context, id models.ID) error
}

// TitleService — контракт сценариев словарных записей (званий/титулов), отдаваемых
// в HTTP: список, поиск, чтение, создание, изменение, удаление.
type TitleService interface {
	ListTitles(ctx context.Context, access models.Access, page models.Page) ([]models.Title, error)
	SearchTitles(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Title, error)
	GetTitle(ctx context.Context, id models.ID) (models.Title, error)
	CreateTitle(ctx context.Context, x models.Title) (models.Title, error)
	UpdateTitle(ctx context.Context, x models.Title) error
	DeleteTitle(ctx context.Context, id models.ID) error
}

// GivenNameService — контракт сценариев словарных записей имён, отдаваемых
// в HTTP: список, поиск, чтение, создание, изменение, удаление.
type GivenNameService interface {
	ListGivenNames(ctx context.Context, access models.Access, page models.Page) ([]models.GivenName, error)
	SearchGivenNames(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.GivenName, error)
	GetGivenName(ctx context.Context, id models.ID) (models.GivenName, error)
	CreateGivenName(ctx context.Context, x models.GivenName) (models.GivenName, error)
	UpdateGivenName(ctx context.Context, x models.GivenName) error
	DeleteGivenName(ctx context.Context, id models.ID) error
}

// RepositoryService — контракт сценариев хранилищ-контейнеров источников,
// отдаваемых в HTTP: список, поиск, чтение, создание, изменение, удаление.
type RepositoryService interface {
	ListRepositories(ctx context.Context, access models.Access, page models.Page) ([]models.Repository, error)
	SearchRepositories(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Repository, error)
	GetRepository(ctx context.Context, access models.Access, id models.ID) (models.Repository, error)
	CreateRepository(ctx context.Context, r models.Repository) (models.Repository, error)
	UpdateRepository(ctx context.Context, r models.Repository) error
	DeleteRepository(ctx context.Context, id models.ID) error
}

// ChurchService — контракт сценариев церквей, отдаваемых в HTTP: список,
// поиск, чтение, создание, изменение, удаление.
type ChurchService interface {
	ListChurches(ctx context.Context, access models.Access, page models.Page) ([]models.Church, error)
	SearchChurches(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Church, error)
	GetChurch(ctx context.Context, id models.ID) (models.Church, error)
	CreateChurch(ctx context.Context, c models.Church) (models.Church, error)
	UpdateChurch(ctx context.Context, c models.Church) error
	DeleteChurch(ctx context.Context, id models.ID) error
}

// ParishService — контракт сценариев приходов, отдаваемых в HTTP: список,
// поиск, чтение, создание, изменение, удаление.
type ParishService interface {
	ListParishes(ctx context.Context, access models.Access, page models.Page) ([]models.Parish, error)
	SearchParishes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Parish, error)
	GetParish(ctx context.Context, id models.ID) (models.Parish, error)
	CreateParish(ctx context.Context, p models.Parish) (models.Parish, error)
	UpdateParish(ctx context.Context, p models.Parish) error
	DeleteParish(ctx context.Context, id models.ID) error
}

// ArchiveService — контракт сценариев архивов, отдаваемых в HTTP: список,
// поиск, чтение, создание, изменение, удаление.
type ArchiveService interface {
	ListArchives(ctx context.Context, access models.Access, page models.Page) ([]models.Archive, error)
	SearchArchives(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Archive, error)
	GetArchive(ctx context.Context, access models.Access, id models.ID) (models.Archive, error)
	CreateArchive(ctx context.Context, a models.Archive) (models.Archive, error)
	UpdateArchive(ctx context.Context, a models.Archive) error
	DeleteArchive(ctx context.Context, id models.ID) error
}

// ArchiveNodeService — контракт сценариев узлов архивного дерева, отдаваемых
// в HTTP: список (обязательный archive_id — см. models.ArchiveNodeQuery),
// поиск, чтение, создание, изменение, удаление.
type ArchiveNodeService interface {
	ListArchiveNodes(ctx context.Context, access models.Access, q models.ArchiveNodeQuery) ([]models.ArchiveNode, error)
	SearchArchiveNodes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.ArchiveNode, error)
	GetArchiveNode(ctx context.Context, access models.Access, id models.ID) (models.ArchiveNode, error)
	CreateArchiveNode(ctx context.Context, n models.ArchiveNode) (models.ArchiveNode, error)
	UpdateArchiveNode(ctx context.Context, n models.ArchiveNode) error
	DeleteArchiveNode(ctx context.Context, id models.ID) error
}

// ArchiveDocumentService — контракт сценариев документов внутри единиц
// учёта, отдаваемых в HTTP: список, поиск, чтение, создание, изменение,
// удаление.
type ArchiveDocumentService interface {
	ListArchiveDocuments(ctx context.Context, access models.Access, page models.Page) ([]models.ArchiveDocument, error)
	SearchArchiveDocuments(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.ArchiveDocument, error)
	GetArchiveDocument(ctx context.Context, access models.Access, id models.ID) (models.ArchiveDocument, error)
	CreateArchiveDocument(ctx context.Context, d models.ArchiveDocument) (models.ArchiveDocument, error)
	UpdateArchiveDocument(ctx context.Context, d models.ArchiveDocument) error
	DeleteArchiveDocument(ctx context.Context, id models.ID) error
}

// NoteService — контракт сценариев заметок, отдаваемых в HTTP: список,
// поиск, чтение, создание, изменение, удаление.
type NoteService interface {
	ListNotes(ctx context.Context, access models.Access, page models.Page) ([]models.Note, error)
	SearchNotes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Note, error)
	GetNote(ctx context.Context, access models.Access, id models.ID) (models.Note, error)
	CreateNote(ctx context.Context, n models.Note) (models.Note, error)
	UpdateNote(ctx context.Context, n models.Note) error
	DeleteNote(ctx context.Context, id models.ID) error
}

// AttachmentService — контракт сценариев файловых вложений, отдаваемых в
// HTTP: список, поиск, чтение, создание, изменение, удаление.
type AttachmentService interface {
	ListAttachments(ctx context.Context, access models.Access, page models.Page) ([]models.Attachment, error)
	SearchAttachments(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Attachment, error)
	GetAttachment(ctx context.Context, access models.Access, id models.ID) (models.Attachment, error)
	CreateAttachment(ctx context.Context, a models.Attachment) (models.Attachment, error)
	UpdateAttachment(ctx context.Context, a models.Attachment) error
	DeleteAttachment(ctx context.Context, id models.ID) error
}

// SourceService — контракт сценариев источников доказательств, отдаваемых в
// HTTP: список, поиск, чтение, создание, изменение, удаление.
type SourceService interface {
	ListSources(ctx context.Context, access models.Access, page models.Page) ([]models.Source, error)
	SearchSources(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Source, error)
	GetSource(ctx context.Context, access models.Access, id models.ID) (models.Source, error)
	CreateSource(ctx context.Context, s models.Source) (models.Source, error)
	UpdateSource(ctx context.Context, s models.Source) error
	DeleteSource(ctx context.Context, id models.ID) error
}

// CitationService — контракт сценариев цитат, отдаваемых в HTTP: список,
// поиск, чтение, создание, изменение, удаление.
type CitationService interface {
	ListCitations(ctx context.Context, access models.Access, page models.Page) ([]models.Citation, error)
	SearchCitations(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Citation, error)
	GetCitation(ctx context.Context, access models.Access, id models.ID) (models.Citation, error)
	CreateCitation(ctx context.Context, c models.Citation) (models.Citation, error)
	UpdateCitation(ctx context.Context, c models.Citation) error
	DeleteCitation(ctx context.Context, id models.ID) error
}

// AuthService — контракт auth.Service, отдаваемый в HTTP-обработчики.
type AuthService interface {
	Bootstrap(ctx context.Context) (bool, error)
	Register(ctx context.Context, login, password string, invite *string) (authpkg.AuthResult, error)
	Login(ctx context.Context, login, password string) (authpkg.AuthResult, error)
	Refresh(ctx context.Context, rawRefresh string) (authpkg.AuthResult, error)
	Logout(ctx context.Context, rawAccess string) error
	ResolveAccess(ctx context.Context, rawAccess string) (models.Access, *authpkg.ID, error)
	ChangePassword(ctx context.Context, ownerID authpkg.ID, current, newPassword string) error
	CreateInvite(ctx context.Context, ownerID authpkg.ID) (string, error)
	CreateAPIToken(ctx context.Context, ownerID authpkg.ID, label string) (string, authpkg.ID, error)
	ListAPITokens(ctx context.Context, ownerID authpkg.ID) ([]authpkg.APIToken, error)
	RevokeAPIToken(ctx context.Context, ownerID, tokenID authpkg.ID) error
	GetOwner(ctx context.Context, id authpkg.ID) (*authpkg.Owner, error)
}
```

#### `internal/httpapi/api.go` (изменить — итоговое содержимое)
`internal/httpapi/api.go`:
```go
package httpapi

import (
	"io/fs"
	"net/http"
)

// Deps — сервисы, монтируемые в /api (NewAPIHandler) и /api без auth-обёртки
// (NewHandler, юнит-тесты пакета). Явный реестр вместо растущего списка
// позиционных параметров — новая сущность добавляется полем структуры, не
// меняя сигнатуру функций (docs/data-model/entity-write.md §3; решение
// принято при добавлении Surname, первой сущности после AdministrativeDivision).
type Deps struct {
	Divisions    DivisionService
	Surnames     SurnameService
	Patronymics  PatronymicService
	Estates      EstateService
	Titles       TitleService
	GivenNames   GivenNameService
	Repositories RepositoryService
	Churches     ChurchService
	Parishes     ParishService
	Archives     ArchiveService
	ArchiveNodes ArchiveNodeService
	ArchiveDocs  ArchiveDocumentService
	Notes        NoteService
	Attachments  AttachmentService
	Sources      SourceService
	Citations    CitationService
	Auth         AuthService
	DocsFS       fs.FS
	TrustProxy   bool
}

// NewAPIHandler — единая точка входа /api: маршруты делений, фамилий,
// документации и auth на одном mux, обёрнутые ОДИН раз resolveAccess +
// requireCSRFHeader (auth.md §4 — исходный замысел дизайна: оба миддлвари
// вокруг всего /api/, не только /api/auth/*). Это то, что реально монтирует
// internal/app (этап C). Deps.TrustProxy — см. isSecureRequest (auth.go),
// включается флагом -trust-proxy. NewHandler и NewAuthHandler остаются
// отдельно для существующих юнит-тестов пакета, не зависящих от auth.
func NewAPIHandler(deps Deps) http.Handler {
	mux := http.NewServeMux()
	registerDivisionRoutes(mux, deps.Divisions, deps.DocsFS)

	if deps.Surnames != nil {
		registerSurnameRoutes(mux, deps.Surnames)
	}

	if deps.Patronymics != nil {
		registerPatronymicRoutes(mux, deps.Patronymics)
	}

	if deps.Estates != nil {
		registerEstateRoutes(mux, deps.Estates)
	}

	if deps.Titles != nil {
		registerTitleRoutes(mux, deps.Titles)
	}

	if deps.GivenNames != nil {
		registerGivenNameRoutes(mux, deps.GivenNames)
	}

	if deps.Repositories != nil {
		registerRepositoryRoutes(mux, deps.Repositories)
	}

	if deps.Churches != nil {
		registerChurchRoutes(mux, deps.Churches)
	}

	if deps.Parishes != nil {
		registerParishRoutes(mux, deps.Parishes)
	}

	if deps.Archives != nil {
		registerArchiveRoutes(mux, deps.Archives)
	}

	if deps.ArchiveNodes != nil {
		registerArchiveNodeRoutes(mux, deps.ArchiveNodes)
	}

	if deps.ArchiveDocs != nil {
		registerArchiveDocumentRoutes(mux, deps.ArchiveDocs)
	}

	if deps.Notes != nil {
		registerNoteRoutes(mux, deps.Notes)
	}

	if deps.Attachments != nil {
		registerAttachmentRoutes(mux, deps.Attachments)
	}

	if deps.Sources != nil {
		registerSourceRoutes(mux, deps.Sources)
	}

	if deps.Citations != nil {
		registerCitationRoutes(mux, deps.Citations)
	}

	registerAuthRoutes(mux, deps.Auth, deps.TrustProxy)

	return requireCSRFHeader(resolveAccess(deps.Auth)(mux))
}
```

#### `internal/httpapi/httpapi.go` (изменить — итоговое содержимое)
`internal/httpapi/httpapi.go`:
```go
package httpapi

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strings"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// NewHandler возвращает http.Handler с маршрутами /api (без auth-
// оборачивания) — используется юнит-тестами этого пакета напрямую.
// Реальное приложение монтирует NewAPIHandler (api.go). Deps.Auth/TrustProxy
// не используются (без auth-обёртки), Deps.Surnames может быть nil, если
// тесту нужны только маршруты делений.
func NewHandler(deps Deps) http.Handler {
	mux := http.NewServeMux()
	registerDivisionRoutes(mux, deps.Divisions, deps.DocsFS)

	if deps.Surnames != nil {
		registerSurnameRoutes(mux, deps.Surnames)
	}

	if deps.Patronymics != nil {
		registerPatronymicRoutes(mux, deps.Patronymics)
	}

	if deps.Estates != nil {
		registerEstateRoutes(mux, deps.Estates)
	}

	if deps.Titles != nil {
		registerTitleRoutes(mux, deps.Titles)
	}

	if deps.GivenNames != nil {
		registerGivenNameRoutes(mux, deps.GivenNames)
	}

	if deps.Repositories != nil {
		registerRepositoryRoutes(mux, deps.Repositories)
	}

	if deps.Churches != nil {
		registerChurchRoutes(mux, deps.Churches)
	}

	if deps.Parishes != nil {
		registerParishRoutes(mux, deps.Parishes)
	}

	if deps.Archives != nil {
		registerArchiveRoutes(mux, deps.Archives)
	}

	if deps.ArchiveNodes != nil {
		registerArchiveNodeRoutes(mux, deps.ArchiveNodes)
	}

	if deps.ArchiveDocs != nil {
		registerArchiveDocumentRoutes(mux, deps.ArchiveDocs)
	}

	if deps.Notes != nil {
		registerNoteRoutes(mux, deps.Notes)
	}

	if deps.Attachments != nil {
		registerAttachmentRoutes(mux, deps.Attachments)
	}

	if deps.Sources != nil {
		registerSourceRoutes(mux, deps.Sources)
	}

	if deps.Citations != nil {
		registerCitationRoutes(mux, deps.Citations)
	}

	return mux
}

// registerDivisionRoutes регистрирует маршруты /api/admin-divisions,
// /api/docs, /api/health на переданном mux — общий код NewHandler и
// NewAPIHandler.
func registerDivisionRoutes(mux *http.ServeMux, divisions DivisionService, docsFS fs.FS) {
	mux.HandleFunc("GET /api/health", handleHealth)
	mux.HandleFunc("GET /api/admin-divisions", handleDivisionList(divisions))
	mux.HandleFunc("GET /api/admin-divisions/search", handleDivisionSearch(divisions))
	mux.HandleFunc("GET /api/admin-divisions/{id}", handleDivisionGet(divisions))
	mux.HandleFunc("POST /api/admin-divisions", handleDivisionCreate(divisions))
	mux.HandleFunc("PUT /api/admin-divisions/{id}", handleDivisionUpdate(divisions))
	mux.HandleFunc("DELETE /api/admin-divisions/{id}", handleDivisionDelete(divisions))
	mux.HandleFunc("GET /api/docs", handleDocList(docsFS))
	mux.HandleFunc("GET /api/docs/{path}", handleDocContent(docsFS))
}

// registerSurnameRoutes регистрирует маршруты /api/surnames на переданном mux.
func registerSurnameRoutes(mux *http.ServeMux, surnames SurnameService) {
	mux.HandleFunc("GET /api/surnames", handleSurnameList(surnames))
	mux.HandleFunc("GET /api/surnames/search", handleSurnameSearch(surnames))
	mux.HandleFunc("GET /api/surnames/{id}", handleSurnameGet(surnames))
	mux.HandleFunc("POST /api/surnames", handleSurnameCreate(surnames))
	mux.HandleFunc("PUT /api/surnames/{id}", handleSurnameUpdate(surnames))
	mux.HandleFunc("DELETE /api/surnames/{id}", handleSurnameDelete(surnames))
}

// registerPatronymicRoutes регистрирует маршруты /api/patronymics на переданном mux.
func registerPatronymicRoutes(mux *http.ServeMux, patronymics PatronymicService) {
	mux.HandleFunc("GET /api/patronymics", handlePatronymicList(patronymics))
	mux.HandleFunc("GET /api/patronymics/search", handlePatronymicSearch(patronymics))
	mux.HandleFunc("GET /api/patronymics/{id}", handlePatronymicGet(patronymics))
	mux.HandleFunc("POST /api/patronymics", handlePatronymicCreate(patronymics))
	mux.HandleFunc("PUT /api/patronymics/{id}", handlePatronymicUpdate(patronymics))
	mux.HandleFunc("DELETE /api/patronymics/{id}", handlePatronymicDelete(patronymics))
}

// registerEstateRoutes регистрирует маршруты /api/estates на переданном mux.
func registerEstateRoutes(mux *http.ServeMux, estates EstateService) {
	mux.HandleFunc("GET /api/estates", handleEstateList(estates))
	mux.HandleFunc("GET /api/estates/search", handleEstateSearch(estates))
	mux.HandleFunc("GET /api/estates/{id}", handleEstateGet(estates))
	mux.HandleFunc("POST /api/estates", handleEstateCreate(estates))
	mux.HandleFunc("PUT /api/estates/{id}", handleEstateUpdate(estates))
	mux.HandleFunc("DELETE /api/estates/{id}", handleEstateDelete(estates))
}

// registerTitleRoutes регистрирует маршруты /api/titles на переданном mux.
func registerTitleRoutes(mux *http.ServeMux, titles TitleService) {
	mux.HandleFunc("GET /api/titles", handleTitleList(titles))
	mux.HandleFunc("GET /api/titles/search", handleTitleSearch(titles))
	mux.HandleFunc("GET /api/titles/{id}", handleTitleGet(titles))
	mux.HandleFunc("POST /api/titles", handleTitleCreate(titles))
	mux.HandleFunc("PUT /api/titles/{id}", handleTitleUpdate(titles))
	mux.HandleFunc("DELETE /api/titles/{id}", handleTitleDelete(titles))
}

// registerGivenNameRoutes регистрирует маршруты /api/given-names на переданном mux.
func registerGivenNameRoutes(mux *http.ServeMux, givenNames GivenNameService) {
	mux.HandleFunc("GET /api/given-names", handleGivenNameList(givenNames))
	mux.HandleFunc("GET /api/given-names/search", handleGivenNameSearch(givenNames))
	mux.HandleFunc("GET /api/given-names/{id}", handleGivenNameGet(givenNames))
	mux.HandleFunc("POST /api/given-names", handleGivenNameCreate(givenNames))
	mux.HandleFunc("PUT /api/given-names/{id}", handleGivenNameUpdate(givenNames))
	mux.HandleFunc("DELETE /api/given-names/{id}", handleGivenNameDelete(givenNames))
}

// registerRepositoryRoutes регистрирует маршруты /api/repositories на переданном mux.
func registerRepositoryRoutes(mux *http.ServeMux, repositories RepositoryService) {
	mux.HandleFunc("GET /api/repositories", handleRepositoryList(repositories))
	mux.HandleFunc("GET /api/repositories/search", handleRepositorySearch(repositories))
	mux.HandleFunc("GET /api/repositories/{id}", handleRepositoryGet(repositories))
	mux.HandleFunc("POST /api/repositories", handleRepositoryCreate(repositories))
	mux.HandleFunc("PUT /api/repositories/{id}", handleRepositoryUpdate(repositories))
	mux.HandleFunc("DELETE /api/repositories/{id}", handleRepositoryDelete(repositories))
}

// registerChurchRoutes регистрирует маршруты /api/churches на переданном mux.
func registerChurchRoutes(mux *http.ServeMux, churches ChurchService) {
	mux.HandleFunc("GET /api/churches", handleChurchList(churches))
	mux.HandleFunc("GET /api/churches/search", handleChurchSearch(churches))
	mux.HandleFunc("GET /api/churches/{id}", handleChurchGet(churches))
	mux.HandleFunc("POST /api/churches", handleChurchCreate(churches))
	mux.HandleFunc("PUT /api/churches/{id}", handleChurchUpdate(churches))
	mux.HandleFunc("DELETE /api/churches/{id}", handleChurchDelete(churches))
}

// registerParishRoutes регистрирует маршруты /api/parishes на переданном mux.
func registerParishRoutes(mux *http.ServeMux, parishes ParishService) {
	mux.HandleFunc("GET /api/parishes", handleParishList(parishes))
	mux.HandleFunc("GET /api/parishes/search", handleParishSearch(parishes))
	mux.HandleFunc("GET /api/parishes/{id}", handleParishGet(parishes))
	mux.HandleFunc("POST /api/parishes", handleParishCreate(parishes))
	mux.HandleFunc("PUT /api/parishes/{id}", handleParishUpdate(parishes))
	mux.HandleFunc("DELETE /api/parishes/{id}", handleParishDelete(parishes))
}

// registerArchiveRoutes регистрирует маршруты /api/archives на переданном mux.
func registerArchiveRoutes(mux *http.ServeMux, archives ArchiveService) {
	mux.HandleFunc("GET /api/archives", handleArchiveList(archives))
	mux.HandleFunc("GET /api/archives/search", handleArchiveSearch(archives))
	mux.HandleFunc("GET /api/archives/{id}", handleArchiveGet(archives))
	mux.HandleFunc("POST /api/archives", handleArchiveCreate(archives))
	mux.HandleFunc("PUT /api/archives/{id}", handleArchiveUpdate(archives))
	mux.HandleFunc("DELETE /api/archives/{id}", handleArchiveDelete(archives))
}

// registerArchiveNodeRoutes регистрирует маршруты /api/archive-nodes на
// переданном mux. archive_id обязателен для списка (см. parseArchiveNodeQuery).
func registerArchiveNodeRoutes(mux *http.ServeMux, archiveNodes ArchiveNodeService) {
	mux.HandleFunc("GET /api/archive-nodes", handleArchiveNodeList(archiveNodes))
	mux.HandleFunc("GET /api/archive-nodes/search", handleArchiveNodeSearch(archiveNodes))
	mux.HandleFunc("GET /api/archive-nodes/{id}", handleArchiveNodeGet(archiveNodes))
	mux.HandleFunc("POST /api/archive-nodes", handleArchiveNodeCreate(archiveNodes))
	mux.HandleFunc("PUT /api/archive-nodes/{id}", handleArchiveNodeUpdate(archiveNodes))
	mux.HandleFunc("DELETE /api/archive-nodes/{id}", handleArchiveNodeDelete(archiveNodes))
}

// registerArchiveDocumentRoutes регистрирует маршруты /api/archive-documents
// на переданном mux.
func registerArchiveDocumentRoutes(mux *http.ServeMux, archiveDocuments ArchiveDocumentService) {
	mux.HandleFunc("GET /api/archive-documents", handleArchiveDocumentList(archiveDocuments))
	mux.HandleFunc("GET /api/archive-documents/search", handleArchiveDocumentSearch(archiveDocuments))
	mux.HandleFunc("GET /api/archive-documents/{id}", handleArchiveDocumentGet(archiveDocuments))
	mux.HandleFunc("POST /api/archive-documents", handleArchiveDocumentCreate(archiveDocuments))
	mux.HandleFunc("PUT /api/archive-documents/{id}", handleArchiveDocumentUpdate(archiveDocuments))
	mux.HandleFunc("DELETE /api/archive-documents/{id}", handleArchiveDocumentDelete(archiveDocuments))
}

// registerNoteRoutes регистрирует маршруты /api/notes на переданном mux.
func registerNoteRoutes(mux *http.ServeMux, notes NoteService) {
	mux.HandleFunc("GET /api/notes", handleNoteList(notes))
	mux.HandleFunc("GET /api/notes/search", handleNoteSearch(notes))
	mux.HandleFunc("GET /api/notes/{id}", handleNoteGet(notes))
	mux.HandleFunc("POST /api/notes", handleNoteCreate(notes))
	mux.HandleFunc("PUT /api/notes/{id}", handleNoteUpdate(notes))
	mux.HandleFunc("DELETE /api/notes/{id}", handleNoteDelete(notes))
}

// registerAttachmentRoutes регистрирует маршруты /api/attachments на переданном mux.
func registerAttachmentRoutes(mux *http.ServeMux, attachments AttachmentService) {
	mux.HandleFunc("GET /api/attachments", handleAttachmentList(attachments))
	mux.HandleFunc("GET /api/attachments/search", handleAttachmentSearch(attachments))
	mux.HandleFunc("GET /api/attachments/{id}", handleAttachmentGet(attachments))
	mux.HandleFunc("POST /api/attachments", handleAttachmentCreate(attachments))
	mux.HandleFunc("PUT /api/attachments/{id}", handleAttachmentUpdate(attachments))
	mux.HandleFunc("DELETE /api/attachments/{id}", handleAttachmentDelete(attachments))
}

// registerSourceRoutes регистрирует маршруты /api/sources на переданном mux.
func registerSourceRoutes(mux *http.ServeMux, sources SourceService) {
	mux.HandleFunc("GET /api/sources", handleSourceList(sources))
	mux.HandleFunc("GET /api/sources/search", handleSourceSearch(sources))
	mux.HandleFunc("GET /api/sources/{id}", handleSourceGet(sources))
	mux.HandleFunc("POST /api/sources", handleSourceCreate(sources))
	mux.HandleFunc("PUT /api/sources/{id}", handleSourceUpdate(sources))
	mux.HandleFunc("DELETE /api/sources/{id}", handleSourceDelete(sources))
}

// registerCitationRoutes регистрирует маршруты /api/citations на переданном mux.
func registerCitationRoutes(mux *http.ServeMux, citations CitationService) {
	mux.HandleFunc("GET /api/citations", handleCitationList(citations))
	mux.HandleFunc("GET /api/citations/search", handleCitationSearch(citations))
	mux.HandleFunc("GET /api/citations/{id}", handleCitationGet(citations))
	mux.HandleFunc("POST /api/citations", handleCitationCreate(citations))
	mux.HandleFunc("PUT /api/citations/{id}", handleCitationUpdate(citations))
	mux.HandleFunc("DELETE /api/citations/{id}", handleCitationDelete(citations))
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// pathID читает {id} из пути запроса — общий хелпер для всех сущностей
// (division.go's pathDivisionID — исторический синоним, оставлен как есть).
func pathID(r *http.Request) models.ID {
	return models.ID(strings.TrimPrefix(r.PathValue("id"), "/"))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError отвечает на ошибку сценария: *models.ValidationError — 422 с полем,
// models.ErrNotFound — 404, *models.InUseError — 409 с телом InUseErrorBody,
// остальное — 500.
func writeError(w http.ResponseWriter, err error) {
	var ve *models.ValidationError
	if errors.As(err, &ve) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": ve.Error(), "field": ve.Field})

		return
	}

	if errors.Is(err, models.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})

		return
	}

	var iu *models.InUseError
	if errors.As(err, &iu) {
		writeJSON(w, http.StatusConflict, transport.InUseErrorBodyFromModel(iu))

		return
	}

	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}
```

#### `internal/mcp/deps.go` (изменить — итоговое содержимое)
`internal/mcp/deps.go`:
```go
package mcp

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// DivisionService — контракт сценариев административного деления, отдаваемых в
// MCP-тулы: список, чтение, создание, изменение, удаление.
type DivisionService interface {
	ListDivisions(ctx context.Context, access models.Access, q models.DivisionQuery) ([]models.AdministrativeDivision, error)
	SearchDivisions(ctx context.Context, access models.Access, q models.DivisionSearchQuery) ([]models.AdministrativeDivision, error)
	GetDivision(ctx context.Context, id models.ID) (models.AdministrativeDivision, error)
	CreateDivision(ctx context.Context, d models.AdministrativeDivision) (models.AdministrativeDivision, error)
	UpdateDivision(ctx context.Context, d models.AdministrativeDivision) error
	DeleteDivision(ctx context.Context, id models.ID) error
}

// SurnameService — контракт сценариев словарных записей фамилий, отдаваемых
// в MCP-тулы: список, поиск, чтение, создание, изменение, удаление.
type SurnameService interface {
	ListSurnames(ctx context.Context, access models.Access, page models.Page) ([]models.Surname, error)
	SearchSurnames(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Surname, error)
	GetSurname(ctx context.Context, id models.ID) (models.Surname, error)
	CreateSurname(ctx context.Context, sn models.Surname) (models.Surname, error)
	UpdateSurname(ctx context.Context, sn models.Surname) error
	DeleteSurname(ctx context.Context, id models.ID) error
}

// PatronymicService — контракт сценариев словарных записей (отчеств), отдаваемых
// в MCP-тулы: список, поиск, чтение, создание, изменение, удаление.
type PatronymicService interface {
	ListPatronymics(ctx context.Context, access models.Access, page models.Page) ([]models.Patronymic, error)
	SearchPatronymics(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Patronymic, error)
	GetPatronymic(ctx context.Context, id models.ID) (models.Patronymic, error)
	CreatePatronymic(ctx context.Context, x models.Patronymic) (models.Patronymic, error)
	UpdatePatronymic(ctx context.Context, x models.Patronymic) error
	DeletePatronymic(ctx context.Context, id models.ID) error
}

// EstateService — контракт сценариев словарных записей (сословий), отдаваемых
// в MCP-тулы: список, поиск, чтение, создание, изменение, удаление.
type EstateService interface {
	ListEstates(ctx context.Context, access models.Access, page models.Page) ([]models.Estate, error)
	SearchEstates(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Estate, error)
	GetEstate(ctx context.Context, id models.ID) (models.Estate, error)
	CreateEstate(ctx context.Context, x models.Estate) (models.Estate, error)
	UpdateEstate(ctx context.Context, x models.Estate) error
	DeleteEstate(ctx context.Context, id models.ID) error
}

// TitleService — контракт сценариев словарных записей (званий/титулов), отдаваемых
// в MCP-тулы: список, поиск, чтение, создание, изменение, удаление.
type TitleService interface {
	ListTitles(ctx context.Context, access models.Access, page models.Page) ([]models.Title, error)
	SearchTitles(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Title, error)
	GetTitle(ctx context.Context, id models.ID) (models.Title, error)
	CreateTitle(ctx context.Context, x models.Title) (models.Title, error)
	UpdateTitle(ctx context.Context, x models.Title) error
	DeleteTitle(ctx context.Context, id models.ID) error
}

// GivenNameService — контракт сценариев словарных записей имён, отдаваемых
// в MCP-тулы: список, поиск, чтение, создание, изменение, удаление.
type GivenNameService interface {
	ListGivenNames(ctx context.Context, access models.Access, page models.Page) ([]models.GivenName, error)
	SearchGivenNames(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.GivenName, error)
	GetGivenName(ctx context.Context, id models.ID) (models.GivenName, error)
	CreateGivenName(ctx context.Context, x models.GivenName) (models.GivenName, error)
	UpdateGivenName(ctx context.Context, x models.GivenName) error
	DeleteGivenName(ctx context.Context, id models.ID) error
}

// RepositoryService — контракт сценариев хранилищ-контейнеров источников,
// отдаваемых в MCP-тулы: список, поиск, чтение, создание, изменение, удаление.
type RepositoryService interface {
	ListRepositories(ctx context.Context, access models.Access, page models.Page) ([]models.Repository, error)
	SearchRepositories(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Repository, error)
	GetRepository(ctx context.Context, access models.Access, id models.ID) (models.Repository, error)
	CreateRepository(ctx context.Context, r models.Repository) (models.Repository, error)
	UpdateRepository(ctx context.Context, r models.Repository) error
	DeleteRepository(ctx context.Context, id models.ID) error
}

// ChurchService — контракт сценариев церквей, отдаваемых в MCP-тулы: список,
// поиск, чтение, создание, изменение, удаление.
type ChurchService interface {
	ListChurches(ctx context.Context, access models.Access, page models.Page) ([]models.Church, error)
	SearchChurches(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Church, error)
	GetChurch(ctx context.Context, id models.ID) (models.Church, error)
	CreateChurch(ctx context.Context, c models.Church) (models.Church, error)
	UpdateChurch(ctx context.Context, c models.Church) error
	DeleteChurch(ctx context.Context, id models.ID) error
}

// ParishService — контракт сценариев приходов, отдаваемых в MCP-тулы: список,
// поиск, чтение, создание, изменение, удаление.
type ParishService interface {
	ListParishes(ctx context.Context, access models.Access, page models.Page) ([]models.Parish, error)
	SearchParishes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Parish, error)
	GetParish(ctx context.Context, id models.ID) (models.Parish, error)
	CreateParish(ctx context.Context, p models.Parish) (models.Parish, error)
	UpdateParish(ctx context.Context, p models.Parish) error
	DeleteParish(ctx context.Context, id models.ID) error
}

// ArchiveService — контракт сценариев архивов, отдаваемых в MCP-тулы: список,
// поиск, чтение, создание, изменение, удаление.
type ArchiveService interface {
	ListArchives(ctx context.Context, access models.Access, page models.Page) ([]models.Archive, error)
	SearchArchives(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Archive, error)
	GetArchive(ctx context.Context, access models.Access, id models.ID) (models.Archive, error)
	CreateArchive(ctx context.Context, a models.Archive) (models.Archive, error)
	UpdateArchive(ctx context.Context, a models.Archive) error
	DeleteArchive(ctx context.Context, id models.ID) error
}

// ArchiveNodeService — контракт сценариев узлов архивного дерева, отдаваемых
// в MCP-тулы: список (обязательный archive_id), поиск, чтение, создание,
// изменение, удаление.
type ArchiveNodeService interface {
	ListArchiveNodes(ctx context.Context, access models.Access, q models.ArchiveNodeQuery) ([]models.ArchiveNode, error)
	SearchArchiveNodes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.ArchiveNode, error)
	GetArchiveNode(ctx context.Context, access models.Access, id models.ID) (models.ArchiveNode, error)
	CreateArchiveNode(ctx context.Context, n models.ArchiveNode) (models.ArchiveNode, error)
	UpdateArchiveNode(ctx context.Context, n models.ArchiveNode) error
	DeleteArchiveNode(ctx context.Context, id models.ID) error
}

// ArchiveDocumentService — контракт сценариев документов внутри единиц
// учёта, отдаваемых в MCP-тулы: список, поиск, чтение, создание, изменение,
// удаление.
type ArchiveDocumentService interface {
	ListArchiveDocuments(ctx context.Context, access models.Access, page models.Page) ([]models.ArchiveDocument, error)
	SearchArchiveDocuments(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.ArchiveDocument, error)
	GetArchiveDocument(ctx context.Context, access models.Access, id models.ID) (models.ArchiveDocument, error)
	CreateArchiveDocument(ctx context.Context, d models.ArchiveDocument) (models.ArchiveDocument, error)
	UpdateArchiveDocument(ctx context.Context, d models.ArchiveDocument) error
	DeleteArchiveDocument(ctx context.Context, id models.ID) error
}

// NoteService — контракт сценариев заметок, отдаваемых в MCP-тулы: список,
// поиск, чтение, создание, изменение, удаление.
type NoteService interface {
	ListNotes(ctx context.Context, access models.Access, page models.Page) ([]models.Note, error)
	SearchNotes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Note, error)
	GetNote(ctx context.Context, access models.Access, id models.ID) (models.Note, error)
	CreateNote(ctx context.Context, n models.Note) (models.Note, error)
	UpdateNote(ctx context.Context, n models.Note) error
	DeleteNote(ctx context.Context, id models.ID) error
}

// AttachmentService — контракт сценариев файловых вложений, отдаваемых в
// MCP-тулы: список, поиск, чтение, создание, изменение, удаление.
type AttachmentService interface {
	ListAttachments(ctx context.Context, access models.Access, page models.Page) ([]models.Attachment, error)
	SearchAttachments(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Attachment, error)
	GetAttachment(ctx context.Context, access models.Access, id models.ID) (models.Attachment, error)
	CreateAttachment(ctx context.Context, a models.Attachment) (models.Attachment, error)
	UpdateAttachment(ctx context.Context, a models.Attachment) error
	DeleteAttachment(ctx context.Context, id models.ID) error
}

// SourceService — контракт сценариев источников доказательств, отдаваемых в
// MCP-тулы: список, поиск, чтение, создание, изменение, удаление.
type SourceService interface {
	ListSources(ctx context.Context, access models.Access, page models.Page) ([]models.Source, error)
	SearchSources(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Source, error)
	GetSource(ctx context.Context, access models.Access, id models.ID) (models.Source, error)
	CreateSource(ctx context.Context, s models.Source) (models.Source, error)
	UpdateSource(ctx context.Context, s models.Source) error
	DeleteSource(ctx context.Context, id models.ID) error
}

// CitationService — контракт сценариев цитат, отдаваемых в MCP-тулы: список,
// поиск, чтение, создание, изменение, удаление.
type CitationService interface {
	ListCitations(ctx context.Context, access models.Access, page models.Page) ([]models.Citation, error)
	SearchCitations(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Citation, error)
	GetCitation(ctx context.Context, access models.Access, id models.ID) (models.Citation, error)
	CreateCitation(ctx context.Context, c models.Citation) (models.Citation, error)
	UpdateCitation(ctx context.Context, c models.Citation) error
	DeleteCitation(ctx context.Context, id models.ID) error
}
```

#### `internal/mcp/server.go` (изменить — итоговое содержимое)
`internal/mcp/server.go`:
```go
package mcp

import (
	"github.com/mark3labs/mcp-go/server"
)

// Deps — сервисы, отдаваемые в MCP-тулы. Явный реестр вместо растущего
// списка позиционных параметров (docs/data-model/entity-write.md §3) — новая
// сущность добавляется полем структуры, не меняя сигнатуру NewServer.
type Deps struct {
	Divisions    DivisionService
	Surnames     SurnameService
	Patronymics  PatronymicService
	Estates      EstateService
	Titles       TitleService
	GivenNames   GivenNameService
	Repositories RepositoryService
	Churches     ChurchService
	Parishes     ParishService
	Archives     ArchiveService
	ArchiveNodes ArchiveNodeService
	ArchiveDocs  ArchiveDocumentService
	Notes        NoteService
	Attachments  AttachmentService
	Sources      SourceService
	Citations    CitationService
}

// NewServer создаёт MCP-сервер и регистрирует доступные тулы.
func NewServer(deps Deps) *server.MCPServer {
	s := server.NewMCPServer(
		"genodex",
		"0.1.0",
	)

	registerDivisionTools(s, deps.Divisions)

	if deps.Surnames != nil {
		registerSurnameTools(s, deps.Surnames)
	}

	if deps.Patronymics != nil {
		registerPatronymicTools(s, deps.Patronymics)
	}

	if deps.Estates != nil {
		registerEstateTools(s, deps.Estates)
	}

	if deps.Titles != nil {
		registerTitleTools(s, deps.Titles)
	}

	if deps.GivenNames != nil {
		registerGivenNameTools(s, deps.GivenNames)
	}

	if deps.Repositories != nil {
		registerRepositoryTools(s, deps.Repositories)
	}

	if deps.Churches != nil {
		registerChurchTools(s, deps.Churches)
	}

	if deps.Parishes != nil {
		registerParishTools(s, deps.Parishes)
	}

	if deps.Archives != nil {
		registerArchiveTools(s, deps.Archives)
	}

	if deps.ArchiveNodes != nil {
		registerArchiveNodeTools(s, deps.ArchiveNodes)
	}

	if deps.ArchiveDocs != nil {
		registerArchiveDocumentTools(s, deps.ArchiveDocs)
	}

	if deps.Notes != nil {
		registerNoteTools(s, deps.Notes)
	}

	if deps.Attachments != nil {
		registerAttachmentTools(s, deps.Attachments)
	}

	if deps.Sources != nil {
		registerSourceTools(s, deps.Sources)
	}

	if deps.Citations != nil {
		registerCitationTools(s, deps.Citations)
	}

	return s
}
```

#### `internal/app/app.go` (изменить — итоговое содержимое)
`internal/app/app.go`:
```go
package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex"
	"github.com/amarin/genodex/internal/auth"
	"github.com/amarin/genodex/internal/httpapi"
	"github.com/amarin/genodex/internal/idgen"
	"github.com/amarin/genodex/internal/mcp"
	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store/sqlstore"
	create_archive "github.com/amarin/genodex/internal/usecases/create_archive"
	create_archive_document "github.com/amarin/genodex/internal/usecases/create_archive_document"
	create_archive_node "github.com/amarin/genodex/internal/usecases/create_archive_node"
	create_attachment "github.com/amarin/genodex/internal/usecases/create_attachment"
	create_church "github.com/amarin/genodex/internal/usecases/create_church"
	create_citation "github.com/amarin/genodex/internal/usecases/create_citation"
	create_division "github.com/amarin/genodex/internal/usecases/create_division"
	create_estate "github.com/amarin/genodex/internal/usecases/create_estate"
	create_given_name "github.com/amarin/genodex/internal/usecases/create_given_name"
	create_note "github.com/amarin/genodex/internal/usecases/create_note"
	create_parish "github.com/amarin/genodex/internal/usecases/create_parish"
	create_patronymic "github.com/amarin/genodex/internal/usecases/create_patronymic"
	create_repository "github.com/amarin/genodex/internal/usecases/create_repository"
	create_source "github.com/amarin/genodex/internal/usecases/create_source"
	create_surname "github.com/amarin/genodex/internal/usecases/create_surname"
	create_title "github.com/amarin/genodex/internal/usecases/create_title"
	delete_archive "github.com/amarin/genodex/internal/usecases/delete_archive"
	delete_archive_document "github.com/amarin/genodex/internal/usecases/delete_archive_document"
	delete_archive_node "github.com/amarin/genodex/internal/usecases/delete_archive_node"
	delete_attachment "github.com/amarin/genodex/internal/usecases/delete_attachment"
	delete_church "github.com/amarin/genodex/internal/usecases/delete_church"
	delete_citation "github.com/amarin/genodex/internal/usecases/delete_citation"
	delete_division "github.com/amarin/genodex/internal/usecases/delete_division"
	delete_estate "github.com/amarin/genodex/internal/usecases/delete_estate"
	delete_given_name "github.com/amarin/genodex/internal/usecases/delete_given_name"
	delete_note "github.com/amarin/genodex/internal/usecases/delete_note"
	delete_parish "github.com/amarin/genodex/internal/usecases/delete_parish"
	delete_patronymic "github.com/amarin/genodex/internal/usecases/delete_patronymic"
	delete_repository "github.com/amarin/genodex/internal/usecases/delete_repository"
	delete_source "github.com/amarin/genodex/internal/usecases/delete_source"
	delete_surname "github.com/amarin/genodex/internal/usecases/delete_surname"
	delete_title "github.com/amarin/genodex/internal/usecases/delete_title"
	get_archive "github.com/amarin/genodex/internal/usecases/get_archive"
	get_archive_document "github.com/amarin/genodex/internal/usecases/get_archive_document"
	get_archive_node "github.com/amarin/genodex/internal/usecases/get_archive_node"
	get_attachment "github.com/amarin/genodex/internal/usecases/get_attachment"
	get_church "github.com/amarin/genodex/internal/usecases/get_church"
	get_citation "github.com/amarin/genodex/internal/usecases/get_citation"
	get_division "github.com/amarin/genodex/internal/usecases/get_division"
	get_estate "github.com/amarin/genodex/internal/usecases/get_estate"
	get_given_name "github.com/amarin/genodex/internal/usecases/get_given_name"
	get_note "github.com/amarin/genodex/internal/usecases/get_note"
	get_parish "github.com/amarin/genodex/internal/usecases/get_parish"
	get_patronymic "github.com/amarin/genodex/internal/usecases/get_patronymic"
	get_repository "github.com/amarin/genodex/internal/usecases/get_repository"
	get_source "github.com/amarin/genodex/internal/usecases/get_source"
	get_surname "github.com/amarin/genodex/internal/usecases/get_surname"
	get_title "github.com/amarin/genodex/internal/usecases/get_title"
	list_archive_documents "github.com/amarin/genodex/internal/usecases/list_archive_documents"
	list_archive_nodes "github.com/amarin/genodex/internal/usecases/list_archive_nodes"
	list_archives "github.com/amarin/genodex/internal/usecases/list_archives"
	list_attachments "github.com/amarin/genodex/internal/usecases/list_attachments"
	list_churches "github.com/amarin/genodex/internal/usecases/list_churches"
	list_citations "github.com/amarin/genodex/internal/usecases/list_citations"
	list_divisions "github.com/amarin/genodex/internal/usecases/list_divisions"
	list_estates "github.com/amarin/genodex/internal/usecases/list_estates"
	list_given_names "github.com/amarin/genodex/internal/usecases/list_given_names"
	list_notes "github.com/amarin/genodex/internal/usecases/list_notes"
	list_parishes "github.com/amarin/genodex/internal/usecases/list_parishes"
	list_patronymics "github.com/amarin/genodex/internal/usecases/list_patronymics"
	list_repositories "github.com/amarin/genodex/internal/usecases/list_repositories"
	list_sources "github.com/amarin/genodex/internal/usecases/list_sources"
	list_surnames "github.com/amarin/genodex/internal/usecases/list_surnames"
	list_titles "github.com/amarin/genodex/internal/usecases/list_titles"
	search_archive_documents "github.com/amarin/genodex/internal/usecases/search_archive_documents"
	search_archive_nodes "github.com/amarin/genodex/internal/usecases/search_archive_nodes"
	search_archives "github.com/amarin/genodex/internal/usecases/search_archives"
	search_attachments "github.com/amarin/genodex/internal/usecases/search_attachments"
	search_churches "github.com/amarin/genodex/internal/usecases/search_churches"
	search_citations "github.com/amarin/genodex/internal/usecases/search_citations"
	search_divisions "github.com/amarin/genodex/internal/usecases/search_divisions"
	search_estates "github.com/amarin/genodex/internal/usecases/search_estates"
	search_given_names "github.com/amarin/genodex/internal/usecases/search_given_names"
	search_notes "github.com/amarin/genodex/internal/usecases/search_notes"
	search_parishes "github.com/amarin/genodex/internal/usecases/search_parishes"
	search_patronymics "github.com/amarin/genodex/internal/usecases/search_patronymics"
	search_repositories "github.com/amarin/genodex/internal/usecases/search_repositories"
	search_sources "github.com/amarin/genodex/internal/usecases/search_sources"
	search_surnames "github.com/amarin/genodex/internal/usecases/search_surnames"
	search_titles "github.com/amarin/genodex/internal/usecases/search_titles"
	update_archive "github.com/amarin/genodex/internal/usecases/update_archive"
	update_archive_document "github.com/amarin/genodex/internal/usecases/update_archive_document"
	update_archive_node "github.com/amarin/genodex/internal/usecases/update_archive_node"
	update_attachment "github.com/amarin/genodex/internal/usecases/update_attachment"
	update_church "github.com/amarin/genodex/internal/usecases/update_church"
	update_citation "github.com/amarin/genodex/internal/usecases/update_citation"
	update_division "github.com/amarin/genodex/internal/usecases/update_division"
	update_estate "github.com/amarin/genodex/internal/usecases/update_estate"
	update_given_name "github.com/amarin/genodex/internal/usecases/update_given_name"
	update_note "github.com/amarin/genodex/internal/usecases/update_note"
	update_parish "github.com/amarin/genodex/internal/usecases/update_parish"
	update_patronymic "github.com/amarin/genodex/internal/usecases/update_patronymic"
	update_repository "github.com/amarin/genodex/internal/usecases/update_repository"
	update_source "github.com/amarin/genodex/internal/usecases/update_source"
	update_surname "github.com/amarin/genodex/internal/usecases/update_surname"
	update_title "github.com/amarin/genodex/internal/usecases/update_title"
	"github.com/amarin/genodex/web"
)

// Config — параметры запуска приложения.
type Config struct {
	DataDir    string
	Port       int
	WebMode    string
	TrustProxy bool
}

// App — корневой объект приложения: собирает хранилище, usecases и интерфейсы.
type App struct {
	cfg   Config
	http  *http.Server
	store *sqlstore.Store
}

// divisionService — фасад всех сценариев делений, отдаваемых HTTP и MCP.
type divisionService struct {
	list   *list_divisions.Scenario
	search *search_divisions.Scenario
	get    *get_division.Scenario
	create *create_division.Scenario
	update *update_division.Scenario
	del    *delete_division.Scenario
}

func (s *divisionService) ListDivisions(ctx context.Context, access models.Access, q models.DivisionQuery) ([]models.AdministrativeDivision, error) {
	return s.list.ListDivisions(ctx, access, q)
}

func (s *divisionService) SearchDivisions(ctx context.Context, access models.Access, q models.DivisionSearchQuery) ([]models.AdministrativeDivision, error) {
	return s.search.SearchDivisions(ctx, access, q)
}

func (s *divisionService) GetDivision(ctx context.Context, id models.ID) (models.AdministrativeDivision, error) {
	return s.get.GetDivision(ctx, id)
}

func (s *divisionService) CreateDivision(ctx context.Context, d models.AdministrativeDivision) (models.AdministrativeDivision, error) {
	return s.create.CreateDivision(ctx, d)
}

func (s *divisionService) UpdateDivision(ctx context.Context, d models.AdministrativeDivision) error {
	return s.update.UpdateDivision(ctx, d)
}

func (s *divisionService) DeleteDivision(ctx context.Context, id models.ID) error {
	return s.del.DeleteDivision(ctx, id)
}

// surnameService — фасад всех сценариев словарных записей фамилий, отдаваемых
// HTTP и MCP. Тот же приём, что divisionService — по одному полю на
// сценарий, тонкие методы-делегаты (docs/data-model/entity-write.md §3).
type surnameService struct {
	list   *list_surnames.Scenario
	search *search_surnames.Scenario
	get    *get_surname.Scenario
	create *create_surname.Scenario
	update *update_surname.Scenario
	del    *delete_surname.Scenario
}

func (s *surnameService) ListSurnames(ctx context.Context, access models.Access, page models.Page) ([]models.Surname, error) {
	return s.list.ListSurnames(ctx, access, page)
}

func (s *surnameService) SearchSurnames(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Surname, error) {
	return s.search.SearchSurnames(ctx, access, q)
}

func (s *surnameService) GetSurname(ctx context.Context, id models.ID) (models.Surname, error) {
	return s.get.GetSurname(ctx, id)
}

func (s *surnameService) CreateSurname(ctx context.Context, sn models.Surname) (models.Surname, error) {
	return s.create.CreateSurname(ctx, sn)
}

func (s *surnameService) UpdateSurname(ctx context.Context, sn models.Surname) error {
	return s.update.UpdateSurname(ctx, sn)
}

func (s *surnameService) DeleteSurname(ctx context.Context, id models.ID) error {
	return s.del.DeleteSurname(ctx, id)
}

// patronymicService — фасад всех сценариев словарных записей (отчеств), отдаваемых
// HTTP и MCP. Тот же приём, что divisionService/surnameService — по одному
// полю на сценарий, тонкие методы-делегаты (docs/data-model/entity-write.md §3).
type patronymicService struct {
	list   *list_patronymics.Scenario
	search *search_patronymics.Scenario
	get    *get_patronymic.Scenario
	create *create_patronymic.Scenario
	update *update_patronymic.Scenario
	del    *delete_patronymic.Scenario
}

func (s *patronymicService) ListPatronymics(ctx context.Context, access models.Access, page models.Page) ([]models.Patronymic, error) {
	return s.list.ListPatronymics(ctx, access, page)
}

func (s *patronymicService) SearchPatronymics(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Patronymic, error) {
	return s.search.SearchPatronymics(ctx, access, q)
}

func (s *patronymicService) GetPatronymic(ctx context.Context, id models.ID) (models.Patronymic, error) {
	return s.get.GetPatronymic(ctx, id)
}

func (s *patronymicService) CreatePatronymic(ctx context.Context, x models.Patronymic) (models.Patronymic, error) {
	return s.create.CreatePatronymic(ctx, x)
}

func (s *patronymicService) UpdatePatronymic(ctx context.Context, x models.Patronymic) error {
	return s.update.UpdatePatronymic(ctx, x)
}

func (s *patronymicService) DeletePatronymic(ctx context.Context, id models.ID) error {
	return s.del.DeletePatronymic(ctx, id)
}

// estateService — фасад всех сценариев словарных записей (сословий), отдаваемых
// HTTP и MCP. Тот же приём, что divisionService/surnameService — по одному
// полю на сценарий, тонкие методы-делегаты (docs/data-model/entity-write.md §3).
type estateService struct {
	list   *list_estates.Scenario
	search *search_estates.Scenario
	get    *get_estate.Scenario
	create *create_estate.Scenario
	update *update_estate.Scenario
	del    *delete_estate.Scenario
}

func (s *estateService) ListEstates(ctx context.Context, access models.Access, page models.Page) ([]models.Estate, error) {
	return s.list.ListEstates(ctx, access, page)
}

func (s *estateService) SearchEstates(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Estate, error) {
	return s.search.SearchEstates(ctx, access, q)
}

func (s *estateService) GetEstate(ctx context.Context, id models.ID) (models.Estate, error) {
	return s.get.GetEstate(ctx, id)
}

func (s *estateService) CreateEstate(ctx context.Context, x models.Estate) (models.Estate, error) {
	return s.create.CreateEstate(ctx, x)
}

func (s *estateService) UpdateEstate(ctx context.Context, x models.Estate) error {
	return s.update.UpdateEstate(ctx, x)
}

func (s *estateService) DeleteEstate(ctx context.Context, id models.ID) error {
	return s.del.DeleteEstate(ctx, id)
}

// titleService — фасад всех сценариев словарных записей (званий/титулов), отдаваемых
// HTTP и MCP. Тот же приём, что divisionService/surnameService — по одному
// полю на сценарий, тонкие методы-делегаты (docs/data-model/entity-write.md §3).
type titleService struct {
	list   *list_titles.Scenario
	search *search_titles.Scenario
	get    *get_title.Scenario
	create *create_title.Scenario
	update *update_title.Scenario
	del    *delete_title.Scenario
}

func (s *titleService) ListTitles(ctx context.Context, access models.Access, page models.Page) ([]models.Title, error) {
	return s.list.ListTitles(ctx, access, page)
}

func (s *titleService) SearchTitles(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Title, error) {
	return s.search.SearchTitles(ctx, access, q)
}

func (s *titleService) GetTitle(ctx context.Context, id models.ID) (models.Title, error) {
	return s.get.GetTitle(ctx, id)
}

func (s *titleService) CreateTitle(ctx context.Context, x models.Title) (models.Title, error) {
	return s.create.CreateTitle(ctx, x)
}

func (s *titleService) UpdateTitle(ctx context.Context, x models.Title) error {
	return s.update.UpdateTitle(ctx, x)
}

func (s *titleService) DeleteTitle(ctx context.Context, id models.ID) error {
	return s.del.DeleteTitle(ctx, id)
}

// givenNameService — фасад всех сценариев словарных записей (имён), отдаваемых
// HTTP и MCP. Тот же приём, что divisionService/surnameService — по одному
// полю на сценарий, тонкие методы-делегаты (docs/data-model/entity-write.md §3).
type givenNameService struct {
	list   *list_given_names.Scenario
	search *search_given_names.Scenario
	get    *get_given_name.Scenario
	create *create_given_name.Scenario
	update *update_given_name.Scenario
	del    *delete_given_name.Scenario
}

func (s *givenNameService) ListGivenNames(ctx context.Context, access models.Access, page models.Page) ([]models.GivenName, error) {
	return s.list.ListGivenNames(ctx, access, page)
}

func (s *givenNameService) SearchGivenNames(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.GivenName, error) {
	return s.search.SearchGivenNames(ctx, access, q)
}

func (s *givenNameService) GetGivenName(ctx context.Context, id models.ID) (models.GivenName, error) {
	return s.get.GetGivenName(ctx, id)
}

func (s *givenNameService) CreateGivenName(ctx context.Context, x models.GivenName) (models.GivenName, error) {
	return s.create.CreateGivenName(ctx, x)
}

func (s *givenNameService) UpdateGivenName(ctx context.Context, x models.GivenName) error {
	return s.update.UpdateGivenName(ctx, x)
}

func (s *givenNameService) DeleteGivenName(ctx context.Context, id models.ID) error {
	return s.del.DeleteGivenName(ctx, id)
}

// repositoryService — фасад всех сценариев хранилищ-контейнеров источников,
// отдаваемых HTTP и MCP. Тот же приём, что divisionService/surnameService —
// по одному полю на сценарий, тонкие методы-делегаты (docs/data-model/entity-write.md §3).
type repositoryService struct {
	list   *list_repositories.Scenario
	search *search_repositories.Scenario
	get    *get_repository.Scenario
	create *create_repository.Scenario
	update *update_repository.Scenario
	del    *delete_repository.Scenario
}

func (s *repositoryService) ListRepositories(ctx context.Context, access models.Access, page models.Page) ([]models.Repository, error) {
	return s.list.ListRepositories(ctx, access, page)
}

func (s *repositoryService) SearchRepositories(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Repository, error) {
	return s.search.SearchRepositories(ctx, access, q)
}

func (s *repositoryService) GetRepository(ctx context.Context, access models.Access, id models.ID) (models.Repository, error) {
	return s.get.GetRepository(ctx, access, id)
}

func (s *repositoryService) CreateRepository(ctx context.Context, r models.Repository) (models.Repository, error) {
	return s.create.CreateRepository(ctx, r)
}

func (s *repositoryService) UpdateRepository(ctx context.Context, r models.Repository) error {
	return s.update.UpdateRepository(ctx, r)
}

func (s *repositoryService) DeleteRepository(ctx context.Context, id models.ID) error {
	return s.del.DeleteRepository(ctx, id)
}

// churchService — фасад всех сценариев церквей, отдаваемых HTTP и MCP.
type churchService struct {
	list   *list_churches.Scenario
	search *search_churches.Scenario
	get    *get_church.Scenario
	create *create_church.Scenario
	update *update_church.Scenario
	del    *delete_church.Scenario
}

func (s *churchService) ListChurches(ctx context.Context, access models.Access, page models.Page) ([]models.Church, error) {
	return s.list.ListChurches(ctx, access, page)
}

func (s *churchService) SearchChurches(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Church, error) {
	return s.search.SearchChurches(ctx, access, q)
}

func (s *churchService) GetChurch(ctx context.Context, id models.ID) (models.Church, error) {
	return s.get.GetChurch(ctx, id)
}

func (s *churchService) CreateChurch(ctx context.Context, c models.Church) (models.Church, error) {
	return s.create.CreateChurch(ctx, c)
}

func (s *churchService) UpdateChurch(ctx context.Context, c models.Church) error {
	return s.update.UpdateChurch(ctx, c)
}

func (s *churchService) DeleteChurch(ctx context.Context, id models.ID) error {
	return s.del.DeleteChurch(ctx, id)
}

// parishService — фасад всех сценариев приходов, отдаваемых HTTP и MCP.
type parishService struct {
	list   *list_parishes.Scenario
	search *search_parishes.Scenario
	get    *get_parish.Scenario
	create *create_parish.Scenario
	update *update_parish.Scenario
	del    *delete_parish.Scenario
}

func (s *parishService) ListParishes(ctx context.Context, access models.Access, page models.Page) ([]models.Parish, error) {
	return s.list.ListParishes(ctx, access, page)
}

func (s *parishService) SearchParishes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Parish, error) {
	return s.search.SearchParishes(ctx, access, q)
}

func (s *parishService) GetParish(ctx context.Context, id models.ID) (models.Parish, error) {
	return s.get.GetParish(ctx, id)
}

func (s *parishService) CreateParish(ctx context.Context, p models.Parish) (models.Parish, error) {
	return s.create.CreateParish(ctx, p)
}

func (s *parishService) UpdateParish(ctx context.Context, p models.Parish) error {
	return s.update.UpdateParish(ctx, p)
}

func (s *parishService) DeleteParish(ctx context.Context, id models.ID) error {
	return s.del.DeleteParish(ctx, id)
}

// archiveService — фасад всех сценариев архивов, отдаваемых HTTP и MCP.
type archiveService struct {
	list   *list_archives.Scenario
	search *search_archives.Scenario
	get    *get_archive.Scenario
	create *create_archive.Scenario
	update *update_archive.Scenario
	del    *delete_archive.Scenario
}

func (s *archiveService) ListArchives(ctx context.Context, access models.Access, page models.Page) ([]models.Archive, error) {
	return s.list.ListArchives(ctx, access, page)
}

func (s *archiveService) SearchArchives(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Archive, error) {
	return s.search.SearchArchives(ctx, access, q)
}

func (s *archiveService) GetArchive(ctx context.Context, access models.Access, id models.ID) (models.Archive, error) {
	return s.get.GetArchive(ctx, access, id)
}

func (s *archiveService) CreateArchive(ctx context.Context, a models.Archive) (models.Archive, error) {
	return s.create.CreateArchive(ctx, a)
}

func (s *archiveService) UpdateArchive(ctx context.Context, a models.Archive) error {
	return s.update.UpdateArchive(ctx, a)
}

func (s *archiveService) DeleteArchive(ctx context.Context, id models.ID) error {
	return s.del.DeleteArchive(ctx, id)
}

// archiveNodeService — фасад всех сценариев узлов архивного дерева,
// отдаваемых HTTP и MCP.
type archiveNodeService struct {
	list   *list_archive_nodes.Scenario
	search *search_archive_nodes.Scenario
	get    *get_archive_node.Scenario
	create *create_archive_node.Scenario
	update *update_archive_node.Scenario
	del    *delete_archive_node.Scenario
}

func (s *archiveNodeService) ListArchiveNodes(ctx context.Context, access models.Access, q models.ArchiveNodeQuery) ([]models.ArchiveNode, error) {
	return s.list.ListArchiveNodes(ctx, access, q)
}

func (s *archiveNodeService) SearchArchiveNodes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.ArchiveNode, error) {
	return s.search.SearchArchiveNodes(ctx, access, q)
}

func (s *archiveNodeService) GetArchiveNode(ctx context.Context, access models.Access, id models.ID) (models.ArchiveNode, error) {
	return s.get.GetArchiveNode(ctx, access, id)
}

func (s *archiveNodeService) CreateArchiveNode(ctx context.Context, n models.ArchiveNode) (models.ArchiveNode, error) {
	return s.create.CreateArchiveNode(ctx, n)
}

func (s *archiveNodeService) UpdateArchiveNode(ctx context.Context, n models.ArchiveNode) error {
	return s.update.UpdateArchiveNode(ctx, n)
}

func (s *archiveNodeService) DeleteArchiveNode(ctx context.Context, id models.ID) error {
	return s.del.DeleteArchiveNode(ctx, id)
}

// archiveDocumentService — фасад всех сценариев документов внутри единиц
// учёта, отдаваемых HTTP и MCP.
type archiveDocumentService struct {
	list   *list_archive_documents.Scenario
	search *search_archive_documents.Scenario
	get    *get_archive_document.Scenario
	create *create_archive_document.Scenario
	update *update_archive_document.Scenario
	del    *delete_archive_document.Scenario
}

func (s *archiveDocumentService) ListArchiveDocuments(ctx context.Context, access models.Access, page models.Page) ([]models.ArchiveDocument, error) {
	return s.list.ListArchiveDocuments(ctx, access, page)
}

func (s *archiveDocumentService) SearchArchiveDocuments(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.ArchiveDocument, error) {
	return s.search.SearchArchiveDocuments(ctx, access, q)
}

func (s *archiveDocumentService) GetArchiveDocument(ctx context.Context, access models.Access, id models.ID) (models.ArchiveDocument, error) {
	return s.get.GetArchiveDocument(ctx, access, id)
}

func (s *archiveDocumentService) CreateArchiveDocument(ctx context.Context, d models.ArchiveDocument) (models.ArchiveDocument, error) {
	return s.create.CreateArchiveDocument(ctx, d)
}

func (s *archiveDocumentService) UpdateArchiveDocument(ctx context.Context, d models.ArchiveDocument) error {
	return s.update.UpdateArchiveDocument(ctx, d)
}

func (s *archiveDocumentService) DeleteArchiveDocument(ctx context.Context, id models.ID) error {
	return s.del.DeleteArchiveDocument(ctx, id)
}

// noteService — фасад всех сценариев заметок, отдаваемых HTTP и MCP.
type noteService struct {
	list   *list_notes.Scenario
	search *search_notes.Scenario
	get    *get_note.Scenario
	create *create_note.Scenario
	update *update_note.Scenario
	del    *delete_note.Scenario
}

func (s *noteService) ListNotes(ctx context.Context, access models.Access, page models.Page) ([]models.Note, error) {
	return s.list.ListNotes(ctx, access, page)
}

func (s *noteService) SearchNotes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Note, error) {
	return s.search.SearchNotes(ctx, access, q)
}

func (s *noteService) GetNote(ctx context.Context, access models.Access, id models.ID) (models.Note, error) {
	return s.get.GetNote(ctx, access, id)
}

func (s *noteService) CreateNote(ctx context.Context, n models.Note) (models.Note, error) {
	return s.create.CreateNote(ctx, n)
}

func (s *noteService) UpdateNote(ctx context.Context, n models.Note) error {
	return s.update.UpdateNote(ctx, n)
}

func (s *noteService) DeleteNote(ctx context.Context, id models.ID) error {
	return s.del.DeleteNote(ctx, id)
}

// attachmentService — фасад всех сценариев файловых вложений, отдаваемых
// HTTP и MCP.
type attachmentService struct {
	list   *list_attachments.Scenario
	search *search_attachments.Scenario
	get    *get_attachment.Scenario
	create *create_attachment.Scenario
	update *update_attachment.Scenario
	del    *delete_attachment.Scenario
}

func (s *attachmentService) ListAttachments(ctx context.Context, access models.Access, page models.Page) ([]models.Attachment, error) {
	return s.list.ListAttachments(ctx, access, page)
}

func (s *attachmentService) SearchAttachments(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Attachment, error) {
	return s.search.SearchAttachments(ctx, access, q)
}

func (s *attachmentService) GetAttachment(ctx context.Context, access models.Access, id models.ID) (models.Attachment, error) {
	return s.get.GetAttachment(ctx, access, id)
}

func (s *attachmentService) CreateAttachment(ctx context.Context, a models.Attachment) (models.Attachment, error) {
	return s.create.CreateAttachment(ctx, a)
}

func (s *attachmentService) UpdateAttachment(ctx context.Context, a models.Attachment) error {
	return s.update.UpdateAttachment(ctx, a)
}

func (s *attachmentService) DeleteAttachment(ctx context.Context, id models.ID) error {
	return s.del.DeleteAttachment(ctx, id)
}

// sourceService — фасад всех сценариев источников доказательств, отдаваемых
// HTTP и MCP.
type sourceService struct {
	list   *list_sources.Scenario
	search *search_sources.Scenario
	get    *get_source.Scenario
	create *create_source.Scenario
	update *update_source.Scenario
	del    *delete_source.Scenario
}

func (s *sourceService) ListSources(ctx context.Context, access models.Access, page models.Page) ([]models.Source, error) {
	return s.list.ListSources(ctx, access, page)
}

func (s *sourceService) SearchSources(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Source, error) {
	return s.search.SearchSources(ctx, access, q)
}

func (s *sourceService) GetSource(ctx context.Context, access models.Access, id models.ID) (models.Source, error) {
	return s.get.GetSource(ctx, access, id)
}

func (s *sourceService) CreateSource(ctx context.Context, src models.Source) (models.Source, error) {
	return s.create.CreateSource(ctx, src)
}

func (s *sourceService) UpdateSource(ctx context.Context, src models.Source) error {
	return s.update.UpdateSource(ctx, src)
}

func (s *sourceService) DeleteSource(ctx context.Context, id models.ID) error {
	return s.del.DeleteSource(ctx, id)
}

// citationService — фасад всех сценариев цитат, отдаваемых HTTP и MCP.
type citationService struct {
	list   *list_citations.Scenario
	search *search_citations.Scenario
	get    *get_citation.Scenario
	create *create_citation.Scenario
	update *update_citation.Scenario
	del    *delete_citation.Scenario
}

func (s *citationService) ListCitations(ctx context.Context, access models.Access, page models.Page) ([]models.Citation, error) {
	return s.list.ListCitations(ctx, access, page)
}

func (s *citationService) SearchCitations(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Citation, error) {
	return s.search.SearchCitations(ctx, access, q)
}

func (s *citationService) GetCitation(ctx context.Context, access models.Access, id models.ID) (models.Citation, error) {
	return s.get.GetCitation(ctx, access, id)
}

func (s *citationService) CreateCitation(ctx context.Context, c models.Citation) (models.Citation, error) {
	return s.create.CreateCitation(ctx, c)
}

func (s *citationService) UpdateCitation(ctx context.Context, c models.Citation) error {
	return s.update.UpdateCitation(ctx, c)
}

func (s *citationService) DeleteCitation(ctx context.Context, id models.ID) error {
	return s.del.DeleteCitation(ctx, id)
}

var (
	_ httpapi.DivisionService        = (*divisionService)(nil)
	_ mcp.DivisionService            = (*divisionService)(nil)
	_ httpapi.SurnameService         = (*surnameService)(nil)
	_ mcp.SurnameService             = (*surnameService)(nil)
	_ httpapi.PatronymicService      = (*patronymicService)(nil)
	_ mcp.PatronymicService          = (*patronymicService)(nil)
	_ httpapi.EstateService          = (*estateService)(nil)
	_ mcp.EstateService              = (*estateService)(nil)
	_ httpapi.TitleService           = (*titleService)(nil)
	_ mcp.TitleService               = (*titleService)(nil)
	_ httpapi.GivenNameService       = (*givenNameService)(nil)
	_ mcp.GivenNameService           = (*givenNameService)(nil)
	_ httpapi.RepositoryService      = (*repositoryService)(nil)
	_ mcp.RepositoryService          = (*repositoryService)(nil)
	_ httpapi.ChurchService          = (*churchService)(nil)
	_ mcp.ChurchService              = (*churchService)(nil)
	_ httpapi.ParishService          = (*parishService)(nil)
	_ mcp.ParishService              = (*parishService)(nil)
	_ httpapi.ArchiveService         = (*archiveService)(nil)
	_ mcp.ArchiveService             = (*archiveService)(nil)
	_ httpapi.ArchiveNodeService     = (*archiveNodeService)(nil)
	_ mcp.ArchiveNodeService         = (*archiveNodeService)(nil)
	_ httpapi.ArchiveDocumentService = (*archiveDocumentService)(nil)
	_ mcp.ArchiveDocumentService     = (*archiveDocumentService)(nil)
	_ httpapi.NoteService            = (*noteService)(nil)
	_ mcp.NoteService                = (*noteService)(nil)
	_ httpapi.AttachmentService      = (*attachmentService)(nil)
	_ mcp.AttachmentService          = (*attachmentService)(nil)
	_ httpapi.SourceService          = (*sourceService)(nil)
	_ mcp.SourceService              = (*sourceService)(nil)
	_ httpapi.CitationService        = (*citationService)(nil)
	_ mcp.CitationService            = (*citationService)(nil)
	_ httpapi.AuthService            = (*auth.Service)(nil)
	_ mcp.TokenResolver              = (*auth.Service)(nil)
)

// New собирает приложение: хранилище → сценарии/auth → MCP/HTTP интерфейсы.
func New(cfg Config) (*App, error) {
	st, err := sqlstore.Open(cfg.DataDir)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}

	divisions := &divisionService{
		list:   list_divisions.New(st),
		search: search_divisions.New(st),
		get:    get_division.New(st),
		create: create_division.New(st, idgen.New()),
		update: update_division.New(st),
		del:    delete_division.New(st),
	}

	surnames := &surnameService{
		list:   list_surnames.New(st),
		search: search_surnames.New(st),
		get:    get_surname.New(st),
		create: create_surname.New(st, idgen.New()),
		update: update_surname.New(st),
		del:    delete_surname.New(st),
	}

	patronymics := &patronymicService{
		list:   list_patronymics.New(st),
		search: search_patronymics.New(st),
		get:    get_patronymic.New(st),
		create: create_patronymic.New(st, idgen.New()),
		update: update_patronymic.New(st),
		del:    delete_patronymic.New(st),
	}

	estates := &estateService{
		list:   list_estates.New(st),
		search: search_estates.New(st),
		get:    get_estate.New(st),
		create: create_estate.New(st, idgen.New()),
		update: update_estate.New(st),
		del:    delete_estate.New(st),
	}

	titles := &titleService{
		list:   list_titles.New(st),
		search: search_titles.New(st),
		get:    get_title.New(st),
		create: create_title.New(st, idgen.New()),
		update: update_title.New(st),
		del:    delete_title.New(st),
	}

	givenNames := &givenNameService{
		list:   list_given_names.New(st),
		search: search_given_names.New(st),
		get:    get_given_name.New(st),
		create: create_given_name.New(st, idgen.New()),
		update: update_given_name.New(st),
		del:    delete_given_name.New(st),
	}

	repositories := &repositoryService{
		list:   list_repositories.New(st),
		search: search_repositories.New(st),
		get:    get_repository.New(st),
		create: create_repository.New(st, idgen.New()),
		update: update_repository.New(st),
		del:    delete_repository.New(st),
	}

	churches := &churchService{
		list:   list_churches.New(st),
		search: search_churches.New(st),
		get:    get_church.New(st),
		create: create_church.New(st, idgen.New()),
		update: update_church.New(st),
		del:    delete_church.New(st),
	}

	parishes := &parishService{
		list:   list_parishes.New(st),
		search: search_parishes.New(st),
		get:    get_parish.New(st),
		create: create_parish.New(st, idgen.New()),
		update: update_parish.New(st),
		del:    delete_parish.New(st),
	}

	archives := &archiveService{
		list:   list_archives.New(st),
		search: search_archives.New(st),
		get:    get_archive.New(st),
		create: create_archive.New(st, idgen.New()),
		update: update_archive.New(st),
		del:    delete_archive.New(st),
	}

	archiveNodes := &archiveNodeService{
		list:   list_archive_nodes.New(st),
		search: search_archive_nodes.New(st),
		get:    get_archive_node.New(st),
		create: create_archive_node.New(st, idgen.New()),
		update: update_archive_node.New(st),
		del:    delete_archive_node.New(st),
	}

	archiveDocs := &archiveDocumentService{
		list:   list_archive_documents.New(st),
		search: search_archive_documents.New(st),
		get:    get_archive_document.New(st),
		create: create_archive_document.New(st, idgen.New()),
		update: update_archive_document.New(st),
		del:    delete_archive_document.New(st),
	}

	notes := &noteService{
		list:   list_notes.New(st),
		search: search_notes.New(st),
		get:    get_note.New(st),
		create: create_note.New(st, idgen.New()),
		update: update_note.New(st),
		del:    delete_note.New(st),
	}

	attachments := &attachmentService{
		list:   list_attachments.New(st),
		search: search_attachments.New(st),
		get:    get_attachment.New(st),
		create: create_attachment.New(st, idgen.New()),
		update: update_attachment.New(st),
		del:    delete_attachment.New(st),
	}

	sources := &sourceService{
		list:   list_sources.New(st),
		search: search_sources.New(st),
		get:    get_source.New(st),
		create: create_source.New(st, idgen.New()),
		update: update_source.New(st),
		del:    delete_source.New(st),
	}

	citations := &citationService{
		list:   list_citations.New(st),
		search: search_citations.New(st),
		get:    get_citation.New(st),
		create: create_citation.New(st, idgen.New()),
		update: update_citation.New(st),
		del:    delete_citation.New(st),
	}

	// auth-хранилище — на том же соединении, что и общий store (см.
	// sqlstore.Store.DB), файл БД один и тот же (internal/storage/schema_auth.go).
	authService := auth.New(auth.NewSQLStore(st.DB()))

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcp.RequireAPIToken(authService)(server.NewStreamableHTTPServer(
		mcp.NewServer(mcp.Deps{
			Divisions: divisions, Surnames: surnames,
			Patronymics: patronymics, Estates: estates, Titles: titles, GivenNames: givenNames,
			Repositories: repositories, Churches: churches, Parishes: parishes, Archives: archives,
			ArchiveNodes: archiveNodes,
			ArchiveDocs:  archiveDocs,
			Notes:        notes,
			Attachments:  attachments,
			Sources:      sources,
			Citations:    citations,
		}),
	)))
	mux.Handle("/api/", httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:    divisions,
		Surnames:     surnames,
		Patronymics:  patronymics,
		Estates:      estates,
		Titles:       titles,
		GivenNames:   givenNames,
		Repositories: repositories,
		Churches:     churches,
		Parishes:     parishes,
		Archives:     archives,
		ArchiveNodes: archiveNodes,
		ArchiveDocs:  archiveDocs,
		Notes:        notes,
		Attachments:  attachments,
		Sources:      sources,
		Citations:    citations,
		Auth:         authService,
		DocsFS:       genodex.DocsFS(cfg.WebMode),
		TrustProxy:   cfg.TrustProxy,
	}))
	mux.Handle("/static/", http.StripPrefix("/static/", web.StaticHandler(cfg.WebMode)))
	mux.Handle("/", web.SPAHandler(cfg.WebMode))

	return &App{
		cfg: cfg,
		http: &http.Server{
			Addr:    fmt.Sprintf("0.0.0.0:%d", cfg.Port),
			Handler: mux,
		},
		store: st,
	}, nil
}

// Run запускает HTTP-сервер. Останавливается по отмене ctx (graceful shutdown
// HTTP и закрытие хранилища) либо по ошибке ListenAndServe.
func (a *App) Run(ctx context.Context) error {
	log.Printf("Genealogy MCP server started on %s", a.http.Addr)
	log.Printf("  MCP:      http://localhost:%d/mcp", a.cfg.Port)
	log.Printf("  API:      http://localhost:%d/api", a.cfg.Port)
	log.Printf("  Web:      http://localhost:%d/ (web mode: %s)", a.cfg.Port, a.cfg.WebMode)
	log.Printf("  Trust proxy (X-Forwarded-Proto): %v", a.cfg.TrustProxy)

	go func() {
		<-ctx.Done()
		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.http.Shutdown(shCtx); err != nil {
			log.Printf("Graceful shutdown error: %v", err)
		}
		if err := a.store.Close(); err != nil {
			log.Printf("Store close error: %v", err)
		}
	}()

	if err := a.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
```

### 1.5. Real-store интеграционные тесты

#### `internal/httpapi/write_store_test.go` (изменить — итоговое содержимое)
`internal/httpapi/write_store_test.go`:
```go
package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/auth"
	"github.com/amarin/genodex/internal/httpapi"
	"github.com/amarin/genodex/internal/idgen"
	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store/sqlstore"
	"github.com/amarin/genodex/internal/transport"
)

// TestDivisionWriteContractWithRealStore: сквозной путь «хранилище → сценарии
// → HTTP» (так же собран internal/app) для записи делений — через
// NewAPIHandler с реальной сессией владельца: регистрация (bootstrap) →
// создание/чтение/изменение/удаление → 409 со списком ссылающихся →
// анонимная попытка записи — 401 (auth.md §6, приёмка этапа C).
func TestDivisionWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Sources:    newSourceService(t, st),
		Citations:  newCitationService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание корня.
	root := createDivision(t, h, owner, `{"name":"Московская","type":"governorate"}`, http.StatusCreated)
	if root.Name != "Московская" || root.Type != models.AdminDivisionGovernorate || root.ParentID != nil {
		t.Fatalf("root = %+v", root)
	}
	if !strings.HasPrefix(string(root.ID), "AD-") {
		t.Fatalf("id = %q", root.ID)
	}

	// Создание дочерней.
	child := createDivision(t, h, owner,
		fmt.Sprintf(`{"name":"Давыдово","type":"selo","parent_id":%q}`, root.ID), http.StatusCreated)
	if child.ParentID == nil || *child.ParentID != root.ID {
		t.Fatalf("child = %+v", child)
	}

	// Чтение по id — открыто анонимному посетителю.
	rec := getReq(t, h, "/api/admin-divisions/"+string(child.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"name":"Давыдово"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена полей name/type/parent_id.
	rec = putReq(t, h, owner, "/api/admin-divisions/"+string(child.ID),
		fmt.Sprintf(`{"name":"Давыдова","type":"selo","parent_id":%q}`, root.ID))
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeDivisionS(t, rec)
	if updated.Name != "Давыдова" || updated.ID != child.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Родитель занят дочерью → 409 со списком ссылающихся.
	rec = delReq(t, h, owner, "/api/admin-divisions/"+string(root.ID))
	requireStatusS(t, rec, http.StatusConflict)
	if !strings.Contains(rec.Body.String(), `"referrers"`) || !strings.Contains(rec.Body.String(), string(child.ID)) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Удаление дочерней, затем корня.
	requireStatusS(t, delReq(t, h, owner, "/api/admin-divisions/"+string(child.ID)), http.StatusNoContent)
	requireStatusS(t, delReq(t, h, owner, "/api/admin-divisions/"+string(root.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/admin-divisions/"+string(root.ID)), http.StatusNotFound)

	// Неверный формат id в пути — 422 (не 404).
	requireStatusS(t, getReq(t, h, "/api/admin-divisions/obvious-bad"), http.StatusUnprocessableEntity)

	// Несуществующий родитель (id валидного формата) — 422 с полем parent_id.
	rec = postReq(t, h, owner, `{"name":"Давыдово","type":"selo","parent_id":"AD-01ARZ3NDEKTSV4RRFFQ69G5FA9"}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"parent_id"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Ретрофит: строгий FK sources[i].citation_id — сквозная проверка через
	// реальный HTTP-хендлер → сценарий → SQLite (по образцу
	// TestArchiveWriteContractWithRealStore; Division — единственная сущность,
	// у которой Sources пришлось добавлять в read/write-контракт с нуля, а не
	// только в usecase, см. docs/data-model/entity-write.md §3.3).
	src := createSource(t, h, owner,
		`{"kind":"document","title":"Ревизская сказка","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	cit := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)

	withCitation := createDivision(t, h, owner,
		fmt.Sprintf(`{"name":"Гавриловское","type":"volost","sources":[{"citation_id":%q}]}`, cit.ID),
		http.StatusCreated)
	if len(withCitation.Sources) != 1 || withCitation.Sources[0].CitationID != string(cit.ID) {
		t.Fatalf("withCitation.Sources = %+v, want [{citation_id: %q}]", withCitation.Sources, cit.ID)
	}

	// Строгий FK: корректный по формату, но несуществующий citation_id — 422 на sources[0].citation_id.
	rec = postReq(t, h, owner,
		`{"name":"Призрачная волость","type":"volost","sources":[{"citation_id":"C-01ARZ3NDEKTSV4RRFFQ69G5FA9"}]}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"sources[0].citation_id"`) {
		t.Fatalf("body = %s, want field=sources[0].citation_id", rec.Body)
	}

	// Logout инвалидирует сессию: та же cookie больше не проходит requireFull.
	logoutRec := postAuthReq(t, h, "/api/auth/logout", "", owner)
	requireStatusS(t, logoutRec, http.StatusNoContent)

	staleRec := postReq(t, h, owner, `{"name":"После логаута","type":"selo"}`)
	requireStatusS(t, staleRec, http.StatusUnauthorized)

	// Анонимная попытка создать — 401, до разбора тела.
	anonRec := postReq(t, h, nil, `{"name":"Аноним","type":"selo"}`)
	requireStatusS(t, anonRec, http.StatusUnauthorized)
}

// TestSurnameWriteContractWithRealStore: сквозной путь «хранилище → сценарии
// → HTTP» (так же собран internal/app) для записи словарных записей фамилий
// — через NewAPIHandler с реальной сессией владельца: bootstrap-регистрация
// → создание → чтение → изменение → удаление → повторное чтение — 404
// (по образцу TestDivisionWriteContractWithRealStore, но без 409-сценария:
// у Surname нет строгих внешних ключей, docs/data-model/entity-write.md).
func TestSurnameWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Surnames:   newSurnameService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание записи.
	created := createSurname(t, h, owner,
		`{"canonical":"Иванов","variants":[{"text":"Иванова"}],"items":[],"notes":[]}`, http.StatusCreated)
	if created.Canonical != "Иванов" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "SN-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id — открыто анонимному посетителю.
	rec := getReq(t, h, "/api/surnames/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"canonical":"Иванов"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена canonical/variants/items/notes.
	rec = putSurnameReq(t, h, owner, "/api/surnames/"+string(created.ID),
		`{"canonical":"Иванова","variants":[],"items":[],"notes":[]}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeSurnameS(t, rec)
	if updated.Canonical != "Иванова" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление — у Surname нет строгих FK, конфликта не бывает.
	requireStatusS(t, delReq(t, h, owner, "/api/surnames/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/surnames/"+string(created.ID)), http.StatusNotFound)
}

func createSurname(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Surname {
	t.Helper()

	rec := postSurnameReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeSurnameS(t, rec)
}

func decodeSurnameS(t *testing.T, rec *httptest.ResponseRecorder) transport.Surname {
	t.Helper()

	var s transport.Surname
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return s
}

func postSurnameReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/surnames", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putSurnameReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

// TestPatronymicWriteContractWithRealStore: сквозной путь «хранилище →
// сценарии → HTTP» (так же собран internal/app) для записи словарных
// записей отчеств — через NewAPIHandler с реальной сессией владельца:
// bootstrap-регистрация → создание → чтение → изменение → удаление →
// повторное чтение — 404 (по образцу TestSurnameWriteContractWithRealStore,
// без 409-сценария: у Patronymic нет строгих внешних ключей,
// docs/data-model/entity-write.md).
func TestPatronymicWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:   newDivisionService(t, st),
		Patronymics: newPatronymicService(t, st),
		Auth:        authSvc,
		DocsFS:      fstest.MapFS{},
		TrustProxy:  false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание записи.
	created := createPatronymic(t, h, owner,
		`{"canonical":"Иванович","variants":[{"text":"Иванычъ"}],"items":[],"notes":[]}`, http.StatusCreated)
	if created.Canonical != "Иванович" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "PN-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id — открыто анонимному посетителю.
	rec := getReq(t, h, "/api/patronymics/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"canonical":"Иванович"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена canonical/variants/items/notes.
	rec = putPatronymicReq(t, h, owner, "/api/patronymics/"+string(created.ID),
		`{"canonical":"Ивановна","variants":[],"items":[],"notes":[]}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodePatronymicS(t, rec)
	if updated.Canonical != "Ивановна" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление — у Patronymic нет строгих FK, конфликта не бывает.
	requireStatusS(t, delReq(t, h, owner, "/api/patronymics/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/patronymics/"+string(created.ID)), http.StatusNotFound)
}

func createPatronymic(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Patronymic {
	t.Helper()

	rec := postPatronymicReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodePatronymicS(t, rec)
}

func decodePatronymicS(t *testing.T, rec *httptest.ResponseRecorder) transport.Patronymic {
	t.Helper()

	var s transport.Patronymic
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return s
}

func postPatronymicReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/patronymics", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putPatronymicReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

// TestEstateWriteContractWithRealStore: сквозной путь «хранилище → сценарии
// → HTTP» (так же собран internal/app) для записи словарных записей
// сословий — через NewAPIHandler с реальной сессией владельца:
// bootstrap-регистрация → создание → чтение → изменение → удаление →
// повторное чтение — 404 (по образцу TestSurnameWriteContractWithRealStore,
// без 409-сценария: у Estate нет строгих внешних ключей,
// docs/data-model/entity-write.md).
func TestEstateWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Estates:    newEstateService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание записи.
	created := createEstate(t, h, owner,
		`{"canonical":"крестьяне","variants":[{"text":"крестьянство"}],"items":[],"notes":[]}`, http.StatusCreated)
	if created.Canonical != "крестьяне" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "ES-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id — открыто анонимному посетителю.
	rec := getReq(t, h, "/api/estates/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"canonical":"крестьяне"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена canonical/variants/items/notes.
	rec = putEstateReq(t, h, owner, "/api/estates/"+string(created.ID),
		`{"canonical":"мещане","variants":[],"items":[],"notes":[]}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeEstateS(t, rec)
	if updated.Canonical != "мещане" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление — у Estate нет строгих FK, конфликта не бывает.
	requireStatusS(t, delReq(t, h, owner, "/api/estates/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/estates/"+string(created.ID)), http.StatusNotFound)
}

func createEstate(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Estate {
	t.Helper()

	rec := postEstateReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeEstateS(t, rec)
}

func decodeEstateS(t *testing.T, rec *httptest.ResponseRecorder) transport.Estate {
	t.Helper()

	var s transport.Estate
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return s
}

func postEstateReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/estates", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putEstateReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

// TestTitleWriteContractWithRealStore: сквозной путь «хранилище → сценарии
// → HTTP» (так же собран internal/app) для записи словарных записей
// титулов — через NewAPIHandler с реальной сессией владельца:
// bootstrap-регистрация → создание → чтение → изменение → удаление →
// повторное чтение — 404 (по образцу TestSurnameWriteContractWithRealStore,
// без 409-сценария: у Title нет строгих внешних ключей,
// docs/data-model/entity-write.md).
func TestTitleWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Titles:     newTitleService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание записи.
	created := createTitle(t, h, owner,
		`{"canonical":"вдова","variants":[{"text":"вдовица"}],"items":[],"notes":[]}`, http.StatusCreated)
	if created.Canonical != "вдова" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "TT-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id — открыто анонимному посетителю.
	rec := getReq(t, h, "/api/titles/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"canonical":"вдова"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена canonical/variants/items/notes.
	rec = putTitleReq(t, h, owner, "/api/titles/"+string(created.ID),
		`{"canonical":"вдовец","variants":[],"items":[],"notes":[]}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeTitleS(t, rec)
	if updated.Canonical != "вдовец" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление — у Title нет строгих FK, конфликта не бывает.
	requireStatusS(t, delReq(t, h, owner, "/api/titles/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/titles/"+string(created.ID)), http.StatusNotFound)
}

func createTitle(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Title {
	t.Helper()

	rec := postTitleReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeTitleS(t, rec)
}

func decodeTitleS(t *testing.T, rec *httptest.ResponseRecorder) transport.Title {
	t.Helper()

	var s transport.Title
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return s
}

func postTitleReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/titles", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putTitleReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

// TestGivenNameWriteContractWithRealStore: сквозной путь «хранилище →
// сценарии → HTTP» (так же собран internal/app) для записи словарных
// записей имён — через NewAPIHandler с реальной сессией владельца:
// bootstrap-регистрация → создание → чтение → изменение → удаление →
// повторное чтение — 404 (по образцу TestSurnameWriteContractWithRealStore,
// без 409-сценария: у GivenName нет строгих внешних ключей,
// docs/data-model/entity-write.md). Дополнительно проверяет поле gender —
// единственное отличие GivenName от остальных трёх словарных сущностей.
func TestGivenNameWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		GivenNames: newGivenNameService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание записи с полом male.
	created := createGivenName(t, h, owner,
		`{"canonical":"Иван","gender":"male","variants":[{"text":"Иоанн"}],"items":[],"notes":[]}`, http.StatusCreated)
	if created.Canonical != "Иван" {
		t.Fatalf("created = %+v", created)
	}
	if created.Gender != models.NameGenderMale {
		t.Fatalf("created.Gender = %q, want male", created.Gender)
	}
	if !strings.HasPrefix(string(created.ID), "GN-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id — открыто анонимному посетителю, gender виден в ответе.
	rec := getReq(t, h, "/api/given-names/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"canonical":"Иван"`) || !strings.Contains(rec.Body.String(), `"gender":"male"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена canonical/gender/variants/items/notes, gender → neutral.
	rec = putGivenNameReq(t, h, owner, "/api/given-names/"+string(created.ID),
		`{"canonical":"Саша","gender":"neutral","variants":[],"items":[],"notes":[]}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeGivenNameS(t, rec)
	if updated.Canonical != "Саша" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}
	if updated.Gender != models.NameGenderNeutral {
		t.Fatalf("updated.Gender = %q, want neutral", updated.Gender)
	}

	// Удаление — у GivenName нет строгих FK, конфликта не бывает.
	requireStatusS(t, delReq(t, h, owner, "/api/given-names/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/given-names/"+string(created.ID)), http.StatusNotFound)
}

func createGivenName(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.GivenName {
	t.Helper()

	rec := postGivenNameReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeGivenNameS(t, rec)
}

func decodeGivenNameS(t *testing.T, rec *httptest.ResponseRecorder) transport.GivenName {
	t.Helper()

	var s transport.GivenName
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return s
}

func postGivenNameReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/given-names", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putGivenNameReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

// TestDivisionCreateWithoutCSRFHeaderIs400: валидная сессия владельца, но без
// X-Requested-With — 400 (requireCSRFHeader), сценарий не вызывается. Пин на
// то, что NewAPIHandler реально оборачивает division-записи, не только auth.
func TestDivisionCreateWithoutCSRFHeaderIs400(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{Divisions: newDivisionService(t, st), Auth: authSvc, DocsFS: fstest.MapFS{}, TrustProxy: false})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)

	req := httptest.NewRequest(http.MethodPost, "/api/admin-divisions", strings.NewReader(`{"name":"x","type":"selo"}`))
	req.AddCookie(accessCookie)
	// нарочно без X-Requested-With

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	requireStatusS(t, rec, http.StatusBadRequest)
}

func createDivision(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.AdminDivision {
	t.Helper()

	rec := postReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeDivisionS(t, rec)
}

func decodeDivisionS(t *testing.T, rec *httptest.ResponseRecorder) transport.AdminDivision {
	t.Helper()

	var d transport.AdminDivision
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return d
}

func postReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/admin-divisions", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func getReq(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	return rec
}

func putReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func delReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodDelete, path, nil)
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func requireStatusS(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()

	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}
}

// TestRepositoryWriteContractWithRealStore: сквозной путь «хранилище →
// сценарии → HTTP» (по образцу TestGivenNameWriteContractWithRealStore) для
// хранилищ-контейнеров источников: bootstrap-регистрация → создание →
// чтение → изменение → удаление → повторное чтение — 404. Дополнительно
// закрывает Fix 1 (CRITICAL): приватная запись, созданная владельцем, должна
// быть недоступна анонимному GET /api/repositories/{id} — 404, а не 200.
func TestRepositoryWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:    newDivisionService(t, st),
		Repositories: newRepositoryService(t, st),
		Auth:         authSvc,
		DocsFS:       fstest.MapFS{},
		TrustProxy:   false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание.
	created := createRepository(t, h, owner,
		`{"name":"ГАВО","type":"archive","address":"Вологда","urls":[],"notes":[],"private":false}`, http.StatusCreated)
	if created.Name != "ГАВО" || created.Type != "archive" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "R-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id — открыто анонимному посетителю.
	rec := getReq(t, h, "/api/repositories/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"name":"ГАВО"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена name/type/address/urls/notes/private.
	rec = putRepositoryReq(t, h, owner, "/api/repositories/"+string(created.ID),
		`{"name":"ГАВО (испр.)","type":"library","address":"Вологда","urls":[],"notes":[],"private":false}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeRepositoryS(t, rec)
	if updated.Name != "ГАВО (испр.)" || updated.Type != "library" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/repositories/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/repositories/"+string(created.ID)), http.StatusNotFound)

	// Fix 1: приватная запись, анонимный GET — 404, не 200.
	private := createRepository(t, h, owner,
		`{"name":"Частное собрание","type":"private","address":"","urls":[],"notes":[],"private":true}`, http.StatusCreated)
	if !private.Private {
		t.Fatalf("private = %+v, want Private=true", private)
	}

	requireStatusS(t, getReq(t, h, "/api/repositories/"+string(private.ID)), http.StatusNotFound)
}

func createRepository(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Repository {
	t.Helper()

	rec := postRepositoryReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeRepositoryS(t, rec)
}

func decodeRepositoryS(t *testing.T, rec *httptest.ResponseRecorder) transport.Repository {
	t.Helper()

	var r transport.Repository
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return r
}

func postRepositoryReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/repositories", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putRepositoryReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

// TestChurchWriteContractWithRealStore: сквозной путь «хранилище → сценарии →
// HTTP» для церквей (по образцу TestGivenNameWriteContractWithRealStore, без
// приватности — у Church нет поля Private): bootstrap-регистрация →
// создание → чтение → изменение → удаление → повторное чтение — 404.
func TestChurchWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Churches:   newChurchService(t, st),
		Sources:    newSourceService(t, st),
		Citations:  newCitationService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание.
	created := createChurch(t, h, owner,
		`{"name":"Троицкая церковь","settlements":[],"variants":[],"notes":[]}`, http.StatusCreated)
	if created.Name != "Троицкая церковь" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "CH-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id.
	rec := getReq(t, h, "/api/churches/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"name":"Троицкая церковь"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена name/parish/settlements/variants/notes.
	rec = putChurchReq(t, h, owner, "/api/churches/"+string(created.ID),
		`{"name":"Троицкая церковь (испр.)","settlements":[],"variants":[],"notes":[]}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeChurchS(t, rec)
	if updated.Name != "Троицкая церковь (испр.)" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/churches/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/churches/"+string(created.ID)), http.StatusNotFound)

	// Ретрофит: строгий FK sources[i].citation_id — сквозная проверка через
	// реальный HTTP-хендлер → сценарий → SQLite (по образцу
	// TestArchiveWriteContractWithRealStore). Также закрывает create_church's
	// перевод на транзакционный InTx (docs/data-model/entity-write.md §3.3),
	// который иначе не проверяется ни одним real-store тестом.
	src := createSource(t, h, owner,
		`{"kind":"document","title":"Метрическая книга","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	cit := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)

	withCitation := createChurch(t, h, owner,
		fmt.Sprintf(`{"name":"Покровская церковь","settlements":[],"variants":[],"notes":[],"sources":[{"citation_id":%q}]}`, cit.ID),
		http.StatusCreated)
	if len(withCitation.Sources) != 1 || withCitation.Sources[0].CitationID != string(cit.ID) {
		t.Fatalf("withCitation.Sources = %+v, want [{citation_id: %q}]", withCitation.Sources, cit.ID)
	}

	// Строгий FK: корректный по формату, но несуществующий citation_id — 422 на sources[0].citation_id.
	rec = postChurchReq(t, h, owner,
		`{"name":"Церковь-призрак","settlements":[],"variants":[],"notes":[],"sources":[{"citation_id":"C-01ARZ3NDEKTSV4RRFFQ69G5FA9"}]}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"sources[0].citation_id"`) {
		t.Fatalf("body = %s, want field=sources[0].citation_id", rec.Body)
	}
}

func createChurch(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Church {
	t.Helper()

	rec := postChurchReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeChurchS(t, rec)
}

func decodeChurchS(t *testing.T, rec *httptest.ResponseRecorder) transport.Church {
	t.Helper()

	var c transport.Church
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return c
}

func postChurchReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/churches", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putChurchReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

// TestParishWriteContractWithRealStore: сквозной путь «хранилище → сценарии →
// HTTP» для приходов (по образцу TestGivenNameWriteContractWithRealStore, без
// приватности — у Parish нет поля Private): bootstrap-регистрация →
// создание → чтение → изменение → удаление → повторное чтение — 404.
func TestParishWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Parishes:   newParishService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание.
	created := createParish(t, h, owner,
		`{"name":"Троицкий приход","settlements":[],"notes":[]}`, http.StatusCreated)
	if created.Name != "Троицкий приход" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "PR-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id.
	rec := getReq(t, h, "/api/parishes/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"name":"Троицкий приход"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена name/church/settlements/since/until/notes.
	rec = putParishReq(t, h, owner, "/api/parishes/"+string(created.ID),
		`{"name":"Троицкий приход (испр.)","settlements":[],"notes":[]}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeParishS(t, rec)
	if updated.Name != "Троицкий приход (испр.)" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/parishes/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/parishes/"+string(created.ID)), http.StatusNotFound)
}

func createParish(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Parish {
	t.Helper()

	rec := postParishReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeParishS(t, rec)
}

func decodeParishS(t *testing.T, rec *httptest.ResponseRecorder) transport.Parish {
	t.Helper()

	var p transport.Parish
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return p
}

func postParishReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/parishes", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putParishReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

// TestArchiveWriteContractWithRealStore: сквозной путь «хранилище → сценарии
// → HTTP» для архивов (по образцу TestGivenNameWriteContractWithRealStore):
// bootstrap-регистрация → создание → чтение → изменение → удаление →
// повторное чтение — 404. Дополнительно: строгий FK на Repository (валидный
// repository_id сохраняется; несуществующий, но корректный по формату —
// 422 на поле repository_id) и Fix 1 (CRITICAL): приватная запись, анонимный
// GET — 404, не 200.
func TestArchiveWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:    newDivisionService(t, st),
		Repositories: newRepositoryService(t, st),
		Archives:     newArchiveService(t, st),
		Sources:      newSourceService(t, st),
		Citations:    newCitationService(t, st),
		Auth:         authSvc,
		DocsFS:       fstest.MapFS{},
		TrustProxy:   false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание без хранилища.
	created := createArchive(t, h, owner, `{"name":"ГАВО, архив","notes":[],"private":false}`, http.StatusCreated)
	if created.Name != "ГАВО, архив" || created.RepositoryID != "" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "AR-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id.
	rec := getReq(t, h, "/api/archives/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"name":"ГАВО, архив"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена name/system/repository_id/notes/private.
	rec = putArchiveReq(t, h, owner, "/api/archives/"+string(created.ID),
		`{"name":"ГАВО, архив (испр.)","notes":[],"private":false}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeArchiveS(t, rec)
	if updated.Name != "ГАВО, архив (испр.)" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/archives/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/archives/"+string(created.ID)), http.StatusNotFound)

	// Строгий FK: валидный repository_id создаётся и сохраняется как есть.
	repo := createRepository(t, h, owner,
		`{"name":"ГАВО","type":"archive","address":"","urls":[],"notes":[],"private":false}`, http.StatusCreated)

	withRepo := createArchive(t, h, owner,
		fmt.Sprintf(`{"name":"Фонд 1","repository_id":%q,"notes":[],"private":false}`, repo.ID), http.StatusCreated)
	if withRepo.RepositoryID != string(repo.ID) {
		t.Fatalf("withRepo.RepositoryID = %q, want %q", withRepo.RepositoryID, repo.ID)
	}

	// Строгий FK: корректный по формату, но несуществующий repository_id — 422.
	rec = postArchiveReq(t, h, owner,
		`{"name":"Фонд-призрак","repository_id":"R-01ARZ3NDEKTSV4RRFFQ69G5FA9","notes":[],"private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"repository_id"`) {
		t.Fatalf("body = %s, want field=repository_id", rec.Body)
	}

	// Fix 1: приватная запись, анонимный GET — 404, не 200.
	private := createArchive(t, h, owner, `{"name":"Приватный архив","notes":[],"private":true}`, http.StatusCreated)
	if !private.Private {
		t.Fatalf("private = %+v, want Private=true", private)
	}

	requireStatusS(t, getReq(t, h, "/api/archives/"+string(private.ID)), http.StatusNotFound)

	// Ретрофит: строгий FK sources[i].citation_id — сквозная проверка через
	// реальный HTTP-хендлер → сценарий → SQLite, а не только на fake-store.
	src := createSource(t, h, owner,
		`{"kind":"document","title":"Метрическая книга","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	cit := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)

	withCitation := createArchive(t, h, owner,
		fmt.Sprintf(`{"name":"Фонд с цитатой","sources":[{"citation_id":%q}],"notes":[],"private":false}`, cit.ID),
		http.StatusCreated)
	if len(withCitation.Sources) != 1 || withCitation.Sources[0].CitationID != string(cit.ID) {
		t.Fatalf("withCitation.Sources = %+v, want [{citation_id: %q}]", withCitation.Sources, cit.ID)
	}

	// Строгий FK: корректный по формату, но несуществующий citation_id — 422 на sources[0].citation_id.
	rec = postArchiveReq(t, h, owner,
		`{"name":"Фонд-призрак 2","sources":[{"citation_id":"C-01ARZ3NDEKTSV4RRFFQ69G5FA9"}],"notes":[],"private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"sources[0].citation_id"`) {
		t.Fatalf("body = %s, want field=sources[0].citation_id", rec.Body)
	}
}

func createArchive(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Archive {
	t.Helper()

	rec := postArchiveReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeArchiveS(t, rec)
}

func decodeArchiveS(t *testing.T, rec *httptest.ResponseRecorder) transport.Archive {
	t.Helper()

	var a transport.Archive
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return a
}

func postArchiveReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/archives", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putArchiveReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

// TestArchiveNodeWriteContractWithRealStore: сквозной путь «хранилище →
// сценарии → HTTP» (по образцу TestArchiveWriteContractWithRealStore) для
// узлов архивного дерева: bootstrap-регистрация → создание архива → создание
// корневого узла → создание дочернего узла → список с фильтром по
// archive_id (оба узла видны, корректно вложены) → изменение → удаление →
// повторное чтение — 404. Дополнительно закрывает новый для этого
// подпункта инвариант «родитель из того же архива»: несуществующий
// parent_id — 422 на поле parent_id; parent_id, указывающий на реальный
// узел, но из ДРУГОГО архива — тоже 422 на поле parent_id (не archive_id).
func TestArchiveNodeWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:    newDivisionService(t, st),
		Archives:     newArchiveService(t, st),
		ArchiveNodes: newArchiveNodeService(t, st),
		Sources:      newSourceService(t, st),
		Citations:    newCitationService(t, st),
		Auth:         authSvc,
		DocsFS:       fstest.MapFS{},
		TrustProxy:   false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	archive := createArchive(t, h, owner, `{"name":"ГАВО, архив","notes":[],"private":false}`, http.StatusCreated)
	otherArchive := createArchive(t, h, owner, `{"name":"РГАДА","notes":[],"private":false}`, http.StatusCreated)

	// Корневой узел (без parent_id).
	root := createArchiveNode(t, h, owner,
		fmt.Sprintf(`{"type":"fond","archive_id":%q,"label":"Фонд 1"}`, archive.ID), http.StatusCreated)
	if root.ArchiveID != archive.ID || root.ParentID != nil {
		t.Fatalf("root = %+v", root)
	}
	if !strings.HasPrefix(string(root.ID), "AN-") {
		t.Fatalf("id = %q", root.ID)
	}

	// Дочерний узел под корневым, в том же архиве.
	child := createArchiveNode(t, h, owner,
		fmt.Sprintf(`{"type":"opis","archive_id":%q,"parent_id":%q,"label":"Опись 1"}`, archive.ID, root.ID), http.StatusCreated)
	if child.ParentID == nil || *child.ParentID != root.ID {
		t.Fatalf("child = %+v", child)
	}

	// Список с фильтром по archive_id, без parent_id — только корень
	// (models.ArchiveNodeQuery: ParentID nil — корень внутри архива).
	rec := getReq(t, h, "/api/archive-nodes?archive_id="+string(archive.ID))
	requireStatusS(t, rec, http.StatusOK)
	rootList := decodeArchiveNodeListS(t, rec)
	if len(rootList) != 1 || rootList[0].ID != root.ID {
		t.Fatalf("rootList = %+v, want exactly [root]", rootList)
	}

	// Список с parent_id=root — только прямые дети (child).
	rec = getReq(t, h, "/api/archive-nodes?archive_id="+string(archive.ID)+"&parent_id="+string(root.ID))
	requireStatusS(t, rec, http.StatusOK)
	childList := decodeArchiveNodeListS(t, rec)
	if len(childList) != 1 || childList[0].ID != child.ID {
		t.Fatalf("childList = %+v, want exactly [child]", childList)
	}

	// Список без archive_id — 400.
	requireStatusS(t, getReq(t, h, "/api/archive-nodes"), http.StatusBadRequest)

	// Изменение: полная замена label.
	rec = putArchiveNodeReq(t, h, owner, "/api/archive-nodes/"+string(root.ID),
		fmt.Sprintf(`{"type":"fond","archive_id":%q,"label":"Фонд 1 (испр.)"}`, archive.ID))
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeArchiveNodeS(t, rec)
	if updated.Label != "Фонд 1 (испр.)" || updated.ID != root.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Несуществующий parent_id — 422 на поле parent_id.
	rec = postArchiveNodeReq(t, h, owner,
		fmt.Sprintf(`{"type":"delo","archive_id":%q,"parent_id":"AN-01ARZ3NDEKTSV4RRFFQ69G5FA9","label":"Дело-призрак"}`, archive.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"parent_id"`) {
		t.Fatalf("body = %s, want field=parent_id", rec.Body)
	}

	// parent_id из ДРУГОГО архива — тоже 422 на поле parent_id (не archive_id):
	// сам родитель существует, инвариант — «родитель из того же архива».
	rec = postArchiveNodeReq(t, h, owner,
		fmt.Sprintf(`{"type":"delo","archive_id":%q,"parent_id":%q,"label":"Дело из чужого архива"}`, otherArchive.ID, root.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"parent_id"`) {
		t.Fatalf("body = %s, want field=parent_id (родитель из другого архива)", rec.Body)
	}

	// Ретрофит: строгий FK sources[i].citation_id.
	src := createSource(t, h, owner,
		`{"kind":"document","title":"Опись фонда","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	cit := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)

	withCitation := createArchiveNode(t, h, owner,
		fmt.Sprintf(`{"type":"fond","archive_id":%q,"label":"Фонд с цитатой","sources":[{"citation_id":%q}]}`, archive.ID, cit.ID),
		http.StatusCreated)
	if len(withCitation.Sources) != 1 || withCitation.Sources[0].CitationID != string(cit.ID) {
		t.Fatalf("withCitation.Sources = %+v, want [{citation_id: %q}]", withCitation.Sources, cit.ID)
	}

	rec = postArchiveNodeReq(t, h, owner,
		fmt.Sprintf(`{"type":"fond","archive_id":%q,"label":"Фонд-призрак","sources":[{"citation_id":"C-01ARZ3NDEKTSV4RRFFQ69G5FA9"}]}`, archive.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"sources[0].citation_id"`) {
		t.Fatalf("body = %s, want field=sources[0].citation_id", rec.Body)
	}

	// Удаление корня, занятого дочерним узлом — 409.
	requireStatusS(t, delReq(t, h, owner, "/api/archive-nodes/"+string(root.ID)), http.StatusConflict)

	// Удаление дочернего узла, затем корня — оба 204.
	requireStatusS(t, delReq(t, h, owner, "/api/archive-nodes/"+string(child.ID)), http.StatusNoContent)
	requireStatusS(t, delReq(t, h, owner, "/api/archive-nodes/"+string(root.ID)), http.StatusNoContent)

	requireStatusS(t, getReq(t, h, "/api/archive-nodes/"+string(root.ID)), http.StatusNotFound)
}

func createArchiveNode(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.ArchiveNode {
	t.Helper()

	rec := postArchiveNodeReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeArchiveNodeS(t, rec)
}

func decodeArchiveNodeS(t *testing.T, rec *httptest.ResponseRecorder) transport.ArchiveNode {
	t.Helper()

	var n transport.ArchiveNode
	if err := json.Unmarshal(rec.Body.Bytes(), &n); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return n
}

func decodeArchiveNodeListS(t *testing.T, rec *httptest.ResponseRecorder) []transport.ArchiveNode {
	t.Helper()

	var list []transport.ArchiveNode
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return list
}

func postArchiveNodeReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/archive-nodes", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putArchiveNodeReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

// TestArchiveDocumentWriteContractWithRealStore: сквозной путь «хранилище →
// сценарии → HTTP» для документов внутри единиц учёта: bootstrap-регистрация
// → создание архива → создание узла → создание документа под этим узлом →
// чтение → изменение → удаление → повторное чтение — 404. Дополнительно
// закрывает строгий FK unit_id (несуществующий — 422 на поле unit_id) и
// sources[i].citation_id (несуществующий — 422).
func TestArchiveDocumentWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:    newDivisionService(t, st),
		Archives:     newArchiveService(t, st),
		ArchiveNodes: newArchiveNodeService(t, st),
		ArchiveDocs:  newArchiveDocumentService(t, st),
		Sources:      newSourceService(t, st),
		Citations:    newCitationService(t, st),
		Auth:         authSvc,
		DocsFS:       fstest.MapFS{},
		TrustProxy:   false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	archive := createArchive(t, h, owner, `{"name":"ГАВО, архив","notes":[],"private":false}`, http.StatusCreated)
	unit := createArchiveNode(t, h, owner,
		fmt.Sprintf(`{"type":"delo","archive_id":%q,"label":"Дело 1"}`, archive.ID), http.StatusCreated)

	created := createArchiveDocument(t, h, owner,
		fmt.Sprintf(`{"unit_id":%q,"title":"Метрическая книга 1890"}`, unit.ID), http.StatusCreated)
	if created.UnitID != unit.ID || created.Title != "Метрическая книга 1890" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "DC-") {
		t.Fatalf("id = %q", created.ID)
	}

	rec := getReq(t, h, "/api/archive-documents/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"title":"Метрическая книга 1890"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	rec = putArchiveDocumentReq(t, h, owner, "/api/archive-documents/"+string(created.ID),
		fmt.Sprintf(`{"unit_id":%q,"title":"Метрическая книга 1890 (испр.)"}`, unit.ID))
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeArchiveDocumentS(t, rec)
	if updated.Title != "Метрическая книга 1890 (испр.)" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	requireStatusS(t, delReq(t, h, owner, "/api/archive-documents/"+string(created.ID)), http.StatusNoContent)
	requireStatusS(t, getReq(t, h, "/api/archive-documents/"+string(created.ID)), http.StatusNotFound)

	// Строгий FK: несуществующий unit_id — 422 на поле unit_id.
	rec = postArchiveDocumentReq(t, h, owner, `{"unit_id":"AN-01ARZ3NDEKTSV4RRFFQ69G5FA9","title":"Документ-призрак"}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"unit_id"`) {
		t.Fatalf("body = %s, want field=unit_id", rec.Body)
	}

	// Ретрофит: строгий FK sources[i].citation_id.
	src := createSource(t, h, owner,
		`{"kind":"document","title":"Метрическая книга","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	cit := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)

	withCitation := createArchiveDocument(t, h, owner,
		fmt.Sprintf(`{"unit_id":%q,"title":"Документ с цитатой","sources":[{"citation_id":%q}]}`, unit.ID, cit.ID),
		http.StatusCreated)
	if len(withCitation.Sources) != 1 || withCitation.Sources[0].CitationID != string(cit.ID) {
		t.Fatalf("withCitation.Sources = %+v, want [{citation_id: %q}]", withCitation.Sources, cit.ID)
	}

	rec = postArchiveDocumentReq(t, h, owner,
		fmt.Sprintf(`{"unit_id":%q,"title":"Документ-призрак 2","sources":[{"citation_id":"C-01ARZ3NDEKTSV4RRFFQ69G5FA9"}]}`, unit.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"sources[0].citation_id"`) {
		t.Fatalf("body = %s, want field=sources[0].citation_id", rec.Body)
	}
}

func createArchiveDocument(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.ArchiveDocument {
	t.Helper()

	rec := postArchiveDocumentReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeArchiveDocumentS(t, rec)
}

func decodeArchiveDocumentS(t *testing.T, rec *httptest.ResponseRecorder) transport.ArchiveDocument {
	t.Helper()

	var d transport.ArchiveDocument
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return d
}

func postArchiveDocumentReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/archive-documents", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putArchiveDocumentReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

// TestNoteWriteContractWithRealStore: сквозной путь «хранилище → сценарии →
// HTTP» (по образцу TestArchiveWriteContractWithRealStore) для заметок:
// bootstrap-регистрация → создание → чтение → изменение → удаление →
// повторное чтение — 404. Дополнительно закрывает self-ref FK
// (Note.ParentID): несуществующий родитель — 422 на поле parent_id;
// переустановка parent_id в цепочку собственных потомков (цикл) — тоже 422
// на поле parent_id; и Fix 1 (приватная запись, анонимный GET — 404).
func TestNoteWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Notes:      newNoteService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание без родителя.
	created := createNote(t, h, owner,
		`{"kind":"note","title":"Заголовок","text":"Текст записи","private":false}`, http.StatusCreated)
	if created.Title != "Заголовок" || created.Text != "Текст записи" || created.ParentID != "" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "N-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id.
	rec := getReq(t, h, "/api/notes/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"text":"Текст записи"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена kind/title/text/parent_id/private.
	rec = putNoteReq(t, h, owner, "/api/notes/"+string(created.ID),
		`{"kind":"article","title":"Заголовок (испр.)","text":"Текст записи","private":false}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeNoteS(t, rec)
	if updated.Kind != "article" || updated.Title != "Заголовок (испр.)" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/notes/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/notes/"+string(created.ID)), http.StatusNotFound)

	// Self-ref FK: валидный parent_id создаётся и сохраняется как есть.
	parent := createNote(t, h, owner, `{"kind":"book","title":"Книга","private":false}`, http.StatusCreated)

	child := createNote(t, h, owner,
		fmt.Sprintf(`{"kind":"chapter","title":"Глава 1","parent_id":%q,"private":false}`, parent.ID), http.StatusCreated)
	if child.ParentID != string(parent.ID) {
		t.Fatalf("child.ParentID = %q, want %q", child.ParentID, parent.ID)
	}

	// Self-ref FK: корректный по формату, но несуществующий parent_id — 422.
	rec = postNoteReq(t, h, owner,
		`{"kind":"note","title":"Сирота","parent_id":"N-01ARZ3NDEKTSV4RRFFQ69G5FA9","private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"parent_id"`) {
		t.Fatalf("body = %s, want field=parent_id", rec.Body)
	}

	// Цикл: A без родителя, B — потомок A, затем A переставляется в потомки B — 422.
	noteA := createNote(t, h, owner, `{"kind":"note","title":"A","private":false}`, http.StatusCreated)
	noteB := createNote(t, h, owner,
		fmt.Sprintf(`{"kind":"note","title":"B","parent_id":%q,"private":false}`, noteA.ID), http.StatusCreated)

	rec = putNoteReq(t, h, owner, "/api/notes/"+string(noteA.ID),
		fmt.Sprintf(`{"kind":"note","title":"A","parent_id":%q,"private":false}`, noteB.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"parent_id"`) {
		t.Fatalf("body = %s, want field=parent_id", rec.Body)
	}

	// Fix 1: приватная запись, анонимный GET — 404, не 200.
	private := createNote(t, h, owner, `{"kind":"note","title":"Приватная","private":true}`, http.StatusCreated)
	if !private.Private {
		t.Fatalf("private = %+v, want Private=true", private)
	}

	requireStatusS(t, getReq(t, h, "/api/notes/"+string(private.ID)), http.StatusNotFound)
}

func createNote(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Note {
	t.Helper()

	rec := postNoteReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeNoteS(t, rec)
}

func decodeNoteS(t *testing.T, rec *httptest.ResponseRecorder) transport.Note {
	t.Helper()

	var n transport.Note
	if err := json.Unmarshal(rec.Body.Bytes(), &n); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return n
}

func postNoteReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/notes", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putNoteReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

// TestAttachmentWriteContractWithRealStore: сквозной путь «хранилище →
// сценарии → HTTP» (по образцу TestArchiveWriteContractWithRealStore) для
// файловых вложений: bootstrap-регистрация → обязательный строгий FK
// node_id (пустой — 422 на поле node_id; корректный по формату, но
// несуществующий — тоже 422 на поле node_id) → Fix 1 (приватная запись,
// анонимный GET — 404). ArchiveNode ещё не имеет своего CRUD-слоя
// (подпроект 6) — сеется напрямую через generic-хранилище вместе с
// Archive, на который он ссылается.
func TestAttachmentWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	ids := idgen.New()
	archiveID := ids.New(models.TypeArchive)
	if err := st.SaveArchive(t.Context(), &models.Archive{ID: archiveID, Name: "ГАВО, архив"}); err != nil {
		t.Fatalf("seed archive: %v", err)
	}

	nodeID := ids.New(models.TypeArchiveNode)
	if err := st.SaveArchiveNode(t.Context(), &models.ArchiveNode{
		ID: nodeID, Type: "fond", ArchiveID: archiveID, Label: "Фонд 1",
	}); err != nil {
		t.Fatalf("seed archive node: %v", err)
	}

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:   newDivisionService(t, st),
		Attachments: newAttachmentService(t, st),
		Auth:        authSvc,
		DocsFS:      fstest.MapFS{},
		TrustProxy:  false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Строгий FK, всегда обязателен: пустой node_id — 422 на поле node_id.
	rec := postAttachmentReq(t, h, owner, `{"kind":"scan","filename":"скан.jpg","private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"node_id"`) {
		t.Fatalf("body = %s, want field=node_id", rec.Body)
	}

	// Строгий FK: корректный по формату, но несуществующий node_id — 422.
	rec = postAttachmentReq(t, h, owner,
		`{"kind":"scan","filename":"скан.jpg","node_id":"AN-01ARZ3NDEKTSV4RRFFQ69G5FA9","private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"node_id"`) {
		t.Fatalf("body = %s, want field=node_id", rec.Body)
	}

	// Fix 1: приватная запись (с настоящим node_id), анонимный GET — 404, не 200.
	private := createAttachment(t, h, owner,
		fmt.Sprintf(`{"kind":"scan","filename":"скан.jpg","node_id":%q,"private":true}`, nodeID), http.StatusCreated)
	if !private.Private {
		t.Fatalf("private = %+v, want Private=true", private)
	}
	if private.NodeID != string(nodeID) {
		t.Fatalf("private.NodeID = %q, want %q", private.NodeID, nodeID)
	}

	requireStatusS(t, getReq(t, h, "/api/attachments/"+string(private.ID)), http.StatusNotFound)
}

func createAttachment(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Attachment {
	t.Helper()

	rec := postAttachmentReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeAttachmentS(t, rec)
}

func decodeAttachmentS(t *testing.T, rec *httptest.ResponseRecorder) transport.Attachment {
	t.Helper()

	var a transport.Attachment
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return a
}

func postAttachmentReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/attachments", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

// TestSourceWriteContractWithRealStore: сквозной путь «хранилище → сценарии
// → HTTP» (по образцу TestArchiveWriteContractWithRealStore) для источников
// доказательств: bootstrap-регистрация → создание без хранилища → чтение →
// изменение → удаление → повторное чтение — 404. Дополнительно: строгий FK
// на Repository (валидный repository_id сохраняется; несуществующий, но
// корректный по формату — 422 на поле repository_id) и Fix 1 (CRITICAL):
// приватная запись, анонимный GET — 404, не 200.
func TestSourceWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:    newDivisionService(t, st),
		Repositories: newRepositoryService(t, st),
		Sources:      newSourceService(t, st),
		Auth:         authSvc,
		DocsFS:       fstest.MapFS{},
		TrustProxy:   false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание без хранилища.
	created := createSource(t, h, owner,
		`{"kind":"document","title":"Метрическая книга","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	if created.Title != "Метрическая книга" || created.RepositoryID != "" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "S-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id.
	rec := getReq(t, h, "/api/sources/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"title":"Метрическая книга"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена kind/title/author/date/reliability/repository_id/notes/private.
	rec = putSourceReq(t, h, owner, "/api/sources/"+string(created.ID),
		`{"kind":"transcription","title":"Метрическая книга (испр.)","reliability":"contemporary","notes":[],"private":false}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeSourceS(t, rec)
	if updated.Title != "Метрическая книга (испр.)" || updated.Kind != "transcription" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/sources/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/sources/"+string(created.ID)), http.StatusNotFound)

	// Строгий FK: валидный repository_id создаётся и сохраняется как есть.
	repo := createRepository(t, h, owner,
		`{"name":"ГАВО","type":"archive","address":"","urls":[],"notes":[],"private":false}`, http.StatusCreated)

	withRepo := createSource(t, h, owner,
		fmt.Sprintf(`{"kind":"document","title":"Дело 1","reliability":"primary","repository_id":%q,"notes":[],"private":false}`, repo.ID),
		http.StatusCreated)
	if withRepo.RepositoryID != string(repo.ID) {
		t.Fatalf("withRepo.RepositoryID = %q, want %q", withRepo.RepositoryID, repo.ID)
	}

	// Строгий FK: корректный по формату, но несуществующий repository_id — 422.
	rec = postSourceReq(t, h, owner,
		`{"kind":"document","title":"Дело-призрак","reliability":"primary","repository_id":"R-01ARZ3NDEKTSV4RRFFQ69G5FA9","notes":[],"private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"repository_id"`) {
		t.Fatalf("body = %s, want field=repository_id", rec.Body)
	}

	// Fix 1: приватная запись, анонимный GET — 404, не 200.
	private := createSource(t, h, owner,
		`{"kind":"memory","title":"Частные воспоминания","reliability":"memory","notes":[],"private":true}`,
		http.StatusCreated)
	if !private.Private {
		t.Fatalf("private = %+v, want Private=true", private)
	}

	requireStatusS(t, getReq(t, h, "/api/sources/"+string(private.ID)), http.StatusNotFound)
}

func createSource(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Source {
	t.Helper()

	rec := postSourceReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeSourceS(t, rec)
}

func decodeSourceS(t *testing.T, rec *httptest.ResponseRecorder) transport.Source {
	t.Helper()

	var s transport.Source
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return s
}

func postSourceReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/sources", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putSourceReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

// TestCitationWriteContractWithRealStore: сквозной путь «хранилище →
// сценарии → HTTP» (по образцу TestArchiveWriteContractWithRealStore) для
// цитат: bootstrap-регистрация → создание источника → создание цитаты без
// якоря → чтение → изменение → удаление → повторное чтение — 404.
// Дополнительно: строгий FK на Source (SourceID всегда обязателен, в
// отличие от Archive.RepositoryID — несуществующий, но корректный по формату
// — 422 на поле source_id), круговорот якоря (ArchiveAnchor — ссылка на
// несуществующий ArchiveNode — 422 на поле anchor.node_id; URLAnchor —
// круговорот без ссылок) и Fix 1 (CRITICAL): приватная запись, анонимный
// GET — 404, не 200.
func TestCitationWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Sources:    newSourceService(t, st),
		Citations:  newCitationService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	src := createSource(t, h, owner,
		`{"kind":"document","title":"Метрическая книга","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)

	// Создание без якоря.
	created := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)
	if created.SourceID != string(src.ID) || created.Anchor != nil {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "C-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id.
	rec := getReq(t, h, "/api/citations/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), fmt.Sprintf(`"source_id":%q`, src.ID)) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена source_id/anchor/text/note/private, всё ещё без якоря.
	rec = putCitationReq(t, h, owner, "/api/citations/"+string(created.ID),
		fmt.Sprintf(`{"source_id":%q,"text":"л. 12 об.","private":false}`, src.ID))
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeCitationS(t, rec)
	if updated.Text != "л. 12 об." || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Строгий FK: корректный по формату, но несуществующий source_id — 422.
	rec = postCitationReq(t, h, owner,
		`{"source_id":"S-01ARZ3NDEKTSV4RRFFQ69G5FA9","private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"source_id"`) {
		t.Fatalf("body = %s, want field=source_id", rec.Body)
	}

	// Круговорот якоря: URLAnchor без ссылок на другие сущности.
	withAnchor := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"anchor":{"kind":"url","url":"https://example.org/page"},"private":false}`, src.ID),
		http.StatusCreated)
	if withAnchor.Anchor == nil || withAnchor.Anchor.Kind != "url" || withAnchor.Anchor.URL != "https://example.org/page" {
		t.Fatalf("withAnchor.Anchor = %+v", withAnchor.Anchor)
	}

	// Якорь со ссылкой на несуществующий ArchiveNode — 422 на поле anchor.node_id.
	rec = postCitationReq(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"anchor":{"kind":"archive","node_id":"AN-01ARZ3NDEKTSV4RRFFQ69G5FA9","page":1},"private":false}`, src.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"anchor.node_id"`) {
		t.Fatalf("body = %s, want field=anchor.node_id", rec.Body)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/citations/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/citations/"+string(created.ID)), http.StatusNotFound)

	// Fix 1: приватная запись, анонимный GET — 404, не 200.
	private := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":true}`, src.ID), http.StatusCreated)
	if !private.Private {
		t.Fatalf("private = %+v, want Private=true", private)
	}

	requireStatusS(t, getReq(t, h, "/api/citations/"+string(private.ID)), http.StatusNotFound)
}

func createCitation(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Citation {
	t.Helper()

	rec := postCitationReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeCitationS(t, rec)
}

func decodeCitationS(t *testing.T, rec *httptest.ResponseRecorder) transport.Citation {
	t.Helper()

	var c transport.Citation
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return c
}

func postCitationReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/citations", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putCitationReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}
```

#### `internal/httpapi/store_test.go` (изменить — итоговое содержимое)
`internal/httpapi/store_test.go`:
```go
package httpapi_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/httpapi"
	"github.com/amarin/genodex/internal/idgen"
	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store/sqlstore"
	create_archive "github.com/amarin/genodex/internal/usecases/create_archive"
	create_archive_document "github.com/amarin/genodex/internal/usecases/create_archive_document"
	create_archive_node "github.com/amarin/genodex/internal/usecases/create_archive_node"
	create_attachment "github.com/amarin/genodex/internal/usecases/create_attachment"
	create_church "github.com/amarin/genodex/internal/usecases/create_church"
	create_citation "github.com/amarin/genodex/internal/usecases/create_citation"
	create_division "github.com/amarin/genodex/internal/usecases/create_division"
	create_estate "github.com/amarin/genodex/internal/usecases/create_estate"
	create_given_name "github.com/amarin/genodex/internal/usecases/create_given_name"
	create_note "github.com/amarin/genodex/internal/usecases/create_note"
	create_parish "github.com/amarin/genodex/internal/usecases/create_parish"
	create_patronymic "github.com/amarin/genodex/internal/usecases/create_patronymic"
	create_repository "github.com/amarin/genodex/internal/usecases/create_repository"
	create_source "github.com/amarin/genodex/internal/usecases/create_source"
	create_surname "github.com/amarin/genodex/internal/usecases/create_surname"
	create_title "github.com/amarin/genodex/internal/usecases/create_title"
	delete_archive "github.com/amarin/genodex/internal/usecases/delete_archive"
	delete_archive_document "github.com/amarin/genodex/internal/usecases/delete_archive_document"
	delete_archive_node "github.com/amarin/genodex/internal/usecases/delete_archive_node"
	delete_attachment "github.com/amarin/genodex/internal/usecases/delete_attachment"
	delete_church "github.com/amarin/genodex/internal/usecases/delete_church"
	delete_citation "github.com/amarin/genodex/internal/usecases/delete_citation"
	delete_division "github.com/amarin/genodex/internal/usecases/delete_division"
	delete_estate "github.com/amarin/genodex/internal/usecases/delete_estate"
	delete_given_name "github.com/amarin/genodex/internal/usecases/delete_given_name"
	delete_note "github.com/amarin/genodex/internal/usecases/delete_note"
	delete_parish "github.com/amarin/genodex/internal/usecases/delete_parish"
	delete_patronymic "github.com/amarin/genodex/internal/usecases/delete_patronymic"
	delete_repository "github.com/amarin/genodex/internal/usecases/delete_repository"
	delete_source "github.com/amarin/genodex/internal/usecases/delete_source"
	delete_surname "github.com/amarin/genodex/internal/usecases/delete_surname"
	delete_title "github.com/amarin/genodex/internal/usecases/delete_title"
	get_archive "github.com/amarin/genodex/internal/usecases/get_archive"
	get_archive_document "github.com/amarin/genodex/internal/usecases/get_archive_document"
	get_archive_node "github.com/amarin/genodex/internal/usecases/get_archive_node"
	get_attachment "github.com/amarin/genodex/internal/usecases/get_attachment"
	get_church "github.com/amarin/genodex/internal/usecases/get_church"
	get_citation "github.com/amarin/genodex/internal/usecases/get_citation"
	get_division "github.com/amarin/genodex/internal/usecases/get_division"
	get_estate "github.com/amarin/genodex/internal/usecases/get_estate"
	get_given_name "github.com/amarin/genodex/internal/usecases/get_given_name"
	get_note "github.com/amarin/genodex/internal/usecases/get_note"
	get_parish "github.com/amarin/genodex/internal/usecases/get_parish"
	get_patronymic "github.com/amarin/genodex/internal/usecases/get_patronymic"
	get_repository "github.com/amarin/genodex/internal/usecases/get_repository"
	get_source "github.com/amarin/genodex/internal/usecases/get_source"
	get_surname "github.com/amarin/genodex/internal/usecases/get_surname"
	get_title "github.com/amarin/genodex/internal/usecases/get_title"
	list_archive_documents "github.com/amarin/genodex/internal/usecases/list_archive_documents"
	list_archive_nodes "github.com/amarin/genodex/internal/usecases/list_archive_nodes"
	list_archives "github.com/amarin/genodex/internal/usecases/list_archives"
	list_attachments "github.com/amarin/genodex/internal/usecases/list_attachments"
	list_churches "github.com/amarin/genodex/internal/usecases/list_churches"
	list_citations "github.com/amarin/genodex/internal/usecases/list_citations"
	list_divisions "github.com/amarin/genodex/internal/usecases/list_divisions"
	list_estates "github.com/amarin/genodex/internal/usecases/list_estates"
	list_given_names "github.com/amarin/genodex/internal/usecases/list_given_names"
	list_notes "github.com/amarin/genodex/internal/usecases/list_notes"
	list_parishes "github.com/amarin/genodex/internal/usecases/list_parishes"
	list_patronymics "github.com/amarin/genodex/internal/usecases/list_patronymics"
	list_repositories "github.com/amarin/genodex/internal/usecases/list_repositories"
	list_sources "github.com/amarin/genodex/internal/usecases/list_sources"
	list_surnames "github.com/amarin/genodex/internal/usecases/list_surnames"
	list_titles "github.com/amarin/genodex/internal/usecases/list_titles"
	search_archive_documents "github.com/amarin/genodex/internal/usecases/search_archive_documents"
	search_archive_nodes "github.com/amarin/genodex/internal/usecases/search_archive_nodes"
	search_archives "github.com/amarin/genodex/internal/usecases/search_archives"
	search_attachments "github.com/amarin/genodex/internal/usecases/search_attachments"
	search_churches "github.com/amarin/genodex/internal/usecases/search_churches"
	search_citations "github.com/amarin/genodex/internal/usecases/search_citations"
	search_divisions "github.com/amarin/genodex/internal/usecases/search_divisions"
	search_estates "github.com/amarin/genodex/internal/usecases/search_estates"
	search_given_names "github.com/amarin/genodex/internal/usecases/search_given_names"
	search_notes "github.com/amarin/genodex/internal/usecases/search_notes"
	search_parishes "github.com/amarin/genodex/internal/usecases/search_parishes"
	search_patronymics "github.com/amarin/genodex/internal/usecases/search_patronymics"
	search_repositories "github.com/amarin/genodex/internal/usecases/search_repositories"
	search_sources "github.com/amarin/genodex/internal/usecases/search_sources"
	search_surnames "github.com/amarin/genodex/internal/usecases/search_surnames"
	search_titles "github.com/amarin/genodex/internal/usecases/search_titles"
	update_archive "github.com/amarin/genodex/internal/usecases/update_archive"
	update_archive_document "github.com/amarin/genodex/internal/usecases/update_archive_document"
	update_archive_node "github.com/amarin/genodex/internal/usecases/update_archive_node"
	update_attachment "github.com/amarin/genodex/internal/usecases/update_attachment"
	update_church "github.com/amarin/genodex/internal/usecases/update_church"
	update_citation "github.com/amarin/genodex/internal/usecases/update_citation"
	update_division "github.com/amarin/genodex/internal/usecases/update_division"
	update_estate "github.com/amarin/genodex/internal/usecases/update_estate"
	update_given_name "github.com/amarin/genodex/internal/usecases/update_given_name"
	update_note "github.com/amarin/genodex/internal/usecases/update_note"
	update_parish "github.com/amarin/genodex/internal/usecases/update_parish"
	update_patronymic "github.com/amarin/genodex/internal/usecases/update_patronymic"
	update_repository "github.com/amarin/genodex/internal/usecases/update_repository"
	update_source "github.com/amarin/genodex/internal/usecases/update_source"
	update_surname "github.com/amarin/genodex/internal/usecases/update_surname"
	update_title "github.com/amarin/genodex/internal/usecases/update_title"
)

// divisionService — сборка httpapi.DivisionService на настоящих сценариях
// (так же собран internal/app).
type divisionService struct {
	list   *list_divisions.Scenario
	search *search_divisions.Scenario
	get    *get_division.Scenario
	create *create_division.Scenario
	update *update_division.Scenario
	del    *delete_division.Scenario
}

func (s *divisionService) ListDivisions(ctx context.Context, access models.Access, q models.DivisionQuery) ([]models.AdministrativeDivision, error) {
	return s.list.ListDivisions(ctx, access, q)
}

func (s *divisionService) SearchDivisions(ctx context.Context, access models.Access, q models.DivisionSearchQuery) ([]models.AdministrativeDivision, error) {
	return s.search.SearchDivisions(ctx, access, q)
}

func (s *divisionService) GetDivision(ctx context.Context, id models.ID) (models.AdministrativeDivision, error) {
	return s.get.GetDivision(ctx, id)
}

func (s *divisionService) CreateDivision(ctx context.Context, d models.AdministrativeDivision) (models.AdministrativeDivision, error) {
	return s.create.CreateDivision(ctx, d)
}

func (s *divisionService) UpdateDivision(ctx context.Context, d models.AdministrativeDivision) error {
	return s.update.UpdateDivision(ctx, d)
}

func (s *divisionService) DeleteDivision(ctx context.Context, id models.ID) error {
	return s.del.DeleteDivision(ctx, id)
}

// newDivisionService собирает фасад на настоящем хранилище.
func newDivisionService(t *testing.T, st *sqlstore.Store) *divisionService {
	t.Helper()

	return &divisionService{
		list:   list_divisions.New(st),
		search: search_divisions.New(st),
		get:    get_division.New(st),
		create: create_division.New(st, idgen.New()),
		update: update_division.New(st),
		del:    delete_division.New(st),
	}
}

// surnameService — сборка httpapi.SurnameService на настоящих сценариях
// (так же собран internal/app's surnameService).
type surnameService struct {
	list   *list_surnames.Scenario
	search *search_surnames.Scenario
	get    *get_surname.Scenario
	create *create_surname.Scenario
	update *update_surname.Scenario
	del    *delete_surname.Scenario
}

func (s *surnameService) ListSurnames(ctx context.Context, access models.Access, page models.Page) ([]models.Surname, error) {
	return s.list.ListSurnames(ctx, access, page)
}

func (s *surnameService) SearchSurnames(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Surname, error) {
	return s.search.SearchSurnames(ctx, access, q)
}

func (s *surnameService) GetSurname(ctx context.Context, id models.ID) (models.Surname, error) {
	return s.get.GetSurname(ctx, id)
}

func (s *surnameService) CreateSurname(ctx context.Context, sn models.Surname) (models.Surname, error) {
	return s.create.CreateSurname(ctx, sn)
}

func (s *surnameService) UpdateSurname(ctx context.Context, sn models.Surname) error {
	return s.update.UpdateSurname(ctx, sn)
}

func (s *surnameService) DeleteSurname(ctx context.Context, id models.ID) error {
	return s.del.DeleteSurname(ctx, id)
}

// newSurnameService собирает фасад на настоящем хранилище.
func newSurnameService(t *testing.T, st *sqlstore.Store) *surnameService {
	t.Helper()

	return &surnameService{
		list:   list_surnames.New(st),
		search: search_surnames.New(st),
		get:    get_surname.New(st),
		create: create_surname.New(st, idgen.New()),
		update: update_surname.New(st),
		del:    delete_surname.New(st),
	}
}

// patronymicService — сборка httpapi.PatronymicService на настоящих сценариях
// (так же собран internal/app's patronymicService).
type patronymicService struct {
	list   *list_patronymics.Scenario
	search *search_patronymics.Scenario
	get    *get_patronymic.Scenario
	create *create_patronymic.Scenario
	update *update_patronymic.Scenario
	del    *delete_patronymic.Scenario
}

func (s *patronymicService) ListPatronymics(ctx context.Context, access models.Access, page models.Page) ([]models.Patronymic, error) {
	return s.list.ListPatronymics(ctx, access, page)
}

func (s *patronymicService) SearchPatronymics(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Patronymic, error) {
	return s.search.SearchPatronymics(ctx, access, q)
}

func (s *patronymicService) GetPatronymic(ctx context.Context, id models.ID) (models.Patronymic, error) {
	return s.get.GetPatronymic(ctx, id)
}

func (s *patronymicService) CreatePatronymic(ctx context.Context, x models.Patronymic) (models.Patronymic, error) {
	return s.create.CreatePatronymic(ctx, x)
}

func (s *patronymicService) UpdatePatronymic(ctx context.Context, x models.Patronymic) error {
	return s.update.UpdatePatronymic(ctx, x)
}

func (s *patronymicService) DeletePatronymic(ctx context.Context, id models.ID) error {
	return s.del.DeletePatronymic(ctx, id)
}

// newPatronymicService собирает фасад на настоящем хранилище.
func newPatronymicService(t *testing.T, st *sqlstore.Store) *patronymicService {
	t.Helper()

	return &patronymicService{
		list:   list_patronymics.New(st),
		search: search_patronymics.New(st),
		get:    get_patronymic.New(st),
		create: create_patronymic.New(st, idgen.New()),
		update: update_patronymic.New(st),
		del:    delete_patronymic.New(st),
	}
}

// estateService — сборка httpapi.EstateService на настоящих сценариях
// (так же собран internal/app's estateService).
type estateService struct {
	list   *list_estates.Scenario
	search *search_estates.Scenario
	get    *get_estate.Scenario
	create *create_estate.Scenario
	update *update_estate.Scenario
	del    *delete_estate.Scenario
}

func (s *estateService) ListEstates(ctx context.Context, access models.Access, page models.Page) ([]models.Estate, error) {
	return s.list.ListEstates(ctx, access, page)
}

func (s *estateService) SearchEstates(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Estate, error) {
	return s.search.SearchEstates(ctx, access, q)
}

func (s *estateService) GetEstate(ctx context.Context, id models.ID) (models.Estate, error) {
	return s.get.GetEstate(ctx, id)
}

func (s *estateService) CreateEstate(ctx context.Context, x models.Estate) (models.Estate, error) {
	return s.create.CreateEstate(ctx, x)
}

func (s *estateService) UpdateEstate(ctx context.Context, x models.Estate) error {
	return s.update.UpdateEstate(ctx, x)
}

func (s *estateService) DeleteEstate(ctx context.Context, id models.ID) error {
	return s.del.DeleteEstate(ctx, id)
}

// newEstateService собирает фасад на настоящем хранилище.
func newEstateService(t *testing.T, st *sqlstore.Store) *estateService {
	t.Helper()

	return &estateService{
		list:   list_estates.New(st),
		search: search_estates.New(st),
		get:    get_estate.New(st),
		create: create_estate.New(st, idgen.New()),
		update: update_estate.New(st),
		del:    delete_estate.New(st),
	}
}

// titleService — сборка httpapi.TitleService на настоящих сценариях
// (так же собран internal/app's titleService).
type titleService struct {
	list   *list_titles.Scenario
	search *search_titles.Scenario
	get    *get_title.Scenario
	create *create_title.Scenario
	update *update_title.Scenario
	del    *delete_title.Scenario
}

func (s *titleService) ListTitles(ctx context.Context, access models.Access, page models.Page) ([]models.Title, error) {
	return s.list.ListTitles(ctx, access, page)
}

func (s *titleService) SearchTitles(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Title, error) {
	return s.search.SearchTitles(ctx, access, q)
}

func (s *titleService) GetTitle(ctx context.Context, id models.ID) (models.Title, error) {
	return s.get.GetTitle(ctx, id)
}

func (s *titleService) CreateTitle(ctx context.Context, x models.Title) (models.Title, error) {
	return s.create.CreateTitle(ctx, x)
}

func (s *titleService) UpdateTitle(ctx context.Context, x models.Title) error {
	return s.update.UpdateTitle(ctx, x)
}

func (s *titleService) DeleteTitle(ctx context.Context, id models.ID) error {
	return s.del.DeleteTitle(ctx, id)
}

// newTitleService собирает фасад на настоящем хранилище.
func newTitleService(t *testing.T, st *sqlstore.Store) *titleService {
	t.Helper()

	return &titleService{
		list:   list_titles.New(st),
		search: search_titles.New(st),
		get:    get_title.New(st),
		create: create_title.New(st, idgen.New()),
		update: update_title.New(st),
		del:    delete_title.New(st),
	}
}

// givenNameService — сборка httpapi.GivenNameService на настоящих сценариях
// (так же собран internal/app's givenNameService).
type givenNameService struct {
	list   *list_given_names.Scenario
	search *search_given_names.Scenario
	get    *get_given_name.Scenario
	create *create_given_name.Scenario
	update *update_given_name.Scenario
	del    *delete_given_name.Scenario
}

func (s *givenNameService) ListGivenNames(ctx context.Context, access models.Access, page models.Page) ([]models.GivenName, error) {
	return s.list.ListGivenNames(ctx, access, page)
}

func (s *givenNameService) SearchGivenNames(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.GivenName, error) {
	return s.search.SearchGivenNames(ctx, access, q)
}

func (s *givenNameService) GetGivenName(ctx context.Context, id models.ID) (models.GivenName, error) {
	return s.get.GetGivenName(ctx, id)
}

func (s *givenNameService) CreateGivenName(ctx context.Context, x models.GivenName) (models.GivenName, error) {
	return s.create.CreateGivenName(ctx, x)
}

func (s *givenNameService) UpdateGivenName(ctx context.Context, x models.GivenName) error {
	return s.update.UpdateGivenName(ctx, x)
}

func (s *givenNameService) DeleteGivenName(ctx context.Context, id models.ID) error {
	return s.del.DeleteGivenName(ctx, id)
}

// newGivenNameService собирает фасад на настоящем хранилище.
func newGivenNameService(t *testing.T, st *sqlstore.Store) *givenNameService {
	t.Helper()

	return &givenNameService{
		list:   list_given_names.New(st),
		search: search_given_names.New(st),
		get:    get_given_name.New(st),
		create: create_given_name.New(st, idgen.New()),
		update: update_given_name.New(st),
		del:    delete_given_name.New(st),
	}
}

// repositoryService — фасад httpapi.RepositoryService на настоящих сценариях
// (так же собран internal/app's repositoryService).
type repositoryService struct {
	list   *list_repositories.Scenario
	search *search_repositories.Scenario
	get    *get_repository.Scenario
	create *create_repository.Scenario
	update *update_repository.Scenario
	del    *delete_repository.Scenario
}

func (s *repositoryService) ListRepositories(ctx context.Context, access models.Access, page models.Page) ([]models.Repository, error) {
	return s.list.ListRepositories(ctx, access, page)
}

func (s *repositoryService) SearchRepositories(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Repository, error) {
	return s.search.SearchRepositories(ctx, access, q)
}

func (s *repositoryService) GetRepository(ctx context.Context, access models.Access, id models.ID) (models.Repository, error) {
	return s.get.GetRepository(ctx, access, id)
}

func (s *repositoryService) CreateRepository(ctx context.Context, r models.Repository) (models.Repository, error) {
	return s.create.CreateRepository(ctx, r)
}

func (s *repositoryService) UpdateRepository(ctx context.Context, r models.Repository) error {
	return s.update.UpdateRepository(ctx, r)
}

func (s *repositoryService) DeleteRepository(ctx context.Context, id models.ID) error {
	return s.del.DeleteRepository(ctx, id)
}

// newRepositoryService собирает фасад на настоящем хранилище.
func newRepositoryService(t *testing.T, st *sqlstore.Store) *repositoryService {
	t.Helper()

	return &repositoryService{
		list:   list_repositories.New(st),
		search: search_repositories.New(st),
		get:    get_repository.New(st),
		create: create_repository.New(st, idgen.New()),
		update: update_repository.New(st),
		del:    delete_repository.New(st),
	}
}

// churchService — фасад httpapi.ChurchService на настоящих сценариях (так же
// собран internal/app's churchService).
type churchService struct {
	list   *list_churches.Scenario
	search *search_churches.Scenario
	get    *get_church.Scenario
	create *create_church.Scenario
	update *update_church.Scenario
	del    *delete_church.Scenario
}

func (s *churchService) ListChurches(ctx context.Context, access models.Access, page models.Page) ([]models.Church, error) {
	return s.list.ListChurches(ctx, access, page)
}

func (s *churchService) SearchChurches(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Church, error) {
	return s.search.SearchChurches(ctx, access, q)
}

func (s *churchService) GetChurch(ctx context.Context, id models.ID) (models.Church, error) {
	return s.get.GetChurch(ctx, id)
}

func (s *churchService) CreateChurch(ctx context.Context, c models.Church) (models.Church, error) {
	return s.create.CreateChurch(ctx, c)
}

func (s *churchService) UpdateChurch(ctx context.Context, c models.Church) error {
	return s.update.UpdateChurch(ctx, c)
}

func (s *churchService) DeleteChurch(ctx context.Context, id models.ID) error {
	return s.del.DeleteChurch(ctx, id)
}

// newChurchService собирает фасад на настоящем хранилище.
func newChurchService(t *testing.T, st *sqlstore.Store) *churchService {
	t.Helper()

	return &churchService{
		list:   list_churches.New(st),
		search: search_churches.New(st),
		get:    get_church.New(st),
		create: create_church.New(st, idgen.New()),
		update: update_church.New(st),
		del:    delete_church.New(st),
	}
}

// parishService — фасад httpapi.ParishService на настоящих сценариях (так же
// собран internal/app's parishService).
type parishService struct {
	list   *list_parishes.Scenario
	search *search_parishes.Scenario
	get    *get_parish.Scenario
	create *create_parish.Scenario
	update *update_parish.Scenario
	del    *delete_parish.Scenario
}

func (s *parishService) ListParishes(ctx context.Context, access models.Access, page models.Page) ([]models.Parish, error) {
	return s.list.ListParishes(ctx, access, page)
}

func (s *parishService) SearchParishes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Parish, error) {
	return s.search.SearchParishes(ctx, access, q)
}

func (s *parishService) GetParish(ctx context.Context, id models.ID) (models.Parish, error) {
	return s.get.GetParish(ctx, id)
}

func (s *parishService) CreateParish(ctx context.Context, p models.Parish) (models.Parish, error) {
	return s.create.CreateParish(ctx, p)
}

func (s *parishService) UpdateParish(ctx context.Context, p models.Parish) error {
	return s.update.UpdateParish(ctx, p)
}

func (s *parishService) DeleteParish(ctx context.Context, id models.ID) error {
	return s.del.DeleteParish(ctx, id)
}

// newParishService собирает фасад на настоящем хранилище.
func newParishService(t *testing.T, st *sqlstore.Store) *parishService {
	t.Helper()

	return &parishService{
		list:   list_parishes.New(st),
		search: search_parishes.New(st),
		get:    get_parish.New(st),
		create: create_parish.New(st, idgen.New()),
		update: update_parish.New(st),
		del:    delete_parish.New(st),
	}
}

// archiveService — фасад httpapi.ArchiveService на настоящих сценариях (так
// же собран internal/app's archiveService).
type archiveService struct {
	list   *list_archives.Scenario
	search *search_archives.Scenario
	get    *get_archive.Scenario
	create *create_archive.Scenario
	update *update_archive.Scenario
	del    *delete_archive.Scenario
}

func (s *archiveService) ListArchives(ctx context.Context, access models.Access, page models.Page) ([]models.Archive, error) {
	return s.list.ListArchives(ctx, access, page)
}

func (s *archiveService) SearchArchives(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Archive, error) {
	return s.search.SearchArchives(ctx, access, q)
}

func (s *archiveService) GetArchive(ctx context.Context, access models.Access, id models.ID) (models.Archive, error) {
	return s.get.GetArchive(ctx, access, id)
}

func (s *archiveService) CreateArchive(ctx context.Context, a models.Archive) (models.Archive, error) {
	return s.create.CreateArchive(ctx, a)
}

func (s *archiveService) UpdateArchive(ctx context.Context, a models.Archive) error {
	return s.update.UpdateArchive(ctx, a)
}

func (s *archiveService) DeleteArchive(ctx context.Context, id models.ID) error {
	return s.del.DeleteArchive(ctx, id)
}

// newArchiveService собирает фасад на настоящем хранилище.
func newArchiveService(t *testing.T, st *sqlstore.Store) *archiveService {
	t.Helper()

	return &archiveService{
		list:   list_archives.New(st),
		search: search_archives.New(st),
		get:    get_archive.New(st),
		create: create_archive.New(st, idgen.New()),
		update: update_archive.New(st),
		del:    delete_archive.New(st),
	}
}

// archiveNodeService — фасад httpapi.ArchiveNodeService на настоящих
// сценариях (так же собран internal/app's archiveNodeService).
type archiveNodeService struct {
	list   *list_archive_nodes.Scenario
	search *search_archive_nodes.Scenario
	get    *get_archive_node.Scenario
	create *create_archive_node.Scenario
	update *update_archive_node.Scenario
	del    *delete_archive_node.Scenario
}

func (s *archiveNodeService) ListArchiveNodes(ctx context.Context, access models.Access, q models.ArchiveNodeQuery) ([]models.ArchiveNode, error) {
	return s.list.ListArchiveNodes(ctx, access, q)
}

func (s *archiveNodeService) SearchArchiveNodes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.ArchiveNode, error) {
	return s.search.SearchArchiveNodes(ctx, access, q)
}

func (s *archiveNodeService) GetArchiveNode(ctx context.Context, access models.Access, id models.ID) (models.ArchiveNode, error) {
	return s.get.GetArchiveNode(ctx, access, id)
}

func (s *archiveNodeService) CreateArchiveNode(ctx context.Context, n models.ArchiveNode) (models.ArchiveNode, error) {
	return s.create.CreateArchiveNode(ctx, n)
}

func (s *archiveNodeService) UpdateArchiveNode(ctx context.Context, n models.ArchiveNode) error {
	return s.update.UpdateArchiveNode(ctx, n)
}

func (s *archiveNodeService) DeleteArchiveNode(ctx context.Context, id models.ID) error {
	return s.del.DeleteArchiveNode(ctx, id)
}

// newArchiveNodeService собирает фасад на настоящем хранилище.
func newArchiveNodeService(t *testing.T, st *sqlstore.Store) *archiveNodeService {
	t.Helper()

	return &archiveNodeService{
		list:   list_archive_nodes.New(st),
		search: search_archive_nodes.New(st),
		get:    get_archive_node.New(st),
		create: create_archive_node.New(st, idgen.New()),
		update: update_archive_node.New(st),
		del:    delete_archive_node.New(st),
	}
}

// archiveDocumentService — фасад httpapi.ArchiveDocumentService на настоящих
// сценариях (так же собран internal/app's archiveDocumentService).
type archiveDocumentService struct {
	list   *list_archive_documents.Scenario
	search *search_archive_documents.Scenario
	get    *get_archive_document.Scenario
	create *create_archive_document.Scenario
	update *update_archive_document.Scenario
	del    *delete_archive_document.Scenario
}

func (s *archiveDocumentService) ListArchiveDocuments(ctx context.Context, access models.Access, page models.Page) ([]models.ArchiveDocument, error) {
	return s.list.ListArchiveDocuments(ctx, access, page)
}

func (s *archiveDocumentService) SearchArchiveDocuments(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.ArchiveDocument, error) {
	return s.search.SearchArchiveDocuments(ctx, access, q)
}

func (s *archiveDocumentService) GetArchiveDocument(ctx context.Context, access models.Access, id models.ID) (models.ArchiveDocument, error) {
	return s.get.GetArchiveDocument(ctx, access, id)
}

func (s *archiveDocumentService) CreateArchiveDocument(ctx context.Context, d models.ArchiveDocument) (models.ArchiveDocument, error) {
	return s.create.CreateArchiveDocument(ctx, d)
}

func (s *archiveDocumentService) UpdateArchiveDocument(ctx context.Context, d models.ArchiveDocument) error {
	return s.update.UpdateArchiveDocument(ctx, d)
}

func (s *archiveDocumentService) DeleteArchiveDocument(ctx context.Context, id models.ID) error {
	return s.del.DeleteArchiveDocument(ctx, id)
}

// newArchiveDocumentService собирает фасад на настоящем хранилище.
func newArchiveDocumentService(t *testing.T, st *sqlstore.Store) *archiveDocumentService {
	t.Helper()

	return &archiveDocumentService{
		list:   list_archive_documents.New(st),
		search: search_archive_documents.New(st),
		get:    get_archive_document.New(st),
		create: create_archive_document.New(st, idgen.New()),
		update: update_archive_document.New(st),
		del:    delete_archive_document.New(st),
	}
}

// sourceService — фасад httpapi.SourceService на настоящих сценариях (так же
// собран internal/app's sourceService).
type sourceService struct {
	list   *list_sources.Scenario
	search *search_sources.Scenario
	get    *get_source.Scenario
	create *create_source.Scenario
	update *update_source.Scenario
	del    *delete_source.Scenario
}

func (s *sourceService) ListSources(ctx context.Context, access models.Access, page models.Page) ([]models.Source, error) {
	return s.list.ListSources(ctx, access, page)
}

func (s *sourceService) SearchSources(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Source, error) {
	return s.search.SearchSources(ctx, access, q)
}

func (s *sourceService) GetSource(ctx context.Context, access models.Access, id models.ID) (models.Source, error) {
	return s.get.GetSource(ctx, access, id)
}

func (s *sourceService) CreateSource(ctx context.Context, src models.Source) (models.Source, error) {
	return s.create.CreateSource(ctx, src)
}

func (s *sourceService) UpdateSource(ctx context.Context, src models.Source) error {
	return s.update.UpdateSource(ctx, src)
}

func (s *sourceService) DeleteSource(ctx context.Context, id models.ID) error {
	return s.del.DeleteSource(ctx, id)
}

// newSourceService собирает фасад на настоящем хранилище.
func newSourceService(t *testing.T, st *sqlstore.Store) *sourceService {
	t.Helper()

	return &sourceService{
		list:   list_sources.New(st),
		search: search_sources.New(st),
		get:    get_source.New(st),
		create: create_source.New(st, idgen.New()),
		update: update_source.New(st),
		del:    delete_source.New(st),
	}
}

// citationService — фасад httpapi.CitationService на настоящих сценариях
// (так же собран internal/app's citationService).
type citationService struct {
	list   *list_citations.Scenario
	search *search_citations.Scenario
	get    *get_citation.Scenario
	create *create_citation.Scenario
	update *update_citation.Scenario
	del    *delete_citation.Scenario
}

func (s *citationService) ListCitations(ctx context.Context, access models.Access, page models.Page) ([]models.Citation, error) {
	return s.list.ListCitations(ctx, access, page)
}

func (s *citationService) SearchCitations(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Citation, error) {
	return s.search.SearchCitations(ctx, access, q)
}

func (s *citationService) GetCitation(ctx context.Context, access models.Access, id models.ID) (models.Citation, error) {
	return s.get.GetCitation(ctx, access, id)
}

func (s *citationService) CreateCitation(ctx context.Context, c models.Citation) (models.Citation, error) {
	return s.create.CreateCitation(ctx, c)
}

func (s *citationService) UpdateCitation(ctx context.Context, c models.Citation) error {
	return s.update.UpdateCitation(ctx, c)
}

func (s *citationService) DeleteCitation(ctx context.Context, id models.ID) error {
	return s.del.DeleteCitation(ctx, id)
}

// newCitationService собирает фасад на настоящем хранилище.
func newCitationService(t *testing.T, st *sqlstore.Store) *citationService {
	t.Helper()

	return &citationService{
		list:   list_citations.New(st),
		search: search_citations.New(st),
		get:    get_citation.New(st),
		create: create_citation.New(st, idgen.New()),
		update: update_citation.New(st),
		del:    delete_citation.New(st),
	}
}

// noteService — фасад httpapi.NoteService на настоящих сценариях (так же
// собран internal/app's noteService).
type noteService struct {
	list   *list_notes.Scenario
	search *search_notes.Scenario
	get    *get_note.Scenario
	create *create_note.Scenario
	update *update_note.Scenario
	del    *delete_note.Scenario
}

func (s *noteService) ListNotes(ctx context.Context, access models.Access, page models.Page) ([]models.Note, error) {
	return s.list.ListNotes(ctx, access, page)
}

func (s *noteService) SearchNotes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Note, error) {
	return s.search.SearchNotes(ctx, access, q)
}

func (s *noteService) GetNote(ctx context.Context, access models.Access, id models.ID) (models.Note, error) {
	return s.get.GetNote(ctx, access, id)
}

func (s *noteService) CreateNote(ctx context.Context, n models.Note) (models.Note, error) {
	return s.create.CreateNote(ctx, n)
}

func (s *noteService) UpdateNote(ctx context.Context, n models.Note) error {
	return s.update.UpdateNote(ctx, n)
}

func (s *noteService) DeleteNote(ctx context.Context, id models.ID) error {
	return s.del.DeleteNote(ctx, id)
}

// newNoteService собирает фасад на настоящем хранилище.
func newNoteService(t *testing.T, st *sqlstore.Store) *noteService {
	t.Helper()

	return &noteService{
		list:   list_notes.New(st),
		search: search_notes.New(st),
		get:    get_note.New(st),
		create: create_note.New(st, idgen.New()),
		update: update_note.New(st),
		del:    delete_note.New(st),
	}
}

// attachmentService — фасад httpapi.AttachmentService на настоящих сценариях
// (так же собран internal/app's attachmentService).
type attachmentService struct {
	list   *list_attachments.Scenario
	search *search_attachments.Scenario
	get    *get_attachment.Scenario
	create *create_attachment.Scenario
	update *update_attachment.Scenario
	del    *delete_attachment.Scenario
}

func (s *attachmentService) ListAttachments(ctx context.Context, access models.Access, page models.Page) ([]models.Attachment, error) {
	return s.list.ListAttachments(ctx, access, page)
}

func (s *attachmentService) SearchAttachments(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Attachment, error) {
	return s.search.SearchAttachments(ctx, access, q)
}

func (s *attachmentService) GetAttachment(ctx context.Context, access models.Access, id models.ID) (models.Attachment, error) {
	return s.get.GetAttachment(ctx, access, id)
}

func (s *attachmentService) CreateAttachment(ctx context.Context, a models.Attachment) (models.Attachment, error) {
	return s.create.CreateAttachment(ctx, a)
}

func (s *attachmentService) UpdateAttachment(ctx context.Context, a models.Attachment) error {
	return s.update.UpdateAttachment(ctx, a)
}

func (s *attachmentService) DeleteAttachment(ctx context.Context, id models.ID) error {
	return s.del.DeleteAttachment(ctx, id)
}

// newAttachmentService собирает фасад на настоящем хранилище.
func newAttachmentService(t *testing.T, st *sqlstore.Store) *attachmentService {
	t.Helper()

	return &attachmentService{
		list:   list_attachments.New(st),
		search: search_attachments.New(st),
		get:    get_attachment.New(st),
		create: create_attachment.New(st, idgen.New()),
		update: update_attachment.New(st),
		del:    delete_attachment.New(st),
	}
}

// TestAdminDivisionsWithRealStore: сквозной путь «хранилище → сценарий → HTTP»
// на настоящей БД (так же собран internal/app): фильтры, окно после фильтра,
// parent_id, коды ошибок, прежнего пути нет.
func TestAdminDivisionsWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	root := models.ID("AD-11HFE865V215DE1CTEWH0AVNH9")
	ad1 := models.ID("AD-3N65R6PPG2X7R5JQC42EJ5E6RH")
	ad2 := models.ID("AD-7QAQPDH4AFAXRHEM3E2MSH4DMD")
	ad3 := models.ID("AD-7SX9G8FGVSQE8Z5379AV4RRXG0")
	missingParent := "AD-01ARZ3NDEKTSV4RRFFQ69G5FA9" // валидный формат, не сохранён

	for _, d := range []models.AdministrativeDivision{
		{ID: root, Name: "Московская", Type: models.AdminDivisionGovernorate},
		{ID: ad1, Name: "Давыдово", Type: models.AdminDivisionSelo, ParentID: &root},
		{ID: ad2, Name: "Никифоровская", Type: models.AdminDivisionVolost, ParentID: &root,
			Variants: []string{"Никольское"}},
		{ID: ad3, Name: "Никифорово", Type: models.AdminDivisionDerevnya, ParentID: &root},
	} {
		if err := st.SaveAdministrativeDivision(t.Context(), &d); err != nil {
			t.Fatalf("save %s: %v", d.ID, err)
		}
	}

	h := httpapi.NewHandler(httpapi.Deps{Divisions: newDivisionService(t, st), DocsFS: fstest.MapFS{}})

	cases := []struct {
		target string
		code   int
		body   string // пусто — не сверять
	}{
		{"/api/admin-divisions?kind=settlement", http.StatusOK,
			fmt.Sprintf(`[{"id":%q,"name":"Давыдово","type":"selo","parent_id":%q,"sources":[]},`+
				`{"id":%q,"name":"Никифорово","type":"derevnya","parent_id":%q,"sources":[]}]`, ad1, root, ad3, root)},
		{"/api/admin-divisions?kind=settlement&limit=1&offset=1", http.StatusOK,
			fmt.Sprintf(`[{"id":%q,"name":"Никифорово","type":"derevnya","parent_id":%q,"sources":[]}]`, ad3, root)},
		{"/api/admin-divisions?type=governorate", http.StatusOK,
			fmt.Sprintf(`[{"id":%q,"name":"Московская","type":"governorate","parent_id":null,"sources":[]}]`, root)},
		{"/api/admin-divisions?type=castle", http.StatusUnprocessableEntity,
			`{"error":"type: неизвестный тип единицы деления \"castle\"","field":"type"}`},
		{"/api/admin-divisions?limit=x", http.StatusBadRequest,
			`{"error":"параметр limit: ожидалось целое число, получено \"x\""}`},
		{"/api/settlements", http.StatusNotFound, ""},
		{"/api/admin-divisions/search?q=давы", http.StatusOK,
			fmt.Sprintf(`[{"id":%q,"name":"Давыдово","type":"selo","parent_id":%q,"sources":[]}]`, ad1, root)},
		{"/api/admin-divisions/search?q=", http.StatusOK, `[]`},
		{"/api/admin-divisions/search?q=ник", http.StatusOK,
			fmt.Sprintf(`[{"id":%q,"name":"Никифоровская","type":"volost","parent_id":%q,"sources":[]},`+
				`{"id":%q,"name":"Никифорово","type":"derevnya","parent_id":%q,"sources":[]}]`, ad2, root, ad3, root)},
		{"/api/admin-divisions/search?q=никол", http.StatusOK,
			fmt.Sprintf(`[{"id":%q,"name":"Никифоровская","type":"volost","parent_id":%q,"sources":[]}]`, ad2, root)},
		{"/api/admin-divisions/search?q=ик", http.StatusOK, `[]`},
		{"/api/admin-divisions?parent_id=" + string(root), http.StatusOK,
			fmt.Sprintf(`[{"id":%q,"name":"Давыдово","type":"selo","parent_id":%q,"sources":[]},`+
				`{"id":%q,"name":"Никифоровская","type":"volost","parent_id":%q,"sources":[]},`+
				`{"id":%q,"name":"Никифорово","type":"derevnya","parent_id":%q,"sources":[]}]`,
				ad1, root, ad2, root, ad3, root)},
		{"/api/admin-divisions?parent_id=" + missingParent, http.StatusNotFound, ""},
		{"/api/admin-divisions?parent_id=not-an-id", http.StatusUnprocessableEntity, ""},
	}

	for _, c := range cases {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, c.target, nil))

		body := strings.TrimSpace(rec.Body.String())
		if rec.Code != c.code || (c.body != "" && body != c.body) {
			t.Errorf("GET %s = %d %s\n want %d %s", c.target, rec.Code, body, c.code, c.body)
		}
	}
}
```

### 1.6. Документация: `docs/usage.md`, `docs/architecture.md`, `CHANGELOG.md`, `entity-write.md` §3.4

По правилу, закреплённому после подпроекта 4 (доки/CHANGELOG пишутся в плане, а не оставляются на финальное ревью) — ниже точные вставки/замены в уже существующие большие файлы; порядок существующих строк не меняется, только добавляются/заменяются указанные фрагменты. Все четыре файла входят в Задачу 1 целиком и самодостаточны на докам — ни Задача 2, ни Задача 3 доки не трогают.

#### `docs/usage.md` — две новые строки HTTP-таблицы (после `/api/archives`, до `/api/notes`) + правка строки `/api/attachments`
Вставить после строки `/api/archives` и перед строкой `/api/notes`:
```markdown
| `/api/archive-nodes` | Узлы архивного дерева (JSON): `GET` — список `[{"id", "type", "archive_id", "parent_id", "label", "name", "since", "until", "parish", "settlements", "notes", "sources", "private"}]`, **`archive_id` обязателен** (у узла нет смысла вне архива — отсутствующий или пустой параметр — 400), `parent_id` необязателен (отсутствует — корень дерева внутри архива, иначе — только прямые дети этого узла); `POST` — создание; `GET/PUT/DELETE /api/archive-nodes/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404); `GET /api/archive-nodes/search?q=` — поиск по началу метки/названия, **без** сужения по `archive_id` (глобальный поиск по всем архивам). `archive_id` — строгая ссылка на `/api/archives`: несуществующий id — 422 на поле `archive_id`. `parent_id` — необязательная строгая self-ref ссылка на другой узел: несуществующий id — 422 на поле `parent_id`; узел, принадлежащий ДРУГОМУ архиву — тоже 422 на поле `parent_id` (инвариант «родитель из того же архива, что и сам узел»); при `PUT`, если новый `parent_id` образует цикл (прямой или через цепочку), — тоже 422 на поле `parent_id`. |
| `/api/archive-documents` | Документы внутри единиц учёта (JSON): `GET` — список `[{"id", "unit_id", "title", "kind", "since", "until", "parish", "settlements", "notes", "sources", "private"}]` (плоский список, без обязательных фильтров); `POST` — создание; `GET/PUT/DELETE /api/archive-documents/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404); `GET /api/archive-documents/search?q=` — поиск по началу названия. `unit_id` — обязательная строгая ссылка на `/api/archive-nodes` (единицу учёта): несуществующий id — 422 на поле `unit_id`. |
```
Заменить строку `/api/attachments` (упоминание «ещё без своего `/api/`-эндпоинта» устарело — подпроект 6 доставил оба эндпоинта):
```markdown
| `/api/attachments` | Файловые вложения (JSON): `GET` — список `[{"id", "kind", "uri", "filename", "mime", "page", "node_id", "document_id", "note", "private"}]`; `POST` — создание; `GET/PUT/DELETE /api/attachments/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404); `GET /api/attachments/search?q=` — поиск по началу имени файла или URI. `node_id` — обязательная строгая ссылка на архивный узел (`ArchiveNode`, см. `/api/archive-nodes` выше): несуществующий id — 422 на поле `node_id`. `document_id` — необязательная мягкая ссылка на архивный документ (`ArchiveDocument`, см. `/api/archive-documents` выше, `ON DELETE SET NULL` в схеме); при создании/изменении, если задан, существование тоже проверяется — 422 на поле `document_id`. |
```

#### `docs/usage.md` — дополнение к строке про `sources` (после HTTP-таблицы)
В конец существующего абзаца про `sources` у 6 сущностей подпроекта 5 добавить предложение:
```markdown
`/api/archive-nodes`/`/api/archive-documents` (подпроект 6) — первые полностью новые сущности программы с `sources`, редактируемым с самого начала (не ретрофит).
```

#### `docs/usage.md` — 12 новых строк таблицы MCP-тулов (после `archive_delete`, до `note_list`)
```markdown
| `archive_node_list` | Список узлов архивного дерева; аргументы `archive_id` (обязателен), `parent_id` (необязателен — отсутствует: корень дерева внутри архива), `limit`, `offset` |
| `archive_node_search` | Поиск узлов по началу метки/названия, среди всех архивов (без сужения по `archive_id`); аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `archive_node_get` | Запись по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула (как отсутствующая) |
| `archive_node_create` | Создание записи: `type`, `archive_id`, `label` (обязательны), `parent_id` (строгая self-ref ссылка — должен принадлежать тому же архиву, иначе ошибка тула на поле `parent_id`), `name`, `since`/`until` (структурированная дата), `parish` (одиночная необязательная ссылка `{text, ref?, type?}`), `settlements`, `notes`, `sources`, `private`; id генерирует сервер |
| `archive_node_update` | Изменение записи: полная замена `type`/`archive_id`/`parent_id`/`label`/`name`/`since`/`until`/`parish`/`settlements`/`notes`/`private`; несуществующий `archive_id`/`parent_id`, `parent_id` из другого архива или цикл в цепочке родителей — ошибка тула на поле `archive_id`/`parent_id`; `sources` — при отсутствии в вызове текущие источники сохраняются, пустой массив — очищает их |
| `archive_node_delete` | Удаление записи по `id`; на неё ссылаются дочерние узлы или документы — ошибка тула |
| `archive_document_list` | Список документов внутри единиц учёта (плоский список, без обязательных фильтров); аргументы `limit`, `offset` |
| `archive_document_search` | Поиск документов по началу названия; аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `archive_document_get` | Запись по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула (как отсутствующая) |
| `archive_document_create` | Создание записи: `unit_id` (строгая ссылка на узел архивного дерева, обязателен), `title` (обязателен), `kind`, `since`/`until`, `parish`, `settlements`, `notes`, `sources`, `private`; id генерирует сервер |
| `archive_document_update` | Изменение записи: полная замена `unit_id`/`title`/`kind`/`since`/`until`/`parish`/`settlements`/`notes`/`private`; несуществующий `unit_id` — ошибка тула на поле `unit_id`; `sources` — при отсутствии в вызове текущие источники сохраняются, пустой массив — очищает их |
| `archive_document_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
```

#### `docs/architecture.md` — строка `internal/httpapi/`
Заменить:
```markdown
| `internal/httpapi/` | Слой HTTP API: JSON-роуты `/api/*` (health, admin-divisions, surnames, patronymics, estates, titles, given-names, repositories, churches, parishes, archives, notes, attachments, sources, citations, auth, docs). Использует те же сценарии, что и MCP. |
```
на:
```markdown
| `internal/httpapi/` | Слой HTTP API: JSON-роуты `/api/*` (health, admin-divisions, surnames, patronymics, estates, titles, given-names, repositories, churches, parishes, archives, archive-nodes, archive-documents, notes, attachments, sources, citations, auth, docs). Использует те же сценарии, что и MCP. |
```

#### `CHANGELOG.md` — новый пункт (после пункта про Sources у 6 сущностей, перед «Веб: единая точка входа»)
Вставить перед пунктом «Веб: единая точка входа»:
```markdown
- Архивные деревья (`ArchiveNode`/`ArchiveDocument`, backend) — подпроект 6,
  бэкенд: usecase-сценарии, HTTP (`/api/archive-nodes*`, `/api/archive-
  documents*`) и MCP (`archive_node_*`/`archive_document_*` — по 6 тулов на
  сущность). `ArchiveNode` — первое дерево, скопированное per-owner
  (обязательный `ArchiveID`), а не единственное глобальное, как у
  `AdministrativeDivision`: список без `ParentID` — корень внутри архива,
  без выделенного метода хранилища «дети узла» — полный проход по окнам
  generic `ListArchiveNodes` с фильтром по `ArchiveID`+`ParentID` (масштаб
  данных это позволяет). Новый вид кросс-полевой проверки — «родитель
  существует И принадлежит тому же архиву, что и сам узел», не только «X
  существует»: несуществующий/чужого-архива `parent_id` — 422 на поле
  `parent_id`, отдельно от 422 на `archive_id` (несуществующий архив).
  `ArchiveDocument` — плоская сущность со строгим `UnitID` на `ArchiveNode`.
  Обе — первые совершенно новые сущности программы, получившие `Sources
  []SourceLink` сразу редактируемым (не ретрофит, как у 6 сущностей выше)
  (`internal/models/query.go` (`ArchiveNodeQuery`), `internal/usecases/
  {list,search,get,create,update,delete}_archive_{node,document}*`,
  `internal/transport/archive_{node,document}{,_write}.go`,
  `internal/httpapi/archive_{node,document}{,_write}.go`,
  `internal/mcp/archive_{node,document}.go`).
```

#### `docs/data-model/entity-write.md` — новая секция §3.4 (после §3.3, до `## 4.`)
`docs/data-model/entity-write.md (фрагмент — новая секция §3.4)`:
```markdown
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
  «без каскадной валидации», что и везде в этой программе.
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
```

### 1.7. Рубеж

`gofmt -l .` пусто; `go build ./...`, `go vet ./...`, `go test ./...` (весь репозиторий) зелёные — ожидается 1215 тестов, 112 пакетов (было 1097 после подпроекта 5). Живой смок через `curl` (см. «Предпосылка») — не обязателен повторно: создать `Archive` × 2, корневой `ArchiveNode` в первом, дочерний `ArchiveNode` в нём же, проверить `GET /api/archive-nodes?archive_id=…` с/без `parent_id`, проверить 422 на `parent_id` из другого архива, создать `ArchiveDocument`, проверить 422 на несуществующий `unit_id`.

### 1.8. Коммит

```bash
git add \
  internal/models/query.go \
  internal/transport/archive_node.go internal/transport/archive_node_write.go \
  internal/transport/archive_document.go internal/transport/archive_document_write.go \
  internal/usecases/list_archive_nodes internal/usecases/search_archive_nodes internal/usecases/get_archive_node internal/usecases/create_archive_node internal/usecases/update_archive_node internal/usecases/delete_archive_node \
  internal/usecases/list_archive_documents internal/usecases/search_archive_documents internal/usecases/get_archive_document internal/usecases/create_archive_document internal/usecases/update_archive_document internal/usecases/delete_archive_document \
  internal/httpapi/archive_node.go internal/httpapi/archive_node_write.go internal/httpapi/archive_node_test.go internal/httpapi/archive_node_write_test.go \
  internal/httpapi/archive_document.go internal/httpapi/archive_document_write.go internal/httpapi/archive_document_test.go internal/httpapi/archive_document_write_test.go \
  internal/mcp/archive_node.go internal/mcp/archive_node_test.go internal/mcp/archive_document.go internal/mcp/archive_document_test.go \
  internal/httpapi/deps.go internal/httpapi/api.go internal/httpapi/httpapi.go internal/mcp/deps.go internal/mcp/server.go internal/app/app.go \
  internal/httpapi/write_store_test.go internal/httpapi/store_test.go \
  docs/usage.md docs/architecture.md CHANGELOG.md docs/data-model/entity-write.md
git commit -m "feat(backend): ArchiveNode/ArchiveDocument — полный CRUD (usecases/httpapi/mcp) + доки + real-store тесты"
```

## Задача 2. Веб: страницы ArchiveNode/ArchiveDocument + общий picker + роуты/каталог

**Внимание — порядок относительно Задачи 3**: Задача 3 (ретрофит `Attachment`/`AnchorEditor`) зависит от `web/src/ArchiveNodePicker.tsx`, который создаёт эта задача (Шаг 2.1) — в SDD-диспетче Задача 3 обязана исполняться СТРОГО ПОСЛЕ Задачи 2, не параллельно. Файловых пересечений между Задачей 2 и Задачей 3 нет (Задача 3 трогает только `AnchorEditor.tsx`/`AttachmentForm.tsx`/`AttachmentView.tsx`, ни один из которых не входит в Задачу 2) — зависимость только по интерфейсу (`ArchiveNodePicker`/`ArchiveDocumentSelect`), не по содержимому файла.

**Интерфейсы, потребляемые из Задачи 1**: HTTP-маршруты `/api/archive-nodes*`/`/api/archive-documents*` (контракты полей — см. `docs/usage.md`, Шаг 1.6), `web/src/api.ts`'s уже существующие конвенции `authFetch`/`ApiError`/409-`referrers` (подпроект 5, `Source`/`Citation`), `MAX_PAGE_LIMIT`.
**Производит**: `ArchiveNodePicker`/`ArchiveDocumentSelect`/`useArchiveOptions` (`web/src/ArchiveNodePicker.tsx`) — потребляется Задачей 3; типы/функции `ArchiveNode*`/`ArchiveDocument*` в `web/src/api.ts` — потребляются и Задачей 2 (собственные страницы), и Задачей 3 (ретрофит); роуты `/archive-nodes`, `/archive-nodes/:id`, `/archive-documents`, `/archive-documents/:id`.

**Файлы:**
- Создать: `web/src/ArchiveNodePicker.tsx`, `web/src/pages/{ArchiveNodesList,ArchiveNodeForm,ArchiveNodeView}.tsx`, `web/src/pages/{ArchiveDocumentsList,ArchiveDocumentForm,ArchiveDocumentView}.tsx`
- Изменить: `web/src/api.ts` (оба комплекта типов/функций — `ArchiveNode*` и `ArchiveDocument*` — в одном файле, в одном шаге: разделение по задачам оставило бы файл в невалидном промежуточном состоянии), `web/src/App.tsx` (роуты для обеих сущностей), `web/src/pages/EntityCatalog.tsx` (обе строки каталога), `web/src/pages/ArchiveView.tsx` (кросс-ссылка на дерево узлов)

**Важно для исполнителя**: `ArchiveNodesList.tsx` — единственный (кроме `DivisionsList`) список-дерево в программе: обязателен выбор архива (`?archive_id=` в URL или `Select` наверху страницы, `useArchiveOptions`), пока архив не выбран — пусто; дерево — тот же приём, что `DivisionsList.tsx` (root = окна `fetchArchiveNodes` без `parent_id`, отфильтрованные на клиенте до `parent_id == null`, дети — по клику раскрывашки через `loadData`). `ArchiveDocumentsList.tsx` — обычный плоский список (мирроринг `NotesList.tsx`), без обязательных фильтров. `ArchiveNodeForm`'s `parent_id` — редактируемое поле `ArchiveNodePicker`, предзаполненное пропом, когда форма открыта как «добавить дочерний узел» (см. «Предпосылка» — сознательное отличие от `CreateDivisionModal`, где `parent_id` — фиксированный проп, не поле формы). `parent_id`/`node_id`/`unit_id`/`document_id` нигде не входят в `FORM_FIELDS`/`EDIT_FORM_FIELDS` — управляются отдельным `useState`, 422 на них падает в общий `error`/`saveError`. Переносить код ниже как есть, файл за файлом — итоговое содержимое.

### 2.1. `web/src/ArchiveNodePicker.tsx` (создать)

#### `web/src/ArchiveNodePicker.tsx` (создать)
`web/src/ArchiveNodePicker.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Button, Empty, Modal, Select, Space, Tree, Typography } from "antd";
import type { DataNode, EventDataNode } from "antd/es/tree";
import {
  fetchArchiveDocuments,
  fetchArchiveNodes,
  fetchArchives,
  MAX_PAGE_LIMIT,
  type Archive,
  type ArchiveDocument,
  type ArchiveNode,
} from "./api";

function nodeLabel(n: ArchiveNode): string {
  return n.label || n.name || n.id;
}

function toTreeNode(n: ArchiveNode): DataNode {
  return { key: n.id, title: nodeLabel(n) };
}

// updateTreeData — та же иммутабельная рекурсивная подстановка детей узла
// key в дерево antd Tree, что DivisionsList.tsx (скопировано как есть — уже
// проверенная логика, docs/data-model/entity-write.md §3.4).
function updateTreeData(list: DataNode[], key: string, children: DataNode[]): DataNode[] {
  return list.map((node) => {
    if (node.key === key) {
      return { ...node, children, isLeaf: children.length === 0 };
    }
    if (node.children != null) {
      return { ...node, children: updateTreeData(node.children, key, children) };
    }
    return node;
  });
}

// useArchiveOptions — заполняет Select архивов, по образцу
// useRepositoryOptions (ArchiveForm.tsx). Экспортирован — переиспользуется
// ArchiveNodesList.tsx (тот же выбор архива наверху страницы).
export function useArchiveOptions() {
  const [archives, setArchives] = useState<Archive[]>([]);

  useEffect(() => {
    fetchArchives({ limit: 500 })
      .then(setArchives)
      .catch(() => setArchives([]));
  }, []);

  return archives.map((a) => ({ value: a.id, label: a.name }));
}

// ArchiveNodePicker — общий picker для строгих ссылок на ArchiveNode. Четыре
// места использования: ArchiveNodeForm/ArchiveNodeView (parent_id),
// ArchiveDocumentForm/ArchiveDocumentView (unit_id), ретрофит
// AttachmentForm/AttachmentView (node_id) и AnchorEditor (archive-kind
// node_id) — docs/data-model/entity-write.md §3.4/§6.
//
// Кнопка открывает Modal. Если archiveId не передан пропом — сначала нужно
// выбрать архив (Select), пока архив не выбран — дерево не рендерится
// ("Сначала выберите архив"). Если archiveId уже известен вызывающей стороне
// (например, ArchiveNodeView — переродительствование только в пределах
// своего же архива) — шаг выбора архива пропускается, дерево сразу scoped к
// этому archiveId.
//
// Дерево — тот же приём, что DivisionsList.tsx: root = fetchArchiveNodes с
// parent_id не заданным, отфильтрованный на клиенте до parent_id == null
// (бэк отдаёт окна limit/offset, а не «только корни» — проходим все
// страницы до короткой, тот же MAX_PAGE_LIMIT-приём, что
// DivisionsList.loadRoot), дети — по клику раскрывашки через
// loadData/onLoadData, где бэк уже фильтрует по-настоящему.
export function ArchiveNodePicker({
  value,
  label,
  onChange,
  archiveId,
}: {
  value: string;
  label?: string;
  onChange: (id: string, label: string) => void;
  archiveId?: string;
}) {
  const [open, setOpen] = useState(false);
  const [pickedArchiveId, setPickedArchiveId] = useState<string | null>(archiveId ?? null);
  const [treeData, setTreeData] = useState<DataNode[]>([]);
  const [loading, setLoading] = useState(false);
  const archiveOptions = useArchiveOptions();

  const effectiveArchiveId = archiveId ?? pickedArchiveId;

  const loadRoot = async (forArchiveId: string) => {
    setLoading(true);
    try {
      const roots: ArchiveNode[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchArchiveNodes({
          archive_id: forArchiveId,
          limit: MAX_PAGE_LIMIT,
          offset,
        });
        roots.push(...page.filter((n) => n.parent_id == null));
        if (page.length < MAX_PAGE_LIMIT) {
          break;
        }
        offset += MAX_PAGE_LIMIT;
      }
      setTreeData(roots.map(toTreeNode));
    } catch {
      setTreeData([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (open && effectiveArchiveId != null) {
      loadRoot(effectiveArchiveId);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, effectiveArchiveId]);

  const onLoadData = async (node: EventDataNode<DataNode>) => {
    if (effectiveArchiveId == null) {
      return;
    }
    const children = await fetchArchiveNodes({
      archive_id: effectiveArchiveId,
      parent_id: String(node.key),
      limit: MAX_PAGE_LIMIT,
    });
    setTreeData((prev) => updateTreeData(prev, String(node.key), children.map(toTreeNode)));
  };

  const onSelect = (keys: React.Key[], info: { node: DataNode }) => {
    if (keys.length > 0) {
      onChange(String(keys[0]), String(info.node.title ?? keys[0]));
      setOpen(false);
    }
  };

  const openModal = () => {
    setPickedArchiveId(archiveId ?? null);
    setTreeData([]);
    setOpen(true);
  };

  return (
    <>
      {value ? (
        <Space>
          <Typography.Text>Текущий узел: {label ?? value}</Typography.Text>
          <Button size="small" onClick={openModal}>
            Изменить
          </Button>
        </Space>
      ) : (
        <Button type="dashed" onClick={openModal}>
          Выбрать узел
        </Button>
      )}
      <Modal
        title="Выбор архивного узла"
        open={open}
        onCancel={() => setOpen(false)}
        footer={<Button onClick={() => setOpen(false)}>Закрыть</Button>}
        destroyOnHidden
      >
        <Space direction="vertical" style={{ width: "100%" }}>
          {archiveId == null && (
            <Select
              style={{ width: "100%" }}
              placeholder="Выберите архив"
              options={archiveOptions}
              value={pickedArchiveId ?? undefined}
              onChange={(v) => {
                setPickedArchiveId(v);
                setTreeData([]);
              }}
              showSearch
              filterOption={(input, option) =>
                (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
              }
            />
          )}
          {effectiveArchiveId == null ? (
            <Empty description="Сначала выберите архив" />
          ) : loading ? (
            <Typography.Text type="secondary">Загрузка…</Typography.Text>
          ) : (
            <Tree treeData={treeData} loadData={onLoadData} onSelect={onSelect} showLine />
          )}
        </Space>
      </Modal>
    </>
  );
}

// ArchiveDocumentSelect — плоский searchable Select документов внутри
// заданного узла: unit_id === nodeId, отфильтровано на клиенте (тот же
// приём "flat Select, лимит 500, не picker", что у SourceLinkListEditor's
// Select цитат — ArchiveDocument не иерархична, docs/data-model/
// entity-write.md §3.4/§6). Используется рядом с ArchiveNodePicker для
// необязательного document_id-подполя (Attachment.document_id,
// Anchor.document_id).
export function ArchiveDocumentSelect({
  nodeId,
  value,
  onChange,
}: {
  nodeId: string | null | undefined;
  value: string | undefined;
  onChange: (id: string | undefined) => void;
}) {
  const [documents, setDocuments] = useState<ArchiveDocument[]>([]);

  useEffect(() => {
    fetchArchiveDocuments({ limit: MAX_PAGE_LIMIT })
      .then(setDocuments)
      .catch(() => setDocuments([]));
  }, []);

  const options = documents
    .filter((d) => nodeId != null && d.unit_id === nodeId)
    .map((d) => ({ value: d.id, label: `${d.title} (${d.kind || "без вида"})` }));

  return (
    <Select
      style={{ width: 300 }}
      placeholder={nodeId == null ? "Сначала выберите узел" : "Документ (необязательно)"}
      allowClear
      disabled={nodeId == null}
      options={options}
      value={value || undefined}
      onChange={(v) => onChange(v)}
      showSearch
      filterOption={(input, option) =>
        (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
      }
    />
  );
}
```

### 2.2. Страницы ArchiveNode

#### `web/src/pages/ArchiveNodesList.tsx` (создать)
`web/src/pages/ArchiveNodesList.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Empty, Input, List, Select, Spin, Tree, Typography } from "antd";
import type { DataNode, EventDataNode } from "antd/es/tree";
import { fetchArchiveNodes, searchArchiveNodes, MAX_PAGE_LIMIT, type ArchiveNode } from "../api";
import { useArchiveOptions } from "../ArchiveNodePicker";
import { useSession } from "../session";
import { CreateArchiveNodeModal } from "./ArchiveNodeForm";

function nodeLabel(n: ArchiveNode): string {
  return n.label || n.name || n.id;
}

function toTreeNode(n: ArchiveNode): DataNode {
  return { key: n.id, title: nodeLabel(n) };
}

// updateTreeData — та же иммутабельная рекурсивная подстановка детей узла
// key, что DivisionsList.tsx/ArchiveNodePicker.tsx (скопировано как есть).
function updateTreeData(list: DataNode[], key: string, children: DataNode[]): DataNode[] {
  return list.map((node) => {
    if (node.key === key) {
      return { ...node, children, isLeaf: children.length === 0 };
    }
    if (node.children != null) {
      return { ...node, children: updateTreeData(node.children, key, children) };
    }
    return node;
  });
}

// ArchiveNodesList — «Архивные единицы»: тот же tree-паттерн, что
// DivisionsList.tsx (Tree + loadData + поиск-подменяет-дерево), но с
// scoping по обязательному архиву — ArchiveNode не одно глобальное дерево, а
// по одному на архив (docs/data-model/entity-write.md §3.4). Архив читается
// из ?archive_id= URL-параметра (так ArchiveView может вести прямо в дерево
// своего архива, см. "Архивные единицы →" на ArchiveView.tsx) либо
// выбирается тут же Select'ом (useArchiveOptions — тот же хук, что
// ArchiveNodePicker). Пока архив не выбран/не известен — пустое состояние
// вместо дерева.
export default function ArchiveNodesList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [searchParams, setSearchParams] = useSearchParams();
  const archiveOptions = useArchiveOptions();

  const [archiveId, setArchiveId] = useState<string | null>(searchParams.get("archive_id"));
  const [treeData, setTreeData] = useState<DataNode[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<ArchiveNode[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  // loadRoot — как DivisionsList.loadRoot: бэк без parent_id отдаёт узлы
  // окнами (limit/offset), не «только корни», поэтому фильтруем на клиенте
  // по parent_id == null и проходим все страницы до конца.
  const loadRoot = async (forArchiveId: string) => {
    setLoading(true);
    setError(null);
    try {
      const roots: ArchiveNode[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchArchiveNodes({
          archive_id: forArchiveId,
          limit: MAX_PAGE_LIMIT,
          offset,
        });
        roots.push(...page.filter((n) => n.parent_id == null));
        if (page.length < MAX_PAGE_LIMIT) {
          break;
        }
        offset += MAX_PAGE_LIMIT;
      }
      setTreeData(roots.map(toTreeNode));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Не удалось загрузить список");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    setSearchResults(null);
    setError(null);
    if (archiveId != null) {
      loadRoot(archiveId);
    } else {
      setTreeData([]);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [archiveId]);

  const onArchiveChange = (v: string) => {
    setArchiveId(v);
    setSearchParams({ archive_id: v });
  };

  const onLoadData = async (node: EventDataNode<DataNode>) => {
    if (archiveId == null) {
      return;
    }
    try {
      const children = await fetchArchiveNodes({
        archive_id: archiveId,
        parent_id: String(node.key),
        limit: MAX_PAGE_LIMIT,
      });
      setTreeData((prev) => updateTreeData(prev, String(node.key), children.map(toTreeNode)));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Не удалось загрузить дочерние узлы");
      throw e;
    }
  };

  const onSelect = (keys: React.Key[]) => {
    if (keys.length > 0) {
      navigate(`/archive-nodes/${keys[0]}`);
    }
  };

  // onSearch — archive-nodes/search ГЛОБАЛЬНЫЙ (не ограничен archive_id, см.
  // searchArchiveNodes в api.ts) — результаты, найденные в других архивах,
  // здесь не релевантны (страница показывает дерево ОДНОГО выбранного
  // архива), поэтому фильтруем на клиенте до archiveId после запроса.
  const onSearch = (value: string) => {
    const q = value.trim();
    if (!q || archiveId == null) {
      setSearchResults(null);
      return;
    }
    setSearching(true);
    setError(null);
    searchArchiveNodes({ q, limit: MAX_PAGE_LIMIT })
      .then((results) => setSearchResults(results.filter((n) => n.archive_id === archiveId)))
      .catch((e: Error) => setError(e.message))
      .finally(() => setSearching(false));
  };

  const onSearchChange = (value: string) => {
    if (value.trim() === "") {
      setSearchResults(null);
    }
  };

  return (
    <>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "Архивные единицы" }]}
      />
      <Card
        title="Архивные единицы"
        extra={
          session != null && archiveId != null ? (
            <Button type="primary" onClick={() => setCreateOpen(true)}>
              + добавить в корень
            </Button>
          ) : undefined
        }
      >
        <Select
          style={{ width: "100%", marginBottom: 16 }}
          placeholder="Выберите архив"
          options={archiveOptions}
          value={archiveId ?? undefined}
          onChange={onArchiveChange}
          showSearch
          filterOption={(input, option) =>
            (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
          }
        />
        {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
        {archiveId == null ? (
          <Empty description="Сначала выберите архив" />
        ) : (
          <>
            <Input.Search
              placeholder="Поиск по метке/названию…"
              allowClear
              enterButton
              loading={searching}
              onSearch={onSearch}
              onChange={(e) => onSearchChange(e.target.value)}
              style={{ marginBottom: 16 }}
            />
            {searchResults != null ? (
              <List
                dataSource={searchResults}
                locale={{ emptyText: "Найдено пусто" }}
                renderItem={(n) => (
                  <List.Item>
                    <Typography.Link onClick={() => navigate(`/archive-nodes/${n.id}`)}>
                      {nodeLabel(n)}
                    </Typography.Link>
                  </List.Item>
                )}
              />
            ) : loading ? (
              <Spin />
            ) : (
              <Tree treeData={treeData} loadData={onLoadData} onSelect={onSelect} showLine />
            )}
          </>
        )}
        {archiveId != null && (
          <CreateArchiveNodeModal
            open={createOpen}
            archiveId={archiveId}
            parentId={null}
            onClose={() => setCreateOpen(false)}
            onCreated={(n) => {
              setCreateOpen(false);
              navigate(`/archive-nodes/${n.id}`);
            }}
          />
        )}
      </Card>
    </>
  );
}
```

#### `web/src/pages/ArchiveNodeForm.tsx` (создать)
`web/src/pages/ArchiveNodeForm.tsx`:
```tsx
import { useState } from "react";
import { Alert, Checkbox, Form, Input, Modal } from "antd";
import {
  createArchiveNode,
  type ArchiveNode,
  type FactDate,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError } from "../auth";
import { ArchiveNodePicker } from "../ArchiveNodePicker";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";
import { FactDateEditor } from "../FactDateEditor";

interface ArchiveNodeFormValues {
  type: string;
  label: string;
  name?: string;
  parishText?: string;
  private?: boolean;
}

const FORM_FIELDS: (keyof ArchiveNodeFormValues)[] = ["type", "label"];

// CreateArchiveNodeModal — форма создания архивного узла. archive_id —
// фиксирован пропом (модалка всегда открывается в известном контексте
// архива — со страницы ArchiveNodesList или из ArchiveNodeView) и НЕ
// редактируется здесь. parent_id — ArchiveNodePicker, scoped к этому же
// archiveId (родитель только в пределах своего архива — тот же инвариант,
// что проверяет сервер, docs/data-model/entity-write.md §3.4);
// предзаполняется пропом parentId (не null — «добавить дочерний узел», см.
// ArchiveNodesList/ArchiveNodeView), но остаётся редактируемым через
// picker — в отличие от CreateDivisionModal, где parent_id полем формы не
// является вовсе (там ArchiveNode уже умеет предлагать полноценный picker,
// у Division его нет).
export function CreateArchiveNodeModal({
  open,
  archiveId,
  parentId,
  onClose,
  onCreated,
}: {
  open: boolean;
  archiveId: string;
  parentId: string | null;
  onClose: () => void;
  onCreated: (created: ArchiveNode) => void;
}) {
  const [form] = Form.useForm<ArchiveNodeFormValues>();
  const [editParentId, setEditParentId] = useState<string | null>(parentId);
  const [editParentLabel, setEditParentLabel] = useState<string | null>(null);
  const [settlements, setSettlements] = useState<TextRef[]>([]);
  const [since, setSince] = useState<FactDate | null>(null);
  const [until, setUntil] = useState<FactDate | null>(null);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setEditParentId(parentId);
    setEditParentLabel(null);
    setSettlements([]);
    setSince(null);
    setUntil(null);
    setNotes([]);
    setSources([]);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: ArchiveNodeFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const parishText = (values.parishText ?? "").trim();
      const created = await createArchiveNode({
        type: values.type,
        archive_id: archiveId,
        parent_id: editParentId,
        label: values.label,
        name: values.name ?? "",
        since,
        until,
        parish: parishText ? { text: parishText } : null,
        settlements,
        notes,
        sources,
        private: values.private ?? false,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof ArchiveNodeFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать узел");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title={parentId == null ? "Добавить архивный узел" : "Добавить дочерний узел"}
      open={open}
      onCancel={handleClose}
      onOk={() => form.submit()}
      okText="Создать"
      cancelText="Отмена"
      confirmLoading={submitting}
      destroyOnHidden
    >
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
      <Form form={form} layout="vertical" onFinish={onFinish} initialValues={{ private: false }}>
        <Form.Item
          name="type"
          label="Тип"
          rules={[{ required: true, whitespace: true, message: "Введите тип (например, fond/opis/delo)" }]}
        >
          <Input placeholder="fond / opis / delo" autoFocus />
        </Form.Item>
        <Form.Item label="Родитель">
          <ArchiveNodePicker
            value={editParentId ?? ""}
            label={editParentLabel ?? undefined}
            archiveId={archiveId}
            onChange={(id, lbl) => {
              setEditParentId(id);
              setEditParentLabel(lbl);
            }}
          />
        </Form.Item>
        <Form.Item
          name="label"
          label="Метка"
          rules={[{ required: true, whitespace: true, message: "Введите метку" }]}
        >
          <Input placeholder="Фонд 1 / Опись 1 / Дело 1" />
        </Form.Item>
        <Form.Item name="name" label="Название">
          <Input />
        </Form.Item>
        <Form.Item name="parishText" label="Приход (текстом)">
          <Input placeholder="Никольский приход" />
        </Form.Item>
        <Form.Item label="Начало периода">
          <FactDateEditor value={since} onChange={setSince} addLabel="+ дата начала" />
        </Form.Item>
        <Form.Item label="Конец периода">
          <FactDateEditor value={until} onChange={setUntil} addLabel="+ дата конца" />
        </Form.Item>
        <Form.Item label="Населённые пункты">
          <TextRefListEditor value={settlements} onChange={setSettlements} addLabel="+ населённый пункт" />
        </Form.Item>
        <Form.Item label="Заметки">
          <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
        </Form.Item>
        <Form.Item label="Доказательства">
          <SourceLinkListEditor value={sources} onChange={setSources} addLabel="+ доказательство" />
        </Form.Item>
        <Form.Item name="private" valuePropName="checked">
          <Checkbox>Приватная запись</Checkbox>
        </Form.Item>
      </Form>
    </Modal>
  );
}
```

#### `web/src/pages/ArchiveNodeView.tsx` (создать)
`web/src/pages/ArchiveNodeView.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  Alert,
  Breadcrumb,
  Button,
  Card,
  Checkbox,
  Descriptions,
  Form,
  Input,
  List,
  Modal,
  Popconfirm,
  Space,
  Spin,
  Typography,
} from "antd";
import {
  deleteArchiveNode,
  fetchArchive,
  fetchArchiveNode,
  fetchArchiveNodes,
  updateArchiveNode,
  MAX_PAGE_LIMIT,
  type Archive,
  type ArchiveNode,
  type FactDate,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { ArchiveNodePicker } from "../ArchiveNodePicker";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";
import { FactDateEditor, formatFactDate } from "../FactDateEditor";
import { CreateArchiveNodeModal } from "./ArchiveNodeForm";

function nodeLabel(n: ArchiveNode): string {
  return n.label || n.name || n.id;
}

interface EditFormValues {
  type: string;
  label: string;
  name?: string;
  parishText?: string;
  private?: boolean;
}

// EDIT_FORM_FIELDS — как в DivisionView.tsx: parent_id управляется отдельным
// состоянием (editParentId, не полем antd Form), поэтому 422 на parent_id
// (цикл, чужой архив) попадает в общий saveError, а не в конкретное поле
// формы — тот же выбор, что и у DivisionView.
const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["type", "label"];

function TextRefListView({ items }: { items: TextRef[] }) {
  if (items.length === 0) {
    return <Typography.Text type="secondary">—</Typography.Text>;
  }
  return (
    <Space direction="vertical" size={0}>
      {items.map((t, i) => (
        <Typography.Text key={i}>
          {t.text}
          {t.ref != null && t.ref !== "" && (
            <Typography.Text type="secondary"> → {t.type} {t.ref}</Typography.Text>
          )}
        </Typography.Text>
      ))}
    </Space>
  );
}

function SourceLinkListView({ items }: { items: SourceLink[] }) {
  if (items.length === 0) {
    return <Typography.Text type="secondary">—</Typography.Text>;
  }
  return (
    <List
      size="small"
      dataSource={items}
      renderItem={(s) => (
        <List.Item>
          <Link to={`/citations/${s.citation_id}`}>citation {s.citation_id}</Link>
          {s.role ? ` — ${s.role}` : ""}
        </List.Item>
      )}
    />
  );
}

// ArchiveNodeView — просмотр архивного узла, переключаемый в форму
// редактирования на той же странице (toggle+explicit-save, канонический
// вид — ArchiveView.tsx/DivisionView.tsx). Дополнительно к общему шаблону:
// ссылка на владеющий Archive и, если parent_id задан, на родительский
// узел; поле «Родитель» в режиме редактирования — ArchiveNodePicker, scoped
// к archive_id ЭТОГО узла (переродительствование только в пределах своего
// же архива — тот же инвариант, что и на create, сервер бы всё равно
// отверг чужой архив 422, docs/data-model/entity-write.md §3.4).
export default function ArchiveNodeView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [node, setNode] = useState<ArchiveNode | null>(null);
  const [archive, setArchive] = useState<Archive | null>(null);
  const [parent, setParent] = useState<ArchiveNode | null>(null);
  const [children, setChildren] = useState<ArchiveNode[]>([]);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [editParentId, setEditParentId] = useState<string | null>(null);
  const [editParentLabel, setEditParentLabel] = useState<string | null>(null);
  const [settlements, setSettlements] = useState<TextRef[]>([]);
  const [since, setSince] = useState<FactDate | null>(null);
  const [until, setUntil] = useState<FactDate | null>(null);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [addChildOpen, setAddChildOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (nodeId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setNode(null);
    setArchive(null);
    setParent(null);
    setChildren([]);
    fetchArchiveNode(nodeId)
      .then((n) => {
        setNode(n);
        const archivePromise = fetchArchive(n.archive_id)
          .then(setArchive)
          .catch(() => setArchive(null));
        const parentPromise =
          n.parent_id != null
            ? fetchArchiveNode(n.parent_id)
                .then(setParent)
                .catch(() => setParent(null))
            : Promise.resolve(setParent(null));
        const childrenPromise = fetchArchiveNodes({
          archive_id: n.archive_id,
          parent_id: nodeId,
          limit: MAX_PAGE_LIMIT,
        }).then(setChildren);
        return Promise.all([archivePromise, parentPromise, childrenPromise]);
      })
      .catch((e) => {
        if (e instanceof ApiError && e.status === 404) {
          setNotFound(true);
        } else {
          setError(e instanceof Error ? e.message : "Не удалось загрузить узел");
        }
      })
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    setEditing(false);
    setSaveError(null);
    if (id != null) {
      load(id);
    }
  }, [id]);

  const startEdit = () => {
    if (node == null) {
      return;
    }
    form.setFieldsValue({
      type: node.type,
      label: node.label,
      name: node.name ?? "",
      parishText: node.parish?.text ?? "",
      private: node.private,
    });
    setEditParentId(node.parent_id ?? null);
    setEditParentLabel(parent != null ? nodeLabel(parent) : null);
    setSettlements(node.settlements);
    setSince(node.since ?? null);
    setUntil(node.until ?? null);
    setNotes(node.notes);
    setSources(node.sources);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (node == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const parishText = (values.parishText ?? "").trim();
      const parishHasRef = node.parish?.ref != null && node.parish.ref !== "";
      const updated = await updateArchiveNode(node.id, {
        type: values.type,
        archive_id: node.archive_id,
        parent_id: editParentId,
        label: values.label,
        name: values.name ?? "",
        since,
        until,
        parish: parishHasRef && parishText === node.parish?.text
          ? node.parish
          : parishText
            ? { text: parishText }
            : null,
        settlements,
        notes,
        sources,
        private: values.private ?? false,
      });
      setNode(updated);
      if (updated.parent_id != null) {
        fetchArchiveNode(updated.parent_id)
          .then(setParent)
          .catch(() => setParent(null));
      } else {
        setParent(null);
      }
      setEditing(false);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (EDIT_FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof EditFormValues, errors: [e.message] }]);
      } else {
        setSaveError(e instanceof ApiError ? e.message : "Не удалось сохранить изменения");
      }
    } finally {
      setSaving(false);
    }
  };

  const onDelete = async () => {
    if (node == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteArchiveNode(node.id);
      navigate(
        node.parent_id != null
          ? `/archive-nodes/${node.parent_id}`
          : `/archive-nodes?archive_id=${encodeURIComponent(node.archive_id)}`,
      );
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        setConflict(e.referrers ?? []);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось удалить узел");
      }
    } finally {
      setDeleting(false);
    }
  };

  if (loading) {
    return <Spin />;
  }

  if (notFound) {
    return (
      <Alert
        type="warning"
        showIcon
        message="Узел не найден"
        description="Возможно, его удалили. Вернитесь к списку."
        action={
          <Link to="/archive-nodes">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (node == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to={`/archive-nodes?archive_id=${encodeURIComponent(node.archive_id)}`}>Архивные единицы</Link> },
          ...(parent != null
            ? [{ title: <Link to={`/archive-nodes/${parent.id}`}>{nodeLabel(parent)}</Link> }]
            : []),
          { title: nodeLabel(node) },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={nodeLabel(node)} column={1} bordered size="small">
            <Descriptions.Item label="Тип">{node.type}</Descriptions.Item>
            <Descriptions.Item label="Архив">
              <Link to={`/archives/${node.archive_id}`}>{archive?.name ?? node.archive_id}</Link>
            </Descriptions.Item>
            <Descriptions.Item label="Родитель">
              {parent != null ? (
                <Link to={`/archive-nodes/${parent.id}`}>{nodeLabel(parent)}</Link>
              ) : (
                <Typography.Text type="secondary">корень</Typography.Text>
              )}
            </Descriptions.Item>
            <Descriptions.Item label="Название">{node.name || "—"}</Descriptions.Item>
            <Descriptions.Item label="Приход">
              {node.parish == null ? (
                "—"
              ) : (
                <>
                  {node.parish.text}
                  {node.parish.ref && (
                    <Typography.Text type="secondary"> → {node.parish.type} {node.parish.ref}</Typography.Text>
                  )}
                </>
              )}
            </Descriptions.Item>
            <Descriptions.Item label="Начало периода">{formatFactDate(node.since)}</Descriptions.Item>
            <Descriptions.Item label="Конец периода">{formatFactDate(node.until)}</Descriptions.Item>
            <Descriptions.Item label="Населённые пункты"><TextRefListView items={node.settlements} /></Descriptions.Item>
            <Descriptions.Item label="Заметки"><TextRefListView items={node.notes} /></Descriptions.Item>
            <Descriptions.Item label="Доказательства"><SourceLinkListView items={node.sources} /></Descriptions.Item>
            <Descriptions.Item label="Приватная">{node.private ? "да" : "нет"}</Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Button onClick={() => setAddChildOpen(true)}>+ добавить дочерний узел</Button>
              <Popconfirm
                title={`Удалить «${nodeLabel(node)}»?`}
                description="Действие необратимо."
                okText="Удалить"
                cancelText="Отмена"
                onConfirm={onDelete}
              >
                <Button danger loading={deleting}>
                  Удалить
                </Button>
              </Popconfirm>
            </Space>
          )}
        </>
      ) : (
        <Form form={form} layout="vertical" onFinish={onSave} style={{ maxWidth: 560 }}>
          {saveError != null && (
            <Alert type="error" showIcon message={saveError} style={{ marginBottom: 16 }} />
          )}
          <Form.Item
            name="type"
            label="Тип"
            rules={[{ required: true, whitespace: true, message: "Введите тип" }]}
          >
            <Input placeholder="fond / opis / delo" />
          </Form.Item>
          <Form.Item label="Родитель">
            <ArchiveNodePicker
              value={editParentId ?? ""}
              label={editParentLabel ?? undefined}
              archiveId={node.archive_id}
              onChange={(pid, lbl) => {
                setEditParentId(pid);
                setEditParentLabel(lbl);
              }}
            />
            {editParentId != null && (
              <Button
                size="small"
                style={{ marginLeft: 8 }}
                onClick={() => {
                  setEditParentId(null);
                  setEditParentLabel(null);
                }}
              >
                Сделать корневым
              </Button>
            )}
          </Form.Item>
          <Form.Item
            name="label"
            label="Метка"
            rules={[{ required: true, whitespace: true, message: "Введите метку" }]}
          >
            <Input />
          </Form.Item>
          <Form.Item name="name" label="Название">
            <Input />
          </Form.Item>
          <Form.Item name="parishText" label="Приход (текстом)">
            <Input placeholder="Никольский приход" />
          </Form.Item>
          <Form.Item label="Начало периода">
            <FactDateEditor value={since} onChange={setSince} addLabel="+ дата начала" />
          </Form.Item>
          <Form.Item label="Конец периода">
            <FactDateEditor value={until} onChange={setUntil} addLabel="+ дата конца" />
          </Form.Item>
          <Form.Item label="Населённые пункты">
            <TextRefListEditor value={settlements} onChange={setSettlements} addLabel="+ населённый пункт" />
          </Form.Item>
          <Form.Item label="Заметки">
            <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
          </Form.Item>
          <Form.Item label="Доказательства">
            <SourceLinkListEditor value={sources} onChange={setSources} addLabel="+ доказательство" />
          </Form.Item>
          <Form.Item name="private" valuePropName="checked">
            <Checkbox>Приватная запись</Checkbox>
          </Form.Item>
          <Space>
            <Button type="primary" htmlType="submit" loading={saving}>
              Сохранить
            </Button>
            <Button onClick={cancelEdit}>Отмена</Button>
          </Space>
        </Form>
      )}

      <Typography.Title level={5} style={{ marginTop: 24 }}>
        Дочерние узлы
      </Typography.Title>
      <List
        dataSource={children}
        locale={{ emptyText: "Дочерних узлов нет" }}
        renderItem={(c) => (
          <List.Item>
            <Link to={`/archive-nodes/${c.id}`}>{nodeLabel(c)}</Link>
          </List.Item>
        )}
      />

      <CreateArchiveNodeModal
        open={addChildOpen}
        archiveId={node.archive_id}
        parentId={node.id}
        onClose={() => setAddChildOpen(false)}
        onCreated={() => {
          setAddChildOpen(false);
          fetchArchiveNodes({ archive_id: node.archive_id, parent_id: node.id, limit: MAX_PAGE_LIMIT })
            .then(setChildren)
            .catch(() => {});
        }}
      />

      <Modal
        title="Узел используется"
        open={conflict != null}
        onCancel={() => setConflict(null)}
        footer={<Button onClick={() => setConflict(null)}>Закрыть</Button>}
      >
        <Typography.Paragraph>
          Нельзя удалить — на узел ссылаются другие сущности:
        </Typography.Paragraph>
        <List
          size="small"
          dataSource={conflict ?? []}
          renderItem={(r) => (
            <List.Item>
              {r.type} {r.id}
            </List.Item>
          )}
        />
      </Modal>
    </Card>
  );
}
```

### 2.3. Страницы ArchiveDocument

#### `web/src/pages/ArchiveDocumentsList.tsx` (создать)
`web/src/pages/ArchiveDocumentsList.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Input, List, Spin } from "antd";
import { fetchArchiveDocuments, searchArchiveDocuments, MAX_PAGE_LIMIT, type ArchiveDocument } from "../api";
import { useSession } from "../session";
import { CreateArchiveDocumentModal } from "./ArchiveDocumentForm";

function documentLabel(d: ArchiveDocument): string {
  return d.title || d.id;
}

// ArchiveDocumentsList — «Архивные документы»: плоский список (в отличие от
// ArchiveNode — ArchiveDocument не иерархична), тот же
// пагинация-до-короткой-страницы + поиск-подменяет-список, что NotesList.tsx.
export default function ArchiveDocumentsList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<ArchiveDocument[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<ArchiveDocument[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: ArchiveDocument[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchArchiveDocuments({ limit: MAX_PAGE_LIMIT, offset });
        all.push(...page);
        if (page.length < MAX_PAGE_LIMIT) {
          break;
        }
        offset += MAX_PAGE_LIMIT;
      }
      setItems(all);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Не удалось загрузить список");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const onSearch = (value: string) => {
    const q = value.trim();
    if (!q) {
      setSearchResults(null);
      return;
    }
    setSearching(true);
    setError(null);
    searchArchiveDocuments({ q, limit: MAX_PAGE_LIMIT })
      .then(setSearchResults)
      .catch((e: Error) => setError(e.message))
      .finally(() => setSearching(false));
  };

  const onSearchChange = (value: string) => {
    if (value.trim() === "") {
      setSearchResults(null);
    }
  };

  const shown = searchResults ?? items;

  return (
    <>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "Архивные документы" }]}
      />
      <Card
        title="Архивные документы"
        extra={
          session != null ? (
            <Button type="primary" onClick={() => setCreateOpen(true)}>
              + добавить
            </Button>
          ) : undefined
        }
      >
        <Input.Search
          placeholder="Поиск по заголовку…"
          allowClear
          enterButton
          loading={searching}
          onSearch={onSearch}
          onChange={(e) => onSearchChange(e.target.value)}
          style={{ marginBottom: 16 }}
        />
        {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
        {loading && searchResults == null ? (
          <Spin />
        ) : (
          <List
            dataSource={shown}
            locale={{ emptyText: "Список пуст" }}
            renderItem={(d) => (
              <List.Item>
                <Link to={`/archive-documents/${d.id}`}>{documentLabel(d)}</Link>
              </List.Item>
            )}
          />
        )}
        <CreateArchiveDocumentModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(d) => {
            setCreateOpen(false);
            navigate(`/archive-documents/${d.id}`);
          }}
        />
      </Card>
    </>
  );
}
```

#### `web/src/pages/ArchiveDocumentForm.tsx` (создать)
`web/src/pages/ArchiveDocumentForm.tsx`:
```tsx
import { useState } from "react";
import { Alert, Checkbox, Form, Input, Modal } from "antd";
import {
  createArchiveDocument,
  type ArchiveDocument,
  type FactDate,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError } from "../auth";
import { ArchiveNodePicker } from "../ArchiveNodePicker";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";
import { FactDateEditor } from "../FactDateEditor";

interface ArchiveDocumentFormValues {
  title: string;
  kind?: string;
  parishText?: string;
  private?: boolean;
}

const FORM_FIELDS: (keyof ArchiveDocumentFormValues)[] = ["title", "kind"];

// CreateArchiveDocumentModal — форма создания архивного документа (образец
// разбиения — NoteForm.tsx/NoteView.tsx: отдельная create-модалка, View —
// свой файл с toggle-редактированием). unit_id — обязательная строгая
// ссылка на ArchiveNode через ArchiveNodePicker БЕЗ заранее известного
// archiveId: свободностоящее создание документа не привязано к конкретному
// архиву заранее — picker сперва просит выбрать архив, потом узел внутри
// него (docs/data-model/entity-write.md §3.4/§6).
export function CreateArchiveDocumentModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: ArchiveDocument) => void;
}) {
  const [form] = Form.useForm<ArchiveDocumentFormValues>();
  const [unitId, setUnitId] = useState("");
  const [unitLabel, setUnitLabel] = useState<string | null>(null);
  const [settlements, setSettlements] = useState<TextRef[]>([]);
  const [since, setSince] = useState<FactDate | null>(null);
  const [until, setUntil] = useState<FactDate | null>(null);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setUnitId("");
    setUnitLabel(null);
    setSettlements([]);
    setSince(null);
    setUntil(null);
    setNotes([]);
    setSources([]);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: ArchiveDocumentFormValues) => {
    if (!unitId) {
      setError("Выберите единицу хранения");
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      const parishText = (values.parishText ?? "").trim();
      const created = await createArchiveDocument({
        unit_id: unitId,
        title: values.title,
        kind: values.kind ?? "",
        since,
        until,
        parish: parishText ? { text: parishText } : null,
        settlements,
        notes,
        sources,
        private: values.private ?? false,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof ArchiveDocumentFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать документ");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить архивный документ"
      open={open}
      onCancel={handleClose}
      onOk={() => form.submit()}
      okText="Создать"
      cancelText="Отмена"
      confirmLoading={submitting}
      destroyOnHidden
    >
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
      <Form form={form} layout="vertical" onFinish={onFinish} initialValues={{ private: false }}>
        <Form.Item label="Единица хранения" required>
          <ArchiveNodePicker
            value={unitId}
            label={unitLabel ?? undefined}
            onChange={(nid, lbl) => {
              setUnitId(nid);
              setUnitLabel(lbl);
            }}
          />
        </Form.Item>
        <Form.Item
          name="title"
          label="Заголовок"
          rules={[{ required: true, whitespace: true, message: "Введите заголовок" }]}
        >
          <Input autoFocus />
        </Form.Item>
        <Form.Item name="kind" label="Вид">
          <Input placeholder="метрическая книга / исповедная роспись" />
        </Form.Item>
        <Form.Item name="parishText" label="Приход (текстом)">
          <Input placeholder="Никольский приход" />
        </Form.Item>
        <Form.Item label="Начало периода">
          <FactDateEditor value={since} onChange={setSince} addLabel="+ дата начала" />
        </Form.Item>
        <Form.Item label="Конец периода">
          <FactDateEditor value={until} onChange={setUntil} addLabel="+ дата конца" />
        </Form.Item>
        <Form.Item label="Населённые пункты">
          <TextRefListEditor value={settlements} onChange={setSettlements} addLabel="+ населённый пункт" />
        </Form.Item>
        <Form.Item label="Заметки">
          <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
        </Form.Item>
        <Form.Item label="Доказательства">
          <SourceLinkListEditor value={sources} onChange={setSources} addLabel="+ доказательство" />
        </Form.Item>
        <Form.Item name="private" valuePropName="checked">
          <Checkbox>Приватная запись</Checkbox>
        </Form.Item>
      </Form>
    </Modal>
  );
}
```

#### `web/src/pages/ArchiveDocumentView.tsx` (создать)
`web/src/pages/ArchiveDocumentView.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  Alert,
  Breadcrumb,
  Button,
  Card,
  Checkbox,
  Descriptions,
  Form,
  Input,
  List,
  Modal,
  Popconfirm,
  Space,
  Spin,
  Typography,
} from "antd";
import {
  deleteArchiveDocument,
  fetchArchiveDocument,
  fetchArchiveNode,
  updateArchiveDocument,
  type ArchiveDocument,
  type ArchiveNode,
  type FactDate,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { ArchiveNodePicker } from "../ArchiveNodePicker";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";
import { FactDateEditor, formatFactDate } from "../FactDateEditor";

function documentLabel(d: ArchiveDocument): string {
  return d.title || d.id;
}

function nodeLabel(n: ArchiveNode): string {
  return n.label || n.name || n.id;
}

interface EditFormValues {
  title: string;
  kind?: string;
  parishText?: string;
  private?: boolean;
}

// EDIT_FORM_FIELDS — unit_id управляется отдельным состоянием (editUnitId,
// не полем antd Form, как parent_id в ArchiveNodeView.tsx), поэтому 422 на
// unit_id попадает в общий saveError.
const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["title", "kind"];

function TextRefListView({ items }: { items: TextRef[] }) {
  if (items.length === 0) {
    return <Typography.Text type="secondary">—</Typography.Text>;
  }
  return (
    <Space direction="vertical" size={0}>
      {items.map((t, i) => (
        <Typography.Text key={i}>
          {t.text}
          {t.ref != null && t.ref !== "" && (
            <Typography.Text type="secondary"> → {t.type} {t.ref}</Typography.Text>
          )}
        </Typography.Text>
      ))}
    </Space>
  );
}

function SourceLinkListView({ items }: { items: SourceLink[] }) {
  if (items.length === 0) {
    return <Typography.Text type="secondary">—</Typography.Text>;
  }
  return (
    <List
      size="small"
      dataSource={items}
      renderItem={(s) => (
        <List.Item>
          <Link to={`/citations/${s.citation_id}`}>citation {s.citation_id}</Link>
          {s.role ? ` — ${s.role}` : ""}
        </List.Item>
      )}
    />
  );
}

// ArchiveDocumentView — просмотр архивного документа, переключаемый в форму
// редактирования на той же странице (toggle+explicit-save, канонический
// вид — ArchiveView.tsx/NoteView.tsx). Дополнительно к общему шаблону —
// ссылка на владеющий ArchiveNode («Единица хранения»); unit_id в режиме
// редактирования — ArchiveNodePicker БЕЗ заранее известного archiveId (тот
// же выбор, что в ArchiveDocumentForm.tsx — переносить документ можно и в
// узел другого архива, сервер лишь проверяет существование узла, не
// принадлежность архиву, в отличие от ArchiveNode.parent_id).
export default function ArchiveDocumentView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [doc, setDocument] = useState<ArchiveDocument | null>(null);
  const [unit, setUnit] = useState<ArchiveNode | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [editUnitId, setEditUnitId] = useState("");
  const [editUnitLabel, setEditUnitLabel] = useState<string | null>(null);
  const [settlements, setSettlements] = useState<TextRef[]>([]);
  const [since, setSince] = useState<FactDate | null>(null);
  const [until, setUntil] = useState<FactDate | null>(null);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (documentId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setDocument(null);
    setUnit(null);
    fetchArchiveDocument(documentId)
      .then((d) => {
        setDocument(d);
        return fetchArchiveNode(d.unit_id)
          .then(setUnit)
          .catch(() => setUnit(null));
      })
      .catch((e) => {
        if (e instanceof ApiError && e.status === 404) {
          setNotFound(true);
        } else {
          setError(e instanceof Error ? e.message : "Не удалось загрузить документ");
        }
      })
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    setEditing(false);
    setSaveError(null);
    if (id != null) {
      load(id);
    }
  }, [id]);

  const startEdit = () => {
    if (doc == null) {
      return;
    }
    form.setFieldsValue({
      title: doc.title,
      kind: doc.kind ?? "",
      parishText: doc.parish?.text ?? "",
      private: doc.private,
    });
    setEditUnitId(doc.unit_id);
    setEditUnitLabel(unit != null ? nodeLabel(unit) : doc.unit_id);
    setSettlements(doc.settlements);
    setSince(doc.since ?? null);
    setUntil(doc.until ?? null);
    setNotes(doc.notes);
    setSources(doc.sources);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (doc == null) {
      return;
    }
    if (!editUnitId) {
      setSaveError("Выберите единицу хранения");
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const parishText = (values.parishText ?? "").trim();
      const parishHasRef = doc.parish?.ref != null && doc.parish.ref !== "";
      const updated = await updateArchiveDocument(doc.id, {
        unit_id: editUnitId,
        title: values.title,
        kind: values.kind ?? "",
        since,
        until,
        parish: parishHasRef && parishText === doc.parish?.text
          ? doc.parish
          : parishText
            ? { text: parishText }
            : null,
        settlements,
        notes,
        sources,
        private: values.private ?? false,
      });
      setDocument(updated);
      fetchArchiveNode(updated.unit_id)
        .then(setUnit)
        .catch(() => setUnit(null));
      setEditing(false);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (EDIT_FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof EditFormValues, errors: [e.message] }]);
      } else {
        setSaveError(e instanceof ApiError ? e.message : "Не удалось сохранить изменения");
      }
    } finally {
      setSaving(false);
    }
  };

  const onDelete = async () => {
    if (doc == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteArchiveDocument(doc.id);
      navigate("/archive-documents");
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        setConflict(e.referrers ?? []);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось удалить документ");
      }
    } finally {
      setDeleting(false);
    }
  };

  if (loading) {
    return <Spin />;
  }

  if (notFound) {
    return (
      <Alert
        type="warning"
        showIcon
        message="Документ не найден"
        description="Возможно, его удалили. Вернитесь к списку."
        action={
          <Link to="/archive-documents">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (doc == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/archive-documents">Архивные документы</Link> },
          { title: documentLabel(doc) },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={documentLabel(doc)} column={1} bordered size="small">
            <Descriptions.Item label="Вид">{doc.kind || "—"}</Descriptions.Item>
            <Descriptions.Item label="Единица хранения">
              <Link to={`/archive-nodes/${doc.unit_id}`}>{unit != null ? nodeLabel(unit) : doc.unit_id}</Link>
            </Descriptions.Item>
            <Descriptions.Item label="Приход">
              {doc.parish == null ? (
                "—"
              ) : (
                <>
                  {doc.parish.text}
                  {doc.parish.ref && (
                    <Typography.Text type="secondary"> → {doc.parish.type} {doc.parish.ref}</Typography.Text>
                  )}
                </>
              )}
            </Descriptions.Item>
            <Descriptions.Item label="Начало периода">{formatFactDate(doc.since)}</Descriptions.Item>
            <Descriptions.Item label="Конец периода">{formatFactDate(doc.until)}</Descriptions.Item>
            <Descriptions.Item label="Населённые пункты"><TextRefListView items={doc.settlements} /></Descriptions.Item>
            <Descriptions.Item label="Заметки"><TextRefListView items={doc.notes} /></Descriptions.Item>
            <Descriptions.Item label="Доказательства"><SourceLinkListView items={doc.sources} /></Descriptions.Item>
            <Descriptions.Item label="Приватная">{doc.private ? "да" : "нет"}</Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title={`Удалить «${documentLabel(doc)}»?`}
                description="Действие необратимо."
                okText="Удалить"
                cancelText="Отмена"
                onConfirm={onDelete}
              >
                <Button danger loading={deleting}>
                  Удалить
                </Button>
              </Popconfirm>
            </Space>
          )}
        </>
      ) : (
        <Form form={form} layout="vertical" onFinish={onSave} style={{ maxWidth: 560 }}>
          {saveError != null && (
            <Alert type="error" showIcon message={saveError} style={{ marginBottom: 16 }} />
          )}
          <Form.Item label="Единица хранения" required>
            <ArchiveNodePicker
              value={editUnitId}
              label={editUnitLabel ?? undefined}
              onChange={(nid, lbl) => {
                setEditUnitId(nid);
                setEditUnitLabel(lbl);
              }}
            />
          </Form.Item>
          <Form.Item
            name="title"
            label="Заголовок"
            rules={[{ required: true, whitespace: true, message: "Введите заголовок" }]}
          >
            <Input />
          </Form.Item>
          <Form.Item name="kind" label="Вид">
            <Input placeholder="метрическая книга / исповедная роспись" />
          </Form.Item>
          <Form.Item name="parishText" label="Приход (текстом)">
            <Input placeholder="Никольский приход" />
          </Form.Item>
          <Form.Item label="Начало периода">
            <FactDateEditor value={since} onChange={setSince} addLabel="+ дата начала" />
          </Form.Item>
          <Form.Item label="Конец периода">
            <FactDateEditor value={until} onChange={setUntil} addLabel="+ дата конца" />
          </Form.Item>
          <Form.Item label="Населённые пункты">
            <TextRefListEditor value={settlements} onChange={setSettlements} addLabel="+ населённый пункт" />
          </Form.Item>
          <Form.Item label="Заметки">
            <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
          </Form.Item>
          <Form.Item label="Доказательства">
            <SourceLinkListEditor value={sources} onChange={setSources} addLabel="+ доказательство" />
          </Form.Item>
          <Form.Item name="private" valuePropName="checked">
            <Checkbox>Приватная запись</Checkbox>
          </Form.Item>
          <Space>
            <Button type="primary" htmlType="submit" loading={saving}>
              Сохранить
            </Button>
            <Button onClick={cancelEdit}>Отмена</Button>
          </Space>
        </Form>
      )}

      <Modal
        title="Документ используется"
        open={conflict != null}
        onCancel={() => setConflict(null)}
        footer={<Button onClick={() => setConflict(null)}>Закрыть</Button>}
      >
        <Typography.Paragraph>
          Нельзя удалить — на документ ссылаются другие сущности:
        </Typography.Paragraph>
        <List
          size="small"
          dataSource={conflict ?? []}
          renderItem={(r) => (
            <List.Item>
              {r.type} {r.id}
            </List.Item>
          )}
        />
      </Modal>
    </Card>
  );
}
```

### 2.4. `web/src/api.ts` — типы и функции ArchiveNode/ArchiveDocument

#### `web/src/api.ts` (изменить — итоговое содержимое)
`web/src/api.ts`:
```ts
import { authFetch } from "./auth";

export interface AdminDivision {
  id: string;
  name: string;
  type: string;
  parent_id: string | null;
  sources: SourceLink[];
}

// AdminDivisionType — models.AdminDivisionType (internal/models/administrative_division_type.go).
export type AdminDivisionType =
  | "governorate"
  | "district"
  | "volost"
  | "other"
  | "gorod"
  | "selo"
  | "derevnya"
  | "hutor"
  | "pogost"
  | "stanitsa"
  | "mestechko";

export const ADMIN_DIVISION_TYPE_LABELS: Record<AdminDivisionType, string> = {
  governorate: "Губерния",
  district: "Уезд",
  volost: "Волость",
  other: "Иное",
  gorod: "Город",
  selo: "Село",
  derevnya: "Деревня",
  hutor: "Хутор",
  pogost: "Погост",
  stanitsa: "Станица",
  mestechko: "Местечко",
};

export function adminDivisionTypeLabel(t: string): string {
  return ADMIN_DIVISION_TYPE_LABELS[t as AdminDivisionType] ?? t;
}

// AdminDivisionInput — тело POST/PUT /api/admin-divisions (transport.AdminDivisionCreate
// и transport.AdminDivisionUpdate имеют одинаковую форму: полная замена name/type/parent_id;
// с подпроекта 5 сюда же входит sources — тоже полная замена).
export interface AdminDivisionInput {
  name: string;
  type: AdminDivisionType;
  parent_id: string | null;
  sources: SourceLink[];
}

export interface AdminDivisionQuery {
  kind?: "settlement";
  type?: string;
  parent_id?: string;
  limit?: number;
  offset?: number;
}

export interface DivisionSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

// Максимальный размер окна списка (совпадает с MaxPageLimit на сервере).
export const MAX_PAGE_LIMIT = 500;

export async function fetchAdminDivisions(
  query: AdminDivisionQuery = {},
): Promise<AdminDivision[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/admin-divisions${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchAdminDivisions(
  query: DivisionSearchQuery,
): Promise<AdminDivision[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/admin-divisions/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

// fetchDivision — GET /api/admin-divisions/{id}, открыто анонимному
// посетителю (auth.md §6). Через authFetch — ради ApiError (нужен код 404
// на странице View: единицу могли удалить в другой вкладке).
export async function fetchDivision(id: string): Promise<AdminDivision> {
  return authFetch<AdminDivision>(`/api/admin-divisions/${encodeURIComponent(id)}`);
}

// createDivision/updateDivision/deleteDivision — запись, только для
// вошедшего владельца (requireFull на сервере, internal/httpapi/division_write.go).
export async function createDivision(input: AdminDivisionInput): Promise<AdminDivision> {
  return authFetch<AdminDivision>("/api/admin-divisions", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateDivision(id: string, input: AdminDivisionInput): Promise<AdminDivision> {
  return authFetch<AdminDivision>(`/api/admin-divisions/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteDivision(id: string): Promise<void> {
  return authFetch<void>(`/api/admin-divisions/${encodeURIComponent(id)}`, { method: "DELETE" });
}

// TextRef — контракт элемента списков вроде Surname.variants: текст или
// ссылка на другую сущность (transport.TextRef). v1-формы редактируют
// только text; ref/type только читаются (docs/data-model/entity-write.md §4).
export interface TextRef {
  text: string;
  ref?: string;
  type?: string;
}

export interface Surname {
  id: string;
  canonical: string;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

// SurnameInput — тело POST/PUT /api/surnames (transport.SurnameCreate и
// transport.SurnameUpdate имеют одинаковую форму: полная замена всех полей).
export interface SurnameInput {
  canonical: string;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

export interface SurnameQuery {
  limit?: number;
  offset?: number;
}

export interface SurnameSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchSurnames(query: SurnameQuery = {}): Promise<Surname[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/surnames${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchSurnames(query: SurnameSearchQuery): Promise<Surname[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/surnames/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

// fetchSurname — GET /api/surnames/{id}, открыто анонимному посетителю. Через
// authFetch — ради ApiError (нужен код 404 на странице View).
export async function fetchSurname(id: string): Promise<Surname> {
  return authFetch<Surname>(`/api/surnames/${encodeURIComponent(id)}`);
}

// createSurname/updateSurname/deleteSurname — запись, только для вошедшего
// владельца (requireFull на сервере, internal/httpapi/surname_write.go).
export async function createSurname(input: SurnameInput): Promise<Surname> {
  return authFetch<Surname>("/api/surnames", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateSurname(id: string, input: SurnameInput): Promise<Surname> {
  return authFetch<Surname>(`/api/surnames/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteSurname(id: string): Promise<void> {
  return authFetch<void>(`/api/surnames/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export interface DocFile {
  path: string;
  title: string;
}

export interface DocListResponse {
  files: DocFile[];
}

export async function fetchDocList(): Promise<DocFile[]> {
  const resp = await fetch("/api/docs");
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  const data: DocListResponse = await resp.json();
  return data.files;
}

export async function fetchDoc(path: string): Promise<string> {
  const resp = await fetch(`/api/docs/${encodeURIComponent(path)}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.text();
}

export interface Patronymic {
  id: string;
  canonical: string;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

// PatronymicInput — тело POST/PUT /api/patronymics (transport.PatronymicCreate и
// transport.PatronymicUpdate имеют одинаковую форму: полная замена всех полей).
export interface PatronymicInput {
  canonical: string;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

export interface PatronymicQuery {
  limit?: number;
  offset?: number;
}

export interface PatronymicSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchPatronymics(query: PatronymicQuery = {}): Promise<Patronymic[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/patronymics${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchPatronymics(query: PatronymicSearchQuery): Promise<Patronymic[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/patronymics/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

// fetchPatronymic — GET /api/patronymics/{id}, открыто анонимному посетителю. Через
// authFetch — ради ApiError (нужен код 404 на странице View).
export async function fetchPatronymic(id: string): Promise<Patronymic> {
  return authFetch<Patronymic>(`/api/patronymics/${encodeURIComponent(id)}`);
}

// createPatronymic/updatePatronymic/deletePatronymic — запись, только для вошедшего
// владельца (requireFull на сервере, internal/httpapi/patronymic_write.go).
export async function createPatronymic(input: PatronymicInput): Promise<Patronymic> {
  return authFetch<Patronymic>("/api/patronymics", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updatePatronymic(id: string, input: PatronymicInput): Promise<Patronymic> {
  return authFetch<Patronymic>(`/api/patronymics/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deletePatronymic(id: string): Promise<void> {
  return authFetch<void>(`/api/patronymics/${encodeURIComponent(id)}`, { method: "DELETE" });
}


export interface Estate {
  id: string;
  canonical: string;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

// EstateInput — тело POST/PUT /api/estates (transport.EstateCreate и
// transport.EstateUpdate имеют одинаковую форму: полная замена всех полей).
export interface EstateInput {
  canonical: string;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

export interface EstateQuery {
  limit?: number;
  offset?: number;
}

export interface EstateSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchEstates(query: EstateQuery = {}): Promise<Estate[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/estates${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchEstates(query: EstateSearchQuery): Promise<Estate[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/estates/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

// fetchEstate — GET /api/estates/{id}, открыто анонимному посетителю. Через
// authFetch — ради ApiError (нужен код 404 на странице View).
export async function fetchEstate(id: string): Promise<Estate> {
  return authFetch<Estate>(`/api/estates/${encodeURIComponent(id)}`);
}

// createEstate/updateEstate/deleteEstate — запись, только для вошедшего
// владельца (requireFull на сервере, internal/httpapi/estate_write.go).
export async function createEstate(input: EstateInput): Promise<Estate> {
  return authFetch<Estate>("/api/estates", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateEstate(id: string, input: EstateInput): Promise<Estate> {
  return authFetch<Estate>(`/api/estates/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteEstate(id: string): Promise<void> {
  return authFetch<void>(`/api/estates/${encodeURIComponent(id)}`, { method: "DELETE" });
}


export interface Title {
  id: string;
  canonical: string;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

// TitleInput — тело POST/PUT /api/titles (transport.TitleCreate и
// transport.TitleUpdate имеют одинаковую форму: полная замена всех полей).
export interface TitleInput {
  canonical: string;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

export interface TitleQuery {
  limit?: number;
  offset?: number;
}

export interface TitleSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchTitles(query: TitleQuery = {}): Promise<Title[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/titles${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchTitles(query: TitleSearchQuery): Promise<Title[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/titles/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

// fetchTitle — GET /api/titles/{id}, открыто анонимному посетителю. Через
// authFetch — ради ApiError (нужен код 404 на странице View).
export async function fetchTitle(id: string): Promise<Title> {
  return authFetch<Title>(`/api/titles/${encodeURIComponent(id)}`);
}

// createTitle/updateTitle/deleteTitle — запись, только для вошедшего
// владельца (requireFull на сервере, internal/httpapi/title_write.go).
export async function createTitle(input: TitleInput): Promise<Title> {
  return authFetch<Title>("/api/titles", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateTitle(id: string, input: TitleInput): Promise<Title> {
  return authFetch<Title>(`/api/titles/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteTitle(id: string): Promise<void> {
  return authFetch<void>(`/api/titles/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export type NameGender = "male" | "female" | "neutral";

export interface GivenName {
  id: string;
  canonical: string;
  gender: NameGender | "";
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

// GivenNameInput — тело POST/PUT /api/given-names (transport.GivenNameCreate и
// transport.GivenNameUpdate имеют одинаковую форму: полная замена всех полей).
export interface GivenNameInput {
  canonical: string;
  gender: NameGender;
  variants: TextRef[];
  items: TextRef[];
  notes: TextRef[];
}

export interface GivenNameQuery {
  limit?: number;
  offset?: number;
}

export interface GivenNameSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchGivenNames(query: GivenNameQuery = {}): Promise<GivenName[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/given-names${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchGivenNames(query: GivenNameSearchQuery): Promise<GivenName[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/given-names/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

// fetchGivenName — GET /api/given-names/{id}, открыто анонимному посетителю. Через
// authFetch — ради ApiError (нужен код 404 на странице View).
export async function fetchGivenName(id: string): Promise<GivenName> {
  return authFetch<GivenName>(`/api/given-names/${encodeURIComponent(id)}`);
}

// createGivenName/updateGivenName/deleteGivenName — запись, только для вошедшего
// владельца (requireFull на сервере, internal/httpapi/given_name_write.go).
export async function createGivenName(input: GivenNameInput): Promise<GivenName> {
  return authFetch<GivenName>("/api/given-names", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateGivenName(id: string, input: GivenNameInput): Promise<GivenName> {
  return authFetch<GivenName>(`/api/given-names/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteGivenName(id: string): Promise<void> {
  return authFetch<void>(`/api/given-names/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export interface SourceLink {
  citation_id: string;
  target_type?: string;
  target_id?: string;
  reliability?: string;
  role?: string;
  note?: string;
}

// FactDate — контракт структурированной даты (transport.FactDate): год/
// месяц/день, точность, формулировка, календарь, верхняя граница периода для
// modifier=between. Первое появление в проекте (Parish.Since/Until).
export type FactPrecision = "unknown" | "year" | "month" | "day";
export type FactModifier = "exact" | "approx" | "before" | "after" | "between";
export type FactCalendar = "" | "gregorian" | "julian" | "unknown";

export interface FactDate {
  year: number;
  month?: number;
  day?: number;
  precision: FactPrecision;
  modifier: FactModifier;
  calendar?: FactCalendar;
  year_to?: number;
  month_to?: number;
  day_to?: number;
}

export interface Repository {
  id: string;
  name: string;
  type: string;
  address?: string;
  urls: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

// RepositoryInput — тело POST/PUT /api/repositories (transport.RepositoryCreate
// и transport.RepositoryUpdate имеют одинаковую форму: полная замена всех
// полей, включая sources — редактируется с подпроекта 5).
export interface RepositoryInput {
  name: string;
  type: string;
  address?: string;
  urls: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

export interface RepositoryQuery {
  limit?: number;
  offset?: number;
}

export interface RepositorySearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchRepositories(query: RepositoryQuery = {}): Promise<Repository[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/repositories${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchRepositories(query: RepositorySearchQuery): Promise<Repository[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/repositories/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchRepository(id: string): Promise<Repository> {
  return authFetch<Repository>(`/api/repositories/${encodeURIComponent(id)}`);
}

export async function createRepository(input: RepositoryInput): Promise<Repository> {
  return authFetch<Repository>("/api/repositories", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateRepository(id: string, input: RepositoryInput): Promise<Repository> {
  return authFetch<Repository>(`/api/repositories/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteRepository(id: string): Promise<void> {
  return authFetch<void>(`/api/repositories/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export interface Church {
  id: string;
  name: string;
  parish?: TextRef | null;
  settlements: TextRef[];
  variants: string[];
  notes: TextRef[];
  sources: SourceLink[];
}

export interface ChurchInput {
  name: string;
  parish?: TextRef | null;
  settlements: TextRef[];
  variants: string[];
  notes: TextRef[];
  sources: SourceLink[];
}

export interface ChurchQuery {
  limit?: number;
  offset?: number;
}

export interface ChurchSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchChurches(query: ChurchQuery = {}): Promise<Church[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/churches${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchChurches(query: ChurchSearchQuery): Promise<Church[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/churches/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchChurch(id: string): Promise<Church> {
  return authFetch<Church>(`/api/churches/${encodeURIComponent(id)}`);
}

export async function createChurch(input: ChurchInput): Promise<Church> {
  return authFetch<Church>("/api/churches", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateChurch(id: string, input: ChurchInput): Promise<Church> {
  return authFetch<Church>(`/api/churches/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteChurch(id: string): Promise<void> {
  return authFetch<void>(`/api/churches/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export interface Parish {
  id: string;
  name: string;
  church?: TextRef | null;
  settlements: TextRef[];
  since?: FactDate | null;
  until?: FactDate | null;
  notes: TextRef[];
  sources: SourceLink[];
}

export interface ParishInput {
  name: string;
  church?: TextRef | null;
  settlements: TextRef[];
  since?: FactDate | null;
  until?: FactDate | null;
  notes: TextRef[];
  sources: SourceLink[];
}

export interface ParishQuery {
  limit?: number;
  offset?: number;
}

export interface ParishSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchParishes(query: ParishQuery = {}): Promise<Parish[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/parishes${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchParishes(query: ParishSearchQuery): Promise<Parish[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/parishes/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchParish(id: string): Promise<Parish> {
  return authFetch<Parish>(`/api/parishes/${encodeURIComponent(id)}`);
}

export async function createParish(input: ParishInput): Promise<Parish> {
  return authFetch<Parish>("/api/parishes", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateParish(id: string, input: ParishInput): Promise<Parish> {
  return authFetch<Parish>(`/api/parishes/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteParish(id: string): Promise<void> {
  return authFetch<void>(`/api/parishes/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export interface Archive {
  id: string;
  name: string;
  system?: TextRef | null;
  repository_id?: string;
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

// ArchiveInput — repository_id — просто id (не TextRef, в отличие от
// system/parish/church у других сущностей): пустая строка — без хранилища.
export interface ArchiveInput {
  name: string;
  system?: TextRef | null;
  repository_id?: string;
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

export interface ArchiveQuery {
  limit?: number;
  offset?: number;
}

export interface ArchiveSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchArchives(query: ArchiveQuery = {}): Promise<Archive[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/archives${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchArchives(query: ArchiveSearchQuery): Promise<Archive[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/archives/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchArchive(id: string): Promise<Archive> {
  return authFetch<Archive>(`/api/archives/${encodeURIComponent(id)}`);
}

export async function createArchive(input: ArchiveInput): Promise<Archive> {
  return authFetch<Archive>("/api/archives", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateArchive(id: string, input: ArchiveInput): Promise<Archive> {
  return authFetch<Archive>(`/api/archives/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteArchive(id: string): Promise<void> {
  return authFetch<void>(`/api/archives/${encodeURIComponent(id)}`, { method: "DELETE" });
}

// Note — заметка (markdown-текст с иерархией «книга → главы»). parent_id —
// просто id родительской заметки (не TextRef — строгая self-ref ссылка, как
// у Archive.repository_id); пустая строка — без родителя. sources
// редактируется с подпроекта 5 (Citation теперь имеет CRUD).
export interface Note {
  id: string;
  kind: string;
  title?: string;
  text?: string;
  parent_id?: string;
  sources: SourceLink[];
  private: boolean;
}

export interface NoteInput {
  kind: string;
  title?: string;
  text?: string;
  parent_id?: string;
  sources: SourceLink[];
  private: boolean;
}

export interface NoteQuery {
  limit?: number;
  offset?: number;
}

export interface NoteSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchNotes(query: NoteQuery = {}): Promise<Note[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/notes${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchNotes(query: NoteSearchQuery): Promise<Note[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/notes/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchNote(id: string): Promise<Note> {
  return authFetch<Note>(`/api/notes/${encodeURIComponent(id)}`);
}

export async function createNote(input: NoteInput): Promise<Note> {
  return authFetch<Note>("/api/notes", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateNote(id: string, input: NoteInput): Promise<Note> {
  return authFetch<Note>(`/api/notes/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteNote(id: string): Promise<void> {
  return authFetch<void>(`/api/notes/${encodeURIComponent(id)}`, { method: "DELETE" });
}

// Attachment — файловое вложение. node_id — обязательная строгая ссылка на
// архивный узел (просто id). document_id — необязательная мягкая ссылка (ON
// DELETE SET NULL). Оба поля редактируются через ArchiveNodePicker/
// ArchiveDocumentSelect (AttachmentForm.tsx/AttachmentView.tsx, подпроект 6).
export interface Attachment {
  id: string;
  kind: string;
  uri?: string;
  filename?: string;
  mime?: string;
  page?: number;
  node_id: string;
  document_id?: string;
  note?: string;
  private: boolean;
}

export interface AttachmentInput {
  kind: string;
  uri?: string;
  filename?: string;
  mime?: string;
  page?: number;
  node_id: string;
  document_id?: string;
  note?: string;
  private: boolean;
}

export interface AttachmentQuery {
  limit?: number;
  offset?: number;
}

export interface AttachmentSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchAttachments(query: AttachmentQuery = {}): Promise<Attachment[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/attachments${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchAttachments(query: AttachmentSearchQuery): Promise<Attachment[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/attachments/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchAttachment(id: string): Promise<Attachment> {
  return authFetch<Attachment>(`/api/attachments/${encodeURIComponent(id)}`);
}

export async function createAttachment(input: AttachmentInput): Promise<Attachment> {
  return authFetch<Attachment>("/api/attachments", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateAttachment(id: string, input: AttachmentInput): Promise<Attachment> {
  return authFetch<Attachment>(`/api/attachments/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteAttachment(id: string): Promise<void> {
  return authFetch<void>(`/api/attachments/${encodeURIComponent(id)}`, { method: "DELETE" });
}

export type Reliability = "primary" | "contemporary" | "memory" | "indirect" | "unknown";

// Anchor — контракт полиморфной привязки «где именно» (transport.Anchor):
// плоское представление с дискриминатором kind, по образцу FactDate. undefined
// — привязки нет (цитата может относиться к источнику целиком).
export type AnchorKind = "archive" | "file" | "url";

export interface Anchor {
  kind: AnchorKind;
  node_id?: string;
  document_id?: string;
  page?: number;
  rect?: string;
  attachment_id?: string;
  timecode?: string;
  url?: string;
}

// Source — источник доказательства. date — структурированная дата (см.
// FactDate). repository_id — просто id (мягкая ссылка, необязательна).
export interface Source {
  id: string;
  kind: string;
  title: string;
  author?: string;
  date?: FactDate | null;
  reliability: Reliability;
  repository_id?: string;
  notes: TextRef[];
  private: boolean;
}

export interface SourceInput {
  kind: string;
  title: string;
  author?: string;
  date?: FactDate | null;
  reliability: Reliability;
  repository_id?: string;
  notes: TextRef[];
  private: boolean;
}

export interface SourceQuery {
  limit?: number;
  offset?: number;
}

export interface SourceSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchSources(query: SourceQuery = {}): Promise<Source[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/sources${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchSources(query: SourceSearchQuery): Promise<Source[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/sources/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchSource(id: string): Promise<Source> {
  return authFetch<Source>(`/api/sources/${encodeURIComponent(id)}`);
}

export async function createSource(input: SourceInput): Promise<Source> {
  return authFetch<Source>("/api/sources", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateSource(id: string, input: SourceInput): Promise<Source> {
  return authFetch<Source>(`/api/sources/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteSource(id: string): Promise<void> {
  return authFetch<void>(`/api/sources/${encodeURIComponent(id)}`, { method: "DELETE" });
}

// Citation — цитата из источника. anchor — необязательная полиморфная
// привязка «где именно» (см. Anchor).
export interface Citation {
  id: string;
  source_id: string;
  anchor?: Anchor | null;
  text?: string;
  note?: string;
  private: boolean;
}

export interface CitationInput {
  source_id: string;
  anchor?: Anchor | null;
  text?: string;
  note?: string;
  private: boolean;
}

export interface CitationQuery {
  limit?: number;
  offset?: number;
}

export interface CitationSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchCitations(query: CitationQuery = {}): Promise<Citation[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/citations${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchCitations(query: CitationSearchQuery): Promise<Citation[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/citations/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchCitation(id: string): Promise<Citation> {
  return authFetch<Citation>(`/api/citations/${encodeURIComponent(id)}`);
}

export async function createCitation(input: CitationInput): Promise<Citation> {
  return authFetch<Citation>("/api/citations", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateCitation(id: string, input: CitationInput): Promise<Citation> {
  return authFetch<Citation>(`/api/citations/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteCitation(id: string): Promise<void> {
  return authFetch<void>(`/api/citations/${encodeURIComponent(id)}`, { method: "DELETE" });
}

// ArchiveNode — узел архивного дерева (transport.ArchiveNode). Дерево
// скопировано по обязательному ArchiveID — не одно глобальное дерево, как
// AdminDivision, а по одному на архив (docs/data-model/entity-write.md §3.4).
// ParentID, если задан, — родитель В ПРЕДЕЛАХ ТОГО ЖЕ архива (сервер это
// проверяет — 422 на parent_id, если родитель из другого архива).
export interface ArchiveNode {
  id: string;
  type: string;
  archive_id: string;
  parent_id?: string | null;
  label: string;
  name: string;
  since?: FactDate | null;
  until?: FactDate | null;
  parish?: TextRef | null;
  settlements: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

export interface ArchiveNodeInput {
  type: string;
  archive_id: string;
  parent_id?: string | null;
  label: string;
  name: string;
  since?: FactDate | null;
  until?: FactDate | null;
  parish?: TextRef | null;
  settlements: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

// ArchiveNodeQuery — archive_id ОБЯЗАТЕЛЕН (сервер отдаёт 400 без него, узел
// бессмысленен вне архива). parent_id — как у AdminDivisionQuery: не задан —
// корень (внутри архива), иначе — прямые дети.
export interface ArchiveNodeQuery {
  archive_id: string;
  parent_id?: string;
  limit?: number;
  offset?: number;
}

export interface ArchiveNodeSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchArchiveNodes(query: ArchiveNodeQuery): Promise<ArchiveNode[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/archive-nodes${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

// searchArchiveNodes — GET /api/archive-nodes/search?q=. Глобальный поиск, НЕ
// ограничен archive_id (в отличие от fetchArchiveNodes) — сужение по архиву,
// если нужно, делает вызывающая сторона на клиенте (см. ArchiveNodesList.tsx).
export async function searchArchiveNodes(query: ArchiveNodeSearchQuery): Promise<ArchiveNode[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/archive-nodes/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchArchiveNode(id: string): Promise<ArchiveNode> {
  return authFetch<ArchiveNode>(`/api/archive-nodes/${encodeURIComponent(id)}`);
}

export async function createArchiveNode(input: ArchiveNodeInput): Promise<ArchiveNode> {
  return authFetch<ArchiveNode>("/api/archive-nodes", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateArchiveNode(id: string, input: ArchiveNodeInput): Promise<ArchiveNode> {
  return authFetch<ArchiveNode>(`/api/archive-nodes/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteArchiveNode(id: string): Promise<void> {
  return authFetch<void>(`/api/archive-nodes/${encodeURIComponent(id)}`, { method: "DELETE" });
}

// ArchiveDocument — документ внутри единицы учёта (transport.ArchiveDocument).
// В отличие от ArchiveNode — плоская сущность, без собственной иерархии;
// unit_id — обязательная строгая ссылка на ArchiveNode.
export interface ArchiveDocument {
  id: string;
  unit_id: string;
  title: string;
  kind: string;
  since?: FactDate | null;
  until?: FactDate | null;
  parish?: TextRef | null;
  settlements: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

export interface ArchiveDocumentInput {
  unit_id: string;
  title: string;
  kind: string;
  since?: FactDate | null;
  until?: FactDate | null;
  parish?: TextRef | null;
  settlements: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

export interface ArchiveDocumentQuery {
  limit?: number;
  offset?: number;
}

export interface ArchiveDocumentSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchArchiveDocuments(
  query: ArchiveDocumentQuery = {},
): Promise<ArchiveDocument[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/archive-documents${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchArchiveDocuments(
  query: ArchiveDocumentSearchQuery,
): Promise<ArchiveDocument[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/archive-documents/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchArchiveDocument(id: string): Promise<ArchiveDocument> {
  return authFetch<ArchiveDocument>(`/api/archive-documents/${encodeURIComponent(id)}`);
}

export async function createArchiveDocument(input: ArchiveDocumentInput): Promise<ArchiveDocument> {
  return authFetch<ArchiveDocument>("/api/archive-documents", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateArchiveDocument(
  id: string,
  input: ArchiveDocumentInput,
): Promise<ArchiveDocument> {
  return authFetch<ArchiveDocument>(`/api/archive-documents/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteArchiveDocument(id: string): Promise<void> {
  return authFetch<void>(`/api/archive-documents/${encodeURIComponent(id)}`, { method: "DELETE" });
}
```

### 2.5. Роуты, каталог, кросс-ссылка

#### `web/src/App.tsx` (изменить — итоговое содержимое)
`web/src/App.tsx`:
```tsx
import { BrowserRouter, Routes, Route, Navigate } from "react-router-dom";
import type { ReactNode } from "react";
import { Layout } from "antd";
import { SessionProvider } from "./session";
import { AppHeader } from "./AppHeader";
import DocsPanel from "./docs-panel";
import EntityCatalog from "./pages/EntityCatalog";
import DivisionsList from "./pages/DivisionsList";
import DivisionView from "./pages/DivisionView";
import SurnamesList from "./pages/SurnamesList";
import SurnameView from "./pages/SurnameView";
import PatronymicsList from "./pages/PatronymicsList";
import PatronymicView from "./pages/PatronymicView";
import EstatesList from "./pages/EstatesList";
import EstateView from "./pages/EstateView";
import TitlesList from "./pages/TitlesList";
import TitleView from "./pages/TitleView";
import GivenNamesList from "./pages/GivenNamesList";
import GivenNameView from "./pages/GivenNameView";
import RepositoriesList from "./pages/RepositoriesList";
import RepositoryView from "./pages/RepositoryView";
import ChurchesList from "./pages/ChurchesList";
import ChurchView from "./pages/ChurchView";
import ParishesList from "./pages/ParishesList";
import ParishView from "./pages/ParishView";
import ArchivesList from "./pages/ArchivesList";
import ArchiveView from "./pages/ArchiveView";
import ArchiveNodesList from "./pages/ArchiveNodesList";
import ArchiveNodeView from "./pages/ArchiveNodeView";
import ArchiveDocumentsList from "./pages/ArchiveDocumentsList";
import ArchiveDocumentView from "./pages/ArchiveDocumentView";
import NotesList from "./pages/NotesList";
import NoteView from "./pages/NoteView";
import AttachmentsList from "./pages/AttachmentsList";
import AttachmentView from "./pages/AttachmentView";
import SourcesList from "./pages/SourcesList";
import SourceView from "./pages/SourceView";
import CitationsList from "./pages/CitationsList";
import CitationView from "./pages/CitationView";
import LoginPage from "./pages/Login";
import RegisterPage from "./pages/Register";
import SettingsPage from "./pages/Settings";

const { Content } = Layout;

// PageLayout — общая рамка (шапка + отступы) для каждой страницы каталога.
// Раньше все страницы делили один AppContent с Tabs; теперь у каждой —
// собственный роут, а переключение между ними — через каталог сущностей
// (/) + хлебные крошки на каждой странице, не вкладки.
function PageLayout({ children }: { children: ReactNode }) {
  return (
    <Layout style={{ minHeight: "100vh" }}>
      <AppHeader />
      <Content style={{ padding: 24 }}>{children}</Content>
    </Layout>
  );
}

export default function App() {
  return (
    <BrowserRouter>
      <SessionProvider>
        <Routes>
          <Route path="/" element={<PageLayout><EntityCatalog /></PageLayout>} />
          <Route path="/docs" element={<PageLayout><DocsPanel /></PageLayout>} />
          <Route path="/docs/:docPath*" element={<PageLayout><DocsPanel /></PageLayout>} />
          <Route path="/divisions" element={<PageLayout><DivisionsList /></PageLayout>} />
          <Route path="/divisions/:id" element={<PageLayout><DivisionView /></PageLayout>} />
          <Route path="/surnames" element={<PageLayout><SurnamesList /></PageLayout>} />
          <Route path="/surnames/:id" element={<PageLayout><SurnameView /></PageLayout>} />
          <Route path="/patronymics" element={<PageLayout><PatronymicsList /></PageLayout>} />
          <Route path="/patronymics/:id" element={<PageLayout><PatronymicView /></PageLayout>} />
          <Route path="/estates" element={<PageLayout><EstatesList /></PageLayout>} />
          <Route path="/estates/:id" element={<PageLayout><EstateView /></PageLayout>} />
          <Route path="/titles" element={<PageLayout><TitlesList /></PageLayout>} />
          <Route path="/titles/:id" element={<PageLayout><TitleView /></PageLayout>} />
          <Route path="/given-names" element={<PageLayout><GivenNamesList /></PageLayout>} />
          <Route path="/given-names/:id" element={<PageLayout><GivenNameView /></PageLayout>} />
          <Route path="/repositories" element={<PageLayout><RepositoriesList /></PageLayout>} />
          <Route path="/repositories/:id" element={<PageLayout><RepositoryView /></PageLayout>} />
          <Route path="/churches" element={<PageLayout><ChurchesList /></PageLayout>} />
          <Route path="/churches/:id" element={<PageLayout><ChurchView /></PageLayout>} />
          <Route path="/parishes" element={<PageLayout><ParishesList /></PageLayout>} />
          <Route path="/parishes/:id" element={<PageLayout><ParishView /></PageLayout>} />
          <Route path="/archives" element={<PageLayout><ArchivesList /></PageLayout>} />
          <Route path="/archives/:id" element={<PageLayout><ArchiveView /></PageLayout>} />
          <Route path="/archive-nodes" element={<PageLayout><ArchiveNodesList /></PageLayout>} />
          <Route path="/archive-nodes/:id" element={<PageLayout><ArchiveNodeView /></PageLayout>} />
          <Route path="/archive-documents" element={<PageLayout><ArchiveDocumentsList /></PageLayout>} />
          <Route path="/archive-documents/:id" element={<PageLayout><ArchiveDocumentView /></PageLayout>} />
          <Route path="/notes" element={<PageLayout><NotesList /></PageLayout>} />
          <Route path="/notes/:id" element={<PageLayout><NoteView /></PageLayout>} />
          <Route path="/attachments" element={<PageLayout><AttachmentsList /></PageLayout>} />
          <Route path="/attachments/:id" element={<PageLayout><AttachmentView /></PageLayout>} />
          <Route path="/sources" element={<PageLayout><SourcesList /></PageLayout>} />
          <Route path="/sources/:id" element={<PageLayout><SourceView /></PageLayout>} />
          <Route path="/citations" element={<PageLayout><CitationsList /></PageLayout>} />
          <Route path="/citations/:id" element={<PageLayout><CitationView /></PageLayout>} />
          <Route path="/login" element={<LoginPage />} />
          <Route path="/register" element={<RegisterPage />} />
          <Route path="/settings" element={<SettingsPage />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </SessionProvider>
    </BrowserRouter>
  );
}
```

#### `web/src/pages/EntityCatalog.tsx` (изменить — итоговое содержимое)
`web/src/pages/EntityCatalog.tsx`:
```tsx
import { Card, List, Typography } from "antd";
import { Link } from "react-router-dom";

// CATALOG_ENTRIES — единая точка входа приложения: по алфавиту названия.
// Каждая сущность получает свой список при подключении (docs/data-model/
// entity-write.md §4) — здесь просто добавляется новая строка, без вкладок.
const CATALOG_ENTRIES: { label: string; path: string }[] = [
  { label: "Административное деление", path: "/divisions" },
  { label: "Архивные документы", path: "/archive-documents" },
  { label: "Архивные единицы", path: "/archive-nodes" },
  { label: "Архивы", path: "/archives" },
  { label: "Вложения", path: "/attachments" },
  { label: "Документация", path: "/docs" },
  { label: "Заметки", path: "/notes" },
  { label: "Имена", path: "/given-names" },
  { label: "Отчества", path: "/patronymics" },
  { label: "Источники", path: "/sources" },
  { label: "Приходы", path: "/parishes" },
  { label: "Сословия", path: "/estates" },
  { label: "Титулы", path: "/titles" },
  { label: "Фамилии", path: "/surnames" },
  { label: "Хранилища", path: "/repositories" },
  { label: "Церкви", path: "/churches" },
  { label: "Цитаты", path: "/citations" },
];

const SORTED_ENTRIES = [...CATALOG_ENTRIES].sort((a, b) => a.label.localeCompare(b.label, "ru"));

export default function EntityCatalog() {
  return (
    <Card title="Сущности">
      <List
        dataSource={SORTED_ENTRIES}
        renderItem={(entry) => (
          <List.Item>
            <Link to={entry.path}>
              <Typography.Text>{entry.label}</Typography.Text>
            </Link>
          </List.Item>
        )}
      />
    </Card>
  );
}
```

#### `web/src/pages/ArchiveView.tsx` (изменить — итоговое содержимое)
`web/src/pages/ArchiveView.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  Alert,
  Breadcrumb,
  Button,
  Card,
  Checkbox,
  Descriptions,
  Form,
  Input,
  List,
  Modal,
  Popconfirm,
  Select,
  Space,
  Spin,
  Typography,
} from "antd";
import {
  deleteArchive,
  fetchArchive,
  fetchRepositories,
  updateArchive,
  type Archive,
  type Repository,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";

interface EditFormValues {
  name: string;
  systemText?: string;
  repository_id?: string;
  private?: boolean;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["name", "repository_id"];

function TextRefListView({ items }: { items: TextRef[] }) {
  if (items.length === 0) {
    return <Typography.Text type="secondary">—</Typography.Text>;
  }
  return (
    <Space direction="vertical" size={0}>
      {items.map((t, i) => (
        <Typography.Text key={i}>
          {t.text}
          {t.ref != null && t.ref !== "" && (
            <Typography.Text type="secondary"> → {t.type} {t.ref}</Typography.Text>
          )}
        </Typography.Text>
      ))}
    </Space>
  );
}

function SourceLinkListView({ items }: { items: SourceLink[] }) {
  if (items.length === 0) {
    return <Typography.Text type="secondary">—</Typography.Text>;
  }
  return (
    <List
      size="small"
      dataSource={items}
      renderItem={(s) => (
        <List.Item>
          <Link to={`/citations/${s.citation_id}`}>citation {s.citation_id}</Link>
          {s.role ? ` — ${s.role}` : ""}
        </List.Item>
      )}
    />
  );
}

// ArchiveView — просмотр архива, переключаемый в форму редактирования на той
// же странице (toggle+explicit-save). repository_id — Select со списком
// хранилищ (fetchRepositories), не TextRef.
export default function ArchiveView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [archive, setArchive] = useState<Archive | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [repositories, setRepositories] = useState<Repository[]>([]);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (archiveId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setArchive(null);
    fetchArchive(archiveId)
      .then(setArchive)
      .catch((e) => {
        if (e instanceof ApiError && e.status === 404) {
          setNotFound(true);
        } else {
          setError(e instanceof Error ? e.message : "Не удалось загрузить запись");
        }
      })
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    setEditing(false);
    setSaveError(null);
    if (id != null) {
      load(id);
    }
  }, [id]);

  useEffect(() => {
    fetchRepositories({ limit: 500 })
      .then(setRepositories)
      .catch(() => setRepositories([]));
  }, []);

  const repositoryOptions = repositories.map((r) => ({ value: r.id, label: r.name }));
  const repositoryName = (repoID?: string) =>
    repositories.find((r) => r.id === repoID)?.name ?? repoID;

  const startEdit = () => {
    if (archive == null) {
      return;
    }
    form.setFieldsValue({
      name: archive.name,
      systemText: archive.system?.text ?? "",
      repository_id: archive.repository_id ?? undefined,
      private: archive.private,
    });
    setNotes(archive.notes);
    setSources(archive.sources);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (archive == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const systemText = (values.systemText ?? "").trim();
      const updated = await updateArchive(archive.id, {
        name: values.name,
        system: systemText ? { text: systemText } : null,
        repository_id: values.repository_id ?? "",
        notes,
        sources,
        private: values.private ?? false,
      });
      setArchive(updated);
      setEditing(false);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (EDIT_FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof EditFormValues, errors: [e.message] }]);
      } else {
        setSaveError(e instanceof ApiError ? e.message : "Не удалось сохранить изменения");
      }
    } finally {
      setSaving(false);
    }
  };

  const onDelete = async () => {
    if (archive == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteArchive(archive.id);
      navigate("/archives");
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        setConflict(e.referrers ?? []);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось удалить запись");
      }
    } finally {
      setDeleting(false);
    }
  };

  if (loading) {
    return <Spin />;
  }

  if (notFound) {
    return (
      <Alert
        type="warning"
        showIcon
        message="Запись не найдена"
        description="Возможно, её удалили. Вернитесь к списку."
        action={
          <Link to="/archives">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (archive == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/archives">Архивы</Link> },
          { title: archive.name },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={archive.name} column={1} bordered size="small">
            <Descriptions.Item label="Система иерархии">{archive.system?.text || "—"}</Descriptions.Item>
            <Descriptions.Item label="Хранилище">
              {archive.repository_id ? (
                <Link to={`/repositories/${archive.repository_id}`}>{repositoryName(archive.repository_id)}</Link>
              ) : (
                "—"
              )}
            </Descriptions.Item>
            <Descriptions.Item label="Заметки"><TextRefListView items={archive.notes} /></Descriptions.Item>
            <Descriptions.Item label="Доказательства"><SourceLinkListView items={archive.sources} /></Descriptions.Item>
            <Descriptions.Item label="Приватная">{archive.private ? "да" : "нет"}</Descriptions.Item>
          </Descriptions>
          <Space style={{ marginTop: 16 }}>
            <Link to={`/archive-nodes?archive_id=${encodeURIComponent(archive.id)}`}>
              <Button>Архивные единицы →</Button>
            </Link>
          </Space>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title={`Удалить «${archive.name}»?`}
                description="Действие необратимо."
                okText="Удалить"
                cancelText="Отмена"
                onConfirm={onDelete}
              >
                <Button danger loading={deleting}>
                  Удалить
                </Button>
              </Popconfirm>
            </Space>
          )}
        </>
      ) : (
        <Form form={form} layout="vertical" onFinish={onSave} style={{ maxWidth: 480 }}>
          {saveError != null && (
            <Alert type="error" showIcon message={saveError} style={{ marginBottom: 16 }} />
          )}
          <Form.Item
            name="name"
            label="Название"
            rules={[{ required: true, whitespace: true, message: "Введите название" }]}
          >
            <Input />
          </Form.Item>
          <Form.Item name="systemText" label="Система иерархии (текстом)">
            <Input placeholder="фонд-опись-дело" />
          </Form.Item>
          <Form.Item name="repository_id" label="Хранилище">
            <Select
              allowClear
              placeholder="Не выбрано"
              options={repositoryOptions}
              showSearch
              filterOption={(input, option) =>
                (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
              }
            />
          </Form.Item>
          <Form.Item label="Заметки">
            <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
          </Form.Item>
          <Form.Item label="Доказательства">
            <SourceLinkListEditor value={sources} onChange={setSources} addLabel="+ доказательство" />
          </Form.Item>
          <Form.Item name="private" valuePropName="checked">
            <Checkbox>Приватная запись</Checkbox>
          </Form.Item>
          <Space>
            <Button type="primary" htmlType="submit" loading={saving}>
              Сохранить
            </Button>
            <Button onClick={cancelEdit}>Отмена</Button>
          </Space>
        </Form>
      )}

      <Modal
        title="Запись используется"
        open={conflict != null}
        onCancel={() => setConflict(null)}
        footer={<Button onClick={() => setConflict(null)}>Закрыть</Button>}
      >
        <Typography.Paragraph>
          Нельзя удалить — на запись ссылаются другие сущности:
        </Typography.Paragraph>
        <List
          size="small"
          dataSource={conflict ?? []}
          renderItem={(r) => (
            <List.Item>
              {r.type} {r.id}
            </List.Item>
          )}
        />
      </Modal>
    </Card>
  );
}
```

### 2.6. Рубеж

`cd web && npm run typecheck` (чисто), `npm run build` (чисто, только уже существующее предупреждение про размер chunk'а >500kB — не относится к этой правке). Каталог на `/` показывает «Архивные документы»/«Архивные единицы» в алфавитном порядке. Живая проверка (см. «Предпосылка») — не обязательна повторно: создать архив → `/archive-nodes` → выбрать архив → создать корневой узел → создать дочерний узел через «+ добавить дочерний узел» на View родителя → `/archive-nodes?archive_id=<id>` (кнопка с `ArchiveView.tsx`) открывает дерево с автовыбранным архивом → создать `ArchiveDocument` через picker → View показывает рабочую ссылку на узел.

### 2.7. Коммит

```bash
git add \
  web/src/ArchiveNodePicker.tsx \
  web/src/pages/ArchiveNodesList.tsx web/src/pages/ArchiveNodeForm.tsx web/src/pages/ArchiveNodeView.tsx \
  web/src/pages/ArchiveDocumentsList.tsx web/src/pages/ArchiveDocumentForm.tsx web/src/pages/ArchiveDocumentView.tsx \
  web/src/api.ts web/src/App.tsx web/src/pages/EntityCatalog.tsx web/src/pages/ArchiveView.tsx
git commit -m "feat(web): страницы Архивных узлов/Документов — ArchiveNodePicker, роуты, каталог"
```

## Задача 3. Веб-ретрофит: Attachment + AnchorEditor на ArchiveNodePicker

**Предпосылка выполнения — обязательный порядок**: эта задача зависит от `web/src/ArchiveNodePicker.tsx` (экспорты `ArchiveNodePicker`/`ArchiveDocumentSelect`), созданного Задачей 2 (Шаг 2.1) — при SDD-диспетче запускается СТРОГО ПОСЛЕ Задачи 2, не параллельно с ней (см. предупреждение в начале Задачи 2). Файлы этой задачи (`AnchorEditor.tsx`, `AttachmentForm.tsx`, `AttachmentView.tsx`) Задача 2 не трогает — пересечения по содержимому нет, только зависимость по интерфейсу.

**Интерфейсы, потребляемые из Задачи 2**: `ArchiveNodePicker`/`ArchiveDocumentSelect` (`web/src/ArchiveNodePicker.tsx`).
**Производит**: ничего нового для последующих задач — эта задача последняя в подпроекте 6, замыкает ретрофит текстовых id-полей на picker везде, где он до сих пор не применён.

**Файлы:**
- Изменить: `web/src/AnchorEditor.tsx`, `web/src/pages/AttachmentForm.tsx`, `web/src/pages/AttachmentView.tsx`

**Важно для исполнителя**: замена везде одна и та же — было: обычный `Input` с плейсхолдером `id архивного узла (AN-…)`/`id архивного документа (DC-…, необязательно)`, привязанный либо к полю antd `Form` (`AttachmentForm`/`AttachmentView`), либо к локальному объекту `value`/`onChange` (`AnchorEditor`); стало: `ArchiveNodePicker` для узла (`value`/`label`/`onChange(id, label)`) и `ArchiveDocumentSelect` для документа (`nodeId`/`value`/`onChange`, задизейблен пока `nodeId` пуст, автоматически сбрасывается при смене узла). В `AttachmentForm`/`AttachmentView` `node_id`/`document_id` выводятся из `FORM_FIELDS`/`EDIT_FORM_FIELDS` в отдельный `useState` (`nodeId`/`nodeLabel`/`documentId`) — тот же приём, что `ArchiveNodeView.tsx`'s `editParentId`. `AnchorEditor` дополнительно заводит `nodeLabel` — метка выбранного узла только для отображения в picker'е (`Anchor` не несёт свою метку в контракте). Read-only режим (`AttachmentView`'s `Descriptions` до входа в редактирование) не трогается — остаётся сырыми id, вне объёма ретрофита (см. «Предпосылка»). Переносить код ниже как есть, файл за файлом — итоговое содержимое.

### 3.1. `web/src/AnchorEditor.tsx` — ретрофит archive-варианта

#### `web/src/AnchorEditor.tsx` (изменить — итоговое содержимое)
`web/src/AnchorEditor.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Button, Input, InputNumber, Select, Space } from "antd";
import { MinusCircleOutlined, PlusOutlined } from "@ant-design/icons";
import { fetchAttachments, type Anchor, type AnchorKind, type Attachment } from "./api";
import { ArchiveDocumentSelect, ArchiveNodePicker } from "./ArchiveNodePicker";

const KIND_OPTIONS: { value: AnchorKind; label: string }[] = [
  { value: "archive", label: "Архив (узел/документ)" },
  { value: "file", label: "Файл (вложение)" },
  { value: "url", label: "Ссылка" },
];

const EMPTY_ANCHOR: Anchor = { kind: "archive", page: 1 };

// useAttachmentOptions — заполняет Select вложений для anchor.attachment_id,
// по образцу useRepositoryOptions/ArchiveForm.tsx (обсуждение подпроекта 5).
function useAttachmentOptions() {
  const [attachments, setAttachments] = useState<Attachment[]>([]);

  useEffect(() => {
    fetchAttachments({ limit: 500 })
      .then(setAttachments)
      .catch(() => setAttachments([]));
  }, []);

  return attachments.map((a) => ({ value: a.id, label: a.filename || a.uri || a.id }));
}

// AnchorEditor — редактор полиморфной привязки «где именно» у Citation
// (models.Anchor): дискриминатор kind (archive/file/url) переключает набор
// полей. Первый полиморфный тип в программе — редактор целиком заменяется
// при смене kind (EMPTY_ANCHOR для нового варианта), а не сохраняет поля
// прежнего варианта.
export function AnchorEditor({
  value,
  onChange,
  addLabel,
}: {
  value: Anchor | null | undefined;
  onChange: (next: Anchor | null) => void;
  addLabel: string;
}) {
  const attachmentOptions = useAttachmentOptions();
  // nodeLabel — метка выбранного узла ТОЛЬКО для отображения в picker'е (Anchor
  // не несёт свою метку в контракте, docs/data-model/entity-write.md §3.4/§6);
  // до выбора нового узла ArchiveNodePicker сам покажет сырой node_id как
  // fallback (см. ArchiveNodePicker.tsx).
  const [nodeLabel, setNodeLabel] = useState<string | null>(null);

  if (value == null) {
    return (
      <Button type="dashed" onClick={() => onChange(EMPTY_ANCHOR)} icon={<PlusOutlined />} block>
        {addLabel}
      </Button>
    );
  }

  const set = (patch: Partial<Anchor>) => onChange({ ...value, ...patch });

  const setKind = (kind: AnchorKind) => {
    if (kind === "archive") {
      onChange({ kind, page: 1 });
    } else if (kind === "file") {
      onChange({ kind });
    } else {
      onChange({ kind });
    }
  };

  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      <Space wrap>
        <Select style={{ width: 220 }} value={value.kind} options={KIND_OPTIONS} onChange={setKind} />
        <MinusCircleOutlined onClick={() => onChange(null)} />
      </Space>
      {value.kind === "archive" && (
        <Space wrap style={{ width: "100%" }}>
          <ArchiveNodePicker
            value={value.node_id ?? ""}
            label={nodeLabel ?? undefined}
            onChange={(nid, lbl) => {
              setNodeLabel(lbl);
              set({ node_id: nid, document_id: undefined });
            }}
          />
          <ArchiveDocumentSelect
            nodeId={value.node_id || null}
            value={value.document_id}
            onChange={(v) => set({ document_id: v })}
          />
          <InputNumber
            placeholder="Страница"
            min={1}
            value={value.page}
            onChange={(v) => set({ page: v ?? undefined })}
            style={{ width: 110 }}
          />
          <Input
            placeholder="Область (необязательно)"
            value={value.rect}
            onChange={(e) => set({ rect: e.target.value })}
            style={{ width: 200 }}
          />
        </Space>
      )}
      {value.kind === "file" && (
        <Space wrap style={{ width: "100%" }}>
          <Select
            style={{ width: 300 }}
            placeholder="Вложение"
            options={attachmentOptions}
            value={value.attachment_id || undefined}
            onChange={(v) => set({ attachment_id: v })}
            showSearch
            filterOption={(input, option) =>
              (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
            }
          />
          <Input
            placeholder="Тайм-код (необязательно)"
            value={value.timecode}
            onChange={(e) => set({ timecode: e.target.value })}
            style={{ width: 160 }}
          />
        </Space>
      )}
      {value.kind === "url" && (
        <Input
          placeholder="https://…"
          value={value.url}
          onChange={(e) => set({ url: e.target.value })}
        />
      )}
    </Space>
  );
}
```

### 3.2. `Attachment` — ретрофит Form/View

#### `web/src/pages/AttachmentForm.tsx` (изменить — итоговое содержимое)
`web/src/pages/AttachmentForm.tsx`:
```tsx
import { useState } from "react";
import { Alert, Checkbox, Form, Input, InputNumber, Modal, Select } from "antd";
import { createAttachment, type Attachment } from "../api";
import { ApiError } from "../auth";
import { ArchiveDocumentSelect, ArchiveNodePicker } from "../ArchiveNodePicker";

interface AttachmentFormValues {
  kind: string;
  uri?: string;
  filename?: string;
  mime?: string;
  page?: number;
  note?: string;
  private?: boolean;
}

// FORM_FIELDS — node_id/document_id НЕ входят: они управляются отдельным
// состоянием (nodeId/documentId, не полями antd Form — тот же приём, что
// editParentId в ArchiveNodeView.tsx/editUnitId в ArchiveDocumentForm.tsx),
// поэтому 422 на них попадает в общий error, а не в конкретное поле формы.
const FORM_FIELDS: (keyof AttachmentFormValues)[] = ["kind", "uri", "filename", "mime", "page", "note"];

const KIND_OPTIONS = [
  { value: "scan", label: "скан" },
  { value: "document", label: "документ" },
  { value: "audio", label: "аудио" },
  { value: "photo", label: "фото" },
];

// CreateAttachmentModal — форма создания вложения. node_id/document_id —
// ArchiveNodePicker + ArchiveDocumentSelect (подпроект 6 — ArchiveNode/
// ArchiveDocument теперь имеют CRUD и полноценный picker/Select, см.
// ArchiveNodePicker.tsx). node_id обязателен, document_id — нет, и
// становится доступен только после выбора узла (документ должен
// принадлежать выбранному узлу).
export function CreateAttachmentModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Attachment) => void;
}) {
  const [form] = Form.useForm<AttachmentFormValues>();
  const [nodeId, setNodeId] = useState("");
  const [nodeLabel, setNodeLabel] = useState<string | null>(null);
  const [documentId, setDocumentId] = useState<string | undefined>(undefined);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setNodeId("");
    setNodeLabel(null);
    setDocumentId(undefined);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: AttachmentFormValues) => {
    if (!nodeId) {
      setError("Выберите архивный узел");
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      const created = await createAttachment({
        kind: values.kind,
        uri: values.uri,
        filename: values.filename,
        mime: values.mime,
        page: values.page,
        node_id: nodeId,
        document_id: documentId,
        note: values.note,
        private: values.private ?? false,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof AttachmentFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить вложение"
      open={open}
      onCancel={handleClose}
      onOk={() => form.submit()}
      okText="Создать"
      cancelText="Отмена"
      confirmLoading={submitting}
      destroyOnHidden
    >
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
      <Form form={form} layout="vertical" onFinish={onFinish} initialValues={{ kind: "scan", private: false }}>
        <Form.Item name="kind" label="Вид" rules={[{ required: true, message: "Выберите вид" }]}>
          <Select options={KIND_OPTIONS} />
        </Form.Item>
        <Form.Item name="filename" label="Имя файла">
          <Input autoFocus />
        </Form.Item>
        <Form.Item name="uri" label="URI">
          <Input placeholder="файл, ссылка" />
        </Form.Item>
        <Form.Item name="mime" label="MIME-тип">
          <Input placeholder="image/jpeg" />
        </Form.Item>
        <Form.Item name="page" label="Страница">
          <InputNumber min={0} style={{ width: "100%" }} />
        </Form.Item>
        <Form.Item label="Архивный узел" required>
          <ArchiveNodePicker
            value={nodeId}
            label={nodeLabel ?? undefined}
            onChange={(id, lbl) => {
              setNodeId(id);
              setNodeLabel(lbl);
              setDocumentId(undefined);
            }}
          />
        </Form.Item>
        <Form.Item label="Архивный документ (необязательно)">
          <ArchiveDocumentSelect nodeId={nodeId || null} value={documentId} onChange={setDocumentId} />
        </Form.Item>
        <Form.Item name="note" label="Заметка">
          <Input.TextArea rows={3} />
        </Form.Item>
        <Form.Item name="private" valuePropName="checked">
          <Checkbox>Приватная запись</Checkbox>
        </Form.Item>
      </Form>
    </Modal>
  );
}
```

#### `web/src/pages/AttachmentView.tsx` (изменить — итоговое содержимое)
`web/src/pages/AttachmentView.tsx`:
```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  Alert,
  Breadcrumb,
  Button,
  Card,
  Checkbox,
  Descriptions,
  Form,
  Input,
  InputNumber,
  List,
  Modal,
  Popconfirm,
  Select,
  Space,
  Spin,
  Typography,
} from "antd";
import { deleteAttachment, fetchAttachment, updateAttachment, type Attachment } from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { ArchiveDocumentSelect, ArchiveNodePicker } from "../ArchiveNodePicker";

interface EditFormValues {
  kind: string;
  uri?: string;
  filename?: string;
  mime?: string;
  page?: number;
  note?: string;
  private?: boolean;
}

// EDIT_FORM_FIELDS — node_id/document_id НЕ входят: они управляются отдельным
// состоянием (nodeId/documentId, не полями antd Form — тот же приём, что
// editParentId в ArchiveNodeView.tsx), поэтому 422 на них попадает в общий
// saveError.
const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["kind", "uri", "filename", "mime", "page", "note"];

const KIND_OPTIONS = [
  { value: "scan", label: "скан" },
  { value: "document", label: "документ" },
  { value: "audio", label: "аудио" },
  { value: "photo", label: "фото" },
];

function attachmentLabel(a: Attachment): string {
  return a.filename || a.uri || a.id;
}

// AttachmentView — просмотр вложения, переключаемый в форму редактирования
// на той же странице. node_id/document_id в режиме редактирования —
// ArchiveNodePicker + ArchiveDocumentSelect (подпроект 6 — ArchiveNode/
// ArchiveDocument теперь имеют CRUD, см. AttachmentForm/ArchiveNodePicker.tsx).
export default function AttachmentView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [attachment, setAttachment] = useState<Attachment | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [nodeId, setNodeId] = useState("");
  const [nodeLabel, setNodeLabel] = useState<string | null>(null);
  const [documentId, setDocumentId] = useState<string | undefined>(undefined);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (attachmentId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setAttachment(null);
    fetchAttachment(attachmentId)
      .then(setAttachment)
      .catch((e) => {
        if (e instanceof ApiError && e.status === 404) {
          setNotFound(true);
        } else {
          setError(e instanceof Error ? e.message : "Не удалось загрузить запись");
        }
      })
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    setEditing(false);
    setSaveError(null);
    if (id != null) {
      load(id);
    }
  }, [id]);

  const startEdit = () => {
    if (attachment == null) {
      return;
    }
    form.setFieldsValue({
      kind: attachment.kind,
      uri: attachment.uri ?? "",
      filename: attachment.filename ?? "",
      mime: attachment.mime ?? "",
      page: attachment.page,
      note: attachment.note ?? "",
      private: attachment.private,
    });
    setNodeId(attachment.node_id);
    setNodeLabel(null);
    setDocumentId(attachment.document_id);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (attachment == null) {
      return;
    }
    if (!nodeId) {
      setSaveError("Выберите архивный узел");
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await updateAttachment(attachment.id, {
        kind: values.kind,
        uri: values.uri,
        filename: values.filename,
        mime: values.mime,
        page: values.page,
        node_id: nodeId,
        document_id: documentId,
        note: values.note,
        private: values.private ?? false,
      });
      setAttachment(updated);
      setEditing(false);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (EDIT_FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof EditFormValues, errors: [e.message] }]);
      } else {
        setSaveError(e instanceof ApiError ? e.message : "Не удалось сохранить изменения");
      }
    } finally {
      setSaving(false);
    }
  };

  const onDelete = async () => {
    if (attachment == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteAttachment(attachment.id);
      navigate("/attachments");
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        setConflict(e.referrers ?? []);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось удалить запись");
      }
    } finally {
      setDeleting(false);
    }
  };

  if (loading) {
    return <Spin />;
  }

  if (notFound) {
    return (
      <Alert
        type="warning"
        showIcon
        message="Запись не найдена"
        description="Возможно, её удалили. Вернитесь к списку."
        action={
          <Link to="/attachments">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (attachment == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/attachments">Вложения</Link> },
          { title: attachmentLabel(attachment) },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={attachmentLabel(attachment)} column={1} bordered size="small">
            <Descriptions.Item label="Вид">{attachment.kind}</Descriptions.Item>
            <Descriptions.Item label="Имя файла">{attachment.filename || "—"}</Descriptions.Item>
            <Descriptions.Item label="URI">{attachment.uri || "—"}</Descriptions.Item>
            <Descriptions.Item label="MIME-тип">{attachment.mime || "—"}</Descriptions.Item>
            <Descriptions.Item label="Страница">{attachment.page || "—"}</Descriptions.Item>
            <Descriptions.Item label="Архивный узел">{attachment.node_id}</Descriptions.Item>
            <Descriptions.Item label="Архивный документ">{attachment.document_id || "—"}</Descriptions.Item>
            <Descriptions.Item label="Заметка">{attachment.note || "—"}</Descriptions.Item>
            <Descriptions.Item label="Приватная">{attachment.private ? "да" : "нет"}</Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title={`Удалить «${attachmentLabel(attachment)}»?`}
                description="Действие необратимо."
                okText="Удалить"
                cancelText="Отмена"
                onConfirm={onDelete}
              >
                <Button danger loading={deleting}>
                  Удалить
                </Button>
              </Popconfirm>
            </Space>
          )}
        </>
      ) : (
        <Form form={form} layout="vertical" onFinish={onSave} style={{ maxWidth: 480 }}>
          {saveError != null && (
            <Alert type="error" showIcon message={saveError} style={{ marginBottom: 16 }} />
          )}
          <Form.Item name="kind" label="Вид" rules={[{ required: true, message: "Выберите вид" }]}>
            <Select options={KIND_OPTIONS} />
          </Form.Item>
          <Form.Item name="filename" label="Имя файла">
            <Input />
          </Form.Item>
          <Form.Item name="uri" label="URI">
            <Input placeholder="файл, ссылка" />
          </Form.Item>
          <Form.Item name="mime" label="MIME-тип">
            <Input placeholder="image/jpeg" />
          </Form.Item>
          <Form.Item name="page" label="Страница">
            <InputNumber min={0} style={{ width: "100%" }} />
          </Form.Item>
          <Form.Item label="Архивный узел" required>
            <ArchiveNodePicker
              value={nodeId}
              label={nodeLabel ?? undefined}
              onChange={(nid, lbl) => {
                setNodeId(nid);
                setNodeLabel(lbl);
                setDocumentId(undefined);
              }}
            />
          </Form.Item>
          <Form.Item label="Архивный документ (необязательно)">
            <ArchiveDocumentSelect nodeId={nodeId || null} value={documentId} onChange={setDocumentId} />
          </Form.Item>
          <Form.Item name="note" label="Заметка">
            <Input.TextArea rows={3} />
          </Form.Item>
          <Form.Item name="private" valuePropName="checked">
            <Checkbox>Приватная запись</Checkbox>
          </Form.Item>
          <Space>
            <Button type="primary" htmlType="submit" loading={saving}>
              Сохранить
            </Button>
            <Button onClick={cancelEdit}>Отмена</Button>
          </Space>
        </Form>
      )}

      <Modal
        title="Запись используется"
        open={conflict != null}
        onCancel={() => setConflict(null)}
        footer={<Button onClick={() => setConflict(null)}>Закрыть</Button>}
      >
        <Typography.Paragraph>
          Нельзя удалить — на запись ссылаются другие сущности:
        </Typography.Paragraph>
        <List
          size="small"
          dataSource={conflict ?? []}
          renderItem={(r) => (
            <List.Item>
              {r.type} {r.id}
            </List.Item>
          )}
        />
      </Modal>
    </Card>
  );
}
```

### 3.3. Рубеж

`cd web && npm run typecheck` (чисто), `npm run build` (чисто). Живая проверка (см. «Предпосылка») — не обязательна повторно: `/attachments` → «+ добавить» — «Архивный узел» теперь кнопка «Выбрать узел» (не текстовый `Input`), «Архивный документ» — `Select`, задизейбленный до выбора узла и корректно фильтрующийся по `unit_id` после; создание цитаты с `anchor.kind=archive` в `AnchorEditor` — та же замена, привязка round-trip'ится через `Citation`.

### 3.4. Коммит

```bash
git add web/src/AnchorEditor.tsx web/src/pages/AttachmentForm.tsx web/src/pages/AttachmentView.tsx
git commit -m "feat(web): ретрофит Attachment/AnchorEditor на ArchiveNodePicker"
```

## Рубеж прохода

После Задачи 3: `gofmt -l .` пусто, `go build/vet/test ./...` зелёные (1215 тестов, 112 пакетов), `npm run typecheck`/`build` чисты. Каталог `/` — 17 строк по алфавиту (добавились «Архивные документы», «Архивные единицы»). `ArchiveNode` и `ArchiveDocument` имеют полный CRUD через HTTP, MCP и веб, по конвенциям `docs/data-model/entity-write.md` §3-4, с двумя новыми переиспользуемыми паттернами программы (дерево, скопированное по обязательному внешнему владельцу; кросс-полевая проверка «чужой FK принадлежит тому же контексту, что и мой»). `Attachment`/`AnchorEditor`'s `node_id`/`document_id` больше не текстовые поля — полноценный picker. После всех трёх задач — обзор всей ветки целиком (`git diff main...HEAD` по объёму подпроекта), при необходимости волна точечных фиксов по результатам ревью, затем слияние в `main` — как и в предыдущих пяти подпроектах программы `entity-write`. Следующий подпроект — 7 (`Family`).

## Коммиты

Три коммита в `main`, по одному на задачу — см. Шаги 1.8, 2.7, 3.4.
