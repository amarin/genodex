package sqlstore

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/storage"
)

// --- очистка сирот в общих value-таблицах --------------------------------
//
// Связные таблицы ссылаются на text_refs/dates/anchors внешним ключом
// (`text_ref_id`, `since_id`, `anchor_id`), то есть FK ведёт ИЗ дочерней
// строки В общую таблицу. Каскад владельца сносит дочернюю строку, но
// строка-значение остаётся сиротой. Поэтому при каждой перезаписи сначала
// собираются id значений, затем удаляются дочерние строки, и только потом —
// сами значения (см. docs/data-model/normalization-s1s2.md §5).

// valueRefs — колонки дочерней таблицы, ссылающиеся на одну общую
// value-таблицу.
type valueRefs struct {
	table string   // dates | text_refs | anchors
	cols  []string // колонки дочерней таблицы
}

// deleteValues удаляет строки общей value-таблицы по собранным id.
func deleteValues(tx *sql.Tx, table string, ids []int64) error {
	for _, id := range ids {
		if _, err := tx.Exec(`DELETE FROM `+table+` WHERE id = ?`, id); err != nil {
			return err
		}
	}

	return nil
}

// clearChildren удаляет дочерние строки владельца вместе со строками
// value-таблиц, на которые они ссылались.
func clearChildren(tx *sql.Tx, table, ownerCol, ownerID string, refs ...valueRefs) error {
	collected := make([][]int64, len(refs))

	for i, r := range refs {
		ids, err := collectIDs(tx,
			`SELECT `+strings.Join(r.cols, ", ")+` FROM `+table+` WHERE `+ownerCol+` = ?`, ownerID)
		if err != nil {
			return err
		}

		collected[i] = ids
	}

	if _, err := tx.Exec(`DELETE FROM `+table+` WHERE `+ownerCol+` = ?`, ownerID); err != nil {
		return err
	}

	for i, r := range refs {
		if err := deleteValues(tx, r.table, collected[i]); err != nil {
			return err
		}
	}

	return nil
}

// mainRefs — value-ссылки главной строки сущности, собранные ДО перезаписи.
// Удалять их можно только после upsert: пока колонка указывает на строку,
// FK не даст её удалить.
type mainRefs struct {
	dates   []int64
	texts   []int64
	anchors []int64
}

// collectMainRefs собирает текущие value-ссылки главной строки.
func collectMainRefs(tx *sql.Tx, table, id string, dateCols, textCols, anchorCols []string) (mainRefs, error) {
	var (
		m   mainRefs
		err error
	)

	pick := func(cols []string) ([]int64, error) {
		if len(cols) == 0 {
			return nil, nil
		}

		return collectIDs(tx, `SELECT `+strings.Join(cols, ", ")+` FROM `+table+` WHERE id = ?`, id)
	}

	if m.dates, err = pick(dateCols); err != nil {
		return m, err
	}

	if m.texts, err = pick(textCols); err != nil {
		return m, err
	}

	if m.anchors, err = pick(anchorCols); err != nil {
		return m, err
	}

	return m, nil
}

// drop удаляет собранные ранее значения (вызывается после upsert).
func (m mainRefs) drop(tx *sql.Tx) error {
	if err := deleteValues(tx, "dates", m.dates); err != nil {
		return err
	}

	if err := deleteValues(tx, "text_refs", m.texts); err != nil {
		return err
	}

	return deleteValues(tx, "anchors", m.anchors)
}

// --- text_refs ------------------------------------------------------------

// insertTextRef пишет TextRef в text_refs и возвращает id; пустое значение —
// 0 (колонка получит NULL, элемент списка пропускается).
func insertTextRef(tx *sql.Tx, tr *models.TextRef) (int64, error) {
	if tr == nil || (tr.Text == "" && tr.Ref == "") {
		return 0, nil
	}

	return insertTextRefRow(tx, *tr)
}

// insertTextRefRow пишет строку text_refs безусловно: нужно для NOT NULL
// колонок person_names.surname_id/given_id/patronymic_id.
func insertTextRefRow(tx *sql.Tx, tr models.TextRef) (int64, error) {
	res, err := tx.Exec(`INSERT INTO text_refs(text, ref, ref_type) VALUES (?, ?, ?)`,
		tr.Text, string(tr.Ref), string(tr.Type))
	if err != nil {
		return 0, err
	}

	return res.LastInsertId()
}

