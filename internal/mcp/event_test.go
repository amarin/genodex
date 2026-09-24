package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakeEvents struct {
	list []models.Event
	err  error

	gotQuery  models.EventQuery
	gotSearch models.SearchQuery
	search    []models.Event

	getE      models.Event
	created   models.Event
	gotCreate models.Event
	updated   models.Event
	gotIDs    []models.ID
	deleteErr error
}

func (f *fakeEvents) ListEvents(_ context.Context, _ models.Access, q models.EventQuery) ([]models.Event, error) {
	f.gotQuery = q

	return f.list, f.err
}

func (f *fakeEvents) SearchEvents(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Event, error) {
	f.gotSearch = q

	return f.search, f.err
}

func (f *fakeEvents) GetEvent(_ context.Context, _ models.Access, id models.ID) (models.Event, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Event{}, f.err
	}

	return f.getE, nil
}

func (f *fakeEvents) CreateEvent(_ context.Context, e models.Event) (models.Event, error) {
	f.gotCreate = e
	if f.err != nil {
		return models.Event{}, f.err
	}

	return f.created, nil
}

func (f *fakeEvents) UpdateEvent(_ context.Context, e models.Event) error {
	f.updated = e

	return f.err
}

func (f *fakeEvents) DeleteEvent(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callEventTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestEventListToolPassesPersonIDFilter(t *testing.T) {
	svc := &fakeEvents{}

	res := callEventTool(t, eventListHandler(svc), map[string]any{"person_id": "I-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotQuery.PersonID == nil || *svc.gotQuery.PersonID != "I-1" {
		t.Fatalf("gotQuery.PersonID = %v", svc.gotQuery.PersonID)
	}
}

func TestEventSearchToolPassesQuery(t *testing.T) {
	svc := &fakeEvents{}

	res := callEventTool(t, eventSearchHandler(svc), map[string]any{"q": "село"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotSearch.Text != "село" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}

func TestEventGetToolContract(t *testing.T) {
	svc := &fakeEvents{getE: models.Event{ID: "E-1", Type: models.EventTypeBirth}}

	res := callEventTool(t, eventGetHandler(svc), map[string]any{"id": "E-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "E-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

func TestEventCreateToolPassesFields(t *testing.T) {
	svc := &fakeEvents{created: models.Event{ID: "E-new", Type: models.EventTypeBirth}}

	res := callEventTool(t, eventCreateHandler(svc), map[string]any{
		"type": "birth",
		"place": map[string]any{
			"text": "село Давыдово", "ref": "AD-01ARZ3NDEKTSV4RRFFQ69G5FA9", "type": "administrative_division",
		},
		"participants": []any{
			map[string]any{"person_id": "I-1", "role": "родитель"},
		},
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.Type != models.EventTypeBirth {
		t.Fatalf("gotCreate.Type = %q", svc.gotCreate.Type)
	}
	if svc.gotCreate.Place == nil || svc.gotCreate.Place.Ref != "AD-01ARZ3NDEKTSV4RRFFQ69G5FA9" {
		t.Fatalf("gotCreate.Place = %+v", svc.gotCreate.Place)
	}
	if len(svc.gotCreate.Participants) != 1 || svc.gotCreate.Participants[0].PersonID != "I-1" {
		t.Fatalf("gotCreate.Participants = %+v", svc.gotCreate.Participants)
	}
}

// TestEventUpdateToolTypeAlwaysReplaced: type — REQUIRED, всегда заменяется
// безусловно (как relation_update's kind).
func TestEventUpdateToolTypeAlwaysReplaced(t *testing.T) {
	svc := &fakeEvents{getE: models.Event{ID: "E-1", Type: models.EventTypeBirth}}

	res := callEventTool(t, eventUpdateHandler(svc), map[string]any{"id": "E-1", "type": "death"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Type != models.EventTypeDeath {
		t.Fatalf("updated.Type = %q", svc.updated.Type)
	}
}

// TestEventUpdateToolOmittedPlaceKeepsCurrent: Place — одиночное объектное
// поле (*PlaceRef), самое рискованное в этом подпроекте по истории
// регрессии (fix "null должен снова очищать TextRef/FactDate-поля"):
// отсутствие ключа "place" в вызове сохраняет текущее значение.
func TestEventUpdateToolOmittedPlaceKeepsCurrent(t *testing.T) {
	svc := &fakeEvents{getE: models.Event{
		ID: "E-1", Type: models.EventTypeBirth,
		Place: &models.PlaceRef{Text: "село Давыдово", Ref: "AD-1", Type: models.TypeAdministrativeDivision},
	}}

	res := callEventTool(t, eventUpdateHandler(svc), map[string]any{"id": "E-1", "type": "birth"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Place == nil || svc.updated.Place.Ref != "AD-1" || svc.updated.Place.Text != "село Давыдово" {
		t.Fatalf("updated.Place = %+v, ожидалось сохранение текущего места (omit keeps)", svc.updated.Place)
	}
}

// TestEventUpdateToolNullPlaceClears: явный null у place ОЧИЩАЕТ поле — это
// единственный способ очистить одиночное объектное поле ({} не проходит
// валидацию PlaceRef). Присутствие ключа с raw==nil ("place": nil) должно
// сработать так же, как отсутствие ключа НЕ должно (presence-only guard,
// не raw != nil).
func TestEventUpdateToolNullPlaceClears(t *testing.T) {
	svc := &fakeEvents{getE: models.Event{
		ID: "E-1", Type: models.EventTypeBirth,
		Place: &models.PlaceRef{Text: "село Давыдово", Ref: "AD-1", Type: models.TypeAdministrativeDivision},
	}}

	res := callEventTool(t, eventUpdateHandler(svc), map[string]any{"id": "E-1", "type": "birth", "place": nil})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Place != nil {
		t.Fatalf("updated.Place = %+v, ожидался nil (явный null очищает поле)", svc.updated.Place)
	}
}

// TestEventUpdateToolOmittedParticipantsKeepsCurrent: participants —
// array-of-objects, обычный preserve-on-omit guard (ok && raw != nil), как
// person_update's names.
func TestEventUpdateToolOmittedParticipantsKeepsCurrent(t *testing.T) {
	svc := &fakeEvents{getE: models.Event{
		ID: "E-1", Type: models.EventTypeBirth,
		Participants: []models.EventParticipant{{PersonID: "I-1", Role: "родитель"}},
	}}

	res := callEventTool(t, eventUpdateHandler(svc), map[string]any{"id": "E-1", "type": "birth"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if len(svc.updated.Participants) != 1 || svc.updated.Participants[0].PersonID != "I-1" {
		t.Fatalf("updated.Participants = %+v, ожидалось сохранение текущих участников", svc.updated.Participants)
	}
}

// TestEventUpdateToolEmptyParticipantsClears: явный пустой массив
// очищает список участников.
func TestEventUpdateToolEmptyParticipantsClears(t *testing.T) {
	svc := &fakeEvents{getE: models.Event{
		ID: "E-1", Type: models.EventTypeBirth,
		Participants: []models.EventParticipant{{PersonID: "I-1", Role: "родитель"}},
	}}

	res := callEventTool(t, eventUpdateHandler(svc), map[string]any{
		"id": "E-1", "type": "birth", "participants": []any{},
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if len(svc.updated.Participants) != 0 {
		t.Fatalf("updated.Participants = %+v, ожидался пустой список", svc.updated.Participants)
	}
}

func TestEventUpdateToolOmittedNotesKeepsCurrent(t *testing.T) {
	svc := &fakeEvents{getE: models.Event{ID: "E-1", Type: models.EventTypeBirth, Notes: []models.TextRef{{Text: "заметка"}}}}

	res := callEventTool(t, eventUpdateHandler(svc), map[string]any{"id": "E-1", "type": "birth"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if len(svc.updated.Notes) != 1 || svc.updated.Notes[0].Text != "заметка" {
		t.Fatalf("updated.Notes = %+v, ожидалось сохранение текущих заметок", svc.updated.Notes)
	}
}

func TestEventDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeEvents{deleteErr: &models.InUseError{Type: models.TypeEvent, ID: "E-1"}}

	res := callEventTool(t, eventDeleteHandler(svc), map[string]any{"id": "E-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

// TestNewServerRegistersEventTools: 6 тулов, включая event_search (в
// отличие от relation/residence).
func TestNewServerRegistersEventTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Events: &fakeEvents{}}).ListTools()

	for _, name := range []string{"event_list", "event_search", "event_get", "event_create", "event_update", "event_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}
}
