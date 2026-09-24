import { useState } from "react";
import { Alert, Checkbox, Form, Input, Modal, Select } from "antd";
import {
  createRelation,
  RELATION_KIND_OPTIONS,
  type FactDate,
  type Relation,
  type RelationKind,
  type SourceLink,
  type TextRef,
} from "../api";
import { ApiError } from "../auth";
import { FactDateEditor } from "../FactDateEditor";
import { PersonPicker } from "../PersonPicker";
import { SourceLinkListEditor } from "../SourceLinkList";
import { TextRefListEditor } from "../TextRefList";

interface RelationFormValues {
  kind: RelationKind;
  rel_type?: string;
  private?: boolean;
}

// FORM_FIELDS — только настоящие Form.Item'ы этой формы; person_a/person_b
// управляются отдельным React-состоянием (PersonPicker вне antd Form, как и
// прочие *ListEditor'ы), так что 422 с field="person_a"/"person_b" не может
// быть привязан к конкретному контролу формы и уходит в общий Alert (тот же
// приём, что и для sources[i].citation_id в других формах программы).
const FORM_FIELDS: (keyof RelationFormValues)[] = ["kind", "rel_type"];

// CreateRelationModal — форма создания ребра графа родства. kind — закрытый
// enum (Select); rel_type виден и обязателен ТОЛЬКО при kind=associate
// (models.Relation.Validate — требует rel_type для associate, запрещает
// иначе) — условная видимость поля по дискриминатору, тот же принцип, что
// AnchorEditor.tsx, но проще: меняется видимость одного текстового поля, а
// не весь набор полей формы. person_a/person_b — первое использование
// PersonPicker (подпроект 9) — два независимых picker'а на одной форме,
// первая сущность программы с двумя строгими ссылками на один и тот же тип
// (docs/data-model/entity-write.md §3.8).
export function CreateRelationModal({
  open,
  onClose,
  onCreated,
}: {
  open: boolean;
  onClose: () => void;
  onCreated: (created: Relation) => void;
}) {
  const [form] = Form.useForm<RelationFormValues>();
  const kind = Form.useWatch("kind", form);
  const [personA, setPersonA] = useState("");
  const [personB, setPersonB] = useState("");
  const [since, setSince] = useState<FactDate | null>(null);
  const [until, setUntil] = useState<FactDate | null>(null);
  const [notes, setNotes] = useState<TextRef[]>([]);
  const [sources, setSources] = useState<SourceLink[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const reset = () => {
    form.resetFields();
    setPersonA("");
    setPersonB("");
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

  const onFinish = async (values: RelationFormValues) => {
    setError(null);
    if (personA !== "" && personA === personB) {
      setError("Связь не может быть с самим собой: персона A и персона Б должны отличаться");
      return;
    }
    setSubmitting(true);
    try {
      const created = await createRelation({
        kind: values.kind,
        rel_type: values.kind === "associate" ? (values.rel_type ?? "").trim() : "",
        person_a: personA,
        person_b: personB,
        since,
        until,
        sources,
        notes,
        private: values.private ?? false,
      });
      reset();
      onCreated(created);
    } catch (e) {
      if (e instanceof ApiError && e.field != null && (FORM_FIELDS as string[]).includes(e.field)) {
        form.setFields([{ name: e.field as keyof RelationFormValues, errors: [e.message] }]);
      } else {
        setError(e instanceof ApiError ? e.message : "Не удалось создать запись");
      }
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      title="Добавить связь"
      open={open}
      onCancel={handleClose}
      onOk={() => form.submit()}
      okText="Создать"
      cancelText="Отмена"
      confirmLoading={submitting}
      destroyOnHidden
    >
      {error != null && <Alert type="error" showIcon message={error} style={{ marginBottom: 16 }} />}
      <Form form={form} layout="vertical" onFinish={onFinish} initialValues={{ kind: "blood", private: false }}>
        <Form.Item name="kind" label="Вид связи" rules={[{ required: true, message: "Выберите вид связи" }]}>
          <Select options={RELATION_KIND_OPTIONS} />
        </Form.Item>
        {kind === "associate" && (
          <Form.Item
            name="rel_type"
            label="Тип связи"
            rules={[{ required: true, whitespace: true, message: "Укажите тип связи" }]}
          >
            <Input placeholder="godparent / witness / neighbor / friend / colleague" />
          </Form.Item>
        )}
        <Form.Item label="Персона A" required>
          <PersonPicker value={personA} onChange={setPersonA} placeholder="Персона A" />
        </Form.Item>
        <Form.Item label="Персона Б" required>
          <PersonPicker value={personB} onChange={setPersonB} placeholder="Персона Б" />
        </Form.Item>
        <Form.Item label="Начало периода">
          <FactDateEditor value={since} onChange={setSince} addLabel="+ дата начала" />
        </Form.Item>
        <Form.Item label="Конец периода">
          <FactDateEditor value={until} onChange={setUntil} addLabel="+ дата конца" />
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
