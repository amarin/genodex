package list_events

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	list      []*models.Event
	err       error
	calls     []models.Page
	people    map[models.ID]*models.Person
	citations map[models.ID]*models.Citation
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

func window(list []*models.Event, page models.Page) []*models.Event {
	page = page.Normalized()
	if page.Offset >= len(list) {
		return nil
	}

	return list[page.Offset:min(page.Offset+page.Limit, len(list))]
}

func (f *fakeRepo) ListEvents(_ context.Context, _ models.Access, page models.Page) ([]*models.Event, error) {
	f.calls = append(f.calls, page)
	if f.err != nil {
		return nil, f.err
	}

	return window(f.list, page), nil
}

func pID(last byte) models.ID   { return models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }
func eID(last byte) models.ID   { return models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }
func citID(last byte) models.ID { return models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }

func ids(list []models.Event) []models.ID {
	out := make([]models.ID, 0, len(list))
	for _, e := range list {
		out = append(out, e.ID)
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
	return &fakeRepo{list: []*models.Event{
		{ID: eID('1'), Participants: []models.EventParticipant{{PersonID: pID('1'), Role: "младенец"}}},
		{ID: eID('2'), Participants: []models.EventParticipant{{PersonID: pID('2'), Role: "младенец"}}},
		{ID: eID('3'), Participants: []models.EventParticipant{
			{PersonID: pID('3'), Role: "жених"}, {PersonID: pID('1'), Role: "свидетель"},
		}},
	}}
}

func TestListEventsNoFilterReturnsAll(t *testing.T) {
	got, err := New(sample()).ListEvents(context.Background(), models.AccessFull, models.EventQuery{})
	if err != nil || !sameIDs(ids(got), eID('1'), eID('2'), eID('3')) {
		t.Fatalf("got %v, %v", ids(got), err)
	}
}

// TestListEventsFilterByParticipant: person_id совпадает с ЛЮБЫМ из
// Participants[i].PersonID, не только с первым.
func TestListEventsFilterByParticipant(t *testing.T) {
	person1 := pID('1')

	got, err := New(sample()).ListEvents(context.Background(), models.AccessFull, models.EventQuery{PersonID: &person1})
	if err != nil || !sameIDs(ids(got), eID('1'), eID('3')) {
		t.Fatalf("got %v, %v; ожидались e1 (первый участник) и e3 (второй участник)", ids(got), err)
	}
}

func TestListEventsFilterNoMatch(t *testing.T) {
	other := pID('9')

	got, err := New(sample()).ListEvents(context.Background(), models.AccessFull, models.EventQuery{PersonID: &other})
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v; ожидался пустой результат", ids(got), err)
	}
}

func TestListEventsInvalidQuery(t *testing.T) {
	bad := models.ID("not-an-id")
	repo := sample()

	for _, q := range []models.EventQuery{
		{PersonID: &bad},
		{Page: models.Page{Limit: -1}},
		{Page: models.Page{Offset: -1}},
	} {
		_, err := New(repo).ListEvents(context.Background(), models.AccessFull, q)

		var ve *models.ValidationError
		if !errors.As(err, &ve) {
			t.Errorf("запрос %+v: err = %v, ожидалась *ValidationError", q, err)
		}
	}

	if len(repo.calls) != 0 {
		t.Fatalf("репозиторий вызван %d раз при некорректном запросе", len(repo.calls))
	}
}

func TestListEventsEmptyRepoGivesEmptyNotNil(t *testing.T) {
	got, err := New(&fakeRepo{}).ListEvents(context.Background(), models.AccessFull, models.EventQuery{})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}

	if got == nil || len(got) != 0 {
		t.Fatalf("got %#v, ожидался пустой не-nil срез", got)
	}
}

func TestListEventsPropagatesRepoError(t *testing.T) {
	wantErr := errors.New("repo down")

	if _, err := New(&fakeRepo{err: wantErr}).ListEvents(context.Background(), models.AccessFull, models.EventQuery{}); !errors.Is(err, wantErr) {
		t.Errorf("err=%v, want %v", err, wantErr)
	}
}

// TestListEventsHidesEventReferencingPrivateParticipant: событие само по
// себе не приватно, но один из его участников приватен — для вызывающего
// без полного доступа оно исключается из списка.
func TestListEventsHidesEventReferencingPrivateParticipant(t *testing.T) {
	repo := sample()
	repo.people = map[models.ID]*models.Person{
		pID('1'): {ID: pID('1'), Private: true},
		pID('2'): {ID: pID('2'), Private: false},
		pID('3'): {ID: pID('3'), Private: false},
	}

	got, err := New(repo).ListEvents(context.Background(), models.AccessPublic, models.EventQuery{})
	if err != nil || !sameIDs(ids(got), eID('2')) {
		t.Fatalf("got %v, %v; ожидался только e2 (e1/e3 ссылаются на приватную person1)", ids(got), err)
	}
}

