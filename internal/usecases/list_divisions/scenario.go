package list_divisions

import (
	"context"
	"errors"

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
// конец списка. access прокидывается в репозиторий как получен и, для
// вызывающего без полного доступа, используется также для скрытия единиц,
// ссылающихся (через Sources[i].CitationID) на приватную цитату — у
// AdministrativeDivision нет своего Private, но приватная цитата в
// источниках прячет единицу целиком, тот же принцип, что в get_division.
func (s *Scenario) ListDivisions(ctx context.Context, access models.Access, q models.DivisionQuery) ([]models.AdministrativeDivision, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	// cache — мемоизация Private по id цитаты в рамках ОДНОГО вызова
	// ListDivisions (не переживает вызов, не шарится между запросами).
	cache := map[models.ID]bool{}

	if q.ParentID != nil {
		return s.listChildren(ctx, access, q, *q.ParentID, cache)
	}

	page := q.Page.Normalized()
	out := []models.AdministrativeDivision{}
	matched := 0 // сколько единиц прошло фильтр (для сдвига окна)

	// репозиторий отдаёт окна: обходим их до пустого или до заполнения окна запроса
	for offset := 0; ; offset += models.MaxPageLimit {
		divisions, err := s.adminDivisions.ListAdministrativeDivisions(ctx, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(divisions) == 0 {
			return out, nil
		}

		var full bool
		if full, matched, out, err = applyWindow(ctx, s.adminDivisions, access, q, divisions, page, matched, out, cache); err != nil {
			return nil, err
		} else if full {
			return out, nil
		}
	}
}

// listChildren — ветка «дети родителя»: тот же обход окон, но по ChildrenOfDivision.
func (s *Scenario) listChildren(ctx context.Context, access models.Access, q models.DivisionQuery, parent models.ID, cache map[models.ID]bool) ([]models.AdministrativeDivision, error) {
	page := q.Page.Normalized()
	out := []models.AdministrativeDivision{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		divisions, err := s.adminDivisions.ChildrenOfDivision(ctx, parent, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(divisions) == 0 {
			return out, nil
		}

		var full bool
		if full, matched, out, err = applyWindow(ctx, s.adminDivisions, access, q, divisions, page, matched, out, cache); err != nil {
			return nil, err
		} else if full {
			return out, nil
		}
	}
}

// applyWindow прогоняет одно окно репозитория через фильтр запроса и накапливает
// результат окна запроса. full — окно запроса заполнено (обход можно остановить).
// Для вызывающего без полного доступа единица, ссылающаяся на приватную
// цитату среди источников, пропускается тем же образом, что и не прошедшая
// q.Matches — не расходует окно запроса.
func applyWindow(ctx context.Context, repo AdminDivisionRepo, access models.Access, q models.DivisionQuery, divisions []*models.AdministrativeDivision,
	page models.Page, matched int, out []models.AdministrativeDivision, cache map[models.ID]bool,
) (full bool, nextMatched int, nextOut []models.AdministrativeDivision, err error) {
	for _, d := range divisions {
		if !q.Matches(*d) {
			continue
		}

		if access != models.AccessFull {
			hidden, err := divisionReferencesPrivateCitation(ctx, repo, d, cache)
			if err != nil {
				return false, matched, out, err
			}

			if hidden {
				continue
			}
		}

		if matched >= page.Offset {
			out = append(out, *d)

			if len(out) == page.Limit {
				return true, matched, out, nil
			}
		}

		matched++
	}

	return false, matched, out, nil
}

// divisionReferencesPrivateCitation сообщает, ссылается ли единица деления
// (через Sources[i].CitationID) хотя бы на одну приватную цитату. cache —
// мемоизация в рамках одного вызова ListDivisions, см. её объявление там.
func divisionReferencesPrivateCitation(ctx context.Context, repo AdminDivisionRepo, rec *models.AdministrativeDivision, cache map[models.ID]bool) (bool, error) {
	for _, sl := range rec.Sources {
		hidden, err := citationIsPrivate(ctx, repo, cache, sl.CitationID)
		if err != nil {
			return false, err
		}

		if hidden {
			return true, nil
		}
	}

	return false, nil
}

// citationIsPrivate сообщает, приватна ли цитата id — гонка с конкурентным
// удалением цитаты (models.ErrNotFound от GetCitation) трактуется как
// приватность (запись, ссылающаяся на неё, тоже прячется, а не проваливает
// весь вызов ошибкой), тот же принцип, что и personIsPrivate в
// search_events.
func citationIsPrivate(ctx context.Context, repo AdminDivisionRepo, cache map[models.ID]bool, id models.ID) (bool, error) {
	if v, ok := cache[id]; ok {
		return v, nil
	}

	c, err := repo.GetCitation(ctx, id)
	if err != nil {
		if errors.Is(err, models.ErrNotFound) {
			cache[id] = true

			return true, nil
		}

		return false, err
	}

	cache[id] = c.Private

	return c.Private, nil
}
