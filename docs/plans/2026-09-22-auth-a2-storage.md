# Этап A2. `internal/auth` — хранилище: план этапа

Спека — [auth.md](../data-model/auth.md) §3; дорожная карта —
[auth-roadmap.md](2026-09-22-auth-roadmap.md), раздел A2. Формат — как этапы
S1–S16 и этап A. Исполняется в `main` подрядными маленькими коммитами.

## Goal

Реализация порта `auth.Store` (20 методов, `internal/auth/deps.go`) поверх
`internal/storage` (то же SQLite-соединение, что и у `internal/store/sqlstore`,
но свои четыре таблицы). После этапа `internal/auth.Service` можно поднять на
реальном хранилище, не только на фейке из этапа A.

## Предпосылка из этапа A (уточнение роадмапа)

Роадмап предполагал, что генеральные тесты `internal/store/sqlstore`
(`TestEntityTablesMatchSchema`, `TestSchemaGraphShapeIsSupported`,
`TestTypeRegistryIsConsistent`, тесты `Search`) не увидят auth-таблицы вообще.
На практике `fkgraph.go` строит граф внешних ключей по `PRAGMA
foreign_key_list` **со всех таблиц базы** (не только `entityTables`) — это
намеренно, чтобы новая связная таблица подхватывалась автоматически
(`storage.createFKIndexes` работает так же). Значит:

- `TestTypeRegistryIsConsistent` и тесты `Search` действительно не видят
  auth-таблицы (они оперируют `entityTables`/`search_index` напрямую) —
  предпосылка верна.
- `TestEntityTablesMatchSchema` классифицирует **любую** таблицу базы, у
  которой нет `entityTables`-записи, как обязанную быть либо «дочерней»
  (есть `CASCADE`-ссылка на владельца — `schemaGraph.owner` смотрит **только**
  `ON DELETE CASCADE`), либо перечисленной в `serviceTables` (карта
  «служебных» таблиц — уже существующий, специально для этого созданный
  механизм: туда попадают `dates`/`text_refs`/`anchors`/`source_links`/
  `search_index`/`meta`). Ни одна из четырёх auth-таблиц не подходит под
  «дочернюю» (внешние ключи `owners`→ничего, `sessions.owner_id`/
  `api_tokens.owner_id`/`invites.created_by`→`owners` — все `ON DELETE
  RESTRICT`, не `CASCADE`, иначе `TestSchemaGraphShapeIsSupported` потребовал
  бы, чтобы `owners` был таблицей сущности типа `models.Type`, чем она не
  является и не должна быть).

**Решение:** внешние ключи `sessions.owner_id`/`api_tokens.owner_id`/
`invites.created_by` — настоящие, `ON DELETE RESTRICT` (получают
автоиндекс бесплатно через `createFKIndexes`, как и остальная схема);
`invites.used_by` — `ON DELETE SET NULL` (необязательная ссылка, не
проверяется `TestSchemaGraphShapeIsSupported` вообще — фильтр там смотрит
только `CASCADE`/`RESTRICT`). Единственная правка существующего теста —
**одна строка** в `serviceTables` (`internal/store/sqlstore/fkgraph_test.go`):
добавить `"owners"`, `"sessions"`, `"api_tokens"`, `"invites"`. Это не смена
логики теста, а ровно тот механизм, для которого `serviceTables` заведена.
`TestSchemaGraphShapeIsSupported` не трогается: `RESTRICT`-рёбра на нецелевую
(не entity) таблицу-родителя явно пропускаются («ссылки на value-таблицы не
блокируют удаление сущностей» — `owners` для этой проверки неотличима от
value-таблицы, что и требуется).

## Задача 1. Схема: таблицы и индексация

`internal/storage/schema_auth.go` (новый файл, пакет `storage`):

