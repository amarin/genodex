package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

// fakeDivisions запоминает запрос и отдаёт заданный ответ.
type fakeDivisions struct {
	list []models.AdministrativeDivision
	err  error
	got  models.DivisionQuery
	call int

	getDiv    models.AdministrativeDivision
	created   models.AdministrativeDivision
	gotCreate models.AdministrativeDivision
	updated   models.AdministrativeDivision
	gotIDs    []models.ID
	deleteErr error

	search    []models.AdministrativeDivision
	searchErr error
	gotSearch models.DivisionSearchQuery

	gotListAccess   models.Access
	gotSearchAccess models.Access
}

func (f *fakeDivisions) ListDivisions(_ context.Context, access models.Access, q models.DivisionQuery) ([]models.AdministrativeDivision, error) {
	f.got = q
	f.gotListAccess = access

	return f.list, f.err
}

func (f *fakeDivisions) SearchDivisions(_ context.Context, access models.Access, q models.DivisionSearchQuery) ([]models.AdministrativeDivision, error) {
	f.gotSearch = q
	f.gotSearchAccess = access

	return f.search, f.searchErr
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

func get(t *testing.T, h http.Handler, target string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))

	return rec
}

func TestDivisionListContract(t *testing.T) {
	root := models.ID("ad-root")
	svc := &fakeDivisions{list: []models.AdministrativeDivision{
		{ID: "ad-root", Name: "Московская", Type: models.AdminDivisionGovernorate},
		{ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo, ParentID: &root},
	}}

	rec := get(t, NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}

	want := `[{"id":"ad-root","name":"Московская","type":"governorate","parent_id":null},` +
		`{"id":"ad-1","name":"Давыдово","type":"selo","parent_id":"ad-root"}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}

	if !reflect.DeepEqual(svc.got, models.DivisionQuery{}) {
		t.Fatalf("запрос без параметров дошёл до сценария как %+v", svc.got)
	}
}

func TestDivisionListEmptyIsJSONArray(t *testing.T) {
	rec := get(t, NewHandler(&fakeDivisions{}, fstest.MapFS{}), "/api/admin-divisions")

	if got := strings.TrimSpace(rec.Body.String()); rec.Code != http.StatusOK || got != `[]` {
		t.Fatalf("status = %d, body = %s; ожидалось 200 и []", rec.Code, got)
	}
}

// TestDivisionListPassesParameters: параметры kind/type/limit/offset доходят до сценария.
func TestDivisionListPassesParameters(t *testing.T) {
	svc := &fakeDivisions{}

	rec := get(t, NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions?kind=settlement&type=selo&limit=20&offset=40")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}

	want := models.DivisionQuery{
		Kind: models.DivisionKindSettlement, Type: models.AdminDivisionSelo,
		Page: models.Page{Limit: 20, Offset: 40},
	}
	if svc.got != want {
		t.Fatalf("запрос %+v, ожидался %+v", svc.got, want)
	}
}

// TestDivisionListBadNumberIs400: параметр limit/offset — не число.
func TestDivisionListBadNumberIs400(t *testing.T) {
	for _, target := range []string{"/api/admin-divisions?limit=abc", "/api/admin-divisions?offset=1.5"} {
		svc := &fakeDivisions{}
		rec := get(t, NewHandler(svc, fstest.MapFS{}), target)

		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"error"`) {
			t.Errorf("%s: status = %d, body = %s; ожидался 400 с error", target, rec.Code, rec.Body)
		}

		if svc.got != (models.DivisionQuery{}) {
			t.Errorf("%s: сценарий вызван при неверном параметре: %+v", target, svc.got)
		}
	}
}

