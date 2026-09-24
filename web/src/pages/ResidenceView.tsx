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
  adminDivisionTypeLabel,
  deleteResidence,
  fetchAdminDivisions,
  fetchResidence,
  MAX_PAGE_LIMIT,
  updateResidence,
  type AdminDivision,
  type FactDate,
  type Residence,
  type SourceLink,
} from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { FactDateEditor, formatFactDate } from "../FactDateEditor";
import { personDisplayName } from "../PersonNameList";
import { PersonPicker, usePersonOptions } from "../PersonPicker";
import { SourceLinkListEditor } from "../SourceLinkList";

interface EditFormValues {
  note?: string;
  private?: boolean;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["note"];

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

// ResidenceView — просмотр проживания, переключаемый в форму редактирования
// на той же странице (toggle+explicit-save). person_id — ссылка на
// /people/{id} с отображаемым именем; place_id — ссылка на
// /divisions/{id} с названием деления (обе — по образцу
// repositoryName/ArchiveView.tsx). note — одна строка, Input.TextArea.
export default function ResidenceView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [residence, setResidence] = useState<Residence | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const people = usePersonOptions();
  const [divisions, setDivisions] = useState<AdminDivision[]>([]);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [personId, setPersonId] = useState("");
  const [placeId, setPlaceId] = useState("");
  const [since, setSince] = useState<FactDate | null>(null);
  const [until, setUntil] = useState<FactDate | null>(null);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (residenceId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setResidence(null);
    fetchResidence(residenceId)
      .then(setResidence)
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
    fetchAdminDivisions({ limit: MAX_PAGE_LIMIT })
      .then(setDivisions)
      .catch(() => setDivisions([]));
  }, []);

  const divisionOptions = divisions.map((d) => ({
    value: d.id,
    label: `${d.name} (${adminDivisionTypeLabel(d.type)})`,
  }));

  const personLabel = (personId2: string) => {
    const p = people.find((person) => person.id === personId2);
    return p != null ? personDisplayName(p) : personId2;
  };

  const placeLabel = (placeId2: string) => {
    const d = divisions.find((division) => division.id === placeId2);
    return d != null ? `${d.name} (${adminDivisionTypeLabel(d.type)})` : placeId2;
  };

  const startEdit = () => {
    if (residence == null) {
      return;
    }
    form.setFieldsValue({ note: residence.note ?? "", private: residence.private });
    setPersonId(residence.person_id);
    setPlaceId(residence.place_id);
    setSince(residence.since ?? null);
    setUntil(residence.until ?? null);
    setSources(residence.sources);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (residence == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await updateResidence(residence.id, {
        person_id: personId,
        place_id: placeId,
        since,
        until,
        sources,
        note: (values.note ?? "").trim(),
        private: values.private ?? false,
      });
      setResidence(updated);
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
    if (residence == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteResidence(residence.id);
      navigate("/residences");
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
          <Link to="/residences">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (residence == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  const title = `${personLabel(residence.person_id)} — ${placeLabel(residence.place_id)}`;

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Данные</Link> },
          { title: <Link to="/residences">Проживания</Link> },
          { title },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={title} column={1} bordered size="small">
            <Descriptions.Item label="Персона">
              <Link to={`/people/${residence.person_id}`}>{personLabel(residence.person_id)}</Link>
            </Descriptions.Item>
            <Descriptions.Item label="Место">
              <Link to={`/divisions/${residence.place_id}`}>{placeLabel(residence.place_id)}</Link>
            </Descriptions.Item>
            <Descriptions.Item label="Начало периода">{formatFactDate(residence.since)}</Descriptions.Item>
            <Descriptions.Item label="Конец периода">{formatFactDate(residence.until)}</Descriptions.Item>
            <Descriptions.Item label="Заметка">{residence.note || "—"}</Descriptions.Item>
            <Descriptions.Item label="Доказательства"><SourceLinkListView items={residence.sources} /></Descriptions.Item>
            <Descriptions.Item label="Приватная">{residence.private ? "да" : "нет"}</Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title="Удалить проживание?"
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
          <Form.Item label="Персона" required>
            <PersonPicker value={personId} onChange={setPersonId} placeholder="Персона" />
          </Form.Item>
          <Form.Item label="Место" required>
            <Select
              style={{ width: 320 }}
              placeholder="Административное деление"
              options={divisionOptions}
              value={placeId || undefined}
              onChange={setPlaceId}
              showSearch
              filterOption={(input, option) =>
                (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
              }
            />
          </Form.Item>
          <Form.Item label="Начало периода">
            <FactDateEditor value={since} onChange={setSince} addLabel="+ дата начала" />
          </Form.Item>
          <Form.Item label="Конец периода">
            <FactDateEditor value={until} onChange={setUntil} addLabel="+ дата конца" />
          </Form.Item>
          <Form.Item name="note" label="Заметка">
            <Input.TextArea rows={3} />
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
