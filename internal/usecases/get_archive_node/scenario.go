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
// List/Search (docs/data-model/entity-write.md §3.1). Тот же принцип
// распространяется на цитаты, на которые ссылается узел через Sources:
// если хотя бы один Sources[i].CitationID указывает на приватную цитату,
// узел целиком прячется как отсутствующее, даже если Private == false у
// самого узла — иначе публичный узел выдаёт факт существования и id
// приватной цитаты.
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

	if access != models.AccessFull {
		hidden, err := archiveNodeReferencesPrivateCitation(ctx, s.archiveNodes, n)
		if err != nil {
			return models.ArchiveNode{}, err
		}

		if hidden {
			return models.ArchiveNode{}, models.ErrNotFound
		}
	}

	return *n, nil
}

// archiveNodeReferencesPrivateCitation сообщает, ссылается ли узел (через
// Sources[i].CitationID) хотя бы на одну приватную цитату.
func archiveNodeReferencesPrivateCitation(ctx context.Context, repo ArchiveNodeRepo, rec *models.ArchiveNode) (bool, error) {
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
