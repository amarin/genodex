# Этап B. HTTP-контракт auth: план этапа

Спека — [auth.md](../data-model/auth.md) §4; дорожная карта —
[auth-roadmap.md](2026-09-22-auth-roadmap.md), раздел B. Формат — как этапы A/A2.
Исполняется в `main` подрядными маленькими коммитами.

## Goal

`GET/POST /api/auth/*` (11 маршрутов), cookie-сессии, middleware
`resolveAccess`/`requireCSRFHeader` — полностью самостоятельный, протестированный
узел `internal/httpapi`, ещё **не подключённый** к основному `NewHandler`/`mux`
(это — этап C, вместе с `internal/app`). Экспортируется `httpapi.NewAuthHandler(auth
AuthService) http.Handler`, который этап C смонтирует в общий `mux`.

## Предпосылка из этапа A (уточнение роадмапа)

`auth.Service` (этап A) не имеет публичного метода «прочитать владельца по id» —
только `ResolveAccess` (отдаёт `*ID`, не логин) и приватный доступ через `Store`.
Для `GET /api/auth/session` (нужно показать логин текущего владельца) и
`POST /api/auth/refresh` (после ротации нужно вернуть логин в теле ответа, а
`AuthResult` его не несёт) нужен способ получить `Owner` по `ID`. `POST
/api/auth/register`/`login` в этом не нуждаются — логин уже известен из тела
запроса, не нужно перечитывать хранилище.

**Решение:** добавить один метод `Service.GetOwner(ctx, id) (*Owner, error)` —
тонкая обёртка над `s.store.GetOwner` (задача 1 ниже). Это не меняет контракт
`Store` (метод уже есть в порту с этапа A), только добавляет публичный доступ к
нему через `Service` — минимальное, точечное расширение.

## Архитектурные решения (для истории — auth.md обновляется на рубеже)

- **DTO auth, а не голые домен-типы.** `internal/models`/`internal/transport`
  разделены по правилу DEVELOPER-PREFERENCES («models без JSON-тегов, transport —
  DTO») — то же правило для `internal/auth`: `internal/transport/auth.go` заводит
  DTO с `snake_case`-тегами (`AuthSession`, `AuthStatus`, `APIToken`, `NewAPIToken`,
  `Invite`, запросы `RegisterRequest`/`LoginRequest`/`PasswordChangeRequest`/
  `CreateAPITokenRequest`). `json:"-"` на `Owner.PasswordHash`/`APIToken.TokenHash`
  (добавлены на рубеже этапа A2) остаются как defense-in-depth, но не единственная
  защита — DTO `APIToken` физически не содержит поля хеша.
- **`Register`/`Login` не вызывают `GetOwner`** — логин уже есть в теле запроса.
  Только `Refresh` и `Session` читают владельца заново (см. предпосылку выше).
- **`ChangePassword` инвалидирует и сессию самого вызывающего** (эффект
  `DeleteSessionsByOwner` с этапа A2 бьёт по всем сессиям владельца без
  исключений — так и задумано). Обработчик поэтому сам чистит cookies вызывающего
  после успешной смены пароля и отвечает `204` — веб (этап D) обязан отправить
  пользователя на `/login` заново. Никакого «перевыпуска сессии тут же» не делаем
  (это требовало бы нового метода `Service`, ради UX-удобства, не обязательного
  для контракта) — задокументировать в `auth.md` §4 на рубеже.
- **`invite` — query-параметр**, не поле тела (`POST
  /api/auth/register?invite=<raw>`), как явно написано в таблице §4.
- **`POST /api/auth/tokens` тело** — `{"label": "..."}`; ответ — `id`/`token`
  (сырое)/`label`, единственный раз.

## Задача 1. `Service.GetOwner` и DTO auth

`internal/auth/service.go` — добавить метод (после `Bootstrap`, по алфавиту с
остальными или в логичном месте рядом с `ResolveAccess`):

```go
// GetOwner читает владельца по id; нет такого — ErrNotFound. Нужен
// обработчикам HTTP/MCP, которым известен только OwnerID (из ResolveAccess
// или AuthResult), а не логин.
func (s *Service) GetOwner(ctx context.Context, id ID) (*Owner, error) {
	return s.store.GetOwner(ctx, id)
}
```

- [ ] Шаг 1.1. TDD: `internal/auth/service_test.go` — добавить тест (в файл,
  рядом с остальными `TestService*`):

```go
func TestServiceGetOwner(t *testing.T) {
	svc := newTestService(newFakeStore())
	ctx := context.Background()

	res, err := svc.Register(ctx, "user", "password123", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	owner, err := svc.GetOwner(ctx, res.OwnerID)
	if err != nil || owner.Login != "user" {
		t.Fatalf("GetOwner: %+v, %v", owner, err)
	}

	if _, err := svc.GetOwner(ctx, "OW-nonexistent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, ожидался ErrNotFound", err)
	}
}
```

- [ ] Шаг 1.2. `go test ./internal/auth/...` зелёный.

`internal/transport/auth.go` (новый файл):

```go
package transport

import (
	"time"

	"github.com/amarin/genodex/internal/auth"
)

// AuthSession — ответ /api/auth/session, /register, /login, /refresh: кто
// сейчас аутентифицирован.
type AuthSession struct {
	Login string `json:"login"`
}

// AuthSessionFromOwner строит ответ из владельца.
func AuthSessionFromOwner(o *auth.Owner) AuthSession {
	return AuthSession{Login: o.Login}
}

// AuthStatus — ответ /api/auth/status.
type AuthStatus struct {
	Bootstrap bool `json:"bootstrap"`
}

// RegisterRequest — тело POST /api/auth/register (invite — query-параметр,
// не сюда).
type RegisterRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// LoginRequest — тело POST /api/auth/login.
type LoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// PasswordChangeRequest — тело POST /api/auth/password.
type PasswordChangeRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// CreateAPITokenRequest — тело POST /api/auth/tokens.
type CreateAPITokenRequest struct {
	Label string `json:"label"`
}

// APIToken — элемент ответа GET /api/auth/tokens: метаданные без сырого
// значения (его не хранит и сам auth.APIToken).
type APIToken struct {
	ID         string     `json:"id"`
	Label      string     `json:"label"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
}

// APITokenFromModel конвертирует одну запись.
func APITokenFromModel(t auth.APIToken) APIToken {
	return APIToken{
		ID: string(t.ID), Label: t.Label, CreatedAt: t.CreatedAt,
		LastUsedAt: t.LastUsedAt, RevokedAt: t.RevokedAt,
	}
}

