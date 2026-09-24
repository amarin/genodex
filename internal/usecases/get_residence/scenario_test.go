package get_residence

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	residences map[models.ID]*models.Residence
	people     map[models.ID]*models.Person
	citations  map[models.ID]*models.Citation
}

func (f *fakeRepo) GetResidence(_ context.Context, id models.ID) (*models.Residence, error) {
	r, ok := f.residences[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return r, nil
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

func TestGetResidenceReturnsRecord(t *testing.T) {
	id := models.ID("RS-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{residences: map[models.ID]*models.Residence{id: {ID: id, Note: "изба"}}}

	got, err := New(repo).GetResidence(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetResidence: %v", err)
	}

	if got.Note != "изба" {
		t.Fatalf("Note = %q", got.Note)
	}
}

func TestGetResidenceNotFound(t *testing.T) {
	repo := &fakeRepo{residences: map[models.ID]*models.Residence{}}

	_, err := New(repo).GetResidence(context.Background(), models.AccessFull, "RS-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetResidenceInvalidID(t *testing.T) {
	repo := &fakeRepo{residences: map[models.ID]*models.Residence{}}

	_, err := New(repo).GetResidence(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestGetResidencePrivateHiddenFromPublic(t *testing.T) {
	id := models.ID("RS-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{residences: map[models.ID]*models.Residence{id: {ID: id, Private: true}}}

	_, err := New(repo).GetResidence(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for private record with AccessPublic", err)
	}
}

func TestGetResidencePrivateVisibleToFullAccess(t *testing.T) {
	id := models.ID("RS-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{residences: map[models.ID]*models.Residence{id: {ID: id, Private: true}}}

	got, err := New(repo).GetResidence(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetResidence: %v", err)
	}

	if !got.Private {
		t.Fatalf("Private = %v", got.Private)
	}
}

// TestGetResidenceHiddenWhenReferencedPersonPrivate: проживание само по
// себе не приватно, но PersonID приватен — прячется как отсутствующее для
// вызывающего без полного доступа.
func TestGetResidenceHiddenWhenReferencedPersonPrivate(t *testing.T) {
	id := models.ID("RS-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	person := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		residences: map[models.ID]*models.Residence{id: {ID: id, PersonID: person, Private: false}},
		people:     map[models.ID]*models.Person{person: {ID: person, Private: true}},
	}

	_, err := New(repo).GetResidence(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for public residence referencing a private person", err)
	}
}

// TestGetResidenceVisibleToFullAccessWhenReferencedPersonPrivate: та же
// запись, но для вызывающего с полным доступом — видна.
func TestGetResidenceVisibleToFullAccessWhenReferencedPersonPrivate(t *testing.T) {
	id := models.ID("RS-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	person := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		residences: map[models.ID]*models.Residence{id: {ID: id, PersonID: person, Private: false}},
		people:     map[models.ID]*models.Person{person: {ID: person, Private: true}},
	}

	got, err := New(repo).GetResidence(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetResidence: %v", err)
	}

	if got.ID != id {
		t.Fatalf("got = %+v", got)
	}
}

// TestGetResidenceNotHiddenWhenReferencedPersonPublic: PersonID публична —
// запись не прячется ошибочно (защита от false positive).
func TestGetResidenceNotHiddenWhenReferencedPersonPublic(t *testing.T) {
	id := models.ID("RS-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	person := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		residences: map[models.ID]*models.Residence{id: {ID: id, PersonID: person, Private: false}},
		people:     map[models.ID]*models.Person{person: {ID: person, Private: false}},
	}

	got, err := New(repo).GetResidence(context.Background(), models.AccessPublic, id)
	if err != nil {
		t.Fatalf("GetResidence: %v", err)
	}

	if got.ID != id {
		t.Fatalf("got = %+v", got)
	}
}

// TestGetResidenceHiddenWhenReferencedCitationPrivate: проживание само по
// себе не приватно и PersonID публичен, но одна из Sources ссылается на
// приватную цитату — прячется как отсутствующее для вызывающего без
// полного доступа. Независимая проверка, параллельная
// TestGetResidenceHiddenWhenReferencedPersonPrivate.
func TestGetResidenceHiddenWhenReferencedCitationPrivate(t *testing.T) {
	id := models.ID("RS-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	person := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		residences: map[models.ID]*models.Residence{id: {
			ID: id, PersonID: person, Private: false,
			Sources: []models.SourceLink{{CitationID: citation}},
		}},
		people:    map[models.ID]*models.Person{person: {ID: person, Private: false}},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	_, err := New(repo).GetResidence(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for public residence referencing a private citation", err)
	}
}

// TestGetResidenceVisibleToFullAccessWhenReferencedCitationPrivate: та же
// запись, но для вызывающего с полным доступом — видна.
func TestGetResidenceVisibleToFullAccessWhenReferencedCitationPrivate(t *testing.T) {
	id := models.ID("RS-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	person := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		residences: map[models.ID]*models.Residence{id: {
			ID: id, PersonID: person, Private: false,
			Sources: []models.SourceLink{{CitationID: citation}},
		}},
		people:    map[models.ID]*models.Person{person: {ID: person, Private: false}},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: true}},
	}

	got, err := New(repo).GetResidence(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetResidence: %v", err)
	}

	if got.ID != id {
		t.Fatalf("got = %+v", got)
	}
}

// TestGetResidenceNotHiddenWhenReferencedCitationPublic: ссылка на цитату
// есть, но цитата публична — запись не прячется ошибочно (защита от false
// positive).
func TestGetResidenceNotHiddenWhenReferencedCitationPublic(t *testing.T) {
	id := models.ID("RS-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	person := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	citation := models.ID("C-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{
		residences: map[models.ID]*models.Residence{id: {
			ID: id, PersonID: person, Private: false,
			Sources: []models.SourceLink{{CitationID: citation}},
		}},
		people:    map[models.ID]*models.Person{person: {ID: person, Private: false}},
		citations: map[models.ID]*models.Citation{citation: {ID: citation, Private: false}},
	}

	got, err := New(repo).GetResidence(context.Background(), models.AccessPublic, id)
	if err != nil {
		t.Fatalf("GetResidence: %v", err)
	}

	if got.ID != id {
		t.Fatalf("got = %+v", got)
	}
}
