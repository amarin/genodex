# Этап A. `internal/auth` — домен: план этапа

Спека — [auth.md](../data-model/auth.md); дорожная карта —
[auth-roadmap.md](2026-09-22-auth-roadmap.md), раздел A. Формат — как этапы
S1–S16 программы «ядро чтения и записи». Исполняется в `main` подрядными
маленькими коммитами.

## Goal

Пакет `internal/auth` с доменными типами (`Owner`, `Session`, `APIToken`,
`Invite`), собственным ID-форматом, хешированием пароля/токенов, узким
портом `Store` и сервисом-фасадом (`Service`), полностью протестированным на
фейковом `Store`. Реальное хранилище (SQLite) — отдельный этап A2. Пакет не
зависит от `internal/models`/`internal/store`/`internal/idgen`, кроме одной
точки (`Service.ResolveAccess` возвращает `models.Access` — единственный
импорт `models` во всём пакете).

## Задача 1. `ID` и `Kind`

`internal/auth/id.go` — свой формат `ПРЕФИКС-ULID`, независимый от
`models.ID` (см. `auth.md` §2: три существующих теста жёстко приравнивают
`models.AllTypes()` к общему реестру сущностей — auth не должен туда
попадать).

```go
// Package auth реализует владельцев, сессии, API-токены и invite-ссылки —
// независимый от internal/models домен (см. docs/data-model/auth.md).
package auth

import (
	"errors"
	"fmt"
	"strings"
)

// Kind — вид auth-сущности, кодируется префиксом ID.
type Kind string

const (
	KindOwner    Kind = "owner"
	KindSession  Kind = "session"
	KindAPIToken Kind = "api_token"
	KindInvite   Kind = "invite"
)

// kindPrefix — префиксы ID; не пересекаются с models.idPrefixTable (auth и
// models — независимые ID-пространства, совпадение префиксов не опасно, но
// не проверяется и не должно проверяться намеренно).
var kindPrefix = map[Kind]string{
	KindOwner:    "OW",
	KindSession:  "SS",
	KindAPIToken: "AT",
	KindInvite:   "IV",
}

// ID — идентификатор auth-сущности, формат ПРЕФИКС-ULID (как models.ID:
// 26-символьный ULID Crockford base32, заглавные, без I L O U).
type ID string

const (
	idBodyLen  = 26
	idAlphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
)

// ErrInvalidID — идентификатор нарушает формат ПРЕФИКС-ULID.
var ErrInvalidID = errors.New("неверный формат идентификатора")

func validateIDBody(body string) error {
	if len(body) != idBodyLen {
		return fmt.Errorf("ULID должен содержать %d символов, получено %d", idBodyLen, len(body))
	}
	for i := 0; i < len(body); i++ {
		if !strings.ContainsRune(idAlphabet, rune(body[i])) {
			return fmt.Errorf("недопустимый символ %q в позиции %d", body[i], i)
		}
	}
	if body[0] > '7' {
		return fmt.Errorf("первый символ ULID %q больше «7»: переполнение времени", body[0])
	}

	return nil
}

// parseID разбирает идентификатор вида ПРЕФИКС-ULID и возвращает вид по префиксу.
func parseID(id ID) (Kind, error) {
	prefix, body, ok := strings.Cut(string(id), "-")
	if !ok {
		return "", fmt.Errorf("%w: %q — нет разделителя «-»", ErrInvalidID, string(id))
	}

	var kind Kind

	found := false

	for k, p := range kindPrefix {
		if p == prefix {
			kind, found = k, true

			break
		}
	}

	if !found {
		return "", fmt.Errorf("%w: %q — неизвестный префикс %q", ErrInvalidID, string(id), prefix)
	}

	if err := validateIDBody(body); err != nil {
		return "", fmt.Errorf("%w: %q — %s", ErrInvalidID, string(id), err)
	}

	return kind, nil
}

// Validate проверяет формат идентификатора и что префикс соответствует
// ожидаемому виду.
func (id ID) Validate(want Kind) error {
	got, err := parseID(id)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("%w: %q — префикс относится к виду %s, ожидается %s",
			ErrInvalidID, string(id), got, want)
	}

	return nil
}

// buildID собирает идентификатор из вида и готового ULID.
func buildID(kind Kind, body string) (ID, error) {
	prefix, ok := kindPrefix[kind]
	if !ok {
		return "", fmt.Errorf("%w: у вида %q нет префикса", ErrInvalidID, string(kind))
	}
	if err := validateIDBody(body); err != nil {
		return "", fmt.Errorf("%w: %s", ErrInvalidID, err)
	}

	return ID(prefix + "-" + body), nil
}
```

- [ ] Шаг 1.1. TDD: `internal/auth/id_test.go`:

```go
package auth

import (
	"errors"
	"testing"
)

func TestBuildIDAndValidate(t *testing.T) {
	id, err := buildID(KindOwner, "01ARZ3NDEKTSV4RRFFQ69G5FA9")
	if err != nil {
		t.Fatalf("buildID: %v", err)
	}
	if id != "OW-01ARZ3NDEKTSV4RRFFQ69G5FA9" {
		t.Fatalf("id = %q", id)
	}
	if err := id.Validate(KindOwner); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if err := id.Validate(KindSession); err == nil {
		t.Fatal("ожидалась ошибка при несовпадении вида")
	}
}

func TestBuildIDUnknownKind(t *testing.T) {
	if _, err := buildID(Kind("nonsense"), "01ARZ3NDEKTSV4RRFFQ69G5FA9"); !errors.Is(err, ErrInvalidID) {
		t.Fatalf("err = %v, ожидался ErrInvalidID", err)
	}
}

func TestValidateRejectsBadFormat(t *testing.T) {
	for _, id := range []ID{
		"", "OW", "OW-", "OW-tooshort", "XX-01ARZ3NDEKTSV4RRFFQ69G5FA9",
		"OW-01ARZ3NDEKTSV4RRFFQ69G5FAI", // I — вне алфавита Crockford
		"OW-81ARZ3NDEKTSV4RRFFQ69G5FA9", // первый символ > 7
	} {
		if err := id.Validate(KindOwner); !errors.Is(err, ErrInvalidID) {
			t.Errorf("%q: err = %v, ожидался ErrInvalidID", id, err)
		}
	}
}

func TestAllKindsHavePrefix(t *testing.T) {
	for _, k := range []Kind{KindOwner, KindSession, KindAPIToken, KindInvite} {
		if kindPrefix[k] == "" {
			t.Errorf("у вида %q нет префикса", k)
		}
	}
}

func TestKindPrefixesAreUnique(t *testing.T) {
	seen := map[string]Kind{}
	for k, p := range kindPrefix {
		if other, dup := seen[p]; dup {
			t.Errorf("префикс %q у видов %q и %q", p, other, k)
		}
		seen[p] = k
	}
}
```