```go
package storage

// Схема internal/auth: четыре таблицы, не входящие в generic-систему
// сущностей (models.Type/internal/store — см. docs/data-model/auth.md §3–4).
// owner_id/created_by/used_by — настоящие внешние ключи на owners(id), но
// RESTRICT/SET NULL, не CASCADE: schemaGraph.owner (fkgraph.go) считает
// «дочерней» только CASCADE-связь, поэтому auth-таблицы не становятся частью
// generic-графа Delete*/InUseError, а fkgraph-тесты просто перечисляют их в
// serviceTables (docs/plans/2026-09-22-auth-a2-storage.md, предпосылка).
// Владельца в этом проходе удалить нельзя (нет DeleteOwner) — RESTRICT сейчас
// не сработает ни разу, но выражает верное намерение схемы на будущее.
var authSchemaDDL = []string{
	`CREATE TABLE IF NOT EXISTS owners (
		id            TEXT PRIMARY KEY,
		login         TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		created_at    TEXT NOT NULL
	)`,

	`CREATE TABLE IF NOT EXISTS sessions (
		id                 TEXT PRIMARY KEY,
		owner_id           TEXT NOT NULL REFERENCES owners(id) ON DELETE RESTRICT,
		access_token_hash  TEXT NOT NULL UNIQUE,
		access_expires_at  TEXT NOT NULL,
		refresh_token_hash TEXT NOT NULL UNIQUE,
		refresh_expires_at TEXT NOT NULL,
		created_at         TEXT NOT NULL
	)`,

	`CREATE TABLE IF NOT EXISTS api_tokens (
		id           TEXT PRIMARY KEY,
		owner_id     TEXT NOT NULL REFERENCES owners(id) ON DELETE RESTRICT,
		label        TEXT NOT NULL DEFAULT '',
		token_hash   TEXT NOT NULL UNIQUE,
		created_at   TEXT NOT NULL,
		last_used_at TEXT,
		revoked_at   TEXT
	)`,

	`CREATE TABLE IF NOT EXISTS invites (
		id         TEXT PRIMARY KEY,
		created_by TEXT NOT NULL REFERENCES owners(id) ON DELETE RESTRICT,
		token_hash TEXT NOT NULL UNIQUE,
		expires_at TEXT NOT NULL,
		used_at    TEXT,
		used_by    TEXT REFERENCES owners(id) ON DELETE SET NULL,
		created_at TEXT NOT NULL
	)`,
}
```

`internal/storage/db.go` — подключить схему в `OpenDB` (после существующего
`ddl := append(...)`, перед циклом `for _, q := range ddl`):

```go
	ddl := append(
		[]string{`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`},
		schemaDDL...,
	)
	ddl = append(ddl, authSchemaDDL...)
```

(это единственное изменение в `db.go`; `createFKIndexes(d)` ниже по функции
не трогается — он уже работает по `PRAGMA foreign_key_list` со всех таблиц.)

`internal/store/sqlstore/fkgraph_test.go` — добавить четыре записи в
существующую карту `serviceTables` (см. «Предпосылка» выше):

```go
var serviceTables = map[string]bool{
	"meta": true, "dates": true, "text_refs": true, "anchors": true,
	"source_links": true, "search_index": true,
	"owners": true, "sessions": true, "api_tokens": true, "invites": true,
}
```

- [ ] Шаг 1.1. TDD: `internal/storage/schema_auth_test.go`:

```go
package storage

import (
	"path/filepath"
	"testing"
)

func TestAuthSchemaTablesExist(t *testing.T) {
	db, err := OpenDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	for _, table := range []string{"owners", "sessions", "api_tokens", "invites"} {
		var name string
		err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
		if err != nil {
			t.Errorf("таблица %s не создана: %v", table, err)
		}
	}
}

// TestAuthSchemaLoginUnique: owners.login — UNIQUE (auth.md §3, решение
// финального ревью этапа A).
func TestAuthSchemaLoginUnique(t *testing.T) {
	db, err := OpenDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	const insert = `INSERT INTO owners(id, login, password_hash, created_at) VALUES (?, ?, ?, ?)`

	if _, err := db.Exec(insert, "OW-1", "vladelec", "h1", "2026-01-01T00:00:00Z"); err != nil {
		t.Fatalf("первая вставка: %v", err)
	}

	if _, err := db.Exec(insert, "OW-2", "vladelec", "h2", "2026-01-01T00:00:00Z"); err == nil {
		t.Fatal("ожидалась ошибка уникальности login")
	}
}

// TestAuthSchemaOwnerRestrictsDelete: owner_id REFERENCES ... RESTRICT —
// нельзя удалить владельца, пока есть его сессия.
func TestAuthSchemaOwnerRestrictsDelete(t *testing.T) {
	db, err := OpenDB(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	if _, err := db.Exec(
		`INSERT INTO owners(id, login, password_hash, created_at) VALUES ('OW-1','x','h','2026-01-01T00:00:00Z')`,
	); err != nil {
		t.Fatalf("owner: %v", err)
	}

	if _, err := db.Exec(
		`INSERT INTO sessions(id, owner_id, access_token_hash, access_expires_at,
		 refresh_token_hash, refresh_expires_at, created_at)
		 VALUES ('SS-1','OW-1','ah','2026-01-01T00:00:00Z','rh','2026-02-01T00:00:00Z','2026-01-01T00:00:00Z')`,
	); err != nil {
		t.Fatalf("session: %v", err)
	}

	if _, err := db.Exec(`DELETE FROM owners WHERE id = 'OW-1'`); err == nil {
		t.Fatal("ожидалась ошибка RESTRICT при удалении владельца с сессией")
	}
}

// TestAuthSchemaIdempotentOpen: повторное открытие той же базы не падает
// (CREATE TABLE IF NOT EXISTS) и не плодит индексы.
func TestAuthSchemaIdempotentOpen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")

	db1, err := OpenDB(path)
	if err != nil {
		t.Fatalf("первое открытие: %v", err)
	}
	_ = db1.Close()

	db2, err := OpenDB(path)
	if err != nil {
		t.Fatalf("второе открытие: %v", err)
	}
	t.Cleanup(func() { _ = db2.Close() })
}
```

