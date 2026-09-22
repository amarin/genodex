package httpapi

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleDivisionList — GET /api/admin-divisions?kind=&type=&limit=&offset=.
// Синтаксически неверный параметр (limit не число) — 400; неверное значение
// (неизвестные kind/type, отрицательное окно) — 422 (*models.ValidationError
// из сценария).
func handleDivisionList(divisions DivisionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := parseDivisionQuery(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := divisions.ListDivisions(r.Context(), AccessFromContext(r.Context()), q)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.AdminDivisionsFromModels(list))
	}
}

// parseDivisionQuery разбирает параметры запроса; отсутствующий — нулевое значение.
func parseDivisionQuery(v url.Values) (models.DivisionQuery, error) {
	q := models.DivisionQuery{
		Kind: models.DivisionKind(v.Get("kind")),
		Type: models.AdminDivisionType(v.Get("type")),
	}

	if raw := v.Get("parent_id"); raw != "" {
		pid := models.ID(raw)
		q.ParentID = &pid
	}

	var err error

	if q.Page.Limit, err = intParam(v, "limit"); err != nil {
		return q, err
	}

	if q.Page.Offset, err = intParam(v, "offset"); err != nil {
		return q, err
	}

	return q, nil
}

// handleDivisionSearch — GET /api/admin-divisions/search?q=&limit=&offset=.
// Синтаксически неверный параметр — 400; неверное значение (отрицательное окно) — 422.
func handleDivisionSearch(divisions DivisionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q, err := parseDivisionSearchQuery(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := divisions.SearchDivisions(r.Context(), AccessFromContext(r.Context()), q)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.AdminDivisionsFromModels(list))
	}
}

// parseDivisionSearchQuery разбирает параметры поиска.
func parseDivisionSearchQuery(v url.Values) (models.DivisionSearchQuery, error) {
	q := models.DivisionSearchQuery{Text: v.Get("q")}

	var err error

	if q.Page.Limit, err = intParam(v, "limit"); err != nil {
		return q, err
	}

	if q.Page.Offset, err = intParam(v, "offset"); err != nil {
		return q, err
	}

	return q, nil
}

// intParam читает необязательный целочисленный параметр.
func intParam(v url.Values, name string) (int, error) {
	raw := v.Get(name)
	if raw == "" {
		return 0, nil
	}

	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("параметр %s: ожидалось целое число, получено %q", name, raw)
	}

	return n, nil
}
