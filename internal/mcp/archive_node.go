package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// registerArchiveNodeTools регистрирует тулы для работы с узлами архивного
// дерева. Дерево скопировано по архиву (в отличие от AdministrativeDivision
// — единого глобального дерева): archive_node_list требует archive_id;
// parent_id — необязательный прямой родитель (пусто — корень внутри
// архива). archive_node_create/update дополнительно проверяют, что
// parent_id (если задан) принадлежит тому же архиву, что и сам узел —
// ошибка тула на поле parent_id, если нет.
func registerArchiveNodeTools(s *server.MCPServer, archiveNodes ArchiveNodeService) {
	tool := mcp.NewTool(
		"archive_node_list",
		mcp.WithDescription("Список узлов архивного дерева заданного архива в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithString("archive_id", mcp.Required(), mcp.Description("id архива (обязателен — у узла нет смысла вне архива)")),
		mcp.WithString("parent_id", mcp.Description("id родительского узла; пусто — корень дерева внутри архива")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, archiveNodeListHandler(archiveNodes))

	tool = mcp.NewTool(
		"archive_node_search",
		mcp.WithDescription("Поиск узлов архивного дерева по началу метки/названия, среди всех архивов (без сужения по archive_id); результат — JSON-массив записей. Пустой q — пустой результат"),
		mcp.WithString("q", mcp.Required(), mcp.Description("Начало метки или названия")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, archiveNodeSearchHandler(archiveNodes))

	tool = mcp.NewTool(
		"archive_node_get",
		mcp.WithDescription("Узел архивного дерева по id; результат — JSON записи. Неверный формат id или отсутствующая запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например AN-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, archiveNodeGetHandler(archiveNodes))

	tool = mcp.NewTool(
		"archive_node_create",
		mcp.WithDescription("Создать узел архивного дерева; id генерируется сервером; результат — JSON созданной записи. Несуществующий archive_id или parent_id, либо parent_id из другого архива — ошибка тула"),
		mcp.WithString("type", mcp.Required(), mcp.Description("Уровень узла в системе иерархии архива (fond, opis, delo, …) — открытый список")),
		mcp.WithString("archive_id", mcp.Required(), mcp.Description("id архива")),
		mcp.WithString("parent_id", mcp.Description("id родительского узла (должен принадлежать тому же архиву); пусто — корень")),
		mcp.WithString("label", mcp.Required(), mcp.Description("Шифр/метка узла")),
		mcp.WithString("name", mcp.Description("Название")),
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
	s.AddTool(tool, archiveNodeCreateHandler(archiveNodes))

	tool = mcp.NewTool(
		"archive_node_update",
		mcp.WithDescription("Изменить узел архивного дерева; результат — JSON обновлённой записи. Обновляются переданные поля; при отсутствии аргумента в вызове (кроме обязательных type/archive_id/label) соответствующее поле сохраняет текущее значение, явное пустое значение/пустой список — очищает его (для parent_id явно пустое значение означает «сделать корнем»). Несуществующий archive_id/parent_id, parent_id из другого архива или цикл по parent_id — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("type", mcp.Required(), mcp.Description("Уровень узла")),
		mcp.WithString("archive_id", mcp.Required(), mcp.Description("id архива")),
		mcp.WithString("parent_id", mcp.Description("id родительского узла; пусто — корень")),
		mcp.WithString("label", mcp.Required(), mcp.Description("Шифр/метка узла")),
		mcp.WithString("name", mcp.Description("Название")),
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
	s.AddTool(tool, archiveNodeUpdateHandler(archiveNodes))

	tool = mcp.NewTool(
		"archive_node_delete",
		mcp.WithDescription("Удалить узел архивного дерева. Необратимо. Если на него ссылаются дочерние узлы или документы — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, archiveNodeDeleteHandler(archiveNodes))
}

func archiveNodeListHandler(archiveNodes ArchiveNodeService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.ArchiveNodeQuery{ArchiveID: models.ID(req.GetString("archive_id", ""))}

		if raw := req.GetString("parent_id", ""); raw != "" {
			id := models.ID(raw)
			q.ParentID = &id
		}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := archiveNodes.ListArchiveNodes(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.ArchiveNodesFromModels(list))
	}
}

func archiveNodeSearchHandler(archiveNodes ArchiveNodeService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.SearchQuery{Text: req.GetString("q", "")}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := archiveNodes.SearchArchiveNodes(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось выполнить поиск: %v", err)), nil
		}

		return toolJSONResult(transport.ArchiveNodesFromModels(list))
	}
}

func archiveNodeGetHandler(archiveNodes ArchiveNodeService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		n, err := archiveNodes.GetArchiveNode(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.ArchiveNodeFromModel(n))
	}
}

func archiveNodeCreateHandler(archiveNodes ArchiveNodeService) server.ToolHandlerFunc {
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

		n := models.ArchiveNode{
			Type:        models.ArchiveNodeType(req.GetString("type", "")),
			ArchiveID:   models.ID(req.GetString("archive_id", "")),
			ParentID:    optionalArchiveNodeParentID(req),
			Label:       req.GetString("label", ""),
			Name:        req.GetString("name", ""),
			Since:       since,
			Until:       until,
			Parish:      parish,
			Settlements: textRefsFromStrings(req.GetStringSlice("settlements", nil)),
			Notes:       textRefsFromStrings(req.GetStringSlice("notes", nil)),
			Sources:     sources,
			Private:     req.GetBool("private", false),
		}

		created, err := archiveNodes.CreateArchiveNode(ctx, n)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.ArchiveNodeFromModel(created))
	}
}

func archiveNodeUpdateHandler(archiveNodes ArchiveNodeService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := archiveNodes.GetArchiveNode(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		args := req.GetArguments()

		cur.Type = models.ArchiveNodeType(req.GetString("type", ""))
		cur.ArchiveID = models.ID(req.GetString("archive_id", ""))

		if raw, ok := args["parent_id"]; ok && raw != nil {
			cur.ParentID = optionalArchiveNodeParentID(req)
		}

		cur.Label = req.GetString("label", "")

		if raw, ok := args["name"]; ok && raw != nil {
			cur.Name = req.GetString("name", "")
		}

		if _, ok := args["since"]; ok {
			since, err := optionalFactDate(args, "since")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Since = since
		}

		if _, ok := args["until"]; ok {
			until, err := optionalFactDate(args, "until")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Until = until
		}

		if _, ok := args["parish"]; ok {
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

		if err := archiveNodes.UpdateArchiveNode(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.ArchiveNodeFromModel(cur))
	}
}

func archiveNodeDeleteHandler(archiveNodes ArchiveNodeService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := archiveNodes.DeleteArchiveNode(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}

// optionalArchiveNodeParentID читает необязательный parent_id; пустая строка
// (в т.ч. явный null) — корень (nil-указатель). По образцу
// optionalParentID у division.go.
func optionalArchiveNodeParentID(req mcp.CallToolRequest) *models.ID {
	raw := req.GetString("parent_id", "")
	if raw == "" {
		return nil
	}

	id := models.ID(raw)

	return &id
}
