import { useState } from "react";
import { Alert, Checkbox, Form, Input, InputNumber, Modal, Select } from "antd";
import { createAttachment, type Attachment } from "../api";
import { ApiError } from "../auth";

interface AttachmentFormValues {
  kind: string;
  uri?: string;
  filename?: string;
  mime?: string;
  page?: number;
  node_id: string;
  document_id?: string;
  note?: string;
  private?: boolean;
}

const FORM_FIELDS: (keyof AttachmentFormValues)[] = [
  "kind",
  "uri",
  "filename",
  "mime",
  "page",
  "node_id",
  "document_id",
  "note",
];

const KIND_OPTIONS = [
  { value: "scan", label: "скан" },
  { value: "document", label: "документ" },
  { value: "audio", label: "аудио" },
  { value: "photo", label: "фото" },
];

// CreateAttachmentModal — форма создания вложения. node_id/document_id — в
// v1 обычные текстовые поля ввода id (без picker'а): ArchiveNode/
// ArchiveDocument ещё не имеют своего списка, чтобы искать по нему (подпроект
// 6 добавит их CRUD и вернёт сюда полноценный Select). node_id обязателен,
// document_id — нет.
export function CreateAttachmentModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Attachment) => void;
}) {
  const [form] = Form.useForm<AttachmentFormValues>();
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: AttachmentFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await createAttachment({
        kind: values.kind,
        uri: values.uri,
        filename: values.filename,
        mime: values.mime,
        page: values.page,
        node_id: values.node_id,
        document_id: values.document_id,
        note: values.note,
        private: values.private ?? false,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof AttachmentFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить вложение"
      open={open}
      onCancel={handleClose}
      onOk={() => form.submit()}
      okText="Создать"
      cancelText="Отмена"
      confirmLoading={submitting}
      destroyOnHidden
    >
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
      <Form form={form} layout="vertical" onFinish={onFinish} initialValues={{ kind: "scan", private: false }}>
        <Form.Item name="kind" label="Вид" rules={[{ required: true, message: "Выберите вид" }]}>
          <Select options={KIND_OPTIONS} />
        </Form.Item>
        <Form.Item name="filename" label="Имя файла">
          <Input autoFocus />
        </Form.Item>
        <Form.Item name="uri" label="URI">
          <Input placeholder="файл, ссылка" />
        </Form.Item>
        <Form.Item name="mime" label="MIME-тип">
          <Input placeholder="image/jpeg" />
        </Form.Item>
        <Form.Item name="page" label="Страница">
          <InputNumber min={0} style={{ width: "100%" }} />
        </Form.Item>
        <Form.Item
          name="node_id"
          label="Архивный узел (id)"
          rules={[{ required: true, whitespace: true, message: "Введите id архивного узла" }]}
        >
          <Input placeholder="AN-…" />
        </Form.Item>
        <Form.Item name="document_id" label="Архивный документ (id)">
          <Input placeholder="DC-… (необязательно)" />
        </Form.Item>
        <Form.Item name="note" label="Заметка">
          <Input.TextArea rows={3} />
        </Form.Item>
        <Form.Item name="private" valuePropName="checked">
          <Checkbox>Приватная запись</Checkbox>
        </Form.Item>
      </Form>
    </Modal>
  );
}
