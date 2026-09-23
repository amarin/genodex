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

// DivisionsList — «Административное деление»: дерево от корня (parent_id не
// передан ⇒ models.DivisionQuery.ParentID == nil ⇒ бэк отдаёт корень,
// internal/models/query.go:67), дети подгружаются по клику на
// раскрывашку. Поиск по названию временно подменяет дерево плоским списком
// найденного (как SettlementsTab). Клик по названию узла — переход на
// View (/divisions/:id).
export default function DivisionsList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [treeData, setTreeData] = useState<DataNode[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<AdminDivision[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  const loadRoot = () => {
    setLoading(true);
    setError(null);
    fetchAdminDivisions({ limit: MAX_PAGE_LIMIT })
      .then((items) => setTreeData(items.map(toTreeNode)))
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    loadRoot();
  }, []);

  const onLoadData = async (node: EventDataNode<DataNode>) => {
    const children = await fetchAdminDivisions({
      parent_id: String(node.key),
      limit: MAX_PAGE_LIMIT,
    });
    setTreeData((prev) => updateTreeData(prev, String(node.key), children.map(toTreeNode)));
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
          loadRoot();
          navigate(`/divisions/${d.id}`);
        }}
      />
    </Card>
  );
}
