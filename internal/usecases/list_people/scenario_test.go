package list_people

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	list      []*models.Person
	page      models.Page
	citations map[models.ID]*models.Citation
}

func window(list []*models.Person, page models.Page) []*models.Person {
	page = page.Normalized()
	if page.Offset >= len(list) {
		return nil
	}

	return list[page.Offset:min(page.Offset+page.Limit, len(list))]
}

func (f *fakeRepo) ListPeople(_ context.Context, _ models.Access, page models.Page) ([]*models.Person, error) {
	f.page = page

	return window(f.list, page), nil
}

func (f *fakeRepo) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return c, nil
}

func TestListPeopleReturnsRecords(t *testing.T) {
	repo := &fakeRepo{list: []*models.Person{{ID: "I-01ARZ3NDEKTSV4RRFFQ69G5FA1"}}}

	got, err := New(repo).ListPeople(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListPeople: %v", err)
	}

	if len(got) != 1 || got[0].ID != "I-01ARZ3NDEKTSV4RRFFQ69G5FA1" {
		t.Fatalf("got = %+v", got)
	}
}

func TestListPeopleRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListPeople(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}

// TestListPeopleHidesPersonReferencingPrivateCitation: персона сама по себе
// не приватна, но ссылается (Sources[i].CitationID) на приватную цитату —
// для вызывающего без полного доступа она исключается из списка.
func TestListPeopleHidesPersonReferencingPrivateCitation(t *testing.T) {
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		list: []*models.Person{
			{ID: "I-01ARZ3NDEKTSV4RRFFQ69G5FA1", Sources: []models.SourceLink{{CitationID: citation}}},
			{ID: "I-01ARZ3NDEKTSV4RRFFQ69G5FA2"},
		},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	got, err := New(repo).ListPeople(context.Background(), models.AccessPublic, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListPeople: %v", err)
	}

	if len(got) != 1 || got[0].ID != "I-01ARZ3NDEKTSV4RRFFQ69G5FA2" {
		t.Fatalf("got = %+v, want only the person without a private citation", got)
	}
}

// TestListPeopleShowsPersonReferencingPrivateCitationWithFullAccess: тот же
// набор данных, но для вызывающего с полным доступом — обе персоны видимы.
func TestListPeopleShowsPersonReferencingPrivateCitationWithFullAccess(t *testing.T) {
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		list: []*models.Person{
			{ID: "I-01ARZ3NDEKTSV4RRFFQ69G5FA1", Sources: []models.SourceLink{{CitationID: citation}}},
			{ID: "I-01ARZ3NDEKTSV4RRFFQ69G5FA2"},
		},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	got, err := New(repo).ListPeople(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListPeople: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("got = %+v, want both people with full access", got)
	}
}

// TestListPeoplePagesWithoutDuplicatesWhenHitHiddenByCitation: из двух
// персон первая скрыта приватной цитатой — offset=0/limit=1 дважды подряд не
// должен ни дублировать, ни терять вторую (видимую) персону (по образцу
// list_relations/scenario_test.go:TestListRelationsPagesWithoutDuplicatesWhenHitHiddenByCitation).
func TestListPeoplePagesWithoutDuplicatesWhenHitHiddenByCitation(t *testing.T) {
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		list: []*models.Person{
			{ID: "I-01ARZ3NDEKTSV4RRFFQ69G5FA1", Sources: []models.SourceLink{{CitationID: citation}}},
			{ID: "I-01ARZ3NDEKTSV4RRFFQ69G5FA2"},
		},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	page1, err := New(repo).ListPeople(context.Background(), models.AccessPublic, models.Page{Limit: 1, Offset: 0})
	if err != nil {
		t.Fatalf("ListPeople (page1): %v", err)
	}

	if len(page1) != 1 || page1[0].ID != "I-01ARZ3NDEKTSV4RRFFQ69G5FA2" {
		t.Fatalf("page1 = %+v, want [I2]", page1)
	}

	page2, err := New(repo).ListPeople(context.Background(), models.AccessPublic, models.Page{Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("ListPeople (page2): %v", err)
	}

	if len(page2) != 0 {
		t.Fatalf("page2 = %+v, want empty (единственная видимая запись уже была на page1)", page2)
	}
}
