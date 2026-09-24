package update_relation

import (
	"context"

	"github.com/amarin/genodex/internal/store"
)

// RelationStore — зависимость сценария: транзакция порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type RelationStore interface {
	InTx(ctx context.Context, fn func(store.Store) error) error
}
