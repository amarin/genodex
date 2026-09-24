package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

func callDivisionWrite(t *testing.T, handler server.ToolHandlerFunc, svc *fakeDivisions, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

// TestDivisionGetToolContract: чтение по id возвращает JSON контракта.
func TestDivisionGetToolContract(t *testing.T) {
	svc := &fakeDivisions{getDiv: models.AdministrativeDivision{
		ID: "ad-root", Name: "Московская", Type: models.AdminDivisionGuberniya,
	}}

	res := callDivisionWrite(t, divisionGetHandler(svc), svc, map[string]any{"id": "ad-root"})

	want := `{"id":"ad-root","name":"Московская","type":"guberniya","parent_id":null,"sources":[]}`
	if res.IsError || resultText(t, res) != want {
		t.Fatalf("isError=%v text=%s, want %s", res.IsError, resultText(t, res), want)
	}
	if len(svc.gotIDs) != 1 || svc.gotIDs[0] != "ad-root" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

// TestDivisionGetToolErrorsAreToolErrors: отсутствующая единица и неверный формат
// id — ошибка тула, а не результата.
func TestDivisionGetToolErrorsAreToolErrors(t *testing.T) {
	for name, err := range map[string]error{
		"не найдено":      models.ErrNotFound,
		"неверный формат": &models.ValidationError{Field: "id"},
		"сбой хранилища":  errors.New("хранилище недоступно"),
	} {
		res := callDivisionWrite(t, divisionGetHandler(&fakeDivisions{err: err}), &fakeDivisions{err: err}, map[string]any{"id": "ad-root"})

		if !res.IsError || resultText(t, res) == "" {
			t.Errorf("%s: isError=%v text=%q; ожидалась ошибка тула", name, res.IsError, resultText(t, res))
		}
	}
}

// TestDivisionCreateToolPassesModel: аргументы name/type/parent_id доходят до
// сценария моделью; ответ сценария возвращается как JSON.
func TestDivisionCreateToolPassesModel(t *testing.T) {
	svc := &fakeDivisions{created: models.AdministrativeDivision{
		ID: "ad-new", Name: "Давыдово", Type: models.AdminDivisionSelo,
	}}

	res := callDivisionWrite(t, divisionCreateHandler(svc), svc,
		map[string]any{"name": "Давыдово", "type": "selo", "parent_id": "ad-root"})

	want := `{"id":"ad-new","name":"Давыдово","type":"selo","parent_id":null,"sources":[]}`
	if res.IsError || resultText(t, res) != want {
		t.Fatalf("isError=%v text=%s, want %s", res.IsError, resultText(t, res), want)
	}

	got := svc.gotCreate
	if got.Name != "Давыдово" || got.Type != models.AdminDivisionSelo {
		t.Fatalf("gotCreate = %+v", got)
	}
	if got.ParentID == nil || *got.ParentID != "ad-root" {
		t.Fatalf("gotCreate.ParentID = %v, ожидался ad-root", got.ParentID)
	}
}

// TestDivisionCreateToolEmptyParentMeansRoot: пустой parent_id — корень.
func TestDivisionCreateToolEmptyParentMeansRoot(t *testing.T) {
	svc := &fakeDivisions{created: models.AdministrativeDivision{}}

	callDivisionWrite(t, divisionCreateHandler(svc), svc, map[string]any{"name": "Московская", "type": "guberniya", "parent_id": ""})

	if svc.gotCreate.ParentID != nil {
		t.Fatalf("gotCreate.ParentID = %v, ожидался корень", svc.gotCreate.ParentID)
	}
}

// TestDivisionCreateToolErrorIsToolError: ошибка проверки в сценарии — ошибка тула.
func TestDivisionCreateToolErrorIsToolError(t *testing.T) {
	ve := &models.ValidationError{Field: "name", Reason: "пустое имя"}
	res := callDivisionWrite(t, divisionCreateHandler(&fakeDivisions{err: ve}), &fakeDivisions{err: ve},
		map[string]any{"name": "", "type": "selo"})

	if !res.IsError || !strings.Contains(resultText(t, res), ve.Error()) {
		t.Fatalf("isError=%v text=%q", res.IsError, resultText(t, res))
	}
}

// TestDivisionUpdateToolMergesFields: обновление заменяет name/type/parent_id;
// прочие поля текущей версии (например parent_id-корень) не затрагиваются.
func TestDivisionUpdateToolMergesFields(t *testing.T) {
	svc := &fakeDivisions{getDiv: models.AdministrativeDivision{
		ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo,
		ParentID: func() *models.ID { v := models.ID("ad-root"); return &v }(),
	}}

	res := callDivisionWrite(t, divisionUpdateHandler(svc), svc,
		map[string]any{"id": "ad-1", "name": "Давыдова", "type": "selo", "parent_id": ""})

	if res.IsError {
		t.Fatalf("неожиданная ошибка тула: %s", resultText(t, res))
	}

	got := svc.updated
	if got.ID != "ad-1" || got.Name != "Давыдова" || got.Type != models.AdminDivisionSelo {
		t.Fatalf("updated = %+v", got)
	}
	if got.ParentID != nil {
		t.Fatalf("updated.ParentID = %v, пустой parent_id = корень", got.ParentID)
	}
}

// TestDivisionUpdateToolOmittedParentIDKeepsCurrent: parent_id отсутствует
// в вызове — текущий родитель сохраняется, в отличие от явно пустой строки
// (см. TestDivisionUpdateToolMergesFields), которая делает единицу корнем.
func TestDivisionUpdateToolOmittedParentIDKeepsCurrent(t *testing.T) {
	parent := models.ID("ad-root")
	svc := &fakeDivisions{getDiv: models.AdministrativeDivision{
		ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo, ParentID: &parent,
	}}

	res := callDivisionWrite(t, divisionUpdateHandler(svc), svc,
		map[string]any{"id": "ad-1", "name": "Давыдова", "type": "selo"})

	if res.IsError {
		t.Fatalf("неожиданная ошибка тула: %s", resultText(t, res))
	}

	if svc.updated.ParentID == nil || *svc.updated.ParentID != "ad-root" {
		t.Fatalf("updated.ParentID = %v, ожидалось сохранение текущего родителя", svc.updated.ParentID)
	}
}

// TestDivisionUpdateToolSourcesPassed: непустой sources в аргументах доходит
// до сценария моделью (Fix 1 финального ревью подпроекта 5).
func TestDivisionUpdateToolSourcesPassed(t *testing.T) {
	svc := &fakeDivisions{getDiv: models.AdministrativeDivision{
		ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo,
	}}

	res := callDivisionWrite(t, divisionUpdateHandler(svc), svc, map[string]any{
		"id": "ad-1", "name": "Давыдово", "type": "selo",
		"sources": []any{map[string]any{"citation_id": "C-1", "role": "рождение", "reliability": "primary"}},
	})

	if res.IsError {
		t.Fatalf("неожиданная ошибка тула: %s", resultText(t, res))
	}

	got := svc.updated.Sources
	if len(got) != 1 || got[0].CitationID != "C-1" || got[0].Role != "рождение" || got[0].Reliability != models.ReliabilityPrimary {
		t.Fatalf("updated.Sources = %+v", got)
	}
}

// TestDivisionUpdateToolOmittedSourcesKeepsCurrent: отсутствие sources в
// аргументах сохраняет текущие источники записи (Fix 1 финального ревью
// подпроекта 5: раньше отсутствие ключа сценарий трактовал как явную очистку).
func TestDivisionUpdateToolOmittedSourcesKeepsCurrent(t *testing.T) {
	svc := &fakeDivisions{getDiv: models.AdministrativeDivision{
		ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo,
		Sources: []models.SourceLink{{CitationID: "C-1", Role: "рождение"}},
	}}

	res := callDivisionWrite(t, divisionUpdateHandler(svc), svc, map[string]any{
		"id": "ad-1", "name": "Давыдова", "type": "selo",
	})

	if res.IsError {
		t.Fatalf("неожиданная ошибка тула: %s", resultText(t, res))
	}

	got := svc.updated.Sources
	if len(got) != 1 || got[0].CitationID != "C-1" {
		t.Fatalf("updated.Sources = %+v, ожидалось сохранение текущих источников", got)
	}
}

// TestDivisionUpdateToolEmptySourcesClears: явный пустой массив sources
// очищает текущие источники (в отличие от отсутствия ключа).
func TestDivisionUpdateToolEmptySourcesClears(t *testing.T) {
	svc := &fakeDivisions{getDiv: models.AdministrativeDivision{
		ID: "ad-1", Name: "Давыдово", Type: models.AdminDivisionSelo,
		Sources: []models.SourceLink{{CitationID: "C-1", Role: "рождение"}},
	}}

	res := callDivisionWrite(t, divisionUpdateHandler(svc), svc, map[string]any{
		"id": "ad-1", "name": "Давыдова", "type": "selo", "sources": []any{},
	})

	if res.IsError {
		t.Fatalf("неожиданная ошибка тула: %s", resultText(t, res))
	}

	if got := svc.updated.Sources; len(got) != 0 {
		t.Fatalf("updated.Sources = %+v, ожидалась очистка явным пустым массивом", got)
	}
}

// TestDivisionUpdateToolNotFoundIsError: отсутствующая единица — ошибка тула.
func TestDivisionUpdateToolNotFoundIsError(t *testing.T) {
	res := callDivisionWrite(t, divisionUpdateHandler(&fakeDivisions{err: models.ErrNotFound}),
		&fakeDivisions{err: models.ErrNotFound}, map[string]any{"id": "ad-1", "name": "Давыдова"})

	if !res.IsError {
		t.Fatalf("text = %q; ожидалась ошибка тула", resultText(t, res))
	}
}

// TestDivisionDeleteToolContract: удаление возвращает текст подтверждения.
func TestDivisionDeleteToolContract(t *testing.T) {
	svc := &fakeDivisions{}

	res := callDivisionWrite(t, divisionDeleteHandler(svc), svc, map[string]any{"id": "ad-1"})

	if res.IsError {
		t.Fatalf("неожиданная ошибка тула: %s", resultText(t, res))
	}
	if text := resultText(t, res); !strings.Contains(text, `"ad-1"`) || !strings.Contains(text, "удалено") {
		t.Fatalf("text = %q, ожидалось подтверждение удаления ad-1", text)
	}
	if len(svc.gotIDs) != 1 || svc.gotIDs[0] != "ad-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

// TestDivisionDeleteToolInUseIsError: занятая единица — ошибка тула с текстом ошибки.
func TestDivisionDeleteToolInUseIsError(t *testing.T) {
	iu := &models.InUseError{Type: "administrative_division", ID: "ad-1"}
	res := callDivisionWrite(t, divisionDeleteHandler(&fakeDivisions{deleteErr: iu}),
		&fakeDivisions{deleteErr: iu}, map[string]any{"id": "ad-1"})

	if !res.IsError || !strings.Contains(resultText(t, res), iu.Error()) {
		t.Fatalf("isError=%v text=%q", res.IsError, resultText(t, res))
	}
}

// TestNewServerRegistersDivisionWriteTools: тулы записи зарегистрированы.
func TestNewServerRegistersDivisionWriteTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}}).ListTools()

	for _, name := range []string{"division_get", "division_create", "division_update", "division_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
