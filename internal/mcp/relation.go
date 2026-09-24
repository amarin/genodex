package mcp

import (
	"context"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/amarin/genodex/internal/models"
	"github.com/amarin/genodex/internal/transport"
)

// registerRelationTools регистрирует тулы для работы с рёбрами графа
// родства. Без relation_search — search_relations намеренно не заводится
// (у Relation нет собственных поисковых полей, индекс пуст, docs/data-model/
// entity-write.md §3.8); relation_list принимает необязательный person_id —
// более полезная замена (ребро проходит, если совпадает с person_a ИЛИ
// person_b). person_a/person_b/kind — REQUIRED и в relation_update
// (безусловная полная замена, как citation_update's source_id) — это ядро
// того, что представляет собой ребро, preserve-on-omit тут неуместен.
// rel_type — необязателен (его условная обязательность при kind=associate
// проверяется моделью, не MCP-схемой), обычный preserve-on-omit.
// since/until — одиночные объектные поля (*FactDate): presence-ONLY guard
// (явный null очищает, отсутствие ключа сохраняет) — см. цитированный в
// задаче фикс в family.go/archive_node.go.
func registerRelationTools(s *server.MCPServer, relations RelationService) {
	tool := mcp.NewTool(
		"relation_list",
		mcp.WithDescription("Список рёбер графа родства в порядке сохранения; результат короче размера окна — конец списка"),
		mcp.WithString("person_id", mcp.Description("Необязательный фильтр: только рёбра, где эта персона — person_a или person_b")),
		mcp.WithNumber("limit", mcp.Description("Размер окна (по умолчанию 50, не больше 500)")),
		mcp.WithNumber("offset", mcp.Description("Сдвиг окна (по умолчанию 0)")),
	)
	s.AddTool(tool, relationListHandler(relations))

	tool = mcp.NewTool(
		"relation_get",
		mcp.WithDescription("Ребро графа родства по id; результат — JSON записи. Неверный формат id или отсутствующая/приватная (для не-владельца) запись — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи, например RL-01ARZ3NDEKTSV4RRFFQ69G5FA9")),
	)
	s.AddTool(tool, relationGetHandler(relations))

	tool = mcp.NewTool(
		"relation_create",
		mcp.WithDescription("Создать ребро графа родства; id генерируется сервером; результат — JSON созданной записи. Несуществующие person_a/person_b — ошибка тула"),
		mcp.WithString("kind", mcp.Required(), mcp.Enum("blood", "marriage", "adoption", "associate"), mcp.Description("Вид связи")),
		mcp.WithString("rel_type", mcp.Description("Вид связи для kind=associate (обязателен только при этом kind, иначе должен отсутствовать)")),
		mcp.WithString("person_a", mcp.Required(), mcp.Description("id первой персоны")),
		mcp.WithString("person_b", mcp.Required(), mcp.Description("id второй персоны (должна отличаться от person_a)")),
		mcp.WithObject("since", mcp.Description("Начало периода действия связи"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("until", mcp.Description("Конец периода действия связи"), mcp.Properties(factDateObjectProperties())),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, relationCreateHandler(relations))

	tool = mcp.NewTool(
		"relation_update",
		mcp.WithDescription("Изменить ребро графа родства: kind/person_a/person_b заменяются безусловно при каждом вызове (ядро того, что представляет собой ребро); rel_type/sources/notes/private — при отсутствии аргумента сохраняют текущее значение, явное пустое значение/пустой список — очищает; since/until — одиночные объектные поля: отсутствие ключа сохраняет текущее значение, явный null — очищает (пустой объект {} для очистки не подходит — не проходит валидацию); результат — JSON обновлённой записи. Внимание: при смене kind на значение, отличное от associate, без явной передачи rel_type: \"\" в этом же вызове — обновление упадёт на валидации (rel_type сохраняет прежнее непустое значение по preserve-on-omit, а модель отвергает непустой rel_type при kind != associate); клиент, меняющий kind с associate на другой вид, должен явно очистить rel_type тем же вызовом"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
		mcp.WithString("kind", mcp.Required(), mcp.Enum("blood", "marriage", "adoption", "associate"), mcp.Description("Вид связи")),
		mcp.WithString("rel_type", mcp.Description("Вид связи для kind=associate")),
		mcp.WithString("person_a", mcp.Required(), mcp.Description("id первой персоны")),
		mcp.WithString("person_b", mcp.Required(), mcp.Description("id второй персоны")),
		mcp.WithObject("since", mcp.Description("Начало периода действия связи"), mcp.Properties(factDateObjectProperties())),
		mcp.WithObject("until", mcp.Description("Конец периода действия связи"), mcp.Properties(factDateObjectProperties())),
		mcp.WithArray("sources", mcp.Items(map[string]any{
			"type":       "object",
			"properties": sourceLinkObjectProperties(),
			"required":   []string{"citation_id"},
		}), mcp.Description("Доказательства (ссылки на цитаты)")),
		mcp.WithArray("notes", mcp.WithStringItems(), mcp.Description("Заметки")),
		mcp.WithBoolean("private", mcp.Description("Приватность записи")),
	)
	s.AddTool(tool, relationUpdateHandler(relations))

	tool = mcp.NewTool(
		"relation_delete",
		mcp.WithDescription("Удалить ребро графа родства. Необратимо. Если на него есть строгие ссылки — ошибка тула"),
		mcp.WithString("id", mcp.Required(), mcp.Description("id записи")),
	)
	s.AddTool(tool, relationDeleteHandler(relations))
}

func relationListHandler(relations RelationService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		q := models.RelationQuery{}

		if pid := req.GetString("person_id", ""); pid != "" {
			id := models.ID(pid)
			q.PersonID = &id
		}

		var err error

		if q.Page.Limit, err = optionalInt(req, "limit"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		if q.Page.Offset, err = optionalInt(req, "offset"); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		list, err := relations.ListRelations(ctx, AccessFromContext(ctx), q)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить список: %v", err)), nil
		}

		return toolJSONResult(transport.RelationsFromModels(list))
	}
}

func relationGetHandler(relations RelationService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		r, err := relations.GetRelation(ctx, AccessFromContext(ctx), models.ID(req.GetString("id", "")))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить запись: %v", err)), nil
		}

		return toolJSONResult(transport.RelationFromModel(r))
	}
}

