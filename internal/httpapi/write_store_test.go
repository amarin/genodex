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
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Sources:    newSourceService(t, st),
		Citations:  newCitationService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

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

	// Ретрофит: строгий FK sources[i].citation_id — сквозная проверка через
	// реальный HTTP-хендлер → сценарий → SQLite (по образцу
	// TestArchiveWriteContractWithRealStore; Division — единственная сущность,
	// у которой Sources пришлось добавлять в read/write-контракт с нуля, а не
	// только в usecase, см. docs/data-model/entity-write.md §3.3).
	src := createSource(t, h, owner,
		`{"kind":"document","title":"Ревизская сказка","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	cit := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)

	withCitation := createDivision(t, h, owner,
		fmt.Sprintf(`{"name":"Гавриловское","type":"volost","sources":[{"citation_id":%q}]}`, cit.ID),
		http.StatusCreated)
	if len(withCitation.Sources) != 1 || withCitation.Sources[0].CitationID != string(cit.ID) {
		t.Fatalf("withCitation.Sources = %+v, want [{citation_id: %q}]", withCitation.Sources, cit.ID)
	}

	// Строгий FK: корректный по формату, но несуществующий citation_id — 422 на sources[0].citation_id.
	rec = postReq(t, h, owner,
		`{"name":"Призрачная волость","type":"volost","sources":[{"citation_id":"C-01ARZ3NDEKTSV4RRFFQ69G5FA9"}]}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"sources[0].citation_id"`) {
		t.Fatalf("body = %s, want field=sources[0].citation_id", rec.Body)
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
		Sources:    newSourceService(t, st),
		Citations:  newCitationService(t, st),
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

	// Ретрофит: строгий FK sources[i].citation_id — сквозная проверка через
	// реальный HTTP-хендлер → сценарий → SQLite (по образцу
	// TestArchiveWriteContractWithRealStore). Также закрывает create_church's
	// перевод на транзакционный InTx (docs/data-model/entity-write.md §3.3),
	// который иначе не проверяется ни одним real-store тестом.
	src := createSource(t, h, owner,
		`{"kind":"document","title":"Метрическая книга","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	cit := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)

	withCitation := createChurch(t, h, owner,
		fmt.Sprintf(`{"name":"Покровская церковь","settlements":[],"variants":[],"notes":[],"sources":[{"citation_id":%q}]}`, cit.ID),
		http.StatusCreated)
	if len(withCitation.Sources) != 1 || withCitation.Sources[0].CitationID != string(cit.ID) {
		t.Fatalf("withCitation.Sources = %+v, want [{citation_id: %q}]", withCitation.Sources, cit.ID)
	}

	// Строгий FK: корректный по формату, но несуществующий citation_id — 422 на sources[0].citation_id.
	rec = postChurchReq(t, h, owner,
		`{"name":"Церковь-призрак","settlements":[],"variants":[],"notes":[],"sources":[{"citation_id":"C-01ARZ3NDEKTSV4RRFFQ69G5FA9"}]}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"sources[0].citation_id"`) {
		t.Fatalf("body = %s, want field=sources[0].citation_id", rec.Body)
	}
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
		Sources:      newSourceService(t, st),
		Citations:    newCitationService(t, st),
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

	// Ретрофит: строгий FK sources[i].citation_id — сквозная проверка через
	// реальный HTTP-хендлер → сценарий → SQLite, а не только на fake-store.
	src := createSource(t, h, owner,
		`{"kind":"document","title":"Метрическая книга","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	cit := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)

	withCitation := createArchive(t, h, owner,
		fmt.Sprintf(`{"name":"Фонд с цитатой","sources":[{"citation_id":%q}],"notes":[],"private":false}`, cit.ID),
		http.StatusCreated)
	if len(withCitation.Sources) != 1 || withCitation.Sources[0].CitationID != string(cit.ID) {
		t.Fatalf("withCitation.Sources = %+v, want [{citation_id: %q}]", withCitation.Sources, cit.ID)
	}

	// Строгий FK: корректный по формату, но несуществующий citation_id — 422 на sources[0].citation_id.
	rec = postArchiveReq(t, h, owner,
		`{"name":"Фонд-призрак 2","sources":[{"citation_id":"C-01ARZ3NDEKTSV4RRFFQ69G5FA9"}],"notes":[],"private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"sources[0].citation_id"`) {
		t.Fatalf("body = %s, want field=sources[0].citation_id", rec.Body)
	}
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

// TestArchiveNodeWriteContractWithRealStore: сквозной путь «хранилище →
// сценарии → HTTP» (по образцу TestArchiveWriteContractWithRealStore) для
// узлов архивного дерева: bootstrap-регистрация → создание архива → создание
// корневого узла → создание дочернего узла → список с фильтром по
// archive_id (оба узла видны, корректно вложены) → изменение → удаление →
// повторное чтение — 404. Дополнительно закрывает новый для этого
// подпункта инвариант «родитель из того же архива»: несуществующий
// parent_id — 422 на поле parent_id; parent_id, указывающий на реальный
// узел, но из ДРУГОГО архива — тоже 422 на поле parent_id (не archive_id).
// Также проверяет неизменность archive_id при обновлении: попытка перенести
// существующий узел в другой архив — 422 на поле archive_id.
func TestArchiveNodeWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:    newDivisionService(t, st),
		Archives:     newArchiveService(t, st),
		ArchiveNodes: newArchiveNodeService(t, st),
		Sources:      newSourceService(t, st),
		Citations:    newCitationService(t, st),
		Auth:         authSvc,
		DocsFS:       fstest.MapFS{},
		TrustProxy:   false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	archive := createArchive(t, h, owner, `{"name":"ГАВО, архив","notes":[],"private":false}`, http.StatusCreated)
	otherArchive := createArchive(t, h, owner, `{"name":"РГАДА","notes":[],"private":false}`, http.StatusCreated)

	// Корневой узел (без parent_id).
	root := createArchiveNode(t, h, owner,
		fmt.Sprintf(`{"type":"fond","archive_id":%q,"label":"Фонд 1"}`, archive.ID), http.StatusCreated)
	if root.ArchiveID != archive.ID || root.ParentID != nil {
		t.Fatalf("root = %+v", root)
	}
	if !strings.HasPrefix(string(root.ID), "AN-") {
		t.Fatalf("id = %q", root.ID)
	}

	// Дочерний узел под корневым, в том же архиве.
	child := createArchiveNode(t, h, owner,
		fmt.Sprintf(`{"type":"opis","archive_id":%q,"parent_id":%q,"label":"Опись 1"}`, archive.ID, root.ID), http.StatusCreated)
	if child.ParentID == nil || *child.ParentID != root.ID {
		t.Fatalf("child = %+v", child)
	}

	// Список с фильтром по archive_id, без parent_id — только корень
	// (models.ArchiveNodeQuery: ParentID nil — корень внутри архива).
	rec := getReq(t, h, "/api/archive-nodes?archive_id="+string(archive.ID))
	requireStatusS(t, rec, http.StatusOK)
	rootList := decodeArchiveNodeListS(t, rec)
	if len(rootList) != 1 || rootList[0].ID != root.ID {
		t.Fatalf("rootList = %+v, want exactly [root]", rootList)
	}

	// Список с parent_id=root — только прямые дети (child).
	rec = getReq(t, h, "/api/archive-nodes?archive_id="+string(archive.ID)+"&parent_id="+string(root.ID))
	requireStatusS(t, rec, http.StatusOK)
	childList := decodeArchiveNodeListS(t, rec)
	if len(childList) != 1 || childList[0].ID != child.ID {
		t.Fatalf("childList = %+v, want exactly [child]", childList)
	}

	// Список без archive_id — 400.
	requireStatusS(t, getReq(t, h, "/api/archive-nodes"), http.StatusBadRequest)

	// Изменение: полная замена label.
	rec = putArchiveNodeReq(t, h, owner, "/api/archive-nodes/"+string(root.ID),
		fmt.Sprintf(`{"type":"fond","archive_id":%q,"label":"Фонд 1 (испр.)"}`, archive.ID))
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeArchiveNodeS(t, rec)
	if updated.Label != "Фонд 1 (испр.)" || updated.ID != root.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Несуществующий parent_id — 422 на поле parent_id.
	rec = postArchiveNodeReq(t, h, owner,
		fmt.Sprintf(`{"type":"delo","archive_id":%q,"parent_id":"AN-01ARZ3NDEKTSV4RRFFQ69G5FA9","label":"Дело-призрак"}`, archive.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"parent_id"`) {
		t.Fatalf("body = %s, want field=parent_id", rec.Body)
	}

	// parent_id из ДРУГОГО архива — тоже 422 на поле parent_id (не archive_id):
	// сам родитель существует, инвариант — «родитель из того же архива».
	rec = postArchiveNodeReq(t, h, owner,
		fmt.Sprintf(`{"type":"delo","archive_id":%q,"parent_id":%q,"label":"Дело из чужого архива"}`, otherArchive.ID, root.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"parent_id"`) {
		t.Fatalf("body = %s, want field=parent_id (родитель из другого архива)", rec.Body)
	}

	// Попытка сменить archive_id узла при обновлении — 422 на поле
	// archive_id: принадлежность архиву неизменна после создания, иначе
	// дочерний узел (child) молча пропал бы из обоих деревьев.
	rec = putArchiveNodeReq(t, h, owner, "/api/archive-nodes/"+string(root.ID),
		fmt.Sprintf(`{"type":"fond","archive_id":%q,"label":"Фонд 1 (испр.)"}`, otherArchive.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"archive_id"`) {
		t.Fatalf("body = %s, want field=archive_id (смена archive_id при обновлении)", rec.Body)
	}

	// Ретрофит: строгий FK sources[i].citation_id.
	src := createSource(t, h, owner,
		`{"kind":"document","title":"Опись фонда","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	cit := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)

	withCitation := createArchiveNode(t, h, owner,
		fmt.Sprintf(`{"type":"fond","archive_id":%q,"label":"Фонд с цитатой","sources":[{"citation_id":%q}]}`, archive.ID, cit.ID),
		http.StatusCreated)
	if len(withCitation.Sources) != 1 || withCitation.Sources[0].CitationID != string(cit.ID) {
		t.Fatalf("withCitation.Sources = %+v, want [{citation_id: %q}]", withCitation.Sources, cit.ID)
	}

	rec = postArchiveNodeReq(t, h, owner,
		fmt.Sprintf(`{"type":"fond","archive_id":%q,"label":"Фонд-призрак","sources":[{"citation_id":"C-01ARZ3NDEKTSV4RRFFQ69G5FA9"}]}`, archive.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"sources[0].citation_id"`) {
		t.Fatalf("body = %s, want field=sources[0].citation_id", rec.Body)
	}

	// Удаление корня, занятого дочерним узлом — 409.
	requireStatusS(t, delReq(t, h, owner, "/api/archive-nodes/"+string(root.ID)), http.StatusConflict)

	// Удаление дочернего узла, затем корня — оба 204.
	requireStatusS(t, delReq(t, h, owner, "/api/archive-nodes/"+string(child.ID)), http.StatusNoContent)
	requireStatusS(t, delReq(t, h, owner, "/api/archive-nodes/"+string(root.ID)), http.StatusNoContent)

	requireStatusS(t, getReq(t, h, "/api/archive-nodes/"+string(root.ID)), http.StatusNotFound)
}

func createArchiveNode(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.ArchiveNode {
	t.Helper()

	rec := postArchiveNodeReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeArchiveNodeS(t, rec)
}

func decodeArchiveNodeS(t *testing.T, rec *httptest.ResponseRecorder) transport.ArchiveNode {
	t.Helper()

	var n transport.ArchiveNode
	if err := json.Unmarshal(rec.Body.Bytes(), &n); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return n
}

func decodeArchiveNodeListS(t *testing.T, rec *httptest.ResponseRecorder) []transport.ArchiveNode {
	t.Helper()

	var list []transport.ArchiveNode
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return list
}

func postArchiveNodeReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/archive-nodes", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putArchiveNodeReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestArchiveDocumentWriteContractWithRealStore: сквозной путь «хранилище →
// сценарии → HTTP» для документов внутри единиц учёта: bootstrap-регистрация
// → создание архива → создание узла → создание документа под этим узлом →
// чтение → изменение → удаление → повторное чтение — 404. Дополнительно
// закрывает строгий FK unit_id (несуществующий — 422 на поле unit_id) и
// sources[i].citation_id (несуществующий — 422).
func TestArchiveDocumentWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:    newDivisionService(t, st),
		Archives:     newArchiveService(t, st),
		ArchiveNodes: newArchiveNodeService(t, st),
		ArchiveDocs:  newArchiveDocumentService(t, st),
		Sources:      newSourceService(t, st),
		Citations:    newCitationService(t, st),
		Auth:         authSvc,
		DocsFS:       fstest.MapFS{},
		TrustProxy:   false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	archive := createArchive(t, h, owner, `{"name":"ГАВО, архив","notes":[],"private":false}`, http.StatusCreated)
	unit := createArchiveNode(t, h, owner,
		fmt.Sprintf(`{"type":"delo","archive_id":%q,"label":"Дело 1"}`, archive.ID), http.StatusCreated)

	created := createArchiveDocument(t, h, owner,
		fmt.Sprintf(`{"unit_id":%q,"title":"Метрическая книга 1890"}`, unit.ID), http.StatusCreated)
	if created.UnitID != unit.ID || created.Title != "Метрическая книга 1890" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "DC-") {
		t.Fatalf("id = %q", created.ID)
	}

	rec := getReq(t, h, "/api/archive-documents/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"title":"Метрическая книга 1890"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	rec = putArchiveDocumentReq(t, h, owner, "/api/archive-documents/"+string(created.ID),
		fmt.Sprintf(`{"unit_id":%q,"title":"Метрическая книга 1890 (испр.)"}`, unit.ID))
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeArchiveDocumentS(t, rec)
	if updated.Title != "Метрическая книга 1890 (испр.)" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	requireStatusS(t, delReq(t, h, owner, "/api/archive-documents/"+string(created.ID)), http.StatusNoContent)
	requireStatusS(t, getReq(t, h, "/api/archive-documents/"+string(created.ID)), http.StatusNotFound)

	// Строгий FK: несуществующий unit_id — 422 на поле unit_id.
	rec = postArchiveDocumentReq(t, h, owner, `{"unit_id":"AN-01ARZ3NDEKTSV4RRFFQ69G5FA9","title":"Документ-призрак"}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"unit_id"`) {
		t.Fatalf("body = %s, want field=unit_id", rec.Body)
	}

	// Ретрофит: строгий FK sources[i].citation_id.
	src := createSource(t, h, owner,
		`{"kind":"document","title":"Метрическая книга","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	cit := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)

	withCitation := createArchiveDocument(t, h, owner,
		fmt.Sprintf(`{"unit_id":%q,"title":"Документ с цитатой","sources":[{"citation_id":%q}]}`, unit.ID, cit.ID),
		http.StatusCreated)
	if len(withCitation.Sources) != 1 || withCitation.Sources[0].CitationID != string(cit.ID) {
		t.Fatalf("withCitation.Sources = %+v, want [{citation_id: %q}]", withCitation.Sources, cit.ID)
	}

	rec = postArchiveDocumentReq(t, h, owner,
		fmt.Sprintf(`{"unit_id":%q,"title":"Документ-призрак 2","sources":[{"citation_id":"C-01ARZ3NDEKTSV4RRFFQ69G5FA9"}]}`, unit.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"sources[0].citation_id"`) {
		t.Fatalf("body = %s, want field=sources[0].citation_id", rec.Body)
	}
}

func createArchiveDocument(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.ArchiveDocument {
	t.Helper()

	rec := postArchiveDocumentReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeArchiveDocumentS(t, rec)
}

func decodeArchiveDocumentS(t *testing.T, rec *httptest.ResponseRecorder) transport.ArchiveDocument {
	t.Helper()

	var d transport.ArchiveDocument
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return d
}

func postArchiveDocumentReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/archive-documents", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putArchiveDocumentReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestSourceWriteContractWithRealStore: сквозной путь «хранилище → сценарии
// → HTTP» (по образцу TestArchiveWriteContractWithRealStore) для источников
// доказательств: bootstrap-регистрация → создание без хранилища → чтение →
// изменение → удаление → повторное чтение — 404. Дополнительно: строгий FK
// на Repository (валидный repository_id сохраняется; несуществующий, но
// корректный по формату — 422 на поле repository_id) и Fix 1 (CRITICAL):
// приватная запись, анонимный GET — 404, не 200.
func TestSourceWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:    newDivisionService(t, st),
		Repositories: newRepositoryService(t, st),
		Sources:      newSourceService(t, st),
		Auth:         authSvc,
		DocsFS:       fstest.MapFS{},
		TrustProxy:   false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание без хранилища.
	created := createSource(t, h, owner,
		`{"kind":"document","title":"Метрическая книга","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	if created.Title != "Метрическая книга" || created.RepositoryID != "" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "S-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id.
	rec := getReq(t, h, "/api/sources/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"title":"Метрическая книга"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена kind/title/author/date/reliability/repository_id/notes/private.
	rec = putSourceReq(t, h, owner, "/api/sources/"+string(created.ID),
		`{"kind":"transcription","title":"Метрическая книга (испр.)","reliability":"contemporary","notes":[],"private":false}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeSourceS(t, rec)
	if updated.Title != "Метрическая книга (испр.)" || updated.Kind != "transcription" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/sources/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/sources/"+string(created.ID)), http.StatusNotFound)

	// Строгий FK: валидный repository_id создаётся и сохраняется как есть.
	repo := createRepository(t, h, owner,
		`{"name":"ГАВО","type":"archive","address":"","urls":[],"notes":[],"private":false}`, http.StatusCreated)

	withRepo := createSource(t, h, owner,
		fmt.Sprintf(`{"kind":"document","title":"Дело 1","reliability":"primary","repository_id":%q,"notes":[],"private":false}`, repo.ID),
		http.StatusCreated)
	if withRepo.RepositoryID != string(repo.ID) {
		t.Fatalf("withRepo.RepositoryID = %q, want %q", withRepo.RepositoryID, repo.ID)
	}

	// Строгий FK: корректный по формату, но несуществующий repository_id — 422.
	rec = postSourceReq(t, h, owner,
		`{"kind":"document","title":"Дело-призрак","reliability":"primary","repository_id":"R-01ARZ3NDEKTSV4RRFFQ69G5FA9","notes":[],"private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"repository_id"`) {
		t.Fatalf("body = %s, want field=repository_id", rec.Body)
	}

	// Fix 1: приватная запись, анонимный GET — 404, не 200.
	private := createSource(t, h, owner,
		`{"kind":"memory","title":"Частные воспоминания","reliability":"memory","notes":[],"private":true}`,
		http.StatusCreated)
	if !private.Private {
		t.Fatalf("private = %+v, want Private=true", private)
	}

	requireStatusS(t, getReq(t, h, "/api/sources/"+string(private.ID)), http.StatusNotFound)
}

func createSource(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Source {
	t.Helper()

	rec := postSourceReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeSourceS(t, rec)
}

func decodeSourceS(t *testing.T, rec *httptest.ResponseRecorder) transport.Source {
	t.Helper()

	var s transport.Source
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return s
}

func postSourceReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/sources", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putSourceReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestCitationWriteContractWithRealStore: сквозной путь «хранилище →
// сценарии → HTTP» (по образцу TestArchiveWriteContractWithRealStore) для
// цитат: bootstrap-регистрация → создание источника → создание цитаты без
// якоря → чтение → изменение → удаление → повторное чтение — 404.
// Дополнительно: строгий FK на Source (SourceID всегда обязателен, в
// отличие от Archive.RepositoryID — несуществующий, но корректный по формату
// — 422 на поле source_id), круговорот якоря (ArchiveAnchor — ссылка на
// несуществующий ArchiveNode — 422 на поле anchor.node_id; URLAnchor —
// круговорот без ссылок) и Fix 1 (CRITICAL): приватная запись, анонимный
// GET — 404, не 200.
func TestCitationWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Sources:    newSourceService(t, st),
		Citations:  newCitationService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	src := createSource(t, h, owner,
		`{"kind":"document","title":"Метрическая книга","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)

	// Создание без якоря.
	created := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)
	if created.SourceID != string(src.ID) || created.Anchor != nil {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "C-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id.
	rec := getReq(t, h, "/api/citations/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), fmt.Sprintf(`"source_id":%q`, src.ID)) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена source_id/anchor/text/note/private, всё ещё без якоря.
	rec = putCitationReq(t, h, owner, "/api/citations/"+string(created.ID),
		fmt.Sprintf(`{"source_id":%q,"text":"л. 12 об.","private":false}`, src.ID))
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeCitationS(t, rec)
	if updated.Text != "л. 12 об." || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Строгий FK: корректный по формату, но несуществующий source_id — 422.
	rec = postCitationReq(t, h, owner,
		`{"source_id":"S-01ARZ3NDEKTSV4RRFFQ69G5FA9","private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"source_id"`) {
		t.Fatalf("body = %s, want field=source_id", rec.Body)
	}

	// Круговорот якоря: URLAnchor без ссылок на другие сущности.
	withAnchor := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"anchor":{"kind":"url","url":"https://example.org/page"},"private":false}`, src.ID),
		http.StatusCreated)
	if withAnchor.Anchor == nil || withAnchor.Anchor.Kind != "url" || withAnchor.Anchor.URL != "https://example.org/page" {
		t.Fatalf("withAnchor.Anchor = %+v", withAnchor.Anchor)
	}

	// Якорь со ссылкой на несуществующий ArchiveNode — 422 на поле anchor.node_id.
	rec = postCitationReq(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"anchor":{"kind":"archive","node_id":"AN-01ARZ3NDEKTSV4RRFFQ69G5FA9","page":1},"private":false}`, src.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"anchor.node_id"`) {
		t.Fatalf("body = %s, want field=anchor.node_id", rec.Body)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/citations/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/citations/"+string(created.ID)), http.StatusNotFound)

	// Fix 1: приватная запись, анонимный GET — 404, не 200.
	private := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":true}`, src.ID), http.StatusCreated)
	if !private.Private {
		t.Fatalf("private = %+v, want Private=true", private)
	}

	requireStatusS(t, getReq(t, h, "/api/citations/"+string(private.ID)), http.StatusNotFound)
}

func createCitation(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Citation {
	t.Helper()

	rec := postCitationReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeCitationS(t, rec)
}

func decodeCitationS(t *testing.T, rec *httptest.ResponseRecorder) transport.Citation {
	t.Helper()

	var c transport.Citation
	if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return c
}

func postCitationReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/citations", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putCitationReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestFamilyWriteContractWithRealStore: сквозной путь «хранилище → сценарии
// → HTTP» (по образцу TestRepositoryWriteContractWithRealStore, структурно
// ближайшего шаблона подпроекта 7 — Family отличается от Repository
// отсутствием Type/Address и переименованием URLs → Members) для родов:
// bootstrap-регистрация → создание → чтение → изменение → удаление →
// повторное чтение — 404. Дополнительно закрывает: приватность (Fix 1),
// проверяемая явно через ОБА пути (create И update — этот дефект уже
// повторялся дважды в программе, подпроекты 2 и 4); строгий FK
// sources[i].citation_id; и то, что members — мягкая ссылка на Person (без
// CRUD, подпроект 8) — ref/type round-trip'ятся как обычный TextRef, без
// проверки существования.
func TestFamilyWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		Families:   newFamilyService(t, st),
		Sources:    newSourceService(t, st),
		Citations:  newCitationService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание.
	created := createFamily(t, h, owner,
		`{"name":"Ивановы","members":[],"notes":[],"private":false}`, http.StatusCreated)
	if created.Name != "Ивановы" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "F-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id — открыто анонимному посетителю.
	rec := getReq(t, h, "/api/families/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"name":"Ивановы"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена name/members/notes/sources/private.
	rec = putFamilyReq(t, h, owner, "/api/families/"+string(created.ID),
		`{"name":"Ивановы (испр.)","members":[],"notes":[],"private":false}`)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeFamilyS(t, rec)
	if updated.Name != "Ивановы (испр.)" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/families/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/families/"+string(created.ID)), http.StatusNotFound)

	// Fix 1 (CRITICAL, повторялась дважды — подпроекты 2 и 4): private
	// должен пережить и create, и update, а не только прямое сохранение.
	// Сначала create с private:true.
	private := createFamily(t, h, owner,
		`{"name":"Приватный род","members":[],"notes":[],"private":true}`, http.StatusCreated)
	if !private.Private {
		t.Fatalf("private (после create) = %+v, want Private=true", private)
	}
	requireStatusS(t, getReq(t, h, "/api/families/"+string(private.ID)), http.StatusNotFound)

	// Теперь update, переключающий PUBLIC → PRIVATE (не private → private —
	// та проверка ловит только "update сбрасывает private в false", а не
	// "update не выставляет private в true у публичной записи"; финальный
	// ревью подпроекта 7 нашёл, что прежняя версия этого шага создавала
	// запись сразу приватной, так что реального переключения не проверяла).
	public := createFamily(t, h, owner,
		`{"name":"Публичный род","members":[],"notes":[],"private":false}`, http.StatusCreated)
	requireStatusS(t, getReq(t, h, "/api/families/"+string(public.ID)), http.StatusOK)

	rec = putFamilyReq(t, h, owner, "/api/families/"+string(public.ID),
		`{"name":"Публичный род","members":[],"notes":[],"private":true}`)
	requireStatusS(t, rec, http.StatusOK)
	afterUpdate := decodeFamilyS(t, rec)
	if !afterUpdate.Private {
		t.Fatalf("private (после update) = %+v, want Private=true", afterUpdate)
	}
	requireStatusS(t, getReq(t, h, "/api/families/"+string(public.ID)), http.StatusNotFound)

	// members — мягкая ссылка на Person (без CRUD, подпроект 8):
	// ref/type сохраняются как обычный TextRef, существование не проверяется.
	// Проверяем и ответ create, и то, что ref/type реально пережили запись и
	// чтение из SQLite (а не только эхо сценария в памяти).
	withMember := createFamily(t, h, owner,
		`{"name":"Петровы","members":[{"text":"Пётр Петров","ref":"I-01ARZ3NDEKTSV4RRFFQ69G5FA9","type":"person"}],"notes":[],"private":false}`,
		http.StatusCreated)
	if len(withMember.Members) != 1 || withMember.Members[0].Ref != "I-01ARZ3NDEKTSV4RRFFQ69G5FA9" {
		t.Fatalf("withMember.Members (ответ create) = %+v", withMember.Members)
	}
	reread := getReq(t, h, "/api/families/"+string(withMember.ID))
	requireStatusS(t, reread, http.StatusOK)
	rereadFamily := decodeFamilyS(t, reread)
	if len(rereadFamily.Members) != 1 || rereadFamily.Members[0].Ref != "I-01ARZ3NDEKTSV4RRFFQ69G5FA9" ||
		rereadFamily.Members[0].Type != "person" {
		t.Fatalf("withMember.Members (после чтения из SQLite) = %+v", rereadFamily.Members)
	}

	// Ретрофит-паттерн (по образцу TestArchiveWriteContractWithRealStore):
	// строгий FK sources[i].citation_id — сквозная проверка через реальный
	// HTTP-хендлер → сценарий → SQLite.
	src := createSource(t, h, owner,
		`{"kind":"document","title":"Ревизская сказка","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	cit := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)

	withCitation := createFamily(t, h, owner,
		fmt.Sprintf(`{"name":"Сидоровы","members":[],"notes":[],"sources":[{"citation_id":%q}],"private":false}`, cit.ID),
		http.StatusCreated)
	if len(withCitation.Sources) != 1 || withCitation.Sources[0].CitationID != string(cit.ID) {
		t.Fatalf("withCitation.Sources = %+v, want [{citation_id: %q}]", withCitation.Sources, cit.ID)
	}

	// Строгий FK: корректный по формату, но несуществующий citation_id — 422 на sources[0].citation_id.
	rec = postFamilyReq(t, h, owner,
		`{"name":"Призрачный род","members":[],"notes":[],"sources":[{"citation_id":"C-01ARZ3NDEKTSV4RRFFQ69G5FA9"}],"private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"sources[0].citation_id"`) {
		t.Fatalf("body = %s, want field=sources[0].citation_id", rec.Body)
	}
}

func createFamily(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Family {
	t.Helper()

	rec := postFamilyReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeFamilyS(t, rec)
}

func decodeFamilyS(t *testing.T, rec *httptest.ResponseRecorder) transport.Family {
	t.Helper()

	var f transport.Family
	if err := json.Unmarshal(rec.Body.Bytes(), &f); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return f
}

func postFamilyReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/families", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putFamilyReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestPersonWriteContractWithRealStore: сквозной путь «хранилище → сценарии
// → HTTP» (по образцу TestFamilyWriteContractWithRealStore) для персон —
// ядра графа генеалогии (подпроект 8): bootstrap-регистрация → создание с
// двумя записями Names (main и married), каждая — с мягкой ссылкой на
// заведомо несуществующие Surname/GivenName/Patronymic (доказывает 201, без
// проверки существования) → чтение → изменение → удаление → повторное
// чтение — 404. Дополнительно закрывает: приватность через ОБА пути (create
// И update, PUBLIC → PRIVATE, не private → private — тот же урок финального
// ревью подпроекта 7); строгий FK sources[i].citation_id; и то, что поиск
// находит персону по части фамилии/имени/отчества из ВТОРОЙ записи Names
// (married), а не только из первой (main) — доказывает, что personTerms
// индексирует все имена, не только основное.
func TestPersonWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		People:     newPersonService(t, st),
		Sources:    newSourceService(t, st),
		Citations:  newCitationService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	// Создание: два имени (main + married), каждое — с мягкой ссылкой на
	// несуществующие словарные записи. 201, без проверки существования.
	createBody := `{
		"gender": "female",
		"names": [
			{"type":"main","surname":{"text":"Дорожкина","ref":"SN-01ARZ3NDEKTSV4RRFFQ69G5FA1","type":"surname"},"given":{"text":"Акилина","ref":"GN-01ARZ3NDEKTSV4RRFFQ69G5FA1","type":"given_name"}},
			{"type":"married","surname":{"text":"Петрова","ref":"SN-01ARZ3NDEKTSV4RRFFQ69G5FA2","type":"surname"},"patronymic":{"text":"Ивановна","ref":"PN-01ARZ3NDEKTSV4RRFFQ69G5FA1","type":"patronymic"}}
		],
		"estates": [{"text":"крестьяне"}],
		"titles": [],
		"nicknames": [],
		"notes": [],
		"private": false
	}`
	created := createPerson(t, h, owner, createBody, http.StatusCreated)
	if len(created.Names) != 2 || created.Names[0].Surname.Text != "Дорожкина" || created.Names[1].Surname.Text != "Петрова" {
		t.Fatalf("created.Names = %+v", created.Names)
	}
	if created.Names[0].Surname.Ref != "SN-01ARZ3NDEKTSV4RRFFQ69G5FA1" || created.Names[1].Patronymic.Ref != "PN-01ARZ3NDEKTSV4RRFFQ69G5FA1" {
		t.Fatalf("мягкие ссылки не сохранены как есть: %+v", created.Names)
	}
	if !strings.HasPrefix(string(created.ID), "I-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id — открыто анонимному посетителю; ссылки пережили запись и
	// чтение из SQLite (а не только эхо сценария в памяти).
	rec := getReq(t, h, "/api/people/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	reread := decodePersonS(t, rec)
	if len(reread.Names) != 2 || reread.Names[1].Surname.Ref != "SN-01ARZ3NDEKTSV4RRFFQ69G5FA2" {
		t.Fatalf("reread.Names (после чтения из SQLite) = %+v", reread.Names)
	}

	// Поиск: часть фамилии, которая встречается ТОЛЬКО во второй (married)
	// записи Names, должна находить персону — доказывает, что personTerms
	// индексирует все имена, а не только первое/основное.
	searchRec := getReq(t, h, "/api/people/search?q=Петр")
	requireStatusS(t, searchRec, http.StatusOK)
	if !strings.Contains(searchRec.Body.String(), string(created.ID)) {
		t.Fatalf("поиск по началу фамилии из married-имени не нашёл запись: %s", searchRec.Body)
	}

	// Изменение: полная замена; gender меняется, private остаётся false.
	updateBody := `{
		"gender": "female",
		"names": [
			{"type":"main","surname":{"text":"Дорожкина"},"given":{"text":"Акилина"}}
		],
		"estates": [],
		"titles": [],
		"nicknames": [],
		"notes": [],
		"private": false
	}`
	rec = putPersonReq(t, h, owner, "/api/people/"+string(created.ID), updateBody)
	requireStatusS(t, rec, http.StatusOK)
	updated := decodePersonS(t, rec)
	if len(updated.Names) != 1 || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/people/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/people/"+string(created.ID)), http.StatusNotFound)

	// Fix 1 (CRITICAL, повторялась в программе несколько раз): private
	// должен пережить и create, и update, а не только прямое сохранение.
	// Сначала create с private:true.
	private := createPerson(t, h, owner, `{"names":[],"estates":[],"titles":[],"nicknames":[],"notes":[],"private":true}`, http.StatusCreated)
	if !private.Private {
		t.Fatalf("private (после create) = %+v, want Private=true", private)
	}
	requireStatusS(t, getReq(t, h, "/api/people/"+string(private.ID)), http.StatusNotFound)

	// Теперь update, переключающий PUBLIC → PRIVATE (не private → private —
	// та проверка ловит только "update сбрасывает private в false", а не
	// "update не выставляет private в true у публичной записи"; тот же
	// урок, что и финальное ревью подпроекта 7).
	public := createPerson(t, h, owner,
		`{"names":[{"type":"main","surname":{"text":"Тайнова"},"given":{"text":"Анна"}}],"estates":[],"titles":[],"nicknames":[],"notes":[],"private":false}`,
		http.StatusCreated)
	requireStatusS(t, getReq(t, h, "/api/people/"+string(public.ID)), http.StatusOK)

	rec = putPersonReq(t, h, owner, "/api/people/"+string(public.ID),
		`{"names":[{"type":"main","surname":{"text":"Тайнова"},"given":{"text":"Анна"}}],"estates":[],"titles":[],"nicknames":[],"notes":[],"private":true}`)
	requireStatusS(t, rec, http.StatusOK)
	afterUpdate := decodePersonS(t, rec)
	if !afterUpdate.Private {
		t.Fatalf("private (после update) = %+v, want Private=true", afterUpdate)
	}
	requireStatusS(t, getReq(t, h, "/api/people/"+string(public.ID)), http.StatusNotFound)

	// Приватность работает не только на прямом GET по id — те же правила
	// доступа обязаны отфильтровывать приватную запись и в списке
	// (listByIDs), и в поиске (searchSQL): это отдельные от Get пути,
	// которые теоретически могли бы разойтись.
	listRec := getReq(t, h, "/api/people")
	requireStatusS(t, listRec, http.StatusOK)
	if strings.Contains(listRec.Body.String(), string(public.ID)) {
		t.Fatalf("анонимный список /api/people содержит приватную персону: %s", listRec.Body)
	}
	searchPrivateRec := getReq(t, h, "/api/people/search?q=Тайнова")
	requireStatusS(t, searchPrivateRec, http.StatusOK)
	if strings.Contains(searchPrivateRec.Body.String(), string(public.ID)) {
		t.Fatalf("анонимный поиск /api/people/search содержит приватную персону: %s", searchPrivateRec.Body)
	}

	// Строгий FK: существующая цитата — сохраняется.
	src := createSource(t, h, owner,
		`{"kind":"document","title":"Ревизская сказка","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	cit := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)

	withCitation := createPerson(t, h, owner,
		fmt.Sprintf(`{"names":[],"estates":[],"titles":[],"nicknames":[],"notes":[],"sources":[{"citation_id":%q}],"private":false}`, cit.ID),
		http.StatusCreated)
	if len(withCitation.Sources) != 1 || withCitation.Sources[0].CitationID != string(cit.ID) {
		t.Fatalf("withCitation.Sources = %+v, want [{citation_id: %q}]", withCitation.Sources, cit.ID)
	}

	// Строгий FK: корректный по формату, но несуществующий citation_id — 422 на sources[0].citation_id.
	rec = postPersonReq(t, h, owner,
		`{"names":[],"estates":[],"titles":[],"nicknames":[],"notes":[],"sources":[{"citation_id":"C-01ARZ3NDEKTSV4RRFFQ69G5FA9"}],"private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"sources[0].citation_id"`) {
		t.Fatalf("body = %s, want field=sources[0].citation_id", rec.Body)
	}
}

func createPerson(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Person {
	t.Helper()

	rec := postPersonReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodePersonS(t, rec)
}

func decodePersonS(t *testing.T, rec *httptest.ResponseRecorder) transport.Person {
	t.Helper()

	var p transport.Person
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return p
}

func postPersonReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/people", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putPersonReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestRelationWriteContractWithRealStore: сквозной путь «хранилище →
// сценарии → HTTP» (по образцу TestFamilyWriteContractWithRealStore) для
// рёбер графа родства — первой сущности программы с двумя строгими ссылками
// на один и тот же тип (person_a/person_b → Person, docs/data-model/
// entity-write.md §3.8). Покрывает: create с двумя валидными person_a/
// person_b → чтение → изменение → удаление → 404; 422 на несуществующий
// person_a; 422 на несуществующий person_b (обе стороны проверяются
// независимо); строгий FK sources[0].citation_id; и генуинный
// PUBLIC → PRIVATE update-флип (не private → private).
func TestRelationWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		People:     newPersonService(t, st),
		Relations:  newRelationService(t, st),
		Sources:    newSourceService(t, st),
		Citations:  newCitationService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	personA := createPerson(t, h, owner, `{"names":[{"type":"main","surname":{"text":"Иванов"},"given":{"text":"Иван"}}],"private":false}`, http.StatusCreated)
	personB := createPerson(t, h, owner, `{"names":[{"type":"main","surname":{"text":"Иванова"},"given":{"text":"Мария"}}],"private":false}`, http.StatusCreated)

	// Создание.
	created := createRelation(t, h, owner,
		fmt.Sprintf(`{"kind":"blood","person_a":%q,"person_b":%q,"notes":[],"private":false}`, personA.ID, personB.ID),
		http.StatusCreated)
	if created.Kind != "blood" || created.PersonA != personA.ID || created.PersonB != personB.ID {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "RL-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id — открыто анонимному посетителю.
	rec := getReq(t, h, "/api/relations/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"kind":"blood"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение: полная замена kind/person_a/person_b/sources/notes/private.
	rec = putRelationReq(t, h, owner, "/api/relations/"+string(created.ID),
		fmt.Sprintf(`{"kind":"marriage","person_a":%q,"person_b":%q,"notes":[],"private":false}`, personA.ID, personB.ID))
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeRelationS(t, rec)
	if updated.Kind != "marriage" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/relations/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/relations/"+string(created.ID)), http.StatusNotFound)

	// Строгий FK: несуществующий person_a — 422 на поле person_a.
	rec = postRelationReq(t, h, owner,
		fmt.Sprintf(`{"kind":"blood","person_a":"I-01ARZ3NDEKTSV4RRFFQ69G5FA9","person_b":%q,"notes":[],"private":false}`, personB.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"person_a"`) {
		t.Fatalf("body = %s, want field=person_a", rec.Body)
	}

	// Строгий FK: person_a существует, person_b — нет — 422 на поле person_b
	// (обе стороны проверяются независимо).
	rec = postRelationReq(t, h, owner,
		fmt.Sprintf(`{"kind":"blood","person_a":%q,"person_b":"I-01ARZ3NDEKTSV4RRFFQ69G5FA9","notes":[],"private":false}`, personA.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"person_b"`) {
		t.Fatalf("body = %s, want field=person_b", rec.Body)
	}

	// Строгий FK: sources[0].citation_id несуществующий — 422.
	rec = postRelationReq(t, h, owner,
		fmt.Sprintf(`{"kind":"blood","person_a":%q,"person_b":%q,"sources":[{"citation_id":"C-01ARZ3NDEKTSV4RRFFQ69G5FA9"}],"notes":[],"private":false}`, personA.ID, personB.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"sources[0].citation_id"`) {
		t.Fatalf("body = %s, want field=sources[0].citation_id", rec.Body)
	}

	// Строгий FK: существующая цитата — сохраняется.
	src := createSource(t, h, owner,
		`{"kind":"document","title":"Метрическая книга","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	cit := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)

	withCitation := createRelation(t, h, owner,
		fmt.Sprintf(`{"kind":"blood","person_a":%q,"person_b":%q,"sources":[{"citation_id":%q}],"notes":[],"private":false}`, personA.ID, personB.ID, cit.ID),
		http.StatusCreated)
	if len(withCitation.Sources) != 1 || withCitation.Sources[0].CitationID != string(cit.ID) {
		t.Fatalf("withCitation.Sources = %+v, want [{citation_id: %q}]", withCitation.Sources, cit.ID)
	}

	// PUBLIC → PRIVATE update-флип (не private → private).
	public := createRelation(t, h, owner,
		fmt.Sprintf(`{"kind":"blood","person_a":%q,"person_b":%q,"notes":[],"private":false}`, personA.ID, personB.ID),
		http.StatusCreated)
	requireStatusS(t, getReq(t, h, "/api/relations/"+string(public.ID)), http.StatusOK)

	rec = putRelationReq(t, h, owner, "/api/relations/"+string(public.ID),
		fmt.Sprintf(`{"kind":"blood","person_a":%q,"person_b":%q,"notes":[],"private":true}`, personA.ID, personB.ID))
	requireStatusS(t, rec, http.StatusOK)
	afterUpdate := decodeRelationS(t, rec)
	if !afterUpdate.Private {
		t.Fatalf("private (после update) = %+v, want Private=true", afterUpdate)
	}
	requireStatusS(t, getReq(t, h, "/api/relations/"+string(public.ID)), http.StatusNotFound)

	// person_id-фильтр списка: совпадает с person_a ИЛИ person_b.
	listRec := getReq(t, h, "/api/relations?person_id="+string(personA.ID))
	requireStatusS(t, listRec, http.StatusOK)
	if !strings.Contains(listRec.Body.String(), string(withCitation.ID)) {
		t.Fatalf("список по person_id=%q не содержит ребро %q: %s", personA.ID, withCitation.ID, listRec.Body)
	}
}

func createRelation(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Relation {
	t.Helper()

	rec := postRelationReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeRelationS(t, rec)
}

func decodeRelationS(t *testing.T, rec *httptest.ResponseRecorder) transport.Relation {
	t.Helper()

	var r transport.Relation
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return r
}

func postRelationReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/relations", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putRelationReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestResidenceWriteContractWithRealStore: сквозной путь «хранилище →
// сценарии → HTTP» для проживаний. Покрывает: create с валидными person_id/
// place_id → чтение → изменение → удаление → 404; 422 на несуществующий
// person_id; 422 на несуществующий place_id; строгий FK
// sources[0].citation_id; PUBLIC → PRIVATE update-флип.
func TestResidenceWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		People:     newPersonService(t, st),
		Residences: newResidenceService(t, st),
		Sources:    newSourceService(t, st),
		Citations:  newCitationService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	person := createPerson(t, h, owner, `{"names":[{"type":"main","surname":{"text":"Сидоров"},"given":{"text":"Пётр"}}],"private":false}`, http.StatusCreated)
	place := createDivision(t, h, owner, `{"name":"Давыдово","type":"selo"}`, http.StatusCreated)

	// Создание.
	created := createResidence(t, h, owner,
		fmt.Sprintf(`{"person_id":%q,"place_id":%q,"note":"изба","private":false}`, person.ID, place.ID),
		http.StatusCreated)
	if created.PersonID != person.ID || created.PlaceID != place.ID || created.Note != "изба" {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "RS-") {
		t.Fatalf("id = %q", created.ID)
	}

	// Чтение по id.
	rec := getReq(t, h, "/api/residences/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	if !strings.Contains(rec.Body.String(), `"note":"изба"`) {
		t.Fatalf("body = %s", rec.Body)
	}

	// Изменение.
	rec = putResidenceReq(t, h, owner, "/api/residences/"+string(created.ID),
		fmt.Sprintf(`{"person_id":%q,"place_id":%q,"note":"новая изба","private":false}`, person.ID, place.ID))
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeResidenceS(t, rec)
	if updated.Note != "новая изба" || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/residences/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/residences/"+string(created.ID)), http.StatusNotFound)

	// Строгий FK: несуществующий person_id — 422 на поле person_id.
	rec = postResidenceReq(t, h, owner,
		fmt.Sprintf(`{"person_id":"I-01ARZ3NDEKTSV4RRFFQ69G5FA9","place_id":%q,"private":false}`, place.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"person_id"`) {
		t.Fatalf("body = %s, want field=person_id", rec.Body)
	}

	// Строгий FK: несуществующий place_id — 422 на поле place_id.
	rec = postResidenceReq(t, h, owner,
		fmt.Sprintf(`{"person_id":%q,"place_id":"AD-01ARZ3NDEKTSV4RRFFQ69G5FA9","private":false}`, person.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"place_id"`) {
		t.Fatalf("body = %s, want field=place_id", rec.Body)
	}

	// Строгий FK: sources[0].citation_id несуществующий — 422.
	rec = postResidenceReq(t, h, owner,
		fmt.Sprintf(`{"person_id":%q,"place_id":%q,"sources":[{"citation_id":"C-01ARZ3NDEKTSV4RRFFQ69G5FA9"}],"private":false}`, person.ID, place.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"sources[0].citation_id"`) {
		t.Fatalf("body = %s, want field=sources[0].citation_id", rec.Body)
	}

	// Строгий FK: существующая цитата — сохраняется.
	src := createSource(t, h, owner,
		`{"kind":"document","title":"Ревизская сказка","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	cit := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)

	withCitation := createResidence(t, h, owner,
		fmt.Sprintf(`{"person_id":%q,"place_id":%q,"sources":[{"citation_id":%q}],"private":false}`, person.ID, place.ID, cit.ID),
		http.StatusCreated)
	if len(withCitation.Sources) != 1 || withCitation.Sources[0].CitationID != string(cit.ID) {
		t.Fatalf("withCitation.Sources = %+v, want [{citation_id: %q}]", withCitation.Sources, cit.ID)
	}

	// PUBLIC → PRIVATE update-флип (не private → private).
	public := createResidence(t, h, owner,
		fmt.Sprintf(`{"person_id":%q,"place_id":%q,"private":false}`, person.ID, place.ID), http.StatusCreated)
	requireStatusS(t, getReq(t, h, "/api/residences/"+string(public.ID)), http.StatusOK)

	rec = putResidenceReq(t, h, owner, "/api/residences/"+string(public.ID),
		fmt.Sprintf(`{"person_id":%q,"place_id":%q,"private":true}`, person.ID, place.ID))
	requireStatusS(t, rec, http.StatusOK)
	afterUpdate := decodeResidenceS(t, rec)
	if !afterUpdate.Private {
		t.Fatalf("private (после update) = %+v, want Private=true", afterUpdate)
	}
	requireStatusS(t, getReq(t, h, "/api/residences/"+string(public.ID)), http.StatusNotFound)

	// person_id/place_id-фильтры списка.
	listRec := getReq(t, h, "/api/residences?person_id="+string(person.ID)+"&place_id="+string(place.ID))
	requireStatusS(t, listRec, http.StatusOK)
	if !strings.Contains(listRec.Body.String(), string(withCitation.ID)) {
		t.Fatalf("список по person_id/place_id не содержит запись %q: %s", withCitation.ID, listRec.Body)
	}
}

func createResidence(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Residence {
	t.Helper()

	rec := postResidenceReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeResidenceS(t, rec)
}

func decodeResidenceS(t *testing.T, rec *httptest.ResponseRecorder) transport.Residence {
	t.Helper()

	var r transport.Residence
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return r
}

func postResidenceReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/residences", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putResidenceReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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

// TestEventWriteContractWithRealStore: сквозной путь «хранилище → сценарии
// → HTTP» для событий. Покрывает: create с двумя участниками → чтение →
// изменение → удаление → 404; индексированный 422 на несуществующий
// participants[1].person_id; Place — мягкая ссылка (фабрикованный id
// круговорот без ошибки); строгий FK sources[0].citation_id;
// PUBLIC → PRIVATE update-флип; person_id-фильтр списка;
// search_events по началу текста места.
func TestEventWriteContractWithRealStore(t *testing.T) {
	st, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	authSvc := auth.New(auth.NewSQLStore(st.DB()))
	h := httpapi.NewAPIHandler(httpapi.Deps{
		Divisions:  newDivisionService(t, st),
		People:     newPersonService(t, st),
		Events:     newEventService(t, st),
		Sources:    newSourceService(t, st),
		Citations:  newCitationService(t, st),
		Auth:       authSvc,
		DocsFS:     fstest.MapFS{},
		TrustProxy: false,
	})

	regRec := postAuthReq(t, h, "/api/auth/register", `{"login":"owner","password":"password123"}`, nil)
	requireStatusS(t, regRec, http.StatusCreated)
	accessCookie, _ := sessionCookies(t, regRec)
	owner := []*http.Cookie{accessCookie}

	parent := createPerson(t, h, owner, `{"names":[{"type":"main","surname":{"text":"Кузнецов"},"given":{"text":"Фёдор"}}],"private":false}`, http.StatusCreated)
	witness := createPerson(t, h, owner, `{"names":[{"type":"main","surname":{"text":"Смирнов"},"given":{"text":"Егор"}}],"private":false}`, http.StatusCreated)

	// Создание: два участника, оба существуют + мягкая ссылка Place.
	created := createEvent(t, h, owner, fmt.Sprintf(`{
		"type": "birth",
		"place": {"text":"село Давыдово","ref":"AD-01ARZ3NDEKTSV4RRFFQ69G5FA9","type":"administrative_division"},
		"participants": [
			{"person_id":%q,"role":"родитель"},
			{"person_id":%q,"role":"свидетель"}
		],
		"notes": [],
		"private": false
	}`, parent.ID, witness.ID), http.StatusCreated)
	if created.Type != "birth" || len(created.Participants) != 2 {
		t.Fatalf("created = %+v", created)
	}
	if !strings.HasPrefix(string(created.ID), "E-") {
		t.Fatalf("id = %q", created.ID)
	}
	// Place — мягкая ссылка на заведомо несуществующий id: round-trip без ошибки.
	if created.Place == nil || created.Place.Ref != "AD-01ARZ3NDEKTSV4RRFFQ69G5FA9" || created.Place.Text != "село Давыдово" {
		t.Fatalf("created.Place = %+v, ожидался мягкий round-trip несуществующей ссылки", created.Place)
	}

	// Чтение по id — ссылки/участники пережили запись и чтение из SQLite.
	rec := getReq(t, h, "/api/events/"+string(created.ID))
	requireStatusS(t, rec, http.StatusOK)
	reread := decodeEventS(t, rec)
	if len(reread.Participants) != 2 || reread.Place == nil || reread.Place.Ref != "AD-01ARZ3NDEKTSV4RRFFQ69G5FA9" {
		t.Fatalf("reread (после чтения из SQLite) = %+v", reread)
	}

	// search_events: по началу текста места (весь place-текст индексируется
	// как один термин — "Давыд" не подошёл бы, текст начинается с "село").
	searchRec := getReq(t, h, "/api/events/search?q=село")
	requireStatusS(t, searchRec, http.StatusOK)
	if !strings.Contains(searchRec.Body.String(), string(created.ID)) {
		t.Fatalf("поиск по началу текста места не нашёл событие: %s", searchRec.Body)
	}

	// person_id-фильтр списка: находит событие по КАЖДОМУ из двух участников.
	for _, p := range []transport.Person{parent, witness} {
		listRec := getReq(t, h, "/api/events?person_id="+string(p.ID))
		requireStatusS(t, listRec, http.StatusOK)
		if !strings.Contains(listRec.Body.String(), string(created.ID)) {
			t.Fatalf("список по person_id=%q не содержит событие: %s", p.ID, listRec.Body)
		}
	}

	// Изменение: полная замена, участники сокращены до одного.
	rec = putEventReq(t, h, owner, "/api/events/"+string(created.ID), fmt.Sprintf(`{
		"type": "death",
		"participants": [{"person_id":%q,"role":"умерший"}],
		"notes": [],
		"private": false
	}`, parent.ID))
	requireStatusS(t, rec, http.StatusOK)
	updated := decodeEventS(t, rec)
	if updated.Type != "death" || len(updated.Participants) != 1 || updated.ID != created.ID {
		t.Fatalf("after update = %+v", updated)
	}

	// Удаление.
	requireStatusS(t, delReq(t, h, owner, "/api/events/"+string(created.ID)), http.StatusNoContent)

	// Несуществующая — 404.
	requireStatusS(t, getReq(t, h, "/api/events/"+string(created.ID)), http.StatusNotFound)

	// Строгий FK, индексированная ошибка: participants[0] существует,
	// participants[1] — нет — 422 на participants[1].person_id.
	rec = postEventReq(t, h, owner, fmt.Sprintf(`{
		"type": "birth",
		"participants": [
			{"person_id":%q,"role":"родитель"},
			{"person_id":"I-01ARZ3NDEKTSV4RRFFQ69G5FA9","role":"свидетель"}
		],
		"private": false
	}`, parent.ID))
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"participants[1].person_id"`) {
		t.Fatalf("body = %s, want field=participants[1].person_id", rec.Body)
	}

	// Строгий FK: sources[0].citation_id несуществующий — 422.
	rec = postEventReq(t, h, owner, `{"type":"birth","sources":[{"citation_id":"C-01ARZ3NDEKTSV4RRFFQ69G5FA9"}],"private":false}`)
	requireStatusS(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"sources[0].citation_id"`) {
		t.Fatalf("body = %s, want field=sources[0].citation_id", rec.Body)
	}

	// Строгий FK: существующая цитата — сохраняется.
	src := createSource(t, h, owner,
		`{"kind":"document","title":"Метрическая книга","reliability":"primary","notes":[],"private":false}`,
		http.StatusCreated)
	cit := createCitation(t, h, owner,
		fmt.Sprintf(`{"source_id":%q,"private":false}`, src.ID), http.StatusCreated)

	withCitation := createEvent(t, h, owner,
		fmt.Sprintf(`{"type":"birth","sources":[{"citation_id":%q}],"private":false}`, cit.ID),
		http.StatusCreated)
	if len(withCitation.Sources) != 1 || withCitation.Sources[0].CitationID != string(cit.ID) {
		t.Fatalf("withCitation.Sources = %+v, want [{citation_id: %q}]", withCitation.Sources, cit.ID)
	}

	// PUBLIC → PRIVATE update-флип (не private → private).
	public := createEvent(t, h, owner, `{"type":"birth","private":false}`, http.StatusCreated)
	requireStatusS(t, getReq(t, h, "/api/events/"+string(public.ID)), http.StatusOK)

	rec = putEventReq(t, h, owner, "/api/events/"+string(public.ID), `{"type":"birth","private":true}`)
	requireStatusS(t, rec, http.StatusOK)
	afterUpdate := decodeEventS(t, rec)
	if !afterUpdate.Private {
		t.Fatalf("private (после update) = %+v, want Private=true", afterUpdate)
	}
	requireStatusS(t, getReq(t, h, "/api/events/"+string(public.ID)), http.StatusNotFound)
}

func createEvent(t *testing.T, h http.Handler, cookies []*http.Cookie, body string, want int) transport.Event {
	t.Helper()

	rec := postEventReq(t, h, cookies, body)
	if rec.Code != want {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, want, rec.Body)
	}

	return decodeEventS(t, rec)
}

func decodeEventS(t *testing.T, rec *httptest.ResponseRecorder) transport.Event {
	t.Helper()

	var e transport.Event
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatalf("decode: %v; body = %s", err, rec.Body)
	}

	return e
}

func postEventReq(t *testing.T, h http.Handler, cookies []*http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/events", strings.NewReader(body))
	req.Header.Set("X-Requested-With", "genodex")
	for _, c := range cookies {
		req.AddCookie(c)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	return rec
}

func putEventReq(t *testing.T, h http.Handler, cookies []*http.Cookie, path, body string) *httptest.ResponseRecorder {
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
