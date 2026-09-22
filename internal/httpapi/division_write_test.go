package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	authpkg "github.com/amarin/genodex/internal/auth"
	"github.com/amarin/genodex/internal/models"
)

const writeID = "AD-01ARZ3NDEKTSV4RRFFQ69G5FAV"
const writeRefID = "AD-01ARZ3NDEKTSV4RRFFQ69G5FA0"

// ownerCtx кладёт в контекст запроса Access=Full и OwnerID — как resolveAccess
// при валидной cookie-сессии. Существующие тесты записи проверяют контракт
// хендлера для аутентифицированного владельца; анонимный путь — отдельные
// тесты TestDivision*AnonymousIs401 ниже.
func ownerCtx(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), accessCtxKey, models.AccessFull)
	ctx = context.WithValue(ctx, ownerCtxKey, authpkg.ID("OW-01ARZ3NDEKTSV4RRFFQ69G5FA9"))

	return r.WithContext(ctx)
}

func postD(t *testing.T, h http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, ownerCtx(httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))))

	return rec
}

func putD(t *testing.T, h http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, ownerCtx(httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))))

	return rec
}

func delD(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, ownerCtx(httptest.NewRequest(http.MethodDelete, path, nil)))

	return rec
}

func requireStatus(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()

	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}
}

