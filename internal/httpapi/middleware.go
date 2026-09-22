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
