package get_church

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «словарная запись фамилии по идентификатору».
type Scenario struct {
	churches ChurchRepo
}

// New создаёт сценарий.
func New(churches ChurchRepo) *Scenario {
	return &Scenario{churches: churches}
}

// GetChurch возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound.
func (s *Scenario) GetChurch(ctx context.Context, id models.ID) (models.Church, error) {
	if err := validateID(id); err != nil {
		return models.Church{}, err
	}

	sn, err := s.churches.GetChurch(ctx, id)
	if err != nil {
		return models.Church{}, err
	}

	return *sn, nil
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
