import { useEffect, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import {
  Alert,
  Breadcrumb,
  Button,
  Card,
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
  deleteChurch,
  fetchChurch,
  updateChurch,
  type Church,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";

interface EditFormValues {
  name: string;
  parishText?: string;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["name"];

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

// ChurchView — просмотр церкви, переключаемый в форму редактирования на той
// же странице (toggle+explicit-save). parish показывается текстом; если у
// него уже есть ref — вторичная пометка «→ Type ID» (не кликабельно, picker
// не реализован, docs/data-model/entity-write.md §4-5).
export default function ChurchView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [church, setChurch] = useState<Church | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [settlements, setSettlements] = useState<TextRef[]>([]);
  const [variants, setVariants] = useState<TextRef[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (churchId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setChurch(null);
    fetchChurch(churchId)
      .then(setChurch)
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
    if (church == null) {
      return;
    }
    form.setFieldsValue({ name: church.name, parishText: church.parish?.text ?? "" });
    setSettlements(church.settlements);
    setVariants(church.variants.map((v) => ({ text: v })));
    setNotes(church.notes);
    setSources(church.sources);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (church == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const parishText = (values.parishText ?? "").trim();
      const parishHasRef = church.parish?.ref != null && church.parish.ref !== "";
      const updated = await updateChurch(church.id, {
        name: values.name,
        // Сохраняем существующую ссылку, если поле не тронуто (тот же текст) и
        // уже несло ref; иначе — чистый текст без ref (picker не реализован).
        parish: parishHasRef && parishText === church.parish?.text
          ? church.parish
          : parishText
            ? { text: parishText }
            : null,
        settlements,
        variants: variants.map((v) => v.text).filter((t) => t.trim() !== ""),
        notes,
        sources,
      });
      setChurch(updated);
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
    if (church == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteChurch(church.id);
      navigate("/churches");
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
          <Link to="/churches">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (church == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Данные</Link> },
          { title: <Link to="/churches">Церкви</Link> },
          { title: church.name },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={church.name} column={1} bordered size="small">
            <Descriptions.Item label="Приход">
              {church.parish == null ? (
                "—"
              ) : (
                <>
                  {church.parish.text}
                  {church.parish.ref && (
                    <Typography.Text type="secondary"> → {church.parish.type} {church.parish.ref}</Typography.Text>
                  )}
                </>
              )}
            </Descriptions.Item>
            <Descriptions.Item label="Населённые пункты"><TextRefListView items={church.settlements} /></Descriptions.Item>
            <Descriptions.Item label="Варианты названия">
              {church.variants.length === 0 ? "—" : church.variants.join(", ")}
            </Descriptions.Item>
            <Descriptions.Item label="Заметки"><TextRefListView items={church.notes} /></Descriptions.Item>
            <Descriptions.Item label="Доказательства"><SourceLinkListView items={church.sources} /></Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title={`Удалить «${church.name}»?`}
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
          <Form.Item name="parishText" label="Приход (текстом)">
            <Input placeholder="Никольский приход" />
          </Form.Item>
          <Form.Item label="Населённые пункты">
            <TextRefListEditor value={settlements} onChange={setSettlements} addLabel="+ населённый пункт" />
          </Form.Item>
          <Form.Item label="Варианты названия">
            <TextRefListEditor value={variants} onChange={setVariants} addLabel="+ вариант" />
          </Form.Item>
          <Form.Item label="Заметки">
            <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
          </Form.Item>
          <Form.Item label="Доказательства">
            <SourceLinkListEditor value={sources} onChange={setSources} addLabel="+ доказательство" />
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
