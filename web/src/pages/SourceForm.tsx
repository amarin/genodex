import { useEffect, useState } from "react";
import { Alert, Checkbox, Form, Input, Modal, Select } from "antd";
import { createSource, fetchRepositories, type FactDate, type Reliability, type Repository, type Source, type TextRef } from "../api";
import { ApiError } from "../auth";
import { FactDateEditor } from "../FactDateEditor";
import { TextRefListEditor } from "../TextRefList";

const KIND_OPTIONS = [
  { value: "archival-scan", label: "скан из архива" },
  { value: "transcription", label: "расшифровка" },
  { value: "document", label: "документ" },
  { value: "audio", label: "аудио" },
  { value: "photo", label: "фото" },
  { value: "memory", label: "со слов/по памяти" },
  { value: "external", label: "внешний источник" },
];

const RELIABILITY_OPTIONS: { value: Reliability; label: string }[] = [
  { value: "primary", label: "Первичный" },
  { value: "contemporary", label: "Современник" },
  { value: "memory", label: "Со слов/по памяти" },
  { value: "indirect", label: "Косвенный" },
  { value: "unknown", label: "Неизвестна" },
];

interface SourceFormValues {
  kind: string;
  title: string;
  author?: string;
  reliability: Reliability;
  repository_id?: string;
  private?: boolean;
}

const FORM_FIELDS: (keyof SourceFormValues)[] = ["kind", "title", "author", "reliability", "repository_id"];

// useRepositoryOptions — заполняет Select хранилищ для repository_id, по
// образцу ArchiveForm.tsx (подпроект 3).
function useRepositoryOptions() {
  const [repositories, setRepositories] = useState<Repository[]>([]);

  useEffect(() => {
    fetchRepositories({ limit: 500 })
      .then(setRepositories)
      .catch(() => setRepositories([]));
  }, []);

  return repositories.map((r) => ({ value: r.id, label: r.name }));
}

// CreateSourceModal — форма создания источника. date — FactDateEditor (см.
// подпроект 3), repository_id — Select со списком хранилищ (мягкая ссылка).
export function CreateSourceModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Source) => void;
}) {
  const [form] = Form.useForm<SourceFormValues>();
  const [date, setDate] = useState<FactDate | null>(null);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const repositoryOptions = useRepositoryOptions();

  const reset = () => {
    form.resetFields();
    setDate(null);
    setNotes([]);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: SourceFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await createSource({
        kind: values.kind,
        title: values.title,
        author: values.author,
        date,
        reliability: values.reliability,
        repository_id: values.repository_id ?? "",
        notes,
        private: values.private ?? false,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof SourceFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить источник"
      open={open}
      onCancel={handleClose}
      onOk={() => form.submit()}
      okText="Создать"
      cancelText="Отмена"
      confirmLoading={submitting}
      destroyOnHidden
    >
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
      <Form form={form} layout="vertical" onFinish={onFinish} initialValues={{ kind: "document", reliability: "unknown", private: false }}>
        <Form.Item name="kind" label="Вид" rules={[{ required: true, message: "Выберите вид" }]}>
          <Select options={KIND_OPTIONS} />
        </Form.Item>
        <Form.Item
          name="title"
          label="Название"
          rules={[{ required: true, whitespace: true, message: "Введите название" }]}
        >
          <Input autoFocus />
        </Form.Item>
        <Form.Item name="author" label="Автор">
          <Input />
        </Form.Item>
        <Form.Item label="Дата">
          <FactDateEditor value={date} onChange={setDate} addLabel="+ дата" />
        </Form.Item>
        <Form.Item name="reliability" label="Достоверность" rules={[{ required: true, message: "Выберите достоверность" }]}>
          <Select options={RELIABILITY_OPTIONS} />
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
