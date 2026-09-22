package auth

import (
	"context"
	"database/sql"
	"errors"
)

const sessionSelect = `SELECT id, owner_id, access_token_hash, access_expires_at,
	refresh_token_hash, refresh_expires_at, created_at FROM sessions `

// CreateSession сохраняет новую сессию.
func (s *SQLStore) CreateSession(ctx context.Context, sess Session) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions(id, owner_id, access_token_hash, access_expires_at,
		 refresh_token_hash, refresh_expires_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		string(sess.ID), string(sess.OwnerID), sess.AccessTokenHash, timeToSQL(sess.AccessExpiresAt),
		sess.RefreshTokenHash, timeToSQL(sess.RefreshExpiresAt), timeToSQL(sess.CreatedAt))

	return err
}

// GetSessionByAccessHash читает сессию по хешу access-токена; нет — ErrNotFound.
func (s *SQLStore) GetSessionByAccessHash(ctx context.Context, hash string) (*Session, error) {
	return scanSession(s.db.QueryRowContext(ctx, sessionSelect+`WHERE access_token_hash = ?`, hash))
}

// GetSessionByRefreshHash читает сессию по хешу refresh-токена; нет — ErrNotFound.
func (s *SQLStore) GetSessionByRefreshHash(ctx context.Context, hash string) (*Session, error) {
	return scanSession(s.db.QueryRowContext(ctx, sessionSelect+`WHERE refresh_token_hash = ?`, hash))
}

// ReplaceSession перезаписывает пару токенов существующей сессии (ротация в
// Service.rotateSession); нет такой сессии — ErrNotFound.
func (s *SQLStore) ReplaceSession(ctx context.Context, id ID, sess Session) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE sessions SET owner_id = ?, access_token_hash = ?, access_expires_at = ?,
		 refresh_token_hash = ?, refresh_expires_at = ?, created_at = ? WHERE id = ?`,
		string(sess.OwnerID), sess.AccessTokenHash, timeToSQL(sess.AccessExpiresAt),
		sess.RefreshTokenHash, timeToSQL(sess.RefreshExpiresAt), timeToSQL(sess.CreatedAt), string(id))
	if err != nil {
		return err
	}

	return checkRowsAffected(res)
}

// DeleteSession удаляет сессию (Logout); нет такой — ErrNotFound.
func (s *SQLStore) DeleteSession(ctx context.Context, id ID) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, string(id))
	if err != nil {
		return err
	}

	return checkRowsAffected(res)
}

// DeleteSessionsByOwner удаляет все сессии владельца (ChangePassword);
// отсутствие сессий — не ошибка.
func (s *SQLStore) DeleteSessionsByOwner(ctx context.Context, ownerID ID) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE owner_id = ?`, string(ownerID))

	return err
}

// scanSession читает *sql.Row; sql.ErrNoRows → ErrNotFound.
func scanSession(row *sql.Row) (*Session, error) {
	var (
		id, ownerID                              string
		accessHash, refreshHash                  string
		accessExpires, refreshExpires, createdAt string
	)

	if err := row.Scan(&id, &ownerID, &accessHash, &accessExpires,
		&refreshHash, &refreshExpires, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	sess := &Session{ID: ID(id), OwnerID: ID(ownerID), AccessTokenHash: accessHash, RefreshTokenHash: refreshHash}

	var err error

	if sess.AccessExpiresAt, err = timeFromSQL(accessExpires); err != nil {
		return nil, err
	}

	if sess.RefreshExpiresAt, err = timeFromSQL(refreshExpires); err != nil {
		return nil, err
	}

	if sess.CreatedAt, err = timeFromSQL(createdAt); err != nil {
		return nil, err
	}

	return sess, nil
}
