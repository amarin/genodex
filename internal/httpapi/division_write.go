package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleDivisionGet — GET /api/admin-divisions/{id}. Неверный формат id — 422
// (ValidationError сценария), отсутствующая единица — 404. Чтение открыто
// анонимному посетителю (auth.md §6 — Access здесь не проверяется).
func handleDivisionGet(divisions DivisionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		d, err := divisions.GetDivision(r.Context(), pathDivisionID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.AdminDivisionFromModel(d))
	}
}

// handleDivisionCreate — POST /api/admin-divisions: создаёт единицу, отвечает
// 201 с созданной единицей (id генерирует сценарий). Запись — только для
// вошедшего владельца (auth.md §6, решение 9): без активной сессии — 401
// раньше разбора тела, сценарий не вызывается.
func handleDivisionCreate(divisions DivisionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		var in transport.AdminDivisionCreate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		created, err := divisions.CreateDivision(r.Context(), in.Model())
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusCreated, transport.AdminDivisionFromModel(created))
	}
}

// handleDivisionUpdate — PUT /api/admin-divisions/{id}: полная замена полей
// name/type/parent_id; прочие поля текущей модели сохраняются (обработчик
// берёт версию через get_division и накладывает поля запроса). Запись —
// только для вошедшего владельца, см. handleDivisionCreate.
func handleDivisionUpdate(divisions DivisionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		id := pathDivisionID(r)

		var in transport.AdminDivisionUpdate
		if err := decodeJSON(r, &in); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "не удалось разобрать тело: " + err.Error()})

			return
		}

		cur, err := divisions.GetDivision(r.Context(), id)
		if err != nil {
			writeError(w, err)

			return
		}

		cur.Name = in.Name
		cur.Type = in.Type
		cur.ParentID = cloneID(in.ParentID)
		cur.Sources = transport.SourceLinksToModel(in.Sources)

		if err := divisions.UpdateDivision(r.Context(), cur); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.AdminDivisionFromModel(cur))
	}
}

// handleDivisionDelete — DELETE /api/admin-divisions/{id}: 204 без тела;
// занятая единица — 409 со списком ссылающихся. Запись — только для
// вошедшего владельца, см. handleDivisionCreate.
func handleDivisionDelete(divisions DivisionService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := requireFull(w, r); !ok {
			return
		}

		if err := divisions.DeleteDivision(r.Context(), pathDivisionID(r)); err != nil {
			writeError(w, err)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func pathDivisionID(r *http.Request) models.ID {
	return models.ID(strings.TrimPrefix(r.PathValue("id"), "/"))
}

func cloneID(p *models.ID) *models.ID {
	if p == nil {
		return nil
	}

	v := *p

	return &v
}

func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
