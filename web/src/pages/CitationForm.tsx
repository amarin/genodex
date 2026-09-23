import { useEffect, useState } from "react";
import { Alert, Checkbox, Form, Input, Modal, Select } from "antd";
import { createCitation, fetchSources, type Anchor, type Citation, type Source } from "../api";
import { ApiError } from "../auth";
import { AnchorEditor } from "../AnchorEditor";

interface CitationFormValues {
  source_id: string;
  text?: string;
  note?: string;
  private?: boolean;
}

const FORM_FIELDS: (keyof CitationFormValues)[] = ["source_id", "text", "note"];

// useSourceOptions — заполняет Select источников для source_id, по образцу
// useRepositoryOptions/ArchiveForm.tsx.
function useSourceOptions() {
  const [sources, setSources] = useState<Source[]>([]);

  useEffect(() => {
    fetchSources({ limit: 500 })
      .then(setSources)
      .catch(() => setSources([]));
  }, []);

  return sources.map((s) => ({ value: s.id, label: s.title }));
}

// CreateCitationModal — форма создания цитаты. anchor — AnchorEditor
// (полиморфная привязка, необязательна).
export function CreateCitationModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Citation) => void;
}) {
  const [form] = Form.useForm<CitationFormValues>();
  const [anchor, setAnchor] = useState<Anchor | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const sourceOptions = useSourceOptions();

  const reset = () => {
    form.resetFields();
    setAnchor(null);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: CitationFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await createCitation({
        source_id: values.source_id,
        anchor,
        text: values.text,
        note: values.note,
        private: values.private ?? false,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof CitationFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить цитату"
      open={open}
      onCancel={handleClose}
      onOk={() => form.submit()}
      okText="Создать"
      cancelText="Отмена"
      confirmLoading={submitting}
      destroyOnHidden
    >
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
      <Form form={form} layout="vertical" onFinish={onFinish} initialValues={{ private: false }}>
        <Form.Item
          name="source_id"
          label="Источник"
          rules={[{ required: true, message: "Выберите источник" }]}
        >
          <Select
            options={sourceOptions}
            showSearch
            filterOption={(input, option) =>
              (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
            }
          />
        </Form.Item>
        <Form.Item name="text" label="Текст выписки">
          <Input.TextArea rows={4} />
        </Form.Item>
        <Form.Item label="Привязка">
          <AnchorEditor value={anchor} onChange={setAnchor} addLabel="+ привязка" />
        </Form.Item>
        <Form.Item name="note" label="Заметка">
          <Input />
        </Form.Item>
        <Form.Item name="private" valuePropName="checked">
          <Checkbox>Приватная запись</Checkbox>
        </Form.Item>
      </Form>
    </Modal>
  );
}
