package get_archive

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «словарная запись фамилии по идентификатору».
type Scenario struct {
	archives ArchiveRepo
}

// New создаёт сценарий.
func New(archives ArchiveRepo) *Scenario {
	return &Scenario{archives: archives}
}

// GetArchive возвращает запись по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такой записи — models.ErrNotFound.
func (s *Scenario) GetArchive(ctx context.Context, id models.ID) (models.Archive, error) {
	if err := validateID(id); err != nil {
		return models.Archive{}, err
	}

	sn, err := s.archives.GetArchive(ctx, id)
	if err != nil {
		return models.Archive{}, err
	}

	return *sn, nil
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
