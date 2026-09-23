package get_parish

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «словарная запись фамилии по идентификатору».
type Scenario struct {
	parishes ParishRepo
}

// New создаёт сценарий.
func New(parishes ParishRepo) *Scenario {
	return &Scenario{parishes: parishes}
}

// GetParish возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound.
func (s *Scenario) GetParish(ctx context.Context, id models.ID) (models.Parish, error) {
	if err := validateID(id); err != nil {
		return models.Parish{}, err
	}

	sn, err := s.parishes.GetParish(ctx, id)
	if err != nil {
		return models.Parish{}, err
	}

	return *sn, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeParish)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeParish,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
