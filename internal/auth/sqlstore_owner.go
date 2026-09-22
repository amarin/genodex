package auth

import (
	"context"
	"database/sql"
	"errors"
)

// CreateOwner сохраняет нового владельца.
func (s *SQLStore) CreateOwner(ctx context.Context, o Owner) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO owners(id, login, password_hash, created_at) VALUES (?, ?, ?, ?)`,
		string(o.ID), o.Login, o.PasswordHash, timeToSQL(o.CreatedAt))

	return err
}

// GetOwnerByLogin читает владельца по логину; нет — ErrNotFound.
func (s *SQLStore) GetOwnerByLogin(ctx context.Context, login string) (*Owner, error) {
	return scanOwner(s.db.QueryRowContext(ctx,
		`SELECT id, login, password_hash, created_at FROM owners WHERE login = ?`, login))
}

// GetOwner читает владельца по id; нет — ErrNotFound.
func (s *SQLStore) GetOwner(ctx context.Context, id ID) (*Owner, error) {
	return scanOwner(s.db.QueryRowContext(ctx,
		`SELECT id, login, password_hash, created_at FROM owners WHERE id = ?`, string(id)))
}

// UpdateOwnerPassword заменяет хеш пароля; нет такого владельца — ErrNotFound.
func (s *SQLStore) UpdateOwnerPassword(ctx context.Context, id ID, hash string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE owners SET password_hash = ? WHERE id = ?`, hash, string(id))
	if err != nil {
		return err
	}

	return checkRowsAffected(res)
}

// CountOwners считает владельцев (Bootstrap/Register).
func (s *SQLStore) CountOwners(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM owners`).Scan(&n)

	return n, err
}

// scanOwner читает *sql.Row; sql.ErrNoRows → ErrNotFound.
func scanOwner(row *sql.Row) (*Owner, error) {
	var id, login, hash, createdAt string

	if err := row.Scan(&id, &login, &hash, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	t, err := timeFromSQL(createdAt)
	if err != nil {
		return nil, err
	}

	return &Owner{ID: ID(id), Login: login, PasswordHash: hash, CreatedAt: t}, nil
}
