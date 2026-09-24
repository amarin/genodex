package search_people

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits   []models.Hit
	people map[models.ID]*models.Person
}

func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset > 0 {
		return nil, nil
	}

	return f.hits, nil
}

func (f *fakeRepo) GetPerson(_ context.Context, id models.ID) (*models.Person, error) {
	p, ok := f.people[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return p, nil
}

func TestSearchPeopleFiltersByType(t *testing.T) {
	id := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("F-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypePerson, ID: id},
			{Type: models.TypeFamily, ID: otherID}, // не person — должен быть пропущен
		},
		people: map[models.ID]*models.Person{id: {ID: id}},
	}

	got, err := New(repo).SearchPeople(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchPeople: %v", err)
	}

	if len(got) != 1 || got[0].ID != id {
		t.Fatalf("got = %+v", got)
	}
}

// TestSearchPeopleFindsByMarriedNameOnly проверяет, что поиск находит
// персону по части фамилии/имени/отчества из ЛЮБОГО элемента Names, а не
// только из первого/основного — то же свойство, что доказывает real-store
// тест (TestPersonWriteContractWithRealStore), но здесь на уровне сценария:
// хранилище-фейк отдаёт хит из search_index, GetPerson возвращает запись,
// у которой совпадение было бы только во второй записи Names.
func TestSearchPeopleFindsByMarriedNameOnly(t *testing.T) {
	id := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		hits: []models.Hit{{Type: models.TypePerson, ID: id}},
		people: map[models.ID]*models.Person{id: {ID: id, Names: []models.PersonName{
			{Type: models.PersonNameMain, Surname: models.TextRef{Text: "Иванова"}},
			{Type: models.PersonNameMarried, Surname: models.TextRef{Text: "Петрова"}},
		}}},
	}

	got, err := New(repo).SearchPeople(context.Background(), models.AccessFull, models.SearchQuery{Text: "Петр"})
	if err != nil {
		t.Fatalf("SearchPeople: %v", err)
	}

	if len(got) != 1 || got[0].ID != id {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchPeopleEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchPeople(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchPeople: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}
