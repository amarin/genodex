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
}

func (f *fakeDivisions) ListDivisions(_ context.Context, q models.DivisionQuery) ([]models.AdministrativeDivision, error) {
	f.got = q

	return f.list, f.err
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

// TestOldSettlementsRouteIsGone: прежнего имени контракта нет.
func TestOldSettlementsRouteIsGone(t *testing.T) {
	rec := get(t, NewHandler(&fakeDivisions{}, fstest.MapFS{}), "/api/settlements")

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /api/settlements = %d, ожидался 404", rec.Code)
	}
}
