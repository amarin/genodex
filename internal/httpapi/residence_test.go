package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeResidences struct {
	list []models.Residence
	err  error

	gotQuery models.ResidenceQuery

	getR      models.Residence
	gotIDs    []models.ID
	created   models.Residence
	gotCreate models.Residence
	updated   models.Residence
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

func TestResidenceListReturnsRecords(t *testing.T) {
	svc := &fakeResidences{list: []models.Residence{{ID: "RS-1", PersonID: "I-1", PlaceID: "AD-1"}}}

	rec := get(t, NewHandler(Deps{Residences: svc, DocsFS: fstest.MapFS{}}), "/api/residences")
	requireStatus(t, rec, 200)

	if !strings.Contains(rec.Body.String(), `"person_id":"I-1"`) {
		t.Fatalf("body = %s", rec.Body)
	}
}

// TestResidenceListPassesBothFilters: ?person_id=&place_id= оба разбираются.
func TestResidenceListPassesBothFilters(t *testing.T) {
	svc := &fakeResidences{}

	rec := get(t, NewHandler(Deps{Residences: svc, DocsFS: fstest.MapFS{}}), "/api/residences?person_id=I-1&place_id=AD-1")
	requireStatus(t, rec, 200)

	if svc.gotQuery.PersonID == nil || *svc.gotQuery.PersonID != "I-1" {
		t.Fatalf("gotQuery.PersonID = %v", svc.gotQuery.PersonID)
	}
	if svc.gotQuery.PlaceID == nil || *svc.gotQuery.PlaceID != "AD-1" {
		t.Fatalf("gotQuery.PlaceID = %v", svc.gotQuery.PlaceID)
	}
}

func TestResidenceGetNotFound(t *testing.T) {
	svc := &fakeResidences{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Residences: svc, DocsFS: fstest.MapFS{}}), "/api/residences/RS-1")
	requireStatus(t, rec, 404)
}

// TestResidenceNoSearchRoute: search_residences намеренно не заводится —
// см. TestRelationNoSearchRoute.
func TestResidenceNoSearchRoute(t *testing.T) {
	svc := &fakeResidences{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Residences: svc, DocsFS: fstest.MapFS{}}), "/api/residences/search")
	requireStatus(t, rec, 404)

	if len(svc.gotIDs) != 1 || svc.gotIDs[0] != "search" {
		t.Fatalf("gotIDs = %v, ожидался вызов GetResidence с id=\"search\"", svc.gotIDs)
	}
}
