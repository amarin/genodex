import { useEffect, useState } from "react";
import { Alert, Checkbox, Form, Input, Modal, Select } from "antd";
import { createNote, fetchNotes, type Note } from "../api";
import { ApiError } from "../auth";

interface NoteFormValues {
  kind: string;
  title?: string;
  text?: string;
  parent_id?: string;
  private?: boolean;
}

const FORM_FIELDS: (keyof NoteFormValues)[] = ["kind", "title", "text", "parent_id"];

// useNoteOptions — заполняет Select заметок для parent_id: плоский список +
// Select (не Tree — Note не показывается иерархией в v1, обсуждение
// подпроекта 4), по образцу useRepositoryOptions у Archive.
function useNoteOptions(excludeID?: string) {
  const [notes, setNotes] = useState<Note[]>([]);

  useEffect(() => {
    fetchNotes({ limit: 500 })
      .then(setNotes)
      .catch(() => setNotes([]));
  }, []);

  return notes
    .filter((n) => n.id !== excludeID)
    .map((n) => ({ value: n.id, label: n.title || n.text?.slice(0, 60) || n.id }));
}

// CreateNoteModal — форма создания заметки. parent_id — Select со списком
// заметок (существование и отсутствие циклов проверяет сценарий, при
// создании цикл невозможен — новая запись ещё ничьим предком быть не может).
export function CreateNoteModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Note) => void;
}) {
  const [form] = Form.useForm<NoteFormValues>();
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const noteOptions = useNoteOptions();

  const reset = () => {
    form.resetFields();
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: NoteFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await createNote({
        kind: values.kind,
        title: values.title,
        text: values.text,
        parent_id: values.parent_id ?? "",
        private: values.private ?? false,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof NoteFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить заметку"
      open={open}
      onCancel={handleClose}
      onOk={() => form.submit()}
      okText="Создать"
      cancelText="Отмена"
      confirmLoading={submitting}
      destroyOnHidden
    >
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
      <Form form={form} layout="vertical" onFinish={onFinish} initialValues={{ kind: "note", private: false }}>
        <Form.Item
          name="kind"
          label="Вид"
          rules={[{ required: true, whitespace: true, message: "Введите вид (например, note)" }]}
        >
          <Input placeholder="note / article / book / chapter" autoFocus />
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
        <Form.Item name="private" valuePropName="checked">
          <Checkbox>Приватная запись</Checkbox>
        </Form.Item>
      </Form>
    </Modal>
  );
}
