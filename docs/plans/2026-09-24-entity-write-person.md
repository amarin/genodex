# Веб-CRUD/MCP для всех сущностей — подпроект 8 (Person): план

> **Для агента-исполнителя:** ОБЯЗАТЕЛЬНЫЙ ПОДНАВЫК: superpowers:subagent-driven-development.

Восьмой проход по декомпозиции `docs/data-model/entity-write.md` §2. Вводит `Person` — ядро графа генеалогии, самую связанную сущность программы на сегодня. В отличие от подпроекта 7 (`Family`, «самая механическая сущность программы») `Person` содержит настоящий новый элемент дизайна — вложенную подформу-«массив объектов» `Names []PersonName` (вид имени + три мягкие ссылки на словари `Surname`/`GivenName`/`Patronymic` + служебные части имени + период действия), не сводимую ни к списку `TextRef`, ни к одиночному полиморфному значению вроде `Citation.Anchor`. На бэкенде это второй в программе MCP-аргумент вида «массив объектов» (после `sources`, подпроект 5) и первый, чьи собственные элементы сами несут вложенные объекты. На вебе — новый переиспользуемый паттерн `PersonNameListEditor` (`web/src/PersonNameList.tsx`), первая в проекте повторяющаяся форма-редактор с несколькими текстовыми полями и датой на строку, требующая явного решения о том, как сохранять `ref`/`type` при частичном редактировании строки (см. «Предпосылка» и design judgment call №1 ниже).

## Goal

Полный CRUD (HTTP + MCP + веб) для `Person` — по конвенциям `docs/data-model/entity-write.md` §3-4, включая новую §3.7. Новый переиспользуемый элемент: вложенная подформа `[]PersonName` — на бэкенде (`transport.PersonName`, `personNameObjectProperties`/`optionalPersonNames`) и на вебе (`PersonNameListEditor`). Все остальные поля `Person` (`Estates`/`Titles`/`Nicknames`/`Notes` — списки `TextRef`, `Sources` — список `SourceLink`, `Private`) следуют уже закреплённым конвенциям без изменений.

## Архитектура

`Person` — `{ID, Gender, Names []PersonName, Estates []TextRef, Titles []TextRef, Nicknames []TextRef, Notes []TextRef, Sources []SourceLink, Private bool}`. Все поля, кроме `ID`, опциональны, включая `Gender` (пустая строка — «не указан», значения `male`/`female`/`unknown`). Единственная строго проверяемая ссылка — `Sources[i].CitationID` (`InTx` + `tx.GetCitation`, образец — `create_family`/`update_family`); `Person` access-aware при чтении по id (образец — `get_family`). `PersonName` — `{Type, Surname TextRef, Given TextRef, Patronymic TextRef, Prefix, Suffix, Since *FactDate, Until *FactDate}`: `Type` — вид имени (`main`/`birth`/`married`/`changed`/`pseudonym`, пусто — не указан); `Surname`/`Given`/`Patronymic` — мягкие ссылки на `Surname`/`GivenName`/`Patronymic` (`TextRef.Ref`, если задан, проверяется только по формату, СУЩЕСТВОВАНИЕ НЕ ПРОВЕРЯЕТСЯ — тот же принцип, что и у любого другого `TextRef` в программе, несмотря на то что все три словаря имеют собственный CRUD с подпроектов 1-2); `Prefix`/`Suffix` — простые строки; `Since`/`Until` — `FactDate`. Модель (`internal/models/person.go`, `person_name_validate.go`) уже существует и не меняется этим подпроектом — требует хотя бы одну из трёх частей имени непустой. Поиск (`personTerms`, `internal/store/sqlstore/person.go`, уже существующий generic-код) индексирует `Surname.Text`/`Given.Text`/`Patronymic.Text` из КАЖДОГО элемента `Names`, не только «основного», под одним полем `name` — см. §3.7 ниже и «Глобальные ограничения». На вебе — обычная страница-тройка (List/View+Form), но `Names` редактируется новым `PersonNameListEditor` (`web/src/PersonNameList.tsx`), а не существующим `TextRefListEditor`/`SourceLinkListEditor`.

## Технологии

Backend: Go, `internal/models` (домен, `Person`/`PersonName`/`PersonValidate`/`PersonNameValidate` — уже существуют, не меняются) → `internal/usecases/<scenario>` → `internal/transport` (DTO) → `internal/httpapi` (JSON REST) / `internal/mcp` (`github.com/mark3labs/mcp-go`, Streamable HTTP) → `internal/store`/`internal/store/sqlstore` (generic-хранилище, уже готово для всех 21 типа, в этом проходе не меняется — `store.Store.ListPeople` и `personTerms` существовали до этого подпроекта). Frontend: Vite + React 18 + TypeScript + antd, файлы-страницы по образцу уже существующих (`FamiliesList`/`FamilyForm`/`FamilyView`), плюс новый компонент-редактор `PersonNameList.tsx` по образцу `ChurchView.tsx`'s ref-preservation логики для одиночного `Parish`, но для повторяющихся строк. Тесты: стандартный `go test` (usecases — фейковый стор в памяти; `internal/httpapi/write_store_test.go` — реальный SQLite), `npm run typecheck`/`npm run build` на вебе.

## Спецификация

Источник истины дизайна — `docs/data-model/entity-write.md` (копия из проверочного worktree, см. «Предпосылка» — копия на `main` этого раздела не содержит §3.7): §3.7 «Подпроект 8 (`Person`) — вложенная подформа `PersonName`, backend» — новые бэкенд-паттерны (нет нового вида FK, но первая вложенная подформа-«массив объектов», второй MCP-массив объектов в программе и первый с вложенными объектами внутри своих же элементов, иррегулярное наименование `list_people`/`search_people`, поиск, агрегирующий несколько повторяющихся дочерних записей под одним полем индекса); §4 «Веб-UI конвенции» — общие правила List/View/Create/Delete, которым без отступлений следуют новые страницы `Person` (плоский список, без иерархии). §3.7's последний пункт («Backend-only подпроект») и §5's абзац про `[]PersonName` в момент написания этого плана всё ещё описывают веб-часть как отдельную будущую работу — Задача 2 ниже это выполняет и Шаг 2.4 приводит оба места в соответствие с тем, что реально появилось (см. «Предпосылка», подраздел про доки).

## Глобальные ограничения

Правила программы `entity-write`, обязательные для каждой новой сущности (дословно из `entity-write.md` и правил, закреплённых финальными ревью подпроектов 1-7); подпроект 8 не отменяет ни одного из них:

- **Access-aware `Get` для любой сущности с `Private`.** `get_person` обязан принимать `access models.Access` и прятать приватную запись как отсутствующую для не-владельца: `if rec.Private && access != models.AccessFull { return models.ErrNotFound }` (образец — `internal/usecases/get_family/scenario.go`).
- **Real-store тесты обязательны.** Каждая пишущая сущность получает сквозной тест на реальном SQLite-сторе в `internal/httpapi/write_store_test.go` (не только фейковый стор в usecase-тестах) — здесь `TestPersonWriteContractWithRealStore` (Шаг 1.7).
- **Fetch-then-merge на каждом update.** `update_person` читает текущую версию через `GetPerson`, накладывает новые поля, валидирует и сохраняет через `SavePerson` — полная замена записи, не частичный patch (свойство самого generic-стора, см. `entity-write.md` §1).
- **MCP «отсутствие необязательного списочного аргумента — значит сохранить как есть».** Для `sources` И (новое для этого подпроекта) для `names` в `person_update`: аргумент не передан — текущее значение сохраняется; передан пустым списком `[]` — очищает. `estates`/`titles`/`nicknames`/`notes` остаются на общем v1-ограничении программы (§5) — заменяются целиком при каждом `person_update`, без presence-check. HTTP `PUT`, как обычно, всегда полностью заменяет тело — отсутствие ключа в JSON очищает соответствующее поле.
- **Формулировка описания поиска обязана совпадать с реальным списком полей, индексируемых generic-слоем.** `person_search`/`GET /api/people/search` должны буквально описывать, что индексируется `Surname.Text`/`Given.Text`/`Patronymic.Text` из КАЖДОГО элемента `Names` (не только «основного»), под полем `name` (`personTerms`, `internal/store/sqlstore/person.go`), и что `Estates`/`Titles`/`Nicknames`/`Notes` поиском не охвачены. Расхождение однажды уже поймано финальным ревью подпроекта 4 (`Note`) — с тех пор проверяется явно при каждом новом подпроекте; здесь урок качественно сложнее обычного (не «одно поле вместо всех текстовых», а «поле агрегирует несколько повторяющихся дочерних записей», см. §3.7).
- **Мягкая ссылка (`TextRef`) никогда не проверяется на существование при сохранении, независимо от того, есть ли у цели свой CRUD-слой.** `Names[i].Surname/.Given/.Patronymic` указывают на `Surname`/`GivenName`/`Patronymic`, у которых CRUD есть с подпроектов 1-2 — и всё равно существование не проверяется: принцип действует с самого первого появления `TextRef` в программе, не специфика конкретной цели ссылки. То же верно для `Estates`/`Titles` (мягкая ссылка на словари `Estate`/`Title`).
- **Доки (`docs/usage.md`, `docs/architecture.md`, `CHANGELOG.md`, `docs/data-model/entity-write.md`) обязаны описывать оба поставленных куска — и бэкенд, и веб — до конца прохода, не только на момент коммита Задачи 1.** Это правило впервые сформулировано явно именно в этом подпроекте (см. «Предпосылка»): у `Family` (подпроект 7) весь стек строился одним проходом, поэтому финальные доки Задачи 1 уже покрывали и веб; здесь бэкенд и веб — две последовательные живые проверки, и докам Задачи 1 (написанным ДО того, как веб-часть вообще была спроектирована) это правило выполнить неоткуда — поэтому Задача 2 получает собственный докный шаг (2.4), приводящий `CHANGELOG.md` и `entity-write.md` в состояние «описывают всё, что было поставлено».

## Предпосылка: живая проверка

Весь код ниже применён и проверен в отдельном throwaway git worktree (`/Users/asmarin/dev/mine/genodex-verify-subproject8`, ветка `verify/entity-write-subproject8`, форкнута от `main`) — не на `main` напрямую и не в этом плане с нуля: он транскрибирован из уже рабочего дерева, а не написан заново. Проверка бэкенда и веба проведена двумя последовательными живыми проходами (сперва бэкенд, затем веб поверх него), задокументированными двумя отдельными отчётами в этом же worktree: `/Users/asmarin/dev/mine/genodex-verify-subproject8/BACKEND_REPORT.md` и `/Users/asmarin/dev/mine/genodex-verify-subproject8/WEB_REPORT.md`. Существенное из обоих перенесено сюда, чтобы будущий читатель плана понимал не только ЧТО делает код ниже, но и ПОЧЕМУ он выглядит именно так.

**Проверка бэкенда** (`BACKEND_REPORT.md`): `gofmt -l .` пусто, `go build/vet/test ./...` зелёные — **1293 теста, 119 пакетов** (было 1252 на `main` до этого подпроекта — **+41** новый тест: list_people/search_people/get_person/create_person/update_person/delete_person, `internal/httpapi/person_test.go`/`person_write_test.go`, `internal/httpapi/write_store_test.go` (`TestPersonWriteContractWithRealStore`), `internal/mcp/person_test.go`). Живой смок-тест через `curl` (реальный `genodex serve`, чистая БД, owner зарегистрирован через `POST /api/auth/register`): (1) создание с несуществующим `sources[0].citation_id` → 422 на этом поле; (2) создание с двумя `Names` (`main` и `married`), каждое со своим мягким `ref` на `Surname`/`GivenName`/`Patronymic` → 201, оба `ref` round-trip'ятся как есть, без проверки существования; (3) повторное чтение по id — подтверждает round-trip через реальный SQLite, не in-memory echo; (4) поиск по фрагменту фамилии («Петр»), который встречается ТОЛЬКО во ВТОРОЙ (`married`) записи `Names`, а не в первой (`main`) — запись находится, что доказывает: `personTerms` индексирует КАЖДЫЙ элемент `Names`, не только основной; (5) `private:false → true` через `PUT` (не `private → private`, специально в обе стороны отличную комбинацию, по уроку финального ревью подпроекта 7) — анонимный `GET` до update видит запись (200), после update — не видит (404).

**Проверка веба** (`WEB_REPORT.md`): `cd web && npm run typecheck`/`npm run build` чисты (дважды — в середине и в конце сессии). Живой клик-тест в браузере (`go run ./cmd/genodex -p 8765 -web dev`, чистая `.data/`, owner зарегистрирован через `curl` с `X-Requested-With: genodex`): каталог → «Персоны» → «+ добавить» → модалка рендерит `Пол` (`Select`), `Имена` (пустой `PersonNameListEditor`), `Сословия`/`Титулы`/`Прозвища`/`Заметки` (`TextRefListEditor`), `Доказательства` (`SourceLinkListEditor`), «Приватная запись»; заполнены пол, две строки `Names` (`основное`: Фамилия+Имя+since; `по браку`: только Фамилия — given/patronymic намеренно оставлены пустыми) через «+ имя», плюс словарные текстовые списки → создание → редирект на `/people/{id}`. Страница просмотра отрендерила заголовок «Дорожкина Акилина» (выбор главного имени, `pickDisplayName`), обе строки `Имена` корректно отформатированы, все текстовые списки и `Приватная = нет` — верно. Отдельно создана персона через `curl` с уже проставленными `ref`-ами на `given`/`patronymic`, открыта в браузере, изменена ТОЛЬКО фамилия (текст) в режиме редактирования, сохранена — повторный `curl`-фетч подтвердил: `surname` потерял `ref`/`type` (текст изменился), `given`/`patronymic` сохранили свои исходные `ref`/`type` неизменными (текст не менялся) — независимая по полю сохранность внутри ОДНОГО сохранения строки. Анонимный `curl` на теперь-приватную запись → 404, как и у всех остальных сущностей.

