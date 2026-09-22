# S14: Сценарии записи делений — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Четыре сценария над `AdministrativeDivision` — `get_division`, `delete_division`, `create_division`, `update_division` — с генерацией `ID` (интерфейс `IDGenerator`, реализация — существующий `internal/idgen`), валидацией, проверкой родителя и циклов, записью в `InTx`. Контракты (MCP/HTTP) — этап S15, `internal/app` в S14 не меняется.

**Architecture:** По пакету на сценарий (`internal/usecases/<scenario>/`), узкий `deps.go` с mockgen-директивой. `create`/`update` зависят от `DivisionStore { InTx(ctx, fn func(store.Store) error) }`: проверки существования родителя/циклов и `Save` идут внутри одной транзакции на переданном `store.Store` (импорт порта сценарием допустим: `usecases → store`). `get`/`delete` зависят от узкого `DivisionRepo` (один метод порта). Ошибки: неверный формат `id` аргумента, несуществующий родитель и цикл по `parent_id` — `*models.ValidationError` (поля `id`/`parent_id`; в S15 → 422); отсутствие самой единицы — `models.ErrNotFound` (404); занятость при удалении — `*models.InUseError` (409). `create` отвергает непустой входной `ID` (`*ValidationError` по полю `id`): явный ID допустим только для импорта (§2.2 спеки).

**Tech Stack:** Go 1.26.4, `go.uber.org/mock` v0.6.0 (mockgen в PATH), стандартный `testing`.

**Spec:** `docs/data-model/core-read-write.md` §2.2 (ID), §2.3 (инварианты, проверки с хранилищем — на сценариях), §5 (сценарии); `docs/plans/2026-09-20-core-rw-roadmap.md` (этап S14).

## Global Constraints

- Сценарии принимают и возвращают только `models.<Type>` (плюс `ctx` на входе, `error` на выходе); DTO не появляются.
- Зависимости — интерфейсы в `deps.go` пакета сценария с директивой `//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}`; моки перегенерировать (`mockgen` установлен в `$(go env GOPATH)/bin`).
- В тестах сценариев — рукописные фейки (как `fakeRepo` в `list_divisions`); сгенерированный `deps_test.go` остаётся для соответствия конвенции.
- Ошибки сценариев: неверный формат `id`-аргумента → `*models.ValidationError{Entity: TypeAdministrativeDivision, Field: "id"}`, хранилище не вызывается; несуществующий родитель или цикл по `parent_id` → `*models.ValidationError{Field: "parent_id"}`; несуществующая единица (get/update/delete) → `models.ErrNotFound` как есть; `*models.InUseError` пробрасывается как есть; прочие ошибки хранилища — как есть.
- `create`: непустой входной `ID` → `*ValidationError` по полю `id`; `ID` генерируется до `Validate()`; `Validate()` до `InTx` (хранилище не трогается при невалидной сущности).
- `update`: `Validate()` до `InTx`; внутри `InTx` — существование единицы (`Get`), проверка цепочки родителей (цикл/существование), `Save`.
- Порт `store.Store`, схема БД, `models.AdministrativeDivision` и её `Validate` не меняются. `internal/app` не меняется (подключение — S15).
- Комментарии и тексты ошибок — на русском. Рубеж после каждой задачи: `gofmt -l .` пусто, `go build ./...`, `go vet ./...`, `go test ./...`.

---

### Task 1: Сценарий `get_division`

**Files:**
- Create: `internal/usecases/get_division/deps.go`
- Create: `internal/usecases/get_division/deps_test.go` (генерируется mockgen)
- Create: `internal/usecases/get_division/scenario.go`
- Create: `internal/usecases/get_division/scenario_test.go`

**Interfaces:**
- Consumes: `store.Store.GetAdministrativeDivision(ctx, id) (*models.AdministrativeDivision, error)` (сигнатура повторяется в узком `DivisionRepo`); `models.ID.Validate(models.Type) error`; `models.ErrNotFound`; `models.ValidationError`.
- Produces: `get_division.New(repo DivisionRepo) *Scenario`; `(*Scenario).GetDivision(ctx context.Context, id models.ID) (models.AdministrativeDivision, error)`. S15 будет звать `GetDivision` из обработчиков `GET /api/admin-divisions/{id}` и тула `division_get`.

- [ ] **Step 1: Написать `deps.go`**

```go
package get_division

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// DivisionRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type DivisionRepo interface {
	GetAdministrativeDivision(ctx context.Context, id models.ID) (*models.AdministrativeDivision, error)
}
```

- [ ] **Step 2: Сгенерировать мок**

Run: `go generate ./internal/usecases/get_division/...`
Expected: появился `internal/usecases/get_division/deps_test.go` с `MockDivisionRepo`.

- [ ] **Step 3: Написать падающие тесты**

`internal/usecases/get_division/scenario_test.go`:

