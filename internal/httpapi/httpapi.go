package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/amarin/genodex/internal/store"
)

// NewHandler возвращает http.Handler с маршрутами /api.
func NewHandler(st *store.Store) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handleHealth)
	mux.HandleFunc("GET /api/settlements", handleSettlementList(st))
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
