package create_parish

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type stubIDs struct{ id models.ID }

func (s *stubIDs) New(models.Type) models.ID { return s.id }

type fakeStore struct {
	saved   *models.Parish
	saveErr error
}

func (f *fakeStore) SaveParish(_ context.Context, s *models.Parish) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *s
	f.saved = &cp

	return nil
}

func TestCreateParishGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "PR-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{}
	sc := New(st, ids)

	got, err := sc.CreateParish(context.Background(), models.Parish{Name: "Никольский приход"})
	if err != nil {
		t.Fatalf("CreateParish: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.saved == nil || st.saved.Name != "Никольский приход" {
		t.Fatalf("saved = %+v", st.saved)
	}
}

func TestCreateParishRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{})

	_, err := sc.CreateParish(context.Background(), models.Parish{ID: "PR-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "x"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestCreateParishRejectsEmptyName(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{id: "PR-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateParish(context.Background(), models.Parish{})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, want ValidationError on name", err)
	}
}

func TestCreateParishPropagatesStoreError(t *testing.T) {
	wantErr := errors.New("db down")
	st := &fakeStore{saveErr: wantErr}
	sc := New(st, &stubIDs{id: "PR-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateParish(context.Background(), models.Parish{Name: "Никольский приход"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}
