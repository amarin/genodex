package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

// TestCitationCreateContract — anchor кодируется как вложенный объект с
// дискриминатором kind; используем вариант url — самый простой, без ссылки
// на другую сущность.
func TestCitationCreateContract(t *testing.T) {
	svc := &fakeCitations{created: models.Citation{ID: "C-1", SourceID: "S-1"}}

	rec := postD(t, NewHandler(Deps{Citations: svc, DocsFS: fstest.MapFS{}}), "/api/citations",
		`{"source_id":"S-1","anchor":{"kind":"url","url":"https://example.org"},"text":"запись №5"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.SourceID != "S-1" || svc.gotCreate.Text != "запись №5" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}

	anchor, ok := svc.gotCreate.Anchor.(*models.URLAnchor)
	if !ok || anchor.URL != "https://example.org" {
		t.Fatalf("gotCreate.Anchor = %+v", svc.gotCreate.Anchor)
	}
}

// TestCitationCreateSourceNotFoundIs422 — usecase-слой возвращает
// *models.ValidationError по полю source_id при несуществующем источнике
// (см. create_citation), writeError мапит это на 422, по образцу
// create_archive/create_source.
func TestCitationCreateSourceNotFoundIs422(t *testing.T) {
	svc := &fakeCitations{err: &models.ValidationError{Entity: models.TypeCitation, Field: "source_id", Reason: "источник не найден"}}

	rec := postD(t, NewHandler(Deps{Citations: svc, DocsFS: fstest.MapFS{}}), "/api/citations",
		`{"source_id":"S-999","text":"запись №5"}`)

	requireStatus(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"source_id"`) {
		t.Fatalf("body = %s, ожидалось поле source_id", rec.Body)
	}
}

func TestCitationCreateAnonymousIs401(t *testing.T) {
	svc := &fakeCitations{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Citations: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/citations", strings.NewReader(`{"source_id":"S-1","text":"запись №5"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

// TestCitationUpdateMergesFields — проверяет все поля, включая anchor.
func TestCitationUpdateMergesFields(t *testing.T) {
	svc := &fakeCitations{getC: models.Citation{ID: "C-1", SourceID: "S-1"}}

	rec := putD(t, NewHandler(Deps{Citations: svc, DocsFS: fstest.MapFS{}}), "/api/citations/C-1",
		`{"source_id":"S-2","anchor":{"kind":"url","url":"https://example.org"},`+
			`"text":"запись №5 (испр.)","note":"проверить","private":true}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.SourceID != "S-2" || svc.updated.Text != "запись №5 (испр.)" ||
		svc.updated.Note != "проверить" || svc.updated.Private != true || svc.updated.ID != "C-1" {
		t.Fatalf("updated = %+v", svc.updated)
	}

	anchor, ok := svc.updated.Anchor.(*models.URLAnchor)
	if !ok || anchor.URL != "https://example.org" {
		t.Fatalf("updated.Anchor = %+v", svc.updated.Anchor)
	}
}

func TestCitationDeleteNoContent(t *testing.T) {
	svc := &fakeCitations{}

	rec := delD(t, NewHandler(Deps{Citations: svc, DocsFS: fstest.MapFS{}}), "/api/citations/C-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestCitationDeleteInUseIs409(t *testing.T) {
	svc := &fakeCitations{deleteErr: &models.InUseError{Type: models.TypeCitation, ID: "C-1"}}

	rec := delD(t, NewHandler(Deps{Citations: svc, DocsFS: fstest.MapFS{}}), "/api/citations/C-1")

	requireStatus(t, rec, http.StatusConflict)
}
