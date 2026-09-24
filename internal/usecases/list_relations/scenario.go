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

		full, nextMatched, nextOut, err := applyWindow(ctx, s.relations, access, q, list, page, matched, out)
		if err != nil {
			return nil, err
		}

		matched, out = nextMatched, nextOut
		if full {
			return out, nil
		}
	}
}

// applyWindow прогоняет одно окно репозитория через фильтр запроса и
// накапливает результат окна запроса. full — окно запроса заполнено (обход
// можно остановить).
//
// Помимо q.PersonID, окно фильтрует рёбра, ссылающиеся (PersonA/PersonB) на
// приватную персону: для access != models.AccessFull такое ребро прячется,
// даже если само оно не приватно (см. комментарий GetRelation). Это стоит
// одного дополнительного GetPerson на ребро, попавшее в окно — приемлемо
// при масштабе этого хранилища (локальный SQLite, не высоконагруженный
// веб-сервис); тот же компромисс «без кеша/батчинга», что и в
// create_event для проверки участников.
func applyWindow(ctx context.Context, repo RelationRepo, access models.Access, q models.RelationQuery, list []*models.Relation,
	page models.Page, matched int, out []models.Relation,
) (full bool, nextMatched int, nextOut []models.Relation, err error) {
	for _, r := range list {
		if !matchesPerson(q.PersonID, r.PersonA, r.PersonB) {
			continue
		}

		if access != models.AccessFull {
			hidden, err := relationReferencesPrivatePerson(ctx, repo, r)
			if err != nil {
				return false, matched, out, err
			}

			if hidden {
				continue
			}
		}

		if matched >= page.Offset {
			out = append(out, *r)

			if len(out) == page.Limit {
				return true, matched, out, nil
			}
		}

		matched++
	}

	return false, matched, out, nil
}

// relationReferencesPrivatePerson сообщает, ссылается ли ребро (через
// PersonA или PersonB) на приватную персону — обе стороны проверяются
// независимо. Независимая копия одноимённой функции get_relation: пакеты
// сценариев в этом проекте самодостаточны и не делятся кодом друг с
// другом.
func relationReferencesPrivatePerson(ctx context.Context, repo RelationRepo, r *models.Relation) (bool, error) {
	a, err := repo.GetPerson(ctx, r.PersonA)
	if err != nil {
		return false, err
	}

	if a.Private {
		return true, nil
	}

	b, err := repo.GetPerson(ctx, r.PersonB)
	if err != nil {
		return false, err
	}

	return b.Private, nil
}

// matchesPerson сообщает, проходит ли ребро фильтр по персоне: nil — без
// фильтра (проходит всё); иначе ребро проходит, если персона совпадает с
// одной из его сторон.
func matchesPerson(want *models.ID, a, b models.ID) bool {
	return want == nil || *want == a || *want == b
}
