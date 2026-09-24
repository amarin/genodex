package get_parish

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «запись прихода по идентификатору».
type Scenario struct {
	parishes ParishRepo
}

// New создаёт сценарий.
func New(parishes ParishRepo) *Scenario {
	return &Scenario{parishes: parishes}
}

// GetParish возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound. У Parish нет своего
// Private, но запись целиком прячется как отсутствующая для вызывающего без
// полного доступа, если хотя бы один её Sources[i] ссылается на приватную
// Citation — иначе публичная запись выдаёт сам факт существования и id
// приватной цитаты (тот же принцип, что и для Relation/Event → Person, см.
// get_relation.GetRelation).
func (s *Scenario) GetParish(ctx context.Context, access models.Access, id models.ID) (models.Parish, error) {
	if err := validateID(id); err != nil {
		return models.Parish{}, err
	}

	p, err := s.parishes.GetParish(ctx, id)
	if err != nil {
		return models.Parish{}, err
	}

	if access != models.AccessFull {
		hidden, err := parishReferencesPrivateCitation(ctx, s.parishes, p)
		if err != nil {
			return models.Parish{}, err
		}

		if hidden {
			return models.Parish{}, models.ErrNotFound
		}
	}

	return *p, nil
}

// parishReferencesPrivateCitation сообщает, ссылается ли запись (через
// Sources[i].CitationID) хотя бы на одну приватную цитату.
func parishReferencesPrivateCitation(ctx context.Context, repo ParishRepo, rec *models.Parish) (bool, error) {
	for _, sl := range rec.Sources {
		c, err := repo.GetCitation(ctx, sl.CitationID)
		if err != nil {
			return false, err
		}

		if c.Private {
			return true, nil
		}
	}

	return false, nil
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
