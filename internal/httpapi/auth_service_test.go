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

	return httpapi.NewAuthHandler(auth.New(auth.NewSQLStore(st.DB())), false)
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

	var inviteBody struct {
		Token string `json:"token"`
	}
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