- [ ] Шаг 1.2. `go test ./internal/auth/...` зелёный (реализация уже написана
  выше — тест пишется первым по духу TDD, но код `id.go` даётся сразу
  целиком, т. к. это инфраструктурный примитив без развилок). `gofmt -w
  internal/auth/`.
- [ ] Шаг 1.3. Рубеж: `go build ./...`, `go vet ./...`, `go test ./...`.
  Коммит `feat(auth): формат ID и виды auth-сущностей`.

## Задача 2. Генератор ID

`internal/auth/idgen.go` — генерация нового `ID`. Без монотонной гарантии
внутри миллисекунды (в отличие от `internal/idgen`): списки auth-сущностей в
этом проходе сортируются по времени сохранения в SQL, не по ID (как и весь
остальной проект — `core-read-write.md` §4, «порядок — порядок сохранения»),
поэтому 80 бит `crypto/rand`-энтропии достаточно для уникальности без
состояния генератора. Кодирование бит в base32 — тот же алгоритм, что в
`internal/idgen/idgen.go` (независимая копия: `auth` не зависит от `idgen`,
решение зафиксировано в `auth.md` §2).

```go
package auth

import (
	"crypto/rand"
	"fmt"
	"time"
)

// newID генерирует новый ID вида kind: 48 бит времени (мс) + 80 бит
// crypto/rand, Crockford base32. Не монотонен внутри миллисекунды —
// auth-сущности не нуждаются в строгом порядке по ID (см. пакетный
// комментарий выше).
func newID(kind Kind) (ID, error) {
	if _, ok := kindPrefix[kind]; !ok {
		return "", fmt.Errorf("%w: у вида %q нет префикса", ErrInvalidID, string(kind))
	}

	var entropy [10]byte
	if _, err := rand.Read(entropy[:]); err != nil {
		return "", fmt.Errorf("auth: не удалось получить случайные байты: %w", err)
	}

	ms := uint64(time.Now().UnixMilli())

	return buildID(kind, encodeULID(ms, entropy))
}

// encodeULID кодирует 48 бит времени и 80 бит энтропии в 26 символов
// Crockford base32: 128 бит дополняются двумя нулями слева до 130 (26 по 5
// бит). Копия internal/idgen.encode — та же арифметика, отдельная функция
// по решению «auth не зависит от idgen» (auth.md §2).
func encodeULID(ms uint64, entropy [10]byte) string {
	var raw [16]byte
	for i := 0; i < 6; i++ {
		raw[i] = byte(ms >> (8 * (5 - i)))
	}

	copy(raw[6:], entropy[:])

	var out [26]byte

	for i := range out {
		v := 0
		for b := 0; b < 5; b++ {
			v = v<<1 | ulidBit(&raw, i*5+b-2)
		}

		out[i] = idAlphabet[v]
	}

	return string(out[:])
}

func ulidBit(raw *[16]byte, n int) int {
	if n < 0 {
		return 0
	}

	return int(raw[n/8]>>(7-n%8)) & 1
}
```

- [ ] Шаг 2.1. TDD: `internal/auth/idgen_test.go`:

```go
package auth

import "testing"

// TestEncodeULIDMatchesKnownVectors: те же контрольные значения, что
// internal/idgen/idgen_test.go:TestEncode (тот же алгоритм).
func TestEncodeULIDMatchesKnownVectors(t *testing.T) {
	tests := []struct {
		name    string
		ms      uint64
		entropy [10]byte
		want    string
	}{
		{"нули", 0, [10]byte{}, "00000000000000000000000000"},
		{"только энтропия", 0, [10]byte{255, 255, 255, 255, 255, 255, 255, 255, 255, 255}, "0000000000ZZZZZZZZZZZZZZZZ"},
		{"максимум времени", 1<<48 - 1, [10]byte{}, "7ZZZZZZZZZ0000000000000000"},
		{"пример спецификации", 1469922850259, [10]byte{}, "01ARZ3NDEK0000000000000000"},
	}
	for _, tt := range tests {
		if got := encodeULID(tt.ms, tt.entropy); got != tt.want {
			t.Errorf("%s: encodeULID = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestNewIDValidForAllKinds(t *testing.T) {
	for _, k := range []Kind{KindOwner, KindSession, KindAPIToken, KindInvite} {
		id, err := newID(k)
		if err != nil {
			t.Fatalf("%s: %v", k, err)
		}
		if err := id.Validate(k); err != nil {
			t.Errorf("%s: %v", k, err)
		}
	}
}

func TestNewIDUnknownKindErrors(t *testing.T) {
	if _, err := newID(Kind("nonsense")); err == nil {
		t.Fatal("ожидалась ошибка для неизвестного вида")
	}
}

// TestNewIDUniqueOnSeries: 10 000 значений одного вида без повторов.
func TestNewIDUniqueOnSeries(t *testing.T) {
	seen := make(map[ID]struct{}, 10000)
	for i := 0; i < 10000; i++ {
		id, err := newID(KindSession)
		if err != nil {
			t.Fatalf("newID: %v", err)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("повтор id %q", id)
		}
		seen[id] = struct{}{}
	}
}
```

- [ ] Шаг 2.2. `go test ./internal/auth/...` зелёный; `gofmt -w internal/auth/`.
- [ ] Шаг 2.3. Рубеж; коммит `feat(auth): генератор ID (без монотонности)`.

## Задача 3. Хеширование пароля и токенов

`internal/auth/hash.go`. Добавить зависимость: `go get
golang.org/x/crypto/bcrypt` (первая внешняя зависимость auth — bcrypt
специально для паролей, см. `auth.md` §1 решение 5).

```go
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// apiTokenPrefix — узнаваемый префикс сырого значения API-токена (как
// GitHub/Stripe), только для читаемости — на проверку не влияет.
const apiTokenPrefix = "gnx_"

// hashPassword хеширует пароль bcrypt (стоимость по умолчанию — медленно
// намеренно).
func hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("auth: не удалось хешировать пароль: %w", err)
	}

	return string(hash), nil
}

// verifyPassword сравнивает пароль с bcrypt-хешем; ошибка сравнения (в т.ч.
// неверный пароль) — false, без деталей причины.
func verifyPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// hashToken — SHA-256 сырого токена в hex: для хранения и поиска в БД.
// Токен уже высокоэнтропийный (32 байта crypto/rand) — быстрый хеш
// достаточен, в отличие от пароля.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))

	return hex.EncodeToString(sum[:])
}

// newRawToken возвращает случайную строку: 32 байта crypto/rand в base64url
// без паддинга.
func newRawToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("auth: не удалось получить случайные байты: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(b[:]), nil
}

// newAPIToken — как newRawToken, с префиксом apiTokenPrefix.
func newAPIToken() (string, error) {
	raw, err := newRawToken()
	if err != nil {
		return "", err
	}

	return apiTokenPrefix + raw, nil
}
```

