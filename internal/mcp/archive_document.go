package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// registerArchiveDocumentTools регистрирует тулы для работы с документами
// внутри единиц учёта. unit_id — обязательная строгая ссылка на
// ArchiveNode; в отличие от archive_node_list, archive_document_list —
// плоский список без обязательного фильтра (сущность не иерархична).
func registerArchiveDocumentTools(s *server.MCPServer, archiveDocuments ArchiveDocumentService) {
	tool := mcp.NewTool(
		"archive_document_list",
		mcp.WithDescription("Список документов внутри единиц учёта в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, archiveDocumentListHandler(archiveDocuments))

	tool = mcp.NewTool(
		"archive_document_search",
		mcp.WithDescription("Поиск документов внутри единиц учёта по началу названия; результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало названия")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, archiveDocumentSearchHandler(archiveDocuments))

	tool = mcp.NewTool(
		"archive_document_get",
		mcp.WithDescription("Документ внутри единицы учёта по id; результат — JSON записи. Неверный формат id или отсутствующая запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например DC-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, archiveDocumentGetHandler(archiveDocuments))

	tool = mcp.NewTool(
		"archive_document_create",
		mcp.WithDescription("Создать документ внутри единицы учёта; id генерируется сервером; результат — JSON созданной записи. Несуществующий unit_id — ошибка тула"),
		mcp.WithString("unit_id", mcp.Required(), mcp.Description("id единицы учёта (узла архивного дерева)")),
		mcp.WithString("title", mcp.Required(), mcp.Description("Название документа")),
		mcp.WithString("kind", mcp.Description("Вид документа (свободный текст)")),
		mcp.WithObject("since", mcp.Description("Начало периода"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("until", mcp.Description("Конец периода"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("parish", mcp.Description("Приход (текст или ссылка {text, ref?, type?})"), mcp.Properties(textRefObjectProperties())),
		mcp.WithArray("settlements", mcp.WithStringItems(), mcp.Description("Населённые пункты")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, archiveDocumentCreateHandler(archiveDocuments))

	tool = mcp.NewTool(
		"archive_document_update",
		mcp.WithDescription("Изменить документ внутри единицы учёта; результат — JSON обновлённой записи. Обновляются переданные поля; при отсутствии аргумента в вызове (кроме обязательных unit_id/title) соответствующее поле сохраняет текущее значение, явное пустое значение/пустой список — очищает его. Несуществующий unit_id — ошибка тула; sources — при отсутствии в вызове текущие источники сохраняются, пустой массив — очищает их"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("unit_id", mcp.Required(), mcp.Description("id единицы учёта")),
		mcp.WithString("title", mcp.Required(), mcp.Description("Название документа")),
		mcp.WithString("kind", mcp.Description("Вид документа")),
		mcp.WithObject("since", mcp.Description("Начало периода"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("until", mcp.Description("Конец периода"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("parish", mcp.Description("Приход (текст или ссылка {text, ref?, type?})"), mcp.Properties(textRefObjectProperties())),
		mcp.WithArray("settlements", mcp.WithStringItems(), mcp.Description("Населённые пункты")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, archiveDocumentUpdateHandler(archiveDocuments))

	tool = mcp.NewTool(
		"archive_document_delete",
		mcp.WithDescription("Удалить документ внутри единицы учёта. Необратимо. Если на него есть строгие ссылки от других сущностей — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, archiveDocumentDeleteHandler(archiveDocuments))
}

func archiveDocumentListHandler(archiveDocuments ArchiveDocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		page := models.Page{}

		var err error

		if page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := archiveDocuments.ListArchiveDocuments(ctx, AccessFromContext(ctx), page)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.ArchiveDocumentsFromModels(list))
	}
}

func archiveDocumentSearchHandler(archiveDocuments ArchiveDocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := archiveDocuments.SearchArchiveDocuments(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.ArchiveDocumentsFromModels(list))
	}
}

func archiveDocumentGetHandler(archiveDocuments ArchiveDocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		d, err := archiveDocuments.GetArchiveDocument(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.ArchiveDocumentFromModel(d))
	}
}

func archiveDocumentCreateHandler(archiveDocuments ArchiveDocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

		parish, err := optionalTextRef(args, "parish")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		since, err := optionalFactDate(args, "since")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		until, err := optionalFactDate(args, "until")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sources, err := optionalSourceLinks(args, "sources")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		d := models.ArchiveDocument{
			UnitID:      models.ID(req.GetString("unit_id", "")),
			Title:       req.GetString("title", ""),
			Kind:        req.GetString("kind", ""),
			Since:       since,
			Until:       until,
			Parish:      parish,
			Settlements: textRefsFromStrings(req.GetStringSlice("settlements", nil)),
			Notes:       textRefsFromStrings(req.GetStringSlice("notes", nil)),
			Sources:     sources,
			Private:     req.GetBool("private", false),
		}

		created, err := archiveDocuments.CreateArchiveDocument(ctx, d)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.ArchiveDocumentFromModel(created))
	}
}

func archiveDocumentUpdateHandler(archiveDocuments ArchiveDocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := archiveDocuments.GetArchiveDocument(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		args := req.GetArguments()

		cur.UnitID = models.ID(req.GetString("unit_id", ""))
		cur.Title = req.GetString("title", "")

		if raw, ok := args["kind"]; ok && raw != nil {
			cur.Kind = req.GetString("kind", "")
		}

		if raw, ok := args["since"]; ok && raw != nil {
			since, err := optionalFactDate(args, "since")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Since = since
		}

		if raw, ok := args["until"]; ok && raw != nil {
			until, err := optionalFactDate(args, "until")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Until = until
		}

		if raw, ok := args["parish"]; ok && raw != nil {
			parish, err := optionalTextRef(args, "parish")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Parish = parish
		}

		if raw, ok := args["settlements"]; ok && raw != nil {
			cur.Settlements = textRefsFromStrings(req.GetStringSlice("settlements", nil))
		}

		if raw, ok := args["notes"]; ok && raw != nil {
			cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))
		}

		if raw, ok := args["private"]; ok && raw != nil {
			cur.Private = req.GetBool("private", false)
		}

		if raw, ok := args["sources"]; ok && raw != nil {
			sources, err := optionalSourceLinks(args, "sources")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Sources = sources
		}

		if err := archiveDocuments.UpdateArchiveDocument(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.ArchiveDocumentFromModel(cur))
	}
}

func archiveDocumentDeleteHandler(archiveDocuments ArchiveDocumentService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := archiveDocuments.DeleteArchiveDocument(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
