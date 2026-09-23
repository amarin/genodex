package search_archive_nodes

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск узлов архивного дерева».
type Scenario struct {
	archiveNodes ArchiveNodeRepo
}

// New создаёт сценарий.
func New(archiveNodes ArchiveNodeRepo) *Scenario {
	return &Scenario{archiveNodes: archiveNodes}
}

// SearchArchiveNodes находит узлы, чьи label/name (индексируются вместе под
// полем "name", см. internal/store/sqlstore/archive.go) начинаются с текста
// запроса, и возвращает их целиком. Результат — в том порядке, в каком их
// отдаёт Search, без фильтра по архиву (поиск глобальный по всем архивам —
// сужение по архиву, если понадобится, веб-сторона делает сама). Окно
// (размер и сдвиг) применяется после отбора хитов own-типа: хиты других
// сущностей не расходуют окно. Пустой текст (после обрезки) — пустой
// результат без обращения к репозиторию. Хит, чей узел удалён между поиском
// и чтением (ErrNotFound), пропускается; прочие ошибки пробрасываются.
func (s *Scenario) SearchArchiveNodes(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.ArchiveNode, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.ArchiveNode{}, nil
	}

	page := q.Page.Normalized()
	out := []models.ArchiveNode{}
	matched := 0 // сколько хитов узлов прошло (для сдвига окна)

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.archiveNodes.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeArchiveNode {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.archiveNodes.GetArchiveNode(ctx, h.ID)
			if err != nil {
				if errors.Is(err, models.ErrNotFound) {
					matched++

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