- [ ] Шаг 3.1. `go get golang.org/x/crypto/bcrypt && go mod tidy`.
- [ ] Шаг 3.2. TDD: `internal/auth/hash_test.go`:

```go
package auth

import "testing"

func TestHashPasswordRoundTrip(t *testing.T) {
	hash, err := hashPassword("верный-пароль")
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	if hash == "верный-пароль" {
		t.Fatal("хеш совпал с исходным паролем")
	}
	if !verifyPassword(hash, "верный-пароль") {
		t.Fatal("verifyPassword: верный пароль не прошёл")
	}
	if verifyPassword(hash, "неверный-пароль") {
		t.Fatal("verifyPassword: неверный пароль прошёл")
	}
}

func TestHashPasswordSaltsEachCall(t *testing.T) {
	h1, _ := hashPassword("пароль")
	h2, _ := hashPassword("пароль")
	if h1 == h2 {
		t.Fatal("два хеша одного пароля совпали — соль не применяется")
	}
}

func TestHashTokenDeterministic(t *testing.T) {
	if hashToken("abc") != hashToken("abc") {
		t.Fatal("hashToken недетерминирован")
	}
	if hashToken("abc") == hashToken("abd") {
		t.Fatal("разные токены дали одинаковый хеш")
	}
}

func TestNewRawTokenUniqueAndFormat(t *testing.T) {
	seen := map[string]struct{}{}
	for i := 0; i < 1000; i++ {
		raw, err := newRawToken()
		if err != nil {
			t.Fatalf("newRawToken: %v", err)
		}
		if len(raw) == 0 {
			t.Fatal("пустой токен")
		}
		if _, dup := seen[raw]; dup {
			t.Fatalf("повтор токена %q", raw)
		}
		seen[raw] = struct{}{}
	}
}

func TestNewAPITokenHasPrefix(t *testing.T) {
	raw, err := newAPIToken()
	if err != nil {
		t.Fatalf("newAPIToken: %v", err)
	}
	if len(raw) <= len(apiTokenPrefix) || raw[:len(apiTokenPrefix)] != apiTokenPrefix {
		t.Fatalf("токен %q без префикса %q", raw, apiTokenPrefix)
	}
}
```

- [ ] Шаг 3.3. `go test ./internal/auth/...` зелёный; `gofmt -w internal/auth/`.
- [ ] Шаг 3.4. Рубеж (`go build ./...` — тянет новую зависимость, убедиться,
  что `go.sum` обновился); коммит
  `feat(auth): хеширование пароля (bcrypt) и токенов (SHA-256)`.

## Задача 4. Доменные типы и ошибки

`internal/auth/errors.go`:

```go
package auth

import "errors"

// ErrNotFound — auth-сущность не найдена в Store.
var ErrNotFound = errors.New("не найдено")

// ErrInvalidCredentials — неверный логин/пароль, либо невалидный/отозванный
// токен. Один сентинел на все случаи «кто-то не тот» — не различаем
// «нет такого логина» и «неверный пароль» (защита от перебора логинов).
var ErrInvalidCredentials = errors.New("неверный логин или пароль")

// ErrSessionExpired — refresh-токен просрочен или не найден.
var ErrSessionExpired = errors.New("сессия истекла")

// ErrInviteRequired — регистрация после bootstrap требует invite-ссылку.
var ErrInviteRequired = errors.New("нужна invite-ссылка")

// ErrInviteInvalid — invite-ссылка не найдена, использована или истекла.
var ErrInviteInvalid = errors.New("invite-ссылка недействительна")

// ErrLoginTaken — логин уже занят другим владельцем.
var ErrLoginTaken = errors.New("логин уже занят")
```

`internal/auth/validation.go`:

```go
package auth

import "fmt"

// ValidationError — нарушение инварианта auth-сущности.
type ValidationError struct {
	Field  string
	Reason string
	Err    error
}

func (e *ValidationError) Unwrap() error { return e.Err }

func (e *ValidationError) Error() string {
	if e.Field == "" {
		return e.Reason
	}

	return e.Field + ": " + e.Reason
}

func fieldErr(field, format string, args ...any) *ValidationError {
	return &ValidationError{Field: field, Reason: fmt.Sprintf(format, args...)}
}

func idFieldErr(field string, id ID, want Kind) *ValidationError {
	err := id.Validate(want)
	if err == nil {
		return nil
	}

	return &ValidationError{Field: field, Reason: err.Error(), Err: err}
}
```

`internal/auth/owner.go`, `session.go`, `api_token.go`, `invite.go`:

```go
// owner.go
package auth

import (
	"strings"
	"time"
)

// Owner — владелец: логин/пароль, полный доступ (models.AccessFull).
type Owner struct {
	ID           ID
	Login        string
	PasswordHash string
	CreatedAt    time.Time
}

// Validate проверяет непустые поля и формат ID.
func (o Owner) Validate() error {
	if e := idFieldErr("id", o.ID, KindOwner); e != nil {
		return e
	}
	if strings.TrimSpace(o.Login) == "" {
		return fieldErr("login", "не может быть пустым")
	}
	if o.PasswordHash == "" {
		return fieldErr("password_hash", "не может быть пустым")
	}

	return nil
}
```

```go
// session.go
package auth

import "time"

// Session — пара access/refresh токенов одного входа владельца.
type Session struct {
	ID                ID
	OwnerID           ID
	AccessTokenHash   string
	AccessExpiresAt   time.Time
	RefreshTokenHash  string
	RefreshExpiresAt  time.Time
	CreatedAt         time.Time
}

// Validate проверяет формат id, непустые хеши и refresh дольше access.
func (s Session) Validate() error {
	if e := idFieldErr("id", s.ID, KindSession); e != nil {
		return e
	}
	if e := idFieldErr("owner_id", s.OwnerID, KindOwner); e != nil {
		return e
	}
	if s.AccessTokenHash == "" {
		return fieldErr("access_token_hash", "не может быть пустым")
	}
	if s.RefreshTokenHash == "" {
		return fieldErr("refresh_token_hash", "не может быть пустым")
	}
	if !s.RefreshExpiresAt.After(s.AccessExpiresAt) {
		return fieldErr("refresh_expires_at", "должен быть позже access_expires_at")
	}

	return nil
}
```

