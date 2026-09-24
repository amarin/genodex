package get_person

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
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

func TestGetPersonReturnsRecord(t *testing.T) {
	id := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{people: map[models.ID]*models.Person{id: {ID: id, Gender: models.PersonGenderFemale}}}

	got, err := New(repo).GetPerson(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetPerson: %v", err)
	}

	if got.Gender != models.PersonGenderFemale {
		t.Fatalf("Gender = %q", got.Gender)
	}
}

func TestGetPersonNotFound(t *testing.T) {
	repo := &fakeRepo{people: map[models.ID]*models.Person{}}

	_, err := New(repo).GetPerson(context.Background(), models.AccessFull, "I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetPersonInvalidID(t *testing.T) {
	repo := &fakeRepo{people: map[models.ID]*models.Person{}}

	_, err := New(repo).GetPerson(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestGetPersonPrivateHiddenFromPublic(t *testing.T) {
	id := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{people: map[models.ID]*models.Person{id: {ID: id, Private: true}}}

	_, err := New(repo).GetPerson(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for private record with AccessPublic", err)
	}
}

func TestGetPersonPrivateVisibleToFullAccess(t *testing.T) {
	id := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{people: map[models.ID]*models.Person{id: {ID: id, Private: true}}}

	got, err := New(repo).GetPerson(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetPerson: %v", err)
	}

	if got.ID != id {
		t.Fatalf("ID = %q", got.ID)
	}
}

// TestGetPersonHiddenWhenReferencesPrivateCitation: персона сама по себе
// публична, но ссылается (Sources[i].CitationID) на приватную цитату — для
// вызывающего без полного доступа она отдаётся как models.ErrNotFound, тот
// же принцип «прячем как отсутствующее», что и для собственного Private.
func TestGetPersonHiddenWhenReferencesPrivateCitation(t *testing.T) {
	id := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		people: map[models.ID]*models.Person{
			id: {ID: id, Sources: []models.SourceLink{{CitationID: citation}}},
		},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	_, err := New(repo).GetPerson(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound (ссылка на приватную цитату)", err)
	}
}

// TestGetPersonVisibleWithFullAccessWhenReferencesPrivateCitation: та же
// персона, но для вызывающего с полным доступом — видна.
func TestGetPersonVisibleWithFullAccessWhenReferencesPrivateCitation(t *testing.T) {
	id := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		people: map[models.ID]*models.Person{
			id: {ID: id, Sources: []models.SourceLink{{CitationID: citation}}},
		},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	got, err := New(repo).GetPerson(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetPerson: %v", err)
	}

	if got.ID != id {
		t.Fatalf("ID = %q", got.ID)
	}
}
