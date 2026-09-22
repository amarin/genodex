package mcp_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amarin/genodex/internal/auth"
	"github.com/amarin/genodex/internal/mcp"
	"github.com/amarin/genodex/internal/storage"
)

// TestRequireAPITokenOnRealService: создать владельца, выпустить токен,
// подтвердить его валидность/отзыв через настоящий auth.Service+SQLStore —
// не только фейк.
func TestRequireAPITokenOnRealService(t *testing.T) {
	st, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	svc := auth.New(auth.NewSQLStore(st.DB()))
	ctx := t.Context()

	reg, err := svc.Register(ctx, "user", "password123", nil)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	raw, tokenID, err := svc.CreateAPIToken(ctx, reg.OwnerID, "MCP")
	if err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}

	var gotOwner auth.ID
	h := mcp.RequireAPIToken(svc)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotOwner, _ = mcp.OwnerFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK || gotOwner != reg.OwnerID {
		t.Fatalf("code=%d gotOwner=%v", rec.Code, gotOwner)
	}

	if err := svc.RevokeAPIToken(ctx, reg.OwnerID, tokenID); err != nil {
		t.Fatalf("RevokeAPIToken: %v", err)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("после отзыва: code=%d, ожидался 401", rec.Code)
	}
}
