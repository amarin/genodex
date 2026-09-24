package list_residences

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список проживаний».
type Scenario struct {
	residences ResidenceRepo
}

// New создаёт сценарий.
func New(residences ResidenceRepo) *Scenario {
	return &Scenario{residences: residences}
}

// ListResidences возвращает проживания, прошедшие фильтры запроса
// (q.PersonID/q.PlaceID, если заданы — пересекаются), в порядке сохранения;
// окно применяется после фильтра. Некорректный запрос —
// *models.ValidationError, репозиторий не вызывается. Короткий результат
// (меньше размера окна) означает конец списка.
//
// У Residence нет собственных поисковых полей (search-индекс намеренно пуст,
// docs/data-model/entity-write.md §3.8) — этот фильтр заменяет
// search_residences, которого в этой программе нет. Полное сканирование по
// generic-окнам ListResidences — тот же приём, что и list_relations/
// list_archive_nodes.
func (s *Scenario) ListResidences(ctx context.Context, access models.Access, q models.ResidenceQuery) ([]models.Residence, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	page := q.Page.Normalized()
	out := []models.Residence{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		list, err := s.residences.ListResidences(ctx, access, models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(list) == 0 {
			return out, nil
		}

		var full bool
		if full, matched, out = applyWindow(q, list, page, matched, out); full {
			return out, nil
		}
	}
}

// applyWindow прогоняет одно окно репозитория через фильтр запроса и
// накапливает результат окна запроса. full — окно запроса заполнено (обход
// можно остановить).
func applyWindow(q models.ResidenceQuery, list []*models.Residence,
	page models.Page, matched int, out []models.Residence,
) (full bool, nextMatched int, nextOut []models.Residence) {
	for _, r := range list {
		if q.PersonID != nil && *q.PersonID != r.PersonID {
			continue
		}

		if q.PlaceID != nil && *q.PlaceID != r.PlaceID {
			continue
		}

		if matched >= page.Offset {
			out = append(out, *r)

			if len(out) == page.Limit {
				return true, matched, out
			}
		}

		matched++
	}

	return false, matched, out
}
