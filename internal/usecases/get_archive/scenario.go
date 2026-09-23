package get_archive

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «запись архива по идентификатору».
type Scenario struct {
	archives ArchiveRepo
}

// New создаёт сценарий.
func New(archives ArchiveRepo) *Scenario {
	return &Scenario{archives: archives}
}

// GetArchive возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound. Приватная запись
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound — тот же принцип «прячем как отсутствующее», что и в
// List/Search.
func (s *Scenario) GetArchive(ctx context.Context, access models.Access, id models.ID) (models.Archive, error) {
	if err := validateID(id); err != nil {
		return models.Archive{}, err
	}

	a, err := s.archives.GetArchive(ctx, id)
	if err != nil {
		return models.Archive{}, err
	}

	if a.Private && access != models.AccessFull {
		return models.Archive{}, models.ErrNotFound
	}

	return *a, nil
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeArchive)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeArchive,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
