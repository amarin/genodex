package list_settlements

import "github.com/amarin/genodex/internal/entity"

// SettlementRepo — зависимость сценария: срез порта store.Store.
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package ${GOPACKAGE}
type SettlementRepo interface {
	ListSettlements() ([]*entity.Settlement, error)
}
