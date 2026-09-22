# S15. Контракты записи делений

> План этапа S15 программы «ядро чтения и записи» (`docs/plans/2026-09-20-core-rw-roadmap.md`).
> Базовые доменные правила и запись делений выполнены в S1–S14; этот этап выводит
> запись в публичные интерфейсы: HTTP `/api` и MCP-тулы, прямо на делениях.
> Исполнять шагами TDD; коммиты в `main` малыми.

## Goal

Связать сценарии записи делений (S14) с публичными интерфейсами:
`internal/transport` — DTO запросов и тело ошибки 409;
`internal/httpapi` — `GET/POST/PUT/DELETE /api/admin-divisions[/{id}]`;
`internal/mcp` — тулы `division_get|create|update|delete`;
`internal/app` — прокидка `idgen` + четырёх сценариев через фасад `divisionsService`.
Приёмка из roadmap: 404/409/422 на ошибки, тело 409 со списком ссылающихся,
ошибки MCP-тулов структурой результата; вертикальный срез делений закрыт полностью.

## Architecture

- `internal/transport` — единственный владелец JSON-контрактов:
  - `admin_division_write.go`: `AdminDivisionCreate` (тело `POST` и аргументы тула
    `division_create`), `AdminDivisionUpdate` (тело `PUT` и аргументы
    `division_update`); оба — `{name, type, parent_id}` с методом `Model()`
    (копия указателя `parent_id`; `ID` сценарию `create` выдаёт генератор).
  - `errors.go`: `InUseErrorBody{error, referrers []Ref}` + `Ref{type, id}` и
    конвертер `InUseErrorBodyFromModel(*models.InUseError)` — тело `409`.
- HTTP (стиль как у S13): объявить в `internal/httpapi/deps.go` интерфейс
  `DivisionService` из пяти методов (`ListDivisions`, `GetDivision`,
  `CreateDivision`, `UpdateDivision`, `DeleteDivision`); новые обработчики в
  `internal/httpapi/division_write.go`; расширить `writeError` (404/409/422),
  зарегистрировать пути `GET/POST /api/admin-divisions`,
  `GET/PUT/DELETE /api/admin-divisions/{id}`.
- MCP (стиль как у S13): тот же интерфейс в `internal/mcp/deps.go`; тулы
  `division_get`, `division_create`, `division_update`, `division_delete` —
  каждый решает "не смог" как результат с ошибкой (текст
  `не удалось <что>: <err>`), как у `division_list`; успехи получают
  `create`/`get`/`update` — JSON-строка контракта `AdminDivision`, `delete` —
  текст-подтверждение `деление "<id>" удалено`.
- `internal/app` — фасад `divisionService` (пять именованных полей по сценариям)
  с тонкими делегирующими методами и compile-time assertion на оба интерфейса;
  `idgen.New()` проходит в `create` при сборке.

## Tech Stack

- Go (модуль `github.com/amarin/genodex`, go 1.26.4); stdlib `net/http` (маршруты
  `{id}` из Go 1.22+), `encoding/json`; `internal/models` без JSON-тегов,
  контракты — только `internal/transport`.
- Тесты: stdlib + `net/http/httptest`; фейки-службы с записью аргументов;
  e2e на реальном store (`sqlstore.Open(t.TempDir())`) в `internal/httpapi`.
- mcp-go v0.58: `mcp.WithString(name, mcp.Required(), mcp.Description(...))`,
  `req.GetString(name, "")`, `mcp.NewToolResultText`, `mcp.NewToolResultError`,
  `AsTextContent` — по образцу S13.

## Spec

Источник истины — `docs/data-model/core-read-write.md` §2 (валидация), §4 (`ID`, `ErrNotFound`),
§5 (контракты делений). Ошибки сценариев S14 сохраняются без изменений:
`*models.ValidationError` (поля `id`/`parent_id`/`name`/`type`), `models.ErrNotFound`,
`*models.InUseError` (`models.MaxReferrers` = 20). Формат `id` делений — `AD-…`
(проверка `models.ID` в сценариях).

Коды HTTP:
- `200` — `GET` единицы, `PUT` (тело — слитая единица);
- `201` — `POST` (тело — созданная единица);
- `204` — `DELETE` (без тела);
- `400` — тело не разобрано как JSON (`{error}`);
- `404` — `models.ErrNotFound` (`{error}`);
- `409` — `*models.InUseError` (тело `InUseErrorBody`: `{error, referrers:[{type,id}…]}`);
- `422` — `*models.ValidationError` (`{error, field}`), в т.ч. неверный формат
  `id` в пути `/{id}` (не 404!);
- `500` — прочее (`{error}`).

MCP-тулы: аргументы через `GetString`; `parent_id` не передан/пустая строка →
`nil` (= корень, значение поля не меняется у `division_update` только если
передан явно — см. семантику ниже). Типы `type` — канонические значения
`models.AdminDivisionType`, точная строка (как в `validation.md`).

## Global Constraints

- Правки только в: `internal/transport`, `internal/httpapi`, `internal/mcp`,
  `internal/app`, `docs/plans`, сводные доки (`roadmap`, `usage.md`,
  `core-read-write.md` §5, `todo.md`). Сценарии, модели, хранилище, порт,
  sqlstore не трогаем.
- Дерево gofmt-чистое; рубеж каждого шага:
  `gofmt -l .` пусто, `go build ./...`, `go vet ./...`, `go test ./...`.
- `mockgen` в `PATH` не прописан — команды симуляции требуют
  `PATH="$PATH:$(go env GOPATH)/bin"`. В этом этапе новые интерфейсы-моки не
  нужны: тесты пишем на фейках вручную (как в S13/S14).
- Не коммитим рабочее дерево по мелочи: один коммит на логический шаг.

## Задача 1. `transport` (DTO и тело 409)

Файл `internal/transport/admin_division_write_test.go`:

