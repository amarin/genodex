package get_division

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «единица административного деления по идентификатору».
type Scenario struct {
	divisions DivisionRepo
}

// New создаёт сценарий.
func New(divisions DivisionRepo) *Scenario {
	return &Scenario{divisions: divisions}
}

// GetDivision возвращает единицу деления по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой единицы — models.ErrNotFound. У AdministrativeDivision
// нет своего Private, но единица целиком прячется как отсутствующая для
// вызывающего без полного доступа, если хотя бы один её Sources[i]
// ссылается на приватную Citation — иначе публичная единица выдаёт сам факт
// существования и id приватной цитаты (тот же принцип, что и для
// Relation/Event → Person, см. get_relation.GetRelation).
func (s *Scenario) GetDivision(ctx context.Context, access models.Access, id models.ID) (models.AdministrativeDivision, error) {
	if err := validateID(id); err != nil {
		return models.AdministrativeDivision{}, err
	}

	d, err := s.divisions.GetAdministrativeDivision(ctx, id)
	if err != nil {
		return models.AdministrativeDivision{}, err
	}

	if access != models.AccessFull {
		hidden, err := divisionReferencesPrivateCitation(ctx, s.divisions, d)
		if err != nil {
			return models.AdministrativeDivision{}, err
		}

		if hidden {
			return models.AdministrativeDivision{}, models.ErrNotFound
		}
	}

	return *d, nil
}

// divisionReferencesPrivateCitation сообщает, ссылается ли единица деления
// (через Sources[i].CitationID) хотя бы на одну приватную цитату.
func divisionReferencesPrivateCitation(ctx context.Context, repo DivisionRepo, rec *models.AdministrativeDivision) (bool, error) {
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

// validateID проверяет формат идентификатора деления; ошибка —
// *models.ValidationError по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeAdministrativeDivision)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeAdministrativeDivision,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
