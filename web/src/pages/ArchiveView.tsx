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
  deleteArchive,
  fetchArchive,
  fetchRepositories,
  updateArchive,
  type Archive,
  type Repository,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";

interface EditFormValues {
  name: string;
  systemText?: string;
  repository_id?: string;
  private?: boolean;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["name", "repository_id"];

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

// ArchiveView — просмотр архива, переключаемый в форму редактирования на той
// же странице (toggle+explicit-save). repository_id — Select со списком
// хранилищ (fetchRepositories), не TextRef.
export default function ArchiveView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [archive, setArchive] = useState<Archive | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [repositories, setRepositories] = useState<Repository[]>([]);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (archiveId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setArchive(null);
    fetchArchive(archiveId)
      .then(setArchive)
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
  const repositoryName = (repoID?: string) =>
    repositories.find((r) => r.id === repoID)?.name ?? repoID;

  const startEdit = () => {
    if (archive == null) {
      return;
    }
    form.setFieldsValue({
      name: archive.name,
      systemText: archive.system?.text ?? "",
      repository_id: archive.repository_id ?? undefined,
      private: archive.private,
    });
    setNotes(archive.notes);
    setSources(archive.sources);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (archive == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const systemText = (values.systemText ?? "").trim();
      const updated = await updateArchive(archive.id, {
        name: values.name,
        system: systemText ? { text: systemText } : null,
        repository_id: values.repository_id ?? "",
        notes,
        sources,
        private: values.private ?? false,
      });
      setArchive(updated);
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
    if (archive == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteArchive(archive.id);
      navigate("/archives");
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
          <Link to="/archives">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (archive == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Данные</Link> },
          { title: <Link to="/archives">Архивы</Link> },
          { title: archive.name },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={archive.name} column={1} bordered size="small">
            <Descriptions.Item label="Система иерархии">{archive.system?.text || "—"}</Descriptions.Item>
            <Descriptions.Item label="Хранилище">
              {archive.repository_id ? (
                <Link to={`/repositories/${archive.repository_id}`}>{repositoryName(archive.repository_id)}</Link>
              ) : (
                "—"
              )}
            </Descriptions.Item>
            <Descriptions.Item label="Заметки"><TextRefListView items={archive.notes} /></Descriptions.Item>
            <Descriptions.Item label="Доказательства"><SourceLinkListView items={archive.sources} /></Descriptions.Item>
            <Descriptions.Item label="Приватная">{archive.private ? "да" : "нет"}</Descriptions.Item>
          </Descriptions>
          <Space style={{ marginTop: 16 }}>
            <Link to={`/archive-nodes?archive_id=${encodeURIComponent(archive.id)}`}>
              <Button>Архивные единицы →</Button>
            </Link>
          </Space>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title={`Удалить «${archive.name}»?`}
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
          <Form.Item
            name="name"
            label="Название"
            rules={[{ required: true, whitespace: true, message: "Введите название" }]}
          >
            <Input />
          </Form.Item>
          <Form.Item name="systemText" label="Система иерархии (текстом)">
            <Input placeholder="фонд-опись-дело" />
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