// APITokensFromModels конвертирует список; пустой список — не nil.
func APITokensFromModels(list []auth.APIToken) []APIToken {
	out := make([]APIToken, len(list))
	for i, t := range list {
		out[i] = APITokenFromModel(t)
	}

	return out
}

// NewAPIToken — ответ POST /api/auth/tokens: сырое значение — только здесь.
type NewAPIToken struct {
	ID    string `json:"id"`
	Token string `json:"token"`
	Label string `json:"label"`
}

// Invite — ответ POST /api/auth/invites: сырое значение — только здесь.
type Invite struct {
	Token string `json:"token"`
}
```

- [ ] Шаг 1.3. TDD: `internal/transport/auth_test.go`:

```go
package transport

import (
	"testing"
	"time"

	"github.com/amarin/genodex/internal/auth"
)

func TestAuthSessionFromOwner(t *testing.T) {
	got := AuthSessionFromOwner(&auth.Owner{Login: "vladelec"})
	if got.Login != "vladelec" {
		t.Fatalf("Login = %q", got.Login)
	}
}

func TestAPITokensFromModelsEmptyIsEmptySlice(t *testing.T) {
	got := APITokensFromModels(nil)
	if got == nil || len(got) != 0 {
		t.Fatalf("got = %v, ожидался непустой указатель на пустой срез", got)
	}
}

func TestAPITokenFromModelOmitsHash(t *testing.T) {
	now := time.Now()
	got := APITokenFromModel(auth.APIToken{
		ID: "AT-1", Label: "MCP", TokenHash: "секрет-не-должен-попасть-в-dto",
		CreatedAt: now, LastUsedAt: &now,
	})
	if got.ID != "AT-1" || got.Label != "MCP" || got.LastUsedAt == nil {
		t.Fatalf("got = %+v", got)
	}
	// TokenHash отсутствует как поле DTO — компилятор уже это гарантирует
	// (нет способа его прочитать из got), тест фиксирует поведение конвертера.
}
```

- [ ] Шаг 1.4. `go test ./internal/transport/... ./internal/auth/...` зелёный;
  `gofmt -w internal/auth/ internal/transport/`.
- [ ] Шаг 1.5. Рубеж: `go build ./...`, `go vet ./...`, `go test ./...`.
  Коммит `feat(auth,transport): Service.GetOwner и DTO для HTTP-контракта auth`.

## Задача 2. `internal/httpapi`: `AuthService`, middleware

`internal/httpapi/deps.go` — добавить интерфейс (не трогать существующий
`DivisionService`):

```go
package httpapi

import (
	"context"

	authpkg "github.com/amarin/genodex/internal/auth"
	"github.com/amarin/genodex/internal/models"
)

// AuthService — контракт auth.Service, отдаваемый в HTTP-обработчики.
type AuthService interface {
	Bootstrap(ctx context.Context) (bool, error)
	Register(ctx context.Context, login, password string, invite *string) (authpkg.AuthResult, error)
	Login(ctx context.Context, login, password string) (authpkg.AuthResult, error)
	Refresh(ctx context.Context, rawRefresh string) (authpkg.AuthResult, error)
	Logout(ctx context.Context, rawAccess string) error
	ResolveAccess(ctx context.Context, rawAccess string) (models.Access, *authpkg.ID, error)
	ChangePassword(ctx context.Context, ownerID authpkg.ID, current, newPassword string) error
	CreateInvite(ctx context.Context, ownerID authpkg.ID) (string, error)
	CreateAPIToken(ctx context.Context, ownerID authpkg.ID, label string) (string, authpkg.ID, error)
	ListAPITokens(ctx context.Context, ownerID authpkg.ID) ([]authpkg.APIToken, error)
	RevokeAPIToken(ctx context.Context, ownerID, tokenID authpkg.ID) error
	GetOwner(ctx context.Context, id authpkg.ID) (*authpkg.Owner, error)
}
```

(`authpkg` — алиас импорта: пакет `internal/httpapi` уже возможно захочет
`auth` как имя локальной переменной/параметра в обработчиках `division.go`
кое-где по смыслу «доступ», алиас снимает риск коллизии имени пакета с такими
местами; если коллизий не будет — implementer вправе использовать `auth` без
алиаса для читаемости, значения ради нет смысла спорить с гошным `goimports`.)

`internal/httpapi/middleware.go` (новый файл):

```go
package httpapi

import (
	"context"
	"net/http"

	authpkg "github.com/amarin/genodex/internal/auth"
	"github.com/amarin/genodex/internal/models"
)

const (
	accessCookieName  = "genodex_access"
	refreshCookieName = "genodex_refresh"
)

type ctxKey int

const (
	accessCtxKey ctxKey = iota
	ownerCtxKey
)

// resolveAccess читает genodex_access и кладёт в контекст models.Access и,
// при валидной сессии, OwnerID. Не блокирует запрос — решение «пускать или
// нет» принимает конкретный обработчик (requireFull ниже).
func resolveAccess(svc AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := ""
			if c, err := r.Cookie(accessCookieName); err == nil {
				raw = c.Value
			}

			access, ownerID, err := svc.ResolveAccess(r.Context(), raw)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})

				return
			}

			ctx := context.WithValue(r.Context(), accessCtxKey, access)
			if ownerID != nil {
				ctx = context.WithValue(ctx, ownerCtxKey, *ownerID)
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// AccessFromContext возвращает режим доступа; models.AccessPublic, если
// resolveAccess не отработал (не должно случаться на реальных маршрутах).
func AccessFromContext(ctx context.Context) models.Access {
	if a, ok := ctx.Value(accessCtxKey).(models.Access); ok {
		return a
	}

	return models.AccessPublic
}

// OwnerFromContext возвращает id владельца текущей сессии.
func OwnerFromContext(ctx context.Context) (authpkg.ID, bool) {
	id, ok := ctx.Value(ownerCtxKey).(authpkg.ID)

	return id, ok
}

// requireCSRFHeader требует заголовок X-Requested-With: genodex на любом
// POST/PUT/DELETE без исключений (auth.md §4, решение 9) — простая защита от
// CSRF для SPA на одном origin с cookie-сессией.
func requireCSRFHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mutating := r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete
		if mutating && r.Header.Get("X-Requested-With") != "genodex" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "отсутствует заголовок X-Requested-With"})

			return
		}

		next.ServeHTTP(w, r)
	})
}
```

- [ ] Шаг 2.1. TDD: `internal/httpapi/middleware_test.go`:

```go
package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	authpkg "github.com/amarin/genodex/internal/auth"
	"github.com/amarin/genodex/internal/models"
)

// fakeAuthService — минимальный фейк для тестов middleware (полный фейк —
// в задаче 3, с реализацией всех методов интерфейса).
type fakeAuthService struct {
	access  models.Access
	ownerID *authpkg.ID
	err     error
}