// TestListEventsShowsEventReferencingPrivateParticipantWithFullAccess: та
// же выборка, но для вызывающего с полным доступом — все события видимы.
func TestListEventsShowsEventReferencingPrivateParticipantWithFullAccess(t *testing.T) {
	repo := sample()
	repo.people = map[models.ID]*models.Person{
		pID('1'): {ID: pID('1'), Private: true},
	}

	got, err := New(repo).ListEvents(context.Background(), models.AccessFull, models.EventQuery{})
	if err != nil || !sameIDs(ids(got), eID('1'), eID('2'), eID('3')) {
		t.Fatalf("got %v, %v; ожидались все три события при полном доступе", ids(got), err)
	}
}

// TestListEventsDoesNotHideEventsReferencingPublicParticipants: ни один из
// участников не приватен — список не урезается ошибочно.
func TestListEventsDoesNotHideEventsReferencingPublicParticipants(t *testing.T) {
	repo := sample()
	repo.people = map[models.ID]*models.Person{
		pID('1'): {ID: pID('1'), Private: false},
		pID('2'): {ID: pID('2'), Private: false},
		pID('3'): {ID: pID('3'), Private: false},
	}

	got, err := New(repo).ListEvents(context.Background(), models.AccessPublic, models.EventQuery{})
	if err != nil || !sameIDs(ids(got), eID('1'), eID('2'), eID('3')) {
		t.Fatalf("got %v, %v; ожидались все три события — все участники публичны", ids(got), err)
	}
}

// TestListEventsHidesEventReferencingPrivateCitation: событие само по себе
// не приватно и участники публичны, но одна из его Sources ссылается на
// приватную цитату — для вызывающего без полного доступа оно исключается
// из списка. Независимая проверка, параллельная
// TestListEventsHidesEventReferencingPrivateParticipant.
func TestListEventsHidesEventReferencingPrivateCitation(t *testing.T) {
	repo := sample()
	repo.people = map[models.ID]*models.Person{
		pID('1'): {ID: pID('1'), Private: false},
		pID('2'): {ID: pID('2'), Private: false},
		pID('3'): {ID: pID('3'), Private: false},
	}
	repo.list[0].Sources = []models.SourceLink{{CitationID: citID('1')}}
	repo.citations = map[models.ID]*models.Citation{
		citID('1'): {ID: citID('1'), Private: true},
	}

	got, err := New(repo).ListEvents(context.Background(), models.AccessPublic, models.EventQuery{})
	if err != nil || !sameIDs(ids(got), eID('2'), eID('3')) {
		t.Fatalf("got %v, %v; ожидались e2 и e3 (e1 ссылается на приватную цитату)", ids(got), err)
	}
}

// TestListEventsShowsEventReferencingPrivateCitationWithFullAccess: та же
// выборка, но для вызывающего с полным доступом — все события видимы.
func TestListEventsShowsEventReferencingPrivateCitationWithFullAccess(t *testing.T) {
	repo := sample()
	repo.list[0].Sources = []models.SourceLink{{CitationID: citID('1')}}
	repo.citations = map[models.ID]*models.Citation{
		citID('1'): {ID: citID('1'), Private: true},
	}

	got, err := New(repo).ListEvents(context.Background(), models.AccessFull, models.EventQuery{})
	if err != nil || !sameIDs(ids(got), eID('1'), eID('2'), eID('3')) {
		t.Fatalf("got %v, %v; ожидались все три события при полном доступе", ids(got), err)
	}
}

// TestListEventsPagesWithoutDuplicatesWhenHitHiddenByCitation: из двух
// событий первое скрыто приватной цитатой — offset=0/limit=1 дважды подряд
// не должен ни дублировать, ни терять второе (видимое) событие.
func TestListEventsPagesWithoutDuplicatesWhenHitHiddenByCitation(t *testing.T) {
	repo := &fakeRepo{list: []*models.Event{
		{ID: eID('1'), Participants: []models.EventParticipant{{PersonID: pID('1')}}, Sources: []models.SourceLink{{CitationID: citID('1')}}},
		{ID: eID('2'), Participants: []models.EventParticipant{{PersonID: pID('2')}}},
	}}
	repo.people = map[models.ID]*models.Person{
		pID('1'): {ID: pID('1'), Private: false},
		pID('2'): {ID: pID('2'), Private: false},
	}
	repo.citations = map[models.ID]*models.Citation{
		citID('1'): {ID: citID('1'), Private: true},
	}

	page1, err := New(repo).ListEvents(context.Background(), models.AccessPublic,
		models.EventQuery{Page: models.Page{Limit: 1, Offset: 0}})
	if err != nil || !sameIDs(ids(page1), eID('2')) {
		t.Fatalf("page1 = %v, %v; ожидался e2 (e1 скрыт приватной цитатой)", ids(page1), err)
	}

	page2, err := New(repo).ListEvents(context.Background(), models.AccessPublic,
		models.EventQuery{Page: models.Page{Limit: 1, Offset: 1}})
	if err != nil || len(page2) != 0 {
		t.Fatalf("page2 = %v, %v; ожидался пустой результат (второй страницы нет)", ids(page2), err)
	}
}
