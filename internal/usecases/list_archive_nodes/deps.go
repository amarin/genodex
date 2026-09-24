package list_archive_nodes

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ArchiveNodeRepo — зависимость сценария: срез порта store.Store. В отличие
// от AdministrativeDivision, у ArchiveNode нет отдельного метода «дети
// узла» — фильтрация по архиву и родителю идёт полным сканированием
// ListArchiveNodes (см. scenario.go).
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveNodeRepo interface {
	ListArchiveNodes(ctx context.Context, access models.Access, page models.Page) ([]*models.ArchiveNode, error)
	GetCitation(ctx context.Context, id models.ID) (*models.Citation, error)
}