- [ ] Шаг 1.2. Реализация: создать `internal/storage/schema_auth.go` с кодом
  выше; изменить `internal/storage/db.go` (одна строка `ddl = append(ddl,
  authSchemaDDL...)`); изменить `internal/store/sqlstore/fkgraph_test.go`
  (`serviceTables`).
- [ ] Шаг 1.3. `go test ./internal/storage/... ./internal/store/sqlstore/...`
  зелёный — включая существующие `TestEntityTablesMatchSchema`,
  `TestSchemaGraphShapeIsSupported`, `TestPrivateColumnTablesAreTheEntitiesWithFlag`,
  `TestTypeRegistryIsConsistent` (все три без изменений в самих себе, кроме
  строки в `serviceTables`). `gofmt -w internal/storage/ internal/store/sqlstore/`.
- [ ] Шаг 1.4. Рубеж: `go build ./...`, `go vet ./...`, `go test ./...`.
  Коммит `feat(storage): схема auth-таблиц (owners/sessions/api_tokens/invites)`.

## Задача 2. `internal/auth/sqlstore.go` — каркас и Owner

`internal/auth/sqlstore.go` (общие помощники и конструктор):

```go
package auth

import (
	"database/sql"
	"time"

	"github.com/amarin/genodex/internal/storage"
)

// SQLStore — реализация Store поверх internal/storage: то же соединение
// SQLite, что и у internal/store/sqlstore, но свои таблицы (auth.md §3).
type SQLStore struct {
	db *storage.DB
}

// NewSQLStore оборачивает уже открытое соединение (совместное с
// internal/store/sqlstore — конструирует internal/app, этап C).
func NewSQLStore(db *storage.DB) *SQLStore {
	return &SQLStore{db: db}
}

var _ Store = (*SQLStore)(nil)

// timeToSQL форматирует обязательное время как RFC3339 UTC (как
// storage/backup.go — единственный прецедент хранения time.Time текстом
// в проекте).
func timeToSQL(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// timeFromSQL разбирает обязательное время.
func timeFromSQL(s string) (time.Time, error) {
	return time.Parse(time.RFC3339, s)
}

// nullTime отдаёт SQL NULL для отсутствующего времени.
func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}

	return timeToSQL(*t)
}

// timePtrFromNull читает необязательное время: NULL → nil.
func timePtrFromNull(ns sql.NullString) (*time.Time, error) {
	if !ns.Valid {
		return nil, nil
	}

	t, err := timeFromSQL(ns.String)
	if err != nil {
		return nil, err
	}

	return &t, nil
}

// nullIDPtr отдаёт SQL NULL для отсутствующей ссылки-указателя.
func nullIDPtr(p *ID) any {
	if p == nil {
		return nil
	}

	return string(*p)
}

// idPtrFromNull читает необязательную ссылку: NULL → nil.
func idPtrFromNull(ns sql.NullString) *ID {
	if !ns.Valid {
		return nil
	}

	id := ID(ns.String)

	return &id
}

// checkRowsAffected — ErrNotFound, если операция не затронула ни одной
// строки (целевая строка не существует). Используется UPDATE/DELETE
// «точечных» методов (по id); массовые (DeleteSessionsByOwner) — нет, 0
// совпадений там законный результат.
func checkRowsAffected(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}

	if n == 0 {
		return ErrNotFound
	}

	return nil
}
```