```go
package get_division

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type ctxKey struct{}

// adID возвращает корректный идентификатор деления, отличающийся последним символом.
func adID(last byte) models.ID {
	return models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// fakeRepo отдаёт заранее заданную единицу или ошибку и запоминает вызовы.
type fakeRepo struct {
	division *models.AdministrativeDivision
	err      error
	gotCtx   context.Context
	gotIDs   []models.ID
}

func (f *fakeRepo) GetAdministrativeDivision(ctx context.Context, id models.ID) (*models.AdministrativeDivision, error) {
	f.gotCtx = ctx
	f.gotIDs = append(f.gotIDs, id)

	if f.err != nil {
		return nil, f.err
	}

	return f.division, nil
}

func TestGetDivisionReturnsEntity(t *testing.T) {
	want := &models.AdministrativeDivision{ID: adID('V'), Name: "Село", Type: models.AdminDivisionSelo}
	repo := &fakeRepo{division: want}

	got, err := New(repo).GetDivision(context.Background(), adID('V'))
	if err != nil {
		t.Fatalf("GetDivision: %v", err)
	}

	if got.ID != want.ID || got.Name != want.Name || got.Type != want.Type {
		t.Fatalf("got %+v, ожидалась %+v", got, *want)
	}

	if len(repo.gotIDs) != 1 || repo.gotIDs[0] != adID('V') {
		t.Fatalf("репозиторий вызван с %v, ожидался один вызов с %v", repo.gotIDs, adID('V'))
	}
}

func TestGetDivisionNotFound(t *testing.T) {
	repo := &fakeRepo{err: models.ErrNotFound}

	if _, err := New(repo).GetDivision(context.Background(), adID('V')); !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}
}

func TestGetDivisionPropagatesRepoError(t *testing.T) {
	wantErr := errors.New("repo down")
	repo := &fakeRepo{err: wantErr}

	if _, err := New(repo).GetDivision(context.Background(), adID('V')); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

// TestGetDivisionInvalidID: неверный формат id — *ValidationError по полю id,
// репозиторий не вызывается.
func TestGetDivisionInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	for _, id := range []models.ID{"", "ad-1", "I-01ARZ3NDEKTSV4RRFFQ69G5FAV", "AD-короткий"} {
		_, err := New(repo).GetDivision(context.Background(), id)

		var ve *models.ValidationError
		if !errors.As(err, &ve) || ve.Field != "id" || ve.Entity != models.TypeAdministrativeDivision {
			t.Errorf("id %q: err = %v, ожидалась *ValidationError по полю id", id, err)
		}
	}

	if len(repo.gotIDs) != 0 {
		t.Fatalf("репозиторий вызван %d раз при неверном id", len(repo.gotIDs))
	}
}

func TestGetDivisionPassesContext(t *testing.T) {
	repo := &fakeRepo{division: &models.AdministrativeDivision{ID: adID('V'), Name: "Село", Type: models.AdminDivisionSelo}}
	ctx := context.WithValue(context.Background(), ctxKey{}, "marker")

	if _, err := New(repo).GetDivision(ctx, adID('V')); err != nil {
		t.Fatalf("GetDivision: %v", err)
	}

	if repo.gotCtx == nil || repo.gotCtx.Value(ctxKey{}) != "marker" {
		t.Fatalf("репозиторий получил контекст %v, ожидался переданный сценарию", repo.gotCtx)
	}
}
```

- [ ] **Step 4: Убедиться, что тесты падают**

Run: `go test ./internal/usecases/get_division/...`
Expected: FAIL — `undefined: New`.

- [ ] **Step 5: Реализация**

`internal/usecases/get_division/scenario.go`:

```go
package get_division

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «единица административного деления по идентификатору».
type Scenario struct {
	divisions DivisionRepo
}

// New создаёт сценарий.
func New(divisions DivisionRepo) *Scenario {
	return &Scenario{divisions: divisions}
}

// GetDivision возвращает единицу деления по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой единицы — models.ErrNotFound.
func (s *Scenario) GetDivision(ctx context.Context, id models.ID) (models.AdministrativeDivision, error) {
	if err := validateID(id); err != nil {
		return models.AdministrativeDivision{}, err
	}

	d, err := s.divisions.GetAdministrativeDivision(ctx, id)
	if err != nil {
		return models.AdministrativeDivision{}, err
	}

	return *d, nil
}

// validateID проверяет формат идентификатора деления; ошибка —
// *models.ValidationError по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeAdministrativeDivision)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeAdministrativeDivision,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

- [ ] **Step 6: Тесты проходят**

Run: `go test ./internal/usecases/get_division/...`
Expected: PASS.

- [ ] **Step 7: Рубеж и commit**

Run: `gofmt -l . && go build ./... && go vet ./... && go test ./...`
Expected: gofmt пусто, всё зелёное.

```bash
git add internal/usecases/get_division/
git commit -m "feat(usecases): сценарий get_division — деление по идентификатору"
```

---

### Task 2: Сценарий `delete_division`

**Files:**
- Create: `internal/usecases/delete_division/deps.go`
- Create: `internal/usecases/delete_division/deps_test.go` (генерируется mockgen)
- Create: `internal/usecases/delete_division/scenario.go`
- Create: `internal/usecases/delete_division/scenario_test.go`

**Interfaces:**
- Consumes: `store.Store.DeleteAdministrativeDivision(ctx, id) error`; `models.ErrNotFound`, `*models.InUseError`, `models.EntityRef`.
- Produces: `delete_division.New(repo DivisionRepo) *Scenario`; `(*Scenario).DeleteDivision(ctx context.Context, id models.ID) error`. S15 зовёт из `DELETE /api/admin-divisions/{id}` и тула `division_delete`.

- [ ] **Step 1: Написать `deps.go`**

```go
package delete_division

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// DivisionRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type DivisionRepo interface {
	DeleteAdministrativeDivision(ctx context.Context, id models.ID) error
}
```

- [ ] **Step 2: Сгенерировать мок**

Run: `go generate ./internal/usecases/delete_division/...`
Expected: появился `deps_test.go` с `MockDivisionRepo`.

- [ ] **Step 3: Написать падающие тесты**

`internal/usecases/delete_division/scenario_test.go`:

```go
package delete_division

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type ctxKey struct{}

