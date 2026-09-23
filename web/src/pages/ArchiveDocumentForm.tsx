import { useState } from "react";
import { Alert, Checkbox, Form, Input, Modal } from "antd";
import {
  createArchiveDocument,
  type ArchiveDocument,
  type FactDate,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError } from "../auth";
import { ArchiveNodePicker } from "../ArchiveNodePicker";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";
import { FactDateEditor } from "../FactDateEditor";

interface ArchiveDocumentFormValues {
  title: string;
  kind?: string;
  parishText?: string;
  private?: boolean;
}

const FORM_FIELDS: (keyof ArchiveDocumentFormValues)[] = ["title", "kind"];

// CreateArchiveDocumentModal — форма создания архивного документа (образец
// разбиения — NoteForm.tsx/NoteView.tsx: отдельная create-модалка, View —
// свой файл с toggle-редактированием). unit_id — обязательная строгая
// ссылка на ArchiveNode через ArchiveNodePicker БЕЗ заранее известного
// archiveId: свободностоящее создание документа не привязано к конкретному
// архиву заранее — picker сперва просит выбрать архив, потом узел внутри
// него (docs/data-model/entity-write.md §3.5).
export function CreateArchiveDocumentModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: ArchiveDocument) => void;
}) {
  const [form] = Form.useForm<ArchiveDocumentFormValues>();
  const [unitId, setUnitId] = useState("");
  const [unitLabel, setUnitLabel] = useState<string | null>(null);
  const [settlements, setSettlements] = useState<TextRef[]>([]);
  const [since, setSince] = useState<FactDate | null>(null);
  const [until, setUntil] = useState<FactDate | null>(null);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setUnitId("");
    setUnitLabel(null);
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

  const onFinish = async (values: ArchiveDocumentFormValues) => {
    if (!unitId) {
      setError("Выберите единицу хранения");
      return;
    }
    setSubmitting(true);
    setError(null);
    try {
      const parishText = (values.parishText ?? "").trim();
      const created = await createArchiveDocument({
        unit_id: unitId,
        title: values.title,
        kind: values.kind ?? "",
        since,
        until,
        parish: parishText ? { text: parishText } : null,
        settlements,
        notes,
        sources,
        private: values.private ?? false,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof ArchiveDocumentFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать документ");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить архивный документ"
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
        <Form.Item label="Единица хранения" required>
          <ArchiveNodePicker
            value={unitId}
            label={unitLabel ?? undefined}
            onChange={(nid, lbl) => {
              setUnitId(nid);
              setUnitLabel(lbl);
            }}
          />
        </Form.Item>
        <Form.Item
          name="title"
          label="Заголовок"
          rules={[{ required: true, whitespace: true, message: "Введите заголовок" }]}
        >
          <Input autoFocus />
        </Form.Item>
        <Form.Item name="kind" label="Вид">
          <Input placeholder="метрическая книга / исповедная роспись" />
        </Form.Item>
        <Form.Item name="parishText" label="Приход (текстом)">
          <Input placeholder="Никольский приход" />
        </Form.Item>
        <Form.Item label="Начало периода">
          <FactDateEditor value={since} onChange={setSince} addLabel="+ дата начала" />
        </Form.Item>
        <Form.Item label="Конец периода">
          <FactDateEditor value={until} onChange={setUntil} addLabel="+ дата конца" />
        </Form.Item>
        <Form.Item label="Населённые пункты">
          <TextRefListEditor value={settlements} onChange={setSettlements} addLabel="+ населённый пункт" />
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
