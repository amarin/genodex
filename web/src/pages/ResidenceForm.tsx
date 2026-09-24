import { useEffect, useState } from "react";
import { Alert, Checkbox, Form, Input, Modal, Select } from "antd";
import {
  adminDivisionTypeLabel,
  createResidence,
  fetchAdminDivisions,
  MAX_PAGE_LIMIT,
  type AdminDivision,
  type FactDate,
  type Residence,
  type SourceLink,
} from "../api";
import { ApiError } from "../auth";
import { FactDateEditor } from "../FactDateEditor";
import { PersonPicker } from "../PersonPicker";
import { SourceLinkListEditor } from "../SourceLinkList";

interface ResidenceFormValues {
  note?: string;
  private?: boolean;
}

const FORM_FIELDS: (keyof ResidenceFormValues)[] = ["note"];

// useAdminDivisionOptions — заполняет Select административных делений для
// Residence.PlaceID — первая СТРОГАЯ (существование проверяется на сервере)
// ссылка на AdministrativeDivision в программе (все прочие ссылки на место
// остаются мягким TextRef, docs/data-model/entity-write.md §3.8). Точечный
// хук + инлайн-Select, не отдельный переиспользуемый компонент (в отличие
// от PersonPicker) — единственная точка использования в этом подпроекте,
// по образцу useRepositoryOptions/ArchiveForm.tsx.
function useAdminDivisionOptions() {
  const [divisions, setDivisions] = useState<AdminDivision[]>([]);

  useEffect(() => {
    fetchAdminDivisions({ limit: MAX_PAGE_LIMIT })
      .then(setDivisions)
      .catch(() => setDivisions([]));
  }, []);

  return divisions.map((d) => ({ value: d.id, label: `${d.name} (${adminDivisionTypeLabel(d.type)})` }));
}

// CreateResidenceModal — форма создания проживания персоны в месте.
// person_id — PersonPicker (подпроект 9). place_id — Select административных
// делений (useAdminDivisionOptions). note — ОДНА строка
// (models.Residence.Note), простой Input.TextArea, а не TextRefListEditor,
// в отличие от notes большинства сущностей (docs/data-model/entity-write.md
// §3.8).
export function CreateResidenceModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Residence) => void;
}) {
  const [form] = Form.useForm<ResidenceFormValues>();
  const [personId, setPersonId] = useState("");
  const [placeId, setPlaceId] = useState("");
  const [since, setSince] = useState<FactDate | null>(null);
  const [until, setUntil] = useState<FactDate | null>(null);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const divisionOptions = useAdminDivisionOptions();

  const reset = () => {
    form.resetFields();
    setPersonId("");
    setPlaceId("");
    setSince(null);
    setUntil(null);
    setSources([]);
    setError(null);
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const onFinish = async (values: ResidenceFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const created = await createResidence({
        person_id: personId,
        place_id: placeId,
        since,
        until,
        sources,
        note: (values.note ?? "").trim(),
        private: values.private ?? false,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof ResidenceFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить проживание"
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
        <Form.Item label="Персона" required>
          <PersonPicker value={personId} onChange={setPersonId} placeholder="Персона" />
        </Form.Item>
        <Form.Item label="Место" required>
          <Select
            style={{ width: 320 }}
            placeholder="Административное деление"
            options={divisionOptions}
            value={placeId || undefined}
            onChange={setPlaceId}
            showSearch
            filterOption={(input, option) =>
              (option?.label ?? "").toString().toLowerCase().includes(input.toLowerCase())
            }
          />
        </Form.Item>
        <Form.Item label="Начало периода">
          <FactDateEditor value={since} onChange={setSince} addLabel="+ дата начала" />
        </Form.Item>
        <Form.Item label="Конец периода">
          <FactDateEditor value={until} onChange={setUntil} addLabel="+ дата конца" />
        </Form.Item>
        <Form.Item name="note" label="Заметка">
          <Input.TextArea rows={3} />
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
