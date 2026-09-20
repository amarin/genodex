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
	NodeID ID
	// DocumentID — уточнение до документа внутри единицы (опционально).
	DocumentID ID
	// Page — номер скана/страницы.
	Page int
	// Rect — координаты области выделения на изображении (опционально).
	Rect string
}

// Kind возвращает дискриминатор.
func (a *ArchiveAnchor) Kind() AnchorKind { return AnchorArchive }

// FileAnchor — файл с тайминговой привязкой.
type FileAnchor struct {
	// AttachmentID — вложение.
	AttachmentID ID
	// Timecode — тайм-метка для аудио/видео (опционально).
	Timecode string
}

// Kind возвращает дискриминатор.
func (a *FileAnchor) Kind() AnchorKind { return AnchorFile }

// URLAnchor — внешняя ссылка.
type URLAnchor struct {
	// URL — адрес.
	URL string
}

// Kind возвращает дискриминатор.
func (a *URLAnchor) Kind() AnchorKind { return AnchorURL }
