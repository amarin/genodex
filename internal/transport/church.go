package transport

import "github.com/amarin/genodex/internal/models"

// Church — контракт церкви (GET /api/churches, MCP-тул church_list). Parish —
// необязательная ссылка (текст или ссылка на приход); Settlements — ссылки на
// административные деления; Variants — простые строки (не TextRef).
type Church struct {
	ID          models.ID    `json:"id"`
	Name        string       `json:"name"`
	Parish      *TextRef     `json:"parish,omitempty"`
	Settlements []TextRef    `json:"settlements"`
	Variants    []string     `json:"variants"`
	Notes       []TextRef    `json:"notes"`
	Sources     []SourceLink `json:"sources"`
}

// ChurchFromModel конвертирует запись в контракт.
func ChurchFromModel(c models.Church) Church {
	return Church{
		ID:          c.ID,
		Name:        c.Name,
		Parish:      TextRefFromModelPtr(c.Parish),
		Settlements: TextRefsFromModel(c.Settlements),
		Variants:    stringsOrEmpty(c.Variants),
		Notes:       TextRefsFromModel(c.Notes),
		Sources:     SourceLinksFromModel(c.Sources),
	}
}

// ChurchesFromModels конвертирует список; пустой вход даёт пустой срез, а не nil.
func ChurchesFromModels(cs []models.Church) []Church {
	out := make([]Church, 0, len(cs))
	for _, c := range cs {
		out = append(out, ChurchFromModel(c))
	}

	return out
}

// stringsOrEmpty возвращает пустой срез вместо nil (единый вид JSON-ответа с
// остальными списковыми полями).
func stringsOrEmpty(ss []string) []string {
	if ss == nil {
		return []string{}
	}

	return ss
}
