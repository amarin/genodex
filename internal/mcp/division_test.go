package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

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
	f.call++

	return f.list, f.err
}

func (f *fakeDivisions) SearchDivisions(_ context.Context, access models.Access, q models.DivisionSearchQuery) ([]models.AdministrativeDivision, error) {
	f.gotSearch = q
	f.gotSearchAccess = access
	f.call++

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

func callDivisionList(t *testing.T, svc *fakeDivisions, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := divisionListHandler(svc)(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func resultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()

	text, ok := mcp.AsTextContent(res.Content[0])
	if !ok {
		t.Fatalf("content[0] = %T, want TextContent", res.Content[0])
	}

	return text.Text
}

func TestDivisionListToolContract(t *testing.T) {
	root := models.ID("ad-root")
	svc := &fakeDivisions{list: []models.AdministrativeDivision{
		{ID: "ad-root", Name: "Московская", Type: models.AdminDivisionGovernorate},
		{ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo, ParentID: &root},
	}}

	res := callDivisionList(t, svc, nil)

	want := `[{"id":"ad-root","name":"Московская","type":"governorate","parent_id":null},` +
		`{"id":"ad-1","name":"Давыдово","type":"selo","parent_id":"ad-root"}]`
	if res.IsError || resultText(t, res) != want {
		t.Fatalf("isError=%v text=%s, want %s", res.IsError, resultText(t, res), want)
	}

	if svc.got != (models.DivisionQuery{}) {
		t.Fatalf("вызов без аргументов дошёл до сценария как %+v", svc.got)
	}
}

// TestDivisionListToolPassesArguments: аргументы kind/type/limit/offset
// (число приходит из JSON как float64) доходят до сценария.
func TestDivisionListToolPassesArguments(t *testing.T) {
	svc := &fakeDivisions{}

	res := callDivisionList(t, svc, map[string]any{
		"kind": "settlement", "type": "selo", "limit": float64(20), "offset": float64(40),
	})
	if res.IsError {
		t.Fatalf("неожиданная ошибка тула: %s", resultText(t, res))
	}

	want := models.DivisionQuery{
		Kind: models.DivisionKindSettlement, Type: models.AdminDivisionSelo,
		Page: models.Page{Limit: 20, Offset: 40},
	}
	if svc.got != want {
		t.Fatalf("запрос %+v, ожидался %+v", svc.got, want)
	}
}

// TestDivisionListToolBadNumberIsToolError: нечисловой limit — ошибка тула, сценарий не вызван.
func TestDivisionListToolBadNumberIsToolError(t *testing.T) {
	svc := &fakeDivisions{}

	res := callDivisionList(t, svc, map[string]any{"limit": "abc"})

	if !res.IsError || !strings.Contains(resultText(t, res), "limit") || svc.call != 0 {
		t.Fatalf("isError=%v text=%s calls=%d; ожидалась ошибка про limit без вызова сценария",
			res.IsError, resultText(t, res), svc.call)
	}
}

// TestDivisionListToolNullAndFractionalNumbers: явный null — как отсутствие
// аргумента; дробное число — ошибка тула, а не молчаливое усечение.
func TestDivisionListToolNullAndFractionalNumbers(t *testing.T) {
	svc := &fakeDivisions{}

	res := callDivisionList(t, svc, map[string]any{"limit": nil, "offset": nil})
	if res.IsError || svc.got != (models.DivisionQuery{}) {
		t.Fatalf("null-аргументы: isError=%v запрос=%+v; ожидалось значение по умолчанию", res.IsError, svc.got)
	}

	svc = &fakeDivisions{}

	res = callDivisionList(t, svc, map[string]any{"limit": 1.5})
	if !res.IsError || svc.call != 0 {
		t.Fatalf("дробный limit: isError=%v calls=%d; ожидалась ошибка тула без вызова сценария", res.IsError, svc.call)
	}
}

func TestDivisionListToolErrorsAreToolErrors(t *testing.T) {
	for name, err := range map[string]error{
		"ошибка проверки": &models.ValidationError{Field: "kind", Reason: "неизвестный вид"},
		"сбой хранилища":  errors.New("хранилище недоступно"),
	} {
		res := callDivisionList(t, &fakeDivisions{err: err}, nil)

		if !res.IsError || !strings.Contains(resultText(t, res), err.Error()) {
			t.Errorf("%s: isError=%v text=%s; ожидалась ошибка тула с текстом %q", name, res.IsError, resultText(t, res), err)
		}
	}
}

func callDivisionSearch(t *testing.T, svc *fakeDivisions, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := divisionSearchHandler(svc)(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestDivisionSearchTool(t *testing.T) {
	svc := &fakeDivisions{search: []models.AdministrativeDivision{
		{ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo},
	}}

	res := callDivisionSearch(t, svc, map[string]any{"q": "давы"})

	want := `[{"id":"ad-1","name":"Давыдово","type":"selo","parent_id":null}]`
	if res.IsError || resultText(t, res) != want {
		t.Fatalf("isError=%v text=%s, want %s", res.IsError, resultText(t, res), want)
	}

	if svc.gotSearch.Text != "давы" {
		t.Fatalf("gotSearch.Text = %q, ожидалось %q", svc.gotSearch.Text, "давы")
	}
}

func TestDivisionSearchToolEmptyText(t *testing.T) {
	svc := &fakeDivisions{}

	res := callDivisionSearch(t, svc, map[string]any{"q": ""})

	if res.IsError || resultText(t, res) != `[]` {
		t.Fatalf("isError=%v text=%s, want []", res.IsError, resultText(t, res))
	}
}

func TestDivisionSearchToolInvalidLimitIsError(t *testing.T) {
	svc := &fakeDivisions{}

	res := callDivisionSearch(t, svc, map[string]any{"q": "давы", "limit": "abc"})

	if !res.IsError || svc.call != 0 {
		t.Fatalf("isError=%v calls=%d; ожидалась ошибка тула без вызова сценария", res.IsError, svc.call)
	}
}

// TestDivisionListToolWithParent: parent_id доходит до сценария.
func TestDivisionListToolWithParent(t *testing.T) {
	svc := &fakeDivisions{}

	res := callDivisionList(t, svc, map[string]any{"parent_id": "ad-root"})
	if res.IsError {
		t.Fatalf("неожиданная ошибка тула: %s", resultText(t, res))
	}

	want := models.ID("ad-root")
	if svc.got.ParentID == nil || *svc.got.ParentID != want {
		t.Fatalf("got.ParentID = %v, ожидался %s", svc.got.ParentID, want)
	}
}

// TestNewServerRegistersDivisionSearchTool: тул division_search зарегистрирован.
func TestNewServerRegistersDivisionSearchTool(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}}).ListTools()

	if _, ok := tools["division_search"]; !ok {
		t.Errorf("тул division_search не зарегистрирован: %v", tools)
	}
}

// TestNewServerRegistersDivisionListOnly: тул зарегистрирован под новым именем,
// прежнего settlement_list нет.
func TestNewServerRegistersDivisionListOnly(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}}).ListTools()

	if _, ok := tools["division_list"]; !ok {
		t.Errorf("тул division_list не зарегистрирован: %v", tools)
	}

	if _, ok := tools["settlement_list"]; ok {
		t.Error("прежний тул settlement_list всё ещё зарегистрирован")
	}
}