func relationCreateHandler(relations RelationService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()

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

		r := models.Relation{
			Kind:    models.RelationKind(req.GetString("kind", "")),
			RelType: models.RelationType(req.GetString("rel_type", "")),
			PersonA: models.ID(req.GetString("person_a", "")),
			PersonB: models.ID(req.GetString("person_b", "")),
			Since:   since,
			Until:   until,
			Sources: sources,
			Notes:   textRefsFromStrings(req.GetStringSlice("notes", nil)),
			Private: req.GetBool("private", false),
		}

		created, err := relations.CreateRelation(ctx, r)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось создать запись: %v", err)), nil
		}

		return toolJSONResult(transport.RelationFromModel(created))
	}
}

func relationUpdateHandler(relations RelationService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		cur, err := relations.GetRelation(ctx, AccessFromContext(ctx), id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось получить текущую версию: %v", err)), nil
		}

		args := req.GetArguments()

		cur.Kind = models.RelationKind(req.GetString("kind", ""))
		cur.PersonA = models.ID(req.GetString("person_a", ""))
		cur.PersonB = models.ID(req.GetString("person_b", ""))

		if raw, ok := args["rel_type"]; ok && raw != nil {
			cur.RelType = models.RelationType(req.GetString("rel_type", ""))
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

		if raw, ok := args["sources"]; ok && raw != nil {
			sources, err := optionalSourceLinks(args, "sources")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			cur.Sources = sources
		}

		if raw, ok := args["notes"]; ok && raw != nil {
			cur.Notes = textRefsFromStrings(req.GetStringSlice("notes", nil))
		}

		if raw, ok := args["private"]; ok && raw != nil {
			cur.Private = req.GetBool("private", false)
		}

		if err := relations.UpdateRelation(ctx, cur); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось сохранить изменения: %v", err)), nil
		}

		return toolJSONResult(transport.RelationFromModel(cur))
	}
}

func relationDeleteHandler(relations RelationService) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := models.ID(req.GetString("id", ""))

		if err := relations.DeleteRelation(ctx, id); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("не удалось удалить запись: %v", err)), nil
		}

		return mcp.NewToolResultText(fmt.Sprintf("запись %q удалена", id)), nil
	}
}
