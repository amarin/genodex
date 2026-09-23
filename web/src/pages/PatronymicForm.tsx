import { useState } from "react";
import { Alert, Form, Input, Modal } from "antd";
import { createPatronymic, type Patronymic, type TextRef } from "../api";
import { ApiError } from "../auth";
import { TextRefListEditor } from "../TextRefList";

interface PatronymicFormValues {
  canonical: string;
}

const FORM_FIELDS: (keyof PatronymicFormValues)[] = ["canonical"];

// CreatePatronymicModal — форма создания словарной записи фамилии. variants/
// items/notes редактируются вне antd Form (TextRefListEditor — не обычный
// текстовый инпут), собираются в одно тело запроса на submit.
export function CreatePatronymicModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Patronymic) => void;
}) {
  const [form] = Form.useForm<PatronymicFormValues>();
  const [variants, setVariants] = useState<TextRef[]>([]);
  const [items, setItems] = useState<TextRef[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setVariants([]);
    setItems([]);
    setNotes([]);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: PatronymicFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await createPatronymic({ canonical: values.canonical, variants, items, notes });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof PatronymicFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить отчество"
      open={open}
      onCancel={handleClose}
      onOk={() => form.submit()}
      okText="Создать"
      cancelText="Отмена"
      confirmLoading={submitting}
      destroyOnHidden
    >
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
      <Form form={form} layout="vertical" onFinish={onFinish}>
        <Form.Item
          name="canonical"
          label="Каноническая форма"
          rules={[{ required: true, whitespace: true, message: "Введите каноническую форму" }]}
        >
          <Input autoFocus />
        </Form.Item>
        <Form.Item label="Варианты написания">
          <TextRefListEditor value={variants} onChange={setVariants} addLabel="+ вариант" />
        </Form.Item>
        <Form.Item label="Носители">
          <TextRefListEditor value={items} onChange={setItems} addLabel="+ запись" />
        </Form.Item>
        <Form.Item label="Заметки">
          <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
        </Form.Item>
      </Form>
    </Modal>
  );
}
