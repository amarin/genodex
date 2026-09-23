package create_church

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type stubIDs struct{ id models.ID }

func (s *stubIDs) New(models.Type) models.ID { return s.id }

type fakeStore struct {
	saved   *models.Church
	saveErr error
}

func (f *fakeStore) SaveChurch(_ context.Context, s *models.Church) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *s
	f.saved = &cp

	return nil
}

func TestCreateChurchGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "CH-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{}
	sc := New(st, ids)

	got, err := sc.CreateChurch(context.Background(), models.Church{Name: "Никольская церковь"})
	if err != nil {
		t.Fatalf("CreateChurch: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.saved == nil || st.saved.Name != "Никольская церковь" {
		t.Fatalf("saved = %+v", st.saved)
	}
}

func TestCreateChurchRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{})

	_, err := sc.CreateChurch(context.Background(), models.Church{ID: "CH-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "x"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestCreateChurchRejectsEmptyName(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{id: "CH-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateChurch(context.Background(), models.Church{})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, want ValidationError on name", err)
	}
}

func TestCreateChurchPropagatesStoreError(t *testing.T) {
	wantErr := errors.New("db down")
	st := &fakeStore{saveErr: wantErr}
	sc := New(st, &stubIDs{id: "CH-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateChurch(context.Background(), models.Church{Name: "Никольская церковь"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}
