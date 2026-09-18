package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/store"
)

func handleSettlementList(st *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, st.ListSettlements())
	}
}