```go
package transport

import (
	"encoding/json"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

func TestAdminDivisionCreateJSONAndModel(t *testing.T) {
	src := []byte(`{"name":"Давыдово","type":"selo","parent_id":"AD-root"}`)
	var got AdminDivisionCreate
	if err := json.Unmarshal(src, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Name != "Давыдово" || got.Type != models.AdminDivisionSelo {
		t.Fatalf("got %+v", got)
	}
	if got.ParentID == nil || *got.ParentID != "AD-root" {
		t.Fatalf("parent_id = %v", got.ParentID)
	}

	m := got.Model()
	if m.ID != "" {
		t.Fatalf("model id заведомо пуст: %q", m.ID)
	}
	if m.Name != "Давыдово" || m.Type != models.AdminDivisionSelo || m.ParentID == nil || *m.ParentID != "AD-root" {
		t.Fatalf("model = %+v", m)
	}
	if m.ParentID == got.ParentID {
		t.Fatal("model делит указатель parent_id с DTO")
	}
}

func TestAdminDivisionCreateRoot(t *testing.T) {
	src := []byte(`{"name":"Московская","type":"governorate"}`)
	var got AdminDivisionCreate
	if err := json.Unmarshal(src, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	m := got.Model()
	if m.ParentID != nil {
		t.Fatalf("parent_id должен быть nil, got %v", m.ParentID)
	}
}

func TestAdminDivisionUpdateModel(t *testing.T) {
	src := []byte(`{"name":"Давыдово","type":"selo"}`)
	var got AdminDivisionUpdate
	if err := json.Unmarshal(src, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	m := got.Model()
	if m.Name != "Давыдово" || m.Type != models.AdminDivisionSelo || m.ParentID != nil {
		t.Fatalf("model = %+v", m)
	}
	if m.ID != "" {
		t.Fatalf("id задаёт обработчик при слиянии, в Model() пусто: %q", m.ID)
	}
}

func TestInUseErrorBodyJSONContract(t *testing.T) {
	const idV = "AD-01ARZ3NDEKTSV4RRFFQ69G5FAV"
	const id0  = "AD-01ARZ3NDEKTSV4RRFFQ69G5FA0"
	e := &models.InUseError{
		Type: models.TypeAdministrativeDivision,
		ID:   models.ID(idV),
		Referrers: []models.EntityRef{
			{Type: models.TypeAdministrativeDivision, ID: models.ID(id0)},
		},
	}
	data, err := json.Marshal(InUseErrorBodyFromModel(e))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"error":"` + idV + ` используется: ` + id0 + `","referrers":[{"type":"administrative_division","id":"` + id0 + `"}]}`
	if string(data) != want {
		t.Fatalf("body = %s, want %s", data, want)
	}
}
```

_Примечание по `InUseError.Error()`_: сейчас текст — `administrative_division "…" используется: administrative_division …` (с кавычками id). Тест выше записан конкатенацией от id-констант; если при реальном прогоне текст отличается (формат `%q`/`%s` внутри `InUseError.Error()`), привести `want` к фактическому каноническому тексту, не меняя контракт поля `error` (он свободный), и зафиксировать решение. Таблица: `json.Marshal` экранирует кавычки из `Error()` в `\"`, что тесту до JSON-строки не мешает — проверяем уже сериализованное тело.

Шаг 1.1. Положим тест (файл выше). Прогон падает — `AdminDivisionCreate`/`AdminDivisionUpdate`/`InUseErrorBody` не определены.
Шаг 1.2. Реализация. `internal/transport/admin_division_write.go`:

```go
package transport

import "github.com/amarin/genodex/internal/models"

// AdminDivisionCreate — тело POST /api/admin-divisions и аргументы тула
// division_create. Идентификатор генерирует сценарий; parent_id = null у корня.
type AdminDivisionCreate struct {
	Name     string                   `json:"name"`
	Type     models.AdminDivisionType `json:"type"`
	ParentID *models.ID               `json:"parent_id"`
}

// Model возвращает доменную единицу с пустым ID.
func (d AdminDivisionCreate) Model() models.AdministrativeDivision {
	return models.AdministrativeDivision{
		Name:     d.Name,
		Type:     d.Type,
		ParentID: cloneParentID(d.ParentID),
	}
}

// AdminDivisionUpdate — тело PUT /api/admin-divisions/{id} и аргументы тула
// division_update: полная замена полей name/type/parent_id.
type AdminDivisionUpdate struct {
	Name     string                   `json:"name"`
	Type     models.AdminDivisionType `json:"type"`
	ParentID *models.ID               `json:"parent_id"`
}

func (d AdminDivisionUpdate) Model() models.AdministrativeDivision {
	return models.AdministrativeDivision{
		Name:     d.Name,
		Type:     d.Type,
		ParentID: cloneParentID(d.ParentID),
	}
}

func cloneParentID(p *models.ID) *models.ID {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}
```

`internal/transport/errors.go`:

```go
package transport

import "github.com/amarin/genodex/internal/models"

// Ref — ссылка на сущность в теле ответа 409: тип и идентификатор.
type Ref struct {
	Type models.Type `json:"type"`
	ID   models.ID   `json:"id"`
}

// InUseErrorBody — тело ответа 409 Conflict: текст ошибки и первые
// models.MaxReferrers ссылающихся сущностей в стабильном порядке без повторов.
type InUseErrorBody struct {
	Error     string `json:"error"`
	Referrers []Ref  `json:"referrers"`
}

// InUseErrorBodyFromModel конвертирует ошибку занятости в тело ответа.
func InUseErrorBodyFromModel(e *models.InUseError) InUseErrorBody {
	refs := make([]Ref, 0, len(e.Referrers))
	for _, r := range e.Referrers {
		refs = append(refs, Ref{Type: r.Type, ID: r.ID})
	}
	return InUseErrorBody{Error: e.Error(), Referrers: refs}
}
```

Шаг 1.3. `go test ./internal/transport/` зелёный; рубеж; коммит
`feat(transport): DTO записей делений и тело 409`.

## Задача 2. `internal/httpapi`

### Шаг 2.1. Интерфейс и фейк

`internal/httpapi/deps.go` — расширить:

```go
// DivisionService — сценарии административного деления, отдаваемые в HTTP.
type DivisionService interface {
	ListDivisions(ctx context.Context, q models.DivisionQuery) ([]models.AdministrativeDivision, error)
	GetDivision(ctx context.Context, id models.ID) (models.AdministrativeDivision, error)
	CreateDivision(ctx context.Context, d models.AdministrativeDivision) (models.AdministrativeDivision, error)
	UpdateDivision(ctx context.Context, d models.AdministrativeDivision) error
	DeleteDivision(ctx context.Context, id models.ID) error
}
```

