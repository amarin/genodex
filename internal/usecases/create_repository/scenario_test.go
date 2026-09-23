package create_repository

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type stubIDs struct{ id models.ID }

func (s *stubIDs) New(models.Type) models.ID { return s.id }

type fakeStore struct {
	saved   *models.Repository
	saveErr error
}

func (f *fakeStore) SaveRepository(_ context.Context, s *models.Repository) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	cp := *s
	f.saved = &cp

	return nil
}

func TestCreateRepositoryGeneratesIDAndSaves(t *testing.T) {
	ids := &stubIDs{id: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1"}
	st := &fakeStore{}
	sc := New(st, ids)

	got, err := sc.CreateRepository(context.Background(), models.Repository{Name: "ГАВО", Type: models.RepositoryTypeArchive})
	if err != nil {
		t.Fatalf("CreateRepository: %v", err)
	}

	if got.ID != ids.id {
		t.Fatalf("ID = %q, want %q", got.ID, ids.id)
	}

	if st.saved == nil || st.saved.Name != "ГАВО" {
		t.Fatalf("saved = %+v", st.saved)
	}
}

func TestCreateRepositoryRejectsExplicitID(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{})

	_, err := sc.CreateRepository(context.Background(), models.Repository{ID: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1", Name: "x"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestCreateRepositoryRejectsEmptyName(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{id: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateRepository(context.Background(), models.Repository{})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "name" {
		t.Fatalf("err = %v, want ValidationError on name", err)
	}
}

// TestCreateRepositoryRejectsInvalidType — единственное отличие Repository от
// Surname: тип хранилища обязателен (открытый enum — формат [a-z][a-z0-9_-]*,
// internal/models/dictionary_validate.go, а не фиксированный список).
func TestCreateRepositoryRejectsInvalidType(t *testing.T) {
	sc := New(&fakeStore{}, &stubIDs{id: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateRepository(context.Background(), models.Repository{Name: "ГАВО", Type: "Not Valid"})

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "type" {
		t.Fatalf("err = %v, want ValidationError on type", err)
	}
}

func TestCreateRepositoryPropagatesStoreError(t *testing.T) {
	wantErr := errors.New("db down")
	st := &fakeStore{saveErr: wantErr}
	sc := New(st, &stubIDs{id: "R-01ARZ3NDEKTSV4RRFFQ69G5FA1"})

	_, err := sc.CreateRepository(context.Background(), models.Repository{Name: "ГАВО", Type: models.RepositoryTypeArchive})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
}
