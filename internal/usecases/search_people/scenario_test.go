package search_people

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits      []models.Hit
	people    map[models.ID]*models.Person
	citations map[models.ID]*models.Citation
}

// Search эмулирует хранилищное окно generic-индекса: возвращает срез
// f.hits[page.Offset : page.Offset+page.Limit], а не только "первое окно" —
// без этого фейк не может воспроизвести реальную механику сканирования
// SearchPeople по MaxPageLimit-окнам (см. search_events/scenario_test.go).
func (f *fakeRepo) Search(_ context.Context, _ string, _ models.Access, page models.Page) ([]models.Hit, error) {
	if page.Offset >= len(f.hits) {
		return nil, nil
	}

	end := page.Offset + page.Limit
	if end > len(f.hits) {
		end = len(f.hits)
	}

	return f.hits[page.Offset:end], nil
}

func (f *fakeRepo) GetPerson(_ context.Context, id models.ID) (*models.Person, error) {
	p, ok := f.people[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return p, nil
}

func (f *fakeRepo) GetCitation(_ context.Context, id models.ID) (*models.Citation, error) {
	c, ok := f.citations[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return c, nil
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

// TestSearchPeopleHidesHitReferencingPrivateCitation: найденная персона сама
// по себе не приватна, но ссылается (Sources[i].CitationID) на приватную
// цитату — для вызывающего без полного доступа она исключается из
// результата поиска.
func TestSearchPeopleHidesHitReferencingPrivateCitation(t *testing.T) {
	id := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		hits: []models.Hit{{Type: models.TypePerson, ID: id}},
		people: map[models.ID]*models.Person{
			id: {ID: id, Sources: []models.SourceLink{{CitationID: citation}}},
		},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	got, err := New(repo).SearchPeople(context.Background(), models.AccessPublic, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchPeople: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty (ссылка на приватную цитату)", got)
	}
}

// TestSearchPeopleShowsHitReferencingPrivateCitationWithFullAccess: та же
// персона, но для вызывающего с полным доступом — находится.
func TestSearchPeopleShowsHitReferencingPrivateCitationWithFullAccess(t *testing.T) {
	id := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		hits: []models.Hit{{Type: models.TypePerson, ID: id}},
		people: map[models.ID]*models.Person{
			id: {ID: id, Sources: []models.SourceLink{{CitationID: citation}}},
		},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	got, err := New(repo).SearchPeople(context.Background(), models.AccessFull, models.SearchQuery{Text: "Ив"})
	if err != nil {
		t.Fatalf("SearchPeople: %v", err)
	}

	if len(got) != 1 || got[0].ID != id {
		t.Fatalf("got = %+v", got)
	}
}

// TestSearchPeoplePagesWithoutDuplicatesWhenHitBeforeOffsetIsHidden: хит,
// предшествующий запрошенному offset, скрыт (ссылается на приватную
// цитату). Порядок хитов: [hidden, A, B]. Постранично (limit=1) с offset=0 и
// offset=1 должны вернуться разные видимые персоны — A, затем B — без
// дублей и без пропусков (по образцу
// search_events/scenario_test.go:TestSearchEventsPagesWithoutDuplicatesWhenHitBeforeOffsetIsHidden).
func TestSearchPeoplePagesWithoutDuplicatesWhenHitBeforeOffsetIsHidden(t *testing.T) {
	hiddenID := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	idA := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	idB := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA3")
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypePerson, ID: hiddenID},
			{Type: models.TypePerson, ID: idA},
			{Type: models.TypePerson, ID: idB},
		},
		people: map[models.ID]*models.Person{
			hiddenID: {ID: hiddenID, Sources: []models.SourceLink{{CitationID: citation}}},
			idA:      {ID: idA},
			idB:      {ID: idB},
		},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	scenario := New(repo)

	page1, err := scenario.SearchPeople(context.Background(), models.AccessPublic,
		models.SearchQuery{Text: "Ив", Page: models.Page{Limit: 1, Offset: 0}})
	if err != nil {
		t.Fatalf("SearchPeople (page1): %v", err)
	}

	if len(page1) != 1 || page1[0].ID != idA {
		t.Fatalf("page1 = %+v, want [A]", page1)
	}

	page2, err := scenario.SearchPeople(context.Background(), models.AccessPublic,
		models.SearchQuery{Text: "Ив", Page: models.Page{Limit: 1, Offset: 1}})
	if err != nil {
		t.Fatalf("SearchPeople (page2): %v", err)
	}

	if len(page2) != 1 || page2[0].ID != idB {
		t.Fatalf("page2 = %+v, want [B] (не дубликат page1!)", page2)
	}
}
