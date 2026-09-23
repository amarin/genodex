package create_patronymic

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type stubIDs struct{ id models.ID }

func (s *stubIDs) New(models.Type) models.ID { return s.id }

type fakeStore struct {
	saved   *models.Patronymic
	saveErr error
}

func (f *fakeStore) SavePatronymic(_ context.Context, s *models.Patronymic) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *s
	f.saved = &cp

	return nil
}

func TestCreatePatronymicGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "PN-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{}
	sc := New(st, ids)

	got, err := sc.CreatePatronymic(context.Background(), models.Patronymic{Canonical: "Иванов"})
	if err != nil {
		t.Fatalf("CreatePatronymic: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.saved == nil || st.saved.Canonical != "Иванов" {
		t.Fatalf("saved = %+v", st.saved)
	}
}

func TestCreatePatronymicRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{})

	_, err := sc.CreatePatronymic(context.Background(), models.Patronymic{ID: "PN-01ARZ3NDEKTSV4RRFFQ69G5FA1", Canonical: "x"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestCreatePatronymicRejectsEmptyCanonical(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{id: "PN-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreatePatronymic(context.Background(), models.Patronymic{})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "canonical" {
		t.Fatalf("err = %v, want ValidationError on canonical", err)
	}
}

func TestCreatePatronymicPropagatesStoreError(t *testing.T) {
	wantErr := errors.New("db down")
	st := &fakeStore{saveErr: wantErr}
	sc := New(st, &stubIDs{id: "PN-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreatePatronymic(context.Background(), models.Patronymic{Canonical: "Иванов"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}