func (f *fakeAuthService) Bootstrap(context.Context) (bool, error) { return false, nil }
func (f *fakeAuthService) Register(context.Context, string, string, *string) (authpkg.AuthResult, error) {
	return authpkg.AuthResult{}, nil
}
func (f *fakeAuthService) Login(context.Context, string, string) (authpkg.AuthResult, error) {
	return authpkg.AuthResult{}, nil
}
func (f *fakeAuthService) Refresh(context.Context, string) (authpkg.AuthResult, error) {
	return authpkg.AuthResult{}, nil
}
func (f *fakeAuthService) Logout(context.Context, string) error { return nil }
func (f *fakeAuthService) ResolveAccess(context.Context, string) (models.Access, *authpkg.ID, error) {
	return f.access, f.ownerID, f.err
}
func (f *fakeAuthService) ChangePassword(context.Context, authpkg.ID, string, string) error { return nil }
func (f *fakeAuthService) CreateInvite(context.Context, authpkg.ID) (string, error) { return "", nil }
func (f *fakeAuthService) CreateAPIToken(context.Context, authpkg.ID, string) (string, authpkg.ID, error) {
	return "", "", nil
}
func (f *fakeAuthService) ListAPITokens(context.Context, authpkg.ID) ([]authpkg.APIToken, error) {
	return nil, nil
}
func (f *fakeAuthService) RevokeAPIToken(context.Context, authpkg.ID, authpkg.ID) error { return nil }
func (f *fakeAuthService) GetOwner(context.Context, authpkg.ID) (*authpkg.Owner, error) { return nil, nil }

var _ AuthService = (*fakeAuthService)(nil)

func TestResolveAccessPublicWithoutCookie(t *testing.T) {
	svc := &fakeAuthService{access: models.AccessPublic}
	var gotAccess models.Access
	var gotOwnerOK bool

	h := resolveAccess(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccess = AccessFromContext(r.Context())
		_, gotOwnerOK = OwnerFromContext(r.Context())
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if gotAccess != models.AccessPublic || gotOwnerOK {
		t.Fatalf("access=%v ownerOK=%v", gotAccess, gotOwnerOK)
	}
}

func TestResolveAccessFullWithOwner(t *testing.T) {
	owner := authpkg.ID("OW-1")
	svc := &fakeAuthService{access: models.AccessFull, ownerID: &owner}
	var gotAccess models.Access
	var gotOwner authpkg.ID

	h := resolveAccess(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAccess = AccessFromContext(r.Context())
		gotOwner, _ = OwnerFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: accessCookieName, Value: "raw-access-token"})
	h.ServeHTTP(httptest.NewRecorder(), req)

	if gotAccess != models.AccessFull || gotOwner != owner {
		t.Fatalf("access=%v owner=%v", gotAccess, gotOwner)
	}
}

func TestRequireCSRFHeaderBlocksMutatingWithoutHeader(t *testing.T) {
	called := false
	h := requireCSRFHeader(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		called = false
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "/", nil))

		if rec.Code != http.StatusBadRequest || called {
			t.Errorf("%s без заголовка: code=%d called=%v, ожидался 400 без вызова next", method, rec.Code, called)
		}
	}
}

func TestRequireCSRFHeaderAllowsMutatingWithHeader(t *testing.T) {
	called := false
	h := requireCSRFHeader(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-Requested-With", "genodex")
	h.ServeHTTP(httptest.NewRecorder(), req)

	if !called {
		t.Fatal("next не вызван при наличии заголовка")
	}
}

func TestRequireCSRFHeaderIgnoresGET(t *testing.T) {
	called := false
	h := requireCSRFHeader(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if !called {
		t.Fatal("GET должен проходить без заголовка")
	}
}
```

- [ ] Шаг 2.2. `go test ./internal/httpapi/...` зелёный; `gofmt -w internal/httpapi/`.
- [ ] Шаг 2.3. Рубеж; коммит `feat(httpapi): AuthService, resolveAccess, requireCSRFHeader`.

## Задача 3. `internal/httpapi/auth.go` — сессионные маршруты

`internal/httpapi/auth.go` (новый файл) — часть 1: cookie-хелперы,
`writeAuthError`, шесть обработчиков, `NewAuthHandler` (регистрирует пока эти
шесть — задача 4 дополнит тем же файлом ещё пять):

```go
package httpapi

import (
	"errors"
	"net/http"

	authpkg "github.com/amarin/genodex/internal/auth"
	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// NewAuthHandler строит обработчики /api/auth/*, обёрнутые resolveAccess и
// requireCSRFHeader. Подключается в общий mux на этапе C.
func NewAuthHandler(auth AuthService) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/auth/status", handleAuthStatus(auth))
	mux.HandleFunc("GET /api/auth/session", handleAuthSession(auth))
	mux.HandleFunc("POST /api/auth/register", handleAuthRegister(auth))
	mux.HandleFunc("POST /api/auth/login", handleAuthLogin(auth))
	mux.HandleFunc("POST /api/auth/logout", handleAuthLogout(auth))
	mux.HandleFunc("POST /api/auth/refresh", handleAuthRefresh(auth))

	return requireCSRFHeader(resolveAccess(auth)(mux))
}

// setSessionCookies выставляет пару access/refresh cookie (auth.md §4,
// решение 8): access — Path=/, refresh — Path=/api/auth/refresh (уже, чем
// сайт целиком). Secure — если запрос пришёл по HTTPS.
func setSessionCookies(w http.ResponseWriter, r *http.Request, res authpkg.AuthResult) {
	secure := r.TLS != nil

	http.SetCookie(w, &http.Cookie{
		Name: accessCookieName, Value: res.AccessToken, Path: "/",
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode,
		Expires: res.AccessExpiresAt,
	})
	http.SetCookie(w, &http.Cookie{
		Name: refreshCookieName, Value: res.RefreshToken, Path: "/api/auth/refresh",
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode,
		Expires: res.RefreshExpiresAt,
	})
}

// clearSessionCookies стирает обе cookie (логаут, смена пароля, просроченный
// refresh).
func clearSessionCookies(w http.ResponseWriter, r *http.Request) {
	secure := r.TLS != nil

	http.SetCookie(w, &http.Cookie{
		Name: accessCookieName, Value: "", Path: "/",
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteLaxMode, MaxAge: -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name: refreshCookieName, Value: "", Path: "/api/auth/refresh",
		HttpOnly: true, Secure: secure, SameSite: http.SameSiteStrictMode, MaxAge: -1,
	})
}

// requireFull — 401 без активной сессии; иначе OwnerID.
func requireFull(w http.ResponseWriter, r *http.Request) (authpkg.ID, bool) {
	ownerID, ok := OwnerFromContext(r.Context())
	if !ok || AccessFromContext(r.Context()) != models.AccessFull {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "нужен вход"})

		return "", false
	}

	return ownerID, true
}

