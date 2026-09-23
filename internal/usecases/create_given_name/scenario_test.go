package create_given_name

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type stubIDs struct{ id models.ID }

func (s *stubIDs) New(models.Type) models.ID { return s.id }

type fakeStore struct {
	saved   *models.GivenName
	saveErr error
}

func (f *fakeStore) SaveGivenName(_ context.Context, s *models.GivenName) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *s
	f.saved = &cp

	return nil
}

func TestCreateGivenNameGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{}
	sc := New(st, ids)

	got, err := sc.CreateGivenName(context.Background(), models.GivenName{Canonical: "Иванов", Gender: models.NameGenderMale})
	if err != nil {
		t.Fatalf("CreateGivenName: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.saved == nil || st.saved.Canonical != "Иванов" {
		t.Fatalf("saved = %+v", st.saved)
	}
}

func TestCreateGivenNameRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{})

	_, err := sc.CreateGivenName(context.Background(), models.GivenName{ID: "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1", Canonical: "x"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestCreateGivenNameRejectsEmptyCanonical(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{id: "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateGivenName(context.Background(), models.GivenName{Gender: models.NameGenderMale})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "canonical" {
		t.Fatalf("err = %v, want ValidationError on canonical", err)
	}
}

// TestCreateGivenNameRejectsInvalidGender — единственное отличие GivenName от
// остальных словарей (Surname/Patronymic/Estate/Title): gender обязателен и
// должен быть одним из male/female/neutral (internal/models/dictionary_validate.go).
func TestCreateGivenNameRejectsInvalidGender(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{id: "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateGivenName(context.Background(), models.GivenName{Canonical: "Иван", Gender: "unknown"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "gender" {
		t.Fatalf("err = %v, want ValidationError on gender", err)
	}
}

func TestCreateGivenNamePropagatesStoreError(t *testing.T) {
	wantErr := errors.New("db down")
	st := &fakeStore{saveErr: wantErr}
	sc := New(st, &stubIDs{id: "GN-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateGivenName(context.Background(), models.GivenName{Canonical: "Иванов", Gender: models.NameGenderMale})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}
