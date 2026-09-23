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
  Select,
  Space,
  Spin,
  Typography,
} from "antd";
import {
  deleteGivenName,
  fetchGivenName,
  updateGivenName,
  type GivenName,
  type NameGender,
  type TextRef,
} from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { TextRefListEditor } from "../TextRefList";

interface EditFormValues {
  canonical: string;
  gender: NameGender;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["canonical", "gender"];

const GENDER_OPTIONS: { value: NameGender; label: string }[] = [
  { value: "male", label: "Мужской" },
  { value: "female", label: "Женский" },
  { value: "neutral", label: "Нейтральный (вывод пола запрещён)" },
];

const GENDER_LABELS: Record<string, string> = {
  male: "Мужской",
  female: "Женский",
  neutral: "Нейтральный",
};

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

// GivenNameView — просмотр словарной записи имени, переключаемый в форму
// редактирования на той же странице (тот же toggle+explicit-save, что
// DivisionView — PUT заменяет запись целиком, docs/data-model/entity-write.md
// §4). Без родителя/детей/дерева — GivenName не иерархична, в отличие от
// AdminDivision.
export default function GivenNameView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [givenName, setGivenName] = useState<GivenName | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [variants, setVariants] = useState<TextRef[]>([]);
  const [items, setItems] = useState<TextRef[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (givenNameId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setGivenName(null);
    fetchGivenName(givenNameId)
      .then(setGivenName)
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
    if (givenName == null) {
      return;
    }
    form.setFieldsValue({ canonical: givenName.canonical, gender: givenName.gender || undefined });
    setVariants(givenName.variants);
    setItems(givenName.items);
    setNotes(givenName.notes);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (givenName == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await updateGivenName(givenName.id, {
        canonical: values.canonical,
        gender: values.gender,
        variants,
        items,
        notes,
      });
      setGivenName(updated);
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
    if (givenName == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteGivenName(givenName.id);
      navigate("/given-names");
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
          <Link to="/given-names">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (givenName == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/given-names">Имена</Link> },
          { title: givenName.canonical },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={givenName.canonical} column={1} bordered size="small">
            <Descriptions.Item label="Пол">{GENDER_LABELS[givenName.gender] ?? "—"}</Descriptions.Item>
            <Descriptions.Item label="Варианты"><TextRefListView items={givenName.variants} /></Descriptions.Item>
            <Descriptions.Item label="Носители"><TextRefListView items={givenName.items} /></Descriptions.Item>
            <Descriptions.Item label="Заметки"><TextRefListView items={givenName.notes} /></Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title={`Удалить «${givenName.canonical}»?`}
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
            name="canonical"
            label="Каноническая форма"
            rules={[{ required: true, whitespace: true, message: "Введите каноническую форму" }]}
          >
            <Input />
          </Form.Item>
          <Form.Item
            name="gender"
            label="Пол"
            rules={[{ required: true, message: "Выберите пол" }]}
          >
            <Select options={GENDER_OPTIONS} placeholder="Выберите пол" />
          </Form.Item>
          <Form.Item label="Варианты написания">
            <TextRefListEditor value={variants} onChange={setVariants} addLabel="+ вариант" />
          </Form.Item>
          <Form.Item label="Носители">
            <TextRefListEditor value={items} onChange={setItems} addLabel="+ запись" />
          </Form.Item>
          <Form.Item label="Заметки">
            <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
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