// adID возвращает корректный идентификатор деления, отличающийся последним символом.
func adID(last byte) models.ID {
	return models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// fakeRepo возвращает заданную ошибку удаления и запоминает вызовы.
type fakeRepo struct {
	err    error
	gotCtx context.Context
	gotIDs []models.ID
}

func (f *fakeRepo) DeleteAdministrativeDivision(ctx context.Context, id models.ID) error {
	f.gotCtx = ctx
	f.gotIDs = append(f.gotIDs, id)

	return f.err
}

func TestDeleteDivisionDeletes(t *testing.T) {
	repo := &fakeRepo{}

	if err := New(repo).DeleteDivision(context.Background(), adID('V')); err != nil {
		t.Fatalf("DeleteDivision: %v", err)
	}

	if len(repo.gotIDs) != 1 || repo.gotIDs[0] != adID('V') {
		t.Fatalf("репозиторий вызван с %v, ожидался один вызов с %v", repo.gotIDs, adID('V'))
	}
}

func TestDeleteDivisionNotFound(t *testing.T) {
	repo := &fakeRepo{err: models.ErrNotFound}

	if err := New(repo).DeleteDivision(context.Background(), adID('V')); !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}
}

// TestDeleteDivisionInUse: *InUseError пробрасывается как есть — S15 превратит
// его в 409 со списком ссылающихся.
func TestDeleteDivisionInUse(t *testing.T) {
	inUse := &models.InUseError{
		Type: models.TypeAdministrativeDivision,
		ID:   adID('V'),
		Referrers: []models.EntityRef{
			{Type: models.TypeAdministrativeDivision, ID: adID('0')},
		},
	}
	repo := &fakeRepo{err: inUse}

	err := New(repo).DeleteDivision(context.Background(), adID('V'))

	var got *models.InUseError
	if !errors.As(err, &got) || got != inUse {
		t.Fatalf("err = %v, ожидался исходный *InUseError", err)
	}
}

// TestDeleteDivisionInvalidID: неверный формат id — *ValidationError по полю id,
// репозиторий не вызывается.
func TestDeleteDivisionInvalidID(t *testing.T) {
	repo := &fakeRepo{}

	for _, id := range []models.ID{"", "ad-1", "I-01ARZ3NDEKTSV4RRFFQ69G5FAV"} {
		err := New(repo).DeleteDivision(context.Background(), id)

		var ve *models.ValidationError
		if !errors.As(err, &ve) || ve.Field != "id" || ve.Entity != models.TypeAdministrativeDivision {
			t.Errorf("id %q: err = %v, ожидалась *ValidationError по полю id", id, err)
		}
	}

	if len(repo.gotIDs) != 0 {
		t.Fatalf("репозиторий вызван %d раз при неверном id", len(repo.gotIDs))
	}
}

func TestDeleteDivisionPassesContext(t *testing.T) {
	repo := &fakeRepo{}
	ctx := context.WithValue(context.Background(), ctxKey{}, "marker")

	if err := New(repo).DeleteDivision(ctx, adID('V')); err != nil {
		t.Fatalf("DeleteDivision: %v", err)
	}

	if repo.gotCtx == nil || repo.gotCtx.Value(ctxKey{}) != "marker" {
		t.Fatalf("репозиторий получил контекст %v, ожидался переданный сценарию", repo.gotCtx)
	}
}
```

- [ ] **Step 4: Убедиться, что тесты падают**

Run: `go test ./internal/usecases/delete_division/...`
Expected: FAIL — `undefined: New`.

- [ ] **Step 5: Реализация**

`internal/usecases/delete_division/scenario.go`:

```go
package delete_division

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление единицы административного деления».
type Scenario struct {
	divisions DivisionRepo
}

// New создаёт сценарий.
func New(divisions DivisionRepo) *Scenario {
	return &Scenario{divisions: divisions}
}

// DeleteDivision удаляет единицу деления. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// единицы — models.ErrNotFound; на единицу ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteDivision(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.divisions.DeleteAdministrativeDivision(ctx, id)
}

