package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/httpapi"
	"github.com/amarin/genodex/internal/idgen"
	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store/sqlstore"
	create_division "github.com/amarin/genodex/internal/usecases/create_division"
	delete_division "github.com/amarin/genodex/internal/usecases/delete_division"
	get_division "github.com/amarin/genodex/internal/usecases/get_division"
	list_divisions "github.com/amarin/genodex/internal/usecases/list_divisions"
	update_division "github.com/amarin/genodex/internal/usecases/update_division"
)

// divisionService — сборка httpapi.DivisionService на настоящих сценариях
// (так же собран internal/app).
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

// newDivisionService собирает фасад на настоящем хранилище.
func newDivisionService(t *testing.T, st *sqlstore.Store) *divisionService {
	t.Helper()

	return &divisionService{
		list:   list_divisions.New(st),
		get:    get_division.New(st),
		create: create_division.New(st, idgen.New()),
		update: update_division.New(st),
		del:    delete_division.New(st),
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

	root := models.ID("ad-root")

	for _, d := range []models.AdministrativeDivision{
		{ID: "ad-root", Name: "Московская", Type: models.AdminDivisionGovernorate},
		{ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo, ParentID: &root},
		{ID: "ad-2", Name: "Никифоровская", Type: models.AdminDivisionVolost, ParentID: &root},
		{ID: "ad-3", Name: "Никифорово", Type: models.AdminDivisionDerevnya, ParentID: &root},
	} {
		if err := st.SaveAdministrativeDivision(t.Context(), &d); err != nil {
			t.Fatalf("save %s: %v", d.ID, err)
		}
	}

	h := httpapi.NewHandler(newDivisionService(t, st), fstest.MapFS{})

	cases := []struct {
		target string
		code   int
		body   string // пусто — не сверять
	}{
		{"/api/admin-divisions?kind=settlement", http.StatusOK,
			`[{"id":"ad-1","name":"Давыдово","type":"selo","parent_id":"ad-root"},` +
				`{"id":"ad-3","name":"Никифорово","type":"derevnya","parent_id":"ad-root"}]`},
		{"/api/admin-divisions?kind=settlement&limit=1&offset=1", http.StatusOK,
			`[{"id":"ad-3","name":"Никифорово","type":"derevnya","parent_id":"ad-root"}]`},
		{"/api/admin-divisions?type=governorate", http.StatusOK,
			`[{"id":"ad-root","name":"Московская","type":"governorate","parent_id":null}]`},
		{"/api/admin-divisions?type=castle", http.StatusUnprocessableEntity,
			`{"error":"type: неизвестный тип единицы деления \"castle\"","field":"type"}`},
		{"/api/admin-divisions?limit=x", http.StatusBadRequest,
			`{"error":"параметр limit: ожидалось целое число, получено \"x\""}`},
		{"/api/settlements", http.StatusNotFound, ""},
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
