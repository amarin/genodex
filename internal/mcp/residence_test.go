package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakeResidences struct {
	list []models.Residence
	err  error

	gotQuery models.ResidenceQuery

	getR      models.Residence
	created   models.Residence
	gotCreate models.Residence
	updated   models.Residence
	gotIDs    []models.ID
	deleteErr error
}

func (f *fakeResidences) ListResidences(_ context.Context, _ models.Access, q models.ResidenceQuery) ([]models.Residence, error) {
	f.gotQuery = q

	return f.list, f.err
}

func (f *fakeResidences) GetResidence(_ context.Context, _ models.Access, id models.ID) (models.Residence, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Residence{}, f.err
	}

	return f.getR, nil
}

func (f *fakeResidences) CreateResidence(_ context.Context, r models.Residence) (models.Residence, error) {
	f.gotCreate = r
	if f.err != nil {
		return models.Residence{}, f.err
	}

	return f.created, nil
}

func (f *fakeResidences) UpdateResidence(_ context.Context, r models.Residence) error {
	f.updated = r

	return f.err
}

func (f *fakeResidences) DeleteResidence(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callResidenceTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestResidenceListToolPassesFilters(t *testing.T) {
	svc := &fakeResidences{}

	res := callResidenceTool(t, residenceListHandler(svc), map[string]any{"person_id": "I-1", "place_id": "AD-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotQuery.PersonID == nil || *svc.gotQuery.PersonID != "I-1" {
		t.Fatalf("gotQuery.PersonID = %v", svc.gotQuery.PersonID)
	}
	if svc.gotQuery.PlaceID == nil || *svc.gotQuery.PlaceID != "AD-1" {
		t.Fatalf("gotQuery.PlaceID = %v", svc.gotQuery.PlaceID)
	}
}

func TestResidenceGetToolContract(t *testing.T) {
	svc := &fakeResidences{getR: models.Residence{ID: "RS-1", Note: "изба"}}

	res := callResidenceTool(t, residenceGetHandler(svc), map[string]any{"id": "RS-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "RS-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

func TestResidenceCreateToolPassesFields(t *testing.T) {
	svc := &fakeResidences{created: models.Residence{ID: "RS-new", PersonID: "I-1", PlaceID: "AD-1"}}

	res := callResidenceTool(t, residenceCreateHandler(svc), map[string]any{
		"person_id": "I-1", "place_id": "AD-1", "note": "изба",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.PersonID != "I-1" || svc.gotCreate.PlaceID != "AD-1" || svc.gotCreate.Note != "изба" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestResidenceUpdateToolCoreFieldsAlwaysReplaced: person_id/place_id —
// REQUIRED, всегда заменяются безусловно.
func TestResidenceUpdateToolCoreFieldsAlwaysReplaced(t *testing.T) {
	svc := &fakeResidences{getR: models.Residence{ID: "RS-1", PersonID: "I-1", PlaceID: "AD-1"}}

	res := callResidenceTool(t, residenceUpdateHandler(svc), map[string]any{
		"id": "RS-1", "person_id": "I-2", "place_id": "AD-2",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.PersonID != "I-2" || svc.updated.PlaceID != "AD-2" {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestResidenceUpdateToolOmittedNoteKeepsCurrent(t *testing.T) {
	svc := &fakeResidences{getR: models.Residence{ID: "RS-1", PersonID: "I-1", PlaceID: "AD-1", Note: "изба"}}

	res := callResidenceTool(t, residenceUpdateHandler(svc), map[string]any{
		"id": "RS-1", "person_id": "I-1", "place_id": "AD-1",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Note != "изба" {
		t.Fatalf("updated.Note = %q, ожидалось сохранение текущего", svc.updated.Note)
	}
}

// TestResidenceUpdateToolOmittedSinceKeepsCurrent/NullSinceClears —
// одиночное объектное поле, presence-only guard (см. relation_test.go).
func TestResidenceUpdateToolOmittedSinceKeepsCurrent(t *testing.T) {
	svc := &fakeResidences{getR: models.Residence{
		ID: "RS-1", PersonID: "I-1", PlaceID: "AD-1",
		Since: &models.FactDate{Year: 1900, Precision: models.PrecisionYear},
	}}

	res := callResidenceTool(t, residenceUpdateHandler(svc), map[string]any{
		"id": "RS-1", "person_id": "I-1", "place_id": "AD-1",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Since == nil || svc.updated.Since.Year != 1900 {
		t.Fatalf("updated.Since = %v, ожидалось сохранение текущего", svc.updated.Since)
	}
}

func TestResidenceUpdateToolNullSinceClears(t *testing.T) {
	svc := &fakeResidences{getR: models.Residence{
		ID: "RS-1", PersonID: "I-1", PlaceID: "AD-1",
		Since: &models.FactDate{Year: 1900, Precision: models.PrecisionYear},
	}}

	res := callResidenceTool(t, residenceUpdateHandler(svc), map[string]any{
		"id": "RS-1", "person_id": "I-1", "place_id": "AD-1", "since": nil,
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Since != nil {
		t.Fatalf("updated.Since = %v, ожидался nil", svc.updated.Since)
	}
}

func TestResidenceDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeResidences{deleteErr: &models.InUseError{Type: models.TypeResidence, ID: "RS-1"}}

	res := callResidenceTool(t, residenceDeleteHandler(svc), map[string]any{"id": "RS-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

func TestNewServerRegistersResidenceTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Residences: &fakeResidences{}}).ListTools()

	for _, name := range []string{"residence_list", "residence_get", "residence_create", "residence_update", "residence_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}

	if _, ok := tools["residence_search"]; ok {
		t.Errorf("residence_search не должен существовать")
	}
}