// validateID проверяет формат идентификатора деления; ошибка —
// *models.ValidationError по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeAdministrativeDivision)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeAdministrativeDivision,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
```

- [ ] **Step 6: Тесты проходят**

Run: `go test ./internal/usecases/delete_division/...`
Expected: PASS.

- [ ] **Step 7: Рубеж и commit**

Run: `gofmt -l . && go build ./... && go vet ./... && go test ./...`

```bash
git add internal/usecases/delete_division/
git commit -m "feat(usecases): сценарий delete_division — удаление деления"
```

---

### Task 3: Сценарий `create_division`

**Files:**
- Create: `internal/usecases/create_division/deps.go`
- Create: `internal/usecases/create_division/deps_test.go` (генерируется mockgen)
- Create: `internal/usecases/create_division/scenario.go`
- Create: `internal/usecases/create_division/scenario_test.go`

**Interfaces:**
- Consumes: `store.Store.InTx(ctx, fn func(store.Store) error) error`, внутри транзакции — `GetAdministrativeDivision`, `SaveAdministrativeDivision`; `idgen.Generator.New(models.Type) models.ID` (удовлетворяет `IDGenerator`); `(*models.AdministrativeDivision).Validate() error`.
- Produces: `create_division.New(st DivisionStore, ids IDGenerator) *Scenario`; `(*Scenario).CreateDivision(ctx context.Context, d models.AdministrativeDivision) (models.AdministrativeDivision, error)` — возвращает созданную единицу с заполненным `ID`. `IDGenerator interface { New(models.Type) models.ID }`. S15 зовёт из `POST /api/admin-divisions` и тула `division_create`; `internal/app` (в S15) передаст `sqlstore.Store` и `idgen.New()`.

- [ ] **Step 1: Написать `deps.go`**

```go
package create_division

import (
	"context"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// DivisionStore — зависимость сценария: транзакция порта store.Store. Проверка
// родителя и сохранение идут в одной транзакции на переданном fn хранилище.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type DivisionStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}

// IDGenerator — генератор идентификаторов сущностей (реализация — internal/idgen).
type IDGenerator interface {
	New(t models.Type) models.ID
}
```

- [ ] **Step 2: Сгенерировать мок**

Run: `go generate ./internal/usecases/create_division/...`
Expected: появился `deps_test.go` с `MockDivisionStore` и `MockIDGenerator`.

- [ ] **Step 3: Написать падающие тесты**

`internal/usecases/create_division/scenario_test.go`:

```go
package create_division

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// adID возвращает корректный идентификатор деления, отличающийся последним символом.
func adID(last byte) models.ID {
	return models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// stubIDs — генератор с заранее известным результатом; запоминает запрошенный тип.
type stubIDs struct {
	id      models.ID
	gotType models.Type
}

func (s *stubIDs) New(t models.Type) models.ID {
	s.gotType = t

	return s.id
}

// fakeTx реализует нужные сценарию методы store.Store поверх карты делений;
// остальные методы порта паникуют через nil-встраивание — сценарий не должен
// их звать.
type fakeTx struct {
	store.Store
	divisions map[models.ID]*models.AdministrativeDivision
	saved     []*models.AdministrativeDivision
	getErr    error
	saveErr   error
}

func newFakeTx(existing ...*models.AdministrativeDivision) *fakeTx {
	tx := &fakeTx{divisions: map[models.ID]*models.AdministrativeDivision{}}
	for _, d := range existing {
		tx.divisions[d.ID] = d
	}

	return tx
}

func (f *fakeTx) GetAdministrativeDivision(_ context.Context, id models.ID) (*models.AdministrativeDivision, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

	d, ok := f.divisions[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *d

	return &cp, nil
}

func (f *fakeTx) SaveAdministrativeDivision(_ context.Context, d *models.AdministrativeDivision) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *d
	f.divisions[d.ID] = &cp
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует DivisionStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx      *fakeTx
	inTxErr error // ошибка самого InTx (например, отменённый контекст)
	calls   int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	if f.inTxErr != nil {
		return f.inTxErr
	}

	return fn(f.tx)
}

func validInput() models.AdministrativeDivision {
	return models.AdministrativeDivision{Name: "Село", Type: models.AdminDivisionSelo}
}

func TestCreateDivisionGeneratesIDAndSaves(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	ids := &stubIDs{id: adID('V')}

	got, err := New(st, ids).CreateDivision(context.Background(), validInput())
	if err != nil {
		t.Fatalf("CreateDivision: %v", err)
	}

	if got.ID != adID('V') || got.Name != "Село" {
		t.Fatalf("got %+v, ожидалась единица с ID %v", got, adID('V'))
	}

	if ids.gotType != models.TypeAdministrativeDivision {
		t.Errorf("генератор вызван с типом %q, ожидался administrative_division", ids.gotType)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].ID != adID('V') {
		t.Fatalf("InTx=%d, saved=%v; ожидалось одно сохранение внутри одной транзакции", st.calls, st.tx.saved)
	}
}

// TestCreateDivisionRejectsExplicitID: явный входной ID допустим только для
// импорта — сценарий отвергает его до генерации и обращения к хранилищу.
func TestCreateDivisionRejectsExplicitID(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}
	ids := &stubIDs{id: adID('V')}

	in := validInput()
	in.ID = adID('0')

	_, err := New(st, ids).CreateDivision(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю id", err)
	}

	if st.calls != 0 || ids.gotType != "" {
		t.Fatalf("InTx=%d, генератор вызван с %q; ожидалось: не вызваны", st.calls, ids.gotType)
	}
}

// TestCreateDivisionValidatesBeforeTx: невалидная сущность (пустое название) —
// *ValidationError, транзакция не открывается.
func TestCreateDivisionValidatesBeforeTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	in := validInput()
	in.Name = ""

	_, err := New(st, &stubIDs{id: adID('V')}).CreateDivision(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю name", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

func TestCreateDivisionWithParentSaves(t *testing.T) {
	parent := &models.AdministrativeDivision{ID: adID('0'), Name: "Волость", Type: models.AdminDivisionVolost}
	st := &fakeStore{tx: newFakeTx(parent)}

	in := validInput()
	pid := parent.ID
	in.ParentID = &pid

	got, err := New(st, &stubIDs{id: adID('V')}).CreateDivision(context.Background(), in)
	if err != nil {
		t.Fatalf("CreateDivision: %v", err)
	}

	if got.ParentID == nil || *got.ParentID != parent.ID || len(st.tx.saved) != 1 {
		t.Fatalf("got %+v, saved=%d; ожидалось сохранение с родителем", got, len(st.tx.saved))
	}
}

// TestCreateDivisionParentNotFound: несуществующий родитель — *ValidationError
// по полю parent_id (в S15 — 422), ничего не сохраняется.
func TestCreateDivisionParentNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	in := validInput()
	pid := adID('0')
	in.ParentID = &pid

	_, err := New(st, &stubIDs{id: adID('V')}).CreateDivision(context.Background(), in)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d единиц при несуществующем родителе", len(st.tx.saved))
	}
}

