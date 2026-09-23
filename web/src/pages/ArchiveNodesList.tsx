import { useEffect, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Empty, Input, List, Select, Spin, Tree, Typography } from "antd";
import type { DataNode, EventDataNode } from "antd/es/tree";
import { fetchArchiveNodes, searchArchiveNodes, MAX_PAGE_LIMIT, type ArchiveNode } from "../api";
import { useArchiveOptions } from "../ArchiveNodePicker";
import { useSession } from "../session";
import { CreateArchiveNodeModal } from "./ArchiveNodeForm";

function nodeLabel(n: ArchiveNode): string {
  return n.label || n.name || n.id;
}

function toTreeNode(n: ArchiveNode): DataNode {
  return { key: n.id, title: nodeLabel(n) };
}

// updateTreeData — та же иммутабельная рекурсивная подстановка детей узла
// key, что DivisionsList.tsx/ArchiveNodePicker.tsx (скопировано как есть).
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

// ArchiveNodesList — «Архивные единицы»: тот же tree-паттерн, что
// DivisionsList.tsx (Tree + loadData + поиск-подменяет-дерево), но с
// scoping по обязательному архиву — ArchiveNode не одно глобальное дерево, а
// по одному на архив (docs/data-model/entity-write.md §3.4). Архив читается
// из ?archive_id= URL-параметра (так ArchiveView может вести прямо в дерево
// своего архива, см. "Архивные единицы →" на ArchiveView.tsx) либо
// выбирается тут же Select'ом (useArchiveOptions — тот же хук, что
// ArchiveNodePicker). Пока архив не выбран/не известен — пустое состояние
// вместо дерева.
export default function ArchiveNodesList() {
  const navigate = useNavigate();
  const { session } = useSession();
  const [searchParams, setSearchParams] = useSearchParams();
  const archiveOptions = useArchiveOptions();

  const [archiveId, setArchiveId] = useState<string | null>(searchParams.get("archive_id"));
  const [treeData, setTreeData] = useState<DataNode[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [searchResults, setSearchResults] = useState<ArchiveNode[] | null>(null);
  const [searching, setSearching] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);

  // loadRoot — как DivisionsList.loadRoot: бэк без parent_id отдаёт узлы
  // окнами (limit/offset), не «только корни», поэтому фильтруем на клиенте
  // по parent_id == null и проходим все страницы до конца.
  const loadRoot = async (forArchiveId: string) => {
    setLoading(true);
    setError(null);
    try {
      const roots: ArchiveNode[] = [];
      let offset = 0;
      for (;;) {
        const page = await fetchArchiveNodes({
          archive_id: forArchiveId,
          limit: MAX_PAGE_LIMIT,
          offset,
        });
        roots.push(...page.filter((n) => n.parent_id == null));
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
    setSearchResults(null);
    setError(null);
    if (archiveId != null) {
      loadRoot(archiveId);
    } else {
      setTreeData([]);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [archiveId]);

  const onArchiveChange = (v: string) => {
    setArchiveId(v);
    setSearchParams({ archive_id: v });
  };

  const onLoadData = async (node: EventDataNode<DataNode>) => {
    if (archiveId == null) {
      return;
    }
    try {
      const children = await fetchArchiveNodes({
        archive_id: archiveId,
        parent_id: String(node.key),
        limit: MAX_PAGE_LIMIT,
      });
      setTreeData((prev) => updateTreeData(prev, String(node.key), children.map(toTreeNode)));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Не удалось загрузить дочерние узлы");
      throw e;
    }
  };

  const onSelect = (keys: React.Key[]) => {
    if (keys.length > 0) {
      navigate(`/archive-nodes/${keys[0]}`);
    }
  };

  // onSearch — archive-nodes/search ГЛОБАЛЬНЫЙ (не ограничен archive_id, см.
  // searchArchiveNodes в api.ts) — результаты, найденные в других архивах,
  // здесь не релевантны (страница показывает дерево ОДНОГО выбранного
  // архива), поэтому фильтруем на клиенте до archiveId после запроса.
  const onSearch = (value: string) => {
    const q = value.trim();
    if (!q || archiveId == null) {
      setSearchResults(null);
      return;
    }
    setSearching(true);
    setError(null);
    searchArchiveNodes({ q, limit: MAX_PAGE_LIMIT })
      .then((results) => setSearchResults(results.filter((n) => n.archive_id === archiveId)))
      .catch((e: Error) => setError(e.message))
      .finally(() => setSearching(false));
  };

  const onSearchChange = (value: string) => {
    if (value.trim() === "") {
      setSearchResults(null);
    }
  };

  return (
    <>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[{ title: <Link to="/">Сущности</Link> }, { title: "Архивные единицы" }]}
      />
      <Card
        title="Архивные единицы"
        extra={
          session != null && archiveId != null ? (
            <Button type="primary" onClick={() => setCreateOpen(true)}>
              + добавить в корень
            </Button>
          ) : undefined
        }
      >
        <Select
          style={{ width: "100%", marginBottom: 16 }}
          placeholder="Выберите архив"
          options={archiveOptions}
          value={archiveId ?? undefined}
          onChange={onArchiveChange}
          showSearch
          filterOption={(input, option) =>
            (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
          }
        />
        {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
        {archiveId == null ? (
          <Empty description="Сначала выберите архив" />
        ) : (
          <>
            <Input.Search
              placeholder="Поиск по метке/названию…"
              allowClear
              enterButton
              loading={searching}
              onSearch={onSearch}
              onChange={(e) => onSearchChange(e.target.value)}
              style={{ marginBottom: 16 }}
            />
            {searchResults != null ? (
              <List
                dataSource={searchResults}
                locale={{ emptyText: "Найдено пусто" }}
                renderItem={(n) => (
                  <List.Item>
                    <Typography.Link onClick={() => navigate(`/archive-nodes/${n.id}`)}>
                      {nodeLabel(n)}
                    </Typography.Link>
                  </List.Item>
                )}
              />
            ) : loading ? (
              <Spin />
            ) : (
              <Tree treeData={treeData} loadData={onLoadData} onSelect={onSelect} showLine />
            )}
          </>
        )}
        {archiveId != null && (
          <CreateArchiveNodeModal
            open={createOpen}
            archiveId={archiveId}
            parentId={null}
            onClose={() => setCreateOpen(false)}
            onCreated={(n) => {
              setCreateOpen(false);
              navigate(`/archive-nodes/${n.id}`);
            }}
          />
        )}
      </Card>
    </>
  );
}
