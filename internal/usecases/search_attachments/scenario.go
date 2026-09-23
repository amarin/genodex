package search_attachments

import (
	"context"
	"errors"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «поиск файловых вложений».
type Scenario struct {
	attachments AttachmentRepo
}

// New создаёт сценарий.
func New(attachments AttachmentRepo) *Scenario {
	return &Scenario{attachments: attachments}
}

// SearchAttachments находит вложения, чьё имя файла или URI начинаются
// с текста запроса — та же механика, что и
// search_divisions.SearchDivisions (см. её комментарий).
func (s *Scenario) SearchAttachments(ctx context.Context, access models.Access, q models.SearchQuery) ([]models.Attachment, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	text := strings.TrimSpace(q.Text)
	if text == "" {
		return []models.Attachment{}, nil
	}

	page := q.Page.Normalized()
	out := []models.Attachment{}
	matched := 0

	for offset := 0; ; offset += models.MaxPageLimit {
		hits, err := s.attachments.Search(ctx, text, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(hits) == 0 {
			return out, nil
		}

		for _, h := range hits {
			if h.Type != models.TypeAttachment {
				continue
			}

			if matched < page.Offset {
				matched++

				continue
			}

			got, err := s.attachments.GetAttachment(ctx, h.ID)
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
