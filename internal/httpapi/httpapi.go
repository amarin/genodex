package httpapi

import (
	"encoding/json"
	"io/fs"
	"net/http"

	"github.com/amarin/genodex/internal/store"
)

// NewHandler возвращает http.Handler с маршрутами /api.
// docsFS — файловая система папки docs для раздела «Документация».
func NewHandler(st *store.Store, docsFS fs.FS) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handleHealth)
	mux.HandleFunc("GET /api/settlements", handleSettlementList(st))
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
