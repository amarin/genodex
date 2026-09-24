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
import { deleteNote, fetchNote, fetchNotes, updateNote, type Note, type SourceLink } from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { SourceLinkListEditor } from "../SourceLinkList";

interface EditFormValues {
  kind: string;
  title?: string;
  text?: string;
  parent_id?: string;
  private?: boolean;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["kind", "title", "text", "parent_id"];

function noteLabel(n: Note): string {
  return n.title || n.text?.slice(0, 80) || n.id;
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

// NoteView — просмотр заметки, переключаемый в форму редактирования на той
// же странице (toggle+explicit-save). parent_id — Select со списком заметок
// (fetchNotes), сама заметка исключена из списка (нельзя выбрать себя же
// родителем — сценарий всё равно проверит цикл, но так короче путь до
// ошибки).
export default function NoteView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [note, setNote] = useState<Note | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notes, setNotes] = useState<Note[]>([]);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (noteId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setNote(null);
    fetchNote(noteId)
      .then(setNote)
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
    fetchNotes({ limit: 500 })
      .then(setNotes)
      .catch(() => setNotes([]));
  }, []);

  const noteOptions = notes
    .filter((n) => n.id !== id)
    .map((n) => ({ value: n.id, label: noteLabel(n) }));
  const parentLabel = (parentID?: string) => {
    const parent = notes.find((n) => n.id === parentID);
    return parent != null ? noteLabel(parent) : parentID;
  };

  const startEdit = () => {
    if (note == null) {
      return;
    }
    form.setFieldsValue({
      kind: note.kind,
      title: note.title ?? "",
      text: note.text ?? "",
      parent_id: note.parent_id ?? undefined,
      private: note.private,
    });
    setSources(note.sources);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (note == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await updateNote(note.id, {
        kind: values.kind,
        title: values.title,
        text: values.text,
        parent_id: values.parent_id ?? "",
        sources,
        private: values.private ?? false,
      });
      setNote(updated);
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
    if (note == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteNote(note.id);
      navigate("/notes");
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
          <Link to="/notes">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (note == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Данные</Link> },
          { title: <Link to="/notes">Заметки</Link> },
          { title: noteLabel(note) },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={noteLabel(note)} column={1} bordered size="small">
            <Descriptions.Item label="Вид">{note.kind}</Descriptions.Item>
            <Descriptions.Item label="Заголовок">{note.title || "—"}</Descriptions.Item>
            <Descriptions.Item label="Текст">
              <Typography.Paragraph style={{ whiteSpace: "pre-wrap", marginBottom: 0 }}>
                {note.text || "—"}
              </Typography.Paragraph>
            </Descriptions.Item>
            <Descriptions.Item label="Родительская заметка">
              {note.parent_id ? (
                <Link to={`/notes/${note.parent_id}`}>{parentLabel(note.parent_id)}</Link>
              ) : (
                "—"
              )}
            </Descriptions.Item>
            <Descriptions.Item label="Доказательства"><SourceLinkListView items={note.sources} /></Descriptions.Item>
            <Descriptions.Item label="Приватная">{note.private ? "да" : "нет"}</Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title={`Удалить «${noteLabel(note)}»?`}
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
            name="kind"
            label="Вид"
            rules={[{ required: true, whitespace: true, message: "Введите вид" }]}
          >
            <Input placeholder="note / article / book / chapter" />
          </Form.Item>
          <Form.Item name="title" label="Заголовок">
            <Input />
          </Form.Item>
          <Form.Item name="text" label="Текст (markdown)">
            <Input.TextArea rows={6} />
          </Form.Item>
          <Form.Item name="parent_id" label="Родительская заметка">
            <Select
              allowClear
              placeholder="Не выбрана"
              options={noteOptions}
              showSearch
              filterOption={(input, option) =>
                (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
              }
            />
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
