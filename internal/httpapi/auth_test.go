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

	access     models.Access
	ownerID    *authpkg.ID
	resolveErr error

	changePasswordErr error
	gotChangePassword struct {
		ownerID          authpkg.ID
		current, newPass string
	}

	invite         string
	inviteErr      error
	gotInviteOwner authpkg.ID

	tokenRaw, tokenID string
	tokenErr          error
	gotCreateToken    struct {
		ownerID authpkg.ID
		label   string
	}

	tokens       []authpkg.APIToken
	tokensErr    error
	gotListOwner authpkg.ID

	revokeErr error
	gotRevoke struct{ ownerID, tokenID authpkg.ID }

	owner      *authpkg.Owner
	ownerErr   error
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
