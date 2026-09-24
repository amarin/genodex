package get_relation

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «ребро графа родства по идентификатору».
type Scenario struct {
	relations RelationRepo
}

// New создаёт сценарий.
func New(relations RelationRepo) *Scenario {
	return &Scenario{relations: relations}
}

// GetRelation возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound. Приватная запись
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound — тот же принцип «прячем как отсутствующее», что и в
// List/Search.
func (s *Scenario) GetRelation(ctx context.Context, access models.Access, id models.ID) (models.Relation, error) {
	if err := validateID(id); err != nil {
		return models.Relation{}, err
	}

	r, err := s.relations.GetRelation(ctx, id)
	if err != nil {
		return models.Relation{}, err
	}

	if r.Private && access != models.AccessFull {
		return models.Relation{}, models.ErrNotFound
	}

	return *r, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeRelation)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeRelation,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