// loadTextRef читает TextRef по id; 0 — пустое значение.
func loadTextRef(q queryer, id int64) (models.TextRef, error) {
	var tr models.TextRef

	if id == 0 {
		return tr, nil
	}

	var ref, refType string
	if err := q.QueryRow(`SELECT text, ref, ref_type FROM text_refs WHERE id = ?`, id).
		Scan(&tr.Text, &ref, &refType); err != nil {
		return models.TextRef{}, err
	}

	tr.Ref, tr.Type = models.ID(ref), models.Type(refType)

	return tr, nil
}

// loadTextRefPtr читает необязательное поле TextRef?: NULL → nil.
func loadTextRefPtr(q queryer, id int64) (*models.TextRef, error) {
	if id == 0 {
		return nil, nil
	}

	tr, err := loadTextRef(q, id)
	if err != nil {
		return nil, err
	}

	return &tr, nil
}

// replaceTextRefList переписывает список TextRef в связной таблице.
func replaceTextRefList(tx *sql.Tx, table, ownerCol, ownerID string, items []models.TextRef) error {
	if err := clearChildren(tx, table, ownerCol, ownerID,
		valueRefs{table: "text_refs", cols: []string{"text_ref_id"}}); err != nil {
		return err
	}

	for i, it := range items {
		trID, err := insertTextRef(tx, &it)
		if err != nil {
			return err
		}

		if trID == 0 {
			continue
		}

		if _, err := tx.Exec(
			`INSERT INTO `+table+`(`+ownerCol+`, position, text_ref_id) VALUES (?, ?, ?)`,
			ownerID, i, trID,
		); err != nil {
			return err
		}
	}

	return nil
}

// loadTextRefList читает список TextRef связной таблицы в порядке position.
func loadTextRefList(q queryer, table, ownerCol, ownerID string) ([]models.TextRef, error) {
	return scanRows(q, func(r *sql.Rows) (models.TextRef, error) {
		var (
			tr           models.TextRef
			ref, refType string
		)

		if err := r.Scan(&tr.Text, &ref, &refType); err != nil {
			return tr, err
		}

		tr.Ref, tr.Type = models.ID(ref), models.Type(refType)

		return tr, nil
	},
		`SELECT tr.text, tr.ref, tr.ref_type FROM `+table+` t
		 JOIN text_refs tr ON tr.id = t.text_ref_id
		 WHERE t.`+ownerCol+` = ? ORDER BY t.position`, ownerID)
}

// --- dates ----------------------------------------------------------------

// insertDate пишет FactDate в dates и возвращает id; nil — 0 (колонка NULL).
// Колонка calendar остаётся со значением по умолчанию: у FactDate нет поля
// календаря.
func insertDate(tx *sql.Tx, d *models.FactDate) (int64, error) {
	if d == nil {
		return 0, nil
	}

	res, err := tx.Exec(
		`INSERT INTO dates(year, month, day, precision, modifier, year_to, month_to, day_to)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		d.Year, d.Month, d.Day, string(d.Precision), string(d.Modifier),
		d.YearTo, d.MonthTo, d.DayTo)
	if err != nil {
		return 0, err
	}

	return res.LastInsertId()
}

// loadDate читает FactDate по id; 0 — nil (поле не заполнено).
func loadDate(q queryer, id int64) (*models.FactDate, error) {
	if id == 0 {
		return nil, nil
	}

	var (
		d         models.FactDate
		prec, mod string
	)

	if err := q.QueryRow(
		`SELECT year, month, day, precision, modifier, year_to, month_to, day_to
		 FROM dates WHERE id = ?`, id,
	).Scan(&d.Year, &d.Month, &d.Day, &prec, &mod, &d.YearTo, &d.MonthTo, &d.DayTo); err != nil {
		return nil, err
	}

	d.Precision, d.Modifier = models.FactPrecision(prec), models.FactModifier(mod)

	return &d, nil
}

// --- anchors --------------------------------------------------------------

// insertAnchor пишет Anchor в anchors (дискриминатор — колонка kind).
func insertAnchor(tx *sql.Tx, a models.Anchor) (int64, error) {
	if a == nil {
		return 0, nil
	}

	var (
		kind, nodeID, docID, rect string
		attID, timecode, url      string
		page                      int
	)

	switch v := a.(type) {
	case *models.ArchiveAnchor:
		kind, nodeID, docID, page, rect =
			string(models.AnchorArchive), string(v.NodeID), string(v.DocumentID), v.Page, v.Rect
	case *models.FileAnchor:
		kind, attID, timecode = string(models.AnchorFile), string(v.AttachmentID), v.Timecode
	case *models.URLAnchor:
		kind, url = string(models.AnchorURL), v.URL
	default:
		return 0, fmt.Errorf("anchor: неизвестная реализация %T", a)
	}

	res, err := tx.Exec(
		`INSERT INTO anchors(kind, node_id, document_id, page, rect, attachment_id, timecode, url)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		kind, nodeID, docID, page, rect, attID, timecode, url)
	if err != nil {
		return 0, err
	}

	return res.LastInsertId()
}

