package mcp

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
)

type fakeRelations struct {
	list []models.Relation
	err  error

	gotQuery models.RelationQuery

	getR      models.Relation
	created   models.Relation
	gotCreate models.Relation
	updated   models.Relation
	gotIDs    []models.ID
	deleteErr error
}

func (f *fakeRelations) ListRelations(_ context.Context, _ models.Access, q models.RelationQuery) ([]models.Relation, error) {
	f.gotQuery = q

	return f.list, f.err
}

func (f *fakeRelations) GetRelation(_ context.Context, _ models.Access, id models.ID) (models.Relation, error) {
	f.gotIDs = append(f.gotIDs, id)
	if f.err != nil {
		return models.Relation{}, f.err
	}

	return f.getR, nil
}

func (f *fakeRelations) CreateRelation(_ context.Context, r models.Relation) (models.Relation, error) {
	f.gotCreate = r
	if f.err != nil {
		return models.Relation{}, f.err
	}

	return f.created, nil
}

func (f *fakeRelations) UpdateRelation(_ context.Context, r models.Relation) error {
	f.updated = r

	return f.err
}

func (f *fakeRelations) DeleteRelation(_ context.Context, id models.ID) error {
	f.gotIDs = append(f.gotIDs, id)

	return f.deleteErr
}

func callRelationTool(t *testing.T, handler server.ToolHandlerFunc, args map[string]any) *mcp.CallToolResult {
	t.Helper()

	req := mcp.CallToolRequest{}
	req.Params.Arguments = args

	res, err := handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}

	return res
}

func TestRelationListToolPassesPersonIDFilter(t *testing.T) {
	svc := &fakeRelations{}

	res := callRelationTool(t, relationListHandler(svc), map[string]any{"person_id": "I-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotQuery.PersonID == nil || *svc.gotQuery.PersonID != "I-1" {
		t.Fatalf("gotQuery.PersonID = %v", svc.gotQuery.PersonID)
	}
}

func TestRelationGetToolContract(t *testing.T) {
	svc := &fakeRelations{getR: models.Relation{ID: "RL-1", Kind: models.RelationKindBlood}}

	res := callRelationTool(t, relationGetHandler(svc), map[string]any{"id": "RL-1"})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotIDs[0] != "RL-1" {
		t.Fatalf("gotIDs = %v", svc.gotIDs)
	}
}

func TestRelationCreateToolPassesFields(t *testing.T) {
	svc := &fakeRelations{created: models.Relation{ID: "RL-new", Kind: models.RelationKindBlood, PersonA: "I-1", PersonB: "I-2"}}

	res := callRelationTool(t, relationCreateHandler(svc), map[string]any{
		"kind": "blood", "person_a": "I-1", "person_b": "I-2",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.gotCreate.PersonA != "I-1" || svc.gotCreate.PersonB != "I-2" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestRelationUpdateToolCoreFieldsAlwaysReplaced: kind/person_a/person_b —
// REQUIRED, всегда заменяются безусловно, даже когда значение совпадает со
// старым — сам факт "прошли через _update" достаточен (mirroring citation's
// source_id).
func TestRelationUpdateToolCoreFieldsAlwaysReplaced(t *testing.T) {
	svc := &fakeRelations{getR: models.Relation{ID: "RL-1", Kind: models.RelationKindBlood, PersonA: "I-1", PersonB: "I-2"}}

	res := callRelationTool(t, relationUpdateHandler(svc), map[string]any{
		"id": "RL-1", "kind": "marriage", "person_a": "I-3", "person_b": "I-4",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Kind != models.RelationKindMarriage || svc.updated.PersonA != "I-3" || svc.updated.PersonB != "I-4" {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

// TestRelationUpdateToolOmittedRelTypeKeepsCurrent: rel_type — обычный
// preserve-on-omit скаляр (в отличие от kind/person_a/person_b).
func TestRelationUpdateToolOmittedRelTypeKeepsCurrent(t *testing.T) {
	svc := &fakeRelations{getR: models.Relation{
		ID: "RL-1", Kind: models.RelationKindAssociate, RelType: models.RelationTypeGodparent, PersonA: "I-1", PersonB: "I-2",
	}}

	res := callRelationTool(t, relationUpdateHandler(svc), map[string]any{
		"id": "RL-1", "kind": "associate", "person_a": "I-1", "person_b": "I-2",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.RelType != models.RelationTypeGodparent {
		t.Fatalf("updated.RelType = %q, ожидалось сохранение текущего", svc.updated.RelType)
	}
}

// TestRelationUpdateToolOmittedSinceKeepsCurrent: since — одиночное
// объектное поле (*FactDate), presence-only guard: отсутствие ключа
// сохраняет текущее значение.
func TestRelationUpdateToolOmittedSinceKeepsCurrent(t *testing.T) {
	svc := &fakeRelations{getR: models.Relation{
		ID: "RL-1", Kind: models.RelationKindBlood, PersonA: "I-1", PersonB: "I-2",
		Since: &models.FactDate{Year: 1850, Precision: models.PrecisionYear},
	}}

	res := callRelationTool(t, relationUpdateHandler(svc), map[string]any{
		"id": "RL-1", "kind": "blood", "person_a": "I-1", "person_b": "I-2",
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Since == nil || svc.updated.Since.Year != 1850 {
		t.Fatalf("updated.Since = %v, ожидалось сохранение текущего", svc.updated.Since)
	}
}

// TestRelationUpdateToolNullSinceClears: явный null у since очищает поле
// (presence-only guard — {} не проходит валидацию, значит очистка только null'ом).
func TestRelationUpdateToolNullSinceClears(t *testing.T) {
	svc := &fakeRelations{getR: models.Relation{
		ID: "RL-1", Kind: models.RelationKindBlood, PersonA: "I-1", PersonB: "I-2",
		Since: &models.FactDate{Year: 1850, Precision: models.PrecisionYear},
	}}

	res := callRelationTool(t, relationUpdateHandler(svc), map[string]any{
		"id": "RL-1", "kind": "blood", "person_a": "I-1", "person_b": "I-2", "since": nil,
	})

	if res.IsError {
		t.Fatalf("isError=%v text=%s", res.IsError, resultText(t, res))
	}

	if svc.updated.Since != nil {
		t.Fatalf("updated.Since = %v, ожидался nil (явный null очищает поле)", svc.updated.Since)
	}
}

func TestRelationDeleteToolInUseIsError(t *testing.T) {
	svc := &fakeRelations{deleteErr: &models.InUseError{Type: models.TypeRelation, ID: "RL-1"}}

	res := callRelationTool(t, relationDeleteHandler(svc), map[string]any{"id": "RL-1"})

	if !res.IsError {
		t.Fatalf("isError=%v, want true", res.IsError)
	}
}

// TestNewServerRegistersRelationTools: 5 тулов, без relation_search.
func TestNewServerRegistersRelationTools(t *testing.T) {
	tools := NewServer(Deps{Divisions: &fakeDivisions{}, Relations: &fakeRelations{}}).ListTools()

	for _, name := range []string{"relation_list", "relation_get", "relation_create", "relation_update", "relation_delete"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("тул %s не зарегистрирован: %v", name, tools)
		}
	}

	if _, ok := tools["relation_search"]; ok {
		t.Errorf("relation_search не должен существовать (search_relations намеренно не заводится)")
	}
}
