package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// registerAttachmentTools регистрирует тулы для работы с файловыми
// вложениями. node_id/document_id — просто id (не объекты): node_id
// обязателен, document_id — необязателен (пусто — не задан).
// attachment_create/attachment_update проверяют существование обоих
// (document_id — если задан) через generic-хранилище, независимо от
// собственного CRUD-слоя ArchiveNode/ArchiveDocument.
func registerAttachmentTools(s *server.MCPServer, attachments AttachmentService) {
	tool := mcp.NewTool(
		"attachment_list",
		mcp.WithDescription("Список файловых вложений в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, attachmentListHandler(attachments))

	tool = mcp.NewTool(
		"attachment_search",
		mcp.WithDescription("Поиск вложений по началу имени файла или URI; результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало имени файла или URI")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, attachmentSearchHandler(attachments))

	tool = mcp.NewTool(
		"attachment_get",
		mcp.WithDescription("Вложение по id; результат — JSON записи. Неверный формат id, отсутствующая или приватная (без полного доступа) запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например O-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, attachmentGetHandler(attachments))

	tool = mcp.NewTool(
		"attachment_create",
		mcp.WithDescription("Создать вложение; id генерируется сервером; результат — JSON созданной записи. Нужен uri или filename. Несуществующий node_id или document_id — ошибка тула"),
		mcp.WithString("kind", mcp.Required(), mcp.Enum("scan", "document", "audio", "photo"), mcp.Description("Вид вложения")),
		mcp.WithString("uri", mcp.Description("URI (файл, ссылка)")),
		mcp.WithString("filename", mcp.Description("Имя файла")),
		mcp.WithString("mime", mcp.Description("MIME-тип (тип/подтип)")),
		mcp.WithNumber("page", mcp.Description("Номер страницы (0 — не указана)")),
		mcp.WithString("node_id", mcp.Required(), mcp.Description("id архивного узла (ArchiveNode)")),
		mcp.WithString("document_id", mcp.Description("id архивного документа (ArchiveDocument), необязательно")),
		mcp.WithString("note", mcp.Description("Заметка")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, attachmentCreateHandler(attachments))

	tool = mcp.NewTool(
		"attachment_update",
		mcp.WithDescription("Изменить вложение: полная замена kind/uri/filename/mime/page/node_id/document_id/note/private; результат — JSON обновлённой записи. Несуществующий node_id или document_id — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("kind", mcp.Required(), mcp.Enum("scan", "document", "audio", "photo"), mcp.Description("Вид вложения")),
		mcp.WithString("uri", mcp.Description("URI")),
		mcp.WithString("filename", mcp.Description("Имя файла")),
		mcp.WithString("mime", mcp.Description("MIME-тип")),
		mcp.WithNumber("page", mcp.Description("Номер страницы")),
		mcp.WithString("node_id", mcp.Required(), mcp.Description("id архивного узла")),
		mcp.WithString("document_id", mcp.Description("id архивного документа, необязательно")),
		mcp.WithString("note", mcp.Description("Заметка")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, attachmentUpdateHandler(attachments))

	tool = mcp.NewTool(
		"attachment_delete",
		mcp.WithDescription("Удалить вложение. Необратимо. Если на него есть строгие ссылки от других сущностей — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, attachmentDeleteHandler(attachments))
}

func attachmentListHandler(attachments AttachmentService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := attachments.ListAttachments(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.AttachmentsFromModels(list))
	}
}

func attachmentSearchHandler(attachments AttachmentService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := attachments.SearchAttachments(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.AttachmentsFromModels(list))
	}
}

func attachmentGetHandler(attachments AttachmentService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		a, err := attachments.GetAttachment(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.AttachmentFromModel(a))
	}
}

func attachmentCreateHandler(attachments AttachmentService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page, err := optionalInt(req, "page")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		a := models.Attachment{
			Kind:     models.AttachmentKind(req.GetString("kind", "")),
			URI:      req.GetString("uri", ""),
			Filename: req.GetString("filename", ""),
			MIME:     req.GetString("mime", ""),
			Page:     page,
			NodeID:   models.ID(req.GetString("node_id", "")),
			Note:     req.GetString("note", ""),
			Private:  req.GetBool("private", false),
		}

		if did := req.GetString("document_id", ""); did != "" {
			id := models.ID(did)
			a.DocumentID = &id
		}

		created, err := attachments.CreateAttachment(ctx, a)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.AttachmentFromModel(created))
	}
}

func attachmentUpdateHandler(attachments AttachmentService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := attachments.GetAttachment(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		page, err := optionalInt(req, "page")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cur.Kind = models.AttachmentKind(req.GetString("kind", ""))
		cur.URI = req.GetString("uri", "")
		cur.Filename = req.GetString("filename", "")
		cur.MIME = req.GetString("mime", "")
		cur.Page = page
		cur.NodeID = models.ID(req.GetString("node_id", ""))
		cur.Note = req.GetString("note", "")
		cur.Private = req.GetBool("private", false)

		if did := req.GetString("document_id", ""); did != "" {
			didID := models.ID(did)
			cur.DocumentID = &didID
		} else {
			cur.DocumentID = nil
		}

		if err := attachments.UpdateAttachment(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.AttachmentFromModel(cur))
	}
}

func attachmentDeleteHandler(attachments AttachmentService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := attachments.DeleteAttachment(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
