package delete_given_name

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление словарной записи имени».
type Scenario struct {
	givenNames GivenNameRepo
}

// New создаёт сценарий.
func New(givenNames GivenNameRepo) *Scenario {
	return &Scenario{givenNames: givenNames}
}

// DeleteGivenName удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteGivenName(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.givenNames.DeleteGivenName(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeGivenName)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeGivenName,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
