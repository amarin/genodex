import { useState } from "react";
import { Alert, Checkbox, Form, Input, Modal } from "antd";
import {
  createEvent,
  type Event,
  type EventParticipant,
  type FactDate,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError } from "../auth";
import { EventParticipantListEditor } from "../EventParticipantList";
import { FactDateEditor } from "../FactDateEditor";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";

interface EventFormValues {
  type: string;
  placeText?: string;
  private?: boolean;
}

const FORM_FIELDS: (keyof EventFormValues)[] = ["type"];

// CreateEventModal — форма создания события жизненного факта. type — простой
// Input (открытый enum, как ArchiveNode.type — не Select). place — простой
// текст (Event.Place — PlaceRef, НИКОГДА не проверяется на существование на
// сервере, docs/data-model/entity-write.md §3.8) — при создании ref всегда
// отсутствует, поэтому просто {text}/null без сохранения ссылки (сохранение
// ref-если-текст-не-менялся нужно только в EventView при редактировании,
// ChurchView-style). participants — EventParticipantListEditor (подпроект 9,
// первая строгая ссылка внутри массива-объектов).
export function CreateEventModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Event) => void;
}) {
  const [form] = Form.useForm<EventFormValues>();
  const [date, setDate] = useState<FactDate | null>(null);
  const [participants, setParticipants] = useState<EventParticipant[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setDate(null);
    setParticipants([]);
    setNotes([]);
    setSources([]);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: EventFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const placeText = (values.placeText ?? "").trim();
      const created = await createEvent({
        type: values.type,
        date,
        place: placeText ? { text: placeText } : null,
        participants,
        sources,
        notes,
        private: values.private ?? false,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof EventFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить событие"
      open={open}
      onCancel={handleClose}
      onOk={() => form.submit()}
      okText="Создать"
      cancelText="Отмена"
      confirmLoading={submitting}
      destroyOnHidden
      width={720}
    >
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
      <Form form={form} layout="vertical" onFinish={onFinish} initialValues={{ private: false }}>
        <Form.Item
          name="type"
          label="Вид события"
          rules={[{ required: true, whitespace: true, message: "Укажите вид события" }]}
        >
          <Input placeholder="birth / death / marriage / burial / confession / census" autoFocus />
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
      </Form>
    </Modal>
  );
}
