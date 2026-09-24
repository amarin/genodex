package list_families

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	list      []*models.Family
	page      models.Page
	citations map[models.ID]*models.Citation
}

func window(list []*models.Family, page models.Page) []*models.Family {
	page = page.Normalized()
	if page.Offset >= len(list) {
		return nil
	}

	return list[page.Offset:min(page.Offset+page.Limit, len(list))]
}

func (f *fakeRepo) ListFamilies(_ context.Context, _ models.Access, page models.Page) ([]*models.Family, error) {
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

func TestListFamiliesReturnsRecords(t *testing.T) {
	repo := &fakeRepo{list: []*models.Family{{ID: "F-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "Ивановы"}}}

	got, err := New(repo).ListFamilies(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListFamilies: %v", err)
	}

	if len(got) != 1 || got[0].Name != "Ивановы" {
		t.Fatalf("got = %+v", got)
	}
}

func TestListFamiliesRejectsNegativeLimit(t *testing.T) {
	repo := &fakeRepo{}

	_, err := New(repo).ListFamilies(context.Background(), models.AccessFull, models.Page{Limit: -1})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "limit" {
		t.Fatalf("err = %v, want ValidationError on limit", err)
	}
}

// TestListFamiliesHidesFamilyReferencingPrivateCitation: род сам по себе не
// приватен, но ссылается (Sources[i].CitationID) на приватную цитату — для
// вызывающего без полного доступа он исключается из списка.
func TestListFamiliesHidesFamilyReferencingPrivateCitation(t *testing.T) {
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		list: []*models.Family{
			{ID: "F-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "Ивановы", Sources: []models.SourceLink{{CitationID: citation}}},
			{ID: "F-01ARZ3NDEKTSV4RRFFQ69G5FA2", Name: "Петровы"},
		},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	got, err := New(repo).ListFamilies(context.Background(), models.AccessPublic, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListFamilies: %v", err)
	}

	if len(got) != 1 || got[0].Name != "Петровы" {
		t.Fatalf("got = %+v, want only the family without a private citation", got)
	}
}

// TestListFamiliesShowsFamilyReferencingPrivateCitationWithFullAccess: тот
// же набор данных, но для вызывающего с полным доступом — оба рода видимы.
func TestListFamiliesShowsFamilyReferencingPrivateCitationWithFullAccess(t *testing.T) {
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		list: []*models.Family{
			{ID: "F-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "Ивановы", Sources: []models.SourceLink{{CitationID: citation}}},
			{ID: "F-01ARZ3NDEKTSV4RRFFQ69G5FA2", Name: "Петровы"},
		},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	got, err := New(repo).ListFamilies(context.Background(), models.AccessFull, models.Page{Limit: 10})
	if err != nil {
		t.Fatalf("ListFamilies: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("got = %+v, want both families with full access", got)
	}
}

// TestListFamiliesPagesWithoutDuplicatesWhenHitHiddenByCitation: из двух
// родов первый скрыт приватной цитатой — offset=0/limit=1 дважды подряд не
// должен ни дублировать, ни терять второй (видимый) род (по образцу
// list_relations/scenario_test.go:TestListRelationsPagesWithoutDuplicatesWhenHitHiddenByCitation).
func TestListFamiliesPagesWithoutDuplicatesWhenHitHiddenByCitation(t *testing.T) {
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		list: []*models.Family{
			{ID: "F-01ARZ3NDEKTSV4RRFFQ69G5FA1", Sources: []models.SourceLink{{CitationID: citation}}},
			{ID: "F-01ARZ3NDEKTSV4RRFFQ69G5FA2"},
		},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	page1, err := New(repo).ListFamilies(context.Background(), models.AccessPublic, models.Page{Limit: 1, Offset: 0})
	if err != nil {
		t.Fatalf("ListFamilies (page1): %v", err)
	}

	if len(page1) != 1 || page1[0].ID != "F-01ARZ3NDEKTSV4RRFFQ69G5FA2" {
		t.Fatalf("page1 = %+v, want [F2]", page1)
	}

	page2, err := New(repo).ListFamilies(context.Background(), models.AccessPublic, models.Page{Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("ListFamilies (page2): %v", err)
	}

	if len(page2) != 0 {
		t.Fatalf("page2 = %+v, want empty (единственная видимая запись уже была на page1)", page2)
	}
}
