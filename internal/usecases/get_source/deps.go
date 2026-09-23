package get_source

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// SourceRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type SourceRepo interface {
	GetSource(ctx context.Context, id models.ID) (*models.Source, error)
}
