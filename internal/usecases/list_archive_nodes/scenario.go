package list_archive_nodes

import (
	"context"
	"errors"

	"github.com/amarin/genodex/internal/models"
)

// Scenario — сценарий «список узлов архивного дерева».
type Scenario struct {
	archiveNodes ArchiveNodeRepo
}

// New создаёт сценарий.
func New(archiveNodes ArchiveNodeRepo) *Scenario {
	return &Scenario{archiveNodes: archiveNodes}
}

// ListArchiveNodes возвращает узлы дерева заданного архива (q.ArchiveID
// обязателен), прошедшие фильтры запроса, в порядке сохранения; окно
// применяется после фильтра. С ParentID — только прямые дети этого узла;
// без ParentID — корень дерева внутри архива. Некорректный запрос —
// *models.ValidationError, репозиторий не вызывается. Короткий результат
// (меньше размера окна) означает конец списка.
//
// В отличие от list_divisions (у AdministrativeDivision есть отдельный
// ChildrenOfDivision), у ArchiveNode нет своего метода «дети узла» —
// фильтрация идёт одним проходом по generic-окнам ListArchiveNodes: узел
// проходит, если его ArchiveID совпадает с q.ArchiveID и ParentID совпадает
// с q.ParentID (сравнение nil-safe, см. sameParent). При объёме данных
// этой сущности (архивные деревья, не тысячи записей) полное сканирование
// приемлемо; выделенный метод хранилища не оправдан.
//
// Помимо фильтра запроса, окно исключает узлы, ссылающиеся (через
// Sources[i].CitationID) на приватную цитату — для вызывающего без полного
// доступа такой узел скрывается целиком, даже если сам он не приватен (см.
// комментарий GetArchiveNode).
func (s *Scenario) ListArchiveNodes(ctx context.Context, access models.Access, q models.ArchiveNodeQuery) ([]models.ArchiveNode, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	page := q.Page.Normalized()
	out := []models.ArchiveNode{}
	matched := 0 // сколько узлов прошло фильтр (для сдвига окна)
	// cache — мемоизация Private по id цитаты в рамках ОДНОГО вызова
	// ListArchiveNodes (не переживает вызов, не шарится между запросами).
	cache := map[models.ID]bool{}

	// репозиторий отдаёт окна: обходим их до пустого или до заполнения окна запроса
	for offset := 0; ; offset += models.MaxPageLimit {
		nodes, err := s.archiveNodes.ListArchiveNodes(ctx, access,
			models.Page{Limit: models.MaxPageLimit, Offset: offset})
		if err != nil {
			return nil, err
		}

		if len(nodes) == 0 {
			return out, nil
		}

		var full bool
		if full, matched, out, err = applyWindow(ctx, s.archiveNodes, access, q, nodes, page, matched, out, cache); err != nil {
			return nil, err
		} else if full {
			return out, nil
		}
	}
}

// applyWindow прогоняет одно окно репозитория через фильтр запроса и
// накапливает результат окна запроса. full — окно запроса заполнено (обход
// можно остановить). Помимо q.ArchiveID/q.ParentID, окно фильтрует узлы,
// ссылающиеся на приватную цитату (см. комментарий ListArchiveNodes) —
// для access != models.AccessFull; проверка идёт вторым условием, после
// фильтра запроса.
func applyWindow(ctx context.Context, repo ArchiveNodeRepo, access models.Access, q models.ArchiveNodeQuery, nodes []*models.ArchiveNode,
	page models.Page, matched int, out []models.ArchiveNode, cache map[models.ID]bool,
) (full bool, nextMatched int, nextOut []models.ArchiveNode, err error) {
	for _, n := range nodes {
		if n.ArchiveID != q.ArchiveID || !sameParent(n.ParentID, q.ParentID) {
			continue
		}

		if access != models.AccessFull {
			hidden, err := archiveNodeReferencesPrivateCitation(ctx, repo, n, cache)
			if err != nil {
				return false, matched, out, err
			}

			if hidden {
				continue
			}
		}

		if matched >= page.Offset {
			out = append(out, *n)

			if len(out) == page.Limit {
				return true, matched, out, nil
			}
		}

		matched++
	}

	return false, matched, out, nil
}

// archiveNodeReferencesPrivateCitation сообщает, ссылается ли узел (через
// Sources[i].CitationID) хотя бы на одну приватную цитату. Независимая
// копия одноимённой функции get_archive_node (та не использует cache):
// пакеты сценариев в этом проекте самодостаточны и не делятся кодом друг с
// другом. cache — мемоизация в рамках одного вызова ListArchiveNodes.
func archiveNodeReferencesPrivateCitation(ctx context.Context, repo ArchiveNodeRepo, rec *models.ArchiveNode, cache map[models.ID]bool) (bool, error) {
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
// цитата трактуется как приватная, т.е. узел, ссылающийся на неё, тоже
// прячется, а не проваливает весь вызов ошибкой. cache — мемоизация в
// рамках одного вызова.
func citationIsPrivate(ctx context.Context, repo ArchiveNodeRepo, cache map[models.ID]bool, id models.ID) (bool, error) {
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

// sameParent сравнивает два необязательных id родителя nil-safe: оба nil —
// равны; один nil — не равны; иначе сравниваются значения.
func sameParent(a, b *models.ID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	return *a == *b
}