`internal/auth/sqlstore_owner.go`:

```go
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
```

- [ ] Шаг 2.1. TDD: `internal/auth/sqlstore_test.go` (общий помощник открытия
  + тесты Owner):

```go
package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/amarin/genodex/internal/storage"
)

// newSQLStore открывает адаптер на временном каталоге.
func newSQLStore(t *testing.T) *SQLStore {
	t.Helper()

	st, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	return NewSQLStore(st.DB())
}

func testOwner(id ID) Owner {
	return Owner{ID: id, Login: "vladelec", PasswordHash: "hash", CreatedAt: time.Now().UTC().Truncate(time.Second)}
}

func TestSQLStoreCreateAndGetOwner(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()
	id, _ := newID(KindOwner)
	want := testOwner(id)

	if err := s.CreateOwner(ctx, want); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	byID, err := s.GetOwner(ctx, id)
	if err != nil {
		t.Fatalf("GetOwner: %v", err)
	}
	if *byID != want {
		t.Fatalf("GetOwner = %+v, want %+v", *byID, want)
	}

	byLogin, err := s.GetOwnerByLogin(ctx, "vladelec")
	if err != nil {
		t.Fatalf("GetOwnerByLogin: %v", err)
	}
	if *byLogin != want {
		t.Fatalf("GetOwnerByLogin = %+v, want %+v", *byLogin, want)
	}
}

func TestSQLStoreGetOwnerNotFound(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()

	if _, err := s.GetOwner(ctx, "OW-nonexistent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}

	if _, err := s.GetOwnerByLogin(ctx, "nikto"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}
}

func TestSQLStoreUpdateOwnerPassword(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()
	id, _ := newID(KindOwner)

	if err := s.CreateOwner(ctx, testOwner(id)); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	if err := s.UpdateOwnerPassword(ctx, id, "new-hash"); err != nil {
		t.Fatalf("UpdateOwnerPassword: %v", err)
	}

	got, err := s.GetOwner(ctx, id)
	if err != nil || got.PasswordHash != "new-hash" {
		t.Fatalf("got = %+v, err = %v", got, err)
	}
}

func TestSQLStoreUpdateOwnerPasswordNotFound(t *testing.T) {
	s := newSQLStore(t)

	if err := s.UpdateOwnerPassword(context.Background(), "OW-nonexistent", "h"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}
}

func TestSQLStoreCountOwners(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()

	n, err := s.CountOwners(ctx)
	if err != nil || n != 0 {
		t.Fatalf("n = %d, err = %v; ожидалось 0", n, err)
	}

	id, _ := newID(KindOwner)
	if err := s.CreateOwner(ctx, testOwner(id)); err != nil {
		t.Fatalf("CreateOwner: %v", err)
	}

	n, err = s.CountOwners(ctx)
	if err != nil || n != 1 {
		t.Fatalf("n = %d, err = %v; ожидалось 1", n, err)
	}
}

func TestSQLStoreCreateOwnerDuplicateLoginFails(t *testing.T) {
	s := newSQLStore(t)
	ctx := context.Background()
	id1, _ := newID(KindOwner)
	id2, _ := newID(KindOwner)

	if err := s.CreateOwner(ctx, testOwner(id1)); err != nil {
		t.Fatalf("первый CreateOwner: %v", err)
	}

	if err := s.CreateOwner(ctx, testOwner(id2)); err == nil {
		t.Fatal("ожидалась ошибка (login UNIQUE)")
	}
}
```

- [ ] Шаг 2.2. `go test ./internal/auth/...` зелёный; `gofmt -w internal/auth/`.
- [ ] Шаг 2.3. Рубеж; коммит `feat(auth): SQLStore — Owner (CRUD на SQLite)`.

**Осторожно во всех задачах ниже:** `timeToSQL`/`timeFromSQL` хранят время как
RFC3339 — точность до секунды, доли секунды теряются при записи. Тест,
сравнивающий структуру целиком (`got != want` или `reflect.DeepEqual`) после
`Create*`+`Get*`, обязан строить исходное время через
`time.Now().UTC().Truncate(time.Second)` (как `testOwner` в задаче 2), иначе
сравнение случайно падает на доле секунды. Сравнения через `.Before`/`.After`
(разница в минутах/днях, как в `Task 6`) эту точность не используют — там
достаточно `time.Now()` без усечения.

