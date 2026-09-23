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
	"github.com/amarin/genodex/internal/idgen"
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
	h := httpapi.NewAPIHandler(httpapi.Deps{Divisions: newDivisionService(t, st), Auth: authSvc, DocsFS: fstest.MapFS{}, TrustProxy: false})

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
	requireStatusS(t, delReq(t, h, owner, "/api/admin-divisions/"+string(child.ID)), http.StatusNoContent)
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

	// Logout инвалидирует сессию: та же cookie больше не проходит requireFull.
	logoutRec := postAuthReq(t, h, "/api/auth/logout", "", owner)
	requireStatusS(t, logoutRec, http.StatusNoContent)

	staleRec := postReq(t, h, owner, `{"name":"После логаута","type":"selo"}`)
	requireStatusS(t, staleRec, http.StatusUnauthorized)

	// Анонимная попытка создать — 401, до разбора тела.
	anonRec := postReq(t, h, nil, `{"name":"Аноним","type":"selo"}`)
	requireStatusS(t, anonRec, http.StatusUnauthorized)
}

// TestSurnameWriteContractWithRealStore: сквозной путь «хранилище → сценарии
// → HTTP» (так же собран internal/app) для записи словарных записей фамилий
// — через NewAPIHandler с реальной сессией владельца: bootstrap-регистрация
// → создание → чтение → изменение → удаление → повторное чтение — 404
// (по образцу TestDivisionWriteContractWithRealStore, но без 409-сценария:
// у Surname нет строгих внешних ключей, docs/data-model/entity-write.md).
func TestSurnameWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Surnames:   newSurnameService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание записи.
	created := createSurname(t, h, owner,
		`{"canonical":"Иванов","variants":[{"text":"Иванова"}],"items":[],"notes":[]}`, http.StatusCreated)
	if created.Canonical != "Иванов" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "SN-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id — открыто анонимному посетителю.
	rec := getReq(t, h, "/api/surnames/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"canonical":"Иванов"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена canonical/variants/items/notes.
	rec = putSurnameReq(t, h, owner, "/api/surnames/"+string(created.ID),
		`{"canonical":"Иванова","variants":[],"items":[],"notes":[]}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeSurnameS(t, rec)
	if updated.Canonical != "Иванова" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление — у Surname нет строгих FK, конфликта не бывает.
	requireStatusS(t, delReq(t, h, owner, "/api/surnames/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/surnames/"+string(created.ID)), http.StatusNotFound)
}

func createSurname(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Surname {
	t.Helper()

	rec := postSurnameReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeSurnameS(t, rec)
}

func decodeSurnameS(t *testing.T, rec *httptest.ResponseRecorder) transport.Surname {
	t.Helper()

	var s transport.Surname
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return s
}

func postSurnameReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/surnames", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putSurnameReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestPatronymicWriteContractWithRealStore: сквозной путь «хранилище →
// сценарии → HTTP» (так же собран internal/app) для записи словарных
// записей отчеств — через NewAPIHandler с реальной сессией владельца:
// bootstrap-регистрация → создание → чтение → изменение → удаление →
// повторное чтение — 404 (по образцу TestSurnameWriteContractWithRealStore,
// без 409-сценария: у Patronymic нет строгих внешних ключей,
// docs/data-model/entity-write.md).
func TestPatronymicWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:   newDivisionService(t, st),
		Patronymics: newPatronymicService(t, st),
		Auth:        authSvc,
		DocsFS:      fstest.MapFS{},
		TrustProxy:  false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание записи.
	created := createPatronymic(t, h, owner,
		`{"canonical":"Иванович","variants":[{"text":"Иванычъ"}],"items":[],"notes":[]}`, http.StatusCreated)
	if created.Canonical != "Иванович" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "PN-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id — открыто анонимному посетителю.
	rec := getReq(t, h, "/api/patronymics/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"canonical":"Иванович"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена canonical/variants/items/notes.
	rec = putPatronymicReq(t, h, owner, "/api/patronymics/"+string(created.ID),
		`{"canonical":"Ивановна","variants":[],"items":[],"notes":[]}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodePatronymicS(t, rec)
	if updated.Canonical != "Ивановна" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление — у Patronymic нет строгих FK, конфликта не бывает.
	requireStatusS(t, delReq(t, h, owner, "/api/patronymics/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/patronymics/"+string(created.ID)), http.StatusNotFound)
}

func createPatronymic(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Patronymic {
	t.Helper()

	rec := postPatronymicReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodePatronymicS(t, rec)
}

func decodePatronymicS(t *testing.T, rec *httptest.ResponseRecorder) transport.Patronymic {
	t.Helper()

	var s transport.Patronymic
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return s
}

func postPatronymicReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/patronymics", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putPatronymicReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestEstateWriteContractWithRealStore: сквозной путь «хранилище → сценарии
// → HTTP» (так же собран internal/app) для записи словарных записей
// сословий — через NewAPIHandler с реальной сессией владельца:
// bootstrap-регистрация → создание → чтение → изменение → удаление →
// повторное чтение — 404 (по образцу TestSurnameWriteContractWithRealStore,
// без 409-сценария: у Estate нет строгих внешних ключей,
// docs/data-model/entity-write.md).
func TestEstateWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Estates:    newEstateService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание записи.
	created := createEstate(t, h, owner,
		`{"canonical":"крестьяне","variants":[{"text":"крестьянство"}],"items":[],"notes":[]}`, http.StatusCreated)
	if created.Canonical != "крестьяне" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "ES-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id — открыто анонимному посетителю.
	rec := getReq(t, h, "/api/estates/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"canonical":"крестьяне"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена canonical/variants/items/notes.
	rec = putEstateReq(t, h, owner, "/api/estates/"+string(created.ID),
		`{"canonical":"мещане","variants":[],"items":[],"notes":[]}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeEstateS(t, rec)
	if updated.Canonical != "мещане" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление — у Estate нет строгих FK, конфликта не бывает.
	requireStatusS(t, delReq(t, h, owner, "/api/estates/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/estates/"+string(created.ID)), http.StatusNotFound)
}

func createEstate(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Estate {
	t.Helper()

	rec := postEstateReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeEstateS(t, rec)
}

func decodeEstateS(t *testing.T, rec *httptest.ResponseRecorder) transport.Estate {
	t.Helper()

	var s transport.Estate
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return s
}

func postEstateReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/estates", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putEstateReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestTitleWriteContractWithRealStore: сквозной путь «хранилище → сценарии
// → HTTP» (так же собран internal/app) для записи словарных записей
// титулов — через NewAPIHandler с реальной сессией владельца:
// bootstrap-регистрация → создание → чтение → изменение → удаление →
// повторное чтение — 404 (по образцу TestSurnameWriteContractWithRealStore,
// без 409-сценария: у Title нет строгих внешних ключей,
// docs/data-model/entity-write.md).
func TestTitleWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Titles:     newTitleService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание записи.
	created := createTitle(t, h, owner,
		`{"canonical":"вдова","variants":[{"text":"вдовица"}],"items":[],"notes":[]}`, http.StatusCreated)
	if created.Canonical != "вдова" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "TT-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id — открыто анонимному посетителю.
	rec := getReq(t, h, "/api/titles/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"canonical":"вдова"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена canonical/variants/items/notes.
	rec = putTitleReq(t, h, owner, "/api/titles/"+string(created.ID),
		`{"canonical":"вдовец","variants":[],"items":[],"notes":[]}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeTitleS(t, rec)
	if updated.Canonical != "вдовец" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление — у Title нет строгих FK, конфликта не бывает.
	requireStatusS(t, delReq(t, h, owner, "/api/titles/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/titles/"+string(created.ID)), http.StatusNotFound)
}

func createTitle(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Title {
	t.Helper()

	rec := postTitleReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeTitleS(t, rec)
}

func decodeTitleS(t *testing.T, rec *httptest.ResponseRecorder) transport.Title {
	t.Helper()

	var s transport.Title
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return s
}

func postTitleReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/titles", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putTitleReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestGivenNameWriteContractWithRealStore: сквозной путь «хранилище →
// сценарии → HTTP» (так же собран internal/app) для записи словарных
// записей имён — через NewAPIHandler с реальной сессией владельца:
// bootstrap-регистрация → создание → чтение → изменение → удаление →
// повторное чтение — 404 (по образцу TestSurnameWriteContractWithRealStore,
// без 409-сценария: у GivenName нет строгих внешних ключей,
// docs/data-model/entity-write.md). Дополнительно проверяет поле gender —
// единственное отличие GivenName от остальных трёх словарных сущностей.
func TestGivenNameWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		GivenNames: newGivenNameService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание записи с полом male.
	created := createGivenName(t, h, owner,
		`{"canonical":"Иван","gender":"male","variants":[{"text":"Иоанн"}],"items":[],"notes":[]}`, http.StatusCreated)
	if created.Canonical != "Иван" {
		t.Fatalf("created = %+v", created)
	}
	if created.Gender != models.NameGenderMale {
		t.Fatalf("created.Gender = %q, want male", created.Gender)
	}
	if !strings.HasPrefix(string(created.ID), "GN-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id — открыто анонимному посетителю, gender виден в ответе.
	rec := getReq(t, h, "/api/given-names/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"canonical":"Иван"`) || !strings.Contains(rec.Body.String(), `"gender":"male"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена canonical/gender/variants/items/notes, gender → neutral.
	rec = putGivenNameReq(t, h, owner, "/api/given-names/"+string(created.ID),
		`{"canonical":"Саша","gender":"neutral","variants":[],"items":[],"notes":[]}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeGivenNameS(t, rec)
	if updated.Canonical != "Саша" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}
	if updated.Gender != models.NameGenderNeutral {
		t.Fatalf("updated.Gender = %q, want neutral", updated.Gender)
	}

	// Удаление — у GivenName нет строгих FK, конфликта не бывает.
	requireStatusS(t, delReq(t, h, owner, "/api/given-names/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/given-names/"+string(created.ID)), http.StatusNotFound)
}

func createGivenName(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.GivenName {
	t.Helper()

	rec := postGivenNameReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeGivenNameS(t, rec)
}

func decodeGivenNameS(t *testing.T, rec *httptest.ResponseRecorder) transport.GivenName {
	t.Helper()

	var s transport.GivenName
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return s
}

func postGivenNameReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/given-names", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putGivenNameReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestDivisionCreateWithoutCSRFHeaderIs400: валидная сессия владельца, но без
// X-Requested-With — 400 (requireCSRFHeader), сценарий не вызывается. Пин на
// то, что NewAPIHandler реально оборачивает division-записи, не только auth.
func TestDivisionCreateWithoutCSRFHeaderIs400(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{Divisions: newDivisionService(t, st), Auth: authSvc, DocsFS: fstest.MapFS{}, TrustProxy: false})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)

	req := httptest.NewRequest(http.MethodPost, "/api/admin-divisions", strings.NewReader(`{"name":"x","type":"selo"}`))
	req.AddCookie(accessCookie)
	// нарочно без X-Requested-With

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	requireStatusS(t, rec, http.StatusBadRequest)
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

// TestRepositoryWriteContractWithRealStore: сквозной путь «хранилище →
// сценарии → HTTP» (по образцу TestGivenNameWriteContractWithRealStore) для
// хранилищ-контейнеров источников: bootstrap-регистрация → создание →
// чтение → изменение → удаление → повторное чтение — 404. Дополнительно
// закрывает Fix 1 (CRITICAL): приватная запись, созданная владельцем, должна
// быть недоступна анонимному GET /api/repositories/{id} — 404, а не 200.
func TestRepositoryWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:    newDivisionService(t, st),
		Repositories: newRepositoryService(t, st),
		Auth:         authSvc,
		DocsFS:       fstest.MapFS{},
		TrustProxy:   false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание.
	created := createRepository(t, h, owner,
		`{"name":"ГАВО","type":"archive","address":"Вологда","urls":[],"notes":[],"private":false}`, http.StatusCreated)
	if created.Name != "ГАВО" || created.Type != "archive" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "R-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id — открыто анонимному посетителю.
	rec := getReq(t, h, "/api/repositories/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"name":"ГАВО"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена name/type/address/urls/notes/private.
	rec = putRepositoryReq(t, h, owner, "/api/repositories/"+string(created.ID),
		`{"name":"ГАВО (испр.)","type":"library","address":"Вологда","urls":[],"notes":[],"private":false}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeRepositoryS(t, rec)
	if updated.Name != "ГАВО (испр.)" || updated.Type != "library" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/repositories/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/repositories/"+string(created.ID)), http.StatusNotFound)

	// Fix 1: приватная запись, анонимный GET — 404, не 200.
	private := createRepository(t, h, owner,
		`{"name":"Частное собрание","type":"private","address":"","urls":[],"notes":[],"private":true}`, http.StatusCreated)
	if !private.Private {
		t.Fatalf("private = %+v, want Private=true", private)
	}

	requireStatusS(t, getReq(t, h, "/api/repositories/"+string(private.ID)), http.StatusNotFound)
}

func createRepository(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Repository {
	t.Helper()

	rec := postRepositoryReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeRepositoryS(t, rec)
}

func decodeRepositoryS(t *testing.T, rec *httptest.ResponseRecorder) transport.Repository {
	t.Helper()

	var r transport.Repository
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return r
}

func postRepositoryReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/repositories", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putRepositoryReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestChurchWriteContractWithRealStore: сквозной путь «хранилище → сценарии →
// HTTP» для церквей (по образцу TestGivenNameWriteContractWithRealStore, без
// приватности — у Church нет поля Private): bootstrap-регистрация →
// создание → чтение → изменение → удаление → повторное чтение — 404.
func TestChurchWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Churches:   newChurchService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание.
	created := createChurch(t, h, owner,
		`{"name":"Троицкая церковь","settlements":[],"variants":[],"notes":[]}`, http.StatusCreated)
	if created.Name != "Троицкая церковь" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "CH-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id.
	rec := getReq(t, h, "/api/churches/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"name":"Троицкая церковь"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена name/parish/settlements/variants/notes.
	rec = putChurchReq(t, h, owner, "/api/churches/"+string(created.ID),
		`{"name":"Троицкая церковь (испр.)","settlements":[],"variants":[],"notes":[]}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeChurchS(t, rec)
	if updated.Name != "Троицкая церковь (испр.)" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/churches/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/churches/"+string(created.ID)), http.StatusNotFound)
}

func createChurch(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Church {
	t.Helper()

	rec := postChurchReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeChurchS(t, rec)
}

func decodeChurchS(t *testing.T, rec *httptest.ResponseRecorder) transport.Church {
	t.Helper()

	var c transport.Church
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return c
}

func postChurchReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/churches", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putChurchReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestParishWriteContractWithRealStore: сквозной путь «хранилище → сценарии →
// HTTP» для приходов (по образцу TestGivenNameWriteContractWithRealStore, без
// приватности — у Parish нет поля Private): bootstrap-регистрация →
// создание → чтение → изменение → удаление → повторное чтение — 404.
func TestParishWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Parishes:   newParishService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание.
	created := createParish(t, h, owner,
		`{"name":"Троицкий приход","settlements":[],"notes":[]}`, http.StatusCreated)
	if created.Name != "Троицкий приход" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "PR-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id.
	rec := getReq(t, h, "/api/parishes/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"name":"Троицкий приход"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена name/church/settlements/since/until/notes.
	rec = putParishReq(t, h, owner, "/api/parishes/"+string(created.ID),
		`{"name":"Троицкий приход (испр.)","settlements":[],"notes":[]}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeParishS(t, rec)
	if updated.Name != "Троицкий приход (испр.)" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/parishes/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/parishes/"+string(created.ID)), http.StatusNotFound)
}

func createParish(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Parish {
	t.Helper()

	rec := postParishReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeParishS(t, rec)
}

func decodeParishS(t *testing.T, rec *httptest.ResponseRecorder) transport.Parish {
	t.Helper()

	var p transport.Parish
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return p
}

func postParishReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/parishes", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putParishReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestArchiveWriteContractWithRealStore: сквозной путь «хранилище → сценарии
// → HTTP» для архивов (по образцу TestGivenNameWriteContractWithRealStore):
// bootstrap-регистрация → создание → чтение → изменение → удаление →
// повторное чтение — 404. Дополнительно: строгий FK на Repository (валидный
// repository_id сохраняется; несуществующий, но корректный по формату —
// 422 на поле repository_id) и Fix 1 (CRITICAL): приватная запись, анонимный
// GET — 404, не 200.
func TestArchiveWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:    newDivisionService(t, st),
		Repositories: newRepositoryService(t, st),
		Archives:     newArchiveService(t, st),
		Auth:         authSvc,
		DocsFS:       fstest.MapFS{},
		TrustProxy:   false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание без хранилища.
	created := createArchive(t, h, owner, `{"name":"ГАВО, архив","notes":[],"private":false}`, http.StatusCreated)
	if created.Name != "ГАВО, архив" || created.RepositoryID != "" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "AR-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id.
	rec := getReq(t, h, "/api/archives/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"name":"ГАВО, архив"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена name/system/repository_id/notes/private.
	rec = putArchiveReq(t, h, owner, "/api/archives/"+string(created.ID),
		`{"name":"ГАВО, архив (испр.)","notes":[],"private":false}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeArchiveS(t, rec)
	if updated.Name != "ГАВО, архив (испр.)" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/archives/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/archives/"+string(created.ID)), http.StatusNotFound)

	// Строгий FK: валидный repository_id создаётся и сохраняется как есть.
	repo := createRepository(t, h, owner,
		`{"name":"ГАВО","type":"archive","address":"","urls":[],"notes":[],"private":false}`, http.StatusCreated)

	withRepo := createArchive(t, h, owner,
		fmt.Sprintf(`{"name":"Фонд 1","repository_id":%q,"notes":[],"private":false}`, repo.ID), http.StatusCreated)
	if withRepo.RepositoryID != string(repo.ID) {
		t.Fatalf("withRepo.RepositoryID = %q, want %q", withRepo.RepositoryID, repo.ID)
	}

	// Строгий FK: корректный по формату, но несуществующий repository_id — 422.
	rec = postArchiveReq(t, h, owner,
		`{"name":"Фонд-призрак","repository_id":"R-01ARZ3NDEKTSV4RRFFQ69G5FA9","notes":[],"private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"repository_id"`) {
		t.Fatalf("body = %s, want field=repository_id", rec.Body)
	}

	// Fix 1: приватная запись, анонимный GET — 404, не 200.
	private := createArchive(t, h, owner, `{"name":"Приватный архив","notes":[],"private":true}`, http.StatusCreated)
	if !private.Private {
		t.Fatalf("private = %+v, want Private=true", private)
	}

	requireStatusS(t, getReq(t, h, "/api/archives/"+string(private.ID)), http.StatusNotFound)
}

func createArchive(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Archive {
	t.Helper()

	rec := postArchiveReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeArchiveS(t, rec)
}

func decodeArchiveS(t *testing.T, rec *httptest.ResponseRecorder) transport.Archive {
	t.Helper()

	var a transport.Archive
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return a
}

func postArchiveReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/archives", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putArchiveReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestNoteWriteContractWithRealStore: сквозной путь «хранилище → сценарии →
// HTTP» (по образцу TestArchiveWriteContractWithRealStore) для заметок:
// bootstrap-регистрация → создание → чтение → изменение → удаление →
// повторное чтение — 404. Дополнительно закрывает self-ref FK
// (Note.ParentID): несуществующий родитель — 422 на поле parent_id;
// переустановка parent_id в цепочку собственных потомков (цикл) — тоже 422
// на поле parent_id; и Fix 1 (приватная запись, анонимный GET — 404).
func TestNoteWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Notes:      newNoteService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание без родителя.
	created := createNote(t, h, owner,
		`{"kind":"note","title":"Заголовок","text":"Текст записи","private":false}`, http.StatusCreated)
	if created.Title != "Заголовок" || created.Text != "Текст записи" || created.ParentID != "" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "N-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id.
	rec := getReq(t, h, "/api/notes/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"text":"Текст записи"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена kind/title/text/parent_id/private.
	rec = putNoteReq(t, h, owner, "/api/notes/"+string(created.ID),
		`{"kind":"article","title":"Заголовок (испр.)","text":"Текст записи","private":false}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeNoteS(t, rec)
	if updated.Kind != "article" || updated.Title != "Заголовок (испр.)" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/notes/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/notes/"+string(created.ID)), http.StatusNotFound)

	// Self-ref FK: валидный parent_id создаётся и сохраняется как есть.
	parent := createNote(t, h, owner, `{"kind":"book","title":"Книга","private":false}`, http.StatusCreated)

	child := createNote(t, h, owner,
		fmt.Sprintf(`{"kind":"chapter","title":"Глава 1","parent_id":%q,"private":false}`, parent.ID), http.StatusCreated)
	if child.ParentID != string(parent.ID) {
		t.Fatalf("child.ParentID = %q, want %q", child.ParentID, parent.ID)
	}

	// Self-ref FK: корректный по формату, но несуществующий parent_id — 422.
	rec = postNoteReq(t, h, owner,
		`{"kind":"note","title":"Сирота","parent_id":"N-01ARZ3NDEKTSV4RRFFQ69G5FA9","private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"parent_id"`) {
		t.Fatalf("body = %s, want field=parent_id", rec.Body)
	}

	// Цикл: A без родителя, B — потомок A, затем A переставляется в потомки B — 422.
	noteA := createNote(t, h, owner, `{"kind":"note","title":"A","private":false}`, http.StatusCreated)
	noteB := createNote(t, h, owner,
		fmt.Sprintf(`{"kind":"note","title":"B","parent_id":%q,"private":false}`, noteA.ID), http.StatusCreated)

	rec = putNoteReq(t, h, owner, "/api/notes/"+string(noteA.ID),
		fmt.Sprintf(`{"kind":"note","title":"A","parent_id":%q,"private":false}`, noteB.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"parent_id"`) {
		t.Fatalf("body = %s, want field=parent_id", rec.Body)
	}

	// Fix 1: приватная запись, анонимный GET — 404, не 200.
	private := createNote(t, h, owner, `{"kind":"note","title":"Приватная","private":true}`, http.StatusCreated)
	if !private.Private {
		t.Fatalf("private = %+v, want Private=true", private)
	}

	requireStatusS(t, getReq(t, h, "/api/notes/"+string(private.ID)), http.StatusNotFound)
}

func createNote(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Note {
	t.Helper()

	rec := postNoteReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeNoteS(t, rec)
}

func decodeNoteS(t *testing.T, rec *httptest.ResponseRecorder) transport.Note {
	t.Helper()

	var n transport.Note
	if err := json.Unmarshal(rec.Body.Bytes(), &n); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return n
}

func postNoteReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/notes", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putNoteReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestAttachmentWriteContractWithRealStore: сквозной путь «хранилище →
// сценарии → HTTP» (по образцу TestArchiveWriteContractWithRealStore) для
// файловых вложений: bootstrap-регистрация → обязательный строгий FK
// node_id (пустой — 422 на поле node_id; корректный по формату, но
// несуществующий — тоже 422 на поле node_id) → Fix 1 (приватная запись,
// анонимный GET — 404). ArchiveNode ещё не имеет своего CRUD-слоя
// (подпроект 6) — сеется напрямую через generic-хранилище вместе с
// Archive, на который он ссылается.
func TestAttachmentWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	ids := idgen.New()
	archiveID := ids.New(models.TypeArchive)
	if err := st.SaveArchive(t.Context(), &models.Archive{ID: archiveID, Name: "ГАВО, архив"}); err != nil {
		t.Fatalf("seed archive: %v", err)
	}

	nodeID := ids.New(models.TypeArchiveNode)
	if err := st.SaveArchiveNode(t.Context(), &models.ArchiveNode{
		ID: nodeID, Type: "fond", ArchiveID: archiveID, Label: "Фонд 1",
	}); err != nil {
		t.Fatalf("seed archive node: %v", err)
	}

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:   newDivisionService(t, st),
		Attachments: newAttachmentService(t, st),
		Auth:        authSvc,
		DocsFS:      fstest.MapFS{},
		TrustProxy:  false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Строгий FK, всегда обязателен: пустой node_id — 422 на поле node_id.
	rec := postAttachmentReq(t, h, owner, `{"kind":"scan","filename":"скан.jpg","private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"node_id"`) {
		t.Fatalf("body = %s, want field=node_id", rec.Body)
	}

	// Строгий FK: корректный по формату, но несуществующий node_id — 422.
	rec = postAttachmentReq(t, h, owner,
		`{"kind":"scan","filename":"скан.jpg","node_id":"AN-01ARZ3NDEKTSV4RRFFQ69G5FA9","private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"node_id"`) {
		t.Fatalf("body = %s, want field=node_id", rec.Body)
	}

	// Fix 1: приватная запись (с настоящим node_id), анонимный GET — 404, не 200.
	private := createAttachment(t, h, owner,
		fmt.Sprintf(`{"kind":"scan","filename":"скан.jpg","node_id":%q,"private":true}`, nodeID), http.StatusCreated)
	if !private.Private {
		t.Fatalf("private = %+v, want Private=true", private)
	}
	if private.NodeID != string(nodeID) {
		t.Fatalf("private.NodeID = %q, want %q", private.NodeID, nodeID)
	}

	requireStatusS(t, getReq(t, h, "/api/attachments/"+string(private.ID)), http.StatusNotFound)
}

func createAttachment(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Attachment {
	t.Helper()

	rec := postAttachmentReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeAttachmentS(t, rec)
}

func decodeAttachmentS(t *testing.T, rec *httptest.ResponseRecorder) transport.Attachment {
	t.Helper()

	var a transport.Attachment
	if err := json.Unmarshal(rec.Body.Bytes(), &a); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return a
}

func postAttachmentReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/attachments", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}
