package get_given_name

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «словарная запись имени по идентификатору».
type Scenario struct {
	givenNames GivenNameRepo
}

// New создаёт сценарий.
func New(givenNames GivenNameRepo) *Scenario {
	return &Scenario{givenNames: givenNames}
}

// GetGivenName возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound.
func (s *Scenario) GetGivenName(ctx context.Context, id models.ID) (models.GivenName, error) {
	if err := validateID(id); err != nil {
		return models.GivenName{}, err
	}

	sn, err := s.givenNames.GetGivenName(ctx, id)
	if err != nil {
		return models.GivenName{}, err
	}

	return *sn, nil
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
