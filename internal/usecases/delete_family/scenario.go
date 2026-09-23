package delete_family

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление записи рода».
type Scenario struct {
	families FamilyRepo
}

// New создаёт сценарий.
func New(families FamilyRepo) *Scenario {
	return &Scenario{families: families}
}

// DeleteFamily удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteFamily(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.families.DeleteFamily(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeFamily)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeFamily,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
