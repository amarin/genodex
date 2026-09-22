package httpapi

import (
	"errors"
	"net/http"
	"strings"

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
	mux.HandleFunc("POST /api/auth/password", handleAuthPassword(auth))
	mux.HandleFunc("POST /api/auth/invites", handleAuthCreateInvite(auth))
	mux.HandleFunc("POST /api/auth/tokens", handleAuthCreateToken(auth))
	mux.HandleFunc("GET /api/auth/tokens", handleAuthListTokens(auth))
	mux.HandleFunc("DELETE /api/auth/tokens/{id}", handleAuthRevokeToken(auth))

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
