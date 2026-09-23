package list_sources

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// SourceRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type SourceRepo interface {
	ListSources(ctx context.Context, access models.Access, page models.Page) ([]*models.Source, error)
}
