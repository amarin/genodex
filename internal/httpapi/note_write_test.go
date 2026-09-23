package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

func TestNoteCreateContract(t *testing.T) {
	svc := &fakeNotes{created: models.Note{ID: "N-1", Kind: "note", Text: "текст"}}

	rec := postD(t, NewHandler(Deps{Notes: svc, DocsFS: fstest.MapFS{}}), "/api/notes",
		`{"kind":"note","text":"текст"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Kind != "note" || svc.gotCreate.Text != "текст" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestNoteCreatePassesPrivate — регресс: private изначально отсутствовал в
// NoteCreate DTO, из-за чего его нельзя было выставить при создании.
func TestNoteCreatePassesPrivate(t *testing.T) {
	svc := &fakeNotes{created: models.Note{ID: "N-1", Kind: "note", Text: "текст", Private: true}}

	rec := postD(t, NewHandler(Deps{Notes: svc, DocsFS: fstest.MapFS{}}), "/api/notes",
		`{"kind":"note","text":"текст","private":true}`)

	requireStatus(t, rec, http.StatusCreated)
	if !svc.gotCreate.Private {
		t.Fatalf("gotCreate.Private = %v, want true", svc.gotCreate.Private)
	}
}

func TestNoteCreateAnonymousIs401(t *testing.T) {
	svc := &fakeNotes{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Notes: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/notes", strings.NewReader(`{"kind":"note","text":"текст"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

// TestNoteUpdateMergesFields — включая parent_id, единственное поле у Note
// со строгой self-ref ссылкой.
func TestNoteUpdateMergesFields(t *testing.T) {
	svc := &fakeNotes{getN: models.Note{ID: "N-1", Kind: "note", Text: "текст"}}

	rec := putD(t, NewHandler(Deps{Notes: svc, DocsFS: fstest.MapFS{}}), "/api/notes/N-1",
		`{"kind":"note","text":"новый текст","parent_id":"N-2","private":true}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Text != "новый текст" || svc.updated.ID != "N-1" ||
		svc.updated.ParentID == nil || *svc.updated.ParentID != "N-2" ||
		!svc.updated.Private {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestNoteDeleteNoContent(t *testing.T) {
	svc := &fakeNotes{}

	rec := delD(t, NewHandler(Deps{Notes: svc, DocsFS: fstest.MapFS{}}), "/api/notes/N-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestNoteDeleteInUseIs409(t *testing.T) {
	svc := &fakeNotes{deleteErr: &models.InUseError{Type: models.TypeNote, ID: "N-1"}}

	rec := delD(t, NewHandler(Deps{Notes: svc, DocsFS: fstest.MapFS{}}), "/api/notes/N-1")

	requireStatus(t, rec, http.StatusConflict)
}
