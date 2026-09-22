package httpapi

import (
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// NewHandler возвращает http.Handler с маршрутами /api.
// docsFS — файловая система папки docs для раздела «Документация».
func NewHandler(divisions DivisionService, docsFS fs.FS) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handleHealth)
	mux.HandleFunc("GET /api/admin-divisions", handleDivisionList(divisions))
	mux.HandleFunc("GET /api/admin-divisions/{id}", handleDivisionGet(divisions))
	mux.HandleFunc("POST /api/admin-divisions", handleDivisionCreate(divisions))
	mux.HandleFunc("PUT /api/admin-divisions/{id}", handleDivisionUpdate(divisions))
	mux.HandleFunc("DELETE /api/admin-divisions/{id}", handleDivisionDelete(divisions))
	mux.HandleFunc("GET /api/docs", handleDocList(docsFS))
	mux.HandleFunc("GET /api/docs/{path}", handleDocContent(docsFS))
	return mux
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError отвечает на ошибку сценария: *models.ValidationError — 422 с полем,
// models.ErrNotFound — 404, *models.InUseError — 409 с телом InUseErrorBody,
// остальное — 500.
func writeError(w http.ResponseWriter, err error) {
	var ve *models.ValidationError
	if errors.As(err, &ve) {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{"error": ve.Error(), "field": ve.Field})

		return
	}

	if errors.Is(err, models.ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})

		return
	}

	var iu *models.InUseError
	if errors.As(err, &iu) {
		writeJSON(w, http.StatusConflict, transport.InUseErrorBodyFromModel(iu))

		return
	}

	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}
