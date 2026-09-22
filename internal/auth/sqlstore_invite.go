package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

// CreateInvite сохраняет новую invite-ссылку.
func (s *SQLStore) CreateInvite(ctx context.Context, i Invite) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO invites(id, created_by, token_hash, expires_at, used_at, used_by, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		string(i.ID), string(i.CreatedBy), i.TokenHash, timeToSQL(i.ExpiresAt),
		nullTime(i.UsedAt), nullIDPtr(i.UsedBy), timeToSQL(i.CreatedAt))

	return err
}

// GetInviteByHash читает приглашение по хешу; нет — ErrNotFound.
func (s *SQLStore) GetInviteByHash(ctx context.Context, hash string) (*Invite, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, created_by, token_hash, expires_at, used_at, used_by, created_at
		 FROM invites WHERE token_hash = ?`, hash)

	var (
		id, createdBy, tokenHash, expiresAt, createdAt string
		usedAt, usedBy                                 sql.NullString
	)

	if err := row.Scan(&id, &createdBy, &tokenHash, &expiresAt, &usedAt, &usedBy, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	inv := &Invite{ID: ID(id), CreatedBy: ID(createdBy), TokenHash: tokenHash, UsedBy: idPtrFromNull(usedBy)}

	var err error

	if inv.ExpiresAt, err = timeFromSQL(expiresAt); err != nil {
		return nil, err
	}

	if inv.CreatedAt, err = timeFromSQL(createdAt); err != nil {
		return nil, err
	}

	if inv.UsedAt, err = timePtrFromNull(usedAt); err != nil {
		return nil, err
	}

	return inv, nil
}

// MarkInviteUsed отмечает приглашение использованным; нет такого — ErrNotFound.
func (s *SQLStore) MarkInviteUsed(ctx context.Context, id ID, by ID) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE invites SET used_at = ?, used_by = ? WHERE id = ?`,
		timeToSQL(time.Now()), string(by), string(id))
	if err != nil {
		return err
	}

	return checkRowsAffected(res)
}
