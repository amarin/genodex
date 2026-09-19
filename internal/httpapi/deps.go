package httpapi

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// SettlementService — контракт сценария «список населённых пунктов».
type SettlementService interface {
	ListSettlements(ctx context.Context) ([]models.Settlement, error)
}
