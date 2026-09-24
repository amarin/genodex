package get_family

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «запись рода по идентификатору».
type Scenario struct {
	families FamilyRepo
}

// New создаёт сценарий.
func New(families FamilyRepo) *Scenario {
	return &Scenario{families: families}
}

// GetFamily возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound. Приватная запись
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound — тот же принцип «прячем как отсутствующее», что и в
// List/Search.
func (s *Scenario) GetFamily(ctx context.Context, access models.Access, id models.ID) (models.Family, error) {
	if err := validateID(id); err != nil {
		return models.Family{}, err
	}

	f, err := s.families.GetFamily(ctx, id)
	if err != nil {
		return models.Family{}, err
	}

	if f.Private && access != models.AccessFull {
		return models.Family{}, models.ErrNotFound
	}

	if access != models.AccessFull {
		hidden, err := familyReferencesPrivateCitation(ctx, s.families, f)
		if err != nil {
			return models.Family{}, err
		}

		if hidden {
			return models.Family{}, models.ErrNotFound
		}
	}

	return *f, nil
}

// familyReferencesPrivateCitation сообщает, ссылается ли род (через
// Sources[i].CitationID) хотя бы на одну приватную цитату — тот же принцип,
// что и собственный Private рода (см. комментарий GetFamily): публичный род,
// ссылающийся на приватную цитату, тоже прячется как отсутствующий, иначе он
// выдаёт сам факт существования и id приватной цитаты.
func familyReferencesPrivateCitation(ctx context.Context, repo FamilyRepo, f *models.Family) (bool, error) {
	for _, sl := range f.Sources {
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
	err := id.Validate(models.TypeFamily)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeFamily,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
