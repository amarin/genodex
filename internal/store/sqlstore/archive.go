package sqlstore

import (
	"context"
	"database/sql"

	"github.com/amarin/genodex/internal/models"
)

// --- Archive --------------------------------------------------------------

// SaveArchive сохраняет архив. RepositoryID — необязательная строгая ссылка:
// пустой ID пишется как SQL NULL.
func (s *Store) SaveArchive(ctx context.Context, a *models.Archive) (err error) {
	defer wrapSave(&err, "archive", a.ID)

	return s.inTx(ctx, func(tx runner) error {
		old, err := collectMainRefs(tx, "archives", string(a.ID), nil, []string{"system_id"}, nil)
		if err != nil {
			return err
		}

		systemID, err := insertTextRef(tx, a.System)
		if err != nil {
			return err
		}

		if _, err := tx.Exec(
			`INSERT INTO archives(id, name, system_id, repository_id, private) VALUES (?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET name = excluded.name, system_id = excluded.system_id,
				repository_id = excluded.repository_id, private = excluded.private`,
			string(a.ID), a.Name, nullInt64(systemID), nullID(a.RepositoryID), boolInt(a.Private),
		); err != nil {
			return err
		}

		if err := old.drop(tx); err != nil {
			return err
		}

		if err := replaceTextRefList(tx, "archive_notes", "archive_id", string(a.ID), a.Notes); err != nil {
			return err
		}

		if err := replaceSourceLinks(tx, models.TypeArchive, a.ID, a.Sources); err != nil {
			return err
		}

		return replaceSearchIndex(tx, "archives", a.ID, map[string][]string{"name": {a.Name}})
	})
}

