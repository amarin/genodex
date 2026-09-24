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

// TestPersonUpdateToolOmittedPrivateKeepsCurrent — private отсутствует в
// вызове: текущее значение сохраняется, не сбрасывается в false (см.
// сообщение коммита f6a4f48 — исходный риск регресса, который весь этот
// проход и исправляет: AI-ассистент, вызывающий person_update без private,
// не должен снимать приватность записи).
func TestPersonUpdateToolOmittedPrivateKeepsCurrent(t *testing.T) {
	svc := &fakePeople{getP: models.Person{ID: "I-1", Private: true}}

	res := callPersonTool(t, personUpdateHandler(svc), map[string]any{
		"id": "I-1", "gender": "female",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if !svc.updated.Private {
		t.Fatalf("updated.Private = %v, ожидалось сохранение текущего значения true", svc.updated.Private)
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
