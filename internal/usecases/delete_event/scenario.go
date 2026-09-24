package delete_event

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление события».
type Scenario struct {
	events EventRepo
}

// New создаёт сценарий.
func New(events EventRepo) *Scenario {
	return &Scenario{events: events}
}

// DeleteEvent удаляет запись. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такой
// записи — models.ErrNotFound; на запись ссылаются другие —
// *models.InUseError со списком ссылающихся.
func (s *Scenario) DeleteEvent(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.events.DeleteEvent(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeEvent)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeEvent,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