```go
// api_token.go
package auth

import "time"

// APIToken — долгоживущий токен для MCP/скриптов, отдельно от Session.
type APIToken struct {
	ID         ID
	OwnerID    ID
	Label      string
	TokenHash  string
	CreatedAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}

// Validate проверяет формат id и непустой хеш.
func (t APIToken) Validate() error {
	if e := idFieldErr("id", t.ID, KindAPIToken); e != nil {
		return e
	}
	if e := idFieldErr("owner_id", t.OwnerID, KindOwner); e != nil {
		return e
	}
	if t.TokenHash == "" {
		return fieldErr("token_hash", "не может быть пустым")
	}

	return nil
}
```

```go
// invite.go
package auth

import "time"

// Invite — одноразовая ссылка регистрации совладельца.
type Invite struct {
	ID        ID
	CreatedBy ID
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	UsedBy    *ID
	CreatedAt time.Time
}

// Validate проверяет формат id, непустой хеш и заданный срок действия.
func (i Invite) Validate() error {
	if e := idFieldErr("id", i.ID, KindInvite); e != nil {
		return e
	}
	if e := idFieldErr("created_by", i.CreatedBy, KindOwner); e != nil {
		return e
	}
	if i.TokenHash == "" {
		return fieldErr("token_hash", "не может быть пустым")
	}
	if i.ExpiresAt.IsZero() {
		return fieldErr("expires_at", "не может быть пустым")
	}

	return nil
}
```

- [ ] Шаг 4.1. TDD (пишется до реализации Validate — здесь для компактности
  показан вместе): `internal/auth/owner_test.go` и аналогично для остальных
  трёх типов, например:

```go
package auth

import (
	"errors"
	"testing"
	"time"
)

func validOwner() Owner {
	id, _ := newID(KindOwner)

	return Owner{ID: id, Login: "vladelec", PasswordHash: "hash", CreatedAt: time.Now()}
}

func TestOwnerValidate(t *testing.T) {
	if err := validOwner().Validate(); err != nil {
		t.Fatalf("валидный Owner не прошёл: %v", err)
	}

	bad := validOwner()
	bad.Login = "  "
	var ve *ValidationError
	if err := bad.Validate(); !errors.As(err, &ve) || ve.Field != "login" {
		t.Fatalf("пустой login: err = %v", err)
	}

	bad = validOwner()
	bad.PasswordHash = ""
	if err := bad.Validate(); !errors.As(err, &ve) || ve.Field != "password_hash" {
		t.Fatalf("пустой password_hash: err = %v", err)
	}

	bad = validOwner()
	bad.ID = "not-an-id"
	if err := bad.Validate(); !errors.As(err, &ve) || ve.Field != "id" {
		t.Fatalf("неверный id: err = %v", err)
	}
}
```

Аналогичные `TestSessionValidate` (пустые хеши, `id`/`owner_id` неверного
вида, `refresh_expires_at` не позже `access_expires_at`),
`TestAPITokenValidate`, `TestInviteValidate` (пустой `token_hash`, нулевой
`expires_at`, неверный `created_by`) — по тому же образцу, три поля-инварианта
на тип минимум.

- [ ] Шаг 4.2. `go test ./internal/auth/...` зелёный; `gofmt -w internal/auth/`.
- [ ] Шаг 4.3. Рубеж; коммит
  `feat(auth): доменные типы Owner/Session/APIToken/Invite и Validate()`.

## Задача 5. Узкий порт `Store`

`internal/auth/deps.go`:

```go
package auth

import "context"

// Store — узкий порт auth: свои таблицы, не пересекается с internal/store
// (см. auth.md §3 — не входит в generic 21-сущностную систему).
//
//go:generate mockgen -source $GOFILE -destination deps_test.go -package auth
type Store interface {
	CreateOwner(ctx context.Context, o Owner) error
	GetOwnerByLogin(ctx context.Context, login string) (*Owner, error)
	GetOwner(ctx context.Context, id ID) (*Owner, error)
	UpdateOwnerPassword(ctx context.Context, id ID, hash string) error
	CountOwners(ctx context.Context) (int, error)

	CreateSession(ctx context.Context, s Session) error
	GetSessionByAccessHash(ctx context.Context, hash string) (*Session, error)
	GetSessionByRefreshHash(ctx context.Context, hash string) (*Session, error)
	ReplaceSession(ctx context.Context, id ID, s Session) error
	DeleteSession(ctx context.Context, id ID) error

	CreateAPIToken(ctx context.Context, t APIToken) error
	GetAPIToken(ctx context.Context, id ID) (*APIToken, error)
	GetAPITokenByHash(ctx context.Context, hash string) (*APIToken, error)
	ListAPITokens(ctx context.Context, ownerID ID) ([]APIToken, error)
	RevokeAPIToken(ctx context.Context, id ID) error
	TouchAPIToken(ctx context.Context, id ID) error

	CreateInvite(ctx context.Context, i Invite) error
	GetInviteByHash(ctx context.Context, hash string) (*Invite, error)
	MarkInviteUsed(ctx context.Context, id ID, by ID) error
}
```

(Не в спеке §3 изначально: добавлен `GetAPIToken` — нужен `RevokeAPIToken`
сервиса, чтобы проверить владельца токена перед отзывом, не перебирая
`ListAPITokens`. Мелкое уточнение уровня плана, не меняет дизайн.)

- [ ] Шаг 5.1. Перегенерация мока (mockgen установлен в `$(go env
  GOPATH)/bin`, как для сценариев): `PATH="$PATH:$(go env GOPATH)/bin"
  mockgen -source internal/auth/deps.go -destination
  internal/auth/deps_test.go -package auth`. Файл коммитится (как у
  `list_divisions` и др.) — используется ли он в тестах Задачи 6, решается
  там; проект исторически пишет тесты на ручных фейках, мок — по стандарту
  `DEVELOPER-PREFERENCES`.
- [ ] Шаг 5.2. `go build ./...` (порт компилируется, реализации ещё нет —
  используется только в Задаче 6 через фейк).
- [ ] Шаг 5.3. Рубеж; коммит `feat(auth): узкий порт Store`.

## Задача 6. `Service`

`internal/auth/service.go`:

