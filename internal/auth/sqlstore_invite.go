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

// MarkInviteUsed отмечает приглашение использованным; нет такого приглашения
// ИЛИ оно уже использовано — ErrNotFound.
//
// Условие "used_at IS NULL" в UPDATE — единственный уровень, на котором
// одноразовость invite реально гарантируется на данных: Service.checkInvite
// проверяет "уже использован" в коде приложения ДО этого вызова, а это
// check-then-act. Два параллельных Register с одним invite-токеном могут оба
// пройти checkInvite; без условия в WHERE второй UPDATE тихо перезаписал бы
// used_at/used_by первого. С условием второй вызов затрагивает 0 строк, и
// checkRowsAffected превращает это в ErrNotFound. Это не закрывает гонку
// целиком — см. auth.md §9 про остаточный риск двух Owner-строк.
//
// Как и RevokeAPIToken/TouchAPIToken, использует time.Now() напрямую, а не
// Service.now — UsedAt здесь только nil-проверяется, точное значение нигде
// не сравнивается, так что это не мешает сегодняшним тестам.
func (s *SQLStore) MarkInviteUsed(ctx context.Context, id ID, by ID) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE invites SET used_at = ?, used_by = ? WHERE id = ? AND used_at IS NULL`,
		timeToSQL(time.Now()), string(by), string(id))
	if err != nil {
		return err
	}

	return checkRowsAffected(res)
}
