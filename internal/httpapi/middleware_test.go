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
func (f *fakeAuthService) ChangePassword(context.Context, authpkg.ID, string, string) error {
	return nil
}
func (f *fakeAuthService) CreateInvite(context.Context, authpkg.ID) (string, error) { return "", nil }
func (f *fakeAuthService) CreateAPIToken(context.Context, authpkg.ID, string) (string, authpkg.ID, error) {
	return "", "", nil
}
func (f *fakeAuthService) ListAPITokens(context.Context, authpkg.ID) ([]authpkg.APIToken, error) {
	return nil, nil
}
func (f *fakeAuthService) RevokeAPIToken(context.Context, authpkg.ID, authpkg.ID) error { return nil }
func (f *fakeAuthService) GetOwner(context.Context, authpkg.ID) (*authpkg.Owner, error) {
	return nil, nil
}

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
