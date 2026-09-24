package get_church

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «запись церкви по идентификатору».
type Scenario struct {
	churches ChurchRepo
}

// New создаёт сценарий.
func New(churches ChurchRepo) *Scenario {
	return &Scenario{churches: churches}
}

// GetChurch возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound. У Church нет своего
// Private, но запись целиком прячется как отсутствующая для вызывающего без
// полного доступа, если хотя бы один её Sources[i] ссылается на приватную
// Citation — иначе публичная запись выдаёт сам факт существования и id
// приватной цитаты (тот же принцип, что и для Relation/Event → Person, см.
// get_relation.GetRelation).
func (s *Scenario) GetChurch(ctx context.Context, access models.Access, id models.ID) (models.Church, error) {
	if err := validateID(id); err != nil {
		return models.Church{}, err
	}

	c, err := s.churches.GetChurch(ctx, id)
	if err != nil {
		return models.Church{}, err
	}

	if access != models.AccessFull {
		hidden, err := churchReferencesPrivateCitation(ctx, s.churches, c)
		if err != nil {
			return models.Church{}, err
		}

		if hidden {
			return models.Church{}, models.ErrNotFound
		}
	}

	return *c, nil
}

// churchReferencesPrivateCitation сообщает, ссылается ли запись (через
// Sources[i].CitationID) хотя бы на одну приватную цитату.
func churchReferencesPrivateCitation(ctx context.Context, repo ChurchRepo, rec *models.Church) (bool, error) {
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
