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
// models.ErrNotFound. Тот же принцип распространяется на участников: если
// хотя бы один Participants[i].PersonID приватен, событие целиком прячется
// как отсутствующее, даже если Private == false у самого события
// (docs/data-model/entity-write.md §3.1).
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

	if access != models.AccessFull {
		hidden, err := eventReferencesPrivatePerson(ctx, s.events, e)
		if err != nil {
			return models.Event{}, err
		}

		if hidden {
			return models.Event{}, models.ErrNotFound
		}
	}

	return *e, nil
}

// eventReferencesPrivatePerson сообщает, ссылается ли событие (через
// участников Participants[i].PersonID) хотя бы на одну приватную персону —
// каждый участник проверяется независимо.
func eventReferencesPrivatePerson(ctx context.Context, repo EventRepo, e *models.Event) (bool, error) {
	for _, p := range e.Participants {
		person, err := repo.GetPerson(ctx, p.PersonID)
		if err != nil {
			return false, err
		}

		if person.Private {
			return true, nil
		}
	}

	return false, nil
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
