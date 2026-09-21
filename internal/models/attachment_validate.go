package models

import (
	"mime"
	"strings"
)

// Validate проверяет файловое вложение: вид — закрытый enum; нужен uri или
// имя файла; MIME (если задан) — тип/подтип; страница не отрицательна (0 —
// не указана); узел — archive_node, документ (если задан) — archive_document.
func (a *Attachment) Validate() error {
	return finish(TypeAttachment, a.validate())
}

func (a *Attachment) validate() *ValidationError {
	if e := idErr("id", a.ID, TypeAttachment); e != nil {
		return e
	}
	if !a.Kind.Valid() {
		return fieldErr("kind", "недопустимый вид вложения %q", a.Kind)
	}
	if strings.TrimSpace(a.URI) == "" && strings.TrimSpace(a.Filename) == "" {
		return fieldErr("uri", "нужен uri или имя файла")
	}

	if a.MIME != "" {
		mediaType, _, err := mime.ParseMediaType(a.MIME)
		if err != nil || !strings.Contains(mediaType, "/") {
			return fieldErr("mime", "недопустимый MIME-тип %q: ожидается тип/подтип", a.MIME)
		}
	}
	if a.Page < 0 {
		return fieldErr("page", "номер страницы не может быть отрицательным: %d", a.Page)
	}

	if e := idErr("node_id", a.NodeID, TypeArchiveNode); e != nil {
		return e
	}

	return validateOptionalIDPtr("document_id", a.DocumentID, TypeArchiveDocument)
}
