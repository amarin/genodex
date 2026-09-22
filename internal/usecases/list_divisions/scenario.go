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
// сохранения; окно (размер и сдвиг) применяется после фильтра. С ParentID —
// только прямые дети этой единицы. Некорректный запрос — *models.ValidationError,
// репозиторий не вызывается. Короткий результат (меньше размера окна) означает
// конец списка.
func (s *Scenario) ListDivisions(ctx context.Context, q models.DivisionQuery) ([]models.AdministrativeDivision, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	if q.ParentID != nil {
		return s.listChildren(ctx, q, *q.ParentID)
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

		var full bool
		if full, matched, out = applyWindow(q, divisions, page, matched, out); full {
			return out, nil
		}
	}
}

// listChildren — ветка «дети родителя»: тот же обход окон, но по ChildrenOfDivision.
func (s *Scenario) listChildren(ctx context.Context, q models.DivisionQuery, parent models.ID) ([]models.AdministrativeDivision, error) {
	page := q.Page.Normalized()
	out := []models.AdministrativeDivision{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		divisions, err := s.adminDivisions.ChildrenOfDivision(ctx, parent, models.AccessFull,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(divisions) == 0 {
			return out, nil
		}

		var full bool
		if full, matched, out = applyWindow(q, divisions, page, matched, out); full {
			return out, nil
		}
	}
}

// applyWindow прогоняет одно окно репозитория через фильтр запроса и накапливает
// результат окна запроса. full — окно запроса заполнено (обход можно остановить).
func applyWindow(q models.DivisionQuery, divisions []*models.AdministrativeDivision,
	page models.Page, matched int, out []models.AdministrativeDivision,
) (full bool, nextMatched int, nextOut []models.AdministrativeDivision) {
	for _, d := range divisions {
		if !q.Matches(*d) {
			continue
		}

		if matched >= page.Offset {
			out = append(out, *d)

			if len(out) == page.Limit {
				return true, matched, out
			}
		}

		matched++
	}

	return false, matched, out
}
