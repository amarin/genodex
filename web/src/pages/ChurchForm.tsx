import { useState } from "react";
import { Alert, Form, Input, Modal } from "antd";
import { createChurch, type Church, type SourceLink, type TextRef } from "../api";
import { ApiError } from "../auth";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";

interface ChurchFormValues {
  name: string;
  parishText?: string;
}

const FORM_FIELDS: (keyof ChurchFormValues)[] = ["name"];

// CreateChurchModal — форма создания церкви. parish — одиночная необязательная
// ссылка (text-only в v1, как элементы TextRef-списков — обычный текстовый
// инпут вместо TextRefListEditor, т.к. поле одно, не список). settlements —
// TextRef-список (населённые пункты, ссылки на AdministrativeDivision — v1
// текстом). variants — простые строки (не TextRef), свой список без
// возможности нести ссылку.
export function CreateChurchModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Church) => void;
}) {
  const [form] = Form.useForm<ChurchFormValues>();
  const [settlements, setSettlements] = useState<TextRef[]>([]);
  const [variants, setVariants] = useState<TextRef[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setSettlements([]);
    setVariants([]);
    setNotes([]);
    setSources([]);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: ChurchFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const parishText = (values.parishText ?? "").trim();
      const created = await createChurch({
        name: values.name,
        parish: parishText ? { text: parishText } : null,
        settlements,
        variants: variants.map((v) => v.text).filter((t) => t.trim() !== ""),
        notes,
        sources,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof ChurchFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить церковь"
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
        <Form.Item name="parishText" label="Приход (текстом)">
          <Input placeholder="Никольский приход" />
        </Form.Item>
        <Form.Item label="Населённые пункты">
          <TextRefListEditor value={settlements} onChange={setSettlements} addLabel="+ населённый пункт" />
        </Form.Item>
        <Form.Item label="Варианты названия">
          <TextRefListEditor value={variants} onChange={setVariants} addLabel="+ вариант" />
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
