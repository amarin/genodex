import { useState } from "react";
import { Alert, Form, Input, Modal, Select } from "antd";
import { createGivenName, type GivenName, type NameGender, type TextRef } from "../api";
import { ApiError } from "../auth";
import { TextRefListEditor } from "../TextRefList";

interface GivenNameFormValues {
  canonical: string;
  gender: NameGender;
}

const FORM_FIELDS: (keyof GivenNameFormValues)[] = ["canonical", "gender"];

const GENDER_OPTIONS: { value: NameGender; label: string }[] = [
  { value: "male", label: "Мужской" },
  { value: "female", label: "Женский" },
  { value: "neutral", label: "Нейтральный (вывод пола запрещён)" },
];

// CreateGivenNameModal — форма создания словарной записи имени. gender —
// единственное отличие GivenName от остальных словарей (обязателен,
// docs/data-model/entity-write.md, internal/models/given_name.go). variants/
// items/notes редактируются вне antd Form (TextRefListEditor — не обычный
// текстовый инпут), собираются в одно тело запроса на submit.
export function CreateGivenNameModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: GivenName) => void;
}) {
  const [form] = Form.useForm<GivenNameFormValues>();
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

  const onFinish = async (values: GivenNameFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await createGivenName({
        canonical: values.canonical,
        gender: values.gender,
        variants,
        items,
        notes,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof GivenNameFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить имя"
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
        <Form.Item
          name="gender"
          label="Пол"
          rules={[{ required: true, message: "Выберите пол" }]}
        >
          <Select options={GENDER_OPTIONS} placeholder="Выберите пол" />
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