`internal/httpapi/division_test.go` — в `fakeDivisions` добавить поля и четыре метода:

```go
type fakeDivisions struct {
	list []models.AdministrativeDivision
	err  error
	got  models.DivisionQuery
	call int

	getDiv      models.AdministrativeDivision
	created     models.AdministrativeDivision
	gotCreate   models.AdministrativeDivision
	updated     models.AdministrativeDivision
	gotIDs      []models.ID
	deleteErr   error
}

func (f *fakeDivisions) GetDivision(_ context.Context, id models.ID) (models.AdministrativeDivision, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.AdministrativeDivision{}, f.err
	}
	return f.getDiv, nil
}

func (f *fakeDivisions) CreateDivision(_ context.Context, d models.AdministrativeDivision) (models.AdministrativeDivision, error) {
	f.gotCreate = d
	if f.err != nil {
		return models.AdministrativeDivision{}, f.err
	}
	return f.created, nil
}

func (f *fakeDivisions) UpdateDivision(_ context.Context, d models.AdministrativeDivision) error {
	f.updated = d
	return f.err
}

func (f *fakeDivisions) DeleteDivision(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)
	return f.deleteErr
}
```

### Шаг 2.2. Тесты обработчиков

`internal/httpapi/division_write_test.go`. Помощники поверх `httptest`:

```go
func getD(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func postD(t *testing.T, h http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(body)))
	return rec
}

func putD(t *testing.T, h http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, path, strings.NewReader(body)))
	return rec
}

func delD(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, path, nil))
	return rec
}
```

Тесты:

```go
const validID = "AD-01ARZ3NDEKTSV4RRFFQ69G5FAV"

func TestDivisionGet(t *testing.T) {
	svc := &fakeDivisions{getDiv: models.AdministrativeDivision{ID: validID, Name: "Давыдово", Type: models.AdminDivisionSelo}}
	rec := getD(t, httpapi.NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions/"+validID)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}
	want := `{"id":"` + validID + `","name":"Давыдово","type":"selo","parent_id":null}`
	if rec.Body.String() != want {
		t.Fatalf("body = %s, want %s", rec.Body, want)
	}
	if got := svc.gotIDs[0]; got != validID {
		t.Fatalf("id = %s", got)
	}
}

func TestDivisionGetNotFoundIs404(t *testing.T) {
	svc := &fakeDivisions{err: models.ErrNotFound}
	rec := getD(t, httpapi.NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions/"+validID)
	requireStatus(t, rec, http.StatusNotFound)
	if rec.Body.String() != `{"error":"не найдено"}` {
		t.Fatalf("body = %s", rec.Body)
	}
}

func TestDivisionGetValidationErrorIs422(t *testing.T) {
	svc := &fakeDivisions{err: &models.ValidationError{Field: "id", Reason: "неверный формат"}}
	rec := getD(t, httpapi.NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions/ad-1")
	requireStatus(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"id"`) {
		t.Fatalf("body = %s", rec.Body)
	}
}

