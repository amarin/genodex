package storage

// Колоночная схема БД: сущность = таблица, коллекция = связная таблица,
// value-тип = общая таблица (dates / text_refs / anchors). JSON-блобов нет
// нигде. Дизайн: docs/data-model/normalization-s1s2.md §5.
//
// Правила внешних ключей:
//   - дочерние строки (коллекции сущности) — ON DELETE CASCADE от владельца;
//   - строгие ссылки между сущностями (parent_id, person_a/b, archive_id,
//     node_id, unit_id, person_id, place_id, source_id, citation_id,
//     repository_id) — ON DELETE RESTRICT;
//   - необязательные ссылки на value-таблицы (dates, anchors, text_refs)
//     — ON DELETE SET NULL, обязательные — ON DELETE RESTRICT.
//
// source_links — единственная полиморфная связь (target_type + target_id);
// её чистит владелец утверждения по индексу idx_source_links_target.

// tables — реестр таблиц сущностей. Используется Count (валидация имени
// защищает от SQL-инъекции) и списком счётчиков манифеста бэкапа.
var tables = []string{
	"persons", "relations", "residences", "families",
	"surnames", "given_names", "patronymics", "estates", "titles",
	"churches", "parishes", "administrative_divisions",
	"events", "sources", "citations", "notes", "repositories",
	"archives", "archive_nodes", "archive_documents",
	"attachments",
}

var tableNames = map[string]bool{}

func init() {
	for _, t := range tables {
		tableNames[t] = true
	}
}

