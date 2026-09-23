package search_archive_documents

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск документов внутри единиц учёта».
type Scenario struct {
	archiveDocuments ArchiveDocumentRepo
}

// New создаёт сценарий.
func New(archiveDocuments ArchiveDocumentRepo) *Scenario {
	return &Scenario{archiveDocuments: archiveDocuments}
}

// SearchArchiveDocuments находит документы, чей title (индексируется под
// полем "title", см. internal/store/sqlstore/archive.go) начинается с
// текста запроса, и возвращает их целиком. Результат — в том порядке, в
// каком их отдаёт Search. Окно (размер и сдвиг) применяется после отбора
// хитов own-типа. Пустой текст (после обрезки) — пустой результат без
// обращения к репозиторию. Хит, чей документ удалён между поиском и
// чтением (ErrNotFound), пропускается; прочие ошибки пробрасываются.
func (s *Scenario) SearchArchiveDocuments(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.ArchiveDocument, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.ArchiveDocument{}, nil
	}

	page := q.Page.Normalized()
	out := []models.ArchiveDocument{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.archiveDocuments.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeArchiveDocument {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.archiveDocuments.GetArchiveDocument(ctx, h.ID)
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
