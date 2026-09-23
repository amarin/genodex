package delete_archive_node

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// ArchiveNodeRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ArchiveNodeRepo interface {
	DeleteArchiveNode(ctx context.Context, id models.ID) error
}
