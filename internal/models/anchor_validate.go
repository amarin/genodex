package models

import "net/url"

// validateAnchor проверяет привязку «где именно». Пустой якорь допустим
// (цитата может быть только текстовой); nil-указатель внутри интерфейса и
// неизвестная реализация — ошибка. Путь ошибки самого якоря пуст, полей — по
// имени (node_id, page, url, …).
func validateAnchor(a Anchor) *ValidationError {
	switch v := a.(type) {
	case nil:
		return nil
	case *ArchiveAnchor:
		if v == nil {
			return fieldErr("", "пустой якорь (nil-указатель)")
		}
		if e := idErr("node_id", v.NodeID, TypeArchiveNode); e != nil {
			return e
		}
		if e := validateOptionalID("document_id", v.DocumentID, TypeArchiveDocument); e != nil {
			return e
		}
		if v.Page < 1 {
			return fieldErr("page", "номер страницы должен быть не меньше 1: %d", v.Page)
		}

		return nil
	case *FileAnchor:
		if v == nil {
			return fieldErr("", "пустой якорь (nil-указатель)")
		}

		return idErr("attachment_id", v.AttachmentID, TypeAttachment)
	case *URLAnchor:
		if v == nil {
			return fieldErr("", "пустой якорь (nil-указатель)")
		}
		u, err := url.Parse(v.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
			return fieldErr("url", "нужен абсолютный http(s)-адрес с хостом: %q", v.URL)
		}

		return nil
	default:
		return fieldErr("", "неизвестная реализация якоря %T", a)
	}
}
