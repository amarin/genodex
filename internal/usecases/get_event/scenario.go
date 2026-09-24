package get_event

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «событие по идентификатору».
type Scenario struct {
	events EventRepo
}

// New создаёт сценарий.
func New(events EventRepo) *Scenario {
	return &Scenario{events: events}
}

// GetEvent возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound. Приватная запись
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound.
func (s *Scenario) GetEvent(ctx context.Context, access models.Access, id models.ID) (models.Event, error) {
	if err := validateID(id); err != nil {
		return models.Event{}, err
	}

	e, err := s.events.GetEvent(ctx, id)
	if err != nil {
		return models.Event{}, err
	}

	if e.Private && access != models.AccessFull {
		return models.Event{}, models.ErrNotFound
	}

	return *e, nil
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
