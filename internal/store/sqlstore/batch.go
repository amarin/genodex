package sqlstore

import (
	"database/sql"
	"strings"

	"github.com/amarin/genodex/internal/models"
)

// Пакетная загрузка: вместо запросов на каждую сущность списка — по одному
// запросу на связную таблицу с IN (...) по всем владельцам окна. Число
// запросов не зависит от числа строк (кроме нарезки IN по inChunk значений).

// inChunk — сколько значений подставляется в один IN (...): списки режутся на
// куски, чтобы не упереться в предел SQLite на число параметров запроса.
const inChunk = 500

// placeholders возвращает «?, ?, …» на n значений.
func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?, ", n), ", ")
}

// forChunks вызывает fn по кускам не длиннее inChunk; пустой список — без вызовов.
func forChunks[T any](vals []T, fn func([]T) error) error {
	for start := 0; start < len(vals); start += inChunk {
		if err := fn(vals[start:min(start+inChunk, len(vals))]); err != nil {
			return err
		}
	}

	return nil
}

// anySlice превращает список значений в аргументы запроса.
func anySlice[T any](vals []T) []any {
	out := make([]any, len(vals))
	for i, v := range vals {
		out[i] = v
	}

	return out
}

// keyedRow — строка результата с ключом владельца.
type keyedRow[K comparable, V any] struct {
	key K
	val V
}

// queryGrouped выполняет запрос по IN-списку ключей (кусками) и раскладывает
// строки по ключу с сохранением порядка строк. args — параметры, стоящие в
// запросе до IN-списка; sqlFor получает готовые плейсхолдеры IN.
func queryGrouped[K comparable, V any](
	q queryer, keys []K, args []any, sqlFor func(in string) string, scan func(*sql.Rows) (K, V, error),
) (map[K][]V, error) {
	out := map[K][]V{}

	err := forChunks(keys, func(chunk []K) error {
		rows, err := scanRows(q, func(r *sql.Rows) (keyedRow[K, V], error) {
			k, v, err := scan(r)

			return keyedRow[K, V]{key: k, val: v}, err
		}, sqlFor(placeholders(len(chunk))), append(append([]any{}, args...), anySlice(chunk)...)...)
		if err != nil {
			return err
		}

		for _, kr := range rows {
			out[kr.key] = append(out[kr.key], kr.val)
		}

		return nil
	})

	return out, err
}

// loadTextRefsByID читает TextRef по id (нули пропускаются).
func loadTextRefsByID(q queryer, ids []int64) (map[int64]models.TextRef, error) {
	out := make(map[int64]models.TextRef, len(ids))

	grouped, err := queryGrouped(q, nonZero(ids), nil, func(in string) string {
		return `SELECT id, text, ref, ref_type FROM text_refs WHERE id IN (` + in + `)`
	}, func(r *sql.Rows) (int64, models.TextRef, error) {
		var (
			id           int64
			tr           models.TextRef
			ref, refType string
		)

		err := r.Scan(&id, &tr.Text, &ref, &refType)
		tr.Ref, tr.Type = models.ID(ref), models.Type(refType)

		return id, tr, err
	})
	if err != nil {
		return nil, err
	}

	for id, trs := range grouped {
		out[id] = trs[0]
	}

	return out, nil
}

// loadDatesByID читает FactDate по id (нули пропускаются).
func loadDatesByID(q queryer, ids []int64) (map[int64]*models.FactDate, error) {
	grouped, err := queryGrouped(q, nonZero(ids), nil, func(in string) string {
		return `SELECT id, year, month, day, precision, modifier, year_to, month_to, day_to, calendar
		        FROM dates WHERE id IN (` + in + `)`
	}, func(r *sql.Rows) (int64, *models.FactDate, error) {
		var (
			id             int64
			d              models.FactDate
			prec, mod, cal string
		)

		err := r.Scan(&id, &d.Year, &d.Month, &d.Day, &prec, &mod, &d.YearTo, &d.MonthTo, &d.DayTo, &cal)
		d.Precision, d.Modifier = models.FactPrecision(prec), models.FactModifier(mod)
		d.Calendar = models.FactCalendar(cal)

		return id, &d, err
	})
	if err != nil {
		return nil, err
	}

	out := make(map[int64]*models.FactDate, len(grouped))
	for id, ds := range grouped {
		out[id] = ds[0]
	}

	return out, nil
}

