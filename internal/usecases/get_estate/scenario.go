package get_estate

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «словарная запись фамилии по идентификатору».
type Scenario struct {
	estates EstateRepo
}

// New создаёт сценарий.
func New(estates EstateRepo) *Scenario {
	return &Scenario{estates: estates}
}

// GetEstate возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound.
func (s *Scenario) GetEstate(ctx context.Context, id models.ID) (models.Estate, error) {
	if err := validateID(id); err != nil {
		return models.Estate{}, err
	}

	sn, err := s.estates.GetEstate(ctx, id)
	if err != nil {
		return models.Estate{}, err
	}

	return *sn, nil
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