// loadAnchor читает Anchor по id; 0 — nil.
func loadAnchor(q queryer, id int64) (models.Anchor, error) {
	if id == 0 {
		return nil, nil
	}

	var (
		kind, nodeID, docID, rect string
		attID, timecode, url      string
		page                      int
	)

	if err := q.QueryRow(
		`SELECT kind, node_id, document_id, page, rect, attachment_id, timecode, url
		 FROM anchors WHERE id = ?`, id,
	).Scan(&kind, &nodeID, &docID, &page, &rect, &attID, &timecode, &url); err != nil {
		return nil, err
	}

	switch models.AnchorKind(kind) {
	case models.AnchorArchive:
		return &models.ArchiveAnchor{
			NodeID: models.ID(nodeID), DocumentID: models.ID(docID), Page: page, Rect: rect,
		}, nil
	case models.AnchorFile:
		return &models.FileAnchor{AttachmentID: models.ID(attID), Timecode: timecode}, nil
	case models.AnchorURL:
		return &models.URLAnchor{URL: url}, nil
	}

	return nil, fmt.Errorf("anchor: неизвестный kind %q", kind)
}

// --- source_links ---------------------------------------------------------

// replaceSourceLinks переписывает доказательства сущности. Полиморфная
// связь (target_type + target_id) чистится владельцем утверждения.
func replaceSourceLinks(tx *sql.Tx, t models.Type, id models.ID, links []models.SourceLink) error {
	if _, err := tx.Exec(
		`DELETE FROM source_links WHERE target_type = ? AND target_id = ?`,
		string(t), string(id),
	); err != nil {
		return err
	}

	for _, l := range links {
		if _, err := tx.Exec(
			`INSERT INTO source_links(citation_id, target_type, target_id, reliability, role, note)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			string(l.CitationID), string(t), string(id),
			string(l.Reliability), l.Role, l.Note,
		); err != nil {
			return err
		}
	}

	return nil
}

// loadSourceLinks читает доказательства сущности; target_type/target_id
// восстанавливаются из владельца, а не читаются из строки.
func loadSourceLinks(q queryer, t models.Type, id models.ID) ([]models.SourceLink, error) {
	return scanRows(q, func(r *sql.Rows) (models.SourceLink, error) {
		var (
			sl                                  models.SourceLink
			citationID, reliability, role, note string
		)

		if err := r.Scan(&citationID, &reliability, &role, &note); err != nil {
			return sl, err
		}

		sl.CitationID = models.ID(citationID)
		sl.TargetType, sl.TargetID = t, id
		sl.Reliability = models.Reliability(reliability)
		sl.Role, sl.Note = role, note

		return sl, nil
	},
		`SELECT citation_id, reliability, role, note
		 FROM source_links WHERE target_type = ? AND target_id = ? ORDER BY id`,
		string(t), string(id))
}

// --- списки-значения ------------------------------------------------------

// replaceStringList переписывает список строк (ad_variants, church_variants).
func replaceStringList(tx *sql.Tx, table, ownerCol, ownerID string, values []string) error {
	if err := clearChildren(tx, table, ownerCol, ownerID); err != nil {
		return err
	}

	for i, v := range values {
		if v == "" {
			continue
		}

		if _, err := tx.Exec(
			`INSERT INTO `+table+`(`+ownerCol+`, position, value) VALUES (?, ?, ?)`,
			ownerID, i, v,
		); err != nil {
			return err
		}
	}

	return nil
}

// loadStringList читает список строк в порядке position.
func loadStringList(q queryer, table, ownerCol, ownerID string) ([]string, error) {
	return scanRows(q, func(r *sql.Rows) (string, error) {
		var v string
		err := r.Scan(&v)

		return v, err
	}, `SELECT value FROM `+table+` WHERE `+ownerCol+` = ? ORDER BY position`, ownerID)
}

// replaceRenames переписывает переименования (ad_renames: текст + период
// строками).
func replaceRenames(tx *sql.Tx, table, ownerCol, ownerID string, rns []models.NamedPeriod) error {
	if err := clearChildren(tx, table, ownerCol, ownerID); err != nil {
		return err
	}

	for i, rn := range rns {
		if _, err := tx.Exec(
			`INSERT INTO `+table+`(`+ownerCol+`, position, text, since, until) VALUES (?, ?, ?, ?, ?)`,
			ownerID, i, rn.Text, rn.Since, rn.Until,
		); err != nil {
			return err
		}
	}

	return nil
}

// loadRenames читает переименования в порядке position.
func loadRenames(q queryer, table, ownerCol, ownerID string) ([]models.NamedPeriod, error) {
	return scanRows(q, func(r *sql.Rows) (models.NamedPeriod, error) {
		var rn models.NamedPeriod
		err := r.Scan(&rn.Text, &rn.Since, &rn.Until)

		return rn, err
	}, `SELECT text, since, until FROM `+table+` WHERE `+ownerCol+` = ? ORDER BY position`, ownerID)
}

// --- поисковый индекс -----------------------------------------------------

// replaceSearchIndex переписывает поисковые термины сущности. Термины
// нормализуются в Go (storage.Normalize): нижний регистр, ё→е, без
// диакритики; обе стороны сравнения нормализованы, коллация бинарная.
func replaceSearchIndex(tx *sql.Tx, table string, id models.ID, fields map[string][]string) error {
	if _, err := tx.Exec(
		`DELETE FROM search_index WHERE entity_table = ? AND entity_id = ?`, table, string(id),
	); err != nil {
		return err
	}

	for field, terms := range fields {
		for _, term := range terms {
			norm := storage.Normalize(term)
			if norm == "" {
				continue
			}

			// повтор термина в одном поле допустим на входе, в индексе —
			// один раз (PRIMARY KEY).
			if _, err := tx.Exec(
				`INSERT OR IGNORE INTO search_index(entity_table, entity_id, field, term)
				 VALUES (?, ?, ?, ?)`,
				table, string(id), field, norm,
			); err != nil {
				return err
			}
		}
	}

	return nil
}

// searchIDs возвращает id сущностей таблицы, чей термин начинается с query.
// Запрос нормализуется здесь же — регистронезависимость без участия
// sqlite-коллаций.
func (s *Store) searchIDs(table, query string) ([]models.ID, error) {
	norm := storage.Normalize(query)
	escaped := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(norm)

	return scanRows(s.db, func(r *sql.Rows) (models.ID, error) {
		var id string
		err := r.Scan(&id)

		return models.ID(id), err
	},
		`SELECT DISTINCT entity_id FROM search_index
		 WHERE entity_table = ? AND term LIKE ? ESCAPE '\' ORDER BY entity_id`,
		table, escaped+"%")
}

// wrapSave дополняет ошибку сохранения видом и id сущности; %w сохраняет
// исходную ошибку (например, нарушение внешнего ключа) для errors.Is/As.
// Используется как defer wrapSave(&err, "person", p.ID) в методах Save*.
func wrapSave(err *error, kind string, id models.ID) {
	if *err != nil {
		*err = fmt.Errorf("save %s %q: %w", kind, string(id), *err)
	}
}
