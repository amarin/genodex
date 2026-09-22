package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

const apiTokenSelect = `SELECT id, owner_id, label, token_hash, created_at, last_used_at, revoked_at
	FROM api_tokens `

// CreateAPIToken сохраняет новый API-токен.
func (s *SQLStore) CreateAPIToken(ctx context.Context, t APIToken) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO api_tokens(id, owner_id, label, token_hash, created_at, last_used_at, revoked_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		string(t.ID), string(t.OwnerID), t.Label, t.TokenHash, timeToSQL(t.CreatedAt),
		nullTime(t.LastUsedAt), nullTime(t.RevokedAt))

	return err
}

// GetAPIToken читает токен по id; нет — ErrNotFound.
func (s *SQLStore) GetAPIToken(ctx context.Context, id ID) (*APIToken, error) {
	return scanAPIToken(s.db.QueryRowContext(ctx, apiTokenSelect+`WHERE id = ?`, string(id)))
}

// GetAPITokenByHash читает токен по хешу; нет — ErrNotFound.
func (s *SQLStore) GetAPITokenByHash(ctx context.Context, hash string) (*APIToken, error) {
	return scanAPIToken(s.db.QueryRowContext(ctx, apiTokenSelect+`WHERE token_hash = ?`, hash))
}

// ListAPITokens отдаёт токены владельца в порядке создания.
func (s *SQLStore) ListAPITokens(ctx context.Context, ownerID ID) ([]APIToken, error) {
	rows, err := s.db.QueryContext(ctx, apiTokenSelect+`WHERE owner_id = ? ORDER BY created_at, id`, string(ownerID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []APIToken{}

	for rows.Next() {
		t, err := scanAPITokenRow(rows)
		if err != nil {
			return nil, err
		}

		out = append(out, *t)
	}

	return out, rows.Err()
}

// RevokeAPIToken отмечает токен отозванным; нет такого — ErrNotFound.
//
// Использует time.Now() напрямую, а не инжектируемые часы Service.now:
// RevokedAt/LastUsedAt (TouchAPIToken) только nil-проверяются или сравниваются
// через Before/After с интервалами в минуты-дни, точное значение нигде не
// проверяется — так что это нормально сегодня, но будет иметь значение, если
// когда-нибудь понадобится детерминированное время на уровне хранилища.
func (s *SQLStore) RevokeAPIToken(ctx context.Context, id ID) error {
	res, err := s.db.ExecContext(ctx, `UPDATE api_tokens SET revoked_at = ? WHERE id = ?`,
		timeToSQL(time.Now()), string(id))
	if err != nil {
		return err
	}

	return checkRowsAffected(res)
}

// TouchAPIToken обновляет last_used_at; нет такого токена — ErrNotFound.
//
// Как и RevokeAPIToken, использует time.Now() напрямую, а не Service.now —
// см. комментарий у RevokeAPIToken.
func (s *SQLStore) TouchAPIToken(ctx context.Context, id ID) error {
	res, err := s.db.ExecContext(ctx, `UPDATE api_tokens SET last_used_at = ? WHERE id = ?`,
		timeToSQL(time.Now()), string(id))
	if err != nil {
		return err
	}

	return checkRowsAffected(res)
}

// scanAPIToken читает *sql.Row; sql.ErrNoRows → ErrNotFound.
func scanAPIToken(row *sql.Row) (*APIToken, error) {
	var (
		id, ownerID, label, hash, createdAt string
		lastUsed, revoked                   sql.NullString
	)

	if err := row.Scan(&id, &ownerID, &label, &hash, &createdAt, &lastUsed, &revoked); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}

		return nil, err
	}

	return buildAPIToken(id, ownerID, label, hash, createdAt, lastUsed, revoked)
}

// scanAPITokenRow читает *sql.Rows (ListAPITokens).
func scanAPITokenRow(rows *sql.Rows) (*APIToken, error) {
	var (
		id, ownerID, label, hash, createdAt string
		lastUsed, revoked                   sql.NullString
	)

	if err := rows.Scan(&id, &ownerID, &label, &hash, &createdAt, &lastUsed, &revoked); err != nil {
		return nil, err
	}

	return buildAPIToken(id, ownerID, label, hash, createdAt, lastUsed, revoked)
}

func buildAPIToken(id, ownerID, label, hash, createdAt string, lastUsed, revoked sql.NullString) (*APIToken, error) {
	t := &APIToken{ID: ID(id), OwnerID: ID(ownerID), Label: label, TokenHash: hash}

	var err error

	if t.CreatedAt, err = timeFromSQL(createdAt); err != nil {
		return nil, err
	}

	if t.LastUsedAt, err = timePtrFromNull(lastUsed); err != nil {
		return nil, err
	}

	if t.RevokedAt, err = timePtrFromNull(revoked); err != nil {
		return nil, err
	}

	return t, nil
}
