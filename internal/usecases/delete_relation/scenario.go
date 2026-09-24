package delete_relation

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление ребра графа родства».
type Scenario struct {
	relations RelationRepo
}

// New создаёт сценарий.
func New(relations RelationRepo) *Scenario {
	return &Scenario{relations: relations}
}

// DeleteRelation удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteRelation(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.relations.DeleteRelation(ctx, id)
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
