package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// textRefObjectProperties — JSON-schema свойств объектного аргумента вида
// TextRef ({text, ref?, type?}) — используется в mcp.WithObject для полей
// вроде Church.Parish/Parish.Church/Archive.System. Появляется впервые в этом
// проходе (первые сущности с одиночным *TextRef, не списком).
func textRefObjectProperties() map[string]any {
	return map[string]any{
		"text": map[string]any{"type": "string", "description": "Текст (обязателен, если нет ссылки)"},
		"ref":  map[string]any{"type": "string", "description": "id сущности-ссылки (только чтение — задаётся не через этот тул)"},
		"type": map[string]any{"type": "string", "description": "тип сущности-ссылки (только чтение)"},
	}
}

// factDateObjectProperties — JSON-schema свойств объектного аргумента вида
// FactDate (структурированная дата с точностью). Появляется впервые в этом
// проходе.
func factDateObjectProperties() map[string]any {
	return map[string]any{
		"year":      map[string]any{"type": "integer", "description": "Год (1-9999), обязателен, если precision не unknown"},
		"month":     map[string]any{"type": "integer", "description": "Месяц (1-12), нужен при precision=month/day"},
		"day":       map[string]any{"type": "integer", "description": "День, нужен при precision=day"},
		"precision": map[string]any{"type": "string", "enum": []string{"unknown", "year", "month", "day"}, "description": "Верхняя известная точность"},
		"modifier":  map[string]any{"type": "string", "enum": []string{"exact", "approx", "before", "after", "between"}, "description": "Формулировка: точно/около/до/после/между"},
		"calendar":  map[string]any{"type": "string", "enum": []string{"", "gregorian", "julian", "unknown"}, "description": "Календарь; пусто — не указан"},
		"year_to":   map[string]any{"type": "integer", "description": "Год верхней границы, только при modifier=between"},
		"month_to":  map[string]any{"type": "integer", "description": "Месяц верхней границы"},
		"day_to":    map[string]any{"type": "integer", "description": "День верхней границы"},
	}
}

// optionalTextRef читает необязательный объектный аргумент {text, ref?, type?}
// (см. transport.TextRef) из сырых аргументов тула и конвертирует его в
// модель; отсутствующий или null аргумент — nil, без ошибки.
func optionalTextRef(args map[string]any, name string) (*models.TextRef, error) {
	raw, ok := args[name]
	if !ok || raw == nil {
		return nil, nil
	}

	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	var t transport.TextRef
	if err := json.Unmarshal(b, &t); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	m := t.Model()

	return &m, nil
}

// optionalFactDate читает необязательный объектный аргумент (структурированная
// дата, см. transport.FactDate) из сырых аргументов тула и конвертирует его в
// модель; отсутствующий или null аргумент — nil, без ошибки.
func optionalFactDate(args map[string]any, name string) (*models.FactDate, error) {
	raw, ok := args[name]
	if !ok || raw == nil {
		return nil, nil
	}

	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	var d transport.FactDate
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	return d.Model(), nil
}