func TestDivisionGetContract(t *testing.T) {
	svc := &fakeDivisions{getDiv: models.AdministrativeDivision{ID: writeID, Name: "Давыдово", Type: models.AdminDivisionSelo}}

	rec := get(t, NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions/"+writeID)

	want := `{"id":"` + writeID + `","name":"Давыдово","type":"selo","parent_id":null}`
	if got := strings.TrimSpace(rec.Body.String()); rec.Code != http.StatusOK || got != want {
		t.Fatalf("status = %d, body = %s; ожидались 200 и %s", rec.Code, got, want)
	}

	if got := svc.gotIDs[0]; got != writeID {
		t.Fatalf("id = %s", got)
	}
}

func TestDivisionGetNotFoundIs404(t *testing.T) {
	svc := &fakeDivisions{err: models.ErrNotFound}

	rec := get(t, NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions/"+writeID)

	requireStatus(t, rec, http.StatusNotFound)
	if got, want := strings.TrimSpace(rec.Body.String()), `{"error":"не найдено"}`; got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

// TestDivisionGetInvalidIDIs422: неверный формат id в пути — 422 (не 404).
func TestDivisionGetInvalidIDIs422(t *testing.T) {
	svc := &fakeDivisions{err: &models.ValidationError{Field: "id", Reason: "неверный формат"}}

	rec := get(t, NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions/ad-1")

	requireStatus(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"id"`) {
		t.Fatalf("body = %s", rec.Body)
	}
}

func TestDivisionCreateContract(t *testing.T) {
	svc := &fakeDivisions{created: models.AdministrativeDivision{ID: writeID, Name: "Давыдово", Type: models.AdminDivisionSelo}}

	rec := postD(t, NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions", `{"name":"Давыдово","type":"selo"}`)

	want := `{"id":"` + writeID + `","name":"Давыдово","type":"selo","parent_id":null}`
	if got := strings.TrimSpace(rec.Body.String()); rec.Code != http.StatusCreated || got != want {
		t.Fatalf("status = %d, body = %s; ожидались 201 и %s", rec.Code, got, want)
	}

	if got := svc.gotCreate; got.Name != "Давыдово" || got.Type != models.AdminDivisionSelo || got.ParentID != nil || got.ID != "" {
		t.Fatalf("создана модель %+v, ожидались name/type без id и parent_id", got)
	}
}

func TestDivisionCreateWithParentPassesModel(t *testing.T) {
	svc := &fakeDivisions{created: models.AdministrativeDivision{ID: writeID, Name: "Давыдово", Type: models.AdminDivisionSelo}}

	body := `{"name":"Давыдово","type":"selo","parent_id":"` + writeRefID + `"}`
	rec := postD(t, NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions", body)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.ParentID == nil || *svc.gotCreate.ParentID != writeRefID {
		t.Fatalf("parent_id = %v", svc.gotCreate.ParentID)
	}
}

func TestDivisionCreateBadJSONIs400(t *testing.T) {
	rec := postD(t, NewHandler(&fakeDivisions{}, fstest.MapFS{}), "/api/admin-divisions", `{`)

	requireStatus(t, rec, http.StatusBadRequest)
	if !strings.Contains(rec.Body.String(), "не удалось разобрать тело") {
		t.Fatalf("body = %s", rec.Body)
	}
}

func TestDivisionCreateValidationErrorIs422(t *testing.T) {
	svc := &fakeDivisions{err: &models.ValidationError{Field: "name", Reason: "пустое значение"}}

	rec := postD(t, NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions", `{"name":"","type":"selo"}`)

	requireStatus(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"name"`) {
		t.Fatalf("body = %s", rec.Body)
	}
}

// TestDivisionUpdateMergesFields: PUT заменяет name/type/parent_id, прочие поля
// текущей модели сохраняются.
func TestDivisionUpdateMergesFields(t *testing.T) {
	parent := models.ID(writeRefID)
	svc := &fakeDivisions{getDiv: models.AdministrativeDivision{
		ID: writeID, Name: "Село", Type: models.AdminDivisionSelo,
		ParentID: &parent, Variants: []string{"Давыдова"},
	}}

	rec := putD(t, NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions/"+writeID, `{"name":"Давыдово","type":"selo","parent_id":null}`)

	want := `{"id":"` + writeID + `","name":"Давыдово","type":"selo","parent_id":null}`
	if got := strings.TrimSpace(rec.Body.String()); rec.Code != http.StatusOK || got != want {
		t.Fatalf("status = %d, body = %s; ожидались 200 и %s", rec.Code, got, want)
	}

	if svc.updated.Name != "Давыдово" || svc.updated.ID != writeID {
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

	rec := putD(t, NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions/"+writeID, `{"name":"Давыдово","type":"selo"}`)

	requireStatus(t, rec, http.StatusNotFound)
}

func TestDivisionUpdateInvalidIDIs422(t *testing.T) {
	svc := &fakeDivisions{err: &models.ValidationError{Field: "id", Reason: "неверный формат"}}

	rec := putD(t, NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions/ad-1", `{"name":"Давыдово","type":"selo"}`)

	requireStatus(t, rec, http.StatusUnprocessableEntity)
}

func TestDivisionDeleteNoContent(t *testing.T) {
	svc := &fakeDivisions{}

	rec := delD(t, NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions/"+writeID)

	requireStatus(t, rec, http.StatusNoContent)
	if rec.Body.Len() != 0 {
		t.Fatalf("204 обязан быть без тела: %q", rec.Body)
	}
	if got := svc.gotIDs[0]; got != writeID {
		t.Fatalf("id = %s", got)
	}
}

func TestDivisionDeleteNotFoundIs404(t *testing.T) {
	svc := &fakeDivisions{deleteErr: models.ErrNotFound}

	rec := delD(t, NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions/"+writeID)

	requireStatus(t, rec, http.StatusNotFound)
}

func TestDivisionDeleteInvalidIDIs422(t *testing.T) {
	svc := &fakeDivisions{deleteErr: &models.ValidationError{Field: "id", Reason: "неверный формат"}}

	rec := delD(t, NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions/ad-1")

	requireStatus(t, rec, http.StatusUnprocessableEntity)
}

// TestDivisionDeleteInUseIs409WithReferrers: тело 409 содержит список ссылающихся.
func TestDivisionDeleteInUseIs409WithReferrers(t *testing.T) {
	svc := &fakeDivisions{deleteErr: &models.InUseError{
		Type: models.TypeAdministrativeDivision, ID: writeID,
		Referrers: []models.EntityRef{{Type: models.TypeAdministrativeDivision, ID: writeRefID}},
	}}

	rec := delD(t, NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions/"+writeID)

	requireStatus(t, rec, http.StatusConflict)
	want := `{"error":"administrative_division \"` + writeID + `\" используется: administrative_division ` + writeRefID +
		`","referrers":[{"type":"administrative_division","id":"` + writeRefID + `"}]}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

// TestDivisionCreateAnonymousIs401: без активной сессии запись отклоняется
// раньше разбора тела — сценарий не вызывается (auth.md §6, приёмка этапа C).
func TestDivisionCreateAnonymousIs401(t *testing.T) {
	svc := &fakeDivisions{}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin-divisions", strings.NewReader(`{"name":"x","type":"selo"}`))
	NewHandler(svc, fstest.MapFS{}).ServeHTTP(rec, req)

	requireStatus(t, rec, http.StatusUnauthorized)
	if svc.gotCreate.ID != "" {
		t.Fatalf("сценарий вызван анонимом: %+v", svc.gotCreate)
	}
}

// TestDivisionUpdateAnonymousIs401: аналогично для PUT.
func TestDivisionUpdateAnonymousIs401(t *testing.T) {
	svc := &fakeDivisions{}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/admin-divisions/"+writeID, strings.NewReader(`{}`))
	NewHandler(svc, fstest.MapFS{}).ServeHTTP(rec, req)

	requireStatus(t, rec, http.StatusUnauthorized)
	if len(svc.gotIDs) != 0 {
		t.Fatalf("сценарий вызван анонимом: %v", svc.gotIDs)
	}
}

// TestDivisionDeleteAnonymousIs401: аналогично для DELETE.
func TestDivisionDeleteAnonymousIs401(t *testing.T) {
	svc := &fakeDivisions{}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/admin-divisions/"+writeID, nil)
	NewHandler(svc, fstest.MapFS{}).ServeHTTP(rec, req)

	requireStatus(t, rec, http.StatusUnauthorized)
	if len(svc.gotIDs) != 0 {
		t.Fatalf("сценарий вызван анонимом: %v", svc.gotIDs)
	}
}