## Задача 3. `sqlstore_session.go`

```go
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
		id, ownerID                             string
		accessHash, refreshHash                 string
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
```

- [ ] Шаг 3.1. TDD: `internal/auth/sqlstore_session_test.go` — по образцу
  Owner-тестов: `testSession(id, ownerID)` строит валидную сессию (`newID` для
  обоих id, `AccessExpiresAt`/`RefreshExpiresAt` через `time.Now().Add(...)`,
  усечённые до секунды — RFC3339 не хранит доли секунды). Тесты:
  - `TestSQLStoreCreateAndGetSessionByAccessHash` / `…ByRefreshHash` —
    создать владельца (FK `owner_id` — RESTRICT, строка должна существовать),
    создать сессию, прочитать оба способа, сравнить поля.
  - `TestSQLStoreGetSessionNotFound` — оба метода на случайном хеше →
    `ErrNotFound`.
  - `TestSQLStoreReplaceSessionRotatesAndOldHashGone` — создать сессию,
    `ReplaceSession` с новыми хешами (тот же `ID`/`OwnerID`, новый
    `AccessTokenHash`/`RefreshTokenHash`/сроки), проверить: чтение по старому
    `access_token_hash`/`refresh_token_hash` → `ErrNotFound`; по новому →
    находит; `ID` не изменился.
  - `TestSQLStoreReplaceSessionNotFound` — `ReplaceSession` на
    несуществующий `id` → `ErrNotFound`.
  - `TestSQLStoreDeleteSession` — создать, удалить, `GetSessionByAccessHash`
    → `ErrNotFound`.
  - `TestSQLStoreDeleteSessionNotFound` — `DeleteSession` на несуществующий
    `id` → `ErrNotFound`.
  - `TestSQLStoreDeleteSessionsByOwner` — создать владельцу две сессии (разные
    `id`, разные хеши), одну сессию другому владельцу; `DeleteSessionsByOwner`
    первого владельца; обе его сессии не резолвятся, сессия второго
    владельца — резолвится.
  - `TestSQLStoreDeleteSessionsByOwnerNoSessionsIsNotError` — вызов на
    владельца без сессий возвращает `nil`.
  - `TestSQLStoreSessionOwnerRestrict` (интеграционная проверка схемы, не
    только Store-методов) — создать владельца и его сессию через
    `CreateOwner`/`CreateSession`, попытаться удалить владельца SQL-запросом
    напрямую (`s.db.ExecContext(ctx, "DELETE FROM owners WHERE id = ?", ...)`)
    → ошибка (см. `TestAuthSchemaOwnerRestrictsDelete` из задачи 1, здесь —
    через реальные Store-методы, а не голый SQL).
- [ ] Шаг 3.2. `go test ./internal/auth/...` зелёный; `gofmt -w internal/auth/`.
- [ ] Шаг 3.3. Рубеж; коммит `feat(auth): SQLStore — Session (ротация, отзыв)`.

## Задача 4. `sqlstore_api_token.go`

```go
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
func (s *SQLStore) RevokeAPIToken(ctx context.Context, id ID) error {
	res, err := s.db.ExecContext(ctx, `UPDATE api_tokens SET revoked_at = ? WHERE id = ?`,
		timeToSQL(time.Now()), string(id))
	if err != nil {
		return err
	}

	return checkRowsAffected(res)
}

// TouchAPIToken обновляет last_used_at; нет такого токена — ErrNotFound.
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
```

- [ ] Шаг 4.1. TDD: `internal/auth/sqlstore_api_token_test.go`:
  - `TestSQLStoreCreateAndGetAPIToken` — создать владельца, создать токен
    (`LastUsedAt`/`RevokedAt` — `nil`), прочитать по `id` и по `token_hash`,
    сравнить; оба указателя — `nil` после чтения.
  - `TestSQLStoreGetAPITokenNotFound` — оба метода на отсутствующем → `ErrNotFound`.
  - `TestSQLStoreListAPITokensOrderAndOwnerFilter` — двум разным владельцам
    создать по два токена; `ListAPITokens` одного владельца отдаёт ровно его
    два токена, в порядке создания (проверить по `Label`, каждому токену —
    свой различимый `Label`).
  - `TestSQLStoreListAPITokensEmptyIsEmptySlice` — владелец без токенов →
    `[]APIToken{}` (не `nil`).
  - `TestSQLStoreTouchAPITokenSetsLastUsedAt` — создать токен, `TouchAPIToken`,
    прочитать — `LastUsedAt` не `nil` и не раньше `CreatedAt`.
  - `TestSQLStoreTouchAPITokenNotFound` — на отсутствующий `id` → `ErrNotFound`.
  - `TestSQLStoreRevokeAPITokenSetsRevokedAt` — создать, `RevokeAPIToken`,
    прочитать — `RevokedAt` не `nil`.
  - `TestSQLStoreRevokeAPITokenNotFound` — на отсутствующий `id` → `ErrNotFound`.
