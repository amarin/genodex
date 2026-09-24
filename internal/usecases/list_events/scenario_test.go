package list_events

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	list  []*models.Event
	err   error
	calls []models.Page
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

func pID(last byte) models.ID { return models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }
func eID(last byte) models.ID { return models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA" + string(last)) }

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
