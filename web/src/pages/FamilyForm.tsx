import { useState } from "react";
import { Alert, Checkbox, Form, Input, Modal } from "antd";
import { createFamily, type Family, type SourceLink, type TextRef } from "../api";
import { ApiError } from "../auth";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";

interface FamilyFormValues {
  name: string;
  private?: boolean;
}

const FORM_FIELDS: (keyof FamilyFormValues)[] = ["name"];

// CreateFamilyModal — форма создания рода/линии. Структурно почти идентична
// CreateRepositoryModal (подпроект 3): та же сущность минус Type/Address,
// Members — тот же TextRefListEditor, что URLs у Repository (мягкая
// ссылка на Person, без picker'а — у Person нет CRUD, подпроект 8,
// docs/data-model/entity-write.md). Sources редактируется с рождения
// контракта (SourceLinkListEditor), а не ретрофитом.
export function CreateFamilyModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Family) => void;
}) {
  const [form] = Form.useForm<FamilyFormValues>();
  const [members, setMembers] = useState<TextRef[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setMembers([]);
    setNotes([]);
    setSources([]);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: FamilyFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await createFamily({
        name: values.name,
        members,
        notes,
        sources,
        private: values.private ?? false,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof FamilyFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить род"
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
          name="name"
          label="Название"
          rules={[{ required: true, whitespace: true, message: "Введите название" }]}
        >
          <Input autoFocus />
        </Form.Item>
        <Form.Item label="Члены рода">
          <TextRefListEditor value={members} onChange={setMembers} addLabel="+ член рода" />
        </Form.Item>
        <Form.Item label="Заметки">
          <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
        </Form.Item>
        <Form.Item label="Доказательства">
          <SourceLinkListEditor value={sources} onChange={setSources} addLabel="+ доказательство" />
        </Form.Item>
        <Form.Item name="private" valuePropName="checked">
          <Checkbox>Приватная запись</Checkbox>
        </Form.Item>
      </Form>
    </Modal>
  );
}