- [ ] Шаг 4.2. `go test ./internal/auth/...` зелёный; `gofmt -w internal/auth/`.
- [ ] Шаг 4.3. Рубеж; коммит `feat(auth): SQLStore — APIToken (CRUD, отзыв, touch)`.

## Задача 5. `sqlstore_invite.go`

```go
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
```

- [ ] Шаг 5.1. TDD: `internal/auth/sqlstore_invite_test.go`:
  - `TestSQLStoreCreateAndGetInvite` — создать владельца-автора, создать
    приглашение (`UsedAt`/`UsedBy` — `nil`), прочитать по хешу, сравнить;
    `UsedAt`/`UsedBy` — `nil`.
  - `TestSQLStoreGetInviteNotFound` — на отсутствующий хеш → `ErrNotFound`.
  - `TestSQLStoreMarkInviteUsedSetsFields` — создать автора и второго
    владельца (получателя), создать приглашение, `MarkInviteUsed(id,
    получатель)`, прочитать — `UsedAt` не `nil`, `UsedBy` равен id получателя.
  - `TestSQLStoreMarkInviteUsedNotFound` — на отсутствующий `id` → `ErrNotFound`.
  - `TestSQLStoreInviteCreatedByRestrict` — как
    `TestSQLStoreSessionOwnerRestrict`: создать автора и его приглашение,
    попытаться удалить автора напрямую SQL — ошибка RESTRICT.
- [ ] Шаг 5.2. `go test ./internal/auth/...` зелёный; `gofmt -w internal/auth/`.
- [ ] Шаг 5.3. Рубеж; коммит `feat(auth): SQLStore — Invite (CRUD, отметка использования)`.

## Задача 6. Сквозной тест: `Service` на реальном `SQLStore`

`internal/auth/service_sqlstore_test.go` — тот же сценарий, что
`fakeStore`-тесты `Service` (этап A, `service_test.go`), но на реальном
хранилище: подтверждает, что `SQLStore` действительно удовлетворяет всем
допущениям `Service` (контракт `ErrNotFound`, персистентность между
вызовами — `fakeStore` живёт только в памяти теста, `SQLStore` — реальная
запись на диск).