// writeAuthError отвечает на ошибку auth.Service: *auth.ValidationError —
// 422 с полем; ErrInvalidCredentials/ErrSessionExpired — 401;
// ErrInviteRequired/ErrInviteInvalid — 422; ErrLoginTaken — 409; ErrNotFound
// — 404; остальное — 500.
func writeAuthError(w http.ResponseWriter, err error) {
	var ve *authpkg.ValidationError
	if errors.As(err, &ve) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": ve.Error(), "field": ve.Field})

		return
	}

	switch {
	case errors.Is(err, authpkg.ErrInvalidCredentials), errors.Is(err, authpkg.ErrSessionExpired):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
	case errors.Is(err, authpkg.ErrInviteRequired), errors.Is(err, authpkg.ErrInviteInvalid):
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": err.Error()})
	case errors.Is(err, authpkg.ErrLoginTaken):
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
	case errors.Is(err, authpkg.ErrNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}
}

// handleAuthStatus — GET /api/auth/status.
func handleAuthStatus(auth AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		boot, err := auth.Bootstrap(r.Context())
		if err != nil {
			writeAuthError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.AuthStatus{Bootstrap: boot})
	}
}

// handleAuthSession — GET /api/auth/session: нет сессии — 401.
func handleAuthSession(auth AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID, ok := requireFull(w, r)
		if !ok {
			return
		}

		owner, err := auth.GetOwner(r.Context(), ownerID)
		if err != nil {
			writeAuthError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.AuthSessionFromOwner(owner))
	}
}

// handleAuthRegister — POST /api/auth/register?invite=<raw>; тело
// {login, password}. bootstrap (нет владельцев) — invite не нужен.
func handleAuthRegister(auth AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in transport.RegisterRequest
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		var invite *string
		if raw := r.URL.Query().Get("invite"); raw != "" {
			invite = &raw
		}

		res, err := auth.Register(r.Context(), in.Login, in.Password, invite)
		if err != nil {
			writeAuthError(w, err)

			return
		}

		setSessionCookies(w, r, res)
		writeJSON(w, http.StatusCreated, transport.AuthSession{Login: in.Login})
	}
}

// handleAuthLogin — POST /api/auth/login; тело {login, password}.
func handleAuthLogin(auth AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in transport.LoginRequest
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		res, err := auth.Login(r.Context(), in.Login, in.Password)
		if err != nil {
			writeAuthError(w, err)

			return
		}

		setSessionCookies(w, r, res)
		writeJSON(w, http.StatusOK, transport.AuthSession{Login: in.Login})
	}
}

// handleAuthLogout — POST /api/auth/logout: требует Full.
func handleAuthLogout(auth AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if c, err := r.Cookie(accessCookieName); err == nil {
			if err := auth.Logout(r.Context(), c.Value); err != nil {
				writeAuthError(w, err)

				return
			}
		}

		clearSessionCookies(w, r)
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleAuthRefresh — POST /api/auth/refresh: по refresh-cookie (не по
// access — доступ проверяется отдельно от общего resolveAccess).
func handleAuthRefresh(auth AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(refreshCookieName)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "нет refresh-сессии"})

			return
		}

		res, err := auth.Refresh(r.Context(), c.Value)
		if err != nil {
			clearSessionCookies(w, r)
			writeAuthError(w, err)

			return
		}

		owner, err := auth.GetOwner(r.Context(), res.OwnerID)
		if err != nil {
			writeAuthError(w, err)

			return
		}

		setSessionCookies(w, r, res)
		writeJSON(w, http.StatusOK, transport.AuthSessionFromOwner(owner))
	}
}
```

- [ ] Шаг 3.1. TDD: `internal/httpapi/auth_test.go` (новый файл) — полный
  `fakeAuthService` (реализует весь интерфейс, с полями для контроля каждого
  метода и подсчётом вызовов — по образцу `fakeDivisions` в `division_test.go`):

```go
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authpkg "github.com/amarin/genodex/internal/auth"
	"github.com/amarin/genodex/internal/models"
)

// fakeAuthService — полный фейк AuthService для тестов auth.go.
type fakeAuthService struct {
	bootstrap bool

	registerResult authpkg.AuthResult
	registerErr    error
	gotRegister    struct {
		login, password string
		invite          *string
	}

	loginResult authpkg.AuthResult
	loginErr    error
	gotLogin    struct{ login, password string }

	refreshResult authpkg.AuthResult
	refreshErr    error
	gotRefresh    string

	logoutErr error
	gotLogout string

	access  models.Access
	ownerID *authpkg.ID
	resolveErr error

	changePasswordErr error
	gotChangePassword struct {
		ownerID          authpkg.ID
		current, newPass string
	}

	invite    string
	inviteErr error
	gotInviteOwner authpkg.ID

	tokenRaw, tokenID string
	tokenErr          error
	gotCreateToken    struct {
		ownerID authpkg.ID
		label   string
	}

	tokens    []authpkg.APIToken
	tokensErr error
	gotListOwner authpkg.ID

	revokeErr error
	gotRevoke struct{ ownerID, tokenID authpkg.ID }

	owner    *authpkg.Owner
	ownerErr error
	gotOwnerID authpkg.ID
}

func (f *fakeAuthService) Bootstrap(context.Context) (bool, error) { return f.bootstrap, nil }

func (f *fakeAuthService) Register(_ context.Context, login, password string, invite *string) (authpkg.AuthResult, error) {
	f.gotRegister.login, f.gotRegister.password, f.gotRegister.invite = login, password, invite

	return f.registerResult, f.registerErr
}

func (f *fakeAuthService) Login(_ context.Context, login, password string) (authpkg.AuthResult, error) {
	f.gotLogin.login, f.gotLogin.password = login, password

	return f.loginResult, f.loginErr
}

func (f *fakeAuthService) Refresh(_ context.Context, raw string) (authpkg.AuthResult, error) {
	f.gotRefresh = raw

	return f.refreshResult, f.refreshErr
}

func (f *fakeAuthService) Logout(_ context.Context, raw string) error {
	f.gotLogout = raw

	return f.logoutErr
}

func (f *fakeAuthService) ResolveAccess(context.Context, string) (models.Access, *authpkg.ID, error) {
	return f.access, f.ownerID, f.resolveErr
}

func (f *fakeAuthService) ChangePassword(_ context.Context, ownerID authpkg.ID, current, newPass string) error {
	f.gotChangePassword.ownerID, f.gotChangePassword.current, f.gotChangePassword.newPass = ownerID, current, newPass

	return f.changePasswordErr
}

