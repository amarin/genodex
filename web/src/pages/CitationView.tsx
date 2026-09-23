import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { Alert, Breadcrumb, Button, Card, Checkbox, Descriptions, Form, Input, List, Modal, Popconfirm, Select, Space, Spin, Typography } from "antd";
import { deleteCitation, fetchCitation, fetchSources, updateCitation, type Anchor, type Citation, type Source } from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { AnchorEditor } from "../AnchorEditor";

interface EditFormValues {
  source_id: string;
  text?: string;
  note?: string;
  private?: boolean;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["source_id", "text", "note"];

function citationLabel(c: Citation): string {
  return c.text || c.id;
}

function anchorSummary(a: Anchor | null | undefined): string {
  if (a == null) {
    return "—";
  }
  if (a.kind === "archive") {
    return `архив: узел ${a.node_id ?? "—"}${a.document_id ? `, документ ${a.document_id}` : ""}, стр. ${a.page ?? "—"}`;
  }
  if (a.kind === "file") {
    return `файл: вложение ${a.attachment_id ?? "—"}${a.timecode ? `, ${a.timecode}` : ""}`;
  }
  if (a.kind === "url") {
    return a.url ?? "—";
  }
  return "—";
}

// CitationView — просмотр цитаты, переключаемый в форму редактирования на
// той же странице (toggle+explicit-save), по образцу ArchiveView.tsx.
export default function CitationView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [citation, setCitation] = useState<Citation | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sources, setSources] = useState<Source[]>([]);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [anchor, setAnchor] = useState<Anchor | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (citationId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setCitation(null);
    fetchCitation(citationId)
      .then(setCitation)
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

  useEffect(() => {
    fetchSources({ limit: 500 })
      .then(setSources)
      .catch(() => setSources([]));
  }, []);

  const sourceOptions = sources.map((s) => ({ value: s.id, label: s.title }));
  const sourceTitle = (sourceID?: string) => sources.find((s) => s.id === sourceID)?.title ?? sourceID;

  const startEdit = () => {
    if (citation == null) {
      return;
    }
    form.setFieldsValue({
      source_id: citation.source_id,
      text: citation.text ?? "",
      note: citation.note ?? "",
      private: citation.private,
    });
    setAnchor(citation.anchor ?? null);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (citation == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await updateCitation(citation.id, {
        source_id: values.source_id,
        anchor,
        text: values.text,
        note: values.note,
        private: values.private ?? false,
      });
      setCitation(updated);
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
    if (citation == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteCitation(citation.id);
      navigate("/citations");
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
          <Link to="/citations">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (citation == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/citations">Цитаты</Link> },
          { title: citationLabel(citation) },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={citationLabel(citation)} column={1} bordered size="small">
            <Descriptions.Item label="Источник">
              <Link to={`/sources/${citation.source_id}`}>{sourceTitle(citation.source_id)}</Link>
            </Descriptions.Item>
            <Descriptions.Item label="Текст">
              <Typography.Paragraph style={{ whiteSpace: "pre-wrap", marginBottom: 0 }}>
                {citation.text || "—"}
              </Typography.Paragraph>
            </Descriptions.Item>
            <Descriptions.Item label="Привязка">{anchorSummary(citation.anchor)}</Descriptions.Item>
            <Descriptions.Item label="Заметка">{citation.note || "—"}</Descriptions.Item>
            <Descriptions.Item label="Приватная">{citation.private ? "да" : "нет"}</Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title={`Удалить «${citationLabel(citation)}»?`}
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
        <Form form={form} layout="vertical" onFinish={onSave} style={{ maxWidth: 520 }}>
          {saveError != null && (
            <Alert type="error" showIcon message={saveError} style={{ marginBottom: 16 }} />
          )}
          <Form.Item name="source_id" label="Источник" rules={[{ required: true, message: "Выберите источник" }]}>
            <Select
              options={sourceOptions}
              showSearch
              filterOption={(input, option) =>
                (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
              }
            />
          </Form.Item>
          <Form.Item name="text" label="Текст выписки">
            <Input.TextArea rows={4} />
          </Form.Item>
          <Form.Item label="Привязка">
            <AnchorEditor value={anchor} onChange={setAnchor} addLabel="+ привязка" />
          </Form.Item>
          <Form.Item name="note" label="Заметка">
            <Input />
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