```go
package auth

import (
	"context"
	"errors"
	"time"

	"github.com/amarin/genodex/internal/models"
)

const (
	accessTokenTTL  = 15 * time.Minute
	refreshTokenTTL = 30 * 24 * time.Hour
	inviteTTL       = 7 * 24 * time.Hour
)

// AuthResult — результат успешной аутентификации: сырые токены (вызывающая
// сторона — httpapi — ставит их в cookie) и срок действия.
type AuthResult struct {
	OwnerID           ID
	AccessToken       string
	AccessExpiresAt   time.Time
	RefreshToken      string
	RefreshExpiresAt  time.Time
}

// Service — фасад над Store: вся бизнес-логика auth в одном месте (домен
// маленький и плотно связанный — как сценарии usecases/*, но один пакет).
type Service struct {
	store Store
	now   func() time.Time
}

// New создаёт сервис на системных часах.
func New(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

// Bootstrap сообщает, есть ли хоть один владелец (false — обычная работа,
// true — веб должен предложить мастер регистрации без invite).
func (s *Service) Bootstrap(ctx context.Context) (bool, error) {
	count, err := s.store.CountOwners(ctx)
	if err != nil {
		return false, err
	}

	return count == 0, nil
}

// Register создаёт владельца: до первого владельца invite не нужен
// (bootstrap), после — обязателен, валиден, не использован и не истёк.
func (s *Service) Register(ctx context.Context, login, password string, invite *string) (AuthResult, error) {
	count, err := s.store.CountOwners(ctx)
	if err != nil {
		return AuthResult{}, err
	}

	var usedInvite *Invite

	if count > 0 {
		usedInvite, err = s.checkInvite(ctx, invite)
		if err != nil {
			return AuthResult{}, err
		}
	}

	if _, err := s.store.GetOwnerByLogin(ctx, login); err == nil {
		return AuthResult{}, ErrLoginTaken
	} else if !errors.Is(err, ErrNotFound) {
		return AuthResult{}, err
	}

	hash, err := hashPassword(password)
	if err != nil {
		return AuthResult{}, err
	}

	id, err := newID(KindOwner)
	if err != nil {
		return AuthResult{}, err
	}

	owner := Owner{ID: id, Login: login, PasswordHash: hash, CreatedAt: s.now()}
	if err := owner.Validate(); err != nil {
		return AuthResult{}, err
	}

	if err := s.store.CreateOwner(ctx, owner); err != nil {
		return AuthResult{}, err
	}

	if usedInvite != nil {
		if err := s.store.MarkInviteUsed(ctx, usedInvite.ID, owner.ID); err != nil {
			return AuthResult{}, err
		}
	}

	return s.newSession(ctx, owner.ID)
}

// checkInvite проверяет сырое значение invite-ссылки.
func (s *Service) checkInvite(ctx context.Context, raw *string) (*Invite, error) {
	if raw == nil || *raw == "" {
		return nil, ErrInviteRequired
	}

	inv, err := s.store.GetInviteByHash(ctx, hashToken(*raw))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrInviteInvalid
		}

		return nil, err
	}

	if inv.UsedAt != nil || !inv.ExpiresAt.After(s.now()) {
		return nil, ErrInviteInvalid
	}

	return inv, nil
}

// Login проверяет логин/пароль и заводит новую сессию. Неизвестный логин и
// неверный пароль дают один и тот же ErrInvalidCredentials (защита от
// перебора логинов).
func (s *Service) Login(ctx context.Context, login, password string) (AuthResult, error) {
	owner, err := s.store.GetOwnerByLogin(ctx, login)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return AuthResult{}, ErrInvalidCredentials
		}

		return AuthResult{}, err
	}

	if !verifyPassword(owner.PasswordHash, password) {
		return AuthResult{}, ErrInvalidCredentials
	}

	return s.newSession(ctx, owner.ID)
}

// Refresh ротирует access и refresh по действующему refresh-токену; старый
// refresh перестаёт резолвиться сам собой (ReplaceSession перезаписывает
// хеши той же строки) — повторное использование украденного токена даёт
// ErrSessionExpired.
func (s *Service) Refresh(ctx context.Context, rawRefresh string) (AuthResult, error) {
	session, err := s.store.GetSessionByRefreshHash(ctx, hashToken(rawRefresh))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return AuthResult{}, ErrSessionExpired
		}

		return AuthResult{}, err
	}

	if !session.RefreshExpiresAt.After(s.now()) {
		return AuthResult{}, ErrSessionExpired
	}

	return s.rotateSession(ctx, *session)
}

// Logout удаляет сессию по access-токену; нет такой сессии — не ошибка
// (уже разлогинен).
func (s *Service) Logout(ctx context.Context, rawAccess string) error {
	session, err := s.store.GetSessionByAccessHash(ctx, hashToken(rawAccess))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}

		return err
	}

	return s.store.DeleteSession(ctx, session.ID)
}

// ResolveAccess — режим доступа по access-cookie: пусто, не найдена или
// просрочена — models.AccessPublic без ошибки (это не ошибка запроса, а
// нормальный анонимный путь); валидная — models.AccessFull и OwnerID.
func (s *Service) ResolveAccess(ctx context.Context, rawAccess string) (models.Access, *ID, error) {
	if rawAccess == "" {
		return models.AccessPublic, nil, nil
	}

	session, err := s.store.GetSessionByAccessHash(ctx, hashToken(rawAccess))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return models.AccessPublic, nil, nil
		}

		return models.AccessPublic, nil, err
	}

	if !session.AccessExpiresAt.After(s.now()) {
		return models.AccessPublic, nil, nil
	}

	owner := session.OwnerID

	return models.AccessFull, &owner, nil
}

// ChangePassword проверяет текущий пароль и сохраняет новый хеш.
func (s *Service) ChangePassword(ctx context.Context, ownerID ID, current, newPassword string) error {
	owner, err := s.store.GetOwner(ctx, ownerID)
	if err != nil {
		return err
	}

	if !verifyPassword(owner.PasswordHash, current) {
		return ErrInvalidCredentials
	}

	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}

	return s.store.UpdateOwnerPassword(ctx, ownerID, hash)
}

// CreateInvite создаёт одноразовую ссылку (срок — inviteTTL) и возвращает
// сырое значение (только здесь — дальше хранится лишь хеш).
func (s *Service) CreateInvite(ctx context.Context, ownerID ID) (string, error) {
	id, err := newID(KindInvite)
	if err != nil {
		return "", err
	}

	raw, err := newRawToken()
	if err != nil {
		return "", err
	}

	invite := Invite{
		ID: id, CreatedBy: ownerID, TokenHash: hashToken(raw),
		ExpiresAt: s.now().Add(inviteTTL), CreatedAt: s.now(),
	}
	if err := invite.Validate(); err != nil {
		return "", err
	}

	if err := s.store.CreateInvite(ctx, invite); err != nil {
		return "", err
	}

	return raw, nil
}

// CreateAPIToken создаёт долгоживущий токен для MCP/скриптов; сырое
// значение — только здесь.
func (s *Service) CreateAPIToken(ctx context.Context, ownerID ID, label string) (string, ID, error) {
	id, err := newID(KindAPIToken)
	if err != nil {
		return "", "", err
	}

	raw, err := newAPIToken()
	if err != nil {
		return "", "", err
	}

	token := APIToken{ID: id, OwnerID: ownerID, Label: label, TokenHash: hashToken(raw), CreatedAt: s.now()}
	if err := token.Validate(); err != nil {
		return "", "", err
	}

	if err := s.store.CreateAPIToken(ctx, token); err != nil {
		return "", "", err
	}

	return raw, id, nil
}

// ListAPITokens отдаёт метаданные токенов владельца (без сырых значений —
// они не хранятся).
func (s *Service) ListAPITokens(ctx context.Context, ownerID ID) ([]APIToken, error) {
	return s.store.ListAPITokens(ctx, ownerID)
}

// RevokeAPIToken отзывает токен, если он принадлежит ownerID; чужой токен —
// ErrNotFound (не подтверждаем существование чужого токена).
func (s *Service) RevokeAPIToken(ctx context.Context, ownerID, tokenID ID) error {
	token, err := s.store.GetAPIToken(ctx, tokenID)
	if err != nil {
		return err
	}

	if token.OwnerID != ownerID {
		return ErrNotFound
	}

	return s.store.RevokeAPIToken(ctx, tokenID)
}

// ResolveAPIToken — для MCP-middleware: сырой bearer-токен → OwnerID.
// Отозванный или неизвестный — ErrInvalidCredentials.
func (s *Service) ResolveAPIToken(ctx context.Context, raw string) (ID, error) {
	token, err := s.store.GetAPITokenByHash(ctx, hashToken(raw))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return "", ErrInvalidCredentials
		}

		return "", err
	}

	if token.RevokedAt != nil {
		return "", ErrInvalidCredentials
	}

	if err := s.store.TouchAPIToken(ctx, token.ID); err != nil {
		return "", err
	}

	return token.OwnerID, nil
}

// newSession заводит новую сессию для ownerID.
func (s *Service) newSession(ctx context.Context, ownerID ID) (AuthResult, error) {
	id, err := newID(KindSession)
	if err != nil {
		return AuthResult{}, err
	}

	return s.saveSession(ctx, Session{ID: id, OwnerID: ownerID, CreatedAt: s.now()}, s.store.CreateSession)
}

// rotateSession перевыпускает токены существующей сессии (Refresh).
func (s *Service) rotateSession(ctx context.Context, existing Session) (AuthResult, error) {
	replace := func(ctx context.Context, sess Session) error {
		return s.store.ReplaceSession(ctx, existing.ID, sess)
	}

	return s.saveSession(ctx, existing, replace)
}

// saveSession генерирует новую пару токенов для session (ID/OwnerID/CreatedAt
// уже заполнены вызывающим) и сохраняет через save.
func (s *Service) saveSession(
	ctx context.Context, session Session, save func(context.Context, Session) error,
) (AuthResult, error) {
	rawAccess, err := newRawToken()
	if err != nil {
		return AuthResult{}, err
	}

	rawRefresh, err := newRawToken()
	if err != nil {
		return AuthResult{}, err
	}

	now := s.now()
	session.AccessTokenHash = hashToken(rawAccess)
	session.AccessExpiresAt = now.Add(accessTokenTTL)
	session.RefreshTokenHash = hashToken(rawRefresh)
	session.RefreshExpiresAt = now.Add(refreshTokenTTL)

	if err := session.Validate(); err != nil {
		return AuthResult{}, err
	}

	if err := save(ctx, session); err != nil {
		return AuthResult{}, err
	}

	return AuthResult{
		OwnerID: session.OwnerID,
		AccessToken: rawAccess, AccessExpiresAt: session.AccessExpiresAt,
		RefreshToken: rawRefresh, RefreshExpiresAt: session.RefreshExpiresAt,
	}, nil
}
```