func TestDivisionCreate(t *testing.T) {
	svc := &fakeDivisions{created: models.AdministrativeDivision{ID: validID, Name: "Давыдово", Type: models.AdminDivisionSelo}}
	rec := postD(t, httpapi.NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions", `{"name":"Давыдово","type":"selo"}`)
	requireStatus(t, rec, http.StatusCreated)
	want := `{"id":"` + validID + `","name":"Давыдово","type":"selo","parent_id":null}`
	if rec.Body.String() != want {
		t.Fatalf("body = %s, want %s", rec.Body, want)
	}
	wantM := models.AdministrativeDivision{Name: "Давыдово", Type: models.AdminDivisionSelo}
	if svc.gotCreate.Name != wantM.Name || svc.gotCreate.Type != wantM.Type || svc.gotCreate.ParentID != nil || svc.gotCreate.ID != "" {
		t.Fatalf("create = %+v", svc.gotCreate)
	}
}

func TestDivisionCreateWithParentPassesModel(t *testing.T) {
	svc := &fakeDivisions{created: models.AdministrativeDivision{ID: validID, Name: "Давыдово", Type: models.AdminDivisionSelo}}
	const root = "AD-01ARZ3NDEKTSV4RRFFQ69G5FA0"
	body := `{"name":"Давыдово","type":"selo","parent_id":"` + root + `"}`
	rec := postD(t, httpapi.NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions", body)
	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.ParentID == nil || *svc.gotCreate.ParentID != root {
		t.Fatalf("parent_id = %v", svc.gotCreate.ParentID)
	}
}

func TestDivisionCreateBadJSONIs400(t *testing.T) {
	rec := postD(t, httpapi.NewHandler(&fakeDivisions{}, fstest.MapFS{}), "/api/admin-divisions", `{`)
	requireStatus(t, rec, http.StatusBadRequest)
	if !strings.Contains(rec.Body.String(), "не удалось разобрать тело") {
		t.Fatalf("body = %s", rec.Body)
	}
}

func TestDivisionCreateValidationErrorIs422(t *testing.T) {
	svc := &fakeDivisions{err: &models.ValidationError{Field: "name", Reason: "пустое значение"}}
	rec := postD(t, httpapi.NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions", `{"name":"","type":"selo"}`)
	requireStatus(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"name"`) {
		t.Fatalf("body = %s", rec.Body)
	}
}

func TestDivisionUpdateMergesFields(t *testing.T) {
	parent := models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA0")
	svc := &fakeDivisions{getDiv: models.AdministrativeDivision{
		ID: validID, Name: "Село", Type: models.AdminDivisionSelo,
		ParentID: &parent, Variants: []string{"Давыдова"},
	}}
	rec := putD(t, httpapi.NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions/"+validID, `{"name":"Давыдово","type":"selo","parent_id":null}`)
	requireStatus(t, rec, http.StatusOK)
	want := `{"id":"` + validID + `","name":"Давыдово","type":"selo","parent_id":null}`
	if rec.Body.String() != want {
		t.Fatalf("body = %s, want %s", rec.Body, want)
	}
	if svc.updated.Name != "Давыдово" || svc.updated.ID != validID {
		t.Fatalf("updated = %+v", svc.updated)
	}
	if svc.updated.ParentID != nil {
		t.Fatalf("parent_id должен стать nil, got %v", svc.updated.ParentID)
	}
	if len(svc.updated.Variants) != 1 || svc.updated.Variants[0] != "Давыдова" {
		t.Fatalf("прочие поля должны сохраняться: %+v", svc.updated)
	}
}

func TestDivisionUpdateNotFoundIs404(t *testing.T) {
	svc := &fakeDivisions{err: models.ErrNotFound}
	rec := putD(t, httpapi.NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions/"+validID, `{"name":"Давыдово","type":"selo"}`)
	requireStatus(t, rec, http.StatusNotFound)
}

func TestDivisionUpdateInvalidIDIs422(t *testing.T) {
	svc := &fakeDivisions{err: &models.ValidationError{Field: "id", Reason: "неверный формат"}}
	rec := putD(t, httpapi.NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions/ad-1", `{"name":"Давыдово","type":"selo"}`)
	requireStatus(t, rec, http.StatusUnprocessableEntity)
}

func TestDivisionDelete(t *testing.T) {
	svc := &fakeDivisions{}
	rec := delD(t, httpapi.NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions/"+validID)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("204 обязан быть без тела: %q", rec.Body)
	}
	if got := svc.gotIDs[0]; got != validID {
		t.Fatalf("id = %s", got)
	}
}

func TestDivisionDeleteNotFoundIs404(t *testing.T) {
	svc := &fakeDivisions{deleteErr: models.ErrNotFound}
	rec := delD(t, httpapi.NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions/"+validID)
	requireStatus(t, rec, http.StatusNotFound)
}

func TestDivisionDeleteInvalidIDIs422(t *testing.T) {
	svc := &fakeDivisions{deleteErr: &models.ValidationError{Field: "id", Reason: "неверный формат"}}
	rec := delD(t, httpapi.NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions/ad-1")
	requireStatus(t, rec, http.StatusUnprocessableEntity)
}

func TestDivisionDeleteInUseIs409WithReferrers(t *testing.T) {
	const ref = "AD-01ARZ3NDEKTSV4RRFFQ69G5FA0"
	svc := &fakeDivisions{deleteErr: &models.InUseError{
		Type: models.TypeAdministrativeDivision, ID: validID,
		Referrers: []models.EntityRef{{Type: models.TypeAdministrativeDivision, ID: ref}},
	}}
	rec := delD(t, httpapi.NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions/"+validID)
	requireStatus(t, rec, http.StatusConflict)
	want := `{"error":"` + validID + ` используется: ` + ref + `","referrers":[{"type":"administrative_division","id":"` + ref + `"}]}`
	if rec.Body.String() != want {
		t.Fatalf("body = %s, want %s", rec.Body, want)
	}
}

func requireStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}
}
```

_Договорённость_: у `InUseError.Error()` и `ErrNotFound.Error()` в тестах держим
канонический текст (`не найдено`; у 409 — формат `InUseError`). Формат текста
ошибки — деталь реализации модели, в контрактах важна структура тел.

Шаг 2.2 (продолжение): тесты падают — маршрутов нет (404 признак "не зарегистрирован"),
`writeError` не знает 404/409.

### Шаг 2.3. Реализация

`internal/httpapi/httpapi.go`:

```go
import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)
```

Создать отдельный файл `internal/httpapi/division_write.go`:

```go
package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleDivisionGet — GET /api/admin-divisions/{id}.
func handleDivisionGet(divisions DivisionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		d, err := divisions.GetDivision(r.Context(), pathDivisionID(r))
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, transport.AdminDivisionFromModel(d))
	}
}

// handleDivisionCreate — POST /api/admin-divisions: 201 с созданной единицей.
func handleDivisionCreate(divisions DivisionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in transport.AdminDivisionCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})
			return
		}
		created, err := divisions.CreateDivision(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, transport.AdminDivisionFromModel(created))
	}
}

// handleDivisionUpdate — PUT /api/admin-divisions/{id}. Полная замена
// name/type/parent_id: обработчик берёт текущую версию (сохраняя прочие поля),
// накладывает поля запроса и пишет сценарием update.
func handleDivisionUpdate(divisions DivisionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := pathDivisionID(r)

		var in transport.AdminDivisionUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})
			return
		}

		cur, err := divisions.GetDivision(r.Context(), id)
		if err != nil {
			writeError(w, err)
			return
		}
		cur.Name = in.Name
		cur.Type = in.Type
		cur.ParentID = cloneID(in.ParentID)

		if err := divisions.UpdateDivision(r.Context(), cur); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, transport.AdminDivisionFromModel(cur))
	}
}

// handleDivisionDelete — DELETE /api/admin-divisions/{id}: 204 без тела.
func handleDivisionDelete(divisions DivisionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := divisions.DeleteDivision(r.Context(), pathDivisionID(r)); err != nil {
			writeError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func pathDivisionID(r *http.Request) models.ID {
	return models.ID(strings.TrimPrefix(r.PathValue("id"), "/"))
}

func cloneID(p *models.ID) *models.ID {
	if p == nil {
		return nil
	}
	v := *p
	return &v
}

func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	return dec.Decode(v)
}
```

_Примечание по `json.Decoder`_: без `io.ReadAll`. Пустое тело → ошибка
`EOF`, обёрнутая как 400. Неизвестные поля в теле игнорируются (`Decode` без
`DisallowUnknownFields`) — прямой путь для будущей дополняемости контракта.

`writeError` (в `httpapi.go`) — добавить ветки:

```go
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

В `NewHandler` (в `httpapi.go`):

```go
mux.HandleFunc("GET /api/admin-divisions", handleDivisionList(divisions))
mux.HandleFunc("GET /api/admin-divisions/{id}", handleDivisionGet(divisions))
mux.HandleFunc("POST /api/admin-divisions", handleDivisionCreate(divisions))
mux.HandleFunc("PUT /api/admin-divisions/{id}", handleDivisionUpdate(divisions))
mux.HandleFunc("DELETE /api/admin-divisions/{id}", handleDivisionDelete(divisions))
mux.HandleFunc("GET /api/health", handleHealth)
```

Шаг 2.4. `go test ./internal/httpapi/` зелёный; рубеж; коммит
`feat(httpapi): запись делений — GET/{id}, POST, PUT, DELETE`.

## Задача 3. `internal/httpapi` — e2e на реальном store

`internal/httpapi/store_test.go` — по образцу S13, новая обёртка полного набора:

```go
type divisionService struct {
	list   *list_divisions.Scenario
	get    *get_division.Scenario
	create *create_division.Scenario
	update *update_division.Scenario
	del    *delete_division.Scenario
}

func (s *divisionService) ListDivisions(ctx context.Context, q models.DivisionQuery) ([]models.AdministrativeDivision, error) { return s.list.ListDivisions(ctx, q) }
func (s *divisionService) GetDivision(ctx context.Context, id models.ID) (models.AdministrativeDivision, error)              { return s.get.GetDivision(ctx, id) }
func (s *divisionService) CreateDivision(ctx context.Context, d models.AdministrativeDivision) (models.AdministrativeDivision, error) {
	return s.create.CreateDivision(ctx, d)
}
func (s *divisionService) UpdateDivision(ctx context.Context, d models.AdministrativeDivision) error { return s.update.UpdateDivision(ctx, d) }
func (s *divisionService) DeleteDivision(ctx context.Context, id models.ID) error                    { return s.del.DeleteDivision(ctx, id) }
```

Импорты: `context`, `encoding/json`, `fmt`, `net/http`, `net/http/httptest`, `strings`, `testing`, `testing/fstest`, `models`, `sqlstore`, `idgen`, `list_divisions`, `get_division`, `create_division`, `update_division`, `delete_division`, `httpapi`, `transport`.

```go
func TestDivisionWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	svc := &divisionService{
		list:   list_divisions.New(st),
		get:    get_division.New(st),
		create: create_division.New(st, idgen.New()),
		update: update_division.New(st),
		del:    delete_division.New(st),
	}
	h := httpapi.NewHandler(svc, fstest.MapFS{})

	// Создание корня.
	root := createD(t, h, `{"name":"Московская","type":"governorate"}`, http.StatusCreated)
	if root.Name != "Московская" || root.Type != models.AdminDivisionGovernorate || root.ParentID != nil {
		t.Fatalf("root = %+v", root)
	}
	if !strings.HasPrefix(string(root.ID), "AD-") {
		t.Fatalf("id = %q", root.ID)
	}

	// Создание дочерней.
	childBody := fmt.Sprintf(`{"name":"Давыдово","type":"selo","parent_id":%q}`, root.ID)
	child := createD(t, h, childBody, http.StatusCreated)
	if child.ParentID == nil || *child.ParentID != root.ID {
		t.Fatalf("child = %+v", child)
	}

	// Чтение.
	rec := getRaw(t, h, "/api/admin-divisions/"+string(child.ID))
	requireStatus(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"name":"Давыдово"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Обновление: полная замена name/type/parent_id.
	putBody := fmt.Sprintf(`{"name":"Давыдова","type":"selo","parent_id":%q}`, root.ID)
	rec = putRaw(t, h, "/api/admin-divisions/"+string(child.ID), putBody)
	requireStatus(t, rec, http.StatusOK)
	got := decodeD(t, rec)
	if got.Name != "Давыдова" || got.ID != child.ID {
		t.Fatalf("after update = %+v", got)
	}

	// Родитель занят дочерью → 409 со списком ссылающихся.
	rec = delRaw(t, h, "/api/admin-divisions/"+string(root.ID))
	requireStatus(t, rec, http.StatusConflict)
	if !strings.Contains(rec.Body.String(), `"referrers"`) || !strings.Contains(rec.Body.String(), string(child.ID)) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Удаление дочерней, затем корня.
	delRaw(t, h, "/api/admin-divisions/"+string(child.ID))
	requireStatus(t, delRaw(t, h, "/api/admin-divisions/"+string(root.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatus(t, getRaw(t, h, "/api/admin-divisions/"+string(root.ID)), http.StatusNotFound)

	// Неверный формат id в пути — 422 (не 404).
	requireStatus(t, getRaw(t, h, "/api/admin-divisions/obvious-bad"), http.StatusUnprocessableEntity)

	// Несуществующий родитель — 422 с полем parent_id.
	rec = postRaw(t, h, `/api/admin-divisions`)
	requireStatus(t, rec, http.StatusUnprocessableEntity)
}

func createD(t *testing.T, h http.Handler, body string, want int) transport.AdminDivision {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/admin-divisions", strings.NewReader(body)))
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}
	return decodeD(t, rec)
}

func decodeD(t *testing.T, rec *httptest.ResponseRecorder) transport.AdminDivision {
	t.Helper()
	var d transport.AdminDivision
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}
	return d
}

func getRaw(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func putRaw(t *testing.T, h http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPut, path, strings.NewReader(body)))
	return rec
}

func postRaw(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"name":"Давыдово","type":"selo","parent_id":"AD-01ARZ3NDEKTSV4RRFFQ69G5FA9"}`)))
	return rec
}

func delRaw(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, path, nil))
	return rec
}
```

Примечание: `postRaw` намеренно несуществующего родителя с валидным форматом id —
ожидается `422` с полем `parent_id`. Если сценарий S14 вернёт `*InUseError`/другое —
поправить ожидание по фактическому поведению и зафиксировать решение (см. «Решения этапа»).

Шаг 3.1. Фиксируем тест; падает на ещё не готовых `divisionService`/`createD` и семантике родителя.
Шаг 3.2. Реализация — обёртка `divisionService` и помощь выше (код в `store_test.go`).
Шаг 3.3. `go test ./internal/httpapi/` зелёный; рубеж; коммит
`feat(httpapi): e2e записи делений на реальном store`.

## Задача 4. `internal/mcp`

### Шаг 4.1. Интерфейс и фейк

`internal/mcp/deps.go` — интерфейс, как в httpapi (5 методов). `internal/mcp/division_test.go` —
расширить `fakeDivisions` методами по образцу httpapi-фейка (`getDiv`, `created`,
`gotCreate`, `updated`, `gotIDs`, `deleteErr`).

### Шаг 4.2. Тесты тулов

`internal/mcp/division_write_test.go`:

```go
package mcp_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/amarin/genodex/internal/mcp"
	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

var writeCtx = context.Background()

func callTool(t *testing.T, s *server.MCPServer, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{Params: mcp.CallToolRequestParams{Name: name, Arguments: args}}
	res, err := s.CallTool(writeCtx, req)
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	return res
}

func toolText(t *testing.T, res *mcp.CallToolResult) string {
	if res.IsError {
		t.Fatalf("не ждали ошибку: %+v", res)
	}
	if len(res.Content) == 0 {
		t.Fatal("пустой результат")
	}
	return string(res.Content[0].(mcp.TextContent).Text)
}

func wantToolError(t *testing.T, res *mcp.CallToolResult, substr string) {
	t.Helper()
	if !res.IsError {
		t.Fatalf("ждали ошибку с %q, got %+v", substr, res)
	}
	text := ""
	if len(res.Content) > 0 {
		text = string(res.Content[0].(mcp.TextContent).Text)
	}
	if substr != "" && !strings.Contains(text, substr) {
		t.Fatalf("текст = %q, want содержит %q", text, substr)
	}
}
```

Внимательно с импортами: пакет тестов `mcp_test`, мок-сервер делаем через
`mcp.NewServer(...)` из пакета — как в существующем `division_test.go`. Внутри
`mcp_test` функции-обработчики недоступны; тулы вызываются через `CallTool`
реального сервера. Существующий тест `TestNewServerRegistersDivisionListOnly`
переименовываем в `TestNewServerRegistersDivisionTools` и проверяем все пять имён.

Регистрация — `registerDivisionTools` (в `internal/mcp/division.go`):

```go
func registerDivisionTools(s *server.MCPServer, divisions DivisionService) {
	s.AddTool(mcp.NewTool("division_list", mcp.WithDescription("Единицы административного деления"),
		mcp.WithString("parent_id", mcp.Description("Фильтр по родителю; пусто — все")),
		mcp.WithString("type", mcp.Description("Фильтр по типу")),
		mcp.WithString("kind", mcp.Description("Фильтр по виду")),
		mcp.WithString("offset", mcp.Description("Смещение")),
		mcp.WithString("limit", mcp.Description("Лимит")),
	), divisionListHandler(divisions))
	s.AddTool(mcp.NewTool("division_get",
		mcp.WithDescription("Единица административного деления по идентификатору"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Идентификатор (AD-…)")),
	), divisionGetHandler(divisions))
	s.AddTool(mcp.NewTool("division_create",
		mcp.WithDescription("Создание единицы административного деления; идентификатор генерируется"),
		mcp.WithString("name", mcp.Required(), mcp.Description("Название")),
		mcp.WithString("type", mcp.Required(), mcp.Description("Точный тип: governorate, district, volost, gorod, selo, derevnya, hutor, pogost, stanitsa, mestechko, other")),
		mcp.WithString("parent_id", mcp.Description("Родитель; пусто/нет — корень")),
	), divisionCreateHandler(divisions))
	s.AddTool(mcp.NewTool("division_update",
		mcp.WithDescription("Изменение полей name/type/parent_id единицы деления"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Идентификатор (AD-…)")),
		mcp.WithString("name", mcp.Required(), mcp.Description("Название")),
		mcp.WithString("type", mcp.Required(), mcp.Description("Точный тип")),
		mcp.WithString("parent_id", mcp.Description("Родитель; пусто/нет — корень")),
	), divisionUpdateHandler(divisions))
	s.AddTool(mcp.NewTool("division_delete",
		mcp.WithDescription("Удаление единицы административного деления"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Идентификатор (AD-…)")),
	), divisionDeleteHandler(divisions))
}
```

В `internal/mcp/division.go` добавить обработчики (по стилю `division_list`: ошибка —
`mcp.NewToolResultError("не удалось …: "+err.Error())`):

```go
func divisionGetHandler(divisions DivisionService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))
		d, err := divisions.GetDivision(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить деление: %v", err)), nil
		}
		return toolJSONResult(transport.AdminDivisionFromModel(d))
	}
}
```

```go
func divisionCreateHandler(divisions DivisionService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		d := transport.AdminDivisionCreate{
			Name:     req.GetString("name", ""),
			Type:     models.AdminDivisionType(req.GetString("type", "")),
			ParentID: optionalParentID(req),
		}.Model()
		created, err := divisions.CreateDivision(ctx, d)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать деление: %v", err)), nil
		}
		return toolJSONResult(transport.AdminDivisionFromModel(created))
	}
}
```

```go
func divisionUpdateHandler(divisions DivisionService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))
		cur, err := divisions.GetDivision(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущее деление: %v", err)), nil
		}
		cur.Name = req.GetString("name", "")
		cur.Type = models.AdminDivisionType(req.GetString("type", ""))
		cur.ParentID = optionalParentID(req)
		if err := divisions.UpdateDivision(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось обновить деление: %v", err)), nil
		}
		return toolJSONResult(transport.AdminDivisionFromModel(cur))
	}
}
```

```go
func divisionDeleteHandler(divisions DivisionService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))
		if err := divisions.DeleteDivision(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить деление: %v", err)), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf("деление %q удалено", id)), nil
	}
}
```

```go
// toolJSONResult — JSON-строка контракта как результат тула (как у division_list).
func toolJSONResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("сериализация: %w", err)
	}
	return mcp.NewToolResultText(string(data)), nil
}

// optionalParentID: отсутствие/пустая строка/null → корень (nil).
func optionalParentID(req mcp.CallToolRequest) *models.ID {
	raw, ok := req.GetArguments()["parent_id"]
	if !ok || raw == nil {
		return nil
	}
	s, isString := raw.(string)
	if !isString || s == "" {
		return nil
	}
	id := models.ID(s)
	return &id
}
```

Тесты (файл `division_write_test.go`):

```go
const toolID = "AD-01ARZ3NDEKTSV4RRFFQ69G5FAV"

func TestDivisionGetTool(t *testing.T) {
	svc := &fakeDivisions{getDiv: models.AdministrativeDivision{ID: toolID, Name: "Давыдово", Type: models.AdminDivisionSelo}}
	s := mcp.NewServer(svc)
	res := callTool(t, s, "division_get", map[string]any{"id": toolID})
	text := toolText(t, res)
	want := `{"id":"` + toolID + `","name":"Давыдово","type":"selo","parent_id":null}`
	if text != want {
		t.Fatalf("text = %s, want %s", text, want)
	}
}

func TestDivisionGetToolNotFoundIsError(t *testing.T) {
	svc := &fakeDivisions{err: models.ErrNotFound}
	res := callTool(t, mcp.NewServer(svc), "division_get", map[string]any{"id": toolID})
	wantToolError(t, res, "не удалось получить")
}

func TestDivisionGetToolInvalidIDIsError(t *testing.T) {
	svc := &fakeDivisions{err: &models.ValidationError{Field: "id", Reason: "неверный формат"}}
	res := callTool(t, mcp.NewServer(svc), "division_get", map[string]any{"id": "ad-1"})
	wantToolError(t, res, "id")
}

func TestDivisionCreateTool(t *testing.T) {
	svc := &fakeDivisions{created: models.AdministrativeDivision{ID: toolID, Name: "Давыдово", Type: models.AdminDivisionSelo}}
	res := callTool(t, mcp.NewServer(svc), "division_create", map[string]any{"name": "Давыдово", "type": "selo"})
	text := toolText(t, res)
	if text != `{"id":"`+toolID+`","name":"Давыдово","type":"selo","parent_id":null}` {
		t.Fatalf("text = %s", text)
	}
	wantM := models.AdministrativeDivision{Name: "Давыдово", Type: models.AdminDivisionSelo}
	if svc.gotCreate.Name != wantM.Name || svc.gotCreate.Type != wantM.Type || svc.gotCreate.ParentID != nil {
		t.Fatalf("create = %+v", svc.gotCreate)
	}
}

func TestDivisionCreateToolWithParent(t *testing.T) {
	svc := &fakeDivisions{created: models.AdministrativeDivision{ID: toolID, Name: "Давыдово", Type: models.AdminDivisionSelo}}
	callTool(t, mcp.NewServer(svc), "division_create", map[string]any{"name": "Давыдово", "type": "selo", "parent_id": toolID})
	if svc.gotCreate.ParentID == nil || *svc.gotCreate.ParentID != toolID {
		t.Fatalf("parent_id = %v", svc.gotCreate.ParentID)
	}
}

func TestDivisionCreateToolEmptyParentMeansRoot(t *testing.T) {
	svc := &fakeDivisions{created: models.AdministrativeDivision{ID: toolID, Name: "Давыдово", Type: models.AdminDivisionSelo}}
	callTool(t, mcp.NewServer(svc), "division_create", map[string]any{"name": "Давыдово", "type": "selo", "parent_id": ""})
	if svc.gotCreate.ParentID != nil {
		t.Fatalf("parent_id = %v", svc.gotCreate.ParentID)
	}
}

func TestDivisionCreateToolValidationErrorIsToolError(t *testing.T) {
	svc := &fakeDivisions{err: &models.ValidationError{Field: "name", Reason: "пустое значение"}}
	res := callTool(t, mcp.NewServer(svc), "division_create", map[string]any{"name": "", "type": "selo"})
	wantToolError(t, res, "не удалось создать")
}

func TestDivisionUpdateToolMergesFields(t *testing.T) {
	parent := models.ID(toolID)
	svc := &fakeDivisions{getDiv: models.AdministrativeDivision{
		ID: toolID, Name: "Село", Type: models.AdminDivisionSelo, ParentID: &parent, Variants: []string{"Давыдова"},
	}}
	res := callTool(t, mcp.NewServer(svc), "division_update", map[string]any{"id": toolID, "name": "Давыдово", "type": "selo"})
	text := toolText(t, res)
	if text != `{"id":"`+toolID+`","name":"Давыдово","type":"selo","parent_id":null}` {
		t.Fatalf("text = %s", text)
	}
	if svc.updated.Name != "Давыдово" || len(svc.updated.Variants) != 1 {
		t.Fatalf("updated = %+v", svc.updated)
	}
	if svc.updated.ParentID != nil {
		t.Fatalf("parent_id должен стать nil, got %v", svc.updated.ParentID)
	}
}

func TestDivisionUpdateToolNotFoundIsError(t *testing.T) {
	svc := &fakeDivisions{err: models.ErrNotFound}
	res := callTool(t, mcp.NewServer(svc), "division_update", map[string]any{"id": toolID, "name": "Давыдово", "type": "selo"})
	wantToolError(t, res, "не удалось получить текущее")
}

func TestDivisionDeleteTool(t *testing.T) {
	svc := &fakeDivisions{}
	res := callTool(t, mcp.NewServer(svc), "division_delete", map[string]any{"id": toolID})
	if res.IsError {
		t.Fatalf("res = %+v", res)
	}
	if !strings.Contains(toolText(t, res), "удалено") {
		t.Fatalf("text = %q", toolText(t, res))
	}
	if got := svc.gotIDs[0]; got != toolID {
		t.Fatalf("id = %s", got)
	}
}

func TestDivisionDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeDivisions{deleteErr: &models.InUseError{Type: models.TypeAdministrativeDivision, ID: toolID}}
	res := callTool(t, mcp.NewServer(svc), "division_delete", map[string]any{"id": toolID})
	wantToolError(t, res, "не удалось удалить")
}

func TestNewServerRegistersDivisionWriteTools(t *testing.T) {
	s := mcp.NewServer(&fakeDivisions{})
	for _, name := range []string{"division_get", "division_create", "division_update", "division_delete"} {
		if s.GetTool(name) == nil {
			t.Fatalf("тул %s не зарегистрирован", name)
		}
	}
}
```

_Примечание по `toolText`/`wantToolError`_: сравнение строк как в S13 —
через `AsTextContent`/приведение `res.Content[0]`, не зависящее от типа контента;
если в рантайме рантайм-вид другой (структура с полями) — привести к тому же
формату извлечения текста, что в `division_test.go` (единый хелпер в `mcp_test`).

Шаг 4.3. Реализация: `deps.go`, регистрация, обработчики, `optionalParentID`, `toolJSONResult`.
Шаг 4.4. `go test ./internal/mcp/` зелёный; рубеж; коммит
`feat(mcp): тулы записи делений division_get|create|update|delete`.

## Задача 5. `internal/app` — прокидка

`internal/app/app.go` (стиль S13: фасад + compile-time assertions + сборка):

```go
import (
	// ...
	"github.com/amarin/genodex/internal/idgen"
	"github.com/amarin/genodex/internal/models"
	create_division "github.com/amarin/genodex/internal/usecases/create_division"
	delete_division "github.com/amarin/genodex/internal/usecases/delete_division"
	get_division "github.com/amarin/genodex/internal/usecases/get_division"
	list_divisions "github.com/amarin/genodex/internal/usecases/list_divisions"
	update_division "github.com/amarin/genodex/internal/usecases/update_division"
)

// divisionService — фасад всех сценариев делений, отдаваемых HTTP и MCP.
type divisionService struct {
	list   *list_divisions.Scenario
	get    *get_division.Scenario
	create *create_division.Scenario
	update *update_division.Scenario
	del    *delete_division.Scenario
}

func (s *divisionService) ListDivisions(ctx context.Context, q models.DivisionQuery) ([]models.AdministrativeDivision, error) {
	return s.list.ListDivisions(ctx, q)
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

var (
	_ httpapi.DivisionService = (*divisionService)(nil)
	_ mcp.DivisionService     = (*divisionService)(nil)
)
```

В `New`:

```go
divisions := &divisionService{
	list:   list_divisions.New(st),
	get:    get_division.New(st),
	create: create_division.New(st, idgen.New()),
	update: update_division.New(st),
	del:    delete_division.New(st),
}
```

и в местах использования сейчас `list_divisions.New(st)` (там, где `mcp.NewServer`
и `httpapi.NewHandler`) — заменить на `divisions`.

Шаг 5.1. Реализация; рубеж `gofmt`/`build`/`vet`/`test`; smoke:
`go run ./cmd/genodex -p 9016`, затем:
- `curl /api/health` → `{"status":"ok"}`;
- `curl -s -X POST /api/admin-divisions -d '{"name":"Московская","type":"governorate"}'` → 201 с `id`;
- `curl /api/admin-divisions` — созданная присутствует;
- `curl -s -X GET /api/admin-divisions/<id>` → 200;
- `curl -s -X PUT /api/admin-divisions/<id> -d '{"name":"Давыдово","type":"selo"}'` → 200;
- `curl -s -X DELETE /api/admin-divisions/<id>` → 204;
- MCP-вызов через `go run ./cmd/genodex mcp 2>&1 | grep` не нужен — поменялись только
  регистрации тулов, проверяется `go test ./internal/mcp/`.
Шаг 5.2. Коммит `feat(app): фасад divisionsService и подключение сценариев записи`.

## Задача 6. Документы и рубеж

- `docs/plans/2026-09-20-core-rw-roadmap.md` — в раздел S15 добавить «Решения этапа» (см. ниже)
  + ссылку на этот план.
- `docs/usage.md` — в таблицу эндпоинтов строки `GET/POST/PUT/DELETE /api/admin-divisions[/{id}]`;
  в таблицу MCP-тулов `division_get|create|update|delete`; новый раздел «Запись единиц
  делений» (примеры `curl`, коды 400/404/409/422, тело 409).
- `docs/data-model/core-read-write.md` §5 — вставить bullet «Контракты записи делений (S15)» после
  bullet про ошибки S14 (строки 268–273).
- `docs/todo.md` — в «Следующий проход»: «Выполнены S1–S14 … осталось S15–S16» поменять на
  «Выполнены S1–S15 … остаток: поиск в контрактах (S16)»; в «Открытые вопросы» снять
  пункт про форму ошибки валидации (первая ошибка — решено на S15) либо пометить.
- Шаг 6.1. Правки доков; рубеж; smoke по полному набору (`/api/health`, `/api/admin-divisions`,
  `/`, write-сценарий). Коммит `docs: S15 — контракты записи делений`.

### Решения этапа (для roadmap и history)

1. Тело 409 — `{error, referrers:[{type,id}…]}`. **Total/Truncated не вводим**:
   публичная приёмка требует только список ссылающихся (до 20); «20 из 500» неотличимо
   от «ровно 20» — это принятое ограничение, снимать при горизонтальном срезе людей/событий
   (там понадобится `total`/`has_more`).
2. `GET/PUT/DELETE /api/admin-divisions/{id}`: неверный формат `id` в пути — 422 (не 404),
   через `*ValidationError` сценария; 404 только для отсутствующей единицы с корректным id.
3. PUT — полная замена `name/type/parent_id`, прочие поля модели сохраняются: обработчик
   читает текущую версию (`get_division`), накладывает поля запроса, пишет `update_division`;
   ответ 200 — слитая версия. Тело родителя: отсутствие поля/`null` → корень сценарием
   (`validID`). Опасность стирания колонок сторонних полей через «голую» DTO-модель исключена.
4. POST → 201 + созданная единица; DELETE удачный → 204 без тела; PUT/GET → 200.
5. MCP: ошибки — результат тула с признаком ошибки (`IsError`) и текстом
   `не удалось …: <err>`; успехи `get/create/update` — JSON-строка контракта
   `AdminDivision`; `delete` — текст `деление "<id>" удалено`.
6. `DivisionService` (httpapi и mcp) — единый интерфейс пяти методов; фасад
   `app.divisionService` собирает пять сценариев; `idgen.New()` подключается в `app`.
7. Итог S15: деления полностью закрыты по вертикали CRUD (S13 + S14 + S15) — образец для
   людей/событий.
8. (если во время e2e окажется иная семантика ошибки несуществующего родителя) —
   зафиксировать фактическое поведение сценария и поправить ожидание теста/доки, не меняя
   контракт сценария.

## Коммиты

1. `docs(plans): план этапа S15 — контракты записи делений`
2. `feat(transport): DTO записей делений и тело 409`
3. `feat(httpapi): запись делений — GET/{id}, POST, PUT, DELETE`
4. `feat(httpapi): e2e записи делений на реальном store`
5. `feat(mcp): тулы записи делений division_get|create|update|delete`
6. `feat(app): фасад divisionsService и подключение сценариев записи`
7. `docs: S15 — контракты записи делений`