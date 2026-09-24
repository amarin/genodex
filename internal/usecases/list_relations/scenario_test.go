package list_relations

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	list  []*models.Relation
	err   error
	calls []models.Page
}

func window(list []*models.Relation, page models.Page) []*models.Relation {
	page = page.Normalized()
	if page.Offset >= len(list) {
		return nil
	}

	return list[page.Offset:min(page.Offset+page.Limit, len(list))]
}

func (f *fakeRepo) ListRelations(_ context.Context, _ models.Access, page models.Page) ([]*models.Relation, error) {
	f.calls = append(f.calls, page)
	if f.err != nil {
		return nil, f.err
	}

	return window(f.list, page), nil
}

func pID(last byte) models.ID  { return models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }
func rlID(last byte) models.ID { return models.ID("RL-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }

func ids(list []models.Relation) []models.ID {
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
	return &fakeRepo{list: []*models.Relation{
		{ID: rlID('1'), PersonA: pID('1'), PersonB: pID('2')},
		{ID: rlID('2'), PersonA: pID('3'), PersonB: pID('4')},
		{ID: rlID('3'), PersonA: pID('5'), PersonB: pID('1')}, // person1 as person_b
	}}
}

func TestListRelationsNoFilterReturnsAll(t *testing.T) {
	got, err := New(sample()).ListRelations(context.Background(), models.AccessFull, models.RelationQuery{})
	if err != nil || !sameIDs(ids(got), rlID('1'), rlID('2'), rlID('3')) {
		t.Fatalf("got %v, %v", ids(got), err)
	}
}

// TestListRelationsFilterMatchesEitherSide: person_id совпадает с person_a
// ИЛИ person_b — оба случая проходят фильтр.
func TestListRelationsFilterMatchesEitherSide(t *testing.T) {
	person1 := pID('1')

	got, err := New(sample()).ListRelations(context.Background(), models.AccessFull, models.RelationQuery{PersonID: &person1})
	if err != nil || !sameIDs(ids(got), rlID('1'), rlID('3')) {
		t.Fatalf("got %v, %v; ожидались rl1 (person_a) и rl3 (person_b)", ids(got), err)
	}
}

func TestListRelationsFilterNoMatch(t *testing.T) {
	other := pID('9')

	got, err := New(sample()).ListRelations(context.Background(), models.AccessFull, models.RelationQuery{PersonID: &other})
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v; ожидался пустой результат", ids(got), err)
	}
}

func TestListRelationsInvalidQuery(t *testing.T) {
	bad := models.ID("not-an-id")
	repo := sample()

	for _, q := range []models.RelationQuery{
		{PersonID: &bad},
		{Page: models.Page{Limit: -1}},
		{Page: models.Page{Offset: -1}},
	} {
		_, err := New(repo).ListRelations(context.Background(), models.AccessFull, q)

		var ve *models.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("запрос %+v: err = %v, ожидалась *ValidationError", q, err)
		}
	}

	if len(repo.calls) != 0 {
		t.Fatalf("репозиторий вызван %d раз при некорректном запросе", len(repo.calls))
	}
}

func TestListRelationsEmptyRepoGivesEmptyNotNil(t *testing.T) {
	got, err := New(&fakeRepo{}).ListRelations(context.Background(), models.AccessFull, models.RelationQuery{})
	if err != nil {
		t.Fatalf("ListRelations: %v", err)
	}

	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v, ожидался пустой не-nil срез", got)
	}
}

func TestListRelationsPropagatesRepoError(t *testing.T) {
	wantErr := errors.New("repo down")

	if _, err := New(&fakeRepo{err: wantErr}).ListRelations(context.Background(), models.AccessFull, models.RelationQuery{}); !errors.Is(err, wantErr) {
		t.Errorf("err=%v, want %v", err, wantErr)
	}
}

// TestListRelationsWindowAppliedAfterFilter: сдвиг и размер считаются среди
// прошедших фильтр, а не среди всех рёбер.
func TestListRelationsWindowAppliedAfterFilter(t *testing.T) {
	person1 := pID('1')
	q := models.RelationQuery{PersonID: &person1, Page: models.Page{Limit: 1, Offset: 1}}

	got, err := New(sample()).ListRelations(context.Background(), models.AccessFull, q)
	if err != nil || !sameIDs(ids(got), rlID('3')) {
		t.Fatalf("got %v, %v; ожидался второй элемент отфильтрованного списка (rl3)", ids(got), err)
	}
}