**Доки Задачи 1 описывают ТОЛЬКО бэкенд, не веб — в отличие от подпроекта 7.** Явно проверено построчно по фактическому `git diff main -- docs/usage.md docs/architecture.md CHANGELOG.md docs/data-model/entity-write.md` в проверочном worktree (бэкенд-часть была построена и закоммичена как отдельный живой проход ДО того, как веб-агент вообще начинал работу — в отличие от `Family`, где обе части строились одним проходом и потому CHANGELOG-пункт уже описывал будущие файлы веба заранее). Проверка по каждому из четырёх файлов:
- `docs/usage.md`/`docs/architecture.md` — по устоявшейся конвенции репозитория (см. Предпосылку плана подпроекта 7) эти два файла НЕ описывают веб-страницы вообще, ни у одной сущности программы — значит отсутствие веб-упоминания здесь не пробел, эти файлы уже полны после Задачи 1 и Задача 2 их не трогает.
- `CHANGELOG.md` — единственный файл из четырёх, который по конвенции описывает подпроект целиком (бэкенд+веб в одном пункте, см. пример подпроекта 7). Пункт, добавленный Задачей 1 (Шаг 1.8), буквально начинается со слов «Персоны (`Person`) — подпроект 8, backend (веб — отдельная работа)» — то есть сам текст, написанный бэкенд-агентом, явно фиксирует незавершённость и откладывает веб-часть на будущее. Это ПРОБЕЛ, который нужно закрыть: Шаг 2.4 ниже дополняет этот же пункт списком веб-файлов и убирает оговорку «веб — отдельная работа», приводя пункт к финальному виду, аналогичному пункту подпроекта 7.
- `docs/data-model/entity-write.md` — §3.7 (Шаг 1.8) уже существует в проверочном worktree и подробно описывает бэкенд-паттерны, но его ПОСЛЕДНИЙ пункт дословно гласит: «**Backend-only подпроект.** Веб-страницы (`PersonForm`/`PersonView`/`PeopleList`, редактор подформы `PersonName`) — отдельная работа, выполняется отдельно от этого прохода». Аналогично, §5 («Не в объёме этого документа») содержит фразу «её веб-форма (`PersonForm`/`PersonView`, редактор `[]PersonName`) — отдельная работа, вне объёма этого прохода». Оба места — ПРОБЕЛ по той же причине. Шаг 2.4 заменяет последний пункт §3.7 на описание реально построенного `PersonNameListEditor` (design judgment call №1 ниже) и правит формулировку §5, убирая «вне объёма этого прохода».
Итог: это случай (a) из инструкции подпроекта — доки Задачи 1 покрывают ТОЛЬКО бэкенд ПО СУЩЕСТВУ (`CHANGELOG.md`, `entity-write.md`), а не только по формальному признаку «не переписаны веб-агентом» — сам текст, написанный бэкенд-агентом, явно ссылается на веб как на будущую работу. `usage.md`/`architecture.md` не в счёт — они по конвенции репозитория никогда не описывают веб. Задача 2 ниже получает явный докный шаг (2.4), закрывающий оба реальных пробела.

**Design judgment calls из `BACKEND_REPORT.md`, перенесённые сюда дословно по смыслу:**

1. **Наименование**: usecase-пакеты используют иррегулярное множественное число (`list_people`/`search_people`, зеркалируя `store.Store.ListPeople`), и по цепочке экспортируемые методы `Scenario` тоже называются `ListPeople`/`SearchPeople` (не `ListPersons`) — вплоть до `httpapi.PersonService`/`mcp.PersonService`. Имена MCP-тулов и HTTP-маршрут остаются на едином для программы правиле (`person_list`, `/api/people`), как и было явно указано в задании.
2. **`transport.PeopleFromModels`** (не `PersonsFromModels`) — выбрано по той же логике иррегулярного множественного числа, хотя текст задания явно не требовал такого именования на уровне транспорта; это единственное место, где конвенция была продолжена по аналогии, а не по явной спецификации.
3. Добавлено несколько тестов сверх буквы задания (сценарный тест поиска по имени «по браку», и проверка поискового индекса внутри real-store теста) — дёшево и напрямую машинно доказывают нюанс поиска, который задание подчёркивало; продакшен-код они не меняют.

**Design judgment calls из `WEB_REPORT.md`, перенесённые сюда дословно по смыслу:**

1. **Baseline-per-mount отслеживание ref в `PersonNameListEditor`** — задание указывало применить ref-preservation-логику `ChurchView.tsx` («сохранить `ref`, если текст не изменился по сравнению с загруженным значением») «в точке, где локальное состояние редактирования конвертируется обратно в значение `PersonName[]`, которое получает `onChange` родителя» — поскольку (в отличие от `parishText`-поля формы у `ChurchView` со сравнением в момент отправки) у этого компонента нет отдельного шага отправки: это полностью управляемый (controlled) редактор массива, мутируемый на каждое нажатие клавиши. Реализовано так: один раз при монтировании компонента (`useState`-инициализатор от входного `value`, НЕ `useEffect`, пересинхронизирующийся позже) захватывается базовая линия `rowMeta` (`{id, base: PersonName}[]`), ключом служит стабильный id строки, присвоенный при монтировании/при добавлении строки (не индекс массива — чтобы добавление/удаление строк не путало базовые линии). Каждое нажатие клавиши в `surname`/`given`/`patronymic` сравнивает новый текст с ИСХОДНЫМ текстом этой строки из базовой линии для этого поля; равно ⇒ сохраняется исходный объект `{text, ref, type}` целиком; отличается ⇒ испускается новый объект только с `{text}`. Это работает корректно, потому что в обеих точках использования (`PersonForm.tsx`'s модалка с `destroyOnHidden`, `PersonView.tsx`'s условная ветка `{editing ? <Form>... : ...}`) `PersonNameListEditor` реально размонтируется/монтируется заново при каждой сессии редактирования — подтверждено живьём (см. проверку веба выше). **Задокументированное ограничение**: компоненту потребовался бы другой дизайн (например, явный сигнал «сбросить базовую линию»), если бы он когда-либо переиспользовался там, где остаётся смонтированным при переключении между не связанными записями — это не текущий случай ни в `PersonForm`, ни в `PersonView`, но следующий разработчик, переиспользующий `PersonNameListEditor` где-то ещё, обязан явно проверить это допущение.
2. **`SourceLinkListEditor`'s список цитат устаревает внутри модалки создания**, если цитата создана в другой вкладке уже после открытия модалки — это существующее поведение общего хука `useCitationOptions()` (загружает список один раз при монтировании, как у любой другой формы создания в программе), не новое для этого подпроекта. Стоит отметить отдельно, поскольку форма создания `Person` длиннее и богаче большинства остальных и потому на практике более вероятно с этим столкнётся; исправление вне объёма этого подпроекта.
3. **Конвенция пустой опции `Пол`/`PersonNameType`**: использован тот же паттерн `{ value: "", label: "…" }`, что уже установлен `FactDateEditor.tsx`'s `CALENDAR_OPTIONS` (обычная опция с пустой строкой, подписанная «Не указан(о)», не antd's `allowClear`) — для `GENDER_OPTIONS` (в `api.ts`, рядом с типом `PersonGender`, по образцу `ADMIN_DIVISION_TYPE_LABELS`) и `PERSON_NAME_TYPE_OPTIONS` (в `PersonNameList.tsx`) — соответствует явным формам аргументов задания (`""`/`"male"`/… и `""`/`"main"`/…).
4. **Никакого пикера для `surname`/`given`/`patronymic`** — подтверждено и выполнено по явной инструкции задания: обычные `Input`, привязанные к `.text`, точно как `Church.Parish`, несмотря на то что `Surname`/`GivenName`/`Patronymic` имеют полный CRUD+поиск с подпроектов 1-2.

## Задача 1. Бэкенд: Person — полный стек + доки + real-store тесты

**Интерфейсы, потребляемые из подпроектов 1-7**: `models.Page`/`models.Access`, `internal/httpapi/surname.go`:`parsePage`, `internal/mcp/*.go`:`optionalInt`/`toolJSONResult`/`textRefsFromStrings`, `transport.{TextRef,FactDate,SourceLink}`, `mcp/object_args.go`:`textRefObjectProperties`/`factDateObjectProperties`/`optionalTextRef`/`optionalFactDate`/`sourceLinkObjectProperties`/`optionalSourceLinks` (подпроекты 3, 5), `get_family`'s access-aware `Get`-паттерн, `create_family`'s проверка-FK-в-транзакции паттерн, `store.Store.{GetPerson,GetCitation,InTx,ListPeople}` (generic-хранилище, `internal/store/deps.go`, уже умеет читать/писать `Person` по id и содержит иррегулярный `ListPeople` до появления её usecase-слоя — модель `internal/models/person.go`/`person_validate.go`/`person_name.go`/`person_name_validate.go` уже существует и не меняется этим подпроектом).
**Производит**: `transport.Person`/`transport.PersonName` + Create/Update-варианты, `httpapi.PersonService`, `mcp.PersonService`, HTTP-роуты `/api/people*`, MCP-тулы `person_*` (6 штук), хелперы `personNameObjectProperties`/`optionalPersonNames` в `internal/mcp/object_args.go` — потребляются Задачей 2 (веб-страницы, `PersonNameListEditor`).

**Файлы:**
- Изменить: `internal/httpapi/{deps.go,api.go,httpapi.go}`, `internal/mcp/{deps.go,server.go,object_args.go}`, `internal/app/app.go`, `internal/httpapi/{write_store_test.go,store_test.go}`, `docs/usage.md`, `docs/architecture.md`, `CHANGELOG.md`, `docs/data-model/entity-write.md`
- Создать: `internal/transport/{person,person_name,person_write}.go`, `internal/usecases/{list_people,search_people,get_person,create_person,update_person,delete_person}/{deps.go,scenario.go,scenario_test.go}`, `internal/httpapi/{person,person_write,person_test,person_write_test}.go`, `internal/mcp/{person,person_test}.go`

**Важно для исполнителя**: `list_people`/`search_people`/`delete_person` — механические, по образцу `list_families`/`search_families`/`delete_family` (с поправкой на иррегулярное множественное число `ListPeople`/`SearchPeople`, см. Предпосылку). `get_person` — access-aware с первого черновика (`Person` несёт `Private`). `create_person`/`update_person` — единственная проверка существования — `sources[i].citation_id` (строгий FK), в той же транзакции, что и сохранение (`InTx` с рождения, по образцу `create_family`). `Names[i].Surname/.Given/.Patronymic`, `Estates`, `Titles`, `Nicknames` — мягкие ссылки/тексты, копируются как есть без проверки существования цели. `internal/mcp/object_args.go` — единственный файл в этой задаче, где меняется НЕ ВСЁ содержимое механически: добавляются ровно две новые функции (`personNameObjectProperties`/`optionalPersonNames`), остальной файл (хелперы подпроектов 3 и 5) переносится как есть. Переносить код ниже как есть, файл за файлом — итоговое содержимое, не диффы (кроме секции документации, Шаг 1.8, — там даны точные фрагменты-вставки в существующие большие файлы).

### 1.1. Транспорт

`transport.PersonName` — новый файл, по образцу `transport.Anchor` (подпроект 5, первый прецедент «собственный файл под вложенный не-полиморфный DTO»): `{type, surname, given, patronymic, prefix, suffix, since, until}`, `surname`/`given`/`patronymic` — обычный `transport.TextRef`, `since`/`until` — обычный `transport.FactDate`, ничего нового на уровне value-DTO не понадобилось — только оболочка `PersonNameFromModel`/`(PersonName).Model()` + `PersonNamesFromModel`/`PersonNamesToModel` для среза, зеркалирующая `TextRefsFromModel`/`TextRefsToModel`.
#### `internal/transport/person_name.go` (создать)
```go
package transport

import "github.com/amarin/genodex/internal/models"

// PersonName — контракт одного имени персоны (вложенная подформа
// Person.Names, models.PersonName): вид имени + три мягкие ссылки на
// словари (Surname/GivenName/Patronymic, того же TextRef-контракта, что и
// одиночные списки TextRef в других сущностях) + служебные части имени +
// период действия (FactDate). Первая вложенная подформа-«массив объектов» в
// программе (docs/data-model/entity-write.md §3.7).
type PersonName struct {
	Type       string    `json:"type,omitempty"`
	Surname    TextRef   `json:"surname"`
	Given      TextRef   `json:"given"`
	Patronymic TextRef   `json:"patronymic"`
	Prefix     string    `json:"prefix,omitempty"`
	Suffix     string    `json:"suffix,omitempty"`
	Since      *FactDate `json:"since,omitempty"`
	Until      *FactDate `json:"until,omitempty"`
}

// PersonNameFromModel конвертирует одно имя в контракт.
func PersonNameFromModel(n models.PersonName) PersonName {
	return PersonName{
		Type:       string(n.Type),
		Surname:    TextRefFromModel(n.Surname),
		Given:      TextRefFromModel(n.Given),
		Patronymic: TextRefFromModel(n.Patronymic),
		Prefix:     n.Prefix,
		Suffix:     n.Suffix,
		Since:      FactDateFromModel(n.Since),
		Until:      FactDateFromModel(n.Until),
	}
}

// PersonNamesFromModel конвертирует список; пустой вход даёт пустой срез, а не nil.
func PersonNamesFromModel(ns []models.PersonName) []PersonName {
	out := make([]PersonName, 0, len(ns))
	for _, n := range ns {
		out = append(out, PersonNameFromModel(n))
	}

	return out
}

// Model конвертирует контракт обратно в модель.
func (n PersonName) Model() models.PersonName {
	return models.PersonName{
		Type:       models.PersonNameType(n.Type),
		Surname:    n.Surname.Model(),
		Given:      n.Given.Model(),
		Patronymic: n.Patronymic.Model(),
		Prefix:     n.Prefix,
		Suffix:     n.Suffix,
		Since:      n.Since.Model(),
		Until:      n.Until.Model(),
	}
}

// PersonNamesToModel конвертирует список контрактов в модели; пустой вход
// даёт пустой срез, а не nil.
func PersonNamesToModel(ns []PersonName) []models.PersonName {
	out := make([]models.PersonName, 0, len(ns))
	for _, n := range ns {
		out = append(out, n.Model())
	}

	return out
}
```

