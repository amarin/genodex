package list_relations

import (
	"context"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список рёбер графа родства».
type Scenario struct {
	relations RelationRepo
}

// New создаёт сценарий.
func New(relations RelationRepo) *Scenario {
	return &Scenario{relations: relations}
}

// ListRelations возвращает рёбра, прошедшие фильтр запроса (q.PersonID,
// если задан — ребро проходит, если совпадает с PersonA ИЛИ PersonB), в
// порядке сохранения; окно применяется после фильтра. Некорректный запрос —
// *models.ValidationError, репозиторий не вызывается. Короткий результат
// (меньше размера окна) означает конец списка.
//
// У Relation нет собственных поисковых полей (search-индекс намеренно пуст,
// docs/data-model/entity-write.md §3.8) — этот фильтр заменяет
// search_relations, которого в этой программе нет. Полное сканирование по
// generic-окнам ListRelations — тот же приём, что и list_archive_nodes
// (подпроект 6): выделенный метод хранилища не оправдан при текущем объёме
// данных.
func (s *Scenario) ListRelations(ctx context.Context, access models.Access, q models.RelationQuery) ([]models.Relation, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	page := q.Page.Normalized()
	out := []models.Relation{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		list, err := s.relations.ListRelations(ctx, access, models.Page{Limit: models.MaxPageLimit, Offset: offset})
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
func applyWindow(q models.RelationQuery, list []*models.Relation,
	page models.Page, matched int, out []models.Relation,
) (full bool, nextMatched int, nextOut []models.Relation) {
	for _, r := range list {
		if !matchesPerson(q.PersonID, r.PersonA, r.PersonB) {
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

// matchesPerson сообщает, проходит ли ребро фильтр по персоне: nil — без
// фильтра (проходит всё); иначе ребро проходит, если персона совпадает с
// одной из его сторон.
func matchesPerson(want *models.ID, a, b models.ID) bool {
	return want == nil || *want == a || *want == b
}