// GetArchive читает архив по id; не найден — models.ErrNotFound.
func (s *Store) GetArchive(ctx context.Context, id models.ID) (*models.Archive, error) {
	var (
		a            models.Archive
		rawID        string
		systemID     sql.NullInt64
		repositoryID sql.NullString
	)

	err := s.run(ctx).QueryRow(
		`SELECT id, name, system_id, repository_id, private FROM archives WHERE id = ?`, string(id),
	).Scan(&rawID, &a.Name, &systemID, &repositoryID, &a.Private)
	if notFound(err) {
		return nil, models.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	a.ID, a.RepositoryID = models.ID(rawID), idFrom(repositoryID)

	if a.System, err = loadTextRefPtr(s.run(ctx), int64From(systemID)); err != nil {
		return nil, err
	}

	if a.Notes, err = loadTextRefList(s.run(ctx), "archive_notes", "archive_id", string(id)); err != nil {
		return nil, err
	}

	if a.Sources, err = loadSourceLinks(s.run(ctx), models.TypeArchive, id); err != nil {
		return nil, err
	}

	return &a, nil
}

// ListArchives возвращает все архивы в порядке вставки.
func (s *Store) ListArchives(ctx context.Context, access models.Access, page models.Page) ([]*models.Archive, error) {
	return listEntities(ctx, s, "archives", access, page, s.GetArchive)
}

// --- ArchiveNode ----------------------------------------------------------

// SaveArchiveNode сохраняет узел цепочки хранения.
func (s *Store) SaveArchiveNode(ctx context.Context, n *models.ArchiveNode) (err error) {
	defer wrapSave(&err, "archive node", n.ID)

	return s.inTx(ctx, func(tx runner) error {
		old, err := collectMainRefs(tx, "archive_nodes", string(n.ID),
			[]string{"since_id", "until_id"}, []string{"parish_id"}, nil)
		if err != nil {
			return err
		}

		parishID, err := insertTextRef(tx, n.Parish)
		if err != nil {
			return err
		}

		sinceID, err := insertDate(tx, n.Since)
		if err != nil {
			return err
		}

		untilID, err := insertDate(tx, n.Until)
		if err != nil {
			return err
		}

		if _, err := tx.Exec(
			`INSERT INTO archive_nodes(id, type, archive_id, parent_id, label, name,
				since_id, until_id, parish_id, private)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET type = excluded.type, archive_id = excluded.archive_id,
				parent_id = excluded.parent_id, label = excluded.label, name = excluded.name,
				since_id = excluded.since_id, until_id = excluded.until_id,
				parish_id = excluded.parish_id, private = excluded.private`,
			string(n.ID), string(n.Type), string(n.ArchiveID), nullIDPtr(n.ParentID),
			n.Label, n.Name, nullInt64(sinceID), nullInt64(untilID),
			nullInt64(parishID), boolInt(n.Private),
		); err != nil {
			return err
		}

		if err := old.drop(tx); err != nil {
			return err
		}

		if err := replaceTextRefList(tx, "node_settlements", "node_id", string(n.ID), n.Settlements); err != nil {
			return err
		}

		if err := replaceTextRefList(tx, "node_notes", "node_id", string(n.ID), n.Notes); err != nil {
			return err
		}

		if err := replaceSourceLinks(tx, models.TypeArchiveNode, n.ID, n.Sources); err != nil {
			return err
		}

		return replaceSearchIndex(tx, "archive_nodes", n.ID,
			map[string][]string{"name": {n.Label, n.Name}})
	})
}

// GetArchiveNode читает узел по id; не найден — models.ErrNotFound.
func (s *Store) GetArchiveNode(ctx context.Context, id models.ID) (*models.ArchiveNode, error) {
	var (
		n                          models.ArchiveNode
		rawID, nodeType, archiveID string
		parentID                   sql.NullString
		sinceID, untilID, parishID sql.NullInt64
	)

	err := s.run(ctx).QueryRow(
		`SELECT id, type, archive_id, parent_id, label, name, since_id, until_id, parish_id, private
		 FROM archive_nodes WHERE id = ?`, string(id),
	).Scan(&rawID, &nodeType, &archiveID, &parentID, &n.Label, &n.Name,
		&sinceID, &untilID, &parishID, &n.Private)
	if notFound(err) {
		return nil, models.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	n.ID, n.Type = models.ID(rawID), models.ArchiveNodeType(nodeType)
	n.ArchiveID, n.ParentID = models.ID(archiveID), idPtrFrom(parentID)

	if n.Since, err = loadDate(s.run(ctx), int64From(sinceID)); err != nil {
		return nil, err
	}

	if n.Until, err = loadDate(s.run(ctx), int64From(untilID)); err != nil {
		return nil, err
	}

	if n.Parish, err = loadTextRefPtr(s.run(ctx), int64From(parishID)); err != nil {
		return nil, err
	}

	if n.Settlements, err = loadTextRefList(s.run(ctx), "node_settlements", "node_id", string(id)); err != nil {
		return nil, err
	}

	if n.Notes, err = loadTextRefList(s.run(ctx), "node_notes", "node_id", string(id)); err != nil {
		return nil, err
	}

	if n.Sources, err = loadSourceLinks(s.run(ctx), models.TypeArchiveNode, id); err != nil {
		return nil, err
	}

	return &n, nil
}

// ListArchiveNodes возвращает все узлы в порядке вставки.
func (s *Store) ListArchiveNodes(ctx context.Context, access models.Access, page models.Page) ([]*models.ArchiveNode, error) {
	return listEntities(ctx, s, "archive_nodes", access, page, s.GetArchiveNode)
}

// --- ArchiveDocument ------------------------------------------------------

// SaveArchiveDocument сохраняет документ внутри единицы учёта.
func (s *Store) SaveArchiveDocument(ctx context.Context, d *models.ArchiveDocument) (err error) {
	defer wrapSave(&err, "archive document", d.ID)

	return s.inTx(ctx, func(tx runner) error {
		old, err := collectMainRefs(tx, "archive_documents", string(d.ID),
			[]string{"since_id", "until_id"}, []string{"parish_id"}, nil)
		if err != nil {
			return err
		}

		parishID, err := insertTextRef(tx, d.Parish)
		if err != nil {
			return err
		}

		sinceID, err := insertDate(tx, d.Since)
		if err != nil {
			return err
		}

		untilID, err := insertDate(tx, d.Until)
		if err != nil {
			return err
		}

		if _, err := tx.Exec(
			`INSERT INTO archive_documents(id, unit_id, title, kind, since_id, until_id, parish_id, private)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET unit_id = excluded.unit_id, title = excluded.title,
				kind = excluded.kind, since_id = excluded.since_id, until_id = excluded.until_id,
				parish_id = excluded.parish_id, private = excluded.private`,
			string(d.ID), string(d.UnitID), d.Title, d.Kind,
			nullInt64(sinceID), nullInt64(untilID), nullInt64(parishID), boolInt(d.Private),
		); err != nil {
			return err
		}

		if err := old.drop(tx); err != nil {
			return err
		}

		if err := replaceTextRefList(tx, "doc_settlements", "document_id", string(d.ID), d.Settlements); err != nil {
			return err
		}

		if err := replaceTextRefList(tx, "doc_notes", "document_id", string(d.ID), d.Notes); err != nil {
			return err
		}

		if err := replaceSourceLinks(tx, models.TypeArchiveDocument, d.ID, d.Sources); err != nil {
			return err
		}

		return replaceSearchIndex(tx, "archive_documents", d.ID,
			map[string][]string{"title": {d.Title}})
	})
}

// GetArchiveDocument читает документ по id; не найден — models.ErrNotFound.
func (s *Store) GetArchiveDocument(ctx context.Context, id models.ID) (*models.ArchiveDocument, error) {
	var (
		d                          models.ArchiveDocument
		rawID, unitID              string
		sinceID, untilID, parishID sql.NullInt64
	)

	err := s.run(ctx).QueryRow(
		`SELECT id, unit_id, title, kind, since_id, until_id, parish_id, private
		 FROM archive_documents WHERE id = ?`, string(id),
	).Scan(&rawID, &unitID, &d.Title, &d.Kind, &sinceID, &untilID, &parishID, &d.Private)
	if notFound(err) {
		return nil, models.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	d.ID, d.UnitID = models.ID(rawID), models.ID(unitID)

	if d.Since, err = loadDate(s.run(ctx), int64From(sinceID)); err != nil {
		return nil, err
	}

	if d.Until, err = loadDate(s.run(ctx), int64From(untilID)); err != nil {
		return nil, err
	}

	if d.Parish, err = loadTextRefPtr(s.run(ctx), int64From(parishID)); err != nil {
		return nil, err
	}

	if d.Settlements, err = loadTextRefList(s.run(ctx), "doc_settlements", "document_id", string(id)); err != nil {
		return nil, err
	}

	if d.Notes, err = loadTextRefList(s.run(ctx), "doc_notes", "document_id", string(id)); err != nil {
		return nil, err
	}

	if d.Sources, err = loadSourceLinks(s.run(ctx), models.TypeArchiveDocument, id); err != nil {
		return nil, err
	}

	return &d, nil
}

// ListArchiveDocuments возвращает все документы в порядке вставки.
func (s *Store) ListArchiveDocuments(ctx context.Context, access models.Access, page models.Page) ([]*models.ArchiveDocument, error) {
	return listEntities(ctx, s, "archive_documents", access, page, s.GetArchiveDocument)
}

// --- Attachment -----------------------------------------------------------

// SaveAttachment сохраняет файловое вложение. DocumentID — необязательная
// ссылка: nil/пусто пишется как SQL NULL.
func (s *Store) SaveAttachment(ctx context.Context, a *models.Attachment) (err error) {
	defer wrapSave(&err, "attachment", a.ID)

	return s.inTx(ctx, func(tx runner) error {
		if _, err := tx.Exec(
			`INSERT INTO attachments(id, kind, uri, filename, mime, page, node_id, document_id, note, private)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET kind = excluded.kind, uri = excluded.uri,
				filename = excluded.filename, mime = excluded.mime, page = excluded.page,
				node_id = excluded.node_id, document_id = excluded.document_id,
				note = excluded.note, private = excluded.private`,
			string(a.ID), string(a.Kind), a.URI, a.Filename, a.MIME, a.Page,
			string(a.NodeID), nullIDPtr(a.DocumentID), a.Note, boolInt(a.Private),
		); err != nil {
			return err
		}

		return replaceSearchIndex(tx, "attachments", a.ID, map[string][]string{
			"filename": {a.Filename},
			"uri":      {a.URI},
		})
	})
}

// GetAttachment читает вложение по id; не найдено — models.ErrNotFound.
func (s *Store) GetAttachment(ctx context.Context, id models.ID) (*models.Attachment, error) {
	var (
		a           models.Attachment
		rawID, kind string
		nodeID      string
		documentID  sql.NullString
	)

	err := s.run(ctx).QueryRow(
		`SELECT id, kind, uri, filename, mime, page, node_id, document_id, note, private
		 FROM attachments WHERE id = ?`, string(id),
	).Scan(&rawID, &kind, &a.URI, &a.Filename, &a.MIME, &a.Page,
		&nodeID, &documentID, &a.Note, &a.Private)
	if notFound(err) {
		return nil, models.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	a.ID, a.Kind = models.ID(rawID), models.AttachmentKind(kind)
	a.NodeID, a.DocumentID = models.ID(nodeID), idPtrFrom(documentID)

	return &a, nil
}

// ListAttachments возвращает все вложения в порядке вставки.
func (s *Store) ListAttachments(ctx context.Context, access models.Access, page models.Page) ([]*models.Attachment, error) {
	return listEntities(ctx, s, "attachments", access, page, s.GetAttachment)
}