- [ ] Шаг 6.1. TDD: `internal/auth/service_test.go` — фейковый `Store` в
  памяти (без мока — по образцу `fakeRepo` в `usecases/list_divisions`):

```go
package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeStore struct {
	owners   map[ID]Owner
	sessions map[ID]Session
	tokens   map[ID]APIToken
	invites  map[ID]Invite
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		owners: map[ID]Owner{}, sessions: map[ID]Session{},
		tokens: map[ID]APIToken{}, invites: map[ID]Invite{},
	}
}

func (f *fakeStore) CreateOwner(_ context.Context, o Owner) error {
	f.owners[o.ID] = o

	return nil
}

func (f *fakeStore) GetOwnerByLogin(_ context.Context, login string) (*Owner, error) {
	for _, o := range f.owners {
		if o.Login == login {
			cp := o

			return &cp, nil
		}
	}

	return nil, ErrNotFound
}

func (f *fakeStore) GetOwner(_ context.Context, id ID) (*Owner, error) {
	o, ok := f.owners[id]
	if !ok {
		return nil, ErrNotFound
	}

	return &o, nil
}

func (f *fakeStore) UpdateOwnerPassword(_ context.Context, id ID, hash string) error {
	o, ok := f.owners[id]
	if !ok {
		return ErrNotFound
	}

	o.PasswordHash = hash
	f.owners[id] = o

	return nil
}

func (f *fakeStore) CountOwners(_ context.Context) (int, error) {
	return len(f.owners), nil
}

func (f *fakeStore) CreateSession(_ context.Context, s Session) error {
	f.sessions[s.ID] = s

	return nil
}

func (f *fakeStore) GetSessionByAccessHash(_ context.Context, hash string) (*Session, error) {
	for _, s := range f.sessions {
		if s.AccessTokenHash == hash {
			cp := s

			return &cp, nil
		}
	}

	return nil, ErrNotFound
}

func (f *fakeStore) GetSessionByRefreshHash(_ context.Context, hash string) (*Session, error) {
	for _, s := range f.sessions {
		if s.RefreshTokenHash == hash {
			cp := s

			return &cp, nil
		}
	}

	return nil, ErrNotFound
}

func (f *fakeStore) ReplaceSession(_ context.Context, id ID, s Session) error {
	if _, ok := f.sessions[id]; !ok {
		return ErrNotFound
	}

	f.sessions[id] = s

	return nil
}

func (f *fakeStore) DeleteSession(_ context.Context, id ID) error {
	delete(f.sessions, id)

	return nil
}

func (f *fakeStore) CreateAPIToken(_ context.Context, t APIToken) error {
	f.tokens[t.ID] = t

	return nil
}

func (f *fakeStore) GetAPIToken(_ context.Context, id ID) (*APIToken, error) {
	t, ok := f.tokens[id]
	if !ok {
		return nil, ErrNotFound
	}

	return &t, nil
}

func (f *fakeStore) GetAPITokenByHash(_ context.Context, hash string) (*APIToken, error) {
	for _, t := range f.tokens {
		if t.TokenHash == hash {
			cp := t

			return &cp, nil
		}
	}

	return nil, ErrNotFound
}

func (f *fakeStore) ListAPITokens(_ context.Context, ownerID ID) ([]APIToken, error) {
	var out []APIToken

	for _, t := range f.tokens {
		if t.OwnerID == ownerID {
			out = append(out, t)
		}
	}

	return out, nil
}

func (f *fakeStore) RevokeAPIToken(_ context.Context, id ID) error {
	t, ok := f.tokens[id]
	if !ok {
		return ErrNotFound
	}

	now := time.Now()
	t.RevokedAt = &now
	f.tokens[id] = t

	return nil
}

func (f *fakeStore) TouchAPIToken(_ context.Context, id ID) error {
	t, ok := f.tokens[id]
	if !ok {
		return ErrNotFound
	}

	now := time.Now()
	t.LastUsedAt = &now
	f.tokens[id] = t

	return nil
}

func (f *fakeStore) CreateInvite(_ context.Context, i Invite) error {
	f.invites[i.ID] = i

	return nil
}

func (f *fakeStore) GetInviteByHash(_ context.Context, hash string) (*Invite, error) {
	for _, i := range f.invites {
		if i.TokenHash == hash {
			cp := i

			return &cp, nil
		}
	}

	return nil, ErrNotFound
}

func (f *fakeStore) MarkInviteUsed(_ context.Context, id, by ID) error {
	i, ok := f.invites[id]
	if !ok {
		return ErrNotFound
	}

	now := time.Now()
	i.UsedAt = &now
	i.UsedBy = &by
	f.invites[id] = i

	return nil
}

var _ Store = (*fakeStore)(nil)

func newTestService(store Store) *Service {
	return &Service{store: store, now: time.Now}
}

func TestServiceRegisterBootstrapNoInviteNeeded(t *testing.T) {
	svc := newTestService(newFakeStore())

	res, err := svc.Register(context.Background(), "first", "password123", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if res.AccessToken == "" || res.RefreshToken == "" {
		t.Fatal("пустые токены после регистрации")
	}
}

func TestServiceRegisterAfterBootstrapRequiresInvite(t *testing.T) {
	store := newFakeStore()
	svc := newTestService(store)
	ctx := context.Background()

	if _, err := svc.Register(ctx, "first", "password123", nil); err != nil {
		t.Fatalf("bootstrap Register: %v", err)
	}

	if _, err := svc.Register(ctx, "second", "password123", nil); !errors.Is(err, ErrInviteRequired) {
		t.Fatalf("err = %v, ожидался ErrInviteRequired", err)
	}
}

func TestServiceRegisterWithValidInvite(t *testing.T) {
	store := newFakeStore()
	svc := newTestService(store)
	ctx := context.Background()

	first, err := svc.Register(ctx, "first", "password123", nil)
	if err != nil {
		t.Fatalf("bootstrap Register: %v", err)
	}

	raw, err := svc.CreateInvite(ctx, first.OwnerID)
	if err != nil {
		t.Fatalf("CreateInvite: %v", err)
	}

	if _, err := svc.Register(ctx, "second", "password123", &raw); err != nil {
		t.Fatalf("Register по invite: %v", err)
	}

	// повторное использование той же ссылки — ошибка
	if _, err := svc.Register(ctx, "third", "password123", &raw); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("повторный invite: err = %v, ожидался ErrInviteInvalid", err)
	}
}

func TestServiceRegisterExpiredInviteFails(t *testing.T) {
	store := newFakeStore()
	svc := newTestService(store)
	ctx := context.Background()

	first, _ := svc.Register(ctx, "first", "password123", nil)
	raw, _ := svc.CreateInvite(ctx, first.OwnerID)

	svc.now = func() time.Time { return time.Now().Add(inviteTTL + time.Hour) }

	if _, err := svc.Register(ctx, "second", "password123", &raw); !errors.Is(err, ErrInviteInvalid) {
		t.Fatalf("err = %v, ожидался ErrInviteInvalid", err)
	}
}

func TestServiceRegisterDuplicateLoginFails(t *testing.T) {
	svc := newTestService(newFakeStore())
	ctx := context.Background()

	if _, err := svc.Register(ctx, "dup", "password123", nil); err != nil {
		t.Fatalf("Register: %v", err)
	}

	store := svc.store.(*fakeStore)
	raw, _ := svc.CreateInvite(ctx, func() ID {
		for id := range store.owners {
			return id
		}

		return ""
	}())

	if _, err := svc.Register(ctx, "dup", "password123", &raw); !errors.Is(err, ErrLoginTaken) {
		t.Fatalf("err = %v, ожидался ErrLoginTaken", err)
	}
}

func TestServiceLoginSuccessAndFailure(t *testing.T) {
	svc := newTestService(newFakeStore())
	ctx := context.Background()

	if _, err := svc.Register(ctx, "user", "correct-password", nil); err != nil {
		t.Fatalf("Register: %v", err)
	}

	if _, err := svc.Login(ctx, "user", "correct-password"); err != nil {
		t.Fatalf("Login: %v", err)
	}

	if _, err := svc.Login(ctx, "user", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("неверный пароль: err = %v, ожидался ErrInvalidCredentials", err)
	}

	if _, err := svc.Login(ctx, "nobody", "whatever"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("неизвестный логин: err = %v, ожидался ErrInvalidCredentials", err)
	}
}

func TestServiceRefreshRotatesAndRejectsReuse(t *testing.T) {
	svc := newTestService(newFakeStore())
	ctx := context.Background()

	first, err := svc.Register(ctx, "user", "password123", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	rotated, err := svc.Refresh(ctx, first.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if rotated.AccessToken == first.AccessToken || rotated.RefreshToken == first.RefreshToken {
		t.Fatal("Refresh не сменил токены")
	}

	if _, err := svc.Refresh(ctx, first.RefreshToken); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("повтор старого refresh: err = %v, ожидался ErrSessionExpired", err)
	}

	if _, err := svc.Refresh(ctx, rotated.RefreshToken); err != nil {
		t.Fatalf("новый refresh должен работать: %v", err)
	}
}

func TestServiceRefreshExpiredFails(t *testing.T) {
	svc := newTestService(newFakeStore())
	ctx := context.Background()

	res, err := svc.Register(ctx, "user", "password123", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	svc.now = func() time.Time { return time.Now().Add(refreshTokenTTL + time.Hour) }

	if _, err := svc.Refresh(ctx, res.RefreshToken); !errors.Is(err, ErrSessionExpired) {
		t.Fatalf("err = %v, ожидался ErrSessionExpired", err)
	}
}

func TestServiceLogoutThenResolveAccessIsPublic(t *testing.T) {
	svc := newTestService(newFakeStore())
	ctx := context.Background()

	res, err := svc.Register(ctx, "user", "password123", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	access, owner, err := svc.ResolveAccess(ctx, res.AccessToken)
	if err != nil || access != models.AccessFull || owner == nil {
		t.Fatalf("до logout: access=%v owner=%v err=%v", access, owner, err)
	}

	if err := svc.Logout(ctx, res.AccessToken); err != nil {
		t.Fatalf("Logout: %v", err)
	}

	access, owner, err = svc.ResolveAccess(ctx, res.AccessToken)
	if err != nil || access != models.AccessPublic || owner != nil {
		t.Fatalf("после logout: access=%v owner=%v err=%v", access, owner, err)
	}
}

func TestServiceResolveAccessEmptyAndExpired(t *testing.T) {
	svc := newTestService(newFakeStore())
	ctx := context.Background()

	if access, owner, err := svc.ResolveAccess(ctx, ""); err != nil || access != models.AccessPublic || owner != nil {
		t.Fatalf("пустой токен: access=%v owner=%v err=%v", access, owner, err)
	}

	res, err := svc.Register(ctx, "user", "password123", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	svc.now = func() time.Time { return time.Now().Add(accessTokenTTL + time.Hour) }

	if access, owner, err := svc.ResolveAccess(ctx, res.AccessToken); err != nil || access != models.AccessPublic || owner != nil {
		t.Fatalf("просроченный access: access=%v owner=%v err=%v", access, owner, err)
	}
}

func TestServiceChangePassword(t *testing.T) {
	svc := newTestService(newFakeStore())
	ctx := context.Background()

	res, err := svc.Register(ctx, "user", "old-password", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if err := svc.ChangePassword(ctx, res.OwnerID, "wrong", "new-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("неверный текущий пароль: err = %v", err)
	}

	if err := svc.ChangePassword(ctx, res.OwnerID, "old-password", "new-password"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	if _, err := svc.Login(ctx, "user", "new-password"); err != nil {
		t.Fatalf("логин новым паролем: %v", err)
	}
	if _, err := svc.Login(ctx, "user", "old-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatal("старый пароль всё ещё работает")
	}
}

func TestServiceAPITokenLifecycle(t *testing.T) {
	svc := newTestService(newFakeStore())
	ctx := context.Background()

	owner, err := svc.Register(ctx, "user", "password123", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	raw, id, err := svc.CreateAPIToken(ctx, owner.OwnerID, "MCP")
	if err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}

	resolved, err := svc.ResolveAPIToken(ctx, raw)
	if err != nil || resolved != owner.OwnerID {
		t.Fatalf("ResolveAPIToken: owner=%v err=%v", resolved, err)
	}

	list, err := svc.ListAPITokens(ctx, owner.OwnerID)
	if err != nil || len(list) != 1 || list[0].ID != id {
		t.Fatalf("ListAPITokens: %+v, err=%v", list, err)
	}

	if err := svc.RevokeAPIToken(ctx, owner.OwnerID, id); err != nil {
		t.Fatalf("RevokeAPIToken: %v", err)
	}

	if _, err := svc.ResolveAPIToken(ctx, raw); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("отозванный токен: err = %v, ожидался ErrInvalidCredentials", err)
	}
}

func TestServiceRevokeAPITokenWrongOwnerFails(t *testing.T) {
	svc := newTestService(newFakeStore())
	ctx := context.Background()

	owner1, _ := svc.Register(ctx, "user1", "password123", nil)
	raw, _ := svc.CreateInvite(ctx, owner1.OwnerID)
	owner2, err := svc.Register(ctx, "user2", "password123", &raw)
	if err != nil {
		t.Fatalf("Register user2: %v", err)
	}

	_, id, err := svc.CreateAPIToken(ctx, owner1.OwnerID, "MCP")
	if err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}

	if err := svc.RevokeAPIToken(ctx, owner2.OwnerID, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("чужой токен: err = %v, ожидался ErrNotFound", err)
	}
}

func TestServiceBootstrap(t *testing.T) {
	store := newFakeStore()
	svc := newTestService(store)
	ctx := context.Background()

	boot, err := svc.Bootstrap(ctx)
	if err != nil || !boot {
		t.Fatalf("до первого владельца: boot=%v err=%v", boot, err)
	}

	if _, err := svc.Register(ctx, "user", "password123", nil); err != nil {
		t.Fatalf("Register: %v", err)
	}

	boot, err = svc.Bootstrap(ctx)
	if err != nil || boot {
		t.Fatalf("после первого владельца: boot=%v err=%v", boot, err)
	}
}
```

