import { useState } from "react";
import { Alert, Form, Input, Modal, Select } from "antd";
import {
  ADMIN_DIVISION_TYPE_LABELS,
  createDivision,
  type AdminDivision,
  type AdminDivisionType,
  type SourceLink,
} from "../api";
import { ApiError } from "../auth";
import { SourceLinkListEditor } from "../SourceLinkList";

export const TYPE_OPTIONS = (Object.keys(ADMIN_DIVISION_TYPE_LABELS) as AdminDivisionType[]).map(
  (value) => ({ value, label: ADMIN_DIVISION_TYPE_LABELS[value] }),
);

interface DivisionFormValues {
  name: string;
  type: AdminDivisionType;
}

const FORM_FIELDS: (keyof DivisionFormValues)[] = ["name", "type"];

// CreateDivisionModal — форма создания единицы, используется и на
// DivisionsList (parentId=null — корень) и на DivisionView (parentId —
// текущая единица, «добавить дочернюю»). Единица создаётся с фиксированным
// parent_id из пропа: сам parent_id полем формы не является.
export function CreateDivisionModal({
  open,
  parentId,
  onClose,
  onCreated,
}: {
  open: boolean;
  parentId: string | null;
  onClose: () => void;
  onCreated: (created: AdminDivision) => void;
}) {
  const [form] = Form.useForm<DivisionFormValues>();
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const handleClose = () => {
    form.resetFields();
    setSources([]);
    setError(null);
    onClose();
  };

  const onFinish = async (values: DivisionFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await createDivision({
        name: values.name,
        type: values.type,
        parent_id: parentId,
        sources,
      });
      form.resetFields();
      setSources([]);
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof DivisionFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать единицу");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title={parentId == null ? "Добавить в корень" : "Добавить дочернюю единицу"}
      open={open}
      onCancel={handleClose}
      onOk={() => form.submit()}
      okText="Создать"
      cancelText="Отмена"
      confirmLoading={submitting}
      destroyOnHidden
    >
      {error != null && (
        <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />
      )}
      <Form form={form} layout="vertical" onFinish={onFinish}>
        <Form.Item
          name="name"
          label="Название"
          rules={[{ required: true, whitespace: true, message: "Введите название" }]}
        >
          <Input autoFocus />
        </Form.Item>
        <Form.Item name="type" label="Тип" rules={[{ required: true, message: "Выберите тип" }]}>
          <Select options={TYPE_OPTIONS} placeholder="Выберите тип" />
        </Form.Item>
        <Form.Item label="Доказательства">
          <SourceLinkListEditor value={sources} onChange={setSources} addLabel="+ доказательство" />
        </Form.Item>
      </Form>
    </Modal>
  );
}
