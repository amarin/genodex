package httpapi

import (
	"net/http"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// handleNoteList — GET /api/notes?limit=&offset=. Чтение открыто анонимному
// посетителю (приватные заметки скрыты — см. handleNoteGet).
func handleNoteList(notes NoteService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page, err := parsePage(r.URL.Query())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := notes.ListNotes(r.Context(), AccessFromContext(r.Context()), page)
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.NotesFromModels(list))
	}
}

// handleNoteSearch — GET /api/notes/search?q=&limit=&offset=.
func handleNoteSearch(notes NoteService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		page, err := parsePage(q)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})

			return
		}

		list, err := notes.SearchNotes(r.Context(), AccessFromContext(r.Context()),
			models.SearchQuery{Text: q.Get("q"), Page: page})
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.NotesFromModels(list))
	}
}

// handleNoteGet — GET /api/notes/{id}. Приватная заметка для анонимного или
// не-владельца — 404 (см. get_note.Scenario.GetNote,
// docs/data-model/entity-write.md §3.1).
func handleNoteGet(notes NoteService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		n, err := notes.GetNote(r.Context(), AccessFromContext(r.Context()), pathID(r))
		if err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, http.StatusOK, transport.NoteFromModel(n))
	}
}
