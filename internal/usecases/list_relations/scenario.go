package list_relations

import (
	"context"
	"errors"

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
	// cache — мемоизация Private по id персоны в рамках ОДНОГО вызова
	// ListRelations (не переживает вызов, не шарится между запросами): одна
	// и та же персона часто встречается в нескольких рёбрах одного скана.
	cache := map[models.ID]bool{}

	for offset := 0; ; offset += models.MaxPageLimit {
		list, err := s.relations.ListRelations(ctx, access, models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(list) == 0 {
			return out, nil
		}

		full, nextMatched, nextOut, err := applyWindow(ctx, s.relations, access, q, list, page, matched, out, cache)
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
// до двух дополнительных GetPerson на ребро, попавшее в окно (по одному на
// PersonA и PersonB; меньше при попадании в cache) — приемлемо при масштабе
// этого хранилища (локальный SQLite, не высоконагруженный веб-сервис); тот
// же компромисс «без батчинга», что и в create_event для проверки
// участников.
func applyWindow(ctx context.Context, repo RelationRepo, access models.Access, q models.RelationQuery, list []*models.Relation,
	page models.Page, matched int, out []models.Relation, cache map[models.ID]bool,
) (full bool, nextMatched int, nextOut []models.Relation, err error) {
	for _, r := range list {
		if !matchesPerson(q.PersonID, r.PersonA, r.PersonB) {
			continue
		}

		if access != models.AccessFull {
			hidden, err := relationReferencesPrivatePerson(ctx, repo, r, cache)
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
// независимо. cache — мемоизация в рамках одного вызова ListRelations, см.
// её объявление в ListRelations. Независимая копия одноимённой функции
// get_relation (та не использует cache и не глотает ErrNotFound — см. её
// комментарий): пакеты сценариев в этом проекте самодостаточны и не делятся
// кодом друг с другом.
func relationReferencesPrivatePerson(ctx context.Context, repo RelationRepo, r *models.Relation, cache map[models.ID]bool) (bool, error) {
	hiddenA, err := personIsPrivate(ctx, repo, cache, r.PersonA)
	if err != nil {
		return false, err
	}

	if hiddenA {
		return true, nil
	}

	return personIsPrivate(ctx, repo, cache, r.PersonB)
}

// personIsPrivate сообщает, приватна ли персона id — с точки зрения
// сканирующего ListRelations сюда же относится и гонка с конкурентным
// удалением персоны (models.ErrNotFound от GetPerson): такая персона
// трактуется как приватная, т.е. ребро, ссылающееся на неё, тоже прячется,
// а не проваливает весь список ошибкой (в отличие от get_relation — там
// единичный неожиданный сбой GetPerson информативнее как ошибка, а не как
// «запись не найдена»). cache — мемоизация в рамках одного вызова.
func personIsPrivate(ctx context.Context, repo RelationRepo, cache map[models.ID]bool, id models.ID) (bool, error) {
	if v, ok := cache[id]; ok {
		return v, nil
	}

	p, err := repo.GetPerson(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			cache[id] = true

			return true, nil
		}

		return false, err
	}

	cache[id] = p.Private

	return p.Private, nil
}

// matchesPerson сообщает, проходит ли ребро фильтр по персоне: nil — без
// фильтра (проходит всё); иначе ребро проходит, если персона совпадает с
// одной из его сторон.
func matchesPerson(want *models.ID, a, b models.ID) bool {
	return want == nil || *want == a || *want == b
}
