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
	out := []models.AdministrativeDivision{}

	// репозиторий отдаёт окна: обходим их все до пустого
	for offset := 0; ; offset += models.MaxPageLimit {
		divisions, err := s.adminDivisions.ListAdministrativeDivisions(ctx, models.AccessFull,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(divisions) == 0 {
			return out, nil
		}

		for _, d := range divisions {
			if d.Type.IsSettlement() {
				out = append(out, *d)
			}
		}
	}
}