func TestCreateDivisionPropagatesSaveError(t *testing.T) {
	wantErr := errors.New("save failed")
	st := &fakeStore{tx: newFakeTx()}
	st.tx.saveErr = wantErr

	if _, err := New(st, &stubIDs{id: adID('V')}).CreateDivision(context.Background(), validInput()); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

func TestCreateDivisionPropagatesTxError(t *testing.T) {
	wantErr := errors.New("tx failed")
	st := &fakeStore{tx: newFakeTx(), inTxErr: wantErr}

	if _, err := New(st, &stubIDs{id: adID('V')}).CreateDivision(context.Background(), validInput()); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}

// TestCreateDivisionPropagatesParentGetError: сбой чтения родителя (не
// ErrNotFound) — ошибка хранилища как есть, не *ValidationError.
func TestCreateDivisionPropagatesParentGetError(t *testing.T) {
	wantErr := errors.New("get failed")
	st := &fakeStore{tx: newFakeTx()}
	st.tx.getErr = wantErr

	in := validInput()
	pid := adID('0')
	in.ParentID = &pid

	_, err := New(st, &stubIDs{id: adID('V')}).CreateDivision(context.Background(), in)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}

	var ve *models.ValidationError
	if errors.As(err, &ve) {
		t.Fatalf("сбой хранилища превращён в *ValidationError: %v", err)
	}
}
```

- [ ] **Step 4: Убедиться, что тесты падают**

Run: `go test ./internal/usecases/create_division/...`
Expected: FAIL — `undefined: New`.

- [ ] **Step 5: Реализация**

`internal/usecases/create_division/scenario.go`:

```go
package create_division

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «создание единицы административного деления».
type Scenario struct {
	store DivisionStore
	ids   IDGenerator
}

// New создаёт сценарий.
func New(st DivisionStore, ids IDGenerator) *Scenario {
	return &Scenario{store: st, ids: ids}
}

// CreateDivision создаёт единицу деления: генерирует идентификатор, проверяет
// инварианты, в одной транзакции убеждается в существовании родителя и
// сохраняет. Возвращает созданную единицу с заполненным ID.
//
// Ошибки: непустой входной ID (явный идентификатор допустим только для
// импорта), невалидная сущность и несуществующий родитель —
// *models.ValidationError (поля id, соответствующее и parent_id); прочее —
// ошибки хранилища как есть.
func (s *Scenario) CreateDivision(ctx context.Context, d models.AdministrativeDivision) (models.AdministrativeDivision, error) {
	if d.ID != "" {
		return models.AdministrativeDivision{}, &models.ValidationError{
			Entity: models.TypeAdministrativeDivision,
			Field:  "id",
			Reason: fmt.Sprintf("идентификатор %q задан снаружи: при создании он генерируется", d.ID),
		}
	}

	d.ID = s.ids.New(models.TypeAdministrativeDivision)

	if err := d.Validate(); err != nil {
		return models.AdministrativeDivision{}, err
	}

	err := s.store.InTx(ctx, func(tx store.Store) error {
		if d.ParentID != nil {
			if _, err := tx.GetAdministrativeDivision(ctx, *d.ParentID); err != nil {
				if errors.Is(err, models.ErrNotFound) {
					return parentErr("родитель %q не найден", *d.ParentID)
				}

				return err
			}
		}

		return tx.SaveAdministrativeDivision(ctx, &d)
	})
	if err != nil {
		return models.AdministrativeDivision{}, err
	}

	return d, nil
}

