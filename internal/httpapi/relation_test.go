package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeRelations struct {
	list []models.Relation
	err  error

	gotQuery models.RelationQuery

	getR      models.Relation
	gotIDs    []models.ID
	created   models.Relation
	gotCreate models.Relation
	updated   models.Relation
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

func TestRelationListReturnsRecords(t *testing.T) {
	svc := &fakeRelations{list: []models.Relation{{ID: "RL-1", Kind: models.RelationKindBlood, PersonA: "I-1", PersonB: "I-2"}}}

	rec := get(t, NewHandler(Deps{Relations: svc, DocsFS: fstest.MapFS{}}), "/api/relations")
	requireStatus(t, rec, 200)

	if !strings.Contains(rec.Body.String(), `"person_a":"I-1"`) {
		t.Fatalf("body = %s", rec.Body)
	}
}

// TestRelationListPassesPersonIDFilter: ?person_id= разбирается в
// RelationQuery.PersonID.
func TestRelationListPassesPersonIDFilter(t *testing.T) {
	svc := &fakeRelations{}

	rec := get(t, NewHandler(Deps{Relations: svc, DocsFS: fstest.MapFS{}}), "/api/relations?person_id=I-1")
	requireStatus(t, rec, 200)

	if svc.gotQuery.PersonID == nil || *svc.gotQuery.PersonID != "I-1" {
		t.Fatalf("gotQuery.PersonID = %v", svc.gotQuery.PersonID)
	}
}

func TestRelationListWithoutPersonIDLeavesFilterNil(t *testing.T) {
	svc := &fakeRelations{}

	rec := get(t, NewHandler(Deps{Relations: svc, DocsFS: fstest.MapFS{}}), "/api/relations")
	requireStatus(t, rec, 200)

	if svc.gotQuery.PersonID != nil {
		t.Fatalf("gotQuery.PersonID = %v, want nil", svc.gotQuery.PersonID)
	}
}

func TestRelationGetNotFound(t *testing.T) {
	svc := &fakeRelations{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Relations: svc, DocsFS: fstest.MapFS{}}), "/api/relations/RL-1")
	requireStatus(t, rec, 404)
}

// TestRelationNoSearchRoute: search_relations намеренно не заводится — нет
// /api/relations/search (docs/data-model/entity-write.md §3.8). Запрос по
// такому пути должен попасть в handleRelationGet как id="search" и 404
// (запись "search" не существует), а не в отдельный обработчик поиска.
func TestRelationNoSearchRoute(t *testing.T) {
	svc := &fakeRelations{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Relations: svc, DocsFS: fstest.MapFS{}}), "/api/relations/search")
	requireStatus(t, rec, 404)

	if len(svc.gotIDs) != 1 || svc.gotIDs[0] != "search" {
		t.Fatalf("gotIDs = %v, ожидался вызов GetRelation с id=\"search\" (нет отдельного маршрута /search)", svc.gotIDs)
	}
}
