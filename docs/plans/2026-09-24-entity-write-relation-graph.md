# Веб-CRUD/MCP для всех сущностей — подпроект 9 (граф вокруг Person: `Relation`, `Residence`, `Event`): план

> **Для агента-исполнителя:** ОБЯЗАТЕЛЬНЫЙ ПОДНАВЫК: superpowers:subagent-driven-development.

Девятый и ПОСЛЕДНИЙ проход по декомпозиции `docs/data-model/entity-write.md` §2 — крупнейший и самый нетривиальный подпроект всей программы `entity-write` (73 изменённых/новых пути по `git status`, три сущности одновременно: `Relation`, `Residence`, `Event`+`EventParticipant`). Вместе они образуют «граф вокруг `Person`» — то, ради чего вся программа и затевалась: подпроекты 1-8 построили словари, источники, архивную иерархию, `Family` и, наконец, сам `Person` (подпроект 8), но ни одна из них не связывала персон между собой, не фиксировала, где они жили, и не описывала события их жизни. Три новых технических паттерна, ни один из которых не встречался в подпроектах 1-8: (a) `Relation.PersonA`/`.PersonB` — первая пара СТРОГИХ ссылок на ОДИН И ТОТ ЖЕ тип (`Person`) у одной сущности, обе проверяются независимо в транзакции; (b) осознанный, документированный отказ от `search_relations`/`search_residences` — единственное место в программе, где сущность НЕ получает поисковый тул, потому что поисковый индекс для неё структурно пуст (`replaceSearchIndex(tx, …, nil)`), и полнотекстовый поиск был бы не «менее удобной», а буквально нерабочей функциональностью; вместо него — person/place-scoped фильтрация списка; (c) `EventParticipant.PersonID` — первая СТРОГАЯ (существование проверяется) ссылка ВНУТРИ элемента array-of-objects MCP/HTTP-аргумента в программе (до этого все ссылки внутри массивов объектов — `PersonName.Surname/.Given/.Patronymic`, подпроект 8 — были мягкими). Плюс — первое одиночное объектное поле (`Event.Place`, новый транспортный тип `transport.PlaceRef`), заведённое уже ПОСЛЕ того, как программа закрыла регрессию presence-only update-guard'а для `TextRef`/`FactDate`-подобных полей (см. «Глобальные ограничения» и «Предпосылка»), так что этот подпроект с рождения свободен от того класса бага. На вебе — первый настоящий `PersonPicker` (переиспользуемый во всех трёх новых сущностях, поскольку `Person` наконец получил List/Search в подпроекте 8) и `EventParticipantListEditor`.

## Goal

Полный CRUD (HTTP + MCP + веб) для `Relation` (ребро графа родства/связи между двумя персонами), `Residence` (проживание персоны в административной единице) и `Event`+`EventParticipant` (событие жизненного факта с участниками) — по конвенциям `docs/data-model/entity-write.md` §3-4, включая новую §3.8. `Relation`/`Residence` намеренно НЕ получают `search_*`/`GET /api/*/search` — вместо них person/place-scoped `list`-фильтрация (`RelationQuery`/`ResidenceQuery`). `Event` получает полный набор из 6 операций, включая `event_search` (индексируется только текст `Place`). На вебе: общий `PersonPicker`+`usePersonOptions` (`web/src/PersonPicker.tsx`), `EventParticipantListEditor` (`web/src/EventParticipantList.tsx`), девять страниц (`RelationForm`/`RelationsList`/`RelationView`, `ResidenceForm`/`ResidencesList`/`ResidenceView`, `EventForm`/`EventsList`/`EventView`), роуты и три новые строки каталога сущностей. Программа `entity-write` (все 9 подпроектов) считается завершённой после слияния этого подпроекта — см. «Рубеж прохода».

## Архитектура

**`Relation`** — `{ID, Kind, RelType, PersonA, PersonB ID, Since, Until *FactDate, Sources []SourceLink, Notes []TextRef, Private bool}`. `Kind` — закрытый набор `blood`/`marriage`/`adoption`/`associate`; `RelType` — обязателен, только если `kind=associate` (иначе должен отсутствовать) — условное правило проверяется `Relation.Validate()`, не MCP-схемой (та не умеет условных required). `PersonA`/`PersonB` — ДВЕ строгие ссылки на `Person` (первая пара строгих ссылок на один и тот же тип в программе): обе проверяются НЕЗАВИСИМО в транзакции `create_relation`/`update_relation` — отсутствие `person_a` не мешает отдельно сообщить об отсутствии `person_b`, если тот тоже не найден; различность `PersonA` и `PersonB` — забота модельной `Validate()` (`person_b`: «связь персоны с самой собой»), не usecase-слоя. Модель (`internal/models/relation.go`, `relation_validate.go`) уже существует и не меняется этим подпроектом.

**`Residence`** — `{ID, PersonID, PlaceID ID, Since, Until *FactDate, Sources []SourceLink, Note string, Private bool}`. `PersonID` — строгая ссылка на `Person`; `PlaceID` — строгая ссылка ИМЕННО на `AdministrativeDivision` (не любой `PlaceRef`, в отличие от `Event.Place`). `Note` — ОДНА строка, не список `TextRef` — единственное текстовое поле среди новых сущностей подпроекта, не имеющее формы списка.

**`Event`+`EventParticipant`** — `Event = {ID, Type, Date *FactDate, Place *PlaceRef, Participants []EventParticipant, Sources []SourceLink, Notes []TextRef, Private bool}`; `EventParticipant = {PersonID ID, Role, Note string}`. `Type` — открытый набор строк (`birth`/`death`/`marriage`/`burial`/`confession`/`census` и др.), REQUIRED и на `_update` (безусловная полная замена, как `Relation.Kind`). `Place` — мягкая ссылка НОВОГО объектного вида: `models.PlaceRef` структурно идентичен `models.TextRef` (`{Text, Ref ID, Type Type}`), но это ОТДЕЛЬНЫЙ тип модели — валидация ограничивает `Ref`'s тип только `AdministrativeDivision`/`Church`/`Parish` (`place_ref_validate.go`); существование `Ref` НИКОГДА не проверяется, тот же принцип, что у любого `TextRef` в программе. `Participants[i].PersonID` — первая СТРОГАЯ ссылка внутри элемента array-of-objects MCP/HTTP-аргумента: `create_event`/`update_event` идут циклом `for i, p := range e.Participants { tx.GetPerson(...) }`, индексированная ошибка `participants[%d].person_id`. Модели (`internal/models/{relation,residence,event,event_participant,place_ref}.go` + их `*_validate.go`) уже существуют и не меняются этим подпроектом — новый код в модельном слое ограничен `internal/models/query.go` (три новых типа запроса списка, Шаг 1.1).

**Поиск.** `Relation`/`Residence` — у обеих `SaveRelation`/`SaveResidence` (`internal/store/sqlstore/relations.go`) вызывают `replaceSearchIndex(tx, «relations»/«residences», r.ID, nil)` — передают `nil` вместо карты индексируемых полей: НИ ОДНО поле этих сущностей никогда не попадает в поисковый индекс. `search_relations`/`search_residences` были бы тулами, ВСЕГДА возвращающими пустой результат — программа впервые за 9 подпроектов сознательно НЕ реализует `search_*` для двух сущностей (см. «Предпосылка» и §3.8). Вместо этого — фильтрация списка по персоне/месту: `models.RelationQuery{PersonID *ID, Page}` (ребро проходит, если `PersonID` совпадает с `PersonA` ИЛИ `PersonB`), `models.ResidenceQuery{PersonID, PlaceID *ID, Page}` (оба фильтра пересекаются, если заданы вместе) — `list_relations`/`list_residences` реализуют это full-scan-and-filter по generic-окнам, тот же приём, что `list_archive_nodes` (подпроект 6, §3.4), без новых методов `store.Store`. `Event`, напротив, ЕСТЬ поисковый индекс — `SaveEvent` вызывает `replaceSearchIndex(tx, «events», e.ID, map[string][]string{«place»: {place}})` — индексируется ТОЛЬКО текст `Place` (не `type`, не `date`, не участники); `search_events`/`GET /api/events/search`/`event_search` построены обычным образом (по образцу `search_families`), а `models.EventQuery{PersonID *ID, Page}` — дополнительный, не заменяющий поиск фильтр по участнику для `list_events`.

На вебе — три обычные страницы-тройки (List/View+Form) по конвенциям §4, но с новыми переиспользуемыми элементами: `PersonPicker`+`usePersonOptions` (`web/src/PersonPicker.tsx`) — первый в программе переиспользуемый picker персоны (используется в `Relation.PersonA`/`.PersonB`, `Residence.PersonID`, каждой строке `Event.Participants`), и `EventParticipantListEditor` (`web/src/EventParticipantList.tsx`) — редактор `Event.Participants`, структурно как `SourceLinkListEditor`, но без ref-preservation-логики `PersonNameListEditor` (подпроект 8): `person_id` — строгий id, не мягкий `TextRef`. `RelationsList`/`ResidencesList` рендерятся БЕЗ строки поиска (`Input.Search`) — единственные списковые страницы в программе без неё, отражая отсутствие `search_relations`/`search_residences` на бэкенде. `ResidenceForm.tsx` заводит `useAdminDivisionOptions()` — первый picker, точечно нацеленный на `AdministrativeDivision` как строгую FK-цель (не общий `TextRef`-словарь).

## Технологии

Backend: Go, `internal/models` (домен — `Relation`/`Residence`/`Event`/`EventParticipant`/`PlaceRef` + их `Validate()` уже существуют, не меняются этим подпроектом) → `internal/usecases/<scenario>` → `internal/transport` (DTO) → `internal/httpapi` (JSON REST) / `internal/mcp` (`github.com/mark3labs/mcp-go`, Streamable HTTP) → `internal/store`/`internal/store/sqlstore` (generic-хранилище, готово для всех 21 типа, не меняется — `store.Store.{ListRelations,ListResidences,ListEvents,SaveRelation,SaveResidence,SaveEvent,GetRelation,GetResidence,GetEvent,DeleteRelation,DeleteResidence,DeleteEvent}` существовали до этого подпроекта). Frontend: Vite + React 18 + TypeScript + antd, файлы-страницы по образцу `FamiliesList`/`FamilyForm`/`FamilyView` и `PeopleList`/`PersonForm`/`PersonView` (подпроект 8), плюс новые переиспользуемые компоненты `PersonPicker.tsx`/`EventParticipantList.tsx`. Тесты: стандартный `go test` (usecases — фейковый стор в памяти; `internal/httpapi/write_store_test.go` — реальный SQLite), `npm run typecheck`/`npm run build` на вебе.

## Спецификация

Источник истины дизайна — `docs/data-model/entity-write.md` (копия из проверочного worktree, см. «Предпосылка» — копия на `main` этого раздела не содержит §3.8): §3.8 «Подпроект 9 (граф вокруг Person: `Relation`, `Residence`, `Event`) — backend, новые паттерны» — три новых паттерна программы (пара строгих ссылок на один тип, отказ от `search_relations`/`search_residences`, строгая ссылка внутри массива объектов); §4 «Веб-UI конвенции» — общие правила List/View/Create/Delete, которым без отступлений следуют новые страницы (плоские списки, без иерархии); §5 «Не в объёме этого документа» — на момент завершения Задачи 1 всё ещё говорит, что веб-слой `EventParticipantListEditor` и страниц `Relation`/`Residence`/`Event` — «отдельная последующая задача, вне этого прохода» (Шаг 1.9 вставляет §3.8 и правит этот абзац именно в этом виде — состояние ПОСЛЕ Задачи 1, ДО Задачи 2); Шаг 2.8 приводит формулировку в соответствие с тем, что реально построено (см. «Предпосылка», подраздел про доки).

## Глобальные ограничения

Правила программы `entity-write`, обязательные для каждой новой сущности (дословно из `entity-write.md` и правил, закреплённых финальными ревью подпроектов 1-8); подпроект 9 не отменяет ни одного из них:

- **Access-aware `Get` для любой сущности с `Private`.** `get_relation`/`get_residence`/`get_event` обязаны принимать `access models.Access` и прятать приватную запись как отсутствующую для не-владельца: `if rec.Private && access != models.AccessFull { return models.ErrNotFound }` (образец — `internal/usecases/get_family/scenario.go`, повторено в `get_person` подпроекта 8).
- **Real-store тесты обязательны.** Каждая пишущая сущность получает сквозной тест на реальном SQLite-сторе в `internal/httpapi/write_store_test.go` (не только фейковый стор в usecase-тестах) — здесь `TestRelationWriteContractWithRealStore`, `TestResidenceWriteContractWithRealStore`, `TestEventWriteContractWithRealStore` (Шаг 1.8).
- **Fetch-then-merge на каждом update.** `update_relation`/`update_residence`/`update_event` читают текущую версию через `Get*` (в MCP-обработчике — HTTP `PUT` заменяет тело целиком без fetch, MCP — с fetch-then-merge через `Get*` перед вызовом `Update*`), накладывают новые поля, валидируют и сохраняют через `Save*` — полная замена записи, не частичный patch (свойство самого generic-стора, см. `entity-write.md` §1).
- **MCP «отсутствие необязательного списочного/объектного аргумента — значит сохранить как есть», явно расширенное на одиночные объектные поля.** Это правило подпроект 9 не изобретает, а НАСЛЕДУЕТ уже закрытым в `main` до начала подпроекта program-wide фиксом (коммиты "fix(mcp): необязательные аргументы *_update сохраняют текущее значение при отсутствии" и "fix: ревью — null должен снова очищать TextRef/FactDate-поля в *_update", см. git log в «Предпосылке»): для КАЖДОГО необязательного поля — списочного (`sources`, `notes`, `participants`) ИЛИ одиночного объектного (`since`/`until`, `Event.Place`) — отсутствие ключа в вызове `*_update` сохраняет текущее значение, явная передача (в т.ч. пустой список или `null`) заменяет/очищает его. Для одиночных объектных полей guard обязан быть PRESENCE-ONLY (`if _, ok := args["x"]; ok { … }`), НЕ `raw != nil` — иначе явный `null` не отличить от отсутствия ключа, и поле никогда не очистить (ровно тот баг, который был закрыт финальным ревью прямо перед этим подпроектом). `Relation.PersonA`/`.PersonB`, `Residence.PersonID`/`.PlaceID`, `Relation.Kind`, `Event.Type` — единственное исключение из preserve-on-omit: они REQUIRED на `*_update` и заменяются БЕЗУСЛОВНО при каждом вызове (как `Citation.SourceID`) — это сознательное решение задания подпроекта (см. «Предпосылка», design judgment call №3 бэкенд-отчёта), а не пробел.
- **Мягкая ссылка (`TextRef`/`PlaceRef`) никогда не проверяется на существование при сохранении, независимо от того, есть ли у цели свой CRUD-слой.** `Event.Place.Ref` может указывать на `AdministrativeDivision`/`Church`/`Parish` (все три имеют собственный CRUD) — существование всё равно не проверяется, только формат/ограничение типа в `Event.Validate()`. Тот же принцип действует с первого появления `TextRef` в программе (подпроект 1).
- **Формулировка описания поиска обязана совпадать с реальным списком полей, индексируемых generic-слоем — и с реальным набором операций сущности.** Для `Event` — `event_search`/`GET /api/events/search` обязаны буквально говорить «только по началу текста места (`place`)», не «по типу события» (тот же урок §3.2/§3.6/§3.7, здесь применённый и к самому наличию поиска). Для `Relation`/`Residence` — доки обязаны объяснять ПОЧЕМУ поиска нет (структурно пустой индекс), а не просто не упоминать `search_relations`/`search_residences` молча.
- **Доки (`docs/usage.md`, `docs/architecture.md`, `CHANGELOG.md`, `docs/data-model/entity-write.md`) обязаны описывать оба поставленных куска — и бэкенд, и веб — до конца прохода, не только на момент коммита Задачи 1.** Правило, впервые сформулированное явно в подпроекте 8 (см. его план, «Предпосылка») и подтверждённое здесь: бэкенд и веб этого подпроекта — тоже две последовательные живые проверки (см. «Предпосылка» ниже), и докам Задачи 1 это правило выполнить неоткуда — Задача 2 получает собственный докный шаг (2.8).

## Предпосылка: живая проверка

Весь код ниже применён и проверен в отдельном throwaway git worktree (`/Users/asmarin/dev/mine/genodex-verify-subproject9`, ветка `verify/entity-write-subproject9`, форкнута от `main`) — не на `main` напрямую и не в этом плане с нуля: он транскрибирован из уже рабочего дерева, а не написан заново. Проверка бэкенда и веба проведена двумя последовательными живыми проходами (сперва бэкенд, затем веб поверх него), задокументированными двумя отдельными отчётами в этом же worktree: `/Users/asmarin/dev/mine/genodex-verify-subproject9/BACKEND_REPORT.md` и `/Users/asmarin/dev/mine/genodex-verify-subproject9/WEB_REPORT.md`. Существенное из обоих перенесено сюда.

**Проверка бэкенда** (`BACKEND_REPORT.md`): `gofmt -l .` пусто, `go build/vet/test ./...` зелёные — **1482 теста, 140 пакетов** (было 1341 на `main` до этого подпроекта — **+141** новый тест). Живой смок-тест через `curl` (реальный `genodex serve`, чистая БД, owner зарегистрирован): (1) создание `Relation` с несуществующим `person_b` → 422 на этом поле, текст ошибки — «персона %q не найдена»; (2) создание `Residence` в реально существующем `AdministrativeDivision` («Давыдово», тип `selo`) и 422 на несуществующем `place_id`; (3) создание `Event` с двумя `participants`, второй указывает на несуществующую персону → 422 на `participants[1].person_id` (индексированная ошибка сработала на ВТОРОМ, не первом элементе — доказывает независимость проверок по индексу); (4) создание `Event` с `place.ref`, указывающим на заведомо НЕСУЩЕСТВУЮЩИЙ `AdministrativeDivision`, → 201, `place` round-trip'ится как есть — подтверждено: `Place` не проверяется на существование НИКОГДА; (5) `PUBLIC → PRIVATE` через `PUT` на `Event` (не `private → private`) — анонимный `GET` до update видит запись (200), после — не видит (404); (6) `event_search` по фрагменту «село» находит событие с `place.text="село Троицкое"`, но НЕ находит по фрагменту из середины слова («Давыд» из «село Давыдово» не матчится — поиск префиксный по ВСЕМУ тексту `place`, не потокенный; поймано живым тестом и задокументировано); (7) **решающая проверка** — MCP `event_update` без ключа `place` в вызове сохраняет текущее значение (`{"place":{"text":"село Троицкое"}, ...}` не изменилось), а тот же вызов с явным `"place": null` очищает поле (ключ `place` полностью пропадает из ответа) — presence-only guard (`if _, ok := args["place"]; ok {...}`, НЕ `raw != nil`) подтверждён живьём именно на первом поле программы, заведённом уже после program-wide фикса этого класса бага.

**Проверка веба** (`WEB_REPORT.md`): `npm run typecheck`/`npm run build` чисты (только предсуществующее предупреждение о размере чанка >500kB, не связанное с этим подпроектом). Живой клик-тест в браузере (порт 18877 — см. «Port note» ниже) против реально запущенного сервера: (1) `Relation kind=blood` создана через `CreateRelationModal`, `PersonPicker` показал обеих персон с верными подписями (`personDisplayName`), поле «Тип связи» (`rel_type`) корректно ОТСУТСТВОВАЛО; (2) переключение на `kind=associate` в той же форме — «Тип связи» появилось РОВНО в момент переключения, заполнено, создано; правка обратно на `kind=blood` — поле исчезло, `rel_type` очистился на сервере (проверка `Relation.Validate()`'s условного правила через живой UI); (3) `Residence` создана через `CreateResidenceModal` с `PersonPicker`+`useAdminDivisionOptions`-`Select` (показал «Давыдово (Село)»), `note` как `Input.TextArea` («изба») — round-trip как ОДНА строка, не список; (4) `Event` с двумя `EventParticipantListEditor`-строками создан, персоны и роли отображаются как рабочие ссылки; (5) **решающая веб-проверка ref-preservation `place`** — `place.ref` проставлен через `curl` напрямую (UI-пикера для `place` нет по дизайну), View показал аннотацию «→ administrative_division AD-…»; правка события БЕЗ изменения текста места и сохранение — `ref` сохранился (аннотация осталась); повторная правка С изменением текста — `ref` корректно очистился (аннотация исчезла), тот же паттерн, что `ChurchView.onSave`; (6) `RelationsList`/`ResidencesList` подтверждены как плоские списки БЕЗ `Input.Search` (скриншотом) — соответствует отсутствию `/search` на бэкенде; (7) `?person_id=`-фильтры `Relation`/`Residence` перекрёстно сверены напрямую через API — вернули ожидаемую единственную запись, подтвердив, что параметры запроса с фронта совпадают с тем, что ожидает бэкенд.

**Port note из `WEB_REPORT.md`** (сохранено как контекст, не как инструкция для будущего исполнителя): веб-агент сначала попытался запустить смок-сервер на порту 8766 (по аналогии с бэкенд-агентом), но порт оказался занят СОВЕРШЕННО ДРУГИМ, параллельно выполнявшимся агентским сеансом на этой же машине (`genodex-mcp-preserve-on-omit`, `/tmp/genodex-mcp-fix-smoke`) — веб-агент поймал это по ошибке bind в логе ДО начала тестов, не успев проверить что-либо против чужого сервера, и переключился на порт 18877 со своей отдельной директорией данных, не тронув чужой процесс. Указывает на то, что параллельные живые проверки на одной машине должны выбирать порт/директорию данных осторожно; не относится к итоговому коду ни одним байтом.

**Доки Задачи 1 описывают ТОЛЬКО бэкенд, не веб — та же ситуация, что и в подпроекте 8.** Проверено построчно по фактическому `git diff main -- docs/usage.md docs/architecture.md CHANGELOG.md docs/data-model/entity-write.md` в проверочном worktree (бэкенд был построен и закоммичен как отдельный живой проход ДО того, как веб-агент начинал работу):

- `docs/usage.md`/`docs/architecture.md` — по устоявшейся конвенции репозитория (см. Предпосылку плана подпроекта 7/8) эти два файла НЕ описывают веб-страницы вообще, ни у одной сущности программы — значит отсутствие веб-упоминания здесь не пробел, эти файлы уже полны после Задачи 1 и Задача 2 их не трогает.
- `CHANGELOG.md` — пункт, добавленный Задачей 1 (Шаг 1.9), буквально заканчивается фразой «Веб-слой — отдельная задача, вне этого прохода» — сам текст, написанный бэкенд-агентом, явно откладывает веб-часть. Это ПРОБЕЛ: Шаг 2.8 дополняет этот же пункт списком веб-файлов и убирает оговорку.
- `docs/data-model/entity-write.md` — §3.8 (Шаг 1.9) описывает исключительно бэкенд-паттерны и явно говорит в преамбуле: «этот раздел описывает только backend (usecase-сценарии, транспорт, `httpapi`, MCP); веб-слой — отдельная задача». §5 («Не в объёме этого документа») после Задачи 1 содержит абзац: «веб-слой (`EventParticipantListEditor` и страницы `Relation`/`Residence`/`Event`) — отдельная последующая задача, вне этого прохода» (заменяет прежнюю формулировку подпроекта 8, которая откладывала это ДО подпроекта 9). Оба места — ПРОБЕЛ по той же причине, что и в CHANGELOG. Шаг 2.8 заменяет преамбулу §3.8 и абзац §5 на описание реально построенного веб-слоя.
Итог: это случай (a) — доки Задачи 1 покрывают ТОЛЬКО бэкенд ПО СУЩЕСТВУ (`CHANGELOG.md`, `entity-write.md`), а не только по формальному признаку «не переписаны веб-агентом» — сам текст, написанный бэкенд-агентом, явно ссылается на веб как на будущую работу. `usage.md`/`architecture.md` не в счёт — они по конвенции репозитория никогда не описывают веб. Задача 2 ниже получает явный докный шаг (2.8), закрывающий оба реальных пробела — см. Шаг 2.8.

**Design judgment calls из `BACKEND_REPORT.md`, перенесённые сюда дословно по смыслу:**

1. **Отказ от поиска — структурное решение, подтверждённое чтением кода хранилища, не предположение.** Явно проверено в `internal/store/sqlstore/relations.go`: `SaveRelation`/`SaveResidence` вызывают `replaceSearchIndex(tx, "relations"/"residences", r.ID, nil)` — `search_relations`/`search_residences` ВСЕГДА возвращали бы пустой результат, независимо от запроса. Построена person/place-scoped list-фильтрация (`RelationQuery`/`ResidenceQuery`) вместо них, по заданию, и решение задокументировано явно в §3.8, а не просто пропущено молча.
2. **Тест поиска `Event` использовал «село», не середину слова.** Индекс — префиксный матч по ВСЕМУ тексту `place`, не потокенный: текст «село Давыдово» матчится только по началу «с», не по «Давыд» из середины. Поймано живым провалом теста, real-store тест исправлен соответственно; поведение задокументировано в MCP/HTTP doc-комментариях буквально.
3. **`Relation.PersonA`/`.PersonB` и `Residence.PersonID`/`.PlaceID` намеренно НЕ получили preserve-on-omit на MCP `*_update`**, по явной инструкции задания — они `mcp.Required()` и заменяются безусловно, как `Citation.SourceID`. Более строгий контракт, чем у большинства полей программы (нет «пропусти — сохранится»), но оправдан: подразумевать частичное изменение идентичности записи через отсутствие аргумента было бы двусмысленно.
4. Остальные решения были специфицированы заданием достаточно явно, чтобы не требовать независимых суждений.

**Design judgment calls из `WEB_REPORT.md`, перенесённые сюда дословно по смыслу:**

1. **`personDisplayName` вынесена в `PersonNameList.tsx`.** `PeopleList.tsx` (`personLabel`) и `PersonView.tsx` (расчёт `title`) дублировали одну и ту же формулу «главное имя, иначе первое, иначе id». `PersonPicker.tsx` — первый ТРЕТИЙ вызывающий этой же формулы — стал поводом вынести её единожды как `personDisplayName(p)` (композиция уже существующих `pickDisplayName`/`formatPersonName`), а не дублировать снова; `PeopleList.tsx`/`PersonView.tsx` тоже переключены на неё. **Это НЕСУЩЕСТВЕННЫЙ побочный рефакторинг файлов подпроекта 8, а НЕ новая функциональность подпроекта 9** — важно для ревьюера: `web/src/PersonNameList.tsx`, `web/src/pages/PeopleList.tsx`, `web/src/pages/PersonView.tsx` попадают в файл-лист Задачи 2 ИМЕННО по этой причине, см. Шаг 2.7.
2. **`usePersonOptions` экспортирован отдельно от `PersonPicker`.** View-страницы (`RelationView`/`ResidenceView`/`EventView`) должны резолвить голый `person_id` в подпись/ссылку, а не только предлагать picker — тот же приём, что `ArchiveView.tsx` повторно использует `fetchRepositories` и для `Select`-опций, и для функции подписи.
3. **`useAdminDivisionOptions` оставлен локальным в `ResidenceForm.tsx`**, не вынесен в общий файл — по явной инструкции задания (единственная точка использования в этом подпроекте). `ResidenceView.tsx` дублирует тот же маленький fetch-and-map инлайново (не импортирует из `ResidenceForm.tsx`) — тот же прецедент, что `ArchiveView.tsx` не импортирует `ArchiveForm.tsx`'s `useRepositoryOptions`.
4. **`RelationForm`/`RelationView`'s `FORM_FIELDS` включает только `kind`/`rel_type`** — `person_a`/`person_b` живут как обычный React state вне antd `Form` (как любой другой `*ListEditor`/`PersonPicker`-управляемый элемент в программе), так что 422 на `field="person_a"`/`"person_b"` не привязывается к конкретному контролу и падает в общий `Alert` — тот же паттерн, что у `sources[0].citation_id` везде в программе.
5. **Подписи `kind`/`type`** используют формулировки «Начало периода»/«Конец периода»/«+ дата начала»/«+ дата конца», скопированные из `ParishView.tsx` (установленный прецедент), а не изобретены заново — для единообразия по всему приложению.
6. **Строки списков `Relation`/`Residence`/`Event`** — ни у одной из трёх нет поля `name` (в отличие от `Family`/`Repository`), поэтому построены короткие композитные подписи: `"<вид связи>: <PersonA> — <PersonB>"` для `Relation`, `"<Person> — <Place>"` для `Residence`, `"<type> — <place text> — <date>"` для `Event` — используют те же `usePersonOptions()`/локальные division-lookup'ы, что и View-страницы. Не было указано дословно в задании, но необходимо — списковым страницам нужен хоть какой-то человекочитаемый текст ссылки.

## Задача 1. Бэкенд: Relation + Residence + Event/EventParticipant — полный стек + доки + real-store тесты

**Интерфейсы, потребляемые из подпроектов 1-8**: `models.Page`/`models.Access`, `store.Store.{GetPerson,GetAdministrativeDivision,GetCitation,InTx}` (generic-хранилище), `transport.{TextRef,FactDate,SourceLink}` + их конвертеры, `mcp/object_args.go`:`textRefObjectProperties`/`factDateObjectProperties`/`optionalTextRef`/`optionalFactDate`/`sourceLinkObjectProperties`/`optionalSourceLinks`/`optionalInt`/`toolJSONResult`/`textRefsFromStrings` (подпроекты 3, 5), `get_family`'s access-aware `Get`-паттерн, `create_family`/`create_person`'s проверка-FK-в-транзакции паттерн, `list_archive_nodes`'s full-scan-and-filter паттерн (подпроект 6, §3.4) — образец для `list_relations`/`list_residences`/`list_events`. Модели (`internal/models/{relation,residence,event,event_participant,place_ref}.go` + `*_validate.go`) уже существуют и не меняются этим подпроектом.

**Производит**: `models.{RelationQuery,ResidenceQuery,EventQuery}`, `transport.{Relation,Residence,Event,EventParticipant,PlaceRef}` + Create/Update-варианты, `httpapi.{RelationService,ResidenceService,EventService}`, `mcp.{RelationService,ResidenceService,EventService}`, HTTP-роуты `/api/{relations,residences,events}*`, MCP-тулы `relation_*`/`residence_*` (по 5) и `event_*` (6, включая `event_search`), хелперы `placeRefObjectProperties`/`optionalPlaceRef`/`eventParticipantObjectProperties`/`optionalEventParticipants` в `internal/mcp/object_args.go` — потребляются Задачей 2 (веб-страницы, `PersonPicker`, `EventParticipantListEditor`).

**Файлы:**
- Изменить: `internal/models/query.go`, `internal/httpapi/{deps.go,api.go,httpapi.go,write_store_test.go,store_test.go}`, `internal/mcp/{deps.go,server.go,object_args.go}`, `internal/app/app.go`, `docs/usage.md`, `docs/architecture.md`, `CHANGELOG.md`, `docs/data-model/entity-write.md`
- Создать: `internal/transport/{place_ref,event_participant,relation,relation_write,residence,residence_write,event,event_write}.go`, `internal/usecases/{list,get,create,update,delete}_relation/{deps.go,scenario.go,scenario_test.go}`, `internal/usecases/{list,get,create,update,delete}_residence/{deps.go,scenario.go,scenario_test.go}`, `internal/usecases/{list,search,get,create,update,delete}_event/{deps.go,scenario.go,scenario_test.go}`, `internal/httpapi/{relation,relation_write,relation_test,relation_write_test,residence,residence_write,residence_test,residence_write_test,event,event_write,event_test,event_write_test}.go`, `internal/mcp/{relation,relation_test,residence,residence_test,event,event_test}.go`

**Важно для исполнителя**: `list_relations`/`list_residences`/`list_events` — full-scan-and-filter по образцу `list_archive_nodes`, НЕ новые методы `store.Store`. `get_relation`/`get_residence`/`get_event` — access-aware с первого черновика (все три несут `Private`). `create_relation`/`update_relation` проверяют `person_a`/`person_b` НЕЗАВИСИМО (обе стороны, не короткое замыкание на первой ошибке); `create_residence`/`update_residence` — `person_id` (через `tx.GetPerson`) и `place_id` (через `tx.GetAdministrativeDivision`, СТРОГО этот тип, не любой `PlaceRef`); `create_event`/`update_event` — цикл по `Participants[i].PersonID` (через `tx.GetPerson`, индексированная ошибка) НЕЗАВИСИМЫЙ от цикла по `Sources[i].CitationID`; `Place` НИКОГДА не проверяется на существование ни в одном из трёх. `internal/mcp/object_args.go` — единственный файл в этой задаче, где меняется НЕ ВСЁ содержимое механически: добавляются ровно четыре новые функции (`placeRefObjectProperties`/`optionalPlaceRef`, `eventParticipantObjectProperties`/`optionalEventParticipants`), остальной файл (хелперы подпроектов 3, 5, 8) переносится как есть. Переносить код ниже как есть, файл за файлом — итоговое содержимое, не диффы (кроме секции документации, Шаг 1.9, — там даны точные фрагменты-вставки в существующие большие файлы).

### 1.1. Модель: `RelationQuery`/`ResidenceQuery`/`EventQuery`

`internal/models/query.go` — единственный модельный файл, который меняется этим подпроектом (сами `Relation`/`Residence`/`Event`/`EventParticipant`/`PlaceRef` и их `Validate()` уже существуют). Три новых типа запроса списка, добавленные в конец файла, в том же стиле, что и уже существующий `ArchiveNodeQuery`: `RelationQuery{PersonID *ID, Page}` — фильтр «эта персона — `PersonA` ИЛИ `PersonB`»; `ResidenceQuery{PersonID, PlaceID *ID, Page}` — оба фильтра пересекаются, если заданы вместе; `EventQuery{PersonID *ID, Page}` — фильтр «эта персона участвует» (совпадает с любым элементом `Participants`). У всех трёх `Validate()` проверяет отрицательные `limit`/`offset` и корректность формата `person_id`/`place_id` (если заданы) через уже существующий хелпер `idErr`; `RelationQuery`/`ResidenceQuery` — единственная прямая связь этого файла с решением не заводить `search_relations`/`search_residences` (замена которых они и есть, см. Архитектуру и §3.8 ниже).

#### `internal/models/query.go` (изменить — итоговое содержимое)

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

// RelationQuery — запрос списка рёбер графа родства: необязательный фильтр
// по персоне (совпадает с PersonA ИЛИ PersonB) — без него список плоский.
// Замена search_relations (см. docs/data-model/entity-write.md §3.8): у
// Relation нет собственных поисковых полей (индекс намеренно пуст), поэтому
// поиск по персоне полезнее полнотекстового.
type RelationQuery struct {
	PersonID *ID
	Page     Page
}

// Validate проверяет запрос: person_id (если задан) — валидный id персоны;
// отрицательные размер и сдвиг окна — *ValidationError. Размер окна больше
// MaxPageLimit не ошибка: Page.Normalized сужает его.
func (q RelationQuery) Validate() error {
	switch {
	case q.Page.Limit < 0:
		return fieldErr("limit", "не может быть отрицательным: %d", q.Page.Limit)
	case q.Page.Offset < 0:
		return fieldErr("offset", "не может быть отрицательным: %d", q.Page.Offset)
	}

	if q.PersonID != nil {
		if err := idErr("person_id", *q.PersonID, TypePerson); err != nil {
			return err
		}
	}

	return nil
}

// ResidenceQuery — запрос списка проживаний: необязательные фильтры по
// персоне и по месту (пересекаются, если оба заданы). Замена
// search_residences (см. docs/data-model/entity-write.md §3.8): у Residence
// нет собственных поисковых полей (индекс намеренно пуст).
type ResidenceQuery struct {
	PersonID *ID
	PlaceID  *ID
	Page     Page
}

// Validate проверяет запрос: person_id/place_id (если заданы) — валидные id
// персоны/единицы административного деления; отрицательные размер и сдвиг
// окна — *ValidationError.
func (q ResidenceQuery) Validate() error {
	switch {
	case q.Page.Limit < 0:
		return fieldErr("limit", "не может быть отрицательным: %d", q.Page.Limit)
	case q.Page.Offset < 0:
		return fieldErr("offset", "не может быть отрицательным: %d", q.Page.Offset)
	}

	if q.PersonID != nil {
		if err := idErr("person_id", *q.PersonID, TypePerson); err != nil {
			return err
		}
	}

	if q.PlaceID != nil {
		if err := idErr("place_id", *q.PlaceID, TypeAdministrativeDivision); err != nil {
			return err
		}
	}

	return nil
}

// EventQuery — запрос списка событий: необязательный фильтр по участнику
// (совпадает с любым из Participants[i].PersonID).
type EventQuery struct {
	PersonID *ID
	Page     Page
}

// Validate проверяет запрос: person_id (если задан) — валидный id персоны;
// отрицательные размер и сдвиг окна — *ValidationError.
func (q EventQuery) Validate() error {
	switch {
	case q.Page.Limit < 0:
		return fieldErr("limit", "не может быть отрицательным: %d", q.Page.Limit)
	case q.Page.Offset < 0:
		return fieldErr("offset", "не может быть отрицательным: %d", q.Page.Offset)
	}

	if q.PersonID != nil {
		if err := idErr("person_id", *q.PersonID, TypePerson); err != nil {
			return err
		}
	}

	return nil
}
```

### 1.2. Транспорт

Восемь новых файлов транспорта. `transport.PlaceRef` (по образцу `transport.TextRef`, но отдельный тип — `models.PlaceRef` отдельный тип модели, JSON-форма та же `{text, ref?, type?}`) и `transport.EventParticipant` (флат-объект `{person_id, role, note?}`, по образцу `transport.Anchor`/`transport.PersonName` — «собственный файл под вложенный не-полиморфный DTO», подпроекты 5 и 8, но проще обоих — никаких вложенных объектов внутри) — общие строительные блоки, нужны раньше остальных. Затем `Relation`/`Residence`/`Event`, каждая — пара файлов `<entity>.go` (read-контракт + конвертер из модели) и `<entity>_write.go` (`<Entity>Create`/`<Entity>Update` — тело `POST`/`PUT` и аргументы MCP-тулов `*_create`/`*_update`).

#### `internal/transport/place_ref.go` (создать)

`transport.PlaceRef` — контракт `models.PlaceRef` (мягкая ссылка «место»: текст или ссылка, ограниченная типом `admin_division`/`church`/`parish`). Структурно идентичен `transport.TextRef` (`{text, ref?, type?}`), но заводится отдельным типом, потому что `models.PlaceRef` — отдельный тип домена, не алиас `models.TextRef`.

```go
package transport

import "github.com/amarin/genodex/internal/models"

// PlaceRef — контракт «указания на место» (models.PlaceRef): текст или
// ссылка, чей тип ограничен местом (административное деление, церковь или
// приход). Структурно идентичен TextRef ({text, ref, type}), но это
// отдельный тип модельного слоя (models.PlaceRef != models.TextRef), поэтому
// заводится собственный транспортный тип и собственные конвертеры, а не
// переиспользуется transport.TextRef. Единственное текущее поле такой формы
// в программе — Event.Place (docs/data-model/entity-write.md §3.8).
type PlaceRef struct {
	Text string `json:"text"`
	Ref  string `json:"ref,omitempty"`
	Type string `json:"type,omitempty"`
}

// PlaceRefFromModel конвертирует указание на место в контракт; nil — не задано.
func PlaceRefFromModel(p *models.PlaceRef) *PlaceRef {
	if p == nil {
		return nil
	}

	return &PlaceRef{Text: p.Text, Ref: string(p.Ref), Type: string(p.Type)}
}

// Model конвертирует контракт обратно в модель; nil — не задано.
func (p *PlaceRef) Model() *models.PlaceRef {
	if p == nil {
		return nil
	}

	return &models.PlaceRef{Text: p.Text, Ref: models.ID(p.Ref), Type: models.Type(p.Type)}
}
```

#### `internal/transport/event_participant.go` (создать)

`transport.EventParticipant` — контракт `models.EventParticipant`: `{person_id, role, note?}`, простой флат-объект без вложенных объектов внутри (проще `PersonName`/`SourceLink`) — вся сложность этой сущности не в форме контракта, а в том, что `person_id` требует проверки существования на уровне usecase (см. Шаг 1.3).

```go
package transport

import "github.com/amarin/genodex/internal/models"

// EventParticipant — контракт одного участника события (models.
// EventParticipant): флат-объект {person_id, role, note} — в отличие от
// PersonName (подпроект 8), собственных вложенных объектов не несёт.
// PersonID — первая СТРОГАЯ (проверяемая на существование) ссылка внутри
// массива-объектов MCP-аргумента в программе (docs/data-model/entity-write.md
// §3.8): PersonName/SourceLink несли только мягкие/уже-установленные ссылки.
type EventParticipant struct {
	PersonID string `json:"person_id"`
	Role     string `json:"role"`
	Note     string `json:"note,omitempty"`
}

// EventParticipantFromModel конвертирует участника в контракт.
func EventParticipantFromModel(p models.EventParticipant) EventParticipant {
	return EventParticipant{
		PersonID: string(p.PersonID),
		Role:     p.Role,
		Note:     p.Note,
	}
}

// EventParticipantsFromModel конвертирует список; пустой вход даёт пустой
// срез, а не nil.
func EventParticipantsFromModel(ps []models.EventParticipant) []EventParticipant {
	out := make([]EventParticipant, 0, len(ps))
	for _, p := range ps {
		out = append(out, EventParticipantFromModel(p))
	}

	return out
}

// Model конвертирует контракт обратно в модель.
func (p EventParticipant) Model() models.EventParticipant {
	return models.EventParticipant{
		PersonID: models.ID(p.PersonID),
		Role:     p.Role,
		Note:     p.Note,
	}
}

// EventParticipantsToModel конвертирует список контрактов в модели; пустой
// вход даёт пустой срез, а не nil.
func EventParticipantsToModel(ps []EventParticipant) []models.EventParticipant {
	out := make([]models.EventParticipant, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.Model())
	}

	return out
}
```

#### `internal/transport/relation.go` (создать)

`transport.Relation` — read-контракт: `{id, kind, rel_type?, person_a, person_b, since?, until?, sources, notes, private}` + `RelationFromModel`/`RelationsFromModels`.

```go
package transport

import "github.com/amarin/genodex/internal/models"

// Relation — контракт ребра графа родства (GET /api/relations, MCP-тул
// relation_list). Sources редактируется с рождения контракта (сущность
// заведена уже после подпроекта 5). Первая сущность программы с двумя
// строгими ссылками на один и тот же тип (PersonA/PersonB → Person, см.
// docs/data-model/entity-write.md §3.8).
type Relation struct {
	ID      models.ID    `json:"id"`
	Kind    string       `json:"kind"`
	RelType string       `json:"rel_type,omitempty"`
	PersonA models.ID    `json:"person_a"`
	PersonB models.ID    `json:"person_b"`
	Since   *FactDate    `json:"since,omitempty"`
	Until   *FactDate    `json:"until,omitempty"`
	Sources []SourceLink `json:"sources"`
	Notes   []TextRef    `json:"notes"`
	Private bool         `json:"private"`
}

// RelationFromModel конвертирует запись в контракт.
func RelationFromModel(r models.Relation) Relation {
	return Relation{
		ID:      r.ID,
		Kind:    string(r.Kind),
		RelType: string(r.RelType),
		PersonA: r.PersonA,
		PersonB: r.PersonB,
		Since:   FactDateFromModel(r.Since),
		Until:   FactDateFromModel(r.Until),
		Sources: SourceLinksFromModel(r.Sources),
		Notes:   TextRefsFromModel(r.Notes),
		Private: r.Private,
	}
}

// RelationsFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func RelationsFromModels(rs []models.Relation) []Relation {
	out := make([]Relation, 0, len(rs))
	for _, r := range rs {
		out = append(out, RelationFromModel(r))
	}

	return out
}
```

#### `internal/transport/relation_write.go` (создать)

`transport.RelationCreate`/`RelationUpdate` — тело `POST`/`PUT /api/relations*` и аргументы `relation_create`/`relation_update`.

```go
package transport

import "github.com/amarin/genodex/internal/models"

// RelationCreate — тело POST /api/relations и аргументы тула
// relation_create. Идентификатор генерирует сценарий.
type RelationCreate struct {
	Kind    string       `json:"kind"`
	RelType string       `json:"rel_type,omitempty"`
	PersonA string       `json:"person_a"`
	PersonB string       `json:"person_b"`
	Since   *FactDate    `json:"since,omitempty"`
	Until   *FactDate    `json:"until,omitempty"`
	Sources []SourceLink `json:"sources"`
	Notes   []TextRef    `json:"notes"`
	Private bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (r RelationCreate) Model() models.Relation {
	return models.Relation{
		Kind:    models.RelationKind(r.Kind),
		RelType: models.RelationType(r.RelType),
		PersonA: models.ID(r.PersonA),
		PersonB: models.ID(r.PersonB),
		Since:   r.Since.Model(),
		Until:   r.Until.Model(),
		Sources: SourceLinksToModel(r.Sources),
		Notes:   TextRefsToModel(r.Notes),
		Private: r.Private,
	}
}

// RelationUpdate — тело PUT /api/relations/{id} и аргументы тула
// relation_update: полная замена kind/rel_type/person_a/person_b/since/
// until/sources/notes/private.
type RelationUpdate struct {
	Kind    string       `json:"kind"`
	RelType string       `json:"rel_type,omitempty"`
	PersonA string       `json:"person_a"`
	PersonB string       `json:"person_b"`
	Since   *FactDate    `json:"since,omitempty"`
	Until   *FactDate    `json:"until,omitempty"`
	Sources []SourceLink `json:"sources"`
	Notes   []TextRef    `json:"notes"`
	Private bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (r RelationUpdate) Model() models.Relation {
	return models.Relation{
		Kind:    models.RelationKind(r.Kind),
		RelType: models.RelationType(r.RelType),
		PersonA: models.ID(r.PersonA),
		PersonB: models.ID(r.PersonB),
		Since:   r.Since.Model(),
		Until:   r.Until.Model(),
		Sources: SourceLinksToModel(r.Sources),
		Notes:   TextRefsToModel(r.Notes),
		Private: r.Private,
	}
}
```

#### `internal/transport/residence.go` (создать)

`transport.Residence` — read-контракт: `{id, person_id, place_id, since?, until?, sources, note?, private}` — `note` ОДНА строка (JSON-тег `note`, не список).

```go
package transport

import "github.com/amarin/genodex/internal/models"

// Residence — контракт проживания персоны в месте (GET /api/residences,
// MCP-тул residence_list). Note — единственная строка (не []TextRef, в
// отличие от Notes у большинства сущностей) — отражает models.Residence.Note
// как есть.
type Residence struct {
	ID       models.ID    `json:"id"`
	PersonID models.ID    `json:"person_id"`
	PlaceID  models.ID    `json:"place_id"`
	Since    *FactDate    `json:"since,omitempty"`
	Until    *FactDate    `json:"until,omitempty"`
	Sources  []SourceLink `json:"sources"`
	Note     string       `json:"note,omitempty"`
	Private  bool         `json:"private"`
}

// ResidenceFromModel конвертирует запись в контракт.
func ResidenceFromModel(r models.Residence) Residence {
	return Residence{
		ID:       r.ID,
		PersonID: r.PersonID,
		PlaceID:  r.PlaceID,
		Since:    FactDateFromModel(r.Since),
		Until:    FactDateFromModel(r.Until),
		Sources:  SourceLinksFromModel(r.Sources),
		Note:     r.Note,
		Private:  r.Private,
	}
}

// ResidencesFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func ResidencesFromModels(rs []models.Residence) []Residence {
	out := make([]Residence, 0, len(rs))
	for _, r := range rs {
		out = append(out, ResidenceFromModel(r))
	}

	return out
}
```

#### `internal/transport/residence_write.go` (создать)

`transport.ResidenceCreate`/`ResidenceUpdate`.

```go
package transport

import "github.com/amarin/genodex/internal/models"

// ResidenceCreate — тело POST /api/residences и аргументы тула
// residence_create. Идентификатор генерирует сценарий.
type ResidenceCreate struct {
	PersonID string       `json:"person_id"`
	PlaceID  string       `json:"place_id"`
	Since    *FactDate    `json:"since,omitempty"`
	Until    *FactDate    `json:"until,omitempty"`
	Sources  []SourceLink `json:"sources"`
	Note     string       `json:"note,omitempty"`
	Private  bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (r ResidenceCreate) Model() models.Residence {
	return models.Residence{
		PersonID: models.ID(r.PersonID),
		PlaceID:  models.ID(r.PlaceID),
		Since:    r.Since.Model(),
		Until:    r.Until.Model(),
		Sources:  SourceLinksToModel(r.Sources),
		Note:     r.Note,
		Private:  r.Private,
	}
}

// ResidenceUpdate — тело PUT /api/residences/{id} и аргументы тула
// residence_update: полная замена person_id/place_id/since/until/sources/
// note/private.
type ResidenceUpdate struct {
	PersonID string       `json:"person_id"`
	PlaceID  string       `json:"place_id"`
	Since    *FactDate    `json:"since,omitempty"`
	Until    *FactDate    `json:"until,omitempty"`
	Sources  []SourceLink `json:"sources"`
	Note     string       `json:"note,omitempty"`
	Private  bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (r ResidenceUpdate) Model() models.Residence {
	return models.Residence{
		PersonID: models.ID(r.PersonID),
		PlaceID:  models.ID(r.PlaceID),
		Since:    r.Since.Model(),
		Until:    r.Until.Model(),
		Sources:  SourceLinksToModel(r.Sources),
		Note:     r.Note,
		Private:  r.Private,
	}
}
```

#### `internal/transport/event.go` (создать)

`transport.Event` — read-контракт: `{id, type, date?, place?, participants, sources, notes, private}`; `place` — `*transport.PlaceRef`, `participants` — `[]transport.EventParticipant`.

```go
package transport

import "github.com/amarin/genodex/internal/models"

// Event — контракт события жизненного факта (GET /api/events, MCP-тул
// event_list). Sources редактируется с рождения контракта. Поиск
// (event_search, GET /api/events/search) ищет ТОЛЬКО по началу текста места
// (place) — единственное индексируемое поле события
// (internal/store/sqlstore/records.go:SaveEvent); type/date/участники
// поиском не охвачены.
type Event struct {
	ID           models.ID          `json:"id"`
	Type         string             `json:"type"`
	Date         *FactDate          `json:"date,omitempty"`
	Place        *PlaceRef          `json:"place,omitempty"`
	Participants []EventParticipant `json:"participants"`
	Sources      []SourceLink       `json:"sources"`
	Notes        []TextRef          `json:"notes"`
	Private      bool               `json:"private"`
}

// EventFromModel конвертирует запись в контракт.
func EventFromModel(e models.Event) Event {
	return Event{
		ID:           e.ID,
		Type:         string(e.Type),
		Date:         FactDateFromModel(e.Date),
		Place:        PlaceRefFromModel(e.Place),
		Participants: EventParticipantsFromModel(e.Participants),
		Sources:      SourceLinksFromModel(e.Sources),
		Notes:        TextRefsFromModel(e.Notes),
		Private:      e.Private,
	}
}

// EventsFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func EventsFromModels(es []models.Event) []Event {
	out := make([]Event, 0, len(es))
	for _, e := range es {
		out = append(out, EventFromModel(e))
	}

	return out
}
```

#### `internal/transport/event_write.go` (создать)

`transport.EventCreate`/`EventUpdate`.

```go
package transport

import "github.com/amarin/genodex/internal/models"

// EventCreate — тело POST /api/events и аргументы тула event_create.
// Идентификатор генерирует сценарий.
type EventCreate struct {
	Type         string             `json:"type"`
	Date         *FactDate          `json:"date,omitempty"`
	Place        *PlaceRef          `json:"place,omitempty"`
	Participants []EventParticipant `json:"participants"`
	Sources      []SourceLink       `json:"sources"`
	Notes        []TextRef          `json:"notes"`
	Private      bool               `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (e EventCreate) Model() models.Event {
	return models.Event{
		Type:         models.EventType(e.Type),
		Date:         e.Date.Model(),
		Place:        e.Place.Model(),
		Participants: EventParticipantsToModel(e.Participants),
		Sources:      SourceLinksToModel(e.Sources),
		Notes:        TextRefsToModel(e.Notes),
		Private:      e.Private,
	}
}

// EventUpdate — тело PUT /api/events/{id} и аргументы тула event_update:
// полная замена type/date/place/participants/sources/notes/private.
type EventUpdate struct {
	Type         string             `json:"type"`
	Date         *FactDate          `json:"date,omitempty"`
	Place        *PlaceRef          `json:"place,omitempty"`
	Participants []EventParticipant `json:"participants"`
	Sources      []SourceLink       `json:"sources"`
	Notes        []TextRef          `json:"notes"`
	Private      bool               `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (e EventUpdate) Model() models.Event {
	return models.Event{
		Type:         models.EventType(e.Type),
		Date:         e.Date.Model(),
		Place:        e.Place.Model(),
		Participants: EventParticipantsToModel(e.Participants),
		Sources:      SourceLinksToModel(e.Sources),
		Notes:        TextRefsToModel(e.Notes),
		Private:      e.Private,
	}
}
```

### 1.3. Usecase-сценарии

16 новых пакетов, каждый — `deps.go` (интерфейс(ы) зависимости usecase от хранилища, `RelationRepo`/`RelationStore` и т.п., по образцу существующих `PersonRepo`/`PersonStore`) + `scenario.go` + `scenario_test.go` (фейковый стор в памяти). `list_relations`/`get_relation`/`create_relation`/`update_relation`/`delete_relation` — образец для остальных пяти-операционных сущностей (`*_residence`, зеркально, с заменой `person_b`-проверки на `place_id`-проверку через `tx.GetAdministrativeDivision`); `Event` получает шестой, `search_events`, по образцу `search_families`.

**Relation** (5 пакетов):

#### `internal/usecases/list_relations/`

`ListRelations` — full-scan-and-filter по `models.RelationQuery` (см. Шаг 1.1): фильтр «персона совпадает с `PersonA` ИЛИ `PersonB`», окно применяется ПОСЛЕ фильтра (`applyWindow`/`matchesPerson`, тот же приём накопления через generic-окна `MaxPageLimit`, что `list_archive_nodes`). Заменяет отсутствующий `search_relations` — см. §3.8.

#### `internal/usecases/list_relations/deps.go` (создать)

```go
package list_relations

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// RelationRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type RelationRepo interface {
	ListRelations(ctx context.Context, access models.Access, page models.Page) ([]*models.Relation, error)
}
```

#### `internal/usecases/list_relations/scenario.go` (создать)

```go
package list_relations

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список рёбер графа родства».
type Scenario struct {
	relations RelationRepo
}

// New создаёт сценарий.
func New(relations RelationRepo) *Scenario {
	return &Scenario{relations: relations}
}

// ListRelations возвращает рёбра, прошедшие фильтр запроса (q.PersonID,
// если задан — ребро проходит, если совпадает с PersonA ИЛИ PersonB), в
// порядке сохранения; окно применяется после фильтра. Некорректный запрос —
// *models.ValidationError, репозиторий не вызывается. Короткий результат
// (меньше размера окна) означает конец списка.
//
// У Relation нет собственных поисковых полей (search-индекс намеренно пуст,
// docs/data-model/entity-write.md §3.8) — этот фильтр заменяет
// search_relations, которого в этой программе нет. Полное сканирование по
// generic-окнам ListRelations — тот же приём, что и list_archive_nodes
// (подпроект 6): выделенный метод хранилища не оправдан при текущем объёме
// данных.
func (s *Scenario) ListRelations(ctx context.Context, access models.Access, q models.RelationQuery) ([]models.Relation, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	page := q.Page.Normalized()
	out := []models.Relation{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		list, err := s.relations.ListRelations(ctx, access, models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(list) == 0 {
			return out, nil
		}

		var full bool
		if full, matched, out = applyWindow(q, list, page, matched, out); full {
			return out, nil
		}
	}
}

// applyWindow прогоняет одно окно репозитория через фильтр запроса и
// накапливает результат окна запроса. full — окно запроса заполнено (обход
// можно остановить).
func applyWindow(q models.RelationQuery, list []*models.Relation,
	page models.Page, matched int, out []models.Relation,
) (full bool, nextMatched int, nextOut []models.Relation) {
	for _, r := range list {
		if !matchesPerson(q.PersonID, r.PersonA, r.PersonB) {
			continue
		}

		if matched >= page.Offset {
			out = append(out, *r)

			if len(out) == page.Limit {
				return true, matched, out
			}
		}

		matched++
	}

	return false, matched, out
}

// matchesPerson сообщает, проходит ли ребро фильтр по персоне: nil — без
// фильтра (проходит всё); иначе ребро проходит, если персона совпадает с
// одной из его сторон.
func matchesPerson(want *models.ID, a, b models.ID) bool {
	return want == nil || *want == a || *want == b
}
```

#### `internal/usecases/list_relations/scenario_test.go` (создать)

```go
package list_relations

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	list  []*models.Relation
	err   error
	calls []models.Page
}

func window(list []*models.Relation, page models.Page) []*models.Relation {
	page = page.Normalized()
	if page.Offset >= len(list) {
		return nil
	}

	return list[page.Offset:min(page.Offset+page.Limit, len(list))]
}

func (f *fakeRepo) ListRelations(_ context.Context, _ models.Access, page models.Page) ([]*models.Relation, error) {
	f.calls = append(f.calls, page)
	if f.err != nil {
		return nil, f.err
	}

	return window(f.list, page), nil
}

func pID(last byte) models.ID  { return models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }
func rlID(last byte) models.ID { return models.ID("RL-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }

func ids(list []models.Relation) []models.ID {
	out := make([]models.ID, 0, len(list))
	for _, r := range list {
		out = append(out, r.ID)
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

func sample() *fakeRepo {
	return &fakeRepo{list: []*models.Relation{
		{ID: rlID('1'), PersonA: pID('1'), PersonB: pID('2')},
		{ID: rlID('2'), PersonA: pID('3'), PersonB: pID('4')},
		{ID: rlID('3'), PersonA: pID('5'), PersonB: pID('1')}, // person1 as person_b
	}}
}

func TestListRelationsNoFilterReturnsAll(t *testing.T) {
	got, err := New(sample()).ListRelations(context.Background(), models.AccessFull, models.RelationQuery{})
	if err != nil || !sameIDs(ids(got), rlID('1'), rlID('2'), rlID('3')) {
		t.Fatalf("got %v, %v", ids(got), err)
	}
}

// TestListRelationsFilterMatchesEitherSide: person_id совпадает с person_a
// ИЛИ person_b — оба случая проходят фильтр.
func TestListRelationsFilterMatchesEitherSide(t *testing.T) {
	person1 := pID('1')

	got, err := New(sample()).ListRelations(context.Background(), models.AccessFull, models.RelationQuery{PersonID: &person1})
	if err != nil || !sameIDs(ids(got), rlID('1'), rlID('3')) {
		t.Fatalf("got %v, %v; ожидались rl1 (person_a) и rl3 (person_b)", ids(got), err)
	}
}

func TestListRelationsFilterNoMatch(t *testing.T) {
	other := pID('9')

	got, err := New(sample()).ListRelations(context.Background(), models.AccessFull, models.RelationQuery{PersonID: &other})
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v; ожидался пустой результат", ids(got), err)
	}
}

func TestListRelationsInvalidQuery(t *testing.T) {
	bad := models.ID("not-an-id")
	repo := sample()

	for _, q := range []models.RelationQuery{
		{PersonID: &bad},
		{Page: models.Page{Limit: -1}},
		{Page: models.Page{Offset: -1}},
	} {
		_, err := New(repo).ListRelations(context.Background(), models.AccessFull, q)

		var ve *models.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("запрос %+v: err = %v, ожидалась *ValidationError", q, err)
		}
	}

	if len(repo.calls) != 0 {
		t.Fatalf("репозиторий вызван %d раз при некорректном запросе", len(repo.calls))
	}
}

func TestListRelationsEmptyRepoGivesEmptyNotNil(t *testing.T) {
	got, err := New(&fakeRepo{}).ListRelations(context.Background(), models.AccessFull, models.RelationQuery{})
	if err != nil {
		t.Fatalf("ListRelations: %v", err)
	}

	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v, ожидался пустой не-nil срез", got)
	}
}

func TestListRelationsPropagatesRepoError(t *testing.T) {
	wantErr := errors.New("repo down")

	if _, err := New(&fakeRepo{err: wantErr}).ListRelations(context.Background(), models.AccessFull, models.RelationQuery{}); !errors.Is(err, wantErr) {
		t.Errorf("err=%v, want %v", err, wantErr)
	}
}

// TestListRelationsWindowAppliedAfterFilter: сдвиг и размер считаются среди
// прошедших фильтр, а не среди всех рёбер.
func TestListRelationsWindowAppliedAfterFilter(t *testing.T) {
	person1 := pID('1')
	q := models.RelationQuery{PersonID: &person1, Page: models.Page{Limit: 1, Offset: 1}}

	got, err := New(sample()).ListRelations(context.Background(), models.AccessFull, q)
	if err != nil || !sameIDs(ids(got), rlID('3')) {
		t.Fatalf("got %v, %v; ожидался второй элемент отфильтрованного списка (rl3)", ids(got), err)
	}
}
```

#### `internal/usecases/get_relation/`

`GetRelation` — access-aware чтение по id: неверный формат id — `*models.ValidationError` (репозиторий не вызывается); нет записи — `models.ErrNotFound`; приватная запись без `AccessFull` — тоже `models.ErrNotFound` (прячем как отсутствующее).

#### `internal/usecases/get_relation/deps.go` (создать)

```go
package get_relation

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// RelationRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type RelationRepo interface {
	GetRelation(ctx context.Context, id models.ID) (*models.Relation, error)
}
```

#### `internal/usecases/get_relation/scenario.go` (создать)

```go
package get_relation

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «ребро графа родства по идентификатору».
type Scenario struct {
	relations RelationRepo
}

// New создаёт сценарий.
func New(relations RelationRepo) *Scenario {
	return &Scenario{relations: relations}
}

// GetRelation возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound. Приватная запись
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound — тот же принцип «прячем как отсутствующее», что и в
// List/Search.
func (s *Scenario) GetRelation(ctx context.Context, access models.Access, id models.ID) (models.Relation, error) {
	if err := validateID(id); err != nil {
		return models.Relation{}, err
	}

	r, err := s.relations.GetRelation(ctx, id)
	if err != nil {
		return models.Relation{}, err
	}

	if r.Private && access != models.AccessFull {
		return models.Relation{}, models.ErrNotFound
	}

	return *r, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeRelation)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeRelation,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

#### `internal/usecases/get_relation/scenario_test.go` (создать)

```go
package get_relation

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	relations map[models.ID]*models.Relation
}

func (f *fakeRepo) GetRelation(_ context.Context, id models.ID) (*models.Relation, error) {
	r, ok := f.relations[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return r, nil
}

func TestGetRelationReturnsRecord(t *testing.T) {
	id := models.ID("RL-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{relations: map[models.ID]*models.Relation{id: {ID: id, Kind: models.RelationKindBlood}}}

	got, err := New(repo).GetRelation(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetRelation: %v", err)
	}

	if got.Kind != models.RelationKindBlood {
		t.Fatalf("Kind = %q", got.Kind)
	}
}

func TestGetRelationNotFound(t *testing.T) {
	repo := &fakeRepo{relations: map[models.ID]*models.Relation{}}

	_, err := New(repo).GetRelation(context.Background(), models.AccessFull, "RL-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetRelationInvalidID(t *testing.T) {
	repo := &fakeRepo{relations: map[models.ID]*models.Relation{}}

	_, err := New(repo).GetRelation(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestGetRelationPrivateHiddenFromPublic(t *testing.T) {
	id := models.ID("RL-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{relations: map[models.ID]*models.Relation{id: {ID: id, Private: true}}}

	_, err := New(repo).GetRelation(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for private record with AccessPublic", err)
	}
}

func TestGetRelationPrivateVisibleToFullAccess(t *testing.T) {
	id := models.ID("RL-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{relations: map[models.ID]*models.Relation{id: {ID: id, Private: true}}}

	got, err := New(repo).GetRelation(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetRelation: %v", err)
	}

	if !got.Private {
		t.Fatalf("Private = %v", got.Private)
	}
}
```

#### `internal/usecases/create_relation/`

`CreateRelation` — генерирует id, `Relation.Validate()` (различность `person_a`/`person_b`, условность `rel_type`), затем в ОДНОЙ транзакции: `tx.GetPerson(PersonA)` (422 `person_a`), `tx.GetPerson(PersonB)` (422 `person_b`, проверяется НЕЗАВИСИМО — не пропускается, даже если `person_a` уже не найден бы отдельно), цикл `Sources[i].CitationID` (422 `sources[i].citation_id`), `tx.SaveRelation`. Первая сущность программы с двумя строгими ссылками на ОДИН И ТОТ ЖЕ тип.

#### `internal/usecases/create_relation/deps.go` (создать)

```go
package create_relation

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// RelationStore — зависимость сценария: транзакция порта store.Store.
// Проверки ссылок (person_a/person_b/sources) и сохранение идут в одной
// транзакции на переданном fn хранилище.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type RelationStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
```

#### `internal/usecases/create_relation/scenario.go` (создать)

```go
package create_relation

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание ребра графа родства».
type Scenario struct {
	store RelationStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st RelationStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateRelation создаёт ребро: генерирует идентификатор, проверяет
// инварианты (Relation.Validate — в т.ч. различность person_a/person_b и
// условное правило rel_type), в одной транзакции убеждается в существовании
// обеих персон и цитат из Sources, сохраняет. Возвращает созданную запись с
// заполненным ID.
//
// Первая сущность программы с двумя строгими ссылками на один и тот же тип
// (PersonA/PersonB → Person, docs/data-model/entity-write.md §3.8): person_b
// проверяется даже если person_a уже не найден бы отдельно — обе стороны
// равноправны.
//
// Ошибки: непустой входной ID, невалидная сущность, несуществующая персона
// (поля person_a/person_b) и несуществующая цитата — *models.ValidationError;
// прочее — ошибки хранилища как есть.
func (s *Scenario) CreateRelation(ctx context.Context, r models.Relation) (models.Relation, error) {
	if r.ID != "" {
		return models.Relation{}, &models.ValidationError{
			Entity: models.TypeRelation,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", r.ID),
		}
	}

	r.ID = s.ids.New(models.TypeRelation)

	if err := r.Validate(); err != nil {
		return models.Relation{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetPerson(ctx, r.PersonA); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return personErr("person_a", "персона %q не найдена", r.PersonA)
			}

			return err
		}

		if _, err := tx.GetPerson(ctx, r.PersonB); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return personErr("person_b", "персона %q не найдена", r.PersonB)
			}

			return err
		}

		for i, link := range r.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveRelation(ctx, &r)
	})
	if err != nil {
		return models.Relation{}, err
	}

	return r, nil
}

// personErr — *models.ValidationError по полю person_a/person_b.
func personErr(field, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeRelation,
		Field:  field,
		Reason: fmt.Sprintf(format, args...),
	}
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeRelation,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/create_relation/scenario_test.go` (создать)

```go
package create_relation

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

func pID(last byte) models.ID { return models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }
func cID(last byte) models.ID { return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }

type stubIDs struct{ id models.ID }

func (s *stubIDs) New(models.Type) models.ID { return s.id }

// fakeTx реализует нужные сценарию методы store.Store поверх карт персон и
// цитат; остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	people    map[models.ID]*models.Person
	citations map[models.ID]*models.Citation
	saved     *models.Relation
	saveErr   error
}

func newFakeTx(people []models.ID, citations ...*models.Citation) *fakeTx {
	tx := &fakeTx{people: map[models.ID]*models.Person{}, citations: map[models.ID]*models.Citation{}}
	for _, id := range people {
		tx.people[id] = &models.Person{ID: id}
	}
	for _, c := range citations {
		tx.citations[c.ID] = c
	}

	return tx
}

func (f *fakeTx) GetPerson(_ context.Context, id models.ID) (*models.Person, error) {
	p, ok := f.people[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *p

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

func (f *fakeTx) SaveRelation(_ context.Context, r *models.Relation) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *r
	f.saved = &cp

	return nil
}

type fakeStore struct{ tx *fakeTx }

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	return fn(f.tx)
}

func TestCreateRelationGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "RL-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{tx: newFakeTx([]models.ID{pID('1'), pID('2')})}
	sc := New(st, ids)

	got, err := sc.CreateRelation(context.Background(), models.Relation{
		Kind: models.RelationKindBlood, PersonA: pID('1'), PersonB: pID('2'),
	})
	if err != nil {
		t.Fatalf("CreateRelation: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.tx.saved == nil || st.tx.saved.PersonA != pID('1') || st.tx.saved.PersonB != pID('2') {
		t.Fatalf("saved = %+v", st.tx.saved)
	}
}

func TestCreateRelationRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx(nil)}, &stubIDs{})

	_, err := sc.CreateRelation(context.Background(), models.Relation{
		ID: "RL-01ARZ3NDEKTSV4RRFFQ69G5FA1", Kind: models.RelationKindBlood, PersonA: pID('1'), PersonB: pID('2'),
	})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestCreateRelationRejectsSamePerson(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx(nil)}, &stubIDs{id: "RL-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateRelation(context.Background(), models.Relation{
		Kind: models.RelationKindBlood, PersonA: pID('1'), PersonB: pID('1'),
	})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "person_b" {
		t.Fatalf("err = %v, want ValidationError on person_b (модельная валидация, до InTx)", err)
	}
}

// TestCreateRelationPersonANotFound: несуществующий person_a — 422 на поле
// person_a, ничего не сохраняется.
func TestCreateRelationPersonANotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx([]models.ID{pID('2')})}
	sc := New(st, &stubIDs{id: "RL-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateRelation(context.Background(), models.Relation{
		Kind: models.RelationKindBlood, PersonA: pID('1'), PersonB: pID('2'),
	})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "person_a" {
		t.Fatalf("err = %v, want ValidationError on person_a", err)
	}

	if st.tx.saved != nil {
		t.Fatalf("saved = %+v; ожидалось: ничего не сохранено", st.tx.saved)
	}
}

// TestCreateRelationPersonBNotFound: person_a существует, но person_b — нет
// — 422 на поле person_b (обе стороны проверяются независимо).
func TestCreateRelationPersonBNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx([]models.ID{pID('1')})}
	sc := New(st, &stubIDs{id: "RL-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateRelation(context.Background(), models.Relation{
		Kind: models.RelationKindBlood, PersonA: pID('1'), PersonB: pID('2'),
	})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "person_b" {
		t.Fatalf("err = %v, want ValidationError on person_b", err)
	}
}

func TestCreateRelationSourceCitationNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx([]models.ID{pID('1'), pID('2')})}
	sc := New(st, &stubIDs{id: "RL-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	in := models.Relation{
		Kind: models.RelationKindBlood, PersonA: pID('1'), PersonB: pID('2'),
		Sources: []models.SourceLink{{CitationID: cID('0')}},
	}

	_, err := sc.CreateRelation(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, want ValidationError on sources[0].citation_id", err)
	}
}

func TestCreateRelationAssociateRequiresRelType(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx([]models.ID{pID('1'), pID('2')})}, &stubIDs{id: "RL-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateRelation(context.Background(), models.Relation{
		Kind: models.RelationKindAssociate, PersonA: pID('1'), PersonB: pID('2'),
	})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "rel_type" {
		t.Fatalf("err = %v, want ValidationError on rel_type (модельная валидация)", err)
	}
}
```

#### `internal/usecases/update_relation/`

`UpdateRelation` — полная замена по `r.ID`: `Relation.Validate()`, затем в транзакции `tx.GetRelation(r.ID)` (существование записи), те же проверки `person_a`/`person_b`/`sources[i].citation_id`, что и `create_relation`, `tx.SaveRelation`.

#### `internal/usecases/update_relation/deps.go` (создать)

```go
package update_relation

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// RelationStore — зависимость сценария: транзакция порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type RelationStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
```

#### `internal/usecases/update_relation/scenario.go` (создать)

```go
package update_relation

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение ребра графа родства».
type Scenario struct {
	store RelationStore
}

// New создаёт сценарий.
func New(st RelationStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateRelation полностью заменяет запись по r.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, обе персоны (person_a/
// person_b) и цитаты из Sources существуют, и сохраняет.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; несуществующая персона/цитата — *models.ValidationError;
// прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateRelation(ctx context.Context, r models.Relation) error {
	if err := r.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetRelation(ctx, r.ID); err != nil {
			return err
		}

		if _, err := tx.GetPerson(ctx, r.PersonA); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return personErr("person_a", "персона %q не найдена", r.PersonA)
			}

			return err
		}

		if _, err := tx.GetPerson(ctx, r.PersonB); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return personErr("person_b", "персона %q не найдена", r.PersonB)
			}

			return err
		}

		for i, link := range r.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveRelation(ctx, &r)
	})
}

// personErr — *models.ValidationError по полю person_a/person_b.
func personErr(field, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeRelation,
		Field:  field,
		Reason: fmt.Sprintf(format, args...),
	}
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeRelation,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/update_relation/scenario_test.go` (создать)

```go
package update_relation

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

func pID(last byte) models.ID { return models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }
func cID(last byte) models.ID { return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }

// fakeTx реализует нужные сценарию методы store.Store поверх карт записей;
// остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	relations map[models.ID]*models.Relation
	people    map[models.ID]*models.Person
	citations map[models.ID]*models.Citation
	saved     []*models.Relation
}

func newFakeTx(existing *models.Relation, people ...models.ID) *fakeTx {
	tx := &fakeTx{relations: map[models.ID]*models.Relation{}, people: map[models.ID]*models.Person{}, citations: map[models.ID]*models.Citation{}}
	if existing != nil {
		tx.relations[existing.ID] = existing
	}
	for _, id := range people {
		tx.people[id] = &models.Person{ID: id}
	}

	return tx
}

func (f *fakeTx) GetRelation(_ context.Context, id models.ID) (*models.Relation, error) {
	r, ok := f.relations[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *r

	return &cp, nil
}

func (f *fakeTx) GetPerson(_ context.Context, id models.ID) (*models.Person, error) {
	p, ok := f.people[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *p

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

func (f *fakeTx) SaveRelation(_ context.Context, r *models.Relation) error {
	cp := *r
	f.saved = append(f.saved, &cp)

	return nil
}

type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func relation(id models.ID, a, b models.ID) *models.Relation {
	return &models.Relation{ID: id, Kind: models.RelationKindBlood, PersonA: a, PersonB: b}
}

func TestUpdateRelationSaves(t *testing.T) {
	existing := relation("RL-01ARZ3NDEKTSV4RRFFQ69G5FA1", pID('1'), pID('2'))
	st := &fakeStore{tx: newFakeTx(existing, pID('1'), pID('2'))}

	updated := *existing
	updated.Private = true

	if err := New(st).UpdateRelation(context.Background(), updated); err != nil {
		t.Fatalf("UpdateRelation: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || !st.tx.saved[0].Private {
		t.Fatalf("calls=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateRelationNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx(nil, pID('1'), pID('2'))}

	err := New(st).UpdateRelation(context.Background(), *relation("RL-01ARZ3NDEKTSV4RRFFQ69G5FA1", pID('1'), pID('2')))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateRelationPersonANotFound(t *testing.T) {
	existing := relation("RL-01ARZ3NDEKTSV4RRFFQ69G5FA1", pID('1'), pID('2'))
	st := &fakeStore{tx: newFakeTx(existing, pID('2'))}

	err := New(st).UpdateRelation(context.Background(), *existing)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "person_a" {
		t.Fatalf("err = %v, want ValidationError on person_a", err)
	}
}

func TestUpdateRelationPersonBNotFound(t *testing.T) {
	existing := relation("RL-01ARZ3NDEKTSV4RRFFQ69G5FA1", pID('1'), pID('2'))
	st := &fakeStore{tx: newFakeTx(existing, pID('1'))}

	err := New(st).UpdateRelation(context.Background(), *existing)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "person_b" {
		t.Fatalf("err = %v, want ValidationError on person_b", err)
	}
}

func TestUpdateRelationSourceCitationNotFound(t *testing.T) {
	existing := relation("RL-01ARZ3NDEKTSV4RRFFQ69G5FA1", pID('1'), pID('2'))
	st := &fakeStore{tx: newFakeTx(existing, pID('1'), pID('2'))}

	updated := *existing
	updated.Sources = []models.SourceLink{{CitationID: cID('0')}}

	err := New(st).UpdateRelation(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, want ValidationError on sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d записей при несуществующей цитате", len(st.tx.saved))
	}
}

func TestUpdateRelationInvalidRejectedBeforeInTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx(nil)}

	err := New(st).UpdateRelation(context.Background(), models.Relation{ID: "RL-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("err = %v, want ValidationError", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}
```

#### `internal/usecases/delete_relation/`

`DeleteRelation` — валидация формата id, затем `relations.DeleteRelation`; занятая другой сущностью запись — `*models.InUseError` из самого хранилища.

#### `internal/usecases/delete_relation/deps.go` (создать)

```go
package delete_relation

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// RelationRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type RelationRepo interface {
	DeleteRelation(ctx context.Context, id models.ID) error
}
```

#### `internal/usecases/delete_relation/scenario.go` (создать)

```go
package delete_relation

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление ребра графа родства».
type Scenario struct {
	relations RelationRepo
}

// New создаёт сценарий.
func New(relations RelationRepo) *Scenario {
	return &Scenario{relations: relations}
}

// DeleteRelation удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteRelation(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.relations.DeleteRelation(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeRelation)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeRelation,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

#### `internal/usecases/delete_relation/scenario_test.go` (создать)

```go
package delete_relation

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

func (f *fakeRepo) DeleteRelation(_ context.Context, id models.ID) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)

	return nil
}

func TestDeleteRelationCallsRepo(t *testing.T) {
	repo := &fakeRepo{}
	id := models.ID("RL-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	if err := New(repo).DeleteRelation(context.Background(), id); err != nil {
		t.Fatalf("DeleteRelation: %v", err)
	}

	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("deleted = %v", repo.deleted)
	}
}

func TestDeleteRelationInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	err := New(repo).DeleteRelation(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}

	if len(repo.deleted) != 0 {
		t.Fatalf("repo вызван при невалидном id")
	}
}

func TestDeleteRelationPropagatesInUseError(t *testing.T) {
	wantErr := &models.InUseError{Type: models.TypeRelation, ID: "RL-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	repo := &fakeRepo{err: wantErr}

	err := New(repo).DeleteRelation(context.Background(), "RL-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	var iu *models.InUseError
	if !errors.As(err, &iu) {
		t.Fatalf("err = %v, want InUseError", err)
	}
}
```

**Residence** (5 пакетов, зеркалирует Relation с заменой второй строгой ссылки на `place_id`):

#### `internal/usecases/list_residences/`

`ListResidences` — full-scan-and-filter по `models.ResidenceQuery`: `PersonID`/`PlaceID` пересекаются, если заданы вместе. Заменяет отсутствующий `search_residences`.

#### `internal/usecases/list_residences/deps.go` (создать)

```go
package list_residences

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ResidenceRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ResidenceRepo interface {
	ListResidences(ctx context.Context, access models.Access, page models.Page) ([]*models.Residence, error)
}
```

#### `internal/usecases/list_residences/scenario.go` (создать)

```go
package list_residences

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список проживаний».
type Scenario struct {
	residences ResidenceRepo
}

// New создаёт сценарий.
func New(residences ResidenceRepo) *Scenario {
	return &Scenario{residences: residences}
}

// ListResidences возвращает проживания, прошедшие фильтры запроса
// (q.PersonID/q.PlaceID, если заданы — пересекаются), в порядке сохранения;
// окно применяется после фильтра. Некорректный запрос —
// *models.ValidationError, репозиторий не вызывается. Короткий результат
// (меньше размера окна) означает конец списка.
//
// У Residence нет собственных поисковых полей (search-индекс намеренно пуст,
// docs/data-model/entity-write.md §3.8) — этот фильтр заменяет
// search_residences, которого в этой программе нет. Полное сканирование по
// generic-окнам ListResidences — тот же приём, что и list_relations/
// list_archive_nodes.
func (s *Scenario) ListResidences(ctx context.Context, access models.Access, q models.ResidenceQuery) ([]models.Residence, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	page := q.Page.Normalized()
	out := []models.Residence{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		list, err := s.residences.ListResidences(ctx, access, models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(list) == 0 {
			return out, nil
		}

		var full bool
		if full, matched, out = applyWindow(q, list, page, matched, out); full {
			return out, nil
		}
	}
}

// applyWindow прогоняет одно окно репозитория через фильтр запроса и
// накапливает результат окна запроса. full — окно запроса заполнено (обход
// можно остановить).
func applyWindow(q models.ResidenceQuery, list []*models.Residence,
	page models.Page, matched int, out []models.Residence,
) (full bool, nextMatched int, nextOut []models.Residence) {
	for _, r := range list {
		if q.PersonID != nil && *q.PersonID != r.PersonID {
			continue
		}

		if q.PlaceID != nil && *q.PlaceID != r.PlaceID {
			continue
		}

		if matched >= page.Offset {
			out = append(out, *r)

			if len(out) == page.Limit {
				return true, matched, out
			}
		}

		matched++
	}

	return false, matched, out
}
```

#### `internal/usecases/list_residences/scenario_test.go` (создать)

```go
package list_residences

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	list  []*models.Residence
	err   error
	calls []models.Page
}

func window(list []*models.Residence, page models.Page) []*models.Residence {
	page = page.Normalized()
	if page.Offset >= len(list) {
		return nil
	}

	return list[page.Offset:min(page.Offset+page.Limit, len(list))]
}

func (f *fakeRepo) ListResidences(_ context.Context, _ models.Access, page models.Page) ([]*models.Residence, error) {
	f.calls = append(f.calls, page)
	if f.err != nil {
		return nil, f.err
	}

	return window(f.list, page), nil
}

func pID(last byte) models.ID  { return models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }
func dID(last byte) models.ID  { return models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }
func rsID(last byte) models.ID { return models.ID("RS-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }

func ids(list []models.Residence) []models.ID {
	out := make([]models.ID, 0, len(list))
	for _, r := range list {
		out = append(out, r.ID)
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

func sample() *fakeRepo {
	return &fakeRepo{list: []*models.Residence{
		{ID: rsID('1'), PersonID: pID('1'), PlaceID: dID('1')},
		{ID: rsID('2'), PersonID: pID('1'), PlaceID: dID('2')},
		{ID: rsID('3'), PersonID: pID('2'), PlaceID: dID('1')},
	}}
}

func TestListResidencesNoFilterReturnsAll(t *testing.T) {
	got, err := New(sample()).ListResidences(context.Background(), models.AccessFull, models.ResidenceQuery{})
	if err != nil || !sameIDs(ids(got), rsID('1'), rsID('2'), rsID('3')) {
		t.Fatalf("got %v, %v", ids(got), err)
	}
}

func TestListResidencesFilterByPerson(t *testing.T) {
	person1 := pID('1')

	got, err := New(sample()).ListResidences(context.Background(), models.AccessFull, models.ResidenceQuery{PersonID: &person1})
	if err != nil || !sameIDs(ids(got), rsID('1'), rsID('2')) {
		t.Fatalf("got %v, %v; ожидались rs1, rs2 (person1)", ids(got), err)
	}
}

func TestListResidencesFilterByPlace(t *testing.T) {
	place1 := dID('1')

	got, err := New(sample()).ListResidences(context.Background(), models.AccessFull, models.ResidenceQuery{PlaceID: &place1})
	if err != nil || !sameIDs(ids(got), rsID('1'), rsID('3')) {
		t.Fatalf("got %v, %v; ожидались rs1, rs3 (place1)", ids(got), err)
	}
}

// TestListResidencesFilterIntersects: person_id и place_id вместе —
// пересечение обоих фильтров, не объединение.
func TestListResidencesFilterIntersects(t *testing.T) {
	person1, place1 := pID('1'), dID('1')

	got, err := New(sample()).ListResidences(context.Background(), models.AccessFull,
		models.ResidenceQuery{PersonID: &person1, PlaceID: &place1})
	if err != nil || !sameIDs(ids(got), rsID('1')) {
		t.Fatalf("got %v, %v; ожидался только rs1 (пересечение person1 и place1)", ids(got), err)
	}
}

func TestListResidencesInvalidQuery(t *testing.T) {
	bad := models.ID("not-an-id")
	repo := sample()

	for _, q := range []models.ResidenceQuery{
		{PersonID: &bad},
		{PlaceID: &bad},
		{Page: models.Page{Limit: -1}},
		{Page: models.Page{Offset: -1}},
	} {
		_, err := New(repo).ListResidences(context.Background(), models.AccessFull, q)

		var ve *models.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("запрос %+v: err = %v, ожидалась *ValidationError", q, err)
		}
	}

	if len(repo.calls) != 0 {
		t.Fatalf("репозиторий вызван %d раз при некорректном запросе", len(repo.calls))
	}
}

func TestListResidencesEmptyRepoGivesEmptyNotNil(t *testing.T) {
	got, err := New(&fakeRepo{}).ListResidences(context.Background(), models.AccessFull, models.ResidenceQuery{})
	if err != nil {
		t.Fatalf("ListResidences: %v", err)
	}

	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v, ожидался пустой не-nil срез", got)
	}
}

func TestListResidencesPropagatesRepoError(t *testing.T) {
	wantErr := errors.New("repo down")

	if _, err := New(&fakeRepo{err: wantErr}).ListResidences(context.Background(), models.AccessFull, models.ResidenceQuery{}); !errors.Is(err, wantErr) {
		t.Errorf("err=%v, want %v", err, wantErr)
	}
}
```

#### `internal/usecases/get_residence/`

`GetResidence` — access-aware чтение по id, тот же паттерн, что `get_relation`.

#### `internal/usecases/get_residence/deps.go` (создать)

```go
package get_residence

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ResidenceRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ResidenceRepo interface {
	GetResidence(ctx context.Context, id models.ID) (*models.Residence, error)
}
```

#### `internal/usecases/get_residence/scenario.go` (создать)

```go
package get_residence

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «проживание по идентификатору».
type Scenario struct {
	residences ResidenceRepo
}

// New создаёт сценарий.
func New(residences ResidenceRepo) *Scenario {
	return &Scenario{residences: residences}
}

// GetResidence возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound. Приватная запись
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound.
func (s *Scenario) GetResidence(ctx context.Context, access models.Access, id models.ID) (models.Residence, error) {
	if err := validateID(id); err != nil {
		return models.Residence{}, err
	}

	r, err := s.residences.GetResidence(ctx, id)
	if err != nil {
		return models.Residence{}, err
	}

	if r.Private && access != models.AccessFull {
		return models.Residence{}, models.ErrNotFound
	}

	return *r, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeResidence)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeResidence,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

#### `internal/usecases/get_residence/scenario_test.go` (создать)

```go
package get_residence

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	residences map[models.ID]*models.Residence
}

func (f *fakeRepo) GetResidence(_ context.Context, id models.ID) (*models.Residence, error) {
	r, ok := f.residences[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return r, nil
}

func TestGetResidenceReturnsRecord(t *testing.T) {
	id := models.ID("RS-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{residences: map[models.ID]*models.Residence{id: {ID: id, Note: "изба"}}}

	got, err := New(repo).GetResidence(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetResidence: %v", err)
	}

	if got.Note != "изба" {
		t.Fatalf("Note = %q", got.Note)
	}
}

func TestGetResidenceNotFound(t *testing.T) {
	repo := &fakeRepo{residences: map[models.ID]*models.Residence{}}

	_, err := New(repo).GetResidence(context.Background(), models.AccessFull, "RS-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetResidenceInvalidID(t *testing.T) {
	repo := &fakeRepo{residences: map[models.ID]*models.Residence{}}

	_, err := New(repo).GetResidence(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestGetResidencePrivateHiddenFromPublic(t *testing.T) {
	id := models.ID("RS-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{residences: map[models.ID]*models.Residence{id: {ID: id, Private: true}}}

	_, err := New(repo).GetResidence(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for private record with AccessPublic", err)
	}
}

func TestGetResidencePrivateVisibleToFullAccess(t *testing.T) {
	id := models.ID("RS-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{residences: map[models.ID]*models.Residence{id: {ID: id, Private: true}}}

	got, err := New(repo).GetResidence(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetResidence: %v", err)
	}

	if !got.Private {
		t.Fatalf("Private = %v", got.Private)
	}
}
```

#### `internal/usecases/create_residence/`

`CreateResidence` — генерирует id, `Residence.Validate()`, в транзакции: `tx.GetPerson(PersonID)` (422 `person_id`), `tx.GetAdministrativeDivision(PlaceID)` (422 `place_id` — СТРОГО этот тип, не общий `PlaceRef`), цикл `Sources[i].CitationID`, `tx.SaveResidence`. `store.Store`'s generic `SaveResidence` уже подпирает те же FK реальными SQL-constraint'ами — проверки здесь дают чистую field-specific 422 вместо сырой ошибки БД.

#### `internal/usecases/create_residence/deps.go` (создать)

```go
package create_residence

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// ResidenceStore — зависимость сценария: транзакция порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ResidenceStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
```

#### `internal/usecases/create_residence/scenario.go` (создать)

```go
package create_residence

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание проживания».
type Scenario struct {
	store ResidenceStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st ResidenceStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateResidence создаёт запись: генерирует идентификатор, проверяет
// инварианты, в одной транзакции убеждается в существовании персоны (person_id),
// места (place_id — строго AdministrativeDivision, не любой PlaceRef) и цитат
// из Sources, сохраняет. Возвращает созданную запись с заполненным ID.
//
// store.Store's generic SaveResidence уже подпирает эти же FK реальными SQL-
// constraint'ами — эти проверки здесь дают чистую field-specific 422 вместо
// сырой ошибки БД, по единому для программы правилу (docs/data-model/
// entity-write.md §3.8).
//
// Ошибки: непустой входной ID, невалидная сущность, несуществующая персона/
// место (поля person_id/place_id) и несуществующая цитата —
// *models.ValidationError; прочее — ошибки хранилища как есть.
func (s *Scenario) CreateResidence(ctx context.Context, r models.Residence) (models.Residence, error) {
	if r.ID != "" {
		return models.Residence{}, &models.ValidationError{
			Entity: models.TypeResidence,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", r.ID),
		}
	}

	r.ID = s.ids.New(models.TypeResidence)

	if err := r.Validate(); err != nil {
		return models.Residence{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetPerson(ctx, r.PersonID); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return fieldErr("person_id", "персона %q не найдена", r.PersonID)
			}

			return err
		}

		if _, err := tx.GetAdministrativeDivision(ctx, r.PlaceID); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return fieldErr("place_id", "место %q не найдено", r.PlaceID)
			}

			return err
		}

		for i, link := range r.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveResidence(ctx, &r)
	})
	if err != nil {
		return models.Residence{}, err
	}

	return r, nil
}

// fieldErr — *models.ValidationError по указанному полю.
func fieldErr(field, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeResidence,
		Field:  field,
		Reason: fmt.Sprintf(format, args...),
	}
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeResidence,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/create_residence/scenario_test.go` (создать)

```go
package create_residence

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

func pID(last byte) models.ID { return models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }
func dID(last byte) models.ID { return models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }
func cID(last byte) models.ID { return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }

type stubIDs struct{ id models.ID }

func (s *stubIDs) New(models.Type) models.ID { return s.id }

// fakeTx реализует нужные сценарию методы store.Store поверх карт персон,
// мест и цитат; остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	people    map[models.ID]*models.Person
	places    map[models.ID]*models.AdministrativeDivision
	citations map[models.ID]*models.Citation
	saved     *models.Residence
	saveErr   error
}

func newFakeTx(people, places []models.ID, citations ...*models.Citation) *fakeTx {
	tx := &fakeTx{
		people: map[models.ID]*models.Person{}, places: map[models.ID]*models.AdministrativeDivision{},
		citations: map[models.ID]*models.Citation{},
	}
	for _, id := range people {
		tx.people[id] = &models.Person{ID: id}
	}
	for _, id := range places {
		tx.places[id] = &models.AdministrativeDivision{ID: id}
	}
	for _, c := range citations {
		tx.citations[c.ID] = c
	}

	return tx
}

func (f *fakeTx) GetPerson(_ context.Context, id models.ID) (*models.Person, error) {
	p, ok := f.people[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *p

	return &cp, nil
}

func (f *fakeTx) GetAdministrativeDivision(_ context.Context, id models.ID) (*models.AdministrativeDivision, error) {
	d, ok := f.places[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *d

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

func (f *fakeTx) SaveResidence(_ context.Context, r *models.Residence) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *r
	f.saved = &cp

	return nil
}

type fakeStore struct{ tx *fakeTx }

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	return fn(f.tx)
}

func TestCreateResidenceGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "RS-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{tx: newFakeTx([]models.ID{pID('1')}, []models.ID{dID('1')})}
	sc := New(st, ids)

	got, err := sc.CreateResidence(context.Background(), models.Residence{PersonID: pID('1'), PlaceID: dID('1')})
	if err != nil {
		t.Fatalf("CreateResidence: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.tx.saved == nil || st.tx.saved.PersonID != pID('1') || st.tx.saved.PlaceID != dID('1') {
		t.Fatalf("saved = %+v", st.tx.saved)
	}
}

func TestCreateResidenceRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx(nil, nil)}, &stubIDs{})

	_, err := sc.CreateResidence(context.Background(), models.Residence{
		ID: "RS-01ARZ3NDEKTSV4RRFFQ69G5FA1", PersonID: pID('1'), PlaceID: dID('1'),
	})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

// TestCreateResidencePersonNotFound: несуществующий person_id — 422 на поле
// person_id, ничего не сохраняется.
func TestCreateResidencePersonNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx(nil, []models.ID{dID('1')})}
	sc := New(st, &stubIDs{id: "RS-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateResidence(context.Background(), models.Residence{PersonID: pID('1'), PlaceID: dID('1')})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "person_id" {
		t.Fatalf("err = %v, want ValidationError on person_id", err)
	}

	if st.tx.saved != nil {
		t.Fatalf("saved = %+v; ожидалось: ничего не сохранено", st.tx.saved)
	}
}

// TestCreateResidencePlaceNotFound: person_id существует, но place_id — нет
// — 422 на поле place_id.
func TestCreateResidencePlaceNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx([]models.ID{pID('1')}, nil)}
	sc := New(st, &stubIDs{id: "RS-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateResidence(context.Background(), models.Residence{PersonID: pID('1'), PlaceID: dID('1')})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "place_id" {
		t.Fatalf("err = %v, want ValidationError on place_id", err)
	}
}

func TestCreateResidenceSourceCitationNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx([]models.ID{pID('1')}, []models.ID{dID('1')})}
	sc := New(st, &stubIDs{id: "RS-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	in := models.Residence{PersonID: pID('1'), PlaceID: dID('1'), Sources: []models.SourceLink{{CitationID: cID('0')}}}

	_, err := sc.CreateResidence(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, want ValidationError on sources[0].citation_id", err)
	}
}
```

#### `internal/usecases/update_residence/`

`UpdateResidence` — полная замена по `r.ID`: `Residence.Validate()`, в транзакции `tx.GetResidence(r.ID)`, те же проверки `person_id`/`place_id`/`sources[i].citation_id`, что и `create_residence`, `tx.SaveResidence`.

#### `internal/usecases/update_residence/deps.go` (создать)

```go
package update_residence

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// ResidenceStore — зависимость сценария: транзакция порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ResidenceStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
```

#### `internal/usecases/update_residence/scenario.go` (создать)

```go
package update_residence

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение проживания».
type Scenario struct {
	store ResidenceStore
}

// New создаёт сценарий.
func New(st ResidenceStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateResidence полностью заменяет запись по r.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, персона и место
// существуют, и цитаты из Sources существуют, и сохраняет.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; несуществующая персона/место/цитата —
// *models.ValidationError; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateResidence(ctx context.Context, r models.Residence) error {
	if err := r.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetResidence(ctx, r.ID); err != nil {
			return err
		}

		if _, err := tx.GetPerson(ctx, r.PersonID); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return fieldErr("person_id", "персона %q не найдена", r.PersonID)
			}

			return err
		}

		if _, err := tx.GetAdministrativeDivision(ctx, r.PlaceID); err != nil {
			if errors.Is(err, models.ErrNotFound) {
				return fieldErr("place_id", "место %q не найдено", r.PlaceID)
			}

			return err
		}

		for i, link := range r.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveResidence(ctx, &r)
	})
}

// fieldErr — *models.ValidationError по указанному полю.
func fieldErr(field, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeResidence,
		Field:  field,
		Reason: fmt.Sprintf(format, args...),
	}
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeResidence,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/update_residence/scenario_test.go` (создать)

```go
package update_residence

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

func pID(last byte) models.ID { return models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }
func dID(last byte) models.ID { return models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }
func cID(last byte) models.ID { return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }

type fakeTx struct {
	store.Store
	residences map[models.ID]*models.Residence
	people     map[models.ID]*models.Person
	places     map[models.ID]*models.AdministrativeDivision
	citations  map[models.ID]*models.Citation
	saved      []*models.Residence
}

func newFakeTx(existing *models.Residence, people, places []models.ID) *fakeTx {
	tx := &fakeTx{
		residences: map[models.ID]*models.Residence{}, people: map[models.ID]*models.Person{},
		places: map[models.ID]*models.AdministrativeDivision{}, citations: map[models.ID]*models.Citation{},
	}
	if existing != nil {
		tx.residences[existing.ID] = existing
	}
	for _, id := range people {
		tx.people[id] = &models.Person{ID: id}
	}
	for _, id := range places {
		tx.places[id] = &models.AdministrativeDivision{ID: id}
	}

	return tx
}

func (f *fakeTx) GetResidence(_ context.Context, id models.ID) (*models.Residence, error) {
	r, ok := f.residences[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *r

	return &cp, nil
}

func (f *fakeTx) GetPerson(_ context.Context, id models.ID) (*models.Person, error) {
	p, ok := f.people[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *p

	return &cp, nil
}

func (f *fakeTx) GetAdministrativeDivision(_ context.Context, id models.ID) (*models.AdministrativeDivision, error) {
	d, ok := f.places[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *d

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

func (f *fakeTx) SaveResidence(_ context.Context, r *models.Residence) error {
	cp := *r
	f.saved = append(f.saved, &cp)

	return nil
}

type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func residence(id, person, place models.ID) *models.Residence {
	return &models.Residence{ID: id, PersonID: person, PlaceID: place}
}

func TestUpdateResidenceSaves(t *testing.T) {
	existing := residence("RS-01ARZ3NDEKTSV4RRFFQ69G5FA1", pID('1'), dID('1'))
	st := &fakeStore{tx: newFakeTx(existing, []models.ID{pID('1')}, []models.ID{dID('1')})}

	updated := *existing
	updated.Note = "переезд"

	if err := New(st).UpdateResidence(context.Background(), updated); err != nil {
		t.Fatalf("UpdateResidence: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Note != "переезд" {
		t.Fatalf("calls=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateResidenceNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx(nil, []models.ID{pID('1')}, []models.ID{dID('1')})}

	err := New(st).UpdateResidence(context.Background(), *residence("RS-01ARZ3NDEKTSV4RRFFQ69G5FA1", pID('1'), dID('1')))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdateResidencePersonNotFound(t *testing.T) {
	existing := residence("RS-01ARZ3NDEKTSV4RRFFQ69G5FA1", pID('1'), dID('1'))
	st := &fakeStore{tx: newFakeTx(existing, nil, []models.ID{dID('1')})}

	err := New(st).UpdateResidence(context.Background(), *existing)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "person_id" {
		t.Fatalf("err = %v, want ValidationError on person_id", err)
	}
}

func TestUpdateResidencePlaceNotFound(t *testing.T) {
	existing := residence("RS-01ARZ3NDEKTSV4RRFFQ69G5FA1", pID('1'), dID('1'))
	st := &fakeStore{tx: newFakeTx(existing, []models.ID{pID('1')}, nil)}

	err := New(st).UpdateResidence(context.Background(), *existing)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "place_id" {
		t.Fatalf("err = %v, want ValidationError on place_id", err)
	}
}

func TestUpdateResidenceSourceCitationNotFound(t *testing.T) {
	existing := residence("RS-01ARZ3NDEKTSV4RRFFQ69G5FA1", pID('1'), dID('1'))
	st := &fakeStore{tx: newFakeTx(existing, []models.ID{pID('1')}, []models.ID{dID('1')})}

	updated := *existing
	updated.Sources = []models.SourceLink{{CitationID: cID('0')}}

	err := New(st).UpdateResidence(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, want ValidationError on sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d записей при несуществующей цитате", len(st.tx.saved))
	}
}
```

#### `internal/usecases/delete_residence/`

`DeleteResidence` — валидация формата id, `residences.DeleteResidence`.

#### `internal/usecases/delete_residence/deps.go` (создать)

```go
package delete_residence

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ResidenceRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ResidenceRepo interface {
	DeleteResidence(ctx context.Context, id models.ID) error
}
```

#### `internal/usecases/delete_residence/scenario.go` (создать)

```go
package delete_residence

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление проживания».
type Scenario struct {
	residences ResidenceRepo
}

// New создаёт сценарий.
func New(residences ResidenceRepo) *Scenario {
	return &Scenario{residences: residences}
}

// DeleteResidence удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteResidence(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.residences.DeleteResidence(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeResidence)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeResidence,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

#### `internal/usecases/delete_residence/scenario_test.go` (создать)

```go
package delete_residence

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

func (f *fakeRepo) DeleteResidence(_ context.Context, id models.ID) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)

	return nil
}

func TestDeleteResidenceCallsRepo(t *testing.T) {
	repo := &fakeRepo{}
	id := models.ID("RS-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	if err := New(repo).DeleteResidence(context.Background(), id); err != nil {
		t.Fatalf("DeleteResidence: %v", err)
	}

	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("deleted = %v", repo.deleted)
	}
}

func TestDeleteResidenceInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	err := New(repo).DeleteResidence(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}

	if len(repo.deleted) != 0 {
		t.Fatalf("repo вызван при невалидном id")
	}
}

func TestDeleteResidencePropagatesInUseError(t *testing.T) {
	wantErr := &models.InUseError{Type: models.TypeResidence, ID: "RS-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	repo := &fakeRepo{err: wantErr}

	err := New(repo).DeleteResidence(context.Background(), "RS-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	var iu *models.InUseError
	if !errors.As(err, &iu) {
		t.Fatalf("err = %v, want InUseError", err)
	}
}
```

**Event** (6 пакетов — единственная из трёх сущностей с поиском):

#### `internal/usecases/list_events/`

`ListEvents` — full-scan-and-filter по `models.EventQuery`: фильтр «персона участвует» (совпадает с любым `Participants[i].PersonID`) — дополнительный, НЕ заменяющий поиск фильтр (в отличие от `RelationQuery`/`ResidenceQuery` — `Event` ЕСТЬ поиск, см. `search_events` ниже).

#### `internal/usecases/list_events/deps.go` (создать)

```go
package list_events

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// EventRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type EventRepo interface {
	ListEvents(ctx context.Context, access models.Access, page models.Page) ([]*models.Event, error)
}
```

#### `internal/usecases/list_events/scenario.go` (создать)

```go
package list_events

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список событий».
type Scenario struct {
	events EventRepo
}

// New создаёт сценарий.
func New(events EventRepo) *Scenario {
	return &Scenario{events: events}
}

// ListEvents возвращает события, прошедшие фильтр запроса (q.PersonID, если
// задан — событие проходит, если персона участвует в нём хотя бы одной
// записью Participants), в порядке сохранения; окно применяется после
// фильтра. Некорректный запрос — *models.ValidationError, репозиторий не
// вызывается. Короткий результат (меньше размера окна) означает конец
// списка.
//
// В отличие от Relation/Residence, у Event ЕСТЬ поисковый индекс
// (search_events, по началу текста места) — этот фильтр по участнику
// дополняет поиск, а не заменяет его. Полное сканирование по generic-окнам
// ListEvents — тот же приём, что и list_relations/list_residences.
func (s *Scenario) ListEvents(ctx context.Context, access models.Access, q models.EventQuery) ([]models.Event, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	page := q.Page.Normalized()
	out := []models.Event{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		list, err := s.events.ListEvents(ctx, access, models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(list) == 0 {
			return out, nil
		}

		var full bool
		if full, matched, out = applyWindow(q, list, page, matched, out); full {
			return out, nil
		}
	}
}

// applyWindow прогоняет одно окно репозитория через фильтр запроса и
// накапливает результат окна запроса. full — окно запроса заполнено (обход
// можно остановить).
func applyWindow(q models.EventQuery, list []*models.Event,
	page models.Page, matched int, out []models.Event,
) (full bool, nextMatched int, nextOut []models.Event) {
	for _, e := range list {
		if q.PersonID != nil && !hasParticipant(e.Participants, *q.PersonID) {
			continue
		}

		if matched >= page.Offset {
			out = append(out, *e)

			if len(out) == page.Limit {
				return true, matched, out
			}
		}

		matched++
	}

	return false, matched, out
}

// hasParticipant сообщает, участвует ли персона id в событии.
func hasParticipant(ps []models.EventParticipant, id models.ID) bool {
	for _, p := range ps {
		if p.PersonID == id {
			return true
		}
	}

	return false
}
```

#### `internal/usecases/list_events/scenario_test.go` (создать)

```go
package list_events

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	list  []*models.Event
	err   error
	calls []models.Page
}

func window(list []*models.Event, page models.Page) []*models.Event {
	page = page.Normalized()
	if page.Offset >= len(list) {
		return nil
	}

	return list[page.Offset:min(page.Offset+page.Limit, len(list))]
}

func (f *fakeRepo) ListEvents(_ context.Context, _ models.Access, page models.Page) ([]*models.Event, error) {
	f.calls = append(f.calls, page)
	if f.err != nil {
		return nil, f.err
	}

	return window(f.list, page), nil
}

func pID(last byte) models.ID { return models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }
func eID(last byte) models.ID { return models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }

func ids(list []models.Event) []models.ID {
	out := make([]models.ID, 0, len(list))
	for _, e := range list {
		out = append(out, e.ID)
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

func sample() *fakeRepo {
	return &fakeRepo{list: []*models.Event{
		{ID: eID('1'), Participants: []models.EventParticipant{{PersonID: pID('1'), Role: "младенец"}}},
		{ID: eID('2'), Participants: []models.EventParticipant{{PersonID: pID('2'), Role: "младенец"}}},
		{ID: eID('3'), Participants: []models.EventParticipant{
			{PersonID: pID('3'), Role: "жених"}, {PersonID: pID('1'), Role: "свидетель"},
		}},
	}}
}

func TestListEventsNoFilterReturnsAll(t *testing.T) {
	got, err := New(sample()).ListEvents(context.Background(), models.AccessFull, models.EventQuery{})
	if err != nil || !sameIDs(ids(got), eID('1'), eID('2'), eID('3')) {
		t.Fatalf("got %v, %v", ids(got), err)
	}
}

// TestListEventsFilterByParticipant: person_id совпадает с ЛЮБЫМ из
// Participants[i].PersonID, не только с первым.
func TestListEventsFilterByParticipant(t *testing.T) {
	person1 := pID('1')

	got, err := New(sample()).ListEvents(context.Background(), models.AccessFull, models.EventQuery{PersonID: &person1})
	if err != nil || !sameIDs(ids(got), eID('1'), eID('3')) {
		t.Fatalf("got %v, %v; ожидались e1 (первый участник) и e3 (второй участник)", ids(got), err)
	}
}

func TestListEventsFilterNoMatch(t *testing.T) {
	other := pID('9')

	got, err := New(sample()).ListEvents(context.Background(), models.AccessFull, models.EventQuery{PersonID: &other})
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v; ожидался пустой результат", ids(got), err)
	}
}

func TestListEventsInvalidQuery(t *testing.T) {
	bad := models.ID("not-an-id")
	repo := sample()

	for _, q := range []models.EventQuery{
		{PersonID: &bad},
		{Page: models.Page{Limit: -1}},
		{Page: models.Page{Offset: -1}},
	} {
		_, err := New(repo).ListEvents(context.Background(), models.AccessFull, q)

		var ve *models.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("запрос %+v: err = %v, ожидалась *ValidationError", q, err)
		}
	}

	if len(repo.calls) != 0 {
		t.Fatalf("репозиторий вызван %d раз при некорректном запросе", len(repo.calls))
	}
}

func TestListEventsEmptyRepoGivesEmptyNotNil(t *testing.T) {
	got, err := New(&fakeRepo{}).ListEvents(context.Background(), models.AccessFull, models.EventQuery{})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}

	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v, ожидался пустой не-nil срез", got)
	}
}

func TestListEventsPropagatesRepoError(t *testing.T) {
	wantErr := errors.New("repo down")

	if _, err := New(&fakeRepo{err: wantErr}).ListEvents(context.Background(), models.AccessFull, models.EventQuery{}); !errors.Is(err, wantErr) {
		t.Errorf("err=%v, want %v", err, wantErr)
	}
}
```

#### `internal/usecases/search_events/`

`SearchEvents` — обычный `search_<entity>` по образцу `search_families`: ищет по началу текста `Place` — ЕДИНСТВЕННОЕ индексируемое поле события (`SaveEvent` вызывает `replaceSearchIndex(tx, "events", e.ID, map[string][]string{"place": {place}})`, `type`/`date`/`participants` поиском не охвачены).

#### `internal/usecases/search_events/deps.go` (создать)

```go
package search_events

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// EventRepo — зависимость сценария: поиск по общему индексу + чтение по id.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type EventRepo interface {
	Search(ctx context.Context, text string, access models.Access, page models.Page) ([]models.Hit, error)
	GetEvent(ctx context.Context, id models.ID) (*models.Event, error)
}
```

#### `internal/usecases/search_events/scenario.go` (создать)

```go
package search_events

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск событий».
type Scenario struct {
	events EventRepo
}

// New создаёт сценарий.
func New(events EventRepo) *Scenario {
	return &Scenario{events: events}
}

// SearchEvents находит события по началу текста места (place) — единственное
// индексируемое поле события (internal/store/sqlstore/records.go:SaveEvent,
// replaceSearchIndex(tx, "events", e.ID, map[string][]string{"place": {place}})):
// type/date/участники поиском не охвачены. Та же механика, что и
// search_divisions.SearchDivisions (см. её комментарий).
func (s *Scenario) SearchEvents(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Event, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Event{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Event{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.events.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeEvent {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.events.GetEvent(ctx, h.ID)
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

#### `internal/usecases/search_events/scenario_test.go` (создать)

```go
package search_events

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits   []models.Hit
	events map[models.ID]*models.Event
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetEvent(_ context.Context, id models.ID) (*models.Event, error) {
	e, ok := f.events[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return e, nil
}

func TestSearchEventsFiltersByType(t *testing.T) {
	id := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("F-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeEvent, ID: id},
			{Type: models.TypeFamily, ID: otherID}, // не event — должен быть пропущен
		},
		events: map[models.ID]*models.Event{
			id: {ID: id, Type: models.EventTypeBirth, Place: &models.PlaceRef{Text: "Давыдово"}},
		},
	}

	got, err := New(repo).SearchEvents(context.Background(), models.AccessFull, models.SearchQuery{Text: "Дав"})
	if err != nil {
		t.Fatalf("SearchEvents: %v", err)
	}

	if len(got) != 1 || got[0].Place == nil || got[0].Place.Text != "Давыдово" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchEventsEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchEvents(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchEvents: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
```

#### `internal/usecases/get_event/`

`GetEvent` — access-aware чтение по id, тот же паттерн, что `get_relation`/`get_residence`.

#### `internal/usecases/get_event/deps.go` (создать)

```go
package get_event

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// EventRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type EventRepo interface {
	GetEvent(ctx context.Context, id models.ID) (*models.Event, error)
}
```

#### `internal/usecases/get_event/scenario.go` (создать)

```go
package get_event

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «событие по идентификатору».
type Scenario struct {
	events EventRepo
}

// New создаёт сценарий.
func New(events EventRepo) *Scenario {
	return &Scenario{events: events}
}

// GetEvent возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound. Приватная запись
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound.
func (s *Scenario) GetEvent(ctx context.Context, access models.Access, id models.ID) (models.Event, error) {
	if err := validateID(id); err != nil {
		return models.Event{}, err
	}

	e, err := s.events.GetEvent(ctx, id)
	if err != nil {
		return models.Event{}, err
	}

	if e.Private && access != models.AccessFull {
		return models.Event{}, models.ErrNotFound
	}

	return *e, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeEvent)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeEvent,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

#### `internal/usecases/get_event/scenario_test.go` (создать)

```go
package get_event

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	events map[models.ID]*models.Event
}

func (f *fakeRepo) GetEvent(_ context.Context, id models.ID) (*models.Event, error) {
	e, ok := f.events[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return e, nil
}

func TestGetEventReturnsRecord(t *testing.T) {
	id := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{events: map[models.ID]*models.Event{id: {ID: id, Type: models.EventTypeBirth}}}

	got, err := New(repo).GetEvent(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}

	if got.Type != models.EventTypeBirth {
		t.Fatalf("Type = %q", got.Type)
	}
}

func TestGetEventNotFound(t *testing.T) {
	repo := &fakeRepo{events: map[models.ID]*models.Event{}}

	_, err := New(repo).GetEvent(context.Background(), models.AccessFull, "E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetEventInvalidID(t *testing.T) {
	repo := &fakeRepo{events: map[models.ID]*models.Event{}}

	_, err := New(repo).GetEvent(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestGetEventPrivateHiddenFromPublic(t *testing.T) {
	id := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{events: map[models.ID]*models.Event{id: {ID: id, Private: true}}}

	_, err := New(repo).GetEvent(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for private record with AccessPublic", err)
	}
}

func TestGetEventPrivateVisibleToFullAccess(t *testing.T) {
	id := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{events: map[models.ID]*models.Event{id: {ID: id, Private: true}}}

	got, err := New(repo).GetEvent(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}

	if !got.Private {
		t.Fatalf("Private = %v", got.Private)
	}
}
```

#### `internal/usecases/create_event/`

`CreateEvent` — генерирует id, `Event.Validate()` (формат/ограничение типа `Place`, если задан), в транзакции: цикл `Participants[i].PersonID` через `tx.GetPerson` (422 `participants[i].person_id`, ПЕРВАЯ строгая ссылка внутри элемента array-of-objects аргумента в программе), НЕЗАВИСИМЫЙ цикл `Sources[i].CitationID`, `tx.SaveEvent`. `Place` НИКОГДА не проверяется на существование — мягкая ссылка, тот же принцип, что у любого `TextRef`-подобного поля.

#### `internal/usecases/create_event/deps.go` (создать)

```go
package create_event

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// EventStore — зависимость сценария: транзакция порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type EventStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
```

#### `internal/usecases/create_event/scenario.go` (создать)

```go
package create_event

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание события».
type Scenario struct {
	store EventStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st EventStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateEvent создаёт запись: генерирует идентификатор, проверяет
// инварианты (Event.Validate), в одной транзакции убеждается в существовании
// каждого участника (Participants[i].PersonID — СТРОГАЯ ссылка, в отличие от
// Place) и цитат из Sources, сохраняет. Возвращает созданную запись с
// заполненным ID.
//
// Place (*models.PlaceRef) НИКОГДА не проверяется на существование — мягкая
// ссылка, тот же принцип, что и у любого другого TextRef-подобного поля в
// программе (docs/data-model/entity-write.md §3.8); только форма/ограничение
// типа проверяется в Event.Validate (вызван до InTx).
//
// Ошибки: непустой входной ID, невалидная сущность, несуществующий участник
// (поле participants[i].person_id) и несуществующая цитата —
// *models.ValidationError; прочее — ошибки хранилища как есть.
func (s *Scenario) CreateEvent(ctx context.Context, e models.Event) (models.Event, error) {
	if e.ID != "" {
		return models.Event{}, &models.ValidationError{
			Entity: models.TypeEvent,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", e.ID),
		}
	}

	e.ID = s.ids.New(models.TypeEvent)

	if err := e.Validate(); err != nil {
		return models.Event{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		for i, p := range e.Participants {
			if _, err := tx.GetPerson(ctx, p.PersonID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return participantErr(i, "персона %q не найдена", p.PersonID)
				}

				return err
			}
		}

		for i, link := range e.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveEvent(ctx, &e)
	})
	if err != nil {
		return models.Event{}, err
	}

	return e, nil
}

// participantErr — *models.ValidationError по полю participants[i].person_id.
func participantErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeEvent,
		Field:  fmt.Sprintf("participants[%d].person_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeEvent,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/create_event/scenario_test.go` (создать)

```go
package create_event

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

func pID(last byte) models.ID { return models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }
func cID(last byte) models.ID { return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }

type stubIDs struct{ id models.ID }

func (s *stubIDs) New(models.Type) models.ID { return s.id }

// fakeTx реализует нужные сценарию методы store.Store поверх карт персон и
// цитат; остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	people    map[models.ID]*models.Person
	citations map[models.ID]*models.Citation
	saved     *models.Event
	saveErr   error
}

func newFakeTx(people []models.ID, citations ...*models.Citation) *fakeTx {
	tx := &fakeTx{people: map[models.ID]*models.Person{}, citations: map[models.ID]*models.Citation{}}
	for _, id := range people {
		tx.people[id] = &models.Person{ID: id}
	}
	for _, c := range citations {
		tx.citations[c.ID] = c
	}

	return tx
}

func (f *fakeTx) GetPerson(_ context.Context, id models.ID) (*models.Person, error) {
	p, ok := f.people[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *p

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

func (f *fakeTx) SaveEvent(_ context.Context, e *models.Event) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *e
	f.saved = &cp

	return nil
}

type fakeStore struct{ tx *fakeTx }

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	return fn(f.tx)
}

func TestCreateEventGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "E-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{tx: newFakeTx(nil)}
	sc := New(st, ids)

	got, err := sc.CreateEvent(context.Background(), models.Event{Type: models.EventTypeBirth})
	if err != nil {
		t.Fatalf("CreateEvent: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.tx.saved == nil || st.tx.saved.Type != models.EventTypeBirth {
		t.Fatalf("saved = %+v", st.tx.saved)
	}
}

func TestCreateEventRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx(nil)}, &stubIDs{})

	_, err := sc.CreateEvent(context.Background(), models.Event{ID: "E-01ARZ3NDEKTSV4RRFFQ69G5FA1", Type: models.EventTypeBirth})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

// TestCreateEventPlaceSoftRefNotChecked: Place — мягкая ссылка (PlaceRef),
// НИКОГДА не проверяется на существование при сохранении — даже
// фабрикованный, заведомо несуществующий id проходит, поскольку модельная
// Validate проверяет только формат/тип-ограничение (в отличие от
// participants[i].person_id — строгой ссылки, см.
// TestCreateEventParticipantNotFound).
func TestCreateEventPlaceSoftRefNotChecked(t *testing.T) {
	ids := &stubIDs{id: "E-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{tx: newFakeTx(nil)} // ни одной персоны в хранилище
	sc := New(st, ids)

	in := models.Event{
		Type: models.EventTypeBirth,
		Place: &models.PlaceRef{
			Text: "село Давыдово", Ref: "AD-01ARZ3NDEKTSV4RRFFQ69G5FA9", Type: models.TypeAdministrativeDivision,
		},
	}

	got, err := sc.CreateEvent(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateEvent: %v, ожидался успех (Place — мягкая ссылка)", err)
	}

	if got.Place == nil || got.Place.Ref != "AD-01ARZ3NDEKTSV4RRFFQ69G5FA9" {
		t.Fatalf("Place = %+v", got.Place)
	}
}

// TestCreateEventParticipantNotFound: несуществующий
// participants[i].person_id — 422 на поле participants[i].person_id,
// ничего не сохраняется (в отличие от Place — строгая ссылка).
func TestCreateEventParticipantNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx([]models.ID{pID('1')})} // только первая персона (индекс 0) существует
	sc := New(st, &stubIDs{id: "E-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	in := models.Event{
		Type: models.EventTypeBirth,
		Participants: []models.EventParticipant{
			{PersonID: pID('1'), Role: "родитель"},
			{PersonID: pID('9'), Role: "свидетель"}, // не существует
		},
	}

	_, err := sc.CreateEvent(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "participants[1].person_id" {
		t.Fatalf("err = %v, want ValidationError on participants[1].person_id (индексированная ошибка на позиции несуществующего участника)", err)
	}

	if st.tx.saved != nil {
		t.Fatalf("saved = %+v; ожидалось: ничего не сохранено", st.tx.saved)
	}
}

func TestCreateEventSourceCitationNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx(nil)}
	sc := New(st, &stubIDs{id: "E-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	in := models.Event{Type: models.EventTypeBirth, Sources: []models.SourceLink{{CitationID: cID('0')}}}

	_, err := sc.CreateEvent(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, want ValidationError on sources[0].citation_id", err)
	}
}

func TestCreateEventRejectsInvalidType(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx(nil)}, &stubIDs{id: "E-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateEvent(context.Background(), models.Event{Type: "Not Valid!"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "type" {
		t.Fatalf("err = %v, want ValidationError on type", err)
	}
}
```

#### `internal/usecases/update_event/`

`UpdateEvent` — полная замена по `e.ID`: `Event.Validate()`, в транзакции `tx.GetEvent(e.ID)`, те же проверки `participants[i].person_id`/`sources[i].citation_id`, что и `create_event`, `tx.SaveEvent`.

#### `internal/usecases/update_event/deps.go` (создать)

```go
package update_event

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// EventStore — зависимость сценария: транзакция порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type EventStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
```

#### `internal/usecases/update_event/scenario.go` (создать)

```go
package update_event

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение события».
type Scenario struct {
	store EventStore
}

// New создаёт сценарий.
func New(st EventStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateEvent полностью заменяет запись по e.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, каждый участник
// (Participants[i].PersonID) и цитаты из Sources существуют, и сохраняет.
// Place (мягкая ссылка) не проверяется на существование — см. CreateEvent.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; несуществующий участник/цитата —
// *models.ValidationError; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateEvent(ctx context.Context, e models.Event) error {
	if err := e.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetEvent(ctx, e.ID); err != nil {
			return err
		}

		for i, p := range e.Participants {
			if _, err := tx.GetPerson(ctx, p.PersonID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return participantErr(i, "персона %q не найдена", p.PersonID)
				}

				return err
			}
		}

		for i, link := range e.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SaveEvent(ctx, &e)
	})
}

// participantErr — *models.ValidationError по полю participants[i].person_id.
func participantErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeEvent,
		Field:  fmt.Sprintf("participants[%d].person_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeEvent,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/update_event/scenario_test.go` (создать)

```go
package update_event

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

func pID(last byte) models.ID { return models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }
func cID(last byte) models.ID { return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }

type fakeTx struct {
	store.Store
	events    map[models.ID]*models.Event
	people    map[models.ID]*models.Person
	citations map[models.ID]*models.Citation
	saved     []*models.Event
}

func newFakeTx(existing *models.Event, people []models.ID) *fakeTx {
	tx := &fakeTx{events: map[models.ID]*models.Event{}, people: map[models.ID]*models.Person{}, citations: map[models.ID]*models.Citation{}}
	if existing != nil {
		tx.events[existing.ID] = existing
	}
	for _, id := range people {
		tx.people[id] = &models.Person{ID: id}
	}

	return tx
}

func (f *fakeTx) GetEvent(_ context.Context, id models.ID) (*models.Event, error) {
	e, ok := f.events[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *e

	return &cp, nil
}

func (f *fakeTx) GetPerson(_ context.Context, id models.ID) (*models.Person, error) {
	p, ok := f.people[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *p

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

func (f *fakeTx) SaveEvent(_ context.Context, e *models.Event) error {
	cp := *e
	f.saved = append(f.saved, &cp)

	return nil
}

type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func event(id models.ID) *models.Event {
	return &models.Event{ID: id, Type: models.EventTypeBirth}
}

func TestUpdateEventSaves(t *testing.T) {
	existing := event("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	st := &fakeStore{tx: newFakeTx(existing, nil)}

	updated := *existing
	updated.Private = true

	if err := New(st).UpdateEvent(context.Background(), updated); err != nil {
		t.Fatalf("UpdateEvent: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || !st.tx.saved[0].Private {
		t.Fatalf("calls=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdateEventNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx(nil, nil)}

	err := New(st).UpdateEvent(context.Background(), *event("E-01ARZ3NDEKTSV4RRFFQ69G5FA1"))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

// TestUpdateEventPlaceSoftRefNotChecked: Place — мягкая ссылка, не
// проверяется на существование при обновлении, как и при создании.
func TestUpdateEventPlaceSoftRefNotChecked(t *testing.T) {
	existing := event("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	st := &fakeStore{tx: newFakeTx(existing, nil)}

	updated := *existing
	updated.Place = &models.PlaceRef{Text: "погост", Ref: "AD-01ARZ3NDEKTSV4RRFFQ69G5FA9", Type: models.TypeAdministrativeDivision}

	if err := New(st).UpdateEvent(context.Background(), updated); err != nil {
		t.Fatalf("UpdateEvent: %v, ожидался успех (Place — мягкая ссылка)", err)
	}

	if len(st.tx.saved) != 1 || st.tx.saved[0].Place == nil || st.tx.saved[0].Place.Ref != "AD-01ARZ3NDEKTSV4RRFFQ69G5FA9" {
		t.Fatalf("saved = %+v", st.tx.saved)
	}
}

// TestUpdateEventParticipantNotFound: несуществующий
// participants[i].person_id — 422 на индексированное поле, ничего не
// сохраняется.
func TestUpdateEventParticipantNotFound(t *testing.T) {
	existing := event("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	st := &fakeStore{tx: newFakeTx(existing, []models.ID{pID('1')})}

	updated := *existing
	updated.Participants = []models.EventParticipant{
		{PersonID: pID('1'), Role: "родитель"},
		{PersonID: pID('9'), Role: "свидетель"},
	}

	err := New(st).UpdateEvent(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "participants[1].person_id" {
		t.Fatalf("err = %v, want ValidationError on participants[1].person_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d записей при несуществующем участнике", len(st.tx.saved))
	}
}

func TestUpdateEventSourceCitationNotFound(t *testing.T) {
	existing := event("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	st := &fakeStore{tx: newFakeTx(existing, nil)}

	updated := *existing
	updated.Sources = []models.SourceLink{{CitationID: cID('0')}}

	err := New(st).UpdateEvent(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, want ValidationError on sources[0].citation_id", err)
	}
}
```

#### `internal/usecases/delete_event/`

`DeleteEvent` — валидация формата id, `events.DeleteEvent`.

#### `internal/usecases/delete_event/deps.go` (создать)

```go
package delete_event

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// EventRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type EventRepo interface {
	DeleteEvent(ctx context.Context, id models.ID) error
}
```

#### `internal/usecases/delete_event/scenario.go` (создать)

```go
package delete_event

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление события».
type Scenario struct {
	events EventRepo
}

// New создаёт сценарий.
func New(events EventRepo) *Scenario {
	return &Scenario{events: events}
}

// DeleteEvent удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteEvent(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.events.DeleteEvent(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeEvent)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeEvent,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

#### `internal/usecases/delete_event/scenario_test.go` (создать)

```go
package delete_event

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

func (f *fakeRepo) DeleteEvent(_ context.Context, id models.ID) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)

	return nil
}

func TestDeleteEventCallsRepo(t *testing.T) {
	repo := &fakeRepo{}
	id := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	if err := New(repo).DeleteEvent(context.Background(), id); err != nil {
		t.Fatalf("DeleteEvent: %v", err)
	}

	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("deleted = %v", repo.deleted)
	}
}

func TestDeleteEventInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	err := New(repo).DeleteEvent(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}

	if len(repo.deleted) != 0 {
		t.Fatalf("repo вызван при невалидном id")
	}
}

func TestDeleteEventPropagatesInUseError(t *testing.T) {
	wantErr := &models.InUseError{Type: models.TypeEvent, ID: "E-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	repo := &fakeRepo{err: wantErr}

	err := New(repo).DeleteEvent(context.Background(), "E-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	var iu *models.InUseError
	if !errors.As(err, &iu) {
		t.Fatalf("err = %v, want InUseError", err)
	}
}
```

### 1.4. HTTP API

12 новых файлов, по образцу `internal/httpapi/{family,family_write,family_test,family_write_test}.go` и `person*`-эквивалентов (подпроект 8), с поправкой на отсутствие `/search` у `Relation`/`Residence`. `<entity>.go` — `GET` список (с необязательными query-фильтрами `?person_id=`/`?place_id=`)/`GET по id`/`DELETE`; `<entity>_write.go` — `POST`/`PUT` (декодирование `*Create`/`*Update`, 422 на невалидные тела); `<entity>_test.go`/`<entity>_write_test.go` — тесты на фейковом сервисе.

#### `internal/httpapi/relation*.go`

Без `handleRelationSearch` — нет `/api/relations/search`. Список принимает необязательный `?person_id=`.

#### `internal/httpapi/relation.go` (создать)

```go
package httpapi

import (
	"net/http"
	"net/url"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleRelationList — GET /api/relations?person_id=&limit=&offset=.
// person_id — необязательный фильтр (ребро проходит, если совпадает с
// person_a ИЛИ person_b); без него список плоский. Замена search_relations —
// см. registerRelationRoutes.
func handleRelationList(relations RelationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := parseRelationQuery(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := relations.ListRelations(r.Context(), AccessFromContext(r.Context()), q)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.RelationsFromModels(list))
	}
}

// parseRelationQuery разбирает параметры запроса: person_id необязателен.
func parseRelationQuery(v url.Values) (models.RelationQuery, error) {
	var q models.RelationQuery

	if pid := v.Get("person_id"); pid != "" {
		id := models.ID(pid)
		q.PersonID = &id
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

// handleRelationGet — GET /api/relations/{id}.
func handleRelationGet(relations RelationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rel, err := relations.GetRelation(r.Context(), AccessFromContext(r.Context()), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.RelationFromModel(rel))
	}
}
```

#### `internal/httpapi/relation_write.go` (создать)

```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleRelationCreate — POST /api/relations: создаёт ребро, отвечает 201 с
// созданной записью (id генерирует сценарий). Несуществующий person_a/
// person_b — 422 (см. writeError). Запись — только для вошедшего владельца,
// см. handleFamilyCreate.
func handleRelationCreate(relations RelationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.RelationCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := relations.CreateRelation(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.RelationFromModel(created))
	}
}

// handleRelationUpdate — PUT /api/relations/{id}: полная замена kind/
// rel_type/person_a/person_b/since/until/sources/notes/private
// (fetch-then-merge, docs/data-model/entity-write.md §3).
func handleRelationUpdate(relations RelationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.RelationUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := relations.GetRelation(r.Context(), AccessFromContext(r.Context()), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Kind = m.Kind
		cur.RelType = m.RelType
		cur.PersonA = m.PersonA
		cur.PersonB = m.PersonB
		cur.Since = m.Since
		cur.Until = m.Until
		cur.Sources = m.Sources
		cur.Notes = m.Notes
		cur.Private = m.Private

		if err := relations.UpdateRelation(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.RelationFromModel(cur))
	}
}

// handleRelationDelete — DELETE /api/relations/{id}: 204 без тела; занятая
// запись — 409 со списком ссылающихся. Запись — только для вошедшего
// владельца, см. handleFamilyCreate.
func handleRelationDelete(relations RelationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := relations.DeleteRelation(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
```

#### `internal/httpapi/relation_test.go` (создать)

```go
package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeRelations struct {
	list []models.Relation
	err  error

	gotQuery models.RelationQuery

	getR      models.Relation
	gotIDs    []models.ID
	created   models.Relation
	gotCreate models.Relation
	updated   models.Relation
	deleteErr error
}

func (f *fakeRelations) ListRelations(_ context.Context, _ models.Access, q models.RelationQuery) ([]models.Relation, error) {
	f.gotQuery = q

	return f.list, f.err
}

func (f *fakeRelations) GetRelation(_ context.Context, _ models.Access, id models.ID) (models.Relation, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Relation{}, f.err
	}

	return f.getR, nil
}

func (f *fakeRelations) CreateRelation(_ context.Context, r models.Relation) (models.Relation, error) {
	f.gotCreate = r
	if f.err != nil {
		return models.Relation{}, f.err
	}

	return f.created, nil
}

func (f *fakeRelations) UpdateRelation(_ context.Context, r models.Relation) error {
	f.updated = r

	return f.err
}

func (f *fakeRelations) DeleteRelation(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestRelationListReturnsRecords(t *testing.T) {
	svc := &fakeRelations{list: []models.Relation{{ID: "RL-1", Kind: models.RelationKindBlood, PersonA: "I-1", PersonB: "I-2"}}}

	rec := get(t, NewHandler(Deps{Relations: svc, DocsFS: fstest.MapFS{}}), "/api/relations")
	requireStatus(t, rec, 200)

	if !strings.Contains(rec.Body.String(), `"person_a":"I-1"`) {
		t.Fatalf("body = %s", rec.Body)
	}
}

// TestRelationListPassesPersonIDFilter: ?person_id= разбирается в
// RelationQuery.PersonID.
func TestRelationListPassesPersonIDFilter(t *testing.T) {
	svc := &fakeRelations{}

	rec := get(t, NewHandler(Deps{Relations: svc, DocsFS: fstest.MapFS{}}), "/api/relations?person_id=I-1")
	requireStatus(t, rec, 200)

	if svc.gotQuery.PersonID == nil || *svc.gotQuery.PersonID != "I-1" {
		t.Fatalf("gotQuery.PersonID = %v", svc.gotQuery.PersonID)
	}
}

func TestRelationListWithoutPersonIDLeavesFilterNil(t *testing.T) {
	svc := &fakeRelations{}

	rec := get(t, NewHandler(Deps{Relations: svc, DocsFS: fstest.MapFS{}}), "/api/relations")
	requireStatus(t, rec, 200)

	if svc.gotQuery.PersonID != nil {
		t.Fatalf("gotQuery.PersonID = %v, want nil", svc.gotQuery.PersonID)
	}
}

func TestRelationGetNotFound(t *testing.T) {
	svc := &fakeRelations{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Relations: svc, DocsFS: fstest.MapFS{}}), "/api/relations/RL-1")
	requireStatus(t, rec, 404)
}

// TestRelationNoSearchRoute: search_relations намеренно не заводится — нет
// /api/relations/search (docs/data-model/entity-write.md §3.8). Запрос по
// такому пути должен попасть в handleRelationGet как id="search" и 404
// (запись "search" не существует), а не в отдельный обработчик поиска.
func TestRelationNoSearchRoute(t *testing.T) {
	svc := &fakeRelations{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Relations: svc, DocsFS: fstest.MapFS{}}), "/api/relations/search")
	requireStatus(t, rec, 404)

	if len(svc.gotIDs) != 1 || svc.gotIDs[0] != "search" {
		t.Fatalf("gotIDs = %v, ожидался вызов GetRelation с id=\"search\" (нет отдельного маршрута /search)", svc.gotIDs)
	}
}
```

#### `internal/httpapi/relation_write_test.go` (создать)

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

func TestRelationCreateContract(t *testing.T) {
	svc := &fakeRelations{created: models.Relation{ID: "RL-1", Kind: models.RelationKindBlood, PersonA: "I-1", PersonB: "I-2"}}

	rec := postD(t, NewHandler(Deps{Relations: svc, DocsFS: fstest.MapFS{}}), "/api/relations",
		`{"kind":"blood","person_a":"I-1","person_b":"I-2"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.PersonA != "I-1" || svc.gotCreate.PersonB != "I-2" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
	if !strings.Contains(rec.Body.String(), `"person_a":"I-1"`) {
		t.Fatalf("body = %s", rec.Body)
	}
}

func TestRelationCreateAnonymousIs401(t *testing.T) {
	svc := &fakeRelations{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Relations: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/relations", strings.NewReader(`{"kind":"blood","person_a":"I-1","person_b":"I-2"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestRelationUpdateMergesFields(t *testing.T) {
	svc := &fakeRelations{getR: models.Relation{ID: "RL-1", Kind: models.RelationKindBlood, PersonA: "I-1", PersonB: "I-2"}}

	rec := putD(t, NewHandler(Deps{Relations: svc, DocsFS: fstest.MapFS{}}), "/api/relations/RL-1",
		`{"kind":"marriage","person_a":"I-1","person_b":"I-2","private":true}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Kind != models.RelationKindMarriage || svc.updated.Private != true || svc.updated.ID != "RL-1" {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestRelationDeleteNoContent(t *testing.T) {
	svc := &fakeRelations{}

	rec := delD(t, NewHandler(Deps{Relations: svc, DocsFS: fstest.MapFS{}}), "/api/relations/RL-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestRelationDeleteInUseIs409(t *testing.T) {
	svc := &fakeRelations{deleteErr: &models.InUseError{Type: models.TypeRelation, ID: "RL-1"}}

	rec := delD(t, NewHandler(Deps{Relations: svc, DocsFS: fstest.MapFS{}}), "/api/relations/RL-1")

	requireStatus(t, rec, http.StatusConflict)
}
```

#### `internal/httpapi/residence*.go`

Без `handleResidenceSearch`. Список принимает необязательные `?person_id=`/`?place_id=` (пересекаются, если оба заданы).

#### `internal/httpapi/residence.go` (создать)

```go
package httpapi

import (
	"net/http"
	"net/url"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleResidenceList — GET /api/residences?person_id=&place_id=&limit=&offset=.
// Оба фильтра необязательны и пересекаются, если заданы вместе. Замена
// search_residences — см. registerResidenceRoutes.
func handleResidenceList(residences ResidenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := parseResidenceQuery(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := residences.ListResidences(r.Context(), AccessFromContext(r.Context()), q)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ResidencesFromModels(list))
	}
}

// parseResidenceQuery разбирает параметры запроса: person_id/place_id необязательны.
func parseResidenceQuery(v url.Values) (models.ResidenceQuery, error) {
	var q models.ResidenceQuery

	if pid := v.Get("person_id"); pid != "" {
		id := models.ID(pid)
		q.PersonID = &id
	}

	if plid := v.Get("place_id"); plid != "" {
		id := models.ID(plid)
		q.PlaceID = &id
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

// handleResidenceGet — GET /api/residences/{id}.
func handleResidenceGet(residences ResidenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		res, err := residences.GetResidence(r.Context(), AccessFromContext(r.Context()), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ResidenceFromModel(res))
	}
}
```

#### `internal/httpapi/residence_write.go` (создать)

```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleResidenceCreate — POST /api/residences: создаёт запись, отвечает
// 201 с созданной записью (id генерирует сценарий). Несуществующий
// person_id/place_id — 422 (см. writeError). Запись — только для вошедшего
// владельца, см. handleFamilyCreate.
func handleResidenceCreate(residences ResidenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.ResidenceCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := residences.CreateResidence(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.ResidenceFromModel(created))
	}
}

// handleResidenceUpdate — PUT /api/residences/{id}: полная замена
// person_id/place_id/since/until/sources/note/private (fetch-then-merge,
// docs/data-model/entity-write.md §3).
func handleResidenceUpdate(residences ResidenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.ResidenceUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := residences.GetResidence(r.Context(), AccessFromContext(r.Context()), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.PersonID = m.PersonID
		cur.PlaceID = m.PlaceID
		cur.Since = m.Since
		cur.Until = m.Until
		cur.Sources = m.Sources
		cur.Note = m.Note
		cur.Private = m.Private

		if err := residences.UpdateResidence(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.ResidenceFromModel(cur))
	}
}

// handleResidenceDelete — DELETE /api/residences/{id}: 204 без тела; занятая
// запись — 409 со списком ссылающихся. Запись — только для вошедшего
// владельца, см. handleFamilyCreate.
func handleResidenceDelete(residences ResidenceService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := residences.DeleteResidence(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
```

#### `internal/httpapi/residence_test.go` (создать)

```go
package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeResidences struct {
	list []models.Residence
	err  error

	gotQuery models.ResidenceQuery

	getR      models.Residence
	gotIDs    []models.ID
	created   models.Residence
	gotCreate models.Residence
	updated   models.Residence
	deleteErr error
}

func (f *fakeResidences) ListResidences(_ context.Context, _ models.Access, q models.ResidenceQuery) ([]models.Residence, error) {
	f.gotQuery = q

	return f.list, f.err
}

func (f *fakeResidences) GetResidence(_ context.Context, _ models.Access, id models.ID) (models.Residence, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Residence{}, f.err
	}

	return f.getR, nil
}

func (f *fakeResidences) CreateResidence(_ context.Context, r models.Residence) (models.Residence, error) {
	f.gotCreate = r
	if f.err != nil {
		return models.Residence{}, f.err
	}

	return f.created, nil
}

func (f *fakeResidences) UpdateResidence(_ context.Context, r models.Residence) error {
	f.updated = r

	return f.err
}

func (f *fakeResidences) DeleteResidence(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestResidenceListReturnsRecords(t *testing.T) {
	svc := &fakeResidences{list: []models.Residence{{ID: "RS-1", PersonID: "I-1", PlaceID: "AD-1"}}}

	rec := get(t, NewHandler(Deps{Residences: svc, DocsFS: fstest.MapFS{}}), "/api/residences")
	requireStatus(t, rec, 200)

	if !strings.Contains(rec.Body.String(), `"person_id":"I-1"`) {
		t.Fatalf("body = %s", rec.Body)
	}
}

// TestResidenceListPassesBothFilters: ?person_id=&place_id= оба разбираются.
func TestResidenceListPassesBothFilters(t *testing.T) {
	svc := &fakeResidences{}

	rec := get(t, NewHandler(Deps{Residences: svc, DocsFS: fstest.MapFS{}}), "/api/residences?person_id=I-1&place_id=AD-1")
	requireStatus(t, rec, 200)

	if svc.gotQuery.PersonID == nil || *svc.gotQuery.PersonID != "I-1" {
		t.Fatalf("gotQuery.PersonID = %v", svc.gotQuery.PersonID)
	}
	if svc.gotQuery.PlaceID == nil || *svc.gotQuery.PlaceID != "AD-1" {
		t.Fatalf("gotQuery.PlaceID = %v", svc.gotQuery.PlaceID)
	}
}

func TestResidenceGetNotFound(t *testing.T) {
	svc := &fakeResidences{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Residences: svc, DocsFS: fstest.MapFS{}}), "/api/residences/RS-1")
	requireStatus(t, rec, 404)
}

// TestResidenceNoSearchRoute: search_residences намеренно не заводится —
// см. TestRelationNoSearchRoute.
func TestResidenceNoSearchRoute(t *testing.T) {
	svc := &fakeResidences{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Residences: svc, DocsFS: fstest.MapFS{}}), "/api/residences/search")
	requireStatus(t, rec, 404)

	if len(svc.gotIDs) != 1 || svc.gotIDs[0] != "search" {
		t.Fatalf("gotIDs = %v, ожидался вызов GetResidence с id=\"search\"", svc.gotIDs)
	}
}
```

#### `internal/httpapi/residence_write_test.go` (создать)

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

func TestResidenceCreateContract(t *testing.T) {
	svc := &fakeResidences{created: models.Residence{ID: "RS-1", PersonID: "I-1", PlaceID: "AD-1"}}

	rec := postD(t, NewHandler(Deps{Residences: svc, DocsFS: fstest.MapFS{}}), "/api/residences",
		`{"person_id":"I-1","place_id":"AD-1"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.PersonID != "I-1" || svc.gotCreate.PlaceID != "AD-1" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
	if !strings.Contains(rec.Body.String(), `"place_id":"AD-1"`) {
		t.Fatalf("body = %s", rec.Body)
	}
}

func TestResidenceCreateAnonymousIs401(t *testing.T) {
	svc := &fakeResidences{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Residences: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/residences", strings.NewReader(`{"person_id":"I-1","place_id":"AD-1"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestResidenceUpdateMergesFields(t *testing.T) {
	svc := &fakeResidences{getR: models.Residence{ID: "RS-1", PersonID: "I-1", PlaceID: "AD-1"}}

	rec := putD(t, NewHandler(Deps{Residences: svc, DocsFS: fstest.MapFS{}}), "/api/residences/RS-1",
		`{"person_id":"I-1","place_id":"AD-1","note":"изба","private":true}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Note != "изба" || svc.updated.Private != true || svc.updated.ID != "RS-1" {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestResidenceDeleteNoContent(t *testing.T) {
	svc := &fakeResidences{}

	rec := delD(t, NewHandler(Deps{Residences: svc, DocsFS: fstest.MapFS{}}), "/api/residences/RS-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestResidenceDeleteInUseIs409(t *testing.T) {
	svc := &fakeResidences{deleteErr: &models.InUseError{Type: models.TypeResidence, ID: "RS-1"}}

	rec := delD(t, NewHandler(Deps{Residences: svc, DocsFS: fstest.MapFS{}}), "/api/residences/RS-1")

	requireStatus(t, rec, http.StatusConflict)
}
```

#### `internal/httpapi/event*.go`

Полный набор из 6 маршрутов, включая `handleEventSearch` (`/api/events/search`). Список принимает необязательный `?person_id=`.

#### `internal/httpapi/event.go` (создать)

```go
package httpapi

import (
	"net/http"
	"net/url"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleEventList — GET /api/events?person_id=&limit=&offset=. person_id —
// необязательный фильтр по участнику (совпадает с любым из
// Participants[i].PersonID).
func handleEventList(events EventService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := parseEventQuery(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := events.ListEvents(r.Context(), AccessFromContext(r.Context()), q)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.EventsFromModels(list))
	}
}

// parseEventQuery разбирает параметры запроса: person_id необязателен.
func parseEventQuery(v url.Values) (models.EventQuery, error) {
	var q models.EventQuery

	if pid := v.Get("person_id"); pid != "" {
		id := models.ID(pid)
		q.PersonID = &id
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

// handleEventSearch — GET /api/events/search?q=&limit=&offset=. Ищет ТОЛЬКО
// по началу текста места (place) — единственное индексируемое поле события
// (см. transport.Event).
func handleEventSearch(events EventService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := events.SearchEvents(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.EventsFromModels(list))
	}
}

// handleEventGet — GET /api/events/{id}.
func handleEventGet(events EventService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		e, err := events.GetEvent(r.Context(), AccessFromContext(r.Context()), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.EventFromModel(e))
	}
}
```

#### `internal/httpapi/event_write.go` (создать)

```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleEventCreate — POST /api/events: создаёт событие, отвечает 201 с
// созданной записью (id генерирует сценарий). Несуществующий
// participants[i].person_id — 422 (см. writeError); Place — мягкая ссылка,
// НИКОГДА не проверяется на существование. Запись — только для вошедшего
// владельца, см. handleFamilyCreate.
func handleEventCreate(events EventService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.EventCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := events.CreateEvent(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.EventFromModel(created))
	}
}

// handleEventUpdate — PUT /api/events/{id}: полная замена type/date/place/
// participants/sources/notes/private (fetch-then-merge, docs/data-model/
// entity-write.md §3).
func handleEventUpdate(events EventService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.EventUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := events.GetEvent(r.Context(), AccessFromContext(r.Context()), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Type = m.Type
		cur.Date = m.Date
		cur.Place = m.Place
		cur.Participants = m.Participants
		cur.Sources = m.Sources
		cur.Notes = m.Notes
		cur.Private = m.Private

		if err := events.UpdateEvent(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.EventFromModel(cur))
	}
}

// handleEventDelete — DELETE /api/events/{id}: 204 без тела; занятая запись —
// 409 со списком ссылающихся. Запись — только для вошедшего владельца, см.
// handleFamilyCreate.
func handleEventDelete(events EventService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := events.DeleteEvent(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
```

#### `internal/httpapi/event_test.go` (создать)

```go
package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeEvents struct {
	list []models.Event
	err  error

	gotQuery  models.EventQuery
	gotSearch models.SearchQuery
	search    []models.Event

	getE      models.Event
	gotIDs    []models.ID
	created   models.Event
	gotCreate models.Event
	updated   models.Event
	deleteErr error
}

func (f *fakeEvents) ListEvents(_ context.Context, _ models.Access, q models.EventQuery) ([]models.Event, error) {
	f.gotQuery = q

	return f.list, f.err
}

func (f *fakeEvents) SearchEvents(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Event, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeEvents) GetEvent(_ context.Context, _ models.Access, id models.ID) (models.Event, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Event{}, f.err
	}

	return f.getE, nil
}

func (f *fakeEvents) CreateEvent(_ context.Context, e models.Event) (models.Event, error) {
	f.gotCreate = e
	if f.err != nil {
		return models.Event{}, f.err
	}

	return f.created, nil
}

func (f *fakeEvents) UpdateEvent(_ context.Context, e models.Event) error {
	f.updated = e

	return f.err
}

func (f *fakeEvents) DeleteEvent(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestEventListReturnsRecords(t *testing.T) {
	svc := &fakeEvents{list: []models.Event{{ID: "E-1", Type: models.EventTypeBirth}}}

	rec := get(t, NewHandler(Deps{Events: svc, DocsFS: fstest.MapFS{}}), "/api/events")
	requireStatus(t, rec, 200)

	if !strings.Contains(rec.Body.String(), `"type":"birth"`) {
		t.Fatalf("body = %s", rec.Body)
	}
}

func TestEventListPassesPersonIDFilter(t *testing.T) {
	svc := &fakeEvents{}

	rec := get(t, NewHandler(Deps{Events: svc, DocsFS: fstest.MapFS{}}), "/api/events?person_id=I-1")
	requireStatus(t, rec, 200)

	if svc.gotQuery.PersonID == nil || *svc.gotQuery.PersonID != "I-1" {
		t.Fatalf("gotQuery.PersonID = %v", svc.gotQuery.PersonID)
	}
}

func TestEventGetNotFound(t *testing.T) {
	svc := &fakeEvents{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Events: svc, DocsFS: fstest.MapFS{}}), "/api/events/E-1")
	requireStatus(t, rec, 404)
}

// TestEventSearchPassesQuery: /api/events/search — существует, в отличие от
// relations/residences (Event ЕСТЬ поисковый индекс, docs/data-model/
// entity-write.md §3.8).
func TestEventSearchPassesQuery(t *testing.T) {
	svc := &fakeEvents{}

	rec := get(t, NewHandler(Deps{Events: svc, DocsFS: fstest.MapFS{}}), "/api/events/search?q=Дав")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "Дав" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
```

#### `internal/httpapi/event_write_test.go` (создать)

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

func TestEventCreateContract(t *testing.T) {
	svc := &fakeEvents{created: models.Event{ID: "E-1", Type: models.EventTypeBirth}}

	rec := postD(t, NewHandler(Deps{Events: svc, DocsFS: fstest.MapFS{}}), "/api/events",
		`{"type":"birth"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Type != models.EventTypeBirth || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
	if !strings.Contains(rec.Body.String(), `"type":"birth"`) {
		t.Fatalf("body = %s", rec.Body)
	}
}

func TestEventCreateAnonymousIs401(t *testing.T) {
	svc := &fakeEvents{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Events: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/events", strings.NewReader(`{"type":"birth"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestEventUpdateMergesFields(t *testing.T) {
	svc := &fakeEvents{getE: models.Event{ID: "E-1", Type: models.EventTypeBirth}}

	rec := putD(t, NewHandler(Deps{Events: svc, DocsFS: fstest.MapFS{}}), "/api/events/E-1",
		`{"type":"death","private":true}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Type != models.EventTypeDeath || svc.updated.Private != true || svc.updated.ID != "E-1" {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestEventDeleteNoContent(t *testing.T) {
	svc := &fakeEvents{}

	rec := delD(t, NewHandler(Deps{Events: svc, DocsFS: fstest.MapFS{}}), "/api/events/E-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestEventDeleteInUseIs409(t *testing.T) {
	svc := &fakeEvents{deleteErr: &models.InUseError{Type: models.TypeEvent, ID: "E-1"}}

	rec := delD(t, NewHandler(Deps{Events: svc, DocsFS: fstest.MapFS{}}), "/api/events/E-1")

	requireStatus(t, rec, http.StatusConflict)
}
```

### 1.5. MCP

6 новых файлов (`<entity>.go` + `<entity>_test.go`), по образцу `internal/mcp/{family,person}*.go`. `relation_*` (5 тулов, без `relation_search`), `residence_*` (5, без `residence_search`), `event_*` (6, включая `event_search`). Update-guard-семантика (критично, по program-wide фиксу, наследуемому этим подпроектом, а не изобретаемому им): `Relation.Kind`/`PersonA`/`.PersonB`, `Residence.PersonID`/`.PlaceID`, `Event.Type` — `mcp.Required()`, заменяются БЕЗУСЛОВНО (как `Citation.SourceID`); `Relation.RelType` — обычный preserve-on-omit скаляр; `Relation.Since`/`.Until`, `Residence.Since`/`.Until`, `Event.Date`/`.Place` — одиночные объектные поля, PRESENCE-ONLY guard (`if _, ok := args["x"]; ok { ... }`, БЕЗ `raw != nil`); `Event.Participants`, `sources`, `notes`, `Residence.Sources`, `Relation.Sources`/`.Notes` — стандартный preserve-on-omit array-of-objects/array-of-strings guard.

#### `internal/mcp/relation*.go`

`relation_create`/`relation_update` принимают `since`/`until` (`factDateObjectProperties`), `sources` (`sourceLinkObjectProperties`), `notes` (строки). `relation_list` — необязательный `person_id`, БЕЗ `relation_search`.

#### `internal/mcp/relation.go` (создать)

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

// registerRelationTools регистрирует тулы для работы с рёбрами графа
// родства. Без relation_search — search_relations намеренно не заводится
// (у Relation нет собственных поисковых полей, индекс пуст, docs/data-model/
// entity-write.md §3.8); relation_list принимает необязательный person_id —
// более полезная замена (ребро проходит, если совпадает с person_a ИЛИ
// person_b). person_a/person_b/kind — REQUIRED и в relation_update
// (безусловная полная замена, как citation_update's source_id) — это ядро
// того, что представляет собой ребро, преserve-on-omit тут неуместен.
// rel_type — необязателен (его условная обязательность при kind=associate
// проверяется моделью, не MCP-схемой), обычный preserve-on-omit.
// since/until — одиночные объектные поля (*FactDate): presence-ONLY guard
// (явный null очищает, отсутствие ключа сохраняет) — см. цитированный в
// задаче фикс в family.go/archive_node.go.
func registerRelationTools(s *server.MCPServer, relations RelationService) {
	tool := mcp.NewTool(
		"relation_list",
		mcp.WithDescription("Список рёбер графа родства в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithString("person_id", mcp.Description("Необязательный фильтр: только рёбра, где эта персона — person_a или person_b")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, relationListHandler(relations))

	tool = mcp.NewTool(
		"relation_get",
		mcp.WithDescription("Ребро графа родства по id; результат — JSON записи. Неверный формат id или отсутствующая/приватная (для не-владельца) запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например RL-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, relationGetHandler(relations))

	tool = mcp.NewTool(
		"relation_create",
		mcp.WithDescription("Создать ребро графа родства; id генерируется сервером; результат — JSON созданной записи. Несуществующие person_a/person_b — ошибка тула"),
		mcp.WithString("kind", mcp.Required(), mcp.Enum("blood", "marriage", "adoption", "associate"), mcp.Description("Вид связи")),
		mcp.WithString("rel_type", mcp.Description("Вид связи для kind=associate (обязателен только при этом kind, иначе должен отсутствовать)")),
		mcp.WithString("person_a", mcp.Required(), mcp.Description("id первой персоны")),
		mcp.WithString("person_b", mcp.Required(), mcp.Description("id второй персоны (должна отличаться от person_a)")),
		mcp.WithObject("since", mcp.Description("Начало периода действия связи"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("until", mcp.Description("Конец периода действия связи"), mcp.Properties(factDateObjectProperties())),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, relationCreateHandler(relations))

	tool = mcp.NewTool(
		"relation_update",
		mcp.WithDescription("Изменить ребро графа родства: kind/person_a/person_b заменяются безусловно при каждом вызове (ядро того, что представляет собой ребро); rel_type/sources/notes/private — при отсутствии аргумента сохраняют текущее значение, явное пустое значение/пустой список — очищает; since/until — одиночные объектные поля: отсутствие ключа сохраняет текущее значение, явный null — очищает (пустой объект {} для очистки не подходит — не проходит валидацию); результат — JSON обновлённой записи"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("kind", mcp.Required(), mcp.Enum("blood", "marriage", "adoption", "associate"), mcp.Description("Вид связи")),
		mcp.WithString("rel_type", mcp.Description("Вид связи для kind=associate")),
		mcp.WithString("person_a", mcp.Required(), mcp.Description("id первой персоны")),
		mcp.WithString("person_b", mcp.Required(), mcp.Description("id второй персоны")),
		mcp.WithObject("since", mcp.Description("Начало периода действия связи"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("until", mcp.Description("Конец периода действия связи"), mcp.Properties(factDateObjectProperties())),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, relationUpdateHandler(relations))

	tool = mcp.NewTool(
		"relation_delete",
		mcp.WithDescription("Удалить ребро графа родства. Необратимо. Если на него есть строгие ссылки — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, relationDeleteHandler(relations))
}

func relationListHandler(relations RelationService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.RelationQuery{}

		if pid := req.GetString("person_id", ""); pid != "" {
			id := models.ID(pid)
			q.PersonID = &id
		}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := relations.ListRelations(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.RelationsFromModels(list))
	}
}

func relationGetHandler(relations RelationService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		r, err := relations.GetRelation(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.RelationFromModel(r))
	}
}

func relationCreateHandler(relations RelationService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

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

		r := models.Relation{
			Kind:    models.RelationKind(req.GetString("kind", "")),
			RelType: models.RelationType(req.GetString("rel_type", "")),
			PersonA: models.ID(req.GetString("person_a", "")),
			PersonB: models.ID(req.GetString("person_b", "")),
			Since:   since,
			Until:   until,
			Sources: sources,
			Notes:   textRefsFromStrings(req.GetStringSlice("notes", nil)),
			Private: req.GetBool("private", false),
		}

		created, err := relations.CreateRelation(ctx, r)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.RelationFromModel(created))
	}
}

func relationUpdateHandler(relations RelationService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := relations.GetRelation(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		args := req.GetArguments()

		cur.Kind = models.RelationKind(req.GetString("kind", ""))
		cur.PersonA = models.ID(req.GetString("person_a", ""))
		cur.PersonB = models.ID(req.GetString("person_b", ""))

		if raw, ok := args["rel_type"]; ok && raw != nil {
			cur.RelType = models.RelationType(req.GetString("rel_type", ""))
		}

		if _, ok := args["since"]; ok {
			since, err := optionalFactDate(args, "since")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Since = since
		}

		if _, ok := args["until"]; ok {
			until, err := optionalFactDate(args, "until")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Until = until
		}

		if raw, ok := args["sources"]; ok && raw != nil {
			sources, err := optionalSourceLinks(args, "sources")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Sources = sources
		}

		if raw, ok := args["notes"]; ok && raw != nil {
			cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))
		}

		if raw, ok := args["private"]; ok && raw != nil {
			cur.Private = req.GetBool("private", false)
		}

		if err := relations.UpdateRelation(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.RelationFromModel(cur))
	}
}

func relationDeleteHandler(relations RelationService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := relations.DeleteRelation(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
```

#### `internal/mcp/relation_test.go` (создать)

```go
package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakeRelations struct {
	list []models.Relation
	err  error

	gotQuery models.RelationQuery

	getR      models.Relation
	created   models.Relation
	gotCreate models.Relation
	updated   models.Relation
	gotIDs    []models.ID
	deleteErr error
}

func (f *fakeRelations) ListRelations(_ context.Context, _ models.Access, q models.RelationQuery) ([]models.Relation, error) {
	f.gotQuery = q

	return f.list, f.err
}

func (f *fakeRelations) GetRelation(_ context.Context, _ models.Access, id models.ID) (models.Relation, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Relation{}, f.err
	}

	return f.getR, nil
}

func (f *fakeRelations) CreateRelation(_ context.Context, r models.Relation) (models.Relation, error) {
	f.gotCreate = r
	if f.err != nil {
		return models.Relation{}, f.err
	}

	return f.created, nil
}

func (f *fakeRelations) UpdateRelation(_ context.Context, r models.Relation) error {
	f.updated = r

	return f.err
}

func (f *fakeRelations) DeleteRelation(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callRelationTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestRelationListToolPassesPersonIDFilter(t *testing.T) {
	svc := &fakeRelations{}

	res := callRelationTool(t, relationListHandler(svc), map[string]any{"person_id": "I-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotQuery.PersonID == nil || *svc.gotQuery.PersonID != "I-1" {
		t.Fatalf("gotQuery.PersonID = %v", svc.gotQuery.PersonID)
	}
}

func TestRelationGetToolContract(t *testing.T) {
	svc := &fakeRelations{getR: models.Relation{ID: "RL-1", Kind: models.RelationKindBlood}}

	res := callRelationTool(t, relationGetHandler(svc), map[string]any{"id": "RL-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "RL-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

func TestRelationCreateToolPassesFields(t *testing.T) {
	svc := &fakeRelations{created: models.Relation{ID: "RL-new", Kind: models.RelationKindBlood, PersonA: "I-1", PersonB: "I-2"}}

	res := callRelationTool(t, relationCreateHandler(svc), map[string]any{
		"kind": "blood", "person_a": "I-1", "person_b": "I-2",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.PersonA != "I-1" || svc.gotCreate.PersonB != "I-2" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestRelationUpdateToolCoreFieldsAlwaysReplaced: kind/person_a/person_b —
// REQUIRED, всегда заменяются безусловно, даже когда значение совпадает со
// старым — сам факт "прошли через _update" достаточен (mirroring citation's
// source_id).
func TestRelationUpdateToolCoreFieldsAlwaysReplaced(t *testing.T) {
	svc := &fakeRelations{getR: models.Relation{ID: "RL-1", Kind: models.RelationKindBlood, PersonA: "I-1", PersonB: "I-2"}}

	res := callRelationTool(t, relationUpdateHandler(svc), map[string]any{
		"id": "RL-1", "kind": "marriage", "person_a": "I-3", "person_b": "I-4",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Kind != models.RelationKindMarriage || svc.updated.PersonA != "I-3" || svc.updated.PersonB != "I-4" {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

// TestRelationUpdateToolOmittedRelTypeKeepsCurrent: rel_type — обычный
// preserve-on-omit скаляр (в отличие от kind/person_a/person_b).
func TestRelationUpdateToolOmittedRelTypeKeepsCurrent(t *testing.T) {
	svc := &fakeRelations{getR: models.Relation{
		ID: "RL-1", Kind: models.RelationKindAssociate, RelType: models.RelationTypeGodparent, PersonA: "I-1", PersonB: "I-2",
	}}

	res := callRelationTool(t, relationUpdateHandler(svc), map[string]any{
		"id": "RL-1", "kind": "associate", "person_a": "I-1", "person_b": "I-2",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.RelType != models.RelationTypeGodparent {
		t.Fatalf("updated.RelType = %q, ожидалось сохранение текущего", svc.updated.RelType)
	}
}

// TestRelationUpdateToolOmittedSinceKeepsCurrent: since — одиночное
// объектное поле (*FactDate), presence-only guard: отсутствие ключа
// сохраняет текущее значение.
func TestRelationUpdateToolOmittedSinceKeepsCurrent(t *testing.T) {
	svc := &fakeRelations{getR: models.Relation{
		ID: "RL-1", Kind: models.RelationKindBlood, PersonA: "I-1", PersonB: "I-2",
		Since: &models.FactDate{Year: 1850, Precision: models.PrecisionYear},
	}}

	res := callRelationTool(t, relationUpdateHandler(svc), map[string]any{
		"id": "RL-1", "kind": "blood", "person_a": "I-1", "person_b": "I-2",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Since == nil || svc.updated.Since.Year != 1850 {
		t.Fatalf("updated.Since = %v, ожидалось сохранение текущего", svc.updated.Since)
	}
}

// TestRelationUpdateToolNullSinceClears: явный null у since очищает поле
// (presence-only guard — {} не проходит валидацию, значит очистка только null'ом).
func TestRelationUpdateToolNullSinceClears(t *testing.T) {
	svc := &fakeRelations{getR: models.Relation{
		ID: "RL-1", Kind: models.RelationKindBlood, PersonA: "I-1", PersonB: "I-2",
		Since: &models.FactDate{Year: 1850, Precision: models.PrecisionYear},
	}}

	res := callRelationTool(t, relationUpdateHandler(svc), map[string]any{
		"id": "RL-1", "kind": "blood", "person_a": "I-1", "person_b": "I-2", "since": nil,
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Since != nil {
		t.Fatalf("updated.Since = %v, ожидался nil (явный null очищает поле)", svc.updated.Since)
	}
}

func TestRelationDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeRelations{deleteErr: &models.InUseError{Type: models.TypeRelation, ID: "RL-1"}}

	res := callRelationTool(t, relationDeleteHandler(svc), map[string]any{"id": "RL-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

// TestNewServerRegistersRelationTools: 5 тулов, без relation_search.
func TestNewServerRegistersRelationTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Relations: &fakeRelations{}}).ListTools()

	for _, name := range []string{"relation_list", "relation_get", "relation_create", "relation_update", "relation_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}

	if _, ok := tools["relation_search"]; ok {
		t.Errorf("relation_search не должен существовать (search_relations намеренно не заводится)")
	}
}
```

#### `internal/mcp/residence*.go`

`residence_create`/`residence_update` принимают `since`/`until`, `sources`, `note` (одна строка). `residence_list` — необязательные `person_id`/`place_id`, БЕЗ `residence_search`.

#### `internal/mcp/residence.go` (создать)

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

// registerResidenceTools регистрирует тулы для работы с проживаниями. Без
// residence_search — search_residences намеренно не заводится (индекс пуст,
// docs/data-model/entity-write.md §3.8); residence_list принимает
// необязательные person_id/place_id (пересекаются, если оба заданы).
// person_id/place_id — REQUIRED и в residence_update (безусловная полная
// замена, ядро того, что представляет собой запись — как у relation_update);
// since/until — presence-ONLY guard (см. registerRelationTools). note —
// единственная строка (не список), обычный preserve-on-omit скаляр.
func registerResidenceTools(s *server.MCPServer, residences ResidenceService) {
	tool := mcp.NewTool(
		"residence_list",
		mcp.WithDescription("Список проживаний в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithString("person_id", mcp.Description("Необязательный фильтр по персоне")),
		mcp.WithString("place_id", mcp.Description("Необязательный фильтр по месту")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, residenceListHandler(residences))

	tool = mcp.NewTool(
		"residence_get",
		mcp.WithDescription("Проживание по id; результат — JSON записи. Неверный формат id или отсутствующая/приватная (для не-владельца) запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например RS-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, residenceGetHandler(residences))

	tool = mcp.NewTool(
		"residence_create",
		mcp.WithDescription("Создать проживание; id генерируется сервером; результат — JSON созданной записи. Несуществующие person_id/place_id — ошибка тула"),
		mcp.WithString("person_id", mcp.Required(), mcp.Description("id персоны")),
		mcp.WithString("place_id", mcp.Required(), mcp.Description("id места (административное деление)")),
		mcp.WithObject("since", mcp.Description("Начало проживания"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("until", mcp.Description("Конец проживания"), mcp.Properties(factDateObjectProperties())),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithString("note", mcp.Description("Заметка")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, residenceCreateHandler(residences))

	tool = mcp.NewTool(
		"residence_update",
		mcp.WithDescription("Изменить проживание: person_id/place_id заменяются безусловно при каждом вызове; sources/note/private — при отсутствии аргумента сохраняют текущее значение, явное пустое значение/пустой список — очищает; since/until — одиночные объектные поля: отсутствие ключа сохраняет текущее значение, явный null — очищает; результат — JSON обновлённой записи"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("person_id", mcp.Required(), mcp.Description("id персоны")),
		mcp.WithString("place_id", mcp.Required(), mcp.Description("id места")),
		mcp.WithObject("since", mcp.Description("Начало проживания"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("until", mcp.Description("Конец проживания"), mcp.Properties(factDateObjectProperties())),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithString("note", mcp.Description("Заметка")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, residenceUpdateHandler(residences))

	tool = mcp.NewTool(
		"residence_delete",
		mcp.WithDescription("Удалить проживание. Необратимо. Если на него есть строгие ссылки — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, residenceDeleteHandler(residences))
}

func residenceListHandler(residences ResidenceService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.ResidenceQuery{}

		if pid := req.GetString("person_id", ""); pid != "" {
			id := models.ID(pid)
			q.PersonID = &id
		}

		if plid := req.GetString("place_id", ""); plid != "" {
			id := models.ID(plid)
			q.PlaceID = &id
		}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := residences.ListResidences(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.ResidencesFromModels(list))
	}
}

func residenceGetHandler(residences ResidenceService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		r, err := residences.GetResidence(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.ResidenceFromModel(r))
	}
}

func residenceCreateHandler(residences ResidenceService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

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

		r := models.Residence{
			PersonID: models.ID(req.GetString("person_id", "")),
			PlaceID:  models.ID(req.GetString("place_id", "")),
			Since:    since,
			Until:    until,
			Sources:  sources,
			Note:     req.GetString("note", ""),
			Private:  req.GetBool("private", false),
		}

		created, err := residences.CreateResidence(ctx, r)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.ResidenceFromModel(created))
	}
}

func residenceUpdateHandler(residences ResidenceService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := residences.GetResidence(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		args := req.GetArguments()

		cur.PersonID = models.ID(req.GetString("person_id", ""))
		cur.PlaceID = models.ID(req.GetString("place_id", ""))

		if _, ok := args["since"]; ok {
			since, err := optionalFactDate(args, "since")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Since = since
		}

		if _, ok := args["until"]; ok {
			until, err := optionalFactDate(args, "until")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Until = until
		}

		if raw, ok := args["sources"]; ok && raw != nil {
			sources, err := optionalSourceLinks(args, "sources")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Sources = sources
		}

		if raw, ok := args["note"]; ok && raw != nil {
			cur.Note = req.GetString("note", "")
		}

		if raw, ok := args["private"]; ok && raw != nil {
			cur.Private = req.GetBool("private", false)
		}

		if err := residences.UpdateResidence(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.ResidenceFromModel(cur))
	}
}

func residenceDeleteHandler(residences ResidenceService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := residences.DeleteResidence(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
```

#### `internal/mcp/residence_test.go` (создать)

```go
package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakeResidences struct {
	list []models.Residence
	err  error

	gotQuery models.ResidenceQuery

	getR      models.Residence
	created   models.Residence
	gotCreate models.Residence
	updated   models.Residence
	gotIDs    []models.ID
	deleteErr error
}

func (f *fakeResidences) ListResidences(_ context.Context, _ models.Access, q models.ResidenceQuery) ([]models.Residence, error) {
	f.gotQuery = q

	return f.list, f.err
}

func (f *fakeResidences) GetResidence(_ context.Context, _ models.Access, id models.ID) (models.Residence, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Residence{}, f.err
	}

	return f.getR, nil
}

func (f *fakeResidences) CreateResidence(_ context.Context, r models.Residence) (models.Residence, error) {
	f.gotCreate = r
	if f.err != nil {
		return models.Residence{}, f.err
	}

	return f.created, nil
}

func (f *fakeResidences) UpdateResidence(_ context.Context, r models.Residence) error {
	f.updated = r

	return f.err
}

func (f *fakeResidences) DeleteResidence(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callResidenceTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestResidenceListToolPassesFilters(t *testing.T) {
	svc := &fakeResidences{}

	res := callResidenceTool(t, residenceListHandler(svc), map[string]any{"person_id": "I-1", "place_id": "AD-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotQuery.PersonID == nil || *svc.gotQuery.PersonID != "I-1" {
		t.Fatalf("gotQuery.PersonID = %v", svc.gotQuery.PersonID)
	}
	if svc.gotQuery.PlaceID == nil || *svc.gotQuery.PlaceID != "AD-1" {
		t.Fatalf("gotQuery.PlaceID = %v", svc.gotQuery.PlaceID)
	}
}

func TestResidenceGetToolContract(t *testing.T) {
	svc := &fakeResidences{getR: models.Residence{ID: "RS-1", Note: "изба"}}

	res := callResidenceTool(t, residenceGetHandler(svc), map[string]any{"id": "RS-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "RS-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

func TestResidenceCreateToolPassesFields(t *testing.T) {
	svc := &fakeResidences{created: models.Residence{ID: "RS-new", PersonID: "I-1", PlaceID: "AD-1"}}

	res := callResidenceTool(t, residenceCreateHandler(svc), map[string]any{
		"person_id": "I-1", "place_id": "AD-1", "note": "изба",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.PersonID != "I-1" || svc.gotCreate.PlaceID != "AD-1" || svc.gotCreate.Note != "изба" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestResidenceUpdateToolCoreFieldsAlwaysReplaced: person_id/place_id —
// REQUIRED, всегда заменяются безусловно.
func TestResidenceUpdateToolCoreFieldsAlwaysReplaced(t *testing.T) {
	svc := &fakeResidences{getR: models.Residence{ID: "RS-1", PersonID: "I-1", PlaceID: "AD-1"}}

	res := callResidenceTool(t, residenceUpdateHandler(svc), map[string]any{
		"id": "RS-1", "person_id": "I-2", "place_id": "AD-2",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.PersonID != "I-2" || svc.updated.PlaceID != "AD-2" {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestResidenceUpdateToolOmittedNoteKeepsCurrent(t *testing.T) {
	svc := &fakeResidences{getR: models.Residence{ID: "RS-1", PersonID: "I-1", PlaceID: "AD-1", Note: "изба"}}

	res := callResidenceTool(t, residenceUpdateHandler(svc), map[string]any{
		"id": "RS-1", "person_id": "I-1", "place_id": "AD-1",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Note != "изба" {
		t.Fatalf("updated.Note = %q, ожидалось сохранение текущего", svc.updated.Note)
	}
}

// TestResidenceUpdateToolOmittedSinceKeepsCurrent/NullSinceClears —
// одиночное объектное поле, presence-only guard (см. relation_test.go).
func TestResidenceUpdateToolOmittedSinceKeepsCurrent(t *testing.T) {
	svc := &fakeResidences{getR: models.Residence{
		ID: "RS-1", PersonID: "I-1", PlaceID: "AD-1",
		Since: &models.FactDate{Year: 1900, Precision: models.PrecisionYear},
	}}

	res := callResidenceTool(t, residenceUpdateHandler(svc), map[string]any{
		"id": "RS-1", "person_id": "I-1", "place_id": "AD-1",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Since == nil || svc.updated.Since.Year != 1900 {
		t.Fatalf("updated.Since = %v, ожидалось сохранение текущего", svc.updated.Since)
	}
}

func TestResidenceUpdateToolNullSinceClears(t *testing.T) {
	svc := &fakeResidences{getR: models.Residence{
		ID: "RS-1", PersonID: "I-1", PlaceID: "AD-1",
		Since: &models.FactDate{Year: 1900, Precision: models.PrecisionYear},
	}}

	res := callResidenceTool(t, residenceUpdateHandler(svc), map[string]any{
		"id": "RS-1", "person_id": "I-1", "place_id": "AD-1", "since": nil,
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Since != nil {
		t.Fatalf("updated.Since = %v, ожидался nil", svc.updated.Since)
	}
}

func TestResidenceDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeResidences{deleteErr: &models.InUseError{Type: models.TypeResidence, ID: "RS-1"}}

	res := callResidenceTool(t, residenceDeleteHandler(svc), map[string]any{"id": "RS-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersResidenceTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Residences: &fakeResidences{}}).ListTools()

	for _, name := range []string{"residence_list", "residence_get", "residence_create", "residence_update", "residence_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}

	if _, ok := tools["residence_search"]; ok {
		t.Errorf("residence_search не должен существовать")
	}
}
```

#### `internal/mcp/event*.go`

`event_create`/`event_update` принимают `date` (`factDateObjectProperties`), `place` (`placeRefObjectProperties`, Шаг 1.6), `participants` (`eventParticipantObjectProperties`, Шаг 1.6 — обязательные `person_id`/`role` внутри элемента), `sources`, `notes`. `event_search` ЕСТЬ, ищет только по `place`.

#### `internal/mcp/event.go` (создать)

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

// registerEventTools регистрирует тулы для работы с событиями. event_list
// принимает необязательный person_id (совпадает с любым из
// Participants[i].PersonID). event_search ЕСТЬ (в отличие от Relation/
// Residence) — ищет ТОЛЬКО по началу текста места (place), единственное
// индексируемое поле события (internal/store/sqlstore/records.go:SaveEvent).
// type — REQUIRED и в event_update (безусловная полная замена, как
// relation_update's kind). date/place — одиночные объектные поля
// (*FactDate/*PlaceRef): presence-ONLY guard (отсутствие ключа сохраняет
// текущее значение, явный null очищает — {} не проходит валидацию).
// participants — массив объектов, PersonID внутри — ПЕРВАЯ строгая
// (проверяемая на существование) ссылка внутри array-of-objects MCP-
// аргумента в программе (в отличие от sources/names, чьи ссылки мягкие или
// уже установлены) — обычный preserve-on-omit array-of-objects guard, как
// sources/notes.
func registerEventTools(s *server.MCPServer, events EventService) {
	tool := mcp.NewTool(
		"event_list",
		mcp.WithDescription("Список событий в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithString("person_id", mcp.Description("Необязательный фильтр: только события, где эта персона — участник")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, eventListHandler(events))

	tool = mcp.NewTool(
		"event_search",
		mcp.WithDescription("Поиск событий ТОЛЬКО по началу текста места (place) — единственное индексируемое поле события (type/date/участники поиском не охвачены); результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало текста места")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, eventSearchHandler(events))

	tool = mcp.NewTool(
		"event_get",
		mcp.WithDescription("Событие по id; результат — JSON записи. Неверный формат id или отсутствующая/приватная (для не-владельца) запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например E-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, eventGetHandler(events))

	tool = mcp.NewTool(
		"event_create",
		mcp.WithDescription("Создать событие; id генерируется сервером; результат — JSON созданной записи. Несуществующий participants[i].person_id — ошибка тула; place — мягкая ссылка, НИКОГДА не проверяется на существование"),
		mcp.WithString("type", mcp.Required(), mcp.Description("Вид события (открытый набор: birth/death/marriage/burial/confession/census и др.)")),
		mcp.WithObject("date", mcp.Description("Дата события"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("place", mcp.Description("Место (текст или ссылка на административное деление/церковь/приход; ref/type сохраняются, если переданы; существование НЕ проверяется)"), mcp.Properties(placeRefObjectProperties())),
		mcp.WithArray("participants", mcp.Items(map[string]any{
			"type":       "object",
			"properties": eventParticipantObjectProperties(),
			"required":   []string{"person_id", "role"},
		}), mcp.Description("Участники события (person_id проверяется на существование)")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, eventCreateHandler(events))

	tool = mcp.NewTool(
		"event_update",
		mcp.WithDescription("Изменить событие: type заменяется безусловно при каждом вызове; participants/sources/notes/private — при отсутствии аргумента сохраняют текущее значение, явное пустое значение/пустой список — очищает; date/place — одиночные объектные поля: отсутствие ключа сохраняет текущее значение, явный null — очищает (пустой объект {} для очистки не подходит — не проходит валидацию); результат — JSON обновлённой записи"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("type", mcp.Required(), mcp.Description("Вид события")),
		mcp.WithObject("date", mcp.Description("Дата события"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("place", mcp.Description("Место (текст или ссылка); существование НЕ проверяется"), mcp.Properties(placeRefObjectProperties())),
		mcp.WithArray("participants", mcp.Items(map[string]any{
			"type":       "object",
			"properties": eventParticipantObjectProperties(),
			"required":   []string{"person_id", "role"},
		}), mcp.Description("Участники события; при отсутствии в вызове текущий список сохраняется, пустой массив — очищает его")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, eventUpdateHandler(events))

	tool = mcp.NewTool(
		"event_delete",
		mcp.WithDescription("Удалить событие. Необратимо. Если на него есть строгие ссылки — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, eventDeleteHandler(events))
}

func eventListHandler(events EventService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.EventQuery{}

		if pid := req.GetString("person_id", ""); pid != "" {
			id := models.ID(pid)
			q.PersonID = &id
		}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := events.ListEvents(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.EventsFromModels(list))
	}
}

func eventSearchHandler(events EventService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := events.SearchEvents(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.EventsFromModels(list))
	}
}

func eventGetHandler(events EventService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		e, err := events.GetEvent(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.EventFromModel(e))
	}
}

func eventCreateHandler(events EventService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		date, err := optionalFactDate(args, "date")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		place, err := optionalPlaceRef(args, "place")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		participants, err := optionalEventParticipants(args, "participants")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sources, err := optionalSourceLinks(args, "sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		e := models.Event{
			Type:         models.EventType(req.GetString("type", "")),
			Date:         date,
			Place:        place,
			Participants: participants,
			Sources:      sources,
			Notes:        textRefsFromStrings(req.GetStringSlice("notes", nil)),
			Private:      req.GetBool("private", false),
		}

		created, err := events.CreateEvent(ctx, e)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.EventFromModel(created))
	}
}

func eventUpdateHandler(events EventService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := events.GetEvent(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		args := req.GetArguments()

		cur.Type = models.EventType(req.GetString("type", ""))

		if _, ok := args["date"]; ok {
			date, err := optionalFactDate(args, "date")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Date = date
		}

		if _, ok := args["place"]; ok {
			place, err := optionalPlaceRef(args, "place")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Place = place
		}

		if raw, ok := args["participants"]; ok && raw != nil {
			participants, err := optionalEventParticipants(args, "participants")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Participants = participants
		}

		if raw, ok := args["sources"]; ok && raw != nil {
			sources, err := optionalSourceLinks(args, "sources")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Sources = sources
		}

		if raw, ok := args["notes"]; ok && raw != nil {
			cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))
		}

		if raw, ok := args["private"]; ok && raw != nil {
			cur.Private = req.GetBool("private", false)
		}

		if err := events.UpdateEvent(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.EventFromModel(cur))
	}
}

func eventDeleteHandler(events EventService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := events.DeleteEvent(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
```

#### `internal/mcp/event_test.go` (создать)

```go
package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakeEvents struct {
	list []models.Event
	err  error

	gotQuery  models.EventQuery
	gotSearch models.SearchQuery
	search    []models.Event

	getE      models.Event
	created   models.Event
	gotCreate models.Event
	updated   models.Event
	gotIDs    []models.ID
	deleteErr error
}

func (f *fakeEvents) ListEvents(_ context.Context, _ models.Access, q models.EventQuery) ([]models.Event, error) {
	f.gotQuery = q

	return f.list, f.err
}

func (f *fakeEvents) SearchEvents(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Event, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeEvents) GetEvent(_ context.Context, _ models.Access, id models.ID) (models.Event, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Event{}, f.err
	}

	return f.getE, nil
}

func (f *fakeEvents) CreateEvent(_ context.Context, e models.Event) (models.Event, error) {
	f.gotCreate = e
	if f.err != nil {
		return models.Event{}, f.err
	}

	return f.created, nil
}

func (f *fakeEvents) UpdateEvent(_ context.Context, e models.Event) error {
	f.updated = e

	return f.err
}

func (f *fakeEvents) DeleteEvent(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callEventTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestEventListToolPassesPersonIDFilter(t *testing.T) {
	svc := &fakeEvents{}

	res := callEventTool(t, eventListHandler(svc), map[string]any{"person_id": "I-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotQuery.PersonID == nil || *svc.gotQuery.PersonID != "I-1" {
		t.Fatalf("gotQuery.PersonID = %v", svc.gotQuery.PersonID)
	}
}

func TestEventSearchToolPassesQuery(t *testing.T) {
	svc := &fakeEvents{}

	res := callEventTool(t, eventSearchHandler(svc), map[string]any{"q": "село"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotSearch.Text != "село" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}

func TestEventGetToolContract(t *testing.T) {
	svc := &fakeEvents{getE: models.Event{ID: "E-1", Type: models.EventTypeBirth}}

	res := callEventTool(t, eventGetHandler(svc), map[string]any{"id": "E-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "E-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

func TestEventCreateToolPassesFields(t *testing.T) {
	svc := &fakeEvents{created: models.Event{ID: "E-new", Type: models.EventTypeBirth}}

	res := callEventTool(t, eventCreateHandler(svc), map[string]any{
		"type": "birth",
		"place": map[string]any{
			"text": "село Давыдово", "ref": "AD-01ARZ3NDEKTSV4RRFFQ69G5FA9", "type": "administrative_division",
		},
		"participants": []any{
			map[string]any{"person_id": "I-1", "role": "родитель"},
		},
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.Type != models.EventTypeBirth {
		t.Fatalf("gotCreate.Type = %q", svc.gotCreate.Type)
	}
	if svc.gotCreate.Place == nil || svc.gotCreate.Place.Ref != "AD-01ARZ3NDEKTSV4RRFFQ69G5FA9" {
		t.Fatalf("gotCreate.Place = %+v", svc.gotCreate.Place)
	}
	if len(svc.gotCreate.Participants) != 1 || svc.gotCreate.Participants[0].PersonID != "I-1" {
		t.Fatalf("gotCreate.Participants = %+v", svc.gotCreate.Participants)
	}
}

// TestEventUpdateToolTypeAlwaysReplaced: type — REQUIRED, всегда заменяется
// безусловно (как relation_update's kind).
func TestEventUpdateToolTypeAlwaysReplaced(t *testing.T) {
	svc := &fakeEvents{getE: models.Event{ID: "E-1", Type: models.EventTypeBirth}}

	res := callEventTool(t, eventUpdateHandler(svc), map[string]any{"id": "E-1", "type": "death"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Type != models.EventTypeDeath {
		t.Fatalf("updated.Type = %q", svc.updated.Type)
	}
}

// TestEventUpdateToolOmittedPlaceKeepsCurrent: Place — одиночное объектное
// поле (*PlaceRef), самое рискованное в этом подпроекте по истории
// регрессии (fix "null должен снова очищать TextRef/FactDate-поля"):
// отсутствие ключа "place" в вызове сохраняет текущее значение.
func TestEventUpdateToolOmittedPlaceKeepsCurrent(t *testing.T) {
	svc := &fakeEvents{getE: models.Event{
		ID: "E-1", Type: models.EventTypeBirth,
		Place: &models.PlaceRef{Text: "село Давыдово", Ref: "AD-1", Type: models.TypeAdministrativeDivision},
	}}

	res := callEventTool(t, eventUpdateHandler(svc), map[string]any{"id": "E-1", "type": "birth"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Place == nil || svc.updated.Place.Ref != "AD-1" || svc.updated.Place.Text != "село Давыдово" {
		t.Fatalf("updated.Place = %+v, ожидалось сохранение текущего места (omit keeps)", svc.updated.Place)
	}
}

// TestEventUpdateToolNullPlaceClears: явный null у place ОЧИЩАЕТ поле — это
// единственный способ очистить одиночное объектное поле ({} не проходит
// валидацию PlaceRef). Присутствие ключа с raw==nil ("place": nil) должно
// сработать так же, как отсутствие ключа НЕ должно (presence-only guard,
// не raw != nil).
func TestEventUpdateToolNullPlaceClears(t *testing.T) {
	svc := &fakeEvents{getE: models.Event{
		ID: "E-1", Type: models.EventTypeBirth,
		Place: &models.PlaceRef{Text: "село Давыдово", Ref: "AD-1", Type: models.TypeAdministrativeDivision},
	}}

	res := callEventTool(t, eventUpdateHandler(svc), map[string]any{"id": "E-1", "type": "birth", "place": nil})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Place != nil {
		t.Fatalf("updated.Place = %+v, ожидался nil (явный null очищает поле)", svc.updated.Place)
	}
}

// TestEventUpdateToolOmittedParticipantsKeepsCurrent: participants —
// array-of-objects, обычный preserve-on-omit guard (ok && raw != nil), как
// person_update's names.
func TestEventUpdateToolOmittedParticipantsKeepsCurrent(t *testing.T) {
	svc := &fakeEvents{getE: models.Event{
		ID: "E-1", Type: models.EventTypeBirth,
		Participants: []models.EventParticipant{{PersonID: "I-1", Role: "родитель"}},
	}}

	res := callEventTool(t, eventUpdateHandler(svc), map[string]any{"id": "E-1", "type": "birth"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if len(svc.updated.Participants) != 1 || svc.updated.Participants[0].PersonID != "I-1" {
		t.Fatalf("updated.Participants = %+v, ожидалось сохранение текущих участников", svc.updated.Participants)
	}
}

// TestEventUpdateToolEmptyParticipantsClears: явный пустой массив
// очищает список участников.
func TestEventUpdateToolEmptyParticipantsClears(t *testing.T) {
	svc := &fakeEvents{getE: models.Event{
		ID: "E-1", Type: models.EventTypeBirth,
		Participants: []models.EventParticipant{{PersonID: "I-1", Role: "родитель"}},
	}}

	res := callEventTool(t, eventUpdateHandler(svc), map[string]any{
		"id": "E-1", "type": "birth", "participants": []any{},
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if len(svc.updated.Participants) != 0 {
		t.Fatalf("updated.Participants = %+v, ожидался пустой список", svc.updated.Participants)
	}
}

func TestEventUpdateToolOmittedNotesKeepsCurrent(t *testing.T) {
	svc := &fakeEvents{getE: models.Event{ID: "E-1", Type: models.EventTypeBirth, Notes: []models.TextRef{{Text: "заметка"}}}}

	res := callEventTool(t, eventUpdateHandler(svc), map[string]any{"id": "E-1", "type": "birth"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if len(svc.updated.Notes) != 1 || svc.updated.Notes[0].Text != "заметка" {
		t.Fatalf("updated.Notes = %+v, ожидалось сохранение текущих заметок", svc.updated.Notes)
	}
}

func TestEventDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeEvents{deleteErr: &models.InUseError{Type: models.TypeEvent, ID: "E-1"}}

	res := callEventTool(t, eventDeleteHandler(svc), map[string]any{"id": "E-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

// TestNewServerRegistersEventTools: 6 тулов, включая event_search (в
// отличие от relation/residence).
func TestNewServerRegistersEventTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Events: &fakeEvents{}}).ListTools()

	for _, name := range []string{"event_list", "event_search", "event_get", "event_create", "event_update", "event_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
```

### 1.6. `internal/mcp/object_args.go` — добавить `PlaceRef`/`EventParticipant`-хелперы

Ровно четыре новые функции, добавленные в конец уже существующего файла (хелперы подпроектов 3, 5, 8 переносятся как есть): `placeRefObjectProperties()`/`optionalPlaceRef()` — структурно идентичны `textRefObjectProperties()`/`optionalTextRef()`, но собственная функция, потому что `transport.PlaceRef` — отдельный транспортный тип (`models.PlaceRef` отдельный тип модели, хотя JSON-форма та же `{text, ref?, type?}`); `eventParticipantObjectProperties()`/`optionalEventParticipants()` — флат-объект `{person_id, role, note?}`, ПРОЩЕ `sourceLinkObjectProperties`/`personNameObjectProperties` (никаких вложенных объектов внутри элемента), по тому же техническому приёму `mcp.Items`+marshal/unmarshal сырого `[]any` в `[]transport.EventParticipant`, что и `optionalSourceLinks`/`optionalPersonNames`.

#### `internal/mcp/object_args.go` (изменить — итоговое содержимое)

```go
package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// textRefObjectProperties — JSON-schema свойств объектного аргумента вида
// TextRef ({text, ref?, type?}) — используется в mcp.WithObject для полей
// вроде Church.Parish/Parish.Church. Появляется впервые в этом проходе
// (первые сущности с одиночным *TextRef, не списком). ref/type round-trip'ятся
// как есть (см. transport.TextRef.Model): клиент, уже получивший ref/type
// через church_get/parish_get, сохранит ссылку, отправив их обратно
// неизменными в church_update/parish_update; если их не передать вовсе или
// изменить только text — ссылка будет потеряна или расходиться с ним.
func textRefObjectProperties() map[string]any {
	return map[string]any{
		"text": map[string]any{"type": "string", "description": "Текст (обязателен, если нет ссылки)"},
		"ref":  map[string]any{"type": "string", "description": "id сущности-ссылки; сохраняется, если передать его обратно неизменным (например, из предыдущего *_get)"},
		"type": map[string]any{"type": "string", "description": "тип сущности-ссылки; сохраняется вместе с ref при неизменной передаче"},
	}
}

// factDateObjectProperties — JSON-schema свойств объектного аргумента вида
// FactDate (структурированная дата с точностью). Появляется впервые в этом
// проходе.
func factDateObjectProperties() map[string]any {
	return map[string]any{
		"year":      map[string]any{"type": "integer", "description": "Год (1-9999), обязателен, если precision не unknown"},
		"month":     map[string]any{"type": "integer", "description": "Месяц (1-12), нужен при precision=month/day"},
		"day":       map[string]any{"type": "integer", "description": "День, нужен при precision=day"},
		"precision": map[string]any{"type": "string", "enum": []string{"unknown", "year", "month", "day"}, "description": "Верхняя известная точность"},
		"modifier":  map[string]any{"type": "string", "enum": []string{"exact", "approx", "before", "after", "between"}, "description": "Формулировка: точно/около/до/после/между"},
		"calendar":  map[string]any{"type": "string", "enum": []string{"", "gregorian", "julian", "unknown"}, "description": "Календарь; пусто — не указан"},
		"year_to":   map[string]any{"type": "integer", "description": "Год верхней границы, только при modifier=between"},
		"month_to":  map[string]any{"type": "integer", "description": "Месяц верхней границы"},
		"day_to":    map[string]any{"type": "integer", "description": "День верхней границы"},
	}
}

// anchorObjectProperties — JSON-schema свойств объектного аргумента вида
// Anchor (полиморфная привязка «где именно» у Citation, см.
// transport.Anchor): плоский объект с дискриминатором kind и полями всех
// трёх вариантов вместе (по образцу FactDate), а не вложенный union — проще
// для MCP-клиента, чем oneOf. Первый полиморфный тип в программе.
func anchorObjectProperties() map[string]any {
	return map[string]any{
		"kind":          map[string]any{"type": "string", "enum": []string{"archive", "file", "url"}, "description": "Вид привязки; пустой объект или отсутствие аргумента — без привязки"},
		"node_id":       map[string]any{"type": "string", "description": "id архивного узла (kind=archive, обязателен для этого вида)"},
		"document_id":   map[string]any{"type": "string", "description": "id архивного документа (kind=archive, необязательно)"},
		"page":          map[string]any{"type": "integer", "description": "Номер страницы/скана (kind=archive, обязателен, не меньше 1)"},
		"rect":          map[string]any{"type": "string", "description": "Координаты области выделения на изображении (kind=archive, необязательно)"},
		"attachment_id": map[string]any{"type": "string", "description": "id вложения (kind=file, обязателен для этого вида)"},
		"timecode":      map[string]any{"type": "string", "description": "Тайм-метка для аудио/видео (kind=file, необязательно)"},
		"url":           map[string]any{"type": "string", "description": "Абсолютный http(s)-адрес (kind=url, обязателен для этого вида)"},
	}
}

// optionalAnchor читает необязательный объектный аргумент вида Anchor (см.
// anchorObjectProperties) из сырых аргументов тула и конвертирует его в
// модель; отсутствующий, null или пустой (kind не задан/не распознан)
// аргумент — nil, без ошибки (см. (*transport.Anchor).Model()).
func optionalAnchor(args map[string]any, name string) (models.Anchor, error) {
	raw, ok := args[name]
	if !ok || raw == nil {
		return nil, nil
	}

	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	var a transport.Anchor
	if err := json.Unmarshal(b, &a); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	return a.Model(), nil
}

// sourceLinkObjectProperties — JSON-schema свойств одного элемента массива
// sources (доказательство, см. transport.SourceLink). target_type/target_id
// сюда не входят — клиент их не отправляет, владелец подставляется сервером
// из контекста вызова (см. transport.SourceLink.Model()).
func sourceLinkObjectProperties() map[string]any {
	return map[string]any{
		"citation_id": map[string]any{"type": "string", "description": "id цитаты (обязателен)"},
		"reliability": map[string]any{"type": "string", "enum": []string{"primary", "contemporary", "memory", "indirect", "unknown"}, "description": "Достоверность именно этого утверждения по этой цитате"},
		"role":        map[string]any{"type": "string", "description": "Роль утверждения"},
		"note":        map[string]any{"type": "string", "description": "Заметка"},
	}
}

// optionalSourceLinks читает массив объектов вида SourceLink (см.
// sourceLinkObjectProperties) из сырых аргументов тула и конвертирует его в
// модели; отсутствующий или null аргумент — пустой срез, без ошибки (та же
// механика, что и textRefsFromStrings для списков TextRef).
func optionalSourceLinks(args map[string]any, name string) ([]models.SourceLink, error) {
	raw, ok := args[name]
	if !ok || raw == nil {
		return nil, nil
	}

	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	var links []transport.SourceLink
	if err := json.Unmarshal(b, &links); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	return transport.SourceLinksToModel(links), nil
}

// personNameObjectProperties — JSON-schema свойств одного элемента массива
// names (одно имя персоны, см. transport.PersonName). Первый массив объектов
// в программе, чьи собственные свойства тоже вложенные объекты
// (surname/given/patronymic — TextRef, см. textRefObjectProperties; since/
// until — FactDate, см. factDateObjectProperties) — то же техническое
// решение (mcp.Items с произвольной JSON-schema), что и sourceLinkObjectProperties
// (подпроект 5), только с более сложным элементом.
func personNameObjectProperties() map[string]any {
	return map[string]any{
		"type":       map[string]any{"type": "string", "enum": []string{"", "main", "birth", "married", "changed", "pseudonym"}, "description": "Вид имени; пусто — не указан"},
		"surname":    map[string]any{"type": "object", "properties": textRefObjectProperties(), "description": "Фамилия (текст или мягкая ссылка на словарь фамилий; существование ссылки не проверяется)"},
		"given":      map[string]any{"type": "object", "properties": textRefObjectProperties(), "description": "Имя (текст или мягкая ссылка на словарь личных имён; существование ссылки не проверяется)"},
		"patronymic": map[string]any{"type": "object", "properties": textRefObjectProperties(), "description": "Отчество (текст или мягкая ссылка на словарь отчеств; существование ссылки не проверяется)"},
		"prefix":     map[string]any{"type": "string", "description": "Служебная приставка (фон, де, ван…)"},
		"suffix":     map[string]any{"type": "string", "description": "Служебное окончание (ст., мл.…)"},
		"since":      map[string]any{"type": "object", "properties": factDateObjectProperties(), "description": "Начало периода действия этого имени"},
		"until":      map[string]any{"type": "object", "properties": factDateObjectProperties(), "description": "Конец периода действия этого имени"},
	}
}

// optionalPersonNames читает массив объектов вида PersonName (см.
// personNameObjectProperties) из сырых аргументов тула и конвертирует его в
// модели; отсутствующий или null аргумент — nil, без ошибки (та же
// механика, что и optionalSourceLinks).
func optionalPersonNames(args map[string]any, name string) ([]models.PersonName, error) {
	raw, ok := args[name]
	if !ok || raw == nil {
		return nil, nil
	}

	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	var names []transport.PersonName
	if err := json.Unmarshal(b, &names); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	return transport.PersonNamesToModel(names), nil
}

// optionalTextRef читает необязательный объектный аргумент {text, ref?, type?}
// (см. transport.TextRef) из сырых аргументов тула и конвертирует его в
// модель; отсутствующий или null аргумент — nil, без ошибки.
func optionalTextRef(args map[string]any, name string) (*models.TextRef, error) {
	raw, ok := args[name]
	if !ok || raw == nil {
		return nil, nil
	}

	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	var t transport.TextRef
	if err := json.Unmarshal(b, &t); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	m := t.Model()

	return &m, nil
}

// placeRefObjectProperties — JSON-schema свойств объектного аргумента вида
// PlaceRef ({text, ref?, type?}) — используется в mcp.WithObject для
// Event.Place. Структурно идентична textRefObjectProperties (см.
// models.PlaceRef vs models.TextRef), но это отдельный транспортный тип
// (transport.PlaceRef), поэтому заводится собственная функция, а не
// переиспользуется textRefObjectProperties.
func placeRefObjectProperties() map[string]any {
	return map[string]any{
		"text": map[string]any{"type": "string", "description": "Текст (обязателен, если нет ссылки)"},
		"ref":  map[string]any{"type": "string", "description": "id сущности-места (административное деление, церковь или приход); сохраняется, если передать его обратно неизменным (например, из предыдущего *_get); НЕ проверяется на существование — мягкая ссылка"},
		"type": map[string]any{"type": "string", "description": "тип сущности-ссылки (admin_division/church/parish); сохраняется вместе с ref при неизменной передаче"},
	}
}

// optionalPlaceRef читает необязательный объектный аргумент {text, ref?,
// type?} (см. transport.PlaceRef) из сырых аргументов тула и конвертирует
// его в модель; отсутствующий или null аргумент — nil, без ошибки.
func optionalPlaceRef(args map[string]any, name string) (*models.PlaceRef, error) {
	raw, ok := args[name]
	if !ok || raw == nil {
		return nil, nil
	}

	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	var p transport.PlaceRef
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	return p.Model(), nil
}

// eventParticipantObjectProperties — JSON-schema свойств одного элемента
// массива participants (участник события, см. transport.EventParticipant).
// Флат-объект без вложенных объектов (проще sourceLinkObjectProperties/
// personNameObjectProperties — здесь всего три скаляра); person_id — первая
// СТРОГАЯ (проверяемая на существование) ссылка внутри массива-объектов
// MCP-аргумента в программе (docs/data-model/entity-write.md §3.8).
func eventParticipantObjectProperties() map[string]any {
	return map[string]any{
		"person_id": map[string]any{"type": "string", "description": "id персоны-участника (обязателен, проверяется на существование)"},
		"role":      map[string]any{"type": "string", "description": "Роль в событии (обязательна, непустая)"},
		"note":      map[string]any{"type": "string", "description": "Заметка"},
	}
}

// optionalEventParticipants читает массив объектов вида EventParticipant (см.
// eventParticipantObjectProperties) из сырых аргументов тула и конвертирует
// его в модели; отсутствующий или null аргумент — nil, без ошибки (та же
// механика, что и optionalSourceLinks/optionalPersonNames).
func optionalEventParticipants(args map[string]any, name string) ([]models.EventParticipant, error) {
	raw, ok := args[name]
	if !ok || raw == nil {
		return nil, nil
	}

	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	var ps []transport.EventParticipant
	if err := json.Unmarshal(b, &ps); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	return transport.EventParticipantsToModel(ps), nil
}

// optionalFactDate читает необязательный объектный аргумент (структурированная
// дата, см. transport.FactDate) из сырых аргументов тула и конвертирует его в
// модель; отсутствующий или null аргумент — nil, без ошибки.
func optionalFactDate(args map[string]any, name string) (*models.FactDate, error) {
	raw, ok := args[name]
	if !ok || raw == nil {
		return nil, nil
	}

	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	var d transport.FactDate
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	return d.Model(), nil
}
```

### 1.7. `Deps`-реестр: подключить Relation/Residence/Event

Механическое добавление трёх сервисов (`Relations`/`RelationService`, `Residences`/`ResidenceService`, `Events`/`EventService`) в каждый из шести файлов — по образцу подключения `People`/`PersonService` в подпроекте 8 (поле размещается в конце каждой структуры `Deps`, после `People`, по хронологическому порядку подпроектов, не по алфавиту). `internal/app/app.go` — дополнительно фасады `relationService`/`residenceService`/`eventService` (обёртки над указателями на `Scenario` каждого usecase-пакета, зеркалируют `personService`), интерфейс-ассерции (`var _ httpapi.RelationService = (*relationService)(nil)` и т.п.), конструирование в `New()` и подключение к обоим `mcp.Deps`/`httpapi.Deps`.

#### `internal/httpapi/deps.go` (изменить — итоговое содержимое)

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

// FamilyService — контракт сценариев родов/линий, отдаваемых в HTTP: список,
// поиск, чтение, создание, изменение, удаление.
type FamilyService interface {
	ListFamilies(ctx context.Context, access models.Access, page models.Page) ([]models.Family, error)
	SearchFamilies(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Family, error)
	GetFamily(ctx context.Context, access models.Access, id models.ID) (models.Family, error)
	CreateFamily(ctx context.Context, f models.Family) (models.Family, error)
	UpdateFamily(ctx context.Context, f models.Family) error
	DeleteFamily(ctx context.Context, id models.ID) error
}

// PersonService — контракт сценариев персон, отдаваемых в HTTP: список,
// поиск, чтение, создание, изменение, удаление. Список/поиск используют
// имена ListPeople/SearchPeople (неправильное множественное число,
// зеркалирует store.Store.ListPeople) — единственное исключение из общего
// правила «<Глагол><ИмяСущностиВоМножественномЧисле>» в этой программе.
type PersonService interface {
	ListPeople(ctx context.Context, access models.Access, page models.Page) ([]models.Person, error)
	SearchPeople(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Person, error)
	GetPerson(ctx context.Context, access models.Access, id models.ID) (models.Person, error)
	CreatePerson(ctx context.Context, p models.Person) (models.Person, error)
	UpdatePerson(ctx context.Context, p models.Person) error
	DeletePerson(ctx context.Context, id models.ID) error
}

// RelationService — контракт сценариев рёбер графа родства, отдаваемых в
// HTTP: список (необязательный фильтр по персоне, см. models.RelationQuery),
// чтение, создание, изменение, удаление. Без поиска — search_relations
// намеренно не заводится, см. docs/data-model/entity-write.md §3.8.
type RelationService interface {
	ListRelations(ctx context.Context, access models.Access, q models.RelationQuery) ([]models.Relation, error)
	GetRelation(ctx context.Context, access models.Access, id models.ID) (models.Relation, error)
	CreateRelation(ctx context.Context, r models.Relation) (models.Relation, error)
	UpdateRelation(ctx context.Context, r models.Relation) error
	DeleteRelation(ctx context.Context, id models.ID) error
}

// ResidenceService — контракт сценариев проживаний, отдаваемых в HTTP:
// список (необязательные фильтры по персоне и месту, см.
// models.ResidenceQuery), чтение, создание, изменение, удаление. Без поиска —
// search_residences намеренно не заводится, см.
// docs/data-model/entity-write.md §3.8.
type ResidenceService interface {
	ListResidences(ctx context.Context, access models.Access, q models.ResidenceQuery) ([]models.Residence, error)
	GetResidence(ctx context.Context, access models.Access, id models.ID) (models.Residence, error)
	CreateResidence(ctx context.Context, r models.Residence) (models.Residence, error)
	UpdateResidence(ctx context.Context, r models.Residence) error
	DeleteResidence(ctx context.Context, id models.ID) error
}

// EventService — контракт сценариев событий, отдаваемых в HTTP: список
// (необязательный фильтр по участнику, см. models.EventQuery), поиск (по
// началу текста места), чтение, создание, изменение, удаление.
type EventService interface {
	ListEvents(ctx context.Context, access models.Access, q models.EventQuery) ([]models.Event, error)
	SearchEvents(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Event, error)
	GetEvent(ctx context.Context, access models.Access, id models.ID) (models.Event, error)
	CreateEvent(ctx context.Context, e models.Event) (models.Event, error)
	UpdateEvent(ctx context.Context, e models.Event) error
	DeleteEvent(ctx context.Context, id models.ID) error
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
	Families     FamilyService
	People       PersonService
	Relations    RelationService
	Residences   ResidenceService
	Events       EventService
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

	if deps.Families != nil {
		registerFamilyRoutes(mux, deps.Families)
	}

	if deps.People != nil {
		registerPersonRoutes(mux, deps.People)
	}

	if deps.Relations != nil {
		registerRelationRoutes(mux, deps.Relations)
	}

	if deps.Residences != nil {
		registerResidenceRoutes(mux, deps.Residences)
	}

	if deps.Events != nil {
		registerEventRoutes(mux, deps.Events)
	}

	registerAuthRoutes(mux, deps.Auth, deps.TrustProxy)

	return requireCSRFHeader(resolveAccess(deps.Auth)(mux))
}
```

#### `internal/httpapi/httpapi.go` (изменить — итоговое содержимое)

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

	if deps.Families != nil {
		registerFamilyRoutes(mux, deps.Families)
	}

	if deps.People != nil {
		registerPersonRoutes(mux, deps.People)
	}

	if deps.Relations != nil {
		registerRelationRoutes(mux, deps.Relations)
	}

	if deps.Residences != nil {
		registerResidenceRoutes(mux, deps.Residences)
	}

	if deps.Events != nil {
		registerEventRoutes(mux, deps.Events)
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

// registerFamilyRoutes регистрирует маршруты /api/families на переданном mux.
func registerFamilyRoutes(mux *http.ServeMux, families FamilyService) {
	mux.HandleFunc("GET /api/families", handleFamilyList(families))
	mux.HandleFunc("GET /api/families/search", handleFamilySearch(families))
	mux.HandleFunc("GET /api/families/{id}", handleFamilyGet(families))
	mux.HandleFunc("POST /api/families", handleFamilyCreate(families))
	mux.HandleFunc("PUT /api/families/{id}", handleFamilyUpdate(families))
	mux.HandleFunc("DELETE /api/families/{id}", handleFamilyDelete(families))
}

// registerPersonRoutes регистрирует маршруты /api/people на переданном mux.
func registerPersonRoutes(mux *http.ServeMux, people PersonService) {
	mux.HandleFunc("GET /api/people", handlePersonList(people))
	mux.HandleFunc("GET /api/people/search", handlePersonSearch(people))
	mux.HandleFunc("GET /api/people/{id}", handlePersonGet(people))
	mux.HandleFunc("POST /api/people", handlePersonCreate(people))
	mux.HandleFunc("PUT /api/people/{id}", handlePersonUpdate(people))
	mux.HandleFunc("DELETE /api/people/{id}", handlePersonDelete(people))
}

// registerRelationRoutes регистрирует маршруты /api/relations на переданном
// mux. Без /search — search_relations намеренно не заводится (индекс пуст,
// см. docs/data-model/entity-write.md §3.8); список принимает необязательный
// ?person_id=.
func registerRelationRoutes(mux *http.ServeMux, relations RelationService) {
	mux.HandleFunc("GET /api/relations", handleRelationList(relations))
	mux.HandleFunc("GET /api/relations/{id}", handleRelationGet(relations))
	mux.HandleFunc("POST /api/relations", handleRelationCreate(relations))
	mux.HandleFunc("PUT /api/relations/{id}", handleRelationUpdate(relations))
	mux.HandleFunc("DELETE /api/relations/{id}", handleRelationDelete(relations))
}

// registerResidenceRoutes регистрирует маршруты /api/residences на
// переданном mux. Без /search (см. registerRelationRoutes); список
// принимает необязательные ?person_id=/?place_id=.
func registerResidenceRoutes(mux *http.ServeMux, residences ResidenceService) {
	mux.HandleFunc("GET /api/residences", handleResidenceList(residences))
	mux.HandleFunc("GET /api/residences/{id}", handleResidenceGet(residences))
	mux.HandleFunc("POST /api/residences", handleResidenceCreate(residences))
	mux.HandleFunc("PUT /api/residences/{id}", handleResidenceUpdate(residences))
	mux.HandleFunc("DELETE /api/residences/{id}", handleResidenceDelete(residences))
}

// registerEventRoutes регистрирует маршруты /api/events на переданном mux.
// Event ЕСТЬ поисковый индекс (по началу текста места) — полный набор из 6
// маршрутов, как у большинства сущностей; список принимает необязательный
// ?person_id=.
func registerEventRoutes(mux *http.ServeMux, events EventService) {
	mux.HandleFunc("GET /api/events", handleEventList(events))
	mux.HandleFunc("GET /api/events/search", handleEventSearch(events))
	mux.HandleFunc("GET /api/events/{id}", handleEventGet(events))
	mux.HandleFunc("POST /api/events", handleEventCreate(events))
	mux.HandleFunc("PUT /api/events/{id}", handleEventUpdate(events))
	mux.HandleFunc("DELETE /api/events/{id}", handleEventDelete(events))
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

// FamilyService — контракт сценариев родов/линий, отдаваемых в MCP-тулы:
// список, поиск, чтение, создание, изменение, удаление.
type FamilyService interface {
	ListFamilies(ctx context.Context, access models.Access, page models.Page) ([]models.Family, error)
	SearchFamilies(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Family, error)
	GetFamily(ctx context.Context, access models.Access, id models.ID) (models.Family, error)
	CreateFamily(ctx context.Context, f models.Family) (models.Family, error)
	UpdateFamily(ctx context.Context, f models.Family) error
	DeleteFamily(ctx context.Context, id models.ID) error
}

// PersonService — контракт сценариев персон, отдаваемых в MCP-тулы: список,
// поиск, чтение, создание, изменение, удаление. Список/поиск используют
// имена ListPeople/SearchPeople (неправильное множественное число,
// зеркалирует store.Store.ListPeople) — единственное исключение из общего
// правила «<Глагол><ИмяСущностиВоМножественномЧисле>» в этой программе; сами
// MCP-тулы всё равно называются person_list/person_search — единым
// префиксом в единственном числе, как и у всех остальных сущностей.
type PersonService interface {
	ListPeople(ctx context.Context, access models.Access, page models.Page) ([]models.Person, error)
	SearchPeople(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Person, error)
	GetPerson(ctx context.Context, access models.Access, id models.ID) (models.Person, error)
	CreatePerson(ctx context.Context, p models.Person) (models.Person, error)
	UpdatePerson(ctx context.Context, p models.Person) error
	DeletePerson(ctx context.Context, id models.ID) error
}

// RelationService — контракт сценариев рёбер графа родства, отдаваемых в
// MCP-тулы: список (необязательный фильтр по персоне), чтение, создание,
// изменение, удаление. Без поиска — search_relations намеренно не заводится
// (индекс пуст, docs/data-model/entity-write.md §3.8).
type RelationService interface {
	ListRelations(ctx context.Context, access models.Access, q models.RelationQuery) ([]models.Relation, error)
	GetRelation(ctx context.Context, access models.Access, id models.ID) (models.Relation, error)
	CreateRelation(ctx context.Context, r models.Relation) (models.Relation, error)
	UpdateRelation(ctx context.Context, r models.Relation) error
	DeleteRelation(ctx context.Context, id models.ID) error
}

// ResidenceService — контракт сценариев проживаний, отдаваемых в MCP-тулы:
// список (необязательные фильтры по персоне и месту), чтение, создание,
// изменение, удаление. Без поиска — search_residences намеренно не
// заводится (см. RelationService).
type ResidenceService interface {
	ListResidences(ctx context.Context, access models.Access, q models.ResidenceQuery) ([]models.Residence, error)
	GetResidence(ctx context.Context, access models.Access, id models.ID) (models.Residence, error)
	CreateResidence(ctx context.Context, r models.Residence) (models.Residence, error)
	UpdateResidence(ctx context.Context, r models.Residence) error
	DeleteResidence(ctx context.Context, id models.ID) error
}

// EventService — контракт сценариев событий, отдаваемых в MCP-тулы: список
// (необязательный фильтр по участнику), поиск (по началу текста места),
// чтение, создание, изменение, удаление.
type EventService interface {
	ListEvents(ctx context.Context, access models.Access, q models.EventQuery) ([]models.Event, error)
	SearchEvents(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Event, error)
	GetEvent(ctx context.Context, access models.Access, id models.ID) (models.Event, error)
	CreateEvent(ctx context.Context, e models.Event) (models.Event, error)
	UpdateEvent(ctx context.Context, e models.Event) error
	DeleteEvent(ctx context.Context, id models.ID) error
}
```

#### `internal/mcp/server.go` (изменить — итоговое содержимое)

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
	Families     FamilyService
	People       PersonService
	Relations    RelationService
	Residences   ResidenceService
	Events       EventService
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

	if deps.Families != nil {
		registerFamilyTools(s, deps.Families)
	}

	if deps.People != nil {
		registerPersonTools(s, deps.People)
	}

	if deps.Relations != nil {
		registerRelationTools(s, deps.Relations)
	}

	if deps.Residences != nil {
		registerResidenceTools(s, deps.Residences)
	}

	if deps.Events != nil {
		registerEventTools(s, deps.Events)
	}

	return s
}
```

#### `internal/app/app.go` (изменить — итоговое содержимое)

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
	create_event "github.com/amarin/genodex/internal/usecases/create_event"
	create_family "github.com/amarin/genodex/internal/usecases/create_family"
	create_given_name "github.com/amarin/genodex/internal/usecases/create_given_name"
	create_note "github.com/amarin/genodex/internal/usecases/create_note"
	create_parish "github.com/amarin/genodex/internal/usecases/create_parish"
	create_patronymic "github.com/amarin/genodex/internal/usecases/create_patronymic"
	create_person "github.com/amarin/genodex/internal/usecases/create_person"
	create_relation "github.com/amarin/genodex/internal/usecases/create_relation"
	create_repository "github.com/amarin/genodex/internal/usecases/create_repository"
	create_residence "github.com/amarin/genodex/internal/usecases/create_residence"
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
	delete_event "github.com/amarin/genodex/internal/usecases/delete_event"
	delete_family "github.com/amarin/genodex/internal/usecases/delete_family"
	delete_given_name "github.com/amarin/genodex/internal/usecases/delete_given_name"
	delete_note "github.com/amarin/genodex/internal/usecases/delete_note"
	delete_parish "github.com/amarin/genodex/internal/usecases/delete_parish"
	delete_patronymic "github.com/amarin/genodex/internal/usecases/delete_patronymic"
	delete_person "github.com/amarin/genodex/internal/usecases/delete_person"
	delete_relation "github.com/amarin/genodex/internal/usecases/delete_relation"
	delete_repository "github.com/amarin/genodex/internal/usecases/delete_repository"
	delete_residence "github.com/amarin/genodex/internal/usecases/delete_residence"
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
	get_event "github.com/amarin/genodex/internal/usecases/get_event"
	get_family "github.com/amarin/genodex/internal/usecases/get_family"
	get_given_name "github.com/amarin/genodex/internal/usecases/get_given_name"
	get_note "github.com/amarin/genodex/internal/usecases/get_note"
	get_parish "github.com/amarin/genodex/internal/usecases/get_parish"
	get_patronymic "github.com/amarin/genodex/internal/usecases/get_patronymic"
	get_person "github.com/amarin/genodex/internal/usecases/get_person"
	get_relation "github.com/amarin/genodex/internal/usecases/get_relation"
	get_repository "github.com/amarin/genodex/internal/usecases/get_repository"
	get_residence "github.com/amarin/genodex/internal/usecases/get_residence"
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
	list_events "github.com/amarin/genodex/internal/usecases/list_events"
	list_families "github.com/amarin/genodex/internal/usecases/list_families"
	list_given_names "github.com/amarin/genodex/internal/usecases/list_given_names"
	list_notes "github.com/amarin/genodex/internal/usecases/list_notes"
	list_parishes "github.com/amarin/genodex/internal/usecases/list_parishes"
	list_patronymics "github.com/amarin/genodex/internal/usecases/list_patronymics"
	list_people "github.com/amarin/genodex/internal/usecases/list_people"
	list_relations "github.com/amarin/genodex/internal/usecases/list_relations"
	list_repositories "github.com/amarin/genodex/internal/usecases/list_repositories"
	list_residences "github.com/amarin/genodex/internal/usecases/list_residences"
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
	search_events "github.com/amarin/genodex/internal/usecases/search_events"
	search_families "github.com/amarin/genodex/internal/usecases/search_families"
	search_given_names "github.com/amarin/genodex/internal/usecases/search_given_names"
	search_notes "github.com/amarin/genodex/internal/usecases/search_notes"
	search_parishes "github.com/amarin/genodex/internal/usecases/search_parishes"
	search_patronymics "github.com/amarin/genodex/internal/usecases/search_patronymics"
	search_people "github.com/amarin/genodex/internal/usecases/search_people"
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
	update_event "github.com/amarin/genodex/internal/usecases/update_event"
	update_family "github.com/amarin/genodex/internal/usecases/update_family"
	update_given_name "github.com/amarin/genodex/internal/usecases/update_given_name"
	update_note "github.com/amarin/genodex/internal/usecases/update_note"
	update_parish "github.com/amarin/genodex/internal/usecases/update_parish"
	update_patronymic "github.com/amarin/genodex/internal/usecases/update_patronymic"
	update_person "github.com/amarin/genodex/internal/usecases/update_person"
	update_relation "github.com/amarin/genodex/internal/usecases/update_relation"
	update_repository "github.com/amarin/genodex/internal/usecases/update_repository"
	update_residence "github.com/amarin/genodex/internal/usecases/update_residence"
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

// familyService — фасад всех сценариев родов/линий, отдаваемых HTTP и MCP.
type familyService struct {
	list   *list_families.Scenario
	search *search_families.Scenario
	get    *get_family.Scenario
	create *create_family.Scenario
	update *update_family.Scenario
	del    *delete_family.Scenario
}

func (s *familyService) ListFamilies(ctx context.Context, access models.Access, page models.Page) ([]models.Family, error) {
	return s.list.ListFamilies(ctx, access, page)
}

func (s *familyService) SearchFamilies(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Family, error) {
	return s.search.SearchFamilies(ctx, access, q)
}

func (s *familyService) GetFamily(ctx context.Context, access models.Access, id models.ID) (models.Family, error) {
	return s.get.GetFamily(ctx, access, id)
}

func (s *familyService) CreateFamily(ctx context.Context, f models.Family) (models.Family, error) {
	return s.create.CreateFamily(ctx, f)
}

func (s *familyService) UpdateFamily(ctx context.Context, f models.Family) error {
	return s.update.UpdateFamily(ctx, f)
}

func (s *familyService) DeleteFamily(ctx context.Context, id models.ID) error {
	return s.del.DeleteFamily(ctx, id)
}

// personService — фасад всех сценариев персон, отдаваемых HTTP и MCP.
// list/search используют имена ListPeople/SearchPeople (неправильное
// множественное число, зеркалирует store.Store.ListPeople) — см.
// httpapi.PersonService/mcp.PersonService.
type personService struct {
	list   *list_people.Scenario
	search *search_people.Scenario
	get    *get_person.Scenario
	create *create_person.Scenario
	update *update_person.Scenario
	del    *delete_person.Scenario
}

func (s *personService) ListPeople(ctx context.Context, access models.Access, page models.Page) ([]models.Person, error) {
	return s.list.ListPeople(ctx, access, page)
}

func (s *personService) SearchPeople(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Person, error) {
	return s.search.SearchPeople(ctx, access, q)
}

func (s *personService) GetPerson(ctx context.Context, access models.Access, id models.ID) (models.Person, error) {
	return s.get.GetPerson(ctx, access, id)
}

func (s *personService) CreatePerson(ctx context.Context, p models.Person) (models.Person, error) {
	return s.create.CreatePerson(ctx, p)
}

func (s *personService) UpdatePerson(ctx context.Context, p models.Person) error {
	return s.update.UpdatePerson(ctx, p)
}

func (s *personService) DeletePerson(ctx context.Context, id models.ID) error {
	return s.del.DeletePerson(ctx, id)
}

// relationService — фасад всех сценариев рёбер графа родства, отдаваемых
// HTTP и MCP. Без search (search_relations намеренно не заводится, см.
// docs/data-model/entity-write.md §3.8).
type relationService struct {
	list   *list_relations.Scenario
	get    *get_relation.Scenario
	create *create_relation.Scenario
	update *update_relation.Scenario
	del    *delete_relation.Scenario
}

func (s *relationService) ListRelations(ctx context.Context, access models.Access, q models.RelationQuery) ([]models.Relation, error) {
	return s.list.ListRelations(ctx, access, q)
}

func (s *relationService) GetRelation(ctx context.Context, access models.Access, id models.ID) (models.Relation, error) {
	return s.get.GetRelation(ctx, access, id)
}

func (s *relationService) CreateRelation(ctx context.Context, r models.Relation) (models.Relation, error) {
	return s.create.CreateRelation(ctx, r)
}

func (s *relationService) UpdateRelation(ctx context.Context, r models.Relation) error {
	return s.update.UpdateRelation(ctx, r)
}

func (s *relationService) DeleteRelation(ctx context.Context, id models.ID) error {
	return s.del.DeleteRelation(ctx, id)
}

// residenceService — фасад всех сценариев проживаний, отдаваемых HTTP и MCP.
// Без search (см. relationService).
type residenceService struct {
	list   *list_residences.Scenario
	get    *get_residence.Scenario
	create *create_residence.Scenario
	update *update_residence.Scenario
	del    *delete_residence.Scenario
}

func (s *residenceService) ListResidences(ctx context.Context, access models.Access, q models.ResidenceQuery) ([]models.Residence, error) {
	return s.list.ListResidences(ctx, access, q)
}

func (s *residenceService) GetResidence(ctx context.Context, access models.Access, id models.ID) (models.Residence, error) {
	return s.get.GetResidence(ctx, access, id)
}

func (s *residenceService) CreateResidence(ctx context.Context, r models.Residence) (models.Residence, error) {
	return s.create.CreateResidence(ctx, r)
}

func (s *residenceService) UpdateResidence(ctx context.Context, r models.Residence) error {
	return s.update.UpdateResidence(ctx, r)
}

func (s *residenceService) DeleteResidence(ctx context.Context, id models.ID) error {
	return s.del.DeleteResidence(ctx, id)
}

// eventService — фасад всех сценариев событий, отдаваемых HTTP и MCP.
type eventService struct {
	list   *list_events.Scenario
	search *search_events.Scenario
	get    *get_event.Scenario
	create *create_event.Scenario
	update *update_event.Scenario
	del    *delete_event.Scenario
}

func (s *eventService) ListEvents(ctx context.Context, access models.Access, q models.EventQuery) ([]models.Event, error) {
	return s.list.ListEvents(ctx, access, q)
}

func (s *eventService) SearchEvents(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Event, error) {
	return s.search.SearchEvents(ctx, access, q)
}

func (s *eventService) GetEvent(ctx context.Context, access models.Access, id models.ID) (models.Event, error) {
	return s.get.GetEvent(ctx, access, id)
}

func (s *eventService) CreateEvent(ctx context.Context, e models.Event) (models.Event, error) {
	return s.create.CreateEvent(ctx, e)
}

func (s *eventService) UpdateEvent(ctx context.Context, e models.Event) error {
	return s.update.UpdateEvent(ctx, e)
}

func (s *eventService) DeleteEvent(ctx context.Context, id models.ID) error {
	return s.del.DeleteEvent(ctx, id)
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
	_ httpapi.FamilyService          = (*familyService)(nil)
	_ mcp.FamilyService              = (*familyService)(nil)
	_ httpapi.PersonService          = (*personService)(nil)
	_ mcp.PersonService              = (*personService)(nil)
	_ httpapi.RelationService        = (*relationService)(nil)
	_ mcp.RelationService            = (*relationService)(nil)
	_ httpapi.ResidenceService       = (*residenceService)(nil)
	_ mcp.ResidenceService           = (*residenceService)(nil)
	_ httpapi.EventService           = (*eventService)(nil)
	_ mcp.EventService               = (*eventService)(nil)
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

	families := &familyService{
		list:   list_families.New(st),
		search: search_families.New(st),
		get:    get_family.New(st),
		create: create_family.New(st, idgen.New()),
		update: update_family.New(st),
		del:    delete_family.New(st),
	}

	people := &personService{
		list:   list_people.New(st),
		search: search_people.New(st),
		get:    get_person.New(st),
		create: create_person.New(st, idgen.New()),
		update: update_person.New(st),
		del:    delete_person.New(st),
	}

	relations := &relationService{
		list:   list_relations.New(st),
		get:    get_relation.New(st),
		create: create_relation.New(st, idgen.New()),
		update: update_relation.New(st),
		del:    delete_relation.New(st),
	}

	residences := &residenceService{
		list:   list_residences.New(st),
		get:    get_residence.New(st),
		create: create_residence.New(st, idgen.New()),
		update: update_residence.New(st),
		del:    delete_residence.New(st),
	}

	events := &eventService{
		list:   list_events.New(st),
		search: search_events.New(st),
		get:    get_event.New(st),
		create: create_event.New(st, idgen.New()),
		update: update_event.New(st),
		del:    delete_event.New(st),
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
			Families:     families,
			People:       people,
			Relations:    relations,
			Residences:   residences,
			Events:       events,
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
		Families:     families,
		People:       people,
		Relations:    relations,
		Residences:   residences,
		Events:       events,
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

### 1.8. Real-store интеграционные тесты

`internal/httpapi/store_test.go` — добавляет `relationService`/`residenceService`/`eventService` (те же фасады, что `internal/app`, но собранные на реальном `sqlstore.Store` в тесте, через `newRelationService`/`newResidenceService`/`newEventService`) — тестовые двойники, дающие остальным тестам пакета доступ к настоящему хранилищу через `httpapi.RelationService`/`ResidenceService`/`EventService`. `internal/httpapi/write_store_test.go` — добавляет `TestRelationWriteContractWithRealStore`, `TestResidenceWriteContractWithRealStore`, `TestEventWriteContractWithRealStore` (genuine `sqlstore.Open`), каждый покрывает: создание с валидным FK(ами) → чтение → изменение → удаление → 404; специфичные 422 на несуществующий(е) FK (обе персоны для `Relation`, персона+место для `Residence`, `participants[1].person_id` для `Event` — индексированная ошибка на ВТОРОМ элементе, не первом); `sources[0].citation_id` 422; настоящий флип `PUBLIC → PRIVATE` через `PUT` (не `private → private`, по уроку финального ревью подпроекта 7) — анонимный `GET` до update видит запись (200), после — не видит (404); `Event` дополнительно покрывает `?person_id=`-фильтр списка и совпадение `search_events` по тексту места.

#### `internal/httpapi/store_test.go` (изменить — итоговое содержимое)

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
	create_event "github.com/amarin/genodex/internal/usecases/create_event"
	create_family "github.com/amarin/genodex/internal/usecases/create_family"
	create_given_name "github.com/amarin/genodex/internal/usecases/create_given_name"
	create_note "github.com/amarin/genodex/internal/usecases/create_note"
	create_parish "github.com/amarin/genodex/internal/usecases/create_parish"
	create_patronymic "github.com/amarin/genodex/internal/usecases/create_patronymic"
	create_person "github.com/amarin/genodex/internal/usecases/create_person"
	create_relation "github.com/amarin/genodex/internal/usecases/create_relation"
	create_repository "github.com/amarin/genodex/internal/usecases/create_repository"
	create_residence "github.com/amarin/genodex/internal/usecases/create_residence"
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
	delete_event "github.com/amarin/genodex/internal/usecases/delete_event"
	delete_family "github.com/amarin/genodex/internal/usecases/delete_family"
	delete_given_name "github.com/amarin/genodex/internal/usecases/delete_given_name"
	delete_note "github.com/amarin/genodex/internal/usecases/delete_note"
	delete_parish "github.com/amarin/genodex/internal/usecases/delete_parish"
	delete_patronymic "github.com/amarin/genodex/internal/usecases/delete_patronymic"
	delete_person "github.com/amarin/genodex/internal/usecases/delete_person"
	delete_relation "github.com/amarin/genodex/internal/usecases/delete_relation"
	delete_repository "github.com/amarin/genodex/internal/usecases/delete_repository"
	delete_residence "github.com/amarin/genodex/internal/usecases/delete_residence"
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
	get_event "github.com/amarin/genodex/internal/usecases/get_event"
	get_family "github.com/amarin/genodex/internal/usecases/get_family"
	get_given_name "github.com/amarin/genodex/internal/usecases/get_given_name"
	get_note "github.com/amarin/genodex/internal/usecases/get_note"
	get_parish "github.com/amarin/genodex/internal/usecases/get_parish"
	get_patronymic "github.com/amarin/genodex/internal/usecases/get_patronymic"
	get_person "github.com/amarin/genodex/internal/usecases/get_person"
	get_relation "github.com/amarin/genodex/internal/usecases/get_relation"
	get_repository "github.com/amarin/genodex/internal/usecases/get_repository"
	get_residence "github.com/amarin/genodex/internal/usecases/get_residence"
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
	list_events "github.com/amarin/genodex/internal/usecases/list_events"
	list_families "github.com/amarin/genodex/internal/usecases/list_families"
	list_given_names "github.com/amarin/genodex/internal/usecases/list_given_names"
	list_notes "github.com/amarin/genodex/internal/usecases/list_notes"
	list_parishes "github.com/amarin/genodex/internal/usecases/list_parishes"
	list_patronymics "github.com/amarin/genodex/internal/usecases/list_patronymics"
	list_people "github.com/amarin/genodex/internal/usecases/list_people"
	list_relations "github.com/amarin/genodex/internal/usecases/list_relations"
	list_repositories "github.com/amarin/genodex/internal/usecases/list_repositories"
	list_residences "github.com/amarin/genodex/internal/usecases/list_residences"
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
	search_events "github.com/amarin/genodex/internal/usecases/search_events"
	search_families "github.com/amarin/genodex/internal/usecases/search_families"
	search_given_names "github.com/amarin/genodex/internal/usecases/search_given_names"
	search_notes "github.com/amarin/genodex/internal/usecases/search_notes"
	search_parishes "github.com/amarin/genodex/internal/usecases/search_parishes"
	search_patronymics "github.com/amarin/genodex/internal/usecases/search_patronymics"
	search_people "github.com/amarin/genodex/internal/usecases/search_people"
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
	update_event "github.com/amarin/genodex/internal/usecases/update_event"
	update_family "github.com/amarin/genodex/internal/usecases/update_family"
	update_given_name "github.com/amarin/genodex/internal/usecases/update_given_name"
	update_note "github.com/amarin/genodex/internal/usecases/update_note"
	update_parish "github.com/amarin/genodex/internal/usecases/update_parish"
	update_patronymic "github.com/amarin/genodex/internal/usecases/update_patronymic"
	update_person "github.com/amarin/genodex/internal/usecases/update_person"
	update_relation "github.com/amarin/genodex/internal/usecases/update_relation"
	update_repository "github.com/amarin/genodex/internal/usecases/update_repository"
	update_residence "github.com/amarin/genodex/internal/usecases/update_residence"
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

// familyService — фасад httpapi.FamilyService на настоящих сценариях (так же
// собран internal/app's familyService).
type familyService struct {
	list   *list_families.Scenario
	search *search_families.Scenario
	get    *get_family.Scenario
	create *create_family.Scenario
	update *update_family.Scenario
	del    *delete_family.Scenario
}

func (s *familyService) ListFamilies(ctx context.Context, access models.Access, page models.Page) ([]models.Family, error) {
	return s.list.ListFamilies(ctx, access, page)
}

func (s *familyService) SearchFamilies(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Family, error) {
	return s.search.SearchFamilies(ctx, access, q)
}

func (s *familyService) GetFamily(ctx context.Context, access models.Access, id models.ID) (models.Family, error) {
	return s.get.GetFamily(ctx, access, id)
}

func (s *familyService) CreateFamily(ctx context.Context, f models.Family) (models.Family, error) {
	return s.create.CreateFamily(ctx, f)
}

func (s *familyService) UpdateFamily(ctx context.Context, f models.Family) error {
	return s.update.UpdateFamily(ctx, f)
}

func (s *familyService) DeleteFamily(ctx context.Context, id models.ID) error {
	return s.del.DeleteFamily(ctx, id)
}

// newFamilyService собирает фасад на настоящем хранилище.
func newFamilyService(t *testing.T, st *sqlstore.Store) *familyService {
	t.Helper()

	return &familyService{
		list:   list_families.New(st),
		search: search_families.New(st),
		get:    get_family.New(st),
		create: create_family.New(st, idgen.New()),
		update: update_family.New(st),
		del:    delete_family.New(st),
	}
}

// personService — фасад httpapi.PersonService на настоящих сценариях (так же
// собран internal/app's personService).
type personService struct {
	list   *list_people.Scenario
	search *search_people.Scenario
	get    *get_person.Scenario
	create *create_person.Scenario
	update *update_person.Scenario
	del    *delete_person.Scenario
}

func (s *personService) ListPeople(ctx context.Context, access models.Access, page models.Page) ([]models.Person, error) {
	return s.list.ListPeople(ctx, access, page)
}

func (s *personService) SearchPeople(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Person, error) {
	return s.search.SearchPeople(ctx, access, q)
}

func (s *personService) GetPerson(ctx context.Context, access models.Access, id models.ID) (models.Person, error) {
	return s.get.GetPerson(ctx, access, id)
}

func (s *personService) CreatePerson(ctx context.Context, p models.Person) (models.Person, error) {
	return s.create.CreatePerson(ctx, p)
}

func (s *personService) UpdatePerson(ctx context.Context, p models.Person) error {
	return s.update.UpdatePerson(ctx, p)
}

func (s *personService) DeletePerson(ctx context.Context, id models.ID) error {
	return s.del.DeletePerson(ctx, id)
}

// newPersonService собирает фасад на настоящем хранилище.
func newPersonService(t *testing.T, st *sqlstore.Store) *personService {
	t.Helper()

	return &personService{
		list:   list_people.New(st),
		search: search_people.New(st),
		get:    get_person.New(st),
		create: create_person.New(st, idgen.New()),
		update: update_person.New(st),
		del:    delete_person.New(st),
	}
}

// relationService — фасад httpapi.RelationService на настоящих сценариях
// (так же собран internal/app's relationService). Без search
// (search_relations намеренно не заводится, docs/data-model/
// entity-write.md §3.8).
type relationService struct {
	list   *list_relations.Scenario
	get    *get_relation.Scenario
	create *create_relation.Scenario
	update *update_relation.Scenario
	del    *delete_relation.Scenario
}

func (s *relationService) ListRelations(ctx context.Context, access models.Access, q models.RelationQuery) ([]models.Relation, error) {
	return s.list.ListRelations(ctx, access, q)
}

func (s *relationService) GetRelation(ctx context.Context, access models.Access, id models.ID) (models.Relation, error) {
	return s.get.GetRelation(ctx, access, id)
}

func (s *relationService) CreateRelation(ctx context.Context, r models.Relation) (models.Relation, error) {
	return s.create.CreateRelation(ctx, r)
}

func (s *relationService) UpdateRelation(ctx context.Context, r models.Relation) error {
	return s.update.UpdateRelation(ctx, r)
}

func (s *relationService) DeleteRelation(ctx context.Context, id models.ID) error {
	return s.del.DeleteRelation(ctx, id)
}

// newRelationService собирает фасад на настоящем хранилище.
func newRelationService(t *testing.T, st *sqlstore.Store) *relationService {
	t.Helper()

	return &relationService{
		list:   list_relations.New(st),
		get:    get_relation.New(st),
		create: create_relation.New(st, idgen.New()),
		update: update_relation.New(st),
		del:    delete_relation.New(st),
	}
}

// residenceService — фасад httpapi.ResidenceService на настоящих сценариях
// (так же собран internal/app's residenceService). Без search (см.
// relationService).
type residenceService struct {
	list   *list_residences.Scenario
	get    *get_residence.Scenario
	create *create_residence.Scenario
	update *update_residence.Scenario
	del    *delete_residence.Scenario
}

func (s *residenceService) ListResidences(ctx context.Context, access models.Access, q models.ResidenceQuery) ([]models.Residence, error) {
	return s.list.ListResidences(ctx, access, q)
}

func (s *residenceService) GetResidence(ctx context.Context, access models.Access, id models.ID) (models.Residence, error) {
	return s.get.GetResidence(ctx, access, id)
}

func (s *residenceService) CreateResidence(ctx context.Context, r models.Residence) (models.Residence, error) {
	return s.create.CreateResidence(ctx, r)
}

func (s *residenceService) UpdateResidence(ctx context.Context, r models.Residence) error {
	return s.update.UpdateResidence(ctx, r)
}

func (s *residenceService) DeleteResidence(ctx context.Context, id models.ID) error {
	return s.del.DeleteResidence(ctx, id)
}

// newResidenceService собирает фасад на настоящем хранилище.
func newResidenceService(t *testing.T, st *sqlstore.Store) *residenceService {
	t.Helper()

	return &residenceService{
		list:   list_residences.New(st),
		get:    get_residence.New(st),
		create: create_residence.New(st, idgen.New()),
		update: update_residence.New(st),
		del:    delete_residence.New(st),
	}
}

// eventService — фасад httpapi.EventService на настоящих сценариях (так же
// собран internal/app's eventService).
type eventService struct {
	list   *list_events.Scenario
	search *search_events.Scenario
	get    *get_event.Scenario
	create *create_event.Scenario
	update *update_event.Scenario
	del    *delete_event.Scenario
}

func (s *eventService) ListEvents(ctx context.Context, access models.Access, q models.EventQuery) ([]models.Event, error) {
	return s.list.ListEvents(ctx, access, q)
}

func (s *eventService) SearchEvents(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Event, error) {
	return s.search.SearchEvents(ctx, access, q)
}

func (s *eventService) GetEvent(ctx context.Context, access models.Access, id models.ID) (models.Event, error) {
	return s.get.GetEvent(ctx, access, id)
}

func (s *eventService) CreateEvent(ctx context.Context, e models.Event) (models.Event, error) {
	return s.create.CreateEvent(ctx, e)
}

func (s *eventService) UpdateEvent(ctx context.Context, e models.Event) error {
	return s.update.UpdateEvent(ctx, e)
}

func (s *eventService) DeleteEvent(ctx context.Context, id models.ID) error {
	return s.del.DeleteEvent(ctx, id)
}

// newEventService собирает фасад на настоящем хранилище.
func newEventService(t *testing.T, st *sqlstore.Store) *eventService {
	t.Helper()

	return &eventService{
		list:   list_events.New(st),
		search: search_events.New(st),
		get:    get_event.New(st),
		create: create_event.New(st, idgen.New()),
		update: update_event.New(st),
		del:    delete_event.New(st),
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

#### `internal/httpapi/write_store_test.go` (изменить — итоговое содержимое)

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
// Также проверяет неизменность archive_id при обновлении: попытка перенести
// существующий узел в другой архив — 422 на поле archive_id.
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

	// Попытка сменить archive_id узла при обновлении — 422 на поле
	// archive_id: принадлежность архиву неизменна после создания, иначе
	// дочерний узел (child) молча пропал бы из обоих деревьев.
	rec = putArchiveNodeReq(t, h, owner, "/api/archive-nodes/"+string(root.ID),
		fmt.Sprintf(`{"type":"fond","archive_id":%q,"label":"Фонд 1 (испр.)"}`, otherArchive.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"archive_id"`) {
		t.Fatalf("body = %s, want field=archive_id (смена archive_id при обновлении)", rec.Body)
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

// TestFamilyWriteContractWithRealStore: сквозной путь «хранилище → сценарии
// → HTTP» (по образцу TestRepositoryWriteContractWithRealStore, структурно
// ближайшего шаблона подпроекта 7 — Family отличается от Repository
// отсутствием Type/Address и переименованием URLs → Members) для родов:
// bootstrap-регистрация → создание → чтение → изменение → удаление →
// повторное чтение — 404. Дополнительно закрывает: приватность (Fix 1),
// проверяемая явно через ОБА пути (create И update — этот дефект уже
// повторялся дважды в программе, подпроекты 2 и 4); строгий FK
// sources[i].citation_id; и то, что members — мягкая ссылка на Person (без
// CRUD, подпроект 8) — ref/type round-trip'ятся как обычный TextRef, без
// проверки существования.
func TestFamilyWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Families:   newFamilyService(t, st),
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
	created := createFamily(t, h, owner,
		`{"name":"Ивановы","members":[],"notes":[],"private":false}`, http.StatusCreated)
	if created.Name != "Ивановы" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "F-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id — открыто анонимному посетителю.
	rec := getReq(t, h, "/api/families/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"name":"Ивановы"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена name/members/notes/sources/private.
	rec = putFamilyReq(t, h, owner, "/api/families/"+string(created.ID),
		`{"name":"Ивановы (испр.)","members":[],"notes":[],"private":false}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeFamilyS(t, rec)
	if updated.Name != "Ивановы (испр.)" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/families/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/families/"+string(created.ID)), http.StatusNotFound)

	// Fix 1 (CRITICAL, повторялась дважды — подпроекты 2 и 4): private
	// должен пережить и create, и update, а не только прямое сохранение.
	// Сначала create с private:true.
	private := createFamily(t, h, owner,
		`{"name":"Приватный род","members":[],"notes":[],"private":true}`, http.StatusCreated)
	if !private.Private {
		t.Fatalf("private (после create) = %+v, want Private=true", private)
	}
	requireStatusS(t, getReq(t, h, "/api/families/"+string(private.ID)), http.StatusNotFound)

	// Теперь update, переключающий PUBLIC → PRIVATE (не private → private —
	// та проверка ловит только "update сбрасывает private в false", а не
	// "update не выставляет private в true у публичной записи"; финальный
	// ревью подпроекта 7 нашёл, что прежняя версия этого шага создавала
	// запись сразу приватной, так что реального переключения не проверяла).
	public := createFamily(t, h, owner,
		`{"name":"Публичный род","members":[],"notes":[],"private":false}`, http.StatusCreated)
	requireStatusS(t, getReq(t, h, "/api/families/"+string(public.ID)), http.StatusOK)

	rec = putFamilyReq(t, h, owner, "/api/families/"+string(public.ID),
		`{"name":"Публичный род","members":[],"notes":[],"private":true}`)
	requireStatusS(t, rec, http.StatusOK)
	afterUpdate := decodeFamilyS(t, rec)
	if !afterUpdate.Private {
		t.Fatalf("private (после update) = %+v, want Private=true", afterUpdate)
	}
	requireStatusS(t, getReq(t, h, "/api/families/"+string(public.ID)), http.StatusNotFound)

	// members — мягкая ссылка на Person (без CRUD, подпроект 8):
	// ref/type сохраняются как обычный TextRef, существование не проверяется.
	// Проверяем и ответ create, и то, что ref/type реально пережили запись и
	// чтение из SQLite (а не только эхо сценария в памяти).
	withMember := createFamily(t, h, owner,
		`{"name":"Петровы","members":[{"text":"Пётр Петров","ref":"I-01ARZ3NDEKTSV4RRFFQ69G5FA9","type":"person"}],"notes":[],"private":false}`,
		http.StatusCreated)
	if len(withMember.Members) != 1 || withMember.Members[0].Ref != "I-01ARZ3NDEKTSV4RRFFQ69G5FA9" {
		t.Fatalf("withMember.Members (ответ create) = %+v", withMember.Members)
	}
	reread := getReq(t, h, "/api/families/"+string(withMember.ID))
	requireStatusS(t, reread, http.StatusOK)
	rereadFamily := decodeFamilyS(t, reread)
	if len(rereadFamily.Members) != 1 || rereadFamily.Members[0].Ref != "I-01ARZ3NDEKTSV4RRFFQ69G5FA9" ||
		rereadFamily.Members[0].Type != "person" {
		t.Fatalf("withMember.Members (после чтения из SQLite) = %+v", rereadFamily.Members)
	}

	// Ретрофит-паттерн (по образцу TestArchiveWriteContractWithRealStore):
	// строгий FK sources[i].citation_id — сквозная проверка через реальный
	// HTTP-хендлер → сценарий → SQLite.
	src := createSource(t, h, owner,
		`{"kind":"document","title":"Ревизская сказка","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	cit := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)

	withCitation := createFamily(t, h, owner,
		fmt.Sprintf(`{"name":"Сидоровы","members":[],"notes":[],"sources":[{"citation_id":%q}],"private":false}`, cit.ID),
		http.StatusCreated)
	if len(withCitation.Sources) != 1 || withCitation.Sources[0].CitationID != string(cit.ID) {
		t.Fatalf("withCitation.Sources = %+v, want [{citation_id: %q}]", withCitation.Sources, cit.ID)
	}

	// Строгий FK: корректный по формату, но несуществующий citation_id — 422 на sources[0].citation_id.
	rec = postFamilyReq(t, h, owner,
		`{"name":"Призрачный род","members":[],"notes":[],"sources":[{"citation_id":"C-01ARZ3NDEKTSV4RRFFQ69G5FA9"}],"private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"sources[0].citation_id"`) {
		t.Fatalf("body = %s, want field=sources[0].citation_id", rec.Body)
	}
}

func createFamily(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Family {
	t.Helper()

	rec := postFamilyReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeFamilyS(t, rec)
}

func decodeFamilyS(t *testing.T, rec *httptest.ResponseRecorder) transport.Family {
	t.Helper()

	var f transport.Family
	if err := json.Unmarshal(rec.Body.Bytes(), &f); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return f
}

func postFamilyReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/families", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putFamilyReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestPersonWriteContractWithRealStore: сквозной путь «хранилище → сценарии
// → HTTP» (по образцу TestFamilyWriteContractWithRealStore) для персон —
// ядра графа генеалогии (подпроект 8): bootstrap-регистрация → создание с
// двумя записями Names (main и married), каждая — с мягкой ссылкой на
// заведомо несуществующие Surname/GivenName/Patronymic (доказывает 201, без
// проверки существования) → чтение → изменение → удаление → повторное
// чтение — 404. Дополнительно закрывает: приватность через ОБА пути (create
// И update, PUBLIC → PRIVATE, не private → private — тот же урок финального
// ревью подпроекта 7); строгий FK sources[i].citation_id; и то, что поиск
// находит персону по части фамилии/имени/отчества из ВТОРОЙ записи Names
// (married), а не только из первой (main) — доказывает, что personTerms
// индексирует все имена, не только основное.
func TestPersonWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		People:     newPersonService(t, st),
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

	// Создание: два имени (main + married), каждое — с мягкой ссылкой на
	// несуществующие словарные записи. 201, без проверки существования.
	createBody := `{
		"gender": "female",
		"names": [
			{"type":"main","surname":{"text":"Дорожкина","ref":"SN-01ARZ3NDEKTSV4RRFFQ69G5FA1","type":"surname"},"given":{"text":"Акилина","ref":"GN-01ARZ3NDEKTSV4RRFFQ69G5FA1","type":"given_name"}},
			{"type":"married","surname":{"text":"Петрова","ref":"SN-01ARZ3NDEKTSV4RRFFQ69G5FA2","type":"surname"},"patronymic":{"text":"Ивановна","ref":"PN-01ARZ3NDEKTSV4RRFFQ69G5FA1","type":"patronymic"}}
		],
		"estates": [{"text":"крестьяне"}],
		"titles": [],
		"nicknames": [],
		"notes": [],
		"private": false
	}`
	created := createPerson(t, h, owner, createBody, http.StatusCreated)
	if len(created.Names) != 2 || created.Names[0].Surname.Text != "Дорожкина" || created.Names[1].Surname.Text != "Петрова" {
		t.Fatalf("created.Names = %+v", created.Names)
	}
	if created.Names[0].Surname.Ref != "SN-01ARZ3NDEKTSV4RRFFQ69G5FA1" || created.Names[1].Patronymic.Ref != "PN-01ARZ3NDEKTSV4RRFFQ69G5FA1" {
		t.Fatalf("мягкие ссылки не сохранены как есть: %+v", created.Names)
	}
	if !strings.HasPrefix(string(created.ID), "I-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id — открыто анонимному посетителю; ссылки пережили запись и
	// чтение из SQLite (а не только эхо сценария в памяти).
	rec := getReq(t, h, "/api/people/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	reread := decodePersonS(t, rec)
	if len(reread.Names) != 2 || reread.Names[1].Surname.Ref != "SN-01ARZ3NDEKTSV4RRFFQ69G5FA2" {
		t.Fatalf("reread.Names (после чтения из SQLite) = %+v", reread.Names)
	}

	// Поиск: часть фамилии, которая встречается ТОЛЬКО во второй (married)
	// записи Names, должна находить персону — доказывает, что personTerms
	// индексирует все имена, а не только первое/основное.
	searchRec := getReq(t, h, "/api/people/search?q=Петр")
	requireStatusS(t, searchRec, http.StatusOK)
	if !strings.Contains(searchRec.Body.String(), string(created.ID)) {
		t.Fatalf("поиск по началу фамилии из married-имени не нашёл запись: %s", searchRec.Body)
	}

	// Изменение: полная замена; gender меняется, private остаётся false.
	updateBody := `{
		"gender": "female",
		"names": [
			{"type":"main","surname":{"text":"Дорожкина"},"given":{"text":"Акилина"}}
		],
		"estates": [],
		"titles": [],
		"nicknames": [],
		"notes": [],
		"private": false
	}`
	rec = putPersonReq(t, h, owner, "/api/people/"+string(created.ID), updateBody)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodePersonS(t, rec)
	if len(updated.Names) != 1 || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/people/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/people/"+string(created.ID)), http.StatusNotFound)

	// Fix 1 (CRITICAL, повторялась в программе несколько раз): private
	// должен пережить и create, и update, а не только прямое сохранение.
	// Сначала create с private:true.
	private := createPerson(t, h, owner, `{"names":[],"estates":[],"titles":[],"nicknames":[],"notes":[],"private":true}`, http.StatusCreated)
	if !private.Private {
		t.Fatalf("private (после create) = %+v, want Private=true", private)
	}
	requireStatusS(t, getReq(t, h, "/api/people/"+string(private.ID)), http.StatusNotFound)

	// Теперь update, переключающий PUBLIC → PRIVATE (не private → private —
	// та проверка ловит только "update сбрасывает private в false", а не
	// "update не выставляет private в true у публичной записи"; тот же
	// урок, что и финальное ревью подпроекта 7).
	public := createPerson(t, h, owner,
		`{"names":[{"type":"main","surname":{"text":"Тайнова"},"given":{"text":"Анна"}}],"estates":[],"titles":[],"nicknames":[],"notes":[],"private":false}`,
		http.StatusCreated)
	requireStatusS(t, getReq(t, h, "/api/people/"+string(public.ID)), http.StatusOK)

	rec = putPersonReq(t, h, owner, "/api/people/"+string(public.ID),
		`{"names":[{"type":"main","surname":{"text":"Тайнова"},"given":{"text":"Анна"}}],"estates":[],"titles":[],"nicknames":[],"notes":[],"private":true}`)
	requireStatusS(t, rec, http.StatusOK)
	afterUpdate := decodePersonS(t, rec)
	if !afterUpdate.Private {
		t.Fatalf("private (после update) = %+v, want Private=true", afterUpdate)
	}
	requireStatusS(t, getReq(t, h, "/api/people/"+string(public.ID)), http.StatusNotFound)

	// Приватность работает не только на прямом GET по id — те же правила
	// доступа обязаны отфильтровывать приватную запись и в списке
	// (listByIDs), и в поиске (searchSQL): это отдельные от Get пути,
	// которые теоретически могли бы разойтись.
	listRec := getReq(t, h, "/api/people")
	requireStatusS(t, listRec, http.StatusOK)
	if strings.Contains(listRec.Body.String(), string(public.ID)) {
		t.Fatalf("анонимный список /api/people содержит приватную персону: %s", listRec.Body)
	}
	searchPrivateRec := getReq(t, h, "/api/people/search?q=Тайнова")
	requireStatusS(t, searchPrivateRec, http.StatusOK)
	if strings.Contains(searchPrivateRec.Body.String(), string(public.ID)) {
		t.Fatalf("анонимный поиск /api/people/search содержит приватную персону: %s", searchPrivateRec.Body)
	}

	// Строгий FK: существующая цитата — сохраняется.
	src := createSource(t, h, owner,
		`{"kind":"document","title":"Ревизская сказка","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	cit := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)

	withCitation := createPerson(t, h, owner,
		fmt.Sprintf(`{"names":[],"estates":[],"titles":[],"nicknames":[],"notes":[],"sources":[{"citation_id":%q}],"private":false}`, cit.ID),
		http.StatusCreated)
	if len(withCitation.Sources) != 1 || withCitation.Sources[0].CitationID != string(cit.ID) {
		t.Fatalf("withCitation.Sources = %+v, want [{citation_id: %q}]", withCitation.Sources, cit.ID)
	}

	// Строгий FK: корректный по формату, но несуществующий citation_id — 422 на sources[0].citation_id.
	rec = postPersonReq(t, h, owner,
		`{"names":[],"estates":[],"titles":[],"nicknames":[],"notes":[],"sources":[{"citation_id":"C-01ARZ3NDEKTSV4RRFFQ69G5FA9"}],"private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"sources[0].citation_id"`) {
		t.Fatalf("body = %s, want field=sources[0].citation_id", rec.Body)
	}
}

func createPerson(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Person {
	t.Helper()

	rec := postPersonReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodePersonS(t, rec)
}

func decodePersonS(t *testing.T, rec *httptest.ResponseRecorder) transport.Person {
	t.Helper()

	var p transport.Person
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return p
}

func postPersonReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/people", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putPersonReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestRelationWriteContractWithRealStore: сквозной путь «хранилище →
// сценарии → HTTP» (по образцу TestFamilyWriteContractWithRealStore) для
// рёбер графа родства — первой сущности программы с двумя строгими ссылками
// на один и тот же тип (person_a/person_b → Person, docs/data-model/
// entity-write.md §3.8). Покрывает: create с двумя валидными person_a/
// person_b → чтение → изменение → удаление → 404; 422 на несуществующий
// person_a; 422 на несуществующий person_b (обе стороны проверяются
// независимо); строгий FK sources[0].citation_id; и генуинный
// PUBLIC → PRIVATE update-флип (не private → private).
func TestRelationWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		People:     newPersonService(t, st),
		Relations:  newRelationService(t, st),
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

	personA := createPerson(t, h, owner, `{"names":[{"type":"main","surname":{"text":"Иванов"},"given":{"text":"Иван"}}],"private":false}`, http.StatusCreated)
	personB := createPerson(t, h, owner, `{"names":[{"type":"main","surname":{"text":"Иванова"},"given":{"text":"Мария"}}],"private":false}`, http.StatusCreated)

	// Создание.
	created := createRelation(t, h, owner,
		fmt.Sprintf(`{"kind":"blood","person_a":%q,"person_b":%q,"notes":[],"private":false}`, personA.ID, personB.ID),
		http.StatusCreated)
	if created.Kind != "blood" || created.PersonA != personA.ID || created.PersonB != personB.ID {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "RL-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id — открыто анонимному посетителю.
	rec := getReq(t, h, "/api/relations/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"kind":"blood"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена kind/person_a/person_b/sources/notes/private.
	rec = putRelationReq(t, h, owner, "/api/relations/"+string(created.ID),
		fmt.Sprintf(`{"kind":"marriage","person_a":%q,"person_b":%q,"notes":[],"private":false}`, personA.ID, personB.ID))
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeRelationS(t, rec)
	if updated.Kind != "marriage" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/relations/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/relations/"+string(created.ID)), http.StatusNotFound)

	// Строгий FK: несуществующий person_a — 422 на поле person_a.
	rec = postRelationReq(t, h, owner,
		fmt.Sprintf(`{"kind":"blood","person_a":"I-01ARZ3NDEKTSV4RRFFQ69G5FA9","person_b":%q,"notes":[],"private":false}`, personB.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"person_a"`) {
		t.Fatalf("body = %s, want field=person_a", rec.Body)
	}

	// Строгий FK: person_a существует, person_b — нет — 422 на поле person_b
	// (обе стороны проверяются независимо).
	rec = postRelationReq(t, h, owner,
		fmt.Sprintf(`{"kind":"blood","person_a":%q,"person_b":"I-01ARZ3NDEKTSV4RRFFQ69G5FA9","notes":[],"private":false}`, personA.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"person_b"`) {
		t.Fatalf("body = %s, want field=person_b", rec.Body)
	}

	// Строгий FK: sources[0].citation_id несуществующий — 422.
	rec = postRelationReq(t, h, owner,
		fmt.Sprintf(`{"kind":"blood","person_a":%q,"person_b":%q,"sources":[{"citation_id":"C-01ARZ3NDEKTSV4RRFFQ69G5FA9"}],"notes":[],"private":false}`, personA.ID, personB.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"sources[0].citation_id"`) {
		t.Fatalf("body = %s, want field=sources[0].citation_id", rec.Body)
	}

	// Строгий FK: существующая цитата — сохраняется.
	src := createSource(t, h, owner,
		`{"kind":"document","title":"Метрическая книга","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	cit := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)

	withCitation := createRelation(t, h, owner,
		fmt.Sprintf(`{"kind":"blood","person_a":%q,"person_b":%q,"sources":[{"citation_id":%q}],"notes":[],"private":false}`, personA.ID, personB.ID, cit.ID),
		http.StatusCreated)
	if len(withCitation.Sources) != 1 || withCitation.Sources[0].CitationID != string(cit.ID) {
		t.Fatalf("withCitation.Sources = %+v, want [{citation_id: %q}]", withCitation.Sources, cit.ID)
	}

	// PUBLIC → PRIVATE update-флип (не private → private).
	public := createRelation(t, h, owner,
		fmt.Sprintf(`{"kind":"blood","person_a":%q,"person_b":%q,"notes":[],"private":false}`, personA.ID, personB.ID),
		http.StatusCreated)
	requireStatusS(t, getReq(t, h, "/api/relations/"+string(public.ID)), http.StatusOK)

	rec = putRelationReq(t, h, owner, "/api/relations/"+string(public.ID),
		fmt.Sprintf(`{"kind":"blood","person_a":%q,"person_b":%q,"notes":[],"private":true}`, personA.ID, personB.ID))
	requireStatusS(t, rec, http.StatusOK)
	afterUpdate := decodeRelationS(t, rec)
	if !afterUpdate.Private {
		t.Fatalf("private (после update) = %+v, want Private=true", afterUpdate)
	}
	requireStatusS(t, getReq(t, h, "/api/relations/"+string(public.ID)), http.StatusNotFound)

	// person_id-фильтр списка: совпадает с person_a ИЛИ person_b.
	listRec := getReq(t, h, "/api/relations?person_id="+string(personA.ID))
	requireStatusS(t, listRec, http.StatusOK)
	if !strings.Contains(listRec.Body.String(), string(withCitation.ID)) {
		t.Fatalf("список по person_id=%q не содержит ребро %q: %s", personA.ID, withCitation.ID, listRec.Body)
	}
}

func createRelation(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Relation {
	t.Helper()

	rec := postRelationReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeRelationS(t, rec)
}

func decodeRelationS(t *testing.T, rec *httptest.ResponseRecorder) transport.Relation {
	t.Helper()

	var r transport.Relation
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return r
}

func postRelationReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/relations", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putRelationReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestResidenceWriteContractWithRealStore: сквозной путь «хранилище →
// сценарии → HTTP» для проживаний. Покрывает: create с валидными person_id/
// place_id → чтение → изменение → удаление → 404; 422 на несуществующий
// person_id; 422 на несуществующий place_id; строгий FK
// sources[0].citation_id; PUBLIC → PRIVATE update-флип.
func TestResidenceWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		People:     newPersonService(t, st),
		Residences: newResidenceService(t, st),
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

	person := createPerson(t, h, owner, `{"names":[{"type":"main","surname":{"text":"Сидоров"},"given":{"text":"Пётр"}}],"private":false}`, http.StatusCreated)
	place := createDivision(t, h, owner, `{"name":"Давыдово","type":"selo"}`, http.StatusCreated)

	// Создание.
	created := createResidence(t, h, owner,
		fmt.Sprintf(`{"person_id":%q,"place_id":%q,"note":"изба","private":false}`, person.ID, place.ID),
		http.StatusCreated)
	if created.PersonID != person.ID || created.PlaceID != place.ID || created.Note != "изба" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "RS-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id.
	rec := getReq(t, h, "/api/residences/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"note":"изба"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение.
	rec = putResidenceReq(t, h, owner, "/api/residences/"+string(created.ID),
		fmt.Sprintf(`{"person_id":%q,"place_id":%q,"note":"новая изба","private":false}`, person.ID, place.ID))
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeResidenceS(t, rec)
	if updated.Note != "новая изба" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/residences/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/residences/"+string(created.ID)), http.StatusNotFound)

	// Строгий FK: несуществующий person_id — 422 на поле person_id.
	rec = postResidenceReq(t, h, owner,
		fmt.Sprintf(`{"person_id":"I-01ARZ3NDEKTSV4RRFFQ69G5FA9","place_id":%q,"private":false}`, place.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"person_id"`) {
		t.Fatalf("body = %s, want field=person_id", rec.Body)
	}

	// Строгий FK: несуществующий place_id — 422 на поле place_id.
	rec = postResidenceReq(t, h, owner,
		fmt.Sprintf(`{"person_id":%q,"place_id":"AD-01ARZ3NDEKTSV4RRFFQ69G5FA9","private":false}`, person.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"place_id"`) {
		t.Fatalf("body = %s, want field=place_id", rec.Body)
	}

	// Строгий FK: sources[0].citation_id несуществующий — 422.
	rec = postResidenceReq(t, h, owner,
		fmt.Sprintf(`{"person_id":%q,"place_id":%q,"sources":[{"citation_id":"C-01ARZ3NDEKTSV4RRFFQ69G5FA9"}],"private":false}`, person.ID, place.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"sources[0].citation_id"`) {
		t.Fatalf("body = %s, want field=sources[0].citation_id", rec.Body)
	}

	// Строгий FK: существующая цитата — сохраняется.
	src := createSource(t, h, owner,
		`{"kind":"document","title":"Ревизская сказка","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	cit := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)

	withCitation := createResidence(t, h, owner,
		fmt.Sprintf(`{"person_id":%q,"place_id":%q,"sources":[{"citation_id":%q}],"private":false}`, person.ID, place.ID, cit.ID),
		http.StatusCreated)
	if len(withCitation.Sources) != 1 || withCitation.Sources[0].CitationID != string(cit.ID) {
		t.Fatalf("withCitation.Sources = %+v, want [{citation_id: %q}]", withCitation.Sources, cit.ID)
	}

	// PUBLIC → PRIVATE update-флип (не private → private).
	public := createResidence(t, h, owner,
		fmt.Sprintf(`{"person_id":%q,"place_id":%q,"private":false}`, person.ID, place.ID), http.StatusCreated)
	requireStatusS(t, getReq(t, h, "/api/residences/"+string(public.ID)), http.StatusOK)

	rec = putResidenceReq(t, h, owner, "/api/residences/"+string(public.ID),
		fmt.Sprintf(`{"person_id":%q,"place_id":%q,"private":true}`, person.ID, place.ID))
	requireStatusS(t, rec, http.StatusOK)
	afterUpdate := decodeResidenceS(t, rec)
	if !afterUpdate.Private {
		t.Fatalf("private (после update) = %+v, want Private=true", afterUpdate)
	}
	requireStatusS(t, getReq(t, h, "/api/residences/"+string(public.ID)), http.StatusNotFound)

	// person_id/place_id-фильтры списка.
	listRec := getReq(t, h, "/api/residences?person_id="+string(person.ID)+"&place_id="+string(place.ID))
	requireStatusS(t, listRec, http.StatusOK)
	if !strings.Contains(listRec.Body.String(), string(withCitation.ID)) {
		t.Fatalf("список по person_id/place_id не содержит запись %q: %s", withCitation.ID, listRec.Body)
	}
}

func createResidence(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Residence {
	t.Helper()

	rec := postResidenceReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeResidenceS(t, rec)
}

func decodeResidenceS(t *testing.T, rec *httptest.ResponseRecorder) transport.Residence {
	t.Helper()

	var r transport.Residence
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return r
}

func postResidenceReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/residences", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putResidenceReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestEventWriteContractWithRealStore: сквозной путь «хранилище → сценарии
// → HTTP» для событий. Покрывает: create с двумя участниками → чтение →
// изменение → удаление → 404; индексированный 422 на несуществующий
// participants[1].person_id; Place — мягкая ссылка (фабрикованный id
// круговорот без ошибки); строгий FK sources[0].citation_id;
// PUBLIC → PRIVATE update-флип; person_id-фильтр списка;
// search_events по началу текста места.
func TestEventWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		People:     newPersonService(t, st),
		Events:     newEventService(t, st),
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

	parent := createPerson(t, h, owner, `{"names":[{"type":"main","surname":{"text":"Кузнецов"},"given":{"text":"Фёдор"}}],"private":false}`, http.StatusCreated)
	witness := createPerson(t, h, owner, `{"names":[{"type":"main","surname":{"text":"Смирнов"},"given":{"text":"Егор"}}],"private":false}`, http.StatusCreated)

	// Создание: два участника, оба существуют + мягкая ссылка Place.
	created := createEvent(t, h, owner, fmt.Sprintf(`{
		"type": "birth",
		"place": {"text":"село Давыдово","ref":"AD-01ARZ3NDEKTSV4RRFFQ69G5FA9","type":"administrative_division"},
		"participants": [
			{"person_id":%q,"role":"родитель"},
			{"person_id":%q,"role":"свидетель"}
		],
		"notes": [],
		"private": false
	}`, parent.ID, witness.ID), http.StatusCreated)
	if created.Type != "birth" || len(created.Participants) != 2 {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "E-") {
		t.Fatalf("id = %q", created.ID)
	}
	// Place — мягкая ссылка на заведомо несуществующий id: round-trip без ошибки.
	if created.Place == nil || created.Place.Ref != "AD-01ARZ3NDEKTSV4RRFFQ69G5FA9" || created.Place.Text != "село Давыдово" {
		t.Fatalf("created.Place = %+v, ожидался мягкий round-trip несуществующей ссылки", created.Place)
	}

	// Чтение по id — ссылки/участники пережили запись и чтение из SQLite.
	rec := getReq(t, h, "/api/events/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	reread := decodeEventS(t, rec)
	if len(reread.Participants) != 2 || reread.Place == nil || reread.Place.Ref != "AD-01ARZ3NDEKTSV4RRFFQ69G5FA9" {
		t.Fatalf("reread (после чтения из SQLite) = %+v", reread)
	}

	// search_events: по началу текста места (весь place-текст индексируется
	// как один термин — "Давыд" не подошёл бы, текст начинается с "село").
	searchRec := getReq(t, h, "/api/events/search?q=село")
	requireStatusS(t, searchRec, http.StatusOK)
	if !strings.Contains(searchRec.Body.String(), string(created.ID)) {
		t.Fatalf("поиск по началу текста места не нашёл событие: %s", searchRec.Body)
	}

	// person_id-фильтр списка: находит событие по КАЖДОМУ из двух участников.
	for _, p := range []transport.Person{parent, witness} {
		listRec := getReq(t, h, "/api/events?person_id="+string(p.ID))
		requireStatusS(t, listRec, http.StatusOK)
		if !strings.Contains(listRec.Body.String(), string(created.ID)) {
			t.Fatalf("список по person_id=%q не содержит событие: %s", p.ID, listRec.Body)
		}
	}

	// Изменение: полная замена, участники сокращены до одного.
	rec = putEventReq(t, h, owner, "/api/events/"+string(created.ID), fmt.Sprintf(`{
		"type": "death",
		"participants": [{"person_id":%q,"role":"умерший"}],
		"notes": [],
		"private": false
	}`, parent.ID))
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeEventS(t, rec)
	if updated.Type != "death" || len(updated.Participants) != 1 || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/events/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/events/"+string(created.ID)), http.StatusNotFound)

	// Строгий FK, индексированная ошибка: participants[0] существует,
	// participants[1] — нет — 422 на participants[1].person_id.
	rec = postEventReq(t, h, owner, fmt.Sprintf(`{
		"type": "birth",
		"participants": [
			{"person_id":%q,"role":"родитель"},
			{"person_id":"I-01ARZ3NDEKTSV4RRFFQ69G5FA9","role":"свидетель"}
		],
		"private": false
	}`, parent.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"participants[1].person_id"`) {
		t.Fatalf("body = %s, want field=participants[1].person_id", rec.Body)
	}

	// Строгий FK: sources[0].citation_id несуществующий — 422.
	rec = postEventReq(t, h, owner, `{"type":"birth","sources":[{"citation_id":"C-01ARZ3NDEKTSV4RRFFQ69G5FA9"}],"private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"sources[0].citation_id"`) {
		t.Fatalf("body = %s, want field=sources[0].citation_id", rec.Body)
	}

	// Строгий FK: существующая цитата — сохраняется.
	src := createSource(t, h, owner,
		`{"kind":"document","title":"Метрическая книга","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	cit := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)

	withCitation := createEvent(t, h, owner,
		fmt.Sprintf(`{"type":"birth","sources":[{"citation_id":%q}],"private":false}`, cit.ID),
		http.StatusCreated)
	if len(withCitation.Sources) != 1 || withCitation.Sources[0].CitationID != string(cit.ID) {
		t.Fatalf("withCitation.Sources = %+v, want [{citation_id: %q}]", withCitation.Sources, cit.ID)
	}

	// PUBLIC → PRIVATE update-флип (не private → private).
	public := createEvent(t, h, owner, `{"type":"birth","private":false}`, http.StatusCreated)
	requireStatusS(t, getReq(t, h, "/api/events/"+string(public.ID)), http.StatusOK)

	rec = putEventReq(t, h, owner, "/api/events/"+string(public.ID), `{"type":"birth","private":true}`)
	requireStatusS(t, rec, http.StatusOK)
	afterUpdate := decodeEventS(t, rec)
	if !afterUpdate.Private {
		t.Fatalf("private (после update) = %+v, want Private=true", afterUpdate)
	}
	requireStatusS(t, getReq(t, h, "/api/events/"+string(public.ID)), http.StatusNotFound)
}

func createEvent(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Event {
	t.Helper()

	rec := postEventReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeEventS(t, rec)
}

func decodeEventS(t *testing.T, rec *httptest.ResponseRecorder) transport.Event {
	t.Helper()

	var e transport.Event
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return e
}

func postEventReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/events", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putEventReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

### 1.9. Документация: `docs/usage.md`, `docs/architecture.md`, `CHANGELOG.md`, `entity-write.md` §3.8/§5

По правилу, закреплённому после подпроекта 4 (доки/CHANGELOG пишутся в плане, а не оставляются на финальное ревью) — ниже точные вставки/замены в уже существующие большие файлы; порядок существующих строк не меняется, только добавляются/заменяются указанные фрагменты. Эти фрагменты закрывают ТОЛЬКО бэкенд-часть доков — см. «Предпосылка»: и `CHANGELOG.md`-пункт, и преамбула §3.8, и абзац §5 буквально говорят «веб — отдельная работа/задача»; Шаг 2.8 (Задача 2) эти формулировки заменяет на описание реально построенного.

#### `docs/usage.md` — три новые строки HTTP-таблицы (после строки `/api/people`, до `/api/notes`)

Вставить после существующей строки `/api/people`:

```markdown
| `/api/relations` | Рёбра графа родства между двумя персонами (JSON, подпроект 9): `GET` — список `[{"id", "kind", "rel_type", "person_a", "person_b", "since", "until", "sources", "notes", "private"}]`, необязательный `?person_id=` — только рёбра, где эта персона `person_a` ИЛИ `person_b`; `POST` — создание; `GET/PUT/DELETE /api/relations/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404). **Без `/api/relations/search`** — у `Relation` нет собственных поисковых полей (индекс намеренно пуст), `?person_id=` — более полезная замена. `person_a`/`person_b` — ДВЕ строгие ссылки на `Person` (первая пара строгих ссылок на один и тот же тип в программе): несуществующая — 422 на поле `person_a`/`person_b` соответственно; должны отличаться друг от друга (проверка модели). `kind` — закрытый набор `blood`/`marriage`/`adoption`/`associate`; `rel_type` — обязателен и только при `kind=associate`, иначе должен отсутствовать (открытый набор). |
| `/api/residences` | Проживания персоны в месте (JSON, подпроект 9): `GET` — список `[{"id", "person_id", "place_id", "since", "until", "sources", "note", "private"}]`, необязательные `?person_id=`/`?place_id=` (пересекаются, если оба заданы); `POST` — создание; `GET/PUT/DELETE /api/residences/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404). **Без `/api/residences/search`** (см. `/api/relations` — тот же принцип). `person_id` — строгая ссылка на `Person`, `place_id` — строгая ссылка именно на `AdministrativeDivision` (не любой `PlaceRef`): несуществующая — 422 на поле `person_id`/`place_id`. `note` — ОДНА строка (не список `TextRef`, в отличие от `notes` у большинства сущностей). |
| `/api/events` | События жизненного факта (JSON, подпроект 9): `GET` — список `[{"id", "type", "date", "place", "participants", "sources", "notes", "private"}]`, необязательный `?person_id=` — только события, где эта персона участвует (любой элемент `participants`); `POST` — создание; `GET/PUT/DELETE /api/events/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404); `GET /api/events/search?q=` — ищет ТОЛЬКО по началу текста `place` (единственное индексируемое поле события — `type`/`date`/`participants` поиском не охвачены). `type` — открытый набор (`birth`/`death`/`marriage`/`burial`/`confession`/`census` и др.). `place` — необязательная МЯГКАЯ ссылка (`PlaceRef`: текст или ссылка, ограниченная типом `admin_division`/`church`/`parish`) — существование НИКОГДА не проверяется, только формат/тип. `participants` — список `{person_id, role, note?}`: `person_id` — СТРОГАЯ ссылка на `Person` (единственная строгая ссылка внутри массива объектов в программе): несуществующая — 422 на поле `participants[i].person_id` (по индексу элемента); `role` обязательна и непуста. |
```

#### `docs/usage.md` — дополнение к абзацу про `sources` (после HTTP-таблицы)

Заменить предложение в конце существующего абзаца про `sources`:

```markdown
`/api/archive-nodes`/`/api/archive-documents` (подпроект 6), `/api/families` (подпроект 7) и `/api/people` (подпроект 8) — первые полностью новые сущности программы с `sources`, редактируемым с самого начала (не ретрофит).
```
на:
```markdown
`/api/archive-nodes`/`/api/archive-documents` (подпроект 6), `/api/families` (подпроект 7), `/api/people` (подпроект 8) и `/api/relations`/`/api/residences`/`/api/events` (подпроект 9) — первые полностью новые сущности программы с `sources`, редактируемым с самого начала (не ретрофит).
```

#### `docs/usage.md` — 18 новых строк таблицы MCP-тулов (после `person_delete`, до абзаца про `sources`-аргумент) и дополнение самого абзаца

Заменить существующий блок (последние три строки таблицы про `person_*` и абзац сразу после неё):

```markdown
| `person_create` | Создание записи: все поля необязательны — `gender` (`male`/`female`/`unknown`), `names` (массив объектов `{type?, surname, given, patronymic, prefix?, suffix?, since?, until?}` — первый массив объектов, чьи собственные элементы несут вложенные объекты, см. `personNameObjectProperties`; `surname`/`given`/`patronymic` — мягкие ссылки, существование не проверяется), `estates`/`titles`/`nicknames`/`notes` (тексты; `estates`/`titles` — мягкая ссылка на словарь, без проверки существования), `sources`, `private`; id генерирует сервер |
| `person_update` | Изменение записи: все поля необязательны — `gender`/`estates`/`titles`/`nicknames`/`notes`/`private`/`names`/`sources` при отсутствии аргумента в вызове сохраняют текущее значение, явный пустой список/массив/`false` — очищает/сбрасывает его; `estates`/`titles`/`nicknames`/`notes` принимают только текст (без ref/type в MCP-контракте) — ссылка на элементе теряется, если поле передано |
| `person_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |

`<entity>_create`/`<entity>_update` у `division`/`repository`/`church`/`parish`/`archive`/`note`/`archive_node`/`archive_document`/`family`/`person` принимают аргумент `sources` — массив объектов `{citation_id, reliability?, role?, note?}` (первый MCP-аргумент вида «массив объектов» в программе, введён в подпроекте 5, см. `sourceLinkObjectProperties`; раньше массивы были только строками). Несуществующий `citation_id` — ошибка тула на поле `sources[i].citation_id`. `person_create`/`person_update` (подпроект 8) дополнительно принимают `names` — первый массив объектов, чьи собственные элементы тоже несут вложенные объекты (`surname`/`given`/`patronymic` — `TextRef`, `since`/`until` — `FactDate`, см. `personNameObjectProperties`).
```
на:
```markdown
| `person_create` | Создание записи: все поля необязательны — `gender` (`male`/`female`/`unknown`), `names` (массив объектов `{type?, surname, given, patronymic, prefix?, suffix?, since?, until?}` — первый массив объектов, чьи собственные элементы несут вложенные объекты, см. `personNameObjectProperties`; `surname`/`given`/`patronymic` — мягкие ссылки, существование не проверяется), `estates`/`titles`/`nicknames`/`notes` (тексты; `estates`/`titles` — мягкая ссылка на словарь, без проверки существования), `sources`, `private`; id генерирует сервер |
| `person_update` | Изменение записи: все поля необязательны — `gender`/`estates`/`titles`/`nicknames`/`notes`/`private`/`names`/`sources` при отсутствии аргумента в вызове сохраняют текущее значение, явный пустой список/массив/`false` — очищает/сбрасывает его; `estates`/`titles`/`nicknames`/`notes` принимают только текст (без ref/type в MCP-контракте) — ссылка на элементе теряется, если поле передано |
| `person_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
| `relation_list` | Список рёбер графа родства; аргументы `person_id` (необязателен — только рёбра, где эта персона `person_a` ИЛИ `person_b`), `limit`, `offset`. **Нет `relation_search`** — у `Relation` нет собственных поисковых полей, см. `/api/relations` выше |
| `relation_get` | Ребро по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула |
| `relation_create` | Создание ребра: `kind` (обязателен, `blood`/`marriage`/`adoption`/`associate`), `rel_type` (обязателен только при `kind=associate`), `person_a`/`person_b` (обязательны, строгие ссылки на персону, должны отличаться), `since`/`until` (структурированная дата), `sources`, `notes`, `private`; id генерирует сервер |
| `relation_update` | Изменение ребра: `kind`/`person_a`/`person_b` — REQUIRED, заменяются БЕЗУСЛОВНО при каждом вызове (ядро того, что представляет собой ребро, как `citation_update`'s `source_id`); `rel_type`/`sources`/`notes`/`private` — обычный preserve-on-omit (отсутствие аргумента сохраняет текущее значение, явное пустое значение/пустой список очищает); `since`/`until` — одиночные объектные поля: presence-ONLY guard — отсутствие ключа сохраняет текущее значение, явный `null` очищает (пустой объект `{}` не проходит валидацию, так что это единственный способ очистки) |
| `relation_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
| `residence_list` | Список проживаний; аргументы `person_id`/`place_id` (оба необязательны, пересекаются, если заданы вместе), `limit`, `offset`. Нет `residence_search` (см. `relation_list`) |
| `residence_get` | Проживание по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула |
| `residence_create` | Создание проживания: `person_id`/`place_id` (обязательны, строгие ссылки — `place_id` именно на `AdministrativeDivision`), `since`/`until`, `sources`, `note` (одна строка), `private`; id генерирует сервер |
| `residence_update` | Изменение проживания: `person_id`/`place_id` — REQUIRED, заменяются безусловно (как `relation_update`'s `person_a`/`person_b`); `sources`/`note`/`private` — preserve-on-omit; `since`/`until` — presence-only guard (см. `relation_update`) |
| `residence_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
| `event_list` | Список событий; аргумент `person_id` (необязателен — только события, где эта персона участвует), `limit`, `offset` |
| `event_search` | Поиск событий ТОЛЬКО по началу текста места (`place`) — единственное индексируемое поле события; аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `event_get` | Событие по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула |
| `event_create` | Создание события: `type` (обязателен, открытый набор), `date` (структурированная дата), `place` (объект `{text, ref?, type?}` — МЯГКАЯ ссылка, существование НИКОГДА не проверяется, только формат/тип-ограничение), `participants` (массив объектов `{person_id, role, note?}` — `person_id` СТРОГАЯ ссылка на персону, единственная в этом подпроекте строгая ссылка внутри массива объектов; несуществующая — ошибка тула на индексированное поле `participants[i].person_id`), `sources`, `notes`, `private`; id генерирует сервер |
| `event_update` | Изменение события: `type` — REQUIRED, заменяется безусловно; `participants`/`sources`/`notes`/`private` — preserve-on-omit (отсутствие аргумента сохраняет текущее значение, явный пустой список очищает); `date`/`place` — одиночные объектные поля: presence-ONLY guard — отсутствие ключа сохраняет текущее значение, явный `null` очищает. `place` — самое рискованное поле этого подпроекта по истории регрессии (см. коммит "fix: null должен снова очищать TextRef/FactDate-поля") — нужен ИМЕННО presence-only guard (`if _, ok := args["place"]; ok {…}`), не `raw != nil` |
| `event_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |

`<entity>_create`/`<entity>_update` у `division`/`repository`/`church`/`parish`/`archive`/`note`/`archive_node`/`archive_document`/`family`/`person`/`relation`/`residence`/`event` принимают аргумент `sources` — массив объектов `{citation_id, reliability?, role?, note?}` (первый MCP-аргумент вида «массив объектов» в программе, введён в подпроекте 5, см. `sourceLinkObjectProperties`; раньше массивы были только строками). Несуществующий `citation_id` — ошибка тула на поле `sources[i].citation_id`. `person_create`/`person_update` (подпроект 8) дополнительно принимают `names` — первый массив объектов, чьи собственные элементы тоже несут вложенные объекты (`surname`/`given`/`patronymic` — `TextRef`, `since`/`until` — `FactDate`, см. `personNameObjectProperties`). `event_create`/`event_update` (подпроект 9) принимают `participants` — массив объектов `{person_id, role, note?}`, без вложенных объектов (проще `sources`/`names`), но с первой в программе СТРОГОЙ (проверяемой на существование) ссылкой внутри массива объектов — `person_id`. `event_create`/`event_update` и `relation_create`/`relation_update`/`residence_create`/`residence_update` (подпроект 9) — первые тулы программы с одиночным объектным аргументом ЗА ПРЕДЕЛАМИ `TextRef`/`FactDate`: `place` у события — `PlaceRef` (`{text, ref?, type?}`, структурно как `TextRef`, но отдельный транспортный тип, т.к. `models.PlaceRef` — отдельный тип модели).
```

И дополнить (без изменений — переносится как есть) абзац «Семантика отсутствующего аргумента в `*_update`, единая для всей программы» сразу после этого блока — он уже сформулирован достаточно общо, чтобы не требовать правки под подпроект 9.

#### `docs/architecture.md` — строка `internal/httpapi/`

Заменить:

```markdown
| `internal/httpapi/` | Слой HTTP API: JSON-роуты `/api/*` (health, admin-divisions, surnames, patronymics, estates, titles, given-names, repositories, churches, parishes, archives, archive-nodes, archive-documents, notes, attachments, sources, citations, families, people, auth, docs). Использует те же сценарии, что и MCP. |
```
на:
```markdown
| `internal/httpapi/` | Слой HTTP API: JSON-роуты `/api/*` (health, admin-divisions, surnames, patronymics, estates, titles, given-names, repositories, churches, parishes, archives, archive-nodes, archive-documents, notes, attachments, sources, citations, families, people, relations, residences, events, auth, docs). Использует те же сценарии, что и MCP. |
```

#### `CHANGELOG.md` — новый пункт (после пункта про Person, перед «Веб: единая точка входа»)

Вставить перед пунктом «Веб: единая точка входа»:

```markdown
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
```

#### `docs/data-model/entity-write.md` — новая секция §3.8 (после §3.7, до `## 4.`)

Вставить новую секцию §3.8 после последнего пункта §3.7, до `## 4. Веб-UI конвенции`:

```markdown
### 3.8. Подпроект 9 (граф вокруг Person: `Relation`, `Residence`, `Event`) — backend, новые паттерны

Последний подпроект программы: три сущности, которые вместе образуют «граф
вокруг Person» — `Relation` (ребро родства/связи между двумя персонами),
`Residence` (проживание персоны в месте) и `Event`+`EventParticipant`
(событие с участниками). Этот раздел описывает только backend
(usecase-сценарии, транспорт, `httpapi`, MCP); веб-слой — отдельная задача.

- **Первая пара строгих ссылок на ОДИН И ТОТ ЖЕ тип.** `Relation.PersonA`/
  `.PersonB` — обе строгие ссылки на `Person`, а не на разные сущности (как
  `Archive.RepositoryID`, §3.1) или self-ref (как `Note.ParentID`, §3.2).
  `create_relation`/`update_relation` проверяют ОБЕ стороны независимо в
  одной транзакции — `person_a` не найден не мешает отдельно проверить и
  сообщить об отсутствии `person_b`, если тот тоже отсутствует (каждая
  проверка — свой ранний `return` с собственным полем ошибки). Различность
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
- **`EventParticipant.PersonID` — первая СТРОГАЯ ссылка внутри
  array-of-objects аргумента в программе.** До этого подпроекта все
  массивы объектов несли только мягкие или уже установленные ссылки:
  `SourceLink.CitationID` (подпроект 5) — строгая, но это плоский скаляр
  сбоку от массива-объекта, а не поле ВНУТРИ элемента, требующее проверки
  наравне с остальными; `PersonName.Surname/.Given/.Patronymic` (подпроект
  8) — мягкие `TextRef`, существование не проверяется вовсе. `Participants[
  i].PersonID` — первый случай, когда каждый элемент массива объектов сам
  несёт поле, которое usecase обязан проверить в транзакции, с
  индексированной ошибкой на элемент: `create_event`/`update_event` идут
  циклом `for i, p := range e.Participants { tx.GetPerson(...) }`,
  результат — `*models.ValidationError{Field: fmt.Sprintf(
  "participants[%d].person_id", i)}` (тот же приём индексации, что и
  `sourceLinkErr`, только на другом поле массива). Цикл по `Participants`
  и цикл по `Sources` — независимы (оба нужны, разные массивы, разные
  проверяемые сущности).
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
```

Это ровно то, что уже присутствует в проверочном worktree на момент написания этого плана (преамбула §3.8 отражает состояние ПОСЛЕ Задачи 1, ДО Задачи 2 — «этот раздел описывает только backend […]; веб-слой — отдельная задача») — Шаг 2.8 ниже заменяет именно преамбулу на описание реально построенного веб-слоя, отдельной вставкой поверх Шага 1.9, а не переписыванием его заново здесь.

#### `docs/data-model/entity-write.md` — правка §5 (два абзаца: picker персоны и `[]EventParticipant`)

Заменить существующий абзац про picker для `TextRef.Ref`:

```markdown
- Picker для `TextRef.Ref` (выбор ссылки на произвольную сущность) —
  отложен до реальной необходимости (подпроект 9 и позже).
```
на:
```markdown
- Picker для `TextRef.Ref` (выбор ссылки на произвольную сущность) —
  отложен до реальной необходимости; актуален и после подпроекта 9
  (`Person`-picker для `Relation.PersonA`/`.PersonB`, `Residence.PersonID`,
  `EventParticipant.PersonID` пригодился бы веб-форме этих сущностей).
```

И заменить абзац про `[]PersonName`/`[]EventParticipant`:

```markdown
- Вложенная подформа Person (`[]PersonName`) реализована целиком в
  подпроекте 8 — backend в §3.7, веб (`PersonNameListEditor`,
  `PersonForm`/`PersonView`) там же. Участники Event
  (`[]EventParticipant`) — детальный дизайн по-прежнему откладывается до
  подпроекта 9, когда до него дойдёт очередь; общие конвенции §3-4 всё
  равно применяются как основа.
```
на:
```markdown
- Вложенная подформа Person (`[]PersonName`) реализована целиком в
  подпроекте 8 — backend в §3.7, веб (`PersonNameListEditor`,
  `PersonForm`/`PersonView`) там же. Участники Event
  (`[]EventParticipant`) — backend завершён подпроектом 9 (§3.8: строгая
  проверка `person_id`, транспорт, `httpapi`, MCP); веб-слой
  (`EventParticipantListEditor` и страницы `Relation`/`Residence`/`Event`)
  — отдельная последующая задача, вне этого прохода.
```
И это тоже — состояние ПОСЛЕ Задачи 1, ДО Задачи 2; Шаг 2.8 правит фразу «отдельная последующая задача, вне этого прохода» ещё раз, когда веб реально готов.

### 1.10. Рубеж

`gofmt -l .` пусто; `go build ./...`, `go vet ./...`, `go test ./...` (весь репозиторий) зелёные — ожидается 1482 теста, 140 пакетов (было 1341 после подпроекта 8). Живой смок через `curl` (см. «Предпосылка») — не обязателен повторно: создать `Relation kind=blood` между двумя реальными персонами → round-trip; 422 на несуществующем `person_b`; создать `Residence` в реальном `AdministrativeDivision` → round-trip; 422 на несуществующем `place_id`; создать `Event` с двумя `participants`, второй указывает на несуществующую персону → 422 именно на `participants[1].person_id`; создать `Event` с фиктивным (несуществующим) `place.ref` → 201, `place` round-trip'ится без проверки; `PUT` `private:false → true` на `Event` → анонимный `GET` 200 → 404; `event_search` находит по `place`-префиксу, не находит по середине слова; MCP `event_update` без `place` сохраняет текущее значение, с `"place": null` — очищает.

### 1.11. Коммит

```bash
git add \
  internal/models/query.go \
  internal/transport/place_ref.go internal/transport/event_participant.go \
  internal/transport/relation.go internal/transport/relation_write.go \
  internal/transport/residence.go internal/transport/residence_write.go \
  internal/transport/event.go internal/transport/event_write.go \
  internal/usecases/list_relations internal/usecases/get_relation internal/usecases/create_relation internal/usecases/update_relation internal/usecases/delete_relation \
  internal/usecases/list_residences internal/usecases/get_residence internal/usecases/create_residence internal/usecases/update_residence internal/usecases/delete_residence \
  internal/usecases/list_events internal/usecases/search_events internal/usecases/get_event internal/usecases/create_event internal/usecases/update_event internal/usecases/delete_event \
  internal/httpapi/relation.go internal/httpapi/relation_write.go internal/httpapi/relation_test.go internal/httpapi/relation_write_test.go \
  internal/httpapi/residence.go internal/httpapi/residence_write.go internal/httpapi/residence_test.go internal/httpapi/residence_write_test.go \
  internal/httpapi/event.go internal/httpapi/event_write.go internal/httpapi/event_test.go internal/httpapi/event_write_test.go \
  internal/mcp/relation.go internal/mcp/relation_test.go internal/mcp/residence.go internal/mcp/residence_test.go internal/mcp/event.go internal/mcp/event_test.go \
  internal/mcp/object_args.go \
  internal/httpapi/deps.go internal/httpapi/api.go internal/httpapi/httpapi.go internal/mcp/deps.go internal/mcp/server.go internal/app/app.go \
  internal/httpapi/store_test.go internal/httpapi/write_store_test.go \
  docs/usage.md docs/architecture.md CHANGELOG.md docs/data-model/entity-write.md
git commit -m "feat(backend): Relation/Residence/Event — полный CRUD (usecases/httpapi/mcp) + доки + real-store тесты"
```

## Задача 2. Веб: `PersonPicker` + `AdminDivision`-picker + `EventParticipantListEditor` + 9 страниц + роуты/каталог

**Интерфейсы, потребляемые из Задачи 1**: HTTP-контракты `/api/relations*`, `/api/residences*`, `/api/events*` (Шаги 1.4, 1.7). **Интерфейсы, потребляемые с веба подпроектов 1-8**: `TextRefListEditor`, `SourceLinkListEditor`, `FactDateEditor` (подпроект 3), общий каркас страницы-тройки по образцу `PeopleList.tsx`/`PersonForm.tsx`/`PersonView.tsx` (подпроект 8) и `FamiliesList.tsx`/`FamilyForm.tsx`/`FamilyView.tsx` (подпроект 7), `fetchPeople`/`MAX_PAGE_LIMIT` (`api.ts`, подпроект 8 — `Person` впервые получил List/Search, что и делает возможным `PersonPicker`), ref-preservation-логика одиночного `*TextRef`/`*PlaceRef` из `ChurchView.tsx` (подпроект 3) — источник идеи для `EventView`'s обработки `place`.

**Производит**: `PersonPicker`+`usePersonOptions` (`web/src/PersonPicker.tsx`), `EventParticipantListEditor` (`web/src/EventParticipantList.tsx`), страницы `/relations`, `/relations/:id`, `/residences`, `/residences/:id`, `/events`, `/events/:id`, три строки в каталоге сущностей.

**Файлы:**
- Изменить: `web/src/api.ts`, `web/src/App.tsx`, `web/src/pages/EntityCatalog.tsx`, `web/src/PersonNameList.tsx`, `web/src/pages/PeopleList.tsx`, `web/src/pages/PersonView.tsx` (последние три — несущественный побочный рефакторинг подпроекта 8, см. Шаг 2.7 и «Предпосылка»), `CHANGELOG.md`, `docs/data-model/entity-write.md` (докный шаг 2.8 — см. «Предпосылка», случай (a): доки Задачи 1 описывают только бэкенд)
- Создать: `web/src/PersonPicker.tsx`, `web/src/EventParticipantList.tsx`, `web/src/pages/{RelationForm,RelationsList,RelationView,ResidenceForm,ResidencesList,ResidenceView,EventForm,EventsList,EventView}.tsx`

**Важно для исполнителя**: `PersonPicker.tsx`/`EventParticipantList.tsx` — новые переиспользуемые паттерны, не копии существующих компонентов. Девять страниц — копия страниц `Relation`/`Residence`/`Event` из соответствующего каркаса `Person`/`Family` с добавлением `PersonPicker` (`person_a`/`person_b`/`person_id`), `useAdminDivisionOptions` (только `Residence.place_id`), `EventParticipantListEditor` (только `Event.participants`) и `place`-ref-preservation ТОЛЬКО в `EventView.tsx` (по образцу `ChurchView.tsx.onSave` — сохранить `ref`/`type`, если текст `place` не изменился по сравнению с загруженным значением; изменился — отправить `{text}` без `ref`/`type`). `RelationsList.tsx`/`ResidencesList.tsx` НЕ содержат `Input.Search` (нет `/search` у этих сущностей на бэкенде) — единственные списковые страницы в программе без строки поиска; `EventsList.tsx`, напротив, обычная списковая страница с поиском. Переносить код ниже как есть, файл за файлом — итоговое содержимое.

### 2.1. `PersonPicker` + `usePersonOptions` (создать)

`web/src/PersonPicker.tsx` — первый в программе переиспользуемый picker персоны, ставший возможным только теперь, когда `Person` (подпроект 8) наконец имеет `List`/`Search`. `usePersonOptions()` грузит до `MAX_PAGE_LIMIT` персон одним запросом и экспортируется ОТДЕЛЬНО от компонента `PersonPicker` — используется и им самим для опций `Select`, и напрямую View-страницами (`RelationView`/`ResidenceView`/`EventView`), которым нужно резолвить голый `person_id` в подпись/ссылку, а не предлагать выбор (тот же приём, что `ArchiveView.tsx` переиспользует `fetchRepositories` для обеих целей). `PersonPicker` — плоский `Select` с клиентским поиском (`showSearch`+`filterOption`), опции — `{value: p.id, label: personDisplayName(p)}` (использует общую формулу из Шага 2.7); используется в трёх местах этого подпроекта: `Relation.person_a`/`.person_b` (два picker'а на одной форме), `Residence.person_id`, каждая строка `Event.participants` (внутри `EventParticipantListEditor`, Шаг 2.2).

#### `web/src/PersonPicker.tsx` (создать)

```tsx
import { useEffect, useState } from "react";
import { Select } from "antd";
import { fetchPeople, MAX_PAGE_LIMIT, type Person } from "./api";
import { personDisplayName } from "./PersonNameList";

// usePersonOptions — постранично (до MAX_PAGE_LIMIT — Person уже имеет
// List/Search с подпроекта 8) грузит список персон. Экспортируется отдельно
// от PersonPicker (не только Select-опции), т.к. отображаемое имя по
// person_id нужно и в read-only View-страницах (Relation/Residence/Event) —
// по образцу useRepositoryOptions/ArchiveForm.tsx и повторного fetch'а
// репозиториев в ArchiveView.tsx для подписи, не только для picker'а.
export function usePersonOptions(): Person[] {
  const [people, setPeople] = useState<Person[]>([]);

  useEffect(() => {
    fetchPeople({ limit: MAX_PAGE_LIMIT })
      .then(setPeople)
      .catch(() => setPeople([]));
  }, []);

  return people;
}

// PersonPicker — первый в программе переиспользуемый picker персоны
// (подпроект 9: Relation.PersonA/PersonB — два picker'а на одной форме,
// Residence.PersonID, каждая строка Event.Participants,
// docs/data-model/entity-write.md §3.8). Плоский Select с клиентским
// поиском — тот же принцип "точечный picker под конкретную строгую
// FK-связь", что и useRepositoryOptions/useCitationOptions, но вынесен в
// общий файл, а не в форму одной сущности, т.к. используется в трёх разных
// сущностях этого подпроекта. Никакого дерева/модалки — не нужно, Person
// уже отдаёт плоский список.
export function PersonPicker({
  value,
  onChange,
  placeholder,
}: {
  value: string;
  onChange: (id: string) => void;
  placeholder?: string;
}) {
  const people = usePersonOptions();
  const options = people.map((p) => ({ value: p.id, label: personDisplayName(p) }));

  return (
    <Select
      style={{ width: 260 }}
      placeholder={placeholder ?? "Персона"}
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

### 2.2. `EventParticipantListEditor` (создать)

`web/src/EventParticipantList.tsx` — редактор `Event.Participants`: список строк `person_id` (`PersonPicker`) + `role` (обязательна на бэкенде) + `note` (необязательна). Структурно повторяет `SourceLinkListEditor` (список строк, добавить/убрать по индексу), но ПРОЩЕ `PersonNameListEditor` (подпроект 8): `person_id` — СТРОГАЯ ссылка на уже существующую персону, проверяемая на сервере, а не мягкий `TextRef` с ref-preservation-если-текст-не-менялся — значит никакой baseline-per-mount-логики `PersonNameListEditor` здесь не нужно (`docs/data-model/entity-write.md` §3.8: первая строгая ссылка внутри массива-объектов MCP/HTTP-аргумента в программе, но именно поэтому и самый простой веб-редактор из трёх array-of-objects редакторов программы — сложность у неё серверная, не клиентская).

#### `web/src/EventParticipantList.tsx` (создать)

```tsx
import { Button, Input, Space } from "antd";
import { MinusCircleOutlined, PlusOutlined } from "@ant-design/icons";
import type { EventParticipant } from "./api";
import { PersonPicker } from "./PersonPicker";

// EventParticipantListEditor — редактор Event.Participants: список строк
// person_id (PersonPicker) + role (обязательно на бэкенде, requireText) +
// note (необязательно). Структурно повторяет SourceLinkListEditor (список
// строк + добавить/убрать по индексу), но проще PersonNameListEditor
// (подпроект 8): person_id — СТРОГАЯ ссылка на уже существующую персону
// (models.EventParticipant.PersonID, проверяется на сервере), а не мягкий
// TextRef с ref-preservation-если-текст-не-менялся — значит никакой
// ChurchView-style "снимок при появлении строки" логики не нужно
// (docs/data-model/entity-write.md §3.8: первая строгая ссылка внутри
// массива-объектов MCP/HTTP аргумента в программе).
export function EventParticipantListEditor({
  value,
  onChange,
  addLabel,
}: {
  value: EventParticipant[];
  onChange: (next: EventParticipant[]) => void;
  addLabel: string;
}) {
  const setField = (i: number, patch: Partial<EventParticipant>) => {
    const next = value.slice();
    next[i] = { ...next[i], ...patch };
    onChange(next);
  };

  const remove = (i: number) => onChange(value.filter((_, idx) => idx !== i));

  const add = () => onChange([...value, { person_id: "", role: "" }]);

  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      {value.map((item, i) => (
        <Space key={i} wrap style={{ width: "100%" }}>
          <PersonPicker
            value={item.person_id}
            onChange={(v) => setField(i, { person_id: v })}
            placeholder="Персона"
          />
          <Input
            placeholder="Роль"
            value={item.role}
            onChange={(e) => setField(i, { role: e.target.value })}
            style={{ width: 160 }}
          />
          <Input
            placeholder="Заметка"
            value={item.note}
            onChange={(e) => setField(i, { note: e.target.value })}
            style={{ width: 200 }}
          />
          <MinusCircleOutlined onClick={() => remove(i)} />
        </Space>
      ))}
      <Button type="dashed" onClick={add} icon={<PlusOutlined />} block>
        {addLabel}
      </Button>
    </Space>
  );
}
```

### 2.3. Страницы `Relation`

`RelationForm.tsx` (`CreateRelationModal`) — `kind` (`Select`, закрытый набор), `rel_type` (`Input`, рендерится ТОЛЬКО при `kind="associate"` — условность зеркалирует `Relation.Validate()`'s серверное правило на клиенте, чисто UX, сервер всё равно перепроверяет), `person_a`/`person_b` (два `PersonPicker`, обычный React state вне antd `Form` — design judgment call №4 в «Предпосылке»), `since`/`until` (`FactDateEditor` x2), `sources` (`SourceLinkListEditor`), `notes` (`TextRefListEditor`), `private`. `RelationsList.tsx` — плоский список БЕЗ `Input.Search` (нет `/search`), офсетная пагинация до короткой страницы (тот же цикл, что `PeopleList`/`FamiliesList`, без ветки поиска), строка — `"<вид связи>: <PersonA> — <PersonB>"` через `usePersonOptions()`. `RelationView.tsx` — просмотр/редактирование toggle: показывает `kind`/`rel_type` (только если задан), обе стороны как рабочие ссылки на `/people/{id}`, период, источники, заметки, приватность; в режиме редактирования — та же форма, что `RelationForm`, встроенная инлайново (по образцу `FamilyView`/`PersonView`).

#### `web/src/pages/RelationForm.tsx` (создать)

```tsx
import { useState } from "react";
import { Alert, Checkbox, Form, Input, Modal, Select } from "antd";
import {
  createRelation,
  RELATION_KIND_OPTIONS,
  type FactDate,
  type Relation,
  type RelationKind,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError } from "../auth";
import { FactDateEditor } from "../FactDateEditor";
import { PersonPicker } from "../PersonPicker";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";

interface RelationFormValues {
  kind: RelationKind;
  rel_type?: string;
  private?: boolean;
}

// FORM_FIELDS — только настоящие Form.Item'ы этой формы; person_a/person_b
// управляются отдельным React-состоянием (PersonPicker вне antd Form, как и
// прочие *ListEditor'ы), так что 422 с field="person_a"/"person_b" не может
// быть привязан к конкретному контролу формы и уходит в общий Alert (тот же
// приём, что и для sources[i].citation_id в других формах программы).
const FORM_FIELDS: (keyof RelationFormValues)[] = ["kind", "rel_type"];

// CreateRelationModal — форма создания ребра графа родства. kind — закрытый
// enum (Select); rel_type виден и обязателен ТОЛЬКО при kind=associate
// (models.Relation.Validate — требует rel_type для associate, запрещает
// иначе) — условная видимость поля по дискриминатору, тот же принцип, что
// AnchorEditor.tsx, но проще: меняется видимость одного текстового поля, а
// не весь набор полей формы. person_a/person_b — первое использование
// PersonPicker (подпроект 9) — два независимых picker'а на одной форме,
// первая сущность программы с двумя строгими ссылками на один и тот же тип
// (docs/data-model/entity-write.md §3.8).
export function CreateRelationModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Relation) => void;
}) {
  const [form] = Form.useForm<RelationFormValues>();
  const kind = Form.useWatch("kind", form);
  const [personA, setPersonA] = useState("");
  const [personB, setPersonB] = useState("");
  const [since, setSince] = useState<FactDate | null>(null);
  const [until, setUntil] = useState<FactDate | null>(null);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setPersonA("");
    setPersonB("");
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

  const onFinish = async (values: RelationFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await createRelation({
        kind: values.kind,
        rel_type: values.kind === "associate" ? (values.rel_type ?? "").trim() : "",
        person_a: personA,
        person_b: personB,
        since,
        until,
        sources,
        notes,
        private: values.private ?? false,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof RelationFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить связь"
      open={open}
      onCancel={handleClose}
      onOk={() => form.submit()}
      okText="Создать"
      cancelText="Отмена"
      confirmLoading={submitting}
      destroyOnHidden
    >
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
      <Form form={form} layout="vertical" onFinish={onFinish} initialValues={{ kind: "blood", private: false }}>
        <Form.Item name="kind" label="Вид связи" rules={[{ required: true, message: "Выберите вид связи" }]}>
          <Select options={RELATION_KIND_OPTIONS} />
        </Form.Item>
        {kind === "associate" && (
          <Form.Item
            name="rel_type"
            label="Тип связи"
            rules={[{ required: true, whitespace: true, message: "Укажите тип связи" }]}
          >
            <Input placeholder="godparent / witness / neighbor / friend / colleague" />
          </Form.Item>
        )}
        <Form.Item label="Персона A" required>
          <PersonPicker value={personA} onChange={setPersonA} placeholder="Персона A" />
        </Form.Item>
        <Form.Item label="Персона Б" required>
          <PersonPicker value={personB} onChange={setPersonB} placeholder="Персона Б" />
        </Form.Item>
        <Form.Item label="Начало периода">
          <FactDateEditor value={since} onChange={setSince} addLabel="+ дата начала" />
        </Form.Item>
        <Form.Item label="Конец периода">
          <FactDateEditor value={until} onChange={setUntil} addLabel="+ дата конца" />
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

#### `web/src/pages/RelationsList.tsx` (создать)

```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, List, Spin } from "antd";
import { fetchRelations, MAX_PAGE_LIMIT, relationKindLabel, type Relation } from "../api";
import { useSession } from "../session";
import { usePersonOptions } from "../PersonPicker";
import { personDisplayName } from "../PersonNameList";
import { CreateRelationModal } from "./RelationForm";

// RelationsList — «Связи»: плоский список БЕЗ поиска (Relation не имеет
// /search — его search-индекс всегда пуст по конструкции,
// docs/data-model/entity-write.md §3.8) — пагинация по offset до короткой
// страницы, как у прочих плоских списков, просто без Input.Search и без
// онсёрч-подмены списка.
export default function RelationsList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<Relation[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const people = usePersonOptions();

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: Relation[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchRelations({ limit: MAX_PAGE_LIMIT, offset });
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

  const personLabel = (id: string) => {
    const p = people.find((person) => person.id === id);
    return p != null ? personDisplayName(p) : id;
  };

  const relationLabel = (r: Relation) => {
    const kind = relationKindLabel(r.kind) + (r.kind === "associate" && r.rel_type ? ` (${r.rel_type})` : "");
    return `${kind}: ${personLabel(r.person_a)} — ${personLabel(r.person_b)}`;
  };

  return (
    <>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "Связи" }]}
      />
      <Card
        title="Связи"
        extra={
          session != null ? (
            <Button type="primary" onClick={() => setCreateOpen(true)}>
              + добавить
            </Button>
          ) : undefined
        }
      >
        {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
        {loading ? (
          <Spin />
        ) : (
          <List
            dataSource={items}
            locale={{ emptyText: "Список пуст" }}
            renderItem={(r) => (
              <List.Item>
                <Link to={`/relations/${r.id}`}>{relationLabel(r)}</Link>
              </List.Item>
            )}
          />
        )}
        <CreateRelationModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(r) => {
            setCreateOpen(false);
            navigate(`/relations/${r.id}`);
          }}
        />
      </Card>
    </>
  );
}
```

#### `web/src/pages/RelationView.tsx` (создать)

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
  deleteRelation,
  fetchRelation,
  relationKindLabel,
  RELATION_KIND_OPTIONS,
  updateRelation,
  type FactDate,
  type Relation,
  type RelationKind,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { FactDateEditor, formatFactDate } from "../FactDateEditor";
import { personDisplayName } from "../PersonNameList";
import { PersonPicker, usePersonOptions } from "../PersonPicker";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";

interface EditFormValues {
  kind: RelationKind;
  rel_type?: string;
  private?: boolean;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["kind", "rel_type"];

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

// RelationView — просмотр связи, переключаемый в форму редактирования на
// той же странице (toggle+explicit-save, как FamilyView/PersonView).
// person_a/person_b показываются кликабельной ссылкой на /people/{id} с
// отображаемым именем (personDisplayName) — тот же приём, что
// repositoryName в ArchiveView.tsx, только со ссылкой. rel_type
// показывается/редактируется только при kind=associate.
export default function RelationView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [relation, setRelation] = useState<Relation | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const people = usePersonOptions();

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const kind = Form.useWatch("kind", form);
  const [personA, setPersonA] = useState("");
  const [personB, setPersonB] = useState("");
  const [since, setSince] = useState<FactDate | null>(null);
  const [until, setUntil] = useState<FactDate | null>(null);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (relationId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setRelation(null);
    fetchRelation(relationId)
      .then(setRelation)
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

  const personLabel = (personId: string) => {
    const p = people.find((person) => person.id === personId);
    return p != null ? personDisplayName(p) : personId;
  };

  const startEdit = () => {
    if (relation == null) {
      return;
    }
    form.setFieldsValue({
      kind: relation.kind,
      rel_type: relation.rel_type ?? "",
      private: relation.private,
    });
    setPersonA(relation.person_a);
    setPersonB(relation.person_b);
    setSince(relation.since ?? null);
    setUntil(relation.until ?? null);
    setNotes(relation.notes);
    setSources(relation.sources);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (relation == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await updateRelation(relation.id, {
        kind: values.kind,
        rel_type: values.kind === "associate" ? (values.rel_type ?? "").trim() : "",
        person_a: personA,
        person_b: personB,
        since,
        until,
        sources,
        notes,
        private: values.private ?? false,
      });
      setRelation(updated);
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
    if (relation == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteRelation(relation.id);
      navigate("/relations");
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
          <Link to="/relations">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (relation == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  const title = `${relationKindLabel(relation.kind)}: ${personLabel(relation.person_a)} — ${personLabel(relation.person_b)}`;

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/relations">Связи</Link> },
          { title },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={title} column={1} bordered size="small">
            <Descriptions.Item label="Вид связи">{relationKindLabel(relation.kind)}</Descriptions.Item>
            {relation.kind === "associate" && (
              <Descriptions.Item label="Тип связи">{relation.rel_type || "—"}</Descriptions.Item>
            )}
            <Descriptions.Item label="Персона A">
              <Link to={`/people/${relation.person_a}`}>{personLabel(relation.person_a)}</Link>
            </Descriptions.Item>
            <Descriptions.Item label="Персона Б">
              <Link to={`/people/${relation.person_b}`}>{personLabel(relation.person_b)}</Link>
            </Descriptions.Item>
            <Descriptions.Item label="Начало периода">{formatFactDate(relation.since)}</Descriptions.Item>
            <Descriptions.Item label="Конец периода">{formatFactDate(relation.until)}</Descriptions.Item>
            <Descriptions.Item label="Заметки"><TextRefListView items={relation.notes} /></Descriptions.Item>
            <Descriptions.Item label="Доказательства"><SourceLinkListView items={relation.sources} /></Descriptions.Item>
            <Descriptions.Item label="Приватная">{relation.private ? "да" : "нет"}</Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title="Удалить связь?"
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
          <Form.Item name="kind" label="Вид связи" rules={[{ required: true, message: "Выберите вид связи" }]}>
            <Select options={RELATION_KIND_OPTIONS} />
          </Form.Item>
          {kind === "associate" && (
            <Form.Item
              name="rel_type"
              label="Тип связи"
              rules={[{ required: true, whitespace: true, message: "Укажите тип связи" }]}
            >
              <Input placeholder="godparent / witness / neighbor / friend / colleague" />
            </Form.Item>
          )}
          <Form.Item label="Персона A" required>
            <PersonPicker value={personA} onChange={setPersonA} placeholder="Персона A" />
          </Form.Item>
          <Form.Item label="Персона Б" required>
            <PersonPicker value={personB} onChange={setPersonB} placeholder="Персона Б" />
          </Form.Item>
          <Form.Item label="Начало периода">
            <FactDateEditor value={since} onChange={setSince} addLabel="+ дата начала" />
          </Form.Item>
          <Form.Item label="Конец периода">
            <FactDateEditor value={until} onChange={setUntil} addLabel="+ дата конца" />
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

### 2.4. Страницы `Residence`

`ResidenceForm.tsx` (`CreateResidenceModal`) — `person_id` (`PersonPicker`), `place_id` (`useAdminDivisionOptions()`, локальный хук этого файла — первый picker, точечно нацеленный на `AdministrativeDivision` как строгую FK-цель, не общий `TextRef`-словарь; см. design judgment call №3 в «Предпосылке» — НЕ вынесен в общий файл по явной инструкции задания), `since`/`until`, `sources`, `note` (ОДНА `Input.TextArea`, не список — единственное текстовое поле подпроекта в форме одиночной строки, не списка), `private`. `ResidencesList.tsx` — плоский список БЕЗ `Input.Search`, строка — `"<Person> — <Place>"`. `ResidenceView.tsx` — просмотр/редактирование toggle: `person_id`/`place_id` как рабочие ссылки на `/people/{id}`/`/divisions/{id}`, `useAdminDivisionOptions` продублирован инлайново (не импортируется из `ResidenceForm.tsx` — тот же прецедент, что `ArchiveView.tsx` не импортирует `ArchiveForm.tsx`'s `useRepositoryOptions`).

#### `web/src/pages/ResidenceForm.tsx` (создать)

```tsx
import { useEffect, useState } from "react";
import { Alert, Checkbox, Form, Input, Modal, Select } from "antd";
import {
  adminDivisionTypeLabel,
  createResidence,
  fetchAdminDivisions,
  MAX_PAGE_LIMIT,
  type AdminDivision,
  type FactDate,
  type Residence,
  type SourceLink,
} from "../api";
import { ApiError } from "../auth";
import { FactDateEditor } from "../FactDateEditor";
import { PersonPicker } from "../PersonPicker";
import { SourceLinkListEditor } from "../SourceLinkList";

interface ResidenceFormValues {
  note?: string;
  private?: boolean;
}

const FORM_FIELDS: (keyof ResidenceFormValues)[] = ["note"];

// useAdminDivisionOptions — заполняет Select административных делений для
// Residence.PlaceID — первая СТРОГАЯ (существование проверяется на сервере)
// ссылка на AdministrativeDivision в программе (все прочие ссылки на место
// остаются мягким TextRef, docs/data-model/entity-write.md §3.8). Точечный
// хук + инлайн-Select, не отдельный переиспользуемый компонент (в отличие
// от PersonPicker) — единственная точка использования в этом подпроекте,
// по образцу useRepositoryOptions/ArchiveForm.tsx.
function useAdminDivisionOptions() {
  const [divisions, setDivisions] = useState<AdminDivision[]>([]);

  useEffect(() => {
    fetchAdminDivisions({ limit: MAX_PAGE_LIMIT })
      .then(setDivisions)
      .catch(() => setDivisions([]));
  }, []);

  return divisions.map((d) => ({ value: d.id, label: `${d.name} (${adminDivisionTypeLabel(d.type)})` }));
}

// CreateResidenceModal — форма создания проживания персоны в месте.
// person_id — PersonPicker (подпроект 9). place_id — Select административных
// делений (useAdminDivisionOptions). note — ОДНА строка
// (models.Residence.Note), простой Input.TextArea, а не TextRefListEditor,
// в отличие от notes большинства сущностей (docs/data-model/entity-write.md
// §3.8).
export function CreateResidenceModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Residence) => void;
}) {
  const [form] = Form.useForm<ResidenceFormValues>();
  const [personId, setPersonId] = useState("");
  const [placeId, setPlaceId] = useState("");
  const [since, setSince] = useState<FactDate | null>(null);
  const [until, setUntil] = useState<FactDate | null>(null);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const divisionOptions = useAdminDivisionOptions();

  const reset = () => {
    form.resetFields();
    setPersonId("");
    setPlaceId("");
    setSince(null);
    setUntil(null);
    setSources([]);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: ResidenceFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await createResidence({
        person_id: personId,
        place_id: placeId,
        since,
        until,
        sources,
        note: (values.note ?? "").trim(),
        private: values.private ?? false,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof ResidenceFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить проживание"
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
        <Form.Item label="Персона" required>
          <PersonPicker value={personId} onChange={setPersonId} placeholder="Персона" />
        </Form.Item>
        <Form.Item label="Место" required>
          <Select
            style={{ width: 320 }}
            placeholder="Административное деление"
            options={divisionOptions}
            value={placeId || undefined}
            onChange={setPlaceId}
            showSearch
            filterOption={(input, option) =>
              (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
            }
          />
        </Form.Item>
        <Form.Item label="Начало периода">
          <FactDateEditor value={since} onChange={setSince} addLabel="+ дата начала" />
        </Form.Item>
        <Form.Item label="Конец периода">
          <FactDateEditor value={until} onChange={setUntil} addLabel="+ дата конца" />
        </Form.Item>
        <Form.Item name="note" label="Заметка">
          <Input.TextArea rows={3} />
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

#### `web/src/pages/ResidencesList.tsx` (создать)

```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, List, Spin } from "antd";
import {
  adminDivisionTypeLabel,
  fetchAdminDivisions,
  fetchResidences,
  MAX_PAGE_LIMIT,
  type AdminDivision,
  type Residence,
} from "../api";
import { useSession } from "../session";
import { usePersonOptions } from "../PersonPicker";
import { personDisplayName } from "../PersonNameList";
import { CreateResidenceModal } from "./ResidenceForm";

// ResidencesList — «Проживания»: плоский список БЕЗ поиска (Residence, как
// Relation, не имеет /search — пустой search-индекс по конструкции,
// docs/data-model/entity-write.md §3.8) — пагинация по offset до короткой
// страницы.
export default function ResidencesList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<Residence[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [divisions, setDivisions] = useState<AdminDivision[]>([]);
  const people = usePersonOptions();

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: Residence[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchResidences({ limit: MAX_PAGE_LIMIT, offset });
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

  useEffect(() => {
    fetchAdminDivisions({ limit: MAX_PAGE_LIMIT })
      .then(setDivisions)
      .catch(() => setDivisions([]));
  }, []);

  const personLabel = (id: string) => {
    const p = people.find((person) => person.id === id);
    return p != null ? personDisplayName(p) : id;
  };

  const placeLabel = (id: string) => {
    const d = divisions.find((division) => division.id === id);
    return d != null ? `${d.name} (${adminDivisionTypeLabel(d.type)})` : id;
  };

  return (
    <>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "Проживания" }]}
      />
      <Card
        title="Проживания"
        extra={
          session != null ? (
            <Button type="primary" onClick={() => setCreateOpen(true)}>
              + добавить
            </Button>
          ) : undefined
        }
      >
        {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
        {loading ? (
          <Spin />
        ) : (
          <List
            dataSource={items}
            locale={{ emptyText: "Список пуст" }}
            renderItem={(r) => (
              <List.Item>
                <Link to={`/residences/${r.id}`}>
                  {personLabel(r.person_id)} — {placeLabel(r.place_id)}
                </Link>
              </List.Item>
            )}
          />
        )}
        <CreateResidenceModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(r) => {
            setCreateOpen(false);
            navigate(`/residences/${r.id}`);
          }}
        />
      </Card>
    </>
  );
}
```

#### `web/src/pages/ResidenceView.tsx` (создать)

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
  adminDivisionTypeLabel,
  deleteResidence,
  fetchAdminDivisions,
  fetchResidence,
  MAX_PAGE_LIMIT,
  updateResidence,
  type AdminDivision,
  type FactDate,
  type Residence,
  type SourceLink,
} from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { FactDateEditor, formatFactDate } from "../FactDateEditor";
import { personDisplayName } from "../PersonNameList";
import { PersonPicker, usePersonOptions } from "../PersonPicker";
import { SourceLinkListEditor } from "../SourceLinkList";

interface EditFormValues {
  note?: string;
  private?: boolean;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["note"];

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

// ResidenceView — просмотр проживания, переключаемый в форму редактирования
// на той же странице (toggle+explicit-save). person_id — ссылка на
// /people/{id} с отображаемым именем; place_id — ссылка на
// /divisions/{id} с названием деления (обе — по образцу
// repositoryName/ArchiveView.tsx). note — одна строка, Input.TextArea.
export default function ResidenceView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [residence, setResidence] = useState<Residence | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const people = usePersonOptions();
  const [divisions, setDivisions] = useState<AdminDivision[]>([]);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [personId, setPersonId] = useState("");
  const [placeId, setPlaceId] = useState("");
  const [since, setSince] = useState<FactDate | null>(null);
  const [until, setUntil] = useState<FactDate | null>(null);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (residenceId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setResidence(null);
    fetchResidence(residenceId)
      .then(setResidence)
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
    fetchAdminDivisions({ limit: MAX_PAGE_LIMIT })
      .then(setDivisions)
      .catch(() => setDivisions([]));
  }, []);

  const divisionOptions = divisions.map((d) => ({
    value: d.id,
    label: `${d.name} (${adminDivisionTypeLabel(d.type)})`,
  }));

  const personLabel = (personId2: string) => {
    const p = people.find((person) => person.id === personId2);
    return p != null ? personDisplayName(p) : personId2;
  };

  const placeLabel = (placeId2: string) => {
    const d = divisions.find((division) => division.id === placeId2);
    return d != null ? `${d.name} (${adminDivisionTypeLabel(d.type)})` : placeId2;
  };

  const startEdit = () => {
    if (residence == null) {
      return;
    }
    form.setFieldsValue({ note: residence.note ?? "", private: residence.private });
    setPersonId(residence.person_id);
    setPlaceId(residence.place_id);
    setSince(residence.since ?? null);
    setUntil(residence.until ?? null);
    setSources(residence.sources);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (residence == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await updateResidence(residence.id, {
        person_id: personId,
        place_id: placeId,
        since,
        until,
        sources,
        note: (values.note ?? "").trim(),
        private: values.private ?? false,
      });
      setResidence(updated);
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
    if (residence == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteResidence(residence.id);
      navigate("/residences");
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
          <Link to="/residences">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (residence == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  const title = `${personLabel(residence.person_id)} — ${placeLabel(residence.place_id)}`;

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/residences">Проживания</Link> },
          { title },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={title} column={1} bordered size="small">
            <Descriptions.Item label="Персона">
              <Link to={`/people/${residence.person_id}`}>{personLabel(residence.person_id)}</Link>
            </Descriptions.Item>
            <Descriptions.Item label="Место">
              <Link to={`/divisions/${residence.place_id}`}>{placeLabel(residence.place_id)}</Link>
            </Descriptions.Item>
            <Descriptions.Item label="Начало периода">{formatFactDate(residence.since)}</Descriptions.Item>
            <Descriptions.Item label="Конец периода">{formatFactDate(residence.until)}</Descriptions.Item>
            <Descriptions.Item label="Заметка">{residence.note || "—"}</Descriptions.Item>
            <Descriptions.Item label="Доказательства"><SourceLinkListView items={residence.sources} /></Descriptions.Item>
            <Descriptions.Item label="Приватная">{residence.private ? "да" : "нет"}</Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title="Удалить проживание?"
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
          <Form.Item label="Персона" required>
            <PersonPicker value={personId} onChange={setPersonId} placeholder="Персона" />
          </Form.Item>
          <Form.Item label="Место" required>
            <Select
              style={{ width: 320 }}
              placeholder="Административное деление"
              options={divisionOptions}
              value={placeId || undefined}
              onChange={setPlaceId}
              showSearch
              filterOption={(input, option) =>
                (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
              }
            />
          </Form.Item>
          <Form.Item label="Начало периода">
            <FactDateEditor value={since} onChange={setSince} addLabel="+ дата начала" />
          </Form.Item>
          <Form.Item label="Конец периода">
            <FactDateEditor value={until} onChange={setUntil} addLabel="+ дата конца" />
          </Form.Item>
          <Form.Item name="note" label="Заметка">
            <Input.TextArea rows={3} />
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

### 2.5. Страницы `Event`

`EventForm.tsx` (`CreateEventModal`) — `type` (`Input`, открытый набор — просто текст, без `Select` с фиксированными опциями), `date` (`FactDateEditor`), `place` — только текстовый `Input`, БЕЗ пикера (`Event.Place.Ref` не имеет UI-picker'а по дизайну — та же формулировка, что у `Church.Parish` в подпроекте 3: `ref`/`type` можно проставить только через API/MCP напрямую, форма создания их не отправляет), `participants` (`EventParticipantListEditor`), `sources`, `notes`, `private`. `EventsList.tsx` — обычный список С `Input.Search` (Event ЕСТЬ поиск, единственная из трёх сущностей подпроекта) по префиксу текста `place`, экспортирует `eventLabel()` — переиспользуется `EventView.tsx`; строка списка — `"<type> — <place text> — <date>"`. `EventView.tsx` — просмотр/редактирование toggle, `place` обрабатывается ИМЕННО как `ChurchView.tsx.onSave` (Шаг 3.1 подпроекта 3): если текст `place` в форме совпадает с загруженным текстом — при сохранении отправляется исходный объект `{text, ref, type}` целиком (сохраняя `ref`); если текст изменился — отправляется `{text}` без `ref`/`type` (осознанно теряя ссылку). Единственная точка в этом подпроекте, где ref-preservation вообще нужна (participants — строгие ссылки, не мягкие; person_a/person_b/person_id — тоже строгие).

#### `web/src/pages/EventForm.tsx` (создать)

```tsx
import { useState } from "react";
import { Alert, Checkbox, Form, Input, Modal } from "antd";
import {
  createEvent,
  type Event,
  type EventParticipant,
  type FactDate,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError } from "../auth";
import { EventParticipantListEditor } from "../EventParticipantList";
import { FactDateEditor } from "../FactDateEditor";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";

interface EventFormValues {
  type: string;
  placeText?: string;
  private?: boolean;
}

const FORM_FIELDS: (keyof EventFormValues)[] = ["type"];

// CreateEventModal — форма создания события жизненного факта. type — простой
// Input (открытый enum, как ArchiveNode.type — не Select). place — простой
// текст (Event.Place — PlaceRef, НИКОГДА не проверяется на существование на
// сервере, docs/data-model/entity-write.md §3.8) — при создании ref всегда
// отсутствует, поэтому просто {text}/null без сохранения ссылки (сохранение
// ref-если-текст-не-менялся нужно только в EventView при редактировании,
// ChurchView-style). participants — EventParticipantListEditor (подпроект 9,
// первая строгая ссылка внутри массива-объектов).
export function CreateEventModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Event) => void;
}) {
  const [form] = Form.useForm<EventFormValues>();
  const [date, setDate] = useState<FactDate | null>(null);
  const [participants, setParticipants] = useState<EventParticipant[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setDate(null);
    setParticipants([]);
    setNotes([]);
    setSources([]);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: EventFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const placeText = (values.placeText ?? "").trim();
      const created = await createEvent({
        type: values.type,
        date,
        place: placeText ? { text: placeText } : null,
        participants,
        sources,
        notes,
        private: values.private ?? false,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof EventFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить событие"
      open={open}
      onCancel={handleClose}
      onOk={() => form.submit()}
      okText="Создать"
      cancelText="Отмена"
      confirmLoading={submitting}
      destroyOnHidden
      width={720}
    >
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
      <Form form={form} layout="vertical" onFinish={onFinish} initialValues={{ private: false }}>
        <Form.Item
          name="type"
          label="Вид события"
          rules={[{ required: true, whitespace: true, message: "Укажите вид события" }]}
        >
          <Input placeholder="birth / death / marriage / burial / confession / census" autoFocus />
        </Form.Item>
        <Form.Item label="Дата">
          <FactDateEditor value={date} onChange={setDate} addLabel="+ дата" />
        </Form.Item>
        <Form.Item name="placeText" label="Место (текстом)">
          <Input placeholder="село Давыдово" />
        </Form.Item>
        <Form.Item label="Участники">
          <EventParticipantListEditor value={participants} onChange={setParticipants} addLabel="+ участник" />
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

#### `web/src/pages/EventsList.tsx` (создать)

```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Input, List, Spin } from "antd";
import { fetchEvents, searchEvents, MAX_PAGE_LIMIT, type Event } from "../api";
import { useSession } from "../session";
import { formatFactDate } from "../FactDateEditor";
import { CreateEventModal } from "./EventForm";

// eventLabel — вид события + место (если есть) + дата (если есть), по
// образцу personNameLine/PersonView.tsx — короткая строка для списка и
// для заголовка EventView.
export function eventLabel(e: Event): string {
  const parts = [e.type];
  if (e.place?.text) {
    parts.push(e.place.text);
  }
  const date = formatFactDate(e.date);
  if (date !== "—") {
    parts.push(date);
  }
  return parts.join(" — ");
}

// EventsList — «События»: единственная сущность подпроекта 9 с /search
// (место-текст-префиксный поиск, docs/data-model/entity-write.md §3.8) —
// структурно идентична FamiliesList/PeopleList (поиск временно подменяет
// список найденным).
export default function EventsList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<Event[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<Event[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: Event[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchEvents({ limit: MAX_PAGE_LIMIT, offset });
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
    searchEvents({ q, limit: MAX_PAGE_LIMIT })
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
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "События" }]}
      />
      <Card
        title="События"
        extra={
          session != null ? (
            <Button type="primary" onClick={() => setCreateOpen(true)}>
              + добавить
            </Button>
          ) : undefined
        }
      >
        <Input.Search
          placeholder="Поиск по началу текста места…"
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
            renderItem={(e) => (
              <List.Item>
                <Link to={`/events/${e.id}`}>{eventLabel(e)}</Link>
              </List.Item>
            )}
          />
        )}
        <CreateEventModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(e) => {
            setCreateOpen(false);
            navigate(`/events/${e.id}`);
          }}
        />
      </Card>
    </>
  );
}
```

#### `web/src/pages/EventView.tsx` (создать)

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
  deleteEvent,
  fetchEvent,
  updateEvent,
  type Event,
  type EventParticipant,
  type FactDate,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { EventParticipantListEditor } from "../EventParticipantList";
import { FactDateEditor, formatFactDate } from "../FactDateEditor";
import { personDisplayName } from "../PersonNameList";
import { usePersonOptions } from "../PersonPicker";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";
import { eventLabel } from "./EventsList";

interface EditFormValues {
  type: string;
  placeText?: string;
  private?: boolean;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["type"];

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

// ParticipantListView — read-only отображение Event.Participants (личность
// участника — ссылка на /people/{id} с отображаемым именем, роль и заметка
// подписью рядом); редактируется EventParticipantListEditor в форме ниже.
function ParticipantListView({
  items,
  personLabel,
}: {
  items: EventParticipant[];
  personLabel: (id: string) => string;
}) {
  if (items.length === 0) {
    return <Typography.Text type="secondary">—</Typography.Text>;
  }
  return (
    <List
      size="small"
      dataSource={items}
      renderItem={(p, i) => (
        <List.Item key={i}>
          <Link to={`/people/${p.person_id}`}>{personLabel(p.person_id)}</Link>
          {` — ${p.role}`}
          {p.note ? ` (${p.note})` : ""}
        </List.Item>
      )}
    />
  );
}

// EventView — просмотр события, переключаемый в форму редактирования на той
// же странице (toggle+explicit-save). place показывается текстом; если у
// него уже есть ref — вторичная пометка «→ Type ID» (не кликабельно, picker
// не реализован — Place никогда не проверяется на существование на сервере,
// docs/data-model/entity-write.md §3.8). Сохранение ref при неизменном
// тексте — ТОЧНО тот же приём, что ChurchView.onSave (parishText/
// parishHasRef), просто переименованный под PlaceRef/place.
export default function EventView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [event, setEvent] = useState<Event | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const people = usePersonOptions();

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [date, setDate] = useState<FactDate | null>(null);
  const [participants, setParticipants] = useState<EventParticipant[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (eventId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setEvent(null);
    fetchEvent(eventId)
      .then(setEvent)
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

  const personLabel = (personId: string) => {
    const p = people.find((person) => person.id === personId);
    return p != null ? personDisplayName(p) : personId;
  };

  const startEdit = () => {
    if (event == null) {
      return;
    }
    form.setFieldsValue({
      type: event.type,
      placeText: event.place?.text ?? "",
      private: event.private,
    });
    setDate(event.date ?? null);
    setParticipants(event.participants);
    setNotes(event.notes);
    setSources(event.sources);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (event == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const placeText = (values.placeText ?? "").trim();
      const placeHasRef = event.place?.ref != null && event.place.ref !== "";
      const updated = await updateEvent(event.id, {
        type: values.type,
        date,
        // Сохраняем существующую ссылку, если поле не тронуто (тот же текст)
        // и уже несло ref; иначе — чистый текст без ref (picker не
        // реализован — по образцу ChurchView.onSave/parishText).
        place: placeHasRef && placeText === event.place?.text
          ? event.place
          : placeText
            ? { text: placeText }
            : null,
        participants,
        sources,
        notes,
        private: values.private ?? false,
      });
      setEvent(updated);
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
    if (event == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteEvent(event.id);
      navigate("/events");
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
          <Link to="/events">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (event == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  const title = eventLabel(event);

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/events">События</Link> },
          { title },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={title} column={1} bordered size="small">
            <Descriptions.Item label="Вид события">{event.type}</Descriptions.Item>
            <Descriptions.Item label="Дата">{formatFactDate(event.date)}</Descriptions.Item>
            <Descriptions.Item label="Место">
              {event.place == null ? (
                "—"
              ) : (
                <>
                  {event.place.text}
                  {event.place.ref && (
                    <Typography.Text type="secondary"> → {event.place.type} {event.place.ref}</Typography.Text>
                  )}
                </>
              )}
            </Descriptions.Item>
            <Descriptions.Item label="Участники">
              <ParticipantListView items={event.participants} personLabel={personLabel} />
            </Descriptions.Item>
            <Descriptions.Item label="Заметки"><TextRefListView items={event.notes} /></Descriptions.Item>
            <Descriptions.Item label="Доказательства"><SourceLinkListView items={event.sources} /></Descriptions.Item>
            <Descriptions.Item label="Приватная">{event.private ? "да" : "нет"}</Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title="Удалить событие?"
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
        <Form form={form} layout="vertical" onFinish={onSave} style={{ maxWidth: 720 }}>
          {saveError != null && (
            <Alert type="error" showIcon message={saveError} style={{ marginBottom: 16 }} />
          )}
          <Form.Item
            name="type"
            label="Вид события"
            rules={[{ required: true, whitespace: true, message: "Укажите вид события" }]}
          >
            <Input placeholder="birth / death / marriage / burial / confession / census" />
          </Form.Item>
          <Form.Item label="Дата">
            <FactDateEditor value={date} onChange={setDate} addLabel="+ дата" />
          </Form.Item>
          <Form.Item name="placeText" label="Место (текстом)">
            <Input placeholder="село Давыдово" />
          </Form.Item>
          <Form.Item label="Участники">
            <EventParticipantListEditor value={participants} onChange={setParticipants} addLabel="+ участник" />
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

### 2.6. `api.ts`, `App.tsx`, `EntityCatalog.tsx` — типы/функции, роуты, каталог

`api.ts` — добавлены `Relation`/`RelationKind`/`RelationInput`/`RelationQuery`, `Residence`/`ResidenceInput`/`ResidenceQuery`, `PlaceRef`, `EventParticipant`, `Event`/`EventInput`/`EventQuery`/`EventSearchQuery` + `fetchRelations`/`fetchRelation`/`createRelation`/`updateRelation`/`deleteRelation` (без `searchRelations`), тот же 5-функциональный набор для `Residence` (без `searchResidences`), все 6 для `Event` (включая `searchEvents`) — те же конвенции, что уже существующие секции `Person`/`Family` (`ApiError`, `MAX_PAGE_LIMIT`, `URLSearchParams`-паттерн построения query-строки). `App.tsx` — шесть новых маршрутов (`/relations`, `/relations/:id`, `/residences`, `/residences/:id`, `/events`, `/events/:id`), тот же раздельный список/просмотр-паттерн, что у `Family`/`Person`, без отдельного маршрута создания (создание — модалка со списковой страницы). `EntityCatalog.tsx` — три новые алфавитно-упорядоченные строки: «Проживания» (`/residences`), «Связи» (`/relations`), «События» (`/events`).

#### `web/src/api.ts` (изменить — итоговое содержимое)

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

export interface Family {
  id: string;
  name: string;
  members: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

// FamilyInput — тело POST/PUT /api/families (transport.FamilyCreate и
// transport.FamilyUpdate имеют одинаковую форму: полная замена всех полей,
// включая sources — редактируется с рождения контракта, docs/data-model/
// entity-write.md §3.3/§3.6).
export interface FamilyInput {
  name: string;
  members: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

export interface FamilyQuery {
  limit?: number;
  offset?: number;
}

export interface FamilySearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchFamilies(query: FamilyQuery = {}): Promise<Family[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/families${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchFamilies(query: FamilySearchQuery): Promise<Family[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/families/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchFamily(id: string): Promise<Family> {
  return authFetch<Family>(`/api/families/${encodeURIComponent(id)}`);
}

export async function createFamily(input: FamilyInput): Promise<Family> {
  return authFetch<Family>("/api/families", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateFamily(id: string, input: FamilyInput): Promise<Family> {
  return authFetch<Family>(`/api/families/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteFamily(id: string): Promise<void> {
  return authFetch<void>(`/api/families/${encodeURIComponent(id)}`, { method: "DELETE" });
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
// архивный узел (просто id — строка, не объект). document_id —
// необязательная мягкая ссылка (ON DELETE SET NULL). Оба поля в веб-форме
// заполняются через ArchiveNodePicker/ArchiveDocumentSelect (см.
// ArchiveNodePicker.tsx).
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

// PersonName — контракт одного имени персоны (transport.PersonName):
// вложенная подформа Person.names — вид имени + три мягкие ссылки на
// словари (Surname/GivenName/Patronymic, тот же TextRef, что и одиночные
// поля/списки TextRef в других сущностях — НЕ picker, docs/data-model/
// entity-write.md §3.7/§4) + служебные части имени (prefix/suffix) + период
// действия (FactDate since/until, тот же FactDateEditor, что у Parish).
export type PersonNameType = "" | "main" | "birth" | "married" | "changed" | "pseudonym";

export interface PersonName {
  type?: PersonNameType;
  surname: TextRef;
  given: TextRef;
  patronymic: TextRef;
  prefix?: string;
  suffix?: string;
  since?: FactDate | null;
  until?: FactDate | null;
}

// Person — персона, ядро графа генеалогии (transport.Person). gender —
// простая строка (не TextRef). names — первая в проекте вложенная подформа
// «массив объектов» (не массив строк/TextRef — PersonNameListEditor,
// docs/data-model/entity-write.md §3.7).
export type PersonGender = "" | "male" | "female" | "unknown";

// GENDER_OPTIONS/genderLabel — models.PersonGender (internal/models/person_gender.go),
// по образцу ADMIN_DIVISION_TYPE_LABELS/adminDivisionTypeLabel выше.
export const GENDER_OPTIONS: { value: PersonGender; label: string }[] = [
  { value: "", label: "Не указан" },
  { value: "male", label: "Мужской" },
  { value: "female", label: "Женский" },
  { value: "unknown", label: "Неизвестен" },
];

export function genderLabel(g: string | undefined): string {
  return GENDER_OPTIONS.find((o) => o.value === (g ?? ""))?.label ?? (g || "");
}

export interface Person {
  id: string;
  gender?: PersonGender;
  names: PersonName[];
  estates: TextRef[];
  titles: TextRef[];
  nicknames: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

// PersonInput — тело POST/PUT /api/people (transport.PersonCreate и
// transport.PersonUpdate имеют одинаковую форму: полная замена всех полей).
// names и sources — обязательные поля (полная замена), как и у всех прочих
// *Input с рождения контракта.
export interface PersonInput {
  gender?: PersonGender;
  names: PersonName[];
  estates: TextRef[];
  titles: TextRef[];
  nicknames: TextRef[];
  notes: TextRef[];
  sources: SourceLink[];
  private: boolean;
}

export interface PersonQuery {
  limit?: number;
  offset?: number;
}

export interface PersonSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchPeople(query: PersonQuery = {}): Promise<Person[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/people${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchPeople(query: PersonSearchQuery): Promise<Person[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/people/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchPerson(id: string): Promise<Person> {
  return authFetch<Person>(`/api/people/${encodeURIComponent(id)}`);
}

export async function createPerson(input: PersonInput): Promise<Person> {
  return authFetch<Person>("/api/people", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updatePerson(id: string, input: PersonInput): Promise<Person> {
  return authFetch<Person>(`/api/people/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deletePerson(id: string): Promise<void> {
  return authFetch<void>(`/api/people/${encodeURIComponent(id)}`, { method: "DELETE" });
}

// Relation — ребро графа родства между двумя персонами (transport.Relation).
// Первая сущность программы с двумя строгими ссылками на один и тот же тип
// (PersonA/PersonB → Person, docs/data-model/entity-write.md §3.8). Sources
// редактируется с рождения контракта — обязательный массив, как и у всех
// прочих *Input.
export type RelationKind = "blood" | "marriage" | "adoption" | "associate";

// RELATION_KIND_OPTIONS/relationKindLabel — models.RelationKind
// (internal/models/relation_kind.go), по образцу GENDER_OPTIONS/genderLabel.
export const RELATION_KIND_OPTIONS: { value: RelationKind; label: string }[] = [
  { value: "blood", label: "Родство" },
  { value: "marriage", label: "Брак" },
  { value: "adoption", label: "Усыновление" },
  { value: "associate", label: "Иная связь" },
];

export function relationKindLabel(k: string | undefined): string {
  return RELATION_KIND_OPTIONS.find((o) => o.value === k)?.label ?? (k || "");
}

export interface Relation {
  id: string;
  kind: RelationKind;
  rel_type?: string;
  person_a: string;
  person_b: string;
  since?: FactDate | null;
  until?: FactDate | null;
  sources: SourceLink[];
  notes: TextRef[];
  private: boolean;
}

// RelationInput — тело POST/PUT /api/relations (transport.RelationCreate и
// transport.RelationUpdate имеют одинаковую форму: полная замена всех полей).
export interface RelationInput {
  kind: RelationKind;
  rel_type?: string;
  person_a: string;
  person_b: string;
  since?: FactDate | null;
  until?: FactDate | null;
  sources: SourceLink[];
  notes: TextRef[];
  private: boolean;
}

export interface RelationQuery {
  person_id?: string;
  limit?: number;
  offset?: number;
}

// Нет searchRelations — search-индекс Relation всегда пуст, по конструкции
// (internal/store/sqlstore/relations.go: replaceSearchIndex(..., nil));
// person_id-фильтр на списке — предусмотренная замена свободного поиска
// (docs/data-model/entity-write.md §3.8).
export async function fetchRelations(query: RelationQuery = {}): Promise<Relation[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/relations${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchRelation(id: string): Promise<Relation> {
  return authFetch<Relation>(`/api/relations/${encodeURIComponent(id)}`);
}

export async function createRelation(input: RelationInput): Promise<Relation> {
  return authFetch<Relation>("/api/relations", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateRelation(id: string, input: RelationInput): Promise<Relation> {
  return authFetch<Relation>(`/api/relations/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteRelation(id: string): Promise<void> {
  return authFetch<void>(`/api/relations/${encodeURIComponent(id)}`, { method: "DELETE" });
}

// Residence — проживание персоны в месте (transport.Residence). place_id —
// первая СТРОГАЯ (существование проверяется на сервере) ссылка на
// AdministrativeDivision в программе — все прочие ссылки на деление
// остаются мягким TextRef (docs/data-model/entity-write.md §3.8). note —
// одна строка (models.Residence.Note), НЕ список TextRef, в отличие от
// notes у большинства сущностей.
export interface Residence {
  id: string;
  person_id: string;
  place_id: string;
  since?: FactDate | null;
  until?: FactDate | null;
  sources: SourceLink[];
  note?: string;
  private: boolean;
}

// ResidenceInput — тело POST/PUT /api/residences (transport.ResidenceCreate
// и transport.ResidenceUpdate имеют одинаковую форму: полная замена всех
// полей).
export interface ResidenceInput {
  person_id: string;
  place_id: string;
  since?: FactDate | null;
  until?: FactDate | null;
  sources: SourceLink[];
  note?: string;
  private: boolean;
}

export interface ResidenceQuery {
  person_id?: string;
  place_id?: string;
  limit?: number;
  offset?: number;
}

// Нет searchResidences — тот же структурный повод, что у Relation (пустой
// search-индекс по конструкции, docs/data-model/entity-write.md §3.8).
export async function fetchResidences(query: ResidenceQuery = {}): Promise<Residence[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/residences${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchResidence(id: string): Promise<Residence> {
  return authFetch<Residence>(`/api/residences/${encodeURIComponent(id)}`);
}

export async function createResidence(input: ResidenceInput): Promise<Residence> {
  return authFetch<Residence>("/api/residences", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateResidence(id: string, input: ResidenceInput): Promise<Residence> {
  return authFetch<Residence>(`/api/residences/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteResidence(id: string): Promise<void> {
  return authFetch<void>(`/api/residences/${encodeURIComponent(id)}`, { method: "DELETE" });
}

// PlaceRef — контракт «указания на место» (transport.PlaceRef): структурно
// идентичен TextRef ({text, ref, type}), но отдельный тип на бэкенде
// (models.PlaceRef != models.TextRef) — Event.Place, единственное текущее
// поле такой формы в программе. В форме остаётся простым текстом с
// preserve-ref-если-текст-не-менялся, по образцу ChurchView.parish (picker'а
// нет — Place никогда не проверяется на существование на сервере,
// docs/data-model/entity-write.md §3.8).
export interface PlaceRef {
  text: string;
  ref?: string;
  type?: string;
}

// EventParticipant — контракт одного участника события
// (transport.EventParticipant): флат-объект {person_id, role, note}. Первая
// строгая (проверяемая на существование) ссылка внутри массива-объектов
// HTTP/MCP-аргумента в программе (docs/data-model/entity-write.md §3.8).
export interface EventParticipant {
  person_id: string;
  role: string;
  note?: string;
}

// Event — событие жизненного факта (transport.Event). type — открытый enum
// (простой Input, как ArchiveNode.type — не Select). Поиск (searchEvents)
// ищет ТОЛЬКО по началу текста place.text (docs/data-model/entity-write.md
// §3.8) — единственная сущность подпроекта 9 с search-маршрутом.
export interface Event {
  id: string;
  type: string;
  date?: FactDate | null;
  place?: PlaceRef | null;
  participants: EventParticipant[];
  sources: SourceLink[];
  notes: TextRef[];
  private: boolean;
}

// EventInput — тело POST/PUT /api/events (transport.EventCreate и
// transport.EventUpdate имеют одинаковую форму: полная замена всех полей).
// participants и sources — обязательные массивы.
export interface EventInput {
  type: string;
  date?: FactDate | null;
  place?: PlaceRef | null;
  participants: EventParticipant[];
  sources: SourceLink[];
  notes: TextRef[];
  private: boolean;
}

export interface EventQuery {
  person_id?: string;
  limit?: number;
  offset?: number;
}

export interface EventSearchQuery {
  q: string;
  limit?: number;
  offset?: number;
}

export async function fetchEvents(query: EventQuery = {}): Promise<Event[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/events${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function searchEvents(query: EventSearchQuery): Promise<Event[]> {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined) {
      params.set(key, String(value));
    }
  }
  const qs = params.toString();
  const resp = await fetch(`/api/events/search${qs ? `?${qs}` : ""}`);
  if (!resp.ok) {
    throw new Error(`API error: ${resp.status}`);
  }
  return resp.json();
}

export async function fetchEvent(id: string): Promise<Event> {
  return authFetch<Event>(`/api/events/${encodeURIComponent(id)}`);
}

export async function createEvent(input: EventInput): Promise<Event> {
  return authFetch<Event>("/api/events", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateEvent(id: string, input: EventInput): Promise<Event> {
  return authFetch<Event>(`/api/events/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteEvent(id: string): Promise<void> {
  return authFetch<void>(`/api/events/${encodeURIComponent(id)}`, { method: "DELETE" });
}
```

#### `web/src/App.tsx` (изменить — итоговое содержимое)

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
import FamiliesList from "./pages/FamiliesList";
import FamilyView from "./pages/FamilyView";
import PeopleList from "./pages/PeopleList";
import PersonView from "./pages/PersonView";
import RelationsList from "./pages/RelationsList";
import RelationView from "./pages/RelationView";
import ResidencesList from "./pages/ResidencesList";
import ResidenceView from "./pages/ResidenceView";
import EventsList from "./pages/EventsList";
import EventView from "./pages/EventView";
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
          <Route path="/families" element={<PageLayout><FamiliesList /></PageLayout>} />
          <Route path="/families/:id" element={<PageLayout><FamilyView /></PageLayout>} />
          <Route path="/people" element={<PageLayout><PeopleList /></PageLayout>} />
          <Route path="/people/:id" element={<PageLayout><PersonView /></PageLayout>} />
          <Route path="/relations" element={<PageLayout><RelationsList /></PageLayout>} />
          <Route path="/relations/:id" element={<PageLayout><RelationView /></PageLayout>} />
          <Route path="/residences" element={<PageLayout><ResidencesList /></PageLayout>} />
          <Route path="/residences/:id" element={<PageLayout><ResidenceView /></PageLayout>} />
          <Route path="/events" element={<PageLayout><EventsList /></PageLayout>} />
          <Route path="/events/:id" element={<PageLayout><EventView /></PageLayout>} />
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
  { label: "Персоны", path: "/people" },
  { label: "Приходы", path: "/parishes" },
  { label: "Проживания", path: "/residences" },
  { label: "Роды", path: "/families" },
  { label: "Связи", path: "/relations" },
  { label: "События", path: "/events" },
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

### 2.7. Несущественный побочный рефакторинг подпроекта 8: `personDisplayName`

**Это НЕ новая функциональность подпроекта 9** — три файла ниже принадлежат подпроекту 8 (`Person`) и правятся здесь ТОЛЬКО потому, что `PersonPicker.tsx` (Шаг 2.1) стал ТРЕТЬИМ местом, которому нужна формула «главное имя персоны, иначе первое, иначе id»: `PeopleList.tsx` (`personLabel`) и `PersonView.tsx` (расчёт `title`) уже дублировали её каждый по-своему (см. «Предпосылка», design judgment call №1 веб-отчёта). Вместо третьего дубля — единая `personDisplayName(p)`, вынесенная в `web/src/PersonNameList.tsx` рядом с уже существующими `pickDisplayName`/`formatPersonName`, которые она композирует; `PeopleList.tsx` и `PersonView.tsx` переключены на импорт этой функции вместо собственных копий. Флагируется отдельным шагом специально для ревьюера — файлы из уже смёрженного подпроекта 8 меняются здесь не как регрессия объёма, а как целенаправленный, предусмотренный экономный рефакторинг.

#### `web/src/PersonNameList.tsx` (изменить — итоговое содержимое)

```tsx
import { useRef, useState } from "react";
import { Button, Divider, Input, Select, Space } from "antd";
import { MinusCircleOutlined, PlusOutlined } from "@ant-design/icons";
import type { Person, PersonName, PersonNameType, TextRef } from "./api";
import { FactDateEditor } from "./FactDateEditor";

// PERSON_NAME_TYPE_OPTIONS — models.PersonNameType (internal/models/person_name_type.go).
export const PERSON_NAME_TYPE_OPTIONS: { value: PersonNameType; label: string }[] = [
  { value: "", label: "Не указано" },
  { value: "main", label: "основное" },
  { value: "birth", label: "при рождении" },
  { value: "married", label: "по браку" },
  { value: "changed", label: "изменённое" },
  { value: "pseudonym", label: "псевдоним" },
];

export function personNameTypeLabel(t: string | undefined): string {
  return PERSON_NAME_TYPE_OPTIONS.find((o) => o.value === (t ?? ""))?.label ?? (t || "");
}

// formatPersonName — "Фамилия Имя Отчество" (пропускает пустые части),
// общая для PeopleList (строка списка) и PersonView (просмотр имени).
export function formatPersonName(n: PersonName): string {
  return [n.surname.text, n.given.text, n.patronymic.text]
    .map((s) => s.trim())
    .filter((s) => s !== "")
    .join(" ");
}

// pickDisplayName — имя типа "main", иначе первое из списка; null, если
// список пуст.
export function pickDisplayName(names: PersonName[]): PersonName | null {
  if (names.length === 0) {
    return null;
  }
  return names.find((n) => n.type === "main") ?? names[0];
}

// personDisplayName — единая формула отображаемого имени персоны (id как
// fallback, если имён нет или все части пустые): была продублирована как
// personLabel в PeopleList.tsx и как расчёт title в PersonView.tsx —
// вынесена сюда с подпроекта 9 (PersonPicker.tsx — первый общий picker,
// docs/data-model/entity-write.md §3.8), оба места теперь используют эту
// функцию вместо собственных копий.
export function personDisplayName(p: Person): string {
  const name = pickDisplayName(p.names);
  const formatted = name != null ? formatPersonName(name) : "";
  return formatted !== "" ? formatted : p.id;
}

const EMPTY_TEXT_REF: TextRef = { text: "" };

function emptyPersonName(): PersonName {
  return {
    type: "",
    surname: { ...EMPTY_TEXT_REF },
    given: { ...EMPTY_TEXT_REF },
    patronymic: { ...EMPTY_TEXT_REF },
    prefix: "",
    suffix: "",
    since: null,
    until: null,
  };
}

interface RowMeta {
  id: number;
  // base — исходный (загруженный) PersonName этой строки на момент её
  // появления в редакторе: точка отсчёта для решения "сохранить ref или
  // сбросить" (ChurchView-style, docs/data-model/entity-write.md §4) —
  // берётся один раз при монтировании/добавлении строки, а не при каждом
  // изменении value, иначе ref терялся бы сразу после первого чтения.
  base: PersonName;
}

// PersonNameListEditor — редактор Person.names: первая в проекте вложенная
// подформа «массив объектов» (не массив строк/TextRef, как
// TextRefListEditor, и не массив с одной строгой ссылкой, как
// SourceLinkListEditor) — структурно та же форма "список строк, добавить/
// убрать", но каждая строка богаче: вид имени (Select) + три TextRef-поля
// (Фамилия/Имя/Отчество — простые Input, БЕЗ picker'а, тот же принцип, что
// Church.Parish: строгие FK получают Select, мягкий TextRef — нет) + два
// служебных текстовых поля (Приставка/Суффикс) + период действия
// (FactDateEditor since/until, тот же компонент, что у Parish).
//
// Сохранение ref при неизменном тексте — независимо для surname/given/
// patronymic внутри каждой строки, по образцу ChurchView.onSave
// (parishText/parishHasRef): если текст поля не отличается от того, что
// было при появлении строки в редакторе, и то исходное значение несло ref —
// наружу уходит исходный объект {text, ref, type} без изменений; иначе —
// свежий {text}, без ref/type (пользователь напечатал что-то другое —
// прежняя ссылка на конкретную словарную запись больше не действует).
export function PersonNameListEditor({
  value,
  onChange,
  addLabel,
}: {
  value: PersonName[];
  onChange: (next: PersonName[]) => void;
  addLabel: string;
}) {
  // Снимок «во что редактор был инициализирован» — один раз при монтировании
  // (компонент пересоздаётся при каждом входе в режим редактирования/
  // открытии модалки создания, см. PersonForm.tsx/PersonView.tsx, так что
  // это соответствует «на момент появления строки»).
  const [rowMeta, setRowMeta] = useState<RowMeta[]>(() => value.map((n, i) => ({ id: i, base: n })));
  const nextId = useRef(value.length);

  const setRow = (i: number, patch: Partial<PersonName>) => {
    const next = value.slice();
    next[i] = { ...next[i], ...patch };
    onChange(next);
  };

  const setTextRefField = (i: number, field: "surname" | "given" | "patronymic", text: string) => {
    const base = rowMeta[i]?.base;
    const baseField = base ? base[field] : undefined;
    const hasRef = baseField != null && baseField.ref != null && baseField.ref !== "";
    const nextField: TextRef = hasRef && text === baseField.text ? baseField : { text };
    setRow(i, { [field]: nextField } as Partial<PersonName>);
  };

  const add = () => {
    const fresh = emptyPersonName();
    onChange([...value, fresh]);
    setRowMeta([...rowMeta, { id: nextId.current++, base: fresh }]);
  };

  const remove = (i: number) => {
    onChange(value.filter((_, idx) => idx !== i));
    setRowMeta(rowMeta.filter((_, idx) => idx !== i));
  };

  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      {value.map((item, i) => (
        <div key={rowMeta[i]?.id ?? i}>
          <Space direction="vertical" style={{ width: "100%" }}>
            <Space wrap style={{ width: "100%" }}>
              <Select
                style={{ width: 160 }}
                placeholder="Вид имени"
                options={PERSON_NAME_TYPE_OPTIONS}
                value={item.type ?? ""}
                onChange={(v) => setRow(i, { type: v })}
              />
              <Input
                placeholder="Фамилия"
                value={item.surname.text}
                onChange={(e) => setTextRefField(i, "surname", e.target.value)}
                style={{ width: 160 }}
              />
              <Input
                placeholder="Имя"
                value={item.given.text}
                onChange={(e) => setTextRefField(i, "given", e.target.value)}
                style={{ width: 160 }}
              />
              <Input
                placeholder="Отчество"
                value={item.patronymic.text}
                onChange={(e) => setTextRefField(i, "patronymic", e.target.value)}
                style={{ width: 160 }}
              />
              <Input
                placeholder="Приставка"
                value={item.prefix}
                onChange={(e) => setRow(i, { prefix: e.target.value })}
                style={{ width: 120 }}
              />
              <Input
                placeholder="Суффикс"
                value={item.suffix}
                onChange={(e) => setRow(i, { suffix: e.target.value })}
                style={{ width: 120 }}
              />
              <MinusCircleOutlined onClick={() => remove(i)} />
            </Space>
            <Space wrap style={{ width: "100%" }}>
              <FactDateEditor
                value={item.since ?? null}
                onChange={(since) => setRow(i, { since })}
                addLabel="+ действует с"
              />
              <FactDateEditor
                value={item.until ?? null}
                onChange={(until) => setRow(i, { until })}
                addLabel="+ действует по"
              />
            </Space>
          </Space>
          <Divider style={{ margin: "12px 0" }} />
        </div>
      ))}
      <Button type="dashed" onClick={add} icon={<PlusOutlined />} block>
        {addLabel}
      </Button>
    </Space>
  );
}
```

#### `web/src/pages/PeopleList.tsx` (изменить — итоговое содержимое)

```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Input, List, Spin } from "antd";
import { fetchPeople, searchPeople, MAX_PAGE_LIMIT, type Person } from "../api";
import { useSession } from "../session";
import { personDisplayName } from "../PersonNameList";
import { CreatePersonModal } from "./PersonForm";

// PeopleList — «Персоны»: плоский список (как FamiliesList — не иерархична),
// пагинация по offset до короткой страницы, поиск временно подменяет список
// найденным. Клик по строке — переход на View.
export default function PeopleList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [items, setItems] = useState<Person[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<Person[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const all: Person[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchPeople({ limit: MAX_PAGE_LIMIT, offset });
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
    searchPeople({ q, limit: MAX_PAGE_LIMIT })
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
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "Персоны" }]}
      />
      <Card
        title="Персоны"
        extra={
          session != null ? (
            <Button type="primary" onClick={() => setCreateOpen(true)}>
              + добавить
            </Button>
          ) : undefined
        }
      >
        <Input.Search
          placeholder="Поиск по фамилии/имени/отчеству…"
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
            renderItem={(p) => (
              <List.Item>
                <Link to={`/people/${p.id}`}>{personDisplayName(p)}</Link>
              </List.Item>
            )}
          />
        )}
        <CreatePersonModal
          open={createOpen}
          onClose={() => setCreateOpen(false)}
          onCreated={(p) => {
            setCreateOpen(false);
            navigate(`/people/${p.id}`);
          }}
        />
      </Card>
    </>
  );
}
```

#### `web/src/pages/PersonView.tsx` (изменить — итоговое содержимое)

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
  List,
  Modal,
  Popconfirm,
  Select,
  Space,
  Spin,
  Typography,
} from "antd";
import {
  deletePerson,
  fetchPerson,
  GENDER_OPTIONS,
  genderLabel,
  updatePerson,
  type Person,
  type PersonGender,
  type PersonName,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { formatFactDate } from "../FactDateEditor";
import { formatPersonName, personDisplayName, personNameTypeLabel, PersonNameListEditor } from "../PersonNameList";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";

interface EditFormValues {
  gender?: PersonGender;
  private?: boolean;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["gender"];

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

// SourceLinkListView — read-only отображение списка доказательств (Sources);
// редактируется отдельным SourceLinkListEditor в форме ниже.
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

// personNameLine — вид имени + "Фамилия Имя Отчество" + приставка/суффикс,
// если заданы, + период действия (formatFactDate) — та же формула, что
// personLabel в PeopleList.tsx для одного имени, плюс служебные части и
// период для полного просмотра.
function personNameLine(n: PersonName): string {
  const parts: string[] = [];
  if (n.type !== "") {
    parts.push(personNameTypeLabel(n.type));
  }
  const core = formatPersonName(n);
  const namePiece = core !== "" ? core : "—";
  parts.push(n.prefix ? `${n.prefix} ${namePiece}` : namePiece);
  if (n.suffix) {
    parts.push(n.suffix);
  }
  const since = n.since != null ? formatFactDate(n.since) : null;
  const until = n.until != null ? formatFactDate(n.until) : null;
  if (since != null || until != null) {
    parts.push(`(${since ?? "…"} – ${until ?? "…"})`);
  }
  return parts.join(" — ");
}

// PersonNameListView — read-only отображение Person.names; редактируется
// PersonNameListEditor в форме ниже.
function PersonNameListView({ items }: { items: PersonName[] }) {
  if (items.length === 0) {
    return <Typography.Text type="secondary">—</Typography.Text>;
  }
  return (
    <List
      size="small"
      dataSource={items}
      renderItem={(n, i) => <List.Item key={i}>{personNameLine(n)}</List.Item>}
    />
  );
}

// PersonView — просмотр персоны, переключаемый в форму редактирования на
// той же странице (toggle+explicit-save, как FamilyView). Заголовок карточки
// — отображаемое имя (pickDisplayName), а не поле name (у Person его нет).
export default function PersonView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [person, setPerson] = useState<Person | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [names, setNames] = useState<PersonName[]>([]);
  const [estates, setEstates] = useState<TextRef[]>([]);
  const [titles, setTitles] = useState<TextRef[]>([]);
  const [nicknames, setNicknames] = useState<TextRef[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (personId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setPerson(null);
    fetchPerson(personId)
      .then(setPerson)
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
    if (person == null) {
      return;
    }
    form.setFieldsValue({
      gender: (person.gender ?? "") as PersonGender,
      private: person.private,
    });
    setNames(person.names);
    setEstates(person.estates);
    setTitles(person.titles);
    setNicknames(person.nicknames);
    setNotes(person.notes);
    setSources(person.sources);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (person == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await updatePerson(person.id, {
        gender: values.gender ?? "",
        names,
        estates,
        titles,
        nicknames,
        notes,
        sources,
        private: values.private ?? false,
      });
      setPerson(updated);
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
    if (person == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deletePerson(person.id);
      navigate("/people");
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
          <Link to="/people">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (person == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  const title = personDisplayName(person);

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/people">Персоны</Link> },
          { title },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={title} column={1} bordered size="small">
            <Descriptions.Item label="Пол">{genderLabel(person.gender)}</Descriptions.Item>
            <Descriptions.Item label="Имена"><PersonNameListView items={person.names} /></Descriptions.Item>
            <Descriptions.Item label="Сословия"><TextRefListView items={person.estates} /></Descriptions.Item>
            <Descriptions.Item label="Титулы"><TextRefListView items={person.titles} /></Descriptions.Item>
            <Descriptions.Item label="Прозвища"><TextRefListView items={person.nicknames} /></Descriptions.Item>
            <Descriptions.Item label="Заметки"><TextRefListView items={person.notes} /></Descriptions.Item>
            <Descriptions.Item label="Доказательства"><SourceLinkListView items={person.sources} /></Descriptions.Item>
            <Descriptions.Item label="Приватная">{person.private ? "да" : "нет"}</Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title={`Удалить «${title}»?`}
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
        <Form form={form} layout="vertical" onFinish={onSave} style={{ maxWidth: 720 }}>
          {saveError != null && (
            <Alert type="error" showIcon message={saveError} style={{ marginBottom: 16 }} />
          )}
          <Form.Item name="gender" label="Пол">
            <Select options={GENDER_OPTIONS} />
          </Form.Item>
          <Form.Item label="Имена">
            <PersonNameListEditor value={names} onChange={setNames} addLabel="+ имя" />
          </Form.Item>
          <Form.Item label="Сословия">
            <TextRefListEditor value={estates} onChange={setEstates} addLabel="+ сословие" />
          </Form.Item>
          <Form.Item label="Титулы">
            <TextRefListEditor value={titles} onChange={setTitles} addLabel="+ титул" />
          </Form.Item>
          <Form.Item label="Прозвища">
            <TextRefListEditor value={nicknames} onChange={setNicknames} addLabel="+ прозвище" />
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

### 2.8. Документация: расширить `CHANGELOG.md` и `entity-write.md` веб-частью

Явный докный шаг, добавленный по инструкции этого подпроекта (см. «Предпосылка», случай (a)) — `docs/usage.md`/`docs/architecture.md` НЕ трогаются (по конвенции репозитория не описывают веб-страницы вообще, см. Предпосылку и аналогичный вывод планов подпроектов 7-8); `CHANGELOG.md` и `docs/data-model/entity-write.md` дополняются, чтобы после Задачи 2 доки полностью описывали и бэкенд, и веб программы `entity-write` целиком — не только этот подпроект, а всю программу, поскольку это её последний подпроект.

#### `CHANGELOG.md` — дополнить пункт про граф вокруг Person (добавленный Шагом 1.9) веб-частью

Заменить последнее предложение пункта:

```markdown
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
```
на:
```markdown
  и `transport.EventParticipant` (плоский объект, новый файл по образцу
  `transport.PersonName`).
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
  `optionalEventParticipants`). Веб: общий переиспользуемый
  `PersonPicker`+`usePersonOptions` (`web/src/PersonPicker.tsx`, первый
  picker персоны в программе — возможен только теперь, когда `Person`
  имеет List/Search), `EventParticipantListEditor`
  (`web/src/EventParticipantList.tsx`, проще `PersonNameListEditor` —
  `person_id` строгая ссылка, не мягкий `TextRef`, ref-preservation не
  нужна), локальный `useAdminDivisionOptions` в `ResidenceForm.tsx`
  (первый picker, нацеленный именно на `AdministrativeDivision`); девять
  страниц (`web/src/pages/{RelationForm,RelationsList,RelationView,
  ResidenceForm,ResidencesList,ResidenceView,EventForm,EventsList,
  EventView}.tsx`) — `RelationsList`/`ResidencesList` без строки поиска
  (нет `/search` у этих сущностей), `EventView` — единственная страница
  подпроекта с ref-preservation (`place`, по образцу `ChurchView.onSave`);
  строки «Связи»/«Проживания»/«События» в каталоге сущностей
  (`web/src/api.ts`, `web/src/App.tsx`,
  `web/src/pages/EntityCatalog.tsx`). Побочно: `personDisplayName(p)`
  вынесена в `web/src/PersonNameList.tsx` (третье место, нуждающееся в
  формуле отображаемого имени персоны, после `PeopleList.tsx`/
  `PersonView.tsx` подпроекта 8 — оба переключены на неё вместо
  собственных копий). **Подпроект 9 — последний в программе
  `entity-write`: с его слиянием все 21 сущность домена получают полный
  CRUD через HTTP, MCP и веб.**
```

#### `docs/data-model/entity-write.md` — заменить преамбулу §3.8 (веб теперь готов)

Заменить преамбулу, добавленную Шагом 1.9:

```markdown
Последний подпроект программы: три сущности, которые вместе образуют «граф
вокруг Person» — `Relation` (ребро родства/связи между двумя персонами),
`Residence` (проживание персоны в месте) и `Event`+`EventParticipant`
(событие с участниками). Этот раздел описывает только backend
(usecase-сценарии, транспорт, `httpapi`, MCP); веб-слой — отдельная задача.
```
на:
```markdown
Последний подпроект программы: три сущности, которые вместе образуют «граф
вокруг Person» — `Relation` (ребро родства/связи между двумя персонами),
`Residence` (проживание персоны в месте) и `Event`+`EventParticipant`
(событие с участниками). Backend и веб построены и проверены двумя
последовательными живыми проходами (см. `BACKEND_REPORT.md`/
`WEB_REPORT.md` в проверочном worktree подпроекта); этот раздел описывает
оба. Веб добавляет первый в программе переиспользуемый picker персоны
(`PersonPicker`+`usePersonOptions`, `web/src/PersonPicker.tsx`) и
`EventParticipantListEditor` (`web/src/EventParticipantList.tsx`) — оба
описаны в последнем пункте этого раздела.
```

#### `docs/data-model/entity-write.md` — добавить пункт о вебе в конец §3.8

Добавить в конец §3.8 (после пункта про `transport.EventParticipant`, перед `## 4.`):

```markdown
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
```

#### `docs/data-model/entity-write.md` — заменить абзац §5 про `[]EventParticipant` (веб теперь готов)

Заменить абзац, изменённый Шагом 1.9:

```markdown
- Вложенная подформа Person (`[]PersonName`) реализована целиком в
  подпроекте 8 — backend в §3.7, веб (`PersonNameListEditor`,
  `PersonForm`/`PersonView`) там же. Участники Event
  (`[]EventParticipant`) — backend завершён подпроектом 9 (§3.8: строгая
  проверка `person_id`, транспорт, `httpapi`, MCP); веб-слой
  (`EventParticipantListEditor` и страницы `Relation`/`Residence`/`Event`)
  — отдельная последующая задача, вне этого прохода.
```
на:
```markdown
- Вложенные подформы Person (`[]PersonName`, подпроект 8) и Event
  (`[]EventParticipant`, подпроект 9) реализованы целиком, backend и веб —
  `PersonNameListEditor` (§3.7) и `EventParticipantListEditor` (§3.8)
  соответственно. Программа `entity-write` завершена подпроектом 9: все 21
  сущность домена имеют полный CRUD через HTTP, MCP и веб.
```

### 2.9. Рубеж

`cd web && npm run typecheck` (чисто), `npm run build` (чисто). Живая проверка (см. «Предпосылка») — не обязательна повторно: `/relations`/`/residences`/`/events` рендерятся плоскими списками (первые два — без строки поиска), «+ добавить» открывает соответствующую модалку с `PersonPicker`(-ами); создание `Relation kind=associate` показывает поле «Тип связи» ровно при этом значении `kind`; создание `Residence` показывает `useAdminDivisionOptions`-`Select` и `note` как `Input.TextArea`; создание `Event` с `EventParticipantListEditor` — минимум два участника с ролями; `EventView`'s ref-preservation для `place` — правка без изменения текста сохраняет `ref` (аннотация `→ …` остаётся), правка с изменением текста — `ref` очищается (аннотация исчезает); каталог сущностей — на три строки больше, «Проживания»/«Связи»/«События» в алфавитном порядке.

### 2.10. Коммит

```bash
git add web/src/PersonPicker.tsx web/src/EventParticipantList.tsx \
  web/src/pages/RelationForm.tsx web/src/pages/RelationsList.tsx web/src/pages/RelationView.tsx \
  web/src/pages/ResidenceForm.tsx web/src/pages/ResidencesList.tsx web/src/pages/ResidenceView.tsx \
  web/src/pages/EventForm.tsx web/src/pages/EventsList.tsx web/src/pages/EventView.tsx \
  web/src/api.ts web/src/App.tsx web/src/pages/EntityCatalog.tsx \
  web/src/PersonNameList.tsx web/src/pages/PeopleList.tsx web/src/pages/PersonView.tsx \
  CHANGELOG.md docs/data-model/entity-write.md
git commit -m "feat(web): страницы Relation/Residence/Event — PersonPicker, EventParticipantListEditor, каталог + доки"
```

## Рубеж прохода

После Задачи 2: `gofmt -l .` пусто, `go build/vet/test ./...` зелёные (1482 теста, 140 пакетов), `npm run typecheck`/`build` чисты. Каталог `/` — на три строки больше («Проживания»/«Связи»/«События», алфавитный порядок). `Relation`/`Residence`/`Event`+`EventParticipant` имеют полный CRUD через HTTP, MCP и веб, по конвенциям `docs/data-model/entity-write.md` §3-4 и новой §3.8, с тремя новыми программными паттернами (пара строгих ссылок на один тип, осознанный отказ от поиска у двух сущностей, строгая ссылка внутри массива объектов) и двумя новыми переиспользуемыми веб-компонентами (`PersonPicker`, `EventParticipantListEditor`) — подтверждено обоими отчётами живой проверки и обновлённой §3.8/§5 `entity-write.md`. `CHANGELOG.md` и `entity-write.md` после Задачи 2 описывают и бэкенд, и веб — пробел, отмеченный в «Предпосылке» (случай (a)), закрыт Шагом 2.8.

**Это последний подпроект программы `entity-write`.** После слияния этой ветки в `main` ВСЕ 21 сущность домена (`AdministrativeDivision`, `Surname`, `Patronymic`, `Estate`, `Title`, `GivenName`, `Repository`, `Church`, `Parish`, `Archive`, `ArchiveNode`, `ArchiveDocument`, `Note`, `Attachment`, `Source`, `Citation`, `Family`, `Person`, `Relation`, `Residence`, `Event`) имеют полный CRUD через HTTP API, MCP-тулы и веб-интерфейс, по единым конвенциям `docs/data-model/entity-write.md`. После обеих задач — обзор всей ветки целиком (`git diff main...HEAD` по объёму подпроекта), при необходимости волна точечных фиксов по результатам ревью, затем слияние в `main` — как и в предыдущих восьми подпроектах программы. Дальнейшая работа над записью данных (если потребуется) — это уже не декомпозиция `docs/data-model/entity-write.md` §2, а отдельная программа с собственным дизайн-документом: текущий список из 21 сущности исчерпан.

## Коммиты

Два коммита в `main`, по одному на задачу — см. Шаги 1.11, 2.10.
