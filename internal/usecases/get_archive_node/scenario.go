package get_archive_node

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «узел архивного дерева по идентификатору».
type Scenario struct {
	archiveNodes ArchiveNodeRepo
}

// New создаёт сценарий.
func New(archiveNodes ArchiveNodeRepo) *Scenario {
	return &Scenario{archiveNodes: archiveNodes}
}

// GetArchiveNode возвращает узел по идентификатору. Неверный формат
// идентификатора — *models.ValidationError (поле id), репозиторий не
// вызывается; нет такого узла — models.ErrNotFound. Приватный узел
// (Private == true) для вызывающего без полного доступа тоже отдаётся как
// models.ErrNotFound — тот же принцип «прячем как отсутствующее», что и в
// List/Search (docs/data-model/entity-write.md §3.1).
func (s *Scenario) GetArchiveNode(ctx context.Context, access models.Access, id models.ID) (models.ArchiveNode, error) {
	if err := validateID(id); err != nil {
		return models.ArchiveNode{}, err
	}

	n, err := s.archiveNodes.GetArchiveNode(ctx, id)
	if err != nil {
		return models.ArchiveNode{}, err
	}

	if n.Private && access != models.AccessFull {
		return models.ArchiveNode{}, models.ErrNotFound
	}

	return *n, nil
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
