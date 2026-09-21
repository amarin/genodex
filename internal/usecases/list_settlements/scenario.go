package list_settlements

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список населённых пунктов».
type Scenario struct {
	adminDivisions AdminDivisionRepo
}

// New создаёт сценарий.
func New(adminDivisions AdminDivisionRepo) *Scenario {
	return &Scenario{adminDivisions: adminDivisions}
}

// ListSettlements возвращает населённые пункты (единицы деления вида нас. пункта).
func (s *Scenario) ListSettlements(ctx context.Context) ([]models.AdministrativeDivision, error) {
	divisions, err := s.adminDivisions.ListAdministrativeDivisions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]models.AdministrativeDivision, 0, len(divisions))
	for _, d := range divisions {
		if d.Type.IsSettlement() {
			out = append(out, *d)
		}
	}
	return out, nil
}
