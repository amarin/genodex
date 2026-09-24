package search_divisions

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск единиц административного деления».
type Scenario struct {
	adminDivisions DivisionRepo
}

// New создаёт сценарий.
func New(adminDivisions DivisionRepo) *Scenario {
	return &Scenario{adminDivisions: adminDivisions}
}

// SearchDivisions находит единицы деления, названия (или варианты) которых
// начинаются с текста запроса, и возвращает их целиком (id, название, тип,
// родитель). Результат — в том порядке, в каком их отдаёт Search. Окно
// (размер и сдвиг) применяется после отбора хитов по типу деления и после
// проверки приватности: хиты других сущностей и скрытые единицы не расходуют
// окно. Пустой текст (после обрезки) — пустой результат без обращения к
// репозиторию. Хит, чья единица удалена между поиском и чтением
// (ErrNotFound), пропускается; прочие ошибки пробрасываются. access
// прокидывается в Search как получен (см. list_divisions.ListDivisions) и,
// для вызывающего без полного доступа, используется также для скрытия
// единиц, ссылающихся (через Sources[i].CitationID) на приватную цитату — у
// AdministrativeDivision нет своего Private, но приватная цитата в
// источниках прячет единицу целиком, тот же принцип, что в get_division.
func (s *Scenario) SearchDivisions(ctx context.Context, access models.Access, q models.DivisionSearchQuery) ([]models.AdministrativeDivision, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.AdministrativeDivision{}, nil
	}

	page := q.Page.Normalized()
	out := []models.AdministrativeDivision{}
	matched := 0 // сколько division-хитов прошло (для сдвига окна)
	// cache — мемоизация Private по id цитаты в рамках ОДНОГО вызова
	// SearchDivisions (не переживает вызов, не шарится между запросами).
	cache := map[models.ID]bool{}

	// репозиторий отдаёт окна хитов: обходим их до пустого или до заполнения окна.
	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.adminDivisions.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeAdministrativeDivision {
				continue
			}

			// Сначала загружаем запись и проверяем приватность — и только
			// ПОТОМ считаем сдвиг (offset), см. search_events.SearchEvents.
			got, err := s.adminDivisions.GetAdministrativeDivision(ctx, h.ID)
			if err != nil {
				if errors.Is(err, models.ErrNotFound) {
					continue
				}

				return nil, err
			}

			if access != models.AccessFull {
				hidden, err := divisionReferencesPrivateCitation(ctx, s.adminDivisions, got, cache)
				if err != nil {
					return nil, err
				}

				if hidden {
					continue
				}
			}

			if matched < page.Offset {
				matched++

				continue
			}

			out = append(out, *got)

			if len(out) == page.Limit {
				return out, nil
			}

			matched++
		}
	}
}

// divisionReferencesPrivateCitation сообщает, ссылается ли единица деления
// (через Sources[i].CitationID) хотя бы на одну приватную цитату.
// Независимая копия одноимённой функции list_divisions: пакеты сценариев в
// этом проекте самодостаточны и не делятся кодом друг с другом. cache —
// мемоизация в рамках одного вызова SearchDivisions.
func divisionReferencesPrivateCitation(ctx context.Context, repo DivisionRepo, rec *models.AdministrativeDivision, cache map[models.ID]bool) (bool, error) {
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

// citationIsPrivate сообщает, приватна ли цитата id; гонка с конкурентным
// удалением цитаты трактуется как приватность, см. одноимённую функцию в
// list_divisions.
func citationIsPrivate(ctx context.Context, repo DivisionRepo, cache map[models.ID]bool, id models.ID) (bool, error) {
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
