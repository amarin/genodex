package list_settlements

import (
	"context"

	"github.com/amarin/genodex/internal/entity"
	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список населённых пунктов».
type Scenario struct {
	settlements SettlementRepo
}

// New собирает сценарий.
func New(settlements SettlementRepo) *Scenario {
	return &Scenario{settlements: settlements}
}

// ListSettlements возвращает все населённые пункты.
func (s *Scenario) ListSettlements(ctx context.Context) ([]models.Settlement, error) {
	settlements, err := s.settlements.ListSettlements()
	if err != nil {
		return nil, err
	}
	out := make([]models.Settlement, 0, len(settlements))
	for _, settlement := range settlements {
		out = append(out, toModel(settlement))
	}
	return out, nil
}

func toModel(settlement *entity.Settlement) models.Settlement {
	return models.Settlement{
		ID:       settlement.ID,
		Name:     settlement.Name,
		Metadata: settlement.Metadata,
	}
}
