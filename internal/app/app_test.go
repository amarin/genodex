package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amarin/genodex/web"
)

// TestAppRejectsUnauthenticatedWrites: сквозная проверка сборки New —
// реальная сборка действительно подключает auth к /api и /mcp (этап C).
// Полное поведение самого auth — в internal/httpapi и internal/mcp; здесь —
// только доказательство того, что New их реально соединяет.
func TestAppRejectsUnauthenticatedWrites(t *testing.T) {
	a, err := New(Config{DataDir: t.TempDir(), Port: 0, WebMode: web.ModeProd})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = a.store.Close() })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin-divisions", nil)
	req.Header.Set("X-Requested-With", "genodex")
	a.http.Handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("POST /api/admin-divisions без сессии = %d, ожидался 401", rec.Code)
	}

	rec = httptest.NewRecorder()
	a.http.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("POST /mcp без токена = %d, ожидался 401", rec.Code)
	}

	// Позитивный ассерт: чтение по-прежнему доступно анонимно — полный
	// misroute внутри NewAPIHandler (например, requireFull по ошибке
	// накрывший весь /api/) тоже был бы пойман здесь, на уровне сборки
	// приложения, а не только в юнит-тестах пакета httpapi.
	rec = httptest.NewRecorder()
	a.http.Handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/health", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/health = %d, ожидался 200", rec.Code)
	}
}
