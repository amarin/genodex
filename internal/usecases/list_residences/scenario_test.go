package list_residences

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	list  []*models.Residence
	err   error
	calls []models.Page
}

func window(list []*models.Residence, page models.Page) []*models.Residence {
	page = page.Normalized()
	if page.Offset >= len(list) {
		return nil
	}

	return list[page.Offset:min(page.Offset+page.Limit, len(list))]
}

func (f *fakeRepo) ListResidences(_ context.Context, _ models.Access, page models.Page) ([]*models.Residence, error) {
	f.calls = append(f.calls, page)
	if f.err != nil {
		return nil, f.err
	}

	return window(f.list, page), nil
}

func pID(last byte) models.ID  { return models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }
func dID(last byte) models.ID  { return models.ID("AD-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }
func rsID(last byte) models.ID { return models.ID("RS-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }

func ids(list []models.Residence) []models.ID {
	out := make([]models.ID, 0, len(list))
	for _, r := range list {
		out = append(out, r.ID)
	}

	return out
}

func sameIDs(got []models.ID, want ...models.ID) bool {
	if len(got) != len(want) {
		return false
	}

	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}

	return true
}

func sample() *fakeRepo {
	return &fakeRepo{list: []*models.Residence{
		{ID: rsID('1'), PersonID: pID('1'), PlaceID: dID('1')},
		{ID: rsID('2'), PersonID: pID('1'), PlaceID: dID('2')},
		{ID: rsID('3'), PersonID: pID('2'), PlaceID: dID('1')},
	}}
}

func TestListResidencesNoFilterReturnsAll(t *testing.T) {
	got, err := New(sample()).ListResidences(context.Background(), models.AccessFull, models.ResidenceQuery{})
	if err != nil || !sameIDs(ids(got), rsID('1'), rsID('2'), rsID('3')) {
		t.Fatalf("got %v, %v", ids(got), err)
	}
}

func TestListResidencesFilterByPerson(t *testing.T) {
	person1 := pID('1')

	got, err := New(sample()).ListResidences(context.Background(), models.AccessFull, models.ResidenceQuery{PersonID: &person1})
	if err != nil || !sameIDs(ids(got), rsID('1'), rsID('2')) {
		t.Fatalf("got %v, %v; ожидались rs1, rs2 (person1)", ids(got), err)
	}
}

func TestListResidencesFilterByPlace(t *testing.T) {
	place1 := dID('1')

	got, err := New(sample()).ListResidences(context.Background(), models.AccessFull, models.ResidenceQuery{PlaceID: &place1})
	if err != nil || !sameIDs(ids(got), rsID('1'), rsID('3')) {
		t.Fatalf("got %v, %v; ожидались rs1, rs3 (place1)", ids(got), err)
	}
}

// TestListResidencesFilterIntersects: person_id и place_id вместе —
// пересечение обоих фильтров, не объединение.
func TestListResidencesFilterIntersects(t *testing.T) {
	person1, place1 := pID('1'), dID('1')

	got, err := New(sample()).ListResidences(context.Background(), models.AccessFull,
		models.ResidenceQuery{PersonID: &person1, PlaceID: &place1})
	if err != nil || !sameIDs(ids(got), rsID('1')) {
		t.Fatalf("got %v, %v; ожидался только rs1 (пересечение person1 и place1)", ids(got), err)
	}
}

func TestListResidencesInvalidQuery(t *testing.T) {
	bad := models.ID("not-an-id")
	repo := sample()

	for _, q := range []models.ResidenceQuery{
		{PersonID: &bad},
		{PlaceID: &bad},
		{Page: models.Page{Limit: -1}},
		{Page: models.Page{Offset: -1}},
	} {
		_, err := New(repo).ListResidences(context.Background(), models.AccessFull, q)

		var ve *models.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("запрос %+v: err = %v, ожидалась *ValidationError", q, err)
		}
	}

	if len(repo.calls) != 0 {
		t.Fatalf("репозиторий вызван %d раз при некорректном запросе", len(repo.calls))
	}
}

func TestListResidencesEmptyRepoGivesEmptyNotNil(t *testing.T) {
	got, err := New(&fakeRepo{}).ListResidences(context.Background(), models.AccessFull, models.ResidenceQuery{})
	if err != nil {
		t.Fatalf("ListResidences: %v", err)
	}

	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v, ожидался пустой не-nil срез", got)
	}
}

func TestListResidencesPropagatesRepoError(t *testing.T) {
	wantErr := errors.New("repo down")

	if _, err := New(&fakeRepo{err: wantErr}).ListResidences(context.Background(), models.AccessFull, models.ResidenceQuery{}); !errors.Is(err, wantErr) {
		t.Errorf("err=%v, want %v", err, wantErr)
	}
}
