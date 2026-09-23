import { useState } from "react";
import { Alert, Checkbox, Form, Input, Modal } from "antd";
import { createRepository, type Repository, type SourceLink, type TextRef } from "../api";
import { ApiError } from "../auth";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";

interface RepositoryFormValues {
  name: string;
  type: string;
  address?: string;
  private?: boolean;
}

const FORM_FIELDS: (keyof RepositoryFormValues)[] = ["name", "type", "address"];

// CreateRepositoryModal — форма создания хранилища-контейнера источников.
// type — открытый список (любая строка формата [a-z][a-z0-9_-]*, а не
// фиксированный enum вроде AdminDivisionType/Gender) — обычный текстовый
// инпут, не Select (docs/data-model/entity-write.md, models/repository_type.go
// перечисляет типовые значения archive/library/museum/private/other как
// подсказку, но допустимы и другие). urls/notes редактируются вне antd Form
// (TextRefListEditor), sources — тем же паттерном (SourceLinkListEditor),
// что и у остальных пяти ретрофитнутых сущностей (подпроект 5).
export function CreateRepositoryModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Repository) => void;
}) {
  const [form] = Form.useForm<RepositoryFormValues>();
  const [urls, setUrls] = useState<TextRef[]>([]);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setUrls([]);
    setNotes([]);
    setSources([]);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: RepositoryFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await createRepository({
        name: values.name,
        type: values.type,
        address: values.address ?? "",
        urls,
        notes,
        sources,
        private: values.private ?? false,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof RepositoryFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить хранилище"
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
        <Form.Item
          name="type"
          label="Тип"
          rules={[{ required: true, whitespace: true, message: "Введите тип (archive/library/museum/private/other или другой)" }]}
        >
          <Input placeholder="archive" />
        </Form.Item>
        <Form.Item name="address" label="Адрес">
          <Input />
        </Form.Item>
        <Form.Item label="Ссылки">
          <TextRefListEditor value={urls} onChange={setUrls} addLabel="+ ссылка" />
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
