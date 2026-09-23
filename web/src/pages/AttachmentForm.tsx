import { useState } from "react";
import { Alert, Checkbox, Form, Input, InputNumber, Modal, Select } from "antd";
import { createAttachment, type Attachment } from "../api";
import { ApiError } from "../auth";
import { ArchiveDocumentSelect, ArchiveNodePicker } from "../ArchiveNodePicker";

interface AttachmentFormValues {
  kind: string;
  uri?: string;
  filename?: string;
  mime?: string;
  page?: number;
  note?: string;
  private?: boolean;
}

// FORM_FIELDS — node_id/document_id НЕ входят: они управляются отдельным
// состоянием (nodeId/documentId, не полями antd Form — тот же приём, что
// editParentId в ArchiveNodeView.tsx/editUnitId в ArchiveDocumentForm.tsx),
// поэтому 422 на них попадает в общий error, а не в конкретное поле формы.
const FORM_FIELDS: (keyof AttachmentFormValues)[] = ["kind", "uri", "filename", "mime", "page", "note"];

const KIND_OPTIONS = [
  { value: "scan", label: "скан" },
  { value: "document", label: "документ" },
  { value: "audio", label: "аудио" },
  { value: "photo", label: "фото" },
];

// CreateAttachmentModal — форма создания вложения. node_id/document_id —
// ArchiveNodePicker + ArchiveDocumentSelect (подпроект 6 — ArchiveNode/
// ArchiveDocument теперь имеют CRUD и полноценный picker/Select, см.
// ArchiveNodePicker.tsx). node_id обязателен, document_id — нет, и
// становится доступен только после выбора узла (документ должен
// принадлежать выбранному узлу).
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
  const [nodeId, setNodeId] = useState("");
  const [nodeLabel, setNodeLabel] = useState<string | null>(null);
  const [documentId, setDocumentId] = useState<string | undefined>(undefined);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setNodeId("");
    setNodeLabel(null);
    setDocumentId(undefined);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: AttachmentFormValues) => {
    if (!nodeId) {
      setError("Выберите архивный узел");
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      const created = await createAttachment({
        kind: values.kind,
        uri: values.uri,
        filename: values.filename,
        mime: values.mime,
        page: values.page,
        node_id: nodeId,
        document_id: documentId,
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
        <Form.Item label="Архивный узел" required>
          <ArchiveNodePicker
            value={nodeId}
            label={nodeLabel ?? undefined}
            onChange={(id, lbl) => {
              setNodeId(id);
              setNodeLabel(lbl);
              setDocumentId(undefined);
            }}
          />
        </Form.Item>
        <Form.Item label="Архивный документ (необязательно)">
          <ArchiveDocumentSelect nodeId={nodeId || null} value={documentId} onChange={setDocumentId} />
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
