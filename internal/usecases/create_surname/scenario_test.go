package create_surname

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type stubIDs struct{ id models.ID }

func (s *stubIDs) New(models.Type) models.ID { return s.id }

type fakeStore struct {
	saved   *models.Surname
	saveErr error
}

func (f *fakeStore) SaveSurname(_ context.Context, s *models.Surname) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *s
	f.saved = &cp

	return nil
}

func TestCreateSurnameGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "SN-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{}
	sc := New(st, ids)

	got, err := sc.CreateSurname(context.Background(), models.Surname{Canonical: "Иванов"})
	if err != nil {
		t.Fatalf("CreateSurname: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.saved == nil || st.saved.Canonical != "Иванов" {
		t.Fatalf("saved = %+v", st.saved)
	}
}

func TestCreateSurnameRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{})

	_, err := sc.CreateSurname(context.Background(), models.Surname{ID: "SN-01ARZ3NDEKTSV4RRFFQ69G5FA1", Canonical: "x"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestCreateSurnameRejectsEmptyCanonical(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{id: "SN-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateSurname(context.Background(), models.Surname{})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "canonical" {
		t.Fatalf("err = %v, want ValidationError on canonical", err)
	}
}

func TestCreateSurnamePropagatesStoreError(t *testing.T) {
	wantErr := errors.New("db down")
	st := &fakeStore{saveErr: wantErr}
	sc := New(st, &stubIDs{id: "SN-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateSurname(context.Background(), models.Surname{Canonical: "Иванов"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}
