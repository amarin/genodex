package search_events

import (
	"context"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	hits   []models.Hit
	events map[models.ID]*models.Event
	people map[models.ID]*models.Person
}

// Search эмулирует хранилищное окно generic-индекса: возвращает срез
// f.hits[page.Offset : page.Offset+page.Limit], а не только "первое окно".
// Без этого фейк не может воспроизвести реальную механику сканирования
// SearchEvents по MaxPageLimit-окнам и не годится для теста пагинации
// (Fix 1 ревью).
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

func (f *fakeRepo) GetEvent(_ context.Context, id models.ID) (*models.Event, error) {
	e, ok := f.events[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return e, nil
}

func (f *fakeRepo) GetPerson(_ context.Context, id models.ID) (*models.Person, error) {
	p, ok := f.people[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return p, nil
}

func TestSearchEventsFiltersByType(t *testing.T) {
	id := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	otherID := models.ID("F-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeEvent, ID: id},
			{Type: models.TypeFamily, ID: otherID}, // не event — должен быть пропущен
		},
		events: map[models.ID]*models.Event{
			id: {ID: id, Type: models.EventTypeBirth, Place: &models.PlaceRef{Text: "Давыдово"}},
		},
	}

	got, err := New(repo).SearchEvents(context.Background(), models.AccessFull, models.SearchQuery{Text: "Дав"})
	if err != nil {
		t.Fatalf("SearchEvents: %v", err)
	}

	if len(got) != 1 || got[0].Place == nil || got[0].Place.Text != "Давыдово" {
		t.Fatalf("got = %+v", got)
	}
}

func TestSearchEventsEmptyText(t *testing.T) {
	got, err := New(&fakeRepo{}).SearchEvents(context.Background(), models.AccessFull, models.SearchQuery{Text: "  "})
	if err != nil {
		t.Fatalf("SearchEvents: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty", got)
	}
}

// TestSearchEventsHidesHitReferencingPrivateParticipant: найденное событие
// само по себе не приватно, но один из его участников приватен — для
// вызывающего без полного доступа оно исключается из результата поиска
// (тот же приём, что и для исчезнувшей между поиском и чтением записи:
// матч пропускается, а не превращается в ошибку).
func TestSearchEventsHidesHitReferencingPrivateParticipant(t *testing.T) {
	id := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	person := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		hits: []models.Hit{{Type: models.TypeEvent, ID: id}},
		events: map[models.ID]*models.Event{
			id: {
				ID: id, Private: false, Place: &models.PlaceRef{Text: "Давыдово"},
				Participants: []models.EventParticipant{{PersonID: person}},
			},
		},
		people: map[models.ID]*models.Person{person: {ID: person, Private: true}},
	}

	got, err := New(repo).SearchEvents(context.Background(), models.AccessPublic, models.SearchQuery{Text: "Дав"})
	if err != nil {
		t.Fatalf("SearchEvents: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got = %+v, want empty (участник приватен)", got)
	}
}

// TestSearchEventsShowsHitReferencingPrivateParticipantWithFullAccess: то
// же событие, но для вызывающего с полным доступом — находится.
func TestSearchEventsShowsHitReferencingPrivateParticipantWithFullAccess(t *testing.T) {
	id := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	person := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		hits: []models.Hit{{Type: models.TypeEvent, ID: id}},
		events: map[models.ID]*models.Event{
			id: {
				ID: id, Private: false, Place: &models.PlaceRef{Text: "Давыдово"},
				Participants: []models.EventParticipant{{PersonID: person}},
			},
		},
		people: map[models.ID]*models.Person{person: {ID: person, Private: true}},
	}

	got, err := New(repo).SearchEvents(context.Background(), models.AccessFull, models.SearchQuery{Text: "Дав"})
	if err != nil {
		t.Fatalf("SearchEvents: %v", err)
	}

	if len(got) != 1 || got[0].ID != id {
		t.Fatalf("got = %+v", got)
	}
}