#### `internal/transport/person.go` (создать)
```go
package transport

import "github.com/amarin/genodex/internal/models"

// Person — контракт персоны (GET /api/people, MCP-тул person_list) — ядро
// графа генеалогии. Sources редактируется с рождения контракта (сущность
// заведена уже после подпроекта 5, см. internal/transport/source_link.go).
//
// Поиск (person_search, GET /api/people/search) ищет по началу фамилии,
// имени или отчества из ЛЮБОГО элемента Names (не только основного) — все
// имена персоны индексируются под единым полем "name"
// (internal/store/sqlstore/person.go:personTerms). Estates/Titles/
// Nicknames/Notes поиском не охватываются.
type Person struct {
	ID        models.ID    `json:"id"`
	Gender    string       `json:"gender,omitempty"`
	Names     []PersonName `json:"names"`
	Estates   []TextRef    `json:"estates"`
	Titles    []TextRef    `json:"titles"`
	Nicknames []TextRef    `json:"nicknames"`
	Notes     []TextRef    `json:"notes"`
	Sources   []SourceLink `json:"sources"`
	Private   bool         `json:"private"`
}

// PersonFromModel конвертирует запись в контракт.
func PersonFromModel(p models.Person) Person {
	return Person{
		ID:        p.ID,
		Gender:    string(p.Gender),
		Names:     PersonNamesFromModel(p.Names),
		Estates:   TextRefsFromModel(p.Estates),
		Titles:    TextRefsFromModel(p.Titles),
		Nicknames: TextRefsFromModel(p.Nicknames),
		Notes:     TextRefsFromModel(p.Notes),
		Sources:   SourceLinksFromModel(p.Sources),
		Private:   p.Private,
	}
}

// PeopleFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func PeopleFromModels(ps []models.Person) []Person {
	out := make([]Person, 0, len(ps))
	for _, p := range ps {
		out = append(out, PersonFromModel(p))
	}

	return out
}
```

#### `internal/transport/person_write.go` (создать)
```go
package transport

import "github.com/amarin/genodex/internal/models"

// PersonCreate — тело POST /api/people и аргументы тула person_create.
// Идентификатор генерирует сценарий.
type PersonCreate struct {
	Gender    string       `json:"gender,omitempty"`
	Names     []PersonName `json:"names"`
	Estates   []TextRef    `json:"estates"`
	Titles    []TextRef    `json:"titles"`
	Nicknames []TextRef    `json:"nicknames"`
	Notes     []TextRef    `json:"notes"`
	Sources   []SourceLink `json:"sources"`
	Private   bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (p PersonCreate) Model() models.Person {
	return models.Person{
		Gender:    models.PersonGender(p.Gender),
		Names:     PersonNamesToModel(p.Names),
		Estates:   TextRefsToModel(p.Estates),
		Titles:    TextRefsToModel(p.Titles),
		Nicknames: TextRefsToModel(p.Nicknames),
		Notes:     TextRefsToModel(p.Notes),
		Sources:   SourceLinksToModel(p.Sources),
		Private:   p.Private,
	}
}

// PersonUpdate — тело PUT /api/people/{id} и аргументы тула person_update:
// полная замена gender/names/estates/titles/nicknames/notes/sources/private.
type PersonUpdate struct {
	Gender    string       `json:"gender,omitempty"`
	Names     []PersonName `json:"names"`
	Estates   []TextRef    `json:"estates"`
	Titles    []TextRef    `json:"titles"`
	Nicknames []TextRef    `json:"nicknames"`
	Notes     []TextRef    `json:"notes"`
	Sources   []SourceLink `json:"sources"`
	Private   bool         `json:"private"`
}

// Model возвращает доменную запись с пустым ID.
func (p PersonUpdate) Model() models.Person {
	return models.Person{
		Gender:    models.PersonGender(p.Gender),
		Names:     PersonNamesToModel(p.Names),
		Estates:   TextRefsToModel(p.Estates),
		Titles:    TextRefsToModel(p.Titles),
		Nicknames: TextRefsToModel(p.Nicknames),
		Notes:     TextRefsToModel(p.Notes),
		Sources:   SourceLinksToModel(p.Sources),
		Private:   p.Private,
	}
}
```

### 1.2. Usecase-сценарии

Шесть пакетов по три файла (18 файлов) — группа однотипной работы, как в подпроектах 5/6: `list_people`/`search_people` механические (окно/фильтр по generic-хранилищу), `get_person` — access-aware, `create_person`/`update_person` — единственная проверка `sources[i].citation_id` в транзакции, `delete_person` — тонкая обёртка над `store.DeletePerson` (вся `*InUseError`-логика уже в generic-слое).

#### `internal/usecases/list_people/`

#### `internal/usecases/list_people/deps.go` (создать)
```go
package list_people

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// PersonRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type PersonRepo interface {
	ListPeople(ctx context.Context, access models.Access, page models.Page) ([]*models.Person, error)
}
```

#### `internal/usecases/list_people/scenario.go` (создать)
```go
package list_people

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список персон».
type Scenario struct {
	people PersonRepo
}

// New создаёт сценарий.
func New(people PersonRepo) *Scenario {
	return &Scenario{people: people}
}

// ListPeople возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListPeople(ctx context.Context, access models.Access, page models.Page) ([]models.Person, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypePerson, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypePerson, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.people.ListPeople(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Person, 0, len(list))
	for _, p := range list {
		out = append(out, *p)
	}

	return out, nil
}
```

#### `internal/usecases/list_people/scenario_test.go` (создать)
```go
package list_people

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	page models.Page
	out  []*models.Person
}

func (f *fakeRepo) ListPeople(_ context.Context, _ models.Access, page models.Page) ([]*models.Person, error) {
	f.page = page

	return f.out, nil
}

func TestListPeopleReturnsRecords(t *testing.T) {
	repo := &fakeRepo{out: []*models.Person{{ID: "I-01ARZ3NDEKTSV4RRFFQ69G5FA1"}}}

	got, err := New(repo).ListPeople(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListPeople: %v", err)
	}

	if len(got) != 1 || got[0].ID != "I-01ARZ3NDEKTSV4RRFFQ69G5FA1" {
		t.Fatalf("got = %+v", got)
	}

	if repo.page.Limit != 10 {
		t.Fatalf("page = %+v, лимит не дошёл до репозитория", repo.page)
	}
}

func TestListPeopleRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListPeople(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}
```


#### `internal/usecases/search_people/`

#### `internal/usecases/search_people/deps.go` (создать)
```go
package search_people

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// PersonRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type PersonRepo interface {
	Search(ctx context.Context, query string, access models.Access, page models.Page) ([]models.Hit, error)
	GetPerson(ctx context.Context, id models.ID) (*models.Person, error)
}
```

#### `internal/usecases/search_people/scenario.go` (создать)
```go
package search_people

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск персон».
type Scenario struct {
	people PersonRepo
}

// New создаёт сценарий.
func New(people PersonRepo) *Scenario {
	return &Scenario{people: people}
}

// SearchPeople находит записи, у которых фамилия/имя/отчество из ЛЮБОГО
// элемента Names начинается с текста запроса — единое поисковое поле "name"
// индексирует все имена персоны, не только основное (см.
// internal/store/sqlstore/person.go:personTerms). Та же механика окна, что
// и search_divisions.SearchDivisions (см. её комментарий).
func (s *Scenario) SearchPeople(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Person, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Person{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Person{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.people.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypePerson {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.people.GetPerson(ctx, h.ID)
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

#### `internal/usecases/search_people/scenario_test.go` (создать)
```go
package search_people

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits   []models.Hit
	people map[models.ID]*models.Person
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetPerson(_ context.Context, id models.ID) (*models.Person, error) {
	p, ok := f.people[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return p, nil
}

func TestSearchPeopleFiltersByType(t *testing.T) {
	id := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("F-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypePerson, ID: id},
			{Type: models.TypeFamily, ID: otherID}, // не person — должен быть пропущен
		},
		people: map[models.ID]*models.Person{id: {ID: id}},
	}

	got, err := New(repo).SearchPeople(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchPeople: %v", err)
	}

	if len(got) != 1 || got[0].ID != id {
		t.Fatalf("got = %+v", got)
	}
}

