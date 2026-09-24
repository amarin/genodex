package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// textRefObjectProperties — JSON-schema свойств объектного аргумента вида
// TextRef ({text, ref?, type?}) — используется в mcp.WithObject для полей
// вроде Church.Parish/Parish.Church. Появляется впервые в этом проходе
// (первые сущности с одиночным *TextRef, не списком). ref/type round-trip'ятся
// как есть (см. transport.TextRef.Model): клиент, уже получивший ref/type
// через church_get/parish_get, сохранит ссылку, отправив их обратно
// неизменными в church_update/parish_update; если их не передать вовсе или
// изменить только text — ссылка будет потеряна или расходиться с ним.
func textRefObjectProperties() map[string]any {
	return map[string]any{
		"text": map[string]any{"type": "string", "description": "Текст (обязателен, если нет ссылки)"},
		"ref":  map[string]any{"type": "string", "description": "id сущности-ссылки; сохраняется, если передать его обратно неизменным (например, из предыдущего *_get)"},
		"type": map[string]any{"type": "string", "description": "тип сущности-ссылки; сохраняется вместе с ref при неизменной передаче"},
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

// anchorObjectProperties — JSON-schema свойств объектного аргумента вида
// Anchor (полиморфная привязка «где именно» у Citation, см.
// transport.Anchor): плоский объект с дискриминатором kind и полями всех
// трёх вариантов вместе (по образцу FactDate), а не вложенный union — проще
// для MCP-клиента, чем oneOf. Первый полиморфный тип в программе.
func anchorObjectProperties() map[string]any {
	return map[string]any{
		"kind":          map[string]any{"type": "string", "enum": []string{"archive", "file", "url"}, "description": "Вид привязки; пустой объект или отсутствие аргумента — без привязки"},
		"node_id":       map[string]any{"type": "string", "description": "id архивного узла (kind=archive, обязателен для этого вида)"},
		"document_id":   map[string]any{"type": "string", "description": "id архивного документа (kind=archive, необязательно)"},
		"page":          map[string]any{"type": "integer", "description": "Номер страницы/скана (kind=archive, обязателен, не меньше 1)"},
		"rect":          map[string]any{"type": "string", "description": "Координаты области выделения на изображении (kind=archive, необязательно)"},
		"attachment_id": map[string]any{"type": "string", "description": "id вложения (kind=file, обязателен для этого вида)"},
		"timecode":      map[string]any{"type": "string", "description": "Тайм-метка для аудио/видео (kind=file, необязательно)"},
		"url":           map[string]any{"type": "string", "description": "Абсолютный http(s)-адрес (kind=url, обязателен для этого вида)"},
	}
}

// optionalAnchor читает необязательный объектный аргумент вида Anchor (см.
// anchorObjectProperties) из сырых аргументов тула и конвертирует его в
// модель; отсутствующий, null или пустой (kind не задан/не распознан)
// аргумент — nil, без ошибки (см. (*transport.Anchor).Model()).
func optionalAnchor(args map[string]any, name string) (models.Anchor, error) {
	raw, ok := args[name]
	if !ok || raw == nil {
		return nil, nil
	}

	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	var a transport.Anchor
	if err := json.Unmarshal(b, &a); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	return a.Model(), nil
}

// sourceLinkObjectProperties — JSON-schema свойств одного элемента массива
// sources (доказательство, см. transport.SourceLink). target_type/target_id
// сюда не входят — клиент их не отправляет, владелец подставляется сервером
// из контекста вызова (см. transport.SourceLink.Model()).
func sourceLinkObjectProperties() map[string]any {
	return map[string]any{
		"citation_id": map[string]any{"type": "string", "description": "id цитаты (обязателен)"},
		"reliability": map[string]any{"type": "string", "enum": []string{"primary", "contemporary", "memory", "indirect", "unknown"}, "description": "Достоверность именно этого утверждения по этой цитате"},
		"role":        map[string]any{"type": "string", "description": "Роль утверждения"},
		"note":        map[string]any{"type": "string", "description": "Заметка"},
	}
}

// optionalSourceLinks читает массив объектов вида SourceLink (см.
// sourceLinkObjectProperties) из сырых аргументов тула и конвертирует его в
// модели; отсутствующий или null аргумент — пустой срез, без ошибки (та же
// механика, что и textRefsFromStrings для списков TextRef).
func optionalSourceLinks(args map[string]any, name string) ([]models.SourceLink, error) {
	raw, ok := args[name]
	if !ok || raw == nil {
		return nil, nil
	}

	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	var links []transport.SourceLink
	if err := json.Unmarshal(b, &links); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	return transport.SourceLinksToModel(links), nil
}

// personNameObjectProperties — JSON-schema свойств одного элемента массива
// names (одно имя персоны, см. transport.PersonName). Первый массив объектов
// в программе, чьи собственные свойства тоже вложенные объекты
// (surname/given/patronymic — TextRef, см. textRefObjectProperties; since/
// until — FactDate, см. factDateObjectProperties) — то же техническое
// решение (mcp.Items с произвольной JSON-schema), что и sourceLinkObjectProperties
// (подпроект 5), только с более сложным элементом.
func personNameObjectProperties() map[string]any {
	return map[string]any{
		"type":       map[string]any{"type": "string", "enum": []string{"", "main", "birth", "married", "changed", "pseudonym"}, "description": "Вид имени; пусто — не указан"},
		"surname":    map[string]any{"type": "object", "properties": textRefObjectProperties(), "description": "Фамилия (текст или мягкая ссылка на словарь фамилий; существование ссылки не проверяется)"},
		"given":      map[string]any{"type": "object", "properties": textRefObjectProperties(), "description": "Имя (текст или мягкая ссылка на словарь личных имён; существование ссылки не проверяется)"},
		"patronymic": map[string]any{"type": "object", "properties": textRefObjectProperties(), "description": "Отчество (текст или мягкая ссылка на словарь отчеств; существование ссылки не проверяется)"},
		"prefix":     map[string]any{"type": "string", "description": "Служебная приставка (фон, де, ван…)"},
		"suffix":     map[string]any{"type": "string", "description": "Служебное окончание (ст., мл.…)"},
		"since":      map[string]any{"type": "object", "properties": factDateObjectProperties(), "description": "Начало периода действия этого имени"},
		"until":      map[string]any{"type": "object", "properties": factDateObjectProperties(), "description": "Конец периода действия этого имени"},
	}
}

// optionalPersonNames читает массив объектов вида PersonName (см.
// personNameObjectProperties) из сырых аргументов тула и конвертирует его в
// модели; отсутствующий или null аргумент — nil, без ошибки (та же
// механика, что и optionalSourceLinks).
func optionalPersonNames(args map[string]any, name string) ([]models.PersonName, error) {
	raw, ok := args[name]
	if !ok || raw == nil {
		return nil, nil
	}

	b, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	var names []transport.PersonName
	if err := json.Unmarshal(b, &names); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}

	return transport.PersonNamesToModel(names), nil
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
