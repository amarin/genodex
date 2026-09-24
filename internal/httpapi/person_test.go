package httpapi

import (
	"context"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

type fakePeople struct {
	list []models.Person
	err  error
	page models.Page

	getP      models.Person
	gotIDs    []models.ID
	created   models.Person
	gotCreate models.Person
	updated   models.Person
	deleteErr error

	search    []models.Person
	gotSearch models.SearchQuery
}

func (f *fakePeople) ListPeople(_ context.Context, _ models.Access, page models.Page) ([]models.Person, error) {
	f.page = page

	return f.list, f.err
}

func (f *fakePeople) SearchPeople(_ context.Context, _ models.Access, q models.SearchQuery) ([]models.Person, error) {
	f.gotSearch = q

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

func TestPersonListReturnsRecords(t *testing.T) {
	svc := &fakePeople{list: []models.Person{{ID: "I-1", Gender: models.PersonGenderFemale}}}

	rec := get(t, NewHandler(Deps{People: svc, DocsFS: fstest.MapFS{}}), "/api/people")
	requireStatus(t, rec, 200)

	want := `[{"id":"I-1","gender":"female","names":[],"estates":[],"titles":[],"nicknames":[],"notes":[],"sources":[],"private":false}]`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Fatalf("body = %s, want %s", got, want)
	}
}

func TestPersonGetNotFound(t *testing.T) {
	svc := &fakePeople{err: models.ErrNotFound}

	rec := get(t, NewHandler(Deps{People: svc, DocsFS: fstest.MapFS{}}), "/api/people/I-1")
	requireStatus(t, rec, 404)
}

func TestPersonSearchPassesQuery(t *testing.T) {
	svc := &fakePeople{}

	rec := get(t, NewHandler(Deps{People: svc, DocsFS: fstest.MapFS{}}), "/api/people/search?q=Ив")
	requireStatus(t, rec, 200)

	if svc.gotSearch.Text != "Ив" {
		t.Fatalf("gotSearch.Text = %q", svc.gotSearch.Text)
	}
}
