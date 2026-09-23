import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { Alert, Button, Card, Input, List, Spin, Tree, Typography } from "antd";
import type { DataNode, EventDataNode } from "antd/es/tree";
import {
  adminDivisionTypeLabel,
  fetchAdminDivisions,
  searchAdminDivisions,
  MAX_PAGE_LIMIT,
  type AdminDivision,
} from "../api";
import { useSession } from "../session";
import { CreateDivisionModal } from "./DivisionForm";

function divisionLabel(d: AdminDivision): string {
  return `${d.name} (${adminDivisionTypeLabel(d.type)})`;
}

function toTreeNode(d: AdminDivision): DataNode {
  return { key: d.id, title: divisionLabel(d) };
}

// updateTreeData — иммутабельно подставляет загруженных детей узла key в
// дерево antd Tree (рекурсивно, узел может быть на любой глубине).
function updateTreeData(list: DataNode[], key: string, children: DataNode[]): DataNode[] {
  return list.map((node) => {
    if (node.key === key) {
      return { ...node, children, isLeaf: children.length === 0 };
    }
    if (node.children != null) {
      return { ...node, children: updateTreeData(node.children, key, children) };
    }
    return node;
  });
}

// DivisionsList — «Административное деление»: дерево от корня. Без
// parent_id бэк (internal/usecases/list_divisions/scenario.go:31-33)
// отдаёт ВЕСЬ список без фильтра по родителю (комментарий
// "nil — корень" в internal/models/query.go:67 вводит в заблуждение —
// настоящей фильтрации на корень там нет), поэтому корень фильтруем на
// клиенте по parent_id === null. Дети подгружаются по клику на
// раскрывашку — там parent_id передаётся явно, и бэк (listChildren, та
// же scenario.go) фильтрует по-настоящему. Поиск по названию временно
// подменяет дерево плоским списком найденного (как SettlementsTab). Клик
// по названию узла — переход на View (/divisions/:id).
export default function DivisionsList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [treeData, setTreeData] = useState<DataNode[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<AdminDivision[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  // loadRoot — бэк без parent_id отдаёт единицы окнами (limit/offset),
  // отсортированными по порядку вставки, а не по признаку «корень/не
  // корень». Поэтому проходим все страницы до конца (тот же приём, что
  // internal/usecases/list_divisions/scenario.go на бэке), иначе корни,
  // вставленные позже первых MAX_PAGE_LIMIT записей, молча пропадают.
  const loadRoot = async () => {
    setLoading(true);
    setError(null);
    try {
      const roots: AdminDivision[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchAdminDivisions({ limit: MAX_PAGE_LIMIT, offset });
        roots.push(...page.filter((d) => d.parent_id == null));
        if (page.length < MAX_PAGE_LIMIT) {
          break;
        }
        offset += MAX_PAGE_LIMIT;
      }
      setTreeData(roots.map(toTreeNode));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Не удалось загрузить список");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadRoot();
  }, []);

  const onLoadData = async (node: EventDataNode<DataNode>) => {
    try {
      const children = await fetchAdminDivisions({
        parent_id: String(node.key),
        limit: MAX_PAGE_LIMIT,
      });
      setTreeData((prev) => updateTreeData(prev, String(node.key), children.map(toTreeNode)));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Не удалось загрузить дочерние единицы");
      throw e;
    }
  };

  const onSelect = (keys: React.Key[]) => {
    if (keys.length > 0) {
      navigate(`/divisions/${keys[0]}`);
    }
  };

  const onSearch = (value: string) => {
    const q = value.trim();
    if (!q) {
      setSearchResults(null);
      return;
    }
    setSearching(true);
    setError(null);
    searchAdminDivisions({ q, limit: MAX_PAGE_LIMIT })
      .then(setSearchResults)
      .catch((e: Error) => setError(e.message))
      .finally(() => setSearching(false));
  };

  const onSearchChange = (value: string) => {
    if (value.trim() === "") {
      setSearchResults(null);
    }
  };

  return (
    <Card
      title="Административное деление"
      extra={
        session != null ? (
          <Button type="primary" onClick={() => setCreateOpen(true)}>
            + добавить в корень
          </Button>
        ) : undefined
      }
    >
      <Input.Search
        placeholder="Поиск по названию…"
        allowClear
        enterButton
        loading={searching}
        onSearch={onSearch}
        onChange={(e) => onSearchChange(e.target.value)}
        style={{ marginBottom: 16 }}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
      {searchResults != null ? (
        <List
          dataSource={searchResults}
          locale={{ emptyText: "Найдено пусто" }}
          renderItem={(d) => (
            <List.Item>
              <Typography.Link onClick={() => navigate(`/divisions/${d.id}`)}>
                {divisionLabel(d)}
              </Typography.Link>
            </List.Item>
          )}
        />
      ) : loading ? (
        <Spin />
      ) : (
        <Tree treeData={treeData} loadData={onLoadData} onSelect={onSelect} showLine />
      )}
      <CreateDivisionModal
        open={createOpen}
        parentId={null}
        onClose={() => setCreateOpen(false)}
        onCreated={(d) => {
          setCreateOpen(false);
          navigate(`/divisions/${d.id}`);
        }}
      />
    </Card>
  );
}
