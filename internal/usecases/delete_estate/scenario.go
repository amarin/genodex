package delete_estate

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление словарной записи сословия».
type Scenario struct {
	estates EstateRepo
}

// New создаёт сценарий.
func New(estates EstateRepo) *Scenario {
	return &Scenario{estates: estates}
}

// DeleteEstate удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteEstate(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.estates.DeleteEstate(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeEstate)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeEstate,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
