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
  deleteRelation,
  fetchRelation,
  relationKindLabel,
  RELATION_KIND_OPTIONS,
  updateRelation,
  type FactDate,
  type Relation,
  type RelationKind,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { FactDateEditor, formatFactDate } from "../FactDateEditor";
import { personDisplayName } from "../PersonNameList";
import { PersonPicker, usePersonOptions } from "../PersonPicker";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";

interface EditFormValues {
  kind: RelationKind;
  rel_type?: string;
  private?: boolean;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["kind", "rel_type"];

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

// RelationView — просмотр связи, переключаемый в форму редактирования на
// той же странице (toggle+explicit-save, как FamilyView/PersonView).
// person_a/person_b показываются кликабельной ссылкой на /people/{id} с
// отображаемым именем (personDisplayName) — тот же приём, что
// repositoryName в ArchiveView.tsx, только со ссылкой. rel_type
// показывается/редактируется только при kind=associate.
export default function RelationView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [relation, setRelation] = useState<Relation | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const people = usePersonOptions();

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const kind = Form.useWatch("kind", form);
  const [personA, setPersonA] = useState("");
  const [personB, setPersonB] = useState("");
  const [since, setSince] = useState<FactDate | null>(null);
  const [until, setUntil] = useState<FactDate | null>(null);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (relationId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setRelation(null);
    fetchRelation(relationId)
      .then(setRelation)
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

  const personLabel = (personId: string) => {
    const p = people.find((person) => person.id === personId);
    return p != null ? personDisplayName(p) : personId;
  };

  const startEdit = () => {
    if (relation == null) {
      return;
    }
    form.setFieldsValue({
      kind: relation.kind,
      rel_type: relation.rel_type ?? "",
      private: relation.private,
    });
    setPersonA(relation.person_a);
    setPersonB(relation.person_b);
    setSince(relation.since ?? null);
    setUntil(relation.until ?? null);
    setNotes(relation.notes);
    setSources(relation.sources);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (relation == null) {
      return;
    }
    setSaveError(null);
    if (personA !== "" && personA === personB) {
      setSaveError("Связь не может быть с самим собой: персона A и персона Б должны отличаться");
      return;
    }
    setSaving(true);
    try {
      const updated = await updateRelation(relation.id, {
        kind: values.kind,
        rel_type: values.kind === "associate" ? (values.rel_type ?? "").trim() : "",
        person_a: personA,
        person_b: personB,
        since,
        until,
        sources,
        notes,
        private: values.private ?? false,
      });
      setRelation(updated);
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
    if (relation == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteRelation(relation.id);
      navigate("/relations");
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
          <Link to="/relations">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (relation == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  const title = `${relationKindLabel(relation.kind)}: ${personLabel(relation.person_a)} — ${personLabel(relation.person_b)}`;

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Данные</Link> },
          { title: <Link to="/relations">Связи</Link> },
          { title },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={title} column={1} bordered size="small">
            <Descriptions.Item label="Вид связи">{relationKindLabel(relation.kind)}</Descriptions.Item>
            {relation.kind === "associate" && (
              <Descriptions.Item label="Тип связи">{relation.rel_type || "—"}</Descriptions.Item>
            )}
            <Descriptions.Item label="Персона A">
              <Link to={`/people/${relation.person_a}`}>{personLabel(relation.person_a)}</Link>
            </Descriptions.Item>
            <Descriptions.Item label="Персона Б">
              <Link to={`/people/${relation.person_b}`}>{personLabel(relation.person_b)}</Link>
            </Descriptions.Item>
            <Descriptions.Item label="Начало периода">{formatFactDate(relation.since)}</Descriptions.Item>
            <Descriptions.Item label="Конец периода">{formatFactDate(relation.until)}</Descriptions.Item>
            <Descriptions.Item label="Заметки"><TextRefListView items={relation.notes} /></Descriptions.Item>
            <Descriptions.Item label="Доказательства"><SourceLinkListView items={relation.sources} /></Descriptions.Item>
            <Descriptions.Item label="Приватная">{relation.private ? "да" : "нет"}</Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title="Удалить связь?"
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
          <Form.Item name="kind" label="Вид связи" rules={[{ required: true, message: "Выберите вид связи" }]}>
            <Select options={RELATION_KIND_OPTIONS} />
          </Form.Item>
          {kind === "associate" && (
            <Form.Item
              name="rel_type"
              label="Тип связи"
              rules={[{ required: true, whitespace: true, message: "Укажите тип связи" }]}
            >
              <Input placeholder="godparent / witness / neighbor / friend / colleague" />
            </Form.Item>
          )}
          <Form.Item label="Персона A" required>
            <PersonPicker value={personA} onChange={setPersonA} placeholder="Персона A" />
          </Form.Item>
          <Form.Item label="Персона Б" required>
            <PersonPicker value={personB} onChange={setPersonB} placeholder="Персона Б" />
          </Form.Item>
          <Form.Item label="Начало периода">
            <FactDateEditor value={since} onChange={setSince} addLabel="+ дата начала" />
          </Form.Item>
          <Form.Item label="Конец периода">
            <FactDateEditor value={until} onChange={setUntil} addLabel="+ дата конца" />
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
