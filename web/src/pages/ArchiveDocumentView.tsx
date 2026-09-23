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
  deleteArchiveDocument,
  fetchArchiveDocument,
  fetchArchiveNode,
  updateArchiveDocument,
  type ArchiveDocument,
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

function documentLabel(d: ArchiveDocument): string {
  return d.title || d.id;
}

function nodeLabel(n: ArchiveNode): string {
  return n.label || n.name || n.id;
}

interface EditFormValues {
  title: string;
  kind?: string;
  parishText?: string;
  private?: boolean;
}

// EDIT_FORM_FIELDS — unit_id управляется отдельным состоянием (editUnitId,
// не полем antd Form, как parent_id в ArchiveNodeView.tsx), поэтому 422 на
// unit_id попадает в общий saveError.
const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["title", "kind"];

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

// ArchiveDocumentView — просмотр архивного документа, переключаемый в форму
// редактирования на той же странице (toggle+explicit-save, канонический
// вид — ArchiveView.tsx/NoteView.tsx). Дополнительно к общему шаблону —
// ссылка на владеющий ArchiveNode («Единица хранения»); unit_id в режиме
// редактирования — ArchiveNodePicker БЕЗ заранее известного archiveId (тот
// же выбор, что в ArchiveDocumentForm.tsx — переносить документ можно и в
// узел другого архива, сервер лишь проверяет существование узла, не
// принадлежность архиву, в отличие от ArchiveNode.parent_id).
export default function ArchiveDocumentView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [doc, setDocument] = useState<ArchiveDocument | null>(null);
  const [unit, setUnit] = useState<ArchiveNode | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [editUnitId, setEditUnitId] = useState("");
  const [editUnitLabel, setEditUnitLabel] = useState<string | null>(null);
  const [settlements, setSettlements] = useState<TextRef[]>([]);
  const [since, setSince] = useState<FactDate | null>(null);
  const [until, setUntil] = useState<FactDate | null>(null);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (documentId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setDocument(null);
    setUnit(null);
    fetchArchiveDocument(documentId)
      .then((d) => {
        setDocument(d);
        return fetchArchiveNode(d.unit_id)
          .then(setUnit)
          .catch(() => setUnit(null));
      })
      .catch((e) => {
        if (e instanceof ApiError && e.status === 404) {
          setNotFound(true);
        } else {
          setError(e instanceof Error ? e.message : "Не удалось загрузить документ");
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
    if (doc == null) {
      return;
    }
    form.setFieldsValue({
      title: doc.title,
      kind: doc.kind ?? "",
      parishText: doc.parish?.text ?? "",
      private: doc.private,
    });
    setEditUnitId(doc.unit_id);
    setEditUnitLabel(unit != null ? nodeLabel(unit) : doc.unit_id);
    setSettlements(doc.settlements);
    setSince(doc.since ?? null);
    setUntil(doc.until ?? null);
    setNotes(doc.notes);
    setSources(doc.sources);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (doc == null) {
      return;
    }
    if (!editUnitId) {
      setSaveError("Выберите единицу хранения");
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const parishText = (values.parishText ?? "").trim();
      const parishHasRef = doc.parish?.ref != null && doc.parish.ref !== "";
      const updated = await updateArchiveDocument(doc.id, {
        unit_id: editUnitId,
        title: values.title,
        kind: values.kind ?? "",
        since,
        until,
        parish: parishHasRef && parishText === doc.parish?.text
          ? doc.parish
          : parishText
            ? { text: parishText }
            : null,
        settlements,
        notes,
        sources,
        private: values.private ?? false,
      });
      setDocument(updated);
      fetchArchiveNode(updated.unit_id)
        .then(setUnit)
        .catch(() => setUnit(null));
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
    if (doc == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteArchiveDocument(doc.id);
      navigate("/archive-documents");
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        setConflict(e.referrers ?? []);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось удалить документ");
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
        message="Документ не найден"
        description="Возможно, его удалили. Вернитесь к списку."
        action={
          <Link to="/archive-documents">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (doc == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/archive-documents">Архивные документы</Link> },
          { title: documentLabel(doc) },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={documentLabel(doc)} column={1} bordered size="small">
            <Descriptions.Item label="Вид">{doc.kind || "—"}</Descriptions.Item>
            <Descriptions.Item label="Единица хранения">
              <Link to={`/archive-nodes/${doc.unit_id}`}>{unit != null ? nodeLabel(unit) : doc.unit_id}</Link>
            </Descriptions.Item>
            <Descriptions.Item label="Приход">
              {doc.parish == null ? (
                "—"
              ) : (
                <>
                  {doc.parish.text}
                  {doc.parish.ref && (
                    <Typography.Text type="secondary"> → {doc.parish.type} {doc.parish.ref}</Typography.Text>
                  )}
                </>
              )}
            </Descriptions.Item>
            <Descriptions.Item label="Начало периода">{formatFactDate(doc.since)}</Descriptions.Item>
            <Descriptions.Item label="Конец периода">{formatFactDate(doc.until)}</Descriptions.Item>
            <Descriptions.Item label="Населённые пункты"><TextRefListView items={doc.settlements} /></Descriptions.Item>
            <Descriptions.Item label="Заметки"><TextRefListView items={doc.notes} /></Descriptions.Item>
            <Descriptions.Item label="Доказательства"><SourceLinkListView items={doc.sources} /></Descriptions.Item>
            <Descriptions.Item label="Приватная">{doc.private ? "да" : "нет"}</Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title={`Удалить «${documentLabel(doc)}»?`}
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
          <Form.Item label="Единица хранения" required>
            <ArchiveNodePicker
              value={editUnitId}
              label={editUnitLabel ?? undefined}
              onChange={(nid, lbl) => {
                setEditUnitId(nid);
                setEditUnitLabel(lbl);
              }}
            />
          </Form.Item>
          <Form.Item
            name="title"
            label="Заголовок"
            rules={[{ required: true, whitespace: true, message: "Введите заголовок" }]}
          >
            <Input />
          </Form.Item>
          <Form.Item name="kind" label="Вид">
            <Input placeholder="метрическая книга / исповедная роспись" />
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

      <Modal
        title="Документ используется"
        open={conflict != null}
        onCancel={() => setConflict(null)}
        footer={<Button onClick={() => setConflict(null)}>Закрыть</Button>}
      >
        <Typography.Paragraph>
          Нельзя удалить — на документ ссылаются другие сущности:
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
