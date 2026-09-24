package get_event

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	events map[models.ID]*models.Event
}

func (f *fakeRepo) GetEvent(_ context.Context, id models.ID) (*models.Event, error) {
	e, ok := f.events[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return e, nil
}

func TestGetEventReturnsRecord(t *testing.T) {
	id := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{events: map[models.ID]*models.Event{id: {ID: id, Type: models.EventTypeBirth}}}

	got, err := New(repo).GetEvent(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}

	if got.Type != models.EventTypeBirth {
		t.Fatalf("Type = %q", got.Type)
	}
}

func TestGetEventNotFound(t *testing.T) {
	repo := &fakeRepo{events: map[models.ID]*models.Event{}}

	_, err := New(repo).GetEvent(context.Background(), models.AccessFull, "E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetEventInvalidID(t *testing.T) {
	repo := &fakeRepo{events: map[models.ID]*models.Event{}}

	_, err := New(repo).GetEvent(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestGetEventPrivateHiddenFromPublic(t *testing.T) {
	id := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{events: map[models.ID]*models.Event{id: {ID: id, Private: true}}}

	_, err := New(repo).GetEvent(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for private record with AccessPublic", err)
	}
}

func TestGetEventPrivateVisibleToFullAccess(t *testing.T) {
	id := models.ID("E-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{events: map[models.ID]*models.Event{id: {ID: id, Private: true}}}

	got, err := New(repo).GetEvent(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}

	if !got.Private {
		t.Fatalf("Private = %v", got.Private)
	}
}