func (f *fakeAuthService) CreateInvite(_ context.Context, ownerID authpkg.ID) (string, error) {
	f.gotInviteOwner = ownerID

	return f.invite, f.inviteErr
}

func (f *fakeAuthService) CreateAPIToken(_ context.Context, ownerID authpkg.ID, label string) (string, authpkg.ID, error) {
	f.gotCreateToken.ownerID, f.gotCreateToken.label = ownerID, label

	return f.tokenRaw, authpkg.ID(f.tokenID), f.tokenErr
}

func (f *fakeAuthService) ListAPITokens(_ context.Context, ownerID authpkg.ID) ([]authpkg.APIToken, error) {
	f.gotListOwner = ownerID

	return f.tokens, f.tokensErr
}

func (f *fakeAuthService) RevokeAPIToken(_ context.Context, ownerID, tokenID authpkg.ID) error {
	f.gotRevoke.ownerID, f.gotRevoke.tokenID = ownerID, tokenID

	return f.revokeErr
}

func (f *fakeAuthService) GetOwner(_ context.Context, id authpkg.ID) (*authpkg.Owner, error) {
	f.gotOwnerID = id

	return f.owner, f.ownerErr
}

var _ AuthService = (*fakeAuthService)(nil)

func postAuth(t *testing.T, h http.Handler, target, body string, withCSRF bool) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	if withCSRF {
		req.Header.Set("X-Requested-With", "genodex")
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func TestAuthStatusBootstrap(t *testing.T) {
	svc := &fakeAuthService{bootstrap: true}
	h := NewAuthHandler(svc)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/auth/status", nil))

	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"bootstrap":true}` {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
	}
}

func TestAuthRegisterBootstrap(t *testing.T) {
	svc := &fakeAuthService{registerResult: authpkg.AuthResult{
		OwnerID: "OW-1", AccessToken: "acc", RefreshToken: "ref",
	}}
	h := NewAuthHandler(svc)

	rec := postAuth(t, h, "/api/auth/register", `{"login":"first","password":"password123"}`, true)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
	}
	if svc.gotRegister.login != "first" || svc.gotRegister.invite != nil {
		t.Fatalf("gotRegister=%+v", svc.gotRegister)
	}

	var access, refresh bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == accessCookieName && c.Value == "acc" {
			access = true
		}
		if c.Name == refreshCookieName && c.Value == "ref" {
			refresh = true
		}
	}
	if !access || !refresh {
		t.Fatalf("cookies = %v", rec.Result().Cookies())
	}
}

// TestAuthRegisterRequiresInviteAfterBootstrap: query-параметр invite
// передаётся сценарию как есть (сам сценарий решает bootstrap/обязательность —
// здесь фейк просто отдаёт ErrInviteRequired, проверяем код и что параметр
// действительно дошёл).
func TestAuthRegisterRequiresInviteAfterBootstrap(t *testing.T) {
	svc := &fakeAuthService{registerErr: authpkg.ErrInviteRequired}
	h := NewAuthHandler(svc)

	rec := postAuth(t, h, "/api/auth/register", `{"login":"second","password":"password123"}`, true)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
	}

	svc2 := &fakeAuthService{registerResult: authpkg.AuthResult{OwnerID: "OW-2"}}
	h2 := NewAuthHandler(svc2)
	postAuth(t, h2, "/api/auth/register?invite=raw-invite-token", `{"login":"second","password":"password123"}`, true)

	if svc2.gotRegister.invite == nil || *svc2.gotRegister.invite != "raw-invite-token" {
		t.Fatalf("gotRegister.invite = %v", svc2.gotRegister.invite)
	}
}

func TestAuthLoginSetsCookies(t *testing.T) {
	svc := &fakeAuthService{loginResult: authpkg.AuthResult{
		OwnerID: "OW-1", AccessToken: "acc", RefreshToken: "ref",
	}}
	h := NewAuthHandler(svc)

	rec := postAuth(t, h, "/api/auth/login", `{"login":"user","password":"password123"}`, true)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body["login"] != "user" {
		t.Fatalf("body=%s err=%v", rec.Body, err)
	}

	found := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == accessCookieName {
			found = true
		}
	}
	if !found {
		t.Fatal("access cookie не выставлена")
	}
}

func TestAuthLoginInvalidCredentialsIs401(t *testing.T) {
	svc := &fakeAuthService{loginErr: authpkg.ErrInvalidCredentials}
	h := NewAuthHandler(svc)

	rec := postAuth(t, h, "/api/auth/login", `{"login":"user","password":"wrong"}`, true)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestAuthLogoutClearsSession(t *testing.T) {
	owner := authpkg.ID("OW-1")
	svc := &fakeAuthService{access: models.AccessFull, ownerID: &owner}
	h := NewAuthHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.Header.Set("X-Requested-With", "genodex")
	req.AddCookie(&http.Cookie{Name: accessCookieName, Value: "raw-access"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
	}
	if svc.gotLogout != "raw-access" {
		t.Fatalf("gotLogout=%q", svc.gotLogout)
	}

	cleared := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == accessCookieName && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("access cookie не очищена (MaxAge не отрицательный)")
	}
}

func TestAuthLogoutAnonymousIs401(t *testing.T) {
	svc := &fakeAuthService{access: models.AccessPublic}
	h := NewAuthHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
	req.Header.Set("X-Requested-With", "genodex")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestAuthRefreshRotates(t *testing.T) {
	svc := &fakeAuthService{
		refreshResult: authpkg.AuthResult{OwnerID: "OW-1", AccessToken: "new-acc", RefreshToken: "new-ref"},
		owner:         &authpkg.Owner{Login: "user"},
	}
	h := NewAuthHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	req.Header.Set("X-Requested-With", "genodex")
	req.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "old-ref"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
	}
	if svc.gotRefresh != "old-ref" {
		t.Fatalf("gotRefresh=%q", svc.gotRefresh)
	}

	var newAccess bool
	for _, c := range rec.Result().Cookies() {
		if c.Name == accessCookieName && c.Value == "new-acc" {
			newAccess = true
		}
	}
	if !newAccess {
		t.Fatal("новая access cookie не выставлена")
	}
}

// TestAuthRefreshReuseIsRejected: фейк отдаёт ErrSessionExpired (реальная
// ротация/одноразовость проверена на уровне auth.Service — этап A; здесь
// проверяем только HTTP-код и очистку cookies).
func TestAuthRefreshReuseIsRejected(t *testing.T) {
	svc := &fakeAuthService{refreshErr: authpkg.ErrSessionExpired}
	h := NewAuthHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	req.Header.Set("X-Requested-With", "genodex")
	req.AddCookie(&http.Cookie{Name: refreshCookieName, Value: "reused-old-ref"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
	}

	cleared := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == accessCookieName && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("cookies не очищены при просроченном refresh")
	}
}

func TestAuthRefreshNoCookieIs401(t *testing.T) {
	svc := &fakeAuthService{}
	h := NewAuthHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", nil)
	req.Header.Set("X-Requested-With", "genodex")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestAuthSessionNoCookieIs401(t *testing.T) {
	svc := &fakeAuthService{access: models.AccessPublic}
	h := NewAuthHandler(svc)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/auth/session", nil))

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestAuthSessionReturnsLogin(t *testing.T) {
	owner := authpkg.ID("OW-1")
	svc := &fakeAuthService{access: models.AccessFull, ownerID: &owner, owner: &authpkg.Owner{Login: "user"}}
	h := NewAuthHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/session", nil)
	req.AddCookie(&http.Cookie{Name: accessCookieName, Value: "raw-access"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"login":"user"}` {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
	}
}

