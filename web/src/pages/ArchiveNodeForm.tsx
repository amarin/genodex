import { useState } from "react";
import { Alert, Checkbox, Form, Input, Modal } from "antd";
import {
  createArchiveNode,
  type ArchiveNode,
  type FactDate,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError } from "../auth";
import { ArchiveNodePicker } from "../ArchiveNodePicker";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";
import { FactDateEditor } from "../FactDateEditor";

interface ArchiveNodeFormValues {
  type: string;
  label: string;
  name?: string;
  parishText?: string;
  private?: boolean;
}

const FORM_FIELDS: (keyof ArchiveNodeFormValues)[] = ["type", "label"];

// CreateArchiveNodeModal — форма создания архивного узла. archive_id —
// фиксирован пропом (модалка всегда открывается в известном контексте
// архива — со страницы ArchiveNodesList или из ArchiveNodeView) и НЕ
// редактируется здесь. parent_id — ArchiveNodePicker, scoped к этому же
// archiveId (родитель только в пределах своего архива — тот же инвариант,
// что проверяет сервер, docs/data-model/entity-write.md §3.4);
// предзаполняется пропом parentId (не null — «добавить дочерний узел», см.
// ArchiveNodesList/ArchiveNodeView), но остаётся редактируемым через
// picker — в отличие от CreateDivisionModal, где parent_id полем формы не
// является вовсе (там ArchiveNode уже умеет предлагать полноценный picker,
// у Division его нет).
export function CreateArchiveNodeModal({
  open,
  archiveId,
  parentId,
  onClose,
  onCreated,
}: {
  open: boolean;
  archiveId: string;
  parentId: string | null;
  onClose: () => void;
  onCreated: (created: ArchiveNode) => void;
}) {
  const [form] = Form.useForm<ArchiveNodeFormValues>();
  const [editParentId, setEditParentId] = useState<string | null>(parentId);
  const [editParentLabel, setEditParentLabel] = useState<string | null>(null);
  const [settlements, setSettlements] = useState<TextRef[]>([]);
  const [since, setSince] = useState<FactDate | null>(null);
  const [until, setUntil] = useState<FactDate | null>(null);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setEditParentId(parentId);
    setEditParentLabel(null);
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

  const onFinish = async (values: ArchiveNodeFormValues) => {
    setSubmitting(true);
    setError(null);
    try {
      const parishText = (values.parishText ?? "").trim();
      const created = await createArchiveNode({
        type: values.type,
        archive_id: archiveId,
        parent_id: editParentId,
        label: values.label,
        name: values.name ?? "",
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
        form.setFields([{ name: e.field as keyof ArchiveNodeFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать узел");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title={parentId == null ? "Добавить архивный узел" : "Добавить дочерний узел"}
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
          name="type"
          label="Тип"
          rules={[{ required: true, whitespace: true, message: "Введите тип (например, fond/opis/delo)" }]}
        >
          <Input placeholder="fond / opis / delo" autoFocus />
        </Form.Item>
        <Form.Item label="Родитель">
          <ArchiveNodePicker
            value={editParentId ?? ""}
            label={editParentLabel ?? undefined}
            archiveId={archiveId}
            onChange={(id, lbl) => {
              setEditParentId(id);
              setEditParentLabel(lbl);
            }}
          />
        </Form.Item>
        <Form.Item
          name="label"
          label="Метка"
          rules={[{ required: true, whitespace: true, message: "Введите метку" }]}
        >
          <Input placeholder="Фонд 1 / Опись 1 / Дело 1" />
        </Form.Item>
        <Form.Item name="name" label="Название">
          <Input />
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
