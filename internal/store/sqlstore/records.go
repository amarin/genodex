package sqlstore

import (
	"context"
	"database/sql"

	"github.com/amarin/genodex/internal/models"
)

// --- Event ----------------------------------------------------------------

// SaveEvent сохраняет событие вместе с коллекцией участников.
func (s *Store) SaveEvent(ctx context.Context, e *models.Event) (err error) {
	defer wrapSave(&err, "event", e.ID)

	return s.inTx(ctx, func(tx runner) error {
		old, err := collectMainRefs(tx, "events", string(e.ID),
			[]string{"date_id"}, []string{"place_id"}, nil)
		if err != nil {
			return err
		}

		dateID, err := insertDate(tx, e.Date)
		if err != nil {
			return err
		}

		placeID, err := insertTextRef(tx, placeRefToTextRef(e.Place))
		if err != nil {
			return err
		}

		if _, err := tx.Exec(
			`INSERT INTO events(id, type, date_id, place_id, private) VALUES (?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET type = excluded.type, date_id = excluded.date_id,
				place_id = excluded.place_id, private = excluded.private`,
			string(e.ID), string(e.Type), nullInt64(dateID), nullInt64(placeID), boolInt(e.Private),
		); err != nil {
			return err
		}

		if err := old.drop(tx); err != nil {
			return err
		}

		if err := clearChildren(tx, "event_participants", "event_id", string(e.ID)); err != nil {
			return err
		}

		for i, p := range e.Participants {
			if _, err := tx.Exec(
				`INSERT INTO event_participants(event_id, position, person_id, role, note)
				 VALUES (?, ?, ?, ?, ?)`,
				string(e.ID), i, string(p.PersonID), p.Role, p.Note,
			); err != nil {
				return err
			}
		}

		if err := replaceTextRefList(tx, "event_notes", "event_id", string(e.ID), e.Notes); err != nil {
			return err
		}

		if err := replaceSourceLinks(tx, models.TypeEvent, e.ID, e.Sources); err != nil {
			return err
		}

		var place string
		if e.Place != nil {
			place = e.Place.Text
		}

		return replaceSearchIndex(tx, "events", e.ID, map[string][]string{"place": {place}})
	})
}

// placeRefToTextRef переводит указание на место в общий TextRef.
func placeRefToTextRef(p *models.PlaceRef) *models.TextRef {
	if p == nil {
		return nil
	}

	return &models.TextRef{Text: p.Text, Ref: p.Ref, Type: p.Type}
}

// textRefToPlaceRef — обратный перевод; nil остаётся nil.
func textRefToPlaceRef(tr *models.TextRef) *models.PlaceRef {
	if tr == nil {
		return nil
	}

	return &models.PlaceRef{Text: tr.Text, Ref: tr.Ref, Type: tr.Type}
}