// nonZero отбирает ненулевые id (0 — «значения нет»).
func nonZero(ids []int64) []int64 {
	out := make([]int64, 0, len(ids))

	for _, id := range ids {
		if id != 0 {
			out = append(out, id)
		}
	}

	return out
}

// loadTextRefListsBatch читает списки TextRef связной таблицы для всех
// владельцев: владелец → список в порядке position.
func loadTextRefListsBatch(q queryer, table, ownerCol string, owners []string) (map[string][]models.TextRef, error) {
	return queryGrouped(q, owners, nil, func(in string) string {
		return `SELECT t.` + ownerCol + `, tr.text, tr.ref, tr.ref_type FROM ` + table + ` t
		        JOIN text_refs tr ON tr.id = t.text_ref_id
		        WHERE t.` + ownerCol + ` IN (` + in + `) ORDER BY t.position`
	}, func(r *sql.Rows) (string, models.TextRef, error) {
		var (
			owner        string
			tr           models.TextRef
			ref, refType string
		)

		err := r.Scan(&owner, &tr.Text, &ref, &refType)
		tr.Ref, tr.Type = models.ID(ref), models.Type(refType)

		return owner, tr, err
	})
}

// loadStringListsBatch читает списки строк для всех владельцев.
func loadStringListsBatch(q queryer, table, ownerCol string, owners []string) (map[string][]string, error) {
	return queryGrouped(q, owners, nil, func(in string) string {
		return `SELECT ` + ownerCol + `, value FROM ` + table +
			` WHERE ` + ownerCol + ` IN (` + in + `) ORDER BY position`
	}, func(r *sql.Rows) (string, string, error) {
		var owner, v string
		err := r.Scan(&owner, &v)

		return owner, v, err
	})
}

// loadRenamesBatch читает переименования для всех владельцев.
func loadRenamesBatch(q queryer, table, ownerCol string, owners []string) (map[string][]models.NamedPeriod, error) {
	return queryGrouped(q, owners, nil, func(in string) string {
		return `SELECT ` + ownerCol + `, text, since, until FROM ` + table +
			` WHERE ` + ownerCol + ` IN (` + in + `) ORDER BY position`
	}, func(r *sql.Rows) (string, models.NamedPeriod, error) {
		var (
			owner string
			rn    models.NamedPeriod
		)

		err := r.Scan(&owner, &rn.Text, &rn.Since, &rn.Until)

		return owner, rn, err
	})
}

// loadSourceLinksBatch читает доказательства сущностей вида t: владелец →
// доказательства в порядке записи; target_type/target_id восстанавливаются из
// владельца, как в loadSourceLinks.
func loadSourceLinksBatch(q queryer, t models.Type, owners []string) (map[string][]models.SourceLink, error) {
	return queryGrouped(q, owners, []any{string(t)}, func(in string) string {
		return `SELECT target_id, citation_id, reliability, role, note
		        FROM source_links WHERE target_type = ? AND target_id IN (` + in + `) ORDER BY id`
	}, func(r *sql.Rows) (string, models.SourceLink, error) {
		var (
			owner, citationID, reliability string
			sl                             models.SourceLink
		)

		err := r.Scan(&owner, &citationID, &reliability, &sl.Role, &sl.Note)
		sl.CitationID = models.ID(citationID)
		sl.TargetType, sl.TargetID = t, models.ID(owner)
		sl.Reliability = models.Reliability(reliability)

		return owner, sl, err
	})
}

// idStrings переводит идентификаторы сущностей в строки для параметров запроса.
func idStrings(ids []models.ID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = string(id)
	}

	return out
}