func TestMissingCSRFHeaderIs4xx(t *testing.T) {
	svc := &fakeAuthService{}
	h := NewAuthHandler(svc)

	rec := postAuth(t, h, "/api/auth/login", `{"login":"user","password":"x"}`, false)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, ожидался 400 без X-Requested-With", rec.Code)
	}
}
```

- [ ] Шаг 3.2. `go test ./internal/httpapi/...` зелёный; `gofmt -w internal/httpapi/`.
- [ ] Шаг 3.3. Рубеж; коммит
  `feat(httpapi): auth.go — status/session/register/login/logout/refresh`.

## Задача 4. `internal/httpapi/auth.go` — владелец: пароль, invite, токены

Дополнить `internal/auth.go`: в `NewAuthHandler` добавить пять строк
регистрации маршрутов (после существующих шести):

```go
	mux.HandleFunc("POST /api/auth/password", handleAuthPassword(auth))
	mux.HandleFunc("POST /api/auth/invites", handleAuthCreateInvite(auth))
	mux.HandleFunc("POST /api/auth/tokens", handleAuthCreateToken(auth))
	mux.HandleFunc("GET /api/auth/tokens", handleAuthListTokens(auth))
	mux.HandleFunc("DELETE /api/auth/tokens/{id}", handleAuthRevokeToken(auth))
```

И добавить пять обработчиков (после существующих, тем же файлом):

```go
// handleAuthPassword — POST /api/auth/password: требует Full; тело
// {current_password, new_password}. Успех гасит ВСЕ сессии владельца
// (эффект Service.ChangePassword, этап A2) включая сессию самого вызывающего
// — обработчик поэтому сам чистит его cookies и отвечает 204 без тела; веб
// (этап D) обязан отправить пользователя на /login.
func handleAuthPassword(auth AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID, ok := requireFull(w, r)
		if !ok {
			return
		}

		var in transport.PasswordChangeRequest
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		if err := auth.ChangePassword(r.Context(), ownerID, in.CurrentPassword, in.NewPassword); err != nil {
			writeAuthError(w, err)

			return
		}

		clearSessionCookies(w, r)
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleAuthCreateInvite — POST /api/auth/invites: требует Full; тело не
// нужно. Сырое значение ссылки — только в этом ответе.
func handleAuthCreateInvite(auth AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID, ok := requireFull(w, r)
		if !ok {
			return
		}

		raw, err := auth.CreateInvite(r.Context(), ownerID)
		if err != nil {
			writeAuthError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.Invite{Token: raw})
	}
}

// handleAuthCreateToken — POST /api/auth/tokens: требует Full; тело
// {label}. Сырое значение токена — только в этом ответе.
func handleAuthCreateToken(auth AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID, ok := requireFull(w, r)
		if !ok {
			return
		}

		var in transport.CreateAPITokenRequest
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		raw, id, err := auth.CreateAPIToken(r.Context(), ownerID, in.Label)
		if err != nil {
			writeAuthError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.NewAPIToken{ID: string(id), Token: raw, Label: in.Label})
	}
}

// handleAuthListTokens — GET /api/auth/tokens: требует Full; метаданные без
// сырых значений.
func handleAuthListTokens(auth AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID, ok := requireFull(w, r)
		if !ok {
			return
		}

		list, err := auth.ListAPITokens(r.Context(), ownerID)
		if err != nil {
			writeAuthError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.APITokensFromModels(list))
	}
}

// handleAuthRevokeToken — DELETE /api/auth/tokens/{id}: требует Full;
// сценарий проверяет владельца токена (чужой — ErrNotFound → 404, не
// подтверждаем существование).
func handleAuthRevokeToken(auth AuthService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ownerID, ok := requireFull(w, r)
		if !ok {
			return
		}

		tokenID := authpkg.ID(strings.TrimPrefix(r.PathValue("id"), "/"))

		if err := auth.RevokeAPIToken(r.Context(), ownerID, tokenID); err != nil {
			writeAuthError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}
```

(добавить импорт `"strings"` в `internal/httpapi/auth.go`, если ещё не
импортирован — понадобится `strings.TrimPrefix`, как в `division_write.go:pathDivisionID`.)

- [ ] Шаг 4.1. TDD: дополнить `internal/httpapi/auth_test.go`:

```go
func TestAuthPasswordChangeRequiresFull(t *testing.T) {
	svc := &fakeAuthService{access: models.AccessPublic}
	h := NewAuthHandler(svc)

	rec := postAuth(t, h, "/api/auth/password", `{"current_password":"a","new_password":"b"}`, true)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d, ожидался 401 для анонима", rec.Code)
	}
}

func TestAuthPasswordChangeSuccessClearsCookies(t *testing.T) {
	owner := authpkg.ID("OW-1")
	svc := &fakeAuthService{access: models.AccessFull, ownerID: &owner}
	h := NewAuthHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/password",
		strings.NewReader(`{"current_password":"old","new_password":"new-password456"}`))
	req.Header.Set("X-Requested-With", "genodex")
	req.AddCookie(&http.Cookie{Name: accessCookieName, Value: "raw-access"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
	}
	if svc.gotChangePassword.ownerID != owner || svc.gotChangePassword.current != "old" {
		t.Fatalf("gotChangePassword=%+v", svc.gotChangePassword)
	}

	cleared := false
	for _, c := range rec.Result().Cookies() {
		if c.Name == accessCookieName && c.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Fatal("cookies не очищены после смены пароля")
	}
}

