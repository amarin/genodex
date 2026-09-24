package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/transport"
)

// handleDivisionTypeList — GET /api/admin-division-types: справочник типов
// единиц деления (название, ранг, населённый пункт, допустимые дочерние
// типы). Чтение открыто анонимному посетителю, как и список делений.
func handleDivisionTypeList(divisions DivisionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		infos, err := divisions.ListDivisionTypes(r.Context())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.AdminDivisionTypeInfosFromModels(infos))
	}
}
