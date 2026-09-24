package update_residence

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// ResidenceStore — зависимость сценария: транзакция порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type ResidenceStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
