import { useEffect, useState } from "react";
import { Button, Empty, Modal, Select, Space, Tree, Typography } from "antd";
import type { DataNode, EventDataNode } from "antd/es/tree";
import {
  fetchArchiveDocuments,
  fetchArchiveNodes,
  fetchArchives,
  MAX_PAGE_LIMIT,
  type Archive,
  type ArchiveDocument,
  type ArchiveNode,
} from "./api";

function nodeLabel(n: ArchiveNode): string {
  return n.label || n.name || n.id;
}

function toTreeNode(n: ArchiveNode): DataNode {
  return { key: n.id, title: nodeLabel(n) };
}

// updateTreeData — та же иммутабельная рекурсивная подстановка детей узла
// key в дерево antd Tree, что DivisionsList.tsx (скопировано как есть — уже
// проверенная логика, docs/data-model/entity-write.md §3.4).
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

// useArchiveOptions — заполняет Select архивов, по образцу
// useRepositoryOptions (ArchiveForm.tsx). Экспортирован — переиспользуется
// ArchiveNodesList.tsx (тот же выбор архива наверху страницы).
export function useArchiveOptions() {
  const [archives, setArchives] = useState<Archive[]>([]);

  useEffect(() => {
    fetchArchives({ limit: 500 })
      .then(setArchives)
      .catch(() => setArchives([]));
  }, []);

  return archives.map((a) => ({ value: a.id, label: a.name }));
}

// ArchiveNodePicker — общий picker для строгих ссылок на ArchiveNode. Четыре
// места использования: ArchiveNodeForm/ArchiveNodeView (parent_id),
// ArchiveDocumentForm/ArchiveDocumentView (unit_id), ретрофит
// AttachmentForm/AttachmentView (node_id) и AnchorEditor (archive-kind
// node_id) — docs/data-model/entity-write.md §3.5.
//
// Кнопка открывает Modal. Если archiveId не передан пропом — сначала нужно
// выбрать архив (Select), пока архив не выбран — дерево не рендерится
// ("Сначала выберите архив"). Если archiveId уже известен вызывающей стороне
// (например, ArchiveNodeView — переродительствование только в пределах
// своего же архива) — шаг выбора архива пропускается, дерево сразу scoped к
// этому archiveId.
//
// Дерево — тот же приём, что DivisionsList.tsx: root = fetchArchiveNodes с
// parent_id не заданным — бэк (list_archive_nodes) уже отдаёт только корни
// архива в этом случае, клиентский фильтр до parent_id == null — no-op,
// подстраховка на будущее, а не реальная фильтрация; окна listим до
// короткой страницы, тот же MAX_PAGE_LIMIT-приём, что DivisionsList.loadRoot,
// дети — по клику раскрывашки через loadData/onLoadData, где бэк фильтрует
// по parent_id напрямую.
export function ArchiveNodePicker({
  value,
  label,
  onChange,
  archiveId,
}: {
  value: string;
  label?: string;
  onChange: (id: string, label: string) => void;
  archiveId?: string;
}) {
  const [open, setOpen] = useState(false);
  const [pickedArchiveId, setPickedArchiveId] = useState<string | null>(archiveId ?? null);
  const [treeData, setTreeData] = useState<DataNode[]>([]);
  const [loading, setLoading] = useState(false);
  const archiveOptions = useArchiveOptions();

  const effectiveArchiveId = archiveId ?? pickedArchiveId;

  const loadRoot = async (forArchiveId: string) => {
    setLoading(true);
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
    } catch {
      setTreeData([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (open && effectiveArchiveId != null) {
      loadRoot(effectiveArchiveId);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, effectiveArchiveId]);

  const onLoadData = async (node: EventDataNode<DataNode>) => {
    if (effectiveArchiveId == null) {
      return;
    }
    const children = await fetchArchiveNodes({
      archive_id: effectiveArchiveId,
      parent_id: String(node.key),
      limit: MAX_PAGE_LIMIT,
    });
    setTreeData((prev) => updateTreeData(prev, String(node.key), children.map(toTreeNode)));
  };

  const onSelect = (keys: React.Key[], info: { node: DataNode }) => {
    if (keys.length > 0) {
      onChange(String(keys[0]), String(info.node.title ?? keys[0]));
      setOpen(false);
    }
  };

  const openModal = () => {
    setPickedArchiveId(archiveId ?? null);
    setTreeData([]);
    setOpen(true);
  };

  return (
    <>
      {value ? (
        <Space>
          <Typography.Text>Текущий узел: {label ?? value}</Typography.Text>
          <Button size="small" onClick={openModal}>
            Изменить
          </Button>
        </Space>
      ) : (
        <Button type="dashed" onClick={openModal}>
          Выбрать узел
        </Button>
      )}
      <Modal
        title="Выбор архивного узла"
        open={open}
        onCancel={() => setOpen(false)}
        footer={<Button onClick={() => setOpen(false)}>Закрыть</Button>}
        destroyOnHidden
      >
        <Space direction="vertical" style={{ width: "100%" }}>
          {archiveId == null && (
            <Select
              style={{ width: "100%" }}
              placeholder="Выберите архив"
              options={archiveOptions}
              value={pickedArchiveId ?? undefined}
              onChange={(v) => {
                setPickedArchiveId(v);
                setTreeData([]);
              }}
              showSearch
              filterOption={(input, option) =>
                (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
              }
            />
          )}
          {effectiveArchiveId == null ? (
            <Empty description="Сначала выберите архив" />
          ) : loading ? (
            <Typography.Text type="secondary">Загрузка…</Typography.Text>
          ) : (
            <Tree treeData={treeData} loadData={onLoadData} onSelect={onSelect} showLine />
          )}
        </Space>
      </Modal>
    </>
  );
}

// ArchiveDocumentSelect — плоский searchable Select документов внутри
// заданного узла: unit_id === nodeId, отфильтровано на клиенте (тот же
// приём "flat Select, лимит 500, не picker", что у SourceLinkListEditor's
// Select цитат — ArchiveDocument не иерархична, docs/data-model/
// entity-write.md §3.5). Используется рядом с ArchiveNodePicker для
// необязательного document_id-подполя (Attachment.document_id,
// Anchor.document_id).
export function ArchiveDocumentSelect({
  nodeId,
  value,
  onChange,
}: {
  nodeId: string | null | undefined;
  value: string | undefined;
  onChange: (id: string | undefined) => void;
}) {
  const [documents, setDocuments] = useState<ArchiveDocument[]>([]);

  useEffect(() => {
    fetchArchiveDocuments({ limit: MAX_PAGE_LIMIT })
      .then(setDocuments)
      .catch(() => setDocuments([]));
  }, []);

  const options = documents
    .filter((d) => nodeId != null && d.unit_id === nodeId)
    .map((d) => ({ value: d.id, label: `${d.title} (${d.kind || "без вида"})` }));

  return (
    <Select
      style={{ width: 300 }}
      placeholder={nodeId == null ? "Сначала выберите узел" : "Документ (необязательно)"}
      allowClear
      disabled={nodeId == null}
      options={options}
      value={value || undefined}
      onChange={(v) => onChange(v)}
      showSearch
      filterOption={(input, option) =>
        (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
      }
    />
  );
}