// parentErr — *models.ValidationError по полю parent_id.
func parentErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeAdministrativeDivision,
		Field:  "parent_id",
		Reason: fmt.Sprintf(format, args...),
	}
}
```

- [ ] **Step 6: Тесты проходят**

Run: `go test ./internal/usecases/create_division/...`
Expected: PASS.

- [ ] **Step 7: Рубеж и commit**

Run: `gofmt -l . && go build ./... && go vet ./... && go test ./...`

```bash
git add internal/usecases/create_division/
git commit -m "feat(usecases): сценарий create_division — создание деления с генерацией ID"
```

---

### Task 4: Сценарий `update_division`

**Files:**
- Create: `internal/usecases/update_division/deps.go`
- Create: `internal/usecases/update_division/deps_test.go` (генерируется mockgen)
- Create: `internal/usecases/update_division/scenario.go`
- Create: `internal/usecases/update_division/scenario_test.go`

**Interfaces:**
- Consumes: `store.Store.InTx`, `GetAdministrativeDivision`, `SaveAdministrativeDivision`; `(*models.AdministrativeDivision).Validate() error` (само-родитель уже отвергается там — `validateNotSelf`).
- Produces: `update_division.New(st DivisionStore) *Scenario`; `(*Scenario).UpdateDivision(ctx context.Context, d models.AdministrativeDivision) error` — полная замена единицы по `d.ID`. S15 зовёт из `PUT /api/admin-divisions/{id}` и тула `division_update`.

- [ ] **Step 1: Написать `deps.go`**

```go
package update_division

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// DivisionStore — зависимость сценария: транзакция порта store.Store. Чтение
// текущей единицы, проверка цепочки родителей и сохранение идут в одной
// транзакции на переданном fn хранилище.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type DivisionStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
```

- [ ] **Step 2: Сгенерировать мок**

Run: `go generate ./internal/usecases/update_division/...`
Expected: появился `deps_test.go` с `MockDivisionStore`.

- [ ] **Step 3: Написать падающие тесты**

`internal/usecases/update_division/scenario_test.go`:

```go
package update_division

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// adID возвращает корректный идентификатор деления, отличающийся последним символом.
func adID(last byte) models.ID {
	return models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last))
}

// fakeTx реализует нужные сценарию методы store.Store поверх карты делений;
// остальные методы порта паникуют через nil-встраивание.
type fakeTx struct {
	store.Store
	divisions map[models.ID]*models.AdministrativeDivision
	saved     []*models.AdministrativeDivision
	saveErr   error
	gets      int
}

func newFakeTx(existing ...*models.AdministrativeDivision) *fakeTx {
	tx := &fakeTx{divisions: map[models.ID]*models.AdministrativeDivision{}}
	for _, d := range existing {
		tx.divisions[d.ID] = d
	}

	return tx
}

func (f *fakeTx) GetAdministrativeDivision(_ context.Context, id models.ID) (*models.AdministrativeDivision, error) {
	f.gets++

	d, ok := f.divisions[id]
	if !ok {
		return nil, models.ErrNotFound
	}
	cp := *d

	return &cp, nil
}

func (f *fakeTx) SaveAdministrativeDivision(_ context.Context, d *models.AdministrativeDivision) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *d
	f.divisions[d.ID] = &cp
	f.saved = append(f.saved, &cp)

	return nil
}

// fakeStore реализует DivisionStore: InTx выполняет fn на встроенном fakeTx.
type fakeStore struct {
	tx    *fakeTx
	calls int
}

func (f *fakeStore) InTx(_ context.Context, fn func(store.Store) error) error {
	f.calls++

	return fn(f.tx)
}

func division(id models.ID, parent *models.ID) *models.AdministrativeDivision {
	return &models.AdministrativeDivision{ID: id, Name: "Единица " + string(id), Type: models.AdminDivisionSelo, ParentID: parent}
}

func TestUpdateDivisionSaves(t *testing.T) {
	existing := division(adID('V'), nil)
	st := &fakeStore{tx: newFakeTx(existing)}

	updated := *existing
	updated.Name = "Новое название"

	if err := New(st).UpdateDivision(context.Background(), updated); err != nil {
		t.Fatalf("UpdateDivision: %v", err)
	}

	if st.calls != 1 || len(st.tx.saved) != 1 || st.tx.saved[0].Name != "Новое название" {
		t.Fatalf("InTx=%d, saved=%v; ожидалось одно сохранение с новым названием", st.calls, st.tx.saved)
	}
}

func TestUpdateDivisionNotFound(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	err := New(st).UpdateDivision(context.Background(), *division(adID('V'), nil))
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d единиц при несуществующей единице", len(st.tx.saved))
	}
}

// TestUpdateDivisionValidatesBeforeTx: невалидная сущность — *ValidationError,
// транзакция не открывается.
func TestUpdateDivisionValidatesBeforeTx(t *testing.T) {
	st := &fakeStore{tx: newFakeTx()}

	bad := *division(adID('V'), nil)
	bad.Name = ""

	err := New(st).UpdateDivision(context.Background(), bad)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю name", err)
	}

	if st.calls != 0 {
		t.Fatalf("InTx вызван %d раз при невалидной сущности", st.calls)
	}
}

