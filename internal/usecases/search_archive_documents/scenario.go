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
// каком их отдаёт Search. Пустой текст (после обрезки) — пустой результат
// без обращения к репозиторию. Документ, ссылающийся (через
// Sources[i].CitationID) на приватную цитату, для вызывающего без полного
// доступа прячется, даже если сам он не приватен (см. комментарий
// get_archive_document.GetArchiveDocument).
//
// Порядок проверок — как в search_events: сначала загрузка документа
// (пропуск хита без учёта offset-бюджета при ErrNotFound), затем проверка
// приватной цитаты (тот же пропуск), и только после этого — учёт offset;
// иначе скрытый хит перед запрошенным offset "съедает" часть offset-бюджета
// вслепую, и соседние страницы дублируют/теряют записи.
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
	// cache — мемоизация Private по id цитаты в рамках ОДНОГО вызова
	// SearchArchiveDocuments (не переживает вызов, не шарится между
	// запросами).
	cache := map[models.ID]bool{}

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

			got, err := s.archiveDocuments.GetArchiveDocument(ctx, h.ID)
			if err != nil {
				if errors.Is(err, models.ErrNotFound) {
					continue
				}

				return nil, err
			}

			if access != models.AccessFull {
				hidden, err := archiveDocumentReferencesPrivateCitation(ctx, s.archiveDocuments, got, cache)
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

// archiveDocumentReferencesPrivateCitation сообщает, ссылается ли документ
// (через Sources[i].CitationID) хотя бы на одну приватную цитату.
// Независимая копия одноимённой функции get_archive_document/
// list_archive_documents: пакеты сценариев в этом проекте самодостаточны и
// не делятся кодом друг с другом. cache — мемоизация в рамках одного
// вызова SearchArchiveDocuments.
func archiveDocumentReferencesPrivateCitation(ctx context.Context, repo ArchiveDocumentRepo, rec *models.ArchiveDocument, cache map[models.ID]bool) (bool, error) {
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

// citationIsPrivate сообщает, приватна ли цитата id — с точки зрения
// сканирующего List*/Search*-сценария сюда же относится и гонка с
// конкурентным удалением цитаты (models.ErrNotFound от GetCitation): такая
// цитата трактуется как приватная, т.е. запись, ссылающаяся на неё, тоже
// прячется, а не проваливает весь вызов ошибкой. cache — мемоизация в
// рамках одного вызова.
func citationIsPrivate(ctx context.Context, repo ArchiveDocumentRepo, cache map[models.ID]bool, id models.ID) (bool, error) {
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