func TestAuthCreateInviteRequiresFull(t *testing.T) {
	svc := &fakeAuthService{access: models.AccessPublic}
	h := NewAuthHandler(svc)

	rec := postAuth(t, h, "/api/auth/invites", ``, true)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestAuthCreateInviteReturnsToken(t *testing.T) {
	owner := authpkg.ID("OW-1")
	svc := &fakeAuthService{access: models.AccessFull, ownerID: &owner, invite: "raw-invite-value"}
	h := NewAuthHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/invites", nil)
	req.Header.Set("X-Requested-With", "genodex")
	req.AddCookie(&http.Cookie{Name: accessCookieName, Value: "raw-access"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated || strings.TrimSpace(rec.Body.String()) != `{"token":"raw-invite-value"}` {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
	}
	if svc.gotInviteOwner != owner {
		t.Fatalf("gotInviteOwner=%v", svc.gotInviteOwner)
	}
}

// TestAuthTokensCRUDRequiresFull: create/list/revoke все требуют Full.
func TestAuthTokensCRUDRequiresFull(t *testing.T) {
	svc := &fakeAuthService{access: models.AccessPublic}
	h := NewAuthHandler(svc)

	cases := []struct {
		method, target, body string
	}{
		{http.MethodPost, "/api/auth/tokens", `{"label":"MCP"}`},
		{http.MethodGet, "/api/auth/tokens", ""},
		{http.MethodDelete, "/api/auth/tokens/AT-1", ""},
	}

	for _, c := range cases {
		req := httptest.NewRequest(c.method, c.target, strings.NewReader(c.body))
		req.Header.Set("X-Requested-With", "genodex")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s: status=%d, ожидался 401 для анонима", c.method, c.target, rec.Code)
		}
	}
}

func TestAuthCreateTokenReturnsRawOnce(t *testing.T) {
	owner := authpkg.ID("OW-1")
	svc := &fakeAuthService{
		access: models.AccessFull, ownerID: &owner,
		tokenRaw: "gnx_raw-value", tokenID: "AT-1",
	}
	h := NewAuthHandler(svc)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/tokens", strings.NewReader(`{"label":"MCP"}`))
	req.Header.Set("X-Requested-With", "genodex")
	req.AddCookie(&http.Cookie{Name: accessCookieName, Value: "raw-access"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	want := `{"id":"AT-1","token":"gnx_raw-value","label":"MCP"}`
	if rec.Code != http.StatusCreated || strings.TrimSpace(rec.Body.String()) != want {
		t.Fatalf("status=%d body=%s, want %s", rec.Code, rec.Body, want)
	}
	if svc.gotCreateToken.ownerID != owner || svc.gotCreateToken.label != "MCP" {
		t.Fatalf("gotCreateToken=%+v", svc.gotCreateToken)
	}
}

func TestAuthListTokensOmitsRawValue(t *testing.T) {
	owner := authpkg.ID("OW-1")
	svc := &fakeAuthService{
		access: models.AccessFull, ownerID: &owner,
		tokens: []authpkg.APIToken{{ID: "AT-1", Label: "MCP", TokenHash: "секрет"}},
	}
	h := NewAuthHandler(svc)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/tokens", nil)
	req.AddCookie(&http.Cookie{Name: accessCookieName, Value: "raw-access"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	body := rec.Body.String()
	if rec.Code != http.StatusOK || strings.Contains(body, "секрет") || !strings.Contains(body, `"id":"AT-1"`) {
		t.Fatalf("status=%d body=%s", rec.Code, body)
	}
	if svc.gotListOwner != owner {
		t.Fatalf("gotListOwner=%v", svc.gotListOwner)
	}
}

func TestAuthRevokeToken(t *testing.T) {
	owner := authpkg.ID("OW-1")
	svc := &fakeAuthService{access: models.AccessFull, ownerID: &owner}
	h := NewAuthHandler(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/auth/tokens/AT-1", nil)
	req.Header.Set("X-Requested-With", "genodex")
	req.AddCookie(&http.Cookie{Name: accessCookieName, Value: "raw-access"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body)
	}
	if svc.gotRevoke.ownerID != owner || svc.gotRevoke.tokenID != "AT-1" {
		t.Fatalf("gotRevoke=%+v", svc.gotRevoke)
	}
}

func TestAuthRevokeTokenNotFoundIs404(t *testing.T) {
	owner := authpkg.ID("OW-1")
	svc := &fakeAuthService{access: models.AccessFull, ownerID: &owner, revokeErr: authpkg.ErrNotFound}
	h := NewAuthHandler(svc)

	req := httptest.NewRequest(http.MethodDelete, "/api/auth/tokens/AT-nonexistent", nil)
	req.Header.Set("X-Requested-With", "genodex")
	req.AddCookie(&http.Cookie{Name: accessCookieName, Value: "raw-access"})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d", rec.Code)
	}
}
```

- [ ] Шаг 4.2. `go test ./internal/httpapi/...` зелёный; `gofmt -w internal/httpapi/`.
- [ ] Шаг 4.3. Рубеж: `go build ./...`, `go vet ./...`, `go test ./...`.
  Коммит `feat(httpapi): auth.go — password/invites/tokens`.

## Задача 5. Сквозной тест: `NewAuthHandler` на реальном `auth.Service`

`internal/httpapi/auth_service_test.go` (новый файл, `package httpapi_test` —
как `store_test.go` для делений, чтобы использовать `httpapi.NewAuthHandler`
и `auth.New`/`auth.NewSQLStore` извне пакета) — полный HTTP-сценарий на
настоящем `auth.Service` + `auth.SQLStore` (этапы A/A2), без фейка:

```go
package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amarin/genodex/internal/auth"
	"github.com/amarin/genodex/internal/httpapi"
	"github.com/amarin/genodex/internal/storage"
)

func newAuthHandler(t *testing.T) http.Handler {
	t.Helper()

	st, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	return httpapi.NewAuthHandler(auth.New(auth.NewSQLStore(st.DB())))
}

// TestAuthHTTPFullLifecycle: bootstrap-регистрация → логин → сессия →
// создание invite → регистрация по нему второго владельца → создание
// API-токена → смена пароля гасит сессию (следующий /session — 401) →
// повторный логин → отзыв токена.
func TestAuthHTTPFullLifecycle(t *testing.T) {
	h := newAuthHandler(t)

	// bootstrap: /status показывает true, регистрация без invite проходит
	statusRec := getAuth(t, h, "/api/auth/status", nil)
	requireBody(t, statusRec, http.StatusOK, `{"bootstrap":true}`)

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"first","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, refreshCookie := sessionCookies(t, regRec)

	// /status теперь false
	statusRec = getAuth(t, h, "/api/auth/status", nil)
	requireBody(t, statusRec, http.StatusOK, `{"bootstrap":false}`)

	// /session с валидной cookie
	sessRec := getAuth(t, h, "/api/auth/session", []*http.Cookie{accessCookie})
	requireBody(t, sessRec, http.StatusOK, `{"login":"first"}`)

	// invite → второй владелец
	inviteRec := postAuthReq(t, h, "/api/auth/invites", ``, []*http.Cookie{accessCookie})
	requireStatusS(t, inviteRec, http.StatusCreated)

	var inviteBody struct{ Token string `json:"token"` }
	if err := json.Unmarshal(inviteRec.Body.Bytes(), &inviteBody); err != nil {
		t.Fatalf("decode invite: %v", err)
	}

	reg2Rec := postAuthReq(t, h, "/api/auth/register?invite="+inviteBody.Token,
		`{"login":"second","password":"password123"}`, nil)
	requireStatusS(t, reg2Rec, http.StatusCreated)

	// без invite после bootstrap — 422
	reg3Rec := postAuthReq(t, h, "/api/auth/register", `{"login":"third","password":"password123"}`, nil)
	requireStatusS(t, reg3Rec, http.StatusUnprocessableEntity)

	// API-токен
	tokenRec := postAuthReq(t, h, "/api/auth/tokens", `{"label":"MCP"}`, []*http.Cookie{accessCookie})
	requireStatusS(t, tokenRec, http.StatusCreated)

	var tokenBody struct {
		ID    string `json:"id"`
		Token string `json:"token"`
		Label string `json:"label"`
	}
	if err := json.Unmarshal(tokenRec.Body.Bytes(), &tokenBody); err != nil {
		t.Fatalf("decode token: %v", err)
	}

	listRec := getAuth(t, h, "/api/auth/tokens", []*http.Cookie{accessCookie})
	requireStatusS(t, listRec, http.StatusOK)
	if !strings.Contains(listRec.Body.String(), `"label":"MCP"`) {
		t.Fatalf("list body = %s", listRec.Body)
	}

	// смена пароля гасит текущую сессию
	pwRec := postAuthReq(t, h, "/api/auth/password",
		`{"current_password":"password123","new_password":"new-password456"}`, []*http.Cookie{accessCookie})
	requireStatusS(t, pwRec, http.StatusNoContent)

	sessAfterPw := getAuth(t, h, "/api/auth/session", []*http.Cookie{accessCookie})
	requireStatusS(t, sessAfterPw, http.StatusUnauthorized)

	// refresh со старым refresh-cookie тоже мёртв (сессия удалена целиком)
	refreshRec := postAuthReq(t, h, "/api/auth/refresh", ``, []*http.Cookie{refreshCookie})
	requireStatusS(t, refreshRec, http.StatusUnauthorized)

	// логин новым паролем работает
	loginRec := postAuthReq(t, h, "/api/auth/login", `{"login":"first","password":"new-password456"}`, nil)
	requireStatusS(t, loginRec, http.StatusOK)

	newAccessCookie, _ := sessionCookies(t, loginRec)

	// отзыв токена
	revokeRec := deleteAuth(t, h, "/api/auth/tokens/"+tokenBody.ID, []*http.Cookie{newAccessCookie})
	requireStatusS(t, revokeRec, http.StatusNoContent)
}

func getAuth(t *testing.T, h http.Handler, target string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, target, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func postAuthReq(t *testing.T, h http.Handler, target, body string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, target, strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func deleteAuth(t *testing.T, h http.Handler, target string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodDelete, target, nil)
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func sessionCookies(t *testing.T, rec *httptest.ResponseRecorder) (access, refresh *http.Cookie) {
	t.Helper()

	for _, c := range rec.Result().Cookies() {
		switch c.Name {
		case "genodex_access":
			access = c
		case "genodex_refresh":
			refresh = c
		}
	}

	if access == nil || refresh == nil {
		t.Fatalf("cookies не выставлены: %v", rec.Result().Cookies())
	}

	return access, refresh
}

func requireBody(t *testing.T, rec *httptest.ResponseRecorder, wantCode int, wantBody string) {
	t.Helper()

	if rec.Code != wantCode || strings.TrimSpace(rec.Body.String()) != wantBody {
		t.Fatalf("status=%d body=%s, want %d %s", rec.Code, rec.Body, wantCode, wantBody)
	}
}
```

**Внимание при переносе примера в файл:** пакет `httpapi_test` уже объявляет
`requireStatusS(t, rec, want)` в `write_store_test.go` (тот же пакет, тот же
файл-набор) — **не объявляй эту функцию заново** в новом файле, просто
пользуйся существующей (сигнатура совпадает один в один: она и нужна). Из
примера выше используй `requireBody` как есть (нового имени в пакете ещё
нет) и `getAuth`/`postAuthReq`/`deleteAuth`/`sessionCookies` как есть (тоже
не пересекаются: существующие `getReq`/`postReq`/`putReq`/`delReq` в
`write_store_test.go` принимают только `path`/`body`, без `cookies` — другая
сигнатура и другое имя, конфликта нет).

- [ ] Шаг 5.1. Перед тем как писать файл, прочитать
  `internal/httpapi/write_store_test.go` целиком, чтобы не задеть уже
  занятые имена пакета `httpapi_test` (см. примечание выше — оно уже
  учитывает то, что там есть на момент написания этого плана; на всякий
  случай перепроверить, вдруг что-то успело добавиться).
- [ ] Шаг 5.2. `go test ./internal/httpapi/...` зелёный (включая новый
  сквозной тест); `gofmt -w internal/httpapi/`.
- [ ] Шаг 5.3. Рубеж: `go build ./...`, `go vet ./...`, `go test ./...`.
  Коммит `test(httpapi): NewAuthHandler на реальном auth.Service — сквозной сценарий`.

## Рубеж этапа

- `gofmt -l .` пусто, `go build ./...`, `go vet ./...`, `go test ./...`.
- `httpapi.NewAuthHandler` существует, полностью протестирован, но **не
  подключён** ни к `NewHandler`, ни к `internal/app` — это этап C.
- Обновить `docs/plans/2026-09-22-auth-roadmap.md` (статус этапа B) и
  `docs/data-model/auth.md` §4 (решение про `ChangePassword` чистит cookies
  вызывающего, а не перевыпускает сессию — см. «Архитектурные решения» выше).

## Коммиты

1. `feat(auth,transport): Service.GetOwner и DTO для HTTP-контракта auth`
2. `feat(httpapi): AuthService, resolveAccess, requireCSRFHeader`
3. `feat(httpapi): auth.go — status/session/register/login/logout/refresh`
4. `feat(httpapi): auth.go — password/invites/tokens`
5. `test(httpapi): NewAuthHandler на реальном auth.Service — сквозной сценарий`