// TestUpdateDivisionParentNotFound: новый родитель не существует —
// *ValidationError по полю parent_id, ничего не сохраняется.
func TestUpdateDivisionParentNotFound(t *testing.T) {
	existing := division(adID('V'), nil)
	st := &fakeStore{tx: newFakeTx(existing)}

	missing := adID('0')
	updated := *existing
	updated.ParentID = &missing

	err := New(st).UpdateDivision(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d единиц при несуществующем родителе", len(st.tx.saved))
	}
}

// TestUpdateDivisionDirectCycle: B — дочка A; попытка сделать A дочкой B — цикл,
// *ValidationError по полю parent_id.
func TestUpdateDivisionDirectCycle(t *testing.T) {
	idA, idB := adID('A'), adID('B')
	a := division(idA, nil)
	b := division(idB, &idA)
	st := &fakeStore{tx: newFakeTx(a, b)}

	updated := *a
	updated.ParentID = &idB

	err := New(st).UpdateDivision(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id (цикл)", err)
	}

	if len(st.tx.saved) != 0 {
		t.Fatalf("сохранено %d единиц при цикле", len(st.tx.saved))
	}
}

// TestUpdateDivisionLongCycle: цепочка C → B → A; попытка сделать A дочкой C —
// цикл длиной три, *ValidationError по полю parent_id.
func TestUpdateDivisionLongCycle(t *testing.T) {
	idA, idB, idC := adID('A'), adID('B'), adID('C')
	a := division(idA, nil)
	b := division(idB, &idA)
	c := division(idC, &idB)
	st := &fakeStore{tx: newFakeTx(a, b, c)}

	updated := *a
	updated.ParentID = &idC

	err := New(st).UpdateDivision(context.Background(), updated)

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "parent_id" {
		t.Fatalf("err = %v, ожидалась *ValidationError по полю parent_id (цикл)", err)
	}
}

// TestUpdateDivisionReparentOK: перенос под другого корректного родителя проходит.
func TestUpdateDivisionReparentOK(t *testing.T) {
	idA, idB, idC := adID('A'), adID('B'), adID('C')
	a := division(idA, nil)
	b := division(idB, &idA)
	c := division(idC, nil)
	st := &fakeStore{tx: newFakeTx(a, b, c)}

	updated := *b
	updated.ParentID = &idC

	if err := New(st).UpdateDivision(context.Background(), updated); err != nil {
		t.Fatalf("UpdateDivision: %v", err)
	}

	if len(st.tx.saved) != 1 || *st.tx.saved[0].ParentID != idC {
		t.Fatalf("saved=%v; ожидалось сохранение с родителем %v", st.tx.saved, idC)
	}
}

// TestUpdateDivisionWithoutParentSkipsWalk: без родителя цепочка не обходится —
// одно чтение (существование самой единицы).
func TestUpdateDivisionWithoutParentSkipsWalk(t *testing.T) {
	existing := division(adID('V'), nil)
	st := &fakeStore{tx: newFakeTx(existing)}

	if err := New(st).UpdateDivision(context.Background(), *existing); err != nil {
		t.Fatalf("UpdateDivision: %v", err)
	}

	if st.tx.gets != 1 {
		t.Fatalf("чтений %d, ожидалось одно (без обхода родителей)", st.tx.gets)
	}
}

func TestUpdateDivisionPropagatesSaveError(t *testing.T) {
	wantErr := errors.New("save failed")
	existing := division(adID('V'), nil)
	st := &fakeStore{tx: newFakeTx(existing)}
	st.tx.saveErr = wantErr

	if err := New(st).UpdateDivision(context.Background(), *existing); !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, ожидалась %v", err, wantErr)
	}
}
```

- [ ] **Step 4: Убедиться, что тесты падают**

Run: `go test ./internal/usecases/update_division/...`
Expected: FAIL — `undefined: New`.

- [ ] **Step 5: Реализация**

`internal/usecases/update_division/scenario.go`:

```go
package update_division

import (
	"context"
	"errors"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store"
)

// Scenario — сценарий «изменение единицы административного деления».
type Scenario struct {
	store DivisionStore
}

// New создаёт сценарий.
func New(st DivisionStore) *Scenario {
	return &Scenario{store: st}
}

// UpdateDivision полностью заменяет единицу деления по d.ID: проверяет
// инварианты, в одной транзакции убеждается, что единица существует, а цепочка
// родителей не проходит через неё саму (цикл), и сохраняет.
//
// Ошибки: невалидная сущность, несуществующий родитель и цикл по parent_id —
// *models.ValidationError (соответствующее поле и parent_id); нет такой
// единицы — models.ErrNotFound; прочее — ошибки хранилища как есть.
func (s *Scenario) UpdateDivision(ctx context.Context, d models.AdministrativeDivision) error {
	if err := d.Validate(); err != nil {
		return err
	}

	return s.store.InTx(ctx, func(tx store.Store) error {
		if _, err := tx.GetAdministrativeDivision(ctx, d.ID); err != nil {
			return err
		}

		if err := checkParentChain(ctx, tx, &d); err != nil {
			return err
		}

		return tx.SaveAdministrativeDivision(ctx, &d)
	})
}

