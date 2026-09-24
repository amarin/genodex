package get_relation

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	relations map[models.ID]*models.Relation
	people    map[models.ID]*models.Person
	personErr error // если задан, GetPerson всегда возвращает эту ошибку
}

func (f *fakeRepo) GetRelation(_ context.Context, id models.ID) (*models.Relation, error) {
	r, ok := f.relations[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return r, nil
}

func (f *fakeRepo) GetPerson(_ context.Context, id models.ID) (*models.Person, error) {
	if f.personErr != nil {
		return nil, f.personErr
	}

	p, ok := f.people[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return p, nil
}

func TestGetRelationReturnsRecord(t *testing.T) {
	id := models.ID("RL-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{relations: map[models.ID]*models.Relation{id: {ID: id, Kind: models.RelationKindBlood}}}

	got, err := New(repo).GetRelation(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetRelation: %v", err)
	}

	if got.Kind != models.RelationKindBlood {
		t.Fatalf("Kind = %q", got.Kind)
	}
}

func TestGetRelationNotFound(t *testing.T) {
	repo := &fakeRepo{relations: map[models.ID]*models.Relation{}}

	_, err := New(repo).GetRelation(context.Background(), models.AccessFull, "RL-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestGetRelationInvalidID(t *testing.T) {
	repo := &fakeRepo{relations: map[models.ID]*models.Relation{}}

	_, err := New(repo).GetRelation(context.Background(), models.AccessFull, "bogus")

	var ve *models.ValidationError
	if !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("err = %v, want ValidationError on id", err)
	}
}

func TestGetRelationPrivateHiddenFromPublic(t *testing.T) {
	id := models.ID("RL-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{relations: map[models.ID]*models.Relation{id: {ID: id, Private: true}}}

	_, err := New(repo).GetRelation(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for private record with AccessPublic", err)
	}
}

func TestGetRelationPrivateVisibleToFullAccess(t *testing.T) {
	id := models.ID("RL-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	repo := &fakeRepo{relations: map[models.ID]*models.Relation{id: {ID: id, Private: true}}}

	got, err := New(repo).GetRelation(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetRelation: %v", err)
	}

	if !got.Private {
		t.Fatalf("Private = %v", got.Private)
	}
}

// TestGetRelationHiddenWhenReferencedPersonPrivate: ребро само по себе не
// приватно (Private == false), но PersonA приватна — ребро всё равно
// прячется как отсутствующее для вызывающего без полного доступа.
func TestGetRelationHiddenWhenReferencedPersonPrivate(t *testing.T) {
	id := models.ID("RL-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	personA := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	personB := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		relations: map[models.ID]*models.Relation{id: {ID: id, PersonA: personA, PersonB: personB, Private: false}},
		people: map[models.ID]*models.Person{
			personA: {ID: personA, Private: true},
			personB: {ID: personB, Private: false},
		},
	}

	_, err := New(repo).GetRelation(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound for public relation referencing a private person", err)
	}
}

// TestGetRelationVisibleToFullAccessWhenReferencedPersonPrivate: то же
// ребро, но для вызывающего с полным доступом — видно.
func TestGetRelationVisibleToFullAccessWhenReferencedPersonPrivate(t *testing.T) {
	id := models.ID("RL-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	personA := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	personB := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		relations: map[models.ID]*models.Relation{id: {ID: id, PersonA: personA, PersonB: personB, Private: false}},
		people: map[models.ID]*models.Person{
			personA: {ID: personA, Private: true},
			personB: {ID: personB, Private: false},
		},
	}

	got, err := New(repo).GetRelation(context.Background(), models.AccessFull, id)
	if err != nil {
		t.Fatalf("GetRelation: %v", err)
	}

	if got.ID != id {
		t.Fatalf("got = %+v", got)
	}
}

// TestGetRelationHiddenWhenOtherReferencedPersonPrivate: PersonA публична,
// PersonB приватна — ребро прячется тоже, обе стороны проверяются
// независимо (не только первая).
func TestGetRelationHiddenWhenOtherReferencedPersonPrivate(t *testing.T) {
	id := models.ID("RL-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	personA := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	personB := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		relations: map[models.ID]*models.Relation{id: {ID: id, PersonA: personA, PersonB: personB, Private: false}},
		people: map[models.ID]*models.Person{
			personA: {ID: personA, Private: false},
			personB: {ID: personB, Private: true},
		},
	}

	_, err := New(repo).GetRelation(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound when PersonB is private", err)
	}
}

// TestGetRelationNotHiddenWhenReferencedPeoplePublic: обе персоны
// публичны — ребро не прячется ошибочно (защита от false positive).
func TestGetRelationNotHiddenWhenReferencedPeoplePublic(t *testing.T) {
	id := models.ID("RL-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	personA := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	personB := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	repo := &fakeRepo{
		relations: map[models.ID]*models.Relation{id: {ID: id, PersonA: personA, PersonB: personB, Private: false}},
		people: map[models.ID]*models.Person{
			personA: {ID: personA, Private: false},
			personB: {ID: personB, Private: false},
		},
	}

	got, err := New(repo).GetRelation(context.Background(), models.AccessPublic, id)
	if err != nil {
		t.Fatalf("GetRelation: %v", err)
	}

	if got.ID != id {
		t.Fatalf("got = %+v", got)
	}
}

// TestGetRelationPropagatesNonNotFoundPersonError: единичный Get-путь (в
// отличие от List*/Search*, см. Fix 2 ревью) НЕ трактует произвольный сбой
// GetPerson как «персона не найдена» — генуинная неожиданная ошибка
// хранилища (не models.ErrNotFound) должна честно распространиться как
// ошибка вызова, а не молча превратиться в models.ErrNotFound (что скрыло
// бы от вызывающего разницу между «ребро приватно/ссылается на приватную
// персону» и «хранилище сейчас недоступно»).
func TestGetRelationPropagatesNonNotFoundPersonError(t *testing.T) {
	id := models.ID("RL-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	personA := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA1")
	personB := models.ID("I-01ARZ3NDEKTSV4RRFFQ69G5FA2")
	wantErr := errors.New("store unavailable")
	repo := &fakeRepo{
		relations: map[models.ID]*models.Relation{id: {ID: id, PersonA: personA, PersonB: personB, Private: false}},
		personErr: wantErr,
	}

	_, err := New(repo).GetRelation(context.Background(), models.AccessPublic, id)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v (не должна превращаться в ErrNotFound)", err, wantErr)
	}

	if errors.Is(err, models.ErrNotFound) {
		t.Fatalf("err = %v, must NOT be treated as ErrNotFound", err)
	}
}
