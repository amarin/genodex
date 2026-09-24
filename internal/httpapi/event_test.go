package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakeEvents struct {
	list []models.Event
	err  error

	gotQuery  models.EventQuery
	gotSearch models.SearchQuery
	search    []models.Event

	getE      models.Event
	gotIDs    []models.ID
	created   models.Event
	gotCreate models.Event
	updated   models.Event
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

func TestEventListReturnsRecords(t *testing.T) {
	svc := &fakeEvents{list: []models.Event{{ID: "E-1", Type: models.EventTypeBirth}}}

	rec := get(t, NewHandler(Deps{Events: svc, DocsFS: fstest.MapFS{}}), "/api/events")
	requireStatus(t, rec, 200)

	if !strings.Contains(rec.Body.String(), `"type":"birth"`) {
		t.Fatalf("body = %s", rec.Body)
	}
}

func TestEventListPassesPersonIDFilter(t *testing.T) {
	svc := &fakeEvents{}

	rec := get(t, NewHandler(Deps{Events: svc, DocsFS: fstest.MapFS{}}), "/api/events?person_id=I-1")
	requireStatus(t, rec, 200)

	if svc.gotQuery.PersonID == nil || *svc.gotQuery.PersonID != "I-1" {
		t.Fatalf("gotQuery.PersonID = %v", svc.gotQuery.PersonID)
	}
}

func TestEventGetNotFound(t *testing.T) {
	svc := &fakeEvents{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{Events: svc, DocsFS: fstest.MapFS{}}), "/api/events/E-1")
	requireStatus(t, rec, 404)
}

// TestEventSearchPassesQuery: /api/events/search — существует, в отличие от
// relations/residences (Event ЕСТЬ поисковый индекс, docs/data-model/
// entity-write.md §3.8).
func TestEventSearchPassesQuery(t *testing.T) {
	svc := &fakeEvents{}

	rec := get(t, NewHandler(Deps{Events: svc, DocsFS: fstest.MapFS{}}), "/api/events/search?q=Дав")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "Дав" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
