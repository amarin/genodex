package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/amarin/genodex/internal/auth"
	"github.com/amarin/genodex/internal/models"
)

// TokenResolver — узкий контракт auth.Service для bearer-аутентификации MCP:
// ResolveAPIToken сама обновляет last_used_at изнутри (best-effort), лишнего
// вызова TouchAPIToken тут не нужно.
type TokenResolver interface {
	ResolveAPIToken(ctx context.Context, raw string) (auth.ID, error)
}

type ctxKey int

const (
	accessCtxKey ctxKey = iota
	ownerCtxKey
)

// RequireAPIToken — middleware на /mcp: без валидного Authorization: Bearer
// gnx_... — 401 до входа в протокол MCP (auth.md §5, решение #33 — MCP
// исключительно владельческий инструмент, анонимного пути нет вовсе, в
// отличие от httpapi.resolveAccess).
func RequireAPIToken(svc TokenResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := bearerToken(r)
			if !ok {
				writeUnauthorized(w)

				return
			}

			ownerID, err := svc.ResolveAPIToken(r.Context(), raw)
			if err != nil {
				// сбой инфраструктуры не маскируем под отказ по аутентификации — отличаем
				// явную ошибку токена (не найден/отозван) от невозможности проверить
				if errors.Is(err, auth.ErrInvalidCredentials) {
					writeUnauthorized(w)
				} else {
					writeInternalError(w, err)
				}

				return
			}

			ctx := context.WithValue(r.Context(), accessCtxKey, models.AccessFull)
			ctx = context.WithValue(ctx, ownerCtxKey, ownerID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// bearerToken разбирает заголовок Authorization: Bearer <token>; пусто после
// префикса — тоже отсутствие токена. Схема case-insensitive по RFC 7235.
func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	if h == "" {
		return "", false
	}

	scheme, token, ok := strings.Cut(h, " ")
	if !ok || !strings.EqualFold(scheme, "Bearer") {
		return "", false
	}

	token = strings.TrimSpace(token)
	if token == "" {
		return "", false
	}

	return token, true
}

// writeUnauthorized — минимальный JSON-ответ 401 (без зависимости от
// httpapi.writeJSON — другой пакет, ответ MCP не обязан повторять его формат
// дословно, важен код и то, что тело — валидный JSON).
func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"нужен валидный API-токен (Authorization: Bearer gnx_...)"}`))
}

// writeInternalError — JSON-ответ 500 для сбоев инфраструктуры (например, БД недоступна).
// Отражает реальную причину вместо молчаливого маскирования под отказ по аутентификации.
func writeInternalError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)
	resp := map[string]string{
		"error": "внутренняя ошибка сервера: " + err.Error(),
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// AccessFromContext возвращает режим доступа; models.AccessPublic, если
// RequireAPIToken не отработал (не должно случаться на реальных маршрутах —
// при отсутствии валидного токена запрос блокируется раньше, до обработчика).
func AccessFromContext(ctx context.Context) models.Access {
	if a, ok := ctx.Value(accessCtxKey).(models.Access); ok {
		return a
	}

	return models.AccessPublic
}

// OwnerFromContext возвращает id владельца токена, которым выполнен запрос.
func OwnerFromContext(ctx context.Context) (auth.ID, bool) {
	id, ok := ctx.Value(ownerCtxKey).(auth.ID)

	return id, ok
}
