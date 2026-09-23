import { useEffect, useState } from "react";
import { Alert, Checkbox, Form, Input, Modal, Select } from "antd";
import { createArchive, fetchRepositories, type Archive, type Repository, type TextRef } from "../api";
import { ApiError } from "../auth";
import { TextRefListEditor } from "../TextRefList";

interface ArchiveFormValues {
  name: string;
  systemText?: string;
  repository_id?: string;
  private?: boolean;
}

const FORM_FIELDS: (keyof ArchiveFormValues)[] = ["name", "repository_id"];

// useRepositoryOptions — заполняет Select репозиториев для repository_id.
// Не общий picker (тот отложен до подпроекта 9) — точечный select именно для
// этой связи, раз у Repository уже есть свой (небольшой, целиком постраничный)
// список (docs/data-model/entity-write.md, обсуждение подпроекта 3).
function useRepositoryOptions() {
  const [repositories, setRepositories] = useState<Repository[]>([]);

  useEffect(() => {
    fetchRepositories({ limit: 500 })
      .then(setRepositories)
      .catch(() => setRepositories([]));
  }, []);

  return repositories.map((r) => ({ value: r.id, label: r.name }));
}

// CreateArchiveModal — форма создания архива. system — только текстом (ссылка
// на сущность не допускается, models.Archive.Validate). repository_id —
// Select со списком хранилищ, не TextRef (просто id, строгая ссылка).
export function CreateArchiveModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Archive) => void;
}) {
  const [form] = Form.useForm<ArchiveFormValues>();
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const repositoryOptions = useRepositoryOptions();

  const reset = () => {
    form.resetFields();
    setNotes([]);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: ArchiveFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const systemText = (values.systemText ?? "").trim();
      const created = await createArchive({
        name: values.name,
        system: systemText ? { text: systemText } : null,
        repository_id: values.repository_id ?? "",
        notes,
        private: values.private ?? false,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof ArchiveFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить архив"
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
        <Form.Item name="systemText" label="Система иерархии (текстом)">
          <Input placeholder="фонд-опись-дело" />
        </Form.Item>
        <Form.Item name="repository_id" label="Хранилище">
          <Select
            allowClear
            placeholder="Не выбрано"
            options={repositoryOptions}
            showSearch
            filterOption={(input, option) =>
              (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
            }
          />
        </Form.Item>
        <Form.Item label="Заметки">
          <TextRefListEditor value={notes} onChange={setNotes} addLabel="+ заметка" />
        </Form.Item>
        <Form.Item name="private" valuePropName="checked">
          <Checkbox>Приватная запись</Checkbox>
        </Form.Item>
      </Form>
    </Modal>
  );
}
