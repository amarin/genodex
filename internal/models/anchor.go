package models

// AnchorKind — дискриминатор реализации Anchor.
type AnchorKind string

const (
	AnchorArchive AnchorKind = "archive"
	AnchorFile    AnchorKind = "file"
	AnchorURL     AnchorKind = "url"
)

// Anchor — привязка «где именно» к доказательству. Общий контракт
// (дискриминатор Kind для сериализации) + отдельные реализации.
type Anchor interface {
	Kind() AnchorKind
}

// ArchiveAnchor — скан страницы единицы учёта в архиве.
type ArchiveAnchor struct {
	// NodeID — узел цепочки (единица учёта).
	NodeID string `json:"node_id"`
	// DocumentID — уточнение до документа внутри единицы (опционально).
	DocumentID string `json:"document_id,omitempty"`
	// Page — номер скана/страницы.
	Page int `json:"page"`
	// Rect — координаты области выделения на изображении (опционально).
	Rect string `json:"rect,omitempty"`
}

// Kind возвращает дискриминатор.
func (a *ArchiveAnchor) Kind() AnchorKind { return AnchorArchive }

// FileAnchor — файл с тайминговой привязкой.
type FileAnchor struct {
	// AttachmentID — вложение.
	AttachmentID string `json:"attachment_id"`
	// Timecode — тайм-метка для аудио/видео (опционально).
	Timecode string `json:"timecode,omitempty"`
}

// Kind возвращает дискриминатор.
func (a *FileAnchor) Kind() AnchorKind { return AnchorFile }

// URLAnchor — внешняя ссылка.
type URLAnchor struct {
	// URL — адрес.
	URL string `json:"url"`
}

// Kind возвращает дискриминатор.
func (a *URLAnchor) Kind() AnchorKind { return AnchorURL }
