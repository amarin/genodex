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
  InputNumber,
  List,
  Modal,
  Popconfirm,
  Select,
  Space,
  Spin,
  Typography,
} from "antd";
import { deleteAttachment, fetchAttachment, updateAttachment, type Attachment } from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { ArchiveDocumentSelect, ArchiveNodePicker } from "../ArchiveNodePicker";

interface EditFormValues {
  kind: string;
  uri?: string;
  filename?: string;
  mime?: string;
  page?: number;
  note?: string;
  private?: boolean;
}

// EDIT_FORM_FIELDS — node_id/document_id НЕ входят: они управляются отдельным
// состоянием (nodeId/documentId, не полями antd Form — тот же приём, что
// editParentId в ArchiveNodeView.tsx), поэтому 422 на них попадает в общий
// saveError.
const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["kind", "uri", "filename", "mime", "page", "note"];

const KIND_OPTIONS = [
  { value: "scan", label: "скан" },
  { value: "document", label: "документ" },
  { value: "audio", label: "аудио" },
  { value: "photo", label: "фото" },
];

function attachmentLabel(a: Attachment): string {
  return a.filename || a.uri || a.id;
}

// AttachmentView — просмотр вложения, переключаемый в форму редактирования
// на той же странице. node_id/document_id в режиме редактирования —
// ArchiveNodePicker + ArchiveDocumentSelect (подпроект 6 — ArchiveNode/
// ArchiveDocument теперь имеют CRUD, см. AttachmentForm/ArchiveNodePicker.tsx).
export default function AttachmentView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [attachment, setAttachment] = useState<Attachment | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [nodeId, setNodeId] = useState("");
  const [nodeLabel, setNodeLabel] = useState<string | null>(null);
  const [documentId, setDocumentId] = useState<string | undefined>(undefined);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (attachmentId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setAttachment(null);
    fetchAttachment(attachmentId)
      .then(setAttachment)
      .catch((e) => {
        if (e instanceof ApiError && e.status === 404) {
          setNotFound(true);
        } else {
          setError(e instanceof Error ? e.message : "Не удалось загрузить запись");
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
    if (attachment == null) {
      return;
    }
    form.setFieldsValue({
      kind: attachment.kind,
      uri: attachment.uri ?? "",
      filename: attachment.filename ?? "",
      mime: attachment.mime ?? "",
      page: attachment.page,
      note: attachment.note ?? "",
      private: attachment.private,
    });
    setNodeId(attachment.node_id);
    setNodeLabel(null);
    setDocumentId(attachment.document_id);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (attachment == null) {
      return;
    }
    if (!nodeId) {
      setSaveError("Выберите архивный узел");
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await updateAttachment(attachment.id, {
        kind: values.kind,
        uri: values.uri,
        filename: values.filename,
        mime: values.mime,
        page: values.page,
        node_id: nodeId,
        document_id: documentId,
        note: values.note,
        private: values.private ?? false,
      });
      setAttachment(updated);
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
    if (attachment == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteAttachment(attachment.id);
      navigate("/attachments");
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        setConflict(e.referrers ?? []);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось удалить запись");
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
        message="Запись не найдена"
        description="Возможно, её удалили. Вернитесь к списку."
        action={
          <Link to="/attachments">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (attachment == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/attachments">Вложения</Link> },
          { title: attachmentLabel(attachment) },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={attachmentLabel(attachment)} column={1} bordered size="small">
            <Descriptions.Item label="Вид">{attachment.kind}</Descriptions.Item>
            <Descriptions.Item label="Имя файла">{attachment.filename || "—"}</Descriptions.Item>
            <Descriptions.Item label="URI">{attachment.uri || "—"}</Descriptions.Item>
            <Descriptions.Item label="MIME-тип">{attachment.mime || "—"}</Descriptions.Item>
            <Descriptions.Item label="Страница">{attachment.page || "—"}</Descriptions.Item>
            <Descriptions.Item label="Архивный узел">{attachment.node_id}</Descriptions.Item>
            <Descriptions.Item label="Архивный документ">{attachment.document_id || "—"}</Descriptions.Item>
            <Descriptions.Item label="Заметка">{attachment.note || "—"}</Descriptions.Item>
            <Descriptions.Item label="Приватная">{attachment.private ? "да" : "нет"}</Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title={`Удалить «${attachmentLabel(attachment)}»?`}
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
        <Form form={form} layout="vertical" onFinish={onSave} style={{ maxWidth: 480 }}>
          {saveError != null && (
            <Alert type="error" showIcon message={saveError} style={{ marginBottom: 16 }} />
          )}
          <Form.Item name="kind" label="Вид" rules={[{ required: true, message: "Выберите вид" }]}>
            <Select options={KIND_OPTIONS} />
          </Form.Item>
          <Form.Item name="filename" label="Имя файла">
            <Input />
          </Form.Item>
          <Form.Item name="uri" label="URI">
            <Input placeholder="файл, ссылка" />
          </Form.Item>
          <Form.Item name="mime" label="MIME-тип">
            <Input placeholder="image/jpeg" />
          </Form.Item>
          <Form.Item name="page" label="Страница">
            <InputNumber min={0} style={{ width: "100%" }} />
          </Form.Item>
          <Form.Item label="Архивный узел" required>
            <ArchiveNodePicker
              value={nodeId}
              label={nodeLabel ?? undefined}
              onChange={(nid, lbl) => {
                setNodeId(nid);
                setNodeLabel(lbl);
                setDocumentId(undefined);
              }}
            />
          </Form.Item>
          <Form.Item label="Архивный документ (необязательно)">
            <ArchiveDocumentSelect nodeId={nodeId || null} value={documentId} onChange={setDocumentId} />
          </Form.Item>
          <Form.Item name="note" label="Заметка">
            <Input.TextArea rows={3} />
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
        title="Запись используется"
        open={conflict != null}
        onCancel={() => setConflict(null)}
        footer={<Button onClick={() => setConflict(null)}>Закрыть</Button>}
      >
        <Typography.Paragraph>
          Нельзя удалить — на запись ссылаются другие сущности:
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
