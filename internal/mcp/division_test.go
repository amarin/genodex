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
}

func (f *fakeDivisions) ListDivisions(_ context.Context, q models.DivisionQuery) ([]models.AdministrativeDivision, error) {
	f.got = q
	f.call++

	return f.list, f.err
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

// TestNewServerRegistersDivisionListOnly: тул зарегистрирован под новым именем,
// прежнего settlement_list нет.
func TestNewServerRegistersDivisionListOnly(t *testing.T) {
	tools := NewServer(&fakeDivisions{}).ListTools()

	if _, ok := tools["division_list"]; !ok {
		t.Errorf("тул division_list не зарегистрирован: %v", tools)
	}

	if _, ok := tools["settlement_list"]; ok {
		t.Error("прежний тул settlement_list всё ещё зарегистрирован")
	}
}
