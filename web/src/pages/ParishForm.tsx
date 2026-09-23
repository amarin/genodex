import { useState } from "react";
import { Alert, Form, Input, Modal } from "antd";
import { createParish, type FactDate, type Parish, type SourceLink, type TextRef } from "../api";
import { ApiError } from "../auth";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";
import { FactDateEditor } from "../FactDateEditor";

interface ParishFormValues {
  name: string;
  churchText?: string;
}

const FORM_FIELDS: (keyof ParishFormValues)[] = ["name"];

// CreateParishModal — форма создания прихода. church — одиночная
// необязательная ссылка (text-only в v1). since/until — структурированная
// дата (FactDateEditor, первое появление в проекте).
export function CreateParishModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Parish) => void;
}) {
  const [form] = Form.useForm<ParishFormValues>();
  const [settlements, setSettlements] = useState<TextRef[]>([]);
  const [since, setSince] = useState<FactDate | null>(null);
  const [until, setUntil] = useState<FactDate | null>(null);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setSettlements([]);
    setSince(null);
    setUntil(null);
    setNotes([]);
    setSources([]);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: ParishFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const churchText = (values.churchText ?? "").trim();
      const created = await createParish({
        name: values.name,
        church: churchText ? { text: churchText } : null,
        settlements,
        since,
        until,
        notes,
        sources,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof ParishFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить приход"
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
          name="name"
          label="Название"
          rules={[{ required: true, whitespace: true, message: "Введите название" }]}
        >
          <Input autoFocus />
        </Form.Item>
        <Form.Item name="churchText" label="Церковь (текстом)">
          <Input placeholder="Никольская церковь" />
        </Form.Item>
        <Form.Item label="Населённые пункты">
          <TextRefListEditor value={settlements} onChange={setSettlements} addLabel="+ населённый пункт" />
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
      </Form>
    </Modal>
  );
}