// TestSearchPeopleFindsByMarriedNameOnly проверяет, что поиск находит
// персону по части фамилии/имени/отчества из ЛЮБОГО элемента Names, а не
// только из первого/основного — то же свойство, что доказывает real-store
// тест (TestPersonWriteContractWithRealStore), но здесь на уровне сценария:
// хранилище-фейк отдаёт хит из search_index, GetPerson возвращает запись,
// у которой совпадение было бы только во второй записи Names.
func TestSearchPeopleFindsByMarriedNameOnly(t *testing.T) {
	id := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		hits: []models.Hit{{Type: models.TypePerson, ID: id}},
		people: map[models.ID]*models.Person{id: {ID: id, Names: []models.PersonName{
			{Type: models.PersonNameMain, Surname: models.TextRef{Text: "Иванова"}},
			{Type: models.PersonNameMarried, Surname: models.TextRef{Text: "Петрова"}},
		}}},
	}

	got, err := New(repo).SearchPeople(context.Background(), models.AccessFull, models.SearchQuery{Text: "Петр"})
	if err != nil {
		t.Fatalf("SearchPeople: %v", err)
	}

	if len(got) != 1 || got[0].ID != id {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchPeopleEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchPeople(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchPeople: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
```


#### `internal/usecases/get_person/`

#### `internal/usecases/get_person/deps.go` (создать)
```go
package get_person

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// PersonRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type PersonRepo interface {
	GetPerson(ctx context.Context, id models.ID) (*models.Person, error)
}
```

#### `internal/usecases/get_person/scenario.go` (создать)
```go
package get_person

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «персона по идентификатору».
type Scenario struct {
	people PersonRepo
}

// New создаёт сценарий.
func New(people PersonRepo) *Scenario {
	return &Scenario{people: people}
}

// GetPerson возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound. Приватная запись
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound — тот же принцип «прячем как отсутствующее», что и в
// List/Search.
func (s *Scenario) GetPerson(ctx context.Context, access models.Access, id models.ID) (models.Person, error) {
	if err := validateID(id); err != nil {
		return models.Person{}, err
	}

	p, err := s.people.GetPerson(ctx, id)
	if err != nil {
		return models.Person{}, err
	}

	if p.Private && access != models.AccessFull {
		return models.Person{}, models.ErrNotFound
	}

	return *p, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypePerson)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypePerson,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

#### `internal/usecases/get_person/scenario_test.go` (создать)
```go
package get_person

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	people map[models.ID]*models.Person
}

func (f *fakeRepo) GetPerson(_ context.Context, id models.ID) (*models.Person, error) {
	p, ok := f.people[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return p, nil
}

func TestGetPersonReturnsRecord(t *testing.T) {
	id := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{people: map[models.ID]*models.Person{id: {ID: id, Gender: models.PersonGenderFemale}}}

	got, err := New(repo).GetPerson(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetPerson: %v", err)
	}

	if got.Gender != models.PersonGenderFemale {
		t.Fatalf("Gender = %q", got.Gender)
	}
}

func TestGetPersonNotFound(t *testing.T) {
	repo := &fakeRepo{people: map[models.ID]*models.Person{}}

	_, err := New(repo).GetPerson(context.Background(), models.AccessFull, "I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetPersonInvalidID(t *testing.T) {
	repo := &fakeRepo{people: map[models.ID]*models.Person{}}

	_, err := New(repo).GetPerson(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestGetPersonPrivateHiddenFromPublic(t *testing.T) {
	id := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{people: map[models.ID]*models.Person{id: {ID: id, Private: true}}}

	_, err := New(repo).GetPerson(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for private record with AccessPublic", err)
	}
}

func TestGetPersonPrivateVisibleToFullAccess(t *testing.T) {
	id := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{people: map[models.ID]*models.Person{id: {ID: id, Private: true}}}

	got, err := New(repo).GetPerson(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetPerson: %v", err)
	}

	if got.ID != id {
		t.Fatalf("ID = %q", got.ID)
	}
}
```


#### `internal/usecases/create_person/`

#### `internal/usecases/create_person/deps.go` (создать)
```go
package create_person

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// PersonStore — зависимость сценария: транзакция порта store.Store.
// Проверка ссылок (Sources) и сохранение идут в одной транзакции на
// переданном fn хранилище.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type PersonStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
```

#### `internal/usecases/create_person/scenario.go` (создать)
```go
package create_person

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание персоны».
type Scenario struct {
	store PersonStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st PersonStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreatePerson создаёт запись: генерирует идентификатор, проверяет
// инварианты, в одной транзакции убеждается в существовании цитат из Sources
// и сохраняет. Возвращает созданную запись с заполненным ID.
//
// Единственная строго проверяемая ссылка — Sources[i].CitationID. Все
// остальные ссылки (Names[i].Surname/.Given/.Patronymic, Estates, Titles,
// Nicknames) — мягкие TextRef и НЕ проверяются на существование, как и у
// любого другого списка TextRef в программе (docs/data-model/entity-write.md).
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта), невалидная сущность и несуществующая цитата —
// *models.ValidationError; прочее — ошибки хранилища как есть.
func (s *Scenario) CreatePerson(ctx context.Context, p models.Person) (models.Person, error) {
	if p.ID != "" {
		return models.Person{}, &models.ValidationError{
			Entity: models.TypePerson,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", p.ID),
		}
	}

	p.ID = s.ids.New(models.TypePerson)

	if err := p.Validate(); err != nil {
		return models.Person{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		for i, link := range p.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SavePerson(ctx, &p)
	})
	if err != nil {
		return models.Person{}, err
	}

	return p, nil
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypePerson,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/create_person/scenario_test.go` (создать)
```go
package create_person

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// cID возвращает корректный идентификатор цитаты, отличающийся последним символом.
func cID(last byte) models.ID {
	return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

type stubIDs struct{ id models.ID }

func (s *stubIDs) New(models.Type) models.ID { return s.id }

// fakeTx реализует нужные сценарию методы store.Store поверх карты цитат;
// остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	citations map[models.ID]*models.Citation
	saved     *models.Person
	saveErr   error
}

func newFakeTx(existingCitations ...*models.Citation) *fakeTx {
	tx := &fakeTx{citations: map[models.ID]*models.Citation{}}
	for _, c := range existingCitations {
		tx.citations[c.ID] = c
	}

	return tx
}

func (f *fakeTx) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *c

	return &cp, nil
}

func (f *fakeTx) SavePerson(_ context.Context, p *models.Person) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *p
	f.saved = &cp

	return nil
}

// fakeStore реализует PersonStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func TestCreatePersonGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "I-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{tx: newFakeTx()}
	sc := New(st, ids)

	in := models.Person{Gender: models.PersonGenderFemale, Names: []models.PersonName{
		{Given: models.TextRef{Text: "Акилина"}},
	}}

	got, err := sc.CreatePerson(context.Background(), in)
	if err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.tx.saved == nil || st.tx.saved.Gender != models.PersonGenderFemale {
		t.Fatalf("saved = %+v", st.tx.saved)
	}
}

func TestCreatePersonRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx()}, &stubIDs{})

	_, err := sc.CreatePerson(context.Background(), models.Person{ID: "I-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestCreatePersonRejectsBadGender(t *testing.T) {
	sc := New(&fakeStore{tx: newFakeTx()}, &stubIDs{id: "I-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreatePerson(context.Background(), models.Person{Gender: "x"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "gender" {
		t.Fatalf("err = %v, want ValidationError on gender", err)
	}
}

func TestCreatePersonAllFieldsOptional(t *testing.T) {
	ids := &stubIDs{id: "I-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{tx: newFakeTx()}
	sc := New(st, ids)

	got, err := sc.CreatePerson(context.Background(), models.Person{})
	if err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q", got.ID)
	}
}

func TestCreatePersonPropagatesStoreError(t *testing.T) {
	wantErr := errors.New("db down")
	st := &fakeStore{tx: newFakeTx()}
	st.tx.saveErr = wantErr
	sc := New(st, &stubIDs{id: "I-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreatePerson(context.Background(), models.Person{})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}

// TestCreatePersonSourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestCreatePersonSourceCitationNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	sc := New(st, &stubIDs{id: "I-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	in := models.Person{Sources: []models.SourceLink{{CitationID: cID('0')}}}

	_, err := sc.CreatePerson(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if st.tx.saved != nil {
		t.Fatalf("saved = %+v; ожидалось: ничего не сохранено", st.tx.saved)
	}
}

// TestCreatePersonNamesSoftRefNotChecked: Names[i].Surname/.Given/.Patronymic
// — мягкие ссылки на Surname/GivenName/Patronymic (все три имеют CRUD с
// подпроектов 1-2), TextRef.Ref не проверяется на существование при
// сохранении — только формат (тот же принцип, что у Family.Members на
// Person, см. TestCreateFamilyMembersSoftRefNotChecked). Ссылки указывают на
// заведомо несуществующие записи словарей — сохранение всё равно проходит.
func TestCreatePersonNamesSoftRefNotChecked(t *testing.T) {
	ids := &stubIDs{id: "I-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{tx: newFakeTx()}
	sc := New(st, ids)

	in := models.Person{
		Names: []models.PersonName{{
			Type:       models.PersonNameMain,
			Surname:    models.TextRef{Text: "Дорожкина", Ref: "SN-01ARZ3NDEKTSV4RRFFQ69G5FA1", Type: models.TypeSurname},
			Given:      models.TextRef{Text: "Акилина", Ref: "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1", Type: models.TypeGivenName},
			Patronymic: models.TextRef{Text: "Ивановна", Ref: "PN-01ARZ3NDEKTSV4RRFFQ69G5FA1", Type: models.TypePatronymic},
		}},
	}

	got, err := sc.CreatePerson(context.Background(), in)
	if err != nil {
		t.Fatalf("CreatePerson: %v", err)
	}

	if len(got.Names) != 1 ||
		got.Names[0].Surname.Ref != "SN-01ARZ3NDEKTSV4RRFFQ69G5FA1" ||
		got.Names[0].Given.Ref != "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1" ||
		got.Names[0].Patronymic.Ref != "PN-01ARZ3NDEKTSV4RRFFQ69G5FA1" {
		t.Fatalf("Names = %+v", got.Names)
	}

	if st.tx.saved == nil {
		t.Fatalf("saved = nil, want сохранённая запись")
	}
}
```


#### `internal/usecases/update_person/`

#### `internal/usecases/update_person/deps.go` (создать)
```go
package update_person

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// PersonStore — зависимость сценария: транзакция порта store.Store.
// Проверка существования и сохранение идут в одной транзакции.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type PersonStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
```

#### `internal/usecases/update_person/scenario.go` (создать)
```go
package update_person

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение персоны».
type Scenario struct {
	store PersonStore
}

// New создаёт сценарий.
func New(st PersonStore) *Scenario {
	return &Scenario{store: st}
}

// UpdatePerson полностью заменяет запись по p.ID: проверяет инварианты, в
// одной транзакции убеждается, что запись существует, и сохраняет.
//
// Единственная строго проверяемая ссылка — Sources[i].CitationID (как в
// CreatePerson); Names[i].Surname/.Given/.Patronymic, Estates, Titles,
// Nicknames — мягкие TextRef, без проверки существования.
//
// Ошибки: невалидная сущность — *models.ValidationError; нет такой записи —
// models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdatePerson(ctx context.Context, p models.Person) error {
	if err := p.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetPerson(ctx, p.ID); err != nil {
			return err
		}

		for i, link := range p.Sources {
			if _, err := tx.GetCitation(ctx, link.CitationID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return sourceLinkErr(i, "цитата %q не найдена", link.CitationID)
				}

				return err
			}
		}

		return tx.SavePerson(ctx, &p)
	})
}

// sourceLinkErr — *models.ValidationError по полю sources[i].citation_id.
func sourceLinkErr(i int, format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypePerson,
		Field:  fmt.Sprintf("sources[%d].citation_id", i),
		Reason: fmt.Sprintf(format, args...),
	}
}
```

#### `internal/usecases/update_person/scenario_test.go` (создать)
```go
package update_person

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// fakeTx реализует нужные сценарию методы store.Store поверх карты записей;
// остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	people    map[models.ID]*models.Person
	citations map[models.ID]*models.Citation
	saved     []*models.Person
}

func newFakeTx(existing ...*models.Person) *fakeTx {
	tx := &fakeTx{people: map[models.ID]*models.Person{}, citations: map[models.ID]*models.Citation{}}
	for _, p := range existing {
		tx.people[p.ID] = p
	}

	return tx
}

// cID возвращает корректный идентификатор цитаты.
func cID(last byte) models.ID {
	return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

func (f *fakeTx) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *c

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

func (f *fakeTx) SavePerson(_ context.Context, p *models.Person) error {
	cp := *p
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует PersonStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func person(id models.ID) *models.Person {
	return &models.Person{ID: id}
}

func TestUpdatePersonSaves(t *testing.T) {
	existing := person("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Gender = models.PersonGenderMale

	if err := New(st).UpdatePerson(context.Background(), updated); err != nil {
		t.Fatalf("UpdatePerson: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Gender != models.PersonGenderMale {
		t.Fatalf("InTx=%d, saved=%v", st.calls, st.tx.saved)
	}
}

func TestUpdatePersonNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdatePerson(context.Background(), *person("I-01ARZ3NDEKTSV4RRFFQ69G5FA1"))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdatePersonRejectsBadGender(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdatePerson(context.Background(), models.Person{ID: "I-01ARZ3NDEKTSV4RRFFQ69G5FA1", Gender: "x"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "gender" {
		t.Fatalf("err = %v, want ValidationError on gender", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

// TestUpdatePersonSourceCitationNotFound: Sources ссылается на
// несуществующую цитату — *models.ValidationError по полю
// sources[0].citation_id, ничего не сохраняется.
func TestUpdatePersonSourceCitationNotFound(t *testing.T) {
	existing := person("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Sources = []models.SourceLink{{CitationID: cID('0')}}

	err := New(st).UpdatePerson(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "sources[0].citation_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю sources[0].citation_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d записей при несуществующей цитате", len(st.tx.saved))
	}
}

// TestUpdatePersonNamesSoftRefNotChecked: как и при создании (см.
// create_person.TestCreatePersonNamesSoftRefNotChecked), Names[i].Surname/
// .Given/.Patronymic не проверяются на существование при обновлении.
func TestUpdatePersonNamesSoftRefNotChecked(t *testing.T) {
	existing := person("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Names = []models.PersonName{{
		Type:       models.PersonNameMarried,
		Surname:    models.TextRef{Text: "Петрова", Ref: "SN-01ARZ3NDEKTSV4RRFFQ69G5FA1", Type: models.TypeSurname},
		Given:      models.TextRef{Text: "Мария", Ref: "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1", Type: models.TypeGivenName},
		Patronymic: models.TextRef{Text: "Ивановна", Ref: "PN-01ARZ3NDEKTSV4RRFFQ69G5FA1", Type: models.TypePatronymic},
	}}

	if err := New(st).UpdatePerson(context.Background(), updated); err != nil {
		t.Fatalf("UpdatePerson: %v", err)
	}

	if len(st.tx.saved) != 1 || st.tx.saved[0].Names[0].Surname.Ref != "SN-01ARZ3NDEKTSV4RRFFQ69G5FA1" {
		t.Fatalf("saved = %+v", st.tx.saved)
	}
}
```


#### `internal/usecases/delete_person/`

#### `internal/usecases/delete_person/deps.go` (создать)
```go
package delete_person

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// PersonRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type PersonRepo interface {
	DeletePerson(ctx context.Context, id models.ID) error
}
```

#### `internal/usecases/delete_person/scenario.go` (создать)
```go
package delete_person

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление персоны».
type Scenario struct {
	people PersonRepo
}

// New создаёт сценарий.
func New(people PersonRepo) *Scenario {
	return &Scenario{people: people}
}

// DeletePerson удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeletePerson(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.people.DeletePerson(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypePerson)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypePerson,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

#### `internal/usecases/delete_person/scenario_test.go` (создать)
```go
package delete_person

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

func (f *fakeRepo) DeletePerson(_ context.Context, id models.ID) error {
	if f.err != nil {
		return f.err
	}
	f.deleted = append(f.deleted, id)

	return nil
}

func TestDeletePersonCallsRepo(t *testing.T) {
	repo := &fakeRepo{}
	id := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	if err := New(repo).DeletePerson(context.Background(), id); err != nil {
		t.Fatalf("DeletePerson: %v", err)
	}

	if len(repo.deleted) != 1 || repo.deleted[0] != id {
		t.Fatalf("deleted = %v", repo.deleted)
	}
}

func TestDeletePersonInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	err := New(repo).DeletePerson(context.Background(), "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}

	if len(repo.deleted) != 0 {
		t.Fatalf("repo вызван при невалидном id")
	}
}

func TestDeletePersonPropagatesInUseError(t *testing.T) {
	wantErr := &models.InUseError{Type: models.TypePerson, ID: "I-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	repo := &fakeRepo{err: wantErr}

	err := New(repo).DeletePerson(context.Background(), "I-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	var iu *models.InUseError
	if !errors.As(err, &iu) {
		t.Fatalf("err = %v, want InUseError", err)
	}
}
```

### 1.3. HTTP API

`internal/httpapi/person.go` (list/search/get), `person_write.go` (create/update/delete) — по образцу `family.go`/`family_write.go`; `person_test.go`/`person_write_test.go` — юнит-тесты на фейках, зеркалируют `family_test.go`/`family_write_test.go`.
#### `internal/httpapi/person.go` (создать)
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handlePersonList — GET /api/people?limit=&offset=. Чтение открыто
// анонимному посетителю.
func handlePersonList(people PersonService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := people.ListPeople(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.PeopleFromModels(list))
	}
}

// handlePersonSearch — GET /api/people/search?q=&limit=&offset=. Ищет по
// началу фамилии/имени/отчества из ЛЮБОГО из имён персоны (не только
// основного) — см. transport.Person.
func handlePersonSearch(people PersonService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := people.SearchPeople(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.PeopleFromModels(list))
	}
}

// handlePersonGet — GET /api/people/{id}.
func handlePersonGet(people PersonService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := people.GetPerson(r.Context(), AccessFromContext(r.Context()), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.PersonFromModel(p))
	}
}
```

#### `internal/httpapi/person_write.go` (создать)
```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handlePersonCreate — POST /api/people: создаёт запись, отвечает
// 201 с созданной записью (id генерирует сценарий). Запись — только для
// вошедшего владельца, см. handleDivisionCreate.
func handlePersonCreate(people PersonService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.PersonCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := people.CreatePerson(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.PersonFromModel(created))
	}
}

// handlePersonUpdate — PUT /api/people/{id}: полная замена
// gender/names/estates/titles/nicknames/notes/sources/private. Читает
// текущую версию, накладывает поля запроса (fetch-then-merge,
// docs/data-model/entity-write.md §3).
func handlePersonUpdate(people PersonService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathID(r)

		var in transport.PersonUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := people.GetPerson(r.Context(), AccessFromContext(r.Context()), id)
		if err != nil {
			writeError(w, err)

			return
		}

		m := in.Model()
		cur.Gender = m.Gender
		cur.Names = m.Names
		cur.Estates = m.Estates
		cur.Titles = m.Titles
		cur.Nicknames = m.Nicknames
		cur.Notes = m.Notes
		cur.Sources = m.Sources
		cur.Private = m.Private

		if err := people.UpdatePerson(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.PersonFromModel(cur))
	}
}

// handlePersonDelete — DELETE /api/people/{id}: 204 без тела; занятая
// запись — 409 со списком ссылающихся. Запись — только для вошедшего
// владельца, см. handleDivisionCreate.
func handlePersonDelete(people PersonService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := people.DeletePerson(r.Context(), pathID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
```

#### `internal/httpapi/person_test.go` (создать)
```go
package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakePeople struct {
	list []models.Person
	err  error
	page models.Page

	getP      models.Person
	gotIDs    []models.ID
	created   models.Person
	gotCreate models.Person
	updated   models.Person
	deleteErr error

	search    []models.Person
	gotSearch models.SearchQuery
}

func (f *fakePeople) ListPeople(_ context.Context, _ models.Access, page models.Page) ([]models.Person, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakePeople) SearchPeople(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Person, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakePeople) GetPerson(_ context.Context, _ models.Access, id models.ID) (models.Person, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Person{}, f.err
	}

	return f.getP, nil
}

func (f *fakePeople) CreatePerson(_ context.Context, p models.Person) (models.Person, error) {
	f.gotCreate = p
	if f.err != nil {
		return models.Person{}, f.err
	}

	return f.created, nil
}

func (f *fakePeople) UpdatePerson(_ context.Context, p models.Person) error {
	f.updated = p

	return f.err
}

func (f *fakePeople) DeletePerson(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func TestPersonListReturnsRecords(t *testing.T) {
	svc := &fakePeople{list: []models.Person{{ID: "I-1", Gender: models.PersonGenderFemale}}}

	rec := get(t, NewHandler(Deps{People: svc, DocsFS: fstest.MapFS{}}), "/api/people")
	requireStatus(t, rec, 200)

	want := `[{"id":"I-1","gender":"female","names":[],"estates":[],"titles":[],"nicknames":[],"notes":[],"sources":[],"private":false}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestPersonGetNotFound(t *testing.T) {
	svc := &fakePeople{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{People: svc, DocsFS: fstest.MapFS{}}), "/api/people/I-1")
	requireStatus(t, rec, 404)
}

func TestPersonSearchPassesQuery(t *testing.T) {
	svc := &fakePeople{}

	rec := get(t, NewHandler(Deps{People: svc, DocsFS: fstest.MapFS{}}), "/api/people/search?q=Ив")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "Ив" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
```

#### `internal/httpapi/person_write_test.go` (создать)
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

func TestPersonCreateContract(t *testing.T) {
	svc := &fakePeople{created: models.Person{ID: "I-1", Gender: models.PersonGenderFemale}}

	rec := postD(t, NewHandler(Deps{People: svc, DocsFS: fstest.MapFS{}}), "/api/people",
		`{"gender":"female"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Gender != models.PersonGenderFemale || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
	if !strings.Contains(rec.Body.String(), `"gender":"female"`) {
		t.Fatalf("body = %s, gender не в ответе", rec.Body)
	}
}

func TestPersonCreateAnonymousIs401(t *testing.T) {
	svc := &fakePeople{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{People: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/people", strings.NewReader(`{}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestPersonUpdateMergesFields(t *testing.T) {
	svc := &fakePeople{getP: models.Person{ID: "I-1", Gender: models.PersonGenderFemale}}

	rec := putD(t, NewHandler(Deps{People: svc, DocsFS: fstest.MapFS{}}), "/api/people/I-1",
		`{"gender":"male","private":true}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Gender != models.PersonGenderMale || svc.updated.Private != true || svc.updated.ID != "I-1" {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestPersonDeleteNoContent(t *testing.T) {
	svc := &fakePeople{}

	rec := delD(t, NewHandler(Deps{People: svc, DocsFS: fstest.MapFS{}}), "/api/people/I-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestPersonDeleteInUseIs409(t *testing.T) {
	svc := &fakePeople{deleteErr: &models.InUseError{Type: models.TypePerson, ID: "I-1"}}

	rec := delD(t, NewHandler(Deps{People: svc, DocsFS: fstest.MapFS{}}), "/api/people/I-1")

	requireStatus(t, rec, http.StatusConflict)
}
```

### 1.4. MCP

`internal/mcp/person.go` — регистрация и обработчики шести тулов `person_list/search/get/create/update/delete`; `names` — второй в программе MCP-аргумент вида «массив объектов» и первый, чьи элементы сами несут вложенные объекты (`mcp.WithArray("names", mcp.Items(map[string]any{"type": "object", "properties": personNameObjectProperties()}), ...)`, чтение через `optionalPersonNames`, Шаг 1.5 ниже); `person_update` расширяет presence-check «отсутствие аргумента сохраняет текущее значение» с `sources` (единственного прежнего исключения) также на `names` — `internal/mcp/person_test.go` включает `TestPersonUpdateToolKeepsNamesWhenAbsent`/`TestPersonUpdateToolClearsNamesWhenEmptyArray` (симметрично существующей проверке presence-check для `sources`).
#### `internal/mcp/person.go` (создать)
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

// registerPersonTools регистрирует тулы для работы с персонами — ядром
// графа генеалогии. names — массив объектов вида PersonName (см.
// personNameObjectProperties, internal/mcp/object_args.go): вид имени + три
// мягкие ссылки на словари (surname/given/patronymic — существование НЕ
// проверяется, тот же принцип, что и у любого другого TextRef в программе)
// + служебные части + период. estates/titles/nicknames/notes — только
// текстом (v1, docs/data-model/entity-write.md §4). ВАЖНО: person_update
// заменяет estates/titles/nicknames/notes целиком текстом — существующие
// ref/type будут потеряны при любом обновлении через MCP, пока не появится
// picker; names и sources, наоборот, при отсутствии в вызове сохраняют
// текущее значение (см. personUpdateHandler).
func registerPersonTools(s *server.MCPServer, people PersonService) {
	tool := mcp.NewTool(
		"person_list",
		mcp.WithDescription("Список персон в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, personListHandler(people))

	tool = mcp.NewTool(
		"person_search",
		mcp.WithDescription("Поиск персон по началу фамилии, имени или отчества из ЛЮБОГО из имён персоны (не только основного); результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало фамилии, имени или отчества")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, personSearchHandler(people))

	tool = mcp.NewTool(
		"person_get",
		mcp.WithDescription("Персона по id; результат — JSON записи. Неверный формат id или отсутствующая/приватная (для не-владельца) запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например I-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, personGetHandler(people))

	tool = mcp.NewTool(
		"person_create",
		mcp.WithDescription("Создать персону; id генерируется сервером; результат — JSON созданной записи. Все поля, кроме id, необязательны"),
		mcp.WithString("gender", mcp.Description("Пол: male/female/unknown; пусто — не указан")),
		mcp.WithArray("names", mcp.Items(map[string]any{
			"type":       "object",
			"properties": personNameObjectProperties(),
		}), mcp.Description("Имена персоны (основное, при рождении, по браку…); каждое — хотя бы одна из частей surname/given/patronymic")),
		mcp.WithArray("estates", mcp.WithStringItems(), mcp.Description("Сословия (текстом; мягкая ссылка на словарь сословий, без проверки существования)")),
		mcp.WithArray("titles", mcp.WithStringItems(), mcp.Description("Титулы (текстом; мягкая ссылка на словарь титулов, без проверки существования)")),
		mcp.WithArray("nicknames", mcp.WithStringItems(), mcp.Description("Прозвища")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, personCreateHandler(people))

	tool = mcp.NewTool(
		"person_update",
		mcp.WithDescription("Изменить персону: полная замена gender/estates/titles/nicknames/notes/private; результат — JSON обновлённой записи; names и sources — при отсутствии в вызове текущее значение сохраняется, пустой массив — очищает его"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("gender", mcp.Description("Пол: male/female/unknown; пусто — не указан")),
		mcp.WithArray("names", mcp.Items(map[string]any{
			"type":       "object",
			"properties": personNameObjectProperties(),
		}), mcp.Description("Имена персоны; при отсутствии в вызове текущие имена сохраняются, пустой массив — очищает их")),
		mcp.WithArray("estates", mcp.WithStringItems(), mcp.Description("Сословия")),
		mcp.WithArray("titles", mcp.WithStringItems(), mcp.Description("Титулы")),
		mcp.WithArray("nicknames", mcp.WithStringItems(), mcp.Description("Прозвища")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, personUpdateHandler(people))

	tool = mcp.NewTool(
		"person_delete",
		mcp.WithDescription("Удалить персону. Необратимо. Если на неё есть строгие ссылки от других сущностей — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, personDeleteHandler(people))
}

func personListHandler(people PersonService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := people.ListPeople(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.PeopleFromModels(list))
	}
}

func personSearchHandler(people PersonService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := people.SearchPeople(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.PeopleFromModels(list))
	}
}

func personGetHandler(people PersonService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		p, err := people.GetPerson(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.PersonFromModel(p))
	}
}

func personCreateHandler(people PersonService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		names, err := optionalPersonNames(req.GetArguments(), "names")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sources, err := optionalSourceLinks(req.GetArguments(), "sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		p := models.Person{
			Gender:    models.PersonGender(req.GetString("gender", "")),
			Names:     names,
			Estates:   textRefsFromStrings(req.GetStringSlice("estates", nil)),
			Titles:    textRefsFromStrings(req.GetStringSlice("titles", nil)),
			Nicknames: textRefsFromStrings(req.GetStringSlice("nicknames", nil)),
			Notes:     textRefsFromStrings(req.GetStringSlice("notes", nil)),
			Sources:   sources,
			Private:   req.GetBool("private", false),
		}

		created, err := people.CreatePerson(ctx, p)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.PersonFromModel(created))
	}
}

func personUpdateHandler(people PersonService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := people.GetPerson(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		cur.Gender = models.PersonGender(req.GetString("gender", ""))
		cur.Estates = textRefsFromStrings(req.GetStringSlice("estates", nil))
		cur.Titles = textRefsFromStrings(req.GetStringSlice("titles", nil))
		cur.Nicknames = textRefsFromStrings(req.GetStringSlice("nicknames", nil))
		cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))
		cur.Private = req.GetBool("private", false)

		if raw, ok := req.GetArguments()["names"]; ok && raw != nil {
			names, err := optionalPersonNames(req.GetArguments(), "names")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Names = names
		}

		if raw, ok := req.GetArguments()["sources"]; ok && raw != nil {
			sources, err := optionalSourceLinks(req.GetArguments(), "sources")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Sources = sources
		}

		if err := people.UpdatePerson(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.PersonFromModel(cur))
	}
}

func personDeleteHandler(people PersonService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := people.DeletePerson(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
```

#### `internal/mcp/person_test.go` (создать)
```go
package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakePeople struct {
	list []models.Person
	err  error

	getP      models.Person
	created   models.Person
	gotCreate models.Person
	updated   models.Person
	gotIDs    []models.ID
	deleteErr error

	search []models.Person
}

func (f *fakePeople) ListPeople(context.Context, models.Access, models.Page) ([]models.Person, error) {
	return f.list, f.err
}

func (f *fakePeople) SearchPeople(context.Context, models.Access, models.SearchQuery) ([]models.Person, error) {
	return f.search, f.err
}

func (f *fakePeople) GetPerson(_ context.Context, _ models.Access, id models.ID) (models.Person, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Person{}, f.err
	}

	return f.getP, nil
}

func (f *fakePeople) CreatePerson(_ context.Context, p models.Person) (models.Person, error) {
	f.gotCreate = p
	if f.err != nil {
		return models.Person{}, f.err
	}

	return f.created, nil
}

func (f *fakePeople) UpdatePerson(_ context.Context, p models.Person) error {
	f.updated = p

	return f.err
}

func (f *fakePeople) DeletePerson(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callPersonTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestPersonGetToolContract(t *testing.T) {
	svc := &fakePeople{getP: models.Person{ID: "I-1", Gender: models.PersonGenderFemale}}

	res := callPersonTool(t, personGetHandler(svc), map[string]any{"id": "I-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "I-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

func TestPersonCreateToolPassesNames(t *testing.T) {
	svc := &fakePeople{created: models.Person{ID: "I-new"}}

	res := callPersonTool(t, personCreateHandler(svc), map[string]any{
		"gender": "female",
		"names": []any{
			map[string]any{
				"type":    "main",
				"surname": map[string]any{"text": "Дорожкина"},
				"given":   map[string]any{"text": "Акилина"},
			},
			map[string]any{
				"type":    "married",
				"surname": map[string]any{"text": "Петрова", "ref": "SN-01ARZ3NDEKTSV4RRFFQ69G5FA1", "type": "surname"},
			},
		},
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if len(svc.gotCreate.Names) != 2 || svc.gotCreate.Names[0].Surname.Text != "Дорожкина" {
		t.Fatalf("gotCreate.Names = %+v", svc.gotCreate.Names)
	}

	if svc.gotCreate.Names[1].Surname.Ref != "SN-01ARZ3NDEKTSV4RRFFQ69G5FA1" {
		t.Fatalf("gotCreate.Names[1] = %+v, ref не дошёл (мягкая ссылка должна округляться как есть)", svc.gotCreate.Names[1])
	}
}

func TestPersonUpdateToolSetsFields(t *testing.T) {
	svc := &fakePeople{getP: models.Person{ID: "I-1"}}

	res := callPersonTool(t, personUpdateHandler(svc), map[string]any{
		"id":      "I-1",
		"gender":  "male",
		"private": true,
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Private != true || svc.updated.Gender != models.PersonGenderMale {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

// TestPersonUpdateToolKeepsNamesWhenAbsent: отсутствие ключа "names" в
// вызове person_update сохраняет текущие имена — тот же принцип, что и у
// sources (см. TestFamilyUpdateToolSetsFields/family_update, подпроект 5).
func TestPersonUpdateToolKeepsNamesWhenAbsent(t *testing.T) {
	existing := []models.PersonName{{Surname: models.TextRef{Text: "Иванова"}}}
	svc := &fakePeople{getP: models.Person{ID: "I-1", Names: existing}}

	res := callPersonTool(t, personUpdateHandler(svc), map[string]any{
		"id":     "I-1",
		"gender": "female",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if len(svc.updated.Names) != 1 || svc.updated.Names[0].Surname.Text != "Иванова" {
		t.Fatalf("updated.Names = %+v, want сохранённые текущие имена", svc.updated.Names)
	}
}

// TestPersonUpdateToolClearsNamesWhenEmptyArray: пустой массив names,
// наоборот, очищает список — отличие "отсутствует" от "пусто".
func TestPersonUpdateToolClearsNamesWhenEmptyArray(t *testing.T) {
	existing := []models.PersonName{{Surname: models.TextRef{Text: "Иванова"}}}
	svc := &fakePeople{getP: models.Person{ID: "I-1", Names: existing}}

	res := callPersonTool(t, personUpdateHandler(svc), map[string]any{
		"id":     "I-1",
		"gender": "female",
		"names":  []any{},
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if len(svc.updated.Names) != 0 {
		t.Fatalf("updated.Names = %+v, want пустой список", svc.updated.Names)
	}
}

func TestPersonDeleteToolInUseIsError(t *testing.T) {
	svc := &fakePeople{deleteErr: &models.InUseError{Type: models.TypePerson, ID: "I-1"}}

	res := callPersonTool(t, personDeleteHandler(svc), map[string]any{"id": "I-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersPersonTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, People: &fakePeople{}}).ListTools()

	for _, name := range []string{"person_list", "person_search", "person_get", "person_create", "person_update", "person_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
```

### 1.5. `internal/mcp/object_args.go` — добавить `PersonName`-хелперы

Изменить существующий файл (добавлены ровно две функции после `optionalSourceLinks`, остальное содержимое — хелперы подпроектов 3 и 5 — переносится без изменений) — итоговое содержимое ниже. `personNameObjectProperties()` — тот же технический приём, что и `sourceLinkObjectProperties()` (подпроект 5, `mcp.Items` с произвольной JSON-schema вместо только примитивов), но с более сложным элементом: поля `surname`/`given`/`patronymic` сами описаны вложенным объектом через `textRefObjectProperties()`, `since`/`until` — через `factDateObjectProperties()` (обе — из подпроекта 3). Это первый случай в программе, когда элемент MCP-массива сам несёт вложенные MCP-объекты, а не только плоские скаляры. `optionalPersonNames` — тот же приём marshal/unmarshal сырого `[]any` из аргументов тула в `[]transport.PersonName`, что и `optionalSourceLinks` для `[]transport.SourceLink`.
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

### 1.6. `Deps`-реестр: подключить Person

Механическое добавление поля `People`/`PersonService` в каждый из шести файлов — по образцу подключения `Families`/`FamilyService` в подпроекте 7 (поле размещается в конце каждой структуры `Deps`, после `Families`, по хронологическому порядку подпроектов, не по алфавиту).
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
	create_family "github.com/amarin/genodex/internal/usecases/create_family"
	create_given_name "github.com/amarin/genodex/internal/usecases/create_given_name"
	create_note "github.com/amarin/genodex/internal/usecases/create_note"
	create_parish "github.com/amarin/genodex/internal/usecases/create_parish"
	create_patronymic "github.com/amarin/genodex/internal/usecases/create_patronymic"
	create_person "github.com/amarin/genodex/internal/usecases/create_person"
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
	delete_family "github.com/amarin/genodex/internal/usecases/delete_family"
	delete_given_name "github.com/amarin/genodex/internal/usecases/delete_given_name"
	delete_note "github.com/amarin/genodex/internal/usecases/delete_note"
	delete_parish "github.com/amarin/genodex/internal/usecases/delete_parish"
	delete_patronymic "github.com/amarin/genodex/internal/usecases/delete_patronymic"
	delete_person "github.com/amarin/genodex/internal/usecases/delete_person"
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
	get_family "github.com/amarin/genodex/internal/usecases/get_family"
	get_given_name "github.com/amarin/genodex/internal/usecases/get_given_name"
	get_note "github.com/amarin/genodex/internal/usecases/get_note"
	get_parish "github.com/amarin/genodex/internal/usecases/get_parish"
	get_patronymic "github.com/amarin/genodex/internal/usecases/get_patronymic"
	get_person "github.com/amarin/genodex/internal/usecases/get_person"
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
	list_families "github.com/amarin/genodex/internal/usecases/list_families"
	list_given_names "github.com/amarin/genodex/internal/usecases/list_given_names"
	list_notes "github.com/amarin/genodex/internal/usecases/list_notes"
	list_parishes "github.com/amarin/genodex/internal/usecases/list_parishes"
	list_patronymics "github.com/amarin/genodex/internal/usecases/list_patronymics"
	list_people "github.com/amarin/genodex/internal/usecases/list_people"
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
	update_family "github.com/amarin/genodex/internal/usecases/update_family"
	update_given_name "github.com/amarin/genodex/internal/usecases/update_given_name"
	update_note "github.com/amarin/genodex/internal/usecases/update_note"
	update_parish "github.com/amarin/genodex/internal/usecases/update_parish"
	update_patronymic "github.com/amarin/genodex/internal/usecases/update_patronymic"
	update_person "github.com/amarin/genodex/internal/usecases/update_person"
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

### 1.7. Real-store интеграционные тесты

`internal/httpapi/store_test.go` — добавлен `personService`-фасад (мирроит `familyService`) + `newPersonService`, используемые `write_store_test.go`. `internal/httpapi/write_store_test.go` — добавлен `TestPersonWriteContractWithRealStore` (реальный `sqlstore.Open`) + хелперы `createPerson`/`decodePersonS`/`postPersonReq`/`putPersonReq`; тест явно проверяет и мягкий `Names[i]` ref (без проверки существования), и поиск по фрагменту фамилии, встречающемуся только во ВТОРОЙ (не основной) записи `Names` (см. «Глобальные ограничения»), и переключение `private` `false → true` (не `private → private`, по уроку финального ревью подпроекта 7 — см. коммит `28127d0` на `main`).
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
	create_family "github.com/amarin/genodex/internal/usecases/create_family"
	create_given_name "github.com/amarin/genodex/internal/usecases/create_given_name"
	create_note "github.com/amarin/genodex/internal/usecases/create_note"
	create_parish "github.com/amarin/genodex/internal/usecases/create_parish"
	create_patronymic "github.com/amarin/genodex/internal/usecases/create_patronymic"
	create_person "github.com/amarin/genodex/internal/usecases/create_person"
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
	delete_family "github.com/amarin/genodex/internal/usecases/delete_family"
	delete_given_name "github.com/amarin/genodex/internal/usecases/delete_given_name"
	delete_note "github.com/amarin/genodex/internal/usecases/delete_note"
	delete_parish "github.com/amarin/genodex/internal/usecases/delete_parish"
	delete_patronymic "github.com/amarin/genodex/internal/usecases/delete_patronymic"
	delete_person "github.com/amarin/genodex/internal/usecases/delete_person"
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
	get_family "github.com/amarin/genodex/internal/usecases/get_family"
	get_given_name "github.com/amarin/genodex/internal/usecases/get_given_name"
	get_note "github.com/amarin/genodex/internal/usecases/get_note"
	get_parish "github.com/amarin/genodex/internal/usecases/get_parish"
	get_patronymic "github.com/amarin/genodex/internal/usecases/get_patronymic"
	get_person "github.com/amarin/genodex/internal/usecases/get_person"
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
	list_families "github.com/amarin/genodex/internal/usecases/list_families"
	list_given_names "github.com/amarin/genodex/internal/usecases/list_given_names"
	list_notes "github.com/amarin/genodex/internal/usecases/list_notes"
	list_parishes "github.com/amarin/genodex/internal/usecases/list_parishes"
	list_patronymics "github.com/amarin/genodex/internal/usecases/list_patronymics"
	list_people "github.com/amarin/genodex/internal/usecases/list_people"
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
	update_family "github.com/amarin/genodex/internal/usecases/update_family"
	update_given_name "github.com/amarin/genodex/internal/usecases/update_given_name"
	update_note "github.com/amarin/genodex/internal/usecases/update_note"
	update_parish "github.com/amarin/genodex/internal/usecases/update_parish"
	update_patronymic "github.com/amarin/genodex/internal/usecases/update_patronymic"
	update_person "github.com/amarin/genodex/internal/usecases/update_person"
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
	public := createPerson(t, h, owner, `{"names":[],"estates":[],"titles":[],"nicknames":[],"notes":[],"private":false}`, http.StatusCreated)
	requireStatusS(t, getReq(t, h, "/api/people/"+string(public.ID)), http.StatusOK)

	rec = putPersonReq(t, h, owner, "/api/people/"+string(public.ID),
		`{"names":[],"estates":[],"titles":[],"nicknames":[],"notes":[],"private":true}`)
	requireStatusS(t, rec, http.StatusOK)
	afterUpdate := decodePersonS(t, rec)
	if !afterUpdate.Private {
		t.Fatalf("private (после update) = %+v, want Private=true", afterUpdate)
	}
	requireStatusS(t, getReq(t, h, "/api/people/"+string(public.ID)), http.StatusNotFound)

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
```

### 1.8. Документация: `docs/usage.md`, `docs/architecture.md`, `CHANGELOG.md`, `entity-write.md` §3.6/§3.7/§5

По правилу, закреплённому после подпроекта 4 (доки/CHANGELOG пишутся в плане, а не оставляются на финальное ревью) — ниже точные вставки/замены в уже существующие большие файлы; порядок существующих строк не меняется, только добавляются/заменяются указанные фрагменты. Эти фрагменты закрывают ТОЛЬКО бэкенд-часть доков — см. «Предпосылка»: `CHANGELOG.md`-пункт ниже буквально содержит оговорку «(веб — отдельная работа)», а §3.7/§5 `entity-write.md` — фразы «отдельная работа»/«вне объёма этого прохода» про веб; Шаг 2.4 (Задача 2) эти формулировки заменяет на описание реально построенного.

#### `docs/usage.md` — новая строка HTTP-таблицы (после строки `/api/families`, до `/api/notes`) и правка ссылки на `Person` в самой строке `/api/families`
Заменить существующую строку `/api/families` (правка последней фразы — ссылка на будущий CRUD персоны становится ссылкой на уже существующий) и вставить сразу после неё новую строку `/api/people`:
```markdown
| `/api/families` | Роды/линии (JSON): `GET` — список `[{"id", "name", "members", "notes", "sources", "private"}]`; `POST` — создание; `GET/PUT/DELETE /api/families/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404, как отсутствующая); `GET /api/families/search?q=` — поиск по началу названия. `members` — мягкая ссылка на `Person` (`TypePerson`, полный CRUD — `/api/people`, ниже): `TextRef.Ref`, если задан, только проверяется по формату, существование не проверяется — тот же принцип, что и у любого другого списка `TextRef` в программе. |
| `/api/people` | Персоны (JSON) — ядро графа генеалогии (подпроект 8): `GET` — список `[{"id", "gender", "names", "estates", "titles", "nicknames", "notes", "sources", "private"}]`; `POST` — создание; `GET/PUT/DELETE /api/people/{id}` — чтение/изменение/удаление записи (приватная запись для не-владельца — 404, как отсутствующая); `GET /api/people/search?q=` — ищет по началу фамилии, имени или отчества из ЛЮБОГО элемента `names` (не только основного) — все имена персоны индексируются под единым полем `name`; `estates`/`titles`/`nicknames`/`notes` поиском не охватываются. `gender` — необязательный `male`/`female`/`unknown`. `names` — список вложенных объектов `{type?, surname, given, patronymic, prefix?, suffix?, since?, until?}`: `type` — `main`/`birth`/`married`/`changed`/`pseudonym` (пусто — не указан); `surname`/`given`/`patronymic` — обычные `TextRef` (мягкая ссылка на `Surname`/`GivenName`/`Patronymic`, существование НЕ проверяется — тот же принцип, что и у любого другого `TextRef` в программе), хотя бы одна часть должна быть заполнена; `since`/`until` — структурированная дата (`FactDate`). `estates`/`titles` — мягкие `TextRef` на `Estate`/`Title`; `nicknames`/`notes` — обычные нетипизированные `TextRef`. Единственная строго проверяемая ссылка — `sources[i].citation_id` (как и везде): несуществующая цитата — 422 на поле `sources[i].citation_id`. |
```

#### `docs/usage.md` — дополнение к абзацу про `sources` (после HTTP-таблицы)
Заменить предложение в конце существующего абзаца про `sources`:
```markdown
`/api/archive-nodes`/`/api/archive-documents` (подпроект 6) и `/api/families` (подпроект 7) — первые полностью новые сущности программы с `sources`, редактируемым с самого начала (не ретрофит).
```
на:
```markdown
`/api/archive-nodes`/`/api/archive-documents` (подпроект 6), `/api/families` (подпроект 7) и `/api/people` (подпроект 8) — первые полностью новые сущности программы с `sources`, редактируемым с самого начала (не ретрофит).
```

#### `docs/usage.md` — 6 новых строк таблицы MCP-тулов (после `family_delete`, до абзаца про `sources`-аргумент) и дополнение самого абзаца
Вставить после строки `family_delete`:
```markdown
| `person_list` | Список персон; аргументы `limit`, `offset` |
| `person_search` | Поиск персон по началу фамилии, имени или отчества из ЛЮБОГО из имён персоны (не только основного); аргументы `q`, `limit`, `offset`; пустой `q` — пустой результат |
| `person_get` | Запись по `id` (JSON контракта); приватная запись для не-владельца — ошибка тула (как отсутствующая) |
| `person_create` | Создание записи: все поля необязательны — `gender` (`male`/`female`/`unknown`), `names` (массив объектов `{type?, surname, given, patronymic, prefix?, suffix?, since?, until?}` — первая вложенная подформа-«массив объектов» в программе, см. `personNameObjectProperties`; `surname`/`given`/`patronymic` — мягкие ссылки, существование не проверяется), `estates`/`titles`/`nicknames`/`notes` (тексты; `estates`/`titles` — мягкая ссылка на словарь, без проверки существования), `sources`, `private`; id генерирует сервер |
| `person_update` | Изменение записи: полная замена `gender`/`estates`/`titles`/`nicknames`/`notes`/`private`; `estates`/`titles`/`nicknames`/`notes` принимают только текст (без ref/type в MCP-контракте) — ссылка на элементе теряется при любом обновлении, пока не появится picker; `names` и `sources` — при отсутствии в вызове текущее значение сохраняется, пустой массив — очищает его |
| `person_delete` | Удаление записи по `id`; занятая другой сущностью — ошибка тула |
```
Заменить последнее предложение существующего абзаца про `sources`-аргумент MCP:
```markdown
`<entity>_create`/`<entity>_update` у `division`/`repository`/`church`/`parish`/`archive`/`note` с подпроекта 5 принимают аргумент `sources` — массив объектов `{citation_id, reliability?, role?, note?}` (первый MCP-аргумент вида «массив объектов» в программе, см. `sourceLinkObjectProperties`; раньше массивы были только строками). Несуществующий `citation_id` — ошибка тула на поле `sources[i].citation_id`. В отличие от остальных полей `*_update` (которые заменяются полностью, вплоть до значения по умолчанию, если не переданы), `sources` — исключение: отсутствие аргумента `sources` в вызове `*_update` сохраняет текущие источники записи как есть; чтобы очистить их, нужно явно передать `"sources": []` (в отличие от HTTP `PUT`, где отсутствие ключа `sources` в теле запроса всегда очищает список, см. выше).
```
на:
```markdown
`<entity>_create`/`<entity>_update` у `division`/`repository`/`church`/`parish`/`archive`/`note` с подпроекта 5 принимают аргумент `sources` — массив объектов `{citation_id, reliability?, role?, note?}` (первый MCP-аргумент вида «массив объектов» в программе, см. `sourceLinkObjectProperties`; раньше массивы были только строками). Несуществующий `citation_id` — ошибка тула на поле `sources[i].citation_id`. В отличие от остальных полей `*_update` (которые заменяются полностью, вплоть до значения по умолчанию, если не переданы), `sources` — исключение: отсутствие аргумента `sources` в вызове `*_update` сохраняет текущие источники записи как есть; чтобы очистить их, нужно явно передать `"sources": []` (в отличие от HTTP `PUT`, где отсутствие ключа `sources` в теле запроса всегда очищает список, см. выше). `person_create`/`person_update` (подпроект 8) — первый массив объектов, чьи собственные элементы тоже вложенные объекты (`surname`/`given`/`patronymic` — `TextRef`, `since`/`until` — `FactDate`); `person_update` расширяет исключение «отсутствие аргумента сохраняет текущее значение» с `sources` также на `names`.
```

#### `docs/architecture.md` — строка `internal/httpapi/`
Заменить:
```markdown
| `internal/httpapi/` | Слой HTTP API: JSON-роуты `/api/*` (health, admin-divisions, surnames, patronymics, estates, titles, given-names, repositories, churches, parishes, archives, archive-nodes, archive-documents, notes, attachments, sources, citations, families, auth, docs). Использует те же сценарии, что и MCP. |
```
на:
```markdown
| `internal/httpapi/` | Слой HTTP API: JSON-роуты `/api/*` (health, admin-divisions, surnames, patronymics, estates, titles, given-names, repositories, churches, parishes, archives, archive-nodes, archive-documents, notes, attachments, sources, citations, families, people, auth, docs). Использует те же сценарии, что и MCP. |
```

#### `CHANGELOG.md` — новый пункт (после пункта про Family, перед «Веб: единая точка входа»)
Вставить перед пунктом «Веб: единая точка входа»:
```markdown
- Персоны (`Person`) — подпроект 8, backend (веб — отдельная работа): ядро
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
  `internal/mcp/person.go`).
```

#### `docs/data-model/entity-write.md` — правка §3.6 (Family `Members` bullet) и новая секция §3.7 (после §3.6, до `## 4.`)
Заменить в §3.6 существующий пункт про `Members`:
```markdown
- **`Members` — мягкая ссылка на `Person`, `TypePerson`.** `Person` не
  получит CRUD до подпроекта 8 — как и `ArchiveNode`/`ArchiveDocument` до
  подпроекта 6 (§3.2), это не мешает `Family` ссылаться на него: `TextRef`
```
на:
```markdown
- **`Members` — мягкая ссылка на `Person`, `TypePerson`.** На момент этого
  подпроекта `Person` ещё не имел CRUD (получил его в подпроекте 8, §3.7) —
  как и `ArchiveNode`/`ArchiveDocument` до подпроекта 6 (§3.2), это не
  мешает `Family` ссылаться на него: `TextRef`
```
Вставить новую секцию §3.7 после §3.6, до `## 4. Веб-UI конвенции`:
```markdown
### 3.7. Подпроект 8 (`Person`) — вложенная подформа `PersonName`, backend

- **Ядро графа генеалогии, но без нового FK-паттерна.** `Person` —
  `{ID, Gender, Names []PersonName, Estates []TextRef, Titles []TextRef,
  Nicknames []TextRef, Notes []TextRef, Sources []SourceLink, Private bool}`
  — все поля, кроме `ID`, опциональны, включая `Gender` (пустая строка —
  «не указан»). Единственная строго проверяемая ссылка — по-прежнему
  `Sources[i].CitationID` (`InTx`+`tx.GetCitation`, образец —
  `create_family`/`update_family`); `Person` доступен и access-aware (образец
  — `get_family`). Новый содержательный элемент этого подпроекта —
  вложенная подформа `[]PersonName`, а не новый вид FK.
- **`PersonName` — первая вложенная подформа-«массив объектов» в
  программе**, а не просто список `TextRef`. Каждый элемент —
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
- **Backend-only подпроект.** Веб-страницы (`PersonForm`/`PersonView`/
  `PeopleList`, редактор подформы `PersonName`) — отдельная работа,
  выполняется отдельно от этого прохода; конвенции §3-4 (в т.ч. `TextRef`-
  списки только текстом в v1, без пикера) применяются к ним без изменений,
  когда до них дойдёт очередь.
```
Это ровно то, что уже присутствует в проверочном worktree на момент написания этого плана (последний пункт «Backend-only подпроект» отражает состояние ПОСЛЕ Задачи 1, ДО Задачи 2) — Шаг 2.4 ниже заменяет именно этот последний пункт на описание реально построенного веб-редактора, отдельной вставкой поверх Шага 1.8, а не переписыванием его здесь заново.

#### `docs/data-model/entity-write.md` — правка §5 (абзац про `[]PersonName`)
Заменить существующий абзац:
```markdown
- Вложенные подформы Person (`[]PersonName`) и участники Event
  (`[]EventParticipant`) — детальный дизайн откладывается до
  подпроектов 8-9, когда до них дойдёт очередь; общие конвенции §3-4
  всё равно применяются как основа.
```
на:
```markdown
- Вложенная подформа Person (`[]PersonName`) backend — реализована в
  подпроекте 8 (§3.7); её веб-форма (`PersonForm`/`PersonView`, редактор
  `[]PersonName`) — отдельная работа, вне объёма этого прохода. Участники
  Event (`[]EventParticipant`) — детальный дизайн по-прежнему откладывается
  до подпроекта 9, когда до него дойдёт очередь; общие конвенции §3-4 всё
  равно применяются как основа.
```
И это тоже — состояние ПОСЛЕ Задачи 1, ДО Задачи 2; Шаг 2.4 правит фразу «отдельная работа, вне объёма этого прохода» ещё раз, когда веб реально готов.

### 1.9. Рубеж

`gofmt -l .` пусто; `go build ./...`, `go vet ./...`, `go test ./...` (весь репозиторий) зелёные — ожидается 1293 теста, 119 пакетов (было 1252 после подпроекта 7). Живой смок через `curl` (см. «Предпосылка») — не обязателен повторно: создать `Person` с двумя `Names` (main+married), один из мягких `ref`-ов на словарную сущность — round-trip без проверки существования; поиск по фрагменту фамилии из ВТОРОЙ записи `Names`; `sources[0].citation_id`, указывающий на несуществующую цитату, — 422 на этом поле; `PUT` `private:false → true`; `DELETE` → 204, повторный `GET` → 404.

### 1.10. Коммит

```bash
git add \
  internal/transport/person.go internal/transport/person_name.go internal/transport/person_write.go \
  internal/usecases/list_people internal/usecases/search_people internal/usecases/get_person internal/usecases/create_person internal/usecases/update_person internal/usecases/delete_person \
  internal/httpapi/person.go internal/httpapi/person_write.go internal/httpapi/person_test.go internal/httpapi/person_write_test.go \
  internal/mcp/person.go internal/mcp/person_test.go \
  internal/httpapi/deps.go internal/httpapi/api.go internal/httpapi/httpapi.go internal/mcp/deps.go internal/mcp/server.go internal/mcp/object_args.go internal/app/app.go \
  internal/httpapi/write_store_test.go internal/httpapi/store_test.go \
  docs/usage.md docs/architecture.md CHANGELOG.md docs/data-model/entity-write.md
git commit -m "feat(backend): Person — полный CRUD (usecases/httpapi/mcp) + доки + real-store тесты"
```

## Задача 2. Веб: `PersonNameListEditor` + страницы Person + роуты/каталог

**Интерфейсы, потребляемые из Задачи 1**: HTTP-контракт `/api/people*` (Шаги 1.3, 1.6).
**Интерфейсы, потребляемые с веба подпроектов 1-7**: `TextRefListEditor`, `SourceLinkListEditor` (переиспользуемые редакторы списков `TextRef`/`SourceLink`, используются уже `FamilyForm`/`FamilyView` и другими страницами), `FactDateEditor` (структурированная дата, подпроект 3, используется внутри `PersonNameListEditor` для `since`/`until`), общий каркас страницы-тройки (List/View+Form) по образцу `FamiliesList.tsx`/`FamilyForm.tsx`/`FamilyView.tsx`, ref-preservation логика одиночного `*TextRef` из `ChurchView.tsx` (подпроект 3) — источник идеи для `PersonNameListEditor`, но не копируемый код (другая механика, см. design judgment call №1 в «Предпосылке»).
**Производит**: новый компонент `PersonNameListEditor` (`web/src/PersonNameList.tsx`), страницы `/people`, `/people/:id`, строку «Персоны» в каталоге сущностей.

**Файлы:**
- Изменить: `web/src/api.ts`, `web/src/App.tsx`, `web/src/pages/EntityCatalog.tsx`, `CHANGELOG.md`, `docs/data-model/entity-write.md` (докный шаг 2.4 — см. «Предпосылка», случай (a): доки Задачи 1 описывают только бэкенд)
- Создать: `web/src/PersonNameList.tsx`, `web/src/pages/{PeopleList,PersonForm,PersonView}.tsx`

**Важно для исполнителя**: `PersonNameList.tsx` — новый переиспользуемый паттерн, не копия существующего компонента; остальные три файла — копия страниц `Family` с добавлением полей `gender` (`Select`) и `names` (`PersonNameListEditor`). `PersonNameListEditor` использует **baseline-per-mount** ref-preservation (design judgment call №1 в «Предпосылке»): `rowMeta` — `{id, base: PersonName}[]`, захватывается ОДИН РАЗ через `useState`-инициализатор от входного `value` (НЕ `useEffect`), ключ строки — стабильный id, присвоенный при монтировании/добавлении строки, НЕ индекс массива. Каждое изменение `surname`/`given`/`patronymic` сравнивает новый текст с ИСХОДНЫМ текстом ЭТОЙ строки из базовой линии; совпадает ⇒ сохраняется весь исходный `{text, ref, type}`; отличается ⇒ испускается `{text}` без `ref`/`type`. Работает корректно только потому, что оба вызывающих места (`PersonForm.tsx` с `destroyOnHidden`, `PersonView.tsx`'s `{editing ? <Form>... : ...}`) реально размонтируют/монтируют компонент заново на каждую сессию редактирования — переносить как есть, не менять на `useEffect`-пересинхронизацию без явного запроса пользователя (это изменило бы поведение с «сохранять baseline навсегда» на «пересчитывать baseline при каждом апдейте `value` извне», что ломает саму суть ref-preservation). Переносить код ниже как есть, файл за файлом — итоговое содержимое.

### 2.1. `PersonNameListEditor` (создать)

`web/src/PersonNameList.tsx` — первое «array of objects» подформа на вебе программы (после `TextRefListEditor`/`SourceLinkListEditor`, которые редактируют более простые элементы). Каждая строка: селектор `type` (`PERSON_NAME_TYPE_OPTIONS`, пустая опция `""` = «Не указан(о)», тот же паттерн, что `FactDateEditor.tsx`'s `CALENDAR_OPTIONS`), три текстовых поля `surname`/`given`/`patronymic` (обычные `Input`, без пикера — см. design judgment call №4), `prefix`/`suffix` (свободный текст), период `since`/`until` (`FactDateEditor` x2). Модуль также экспортирует `PERSON_NAME_TYPE_OPTIONS`, `personNameTypeLabel`, `formatPersonName` ("Фамилия Имя Отчество", пропускает пустые части) и `pickDisplayName` (главное имя, иначе первое, иначе `null`) — используются `PeopleList.tsx` и `PersonView.tsx` для заголовка/строки.
#### `web/src/PersonNameList.tsx` (создать)
```tsx
import { useRef, useState } from "react";
import { Button, Divider, Input, Select, Space } from "antd";
import { MinusCircleOutlined, PlusOutlined } from "@ant-design/icons";
import type { PersonName, PersonNameType, TextRef } from "./api";
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

### 2.2. Страницы Person

`PeopleList.tsx` — плоский список (маршрут `/people`), зеркалирует `FamiliesList.tsx` (пагинация через `MAX_PAGE_LIMIT`, поле поиска, «+ добавить», переход по клику на View); строка списка использует `pickDisplayName`+`formatPersonName`, падая обратно на `id`, когда `names` пуст или все части пусты. `PersonForm.tsx` — `CreatePersonModal`, зеркалирует разбиение `FamilyForm.tsx`'s `CreateFamilyModal` (создание через модалку, без отдельного маршрута `/people/new` — как и у `/families`), добавляет `gender` (`Select`) и `names` (`PersonNameListEditor`) поверх паттерна `estates/titles/nicknames/notes` (`TextRefListEditor`) + `sources` (`SourceLinkListEditor`) + `private` (`Checkbox`). `PersonView.tsx` — страница просмотра+инлайн-редактирования (маршрут `/people/:id`), зеркалирует структуру `FamilyView.tsx` (тот же toggle+явное сохранение через `PUT`, та же обработка 404/409 через `ApiError`); read-only-секция «Имена» форматирует каждую строку как «тип — Фамилия Имя Отчество — приставка — суффикс — (since – until)» через локальный хелпер `personNameLine`, построенный на `formatPersonName` + `personNameTypeLabel` + `formatFactDate`.
#### `web/src/pages/PeopleList.tsx` (создать)
```tsx
import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Input, List, Spin } from "antd";
import { fetchPeople, searchPeople, MAX_PAGE_LIMIT, type Person } from "../api";
import { useSession } from "../session";
import { pickDisplayName, formatPersonName } from "../PersonNameList";
import { CreatePersonModal } from "./PersonForm";

// personLabel — строка списка: "main" имя, иначе первое из Names,
// отформатированное как "Фамилия Имя Отчество" (PersonNameList.formatPersonName);
// если Names пуст или все части пустые — id (docs/data-model/entity-write.md §3.7).
function personLabel(p: Person): string {
  const name = pickDisplayName(p.names);
  const formatted = name != null ? formatPersonName(name) : "";
  return formatted !== "" ? formatted : p.id;
}

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
                <Link to={`/people/${p.id}`}>{personLabel(p)}</Link>
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

#### `web/src/pages/PersonForm.tsx` (создать)
```tsx
import { useState } from "react";
import { Alert, Checkbox, Form, Modal, Select } from "antd";
import {
  createPerson,
  GENDER_OPTIONS,
  type Person,
  type PersonGender,
  type PersonName,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError } from "../auth";
import { PersonNameListEditor } from "../PersonNameList";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";

interface PersonFormValues {
  gender?: PersonGender;
  private?: boolean;
}

const FORM_FIELDS: (keyof PersonFormValues)[] = ["gender"];

// CreatePersonModal — форма создания персоны, структурно по образцу
// CreateFamilyModal (подпроект 7): та же сущность плюс gender (Select) и
// names — новая вложенная подформа-«массив объектов» (PersonNameListEditor,
// docs/data-model/entity-write.md §3.7), вместо простого TextRefListEditor.
// Sources редактируется с рождения контракта (SourceLinkListEditor).
export function CreatePersonModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Person) => void;
}) {
  const [form] = Form.useForm<PersonFormValues>();
  const [names, setNames] = useState<PersonName[]>([]);
  const [estates, setEstates] = useState<TextRef[]>([]);
  const [titles, setTitles] = useState<TextRef[]>([]);
  const [nicknames, setNicknames] = useState<TextRef[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setNames([]);
    setEstates([]);
    setTitles([]);
    setNicknames([]);
    setNotes([]);
    setSources([]);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: PersonFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await createPerson({
        gender: values.gender ?? "",
        names,
        estates,
        titles,
        nicknames,
        notes,
        sources,
        private: values.private ?? false,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof PersonFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить персону"
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
      <Form form={form} layout="vertical" onFinish={onFinish} initialValues={{ gender: "", private: false }}>
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
      </Form>
    </Modal>
  );
}
```

#### `web/src/pages/PersonView.tsx` (создать)
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
import { formatPersonName, personNameTypeLabel, PersonNameListEditor, pickDisplayName } from "../PersonNameList";
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
  const typeLabel = personNameTypeLabel(n.type);
  if (typeLabel !== "") {
    parts.push(typeLabel);
  }
  const name = formatPersonName(n);
  parts.push(name !== "" ? name : "—");
  if (n.prefix) {
    parts.push(n.prefix);
  }
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

  const displayName = pickDisplayName(person.names);
  const title = displayName != null && formatPersonName(displayName) !== "" ? formatPersonName(displayName) : person.id;

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

### 2.3. `api.ts`, `App.tsx`, `EntityCatalog.tsx` — типы/функции, роуты, каталог

`web/src/api.ts` — добавлено (в самом конце файла, после `ArchiveDocument`, там же, где живут конвенции `Family`) `PersonName`, `PersonNameType`, `Person`, `PersonGender` (+ `GENDER_OPTIONS`/`genderLabel`, по образцу `ADMIN_DIVISION_TYPE_LABELS`/`adminDivisionTypeLabel`), `PersonInput` (`names`/`sources` обязательны, как у любого другого `*Input` в программе), `PersonQuery`/`PersonSearchQuery`, и `fetchPeople`/`searchPeople`/`fetchPerson`/`createPerson`/`updatePerson`/`deletePerson` — та же форма, что и функции `Family` (обычный `fetch` для list/search, `authFetch` для get/create/update/delete, та же обработка `ApiError`/409-`referrers`, унаследованная от `authFetch`). `App.tsx` — добавлены маршруты `/people` и `/people/:id` (без отдельного маршрута создания, как у `/families`). `EntityCatalog.tsx` — добавлена строка `{ label: "Персоны", path: "/people" }` (каталог автосортируется по алфавиту, позиция в исходном массиве не важна).
#### `web/src/api.ts` (изменить — итоговое содержимое)
```typescript
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
  { label: "Роды", path: "/families" },
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

### 2.4. Документация: расширить `CHANGELOG.md` и `entity-write.md` веб-частью

Явный докный шаг, добавленный по инструкции этого подпроекта (см. «Предпосылка», случай (a)) — `docs/usage.md`/`docs/architecture.md` НЕ трогаются (по конвенции репозитория не описывают веб-страницы вообще, см. Предпосылку и аналогичный вывод плана подпроекта 7); `CHANGELOG.md` и `docs/data-model/entity-write.md` дополняются, чтобы после Задачи 2 доки полностью описывали и бэкенд, и веб, а не только бэкенд, как оставила их Задача 1.

#### `CHANGELOG.md` — дополнить пункт про Person (добавленный Шагом 1.8) веб-частью
Заменить первую строку пункта:
```markdown
- Персоны (`Person`) — подпроект 8, backend (веб — отдельная работа): ядро
```
на:
```markdown
- Персоны (`Person`) — подпроект 8: ядро
```
И добавить в конец того же пункта (перед закрывающей скобкой со списком файлов бэкенда), новое предложение и веб-файлы в список:
```markdown
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
```

#### `docs/data-model/entity-write.md` — заменить последний пункт §3.7 (веб теперь готов)
Заменить пункт, добавленный Шагом 1.8:
```markdown
- **Backend-only подпроект.** Веб-страницы (`PersonForm`/`PersonView`/
  `PeopleList`, редактор подформы `PersonName`) — отдельная работа,
  выполняется отдельно от этого прохода; конвенции §3-4 (в т.ч. `TextRef`-
  списки только текстом в v1, без пикера) применяются к ним без изменений,
  когда до них дойдёт очередь.
```
на:
```markdown
- **Веб: `PersonNameListEditor` — первый в программе повторяющийся
  форма-редактор с несколькими текстовыми полями и датой на строку.**
  `web/src/PersonNameList.tsx` редактирует `[]PersonName` построчно (тип,
  три текстовых поля `surname`/`given`/`patronymic` без пикера — как
  `Church.Parish`, `prefix`/`suffix`, `FactDateEditor` x2 для
  `since`/`until`). Ref-preservation для `surname`/`given`/`patronymic`
  построена на **baseline захваченной один раз при монтировании
  компонента** (`rowMeta: {id, base: PersonName}[]`, `useState`-
  инициализатор, НЕ `useEffect`-пересинхронизация), с ключом по
  стабильному id строки (не по индексу массива, чтобы добавление/удаление
  строк не путало баланс): текст поля, не изменившийся относительно этой
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
```

#### `docs/data-model/entity-write.md` — заменить абзац §5 про `[]PersonName` (веб теперь в объёме)
Заменить абзац, изменённый Шагом 1.8:
```markdown
- Вложенная подформа Person (`[]PersonName`) backend — реализована в
  подпроекте 8 (§3.7); её веб-форма (`PersonForm`/`PersonView`, редактор
  `[]PersonName`) — отдельная работа, вне объёма этого прохода. Участники
  Event (`[]EventParticipant`) — детальный дизайн по-прежнему откладывается
  до подпроекта 9, когда до него дойдёт очередь; общие конвенции §3-4 всё
  равно применяются как основа.
```
на:
```markdown
- Вложенная подформа Person (`[]PersonName`) реализована целиком в
  подпроекте 8 — backend в §3.7, веб (`PersonNameListEditor`,
  `PersonForm`/`PersonView`) там же. Участники Event
  (`[]EventParticipant`) — детальный дизайн по-прежнему откладывается до
  подпроекта 9, когда до него дойдёт очередь; общие конвенции §3-4 всё
  равно применяются как основа.
```

### 2.5. Рубеж

`cd web && npm run typecheck` (чисто), `npm run build` (чисто). Живая проверка (см. «Предпосылка») — не обязательна повторно: `/people` список рендерится, «+ добавить» открывает `CreatePersonModal` с `Пол`+`Имена`+словарными списками+`Доказательства`+«Приватная запись»; заполнение двух строк `Names` (одна — Фамилия+Имя, другая — только Фамилия) через «+ имя», отправка → переход на `/people/{id}`, `PersonView` корректно показывает заголовок (главное имя) и обе строки «Имена»; «Редактировать» на записи, созданной с уже проставленными `ref` на `given`/`patronymic`, → изменение ТОЛЬКО фамилии → «Сохранить» → повторный фетч подтверждает: `surname` потерял `ref`/`type`, `given`/`patronymic` сохранили их неизменными в том же самом сохранении.

### 2.6. Коммит

```bash
git add web/src/PersonNameList.tsx web/src/pages/PeopleList.tsx web/src/pages/PersonForm.tsx web/src/pages/PersonView.tsx web/src/api.ts web/src/App.tsx web/src/pages/EntityCatalog.tsx CHANGELOG.md docs/data-model/entity-write.md
git commit -m "feat(web): страницы Person — PersonNameListEditor, список/просмотр/редактирование/создание, каталог + доки"
```

## Рубеж прохода

После Задачи 2: `gofmt -l .` пусто, `go build/vet/test ./...` зелёные (1293 теста, 119 пакетов), `npm run typecheck`/`build` чисты. Каталог `/` — на одну строку больше, «Персоны» в алфавитном порядке. `Person` имеет полный CRUD через HTTP, MCP и веб, по конвенциям `docs/data-model/entity-write.md` §3-4 и новой §3.7, с одним новым переиспользуемым паттерном (`PersonNameListEditor`, первая повторяющаяся форма-редактор с несколькими текстовыми полями и датой на строку в программе) — подтверждено обоими отчётами живой проверки и обновлённой §3.7/§5 `entity-write.md`. `CHANGELOG.md` и `entity-write.md` после Задачи 2 описывают и бэкенд, и веб — пробел, отмеченный в «Предпосылке» (случай (a)), закрыт Шагом 2.4. После обеих задач — обзор всей ветки целиком (`git diff main...HEAD` по объёму подпроекта), при необходимости волна точечных фиксов по результатам ревью, затем слияние в `main` — как и в предыдущих семи подпроектах программы `entity-write`. Следующий подпроект — 9 (граф вокруг Person: `Relation`, `Residence`, `Event`+`EventParticipant`).

## Коммиты

Два коммита в `main`, по одному на задачу — см. Шаги 1.10, 2.6.
