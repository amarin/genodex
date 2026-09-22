package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/auth"
	"github.com/amarin/genodex/internal/httpapi"
	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/store/sqlstore"
	"github.com/amarin/genodex/internal/transport"
)

// TestDivisionWriteContractWithRealStore: сквозной путь «хранилище → сценарии
// → HTTP» (так же собран internal/app) для записи делений — через
// NewAPIHandler с реальной сессией владельца: регистрация (bootstrap) →
// создание/чтение/изменение/удаление → 409 со списком ссылающихся →
// анонимная попытка записи — 401 (auth.md §6, приёмка этапа C).
func TestDivisionWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(newDivisionService(t, st), authSvc, fstest.MapFS{})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание корня.
	root := createDivision(t, h, owner, `{"name":"Московская","type":"governorate"}`, http.StatusCreated)
	if root.Name != "Московская" || root.Type != models.AdminDivisionGovernorate || root.ParentID != nil {
		t.Fatalf("root = %+v", root)
	}
	if !strings.HasPrefix(string(root.ID), "AD-") {
		t.Fatalf("id = %q", root.ID)
	}

	// Создание дочерней.
	child := createDivision(t, h, owner,
		fmt.Sprintf(`{"name":"Давыдово","type":"selo","parent_id":%q}`, root.ID), http.StatusCreated)
	if child.ParentID == nil || *child.ParentID != root.ID {
		t.Fatalf("child = %+v", child)
	}

	// Чтение по id — открыто анонимному посетителю.
	rec := getReq(t, h, "/api/admin-divisions/"+string(child.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"name":"Давыдово"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена полей name/type/parent_id.
	rec = putReq(t, h, owner, "/api/admin-divisions/"+string(child.ID),
		fmt.Sprintf(`{"name":"Давыдова","type":"selo","parent_id":%q}`, root.ID))
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeDivisionS(t, rec)
	if updated.Name != "Давыдова" || updated.ID != child.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Родитель занят дочерью → 409 со списком ссылающихся.
	rec = delReq(t, h, owner, "/api/admin-divisions/"+string(root.ID))
	requireStatusS(t, rec, http.StatusConflict)
	if !strings.Contains(rec.Body.String(), `"referrers"`) || !strings.Contains(rec.Body.String(), string(child.ID)) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Удаление дочерней, затем корня.
	delReq(t, h, owner, "/api/admin-divisions/"+string(child.ID))
	requireStatusS(t, delReq(t, h, owner, "/api/admin-divisions/"+string(root.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/admin-divisions/"+string(root.ID)), http.StatusNotFound)

	// Неверный формат id в пути — 422 (не 404).
	requireStatusS(t, getReq(t, h, "/api/admin-divisions/obvious-bad"), http.StatusUnprocessableEntity)

	// Несуществующий родитель (id валидного формата) — 422 с полем parent_id.
	rec = postReq(t, h, owner, `{"name":"Давыдово","type":"selo","parent_id":"AD-01ARZ3NDEKTSV4RRFFQ69G5FA9"}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"parent_id"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Анонимная попытка создать — 401, до разбора тела.
	anonRec := postReq(t, h, nil, `{"name":"Аноним","type":"selo"}`)
	requireStatusS(t, anonRec, http.StatusUnauthorized)
}

func createDivision(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.AdminDivision {
	t.Helper()

	rec := postReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeDivisionS(t, rec)
}

func decodeDivisionS(t *testing.T, rec *httptest.ResponseRecorder) transport.AdminDivision {
	t.Helper()

	var d transport.AdminDivision
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return d
}

func postReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/admin-divisions", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func getReq(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))

	return rec
}

func putReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func delReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodDelete, path, nil)
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func requireStatusS(t *testing.T, rec *httptest.ResponseRecorder, want int) {
	t.Helper()

	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}
}
