package models

// Attachment — файловое вложение (скан страницы, документ, аудио, фото).
type Attachment struct {
	ID         ID
	Kind       AttachmentKind
	URI        string
	Filename   string
	MIME       string
	Page       int
	NodeID     ID
	DocumentID *ID
	Note       string
	Private    bool
}

// EntityType возвращает тип сущности.
func (a *Attachment) EntityType() Type { return TypeAttachment }
