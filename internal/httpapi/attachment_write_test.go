package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/amarin/genodex/internal/models"
)

func TestAttachmentCreateContract(t *testing.T) {
	svc := &fakeAttachments{created: models.Attachment{ID: "O-1", Kind: models.AttachmentKindScan, Filename: "0012.jpg", NodeID: "AN-1"}}

	rec := postD(t, NewHandler(Deps{Attachments: svc, DocsFS: fstest.MapFS{}}), "/api/attachments",
		`{"kind":"scan","filename":"0012.jpg","node_id":"AN-1"}`)

	requireStatus(t, rec, http.StatusCreated)
	if svc.gotCreate.Kind != models.AttachmentKindScan || svc.gotCreate.NodeID != "AN-1" || svc.gotCreate.ID != "" {
		t.Fatalf("gotCreate = %+v", svc.gotCreate)
	}
}

// TestAttachmentCreateNodeNotFoundIs422 — usecase-слой возвращает
// *models.ValidationError по полю node_id при несуществующем узле,
// writeError мапит это на 422.
func TestAttachmentCreateNodeNotFoundIs422(t *testing.T) {
	svc := &fakeAttachments{err: &models.ValidationError{Entity: models.TypeAttachment, Field: "node_id", Reason: "узел не найден"}}

	rec := postD(t, NewHandler(Deps{Attachments: svc, DocsFS: fstest.MapFS{}}), "/api/attachments",
		`{"kind":"scan","filename":"0012.jpg","node_id":"AN-999"}`)

	requireStatus(t, rec, http.StatusUnprocessableEntity)
	if !strings.Contains(rec.Body.String(), `"field":"node_id"`) {
		t.Fatalf("body = %s, ожидалось поле node_id", rec.Body)
	}
}

func TestAttachmentCreateAnonymousIs401(t *testing.T) {
	svc := &fakeAttachments{}

	rec := httptest.NewRecorder()
	NewHandler(Deps{Attachments: svc, DocsFS: fstest.MapFS{}}).ServeHTTP(rec,
		httptest.NewRequest(http.MethodPost, "/api/attachments", strings.NewReader(`{"kind":"scan","filename":"0012.jpg","node_id":"AN-1"}`)))

	requireStatus(t, rec, http.StatusUnauthorized)
}

func TestAttachmentUpdateMergesFields(t *testing.T) {
	svc := &fakeAttachments{getA: models.Attachment{ID: "O-1", Kind: models.AttachmentKindScan, Filename: "0012.jpg", NodeID: "AN-1"}}

	rec := putD(t, NewHandler(Deps{Attachments: svc, DocsFS: fstest.MapFS{}}), "/api/attachments/O-1",
		`{"kind":"photo","filename":"0013.jpg","node_id":"AN-1","document_id":"DC-1","private":true}`)

	requireStatus(t, rec, http.StatusOK)
	if svc.updated.Kind != models.AttachmentKindPhoto || svc.updated.Filename != "0013.jpg" ||
		svc.updated.DocumentID == nil || *svc.updated.DocumentID != "DC-1" ||
		svc.updated.Private != true || svc.updated.ID != "O-1" {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestAttachmentDeleteNoContent(t *testing.T) {
	svc := &fakeAttachments{}

	rec := delD(t, NewHandler(Deps{Attachments: svc, DocsFS: fstest.MapFS{}}), "/api/attachments/O-1")

	requireStatus(t, rec, http.StatusNoContent)
}

func TestAttachmentDeleteInUseIs409(t *testing.T) {
	svc := &fakeAttachments{deleteErr: &models.InUseError{Type: models.TypeAttachment, ID: "O-1"}}

	rec := delD(t, NewHandler(Deps{Attachments: svc, DocsFS: fstest.MapFS{}}), "/api/attachments/O-1")

	requireStatus(t, rec, http.StatusConflict)
}
