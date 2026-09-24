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
// List/Search. Тот же принцип распространяется на цитаты, на которые
// ссылается архив через Sources: если хотя бы один Sources[i].CitationID
// указывает на приватную цитату, архив целиком прячется как отсутствующее,
// даже если Private == false у самого архива — иначе публичный архив
// выдаёт факт существования и id приватной цитаты.
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

	if access != models.AccessFull {
		hidden, err := archiveReferencesPrivateCitation(ctx, s.archives, a)
		if err != nil {
			return models.Archive{}, err
		}

		if hidden {
			return models.Archive{}, models.ErrNotFound
		}
	}

	return *a, nil
}

// archiveReferencesPrivateCitation сообщает, ссылается ли архив (через
// Sources[i].CitationID) хотя бы на одну приватную цитату.
func archiveReferencesPrivateCitation(ctx context.Context, repo ArchiveRepo, rec *models.Archive) (bool, error) {
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
