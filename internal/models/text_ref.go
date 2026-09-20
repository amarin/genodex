package models

// TextRef — универсальный «текст-или-ссылка»: элемент может быть либо строкой,
// либо ссылкой на сущность.
type TextRef struct {
	// Text — отображаемая строка (обязательна, если не задана ссылка).
	Text string
	// Ref — id целевой сущности (пусто, если элемент — просто текст).
	Ref ID
	// Type — тип целевой сущности (задаётся вместе с Ref).
	Type Type
}

// NamedPeriod — наименование с периодом действия (открытым с одной или обеих сторон).
type NamedPeriod struct {
	Text  string
	Since string
	Until string
}

// PlaceRef — указание на место: TextRef, чья ссылка (если есть) ведёт на сущность
// места (AdministrativeDivision | Church | Parish). Свободный текст — когда
// сущности места ещё нет.
type PlaceRef struct {
	Text string
	Ref  ID
	Type Type
}
