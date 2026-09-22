package mcp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amarin/genodex/internal/auth"
	"github.com/amarin/genodex/internal/models"
)

// fakeTokenResolver — фейк для тестов middleware.
type fakeTokenResolver struct {
	ownerID auth.ID
	err     error
	gotRaw  string
}

func (f *fakeTokenResolver) ResolveAPIToken(_ context.Context, raw string) (auth.ID, error) {
	f.gotRaw = raw

	return f.ownerID, f.err
}

var _ TokenResolver = (*fakeTokenResolver)(nil)

func TestRequireAPITokenNoHeaderIs401(t *testing.T) {
	called := false
	svc := &fakeTokenResolver{}
	h := RequireAPIToken(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", nil))

	if rec.Code != http.StatusUnauthorized || called {
		t.Fatalf("code=%d called=%v, ожидался 401 без вызова next", rec.Code, called)
	}
}

func TestRequireAPITokenMalformedHeaderIs401(t *testing.T) {
	svc := &fakeTokenResolver{}
	h := RequireAPIToken(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	for _, header := range []string{"gnx_raw-no-bearer-prefix", "Bearer", "Bearer "} {
		req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Authorization=%q: code=%d, ожидался 401", header, rec.Code)
		}
	}
}

func TestRequireAPITokenInvalidTokenIs401(t *testing.T) {
	svc := &fakeTokenResolver{err: auth.ErrInvalidCredentials}
	h := RequireAPIToken(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer gnx_revoked-or-unknown")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d, ожидался 401", rec.Code)
	}
	if svc.gotRaw != "gnx_revoked-or-unknown" {
		t.Fatalf("gotRaw=%q", svc.gotRaw)
	}
}

func TestRequireAPITokenValidTokenResolvesContext(t *testing.T) {
	owner := auth.ID("OW-1")
	svc := &fakeTokenResolver{ownerID: owner}

	var gotAccess models.Access
	var gotOwner auth.ID
	var called bool

	h := RequireAPIToken(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		gotAccess = AccessFromContext(r.Context())
		gotOwner, _ = OwnerFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer gnx_valid-token")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if !called || gotAccess != models.AccessFull || gotOwner != owner {
		t.Fatalf("called=%v access=%v owner=%v", called, gotAccess, gotOwner)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d, ожидался 200 от next (не переопределён middleware)", rec.Code)
	}
}

func TestRequireAPITokenInfrastructureErrorIs500(t *testing.T) {
	called := false
	svc := &fakeTokenResolver{err: errors.New("db connection failed")}
	h := RequireAPIToken(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true }))

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer gnx_valid-format-but-db-down")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError || called {
		t.Fatalf("code=%d called=%v, ожидался 500 без вызова next", rec.Code, called)
	}
}

func TestAccessFromContextDefaultsToPublic(t *testing.T) {
	if got := AccessFromContext(context.Background()); got != models.AccessPublic {
		t.Fatalf("got=%v, ожидался AccessPublic по умолчанию", got)
	}
}

func TestOwnerFromContextMissing(t *testing.T) {
	if _, ok := OwnerFromContext(context.Background()); ok {
		t.Fatal("ожидался ok=false без резолвнутого владельца")
	}
}
