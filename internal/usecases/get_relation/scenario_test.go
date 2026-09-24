package get_relation

import (
	"context"
	"errors"
	"testing"

	"github.com/amarin/genodex/internal/models"
)

type fakeRepo struct {
	relations map[models.ID]*models.Relation
}

func (f *fakeRepo) GetRelation(_ context.Context, id models.ID) (*models.Relation, error) {
	r, ok := f.relations[id]
	if !ok {
		return nil, models.ErrNotFound
	}

	return r, nil
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
