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
  Select,
  Space,
  Spin,
  Typography,
} from "antd";
import {
  deleteSource,
  fetchRepositories,
  fetchSource,
  updateSource,
  type FactDate,
  type Reliability,
  type Repository,
  type Source,
  type TextRef,
} from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { FactDateEditor, formatFactDate } from "../FactDateEditor";
import { TextRefListEditor } from "../TextRefList";

const KIND_OPTIONS = [
  { value: "archival-scan", label: "скан из архива" },
  { value: "transcription", label: "расшифровка" },
  { value: "document", label: "документ" },
  { value: "audio", label: "аудио" },
  { value: "photo", label: "фото" },
  { value: "memory", label: "со слов/по памяти" },
  { value: "external", label: "внешний источник" },
];

const RELIABILITY_OPTIONS: { value: Reliability; label: string }[] = [
  { value: "primary", label: "Первичный" },
  { value: "contemporary", label: "Современник" },
  { value: "memory", label: "Со слов/по памяти" },
  { value: "indirect", label: "Косвенный" },
  { value: "unknown", label: "Неизвестна" },
];

const RELIABILITY_LABELS: Record<string, string> = Object.fromEntries(
  RELIABILITY_OPTIONS.map((o) => [o.value, o.label]),
);

interface EditFormValues {
  kind: string;
  title: string;
  author?: string;
  reliability: Reliability;
  repository_id?: string;
  private?: boolean;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["kind", "title", "author", "reliability", "repository_id"];

// SourceView — просмотр источника, переключаемый в форму редактирования на
// той же странице (toggle+explicit-save), по образцу ArchiveView.tsx.
export default function SourceView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [source, setSource] = useState<Source | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [repositories, setRepositories] = useState<Repository[]>([]);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [date, setDate] = useState<FactDate | null>(null);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (sourceId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setSource(null);
    fetchSource(sourceId)
      .then(setSource)
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
    fetchRepositories({ limit: 500 })
      .then(setRepositories)
      .catch(() => setRepositories([]));
  }, []);

  const repositoryOptions = repositories.map((r) => ({ value: r.id, label: r.name }));
  const repositoryName = (repoID?: string) => repositories.find((r) => r.id === repoID)?.name ?? repoID;

  const startEdit = () => {
    if (source == null) {
      return;
    }
    form.setFieldsValue({
      kind: source.kind,
      title: source.title,
      author: source.author ?? "",
      reliability: source.reliability,
      repository_id: source.repository_id ?? undefined,
      private: source.private,
    });
    setDate(source.date ?? null);
    setNotes(source.notes);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (source == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await updateSource(source.id, {
        kind: values.kind,
        title: values.title,
        author: values.author,
        date,
        reliability: values.reliability,
        repository_id: values.repository_id ?? "",
        notes,
        private: values.private ?? false,
      });
      setSource(updated);
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
    if (source == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteSource(source.id);
      navigate("/sources");
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
          <Link to="/sources">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (source == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/sources">Источники</Link> },
          { title: source.title },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={source.title} column={1} bordered size="small">
            <Descriptions.Item label="Вид">{source.kind}</Descriptions.Item>
            <Descriptions.Item label="Автор">{source.author || "—"}</Descriptions.Item>
            <Descriptions.Item label="Дата">{formatFactDate(source.date)}</Descriptions.Item>
            <Descriptions.Item label="Достоверность">{RELIABILITY_LABELS[source.reliability] ?? source.reliability}</Descriptions.Item>
            <Descriptions.Item label="Хранилище">
              {source.repository_id ? (
                <Link to={`/repositories/${source.repository_id}`}>{repositoryName(source.repository_id)}</Link>
              ) : (
                "—"
              )}
            </Descriptions.Item>
            <Descriptions.Item label="Заметки">
              {source.notes.length === 0 ? (
                <Typography.Text type="secondary">—</Typography.Text>
              ) : (
                <Space direction="vertical" size={0}>
                  {source.notes.map((n, i) => (
                    <Typography.Text key={i}>{n.text}</Typography.Text>
                  ))}
                </Space>
              )}
            </Descriptions.Item>
            <Descriptions.Item label="Приватная">{source.private ? "да" : "нет"}</Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title={`Удалить «${source.title}»?`}
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
          <Form.Item name="kind" label="Вид" rules={[{ required: true, message: "Выберите вид" }]}>
            <Select options={KIND_OPTIONS} />
          </Form.Item>
          <Form.Item
            name="title"
            label="Название"
            rules={[{ required: true, whitespace: true, message: "Введите название" }]}
          >
            <Input />
          </Form.Item>
          <Form.Item name="author" label="Автор">
            <Input />
          </Form.Item>
          <Form.Item label="Дата">
            <FactDateEditor value={date} onChange={setDate} addLabel="+ дата" />
          </Form.Item>
          <Form.Item name="reliability" label="Достоверность" rules={[{ required: true, message: "Выберите достоверность" }]}>
            <Select options={RELIABILITY_OPTIONS} />
          </Form.Item>
          <Form.Item name="repository_id" label="Хранилище">
            <Select
              allowClear
              placeholder="Не выбрано"
              options={repositoryOptions}
              showSearch
              filterOption={(input, option) =>
                (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
              }
            />
          </Form.Item>
          <Form.Item label="Заметки">
            <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
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