(`TestServiceLogoutThenResolveAccessIsPublic`/`TestServiceResolveAccessEmptyAndExpired`
используют `models.AccessFull`/`models.AccessPublic` — добавить импорт
`"github.com/amarin/genodex/internal/models"` в `service_test.go`.)

- [ ] Шаг 6.2. `go test ./internal/auth/...` зелёный; `gofmt -w internal/auth/`.
- [ ] Шаг 6.3. Рубеж: `go build ./...`, `go vet ./...`, `go test ./...`.
  Коммит `feat(auth): Service — регистрация, вход, refresh, токены, invite`.

## Рубеж этапа

- `gofmt -l .` пусто, `go build ./...`, `go vet ./...`, `go test ./...`.
- `go list -m all | grep golang.org/x/crypto` — зависимость закреплена в
  `go.sum`.
- `internal/auth` не импортируется нигде за пределами себя (этап A
  самодостаточен, подключение — в этапах B/B2/C).
- Обновить `docs/plans/2026-09-22-auth-roadmap.md`: отметить этап A
  выполненным (после мерджа всех задач).

## Коммиты

1. `feat(auth): формат ID и виды auth-сущностей`
2. `feat(auth): генератор ID (без монотонности)`
3. `feat(auth): хеширование пароля (bcrypt) и токенов (SHA-256)`
4. `feat(auth): доменные типы Owner/Session/APIToken/Invite и Validate()`
5. `feat(auth): узкий порт Store`
6. `feat(auth): Service — регистрация, вход, refresh, токены, invite`
