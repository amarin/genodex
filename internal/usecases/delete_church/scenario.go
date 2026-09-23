package delete_church

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление записи церкви».
type Scenario struct {
	churches ChurchRepo
}

// New создаёт сценарий.
func New(churches ChurchRepo) *Scenario {
	return &Scenario{churches: churches}
}

// DeleteChurch удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteChurch(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.churches.DeleteChurch(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeChurch)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeChurch,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