// checkParentChain обходит цепочку родителей d вверх: каждый предок должен
// существовать, и цепочка не должна проходить через саму единицу или
// замыкаться (защита от испорченных данных). Ошибка цепочки —
// *models.ValidationError по полю parent_id.
func checkParentChain(ctx context.Context, tx store.Store, d *models.AdministrativeDivision) error {
	seen := map[models.ID]bool{d.ID: true}

	for cur := d.ParentID; cur != nil; {
		if seen[*cur] {
			return parentErr("цепочка родителей %q проходит через саму единицу или замыкается на %q", d.ID, *cur)
		}
		seen[*cur] = true

		p, err := tx.GetAdministrativeDivision(ctx, *cur)
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

// parentErr — *models.ValidationError по полю parent_id.
func parentErr(format string, args ...any) *models.ValidationError {
	return &models.ValidationError{
		Entity: models.TypeAdministrativeDivision,
		Field:  "parent_id",
		Reason: fmt.Sprintf(format, args...),
	}
}
```

- [ ] **Step 6: Тесты проходят**

Run: `go test ./internal/usecases/update_division/...`
Expected: PASS.

- [ ] **Step 7: Рубеж и commit**

Run: `gofmt -l . && go build ./... && go vet ./... && go test ./...`

```bash
git add internal/usecases/update_division/
git commit -m "feat(usecases): сценарий update_division — изменение деления с проверкой циклов"
```

---

### Task 5: Документация и рубеж этапа

**Files:**
- Modify: `docs/plans/2026-09-20-core-rw-roadmap.md` (раздел S14: решения этапа, ссылка на план; раздел S15: предпосылки из S14)
- Modify: `docs/data-model/core-read-write.md` §5 (уточнение семантики ошибок сценариев записи)

**Interfaces:**
- Consumes: фактические решения задач 1–4.
- Produces: зафиксированные предпосылки для плана S15.

- [ ] **Step 1: Обновить раздел S14 дорожной карты**

В `docs/plans/2026-09-20-core-rw-roadmap.md` в конец раздела «S14. Сценарии записи делений» добавить:

```markdown
- **Решения этапа:** `create` отвергает непустой входной `ID`
  (`*ValidationError` по полю `id`; явный ID — только для импорта);
  несуществующий родитель и цикл по `parent_id` — `*ValidationError` по полю
  `parent_id` (в S15 → 422, не 404); `get`/`delete` при неверном формате
  `id`-аргумента — `*ValidationError` по полю `id`; `update` — полная замена
  сущности по `d.ID` с обходом цепочки родителей внутри `InTx` (`update`
  возвращает только ошибку, тело для ответа S15 берёт из запроса);
  `create`/`update` зависят от `DivisionStore{InTx}` и работают с полным
  `store.Store` внутри транзакции; `IDGenerator` объявлен в
  `create_division/deps.go`, реализация — `internal/idgen` без обёрток.
  Предпосылка S4 о канонических фикстурах sqlstore не понадобилась: тесты
  сценариев работают на фейках; для сквозных тестов S15 сущности создаются
  через сам сценарий `create`. План — `2026-09-22-core-rw-s14-division-write.md`.
```

- [ ] **Step 2: Дополнить предпосылки S15 в дорожной карте**

В раздел «S15. Контракты записи делений» после существующих предпосылок добавить:

```markdown
- **Предпосылка из S14:** обработчикам достаточно `errors.As`/`errors.Is` по
  трём типам: `*ValidationError` → 422 (включая неверный формат `id` в пути —
  не 404), `ErrNotFound` → 404, `*InUseError` → 409; сценарии собраны, в
  `internal/app` не подключены — S15 добавляет создание `idgen.New()` и
  прокидку четырёх сценариев в `httpapi`/`mcp`.
```

- [ ] **Step 3: Уточнить §5 спеки**

В `docs/data-model/core-read-write.md`, §5, после абзаца «Сценарий `create` генерирует `ID` …» добавить:

```markdown
- Ошибки сценариев записи делений (S14): неверный формат `id`-аргумента у
  `get`/`delete`, явный `ID` на входе `create`, несуществующий родитель и цикл
  по `parent_id` — `*ValidationError` (поля `id`/`parent_id`); отсутствие самой
  единицы — `ErrNotFound`. Проверка родителя/циклов и сохранение — в одной
  транзакции (`InTx`); цикл ищется обходом цепочки родителей вверх с защитой от
  замыкания на испорченных данных.
```

- [ ] **Step 4: Рубеж этапа**

Run: `gofmt -l . && go build ./... && go vet ./... && go test ./...`
Expected: gofmt пусто, всё зелёное.

- [ ] **Step 5: Commit**

```bash
git add docs/plans/2026-09-20-core-rw-roadmap.md docs/data-model/core-read-write.md
git commit -m "docs: S14 — решения по сценариям записи делений, предпосылки S15"
```
