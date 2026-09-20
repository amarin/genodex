package models

// TextRef — универсальный «текст-или-ссылка»: элемент может быть либо строкой,
// либо ссылкой на сущность.
type TextRef struct {
	// Text — отображаемая строка (обязательна).
	Text string `json:"text"`
	// Ref — id целевой сущности (пусто, если элемент — просто текст).
	Ref string `json:"ref,omitempty"`
	// Type — тип целевой сущности (задаётся вместе с Ref).
	Type string `json:"type,omitempty"`
}

// NamedPeriod — наименование с периодом действия (открытым с одной или обеих сторон).
type NamedPeriod struct {
	Text  string `json:"text"`
	Since string `json:"since,omitempty"`
	Until string `json:"until,omitempty"`
}

// PlaceRef — указание на место: TextRef, чья ссылка (если есть) ведёт на сущность
// места (AdministrativeDivision | Church | Parish). Свободный текст — когда
// сущности места ещё нет.
type PlaceRef struct {
	Text string `json:"text"`
	Ref  string `json:"ref,omitempty"`
	Type string `json:"type,omitempty"`
}
