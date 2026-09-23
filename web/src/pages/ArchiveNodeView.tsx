import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  Alert,
  Breadcrumb,
  Button,
  Card,
  Checkbox,
  Descriptions,
  Form,
  Input,
  List,
  Modal,
  Popconfirm,
  Space,
  Spin,
  Typography,
} from "antd";
import {
  deleteArchiveNode,
  fetchArchive,
  fetchArchiveNode,
  fetchArchiveNodes,
  updateArchiveNode,
  MAX_PAGE_LIMIT,
  type Archive,
  type ArchiveNode,
  type FactDate,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { ArchiveNodePicker } from "../ArchiveNodePicker";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";
import { FactDateEditor, formatFactDate } from "../FactDateEditor";
import { CreateArchiveNodeModal } from "./ArchiveNodeForm";

function nodeLabel(n: ArchiveNode): string {
  return n.label || n.name || n.id;
}

interface EditFormValues {
  type: string;
  label: string;
  name?: string;
  parishText?: string;
  private?: boolean;
}

// EDIT_FORM_FIELDS — как в DivisionView.tsx: parent_id управляется отдельным
// состоянием (editParentId, не полем antd Form), поэтому 422 на parent_id
// (цикл, чужой архив) попадает в общий saveError, а не в конкретное поле
// формы — тот же выбор, что и у DivisionView.
const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["type", "label"];

function TextRefListView({ items }: { items: TextRef[] }) {
  if (items.length === 0) {
    return <Typography.Text type="secondary">—</Typography.Text>;
  }
  return (
    <Space direction="vertical" size={0}>
      {items.map((t, i) => (
        <Typography.Text key={i}>
          {t.text}
          {t.ref != null && t.ref !== "" && (
            <Typography.Text type="secondary"> → {t.type} {t.ref}</Typography.Text>
          )}
        </Typography.Text>
      ))}
    </Space>
  );
}

function SourceLinkListView({ items }: { items: SourceLink[] }) {
  if (items.length === 0) {
    return <Typography.Text type="secondary">—</Typography.Text>;
  }
  return (
    <List
      size="small"
      dataSource={items}
      renderItem={(s) => (
        <List.Item>
          <Link to={`/citations/${s.citation_id}`}>citation {s.citation_id}</Link>
          {s.role ? ` — ${s.role}` : ""}
        </List.Item>
      )}
    />
  );
}

// ArchiveNodeView — просмотр архивного узла, переключаемый в форму
// редактирования на той же странице (toggle+explicit-save, канонический
// вид — ArchiveView.tsx/DivisionView.tsx). Дополнительно к общему шаблону:
// ссылка на владеющий Archive и, если parent_id задан, на родительский
// узел; поле «Родитель» в режиме редактирования — ArchiveNodePicker, scoped
// к archive_id ЭТОГО узла (переродительствование только в пределах своего
// же архива — тот же инвариант, что и на create, сервер бы всё равно
// отверг чужой архив 422, docs/data-model/entity-write.md §3.4).
export default function ArchiveNodeView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [node, setNode] = useState<ArchiveNode | null>(null);
  const [archive, setArchive] = useState<Archive | null>(null);
  const [parent, setParent] = useState<ArchiveNode | null>(null);
  const [children, setChildren] = useState<ArchiveNode[]>([]);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [editParentId, setEditParentId] = useState<string | null>(null);
  const [editParentLabel, setEditParentLabel] = useState<string | null>(null);
  const [settlements, setSettlements] = useState<TextRef[]>([]);
  const [since, setSince] = useState<FactDate | null>(null);
  const [until, setUntil] = useState<FactDate | null>(null);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [addChildOpen, setAddChildOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (nodeId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setNode(null);
    setArchive(null);
    setParent(null);
    setChildren([]);
    fetchArchiveNode(nodeId)
      .then((n) => {
        setNode(n);
        const archivePromise = fetchArchive(n.archive_id)
          .then(setArchive)
          .catch(() => setArchive(null));
        const parentPromise =
          n.parent_id != null
            ? fetchArchiveNode(n.parent_id)
                .then(setParent)
                .catch(() => setParent(null))
            : Promise.resolve(setParent(null));
        const childrenPromise = fetchArchiveNodes({
          archive_id: n.archive_id,
          parent_id: nodeId,
          limit: MAX_PAGE_LIMIT,
        }).then(setChildren);
        return Promise.all([archivePromise, parentPromise, childrenPromise]);
      })
      .catch((e) => {
        if (e instanceof ApiError && e.status === 404) {
          setNotFound(true);
        } else {
          setError(e instanceof Error ? e.message : "Не удалось загрузить узел");
        }
      })
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    setEditing(false);
    setSaveError(null);
    if (id != null) {
      load(id);
    }
  }, [id]);

  const startEdit = () => {
    if (node == null) {
      return;
    }
    form.setFieldsValue({
      type: node.type,
      label: node.label,
      name: node.name ?? "",
      parishText: node.parish?.text ?? "",
      private: node.private,
    });
    setEditParentId(node.parent_id ?? null);
    setEditParentLabel(parent != null ? nodeLabel(parent) : null);
    setSettlements(node.settlements);
    setSince(node.since ?? null);
    setUntil(node.until ?? null);
    setNotes(node.notes);
    setSources(node.sources);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (node == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const parishText = (values.parishText ?? "").trim();
      const parishHasRef = node.parish?.ref != null && node.parish.ref !== "";
      const updated = await updateArchiveNode(node.id, {
        type: values.type,
        archive_id: node.archive_id,
        parent_id: editParentId,
        label: values.label,
        name: values.name ?? "",
        since,
        until,
        parish: parishHasRef && parishText === node.parish?.text
          ? node.parish
          : parishText
            ? { text: parishText }
            : null,
        settlements,
        notes,
        sources,
        private: values.private ?? false,
      });
      setNode(updated);
      if (updated.parent_id != null) {
        fetchArchiveNode(updated.parent_id)
          .then(setParent)
          .catch(() => setParent(null));
      } else {
        setParent(null);
      }
      setEditing(false);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (EDIT_FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof EditFormValues, errors: [e.message] }]);
      } else {
        setSaveError(e instanceof ApiError ? e.message : "Не удалось сохранить изменения");
      }
    } finally {
      setSaving(false);
    }
  };

  const onDelete = async () => {
    if (node == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteArchiveNode(node.id);
      navigate(
        node.parent_id != null
          ? `/archive-nodes/${node.parent_id}`
          : `/archive-nodes?archive_id=${encodeURIComponent(node.archive_id)}`,
      );
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        setConflict(e.referrers ?? []);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось удалить узел");
      }
    } finally {
      setDeleting(false);
    }
  };

  if (loading) {
    return <Spin />;
  }

  if (notFound) {
    return (
      <Alert
        type="warning"
        showIcon
        message="Узел не найден"
        description="Возможно, его удалили. Вернитесь к списку."
        action={
          <Link to="/archive-nodes">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (node == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to={`/archive-nodes?archive_id=${encodeURIComponent(node.archive_id)}`}>Архивные единицы</Link> },
          ...(parent != null
            ? [{ title: <Link to={`/archive-nodes/${parent.id}`}>{nodeLabel(parent)}</Link> }]
            : []),
          { title: nodeLabel(node) },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={nodeLabel(node)} column={1} bordered size="small">
            <Descriptions.Item label="Тип">{node.type}</Descriptions.Item>
            <Descriptions.Item label="Архив">
              <Link to={`/archives/${node.archive_id}`}>{archive?.name ?? node.archive_id}</Link>
            </Descriptions.Item>
            <Descriptions.Item label="Родитель">
              {parent != null ? (
                <Link to={`/archive-nodes/${parent.id}`}>{nodeLabel(parent)}</Link>
              ) : (
                <Typography.Text type="secondary">корень</Typography.Text>
              )}
            </Descriptions.Item>
            <Descriptions.Item label="Название">{node.name || "—"}</Descriptions.Item>
            <Descriptions.Item label="Приход">
              {node.parish == null ? (
                "—"
              ) : (
                <>
                  {node.parish.text}
                  {node.parish.ref && (
                    <Typography.Text type="secondary"> → {node.parish.type} {node.parish.ref}</Typography.Text>
                  )}
                </>
              )}
            </Descriptions.Item>
            <Descriptions.Item label="Начало периода">{formatFactDate(node.since)}</Descriptions.Item>
            <Descriptions.Item label="Конец периода">{formatFactDate(node.until)}</Descriptions.Item>
            <Descriptions.Item label="Населённые пункты"><TextRefListView items={node.settlements} /></Descriptions.Item>
            <Descriptions.Item label="Заметки"><TextRefListView items={node.notes} /></Descriptions.Item>
            <Descriptions.Item label="Доказательства"><SourceLinkListView items={node.sources} /></Descriptions.Item>
            <Descriptions.Item label="Приватная">{node.private ? "да" : "нет"}</Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Button onClick={() => setAddChildOpen(true)}>+ добавить дочерний узел</Button>
              <Popconfirm
                title={`Удалить «${nodeLabel(node)}»?`}
                description="Действие необратимо."
                okText="Удалить"
                cancelText="Отмена"
                onConfirm={onDelete}
              >
                <Button danger loading={deleting}>
                  Удалить
                </Button>
              </Popconfirm>
            </Space>
          )}
        </>
      ) : (
        <Form form={form} layout="vertical" onFinish={onSave} style={{ maxWidth: 560 }}>
          {saveError != null && (
            <Alert type="error" showIcon message={saveError} style={{ marginBottom: 16 }} />
          )}
          <Form.Item
            name="type"
            label="Тип"
            rules={[{ required: true, whitespace: true, message: "Введите тип" }]}
          >
            <Input placeholder="fond / opis / delo" />
          </Form.Item>
          <Form.Item label="Родитель">
            <ArchiveNodePicker
              value={editParentId ?? ""}
              label={editParentLabel ?? undefined}
              archiveId={node.archive_id}
              onChange={(pid, lbl) => {
                setEditParentId(pid);
                setEditParentLabel(lbl);
              }}
            />
            {editParentId != null && (
              <Button
                size="small"
                style={{ marginLeft: 8 }}
                onClick={() => {
                  setEditParentId(null);
                  setEditParentLabel(null);
                }}
              >
                Сделать корневым
              </Button>
            )}
          </Form.Item>
          <Form.Item
            name="label"
            label="Метка"
            rules={[{ required: true, whitespace: true, message: "Введите метку" }]}
          >
            <Input />
          </Form.Item>
          <Form.Item name="name" label="Название">
            <Input />
          </Form.Item>
          <Form.Item name="parishText" label="Приход (текстом)">
            <Input placeholder="Никольский приход" />
          </Form.Item>
          <Form.Item label="Начало периода">
            <FactDateEditor value={since} onChange={setSince} addLabel="+ дата начала" />
          </Form.Item>
          <Form.Item label="Конец периода">
            <FactDateEditor value={until} onChange={setUntil} addLabel="+ дата конца" />
          </Form.Item>
          <Form.Item label="Населённые пункты">
            <TextRefListEditor value={settlements} onChange={setSettlements} addLabel="+ населённый пункт" />
          </Form.Item>
          <Form.Item label="Заметки">
            <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
          </Form.Item>
          <Form.Item label="Доказательства">
            <SourceLinkListEditor value={sources} onChange={setSources} addLabel="+ доказательство" />
          </Form.Item>
          <Form.Item name="private" valuePropName="checked">
            <Checkbox>Приватная запись</Checkbox>
          </Form.Item>
          <Space>
            <Button type="primary" htmlType="submit" loading={saving}>
              Сохранить
            </Button>
            <Button onClick={cancelEdit}>Отмена</Button>
          </Space>
        </Form>
      )}

      <Typography.Title level={5} style={{ marginTop: 24 }}>
        Дочерние узлы
      </Typography.Title>
      <List
        dataSource={children}
        locale={{ emptyText: "Дочерних узлов нет" }}
        renderItem={(c) => (
          <List.Item>
            <Link to={`/archive-nodes/${c.id}`}>{nodeLabel(c)}</Link>
          </List.Item>
        )}
      />

      <CreateArchiveNodeModal
        open={addChildOpen}
        archiveId={node.archive_id}
        parentId={node.id}
        onClose={() => setAddChildOpen(false)}
        onCreated={() => {
          setAddChildOpen(false);
          fetchArchiveNodes({ archive_id: node.archive_id, parent_id: node.id, limit: MAX_PAGE_LIMIT })
            .then(setChildren)
            .catch(() => {});
        }}
      />

      <Modal
        title="Узел используется"
        open={conflict != null}
        onCancel={() => setConflict(null)}
        footer={<Button onClick={() => setConflict(null)}>Закрыть</Button>}
      >
        <Typography.Paragraph>
          Нельзя удалить — на узел ссылаются другие сущности:
        </Typography.Paragraph>
        <List
          size="small"
          dataSource={conflict ?? []}
          renderItem={(r) => (
            <List.Item>
              {r.type} {r.id}
            </List.Item>
          )}
        />
      </Modal>
    </Card>
  );
}