// TestDivisionListToolPassesAccessFromContext: Access, положенный
// RequireAPIToken в контекст запроса, доходит до сценария как есть (не
// захардкожен на AccessFull).
func TestDivisionListToolPassesAccessFromContext(t *testing.T) {
	svc := &fakeDivisions{}

	ctx := context.WithValue(context.Background(), accessCtxKey, models.AccessPublic)
	req := mcp.CallToolRequest{}

	if _, err := divisionListHandler(svc)(ctx, req); err != nil {
		t.Fatal(err)
	}

	if svc.gotListAccess != models.AccessPublic {
		t.Fatalf("gotListAccess = %v, ожидался AccessPublic", svc.gotListAccess)
	}
}

// TestDivisionSearchToolPassesAccessFromContext: аналогично для поиска.
func TestDivisionSearchToolPassesAccessFromContext(t *testing.T) {
	svc := &fakeDivisions{}

	ctx := context.WithValue(context.Background(), accessCtxKey, models.AccessPublic)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"q": "давы"}

	if _, err := divisionSearchHandler(svc)(ctx, req); err != nil {
		t.Fatal(err)
	}

	if svc.gotSearchAccess != models.AccessPublic {
		t.Fatalf("gotSearchAccess = %v, ожидался AccessPublic", svc.gotSearchAccess)
	}
}
