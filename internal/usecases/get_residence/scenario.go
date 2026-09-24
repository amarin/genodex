package get_residence

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «проживание по идентификатору».
type Scenario struct {
	residences ResidenceRepo
}

// New создаёт сценарий.
func New(residences ResidenceRepo) *Scenario {
	return &Scenario{residences: residences}
}

// GetResidence возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound. Приватная запись
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound. Тот же принцип распространяется на персону, на
// которую ссылается запись: приватный PersonID прячет проживание целиком,
// даже если Private == false у самой записи (docs/data-model/
// entity-write.md §3.1).
func (s *Scenario) GetResidence(ctx context.Context, access models.Access, id models.ID) (models.Residence, error) {
	if err := validateID(id); err != nil {
		return models.Residence{}, err
	}

	r, err := s.residences.GetResidence(ctx, id)
	if err != nil {
		return models.Residence{}, err
	}

	if r.Private && access != models.AccessFull {
		return models.Residence{}, models.ErrNotFound
	}

	if access != models.AccessFull {
		hidden, err := residenceReferencesPrivatePerson(ctx, s.residences, r)
		if err != nil {
			return models.Residence{}, err
		}

		if hidden {
			return models.Residence{}, models.ErrNotFound
		}
	}

	if access != models.AccessFull {
		hidden, err := residenceReferencesPrivateCitation(ctx, s.residences, r)
		if err != nil {
			return models.Residence{}, err
		}

		if hidden {
			return models.Residence{}, models.ErrNotFound
		}
	}

	return *r, nil
}

// residenceReferencesPrivatePerson сообщает, ссылается ли проживание
// (через PersonID) на приватную персону.
func residenceReferencesPrivatePerson(ctx context.Context, repo ResidenceRepo, r *models.Residence) (bool, error) {
	p, err := repo.GetPerson(ctx, r.PersonID)
	if err != nil {
		return false, err
	}

	return p.Private, nil
}

// residenceReferencesPrivateCitation сообщает, ссылается ли проживание
// (через Sources[i].CitationID) хотя бы на одну приватную цитату —
// независимая проверка, параллельная residenceReferencesPrivatePerson (см.
// её комментарий и комментарий GetResidence).
func residenceReferencesPrivateCitation(ctx context.Context, repo ResidenceRepo, r *models.Residence) (bool, error) {
	for _, sl := range r.Sources {
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
	err := id.Validate(models.TypeResidence)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeResidence,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
