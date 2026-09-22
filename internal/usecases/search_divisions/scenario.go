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
// (размер и сдвиг) применяется после отбора хитов по типу деления: хиты других
// сущностей не расходуют окно. Пустой текст (после обрезки) — пустой результат
// без обращения к репозиторию. Хит, чья единица удалена между поиском и чтением
// (ErrNotFound), пропускается; прочие ошибки пробрасываются.
func (s *Scenario) SearchDivisions(ctx context.Context, q models.DivisionSearchQuery) ([]models.AdministrativeDivision, error) {
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

	// репозиторий отдаёт окна хитов: обходим их до пустого или до заполнения окна.
	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.adminDivisions.Search(ctx, text, models.AccessFull,
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

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.adminDivisions.GetAdministrativeDivision(ctx, h.ID)
			if err != nil {
				if errors.Is(err, models.ErrNotFound) {
					matched++ // хит без живой единицы не расходует окно, но сдвиг вперёд
					continue
				}

				return nil, err
			}

			out = append(out, *got)

			if len(out) == page.Limit {
				return out, nil
			}

			matched++
		}
	}
}
