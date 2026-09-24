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
  Space,
  Spin,
  Typography,
} from "antd";
import {
  deleteEvent,
  fetchEvent,
  updateEvent,
  type Event,
  type EventParticipant,
  type FactDate,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError, type ApiErrorReferrer } from "../auth";
import { useSession } from "../session";
import { EventParticipantListEditor } from "../EventParticipantList";
import { FactDateEditor, formatFactDate } from "../FactDateEditor";
import { personDisplayName } from "../PersonNameList";
import { usePersonOptions } from "../PersonPicker";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";
import { eventLabel } from "./EventsList";

interface EditFormValues {
  type: string;
  placeText?: string;
  private?: boolean;
}

const EDIT_FORM_FIELDS: (keyof EditFormValues)[] = ["type"];

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

// ParticipantListView — read-only отображение Event.Participants (личность
// участника — ссылка на /people/{id} с отображаемым именем, роль и заметка
// подписью рядом); редактируется EventParticipantListEditor в форме ниже.
function ParticipantListView({
  items,
  personLabel,
}: {
  items: EventParticipant[];
  personLabel: (id: string) => string;
}) {
  if (items.length === 0) {
    return <Typography.Text type="secondary">—</Typography.Text>;
  }
  return (
    <List
      size="small"
      dataSource={items}
      renderItem={(p, i) => (
        <List.Item key={i}>
          <Link to={`/people/${p.person_id}`}>{personLabel(p.person_id)}</Link>
          {` — ${p.role}`}
          {p.note ? ` (${p.note})` : ""}
        </List.Item>
      )}
    />
  );
}

// EventView — просмотр события, переключаемый в форму редактирования на той
// же странице (toggle+explicit-save). place показывается текстом; если у
// него уже есть ref — вторичная пометка «→ Type ID» (не кликабельно, picker
// не реализован — Place никогда не проверяется на существование на сервере,
// docs/data-model/entity-write.md §3.8). Сохранение ref при неизменном
// тексте — ТОЧНО тот же приём, что ChurchView.onSave (parishText/
// parishHasRef), просто переименованный под PlaceRef/place.
export default function EventView() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const { session } = useSession();

  const [event, setEvent] = useState<Event | null>(null);
  const [loading, setLoading] = useState(true);
  const [notFound, setNotFound] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const people = usePersonOptions();

  const [editing, setEditing] = useState(false);
  const [form] = Form.useForm<EditFormValues>();
  const [date, setDate] = useState<FactDate | null>(null);
  const [participants, setParticipants] = useState<EventParticipant[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const [deleting, setDeleting] = useState(false);
  const [conflict, setConflict] = useState<ApiErrorReferrer[] | null>(null);

  const load = (eventId: string) => {
    setLoading(true);
    setNotFound(false);
    setError(null);
    setEvent(null);
    fetchEvent(eventId)
      .then(setEvent)
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
    if (event == null) {
      return;
    }
    form.setFieldsValue({
      type: event.type,
      placeText: event.place?.text ?? "",
      private: event.private,
    });
    setDate(event.date ?? null);
    setParticipants(event.participants);
    setNotes(event.notes);
    setSources(event.sources);
    setSaveError(null);
    setEditing(true);
  };

  const cancelEdit = () => {
    setEditing(false);
    setSaveError(null);
  };

  const onSave = async (values: EditFormValues) => {
    if (event == null) {
      return;
    }
    setSaving(true);
    setSaveError(null);
    try {
      const placeText = (values.placeText ?? "").trim();
      const placeHasRef = event.place?.ref != null && event.place.ref !== "";
      const updated = await updateEvent(event.id, {
        type: values.type,
        date,
        // Сохраняем существующую ссылку, если поле не тронуто (тот же текст)
        // и уже несло ref; иначе — чистый текст без ref (picker не
        // реализован — по образцу ChurchView.onSave/parishText).
        place: placeHasRef && placeText === event.place?.text
          ? event.place
          : placeText
            ? { text: placeText }
            : null,
        participants,
        sources,
        notes,
        private: values.private ?? false,
      });
      setEvent(updated);
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
    if (event == null) {
      return;
    }
    setDeleting(true);
    setError(null);
    try {
      await deleteEvent(event.id);
      navigate("/events");
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
          <Link to="/events">
            <Button size="small">К списку</Button>
          </Link>
        }
      />
    );
  }

  if (event == null) {
    return error != null ? <Alert type="error" showIcon message={error} /> : null;
  }

  const title = eventLabel(event);

  return (
    <Card>
      <Breadcrumb
        style={{ marginBottom: 16 }}
        items={[
          { title: <Link to="/">Данные</Link> },
          { title: <Link to="/events">События</Link> },
          { title },
        ]}
      />
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}

      {!editing ? (
        <>
          <Descriptions title={title} column={1} bordered size="small">
            <Descriptions.Item label="Вид события">{event.type}</Descriptions.Item>
            <Descriptions.Item label="Дата">{formatFactDate(event.date)}</Descriptions.Item>
            <Descriptions.Item label="Место">
              {event.place == null ? (
                "—"
              ) : (
                <>
                  {event.place.text}
                  {event.place.ref && (
                    <Typography.Text type="secondary"> → {event.place.type} {event.place.ref}</Typography.Text>
                  )}
                </>
              )}
            </Descriptions.Item>
            <Descriptions.Item label="Участники">
              <ParticipantListView items={event.participants} personLabel={personLabel} />
            </Descriptions.Item>
            <Descriptions.Item label="Заметки"><TextRefListView items={event.notes} /></Descriptions.Item>
            <Descriptions.Item label="Доказательства"><SourceLinkListView items={event.sources} /></Descriptions.Item>
            <Descriptions.Item label="Приватная">{event.private ? "да" : "нет"}</Descriptions.Item>
          </Descriptions>
          {session != null && (
            <Space style={{ marginTop: 16 }}>
              <Button onClick={startEdit}>Редактировать</Button>
              <Popconfirm
                title="Удалить событие?"
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
          <Form.Item
            name="type"
            label="Вид события"
            rules={[{ required: true, whitespace: true, message: "Укажите вид события" }]}
          >
            <Input placeholder="birth / death / marriage / burial / confession / census" />
          </Form.Item>
          <Form.Item label="Дата">
            <FactDateEditor value={date} onChange={setDate} addLabel="+ дата" />
          </Form.Item>
          <Form.Item name="placeText" label="Место (текстом)">
            <Input placeholder="село Давыдово" />
          </Form.Item>
          <Form.Item label="Участники">
            <EventParticipantListEditor value={participants} onChange={setParticipants} addLabel="+ участник" />
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
