import { useMemo, useState } from "react";
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
import {
  allowedChildTypes,
  childKinds,
  childKindsPhrase,
  DIVISION_NAME_PLACEHOLDER,
  DIVISION_NAME_RULES,
  useAdminDivisionTypes,
} from "../divisionTypes";

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
// parentType — тип родителя: по нему фильтруются допустимые типы
// (allowedChildTypes) и выбирается заголовок; для корня — null.
export function CreateDivisionModal({
  open,
  parentId,
  parentType,
  onClose,
  onCreated,
}: {
  open: boolean;
  parentId: string | null;
  parentType: string | null;
  onClose: () => void;
  onCreated: (created: AdminDivision) => void;
}) {
  const [form] = Form.useForm<DivisionFormValues>();
  const typeInfos = useAdminDivisionTypes();
  const allowedTypes = useMemo(
    () => allowedChildTypes(typeInfos, parentType),
    [typeInfos, parentType],
  );
  const typeOptions = useMemo(
    () => TYPE_OPTIONS.filter((o) => allowedTypes.includes(o.value)),
    [allowedTypes],
  );
  const name = Form.useWatch("name", form);
  const type = Form.useWatch("type", form);
  const canSubmit =
    (name ?? "").trim() !== "" &&
    type != null &&
    allowedTypes.includes(type);
  const phrase = childKindsPhrase(childKinds(allowedTypes)) ?? "единицу";
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
      title={parentId == null ? "Добавить в корень" : `Добавить ${phrase}`}
      open={open}
      onCancel={handleClose}
      onOk={() => form.submit()}
      okText="Создать"
      cancelText="Отмена"
      okButtonProps={{ disabled: !canSubmit }}
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
          rules={DIVISION_NAME_RULES}
        >
          <Input autoFocus placeholder={DIVISION_NAME_PLACEHOLDER} />
        </Form.Item>
        <Form.Item name="type" label="Тип" rules={[{ required: true, message: "Выберите тип" }]}>
          <Select options={typeOptions} placeholder="Выберите тип" />
        </Form.Item>
        <Form.Item label="Доказательства">
          <SourceLinkListEditor value={sources} onChange={setSources} addLabel="+ доказательство" />
        </Form.Item>
      </Form>
    </Modal>
  );
}