// GetEvent читает событие по id; не найдено — models.ErrNotFound.
func (s *Store) GetEvent(ctx context.Context, id models.ID) (*models.Event, error) {
	var (
		e                models.Event
		rawID, eventType string
		dateID, placeID  sql.NullInt64
	)

	err := s.run(ctx).QueryRow(
		`SELECT id, type, date_id, place_id, private FROM events WHERE id = ?`, string(id),
	).Scan(&rawID, &eventType, &dateID, &placeID, &e.Private)
	if notFound(err) {
		return nil, models.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	e.ID, e.Type = models.ID(rawID), models.EventType(eventType)

	if e.Date, err = loadDate(s.run(ctx), int64From(dateID)); err != nil {
		return nil, err
	}

	place, err := loadTextRefPtr(s.run(ctx), int64From(placeID))
	if err != nil {
		return nil, err
	}

	e.Place = textRefToPlaceRef(place)

	if e.Participants, err = loadParticipants(s.run(ctx), id); err != nil {
		return nil, err
	}

	if e.Sources, err = loadSourceLinks(s.run(ctx), models.TypeEvent, id); err != nil {
		return nil, err
	}

	if e.Notes, err = loadTextRefList(s.run(ctx), "event_notes", "event_id", string(id)); err != nil {
		return nil, err
	}

	return &e, nil
}

// loadParticipants читает участников события в порядке position.
func loadParticipants(q queryer, id models.ID) ([]models.EventParticipant, error) {
	return scanRows(q, func(r *sql.Rows) (models.EventParticipant, error) {
		var (
			p        models.EventParticipant
			personID string
		)

		if err := r.Scan(&personID, &p.Role, &p.Note); err != nil {
			return p, err
		}

		p.PersonID = models.ID(personID)

		return p, nil
	},
		`SELECT person_id, role, note FROM event_participants
		 WHERE event_id = ? ORDER BY position`, string(id))
}

// ListEvents возвращает все события в порядке вставки.
func (s *Store) ListEvents(ctx context.Context, access models.Access, page models.Page) ([]*models.Event, error) {
	return listEntities(ctx, s, "events", access, page, s.GetEvent)
}

// --- Source ---------------------------------------------------------------

// SaveSource сохраняет источник. RepositoryID — необязательная строгая
// ссылка: пустой ID пишется как SQL NULL (иначе FK RESTRICT отвергнет вставку).
func (s *Store) SaveSource(ctx context.Context, src *models.Source) (err error) {
	defer wrapSave(&err, "source", src.ID)

	return s.inTx(ctx, func(tx runner) error {
		old, err := collectMainRefs(tx, "sources", string(src.ID), []string{"date_id"}, nil, nil)
		if err != nil {
			return err
		}

		dateID, err := insertDate(tx, src.Date)
		if err != nil {
			return err
		}

		if _, err := tx.Exec(
			`INSERT INTO sources(id, kind, title, author, date_id, reliability, repository_id, private)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET kind = excluded.kind, title = excluded.title,
				author = excluded.author, date_id = excluded.date_id,
				reliability = excluded.reliability, repository_id = excluded.repository_id,
				private = excluded.private`,
			string(src.ID), string(src.Kind), src.Title, src.Author,
			nullInt64(dateID), string(src.Reliability), nullID(src.RepositoryID), boolInt(src.Private),
		); err != nil {
			return err
		}

		if err := old.drop(tx); err != nil {
			return err
		}

		if err := replaceTextRefList(tx, "source_notes", "source_id", string(src.ID), src.Notes); err != nil {
			return err
		}

		// у Source нет поля Sources: доказательство само не доказывается.
		return replaceSearchIndex(tx, "sources", src.ID, map[string][]string{
			"title":  {src.Title},
			"author": {src.Author},
		})
	})
}

// GetSource читает источник по id; не найден — models.ErrNotFound.
func (s *Store) GetSource(ctx context.Context, id models.ID) (*models.Source, error) {
	var (
		src              models.Source
		rawID, kind, rel string
		dateID           sql.NullInt64
		repositoryID     sql.NullString
	)

	err := s.run(ctx).QueryRow(
		`SELECT id, kind, title, author, date_id, reliability, repository_id, private
		 FROM sources WHERE id = ?`, string(id),
	).Scan(&rawID, &kind, &src.Title, &src.Author, &dateID, &rel, &repositoryID, &src.Private)
	if notFound(err) {
		return nil, models.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	src.ID, src.Kind = models.ID(rawID), models.SourceKind(kind)
	src.Reliability = models.Reliability(rel)
	src.RepositoryID = idFrom(repositoryID)

	if src.Date, err = loadDate(s.run(ctx), int64From(dateID)); err != nil {
		return nil, err
	}

	if src.Notes, err = loadTextRefList(s.run(ctx), "source_notes", "source_id", string(id)); err != nil {
		return nil, err
	}

	return &src, nil
}

// ListSources возвращает все источники в порядке вставки.
func (s *Store) ListSources(ctx context.Context, access models.Access, page models.Page) ([]*models.Source, error) {
	return listEntities(ctx, s, "sources", access, page, s.GetSource)
}

// --- Citation -------------------------------------------------------------

// SaveCitation сохраняет цитату из источника вместе с якорем.
func (s *Store) SaveCitation(ctx context.Context, c *models.Citation) (err error) {
	defer wrapSave(&err, "citation", c.ID)

	return s.inTx(ctx, func(tx runner) error {
		old, err := collectMainRefs(tx, "citations", string(c.ID), nil, nil, []string{"anchor_id"})
		if err != nil {
			return err
		}

		anchorID, err := insertAnchor(tx, c.Anchor)
		if err != nil {
			return err
		}

		if _, err := tx.Exec(
			`INSERT INTO citations(id, source_id, anchor_id, text, note, private)
			 VALUES (?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET source_id = excluded.source_id,
				anchor_id = excluded.anchor_id, text = excluded.text,
				note = excluded.note, private = excluded.private`,
			string(c.ID), string(c.SourceID), nullInt64(anchorID), c.Text, c.Note, boolInt(c.Private),
		); err != nil {
			return err
		}

		if err := old.drop(tx); err != nil {
			return err
		}

		return replaceSearchIndex(tx, "citations", c.ID, map[string][]string{"text": {c.Text}})
	})
}

// GetCitation читает цитату по id; не найдена — models.ErrNotFound.
func (s *Store) GetCitation(ctx context.Context, id models.ID) (*models.Citation, error) {
	var (
		c               models.Citation
		rawID, sourceID string
		anchorID        sql.NullInt64
	)

	err := s.run(ctx).QueryRow(
		`SELECT id, source_id, anchor_id, text, note, private FROM citations WHERE id = ?`, string(id),
	).Scan(&rawID, &sourceID, &anchorID, &c.Text, &c.Note, &c.Private)
	if notFound(err) {
		return nil, models.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	c.ID, c.SourceID = models.ID(rawID), models.ID(sourceID)

	if c.Anchor, err = loadAnchor(s.run(ctx), int64From(anchorID)); err != nil {
		return nil, err
	}

	return &c, nil
}

// ListCitations возвращает все цитаты в порядке вставки.
func (s *Store) ListCitations(ctx context.Context, access models.Access, page models.Page) ([]*models.Citation, error) {
	return listEntities(ctx, s, "citations", access, page, s.GetCitation)
}

// --- Note -----------------------------------------------------------------

// SaveNote сохраняет заметку (иерархия «книга → главы» через parent_id).
func (s *Store) SaveNote(ctx context.Context, n *models.Note) (err error) {
	defer wrapSave(&err, "note", n.ID)

	return s.inTx(ctx, func(tx runner) error {
		if _, err := tx.Exec(
			`INSERT INTO notes(id, kind, title, text, parent_id, private) VALUES (?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET kind = excluded.kind, title = excluded.title,
				text = excluded.text, parent_id = excluded.parent_id, private = excluded.private`,
			string(n.ID), string(n.Kind), n.Title, n.Text, nullIDPtr(n.ParentID), boolInt(n.Private),
		); err != nil {
			return err
		}

		if err := replaceSourceLinks(tx, models.TypeNote, n.ID, n.Sources); err != nil {
			return err
		}

		return replaceSearchIndex(tx, "notes", n.ID, map[string][]string{"title": {n.Title}})
	})
}

// GetNote читает заметку по id; не найдена — models.ErrNotFound.
func (s *Store) GetNote(ctx context.Context, id models.ID) (*models.Note, error) {
	var (
		n           models.Note
		rawID, kind string
		parentID    sql.NullString
	)

	err := s.run(ctx).QueryRow(
		`SELECT id, kind, title, text, parent_id, private FROM notes WHERE id = ?`, string(id),
	).Scan(&rawID, &kind, &n.Title, &n.Text, &parentID, &n.Private)
	if notFound(err) {
		return nil, models.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	n.ID, n.Kind = models.ID(rawID), models.NoteKind(kind)
	n.ParentID = idPtrFrom(parentID)

	if n.Sources, err = loadSourceLinks(s.run(ctx), models.TypeNote, id); err != nil {
		return nil, err
	}

	return &n, nil
}

// ListNotes возвращает все заметки в порядке вставки.
func (s *Store) ListNotes(ctx context.Context, access models.Access, page models.Page) ([]*models.Note, error) {
	return listEntities(ctx, s, "notes", access, page, s.GetNote)
}

// --- Repository -----------------------------------------------------------

// SaveRepository сохраняет хранилище-контейнер источников.
func (s *Store) SaveRepository(ctx context.Context, r *models.Repository) (err error) {
	defer wrapSave(&err, "repository", r.ID)

	return s.inTx(ctx, func(tx runner) error {
		if _, err := tx.Exec(
			`INSERT INTO repositories(id, name, type, address, private) VALUES (?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET name = excluded.name, type = excluded.type,
				address = excluded.address, private = excluded.private`,
			string(r.ID), r.Name, string(r.Type), r.Address, boolInt(r.Private),
		); err != nil {
			return err
		}

		if err := replaceTextRefList(tx, "repository_urls", "repository_id", string(r.ID), r.URLs); err != nil {
			return err
		}

		if err := replaceTextRefList(tx, "repository_notes", "repository_id", string(r.ID), r.Notes); err != nil {
			return err
		}

		if err := replaceSourceLinks(tx, models.TypeRepository, r.ID, r.Sources); err != nil {
			return err
		}

		return replaceSearchIndex(tx, "repositories", r.ID, map[string][]string{"name": {r.Name}})
	})
}

// GetRepository читает хранилище по id; не найдено — models.ErrNotFound.
func (s *Store) GetRepository(ctx context.Context, id models.ID) (*models.Repository, error) {
	var (
		r              models.Repository
		rawID, repType string
	)

	err := s.run(ctx).QueryRow(
		`SELECT id, name, type, address, private FROM repositories WHERE id = ?`, string(id),
	).Scan(&rawID, &r.Name, &repType, &r.Address, &r.Private)
	if notFound(err) {
		return nil, models.ErrNotFound
	}

	if err != nil {
		return nil, err
	}

	r.ID, r.Type = models.ID(rawID), models.RepositoryType(repType)

	if r.URLs, err = loadTextRefList(s.run(ctx), "repository_urls", "repository_id", string(id)); err != nil {
		return nil, err
	}

	if r.Notes, err = loadTextRefList(s.run(ctx), "repository_notes", "repository_id", string(id)); err != nil {
		return nil, err
	}

	if r.Sources, err = loadSourceLinks(s.run(ctx), models.TypeRepository, id); err != nil {
		return nil, err
	}

	return &r, nil
}

// ListRepositories возвращает все хранилища в порядке вставки.
func (s *Store) ListRepositories(ctx context.Context, access models.Access, page models.Page) ([]*models.Repository, error) {
	return listEntities(ctx, s, "repositories", access, page, s.GetRepository)
}
