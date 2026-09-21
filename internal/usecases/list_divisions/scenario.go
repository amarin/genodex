package list_divisions

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список единиц административного деления».
type Scenario struct {
	adminDivisions AdminDivisionRepo
}

// New создаёт сценарий.
func New(adminDivisions AdminDivisionRepo) *Scenario {
	return &Scenario{adminDivisions: adminDivisions}
}

// ListDivisions возвращает единицы деления, прошедшие фильтры запроса, в порядке
// сохранения; окно (размер и сдвиг) применяется после фильтра. Некорректный запрос
// — *models.ValidationError, репозиторий не вызывается. Короткий результат
// (меньше размера окна) означает конец списка.
func (s *Scenario) ListDivisions(ctx context.Context, q models.DivisionQuery) ([]models.AdministrativeDivision, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	page := q.Page.Normalized()
	out := []models.AdministrativeDivision{}
	matched := 0 // сколько единиц прошло фильтр (для сдвига окна)

	// репозиторий отдаёт окна: обходим их до пустого или до заполнения окна запроса
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
			if !q.Matches(*d) {
				continue
			}

			if matched >= page.Offset {
				out = append(out, *d)

				if len(out) == page.Limit {
					return out, nil
				}
			}

			matched++
		}
	}
}
