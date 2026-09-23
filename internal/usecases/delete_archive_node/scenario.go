package delete_archive_node

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «удаление узла архивного дерева».
type Scenario struct {
	archiveNodes ArchiveNodeRepo
}

// New создаёт сценарий.
func New(archiveNodes ArchiveNodeRepo) *Scenario {
	return &Scenario{archiveNodes: archiveNodes}
}

// DeleteArchiveNode удаляет узел. Неверный формат идентификатора —
// *models.ValidationError (поле id), репозиторий не вызывается; нет такого
// узла — models.ErrNotFound; на узел ссылаются другие (дочерние узлы,
// документы) — *models.InUseError со списком ссылающихся (генерическая
// реализация ON DELETE RESTRICT уже в internal/store/sqlstore).
func (s *Scenario) DeleteArchiveNode(ctx context.Context, id models.ID) error {
	if err := validateID(id); err != nil {
		return err
	}

	return s.archiveNodes.DeleteArchiveNode(ctx, id)
}

// validateID проверяет формат идентификатора; ошибка — *models.ValidationError
// по полю id.
func validateID(id models.ID) error {
	err := id.Validate(models.TypeArchiveNode)
	if err == nil {
		return nil
	}

	return &models.ValidationError{
		Entity: models.TypeArchiveNode,
		Field:  "id",
		Reason: err.Error(),
		Err:    err,
	}
}
