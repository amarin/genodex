package list_archive_nodes

import (
	"context"

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
func (s *Scenario) ListArchiveNodes(ctx context.Context, access models.Access, q models.ArchiveNodeQuery) ([]models.ArchiveNode, error) {
	if err := q.Validate(); err != nil {
		return nil, err
	}

	page := q.Page.Normalized()
	out := []models.ArchiveNode{}
	matched := 0 // сколько узлов прошло фильтр (для сдвига окна)

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
		if full, matched, out = applyWindow(q, nodes, page, matched, out); full {
			return out, nil
		}
	}
}

// applyWindow прогоняет одно окно репозитория через фильтр запроса и
// накапливает результат окна запроса. full — окно запроса заполнено (обход
// можно остановить).
func applyWindow(q models.ArchiveNodeQuery, nodes []*models.ArchiveNode,
	page models.Page, matched int, out []models.ArchiveNode,
) (full bool, nextMatched int, nextOut []models.ArchiveNode) {
	for _, n := range nodes {
		if n.ArchiveID != q.ArchiveID || !sameParent(n.ParentID, q.ParentID) {
			continue
		}

		if matched >= page.Offset {
			out = append(out, *n)

			if len(out) == page.Limit {
				return true, matched, out
			}
		}

		matched++
	}

	return false, matched, out
}

// sameParent сравнивает два необязательных id родителя nil-safe: оба nil —
// равны; один nil — не равны; иначе сравниваются значения.
func sameParent(a, b *models.ID) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}

	return *a == *b
}
