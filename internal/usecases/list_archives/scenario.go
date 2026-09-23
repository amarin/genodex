package list_archives

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список словарных записей фамилий».
type Scenario struct {
	archives ArchiveRepo
}

// New создаёт сценарий.
func New(archives ArchiveRepo) *Scenario {
	return &Scenario{archives: archives}
}

// ListArchives возвращает записи в порядке сохранения, окном page.
// Неверные размер/сдвиг окна — *models.ValidationError (поля limit/offset).
// Короткий результат (меньше размера окна) означает конец списка.
func (s *Scenario) ListArchives(ctx context.Context, access models.Access, page models.Page) ([]models.Archive, error) {
	if page.Limit < 0 {
		return nil, &models.ValidationError{Entity: models.TypeArchive, Field: "limit", Reason: "не может быть отрицательным"}
	}

	if page.Offset < 0 {
		return nil, &models.ValidationError{Entity: models.TypeArchive, Field: "offset", Reason: "не может быть отрицательным"}
	}

	list, err := s.archives.ListArchives(ctx, access, page.Normalized())
	if err != nil {
		return nil, err
	}

	out := make([]models.Archive, 0, len(list))
	for _, sn := range list {
		out = append(out, *sn)
	}

	return out, nil
}