```go
package auth

import (
	"context"
	"errors"
	"testing"
)

func newTestServiceSQL(t *testing.T) *Service {
	t.Helper()

	return New(newSQLStore(t))
}

// TestServiceOnSQLStoreFullLifecycle: bootstrap → invite → второй владелец →
// логин → refresh → API-токен → смена пароля гасит сессии → анонимный доступ.
func TestServiceOnSQLStoreFullLifecycle(t *testing.T) {
	svc := newTestServiceSQL(t)
	ctx := context.Background()

	boot, err := svc.Bootstrap(ctx)
	if err != nil || !boot {
		t.Fatalf("Bootstrap = %v, %v; ожидалось true, nil", boot, err)
	}

	first, err := svc.Register(ctx, "first", "password123", nil)
	if err != nil {
		t.Fatalf("Register (bootstrap): %v", err)
	}

	boot, err = svc.Bootstrap(ctx)
	if err != nil || boot {
		t.Fatalf("Bootstrap после первого владельца = %v, %v; ожидалось false, nil", boot, err)
	}

	raw, err := svc.CreateInvite(ctx, first.OwnerID)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	second, err := svc.Register(ctx, "second", "password123", &raw)
	if err != nil {
		t.Fatalf("Register (invite): %v", err)
	}

	loggedIn, err := svc.Login(ctx, "second", "password123")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}

	access, owner, err := svc.ResolveAccess(ctx, loggedIn.AccessToken)
	if err != nil || owner == nil || *owner != second.OwnerID {
		t.Fatalf("ResolveAccess = %v, %v, %v", access, owner, err)
	}

	rotated, err := svc.Refresh(ctx, loggedIn.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if _, err := svc.Refresh(ctx, loggedIn.RefreshToken); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("повтор старого refresh: err = %v, ожидался ErrSessionExpired", err)
	}

	rawToken, tokenID, err := svc.CreateAPIToken(ctx, second.OwnerID, "MCP")
	if err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}

	if resolved, err := svc.ResolveAPIToken(ctx, rawToken); err != nil || resolved != second.OwnerID {
		t.Fatalf("ResolveAPIToken: %v, %v", resolved, err)
	}

	extraLogin, err := svc.Login(ctx, "second", "password123")
	if err != nil {
		t.Fatalf("второй Login: %v", err)
	}

	if err := svc.ChangePassword(ctx, second.OwnerID, "password123", "new-password456"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	for name, tok := range map[string]string{"rotated": rotated.AccessToken, "extraLogin": extraLogin.AccessToken} {
		if access, owner, err := svc.ResolveAccess(ctx, tok); err != nil || access != models.AccessPublic || owner != nil {
			t.Errorf("%s после ChangePassword: access=%v owner=%v err=%v", name, access, owner, err)
		}
	}

	if err := svc.RevokeAPIToken(ctx, second.OwnerID, tokenID); err != nil {
		t.Fatalf("RevokeAPIToken: %v", err)
	}

	if _, err := svc.ResolveAPIToken(ctx, rawToken); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("отозванный токен: err = %v, ожидался ErrInvalidCredentials", err)
	}
}

// TestServiceOnSQLStorePersistsAcrossInstances: новый *Service на том же
// файле БД видит данные, записанные предыдущим — подтверждает, что данные
// реально на диске, а не только в памяти процесса.
func TestServiceOnSQLStorePersistsAcrossInstances(t *testing.T) {
	dir := t.TempDir()

	open := func(t *testing.T) *Service {
		t.Helper()

		st, err := storage.Open(dir)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		t.Cleanup(func() { _ = st.Close() })

		return New(NewSQLStore(st.DB()))
	}

	svc1 := open(t)

	res, err := svc1.Register(context.Background(), "user", "password123", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	svc2 := open(t)

	access, owner, err := svc2.ResolveAccess(context.Background(), res.AccessToken)
	if err != nil || access != models.AccessFull || owner == nil || *owner != res.OwnerID {
		t.Fatalf("после переоткрытия: access=%v owner=%v err=%v", access, owner, err)
	}
}
```

(добавить импорт `"github.com/amarin/genodex/internal/models"` и
`"github.com/amarin/genodex/internal/storage"` в этот файл — оба нужны для
`models.AccessPublic`/`models.AccessFull` и повторного открытия в
`TestServiceOnSQLStorePersistsAcrossInstances`.)

- [ ] Шаг 6.1. `go test ./internal/auth/...` зелёный (включая новые сквозные
  тесты); `gofmt -w internal/auth/`.
- [ ] Шаг 6.2. Рубеж: `go build ./...`, `go vet ./...`, `go test ./...`.
  Коммит `test(auth): Service на реальном SQLStore — сквозной сценарий и персистентность`.

## Рубеж этапа

- `gofmt -l .` пусто, `go build ./...`, `go vet ./...`, `go test ./...`.
- `internal/auth` по-прежнему не подключён ни к `internal/app`, ни к
  `internal/httpapi`/`internal/mcp` — это этапы B/B2/C.
- Обновить `docs/plans/2026-09-22-auth-roadmap.md`: отметить этап A2
  выполненным.
- Обновить `docs/data-model/auth.md` §3, если по ходу реализации что-то
  уточнилось (например, конкретное имя автоиндекса) — держать спеку и код
  синхронными, как раньше.

## Коммиты

1. `feat(storage): схема auth-таблиц (owners/sessions/api_tokens/invites)`
2. `feat(auth): SQLStore — Owner (CRUD на SQLite)`
3. `feat(auth): SQLStore — Session (ротация, отзыв)`
4. `feat(auth): SQLStore — APIToken (CRUD, отзыв, touch)`
5. `feat(auth): SQLStore — Invite (CRUD, отметка использования)`
6. `test(auth): Service на реальном SQLStore — сквозной сценарий и персистентность`
