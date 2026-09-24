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
  List,
  Modal,
  Popconfirm,
  Select,
  Space,
  Spin,
  Typography,
} from "antd";
import {
  deletePerson,
  fetchPerson,
  GENDER_OPTIONS,
  genderLabel,
  updatePerson,
  type Person,
  type PersonGender,
  type PersonName,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { formatFactDate } from "../FactDateEditor";
import { formatPersonName, personNameTypeLabel, PersonNameListEditor, pickDisplayName } from "../PersonNameList";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";

interface EditFormValues {
  gender?: PersonGender;
  private?: boolean;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["gender"];

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

// SourceLinkListView — read-only отображение списка доказательств (Sources);
// редактируется отдельным SourceLinkListEditor в форме ниже.
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

// personNameLine — вид имени + "Фамилия Имя Отчество" + приставка/суффикс,
// если заданы, + период действия (formatFactDate) — та же формула, что
// personLabel в PeopleList.tsx для одного имени, плюс служебные части и
// период для полного просмотра.
function personNameLine(n: PersonName): string {
  const parts: string[] = [];
  const typeLabel = personNameTypeLabel(n.type);
  if (typeLabel !== "") {
    parts.push(typeLabel);
  }
  const name = formatPersonName(n);
  parts.push(name !== "" ? name : "—");
  if (n.prefix) {
    parts.push(n.prefix);
  }
  if (n.suffix) {
    parts.push(n.suffix);
  }
  const since = n.since != null ? formatFactDate(n.since) : null;
  const until = n.until != null ? formatFactDate(n.until) : null;
  if (since != null || until != null) {
    parts.push(`(${since ?? "…"} – ${until ?? "…"})`);
  }
  return parts.join(" — ");
}

// PersonNameListView — read-only отображение Person.names; редактируется
// PersonNameListEditor в форме ниже.
function PersonNameListView({ items }: { items: PersonName[] }) {
  if (items.length === 0) {
    return <Typography.Text type="secondary">—</Typography.Text>;
  }
  return (
    <List
      size="small"
      dataSource={items}
      renderItem={(n, i) => <List.Item key={i}>{personNameLine(n)}</List.Item>}
    />
  );
}

// PersonView — просмотр персоны, переключаемый в форму редактирования на
// той же странице (toggle+explicit-save, как FamilyView). Заголовок карточки
// — отображаемое имя (pickDisplayName), а не поле name (у Person его нет).
export default function PersonView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [person, setPerson] = useState<Person | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [names, setNames] = useState<PersonName[]>([]);
  const [estates, setEstates] = useState<TextRef[]>([]);
  const [titles, setTitles] = useState<TextRef[]>([]);
  const [nicknames, setNicknames] = useState<TextRef[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (personId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setPerson(null);
    fetchPerson(personId)
      .then(setPerson)
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
    if (person == null) {
      return;
    }
    form.setFieldsValue({
      gender: (person.gender ?? "") as PersonGender,
      private: person.private,
    });
    setNames(person.names);
    setEstates(person.estates);
    setTitles(person.titles);
    setNicknames(person.nicknames);
    setNotes(person.notes);
    setSources(person.sources);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (person == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await updatePerson(person.id, {
        gender: values.gender ?? "",
        names,
        estates,
        titles,
        nicknames,
        notes,
        sources,
        private: values.private ?? false,
      });
      setPerson(updated);
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
    if (person == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deletePerson(person.id);
      navigate("/people");
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
          <Link to="/people">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (person == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  const displayName = pickDisplayName(person.names);
  const title = displayName != null && formatPersonName(displayName) !== "" ? formatPersonName(displayName) : person.id;

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Сущности</Link> },
          { title: <Link to="/people">Персоны</Link> },
          { title },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={title} column={1} bordered size="small">
            <Descriptions.Item label="Пол">{genderLabel(person.gender)}</Descriptions.Item>
            <Descriptions.Item label="Имена"><PersonNameListView items={person.names} /></Descriptions.Item>
            <Descriptions.Item label="Сословия"><TextRefListView items={person.estates} /></Descriptions.Item>
            <Descriptions.Item label="Титулы"><TextRefListView items={person.titles} /></Descriptions.Item>
            <Descriptions.Item label="Прозвища"><TextRefListView items={person.nicknames} /></Descriptions.Item>
            <Descriptions.Item label="Заметки"><TextRefListView items={person.notes} /></Descriptions.Item>
            <Descriptions.Item label="Доказательства"><SourceLinkListView items={person.sources} /></Descriptions.Item>
            <Descriptions.Item label="Приватная">{person.private ? "да" : "нет"}</Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title={`Удалить «${title}»?`}
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
        <Form form={form} layout="vertical" onFinish={onSave} style={{ maxWidth: 720 }}>
          {saveError != null && (
            <Alert type="error" showIcon message={saveError} style={{ marginBottom: 16 }} />
          )}
          <Form.Item name="gender" label="Пол">
            <Select options={GENDER_OPTIONS} />
          </Form.Item>
          <Form.Item label="Имена">
            <PersonNameListEditor value={names} onChange={setNames} addLabel="+ имя" />
          </Form.Item>
          <Form.Item label="Сословия">
            <TextRefListEditor value={estates} onChange={setEstates} addLabel="+ сословие" />
          </Form.Item>
          <Form.Item label="Титулы">
            <TextRefListEditor value={titles} onChange={setTitles} addLabel="+ титул" />
          </Form.Item>
          <Form.Item label="Прозвища">
            <TextRefListEditor value={nicknames} onChange={setNicknames} addLabel="+ прозвище" />
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