// TestSearchEventsDoesNotHideHitReferencingPublicParticipant: участник
// публичен — событие не пропадает из результата ошибочно.
func TestSearchEventsDoesNotHideHitReferencingPublicParticipant(t *testing.T) {
	id := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	person := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		hits: []models.Hit{{Type: models.TypeEvent, ID: id}},
		events: map[models.ID]*models.Event{
			id: {
				ID: id, Private: false, Place: &models.PlaceRef{Text: "Давыдово"},
				Participants: []models.EventParticipant{{PersonID: person}},
			},
		},
		people: map[models.ID]*models.Person{person: {ID: person, Private: false}},
	}

	got, err := New(repo).SearchEvents(context.Background(), models.AccessPublic, models.SearchQuery{Text: "Дав"})
	if err != nil {
		t.Fatalf("SearchEvents: %v", err)
	}

	if len(got) != 1 || got[0].ID != id {
		t.Fatalf("got = %+v", got)
	}
}

// TestSearchEventsPagesWithoutDuplicatesWhenHitBeforeOffsetIsHidden:
// регрессия на ревью-баг (Fix 1) — хит, ПРЕДШЕСТВУЮЩИЙ запрошенному offset,
// скрыт (ссылается на приватного участника). До фикса offset считался ДО
// проверки приватности: скрытый хит на второй странице съедал часть
// offset-бюджета "вслепую" (даже не проверив, что он скрыт), из-за чего
// вторая страница возвращала тот же элемент, что и первая. Порядок хитов:
// [hidden, A (public), B (public)]. Постранично (limit=1) с offset=0 и
// offset=1 должны вернуться РАЗНЫЕ видимые события — A, затем B — без
// дублей и без пропусков.
func TestSearchEventsPagesWithoutDuplicatesWhenHitBeforeOffsetIsHidden(t *testing.T) {
	hiddenID := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	idA := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	idB := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA3")
	privatePerson := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")

	repo := &fakeRepo{
		hits: []models.Hit{
			{Type: models.TypeEvent, ID: hiddenID},
			{Type: models.TypeEvent, ID: idA},
			{Type: models.TypeEvent, ID: idB},
		},
		events: map[models.ID]*models.Event{
			hiddenID: {
				ID: hiddenID, Private: false, Place: &models.PlaceRef{Text: "Давыдово-0"},
				Participants: []models.EventParticipant{{PersonID: privatePerson}},
			},
			idA: {ID: idA, Private: false, Place: &models.PlaceRef{Text: "Давыдово-A"}},
			idB: {ID: idB, Private: false, Place: &models.PlaceRef{Text: "Давыдово-B"}},
		},
		people: map[models.ID]*models.Person{privatePerson: {ID: privatePerson, Private: true}},
	}

	scenario := New(repo)

	page1, err := scenario.SearchEvents(context.Background(), models.AccessPublic,
		models.SearchQuery{Text: "Дав", Page: models.Page{Limit: 1, Offset: 0}})
	if err != nil {
		t.Fatalf("SearchEvents (page1): %v", err)
	}

	if len(page1) != 1 || page1[0].ID != idA {
		t.Fatalf("page1 = %+v, want [A]", page1)
	}

	page2, err := scenario.SearchEvents(context.Background(), models.AccessPublic,
		models.SearchQuery{Text: "Дав", Page: models.Page{Limit: 1, Offset: 1}})
	if err != nil {
		t.Fatalf("SearchEvents (page2): %v", err)
	}

	if len(page2) != 1 || page2[0].ID != idB {
		t.Fatalf("page2 = %+v, want [B] (не дубликат page1!)", page2)
	}

	// Третья страница — конец списка видимых событий (только A и B видимы).
	page3, err := scenario.SearchEvents(context.Background(), models.AccessPublic,
		models.SearchQuery{Text: "Дав", Page: models.Page{Limit: 1, Offset: 2}})
	if err != nil {
		t.Fatalf("SearchEvents (page3): %v", err)
	}

	if len(page3) != 0 {
		t.Fatalf("page3 = %+v, want empty (видимых событий всего 2)", page3)
	}

	// Дополнительно: весь видимый результат одним вызовом (без пагинации)
	// содержит ровно A и B, hiddenID отсутствует.
	all, err := scenario.SearchEvents(context.Background(), models.AccessPublic, models.SearchQuery{Text: "Дав"})
	if err != nil {
		t.Fatalf("SearchEvents (all): %v", err)
	}

	if len(all) != 2 || all[0].ID != idA || all[1].ID != idB {
		t.Fatalf("all = %+v, want [A, B]", all)
	}
}