// schemaDDL — полный DDL схемы; выполняется целиком при каждом OpenDB
// (все выражения идемпотентны).
var schemaDDL = []string{
	// --- общие таблицы value-типов -------------------------------------

	`CREATE TABLE IF NOT EXISTS dates (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		year      INTEGER NOT NULL DEFAULT 0,
		month     INTEGER NOT NULL DEFAULT 0,
		day       INTEGER NOT NULL DEFAULT 0,
		precision TEXT    NOT NULL,
		modifier  TEXT    NOT NULL,
		year_to   INTEGER NOT NULL DEFAULT 0,
		month_to  INTEGER NOT NULL DEFAULT 0,
		day_to    INTEGER NOT NULL DEFAULT 0,
		calendar  TEXT    NOT NULL DEFAULT ''
	)`,

	`CREATE TABLE IF NOT EXISTS text_refs (
		id       INTEGER PRIMARY KEY AUTOINCREMENT,
		text     TEXT NOT NULL,
		ref      TEXT NOT NULL DEFAULT '',
		ref_type TEXT NOT NULL DEFAULT ''
	)`,

	`CREATE TABLE IF NOT EXISTS anchors (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		kind          TEXT    NOT NULL,
		node_id       TEXT    NOT NULL DEFAULT '',
		document_id   TEXT    NOT NULL DEFAULT '',
		page          INTEGER NOT NULL DEFAULT 0,
		rect          TEXT    NOT NULL DEFAULT '',
		attachment_id TEXT    NOT NULL DEFAULT '',
		timecode      TEXT    NOT NULL DEFAULT '',
		url           TEXT    NOT NULL DEFAULT ''
	)`,

	// --- персоны -------------------------------------------------------

	`CREATE TABLE IF NOT EXISTS persons (
		id      TEXT PRIMARY KEY,
		gender  TEXT    NOT NULL DEFAULT '',
		private INTEGER NOT NULL DEFAULT 0
	)`,

	`CREATE TABLE IF NOT EXISTS person_names (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		person_id     TEXT    NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
		type          TEXT    NOT NULL DEFAULT '',
		surname_id    INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT,
		given_id      INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT,
		patronymic_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT,
		prefix        TEXT    NOT NULL DEFAULT '',
		suffix        TEXT    NOT NULL DEFAULT '',
		since_id      INTEGER REFERENCES dates(id) ON DELETE SET NULL,
		until_id      INTEGER REFERENCES dates(id) ON DELETE SET NULL
	)`,

	`CREATE TABLE IF NOT EXISTS person_estates (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		person_id   TEXT    NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	`CREATE TABLE IF NOT EXISTS person_titles (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		person_id   TEXT    NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	`CREATE TABLE IF NOT EXISTS person_nicknames (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		person_id   TEXT    NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	`CREATE TABLE IF NOT EXISTS person_notes (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		person_id   TEXT    NOT NULL REFERENCES persons(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	// --- связи и семьи -------------------------------------------------

	`CREATE TABLE IF NOT EXISTS relations (
		id       TEXT PRIMARY KEY,
		kind     TEXT    NOT NULL,
		rel_type TEXT    NOT NULL DEFAULT '',
		person_a TEXT    NOT NULL REFERENCES persons(id) ON DELETE RESTRICT,
		person_b TEXT    NOT NULL REFERENCES persons(id) ON DELETE RESTRICT,
		since_id INTEGER REFERENCES dates(id) ON DELETE SET NULL,
		until_id INTEGER REFERENCES dates(id) ON DELETE SET NULL,
		private  INTEGER NOT NULL DEFAULT 0
	)`,

	`CREATE TABLE IF NOT EXISTS relation_notes (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		relation_id TEXT    NOT NULL REFERENCES relations(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	`CREATE TABLE IF NOT EXISTS families (
		id      TEXT PRIMARY KEY,
		name    TEXT    NOT NULL DEFAULT '',
		private INTEGER NOT NULL DEFAULT 0
	)`,

	`CREATE TABLE IF NOT EXISTS family_members (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		family_id   TEXT    NOT NULL REFERENCES families(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	`CREATE TABLE IF NOT EXISTS family_notes (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		family_id   TEXT    NOT NULL REFERENCES families(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	// --- словари: surname / given_name / patronymic / estate / title ----

	`CREATE TABLE IF NOT EXISTS surnames (
		id        TEXT PRIMARY KEY,
		canonical TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS surname_variants (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		surname_id  TEXT    NOT NULL REFERENCES surnames(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,
	`CREATE TABLE IF NOT EXISTS surname_items (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		surname_id  TEXT    NOT NULL REFERENCES surnames(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,
	`CREATE TABLE IF NOT EXISTS surname_notes (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		surname_id  TEXT    NOT NULL REFERENCES surnames(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	`CREATE TABLE IF NOT EXISTS given_names (
		id        TEXT PRIMARY KEY,
		canonical TEXT NOT NULL,
		gender    TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS given_name_variants (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		given_name_id TEXT    NOT NULL REFERENCES given_names(id) ON DELETE CASCADE,
		position      INTEGER NOT NULL,
		text_ref_id   INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,
	`CREATE TABLE IF NOT EXISTS given_name_items (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		given_name_id TEXT    NOT NULL REFERENCES given_names(id) ON DELETE CASCADE,
		position      INTEGER NOT NULL,
		text_ref_id   INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,
	`CREATE TABLE IF NOT EXISTS given_name_notes (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		given_name_id TEXT    NOT NULL REFERENCES given_names(id) ON DELETE CASCADE,
		position      INTEGER NOT NULL,
		text_ref_id   INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	`CREATE TABLE IF NOT EXISTS patronymics (
		id        TEXT PRIMARY KEY,
		canonical TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS patronymic_variants (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		patronymic_id  TEXT    NOT NULL REFERENCES patronymics(id) ON DELETE CASCADE,
		position       INTEGER NOT NULL,
		text_ref_id    INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,
	`CREATE TABLE IF NOT EXISTS patronymic_items (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		patronymic_id  TEXT    NOT NULL REFERENCES patronymics(id) ON DELETE CASCADE,
		position       INTEGER NOT NULL,
		text_ref_id    INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,
	`CREATE TABLE IF NOT EXISTS patronymic_notes (
		id             INTEGER PRIMARY KEY AUTOINCREMENT,
		patronymic_id  TEXT    NOT NULL REFERENCES patronymics(id) ON DELETE CASCADE,
		position       INTEGER NOT NULL,
		text_ref_id    INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	`CREATE TABLE IF NOT EXISTS estates (
		id        TEXT PRIMARY KEY,
		canonical TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS estate_variants (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		estate_id   TEXT    NOT NULL REFERENCES estates(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,
	`CREATE TABLE IF NOT EXISTS estate_items (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		estate_id   TEXT    NOT NULL REFERENCES estates(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,
	`CREATE TABLE IF NOT EXISTS estate_notes (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		estate_id   TEXT    NOT NULL REFERENCES estates(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	`CREATE TABLE IF NOT EXISTS titles (
		id        TEXT PRIMARY KEY,
		canonical TEXT NOT NULL
	)`,
	`CREATE TABLE IF NOT EXISTS title_variants (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		title_id    TEXT    NOT NULL REFERENCES titles(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,
	`CREATE TABLE IF NOT EXISTS title_items (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		title_id    TEXT    NOT NULL REFERENCES titles(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,
	`CREATE TABLE IF NOT EXISTS title_notes (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		title_id    TEXT    NOT NULL REFERENCES titles(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	// --- административное деление ---------------------------------------

	`CREATE TABLE IF NOT EXISTS administrative_divisions (
		id        TEXT PRIMARY KEY,
		name      TEXT NOT NULL,
		type      TEXT NOT NULL,
		parent_id TEXT    REFERENCES administrative_divisions(id) ON DELETE RESTRICT,
		since_id  INTEGER REFERENCES dates(id) ON DELETE SET NULL,
		until_id  INTEGER REFERENCES dates(id) ON DELETE SET NULL
	)`,

	`CREATE TABLE IF NOT EXISTS ad_items (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		ad_id       TEXT    NOT NULL REFERENCES administrative_divisions(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	`CREATE TABLE IF NOT EXISTS ad_variants (
		id       INTEGER PRIMARY KEY AUTOINCREMENT,
		ad_id    TEXT    NOT NULL REFERENCES administrative_divisions(id) ON DELETE CASCADE,
		position INTEGER NOT NULL,
		value    TEXT    NOT NULL
	)`,

	`CREATE TABLE IF NOT EXISTS ad_renames (
		id       INTEGER PRIMARY KEY AUTOINCREMENT,
		ad_id    TEXT    NOT NULL REFERENCES administrative_divisions(id) ON DELETE CASCADE,
		position INTEGER NOT NULL,
		text     TEXT    NOT NULL,
		since    TEXT    NOT NULL DEFAULT '',
		until    TEXT    NOT NULL DEFAULT ''
	)`,

	`CREATE TABLE IF NOT EXISTS ad_successors (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		ad_id       TEXT    NOT NULL REFERENCES administrative_divisions(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	`CREATE TABLE IF NOT EXISTS ad_notes (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		ad_id       TEXT    NOT NULL REFERENCES administrative_divisions(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	// --- церкви и приходы ------------------------------------------------

	`CREATE TABLE IF NOT EXISTS churches (
		id        TEXT PRIMARY KEY,
		name      TEXT NOT NULL,
		parish_id INTEGER REFERENCES text_refs(id) ON DELETE SET NULL
	)`,

	`CREATE TABLE IF NOT EXISTS church_settlements (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		church_id   TEXT    NOT NULL REFERENCES churches(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	`CREATE TABLE IF NOT EXISTS church_variants (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		church_id TEXT    NOT NULL REFERENCES churches(id) ON DELETE CASCADE,
		position  INTEGER NOT NULL,
		value     TEXT    NOT NULL
	)`,

	`CREATE TABLE IF NOT EXISTS church_notes (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		church_id   TEXT    NOT NULL REFERENCES churches(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	`CREATE TABLE IF NOT EXISTS parishes (
		id        TEXT PRIMARY KEY,
		name      TEXT NOT NULL,
		church_id INTEGER REFERENCES text_refs(id) ON DELETE SET NULL,
		since_id  INTEGER REFERENCES dates(id) ON DELETE SET NULL,
		until_id  INTEGER REFERENCES dates(id) ON DELETE SET NULL
	)`,

	`CREATE TABLE IF NOT EXISTS parish_settlements (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		parish_id   TEXT    NOT NULL REFERENCES parishes(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	`CREATE TABLE IF NOT EXISTS parish_notes (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		parish_id   TEXT    NOT NULL REFERENCES parishes(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	// --- события и проживание --------------------------------------------

	`CREATE TABLE IF NOT EXISTS events (
		id       TEXT PRIMARY KEY,
		type     TEXT NOT NULL,
		date_id  INTEGER REFERENCES dates(id) ON DELETE SET NULL,
		place_id INTEGER REFERENCES text_refs(id) ON DELETE SET NULL,
		private  INTEGER NOT NULL DEFAULT 0
	)`,

	`CREATE TABLE IF NOT EXISTS event_participants (
		id        INTEGER PRIMARY KEY AUTOINCREMENT,
		event_id  TEXT    NOT NULL REFERENCES events(id) ON DELETE CASCADE,
		position  INTEGER NOT NULL,
		person_id TEXT    NOT NULL REFERENCES persons(id) ON DELETE RESTRICT,
		role      TEXT    NOT NULL DEFAULT '',
		note      TEXT    NOT NULL DEFAULT ''
	)`,

	`CREATE TABLE IF NOT EXISTS event_notes (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		event_id    TEXT    NOT NULL REFERENCES events(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	`CREATE TABLE IF NOT EXISTS residences (
		id        TEXT PRIMARY KEY,
		person_id TEXT    NOT NULL REFERENCES persons(id) ON DELETE RESTRICT,
		place_id  TEXT    NOT NULL REFERENCES administrative_divisions(id) ON DELETE RESTRICT,
		since_id  INTEGER REFERENCES dates(id) ON DELETE SET NULL,
		until_id  INTEGER REFERENCES dates(id) ON DELETE SET NULL,
		note      TEXT    NOT NULL DEFAULT '',
		private   INTEGER NOT NULL DEFAULT 0
	)`,

	// --- доказательства ---------------------------------------------------

	`CREATE TABLE IF NOT EXISTS sources (
		id            TEXT PRIMARY KEY,
		kind          TEXT NOT NULL,
		title         TEXT NOT NULL,
		author        TEXT NOT NULL DEFAULT '',
		date_id       INTEGER REFERENCES dates(id) ON DELETE SET NULL,
		reliability   TEXT NOT NULL DEFAULT '',
		repository_id TEXT REFERENCES repositories(id) ON DELETE RESTRICT,
		private       INTEGER NOT NULL DEFAULT 0
	)`,

	`CREATE TABLE IF NOT EXISTS source_notes (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		source_id   TEXT    NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	`CREATE TABLE IF NOT EXISTS citations (
		id        TEXT PRIMARY KEY,
		source_id TEXT    NOT NULL REFERENCES sources(id) ON DELETE RESTRICT,
		anchor_id INTEGER REFERENCES anchors(id) ON DELETE SET NULL,
		text      TEXT    NOT NULL DEFAULT '',
		note      TEXT    NOT NULL DEFAULT '',
		private   INTEGER NOT NULL DEFAULT 0
	)`,

	`CREATE TABLE IF NOT EXISTS notes (
		id        TEXT PRIMARY KEY,
		kind      TEXT    NOT NULL DEFAULT '',
		title     TEXT    NOT NULL DEFAULT '',
		text      TEXT    NOT NULL DEFAULT '',
		parent_id TEXT    REFERENCES notes(id) ON DELETE RESTRICT,
		private   INTEGER NOT NULL DEFAULT 0
	)`,

	`CREATE TABLE IF NOT EXISTS repositories (
		id      TEXT PRIMARY KEY,
		name    TEXT    NOT NULL,
		type    TEXT    NOT NULL DEFAULT '',
		address TEXT    NOT NULL DEFAULT '',
		private INTEGER NOT NULL DEFAULT 0
	)`,

	`CREATE TABLE IF NOT EXISTS repository_urls (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		repository_id TEXT    NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
		position      INTEGER NOT NULL,
		text_ref_id   INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	`CREATE TABLE IF NOT EXISTS repository_notes (
		id            INTEGER PRIMARY KEY AUTOINCREMENT,
		repository_id TEXT    NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
		position      INTEGER NOT NULL,
		text_ref_id   INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	// --- архивы -----------------------------------------------------------

	`CREATE TABLE IF NOT EXISTS archives (
		id            TEXT PRIMARY KEY,
		name          TEXT    NOT NULL,
		system_id     INTEGER REFERENCES text_refs(id) ON DELETE SET NULL,
		repository_id TEXT    REFERENCES repositories(id) ON DELETE RESTRICT,
		private       INTEGER NOT NULL DEFAULT 0
	)`,

	`CREATE TABLE IF NOT EXISTS archive_notes (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		archive_id  TEXT    NOT NULL REFERENCES archives(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	`CREATE TABLE IF NOT EXISTS archive_nodes (
		id         TEXT PRIMARY KEY,
		type       TEXT    NOT NULL DEFAULT '',
		archive_id TEXT    NOT NULL REFERENCES archives(id) ON DELETE RESTRICT,
		parent_id  TEXT    REFERENCES archive_nodes(id) ON DELETE RESTRICT,
		label      TEXT    NOT NULL DEFAULT '',
		name       TEXT    NOT NULL DEFAULT '',
		since_id   INTEGER REFERENCES dates(id) ON DELETE SET NULL,
		until_id   INTEGER REFERENCES dates(id) ON DELETE SET NULL,
		parish_id  INTEGER REFERENCES text_refs(id) ON DELETE SET NULL,
		private    INTEGER NOT NULL DEFAULT 0
	)`,

	`CREATE TABLE IF NOT EXISTS node_settlements (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		node_id     TEXT    NOT NULL REFERENCES archive_nodes(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	`CREATE TABLE IF NOT EXISTS node_notes (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		node_id     TEXT    NOT NULL REFERENCES archive_nodes(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	`CREATE TABLE IF NOT EXISTS archive_documents (
		id        TEXT PRIMARY KEY,
		unit_id   TEXT    NOT NULL REFERENCES archive_nodes(id) ON DELETE RESTRICT,
		title     TEXT    NOT NULL,
		kind      TEXT    NOT NULL DEFAULT '',
		since_id  INTEGER REFERENCES dates(id) ON DELETE SET NULL,
		until_id  INTEGER REFERENCES dates(id) ON DELETE SET NULL,
		parish_id INTEGER REFERENCES text_refs(id) ON DELETE SET NULL,
		private   INTEGER NOT NULL DEFAULT 0
	)`,

	`CREATE TABLE IF NOT EXISTS doc_settlements (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		document_id TEXT    NOT NULL REFERENCES archive_documents(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	`CREATE TABLE IF NOT EXISTS doc_notes (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		document_id TEXT    NOT NULL REFERENCES archive_documents(id) ON DELETE CASCADE,
		position    INTEGER NOT NULL,
		text_ref_id INTEGER NOT NULL REFERENCES text_refs(id) ON DELETE RESTRICT
	)`,

	`CREATE TABLE IF NOT EXISTS attachments (
		id          TEXT PRIMARY KEY,
		kind        TEXT    NOT NULL,
		uri         TEXT    NOT NULL DEFAULT '',
		filename    TEXT    NOT NULL DEFAULT '',
		mime        TEXT    NOT NULL DEFAULT '',
		page        INTEGER NOT NULL DEFAULT 0,
		node_id     TEXT    NOT NULL REFERENCES archive_nodes(id) ON DELETE RESTRICT,
		document_id TEXT    REFERENCES archive_documents(id) ON DELETE SET NULL,
		note        TEXT    NOT NULL DEFAULT '',
		private     INTEGER NOT NULL DEFAULT 0
	)`,

	// --- связь «утверждение → цитата» -------------------------------------

	`CREATE TABLE IF NOT EXISTS source_links (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		citation_id TEXT NOT NULL REFERENCES citations(id) ON DELETE RESTRICT,
		target_type TEXT NOT NULL,
		target_id   TEXT NOT NULL,
		reliability TEXT NOT NULL DEFAULT '',
		role        TEXT NOT NULL DEFAULT '',
		note        TEXT NOT NULL DEFAULT ''
	)`,

	// очистка ссылок при удалении сущности-владельца идёт по (type, id)
	`CREATE INDEX IF NOT EXISTS idx_source_links_target
		ON source_links (target_type, target_id)`,

	// --- поисковый индекс --------------------------------------------------

	// Термины нормализуются в Go (Normalize): нижний регистр, ё→е, без
	// диакритики. Обе стороны сравнения нормализованы, поэтому коллация
	// бинарная, а запрос — обычный LIKE.
	`CREATE TABLE IF NOT EXISTS search_index (
		entity_table TEXT NOT NULL,
		entity_id    TEXT NOT NULL,
		field        TEXT NOT NULL,
		term         TEXT NOT NULL COLLATE BINARY,
		PRIMARY KEY (entity_table, entity_id, field, term)
	)`,
}
