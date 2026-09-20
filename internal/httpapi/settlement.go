package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

func handleSettlementList(settlements SettlementService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		list, err := settlements.ListSettlements(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, transport.SettlementsFromModels(list))
	}
}