// TestDivisionListValidationErrorIs422: неверное значение — 422 с полем.
func TestDivisionListValidationErrorIs422(t *testing.T) {
	svc := &fakeDivisions{err: &models.ValidationError{Field: "kind", Reason: "неизвестный вид"}}

	rec := get(t, NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions?kind=village")

	want := `{"error":"kind: неизвестный вид","field":"kind"}`
	if got := strings.TrimSpace(rec.Body.String()); rec.Code != http.StatusUnprocessableEntity || got != want {
		t.Fatalf("status = %d, body = %s; ожидалось 422 и %s", rec.Code, got, want)
	}
}

func TestDivisionListServiceErrorIs500(t *testing.T) {
	svc := &fakeDivisions{err: errors.New("хранилище недоступно")}

	rec := get(t, NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions")

	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "хранилище недоступно") {
		t.Fatalf("status = %d, body = %s; ожидался 500 с текстом ошибки", rec.Code, rec.Body)
	}
}

// TestDivisionListPassesParentID: parent_id доходит до сценария.
func TestDivisionListPassesParentID(t *testing.T) {
	svc := &fakeDivisions{}

	rec := get(t, NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions?parent_id=ad-root")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body)
	}

	want := models.ID("ad-root")
	if svc.got.ParentID == nil || *svc.got.ParentID != want {
		t.Fatalf("got.ParentID = %v, ожидался %s", svc.got.ParentID, want)
	}
}

func TestDivisionSearch(t *testing.T) {
	svc := &fakeDivisions{search: []models.AdministrativeDivision{
		{ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo},
	}}

	rec := get(t, NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions/search?q=давы")

	want := `[{"id":"ad-1","name":"Давыдово","type":"selo","parent_id":null}]`
	if got := strings.TrimSpace(rec.Body.String()); rec.Code != http.StatusOK || got != want {
		t.Fatalf("status = %d, body = %s; ожидалось 200 и %s", rec.Code, got, want)
	}

	if svc.gotSearch.Text != "давы" {
		t.Fatalf("gotSearch.Text = %q, ожидалось %q", svc.gotSearch.Text, "давы")
	}
}

// TestDivisionSearchRouteDoesNotHitID: литеральный маршрут /search побеждает {id};
// пустой фейк отвечает 200 пустым массивом, не 422 «неверный формат id».
func TestDivisionSearchRouteDoesNotHitID(t *testing.T) {
	rec := get(t, NewHandler(&fakeDivisions{}, fstest.MapFS{}), "/api/admin-divisions/search?q=давы")

	if got := strings.TrimSpace(rec.Body.String()); rec.Code != http.StatusOK || got != `[]` {
		t.Fatalf("status = %d, body = %s; ожидалось 200 []", rec.Code, got)
	}
}

// TestDivisionSearchBadNumberIs400: limit/offset — не число.
func TestDivisionSearchBadNumberIs400(t *testing.T) {
	svc := &fakeDivisions{}

	rec := get(t, NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions/search?q=давы&limit=abc")

	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"error"`) {
		t.Fatalf("status = %d, body = %s; ожидался 400 с error", rec.Code, rec.Body)
	}
}

// TestDivisionSearchNegativeLimitIs422: отрицательное окно — ошибка валидации сценария.
func TestDivisionSearchNegativeLimitIs422(t *testing.T) {
	svc := &fakeDivisions{searchErr: &models.ValidationError{Field: "limit", Reason: "не может быть отрицательным"}}

	rec := get(t, NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions/search?q=давы&limit=-1")

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %s; ожидался 422", rec.Code, rec.Body)
	}
}

// TestDivisionSearchServiceErrorIs500: прочая ошибка сценария — 500.
func TestDivisionSearchServiceErrorIs500(t *testing.T) {
	svc := &fakeDivisions{searchErr: errors.New("хранилище недоступно")}

	rec := get(t, NewHandler(svc, fstest.MapFS{}), "/api/admin-divisions/search?q=давы")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body = %s; ожидался 500", rec.Code, rec.Body)
	}
}

// TestOldSettlementsRouteIsGone: прежнего имени контракта нет.
func TestOldSettlementsRouteIsGone(t *testing.T) {
	rec := get(t, NewHandler(&fakeDivisions{}, fstest.MapFS{}), "/api/settlements")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /api/settlements = %d, ожидался 404", rec.Code)
	}
}

// TestDivisionListPassesAccessFromContext: Access, положенный resolveAccess в
// контекст запроса, доходит до сценария как есть (не захардкожен на
// AccessFull — auth.md §6, приёмка этапа C).
func TestDivisionListPassesAccessFromContext(t *testing.T) {
	svc := &fakeDivisions{}

	req := httptest.NewRequest(http.MethodGet, "/api/admin-divisions", nil)
	req = req.WithContext(context.WithValue(req.Context(), accessCtxKey, models.AccessPublic))

	rec := httptest.NewRecorder()
	NewHandler(svc, fstest.MapFS{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if svc.gotListAccess != models.AccessPublic {
		t.Fatalf("gotListAccess = %v, ожидался AccessPublic", svc.gotListAccess)
	}
}

// TestDivisionSearchPassesAccessFromContext: аналогично для поиска.
func TestDivisionSearchPassesAccessFromContext(t *testing.T) {
	svc := &fakeDivisions{}

	req := httptest.NewRequest(http.MethodGet, "/api/admin-divisions/search?q=давы", nil)
	req = req.WithContext(context.WithValue(req.Context(), accessCtxKey, models.AccessPublic))

	rec := httptest.NewRecorder()
	NewHandler(svc, fstest.MapFS{}).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if svc.gotSearchAccess != models.AccessPublic {
		t.Fatalf("gotSearchAccess = %v, ожидался AccessPublic", svc.gotSearchAccess)
	}
}
