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
  adminDivisionTypeLabel,
  deleteDivision,
  fetchAdminDivisions,
  fetchDivision,
  searchAdminDivisions,
  updateDivision,
  MAX_PAGE_LIMIT,
  type AdminDivision,
  type AdminDivisionType,
} from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { CreateDivisionModal, TYPE_OPTIONS } from "./DivisionForm";

function divisionLabel(d: AdminDivision): string {
  return `${d.name} (${adminDivisionTypeLabel(d.type)})`;
}

interface EditFormValues {
  name: string;
  type: AdminDivisionType;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["name", "type"];

// DivisionView — просмотр единицы, переключаемый в форму редактирования
// (та же страница, без смены URL — решение: PUT заменяет
// name/type/parent_id разом, автосейв по полям не подходит). Дочерние
// единицы — список со ссылками на их View; «добавить дочернюю» открывает
// CreateDivisionModal с parentId = текущая единица. Удаление — Popconfirm,
// конфликт (409, единица занята) — модалка со списком ссылающихся.
export default function DivisionView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [division, setDivision] = useState<AdminDivision | null>(null);
  const [parent, setParent] = useState<AdminDivision | null>(null);
  const [children, setChildren] = useState<AdminDivision[]>([]);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  // editParentId — родитель, выбранный в форме редактирования (null — корень).
  const [editParentId, setEditParentId] = useState<string | null>(null);
  const [editParentLabel, setEditParentLabel] = useState<string | null>(null);
  const [parentPickerOpen, setParentPickerOpen] = useState(false);
  const [parentQuery, setParentQuery] = useState("");
  const [parentResults, setParentResults] = useState<AdminDivision[]>([]);
  const [parentSearching, setParentSearching] = useState(false);

  const [addChildOpen, setAddChildOpen] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (divisionId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    fetchDivision(divisionId)
      .then((d) => {
        setDivision(d);
        if (d.parent_id != null) {
          fetchDivision(d.parent_id)
            .then(setParent)
            .catch(() => setParent(null));
        } else {
          setParent(null);
        }
        return fetchAdminDivisions({ parent_id: divisionId, limit: MAX_PAGE_LIMIT });
      })
      .then(setChildren)
      .catch((e) => {
        if (e instanceof ApiError && e.status === 404) {
          setNotFound(true);
        } else {
          setError(e instanceof Error ? e.message : "Не удалось загрузить единицу");
        }
      })
      .finally(() => setLoading(false));
  };

  useEffect(() => {
    if (id != null) {
      load(id);
    }
  }, [id]);

  const startEdit = () => {
    if (division == null) {
      return;
    }
    form.setFieldsValue({ name: division.name, type: division.type as AdminDivisionType });
    setEditParentId(division.parent_id);
    setEditParentLabel(parent != null ? divisionLabel(parent) : null);
    setParentPickerOpen(false);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setParentPickerOpen(false);
    setSaveError(null);
  };

  const onParentSearch = (value: string) => {
    setParentQuery(value);
    const q = value.trim();
    if (!q) {
      setParentResults([]);
      return;
    }
    setParentSearching(true);
    searchAdminDivisions({ q, limit: 20 })
      .then((results) => setParentResults(results.filter((r) => r.id !== division?.id)))
      .catch(() => setParentResults([]))
      .finally(() => setParentSearching(false));
  };

  const pickParent = (d: AdminDivision | null) => {
    setEditParentId(d?.id ?? null);
    setEditParentLabel(d != null ? divisionLabel(d) : null);
    setParentPickerOpen(false);
    setParentQuery("");
    setParentResults([]);
  };

  const onSave = async (values: EditFormValues) => {
    if (division == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await updateDivision(division.id, {
        name: values.name,
        type: values.type,
        parent_id: editParentId,
      });
      setDivision(updated);
      if (updated.parent_id != null) {
        fetchDivision(updated.parent_id)
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
    if (division == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteDivision(division.id);
      navigate(division.parent_id != null ? `/divisions/${division.parent_id}` : "/divisions");
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        setConflict(e.referrers ?? []);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось удалить единицу");
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
        message="Единица не найдена"
        description="Возможно, её удалили. Вернитесь к списку."
        action={
          <Link to="/divisions">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (division == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/divisions">Административное деление</Link> },
          ...(parent != null
            ? [{ title: <Link to={`/divisions/${parent.id}`}>{parent.name}</Link> }]
            : []),
          { title: division.name },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={division.name} column={1} bordered size="small">
            <Descriptions.Item label="Тип">{adminDivisionTypeLabel(division.type)}</Descriptions.Item>
            <Descriptions.Item label="Родитель">
              {parent != null ? (
                <Link to={`/divisions/${parent.id}`}>{parent.name}</Link>
              ) : (
                <Typography.Text type="secondary">корень</Typography.Text>
              )}
            </Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Button onClick={() => setAddChildOpen(true)}>+ добавить дочернюю</Button>
              <Popconfirm
                title={`Удалить «${division.name}»?`}
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
            rules={[{ required: true, message: "Введите название" }]}
          >
            <Input />
          </Form.Item>
          <Form.Item name="type" label="Тип" rules={[{ required: true, message: "Выберите тип" }]}>
            <Select options={TYPE_OPTIONS} />
          </Form.Item>
          <Form.Item label="Родитель">
            <Space direction="vertical" style={{ width: "100%" }}>
              <Space>
                <Typography.Text>
                  {editParentLabel ?? <Typography.Text type="secondary">корень</Typography.Text>}
                </Typography.Text>
                <Button size="small" onClick={() => setParentPickerOpen((v) => !v)}>
                  Изменить
                </Button>
                {editParentId != null && (
                  <Button size="small" onClick={() => pickParent(null)}>
                    Сделать корневой
                  </Button>
                )}
              </Space>
              {parentPickerOpen && (
                <Card size="small">
                  <Input.Search
                    placeholder="Поиск родителя по названию…"
                    value={parentQuery}
                    onChange={(e) => onParentSearch(e.target.value)}
                    loading={parentSearching}
                    allowClear
                  />
                  <List
                    size="small"
                    dataSource={parentResults}
                    locale={{ emptyText: "Ничего не найдено" }}
                    renderItem={(d) => (
                      <List.Item style={{ cursor: "pointer" }} onClick={() => pickParent(d)}>
                        {divisionLabel(d)}
                      </List.Item>
                    )}
                  />
                </Card>
              )}
            </Space>
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
        Дочерние единицы
      </Typography.Title>
      <List
        dataSource={children}
        locale={{ emptyText: "Дочерних единиц нет" }}
        renderItem={(c) => (
          <List.Item>
            <Link to={`/divisions/${c.id}`}>{divisionLabel(c)}</Link>
          </List.Item>
        )}
      />

      <CreateDivisionModal
        open={addChildOpen}
        parentId={division.id}
        onClose={() => setAddChildOpen(false)}
        onCreated={() => {
          setAddChildOpen(false);
          fetchAdminDivisions({ parent_id: division.id, limit: MAX_PAGE_LIMIT })
            .then(setChildren)
            .catch(() => {});
        }}
      />

      <Modal
        title="Единица используется"
        open={conflict != null}
        onCancel={() => setConflict(null)}
        footer={<Button onClick={() => setConflict(null)}>Закрыть</Button>}
      >
        <Typography.Paragraph>
          Нельзя удалить — на единицу ссылаются другие сущности:
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
